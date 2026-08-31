'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { parseDocument, isScalar, isAlias, isSeq, isMap } = require('yaml');
const { homedir } = require('../homedir')

function expandPath(filePath) {
  if (!filePath || typeof filePath !== 'string') return filePath;
  if (filePath === '~') {
    return homedir();
  }
  if (filePath.startsWith('~/') || filePath.startsWith('~\\')) {
    return path.join(homedir(), filePath.slice(2));
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
    // ML-1A namespaces — see ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-
    // namespaces-tipados.md. Keys stay flat at the YAML root; these are memory-only groupings
    // populated by the same single parse() below, not a second read of trackfw.yaml.
    update: {
      hooks: '',
      ci: '',
      backend: '',
      frontend: '',
      pkgManager: '',
      // ML-2A (agentes especialistas aceitam contexto de convenções) field — mirrors Go's
      // UpdateConfig.AgentConventions: agent_conventions, free-text, multi-line (default: '').
      agentConventions: '',
    },
    sync: {
      linearApiKey: '',
      linearTeamId: '',
      jiraBaseUrl: '',
      jiraEmail: '',
      jiraToken: '',
      jiraProject: '',
    },
    // credential_guard field — see ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-
    // credenciais-reais-por-subagentes.md.
    credentialGuard: {
      mode: 'warn',
    },
    // agent_models field — see ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
    // Maps tier name (e.g. "opus", "sonnet") to version string (e.g. "5", "4.6").
    // Absent or empty map → behavior identical to today (tier alias used verbatim).
    agentModels: {},
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

// MALFORMED_GLOBAL_CONFIG_MESSAGE is written to stderr (non-fatal) when ~/.trackfw/trackfw.yaml
// exists but fails to parse as valid YAML. Must be byte-identical to Go's MalformedGlobalConfigMessage
// and Python's MALFORMED_GLOBAL_CONFIG_MESSAGE — ADR-2026-08-23.
const MALFORMED_GLOBAL_CONFIG_MESSAGE = 'trackfw: aviso: "~/.trackfw/trackfw.yaml" tem YAML malformado — config global de modelo ignorada; usando tier canônico.';

// GLOBAL_AGENT_MODELS_NONE_MESSAGE is emitted to stderr when agent_models is not configured
// in the global config and not found in cwd's trackfw.yaml either.
// Must be byte-identical to Go's GlobalAgentModelsNoneMessage and Python's equivalent.
const GLOBAL_AGENT_MODELS_NONE_MESSAGE = 'trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml — usando tier canônico. Configure em ~/.trackfw/trackfw.yaml para pinar versões.';

// GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE is emitted when agent_models is found in the
// project's trackfw.yaml but not in the global config (AC4/AC14). Value NOT applied.
// Must be byte-identical to Go's GlobalAgentModelsProjectOnlyMessage and Python's equivalent.
const GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE = 'trackfw: agents global: agent_models configurado em trackfw.yaml do projeto mas não vale para escopo global. Mova a chave para ~/.trackfw/trackfw.yaml.';

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

  // ML-1A — update and sync namespaces. Same normalized `m` as above, no second read.
  if (stringVal(m, 'hooks') !== undefined) cfg.update.hooks = m.hooks;
  if (stringVal(m, 'ci') !== undefined) cfg.update.ci = m.ci;
  if (stringVal(m, 'backend') !== undefined) cfg.update.backend = m.backend;
  if (stringVal(m, 'frontend') !== undefined) cfg.update.frontend = m.frontend;
  if (stringVal(m, 'pkg_manager') !== undefined) cfg.update.pkgManager = m.pkg_manager;
  if (stringVal(m, 'agent_conventions') !== undefined) cfg.update.agentConventions = m.agent_conventions;
  if (stringVal(m, 'linear_api_key') !== undefined) cfg.sync.linearApiKey = m.linear_api_key;
  if (stringVal(m, 'linear_team_id') !== undefined) cfg.sync.linearTeamId = m.linear_team_id;
  if (stringVal(m, 'jira_base_url') !== undefined) cfg.sync.jiraBaseUrl = m.jira_base_url;
  if (stringVal(m, 'jira_email') !== undefined) cfg.sync.jiraEmail = m.jira_email;
  if (stringVal(m, 'jira_token') !== undefined) cfg.sync.jiraToken = m.jira_token;
  if (stringVal(m, 'jira_project') !== undefined) cfg.sync.jiraProject = m.jira_project;

  // credential_guard field — nested mapping, same shape as link_fields above. An unrecognized
  // mode value (or an absent/malformed credential_guard block) falls back to the safe default
  // ("warn") silently, matching how every other unrecognized-shape field in this parser behaves
  // (e.g. roadmap_namespacing, forge) — no fatal path, no stderr message, for one enum key.
  if (m.credential_guard !== undefined && typeof m.credential_guard === 'object' && !Array.isArray(m.credential_guard)) {
    const mode = stringVal(m.credential_guard, 'mode');
    if (mode === 'warn' || mode === 'block') cfg.credentialGuard.mode = mode;
  }

  // agent_models field — flat mapping from tier name to version string.
  // See ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
  // An absent, malformed, or empty block leaves agentModels as the empty object from defaults(),
  // preserving identical behavior to today. A key with an empty string value is stored as-is
  // (the render layer treats empty string as "no pin" — fall back to tier alias).
  if (m.agent_models !== undefined && typeof m.agent_models === 'object' && !Array.isArray(m.agent_models)) {
    for (const [k, v] of Object.entries(m.agent_models)) {
      if (typeof v === 'string') cfg.agentModels[k] = v;
    }
  }

  return false;
}

// parseRulesFromContent parses only the `rules:` mapping out of arbitrary trackfw.yaml content
// (e.g. a git-HEAD blob obtained via `git show HEAD:./trackfw.yaml`, not the CWD file load()
// reads) and returns it as name->severity. Used by the validator's HEAD-anchored credential-guard
// rules — see ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-
// entre-head-e-disco.md — which need `rules:` as it existed at a specific git ref, not the CWD, so
// they cannot go through the load() singleton (always reads the CWD file and caches it once per
// process). Reuses parse() over an ephemeral cfg object, mirroring Go's ParseRulesFromContent, so
// this and load() can never diverge on how `rules:` itself is read — purely additive, does not
// touch load()/parse() otherwise. Malformed content (parse() returning true) is treated the same
// as "no rules:" — an empty map — since the caller only wants a best-effort read of a historical
// git blob, not a fatal exit like load() has for the live CWD file.
function parseRulesFromContent(content) {
  // parse() assumes cfg already has the nested-object shape defaults() provides for
  // credentialGuard/update/sync/linkFields (it assigns into them, e.g. cfg.update.hooks = ...,
  // never creates them) — so this cannot be a bare { rules: {} } literal or parse() throws on any
  // content that sets one of those nested keys. rules starts empty (not seeded from defaults()),
  // deliberately: the caller wants exactly what `rules:` in content declares, nothing else.
  const cfg = { rules: {}, credentialGuard: {}, update: {}, sync: {}, linkFields: {}, agentModels: {} };
  parse(content, cfg);
  return cfg.rules;
}

// readAgentConventions reads the `agent_conventions` key directly out of <cwd>/trackfw.yaml,
// bypassing the load() singleton — mirrors Go's config.ReadAgentConventions, needed here because
// generators (init.js's injectOrUpdateRules) inject rules into agent files for a given cwd that is
// not necessarily the process's own working directory the singleton is cached against. The file
// being absent, unreadable, malformed or simply missing the key are all treated the same: return
// '' silently, never an error — this is a best-effort read of an optional, free-text field.
function readAgentConventions(cwd) {
  let content;
  try {
    content = fs.readFileSync(path.join(cwd || process.cwd(), 'trackfw.yaml'), 'utf8');
  } catch (_) {
    return '';
  }
  const cfg = { rules: {}, credentialGuard: {}, update: { agentConventions: '' }, sync: {}, linkFields: {}, agentModels: {} };
  try {
    parse(content, cfg);
  } catch (_) {
    return '';
  }
  return cfg.update.agentConventions || '';
}

// _cwdAgentModelsSource returns 'project_only' if cwd's trackfw.yaml has agent_models configured,
// 'none' otherwise. Used for the AC14 diagnostic in loadGlobalAgentModels.
function _cwdAgentModelsSource(cwd) {
  if (!cwd) return 'none';
  try {
    const cwdContent = fs.readFileSync(path.join(cwd, 'trackfw.yaml'), 'utf8');
    const cwdCfg = { rules: {}, credentialGuard: {}, update: {}, sync: {}, linkFields: {}, agentModels: {} };
    parse(cwdContent, cwdCfg);
    if (cwdCfg.agentModels && Object.keys(cwdCfg.agentModels).length > 0) return 'project_only';
  } catch (_) {}
  return 'none';
}

// loadGlobalAgentModels reads agent_models from ~/.trackfw/trackfw.yaml, bypassing the load()
// singleton. homeDir is homedir(); cwd is process.cwd() (used only for AC14 diagnostic).
// Returns { models: {}, source } where source is one of:
//   'global'            — agent_models resolved from ~/.trackfw/trackfw.yaml
//   'none'              — global config absent or has no agent_models; cwd also has none
//   'project_only'      — agent_models in cwd's trackfw.yaml but not global (AC14 trigger)
//   'global_malformed'  — ~/.trackfw/trackfw.yaml exists but has invalid YAML (AC12 trigger)
// Never calls process.exit. Pattern mirrors readAgentConventions above.
function loadGlobalAgentModels(homeDir, cwd) {
  const globalPath = path.join(homeDir, '.trackfw', 'trackfw.yaml');
  let data;
  try {
    data = fs.readFileSync(globalPath, 'utf8');
  } catch (_) {
    return { models: {}, source: _cwdAgentModelsSource(cwd) };
  }
  // Validate YAML (non-fatal for global config, per AC12).
  const globalCfg = { rules: {}, credentialGuard: {}, update: {}, sync: {}, linkFields: {}, agentModels: {} };
  const malformed = parse(data, globalCfg);
  if (malformed) {
    return { models: {}, source: 'global_malformed' };
  }
  if (globalCfg.agentModels && Object.keys(globalCfg.agentModels).length > 0) {
    return { models: globalCfg.agentModels, source: 'global' };
  }
  // Global file exists but has no agent_models → check cwd (AC14).
  return { models: {}, source: _cwdAgentModelsSource(cwd) };
}

// resolveAgentModels returns { models, warning } for the given scope.
// For global scope, reads from ~/.trackfw/trackfw.yaml via loadGlobalAgentModels.
// For project scope, reads from the cwd's trackfw.yaml via load().
// warning is a non-empty string when the caller should emit an advisory to stderr.
// resolveAgentModels never writes to stderr itself.
function resolveAgentModels(scope, homeDir, cwd) {
  if (scope !== 'global') {
    return { models: load().agentModels || {}, warning: '' };
  }
  const { models, source } = loadGlobalAgentModels(homeDir, cwd);
  let warning = '';
  if (source === 'none') warning = GLOBAL_AGENT_MODELS_NONE_MESSAGE;
  else if (source === 'project_only') warning = GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE;
  else if (source === 'global_malformed') warning = MALFORMED_GLOBAL_CONFIG_MESSAGE;
  return { models, warning };
}

const NAMESPACING_FLAT = 'flat';
const NAMESPACING_BY_AGENT = 'by_agent';

module.exports = {
  load,
  reset,
  defaults,
  expandPath,
  parseRulesFromContent,
  readAgentConventions,
  loadGlobalAgentModels,
  resolveAgentModels,
  NAMESPACING_FLAT,
  NAMESPACING_BY_AGENT,
  MALFORMED_CONFIG_MESSAGE,
  MALFORMED_GLOBAL_CONFIG_MESSAGE,
  GLOBAL_AGENT_MODELS_NONE_MESSAGE,
  GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE,
};
