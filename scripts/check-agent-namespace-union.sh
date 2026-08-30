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
#   infra filter — `node_modules` and an orphan state-name dir (`wip/` alone
#          at the top of roadmap_dir) never trigger the undeclared-namespace
#          violation; `.git`-like dot-prefixed dirs neither (ML-4A: see
#          "hidden namespace" below — they get a low-noise WARNING instead of
#          silence AND instead of the hard violation).
#   hidden namespace (ML-4A, achado 1) — a dot-prefixed namespace with real
#          roadmap content on disk (e.g. `.ghost`) is NEVER invisible: it
#          stays enumerated by status/validate/move exactly like any other
#          undeclared namespace, but the undeclared-namespace signal is the
#          low-noise `agent_namespace_hidden` warning, not the hard
#          `agent_namespace_undeclared` violation.
#   glob metacharacter safety (ML-4A, achado 2) — an on-disk namespace named
#          literally "*" does not cross-match every other namespace's wip/
#          (silent WIP-count corruption), and a namespace with an unbalanced
#          "[" does not abort `validate` with a raw pattern error (Go's
#          filepath.Glob receiving an unescaped, disk-derived path segment).
#   flat  — `roadmap_namespacing: flat` never emits `agent_namespace_undeclared`.
#   declared-first ordering (artemis-tf, 2026-08-30, ML-4B docs follow-up) —
#          the resolver returns declared namespaces first (in `agents:` order,
#          deduplicated), then disk-only extras alphabetically. This property
#          was already documented in docs/cli-parity.md as
#          "load-bearing for gate", but until this addition no scenario in
#          this file (or in check-gates-falsify.sh, whose Cenários 34/35 were
#          RETARGETED away from ordering onto the `agent_namespace_undeclared`
#          violation signal in ML-3A) actually asserted it — the annotation
#          was overclaiming coverage the gate didn't have. `roadmap list` is
#          checked (the surface docs/cli-parity.md names explicitly), across
#          all 3 runtimes.
#
# Falsification, BOTH directions, SELF-CONTAINED (this script has no external
# caller — check-gates-falsify.sh is out of scope for new scenarios in this
# ML, see roadmap ML-3A file list — so the corruption+detection lives here,
# same technique as check-gates-falsify.sh's corrupt_literal, duplicated
# rather than sourced to keep this gate independently runnable):
#   Direction A  — union reverts to substitution (the pre-REQ-2026-08-29
#                  defect): an undeclared-but-on-disk namespace goes
#                  invisible again the moment `agents:` is non-empty.
#   Direction B1 — infra filter disabled: `node_modules` starts being
#                  treated as a namespace (the REQ's "ADR-2026-08-17: a guard
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
#   Direction B3 — ML-4A, achado 1 corrective, hades-tf 2026-08-30: the
#                  dot-prefix carve-out reverts to isInfraDirName's old,
#                  overbroad "any name starting with '.'" — a dot-prefixed
#                  namespace with real content (`.ghost`) goes fully invisible
#                  again (the exact defect this ML exists to close).
#   Direction B4 — ML-4A, achado 2 corrective, hades-tf 2026-08-30 (Go only —
#                  Node/Python were never exposed, see achado 2's runtime
#                  note): ListMDFiles reverts to filepath.Glob on a
#                  disk-derived agent name — a namespace literally named "*"
#                  cross-matches every other namespace's wip/ again.
#   Direction C  — declared-first ordering regresses to plain alphabetical
#                  (the resolver stops putting `agents:`-declared namespaces
#                  first): a single `sort()`/`sort.Strings()`/`.sort()` added
#                  right before the resolver returns is enough in all 3
#                  runtimes — there is no "invisibility vs. over-visibility"
#                  duality here (unlike Directions A/B1/B2/B3/B4, which each
#                  pair an under- and an over-enumeration failure mode), so
#                  this direction is proved once per runtime, not paired with
#                  an opposite-direction arm — same precedent as
#                  check-gates-falsify.sh's Cenário 33 (Python-only ordering
#                  falsification, single direction, reverse-sort corruption).
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
# Infra filter — node_modules / orphan state-name dir never trigger the hard
# violation NOR the hidden-namespace warning (they carry zero ambiguity: no
# operator names an agent "node_modules", and "wip" is a reserved state
# name). `.git` — a dot-prefixed name — is NOT filtered from the union
# anymore (ML-4A, achado 1): it must never be accused via the hard
# `agent_namespace_undeclared` violation, but it MUST surface via the new
# low-noise `agent_namespace_hidden` warning — never zero signal.
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
    fail "infra-filter/$runtime/dotgit-not-hard-violation" ".git accused as undeclared namespace (hard violation) — output: $(printf '%q' "$validate_out")"
  fi
  if grep -qF 'agent namespace "node_modules"' <<<"$validate_out"; then
    fail "infra-filter/$runtime/node_modules" "node_modules accused as undeclared namespace — output: $(printf '%q' "$validate_out")"
  fi
  if grep -qF 'agent namespace "wip"' <<<"$validate_out"; then
    fail "infra-filter/$runtime/orphan-wip" "orphan wip/ accused as undeclared namespace — output: $(printf '%q' "$validate_out")"
  fi
  if grep -qF 'dot-prefixed directory "node_modules"' <<<"$validate_out"; then
    fail "infra-filter/$runtime/node_modules-not-hidden-either" "node_modules should be fully filtered by the closed infra list, not merely downgraded to a hidden-namespace warning — output: $(printf '%q' "$validate_out")"
  fi
  ok "infra-filter/$runtime/node_modules-orphan-wip-silent"

  if ! grep -qF 'dot-prefixed directory ".git"' <<<"$validate_out"; then
    fail "hidden-namespace/$runtime/dotgit-never-invisible" ".git is no longer in the closed infra list — it must surface via the agent_namespace_hidden warning (ML-4A, achado 1); it disappeared with zero signal instead — output: $(printf '%q' "$validate_out")"
  fi
  ok "hidden-namespace/$runtime/dotgit-surfaces-as-warning"
