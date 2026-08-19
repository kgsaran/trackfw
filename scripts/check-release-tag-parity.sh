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
# write_release_gh_stub DIR CALL_LOG — a `gh` stub that answers exactly the two `gh api` calls
# release tag makes (POST git/tags, POST git/refs), recording each request body to CALL_LOG in
# call order, and returning a tag-object sha DELIBERATELY DIFFERENT from any real commit sha.
# ---------------------------------------------------------------------------
write_release_gh_stub() {
  local dir=$1 call_log=$2
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
  local run_path="$BASE_PATH"
  if [[ -n "$path_prefix" ]]; then
    run_path="$path_prefix:$BASE_PATH"
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
  for runtime in go node py; do
    run_release "$runtime" "$fixture" "" "$dest" "$RELEASE_VERSION"
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
for runtime in go node py; do
  run_release "$runtime" "$fixture" "" "$dest" "$RELEASE_VERSION"
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
stub="$dest-stub"
call_log="$dest-calls"
write_release_gh_stub "$stub" "$call_log"
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
  write_release_gh_stub "$stub-$runtime" "$call_log"
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
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-release-tag-parity.sh scenarios passed."
else
  echo "check-release-tag-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
