#!/usr/bin/env bash
# check-push-parity.sh — proves `trackfw push` behaves byte-for-byte identically in Go, Node.js,
# and Python across the five behavioral paths the command implements:
#
#   (a) feat/<slug> with governance satisfied, no upstream configured yet — exit 0, stdout
#       contains "[dry-run] git push -u origin" and never contains "pull request" (Direction B
#       vacuity guard proving push never opens a PR/MR).
#   (b) feat/<slug> with governance satisfied, upstream already configured — exit 0, stdout
#       contains "[dry-run] git push origin" WITHOUT the "-u" flag.
#   (c) main branch — blocked unconditionally at Step 1, byte-identical error across runtimes.
#   (d) feat/<slug> WITHOUT governance artifacts — blocked at Step 2, byte-identical
#       "Governance check failed" output across runtimes.
#   (e) chore/<slug> WITHOUT governance artifacts — governance skipped, exit 0, stdout contains
#       "Governance: skipped (chore/docs branch)".
#
# Every scenario uses a byte-level diff -u of BOTH stdout and stderr (assert_three_way) plus a
# per-runtime vacuity guard on the discriminating marker, following the exact conventions of
# scripts/check-ship-parity.sh and scripts/check-commit-parity.sh.
#
# Scenario (b) builds a real bare-origin + clone so git rev-parse @{u} actually resolves — the
# same technique check-ship-parity.sh scenario (e) already uses.  Setting branch.*.remote/merge
# config without a real remote ref makes rev-parse exit 128 ("fatal: no upstream configured"),
# which would fall through to the "no upstream" arm and render scenario (b) vacuous.
#
# Every scenario stages a NON-doc file so the allDocOnly exception (present in commit, NOT in
# push — push has no doc-only exception) never interferes.  Push governance is either hard-gated
# (feat/fix/refactor) or fully skipped (chore/docs) with nothing in between.
#
# Direction B vacuity target: scenario (a) checks that "pull request" is absent from stdout.
# scripts/check-gates-falsify.sh Cenário 162 sabotages Go push.go to emit an Opening-PR line and
# proves this guard AND assert_three_way's go-vs-node diff jointly catch the regression.
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
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-push-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-ship-parity.sh / check-commit-parity.sh.
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
  echo "check-push-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-push-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# run_push RUNTIME DIR ARGS...
# Runs `trackfw push ARGS...` from DIR. Sets PU_EXIT and writes stdout/stderr to
# $WORK/<label>.<runtime>.out / .err (label passed by caller via PU_LABEL).
run_push() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/$PU_LABEL.$runtime.out" err_file="$WORK/$PU_LABEL.$runtime.err"
  set +e
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" push "$@")                               >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && node "$NODE_CLI" push "$@")                        >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw push "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_push: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  PU_EXIT=$?
  set -e
  PU_OUT_FILE=$out_file
  PU_ERR_FILE=$err_file
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
      fail "push-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "push-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "push-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "push-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Shared minimal fixture: real git repo on a given branch with an empty governance
# tree and one staged NON-doc file.  Used for scenarios where governance is either
# not required (chore/docs) or expected to fail (feat/ without roadmap).
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

# make_governed_fixture DIR BRANCH SLUG
# Like make_fixture but also writes a roadmap in wip/ whose name contains SLUG and a REQ file,
# satisfying the governance gate for feat/fix/refactor branches.
make_governed_fixture() {
  local dir=$1 branch=$2 slug=$3
  make_fixture "$dir" "$branch"
  # Governance files only need to be on disk — push reads them from the filesystem directly.
  # The roadmap must contain a "REQ:" marker with a non-empty value so validateWIPHasREQ()
  # does not report a violation.  contentHasMarker checks for the literal string "REQ:".
  printf 'REQ: REQ-push-parity-test.md\n\n# Roadmap: push parity test\n' \
    >"$dir/docs/roadmaps/wip/ROADMAP-$slug.md"
  echo "# REQ: push parity test" >"$dir/docs/req/REQ-push-parity-test.md"
}

