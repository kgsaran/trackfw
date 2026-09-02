'use strict'

// Mirrors internal/commands/doctor_remote_test.go — same scenarios, same non-vacuity guard
// (asserting the fake was actually called with the expected gh subcommand, not just inspecting
// the resulting findings).

const test = require('node:test')
const assert = require('node:assert/strict')

const { runDoctorRemote } = require('../src/integrations/doctor_remote')
const {
  REQUIRED_STATUS_CHECKS_MISSING,
  ENFORCE_ADMINS_DISABLED,
  HOOKS_PATH_NEUTRALIZED,
  NOT_EVALUATED,
} = require('../src/integrations/doctor')

function fakeExec(responses = {}, errors = {}) {
  const calls = []
  const exec = (name, args, stdin) => {
    const key = args.join(' ')
    calls.push(key)
    if (errors[key]) return { stdout: '', error: errors[key] }
    if (key in responses) return { stdout: responses[key], error: null }
    return { stdout: '{}', error: null }
  }
  exec.calls = calls
  return exec
}

function baseDeps({ hooksPath, hooksPathError, execForgeAPI, availGH = true }) {
  return {
    execGit: args => {
      const joined = args.join(' ')
      if (joined === 'config --get core.hooksPath') {
        if (hooksPathError) return { stdout: '', error: hooksPathError }
        return { stdout: hooksPath || '', error: null }
      }
      if (joined === 'remote get-url origin') {
        return { stdout: 'https://github.com/kgsaran/trackfw.git', error: null }
      }
      return { stdout: '', error: null }
    },
    execForgeAPI,
    availFn: name => (name === 'gh' ? availGH : false),
    configForge: '',
    repoDir: '',
  }
}

function hasKind(findings, kind) {
  return findings.some(f => f.finding === kind)
}

// ── Falsification direction (a): repo WITHOUT required_status_checks → finding ────────────
test('doctorRemote: no branch protection -> both findings', () => {
  const exec = fakeExec(
    { 'auth status': 'logged in', 'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":true}}' },
    { 'api repos/{owner}/{repo}/branches/main/protection': new Error('gh: Branch not protected (HTTP 404)') }
  )
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec }))
  assert.ok(hasKind(findings, REQUIRED_STATUS_CHECKS_MISSING))
  assert.ok(hasKind(findings, ENFORCE_ADMINS_DISABLED))
  assert.ok(!hasKind(findings, NOT_EVALUATED))
  assert.ok(exec.calls.includes('api repos/{owner}/{repo}/branches/main/protection'))
})

// ── Falsification direction (b), the CONTROL: repo WITH the gate configured → no finding ──
test('doctorRemote: protection configured -> zero findings (control)', () => {
  const exec = fakeExec({
    'auth status': 'logged in',
    'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":true}}',
    'api repos/{owner}/{repo}/branches/main/protection': '{"required_status_checks":{"strict":true,"contexts":["governance-go-install"]},"enforce_admins":{"enabled":true}}',
  })
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec }))
  assert.equal(findings.length, 0)
  assert.ok(exec.calls.includes('api repos/{owner}/{repo}/branches/main/protection'))
})

test('doctorRemote: protection via checks field (not contexts) -> zero findings', () => {
  const exec = fakeExec({
    'auth status': 'logged in',
    'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":true}}',
    'api repos/{owner}/{repo}/branches/main/protection': '{"required_status_checks":{"contexts":[],"checks":[{"context":"governance-go-install"}]},"enforce_admins":{"enabled":true}}',
  })
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec }))
  assert.equal(findings.length, 0)
})

// ── The case that decides the ADR: --remote with no credential → not-evaluated, never ok ──
test('doctorRemote: no credential -> not-evaluated, never a finding claiming absence', () => {
  const exec = fakeExec({}, { 'auth status': new Error('gh: not logged into any GitHub hosts') })
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec }))
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, NOT_EVALUATED)
  assert.ok(findings[0].remedy.includes('authenticate'))
  assert.ok(!exec.calls.some(c => c.startsWith('api repos/{owner}/{repo}/branches/')))
})

// ── Token present but insufficient scope — DISTINCT message from no-credential ────────────
test('doctorRemote: insufficient admin scope -> not-evaluated, distinct remedy from no-credential', () => {
  const exec = fakeExec({
    'auth status': 'logged in',
    'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":false}}',
  })
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec }))
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, NOT_EVALUATED)
  assert.ok(!findings[0].remedy.includes('authenticate first'))
  assert.ok(findings[0].remedy.includes('admin access'))
  assert.ok(!exec.calls.some(c => c.startsWith('api repos/{owner}/{repo}/branches/')))
})

test('doctorRemote: gh CLI absent -> not-evaluated, never shells out', () => {
  const exec = fakeExec()
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec, availGH: false }))
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, NOT_EVALUATED)
  assert.equal(exec.calls.length, 0)
})

test('doctorRemote: non-GitHub forge -> not-evaluated, never shells out to gh', () => {
  const exec = fakeExec()
  const deps = baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec })
  deps.execGit = args => {
    if (args.join(' ') === 'remote get-url origin') return { stdout: 'git@gitlab.com:kgsaran/trackfw.git', error: null }
    return { stdout: '', error: new Error('not set') }
  }
  const findings = runDoctorRemote(deps)
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, NOT_EVALUATED)
  assert.equal(exec.calls.length, 0)
})

// ── hooksPath: falsification in both directions ──────────────────────────────────────────
test('doctorRemote: core.hooksPath=/dev/null -> finding', () => {
  const exec = fakeExec({
    'auth status': 'logged in',
    'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":true}}',
    'api repos/{owner}/{repo}/branches/main/protection': '{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}',
  })
  const findings = runDoctorRemote(baseDeps({ hooksPath: '/dev/null', execForgeAPI: exec }))
  assert.ok(hasKind(findings, HOOKS_PATH_NEUTRALIZED))
})

test('doctorRemote: core.hooksPath unset -> no finding (control)', () => {
  const exec = fakeExec({
    'auth status': 'logged in',
    'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":true}}',
    'api repos/{owner}/{repo}/branches/main/protection': '{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}',
  })
  const findings = runDoctorRemote(baseDeps({ hooksPathError: new Error('not set'), execForgeAPI: exec }))
  assert.ok(!hasKind(findings, HOOKS_PATH_NEUTRALIZED))
})

test('doctorRemote: core.hooksPath=.husky/_ (legitimate) -> no finding', () => {
  const exec = fakeExec({
    'auth status': 'logged in',
    'api repos/{owner}/{repo}': '{"default_branch":"main","permissions":{"admin":true}}',
    'api repos/{owner}/{repo}/branches/main/protection': '{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}',
  })
  const findings = runDoctorRemote(baseDeps({ hooksPath: '.husky/_', execForgeAPI: exec }))
  assert.ok(!hasKind(findings, HOOKS_PATH_NEUTRALIZED))
})
