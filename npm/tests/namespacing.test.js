'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const config = require('../src/config/index.js')
const validator = require('../src/validator/index.js')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

// Helper: cria tmp dir, escreve trackfw.yaml, muda cwd, executa fn, restaura
function withTmpDir(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ns-'))
  const origCwd = process.cwd()
  try {
    if (yaml) fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8')
    config.reset()
    process.chdir(tmp)
    fn(tmp)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

// ─── Testes de Config ─────────────────────────────────────────────────────────

test('roadmap_namespacing: by_agent → cfg.roadmapNamespacing correto', () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.roadmapNamespacing, 'by_agent')
  })
})

test('roadmap_namespacing: by_agent + agents: [zeus, apolo] → cfg.agents correto', () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.agents, ['zeus', 'apolo'])
  })
})

test('sem roadmap_namespacing → default flat', () => {
  withTmpDir(null, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.roadmapNamespacing, 'flat')
  })
})

test('sem agents → default []', () => {
  withTmpDir(null, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.agents, [])
  })
})

test('agents com três itens → array completo', () => {
  const yaml = `agents:\n  - zeus\n  - apolo\n  - artemis\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.agents, ['zeus', 'apolo', 'artemis'])
  })
})

// ─── Testes de resolveWIPDirs ──────────────────────────────────────────────────

test('resolveWIPDirs com by_agent retorna wip por agente', () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\nroadmap_dir: docs/roadmaps\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    const wipDirs = validator.resolveWIPDirs(cfg)
    assert.strictEqual(wipDirs.length, 2)
    assert(wipDirs.some(d => d.includes('zeus/wip')), `Expected zeus/wip in ${JSON.stringify(wipDirs)}`)
    assert(wipDirs.some(d => d.includes('apolo/wip')), `Expected apolo/wip in ${JSON.stringify(wipDirs)}`)
  })
})

test('resolveWIPDirs sem namespacing retorna wip flat', () => {
  const yaml = `roadmap_dir: docs/roadmaps\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    const wipDirs = validator.resolveWIPDirs(cfg)
    assert.strictEqual(wipDirs.length, 1)
    assert(wipDirs[0].endsWith('/wip'), `Expected single wip dir, got ${JSON.stringify(wipDirs)}`)
    assert(!wipDirs[0].includes('zeus'), 'flat mode must not include agent name')
  })
})

// ─── Testes de Validator com by_agent ────────────────────────────────────────

test('validateWIPHasREQ com by_agent varre hierarquia de dois níveis', () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\nroadmap_dir: docs/roadmaps\n`
  withTmpDir(yaml, (tmp) => {
    // Criar estrutura: docs/roadmaps/<agent>/wip/
    for (const agent of ['zeus', 'apolo']) {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps', agent, 'wip'), { recursive: true })
    }
    // Roadmap de zeus sem REQ → deve gerar violation
    fs.writeFileSync(
      path.join(tmp, 'docs/roadmaps/zeus/wip/ZEUS-001.md'),
      '---\nstatus: WIP\n---\n# Zeus Roadmap\n## Acceptance Criteria\n- [ ] done\n'
    )
    // Roadmap de apolo com REQ → não deve gerar violation
    fs.writeFileSync(
      path.join(tmp, 'docs/roadmaps/apolo/wip/APOLO-001.md'),
      '---\nstatus: WIP\nREQ: docs/req/REQ-001.md\n---\n# Apolo Roadmap\n## Acceptance Criteria\n- [ ] done\n'
    )

    const violations = validator.validateWIPHasREQ()
    assert(violations.some(v => v.includes('ZEUS-001.md')), `Expected violation for ZEUS-001.md, got: ${JSON.stringify(violations)}`)
    assert(!violations.some(v => v.includes('APOLO-001.md')), `Should not flag APOLO-001.md, got: ${JSON.stringify(violations)}`)
  })
})

test('validateWIPHasAcceptanceCriteria com by_agent varre dois agentes', () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\nroadmap_dir: docs/roadmaps\n`
  withTmpDir(yaml, (tmp) => {
    for (const agent of ['zeus', 'apolo']) {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps', agent, 'wip'), { recursive: true })
    }
    // zeus sem acceptance criteria
    fs.writeFileSync(
      path.join(tmp, 'docs/roadmaps/zeus/wip/ZEUS-002.md'),
      '---\nstatus: WIP\nREQ: docs/req/REQ-001.md\n---\n# Zeus Roadmap sem AC\n'
    )
    // apolo com acceptance criteria
    fs.writeFileSync(
      path.join(tmp, 'docs/roadmaps/apolo/wip/APOLO-002.md'),
      '---\nstatus: WIP\nREQ: docs/req/REQ-001.md\n---\n# Apolo OK\n## Acceptance Criteria\n- [ ] done\n'
    )

    const violations = validator.validateWIPHasAcceptanceCriteria()
    assert(violations.some(v => v.includes('ZEUS-002.md')), `Expected violation for ZEUS-002.md, got: ${JSON.stringify(violations)}`)
    assert(!violations.some(v => v.includes('APOLO-002.md')), `Should not flag APOLO-002.md, got: ${JSON.stringify(violations)}`)
  })
})

