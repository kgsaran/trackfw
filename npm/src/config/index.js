'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { parseDocument, isScalar, isAlias, isSeq, isMap } = require('yaml');

function expandPath(filePath) {
  if (!filePath || typeof filePath !== 'string') return filePath;
  if (filePath === '~') {
    return os.homedir();
  }
  if (filePath.startsWith('~/') || filePath.startsWith('~\\')) {
    return path.join(os.homedir(), filePath.slice(2));
  }
  return filePath;
}

function defaults() {
  return {
    adrDirs: ['docs/adr'].map(expandPath),
    reqDir: expandPath('docs/req'),
    roadmapDir: expandPath('docs/roadmaps'),
    roadmapNamespacing: 'flat',
    agents: [],
    governanceMode: '',
    lenientUntil: '',
    wipLimit: 1,
    wipBySquad: false,
    staleWipDays: 7,
    requireReqInCommit: false,
    strictCiPaths: false,
    traceIdField: '',
    forge: '',
    // NOVOS campos:
    linkFields: {
      req:     ['REQ:'],
      adr:     ['ADR:'],
      roadmap: ['Roadmap:'],
    },
    acceptanceMarkers: ['## Acceptance Criteria', '## Critérios de Aceite'],
    rules: {
      wip_has_req:          'error',
      wip_acceptance:       'error',
      wip_limit:            'error',
      stale_wip:            'warning',
      adr_orphan:           'warning',
      ref_targets_exist:    'error',
      folder_status:        'warning',
      filename_uniqueness:  'error',
      blocked_by_draft_adr: 'error',
      adr_accepted_when_req_done: 'error',
    },
  };
}

let _instance = null;

// MALFORMED_CONFIG_MESSAGE is written to stderr, verbatim, when trackfw.yaml exists but fails to
// parse as YAML. Kept identical, character-for-character, to Go's MalformedConfigMessage and
// Python's MALFORMED_CONFIG_MESSAGE — see the comment on parse() below for why the text is
// static rather than built from the underlying library's error.
const MALFORMED_CONFIG_MESSAGE = 'trackfw: erro ao carregar "trackfw.yaml": YAML malformado. Corrija a sintaxe do arquivo antes de continuar.';

function load(cwd) {
  if (_instance) return _instance;
  _instance = defaults();
  const yamlPath = path.join(cwd || process.cwd(), 'trackfw.yaml');
  if (!fs.existsSync(yamlPath)) return _instance;
  const content = fs.readFileSync(yamlPath, 'utf8');
  const malformed = parse(content, _instance);
  if (malformed) {
    process.stderr.write(MALFORMED_CONFIG_MESSAGE + '\n');
    process.exit(1);
  }
  return _instance;
}

function reset() {
  _instance = null;
}

// resolveAlias segue a cadeia de um Alias (b: *x) até o nó âncora. Ler .source direto de um
// Alias não resolvido devolveria o NOME da âncora, não o valor — risco confirmado no ML-0A.
//
// state (opcional) é marcado como { unresolved: true } quando node.resolve(doc) devolve
// undefined — caso de uma âncora referenciada antes de ser definida (b: *x / a: &x 3), que a
// spec YAML trata como inválido. gopkg.in/yaml.v3 e PyYAML rejeitam esse arquivo (yaml.v3:
// "unknown anchor 'x' referenced"; PyYAML: ComposerError "found undefined alias"); a lib `yaml`
// do Node não popula doc.errors para isso — resolve() simplesmente devolve undefined em tempo
// de leitura. Sem este sinalizador, o valor viraria string vazia em silêncio (divergência de
// exit code encontrada na auditoria cruzada do ML-1B); com ele, parse() devolve malformado=true
// e load() converge com Go/Python.
function resolveAlias(doc, node, state) {
  let n = node;
  while (isAlias(n)) {
    const resolved = n.resolve(doc);
    if (resolved === undefined) {
      if (state) state.unresolved = true;
      return null;
    }
    n = resolved;
  }
  return n;
}

// normalizeNode converte um nó da árvore `yaml` (Scalar/Seq/Map/Alias) para uma string
// (escalar, usando o texto bruto pré-coerção via Scalar.source), um array de strings
// (sequência) ou um objeto plano (mapeamento) — recursivamente.
//
// Scalar.source já devolve o texto correto tanto para escalares "plain" (não processados —
// preserva "yes", "010", "2026-08-02" como estão no arquivo) quanto para escalares quoted/bloco
// (já des-escapados, iguais ao .value) — confirmado empiricamente no ML-1A: não há necessidade
// de tratar quoted e plain de formas diferentes.
function normalizeNode(doc, node, state) {
  const n = resolveAlias(doc, node, state);
  if (n == null) return '';
  if (isScalar(n)) {
    return n.source != null ? n.source : (n.value == null ? '' : String(n.value));
  }
  if (isSeq(n)) {
    return n.items.map((item) => normalizeNode(doc, item, state));
  }
  if (isMap(n)) {
    const result = {};
    for (const pair of n.items) {
      const key = resolveAlias(doc, pair.key, state);
      const keyStr = isScalar(key) ? (key.source != null ? key.source : String(key.value)) : String(key);
      result[keyStr] = normalizeNode(doc, pair.value, state);
    }
    return result;
  }
  return '';
}

function stringVal(m, key) {
  const v = m[key];
  return typeof v === 'string' ? v : undefined;
}

