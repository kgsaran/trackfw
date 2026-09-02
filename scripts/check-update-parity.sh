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

# install_claude_agents HOME_DIR — installs the claude agents target into an
# isolated HOME. Runs from a throwaway scratch cwd under $WORK (removed by
# the top-level trap), never the caller's cwd: `agents install --scope
# global` writes CLAUDE.md into the *current project* in addition to the
# redirected HOME, so without an isolated cwd this gate would mutate
# whichever repo invoked it (see vault/notes/update-parity-gate-writes-real-claude-md-2026-07-29.md).
install_claude_agents() {
  local home_dir=$1
  mkdir -p "$home_dir"
  local scratch_dir
  scratch_dir=$(mktemp -d "$WORK/agents-install-cwd.XXXXXX")
  (cd "$scratch_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets claude --scope global \
    --identity-preset neutral --json >/dev/null)
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

# ===========================================================================
# Scenario 6 — skip-parity/global-scope
#
# Proves that all three runtimes emit byte-identical skip warnings when
# `agents install` encounters a global artifact that is outdated+owned.
# The fixture simulates the outdated state by patching the manifest after a
# real install, as described in docs/cli-parity.md §install sobre artefato
# gerenciado desatualizado. ML-3A.
#
# For global scope the HOME path is taken from the env var `$HOME`, resolved
# via os.homedir()/os.UserHomeDir() which all three runtimes read without
# symlink expansion when the value is already absolute. A single Go-created
# manifest is therefore readable by all three runtimes.
# ===========================================================================

# patch_manifest_outdated MANIFEST ARTIFACT SENTINEL
# Overwrites ARTIFACT with SENTINEL (+ newline), then patches MANIFEST so that
# the entry for ARTIFACT has sha256 = sha256(SENTINEL+newline) and an old
# catalog_version. This makes the artifact state outdated+owned (sha256 still
# matches → not modified; catalog_version differs → outdated).
patch_manifest_outdated() {
  local manifest=$1 artifact=$2 sentinel=$3
  printf '%s\n' "$sentinel" > "$artifact"
  python3 - "$manifest" "$artifact" "$sentinel" <<'PY'
import json, hashlib, sys
manifest_path, artifact_key, sentinel = sys.argv[1], sys.argv[2], sys.argv[3]
new_sha = hashlib.sha256((sentinel + "\n").encode()).hexdigest()
with open(manifest_path) as f:
    data = json.load(f)
if artifact_key not in data["artifacts"]:
    print(f"patch_manifest_outdated: key {artifact_key!r} not in manifest", file=sys.stderr)
    sys.exit(1)
data["artifacts"][artifact_key]["sha256"] = new_sha
data["artifacts"][artifact_key]["catalog_version"] = "simulated-old-0.9.0"
with open(manifest_path, "w") as f:
    json.dump(data, f, indent=2)
PY
}

# install_agent_global RUNTIME HOME_DIR TARGET ITEM
# Installs a single agent into an isolated HOME using the given runtime.
# Runs from a scratch cwd to avoid mutating the caller's project.
install_agent_global() {
  local runtime=$1 home_dir=$2 target=$3 item=$4
  local scratch
  scratch=$(mktemp -d "$WORK/install-global-cwd.XXXXXX")
  case "$runtime" in
  go)   (cd "$scratch" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --items "$item" --scope global >/dev/null 2>&1) ;;
  node) (cd "$scratch" && HOME="$home_dir" node "$NODE_CLI" agents install --targets "$target" --items "$item" --scope global >/dev/null 2>&1) ;;
  py)   (cd "$scratch" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" PYTHONDONTWRITEBYTECODE=1 python3 -m trackfw agents install --targets "$target" --items "$item" --scope global >/dev/null 2>&1) ;;
  esac
}

