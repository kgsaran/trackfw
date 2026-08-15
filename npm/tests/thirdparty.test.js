'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const thirdpartyCmd = require('../src/commands/thirdparty')
const { createLifecycleCommand } = require('../src/commands/integrations')
const { checkMarkers, checksum } = require('../src/thirdparty/markers')
const provenance = require('../src/thirdparty/provenance')
const validator = require('../src/validator')
const fetchMod = require('../src/thirdparty/fetch')

const BENIGN_CONTENT = '# Example Third-Party Skill\n\nSome helpful, benign content for the agent to consume.\n'

function tmpHome() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-thirdparty-home-'))
}

function tmpProject() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-thirdparty-project-'))
}

// runInProject executa fn com HOME e cwd redirecionados para diretórios
// temporários isolados, nunca tocando o ~ real. Restaura ambos ao final.
// Mirrors npm/tests/identity-wizard.test.js:runInProject.
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

// withOrchestratorSession sets TRACKFW_ORCHESTRATOR_SESSION so tests can
// exercise fetch/install past the D2 guardrail; the guardrail test below
// deliberately does NOT use this helper. Mirrors
// internal/commands/integrations_thirdparty_test.go:withOrchestratorSession.
function withOrchestratorSession(fn) {
  const had = Object.prototype.hasOwnProperty.call(process.env, 'TRACKFW_ORCHESTRATOR_SESSION')
  const old = process.env.TRACKFW_ORCHESTRATOR_SESSION
  process.env.TRACKFW_ORCHESTRATOR_SESSION = '1'
  const restore = () => {
    if (had) process.env.TRACKFW_ORCHESTRATOR_SESSION = old
    else delete process.env.TRACKFW_ORCHESTRATOR_SESSION
  }
  if (typeof fn !== 'function') return restore
  try {
    const result = fn()
    if (result && typeof result.finally === 'function') return result.finally(restore)
    restore()
    return result
  } catch (err) {
    restore()
    throw err
  }
}

function stubThirdPartyFetch(content) {
  const old = thirdpartyCmd.thirdPartyFetch
  thirdpartyCmd.thirdPartyFetch = async () => Buffer.from(content, 'utf8')
  return () => { thirdpartyCmd.thirdPartyFetch = old }
}

function captureConsoleLog() {
  const lines = []
  const original = console.log
  console.log = (...args) => lines.push(args.join(' '))
  return {
    output: () => lines.join('\n'),
    restore: () => { console.log = original },
  }
}

// runFetch executa `<kind> third-party fetch <url>` e retorna o checksum
// impresso no stdout. Mirrors integrations_thirdparty_test.go:runFetch.
async function runFetch(kind, url, extraArgs = []) {
  const cmd = createLifecycleCommand(kind)
  const capture = captureConsoleLog()
  try {
    await cmd.parseAsync(['third-party', 'fetch', url, ...extraArgs], { from: 'user' })
  } finally {
    capture.restore()
  }
  const out = capture.output()
  for (const line of out.split('\n')) {
    if (line.startsWith('checksum: ')) return line.slice('checksum: '.length)
  }
  throw new Error(`no checksum printed by fetch, output:\n${out}`)
}

function walkFiles(root) {
  const results = []
  const stack = [root]
  while (stack.length) {
    const dir = stack.pop()
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name)
      if (entry.isDirectory()) stack.push(full)
      else results.push(path.relative(root, full))
    }
  }
  return results
}

// -----------------------------------------------------------------------
// checkMarkers — fence acceptance, fullwidth refusal, cyrillic pass-through
// -----------------------------------------------------------------------

