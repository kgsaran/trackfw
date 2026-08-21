#!/usr/bin/env bash
# check-release-tag-parity.sh — proves `trackfw release tag <version>` behaves byte-for-byte
# identically in Go, Node.js, and Python (ML-2B, ROADMAP-2026-08-19-caminho-governado-para-
# push-forcado-e-tag-de-release.md).
#
# `release tag` has NO --forge flag (unlike `ship`) — the only way to reach the "forge CLI
# available" branch in this fixture is a real trackfw.yaml with `forge: github`. A local bare
# remote never resolves to a known host, so forge.Resolve() lands on "manual" (Source: "none")
# without that yaml file. Getting this wrong makes the no-forge-cli/success scenarios silently
# vacuous — they would still refuse/succeed and still diff clean across 3 runtimes, but for the
# WRONG reason (unsupported-forge, not no-CLI). Every scenario below therefore asserts its own
# distinct refusal literal via grep -qF, never exit-code-only.
#
# Nine refusal paths + one success path, all against a SHARED fixture per scenario (never
# rebuilt per runtime): unlike ship --force-with-lease, NOTHING in this command writes to the
# remote in any scenario, including success — publishing goes through the two `gh api` calls,
# which are always a local stub here. A shared fixture means no per-runtime SHA/path
# normalization is needed before the byte-diff (contrast check-ship-force-parity.sh, whose
# success path genuinely rewrites remote history and needs a fresh fixture + path scrub).
#
# The load-bearing assertion for the success/P4 scenario is the SHA LINKAGE between the two `gh
# api` calls, not just "two calls happened": the ref-creation payload's `sha` field must equal
# the tag-OBJECT sha the first call returned (a fixed fake, deliberately different from the
# commit sha) — never the commit sha itself. A tag created by pointing the ref straight at the
# commit (skipping the tag-object call, or wiring the second payload to the wrong sha) is a
# LIGHTWEIGHT tag: the ref exists, `git describe`/`git tag -l` find it, and the loss is
# invisible until someone looks for the release message on the tag object. This is exactly what
# scripts/check-gates-falsify.sh's P4 scenario sabotages (single-literal delta on
# internal/commands/release.go's refPayload construction, isolated Go copy) and this scenario's
# payload-linkage assertion is what catches it.
#
# Fixture conventions, per KG's constraints (same as check-ship-force-parity.sh):
#   1. A REAL bare git remote, local, offline — never a mocked `git`.
#   2. $HOME redirected, GIT_CONFIG_GLOBAL/GIT_CONFIG_SYSTEM isolated — never the real user
#      gitconfig or credential helpers. Unlike check-ship-force-parity.sh (whose operations never
#      depend on git identity), release tag's own identity precondition means this isolation must
#      also apply at INVOCATION time, not only during fixture construction — otherwise a
#      developer machine with a real `git config --global user.name` set would make the
#      identity-missing scenario silently pass for the wrong reason (falling through to the real
#      global identity instead of refusing).
#   3. `gh` is stubbed via a directory prepended to a PATH built from scratch (never the
#      inherited PATH) — this machine may have a real `gh` installed, and PATH must guarantee the
#      "no forge CLI" scenario truly sees none.
#   4. The gate NEVER touches a real remote or forge: origin is a local bare repo (file
#      protocol, no credential helper needed), and the ONLY "publish" ever exercised is against
#      the local `gh` stub, whose two calls are captured to disk and asserted on directly.
#   5. File edits to fixture content (isolating one of the 5 version checks) use python3, never
#      `sed -i` — BSD vs GNU divergence already broke CI once in this series.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-release-tag-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-ship-force-parity.sh.
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
  echo "check-release-tag-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-release-tag-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

REAL_NODE=$(command -v node || true)
REAL_PYTHON3=$(command -v python3 || true)
if [[ -z "$REAL_NODE" ]]; then
  echo "check-release-tag-parity: node not found in PATH" >&2
  exit 1
fi
if [[ -z "$REAL_PYTHON3" ]]; then
  echo "check-release-tag-parity: python3 not found in PATH" >&2
  exit 1
fi

# runtimebin/ carries ONLY the interpreters the three CLIs need, symlinked from their real
# location — the scenario-controlled PATH built below never inherits the caller's PATH, so a
# real gh installed on this machine can never leak into a scenario that must see none.
RUNTIME_BIN="$WORK/runtimebin"
mkdir -p "$RUNTIME_BIN"
ln -s "$REAL_NODE" "$RUNTIME_BIN/node"
ln -s "$REAL_PYTHON3" "$RUNTIME_BIN/python3"

# BASE_PATH: git + coreutils + python3 (used by patch_version_file below) only, plus the two
# interpreters above. No gh anywhere unless a scenario explicitly prepends its own stub dir.
BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"

# Never let an inherited TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make the forge adapter report
# "unavailable" regardless of PATH — that would collapse the no-forge-cli and success scenarios
# onto the same outcome for the wrong reason.
unset TRACKFW_DISABLE_EXTERNAL_COMMANDS || true

# NO_FORGE_PATH — used ONLY by Scenario 7 (no-forge-cli) below, which must prove genuine absence
# of a forge CLI via defaultAvailFn's real exec.LookPath("gh") call, not via
# TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 (which every other scenario here needs OFF anyway, and
# which would skip that code path instead of exercising it). BASE_PATH's "/usr/bin:/bin" is NOT
# safe for this: the GitHub Actions ubuntu-latest runner ships a real `gh` at /usr/bin/gh (see
# ML-6B / the "no-forge-cli" CI failure this gate never actually reached before). GIT_ONLY_BIN
# carries nothing but a symlink to the real `git` this host resolves — the product only ever execs
# "git" and the resolved forge CLI name (gh); patch_version_file/json_field's python3 calls below
# run in the SCRIPT's own process, outside this restricted PATH, so they are unaffected.
REAL_GIT=$(command -v git || true)
if [[ -z "$REAL_GIT" ]]; then
  echo "check-release-tag-parity: git not found in PATH" >&2
  exit 1
fi
GIT_ONLY_BIN="$WORK/gitonlybin"
mkdir -p "$GIT_ONLY_BIN"
ln -s "$REAL_GIT" "$GIT_ONLY_BIN/git"
NO_FORGE_PATH="$RUNTIME_BIN:$GIT_ONLY_BIN"

# Non-vacuity guard — fails BEFORE any scenario runs if gh/glab/az somehow resolve on
# NO_FORGE_PATH, or if git does NOT resolve on it. A "no forge CLI" scenario running against a
# PATH secretly still carrying a forge CLI would pass for the wrong reason.
for cli in gh glab az; do
  if resolved=$(PATH="$NO_FORGE_PATH" command -v "$cli" 2>/dev/null); then
    echo "check-release-tag-parity: vacuity guard failed — '$cli' resolves on NO_FORGE_PATH ($NO_FORGE_PATH) at $resolved; the no-forge-cli scenario would prove nothing" >&2
    exit 1
  fi
done
if ! PATH="$NO_FORGE_PATH" command -v git >/dev/null 2>&1; then
  echo "check-release-tag-parity: vacuity guard failed — git does not resolve on NO_FORGE_PATH ($NO_FORGE_PATH)" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

RELEASE_VERSION="9.9.9"
RELEASE_TAG="v9.9.9"
# Deliberately different from any real commit sha the fixture produces — the discriminant P4
# sabotages: a lightweight-tag regression makes the ref payload's sha collapse to the COMMIT
# sha instead of this fake tag-object sha.
FAKE_TAG_OBJECT_SHA="deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

# ---------------------------------------------------------------------------
# write_version_files DIR VERSION — writes the 5 files release tag reads, all agreeing with
# VERSION. Mirrors internal/commands/release_test.go's validReleaseVersionFiles.
# ---------------------------------------------------------------------------
write_version_files() {
  local dir=$1 version=$2
  mkdir -p "$dir/internal/version" "$dir/npm" "$dir/pypi/trackfw"
  cat >"$dir/internal/version/version.go" <<EOF
package version

var Version = "$version"
EOF
  cat >"$dir/npm/package.json" <<EOF
{"name":"trackfw","version":"$version"}
EOF
  cat >"$dir/pypi/pyproject.toml" <<EOF
[project]
name = "trackfw"
version = "$version"
EOF
  cat >"$dir/pypi/trackfw/__init__.py" <<EOF
try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "$version"
except Exception:
    __version__ = "$version"
EOF
  cat >"$dir/CHANGELOG.md" <<EOF
# Changelog

## [$version] - 2026-08-19

### Added
- x
EOF
}

