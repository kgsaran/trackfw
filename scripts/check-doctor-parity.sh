#!/usr/bin/env bash
# check-doctor-parity.sh — proves `trackfw doctor` behaves byte-for-byte identically in Go,
# Node.js, and Python, across both surfaces (text report and --json), for the two finding
# classes ML-2A introduced (unregistered-write, hand-modified) plus three silent paths (clean
# baseline, unmanaged alien content at a real catalog destination, and a destination registered
# under a DIFFERENT claim — the near-miss false positive the ML-2A audit trail flagged) — see
# internal/integrations/doctor.go, docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-
# ausente-do-manifesto-apos-janela-de-gravacao-parcial.md and
# ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md.
#
# doctor is READ-ONLY (it never writes — every finding just prints a remedy command), so unlike
# check-branch-prune-parity.sh's --apply arm, a single fixture project/home pair can be built
# ONCE per scenario and then inspected by all three runtimes in turn without cross-contamination.
#
# Fixture hard constraints (each already cost a cycle in this series — see the roadmap's
# ML-2B section):
#   1. $HOME is redirected to a per-scenario temp dir for EVERY invocation (fixture build AND
#      doctor run). doctor sweeps the GLOBAL scope in addition to project scope; without this the
#      gate would read the real ~/.trackfw of whoever runs it and the result would depend on the
#      machine, not the fixture.
#   2. Both mismatch states are built by installing for real (`agents install`) and then
#      mutating the result — never by hand-crafting manifest/artifact bytes from a hardcoded
#      template. A hardcoded template rots silently the next time the catalog template changes.
#   3. Identity is fixed explicitly (a real identity.json written into $HOME before install) —
#      identity.Load's zero-value fallback would make all three runtimes agree by construction
#      whether or not identity-aware rendering is actually exercised, closing AC1 (parity across
#      the *real* three outputs) vacuously.
#   4. Manifest edits go through python3, never `sed -i` (BSD vs GNU `-i` divergence was the
#      exact class of the prior CI-only failure in this series).
#
# Follows the conventions of check-branch-prune-parity.sh: set -euo pipefail, NO_COLOR=1/
# TERM=dumb, BASH_SOURCE-relative ROOT_DIR, mktemp -d fixture with cleanup trap, ok()/fail()
# accumulating FAIL=1, byte-level diff -u between runtimes on both stdout and stderr.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-doctor-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-branch-prune-parity.sh.
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
  echo "check-doctor-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-doctor-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Fixture builder — a fresh project+home pair with a FIXED identity written
# before anything else touches $HOME (restriction 3). Returns "project home"
# on stdout; the caller does `read -r project home <<<"$(build_fixture ...)"`.
# ---------------------------------------------------------------------------
IDENTITY_JSON='{
  "schema_version": 1,
  "user_nickname": "KG",
  "agents": {
    "backend": { "display_name": "Apolo", "slug": "apolo" }
  }
}'

build_fixture() {
  local dest=$1
  local project="$dest/project"
  local home="$dest/home"
  mkdir -p "$project" "$home/.trackfw"
  printf '%s\n' "$IDENTITY_JSON" >"$home/.trackfw/identity.json"
  # Resolve symlinks NOW (macOS: /var -> /private/var, /tmp -> /private/tmp) so the
  # project/home values this script uses (and later reads back out of manifest.json) match the
  # PHYSICAL path each CLI's own cwd resolution reports internally — Node/Python's cwd
  # resolution is always physical, Go's is physical only after an explicit EvalSymlinks that
  # project-root resolution does not perform. Same fix as check-thirdparty-parity.sh; without
  # it, Go writes the manifest keyed by the non-canonical path while Node/Python look it up by
  # the canonical one, so every Node/Python inspection reads back "not registered" regardless of
  # what was actually installed — an environment artifact of this gate, not a product bug.
  project=$(cd "$project" && pwd -P)
  home=$(cd "$home" && pwd -P)
  echo "$project $home"
}

