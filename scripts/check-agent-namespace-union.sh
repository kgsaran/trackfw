#!/usr/bin/env bash
# check-agent-namespace-union.sh — gate for REQ-2026-08-29 (Wave 3 / ML-3A,
# ROADMAP-2026-08-29-lista-de-agentes-complementa-o-disco-e-namespace-nao-
# declarado-vira-violacao.md): in `roadmap_namespacing: by_agent`, `agents:`
# COMPLEMENTS the disk (union) instead of substituting it, and a namespace
# present on disk but absent from `agents:` becomes a VIOLATION — without the
# union ever depending on that violation being active (AC5, the property that
# defines the REQ).
#
# Covers, across the 3 runtimes (Go/Node/Python):
#   AC1  — union: an undeclared-but-on-disk namespace is enumerated by
#          `status`, `validate` and `roadmap move`.
#   AC4  — violation message is byte-identical in the 3 runtimes.
#   AC5  — independence in BOTH directions: (a) declaring the namespace
#          silences the violation without hiding the artifacts; (b) the
#          artifacts stay enumerated WHILE the violation is active.
#   AC12 — the disk scan never follows a symlink (`roadmap move` must not
#          write outside the project through a namespace dir that is a
#          symlink).
#   infra filter — `.git`, `node_modules` and an orphan state-name dir
#          (`wip/` alone at the top of roadmap_dir) never trigger the
#          undeclared-namespace violation.
#   flat  — `roadmap_namespacing: flat` never emits `agent_namespace_undeclared`.
#
# Falsification, BOTH directions, SELF-CONTAINED (this script has no external
# caller — check-gates-falsify.sh is out of scope for new scenarios in this
# ML, see roadmap ML-3A file list — so the corruption+detection lives here,
# same technique as check-gates-falsify.sh's corrupt_literal, duplicated
# rather than sourced to keep this gate independently runnable):
#   Direction A  — union reverts to substitution (the pre-REQ-2026-08-29
#                  defect): an undeclared-but-on-disk namespace goes
#                  invisible again the moment `agents:` is non-empty.
#   Direction B1 — infra filter disabled: `.git`/`node_modules` start being
#                  treated as namespaces (the REQ's "ADR-2026-08-17: a guard
#                  that annoys is a guard that gets turned off" failure mode).
#   Direction B2 — AC12 regression: the disk scan starts following symlinks
#                  again (reproduced live in Node/Python during ML-0A;
#                  Go's os.ReadDir()+entry.IsDir() never follows a symlink by
#                  API design — dirent.d_type comes from the parent directory
#                  entry, not a stat() of the target — so there is no small
#                  literal edit that reintroduces the vector in Go the same
#                  way; a Go arm here would need to replace the whole
#                  primitive with a stat-based walk, which is not what "one
#                  wrong edit regressed this" looks like for Go. Proven only
#                  in Node/Python, where it was proven live in ML-0A).
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-agent-namespace-union.XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

# Isolated $HOME — same reason as check-gates-falsify.sh: without this, any
# `validate` run here would see the REAL global guard scope of whoever runs
# this gate, contaminating scenarios that have nothing to do with guards.
export HOME="$WORK/home"
mkdir -p "$HOME"
export NO_COLOR=1
export TERM=dumb

# Real Go caches preserved so `go build` here stays fast (same as
# check-gates-falsify.sh / check-audit-surface.sh).
export GOPATH="${GOPATH:-$(go env GOPATH)}"
export GOCACHE="${GOCACHE:-$(go env GOCACHE)}"
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"

