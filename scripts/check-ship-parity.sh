#!/usr/bin/env bash
# check-ship-parity.sh — proves `trackfw ship` behaves byte-for-byte identically in Go, Node.js,
# and Python for the chore/docs branch-type governance exemption introduced by
# docs/roadmaps/wip/ROADMAP-2026-08-16-trackfw-ship-aceita-branches-chore-e-docs-sem-gate-de-roadmap.md
# (the `ship` sibling of `trackfw branch new`'s #177 fix), plus a non-regression assertion that
# feat/fix/refactor remain hard-gated.
#
# No check-ship-parity.sh existed before this ML — `trackfw ship` had no dedicated behavioral
# parity script (only the command-surface floor check in check-cli-parity.sh). This script fills
# that gap, following the exact conventions of scripts/check-branch-new-parity.sh and
# scripts/check-commit-parity.sh: set -euo pipefail, mktemp -d fixtures with a cleanup trap,
# BASH_SOURCE-relative ROOT_DIR, ok()/fail() accumulating FAIL=1.
#
# Scenarios (a)/(b) below — the new chore/docs branch-type skip, fully under this ML's control —
# use a byte-level diff -u of both stdout and stderr (assert_three_way), same as the sibling
# scripts. Scenarios (c)/(d) hit code paths with a PRE-EXISTING, out-of-scope cross-runtime
# divergence unrelated to this ML: checkGovernance's real violation-message wording differs
# ("nor done/" in Go vs not in Node/Python), and ship.go never sets SilenceErrors (unlike
# branch.go/commit.go), so Go's cobra error lands on stderr ("Error: ...") while Node/Python
# write the equivalent message explicitly to stdout ("error: ..."). Those two scenarios therefore
# use targeted content (grep) + exit-code assertions instead of a full-stream diff, so this gate
# proves what this ML actually changed without going red on a pre-existing, unrelated gap.
#
# Every scenario below stages a NON-doc file (not just *.md/docs//vault/) — staging only doc
# files would make every chore/docs scenario pass via the pre-existing doc-only exception
# (allDocOnly) and prove nothing about the new branch-type exemption. See advisor review in the
# implementing ML's transcript.
#
# Scenarios:
#   (a) chore/<slug>, wip/ EMPTY, non-doc file staged, --dry-run --no-pr → exit 0, stdout
#       contains the branch-type skip marker "Governance: skipped (chore/docs branch)".
#   (b) docs/<slug> — same as (a).
#   (c) non-regression: feat/<slug>, wip/ EMPTY, non-doc file staged → exit 1, "governance check
#       failed", and the branch-type skip marker must be ABSENT — proves loosening the gate for
#       chore/docs did not loosen it for feat/fix/refactor.
#   (d) branch outside the ship vocabulary (feat|fix|refactor|chore|docs), non-doc file staged →
#       exit 1, "does not match the required pattern" — byte-identical stderr across runtimes.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb
export TRACKFW_DISABLE_EXTERNAL_COMMANDS=1

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-ship-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-branch-new-parity.sh / check-commit-parity.sh.
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
  echo "check-ship-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-ship-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# run_ship RUNTIME DIR ARGS...
# Runs `trackfw ship ARGS...` from DIR. Sets SH_EXIT and writes stdout/stderr to
# $WORK/<label>.<runtime>.out / .err (label passed by caller via SH_LABEL).
run_ship() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/$SH_LABEL.$runtime.out" err_file="$WORK/$SH_LABEL.$runtime.err"
  set +e
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" ship "$@")                              >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && node "$NODE_CLI" ship "$@")                       >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw ship "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_ship: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  SH_EXIT=$?
  set -e
  SH_OUT_FILE=$out_file
  SH_ERR_FILE=$err_file
}

# assert_three_way LABEL — diffs go vs node and go vs py for both stdout and stderr previously
# captured under $WORK/<label>.<runtime>.{out,err}, plus exit codes recorded in
# $WORK/<label>.<runtime>.exit.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "ship-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "ship-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "ship-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "ship-parity/$label"
  fi
}

