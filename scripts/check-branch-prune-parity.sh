#!/usr/bin/env bash
# check-branch-prune-parity.sh — proves `trackfw branch prune` behaves byte-for-byte identically
# in Go, Node.js, and Python, against a REAL git repository (not a mock — see Cenário 50 in
# scripts/check-gates-falsify.sh for precedent, and REQ-2026-08-18: "Mock de `git` provaria só que
# o mock concorda com o código").
#
# Fixture (shared across all three runtimes, rebuilt per runtime so --apply's real deletion in one
# runtime never contaminates another): local BARE repo as "origin" (offline, no network) + a
# clone. On the clone:
#   1. main: commit base.txt; push
#   2. feat/a: touches a.txt, squash-merged into main (no ancestry — the `git branch -d` false
#      negative)
#   3. feat/b: touches b.txt, branched AFTER feat/a's squash-merge landed, also squash-merged —
#      main advances further, so feat/a is now BEHIND origin/main but fully integrated (the
#      naive-bidirectional-diff false positive — AC2's discriminant)
#   4. push main; fetch — origin/main now reflects both squashes
#   5. feat/pending: touches c.txt, never merged — genuinely pending work
#
# Scenarios:
#   (a) no --apply (default): reports feat/a and feat/b as "delete", feat/pending as "keep", main
#       as skipped — but deletes NOTHING. Branch count before/after is identical.
#   (b) --apply: deletes feat/a and feat/b, keeps feat/pending and main. Branch count decreases by
#       exactly 2.
#   (c) --apply on the CURRENT branch (checked out on feat/a): feat/a is reported "keep" with the
#       current-branch reason, never deleted, even though it would otherwise qualify.
#   (d) no origin/main resolvable (fresh repo, no remote): refuses everything, exit 1, message
#       names origin/main, zero branches deleted.
#
# Follows the conventions of check-branch-new-parity.sh / check-ship-parity.sh: set -euo
# pipefail, mktemp -d fixtures with a cleanup trap, BASH_SOURCE-relative ROOT_DIR, ok()/fail()
# accumulating FAIL=1, byte-level diff -u between runtimes on both stdout and stderr.
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
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-branch-prune-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-branch-new-parity.sh.
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
  echo "check-branch-prune-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-branch-prune-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Fixture builder — real bare "origin" + clone, isolated $HOME/gitconfig (mirrors s50_commit_fixture
# in check-gates-falsify.sh). One clone per (scenario, runtime) so --apply's real `git branch -D`
# in one runtime run never affects another.
# ---------------------------------------------------------------------------
build_fixture() {
  local dest=$1
  local bare="$dest/origin.git"
  local clone="$dest/clone"
  local gitcfg="$dest/empty-gitconfig"
  mkdir -p "$dest"
  : >"$gitcfg"

  local -a env_args=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
  )

  git init -q --bare -b main "$bare" >"$dest/build.log" 2>&1
  env "${env_args[@]}" git clone -q "$bare" "$clone" >>"$dest/build.log" 2>&1
  (
    cd "$clone"
    env "${env_args[@]}" git config user.email "falsify@trackfw.test"
    env "${env_args[@]}" git config user.name "trackfw falsify"
    env "${env_args[@]}" git config commit.gpgsign false
    env "${env_args[@]}" git config core.hooksPath /dev/null

    echo base >base.txt
    env "${env_args[@]}" git add base.txt
    env "${env_args[@]}" git commit -q -m "base commit"
    env "${env_args[@]}" git push -q origin main

    env "${env_args[@]}" git checkout -q -b feat/a
    echo a >a.txt
    env "${env_args[@]}" git add a.txt
    env "${env_args[@]}" git commit -q -m "feat/a work"
    env "${env_args[@]}" git checkout -q main
    env "${env_args[@]}" git merge -q --squash feat/a
    env "${env_args[@]}" git commit -q -m "squash-merge feat/a"

    env "${env_args[@]}" git checkout -q -b feat/b
    echo b >b.txt
    env "${env_args[@]}" git add b.txt
    env "${env_args[@]}" git commit -q -m "feat/b work"
    env "${env_args[@]}" git checkout -q main
    env "${env_args[@]}" git merge -q --squash feat/b
    env "${env_args[@]}" git commit -q -m "squash-merge feat/b"

    env "${env_args[@]}" git push -q origin main
    env "${env_args[@]}" git fetch -q origin

    env "${env_args[@]}" git checkout -q -b feat/pending
    echo c >c.txt
    env "${env_args[@]}" git add c.txt
    env "${env_args[@]}" git commit -q -m "feat/pending work, never merged"
  ) >>"$dest/build.log" 2>&1
  echo "$clone"
}

