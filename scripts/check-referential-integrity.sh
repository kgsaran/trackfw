#!/usr/bin/env bash
# Verifica que refs canônicas de frontmatter em REQs apontam para arquivos existentes.
# Lê req_dir de trackfw.yaml (fallback: docs/req) — corrige o literal hardcoded que ignorava
# a configuração (REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md, AC5).
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

# Extrai req_dir de trackfw.yaml. Usa awk para o caso mais comum (chave simples com ou sem aspas).
# Fallback para docs/req quando o arquivo não existe ou a chave está ausente.
REQ_DIR="docs/req"
if [[ -f "$ROOT_DIR/trackfw.yaml" ]]; then
  _VAL=$(awk '/^req_dir[[:space:]]*:/ {
    sub(/^req_dir[[:space:]]*:[[:space:]]*/, "")
    gsub(/^["'"'"']|["'"'"']$/, "")
    gsub(/[[:space:]]*$/, "")
    print; exit
  }' "$ROOT_DIR/trackfw.yaml")
  [[ -n "$_VAL" ]] && REQ_DIR="$_VAL"
fi

status=0

# Enumera REQs no layout flat e por subdiretório (depth ≤ 2 cobre by_agent e por-estado).
# Se REQ_DIR não existe, a iteração é vazia e o script finaliza com "Referential integrity OK".
while IFS= read -r req; do
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
done < <(find "$ROOT_DIR/$REQ_DIR" -maxdepth 2 -name "*.md" 2>/dev/null | sort)

if [[ $status -ne 0 ]]; then
  exit "$status"
fi

echo "Referential integrity OK"