done

# ===========================================================================
# ML-4A, achado 1 — a dot-prefixed namespace with REAL roadmap content
# (`.ghost`, not just an empty dir) must never be invisible: enumerated by
# status/validate/move exactly like `bob` in P1 (AC1), the undeclared signal
# is the low-noise warning (not the hard violation), and `roadmap move`
# actually relocates the file.
# ===========================================================================
P5="$WORK/p5-hidden-namespace"
scaffold_by_agent "$P5" "- alice"
mkdir -p "$P5/docs/roadmaps/alice/wip"
write_wip_roadmap "$P5/docs/roadmaps/alice/wip/ROADMAP-alice-wip.md" "alice wip"
write_wip_roadmap "$P5/docs/roadmaps/.ghost/wip/ROADMAP-ghost-wip.md" "ghost wip (dot-prefixed, não declarado)"

for runtime in go node python; do
  status_out=$(run_"$runtime" "$P5" status; true)
  if ! grep -qF "ROADMAP-ghost-wip.md" <<<"$status_out"; then
    fail "hidden-namespace/$runtime/status-enumerates-dotghost" "'status' did not list .ghost's roadmap — dot-prefixed namespace went invisible (the exact defect this ML exists to close)"
  fi
  ok "hidden-namespace/$runtime/status-enumerates-dotghost"

  validate_out=$(run_"$runtime" "$P5" validate; true)
  if ! grep -qF 'roadmap "ROADMAP-ghost-wip.md" is in wip but has no linked REQ' <<<"$validate_out"; then
    fail "hidden-namespace/$runtime/validate-scans-dotghost" "'validate' did not scan .ghost's roadmap file content — union not applied to a dot-prefixed namespace"
  fi
  if grep -qiF 'agent namespace ".ghost"' <<<"$validate_out"; then
    fail "hidden-namespace/$runtime/dotghost-not-hard-violation" ".ghost accused via the hard agent_namespace_undeclared violation — should be the low-noise agent_namespace_hidden warning instead — output: $(printf '%q' "$validate_out")"
  fi
  if ! grep -qF 'dot-prefixed directory ".ghost"' <<<"$validate_out"; then
    fail "hidden-namespace/$runtime/dotghost-surfaces-as-warning" ".ghost produced no signal at all (neither hard violation nor hidden-namespace warning) — zero-signal invisibility — output: $(printf '%q' "$validate_out")"
  fi
  ok "hidden-namespace/$runtime/dotghost-validate-signal"
