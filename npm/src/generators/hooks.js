'use strict'

const fs = require('fs')
const path = require('path')
const os = require('os')

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

/**
 * Merge helper para arrays de hooks "simples" tipo Cursor
 * (hooks.beforeShellExecution/afterShellExecution): cada entry é um objeto
 * plano {"command": "..."} — sem matcher, sem {type, hooks:[...]} aninhado
 * como Claude/Codex/Gemini. Mirrors internal/generators/agentfiles.go:
 * mergeSimpleCommandArray.
 */
function mergeSimpleCommandArray(existing, command) {
  const arr = Array.isArray(existing) ? existing.slice() : []
  if (hasEntry(arr, 'command', command)) return arr
  arr.push({ command })
  return arr
}

/**
 * Merge helper para arrays de hooks tipo GitHub Copilot
 * (hooks.preToolUse/postToolUse): cada entry é
 * {"type":"command","matcher":"bash","bash":"...","cwd":".","timeoutSec":10}
 * — o campo de match é "bash" (não "command", como no shape "simples" do
 * Cursor), então mergeSimpleCommandArray não serve aqui.
 * Mirrors internal/generators/update.go:mergeCredentialGuardCopilotHooks
 * (ROADMAP-2026-08-06 Wave 2/ML-2E — see that Go function's doc comment for
 * the full ~/.copilot/settings.json format investigation).
 */
