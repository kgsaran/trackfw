#!/usr/bin/env bash
# check-push-force-parity.sh — proves `trackfw push --force-with-lease` behaves byte-for-byte
# identically in Go, Node.js, and Python (ML-4B, ROADMAP-2026-08-22-trackfw-push-comando-proprio-
# para-empurrar-commits-ja-criados.md).
#
# The four paths, exactly as ML-4A's audit verified against a real fixture:
#
#   sem CLI de forge        -> RECUSA  "requires a forge CLI (gh, glab, or az) to confirm ..."
#   forge, zero PR          -> RECUSA  "has no open pull/merge request. Open the PR/MR first"
#   forge, nao verificavel  -> RECUSA  "could not verify ... Refusing rather than risking ..."
#   forge, PR aberto        -> EMPURRA, and the remote history is genuinely rewritten
#
# These are THREE distinct refusal classes, never two — conflating "no PR" with "cannot verify"
# would make a `gh` auth failure look like "no PR exists", nudging the caller to open a PR that
# already exists. Every scenario below diffs the THREE real outputs (stdout, stderr, exit code)
# byte-for-byte across the 3 CLIs.
#
# Key difference vs check-ship-force-parity.sh: `push` has NO --forge flag and never opens a PR
# (no --no-pr flag either). Forge is resolved via the `forge: github` key in `trackfw.yaml`
# (written by make_fixture into the clone root). This mirrors the production path exactly —
# config.Load().Forge is the ConfigForge injected into pushDeps in push.go's RunE. No
# TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 here: push's forge lookup is ALWAYS live.
#
# Fixture conventions, per KG's constraints (all already cost a cycle once in this series):
#   1. A REAL bare git remote, local, offline — never a mocked `git`. See
#      check-branch-prune-parity.sh / check-doctor-parity.sh for precedent.
#   2. Fixture is rebuilt per (scenario, runtime) — the success path genuinely rewrites remote
#      history, so a shared fixture would let one runtime's push contaminate the next.
#   3. $HOME redirected, GIT_CONFIG_GLOBAL/GIT_CONFIG_SYSTEM isolated — never touches the real
#      user gitconfig or credential helpers.
#   4. `gh` is stubbed via a directory prepended to a PATH built from scratch (never the
#      inherited PATH) — this machine has a real `gh`/`az` installed, and PATH must guarantee
#      the "no forge CLI" scenario truly sees none, not "whichever gh happens to be first".
#      A stub returning invalid JSON or a non-zero exit lands in the "cannot verify" class, not
#      "no PR" — conflating them was a real mistake made once while building this gate.
#   5. All scenarios use chore/ branches — push already exempts chore/docs from the
#      REQ+roadmap gate; this avoids having to plant full governance artifacts in every fixture
#      while still exercising the real binary path (governance skip is logged to stdout as
#      "Governance: skipped (chore/docs branch)").
#   6. Success is proved by the remote SHA changing (git --git-dir=<bare> rev-parse <branch>
#      before/after), never by the printed message alone.
#
# Fifth scenario — the semantic discriminant, stronger than inspecting the push argv string:
# after the "PR open" fixture is built, a SECOND clone pushes one more legitimate commit to the
# same branch on the shared remote. Our clone's remote-tracking ref for that branch is pinned
# stale on purpose (remote.origin.fetch restricted to main only), so push's own internal
# `git fetch origin --prune` (Step 3) never learns about the other clone's push. This is exactly
# the situation --force-with-lease exists to protect against: with the correct flag, `git push
# --force-with-lease` refuses because the remote moved past what our clone last recorded, and
# the other clone's commit survives untouched. A raw `--force` push does not consult that
# recorded state at all — it destroys the other clone's commit unconditionally.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-push-force-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-push-parity.sh / check-ship-force-parity.sh.
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
  echo "check-push-force-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-push-force-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

REAL_NODE=$(command -v node || true)
REAL_PYTHON3=$(command -v python3 || true)
if [[ -z "$REAL_NODE" ]]; then
  echo "check-push-force-parity: node not found in PATH" >&2
  exit 1
fi
if [[ -z "$REAL_PYTHON3" ]]; then
  echo "check-push-force-parity: python3 not found in PATH" >&2
  exit 1
