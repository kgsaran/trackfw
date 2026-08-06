'use strict';

const { Command } = require('commander');
const fs = require('fs');
const os = require('os');
const path = require('path');
const identityStore = require('../identity');
const projectConfig = require('../config');
const { runFileTarget, validateTargets, buildDocument, humanReport, silenceConsole } = require('../lib/update-engine');

// `trackfw update` is project-scoped only — see docs/cli-parity.md,
// "`trackfw update` vs `trackfw update harness`". It must NEVER touch global
// state (~/.claude, ~/.codex, etc.). Global artifacts moved to
// `trackfw update harness` (update-harness.js).

// loadUpdateConfig replaces the former artisanal line-by-line scanner (readUpdateConfig) with
// the single config loader (../config, see ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-
// yaml-com-namespaces-tipados.md). Returns the same snake_case shape the rest of this file
// already consumes (cfg.hooks, cfg.ci, cfg.backend, cfg.frontend, cfg.pkg_manager) so downstream
// code (updateHooksSurgical, buildProjectTargets) needs no further change.
function loadUpdateConfig(rootDir) {
  const u = projectConfig.load(rootDir).update;
  return {
    hooks: u.hooks,
    ci: u.ci,
    backend: u.backend,
    frontend: u.frontend,
    pkg_manager: u.pkgManager,
  };
}

function updateHooksSurgical(cfg, rootDir) {
  const hooks = cfg.hooks || '';
  if (hooks === 'husky') {
    const hookPath = path.join(rootDir, '.husky', 'pre-commit');
    const content = fs.existsSync(hookPath) ? fs.readFileSync(hookPath, 'utf8') : '';
    if (content.includes('trackfw validate')) {
      console.log('  ✓ .husky/pre-commit — trackfw validate já presente');
    } else {
      fs.mkdirSync(path.join(rootDir, '.husky'), { recursive: true });
      fs.appendFileSync(hookPath, '\ntrackfw validate\n', 'utf8');
      try { fs.chmodSync(hookPath, 0o755); } catch (_) {}
      console.log('  ✓ .husky/pre-commit — trackfw validate injetado');
    }
  } else if (hooks === 'lefthook') {
    const lefthookPath = path.join(rootDir, 'lefthook.yml');
    const content = fs.existsSync(lefthookPath) ? fs.readFileSync(lefthookPath, 'utf8') : '';
    if (content.includes('trackfw-validate:') || content.includes('trackfw validate')) {
      console.log('  ✓ lefthook.yml — trackfw já presente');
    } else {
      fs.appendFileSync(lefthookPath, '\npre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n', 'utf8');
      console.log('  ✓ lefthook.yml — trackfw-validate injetado');
    }
  }
}

// codexProjectAgentsTarget — installs/reports the project-scoped Codex
// agents/skills bundle (catalog-based). Uses IntegrationManager.inspect,
// which is read-only, to compute state under --dry-run too; only the
// mutating manager.update() call is skipped when dryRun is true. Kept
// separate from runFileTarget because IntegrationManager already owns a
// correct, manifest-aware not-installed/current/outdated/modified state
// machine — re-deriving it via directory hashing would risk diverging from
// that source of truth (and would need the .trackfw manifest copied into
// the simulation, which is unnecessary complexity here).
function codexProjectAgentsTarget(cwd, identityConfig, { dryRun, installMissing }) {
  const id = 'codex-project-agents';
  const displayPath = '.codex/agents, .agents/skills';
  const detected = fs.existsSync(path.join(cwd, 'AGENTS.md')) || fs.existsSync(path.join(cwd, '.codex'));
  if (!detected) return { id, state: 'missing', path: displayPath };

  try {
    const { buildPlans, IntegrationManager } = require('../integrations');
    const manager = new IntegrationManager({ projectRoot: cwd });
    let wroteAny = false;
    for (const kind of ['agents', 'skills']) {
      const plans = buildPlans(kind, { targets: ['codex'], scope: 'project', identity: identityConfig });
      const statuses = manager.inspect(plans);
      const toWrite = plans.filter((_, index) => {
        const state = statuses[index].state;
        return state === 'outdated' || (installMissing && state === 'not-installed');
      });
      if (toWrite.length) {
        wroteAny = true;
        if (!dryRun) manager.update(toWrite);
      }
    }
    return { id, state: wroteAny ? 'updated' : 'skipped', path: displayPath };
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message };
  }
}

