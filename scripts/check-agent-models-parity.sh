#!/usr/bin/env bash
# check-agent-models-parity.sh — proves the three CLI runtimes (Go, Node.js, Python)
# implement `agent_models` composition identically, and that the namespace
# boundary ("only the claude target receives composed model IDs") is enforced.
#
# Contract frozen in docs/cli-parity.md ("## `agent_models` — version composition
# and namespace boundary").  Closes ML-3A of
# ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
#
# Four cases, comparing **real outputs** (generated agent files):
#
#   Case 1 — Composition
#     With agent_models: {sonnet: "4.6", opus: "5"} in trackfw.yaml, the Claude
#     target generates:
#       architect → model: claude-opus-5   (opus tier, major-only)
#       backend   → model: claude-sonnet-4-6  (sonnet tier, ponto→traço)
#     All three runtimes must produce byte-identical files.
#
#   Case 2 — No namespace leak  ← the most important case
#     Codex and Gemini output must be byte-identical whether or not agent_models
#     is configured.  This is NOT a cross-runtime comparison: a leak that hits
#     all three runtimes identically would pass a cross-runtime check but still
#     break every user.  The correct axis is with-config vs without-config,
#     done independently per runtime.
#
#   Case 3 — Absent config
#     Without agent_models, Claude agents keep the canonical tier alias
#     (model: sonnet, model: opus) unchanged.  Cross-runtime comparison to
#     guard against regression to all users who don't set agent_models.
#
#   Case 4 — Escape hatch
#     A value that contains hyphens (e.g. "claude-sonnet-4-5-20250929") is
#     not a version string and must be written literally.  Cross-runtime
#     comparison.
#
# Follows the conventions of check-update-parity.sh, check-identity-parity.sh:
#   set -euo pipefail, mktemp -d fixtures with cleanup trap, HOME redirected
#   on every invocation (agents install writes into the user's home), absolute
#   GO_BIN, vacuity guard before every comparison, OK/FAIL helpers, FAIL
#   accumulator so all failures surface before exit.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
# Guarantee absolute path — Makefile may pass a relative path (e.g. bin/trackfw)
# that becomes invalid when subshells cd into WORK subdirectories.
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-agent-models-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="$ROOT_DIR/pypi"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-agent-models-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# $HOME is isolated for every invocation — agents install can write into the
# user's home directory (global scope, manifest). Same precedent as
# check-artifact-parity.sh lines 29-41 and check-update-parity.sh line 74-78.
# Using the real $HOME would mix in the guard warnings from the user's real
# ~/.trackfw/scripts/ installation and make the gate flaky.
export HOME="$WORK/home-global"
mkdir -p "$HOME"

FAIL=0
ok()   { echo "OK   [$1]"; }
diag() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# run_install RUNTIME PROJECT_DIR TARGET
# Installs TARGET (agents scope=project) using the given runtime. PROJECT_DIR
# must already contain a valid trackfw.yaml. Exits the whole script on
# non-zero to surface fixture problems immediately.
# ---------------------------------------------------------------------------
run_install() {
  local rt=$1 proj=$2 target=$3
  local home_dir="$WORK/home-$rt-${target//\//-}"
  mkdir -p "$home_dir"
  case "$rt" in
    go)   (cd "$proj" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --scope project >/dev/null 2>&1) ;;
    node) (cd "$proj" && HOME="$home_dir" node "$NODE_CLI" agents install --targets "$target" --scope project >/dev/null 2>&1) ;;
    py)   (cd "$proj" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" python3 -m trackfw agents install --targets "$target" --scope project >/dev/null 2>&1) ;;
    *)    echo "check-agent-models-parity: unknown runtime '$rt'" >&2; exit 1 ;;
  esac
}

# write_yaml PROJECT_DIR WITH_AGENT_MODELS
# Writes a minimal trackfw.yaml. When WITH_AGENT_MODELS=1, adds the agent_models
# stanza; otherwise omits it entirely.
write_yaml() {
  local proj=$1 with_models=${2:-0}
  mkdir -p "$proj"
  if [[ "$with_models" -eq 1 ]]; then
    cat >"$proj/trackfw.yaml" <<'YAML'
project_name: agent-models-parity-test
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
agent_models:
  sonnet: "4.6"
  opus: "5"
YAML
  else
    cat >"$proj/trackfw.yaml" <<'YAML'
project_name: agent-models-parity-test
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
YAML
  fi
}

