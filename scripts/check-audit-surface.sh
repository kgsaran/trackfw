#!/usr/bin/env bash
# check-audit-surface.sh — gate for `trackfw audit-surface` (Wave 2 / ML-2A,
# ROADMAP-2026-08-26-comando-que-audita-a-superficie-executavel-de-um-checkout-de-pr).
#
# Builds throwaway git fixture repos under a temp dir, drives all three compiled/
# interpreted CLIs (Go, Node.js, Python) against them, and asserts:
#   FN-1  hook wiring present → reported
#   FN-2  only the script changes, wiring intact → digest changes (AC2/AC14)
#   FN-3  hook in runtime whose path was absent → still scanned/reported (AC13)
#   FN-4  matcher widens "Bash" → "*" → tuple changes (AC14)
#   FN-5  instruction file present → reported with "instruction" label (AC15)
#   FP-1  docs/cli-parity.md NOT in output (AC16, free fixture: real repo HEAD)
#   FP-2  internal/generators/agentfiles.go NOT in output (AC16, free fixture)
#
# Self-test seam for check-gates-falsify.sh:
#   AUDIT_SURFACE_SELFTEST_BREAK=A  builds a digest-constant binary; FN-2
#     fails at audit-surface/fn-2/digest-changes-when-script-changes
#   AUDIT_SURFACE_SELFTEST_BREAK=B  builds an instruction-path-extended binary
#     (docs/cli-parity.md added to instructionFilePaths); FP-1 fails at
#     audit-surface/fp-1/cli-parity-absent
#
# Git commit guard note: the branch guard fires at the agent's Bash TOOL level,
# not at subprocess level. All git operations here run as subprocesses inside
# this shell script, so they are never intercepted. Each fixture repo uses
# `git config core.hooksPath /dev/null` and `commit.gpgsign false` to suppress
# any git hooks that might exist in the fixture directories themselves.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-audit-surface.XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

# Isolated HOME so that any `trackfw validate` executed by the CLIs under test
# does not see the real user's global guards (same pattern as check-barrier.sh).
export HOME="$WORK/home"
mkdir -p "$HOME"
export NO_COLOR=1
export TERM=dumb

# Preserve real Go caches so that `go build` inside this script remains fast.
export GOPATH="${GOPATH:-$(go env GOPATH)}"
export GOCACHE="${GOCACHE:-$(go env GOCACHE)}"
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"

# ---------------------------------------------------------------------------
# Resolve the three runtimes.
# GO_BIN may be passed in (absolute or relative to ROOT_DIR, as the Makefile
# does with GO_BIN=$(BUILD_DIR)/$(BINARY)); otherwise build a throwaway binary.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-audit-surface: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-audit-surface: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Self-test seam. When AUDIT_SURFACE_SELFTEST_BREAK is set, build a sabotaged
# Go binary and store it in a seam variable. Each scenario uses its designated
# seam variable (not GO_BIN) for the specific assertion that exercises AC9.
# ---------------------------------------------------------------------------
SELFTEST_BREAK="${AUDIT_SURFACE_SELFTEST_BREAK:-}"

EVAL_BIN_FN2="$GO_BIN"   # binary used for FN-2 digest comparison
EVAL_BIN_FP1="$GO_BIN"   # binary used for FP-1/FP-2 false-positive checks

if [[ "$SELFTEST_BREAK" == "A" ]]; then
  # Direction A: constant digest — FN-2 sees same digest at both refs.
  # Sabotage: replace the sha256 computation with a string constant.
  T_A="$WORK/selftest-a"
  mkdir -p "$T_A/cmd" "$T_A/internal"
  cp -r "$ROOT_DIR/cmd/." "$T_A/cmd/"
  cp -r "$ROOT_DIR/internal/." "$T_A/internal/"
  cp "$ROOT_DIR/go.mod" "$T_A/"
  cp "$ROOT_DIR/go.sum" "$T_A/"

  sed 's/h := sha256\.Sum256(scriptBytes)/h := sha256.Sum256(nil); _ = scriptBytes/' \
    "$ROOT_DIR/internal/auditsurface/auditsurface.go" \
    > "$T_A/internal/auditsurface/auditsurface.go"

  if cmp -s \
      "$ROOT_DIR/internal/auditsurface/auditsurface.go" \
      "$T_A/internal/auditsurface/auditsurface.go"; then
    echo "FAIL [audit-surface/selftest-break-a/setup]: sed did not alter auditsurface.go — pattern not found; falsification invalid" >&2
    exit 1
  fi

  SELFTEST_A_BIN="$WORK/selftest-a-bin/trackfw"
  mkdir -p "$(dirname "$SELFTEST_A_BIN")"
  (cd "$T_A" && go build -o "$SELFTEST_A_BIN" ./cmd/trackfw) || {
    echo "FAIL [audit-surface/selftest-break-a/build]: go build of sabotaged binary failed" >&2
    exit 1
  }
  EVAL_BIN_FN2="$SELFTEST_A_BIN"
