'use strict'

const fs = require('fs')
const path = require('path')
const os = require('os')
const { homedir } = require('../homedir')

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
 * Merge helper for Windsurf's `.windsurf/hooks.json` `hooks.<event>` arrays
 * (e.g. `hooks.pre_run_command`): each entry is `{"command": "...",
 * "show_output": bool}` -- one field richer than mergeSimpleCommandArray's
 * plain `{"command"}` shape (Cursor), so it needs its own merge helper.
 * Mirrors Go's mergeSimpleCommandArray call in InjectWindsurfHooks
 * (internal/generators/agentfiles.go), which is generic over the entry
 * shape via makeEntry/getCmd callbacks -- Node has no equivalent generic
 * variant, so this is a dedicated function instead.
 */
function mergeWindsurfHookArray(existing, command, showOutput) {
  const arr = Array.isArray(existing) ? existing.slice() : []
  if (hasEntry(arr, 'command', command)) return arr
  arr.push({ command, show_output: showOutput })
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
// GIT_BRANCH_GUARD_SCRIPT — trackfw-git-branch-guard.sh (ML-1A, ROADMAP-2026-08-14)
// ---------------------------------------------------------------------------
// Byte-identical port of internal/generators/scaffold.go:gitBranchGuardScript
// (Go's canonical reference). Blocks raw `git commit`/`git push`/
// `git checkout -b` by a subagent, regardless of runtime contract: emits BOTH
// `{"decision":"block","reason":"..."}` on stdout (Claude/Gemini JSON-stdout
// style) AND `exit 2` (Codex/Windsurf/Cursor exit-code style) simultaneously,
// same simplification decision as the Go const's doc comment. Unlike
// CREDENTIAL_GUARD_SCRIPT, this script is IDENTICAL between project and
// global scope (no trackfw.yaml dependency, no mode/roadmap_dir resolution)
// — a single constant is written by both generateGitBranchGuardScript and
// generateGlobalGitBranchGuardScript below, mirroring Go's single
// `gitBranchGuardScript` const reused by both Generate*/GenerateGlobal*
// functions.
//
// KNOWN LIMITATION (fix for 3 real bugs found by ML-4A manual E2E testing, 2026-08-14 —
// chained command `git status; git push ...` escaping detection, absolute path
// `/usr/bin/git commit` escaping via exact-string compare, and prose mentioning "git commit"
// inside a quoted string being read as a real command): the script's `match_subcommand()`
// now splits the raw command into segments on `;`/`&&`/`||`/`|`/newline and only treats a
// segment as a real git invocation if `git` (by basename) is that segment's FIRST token —
// this fixes all 3 bugs, but the splitter is NOT shell-quote-aware. It splits on those
// delimiter characters wherever they occur in the raw string, even inside single/double
// quotes. Practical consequence proven live in this session: a line inside a multi-line
// heredoc body that itself STARTS with the token `git` still blocks (newline is treated as
// a command boundary regardless of quoting context), and testing this script by embedding a
// literal `; git push`/`; git commit` substring inside a quoted JSON payload passed as a raw
// Bash tool command can trip the SAME guard when it's wired as this session's own
// PreToolUse/Bash hook — write such payloads to a file and pipe `< file.json` instead of
// inlining them in the shell command text. This limitation applies equally to the generated
// shell script itself (Go-generated and Node-generated content is byte-identical), even
// though internal/generators/scaffold.go's gitBranchGuardScript doc comment does not yet
// call it out explicitly — flagging for trackfw_architect to consider adding there too.
// GBG_BACKTICK — literal 2-char sequence "\\`" (backslash + backtick), used to embed a
// shell-escaped backtick (so REASON's `` `cmd` `` doesn't trigger bash command substitution
// inside the surrounding double-quoted string) without breaking the enclosing JS template
// literal. Mirrors the exact same problem Go's `gitBranchGuardScript` const solves by
// breaking out of its raw string and concatenating `"`"` — see scaffold.go's REASON lines
// (`\` + "`" + `trackfw ...`). A plain, unescaped backtick inside a JS template literal
// would either be interpreted as JS template-literal syntax (illegal placement) or, if
// naively escaped as just `` \` ``, would defeat the same regex-based source extraction the
// credential-guard parity test already documents as unable to handle string concatenation
// (see the CG_PROJECT_TAIL/CG_GLOBAL_TAIL comment above) — an embedded, un-terminated
// `` \` `` inside a single backtick-delimited block would make a naive
// `` const NAME = \`...\` `` regex stop at the first inner backtick instead of the real
// closing one. Splitting at every backtick insertion point, exactly like Go's own
// workaround, sidesteps both problems.
const GBG_BACKTICK = '\\`'

const GIT_BRANCH_GUARD_SCRIPT = `#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b/branch/worktree add -b
# brutos por subagente
#
# TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA: detecta o caso óbvio — comando git literal, sem
# indireção de shell — não é defesa contra um agente adversário competente. Evasões que
# exigem tokenizar como o bash (ex.: git\${IFS}push, {git,push}, g""it push) permanecem
# abertas por decisão: ver docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-
# com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md. O stripping de
# env/command abaixo reconhece as formas SEM argumentos antes de git (env git ...,
# command git ...) e o env seguido de uma sequência de atribuições CHAVE=valor
# (env FOO=bar git ..., env FOO=bar BAZ=qux git ...) — env com FLAGS (env -i git ...,
# env --ignore-environment git ...) e command com flags (command -p git ...) continuam
# evadindo; declarado, não fechado (ver AC5 do ML que adicionou esse stripping). A
# segmentação abaixo
# (quote_aware_split) evita falso-positivo em texto citado — não deve ser lida como imune a
# evasão por citação/tokenização do shell.
set -euo pipefail
set -f

# --- 0. Drena o stdin ANTES de qualquer saída antecipada (ML-1B, ROADMAP-2026-08-17-guard-
# global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md): sem isso,
# quem escreve o payload JSON no pipe recebe EPIPE quando o no-op abaixo sai com 0 antes de ler
# — reprodutível em 100% das chamadas fora de projeto trackfw, não é corrida de timing. Só drena
# se stdin não for um terminal interativo (-t 0): em invocação manual sem pipe, "cat" bloquearia
# esperando EOF (Ctrl-D). O valor lido é reaproveitado no passo 1 abaixo — nunca há uma segunda
# leitura.
_TRACKFW_STDIN=""
[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)

# --- 0b. No-op fora de projeto trackfw (ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-trackfw.md): sobe diretórios a partir do cwd FÍSICO (pwd -P, resolve symlink) até
# achar trackfw.yaml na raiz do projeto. Sem trackfw.yaml em nenhum ancestral, o guard não se
# aplica — fora de projeto trackfw não há trackfw ship como alternativa, e bloquear ali é custo
# sem contrapartida. Custo medido: só parameter expansion e test -f por nível, nenhum fork de
# processo; limitado pela profundidade do caminho.
_TRACKFW_ROOT_DIR=$(pwd -P)
_TRACKFW_FOUND=0
while :; do
  if [ -f "$_TRACKFW_ROOT_DIR/trackfw.yaml" ]; then
    _TRACKFW_FOUND=1
    break
  fi
  if [ "$_TRACKFW_ROOT_DIR" = "/" ]; then
    break
  fi
  _TRACKFW_ROOT_DIR="\${_TRACKFW_ROOT_DIR%/*}"
  if [ -z "$_TRACKFW_ROOT_DIR" ]; then
    _TRACKFW_ROOT_DIR="/"
  fi
done
[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT="$_TRACKFW_STDIN"
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \\{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="\${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Pré-processamento anti-falso-positivo: neutraliza separadores reais (';', '&&',
# '||', '|', quebra de linha) que estão DENTRO de aspas ou de corpo de heredoc, para que
# conteúdo de mensagem (ex.: ` + "`" + `-m "linha 1\\nlinha 2"` + "`" + `) nunca seja fatiado em pseudo-segmentos
# e lido como comando -------------------------------------------------------------------
#
# strip_heredoc_bodies: remove o CORPO de blocos heredoc (<<DELIM ... DELIM), preservando a
# linha de abertura e a linha terminadora — cobre o padrão ` + "`" + `git commit -F- <<'EOF' ... EOF` + "`" + `
# (heredoc não citado, fora do escopo de quote_aware_split abaixo). Heurística por linha, não
# sintaxe completa de shell: só remove o corpo quando encontra a linha terminadora
# correspondente. Se o heredoc nunca fecha (terminador ausente ou não localizado), devolve o
# texto ORIGINAL sem qualquer alteração — lado seguro: mais restritivo é preferível a esconder
# um comando real atrás de um heredoc mal-formado.
strip_heredoc_bodies() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      in_heredoc = 0
      delim = ""
      ok = 1
    }
    {
      raw = raw $0 "\\n"
      if (in_heredoc) {
        trimmed = $0
        sub(/^[ \\t]+/, "", trimmed)
        sub(/[ \\t]+$/, "", trimmed)
        if (trimmed == delim) {
          in_heredoc = 0
          out = out $0 "\\n"
        }
        next
      }
      if (match($0, /<<-?[ \\t]*[^ \\t]+/)) {
        d = substr($0, RSTART, RLENGTH)
        sub(/^<<-?[ \\t]*/, "", d)
        gsub(dq, "", d)
        gsub(sq, "", d)
        if (d != "") {
          delim = d
          in_heredoc = 1
        }
      }
      out = out $0 "\\n"
    }
    END {
      if (in_heredoc) ok = 0
      if (ok) { printf "%s", out } else { printf "%s", raw }
    }
  '
}

# quote_aware_split: emite o texto com ';' isolado, '&&', '||' e '|' isolado convertidos em
# quebra de linha — EXCETO quando ocorrem dentro de uma string entre aspas simples ou duplas,
# caso em que são preservados como texto e uma quebra de linha real dentro das aspas vira
# espaço (nunca gera um novo pseudo-segmento). Substitui o antigo ` + "`" + `sed` + "`" + ` cego, que não
# distinguia texto citado de sintaxe de comando — a causa raiz do falso-positivo de linha de
# mensagem de commit iniciada por "git ...". Aspas não fechadas até o fim da entrada
# permanecem "abertas" até o fim — mesma semântica do shell real: uma aspa não fechada nunca
# deixa o texto seguinte executar como comando novo, só torna o restante parte da mesma
# string.
quote_aware_split() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      bs = sprintf("%c", 92)
      nl = sprintf("%c", 10)
    }
    { s = (NR == 1) ? $0 : s nl $0 }
    END {
      n = length(s)
      q = ""
      out = ""
      i = 1
      while (i <= n) {
        c = substr(s, i, 1)
        if (q != "") {
          if (q == dq && c == bs && i < n) {
            nx = substr(s, i + 1, 1)
            out = out c (nx == nl ? " " : nx)
            i += 2
            continue
          }
          if (c == q) {
            q = ""
            out = out c
            i++
            continue
          }
          out = out (c == nl ? " " : c)
          i++
          continue
        }
        if (c == dq || c == sq) {
          q = c
          out = out c
          i++
          continue
        }
        if (substr(s, i, 2) == "&&" || substr(s, i, 2) == "||") {
          out = out nl
          i += 2
          continue
        }
        if (c == ";" || c == "|") {
          out = out nl
          i++
          continue
        }
        out = out c
        i++
      }
      printf "%s", out
    }
  '
}

# match_subcommand — casa contra "git (commit|push|checkout -b|switch -c)", segmento por
# segmento. Cada segmento é um comando real, obtido depois do pré-processamento acima
# (strip_heredoc_bodies + quote_aware_split), que converte ';', '&&', '||', '|' fora de aspas
# em quebra de linha e neutraliza os mesmos separadores quando aparecem dentro de
# aspas/heredoc. "git" só conta se for o PRIMEIRO token do segmento (por basename, então
# /usr/bin/git também casa) — nunca uma ocorrência solta em qualquer posição da string
# inteira. Isso evita: (a) o segundo comando de uma cadeia escapar da checagem, (b) um path
# absoluto para o git escapar por comparação de igualdade exata, e (c) texto de prosa —
# inclusive linha de mensagem de commit que COMEÇA com "git <sub>" (ex.: uma tabela
# documentando comandos bloqueados) — ser tratado como comando, porque esse texto agora nunca
# produz um novo segmento. ` + "`" + `git switch -c/-C/--create` + "`" + ` (forma alternativa a ` + "`" + `checkout -b` + "`" + `
# para criar branch) é reconhecido varrendo TODOS os tokens após o subcomando, não só o
# primeiro — cobre ` + "`" + `git switch --track -c feat/x` + "`" + ` (flag antes de -c).
# checkout -b é reconhecido do mesmo jeito: varre TODOS os tokens até achar -b/-B/--orphan,
# não só o primeiro. Prefixos env e command antes de git são descartados antes da checagem do
# basename — cobre env git push/command git push sem exigir tokenizar como o bash.
match_subcommand() {
  normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="\${first##*/}"
    while [ "$base" = "env" ] || [ "$base" = "command" ]; do
      is_env="$base"
      shift
      [ "$#" -gt 0 ] || break
      if [ "$is_env" = "env" ]; then
        while [ "$#" -gt 0 ]; do
          case "$1" in
            -*)
              break
              ;;
            *=*)
              shift
              ;;
            *)
              break
              ;;
          esac
        done
        [ "$#" -gt 0 ] || break
      fi
      first="$1"
      base="\${first##*/}"
    done
    [ "$base" = "git" ] || continue
    shift

    sub=""
    while [ "$#" -gt 0 ]; do
      tok="$1"
      case "$tok" in
        -C|-c|--work-tree|--git-dir|--namespace)
          if [ "$#" -ge 2 ]; then shift 2; else shift; fi
          continue
          ;;
        -*)
          shift
          continue
          ;;
        *)
          sub="$tok"
          shift
          break
          ;;
      esac
    done

    case "$sub" in
      commit)
        echo "commit"
        return 0
        ;;
      push)
        echo "push"
        return 0
        ;;
      checkout)
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
        # git checkout -- <path> | git checkout . descarta alterações não commitadas do
        # caminho indicado, de forma irreversível, no worktree compartilhado — bloqueia
        # quando '--' aparece em qualquer posição (forma explícita de pathspec) ou quando
        # '.' aparece como token isolado. 'git checkout <branch>' sem nenhum dos dois
        # segue liberado por decisão (distinguir branch de caminho sem '--' é ambíguo, e
        # adivinhar produziria falso-positivo).
        checkout_path=0
        for tok2 in "$@"; do
          case "$tok2" in
            --|.)
              checkout_path=1
              ;;
          esac
        done
        if [ "$checkout_path" = "1" ]; then
          echo "checkout-path"
          return 0
        fi
        ;;
      switch)
        for tok2 in "$@"; do
          case "$tok2" in
            -c|-C|--create|--create=*|--force-create|--force-create=*)
              echo "switch-c"
              return 0
              ;;
          esac
        done
        ;;
      stash)
        # git stash: liberado só para leitura (list/show) — bloqueia a forma bare
        # (equivale a "push"), push, save, clear e drop. Decisão de KG: bloquear a
        # classe inteira, não só os literais medidos (ver REQ). Repositório com um único
        # worktree compartilhado entre subagentes paralelos — um stash de um agente
        # remove as alterações não commitadas de todos os outros.
        stash_sub="\${1:-}"
        case "$stash_sub" in
          list|show)
            ;;
          *)
            echo "stash"
            return 0
            ;;
        esac
        ;;
      reset)
        # Só --hard bloqueia, em qualquer posição de token — --soft/--mixed (inclusive
        # sem flag, que é --mixed implícito) seguem liberados: --soft é o contorno
        # padrão para reempurrar trabalho staged via ` + "`" + `trackfw ship -m "..."` + "`" + ` (ainda falta commitar após --soft).
        for tok2 in "$@"; do
          case "$tok2" in
            --hard)
              echo "reset-hard"
              return 0
              ;;
          esac
        done
        ;;
      clean)
        # Bloqueia qualquer forma com force (-f, -fd, -fx, --force) ou -x/-X, EXCETO
        # quando -n/--dry-run também está presente (dry-run nunca apaga nada).
        clean_dry=0
        clean_force=0
        for tok2 in "$@"; do
          case "$tok2" in
            -n|--dry-run)
              clean_dry=1
              ;;
            -f*|--force|--force=*|-x|-X)
              clean_force=1
              ;;
          esac
        done
        if [ "$clean_dry" != "1" ] && [ "$clean_force" = "1" ]; then
          echo "clean-force"
          return 0
        fi
        ;;
      restore)
        # git restore --staged SOZINHO nunca toca o working tree (mexe só no
        # index), então segue liberado mesmo com path. Mas --worktree/-W (com ou
        # sem --staged junto) SEMPRE afeta o working tree — inclusive
        # "--staged --worktree", que restaura os dois — então bloqueia sempre que
        # --worktree/-W aparecer, e também no caso padrão (sem --staged em
        # nenhuma forma) com um argumento posicional (o path).
        restore_staged=0
        restore_worktree=0
        restore_positional=0
        for tok2 in "$@"; do
          case "$tok2" in
            --staged)
              restore_staged=1
              ;;
            --worktree|-W)
              restore_worktree=1
              ;;
            -*)
              ;;
            *)
              restore_positional=1
              ;;
          esac
        done
        if [ "$restore_positional" = "1" ]; then
          if [ "$restore_worktree" = "1" ] || [ "$restore_staged" != "1" ]; then
            echo "restore-path"
            return 0
          fi
        fi
        ;;
      branch)
        # git branch é majoritariamente leitura (sem args, -a, -r, -l, --list, -v/-vv,
        # --show-current, --contains, --no-contains, --merged, --no-merged, --sort=,
        # --format=, --points-at, -d/-D/--delete) — bloquear leitura seria pior que a
        # brecha. Só bloqueia: (a) -c/-C/-m/-M/--copy/--move (cria/renomeia branch,
        # qualquer posição de token) ou (b) um argumento posicional puro (nome da branch a
        # criar), a menos que -d/-D/--delete também esteja presente (delete tem
        # posicional legítimo — o nome a apagar). Flags de valor conhecidas (--contains,
        # --no-contains, --sort, --format, --points-at, --merged, --no-merged) têm seu
        # valor seguinte pulado quando vem em token separado, para não ser lido como
        # posicional de criação.
        branch_action=0
        has_delete=0
        saw_positional=0
        skip_next=0
        for tok2 in "$@"; do
          if [ "$skip_next" = "1" ]; then
            skip_next=0
            continue
          fi
          case "$tok2" in
            -c|-C|-m|-M|--copy|--copy=*|--move|--move=*)
              branch_action=1
              ;;
            -d|-D|--delete|--delete=*)
              has_delete=1
              ;;
            --contains|--no-contains|--sort|--format|--points-at|--merged|--no-merged)
              skip_next=1
              ;;
            -*)
              ;;
            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then
          if [ "$branch_action" = "1" ] || [ "$saw_positional" = "1" ]; then
            echo "branch-create"
            return 0
          fi
        fi
        ;;
      worktree)
        if [ "\${1:-}" = "add" ]; then
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -b|-B)
                echo "worktree-add-b"
                return 0
                ;;
            esac
          done
        elif [ "\${1:-}" = "remove" ]; then
          # git worktree remove SEM -f/--force já recusa sozinho quando há alteração não
          # commitada no worktree indicado — só a forma com force é irreversível o bastante
          # para bloquear aqui.
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -f|--force)
                echo "worktree-remove-force"
                return 0
                ;;
            esac
          done
        fi
        ;;
      update-ref)
        # git update-ref reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o
        # objeto apontado nem exigir push — foi o mecanismo que tornou alcançável o exploit
        # descrito no ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md
        # (Emenda 1): forjar origin/<base> localmente para desviar o commit-alvo de trackfw
        # release tag. Sem forma de leitura equivalente a bloquear seletivamente — a
        # subcommand inteira é escrita — bloqueia sempre, sem exceção de token.
        echo "update-ref"
        return 0
        ;;
      rm)
        # git rm -f/--force apaga do working tree e do index de forma irreversível, mesma
        # classe de git clean -f/git reset --hard já bloqueados acima — sem exceção para
        # --cached (destrancar do index sem -f já segue liberado por não precisar de force).
        for tok2 in "$@"; do
          case "$tok2" in
            -f*|--force|--force=*)
              echo "rm-force"
              return 0
              ;;
          esac
        done
        ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use ` + GBG_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  switch-c)
    REASON="trackfw: git switch -c bruto bloqueado. Use ` + GBG_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  branch-create)
    REASON="trackfw: git branch bruto bloqueado. Use ` + GBG_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-add-b)
    REASON="trackfw: git worktree add -b bruto bloqueado. Use ` + GBG_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use ` + GBG_BACKTICK + `trackfw commit -m '<mensagem>'` + GBG_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use ` + GBG_BACKTICK + `trackfw push` + GBG_BACKTICK + ` (para empurrar commits já criados), ` + GBG_BACKTICK + `trackfw ship` + GBG_BACKTICK + ` (para commit+push+PR em uma etapa) ou ` + GBG_BACKTICK + `trackfw release tag` + GBG_BACKTICK + ` (para publicar uma tag de release). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  stash)
    REASON="trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. ` + GBG_BACKTICK + `git stash list` + GBG_BACKTICK + `/` + GBG_BACKTICK + `git stash show` + GBG_BACKTICK + ` seguem liberados; para guardar trabalho em progresso, use uma branch própria via ` + GBG_BACKTICK + `trackfw branch new` + GBG_BACKTICK + ` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  reset-hard)
    REASON="trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. ` + GBG_BACKTICK + `git reset --soft` + GBG_BACKTICK + `/` + GBG_BACKTICK + `--mixed` + GBG_BACKTICK + ` seguem liberados (ex.: ` + GBG_BACKTICK + `git reset --soft HEAD~1` + GBG_BACKTICK + ` é o caminho padrão; use ` + GBG_BACKTICK + `trackfw ship -m "..."` + GBG_BACKTICK + ` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  clean-force)
    REASON="trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. ` + GBG_BACKTICK + `git clean -n` + GBG_BACKTICK + `/` + GBG_BACKTICK + `--dry-run` + GBG_BACKTICK + ` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  restore-path)
    REASON="trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. ` + GBG_BACKTICK + `git restore --staged` + GBG_BACKTICK + ` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  checkout-path)
    REASON="trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. ` + GBG_BACKTICK + `git checkout <branch>` + GBG_BACKTICK + `/` + GBG_BACKTICK + `git switch <branch>` + GBG_BACKTICK + ` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  update-ref)
    REASON="trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que ` + GBG_BACKTICK + `trackfw release tag` + GBG_BACKTICK + ` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-remove-force)
    REASON="trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. ` + GBG_BACKTICK + `git worktree remove` + GBG_BACKTICK + ` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  rm-force)
    REASON="trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de ` + GBG_BACKTICK + `git clean -f` + GBG_BACKTICK + `/` + GBG_BACKTICK + `git reset --hard` + GBG_BACKTICK + ` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\\n' "$REASON"