write_yaml_escape_hatch() {
  local proj=$1
  mkdir -p "$proj"
  cat >"$proj/trackfw.yaml" <<'YAML'
project_name: agent-models-parity-test
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
agent_models:
  sonnet: "claude-sonnet-4-5-20250929"
  opus: "claude-opus-5"
YAML
}

# ===========================================================================
# Case 1 — Composition: claude target with agent_models, cross-runtime
#
# Each runtime installs into its own project directory (same config). We then
# compare the generated agent files byte-by-byte across the three runtimes.
#
# Vacuity guard: the architect file must contain "model: claude-opus-5" and
# the backend file must contain "model: claude-sonnet-4-6" in the Go output
# before any cross-runtime comparison — otherwise we could be comparing three
# empty or identical tier-alias files and incorrectly call it a pass.
# ===========================================================================
for rt in go node py; do
  proj="$WORK/case1-$rt"
  write_yaml "$proj" 1
  run_install "$rt" "$proj" "claude"
done

# Vacuity guard
go_arch="$WORK/case1-go/.claude/agents/trackfw-architect.md"
go_back="$WORK/case1-go/.claude/agents/trackfw-backend.md"
if [[ ! -f "$go_arch" ]]; then
  diag "composition/vacuity-guard" "Go CLI did not generate .claude/agents/trackfw-architect.md — fixture broken"
elif ! grep -q 'model: claude-opus-5' "$go_arch"; then
  diag "composition/vacuity-guard" "Go architect does not contain 'model: claude-opus-5' — composition may be broken (got: $(grep 'model:' "$go_arch" || echo 'no model line'))"
else
  ok "composition/vacuity-guard/architect-opus"
fi

if [[ ! -f "$go_back" ]]; then
  diag "composition/vacuity-guard" "Go CLI did not generate .claude/agents/trackfw-backend.md — fixture broken"
elif ! grep -q 'model: claude-sonnet-4-6' "$go_back"; then
  diag "composition/vacuity-guard" "Go backend does not contain 'model: claude-sonnet-4-6' — composition may be broken (got: $(grep 'model:' "$go_back" || echo 'no model line'))"
else
  ok "composition/vacuity-guard/backend-sonnet"
fi

# Cross-runtime file comparison (12 agents × go-vs-node, go-vs-py)
if [[ $FAIL -eq 0 ]]; then
  COMP_FAIL=0
  for rel in $(find "$WORK/case1-go/.claude/agents" -name 'trackfw-*.md' -exec basename {} \; | sort); do
    go_f="$WORK/case1-go/.claude/agents/$rel"
    node_f="$WORK/case1-node/.claude/agents/$rel"
    py_f="$WORK/case1-py/.claude/agents/$rel"

    if [[ ! -f "$node_f" ]]; then
      diag "composition/cross-runtime" "Node missing: .claude/agents/$rel"; COMP_FAIL=1; continue
    fi
    if [[ ! -f "$py_f" ]]; then
      diag "composition/cross-runtime" "Python missing: .claude/agents/$rel"; COMP_FAIL=1; continue
    fi

    if ! cmp -s "$go_f" "$node_f"; then
      diag "composition/cross-runtime/go-vs-node" "$rel differs"
      diff "$go_f" "$node_f" >&2 || true
      COMP_FAIL=1
    fi
    if ! cmp -s "$go_f" "$py_f"; then
      diag "composition/cross-runtime/go-vs-python" "$rel differs"
      diff "$go_f" "$py_f" >&2 || true
      COMP_FAIL=1
    fi
  done
  if [[ $COMP_FAIL -eq 0 ]]; then
    ok "composition/cross-runtime/claude-12-agents-byte-identical"
  fi
fi

