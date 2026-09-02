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

# ===========================================================================
# FN-F1a — F1 fix: .bash extension — digest changes when script changes (AC14)
# ===========================================================================
FNF1A="$WORK/fn-f1a"
mkdir -p "$FNF1A/.claude" "$FNF1A/scripts"
make_repo "$FNF1A"

cat > "$FNF1A/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/hook.bash", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "hook-f1a-v1"\n' > "$FNF1A/scripts/hook.bash"
git -C "$FNF1A" add -A
git -C "$FNF1A" commit -q -m "init: .bash extension"
FNF1A_REF1=$(git -C "$FNF1A" rev-parse HEAD)

printf '#!/usr/bin/env bash\necho "hook-f1a-v2-HOSTILE"\n' > "$FNF1A/scripts/hook.bash"
git -C "$FNF1A" add scripts/hook.bash
git -C "$FNF1A" commit -q -m "script change only"
FNF1A_REF2=$(git -C "$FNF1A" rev-parse HEAD)

# Companion guard: settings.json must be identical between refs.
if [[ -n "$(git -C "$FNF1A" diff "$FNF1A_REF1" "$FNF1A_REF2" -- .claude/settings.json)" ]]; then
  fail "audit-surface/fn-f1a/settings-unchanged" "settings.json should not differ — fixture setup error"
