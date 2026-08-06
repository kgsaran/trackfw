#!/usr/bin/env bash
# check-agent-hooks-parity.sh — proves the per-CLI agent hook files written by
# `trackfw discover --init` (.claude/settings.json, .codex/hooks.json,
# .gemini/settings.json, .github/hooks/trackfw-attention.json,
# .cursor/hooks.json, .kiro/hooks/trackfw-attention.json) are STRUCTURALLY
# identical across Go, Node.js and Python for each of the 6 native-wave CLIs
# (Claude Code, Codex, Gemini CLI, GitHub Copilot, Cursor, Kiro).
#
# Extends the family started by check-attention-scripts-parity.sh (which only
# covers the two shell scripts, byte-for-byte). Each CLI has its own JSON
# schema by design (docs/cli-parity.md documents each one), so this gate is
# NOT byte-identical like the shell-script gate — it parses each generated
# file as JSON and deep-compares the parsed structure (keys, nesting, values,
# including array order, since e.g. GitHub Copilot's own docs pin execution
# order to array order — see docs/cli-parity.md "GitHub Copilot wiring
# (ML-2D)"). JSON indentation/key-insertion-order differences between the Go,
# Node.js and Python serializers are irrelevant and never reported as drift.
#
# Follows the conventions of check-attention-scripts-parity.sh: set -euo
# pipefail, mktemp -d fixture with a cleanup trap, BASH_SOURCE-relative
# ROOT_DIR, GO_BIN resolution (build a throwaway binary if unset), explicit
# diagnostics naming the CLI/stack pair and the divergent JSON path (never a
# hash-only comparison).
#
# Real invocation, not internal generator calls: each stack runs its own real
# `discover --init` entry point (Go binary / `node npm/bin/trackfw` /
# `python3 -m trackfw`) exactly once, against a single fixture directory per
# stack that carries ALL 6 CLIs' detection markers at once (see
# internal/generators/hooks.go:InjectHooksDetected and its Node/Python
# equivalents) — CLAUDE.md, AGENTS.md, GEMINI.md, .kiro/,
# .github/copilot-instructions.md, .cursor/. This exercises the detection
# dispatcher as a whole (all 6 branches in one run), not just each per-CLI
# injector function in isolation, and keeps this gate to 3 `discover --init`
# invocations (one full scaffold + gate install each) instead of 18 — a
# per-CLI-isolated fixture set was measured to add ~15s to `make quality` for
# no detection benefit the per-file vacuity guards below don't already cover
# (a detector regression that silently skips one CLI still fails that CLI's
# "missing or empty" guard; nothing about co-locating the 6 markers in one
# fixture can mask that).
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-agent-hooks-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-attention-scripts-parity.sh:
#   GO_BIN unset → build a throwaway binary so the script also works standalone.
#   GO_BIN relative → prefix with ROOT_DIR.
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
  echo "check-agent-hooks-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-agent-hooks-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Per-CLI table: marker file/dir that InjectHooksDetected (Go/Node/Python)
# requires to detect the CLI, and the relative path of the hook file each
# InjectXHooks writes. All 6 markers are placed together in one fixture dir
# per stack (see file header for why single-fixture is safe here).
# ---------------------------------------------------------------------------
CLIS="claude codex gemini copilot cursor kiro"

marker_for() {
  case "$1" in
    claude)  echo "file:CLAUDE.md" ;;
    codex)   echo "file:AGENTS.md" ;;
    gemini)  echo "file:GEMINI.md" ;;
    copilot) echo "file:.github/copilot-instructions.md" ;;
    cursor)  echo "dir:.cursor" ;;
    kiro)    echo "dir:.kiro" ;;
    *) echo "marker_for: unknown cli '$1'" >&2; exit 1 ;;
  esac
}

hookfile_for() {
  case "$1" in
    claude)  echo ".claude/settings.json" ;;
    codex)   echo ".codex/hooks.json" ;;
    gemini)  echo ".gemini/settings.json" ;;
    copilot) echo ".github/hooks/trackfw-attention.json" ;;
    cursor)  echo ".cursor/hooks.json" ;;
    kiro)    echo ".kiro/hooks/trackfw-attention.json" ;;
    *) echo "hookfile_for: unknown cli '$1'" >&2; exit 1 ;;
  esac
}

place_marker() {
  local dir=$1 marker=$2
  local kind=${marker%%:*} rel=${marker#*:}
  case "$kind" in
    file) mkdir -p "$(dirname "$dir/$rel")" && : >"$dir/$rel" ;;
    dir)  mkdir -p "$dir/$rel" ;;
    *) echo "place_marker: unknown marker kind '$kind'" >&2; exit 1 ;;
  esac
}