# install_agent_project RUNTIME PROJECT_DIR HOME_DIR TARGET ITEM
# Installs a single agent into an isolated project using the given runtime.
install_agent_project() {
  local runtime=$1 project_dir=$2 home_dir=$3 target=$4 item=$5
  case "$runtime" in
  go)   (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --items "$item" --scope project >/dev/null 2>&1) ;;
  node) (cd "$project_dir" && HOME="$home_dir" node "$NODE_CLI" agents install --targets "$target" --items "$item" --scope project >/dev/null 2>&1) ;;
  py)   (cd "$project_dir" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" PYTHONDONTWRITEBYTECODE=1 python3 -m trackfw agents install --targets "$target" --items "$item" --scope project >/dev/null 2>&1) ;;
  esac
}

# run_agents_install RUNTIME HOME_DIR PROJECT_DIR TARGET ITEM SCOPE
# Sets AGENTS_INSTALL_EXIT, AGENTS_INSTALL_STDERR as globals.
run_agents_install() {
  local runtime=$1 home_dir=$2 project_dir=$3 target=$4 item=$5 scope=$6
  local err_file="$WORK/err.$$.$RANDOM"
  set +e
  case "$runtime" in
  go)   (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --items "$item" --scope "$scope" >/dev/null 2>"$err_file") ;;
  node) (cd "$project_dir" && HOME="$home_dir" node "$NODE_CLI" agents install --targets "$target" --items "$item" --scope "$scope" >/dev/null 2>"$err_file") ;;
  py)   (cd "$project_dir" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" PYTHONDONTWRITEBYTECODE=1 python3 -m trackfw agents install --targets "$target" --items "$item" --scope "$scope" >/dev/null 2>"$err_file") ;;
  *) echo "run_agents_install: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  AGENTS_INSTALL_EXIT=$?
  set -e
  AGENTS_INSTALL_STDERR=$(cat "$err_file")
  rm -f "$err_file"
}

# run_init RUNTIME HOME_DIR PROJECT_DIR AI_TOOL
# Sets INIT_EXIT, INIT_STDERR as globals.
run_init() {
  local runtime=$1 home_dir=$2 project_dir=$3 tool=$4
  local err_file="$WORK/err.$$.$RANDOM"
  set +e
  case "$runtime" in
  go)   (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" init --ai-tools "$tool" >/dev/null 2>"$err_file") ;;
  node) (cd "$project_dir" && HOME="$home_dir" node "$NODE_CLI" init --ai-tools "$tool" >/dev/null 2>"$err_file") ;;
  py)   (cd "$project_dir" && HOME="$home_dir" PYTHONPATH="$PY_ROOT" PYTHONDONTWRITEBYTECODE=1 python3 -m trackfw init --ai-tools "$tool" >/dev/null 2>"$err_file") ;;
  *) echo "run_init: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  INIT_EXIT=$?
  set -e
  INIT_STDERR=$(cat "$err_file")
  rm -f "$err_file"
}

SKIP_SENTINEL="OUTDATED-SENTINEL-DO-NOT-OVERWRITE-by-check-update-parity"

# Scenario 6: skip warning byte-identical, global scope.
# One fresh HOME per runtime; fixture built by the respective runtime.
S6_PROJ="$WORK/s6-project"
mkdir -p "$S6_PROJ"
for h in go node py; do
  home="$WORK/s6-home-$h"
  install_agent_global "$h" "$home" "gemini" "architect"
  artifact="$home/.gemini/agents/trackfw-architect.md"
  manifest="$home/.trackfw/integrations-manifest.json"
  art_key=$(python3 -c "import json; d=json.load(open('$manifest')); print(list(d['artifacts'].keys())[0])")
  patch_manifest_outdated "$manifest" "$art_key" "$SKIP_SENTINEL"
done

run_agents_install go   "$WORK/s6-home-go"   "$S6_PROJ" "gemini" "architect" "global"
S6_GO_EXIT=$AGENTS_INSTALL_EXIT
S6_GO_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)
run_agents_install node "$WORK/s6-home-node" "$S6_PROJ" "gemini" "architect" "global"
S6_NODE_EXIT=$AGENTS_INSTALL_EXIT
S6_NODE_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)
run_agents_install py   "$WORK/s6-home-py"   "$S6_PROJ" "gemini" "architect" "global"
S6_PY_EXIT=$AGENTS_INSTALL_EXIT
S6_PY_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)