function mergeCopilotHookArray(existing, scriptPath) {
  const arr = Array.isArray(existing) ? existing.slice() : []
  if (hasEntry(arr, 'bash', scriptPath)) return arr
  arr.push({ type: 'command', matcher: 'bash', bash: scriptPath, cwd: '.', timeoutSec: 10 })
  return arr
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

// migrateHookCommand rewrites a legacy hook command to a new one, in place, for every entry
// matching the given matcher inside a "matcher + hooks[].command" shaped array -- the format
// shared by Claude, Codex and Gemini's merge-based settings files (PreToolUse/PostToolUse/
// PermissionRequest/Notification/BeforeTool/AfterTool). Used to fix settings files already written
// by an older trackfw before a command string changes -- without this, re-running
// `trackfw init`/`update` only ever appends the new (fixed) command alongside the stale one (merge
// dedup in mergeClaudeHookArray keys on the exact command string, so it can't tell "same guard, new
// path" from "a different hook"), leaving the broken entry in place to keep firing and failing
// forever. Originally written for Claude only (hence the doc comment history); generalized
// (ROADMAP-2026-08-11 ML-1A) so Codex/Gemini injectors can call it too, ahead of the
// mechanism-specific string changes those CLIs' waves make. Must always be called before the
// corresponding mergeClaudeHookArray call for the same matcher, or the merge's exact-string dedup
// will append a duplicate instead of rewriting in place.
function migrateHookCommand(existing, matcher, oldCommand, newCommand) {
  const arr = Array.isArray(existing) ? existing : []
  for (const item of arr) {
    if (!item || item.matcher !== matcher) continue
    const innerHooks = Array.isArray(item.hooks) ? item.hooks : []
    for (const h of innerHooks) {
      if (h && h.command === oldCommand) h.command = newCommand
    }
  }
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

// CG_HEADER/CG_PROJECT_GUARD/CG_DETECTION_CORE/CG_PROJECT_TAIL/CG_GLOBAL_TAIL compõem
// CREDENTIAL_GUARD_SCRIPT (escopo de projeto) e GLOBAL_CREDENTIAL_GUARD_SCRIPT (escopo global,
// ~/.trackfw/scripts/, instalado via `trackfw update harness`) sem duplicar a lógica de detecção
// JWT/AWS-key em dois lugares — espelha a mesma decomposição em internal/generators/scaffold.go
// (credentialGuardHeader/credentialGuardDetectionCore/...).

const CG_HEADER = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

`

const CG_PROJECT_GUARD = `# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

`

const CG_DETECTION_CORE = `JWT_PATTERN='eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

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

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=\${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="\${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

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

`

// Resolução de MODE (grep de `credential_guard.mode` em trackfw.yaml + fallback) é inlined,
// idêntica em CG_PROJECT_TAIL (fallback "warn") e CG_GLOBAL_TAIL (fallback "block") — ao invés de
// extrair para uma constante JS compartilhada e concatenar (como o Go faz via
// credentialGuardModeResolution/DEFAULT_MODE), aqui o texto é replicado como literal em cada
// template literal: o gate de paridade Go/Node/Python
// (internal/generators/credential_guard_test.go, getNodeSourceBlock) extrai cada constante via
// regex de um único bloco `` `const NAME = \`...\`` `` sem suportar concatenação de string —
// concatenar quebraria a extração estática. Nunca editar a lógica de resolução em só um dos dois
// blocos sem replicar no outro (ML-1B, ADR-2026-08-06 emenda 6). Mirrors Go's
// credentialGuardModeResolution (semântica idêntica, forma sintática diferente por essa restrição
// do parser de paridade).
const CG_PROJECT_TAIL = `DEFAULT_MODE="warn"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
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

// CG_GLOBAL_TAIL é a contraparte de CG_PROJECT_TAIL para o escopo global
// (~/.trackfw/scripts/trackfw-credential-guard.sh, instalado via `trackfw update harness`).
//
// Decisão (ML-1B, ver ADR-2026-08-06 emenda 6 de 2026-08-08 e ROADMAP-2026-08-08, Wave 1): o modo
// em escopo global reusa a MESMA leitura de `credential_guard.mode` de trackfw.yaml que
// CG_PROJECT_TAIL já faz (mesma resolução, replicada aqui — ver o comentário de CG_PROJECT_TAIL
// sobre por que não é extraída para uma constante compartilhada em Node) — sem exigir trackfw.yaml
// existir (não há o guard `[ -f trackfw.yaml ] || exit 0` da variante de projeto: o objetivo do
// escopo global é proteger qualquer projeto, com ou sem trackfw.yaml). Quando o hook global roda a
// partir do cwd de um projeto com trackfw.yaml e credential_guard.mode explícito, esse valor é
// respeitado (warn ou block) — nenhuma mudança de comportamento para quem já definiu mode: warn
// explicitamente. Em qualquer outro caso (sem trackfw.yaml, ou trackfw.yaml sem essa chave), o
// fallback deixa de ser "warn" e passa a ser "block": um guard opt-in que nunca bloqueia por
// padrão é uma falsa sensação de proteção — o usuário que rodou `trackfw update harness` já
// demonstrou intenção explícita de ter o mecanismo ativo. Supersede a decisão original ("modo
// global sempre warn", opção "b" avaliada na ADR original) — não cria ~/.trackfw/config.yaml nem
// nenhuma outra segunda fonte de configuração só para isto.
//
// ROADMAP_DIR em escopo global: como não há garantia de trackfw.yaml para ler `roadmap_dir:`, o
// script usa o caminho padrão fixo "docs/roadmaps" relativo ao cwd de onde o hook foi disparado, e
// só grava o attention signal se esse diretório já existir (e só em modo warn — modo block nunca
// grava o attention signal, mesma decisão da variante de projeto). Não cria "docs/roadmaps" em um
// projeto aleatório só para sinalizar isso — isso pareceria ao usuário que o trackfw foi
// "instalado" nesse projeto, o que não é verdade. O texto de warning/block em stderr acontece
// sempre (visível no output do CLI/hook), independente de o diretório de attention existir.
const CG_GLOBAL_TAIL = `DEFAULT_MODE="block"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR="docs/roadmaps"
if [ ! -d "$ROADMAP_DIR" ]; then
  exit 0
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

const CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_PROJECT_GUARD + CG_DETECTION_CORE + CG_PROJECT_TAIL

const GLOBAL_CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_DETECTION_CORE + CG_GLOBAL_TAIL

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

// ---------------------------------------------------------------------------
// generateGlobalCredentialGuardScript — writes <home>/.trackfw/scripts/trackfw-credential-guard.sh
// ---------------------------------------------------------------------------
// Destinado a ser referenciado por hooks globais de CLI, instalados via `trackfw update harness`
// (ver ROADMAP-2026-08-06, Wave 2) -- não é chamado por `trackfw init`/`trackfw update` (escopo de
// projeto), que continuam usando generateCredentialGuardScript.
function generateGlobalCredentialGuardScript(home) {
  if (!home) {
    throw new Error('home directory vazio')
  }
  const scriptsDir = path.join(home, '.trackfw', 'scripts')
  fs.mkdirSync(scriptsDir, { recursive: true })

  const scriptPath = path.join(scriptsDir, 'trackfw-credential-guard.sh')
  fs.writeFileSync(scriptPath, GLOBAL_CREDENTIAL_GUARD_SCRIPT, { encoding: 'utf8', mode: 0o755 })

  console.log('  ✓ .trackfw/scripts/trackfw-credential-guard.sh')
}

const SIGNAL_CMD = 'scripts/trackfw-attention-signal.sh'
const CLEANUP_CMD = 'scripts/trackfw-attention-cleanup.sh'
const GUARD_CMD = 'scripts/trackfw-credential-guard.sh'
// Claude Code only (2026-08-09 fix, reported in production against the CMDB project): Claude Code
// resolves a bare relative hook command against the hook's *dynamic* cwd (tracks `cd`s the agent
// runs during the session), not the project root -- confirmed against
// https://code.claude.com/docs/en/hooks: "Handlers run in the current directory... cwd is dynamic".
// Any Bash/Read/Write/Edit call after the agent `cd`s into a subdirectory (e.g. a monorepo package)
// made the hook fail with "No such file or directory". $CLAUDE_PROJECT_DIR is the env var Claude
// Code guarantees stays pinned to the project root regardless of cwd drift (same doc) -- used here
// instead of GUARD_CMD, matching the pattern this project's own custom hooks already relied on
// successfully in practice.
const GUARD_CMD_CLAUDE = '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'

// ---------------------------------------------------------------------------
// Global credential-guard dedup (ROADMAP-2026-08-06 Wave 3/ML-3A)
// ---------------------------------------------------------------------------
// injectClaudeHooks/injectCodexHooks/injectGeminiHooks/injectCursorHooks/
// injectCopilotHooks/injectKiroHooks each check, read-only, whether the user
// already has the global-scope credential-guard wiring installed for that
// CLI (via `trackfw update harness --targets <tool>-credential-guard`,
// npm/src/commands/update-harness.js) before adding the project-scope
// credential-guard entry. If the global entry is already present, the
// project-scope entry is skipped entirely — attention-signal/cleanup are
// unaffected (inherently project-scoped, ADR-2026-08-06 Decision #4).
//
// Fail-open is mandatory: any failure to resolve $HOME, read the global
// file, or parse its JSON is treated as "not installed globally" -- this
// section never writes to the global file (read-only by construction).

/** Mirrors Go's globalCredentialGuardScriptPath. */
function globalCredentialGuardScriptPath() {
  const home = os.homedir()
  if (!home) return null
  return path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
}

/** Reads+parses JSON at $HOME/<...relParts>; returns null on any failure (fail-open). */
function readGlobalHookJSON(...relParts) {
  const home = os.homedir()
  if (!home) return null
  try {
    const raw = fs.readFileSync(path.join(home, ...relParts), 'utf8')
    return JSON.parse(raw)
  } catch (_) {
    return null
  }
}

/** Read-only counterpart of mergeClaudeHookArray. */
function hookArrayHasCommand(existing, matcher, command) {
  const arr = Array.isArray(existing) ? existing : []
  for (const item of arr) {
    if (!item || item.matcher !== matcher) continue
    const inner = Array.isArray(item.hooks) ? item.hooks : []
    if (inner.some(h => h && h.command === command)) return true
  }
  return false
}

function globalCredentialGuardInstalledClaude() {
  const scriptPath = globalCredentialGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.claude', 'settings.json')
  if (!root || !root.hooks) return false
  return hookArrayHasCommand(root.hooks.PreToolUse, 'Bash', scriptPath)
}

function globalCredentialGuardInstalledCodex() {
  const scriptPath = globalCredentialGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.codex', 'hooks.json')
  if (!root || !root.hooks) return false
  return hookArrayHasCommand(root.hooks.PreToolUse, 'Bash', scriptPath)
}

function globalCredentialGuardInstalledGemini() {
  const scriptPath = globalCredentialGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.gemini', 'settings.json')
  if (!root || !root.hooks) return false
  return hookArrayHasCommand(root.hooks.BeforeTool, 'run_shell_command', scriptPath)
}

function globalCredentialGuardInstalledCursor() {
  const scriptPath = globalCredentialGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.cursor', 'hooks.json')
  if (!root || !root.hooks) return false
  return hasEntry(root.hooks.beforeShellExecution, 'command', scriptPath)
}

function globalCredentialGuardInstalledCopilot() {
  const scriptPath = globalCredentialGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.copilot', 'settings.json')
  if (!root || !root.hooks) return false
  return hasEntry(root.hooks.preToolUse, 'bash', scriptPath)
}

/**
 * ~/.kiro/hooks/trackfw-credential-guard.json is 100% dedicated to the
 * global credential-guard wiring (overwritten wholesale, never merged), so
 * presence + non-empty content is sufficient — matches the roadmap's
 * explicit instruction for Kiro.
 */
function globalCredentialGuardInstalledKiro() {
  const home = os.homedir()
  if (!home) return false
  try {
    const stat = fs.statSync(path.join(home, '.kiro', 'hooks', 'trackfw-credential-guard.json'))
    return stat.size > 0
  } catch (_) {
    return false
  }
}

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

  // Rewrite any stale relative-path credential-guard command from an older trackfw run before
  // merging the fixed one below, so upgrading doesn't just append a second, still-broken entry
  // alongside the new one (see GUARD_CMD_CLAUDE comment for the "No such file or directory" bug).
  for (const matcher of ['Bash', 'Read', 'Write|Edit']) {
    migrateHookCommand(data.hooks.PreToolUse, matcher, GUARD_CMD, GUARD_CMD_CLAUDE)
    migrateHookCommand(data.hooks.PostToolUse, matcher, GUARD_CMD, GUARD_CMD_CLAUDE)
  }

  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to Read/Write|Edit): skip project-scope credential-guard when the global one is
  // already installed for this CLI.
  if (!globalCredentialGuardInstalledClaude()) {
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GUARD_CMD_CLAUDE)
    // ADR-2026-08-06 emenda 7 (2026-08-08): Read/Write/Edit coverage — extraction via direct
    // file read, or materialization via write/edit, never went through the hook before.
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Read', GUARD_CMD_CLAUDE)
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Write|Edit', GUARD_CMD_CLAUDE)
  }
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'AskUserQuestion', CLEANUP_CMD)
  if (!globalCredentialGuardInstalledClaude()) {
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Bash', GUARD_CMD_CLAUDE)
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Read', GUARD_CMD_CLAUDE)
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Write|Edit', GUARD_CMD_CLAUDE)
  }

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
//
// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2, 2026-08-08):
// Codex has NO dedicated, interceptable read-tool matcher -- confirmed against
// https://learn.chatgpt.com/docs/hooks -- so no read matcher is added here; this is a documented
// limitation (also called out in docs/cli-parity.md), not a workaround. Write/edit materialization
// IS covered via the `apply_patch` matcher (documented aliases `Edit`/`Write`).
// ---------------------------------------------------------------------------