fi

# runtimebin/ carries ONLY the interpreters the three CLIs need, symlinked from their real
# location — the scenario-controlled PATH built below never inherits the caller's PATH, so a
# real gh/az/glab installed on the host can never leak into a scenario that must see none.
RUNTIME_BIN="$WORK/runtimebin"
mkdir -p "$RUNTIME_BIN"
ln -s "$REAL_NODE" "$RUNTIME_BIN/node"
ln -s "$REAL_PYTHON3" "$RUNTIME_BIN/python3"

# BASE_PATH: git + coreutils only, plus the two interpreters above. No gh/glab/az anywhere
# unless a scenario explicitly prepends its own stub directory.
BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"

# Never let an inherited TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make the forge adapter report
# "unavailable" regardless of PATH — CI's `make parity` step sets this env var for every gate in
# the target (it exists so check-push-parity.sh can force the no-forge-CLI path deterministically
# there), and it leaks into every script `make parity` runs afterwards, including this one. Left
# unset here, it collapses scenarios (b)/(c)/(d) — which all stub `gh` in PATH and expect it to
# be detected — onto the same "no forge CLI" refusal as scenario (a), for the wrong reason. Same
# fix already applied in check-ship-force-parity.sh and check-release-tag-parity.sh.
unset TRACKFW_DISABLE_EXTERNAL_COMMANDS || true

# NO_FORGE_PATH — used ONLY by scenario (a) below, which must prove genuine absence of a forge
# CLI via defaultAvailFn's real exec.LookPath(name) call (never via
# TRACKFW_DISABLE_EXTERNAL_COMMANDS=1, which the other scenarios need OFF anyway and which would
# skip that code path entirely instead of exercising it). BASE_PATH's "/usr/bin:/bin" is NOT safe
# for this: the GitHub Actions ubuntu-latest runner ships a real `gh` at /usr/bin/gh (ML-6B), so a
# scenario meant to see no forge CLI would see a real one there. GIT_ONLY_BIN carries nothing but
# a symlink to the real `git` this host resolves — no coreutils, no /usr/bin, no /bin — because
# nothing this scenario exercises (git plumbing over the local bare-repo file transport, plus the
# node/python3 interpreters already isolated in RUNTIME_BIN) needs anything else on PATH.
REAL_GIT=$(command -v git || true)
if [[ -z "$REAL_GIT" ]]; then
  echo "check-push-force-parity: git not found in PATH" >&2
  exit 1
fi
GIT_ONLY_BIN="$WORK/gitonlybin"
mkdir -p "$GIT_ONLY_BIN"
ln -s "$REAL_GIT" "$GIT_ONLY_BIN/git"
NO_FORGE_PATH="$RUNTIME_BIN:$GIT_ONLY_BIN"

# Non-vacuity guard — fails BEFORE any scenario runs if gh/glab/az somehow resolve on
# NO_FORGE_PATH, or if git does NOT resolve on it.
for cli in gh glab az; do
  if resolved=$(PATH="$NO_FORGE_PATH" command -v "$cli" 2>/dev/null); then
    echo "check-push-force-parity: vacuity guard failed — '$cli' resolves on NO_FORGE_PATH ($NO_FORGE_PATH) at $resolved; the no-forge-cli scenario would prove nothing" >&2
    exit 1
  fi
done
if ! PATH="$NO_FORGE_PATH" command -v git >/dev/null 2>&1; then
  echo "check-push-force-parity: vacuity guard failed — git does not resolve on NO_FORGE_PATH ($NO_FORGE_PATH)" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# gh stubs — one directory per PR state. Never emits anything gh's real output shape wouldn't:
# a JSON array (possibly empty) on success, non-zero exit + stderr on failure.
# ---------------------------------------------------------------------------
write_gh_stub() {
  local dir=$1 mode=$2
  mkdir -p "$dir"
  case "$mode" in
    open)
      cat >"$dir/gh" <<'EOF'
#!/usr/bin/env bash
echo '[{"number":42}]'
exit 0
EOF
      ;;
    empty)
      cat >"$dir/gh" <<'EOF'
