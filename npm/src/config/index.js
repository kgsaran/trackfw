'use strict';

const fs = require('fs');
const path = require('path');

function defaults() {
  return {
    adrDirs: ['docs/adr'],
    reqDir: 'docs/req',
    roadmapDir: 'docs/roadmaps',
    roadmapNamespacing: 'flat',
    agents: [],
    governanceMode: '',
    lenientUntil: '',
    wipLimit: 1,
    wipBySquad: false,
    requireReqInCommit: false,
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
  let inAdrDirs = false;
  let inAgents = false;
  let adrDirs = [];
  let agents = [];

  for (const rawLine of lines) {
    const line = rawLine.trim();

    if (inAdrDirs) {
      if (line.startsWith('- ')) {
        adrDirs.push(line.slice(2).trim());
        continue;
      }
      inAdrDirs = false;
      if (adrDirs.length) cfg.adrDirs = adrDirs;
    }
    if (inAgents) {
      if (line.startsWith('- ')) {
        agents.push(line.slice(2).trim());
        continue;
      }
      inAgents = false;
      if (agents.length) cfg.agents = agents;
    }

    const colonIdx = line.indexOf(':');
    if (colonIdx < 0) continue;
    const key = line.slice(0, colonIdx).trim();
    const val = line.slice(colonIdx + 1).trim();
    if (!key) continue;

    switch (key) {
      case 'adr_dirs': inAdrDirs = true; adrDirs = []; break;
      case 'req_dir': cfg.reqDir = val; break;
      case 'roadmap_dir': cfg.roadmapDir = val; break;
      case 'roadmap_namespacing': cfg.roadmapNamespacing = val; break;
      case 'agents': inAgents = true; agents = []; break;
      case 'governance_mode': cfg.governanceMode = val; break;
      case 'lenient_until': cfg.lenientUntil = val; break;
      case 'wip_limit': { const n = parseInt(val, 10); if (n > 0) cfg.wipLimit = n; break; }
      case 'wip_by_squad': cfg.wipBySquad = val === 'true'; break;
      case 'require_req_in_commit': cfg.requireReqInCommit = val === 'true'; break;
    }
  }

  // flush pending lists at EOF
  if (inAdrDirs && adrDirs.length) cfg.adrDirs = adrDirs;
  if (inAgents && agents.length) cfg.agents = agents;
}

module.exports = { load, reset, defaults };