function injectCodexHooks(cwd) {
  const filePath = path.join(cwd, '.codex', 'hooks.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}

  // Migration wiring (ROADMAP-2026-08-11 ML-1A): old === new is a functional no-op today, but
  // proves the call point exists and runs before the merge below. The wave that changes the Codex
  // command strings (ML-3A) updates the oldCommand argument here instead of adding this call from
  // scratch -- without it, the merge's exact-string dedup would append a duplicate alongside the
  // stale entry.
  migrateHookCommand(data.hooks.PermissionRequest, '.*', SIGNAL_CMD, SIGNAL_CMD)
  migrateHookCommand(data.hooks.PreToolUse, 'Bash', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.PreToolUse, 'apply_patch', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.PostToolUse, '.*', CLEANUP_CMD, CLEANUP_CMD)
  migrateHookCommand(data.hooks.PostToolUse, 'Bash', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.PostToolUse, 'apply_patch', GUARD_CMD, GUARD_CMD)

  data.hooks.PermissionRequest = mergeClaudeHookArray(data.hooks.PermissionRequest, '.*', SIGNAL_CMD)
  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to apply_patch): skip project-scope credential-guard when the global one is already
  // installed for this CLI.
  const skipCodexCG = globalCredentialGuardInstalledCodex()
  if (!skipCodexCG) {
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GUARD_CMD)
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'apply_patch', GUARD_CMD)
  }
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, '.*', CLEANUP_CMD)
  if (!skipCodexCG) {
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Bash', GUARD_CMD)
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'apply_patch', GUARD_CMD)
  }

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
//
// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2, 2026-08-08): the
// Gemini CLI tools table (https://geminicli.com/docs/reference/tools) documents `read_file`/
// `read_many_files` as the file-read tools and `write_file`/`replace` as the file-write/edit
// tools -- matcher below follows the same regex-over-tool_name convention already used for
// `run_shell_command`.
// ---------------------------------------------------------------------------

