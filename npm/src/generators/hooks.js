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

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \\(.tool_name // "unknown")") | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

TOOL_ESC=$(echo "$TOOL" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$TOOL_ESC" \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
`

const CLEANUP_SCRIPT = `#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
`

const CREDENTIAL_GUARD_SCRIPT = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

[ -n "$MATCH" ] || exit 0

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \\" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\\$\\{?[A-Za-z_][A-Za-z0-9_]*\\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\\$\\{?([A-Za-z_][A-Za-z0-9_]*)\\}?$/\\1/')
    pattern="*\${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="warn" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// ---------------------------------------------------------------------------
// generateCredentialGuardScript — writes scripts/trackfw-credential-guard.sh
// ---------------------------------------------------------------------------
// ML-1A only: creates the script. It is NOT wired into any hooks.json/settings.json
// here -- that is Wave 2's scope (see ROADMAP-2026-08-05-hooks-de-guarda-contra-
// materializacao-de-credenciais-reais-por-subagentes.md).
function generateCredentialGuardScript(cwd) {
  const root = cwd || process.cwd()
  const scriptsDir = path.join(root, 'scripts')
  fs.mkdirSync(scriptsDir, { recursive: true })

  const scriptPath = path.join(scriptsDir, 'trackfw-credential-guard.sh')
  fs.writeFileSync(scriptPath, CREDENTIAL_GUARD_SCRIPT, { encoding: 'utf8', mode: 0o755 })

  console.log('  ✓ scripts/trackfw-credential-guard.sh')
}

const SIGNAL_CMD = 'scripts/trackfw-attention-signal.sh'
const CLEANUP_CMD = 'scripts/trackfw-attention-cleanup.sh'
const GUARD_CMD = 'scripts/trackfw-credential-guard.sh'

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
  data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GUARD_CMD)
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'AskUserQuestion', CLEANUP_CMD)
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Bash', GUARD_CMD)

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Codex — .codex/hooks.json
//
// Two independent hook events: PermissionRequest (matcher ".*") for the existing
// attention-signal -- only fires when Codex is about to prompt for approval, not
// for every command -- and PreToolUse/PostToolUse (matcher "Bash") for
// credential-guard, which fires for every Bash tool call regardless of approval.
// Confirmed against https://developers.openai.com/codex/hooks (2026-08-05): hooks
// are enabled by default (no `[features] hooks = true`/`codex_hooks` opt-in
// needed -- that flag exists only to turn hooks OFF), and PreToolUse blocking
// uses exit code 2 + stderr (matching trackfw-credential-guard.sh's "block" mode).
// ---------------------------------------------------------------------------

function injectCodexHooks(cwd) {
  const filePath = path.join(cwd, '.codex', 'hooks.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}
  data.hooks.PermissionRequest = mergeClaudeHookArray(data.hooks.PermissionRequest, '.*', SIGNAL_CMD)
  data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GUARD_CMD)
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, '.*', CLEANUP_CMD)
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Bash', GUARD_CMD)

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Gemini — .gemini/settings.json
//
// Three independent hook events: Notification (matcher "ToolPermission") for the
// existing attention-signal -- only fires when Gemini CLI is about to prompt for
// permission, not for every tool call -- and BeforeTool/AfterTool (matcher
// "run_shell_command") for credential-guard, which fires for every shell tool call
// regardless of whether a permission prompt is needed. Confirmed against
// https://geminicli.com/docs/hooks/reference (retrieved 2026-08-05): BeforeTool
// "Fires before a tool is invoked. Used for argument validation, security checks,
// and parameter rewriting" and supports "Exit Code 2 (Block Tool): Prevents
// execution. Uses stderr as the reason" -- matching trackfw-credential-guard.sh's
// existing "block" mode. The shell tool's canonical name is "run_shell_command"
// (doc: "you can match any built-in tool (for example, read_file,
// run_shell_command)"); matcher is a regex evaluated against tool_name. AfterTool
// (matcher "*") is the pre-existing attention-cleanup wiring, unrelated to the new
// credential-guard entry added as a separate array entry (different matcher) in the
// same event.
// ---------------------------------------------------------------------------

function injectGeminiHooks(cwd) {
  const filePath = path.join(cwd, '.gemini', 'settings.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}
  data.hooks.Notification = mergeClaudeHookArray(data.hooks.Notification, 'ToolPermission', SIGNAL_CMD)
  data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'run_shell_command', GUARD_CMD)
  data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, '*', CLEANUP_CMD)
  data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'run_shell_command', GUARD_CMD)

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
  generateCredentialGuardScript,
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectKiroHooks,
  injectCopilotHooks,
  injectCursorHooks,
  injectWindsurfHooks,
  injectHooksDetected,
}