# ===========================================================================
# Case 2 — No namespace leak: per-runtime baseline vs candidate
#
# For each runtime independently: generate Codex and Gemini agent files WITHOUT
# agent_models (baseline), then WITH agent_models (candidate). Assert cmp equal.
#
# This is NOT a cross-runtime comparison. A leak that hits all three runtimes
# identically would produce three matching pairs and pass a cross-runtime gate
# — but it would still be wrong. The correct axis is with-config vs
# without-config, verified separately per runtime.
#
# Vacuity guards per runtime:
#   - Baseline Codex backend must contain 'model = "gpt-5.4-mini"'
#   - Baseline Gemini backend must contain 'model: sonnet'
# These guards fail the gate immediately if the fixture is broken, ensuring
# we never compare two identical-but-wrong baselines.
# ===========================================================================
CODEX_BACK_REL=".codex/agents/trackfw-backend.toml"
GEMINI_BACK_REL=".gemini/agents/trackfw-backend.md"

for rt in go node py; do
  base_codex="$WORK/case2-base-codex-$rt"
  cand_codex="$WORK/case2-cand-codex-$rt"
  base_gemini="$WORK/case2-base-gemini-$rt"
  cand_gemini="$WORK/case2-cand-gemini-$rt"

  write_yaml "$base_codex"  0; run_install "$rt" "$base_codex"  "codex"
  write_yaml "$cand_codex"  1; run_install "$rt" "$cand_codex"  "codex"
  write_yaml "$base_gemini" 0; run_install "$rt" "$base_gemini" "gemini"
  write_yaml "$cand_gemini" 1; run_install "$rt" "$cand_gemini" "gemini"

  # Vacuity guard — codex
  if [[ ! -f "$base_codex/$CODEX_BACK_REL" ]]; then
    diag "no-namespace-leak/$rt/vacuity-codex" "baseline codex backend not found — fixture broken"
  elif ! grep -q 'model = "gpt-5.4-mini"' "$base_codex/$CODEX_BACK_REL"; then
    diag "no-namespace-leak/$rt/vacuity-codex" "baseline codex backend lacks expected model line (got: $(grep 'model' "$base_codex/$CODEX_BACK_REL" || echo 'no model line'))"
  else
    ok "no-namespace-leak/$rt/vacuity-codex"
  fi

  # Vacuity guard — gemini
  if [[ ! -f "$base_gemini/$GEMINI_BACK_REL" ]]; then
    diag "no-namespace-leak/$rt/vacuity-gemini" "baseline gemini backend not found — fixture broken"
  elif ! grep -q 'model: sonnet' "$base_gemini/$GEMINI_BACK_REL"; then
    diag "no-namespace-leak/$rt/vacuity-gemini" "baseline gemini backend lacks expected model line (got: $(grep 'model:' "$base_gemini/$GEMINI_BACK_REL" || echo 'no model line'))"
  else
    ok "no-namespace-leak/$rt/vacuity-gemini"
  fi

  # Byte-identical comparison: codex
  LEAK_FAIL=0
  for rel in $(find "$base_codex/.codex/agents" -name 'trackfw-*.toml' -exec basename {} \; | sort); do
    base_f="$base_codex/.codex/agents/$rel"
    cand_f="$cand_codex/.codex/agents/$rel"
    if [[ ! -f "$cand_f" ]]; then
      diag "no-namespace-leak/$rt/codex" "candidate missing: .codex/agents/$rel"; LEAK_FAIL=1; continue
    fi
    if ! cmp -s "$base_f" "$cand_f"; then
      diag "no-namespace-leak/$rt/codex" "$rel changed when agent_models was added (namespace leak!)"
      diff "$base_f" "$cand_f" >&2 || true
      LEAK_FAIL=1
    fi
  done
  [[ $LEAK_FAIL -eq 0 ]] && ok "no-namespace-leak/$rt/codex-all-agents-unchanged"

  # Byte-identical comparison: gemini
  LEAK_FAIL=0
  for rel in $(find "$base_gemini/.gemini/agents" -name 'trackfw-*.md' -exec basename {} \; | sort); do
    base_f="$base_gemini/.gemini/agents/$rel"
    cand_f="$cand_gemini/.gemini/agents/$rel"
    if [[ ! -f "$cand_f" ]]; then
      diag "no-namespace-leak/$rt/gemini" "candidate missing: .gemini/agents/$rel"; LEAK_FAIL=1; continue
    fi
    if ! cmp -s "$base_f" "$cand_f"; then
      diag "no-namespace-leak/$rt/gemini" "$rel changed when agent_models was added (namespace leak!)"
      diff "$base_f" "$cand_f" >&2 || true
      LEAK_FAIL=1
    fi
  done
  [[ $LEAK_FAIL -eq 0 ]] && ok "no-namespace-leak/$rt/gemini-all-agents-unchanged"
