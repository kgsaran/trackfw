#!/usr/bin/env bash
# check-ship-parity.sh — proves `trackfw ship` behaves byte-for-byte identically in Go, Node.js,
# and Python:
#   - for the chore/docs branch-type governance exemption introduced by
#     docs/roadmaps/wip/ROADMAP-2026-08-16-trackfw-ship-aceita-branches-chore-e-docs-sem-gate-de-roadmap.md
#     (the `ship` sibling of `trackfw branch new`'s #177 fix), plus a non-regression assertion
#     that feat/fix/refactor remain hard-gated;
#   - for the checkShipGovernance wording and error-stream/prefix parity fixed by ML-1B of
#     docs/roadmaps/wip/ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md
#     (see vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md for the
#     root cause this ML eliminated).
#
# No check-ship-parity.sh existed before the chore/docs ML — `trackfw ship` had no dedicated
# behavioral parity script (only the command-surface floor check in check-cli-parity.sh). This
# script fills that gap, following the exact conventions of scripts/check-branch-new-parity.sh
# and scripts/check-commit-parity.sh: set -euo pipefail, mktemp -d fixtures with a cleanup trap,
# BASH_SOURCE-relative ROOT_DIR, ok()/fail() accumulating FAIL=1.
#
# Every scenario below uses a byte-level diff -u of BOTH stdout and stderr (assert_three_way) —
# scenarios (c) and (d) used to fall back to a normalizing content-only check because the
# checkShipGovernance wording and the error stream/prefix genuinely diverged across runtimes at
# the time this script was first written (Go: "wip/ nor done/", stderr, "Error: "; Node/Python:
# "wip/" only, stdout, "error: "). ML-1B fixed the divergence at the source — Node/Python's
# checkShipGovernance now delegates to the same shared validator functions Go's
# CheckShipGovernance calls (validateBranchHasWIPRoadmap + validateWIPHasREQ), and all three
# runtimes now write every abort path's final summary to stderr with an "Error: " prefix — so the
# normalizing helpers (assert_exit_equal, assert_message_byte_identical) that used to carry this
# script's documentation of the accepted gap are gone; a real regression in either the wording or
# the stream/prefix now fails this gate directly, byte-for-byte.
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
#   (c) non-regression: feat/<slug>, wip/ EMPTY, non-doc file staged → exit 1, byte-identical
#       stdout+stderr ("Error: ... no roadmap is in wip/ nor done/ ...", stderr summary line) —
#       proves loosening the gate for chore/docs did not loosen it for feat/fix/refactor, and
#       proves checkShipGovernance's wording/stream/prefix stayed in lockstep across runtimes.
#   (d) branch outside the ship vocabulary (feat|fix|refactor|chore|docs), non-doc file staged →
#       exit 1, byte-identical stdout+stderr ("Error: ... does not match the required
#       pattern...", stderr).
#   (e) squash-merge warning (AC8 of
#       docs/roadmaps/wip/ROADMAP-2026-08-18-branch-prune-com-dry-run-por-padrao-e-heuristica-de-arquivos-tocados.md,
#       ML-2B): a REAL git remote (bare origin + clone) with one branch that was squash-merged
#       into main and is now merely stale (main advanced further afterwards — the #181/#182
#       incident) and one branch that is genuinely never merged. Runs `ship` WITHOUT --dry-run
#       (dry-run skips Step 3's fetch and detectPendingSquashMerges entirely — see ship.go:212-220
#       — so it cannot exercise this path) so the real `git fetch origin --prune` +
#       detectPendingSquashMerges call the same evaluateBranchIntegration heuristic
#       `check-branch-prune-parity.sh` already proves per-runtime. Proves, byte-for-byte across
#       all three runtimes: the stale-but-integrated branch gets NO warning, the genuinely
#       pending branch gets the warning, and the two are never conflated.
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
# Full-stream byte diff: checkShipGovernance now delegates to the same shared validator
# functions Go's CheckShipGovernance calls (ML-1B), and the stderr summary line + "Error: "
# prefix are shared across all three runtimes too — proves the wording ("nor done/") and the
# stream/prefix stayed in lockstep, not just the exit code and the two markers checked above.
assert_three_way "$SH_LABEL"

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
# Full-stream byte diff: all three runtimes now write this error to stderr with the "Error: "
# prefix (ML-1B) — a real drift anywhere in the message (e.g. the vocabulary list, or the
# "git checkout -b feat/<slug>" hint) or in which stream it lands on still fails this gate.
assert_three_way "$SH_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — squash-merge warning parity (AC8, ML-2B).
#
# make_squash_fixture builds a REAL bare "origin" + a single working clone (mirrors
# TestDetectPendingSquashMerges_RealGitRepo_StaleIntegratedVsGenuinelyPending in
# internal/commands/ship_test.go, the Go-only proof this scenario now extends to all 3
# runtimes):
#   - feat/a: pushed to origin, squash-merged into main, then main advances further
#     (an unrelated follow-up commit) — feat/a is content-integrated but ancestry-stale.
#   - feat/pending: pushed to origin, genuinely never merged anywhere.
#   - a NEW branch (chore/<slug>) is checked out off the advanced main, with one non-doc file
#     staged — this is the branch `ship` actually runs from; it is never itself a candidate
#     (detectPendingSquashMerges always skips currentBranch).
# ---------------------------------------------------------------------------
make_squash_fixture() {
  local dir=$1
  local bare="$dir-origin.git"
  mkdir -p "$bare"
  (cd "$bare" && git init -q --bare -b main .)

  git clone -q "$bare" "$dir"
  (
    cd "$dir"
    git config user.email test@example.com
    git config user.name "trackfw parity gate"

    echo base >base.txt
    git add base.txt
    git commit -qm "base commit"
    git push -q origin main

    # feat/a — pushed, then squash-merged into main. Ancestry never records the merge (the
    # git branch -d false negative), so it stays in `branch -r --no-merged origin/main`.
    git checkout -q -b feat/a
    echo a >a.txt
    git add a.txt
    git commit -qm "feat/a work"
    git push -q origin feat/a
    git checkout -q main
    git merge -q --squash feat/a
    git commit -qm "squash-merge feat/a (PR #181)"

    # main advances further (PR #182) — this is what makes the naive bidirectional diff
    # non-empty for the already-integrated feat/a, and what the touched-files heuristic must
    # see through.
    echo b >b.txt
    git add b.txt
    git commit -qm "unrelated follow-up (PR #182)"
    git push -q origin main

    # feat/pending — pushed, genuinely never merged anywhere.
    git checkout -q -b feat/pending
    echo c >c.txt
    git add c.txt
    git commit -qm "feat/pending work, never merged"
    git push -q origin feat/pending

    # The branch `ship` actually runs from — off the advanced main, with one staged non-doc
    # file. chore/ skips Step 2's governance gate so no roadmap/REQ fixture is needed.
    git checkout -q main
    git checkout -q -b "chore/ship-parity-squash-check"
    echo note >note.txt
    git add note.txt
  )
}