# ---------------------------------------------------------------------------
# Generate all 6 hook files with each runtime via a single `discover --init`
# on a fixture directory that carries every CLI's detection marker — the same
# production entry point used by check-attention-scripts-parity.sh, so this
# gate also catches a runtime that stops wiring a CLI's injector into
# InjectHooksDetected/injectHooksDetected/inject_hooks_detected.
# ---------------------------------------------------------------------------
run_discover_init() {
  local runtime=$1 dir=$2
  mkdir -p "$dir"
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" discover --init)                              >/dev/null 2>"$WORK/$runtime.err" ;;
    node) (cd "$dir" && node "$NODE_CLI" discover --init)                       >/dev/null 2>"$WORK/$runtime.err" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw discover --init) >/dev/null 2>"$WORK/$runtime.err" ;;
    *)    echo "run_discover_init: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
}

for runtime in go node py; do
  dir="$WORK/$runtime"
  for cli in $CLIS; do
    place_marker "$dir" "$(marker_for "$cli")"
  done
done

set +e
for runtime in go node py; do
  run_discover_init "$runtime" "$WORK/$runtime"
  status=$?
  if [[ "$status" -ne 0 ]]; then
    fail "agent-hooks-parity/$runtime/discover-init" \
      "discover --init exited $status; stderr: $(cat "$WORK/$runtime.err" 2>/dev/null)"
  fi
done
set -e

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-agent-hooks-parity.sh: one or more scenarios FAILED (setup)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# P2 vacuity guards, two of them:
#   1. the hook file exists and is non-empty for all three runtimes, per CLI
#      (a missing/empty file on one side would make a structural diff either
#      error out uninformatively or, worse, be silently skipped);
#   2. the hook file actually references scripts/trackfw-credential-guard.sh
#      at least once, per runtime — a regression that dropped the
#      credential-guard entry from all three stacks identically would
#      otherwise still "pass" a pure cross-stack equality check, defeating
#      the whole point of this ML.
# ---------------------------------------------------------------------------
for cli in $CLIS; do
  hookfile=$(hookfile_for "$cli")
  for runtime in go node py; do
    path="$WORK/$runtime/$hookfile"
    if [[ ! -s "$path" ]]; then
      fail "agent-hooks-parity/$cli/$runtime/$hookfile" "missing or empty: $path"
      continue
    fi
    if ! grep -q "trackfw-credential-guard.sh" "$path"; then
      fail "agent-hooks-parity/$cli/$runtime/credential-guard-present" \
        "scripts/trackfw-credential-guard.sh not referenced anywhere in $path"
    fi
  done
done

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-agent-hooks-parity.sh: one or more scenarios FAILED (vacuity guard)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Structural (parsed-JSON) diff, go-vs-node and go-vs-py, per CLI. Array order
# is significant (semantic for at least one CLI — see file header); object
# key order and whitespace/indentation are not, since json.load in the
# comparator below normalizes both away.
# ---------------------------------------------------------------------------
compare_json() {
  local label=$1 go_file=$2 other_file=$3
  local out
  if out=$(python3 -c "
import json
import sys

a_path, b_path = sys.argv[1], sys.argv[2]
with open(a_path) as f:
    a = json.load(f)
with open(b_path) as f:
    b = json.load(f)


def diff(path, x, y, out):
    if type(x) is not type(y) and not (isinstance(x, (int, float)) and isinstance(y, (int, float))):
        out.append(f'{path}: type {type(x).__name__} (go) vs {type(y).__name__} (other) -- go={x!r} other={y!r}')
        return
    if isinstance(x, dict):
        xk, yk = set(x.keys()), set(y.keys())
        for k in sorted(xk - yk):
            out.append(f'{path}.{k}: present in go, missing in other')
        for k in sorted(yk - xk):
            out.append(f'{path}.{k}: missing in go, present in other')
        for k in sorted(xk & yk):
            diff(f'{path}.{k}', x[k], y[k], out)
    elif isinstance(x, list):
        if len(x) != len(y):
            out.append(f'{path}: array length {len(x)} (go) vs {len(y)} (other)')
        for i, (xi, yi) in enumerate(zip(x, y)):
            diff(f'{path}[{i}]', xi, yi, out)
    else:
        if x != y:
            out.append(f'{path}: value {x!r} (go) vs {y!r} (other)')


diffs = []
diff('\$', a, b, diffs)
if diffs:
    print('\n'.join(diffs))
    sys.exit(1)
" "$go_file" "$other_file" 2>&1); then
    ok "$label"
  else
    fail "$label" "structural drift:
$out"
  fi
}

for cli in $CLIS; do
  hookfile=$(hookfile_for "$cli")
  go_file="$WORK/go/$hookfile"
  node_file="$WORK/node/$hookfile"
  py_file="$WORK/py/$hookfile"

  compare_json "agent-hooks-parity/$cli/go-vs-node" "$go_file" "$node_file"
  compare_json "agent-hooks-parity/$cli/go-vs-py" "$go_file" "$py_file"
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-agent-hooks-parity.sh scenarios passed."
else
  echo "check-agent-hooks-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
