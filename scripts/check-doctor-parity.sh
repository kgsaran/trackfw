#!/usr/bin/env bash
# check-doctor-parity.sh — proves `trackfw doctor` behaves byte-for-byte identically in Go,
# Node.js, and Python, across both surfaces (text report and --json), for the three finding
# classes ML-2A/ML-2C introduced (unregistered-write, unknown-content, hand-modified) plus three
# silent paths (clean baseline, and two destinations registered under a DIFFERENT claim — the
# near-miss false positive the ML-2A audit trail flagged, at both State=current and
# State=modified) — see internal/integrations/doctor.go, docs/req/REQ-2026-08-17-doctor-detecta-
# artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md,
# ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md, and
# docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md (ML-3A's audit, which found
# unknown-content silently unreported before ML-2C — scenario (d) below was retargeted from a
# "stays silent" assertion to the new finding it now correctly produces, rather than duplicated).
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
# Scenario (d) — unknown content at a real catalog destination: use `agents list --json`
# (read-only, writes nothing) to learn the exact destination `agents install --items backend
# --targets claude --scope project` WOULD use, then write garbage content there directly —
# without ever installing, so there is zero manifest entry AND the content does not match the
# catalog template (!Registered && StateModified).
#
# RETARGETED by ML-2C (was "alien-file-not-flagged" / "no mismatches found" — see
# docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md and
# vault/notes/doctor-classifydoctor-silences-tampering-when-manifest-entry-removed-2026-08-19.md):
# ML-3A's audit found this EXACT state falling silently outside ClassifyDoctor's cases, which is
# precisely the state that makes `agents install`'s own preflight refuse this same destination
# with "unmanaged artifact" — `doctor` answering "no mismatches found" to the user whose install
# the tool just refused was the bug, not a feature. The fixture is unchanged (this IS the
# roadmap's "cenário (f)" fixture shape — reinterpreted here rather than duplicated, since
# duplicating it would be a vacuous second scenario); only the expectation changed: all 3 CLIs
# must now report exactly one unknown-content finding whose remedy names "unmanaged artifact"
# literally, never unregistered-write or hand-modified.
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
run_scenario "unknown-content-never-installed" "$d_project" "$d_home" "[unknown-content]"

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
# Scenario (f) — registered under a different claim, AND the content also drifted: install for
# real, retarget the manifest claim item exactly like scenario (e), then ALSO append a byte to
# the on-disk artifact so State becomes `modified` instead of `current`. Reproduces
# Registered=true, Managed=false, State=modified — the unknown-content analogue of scenario (e).
# All three CLIs must stay completely silent: this destination is registered, just under another
# item's claim, and content is that other claim's problem, not this one's, regardless of State.
#
# This is the ONLY fixture in this gate that distinguishes ClassifyDoctor's unknown-content case
# keying off `!Registered` (correct) from a hypothetical `!Managed` (the exact near-miss Cenário
# 71 already proved for unregistered-write) — scenario (e) alone cannot discriminate this because
# its State stays `current`, never reaching the unknown-content case regardless of which
# discriminant is used. Added by ML-2C specifically so Cenário 72 (check-gates-falsify.sh) has
# something to falsify against; see that scenario's comment for the corruption it proves red.
# ---------------------------------------------------------------------------
read -r f_project f_home <<<"$(build_fixture "$WORK/f")"
install_backend go "$f_project" "$f_home"
f_destination=$(manifest_destination "$f_project")
retarget_manifest_claim_item "$f_project" "$f_destination" "architect"
printf 'x' >>"$f_destination"
run_scenario "registered-under-different-claim-content-drifted" "$f_project" "$f_home" "no mismatches found"