fi
FNF1A_D1=$(cd "$FNF1A" && "$GO_BIN" audit-surface "$FNF1A_REF1" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
FNF1A_D2=$(cd "$FNF1A" && "$GO_BIN" audit-surface "$FNF1A_REF2" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
if [[ -z "$FNF1A_D1" ]]; then
  fail "audit-surface/fn-f1a/vacuity" "hook [claude] not found at ref1 — vacuity check failed"
fi
if [[ "$FNF1A_D1" == "$FNF1A_D2" ]]; then
  fail "audit-surface/fn-f1a/digest-changes" "digest did not change for .bash extension: both are $FNF1A_D1"
fi
ok "audit-surface/fn-f1a/digest-changes"
assert_parity "audit-surface/fn-f1a" "$FNF1A" "$FNF1A_REF2"
ok "audit-surface/fn-f1a/parity"

# ===========================================================================
# FN-F1b — F1 fix: command with arguments — digest changes when script changes
# ===========================================================================
FNF1B="$WORK/fn-f1b"
mkdir -p "$FNF1B/.claude" "$FNF1B/scripts"
make_repo "$FNF1B"

cat > "$FNF1B/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/hook.sh --strict", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "hook-f1b-v1"\n' > "$FNF1B/scripts/hook.sh"
git -C "$FNF1B" add -A
git -C "$FNF1B" commit -q -m "init: command with args"
FNF1B_REF1=$(git -C "$FNF1B" rev-parse HEAD)

printf '#!/usr/bin/env bash\necho "hook-f1b-v2-HOSTILE"\n' > "$FNF1B/scripts/hook.sh"
git -C "$FNF1B" add scripts/hook.sh
git -C "$FNF1B" commit -q -m "script change only"
FNF1B_REF2=$(git -C "$FNF1B" rev-parse HEAD)

if [[ -n "$(git -C "$FNF1B" diff "$FNF1B_REF1" "$FNF1B_REF2" -- .claude/settings.json)" ]]; then
  fail "audit-surface/fn-f1b/settings-unchanged" "settings.json should not differ — fixture setup error"
fi
FNF1B_D1=$(cd "$FNF1B" && "$GO_BIN" audit-surface "$FNF1B_REF1" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
FNF1B_D2=$(cd "$FNF1B" && "$GO_BIN" audit-surface "$FNF1B_REF2" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
if [[ -z "$FNF1B_D1" ]]; then
  fail "audit-surface/fn-f1b/vacuity" "hook [claude] not found at ref1 — vacuity check failed"
fi
if [[ "$FNF1B_D1" == "$FNF1B_D2" ]]; then
  fail "audit-surface/fn-f1b/digest-changes" "digest did not change for command-with-args: both are $FNF1B_D1"
fi
ok "audit-surface/fn-f1b/digest-changes"
assert_parity "audit-surface/fn-f1b" "$FNF1B" "$FNF1B_REF2"
ok "audit-surface/fn-f1b/parity"

# ===========================================================================
# FN-F1c — F1 fix: interpreter prefix — digest changes when script changes
# ===========================================================================
FNF1C="$WORK/fn-f1c"
mkdir -p "$FNF1C/.claude" "$FNF1C/scripts"
make_repo "$FNF1C"

cat > "$FNF1C/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "bash scripts/hook.sh", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "hook-f1c-v1"\n' > "$FNF1C/scripts/hook.sh"
git -C "$FNF1C" add -A
git -C "$FNF1C" commit -q -m "init: interpreter prefix"
FNF1C_REF1=$(git -C "$FNF1C" rev-parse HEAD)

printf '#!/usr/bin/env bash\necho "hook-f1c-v2-HOSTILE"\n' > "$FNF1C/scripts/hook.sh"
git -C "$FNF1C" add scripts/hook.sh
git -C "$FNF1C" commit -q -m "script change only"
FNF1C_REF2=$(git -C "$FNF1C" rev-parse HEAD)

if [[ -n "$(git -C "$FNF1C" diff "$FNF1C_REF1" "$FNF1C_REF2" -- .claude/settings.json)" ]]; then
  fail "audit-surface/fn-f1c/settings-unchanged" "settings.json should not differ — fixture setup error"
fi
FNF1C_D1=$(cd "$FNF1C" && "$GO_BIN" audit-surface "$FNF1C_REF1" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
FNF1C_D2=$(cd "$FNF1C" && "$GO_BIN" audit-surface "$FNF1C_REF2" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
if [[ -z "$FNF1C_D1" ]]; then
  fail "audit-surface/fn-f1c/vacuity" "hook [claude] not found at ref1 — vacuity check failed"
fi
if [[ "$FNF1C_D1" == "$FNF1C_D2" ]]; then
  fail "audit-surface/fn-f1c/digest-changes" "digest did not change for interpreter-prefix: both are $FNF1C_D1"
fi
ok "audit-surface/fn-f1c/digest-changes"
assert_parity "audit-surface/fn-f1c" "$FNF1C" "$FNF1C_REF2"
ok "audit-surface/fn-f1c/parity"

# ===========================================================================
# FN-F2 — F2 fix: symlink as script — digest follows real content (AC14)
# ===========================================================================
FNF2="$WORK/fn-f2"
mkdir -p "$FNF2/.claude" "$FNF2/scripts"
make_repo "$FNF2"

cat > "$FNF2/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/link.sh", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "real-v1"\n' > "$FNF2/scripts/real.sh"
# Create symlink: scripts/link.sh -> real.sh (relative target)
ln -s real.sh "$FNF2/scripts/link.sh"

git -C "$FNF2" add -A
git -C "$FNF2" commit -q -m "init: symlink scripts/link.sh -> real.sh"
FNF2_REF1=$(git -C "$FNF2" rev-parse HEAD)

# Change ONLY real.sh — link.sh symlink and settings.json are untouched.
printf '#!/usr/bin/env bash\necho "real-v2-HOSTILE"\n' > "$FNF2/scripts/real.sh"
git -C "$FNF2" add scripts/real.sh
git -C "$FNF2" commit -q -m "change real.sh only — symlink unchanged"
FNF2_REF2=$(git -C "$FNF2" rev-parse HEAD)

# Companion guard: settings.json and the symlink entry must be identical between refs.
if [[ -n "$(git -C "$FNF2" diff "$FNF2_REF1" "$FNF2_REF2" -- .claude/settings.json scripts/link.sh)" ]]; then
  fail "audit-surface/fn-f2/symlink-wiring-unchanged" "settings.json or link.sh should not differ — fixture setup error"
fi

FNF2_OUT1=$(cd "$FNF2" && "$GO_BIN" audit-surface "$FNF2_REF1" 2>/dev/null)
FNF2_D1=$(grep 'hook \[claude\]' <<<"$FNF2_OUT1" | awk '{print $NF}')
FNF2_D2=$(cd "$FNF2" && "$GO_BIN" audit-surface "$FNF2_REF2" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')

if [[ -z "$FNF2_D1" ]]; then
  fail "audit-surface/fn-f2/vacuity" "hook [claude] not found at ref1 — vacuity check failed"
fi
# Symlink must be detected: digest field must contain "symlink->".
FNF2_HOOK_LINE=$(grep 'hook \[claude\]' <<<"$FNF2_OUT1")
if ! grep -qF 'symlink->' <<<"$FNF2_HOOK_LINE"; then
  fail "audit-surface/fn-f2/symlink-reported" "expected 'symlink->' in digest for symlink script, got: $FNF2_HOOK_LINE"
fi
ok "audit-surface/fn-f2/symlink-reported"
# Digest must change when real.sh content changes.
if [[ "$FNF2_D1" == "$FNF2_D2" ]]; then
  fail "audit-surface/fn-f2/digest-changes" "digest did not change when symlink target content changed: both are $FNF2_D1"
fi
ok "audit-surface/fn-f2/digest-changes"

assert_parity "audit-surface/fn-f2" "$FNF2" "$FNF2_REF2"
ok "audit-surface/fn-f2/parity"

# ===========================================================================
# FN-F3 — F3 fix: invalid ref → nonzero exit, empty stdout
# ===========================================================================
FNF3="$WORK/fn-f3"
mkdir -p "$FNF3"
make_repo "$FNF3"
printf "placeholder\n" > "$FNF3/placeholder.txt"
git -C "$FNF3" add -A
git -C "$FNF3" commit -q -m "init: placeholder"

# 40-hex SHA that is NOT an object in this fixture repo.
FAKE_SHA="0000000000000000000000000000000000000042"

FNF3_GO_EXIT=0
(cd "$FNF3" && "$GO_BIN" audit-surface "$FAKE_SHA") >/dev/null 2>&1 || FNF3_GO_EXIT=$?
if [[ "$FNF3_GO_EXIT" -eq 0 ]]; then
  fail "audit-surface/fn-f3/invalid-ref-go" "expected nonzero exit for invalid ref, got 0"
fi
ok "audit-surface/fn-f3/invalid-ref-go"

FNF3_NODE_EXIT=0
(cd "$FNF3" && node "$NODE_CLI" audit-surface "$FAKE_SHA") >/dev/null 2>&1 || FNF3_NODE_EXIT=$?
if [[ "$FNF3_NODE_EXIT" -eq 0 ]]; then
  fail "audit-surface/fn-f3/invalid-ref-node" "expected nonzero exit for invalid ref, got 0"
fi
ok "audit-surface/fn-f3/invalid-ref-node"

FNF3_PY_EXIT=0
(cd "$FNF3" && PYTHONPATH="$PY_ROOT" python3 -m trackfw audit-surface "$FAKE_SHA") >/dev/null 2>&1 || FNF3_PY_EXIT=$?
if [[ "$FNF3_PY_EXIT" -eq 0 ]]; then
  fail "audit-surface/fn-f3/invalid-ref-py" "expected nonzero exit for invalid ref, got 0"
fi
ok "audit-surface/fn-f3/invalid-ref-py"

# Stdout must be empty on invalid ref (error output goes to stderr).
FNF3_STDOUT=""
FNF3_STDOUT=$((cd "$FNF3" && "$GO_BIN" audit-surface "$FAKE_SHA" 2>/dev/null) || true)
if [[ -n "$FNF3_STDOUT" ]]; then
  fail "audit-surface/fn-f3/no-stdout-on-error" "expected empty stdout for invalid ref, got: $FNF3_STDOUT"
fi
ok "audit-surface/fn-f3/no-stdout-on-error"

# ===========================================================================
# FN-R3-chain — R3 fix: 2-level symlink chain — digest follows real content
# Fixture: link.sh → middle.sh → real.sh (two hops)
# ===========================================================================
FNR3C="$WORK/fn-r3-chain"
mkdir -p "$FNR3C/.claude" "$FNR3C/scripts"
make_repo "$FNR3C"

cat > "$FNR3C/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/link.sh", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "real-v1"\n' > "$FNR3C/scripts/real.sh"
# middle.sh is a symlink → real.sh; link.sh is a symlink → middle.sh
ln -s real.sh "$FNR3C/scripts/middle.sh"
ln -s middle.sh "$FNR3C/scripts/link.sh"

git -C "$FNR3C" add -A
git -C "$FNR3C" commit -q -m "init: chain link.sh -> middle.sh -> real.sh"
FNR3C_REF1=$(git -C "$FNR3C" rev-parse HEAD)

# Change ONLY real.sh — the two symlinks are untouched.
printf '#!/usr/bin/env bash\necho "real-v2-HOSTILE"\n' > "$FNR3C/scripts/real.sh"
git -C "$FNR3C" add scripts/real.sh
git -C "$FNR3C" commit -q -m "change real.sh only — symlink chain unchanged"
FNR3C_REF2=$(git -C "$FNR3C" rev-parse HEAD)

# Companion guard: settings.json and the two symlinks must be identical between refs.
if [[ -n "$(git -C "$FNR3C" diff "$FNR3C_REF1" "$FNR3C_REF2" -- .claude/settings.json scripts/link.sh scripts/middle.sh)" ]]; then
  fail "audit-surface/fn-r3-chain/wiring-unchanged" "settings.json or symlinks should not differ — fixture setup error"
fi

FNR3C_OUT1=$(cd "$FNR3C" && "$GO_BIN" audit-surface "$FNR3C_REF1" 2>/dev/null)
FNR3C_D1=$(grep 'hook \[claude\]' <<<"$FNR3C_OUT1" | awk '{print $NF}')

# Vacuity guard 1: hook line must exist (chain was detected).
if [[ -z "$FNR3C_D1" ]]; then
  fail "audit-surface/fn-r3-chain/vacuity" "hook [claude] not found at ref1 — vacuity check failed"
fi

# Vacuity guard 2: the digest must actually contain sha256 (resolved to real content).
if ! grep -qF 'sha256:' <<<"$FNR3C_D1"; then
  fail "audit-surface/fn-r3-chain/sha256-present" "digest at ref1 does not contain sha256 — resolution failed: $FNR3C_D1"
fi
ok "audit-surface/fn-r3-chain/sha256-present"

FNR3C_D2=$(cd "$FNR3C" && "$GO_BIN" audit-surface "$FNR3C_REF2" 2>/dev/null | grep 'hook \[claude\]' | awk '{print $NF}')
if [[ "$FNR3C_D1" == "$FNR3C_D2" ]]; then
  fail "audit-surface/fn-r3-chain/digest-changes" "digest did not change when real.sh changed through 2-level chain: both are $FNR3C_D1"
fi
ok "audit-surface/fn-r3-chain/digest-changes"

assert_parity "audit-surface/fn-r3-chain" "$FNR3C" "$FNR3C_REF2"
ok "audit-surface/fn-r3-chain/parity"

# ===========================================================================
# FN-R3-cycle — R3/R4 fix: circular symlink → circular-not-supported, no sha256
# Fixture: link.sh → link.sh (self-loop)
# ===========================================================================
FNR3CY="$WORK/fn-r3-cycle"
mkdir -p "$FNR3CY/.claude" "$FNR3CY/scripts"
make_repo "$FNR3CY"

cat > "$FNR3CY/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/link.sh", "type": "command"}]
      }
    ]
  }
}
EOF
# Circular symlink: link.sh → link.sh
ln -s link.sh "$FNR3CY/scripts/link.sh"

git -C "$FNR3CY" add -A
git -C "$FNR3CY" commit -q -m "init: circular symlink link.sh -> link.sh"
FNR3CY_REF=$(git -C "$FNR3CY" rev-parse HEAD)

FNR3CY_OUT=$(cd "$FNR3CY" && "$GO_BIN" audit-surface "$FNR3CY_REF" 2>/dev/null)
FNR3CY_LINE=$(grep 'hook \[claude\]' <<<"$FNR3CY_OUT")

# Vacuity guard 1: the hook line must appear (the script path is still reported).
if [[ -z "$FNR3CY_LINE" ]]; then
  fail "audit-surface/fn-r3-cycle/vacuity" "hook [claude] not found in output — vacuity check failed"
fi

# Vacuity guard 2: must NOT contain sha256 (we must not hash fake content).
if grep -qF 'sha256:' <<<"$FNR3CY_LINE"; then
  fail "audit-surface/fn-r3-cycle/no-sha256" "output must not contain sha256 for circular symlink, got: $FNR3CY_LINE"
fi
ok "audit-surface/fn-r3-cycle/no-sha256"

if ! grep -qF 'circular-not-supported' <<<"$FNR3CY_LINE"; then
  fail "audit-surface/fn-r3-cycle/circular-reported" "expected 'circular-not-supported' in digest, got: $FNR3CY_LINE"
fi
ok "audit-surface/fn-r3-cycle/circular-reported"

assert_parity "audit-surface/fn-r3-cycle" "$FNR3CY" "$FNR3CY_REF"
ok "audit-surface/fn-r3-cycle/parity"

# ===========================================================================
# FN-R3-depth — depth limit exceeded → chain-not-supported, no sha256
# Fixture: 9-hop chain (a→b→c→d→e→f→g→h→i→real, exceeds limit of 8)
# ===========================================================================
FNR3D="$WORK/fn-r3-depth"
mkdir -p "$FNR3D/.claude" "$FNR3D/scripts"
make_repo "$FNR3D"

cat > "$FNR3D/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"command": "scripts/a.sh", "type": "command"}]
      }
    ]
  }
}
EOF
printf '#!/usr/bin/env bash\necho "real"\n' > "$FNR3D/scripts/real.sh"
# Build the chain: i→real, h→i, ..., b→c, a→b  (9 symlinks before the real file)
ln -s real.sh "$FNR3D/scripts/i.sh"
ln -s i.sh    "$FNR3D/scripts/h.sh"
ln -s h.sh    "$FNR3D/scripts/g.sh"
ln -s g.sh    "$FNR3D/scripts/f.sh"
ln -s f.sh    "$FNR3D/scripts/e.sh"
ln -s e.sh    "$FNR3D/scripts/d.sh"
ln -s d.sh    "$FNR3D/scripts/c.sh"
ln -s c.sh    "$FNR3D/scripts/b.sh"
ln -s b.sh    "$FNR3D/scripts/a.sh"