for pair in "go:$S6_GO_EXIT" "node:$S6_NODE_EXIT" "python:$S6_PY_EXIT"; do
  rt=${pair%%:*}; ec=${pair##*:}
  if [[ "$ec" != "0" ]]; then
    diag "skip-parity/global-scope/exit-zero" "$rt exited $ec — install over outdated+owned must be exit 0"
  fi
done

# Vacuity guard: Go must have produced a non-empty skip warning.
if [[ -z "${S6_GO_WARN:-}" ]]; then
  diag "skip-parity/global-scope/vacuity-guard" "Go emitted no skip warning — fixture may be broken; nothing to compare"
else
  ok "skip-parity/global-scope/vacuity-guard"

  if [[ "$S6_GO_WARN" != "$S6_NODE_WARN" ]]; then
    diag "skip-parity/global-scope/go-vs-node" "skip warning diverges
  go:   $S6_GO_WARN
  node: $S6_NODE_WARN"
  fi
  if [[ "$S6_GO_WARN" != "$S6_PY_WARN" ]]; then
    diag "skip-parity/global-scope/go-vs-python" "skip warning diverges
  go:     $S6_GO_WARN
  python: $S6_PY_WARN"
  fi
  if [[ "$S6_GO_WARN" == "$S6_NODE_WARN" && "$S6_GO_WARN" == "$S6_PY_WARN" ]]; then
    ok "skip-parity/global-scope/three-runtimes-identical"
  fi
fi

# ===========================================================================
# Scenario 7 — skip-parity/project-scope
#
# Proves byte-identical skip warnings for project scope. Each runtime sets up
# its own project fixture because Node.js and Python resolve process.cwd()
# through /private/ on macOS (symlink), producing absolute paths different
# from Go's filepath.Abs output for the same directory. Using runtime-specific
# manifests sidesteps this platform difference.
# ===========================================================================
for h in go node py; do
  proj="$WORK/s7-proj-$h"
  home="$WORK/s7-home-$h"
  mkdir -p "$proj" "$home"
  install_agent_project "$h" "$proj" "$home" "claude" "architect"
  manifest="$proj/.trackfw/integrations-manifest.json"
  art_key=$(python3 -c "import json; d=json.load(open('$manifest')); print(list(d['artifacts'].keys())[0])")
  artifact="$art_key"  # manifest key IS the absolute artifact path for project scope
  patch_manifest_outdated "$manifest" "$artifact" "$SKIP_SENTINEL"
done

S7_PROJ_GO="$WORK/s7-proj-go";   S7_HOME_GO="$WORK/s7-home-go"
S7_PROJ_NODE="$WORK/s7-proj-node"; S7_HOME_NODE="$WORK/s7-home-node"
S7_PROJ_PY="$WORK/s7-proj-py";   S7_HOME_PY="$WORK/s7-home-py"

run_agents_install go   "$S7_HOME_GO"   "$S7_PROJ_GO"   "claude" "architect" "project"
S7_GO_EXIT=$AGENTS_INSTALL_EXIT
S7_GO_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)
run_agents_install node "$S7_HOME_NODE" "$S7_PROJ_NODE" "claude" "architect" "project"
S7_NODE_EXIT=$AGENTS_INSTALL_EXIT
S7_NODE_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)
run_agents_install py   "$S7_HOME_PY"   "$S7_PROJ_PY"   "claude" "architect" "project"
S7_PY_EXIT=$AGENTS_INSTALL_EXIT
S7_PY_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)

