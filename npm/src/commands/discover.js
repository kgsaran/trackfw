'use strict';

const { Command } = require('commander');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const { resolve: resolveForge } = require('../forge/resolve');

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
    hookFramework: 'none',
    ciSystem: 'none',
    forge: '',
    // suggestedTestFramework é uma sugestão best-effort (nunca erro, '' quando nada bate) baseada
    // em arquivos de configuração presentes na raiz do projeto. É apenas impressa como sugestão
    // pelo comando `discover` — nunca escrita automaticamente em trackfw.yaml (a convenção de
    // agent_conventions deve ser sempre declarada pelo time, não inferida).
    suggestedTestFramework: '',
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
      const analyzingDir = path.join(roadmapRoot, sub, 'analyzing');
      const doneDir = path.join(roadmapRoot, sub, 'done');
      const abandonedDir = path.join(roadmapRoot, sub, 'abandoned');
      const blockedDir = path.join(roadmapRoot, sub, 'blocked');
      if (isDir(wipDir) || isDir(backlogDir) || isDir(analyzingDir) || isDir(doneDir) || isDir(abandonedDir) || isDir(blockedDir)) {
        byAgent = true;
        agents.push(sub);
      }
    }
    if (byAgent) {
      r.roadmapNamespacing = 'by_agent';
      r.agents = agents;
      for (const agent of agents) {
        for (const state of ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']) {
          r.roadmapCount += countMD(path.join(roadmapRoot, agent, state));
        }
      }
    } else {
      r.roadmapNamespacing = 'flat';
      for (const state of ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']) {
        r.roadmapCount += countMD(path.join(roadmapRoot, state));
      }
    }

    r.hasTrackfwLog = fs.existsSync(path.join(roadmapRoot, '.trackfw-log'));
  }

  // Hook framework
  if (isFile(path.join(rootDir, 'lefthook.yml')) || isFile(path.join(rootDir, '.lefthook.yml'))) {
    r.hookFramework = 'lefthook';
  } else if (isDir(path.join(rootDir, '.husky'))) {
    r.hookFramework = 'husky';
  } else if (isFile(path.join(rootDir, '.pre-commit-config.yaml'))) {
    r.hookFramework = 'pre-commit';
  } else {
    r.hookFramework = 'none';
  }

  // CI
  if (isDir(path.join(rootDir, '.github', 'workflows'))) {
    r.ciSystem = 'github-actions';
  } else if (isFile(path.join(rootDir, '.gitlab-ci.yml'))) {
    r.ciSystem = 'gitlab';
  } else {
    r.ciSystem = 'none';
  }

  // Suggested test framework — best-effort heuristic, never an error.
  r.suggestedTestFramework = detectTestFramework(rootDir);

  // Forge detection — reuse forge/resolve.js (no duplicate parse).
  // gitRemoteURL returns '' on any error; CI detection is filesystem-based.
  try {
    const { resolveFromRepo } = require('../forge/resolve');
    const res = resolveFromRepo('', '', rootDir);
    if (res.source !== 'none') {
      r.forge = res.forge;
    }
  } catch (_) {
    // forge detection is best-effort; never block scan on it
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

  out += `hooks: ${r.hookFramework}\n`;
  out += `ci: ${r.ciSystem}\n`;

  if (r.forge) {
    out += `forge: ${r.forge}\n`;
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

// installGates instala artefatos de governança: validate script, hook entry, CI workflow.
function installGates(r, rootDir) {
  writeValidateScript(rootDir);
  installHook(r.hookFramework, rootDir);
  if (r.ciSystem === 'github-actions') {
    writeCIWorkflow(rootDir);
  }
}

function writeValidateScript(rootDir) {
  const scriptsDir = path.join(rootDir, 'scripts');
  if (!isDir(scriptsDir)) fs.mkdirSync(scriptsDir, { recursive: true });
  const content = '#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n';
  const dest = path.join(scriptsDir, 'trackfw-validate.sh');
  fs.writeFileSync(dest, content, { mode: 0o755 });
}

function installHook(framework, rootDir) {
  const hookEntry = '\npre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n';
  const huskyEntry = '\nscripts/trackfw-validate.sh\n';

  if (framework === 'lefthook') {
    let cfgPath = path.join(rootDir, 'lefthook.yml');
    if (!isFile(cfgPath)) cfgPath = path.join(rootDir, '.lefthook.yml');
    const content = fs.readFileSync(cfgPath, 'utf8');
    if (content.includes('trackfw')) return; // idempotente
    fs.appendFileSync(cfgPath, hookEntry, 'utf8');
  } else if (framework === 'husky') {
    const huskyHook = path.join(rootDir, '.husky', 'pre-commit');
    fs.appendFileSync(huskyHook, huskyEntry, 'utf8');
  } else {
    const pkgJson = path.join(rootDir, 'package.json');
    if (fs.existsSync(pkgJson)) {
      installHusky(rootDir);
    } else {
      // Node.js disponível mas sem package.json → husky via npx
      try {
        execSync('node --version', { stdio: 'pipe' });
        installHuskyNPX(rootDir);
        return;
      } catch (_) {}
      // fallback: lefthook
      installLefthook(rootDir);
    }
  }
}

function installHuskyNPX(rootDir) {
  console.log('ℹ node detected — using husky via npx (no package.json required)');
  try {
    execSync('npx husky init', { cwd: rootDir, stdio: 'inherit' });
  } catch (_) {
    console.log('⚠ npx husky init failed — install manually: npx husky init');
  }
  const huskyDir = path.join(rootDir, '.husky');
  fs.mkdirSync(huskyDir, { recursive: true });
  const hookPath = path.join(huskyDir, 'pre-commit');
  fs.appendFileSync(hookPath, '\nscripts/trackfw-validate.sh\n', { mode: 0o755 });
  console.log('✓ trackfw entry added to .husky/pre-commit');
}

function installHusky(rootDir) {
  try {
    execSync('npm install --save-dev husky', { cwd: rootDir, stdio: 'inherit' });
  } catch (e) {
    console.warn('⚠ trackfw: falha ao instalar husky:', e.message);
    return;
  }
  try {
    execSync('npx husky init', { cwd: rootDir, stdio: 'inherit' });
  } catch (e) {
    console.warn('⚠ trackfw: falha ao inicializar husky:', e.message);
    return;
  }
  try {
    const huskyDir = path.join(rootDir, '.husky');
    if (!isDir(huskyDir)) fs.mkdirSync(huskyDir, { recursive: true });
    fs.appendFileSync(path.join(huskyDir, 'pre-commit'), '\nscripts/trackfw-validate.sh\n', 'utf8');
  } catch (e) {
    console.warn('⚠ trackfw: falha ao configurar hook pre-commit do husky:', e.message);
  }
}

function installLefthook(rootDir) {
  const lefthookPath = path.join(rootDir, 'lefthook.yml');
  if (isFile(lefthookPath)) {
    const existing = fs.readFileSync(lefthookPath, 'utf8');
    if (existing.includes('trackfw')) return; // idempotente
    fs.appendFileSync(lefthookPath, '\npre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n', 'utf8');
  } else {
    const content = 'pre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n';
    fs.writeFileSync(lefthookPath, content, 'utf8');
  }
  try {
    execSync('lefthook install', { cwd: rootDir, stdio: 'inherit' });
  } catch (e) {
    console.warn('⚠ trackfw: lefthook não encontrado no PATH — hook registrado em lefthook.yml mas não instalado:', e.message);
  }
}

// DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH is the canonical relative path of the second,
// independent CI workflow trackfw writes — the one `trackfw discover --init` generates
// via installGates, distinct from GITHUB_ACTIONS_WORKFLOW_PATH (trackfw-gate.yml,
// written by init/update). Both files can coexist in the same project
// (ADR-2026-08-28). Exported so scaffold_doctor.js can compare against it by path.
const DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH = '.github/workflows/trackfw-validate.yml';

// buildDiscoverGitHubActionsWorkflowContent returns the template content trackfw writes
// to DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH. Mirrors
// generators.BuildDiscoverGitHubActionsWorkflowContent in
// internal/generators/scaffold_doctor.go (Go, canonical source of truth) byte-for-byte
// for the same version.
//
// NOT version-independent (ADR-2026-08-28, REQ-2026-08-28 AC6/AC7): the `go install
// .../cmd/trackfw@vX.Y.Z` step pins the second install mechanism (`go install ...@latest`)
// to the version of npm/package.json — the version of the CLI that generated/updated the
// project — mirroring the install.sh pin already applied to
// buildGitHubActionsWorkflowContent (trackfw-gate.yml) in generators/init.js. Never
// hardcoded: read lazily to avoid a circular require at module load time.
function buildDiscoverGitHubActionsWorkflowContent() {
  const { version } = require('../../package.json');
  return `name: trackfw validate
on: [push, pull_request]
jobs:
  governance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v${version}
      - run: trackfw validate
`;
}

function writeCIWorkflow(rootDir) {
  const workflowsDir = path.join(rootDir, '.github', 'workflows');
  if (!isDir(workflowsDir)) fs.mkdirSync(workflowsDir, { recursive: true });
  const dest = path.join(workflowsDir, 'trackfw-validate.yml');
  // Uses fs.lstatSync directly, NOT the isFile helper (fs.statSync, follows
  // symlinks): a DANGLING symlink at dest resolves to "does not exist" under
  // fs.statSync, so the idempotency guard below would not fire, and
  // fs.writeFileSync would then follow the link and CREATE the workflow
  // template at whatever path outside the project the symlink points to. A
  // symlink here — live or dangling — is treated as "already present" so
  // this function never writes through it; it refuses loudly instead of
  // silently creating a file somewhere the caller never asked for.
  let info
  try {
    info = fs.lstatSync(dest)
  } catch (e) {
    info = null
  }
  if (info) {
    if (info.isSymbolicLink()) {
      console.error(`aviso: ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} é um symlink; trackfw discover não escreve através de symlinks — arquivo não foi tocado`)
    }
    return; // idempotente — não sobrescreve (nem segue o link)
  }
  const content = buildDiscoverGitHubActionsWorkflowContent();
  fs.writeFileSync(dest, content, 'utf8');
}

// helpers
function isDir(p) {
  try { return fs.statSync(p).isDirectory(); } catch { return false; }
}

function isFile(p) {
  try { return fs.statSync(p).isFile(); } catch { return false; }
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

// detectTestFramework é uma heurística best-effort para sugerir um framework de teste com base
// em arquivos de configuração presentes na raiz do projeto. Nunca retorna erro — retorna '' quando
// nenhum arquivo-gatilho é encontrado. Ordem de precedência: jest, vitest, pytest, go test.
function detectTestFramework(rootDir) {
  if (isFile(path.join(rootDir, 'jest.config.js')) || isFile(path.join(rootDir, 'jest.config.ts'))) {
    return 'jest';
  }
  if (isFile(path.join(rootDir, 'vitest.config.js')) || isFile(path.join(rootDir, 'vitest.config.ts'))) {
    return 'vitest';
  }
  if (isFile(path.join(rootDir, 'pytest.ini'))) {
    return 'pytest';
  }
  if (hasFileWithSubstring(path.join(rootDir, 'pyproject.toml'), '[tool.pytest')) {
    return 'pytest';
  }
  if (hasFileWithSubstring(path.join(rootDir, 'setup.cfg'), '[tool:pytest]')) {
    return 'pytest';
  }
  if (isFile(path.join(rootDir, 'go.mod')) && hasGoTestFile(rootDir)) {
    return 'go test';
  }
  return '';
}

// hasFileWithSubstring lê path e retorna true se seu conteúdo contém sub. Retorna false
// silenciosamente se o arquivo não existir ou não puder ser lido (best-effort).
function hasFileWithSubstring(p, sub) {
  let content;
  try { content = fs.readFileSync(p, 'utf8'); } catch { return false; }
  return content.includes(sub);
}

// hasGoTestFile percorre rootDir recursivamente procurando qualquer arquivo *_test.go.
function hasGoTestFile(rootDir) {
  let found = false;
  function walk(d) {
    if (found) return;
    let entries;
    try { entries = fs.readdirSync(d, { withFileTypes: true }); } catch { return; }
    for (const e of entries) {
      if (found) return;
      if (e.isDirectory()) {
        walk(path.join(d, e.name));
      } else if (e.name.endsWith('_test.go')) {
        found = true;
        return;
      }
    }
  }
  walk(rootDir);
  return found;
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

  // hooks
  if (r.hookFramework !== 'none') {
    console.log(`✓ Hooks: ${r.hookFramework}`);
  } else {
    console.log('⚠ No hook framework detected');
  }

  // CI
  if (r.ciSystem !== 'none') {
    console.log(`✓ CI: ${r.ciSystem}`);
  } else {
    console.log('⚠ No CI system detected');
  }

  // suggested test framework — printed only, never written to trackfw.yaml automatically
  // (agent_conventions must always be declared by the team).
  if (r.suggestedTestFramework) {
    console.log(`Suggested test framework: ${r.suggestedTestFramework} (add to trackfw.yaml as agent_conventions: if correct)`);
  }

  console.log(`\nGovernance Score: ${r.governanceScore}/100`);

  if (opts.init) {
    const yamlPath = path.join(cwd, 'trackfw.yaml');
    if (fs.existsSync(yamlPath)) {
      console.log('\n⚠ trackfw.yaml already exists — skipping (remove it first to regenerate)');
    } else {
      const yaml = generateYAML(r);
      fs.writeFileSync(yamlPath, yaml, 'utf8');
      console.log('\n✓ trackfw.yaml generated');
      try {
        installGates(r, cwd);
        console.log('✓ governance gates installed');
      } catch (e) {
        console.log(`⚠ gates install partial: ${e.message}`);
      }
      try {
        const generators = require('../generators/init');
        generators.injectRulesDetected(cwd);
        console.log('✓ trackfw rules injected into agent config files');
      } catch (e) {
        console.log(`⚠ agent rules inject partial: ${e.message}`);
      }
      try {
        const { generateAttentionScripts } = require('../generators/hooks');
        generateAttentionScripts({}, cwd);
      } catch (e) {
        console.warn(`⚠ attention scripts: ${e.message}`);
      }
      try {
        const { generateCredentialGuardScript } = require('../generators/hooks');
        generateCredentialGuardScript(cwd);
      } catch (e) {
        console.warn(`⚠ credential guard script: ${e.message}`);
      }
      try {
        const { generateGitBranchGuardScript } = require('../generators/hooks');
        generateGitBranchGuardScript(cwd);
      } catch (e) {
        console.warn(`⚠ git branch guard script: ${e.message}`);
      }
      try {
        const { injectHooksDetected } = require('../generators/hooks');
        injectHooksDetected(cwd);
        console.log('✓ agent hooks injected');
      } catch (e) {
        console.warn(`⚠ agent hooks inject partial: ${e.message}`);
      }
      try {
        const generators = require('../generators/init')
        generators.printArchitectNextSteps(cwd)
      } catch (e) {}
    }
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

function writeCIWorkflowForce(rootDir) {
  const workflowsDir = path.join(rootDir, '.github', 'workflows');
  if (!isDir(workflowsDir)) fs.mkdirSync(workflowsDir, { recursive: true });
  const dest = path.join(workflowsDir, 'trackfw-validate.yml');
  // Same symlink guard as writeCIWorkflow above: "force" means "overwrite
  // an existing REGULAR file unconditionally", not "follow whatever this
  // path resolves to". A symlink here — live or dangling — is refused, not
  // followed.
  try {
    const info = fs.lstatSync(dest)
    if (info.isSymbolicLink()) {
      console.error(`aviso: ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} é um symlink; trackfw discover não escreve através de symlinks — arquivo não foi tocado`)
      return;
    }
  } catch (e) {
    // does not exist — fine, this is the "force create" case
  }
  const content = buildDiscoverGitHubActionsWorkflowContent();
  fs.writeFileSync(dest, content, 'utf8');
}

module.exports = cmd;
module.exports.scan = scan;
module.exports.generateYAML = generateYAML;
module.exports.generateBootstrapLog = generateBootstrapLog;
module.exports.writeValidateScript = writeValidateScript;
module.exports.writeCIWorkflow = writeCIWorkflow;
module.exports.writeCIWorkflowForce = writeCIWorkflowForce;
module.exports.detectTestFramework = detectTestFramework;
module.exports.buildDiscoverGitHubActionsWorkflowContent = buildDiscoverGitHubActionsWorkflowContent;
module.exports.DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH = DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH;