function injectGeminiHooks(cwd) {
  const filePath = path.join(cwd, '.gemini', 'settings.json')
  const data = readJSON(filePath)

  if (!data.hooks) data.hooks = {}

  // Migration wiring (ROADMAP-2026-08-11 ML-1A): old === new is a functional no-op today, but
  // proves the call point exists and runs before the merge below. The wave that changes the
  // Gemini command strings (ML-4A) updates the oldCommand argument here instead of adding this
  // call from scratch -- without it, the merge's exact-string dedup would append a duplicate
  // alongside the stale entry.
  migrateHookCommand(data.hooks.Notification, 'ToolPermission', SIGNAL_CMD, SIGNAL_CMD)
  migrateHookCommand(data.hooks.BeforeTool, 'run_shell_command', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.BeforeTool, 'read_file|read_many_files', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.BeforeTool, 'write_file|replace', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.AfterTool, '*', CLEANUP_CMD, CLEANUP_CMD)
  migrateHookCommand(data.hooks.AfterTool, 'run_shell_command', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.AfterTool, 'read_file|read_many_files', GUARD_CMD, GUARD_CMD)
  migrateHookCommand(data.hooks.AfterTool, 'write_file|replace', GUARD_CMD, GUARD_CMD)

  data.hooks.Notification = mergeClaudeHookArray(data.hooks.Notification, 'ToolPermission', SIGNAL_CMD)
  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to read_file|read_many_files / write_file|replace): skip project-scope
  // credential-guard when the global one is already installed.
  const skipGeminiCG = globalCredentialGuardInstalledGemini()
  if (!skipGeminiCG) {
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'run_shell_command', GUARD_CMD)
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'read_file|read_many_files', GUARD_CMD)
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'write_file|replace', GUARD_CMD)
  }
  data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, '*', CLEANUP_CMD)
  if (!skipGeminiCG) {
    data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'run_shell_command', GUARD_CMD)
    data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'read_file|read_many_files', GUARD_CMD)
    data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'write_file|replace', GUARD_CMD)
  }

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Kiro — .kiro/hooks/trackfw-attention.json (dedicated file, safe overwrite)
//
// Format confirmed against https://kiro.dev/docs/hooks/ , https://kiro.dev/docs/hooks/types and
// https://kiro.dev/docs/hooks/actions/ (retrieved 2026-08-05). Top level is {"version": "v1", "hooks":
// [...]} ("version" is the string "v1"), each entry {"name", "description"?, "trigger", "matcher"?,
// "action", ...}. The field is "trigger" (NOT "event" as previously emitted here and in the Go/Python
// siblings -- "event" does not exist in the documented schema). "matcher" is a plain regex string
// matched against tool name for PreToolUse/PostToolUse (NOT an object like {tool_name: ".*"} as
// previously emitted) -- "*" is the documented wildcard for "all tools"; ".*" is not a documented
// matcher value. PreToolUse ("Before a tool is about to execute", Can block: Yes) is confirmed distinct
// from PostFileSave/file-save events, resolving the ADR's open question about Kiro intercepting shell
// commands pre-execution. Blocking contract: any non-zero exit from a PreToolUse command hook blocks
// the tool invocation (stricter than the exit-code-2-specific contract of Claude Code/Codex/Gemini);
// trackfw-credential-guard.sh only ever exits 0 or 2 on its normal-operation paths (ML-1A), so this is
// safe. Shell tool matcher uses the documented alias "shell" ("all built-in shell command-related
// tools"), broader than the single canonical tool id "execute_bash". This file is fully
// generated/overwritten by trackfw (not merged with user content), so the legacy attention-signal/
// cleanup entries are realigned to the correct schema here too rather than left in the old, never-valid
// shape (same situation as the GitHub Copilot fix in ML-2D).
// ---------------------------------------------------------------------------

