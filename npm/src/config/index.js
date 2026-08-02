'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

// isInlineList detecta a forma flow-style de lista YAML na própria linha da chave:
// "chave: [a, b]". Não confundir com bloco (linhas seguintes com "- item").
function isInlineList(val) {
  return typeof val === 'string' && val.trim().startsWith('[');
}

// splitTopLevelCommas separa s por vírgulas fora de aspas (simples ou duplas), preservando
// vírgulas dentro de itens citados (caso 8 do contrato: ["a, b", "c"]).
function splitTopLevelCommas(s) {
  const tokens = [];
  let cur = '';
  let quote = null;
  for (const ch of s) {
    if (quote) {
      cur += ch;
      if (ch === quote) quote = null;
    } else if (ch === '"' || ch === "'") {
      quote = ch;
      cur += ch;
    } else if (ch === ',') {
      tokens.push(cur);
      cur = '';
    } else {
      cur += ch;
    }
  }
  tokens.push(cur);
  return tokens;
}

// parseInlineList decompõe uma lista YAML inline ("[a, b]") em itens, respeitando aspas
// simples e duplas ao redor de itens. "[]" retorna array vazio (não undefined), para
// distinguir "presente e vazio" de "ausente" no chamador.
function parseInlineList(val) {
  let inner = val.trim();
  if (inner.startsWith('[')) inner = inner.slice(1);
  if (inner.endsWith(']')) inner = inner.slice(0, -1);
  inner = inner.trim();
  if (inner === '') return [];
  return splitTopLevelCommas(inner).map((t) => t.trim().replace(/^["']|["']$/g, ''));
}

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

function load(cwd) {
  if (_instance) return _instance;
  _instance = defaults();
  const yamlPath = path.join(cwd || process.cwd(), 'trackfw.yaml');
  if (!fs.existsSync(yamlPath)) return _instance;
  const content = fs.readFileSync(yamlPath, 'utf8');
  parse(content, _instance);
  return _instance;
}

function reset() {
  _instance = null;
}

function parse(content, cfg) {
  const lines = content.split('\n');

  // estados existentes
  let inAdrDirs = false;
  let inAgents = false;
  let adrDirs = [];
  let agents = [];

  // NOVOS estados
  let inLinkFields = false;
  let inLinkFieldsReq = false;
  let inLinkFieldsAdr = false;
  let inLinkFieldsRoadmap = false;
  let linkFieldsReq = [];
  let linkFieldsAdr = [];
  let linkFieldsRoadmap = [];

  let inAcceptanceMarkers = false;
  let acceptanceMarkers = [];

  let inRules = false;
  let rules = {};

  function flushBlocks() {
    if (inAdrDirs && adrDirs.length) cfg.adrDirs = adrDirs.map(expandPath);
    if (inAgents && agents.length) cfg.agents = agents;
    if (inLinkFields) {
      if (inLinkFieldsReq && linkFieldsReq.length) cfg.linkFields.req = linkFieldsReq;
      if (inLinkFieldsAdr && linkFieldsAdr.length) cfg.linkFields.adr = linkFieldsAdr;
      if (inLinkFieldsRoadmap && linkFieldsRoadmap.length) cfg.linkFields.roadmap = linkFieldsRoadmap;
    }
    if (inAcceptanceMarkers && acceptanceMarkers.length) cfg.acceptanceMarkers = acceptanceMarkers;
    if (inRules && Object.keys(rules).length) Object.assign(cfg.rules, rules);
    // reset
    inAdrDirs = false; adrDirs = [];
    inAgents = false; agents = [];
    inLinkFields = false;
    inLinkFieldsReq = false; inLinkFieldsAdr = false; inLinkFieldsRoadmap = false;
    linkFieldsReq = []; linkFieldsAdr = []; linkFieldsRoadmap = [];
    inAcceptanceMarkers = false; acceptanceMarkers = [];
    inRules = false; rules = {};
  }

  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line) continue;
    const hasIndent = rawLine.length > 0 && (rawLine[0] === ' ' || rawLine[0] === '\t');

    // Uma sequência em bloco pode estar no mesmo nível de indentação da chave que a abre
    // (YAML válido: "agents:\n- zeus\n- apolo"). Uma linha "- " sem indentação continua a
    // lista aberta em vez de ser tratada como nova chave top-level.
    const isListItem = line.startsWith('- ');
    const continuesOpenList = isListItem && (inAdrDirs || inAgents || inAcceptanceMarkers);

    if (!hasIndent && !continuesOpenList) {
      flushBlocks();
    }

    if (hasIndent || continuesOpenList) {
      if (inAdrDirs) {
        if (line.startsWith('- ')) {
          let val = line.slice(2).trim();
          val = val.replace(/^["']|["']$/g, '');
          adrDirs.push(expandPath(val));
        }
        continue;
      }
      if (inAgents) {
        if (line.startsWith('- ')) agents.push(line.slice(2).trim());
        continue;
      }
      if (inAcceptanceMarkers) {
        if (line.startsWith('- ')) {
          let val = line.slice(2).trim();
          val = val.replace(/^["']|["']$/g, '');
          acceptanceMarkers.push(val);
        }
        continue;
      }
      if (inRules) {
        const colonIdx = line.indexOf(':');
        if (colonIdx > 0) {
          const k = line.slice(0, colonIdx).trim();
          const v = line.slice(colonIdx + 1).trim().replace(/^["']|["']$/g, '');
          if (k) rules[k] = v;
        }
        continue;
      }
      if (inLinkFields) {
        if (line.startsWith('- ')) {
          let val = line.slice(2).trim();
          val = val.replace(/^["']|["']$/g, '');
          if (inLinkFieldsReq) linkFieldsReq.push(val);
          else if (inLinkFieldsAdr) linkFieldsAdr.push(val);
          else if (inLinkFieldsRoadmap) linkFieldsRoadmap.push(val);
        } else {
          // sub-chave dentro de link_fields
          const colonIdx = line.indexOf(':');
          const subKey = colonIdx > 0 ? line.slice(0, colonIdx).trim() : line.replace(':', '').trim();
          const subVal = colonIdx > 0 ? line.slice(colonIdx + 1).trim() : '';
          // flush sub-campo anterior
          if (inLinkFieldsReq && linkFieldsReq.length) { cfg.linkFields.req = linkFieldsReq; linkFieldsReq = []; }
          if (inLinkFieldsAdr && linkFieldsAdr.length) { cfg.linkFields.adr = linkFieldsAdr; linkFieldsAdr = []; }
          if (inLinkFieldsRoadmap && linkFieldsRoadmap.length) { cfg.linkFields.roadmap = linkFieldsRoadmap; linkFieldsRoadmap = []; }
          inLinkFieldsReq = false; inLinkFieldsAdr = false; inLinkFieldsRoadmap = false;
          if (isInlineList(subVal)) {
            const items = parseInlineList(subVal);
            if (subKey === 'req') cfg.linkFields.req = items;
            else if (subKey === 'adr') cfg.linkFields.adr = items;
            else if (subKey === 'roadmap') cfg.linkFields.roadmap = items;
          } else {
            if (subKey === 'req') inLinkFieldsReq = true;
            else if (subKey === 'adr') inLinkFieldsAdr = true;
            else if (subKey === 'roadmap') inLinkFieldsRoadmap = true;
          }
        }
        continue;
      }
      continue;
    }

    // linha top-level
    const colonIdx = line.indexOf(':');
    if (colonIdx < 0) continue;
    const key = line.slice(0, colonIdx).trim();
    const val = line.slice(colonIdx + 1).trim();
    if (!key) continue;

    switch (key) {
      case 'adr_dirs':
        if (isInlineList(val)) { cfg.adrDirs = parseInlineList(val).map(expandPath); }
        else { inAdrDirs = true; adrDirs = []; }
        break;
      case 'req_dir':               cfg.reqDir = expandPath(val.replace(/^["']|["']$/g, '')); break;
      case 'roadmap_dir':           cfg.roadmapDir = expandPath(val.replace(/^["']|["']$/g, '')); break;
      case 'roadmap_namespacing':   cfg.roadmapNamespacing = val; break;
      case 'agents':
        if (isInlineList(val)) { cfg.agents = parseInlineList(val); }
        else { inAgents = true; agents = []; }
        break;
      case 'governance_mode':       cfg.governanceMode = val; break;
      case 'lenient_until':         cfg.lenientUntil = val; break;
      case 'wip_limit':             { const n = parseInt(val, 10); if (n > 0) cfg.wipLimit = n; break; }
      case 'wip_by_squad':          cfg.wipBySquad = val === 'true'; break;
      case 'stale_wip_days':        { const n = parseInt(val, 10); if (n > 0) cfg.staleWipDays = n; break; }
      case 'require_req_in_commit': cfg.requireReqInCommit = val === 'true'; break;
      case 'strict_ci_paths':       cfg.strictCiPaths = val === 'true'; break;
      case 'trace_id_field':        cfg.traceIdField = val.replace(/^["']|["']$/g, ''); break;
      case 'forge':                 cfg.forge = val.replace(/^["']|["']$/g, ''); break;
      case 'link_fields':           inLinkFields = true; break;
      case 'acceptance_markers':
        if (isInlineList(val)) { cfg.acceptanceMarkers = parseInlineList(val); }
        else { inAcceptanceMarkers = true; acceptanceMarkers = []; }
        break;
      case 'rules':                 inRules = true; rules = {}; break;
    }
  }

  // flush final (EOF)
  flushBlocks();
}

const NAMESPACING_FLAT = 'flat';
const NAMESPACING_BY_AGENT = 'by_agent';

module.exports = { load, reset, defaults, expandPath, NAMESPACING_FLAT, NAMESPACING_BY_AGENT };