# install_backend RUNTIME PROJECT HOME [EXTRA_ARGS...] — runs `agents install --items backend
# --targets claude --scope project` for real, through RUNTIME, so the manifest+artifact this
# scenario mutates were produced by an actual install (restriction 2), never hardcoded.
install_backend() {
  local runtime=$1 project=$2 home=$3
  shift 3
  case "$runtime" in
    go)   (cd "$project" && HOME="$home" "$GO_BIN" agents install --items backend --targets claude --scope project "$@") >/dev/null ;;
    node) (cd "$project" && HOME="$home" node "$NODE_CLI" agents install --items backend --targets claude --scope project "$@") >/dev/null ;;
    py)   (cd "$project" && HOME="$home" PYTHONPATH="$PY_ROOT" python3 -m trackfw agents install --items backend --targets claude --scope project "$@") >/dev/null ;;
    *)    echo "install_backend: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
}

# run_doctor RUNTIME PROJECT HOME OUT_FILE ERR_FILE [ARGS...] — runs `trackfw doctor` from
# PROJECT with $HOME=HOME, capturing stdout/stderr/exit. Sets DR_EXIT.
run_doctor() {
  local runtime=$1 project=$2 home=$3 out_file=$4 err_file=$5
  shift 5
  set +e
  case "$runtime" in
    go)   (cd "$project" && HOME="$home" "$GO_BIN" doctor "$@")                              >"$out_file" 2>"$err_file" ;;
    node) (cd "$project" && HOME="$home" node "$NODE_CLI" doctor "$@")                       >"$out_file" 2>"$err_file" ;;
    py)   (cd "$project" && HOME="$home" PYTHONPATH="$PY_ROOT" python3 -m trackfw doctor "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_doctor: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  DR_EXIT=$?
  set -e
}

# manifest_destination PROJECT — prints the single manifest artifact destination whose claims
# include item=="backend" (there is exactly one surface for the "claude" target, so exactly one
# artifact is expected after install_backend). Fails loudly instead of silently returning empty
# if the assumption ever stops holding (e.g. the catalog grows a second claude surface).
manifest_destination() {
  local project=$1
  python3 - "$project/.trackfw/integrations-manifest.json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    manifest = json.load(fh)
matches = [
    dest for dest, artifact in manifest["artifacts"].items()
    if any(claim["item"] == "backend" for claim in artifact["claims"])
]
if len(matches) != 1:
    print(f"manifest_destination: expected exactly 1 backend artifact, got {len(matches)}: {matches}", file=sys.stderr)
    sys.exit(1)
print(matches[0])
PY
}

# remove_manifest_entry PROJECT DESTINATION — deletes DESTINATION's entry from the manifest via
# python3 (restriction 4), leaving the on-disk artifact bytes untouched. This is the
# unregistered-write state: content still matches the catalog template, only the record is gone.
remove_manifest_entry() {
  local project=$1 destination=$2
  python3 - "$project/.trackfw/integrations-manifest.json" "$destination" <<'PY'
import json, sys
filename, destination = sys.argv[1:3]
with open(filename, encoding="utf-8") as fh:
    manifest = json.load(fh)
if destination not in manifest["artifacts"]:
    print(f"remove_manifest_entry: {destination} not found in manifest", file=sys.stderr)
    sys.exit(1)
del manifest["artifacts"][destination]
with open(filename, "w", encoding="utf-8") as fh:
    json.dump(manifest, fh, indent=2)
    fh.write("\n")
PY
}

