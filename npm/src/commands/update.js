'use strict';

const { Command } = require('commander');
const fs = require('fs');
const path = require('path');

function readUpdateConfig(rootDir) {
  const yaml = path.join(rootDir, 'trackfw.yaml');
  if (!fs.existsSync(yaml)) return {};
  const lines = fs.readFileSync(yaml, 'utf8').split('\n');
  const cfg = {};
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf(':');
    if (idx < 0) continue;
    const key = trimmed.slice(0, idx).trim();
    let val = trimmed.slice(idx + 1).trim();
    const ci = val.indexOf(' #');
    if (ci >= 0) val = val.slice(0, ci).trim();
    cfg[key] = val;
  }
  return cfg;
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

const cmd = new Command('update');
cmd.description('Update trackfw-managed artifacts to the current version');
cmd.action(() => {
  const cwd = process.cwd();
  const yaml = path.join(cwd, 'trackfw.yaml');
  if (!fs.existsSync(yaml)) {
    console.error('✗ trackfw.yaml não encontrado — execute trackfw init primeiro');
    process.exit(1);
  }

  const cfg = readUpdateConfig(cwd);
  const generators = require('../generators/init');
  const discover = require('./discover');

  console.log('trackfw update — re-aplicando templates atuais...\n');

  // 1. Agent rules (marker-delimited, idempotent)
  try {
    generators.injectRulesDetected(cwd);
    console.log('  ✓ agent rules atualizadas');
  } catch (e) {
    console.log(`  ⚠ agent rules: ${e.message}`);
  }
  if (fs.existsSync(path.join(cwd, 'AGENTS.md')) || fs.existsSync(path.join(cwd, '.codex'))) {
    try {
      const { buildPlans, IntegrationManager } = require('../integrations');
      const roots = { projectRoot: cwd };
      const manager = new IntegrationManager(roots);
      for (const kind of ['agents', 'skills']) {
        const plans = buildPlans(kind, { targets: ['codex'], scope: 'project' });
        const statuses = manager.inspect(plans);
        const existing = plans.filter((_, index) => statuses[index].state !== 'not-installed');
        manager.update(existing);
      }
    } catch (e) {
      console.warn(`  ⚠ Codex integration: ${e.message}`);
    }
  }

  // 1b. Agent hooks (attention signal/cleanup)
  try {
    const { injectHooksDetected, generateAttentionScripts } = require('../generators/hooks');
    injectHooksDetected(cwd);
    generateAttentionScripts(cfg, cwd);
    console.log('  ✓ agent hooks atualizados');
  } catch (e) {
    console.warn(`  ⚠ agent hooks: ${e.message}`);
  }

  // 2. Validate script (trackfw-owned, overwrite)
  try {
    discover.writeValidateScript(cwd);
    console.log('  ✓ scripts/trackfw-validate.sh atualizado');
  } catch (e) {
    console.log(`  ⚠ validate script: ${e.message}`);
  }

  // 3. CI workflow (trackfw-owned, overwrite)
  if (cfg.ci === 'github-actions' || cfg.ci === 'github_actions') {
    try {
      discover.writeCIWorkflowForce(cwd);
      console.log('  ✓ .github/workflows/trackfw-validate.yml atualizado');
    } catch (e) {
      console.log(`  ⚠ CI workflow: ${e.message}`);
    }
  }

  // 4. Git hooks (surgical)
  updateHooksSurgical(cfg, cwd);

  // 5. Claude commands (force overwrite)
  try {
    generators.generateClaudeCommandsForce(cwd);
    console.log('  ✓ .claude/commands/trackfw/ atualizado');
  } catch (e) {
    console.log(`  ⚠ Claude commands: ${e.message}`);
  }

  // 6. Global skill (force overwrite)
  try {
    generators.installSkillsForce(cwd);
    console.log('  ✓ skill global atualizada');
  } catch (e) {
    console.log(`  ⚠ skills: ${e.message}`);
  }

  console.log('\n✓ trackfw update concluído');
  require('../generators/init').printArchitectNextSteps(process.cwd())
});

module.exports = cmd;