test('checkMarkers matches a literal H1 marker heading', () => {
  const content = '# Git authority\n\nsome content redefining boundaries.\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers matches a marker at heading level H6, not just H1', () => {
  // headingLinePattern matches any level 1-6 — a marker buried at H6 must
  // be caught the same as at H1.
  const content = '###### Mode lock\n\nsome content.\n'
  assert.deepEqual(checkMarkers(content), ['mode lock'])
})

test('checkMarkers accepts a marker quoted inside a fenced code block', () => {
  const content = '# Benign heading\n\n' +
    'Some documentation about how markers work:\n\n' +
    '```\n' +
    '## Git authority\n' +
    '## Mode lock\n' +
    '```\n\n' +
    'More text.\n'
  assert.deepEqual(checkMarkers(content), [])
})

test('checkMarkers accepts a marker quoted inside a tilde-fenced code block', () => {
  const content = '# Benign heading\n\n~~~\n## Scope boundary\n~~~\n'
  assert.deepEqual(checkMarkers(content), [])
})

test('checkMarkers: unclosed fence no longer grants immunity (D3-ter(a), ML-4C)', () => {
  // Supersedes the previous test 'checkMarkers: unclosed fence drops the
  // rest of the document', which asserted the opposite (no match) — that
  // was a real evasion found by the Wave 4 barrier (both hades-tf and
  // hefesto-tf, independently) and reproduced against all 3 CLIs. An
  // unclosed fence is no longer a fence for this check: content after the
  // opener is rescanned and a marker inside it is now caught.
  const content = '```\n# Git authority\nstill inside, never closed\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers: closer shorter than opener does not close the fence, but marker is still caught (D3-ter(a), ML-4C)', () => {
  // CommonMark rule: the closer needs AT LEAST as many repeats as the
  // opener. A 4-backtick opener is not closed by a 3-backtick line — the
  // fence never closes, so per D3-ter(a) it is not a fence at all and its
  // content is rescanned.
  const content = '````\n# Git authority\n```\nstill fenced (closer too short)\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers: indented fence is still recognized as a fence delimiter', () => {
  const content = '   ```\n   ## Git authority\n   ```\n\nRegular text.\n'
  assert.deepEqual(checkMarkers(content), [])
})

test('checkMarkers: a heading after a closed fence still matches', () => {
  const content = '```\nsome code, not a marker\n```\n\n## Git authority\n\nRegular text.\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers refuses fullwidth compatibility characters (NFKC folds to ASCII)', () => {
  // U+FF03 FULLWIDTH NUMBER SIGN, U+FF27 FULLWIDTH LATIN CAPITAL LETTER G —
  // NFKC folds both to ASCII "#" and "G". This is exactly what NFKC (step 3)
  // is meant to defeat.
  const content = '＃＃ Ｇit authority\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers PASSES a cyrillic homoglyph heading — documented D3 gap, not a bug', () => {
  // U+0430 CYRILLIC SMALL LETTER A in place of Latin "a" in "authority".
  // NFKC does NOT fold cross-script homoglyphs — documented as an explicit,
  // deliberate gap in D3 ("o que este critério NÃO cobre"). Content PASSES.
  const content = '## Git аuthority\n'
  assert.deepEqual(checkMarkers(content), [])
})

test('checkMarkers: marker inside a neutralized HTML comment still matches (D3-ter(b), ML-4C)', () => {
  // Supersedes the Go-only 'HTMLCommentStrippedBeforeMatch' assertion —
  // that assertion contradicted D3's own written justification for step 1
  // ("an LLM reads HTML comments in the token stream") and was reproduced
  // as a real evasion: `<!-- ## Git authority -->` passed clean. Step 1
  // now strips only the comment delimiters, keeping the inner text in
  // place to be scanned.
  const content = '<!-- ## Git authority -->\n# Benign heading\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers: marker inside a multi-line HTML comment still matches (D3-ter(b), ML-4C)', () => {
  const content = '<!--\n## Git authority\nsome other commented-out text\n-->\n# Benign heading\n'
  assert.deepEqual(checkMarkers(content), ['git authority'])
})

test('checkMarkers: benign HTML comment text stays benign (D3-ter(b), ML-4C)', () => {
  const content = '<!-- just an ordinary editorial note, nothing boundary-related -->\n# Benign heading\n'
  assert.deepEqual(checkMarkers(content), [])
})

test('checkMarkers: casefold is simple lowercase, not full Unicode casefold (D3-ter(c), ML-4C)', () => {
  // Pins step 4's chosen semantics — unified across the 3 CLIs so none of
  // them silently diverges on a normalization step feeding a security
  // check. No known exploit against the 6 ASCII markers either way; German
  // sharp S (ß) is the textbook divergence case (ß.toLowerCase() stays
  // "ß"; a full Unicode casefold would turn it into "ss") and is used here
  // only to pin which semantics is in effect.
  const content = '# Straße\n\nAn unrelated heading using a German sharp S.\n'
  assert.deepEqual(checkMarkers(content), [])
})

test('checkMarkers: the security opinion document does not refuse itself (non-regression, ML-4C)', () => {
  // Falsification test named by the ML-4C AC: the D3-ter(a)/(b) fixes above
  // must NOT reintroduce the exact self-refusal the original D3 amendment
  // (fenced-block removal) exists to prevent. The opinion document itself
  // lists all 6 literal markers as headings, but inside a properly CLOSED
  // fence — running the checker against the real file must still return
  // zero matches.
  const docPath = path.join(__dirname, '..', '..', 'docs', 'seguranca', '2026-08-15-skills-de-terceiro-via-url.md')
  const content = fs.readFileSync(docPath, 'utf8')
  assert.deepEqual(checkMarkers(content), [])
})

test('checksum is the SHA-256 hex digest of the raw bytes', () => {
  const crypto = require('node:crypto')
  const content = '# Hello\n\nSome deterministic content.\n'
  const want = crypto.createHash('sha256').update(Buffer.from(content, 'utf8')).digest('hex')
  assert.equal(checksum(content), want)
  assert.equal(checksum(content), checksum(content))
})

// -----------------------------------------------------------------------
// references.js — end < start guard (hefesto-tf finding, ML-4C)
// -----------------------------------------------------------------------

test('applyThirdPartyReferences treats an end marker occurring before start as malformed, not corruption', () => {
  const { applyThirdPartyReferences, upsertThirdPartyReference, THIRD_PARTY_REF_START, THIRD_PARTY_REF_END } = require('../src/thirdparty/references')
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-thirdparty-refs-'))
  upsertThirdPartyReference(root, 'claude', 'backend', {
    slug: 'my-skill', destination: '.claude/skills/thirdparty/my-skill.md', url: 'https://example.com/my-skill.md',
  })

  // A stray end marker appears BEFORE the genuine start marker, with no end
  // marker after it — the exact shape that used to produce end < start
  // when applyThirdPartyReferences searched the whole text instead of
  // anchoring the search at start.
  const content = `${THIRD_PARTY_REF_END}\n\nUnrelated leftover text.\n\n${THIRD_PARTY_REF_START}\nstale content, no closing marker\n`

  const got = applyThirdPartyReferences(root, content, 'claude', 'backend')
  assert.match(got, /my-skill/)
  assert.match(got, /https:\/\/example\.com\/my-skill\.md/)
  assert.match(got, /Unrelated leftover text\./)
})

// -----------------------------------------------------------------------
// fetch.js — D7 network policy (requestOnce substituted, no real socket)
// -----------------------------------------------------------------------

test('fetch refuses a non-200, non-redirect HTTP status (hefesto-tf finding, ML-4C)', async () => {
  // The resp.statusCode !== 200 branch in fetchOnce existed since ML-2A but
  // was never exercised by any test in this stack (present in Python's
  // suite already). requestOnce is substituted at the same module-level
  // indirection point ADR-2026-08-15's Go reference uses for fetchClient.
  const old = fetchMod.requestOnce
  fetchMod.requestOnce = async () => ({
    statusCode: 404,
    headers: { 'content-type': 'text/plain' },
    body: Buffer.from('not found'),
    tooLarge: false,
  })
  try {
    await assert.rejects(
      fetchMod.fetch('https://example.com/skills/missing.md'),
      /404/,
    )
  } finally {
    fetchMod.requestOnce = old
  }
})

// -----------------------------------------------------------------------
// redactURL (D6-bis)
// -----------------------------------------------------------------------

test('redactURL strips the query string', () => {
  const { redactURL } = require('../src/thirdparty/markers')
  const got = redactURL('https://example.com/skills/my-skill.md?token=abc123')
  assert.equal(got, 'https://example.com/skills/my-skill.md?[redacted]')
  assert.equal(got.includes('abc123'), false)
})

test('redactURL strips userinfo', () => {
  const { redactURL } = require('../src/thirdparty/markers')
  const got = redactURL('https://user:supersecret@example.com/skills/my-skill.md')
  assert.equal(got, 'https://example.com/skills/my-skill.md')
  assert.equal(got.includes('supersecret'), false)
})

test('redactURL leaves a URL with no query or userinfo unchanged', () => {
  const { redactURL } = require('../src/thirdparty/markers')
  const got = redactURL('https://example.com/skills/my-skill.md')
  assert.equal(got, 'https://example.com/skills/my-skill.md')
})

test('redactURL is idempotent', () => {
  const { redactURL } = require('../src/thirdparty/markers')
  const once = redactURL('https://example.com/skills/my-skill.md?token=abc123')
  const twice = redactURL(once)
  assert.equal(once, twice)
})

// -----------------------------------------------------------------------
// third-party fetch — quarantine-only writes, marker refusal, guardrail
// -----------------------------------------------------------------------

test('third-party fetch never writes outside the quarantine directory', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const checksumValue = await runFetch('skills', 'https://example.com/skills/my-skill.md')
      const quarantinePath = path.join(project, '.trackfw', 'thirdparty-quarantine', `${checksumValue}.json`)
      assert.equal(fs.existsSync(quarantinePath), true)

      const unexpected = walkFiles(project).filter(rel => !rel.startsWith(path.join('.trackfw', 'thirdparty-quarantine')))
      assert.deepEqual(unexpected, [])
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('third-party fetch redacts the query string in the quarantine record (D6-bis)', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const checksumValue = await runFetch('skills', 'https://example.com/skills/my-skill.md?token=super-secret-value')
      const quarantinePath = path.join(project, '.trackfw', 'thirdparty-quarantine', `${checksumValue}.json`)
      const raw = fs.readFileSync(quarantinePath, 'utf8')
      assert.equal(raw.includes('super-secret-value'), false)
      assert.equal(raw.includes('[redacted]'), true)
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('third-party fetch refuses a marker-matching artifact by default', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch('# Git authority\n\nsome content redefining boundaries.\n')
    try {
      const cmd = createLifecycleCommand('skills')
      await assert.rejects(
        cmd.parseAsync(['third-party', 'fetch', 'https://example.com/skills/evil.md'], { from: 'user' }),
        err => {
          assert.match(err.message.toLowerCase(), /git authority/)
          return true
        },
      )
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('third-party fetch refuses without TRACKFW_ORCHESTRATOR_SESSION', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const had = Object.prototype.hasOwnProperty.call(process.env, 'TRACKFW_ORCHESTRATOR_SESSION')
    const old = process.env.TRACKFW_ORCHESTRATOR_SESSION
    delete process.env.TRACKFW_ORCHESTRATOR_SESSION
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const cmd = createLifecycleCommand('skills')
      await assert.rejects(
        cmd.parseAsync(['third-party', 'fetch', 'https://example.com/skills/my-skill.md'], { from: 'user' }),
        err => {
          assert.match(err.message, /guardrail/)
          assert.match(err.message, new RegExp(thirdpartyCmd.THIRD_PARTY_PROVENANCE_RULE))
          assert.match(err.message, /not a security control/)
          return true
        },
      )
    } finally {
      restoreFetch()
      if (had) process.env.TRACKFW_ORCHESTRATOR_SESSION = old
      else delete process.env.TRACKFW_ORCHESTRATOR_SESSION
    }
  })
})

// -----------------------------------------------------------------------
// third-party install — approval, TOCTOU, byte-identical attach, AC5, D4
// -----------------------------------------------------------------------

test('third-party install fails without a provenance approval', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const checksumValue = await runFetch('skills', 'https://example.com/skills/my-skill.md')
      const cmd = createLifecycleCommand('skills')
      await assert.rejects(
        cmd.parseAsync(['third-party', 'install', '--checksum', checksumValue, '--targets', 'claude', '--yes-i-trust-this-source'], { from: 'user' }),
        /not approved/,
      )
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('third-party install fails on TOCTOU checksum mismatch', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      // Tamper the quarantine record in place: same filename (still named by
      // the ORIGINAL checksum), different content_base64.
      const quarantinePath = path.join(project, '.trackfw', 'thirdparty-quarantine', `${checksumValue}.json`)
      const record = JSON.parse(fs.readFileSync(quarantinePath, 'utf8'))
      record.content_base64 = Buffer.from('tampered-content', 'utf8').toString('base64')
      fs.writeFileSync(quarantinePath, JSON.stringify(record), 'utf8')

      const cmd = createLifecycleCommand('skills')
      await assert.rejects(
        cmd.parseAsync(['third-party', 'install', '--checksum', checksumValue, '--targets', 'claude', '--yes-i-trust-this-source'], { from: 'user' }),
        err => {
          assert.match(err.message, /(TOCTOU|checksum)/)
          return true
        },
      )
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('third-party install with --apply-to leaves the catalog agent file byte-identical outside the marker block', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    try {
      const agentsInstall = createLifecycleCommand('agents')
      await agentsInstall.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })
      const agentPath = path.join(project, '.claude', 'agents', 'trackfw-backend.md')
      const before = fs.readFileSync(agentPath, 'utf8')

      const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      const install = createLifecycleCommand('skills')
      const capture = captureConsoleLog()
      try {
        await install.parseAsync([
          'third-party', 'install', '--checksum', checksumValue, '--targets', 'claude',
          '--apply-to', 'backend', '--yes-i-trust-this-source',
        ], { from: 'user' })
      } finally {
        capture.restore()
        restoreFetch()
      }

      const after = fs.readFileSync(agentPath, 'utf8')
      const start = '<!-- trackfw:thirdparty-skills:start -->'
      const end = '<!-- trackfw:thirdparty-skills:end -->'
      const blockStart = after.indexOf(start)
      const blockEnd = after.indexOf(end)
      assert.notEqual(blockStart, -1)
      assert.notEqual(blockEnd, -1)

      const excised = `${after.slice(0, blockStart).replace(/\n+$/, '')}\n`
      assert.equal(excised, before)
      assert.match(after, new RegExp(dest.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))

      const skillPath = path.join(project, '.claude', 'skills', 'thirdparty', 'my-skill.md')
      assert.equal(fs.existsSync(skillPath), true)
      assert.match(fs.readFileSync(skillPath, 'utf8'), /Example Third-Party Skill/)
    } finally {
      restoreEnv()
    }
  })
})

test('a plain `agents update` after attach stays state=current and rewrites nothing (AC5)', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    try {
      const agentsInstall = createLifecycleCommand('agents')
      await agentsInstall.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })

      const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      const install = createLifecycleCommand('skills')
      const installCapture = captureConsoleLog()
      try {
        await install.parseAsync([
          'third-party', 'install', '--checksum', checksumValue, '--targets', 'claude',
          '--apply-to', 'backend', '--yes-i-trust-this-source',
        ], { from: 'user' })
      } finally {
        installCapture.restore()
        restoreFetch()
      }

      const agentPath = path.join(project, '.claude', 'agents', 'trackfw-backend.md')
      const attached = fs.readFileSync(agentPath, 'utf8')

      const update = createLifecycleCommand('agents')
      const updateCapture = captureConsoleLog()
      try {
        await update.parseAsync(['update', '--targets', 'claude', '--items', 'backend', '--scope', 'project', '--json'], { from: 'user' })
      } finally {
        updateCapture.restore()
      }
      const output = JSON.parse(updateCapture.output())
      assert.equal(output.deployments.length, 1)
      assert.notEqual(output.deployments[0].state, 'modified')
      assert.equal(output.deployments[0].state, 'current')

      const afterUpdate = fs.readFileSync(agentPath, 'utf8')
      assert.equal(afterUpdate, attached)
    } finally {
      restoreEnv()
    }
  })
})