# patch_version_file FILE OLD NEW — first-occurrence literal substring replace via python3, never
# sed -i (BSD/GNU divergence already broke CI once in this series). Fails loudly if OLD is
# absent, so a scenario can never silently degrade into "nothing changed, refusal proves
# nothing".
patch_version_file() {
  local file=$1 old=$2 new=$3
  python3 - "$file" "$old" "$new" <<'PYEOF'
import sys
path, old, new = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path) as f:
    content = f.read()
if old not in content:
    print(f"patch_version_file: pattern not found in {path}: {old!r}", file=sys.stderr)
    sys.exit(1)
content = content.replace(old, new, 1)
with open(path, "w") as f:
    f.write(content)
PYEOF
}

# json_field FILE KEY — prints a top-level string field from a small JSON file, via python3.
json_field() {
  local file=$1 key=$2
  python3 - "$file" "$key" <<'PYEOF'
import json, sys
path, key = sys.argv[1], sys.argv[2]
with open(path) as f:
    data = json.load(f)
print(data[key])
PYEOF
}

# ---------------------------------------------------------------------------
# write_release_gh_stub DIR CALL_LOG [FORGE_DEFAULT_BRANCH] [FORGE_COMMIT_SHA] — a `gh` stub
# that answers all four `gh api` calls release tag makes:
#   GET  repos/{owner}/{repo}                      -> default_branch (FORGE_DEFAULT_BRANCH,
#        default "main" — the forge's answer, never read from a local ref)
#   GET  repos/{owner}/{repo}/commits/<branch>      -> sha (FORGE_COMMIT_SHA, default the fixed
#        RELEASE_TAG-unrelated fallback below when unset — callers that need a REAL commit sha
#        pass it explicitly)
#   POST repos/{owner}/{repo}/git/tags              -> sha DELIBERATELY DIFFERENT from any real
#        commit sha (FAKE_TAG_OBJECT_SHA)
#   POST repos/{owner}/{repo}/git/refs              -> {}
# Only the two POST calls are recorded to CALL_LOG (in call order) — the GET calls are answered
# but never logged, so scenario 9's "no publish happened" assertion (absence of *.json files in
# CALL_LOG) stays meaningful even though GET calls now happen before the identity check.
# ---------------------------------------------------------------------------
write_release_gh_stub() {
  local dir=$1 call_log=$2 forge_default_branch=${3:-main} forge_commit_sha=${4:-0000000000000000000000000000000000000f}
  mkdir -p "$dir" "$call_log"
  cat >"$dir/gh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
body=\$(cat)
n=\$(find "$call_log" -maxdepth 1 -name '*.json' | wc -l | tr -d ' ')
n=\$((n + 1))
case "\$2" in
  *git/tags*)
    printf '%s' "\$body" >"$call_log/\$(printf '%02d' "\$n")-tags-request.json"
    echo '{"sha":"$FAKE_TAG_OBJECT_SHA"}'
    ;;
  *git/refs*)
    printf '%s' "\$body" >"$call_log/\$(printf '%02d' "\$n")-refs-request.json"
    echo '{"ref":"refs/tags/$RELEASE_TAG","object":{"sha":"$FAKE_TAG_OBJECT_SHA"}}'
    ;;
  *commits/*)
    echo '{"sha":"$forge_commit_sha"}'
    ;;
  repos/\{owner\}/\{repo\})
    echo '{"default_branch":"$forge_default_branch"}'
    ;;
  *)
    echo "release-tag-parity stub: unexpected gh call: \$*" >&2
    exit 1
    ;;
esac
EOF
  chmod +x "$dir/gh"
}

# ---------------------------------------------------------------------------
# build_fixture DEST FORGE SET_IDENTITY — real bare "origin" + a clone on "main" carrying the 5
# version files (all at RELEASE_VERSION) + CHANGELOG.md with the matching section, committed and
# pushed so local main == origin/main (precondition 2 satisfied by construction). FORGE="github"
# writes trackfw.yaml with `forge: github`; FORGE="" writes no trackfw.yaml (forge resolves to
# "manual" against this fixture — no known host, no CI files). SET_IDENTITY=1 sets LOCAL
# user.name/user.email (git config, no --global) so the identity precondition passes;
# SET_IDENTITY=0 commits via GIT_AUTHOR_*/GIT_COMMITTER_* env vars instead (bootstrap only) and
# never touches local config, so `git config user.name` at invocation time returns empty against
# the isolated GIT_CONFIG_GLOBAL/HOME this gate always runs with. Prints the clone path.
# ---------------------------------------------------------------------------
build_fixture() {
  local dest=$1 forge=$2 set_identity=$3
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

  write_version_files "$clone" "$RELEASE_VERSION"
  if [[ "$forge" == "github" ]]; then
    printf 'forge: github\n' >"$clone/trackfw.yaml"
  fi

  (
    cd "$clone"
    env "${e[@]}" git config commit.gpgsign false
    env "${e[@]}" git config core.hooksPath /dev/null
    if [[ "$set_identity" == "1" ]]; then
      env "${e[@]}" git config user.email "release-parity@trackfw.test"
      env "${e[@]}" git config user.name "trackfw release parity"
      env "${e[@]}" git add -A
      env "${e[@]}" git commit -q -m "fixture: valid release state"
    else
      env "${e[@]}" GIT_AUTHOR_NAME="fixture bootstrap" GIT_AUTHOR_EMAIL="bootstrap@trackfw.test" \
          GIT_COMMITTER_NAME="fixture bootstrap" GIT_COMMITTER_EMAIL="bootstrap@trackfw.test" \
          git add -A
      env "${e[@]}" GIT_AUTHOR_NAME="fixture bootstrap" GIT_AUTHOR_EMAIL="bootstrap@trackfw.test" \
          GIT_COMMITTER_NAME="fixture bootstrap" GIT_COMMITTER_EMAIL="bootstrap@trackfw.test" \
          git commit -q -m "fixture: valid release state (no identity)"
    fi
    env "${e[@]}" git push -q origin main
  ) >>"$dest/build.log" 2>&1
  echo "$clone"
}

# commit_and_push_mutation CLONE ENV_FILE — commits whatever is dirty in CLONE (identity already
# configured locally by build_fixture) and pushes to origin/main, so local main stays == origin/
# main and precondition 2 never fires ahead of the precondition under test.
commit_and_push_mutation() {
  local clone=$1 gitcfg=$2 dest=$3
  local -a e=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
  )
  (
    cd "$clone"
    env "${e[@]}" git add -A
    env "${e[@]}" git commit -q -m "fixture: mutation"
    env "${e[@]}" git push -q origin main
  ) >>"$dest/build.log" 2>&1
}

# run_release RUNTIME DIR PATH_PREFIX DEST ARGS... — runs `trackfw release tag ARGS...` from DIR,
# with PATH="<PATH_PREFIX>:$BASE_PATH" (PATH_PREFIX may be empty) and $HOME/GIT_CONFIG_GLOBAL
# isolated to DEST's fixture files — this isolation matters at INVOCATION time here (unlike
# check-ship-force-parity.sh's run_ship), because the identity precondition would otherwise fall
# through to whatever real global git identity this machine happens to have configured.
run_release() {
  local runtime=$1 dir=$2 path_prefix=$3 dest=$4
  shift 4
  local out_file="$WORK/$RT_LABEL.$runtime.out" err_file="$WORK/$RT_LABEL.$runtime.err"
  local run_path
  if [[ -n "${RUN_PATH_OVERRIDE:-}" ]]; then
    run_path="$RUN_PATH_OVERRIDE"
  else
    run_path="$BASE_PATH"
    if [[ -n "$path_prefix" ]]; then
      run_path="$path_prefix:$BASE_PATH"
    fi
  fi
  local gitcfg="$dest/empty-gitconfig"
  local -a e=(
    "GIT_CONFIG_GLOBAL=$gitcfg"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$dest"
    "PATH=$run_path"
  )
  set +e
  case "$runtime" in
    go)   (cd "$dir" && env "${e[@]}" "$GO_BIN" release tag "$@")                                >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && env "${e[@]}" node "$NODE_CLI" release tag "$@")                         >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && env "${e[@]}" PYTHONPATH="$PY_ROOT" python3 -m trackfw release tag "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_release: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  RT_EXIT=$?
  set -e
  RT_OUT_FILE=$out_file
  RT_ERR_FILE=$err_file
}