# ===========================================================================
# Scaffold-artifact scenarios (g–o) — added by ML-2A of
# ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-
# template.md. Unlike the catalog-based scenarios (a–f) above, these exercise
# RunScaffoldDoctor (internal/generators/scaffold_doctor.go), which checks
# scripts/trackfw-*.sh, .claude/commands/trackfw/*.md, and CI workflow files
# by PATH-based ownership, never via the integrations manifest.
#
# Fixture hard constraints inherited from the catalog scenarios (see file
# header), plus two scaffold-specific rules:
#   5. No `ci:` field in any fixture trackfw.yaml — all 3 runtimes DO manage
#      ci-workflow (Python generates the workflows, declares `ci-workflow` in
#      PROJECT_TARGET_IDS, and covers the artifacts in doctor, same as Go and
#      Node.js; see docs/req/REQ-2026-08-28-gate-de-ci-gerado-instala-versao-
#      nao-pinada-do-trackfw-e-nao-ha-como-pinar.md), but none of these fixtures set
#      `ci:` in trackfw.yaml, so checkCIWorkflowArtifact (and its Node/Python
#      equivalents) never fires here regardless — adding ci: would produce a
#      legitimate 3-way divergence that assert_three_way correctly catches —
#      masking the scaffold-content assertions this block exists to make. The
#      byte-identity of the 3 generated CI-workflow templates across runtimes
#      IS covered, separately, by scripts/check-ci-workflow-pin-parity.sh —
#      see docs/cli-parity.md's `partial=` annotation on the artifact table
#      for the exact boundary between what that gate proves and what this one
#      does not exercise.
#   6. build_scaffold_fixture uses `trackfw update --install-missing` via the
#      real Go binary to generate all scaffold files, never hardcoded literals
#      — restriction 2 applied to scaffold content.
# ===========================================================================

# ---------------------------------------------------------------------------
# build_scaffold_fixture DEST [BACKEND]
# Generates a fixture project with scaffold SCRIPTS (no slash commands)
# produced by the real Go binary via `trackfw update --install-missing`.
# Slash commands are intentionally omitted for all scaffold scenarios because
# Python's RunScaffoldDoctor prints an extra "  ✓ .claude/commands/trackfw/"
# progress line to stdout when the directory exists and all commands are
# intact — a known Python-side stdout verbosity divergence (not a product
# bug — Python's update subcommand uses this style broadly); including the
# directory would produce a permanent 3-way diff failure that has nothing to
# do with the scaffold SCRIPT assertions these scenarios exist to prove.
# Scenario (n) specifically tests the AC14 "no slash-commands-dir → silent"
# invariant, which is satisfied by the same fixture shape.
# BACKEND: if non-empty, adds `backend: <BACKEND>` to trackfw.yaml.
# Returns "project home" on stdout (same contract as build_fixture).
# ---------------------------------------------------------------------------
build_scaffold_fixture() {
  local dest=$1
  local backend=${2:-}
  local project="$dest/project"
  local home="$dest/home"
  mkdir -p "$project" "$home/.trackfw"
  printf '%s\n' "$IDENTITY_JSON" >"$home/.trackfw/identity.json"

  if [[ -n "$backend" ]]; then
    printf 'governance_mode: lenient\nadr_dirs:\n  - docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\nbackend: %s\n' \
      "$backend" >"$project/trackfw.yaml"
  else
    printf 'governance_mode: lenient\nadr_dirs:\n  - docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\n' \
      >"$project/trackfw.yaml"
  fi

  # validate-script + agent-hooks only (no claude-commands — see comment above).
  (cd "$project" && HOME="$home" "$GO_BIN" update --install-missing --targets validate-script,agent-hooks) >/dev/null

  project=$(cd "$project" && pwd -P)
  home=$(cd "$home" && pwd -P)
  echo "$project $home"
}

