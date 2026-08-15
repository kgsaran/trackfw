#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b brutos por subagente
set -euo pipefail
set -f

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT=$(cat 2>/dev/null || true)
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Casar contra "git (commit|push|checkout -b)", segmento por segmento -----------------
# Cada segmento é um comando real (dividido por ; && || | e por quebra de linha). "git" só
# conta se for o PRIMEIRO token do segmento (por basename, então /usr/bin/git também casa) —
# nunca uma ocorrência solta em qualquer posição da string inteira. Isso evita: (a) o segundo
# comando de uma cadeia escapar da checagem, (b) um path absoluto para o git escapar por
# comparação de igualdade exata, e (c) texto de prosa (ex.: mensagem de commit mencionando
# "git commit" no meio de uma frase) ser tratado como comando.
match_subcommand() {
  normalized=$(printf '%s' "$1" | sed -e 's/&&/\n/g' -e 's/||/\n/g' -e 's/[;|]/\n/g')
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="${first##*/}"
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
        if [ "${1:-}" = "-b" ]; then
          echo "checkout-b"
          return 0
        fi
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
    REASON="trackfw: git checkout -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use \`trackfw commit -m '<mensagem>'\`. Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use \`trackfw ship\`. Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\n' "$REASON"
echo "$REASON" >&2
exit 2