# make_governed_upstream_fixture DIR BRANCH SLUG
# Like make_governed_fixture but builds a real bare-origin + clone so that
# `git rev-parse --abbrev-ref --symbolic-full-name @{u}` succeeds.  buildPushArgs
# (shared across all 3 CLIs) uses this command to detect whether -u is needed;
# configuring branch.*.remote/merge without a real remote ref makes rev-parse exit 128
# and fall through to the "no upstream" arm, rendering the scenario vacuous.
make_governed_upstream_fixture() {
  local dir=$1 branch=$2 slug=$3
  local bare="$dir-origin.git"
  mkdir -p "$bare"
  (cd "$bare" && git init -q --bare -b main .)

  git clone -q "$bare" "$dir"
  (
    cd "$dir"
    git config user.email test@example.com
    git config user.name "trackfw parity gate"

    # Initial commit on main and push to establish the remote.
    echo init >README.md
    git add README.md
    git commit -qm init
    git push -q origin main

    # Create the feature branch and push it with -u to establish upstream tracking.
    # After this, `git rev-parse @{u}` succeeds, making buildPushArgs use
    # `push origin <branch>` (no -u flag).
    git checkout -q -b "$branch"
    echo "placeholder" >placeholder.txt
    git add placeholder.txt
    git commit -qm "placeholder commit to allow push"
    git push -q -u origin "$branch"

    # Now set up governance tree and the non-doc staged file.
    mkdir -p docs/roadmaps/wip docs/roadmaps/done docs/req docs/adr
    cat >trackfw.yaml <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dir: docs/adr
EOF
    printf 'REQ: REQ-push-parity-test.md\n\n# Roadmap: push parity test\n' \
      >"docs/roadmaps/wip/ROADMAP-$slug.md"
    echo "# REQ: push parity test" >"docs/req/REQ-push-parity-test.md"
    echo "non-doc content" >src.txt
    git add src.txt
  )
}

# ---------------------------------------------------------------------------
# Scenario (a) — feat/<slug> with governance satisfied, NO upstream configured.
# Expects exit 0, stdout contains "[dry-run] git push -u origin" (proving buildPushArgs
# detected no upstream and added -u).
# Direction B vacuity guard: "pull request" must be ABSENT from stdout — proves push never
# opens a PR/MR.  scripts/check-gates-falsify.sh Cenário 162 sabotages Go to emit an
# Opening-PR line and proves this guard AND assert_three_way's go-vs-node diff catch it.
# ---------------------------------------------------------------------------
PU_LABEL="feat-governance-ok-no-upstream"
for runtime in go node py; do
  fixture="$WORK/a-$runtime"
  make_governed_fixture "$fixture" "feat/push-parity-no-upstream" "push-parity-no-upstream"
  run_push "$runtime" "$fixture" --dry-run
  echo "$PU_EXIT" >"$WORK/$PU_LABEL.$runtime.exit"
  if [[ "$PU_EXIT" -ne 0 ]]; then
    fail "push-parity/$PU_LABEL/$runtime" "expected exit 0 (governance satisfied, no upstream), got $PU_EXIT; stderr: $(cat "$PU_ERR_FILE")"
    continue
  fi
  # Vacuity guard (upstream detection): -u must be present in the push command.
  if ! grep -qF '[dry-run] git push -u origin' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "vacuity guard: stdout missing '[dry-run] git push -u origin'; stdout: $(cat "$PU_OUT_FILE")"
    continue
  fi
  # Direction B vacuity guard: push must NEVER emit "pull request" text.
  if grep -qiF 'pull request' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "push must not open a PR; stdout contained 'pull request': $(cat "$PU_OUT_FILE")"
    continue
  fi
done
assert_three_way "$PU_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — feat/<slug> with governance satisfied, upstream ALREADY configured.
# Uses a real bare-origin + clone so git rev-parse @{u} succeeds.
# Expects exit 0, stdout contains "[dry-run] git push origin" WITHOUT "-u".
# ---------------------------------------------------------------------------
PU_LABEL="feat-governance-ok-with-upstream"
for runtime in go node py; do
  fixture="$WORK/b-$runtime"
  make_governed_upstream_fixture "$fixture" "feat/push-parity-with-upstream" "push-parity-with-upstream"
  run_push "$runtime" "$fixture" --dry-run
  echo "$PU_EXIT" >"$WORK/$PU_LABEL.$runtime.exit"
  if [[ "$PU_EXIT" -ne 0 ]]; then
    fail "push-parity/$PU_LABEL/$runtime" "expected exit 0 (governance satisfied, upstream configured), got $PU_EXIT; stderr: $(cat "$PU_ERR_FILE")"
    continue
  fi
  # Vacuity guard (upstream detection): -u must be ABSENT in the push command.
  # The push line must start "[dry-run] git push origin" (not "[dry-run] git push -u origin").
  if ! grep -qF '[dry-run] git push origin' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "vacuity guard: stdout missing '[dry-run] git push origin'; stdout: $(cat "$PU_OUT_FILE")"
    continue
  fi
  if grep -qF '[dry-run] git push -u ' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "vacuity guard: -u must be absent when upstream is configured; stdout: $(cat "$PU_OUT_FILE")"
    continue
  fi