git -C "$FNR3D" add -A
git -C "$FNR3D" commit -q -m "init: 9-hop symlink chain a→…→real"
FNR3D_REF=$(git -C "$FNR3D" rev-parse HEAD)

FNR3D_OUT=$(cd "$FNR3D" && "$GO_BIN" audit-surface "$FNR3D_REF" 2>/dev/null)
FNR3D_LINE=$(grep 'hook \[claude\]' <<<"$FNR3D_OUT")

# Vacuity guard 1: the hook line must appear.
if [[ -z "$FNR3D_LINE" ]]; then
  fail "audit-surface/fn-r3-depth/vacuity" "hook [claude] not found in output — vacuity check failed"
fi

# Vacuity guard 2: must NOT contain sha256.
if grep -qF 'sha256:' <<<"$FNR3D_LINE"; then
  fail "audit-surface/fn-r3-depth/no-sha256" "output must not contain sha256 for depth-exceeded chain, got: $FNR3D_LINE"
fi
ok "audit-surface/fn-r3-depth/no-sha256"

if ! grep -qF 'chain-not-supported' <<<"$FNR3D_LINE"; then
  fail "audit-surface/fn-r3-depth/chain-reported" "expected 'chain-not-supported' in digest, got: $FNR3D_LINE"
fi
ok "audit-surface/fn-r3-depth/chain-reported"

