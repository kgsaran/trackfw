#!/usr/bin/env bash
# check-roadmap-move-parity.sh — proves `trackfw roadmap move` implements REQ sync
# identically in Go, Node.js, and Python, covering the five cardinalities pinned in
# docs/cli-parity.md (§"roadmap move synchronizes the paired REQ reference"):
#   1. zero REQs pointing → no-op, only `✓ moved` line, no `✓ synced`
#   2. one REQ            → one `✓ synced` line
#   3. several REQs       → all synced, lexicographic order by basename
#                           (discriminant by_agent fixture: apolo/REQ-zzz + zeus/REQ-aaa → aaa, zzz)
#   4. REQ points at a different roadmap → not touched, only `✓ moved`
#   5. reference already correct → no write, only `✓ moved` (byte-level idempotency)
#
# Also verifies `✓ moved` line is byte-identical across all three runtimes in both
# flat and by_agent layouts.
#
# Follows the conventions of scripts/check-barrier.sh and scripts/check-update-parity.sh:
#   set -euo pipefail, mktemp -d fixtures with a cleanup trap, BASH_SOURCE-relative ROOT_DIR,
#   "OK [scenario/name]" on success, accumulate failures before exiting.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-roadmap-move-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-barrier.sh pattern:
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
  echo "check-roadmap-move-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-roadmap-move-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# run_move RUNTIME DIR NAME STATE
# Runs `trackfw roadmap move <name> <state>` from DIR.
# Sets MOVE_EXIT, MOVE_STDOUT, MOVE_STDERR as globals.
run_move() {
  local runtime=$1 dir=$2 name=$3 state=$4
  local out_file="$WORK/out.$$.$RANDOM" err_file="$WORK/err.$$.$RANDOM"
  set +e
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" roadmap move "$name" "$state")                              >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && node "$NODE_CLI" roadmap move "$name" "$state")                        >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw roadmap move "$name" "$state") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_move: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  MOVE_EXIT=$?
  set -e
  MOVE_STDOUT=$(cat "$out_file")
  MOVE_STDERR=$(cat "$err_file")
  rm -f "$out_file" "$err_file"
}

# ---------------------------------------------------------------------------
# Fixture scaffolding helpers
# ---------------------------------------------------------------------------

# make_flat_base DIR — creates the minimal flat-mode tree; caller adds files.
make_flat_base() {
  local dir=$1
  mkdir -p "$dir/docs/roadmaps/backlog" "$dir/docs/roadmaps/wip" \
            "$dir/docs/roadmaps/done"   "$dir/docs/req"          "$dir/docs/adr"
  cat >"$dir/trackfw.yaml" <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dir: docs/adr
EOF
}

# make_byagent_base DIR — creates the minimal by_agent tree; caller adds files.
make_byagent_base() {
  local dir=$1
  mkdir -p "$dir/docs/roadmaps/zeus/backlog" "$dir/docs/roadmaps/zeus/wip" \
            "$dir/docs/roadmaps/zeus/done" \
            "$dir/docs/req/apolo/done"     "$dir/docs/req/zeus/backlog" "$dir/docs/adr"
  cat >"$dir/trackfw.yaml" <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents: [zeus, apolo]
adr_dir: docs/adr
EOF
}

# write_roadmap DIR STATE BASENAME — writes a minimal roadmap frontmatter.
write_roadmap() {
  local dir=$1 state=$2 basename=$3
  mkdir -p "$dir/docs/roadmaps/$state"
  cat >"$dir/docs/roadmaps/$state/$basename" <<EOF
---
status: $state
date: 2026-01-01
req: ""
---
# Roadmap: parity fixture
EOF
}