test('validateWIPLimit com by_agent verifica por agente separadamente', () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\nroadmap_dir: docs/roadmaps\nwip_limit: 1\n`
  withTmpDir(yaml, (tmp) => {
    for (const agent of ['zeus', 'apolo']) {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps', agent, 'wip'), { recursive: true })
    }
    // zeus com 2 roadmaps (acima do limite de 1)
    fs.writeFileSync(path.join(tmp, 'docs/roadmaps/zeus/wip/ZEUS-A.md'), '# A\n')
    fs.writeFileSync(path.join(tmp, 'docs/roadmaps/zeus/wip/ZEUS-B.md'), '# B\n')
    // apolo com 1 roadmap (dentro do limite)
    fs.writeFileSync(path.join(tmp, 'docs/roadmaps/apolo/wip/APOLO-A.md'), '# A\n')

    const result = validator.validateWIPLimit()
    assert(result.warnings.some(w => w.includes('zeus') && w.includes('2')), `Expected zeus WIP warning, got: ${JSON.stringify(result.warnings)}`)
    assert(!result.warnings.some(w => w.includes('apolo') && w.includes('1')), `Should not warn apolo, got: ${JSON.stringify(result.warnings)}`)
  })
})

// ─── Testes de comportamento flat inalterado ───────────────────────────────────

test('modo flat: validateWIPHasREQ usa wip/ plano', () => {
  const yaml = `roadmap_dir: docs/roadmaps\n`
  withTmpDir(yaml, (tmp) => {
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
    fs.writeFileSync(
      path.join(tmp, 'docs/roadmaps/wip/FLAT-001.md'),
      '---\nstatus: WIP\n---\n# Flat Roadmap\n'
    )

    const violations = validator.validateWIPHasREQ()
    assert(violations.some(v => v.includes('FLAT-001.md')), `Expected violation for FLAT-001.md, got: ${JSON.stringify(violations)}`)
  })
})

test('modo flat: resolveWIPDirs retorna apenas wip/ sem subdivisão por agente', () => {
  const yaml = `roadmap_dir: docs/roadmaps\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    const wipDirs = validator.resolveWIPDirs(cfg)
    assert.strictEqual(wipDirs.length, 1)
    assert(wipDirs[0].includes('roadmaps/wip'), `Expected roadmaps/wip, got: ${JSON.stringify(wipDirs)}`)
  })
})

test('modo flat: validateWIPLimit usa lógica global', () => {
  const yaml = `roadmap_dir: docs/roadmaps\nwip_limit: 1\n`
  withTmpDir(yaml, (tmp) => {
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
    fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-A.md'), '# A\n')
    fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-B.md'), '# B\n')

    const result = validator.validateWIPLimit()
    assert(result.warnings.some(w => w.includes('2') && w.includes('limit: 1')), `Expected global WIP warning, got: ${JSON.stringify(result.warnings)}`)
    // Não deve mencionar agentes
    assert(!result.warnings.some(w => w.includes('agent')), `Flat mode should not mention agents, got: ${JSON.stringify(result.warnings)}`)
  })
})

// ─── Testes de getStatus com by_agent ─────────────────────────────────────────

test('getStatus com by_agent exibe breakdown por agente', async () => {
  const yaml = `roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\nroadmap_dir: docs/roadmaps\n`
  await new Promise((resolve, reject) => {
    withTmpDir(yaml, async (tmp) => {
      for (const agent of ['zeus', 'apolo']) {
        fs.mkdirSync(path.join(tmp, 'docs/roadmaps', agent, 'wip'), { recursive: true })
      }
      fs.writeFileSync(path.join(tmp, 'docs/roadmaps/zeus/wip/ZEUS-001.md'), '# Zeus WIP\n')

      try {
        const out = await validator.getStatus()
        assert(out.includes('WIP by Agent'), `Expected "WIP by Agent" in output, got: ${out}`)
        assert(out.includes('[zeus]'), `Expected "[zeus]" in output, got: ${out}`)
        assert(out.includes('ZEUS-001.md'), `Expected "ZEUS-001.md" in output, got: ${out}`)
        resolve()
      } catch (e) {
        reject(e)
      }
    })
  })
})

// ─── CONSTANTS ────────────────────────────────────────────────────────────────

test('NAMESPACING_FLAT e NAMESPACING_BY_AGENT exportados corretamente', () => {
  assert.strictEqual(config.NAMESPACING_FLAT, 'flat')
  assert.strictEqual(config.NAMESPACING_BY_AGENT, 'by_agent')
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