// PROJECT_TARGET_IDS — the fixed declared order for `trackfw update`
// targets. `ci-workflow` and `git-hooks` only appear when the project's
// trackfw.yaml opted into a CI system / hook framework — see ambiguity
// note in the ML-6C handoff report about config-conditional target lists.
const PROJECT_TARGET_IDS = [
  'agent-rules',
  'agent-hooks',
  'codex-project-agents',
  'validate-script',
  'ci-workflow',
  'git-hooks',
  'claude-commands',
];

// buildProjectTargets — `wanted` (nullable) restricts which targets are
// even computed/applied. This must be enforced HERE, before any apply()
// runs, not as a post-hoc filter on the returned array: every target's
// apply() is a real filesystem side effect (outside --dry-run), so
// filtering afterwards would still have written every unrequested target.
function buildProjectTargets(cwd, cfg, identityConfig, { dryRun, installMissing }, wanted) {
  const generators = require('../generators/init');
  const discover = require('./discover');
  const hooksGen = require('../generators/hooks');
  const include = (id) => !wanted || wanted.includes(id);

  const targets = [];

  if (include('agent-rules')) targets.push(runFileTarget({
    id: 'agent-rules',
    path: 'CLAUDE.md, AGENTS.md, GEMINI.md, .github/copilot-instructions.md, .windsurfrules, .amazonq/developer/guidelines.md, .cursor/rules/trackfw.mdc',
    root: cwd,
    relPaths: ['CLAUDE.md', 'AGENTS.md', 'GEMINI.md', '.github/copilot-instructions.md', '.windsurfrules', '.amazonq/developer/guidelines.md', '.cursor/rules/trackfw.mdc'],
    apply: (root) => generators.injectRulesDetected(root),
    dryRun,
    installMissing,
  }));

  if (include('agent-hooks')) targets.push(runFileTarget({
    id: 'agent-hooks',
    path: '.claude/settings.json, .codex/hooks.json, .gemini/settings.json, .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, scripts/trackfw-attention-*.sh, scripts/trackfw-credential-guard.sh',
    root: cwd,
    relPaths: [
      '.claude/settings.json',
      '.codex/hooks.json',
      '.gemini/settings.json',
      '.kiro/hooks/trackfw-attention.json',
      '.github/hooks/trackfw-attention.json',
      '.cursor/hooks.json',
      'scripts/trackfw-attention-signal.sh',
      'scripts/trackfw-attention-cleanup.sh',
      'scripts/trackfw-credential-guard.sh',
    ],
    apply: (root) => {
      hooksGen.injectHooksDetected(root);
      hooksGen.generateAttentionScripts(cfg, root);
      hooksGen.generateCredentialGuardScript(root);
    },
    dryRun,
    installMissing,
  }));

  if (include('codex-project-agents')) targets.push(codexProjectAgentsTarget(cwd, identityConfig, { dryRun, installMissing }));

  if (include('validate-script')) targets.push(runFileTarget({
    id: 'validate-script',
    path: 'scripts/trackfw-validate.sh',
    root: cwd,
    relPaths: ['scripts/trackfw-validate.sh'],
    // Reuses generators/init.js's generateValidateScript — the SAME
    // generator `trackfw init` uses to write this file — not
    // discover.js's writeValidateScript, which produces a different
    // (simpler, non-per-backend) script and made every `update` re-run
    // report "updated" against a project actually already current
    // (ML-6H fix). loadUpdateConfig returns raw trackfw.yaml keys
    // (snake_case, e.g. "pkg_manager"); buildValidateScript expects the
    // camelCase shape used by the rest of the init generators.
    apply: (root) => generators.generateValidateScript({
      backend: cfg.backend,
      frontend: cfg.frontend,
      pkgManager: cfg.pkg_manager,
    }, root),
    dryRun,
    installMissing,
  }));

  if (include('ci-workflow') && (cfg.ci === 'github-actions' || cfg.ci === 'github_actions')) {
    targets.push(runFileTarget({
      id: 'ci-workflow',
      path: '.github/workflows/trackfw-validate.yml',
      root: cwd,
      relPaths: ['.github/workflows/trackfw-validate.yml'],
      apply: (root) => discover.writeCIWorkflowForce(root),
      dryRun,
      installMissing,
    }));
  }

  if (include('git-hooks') && (cfg.hooks === 'husky' || cfg.hooks === 'lefthook')) {
    const relPath = cfg.hooks === 'husky' ? '.husky/pre-commit' : 'lefthook.yml';
    targets.push(runFileTarget({
      id: 'git-hooks',
      path: relPath,
      root: cwd,
      relPaths: [relPath],
      apply: (root) => updateHooksSurgical(cfg, root),
      dryRun,
      installMissing,
    }));
  }

  if (include('claude-commands')) targets.push(runFileTarget({
    id: 'claude-commands',
    path: '.claude/commands/trackfw',
    root: cwd,
    relPaths: ['.claude/commands/trackfw'],
    apply: (root) => generators.generateClaudeCommandsForce(root),
    dryRun,
    installMissing,
  }));

  return targets;
}