function injectKiroHooks(cwd) {
  const filePath = path.join(cwd, '.kiro', 'hooks', 'trackfw-attention.json')
  const hooks = [
    {
      name: 'trackfw-attention-signal',
      description: 'Signals trackfw board when agent executes a tool',
      trigger: 'PreToolUse',
      matcher: '*',
      action: { type: 'command', command: SIGNAL_CMD },
    },
    {
      name: 'trackfw-attention-cleanup',
      description: 'Clears trackfw board attention after tool completes',
      trigger: 'PostToolUse',
      matcher: '*',
      action: { type: 'command', command: CLEANUP_CMD },
    },
  ]

  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to read/write): skip project-scope credential-guard entries when the global one is
  // already installed.
  if (!globalCredentialGuardInstalledKiro()) {
    hooks.push(
      {
        name: 'trackfw-credential-guard-pre',
        description: 'Blocks/warns on possible plaintext credential materialization before a shell command executes',
        trigger: 'PreToolUse',
        matcher: 'shell',
        action: { type: 'command', command: GUARD_CMD },
      },
      {
        name: 'trackfw-credential-guard-post',
        description: 'Warns on possible plaintext credential materialization after a shell command executes',
        trigger: 'PostToolUse',
        matcher: 'shell',
        action: { type: 'command', command: GUARD_CMD },
      },
      // Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): "read" and "write" are the
      // documented Kiro tool-category aliases (fs_read/fs_write), same pattern as "shell" above.
      {
        name: 'trackfw-credential-guard-read-pre',
        description: 'Blocks/warns on possible plaintext credential materialization before a file read',
        trigger: 'PreToolUse',
        matcher: 'read',
        action: { type: 'command', command: GUARD_CMD },
      },
      {
        name: 'trackfw-credential-guard-read-post',
        description: 'Warns on possible plaintext credential materialization after a file read',
        trigger: 'PostToolUse',
        matcher: 'read',
        action: { type: 'command', command: GUARD_CMD },
      },
      {
        name: 'trackfw-credential-guard-write-pre',
        description: 'Blocks/warns on possible plaintext credential materialization before a file write',
        trigger: 'PreToolUse',
        matcher: 'write',
        action: { type: 'command', command: GUARD_CMD },
      },
      {
        name: 'trackfw-credential-guard-write-post',
        description: 'Warns on possible plaintext credential materialization after a file write',
        trigger: 'PostToolUse',
        matcher: 'write',
        action: { type: 'command', command: GUARD_CMD },
      },
    )
  }

  const data = { version: 'v1', hooks }
  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Copilot — .github/hooks/trackfw-attention.json (dedicated file, safe overwrite)