echo "$REASON" >&2
exit 2
`

// ---------------------------------------------------------------------------
// generateGitBranchGuardScript — writes scripts/trackfw-git-branch-guard.sh
// ---------------------------------------------------------------------------
// This ML only creates the script — it is NOT wired into any hooks.json/
// settings.json here (that is this roadmap's Wave 3 scope). Mirrors
// generateCredentialGuardScript exactly (mkdirSync + writeFileSync, mode 0o755).
function generateGitBranchGuardScript(cwd) {
  const root = cwd || process.cwd()
  const scriptsDir = path.join(root, 'scripts')
  fs.mkdirSync(scriptsDir, { recursive: true })

  const scriptPath = path.join(scriptsDir, 'trackfw-git-branch-guard.sh')
  fs.writeFileSync(scriptPath, GIT_BRANCH_GUARD_SCRIPT, { encoding: 'utf8', mode: 0o755 })
  // AC9: chmodSync restores execute bit on existing files.
  fs.chmodSync(scriptPath, 0o755)

  console.log('  ✓ scripts/trackfw-git-branch-guard.sh')
}

// ---------------------------------------------------------------------------
// generateGlobalGitBranchGuardScript — writes <home>/.trackfw/scripts/trackfw-git-branch-guard.sh
// ---------------------------------------------------------------------------
// Destinado a ser referenciado por hooks globais de CLI, instalados via
// `trackfw update harness` -- não é chamado por `trackfw init`/`trackfw
// update` (escopo de projeto), que continuam usando
// generateGitBranchGuardScript. Mirrors generateGlobalCredentialGuardScript.
function generateGlobalGitBranchGuardScript(home) {
  if (!home) {
    throw new Error('home directory vazio')
  }
  const scriptsDir = path.join(home, '.trackfw', 'scripts')
  fs.mkdirSync(scriptsDir, { recursive: true })

  const scriptPath = path.join(scriptsDir, 'trackfw-git-branch-guard.sh')
  fs.writeFileSync(scriptPath, GIT_BRANCH_GUARD_SCRIPT, { encoding: 'utf8', mode: 0o755 })

  console.log('  ✓ .trackfw/scripts/trackfw-git-branch-guard.sh')
}

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
  // AC9: chmodSync restores execute bit on existing files.
  fs.chmodSync(scriptPath, 0o755)

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

