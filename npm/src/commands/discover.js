'use strict';

const { Command } = require('commander');
const fs = require('fs');
const path = require('path');

function scan(rootDir) {
  const r = {
    adrDirs: [],
    reqDir: '',
    roadmapDir: '',
    roadmapNamespacing: 'flat',
    agents: [],
    adrCount: 0,
    reqCount: 0,
    roadmapCount: 0,
    hasTrackfwYAML: false,
    hasTrackfwLog: false,
    governanceScore: 0,
  };

  // trackfw.yaml
  r.hasTrackfwYAML = fs.existsSync(path.join(rootDir, 'trackfw.yaml'));

  // REQ dir
  for (const candidate of ['docs/req', 'docs/requisições', 'docs/requirements', 'docs/reqs']) {
    const full = path.join(rootDir, candidate);
    if (isDir(full)) {
      r.reqDir = candidate;
      r.reqCount = countMD(full);
      break;
    }
  }

  // ADR dirs
  const adrRoot = path.join(rootDir, 'docs', 'adr');
  if (isDir(adrRoot)) {
    const subDirs = listSubDirs(adrRoot);
    if (subDirs.length > 0) {
      for (const sub of subDirs) {
        const rel = 'docs/adr/' + sub;
        r.adrDirs.push(rel);
        r.adrCount += countMD(path.join(rootDir, rel));
      }
    } else {
      r.adrDirs = ['docs/adr'];
      r.adrCount = countMD(adrRoot);
    }
  }

  // Roadmap dir e namespacing
  const roadmapRoot = path.join(rootDir, 'docs', 'roadmaps');
  if (isDir(roadmapRoot)) {
    r.roadmapDir = 'docs/roadmaps';
    const agentDirs = listSubDirs(roadmapRoot);
    let byAgent = false;
    const agents = [];
    for (const sub of agentDirs) {
      const wipDir = path.join(roadmapRoot, sub, 'wip');
      const backlogDir = path.join(roadmapRoot, sub, 'backlog');
      const doneDir = path.join(roadmapRoot, sub, 'done');
      const abandonedDir = path.join(roadmapRoot, sub, 'abandoned');
      const blockedDir = path.join(roadmapRoot, sub, 'blocked');
      if (isDir(wipDir) || isDir(backlogDir) || isDir(doneDir) || isDir(abandonedDir) || isDir(blockedDir)) {
        byAgent = true;
        agents.push(sub);
      }
    }
    if (byAgent) {
      r.roadmapNamespacing = 'by_agent';
      r.agents = agents;
      for (const agent of agents) {
        for (const state of ['backlog', 'wip', 'blocked', 'done', 'abandoned']) {
          r.roadmapCount += countMD(path.join(roadmapRoot, agent, state));
        }
      }
    } else {
      r.roadmapNamespacing = 'flat';
      for (const state of ['backlog', 'wip', 'blocked', 'done', 'abandoned']) {
        r.roadmapCount += countMD(path.join(roadmapRoot, state));
      }
    }

    r.hasTrackfwLog = fs.existsSync(path.join(roadmapRoot, '.trackfw-log'));
  }

  r.governanceScore = calcScore(r);
  return r;
}

function calcScore(r) {
  let score = 0;
  if (r.adrCount > 0) score += 20;
  if (r.reqCount > 0) score += 20;
  if (r.roadmapCount > 0) score += 20;
  if (r.hasTrackfwYAML) score += 20;
  if (r.hasTrackfwLog) score += 20;
  return score;
}

function generateYAML(r) {
  let out = '# trackfw configuration — gerado por trackfw discover\n';
  out += '# governance_mode: lenient permite validação não-bloqueante durante onboarding\n\n';
  out += 'governance_mode: lenient\n\n';

  if (r.adrDirs.length > 0) {
    out += 'adr_dirs:\n';
    r.adrDirs.forEach(d => { out += `  - ${d}\n`; });
  } else {
    out += 'adr_dirs:\n  - docs/adr\n';
  }

  out += `req_dir: ${r.reqDir || 'docs/req'}\n`;
  out += `roadmap_dir: ${r.roadmapDir || 'docs/roadmaps'}\n`;
  out += `roadmap_namespacing: ${r.roadmapNamespacing}\n`;

  if (r.agents.length > 0) {
    out += 'agents:\n';
    r.agents.forEach(a => { out += `  - ${a}\n`; });
  }

  return out;
}

