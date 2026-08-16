#!/usr/bin/env bash
# check-branch-new-parity.sh — proves `trackfw branch new <type>/<slug>` behaves
# byte-for-byte identically in Go, Node.js, and Python, covering the contract pinned
# in docs/cli-parity.md (§"trackfw branch new"):
#   (a) no match            → blocks, `git checkout -b` never runs, stdout + the
#                              `blocked: ...` stderr line + exit code identical
#   (b) match + --dry-run   → "would create branch ..." on stdout, exit 0, identical
#   (c) match, real git,
#       branch already exists → git checkout -b actually runs; stdout/stderr/exit
#                              code are Git's own (128), propagated literally —
#                              the one scenario that exercises the production
#                              defaultGitCheckout/_default_git_checkout wrapper,
#                              not just the injectable-fake code paths unit tests
#                              cover (see
#                              vault/notes/branch-new-exit-code-leak-vs-propagation-2026-08-04.md)
#
# Follows the conventions of scripts/check-roadmap-move-parity.sh: set -euo pipefail,
# mktemp -d fixtures with a cleanup trap, BASH_SOURCE-relative ROOT_DIR, ok()/fail()
# accumulating FAIL=1, byte-level diff -u between runtimes on both stdout and stderr.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-branch-new-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-roadmap-move-parity.sh:
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
  echo "check-branch-new-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-branch-new-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# run_branch_new RUNTIME DIR ARGS...
# Runs `trackfw branch new ARGS...` from DIR. Sets BN_EXIT and writes stdout/stderr
# to $WORK/<label>.<runtime>.out / .err (label passed by caller via BN_LABEL).
run_branch_new() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/$BN_LABEL.$runtime.out" err_file="$WORK/$BN_LABEL.$runtime.err"
  set +e
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" branch new "$@")                              >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && node "$NODE_CLI" branch new "$@")                       >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw branch new "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_branch_new: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  BN_EXIT=$?
  set -e
  BN_OUT_FILE=$out_file
  BN_ERR_FILE=$err_file
}

# assert_three_way LABEL — diffs go vs node and go vs py for both stdout and
# stderr previously captured under $WORK/<label>.<runtime>.{out,err}, plus exit
# codes recorded in $WORK/<label>.<runtime>.exit. Fails with a labeled diagnostic
# and the unified diff on divergence (never silently degrades — P2).
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "branch-new-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "branch-new-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "branch-new-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "branch-new-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Shared fixture scaffolding — flat layout, minimal governance tree.
# ---------------------------------------------------------------------------
make_base() {
  local dir=$1
  mkdir -p "$dir/docs/roadmaps/wip" "$dir/docs/roadmaps/done" "$dir/docs/req" "$dir/docs/adr"
  cat >"$dir/trackfw.yaml" <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dir: docs/adr
EOF
}

# ---------------------------------------------------------------------------
# Scenario (a) — no match: blocks, exit 1, `git checkout -b` never runs.
# Vacuity guard: stdout must contain the orientation message ("has no matching
# roadmap"), proving the runtime actually reached the matching logic instead of
# failing earlier (e.g. on argument parsing).
# ---------------------------------------------------------------------------
BN_LABEL="no-match"
for runtime in go node py; do
  fixture="$WORK/a-$runtime"
  make_base "$fixture"
  cat >"$fixture/docs/roadmaps/wip/ROADMAP-2026-08-04-my-feature.md" <<'EOF'
---
status: wip
---
# Roadmap: my feature
EOF
  run_branch_new "$runtime" "$fixture" feat/no-match --dry-run
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -eq 0 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected non-zero exit (blocked), got 0"
    continue
  fi
  if ! grep -qF 'has no matching roadmap' "$BN_OUT_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stdout missing orientation message; stdout: $(cat "$BN_OUT_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — match + --dry-run: reports "would create", exit 0, never
# touches git.
# ---------------------------------------------------------------------------
BN_LABEL="match-dry-run"
for runtime in go node py; do
  fixture="$WORK/b-$runtime"
  make_base "$fixture"
  cat >"$fixture/docs/roadmaps/wip/ROADMAP-2026-08-04-my-feature.md" <<'EOF'
