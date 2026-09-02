#!/usr/bin/env bash
# check-doctor-remote-parity.sh — proves `trackfw doctor --remote` behaves byte-for-byte
# identically in Go, Node.js, and Python (ML-3A,
# ADR-2026-09-02-doctor-ganha-modalidade-remota-opcional-e-ausencia-de-credencial-vira-nao-
# avaliado-nunca-aprovacao.md), for every path the --remote modality can take:
#
#   (a) no branch protection at all      -> required-status-checks-missing + enforce-admins-disabled
#   (b) branch protection fully configured (the CONTROL for (a) — without it we would have a
#       doctor that accuses always, and an alarm that always fires gets learned to be ignored)
#   (c) no GitHub credential              -> not-evaluated, remedy names "authenticate"
#   (d) credential present but lacks admin access on the repo -> not-evaluated, DISTINCT remedy
#       from (c) — this is the exact case the ADR exists to name: token-sem-escopo is not
#       token-ausente
#   (e) gh CLI absent from PATH           -> not-evaluated, never shells out
#   (f) forge is not GitHub               -> not-evaluated, never shells out to gh
#   (g) core.hooksPath=/dev/null          -> hooks-path-neutralized (local-only, no network)
#   (h) core.hooksPath unset              -> the CONTROL for (g)
#
# Mechanism: a `gh` STUB script on PATH, not a real network call — the same pattern
# check-release-tag-parity.sh already established for `gh api`. This is deterministic and
# fully offline-capable (the ADR itself acknowledges the real network path cannot be exercised
# in CI: "ninguém consegue exercitá-lo em CI offline" — that gap is real and stays undescribed
# by this gate, which only proves the CLI's own dispatch/parsing/message logic, not that gh
# itself would answer this way against a real GitHub API). It avoids reinventing an HTTP+token
# client 3x (release tag already chose `gh api` over net/http for the same reason — see
# internal/commands/release.go).
#
# Follows the conventions of check-release-tag-parity.sh / check-doctor-parity.sh: set -euo
# pipefail, NO_COLOR=1/TERM=dumb, BASH_SOURCE-relative ROOT_DIR, mktemp -d fixture with cleanup
# trap, ok()/fail() accumulating FAIL=1, byte-level diff -u between runtimes, and a vacuity
# guard proving the restricted "no gh on PATH" scenario genuinely has none.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-doctor-remote-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes.
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
  echo "check-doctor-remote-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-doctor-remote-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

REAL_NODE=$(command -v node || true)
REAL_PYTHON3=$(command -v python3 || true)
REAL_GIT=$(command -v git || true)
if [[ -z "$REAL_NODE" ]]; then
  echo "check-doctor-remote-parity: node not found in PATH" >&2
  exit 1
fi
if [[ -z "$REAL_PYTHON3" ]]; then
  echo "check-doctor-remote-parity: python3 not found in PATH" >&2
  exit 1
fi
if [[ -z "$REAL_GIT" ]]; then
  echo "check-doctor-remote-parity: git not found in PATH" >&2
  exit 1
fi

# RUNTIME_BIN carries only the interpreters + git the three CLIs need, symlinked from their
# real location — scenario PATHs below never inherit the caller's PATH, so a real gh installed
# on this machine can never leak into a scenario that must see none.
RUNTIME_BIN="$WORK/runtimebin"
mkdir -p "$RUNTIME_BIN"
ln -s "$REAL_NODE" "$RUNTIME_BIN/node"
ln -s "$REAL_PYTHON3" "$RUNTIME_BIN/python3"
ln -s "$REAL_GIT" "$RUNTIME_BIN/git"

unset TRACKFW_DISABLE_EXTERNAL_COMMANDS || true

