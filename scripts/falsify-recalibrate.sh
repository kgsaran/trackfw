#!/usr/bin/env bash
# falsify-recalibrate.sh <run-id> — baixa os timing logs de um run de CI e
# regenera scripts/falsify-scenario-weights.json a partir deles.
#
# Uso:
#   bash scripts/falsify-recalibrate.sh <run-id>
#   FALSIFY_WEIGHTS_OUT=caminho/alternativo.json bash scripts/falsify-recalibrate.sh <run-id>
#   GH_BIN=/caminho/gh bash scripts/falsify-recalibrate.sh <run-id>   # para autoteste
#
# Requer: gh autenticado como mantenedor do repo kgsaran/trackfw.
# O destino final é escrito de forma atômica pelo gen-falsify-scenario-weights.py.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# FALSIFY_REPO_ROOT permite que o autoteste grave o script sabotado fora de scripts/
# (em $SCRATCH) e ainda assim resolva o workflow e o gerador Python corretamente.
# Quando definido, SCRIPT_DIR e REPO_ROOT derivam de FALSIFY_REPO_ROOT (nao de BASH_SOURCE),
# para que gen-falsify-scenario-weights.py seja encontrado no repo real.
if [[ -n "${FALSIFY_REPO_ROOT:-}" ]]; then
  REPO_ROOT="$(cd "$FALSIFY_REPO_ROOT" && pwd)"
  SCRIPT_DIR="$REPO_ROOT/scripts"
else
  REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

GH_BIN="${GH_BIN:-gh}"
DEST="${FALSIFY_WEIGHTS_OUT:-$REPO_ROOT/scripts/falsify-scenario-weights.json}"

# --- 1. Argumento obrigatório ---
if [[ $# -lt 1 || -z "${1:-}" ]]; then
  echo "falsify-recalibrate: uso: $0 <run-id>" >&2
  exit 1
fi
RUN="$1"

# --- (a) Verificar que o run pertence ao repo correto ---
head_repo=$("$GH_BIN" api "repos/kgsaran/trackfw/actions/runs/$RUN" --jq '.head_repository.full_name' 2>/dev/null) || {
  echo "falsify-recalibrate: nao foi possivel consultar o run $RUN via gh api" >&2
  exit 1
}
if [[ "$head_repo" != "kgsaran/trackfw" ]]; then
  echo "falsify-recalibrate: run $RUN pertence a '$head_repo', nao a 'kgsaran/trackfw' -- abortando para nao calibrar com dados de fork" >&2
  exit 1
fi

# --- (b) Ler FALSIFY_SHARD_COUNT do workflow ---
WF="$REPO_ROOT/.github/workflows/quality.yml"
SHARD_COUNT=$(grep -m1 'FALSIFY_SHARD_COUNT:' "$WF" | sed -E 's/.*FALSIFY_SHARD_COUNT:[[:space:]]*"?([0-9]+)"?.*/\1/') || true
if [[ -z "$SHARD_COUNT" || ! "$SHARD_COUNT" =~ ^[0-9]+$ ]]; then
  echo "falsify-recalibrate: nao foi possivel ler FALSIFY_SHARD_COUNT de $WF" >&2
  exit 1
fi

# --- (c) Baixar artefatos falsify-shard-* ---
TMP=$(mktemp -d "${TMPDIR:-/tmp}/falsify-recalibrate.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

"$GH_BIN" run download "$RUN" --repo kgsaran/trackfw --pattern 'falsify-shard-*' --dir "$TMP"

# --- (d) Exigir timing_N.log não vazio para cada shard ---
for n in $(seq 0 $((SHARD_COUNT - 1))); do
  timing_file=$(find "$TMP" -name "timing_${n}.log" | head -1 || true)
  if [[ -z "$timing_file" || ! -s "$timing_file" ]]; then
    echo "falsify-recalibrate: timing_${n}.log ausente ou vazio no run $RUN (shard $n de $SHARD_COUNT) -- abortando sem tocar o destino" >&2
    exit 1
  fi
done

# --- (e) Concatenar e rodar o gerador ---
CONCAT="$TMP/timing_all.log"
for n in $(seq 0 $((SHARD_COUNT - 1))); do
  find "$TMP" -name "timing_${n}.log" | head -1 | xargs cat
done > "$CONCAT"

python3 "$SCRIPT_DIR/gen-falsify-scenario-weights.py" "$CONCAT" "$DEST"

echo "falsify-recalibrate: pesos escritos em $DEST"
