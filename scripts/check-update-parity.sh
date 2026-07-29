#!/usr/bin/env bash
# check-update-parity.sh — proves the three CLI runtimes (Go, Node.js, Python)
# implement `trackfw update` and `trackfw update harness` identically, per
# the contract frozen in docs/cli-parity.md ("## `trackfw update` vs
# `trackfw update harness`", ML-6A).
#
# ML-6G (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador).
#
# Diagnosis this gate closes: the first Wave 6 implementation round (ML-6B/
# 6C/6D) each passed their own suite, yet cross-runtime audit found four
# divergences the per-runtime tests never compared against each other:
#   1. Harness target count: Go=3, Node=19, Python=19.
#   2. `path` rendering: Node tilde-abbreviated, Python absolute.
#   3. The `claude-skills` artifact id: Node -> trackfw-architecture-skill,
#      Python -> trackfw-governance.
#   4. `update` (project scope) flag/JSON surface: Node exposes the four
#      flags and --json; Go and Python do not.
# ML-6F is the corrective ML for these four; this gate is the automated
# proof that closes the loop so the divergence cannot silently return.
#
# Method: this compares raw JSON bytes after reparsing+redumping to strip
# only *whitespace* differences (Node pretty-prints with indent 2, Go/Python
# emit compact JSON) — key order and target order are preserved (NOT
# sorted), because sorting is exactly what let the ML-2E key-order
# divergence survive an earlier audit in this same session (see
# check-barrier.sh's normalize_barrier_json for the same rationale). Where a
# file-level diff suffices (none needed here since there is no filesystem
# artifact to compare directly, only stdout), diff -u is used directly.
#
# Follows the conventions of scripts/check-rules-parity.sh and
# scripts/check-slash-parity.sh: set -euo pipefail, mktemp -d fixtures with
# a cleanup trap, HOME redirected for every invocation, a vacuity guard
# before any comparison, "OK [scenario/name]" on success, and accumulating
# all drift before exiting instead of failing on the first mismatch (useful
# here since multiple divergences can be fixed in the same pass).
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-update-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="$ROOT_DIR/pypi"

if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-update-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-update-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

FAIL=0

ok() { echo "OK   [$1]"; }
diag() {
  echo "FAIL [$1]: $2" >&2
  FAIL=1
}

# ---------------------------------------------------------------------------
# run_update RUNTIME HOME_DIR PROJECT_DIR ARGS...
# Sets UPDATE_EXIT, UPDATE_STDOUT, UPDATE_STDERR as globals. HOME is
# redirected to an isolated per-scenario/per-runtime directory on EVERY
# invocation — this gate exercises `update harness`, which by contract
# writes into the user's home directory, so the real HOME must never be
# reachable from here (mirrors scripts/check-rules-parity.sh and
# scripts/check-identity-parity.sh).
# ---------------------------------------------------------------------------
run_update() {
  local runtime=$1 home_dir=$2 project_dir=$3
  shift 3
  local out_file="$WORK/out.$$.$RANDOM" err_file="$WORK/err.$$.$RANDOM"
  set +e
  case "$runtime" in
  go) (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" update "$@") >"$out_file" 2>"$err_file" ;;
  node) (cd "$project_dir" && HOME="$home_dir" node "$NODE_CLI" update "$@") >"$out_file" 2>"$err_file" ;;
  py) (cd "$project_dir" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" python3 -m trackfw update "$@") >"$out_file" 2>"$err_file" ;;
  *) echo "run_update: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  UPDATE_EXIT=$?
  set -e
  UPDATE_STDOUT=$(cat "$out_file")
  UPDATE_STDERR=$(cat "$err_file")
  rm -f "$out_file" "$err_file"
}

# normalize_update_json — reparses stdin preserving key order and target
# order (object_pairs_hook=OrderedDict, no sort_keys) and redumps with a
# fixed indent, so only real shape/order/content differences survive —
# whitespace-only differences between Node's pretty-printer and Go/Python's
# compact emitter must never be reported as drift.
normalize_update_json() {
  python3 -c "
import json, sys
from collections import OrderedDict
d = json.loads(sys.stdin.read(), object_pairs_hook=OrderedDict)
json.dump(d, sys.stdout, indent=2, ensure_ascii=False)
"
}

# target_ids_json DOC — prints the JSON array of target ids, in declared
# order, from an update result document.
target_ids_json() {
  python3 -c "
import json, sys
from collections import OrderedDict
d = json.loads(sys.argv[1], object_pairs_hook=OrderedDict)
print(json.dumps([t['id'] for t in d['targets']]))
" "$1"
}

