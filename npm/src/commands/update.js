'use strict';

const { Command } = require('commander');
const fs = require('fs');
const os = require('os');
const path = require('path');
const identityStore = require('../identity');
const projectConfig = require('../config');
const { runFileTarget, validateTargets, buildDocument, humanReport, silenceConsole } = require('../lib/update-engine');
const { homedir } = require('../homedir')

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

// ensureGlobalAdrDirRegistered — mirrors internal/generators/update.go's
// ensureGlobalADRDirRegistered (Go implementation, same REQ, ML-1A). Registers
// ~/.trackfw/adr in the project's trackfw.yaml `adr_dirs` list, but ONLY when
// that directory exists AND contains at least one `ADR-*.md` file — an empty
// or absent global ADR dir is a no-op, never written. The edit is surgical
// (line-based text splice, not a YAML parse+reserialize) to preserve 100% of
// the user's existing trackfw.yaml content (comments, key order, formatting).
function ensureGlobalAdrDirRegistered(cwd) {
  const home = homedir();
  const globalDir = path.join(home, '.trackfw', 'adr');
  if (!fs.existsSync(globalDir)) return;

  let hasAdr = false;
  try {
    hasAdr = fs.readdirSync(globalDir).some((f) => /^ADR-.*\.md$/.test(f));
  } catch (_) {
    return;
  }
  if (!hasAdr) return;

  const yamlPath = path.join(cwd, 'trackfw.yaml');
  let content;
  try {
    content = fs.readFileSync(yamlPath, 'utf8');
  } catch (_) {
    return;
  }

  const resolvesToGlobal = (entry) => {
    const trimmed = entry.trim();
    if (trimmed === '~/.trackfw/adr') return true;
    const expanded = trimmed.startsWith('~') ? path.join(home, trimmed.slice(1)) : trimmed;
    return path.resolve(expanded) === path.resolve(globalDir);
  };

  const lines = content.split('\n');
  let adrDirsIndex = -1;
  for (let i = 0; i < lines.length; i++) {
    if (/^adr_dirs:\s*$/.test(lines[i])) {
      adrDirsIndex = i;
      break;
    }
  }

  if (adrDirsIndex === -1) {
    // No adr_dirs key at all — the implicit default is `docs/adr`; append a
    // new block preserving that default explicitly, plus the global entry.
    let newContent = content;
    if (newContent.length && !newContent.endsWith('\n')) newContent += '\n';
    newContent += 'adr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n';
    fs.writeFileSync(yamlPath, newContent, 'utf8');
    console.log('  ✓ adr_dirs: ~/.trackfw/adr registrado');
    return;
  }

  const itemRe = /^(\s*)-\s*(.+?)\s*$/;
  let end = adrDirsIndex + 1;
  let lastItemIndex = -1;
  let indent = '  ';
  while (end < lines.length) {
    const m = lines[end].match(itemRe);
    if (!m) break;
    indent = m[1] || indent;
    if (resolvesToGlobal(m[2])) {
      // Already registered under this or an equivalent path form — no-op.
      return;
    }
    lastItemIndex = end;
    end++;
  }

  const insertAt = lastItemIndex === -1 ? adrDirsIndex + 1 : lastItemIndex + 1;
  lines.splice(insertAt, 0, `${indent}- ~/.trackfw/adr`);
  fs.writeFileSync(yamlPath, lines.join('\n'), 'utf8');
  console.log('  ✓ adr_dirs: ~/.trackfw/adr registrado');
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
      const plans = buildPlans(kind, { targets: ['codex'], scope: 'project', identity: identityConfig, agentModels: projectConfig.load(cwd).agentModels || {} });
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

// discoverWorkflowPresent — reports whether
// .github/workflows/trackfw-validate.yml (written by `trackfw discover
// --init`, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH in discover.js) already
// exists under cwd AS A REGULAR FILE. Used both to decide whether
// `ci-workflow` is declared (AC17(c), REQ-2026-08-28) and, inside its
// apply(), to decide whether to refresh it — existence is checked against
// the REAL cwd, never the --dry-run sandbox, mirroring how cfg itself is
// read before the sandbox is built.
//
// Uses fs.lstatSync, NOT fs.existsSync: fs.existsSync follows symlinks, so a
// symlink at this path — even a live one pointing outside the project —
// would report "present" purely because its target resolves, pulling
// `ci-workflow` into the declared target set on the strength of a link this
// command does not own. Symlinks are therefore treated as NOT present here:
// `update` will not declare/manage a target on their account, and
// refreshDiscoverGitHubActionsWorkflowIfPresent below refuses to write
// through them regardless.
function discoverWorkflowPresent(cwd) {
  const { DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH } = require('./discover')
  const dest = path.join(cwd, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
  let info
  try {
    info = fs.lstatSync(dest)
  } catch (e) {
    return false
  }
  return !info.isSymbolicLink()
}

// refreshDiscoverGitHubActionsWorkflowIfPresent — refreshes
// .github/workflows/trackfw-validate.yml ONLY when it already exists under
// root as a REGULAR FILE — `update` never creates this file (AC17(b)):
// ownership of the install decision belongs to `trackfw discover --init`,
// not `update`. Writes the SAME builder scaffold doctor compares against
// (buildDiscoverGitHubActionsWorkflowContent, discover.js) so what `update`
// writes and what `doctor` expects can never drift apart by construction.
//
// Uses fs.lstatSync to decide presence, not fs.existsSync: this path is the
// most sensitive one `update` can write to (it controls what runs in CI for
// anyone who checks the project out), so if it is a symlink — live or
// dangling — this function refuses to write through it. Refusing is loud
// (stderr), never silent, so "update didn't refresh my workflow" stays
// diagnosable instead of a silent no-op.
function refreshDiscoverGitHubActionsWorkflowIfPresent(root) {
  const { DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH, buildDiscoverGitHubActionsWorkflowContent } = require('./discover')
  const dest = path.join(root, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
  let info
  try {
    info = fs.lstatSync(dest)
  } catch (e) {
    return // not installed — update never creates it (AC17(b))
  }
  if (info.isSymbolicLink()) {
    console.error(`aviso: ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} é um symlink; trackfw update não escreve através de symlinks — arquivo não foi tocado`)
    return
  }
  fs.writeFileSync(dest, buildDiscoverGitHubActionsWorkflowContent(), 'utf8')
}

// PROJECT_TARGET_IDS — the fixed declared order for `trackfw update`
// targets. `ci-workflow` and `git-hooks` only appear when the project's
// trackfw.yaml opted into a CI system / hook framework, OR — for
// `ci-workflow` only — when trackfw-validate.yml already exists on disk
// (AC17(c)) — see ambiguity note in the ML-6C handoff report about
// config-conditional target lists. This constant is the full declared
// surface (used to validate --targets); the config/disk-conditional
// inclusion itself lives in buildProjectTargets' `include('ci-workflow')`
// call below.
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
  const hooksGen = require('../generators/hooks');
  const include = (id) => !wanted || wanted.includes(id);

  const targets = [];

  if (include('agent-rules')) targets.push(runFileTarget({
    id: 'agent-rules',
    path: 'CLAUDE.md, AGENTS.md, GEMINI.md, .github/copilot-instructions.md, .windsurfrules, .amazonq/developer/guidelines.md, .cursor/rules/trackfw.mdc',
    root: cwd,
    relPaths: ['CLAUDE.md', 'AGENTS.md', 'GEMINI.md', '.github/copilot-instructions.md', '.windsurfrules', '.amazonq/developer/guidelines.md', '.cursor/rules/trackfw.mdc'],
    // Gap E: injectRulesDetected → readAgentConventions reads trackfw.yaml from root;
    // without it in the dry-run sandbox, agent_conventions is silently omitted from
    // CLAUDE.md, producing a hash that diverges from the real run.
    seeds: ['trackfw.yaml'],
    apply: (root) => generators.injectRulesDetected(root),
    dryRun,
    installMissing,
  }));

  if (include('agent-hooks')) targets.push(runFileTarget({
    id: 'agent-hooks',
    path: '.claude/settings.json, .codex/hooks.json, .gemini/settings.json, .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, scripts/trackfw-attention-*.sh, scripts/trackfw-credential-guard.sh, scripts/trackfw-git-branch-guard.sh, .windsurf/hooks.json, .amazonq/cli-agents/q_cli_default.json',
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
      'scripts/trackfw-git-branch-guard.sh',
      '.windsurf/hooks.json',                   // Gap A: injectWindsurfHooks writes this
      '.amazonq/cli-agents/q_cli_default.json', // Gap B: injectAmazonQHooks writes this
    ],
    // Gap C: injectHooksDetected checks these files to decide which hooks to
    // inject. Without them in the dry-run sandbox, hooks for installed agents
    // (windsurf, copilot, etc.) are silently omitted — dry-run lies by omission.
    // Gap E: trackfw.yaml may be read by hooks generators for config.
    seeds: [
      'trackfw.yaml',
      'CLAUDE.md',
      'AGENTS.md',
      'GEMINI.md',
      '.github/copilot-instructions.md',
      '.windsurfrules',
      '.amazonq/developer/guidelines.md',
    ],
    apply: (root) => {
      hooksGen.injectHooksDetected(root);
      hooksGen.generateAttentionScripts(cfg, root);
      hooksGen.generateCredentialGuardScript(root);
      hooksGen.generateGitBranchGuardScript(root);
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

  // ci-workflow mirrors Go's internal/generators/update.go:1925 target: it manages
  // the SAME pair of files Go manages — .github/workflows/trackfw-gate.yml (the
  // generated, version-pinned governance gate, ADR-2026-08-28) and
  // .gitlab-ci-trackfw.yml — AND, since ML-2G (AC17), ALSO
  // .github/workflows/trackfw-validate.yml, the different file written by
  // discover.js (own template, `go install …@latest`, version-pinned to the
  // generating binary). That third file is refreshed-only-if-present
  // (AC17(a)/(b)): `update` never installs it, it only keeps an
  // already-discover-installed copy in sync with what the current binary
  // would generate, using discover.js's own builder so `update` and `doctor`
  // can never drift apart by construction.
  // Writing directly with the byte-identical builders (instead of calling
  // generateCIWorkflow/generateGitHubActionsWorkflow from generators/init.js) is
  // deliberate: those functions hardcode paths relative to process.cwd() with no
  // root parameter, which would silently escape the dry-run tmp sandbox that
  // runFileTarget builds for every other target here.
  //
  // AC17(c): the target is included when cfg.ci opts into github-actions/
  // gitlab-ci (as before) OR when trackfw-validate.yml already exists on
  // disk — otherwise a `ci: none` project that ran `discover` would have
  // that file outside any command's management.
  const ciWorkflowConfigOptIn = cfg.ci === 'github-actions' || cfg.ci === 'github_actions' || cfg.ci === 'gitlab-ci';
  if (include('ci-workflow') && (ciWorkflowConfigOptIn || discoverWorkflowPresent(cwd))) {
    const isGitLab = cfg.ci === 'gitlab-ci';
    const { DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH } = require('./discover');
    targets.push(runFileTarget({
      id: 'ci-workflow',
      path: `.github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml, ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH}`,
      root: cwd,
      relPaths: ['.github/workflows/trackfw-gate.yml', '.gitlab-ci-trackfw.yml', DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH],
      apply: (root) => {
        if (isGitLab) {
          fs.writeFileSync(path.join(root, '.gitlab-ci-trackfw.yml'), generators.buildGitLabCIWorkflowContent(cfg), 'utf8');
        } else if (cfg.ci === 'github-actions' || cfg.ci === 'github_actions') {
          fs.mkdirSync(path.join(root, '.github/workflows'), { recursive: true });
          fs.writeFileSync(path.join(root, '.github/workflows/trackfw-gate.yml'), generators.buildGitHubActionsWorkflowContent(cfg), 'utf8');
        }
        refreshDiscoverGitHubActionsWorkflowIfPresent(root);
      },
      dryRun,
      installMissing,
    }));
  } else if (include('ci-workflow')) {
    // ML-3E (audit of ML-3D): the branch above is skipped exactly when
    // `ci-workflow` isn't config-opted-in AND discoverWorkflowPresent(cwd)
    // is false — which is also true when the file at
    // DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH is a symlink (live or dangling),
    // since discoverWorkflowPresent() deliberately treats symlinks as NOT
    // present (AC17(c) comment above). Without this call, a `ci: none`
    // project with a discover-installed symlink went completely silent:
    // no target declared, no write, no warning — the exact failure mode
    // this REQ exists to close (ML-3D's own stderr warning only fired from
    // inside the branch above, i.e. only in cases where it was already not
    // needed). refreshDiscoverGitHubActionsWorkflowIfPresent(cwd) re-checks
    // the real cwd independently: it is a no-op when the file is absent
    // (AC17(b) — update never creates it) and prints the SAME stderr
    // warning Go/Python emit — byte-identical, see
    // internal/generators/update.go:1899 and
    // pypi/trackfw/commands/update.py:221-225 — when it is a symlink. No
    // regular file ever reaches this branch (that case is always declared
    // above via discoverWorkflowPresent(cwd) === true), so this can never
    // duplicate the write/warning already performed by the branch above,
    // and never writes through a symlink itself.
    refreshDiscoverGitHubActionsWorkflowIfPresent(cwd);
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

  ensureGlobalAdrDirRegistered(cwd);

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
  const identityConfig = identityStore.load(homedir());

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
