'use strict';

// Port of the Go reference tests for the ML-2A (agentes especialistas aceitam contexto de
// convenções específico do projeto) feature — internal/config/config_test.go and
// internal/generators/agentfiles_test.go / internal/discover/discover_test.go equivalents.

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const config = require('../src/config/index.js');
const { trackfwRulesBlock } = require('../src/generators/init.js');
const { scan, generateYAML, detectTestFramework } = require('../src/commands/discover.js');

function mkTmp(prefix) {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix || 'trackfw-agent-conventions-'));
}

function cleanup(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
}

let passed = 0;
let failed = 0;

function test(name, fn) {
  try {
    fn();
    console.log(`✓ ${name}`);
    passed++;
  } catch (e) {
    console.error(`✗ ${name}: ${e.message}`);
    failed++;
  }
}

// ──────────────────────────────────────────────────────────────
// config.js — parse() e readAgentConventions()
// ──────────────────────────────────────────────────────────────

test('config.load — agent_conventions ausente → cfg.update.agentConventions vazio', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'backend: go\n', 'utf8');
    config.reset();
    const cfg = config.load(tmp);
    assert.strictEqual(cfg.update.agentConventions, '');
  } finally {
    config.reset();
    cleanup(tmp);
  }
});

test('config.load — agent_conventions single-line → lido corretamente', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'agent_conventions: Use pytest, not unittest.\n', 'utf8');
    config.reset();
    const cfg = config.load(tmp);
    assert.strictEqual(cfg.update.agentConventions, 'Use pytest, not unittest.');
  } finally {
    config.reset();
    cleanup(tmp);
  }
});

test('config.load — agent_conventions multi-linha (block scalar) → lido corretamente', () => {
  const tmp = mkTmp();
  try {
    const yaml = [
      'agent_conventions: |',
      '  Use pytest, not unittest.',
      '  API REST, no GraphQL.',
      '',
    ].join('\n');
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8');
    config.reset();
    const cfg = config.load(tmp);
    assert.strictEqual(cfg.update.agentConventions, 'Use pytest, not unittest.\nAPI REST, no GraphQL.\n');
  } finally {
    config.reset();
    cleanup(tmp);
  }
});

test('config.readAgentConventions — arquivo ausente → string vazia, nunca erro', () => {
  const tmp = mkTmp();
  try {
    assert.strictEqual(config.readAgentConventions(tmp), '');
  } finally {
    cleanup(tmp);
  }
});

test('config.readAgentConventions — chave ausente → string vazia', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'backend: go\n', 'utf8');
    assert.strictEqual(config.readAgentConventions(tmp), '');
  } finally {
    cleanup(tmp);
  }
});

test('config.readAgentConventions — chave presente → valor lido, sem depender do singleton load()', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'agent_conventions: |\n  Use jest.\n', 'utf8');
    // não chama config.load()/reset() antes — prova que readAgentConventions não depende do
    // singleton (mesmo isolamento do Go ReadAgentConventions).
    assert.strictEqual(config.readAgentConventions(tmp), 'Use jest.\n');
  } finally {
    cleanup(tmp);
  }
});

test('config.readAgentConventions — trackfw.yaml malformado → string vazia, nunca erro', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), ':::not valid yaml:::\n  - [unterminated\n', 'utf8');
    assert.doesNotThrow(() => config.readAgentConventions(tmp));
  } finally {
    cleanup(tmp);
  }
});

// ──────────────────────────────────────────────────────────────
// generators/init.js — trackfwRulesBlock()
// ──────────────────────────────────────────────────────────────

test('trackfwRulesBlock — agent_conventions vazio → seção "### Project Conventions" ausente (byte-idêntico ao comportamento pré-ML)', () => {
  const withEmpty = trackfwRulesBlock('');
  const withUndefined = trackfwRulesBlock();
  assert.ok(!withEmpty.includes('### Project Conventions'));
  assert.strictEqual(withEmpty, withUndefined);
});

test('trackfwRulesBlock — agent_conventions com conteúdo → seção presente, texto do time preservado verbatim', () => {
  const conventions = 'Use pytest, not unittest.\nAPI REST, no GraphQL.';
  const block = trackfwRulesBlock(conventions);
  assert.ok(block.includes('### Project Conventions'));
  assert.ok(block.includes("Declared by the team in `trackfw.yaml`'s `agent_conventions` field"));
  assert.ok(block.includes(conventions));
  // a seção deve vir antes de "### Key Commands", mesma posição do Go
  assert.ok(block.indexOf('### Project Conventions') < block.indexOf('### Key Commands'));
});

test('trackfwRulesBlock — agent_conventions só com espaços → tratado como vazio (strings.TrimSpace equivalente)', () => {
  const block = trackfwRulesBlock('   \n  \n');
  assert.ok(!block.includes('### Project Conventions'));
});

// ──────────────────────────────────────────────────────────────
// generators/init.js — injectOrUpdateRules() lendo trackfw.yaml do cwd
// ──────────────────────────────────────────────────────────────

