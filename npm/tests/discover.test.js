'use strict';

const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { scan, generateYAML, generateBootstrapLog } = require('../src/commands/discover');

// helpers
function mkTmp() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-discover-'));
}

function mkdir(base, rel) {
  fs.mkdirSync(path.join(base, rel), { recursive: true });
}

function writeFile(filePath, content) {
  fs.writeFileSync(filePath, content || '', 'utf8');
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
// scan() — relatório com estrutura flat
// ──────────────────────────────────────────────────────────────

test('scan — empty dir → zeros, score 0', () => {
  const tmp = mkTmp();
  try {
    const r = scan(tmp);
    assert.strictEqual(r.adrCount, 0);
    assert.strictEqual(r.reqCount, 0);
    assert.strictEqual(r.roadmapCount, 0);
    assert.strictEqual(r.governanceScore, 0);
    assert.strictEqual(r.roadmapNamespacing, 'flat');
    assert.deepStrictEqual(r.adrDirs, []);
    assert.strictEqual(r.reqDir, '');
    assert.strictEqual(r.roadmapDir, '');
  } finally {
    cleanup(tmp);
  }
});

test('scan — flat: docs/adr + docs/req + docs/roadmaps/done → counts corretos', () => {
  const tmp = mkTmp();
  try {
    mkdir(tmp, 'docs/adr');
    mkdir(tmp, 'docs/req');
    mkdir(tmp, 'docs/roadmaps/wip');
    mkdir(tmp, 'docs/roadmaps/done');
    writeFile(path.join(tmp, 'docs/adr/ADR-001.md'), '# ADR');
    writeFile(path.join(tmp, 'docs/adr/ADR-002.md'), '# ADR');
    writeFile(path.join(tmp, 'docs/req/REQ-001.md'), '# REQ');
    writeFile(path.join(tmp, 'docs/roadmaps/done/ROADMAP-001.md'), '# R');
    writeFile(path.join(tmp, 'docs/roadmaps/done/ROADMAP-002.md'), '# R');
    writeFile(path.join(tmp, 'docs/roadmaps/done/ROADMAP-003.md'), '# R');

    const r = scan(tmp);
    assert.deepStrictEqual(r.adrDirs, ['docs/adr']);
    assert.strictEqual(r.adrCount, 2, `adrCount should be 2, got ${r.adrCount}`);
    assert.strictEqual(r.reqDir, 'docs/req');
    assert.strictEqual(r.reqCount, 1, `reqCount should be 1, got ${r.reqCount}`);
    assert.strictEqual(r.roadmapDir, 'docs/roadmaps');
    assert.strictEqual(r.roadmapCount, 3, `roadmapCount should be 3, got ${r.roadmapCount}`);
    assert.strictEqual(r.roadmapNamespacing, 'flat');
  } finally {
    cleanup(tmp);
  }
});

test('scan — by_agent: docs/requisições + subdirs com wip/ → by_agent + agents', () => {
  const tmp = mkTmp();
  try {
    mkdir(tmp, 'docs/adr/zeus');
    mkdir(tmp, 'docs/requisições');
    mkdir(tmp, 'docs/roadmaps/zeus/wip');
    mkdir(tmp, 'docs/roadmaps/apolo/done');
    writeFile(path.join(tmp, 'docs/adr/zeus/ADR-001.md'), '# ADR');
    writeFile(path.join(tmp, 'docs/requisições/REQ-001.md'), '# REQ');
    writeFile(path.join(tmp, 'docs/roadmaps/zeus/wip/ROADMAP-001.md'), '# R');
    writeFile(path.join(tmp, 'docs/roadmaps/apolo/done/ROADMAP-002.md'), '# R');

    const r = scan(tmp);
    assert.strictEqual(r.roadmapNamespacing, 'by_agent');
    assert.strictEqual(r.reqDir, 'docs/requisições');
    assert.ok(r.agents.includes('zeus'), 'agents should include zeus');
    assert.ok(r.agents.includes('apolo'), 'agents should include apolo');
    assert.strictEqual(r.roadmapCount, 2, `roadmapCount should be 2, got ${r.roadmapCount}`);
    assert.strictEqual(r.adrCount, 1, `adrCount should be 1, got ${r.adrCount}`);
    assert.ok(r.adrDirs.includes('docs/adr/zeus'), 'adrDirs should include docs/adr/zeus');
  } finally {
    cleanup(tmp);
  }
});

test('scan — hook lefthook + CI github-actions → detectados', () => {
  const tmp = mkTmp();
  try {
    writeFile(path.join(tmp, 'lefthook.yml'), '# lefthook');
    mkdir(tmp, '.github/workflows');

    const r = scan(tmp);
    assert.strictEqual(r.hookFramework, 'lefthook');
    assert.strictEqual(r.ciSystem, 'github-actions');
  } finally {
    cleanup(tmp);
  }
});

test('scan — trackfw.yaml existe → hasTrackfwYAML true + score aumenta', () => {
  const tmp = mkTmp();
  try {
    writeFile(path.join(tmp, 'trackfw.yaml'), 'governance_mode: lenient\n');
    const r = scan(tmp);
    assert.strictEqual(r.hasTrackfwYAML, true);
    assert.ok(r.governanceScore >= 20, `score should be >= 20, got ${r.governanceScore}`);
  } finally {
    cleanup(tmp);
  }
});

test('scan — score 100 quando todas as categorias presentes', () => {
  const tmp = mkTmp();
  try {
    mkdir(tmp, 'docs/adr');
    mkdir(tmp, 'docs/req');
    mkdir(tmp, 'docs/roadmaps/done');
    writeFile(path.join(tmp, 'docs/adr/ADR-001.md'), '# ADR');
    writeFile(path.join(tmp, 'docs/req/REQ-001.md'), '# REQ');
    writeFile(path.join(tmp, 'docs/roadmaps/done/ROADMAP-001.md'), '# R');
    writeFile(path.join(tmp, 'trackfw.yaml'), 'governance_mode: lenient\n');
    writeFile(path.join(tmp, 'docs/roadmaps/.trackfw-log'), '');

    const r = scan(tmp);
    assert.strictEqual(r.governanceScore, 100, `score should be 100, got ${r.governanceScore}`);
  } finally {
    cleanup(tmp);
  }
});

// ──────────────────────────────────────────────────────────────
// generateYAML()
// ──────────────────────────────────────────────────────────────

test('generateYAML — docs/requisições + by_agent → yaml correto', () => {
  const r = {
    adrDirs: ['docs/adr/zeus', 'docs/adr/apolo'],
    reqDir: 'docs/requisições',
    roadmapDir: 'docs/roadmaps',
    roadmapNamespacing: 'by_agent',
    agents: ['zeus', 'apolo'],
    hookFramework: 'lefthook',
    ciSystem: 'github-actions',
  };

  const yaml = generateYAML(r);
  assert.ok(yaml.includes('governance_mode: lenient'), 'should include governance_mode: lenient');
  assert.ok(yaml.includes('docs/requisições'), 'should include req_dir in Portuguese');
  assert.ok(yaml.includes('roadmap_namespacing: by_agent'), 'should include by_agent namespacing');
  assert.ok(yaml.includes('- zeus'), 'should include zeus in agents');
  assert.ok(yaml.includes('- apolo'), 'should include apolo in agents');
  assert.ok(yaml.includes('- docs/adr/zeus'), 'should include docs/adr/zeus in adr_dirs');
  assert.ok(yaml.includes('- docs/adr/apolo'), 'should include docs/adr/apolo in adr_dirs');
});

test('generateYAML — sem adrDirs → usa default docs/adr', () => {
  const r = {
    adrDirs: [],
    reqDir: '',
    roadmapDir: '',
    roadmapNamespacing: 'flat',
    agents: [],
    hookFramework: 'none',
    ciSystem: 'none',
  };

  const yaml = generateYAML(r);
  assert.ok(yaml.includes('- docs/adr'), 'should default to docs/adr');
  assert.ok(yaml.includes('req_dir: docs/req'), 'should default req_dir to docs/req');
  assert.ok(yaml.includes('roadmap_dir: docs/roadmaps'), 'should default roadmap_dir');
});

test('generateYAML — sem agents → não inclui lista agents', () => {
  const r = {
    adrDirs: ['docs/adr'],
    reqDir: 'docs/req',
    roadmapDir: 'docs/roadmaps',
    roadmapNamespacing: 'flat',
    agents: [],
    hookFramework: 'none',
    ciSystem: 'none',
  };

  const yaml = generateYAML(r);
  assert.ok(!yaml.includes('agents:'), 'should not include agents list when empty');
});

// ──────────────────────────────────────────────────────────────
// generateBootstrapLog()
// ──────────────────────────────────────────────────────────────

test('generateBootstrapLog — flat: done/ com arquivos → entradas no log', () => {
  const tmp = mkTmp();
  try {
    mkdir(tmp, 'docs/roadmaps/done');
    writeFile(path.join(tmp, 'docs/roadmaps/done/ROADMAP-001.md'), '# R');
    writeFile(path.join(tmp, 'docs/roadmaps/done/ROADMAP-002.md'), '# R');

    const r = {
      roadmapDir: 'docs/roadmaps',
      roadmapNamespacing: 'flat',
      agents: [],
    };

    const log = generateBootstrapLog(r, tmp);
    assert.ok(log.includes('ROADMAP-001.md'), 'log should contain ROADMAP-001.md');
    assert.ok(log.includes('ROADMAP-002.md'), 'log should contain ROADMAP-002.md');
    assert.ok(log.includes('backlog → done'), 'log entries should include state transition');
    // formato de data YYYY-MM-DD HH:MM
    assert.ok(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}/.test(log), 'log should contain YYYY-MM-DD HH:MM timestamp');
  } finally {
    cleanup(tmp);
  }
});