# assert_exit_equal LABEL — exit codes recorded in $WORK/<label>.<runtime>.exit must match
# across all three runtimes. Used instead of assert_three_way for scenarios that exercise
# checkGovernance's real (non-mocked) violation path or the cobra-vs-explicit-writeln error
# surface, both of which carry a pre-existing, out-of-scope cross-runtime divergence in wording
# and stdout/stderr placement (see vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md) —
# unrelated to the chore/docs behavior this script proves. Content is instead checked with
# targeted grep assertions in the caller, anchored on the exact strings this ML controls.
assert_exit_equal() {
  local label=$1
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "ship-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
  else
    ok "ship-parity/$label"
  fi
}

# assert_message_byte_identical LABEL — proves the message BODY is byte-identical across
# runtimes despite the pre-existing stdout/stderr placement divergence documented in
# vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md: concatenates
# stdout+stderr per runtime (the message lands on one or the other depending on runtime) and
# normalizes only the "Error: " (Go/cobra) vs "error: " (Node/Python/writeln) prefix before
# diffing — a real content drift anywhere else in the message still fails this. Also asserts
# exit codes match.
assert_message_byte_identical() {
  local label=$1
  local diverged=0
  norm() { cat "$1" "$2" 2>/dev/null | sed -e 's/^Error: /error: /'; }
  if ! diff -u <(norm "$WORK/$label.go.out" "$WORK/$label.go.err") \
               <(norm "$WORK/$label.node.out" "$WORK/$label.node.err") \
               >"$WORK/$label.diff.go-node.msg" 2>&1; then
    fail "ship-parity/$label/go-vs-node/message" "message body diverges:
$(cat "$WORK/$label.diff.go-node.msg")"
    diverged=1
  fi
  if ! diff -u <(norm "$WORK/$label.go.out" "$WORK/$label.go.err") \
               <(norm "$WORK/$label.py.out" "$WORK/$label.py.err") \
               >"$WORK/$label.diff.go-py.msg" 2>&1; then
    fail "ship-parity/$label/go-vs-py/message" "message body diverges:
$(cat "$WORK/$label.diff.go-py.msg")"
    diverged=1
  fi
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "ship-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "ship-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Shared fixture: real git repo (symbolic-ref --short HEAD needs a real branch + commit), a
# minimal governance tree, and one staged NON-doc file so the doc-only exception never masks
# the branch-type exemption under test.
# ---------------------------------------------------------------------------
make_fixture() {
  local dir=$1 branch=$2
  mkdir -p "$dir/docs/roadmaps/wip" "$dir/docs/roadmaps/done" "$dir/docs/req" "$dir/docs/adr"
  cat >"$dir/trackfw.yaml" <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dir: docs/adr
EOF
  (
    cd "$dir"
    git init -q
    git config user.email test@example.com
    git config user.name "trackfw parity gate"
    echo init >README.md
    git add README.md
    git commit -qm init
    git checkout -q -b "$branch"
    echo "non-doc content" >src.txt
    git add src.txt
  )
}

# ---------------------------------------------------------------------------
# Scenario (a) — chore/<slug>: wip/ EMPTY, non-doc file staged, --dry-run --no-pr.
# ---------------------------------------------------------------------------
SH_LABEL="chore-skips-gate"
for runtime in go node py; do
  fixture="$WORK/a-$runtime"
  make_fixture "$fixture" "chore/release-x.y.z"
  run_ship "$runtime" "$fixture" -m "chore: release x.y.z" --dry-run --no-pr
  echo "$SH_EXIT" >"$WORK/$SH_LABEL.$runtime.exit"
  if [[ "$SH_EXIT" -ne 0 ]]; then
    fail "ship-parity/$SH_LABEL/$runtime" "expected exit 0 (no gate for chore), got $SH_EXIT; stderr: $(cat "$SH_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'Governance: skipped (chore/docs branch)' "$SH_OUT_FILE"; then
    fail "ship-parity/$SH_LABEL/$runtime" "vacuity guard: stdout missing branch-type skip marker; stdout: $(cat "$SH_OUT_FILE")"
    continue
  fi
done
assert_three_way "$SH_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — docs/<slug>: same as (a).
# ---------------------------------------------------------------------------
SH_LABEL="docs-skips-gate"
for runtime in go node py; do
  fixture="$WORK/b-$runtime"
  make_fixture "$fixture" "docs/update-readme"
  run_ship "$runtime" "$fixture" -m "docs: update readme" --dry-run --no-pr
  echo "$SH_EXIT" >"$WORK/$SH_LABEL.$runtime.exit"
  if [[ "$SH_EXIT" -ne 0 ]]; then
    fail "ship-parity/$SH_LABEL/$runtime" "expected exit 0 (no gate for docs), got $SH_EXIT; stderr: $(cat "$SH_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'Governance: skipped (chore/docs branch)' "$SH_OUT_FILE"; then
    fail "ship-parity/$SH_LABEL/$runtime" "vacuity guard: stdout missing branch-type skip marker; stdout: $(cat "$SH_OUT_FILE")"
    continue
  fi
done
assert_three_way "$SH_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — non-regression: feat/<slug> WITHOUT a matching roadmap still blocks, and the
# branch-type skip marker must never appear for a gated branch.
# ---------------------------------------------------------------------------
SH_LABEL="feat-still-gated-non-regression"
for runtime in go node py; do
  fixture="$WORK/c-$runtime"
  make_fixture "$fixture" "feat/no-roadmap-for-this"
  run_ship "$runtime" "$fixture" -m "feat: x" --dry-run --no-pr
  echo "$SH_EXIT" >"$WORK/$SH_LABEL.$runtime.exit"
  if [[ "$SH_EXIT" -eq 0 ]]; then
    fail "ship-parity/$SH_LABEL/$runtime" "expected non-zero exit (gate must still block feat without a roadmap), got 0"
    continue
  fi
  if ! grep -qF 'Governance check failed' "$SH_OUT_FILE"; then
    fail "ship-parity/$SH_LABEL/$runtime" "vacuity guard: stdout missing 'Governance check failed'; stdout: $(cat "$SH_OUT_FILE")"
    continue
  fi
  if grep -qF 'Governance: skipped' "$SH_OUT_FILE"; then
    fail "ship-parity/$SH_LABEL/$runtime" "feat branch must never print a governance-skipped message; stdout: $(cat "$SH_OUT_FILE")"
    continue
  fi
done
# Content (not full-stream diff): checkGovernance's real violation-message wording and the
# cobra-vs-explicit-writeln stdout/stderr split are a pre-existing, out-of-scope divergence
# unrelated to this ML (both runtimes agree on "governance check failed" and the absence of the
# skip marker, asserted above per-runtime).
assert_exit_equal "$SH_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — branch outside the ship vocabulary (feat|fix|refactor|chore|docs): blocked at
# Step 1, byte-identical error message across runtimes.
# ---------------------------------------------------------------------------
SH_LABEL="invalid-branch-vocabulary"
for runtime in go node py; do
  fixture="$WORK/d-$runtime"
  make_fixture "$fixture" "hotfix/whatever"
  run_ship "$runtime" "$fixture" -m "fix: x" --dry-run --no-pr
  echo "$SH_EXIT" >"$WORK/$SH_LABEL.$runtime.exit"
  if [[ "$SH_EXIT" -eq 0 ]]; then
    fail "ship-parity/$SH_LABEL/$runtime" "expected non-zero exit for out-of-vocabulary branch, got 0"
    continue
  fi
  if ! grep -qF 'does not match the required pattern feat|fix|refactor|chore|docs/<slug>' "$SH_OUT_FILE" "$SH_ERR_FILE" 2>/dev/null; then
    fail "ship-parity/$SH_LABEL/$runtime" "vacuity guard: output missing full vocabulary in the pattern error; stdout: $(cat "$SH_OUT_FILE"); stderr: $(cat "$SH_ERR_FILE")"
    continue
  fi
done
# Normalized byte diff (not a raw full-stream diff): Go's cobra prints this error on stderr
# (Error: ...) while Node/Python write it explicitly to stdout (error: ...) — a pre-existing,
# out-of-scope stdout/stderr placement divergence (ship.go never sets SilenceErrors, unlike
# branch.go and commit.go). assert_message_byte_identical concatenates both streams and
# normalizes only that prefix, so a real drift anywhere else in the message (e.g. the
# vocabulary list, or the "git checkout -b feat/<slug>" hint) still fails this gate.
assert_message_byte_identical "$SH_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-ship-parity.sh scenarios passed."
else
  echo "check-ship-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
