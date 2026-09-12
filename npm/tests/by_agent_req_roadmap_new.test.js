'use strict'
// Tests for ML-1B: --agent flag in req new and roadmap new (AC4, AC5, AC10, AC11, AC12, AC14)
// Each test has an explicit reconciliation sentence stating which conclusion of ML-1B it affirms.

const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const config = require('../src/config/index.js')
const { newREQ } = require('../src/generators/req')
const { newRoadmap, newRoadmapFromReq, agentFromPath } = require('../src/generators/roadmap')
const { resolveAgentForWrite } = require('../src/validator/index.js')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

// Helper: cria tmp dir com trackfw.yaml, muda cwd, executa fn SÍNCRONA, restaura.
function withProject(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ml1b-'))
  const origCwd = process.cwd()
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8')
    config.reset()
    process.chdir(tmp)
    fn(tmp)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

// Helper async: como withProject mas aguarda a fn async antes de restaurar o ambiente.
async function withProjectAsync(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ml1b-async-'))
  const origCwd = process.cwd()
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8')
    config.reset()
    process.chdir(tmp)
    await fn(tmp)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

// ─── resolveAgentForWrite (shared resolver) ───────────────────────────────────

// Reconciliation: affirms that resolveAgentForWrite returns explicit agent regardless of agents: list.
test('resolveAgentForWrite: agente explícito retorna sem consultar a lista', () => {
  const cfg = { agents: ['alpha', 'beta'], roadmapNamespacing: 'by_agent' }
  const result = resolveAgentForWrite(cfg, 'beta')
  assert.strictEqual(result, 'beta')
})

// Reconciliation: affirms that resolveAgentForWrite uses the single agent without error.
test('resolveAgentForWrite: um namespace → retorna aquele, sem erro', () => {
  const cfg = { agents: ['alpha'], roadmapNamespacing: 'by_agent' }
  const result = resolveAgentForWrite(cfg, undefined)
  assert.strictEqual(result, 'alpha')
})

// Reconciliation: affirms that resolveAgentForWrite ignores empty entries, treating ["","zeus"] as one namespace.
test('resolveAgentForWrite: nomes vazios não contam — ["","zeus"] é UM namespace', () => {
  const cfg = { agents: ['', 'zeus'] }
  const result = resolveAgentForWrite(cfg, undefined)
  assert.strictEqual(result, 'zeus')
})

// Reconciliation: affirms that resolveAgentForWrite throws the byte-identical parity message (contrato de paridade Go/Node/Python).
test('resolveAgentForWrite: múltiplos namespaces sem flag → mensagem byte-idêntica ao contrato de paridade', () => {
  const cfg = { agents: ['alpha', 'beta'] }
  let threw = false
  let msg = ''
  try {
    resolveAgentForWrite(cfg, undefined)
  } catch (e) {
    threw = true
    msg = e.message
  }
  assert(threw, 'deve lançar erro em multi-agente sem flag explícita')
  const expected = 'by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one'
  assert.strictEqual(msg, expected, `mensagem deve ser byte-idêntica ao contrato; recebeu: "${msg}"`)
})

// Reconciliation: affirms that resolveAgentForWrite with empty entries still produces the parity message with only non-empty names.
test('resolveAgentForWrite: múltiplos namespaces com vazio → mensagem cita só os não-vazios (paridade)', () => {
  const cfg = { agents: ['', 'alpha', 'beta'] }
  let threw = false
  let msg = ''
  try {
    resolveAgentForWrite(cfg, undefined)
  } catch (e) {
    threw = true
    msg = e.message
  }
  assert(threw, 'deve lançar erro — ["","alpha","beta"] tem dois namespaces não-vazios')
  const expected = 'by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one'
  assert.strictEqual(msg, expected, `mensagem deve ser byte-idêntica; recebeu: "${msg}"`)
})

// ─── agentFromPath ────────────────────────────────────────────────────────────

// Reconciliation: affirms that agentFromPath extracts agent from canonical REQ path (one level deep).
test('agentFromPath: req_dir/beta/REQ-x.md → "beta"', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'afp-'))
  try {
    const reqDir = path.join(tmp, 'docs', 'req')
    const reqFile = path.join(reqDir, 'beta', 'REQ-001.md')
    const result = agentFromPath(reqFile, reqDir)
    assert.strictEqual(result, 'beta')
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Reconciliation: affirms that agentFromPath also works for roadmap paths (two levels deep), consistent with moveRoadmap.
test('agentFromPath: roadmap_dir/beta/wip/R.md → "beta"', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'afp2-'))
  try {
    const rmDir = path.join(tmp, 'docs', 'roadmaps')
    const rmFile = path.join(rmDir, 'beta', 'wip', 'ROADMAP-001.md')
    const result = agentFromPath(rmFile, rmDir)
    assert.strictEqual(result, 'beta')
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ─── req new: by_agent com múltiplos agentes + --agent ───────────────────────

// Reconciliation: affirms that newREQ with --agent beta creates file in beta/ (path is authoritative; no squad: key in REQ frontmatter).
// newREQ is async but runs synchronously in non-TTY mode — call without await, check fs state immediately.
test('newREQ: by_agent [alpha,beta] + --agent beta → artefato em beta/, SEM squad: no frontmatter', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nreq_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    const reqDir = path.join(tmp, 'docs', 'req')
    process.exitCode = 0
    newREQ({ title: 'Test REQ beta', motivation: '', criteria: '', dependsOnADRs: [] }, 'beta')
    assert.strictEqual(process.exitCode, 0, 'process.exitCode deve ser 0')
    const betaDir = path.join(reqDir, 'beta')
    const files = fs.existsSync(betaDir) ? fs.readdirSync(betaDir).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Esperava REQ em beta/, encontrou: ${JSON.stringify(fs.existsSync(betaDir) ? fs.readdirSync(betaDir) : [])}`)
    const content = fs.readFileSync(path.join(betaDir, files[0]), 'utf8')
    assert(!content.includes('squad:'), `Frontmatter da REQ NÃO deve ter squad: (o caminho é a fonte de verdade). Got:\n${content.slice(0, 300)}`)
  })
})

// Reconciliation: affirms that newREQ without --agent errors with a message naming both alpha AND beta.
test('newREQ: by_agent [alpha,beta] sem --agent → erro nomeando alpha E beta', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nreq_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    const errMessages = []
    const origErr = console.error
    console.error = (...args) => errMessages.push(args.join(' '))
    process.exitCode = 0
    newREQ({ title: 'Ambiguous REQ', motivation: '', criteria: '', dependsOnADRs: [] }, undefined)
    console.error = origErr
    assert.notStrictEqual(process.exitCode, 0, 'process.exitCode deve ser não-zero (erro de ambiguidade)')
    const combined = errMessages.join('\n')
    assert(combined.includes('alpha'), `Mensagem de erro deve nomear "alpha". Got: "${combined}"`)
    assert(combined.includes('beta'), `Mensagem de erro deve nomear "beta". Got: "${combined}"`)
  })
})

// Reconciliation: affirms that newREQ without --agent in single-agent by_agent succeeds without error (counter-arm: guard must not fire always).
test('newREQ: by_agent [alpha] sem --agent → cria em alpha/, SEM erro', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\nreq_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    const reqDir = path.join(tmp, 'docs', 'req')
    process.exitCode = 0
    newREQ({ title: 'Single Agent REQ', motivation: '', criteria: '', dependsOnADRs: [] }, undefined)
    assert.strictEqual(process.exitCode, 0, 'process.exitCode deve ser 0 — projeto single-agent nunca vê o erro')
    const alphaDir = path.join(reqDir, 'alpha')
    const files = fs.existsSync(alphaDir) ? fs.readdirSync(alphaDir).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Esperava REQ em alpha/, encontrou: ${JSON.stringify(fs.existsSync(alphaDir) ? fs.readdirSync(alphaDir) : [])}`)
  })
})