fi

if [[ "$SELFTEST_BREAK" == "B" ]]; then
  # Direction B: docs/cli-parity.md added to instructionFilePaths — FP-1 sees
  # it in the output and the "absent from output" assertion fails.
  T_B="$WORK/selftest-b"
  mkdir -p "$T_B/cmd" "$T_B/internal"
  cp -r "$ROOT_DIR/cmd/." "$T_B/cmd/"
  cp -r "$ROOT_DIR/internal/." "$T_B/internal/"
  cp "$ROOT_DIR/go.mod" "$T_B/"
  cp "$ROOT_DIR/go.sum" "$T_B/"

  python3 - "$ROOT_DIR/internal/auditsurface/auditsurface.go" \
             "$T_B/internal/auditsurface/auditsurface.go" << 'PYEOF'
import sys
src, dst = sys.argv[1], sys.argv[2]
content = open(src).read()
new_content = content.replace(
    '".cursor/rules/trackfw.mdc"',
    '".cursor/rules/trackfw.mdc",\n\t"docs/cli-parity.md"'
)
open(dst, 'w').write(new_content)
PYEOF

  if cmp -s \
      "$ROOT_DIR/internal/auditsurface/auditsurface.go" \
      "$T_B/internal/auditsurface/auditsurface.go"; then
    echo "FAIL [audit-surface/selftest-break-b/setup]: python3 did not alter auditsurface.go — pattern not found; falsification invalid" >&2
    exit 1
  fi

  SELFTEST_B_BIN="$WORK/selftest-b-bin/trackfw"
  mkdir -p "$(dirname "$SELFTEST_B_BIN")"
  (cd "$T_B" && go build -o "$SELFTEST_B_BIN" ./cmd/trackfw) || {
    echo "FAIL [audit-surface/selftest-break-b/build]: go build of sabotaged binary failed" >&2
    exit 1
  }
  EVAL_BIN_FP1="$SELFTEST_B_BIN"
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; exit 1; }

# make_repo DIR — initialise a minimal git repo for fixtures.
make_repo() {
  local dir=$1
  git -C "$dir" init -q
  git -C "$dir" config user.email "test@example.com"
  git -C "$dir" config user.name "Test"
  git -C "$dir" config core.hooksPath /dev/null
  git -C "$dir" config commit.gpgsign false
}

# run_audit BINARY DIR REF [extra args...]
# Runs the given binary as `audit-surface REF` from DIR. Outputs stdout.
run_audit_go()   { local dir=$1 ref=$2; shift 2; (cd "$dir" && "$GO_BIN"   audit-surface "$ref" "$@" 2>/dev/null); }
run_audit_node() { local dir=$1 ref=$2; shift 2; (cd "$dir" && node "$NODE_CLI" audit-surface "$ref" "$@" 2>/dev/null); }
run_audit_py()   { local dir=$1 ref=$2; shift 2; (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw audit-surface "$ref" "$@" 2>/dev/null); }

# assert_parity LABEL DIR REF [extra args...]
# Asserts that all three CLIs produce byte-identical output.
assert_parity() {
  local label=$1 dir=$2 ref=$3
  shift 3
  local go_out node_out py_out
  go_out=$(run_audit_go   "$dir" "$ref" "$@")
  node_out=$(run_audit_node "$dir" "$ref" "$@")
  py_out=$(run_audit_py   "$dir" "$ref" "$@")

  if [[ "$go_out" != "$node_out" ]]; then
    fail "$label/go-vs-node" "Go and Node.js outputs differ"
  fi
  if [[ "$go_out" != "$py_out" ]]; then
    fail "$label/go-vs-py" "Go and Python outputs differ"
  fi
}