#!/usr/bin/env bash
echo '[]'
exit 0
EOF
      ;;
    unverifiable)
      cat >"$dir/gh" <<'EOF'
#!/usr/bin/env bash
echo 'gh: authentication required, run `gh auth login` (stub)' >&2
exit 1
EOF
      ;;
    *)
      echo "write_gh_stub: unknown mode '$mode'" >&2
      exit 1
      ;;
  esac
  chmod +x "$dir/gh"
}

# ---------------------------------------------------------------------------
# make_fixture DEST BRANCH — real bare "origin" + a clone on BRANCH, one commit pushed, then
# amended locally (new SHA) so a plain push would be rejected and --force-with-lease is the
# governed way through.
#
# Key difference vs check-ship-force-parity.sh: writes `forge: github` into trackfw.yaml inside
# the clone. push has no --forge flag; it reads ConfigForge from config.Load().Forge, which reads
# trackfw.yaml in cwd. This is the production path — verified empirically against all 3 runtimes.
#
# Prints the clone path.
# ---------------------------------------------------------------------------
make_fixture() {
  local dest=$1 branch=$2
  local bare="$dest/origin.git"
  local clone="$dest/clone"
  local gitcfg="$dest/empty-gitconfig"
  mkdir -p "$dest"
  : >"$gitcfg"

  local -a e=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
  )

  git init -q --bare -b main "$bare" >"$dest/build.log" 2>&1
  env "${e[@]}" git clone -q "$bare" "$clone" >>"$dest/build.log" 2>&1
  (
    cd "$clone"
    env "${e[@]}" git config user.email "falsify@trackfw.test"
    env "${e[@]}" git config user.name "trackfw parity gate"
    env "${e[@]}" git config commit.gpgsign false
    env "${e[@]}" git config core.hooksPath /dev/null

    # trackfw.yaml with forge: github — push reads this via config.Load().Forge.
    # All scenarios use chore/ branches to skip the REQ+roadmap governance gate.
    cat >trackfw.yaml <<'YAML'
version: "1"
req_dir: docs/req/
roadmap_dir: docs/roadmaps/
forge: github
YAML

    echo base >base.txt
    env "${e[@]}" git add base.txt trackfw.yaml
    env "${e[@]}" git commit -q -m "base commit"
    env "${e[@]}" git push -q origin main

    env "${e[@]}" git checkout -q -b "$branch"
    echo work >work.txt
    env "${e[@]}" git add work.txt
    env "${e[@]}" git commit -q -m "work on $branch"
    env "${e[@]}" git push -q origin "$branch"

    # Amend locally — new SHA, remote unaware. A plain `git push` would now be rejected
    # (non-fast-forward); --force-with-lease is the governed path through.
    env "${e[@]}" git commit -q --amend -m "work on $branch (amended)"
  ) >>"$dest/build.log" 2>&1
  echo "$clone"
}

remote_head() {
  local bare=$1 branch=$2
  git --git-dir="$bare" rev-parse "$branch" 2>/dev/null || echo "<no-ref>"
}

# run_push RUNTIME DIR PATH_PREFIX ARGS...
# Runs `trackfw push ARGS...` from DIR with PATH="<PATH_PREFIX>:$BASE_PATH" (PATH_PREFIX may be
# empty). Sets PF_EXIT and writes stdout/stderr to $WORK/<label>.<runtime>.{out,err}.
# If RUN_PATH_OVERRIDE is set (non-empty), it REPLACES the whole PATH computation above —
# BASE_PATH is not consulted at all. Used only by scenario (a), which needs NO_FORGE_PATH exactly
# (no /usr/bin, no /bin) rather than a prefix layered on top of BASE_PATH.
PF_LABEL=""
PF_EXIT=0
PF_OUT_FILE=""
PF_ERR_FILE=""
run_push() {
  local runtime=$1 dir=$2 path_prefix=$3
  shift 3
  local out_file="$WORK/$PF_LABEL.$runtime.out" err_file="$WORK/$PF_LABEL.$runtime.err"
  local run_path
  if [[ -n "${RUN_PATH_OVERRIDE:-}" ]]; then
    run_path="$RUN_PATH_OVERRIDE"
  else
    run_path="$BASE_PATH"
    if [[ -n "$path_prefix" ]]; then
      run_path="$path_prefix:$BASE_PATH"
    fi
  fi
  set +e
  case "$runtime" in
    go)   (cd "$dir" && PATH="$run_path" "$GO_BIN" push "$@")                                >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && PATH="$run_path" node "$NODE_CLI" push "$@")                         >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PATH="$run_path" PYTHONPATH="$PY_ROOT" python3 -m trackfw push "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_push: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  PF_EXIT=$?
  set -e
  PF_OUT_FILE=$out_file
  PF_ERR_FILE=$err_file
}

