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

test('checksum is the SHA-256 hex digest of the raw bytes', () => {
  const crypto = require('node:crypto')
  const content = '# Hello\n\nSome deterministic content.\n'
  const want = crypto.createHash('sha256').update(Buffer.from(content, 'utf8')).digest('hex')
  assert.equal(checksum(content), want)
  assert.equal(checksum(content), checksum(content))
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