# ===========================================================================
# FN-1 — Hook wiring present → reported
# ===========================================================================
FN1="$WORK/fn1"
mkdir -p "$FN1/.claude" "$FN1/scripts"
make_repo "$FN1"

echo '#!/usr/bin/env bash' > "$FN1/scripts/hook.sh"
echo 'echo hook-fn1' >> "$FN1/scripts/hook.sh"
cat > "$FN1/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/hook.sh", "type": "command"}]
      }
    ]
  }
}
EOF

git -C "$FN1" add -A
git -C "$FN1" commit -q -m "init: hook wiring"

FN1_OUT=$(run_audit_go "$FN1" HEAD)

# Vacuity
if [[ -z "$FN1_OUT" ]]; then
  fail "audit-surface/fn-1/vacuity" "output is empty — vacuity check failed"
fi
# Semantic
if ! grep -qF 'hook [claude]' <<<"$FN1_OUT"; then
  fail "audit-surface/fn-1/hook-reported" "expected 'hook [claude]' in output but got: $FN1_OUT"
fi
ok "audit-surface/fn-1/hook-reported"

assert_parity "audit-surface/fn-1" "$FN1" HEAD
ok "audit-surface/fn-1/parity"

# ===========================================================================
# FN-2 — Only the script changes, wiring intact → digest changes (AC2/AC14)
# Uses EVAL_BIN_FN2 for the semantic check so that SELFTEST_BREAK=A sabotage
# (constant digest) causes this specific assertion to fail.
# ===========================================================================
FN2="$WORK/fn2"
mkdir -p "$FN2/.claude" "$FN2/scripts"
make_repo "$FN2"

cat > "$FN2/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/hook.sh", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "hook version 1"\n' > "$FN2/scripts/hook.sh"

git -C "$FN2" add -A
git -C "$FN2" commit -q -m "init: wiring + hook v1"
FN2_REF1=$(git -C "$FN2" rev-parse HEAD)

# Change ONLY the script — settings.json untouched
printf '#!/usr/bin/env bash\necho "hook version 2 — different content"\n' > "$FN2/scripts/hook.sh"

git -C "$FN2" add scripts/hook.sh
git -C "$FN2" commit -q -m "update script only — wiring unchanged"
FN2_REF2=$(git -C "$FN2" rev-parse HEAD)

# Companion guard: settings.json must be identical between refs
FN2_SETTINGS_DIFF=$(git -C "$FN2" diff "$FN2_REF1" "$FN2_REF2" -- .claude/settings.json)
if [[ -n "$FN2_SETTINGS_DIFF" ]]; then
  fail "audit-surface/fn-2/settings-unchanged" \
    "settings.json should not differ between refs — fixture setup error"
fi

# Semantic: digests must differ (use EVAL_BIN_FN2 for sabotage seam)
FN2_DIGEST1=$(cd "$FN2" && "$EVAL_BIN_FN2" audit-surface "$FN2_REF1" 2>/dev/null \
              | grep 'hook \[claude\]' | awk '{print $NF}')
FN2_DIGEST2=$(cd "$FN2" && "$EVAL_BIN_FN2" audit-surface "$FN2_REF2" 2>/dev/null \
              | grep 'hook \[claude\]' | awk '{print $NF}')

if [[ -z "$FN2_DIGEST1" ]]; then
  fail "audit-surface/fn-2/vacuity-ref1" \
    "hook [claude] not found in output at ref1 — vacuity check failed"
fi
if [[ -z "$FN2_DIGEST2" ]]; then
  fail "audit-surface/fn-2/vacuity-ref2" \
    "hook [claude] not found in output at ref2 — vacuity check failed"
fi
if [[ "$FN2_DIGEST1" == "$FN2_DIGEST2" ]]; then
  fail "audit-surface/fn-2/digest-changes-when-script-changes" \
    "digest did not change between ref1 and ref2: both are $FN2_DIGEST1"