// SIGNAL_CMD_*/CLEANUP_CMD_* — split per-CLI (ROADMAP-2026-08-11 ML-2A) from what used to be two
// constants (SIGNAL_CMD/CLEANUP_CMD) shared by all 6 CLI injectors. Mutating a shared constant to
// fix one CLI's path-resolution mechanism silently changed the emission of the other 5 -- Wave 0 of
// that roadmap proved Cursor and Copilot are *already correct* (Cursor hooks run from the project
// root; Copilot emits the native "cwd": "." field), so an accidental shared-constant edit there
// would be a regression in verified-good wiring, not just scope creep. Each constant starts equal
// to the pre-split literal; only the Claude ones (ML-2A, this change) move to the
// $CLAUDE_PROJECT_DIR-pinned form below. GUARD_CMD was later split too (ML-3A) once Codex's guard
// command needed to change -- see the GUARD_CMD_* block below.
// ROADMAP-2026-08-11 ML-3A: Codex CLI does not expose a project-root env var for repo-local hooks
// (unlike Claude's $CLAUDE_PROJECT_DIR or Gemini's $GEMINI_PROJECT_DIR) -- the only documented
// mechanism is shell substitution. Per ADR-2026-08-11 ("Codex — alterar, com dependência explícita
// de shell e git"), the command is wrapped in literal double quotes around
// `$(git rev-parse --show-toplevel)`, matching every repo-local hook example in the official Codex
// docs (https://developers.openai.com/codex/config-advanced): "For repo-local hooks, prefer
// resolving from the git root instead of using a relative path such as `.codex/hooks/...`."
const CODEX_ROOT = '"$(git rev-parse --show-toplevel)'

