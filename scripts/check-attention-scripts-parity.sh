#!/usr/bin/env bash
# check-attention-scripts-parity.sh — proves the two attention-hook scripts
# (scripts/trackfw-attention-signal.sh and scripts/trackfw-attention-cleanup.sh),
# emitted by `trackfw discover --init`, are byte-identical across Go, Node.js
# and Python. The three runtimes each embed the script content as a source
# literal (internal/generators/scaffold.go, npm/src/generators/hooks.js,
# pypi/trackfw/generators/init_gen.py) — nothing enforced this stayed in sync
# until now, and the three literals drifted (comment language, a blank line,
# `sed` invocation style) without any gate noticing
# (see docs/req/REQ-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md).
#
# Follows the conventions of scripts/check-branch-new-parity.sh: set -euo pipefail,
# mktemp -d fixture with a cleanup trap, BASH_SOURCE-relative ROOT_DIR, GO_BIN
# resolution (build a throwaway binary if unset), byte-level diff -u between
# runtimes (never a hash-only comparison — a hash mismatch alone gives no
# actionable diagnostic).
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-attention-scripts-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-branch-new-parity.sh:
#   GO_BIN unset → build a throwaway binary so the script also works standalone.
#   GO_BIN relative → prefix with ROOT_DIR.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-attention-scripts-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-attention-scripts-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Generate the two scripts with each runtime via `discover --init` on a fresh,
# empty fixture directory (the same entry point used in production — no
# reliance on internal generator APIs, so the gate also catches a runtime that
# stops wiring the generator into `discover --init`).
# ---------------------------------------------------------------------------
run_discover_init() {
  local runtime=$1 dir=$2
  mkdir -p "$dir"
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" discover --init)                              >/dev/null 2>"$WORK/$runtime.err" ;;
    node) (cd "$dir" && node "$NODE_CLI" discover --init)                       >/dev/null 2>"$WORK/$runtime.err" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw discover --init) >/dev/null 2>"$WORK/$runtime.err" ;;
    *)    echo "run_discover_init: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
}

GO_DIR="$WORK/go"
NODE_DIR="$WORK/node"
PY_DIR="$WORK/py"

set +e
run_discover_init go "$GO_DIR"
GO_STATUS=$?
run_discover_init node "$NODE_DIR"
NODE_STATUS=$?
run_discover_init py "$PY_DIR"
PY_STATUS=$?
set -e

for pair in "go:$GO_STATUS" "node:$NODE_STATUS" "py:$PY_STATUS"; do
  runtime=${pair%%:*}
  status=${pair##*:}
  if [[ "$status" -ne 0 ]]; then
    fail "attention-scripts-parity/$runtime/discover-init" \
      "discover --init exited $status; stderr: $(cat "$WORK/$runtime.err" 2>/dev/null)"
  fi
done

# ---------------------------------------------------------------------------
# P2 vacuity guard: if any runtime failed to even produce the two files, a
# byte-for-byte diff of empty/missing files would trivially "pass" — assert
# both scripts exist and are non-empty for all three runtimes before diffing.
# ---------------------------------------------------------------------------
for runtime_dir in "go:$GO_DIR" "node:$NODE_DIR" "py:$PY_DIR"; do
  runtime=${runtime_dir%%:*}
  dir=${runtime_dir##*:}
  for script in trackfw-attention-signal.sh trackfw-attention-cleanup.sh; do
    path="$dir/scripts/$script"
    if [[ ! -s "$path" ]]; then
      fail "attention-scripts-parity/$runtime/$script" "missing or empty: $path"
    fi
  done
done

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-attention-scripts-parity.sh: one or more scenarios FAILED (setup)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Byte-for-byte diff of both scripts, go-vs-node and go-vs-py.
# ---------------------------------------------------------------------------
for script in trackfw-attention-signal.sh trackfw-attention-cleanup.sh; do
  go_file="$GO_DIR/scripts/$script"
  node_file="$NODE_DIR/scripts/$script"
  py_file="$PY_DIR/scripts/$script"

  if diff -u "$go_file" "$node_file" >"$WORK/$script.diff.go-node" 2>&1; then
    ok "attention-scripts-parity/$script/go-vs-node"
  else
    fail "attention-scripts-parity/$script/go-vs-node" \
      "byte drift between Go and Node.js:
$(cat "$WORK/$script.diff.go-node")"
  fi

  if diff -u "$go_file" "$py_file" >"$WORK/$script.diff.go-py" 2>&1; then
    ok "attention-scripts-parity/$script/go-vs-py"
  else
    fail "attention-scripts-parity/$script/go-vs-py" \
      "byte drift between Go and Python:
$(cat "$WORK/$script.diff.go-py")"
  fi
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-attention-scripts-parity.sh scenarios passed."
else
  echo "check-attention-scripts-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
