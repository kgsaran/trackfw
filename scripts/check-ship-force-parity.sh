#!/usr/bin/env bash
# check-ship-force-parity.sh — proves `trackfw ship --force-with-lease` behaves byte-for-byte
# identically in Go, Node.js, and Python (ML-1B, ROADMAP-2026-08-19-caminho-governado-para-
# push-forcado-e-tag-de-release.md).
#
# The four paths, exactly as ML-1A's audit verified against a real fixture:
#
#   sem CLI de forge        -> RECUSA  "requires a forge CLI (gh, glab, or az) to confirm ..."
#   forge, zero PR           -> RECUSA  "has no open pull/merge request. Open the PR/MR first"
#   forge, nao verificavel   -> RECUSA  "could not verify ... Refusing rather than risking ..."
#   forge, PR aberto         -> EMPURRA, and the remote history is genuinely rewritten
#
# These are THREE distinct refusal classes, never two — conflating "no PR" with "cannot verify"
# would make a `gh` auth failure look like "no PR exists", nudging the caller to open a PR that
# already exists. Every scenario below diffs the THREE real outputs (stdout, stderr, exit code)
# byte-for-byte across the 3 CLIs — testing per-stack never closes this AC (this series has
# proven that three times already).
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
#   5. Every fixture carries valid governance (or uses a chore/ branch, which `ship` already
#      exempts from the REQ+roadmap gate) — force-with-lease's own gate runs before governance
#      would even matter for chore/, but feat/ would additionally need REQ+roadmap in wip/ with
#      the roadmap referencing the REQ in its body, not just the frontmatter `req:` field.
#   6. Success is proved by the remote SHA changing (git --git-dir=<bare> rev-parse <branch>
#      before/after), never by the printed message alone.
#
# Fifth scenario — the semantic discriminant, stronger than inspecting the push argv string:
# after the "PR open" fixture is built, a SECOND clone pushes one more legitimate commit to the
# same branch on the shared remote. Our clone's remote-tracking ref for that branch is pinned
# stale on purpose (remote.origin.fetch restricted to main only), so `ship`'s own internal
# `git fetch origin --prune` (Step 3) never learns about the other clone's push. This is exactly
# the situation --force-with-lease exists to protect against: with the correct flag, `git push
# --force-with-lease` refuses because the remote moved past what our clone last recorded, and
# the other clone's commit survives untouched. A raw `--force` push does not consult that
# recorded state at all — it destroys the other clone's commit unconditionally. This is what
# scripts/check-gates-falsify.sh's P4 scenario sabotages (single-literal delta on
# internal/commands/ship.go's push-arg construction, isolated Go copy) and this scenario is what
# catches it: the sabotaged binary pushes successfully and the other clone's commit disappears
# from the remote.
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
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-ship-force-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-ship-parity.sh / check-branch-prune-parity.sh.
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
  echo "check-ship-force-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-ship-force-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

REAL_NODE=$(command -v node || true)
REAL_PYTHON3=$(command -v python3 || true)
if [[ -z "$REAL_NODE" ]]; then
  echo "check-ship-force-parity: node not found in PATH" >&2
  exit 1
fi
if [[ -z "$REAL_PYTHON3" ]]; then
  echo "check-ship-force-parity: python3 not found in PATH" >&2
  exit 1
fi

# git must be detected before RUNTIME_BIN so GIT_BIN_DIR is available for BASE_PATH. An unset
# GIT_BIN_DIR would produce an empty PATH component ("::"), making CWD searchable for git — a
# silent false-pass vector that the vacuity guard exists to prevent.
REAL_GIT=$(command -v git || true)
if [[ -z "$REAL_GIT" ]]; then
  echo "check-ship-force-parity: git not found in PATH" >&2
  exit 1
fi
# Named GIT_BIN_DIR, not GIT_DIR: git treats GIT_DIR as a reserved environment variable —
# when exported it overrides the repository path for every subsequent git call, silently
# redirecting them to an arbitrary directory. This variable is never exported and must NOT be
# renamed back to GIT_DIR (see check-gates-falsify.sh credential-guard-git-env-bypass scenario,
# where GIT_DIR+GIT_WORK_TREE diverts a git -C call to a decoy repository).
GIT_BIN_DIR="$(dirname "$REAL_GIT")"