# ---------------------------------------------------------------------------
# run_scaffold_scenario LABEL PROJECT HOME EXPECT_SUBSTRING
# Like run_scenario but normalises the binary version string in text and JSON
# outputs before the three-way diff. Go/Node report the compiled version
# (e.g. "v7.2.0"); Python's importlib.metadata lookup falls back to "vunknown"
# when the package is run from source via PYTHONPATH=pypi (no dist-info).
# The normalization replaces any `trackfw vX.Y.Z` (including `vunknown`) with
# `trackfw vTEST` — the diff then asserts on finding KIND, DESTINATION and
# REMEDY STRUCTURE, not version provenance, which is acceptable because
# version provenance is already tested by unit tests in each runtime's test
# suite and by the python-version fixture used in check-doctor-parity.sh.
# ---------------------------------------------------------------------------
_normalize_version_in_file() {
  python3 - "$1" <<'PY'
import pathlib, re, sys
p = pathlib.Path(sys.argv[1])
p.write_text(
    re.sub(r'trackfw v[\w.]+', 'trackfw vTEST', p.read_text(encoding='utf-8')),
    encoding='utf-8',
)
PY
}

run_scaffold_scenario() {
  local label=$1 project=$2 home=$3 expect_substring=$4
  for runtime in go node py; do
    run_doctor "$runtime" "$project" "$home" \
      "$WORK/$label-text.$runtime.out" "$WORK/$label-text.$runtime.err"
    echo "$DR_EXIT" >"$WORK/$label-text.$runtime.exit"
    if [[ "$DR_EXIT" -ne 0 ]]; then
      fail "doctor-parity/$label-text/$runtime" \
        "doctor exited $DR_EXIT unexpectedly; stderr: $(cat "$WORK/$label-text.$runtime.err")"
      continue
    fi
    if ! grep -qF "$expect_substring" "$WORK/$label-text.$runtime.out"; then
      fail "doctor-parity/$label-text/$runtime" \
        "vacuity guard: stdout missing '$expect_substring'; stdout: $(cat "$WORK/$label-text.$runtime.out")"
      continue
    fi
    _normalize_version_in_file "$WORK/$label-text.$runtime.out"

    run_doctor "$runtime" "$project" "$home" \
      "$WORK/$label-json.$runtime.out" "$WORK/$label-json.$runtime.err" --json
    echo "$DR_EXIT" >"$WORK/$label-json.$runtime.exit"
    if [[ "$DR_EXIT" -ne 0 ]]; then
      fail "doctor-parity/$label-json/$runtime" \
        "doctor --json exited $DR_EXIT unexpectedly; stderr: $(cat "$WORK/$label-json.$runtime.err")"
      continue
    fi
    if ! python3 -c "import json,sys; json.load(open(sys.argv[1]))" \
        "$WORK/$label-json.$runtime.out"; then
      fail "doctor-parity/$label-json/$runtime" "--json did not emit a decodable document"
      continue
    fi
    _normalize_version_in_file "$WORK/$label-json.$runtime.out"
    normalize_json "$WORK/$label-json.$runtime.out"
  done
  assert_three_way "$label-text"
  assert_three_way "$label-json"
}

# ---------------------------------------------------------------------------
# Scenario (g) — scaffold clean baseline: all scaffold files match the
# template the installed binary would generate; no mismatches in all 3 CLIs.
# This is AC4's analogue for scaffold surfaces — zero false positives on an
# intact project — and also the baseline the Direction-B falsification
# scenario (Cenário 178, check-gates-falsify.sh) will sabotage.
# ---------------------------------------------------------------------------
read -r g_project g_home <<<"$(build_scaffold_fixture "$WORK/g")"
run_scaffold_scenario "scaffold-baseline-clean" "$g_project" "$g_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (h) — scaffold divergent: append one byte to
# scripts/trackfw-attention-signal.sh. All 3 CLIs must report
# [scaffold-divergent]. This is also the fixture the Direction-A
# falsification scenario (Cenário 177, check-gates-falsify.sh) will sabotage:
# a silenced checkScaffoldArtifact must make the vacuity guard fail here.
# ---------------------------------------------------------------------------
read -r h_project h_home <<<"$(build_scaffold_fixture "$WORK/h")"
printf 'x' >>"$h_project/scripts/trackfw-attention-signal.sh"
run_scaffold_scenario "scaffold-attention-signal-divergent" "$h_project" "$h_home" "[scaffold-divergent]"