# assert_three_way LABEL — byte-level diff of stdout/stderr across the 3 runtimes, plus exit code
# equality. Mirrors check-ship-force-parity.sh's assert_three_way exactly. Safe unnormalized: no
# scenario below ever prints a wall-clock timestamp, absolute fixture path, or SHA that differs
# run-to-run — every SHA in a message is either the fixed RELEASE_VERSION/RELEASE_TAG, the
# FAKE_TAG_OBJECT_SHA, or a real commit sha that is IDENTICAL across runtimes because the fixture
# is shared, not rebuilt per runtime.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "release-tag-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "release-tag-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "release-tag-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "release-tag-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Scenario 1 — dirty working tree. Also proves the ML-2B coherence fix: the refusal must name
# `trackfw commit`, and must NEVER recommend `git stash` (the git-branch-guard has blocked stash
# since ML-3A of this same roadmap — recommending a command the product itself refuses would be
# incoherent).
# ---------------------------------------------------------------------------
RT_LABEL="dirty-tree"
fixture=$(build_fixture "$WORK/s1" "github" "1")
echo dirty >>"$fixture/CHANGELOG.md"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$WORK/s1" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit for a dirty working tree, got 0"
    continue
  fi
  if ! grep -qF "working tree is not clean" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the dirty-tree refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "trackfw commit" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "coherence fix: refusal must name 'trackfw commit'; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if grep -qi "stash" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "coherence fix: refusal must NEVER recommend 'git stash' — the guard blocks it; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 2 — local main diverges from origin/main (amended locally, never pushed).
# ---------------------------------------------------------------------------
RT_LABEL="main-stale"
fixture=$(build_fixture "$WORK/s2" "github" "1")
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$WORK/s2/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$WORK/s2" \
    git commit -q --amend -m "fixture: valid release state (amended, unpushed)"
) >>"$WORK/s2/build.log" 2>&1
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$WORK/s2" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when local main diverges from origin/main, got 0"
    continue
  fi
  if ! grep -qF "is not up to date with origin/main" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the stale-local-branch refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenarios 3a-3e — the 5 version checks across the 4 files, one isolated mismatch each. Checks
# 4 and 5 share pypi/trackfw/__init__.py but target distinct, non-overlapping substrings (the
# try-block fallback vs. the except-block fallback), per the file's own doc comment.
# ---------------------------------------------------------------------------
declare -a MISMATCH_CASES=(
  "version-mismatch-go|internal/version/version.go|Version = \"$RELEASE_VERSION\"|Version = \"9.9.8\"|internal/version/version.go has version \"9.9.8\", expected \"$RELEASE_VERSION\""
  "version-mismatch-npm|npm/package.json|\"version\":\"$RELEASE_VERSION\"|\"version\":\"9.9.8\"|npm/package.json has version \"9.9.8\", expected \"$RELEASE_VERSION\""
  "version-mismatch-pyproject|pypi/pyproject.toml|version = \"$RELEASE_VERSION\"|version = \"9.9.8\"|pypi/pyproject.toml has version \"9.9.8\", expected \"$RELEASE_VERSION\""
  "version-mismatch-init-try|pypi/trackfw/__init__.py|or \"$RELEASE_VERSION\"|or \"9.9.8\"|pypi/trackfw/__init__.py (importlib.metadata fallback) has version \"9.9.8\", expected \"$RELEASE_VERSION\""
  "version-mismatch-init-except|pypi/trackfw/__init__.py|__version__ = \"$RELEASE_VERSION\"|__version__ = \"9.9.8\"|pypi/trackfw/__init__.py (except fallback) has version \"9.9.8\", expected \"$RELEASE_VERSION\""
)
for case_spec in "${MISMATCH_CASES[@]}"; do
  IFS='|' read -r RT_LABEL rel_path old_pattern new_pattern expect_msg <<<"$case_spec"
  dest="$WORK/$RT_LABEL"
  fixture=$(build_fixture "$dest" "github" "1")
  patch_version_file "$fixture/$rel_path" "$old_pattern" "$new_pattern"
  commit_and_push_mutation "$fixture" "$dest/empty-gitconfig" "$dest"
  # ML-2A (ROADMAP-2026-08-21): P3/P4 moved after forge resolution — a gh stub is now required
  # to reach P3/P4. The stub returns the post-mutation sha so git show <sha>:<path> returns the
  # mutated (mismatched) content and fires the expected refusal. Repair, not extension.
  real_sha=$(git --git-dir="$dest/origin.git" rev-parse main)
  stub_dir="$dest-stub"
  write_release_gh_stub "$stub_dir" "$dest-calls" "main" "$real_sha"
  for runtime in go node py; do
    run_release "$runtime" "$fixture" "$stub_dir" "$dest" "$RELEASE_VERSION"
    echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
    if [[ "$RT_EXIT" -eq 0 ]]; then
      fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit for a version mismatch in $rel_path, got 0"
      continue
    fi
    if ! grep -qF "$expect_msg" "$RT_ERR_FILE"; then
      fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing '$expect_msg'; stderr: $(cat "$RT_ERR_FILE")"
      continue
    fi
  done
  # The stub now makes publishing physically reachable — verify no POST calls were made.
  if [[ -n "$(find "$dest-calls" -maxdepth 1 -name '*.json' 2>/dev/null)" ]]; then
    fail "release-tag-parity/$RT_LABEL/no-publish" "refusal must never reach the publish calls; found: $(ls "$dest-calls")"
  fi
  assert_three_way "$RT_LABEL"
done

# ---------------------------------------------------------------------------
# Scenario 4 — CHANGELOG.md missing the version's section.
# ---------------------------------------------------------------------------
RT_LABEL="changelog-missing"
dest="$WORK/s4"
fixture=$(build_fixture "$dest" "github" "1")
printf '# Changelog\n' >"$fixture/CHANGELOG.md"
commit_and_push_mutation "$fixture" "$dest/empty-gitconfig" "$dest"
# ML-2A (ROADMAP-2026-08-21): P4 moved after forge resolution — stub required to reach P4.
# The real post-mutation sha is used so git show <sha>:CHANGELOG.md returns the bare file
# (no version section) and fires the expected refusal. Repair, not extension.
real_sha_s4=$(git --git-dir="$dest/origin.git" rev-parse main)
stub_s4="$dest-stub"
write_release_gh_stub "$stub_s4" "$dest-calls" "main" "$real_sha_s4"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "$stub_s4" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when CHANGELOG.md lacks the version section, got 0"
    continue
  fi
  if ! grep -qF "version \"$RELEASE_VERSION\" not found in CHANGELOG.md" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the changelog-missing refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "## [$RELEASE_VERSION] - YYYY-MM-DD" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "refusal must name the exact section header to add; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
# The stub now makes publishing physically reachable — verify no POST calls were made.
if [[ -n "$(find "$dest-calls" -maxdepth 1 -name '*.json' 2>/dev/null)" ]]; then
  fail "release-tag-parity/$RT_LABEL/no-publish" "refusal must never reach the publish calls; found: $(ls "$dest-calls")"