test('third-party install defaults to project scope, never global, when --scope is omitted', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      const install = createLifecycleCommand('skills')
      const capture = captureConsoleLog()
      try {
        // Deliberately no --scope flag: must default to project (D4),
        // unlike `skills install`/`agents install`, which default to
        // global and are asserted unaffected by agents-skills.test.js.
        await install.parseAsync(['third-party', 'install', '--checksum', checksumValue, '--targets', 'claude', '--yes-i-trust-this-source'], { from: 'user' })
      } finally {
        capture.restore()
      }

      assert.equal(fs.existsSync(path.join(project, '.claude', 'skills', 'thirdparty', 'my-skill.md')), true)
      assert.equal(fs.existsSync(path.join(home, '.claude', 'skills', 'thirdparty', 'my-skill.md')), false)
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('third-party install --scope global requires its own confirmation, distinct from --yes-i-trust-this-source (D4-bis)', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      // Global scope resolves to a "~/"-prefixed destination string,
      // distinct from project scope's project-relative one.
      const dest = '~/.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'global',
      })

      const install1 = createLifecycleCommand('skills')
      const capture1 = captureConsoleLog()
      let err1
      try {
        await install1.parseAsync(['third-party', 'install', '--checksum', checksumValue, '--targets', 'claude', '--scope', 'global', '--yes-i-trust-this-source'], { from: 'user' })
      } catch (err) {
        err1 = err
      } finally {
        capture1.restore()
      }
      assert.ok(err1, 'expected install to fail with --yes-i-trust-this-source alone for --scope global')
      assert.match(err1.message, /yes-global-scope-unverified/)
      assert.match(capture1.output(), /trackfw validate/)
      assert.equal(fs.existsSync(path.join(home, '.claude', 'skills', 'thirdparty', 'my-skill.md')), false)

      const install2 = createLifecycleCommand('skills')
      const capture2 = captureConsoleLog()
      try {
        await install2.parseAsync(['third-party', 'install', '--checksum', checksumValue, '--targets', 'claude', '--scope', 'global', '--yes-i-trust-this-source', '--yes-global-scope-unverified'], { from: 'user' })
      } finally {
        capture2.restore()
      }
      assert.match(capture2.output(), /trackfw validate/)
      assert.equal(fs.existsSync(path.join(home, '.claude', 'skills', 'thirdparty', 'my-skill.md')), true)
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

