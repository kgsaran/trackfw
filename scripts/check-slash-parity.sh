#!/usr/bin/env bash
# check-slash-parity.sh — proves the three CLI runtimes (Go, Node.js, Python)
# install the exact same set of `.claude/commands/trackfw/*.md` slash
# commands, byte-for-byte identical, both in name and content.
#
# ML-5D (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador).
#
# Diagnosis this gate closes: the slash-command set is hand-maintained in
# three separate generators (internal/generators/scaffold.go,
# npm/src/generators/init.js, pypi/trackfw/generators/init_gen.py). No gate
# compared all three before this one — check-artifact-parity.sh only checks
# the content of roadmap.md (scenario slash-roadmap-content-drift). Real
# defects (barrier.md missing from Node's forced map, a duplicate map with
# 6/9 commands) were only found by manual inspection.
#
# Strategy: run `<cli> init` in a throwaway directory for each runtime — this
# is the one Go code path that reaches generateClaudeCommandsInner — then
# `diff -r` the three resulting .claude/commands/trackfw/ directories. This
# sidesteps literal-extraction mojibake entirely (nothing parses source
# strings) and covers both the name-set (`diff -r` reports "Only in ...")
# and content (byte diff) requirements in one operation.
#
# Follows the conventions of scripts/check-barrier.sh and
# scripts/check-gates-falsify.sh: set -euo pipefail, mktemp -d fixtures with
# a cleanup trap, "OK [scenario/name]" on success, explicit diagnostic on
# failure. Unlike check-barrier.sh, this script accumulates ALL drift before
# exiting (the check-artifact-parity.sh FAIL=1 pattern) instead of failing on
# the first mismatch — see the note below on why fail-fast would make this
# gate's own falsification scenario vacuous.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-slash-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-slash-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/go" "$WORK/node" "$WORK/python"

(cd "$WORK/go" && "$GO_BIN" init >/dev/null)
(cd "$WORK/node" && node "$ROOT_DIR/npm/bin/trackfw" init >/dev/null)
(cd "$WORK/python" && PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw init >/dev/null)

GO_DIR="$WORK/go/.claude/commands/trackfw"
NODE_DIR="$WORK/node/.claude/commands/trackfw"
PY_DIR="$WORK/python/.claude/commands/trackfw"

# ---------------------------------------------------------------------------
# Vacuity guard — the exact nine slash commands must exist in all three
# runtimes before any comparison. Without this, two empty/partial
# directories would diff clean and the gate would pass vacuously.
# ---------------------------------------------------------------------------
EXPECTED_COMMANDS=(adr.md architect.md barrier.md implement.md move.md req.md roadmap.md status.md validate.md)

FAIL=0
for RUNTIME in go node python; do
  case "$RUNTIME" in
    go)     DIR="$GO_DIR" ;;
    node)   DIR="$NODE_DIR" ;;
    python) DIR="$PY_DIR" ;;
  esac
  if [[ ! -d "$DIR" ]]; then
    echo "check-slash-parity: vacuity guard failed — $RUNTIME did not create $DIR" >&2
    FAIL=1
    continue
  fi
  for CMD in "${EXPECTED_COMMANDS[@]}"; do
    if [[ ! -f "$DIR/$CMD" ]]; then
      echo "slash parity drift: $CMD missing ($RUNTIME) — vacuity guard failed" >&2
      FAIL=1
    fi
  done
  ACTUAL_COUNT=$(find "$DIR" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')
  if [[ "$ACTUAL_COUNT" -ne "${#EXPECTED_COMMANDS[@]}" ]]; then
    echo "slash parity drift: $RUNTIME installed $ACTUAL_COUNT commands, expected ${#EXPECTED_COMMANDS[@]} (unexpected extra/missing file)" >&2
    FAIL=1
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-slash-parity: vacuity guard failed — generation incomplete, comparison aborted" >&2
  exit 1
fi

echo "OK   [slash-parity/vacuity-guard]"

# ---------------------------------------------------------------------------
# Name-set + byte-content comparison. diff -r reports both:
#   "Only in <dir>: <file>"          → name-set drift
#   "Files <a> and <b> differ"       → content drift (with unified diff below)
# Accumulate all three pairwise comparisons before exiting so that a single
# run reports every divergent file at once, not just the first.
# ---------------------------------------------------------------------------
GO_VS_NODE_OK=1
GO_VS_PY_OK=1

if ! diff -ru "$GO_DIR" "$NODE_DIR" >"$WORK/diff-go-node.txt" 2>&1; then
  GO_VS_NODE_OK=0
  FAIL=1
fi

if ! diff -ru "$GO_DIR" "$PY_DIR" >"$WORK/diff-go-py.txt" 2>&1; then
  GO_VS_PY_OK=0
  FAIL=1
fi

# extract_drifted_files DIFF_FILE LABEL — echoes one "slash parity drift: ..."
# line per divergent filename found in a `diff -ru DIR_A DIR_B` output.
#   "Only in <dir>: <file>"        → name-set drift (file missing on one side)
#   "--- <dir_a>/<file>\t<date>"   → unified-diff header for a content-differing
#                                    file (diff -ru never prints the classic
#                                    "Files X and Y differ" line for text files
#                                    — only "--- "/"+++ " unified headers)
extract_drifted_files() {
  local diff_file=$1 label=$2
  while IFS= read -r line; do
    if [[ "$line" == "Only in"* ]]; then
      local file
      file=$(echo "$line" | sed -E 's/^Only in [^:]+: //')
      echo "slash parity drift: $file ($label) — file missing in one runtime" >&2
    elif [[ "$line" == "--- "* ]]; then
      local file
      file=$(basename "$(echo "$line" | sed -E 's/^--- ([^	]+).*/\1/')")
      echo "slash parity drift: $file ($label)" >&2
    fi
  done <"$diff_file"
}

if [[ "$GO_VS_NODE_OK" -eq 0 ]]; then
  echo "slash parity drift (go vs node):" >&2
  extract_drifted_files "$WORK/diff-go-node.txt" "go vs node"
  sed 's/^/  /' "$WORK/diff-go-node.txt" >&2
fi

if [[ "$GO_VS_PY_OK" -eq 0 ]]; then
  echo "slash parity drift (go vs python):" >&2
  extract_drifted_files "$WORK/diff-go-py.txt" "go vs python"
  sed 's/^/  /' "$WORK/diff-go-py.txt" >&2
fi

if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi

echo "OK   [slash-parity/three-runtimes-identical]"
echo "Slash command parity checks passed (${#EXPECTED_COMMANDS[@]} commands x 3 runtimes)."