done

# ===========================================================================
# Case 3 — Absent config: Claude tier alias preserved, cross-runtime
#
# Without agent_models, the Claude backend must keep "model: sonnet" (the
# canonical tier alias). All three runtimes must produce byte-identical files.
# ===========================================================================
for rt in go node py; do
  proj="$WORK/case3-$rt"
  write_yaml "$proj" 0
  run_install "$rt" "$proj" "claude"
done

# Vacuity guard
go_back3="$WORK/case3-go/.claude/agents/trackfw-backend.md"
if [[ ! -f "$go_back3" ]]; then
  diag "absent-config/vacuity-guard" "Go did not generate trackfw-backend.md — fixture broken"
elif ! grep -q 'model: sonnet' "$go_back3"; then
  diag "absent-config/vacuity-guard" "Go backend missing 'model: sonnet' — regression: $(grep 'model' "$go_back3" || echo 'no model line')"
else
  ok "absent-config/vacuity-guard/tier-alias-preserved"
fi

if [[ $FAIL -eq 0 ]]; then
  C3_FAIL=0
  for rel in $(find "$WORK/case3-go/.claude/agents" -name 'trackfw-*.md' -exec basename {} \; | sort); do
    go_f="$WORK/case3-go/.claude/agents/$rel"
    node_f="$WORK/case3-node/.claude/agents/$rel"
    py_f="$WORK/case3-py/.claude/agents/$rel"
    if ! cmp -s "$go_f" "$node_f" 2>/dev/null; then
      diag "absent-config/cross-runtime/go-vs-node" "$rel differs"
      diff "$go_f" "$node_f" >&2 || true
      C3_FAIL=1
    fi
    if ! cmp -s "$go_f" "$py_f" 2>/dev/null; then
      diag "absent-config/cross-runtime/go-vs-python" "$rel differs"
      diff "$go_f" "$py_f" >&2 || true
      C3_FAIL=1
    fi
  done
  [[ $C3_FAIL -eq 0 ]] && ok "absent-config/cross-runtime/claude-12-agents-byte-identical"
fi

# ===========================================================================
# Case 4 — Escape hatch: dated ID written literally, cross-runtime
#
# With agent_models: {sonnet: "claude-sonnet-4-5-20250929"}, the backend must
# have "model: claude-sonnet-4-5-20250929" — not composed, not mapped.
# ===========================================================================
for rt in go node py; do
  proj="$WORK/case4-$rt"
  write_yaml_escape_hatch "$proj"
  run_install "$rt" "$proj" "claude"
done

# Vacuity guard
go_back4="$WORK/case4-go/.claude/agents/trackfw-backend.md"
if [[ ! -f "$go_back4" ]]; then
  diag "escape-hatch/vacuity-guard" "Go did not generate trackfw-backend.md — fixture broken"
elif ! grep -q 'model: claude-sonnet-4-5-20250929' "$go_back4"; then
  diag "escape-hatch/vacuity-guard" "Go backend does not contain literal escape hatch value (got: $(grep 'model:' "$go_back4" || echo 'no model line'))"
else
  ok "escape-hatch/vacuity-guard/literal-value-written"
fi

if [[ $FAIL -eq 0 ]]; then
  C4_FAIL=0
  for rel in $(find "$WORK/case4-go/.claude/agents" -name 'trackfw-*.md' -exec basename {} \; | sort); do
    go_f="$WORK/case4-go/.claude/agents/$rel"
    node_f="$WORK/case4-node/.claude/agents/$rel"
    py_f="$WORK/case4-py/.claude/agents/$rel"
    if ! cmp -s "$go_f" "$node_f" 2>/dev/null; then
      diag "escape-hatch/cross-runtime/go-vs-node" "$rel differs"
      diff "$go_f" "$node_f" >&2 || true
      C4_FAIL=1
    fi
    if ! cmp -s "$go_f" "$py_f" 2>/dev/null; then
      diag "escape-hatch/cross-runtime/go-vs-python" "$rel differs"
      diff "$go_f" "$py_f" >&2 || true
      C4_FAIL=1
    fi
  done
  [[ $C4_FAIL -eq 0 ]] && ok "escape-hatch/cross-runtime/claude-12-agents-byte-identical"