// stringList converte um valor normalizado (array) em array de strings. Uma sequência
// presente-porém-vazia devolve array vazio (não undefined), distinguindo "presente e vazio" de
// "ausente" — contrato herdado do fix de lista inline.
function stringList(v) {
  if (!Array.isArray(v)) return undefined;
  return v.filter((item) => typeof item === 'string');
}

// NON_FATAL_ERROR_CODES holds `yaml` package error codes that must NOT trigger the fatal path,
// because gopkg.in/yaml.v3 (decoding into a generic Node, not a struct) and PyYAML's
// yaml.compose() silently accept the same input — divergence found by ML-1B audit: a
// "wip_limit: 3\nwip_limit: 4\n" duplicate-key file made Node exit 1 while Go and Python both
// parsed it fine (both resolve to "last key wins", same value trackfw ends up using in all
// three once DUPLICATE_KEY is excluded here). Only Node's `yaml` treats duplicate keys as a
// composer-level error; treating it as fatal here would make the fatal trigger itself diverge
// across CLIs, which is exactly the defect this ML exists to avoid. Any other doc.errors entry
// (e.g. BAD_INDENT — an actually-unparseable document) still triggers the fatal path below.
const NON_FATAL_ERROR_CODES = new Set(['DUPLICATE_KEY']);

// parse applies the ~20 known keys from content onto cfg. Returns true when content is
// malformed YAML (the caller, load(), turns that into a fatal stderr message + exit 1) and
// false otherwise — including the benign cases of an absent/empty/comments-only document
// (doc.contents === null with no errors) or a document whose top-level node parses fine but
// isn't a mapping (valid YAML, unexpected shape): neither of those is a parse failure, so both
// stay silent no-ops, same as before this function grew an error signal.
//
// The `yaml` package doesn't throw on syntax errors — parseDocument() always returns a Document,
// and populates doc.errors instead (a passive failure channel). The try/catch below is kept as
// defense-in-depth for library versions/inputs that do throw, but the actual malformed-YAML path
// exercised by trackfw.yaml goes through the doc.errors.length > 0 check.
function parse(content, cfg) {
  let doc;
  try {
    doc = parseDocument(content);
  } catch (e) {
    return true;
  }
  if (doc && doc.errors && doc.errors.some((e) => !NON_FATAL_ERROR_CODES.has(e.code))) return true;
  if (!doc || !doc.contents) return false;
  if (!isMap(doc.contents)) return false;

  const state = { unresolved: false };
  const m = normalizeNode(doc, doc.contents, state);
  if (state.unresolved) return true;
  if (typeof m !== 'object' || m === null || Array.isArray(m)) return false;

  if (m.adr_dirs !== undefined) {
    const items = stringList(m.adr_dirs);
    if (items) cfg.adrDirs = items.map(expandPath);
  }
  if (stringVal(m, 'req_dir') !== undefined) cfg.reqDir = expandPath(m.req_dir);
  if (stringVal(m, 'roadmap_dir') !== undefined) cfg.roadmapDir = expandPath(m.roadmap_dir);
  if (stringVal(m, 'roadmap_namespacing') !== undefined) cfg.roadmapNamespacing = m.roadmap_namespacing;
  if (m.agents !== undefined) {
    const items = stringList(m.agents);
    if (items) cfg.agents = items;
  }
  if (stringVal(m, 'governance_mode') !== undefined) cfg.governanceMode = m.governance_mode;
  if (stringVal(m, 'lenient_until') !== undefined) cfg.lenientUntil = m.lenient_until;
  if (stringVal(m, 'wip_limit') !== undefined) {
    const n = parseInt(m.wip_limit, 10);
    if (n > 0) cfg.wipLimit = n;
  }
  if (stringVal(m, 'wip_by_squad') !== undefined) cfg.wipBySquad = m.wip_by_squad === 'true';
  if (stringVal(m, 'stale_wip_days') !== undefined) {
    const n = parseInt(m.stale_wip_days, 10);
    if (n > 0) cfg.staleWipDays = n;
  }
  if (stringVal(m, 'require_req_in_commit') !== undefined) cfg.requireReqInCommit = m.require_req_in_commit === 'true';
  if (stringVal(m, 'strict_ci_paths') !== undefined) cfg.strictCiPaths = m.strict_ci_paths === 'true';
  if (stringVal(m, 'trace_id_field') !== undefined) cfg.traceIdField = m.trace_id_field;
  if (stringVal(m, 'forge') !== undefined) cfg.forge = m.forge;
  if (m.acceptance_markers !== undefined) {
    const items = stringList(m.acceptance_markers);
    if (items) cfg.acceptanceMarkers = items;
  }
  if (m.link_fields !== undefined && typeof m.link_fields === 'object' && !Array.isArray(m.link_fields)) {
    const lf = m.link_fields;
    const req = stringList(lf.req);
    if (req) cfg.linkFields.req = req;
    const adr = stringList(lf.adr);
    if (adr) cfg.linkFields.adr = adr;
    const roadmap = stringList(lf.roadmap);
    if (roadmap) cfg.linkFields.roadmap = roadmap;
  }
  if (m.rules !== undefined && typeof m.rules === 'object' && !Array.isArray(m.rules)) {
    for (const [k, v] of Object.entries(m.rules)) {
      if (typeof v === 'string') cfg.rules[k] = v;
    }
  }
  return false;
}

const NAMESPACING_FLAT = 'flat';
const NAMESPACING_BY_AGENT = 'by_agent';

module.exports = {
  load,
  reset,
  defaults,
  expandPath,
  NAMESPACING_FLAT,
  NAMESPACING_BY_AGENT,
  MALFORMED_CONFIG_MESSAGE,
};