# BASE_PATH: RUNTIME_BIN only — no /usr/bin:/bin. An earlier version of this gate added
# /usr/bin:/bin here because the gh STUB's shebang was "#!/usr/bin/env bash", and `env` needs
# something on PATH to resolve `bash`. That fixed the stub locally but broke the "no gh on
# PATH" scenario's own premise on GitHub's runners, where `gh` ships preinstalled at
# /usr/bin/gh: BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin" made `gh` resolvable again by construction,
# and the vacuity guard below correctly refused to validate a "no gh" scenario that in fact had
# one (see vault/notes/gh-stub-shebang-needs-usr-bin-in-restricted-path-2026-09-02.md for the
# original finding, and the note this gate's own CI failure produced on 2026-09-02 for the
# sequel). The fix is upstream of BASE_PATH: the gh STUB below uses an ABSOLUTE shebang
# ("#!/bin/bash", present at that exact path on both macOS and every GitHub-hosted Linux/macOS
# runner) instead of "#!/usr/bin/env bash", so it never depends on PATH to resolve its own
# interpreter. That removes the only reason /usr/bin:/bin was ever in BASE_PATH — RUNTIME_BIN
# alone is sufficient (proven: `git init`/`remote add`/`config` need nothing outside it either).
BASE_PATH="$RUNTIME_BIN"

# Vacuity guard — fails BEFORE any scenario runs if gh somehow resolves on BASE_PATH alone
# (the "no gh CLI" scenario's PATH). A scenario claiming "no gh on PATH" that secretly still
# had one would prove nothing.
if resolved=$(PATH="$BASE_PATH" command -v gh 2>/dev/null); then
  echo "check-doctor-remote-parity: vacuity guard failed — 'gh' resolves on the no-gh PATH ($BASE_PATH) at $resolved" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# write_gh_stub DIR MODE — a `gh` stub answering exactly the calls runDoctorRemote makes, in
# order: `auth status`, `api repos/{owner}/{repo}`, `api repos/{owner}/{repo}/branches/<b>/
# protection`. MODE selects which fixture branch-protection response to answer with.
#   MODE=no-auth        -> "auth status" fails (simulates no credential)
#   MODE=no-admin       -> repo info reports permissions.admin=false
#   MODE=no-protection  -> protection lookup 404s (branch genuinely unprotected)
#   MODE=configured     -> protection lookup returns a fully configured gate
# ---------------------------------------------------------------------------
write_gh_stub() {
  local dir=$1 mode=$2
  mkdir -p "$dir"
  # Absolute shebang, not "#!/usr/bin/env bash": the stub must launch under BASE_PATH="$RUNTIME_BIN"
  # alone (no /usr/bin:/bin), and "env" would need something on PATH to resolve "bash". /bin/bash
  # exists at that exact absolute path on macOS and on every GitHub-hosted Linux/macOS runner.
  cat >"$dir/gh" <<EOF
#!/bin/bash
set -euo pipefail
case "\$1 \$2" in
  "auth status")
$( [[ "$mode" == "no-auth" ]] && echo '    echo "gh: not logged into any GitHub hosts" >&2; exit 1' || echo '    echo "logged in"' )
    ;;
  "api repos/{owner}/{repo}")
$( [[ "$mode" == "no-admin" ]] \
     && echo '    echo "{\"default_branch\":\"main\",\"permissions\":{\"admin\":false}}"' \
     || echo '    echo "{\"default_branch\":\"main\",\"permissions\":{\"admin\":true}}"' )
    ;;
  "api repos/{owner}/{repo}/branches/main/protection")
$( case "$mode" in
     no-protection) echo '    echo "gh: Branch not protected (HTTP 404)" >&2; exit 1' ;;
     configured)    echo '    echo "{\"required_status_checks\":{\"strict\":true,\"contexts\":[\"governance-go-install\"]},\"enforce_admins\":{\"enabled\":true}}"' ;;
     *)             echo '    echo "unexpected call for mode $mode: \$*" >&2; exit 1' ;;
   esac )
    ;;
  *)
    echo "doctor-remote-parity gh stub: unexpected call: \$*" >&2
    exit 1
    ;;
esac
EOF
  chmod +x "$dir/gh"
}