fi
ok "audit-surface/fn-2/digest-changes-when-script-changes"

assert_parity "audit-surface/fn-2" "$FN2" "$FN2_REF2"
ok "audit-surface/fn-2/parity"

# ===========================================================================
# FN-3 — Hook in runtime path that was absent → still scanned/reported (AC13)
# Uses Codex (same JSON schema as Claude, wiring file .codex/hooks.json).
# ===========================================================================
FN3="$WORK/fn3"
mkdir -p "$FN3/.codex" "$FN3/scripts"
make_repo "$FN3"

# Only Codex is wired — Claude wiring file is absent by design
printf '#!/usr/bin/env bash\necho "codex-hook"\n' > "$FN3/scripts/hook.sh"
cat > "$FN3/.codex/hooks.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/hook.sh", "type": "command"}]
      }
    ]
  }
}
EOF

git -C "$FN3" add -A
git -C "$FN3" commit -q -m "init: codex hook only (claude path absent)"

FN3_OUT=$(run_audit_go "$FN3" HEAD)

# Vacuity
if [[ -z "$FN3_OUT" ]]; then
  fail "audit-surface/fn-3/vacuity" "output is empty — vacuity check failed"
fi
# Semantic: codex hook reported
if ! grep -qF 'hook [codex]' <<<"$FN3_OUT"; then
  fail "audit-surface/fn-3/hook-reported" "expected 'hook [codex]' in output but got: $FN3_OUT"
fi
# AC13: absence of claude path is also reported (absence is information)
if ! grep -qF 'absent [claude]' <<<"$FN3_OUT"; then
  fail "audit-surface/fn-3/absent-reported" \
    "expected 'absent [claude]' in output (AC13: absent path is information) but got: $FN3_OUT"
fi
ok "audit-surface/fn-3/hook-reported"
ok "audit-surface/fn-3/absent-reported"

assert_parity "audit-surface/fn-3" "$FN3" HEAD
ok "audit-surface/fn-3/parity"

# ===========================================================================
# FN-4 — Matcher widens "Bash" → "*" → tuple changes (AC14)
# ===========================================================================
FN4="$WORK/fn4"
mkdir -p "$FN4/.claude" "$FN4/scripts"
make_repo "$FN4"

printf '#!/usr/bin/env bash\necho "fn4-hook"\n' > "$FN4/scripts/hook.sh"
cat > "$FN4/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/hook.sh", "type": "command"}]
      }
    ]
  }
}
EOF

git -C "$FN4" add -A
git -C "$FN4" commit -q -m "init: matcher=Bash"
FN4_REF1=$(git -C "$FN4" rev-parse HEAD)

# Change ONLY the matcher — same command, same script
cat > "$FN4/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [{"command": "scripts/hook.sh", "type": "command"}]
      }
    ]
  }
}
EOF

git -C "$FN4" add .claude/settings.json
git -C "$FN4" commit -q -m "widen matcher: Bash -> *"
FN4_REF2=$(git -C "$FN4" rev-parse HEAD)

FN4_OUT1=$(run_audit_go "$FN4" "$FN4_REF1")
FN4_OUT2=$(run_audit_go "$FN4" "$FN4_REF2")

# Vacuity at both refs
if ! grep -qF 'hook [claude]' <<<"$FN4_OUT1"; then
  fail "audit-surface/fn-4/vacuity-ref1" "hook [claude] not found at ref1 — vacuity failed"
fi
if ! grep -qF 'hook [claude]' <<<"$FN4_OUT2"; then
  fail "audit-surface/fn-4/vacuity-ref2" "hook [claude] not found at ref2 — vacuity failed"
fi
# Semantic: matcher "Bash" at ref1
if ! grep -qF 'PreToolUse/Bash' <<<"$FN4_OUT1"; then
  fail "audit-surface/fn-4/matcher-bash" "expected 'PreToolUse/Bash' at ref1 but got: $FN4_OUT1"
fi
# Semantic: matcher "*" at ref2 (formatted as PreToolUse/*)
if ! grep -qF 'PreToolUse/*' <<<"$FN4_OUT2"; then
  fail "audit-surface/fn-4/matcher-wildcard" "expected 'PreToolUse/*' at ref2 but got: $FN4_OUT2"