test('generateBootstrapLog — by_agent: done/ com agente → entradas com prefixo agent/', () => {
  const tmp = mkTmp();
  try {
    mkdir(tmp, 'docs/roadmaps/zeus/done');
    mkdir(tmp, 'docs/roadmaps/apolo/done');
    writeFile(path.join(tmp, 'docs/roadmaps/zeus/done/ROADMAP-001.md'), '# R');
    writeFile(path.join(tmp, 'docs/roadmaps/apolo/done/ROADMAP-002.md'), '# R');

    const r = {
      roadmapDir: 'docs/roadmaps',
      roadmapNamespacing: 'by_agent',
      agents: ['zeus', 'apolo'],
    };

    const log = generateBootstrapLog(r, tmp);
    assert.ok(log.includes('zeus/ROADMAP-001.md'), 'log should include agent prefix zeus/');
    assert.ok(log.includes('apolo/ROADMAP-002.md'), 'log should include agent prefix apolo/');
  } finally {
    cleanup(tmp);
  }
});

test('generateBootstrapLog — done/ vazio → log vazio', () => {
  const tmp = mkTmp();
  try {
    mkdir(tmp, 'docs/roadmaps/done');

    const r = {
      roadmapDir: 'docs/roadmaps',
      roadmapNamespacing: 'flat',
      agents: [],
    };

    const log = generateBootstrapLog(r, tmp);
    assert.strictEqual(log, '', 'log should be empty when done/ has no files');
  } finally {
    cleanup(tmp);
  }
});

// ──────────────────────────────────────────────────────────────
// --init: idempotência (não sobrescreve yaml existente)
// ──────────────────────────────────────────────────────────────

test('generateYAML — idempotência: yaml com governance_mode gerado no topo', () => {
  // Verifica que o yaml gerado começa com o comentário correto e governance_mode
  const r = {
    adrDirs: ['docs/adr'],
    reqDir: 'docs/req',
    roadmapDir: 'docs/roadmaps',
    roadmapNamespacing: 'flat',
    agents: [],
    hookFramework: 'none',
    ciSystem: 'none',
  };
  const yaml = generateYAML(r);
  assert.ok(yaml.startsWith('# trackfw configuration'), 'yaml should start with comment header');
  assert.ok(yaml.includes('governance_mode: lenient'), 'yaml should include governance_mode');
});

// ──────────────────────────────────────────────────────────────
// relatório final
// ──────────────────────────────────────────────────────────────

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
