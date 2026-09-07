#!/usr/bin/env bash
# Driver de paralelização do check-gates-falsify.sh (ML-2D,
# ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify).
#
# Mecanismo de isolamento: um processo bash por chunk. Cada chunk é o mesmo
# preâmbulo do script original (byte a byte, via gen-falsify-chunks.py) —
# então cada chunk cria seu PRÓPRIO $WORK (mktemp -d) e seu PRÓPRIO
# $HOME="$WORK/home", exatamente como o script original cria um único desses
# para si. Nenhum estado é compartilhado ENTRE chunks, com estas exceções
# deliberadas (nomeadas, não descobertas por acidente):
#   - GOPATH/GOCACHE/GOMODCACHE: valores REAIS do ambiente, iguais em todos os
#     chunks — preserva o cache de build quente que o ML-1A mediu (mediana
#     0,84s/build); risco aceito de contenção de lock sob build concorrente,
#     já documentado no ML-1A.
#   - $ROOT_DIR/bin/trackfw: lido (nunca escrito) por muitos cenários via
#     GO_BIN — nenhum chunk o reconstrói.
#   - Os arquivos de chunk materializados NÃO vivem dentro de $ROOT_DIR: o
#     WORKDIR deste driver é seu próprio `mktemp -d`, fora da árvore do repo —
#     o Cenário 18 (no-repo-mutation) audita `git status --porcelain` sobre
#     $ROOT_DIR, e materializar chunk em /tmp evita que a própria geração seja
#     contada como mutação da árvore.
#
# Falha em qualquer chunk propaga: o driver agrega o exit code de todos os
# processos e sai não-zero se qualquer um falhar (nunca mascara falha parcial
# como sucesso do conjunto).
#
# Guarda de CONJUNTO (ML-2D, correção pós-reprovação): rc != 0 já denunciava
# falha antes, mas não NOMEAVA cobertura perdida -- o incidente medido teve
# rc=1 e 77 rótulos silenciosamente ausentes, achados só por diff manual do
# arquiteto. Duas checagens independentes, nenhuma lista congelada:
#   1. Sentinela por chunk: gen-falsify-chunks.py grava "CHUNK_COMPLETE $i"
#      como ÚLTIMA linha de todo chunk materializado. Se o log não termina
#      nela, o chunk morreu no meio (crash, kill, `exit` cedo) mesmo que o
#      exit code agregado por algum motivo não tivesse propagado.
#   2. Rótulos esperados: o gerador extrai, do PRÓPRIO texto de cada chunk,
#      os rótulos que os assert_* daquele chunk podem emitir (literais e,
#      para rótulo com `$var` resolvido só em runtime, um prefixo glob). O
#      driver confere que cada um aparece nos logs (OK/FAIL/PROOF) do MESMO
#      chunk -- e nomeia o que faltar.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# TRACKFW_FALSIFY_SCRIPT: só para prova por sabotagem do próprio driver (ML-2D)
# -- aponta o gerador+guarda para um fonte sintético minúsculo em vez do
# check-gates-falsify.sh real, sem precisar de uma cópia paralela deste
# script. Sem override, comportamento em produção é idêntico ao anterior.
SCRIPT="${TRACKFW_FALSIFY_SCRIPT:-$ROOT_DIR/scripts/check-gates-falsify.sh}"
# TRACKFW_FALSIFY_GEN: mesmo motivo do override acima -- só para sabotagem
# do próprio gerador em prova de falsificação (ver vault/notes do ML-2D).
GEN="${TRACKFW_FALSIFY_GEN:-$ROOT_DIR/scripts/gen-falsify-chunks.py}"

# Grau de paralelismo: parametrizável via TRACKFW_FALSIFY_JOBS. Sem override,
# descobre o nº de CPUs em runtime (nunca hardcoded) com piso 1 e teto 8 —
# o runner do CI tem 4 vCPUs; máquinas de desenvolvimento podem ter mais, mas
# acima de 8 o SO já não entrega paralelismo real para uma carga dominada por
# `go build` (mesma observação de contenção do ML-1A/nota do job `parity`).
detect_cpus() {
  if command -v nproc >/dev/null 2>&1; then
    nproc
  elif command -v sysctl >/dev/null 2>&1; then
    sysctl -n hw.ncpu
  elif command -v getconf >/dev/null 2>&1; then
    getconf _NPROCESSORS_ONLN
  else
    echo 4
  fi
}

if [[ -n "${TRACKFW_FALSIFY_JOBS:-}" ]]; then
  JOBS="$TRACKFW_FALSIFY_JOBS"
else
  JOBS=$(detect_cpus)
  [[ "$JOBS" -lt 1 ]] && JOBS=1
  [[ "$JOBS" -gt 8 ]] && JOBS=8
fi

if [[ "$JOBS" -le 1 ]]; then
  echo "run-gates-falsify-parallel: JOBS=$JOBS -- executando serial (script original, sem split)" >&2
  exec bash "$SCRIPT"
fi

WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-falsify-parallel.XXXXXX")
trap 'rm -rf "$WORKDIR"' EXIT