done
assert_three_way "$PU_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — main branch: blocked unconditionally at Step 1.
# Byte-identical error message across runtimes.
# ---------------------------------------------------------------------------
PU_LABEL="main-blocked"
for runtime in go node py; do
  fixture="$WORK/c-$runtime"
  # Stay on the default branch (main) — do not call git checkout -b.
  mkdir -p "$fixture/docs/roadmaps/wip" "$fixture/docs/roadmaps/done" "$fixture/docs/req" "$fixture/docs/adr"
  cat >"$fixture/trackfw.yaml" <<'YAML'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dir: docs/adr
YAML
  (
    cd "$fixture"
    git init -q -b main
    git config user.email test@example.com
    git config user.name "trackfw parity gate"
    echo init >README.md
    git add README.md
    git commit -qm init
    echo "non-doc content" >src.txt
    git add src.txt
  )
  run_push "$runtime" "$fixture" --dry-run
  echo "$PU_EXIT" >"$WORK/$PU_LABEL.$runtime.exit"
  if [[ "$PU_EXIT" -eq 0 ]]; then
    fail "push-parity/$PU_LABEL/$runtime" "expected non-zero exit for push on main, got 0"
    continue
  fi
  # Vacuity guard: the exact error phrase must appear.
  if ! grep -qF 'trackfw push cannot run on' "$PU_OUT_FILE" "$PU_ERR_FILE" 2>/dev/null; then
    fail "push-parity/$PU_LABEL/$runtime" "vacuity guard: output missing 'trackfw push cannot run on'; stdout: $(cat "$PU_OUT_FILE"); stderr: $(cat "$PU_ERR_FILE")"
    continue
  fi
done
assert_three_way "$PU_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — feat/<slug> WITHOUT governance artifacts: blocked at Step 2.
# Governance check must fail; "Governance: skipped" must never appear.
# ---------------------------------------------------------------------------
PU_LABEL="feat-governance-blocked"
for runtime in go node py; do
  fixture="$WORK/d-$runtime"
  make_fixture "$fixture" "feat/no-roadmap-for-push-parity"
  run_push "$runtime" "$fixture" --dry-run
  echo "$PU_EXIT" >"$WORK/$PU_LABEL.$runtime.exit"
  if [[ "$PU_EXIT" -eq 0 ]]; then
    fail "push-parity/$PU_LABEL/$runtime" "expected non-zero exit (governance gate must block feat/ without roadmap), got 0"
    continue
  fi
  if ! grep -qF 'Governance check failed' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "vacuity guard: stdout missing 'Governance check failed'; stdout: $(cat "$PU_OUT_FILE")"
    continue
  fi
  if grep -qF 'Governance: skipped' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "feat branch must never print a governance-skipped message; stdout: $(cat "$PU_OUT_FILE")"
    continue
  fi
done
assert_three_way "$PU_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — chore/<slug> WITHOUT governance artifacts: governance skipped.
# Proves push exempts chore/docs branches from the governance gate, mirroring
# `trackfw commit` and `trackfw ship`.
# ---------------------------------------------------------------------------
PU_LABEL="chore-docs-exempt"
for runtime in go node py; do
  fixture="$WORK/e-$runtime"
  make_fixture "$fixture" "chore/push-parity-exempt"
  run_push "$runtime" "$fixture" --dry-run
  echo "$PU_EXIT" >"$WORK/$PU_LABEL.$runtime.exit"
  if [[ "$PU_EXIT" -ne 0 ]]; then
    fail "push-parity/$PU_LABEL/$runtime" "expected exit 0 (chore/ exempt from governance), got $PU_EXIT; stderr: $(cat "$PU_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'Governance: skipped (chore/docs branch)' "$PU_OUT_FILE"; then
    fail "push-parity/$PU_LABEL/$runtime" "vacuity guard: stdout missing branch-type skip marker; stdout: $(cat "$PU_OUT_FILE")"
    continue
  fi
done
assert_three_way "$PU_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-push-parity.sh scenarios passed."
else
  echo "check-push-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