fi

# ===========================================================================
# Case 5 — Control-character injection: agents install MUST refuse a
# trackfw.yaml whose agent_models value contains a newline.
#
# Two injection variants (ML-5A):
#   5a. "claude-sonnet-4-6\ntools: Bash"    — YAML key injection
#   5b. "claude-sonnet-4-6\n---\nINJECTED" — frontmatter-close + body injection
#
# Expected: install exits non-zero for each variant × each runtime.
# A zero exit (silent acceptance of control chars) is the failure.
#
# IMPORTANT: run_install silences stdout/stderr and, under set -e/-u, a
# failing subshell would kill the whole script. We use "if ! cmd; then …"
# which disarms set -e for exactly that invocation — the same pattern used
# by the P4 sabotage braço in check-update-parity.sh.
# ===========================================================================

write_yaml_control_key_injection() {
  local proj=$1
  mkdir -p "$proj"
  # YAML double-quoted scalars interpret \n as a newline escape sequence
  # (yaml.v3/PyYAML/js-yaml all parse this as a string with a literal \x0A).
  # This is the exact payload that triggered the injection in ML-4A.
  cat >"$proj/trackfw.yaml" <<'YAML'
project_name: agent-models-parity-test
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
agent_models:
  sonnet: "claude-sonnet-4-6\ntools: Bash"
  opus: "5"
YAML
}

write_yaml_control_body_injection() {
  local proj=$1
  mkdir -p "$proj"
  # Variant b: \n---\n closes the frontmatter block and injects body content.
  # This is the most severe variant (body = executable instruction for the agent).
  cat >"$proj/trackfw.yaml" <<'YAML'
project_name: agent-models-parity-test
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
agent_models:
  sonnet: "claude-sonnet-4-6\n---\nINSTRUCAO INJETADA NO CORPO"
  opus: "5"
YAML
}

# run_install_expect_fail RUNTIME PROJECT_DIR TARGET
# Runs the install command and returns 0 if it FAILS (exit != 0), 1 if it
# succeeds. The "if !" form disarms set -e for the subshell.
run_install_expect_fail() {
  local rt=$1 proj=$2 target=$3
  local home_dir="$WORK/home-$rt-${target//\//-}-fail"
  mkdir -p "$home_dir"
  local exit_code=0
  case "$rt" in
    go)
      if (cd "$proj" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --scope project >/dev/null 2>&1); then
        exit_code=1
      fi
      ;;
    node)
      if (cd "$proj" && HOME="$home_dir" node "$NODE_CLI" agents install --targets "$target" --scope project >/dev/null 2>&1); then
        exit_code=1
      fi
      ;;
    py)
      if (cd "$proj" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" python3 -m trackfw agents install --targets "$target" --scope project >/dev/null 2>&1); then
        exit_code=1
      fi
      ;;
    *)
      echo "check-agent-models-parity: unknown runtime '$rt'" >&2; exit 1 ;;
  esac
  return $exit_code
}

# Variant 5a — key injection (\n in value injects a YAML key)
for rt in go node py; do
  proj="$WORK/case5a-$rt"
  write_yaml_control_key_injection "$proj"
  if run_install_expect_fail "$rt" "$proj" "claude"; then
    ok "control-char/key-injection/$rt/install-rejected"
  else
    diag "control-char/key-injection/$rt/install-rejected" "$rt accepted control char value (exit 0) — frontmatter injection not blocked"
  fi
done

# Variant 5b — body injection (\n---\n closes frontmatter, injects body content)
for rt in go node py; do
  proj="$WORK/case5b-$rt"
  write_yaml_control_body_injection "$proj"
  if run_install_expect_fail "$rt" "$proj" "claude"; then
    ok "control-char/body-injection/$rt/install-rejected"
  else
    diag "control-char/body-injection/$rt/install-rejected" "$rt accepted control char value (exit 0) — frontmatter/body injection not blocked"
  fi