done

for runtime in go node python; do
  P5_MOVE="$WORK/p5-move-$runtime"
  cp -r "$P5" "$P5_MOVE"
  move_out=$(run_"$runtime" "$P5_MOVE" roadmap move ROADMAP-ghost-wip done; true)
  if [[ ! -f "$P5_MOVE/docs/roadmaps/.ghost/done/ROADMAP-ghost-wip.md" ]]; then
    fail "hidden-namespace/$runtime/roadmap-move-finds-dotghost" "roadmap move did not relocate .ghost's roadmap to done/ — output: $(printf '%q' "$move_out")"
  fi
  ok "hidden-namespace/$runtime/roadmap-move-finds-dotghost"
done

# ===========================================================================
# ML-4A, achado 2 — an on-disk agent namespace named "*" must not
# cross-match every other namespace's wip/ (silent WIP-count corruption),
# and one named with an unbalanced "[" must not abort `validate` (Go's
# filepath.Glob receiving an unescaped, disk-derived path segment) nor break
# `--json`. Fixture mirrors hades-tf's live repro: alfa 3 files, beta 1 file
# (undeclared, ordinary control), a namespace LITERALLY named "*" with 1
# file, wip_limit: 1.
# ===========================================================================
P6="$WORK/p6-glob-metachar"
mkdir -p "$P6/docs/adr" "$P6/docs/req"
mkdir -p "$P6/docs/roadmaps/alfa/wip" "$P6/docs/roadmaps/beta/wip" "$P6/docs/roadmaps/*/wip"
cat > "$P6/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
  - alfa
wip_limit: 1
EOF
write_wip_roadmap "$P6/docs/roadmaps/alfa/wip/ROADMAP-alfa-1.md" "alfa 1"
write_wip_roadmap "$P6/docs/roadmaps/alfa/wip/ROADMAP-alfa-2.md" "alfa 2"
write_wip_roadmap "$P6/docs/roadmaps/alfa/wip/ROADMAP-alfa-3.md" "alfa 3"
write_wip_roadmap "$P6/docs/roadmaps/beta/wip/ROADMAP-beta-1.md" "beta 1"
write_wip_roadmap "$P6/docs/roadmaps/*/wip/ROADMAP-star-1.md" "star 1"
write_wip_roadmap "$P6/docs/roadmaps/*/wip/ROADMAP-star-2.md" "star 2"

for runtime in go node python; do
  validate_out=$(run_"$runtime" "$P6" validate; true)
  if ! grep -qF '3 roadmaps in wip/ for agent "alfa"' <<<"$validate_out"; then
    fail "glob-metachar/$runtime/liveness-anchor" "validate produced no usable wip_limit warning for alfa at all — cannot distinguish 'star correctly isolated' from 'validate crashed/empty output'; output: $(printf '%q' "$validate_out")"
  fi
  if ! grep -qF '2 roadmaps in wip/ for agent "*"' <<<"$validate_out"; then
    fail "glob-metachar/$runtime/star-count-not-inflated" "the \"*\" namespace's WIP count is not exactly 2 (its own files only) — a literal \"*\" namespace is cross-matching other namespaces' wip/ via an unescaped glob pattern — output: $(printf '%q' "$validate_out")"
  fi
  ok "glob-metachar/$runtime/star-namespace-isolated-count"
done

P7="$WORK/p7-glob-bracket"
mkdir -p "$P7/docs/adr" "$P7/docs/req"
mkdir -p "$P7/docs/roadmaps/alfa/wip" "$P7/docs/roadmaps/unmatched[bracket/wip"
cat > "$P7/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
  - alfa
EOF
write_wip_roadmap "$P7/docs/roadmaps/alfa/wip/ROADMAP-alfa-1.md" "alfa 1"
write_wip_roadmap "$P7/docs/roadmaps/unmatched[bracket/wip/ROADMAP-bracket-1.md" "bracket 1"

