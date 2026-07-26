'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const identityWizard = require('../src/commands/identity-wizard')
const { createLifecycleCommand } = require('../src/commands/integrations')
const identityStore = require('../src/identity')
const { catalog } = require('../src/integrations')

function tmpHome() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-identity-wizard-home-'))
}

function tmpProject() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-identity-wizard-project-'))
}

// withTTY força process.stdin.isTTY para o valor dado durante fn, restaurando
// o descriptor original ao final (mesmo em caso de exceção). Necessário
// porque a decisão de disparar o wizard depende de stdin ser um TTY, e os
// testes rodam sob node:test sem um TTY real.
async function withTTY(value, fn) {
  const original = Object.getOwnPropertyDescriptor(process.stdin, 'isTTY')
  Object.defineProperty(process.stdin, 'isTTY', { value, configurable: true })
  try {
    return await fn()
  } finally {
    if (original) Object.defineProperty(process.stdin, 'isTTY', original)
    else delete process.stdin.isTTY
  }
}

// runInProject executa fn com HOME e cwd redirecionados para diretórios
// temporários isolados, nunca tocando o ~ real. Restaura ambos ao final.
async function runInProject(home, project, fn) {
  const originalHome = process.env.HOME
  const originalCwd = process.cwd()
  process.env.HOME = home
  process.chdir(project)
  try {
    return await fn()
  } finally {
    process.env.HOME = originalHome
    process.chdir(originalCwd)
  }
}

function writeGreekIdentity(home) {
  fs.mkdirSync(path.join(home, '.trackfw'), { recursive: true })
  const cfg = { schema_version: 1, agents: {} }
  const greek = identityStore.preset('greek')
  cfg.agents = greek.agents
  fs.writeFileSync(path.join(home, '.trackfw', 'identity.json'), `${JSON.stringify(cfg, null, 2)}\n`)
}

function identityFile(home) {
  return path.join(home, '.trackfw', 'identity.json')
}

// -----------------------------------------------------------------------
// shouldPromptIdentity — tabela exaustiva (espelha
// internal/commands/identity_wizard_test.go:TestShouldPromptIdentityExhaustive)
// -----------------------------------------------------------------------
test('shouldPromptIdentity — tabela exaustiva das 10 combinações', () => {
  const cases = [
    ['agents+tty+absent+noforce -> prompt', 'agents', true, false, false, true],
    ['agents+tty+absent+force -> prompt', 'agents', true, false, true, true],
    ['agents+tty+existing+noforce -> silent', 'agents', true, true, false, false],
    ['agents+tty+existing+force -> prompt', 'agents', true, true, true, true],
    ['agents+notty+absent+noforce -> never blocks', 'agents', false, false, false, false],
    ['agents+notty+absent+force -> never blocks', 'agents', false, false, true, false],
    ['agents+notty+existing+force -> never blocks', 'agents', false, true, true, false],
    ['skills+tty+absent+noforce -> never (D5)', 'skills', true, false, false, false],
    ['skills+tty+absent+force -> never (D5)', 'skills', true, false, true, false],
    ['skills+notty+existing+force -> never (D5)', 'skills', false, true, true, false],
  ]
  for (const [name, kind, isTTY, identityExists, forceFlag, want] of cases) {
    assert.equal(identityWizard.shouldPromptIdentity(kind, isTTY, identityExists, forceFlag), want, name)
  }
})

// -----------------------------------------------------------------------
// agents install — com identity.json existente e sem --identity: não invoca
// -----------------------------------------------------------------------
test('agents install com identidade existente e sem --identity não invoca o wizard', async () => {
  const home = tmpHome()
  const project = tmpProject()
  writeGreekIdentity(home)

  const originalRunner = identityWizard.runIdentityWizard
  let called = false
  identityWizard.runIdentityWizard = async () => { called = true; return [{}, false] }

  let stdout = ''
  const originalLog = console.log
  console.log = (...args) => { stdout += `${args.join(' ')}\n` }

  try {
    await withTTY(true, () => runInProject(home, project, async () => {
      const cmd = createLifecycleCommand('agents')
      await cmd.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })
    }))
  } finally {
    identityWizard.runIdentityWizard = originalRunner
    console.log = originalLog
  }

  assert.equal(called, false, 'wizard não deve ser invocado com identidade já configurada e sem --identity')
  assert.match(stdout, /identity:|identidade:/)
})

