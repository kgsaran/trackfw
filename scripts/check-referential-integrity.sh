#!/usr/bin/env bash
# Verifica que refs canônicas de frontmatter em REQs apontam para arquivos existentes.
# Lê req_dir de trackfw.yaml (fallback: docs/req) — corrige o literal hardcoded que ignorava
# a configuração (REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md, AC5).
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

# Extrai req_dir de trackfw.yaml. Usa awk para o caso mais comum (chave simples com ou sem aspas).
# Fallback para docs/req quando o arquivo não existe ou a chave está ausente.
_REQ_DIR_SRC="fallback docs/req"
REQ_DIR="docs/req"
if [[ -f "$ROOT_DIR/trackfw.yaml" ]]; then
  _VAL=$(awk '/^req_dir[[:space:]]*:/ {
    sub(/\r$/, "")
    sub(/^req_dir[[:space:]]*:[[:space:]]*/, "")
    gsub(/^["'"'"']|["'"'"']$/, "")
    gsub(/[[:space:]]*$/, "")
    print; exit
  }' "$ROOT_DIR/trackfw.yaml")
  if [[ -n "$_VAL" ]]; then
    REQ_DIR="$_VAL"
    _REQ_DIR_SRC="trackfw.yaml"
  fi
fi

# Guarda de vacuidade: req_dir deve existir e ser acessível.
# Nota: ambientes de CI que rodam como root ignoram chmod; o braço de permissão não tem
# cenário de falsificação dedicado por esse motivo (Seção 4, resíduo aceito na Wave 0).
if [[ ! -d "$ROOT_DIR/$REQ_DIR" ]]; then
  echo "FAIL: req_dir '$ROOT_DIR/$REQ_DIR' (de $_REQ_DIR_SRC) nao existe" >&2
  exit 1
fi
if [[ ! -r "$ROOT_DIR/$REQ_DIR" ]]; then
  echo "FAIL: req_dir '$ROOT_DIR/$REQ_DIR' (de $_REQ_DIR_SRC) existe mas nao pode ser lido" >&2
  exit 1
fi

status=0
req_count=0
ref_count=0

# Enumera REQs no layout flat e por subdiretório (depth ≤ 2 cobre by_agent e por-estado).
# find roda sem 2>/dev/null: erros de permissão em subdiretórios vão para stderr e o exit
# code é verificado (separado do sort via pipefail herdado dentro de $(...)).
if ! req_list=$(find "$ROOT_DIR/$REQ_DIR" -maxdepth 2 -name "*.md" | sort); then
  echo "FAIL: find em '$ROOT_DIR/$REQ_DIR' falhou" >&2
  exit 1
fi

while IFS= read -r req; do
  [[ -f "$req" ]] || continue
  req_count=$((req_count + 1))
  in_frontmatter=0
  seen_frontmatter=0

  while IFS= read -r line; do
    line=${line%$'\r'}
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

        ref_count=$((ref_count + 1))
        if [[ ! -f "$value" ]]; then
          printf 'referential integrity failed: %s %s "%s" does not exist\n' "$req" "$key" "$value" >&2
          status=1
        fi
        ;;
    esac
  done < "$req"
done <<<"$req_list"

if [[ $req_count -eq 0 ]]; then
  echo "FAIL: zero REQs enumeradas em '$ROOT_DIR/$REQ_DIR' (de $_REQ_DIR_SRC)" >&2
  exit 1
fi

if [[ $req_count -gt 0 && $ref_count -eq 0 ]]; then
  echo "FAIL: $req_count REQs enumeradas mas zero referencias verificadas — provavel CRLF no frontmatter, frontmatter com outra gramatica, ou REQs sem campo adr/roadmap" >&2
  exit 1
fi

if [[ $status -ne 0 ]]; then
  exit "$status"
fi

echo "Referential integrity OK: $req_count REQs, $ref_count referencias verificadas"
