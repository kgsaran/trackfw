'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const { runReleaseTag } = require('../src/release/runner')

const VERSION = '9.9.9'
const TAG = 'v9.9.9'
const SHA = 'abc123def456'

function validFiles(version) {
  return {
    'internal/version/version.go': `package version\n\nvar Version = "${version}"\n`,
    'npm/package.json': JSON.stringify({ name: 'trackfw', version }),
    'pypi/pyproject.toml': `[project]\nname = "trackfw"\nversion = "${version}"\n`,
    'pypi/trackfw/__init__.py':
      'try:\n    from importlib.metadata import version\n' +
      `    __version__ = version("trackfw") or "${version}"\n` +
      `except Exception:\n    __version__ = "${version}"\n`,
    'CHANGELOG.md': `# Changelog\n\n## [${version}] - 2026-08-19\n\n### Added\n- x\n`,
  }
}

function makeMockGit(overrides = {}) {
  const responses = {
    'status --porcelain': '',
    'fetch origin --prune': '',
    'symbolic-ref refs/remotes/origin/HEAD': 'refs/remotes/origin/main',
    'rev-parse origin/main': SHA,
    'remote get-url origin': 'https://github.com/kgsaran/trackfw.git',
    'config user.name': 'Test User',
    'config user.email': 'test@example.com',
    [`ls-remote --tags origin refs/tags/${TAG}`]: '',
  }
  const errors = {
    'rev-parse -q --verify refs/heads/main': new Error('no such branch'),
    [`rev-parse -q --verify refs/tags/${TAG}`]: new Error('no such tag'),
  }
  Object.assign(responses, overrides.responses || {})
  Object.assign(errors, overrides.errors || {})
  for (const k of Object.keys(overrides.clearErrors || {})) delete errors[k]

  const calls = []
  function execGit(args) {
    calls.push(args.slice())
    const key = args.join(' ')
    if (key in errors) return { stdout: '', error: errors[key] }
    if (key in responses) return { stdout: responses[key], error: null }
    return { stdout: '', error: null }
  }
  execGit.calls = calls
  execGit.responses = responses
  execGit.errors = errors
  return execGit
}

function makeDeps({ fileOverrides = {}, gitOverrides = {}, availFn = () => true, execForgeAPI = null } = {}) {
  const files = { ...validFiles(VERSION), ...fileOverrides }
  const execGit = makeMockGit(gitOverrides)
  const outLines = []
  const errLines = []
  const deps = {
    execGit,
    readFile: (p) => {
      if (!(p in files)) throw new Error(`file not found: ${p}`)
      return files[p]
    },
    writeln: (s) => outLines.push(s),
    writeErr: (s) => errLines.push(s),
    configForge: '',
    repoDir: '',
    availFn,
    execForgeAPI:
      execForgeAPI ||
      ((name, args, stdin) => {
        if (args[1].includes('git/tags')) return { stdout: '{"sha":"tagobjectsha000"}', error: null }
        return { stdout: '{}', error: null }
      }),
  }
  return { deps, execGit, outLines, errLines }
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 1 — clean working tree
// ────────────────────────────────────────────────────────────────────────────

test('release tag: dirty tree aborts', () => {
  const { deps, errLines } = makeDeps({ gitOverrides: { responses: { 'status --porcelain': ' M some/file.go\n' } } })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /working tree is not clean/)
})

// ────────────────────────────────────────────────────────────────────────────
// Precondition 2 — default branch up to date with origin
// ────────────────────────────────────────────────────────────────────────────

test('release tag: fetch failure aborts', () => {
  const { deps, errLines } = makeDeps({ gitOverrides: { errors: { 'fetch origin --prune': new Error('could not connect') } } })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /could not fetch origin/)
})

