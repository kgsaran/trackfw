#!/usr/bin/env bash
# check-unknown-command-parity.sh — proves the canonical "unknown command" message
# introduced by ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-
# binario-de-terceiro.md (D3) is byte-for-byte identical, on stderr, exit code 1,
# across Go, Node.js, and Python, for the scenarios pinned in ML-2A of
# ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md:
#
#   (a) no close-enough command exists      → two-line message, no suggestion.
#   (b) a close typo exists ("vaildate")    → three-line message with the exact
#                                              same suggestion in all three CLIs.
#   (c) the removed "plugins" command       → treated as any other unknown
#                                              command — no special-cased message.
#   (d) falsification: a REAL `trackfw-vaildate` executable on PATH must never be
#       invoked — this is the vector ADR D3 closes (the removed plugin-execution
#       fallback used to run exactly this). The fixture executable prints a
#       distinctive marker; its absence from stdout is the proof.
#   (e) unrelated errors keep their pre-existing, deliberately unpinned exit code
#       (argparse's other errors stay 2; this gate must not have widened its own
#       fix to cover errors it wasn't asked to touch).
#
# Follows the exact conventions of scripts/check-commit-parity.sh: set -euo
# pipefail, mktemp -d fixtures with a cleanup trap, BASH_SOURCE-relative ROOT_DIR,
# ok()/fail() accumulating FAIL=1, byte-level diff -u between runtimes on stdout
# and stderr.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-unknown-command-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-commit-parity.sh.
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
  echo "check-unknown-command-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-unknown-command-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# run_cli RUNTIME PATH_PREPEND ARGS...
# Runs `trackfw ARGS...` with PATH_PREPEND (may be empty) prepended to PATH.
# Sets UC_EXIT and writes stdout/stderr to $WORK/<label>.<runtime>.{out,err}
# (label passed by caller via UC_LABEL).
run_cli() {
  local runtime=$1 path_prepend=$2
  shift 2
  local out_file="$WORK/$UC_LABEL.$runtime.out" err_file="$WORK/$UC_LABEL.$runtime.err"
  local run_path="$PATH"
  [[ -n "$path_prepend" ]] && run_path="$path_prepend:$PATH"
  set +e
  case "$runtime" in
    go)   PATH="$run_path" "$GO_BIN" "$@"                              >"$out_file" 2>"$err_file" ;;
    node) PATH="$run_path" node "$NODE_CLI" "$@"                       >"$out_file" 2>"$err_file" ;;
    py)   PATH="$run_path" PYTHONPATH="$PY_ROOT" python3 -m trackfw.cli "$@" >"$out_file" 2>"$err_file" ;;
    *)    echo "run_cli: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  UC_EXIT=$?
  set -e
  UC_OUT_FILE=$out_file
  UC_ERR_FILE=$err_file
}

# assert_three_way LABEL — diffs go vs node and go vs py for stdout and stderr,
# plus exit codes recorded in $WORK/<label>.<runtime>.exit.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "unknown-command-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "unknown-command-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "unknown-command-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$go_exit" != "1" ]]; then
    fail "unknown-command-parity/$label/exit-code" "expected exit 1 in all three, got go=$go_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "unknown-command-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Scenario (a) — no close-enough command: two-line message, no suggestion.
# Vacuity guard: stderr must NOT contain "Did you mean".
# ---------------------------------------------------------------------------
UC_LABEL="no-suggestion"
for runtime in go node py; do
  run_cli "$runtime" "" zzzzzzzzzz-nao-existe
  echo "$UC_EXIT" >"$WORK/$UC_LABEL.$runtime.exit"
  if grep -qF 'Did you mean' "$UC_ERR_FILE"; then
    fail "unknown-command-parity/$UC_LABEL/$runtime" "vacuity guard: unexpected suggestion line; stderr: $(cat "$UC_ERR_FILE")"
  fi
  if ! grep -qF 'unknown command "zzzzzzzzzz-nao-existe" for "trackfw"' "$UC_ERR_FILE"; then
    fail "unknown-command-parity/$UC_LABEL/$runtime" "vacuity guard: canonical message missing; stderr: $(cat "$UC_ERR_FILE")"
  fi