fi
# Tuples must differ (AC14)
if [[ "$FN4_OUT1" == "$FN4_OUT2" ]]; then
  fail "audit-surface/fn-4/tuples-differ" \
    "outputs at ref1 and ref2 are identical — matcher change not detected"
fi
ok "audit-surface/fn-4/matcher-change-detected"

assert_parity "audit-surface/fn-4" "$FN4" "$FN4_REF2"
ok "audit-surface/fn-4/parity"

# ===========================================================================
# FN-5 — Instruction file present → reported with "instruction" label (AC15)
# ===========================================================================
FN5="$WORK/fn5"
mkdir -p "$FN5"
make_repo "$FN5"

cat > "$FN5/CLAUDE.md" << 'EOF'
# CLAUDE.md — test instruction file
This is a test project instruction file for FN-5.
EOF

git -C "$FN5" add -A
git -C "$FN5" commit -q -m "init: instruction file CLAUDE.md"

FN5_OUT=$(run_audit_go "$FN5" HEAD)

# Vacuity
if [[ -z "$FN5_OUT" ]]; then
  fail "audit-surface/fn-5/vacuity" "output is empty — vacuity check failed"
fi
# Semantic: instruction file reported with "instruction [present]" label (AC15)
if ! grep -qF 'instruction [present] CLAUDE.md' <<<"$FN5_OUT"; then
  fail "audit-surface/fn-5/instruction-reported" \
    "expected 'instruction [present] CLAUDE.md' in output but got: $FN5_OUT"
fi
ok "audit-surface/fn-5/instruction-reported"

assert_parity "audit-surface/fn-5" "$FN5" HEAD
ok "audit-surface/fn-5/parity"

# ===========================================================================
# FP-1 — docs/cli-parity.md NOT in audit output (AC16, free fixture: real repo)
# Uses EVAL_BIN_FP1 for the semantic check so that SELFTEST_BREAK=B sabotage
# (docs/cli-parity.md added to instructionFilePaths) causes this assertion to fail.
# ===========================================================================
FP1_OUT=$(cd "$ROOT_DIR" && "$EVAL_BIN_FP1" audit-surface HEAD 2>/dev/null)

# Vacuity: real repo must have at least one hook (proving output is non-empty and meaningful)
if ! grep -qF 'hook [claude]' <<<"$FP1_OUT"; then
  fail "audit-surface/fp-1/vacuity" \
    "expected 'hook [claude]' in real-repo output — vacuity check failed (repo should have hook wired)"
fi
# Semantic: docs/cli-parity.md must NOT appear (AC16)
if grep -qF 'docs/cli-parity.md' <<<"$FP1_OUT"; then
  fail "audit-surface/fp-1/cli-parity-absent" \
    "docs/cli-parity.md appeared in audit-surface output"
fi
ok "audit-surface/fp-1/cli-parity-absent"

# Parity for FP-1 uses the real GO_BIN (not the seam binary)
FP1_OUT_GO=$(run_audit_go     "$ROOT_DIR" HEAD)
FP1_OUT_NODE=$(run_audit_node "$ROOT_DIR" HEAD)
FP1_OUT_PY=$(run_audit_py     "$ROOT_DIR" HEAD)
if [[ "$FP1_OUT_GO" != "$FP1_OUT_NODE" ]]; then
  fail "audit-surface/fp-1/go-vs-node" "Go and Node.js outputs differ"
fi
if [[ "$FP1_OUT_GO" != "$FP1_OUT_PY" ]]; then
  fail "audit-surface/fp-1/go-vs-py" "Go and Python outputs differ"
fi
ok "audit-surface/fp-1/parity"

# ===========================================================================
# FP-2 — internal/generators/agentfiles.go NOT in audit output (AC16)
# Reuses the real-binary output from FP-1.
# ===========================================================================
if grep -qF 'internal/generators/agentfiles.go' <<<"$FP1_OUT_GO"; then
  fail "audit-surface/fp-2/agentfiles-absent" \
    "internal/generators/agentfiles.go appeared in audit-surface output"
fi
ok "audit-surface/fp-2/agentfiles-absent"

echo "check-audit-surface: all 7 scenarios passed (FN-1..5, FP-1..2)"
