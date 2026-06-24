'use strict'

const fs = require('fs')
const path = require('path')

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Lê JSON de arquivo (retorna {} se não existir ou inválido) */
function readJSON(filePath) {
  try {
    const raw = fs.readFileSync(filePath, 'utf8')
    return JSON.parse(raw)
  } catch (_) {
    return {}
  }
}

/** Escreve JSON com indent 2 */
function writeJSON(filePath, data) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, JSON.stringify(data, null, 2) + '\n', 'utf8')
}

/** Verifica se array já tem entry com determinado campo=valor */
function hasEntry(arr, field, value) {
  return Array.isArray(arr) && arr.some(e => e && e[field] === value)
}

// ---------------------------------------------------------------------------
// Scripts content
// ---------------------------------------------------------------------------

const SIGNAL_SCRIPT = `#!/usr/bin/env bash
# trackfw attention signal — permission/notification hook
# Writes .trackfw-attention.json so trackfw serve board shows a banner.
set -euo pipefail

INPUT=$(cat)
HOOK_CWD=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null || true)
[ -n "$HOOK_CWD" ] && cd "$HOOK_CWD"
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // .notification_type // ""')
  MSG=$(echo "$INPUT" | jq -r '(.message // .tool_input.description // .tool_input.question // .tool_input.command // ("Approval required for: " + (.tool_name // .notification_type // "unknown"))) | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name') or d.get('notification_type') or '')" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((d.get('message') or ti.get('description') or ti.get('question') or ti.get('command') or 'Approval required for: '+(d.get('tool_name') or d.get('notification_type') or 'unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d "\"'" | head -1)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$(echo "$TOOL" | sed 's/"/\\\\"/g')" \\
  "$(echo "$MSG"  | sed 's/"/\\\\"/g; s/$//' | tr -d '\\n')" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
`

const CLEANUP_SCRIPT = `#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

INPUT=$(cat)
HOOK_CWD=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null || true)
[ -n "$HOOK_CWD" ] && cd "$HOOK_CWD"
[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d "\"'" | head -1)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
`

const SIGNAL_CMD = 'scripts/trackfw-attention-signal.sh'
const CLEANUP_CMD = 'scripts/trackfw-attention-cleanup.sh'

// ---------------------------------------------------------------------------
// generateAttentionScripts — writes the two shell scripts to scripts/
// ---------------------------------------------------------------------------

function generateAttentionScripts(cfg, cwd) {
  const root = cwd || process.cwd()
  const scriptsDir = path.join(root, 'scripts')
  fs.mkdirSync(scriptsDir, { recursive: true })

  const signalPath = path.join(scriptsDir, 'trackfw-attention-signal.sh')
  fs.writeFileSync(signalPath, SIGNAL_SCRIPT, { encoding: 'utf8', mode: 0o755 })

  const cleanupPath = path.join(scriptsDir, 'trackfw-attention-cleanup.sh')
  fs.writeFileSync(cleanupPath, CLEANUP_SCRIPT, { encoding: 'utf8', mode: 0o755 })

  console.log('  ✓ scripts/trackfw-attention-signal.sh')
  console.log('  ✓ scripts/trackfw-attention-cleanup.sh')
}

// ---------------------------------------------------------------------------
// Claude Code — .claude/settings.json
// ---------------------------------------------------------------------------