SH_LABEL="squash-merge-warning"
for runtime in go node py; do
  fixture="$WORK/e-$runtime"
  make_squash_fixture "$fixture"
  run_ship "$runtime" "$fixture" -m "chore: ship parity squash-merge check" --no-pr
  echo "$SH_EXIT" >"$WORK/$SH_LABEL.$runtime.exit"
  if [[ "$SH_EXIT" -ne 0 ]]; then
    fail "ship-parity/$SH_LABEL/$runtime" "expected exit 0, got $SH_EXIT; stdout: $(cat "$SH_OUT_FILE"); stderr: $(cat "$SH_ERR_FILE")"
    continue
  fi
  # Vacuity guard: the warning must actually fire for the genuinely pending branch — otherwise
  # an empty-vs-empty diff below would "pass" without proving anything (risk 1 of the ML).
  if ! grep -qF 'Warning: branch "feat/pending" appears to have unmerged changes vs origin/main.' "$SH_OUT_FILE"; then
    fail "ship-parity/$SH_LABEL/$runtime" "vacuity guard: stdout missing the pending-branch warning; stdout: $(cat "$SH_OUT_FILE")"
    continue
  fi
  # The stale-but-integrated branch must never be warned about.
  if grep -qF 'feat/a' "$SH_OUT_FILE"; then
    fail "ship-parity/$SH_LABEL/$runtime" "stale-but-integrated feat/a must never appear in a warning; stdout: $(cat "$SH_OUT_FILE")"
    continue
  fi
done
assert_three_way "$SH_LABEL"

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