//
// Format confirmed against https://docs.github.com/en/copilot/reference/hooks-reference (retrieved
// 2026-08-05): repository-level hook files live at .github/hooks/*.json, using the schema
// {"version": 1, "hooks": {"<event>": [<command entry>, ...]}}, where a command entry is
// {"type": "command", "bash": "...", "cwd": "...", "timeoutSec": N}. This is the format
// `inject_copilot_hooks` (Python) already used; the {"hooks": [{"event", "run"}]} shape this function
// previously emitted does not match any format documented by GitHub -- this ML aligns Go/Node to
// Python (which was correct) rather than the other way around.
//
// Matcher: the doc's matcher-filtering table lists `preToolUse -> toolName` and `postToolUse ->
// toolName` (a regex, anchored `^(?:PATTERN)$`), and shows a worked `"matcher"` field inline on a
// postToolUse command entry. With camelCase event names (preToolUse/postToolUse, used here), toolName
// carries the runtime tool name, and the shell tool's runtime name is "bash" (lowercase) -- distinct
// from PascalCase events, which report the Claude-mapped name "Bash". trackfw-credential-guard.sh
// scans the raw JSON payload for JWT/AWS-key patterns regardless of field names (ML-1A), so it works
// under either payload shape; the matcher below is a scope-narrowing optimization only.
//
// Concurrency: "If multiple hooks of the same type are configured, they execute in order" (same
// section) -- Copilot hooks run serially, in configured order, for the same event, unlike Codex's
// confirmed-concurrent or Gemini's undocumented cross-group model. The ML-1A fix (credential-guard's
// "warn" mode writes to its own dedicated $ROADMAP_DIR/.trackfw-credential-guard.json, never touching
// the shared .trackfw-attention.json that trackfw-attention-cleanup.sh deletes) makes ordering moot
// regardless.
// ---------------------------------------------------------------------------