const cmd = new Command('update');
cmd.description('Update trackfw-managed artifacts. Bare form is project-scoped (never touches global state); `update harness` updates the global harness instead.');
// `[mode]` is a plain positional argument, not a nested commander.Command —
// see the long comment in update-harness.js:run for why: a real subcommand
// redeclaring the same --json/--dry-run/--targets/--install-missing flags
// as this parent silently drops them (commander@12 quirk, confirmed by
// reproduction). One Command, one Option per flag, branch on `mode` inside
// a single action — this is the only structure that parses correctly.
cmd.argument('[mode]', 'Pass "harness" to update the global harness instead of the current project');
cmd.option('--dry-run', 'Compute and report states without writing anything');
cmd.option('--json', 'Emit the result document as JSON');
cmd.option('--targets <ids>', 'Comma-separated subset of target ids');
cmd.option('--install-missing', 'Allow missing targets to be installed');
cmd.action((mode, options) => {
  if (mode === 'harness') {
    return require('./update-harness').run(options);
  }
  if (mode) {
    console.error(`✗ Unknown update mode: ${mode} (expected "harness" or no argument)`);
    process.exit(1);
  }

  const cwd = process.cwd();
  const yaml = path.join(cwd, 'trackfw.yaml');
  if (!fs.existsSync(yaml)) {
    console.error('✗ trackfw.yaml não encontrado — execute trackfw init primeiro');
    process.exit(1);
  }

  let wanted;
  try {
    const requested = options.targets ? String(options.targets).split(',').map((s) => s.trim()).filter(Boolean) : [];
    wanted = validateTargets(PROJECT_TARGET_IDS, requested);
  } catch (e) {
    console.error(`✗ ${e.message}`);
    process.exit(1);
  }

  const cfg = loadUpdateConfig(cwd);
  const dryRun = Boolean(options.dryRun);
  const installMissing = Boolean(options.installMissing);

  // A identidade é carregada uma única vez, fora de qualquer try/catch —
  // um identity.json corrompido deve abortar o comando inteiro, nunca cair
  // silenciosamente para os nomes neutros default.
  const identityConfig = identityStore.load(os.homedir());

  // With --json, stdout must carry only the result document — apply()
  // functions log human progress lines as a side effect; silence them so a
  // consumer parsing --json output never has to skip preamble noise.
  const targets = options.json
    ? silenceConsole(() => buildProjectTargets(cwd, cfg, identityConfig, { dryRun, installMissing }, wanted))
    : buildProjectTargets(cwd, cfg, identityConfig, { dryRun, installMissing }, wanted);

  const doc = buildDocument('project', dryRun, targets);

  if (options.json) {
    console.log(JSON.stringify(doc, null, 2));
  } else {
    console.log(humanReport('project', dryRun, targets));
  }

  if (doc.summary.failed > 0) process.exitCode = 1;
});

module.exports = cmd;