assert_parity "audit-surface/fn-r3-depth" "$FNR3D" "$FNR3D_REF"
ok "audit-surface/fn-r3-depth/parity"

# ===========================================================================
# FN-R3-nonreg — non-regression: absolute target → not-supported,
#                                absent target   → not-found  (1-level, ML-1B)
# ===========================================================================
FNR3NR="$WORK/fn-r3-nonreg"
mkdir -p "$FNR3NR/.claude" "$FNR3NR/scripts"
make_repo "$FNR3NR"

cat > "$FNR3NR/.claude/settings.json" << 'EOF'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "scripts/abs.sh", "type": "command"},
          {"command": "scripts/miss.sh", "type": "command"}
        ]
      }
    ]
  }
}
EOF
# abs.sh → /etc/passwd (absolute target)
ln -s /etc/passwd "$FNR3NR/scripts/abs.sh"
# miss.sh → nonexistent.sh (absent target)
ln -s nonexistent.sh "$FNR3NR/scripts/miss.sh"

git -C "$FNR3NR" add -A
git -C "$FNR3NR" commit -q -m "init: absolute and absent symlink targets"
FNR3NR_REF=$(git -C "$FNR3NR" rev-parse HEAD)

FNR3NR_OUT=$(cd "$FNR3NR" && "$GO_BIN" audit-surface "$FNR3NR_REF" 2>/dev/null)