# runtimebin/ carries ONLY the interpreters the three CLIs need, symlinked from their real
# location — the scenario-controlled PATH built below never inherits the caller's PATH, so a
# real gh/az/glab installed on the host (both present on the machine this gate was authored on)
# can never leak into a scenario that must see none.
RUNTIME_BIN="$WORK/runtimebin"
mkdir -p "$RUNTIME_BIN"
ln -s "$REAL_NODE" "$RUNTIME_BIN/node"
ln -s "$REAL_PYTHON3" "$RUNTIME_BIN/python3"

# BASE_PATH: git + coreutils only, plus the two interpreters above. No gh/glab/az anywhere
# unless a scenario explicitly prepends its own stub directory.
# GIT_BIN_DIR is prepended so native child processes (Go exec.Command, Python subprocess.run) find
# git.exe via PATHEXT — on Git for Windows, git.exe lives in /clangarm64/bin (ARM64) or
# /mingw64/bin (x64), neither of which is /usr/bin or /bin (measured: ML-R2a run 34406101512).
# Mechanism: directory prepended (not single placed file) because /usr/bin is already in
# BASE_PATH; /clangarm64/bin and /mingw64/bin do not contain forge CLIs, sh, or bash (measured:
# ML-R2a). The alias /bin == /usr/bin (cygpath proves these map to the same Windows path) means
# the original two entries were one entry anyway.
BASE_PATH="$RUNTIME_BIN:$GIT_BIN_DIR:/usr/bin:/bin"

# Never let an inherited TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make the forge adapter report
# "unavailable" regardless of PATH — CI's `make parity` step sets this env var for every gate in
# the target (it exists so check-ship-parity.sh can force the no-forge-CLI path deterministically
# there), and it leaks into every script `make parity` runs afterwards, including this one. Left
# unset here, it collapses scenarios (b)/(c)/(d) — which all stub `gh` in PATH and expect it to
# be detected — onto the same "no forge CLI" refusal as scenario (a), for the wrong reason. Same
# fix already applied in check-release-tag-parity.sh's sibling gate.
unset TRACKFW_DISABLE_EXTERNAL_COMMANDS || true

# NO_FORGE_PATH — used ONLY by scenario (a) below, which must prove genuine absence of a forge
# CLI via defaultAvailFn's real exec.LookPath(name) call (never via
# TRACKFW_DISABLE_EXTERNAL_COMMANDS=1, which the other scenarios need OFF anyway and which would
# skip that code path entirely instead of exercising it). BASE_PATH's "/usr/bin:/bin" is NOT safe
# for this: the GitHub Actions ubuntu-latest runner ships a real `gh` at /usr/bin/gh (ML-6B), so a
# scenario meant to see no forge CLI would see a real one there. GIT_ONLY_BIN carries nothing but
# a symlink to the real `git` this host resolves — no coreutils, no /usr/bin, no /bin — because
# nothing this scenario exercises (git plumbing over the local bare-repo file transport, plus the
# node/python3 interpreters already isolated in RUNTIME_BIN) needs anything else on PATH: the
# product itself only ever execs "git" and the resolved forge CLI name (grep -rn 'exec.Command\|
# spawnSync\|subprocess.run' confirms this across all 3 stacks).
# REAL_GIT and GIT_BIN_DIR are computed above (before BASE_PATH) — do not re-detect here.
#
# Forma 2 fix — NO_FORGE_PATH: two mechanisms, one per platform, declared here:
#
#   Windows (GfW, detected by ${REAL_GIT}.exe sibling): git.exe is a DLL-dependent wrapper.
#   Placing a single git.exe copy or hardlink in a new directory fails with STATUS_DLL_NOT_FOUND
#   (0xC0000135) because the DLLs live alongside git.exe in its installation directory and
#   Windows DLL search starts in the executable's own directory. The only correct fix is to add
#   GIT_BIN_DIR itself to NO_FORGE_PATH. /clangarm64/bin (ARM64) and /mingw64/bin (x64) contain no
#   forge CLIs (gh/glab/az), sh, or bash — measured in ML-R2a and confirmed by the vacuity
#   guard below — so the no-forge discriminant is fully preserved.
#
#   POSIX (no .exe sibling): git is typically a standalone binary; a single symlink in an
#   isolated GIT_ONLY_BIN is sufficient and keeps /usr/bin out of NO_FORGE_PATH — critical
#   because ubuntu-latest runners carry a real gh at /usr/bin/gh (the CI failure that
#   motivated GIT_ONLY_BIN in the first place, ML-6B). On POSIX the symlink is seen by native
#   child processes (exec(2) follows it transparently), so this form has no broken-symlink risk.
if [[ -f "${REAL_GIT}.exe" ]]; then
  NO_FORGE_PATH="$RUNTIME_BIN:$GIT_BIN_DIR"