// Reconciliation: affirms that newREQ in flat mode behaves unchanged (agent param is ignored).
test('newREQ: flat → comportamento inalterado (req_dir raiz, sem squad de agente)', () => {
  const yaml = 'req_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    const reqDir = path.join(tmp, 'docs', 'req')
    process.exitCode = 0
    newREQ({ title: 'Flat REQ', motivation: '', criteria: '', dependsOnADRs: [] }, undefined)
    assert.strictEqual(process.exitCode, 0, 'flat mode deve funcionar sem erro')
    const files = fs.existsSync(reqDir) ? fs.readdirSync(reqDir).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Esperava REQ diretamente em docs/req/. Got: ${JSON.stringify(fs.existsSync(reqDir) ? fs.readdirSync(reqDir) : [])}`)
  })
})

// ─── roadmap new: by_agent com múltiplos agentes + --agent ───────────────────

// Reconciliation: affirms that newRoadmap with --agent beta creates roadmap in beta/ and records beta in squad frontmatter.
test('newRoadmap: by_agent [alpha,beta] + --agent beta → artefato em beta/, frontmatter squad:beta', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nroadmap_dir: docs/roadmaps\n'
  withProject(yaml, (tmp) => {
    process.exitCode = 0
    newRoadmap('Beta Roadmap', '', 'beta')
    assert.strictEqual(process.exitCode, 0, 'process.exitCode deve ser 0')
    const betaBacklog = path.join(tmp, 'docs', 'roadmaps', 'beta', 'backlog')
    const files = fs.existsSync(betaBacklog) ? fs.readdirSync(betaBacklog).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Esperava roadmap em beta/backlog/. Got: ${JSON.stringify(fs.existsSync(betaBacklog) ? fs.readdirSync(betaBacklog) : [])}`)
    const content = fs.readFileSync(path.join(betaBacklog, files[0]), 'utf8')
    assert(content.includes('squad: "beta"'), `Frontmatter deve ter squad: "beta". Got:\n${content.slice(0, 300)}`)
  })
})

