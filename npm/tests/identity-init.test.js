'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

function dirs() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-init-identity-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  return { base, projectRoot, homeRoot }
}

function runInit(cwd, homeRoot, args = []) {
  const bin = path.resolve(__dirname, '../bin/trackfw')
  return spawnSync(process.execPath, [bin, 'init', '--ai-tools', '', ...args], {
    cwd,
    env: { ...process.env, HOME: homeRoot },
    input: '',
    encoding: 'utf8',
  })
}

function identityFile(homeRoot) {
  return path.join(homeRoot, '.trackfw', 'identity.json')
}

test('init não-TTY não bloqueia e conclui com sucesso', () => {
  const { projectRoot, homeRoot } = dirs()
  const run = runInit(projectRoot, homeRoot)
  assert.equal(run.status, 0, run.stderr)
  assert.match(run.stdout, /trackfw inicializado|trackfw initialized/)
})

test('init --identity-preset invalido falha listando os presets válidos', () => {
  const { projectRoot, homeRoot } = dirs()
  const run = runInit(projectRoot, homeRoot, ['--identity-preset', 'bogus'])
  assert.notEqual(run.status, 0)
  assert.match(run.stderr, /identity-preset invalido.*greek/)
  assert.equal(fs.existsSync(identityFile(homeRoot)), false)
})

test('init --identity-preset greek grava identity.json com o preset esperado', () => {
  const { projectRoot, homeRoot } = dirs()
  const run = runInit(projectRoot, homeRoot, ['--identity-preset', 'greek'])
  assert.equal(run.status, 0, run.stderr)
  const cfg = JSON.parse(fs.readFileSync(identityFile(homeRoot), 'utf8'))
  assert.equal(cfg.schema_version, 1)
  assert.deepEqual(cfg.agents.architect, { display_name: 'Zeus', slug: 'zeus' })
  assert.deepEqual(cfg.agents.backend, { display_name: 'Apolo', slug: 'apolo' })
})

test('init --identity-preset none/neutral não grava identity.json', () => {
  const { projectRoot: p1, homeRoot: h1 } = dirs()
  assert.equal(runInit(p1, h1, ['--identity-preset', 'none']).status, 0)
  assert.equal(fs.existsSync(identityFile(h1)), false)

  const { projectRoot: p2, homeRoot: h2 } = dirs()
  assert.equal(runInit(p2, h2, ['--identity-preset', 'neutral']).status, 0)
  assert.equal(fs.existsSync(identityFile(h2)), false)
})

// D5 (ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills):
// `init` deve imprimir os destinos resolvidos antes de gravar os artefatos de
// AI tools, assim como o CLI Go (installAITools) e o CLI Python
// (commands/init.py). Antes do ML-2A, `npm/src/commands/init.js` era o único
// dos 3 CLIs que ficava silencioso aqui (divergência #2 do roadmap).
test('init --ai-tools claude sem TTY imprime destinos antes da gravação (D5)', () => {
  const { projectRoot, homeRoot } = dirs()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'init', '--ai-tools', 'claude'], {
    cwd: projectRoot,
    env: { ...process.env, HOME: homeRoot },
    input: '',
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  assert.match(run.stdout, /Destino \(global\):/)
  assert.match(run.stdout, /\.claude[\\/]agents[\\/]trackfw-architect\.md/)
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude/agents/trackfw-architect.md')), true)
})

test('re-init sem flag preserva identity.json já existente (idempotente)', () => {
  const { projectRoot, homeRoot } = dirs()
  const first = runInit(projectRoot, homeRoot, ['--identity-preset', 'greek'])
  assert.equal(first.status, 0, first.stderr)
  const before = fs.readFileSync(identityFile(homeRoot), 'utf8')

  const second = runInit(projectRoot, homeRoot)
  assert.equal(second.status, 0, second.stderr)
  const after = fs.readFileSync(identityFile(homeRoot), 'utf8')
  assert.equal(after, before)
})
