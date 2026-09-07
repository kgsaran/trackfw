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
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SCRIPT="$ROOT_DIR/scripts/check-gates-falsify.sh"
GEN="$ROOT_DIR/scripts/gen-falsify-chunks.py"

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
  exit 1
fi

exit 0
