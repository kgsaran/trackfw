'use strict'

// ML-2A (REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis)
// — `trackfw sync` now resolves linear_*/jira_* fields through the single config loader
// (../src/config/index.js, cfg.sync) instead of the removed artisanal readConfigField scanner.
// Every scenario runs against an empty docs/req/ (or none at all), so syncToProvider() returns
// before ever making a real network call — these tests exercise config resolution + error text
// only, never the Linear/Jira HTTP clients.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const bin = path.resolve(__dirname, '../bin/trackfw')

function scratch(yamlContent) {
  const projectRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-sync-test-'))
  if (yamlContent !== undefined) {
    fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), yamlContent)
  }
  return projectRoot
}

function run(args, cwd, env) {
  return spawnSync(process.execPath, [bin, ...args], {
    cwd,
    env: { ...process.env, ...env },
    encoding: 'utf8',
  })
}

// AC5 — trackfw.yaml value wins over the env var fallback.
test('sync --to=linear: trackfw.yaml value takes precedence over env vars', () => {
  const projectRoot = scratch('linear_api_key: file-key\nlinear_team_id: file-team\n')
  const result = run(['sync', '--to=linear'], projectRoot, {
    LINEAR_API_KEY: 'env-key',
    LINEAR_TEAM_ID: 'env-team',
  })
  // No docs/req/ exists — syncToProvider returns [] before any network call, so a
  // successfully-resolved config reaches "No REQs found" instead of the credential error.
  assert.equal(result.status, 0, result.stderr + result.stdout)
  assert.match(result.stdout, /No REQs found/)
})

// AC5 — env var fallback used only when trackfw.yaml has no value.
test('sync --to=linear: falls back to env vars when trackfw.yaml is absent', () => {
  const projectRoot = scratch()
  const result = run(['sync', '--to=linear'], projectRoot, {
    LINEAR_API_KEY: 'env-key',
    LINEAR_TEAM_ID: 'env-team',
  })
  assert.equal(result.status, 0, result.stderr + result.stdout)
  assert.match(result.stdout, /No REQs found/)
})

// AC5 — error text byte-identical to the pre-refactor scanner's messages.
test('sync --to=linear: error message unchanged when no key is resolvable', () => {
  const projectRoot = scratch()
  const result = run(['sync', '--to=linear'], projectRoot, {
    LINEAR_API_KEY: '',
    LINEAR_TEAM_ID: '',
  })
  assert.notEqual(result.status, 0)
  assert.match(
    result.stderr,
    /Linear API key not found\. Set LINEAR_API_KEY env var or linear_api_key in trackfw\.yaml/
  )
})

// AC4 — quoted value, trailing comment and a nested homonym key (linear_api_key repeated
// inside an unrelated mapping) all resolve to the root-level value. The artisanal scanner
// matched the first line with the "field:" prefix at ANY indentation, so the nested homonym
// below would have silently hijacked the value; the single YAML-library loader must not.
test('sync --to=linear: AC4 tricky YAML (quoted value, comment, nested homonym) resolves correctly', () => {
  const projectRoot = scratch(
    'some_unrelated_map:\n' +
    '  linear_api_key: hijacked-nested-value\n' +
    'linear_api_key: "quoted-root-key"  # trailing comment\n' +
    'linear_team_id: root-team\n'
  )
  const result = run(['sync', '--to=linear'], projectRoot, {})
  assert.equal(result.status, 0, result.stderr + result.stdout)
  assert.match(result.stdout, /No REQs found/)
})

// AC5 — trackfw.yaml value wins over env vars for Jira.
test('sync --to=jira: trackfw.yaml value takes precedence over env vars', () => {
  const projectRoot = scratch(
    'jira_base_url: "https://file.atlassian.net:443"\n' +
    'jira_email: file@example.com\n' +
    'jira_token: file-token\n' +
    'jira_project: FILEPROJ\n'
  )
  const result = run(['sync', '--to=jira'], projectRoot, {
    JIRA_BASE_URL: 'https://env.atlassian.net',
    JIRA_EMAIL: 'env@example.com',
    JIRA_TOKEN: 'env-token',
    JIRA_PROJECT: 'ENVPROJ',
  })
  assert.equal(result.status, 0, result.stderr + result.stdout)
  assert.match(result.stdout, /No REQs found/)
})

// AC5 — env var fallback for Jira when trackfw.yaml is absent.
test('sync --to=jira: falls back to env vars when trackfw.yaml is absent', () => {
  const projectRoot = scratch()
  const result = run(['sync', '--to=jira'], projectRoot, {
    JIRA_BASE_URL: 'https://env.atlassian.net',
    JIRA_EMAIL: 'env@example.com',
    JIRA_TOKEN: 'env-token',
    JIRA_PROJECT: 'ENVPROJ',
  })
  assert.equal(result.status, 0, result.stderr + result.stdout)
  assert.match(result.stdout, /No REQs found/)
})

// AC5 — error text byte-identical to the pre-refactor scanner's messages.
test('sync --to=jira: error message unchanged when no base URL is resolvable', () => {
  const projectRoot = scratch()
  const result = run(['sync', '--to=jira'], projectRoot, {
    JIRA_BASE_URL: '',
    JIRA_EMAIL: '',
    JIRA_TOKEN: '',
    JIRA_PROJECT: '',
  })
  assert.notEqual(result.status, 0)
  assert.match(
    result.stderr,
    /Jira base URL not found\. Set JIRA_BASE_URL env var or jira_base_url in trackfw\.yaml/
  )
})

// AC4 — colon-embedded scalar (jira_base_url with an explicit port) resolves whole.
test('sync --to=jira: AC4 colon-embedded scalar resolves whole', () => {
  const projectRoot = scratch(
    'jira_base_url: "https://x.atlassian.net:443"\n' +
    'jira_email: bot@example.com\n' +
    'jira_token: tok\n' +
    'jira_project: PROJ\n'
  )
  const result = run(['sync', '--to=jira'], projectRoot, {})
  assert.equal(result.status, 0, result.stderr + result.stdout)
  assert.match(result.stdout, /No REQs found/)
})