// -----------------------------------------------------------------------
// agents install — sem identidade em TTY: invoca
// -----------------------------------------------------------------------
test('agents install sem identidade em TTY invoca o wizard', async () => {
  const home = tmpHome()
  const project = tmpProject()

  const originalRunner = identityWizard.runIdentityWizard
  let called = false
  identityWizard.runIdentityWizard = async () => { called = true; return [{}, false] }

  try {
    await withTTY(true, () => runInProject(home, project, async () => {
      const cmd = createLifecycleCommand('agents')
      await cmd.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })
    }))
  } finally {
    identityWizard.runIdentityWizard = originalRunner
  }

  assert.equal(called, true, 'wizard deve ser invocado quando identity.json está ausente e stdin é TTY')
})

// -----------------------------------------------------------------------
// agents install — com --identity e identidade existente: invoca
// -----------------------------------------------------------------------
test('agents install com --identity e identidade existente invoca o wizard', async () => {
  const home = tmpHome()
  const project = tmpProject()
  writeGreekIdentity(home)

  const originalRunner = identityWizard.runIdentityWizard
  let called = false
  identityWizard.runIdentityWizard = async () => { called = true; return [{}, false] }

  try {
    await withTTY(true, () => runInProject(home, project, async () => {
      const cmd = createLifecycleCommand('agents')
      await cmd.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project', '--identity'], { from: 'user' })
    }))
  } finally {
    identityWizard.runIdentityWizard = originalRunner
  }

  assert.equal(called, true, 'wizard deve ser invocado quando --identity é passado, mesmo com identity.json existente')
})

// -----------------------------------------------------------------------
// skills install — nunca invoca (ADR D5)
// -----------------------------------------------------------------------
test('skills install nunca invoca o wizard, mesmo sem identity.json', async () => {
  const home = tmpHome()
  const project = tmpProject()

  const originalRunner = identityWizard.runIdentityWizard
  let called = false
  identityWizard.runIdentityWizard = async () => { called = true; return [{}, false] }

  try {
    await withTTY(true, () => runInProject(home, project, async () => {
      const cmd = createLifecycleCommand('skills')
      await cmd.parseAsync(['install', '--targets', 'claude', '--items', 'governance', '--scope', 'project'], { from: 'user' })
    }))
  } finally {
    identityWizard.runIdentityWizard = originalRunner
  }

  assert.equal(called, false, 'skills install nunca deve invocar o wizard de identidade')
})

test('skills install não registra as flags de identidade', () => {
  const cmd = createLifecycleCommand('skills')
  const install = cmd.commands.find(entry => entry.name() === 'install')
  assert.equal(install.options.some(option => option.long === '--identity'), false)
  assert.equal(install.options.some(option => option.long === '--identity-preset'), false)
})

test('agents install/update/uninstall registram as flags de identidade', () => {
  const cmd = createLifecycleCommand('agents')
  for (const operation of ['install', 'update', 'uninstall']) {
    const sub = cmd.commands.find(entry => entry.name() === operation)
    assert.equal(sub.options.some(option => option.long === '--identity'), true, operation)
    assert.equal(sub.options.some(option => option.long === '--identity-preset'), true, operation)
  }
  const list = cmd.commands.find(entry => entry.name() === 'list')
  assert.equal(list.options.some(option => option.long === '--identity'), false)
})

// -----------------------------------------------------------------------
// não-TTY — não bloqueia, continua exigindo --targets; --identity-preset
// resolve identidade sem nunca invocar o wizard interativo
// -----------------------------------------------------------------------
test('agents install não-TTY não bloqueia e --identity-preset nunca invoca o wizard', async () => {
  const home = tmpHome()
  const project = tmpProject()

  const originalRunner = identityWizard.runIdentityWizard
  let called = false
  identityWizard.runIdentityWizard = async () => { called = true; return [{}, false] }

  try {
    await runInProject(home, project, async () => {
      const cmd = createLifecycleCommand('agents')
      await cmd.parseAsync(['install', '--identity-preset', 'greek', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })
    })
  } finally {
    identityWizard.runIdentityWizard = originalRunner
  }

  assert.equal(called, false, '--identity-preset deve resolver a identidade sem nunca invocar o wizard interativo')
  const cfg = JSON.parse(fs.readFileSync(identityFile(home), 'utf8'))
  assert.equal(Object.keys(cfg.agents).length, 12)

  const home2 = tmpHome()
  const project2 = tmpProject()
  await assert.rejects(
    runInProject(home2, project2, async () => {
      const cmd = createLifecycleCommand('agents')
      await cmd.parseAsync(['install', '--identity-preset', 'greek'], { from: 'user' })
    }),
    /requires --targets in non-interactive mode/,
  )
})