test('injectRulesForTool — sem trackfw.yaml → CLAUDE.md sem "### Project Conventions" (não-regressão)', () => {
  const tmp = mkTmp();
  try {
    const { injectRulesForTool } = require('../src/generators/init.js');
    injectRulesForTool('claude', tmp);
    const content = fs.readFileSync(path.join(tmp, 'CLAUDE.md'), 'utf8');
    assert.ok(!content.includes('### Project Conventions'));
  } finally {
    cleanup(tmp);
  }
});

test('injectRulesForTool — trackfw.yaml com agent_conventions → CLAUDE.md contém a seção com o texto exato', () => {
  const tmp = mkTmp();
  try {
    const yaml = [
      'agent_conventions: |',
      '  Use pytest, not unittest.',
      '  API REST, no GraphQL.',
      '',
    ].join('\n');
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8');
    const { injectRulesForTool } = require('../src/generators/init.js');
    injectRulesForTool('claude', tmp);
    const content = fs.readFileSync(path.join(tmp, 'CLAUDE.md'), 'utf8');
    assert.ok(content.includes('### Project Conventions'));
    assert.ok(content.includes('Use pytest, not unittest.\nAPI REST, no GraphQL.'));
  } finally {
    cleanup(tmp);
  }
});

// ──────────────────────────────────────────────────────────────
// commands/discover.js — detectTestFramework() / scan()
// ──────────────────────────────────────────────────────────────

test('detectTestFramework — nenhum arquivo-gatilho → string vazia', () => {
  const tmp = mkTmp();
  try {
    assert.strictEqual(detectTestFramework(tmp), '');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — jest.config.js → "jest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'jest.config.js'), 'module.exports = {}\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'jest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — jest.config.ts → "jest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'jest.config.ts'), 'export default {}\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'jest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — vitest.config.js → "vitest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'vitest.config.js'), 'export default {}\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'vitest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — vitest.config.ts → "vitest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'vitest.config.ts'), 'export default {}\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'vitest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — pytest.ini → "pytest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'pytest.ini'), '[pytest]\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'pytest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — pyproject.toml com [tool.pytest...] → "pytest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'pyproject.toml'), '[tool.pytest.ini_options]\naddopts = "-v"\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'pytest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — pyproject.toml sem seção pytest → não bate', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'pyproject.toml'), '[tool.black]\nline-length = 100\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), '');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — setup.cfg com [tool:pytest] → "pytest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'setup.cfg'), '[tool:pytest]\ntestpaths = tests\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'pytest');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — go.mod + *_test.go → "go test"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'go.mod'), 'module example.com/x\n\ngo 1.22\n', 'utf8');
    fs.mkdirSync(path.join(tmp, 'internal'), { recursive: true });
    fs.writeFileSync(path.join(tmp, 'internal', 'foo_test.go'), 'package internal\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'go test');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — go.mod sem nenhum *_test.go → não bate', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'go.mod'), 'module example.com/x\n\ngo 1.22\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), '');
  } finally {
    cleanup(tmp);
  }
});

test('detectTestFramework — precedência jest > vitest > pytest > go test', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'jest.config.js'), '{}\n', 'utf8');
    fs.writeFileSync(path.join(tmp, 'vitest.config.js'), '{}\n', 'utf8');
    fs.writeFileSync(path.join(tmp, 'pytest.ini'), '[pytest]\n', 'utf8');
    fs.writeFileSync(path.join(tmp, 'go.mod'), 'module x\n', 'utf8');
    assert.strictEqual(detectTestFramework(tmp), 'jest');
  } finally {
    cleanup(tmp);
  }
});

test('scan — jest.config.js presente → r.suggestedTestFramework === "jest"', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'jest.config.js'), 'module.exports = {}\n', 'utf8');
    const r = scan(tmp);
    assert.strictEqual(r.suggestedTestFramework, 'jest');
  } finally {
    cleanup(tmp);
  }
});

test('scan — sem arquivo-gatilho → r.suggestedTestFramework === ""', () => {
  const tmp = mkTmp();
  try {
    const r = scan(tmp);
    assert.strictEqual(r.suggestedTestFramework, '');
  } finally {
    cleanup(tmp);
  }
});

// ──────────────────────────────────────────────────────────────
// --init nunca escreve agent_conventions automaticamente
// ──────────────────────────────────────────────────────────────

test('generateYAML — nunca inclui agent_conventions, mesmo com framework de teste sugerido', () => {
  const tmp = mkTmp();
  try {
    fs.writeFileSync(path.join(tmp, 'jest.config.js'), 'module.exports = {}\n', 'utf8');
    const r = scan(tmp);
    assert.strictEqual(r.suggestedTestFramework, 'jest');
    const yaml = generateYAML(r);
    assert.ok(!yaml.includes('agent_conventions'));
  } finally {
    cleanup(tmp);
  }
});

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exitCode = 1;