test('--apply-to rejects a hand-modified agent artifact before any write (no partial state)', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    try {
      const agentsInstall = createLifecycleCommand('agents')
      await agentsInstall.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })
      const agentPath = path.join(project, '.claude', 'agents', 'trackfw-backend.md')
      fs.writeFileSync(agentPath, 'hand-edited content, not trackfw-managed anymore\n')

      const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      const install = createLifecycleCommand('skills')
      try {
        await assert.rejects(
          install.parseAsync([
            'third-party', 'install', '--checksum', checksumValue, '--targets', 'claude',
            '--apply-to', 'backend', '--yes-i-trust-this-source',
          ], { from: 'user' }),
          /modified/,
        )
      } finally {
        restoreFetch()
      }

      assert.equal(fs.existsSync(path.join(project, '.claude', 'skills', 'thirdparty', 'my-skill.md')), false)
      assert.equal(fs.existsSync(path.join(project, '.trackfw', 'thirdparty-references.json')), false)
    } finally {
      restoreEnv()
    }
  })
})

test('third-party subcommand is reachable from both `agents` and `skills`', () => {
  for (const kind of ['agents', 'skills']) {
    const root = createLifecycleCommand(kind)
    const thirdParty = root.commands.find(cmd => cmd.name() === 'third-party')
    assert.ok(thirdParty, `${kind} is missing the third-party subcommand`)
    for (const sub of ['fetch', 'install']) {
      assert.ok(thirdParty.commands.find(cmd => cmd.name() === sub), `${kind} third-party is missing ${sub}`)
    }
  }
})