for runtime in go node python; do
  set +e
  validate_out=$(run_"$runtime" "$P7" validate)
  validate_status=$?
  set -e
  if grep -qF 'syntax error in pattern' <<<"$validate_out"; then
    fail "glob-metachar/$runtime/bracket-no-crash" "'validate' leaked a raw Go pattern error instead of the intended agent_namespace_undeclared violation — output: $(printf '%q' "$validate_out"), exit=$validate_status"
  fi
  if ! grep -qF 'agent namespace "unmatched[bracket"' <<<"$validate_out"; then
    fail "glob-metachar/$runtime/bracket-still-enumerated" "the bracket-named namespace produced no agent_namespace_undeclared violation at all — output: $(printf '%q' "$validate_out"), exit=$validate_status"
  fi
  ok "glob-metachar/$runtime/bracket-no-crash-still-violates"

  # stdout-only (NOT run_*, which merges stderr via 2>&1 — the "N violation(s) found" line goes to
  # stderr and would corrupt the JSON payload if merged; Node pretty-prints JSON across many lines,
  # so a naive "first line only" split doesn't isolate it either).
  set +e
  case "$runtime" in
    go)     validate_json_stdout=$(cd "$P7" && "$GO_BIN" validate --json 2>/dev/null) ;;
    node)   validate_json_stdout=$(cd "$P7" && node "$NODE_CLI" validate --json 2>/dev/null) ;;
    python) validate_json_stdout=$(cd "$P7" && env PYTHONPATH="$PY_ROOT" python3 -m trackfw validate --json 2>/dev/null) ;;
  esac
  set -e
  if ! python3 -c "import json,sys; json.loads(sys.argv[1])" "$validate_json_stdout" >/dev/null 2>&1; then
    fail "glob-metachar/$runtime/bracket-json-valid" "'validate --json' did not emit valid JSON for the bracket-named namespace — stdout: $(printf '%q' "$validate_json_stdout")"
  fi
  ok "glob-metachar/$runtime/bracket-json-valid"
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

# ===========================================================================
# Declared-first ordering — the resolver returns `agents:`-declared
# namespaces first (in declared order, deduplicated), then disk-only extras
# alphabetically. docs/cli-parity.md names this "load-bearing for gate" but,
# until this scenario, nothing in this repo actually asserted it for Go/Node
# — check-gates-falsify.sh's Cenários 34/35 were RETARGETED away from order
# onto the `agent_namespace_undeclared` violation signal (ML-3A), and its
# only remaining order-sensitive scenario (Cenário 33,
# falsify/status-by-agent-fallback-order) is Python-only and exercises
# `status`, not `roadmap list` — the surface docs/cli-parity.md names
# explicitly. `agents: [zulu, alfa]` deliberately inverts alphabetical order
# (zulu declared BEFORE alfa) so a plain-alphabetical regression is
# distinguishable from the correct declared-first output; `extra` is
# disk-only and undeclared, expected last.
# ===========================================================================
P7="$WORK/p7-ordering"
scaffold_by_agent "$P7" $'- zulu\n- alfa'
mkdir -p "$P7/docs/roadmaps/zulu/wip" "$P7/docs/roadmaps/alfa/wip" "$P7/docs/roadmaps/extra/wip"
write_wip_roadmap "$P7/docs/roadmaps/zulu/wip/ROADMAP-zulu-wip.md" "zulu wip (declared 1st in agents:, alphabetically 2nd)"
write_wip_roadmap "$P7/docs/roadmaps/alfa/wip/ROADMAP-alfa-wip.md" "alfa wip (declared 2nd in agents:, alphabetically 1st)"
write_wip_roadmap "$P7/docs/roadmaps/extra/wip/ROADMAP-extra-wip.md" "extra wip (undeclared, disk-only)"

# assert_order OUTPUT RUNTIME LABEL MARKER... — fails unless each marker's
# first line number strictly increases through the given output, in the
# given sequence. A marker missing entirely is a distinct failure from a
# marker present but out of order (both are diagnosed by name, not by a
# single generic "order wrong" message).
assert_order() {
  local out=$1 runtime=$2 label=$3
  shift 3
  local prev_ln=0 marker ln
  for marker in "$@"; do
    ln=$(printf '%s\n' "$out" | grep -n -F -- "$marker" | head -1 | cut -d: -f1)
    if [[ -z "$ln" ]]; then
      fail "$label/$runtime/marker-missing" "marker '$marker' not found in output — output: $(printf '%q' "$out")"
    fi
    if (( ln <= prev_ln )); then
      fail "$label/$runtime/order-wrong" "marker '$marker' at line $ln did not come strictly after the previous marker (line $prev_ln) — output: $(printf '%q' "$out")"
    fi
    prev_ln=$ln
  done
}