else
  GIT_ONLY_BIN="$WORK/gitonlybin"
  mkdir -p "$GIT_ONLY_BIN"
  ln -s "$REAL_GIT" "$GIT_ONLY_BIN/git"
  NO_FORGE_PATH="$RUNTIME_BIN:$GIT_ONLY_BIN"
fi

# Non-vacuity guard — fails BEFORE any scenario runs if gh/glab/az somehow resolve on
# NO_FORGE_PATH, or if git does NOT resolve on it. A "no forge CLI" scenario that runs against a
# PATH secretly still carrying a forge CLI would pass for the wrong reason — exactly the class of
# bug this ML exists to fix (see the "Correção do meu próprio erro de auditoria" note in the
# roadmap: the CI-observed failure was a real `gh` reachable via /usr/bin, not a hypothesis).
for cli in gh glab az; do
  if resolved=$(PATH="$NO_FORGE_PATH" command -v "$cli" 2>/dev/null); then
    echo "check-ship-force-parity: vacuity guard failed — '$cli' resolves on NO_FORGE_PATH ($NO_FORGE_PATH) at $resolved; the no-forge-cli scenario would prove nothing" >&2
    exit 1
  fi
done
# git resolution: use native child process (python3, already in RUNTIME_BIN ⊂ NO_FORGE_PATH)
# instead of bash's command -v — the bug class this ML corrects is a git that bash resolves via
# MSYS symlink but CreateProcess cannot (ML-R2c Forma 2). python3's subprocess.run uses the
# same CreateProcess + PATHEXT lookup the product uses. Reconciliation: this guard asserts that
# a native child, not bash, resolves git on the path the no-forge scenario uses.
if ! PATH="$NO_FORGE_PATH" python3 -c \
    'import subprocess,sys; sys.exit(subprocess.run(["git","--version"],capture_output=True).returncode)' \
    2>/dev/null; then
  echo "check-ship-force-parity: vacuity guard failed — git does not resolve on NO_FORGE_PATH ($NO_FORGE_PATH) for a native child process" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Windows compatibility: gh.exe shim — defined HERE (before the vacuity guard)
# so _build_gh_stub_shim_once is callable on Windows without a forward-reference
# "command not found". On POSIX the guard block is always skipped
# ([[ -f "${REAL_GIT}.exe" ]] is false), so ordering is irrelevant there.
# (See full rationale in check-release-tag-parity.sh)
# ---------------------------------------------------------------------------
_GH_STUB_SHIM=""  # POSIX path to the compiled gh.exe (empty on POSIX)