# snapshot_tree DIR — sha256 of every regular file under DIR, path-relative,
# sorted — used to prove --dry-run performs zero writes.
snapshot_tree() {
  local dir=$1
  if [[ -d "$dir" ]]; then
    (cd "$dir" && find . -type f -print0 | sort -z | xargs -0 shasum -a 256 2>/dev/null) || true
  fi
}

install_claude_agents() {
  local home_dir=$1
  mkdir -p "$home_dir"
  HOME="$home_dir" "$GO_BIN" agents install --targets claude --scope global \
    --identity-preset neutral --json >/dev/null
}

# ===========================================================================
# Scenario 1 — `update harness --json` on an empty harness. All three
# runtimes must report every target `missing`, exit 0 (missing is not an
# error per the ML-6A contract), and emit byte-identical JSON.
# ===========================================================================
S1_PROJECT="$WORK/s1-project"
mkdir -p "$S1_PROJECT"
mkdir -p "$WORK/s1-home-go" "$WORK/s1-home-node" "$WORK/s1-home-py"

run_update go "$WORK/s1-home-go" "$S1_PROJECT" harness --json
S1_GO_EXIT=$UPDATE_EXIT; S1_GO_OUT=$UPDATE_STDOUT
run_update node "$WORK/s1-home-node" "$S1_PROJECT" harness --json
S1_NODE_EXIT=$UPDATE_EXIT; S1_NODE_OUT=$UPDATE_STDOUT
run_update py "$WORK/s1-home-py" "$S1_PROJECT" harness --json
S1_PY_EXIT=$UPDATE_EXIT; S1_PY_OUT=$UPDATE_STDOUT

for pair in "go:$S1_GO_EXIT" "node:$S1_NODE_EXIT" "python:$S1_PY_EXIT"; do
  rt=${pair%%:*}; ec=${pair##*:}
  if [[ "$ec" != "0" ]]; then
    diag "update-harness/empty-harness/exit-zero" "$rt exited $ec on an empty harness (missing must never be an error)"
  fi
done

# Vacuity guard: each output must actually parse as JSON and carry a
# non-empty `targets` array before any comparison is meaningful.
for pair in "go:$S1_GO_OUT" "node:$S1_NODE_OUT" "python:$S1_PY_OUT"; do
  rt=${pair%%:*}; out=${pair#*:}
  count=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); print(len(d.get('targets',[])))" "$out" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$count" == "PARSE_ERROR" || "$count" == "0" ]]; then
    diag "update-harness/empty-harness/vacuity-guard" "$rt produced no parseable/non-empty targets array — cannot compare"
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  echo "check-update-parity: vacuity guard failed for scenario 1 — aborting further scenario-1 comparison" >&2
else
  ok "update-harness/empty-harness/vacuity-guard"

  echo "$S1_GO_OUT" | normalize_update_json >"$WORK/s1.go.json"
  echo "$S1_NODE_OUT" | normalize_update_json >"$WORK/s1.node.json"
  echo "$S1_PY_OUT" | normalize_update_json >"$WORK/s1.py.json"

  if ! diff -u "$WORK/s1.go.json" "$WORK/s1.node.json" >"$WORK/s1.diff.go-node.txt"; then
    diag "update-harness/empty-harness/go-vs-node" "JSON diverges (see diff below)
$(cat "$WORK/s1.diff.go-node.txt")"
  fi
  if ! diff -u "$WORK/s1.go.json" "$WORK/s1.py.json" >"$WORK/s1.diff.go-py.txt"; then
    diag "update-harness/empty-harness/go-vs-python" "JSON diverges (see diff below)
$(cat "$WORK/s1.diff.go-py.txt")"
  fi
  if [[ "$FAIL" -eq 0 ]]; then
    ok "update-harness/empty-harness/three-runtimes-identical"
  fi

  # Every target must be `missing` and summary must equal
  # {updated:0, skipped:0, missing:<n>, failed:0}.
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
n = len(d['targets'])
bad = [t['id'] for t in d['targets'] if t['state'] != 'missing']
s = d['summary']
if bad or s != {'updated': 0, 'skipped': 0, 'missing': n, 'failed': 0}:
    print('go: bad states=%r summary=%r' % (bad, s))
    sys.exit(1)
" "$S1_GO_OUT" || diag "update-harness/empty-harness/all-missing" "Go: not every target is 'missing' or summary miscounts"
fi