python3 "$GEN" "$SCRIPT" "$WORKDIR" "$JOBS" > "$WORKDIR/manifest.txt"
cat "$WORKDIR/manifest.txt" >&2

mapfile -t CHUNKS < <(find "$WORKDIR" -maxdepth 1 -name 'chunk_*.sh' | sort)
if [[ ${#CHUNKS[@]} -eq 0 ]]; then
  echo "run-gates-falsify-parallel: gerador nao produziu nenhum chunk" >&2
  exit 1
fi

echo "run-gates-falsify-parallel: ${#CHUNKS[@]} chunks (JOBS solicitado=$JOBS)" >&2

PIDS=()
for chunk in "${CHUNKS[@]}"; do
  log="${chunk%.sh}.log"
  ( TRACKFW_ROOT_DIR="$ROOT_DIR" bash "$chunk" ) >"$log" 2>&1 &
  PIDS+=("$!:$chunk:$log")
done

FAILED=0
for entry in "${PIDS[@]}"; do
  pid="${entry%%:*}"
  rest="${entry#*:}"
  chunk="${rest%%:*}"
  log="${rest#*:}"
  if ! wait "$pid"; then
    FAILED=1
    echo "run-gates-falsify-parallel: FALHOU $chunk -- log:" >&2
    cat "$log" >&2
  else
    cat "$log"
  fi
done

if [[ "$FAILED" -ne 0 ]]; then
  echo "run-gates-falsify-parallel: pelo menos um chunk reprovou -- exit != 0" >&2
fi

# --- Guarda de conjunto ------------------------------------------------
# Roda SEMPRE, mesmo se algum chunk já reprovou -- é exatamente o caso do
# incidente medido (rc=1, cobertura perdida sem ninguém nomeá-la).
COVERAGE_FAILED=0

for chunk in "${CHUNKS[@]}"; do
  idx=$(basename "$chunk" .sh)
  idx=${idx#chunk_}
  log="${chunk%.sh}.log"

  last_line=$(tail -n 1 "$log" 2>/dev/null || true)
  if [[ "$last_line" != "CHUNK_COMPLETE $idx" ]]; then
    COVERAGE_FAILED=1
    echo "run-gates-falsify-parallel: GUARDA -- chunk_$idx nao chegou ao sentinela CHUNK_COMPLETE (ultima linha do log: '$last_line') -- chunk morreu no meio, cobertura potencialmente perdida" >&2
  fi

  # Rótulos emitidos por este chunk (OK/FAIL/PROOF [falsify/label]...).
  actual_labels_file="$WORKDIR/chunk_${idx}.actual"
  grep -oE '^(OK|FAIL|PROOF)[[:space:]]+\[falsify/[^]]+\]' "$log" 2>/dev/null \
    | sed -E 's/^(OK|FAIL|PROOF)[[:space:]]+\[falsify\///; s/\]$//' \
    > "$actual_labels_file" || true

  while IFS= read -r line; do
    [[ "$line" == "chunk=$idx label="* ]] || continue
    expected="${line#chunk=$idx label=}"
    if ! grep -qxF "$expected" "$actual_labels_file"; then
      COVERAGE_FAILED=1
      echo "run-gates-falsify-parallel: GUARDA -- chunk_$idx: rotulo esperado AUSENTE: $expected" >&2
    fi
  done < "$WORKDIR/manifest.txt"

  while IFS= read -r line; do
    [[ "$line" == "chunk=$idx label_glob="* ]] || continue
    prefix="${line#chunk=$idx label_glob=}"
    if ! grep -qF "$prefix" "$actual_labels_file"; then
      COVERAGE_FAILED=1
      echo "run-gates-falsify-parallel: GUARDA -- chunk_$idx: nenhum rotulo emitido casa o prefixo esperado: ${prefix}*" >&2
    fi
  done < "$WORKDIR/manifest.txt"
done

if [[ "$COVERAGE_FAILED" -ne 0 ]]; then
  echo "run-gates-falsify-parallel: guarda de conjunto reprovou -- cobertura perdida, ver GUARDA acima -- exit != 0" >&2
  exit 1
fi

if [[ "$FAILED" -ne 0 ]]; then
  exit 1
fi

# Resumo do DRIVER, não do arquivo original -- o `echo "Falsification checks
# passed (all N scenarios...)"` que fecha check-gates-falsify.sh é a última
# linha da última segmento de asserção do arquivo: sob paralelismo ela só
# imprime UMA vez, pelo chunk que ficou com essa segmento (tipicamente
# 20-30 dos 118 segmentos, não os 118), e nesse contexto essa frase describe
# só aquela chunk -- não mais a suíte inteira. A guarda de conjunto acima já
# provou a cobertura completa; este resumo é o que fala pela suíte.
total_ok=$(cat "$WORKDIR"/chunk_*.log 2>/dev/null | grep -c '^OK' || true)
total_fail=$(cat "$WORKDIR"/chunk_*.log 2>/dev/null | grep -c '^FAIL' || true)
echo "run-gates-falsify-parallel: suite completa -- ${#CHUNKS[@]} chunks, ${total_ok} OK, ${total_fail} FAIL, guarda de conjunto OK (nenhum rotulo esperado ausente)" >&2

exit 0