for runtime in go node python; do
  list_out=$(run_"$runtime" "$P7" roadmap list; true)
  # "[zulu" matches both Go/Node's "[zulu/wip]" and Python's "[zulu]" — a
  # substring, not the full bracketed token, is deliberate so the same
  # markers work across the 3 runtimes' differing `roadmap list` formatting
  # (see the known `gap` for that formatting divergence elsewhere in
  # docs/cli-parity.md; only ORDER is asserted here, never presentation).
  assert_order "$list_out" "$runtime" "ordering" "[zulu" "[alfa" "[extra"
  ok "ordering/$runtime/declared-first-then-disk-only-alphabetical"
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
# Direction B1 — infra filter disabled: `node_modules` starts being accused
# as an undeclared namespace. (`.git` is deliberately NOT part of this
# direction anymore — ML-4A narrowed the closed list to `node_modules` only;
# `.git`'s regression is Direction B3 below, and it regresses to invisibility,
# not to an accusation, because dot-prefixed names get a different — and
# lower-severity — treatment now.)
# ===========================================================================

# --- Go ---
TB1_GO_MOD="$WORK/dirb1-go-mod"
mkdir -p "$TB1_GO_MOD/cmd" "$TB1_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$TB1_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$TB1_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$TB1_GO_MOD/"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$TB1_GO_MOD/internal/validator/validator.go" \
  'return name == "node_modules"' \
  'return false' \
  "direction-b1-go"
TB1_GO_BIN="$WORK/dirb1-go-bin/trackfw"
mkdir -p "$(dirname "$TB1_GO_BIN")"
build_go_or_fail "direction-b1/go/build" "$TB1_GO_MOD" "$TB1_GO_BIN"

dirb1_go_out=$(cd "$P2" && "$TB1_GO_BIN" validate 2>&1; true)
if ! grep -qF 'agent namespace "node_modules"' <<<"$dirb1_go_out"; then
  fail "direction-b1/go/detects-infra-filter-regression" "corrupted binary did not accuse node_modules — checagem vácua"
fi
ok "direction-b1/go/detects-infra-filter-regression"

# --- Node ---
TB1_N="$WORK/dirb1-node"
setup_npm_tree "$TB1_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$TB1_N/npm/src/validator/index.js" \
  "return name === 'node_modules'" \
  "return false" \
  "direction-b1-node"

dirb1_node_out=$(cd "$P2" && node "$TB1_N/npm/bin/trackfw" validate 2>&1; true)
if ! grep -qF 'agent namespace "node_modules"' <<<"$dirb1_node_out"; then
  fail "direction-b1/node/detects-infra-filter-regression" "corrupted binary did not accuse node_modules — checagem vácua"
fi
ok "direction-b1/node/detects-infra-filter-regression"

# --- Python ---
TB1_P="$WORK/dirb1-python"
setup_py_tree "$TB1_P"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$TB1_P/pypi/trackfw/config.py" \
  'return name == "node_modules"' \
  'return False' \
  "direction-b1-python"

dirb1_python_out=$(cd "$P2" && env PYTHONPATH="$TB1_P/pypi" python3 -m trackfw validate 2>&1; true)
if ! grep -qF 'agent namespace "node_modules"' <<<"$dirb1_python_out"; then
  fail "direction-b1/python/detects-infra-filter-regression" "corrupted binary did not accuse node_modules — checagem vácua"
fi
ok "direction-b1/python/detects-infra-filter-regression"

# ===========================================================================
# Direction B3 (ML-4A, achado 1) — the dot-prefix carve-out reverts to
# isInfraDirName's old, overbroad "any name starting with '.'": `.ghost`
# (P5, WITH real roadmap content) must go fully invisible again — the exact
# defect this ML exists to close. Corrupts isDotPrefixedName to always
# return false, which collapses undeclaredNamespacesOnDisk back to treating
# every dot-prefixed name as ordinary AND removes it from the union filter
# emulation is not needed here: the real regression under test is
# "isInfraDirName absorbs the dot-prefix check again", so corrupt
# isInfraDirName directly (same function, opposite direction from B1).
# ===========================================================================