const SIGNAL_CMD_CLAUDE = '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh'
const SIGNAL_CMD_CODEX = CODEX_ROOT + '/scripts/trackfw-attention-signal.sh"'
// $GEMINI_PROJECT_DIR (ROADMAP-2026-08-11 ML-4A): distinct from the session-following
// $GEMINI_CWD, documented and used in 100% of the Gemini CLI's official hook command
// examples (ADR-2026-08-11, "Gemini CLI — alterar, por argumento de assimetria"). Expanded
// by the Gemini CLI runtime itself -- no shell substitution needed, no literal quotes.
const SIGNAL_CMD_GEMINI = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-signal.sh'
const SIGNAL_CMD_KIRO = 'scripts/trackfw-attention-signal.sh'
const SIGNAL_CMD_COPILOT = 'scripts/trackfw-attention-signal.sh'
const SIGNAL_CMD_CURSOR = 'scripts/trackfw-attention-signal.sh'

const CLEANUP_CMD_CLAUDE = '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'
const CLEANUP_CMD_CODEX = CODEX_ROOT + '/scripts/trackfw-attention-cleanup.sh"'
const CLEANUP_CMD_GEMINI = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'
const CLEANUP_CMD_KIRO = 'scripts/trackfw-attention-cleanup.sh'
const CLEANUP_CMD_COPILOT = 'scripts/trackfw-attention-cleanup.sh'
const CLEANUP_CMD_CURSOR = 'scripts/trackfw-attention-cleanup.sh'

// Pre-ML-2A literal value of SIGNAL_CMD_CLAUDE/CLEANUP_CMD_CLAUDE, kept only as the `oldCommand`
// argument to the migration calls in injectClaudeHooks below (rewrites settings.json entries
// written by a pre-ML-2A trackfw in place, mirroring the GUARD_CMD_CLAUDE migration pattern).
const SIGNAL_CMD_CLAUDE_LEGACY = 'scripts/trackfw-attention-signal.sh'
const CLEANUP_CMD_CLAUDE_LEGACY = 'scripts/trackfw-attention-cleanup.sh'

// Pre-ML-3A literal value of SIGNAL_CMD_CODEX/CLEANUP_CMD_CODEX, kept only as the `oldCommand`
// argument to the migration calls in injectCodexHooks below.
const SIGNAL_CMD_CODEX_LEGACY = 'scripts/trackfw-attention-signal.sh'
const CLEANUP_CMD_CODEX_LEGACY = 'scripts/trackfw-attention-cleanup.sh'

// Pre-ML-4A literal value of SIGNAL_CMD_GEMINI/CLEANUP_CMD_GEMINI/GUARD_CMD_GEMINI, kept only as
// the `oldCommand` argument to the migration calls in injectGeminiHooks below.
const SIGNAL_CMD_GEMINI_LEGACY = 'scripts/trackfw-attention-signal.sh'
const CLEANUP_CMD_GEMINI_LEGACY = 'scripts/trackfw-attention-cleanup.sh'
const GUARD_CMD_GEMINI_LEGACY = 'scripts/trackfw-credential-guard.sh'

// GUARD_CMD_* -- split per-CLI (ROADMAP-2026-08-11 ML-3A) from what used to be one constant
// (GUARD_CMD) shared by Codex/Gemini/Kiro/Copilot/Cursor (Claude already had its own,
// GUARD_CMD_CLAUDE, since ML-2A). Same rationale as the SIGNAL_CMD_*/CLEANUP_CMD_* split above:
// Cursor and Copilot are verified-correct wiring (ML-0A), so a shared-constant edit made for Codex
// would have silently regressed them. Each constant starts equal to the pre-split literal; only
// GUARD_CMD_CODEX (this ML) moves to the new form below.
// Legacy relative-path literal, pre-dating both the ML-2A Claude fix and the ML-3A Codex fix --
// kept only as the `oldCommand` argument to migration calls (Claude's migration below, and Codex's
// until this ML rewrites it), mirroring the SIGNAL_CMD_CLAUDE_LEGACY/CLEANUP_CMD_CLAUDE_LEGACY
// pattern above.
const GUARD_CMD_LEGACY = 'scripts/trackfw-credential-guard.sh'
const GUARD_CMD_CODEX = CODEX_ROOT + '/scripts/trackfw-credential-guard.sh"'
const GUARD_CMD_GEMINI = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'
const GUARD_CMD_KIRO = 'scripts/trackfw-credential-guard.sh'
const GUARD_CMD_COPILOT = 'scripts/trackfw-credential-guard.sh'
const GUARD_CMD_CURSOR = 'scripts/trackfw-credential-guard.sh'

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
// GBG_CMD_* — git-branch-guard command per CLI (ROADMAP-2026-08-14, ML-3B,
// Wave 3). Same path-resolution mechanism per runtime already established
// above for GUARD_CMD_*/SIGNAL_CMD_* (Claude's $CLAUDE_PROJECT_DIR pin,
// Codex's `$(git rev-parse --show-toplevel)` substitution, Gemini's
// $GEMINI_PROJECT_DIR, Copilot/Cursor's project-root-relative path) — the
// git branch guard has no per-CLI variance of its own, it just needs the
// same "always resolve to the project root regardless of cwd drift" fix
// each of those constants already encodes for the credential guard.
//
// Wiring scope of this ML (Claude/Codex/Gemini/Cursor): registers the guard
// against the SAME hook events/matchers already used for the credential
// guard in each of these 4 CLIs' existing merge-based hooks.json/
// settings.json contract — a `Bash`/`run_shell_command`/`beforeShellExecution`
// PreToolUse-equivalent entry is sufficient to intercept `git commit`/`push`/
// `checkout -b` before they execute; no PostToolUse entry is added (unlike
// the credential guard) because blocking after the git command already ran
// is too late to be useful.
//
// NOT implemented in this ML (documented gap, mirrors the Go side of this
// same roadmap wave, which also has not built these yet): GitHub Copilot's
// `--deny-tool='shell(git commit)'`-style entries in a dedicated
// permissions-config.json/settings.json (a different mechanism than the
// existing `.github/hooks/trackfw-attention.json` hooks file this module
// already generates for Copilot); Windsurf's `pre_run_command` hook +
// `windsurf.cascadeCommandsAllowList` deny entry (injectWindsurfHooks today
// only injects textual rules, same as Go's InjectWindsurfHooks); and Amazon Q
// Developer's `preToolUse`/`execute_bash` hook + `deniedCommands` regex +
// restricted custom-agent toolset (no Amazon Q hook generator exists in this
// module today — only the textual `.amazonq/developer/guidelines.md` rules
// generator in generators/init.js — matching Go, which also has no
// InjectAmazonQHooks yet). Each of these requires a NEW file-format decision
// this roadmap wave has not yet settled on either stack; building one from
// scratch here, ahead of that decision, risks diverging from whatever the Go
// side lands on. Flagged for a follow-up ML once that decision is made.
const GBG_CMD_CLAUDE = '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'
const GBG_CMD_CODEX = CODEX_ROOT + '/scripts/trackfw-git-branch-guard.sh"'
const GBG_CMD_GEMINI = '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'
const GBG_CMD_CURSOR = 'scripts/trackfw-git-branch-guard.sh'
const GBG_CMD_COPILOT = 'scripts/trackfw-git-branch-guard.sh'
const GBG_CMD_PLAIN = 'scripts/trackfw-git-branch-guard.sh'
// GBG_CMD_WINDSURF -- Windsurf invokes hooks.pre_run_command entries via a shell (confirmed
// against https://docs.devin.ai/desktop/cascade/hooks), so the guard script is wrapped in
// `bash <path>` rather than invoked directly, unlike every other CLI above. Mirrors Go's
// windsurfGitGuardCmd (internal/generators/agentfiles.go).
const GBG_CMD_WINDSURF = 'bash scripts/trackfw-git-branch-guard.sh'

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
  const home = homedir()
  if (!home) return null
  return path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
}

