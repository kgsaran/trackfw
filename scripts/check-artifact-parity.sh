#!/usr/bin/env bash
# check-artifact-parity.sh — Verifica que os 3 CLIs (Go, Node.js, Python)
# geram artefatos byte-a-byte idênticos (conteúdo e nome de arquivo).
#
# Artefatos verificados: req, adr, roadmap, note + vault/notes/index.md
#
# Título utilizado: contém acento e cedilha para validar a normalização NFKD
# de slug portável nos 3 runtimes (REQ-2026-07-27-convergencia-templates-python).
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
# Garantir caminho absoluto — Makefile pode passar um path relativo (ex: bin/trackfw)
# que ficaria inválido ao fazer `cd "$WORK/<runtime>"` dentro das subshells.
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-artifact-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/go" "$WORK/node" "$WORK/python"

TITLE="Autenticação e Sessão"

# ── Midnight rollover guard ──────────────────────────────────────────────────
# Captura a data ANTES da geração. Se a data mudar durante o processo os nomes
# de arquivo serão inconsistentes entre runtimes — falha explícita em vez de
# diagnóstico misterioso de diff.
DATE_BEFORE=$(date +%F)

# ── Geração dos artefatos ────────────────────────────────────────────────────
(cd "$WORK/go" && "$GO_BIN"                                       req     new "$TITLE")
(cd "$WORK/go" && "$GO_BIN"                                       adr     new "$TITLE")
(cd "$WORK/go" && "$GO_BIN"                                       roadmap new "$TITLE")
(cd "$WORK/go" && "$GO_BIN"                                       note    new "$TITLE")

(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              req     new "$TITLE")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              adr     new "$TITLE")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              roadmap new "$TITLE")
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw"              note    new "$TITLE")

(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw req     new "$TITLE")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw adr     new "$TITLE")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw roadmap new "$TITLE")
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw note    new "$TITLE")

DATE_AFTER=$(date +%F)
if [[ "$DATE_BEFORE" != "$DATE_AFTER" ]]; then
  echo "check-artifact-parity: data rolou durante a geração ($DATE_BEFORE → $DATE_AFTER)" >&2
  echo "  Reexecute o gate." >&2
  exit 1
fi

DATE="$DATE_AFTER"
SLUG="autenticacao-e-sessao"

# ── Caminhos esperados por tipo ──────────────────────────────────────────────
# EXPECTED_<KIND> é o caminho relativo dentro de cada WORK/<runtime>/.
# O vacuity guard usa esses paths para garantir que cada runtime gerou exatamente
# o arquivo esperado — zero arquivos → falha explícita, nunca passe trivial.
EXPECTED_REQ="docs/req/REQ-${DATE}-${SLUG}.md"
EXPECTED_ADR="docs/adr/ADR-${DATE}-${SLUG}.md"
EXPECTED_ROADMAP="docs/roadmaps/backlog/ROADMAP-${DATE}-${SLUG}.md"
EXPECTED_NOTE="vault/notes/${SLUG}-${DATE}.md"
EXPECTED_INDEX="vault/notes/index.md"

KINDS=("req" "adr" "roadmap" "note" "note_index")

expected_path() {
  case "$1" in
    req)        echo "$EXPECTED_REQ"     ;;
    adr)        echo "$EXPECTED_ADR"     ;;
    roadmap)    echo "$EXPECTED_ROADMAP" ;;
    note)       echo "$EXPECTED_NOTE"    ;;
    note_index) echo "$EXPECTED_INDEX"   ;;
  esac
}

# ── Vacuity guard ────────────────────────────────────────────────────────────
FAIL=0
for KIND in "${KINDS[@]}"; do
  REL=$(expected_path "$KIND")
  for RUNTIME in go node python; do
    TARGET="$WORK/$RUNTIME/$REL"
    if [[ ! -f "$TARGET" ]]; then
      echo "artifact parity drift: $KIND ($RUNTIME) — arquivo ausente: $REL" >&2
      FAIL=1
    fi
  done
done

if [[ $FAIL -ne 0 ]]; then
  echo "check-artifact-parity: vacuity guard falhou — geração incompleta, comparação abortada" >&2
  exit 1
fi

# ── Comparação conteúdo e nome de arquivo ────────────────────────────────────
# Nome: os paths relativos dentro de WORK/<runtime> são idênticos por construção
# (todos usam a mesma EXPECTED_<KIND>). O vacuity guard acima já confirmou que
# cada arquivo existe no path exato esperado — a divergência de nome de arquivo
# é detectada quando o runtime gera um path diferente do esperado (vacuity falha).
#
# Conteúdo: diff byte-a-byte acumulando todos os erros antes de sair,
# para que o diagnóstico cubra todos os artefatos divergentes de uma vez.
for KIND in "${KINDS[@]}"; do
  REL=$(expected_path "$KIND")
  GO_FILE="$WORK/go/$REL"
  NODE_FILE="$WORK/node/$REL"
  PY_FILE="$WORK/python/$REL"

  if ! diff -q "$GO_FILE" "$NODE_FILE" >/dev/null 2>&1; then
    echo "artifact parity drift: $KIND (go vs node)" >&2
    diff "$GO_FILE" "$NODE_FILE" >&2 || true
    FAIL=1
  fi

  if ! diff -q "$GO_FILE" "$PY_FILE" >/dev/null 2>&1; then
    echo "artifact parity drift: $KIND (go vs python)" >&2
    diff "$GO_FILE" "$PY_FILE" >&2 || true
    FAIL=1
  fi
done

if [[ $FAIL -ne 0 ]]; then
  exit 1
fi

echo "Artifact parity checks passed (5 artifact types × 3 runtimes)"