test('release tag: stale local main aborts', () => {
  const { deps, errLines } = makeDeps({
    gitOverrides: {
      clearErrors: { 'rev-parse -q --verify refs/heads/main': true },
      responses: { 'rev-parse -q --verify refs/heads/main': '', 'rev-parse refs/heads/main': 'stalesha000' },
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /not up to date with origin\/main/)
})

test('release tag: local main matching origin is not blocked', () => {
  const { deps, errLines } = makeDeps({
    gitOverrides: {
      clearErrors: { 'rev-parse -q --verify refs/heads/main': true },
      responses: { 'rev-parse -q --verify refs/heads/main': '', 'rev-parse refs/heads/main': SHA },
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 0, errLines.join('\n'))
})

test('release tag: no local main branch is not blocked', () => {
  const { deps, errLines } = makeDeps()
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 0, errLines.join('\n'))
})

// ────────────────────────────────────────────────────────────────────────────
// Precondition 3 — the 4 version files must all match
// ────────────────────────────────────────────────────────────────────────────

test('release tag: mismatched Go version names the file', () => {
  const { deps, errLines } = makeDeps({
    fileOverrides: { 'internal/version/version.go': 'package version\n\nvar Version = "0.0.1"\n' },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /internal\/version\/version\.go/)
  assert.match(errLines[0], /"0\.0\.1"/)
  assert.match(errLines[0], new RegExp(VERSION))
})

test('release tag: mismatched npm version names the file', () => {
  const { deps, errLines } = makeDeps({
    fileOverrides: { 'npm/package.json': JSON.stringify({ name: 'trackfw', version: '0.0.1' }) },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /npm\/package\.json/)
})

test('release tag: mismatched pyproject version names the file', () => {
  const { deps, errLines } = makeDeps({
    fileOverrides: { 'pypi/pyproject.toml': '[project]\nversion = "0.0.1"\n' },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /pypi\/pyproject\.toml/)
})

test('release tag: mismatched __init__.py try-block fallback names it', () => {
  const { deps, errLines } = makeDeps({
    fileOverrides: {
      'pypi/trackfw/__init__.py':
        'try:\n    from importlib.metadata import version\n' +
        '    __version__ = version("trackfw") or "0.0.1"\n' +
        `except Exception:\n    __version__ = "${VERSION}"\n`,
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /importlib\.metadata fallback/)
})

test('release tag: mismatched __init__.py except-block fallback names it', () => {
  const { deps, errLines } = makeDeps({
    fileOverrides: {
      'pypi/trackfw/__init__.py':
        'try:\n    from importlib.metadata import version\n' +
        `    __version__ = version("trackfw") or "${VERSION}"\n` +
        'except Exception:\n    __version__ = "0.0.1"\n',
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /except fallback/)
})

test('release tag: accepts a leading "v" on the CLI argument', () => {
  const { deps, errLines } = makeDeps()
  const code = runReleaseTag(`v${VERSION}`, deps)
  assert.equal(code, 0, errLines.join('\n'))
})

// ────────────────────────────────────────────────────────────────────────────
// Precondition 4 — CHANGELOG.md must have the version's section
// ────────────────────────────────────────────────────────────────────────────

test('release tag: missing CHANGELOG section aborts', () => {
  const { deps, errLines } = makeDeps({
    fileOverrides: { 'CHANGELOG.md': '# Changelog\n\n## [1.0.0] - 2020-01-01\n\n### Added\n- x\n' },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], new RegExp(VERSION))
  assert.match(errLines[0], /not found in CHANGELOG\.md/)
})

// ────────────────────────────────────────────────────────────────────────────
// Precondition 5 — tag must not already exist, local or remote
// ────────────────────────────────────────────────────────────────────────────

test('release tag: existing local tag aborts', () => {
  const { deps, errLines } = makeDeps({
    gitOverrides: {
      clearErrors: { [`rev-parse -q --verify refs/tags/${TAG}`]: true },
      responses: { [`rev-parse -q --verify refs/tags/${TAG}`]: SHA },
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], new RegExp(TAG))
  assert.match(errLines[0], /already exists locally/)
})

test('release tag: existing remote tag aborts', () => {
  const { deps, errLines } = makeDeps({
    gitOverrides: { responses: { [`ls-remote --tags origin refs/tags/${TAG}`]: `${SHA}\trefs/tags/${TAG}` } },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], new RegExp(TAG))
  assert.match(errLines[0], /already exists on origin/)
})

// ────────────────────────────────────────────────────────────────────────────
// Precondition 6 — forge CLI available, GitHub only
// ────────────────────────────────────────────────────────────────────────────

test('release tag: no forge CLI aborts', () => {
  const { deps, errLines } = makeDeps({ availFn: () => false })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /requires the GitHub CLI \(gh\)/)
  assert.match(errLines[0], new RegExp(`git tag -a ${TAG}`))
})

test('release tag: unsupported forge aborts', () => {
  const { deps, errLines } = makeDeps({
    gitOverrides: { responses: { 'remote get-url origin': 'git@gitlab.com:kgsaran/trackfw.git' } },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /currently only supports GitHub/)
  assert.match(errLines[0], /gitlab/)
})

test('release tag: manual (unresolved) forge aborts', () => {
  const { deps, errLines } = makeDeps({
    gitOverrides: { responses: { 'remote get-url origin': 'git@example.internal:kgsaran/trackfw.git' } },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /resolved forge: "manual"/)
})

// ────────────────────────────────────────────────────────────────────────────
// Git identity
// ────────────────────────────────────────────────────────────────────────────

test('release tag: missing git identity aborts', () => {
  const { deps, errLines } = makeDeps({ gitOverrides: { responses: { 'config user.name': '' } } })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.match(errLines[0], /git config user\.name/)
})

// ────────────────────────────────────────────────────────────────────────────
// Success path — verifies the annotated-tag publish sequence
// ────────────────────────────────────────────────────────────────────────────

test('release tag: success publishes an annotated tag via two gh api calls', () => {
  const calls = []
  const { deps, outLines, errLines } = makeDeps({
    execForgeAPI: (name, args, stdin) => {
      calls.push({ name, args, stdin })
      if (args[1].includes('git/tags')) return { stdout: '{"sha":"tagobjectsha000"}', error: null }
      return { stdout: '{}', error: null }
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 0, errLines.join('\n'))
  assert.equal(calls.length, 2)

  assert.match(calls[0].args[1], /git\/tags/)
  const tagPayload = JSON.parse(calls[0].stdin)
  assert.equal(tagPayload.tag, TAG)
  assert.equal(tagPayload.object, SHA)
  assert.equal(tagPayload.type, 'commit')
  assert.match(tagPayload.message, new RegExp(VERSION))
  assert.equal(tagPayload.tagger.name, 'Test User')
  assert.equal(tagPayload.tagger.email, 'test@example.com')

  assert.match(calls[1].args[1], /git\/refs/)
  const refPayload = JSON.parse(calls[1].stdin)
  assert.equal(refPayload.ref, `refs/tags/${TAG}`)
  assert.equal(refPayload.sha, 'tagobjectsha000')

  assert.ok(outLines.join('\n').includes(TAG))
})

test('release tag: tag object call failure never reaches the ref call', () => {
  let refCalled = false
  const { deps, errLines } = makeDeps({
    execForgeAPI: (name, args) => {
      if (args[1].includes('git/tags')) return { stdout: '', error: new Error('401 Unauthorized') }
      refCalled = true
      return { stdout: '{}', error: null }
    },
  })
  const code = runReleaseTag(VERSION, deps)
  assert.equal(code, 1)
  assert.equal(refCalled, false)
  assert.match(errLines[0], /gh api failed creating the tag object/)
})