for pair in "go:$S7_GO_EXIT" "node:$S7_NODE_EXIT" "python:$S7_PY_EXIT"; do
  rt=${pair%%:*}; ec=${pair##*:}
  if [[ "$ec" != "0" ]]; then
    diag "skip-parity/project-scope/exit-zero" "$rt exited $ec — install over outdated+owned must be exit 0"
  fi
done

if [[ -z "${S7_GO_WARN:-}" ]]; then
  diag "skip-parity/project-scope/vacuity-guard" "Go emitted no skip warning — fixture may be broken; nothing to compare"
else
  ok "skip-parity/project-scope/vacuity-guard"

  if [[ "$S7_GO_WARN" != "$S7_NODE_WARN" ]]; then
    diag "skip-parity/project-scope/go-vs-node" "skip warning diverges
  go:   $S7_GO_WARN
  node: $S7_NODE_WARN"
  fi
  if [[ "$S7_GO_WARN" != "$S7_PY_WARN" ]]; then
    diag "skip-parity/project-scope/go-vs-python" "skip warning diverges
  go:     $S7_GO_WARN
  python: $S7_PY_WARN"
  fi
  if [[ "$S7_GO_WARN" == "$S7_NODE_WARN" && "$S7_GO_WARN" == "$S7_PY_WARN" ]]; then
    ok "skip-parity/project-scope/three-runtimes-identical"
  fi
fi

# ===========================================================================
# Scenario 8 — E2E: init with outdated global artifact
#
# The original symptom (REQ-2026-07-29-install-pula-artefato-desatualizado-
# em-vez-de-abortar): `trackfw init --ai-tools gemini` aborted when a global
# artifact was outdated+owned. This scenario proves exit 0 + scaffold complete
# + bytes preserved + sibling artifact written (skip ≠ abort) for all three
# runtimes.
#
# Strategy: install gemini/architect globally (using the respective runtime),
# patch to outdated+owned (sentinel bytes), run `init --ai-tools gemini`,
# then assert the four criteria. For global scope the Go-created manifest is
# readable by all three runtimes (HOME path not symlink-expanded by
# os.homedir()/os.UserHomeDir()), so this scenario uses each runtime for
# both install and init to keep the manifest paths consistent.
# ===========================================================================
for h in go node py; do
  home="$WORK/s8-home-$h"
  proj="$WORK/s8-proj-$h"
  mkdir -p "$home" "$proj"
  install_agent_global "$h" "$home" "gemini" "architect"
  manifest="$home/.trackfw/integrations-manifest.json"
  art_key=$(python3 -c "import json; d=json.load(open('$manifest')); print(list(d['artifacts'].keys())[0])")
  patch_manifest_outdated "$manifest" "$art_key" "$SKIP_SENTINEL"
done

for h in go node py; do
  home="$WORK/s8-home-$h"
  proj="$WORK/s8-proj-$h"
  architect="$home/.gemini/agents/trackfw-architect.md"
  backend="$home/.gemini/agents/trackfw-backend.md"

  run_init "$h" "$home" "$proj" "gemini"
  if [[ "$INIT_EXIT" -ne 0 ]]; then
    diag "e2e/init-outdated-global/$h/exit-zero" "init exited $INIT_EXIT — expected 0 with outdated+owned global artifact"
    continue
  fi

  # (b) project scaffold: trackfw.yaml must exist
  if [[ ! -f "$proj/trackfw.yaml" ]]; then
    diag "e2e/init-outdated-global/$h/scaffold" "trackfw.yaml missing — scaffold incomplete"
  fi

  # (c) outdated artifact bytes preserved (sentinel untouched)
  if [[ ! -f "$architect" ]]; then
    diag "e2e/init-outdated-global/$h/bytes-preserved" "architect artifact was removed"
  elif ! grep -qF "$SKIP_SENTINEL" "$architect"; then
    diag "e2e/init-outdated-global/$h/bytes-preserved" "sentinel bytes overwritten — skip did not preserve artifact"
  fi

  # (c-sibling) a sibling gemini artifact was written (proves skip ≠ abort)
  if [[ ! -f "$backend" ]]; then
    diag "e2e/init-outdated-global/$h/sibling-written" "trackfw-backend.md was not written — init may have aborted"
  fi

  # (d) skip warning in stderr
  warn_line=$(grep '^warning: skipping outdated artifact ' <<<"$INIT_STDERR" || true)
  if [[ -z "$warn_line" ]]; then
    diag "e2e/init-outdated-global/$h/warning-in-stderr" "no skip warning in stderr — expected 'warning: skipping outdated artifact'"
  fi

  if [[ "$FAIL" -eq 0 ]]; then
    ok "e2e/init-outdated-global/$h"
  fi
done

# ===========================================================================
# Scenario 9 — Sandbox by inclusion: dangling symlink OUTSIDE the declared
# set (.venv/bin/python → nonexistent) does NOT abort --dry-run.
#
# Before ML-1A, copyProjectTree walked the whole project tree; os.ReadFile on
# a broken symlink returned "no such file or directory" and aborted. Now
# copyProjectTree only visits the declared relPaths — .venv/bin/python is not
# in any target's relPaths, so the sandbox never touches it.
#
# Vacuity guard: JSON output must have dry_run=true (proves --dry-run ran to
# completion and emitted a real report, not nothing).
# ===========================================================================
S9_PROJ="$WORK/s9-proj"
mkdir -p "$S9_PROJ/.venv/bin"
ln -sf /nonexistent-python3.99 "$S9_PROJ/.venv/bin/python"
cat > "$S9_PROJ/trackfw.yaml" << 'S9YAML'
name: s9
S9YAML

for rt in go node py; do
  run_update "$rt" "$WORK/s9-home-$rt" "$S9_PROJ" --dry-run --json
  # Vacuity guard: must have produced JSON with dry_run=true
  s9_dry_flag=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(str(d.get('dry_run', 'MISSING')).lower())
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$s9_dry_flag" != "true" ]]; then
    diag "sandbox/dangling-outside-set/vacuity/$rt" "dry_run field not true ($s9_dry_flag) — fixture broken or output unparseable"
  fi
  if [[ "$UPDATE_EXIT" != "0" ]]; then
    diag "sandbox/dangling-outside-set/exit-zero/$rt" "$rt exited $UPDATE_EXIT — dangling symlink outside declared set must not abort --dry-run"
  fi
done

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/dangling-outside-set"
fi

# ===========================================================================
# Scenario 10 — Sandbox by inclusion: dangling symlink INSIDE the declared
# set (CLAUDE.md → nonexistent) is treated as absent, not as an error.
#
# CLAUDE.md is in agent-rules relPaths. copyPath uses os.Lstat, which
# succeeds on the symlink itself (does not follow it), then detects the
# symlink and returns without copying — so the sandbox slot is empty.
# hashRelPaths then returns "" for CLAUDE.md → allEmpty → state=missing.
# The run must exit 0 with state=missing (not abort).
#
# Vacuity guard: state must be 'missing' — proves the symlink was processed
# (not skipped because of an early bail-out) and the declared relPath is
# correctly treated as absent.
# ===========================================================================
S10_PROJ="$WORK/s10-proj"
mkdir -p "$S10_PROJ"
cat > "$S10_PROJ/trackfw.yaml" << 'S10YAML'
name: s10
S10YAML
ln -sf /nonexistent-claude "$S10_PROJ/CLAUDE.md"

for rt in go node py; do
  run_update "$rt" "$WORK/s10-home-$rt" "$S10_PROJ" --dry-run --json --targets agent-rules
  s10_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$s10_state" == "PARSE_ERROR"* ]]; then
    diag "sandbox/dangling-inside-set/vacuity/$rt" "output unparseable: $s10_state"
  elif [[ "$s10_state" != "missing" ]]; then
    diag "sandbox/dangling-inside-set/state/$rt" "$rt: expected state=missing for CLAUDE.md broken symlink, got '$s10_state'"
  fi
  if [[ "$UPDATE_EXIT" != "0" ]]; then
    diag "sandbox/dangling-inside-set/exit-zero/$rt" "$rt exited $UPDATE_EXIT — broken symlink inside declared set must not abort"
  fi
done

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/dangling-inside-set"
fi

# ===========================================================================
# Scenario 11 — Gap E: trackfw.yaml with agent_conventions — dry-run and
# real run report the SAME state for agent-rules.
#
# agent-rules calls injectOrUpdateRules(cwd), which reads agent_conventions
# from trackfw.yaml in the sandbox (or real root). If trackfw.yaml is absent
# from the sandbox, the convention block is omitted and the content hash
# diverges from the real run's output — dry-run would say 'skipped' while
# real says 'updated', a lie by omission (Gap E in the threat model).
#
# Fixture: CLAUDE.md seeded with the current trackfw block (no convention),
# trackfw.yaml with agent_conventions set. Both dry and real arms start from
# an identical copy so only the sandbox vs real-root difference can explain
# any state divergence.
#
# Vacuity guard: real-run state must be 'updated' — proves the convention was
# actually absent from the fixture and was genuinely inserted by the real run.
# Cross-runtime: all 3 dry-run states must be identical.
# ===========================================================================

# Seed CLAUDE.md with the current block (no conventions) via a real run
S11_SEED="$WORK/s11-seed"
mkdir -p "$S11_SEED"
echo "# S11 Seed" > "$S11_SEED/CLAUDE.md"
cat > "$S11_SEED/trackfw.yaml" << 'S11SEED'
name: s11-seed
S11SEED
(cd "$S11_SEED" && HOME="$WORK/s11-seed-home" "$GO_BIN" update --targets agent-rules >/dev/null 2>&1) || true

# Verify seed produced CLAUDE.md with trackfw block
if [[ ! -f "$S11_SEED/CLAUDE.md" ]]; then
  diag "sandbox/gap-e/seed-setup" "s11 seed: Go update did not write CLAUDE.md — fixture cannot be built"
  S11_SEED_OK=0
else
  S11_SEED_OK=1
fi

if [[ "$S11_SEED_OK" -eq 1 ]]; then
  # Now add agent_conventions and prepare per-runtime fixtures
  for rt in go node py; do
    S11_DRY="$WORK/s11-dry-$rt"
    S11_REAL="$WORK/s11-real-$rt"
    mkdir -p "$S11_DRY" "$S11_REAL"
    cp "$S11_SEED/CLAUDE.md" "$S11_DRY/CLAUDE.md"
    cp "$S11_SEED/CLAUDE.md" "$S11_REAL/CLAUDE.md"
    # trackfw.yaml WITH agent_conventions (triggers the convention block)
    cat > "$S11_DRY/trackfw.yaml" << 'S11YAML'
name: s11
agent_conventions: "Always commit tests alongside code"
S11YAML
    cp "$S11_DRY/trackfw.yaml" "$S11_REAL/trackfw.yaml"

    run_update "$rt" "$WORK/s11-home-dry-$rt" "$S11_DRY" --dry-run --json --targets agent-rules
    s11_dry_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

    run_update "$rt" "$WORK/s11-home-real-$rt" "$S11_REAL" --json --targets agent-rules
    s11_real_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

    # Vacuity guard: real state must be 'updated'
    if [[ "$s11_real_state" != "updated" ]]; then
      diag "sandbox/gap-e/vacuity/$rt" "real-run state='$s11_real_state' (expected 'updated') — agent_conventions fixture may be broken or convention already present"
    fi

    # The main assertion: dry == real
    if [[ "$s11_dry_state" != "$s11_real_state" ]]; then
      diag "sandbox/gap-e/dry-vs-real/$rt" "dry=$s11_dry_state real=$s11_real_state — dry-run reported different state than real run for agent_conventions fixture; trackfw.yaml may be missing from sandbox"
    fi
  done

  if [[ "$FAIL" -eq 0 ]]; then
    ok "sandbox/gap-e/dry-vs-real/three-runtimes"
  fi
fi

# ===========================================================================
# Scenario 12 — Gap C: .github/copilot-instructions.md present — dry-run and
# real run agree on the agent-hooks target state.
#
# InjectHooksDetected checks for .github/copilot-instructions.md to decide
# whether to call InjectCopilotHooks (which writes .github/hooks/trackfw-
# attention.json, declared in agent-hooks relPaths). If the detection signal
# is absent from the sandbox, the copilot hook is silently omitted — dry-run
# says 'missing' or 'skipped' while real run says 'updated' (Gap C).
#
# Fixture: .github/copilot-instructions.md present + .claude/settings.json
# (one existing hook file so allMissingBefore is false and apply is called).
# .github/hooks/trackfw-attention.json absent (so the copilot hook write is
# a genuine change).
#
# Vacuity guard: .github/hooks/trackfw-attention.json must be written in the
# real-run fixture — proves the copilot hook injection actually fired.
# Cross-runtime: all 3 dry-run states must match real-run states.
# ===========================================================================
for rt in go node py; do
  S12_DRY="$WORK/s12-dry-$rt"
  S12_REAL="$WORK/s12-real-$rt"
  mkdir -p "$S12_DRY/.github" "$S12_DRY/.claude" "$S12_REAL/.github" "$S12_REAL/.claude"

  cat > "$S12_DRY/trackfw.yaml" << 'S12YAML'
name: s12
S12YAML
  touch "$S12_DRY/.github/copilot-instructions.md"
  echo '{}' > "$S12_DRY/.claude/settings.json"

  cp "$S12_DRY/trackfw.yaml" "$S12_REAL/trackfw.yaml"
  touch "$S12_REAL/.github/copilot-instructions.md"
  echo '{}' > "$S12_REAL/.claude/settings.json"

  run_update "$rt" "$WORK/s12-home-dry-$rt" "$S12_DRY" --dry-run --json --targets agent-hooks
  s12_dry_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  run_update "$rt" "$WORK/s12-home-real-$rt" "$S12_REAL" --json --targets agent-hooks
  s12_real_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  # Vacuity guard: copilot hook must have been written in real run
  if [[ ! -f "$S12_REAL/.github/hooks/trackfw-attention.json" ]]; then
    diag "sandbox/gap-c/vacuity/$rt" ".github/hooks/trackfw-attention.json not created in real run — fixture may be missing .github/copilot-instructions.md or fixture is broken"
  fi

  # Main assertion: dry == real
  if [[ "$s12_dry_state" != "$s12_real_state" ]]; then
    diag "sandbox/gap-c/dry-vs-real/$rt" "dry=$s12_dry_state real=$s12_real_state — copilot detection signal (.github/copilot-instructions.md) may be missing from sandbox"
  fi
done

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/gap-c/dry-vs-real/three-runtimes"
fi

# ===========================================================================
# Scenario 13 — Gap A/B: .windsurf/hooks.json and .amazonq/cli-agents/
# q_cli_default.json appear in the agent-hooks declared path list.
#
# InjectWindsurfHooks and InjectAmazonQHooks write these two paths but they
# were absent from the relPaths declaration (Gap A and B in the threat model).
# ML-1A added them. This scenario proves all three runtimes declare them.
#
# Fixture: .windsurfrules present (triggers windsurf detection) + .amazonq/
# dir + .claude/settings.json (ensures allMissingBefore is false so path
# field reflects the full declared list, not an early-exit missing state).
#
# Vacuity guard: state must not be 'missing' — confirms agent-hooks actually
# ran apply and the path field is the full declaration, not the short-circuit
# path that skips apply.
# ===========================================================================
S13_PROJ="$WORK/s13-proj"
mkdir -p "$S13_PROJ/.amazonq" "$S13_PROJ/.claude"
cat > "$S13_PROJ/trackfw.yaml" << 'S13YAML'
name: s13
S13YAML
touch "$S13_PROJ/.windsurfrules"
echo '{}' > "$S13_PROJ/.claude/settings.json"

S13_GO_STATE="" S13_NODE_STATE="" S13_PY_STATE=""
for rt in go node py; do
  run_update "$rt" "$WORK/s13-home-$rt" "$S13_PROJ" --dry-run --json --targets agent-hooks
  s13_path=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['path'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")
  s13_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  case "$rt" in
    go)   S13_GO_STATE="$s13_state"   ;;
    node) S13_NODE_STATE="$s13_state" ;;
    py)   S13_PY_STATE="$s13_state"   ;;
  esac

  # Vacuity guard: must not be missing (apply must have run)
  if [[ "$s13_state" == "missing" ]]; then
    diag "sandbox/gap-ab/vacuity/$rt" "agent-hooks state=missing — fixture may be broken (no existing hook file to trigger apply)"
  fi

  if [[ "$s13_path" == "PARSE_ERROR"* ]]; then
    diag "sandbox/gap-ab/parse/$rt" "output unparseable: $s13_path"
  else
    # Gap A: .windsurf/hooks.json in declared path
    if ! echo "$s13_path" | grep -qF '.windsurf/hooks.json'; then
      diag "sandbox/gap-a/declared-path/$rt" ".windsurf/hooks.json absent from agent-hooks path field: $s13_path"
    fi
    # Gap B: .amazonq/cli-agents/q_cli_default.json in declared path
    if ! echo "$s13_path" | grep -qF '.amazonq/cli-agents/q_cli_default.json'; then
      diag "sandbox/gap-b/declared-path/$rt" ".amazonq/cli-agents/q_cli_default.json absent from agent-hooks path field: $s13_path"
    fi
  fi
