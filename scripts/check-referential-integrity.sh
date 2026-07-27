#!/usr/bin/env bash
# Verifica que refs canônicas de frontmatter em REQs apontam para arquivos existentes.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

status=0

for req in docs/req/*.md; do
  [[ -f "$req" ]] || continue
  in_frontmatter=0
  seen_frontmatter=0

  while IFS= read -r line; do
    if [[ "$line" == "---" ]]; then
      if [[ $seen_frontmatter -eq 0 ]]; then
        seen_frontmatter=1
        in_frontmatter=1
        continue
      fi
      break
    fi

    [[ $in_frontmatter -eq 1 ]] || continue

    case "$line" in
      adr:*|roadmap:*)
        key=${line%%:*}
        value=${line#*:}
        value=${value#"${value%%[![:space:]]*}"}
        value=${value%"${value##*[![:space:]]}"}
        value=${value%\"}
        value=${value#\"}
        value=${value%\'}
        value=${value#\'}

        [[ -n "$value" ]] || continue
        [[ "$value" == "-" || "$value" == "—" ]] && continue

        if [[ ! -f "$value" ]]; then
          printf 'referential integrity failed: %s %s "%s" does not exist\n' "$req" "$key" "$value" >&2
          status=1
        fi
        ;;
    esac
  done < "$req"
done

if [[ $status -ne 0 ]]; then
  exit "$status"
fi

echo "Referential integrity OK"