_build_gh_stub_shim_once() {
  [[ -n "$_GH_STUB_SHIM" ]] && return 0     # already built this run
  [[ ! -f "${REAL_GIT}.exe" ]] && return 0  # not Windows — no-op

  # Capture real bash path at gate build time — the gate runs inside bash and
  # knows the exact executable. Injecting it avoids guessed constants (assumed
  # layout was the root cause of ML-R2c; const gitBash is that defect reintroduced).
  local _real_bash
  _real_bash="$(command -v bash || true)"
  if [[ -z "$_real_bash" ]]; then
    echo "gh-stub shim: bash not found on PATH — cannot build gh.exe shim (Windows requires it)" >&2
    exit 1
  fi
  # cygpath -w converts POSIX path to Windows path; available on all Git for Windows (MSYS2).
  local _win_bash
  _win_bash="$(cygpath -w "$_real_bash" 2>/dev/null || echo "$_real_bash")"

  local _src_dir
  _src_dir=$(mktemp -d)
  local _out="$WORK/gh-stub-shim.exe"

  # Write the captured bash path as a Go double-quoted string literal (backslashes escaped).
  local _win_bash_esc="${_win_bash//\\/\\\\}"
  printf 'package main\n\nconst bashFallback = "%s"\n' "$_win_bash_esc" > "$_src_dir/bash_path.go"

  cat >"$_src_dir/main.go" <<'GOEOF'
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// findBash tries PATH first (works when /usr/bin is in PATH), then the
// real bash path captured by the gate at shim build time (bashFallback,
// injected via bash_path.go — never a guessed constant; assumed-layout
// was the root cause of ML-R2c).
func findBash() string {
	for _, n := range []string{"bash.exe", "bash"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	if bashFallback != "" {
		if _, err := os.Stat(bashFallback); err == nil {
			return bashFallback
		}
	}
	return ""
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stub := filepath.Join(filepath.Dir(exe), "gh")
	bash := findBash()
	if bash == "" {
		fmt.Fprintln(os.Stderr, "gh-stub shim: bash not found on PATH or injected fallback path")
		os.Exit(1)
	}
	args := append([]string{stub}, os.Args[1:]...)
	cmd := exec.Command(bash, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Suppress MSYS brace/path conversion at the shim→bash frontier (run 34468562798,
	// P14-I/P14-J). P14-F: {owner}/{repo} keys intact in shim RECV[2] but already gone
	// in bash child's os.Args[1] — the rewrite happens at the MSYS bash entry point, not
	// at any quoting layer of the Go caller. P14-I confirmed the loss survives when Go
	// calls bash.exe directly (no shim), ruling out shim quoting as the cause. P14-J
	// confirmed these three vars suppress the conversion end-to-end.
	cmd.Env = append(os.Environ(),
		"MSYS=noglob",
		"MSYS_NO_PATHCONV=1",
		"MSYS2_ARG_CONV_EXCL=*",
	)
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
GOEOF

  printf 'module ghstubshim\ngo 1.21\n' > "$_src_dir/go.mod"
  touch "$_src_dir/go.sum"
  # Capture build output — on Windows, build failure is fatal: bash stub alone is not
  # found by native processes via PATHEXT, so the shim is required, not optional.
  local _build_out
  if ! _build_out=$(cd "$_src_dir" && go build -o "$_out" . 2>&1); then
    echo "gh-stub shim: 'go build' failed (Windows: gh.exe shim is required; bash stub alone is not found by native processes via PATHEXT)" >&2
    [[ -n "$_build_out" ]] && echo "$_build_out" >&2
    rm -rf "$_src_dir"
    exit 1
  fi
  _GH_STUB_SHIM="$_out"
  rm -rf "$_src_dir"
}

# ML-R2b1 (corrected): gh vacuity guard — execute, not just resolve.
# shutil.which is a resolver (same lesson as command -v vs CreateProcess — the root of ML-R2c).
# subprocess.run proves the full CreateProcess+PATHEXT chain the product uses.
#   (a) gh must NOT execute on NO_FORGE_PATH (no-forge scenario is meaningful)
#   (b) gh.exe shim must execute AND return a marker (end-to-end proof, not just resolution)
# Guard is Windows-only (POSIX: command -v and subprocess agree; no PATHEXT divergence).
if [[ -f "${REAL_GIT}.exe" ]]; then
  # (a) Execute, not resolve: prove gh is NOT executable on NO_FORGE_PATH.
  # Exceptions cover WinError 2 (not found) and WinError 267 (ENOTDIR variant).
  if PATH="$NO_FORGE_PATH" python3 -c '
import subprocess, sys
try:
    subprocess.run(["gh", "--version"], capture_output=True)
    sys.exit(0)  # gh ran — it was found
except (FileNotFoundError, OSError):
    sys.exit(1)  # not found — guard passes
'; then
    echo "check-ship-force-parity: vacuity guard failed — 'gh' executes on NO_FORGE_PATH ($NO_FORGE_PATH) via native subprocess; the no-forge-cli scenario would prove nothing" >&2
    exit 1
  fi
  # (b) Execute and verify marker: probe PATH is bash-free (probe dir + RUNTIME_BIN only) so
  # the shim's injected bashFallback is the load-bearing branch (exec.LookPath("bash.exe")
  # misses on this PATH), proving the captured-bash-path correction is actually exercised.
  _build_gh_stub_shim_once
  if [[ -n "$_GH_STUB_SHIM" ]]; then
    _PROBE_DIR="$WORK/gh-vacuity-probe"
    mkdir -p "$_PROBE_DIR"
    printf '#!/bin/bash\necho GH_SHIM_OK\n' > "$_PROBE_DIR/gh"
    chmod +x "$_PROBE_DIR/gh"
    cp "$_GH_STUB_SHIM" "$_PROBE_DIR/gh.exe"
    if ! PATH="$_PROBE_DIR:$RUNTIME_BIN" python3 -c '
import subprocess, sys
try:
    r = subprocess.run(["gh", "probe"], capture_output=True, text=True)
    sys.exit(0 if r.returncode == 0 and "GH_SHIM_OK" in r.stdout else 1)
except (FileNotFoundError, OSError):
    sys.exit(1)
'; then
      echo "check-ship-force-parity: vacuity guard failed — gh.exe shim does NOT execute and return marker via native subprocess in probe dir ($WORK/gh-vacuity-probe); the stub fix would be vacuous" >&2
      exit 1
    fi
  fi
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# _build_gh_stub_shim_once — moved before the vacuity guard (ML-R2b1 corrective).
# The guard calls it on Windows; a forward reference causes "command not found" there.
# Current definition is above, before the "ML-R2b1 (corrected):" guard block.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# gh stubs — one directory per PR state. Never emits anything gh's real output shape wouldn't:
# a JSON array (possibly empty) on success, non-zero exit + stderr on failure.
# ---------------------------------------------------------------------------
write_gh_stub() {
  local dir=$1 mode=$2
  _build_gh_stub_shim_once
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
  # Windows: copy compiled PE shim so native processes (Go/Node/Python) find gh via PATHEXT
  [[ -n "$_GH_STUB_SHIM" ]] && cp "$_GH_STUB_SHIM" "$dir/gh.exe" || true
}

# ---------------------------------------------------------------------------
# make_fixture DEST BRANCH — real bare "origin" + a clone on BRANCH, one commit pushed, then
# amended locally (new SHA) so a plain push would be rejected and --force-with-lease is the
# governed way through. Isolated HOME/gitconfig per (scenario, runtime) fixture, same pattern as
# check-branch-prune-parity.sh's build_fixture. Prints the clone path.
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

    echo base >base.txt
    env "${e[@]}" git add base.txt
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

# run_ship RUNTIME DIR PATH_PREFIX ARGS...
# Runs `trackfw ship ARGS...` from DIR with PATH="<PATH_PREFIX>:$BASE_PATH" (PATH_PREFIX may be
# empty). Sets SF_EXIT and writes stdout/stderr to $WORK/<label>.<runtime>.{out,err}.
# If RUN_PATH_OVERRIDE is set (non-empty), it REPLACES the whole PATH computation above —
# BASE_PATH is not consulted at all. Used only by scenario (a), which needs NO_FORGE_PATH exactly
# (no /usr/bin, no /bin) rather than a prefix layered on top of BASE_PATH.
run_ship() {
  local runtime=$1 dir=$2 path_prefix=$3
  shift 3
  local out_file="$WORK/$SF_LABEL.$runtime.out" err_file="$WORK/$SF_LABEL.$runtime.err"
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
    go)   (cd "$dir" && PATH="$run_path" "$GO_BIN" ship "$@")                               >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && PATH="$run_path" node "$NODE_CLI" ship "$@")                        >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PATH="$run_path" PYTHONPATH="$PY_ROOT" python3 -m trackfw ship "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_ship: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  SF_EXIT=$?
  set -e
  SF_OUT_FILE=$out_file
  SF_ERR_FILE=$err_file
}

# assert_three_way LABEL — byte-level diff of stdout/stderr across the 3 runtimes, plus exit
# code equality. Mirrors check-ship-parity.sh's assert_three_way exactly.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "ship-force-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "ship-force-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "ship-force-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "ship-force-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Scenario (a) — sem CLI de forge: PATH has no gh/glab/az at all. Must refuse without
# degrading to a permissive push, remote untouched. Runs against NO_FORGE_PATH (git-only,
# curated), NOT BASE_PATH — see the NO_FORGE_PATH comment above run_ship for why.
# ---------------------------------------------------------------------------
SF_LABEL="no-forge-cli"
RUN_PATH_OVERRIDE="$NO_FORGE_PATH"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/a-$runtime" "chore/no-forge-cli")
  bare="$WORK/a-$runtime/origin.git"
  before=$(remote_head "$bare" "chore/no-forge-cli")
  run_ship "$runtime" "$fixture" "" --force-with-lease --forge github --no-pr
  echo "$SF_EXIT" >"$WORK/$SF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/no-forge-cli")
  if [[ "$SF_EXIT" -eq 0 ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "expected non-zero exit when no forge CLI is available, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "refusal must never touch the remote; before=$before after=$after"
    continue
  fi
  if ! grep -qF 'requires a forge CLI (gh, glab, or az) to confirm an open pull/merge request' "$SF_ERR_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "vacuity guard: stderr missing the no-forge-CLI refusal; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
done
unset RUN_PATH_OVERRIDE
assert_three_way "$SF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — forge, zero PR: gh available, `pr list` returns an empty array. Must refuse
# naming the branch and pointing at opening the PR first; remote untouched.
# ---------------------------------------------------------------------------
SF_LABEL="forge-zero-pr"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/b-$runtime" "chore/zero-pr")
  bare="$WORK/b-$runtime/origin.git"
  stub="$WORK/b-$runtime-stub"
  write_gh_stub "$stub" empty
  before=$(remote_head "$bare" "chore/zero-pr")
  run_ship "$runtime" "$fixture" "$stub" --force-with-lease --forge github --no-pr
  echo "$SF_EXIT" >"$WORK/$SF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/zero-pr")
  if [[ "$SF_EXIT" -eq 0 ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "expected non-zero exit with zero open PRs, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "refusal must never touch the remote; before=$before after=$after"
    continue
  fi
  if ! grep -qF 'has no open pull/merge request. Open the PR/MR first' "$SF_ERR_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "vacuity guard: stderr missing the no-open-PR refusal; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
  if ! grep -qF 'chore/zero-pr' "$SF_ERR_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "refusal must name the branch; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
done
assert_three_way "$SF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — forge, não verificável: gh available but the `pr list` call itself fails
# (auth error). Must be its OWN refusal class — never conflated with "no PR" — and must surface
# the CLI's actual stderr text (the byte-for-byte discriminant that caught the real Go/Node/
# Python divergence this ML fixed: Go's exec.Command().Output() error alone is the generic "exit
# status 1", discarding the CLI's own diagnostic that Node/Python already surfaced).
# ---------------------------------------------------------------------------
SF_LABEL="forge-unverifiable"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/c-$runtime" "chore/unverifiable")
  bare="$WORK/c-$runtime/origin.git"
  stub="$WORK/c-$runtime-stub"
  write_gh_stub "$stub" unverifiable
  before=$(remote_head "$bare" "chore/unverifiable")
  run_ship "$runtime" "$fixture" "$stub" --force-with-lease --forge github --no-pr
  echo "$SF_EXIT" >"$WORK/$SF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/unverifiable")
  if [[ "$SF_EXIT" -eq 0 ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "expected non-zero exit when the PR check cannot be verified, got 0"
    continue
  fi
  if [[ "$before" != "$after" ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "refusal must never touch the remote; before=$before after=$after"
    continue
  fi
  if ! grep -qF 'could not verify whether branch' "$SF_ERR_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "vacuity guard: stderr missing the cannot-verify refusal; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
  if grep -qF 'has no open pull/merge request' "$SF_ERR_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "cannot-verify must never be conflated with no-PR; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
  # The stub's actual stderr text must survive into the refusal — not a generic "exit status N".
  if ! grep -qF 'authentication required' "$SF_ERR_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "the forge CLI's real stderr text must surface in the refusal, not a generic exit-status message; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
done
assert_three_way "$SF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (d) — forge, PR aberto: gh confirms an open PR. Push must actually happen —
# proved by the remote SHA changing, never by the printed message alone.
# ---------------------------------------------------------------------------
SF_LABEL="forge-pr-open-pushes"
for runtime in go node py; do
  fixture=$(make_fixture "$WORK/d-$runtime" "chore/pr-open")
  bare="$WORK/d-$runtime/origin.git"
  stub="$WORK/d-$runtime-stub"
  write_gh_stub "$stub" open
  before=$(remote_head "$bare" "chore/pr-open")
  fixture_head_sha=$(cd "$fixture" && git rev-parse HEAD)
  run_ship "$runtime" "$fixture" "$stub" --force-with-lease --forge github --no-pr
  echo "$SF_EXIT" >"$WORK/$SF_LABEL.$runtime.exit"
  after=$(remote_head "$bare" "chore/pr-open")
  if [[ "$SF_EXIT" -ne 0 ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "expected exit 0 with an open PR confirmed, got $SF_EXIT; stderr: $(cat "$SF_ERR_FILE")"
    continue
  fi
  if [[ "$before" == "$after" ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "vacuity guard: remote SHA did not change — the push never happened; before=$before after=$after"
    continue
  fi
  if [[ "$after" != "$fixture_head_sha" ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "remote SHA after push must equal the local amended commit; local=$fixture_head_sha remote=$after"
    continue
  fi
  if ! grep -qF 'ship complete.' "$SF_OUT_FILE"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "vacuity guard: stdout missing the completion marker; stdout: $(cat "$SF_OUT_FILE")"
    continue
  fi
done
assert_three_way "$SF_LABEL"

# ---------------------------------------------------------------------------
# Scenario (e) — semantic discriminant: --force-with-lease refuses when the remote advanced
# past what this clone last recorded; --force does not, and destroys the other party's commit.
# See the file header for the full construction. Proves the discriminant itself, per-runtime
# (not a cross-runtime diff — the three runtimes must ALL refuse and ALL leave the remote
# untouched; this is what scripts/check-gates-falsify.sh's P4 sabotages and this scenario
# catches).
# ---------------------------------------------------------------------------
SF_LABEL="remote-advanced-lease-mismatch"
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
    echo base >base.txt
    env "${e[@]}" git add base.txt
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
    env "${e[@]}" git checkout -q chore/remote-advanced
    echo extra >extra.txt
    env "${e[@]}" git add extra.txt
    env "${e[@]}" git commit -q -m "another party's legitimate commit"
    env "${e[@]}" git push -q origin chore/remote-advanced
  ) >>"$dest/build.log" 2>&1

  # Our clone's remote-tracking ref stays pinned stale on purpose (fetch refspec restricted to
  # main only) — so ship's own internal `git fetch origin --prune` (Step 3) never learns about
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
  # cost a real debugging cycle building this gate.
  remote_log=$(git --git-dir="$bare" log --oneline chore/remote-advanced 2>/dev/null || true)
  if ! grep -qF "another party's legitimate commit" <<<"$remote_log"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "vacuity guard: the other clone's commit is not on the remote before running ship"
    continue
  fi

  run_ship "$runtime" "$clone" "$stub" --force-with-lease --forge github --no-pr
  echo "$SF_EXIT" >"$WORK/$SF_LABEL.$runtime.exit"
  remote_after=$(remote_head "$bare" "chore/remote-advanced")

  if [[ "$SF_EXIT" -eq 0 ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "--force-with-lease must refuse when the remote advances past the recorded lease (real git safety semantics), got exit 0; stdout: $(cat "$SF_OUT_FILE")"
    continue
  fi
  if [[ "$remote_before" != "$remote_after" ]]; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "the other clone's commit must survive untouched; remote moved from $remote_before to $remote_after"
    continue
  fi
  # Captured into a variable before grep -qF, never piped directly: under `set -o pipefail`, a
  # `git log | grep -q` pipe can make `git log` receive SIGPIPE once grep is satisfied and
  # closes early, turning the pipeline's exit status non-zero even though grep DID match —
  # cost a real debugging cycle building this gate.
  remote_log=$(git --git-dir="$bare" log --oneline chore/remote-advanced 2>/dev/null || true)
  if ! grep -qF "another party's legitimate commit" <<<"$remote_log"; then
    fail "ship-force-parity/$SF_LABEL/$runtime" "the other clone's commit must still be reachable from the remote branch after the refused push"
    continue
  fi

  # Normalize: git's own rejection message embeds the bare remote's absolute filesystem path
  # ("To /.../e-go/origin.git", "error: failed to push ... '/.../e-go/origin.git'") — this
  # differs across runtimes ONLY because each runtime's fixture is built in its own isolated
  # directory (e-go/e-node/e-py), never because of anything the CLI under test does. Substitute
  # the real path with a fixed placeholder before the byte-diff so the comparison targets git's
  # actual diagnostic text, not this gate's own fixture layout.
  sed -i.bak "s#$dest#<FIXTURE_DIR>#g" "$SF_OUT_FILE" "$SF_ERR_FILE" && rm -f "$SF_OUT_FILE.bak" "$SF_ERR_FILE.bak"
done
assert_three_way "$SF_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-ship-force-parity.sh scenarios passed."
else
  echo "check-ship-force-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
