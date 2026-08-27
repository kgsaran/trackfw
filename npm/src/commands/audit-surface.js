'use strict'

// audit-surface.js — `trackfw audit-surface` command (Node.js CLI).
//
// AC16 invariant (no false positives, by construction): this module NEVER reads
// file content looking for hook-path strings. It ONLY opens the 8 exact wiring-file
// paths defined in RUNTIME_WIRING_PATH. Files like docs/cli-parity.md and
// internal/generators/agentfiles.go happen to mention those paths as strings but are
// never opened here — they live at paths not in RUNTIME_WIRING_PATH.

const { Command } = require('commander')
const { execFileSync, spawnSync } = require('child_process')
const crypto = require('crypto')
const path = require('path')

// Runtime names in canonical order (matches check-agent-hooks-parity.sh CLIS).
const CANONICAL_RUNTIMES = ['claude', 'codex', 'gemini', 'copilot', 'cursor', 'kiro', 'windsurf', 'amazonq']

const RUNTIME_WIRING_PATH = {
  claude: '.claude/settings.json',
  codex: '.codex/hooks.json',
  gemini: '.gemini/settings.json',
  copilot: '.github/hooks/trackfw-attention.json',
  cursor: '.cursor/hooks.json',
  kiro: '.kiro/hooks/trackfw-attention.json',
  windsurf: '.windsurf/hooks.json',
  amazonq: '.amazonq/cli-agents/q_cli_default.json',
}

const INSTRUCTION_FILE_PATHS = [
  'CLAUDE.md',
  'AGENTS.md',
  'GEMINI.md',
  '.windsurfrules',
  '.github/copilot-instructions.md',
  '.amazonq/developer/guidelines.md',
  '.cursor/rules/trackfw.mdc',
]

// gitShow reads a file at a given ref via `git show <ref>:<path>`.
// Returns Buffer on success, null if not found.
function gitShow(ref, filePath, gitRoot) {
  const result = spawnSync('git', ['show', `${ref}:${filePath}`], { cwd: gitRoot, maxBuffer: 10 * 1024 * 1024 })
  if (result.status !== 0) return null
  return result.stdout
}

// gitLsTree lists files under a directory at a given ref.
function gitLsTree(ref, dir, gitRoot) {
  const result = spawnSync('git', ['ls-tree', '-r', '--name-only', ref, '--', dir], { cwd: gitRoot })
  if (result.status !== 0) return []
  const lines = result.stdout.toString().replace(/\n$/, '').split('\n').filter(Boolean)
  return lines.sort()
}

// findGitRoot returns the git repository root.
function findGitRoot(cwd) {
  const result = spawnSync('git', ['rev-parse', '--show-toplevel'], { cwd: cwd || process.cwd() })
  if (result.status !== 0) return null
  return result.stdout.toString().trim()
}

// normalizeCommand strips known project-root env-var prefixes and outer quotes
// to produce a repo-relative script path. Returns '' if unresolvable.
function normalizeCommand(rawCmd) {
  let cmd = rawCmd.trim()
  // Strip surrounding double-quotes (Codex format).
  if (cmd.startsWith('"') && cmd.endsWith('"')) {
    cmd = cmd.slice(1, -1)
  }
  const prefixes = [
    '$CLAUDE_PROJECT_DIR/',
    '$GEMINI_PROJECT_DIR/',
    '$(git rev-parse --show-toplevel)/',
  ]
  for (const p of prefixes) {
    if (cmd.startsWith(p)) return cmd.slice(p.length)
  }
  // Accept bare relative script path.
  if (!cmd.includes(' ') && (cmd.endsWith('.sh') || cmd.endsWith('.py') || cmd.endsWith('.js'))) {
    return cmd
  }
  return ''
}

// sha256hex computes SHA-256 of buffer content.
function sha256hex(buf) {
  return 'sha256:' + crypto.createHash('sha256').update(buf).digest('hex')
}

// extractTuples parses wiring-file JSON and returns hook tuples per runtime schema.
function extractTuples(runtime, content) {
  let root
  try {
    root = JSON.parse(content.toString())
  } catch (e) {
    return [{ event: 'parse-error', matcher: '', raw_command: e.message, script_path: '', script_digest: 'unresolvable' }]
  }

  switch (runtime) {
    case 'claude':
    case 'codex':
    case 'amazonq':
    case 'gemini':
      return extractClaudeSchema(root)
    case 'kiro':
      return extractKiroSchema(root)
    case 'copilot':
      return extractCopilotSchema(root)
    case 'cursor':
      return extractCursorSchema(root)
    case 'windsurf':
      return extractWindsurfSchema(root)
    default:
      return []
  }
}