# --- Go ---
TB3_GO_MOD="$WORK/dirb3-go-mod"
mkdir -p "$TB3_GO_MOD/cmd" "$TB3_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$TB3_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$TB3_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$TB3_GO_MOD/"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$TB3_GO_MOD/internal/validator/validator.go" \
  'func isInfraDirName(name string) bool {
	return name == "node_modules"
}' \
  'func isInfraDirName(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules"
}' \
  "direction-b3-go"
TB3_GO_BIN="$WORK/dirb3-go-bin/trackfw"
mkdir -p "$(dirname "$TB3_GO_BIN")"
build_go_or_fail "direction-b3/go/build" "$TB3_GO_MOD" "$TB3_GO_BIN"

dirb3_go_status=$(cd "$P5" && "$TB3_GO_BIN" status 2>&1; true)
if ! grep -qF "ROADMAP-alice-wip.md" <<<"$dirb3_go_status"; then
  fail "direction-b3/go/detects-dotprefix-invisibility-regression" "the corrupted binary produced no usable output at all (alice, the DECLARED namespace, is absent too) — cannot distinguish '.ghost correctly vanished' from 'the binary crashed/empty output'; output: $(printf '%q' "$dirb3_go_status")"
fi
if grep -qF "ROADMAP-ghost-wip.md" <<<"$dirb3_go_status"; then
  fail "direction-b3/go/detects-dotprefix-invisibility-regression" "corrupted binary still shows .ghost's roadmap — checagem vácua"
fi
ok "direction-b3/go/detects-dotprefix-invisibility-regression"

# --- Node ---
TB3_N="$WORK/dirb3-node"
setup_npm_tree "$TB3_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$TB3_N/npm/src/validator/index.js" \
  $'function isInfraDirName(name) {\n  return name === \'node_modules\'\n}' \
  $'function isInfraDirName(name) {\n  return name.startsWith(\'.\') || name === \'node_modules\'\n}' \
  "direction-b3-node"

dirb3_node_status=$(cd "$P5" && node "$TB3_N/npm/bin/trackfw" status 2>&1; true)
if ! grep -qF "ROADMAP-alice-wip.md" <<<"$dirb3_node_status"; then
  fail "direction-b3/node/detects-dotprefix-invisibility-regression" "the corrupted tree produced no usable output at all (alice, the DECLARED namespace, is absent too) — cannot distinguish '.ghost correctly vanished' from 'node crashed/empty output'; output: $(printf '%q' "$dirb3_node_status")"
fi
if grep -qF "ROADMAP-ghost-wip.md" <<<"$dirb3_node_status"; then
  fail "direction-b3/node/detects-dotprefix-invisibility-regression" "corrupted binary still shows .ghost's roadmap — checagem vácua"
fi
ok "direction-b3/node/detects-dotprefix-invisibility-regression"

# --- Python ---
TB3_P="$WORK/dirb3-python"
setup_py_tree "$TB3_P"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$TB3_P/pypi/trackfw/config.py" \
  'return name == "node_modules"' \
  'return name.startswith(".") or name == "node_modules"' \
  "direction-b3-python"

dirb3_python_status=$(cd "$P5" && env PYTHONPATH="$TB3_P/pypi" python3 -m trackfw status 2>&1; true)
if ! grep -qF "ROADMAP-alice-wip.md" <<<"$dirb3_python_status"; then
  fail "direction-b3/python/detects-dotprefix-invisibility-regression" "the corrupted tree produced no usable output at all (alice, the DECLARED namespace, is absent too) — cannot distinguish '.ghost correctly vanished' from 'python crashed/empty output'; output: $(printf '%q' "$dirb3_python_status")"
fi
if grep -qF "ROADMAP-ghost-wip.md" <<<"$dirb3_python_status"; then
  fail "direction-b3/python/detects-dotprefix-invisibility-regression" "corrupted binary still shows .ghost's roadmap — checagem vácua"