# run_prune RUNTIME DIR ARGS... — runs `trackfw branch prune ARGS...` from DIR.
run_prune() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/$BP_LABEL.$runtime.out" err_file="$WORK/$BP_LABEL.$runtime.err"
  set +e
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" branch prune "$@")                              >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && node "$NODE_CLI" branch prune "$@")                       >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw branch prune "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_prune: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  BP_EXIT=$?
  set -e
  BP_OUT_FILE=$out_file
  BP_ERR_FILE=$err_file
}

# normalize_prune_output strips the branch-count preamble line ("evaluating N local branch(es)")
# from FILE in place — N legitimately differs across scenario (c)'s extra checked-out branch vs
# (a)/(b), and this script asserts branch SET membership directly instead. All other lines
# (per-branch decision rows, summary) are compared byte-for-byte.
normalize_prune_output() {
  local file=$1
  grep -v '^trackfw branch prune — evaluating' "$file" > "$file.norm" || true
  mv "$file.norm" "$file"
}

# assert_three_way LABEL — diffs go vs node and go vs py for both stdout and stderr.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "branch-prune-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "branch-prune-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "branch-prune-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "branch-prune-parity/$label"
  fi
}

local_branches() {
  local dir=$1
  (cd "$dir" && git branch --format='%(refname:short)' | sort)
}