// Reconciliation: affirms that newRoadmap without --agent errors naming alpha AND beta in the message.
test('newRoadmap: by_agent [alpha,beta] sem --agent → erro nomeando alpha E beta', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nroadmap_dir: docs/roadmaps\n'
  withProject(yaml, (tmp) => {
    const errMessages = []
    const origErr = console.error
    console.error = (...args) => errMessages.push(args.join(' '))
    process.exitCode = 0
    newRoadmap('Ambiguous Roadmap', '', undefined)
    console.error = origErr
    assert.notStrictEqual(process.exitCode, 0, 'process.exitCode deve ser não-zero')
    const combined = errMessages.join('\n')
    assert(combined.includes('alpha'), `Mensagem deve nomear "alpha". Got: "${combined}"`)
    assert(combined.includes('beta'), `Mensagem deve nomear "beta". Got: "${combined}"`)
  })
})

// Reconciliation: affirms that newRoadmap without --agent in single-agent project succeeds without error (counter-arm).
test('newRoadmap: by_agent [alpha] sem --agent → cria em alpha/, SEM erro (contra-braço)', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\nroadmap_dir: docs/roadmaps\n'
  withProject(yaml, (tmp) => {
    process.exitCode = 0
    newRoadmap('Single Agent Roadmap', '', undefined)
    assert.strictEqual(process.exitCode, 0, 'processo single-agent nunca deve ver o erro de ambiguidade')
    const alphaBacklog = path.join(tmp, 'docs', 'roadmaps', 'alpha', 'backlog')
    const files = fs.existsSync(alphaBacklog) ? fs.readdirSync(alphaBacklog).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Esperava roadmap em alpha/backlog/. Got: ${JSON.stringify(fs.existsSync(alphaBacklog) ? fs.readdirSync(alphaBacklog) : [])}`)
  })
})

// Reconciliation: affirms that newRoadmap in flat mode is unchanged (no agent, no squad field from agent).
test('newRoadmap: flat → comportamento inalterado (backlog/ sem agente)', () => {
  const yaml = 'roadmap_dir: docs/roadmaps\n'
  withProject(yaml, (tmp) => {
    process.exitCode = 0
    newRoadmap('Flat Roadmap', '', undefined)
    assert.strictEqual(process.exitCode, 0, 'flat mode deve funcionar sem erro')
    const backlog = path.join(tmp, 'docs', 'roadmaps', 'backlog')
    const files = fs.existsSync(backlog) ? fs.readdirSync(backlog).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Esperava roadmap em roadmaps/backlog/. Got: ${JSON.stringify(fs.existsSync(backlog) ? fs.readdirSync(backlog) : [])}`)
  })
})

// ─── roadmap new --req: herda agente da REQ (AC11) ───────────────────────────

