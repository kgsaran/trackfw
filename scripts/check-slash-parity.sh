#!/usr/bin/env bash
# check-slash-parity.sh — pins that `trackfw init` installs the complete set
# of .claude/commands/trackfw/*.md slash commands in the Go binary.
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. Cross-runtime comparison (go vs node vs python) is no longer
# applicable. This gate now asserts the Go binary's output alone: the nine
# expected slash commands must exist and be non-empty after `trackfw init`.
#
# Original ML-5D (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-
# orquestrador). Cross-runtime diff removed by ML-3A (v8) because npm/src/
# and pypi/trackfw/ were deleted.
#
# Strategy: run `trackfw init` in a throwaway directory, then verify the nine
# resulting .claude/commands/trackfw/ files exist and are non-empty. Vacuity
# guard is retained — a missing file or an empty file would silently pass a
# byte-identity check, so we assert presence and non-emptiness explicitly.
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
  echo "check-slash-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-slash-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/go" "$WORK/home-go"

(cd "$WORK/go" && HOME="$WORK/home-go" "$GO_BIN" init >/dev/null)

GO_DIR="$WORK/go/.claude/commands/trackfw"

# ---------------------------------------------------------------------------
# Vacuity guard — the exact nine slash commands must exist and be non-empty.
# Without this, missing files would pass the count check silently.
# ---------------------------------------------------------------------------
EXPECTED_COMMANDS=(adr.md architect.md barrier.md implement.md move.md req.md roadmap.md status.md validate.md)

FAIL=0

if [[ ! -d "$GO_DIR" ]]; then
  echo "check-slash-parity: Go did not create $GO_DIR — init may have errored" >&2
  exit 1
fi

for CMD in "${EXPECTED_COMMANDS[@]}"; do
  if [[ ! -s "$GO_DIR/$CMD" ]]; then
    echo "slash parity drift: $CMD missing or empty (go) — vacuity guard failed" >&2
    FAIL=1
  fi
done

ACTUAL_COUNT=$(find "$GO_DIR" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')
if [[ "$ACTUAL_COUNT" -ne "${#EXPECTED_COMMANDS[@]}" ]]; then
  echo "slash parity drift: Go installed $ACTUAL_COUNT commands, expected ${#EXPECTED_COMMANDS[@]} (unexpected extra/missing file)" >&2
  FAIL=1
fi

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-slash-parity: vacuity guard failed — generation incomplete" >&2
  exit 1
fi

echo "OK   [slash-parity/vacuity-guard]"

# ---------------------------------------------------------------------------
# All nine commands verified. Gate passes.
# ---------------------------------------------------------------------------
echo "OK   [slash-parity/go-behavioral-pin]"
echo "Slash command parity checks passed (${#EXPECTED_COMMANDS[@]} commands, Go binary only — v8 single-runtime)."