# ---------------------------------------------------------------------------
# build_fixture DEST HOOKS_PATH_MODE — a real git repo with a github.com origin remote (never
# actually dialed — gh is stubbed) and, optionally, a neutralized core.hooksPath.
# HOOKS_PATH_MODE: "neutralized" sets core.hooksPath=/dev/null; "unset" leaves it untouched;
# "husky" sets it to a plausible custom hooks dir (proves no false positive on legitimate use).
# ---------------------------------------------------------------------------
build_fixture() {
  local dest=$1 hooks_mode=$2
  mkdir -p "$dest"
  (cd "$dest" && PATH="$BASE_PATH" git init -q)
  (cd "$dest" && PATH="$BASE_PATH" git remote add origin https://github.com/kgsaran/trackfw-doctor-remote-fixture.git)
  case "$hooks_mode" in
    neutralized) (cd "$dest" && PATH="$BASE_PATH" git config core.hooksPath /dev/null) ;;
    husky)       (cd "$dest" && PATH="$BASE_PATH" git config core.hooksPath .husky/_) ;;
    unset)       : ;;
    *) echo "build_fixture: unknown hooks_mode $hooks_mode" >&2; exit 1 ;;
  esac
  # Deliberately NO trackfw.yaml: RunScaffoldDoctor's own eligibility check (internal/
  # generators/scaffold_doctor.go) requires it to exist, and this fixture has none of the
  # scaffold artifacts it would then expect — so every scenario would carry 5 unrelated
  # scaffold-missing findings, adding noise (and a byte-level "vunknown" vs "v7.3.0" divergence
  # between runtimes stemming from how each resolves its OWN version string outside a real
  # install, e.g. Python's importlib.metadata falling back when run via PYTHONPATH instead of
  # an installed distribution — an environment artifact of this gate's mechanism, unrelated to
  # --remote). config.Load() tolerates a missing trackfw.yaml (returns defaults, never errors),
  # so omitting it costs nothing the --remote scenarios below need.
}

# run_all LABEL DIR GH_STUB_DIR — runs `doctor --remote` on all three CLIs with a PATH that
# carries BASE_PATH plus the given gh stub dir (empty string = no gh at all), diffs their
# stdout pairwise, and captures Go's stdout in $WORK/$LABEL.go.out for vacuity assertions.
run_all() {
  local label=$1 dir=$2 gh_dir=$3
  local run_path
  if [[ -n "$gh_dir" ]]; then
    run_path="$gh_dir:$BASE_PATH"
  else
    run_path="$BASE_PATH"
  fi

  (cd "$dir" && PATH="$run_path" HOME="$WORK/home" "$GO_BIN" doctor --remote) >"$WORK/$label.go.out" 2>"$WORK/$label.go.err" || true
  (cd "$dir" && PATH="$run_path" HOME="$WORK/home" node "$NODE_CLI" doctor --remote) >"$WORK/$label.node.out" 2>"$WORK/$label.node.err" || true
  (cd "$dir" && PATH="$run_path" HOME="$WORK/home" PYTHONPATH="$PY_ROOT" python3 -m trackfw doctor --remote) >"$WORK/$label.py.out" 2>"$WORK/$label.py.err" || true

  if diff -u "$WORK/$label.go.out" "$WORK/$label.node.out" >"$WORK/$label.diff.go-node" 2>&1; then
    ok "$label/go-vs-node"
  else
    fail "$label/go-vs-node" "$(cat "$WORK/$label.diff.go-node")"
  fi
  if diff -u "$WORK/$label.go.out" "$WORK/$label.py.out" >"$WORK/$label.diff.go-py" 2>&1; then
    ok "$label/go-vs-py"
  else
    fail "$label/go-vs-py" "$(cat "$WORK/$label.diff.go-py")"
  fi
}

mkdir -p "$WORK/home/.trackfw"

# ---------------------------------------------------------------------------
# Scenario (a) — no branch protection at all -> both findings, never not-evaluated.
# ---------------------------------------------------------------------------
a_dir="$WORK/a-no-protection"
build_fixture "$a_dir" unset
a_gh="$WORK/gh-no-protection"
write_gh_stub "$a_gh" no-protection
run_all "no-protection" "$a_dir" "$a_gh"
if grep -q "^\[required-status-checks-missing\]" "$WORK/no-protection.go.out" && grep -q "^\[enforce-admins-disabled\]" "$WORK/no-protection.go.out"; then
  ok "no-protection/vacuity-guard/both-findings-present"
else
  fail "no-protection/vacuity-guard/both-findings-present" "expected both finding kinds in stdout, got: $(cat "$WORK/no-protection.go.out")"
fi
if grep -q "^\[not-evaluated\]" "$WORK/no-protection.go.out"; then
  fail "no-protection/never-not-evaluated" "a genuinely evaluated check must not also claim not-evaluated: $(cat "$WORK/no-protection.go.out")"
else
  ok "no-protection/never-not-evaluated"
fi