// Read/Write coverage (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2, 2026-08-08):
// https://docs.github.com/en/copilot/reference/hooks-reference confirms the camelCase
// preToolUse/postToolUse toolName mapping `view -> Read`, `create -> Write`, `edit -> Edit` --
// "view" is the read matcher, "create|edit" the write/edit matcher, same lowercase-runtime-name
// convention already used for "bash" above.
function injectCopilotHooks(cwd) {
  const filePath = path.join(cwd, '.github', 'hooks', 'trackfw-attention.json')

  const preToolUse = [{ type: 'command', bash: SIGNAL_CMD, cwd: '.', timeoutSec: 10 }]
  const postToolUse = [{ type: 'command', bash: CLEANUP_CMD, cwd: '.', timeoutSec: 10 }]

  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to view / create|edit): skip project-scope credential-guard entries when the global
  // one is already installed.
  if (!globalCredentialGuardInstalledCopilot()) {
    preToolUse.push({ type: 'command', matcher: 'bash', bash: GUARD_CMD, cwd: '.', timeoutSec: 10 })
    preToolUse.push({ type: 'command', matcher: 'view', bash: GUARD_CMD, cwd: '.', timeoutSec: 10 })
    preToolUse.push({ type: 'command', matcher: 'create|edit', bash: GUARD_CMD, cwd: '.', timeoutSec: 10 })
    postToolUse.push({ type: 'command', matcher: 'bash', bash: GUARD_CMD, cwd: '.', timeoutSec: 10 })
    postToolUse.push({ type: 'command', matcher: 'view', bash: GUARD_CMD, cwd: '.', timeoutSec: 10 })
    postToolUse.push({ type: 'command', matcher: 'create|edit', bash: GUARD_CMD, cwd: '.', timeoutSec: 10 })
  }

  const data = {
    version: 1,
    hooks: { preToolUse, postToolUse },
  }
  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Cursor — .cursor/hooks.json
//
// Two independent things are wired here, both nested under the real Cursor
// hook config `{"version": 1, "hooks": {"<eventName>": [...] }}`:
//   - hooks.preToolUse + hooks.postToolUse (migrated by this ML) --
//     attention-signal/cleanup. Prior to this ML these were written to
//     top-level preToolUse/postToolUse arrays, which did not match any
//     documented Cursor event (confirmed 2026-08-05, see docs/cli-parity.md
//     "Cursor wiring (ML-2E)"). Re-fetching https://cursor.com/docs/hooks on
//     2026-08-06 (the /docs/agent/hooks URL now 308-redirects there) shows
//     Cursor's docs were updated in the interim to add three new generic
//     events: preToolUse/postToolUse/postToolUseFailure, "fires for all tool
//     types (Shell, Read, Write, MCP, Task, etc.)". preToolUse's documented
//     input is `{"tool_name","tool_input":{...},"tool_use_id","cwd",...}`
//     and postToolUse's is the same shape plus `tool_output`/`duration` --
//     structurally identical to Claude Code's PreToolUse/PostToolUse payload
//     (`tool_name`/`tool_input`), which is exactly the shape
//     scripts/trackfw-attention-signal.sh and trackfw-attention-cleanup.sh
//     already parse (`.tool_name`, `.tool_input.question // .tool_input.command`).
//     No script changes were needed. Per-hook `matcher` filters by tool type
//     (e.g. "Shell|Read|Write") and is optional; intentionally omitted here,
//     same reasoning as beforeShellExecution below -- the attention signal
//     must fire for every tool use, not a filtered subset.
//   - hooks.beforeShellExecution + hooks.afterShellExecution (ML-2E, prior
//     cycle) -- credential-guard. beforeShellExecution is the real,
//     Bash-specific, pre-execution event: input is
//     `{"command","cwd","sandbox"}`, response (stdout JSON, only read on
//     exit code 0) is `{"permission":"allow"|"deny"|"ask","user_message":"...",
//     "agent_message":"..."}`. Per the documented "Exit code behavior": exit 0 uses the
//     JSON output (or defaults to allow if stdout has none -- confirmed by the doc's own
//     minimal example hook, which exits 0 with no stdout at all), exit 2 blocks the
//     action ("equivalent to returning permission: \"deny\""), any other exit code
//     fail-opens (hook failed, action proceeds). This is already exactly
//     trackfw-credential-guard.sh's existing contract (block mode -> exit 2 + stderr, warn
//     mode -> exit 0), so no script changes were needed to wire Cursor. afterShellExecution
//     is a post-execution audit-only event (input adds "output"/"duration", no
//     allow/deny/ask response defined) -- added in parallel for symmetry with the
//     PostToolUse wiring already used for the other CLIs in this wave. Concurrency between
//     hooks registered on the same event was not documented on the page retrieved for this
//     investigation (unlike Codex, which explicitly documents concurrent execution); not
//     assumed either way -- not a blocker here since this event array only ever contains
//     the single credential-guard entry added by trackfw.
//
// Backward compatibility: a .cursor/hooks.json written by a pre-migration
// trackfw still has the legacy top-level preToolUse/postToolUse arrays. This
// function migrates known trackfw entries out of those top-level arrays into
// the nested hooks.preToolUse/hooks.postToolUse location, and drops the
// top-level key entirely once it is empty -- but never touches or deletes
// unrelated entries a user may have added there themselves (those keys are
// inert either way -- Cursor never read the top-level location -- so leaving
// them is harmless and avoids destroying unrelated user data on a guess).
// ---------------------------------------------------------------------------