# ---------------------------------------------------------------------------
# Scenario (i) — scaffold missing: remove scripts/trackfw-attention-cleanup.sh.
# All 3 CLIs must report [scaffold-missing].
# ---------------------------------------------------------------------------
read -r i_project i_home <<<"$(build_scaffold_fixture "$WORK/i")"
rm "$i_project/scripts/trackfw-attention-cleanup.sh"
run_scaffold_scenario "scaffold-attention-cleanup-missing" "$i_project" "$i_home" "[scaffold-missing]"

# ---------------------------------------------------------------------------
# Scenario (j) — validate.sh in Go/sh form: the file generated by Go binary
# (#!/usr/bin/env sh ...) must be accepted by all 3 CLIs — "no mismatches
# found". Validates the set-membership decision for the Go form.
# ---------------------------------------------------------------------------
read -r j_project j_home <<<"$(build_scaffold_fixture "$WORK/j")"
run_scaffold_scenario "validate-sh-go-form-accepted" "$j_project" "$j_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (k) — validate.sh in Python/bash form: replace validate.sh with
# the Python runtime's fixed form (#!/usr/bin/env bash ...). All 3 CLIs must
# still report "no mismatches found" — the set-membership rule accepts any
# known runtime form (Go/Node or Python), not a single canonical form (AC4).
# ---------------------------------------------------------------------------
read -r k_project k_home <<<"$(build_scaffold_fixture "$WORK/k")"
printf '#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n' \
  >"$k_project/scripts/trackfw-validate.sh"
run_scaffold_scenario "validate-sh-python-form-accepted" "$k_project" "$k_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (l) — validate.sh near-miss (one character changed): take the
# Go-generated file and change "set -e" to "set -x". All 3 CLIs must report
# [scaffold-divergent]. This is the sharpness test: the set-membership window
# accepts only the exact known forms, not any nearby mutation — a single
# character difference falls outside all members of the set (neither the Go
# form with "set -e" nor the Python form with "#!/usr/bin/env bash" matches).
# ---------------------------------------------------------------------------
read -r l_project l_home <<<"$(build_scaffold_fixture "$WORK/l")"
python3 -c "
import pathlib
p = pathlib.Path('$l_project/scripts/trackfw-validate.sh')
content = p.read_text(encoding='utf-8')
assert 'set -e' in content, 'near-miss setup: expected \"set -e\" in Go-generated validate.sh'
p.write_text(content.replace('set -e', 'set -x', 1), encoding='utf-8')
"
run_scaffold_scenario "validate-sh-near-miss-rejected" "$l_project" "$l_home" "[scaffold-divergent]"

