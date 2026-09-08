#!/usr/bin/env bash
# check-falsify-shard-coverage.sh — guarda de CONJUNTO entre jobs de matriz
# (ML-2G, ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify-sem-
# perder-cobertura.md). Roda no job de agregação (`parity`, id preservado por
# ser required_status_check por NOME -- ver comentário em
# .github/workflows/quality.yml), depois de baixar os artefatos de todos os
# shards de `run-gates-falsify-shard.sh`.
#
# Desenho deliberado (parecer hades-tf sobre o ML-2D, generalizado para job):
# os rótulos ESPERADOS são recalculados AQUI, a partir de um checkout FRESCO
# de check-gates-falsify.sh (o mesmo $1) via gen-falsify-chunks.py -- NUNCA
# lidos de algo que um shard alega ter esperado. Só os rótulos EMITIDOS
# (shard_N.actual, fato observado no stdout real do chunk que rodou) vêm dos
# shards. Isso evita reintroduzir, em escala de job, o defeito auto-
# referencial que o hades-tf achou no ML-2D em escala de processo (guarda
# derivava o esperado do MESMO fonte que também gerava o runtime -- sabotar
# o alvo apagava o requisito junto).
#
# Falha nomeando: shard ausente (artefato não baixado -- job não rodou, ou
# upload falhou), shard com rc != 0 (guarda local do próprio shard reprovou,
# OU o chunk reprovou), ou rótulo esperado ausente do conjunto de rótulos
# emitidos por aquele shard especificamente (rótulo emitido por OUTRO shard
# não conta -- cada chunk só pode emitir os rótulos que o gerador atribuiu a
# ele).
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate; exigido por
# scripts/check-output-encoding-declared.sh, ALVO 1): forca UTF-8 no stdio de
# todo python3 deste gate, ANTES da primeira invocacao (resolve_py_bin).
export PYTHONIOENCODING=utf-8

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

SCRIPT="${1:?uso: check-falsify-shard-coverage.sh <script-fonte> <shard-count> <dir-com-artefatos-baixados>}"
SHARD_COUNT="${2:?uso: check-falsify-shard-coverage.sh <script-fonte> <shard-count> <dir-com-artefatos-baixados>}"
ARTIFACTS_DIR="${3:?uso: check-falsify-shard-coverage.sh <script-fonte> <shard-count> <dir-com-artefatos-baixados>}"

if [[ ! -s "$SCRIPT" ]]; then
  echo "check-falsify-shard-coverage: fonte ausente ou vazio: $SCRIPT" >&2
  exit 1
fi
if [[ ! -d "$ARTIFACTS_DIR" ]]; then
  echo "check-falsify-shard-coverage: diretorio de artefatos ausente: $ARTIFACTS_DIR" >&2
  exit 1
fi

resolve_py_bin() {
  local cand
  for cand in python3 python "py -3"; do
    # shellcheck disable=SC2086
    if $cand -c 'import sys; print(sys.version_info[0])' 2>/dev/null | grep -qx 3; then
      echo "$cand"
      return 0
    fi
  done
  return 1
}
if ! PY_BIN=$(resolve_py_bin); then
  echo "check-falsify-shard-coverage: nenhum interpretador Python funcional encontrado" >&2
  exit 1
fi

WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-falsify-coverage.XXXXXX")
trap 'rm -rf "$WORKDIR"' EXIT

# shellcheck disable=SC2086
$PY_BIN "$ROOT_DIR/scripts/gen-falsify-chunks.py" "$SCRIPT" "$WORKDIR" "$SHARD_COUNT" > "$WORKDIR/manifest.txt"
cat "$WORKDIR/manifest.txt" >&2

FAIL=0
CHECKED=0
ok()   { echo "OK   [$1]"; CHECKED=$((CHECKED + 1)); }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; CHECKED=$((CHECKED + 1)); }

idx=0
while [[ "$idx" -lt "$SHARD_COUNT" ]]; do
  actual="$ARTIFACTS_DIR/shard_${idx}.actual"
  rc_file="$ARTIFACTS_DIR/shard_${idx}.rc"

  if [[ ! -f "$actual" || ! -f "$rc_file" ]]; then
    fail "shard-coverage/shard_${idx}/present" \
      "artefato ausente (esperado shard_${idx}.actual + shard_${idx}.rc em $ARTIFACTS_DIR) -- job da matriz nao rodou ou upload falhou"
    idx=$((idx + 1))
    continue
  fi

  shard_rc=$(cat "$rc_file" 2>/dev/null || echo "?")
  if [[ "$shard_rc" != "0" ]]; then
    fail "shard-coverage/shard_${idx}/rc" \
      "shard_${idx} reportou rc=${shard_rc} (chunk falhou, ou a guarda local do shard reprovou -- ver log shard_${idx}.log)"
  else
    ok "shard-coverage/shard_${idx}/rc"
  fi

  label_miss=0
  while IFS= read -r line; do
    [[ "$line" == "chunk=$idx label="* ]] || continue
    expected="${line#chunk=$idx label=}"
    if ! grep -qxF "$expected" "$actual"; then
      fail "shard-coverage/shard_${idx}/label/${expected}" \
        "rotulo esperado AUSENTE do conjunto emitido pelo shard ${idx}: $expected"
      label_miss=1
    fi
  done < "$WORKDIR/manifest.txt"

  while IFS= read -r line; do
    [[ "$line" == "chunk=$idx label_glob="* ]] || continue
    prefix="${line#chunk=$idx label_glob=}"
    if ! grep -qF "$prefix" "$actual"; then
      fail "shard-coverage/shard_${idx}/label_glob/${prefix}" \
        "nenhum rotulo emitido pelo shard ${idx} casa o prefixo esperado: ${prefix}*"
      label_miss=1
    fi
  done < "$WORKDIR/manifest.txt"

  if [[ "$label_miss" -eq 0 ]]; then
    ok "shard-coverage/shard_${idx}/labels"
  fi

  idx=$((idx + 1))
done

if [[ "$CHECKED" -eq 0 ]]; then
  echo "check-falsify-shard-coverage: nenhuma verificacao executada -- guarda de vacuidade final (SHARD_COUNT=$SHARD_COUNT)" >&2
  exit 1
fi

echo "check-falsify-shard-coverage: ${CHECKED} verificacao(oes) sobre ${SHARD_COUNT} shard(s)"
exit "$FAIL"