# ===========================================================================
# Scenario 2 — `update harness --json` on a populated harness. Installs the
# `claude` agents target (via the Go CLI, so the fixture itself is identical
# across the three homes) into three isolated homes, then proves the three
# runtimes' own `update harness --json` agree.
# ===========================================================================
S2_PROJECT="$WORK/s2-project"
mkdir -p "$S2_PROJECT"
for h in go node py; do
  install_claude_agents "$WORK/s2-home-$h"
done

run_update go "$WORK/s2-home-go" "$S2_PROJECT" harness --json
S2_GO_EXIT=$UPDATE_EXIT; S2_GO_OUT=$UPDATE_STDOUT
run_update node "$WORK/s2-home-node" "$S2_PROJECT" harness --json
S2_NODE_EXIT=$UPDATE_EXIT; S2_NODE_OUT=$UPDATE_STDOUT
run_update py "$WORK/s2-home-py" "$S2_PROJECT" harness --json
S2_PY_EXIT=$UPDATE_EXIT; S2_PY_OUT=$UPDATE_STDOUT

for pair in "go:$S2_GO_EXIT" "node:$S2_NODE_EXIT" "python:$S2_PY_EXIT"; do
  rt=${pair%%:*}; ec=${pair##*:}
  if [[ "$ec" != "0" ]]; then
    diag "update-harness/populated-harness/exit-zero" "$rt exited $ec on a harness with claude agents installed"
  fi
done

# Vacuity guard: the whole point of "populated" is that at least one target
# is NOT `missing` in each runtime — otherwise this scenario degenerates
# into scenario 1 and would pass vacuously even if the install fixture
# silently failed.
for pair in "go:$S2_GO_OUT" "node:$S2_NODE_OUT" "python:$S2_PY_OUT"; do
  rt=${pair%%:*}; out=${pair#*:}
  non_missing=$(python3 -c "
import json, sys
d = json.loads(sys.argv[1])
print(sum(1 for t in d['targets'] if t['state'] != 'missing'))
" "$out" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$non_missing" == "PARSE_ERROR" || "$non_missing" == "0" ]]; then
    diag "update-harness/populated-harness/vacuity-guard" "$rt: no non-missing target found after installing claude agents — fixture had no effect, or output unparseable"
  fi
done

if [[ "$FAIL" -eq 0 ]]; then
  ok "update-harness/populated-harness/vacuity-guard"

  echo "$S2_GO_OUT" | normalize_update_json >"$WORK/s2.go.json"
  echo "$S2_NODE_OUT" | normalize_update_json >"$WORK/s2.node.json"
  echo "$S2_PY_OUT" | normalize_update_json >"$WORK/s2.py.json"

  if ! diff -u "$WORK/s2.go.json" "$WORK/s2.node.json" >"$WORK/s2.diff.go-node.txt"; then
    diag "update-harness/populated-harness/go-vs-node" "JSON diverges (see diff below)
$(cat "$WORK/s2.diff.go-node.txt")"
  fi
  if ! diff -u "$WORK/s2.go.json" "$WORK/s2.py.json" >"$WORK/s2.diff.go-py.txt"; then
    diag "update-harness/populated-harness/go-vs-python" "JSON diverges (see diff below)
$(cat "$WORK/s2.diff.go-py.txt")"
  fi
  if [[ "$FAIL" -eq 0 ]]; then
    ok "update-harness/populated-harness/three-runtimes-identical"
  fi
fi

# ===========================================================================
# Scenario 3 — `update --json` (project scope) in an initialized project.
# All three runtimes must accept --json, report scope=="project", exit 0,
# and agree byte-for-byte.
# ===========================================================================
for h in go node py; do
  mkdir -p "$WORK/s3-home-$h" "$WORK/s3-project-$h"
done
(cd "$WORK/s3-project-go" && HOME="$WORK/s3-home-go" "$GO_BIN" init >/dev/null 2>&1)
(cd "$WORK/s3-project-node" && HOME="$WORK/s3-home-node" node "$NODE_CLI" init >/dev/null 2>&1)
(cd "$WORK/s3-project-py" && HOME="$WORK/s3-home-py" PYTHONPATH="$PY_ROOT" python3 -m trackfw init >/dev/null 2>&1)

run_update go "$WORK/s3-home-go" "$WORK/s3-project-go" --json
S3_GO_EXIT=$UPDATE_EXIT; S3_GO_OUT=$UPDATE_STDOUT; S3_GO_ERR=$UPDATE_STDERR
run_update node "$WORK/s3-home-node" "$WORK/s3-project-node" --json
S3_NODE_EXIT=$UPDATE_EXIT; S3_NODE_OUT=$UPDATE_STDOUT; S3_NODE_ERR=$UPDATE_STDERR
run_update py "$WORK/s3-home-py" "$WORK/s3-project-py" --json
S3_PY_EXIT=$UPDATE_EXIT; S3_PY_OUT=$UPDATE_STDOUT; S3_PY_ERR=$UPDATE_STDERR

# Not colon-joined: stderr text itself may contain ':' and newlines, which
# broke a colon-splitting reader in an earlier draft (see vault note). Each
# runtime is handled as its own explicit statement instead.
if [[ "$S3_GO_EXIT" != "0" ]]; then
  diag "update-project/json/exit-zero" "go: 'trackfw update --json' exited $S3_GO_EXIT (contract requires --json on project update too, per ML-6A 'Applies to: both'); stderr: $S3_GO_ERR"
fi
if [[ "$S3_NODE_EXIT" != "0" ]]; then
  diag "update-project/json/exit-zero" "node: 'trackfw update --json' exited $S3_NODE_EXIT (contract requires --json on project update too, per ML-6A 'Applies to: both'); stderr: $S3_NODE_ERR"
fi
if [[ "$S3_PY_EXIT" != "0" ]]; then
  diag "update-project/json/exit-zero" "python: 'trackfw update --json' exited $S3_PY_EXIT (contract requires --json on project update too, per ML-6A 'Applies to: both'); stderr: $S3_PY_ERR"
fi

if [[ "$FAIL" -eq 0 ]]; then
  scope_go=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['scope'])" "$S3_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
  scope_node=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['scope'])" "$S3_NODE_OUT" 2>/dev/null || echo "PARSE_ERROR")
  scope_py=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['scope'])" "$S3_PY_OUT" 2>/dev/null || echo "PARSE_ERROR")
  for pair in "go:$scope_go" "node:$scope_node" "python:$scope_py"; do
    rt=${pair%%:*}; sc=${pair##*:}
    if [[ "$sc" != "project" ]]; then
      diag "update-project/json/scope-field" "$rt: expected scope=\"project\", got \"$sc\""
    fi
  done
fi

if [[ "$FAIL" -eq 0 ]]; then
  echo "$S3_GO_OUT" | normalize_update_json >"$WORK/s3.go.json"
  echo "$S3_NODE_OUT" | normalize_update_json >"$WORK/s3.node.json"
  echo "$S3_PY_OUT" | normalize_update_json >"$WORK/s3.py.json"
  if ! diff -u "$WORK/s3.go.json" "$WORK/s3.node.json" >"$WORK/s3.diff.go-node.txt"; then
    diag "update-project/json/go-vs-node" "JSON diverges (see diff below)
$(cat "$WORK/s3.diff.go-node.txt")"
  fi
  if ! diff -u "$WORK/s3.go.json" "$WORK/s3.py.json" >"$WORK/s3.diff.go-py.txt"; then
    diag "update-project/json/go-vs-python" "JSON diverges (see diff below)
$(cat "$WORK/s3.diff.go-py.txt")"
  fi
  if [[ "$FAIL" -eq 0 ]]; then
    ok "update-project/json/three-runtimes-identical"
  fi
fi

# ===========================================================================
# Scenario 4 — `--dry-run` on `update harness`: proves (a) zero filesystem
# writes happen and (b) the reported states are identical to a real run
# (only `dry_run` differs), across all three runtimes.
# ===========================================================================
for h in go node py; do
  install_claude_agents "$WORK/s4-home-$h"
  # Seed a deliberately stale legacy skill file (claude-skill target) so the
  # dry-run actually has a pending write to suppress. Without this, every
  # target in this fixture is already current/missing and --dry-run would
  # trivially perform zero writes regardless of whether the guard exists —
  # a vacuous proof.
  mkdir -p "$WORK/s4-home-$h/.claude/skills/trackfw"
  echo "stale placeholder content — must never survive --dry-run" \
    >"$WORK/s4-home-$h/.claude/skills/trackfw/SKILL.md"
done
S4_PROJECT="$WORK/s4-project"
mkdir -p "$S4_PROJECT"

for h in go node py; do
  before=$(snapshot_tree "$WORK/s4-home-$h")
  run_update "$h" "$WORK/s4-home-$h" "$S4_PROJECT" harness --dry-run --json
  eval "S4_${h^^}_EXIT=\$UPDATE_EXIT"
  eval "S4_${h^^}_OUT=\$UPDATE_STDOUT"
  after=$(snapshot_tree "$WORK/s4-home-$h")
  if [[ "$before" != "$after" ]]; then
    diag "update-harness/dry-run/no-writes/$h" "filesystem tree under HOME changed during --dry-run (diff of sha256 snapshots is non-empty)"
  fi
done
if [[ "$FAIL" -eq 0 ]]; then
  ok "update-harness/dry-run/no-writes"
fi

for h in go node py; do
  eval "ec=\$S4_${h^^}_EXIT"
  if [[ "$ec" != "0" ]]; then
    diag "update-harness/dry-run/exit-zero/$h" "expected exit 0 for --dry-run, got $ec"
  fi
done

if [[ "$FAIL" -eq 0 ]]; then
  # dry_run must be true; every other field (states, ids, paths) must match
  # exactly what scenario 2 (the real, non-dry-run run against an
  # equivalently-populated home) produced, modulo the dry_run flag itself.
  for h in go node py; do
    eval "out=\$S4_${h^^}_OUT"
    dry=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['dry_run'])" "$out")
    if [[ "$dry" != "True" ]]; then
      diag "update-harness/dry-run/dry-run-field/$h" "expected dry_run=true in JSON, got $dry"
    fi
  done

  echo "$S4_GO_OUT" | normalize_update_json >"$WORK/s4.go.json"
  echo "$S4_NODE_OUT" | normalize_update_json >"$WORK/s4.node.json"
  echo "$S4_PY_OUT" | normalize_update_json >"$WORK/s4.py.json"
  if ! diff -u "$WORK/s4.go.json" "$WORK/s4.node.json" >"$WORK/s4.diff.go-node.txt"; then
    diag "update-harness/dry-run/go-vs-node" "JSON diverges (see diff below)
