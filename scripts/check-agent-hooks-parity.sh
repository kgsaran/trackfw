#!/usr/bin/env bash
# check-agent-hooks-parity.sh — proves the per-CLI agent hook files written by
# `trackfw discover --init` (.claude/settings.json, .codex/hooks.json,
# .gemini/settings.json, .github/hooks/trackfw-attention.json,
# .cursor/hooks.json, .kiro/hooks/trackfw-attention.json, .windsurf/hooks.json,
# .amazonq/cli-agents/q_cli_default.json) are STRUCTURALLY identical across
# Go, Node.js and Python for each of the 8 native-wave CLIs (Claude Code,
# Codex, Gemini CLI, GitHub Copilot, Cursor, Kiro, Windsurf, Amazon Q
# Developer — the last two added ROADMAP-2026-08-20/ML-1B; see that ML's
# report for why the existing generic structural diff needed zero changes to
# extend to their different-but-still-single-fixed-path-JSON-file shape).
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
# stack that carries ALL 8 CLIs' detection markers at once (see
# internal/generators/hooks.go:InjectHooksDetected and its Node/Python
# equivalents) — CLAUDE.md, AGENTS.md, GEMINI.md, .kiro/,
# .github/copilot-instructions.md, .cursor/, .windsurfrules, .amazonq/. This
# exercises the detection dispatcher as a whole (all 8 branches in one run),
# not just each per-CLI injector function in isolation, and keeps this gate
# to 3 `discover --init` invocations (one full scaffold + gate install each)
# instead of 24 — a per-CLI-isolated fixture set was measured to add ~15s to
# `make quality` for no detection benefit the per-file vacuity guards below
# don't already cover (a detector regression that silently skips one CLI
# still fails that CLI's "missing or empty" guard; nothing about co-locating
# the 8 markers in one fixture can mask that).
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
# InjectXHooks writes. All 8 markers are placed together in one fixture dir
# per stack (see file header for why single-fixture is safe here). windsurf/
# amazonq added ROADMAP-2026-08-20/ML-1B — markers/paths confirmed against
# internal/generators/hooks.go's InjectHooksDetected table (file:.windsurfrules
# and dir:.amazonq, same convention already used by file:CLAUDE.md and
# dir:.cursor above), not invented for this gate.
# ---------------------------------------------------------------------------
CLIS="claude codex gemini copilot cursor kiro windsurf amazonq"

marker_for() {
  case "$1" in
    claude)  echo "file:CLAUDE.md" ;;
    codex)   echo "file:AGENTS.md" ;;
    gemini)  echo "file:GEMINI.md" ;;
    copilot) echo "file:.github/copilot-instructions.md" ;;
    cursor)  echo "dir:.cursor" ;;
    kiro)    echo "dir:.kiro" ;;
    windsurf) echo "file:.windsurfrules" ;;
    amazonq)  echo "dir:.amazonq" ;;
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
    windsurf) echo ".windsurf/hooks.json" ;;
    amazonq)  echo ".amazonq/cli-agents/q_cli_default.json" ;;
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
#
# HOME must be isolated per runtime (empty dir under $WORK), matching the
# convention every other real-CLI-invocation gate uses (see
# check-update-parity.sh run_update/run_init/install_agent_*). Since
# ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 1), each InjectXHooks
# dedups the project-scope credential-guard entry against the global one
# already installed at $HOME/.trackfw/scripts/trackfw-credential-guard.sh
# (globalCredentialGuardInstalledClaude/Codex/Gemini/Cursor/Copilot/Kiro —
# read-only, fail-open). Without an isolated HOME this gate reads the REAL
# machine's $HOME: on any dev machine that already ran `trackfw update
# harness` (the product's own onboarding step), the global entry is present,
# so discover --init silently skips the project-scope entry it's supposed to
# add — an environmental false failure identical in nature (and same root
# cause) to the one documented in
# vault/notes/node-global-credential-guard-dedup-breaks-inject-tests-on-real-home-2026-08-08.md
# for npm/tests/credential_guard.test.js, just hitting a shell gate instead
# of a JS test file.
# ---------------------------------------------------------------------------
run_discover_init() {
  local runtime=$1 dir=$2
  local home_dir="$dir.home"
  mkdir -p "$dir" "$home_dir"
  case "$runtime" in
    go)   (cd "$dir" && HOME="$home_dir" "$GO_BIN" discover --init)                              >/dev/null 2>"$WORK/$runtime.err" ;;
    node) (cd "$dir" && HOME="$home_dir" node "$NODE_CLI" discover --init)                       >/dev/null 2>"$WORK/$runtime.err" ;;
    py)   (cd "$dir" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" python3 -m trackfw discover --init) >/dev/null 2>"$WORK/$runtime.err" ;;
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
#   2. the hook file actually references the guard script it's supposed to
#      wire at least once, per runtime — a regression that dropped the
#      guard entry from all three stacks identically would otherwise still
#      "pass" a pure cross-stack equality check, defeating the whole point
#      of this ML. For 6 of the 8 CLIs that's scripts/trackfw-credential-
#      guard.sh; windsurf and amazonq don't wire credential-guard at all —
#      confirmed by grep across agentfiles.go/hooks.js/hooks.py (ML-1A
#      parecer, ROADMAP-2026-08-20) — only git-branch-guard, so their guard
#      marker is trackfw-git-branch-guard.sh instead. Using the
#      credential-guard string for these two would make the guard reprove
#      for the wrong reason (string never present, even on a healthy run).
# ---------------------------------------------------------------------------
guard_marker_for() {
  case "$1" in
    windsurf|amazonq) echo "trackfw-git-branch-guard.sh" ;;
    *) echo "trackfw-credential-guard.sh" ;;
  esac
}

for cli in $CLIS; do
  hookfile=$(hookfile_for "$cli")
  marker=$(guard_marker_for "$cli")
  for runtime in go node py; do
    path="$WORK/$runtime/$hookfile"
    if [[ ! -s "$path" ]]; then
      fail "agent-hooks-parity/$cli/$runtime/$hookfile" "missing or empty: $path"
      continue
    fi
    if ! grep -q "$marker" "$path"; then
      fail "agent-hooks-parity/$cli/$runtime/credential-guard-present" \
        "$marker not referenced anywhere in $path"
    fi
  done
done

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-agent-hooks-parity.sh: one or more scenarios FAILED (vacuity guard)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# P3 vacuity guard — deniedCommands (Amazon Q only, ML-4B):
# A correlated drop of deniedCommands from all 3 stacks passes compare_json
# (both sides missing the key) and the P2 guard above (git-branch-guard
# script string still present in the agent JSON). This guard catches that
# case by asserting the deny pattern is present per runtime. Cenário 83
# proves non-vacuity.
# ---------------------------------------------------------------------------
AQ_HOOKFILE=$(hookfile_for amazonq)
DENIED_PATTERN='^git (commit|push|checkout -b)'
for runtime in go node py; do
  aq_path="$WORK/$runtime/$AQ_HOOKFILE"
  if ! grep -qF "$DENIED_PATTERN" "$aq_path"; then
    fail "agent-hooks-parity/amazonq/$runtime/denied-commands-present" \
      "deniedCommands pattern '$DENIED_PATTERN' not found in $aq_path — correlated drop across all stacks would be silent without this guard (Cenário 83)"
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-agent-hooks-parity.sh: one or more scenarios FAILED (deniedCommands vacuity guard)." >&2
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