/** Reads+parses JSON at $HOME/<...relParts>; returns null on any failure (fail-open). */
function readGlobalHookJSON(...relParts) {
  const home = homedir()
  if (!home) return null
  try {
    const raw = fs.readFileSync(path.join(home, ...relParts), 'utf8')
    return JSON.parse(raw)
  } catch (_) {
    return null
  }
}

/**
 * Collapses runs of consecutive slashes ("//" -> "/", any position,
 * including leading) and strips a trailing slash, so two on-disk forms of
 * the SAME script path compare equal regardless of incidental formatting
 * (e.g. $HOME resolving with a trailing slash, as happens with macOS's
 * $TMPDIR, or a hand-edited config file). Does NOT resolve "." / ".."
 * segments or symlinks -- those transforms would let unrelated paths
 * compare equal (silently disarming the dedup, the more dangerous failure
 * mode here) and symlink resolution errors on a path that does not exist
 * yet, which every caller here must fail OPEN on. Hand-rolled instead of
 * path.normalize because it disagrees with Go's filepath.Clean and Python's
 * os.path.normpath on leading "//" and trailing "/" handling (measured) --
 * mirrored byte-for-byte in internal/generators/agentfiles.go
 * (normalizeGuardPath) and pypi/trackfw/generators/hooks.py
 * (_normalize_guard_path). Never call with anything other than a script
 * path -- it is not a general string normalizer.
 */
function normalizeGuardPath(p) {
  if (!p) return p
  let out = ''
  let prevSlash = false
  for (const ch of p) {
    if (ch === '/') {
      if (prevSlash) continue
      prevSlash = true
    } else {
      prevSlash = false
    }
    out += ch
  }
  if (out.length > 1 && out.endsWith('/')) {
    out = out.replace(/\/+$/, '') || '/'
  }
  return out
}

/** Reports whether a and b denote the same script command path after normalizeGuardPath. */
function samePathCommand(a, b) {
  return normalizeGuardPath(a) === normalizeGuardPath(b)
}

/**
 * Read-only counterpart of mergeClaudeHookArray. Compares command paths via
 * samePathCommand (normalized), not raw string equality.
 *
 * ROADMAP-2026-08-17 ML-4B: also requires the sibling `type` field to equal
 * "command" -- mergeClaudeHookArray always writes
 * {type:'command',command:...}, and Claude/Codex/Gemini all silently ignore
 * a hook entry missing "type":"command" (measured, hades-tf ML-4A barrier
 * finding: a global entry with the correct command but no `type` field
 * looked "installed" to the dedup, so the project-scope entry was skipped
 * in favor of a global entry that never actually executes -- leaving BOTH
 * scopes silently unprotected while `trackfw validate` stayed green).
 * Requiring "type":"command" here closes that gap: a malformed global entry
 * is now treated as "not installed", so the project-scope entry gets
 * re-wired instead of being skipped.
 */
function hookArrayHasCommand(existing, matcher, command) {
  const arr = Array.isArray(existing) ? existing : []
  for (const item of arr) {
    if (!item || item.matcher !== matcher) continue
    const inner = Array.isArray(item.hooks) ? item.hooks : []
    if (inner.some(h => h && h.type === 'command' && typeof h.command === 'string' && samePathCommand(h.command, command))) return true
  }
  return false
}

/**
 * Read-only, path-normalized counterpart of hasEntry -- used ONLY by the
 * global-dedup read paths (globalXInstalledCursor/Copilot below), never by
 * the write-side merge/idempotency helpers (mergeSimpleCommandArray,
 * injectCursorHooks), which must keep comparing raw strings so their
 * idempotency behavior does not drift from Go/Python. See samePathCommand's
 * doc comment for why the value must always be a script path.
 *
 * ROADMAP-2026-08-17 ML-4B: requireCommandType mirrors Go's
 * simpleArrayHasValue -- Copilot entries (mergeCredentialGuardCopilotHooks)
 * always carry "type":"command" and Copilot ignores an entry without it
 * (same hades-tf ML-4A finding as hookArrayHasCommand above), so Copilot
 * callers pass true. Cursor entries (mergeCredentialGuardCursorHooks,
 * {command:...}) never carry a `type` field -- not part of Cursor's schema
 * -- so requiring it there would make this always return false for a
 * perfectly valid, executing Cursor entry; Cursor callers pass false. Do
 * NOT uniformize this across CLIs.
 */
function hasEntryPath(arr, field, value, requireCommandType) {
  return Array.isArray(arr) && arr.some(e => e && (!requireCommandType || e.type === 'command') && typeof e[field] === 'string' && samePathCommand(e[field], value))
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
  return hasEntryPath(root.hooks.beforeShellExecution, 'command', scriptPath, false)
}

function globalCredentialGuardInstalledCopilot() {
  const scriptPath = globalCredentialGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.copilot', 'settings.json')
  if (!root || !root.hooks) return false
  return hasEntryPath(root.hooks.preToolUse, 'bash', scriptPath, true)
}

/**
 * ~/.kiro/hooks/trackfw-credential-guard.json is 100% dedicated to the
 * global credential-guard wiring (overwritten wholesale, never merged), so
 * presence + non-empty content is sufficient — matches the roadmap's
 * explicit instruction for Kiro.
 */
function globalCredentialGuardInstalledKiro() {
  const home = homedir()
  if (!home) return false
  try {
    const stat = fs.statSync(path.join(home, '.kiro', 'hooks', 'trackfw-credential-guard.json'))
    return stat.size > 0
  } catch (_) {
    return false
  }
}

// ---------------------------------------------------------------------------
// git-branch-guard global-installed dedup (ROADMAP-2026-08-17 Wave 2/ML-2B)
//
// Mirrors the globalCredentialGuardInstalled<Tool> family above exactly,
// pointed at ~/.trackfw/scripts/trackfw-git-branch-guard.sh instead of
// trackfw-credential-guard.sh. Only 5 of the 6 credential-guard dedup
// targets have a git-branch-guard counterpart: Kiro's project-scope
// injector never wires git-branch-guard at all (see injectKiroHooks'
// git-branch-guard doc comment), so there is no
// globalGitBranchGuardInstalledKiro. Windsurf/AmazonQ wire git-branch-guard
// at project scope but have no global-scope target (ML-2A only added
// targets for the 6 CLIs credential-guard already covers) and no
// credential-guard dedup precedent either -- consistent, not a gap.
// ---------------------------------------------------------------------------

/** Mirrors Go's globalGitBranchGuardScriptPath. */
function globalGitBranchGuardScriptPath() {
  const home = homedir()
  if (!home) return null
  return path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
}

function globalGitBranchGuardInstalledClaude() {
  const scriptPath = globalGitBranchGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.claude', 'settings.json')
  if (!root || !root.hooks) return false
  return hookArrayHasCommand(root.hooks.PreToolUse, 'Bash', scriptPath)
}

function globalGitBranchGuardInstalledCodex() {
  const scriptPath = globalGitBranchGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.codex', 'hooks.json')
  if (!root || !root.hooks) return false
  return hookArrayHasCommand(root.hooks.PreToolUse, 'Bash', scriptPath)
}

function globalGitBranchGuardInstalledGemini() {
  const scriptPath = globalGitBranchGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.gemini', 'settings.json')
  if (!root || !root.hooks) return false
  return hookArrayHasCommand(root.hooks.BeforeTool, 'run_shell_command', scriptPath)
}

function globalGitBranchGuardInstalledCursor() {
  const scriptPath = globalGitBranchGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.cursor', 'hooks.json')
  if (!root || !root.hooks) return false
  return hasEntryPath(root.hooks.beforeShellExecution, 'command', scriptPath, false)
}