fi
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 5 — tag already exists locally (never pushed).
# ---------------------------------------------------------------------------
RT_LABEL="tag-exists-local"
dest="$WORK/s5"
fixture=$(build_fixture "$dest" "github" "1")
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git tag "$RELEASE_TAG"
) >>"$dest/build.log" 2>&1
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when the tag already exists locally, got 0"
    continue
  fi
  if ! grep -qF "tag \"$RELEASE_TAG\" already exists locally" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the local-tag-exists refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 6 — tag already exists on origin (pushed, then deleted locally so only the remote
# check can discriminate it from Scenario 5). `remote.origin.tagOpt --no-tags` is set BEFORE the
# command's own internal `git fetch origin --prune` runs — without it, git's default tag
# auto-follow would silently re-download the pushed tag (it points at a commit already present
# locally) and this scenario would degrade into a duplicate of Scenario 5, proving nothing about
# the remote-only branch of precondition 5.
# ---------------------------------------------------------------------------
RT_LABEL="tag-exists-remote"
dest="$WORK/s6"
fixture=$(build_fixture "$dest" "github" "1")
(
  cd "$fixture"
  env_args=("GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest")
  env "${env_args[@]}" git tag "$RELEASE_TAG"
  env "${env_args[@]}" git push -q origin "$RELEASE_TAG"
  env "${env_args[@]}" git tag -d "$RELEASE_TAG"
  env "${env_args[@]}" git config remote.origin.tagOpt --no-tags
) >>"$dest/build.log" 2>&1
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when the tag already exists on origin, got 0"
    continue
  fi
  if ! grep -qF "tag \"$RELEASE_TAG\" already exists on origin" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the remote-tag-exists refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 7 — no forge CLI: trackfw.yaml resolves the forge to "github", but `gh` is nowhere on
# PATH. Must be a DIFFERENT refusal from Scenario 8 (unsupported forge) — conflating "forge
# resolved, CLI absent" with "forge not resolved to github at all" would hide a real regression.
# ---------------------------------------------------------------------------
RT_LABEL="no-forge-cli"
dest="$WORK/s7"
fixture=$(build_fixture "$dest" "github" "1")
RUN_PATH_OVERRIDE="$NO_FORGE_PATH"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit with no forge CLI on PATH, got 0"
    continue
  fi
  if ! grep -qF "trackfw release tag requires the GitHub CLI (gh) to publish the tag" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the no-forge-CLI refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
unset RUN_PATH_OVERRIDE
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 8 — unsupported forge: no trackfw.yaml at all, and a local bare remote never matches a
# known host, so forge.Resolve() lands on "manual".
# ---------------------------------------------------------------------------
RT_LABEL="unsupported-forge"
dest="$WORK/s8"
fixture=$(build_fixture "$dest" "" "1")
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when the resolved forge is not github, got 0"
    continue
  fi
  if ! grep -qF 'currently only supports GitHub (resolved forge: "manual")' "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the unsupported-forge refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if grep -qF "git push origin" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "coherence: unsupported-forge refusal must not instruct a raw 'git push origin' the guard would itself block; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 9 — git identity missing. Forge resolved to github and gh present, so the fixture
# actually reaches the identity check; local git config carries no user.name/user.email
# (build_fixture's set_identity=0 path), and run_release's invocation-time HOME/
# GIT_CONFIG_GLOBAL isolation ensures this can never fall through to a real global identity.
# ---------------------------------------------------------------------------
RT_LABEL="git-identity-missing"
dest="$WORK/s9"
fixture=$(build_fixture "$dest" "github" "0")
commit_sha_s9=$(git --git-dir="$dest/origin.git" rev-parse main)
stub="$dest-stub"
call_log="$dest-calls"
write_release_gh_stub "$stub" "$call_log" "main" "$commit_sha_s9"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "$stub" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit with no git identity configured, got 0"
    continue
  fi
  if ! grep -qF "git config user.name and user.email must be set" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the no-identity refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
if [[ -n "$(find "$call_log" -maxdepth 1 -name '*.json' 2>/dev/null)" ]]; then
  fail "release-tag-parity/$RT_LABEL/no-publish" "refusal must never reach the gh api calls; found request files: $(ls "$call_log")"
fi
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 10 — success, published entirely against the local `gh` stub. Never touches a real
# remote or forge. The load-bearing assertion is the SHA LINKAGE between the two calls: the
# second (git/refs) payload's `sha` must equal the FIRST call's returned tag-object sha
# (FAKE_TAG_OBJECT_SHA), never the commit sha — this is the exact property a lightweight-tag
# regression (P4's target) breaks.
# ---------------------------------------------------------------------------
RT_LABEL="success"
dest="$WORK/s10"
fixture=$(build_fixture "$dest" "github" "1")
commit_sha=$(git --git-dir="$dest/origin.git" rev-parse main)
stub="$dest-stub"
for runtime in go node py; do
  call_log="$dest-calls-$runtime"
  write_release_gh_stub "$stub-$runtime" "$call_log" "main" "$commit_sha"
  run_release "$runtime" "$fixture" "$stub-$runtime" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -ne 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected exit 0 on the fully valid fixture, got $RT_EXIT; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "Tag published: $RELEASE_TAG" "$RT_OUT_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stdout missing the completion marker; stdout: $(cat "$RT_OUT_FILE")"
    continue
  fi
  if ! grep -qF "tag object: $FAKE_TAG_OBJECT_SHA" "$RT_OUT_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stdout must echo the tag object sha the stub returned; stdout: $(cat "$RT_OUT_FILE")"
    continue
  fi
  if ! grep -qF "commit:     $commit_sha" "$RT_OUT_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stdout must echo the tagged commit sha; stdout: $(cat "$RT_OUT_FILE")"
    continue
  fi

  call_count=$(find "$call_log" -maxdepth 1 -name '*.json' | wc -l | tr -d ' ')
  if [[ "$call_count" -ne 2 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected exactly 2 gh api calls, got $call_count"
    continue
  fi
  if [[ ! -f "$call_log/01-tags-request.json" || ! -f "$call_log/02-refs-request.json" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected git/tags to be called before git/refs; found: $(ls "$call_log")"
    continue
  fi

  tags_object=$(json_field "$call_log/01-tags-request.json" object)
  tags_tag=$(json_field "$call_log/01-tags-request.json" tag)
  tags_type=$(json_field "$call_log/01-tags-request.json" type)
  refs_sha=$(json_field "$call_log/02-refs-request.json" sha)
  refs_ref=$(json_field "$call_log/02-refs-request.json" ref)

  if [[ "$tags_object" != "$commit_sha" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "tag-object payload 'object' must equal the commit sha; got $tags_object want $commit_sha"
    continue
  fi
  if [[ "$tags_tag" != "$RELEASE_TAG" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "tag-object payload 'tag' mismatch: got $tags_tag want $RELEASE_TAG"
    continue
  fi
  if [[ "$tags_type" != "commit" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "tag-object payload 'type' must be 'commit', got $tags_type"
    continue
  fi
  if [[ "$refs_ref" != "refs/tags/$RELEASE_TAG" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "ref payload 'ref' mismatch: got $refs_ref want refs/tags/$RELEASE_TAG"
    continue
  fi
  # The discriminant: the ref must point at the TAG OBJECT's sha, never the commit's sha
  # directly — that would be a lightweight tag wearing an annotated tag's success message.
  if [[ "$refs_sha" != "$FAKE_TAG_OBJECT_SHA" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "LIGHTWEIGHT-TAG REGRESSION: ref payload 'sha' must equal the tag-object sha ($FAKE_TAG_OBJECT_SHA), got $refs_sha (commit sha is $commit_sha)"
    continue
  fi

  # Affirmative non-publication proof: nothing under this fixture's real remote/local tag
  # namespace was ever touched — the whole "publish" happened only against the stub.
  if [[ -n "$(git --git-dir="$dest/origin.git" tag -l "$RELEASE_TAG")" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "the gate must never publish for real: a real tag now exists on the bare remote"
    continue
  fi
  if (cd "$fixture" && env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" HOME="$dest" git rev-parse -q --verify "refs/tags/$RELEASE_TAG" >/dev/null 2>&1); then
    fail "release-tag-parity/$RT_LABEL/$runtime" "the gate must never publish for real: a real local tag now exists in the fixture clone"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenarios 11-13 — adversarial selection of the commit target (ADR-2026-08-19, Emenda 1).
# Scenarios 12-13 attack the sha cross-check via the origin/<forge-branch> tracking ref: a direct
# `git update-ref` forgery under a narrowed refspec — the exact mechanism the ADR calls out as
# what made the original exploit reachable (12) — and a `remote.origin.fetch` narrowed BEFORE the
# command's own internal `git fetch origin --prune` runs, leaving the tracking ref naturally
# stale against a legitimate second push rather than actively forged (13, same narrowing
# technique check-ship-force-parity.sh's "remote-advanced-lease-mismatch" scenario already
# exercises for `ship --force-with-lease`). The `gh` stub in each scenario answers with the
# FORGE's true, unspoofed values — proving the divergence is caught by comparison against the
# forge, not by the local ref happening to be internally consistent. Both refuse, naming both
# shas.
#
# Scenario 11 attacks the OTHER local ref — the symref-derived branch NAME — and proves the
# opposite property: the forge's default_branch name is authoritative UNCONDITIONALLY, with no
# local-vs-forge name comparison at all. A repointed origin/HEAD symref is neutralized, not
# refused: the command resolves and publishes using the forge's real branch/sha, ignoring the
# repoint entirely. This is deliberate — a fresh/shallow clone legitimately has no origin/HEAD
# symref (defaultBaseBranch falls back to "main"), so treating a name disagreement as a refusal
# would be a false refusal against a legitimate repo, not a security check.
#
# IMPORTANT, discovered while building these three (see vault/notes/git-fetch-self-heals-
# forged-origin-head-and-tracking-refs-2026-08-19.md): the command's OWN internal
# `git fetch origin --prune` (Precondition 2) self-heals a forged origin/<base> tracking ref
# under the DEFAULT refspec — git 2.50 always force-updates `refs/remotes/origin/*` from the real
# remote on fetch. A standalone `git update-ref` forgery, run and then left for the command's own
# fetch to see, is silently undone before the comparison code ever runs — that is NOT this
# command being extra-safe, it is Scenarios 12-13 being vacuous unless the forgery is made to
# survive the fetch on purpose (`remote.origin.fetch` narrowed), matching exactly the mechanism
# the ADR names as the real attack surface.
# ---------------------------------------------------------------------------

# --- Scenario 11 — repointed origin/HEAD symref: proves it is NEUTRALIZED, not refused. The ---
# forge's default_branch ("main") is used unconditionally; origin/main (not origin/chore/other)
# is what gets cross-checked and published, regardless of the symref repoint.
RT_LABEL="forge-symref-repoint-neutralized"
dest="$WORK/s11"
fixture=$(build_fixture "$dest" "github" "1")
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git config remote.origin.followRemoteHEAD never
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/chore/other
) >>"$dest/build.log" 2>&1
commit_sha_s11=$(git --git-dir="$dest/origin.git" rev-parse main)
stub="$dest-stub"
for runtime in go node py; do
  call_log="$dest-calls-$runtime"
  write_release_gh_stub "$stub-$runtime" "$call_log" "main" "$commit_sha_s11"
  run_release "$runtime" "$fixture" "$stub-$runtime" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -ne 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected exit 0: a repointed local symref must be neutralized (the forge's branch name wins), never refused; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if grep -qF "chore/other" "$RT_ERR_FILE" 2>/dev/null; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stderr must not mention chore/other — the repointed symref must be silently ignored, not surfaced as a refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "commit:     $commit_sha_s11" "$RT_OUT_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stdout must echo the forge's real main commit sha, proving the repoint was ignored; stdout: $(cat "$RT_OUT_FILE")"
    continue
  fi
  if [[ ! -f "$call_log/01-tags-request.json" || ! -f "$call_log/02-refs-request.json" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected the neutralized repoint to still reach both publish calls; found: $(ls "$call_log" 2>/dev/null)"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# --- Scenario 12 — origin/main forged via `git update-ref` UNDER A NARROWED REFSPEC (so the ---
# command's own internal fetch cannot undo the forgery before reading it — without narrowing,
# this scenario is vacuous; see the note above). remote.origin.fetch is pinned to a decoy
# branch only, so the forgery on refs/remotes/origin/main survives untouched.
RT_LABEL="forge-commit-diverges-update-ref"
dest="$WORK/s12"
fixture=$(build_fixture "$dest" "github" "1")
real_commit_sha_s12=$(git --git-dir="$dest/origin.git" rev-parse main)
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git config remote.origin.fetch "+refs/heads/s12-decoy:refs/remotes/origin/s12-decoy"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git checkout -q -b s12-decoy
  echo decoy >>CHANGELOG.md
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git add CHANGELOG.md
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git commit -q -m "fixture: decoy commit, distinct sha from main"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s12-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git checkout -q main
) >>"$dest/build.log" 2>&1
forged_sha_s12=$(git --git-dir="$dest/origin.git" rev-parse s12-decoy)
# Forges BOTH refs/remotes/origin/main and the local refs/heads/main to the SAME wrong value —
# a self-consistent local state (Precondition 2's pre-existing local-branch-staleness check,
# which only compares refs/heads/<base> against origin/<base>, sees no disagreement between the
# two and would otherwise fire first for an unrelated reason, proving nothing about the forge
# comparison this scenario exists to exercise).
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git update-ref refs/remotes/origin/main "$forged_sha_s12"
  # git reset --hard (not a second update-ref) moves refs/heads/main, the index AND the
  # working tree together, so Precondition 1's dirty-tree check stays clean — a bare
  # update-ref on refs/heads/main alone would leave the working tree pointing at the OLD
  # commit's content, tripping "working tree is not clean" for an unrelated reason.
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git reset --hard "$forged_sha_s12"
) >>"$dest/build.log" 2>&1
stub="$dest-stub"
call_log="$dest-calls"
write_release_gh_stub "$stub" "$call_log" "main" "$real_commit_sha_s12"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "$stub" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when origin/main is forged via update-ref (under a narrowed refspec) and diverges from the forge's tip, got 0"
    continue
  fi
  if ! grep -qF "$forged_sha_s12" "$RT_ERR_FILE" || ! grep -qF "$real_commit_sha_s12" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing both the forged and the forge's real commit sha; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "diverges" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing the divergence refusal; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
if [[ -n "$(find "$call_log" -maxdepth 1 -name '*.json' 2>/dev/null)" ]]; then
  fail "release-tag-parity/$RT_LABEL/no-publish" "refusal must never reach the publish calls; found request files: $(ls "$call_log")"
fi
assert_three_way "$RT_LABEL"

# --- Scenario 13 — remote.origin.fetch narrowed BEFORE the command's own fetch runs, with NO --
# active forgery: the local tracking ref simply stays pinned to a stale commit while a second,
# independent clone legitimately advances the remote. Distinct from Scenario 12 (active
# forgery): here nothing local is written except the narrowing config itself. Mirrors
# check-ship-force-parity.sh's "remote-advanced-lease-mismatch" narrowing technique.
RT_LABEL="forge-commit-diverges-narrowed-fetch"
dest="$WORK/s13"
fixture=$(build_fixture "$dest" "github" "1")
stale_sha_s13=$(git --git-dir="$dest/origin.git" rev-parse main)
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git branch s13-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s13-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git config remote.origin.fetch "+refs/heads/s13-decoy:refs/remotes/origin/s13-decoy"
) >>"$dest/build.log" 2>&1
# A second, independent clone pushes ONE more legitimate commit to main directly on the bare
# remote — the forge (this scenario's `gh` stub) reports THIS as the real tip, while our
# fixture's remote.origin.fetch narrowing means its own internal `git fetch origin --prune`
# never downloads it, so origin/main stays pinned at stale_sha_s13.
other_s13="$dest/other"
(
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git clone -q "$dest/origin.git" "$other_s13"
  cd "$other_s13"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git config user.email "other@trackfw.test"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git config user.name "trackfw release parity (other clone)"
  echo more >>CHANGELOG.md
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git add CHANGELOG.md
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git commit -q -m "another party's legitimate commit"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin main
) >>"$dest/build.log" 2>&1
advanced_sha_s13=$(git --git-dir="$dest/origin.git" rev-parse main)
if [[ "$advanced_sha_s13" == "$stale_sha_s13" ]]; then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard: the other clone's push did not advance the bare remote — scenario proves nothing"
fi
stub="$dest-stub"
call_log="$dest-calls"
write_release_gh_stub "$stub" "$call_log" "main" "$advanced_sha_s13"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "$stub" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit when a narrowed remote.origin.fetch leaves origin/main stale against the forge's advanced tip, got 0"
    continue
  fi
  if ! grep -qF "$stale_sha_s13" "$RT_ERR_FILE" || ! grep -qF "$advanced_sha_s13" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "vacuity guard: stderr missing both the stale local and the forge's advanced commit sha; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
done
if [[ -n "$(find "$call_log" -maxdepth 1 -name '*.json' 2>/dev/null)" ]]; then
  fail "release-tag-parity/$RT_LABEL/no-publish" "refusal must never reach the publish calls; found request files: $(ls "$call_log")"
fi
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 14 — no local tracking ref at all for the forge's default branch
# (forgeLocalSHA == ""): the missing-branch complement of Scenarios 12-13, which forge a
# PRESENT-but-wrong ref. This is the exact consequence ADR-2026-08-19's Emenda 1 declares:
# with no local ref to cross-check against, the command "can publish a commit the local
# clone never saw" — deliberately, not as a bug, per release.go:384-391's own comment. It
# must publish using the FORGE's sha, never refuse and never fall back to a local guess.
#
# Same narrowing technique as Scenario 13 (remote.origin.fetch pinned away from the
# default branch BEFORE the command's own internal `git fetch origin --prune` runs, so
# that fetch cannot re-populate the tracking ref it doesn't own — see the note above
# Scenario 11). Unlike Scenario 13 (ref present but STALE), here the ref is deleted
# outright via `git update-ref -d` so `git rev-parse origin/main` fails entirely, landing
# on forgeLocalSHA == "" rather than a mismatched value.
#
# The stub answers with FORGE_ONLY_SHA_S14 — an empty commit on s14-decoy whose tree is
# byte-identical to main's (so P3/P4 pass after ML-2A) but whose sha differs from the real
# main sha (self-discriminant). ML-2A (ROADMAP-2026-08-21) moved P3/P4 after forge resolution;
# they now call git show <sha>:<path>. A sha with no local objects (the former c0ffee11... fake)
# causes a named refusal per ADR-2026-08-21 — correct behaviour, but wrong outcome for this
# scenario, which exists to prove that an absent local TRACKING REF (not absent git objects)
# must not block publishing. Using the decoy commit preserves both the object-presence needed
# for P3/P4 and the value-discrimination needed to distinguish forge provenance from a
# silently-repopulated local ref: if refspec narrowing ever decays and git fetch repopulates
# refs/remotes/origin/main with the real main sha, forgeLocalSHA (main sha) diverges from
# FORGE_ONLY_SHA_S14 (decoy sha ≠ main sha) and Precondition 6 refuses — flipping the
# "expected exit 0" assertion loudly red instead of silently collapsing into a vacuous pass.
# ---------------------------------------------------------------------------
RT_LABEL="forge-local-ref-absent-success"
dest="$WORK/s14"
fixture=$(build_fixture "$dest" "github" "1")
(
  cd "$fixture"
  # remote.origin.fetch must name a branch that genuinely exists on origin — an unresolvable
  # refspec makes the command's own `git fetch origin --prune` fail outright (a DIFFERENT,
  # unrelated refusal), rather than simply skip refreshing origin/main.
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git branch s14-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s14-decoy
  # Create an empty commit on s14-decoy — same tree as main (P3/P4 read the correct version
  # files and CHANGELOG), but a DISTINCT sha. This restores the self-discriminant: if the
  # refspec narrowing ever stops isolating origin/main and git fetch silently repopulates it,
  # forgeLocalSHA becomes the real main sha, which diverges from the decoy sha (≠ main sha),
  # and Precondition 6's cross-check refuses — flipping "expected exit 0" loudly red.
  # ML-2A (ROADMAP-2026-08-21): P3/P4 now use git show <sha>:<path>; a sha with no local
  # objects (the former c0ffee11... fake) causes a named refusal per ADR-2026-08-21. The
  # decoy commit's objects ARE in the local store, so P3/P4 succeed.
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -q s14-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git commit -q --allow-empty -m "fixture: decoy commit, tree identical to main, distinct sha"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s14-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -q main
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git config remote.origin.fetch "+refs/heads/s14-decoy:refs/remotes/origin/s14-decoy"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git update-ref -d refs/remotes/origin/main
) >>"$dest/build.log" 2>&1
# Derive FORGE_ONLY_SHA_S14 from the decoy commit on the bare remote: distinct from main's
# sha (self-discriminant preserved) but carrying the same tree (P3/P4 read valid content).
FORGE_ONLY_SHA_S14=$(git --git-dir="$dest/origin.git" rev-parse s14-decoy)
# Vacuity guard: prove the tracking ref is genuinely gone before trusting the scenario at
# all — a failure here means the setup didn't reach the state it claims to.
if (cd "$fixture" && env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" HOME="$dest" \
    git rev-parse -q --verify refs/remotes/origin/main >/dev/null 2>&1); then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard: refs/remotes/origin/main still resolves after the deletion — scenario proves nothing"
fi
stub="$dest-stub"
for runtime in go node py; do
  call_log="$dest-calls-$runtime"
  write_release_gh_stub "$stub-$runtime" "$call_log" "main" "$FORGE_ONLY_SHA_S14"
  run_release "$runtime" "$fixture" "$stub-$runtime" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -ne 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected exit 0: an absent local tracking ref must not block publishing from the forge's sha, got $RT_EXIT; stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "commit:     $FORGE_ONLY_SHA_S14" "$RT_OUT_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stdout must echo the forge's commit sha even with no local tracking ref to fall back on; stdout: $(cat "$RT_OUT_FILE")"
    continue
  fi
  if [[ ! -f "$call_log/01-tags-request.json" || ! -f "$call_log/02-refs-request.json" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected both publish calls to be reached; found: $(ls "$call_log" 2>/dev/null)"
    continue
  fi
  # The load-bearing assertion: the tag-object payload's 'object' field — the actual commit
  # the tag will point at — is read from the gh api PAYLOAD the stub received, never from the
  # success message. It must equal FORGE_ONLY_SHA_S14 (the s14-decoy sha), which differs from
  # origin/main's sha (deleted) and from any local ref this clone resolves — proving the value
  # came from the forge, not from a silently-repopulated local ref.
  tags_object_s14=$(json_field "$call_log/01-tags-request.json" object)
  if [[ "$tags_object_s14" != "$FORGE_ONLY_SHA_S14" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "tag-object payload 'object' must equal the forge's commit sha ($FORGE_ONLY_SHA_S14) when no local ref exists to cross-check against, got $tags_object_s14"
    continue
  fi
  refs_sha_s14=$(json_field "$call_log/02-refs-request.json" sha)
  if [[ "$refs_sha_s14" != "$FAKE_TAG_OBJECT_SHA" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "LIGHTWEIGHT-TAG REGRESSION: ref payload 'sha' must equal the tag-object sha ($FAKE_TAG_OBJECT_SHA), got $refs_sha_s14"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 15 — object-absent: forge returns a sha whose git objects do not
# exist locally → all 3 CLIs refuse naming path AND sha, byte-identical.
#
# Technique: same pin+delete as Scenario 14 — pin remote.origin.fetch to a
# guard branch (s15-guard) that genuinely exists on origin, then delete
# refs/remotes/origin/main so the command's own `git fetch origin --prune`
# refreshes only s15-guard and never repopulates origin/main. This makes
# forgeLocalSHA = "" → cross-check skipped → command proceeds to P3, calls
# `git show FAKE_ABSENT_SHA:<path>` → object absent → refuses naming path and
# sha (ADR-2026-08-21: "objeto ausente → recusar nomeando o quê falta").
#
# FAKE_ABSENT_SHA (40 × 'a') is syntactically valid as a sha40 but will never
# appear in a freshly-initialized local git object store — proven by vacuity
# guard 2 below.  No gh api POST may happen (no-publish guard).
# ---------------------------------------------------------------------------
FAKE_ABSENT_SHA="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
RT_LABEL="object-absent"
dest="$WORK/s15"
fixture=$(build_fixture "$dest" "github" "1")
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git branch s15-guard
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s15-guard
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git config remote.origin.fetch "+refs/heads/s15-guard:refs/remotes/origin/s15-guard"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git update-ref -d refs/remotes/origin/main
) >>"$dest/build.log" 2>&1
# Vacuity guard 1: origin/main tracking ref must be gone
if (cd "$fixture" && env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" HOME="$dest" \
    git rev-parse -q --verify refs/remotes/origin/main >/dev/null 2>&1); then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard: refs/remotes/origin/main still resolves after deletion — scenario proves nothing"
fi
# Vacuity guard 2: the fake sha must not exist as a local git object
if (cd "$fixture" && git cat-file -e "$FAKE_ABSENT_SHA" 2>/dev/null); then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard: $FAKE_ABSENT_SHA exists as a local git object — scenario proves nothing"
fi
stub="$dest-stub"
call_log="$dest-calls"
write_release_gh_stub "$stub" "$call_log" "main" "$FAKE_ABSENT_SHA"
for runtime in go node py; do
  run_release "$runtime" "$fixture" "$stub" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -eq 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected non-zero exit: git object absent for sha $FAKE_ABSENT_SHA, got exit 0"
    continue
  fi
  if ! grep -qF "could not read" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stderr must name the missing path (expected 'could not read'); stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if ! grep -qF "$FAKE_ABSENT_SHA" "$RT_ERR_FILE"; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "stderr must name the absent sha ($FAKE_ABSENT_SHA); stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  # No-publish guard: the command must refuse before reaching any gh api POST
  if [[ -f "$call_log/01-tags-request.json" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "no-publish guard: tag POST was reached despite absent git object"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 16 — content-from-commit-provenance: covers BOTH the legitimate
# PR-bump flow (case 2) and content provenance proof (case 3).
#
# Two-axis fixture (advisor recommendation — both anchored reads discriminated):
#   HEAD (local main): version 9.9.7 in all 5 version files; CHANGELOG has
#     ## [9.9.9] section with "- head-only" (pre-merge state).
#   Decoy commit (objectSHA / forge): version 9.9.9 in all 5 version files;
#     CHANGELOG has ## [9.9.9] section with "- forge-only" (post-merge state).
#
# Real binary reads everything from objectSHA (decoy):
#   P3 → version 9.9.9 == RELEASE_VERSION ✓
#   P4 → ## [9.9.9] section found ✓; message contains "forge-only" ✓
#   Publishes → exit 0 ✓
#
# The two-axis design makes EACH anchored read independently falsifiable (Cenário 87):
#   sabotage CHANGELOG read (objectSHA → "HEAD"): P4 still passes (## [9.9.9]
#     found in HEAD too) but message = "head-only" → provenance assertion fires.
#   sabotage version read (objectSHA → "HEAD"): P3 reads 9.9.7 ≠ 9.9.9 → exit
#     non-zero → exit-code assertion fires.
#
# Same pin+delete technique as Scenarios 14/15: forge stub returns the decoy
# sha; cross-check is skipped (refs/remotes/origin/main deleted); P1 passes
# (working tree matches HEAD); P2 skipped (origin/main gone).
#
# Per-runtime call logs (like Scenario 14) because this is a success scenario
# that makes POST calls.
# ---------------------------------------------------------------------------
RT_LABEL="content-from-commit-provenance"
dest="$WORK/s16"
fixture=$(build_fixture "$dest" "github" "1")
# Step 1: mutate local main to version 9.9.7 + CHANGELOG with head-only marker.
# HEAD carries ## [9.9.9] section (not ## [9.9.7]) so that the sabotaged binary
# (reading CHANGELOG from HEAD) still passes P4 — only the section BODY differs.
write_version_files "$fixture" "9.9.7"
cat >"$fixture/CHANGELOG.md" <<'S16CLEOF'
# Changelog

## [9.9.9] - 2026-08-19

### Added
- head-only
S16CLEOF
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git add -A
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git commit -q -m "fixture: local main at 9.9.7 with head-only CHANGELOG"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin main
) >>"$dest/build.log" 2>&1
# Step 2: create s16-decoy with version 9.9.9 + CHANGELOG with forge-only marker.
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git branch s16-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -q s16-decoy
) >>"$dest/build.log" 2>&1
write_version_files "$fixture" "9.9.9"
cat >"$fixture/CHANGELOG.md" <<'S16CLEOF'
# Changelog

## [9.9.9] - 2026-08-19

### Added
- forge-only
S16CLEOF
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git add -A
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git commit -q -m "fixture: forge commit at 9.9.9 with forge-only CHANGELOG"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s16-decoy
  # Switch back to main, pin refspec to s16-decoy only, delete origin/main tracking ref
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -q main
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git config remote.origin.fetch "+refs/heads/s16-decoy:refs/remotes/origin/s16-decoy"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git update-ref -d refs/remotes/origin/main
) >>"$dest/build.log" 2>&1
FORGE_ONLY_SHA_S16=$(git --git-dir="$dest/origin.git" rev-parse s16-decoy)
# Vacuity guard 1: origin/main tracking ref must be gone
if (cd "$fixture" && env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" HOME="$dest" \
    git rev-parse -q --verify refs/remotes/origin/main >/dev/null 2>&1); then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard: refs/remotes/origin/main still resolves — cross-check would not be skipped"
fi
# Vacuity guard 2: working-tree version.go must have 9.9.7, not 9.9.9 — proves
# that version came from the forge commit, not the local working tree.
if ! grep -qF '= "9.9.7"' "$fixture/internal/version/version.go"; then
  fail "release-tag-parity/$RT_LABEL/setup" 'vacuity guard: working-tree version.go does not contain "9.9.7" — two-axis fixture broken'
fi
for runtime in go node py; do
  call_log="$dest-calls-$runtime"
  stub="$dest-stub-$runtime"
  write_release_gh_stub "$stub" "$call_log" "main" "$FORGE_ONLY_SHA_S16"
  run_release "$runtime" "$fixture" "$stub" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -ne 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected exit 0 for legitimate PR-bump flow (version 9.9.9 on forge commit, 9.9.7 in local tree); stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if [[ ! -f "$call_log/01-tags-request.json" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected tag POST to be reached; found: $(ls "$call_log" 2>/dev/null)"
    continue
  fi
  # Provenance assertion: tag message must come from the FORGE COMMIT's CHANGELOG
  # ("forge-only"), not from the working-tree CHANGELOG ("head-only"). Both carry
  # ## [9.9.9] section, so exit code alone cannot discriminate — the section BODY
  # is the only observable difference. This is the assertion Cenário 87 in
  # check-gates-falsify.sh targets: the sabotaged binary reads CHANGELOG from
  # "HEAD" instead of objectSHA → P4 still passes → exit 0, but message =
  # "head-only" → this assertion fires.
  tags_message_s16=$(json_field "$call_log/01-tags-request.json" message)
  if ! echo "$tags_message_s16" | grep -qF 'forge-only'; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "provenance: tag message must contain 'forge-only' (from forge commit CHANGELOG); got: $tags_message_s16"
    continue
  fi
  if echo "$tags_message_s16" | grep -qF 'head-only'; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "provenance: tag message must NOT contain 'head-only' (from working-tree CHANGELOG); got: $tags_message_s16"
    continue
  fi
  refs_sha_s16=$(json_field "$call_log/02-refs-request.json" sha)
  if [[ "$refs_sha_s16" != "$FAKE_TAG_OBJECT_SHA" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "LIGHTWEIGHT-TAG REGRESSION: ref payload 'sha' must equal the tag-object sha ($FAKE_TAG_OBJECT_SHA), got $refs_sha_s16"
    continue
  fi
done
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Scenario 17 — refs-replace-bypass: proves --no-replace-objects blocks the
# refs/replace/ object-identity substitution attack.
#
# Attack: an adversary writes .git/refs/replace/<forge-sha> → <attacker-sha>
# as a raw file (no git command needed — bypasses any branch guard). Without
# --no-replace-objects, `git show <forge-sha>:<path>` follows the redirect
# and returns the attacker's content instead of the legitimate forge content.
# With --no-replace-objects (first arg after git), git skips the refs/replace/
# layer entirely and reads the original object.
#
# Three-axis fixture:
#   HEAD (local main): version 9.9.7; CHANGELOG has ## [9.9.9] section with
#     "- head-only". Same design as Scenario 16: HEAD must carry ## [9.9.9]
#     so a sabotaged binary reading CHANGELOG from HEAD still passes P4
#     exit-code, but section body "head-only" fires the provenance assertion.
#   Forge commit (s17-decoy): version 9.9.9; CHANGELOG has "- forge-only".
#     This is the LEGITIMATE commit the release was supposed to tag.
#   Attacker commit (LOCAL ONLY, s17-attacker): version 9.9.9; CHANGELOG has
#     "- refs-replace-forged". Never pushed to origin — only its SHA is needed
#     to populate the replace ref.
#
# Replace ref written as file (not via git replace, because the branch guard
# blocks compound git commands in this environment):
#   mkdir -p .git/refs/replace
#   printf '%s\n' <attacker-sha> > .git/refs/replace/<forge-sha>
#
# Three vacuity guards:
#   V1: origin/main tracking ref gone — cross-check is skipped (same as S14-16)
#   V2: attack IS genuine — raw `git show <forge-sha>:CHANGELOG.md` (no flag)
#       returns "refs-replace-forged". If V2 fails, the replace ref is broken.
#   V3: fix works — `git --no-replace-objects show <forge-sha>:CHANGELOG.md`
#       returns "forge-only". If V3 fails, the flag does nothing.
# Post-run guard: .git/refs/replace/<forge-sha> still exists and still
#   redirects after all three CLI invocations, proving the fix works by
#   suppressing a live redirect, not by a vacated attack vector.
#
# Provenance assertions (per-runtime, same design as Scenario 16):
#   tags_message must contain "forge-only" (fix reads legitimate forge content)
#   tags_message must NOT contain "refs-replace-forged" (attacker content blocked)
#
# Per-runtime call logs (success scenario that reaches gh api POST calls).
#
# Scenario 158 in check-gates-falsify.sh (P4) targets this scenario: it
# corrupts the Go binary to remove the --no-replace-objects flag and proves
# the gate turns red (provenance assertion fires: "refs-replace-forged" in
# message instead of "forge-only"). The per-runtime provenance assertion is
# what makes this correlated-revert-proof: if all three stacks revert the
# flag, each runtime's message contains "refs-replace-forged", which fires
# individually — assert_three_way catches the case where only one reverts.
# ---------------------------------------------------------------------------
RT_LABEL="refs-replace-bypass"
dest="$WORK/s17"
fixture=$(build_fixture "$dest" "github" "1")
# Step 1: mutate local main to version 9.9.7 + CHANGELOG with head-only marker.
write_version_files "$fixture" "9.9.7"
cat >"$fixture/CHANGELOG.md" <<'S17CLEOF'
# Changelog

## [9.9.9] - 2026-08-21

### Added
- head-only
S17CLEOF
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git add -A
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git commit -q -m "fixture: local main at 9.9.7 with head-only CHANGELOG"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin main
) >>"$dest/build.log" 2>&1
# Step 2: create s17-decoy with version 9.9.9 + CHANGELOG with forge-only marker (PUSHED).
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git branch s17-decoy
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -q s17-decoy
) >>"$dest/build.log" 2>&1
write_version_files "$fixture" "9.9.9"
cat >"$fixture/CHANGELOG.md" <<'S17CLEOF'
# Changelog

## [9.9.9] - 2026-08-21

### Added
- forge-only
S17CLEOF
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git add -A
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git commit -q -m "fixture: forge commit at 9.9.9 with forge-only CHANGELOG"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git push -q origin s17-decoy
) >>"$dest/build.log" 2>&1
FORGE_ONLY_SHA_S17=$(git --git-dir="$dest/origin.git" rev-parse s17-decoy)
# Step 3: create s17-attacker (LOCAL ONLY — never pushed) with "refs-replace-forged" CHANGELOG.
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -b s17-attacker s17-decoy
) >>"$dest/build.log" 2>&1
cat >"$fixture/CHANGELOG.md" <<'S17CLEOF'
# Changelog

## [9.9.9] - 2026-08-21

### Added
- refs-replace-forged
S17CLEOF
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git add -A
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git commit -q -m "attacker: forged CHANGELOG via refs-replace"
) >>"$dest/build.log" 2>&1
ATTACKER_SHA_S17=$(env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "HOME=$dest" \
  git -C "$fixture" rev-parse HEAD)
# Step 4: return to main, narrow refspec to s17-decoy only, delete origin/main tracking ref.
(
  cd "$fixture"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "GIT_TERMINAL_PROMPT=0" "HOME=$dest" \
    git checkout -q main
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git config remote.origin.fetch "+refs/heads/s17-decoy:refs/remotes/origin/s17-decoy"
  env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
    git update-ref -d refs/remotes/origin/main
) >>"$dest/build.log" 2>&1
# Step 5: write replace ref as file (file write, not git replace — bypasses branch guard).
mkdir -p "$fixture/.git/refs/replace"
printf '%s\n' "$ATTACKER_SHA_S17" >"$fixture/.git/refs/replace/$FORGE_ONLY_SHA_S17"
# Vacuity guard V1: origin/main tracking ref must be gone (same as Scenario 16).
if (cd "$fixture" && env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" HOME="$dest" \
    git rev-parse -q --verify refs/remotes/origin/main >/dev/null 2>&1); then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard V1: refs/remotes/origin/main still resolves — cross-check would not be skipped"
fi
# Vacuity guard V2: attack IS genuine — raw git show (no flag) must return "refs-replace-forged".
# If this fails, the replace ref is broken and the scenario proves nothing.
_v2_content=$(env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
  git -C "$fixture" show "${FORGE_ONLY_SHA_S17}:CHANGELOG.md" 2>/dev/null || true)
if ! echo "$_v2_content" | grep -qF "refs-replace-forged"; then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard V2: raw git show (no flag) returned '$_v2_content' — replace ref not working, attack not genuine"
fi
# Vacuity guard V3: fix works — git --no-replace-objects show must return "forge-only".
# If this fails, --no-replace-objects does not suppress the redirect on this git version.
_v3_content=$(env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
  git -C "$fixture" --no-replace-objects show "${FORGE_ONLY_SHA_S17}:CHANGELOG.md" 2>/dev/null || true)
if ! echo "$_v3_content" | grep -qF "forge-only"; then
  fail "release-tag-parity/$RT_LABEL/setup" "vacuity guard V3: git --no-replace-objects show returned '$_v3_content' — flag does not suppress replace redirect"
fi
# Working-tree version check: main must have 9.9.7 (not 9.9.9) — proves version came from forge commit.
if ! grep -qF '= "9.9.7"' "$fixture/internal/version/version.go"; then
  fail "release-tag-parity/$RT_LABEL/setup" 'vacuity guard: working-tree version.go does not contain "9.9.7" — three-axis fixture broken'
fi
for runtime in go node py; do
  call_log="$dest-calls-$runtime"
  stub="$dest-stub-$runtime"
  write_release_gh_stub "$stub" "$call_log" "main" "$FORGE_ONLY_SHA_S17"
  run_release "$runtime" "$fixture" "$stub" "$dest" "$RELEASE_VERSION"
  echo "$RT_EXIT" >"$WORK/$RT_LABEL.$runtime.exit"
  if [[ "$RT_EXIT" -ne 0 ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected exit 0 (--no-replace-objects blocks redirect, forge-only content read, version 9.9.9 matches); stderr: $(cat "$RT_ERR_FILE")"
    continue
  fi
  if [[ ! -f "$call_log/01-tags-request.json" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "expected tag POST to be reached; found: $(ls "$call_log" 2>/dev/null)"
    continue
  fi
  # Provenance assertion: tag message must come from the FORGE COMMIT's CHANGELOG
  # ("forge-only"), not attacker's CHANGELOG ("refs-replace-forged"). Both carry
  # ## [9.9.9] section, so exit code alone cannot discriminate — the section BODY
  # is the only observable difference. The per-runtime provenance assertion here is
  # also what makes the scenario correlated-revert-proof: if all three stacks drop
  # --no-replace-objects, each runtime fires independently; assert_three_way catches
  # the single-stack revert. (Scenario 158 in check-gates-falsify.sh proves P4.)
  tags_message_s17=$(json_field "$call_log/01-tags-request.json" message)
  if ! echo "$tags_message_s17" | grep -qF 'forge-only'; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "provenance: tag message must contain 'forge-only' (--no-replace-objects reads forge commit); got: $tags_message_s17"
    continue
  fi
  if echo "$tags_message_s17" | grep -qF 'refs-replace-forged'; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "provenance: tag message must NOT contain 'refs-replace-forged' (attacker content must be blocked); got: $tags_message_s17"
    continue
  fi
  refs_sha_s17=$(json_field "$call_log/02-refs-request.json" sha)
  if [[ "$refs_sha_s17" != "$FAKE_TAG_OBJECT_SHA" ]]; then
    fail "release-tag-parity/$RT_LABEL/$runtime" "LIGHTWEIGHT-TAG REGRESSION: ref payload 'sha' must equal the tag-object sha ($FAKE_TAG_OBJECT_SHA), got $refs_sha_s17"
    continue
  fi
done
# Post-run guard: .git/refs/replace/<forge-sha> must still exist and still redirect
# after all three CLI invocations. This proves --no-replace-objects suppresses a LIVE
# redirect, not that the redirect was already gone when the CLIs ran (silent vacuity).
if [[ ! -f "$fixture/.git/refs/replace/$FORGE_ONLY_SHA_S17" ]]; then
  fail "release-tag-parity/$RT_LABEL/post-run" "post-run guard: .git/refs/replace/$FORGE_ONLY_SHA_S17 was removed during CLI invocations — scenario result may be vacuous"
fi
_postrun_ref_content=$(cat "$fixture/.git/refs/replace/$FORGE_ONLY_SHA_S17")
if ! echo "$_postrun_ref_content" | grep -qF "$ATTACKER_SHA_S17"; then
  fail "release-tag-parity/$RT_LABEL/post-run" "post-run guard: replace ref content changed (expected $ATTACKER_SHA_S17, got $_postrun_ref_content)"
fi
_postrun_content=$(env "GIT_CONFIG_GLOBAL=$dest/empty-gitconfig" "GIT_CONFIG_SYSTEM=/dev/null" "HOME=$dest" \
  git -C "$fixture" show "${FORGE_ONLY_SHA_S17}:CHANGELOG.md" 2>/dev/null || true)
if ! echo "$_postrun_content" | grep -qF "refs-replace-forged"; then
  fail "release-tag-parity/$RT_LABEL/post-run" "post-run guard: raw git show (no flag) no longer returns 'refs-replace-forged' after CLI invocations — attack vector vanished, scenario may be vacuous"
fi
assert_three_way "$RT_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-release-tag-parity.sh scenarios passed."
else
  echo "check-release-tag-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