# retarget_manifest_claim_item PROJECT DESTINATION NEW_ITEM — rewrites DESTINATION's single
# claim in the manifest to a DIFFERENT item id, via python3 (restriction 4), leaving the sha256
# and on-disk bytes untouched. Reproduces "registered under a different claim": Registered=true
# (an entry exists) but Managed=false (that entry no longer names the item under inspection),
# State stays Current since content was never touched — the near-miss false positive the
# ClassifyDoctor doc comment and the ML-2A audit trail both call out: keying off Managed instead
# of Registered here would report a destination that is legitimately claimed by ANOTHER item as
# an "unregistered write", which is exactly the dominant false positive doctor exists to avoid.
retarget_manifest_claim_item() {
  local project=$1 destination=$2 new_item=$3
  python3 - "$project/.trackfw/integrations-manifest.json" "$destination" "$new_item" <<'PY'
import json, sys
filename, destination, new_item = sys.argv[1:4]
with open(filename, encoding="utf-8") as fh:
    manifest = json.load(fh)
artifact = manifest["artifacts"][destination]
if len(artifact["claims"]) != 1:
    print(f"retarget_manifest_claim_item: expected exactly 1 claim, got {len(artifact['claims'])}", file=sys.stderr)
    sys.exit(1)
artifact["claims"][0]["item"] = new_item
with open(filename, "w", encoding="utf-8") as fh:
    json.dump(manifest, fh, indent=2)
    fh.write("\n")
PY
}

# normalize_json FILE — re-serializes FILE's JSON with sorted keys and fixed indentation so the
# byte-level diff in assert_three_way compares semantic content, not per-runtime JSON formatting
# style (indent width, trailing newline, key order from dict insertion vs struct field order).
normalize_json() {
  local file=$1
  python3 - "$file" <<'PY'
import json, sys
filename = sys.argv[1]
with open(filename, encoding="utf-8") as fh:
    data = json.load(fh)
with open(filename, "w", encoding="utf-8") as fh:
    json.dump(data, fh, indent=2, sort_keys=True)
    fh.write("\n")
PY
}

# assert_three_way LABEL — diffs go vs node and go vs py for both stdout and stderr, plus exit
# code equality. Mirrors check-branch-prune-parity.sh's helper of the same name.
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "doctor-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "doctor-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "doctor-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "doctor-parity/$label"
  fi
}

# run_scenario LABEL PROJECT HOME EXPECT_SUBSTRING [--json-normalize] — runs `doctor` (text) and
# `doctor --json` for all three runtimes against the SAME (project, home) pair — doctor never
# writes, so re-inspecting the same fixture from three runtimes in a row is safe — and asserts
# EXPECT_SUBSTRING appears in every text-report stdout (vacuity guard) before the byte-level
# three-way diff.
run_scenario() {
  local label=$1 project=$2 home=$3 expect_substring=$4
  for runtime in go node py; do
    run_doctor "$runtime" "$project" "$home" "$WORK/$label-text.$runtime.out" "$WORK/$label-text.$runtime.err"
    echo "$DR_EXIT" >"$WORK/$label-text.$runtime.exit"
    if [[ "$DR_EXIT" -ne 0 ]]; then
      fail "doctor-parity/$label-text/$runtime" "doctor exited $DR_EXIT unexpectedly; stderr: $(cat "$WORK/$label-text.$runtime.err")"
      continue
    fi
    if ! grep -qF "$expect_substring" "$WORK/$label-text.$runtime.out"; then
      fail "doctor-parity/$label-text/$runtime" "vacuity guard: stdout missing '$expect_substring'; stdout: $(cat "$WORK/$label-text.$runtime.out")"
      continue
    fi

    run_doctor "$runtime" "$project" "$home" "$WORK/$label-json.$runtime.out" "$WORK/$label-json.$runtime.err" --json
    echo "$DR_EXIT" >"$WORK/$label-json.$runtime.exit"
    if [[ "$DR_EXIT" -ne 0 ]]; then
      fail "doctor-parity/$label-json/$runtime" "doctor --json exited $DR_EXIT unexpectedly; stderr: $(cat "$WORK/$label-json.$runtime.err")"
      continue
    fi
    if ! python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$WORK/$label-json.$runtime.out"; then
      fail "doctor-parity/$label-json/$runtime" "--json did not emit a decodable document"
      continue
    fi
    normalize_json "$WORK/$label-json.$runtime.out"
  done
  assert_three_way "$label-text"
  assert_three_way "$label-json"
}