---
status: wip
---
# Roadmap: my feature
EOF
  run_branch_new "$runtime" "$fixture" feat/my-feature --dry-run
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -ne 0 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected exit 0, got $BN_EXIT; stderr: $(cat "$BN_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'would create branch' "$BN_OUT_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stdout missing 'would create branch'; stdout: $(cat "$BN_OUT_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — match, real git, target branch already exists: the only
# scenario that exercises the production defaultGitCheckout /
# _default_git_checkout wrapper end-to-end (unit tests inject a fake and never
# reach it — see vault/notes/branch-new-exit-code-leak-vs-propagation-2026-08-04.md).
# Asserts Git's own diagnostic and exit code (128) are propagated literally and
# identically by all three runtimes — no extra "exit status N" line, no
# hardcoded exit 1.
# ---------------------------------------------------------------------------
BN_LABEL="git-checkout-branch-exists"
for runtime in go node py; do
  fixture="$WORK/c-$runtime"
  make_base "$fixture"
  cat >"$fixture/docs/roadmaps/wip/ROADMAP-existing-branch-test.md" <<'EOF'
---
status: wip
---
# Roadmap: existing-branch-test
EOF
  (
    cd "$fixture"
    git init -q
    git config user.email test@example.com
    git config user.name "trackfw parity gate"
    echo x >file.txt
    git add file.txt
    git commit -qm init
    git checkout -q -b feat/existing-branch-test
    git checkout -q main 2>/dev/null || git checkout -q master 2>/dev/null || git branch -m main
  )
  run_branch_new "$runtime" "$fixture" feat/existing-branch-test
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -ne 128 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected exit 128 (git's own code for 'branch already exists'), got $BN_EXIT"
    continue
  fi
  if ! grep -qF "already exists" "$BN_ERR_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stderr missing git's 'already exists' diagnostic; stderr: $(cat "$BN_ERR_FILE")"
    continue
  fi
  # Confirms the "propagate literally" contract: no Go exec.ExitError string
  # artifact ("exit status 128") ever reaches the user (the exact regression
  # documented in the vault note above).
  if grep -qF "exit status" "$BN_ERR_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "stderr leaks a Go exec.ExitError artifact ('exit status ...') never produced by git itself: $(cat "$BN_ERR_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — chore type: housekeeping, creates the branch with wip/ EMPTY
# (no roadmap at all) — proves the branch_has_wip_roadmap gate does not apply to
# chore/docs. Uses --dry-run so no real git repo is needed here.
# ---------------------------------------------------------------------------
BN_LABEL="chore-skips-gate"
for runtime in go node py; do
  fixture="$WORK/d-$runtime"
  make_base "$fixture"
  # wip/ and done/ deliberately left empty.
  run_branch_new "$runtime" "$fixture" chore/release-x.y.z --dry-run
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -ne 0 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected exit 0 (no gate for chore), got $BN_EXIT; stderr: $(cat "$BN_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'would create branch' "$BN_OUT_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stdout missing 'would create branch'; stdout: $(cat "$BN_OUT_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — docs type: same as (d), housekeeping, wip/ EMPTY.
# ---------------------------------------------------------------------------
BN_LABEL="docs-skips-gate"
for runtime in go node py; do
  fixture="$WORK/e-$runtime"
  make_base "$fixture"
  run_branch_new "$runtime" "$fixture" docs/atualiza-readme --dry-run
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -ne 0 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected exit 0 (no gate for docs), got $BN_EXIT; stderr: $(cat "$BN_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'would create branch' "$BN_OUT_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stdout missing 'would create branch'; stdout: $(cat "$BN_OUT_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Scenario (f) — feat WITHOUT a matching roadmap still blocks (non-regression):
# proves loosening the gate for chore/docs did not loosen it for feat/fix/refactor.
# ---------------------------------------------------------------------------
BN_LABEL="feat-still-gated-non-regression"
for runtime in go node py; do
  fixture="$WORK/f-$runtime"
  make_base "$fixture"
  # wip/ and done/ deliberately left empty — no roadmap for this slug anywhere.
  run_branch_new "$runtime" "$fixture" feat/no-roadmap-for-this --dry-run
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -eq 0 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected non-zero exit (gate must still block feat without a roadmap), got 0"
    continue
  fi
  if ! grep -qF 'no roadmap is in wip/ nor done/' "$BN_OUT_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stdout missing orientation message; stdout: $(cat "$BN_OUT_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Scenario (g) — invalid type: vocabulary listed in the error message must be
# byte-identical across the three runtimes (feat, fix, refactor, chore, docs).
# ---------------------------------------------------------------------------
BN_LABEL="invalid-type-vocabulary"
for runtime in go node py; do
  fixture="$WORK/g-$runtime"
  make_base "$fixture"
  run_branch_new "$runtime" "$fixture" banana/whatever
  echo "$BN_EXIT" >"$WORK/$BN_LABEL.$runtime.exit"
  if [[ "$BN_EXIT" -eq 0 ]]; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "expected non-zero exit for invalid type, got 0"
    continue
  fi
  if ! grep -qF 'must be one of feat, fix, refactor, chore, docs' "$BN_ERR_FILE"; then
    fail "branch-new-parity/$BN_LABEL/$runtime" "vacuity guard: stderr missing full vocabulary; stderr: $(cat "$BN_ERR_FILE")"
    continue
  fi
done
assert_three_way "$BN_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-branch-new-parity.sh scenarios passed."
else
  echo "check-branch-new-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