function removeKnownCommandFromLegacyTopLevelArray(data, key, command) {
  if (!Array.isArray(data[key])) return
  const kept = data[key].filter((item) => !(item && item.command === command))
  if (kept.length === 0) {
    delete data[key]
  } else {
    data[key] = kept
  }
}

function injectCursorHooks(cwd) {
  const filePath = path.join(cwd, '.cursor', 'hooks.json')
  const data = readJSON(filePath)

  if (typeof data.version === 'undefined') data.version = 1
  if (typeof data.hooks !== 'object' || data.hooks === null || Array.isArray(data.hooks)) {
    data.hooks = {}
  }

  // Migrate any legacy top-level preToolUse/postToolUse trackfw entries
  // (written by trackfw before this ML) into the nested, real hooks.
  if (!Array.isArray(data.hooks.preToolUse)) data.hooks.preToolUse = []
  if (!hasEntry(data.hooks.preToolUse, 'command', SIGNAL_CMD)) {
    data.hooks.preToolUse.push({ command: SIGNAL_CMD })
  }
  removeKnownCommandFromLegacyTopLevelArray(data, 'preToolUse', SIGNAL_CMD)

  if (!Array.isArray(data.hooks.postToolUse)) data.hooks.postToolUse = []
  if (!hasEntry(data.hooks.postToolUse, 'command', CLEANUP_CMD)) {
    data.hooks.postToolUse.push({ command: CLEANUP_CMD })
  }
  removeKnownCommandFromLegacyTopLevelArray(data, 'postToolUse', CLEANUP_CMD)

  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to Read/Write via the generic preToolUse/postToolUse events): skip project-scope
  // credential-guard entries when the global one is already installed.
  if (!globalCredentialGuardInstalledCursor()) {
    if (!Array.isArray(data.hooks.beforeShellExecution)) data.hooks.beforeShellExecution = []
    if (!hasEntry(data.hooks.beforeShellExecution, 'command', GUARD_CMD)) {
      data.hooks.beforeShellExecution.push({ command: GUARD_CMD })
    }

    if (!Array.isArray(data.hooks.afterShellExecution)) data.hooks.afterShellExecution = []
    if (!hasEntry(data.hooks.afterShellExecution, 'command', GUARD_CMD)) {
      data.hooks.afterShellExecution.push({ command: GUARD_CMD })
    }

    // Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): wired via the generic
    // preToolUse/postToolUse events (distinct from beforeShellExecution/afterShellExecution,
    // which only ever fire for Shell) with an explicit `matcher`, so these entries never fire
    // for the same tool call the unfiltered attention-signal/cleanup entries already handle
    // above in this same array. hasEntry (command-only) is not enough here -- both the
    // unfiltered signal entry and these matcher-scoped guard entries share the same array, so
    // dedup must also check `matcher`.
    const hasGuardMatcherEntry = (arr, matcher) =>
      Array.isArray(arr) && arr.some(e => e && e.command === GUARD_CMD && e.matcher === matcher)

    if (!hasGuardMatcherEntry(data.hooks.preToolUse, 'Read')) {
      data.hooks.preToolUse.push({ command: GUARD_CMD, matcher: 'Read' })
    }
    if (!hasGuardMatcherEntry(data.hooks.preToolUse, 'Write')) {
      data.hooks.preToolUse.push({ command: GUARD_CMD, matcher: 'Write' })
    }
    if (!hasGuardMatcherEntry(data.hooks.postToolUse, 'Read')) {
      data.hooks.postToolUse.push({ command: GUARD_CMD, matcher: 'Read' })
    }
    if (!hasGuardMatcherEntry(data.hooks.postToolUse, 'Write')) {
      data.hooks.postToolUse.push({ command: GUARD_CMD, matcher: 'Write' })
    }
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
  generateGlobalCredentialGuardScript,
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectKiroHooks,
  injectCopilotHooks,
  injectCursorHooks,
  injectWindsurfHooks,
  injectHooksDetected,
  mergeClaudeHookArray,
  mergeSimpleCommandArray,
  mergeCopilotHookArray,
}