# write_req_flat DIR BASENAME ROADMAP_PATH — writes a REQ in flat req_dir.
write_req_flat() {
  local dir=$1 basename=$2 roadmap_path=$3
  cat >"$dir/docs/req/$basename" <<EOF
---
status: backlog
roadmap: $roadmap_path
---
# $basename
Roadmap: \`$roadmap_path\`
EOF
}

# write_req_agent DIR AGENT STATE BASENAME ROADMAP_PATH — writes a REQ in by_agent tree.
write_req_agent() {
  local dir=$1 agent=$2 state=$3 basename=$4 roadmap_path=$5
  mkdir -p "$dir/docs/req/$agent/$state"
  cat >"$dir/docs/req/$agent/$state/$basename" <<EOF
---
status: $state
roadmap: $roadmap_path
---
# $basename
Roadmap: \`$roadmap_path\`
EOF
}

# count_synced_lines STDOUT — prints the number of `✓ synced` lines in STDOUT.
count_synced_lines() {
  echo "$1" | grep -cF '✓ synced' || true
}

# ---------------------------------------------------------------------------
# Scenario 1 — flat/zero-req: no REQs → only `✓ moved`, no `✓ synced`.
# Vacuity guard: stdout must contain `✓ moved` (proves the runtime ran the move).
# ---------------------------------------------------------------------------
S1BASE="$WORK/s1-base"
make_flat_base "$S1BASE"
write_roadmap "$S1BASE" "backlog" "ROADMAP-2026-01-01-probe.md"

for runtime in go node py; do
  fixture="$WORK/s1-$runtime"
  cp -r "$S1BASE" "$fixture"
  run_move "$runtime" "$fixture" "ROADMAP-2026-01-01-probe" "wip"
  if [[ "$MOVE_EXIT" -ne 0 ]]; then
    fail "roadmap-move-parity/zero-req/$runtime" "expected exit 0, got $MOVE_EXIT; stderr: $MOVE_STDERR"
    continue
  fi
  # Vacuity guard
  if ! grep -qF '✓ moved' <<<"$MOVE_STDOUT"; then
    fail "roadmap-move-parity/zero-req/$runtime" "vacuity guard: stdout missing '✓ moved'; stdout: [$MOVE_STDOUT]"
    continue
  fi
  # Contract: no ✓ synced when zero REQs
  n_synced=$(count_synced_lines "$MOVE_STDOUT")
  if [[ "$n_synced" -ne 0 ]]; then
    fail "roadmap-move-parity/zero-req/$runtime" "expected 0 synced lines, got $n_synced; stdout: $MOVE_STDOUT"
    continue
  fi
  echo "$MOVE_STDOUT" >"$WORK/s1-stdout-$runtime.txt"
done
# Cross-runtime byte comparison
if [[ -f "$WORK/s1-stdout-go.txt" && -f "$WORK/s1-stdout-node.txt" ]]; then
  if ! diff -u "$WORK/s1-stdout-go.txt" "$WORK/s1-stdout-node.txt" >"$WORK/s1-diff-go-node.txt" 2>&1; then
    fail "roadmap-move-parity/zero-req/go-vs-node" "stdout diverges between Go and Node.js:
$(cat "$WORK/s1-diff-go-node.txt")"
  fi
fi
if [[ -f "$WORK/s1-stdout-go.txt" && -f "$WORK/s1-stdout-py.txt" ]]; then
  if ! diff -u "$WORK/s1-stdout-go.txt" "$WORK/s1-stdout-py.txt" >"$WORK/s1-diff-go-py.txt" 2>&1; then
    fail "roadmap-move-parity/zero-req/go-vs-py" "stdout diverges between Go and Python:
$(cat "$WORK/s1-diff-go-py.txt")"
  fi
fi
ok "roadmap-move-parity/zero-req"

# ---------------------------------------------------------------------------
# Scenario 2 — flat/one-req: one REQ points at the moved roadmap → one `✓ synced`.
# Vacuity guard: assert exactly one synced line exists.
# ---------------------------------------------------------------------------
S2BASE="$WORK/s2-base"
make_flat_base "$S2BASE"
write_roadmap  "$S2BASE" "backlog" "ROADMAP-2026-01-01-probe.md"
write_req_flat "$S2BASE" "REQ-one.md" "docs/roadmaps/backlog/ROADMAP-2026-01-01-probe.md"

for runtime in go node py; do
  fixture="$WORK/s2-$runtime"
  cp -r "$S2BASE" "$fixture"
  run_move "$runtime" "$fixture" "ROADMAP-2026-01-01-probe" "wip"
  if [[ "$MOVE_EXIT" -ne 0 ]]; then
    fail "roadmap-move-parity/one-req/$runtime" "expected exit 0, got $MOVE_EXIT; stderr: $MOVE_STDERR"
    continue
  fi
  # Vacuity guard: exactly one synced line
  n_synced=$(count_synced_lines "$MOVE_STDOUT")
  if [[ "$n_synced" -ne 1 ]]; then
    fail "roadmap-move-parity/one-req/$runtime" "vacuity guard: expected exactly 1 synced line, got $n_synced; stdout: $MOVE_STDOUT"
    continue
  fi
  # Contract: the synced line names the REQ
  if ! grep -qF '✓ synced REQ-one.md' <<<"$MOVE_STDOUT"; then
    fail "roadmap-move-parity/one-req/$runtime" "synced line missing REQ-one.md; stdout: $MOVE_STDOUT"
    continue
  fi
  echo "$MOVE_STDOUT" >"$WORK/s2-stdout-$runtime.txt"
done
# Cross-runtime byte comparison
if [[ -f "$WORK/s2-stdout-go.txt" && -f "$WORK/s2-stdout-node.txt" ]]; then
  if ! diff -u "$WORK/s2-stdout-go.txt" "$WORK/s2-stdout-node.txt" >"$WORK/s2-diff-go-node.txt" 2>&1; then
    fail "roadmap-move-parity/one-req/go-vs-node" "stdout diverges:
$(cat "$WORK/s2-diff-go-node.txt")"
  fi
fi
if [[ -f "$WORK/s2-stdout-go.txt" && -f "$WORK/s2-stdout-py.txt" ]]; then
  if ! diff -u "$WORK/s2-stdout-go.txt" "$WORK/s2-stdout-py.txt" >"$WORK/s2-diff-go-py.txt" 2>&1; then
    fail "roadmap-move-parity/one-req/go-vs-py" "stdout diverges:
$(cat "$WORK/s2-diff-go-py.txt")"
  fi
fi
ok "roadmap-move-parity/one-req"

# ---------------------------------------------------------------------------
# Scenario 3 — by_agent/discriminant-several: two REQs in by_agent layout, with
# discriminant naming (apolo/REQ-zzz + zeus/REQ-aaa). Path-based sort gives
# zzz,aaa (WRONG); basename sort gives aaa,zzz (CORRECT per contract).
#
# This is the critical fixture: a sort-by-path implementation would agree with
# sort-by-basename on the coincident fixture (apolo/REQ-aaa + zeus/REQ-zzz) but
# diverges here because basename order inverts the agent-path order.
#
# Vacuity guard: assert exactly two synced lines per runtime.
# Sequence assertion: line 0 = REQ-aaa, line 1 = REQ-zzz (positional, not set).
# ---------------------------------------------------------------------------
S3BASE="$WORK/s3-base"
make_byagent_base "$S3BASE"
write_roadmap      "$S3BASE" "zeus/backlog" "ROADMAP-2026-01-01-byagent.md"
write_req_agent    "$S3BASE" "apolo" "done"    "REQ-zzz.md" \
                   "docs/roadmaps/zeus/backlog/ROADMAP-2026-01-01-byagent.md"
write_req_agent    "$S3BASE" "zeus"  "backlog" "REQ-aaa.md" \
                   "docs/roadmaps/zeus/backlog/ROADMAP-2026-01-01-byagent.md"

for runtime in go node py; do
  fixture="$WORK/s3-$runtime"
  cp -r "$S3BASE" "$fixture"
  run_move "$runtime" "$fixture" "ROADMAP-2026-01-01-byagent" "wip"
  if [[ "$MOVE_EXIT" -ne 0 ]]; then
    fail "roadmap-move-parity/by_agent-discriminant/$runtime" "expected exit 0, got $MOVE_EXIT; stderr: $MOVE_STDERR"
    continue
  fi
  # Vacuity guard: exactly two synced lines
  n_synced=$(count_synced_lines "$MOVE_STDOUT")
  if [[ "$n_synced" -ne 2 ]]; then
    fail "roadmap-move-parity/by_agent-discriminant/$runtime" "vacuity guard: expected 2 synced lines, got $n_synced; stdout: $MOVE_STDOUT"
    continue
  fi
  # Sequence assertion (positional, not set membership)
  mapfile -t synced_lines < <(echo "$MOVE_STDOUT" | grep '✓ synced')
  line0="${synced_lines[0]:-}"
  line1="${synced_lines[1]:-}"
  if [[ "$line0" != *"REQ-aaa.md"* ]]; then
    fail "roadmap-move-parity/by_agent-discriminant/$runtime" "line 0 must be REQ-aaa.md (basename order); got: [$line0]; full stdout: $MOVE_STDOUT"
    continue
  fi
  if [[ "$line1" != *"REQ-zzz.md"* ]]; then
    fail "roadmap-move-parity/by_agent-discriminant/$runtime" "line 1 must be REQ-zzz.md (basename order); got: [$line1]; full stdout: $MOVE_STDOUT"
    continue
  fi
  echo "$MOVE_STDOUT" >"$WORK/s3-stdout-$runtime.txt"
done
# Cross-runtime byte comparison (this is what the falsify seam exercises)
if [[ -f "$WORK/s3-stdout-go.txt" && -f "$WORK/s3-stdout-node.txt" ]]; then
  if ! diff -u "$WORK/s3-stdout-go.txt" "$WORK/s3-stdout-node.txt" >"$WORK/s3-diff-go-node.txt" 2>&1; then
    fail "roadmap-move-parity/by_agent-discriminant/go-vs-node" "stdout diverges between Go and Node.js runtimes (ordering regression?):
$(cat "$WORK/s3-diff-go-node.txt")"
  fi
fi
if [[ -f "$WORK/s3-stdout-go.txt" && -f "$WORK/s3-stdout-py.txt" ]]; then
  if ! diff -u "$WORK/s3-stdout-go.txt" "$WORK/s3-stdout-py.txt" >"$WORK/s3-diff-go-py.txt" 2>&1; then
    fail "roadmap-move-parity/by_agent-discriminant/go-vs-py" "stdout diverges between Go and Python:
$(cat "$WORK/s3-diff-go-py.txt")"
  fi
fi
ok "roadmap-move-parity/by_agent-discriminant"

# ---------------------------------------------------------------------------
# Scenario 4 — flat/points-at-other: REQ points at a DIFFERENT roadmap → not touched.
# Vacuity guard: stdout must contain `✓ moved` (runtime ran); zero synced lines.
# ---------------------------------------------------------------------------
S4BASE="$WORK/s4-base"
make_flat_base "$S4BASE"
write_roadmap  "$S4BASE" "backlog" "ROADMAP-2026-01-01-target.md"
write_roadmap  "$S4BASE" "backlog" "ROADMAP-2026-01-01-other.md"
# REQ points at OTHER, not target
write_req_flat "$S4BASE" "REQ-other.md" "docs/roadmaps/backlog/ROADMAP-2026-01-01-other.md"

for runtime in go node py; do
  fixture="$WORK/s4-$runtime"
  cp -r "$S4BASE" "$fixture"
  run_move "$runtime" "$fixture" "ROADMAP-2026-01-01-target" "wip"
  if [[ "$MOVE_EXIT" -ne 0 ]]; then
    fail "roadmap-move-parity/points-at-other/$runtime" "expected exit 0, got $MOVE_EXIT; stderr: $MOVE_STDERR"
    continue
  fi
  # Vacuity guard
  if ! grep -qF '✓ moved' <<<"$MOVE_STDOUT"; then
    fail "roadmap-move-parity/points-at-other/$runtime" "vacuity guard: stdout missing '✓ moved'; stdout: [$MOVE_STDOUT]"
    continue
  fi
  # Contract: REQ pointing at other roadmap must not be touched
  n_synced=$(count_synced_lines "$MOVE_STDOUT")
  if [[ "$n_synced" -ne 0 ]]; then
    fail "roadmap-move-parity/points-at-other/$runtime" "expected 0 synced lines (REQ points at other), got $n_synced; stdout: $MOVE_STDOUT"
    continue
  fi
  # REQ content must be unchanged (still points at other)
  req_content=$(cat "$fixture/docs/req/REQ-other.md")
  if ! grep -qF "ROADMAP-2026-01-01-other.md" <<<"$req_content"; then
    fail "roadmap-move-parity/points-at-other/$runtime" "REQ was modified even though it points at a different roadmap"
    continue
  fi
  echo "$MOVE_STDOUT" >"$WORK/s4-stdout-$runtime.txt"
done
# Cross-runtime byte comparison
if [[ -f "$WORK/s4-stdout-go.txt" && -f "$WORK/s4-stdout-node.txt" ]]; then
  if ! diff -u "$WORK/s4-stdout-go.txt" "$WORK/s4-stdout-node.txt" >"$WORK/s4-diff-go-node.txt" 2>&1; then
    fail "roadmap-move-parity/points-at-other/go-vs-node" "stdout diverges:
$(cat "$WORK/s4-diff-go-node.txt")"
  fi
fi
if [[ -f "$WORK/s4-stdout-go.txt" && -f "$WORK/s4-stdout-py.txt" ]]; then
  if ! diff -u "$WORK/s4-stdout-go.txt" "$WORK/s4-stdout-py.txt" >"$WORK/s4-diff-go-py.txt" 2>&1; then
    fail "roadmap-move-parity/points-at-other/go-vs-py" "stdout diverges:
$(cat "$WORK/s4-diff-go-py.txt")"
  fi
fi
ok "roadmap-move-parity/points-at-other"

# ---------------------------------------------------------------------------
# Scenario 5 — flat/idempotency (already-correct): REQ already points at the
# target path → no write, no `✓ synced` on the second move; bytes unchanged.
#
# Construction:
#   Move 1: backlog → wip  (REQ updated to wip path; one synced line)
#   Move 2: wip → wip      (REQ already correct; zero synced lines; bytes stable)
# ---------------------------------------------------------------------------
S5BASE="$WORK/s5-base"
make_flat_base "$S5BASE"
write_roadmap  "$S5BASE" "backlog" "ROADMAP-2026-01-01-probe.md"
write_req_flat "$S5BASE" "REQ-idempotency.md" "docs/roadmaps/backlog/ROADMAP-2026-01-01-probe.md"

for runtime in go node py; do
  fixture="$WORK/s5-$runtime"
  cp -r "$S5BASE" "$fixture"

  # Move 1: backlog → wip (REQ gets updated)
  run_move "$runtime" "$fixture" "ROADMAP-2026-01-01-probe" "wip"
  if [[ "$MOVE_EXIT" -ne 0 ]]; then
    fail "roadmap-move-parity/idempotency/$runtime/move1" "expected exit 0, got $MOVE_EXIT; stderr: $MOVE_STDERR"
    continue
  fi
  n_synced_1=$(count_synced_lines "$MOVE_STDOUT")
  if [[ "$n_synced_1" -ne 1 ]]; then
    fail "roadmap-move-parity/idempotency/$runtime/move1" "expected 1 synced line on first move, got $n_synced_1; stdout: $MOVE_STDOUT"
    continue
  fi
  # Capture REQ bytes after first move
  req_bytes_after_first=$(cat "$fixture/docs/req/REQ-idempotency.md")

  # Move 2: wip → wip (same state; REQ already correct → no write)
  run_move "$runtime" "$fixture" "ROADMAP-2026-01-01-probe" "wip"
  if [[ "$MOVE_EXIT" -ne 0 ]]; then
    fail "roadmap-move-parity/idempotency/$runtime/move2" "expected exit 0 on second move, got $MOVE_EXIT; stderr: $MOVE_STDERR"
    continue
  fi
  # Vacuity guard: `✓ moved` must appear on second move too
  if ! grep -qF '✓ moved' <<<"$MOVE_STDOUT"; then
    fail "roadmap-move-parity/idempotency/$runtime/move2" "vacuity guard: second move stdout missing '✓ moved'; stdout: [$MOVE_STDOUT]"
    continue
  fi
  # Contract: zero synced lines on second move (already correct)
  n_synced_2=$(count_synced_lines "$MOVE_STDOUT")
  if [[ "$n_synced_2" -ne 0 ]]; then
    fail "roadmap-move-parity/idempotency/$runtime/move2" "expected 0 synced lines on second move (already correct), got $n_synced_2; stdout: $MOVE_STDOUT"
    continue
  fi
  # Byte-level idempotency: REQ content must be identical after both moves
  req_bytes_after_second=$(cat "$fixture/docs/req/REQ-idempotency.md")
  if [[ "$req_bytes_after_first" != "$req_bytes_after_second" ]]; then
    fail "roadmap-move-parity/idempotency/$runtime/bytes" "REQ bytes changed on second move — idempotency violated"
    continue
  fi
  echo "$MOVE_STDOUT" >"$WORK/s5-stdout-$runtime.txt"
done
# Cross-runtime byte comparison for second move stdout
if [[ -f "$WORK/s5-stdout-go.txt" && -f "$WORK/s5-stdout-node.txt" ]]; then
  if ! diff -u "$WORK/s5-stdout-go.txt" "$WORK/s5-stdout-node.txt" >"$WORK/s5-diff-go-node.txt" 2>&1; then
    fail "roadmap-move-parity/idempotency/go-vs-node" "stdout diverges on second move:
$(cat "$WORK/s5-diff-go-node.txt")"
  fi
fi
if [[ -f "$WORK/s5-stdout-go.txt" && -f "$WORK/s5-stdout-py.txt" ]]; then
  if ! diff -u "$WORK/s5-stdout-go.txt" "$WORK/s5-stdout-py.txt" >"$WORK/s5-diff-go-py.txt" 2>&1; then
    fail "roadmap-move-parity/idempotency/go-vs-py" "stdout diverges on second move:
$(cat "$WORK/s5-diff-go-py.txt")"
  fi
fi
ok "roadmap-move-parity/idempotency"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-roadmap-move-parity.sh scenarios passed."
else
  echo "check-roadmap-move-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
