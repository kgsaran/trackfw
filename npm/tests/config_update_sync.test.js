'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const config = require('../src/config/index.js')

function withTmpDir(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-config-update-sync-'))
  try {
    if (yaml) fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8')
    config.reset()
    fn(tmp)
  } finally {
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

// AC2/AC3 da REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis:
// os onze campos historicamente lidos pelos scanners artesanais (update.js / sync.js) resolvem
// como string via o único caminho config.load(), expostos em cfg.update e cfg.sync. As chaves
// continuam planas na raiz do YAML — só o objeto em memória é namespaced.
test('update e sync — onze campos resolvidos como string', () => {
  const yaml = [
    'hooks: husky',
    'ci: github',
    'backend: go',
    'frontend: react',
    'pkg_manager: npm',
    'linear_api_key: lin_api_abc123',
    'linear_team_id: TEAM-1',
    'jira_base_url: "https://x.atlassian.net:443"',
    'jira_email: bot@example.com',
    'jira_token: jira_tok_xyz',
    'jira_project: PROJ',
    '',
  ].join('\n')
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.update.hooks, 'husky')
    assert.strictEqual(cfg.update.ci, 'github')
    assert.strictEqual(cfg.update.backend, 'go')
    assert.strictEqual(cfg.update.frontend, 'react')
    assert.strictEqual(cfg.update.pkgManager, 'npm')
    assert.strictEqual(cfg.sync.linearApiKey, 'lin_api_abc123')
    assert.strictEqual(cfg.sync.linearTeamId, 'TEAM-1')
    assert.strictEqual(cfg.sync.jiraBaseUrl, 'https://x.atlassian.net:443')
    assert.strictEqual(cfg.sync.jiraEmail, 'bot@example.com')
    assert.strictEqual(cfg.sync.jiraToken, 'jira_tok_xyz')
    assert.strictEqual(cfg.sync.jiraProject, 'PROJ')
  })
})

// Default de campo ausente no YAML: string vazia, nos 3 CLIs (spec do ML-1A).
test('update e sync — campos ausentes default para string vazia', () => {
  withTmpDir(null, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.update.hooks, '')
    assert.strictEqual(cfg.update.ci, '')
    assert.strictEqual(cfg.update.backend, '')
    assert.strictEqual(cfg.update.frontend, '')
    assert.strictEqual(cfg.update.pkgManager, '')
    assert.strictEqual(cfg.sync.linearApiKey, '')
    assert.strictEqual(cfg.sync.linearTeamId, '')
    assert.strictEqual(cfg.sync.jiraBaseUrl, '')
    assert.strictEqual(cfg.sync.jiraEmail, '')
    assert.strictEqual(cfg.sync.jiraToken, '')
    assert.strictEqual(cfg.sync.jiraProject, '')
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