function generateBootstrapLog(r, rootDir) {
  let out = '';
  const roadmapRoot = path.join(rootDir, r.roadmapDir);

  const appendEntries = (dir, agent) => {
    if (!isDir(dir)) return;
    for (const entry of fs.readdirSync(dir)) {
      if (!entry.endsWith('.md')) continue;
      const filePath = path.join(dir, entry);
      const stat = fs.statSync(filePath);
      const ts = stat.mtime.toISOString().slice(0, 16).replace('T', ' ');
      const basename = agent ? agent + '/' + entry : entry;
      out += `${ts}  ${basename.padEnd(50)}  backlog → done\n`;
    }
  };

  if (r.roadmapNamespacing === 'by_agent') {
    for (const agent of r.agents) {
      appendEntries(path.join(roadmapRoot, agent, 'done'), agent);
    }
  } else {
    appendEntries(path.join(roadmapRoot, 'done'), '');
  }

  return out;
}

// helpers
function isDir(p) {
  try { return fs.statSync(p).isDirectory(); } catch { return false; }
}

function countMD(dir) {
  let n = 0;
  function walk(d) {
    let entries;
    try { entries = fs.readdirSync(d, { withFileTypes: true }); } catch { return; }
    for (const e of entries) {
      if (e.isDirectory()) walk(path.join(d, e.name));
      else if (e.name.endsWith('.md')) n++;
    }
  }
  walk(dir);
  return n;
}

function listSubDirs(dir) {
  try {
    return fs.readdirSync(dir).filter(f => {
      try { return fs.statSync(path.join(dir, f)).isDirectory(); } catch { return false; }
    });
  } catch { return []; }
}

const cmd = new Command('discover');
cmd.description('Scan the repository and auto-detect the governance structure');
cmd.option('--init', 'generate trackfw.yaml calibrated for this project');
cmd.option('--bootstrap-log', 'create retroactive .trackfw-log from done/ files');
cmd.action((opts) => {
  const cwd = process.cwd();
  console.log(`trackfw discover — scanning ${cwd}\n`);

  const r = scan(cwd);

  // ADR dirs
  if (r.adrCount > 0) {
    const dirs = r.adrDirs.join(', ');
    console.log(`✓ ADRs found:      ${String(r.adrCount).padEnd(4)}  (${dirs})`);
  } else {
    console.log('⚠ No ADRs found');
  }

  // REQ dir
  if (r.reqCount > 0) {
    console.log(`✓ REQs found:      ${String(r.reqCount).padEnd(4)}  (${r.reqDir})`);
  } else {
    console.log('⚠ No REQs found');
  }

  // Roadmaps
  if (r.roadmapCount > 0) {
    const mode = r.roadmapNamespacing === 'by_agent' ? 'by_agent mode' : r.roadmapNamespacing;
    console.log(`✓ Roadmaps found:  ${String(r.roadmapCount).padEnd(4)}  (${r.roadmapDir} — ${mode})`);
  } else {
    console.log('⚠ No roadmaps found');
  }

  // Agents
  if (r.agents.length > 0) {
    console.log(`✓ Agents detected: ${r.agents.join(', ')}`);
  }

  // trackfw.yaml
  if (!r.hasTrackfwYAML) {
    console.log('⚠ No trackfw.yaml — run with --init to generate one');
  } else {
    console.log('✓ trackfw.yaml found');
  }

  // .trackfw-log
  if (!r.hasTrackfwLog) {
    console.log('⚠ No .trackfw-log — run with --bootstrap-log to create retroactive history');
  } else {
    console.log('✓ .trackfw-log found');
  }

  console.log(`\nGovernance Score: ${r.governanceScore}/100`);

  if (opts.init) {
    const yaml = generateYAML(r);
    fs.writeFileSync('trackfw.yaml', yaml, 'utf8');
    console.log('\n✓ trackfw.yaml generated');
  }

  if (opts.bootstrapLog) {
    if (!r.roadmapDir) {
      console.error('⚠ No roadmap dir detected — cannot bootstrap log');
      return;
    }
    const logContent = generateBootstrapLog(r, cwd);
    const logPath = r.roadmapDir + '/.trackfw-log';
    fs.appendFileSync(logPath, logContent, 'utf8');
    console.log(`✓ bootstrap log written to ${logPath}`);
  }
});

module.exports = cmd;
module.exports.scan = scan;
module.exports.generateYAML = generateYAML;
module.exports.generateBootstrapLog = generateBootstrapLog;