# ---------------------------------------------------------------------------
# Scenario (a) — no --apply (default): reports feat/a and feat/b as deletable, feat/pending as
# kept, deletes NOTHING. Branch count before/after must be identical.
# ---------------------------------------------------------------------------
BP_LABEL="dry-run-default"
for runtime in go node py; do
  fixture=$(build_fixture "$WORK/a-$runtime")
  before=$(local_branches "$fixture")
  run_prune "$runtime" "$fixture"
  echo "$BP_EXIT" >"$WORK/$BP_LABEL.$runtime.exit"
  after=$(local_branches "$fixture")
  if [[ "$before" != "$after" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "dry-run must delete nothing; before=[$before] after=[$after]"
    continue
  fi
  if ! grep -qF 'would delete' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "vacuity guard: stdout missing 'would delete'; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
  if ! grep -qF 'feat/a' "$BP_OUT_FILE" || ! grep -qF 'feat/b' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "expected feat/a and feat/b named as delete candidates; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
  normalize_prune_output "$BP_OUT_FILE"
done
assert_three_way "$BP_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — --apply: deletes feat/a and feat/b (AC1 + AC2), keeps feat/pending (AC3) and main.
# Branch count decreases by exactly 2.
# ---------------------------------------------------------------------------
BP_LABEL="apply-deletes-integrated"
for runtime in go node py; do
  fixture=$(build_fixture "$WORK/b-$runtime")
  before_count=$(local_branches "$fixture" | wc -l | tr -d ' ')
  run_prune "$runtime" "$fixture" --apply
  echo "$BP_EXIT" >"$WORK/$BP_LABEL.$runtime.exit"
  after=$(local_branches "$fixture")
  after_count=$(echo "$after" | wc -l | tr -d ' ')
  if [[ "$after_count" -ne $((before_count - 2)) ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "expected branch count to drop by exactly 2, before=$before_count after=$after_count (remaining: $after)"
    continue
  fi
  if echo "$after" | grep -qxF 'feat/a' || echo "$after" | grep -qxF 'feat/b'; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "feat/a and feat/b must have been deleted; remaining: $after"
    continue
  fi
  if ! echo "$after" | grep -qxF 'feat/pending'; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "feat/pending must never be deleted (AC3); remaining: $after"
    continue
  fi
  if ! echo "$after" | grep -qxF 'main'; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "main must never be deleted; remaining: $after"
    continue
  fi
  if ! grep -qF 'deleted 2 branch(es)' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "vacuity guard: stdout missing 'deleted 2 branch(es)'; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
  normalize_prune_output "$BP_OUT_FILE"
done
assert_three_way "$BP_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — AC4: --apply while feat/a is the CURRENT branch never deletes it, even though it
# would otherwise qualify (content_identical).
# ---------------------------------------------------------------------------
BP_LABEL="apply-never-deletes-current-branch"
for runtime in go node py; do
  fixture=$(build_fixture "$WORK/c-$runtime")
  (cd "$fixture" && git checkout -q feat/a)
  run_prune "$runtime" "$fixture" --apply
  echo "$BP_EXIT" >"$WORK/$BP_LABEL.$runtime.exit"
  after=$(local_branches "$fixture")
  if ! echo "$after" | grep -qxF 'feat/a'; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "feat/a is the current branch and must never be deleted; remaining: $after"
    continue
  fi
  if ! grep -qF 'current branch' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "vacuity guard: stdout missing 'current branch' reason for feat/a; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
  normalize_prune_output "$BP_OUT_FILE"
done
assert_three_way "$BP_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — AC6: no origin/main resolvable (fresh repo, never fetched from a remote) refuses
# everything, exit 1, message names origin/main, zero branches deleted.
# ---------------------------------------------------------------------------
BP_LABEL="offline-refuses"
for runtime in go node py; do
  fixture="$WORK/d-$runtime"
  mkdir -p "$fixture"
  (
    cd "$fixture"
    git init -q -b main
    git config user.email "falsify@trackfw.test"
    git config user.name "trackfw falsify"
    git config commit.gpgsign false
    git config core.hooksPath /dev/null
    echo x >f.txt
    git add f.txt
    git commit -q -m "solo commit, no remote"
  )
  before=$(local_branches "$fixture")
  run_prune "$runtime" "$fixture" --apply
  echo "$BP_EXIT" >"$WORK/$BP_LABEL.$runtime.exit"
  after=$(local_branches "$fixture")
  if [[ "$BP_EXIT" -eq 0 ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "expected non-zero exit when origin/main is unresolvable, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "offline must delete nothing; before=[$before] after=[$after]"
    continue
  fi
  if ! grep -qF 'origin/main' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "vacuity guard: stdout missing 'origin/main'; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
done
assert_three_way "$BP_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — ML-1B/ML-1C, side by side in the same fixture (per KG's "Auditoria do ML-1B"):
# review_doc_config requires diverg to be a PROPER subset of touched — genuine partial
# integration, not "every diverging file happens to be doc/config".
#   - feat/docs-review: touches app.txt (code) AND README.md (doc). Only app.txt's content is
#     squash-merged into main; README.md never lands there — genuine housekeeping residue.
#     Must be flagged "review", never "delete" and never auto-deleted by --apply.
#   - feat/doc-real: touches ONLY NEWDOC.md, never merged anywhere (diverg == touched). Must be
#     "keep"/pending work, NEVER "review" — calling brand-new, unmerged documentation "probable
#     housekeeping" is the exact wrong-advice bug ML-1C fixes.
# ---------------------------------------------------------------------------
BP_LABEL="review-doc-config-only"
for runtime in go node py; do
  dest="$WORK/e-$runtime"
  bare="$dest/origin.git"
  clone="$dest/clone"
  gitcfg="$dest/empty-gitconfig"
  mkdir -p "$dest"
  : >"$gitcfg"
  env_args=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
  )
  git init -q --bare -b main "$bare" >"$dest/build.log" 2>&1
  env "${env_args[@]}" git clone -q "$bare" "$clone" >>"$dest/build.log" 2>&1
  (
    cd "$clone"
    env "${env_args[@]}" git config user.email "falsify@trackfw.test"
    env "${env_args[@]}" git config user.name "trackfw falsify"
    env "${env_args[@]}" git config commit.gpgsign false
    env "${env_args[@]}" git config core.hooksPath /dev/null
    echo "# base" >README.md
    env "${env_args[@]}" git add README.md
    env "${env_args[@]}" git commit -q -m "base commit"
    env "${env_args[@]}" git push -q origin main

    # feat/docs-review: code + doc, only the code lands in main (partial integration residue).
    env "${env_args[@]}" git checkout -q -b feat/docs-review
    echo "app work" >app.txt
    echo "# updated docs only, never merged" >README.md
    env "${env_args[@]}" git add app.txt README.md
    env "${env_args[@]}" git commit -q -m "feat/docs-review work: code + doc"
    env "${env_args[@]}" git checkout -q main
    echo "app work" >app.txt
    env "${env_args[@]}" git add app.txt
    env "${env_args[@]}" git commit -q -m "squash-merge feat/docs-review (code only, doc left out)"
    env "${env_args[@]}" git push -q origin main

    # feat/doc-real: brand-new doc, never merged anywhere — ML-1C discriminant.
    env "${env_args[@]}" git checkout -q -b feat/doc-real
    echo "# never merged" >NEWDOC.md
    env "${env_args[@]}" git add NEWDOC.md
    env "${env_args[@]}" git commit -q -m "feat/doc-real: never-merged documentation"
    env "${env_args[@]}" git checkout -q main
    env "${env_args[@]}" git fetch -q origin
  ) >>"$dest/build.log" 2>&1
  before=$(local_branches "$clone")
  run_prune "$runtime" "$clone"
  echo "$BP_EXIT" >"$WORK/$BP_LABEL.$runtime.exit"
  after=$(local_branches "$clone")
  if [[ "$before" != "$after" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "dry-run must delete nothing; before=[$before] after=[$after]"
    continue
  fi
  if ! grep -qF 'need manual review' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "vacuity guard: stdout missing the review summary; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
  review_line=$(grep 'feat/docs-review' "$BP_OUT_FILE" | grep -v 'need manual review' || true)
  review_action=$(echo "$review_line" | awk '{print $2}')
  if [[ "$review_action" != "review" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "feat/docs-review must be reported action=review, got action='$review_action' line: $review_line"
    continue
  fi
  doc_real_line=$(grep 'feat/doc-real' "$BP_OUT_FILE" | grep -v 'need manual review' || true)
  doc_real_action=$(echo "$doc_real_line" | awk '{print $2}')
  if [[ "$doc_real_action" != "keep" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "feat/doc-real (never merged, doc-only) must be reported action=keep, got action='$doc_real_action' line: $doc_real_line"
    continue
  fi
  if echo "$doc_real_line" | grep -qF 'housekeeping'; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "feat/doc-real must NOT be advised as housekeeping — it is real, unmerged work; line: $doc_real_line"
    continue
  fi
  normalize_prune_output "$BP_OUT_FILE"
done
assert_three_way "$BP_LABEL"

# ---------------------------------------------------------------------------
# Scenario (f) — ML-1B: `git fetch origin --prune` failure (broken remote URL) is non-blocking and
# warned, and a stale origin/main only ever makes the result MORE conservative — a branch truly
# integrated upstream but invisible to the stale local ref is reported "keep", never "delete".
# ---------------------------------------------------------------------------
BP_LABEL="stale-origin-main-conservative"
for runtime in go node py; do
  dest="$WORK/f-$runtime"
  bare="$dest/origin.git"
  clone="$dest/clone"
  other="$dest/other-clone"
  gitcfg="$dest/empty-gitconfig"
  mkdir -p "$dest"
  : >"$gitcfg"
  env_args=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
  )
  git init -q --bare -b main "$bare" >"$dest/build.log" 2>&1
  env "${env_args[@]}" git clone -q "$bare" "$clone" >>"$dest/build.log" 2>&1
  (
    cd "$clone"
    env "${env_args[@]}" git config user.email "falsify@trackfw.test"
    env "${env_args[@]}" git config user.name "trackfw falsify"
    env "${env_args[@]}" git config commit.gpgsign false
    env "${env_args[@]}" git config core.hooksPath /dev/null
    echo base >base.txt
    env "${env_args[@]}" git add base.txt
    env "${env_args[@]}" git commit -q -m "base commit"
    env "${env_args[@]}" git push -q origin main
    env "${env_args[@]}" git checkout -q -b feat/mine
    echo "mine v1" >mine.txt
    env "${env_args[@]}" git add mine.txt
    env "${env_args[@]}" git commit -q -m "feat/mine work"
    env "${env_args[@]}" git checkout -q main
  ) >>"$dest/build.log" 2>&1
  # Someone else lands the exact same content upstream via an independent clone — our clone above
  # never learns about it (never fetches again from this point on).
  env "${env_args[@]}" git clone -q "$bare" "$other" >>"$dest/build.log" 2>&1
  (
    cd "$other"
    env "${env_args[@]}" git config user.email "falsify@trackfw.test"
    env "${env_args[@]}" git config user.name "trackfw falsify"
    env "${env_args[@]}" git config commit.gpgsign false
    echo "mine v1" >mine.txt
    env "${env_args[@]}" git add mine.txt
    env "${env_args[@]}" git commit -q -m "someone else lands the same content upstream"
    env "${env_args[@]}" git push -q origin main
  ) >>"$dest/build.log" 2>&1
  # Break the remote URL so `git fetch origin --prune` (run internally by branch prune) fails
  # deterministically — our clone's origin/main stays frozen at "base commit".
  (cd "$clone" && env "${env_args[@]}" git remote set-url origin "$dest/does-not-exist.git")

  before=$(local_branches "$clone")
  run_prune "$runtime" "$clone"
  echo "$BP_EXIT" >"$WORK/$BP_LABEL.$runtime.exit"
  after=$(local_branches "$clone")
  if [[ "$before" != "$after" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "dry-run must delete nothing; before=[$before] after=[$after]"
    continue
  fi
  if ! grep -qi 'warning' "$BP_OUT_FILE"; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "vacuity guard: stdout missing the fetch-failure warning; stdout: $(cat "$BP_OUT_FILE")"
    continue
  fi
  mine_line=$(grep 'feat/mine' "$BP_OUT_FILE" || true)
  mine_action=$(echo "$mine_line" | awk '{print $2}')
  if [[ "$mine_action" != "keep" ]]; then
    fail "branch-prune-parity/$BP_LABEL/$runtime" "stale origin/main must report feat/mine as keep (conservative), got action='$mine_action' line: $mine_line"
    continue
  fi
  normalize_prune_output "$BP_OUT_FILE"
done
assert_three_way "$BP_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-branch-prune-parity.sh scenarios passed."
else
  echo "check-branch-prune-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