# ---------------------------------------------------------------------------
# Scenario (b) — the CONTROL: branch protection fully configured -> clean report.
# ---------------------------------------------------------------------------
b_dir="$WORK/b-configured"
build_fixture "$b_dir" unset
b_gh="$WORK/gh-configured"
write_gh_stub "$b_gh" configured
run_all "configured" "$b_dir" "$b_gh"
if grep -q "no mismatches found" "$WORK/configured.go.out"; then
  ok "configured/control/clean-report"
else
  fail "configured/control/clean-report" "expected a clean report when the gate is fully configured, got: $(cat "$WORK/configured.go.out")"
fi

# ---------------------------------------------------------------------------
# Scenario (c) — no GitHub credential -> not-evaluated, remedy names authentication.
# ---------------------------------------------------------------------------
c_dir="$WORK/c-no-auth"
build_fixture "$c_dir" unset
c_gh="$WORK/gh-no-auth"
write_gh_stub "$c_gh" no-auth
run_all "no-auth" "$c_dir" "$c_gh"
if grep -q "^\[not-evaluated\]" "$WORK/no-auth.go.out" && grep -qi "authenticate" "$WORK/no-auth.go.out"; then
  ok "no-auth/vacuity-guard/not-evaluated-names-authenticate"
else
  fail "no-auth/vacuity-guard/not-evaluated-names-authenticate" "expected not-evaluated naming authentication, got: $(cat "$WORK/no-auth.go.out")"
fi
if grep -q "^\[required-status-checks-missing\]" "$WORK/no-auth.go.out"; then
  fail "no-auth/never-collapses-to-finding" "absence of credential must never be reported as a finding claiming the gate is missing: $(cat "$WORK/no-auth.go.out")"
else
  ok "no-auth/never-collapses-to-finding"
fi

# ---------------------------------------------------------------------------
# Scenario (d) — credential present, lacks admin access -> not-evaluated, DISTINCT from (c).
# ---------------------------------------------------------------------------
d_dir="$WORK/d-no-admin"
build_fixture "$d_dir" unset
d_gh="$WORK/gh-no-admin"
write_gh_stub "$d_gh" no-admin
run_all "no-admin" "$d_dir" "$d_gh"
if grep -q "^\[not-evaluated\]" "$WORK/no-admin.go.out" && grep -qi "admin access" "$WORK/no-admin.go.out"; then
  ok "no-admin/vacuity-guard/not-evaluated-names-admin-access"
else
  fail "no-admin/vacuity-guard/not-evaluated-names-admin-access" "expected not-evaluated naming admin access, got: $(cat "$WORK/no-admin.go.out")"
fi
if diff -q "$WORK/no-auth.go.out" "$WORK/no-admin.go.out" >/dev/null 2>&1; then
  fail "no-admin/distinct-from-no-auth" "insufficient-scope and no-credential produced byte-identical reports — the ADR requires distinct remedies"
else
  ok "no-admin/distinct-from-no-auth"
fi

# ---------------------------------------------------------------------------
# Scenario (e) — gh CLI absent from PATH entirely -> not-evaluated.
# ---------------------------------------------------------------------------
e_dir="$WORK/e-no-gh"
build_fixture "$e_dir" unset
run_all "no-gh" "$e_dir" ""
if grep -q "^\[not-evaluated\]" "$WORK/no-gh.go.out"; then
  ok "no-gh/vacuity-guard/not-evaluated"
else
  fail "no-gh/vacuity-guard/not-evaluated" "expected not-evaluated with no gh on PATH, got: $(cat "$WORK/no-gh.go.out")"
fi

# ---------------------------------------------------------------------------
# Scenario (f) — forge is not GitHub -> not-evaluated, never a finding.
# ---------------------------------------------------------------------------
f_dir="$WORK/f-non-github"
mkdir -p "$f_dir"
(cd "$f_dir" && PATH="$BASE_PATH" git init -q)
(cd "$f_dir" && PATH="$BASE_PATH" git remote add origin git@gitlab.com:kgsaran/trackfw-doctor-remote-fixture.git)
run_all "non-github" "$f_dir" "$a_gh"
if grep -q "^\[not-evaluated\]" "$WORK/non-github.go.out"; then
  ok "non-github/vacuity-guard/not-evaluated"
else
  fail "non-github/vacuity-guard/not-evaluated" "expected not-evaluated for a non-GitHub forge, got: $(cat "$WORK/non-github.go.out")"