// extractClaudeSchema: {"hooks": {"EVENT": [{"matcher":"...","hooks":[{"command":"...","type":"command"}]}]}}
function extractClaudeSchema(root) {
  const hooks = root.hooks || {}
  const events = Object.keys(hooks).sort()
  const tuples = []
  for (const event of events) {
    const entries = (hooks[event] || []).slice().sort((a, b) => (a.matcher || '') < (b.matcher || '') ? -1 : (a.matcher || '') > (b.matcher || '') ? 1 : 0)
    for (const entry of entries) {
      for (const h of (entry.hooks || [])) {
        if (h.type !== 'command') continue
        tuples.push({
          event,
          matcher: entry.matcher || '',
          raw_command: h.command || '',
          script_path: normalizeCommand(h.command || ''),
          script_digest: '',
        })
      }
    }
  }
  return tuples
}

// extractKiroSchema: {"version":"v1","hooks":[{"trigger":"...","matcher":"...","action":{"type":"command","command":"..."}}]}
function extractKiroSchema(root) {
  const hooks = (root.hooks || []).slice().sort((a, b) => {
    if (a.trigger !== b.trigger) return a.trigger < b.trigger ? -1 : 1
    return (a.matcher || '') < (b.matcher || '') ? -1 : (a.matcher || '') > (b.matcher || '') ? 1 : 0
  })
  return hooks
    .filter(h => h.action && h.action.type === 'command')
    .map(h => ({
      event: h.trigger,
      matcher: h.matcher || '',
      raw_command: h.action.command || '',
      script_path: normalizeCommand(h.action.command || ''),
      script_digest: '',
    }))
}

// extractCopilotSchema: {"version":1,"hooks":{"preToolUse":[{"type":"command","bash":"...","matcher":"..."}],...}}
function extractCopilotSchema(root) {
  const hooks = root.hooks || {}
  const events = Object.keys(hooks).sort()
  const tuples = []
  for (const event of events) {
    const entries = (hooks[event] || []).slice().sort((a, b) => {
      const ma = a.matcher || '', mb = b.matcher || ''
      if (ma !== mb) return ma < mb ? -1 : 1
      return (a.bash || '') < (b.bash || '') ? -1 : (a.bash || '') > (b.bash || '') ? 1 : 0
    })
    for (const entry of entries) {
      if (entry.type !== 'command') continue
      tuples.push({
        event,
        matcher: entry.matcher || '',
        raw_command: entry.bash || '',
        script_path: normalizeCommand(entry.bash || ''),
        script_digest: '',
      })
    }
  }
  return tuples
}

// extractCursorSchema: {"hooks":{"preToolUse":[{"command":"...","matcher":"..."}],...}}
function extractCursorSchema(root) {
  const hooks = root.hooks || {}
  const events = Object.keys(hooks).sort()
  const tuples = []
  for (const event of events) {
    const entries = (hooks[event] || []).slice().sort((a, b) => (a.command || '') < (b.command || '') ? -1 : 1)
    for (const entry of entries) {
      if (!entry.command) continue
      tuples.push({
        event,
        matcher: entry.matcher || '',
        raw_command: entry.command,
        script_path: normalizeCommand(entry.command),
        script_digest: '',
      })
    }
  }
  return tuples
}

// extractWindsurfSchema: {"hooks":{"pre_run_command":[{"command":"...","show_output":true}]}}
function extractWindsurfSchema(root) {
  const hooks = root.hooks || {}
  const events = Object.keys(hooks).sort()
  const tuples = []
  for (const event of events) {
    const entries = (hooks[event] || []).slice().sort((a, b) => (a.command || '') < (b.command || '') ? -1 : 1)
    for (const entry of entries) {
      if (!entry.command) continue
      tuples.push({
        event,
        matcher: '*',
        raw_command: entry.command,
        script_path: normalizeCommand(entry.command),
        script_digest: '',
      })
    }
  }
  return tuples
}

// auditLifecycleHooks checks npm lifecycle hooks and .husky/pre-commit.
function auditLifecycleHooks(ref, gitRoot) {
  const hooks = []
  const npmContent = gitShow(ref, 'npm/package.json', gitRoot)
  if (npmContent) {
    let pkg = {}
    try { pkg = JSON.parse(npmContent.toString()) } catch (_) {}
    const scripts = pkg.scripts || {}
    for (const key of ['preinstall', 'postinstall', 'prepare']) {
      if (key in scripts) {
        hooks.push({ file: 'npm/package.json', key, command: scripts[key], present: true })
      } else {
        hooks.push({ file: 'npm/package.json', key, present: false })
      }
    }
  } else {
    for (const key of ['preinstall', 'postinstall', 'prepare']) {
      hooks.push({ file: 'npm/package.json', key, present: false })
    }
  }

  const huskyContent = gitShow(ref, '.husky/pre-commit', gitRoot)
  if (huskyContent) {
    const cmd = extractHuskyCommand(huskyContent.toString())
    hooks.push({ file: '.husky/pre-commit', key: 'pre-commit', command: cmd, present: true })
  } else {
    hooks.push({ file: '.husky/pre-commit', key: 'pre-commit', present: false })
  }
  return hooks
}

function extractHuskyCommand(content) {
  for (const line of content.split('\n')) {
    const t = line.trim()
    if (!t || t.startsWith('#')) continue
    return t
  }
  return ''
}