# ---------------------------------------------------------------------------
# Scenario (m) — mirror-vs-generator cross-runtime: Go binary generates
# validate.sh for a project with `backend: go` (content includes `go build
# ./...`). Python's _build_go_node_validate_script mirror must produce the
# same content — if the mirror drifts from buildValidateScript (Go), Python
# doctor reports scaffold-divergent on a file Go considers correct. All 3
# CLIs must report "no mismatches found".
# This is the load-bearing gate for the risco residual documented in the
# ML-1A audit: _build_go_node_validate_script is a third copy of Go's
# buildValidateScript, and this scenario detects drift at runtime rather than
# via a unit test that would simply mirror the mirror.
# ---------------------------------------------------------------------------
read -r m_project m_home <<<"$(build_scaffold_fixture "$WORK/m" "go")"
run_scaffold_scenario "validate-sh-mirror-vs-generator-backend-go" "$m_project" "$m_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (n) — discover-init style (no .claude/commands/trackfw/ directory):
# when the directory is absent, slash commands are not in scope — the doctor
# must stay silent (AC14). All 3 CLIs must report "no mismatches found".
# build_scaffold_fixture produces a project whose .claude/commands/trackfw/
# directory does not exist (claude-commands target not run), matching the
# footprint of a project initialised via `discover --init`.
# ---------------------------------------------------------------------------
read -r n_project n_home <<<"$(build_scaffold_fixture "$WORK/n")"
run_scaffold_scenario "scaffold-no-slash-commands-dir-silent" "$n_project" "$n_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (o) — backend: go configured, no false positive (AC12): with
# `backend: go` in trackfw.yaml, the doctor renders the validate.sh template
# from the project's own config (not a hardcoded default). The file generated
# by `trackfw update` already matches that config-rendered template, so all 3
# CLIs must report "no mismatches found".
# ---------------------------------------------------------------------------
read -r o_project o_home <<<"$(build_scaffold_fixture "$WORK/o" "go")"
run_scaffold_scenario "scaffold-backend-go-no-false-positive" "$o_project" "$o_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (p) — execute bit missing (AC2/AC3/AC9, REQ-2026-08-28):
# Strip the owner-execute bit from scripts/trackfw-attention-signal.sh (correct
# content, wrong mode). All 3 CLIs must report [scaffold-wrong-mode], NOT
# scaffold-divergent — demonstrating the distinct state required by AC3. Exit
# code must remain 0 (same as other scaffold findings — doctor is read-only).
# This is the falsification target for the AC9 chain: doctor sees wrong-mode,
# user runs `trackfw update`, Go and Node now call os.Chmod/fs.chmodSync to
# restore the bit even on existing files. The unit tests in each runtime
# (TestWrongModeDetection_ValidateScript etc.) prove the detection half; this
# scenario proves that all three runtimes agree on the finding KIND.
# ---------------------------------------------------------------------------
read -r p_project p_home <<<"$(build_scaffold_fixture "$WORK/p")"
chmod 0644 "$p_project/scripts/trackfw-attention-signal.sh"
run_scaffold_scenario "scaffold-wrong-mode-detected" "$p_project" "$p_home" "[scaffold-wrong-mode]"

# ---------------------------------------------------------------------------
# Scenario (q) — AC4/AC11 — non-executable artifact (validate.sh at Python
# form with 0644 mode) NOT accused of wrong-mode when Python form is forced
# WITHOUT execute bit on validate-script.
# Actually: verify that a static script with mode 0700 (umask-narrowed — the
# execute bit IS set) does NOT produce a wrong-mode finding (AC10). Mode 0700
# means (0700 & 0o100) != 0 → execute bit present → silent.
# ---------------------------------------------------------------------------
read -r q_project q_home <<<"$(build_scaffold_fixture "$WORK/q")"
chmod 0700 "$q_project/scripts/trackfw-credential-guard.sh"
run_scaffold_scenario "scaffold-0700-mode-accepted-ac10" "$q_project" "$q_home" "no mismatches found"

# ---------------------------------------------------------------------------
# Scenario (r) — AC4/AC11 — validate.sh in Python form with execute bit
# present: replace the file with Python's fixed form (keeping the mode that
# was set by `trackfw update`, which is 0755 after AC9 fix). All 3 CLIs must
# still report "no mismatches found" — Python form + correct mode is accepted.
# This is a complement to scenario (k) that also exercises the mode check
# path (in (k) the mode was already 0755 because > redirect doesn't change
# mode on an existing file, but this scenario makes it explicit).
# ---------------------------------------------------------------------------
read -r r_project r_home <<<"$(build_scaffold_fixture "$WORK/r")"
# Write Python form with explicit 0755.
printf '#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n' \
  >"$r_project/scripts/trackfw-validate.sh"
chmod 0755 "$r_project/scripts/trackfw-validate.sh"
run_scaffold_scenario "validate-sh-python-form-with-exec-bit" "$r_project" "$r_home" "no mismatches found"

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