function globalGitBranchGuardInstalledCopilot() {
  const scriptPath = globalGitBranchGuardScriptPath()
  if (!scriptPath) return false
  const root = readGlobalHookJSON('.copilot', 'settings.json')
  if (!root || !root.hooks) return false
  return hasEntryPath(root.hooks.preToolUse, 'bash', scriptPath, true)
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
  // AC9 (REQ-2026-08-28): chmodSync restores the execute bit on existing files
  // where writeFileSync's {mode} option has no effect (applies only on O_CREAT).
  fs.chmodSync(signalPath, 0o755)

  const cleanupPath = path.join(scriptsDir, 'trackfw-attention-cleanup.sh')
  fs.writeFileSync(cleanupPath, CLEANUP_SCRIPT, { encoding: 'utf8', mode: 0o755 })
  // AC9: same rationale as signalPath above.
  fs.chmodSync(cleanupPath, 0o755)

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

  // Migration (ROADMAP-2026-08-11 ML-2A): rewrite any stale relative-path attention-signal/cleanup
  // command from an older trackfw run before merging the $CLAUDE_PROJECT_DIR-pinned one below, so
  // upgrading doesn't just append a second, still-cwd-fragile entry alongside the fixed one (same
  // "No such file or directory" bug class as the credential-guard fix below, and the same
  // migrate-before-merge ordering requirement -- mergeClaudeHookArray's dedup keys on the exact
  // command string, so it can't tell "same hook, new path" from "a different hook").
  migrateHookCommand(data.hooks.PreToolUse, 'AskUserQuestion', SIGNAL_CMD_CLAUDE_LEGACY, SIGNAL_CMD_CLAUDE)
  migrateHookCommand(data.hooks.PostToolUse, 'AskUserQuestion', CLEANUP_CMD_CLAUDE_LEGACY, CLEANUP_CMD_CLAUDE)

  data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'AskUserQuestion', SIGNAL_CMD_CLAUDE)

  // Rewrite any stale relative-path credential-guard command from an older trackfw run before
  // merging the fixed one below, so upgrading doesn't just append a second, still-broken entry
  // alongside the new one (see GUARD_CMD_CLAUDE comment for the "No such file or directory" bug).
  for (const matcher of ['Bash', 'Read', 'Write|Edit']) {
    migrateHookCommand(data.hooks.PreToolUse, matcher, GUARD_CMD_LEGACY, GUARD_CMD_CLAUDE)
    migrateHookCommand(data.hooks.PostToolUse, matcher, GUARD_CMD_LEGACY, GUARD_CMD_CLAUDE)
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
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'AskUserQuestion', CLEANUP_CMD_CLAUDE)
  if (!globalCredentialGuardInstalledClaude()) {
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Bash', GUARD_CMD_CLAUDE)
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Read', GUARD_CMD_CLAUDE)
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Write|Edit', GUARD_CMD_CLAUDE)
  }

  // git branch guard (ROADMAP-2026-08-14, ML-3B/Wave 3, step 1): PreToolUse-only, matcher
  // "Bash" — blocks raw `git commit`/`git push`/`git checkout -b` before they execute.
  // Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope entry when the global
  // one is already installed (`trackfw update harness --targets claude-git-branch-guard`),
  // so the guard doesn't fire (and print its block message) twice per Bash call.
  if (!globalGitBranchGuardInstalledClaude()) {
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GBG_CMD_CLAUDE)
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

  // Migration wiring (ROADMAP-2026-08-11 ML-1A, strings updated in ML-3A): rewrites any stale
  // relative-path entry from before this fix in place, so `trackfw update` doesn't just append the
  // new $(git rev-parse ...) entry alongside the still-cwd-fragile old one.
  migrateHookCommand(data.hooks.PermissionRequest, '.*', SIGNAL_CMD_CODEX_LEGACY, SIGNAL_CMD_CODEX)
  migrateHookCommand(data.hooks.PreToolUse, 'Bash', GUARD_CMD_LEGACY, GUARD_CMD_CODEX)
  migrateHookCommand(data.hooks.PreToolUse, 'apply_patch', GUARD_CMD_LEGACY, GUARD_CMD_CODEX)
  migrateHookCommand(data.hooks.PostToolUse, '.*', CLEANUP_CMD_CODEX_LEGACY, CLEANUP_CMD_CODEX)
  migrateHookCommand(data.hooks.PostToolUse, 'Bash', GUARD_CMD_LEGACY, GUARD_CMD_CODEX)
  migrateHookCommand(data.hooks.PostToolUse, 'apply_patch', GUARD_CMD_LEGACY, GUARD_CMD_CODEX)

  data.hooks.PermissionRequest = mergeClaudeHookArray(data.hooks.PermissionRequest, '.*', SIGNAL_CMD_CODEX)
  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to apply_patch): skip project-scope credential-guard when the global one is already
  // installed for this CLI.
  const skipCodexCG = globalCredentialGuardInstalledCodex()
  if (!skipCodexCG) {
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GUARD_CMD_CODEX)
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'apply_patch', GUARD_CMD_CODEX)
  }
  data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, '.*', CLEANUP_CMD_CODEX)
  if (!skipCodexCG) {
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'Bash', GUARD_CMD_CODEX)
    data.hooks.PostToolUse = mergeClaudeHookArray(data.hooks.PostToolUse, 'apply_patch', GUARD_CMD_CODEX)
  }

  // git branch guard (ROADMAP-2026-08-14, ML-3B/Wave 3, step 2): PreToolUse-only, matcher
  // "Bash" -- git commands only ever run via the Bash tool, never apply_patch.
  // Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip project-scope when the global one is
  // already installed (`trackfw update harness --targets codex-git-branch-guard`).
  if (!globalGitBranchGuardInstalledCodex()) {
    data.hooks.PreToolUse = mergeClaudeHookArray(data.hooks.PreToolUse, 'Bash', GBG_CMD_CODEX)
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
  migrateHookCommand(data.hooks.Notification, 'ToolPermission', SIGNAL_CMD_GEMINI_LEGACY, SIGNAL_CMD_GEMINI)
  migrateHookCommand(data.hooks.BeforeTool, 'run_shell_command', GUARD_CMD_GEMINI_LEGACY, GUARD_CMD_GEMINI)
  migrateHookCommand(data.hooks.BeforeTool, 'read_file|read_many_files', GUARD_CMD_GEMINI_LEGACY, GUARD_CMD_GEMINI)
  migrateHookCommand(data.hooks.BeforeTool, 'write_file|replace', GUARD_CMD_GEMINI_LEGACY, GUARD_CMD_GEMINI)
  migrateHookCommand(data.hooks.AfterTool, '*', CLEANUP_CMD_GEMINI_LEGACY, CLEANUP_CMD_GEMINI)
  migrateHookCommand(data.hooks.AfterTool, 'run_shell_command', GUARD_CMD_GEMINI_LEGACY, GUARD_CMD_GEMINI)
  migrateHookCommand(data.hooks.AfterTool, 'read_file|read_many_files', GUARD_CMD_GEMINI_LEGACY, GUARD_CMD_GEMINI)
  migrateHookCommand(data.hooks.AfterTool, 'write_file|replace', GUARD_CMD_GEMINI_LEGACY, GUARD_CMD_GEMINI)

  data.hooks.Notification = mergeClaudeHookArray(data.hooks.Notification, 'ToolPermission', SIGNAL_CMD_GEMINI)
  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to read_file|read_many_files / write_file|replace): skip project-scope
  // credential-guard when the global one is already installed.
  const skipGeminiCG = globalCredentialGuardInstalledGemini()
  if (!skipGeminiCG) {
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'run_shell_command', GUARD_CMD_GEMINI)
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'read_file|read_many_files', GUARD_CMD_GEMINI)
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'write_file|replace', GUARD_CMD_GEMINI)
  }
  data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, '*', CLEANUP_CMD_GEMINI)
  if (!skipGeminiCG) {
    data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'run_shell_command', GUARD_CMD_GEMINI)
    data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'read_file|read_many_files', GUARD_CMD_GEMINI)
    data.hooks.AfterTool = mergeClaudeHookArray(data.hooks.AfterTool, 'write_file|replace', GUARD_CMD_GEMINI)
  }

  // git branch guard (ROADMAP-2026-08-14, ML-3B/Wave 3, step 3): BeforeTool-only, matcher
  // "run_shell_command" -- git commands only ever run via the shell tool.
  // Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip project-scope when the global one is
  // already installed (`trackfw update harness --targets gemini-git-branch-guard`).
  if (!globalGitBranchGuardInstalledGemini()) {
    data.hooks.BeforeTool = mergeClaudeHookArray(data.hooks.BeforeTool, 'run_shell_command', GBG_CMD_GEMINI)
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
      action: { type: 'command', command: SIGNAL_CMD_KIRO },
    },
    {
      name: 'trackfw-attention-cleanup',
      description: 'Clears trackfw board attention after tool completes',
      trigger: 'PostToolUse',
      matcher: '*',
      action: { type: 'command', command: CLEANUP_CMD_KIRO },
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
        action: { type: 'command', command: GUARD_CMD_KIRO },
      },
      {
        name: 'trackfw-credential-guard-post',
        description: 'Warns on possible plaintext credential materialization after a shell command executes',
        trigger: 'PostToolUse',
        matcher: 'shell',
        action: { type: 'command', command: GUARD_CMD_KIRO },
      },
      // Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): "read" and "write" are the
      // documented Kiro tool-category aliases (fs_read/fs_write), same pattern as "shell" above.
      {
        name: 'trackfw-credential-guard-read-pre',
        description: 'Blocks/warns on possible plaintext credential materialization before a file read',
        trigger: 'PreToolUse',
        matcher: 'read',
        action: { type: 'command', command: GUARD_CMD_KIRO },
      },
      {
        name: 'trackfw-credential-guard-read-post',
        description: 'Warns on possible plaintext credential materialization after a file read',
        trigger: 'PostToolUse',
        matcher: 'read',
        action: { type: 'command', command: GUARD_CMD_KIRO },
      },
      {
        name: 'trackfw-credential-guard-write-pre',
        description: 'Blocks/warns on possible plaintext credential materialization before a file write',
        trigger: 'PreToolUse',
        matcher: 'write',
        action: { type: 'command', command: GUARD_CMD_KIRO },
      },
      {
        name: 'trackfw-credential-guard-write-post',
        description: 'Warns on possible plaintext credential materialization after a file write',
        trigger: 'PostToolUse',
        matcher: 'write',
        action: { type: 'command', command: GUARD_CMD_KIRO },
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

  const preToolUse = [{ type: 'command', bash: SIGNAL_CMD_COPILOT, cwd: '.', timeoutSec: 10 }]
  const postToolUse = [{ type: 'command', bash: CLEANUP_CMD_COPILOT, cwd: '.', timeoutSec: 10 }]

  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to view / create|edit): skip project-scope credential-guard entries when the global
  // one is already installed.
  if (!globalCredentialGuardInstalledCopilot()) {
    preToolUse.push({ type: 'command', matcher: 'bash', bash: GUARD_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
    preToolUse.push({ type: 'command', matcher: 'view', bash: GUARD_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
    preToolUse.push({ type: 'command', matcher: 'create|edit', bash: GUARD_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
    postToolUse.push({ type: 'command', matcher: 'bash', bash: GUARD_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
    postToolUse.push({ type: 'command', matcher: 'view', bash: GUARD_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
    postToolUse.push({ type: 'command', matcher: 'create|edit', bash: GUARD_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
  }

  // git branch guard (ROADMAP-2026-08-14, ML-3B/Wave 3, step 4): the roadmap describes
  // `--deny-tool='shell(git commit)'`-style CLI flags in a permissions-config.json/
  // settings.json file, but no such file/flag mechanism exists anywhere else in this
  // module -- Copilot's only established deny-adjacent mechanism here is this same
  // preToolUse/postToolUse hooks file already used for credential-guard above. Mirrors
  // Go's InjectCopilotHooks (internal/generators/agentfiles.go), which made the same
  // choice for the same reason; the CLI-flag/permissions-config.json approach is a
  // documented, deliberate divergence from the roadmap's literal wording. This file is
  // overwritten wholesale every run, but that only means there is no migration concern
  // -- it does not exempt this entry from the dedup-against-global check
  // (ROADMAP-2026-08-17 Wave 2/ML-2B): skip project-scope when the global one is
  // already installed (`trackfw update harness --targets copilot-git-branch-guard`),
  // same reasoning as the credential-guard dedup above.
  if (!globalGitBranchGuardInstalledCopilot()) {
    preToolUse.push({ type: 'command', matcher: 'bash', bash: GBG_CMD_COPILOT, cwd: '.', timeoutSec: 10 })
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
  if (!hasEntry(data.hooks.preToolUse, 'command', SIGNAL_CMD_CURSOR)) {
    data.hooks.preToolUse.push({ command: SIGNAL_CMD_CURSOR })
  }
  removeKnownCommandFromLegacyTopLevelArray(data, 'preToolUse', SIGNAL_CMD_CURSOR)

  if (!Array.isArray(data.hooks.postToolUse)) data.hooks.postToolUse = []
  if (!hasEntry(data.hooks.postToolUse, 'command', CLEANUP_CMD_CURSOR)) {
    data.hooks.postToolUse.push({ command: CLEANUP_CMD_CURSOR })
  }
  removeKnownCommandFromLegacyTopLevelArray(data, 'postToolUse', CLEANUP_CMD_CURSOR)

  // Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08
  // Wave 2 to Read/Write via the generic preToolUse/postToolUse events): skip project-scope
  // credential-guard entries when the global one is already installed.
  if (!globalCredentialGuardInstalledCursor()) {
    if (!Array.isArray(data.hooks.beforeShellExecution)) data.hooks.beforeShellExecution = []
    if (!hasEntry(data.hooks.beforeShellExecution, 'command', GUARD_CMD_CURSOR)) {
      data.hooks.beforeShellExecution.push({ command: GUARD_CMD_CURSOR })
    }

    if (!Array.isArray(data.hooks.afterShellExecution)) data.hooks.afterShellExecution = []
    if (!hasEntry(data.hooks.afterShellExecution, 'command', GUARD_CMD_CURSOR)) {
      data.hooks.afterShellExecution.push({ command: GUARD_CMD_CURSOR })
    }

    // Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): wired via the generic
    // preToolUse/postToolUse events (distinct from beforeShellExecution/afterShellExecution,
    // which only ever fire for Shell) with an explicit `matcher`, so these entries never fire
    // for the same tool call the unfiltered attention-signal/cleanup entries already handle
    // above in this same array. hasEntry (command-only) is not enough here -- both the
    // unfiltered signal entry and these matcher-scoped guard entries share the same array, so
    // dedup must also check `matcher`.
    const hasGuardMatcherEntry = (arr, matcher) =>
      Array.isArray(arr) && arr.some(e => e && e.command === GUARD_CMD_CURSOR && e.matcher === matcher)

    if (!hasGuardMatcherEntry(data.hooks.preToolUse, 'Read')) {
      data.hooks.preToolUse.push({ command: GUARD_CMD_CURSOR, matcher: 'Read' })
    }
    if (!hasGuardMatcherEntry(data.hooks.preToolUse, 'Write')) {
      data.hooks.preToolUse.push({ command: GUARD_CMD_CURSOR, matcher: 'Write' })
    }
    if (!hasGuardMatcherEntry(data.hooks.postToolUse, 'Read')) {
      data.hooks.postToolUse.push({ command: GUARD_CMD_CURSOR, matcher: 'Read' })
    }
    if (!hasGuardMatcherEntry(data.hooks.postToolUse, 'Write')) {
      data.hooks.postToolUse.push({ command: GUARD_CMD_CURSOR, matcher: 'Write' })
    }
  }

  // git branch guard (ROADMAP-2026-08-14, ML-3B/Wave 3, step 5): beforeShellExecution-only --
  // Cursor's dedicated pre-execution Bash event; git commands only ever run there.
  // Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip project-scope when the global one is
  // already installed (`trackfw update harness --targets cursor-git-branch-guard`). The
  // key is intentionally only touched inside this conditional (never created outside it)
  // so that when BOTH credential-guard and git-branch-guard are deduped away, the key
  // stays absent from the emitted JSON rather than becoming a present-but-empty array --
  // matches Go's InjectCursorHooks, which check-agent-hooks-parity.sh's structural
  // comparator treats as significant (absent key vs empty array is drift, not noise).
  if (!globalGitBranchGuardInstalledCursor()) {
    if (!Array.isArray(data.hooks.beforeShellExecution)) data.hooks.beforeShellExecution = []
    if (!hasEntry(data.hooks.beforeShellExecution, 'command', GBG_CMD_CURSOR)) {
      data.hooks.beforeShellExecution.push({ command: GBG_CMD_CURSOR })
    }
  }

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Windsurf — update .windsurfrules with attention instruction, and register
// the `pre_run_command` git-branch-guard hook in Windsurf's real hooks file.
// ---------------------------------------------------------------------------
// Path/schema correction (apolo-tf, 2026-08-14, post-ML-3A audit): the
// original ML-3A implementation wrote a dedicated, wholly-owned file at
// `.windsurf/hooks/trackfw-git-branch-guard.json` with an invented payload
// shape (`{"version":1,"hooks":[{"name":...,"trigger":"pre_run_command",
// "action":{...}}]}`) that was flagged in its own doc comment as UNCONFIRMED
// against official documentation. A verification pass against
// https://docs.devin.ai/desktop/cascade/hooks confirmed both the path and
// the shape were wrong:
//   - Windsurf reads hooks from a single fixed-name file, `.windsurf/hooks.json`
//     -- NOT a directory of per-hook files under `.windsurf/hooks/`.
//   - The schema is `{"hooks": {"<event>": [{"command": "...", "show_output":
//     bool}]}}` -- an object keyed by event name (e.g. "pre_run_command",
//     "post_run_command"), each mapping to an ARRAY of hook defs. There is no
//     "name"/"trigger"/"action" envelope.
//   - The hook script receives its context via stdin as JSON, including
//     `tool_info.command_line` -- GIT_BRANCH_GUARD_SCRIPT above now tries
//     this field explicitly (in addition to the generic `.command`/
//     `.tool_input.command`/`.hook_input.command` fields it already
//     handled). This is a byte-shared script across all 3 CLIs -- do not
//     edit it again without mirroring Go's gitBranchGuardScript.
//
// Merge is idempotent and shaped like every other multi-tool settings file in
// this module: existing `pre_run_command` entries from other tools (or a
// prior trackfw run) are preserved; only an entry with our exact command
// string is deduped via mergeWindsurfHookArray. Other events already present
// (e.g. a user- or third-party-authored `post_run_command`) are left
// untouched.
//
// Migration: if the stale `.windsurf/hooks/trackfw-git-branch-guard.json`
// file from the incorrect ML-3A version exists on disk, it is removed here
// (never left orphaned), and the now-possibly-empty legacy `.windsurf/hooks`
// directory is best-effort cleaned up too -- mirrors Go's InjectWindsurfHooks
// migration step exactly (os.Remove + best-effort rmdir).
//
// `windsurf.cascadeCommandsAllowList` (an IDE *user settings* key, not a
// project-local file) remains out of scope -- same reasoning as before this
// fix: trackfw has no established, confirmed mechanism for rewriting IDE user
// settings safely, and inventing one on a guess repeats the exact mistake
// this fix corrects. Documented as an open gap in docs/cli-parity.md.
const LEGACY_WINDSURF_HOOKS_FILE = 'trackfw-git-branch-guard.json'

function injectWindsurfHooks(cwd) {
  const { injectRulesForTool } = require('./init')
  injectRulesForTool('windsurf', cwd)

  // Migration: remove the incorrect, previously-written dedicated hook file
  // from an older (buggy) trackfw run, so it doesn't linger as a dead,
  // never-consumed artifact once the correct .windsurf/hooks.json exists.
  const legacyDir = path.join(cwd, '.windsurf', 'hooks')
  const legacyPath = path.join(legacyDir, LEGACY_WINDSURF_HOOKS_FILE)
  try {
    fs.unlinkSync(legacyPath)
  } catch (e) {
    if (e.code !== 'ENOENT') throw e
  }
  // Best-effort cleanup of the now-possibly-empty legacy directory; ignore
  // failure (non-empty dir, e.g. holding unrelated user files, or already
  // gone) -- never fatal.
  try {
    fs.rmdirSync(legacyDir)
  } catch (_) {
    // ignore
  }

  const dir = path.join(cwd, '.windsurf')
  fs.mkdirSync(dir, { recursive: true })
  const filePath = path.join(dir, 'hooks.json')
  const data = readJSON(filePath)

  if (typeof data.hooks !== 'object' || data.hooks === null || Array.isArray(data.hooks)) {
    data.hooks = {}
  }

  data.hooks.pre_run_command = mergeWindsurfHookArray(data.hooks.pre_run_command, GBG_CMD_WINDSURF, true)

  writeJSON(filePath, data)
}

// ---------------------------------------------------------------------------
// Amazon Q Developer CLI — .amazonq/cli-agents/q_cli_default.json
// (ROADMAP-2026-08-14, ML-3B/Wave 3, step 7)
// ---------------------------------------------------------------------------
// Path/schema correction (apolo-tf, 2026-08-14, post-ML-3A audit): the
// original ML-3A implementation wrote to `.amazonq/settings.json`, which was
// flagged in its own doc comment as NOT independently confirmed against
// official Amazon Q documentation. Verification against
// https://docs.aws.amazon.com/amazonq/latest/qdeveloper-ug/command-line-custom-agents-configuration.html
// and https://docs.aws.amazon.com/amazonq/latest/qdeveloper-ug/command-line-agents-default-behavior.html
// confirmed the real mechanism is a *custom agent* file, not a settings
// file: `.amazonq/cli-agents/q_cli_default.json` -- `q_cli_default.json` is
// the documented convention closest to activating automatically without
// requiring a manual `--agent` flag, though AWS has an open bug where this
// default-name override is not always honored
// (github.com/aws/amazon-q-developer-cli#2922) -- a custom agent named
// `q_cli_default.json` is not guaranteed to always be picked up
// automatically depending on CLI version/config. Documented rather than
// worked around.
//
// Two guard mechanisms are wired, unchanged from the original ML-3A
// implementation -- only the target file (and the minimal-but-valid custom
// agent envelope around it) moved:
//   - hooks.preToolUse[matcher:"execute_bash"] -> the guard script, same
//     matcher+hooks[].command shape already used by Claude/Codex/Gemini
//     above (reuses mergeClaudeHookArray for idempotent merge).
//   - toolsSettings.execute_bash.deniedCommands -> a regex denylist
//     evaluated before allow, independent of and in addition to the hook.
//
// Minimal-but-valid custom agent schema (per command-line-custom-agents-
// configuration.html): only set defaults for fields not already present, so
// re-running against a hand-edited or previously-generated file never
// clobbers user customization -- same "preserve existing settings" contract
// as every other merge-based injector in this module. `tools: ["*"]` is
// written on first creation so the default agent keeps today's unrestricted
// tool access (this fix does not narrow what any agent can do, only where
// the deny wiring lives). Mirrors Go's InjectAmazonQHooks
// (internal/generators/agentfiles.go) field-for-field -- Go is the
// canonical set (ROADMAP-2026-08-20, ML-1A-bis): only `name`, `description`
// and `tools` are written on first creation. `prompt`/`mcpServers`/
// `toolAliases`/`allowedTools`/`resources`/`useLegacyMcpJson` were written
// here until ML-1A-bis and are now deliberately NOT written -- an extra
// field the real schema doesn't expect risks failing validation, whereas an
// absent optional field usually doesn't (asymmetry-of-risk decision, not a
// verification against the live Amazon Q schema -- see docs/cli-parity.md
// for the recorded limit). Pre-existing occurrences of the dropped fields
// in a user's file are left untouched (never removed, only no longer
// created).
//
// Native custom-agent toolset restriction (REQ acceptance criterion): still
// NOT implemented here -- this ML only wires the guard/deny fields on the
// one default agent file; per-specialist-agent toolset restriction is out of
// scope, same limitation already accepted for Gemini above.
const AMAZONQ_CLI_AGENTS_DIR = 'cli-agents'
const AMAZONQ_DEFAULT_AGENT_FILE = 'q_cli_default.json'
const GBG_DENIED_COMMANDS_PATTERN = '^git (commit|push|checkout -b)'

function injectAmazonQHooks(cwd) {
  const filePath = path.join(cwd, '.amazonq', AMAZONQ_CLI_AGENTS_DIR, AMAZONQ_DEFAULT_AGENT_FILE)
  const data = readJSON(filePath)

  const defaults = {
    name: 'q_cli_default',
    description: 'trackfw-managed default agent — wires the git branch guard hook/denylist. See docs/cli-parity.md.',
    tools: ['*'],
  }
  for (const [k, v] of Object.entries(defaults)) {
    if (!Object.prototype.hasOwnProperty.call(data, k)) data[k] = v
  }

  if (!data.hooks) data.hooks = {}
  data.hooks.preToolUse = mergeClaudeHookArray(data.hooks.preToolUse, 'execute_bash', GBG_CMD_PLAIN)

  if (!data.toolsSettings) data.toolsSettings = {}
  if (!data.toolsSettings.execute_bash) data.toolsSettings.execute_bash = {}
  if (!Array.isArray(data.toolsSettings.execute_bash.deniedCommands)) {
    data.toolsSettings.execute_bash.deniedCommands = []
  }
  if (!data.toolsSettings.execute_bash.deniedCommands.includes(GBG_DENIED_COMMANDS_PATTERN)) {
    data.toolsSettings.execute_bash.deniedCommands.push(GBG_DENIED_COMMANDS_PATTERN)
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
    // amazonq (ROADMAP-2026-08-14, ML-3B/Wave 3, step 7 dispatch): mirrors Go's
    // hooks.go InjectHooksDetected entry -- dispatches injectAmazonQHooks (git branch
    // guard, .amazonq/settings.json) whenever the .amazonq directory is present.
    amazonq: {
      check: () => fs.existsSync(path.join(root, '.amazonq')),
      fn: injectAmazonQHooks,
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
  generateGitBranchGuardScript,
  generateGlobalGitBranchGuardScript,
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectKiroHooks,
  injectCopilotHooks,
  injectCursorHooks,
  injectWindsurfHooks,
  injectAmazonQHooks,
  injectHooksDetected,
  mergeClaudeHookArray,
  mergeSimpleCommandArray,
  mergeCopilotHookArray,
  normalizeGuardPath,
  samePathCommand,
  // Template constants — exported for scaffold doctor (ADR-2026-08-27).
  // These are the single source of truth for each script's content; the
  // scaffold doctor compares on-disk files against these exact bytes, using
  // the same constants the write path uses, so drift between the comparison
  // and the generator is structurally impossible.
  SIGNAL_SCRIPT,
  CLEANUP_SCRIPT,
  CREDENTIAL_GUARD_SCRIPT,
  GIT_BRANCH_GUARD_SCRIPT,
}