# assert_three_way LABEL — byte-level diff of stdout/stderr across the 3 runtimes, plus exit
# code equality.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "push-force-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "push-force-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "push-force-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "push-force-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Scenario (a) — sem CLI de forge: PATH has no gh/glab/az at all. Must refuse without
# degrading to a permissive push, remote untouched. Runs against NO_FORGE_PATH (git-only,
# curated), NOT BASE_PATH — see the NO_FORGE_PATH comment above for why.
# ---------------------------------------------------------------------------
PF_LABEL="no-forge-cli"
RUN_PATH_OVERRIDE="$NO_FORGE_PATH"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/a-$runtime" "chore/no-forge-cli")
  bare="$WORK/a-$runtime/origin.git"
  before=$(remote_head "$bare" "chore/no-forge-cli")
  run_push "$runtime" "$fixture" "" --force-with-lease
  echo "$PF_EXIT" >"$WORK/$PF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/no-forge-cli")
  if [[ "$PF_EXIT" -eq 0 ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "expected non-zero exit when no forge CLI is available, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "refusal must never touch the remote; before=$before after=$after"
    continue
  fi
  if ! grep -qF 'requires a forge CLI (gh, glab, or az) to confirm an open pull/merge request' "$PF_ERR_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "vacuity guard: stderr missing the no-forge-CLI refusal; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
done
unset RUN_PATH_OVERRIDE
assert_three_way "$PF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — forge, zero PR: gh available, `pr list` returns an empty array. Must refuse
# naming the branch and pointing at opening the PR first; remote untouched.
# ---------------------------------------------------------------------------
PF_LABEL="forge-zero-pr"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/b-$runtime" "chore/zero-pr")
  bare="$WORK/b-$runtime/origin.git"
  stub="$WORK/b-$runtime-stub"
  write_gh_stub "$stub" empty
  before=$(remote_head "$bare" "chore/zero-pr")
  run_push "$runtime" "$fixture" "$stub" --force-with-lease
  echo "$PF_EXIT" >"$WORK/$PF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/zero-pr")
  if [[ "$PF_EXIT" -eq 0 ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "expected non-zero exit with zero open PRs, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "refusal must never touch the remote; before=$before after=$after"
    continue
  fi
  if ! grep -qF 'has no open pull/merge request. Open the PR/MR first' "$PF_ERR_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "vacuity guard: stderr missing the no-open-PR refusal; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'chore/zero-pr' "$PF_ERR_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "refusal must name the branch; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
done
assert_three_way "$PF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — forge, não verificável: gh available but the `pr list` call itself fails
# (auth error). Must be its OWN refusal class — never conflated with "no PR" — and must surface
# the CLI's actual stderr text (the byte-for-byte discriminant that caught the real Go/Node/
# Python divergence this series fixed: Go's exec.Command().Output() error alone is the generic
# "exit status 1", discarding the CLI's own diagnostic that Node/Python already surfaced).
# ---------------------------------------------------------------------------
PF_LABEL="forge-unverifiable"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/c-$runtime" "chore/unverifiable")
  bare="$WORK/c-$runtime/origin.git"
  stub="$WORK/c-$runtime-stub"
  write_gh_stub "$stub" unverifiable
  before=$(remote_head "$bare" "chore/unverifiable")
  run_push "$runtime" "$fixture" "$stub" --force-with-lease
  echo "$PF_EXIT" >"$WORK/$PF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/unverifiable")
  if [[ "$PF_EXIT" -eq 0 ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "expected non-zero exit when the PR check cannot be verified, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "refusal must never touch the remote; before=$before after=$after"
    continue
  fi
  if ! grep -qF 'could not verify whether branch' "$PF_ERR_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "vacuity guard: stderr missing the cannot-verify refusal; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
  if grep -qF 'has no open pull/merge request' "$PF_ERR_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "cannot-verify must never be conflated with no-PR; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
  # The stub's actual stderr text must survive into the refusal — not a generic "exit status N".
  if ! grep -qF 'authentication required' "$PF_ERR_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "the forge CLI's real stderr text must surface in the refusal, not a generic exit-status message; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
done
assert_three_way "$PF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — forge, PR aberto: gh confirms an open PR. Push must actually happen —
# proved by the remote SHA changing, never by the printed message alone.
# ---------------------------------------------------------------------------
PF_LABEL="forge-pr-open-pushes"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/d-$runtime" "chore/pr-open")
  bare="$WORK/d-$runtime/origin.git"
  stub="$WORK/d-$runtime-stub"
  write_gh_stub "$stub" open
  before=$(remote_head "$bare" "chore/pr-open")
  fixture_head_sha=$(cd "$fixture" && git rev-parse HEAD)
  run_push "$runtime" "$fixture" "$stub" --force-with-lease
  echo "$PF_EXIT" >"$WORK/$PF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/pr-open")
  if [[ "$PF_EXIT" -ne 0 ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "expected exit 0 with an open PR confirmed, got $PF_EXIT; stderr: $(cat "$PF_ERR_FILE")"
    continue
  fi
  if [[ "$before" == "$after" ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "vacuity guard: remote SHA did not change — the push never happened; before=$before after=$after"
    continue
  fi
  if [[ "$after" != "$fixture_head_sha" ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "remote SHA after push must equal the local amended commit; local=$fixture_head_sha remote=$after"
    continue
  fi
  if ! grep -qF 'Pushed: ' "$PF_OUT_FILE"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "vacuity guard: stdout missing the completion marker ('Pushed: '); stdout: $(cat "$PF_OUT_FILE")"
    continue
  fi
done
assert_three_way "$PF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — semantic discriminant: --force-with-lease refuses when the remote advanced
# past what this clone last recorded; --force does not, and destroys the other party's commit.
# See the file header for the full construction. Proves the discriminant itself, per-runtime
# (not a cross-runtime diff — the three runtimes must ALL refuse and ALL leave the remote
# untouched; this is what scripts/check-gates-falsify.sh's Cenário 163 sabotages and this
# scenario catches).
# ---------------------------------------------------------------------------
PF_LABEL="remote-advanced-lease-mismatch"
for runtime in go node py; do
  dest="$WORK/e-$runtime"
  bare="$dest/origin.git"
  clone="$dest/clone"
  other="$dest/other"
  gitcfg="$dest/empty-gitconfig"
  mkdir -p "$dest"
  : >"$gitcfg"
  e=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
  )
  git init -q --bare -b main "$bare" >"$dest/build.log" 2>&1
  env "${e[@]}" git clone -q "$bare" "$clone" >>"$dest/build.log" 2>&1
  (
    cd "$clone"
    env "${e[@]}" git config user.email "falsify@trackfw.test"
    env "${e[@]}" git config user.name "trackfw parity gate"
    env "${e[@]}" git config commit.gpgsign false
    env "${e[@]}" git config core.hooksPath /dev/null

    # trackfw.yaml with forge: github — push reads this via config.Load().Forge.
    cat >trackfw.yaml <<'YAML'
version: "1"
req_dir: docs/req/
roadmap_dir: docs/roadmaps/
forge: github
YAML

    echo base >base.txt
    env "${e[@]}" git add base.txt trackfw.yaml
    env "${e[@]}" git commit -q -m "base commit"
    env "${e[@]}" git push -q origin main
    env "${e[@]}" git checkout -q -b chore/remote-advanced
    echo work >work.txt
    env "${e[@]}" git add work.txt
    env "${e[@]}" git commit -q -m "work on chore/remote-advanced"
    env "${e[@]}" git push -q origin chore/remote-advanced
  ) >>"$dest/build.log" 2>&1

  # A second, independent clone pushes ONE more legitimate commit to the same branch.
  env "${e[@]}" git clone -q "$bare" "$other" >>"$dest/build.log" 2>&1
  (
    cd "$other"
    env "${e[@]}" git config user.email "other@trackfw.test"
    env "${e[@]}" git config user.name "trackfw parity gate (other clone)"
    env "${e[@]}" git config commit.gpgsign false
    env "${e[@]}" git config core.hooksPath /dev/null
    env "${e[@]}" git checkout -q chore/remote-advanced
    echo extra >extra.txt
    env "${e[@]}" git add extra.txt
    env "${e[@]}" git commit -q -m "another party's legitimate commit"
    env "${e[@]}" git push -q origin chore/remote-advanced
  ) >>"$dest/build.log" 2>&1

  # Our clone's remote-tracking ref stays pinned stale on purpose (fetch refspec restricted to
  # main only) — so push's own internal `git fetch origin --prune` (Step 3) never learns about
  # the other clone's push, and the amended local commit below is the only thing our clone knows
  # to compare against.
  (
    cd "$clone"
    env "${e[@]}" git config remote.origin.fetch "+refs/heads/main:refs/remotes/origin/main"
    env "${e[@]}" git commit -q --amend -m "work on chore/remote-advanced (amended)"
  ) >>"$dest/build.log" 2>&1

  stub="$dest-stub"
  write_gh_stub "$stub" open

  remote_before=$(remote_head "$bare" "chore/remote-advanced")
  # Vacuity guard: the other clone's commit must actually be the current remote tip before we
  # exercise the discriminant, or a "remote unchanged" assertion below would prove nothing.
  # Captured into a variable before grep -qF, never piped directly: under `set -o pipefail`, a
  # `git log | grep -q` pipe can make `git log` receive SIGPIPE once grep is satisfied and
  # closes early, turning the pipeline's exit status non-zero even though grep DID match —
  # cost a real debugging cycle building check-ship-force-parity.sh.
  remote_log=$(git --git-dir="$bare" log --oneline chore/remote-advanced 2>/dev/null || true)
  if ! grep -qF "another party's legitimate commit" <<<"$remote_log"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "vacuity guard: the other clone's commit is not on the remote before running push"
    continue
  fi

  run_push "$runtime" "$clone" "$stub" --force-with-lease
  echo "$PF_EXIT" >"$WORK/$PF_LABEL.$runtime.exit"
  remote_after=$(remote_head "$bare" "chore/remote-advanced")

  if [[ "$PF_EXIT" -eq 0 ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "--force-with-lease must refuse when the remote advances past the recorded lease (real git safety semantics), got exit 0; stdout: $(cat "$PF_OUT_FILE")"
    continue
  fi
  if [[ "$remote_before" != "$remote_after" ]]; then
    fail "push-force-parity/$PF_LABEL/$runtime" "the other clone's commit must survive untouched; remote moved from $remote_before to $remote_after"
    continue
  fi
  # Captured into a variable before grep -qF, never piped directly.
  remote_log=$(git --git-dir="$bare" log --oneline chore/remote-advanced 2>/dev/null || true)
  if ! grep -qF "another party's legitimate commit" <<<"$remote_log"; then
    fail "push-force-parity/$PF_LABEL/$runtime" "the other clone's commit must still be reachable from the remote branch after the refused push"
    continue
  fi

  # Normalize: git's own rejection message embeds the bare remote's absolute filesystem path
  # ("To /.../e-go/origin.git", "error: failed to push ... '/.../e-go/origin.git'") — this
  # differs across runtimes ONLY because each runtime's fixture is built in its own isolated
  # directory (e-go/e-node/e-py), never because of anything the CLI under test does. Substitute
  # the real path with a fixed placeholder before the byte-diff so the comparison targets git's
  # actual diagnostic text, not this gate's own fixture layout.
  sed -i.bak "s#$dest#<FIXTURE_DIR>#g" "$PF_OUT_FILE" "$PF_ERR_FILE" && rm -f "$PF_OUT_FILE.bak" "$PF_ERR_FILE.bak"
done
assert_three_way "$PF_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-push-force-parity.sh scenarios passed."
else
  echo "check-push-force-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