// runAuditSurface performs the full audit and returns a report object.
function runAuditSurface(ref, base, gitRoot) {
  const report = { ref, hook_wiring: [], instruction_files: [], lifecycle_hooks: [] }
  if (base) report.base = base

  // 1. Hook wiring — 8 runtimes in canonical order.
  for (const runtime of CANONICAL_RUNTIMES) {
    const wiringFile = RUNTIME_WIRING_PATH[runtime]
    const content = gitShow(ref, wiringFile, gitRoot)
    if (!content) {
      report.hook_wiring.push({ runtime, wiring_file: wiringFile, present: false, tuples: [] })
      continue
    }
    const tuples = extractTuples(runtime, content)
    // Compute digests for each tuple's script.
    for (const t of tuples) {
      if (t.script_path) {
        const scriptBytes = gitShow(ref, t.script_path, gitRoot)
        t.script_digest = scriptBytes ? sha256hex(scriptBytes) : 'not-found'
      } else {
        t.script_digest = 'unresolvable'
      }
    }
    report.hook_wiring.push({ runtime, wiring_file: wiringFile, present: true, tuples })
  }

  // 2. Instruction files (agent-config kind).
  for (const p of INSTRUCTION_FILE_PATHS) {
    const content = gitShow(ref, p, gitRoot)
    report.instruction_files.push({ path: p, kind: 'agent-config', present: content !== null })
  }

  // 3. Slash commands (.claude/commands/**/*.md).
  const slashFiles = gitLsTree(ref, '.claude/commands', gitRoot).filter(f => f.endsWith('.md'))
  for (const f of slashFiles) {
    report.instruction_files.push({ path: f, kind: 'slash-command', present: true })
  }

  // 4. Lifecycle hooks.
  report.lifecycle_hooks = auditLifecycleHooks(ref, gitRoot)

  return report
}

// tupleCount returns the total number of hook tuples across all runtimes.
function tupleCount(report) {
  return report.hook_wiring.reduce((n, rr) => n + rr.tuples.length, 0)
}

// formatText renders the human-readable report.
// Format is byte-identical to Go and Python implementations.
function formatText(report) {
  const n = tupleCount(report)
  const lines = [`trackfw audit-surface: ${n} hook tuple(s) at ${report.ref}`, '']

  for (const rr of report.hook_wiring) {
    if (!rr.present) {
      lines.push(`absent [${rr.runtime}] ${rr.wiring_file}`)
      continue
    }
    if (rr.tuples.length === 0) {
      lines.push(`no_hooks [${rr.runtime}] ${rr.wiring_file}`)
      continue
    }
    for (const t of rr.tuples) {
      const matcher = t.matcher || '*'
      lines.push(`hook [${rr.runtime}] ${rr.wiring_file} ${t.event}/${matcher} ${t.raw_command} ${t.script_digest}`)
    }
  }

  for (const f of report.instruction_files) {
    if (f.kind === 'slash-command') {
      if (f.present) lines.push(`slash-command ${f.path}`)
    } else {
      lines.push(`instruction [${f.present ? 'present' : 'absent'}] ${f.path}`)
    }
  }

  for (const lh of report.lifecycle_hooks) {
    if (lh.present) {
      lines.push(`lifecycle [present] ${lh.file} ${lh.key} ${lh.command}`)
    } else {
      lines.push(`lifecycle [absent] ${lh.file} ${lh.key}`)
    }
  }

  // Remove the trailing blank line we added if body has entries after the blank.
  // If there are no body entries, remove the blank line too (match Go behavior).
  const bodyStart = lines.indexOf('') + 1
  if (bodyStart >= lines.length) {
    // Nothing after the blank — remove it.
    lines.splice(1, 1)
    return lines.join('\n') + '\n'
  }
  return lines.join('\n') + '\n'
}

const cmd = new Command('audit-surface')
cmd.description('Report the executable surface of a git ref without checking it out')
cmd.argument('<ref>', 'git ref to audit (e.g. FETCH_HEAD, a commit hash, a branch name)')
cmd.option('--json', 'Emit report as JSON instead of text')
cmd.option('--base <base>', 'Base ref for Makefile/CI diff (optional; e.g. HEAD, main)')

cmd.action((ref, options) => {
  const gitRoot = findGitRoot(process.cwd())
  if (!gitRoot) {
    process.stderr.write('audit-surface: not inside a git repository\n')
    process.exit(1)
  }

  // Validate ref resolves.
  const validateResult = spawnSync('git', ['rev-parse', '--verify', ref], { cwd: gitRoot })
  if (validateResult.status !== 0) {
    process.stderr.write(`audit-surface: ref "${ref}" does not resolve\n`)
    process.exit(1)
  }

  const report = runAuditSurface(ref, options.base || '', gitRoot)

  if (options.json) {
    process.stdout.write(JSON.stringify(report, null, 2) + '\n')
    return
  }
  process.stdout.write(formatText(report))
})

module.exports = cmd
