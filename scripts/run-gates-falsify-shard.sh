#!/usr/bin/env bash
# run-gates-falsify-shard.sh — executa UM chunk de check-gates-falsify.sh como
# JOB de CI (ML-2G, ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-
# falsify-sem-perder-cobertura.md), em vez de processo em background dentro
# de um job só (isso já existe: run-gates-falsify-parallel.sh, ML-2D).
#
# Mecanismo: chama o MESMO gerador (gen-falsify-chunks.py <fonte> <dir> <N>)
# usado pelo driver de processo -- "o mesmo mecanismo que distribui entre
# processos distribui entre jobs" (decisão do roadmap). A diferença é
# topológica: aqui só o chunk $SHARD_INDEX é materializado E executado; os
# outros N-1 são descartados sem rodar (rodam em outros jobs da matriz).
#
# Isolamento: cada job de CI é uma VM própria -- isolamento de processo,
# filesystem e ambiente mais forte que o do ML-2D (que compartilhava
# GOPATH/GOCACHE/GOMODCACHE entre chunks do mesmo job). Aqui, o único estado
# potencialmente compartilhado entre shards é o que o GitHub Actions cacheia
# por conta própria (ex. setup-go com cache=true no job `go`, mas o job
# `parity` original NUNCA setou cache:true -- herdado sem alteração aqui).
# Cada shard baixa suas próprias deps (npm ci/pip install) e builda seu
# próprio bin/trackfw -- nada é passado de um shard a outro.
#
# Guarda de conjunto: este script faz só a checagem LOCAL (sentinela +
# rótulos esperados deste chunk, mesmo mecanismo do ML-2D) -- é defesa em
# profundidade, não a guarda completa. A guarda de CONJUNTO entre todos os
# shards vive em check-falsify-shard-coverage.sh, rodado pelo job de
# agregação (`parity`) a partir de uma checagem FRESCA do fonte (nunca dos
# rótulos que os shards alegam ter emitido) -- é isso que evita o defeito
# auto-referencial que o hades-tf achou no ML-2D em escala de processo.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate; exigido por
# scripts/check-output-encoding-declared.sh, ALVO 1): forca UTF-8 no stdio de
# todo python3 deste gate, ANTES da primeira invocacao (resolve_py_bin).
export PYTHONIOENCODING=utf-8

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

: "${SHARD_INDEX:?SHARD_INDEX (0-based) precisa estar setada}"
: "${SHARD_COUNT:?SHARD_COUNT precisa estar setada}"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/falsify-shard-out}"

if ! [[ "$SHARD_INDEX" =~ ^[0-9]+$ ]] || ! [[ "$SHARD_COUNT" =~ ^[0-9]+$ ]]; then
  echo "run-gates-falsify-shard: SHARD_INDEX e SHARD_COUNT precisam ser inteiros nao-negativos (recebido SHARD_INDEX='$SHARD_INDEX' SHARD_COUNT='$SHARD_COUNT')" >&2
  exit 2
fi
if [[ "$SHARD_INDEX" -ge "$SHARD_COUNT" ]]; then
  echo "run-gates-falsify-shard: SHARD_INDEX=$SHARD_INDEX fora de faixa para SHARD_COUNT=$SHARD_COUNT" >&2
  exit 2
fi

DEFAULT_SCRIPT="$ROOT_DIR/scripts/check-gates-falsify.sh"
SCRIPT="${TRACKFW_FALSIFY_SCRIPT:-$DEFAULT_SCRIPT}"
if [[ -n "${TRACKFW_FALSIFY_SCRIPT:-}" ]]; then
  echo "run-gates-falsify-shard: TRACKFW_FALSIFY_SCRIPT setada -- valor efetivo='$SCRIPT' default='$DEFAULT_SCRIPT'" >&2
fi
DEFAULT_GEN="$ROOT_DIR/scripts/gen-falsify-chunks.py"
GEN="${TRACKFW_FALSIFY_GEN:-$DEFAULT_GEN}"
if [[ -n "${TRACKFW_FALSIFY_GEN:-}" ]]; then
  echo "run-gates-falsify-shard: TRACKFW_FALSIFY_GEN setada -- valor efetivo='$GEN' default='$DEFAULT_GEN'" >&2
fi

