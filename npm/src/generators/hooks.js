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

/** Merge helper para arrays de hooks tipo Claude / Codex / Gemini */
function mergeClaudeHookArray(existing, matcher, command) {
  const arr = Array.isArray(existing) ? existing : []

  for (const item of arr) {
    if (!item || item.matcher !== matcher) continue
    const innerHooks = Array.isArray(item.hooks) ? item.hooks : []
    if (innerHooks.some(h => h && h.command === command)) {
      return arr
    }
  }

  let entry = arr.find(e => e && e.matcher === matcher)
  if (!entry) {
    entry = { matcher, hooks: [] }
    arr.push(entry)
  }
  if (!Array.isArray(entry.hooks)) entry.hooks = []
  if (!entry.hooks.some(h => h && h.command === command)) {
    entry.hooks.push({ type: 'command', command })
  }

  return arr
}

// ---------------------------------------------------------------------------
// Scripts content
// ---------------------------------------------------------------------------

const SIGNAL_SCRIPT = `#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
set -euo pipefail

INPUT=$(cat)

[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \\(.tool_name // "unknown")") | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d '"'"'" | head -1)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$(echo "$TOOL" | sed 's/"/\\\\"/g')" \\
  "$(echo "$MSG"  | sed 's/"/\\\\"/g')" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
`

const CLEANUP_SCRIPT = `#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d '"'"'" | head -1)
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
  data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'AskUserQuestion', SIGNAL_CMD)
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'AskUserQuestion', CLEANUP_CMD)

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Codex — .codex/hooks.json
// ---------------------------------------------------------------------------

function injectCodexHooks(cwd) {
  const filePath = path.join(cwd, '.codex', 'hooks.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}
  data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, '.*', SIGNAL_CMD)
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, '.*', CLEANUP_CMD)

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Gemini — .gemini/settings.json
// ---------------------------------------------------------------------------

function injectGeminiHooks(cwd) {
  const filePath = path.join(cwd, '.gemini', 'settings.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}
  data.hooks.Notification = mergeClaudeHookArray(data.hooks.Notification, 'ToolPermission', SIGNAL_CMD)
  data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, '*', CLEANUP_CMD)

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
        description: 'Signals trackfw board when agent executes a tool',
        event: 'PreToolUse',
        matcher: { tool_name: '.*' },
        action: { type: 'command', command: SIGNAL_CMD },
      },
      {
        name: 'trackfw-attention-cleanup',
        description: 'Clears trackfw board attention after tool completes',
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
    hooks: [
      {
        event: 'preToolUse',
        run: SIGNAL_CMD,
      },
      {
        event: 'postToolUse',
        run: CLEANUP_CMD,
      },
    ],
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
// Windsurf — update .windsurfrules with attention instruction
// ---------------------------------------------------------------------------

function injectWindsurfHooks(cwd) {
  const { injectRulesForTool } = require('./init')
  return injectRulesForTool('windsurf', cwd)
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
        fs.existsSync(path.join(root, '.github', 'copilot-instructions.md')) ||
        fs.existsSync(path.join(root, '.github', 'hooks')),
      fn: injectCopilotHooks,
    },
    cursor: {
      check: () => fs.existsSync(path.join(root, '.cursor')),
      fn: injectCursorHooks,
    },
    windsurf: {
      check: () => fs.existsSync(path.join(root, '.windsurfrules')),
      fn: injectWindsurfHooks,
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
  injectWindsurfHooks,
  injectHooksDetected,
}