fi
ok "direction-b3/python/detects-dotprefix-invisibility-regression"

# ===========================================================================
# Direction B4 (ML-4A, achado 2, Go only — Node/Python were never exposed to
# this vector, see achado 2's runtime note in the header comment) —
# ListMDFiles reverts to filepath.Glob on a disk-derived agent name: the
# literal "*" namespace (P6) cross-matches every other namespace's wip/
# again, inflating its WIP count.
# ===========================================================================

TB4_GO_MOD="$WORK/dirb4-go-mod"
mkdir -p "$TB4_GO_MOD/cmd" "$TB4_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$TB4_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$TB4_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$TB4_GO_MOD/"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$TB4_GO_MOD/internal/validator/validator.go" \
  $'\t\t\tfiles := ListMDFiles(filepath.Join(projectCfg.RoadmapDir, agent, "wip"))\n' \
  $'\t\t\tfiles, _ := filepath.Glob(filepath.Join(projectCfg.RoadmapDir, agent, "wip", "*.md")) // CORRUPTED (direction-b4)\n' \
  "direction-b4-go"
TB4_GO_BIN="$WORK/dirb4-go-bin/trackfw"
mkdir -p "$(dirname "$TB4_GO_BIN")"
build_go_or_fail "direction-b4/go/build" "$TB4_GO_MOD" "$TB4_GO_BIN"

dirb4_go_out=$(cd "$P6" && "$TB4_GO_BIN" validate 2>&1; true)
if ! grep -qF '3 roadmaps in wip/ for agent "alfa"' <<<"$dirb4_go_out"; then
  fail "direction-b4/go/detects-glob-crossmatch-regression" "the corrupted binary produced no usable wip_limit warning for alfa at all — cannot distinguish 'star correctly isolated' from 'the binary crashed/empty output'; output: $(printf '%q' "$dirb4_go_out")"
fi
if grep -qF '2 roadmaps in wip/ for agent "*"' <<<"$dirb4_go_out"; then
  fail "direction-b4/go/detects-glob-crossmatch-regression" "corrupted binary still reports the isolated count of 2 for \"*\" — checagem vácua (expected the inflated, cross-matched count)"
fi
ok "direction-b4/go/detects-glob-crossmatch-regression"

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

# ===========================================================================
# Direction C — declared-first ordering regresses to plain alphabetical: a
# single sort of the FULL `ordered` slice/array/list right before the
# resolver returns is enough to lose the property in all 3 runtimes. Proved
# via `roadmap list` against P7 (agents: [zulu, alfa] — zulu declared first,
# alphabetically second): the corrupted output must still contain all 3
# namespaces (liveness — a crash or empty output must not be confused with
# "the order regression was detected") but with alfa now ahead of zulu.
# ===========================================================================

# --- Go ---
TC_GO_MOD="$WORK/dirc-go-mod"
mkdir -p "$TC_GO_MOD/cmd" "$TC_GO_MOD/internal"
cp -r "$ROOT_DIR/cmd/." "$TC_GO_MOD/cmd/"
cp -r "$ROOT_DIR/internal/." "$TC_GO_MOD/internal/"
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$TC_GO_MOD/"
corrupt_literal \
  "$ROOT_DIR/internal/validator/validator.go" "$TC_GO_MOD/internal/validator/validator.go" \
  $'\t\tordered = append(ordered, name)\n\t}\n\treturn ordered\n}' \
  $'\t\tordered = append(ordered, name)\n\t}\n\tsort.Strings(ordered)\n\treturn ordered\n}' \
  "direction-c-go"
TC_GO_BIN="$WORK/dirc-go-bin/trackfw"
mkdir -p "$(dirname "$TC_GO_BIN")"
build_go_or_fail "direction-c/go/build" "$TC_GO_MOD" "$TC_GO_BIN"

dirc_go_out=$(cd "$P7" && "$TC_GO_BIN" roadmap list 2>&1; true)
for marker in "[zulu" "[alfa" "[extra"; do
  if ! grep -qF -- "$marker" <<<"$dirc_go_out"; then
    fail "direction-c/go/detects-order-regression" "the corrupted binary produced no usable output at all (marker '$marker' absent too) — cannot distinguish 'order regressed' from 'binary crashed/empty output'; output: $(printf '%q' "$dirc_go_out")"
  fi