done
assert_three_way "$UC_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — close typo ("vaildate" of "validate"): three-line message,
# same suggestion in all three.
# ---------------------------------------------------------------------------
UC_LABEL="with-suggestion"
for runtime in go node py; do
  run_cli "$runtime" "" vaildate
  echo "$UC_EXIT" >"$WORK/$UC_LABEL.$runtime.exit"
  if ! grep -qF 'Did you mean "validate"?' "$UC_ERR_FILE"; then
    fail "unknown-command-parity/$UC_LABEL/$runtime" "vacuity guard: expected suggestion 'validate' missing; stderr: $(cat "$UC_ERR_FILE")"
  fi
done
assert_three_way "$UC_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — the removed "plugins" command is just another unknown command.
# ---------------------------------------------------------------------------
UC_LABEL="plugins-is-gone"
for runtime in go node py; do
  run_cli "$runtime" "" plugins
  echo "$UC_EXIT" >"$WORK/$UC_LABEL.$runtime.exit"
  if ! grep -qF 'unknown command "plugins" for "trackfw"' "$UC_ERR_FILE"; then
    fail "unknown-command-parity/$UC_LABEL/$runtime" "vacuity guard: canonical message missing for 'plugins'; stderr: $(cat "$UC_ERR_FILE")"
  fi
done
assert_three_way "$UC_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — falsification vector: a REAL trackfw-vaildate executable on
# PATH must never run. This is the exact vector D3 closes (the removed
# plugin-execution fallback used to invoke it on ANY unrecognized command).
# ---------------------------------------------------------------------------
FAKE_BIN_DIR="$WORK/fake-bin"
mkdir -p "$FAKE_BIN_DIR"
cat >"$FAKE_BIN_DIR/trackfw-vaildate" <<'EOF'
#!/bin/sh
echo EXECUTOU_PLUGIN_MALICIOSO
exit 0
EOF
chmod +x "$FAKE_BIN_DIR/trackfw-vaildate"

# Negative control: prove the fixture executable is real (not inert) before
# trusting its absence from CLI output as a negative result (P4).
if [[ "$("$FAKE_BIN_DIR/trackfw-vaildate")" != "EXECUTOU_PLUGIN_MALICIOSO" ]]; then
  echo "check-unknown-command-parity: fixture executable is not a genuine attack — aborting" >&2
  exit 1
fi

UC_LABEL="never-executes-external-binary"
for runtime in go node py; do
  run_cli "$runtime" "$FAKE_BIN_DIR" vaildate
  echo "$UC_EXIT" >"$WORK/$UC_LABEL.$runtime.exit"
  if grep -qF 'EXECUTOU_PLUGIN_MALICIOSO' "$UC_OUT_FILE" "$UC_ERR_FILE"; then
    fail "unknown-command-parity/$UC_LABEL/$runtime" "external binary trackfw-vaildate was executed — plugin dispatch vector reintroduced"
  fi
  if ! grep -qF 'Did you mean "validate"?' "$UC_ERR_FILE"; then
    fail "unknown-command-parity/$UC_LABEL/$runtime" "vacuity guard: canonical message with suggestion missing; stderr: $(cat "$UC_ERR_FILE")"
  fi
done
assert_three_way "$UC_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — unrelated errors keep their pre-existing exit code. Only
# Python's argparse has a distinct exit code (2) for non-unknown-command
# errors; the fix here narrowly targets the "invalid choice" case and must
# not touch this one. Pinned in docs/cli-parity.md "-v is reserved for
# verbose" — this asserts the analogous invariant for a different error
# shape (unrecognized top-level flag).
# ---------------------------------------------------------------------------
UC_LABEL="unrelated-error-exit-code-unchanged"
set +e
PY_FLAG_OUT=$(PYTHONPATH="$PY_ROOT" python3 -m trackfw.cli --this-flag-does-not-exist 2>&1)
PY_FLAG_EXIT=$?
set -e
if [[ "$PY_FLAG_EXIT" -ne 2 ]]; then
  fail "unknown-command-parity/$UC_LABEL/py" "expected argparse's own exit code 2 for an unrelated error (unrecognized flag), got $PY_FLAG_EXIT; output: $PY_FLAG_OUT"
else
  ok "unknown-command-parity/$UC_LABEL"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-unknown-command-parity.sh scenarios passed."
else
  echo "check-unknown-command-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