SCENARIOS=0
ok()   { SCENARIOS=$((SCENARIOS + 1)); echo "OK   [agent-namespace-union/$1]"; }
fail() { echo "FAIL [agent-namespace-union/$1]: $2" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Resolve the three runtimes. GO_BIN may be passed in (absolute or relative
# to ROOT_DIR, as the Makefile does with GO_BIN=$(BUILD_DIR)/$(BINARY));
# otherwise build a throwaway binary.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && env GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  fail "setup/go-binary" "not found/executable at $GO_BIN"
fi
if [[ ! -f "$NODE_CLI" ]]; then
  fail "setup/node-cli" "not found at $NODE_CLI"
fi

# ---------------------------------------------------------------------------
# Helper: corrupts exactly 1 occurrence of `old` into `new`, writing to
# `dest`. Fails loudly if the literal isn't unique — same contract as
# check-gates-falsify.sh's corrupt_literal, duplicated here (this gate has
# no caller to source it from, and is not itself allowed to touch
# check-gates-falsify.sh outside the ML-3A retarget).
# ---------------------------------------------------------------------------
corrupt_literal() {
  local src=$1 dest=$2 old=$3 new=$4 label=$5
  python3 - "$src" "$dest" "$old" "$new" "$label" <<'PY'
import pathlib
import sys

src, dest, old, new, label = sys.argv[1:6]
source = pathlib.Path(src).read_text(encoding="utf-8")
count = source.count(old)
if count != 1:
    raise SystemExit(f"[{label}] expected exactly 1 occurrence of pattern, got {count}")
pathlib.Path(dest).write_text(source.replace(old, new, 1), encoding="utf-8")
PY
}

build_go_or_fail() {
  local label=$1 module_dir=$2 output_bin=$3
  local log_file="$WORK/${label//\//_}.log"
  set +e
  (cd "$module_dir" && env GOCACHE="$WORK/go-build-cache" go build -o "$output_bin" ./cmd/trackfw) \
    >"$log_file" 2>&1
  local status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "  go build output:" >&2
    sed 's/^/    /' "$log_file" >&2
    fail "$label" "go build exited $status"
  fi
}

setup_npm_tree() {
  local dest=$1
  mkdir -p "$dest/npm/bin" "$dest/npm/src"
  cp "$ROOT_DIR/npm/bin/trackfw" "$dest/npm/bin/trackfw"
  ln -s "$ROOT_DIR/npm/node_modules" "$dest/npm/node_modules"
  cp "$ROOT_DIR/npm/package.json" "$dest/npm/package.json"
  cp -r "$ROOT_DIR/npm/src/." "$dest/npm/src/"
}

setup_py_tree() {
  local dest=$1
  mkdir -p "$dest"
  cp -r "$ROOT_DIR/pypi" "$dest/pypi"
}

# ---------------------------------------------------------------------------
# Fixture builders.
# ---------------------------------------------------------------------------

# scaffold_by_agent DEST AGENTS_YAML_BLOCK
# AGENTS_YAML_BLOCK is the raw YAML lines for the `agents:` key (may be
# empty string for "key omitted entirely" — not used by this gate, kept
# only for symmetry with other fixture builders in this repo's gates).
scaffold_by_agent() {
  local dest=$1 agents_block=$2
  mkdir -p "$dest/docs/adr" "$dest/docs/req"
  mkdir -p "$dest/docs/roadmaps"
  {
    echo "governance_mode: strict"
    echo "adr_dirs:"
    echo "  - docs/adr"
    echo "req_dir: docs/req"
    echo "roadmap_dir: docs/roadmaps"
    echo "roadmap_namespacing: by_agent"
    if [[ -n "$agents_block" ]]; then
      echo "agents:"
      echo "$agents_block"
    fi
  } > "$dest/trackfw.yaml"
}

write_wip_roadmap() {
  local dest=$1 title=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: wip
date: 2026-08-29
req: ""
---
# Roadmap: $title

> Created: 2026-08-29 | Status: wip

## Acceptance Criteria
- [ ] x
EOF
}

write_req_placeholder() {
  local dest=$1 title=$2
  mkdir -p "$(dirname "$dest")"
  cat > "$dest" <<EOF
---
status: Open
date: 2026-08-29
author: ""
adr: ""
roadmap: ""
---

# REQ: $title

> Date: 2026-08-29 | Status: Open

## Motivation
fixture

## Acceptance Criteria
- [ ] x

## Linked ADR
ADR:

## Linked Roadmap
Roadmap:
EOF
}

# ===========================================================================
# Fixture P1 — the sonda project: `agents: [alice]` declared; `bob` exists
# ONLY on disk, in both roadmap_dir and req_dir (exercises AC4's
# "roadmap_dir, req_dir" tree-listing in the violation message).
# ===========================================================================
P1="$WORK/p1-union"
scaffold_by_agent "$P1" "- alice"
mkdir -p "$P1/docs/roadmaps/alice"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$P1/docs/roadmaps/bob"/{backlog,analyzing,wip,blocked,done,abandoned}
mkdir -p "$P1/docs/req/bob"
write_wip_roadmap "$P1/docs/roadmaps/alice/wip/ROADMAP-alice-wip.md" "alice wip (declared)"
write_wip_roadmap "$P1/docs/roadmaps/bob/wip/ROADMAP-bob-wip.md" "bob wip (só-disco, não declarado)"
write_req_placeholder "$P1/docs/req/bob/REQ-bob-placeholder.md" "bob placeholder"

MSG_BOB_UNDECLARED='agent namespace "bob" exists in roadmap_dir, req_dir but is not declared in agents: — add it to trackfw.yaml'

run_go()     { (cd "$1" && "$GO_BIN" "${@:2}") 2>&1; }
run_node()   { (cd "$1" && node "$NODE_CLI" "${@:2}") 2>&1; }
run_python() { (cd "$1" && env PYTHONPATH="$PY_ROOT" python3 -m trackfw "${@:2}") 2>&1; }

# ===========================================================================
# AC1 — union: status/validate/roadmap move all see `bob`, in the 3 runtimes.
# ===========================================================================
for runtime in go node python; do
  status_out=$(run_"$runtime" "$P1" status; true)
  if ! grep -qF "ROADMAP-bob-wip.md" <<<"$status_out"; then
    fail "ac1/$runtime/status-enumerates-undeclared" "'status' did not list bob's roadmap — union not applied"
  fi
  ok "ac1/$runtime/status-enumerates-undeclared"

  validate_out=$(run_"$runtime" "$P1" validate; true)
  if ! grep -qF 'roadmap "ROADMAP-bob-wip.md" is in wip but has no linked REQ' <<<"$validate_out"; then
    fail "ac1/$runtime/validate-scans-undeclared" "'validate' did not scan bob's roadmap file content — union not applied to validate"
  fi
  ok "ac1/$runtime/validate-scans-undeclared"
done

# `roadmap move` — exercised once per runtime against an isolated copy of P1
# (the move is destructive; each runtime needs its own untouched fixture).
for runtime in go node python; do
  P1_MOVE="$WORK/p1-move-$runtime"
  cp -r "$P1" "$P1_MOVE"
  move_out=$(run_"$runtime" "$P1_MOVE" roadmap move ROADMAP-bob-wip done; true)
  if [[ ! -f "$P1_MOVE/docs/roadmaps/bob/done/ROADMAP-bob-wip.md" ]]; then
    fail "ac1/$runtime/roadmap-move-finds-undeclared" "roadmap move did not relocate bob's roadmap to done/ — output: $(printf '%q' "$move_out")"
  fi
  ok "ac1/$runtime/roadmap-move-finds-undeclared"
done

# ===========================================================================
# AC4 — violation message byte-identical in the 3 runtimes.
# ===========================================================================
for runtime in go node python; do
  validate_out=$(run_"$runtime" "$P1" validate; true)
  if ! grep -qF "$MSG_BOB_UNDECLARED" <<<"$validate_out"; then
    fail "ac4/$runtime/violation-message" "expected byte-identical message absent — output: $(printf '%q' "$validate_out")"
  fi
  ok "ac4/$runtime/violation-message"
done

# ===========================================================================
# AC5 — independence, both directions.
#   (a) declaring bob silences the violation, keeps the artifact enumerated.
#   (b) with the violation ACTIVE (P1 as-is, bob undeclared), the artifact
#       stays enumerated — asserted here as a SINGLE conjunction on ONE
#       `validate` invocation per runtime (not inferred from two separately
#       passing scenarios elsewhere): both the violation message AND the
#       wip_has_req evidence that bob's file was scanned must be present in
#       the SAME output. This is the property the roadmap calls "the most
#       important scenario" — if the union ever became gated by the
#       violation being active, this is what would catch it.
# ===========================================================================
for runtime in go node python; do
  validate_out=$(run_"$runtime" "$P1" validate; true)
  if ! grep -qF "$MSG_BOB_UNDECLARED" <<<"$validate_out"; then
    fail "ac5/$runtime/independence-b-enumeration-with-violation-active" "violation absent — cannot prove independence without it being active first (output: $(printf '%q' "$validate_out"))"
  fi
  if ! grep -qF 'roadmap "ROADMAP-bob-wip.md" is in wip but has no linked REQ' <<<"$validate_out"; then
    fail "ac5/$runtime/independence-b-enumeration-with-violation-active" "bob's roadmap was not scanned in the SAME output where the violation fired — union may have become gated by the violation (output: $(printf '%q' "$validate_out"))"
  fi
  ok "ac5/$runtime/independence-b-enumeration-with-violation-active"
done

P1_DECLARED="$WORK/p1-bob-declared"
scaffold_by_agent "$P1_DECLARED" $'- alice\n- bob'
cp -r "$P1/docs/roadmaps/alice" "$P1_DECLARED/docs/roadmaps/alice"
cp -r "$P1/docs/roadmaps/bob" "$P1_DECLARED/docs/roadmaps/bob"
cp -r "$P1/docs/req/bob" "$P1_DECLARED/docs/req/bob"

for runtime in go node python; do
  validate_out=$(run_"$runtime" "$P1_DECLARED" validate; true)
  if grep -qF "$MSG_BOB_UNDECLARED" <<<"$validate_out"; then
    fail "ac5/$runtime/declaring-silences-violation" "violation still present after declaring bob — output: $(printf '%q' "$validate_out")"
  fi
  status_out=$(run_"$runtime" "$P1_DECLARED" status; true)
  if ! grep -qF "ROADMAP-bob-wip.md" <<<"$status_out"; then
    fail "ac5/$runtime/declaring-keeps-enumeration" "bob's roadmap disappeared after declaring it — union broke on declaration"
  fi
  ok "ac5/$runtime/declaring-silences-violation-keeps-enumeration"
done

# ===========================================================================
# Infra filter — .git / node_modules / orphan state-name dir never trigger
# the violation.
# ===========================================================================
P2="$WORK/p2-infra"
scaffold_by_agent "$P2" "- alice"
mkdir -p "$P2/docs/roadmaps/alice/wip" "$P2/docs/roadmaps/.git" "$P2/docs/roadmaps/node_modules" "$P2/docs/roadmaps/wip"
write_wip_roadmap "$P2/docs/roadmaps/alice/wip/ROADMAP-alice-wip.md" "alice wip"

for runtime in go node python; do
  validate_out=$(run_"$runtime" "$P2" validate; true)
  if ! grep -qF 'roadmap "ROADMAP-alice-wip.md" is in wip but has no linked REQ' <<<"$validate_out"; then
    fail "infra-filter/$runtime/liveness-anchor" "validate produced no usable output at all (alice's expected wip_has_req violation is absent too) — cannot distinguish 'infra correctly filtered' from 'validate crashed/empty output'; output: $(printf '%q' "$validate_out")"
  fi
  if grep -qiF 'agent namespace ".git"' <<<"$validate_out"; then
    fail "infra-filter/$runtime/dotgit" ".git accused as undeclared namespace — output: $(printf '%q' "$validate_out")"
  fi
  if grep -qF 'agent namespace "node_modules"' <<<"$validate_out"; then
    fail "infra-filter/$runtime/node_modules" "node_modules accused as undeclared namespace — output: $(printf '%q' "$validate_out")"
  fi
  if grep -qF 'agent namespace "wip"' <<<"$validate_out"; then
    fail "infra-filter/$runtime/orphan-wip" "orphan wip/ accused as undeclared namespace — output: $(printf '%q' "$validate_out")"
  fi
  ok "infra-filter/$runtime/dotgit-node_modules-orphan-wip-silent"
done

# ===========================================================================
# flat untouched — `roadmap_namespacing: flat` never emits
# `agent_namespace_undeclared`.
# ===========================================================================
P3="$WORK/p3-flat"
mkdir -p "$P3/docs/adr" "$P3/docs/req" "$P3/docs/roadmaps"/{backlog,analyzing,wip,blocked,done,abandoned}
cat > "$P3/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
EOF
write_wip_roadmap "$P3/docs/roadmaps/wip/ROADMAP-flat-wip.md" "flat wip"

for runtime in go node python; do
  validate_out=$(run_"$runtime" "$P3" validate; true)
  if ! grep -qF 'roadmap "ROADMAP-flat-wip.md" is in wip but has no linked REQ' <<<"$validate_out"; then
    fail "flat-untouched/$runtime/liveness-anchor" "validate produced no usable output at all (the flat project's expected wip_has_req violation is absent too) — cannot distinguish 'flat correctly untouched' from 'validate crashed/empty output'; output: $(printf '%q' "$validate_out")"
  fi
  if grep -qF "agent namespace" <<<"$validate_out"; then
    fail "flat-untouched/$runtime" "flat project emitted an agent-namespace violation — output: $(printf '%q' "$validate_out")"
  fi
  ok "flat-untouched/$runtime"
done

# ===========================================================================
# AC12 — symlink under roadmap_dir pointing OUTSIDE the project: `roadmap
# move` must not write outside the project, and must not report success on
# a file it never touched.
# ===========================================================================
P4_OUT="$WORK/p4-symlink-out"
mkdir -p "$P4_OUT/wip"
write_wip_roadmap "$P4_OUT/wip/ROADMAP-leak.md" "leak"

for runtime in go node python; do
  P4="$WORK/p4-symlink-$runtime"
  scaffold_by_agent "$P4" "- alice"
  mkdir -p "$P4/docs/roadmaps/alice/wip"
  ln -s "$P4_OUT" "$P4/docs/roadmaps/evil"

  set +e
  move_out=$(run_"$runtime" "$P4" roadmap move ROADMAP-leak done)
  move_status=$?
  set -e

  if [[ -e "$P4_OUT/done" ]]; then
    fail "ac12/$runtime/no-symlink-escape" "roadmap move wrote through the symlink — $P4_OUT/done now exists (output: $(printf '%q' "$move_out"), exit=$move_status)"
  fi
  if [[ ! -f "$P4_OUT/wip/ROADMAP-leak.md" ]]; then
    fail "ac12/$runtime/leak-fixture-intact" "the leak fixture itself disappeared without a done/ appearing — move-without-create bug in this gate's own setup, not the CLI"
  fi
  if [[ $move_status -eq 0 ]]; then
    fail "ac12/$runtime/no-false-success" "roadmap move reported exit 0 for a file it never actually relocated — output: $(printf '%q' "$move_out")"
  fi
  ok "ac12/$runtime/no-symlink-escape"
done

echo "--- positive scenarios: $SCENARIOS passed. Starting falsification (both directions). ---"

# ===========================================================================
# Direction A — union reverts to substitution: with `agents:` non-empty, an
# undeclared-but-on-disk namespace must vanish entirely (no violation either,
# because it was never scanned at all — this is the exact pre-REQ-2026-08-29
# defect: `agents:` REPLACES the disk instead of complementing it).
# ===========================================================================

# --- Go ---
TA_GO_MOD="$WORK/dira-go-mod"
mkdir -p "$TA_GO_MOD/cmd" "$TA_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$TA_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$TA_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$TA_GO_MOD/"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$TA_GO_MOD/internal/validator/validator.go" \
  $'\tentries, err := os.ReadDir(dir)\n\tif err != nil {\n\t\treturn ordered\n\t}\n' \
  $'\tif len(ordered) > 0 {\n\t\treturn ordered\n\t}\n\tentries, err := os.ReadDir(dir)\n\tif err != nil {\n\t\treturn ordered\n\t}\n' \
  "direction-a-go"
TA_GO_BIN="$WORK/dira-go-bin/trackfw"
mkdir -p "$(dirname "$TA_GO_BIN")"
build_go_or_fail "direction-a/go/build" "$TA_GO_MOD" "$TA_GO_BIN"

dira_go_status=$(cd "$P1" && "$TA_GO_BIN" status 2>&1; true)
if ! grep -qF "ROADMAP-alice-wip.md" <<<"$dira_go_status"; then
  fail "direction-a/go/detects-substitution-regression" "the corrupted binary produced no usable output at all (alice, the DECLARED namespace, is absent too) — cannot distinguish 'bob correctly vanished' from 'the binary crashed/empty output'; output: $(printf '%q' "$dira_go_status")"
fi
if grep -qF "ROADMAP-bob-wip.md" <<<"$dira_go_status"; then
  fail "direction-a/go/detects-substitution-regression" "corrupted binary still shows bob — checagem vácua"
fi
ok "direction-a/go/detects-substitution-regression"

# --- Node ---
TA_N="$WORK/dira-node"
setup_npm_tree "$TA_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$TA_N/npm/src/validator/index.js" \
  $'  let entries = []\n  try {\n    entries = fs.readdirSync(dir, { withFileTypes: true })\n  } catch (_) {\n    return ordered\n  }\n' \
  $'  if (ordered.length) return ordered\n  let entries = []\n  try {\n    entries = fs.readdirSync(dir, { withFileTypes: true })\n  } catch (_) {\n    return ordered\n  }\n' \
  "direction-a-node"

dira_node_status=$(cd "$P1" && node "$TA_N/npm/bin/trackfw" status 2>&1; true)
if ! grep -qF "ROADMAP-alice-wip.md" <<<"$dira_node_status"; then
  fail "direction-a/node/detects-substitution-regression" "the corrupted tree produced no usable output at all (alice, the DECLARED namespace, is absent too) — cannot distinguish 'bob correctly vanished' from 'node crashed/empty output'; output: $(printf '%q' "$dira_node_status")"
fi
if grep -qF "ROADMAP-bob-wip.md" <<<"$dira_node_status"; then
  fail "direction-a/node/detects-substitution-regression" "corrupted binary still shows bob — checagem vácua"
fi
ok "direction-a/node/detects-substitution-regression"

# --- Python ---
TA_P="$WORK/dira-python"
setup_py_tree "$TA_P"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$TA_P/pypi/trackfw/config.py" \
  $'    try:\n        with os.scandir(directory) as it:\n' \
  $'    if ordered:\n        return ordered\n    try:\n        with os.scandir(directory) as it:\n' \
  "direction-a-python"

dira_python_status=$(cd "$P1" && env PYTHONPATH="$TA_P/pypi" python3 -m trackfw status 2>&1; true)
if ! grep -qF "ROADMAP-alice-wip.md" <<<"$dira_python_status"; then
  fail "direction-a/python/detects-substitution-regression" "the corrupted tree produced no usable output at all (alice, the DECLARED namespace, is absent too) — cannot distinguish 'bob correctly vanished' from 'python crashed/empty output'; output: $(printf '%q' "$dira_python_status")"
fi
if grep -qF "ROADMAP-bob-wip.md" <<<"$dira_python_status"; then
  fail "direction-a/python/detects-substitution-regression" "corrupted binary still shows bob — checagem vácua"
fi
ok "direction-a/python/detects-substitution-regression"

# ===========================================================================
# Direction B1 — infra filter disabled: `.git`/`node_modules` start being
# accused as undeclared namespaces.
# ===========================================================================

# --- Go ---
TB1_GO_MOD="$WORK/dirb1-go-mod"
mkdir -p "$TB1_GO_MOD/cmd" "$TB1_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$TB1_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$TB1_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$TB1_GO_MOD/"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$TB1_GO_MOD/internal/validator/validator.go" \
  'return strings.HasPrefix(name, ".") || name == "node_modules"' \
  'return false' \
  "direction-b1-go"
TB1_GO_BIN="$WORK/dirb1-go-bin/trackfw"
mkdir -p "$(dirname "$TB1_GO_BIN")"
build_go_or_fail "direction-b1/go/build" "$TB1_GO_MOD" "$TB1_GO_BIN"

dirb1_go_out=$(cd "$P2" && "$TB1_GO_BIN" validate 2>&1; true)
if ! grep -qF 'agent namespace ".git"' <<<"$dirb1_go_out"; then
  fail "direction-b1/go/detects-infra-filter-regression" "corrupted binary did not accuse .git — checagem vácua"
fi
ok "direction-b1/go/detects-infra-filter-regression"

# --- Node ---
TB1_N="$WORK/dirb1-node"
setup_npm_tree "$TB1_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$TB1_N/npm/src/validator/index.js" \
  "return name.startsWith('.') || name === 'node_modules'" \
  "return false" \
  "direction-b1-node"

dirb1_node_out=$(cd "$P2" && node "$TB1_N/npm/bin/trackfw" validate 2>&1; true)
if ! grep -qF 'agent namespace ".git"' <<<"$dirb1_node_out"; then
  fail "direction-b1/node/detects-infra-filter-regression" "corrupted binary did not accuse .git — checagem vácua"
fi
ok "direction-b1/node/detects-infra-filter-regression"

# --- Python ---
TB1_P="$WORK/dirb1-python"
setup_py_tree "$TB1_P"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$TB1_P/pypi/trackfw/config.py" \
  'return name.startswith(".") or name == "node_modules"' \
  'return False' \
  "direction-b1-python"

dirb1_python_out=$(cd "$P2" && env PYTHONPATH="$TB1_P/pypi" python3 -m trackfw validate 2>&1; true)
if ! grep -qF 'agent namespace ".git"' <<<"$dirb1_python_out"; then
  fail "direction-b1/python/detects-infra-filter-regression" "corrupted binary did not accuse .git — checagem vácua"
fi
ok "direction-b1/python/detects-infra-filter-regression"

# ===========================================================================
# Direction B2 — AC12 regression: disk scan follows symlinks again
# (reproduced live in ML-0A for Node/Python; Go excluded, see header comment).
# ===========================================================================

P4_LEAK_OUT="$WORK/dirb2-leak-out"
mkdir -p "$P4_LEAK_OUT/wip"
write_wip_roadmap "$P4_LEAK_OUT/wip/ROADMAP-leak.md" "leak"

# --- Node ---
TB2_N="$WORK/dirb2-node"
setup_npm_tree "$TB2_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$TB2_N/npm/src/validator/index.js" \
  '.filter(e => e.isDirectory()) // symlinks retornam false aqui — nunca seguidos (AC12/AC13)' \
  '.filter(e => fs.statSync(path.join(dir, e.name)).isDirectory()) // CORRUPTED (direction-b2): segue symlink' \
  "direction-b2-node"

P4_N="$WORK/dirb2-node-project"
scaffold_by_agent "$P4_N" "- alice"
mkdir -p "$P4_N/docs/roadmaps/alice/wip"
ln -s "$P4_LEAK_OUT" "$P4_N/docs/roadmaps/evil"

set +e
dirb2_node_out=$(cd "$P4_N" && node "$TB2_N/npm/bin/trackfw" roadmap move ROADMAP-leak done 2>&1)
dirb2_node_status=$?
set -e
if [[ ! -f "$P4_LEAK_OUT/done/ROADMAP-leak.md" ]]; then
  fail "direction-b2/node/detects-symlink-regression" "corrupted binary did not escape through the symlink (exit=$dirb2_node_status, output: $(printf '%q' "$dirb2_node_out")) — checagem vácua"
fi
ok "direction-b2/node/detects-symlink-regression"

# --- Python ---
P4_LEAK_OUT_PY="$WORK/dirb2-leak-out-python"
mkdir -p "$P4_LEAK_OUT_PY/wip"
write_wip_roadmap "$P4_LEAK_OUT_PY/wip/ROADMAP-leak.md" "leak"

TB2_P="$WORK/dirb2-python"
setup_py_tree "$TB2_P"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$TB2_P/pypi/trackfw/config.py" \
  $'                if e.is_dir(follow_symlinks=False)  # symlinks retornam False — nunca seguidos\n' \
  $'                if os.path.isdir(os.path.join(directory, e.name))  # CORRUPTED (direction-b2): segue symlink\n' \
  "direction-b2-python"

P4_P="$WORK/dirb2-python-project"
scaffold_by_agent "$P4_P" "- alice"
mkdir -p "$P4_P/docs/roadmaps/alice/wip"
ln -s "$P4_LEAK_OUT_PY" "$P4_P/docs/roadmaps/evil"

set +e
dirb2_python_out=$(cd "$P4_P" && env PYTHONPATH="$TB2_P/pypi" python3 -m trackfw roadmap move ROADMAP-leak done 2>&1)
dirb2_python_status=$?
set -e
if [[ ! -f "$P4_LEAK_OUT_PY/done/ROADMAP-leak.md" ]]; then
  fail "direction-b2/python/detects-symlink-regression" "corrupted binary did not escape through the symlink (exit=$dirb2_python_status, output: $(printf '%q' "$dirb2_python_out")) — checagem vácua"
fi
ok "direction-b2/python/detects-symlink-regression"

echo "check-agent-namespace-union: all $SCENARIOS scenarios passed (AC1 x3 runtimes x3 checks, AC4 x3, AC5 x3+x3, infra-filter x3, flat-untouched x3, AC12 x3, direction-a x3, direction-b1 x3, direction-b2 x2)."