# ---------------------------------------------------------------------------
# Scenario (a) — clean baseline: fresh project+home, nothing installed. All three CLIs must
# report "no mismatches found" and an empty --json array.
# ---------------------------------------------------------------------------
read -r a_project a_home <<<"$(build_fixture "$WORK/a")"
run_scenario "baseline-clean" "$a_project" "$a_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (b) — unregistered-write: install for real, then remove the manifest entry via
# python3, leaving the on-disk artifact untouched (still byte-identical to the catalog
# template). All three CLIs must report exactly one unregistered-write finding, never
# hand-modified.
# ---------------------------------------------------------------------------
read -r b_project b_home <<<"$(build_fixture "$WORK/b")"
install_backend go "$b_project" "$b_home"
b_destination=$(manifest_destination "$b_project")
remove_manifest_entry "$b_project" "$b_destination"
run_scenario "unregistered-write" "$b_project" "$b_home" "[unregistered-write]"

# ---------------------------------------------------------------------------
# Scenario (c) — hand-modified: install for real, then append a byte to the on-disk artifact,
# leaving the manifest's registered hash stale. All three CLIs must report exactly one
# hand-modified finding, never unregistered-write.
# ---------------------------------------------------------------------------
read -r c_project c_home <<<"$(build_fixture "$WORK/c")"
install_backend go "$c_project" "$c_home"
c_destination=$(manifest_destination "$c_project")
printf 'x' >>"$c_destination"
run_scenario "hand-modified" "$c_project" "$c_home" "[hand-modified]"

# ---------------------------------------------------------------------------
# Scenario (d) — alien file at a real catalog destination: use `agents list --json` (read-only,
# writes nothing) to learn the exact destination `agents install --items backend --targets
# claude --scope project` WOULD use, then write garbage content there directly — without ever
# installing, so there is zero manifest entry AND the content does not match the catalog
# template. This is the dominant false-positive risk doctor exists to avoid: a real project file
# that simply is not trackfw's must never be reported.
# ---------------------------------------------------------------------------
read -r d_project d_home <<<"$(build_fixture "$WORK/d")"
d_list_json="$WORK/d-list.json"
(cd "$d_project" && HOME="$d_home" "$GO_BIN" agents list --items backend --targets claude --scope project --json) >"$d_list_json"
d_destination=$(python3 - "$d_list_json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    payload = json.load(fh)
rows = payload["deployments"]
if len(rows) != 1:
    print(f"scenario-d: expected exactly 1 deployment row, got {len(rows)}: {rows}", file=sys.stderr)
    sys.exit(1)
print(rows[0]["destination"])
PY
)
# `agents list --json` reports destination relative to the project root; join it explicitly.
case "$d_destination" in
  /*) ;;
  *) d_destination="$d_project/$d_destination" ;;
esac
mkdir -p "$(dirname "$d_destination")"
printf 'this content does not match any catalog template\n' >"$d_destination"
run_scenario "alien-file-not-flagged" "$d_project" "$d_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (e) — registered under a different claim: install for real, then retarget the
# manifest's claim item from "backend" to "architect" while leaving the sha256 and on-disk bytes
# untouched. Registered=true (an entry exists), Managed=false (that entry names a different
# item), State stays Current. All three CLIs must stay completely silent — this is the false
# positive ClassifyDoctor's own doc comment identifies as "the dominant false-positive doctor
# exists to avoid", and internal/integrations/doctor_test.go pins it at the unit level already;
# this scenario is what proves the SAME discriminant end-to-end through the real `doctor`
# command and across all three CLIs, which is what ML-2B's falsification scenario (see
# check-gates-falsify.sh) sabotages.
# ---------------------------------------------------------------------------
read -r e_project e_home <<<"$(build_fixture "$WORK/e")"
install_backend go "$e_project" "$e_home"
e_destination=$(manifest_destination "$e_project")
retarget_manifest_claim_item "$e_project" "$e_destination" "architect"
run_scenario "registered-under-different-claim" "$e_project" "$e_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-doctor-parity.sh scenarios passed."
else
  echo "check-doctor-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