done

# Vacuity guard: confirm that the YAML fixture actually contains the \n
# escape sequence (two characters: backslash + n) so the parser produces a
# string with a literal newline character. If the fixture is wrong, the test
# would pass trivially because the value would not contain a control char.
case5a_yaml="$WORK/case5a-go/trackfw.yaml"
if [[ -f "$case5a_yaml" ]]; then
  if grep -q '\\n' "$case5a_yaml"; then
    ok "control-char/vacuity/yaml-fixture-contains-backslash-n-escape"
  else
    diag "control-char/vacuity/yaml-fixture-contains-backslash-n-escape" "fixture missing \\n escape — test passed trivially"
  fi
fi

# ===========================================================================
# Case 5c — Unicode line-separator injection: agents install MUST refuse a
# trackfw.yaml whose agent_models value contains U+2028 (LINE SEPARATOR) or
# U+2029 (PARAGRAPH SEPARATOR). yaml.v3 preserves these codepoints verbatim
# in the parsed Go string (bytes 0xE2 0x80 0xA8 / 0xE2 0x80 0xA9, all ≥ 0x80,
# invisible to the original ASCII < 0x20 check). Line-based frontmatter
# parsers treat U+2028 as a line terminator, enabling structural injection.
# (ML-5C, measured 2026-08-21 with `go run` directly against yaml.v3.)
#
# The fixture embeds literal U+2028 bytes (0xE2 0x80 0xA8) directly in the
# YAML double-quoted string. All three parsers (yaml.v3, js-yaml, PyYAML)
# preserve this codepoint verbatim in the parsed value — confirmed by
# direct `go run` measurement 2026-08-21. The vacuity guard below confirms
# the fixture file contains the literal U+2028 bytes.
#
# NEL (U+0085) intentionally excluded: yaml.v3 normalizes it to a space
# before reaching containsControlChar; no injection path exists (measured).
# ===========================================================================

write_yaml_unicode_linesep() {
  local proj=$1
  mkdir -p "$proj"
  # Literal U+2028 bytes in the YAML value. All three parsers preserve
  # U+2028 verbatim in the parsed string (yaml.v3 measured 2026-08-21).
  cat >"$proj/trackfw.yaml" <<'YAML'
project_name: agent-models-parity-test
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
agent_models:
  sonnet: "claude-sonnet-4-6 tools: Bash"
  opus: "5"
YAML
}

# Variant 5c — U+2028 LINE SEPARATOR injection
for rt in go node py; do
  proj="$WORK/case5c-$rt"
  write_yaml_unicode_linesep "$proj"
  if run_install_expect_fail "$rt" "$proj" "claude"; then
    ok "unicode-linesep/U+2028/$rt/install-rejected"
  else
    diag "unicode-linesep/U+2028/$rt/install-rejected" "$rt accepted U+2028 value (exit 0) — unicode line-separator injection not blocked"
  fi
done

# Vacuity guard for case 5c: confirm the YAML fixture contains the literal
# U+2028 bytes (0xE2 0x80 0xA8). If the fixture lost the character (e.g.
# was written as plain ASCII), the test would pass trivially because the
# value would not contain a unicode line separator.
case5c_yaml="$WORK/case5c-go/trackfw.yaml"
if [[ -f "$case5c_yaml" ]]; then
  if grep -qF ' ' "$case5c_yaml"; then
    ok "unicode-linesep/vacuity/yaml-fixture-contains-u2028-escape"
  else
    diag "unicode-linesep/vacuity/yaml-fixture-contains-u2028-escape" "fixture missing \\u2028 escape — test passed trivially"
  fi
fi

# ---------------------------------------------------------------------------
if [[ "$FAIL" -ne 0 ]]; then
  echo "check-agent-models-parity: drift detected — see FAIL lines above." >&2
  exit 1
fi

echo "All check-agent-models-parity.sh scenarios passed (4 cases × 3 runtimes + Case 5 control-char/unicode-separator rejection, namespace isolation confirmed for codex+gemini)."