// Reconciliation: affirms that newRoadmap with --req pointing to beta/ derives agent from REQ path and creates roadmap in beta/.
test('newRoadmap: by_agent [alpha,beta] + --req em beta/ → roadmap em beta/ (herança de agente)', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nroadmap_dir: docs/roadmaps\nreq_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    // Cria REQ no namespace beta (caminho canônico: req_dir/beta/REQ-x.md)
    const betaReqDir = path.join(tmp, 'docs', 'req', 'beta')
    fs.mkdirSync(betaReqDir, { recursive: true })
    const reqFile = path.join(betaReqDir, 'REQ-2026-09-11-test.md')
    fs.writeFileSync(reqFile, '---\nstatus: Open\n---\n# REQ: Test\n', 'utf8')

    process.exitCode = 0
    newRoadmap('Inherited Agent Roadmap', reqFile, undefined)

    assert.strictEqual(process.exitCode, 0, 'deve ter exitCode 0 ao herdar agente da REQ')
    const betaBacklog = path.join(tmp, 'docs', 'roadmaps', 'beta', 'backlog')
    const files = fs.existsSync(betaBacklog) ? fs.readdirSync(betaBacklog).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Roadmap deve estar em beta/backlog/. Got: beta/backlog exists=${fs.existsSync(betaBacklog)}, alpha/backlog exists=${fs.existsSync(path.join(tmp, 'docs', 'roadmaps', 'alpha', 'backlog'))}`)
    const content = fs.readFileSync(path.join(betaBacklog, files[0]), 'utf8')
    assert(content.includes('squad: "beta"'), `squad deve ser "beta" no frontmatter. Got:\n${content.slice(0, 300)}`)
  })
})

// ─── agentFromPath: casos limite ─────────────────────────────────────────────

// Reconciliation: affirms that agentFromPath returns '' for a REQ sitting flat in req_dir (legacy flat layout).
// Without this guard, "REQ-x.md" would become a namespace, silently bypassing the ambiguity error.
test('agentFromPath: REQ plano em req_dir/ → "" (não vira namespace)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'afp-flat-'))
  try {
    const reqDir = path.join(tmp, 'docs', 'req')
    const reqFile = path.join(reqDir, 'REQ-2026-01-01-test.md')
    const result = agentFromPath(reqFile, reqDir)
    assert.strictEqual(result, '', `agentFromPath deve retornar '' para REQ plano, não "${result}"`)
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Reconciliation: affirms that roadmap new --req pointing to a flat REQ (in req_dir/) with multi-agent
// hits the ambiguity error (since agentFromPath returns '', falling through to resolveAgentForWrite).
test('newRoadmap: by_agent [alpha,beta] + --req plano em req_dir/ → erro de ambiguidade', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nroadmap_dir: docs/roadmaps\nreq_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    // REQ está direto em req_dir/ (layout flat legado) — não em req_dir/<agente>/
    const reqDir = path.join(tmp, 'docs', 'req')
    fs.mkdirSync(reqDir, { recursive: true })
    const reqFile = path.join(reqDir, 'REQ-2026-01-01-flat.md')
    fs.writeFileSync(reqFile, '---\nstatus: Open\n---\n# REQ: Flat\n', 'utf8')

    const errMessages = []
    const origErr = console.error
    console.error = (...args) => errMessages.push(args.join(' '))
    process.exitCode = 0
    newRoadmap('Roadmap From Flat REQ', reqFile, undefined)
    console.error = origErr

    assert.notStrictEqual(process.exitCode, 0, 'deve retornar erro de ambiguidade quando REQ plano e múltiplos agentes')
    const combined = errMessages.join('\n')
    assert(combined.includes('alpha'), `Mensagem deve nomear "alpha". Got: "${combined}"`)
    assert(combined.includes('beta'), `Mensagem deve nomear "beta". Got: "${combined}"`)
  })
})

// Reconciliation: affirms that roadmap new --req flat + explicit --agent beta still works (explicit flag wins).
test('newRoadmap: by_agent [alpha,beta] + --req plano + --agent beta → aterrissa em beta/', () => {
  const yaml = 'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nroadmap_dir: docs/roadmaps\nreq_dir: docs/req\n'
  withProject(yaml, (tmp) => {
    const reqDir = path.join(tmp, 'docs', 'req')
    fs.mkdirSync(reqDir, { recursive: true })
    const reqFile = path.join(reqDir, 'REQ-2026-01-01-flat.md')
    fs.writeFileSync(reqFile, '---\nstatus: Open\n---\n# REQ: Flat\n', 'utf8')

    process.exitCode = 0
    newRoadmap('Roadmap Explicit Beta', reqFile, 'beta')

    assert.strictEqual(process.exitCode, 0, 'process.exitCode deve ser 0 quando --agent explícito dado')
    const betaBacklog = path.join(tmp, 'docs', 'roadmaps', 'beta', 'backlog')
    const files = fs.existsSync(betaBacklog) ? fs.readdirSync(betaBacklog).filter(f => f.endsWith('.md')) : []
    assert(files.length > 0, `Roadmap deve estar em beta/backlog/ com --agent explícito. Got: betaBacklog exists=${fs.existsSync(betaBacklog)}`)
  })
})

// ─── commands: --agent flag exposta ──────────────────────────────────────────

// Reconciliation: affirms that req new command exposes --agent flag so the CLI accepts it.
test('comando req new expõe --agent', () => {
  const reqCmd = require('../src/commands/req')
  const newCmd = reqCmd.commands.find(c => c.name() === 'new')
  assert.ok(newCmd, 'subcomando req new deve existir')
  const flags = new Set(newCmd.options.map(o => o.long))
  assert(flags.has('--agent'), 'req new deve expor --agent')
})

// Reconciliation: affirms that roadmap new command exposes --agent flag so the CLI accepts it.
test('comando roadmap new expõe --agent', () => {
  const roadmapCmd = require('../src/commands/roadmap')
  const newCmd = roadmapCmd.commands.find(c => c.name() === 'new')
  assert.ok(newCmd, 'subcomando roadmap new deve existir')
  const flags = new Set(newCmd.options.map(o => o.long))
  assert(flags.has('--agent'), 'roadmap new deve expor --agent')
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