# Vacuity guard: both hook lines must appear.
if ! grep -qF 'scripts/abs.sh' <<<"$FNR3NR_OUT"; then
  fail "audit-surface/fn-r3-nonreg/vacuity-abs" "scripts/abs.sh not found in output — vacuity check failed"
fi
if ! grep -qF 'scripts/miss.sh' <<<"$FNR3NR_OUT"; then
  fail "audit-surface/fn-r3-nonreg/vacuity-miss" "scripts/miss.sh not found in output — vacuity check failed"
fi

FNR3NR_ABS_LINE=$(grep 'scripts/abs.sh' <<<"$FNR3NR_OUT")
FNR3NR_MISS_LINE=$(grep 'scripts/miss.sh' <<<"$FNR3NR_OUT")

if ! grep -qF 'not-supported' <<<"$FNR3NR_ABS_LINE"; then
  fail "audit-surface/fn-r3-nonreg/abs-not-supported" "expected 'not-supported' for absolute symlink, got: $FNR3NR_ABS_LINE"
fi
ok "audit-surface/fn-r3-nonreg/abs-not-supported"

if ! grep -qF 'not-found' <<<"$FNR3NR_MISS_LINE"; then
  fail "audit-surface/fn-r3-nonreg/miss-not-found" "expected 'not-found' for absent symlink target, got: $FNR3NR_MISS_LINE"
fi
ok "audit-surface/fn-r3-nonreg/miss-not-found"

assert_parity "audit-surface/fn-r3-nonreg" "$FNR3NR" "$FNR3NR_REF"
ok "audit-surface/fn-r3-nonreg/parity"

echo "check-audit-surface: all 21 scenarios passed (FN-1..5, FP-1..2, FN-F1a/b/c, FN-F2, FN-F3, FN-R3-chain, FN-R3-cycle, FN-R3-depth, FN-R3-nonreg)"
