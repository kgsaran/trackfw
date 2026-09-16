#!/usr/bin/env bash
# check-rules-parity.sh — pins the governance rules block that `trackfw init
# --ai-tools` injects into GEMINI.md, .github/copilot-instructions.md,
# .windsurfrules and .amazonq/developer/guidelines.md.
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. This gate now asserts the Go binary's output alone — the four rules
# files exist, are non-empty, and contain byte-for-byte identical governance
# blocks delimited by <!-- trackfw:rules:start --> / <!-- trackfw:rules:end -->.
#
# Each AI tool file has a tool-specific heading (e.g. "# GitHub Copilot
# Instructions") so whole-file byte identity is intentionally NOT asserted.
# Only the governance block (between the delimiters) must be identical.
#
# Original ML-5G (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-
# orquestrador). Cross-runtime comparison removed by ML-3A (v8) because
# Node.js and Python CLIs no longer exist (npm/src/ and pypi/trackfw/ deleted).
set -euo pipefail

export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-rules-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-rules-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/go" "$WORK/home-go"

TOOLS="gemini,copilot,windsurf,amazonq"

(cd "$WORK/go" && HOME="$WORK/home-go" "$GO_BIN" init --ai-tools "$TOOLS" >/dev/null)

# Files relative to the project root, one per --ai-tools entry above.
RULES_FILES=(
  "GEMINI.md"
  ".github/copilot-instructions.md"
  ".windsurfrules"
  ".amazonq/developer/guidelines.md"
)

FAIL=0

# ---------------------------------------------------------------------------
# Vacuity guard — the four rules files must exist and be non-empty.
# Without this, missing files would pass silently.
# ---------------------------------------------------------------------------
for FILE in "${RULES_FILES[@]}"; do
  PATH_CANDIDATE="$WORK/go/$FILE"
  if [[ ! -s "$PATH_CANDIDATE" ]]; then
    echo "rules parity drift: $FILE missing or empty (go) — vacuity guard failed" >&2
    FAIL=1
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-rules-parity: vacuity guard failed — generation incomplete, comparison aborted" >&2
  exit 1
fi

echo "OK   [rules-parity/vacuity-guard]"

# ---------------------------------------------------------------------------
# Extract the governance block from each file (between trackfw:rules delimiters)
# and verify all four are byte-for-byte identical.
#
# Each AI tool file has a different H1 heading, so whole-file comparison
# would always fail. The invariant is that the rules BLOCK is the same.
# ---------------------------------------------------------------------------
extract_rules_block() {
  local file="$1"
  awk '/<!-- trackfw:rules:start -->/{found=1} found{print} /<!-- trackfw:rules:end -->/{found=0}' "$file"
}

REFERENCE_FILE="$WORK/go/${RULES_FILES[0]}"
REFERENCE_BLOCK="$WORK/reference-block.txt"
extract_rules_block "$REFERENCE_FILE" > "$REFERENCE_BLOCK"

if [[ ! -s "$REFERENCE_BLOCK" ]]; then
  echo "rules parity: reference file ${RULES_FILES[0]} has no trackfw:rules block — delimiter missing?" >&2
  exit 1
fi

echo "OK   [rules-parity/reference-block-found]"

for FILE in "${RULES_FILES[@]:1}"; do
  CANDIDATE="$WORK/go/$FILE"
  CANDIDATE_BLOCK="$WORK/candidate-block.txt"
  extract_rules_block "$CANDIDATE" > "$CANDIDATE_BLOCK"
  if [[ ! -s "$CANDIDATE_BLOCK" ]]; then
    echo "rules parity: $FILE has no trackfw:rules block — delimiter missing?" >&2
    FAIL=1
    continue
  fi
  if ! cmp -s "$REFERENCE_BLOCK" "$CANDIDATE_BLOCK"; then
    echo "rules parity drift: ${RULES_FILES[0]} rules block differs from $FILE" >&2
    diff -u "$REFERENCE_BLOCK" "$CANDIDATE_BLOCK" >&2 || true
    FAIL=1
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi

echo "OK   [rules-parity/all-ai-tools-identical]"
echo "Rules block parity checks passed (${#RULES_FILES[@]} files, Go binary only — v8 single-runtime)."