# Mesma resolução de Python do driver de processo (ML-2A/ML-1A: python3 bare
# resolve para o stub da Microsoft Store no Windows).
resolve_py_bin() {
  local cand
  for cand in python3 python "py -3"; do
    # shellcheck disable=SC2086 -- "py -3" é dois tokens deliberadamente
    if $cand -c 'import sys; print(sys.version_info[0])' 2>/dev/null | grep -qx 3; then
      echo "$cand"
      return 0
    fi
  done
  return 1
}
if ! PY_BIN=$(resolve_py_bin); then
  echo "run-gates-falsify-shard: nenhum interpretador Python funcional encontrado (tentados: python3, python, py -3)" >&2
  exit 1
fi

WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-falsify-shard.XXXXXX")
trap 'rm -rf "$WORKDIR"' EXIT

# shellcheck disable=SC2086
$PY_BIN "$GEN" "$SCRIPT" "$WORKDIR" "$SHARD_COUNT" > "$WORKDIR/manifest.txt"
cat "$WORKDIR/manifest.txt" >&2

CHUNK="$WORKDIR/chunk_${SHARD_INDEX}.sh"
if [[ ! -f "$CHUNK" ]]; then
  echo "run-gates-falsify-shard: gerador nao produziu chunk_${SHARD_INDEX}.sh para SHARD_COUNT=$SHARD_COUNT" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"
LOG="$WORKDIR/chunk_${SHARD_INDEX}.log"
RC=0
( TRACKFW_ROOT_DIR="$ROOT_DIR" bash "$CHUNK" ) >"$LOG" 2>&1 || RC=$?
cat "$LOG"

# --- Guarda local (defesa em profundidade -- a guarda de CONJUNTO real roda
# no job de agregação, sobre todos os shards, ver header) -------------------
LOCAL_GUARD_FAILED=0

last_line=$(tail -n 1 "$LOG" 2>/dev/null || true)
if [[ "$last_line" != "CHUNK_COMPLETE $SHARD_INDEX" ]]; then
  LOCAL_GUARD_FAILED=1
  echo "run-gates-falsify-shard: GUARDA LOCAL -- chunk_${SHARD_INDEX} nao chegou ao sentinela CHUNK_COMPLETE (ultima linha: '$last_line')" >&2
fi

ACTUAL_LABELS="$OUTPUT_DIR/shard_${SHARD_INDEX}.actual"
grep -oE '^(OK|FAIL|PROOF)[[:space:]]+\[falsify/[^]]+\]' "$LOG" 2>/dev/null \
  | sed -E 's/^(OK|FAIL|PROOF)[[:space:]]+\[falsify\///; s/\]$//' \
  > "$ACTUAL_LABELS" || true

while IFS= read -r line; do
  [[ "$line" == "chunk=$SHARD_INDEX label="* ]] || continue
  expected="${line#chunk=$SHARD_INDEX label=}"
  if ! grep -qxF "$expected" "$ACTUAL_LABELS"; then
    LOCAL_GUARD_FAILED=1
    echo "run-gates-falsify-shard: GUARDA LOCAL -- rotulo esperado AUSENTE: $expected" >&2
  fi
done < "$WORKDIR/manifest.txt"

while IFS= read -r line; do
  [[ "$line" == "chunk=$SHARD_INDEX label_glob="* ]] || continue
  prefix="${line#chunk=$SHARD_INDEX label_glob=}"
  if ! grep -qF "$prefix" "$ACTUAL_LABELS"; then
    LOCAL_GUARD_FAILED=1
    echo "run-gates-falsify-shard: GUARDA LOCAL -- nenhum rotulo emitido casa o prefixo esperado: ${prefix}*" >&2
  fi
done < "$WORKDIR/manifest.txt"

cp "$LOG" "$OUTPUT_DIR/shard_${SHARD_INDEX}.log"

FINAL_RC=0
if [[ "$RC" -ne 0 || "$LOCAL_GUARD_FAILED" -ne 0 ]]; then
  FINAL_RC=1
fi
echo "$FINAL_RC" > "$OUTPUT_DIR/shard_${SHARD_INDEX}.rc"

echo "run-gates-falsify-shard: shard $SHARD_INDEX/$SHARD_COUNT -- chunk_rc=$RC guarda_local_falhou=$LOCAL_GUARD_FAILED -> final_rc=$FINAL_RC" >&2
exit "$FINAL_RC"