done

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/gap-ab/declared-path/three-runtimes"
fi

# ===========================================================================
# Scenario 14 — R-novo-1 fix: declared directory already correct
#
# claude-commands writes .claude/commands/trackfw (a directory). Before the
# R-novo-1 fix, Go and Python copyPath created the directory but did NOT
# recurse into its contents — sandbox received an empty directory. The
# before-hash was sha256("") and the after-hash was sha256(real content), so
# dry-run reported `updated` even when real run would report `skipped`
# (directory already correct).
#
# This scenario proves the fix: prime the project with a real run, then run
# dry vs real — both must report `skipped` in all three runtimes.
#
# Vacuity guard 1: .claude/commands/trackfw is non-empty after prime (proves
#   the prime actually wrote something — fixture is not trivially broken).
# Vacuity guard 2: real state == skipped (proves the directory content was
#   already correct when the arms ran — the idempotency scenario holds).
# ===========================================================================
S14_FAIL_BEFORE="$FAIL"
for rt in go node py; do
  S14_PROJ="$WORK/s14-proj-$rt"
  mkdir -p "$S14_PROJ"
  cat > "$S14_PROJ/trackfw.yaml" << 'S14YAML'
name: s14
S14YAML

  # Prime: real run with --install-missing to populate .claude/commands/trackfw.
  # Without --install-missing, the target returns 'missing' without calling apply
  # when the directory doesn't exist yet (runFileTarget/allEmpty guard).
  run_update "$rt" "$WORK/s14-home-$rt" "$S14_PROJ" --targets claude-commands --install-missing

  # Vacuity guard 1: directory must exist and be non-empty
  if [[ ! -d "$S14_PROJ/.claude/commands/trackfw" ]]; then
    diag "sandbox/dir-already-correct/vacuity-dir-exists/$rt" ".claude/commands/trackfw not created by prime real run — target broken or fixture invalid"
    continue
  fi
  if [[ -z "$(ls -A "$S14_PROJ/.claude/commands/trackfw" 2>/dev/null)" ]]; then
    diag "sandbox/dir-already-correct/vacuity-dir-nonempty/$rt" ".claude/commands/trackfw is empty after prime — claude-commands wrote no files"
    continue
  fi

  # Dry-run arm: same project, directory already correct
  run_update "$rt" "$WORK/s14-home-$rt" "$S14_PROJ" --dry-run --json --targets claude-commands
  s14_dry_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  # Real arm: same project, idempotent second run
  run_update "$rt" "$WORK/s14-home-$rt" "$S14_PROJ" --json --targets claude-commands
  s14_real_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  # Vacuity guard 2: real must be skipped (proves idempotency — content was already correct)
  if [[ "$s14_real_state" != "skipped" ]]; then
    diag "sandbox/dir-already-correct/vacuity-real-skipped/$rt" "real-run state='$s14_real_state' (expected 'skipped') — prime did not produce correct content, or target is non-idempotent"
  fi

  # Main assertion: dry == real
  if [[ "$s14_dry_state" != "$s14_real_state" ]]; then
    diag "sandbox/dir-already-correct/dry-vs-real/$rt" "dry=$s14_dry_state real=$s14_real_state — dry-run diverged from real for already-correct claude-commands directory; copyPath may not be recursing directory contents into sandbox"
  fi
done

if [[ "$FAIL" -eq "$S14_FAIL_BEFORE" ]]; then
  ok "sandbox/dir-already-correct/dry-vs-real/three-runtimes"
fi

# ---------------------------------------------------------------------------
if [[ "$FAIL" -ne 0 ]]; then
  echo "check-update-parity: drift detected — see FAIL lines above." >&2
  exit 1
fi

echo "All check-update-parity.sh scenarios passed."