done
alfa_ln=$(grep -n -F -- "[alfa" <<<"$dirc_go_out" | head -1 | cut -d: -f1)
zulu_ln=$(grep -n -F -- "[zulu" <<<"$dirc_go_out" | head -1 | cut -d: -f1)
if (( zulu_ln < alfa_ln )); then
  fail "direction-c/go/detects-order-regression" "corrupted binary still put zulu (declared 1st) before alfa (declared 2nd) — checagem vácua, output: $(printf '%q' "$dirc_go_out")"
fi
ok "direction-c/go/detects-order-regression"

# --- Node ---
TC_N="$WORK/dirc-node"
setup_npm_tree "$TC_N"
corrupt_literal \
  "$ROOT_DIR/npm/src/validator/index.js" "$TC_N/npm/src/validator/index.js" \
  $'    ordered.push(name)\n  }\n  return ordered\n}' \
  $'    ordered.push(name)\n  }\n  ordered.sort()\n  return ordered\n}' \
  "direction-c-node"

dirc_node_out=$(cd "$P7" && node "$TC_N/npm/bin/trackfw" roadmap list 2>&1; true)
for marker in "[zulu" "[alfa" "[extra"; do
  if ! grep -qF -- "$marker" <<<"$dirc_node_out"; then
    fail "direction-c/node/detects-order-regression" "the corrupted tree produced no usable output at all (marker '$marker' absent too) — cannot distinguish 'order regressed' from 'node crashed/empty output'; output: $(printf '%q' "$dirc_node_out")"
  fi
done
alfa_ln=$(grep -n -F -- "[alfa" <<<"$dirc_node_out" | head -1 | cut -d: -f1)
zulu_ln=$(grep -n -F -- "[zulu" <<<"$dirc_node_out" | head -1 | cut -d: -f1)
if (( zulu_ln < alfa_ln )); then
  fail "direction-c/node/detects-order-regression" "corrupted tree still put zulu (declared 1st) before alfa (declared 2nd) — checagem vácua, output: $(printf '%q' "$dirc_node_out")"
fi
ok "direction-c/node/detects-order-regression"

# --- Python ---
TC_P="$WORK/dirc-python"
setup_py_tree "$TC_P"
corrupt_literal \
  "$ROOT_DIR/pypi/trackfw/config.py" "$TC_P/pypi/trackfw/config.py" \
  $'        ordered.append(name)\n    return ordered\n' \
  $'        ordered.append(name)\n    ordered.sort()\n    return ordered\n' \
  "direction-c-python"

dirc_python_out=$(cd "$P7" && env PYTHONPATH="$TC_P/pypi" python3 -m trackfw roadmap list 2>&1; true)
for marker in "[zulu" "[alfa" "[extra"; do
  if ! grep -qF -- "$marker" <<<"$dirc_python_out"; then
    fail "direction-c/python/detects-order-regression" "the corrupted tree produced no usable output at all (marker '$marker' absent too) — cannot distinguish 'order regressed' from 'python crashed/empty output'; output: $(printf '%q' "$dirc_python_out")"
  fi
done
alfa_ln=$(grep -n -F -- "[alfa" <<<"$dirc_python_out" | head -1 | cut -d: -f1)
zulu_ln=$(grep -n -F -- "[zulu" <<<"$dirc_python_out" | head -1 | cut -d: -f1)
if (( zulu_ln < alfa_ln )); then
  fail "direction-c/python/detects-order-regression" "corrupted tree still put zulu (declared 1st) before alfa (declared 2nd) — checagem vácua, output: $(printf '%q' "$dirc_python_out")"
fi
ok "direction-c/python/detects-order-regression"

echo "check-agent-namespace-union: all $SCENARIOS scenarios passed (AC1 x3 runtimes x3 checks, AC4 x3, AC5 x3+x3, infra-filter x3, hidden-namespace x3x4, glob-metachar x3x3, flat-untouched x3, AC12 x3, ordering x3, direction-a x3, direction-b1 x3, direction-b3 x3, direction-b4 x1, direction-b2 x2, direction-c x3)."