test('third-party install via `agents third-party` still lands the artifact under skills/thirdparty (D5)', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
    try {
      const fetchCmd = createLifecycleCommand('agents')
      const fetchCapture = captureConsoleLog()
      let checksumValue
      try {
        await fetchCmd.parseAsync(['third-party', 'fetch', 'https://example.com/skills/my-skill.md'], { from: 'user' })
        for (const line of fetchCapture.output().split('\n')) {
          if (line.startsWith('checksum: ')) checksumValue = line.slice('checksum: '.length)
        }
      } finally {
        fetchCapture.restore()
      }
      assert.ok(checksumValue, 'no checksum printed by agents third-party fetch')

      const quarantinePath = path.join(project, '.trackfw', 'thirdparty-quarantine', `${checksumValue}.json`)
      const record = JSON.parse(fs.readFileSync(quarantinePath, 'utf8'))
      assert.equal(record.kind, 'agent')

      const url = 'https://example.com/skills/my-skill.md'
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      const install = createLifecycleCommand('agents')
      const installCapture = captureConsoleLog()
      try {
        await install.parseAsync(['third-party', 'install', '--checksum', checksumValue, '--targets', 'claude', '--yes-i-trust-this-source'], { from: 'user' })
      } finally {
        installCapture.restore()
      }

      assert.equal(fs.existsSync(path.join(project, '.claude', 'skills', 'thirdparty', 'my-skill.md')), true)
      assert.equal(fs.existsSync(path.join(project, '.claude', 'agents', 'thirdparty')), false)
    } finally {
      restoreFetch()
      restoreEnv()
    }
  })
})

// ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas, ML-3A —
// thirdparty_artifact_has_provenance end-to-end, real command path (not hand-authored fixtures).
// Mirrors internal/commands/integrations_thirdparty_validate_test.go — this exists because the
// rule's own unit tests (npm/tests/validator.test.js) hand-author manifest/provenance JSON, and an
// incorrect key-domain assumption baked into BOTH the rule and its fixtures would pass there while
// still being wrong against the real command (exactly what happened in Go during this ML: the rule
// initially looked up provenance by the manifest's ABSOLUTE destination, but
// verifyApproval/upsertProvenanceEntry are actually called with the project-relative destination).
test('third-party install passes thirdparty_artifact_has_provenance end-to-end', async () => {
  const home = tmpHome()
  const project = tmpProject()
  await runInProject(home, project, async () => {
    const restoreEnv = withOrchestratorSession()
    try {
      const agentsInstall = createLifecycleCommand('agents')
      await agentsInstall.parseAsync(['install', '--targets', 'claude', '--items', 'backend', '--scope', 'project'], { from: 'user' })

      const restoreFetch = stubThirdPartyFetch(BENIGN_CONTENT)
      const url = 'https://example.com/skills/my-skill.md'
      const checksumValue = await runFetch('skills', url)
      const dest = '.claude/skills/thirdparty/my-skill.md'
      provenance.upsertProvenanceEntry(project, dest, {
        url, checksum_sha256: checksumValue, installed_at: '2026-08-15T00:00:00Z',
        approved_by: 'hades-tf', review_reference: 'docs/seguranca/test.md', scope: 'project',
      })

      const install = createLifecycleCommand('skills')
      const capture = captureConsoleLog()
      try {
        await install.parseAsync([
          'third-party', 'install', '--checksum', checksumValue, '--targets', 'claude',
          '--apply-to', 'backend', '--yes-i-trust-this-source',
        ], { from: 'user' })
      } finally {
        capture.restore()
        restoreFetch()
      }

      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.deepEqual(msgs, [], 'a correctly approved+installed third-party artifact must not trip the rule')

      // Negative counterpart: tamper the installed file, expect the rule to catch it.
      fs.writeFileSync(path.join(project, dest), '# Example Third-Party Skill\n\nTAMPERED.\n')
      const tamperedMsgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.equal(tamperedMsgs.length, 1, JSON.stringify(tamperedMsgs))
      assert.match(tamperedMsgs[0], /D2 branch ii/)
    } finally {
      restoreEnv()
    }
  })
})