fi

# ---------------------------------------------------------------------------
# Scenario (g) — core.hooksPath=/dev/null -> hooks-path-neutralized. Local-only: paired with
# the no-protection gh stub so the run also produces the two branch-protection findings, and
# non-vacuity here means all THREE finding kinds are present together.
# ---------------------------------------------------------------------------
g_dir="$WORK/g-hooks-neutralized"
build_fixture "$g_dir" neutralized
run_all "hooks-neutralized" "$g_dir" "$a_gh"
if grep -q "^\[hooks-path-neutralized\]" "$WORK/hooks-neutralized.go.out"; then
  ok "hooks-neutralized/vacuity-guard/finding-present"
else
  fail "hooks-neutralized/vacuity-guard/finding-present" "expected hooks-path-neutralized, got: $(cat "$WORK/hooks-neutralized.go.out")"
fi

# ---------------------------------------------------------------------------
# Scenario (h) — the CONTROL for (g): core.hooksPath unset -> no hooks-path-neutralized.
# ---------------------------------------------------------------------------
h_dir="$WORK/h-hooks-unset"
build_fixture "$h_dir" unset
run_all "hooks-unset" "$h_dir" "$a_gh"
if grep -q "^\[hooks-path-neutralized\]" "$WORK/hooks-unset.go.out"; then
  fail "hooks-unset/control/no-false-positive" "unset core.hooksPath must never be flagged: $(cat "$WORK/hooks-unset.go.out")"
else
  ok "hooks-unset/control/no-false-positive"
fi

# ---------------------------------------------------------------------------
# Scenario (i) — a custom husky hooksPath must never be flagged either (Wave 0's "does not
# break legitimate flow" constraint).
# ---------------------------------------------------------------------------
i_dir="$WORK/i-hooks-husky"
build_fixture "$i_dir" husky
run_all "hooks-husky" "$i_dir" "$a_gh"
if grep -q "^\[hooks-path-neutralized\]" "$WORK/hooks-husky.go.out"; then
  fail "hooks-husky/no-false-positive" "a legitimate custom hooksPath (.husky/_) must never be flagged: $(cat "$WORK/hooks-husky.go.out")"
else
  ok "hooks-husky/no-false-positive"
fi

# ---------------------------------------------------------------------------
# Scenario (j) — without --remote at all, the flag never fires: doctor stays offline/silent
# on a repo whose gate is genuinely absent. Proves zero regression to the pre-ADR default.
# ---------------------------------------------------------------------------
j_dir="$WORK/j-no-flag"
build_fixture "$j_dir" neutralized
(cd "$j_dir" && PATH="$BASE_PATH" HOME="$WORK/home" "$GO_BIN" doctor) >"$WORK/no-flag.go.out" 2>&1 || true
(cd "$j_dir" && PATH="$BASE_PATH" HOME="$WORK/home" node "$NODE_CLI" doctor) >"$WORK/no-flag.node.out" 2>&1 || true
(cd "$j_dir" && PATH="$BASE_PATH" HOME="$WORK/home" PYTHONPATH="$PY_ROOT" python3 -m trackfw doctor) >"$WORK/no-flag.py.out" 2>&1 || true
if grep -q "hooks-path-neutralized\|not-evaluated\|required-status-checks" "$WORK/no-flag.go.out"; then
  fail "no-flag/no-regression" "doctor without --remote must never emit remote-modality findings, got: $(cat "$WORK/no-flag.go.out")"
else
  ok "no-flag/no-regression"
fi
if diff -u "$WORK/no-flag.go.out" "$WORK/no-flag.node.out" >"$WORK/no-flag.diff.go-node" 2>&1; then
  ok "no-flag/go-vs-node"
else
  fail "no-flag/go-vs-node" "$(cat "$WORK/no-flag.diff.go-node")"
fi
if diff -u "$WORK/no-flag.go.out" "$WORK/no-flag.py.out" >"$WORK/no-flag.diff.go-py" 2>&1; then
  ok "no-flag/go-vs-py"
else
  fail "no-flag/go-vs-py" "$(cat "$WORK/no-flag.diff.go-py")"
fi

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-doctor-remote-parity.sh: one or more scenarios FAILED." >&2
  exit 1
fi
echo "check-doctor-remote-parity.sh: all scenarios passed."