function injectClaudeHooks(cwd) {
  const filePath = path.join(cwd, '.claude', 'settings.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}

  // PreToolUse — AskUserQuestion
  if (!data.hooks.PreToolUse) data.hooks.PreToolUse = []
  let preEntry = data.hooks.PreToolUse.find(e => e && e.matcher === 'AskUserQuestion')
  if (!preEntry) {
    preEntry = { matcher: 'AskUserQuestion', hooks: [] }
    data.hooks.PreToolUse.push(preEntry)
  }
  if (!Array.isArray(preEntry.hooks)) preEntry.hooks = []
  if (!hasEntry(preEntry.hooks, 'command', SIGNAL_CMD)) {
    preEntry.hooks.push({ type: 'command', command: SIGNAL_CMD })
  }

  // PostToolUse — cleanup
  if (!data.hooks.PostToolUse) data.hooks.PostToolUse = []
  let postEntry = data.hooks.PostToolUse.find(e => e && e.matcher === 'AskUserQuestion')
  if (!postEntry) {
    postEntry = { matcher: 'AskUserQuestion', hooks: [] }
    data.hooks.PostToolUse.push(postEntry)
  }
  if (!Array.isArray(postEntry.hooks)) postEntry.hooks = []
  if (!hasEntry(postEntry.hooks, 'command', CLEANUP_CMD)) {
    postEntry.hooks.push({ type: 'command', command: CLEANUP_CMD })
  }

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Codex — .codex/hooks.json
// ---------------------------------------------------------------------------

function injectCodexHooks(cwd) {
  const filePath = path.join(cwd, '.codex', 'hooks.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}
  if (!Array.isArray(data.hooks.PermissionRequest)) data.hooks.PermissionRequest = []
  if (!Array.isArray(data.hooks.PostToolUse)) data.hooks.PostToolUse = []

  const hasNestedCommand = (entries, command) => entries.some(
    entry => Array.isArray(entry && entry.hooks) && entry.hooks.some(h => h && h.command === command)
  )
  if (!hasNestedCommand(data.hooks.PermissionRequest, SIGNAL_CMD)) {
    data.hooks.PermissionRequest.push({
      matcher: '.*',
      hooks: [{ type: 'command', command: SIGNAL_CMD, timeout: 10, statusMessage: 'Waiting for approval' }],
    })
  }
  if (!hasNestedCommand(data.hooks.PostToolUse, CLEANUP_CMD)) {
    data.hooks.PostToolUse.push({
      matcher: '.*',
      hooks: [{ type: 'command', command: CLEANUP_CMD, timeout: 10 }],
    })
  }

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Gemini — .gemini/settings.json
// ---------------------------------------------------------------------------

function injectGeminiHooks(cwd) {
  const filePath = path.join(cwd, '.gemini', 'settings.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}

  if (!Array.isArray(data.hooks.Notification)) data.hooks.Notification = []
  if (!data.hooks.Notification.some(entry =>
    entry && entry.matcher === 'ToolPermission' &&
    Array.isArray(entry.hooks) && entry.hooks.some(hook => hook && hook.command === SIGNAL_CMD)
  )) {
    data.hooks.Notification.push({
      matcher: 'ToolPermission',
      hooks: [{ name: 'trackfw-attention-signal', type: 'command', command: SIGNAL_CMD, timeout: 10000 }],
    })
  }

  if (!Array.isArray(data.hooks.AfterTool)) data.hooks.AfterTool = []
  if (!data.hooks.AfterTool.some(entry =>
    Array.isArray(entry && entry.hooks) && entry.hooks.some(hook => hook && hook.command === CLEANUP_CMD)
  )) {
    data.hooks.AfterTool.push({
      matcher: '*',
      hooks: [{ name: 'trackfw-attention-cleanup', type: 'command', command: CLEANUP_CMD, timeout: 10000 }],
    })
  }

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Kiro — .kiro/hooks/trackfw-attention.json (dedicated file, safe overwrite)
// ---------------------------------------------------------------------------

function injectKiroHooks(cwd) {
  const filePath = path.join(cwd, '.kiro', 'hooks', 'trackfw-attention.json')
  const data = {
    hooks: [
      {
        name: 'trackfw-attention-signal',
        event: 'PreToolUse',
        matcher: { tool_name: '.*' },
        action: { type: 'command', command: SIGNAL_CMD },
      },
      {
        name: 'trackfw-attention-cleanup',
        event: 'PostToolUse',
        matcher: { tool_name: '.*' },
        action: { type: 'command', command: CLEANUP_CMD },
      },
    ],
  }
  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Copilot — .github/hooks/trackfw-attention.json (dedicated file, safe overwrite)
// ---------------------------------------------------------------------------

function injectCopilotHooks(cwd) {
  const filePath = path.join(cwd, '.github', 'hooks', 'trackfw-attention.json')
  const data = {
    version: 1,
    hooks: {
      preToolUse: [{ type: 'command', bash: SIGNAL_CMD, cwd: '.', timeoutSec: 10 }],
      postToolUse: [{ type: 'command', bash: CLEANUP_CMD, cwd: '.', timeoutSec: 10 }],
    },
  }
  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Cursor — .cursor/hooks.json
// ---------------------------------------------------------------------------

function injectCursorHooks(cwd) {
  const filePath = path.join(cwd, '.cursor', 'hooks.json')
  const data = readJSON(filePath)

  if (!Array.isArray(data.preToolUse)) data.preToolUse = []
  if (!hasEntry(data.preToolUse, 'command', SIGNAL_CMD)) {
    data.preToolUse.push({ command: SIGNAL_CMD })
  }

  if (!Array.isArray(data.postToolUse)) data.postToolUse = []
  if (!hasEntry(data.postToolUse, 'command', CLEANUP_CMD)) {
    data.postToolUse.push({ command: CLEANUP_CMD })
  }

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// injectHooksDetected — public entry point
// ---------------------------------------------------------------------------

function injectHooksDetected(cwd) {
  const root = cwd || process.cwd()

  const detections = {
    claude: {
      check: () =>
        fs.existsSync(path.join(root, '.claude')) ||
        fs.existsSync(path.join(root, 'CLAUDE.md')),
      fn: injectClaudeHooks,
    },
    codex: {
      check: () =>
        fs.existsSync(path.join(root, 'AGENTS.md')) ||
        fs.existsSync(path.join(root, '.codex')),
      fn: injectCodexHooks,
    },
    gemini: {
      check: () =>
        fs.existsSync(path.join(root, 'GEMINI.md')) ||
        fs.existsSync(path.join(root, '.gemini')),
      fn: injectGeminiHooks,
    },
    kiro: {
      check: () => fs.existsSync(path.join(root, '.kiro')),
      fn: injectKiroHooks,
    },
    copilot: {
      check: () =>
        fs.existsSync(path.join(root, '.github', 'copilot-instructions.md')),
      fn: injectCopilotHooks,
    },
    cursor: {
      check: () => fs.existsSync(path.join(root, '.cursor')),
      fn: injectCursorHooks,
    },
  }

  for (const [name, { check, fn }] of Object.entries(detections)) {
    if (!check()) continue
    try {
      fn(root)
    } catch (e) {
      console.warn(`  ⚠ hooks (${name}): ${e.message}`)
    }
  }
}

module.exports = {
  generateAttentionScripts,
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectKiroHooks,
  injectCopilotHooks,
  injectCursorHooks,
  injectHooksDetected,
}