// -----------------------------------------------------------------------
// --identity-preset inválido → erro listando os válidos, nada é gravado
// -----------------------------------------------------------------------
test('--identity-preset inválido falha listando os válidos e não grava nada', async () => {
  const home = tmpHome()
  const project = tmpProject()

  await assert.rejects(
    runInProject(home, project, async () => {
      const cmd = createLifecycleCommand('agents')
      await cmd.parseAsync(['install', '--identity-preset', 'not-a-real-preset', '--targets', 'claude'], { from: 'user' })
    }),
    err => {
      for (const want of ['none', 'neutral', 'greek', 'norse', 'egyptian']) {
        assert.match(err.message, new RegExp(want))
      }
      return true
    },
  )
  assert.equal(fs.existsSync(identityFile(home)), false)
})

// -----------------------------------------------------------------------
// --identity-preset starwars via CLI ponta-a-ponta produz artefato dba com
// r2-d2-tf, provando que a identidade recém-gravada é a que é renderizada.
// -----------------------------------------------------------------------
test('--identity-preset starwars renderiza r2-d2-tf no artefato dba', async () => {
  const home = tmpHome()
  const project = tmpProject()

  await runInProject(home, project, async () => {
    const cmd = createLifecycleCommand('agents')
    await cmd.parseAsync(['install', '--identity-preset', 'starwars', '--targets', 'claude', '--items', 'dba', '--scope', 'project'], { from: 'user' })
  })

  const dbaFile = path.join(project, '.claude/agents/trackfw-dba.md')
  assert.equal(fs.existsSync(dbaFile), true)
  assert.match(fs.readFileSync(dbaFile, 'utf8'), /^---\nname: r2-d2-tf\n/)
})

// -----------------------------------------------------------------------
// Recusar confirmação → nenhum arquivo gravado (D3). runIdentityWizard real
// (sem spy) com prompts injetados: primeira volta recusa a confirmação;
// segunda volta escolhe "neutral", que é o caminho "não gravar nada".
// -----------------------------------------------------------------------
test('recusar a confirmação faz o wizard voltar ao início sem gravar nada', async () => {
  const home = tmpHome()
  let selectCalls = 0
  const prompts = {
    select: async () => { selectCalls += 1; return selectCalls === 1 ? 'greek' : 'neutral' },
    input: async () => '',
    confirm: async () => false,
  }

  const [cfg, persisted] = await identityWizard.runIdentityWizard(catalog, home, prompts)

  assert.equal(persisted, false)
  assert.deepEqual(cfg, {})
  assert.equal(selectCalls, 2, 'wizard deve voltar à seleção de preset após a recusa')
  assert.equal(fs.existsSync(identityFile(home)), false)
})

test('confirmar persiste a identidade escolhida', async () => {
  const home = tmpHome()
  const prompts = {
    select: async () => 'greek',
    input: async () => 'Chefe',
    confirm: async () => true,
  }

  const [cfg, persisted] = await identityWizard.runIdentityWizard(catalog, home, prompts)

  assert.equal(persisted, true)
  assert.equal(cfg.agents.architect.display_name, 'Zeus')
  assert.equal(cfg.user_nickname, 'Chefe')
  const onDisk = JSON.parse(fs.readFileSync(identityFile(home), 'utf8'))
  assert.equal(onDisk.agents.architect.display_name, 'Zeus')
})

// -----------------------------------------------------------------------
// Rótulos do modo custom contêm o description do catálogo, não o id cru
// (ADR D4)
// -----------------------------------------------------------------------
test('modo custom rotula os campos com o description do catálogo, nunca o id cru', async () => {
  const home = tmpHome()
  const messages = []
  const knownAgentIds = identityStore.knownAgentIds()
  let inputCalls = 0
  const prompts = {
    select: async () => 'custom',
    input: async ({ message }) => {
      inputCalls += 1
      if (inputCalls <= knownAgentIds.length) {
        messages.push(message)
        return `Agente ${inputCalls}`
      }
      return ''
    },
    confirm: async () => true,
  }

  await identityWizard.runIdentityWizard(catalog, home, prompts)

  assert.equal(messages.length, knownAgentIds.length)
  knownAgentIds.forEach((id, index) => {
    const item = catalog.agents.find(entry => entry.id === id)
    assert.match(messages[index], new RegExp(item.name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
    assert.match(messages[index], new RegExp(item.description.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
    assert.doesNotMatch(messages[index], new RegExp(`\\(${id}\\)`))
  })
})

test('id de agente ausente do catálogo falha explicitamente em vez de rotular errado', async () => {
  const fakeCatalog = { agents: [] }
  const cfg = { agents: { 'not-a-real-agent-id': { display_name: 'X', slug: 'x' } } }
  await assert.rejects(
    identityWizard.confirmIdentitySelection(fakeCatalog, ['not-a-real-agent-id'], cfg, { confirm: async () => true }),
    /has no entry in the agents catalog/,
  )
})