$(cat "$WORK/s4.diff.go-node.txt")"
  fi
  if ! diff -u "$WORK/s4.go.json" "$WORK/s4.py.json" >"$WORK/s4.diff.go-py.txt"; then
    diag "update-harness/dry-run/go-vs-python" "JSON diverges (see diff below)
$(cat "$WORK/s4.diff.go-py.txt")"
  fi
  if [[ "$FAIL" -eq 0 ]]; then
    ok "update-harness/dry-run/three-runtimes-identical"
  fi
fi

# ===========================================================================
# Scenario 5 — the three runtimes declare the same set of target ids, in the
# same order (drawn from scenario 1's empty-harness output, already proven
# non-vacuous above).
# ===========================================================================
if python3 -c "import json,sys" 2>/dev/null && [[ -n "${S1_GO_OUT:-}" && -n "${S1_NODE_OUT:-}" && -n "${S1_PY_OUT:-}" ]]; then
  ids_go=$(target_ids_json "$S1_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
  ids_node=$(target_ids_json "$S1_NODE_OUT" 2>/dev/null || echo "PARSE_ERROR")
  ids_py=$(target_ids_json "$S1_PY_OUT" 2>/dev/null || echo "PARSE_ERROR")

  if [[ "$ids_go" == "PARSE_ERROR" || "$ids_node" == "PARSE_ERROR" || "$ids_py" == "PARSE_ERROR" ]]; then
    diag "update-harness/target-list/vacuity-guard" "could not extract target id list from one or more runtimes"
  elif [[ -z "$ids_go" || "$ids_go" == "[]" ]]; then
    diag "update-harness/target-list/vacuity-guard" "Go declared an empty target list — nothing to compare"
  else
    if [[ "$ids_go" != "$ids_node" ]]; then
      diag "update-harness/target-list/go-vs-node" "target id list/order differs
  go:   $ids_go
  node: $ids_node"
    fi
    if [[ "$ids_go" != "$ids_py" ]]; then
      diag "update-harness/target-list/go-vs-python" "target id list/order differs
  go:     $ids_go
  python: $ids_py"
    fi
    if [[ "$ids_go" == "$ids_node" && "$ids_go" == "$ids_py" ]]; then
      ok "update-harness/target-list/three-runtimes-identical (${ids_go})"
    fi
  fi
fi

# ---------------------------------------------------------------------------
if [[ "$FAIL" -ne 0 ]]; then
  echo "check-update-parity: drift detected — see FAIL lines above." >&2
  exit 1
fi

echo "All check-update-parity.sh scenarios passed."
