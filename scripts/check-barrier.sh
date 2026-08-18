#!/usr/bin/env bash
# check-barrier.sh — E2E, non-vacuous proof of `trackfw barrier` (ML-4A,
# ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador).
#
# Builds throwaway fixtures under a temp dir, drives the three compiled/interpreted
# CLIs against them, and asserts both the positive path (a green wave truly passes)
# and the negative path (a red wave truly blocks, for the specific reason expected).
# Follows the conventions of scripts/check-gates-falsify.sh: set -euo pipefail,
# mktemp -d fixtures with a cleanup trap, "OK [scenario/name]" on success and a
# non-zero exit with a diagnostic on the first failure.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-barrier.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# $HOME isolado e sintético — nunca o real. Sem isto, o gate "validate"
# embutido em `trackfw barrier` enxerga o escopo GLOBAL de guards do usuário
# rodando o gate: desde ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-
# fora-de-projeto-e-integridade-independente-de-fiacao (ML-3A),
# git_branch_guard_script_integrity/credential_guard_script_integrity
# disparam pela EXISTÊNCIA do script em ~/.trackfw/scripts/, não mais só
# quando há fiação, então um $HOME real com o harness instalado e o script
# desatualizado adicionaria um warning que faz o texto de evidência do
# gate "validate" divergir entre CLIs cujo $HOME real difira (ou entre uma
# máquina com o harness instalado e outra sem). Mesmo precedente do
# Cenário 46 em scripts/check-gates-falsify.sh.
export HOME="$WORK/home"
mkdir -p "$HOME"

# ---------------------------------------------------------------------------
# `runGateCommand` (Go/Node/Python) shells out directly via the OS process API;
# TRACKFW_DISABLE_EXTERNAL_COMMANDS only gates the forge/discover PATH lookups
# (internal/forge/adapter.go, npm/src/forge/adapter.js, pypi/trackfw/forge/adapter.py),
# not barrier's gate execution. Unset it anyway so a caller that inherited it from
# `make test` can never make scenario 4's gate-execution proof pass vacuously.
# ---------------------------------------------------------------------------
unset TRACKFW_DISABLE_EXTERNAL_COMMANDS

# ---------------------------------------------------------------------------
# Resolve the three runtimes. GO_BIN may be passed in (absolute or relative to
# ROOT_DIR, as the Makefile does with GO_BIN=$(BUILD_DIR)/$(BINARY)); otherwise
# build a throwaway binary so the script also works standalone.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (
    cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw
  )
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-barrier: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-barrier: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Self-test seam for scripts/check-gates-falsify.sh (ML-4A ENTREGÁVEL 2).
# When BARRIER_SELFTEST_BREAK=1, scenario 1 deliberately builds Wave 2 already
# green (as if the ML-completion check were never enforced), so its own
# "Wave 2 must still be blocked" assertion fails with an explicit diagnostic.
# This is the seam check-gates-falsify.sh exercises to prove check-barrier.sh
# itself is falsifiable, without sed-ing a private copy of this script.
# ---------------------------------------------------------------------------
SELFTEST_BREAK="${BARRIER_SELFTEST_BREAK:-0}"
# Seam for check-gates-falsify.sh Cenário 19 (early-break regression on after position).
# When BARRIER_BIS_SELFTEST_BREAK=1, scenario 9 writes a fully valid fixture (no malformed
# heading), so all runtimes return exit 0 while the assertion expects exit 2 — producing the
# explicit diagnostic the falsify script asserts. This does NOT flip the assertion; it
# corrupts the fixture (same pattern as BARRIER_SELFTEST_BREAK), proving the scenario is
# non-vacuous with respect to the class of early-break bug fixed in ML-2D.
BIS_SELFTEST_BREAK="${BARRIER_BIS_SELFTEST_BREAK:-0}"

ok() { echo "OK   [$1]"; }
fail() {
  echo "FAIL [$1]: $2" >&2
  exit 1
}

# ---------------------------------------------------------------------------
# Fixture scaffolding — mirrors the string-level rules pinned in
# docs/cli-parity.md (`## trackfw barrier` → "Roadmap parsing rules") and the
# fixture builder in internal/commands/barrier_contract_test.go
# (buildBarrierRoadmap), extended to two waves.
# ---------------------------------------------------------------------------
common_dirs() {
  local dir=$1
  mkdir -p \
    "$dir/docs/roadmaps/wip" "$dir/docs/roadmaps/backlog" "$dir/docs/roadmaps/blocked" \
    "$dir/docs/roadmaps/done" "$dir/docs/roadmaps/abandoned" \
    "$dir/docs/req" "$dir/docs/adr"
}

# run_barrier RUNTIME DIR ARGS...
# Sets BARRIER_EXIT, BARRIER_STDOUT, BARRIER_STDERR as globals (bash has no
# multi-return); mirrors run_go/run_node/run_py used across the other
# scripts/check-*-parity.sh gates.
run_barrier() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/out.$$.$RANDOM" err_file="$WORK/err.$$.$RANDOM"
  set +e
  case "$runtime" in
  go) (cd "$dir" && "$GO_BIN" barrier "$@") >"$out_file" 2>"$err_file" ;;
  node) (cd "$dir" && node "$NODE_CLI" barrier "$@") >"$out_file" 2>"$err_file" ;;
  py) (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw barrier "$@") >"$out_file" 2>"$err_file" ;;
  *) echo "run_barrier: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  BARRIER_EXIT=$?
  set -e
  BARRIER_STDOUT=$(cat "$out_file")
  BARRIER_STDERR=$(cat "$err_file")
  rm -f "$out_file" "$err_file"
}

# doc_status DOC — prints the top-level "status" field of a barrier JSON document.
doc_status() {
  local doc=$1
  python3 -c "import json, sys; print(json.loads(sys.argv[1])['status'])" "$doc"
}

# check_field_json DOC NAME FIELD — prints the JSON-encoded value of one field
# (e.g. "commands") of one named check, or 'MISSING' if the check is absent.
check_field_json() {
  local doc=$1 name=$2 field=$3
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
name, field = sys.argv[2], sys.argv[3]
for c in d['checks']:
    if c['name'] == name:
        print(json.dumps(c.get(field)))
        raise SystemExit(0)
print('MISSING')
" "$doc" "$name" "$field"
}

# assert_only_this_check_blocked DOC NAME LABEL — proves the failure is
# isolated: the named check is blocked and every other check is passed.
# Without this, a scenario proves "something is red", not "this check is red"
# (the exact defect class the Wave 2 barrier run found in ML-2D).
assert_only_this_check_blocked() {
  local doc=$1 name=$2 label=$3
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
target = sys.argv[2]
found = False
for c in d['checks']:
    if c['name'] == target:
        found = True
        if c['status'] != 'blocked':
            print('target check %r is %r, want blocked' % (target, c['status']))
            raise SystemExit(1)
    elif c['status'] != 'passed':
        print('check %r is %r, want passed (isolation broken)' % (c['name'], c['status']))
        raise SystemExit(1)
if not found:
    print('check %r not present in document' % target)
    raise SystemExit(1)
" "$doc" "$name" || fail "$label" "isolation assertion failed (see stdout above)"
}

# ---------------------------------------------------------------------------
# Scenario 1 + 2 — two-wave flow, and reexecution after correction.
# ---------------------------------------------------------------------------
S1="$WORK/s1-two-wave"
common_dirs "$S1"

write_two_wave_roadmap() {
  local out=$1 w2_status=$2 w2_criteria_line=$3
  {
    echo "# Roadmap: Barrier E2E Fixture"
    echo
    echo "REQ: REQ-2026-07-29-barrier-fixture"
    echo
    echo "## Acceptance Criteria"
    echo "- [x] fixture roadmap-level criterion"
    echo
    echo "## Wave 1 — Fixture Wave One"
    echo "> Dependências: nenhuma"
    echo
    echo "### ML-1A — Fixture ML One"
    echo "**Status:** ✅"
    echo "**Critérios de aceite:**"
    echo "- [x] build passes"
    echo
    echo "## Wave 2 — Fixture Wave Two"
    echo "> Dependências: Wave 1 completa"
    echo
    echo "### ML-2A — Fixture ML Two"
    echo "**Status:** $w2_status"
    echo "**Critérios de aceite:**"
    echo "$w2_criteria_line"
  } >"$out"
}

ROADMAP1="$S1/docs/roadmaps/wip/ROADMAP-barrier-e2e.md"

if [[ "$SELFTEST_BREAK" == "1" ]]; then
  # Deliberately corrupt: Wave 2's ML is already ✅, as if the mls_complete
  # check were never enforced. The assertion below expects Wave 2 to still be
  # blocked, so this must make the script fail with an explicit diagnostic —
  # that failure IS the falsification proof consumed by check-gates-falsify.sh.
  write_two_wave_roadmap "$ROADMAP1" "✅" "- [x] build passes"
else
  write_two_wave_roadmap "$ROADMAP1" "⬜ Pendente" "- [ ] build passes"
fi

run_barrier go "$S1" ROADMAP-barrier-e2e --wave 1 --json
if [[ "$BARRIER_EXIT" -ne 0 ]]; then
  fail "barrier/two-wave-flow/wave1-passed" "expected exit 0 for Wave 1, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
fi
STATUS1=$(doc_status "$BARRIER_STDOUT")
if [[ "$STATUS1" != "passed" ]]; then
  fail "barrier/two-wave-flow/wave1-passed" "expected status=passed for Wave 1, got $STATUS1"
fi
ok "barrier/two-wave-flow/wave1-passed"

run_barrier go "$S1" ROADMAP-barrier-e2e --wave 2 --json
# No special-cased branch for BARRIER_SELFTEST_BREAK=1: it deliberately makes
# the fixture already ✅ (see write_two_wave_roadmap call above), so the very
# same assertion below — "Wave 2 must be exit 1 / status=blocked" — now fails
# on its own with its own real diagnostic. That natural failure, propagated by
# `fail`, IS the falsification proof scripts/check-gates-falsify.sh consumes;
# no separate seam-only code path is needed or desired.
if [[ "$BARRIER_EXIT" -ne 1 ]]; then
  fail "barrier/two-wave-flow/wave2-blocked" "expected exit 1 for Wave 2, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT"
fi
STATUS2=$(doc_status "$BARRIER_STDOUT")
if [[ "$STATUS2" != "blocked" ]]; then
  fail "barrier/two-wave-flow/wave2-blocked" "expected status=blocked for Wave 2, got $STATUS2"
fi
ok "barrier/two-wave-flow/wave2-blocked"

# Scenario 2 — reexecution after correction: fix Wave 2 in place and prove the
# *same* invocation now passes. Proves the barrier is not a permanent denial gate.
write_two_wave_roadmap "$ROADMAP1" "✅" "- [x] build passes"
run_barrier go "$S1" ROADMAP-barrier-e2e --wave 2 --json
if [[ "$BARRIER_EXIT" -ne 0 ]]; then
  fail "barrier/reexecution-after-fix" "expected exit 0 after correction, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT stderr: $BARRIER_STDERR"
fi
STATUS2FIXED=$(doc_status "$BARRIER_STDOUT")
if [[ "$STATUS2FIXED" != "passed" ]]; then
  fail "barrier/reexecution-after-fix" "expected status=passed after correction, got $STATUS2FIXED"
fi
ok "barrier/reexecution-after-fix"

# ---------------------------------------------------------------------------
# Scenario 3 — each of the four built-in checks blocks in isolation, and this
# holds across all three runtimes (Go, Node.js, Python) — not just Go.
# ---------------------------------------------------------------------------

# 3a — mls_complete: ML pending, evidence otherwise complete.
S3A="$WORK/s3a-mls"
common_dirs "$S3A"
cat >"$S3A/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [x] build passes
EOF
for runtime in go node py; do
  run_barrier "$runtime" "$S3A" ROADMAP-barrier-fixture --wave 1 --json
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/isolated-check/mls_complete/$runtime" "expected exit 1, got $BARRIER_EXIT"
  assert_only_this_check_blocked "$BARRIER_STDOUT" "mls_complete" "barrier/isolated-check/mls_complete/$runtime"
done
ok "barrier/isolated-check/mls_complete"

# 3b — acceptance_evidence: ML done, one criterion unmet.
S3B="$WORK/s3b-evidence"
common_dirs "$S3B"
cat >"$S3B/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
- [ ] tests pass
EOF
for runtime in go node py; do
  run_barrier "$runtime" "$S3B" ROADMAP-barrier-fixture --wave 1 --json
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/isolated-check/acceptance_evidence/$runtime" "expected exit 1, got $BARRIER_EXIT"
  assert_only_this_check_blocked "$BARRIER_STDOUT" "acceptance_evidence" "barrier/isolated-check/acceptance_evidence/$runtime"
done
ok "barrier/isolated-check/acceptance_evidence"

# 3c — gates: a declared gate command exits non-zero.
S3C="$WORK/s3c-gates"
common_dirs "$S3C"
cat >"$S3C/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

**Gates da wave:**
```bash
false
```

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF
for runtime in go node py; do
  run_barrier "$runtime" "$S3C" ROADMAP-barrier-fixture --wave 1 --json
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/isolated-check/gates/$runtime" "expected exit 1, got $BARRIER_EXIT"
  assert_only_this_check_blocked "$BARRIER_STDOUT" "gates" "barrier/isolated-check/gates/$runtime"
done
ok "barrier/isolated-check/gates"

# 3d — validate: wave/ML/gates fully green, only governance fails (no REQ link).
S3D="$WORK/s3d-validate"
common_dirs "$S3D"
cat >"$S3D/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF
for runtime in go node py; do
  run_barrier "$runtime" "$S3D" ROADMAP-barrier-fixture --wave 1 --json
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/isolated-check/validate/$runtime" "expected exit 1, got $BARRIER_EXIT"
  assert_only_this_check_blocked "$BARRIER_STDOUT" "validate" "barrier/isolated-check/validate/$runtime"
done
ok "barrier/isolated-check/validate"

# ---------------------------------------------------------------------------
# Scenario 4 — declared gates are executed; undeclared gates are never invented.
# ---------------------------------------------------------------------------

# 4a — a declared gate command really runs: it must be able to create a
# sentinel file at an absolute path (the gate runs from the fixture's cwd, not
# the trackfw repo, so a relative path here would prove nothing).
S4A="$WORK/s4a-gate-runs"
common_dirs "$S4A"
SENTINEL="$WORK/s4a-sentinel"
[[ ! -e "$SENTINEL" ]] || fail "barrier/gates/declared-gate-executes" "sentinel already existed before the run"
cat >"$S4A/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<EOF
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

**Gates da wave:**
\`\`\`bash
touch "$SENTINEL"
\`\`\`

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF
run_barrier go "$S4A" ROADMAP-barrier-fixture --wave 1 --json
[[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/gates/declared-gate-executes" "expected exit 0, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT"
[[ -e "$SENTINEL" ]] || fail "barrier/gates/declared-gate-executes" "declared gate did not run — sentinel file was not created"
ok "barrier/gates/declared-gate-executes"

# 4b — a wave with no gates block declares zero gates: commands must be [],
# and it must be the case that the mere *presence* of an executable elsewhere
# in the fixture (a shell script the barrier could accidentally pick up) is
# never invoked. This is the neutrality-of-stack proof.
S4B="$WORK/s4b-no-gates"
common_dirs "$S4B"
SENTINEL_B="$WORK/s4b-sentinel"
cat >"$S4B/would-run-if-invented.sh" <<EOF
#!/usr/bin/env bash
touch "$SENTINEL_B"
EOF
chmod +x "$S4B/would-run-if-invented.sh"
cat >"$S4B/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF
run_barrier go "$S4B" ROADMAP-barrier-fixture --wave 1 --json
[[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/gates/no-gates-block-invents-nothing" "expected exit 0, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT"
[[ ! -e "$SENTINEL_B" ]] || fail "barrier/gates/no-gates-block-invents-nothing" "barrier invented and ran a gate that was never declared"
CMDS=$(check_field_json "$BARRIER_STDOUT" "gates" "commands")
[[ "$CMDS" == "[]" ]] || fail "barrier/gates/no-gates-block-invents-nothing" "expected commands=[], got $CMDS"
ok "barrier/gates/no-gates-block-invents-nothing"

# ---------------------------------------------------------------------------
# Scenario 5 — usage errors are exit 2, never a blocked status document.
# ---------------------------------------------------------------------------
S5="$WORK/s5-usage-errors"
common_dirs "$S5"
cat >"$S5/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF

for runtime in go node py; do
  run_barrier "$runtime" "$S5" ROADMAP-does-not-exist --wave 1 --json
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/usage-error/roadmap-not-found/$runtime" "expected exit 2, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT stderr: $BARRIER_STDERR"
  WANT='trackfw barrier: roadmap "ROADMAP-does-not-exist" not found in wip/ nor done/ under docs/roadmaps'
  [[ "$BARRIER_STDERR" == "$WANT"$'\n' || "$BARRIER_STDERR" == "$WANT" ]] || fail "barrier/usage-error/roadmap-not-found/$runtime" "stderr mismatch, want [$WANT], got [$BARRIER_STDERR]"
  [[ "$BARRIER_STDOUT" != *'"status": "blocked"'* && "$BARRIER_STDOUT" != *'"status":"blocked"'* ]] || fail "barrier/usage-error/roadmap-not-found/$runtime" "usage error must never emit a blocked status document"

  run_barrier "$runtime" "$S5" ROADMAP-barrier-fixture --wave 99 --json
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/usage-error/wave-not-found/$runtime" "expected exit 2, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT stderr: $BARRIER_STDERR"
  WANT2='trackfw barrier: wave 99 not found in roadmap "ROADMAP-barrier-fixture.md"'
  [[ "$BARRIER_STDERR" == "$WANT2"$'\n' || "$BARRIER_STDERR" == "$WANT2" ]] || fail "barrier/usage-error/wave-not-found/$runtime" "stderr mismatch, want [$WANT2], got [$BARRIER_STDERR]"
  [[ "$BARRIER_STDOUT" != *'"status": "blocked"'* && "$BARRIER_STDOUT" != *'"status":"blocked"'* ]] || fail "barrier/usage-error/wave-not-found/$runtime" "usage error must never emit a blocked status document"
  ok "barrier/usage-error/$runtime"
done

# ---------------------------------------------------------------------------
# Scenario 6 — the three runtimes agree byte-for-byte over the same fixture.
# ML-2D reproved the previous parity run over exactly this class of drift;
# this reruns the same class of assertion for the E2E flow (not just the
# eight ML-1A contract scenarios, which unmarshal into structs and are
# therefore blind to raw key-order drift).
# ---------------------------------------------------------------------------
S6="$WORK/s6-parity"
common_dirs "$S6"
cat >"$S6/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave
> Dependências: nenhuma

**Gates da wave:**
```bash
true
```

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF

normalize_barrier_json() {
  # Reparses and redumps preserving key order (Python 3.7+ dicts preserve
  # insertion order; we deliberately do NOT sort_keys — the contract pins key
  # order, not just key presence, and sorting would hide exactly the class of
  # drift ML-2D was created to catch). This also normalizes whitespace, which
  # the contract does NOT pin (Node pretty-prints with indent=2; Go/Python
  # emit compact JSON), so only real shape/order/content differences survive.
  python3 -c "
import json, sys
d = json.loads(sys.stdin.read())
d['started_at'] = 'TS'
d['finished_at'] = 'TS'
json.dump(d, sys.stdout, indent=2, ensure_ascii=False)
"
}

run_barrier go "$S6" ROADMAP-barrier-fixture --wave 1 --json
[[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/parity/go" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
echo "$BARRIER_STDOUT" | normalize_barrier_json >"$WORK/go.norm.json"

run_barrier node "$S6" ROADMAP-barrier-fixture --wave 1 --json
[[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/parity/node" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
echo "$BARRIER_STDOUT" | normalize_barrier_json >"$WORK/node.norm.json"

run_barrier py "$S6" ROADMAP-barrier-fixture --wave 1 --json
[[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/parity/py" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
echo "$BARRIER_STDOUT" | normalize_barrier_json >"$WORK/py.norm.json"

if ! diff -u "$WORK/go.norm.json" "$WORK/node.norm.json" >"$WORK/diff-go-node.txt"; then
  fail "barrier/parity/go-vs-node" "JSON diverges between Go and Node.js runtimes:
$(cat "$WORK/diff-go-node.txt")"
fi
if ! diff -u "$WORK/go.norm.json" "$WORK/py.norm.json" >"$WORK/diff-go-py.txt"; then
  fail "barrier/parity/go-vs-python" "JSON diverges between Go and Python runtimes:
$(cat "$WORK/diff-go-py.txt")"
fi
ok "barrier/parity/three-runtimes-identical"

# ---------------------------------------------------------------------------
# Scenario 7 — no specialist asset authorizes Git operations; architect.md
# carries the explicit Git-authority protocol. Static analysis of the
# rendered assets, not a fixture run.
# ---------------------------------------------------------------------------
AGENTS_DIR="$ROOT_DIR/internal/integrations/assets/agents"
SPECIALISTS=(backend code-quality data dba frontend iac infra qa security tooling ux)

for name in "${SPECIALISTS[@]}"; do
  f="$AGENTS_DIR/$name.md"
  [[ -f "$f" ]] || fail "barrier/git-authority/$name" "asset not found: $f"
  # An *instruction* to run a Git operation looks like a fenced/backticked
  # literal git subcommand (`git commit`, `git push -u ...`, `git checkout -b`,
  # `git branch`, `git merge`, `git rebase`). Discussing the words "commit" or
  # "push" in prose (e.g. "hand back for audit and commit") is not an
  # authorization — only a backtick-quoted `git <verb>` invocation is.
  if grep -niE '`git[[:space:]]+(commit|push|checkout|branch|merge|rebase)' "$f"; then
    fail "barrier/git-authority/$name" "asset authorizes a Git operation: $f"
  fi
done
ok "barrier/git-authority/specialists-no-git-instruction"

ARCHITECT="$AGENTS_DIR/architect.md"
[[ -f "$ARCHITECT" ]] || fail "barrier/git-authority/architect" "asset not found: $ARCHITECT"
if ! grep -q 'Git authority' "$ARCHITECT"; then
  fail "barrier/git-authority/architect" "architect.md is missing the explicit Git-authority protocol section"
fi
if ! grep -qE '`git checkout -b`' "$ARCHITECT"; then
  fail "barrier/git-authority/architect" "architect.md does not document branch creation as its own responsibility"
fi
ok "barrier/git-authority/architect-has-protocol"

# ---------------------------------------------------------------------------
# Scenario 8 — malformed heading BEFORE the target wave (third pinned message).
# All three runtimes must detect the malformed heading in the pre-pass and abort
# the entire document before even resolving the requested label.
# Vacuity guard: assert stderr is non-empty before comparing bytes — three empty
# stderrs would agree trivially and prove nothing.
# ---------------------------------------------------------------------------
S8="$WORK/s8-malformed-before"
common_dirs "$S8"
cat >"$S8/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Before Fixture

## Wave X — Bad Heading

### ML-XA — Bad ML
**Status:** ✅
**Critérios de aceite:**
- [x] done

## Wave 1 — Fixture Wave

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
# ## Wave X is at line 3 in the file above.
WANT8='trackfw barrier: malformed wave heading at line 3: "X" is not a valid wave label'
for runtime in go node py; do
  run_barrier "$runtime" "$S8" ROADMAP-barrier-fixture --wave 1
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "expected exit 2, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "stderr is empty — vacuity guard failed"
  [[ "$BARRIER_STDERR" == "$WANT8"$'\n' || "$BARRIER_STDERR" == "$WANT8" ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "stderr mismatch, want [$WANT8], got [$BARRIER_STDERR]"
  [[ "$BARRIER_STDOUT" != *'"status": "blocked"'* && "$BARRIER_STDOUT" != *'"status":"blocked"'* ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "usage error must never emit a blocked status document"
  ok "barrier/wave-label/malformed-before-target/$runtime"
done

# ---------------------------------------------------------------------------
# Scenario 9 — malformed heading AFTER the target wave (early-break regression).
# Both heading positions together are non-vacuous with respect to the early-break
# bug class (cli-parity.md §detection-is-a-full-pre-pass, pinned): a pre-pass
# that stops at the target wave label will miss this heading and exit 0 instead
# of exit 2. Without testing this position, the scenario is vacuous.
#
# BARRIER_BIS_SELFTEST_BREAK seam: when active, the fixture is written without the
# malformed heading (a fully valid document), so all runtimes return exit 0 while
# the assertion expects exit 2 — producing the explicit "expected exit 2 ... got 0"
# diagnostic that check-gates-falsify.sh Cenário 19 asserts. The assertion is never
# changed; only the fixture data changes — same pattern as BARRIER_SELFTEST_BREAK.
# ---------------------------------------------------------------------------
S9="$WORK/s9-malformed-after"
common_dirs "$S9"
ROADMAP9="$S9/docs/roadmaps/wip/ROADMAP-barrier-fixture.md"

if [[ "$BIS_SELFTEST_BREAK" == "1" ]]; then
  # Seam active: omit the malformed heading so runtimes return exit 0.
  # REQ + Acceptance Criteria included so the validate check also passes
  # (exit 0 = all four checks pass), making the assertion for exit 2 fail.
  # → falsification proof for Cenário 19.
  cat >"$ROADMAP9" <<'EOF'
# Roadmap: Barrier After Fixture (seam: no malformed heading)

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done

## Wave 2 — Valid Second Wave

### ML-2A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
else
  # Normal: Wave X heading at line 10 (after the target Wave 1 block).
  cat >"$ROADMAP9" <<'EOF'
# Roadmap: Barrier After Fixture

## Wave 1 — Fixture Wave

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done

## Wave X — Bad Heading After Target

### ML-XA — Bad ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
fi
# ## Wave X is at line 10 in the normal fixture above.
WANT9='trackfw barrier: malformed wave heading at line 10: "X" is not a valid wave label'
for runtime in go node py; do
  run_barrier "$runtime" "$S9" ROADMAP-barrier-fixture --wave 1
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "expected exit 2 for after-position malformed heading, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "stderr is empty — vacuity guard failed"
  [[ "$BARRIER_STDERR" == "$WANT9"$'\n' || "$BARRIER_STDERR" == "$WANT9" ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "stderr mismatch, want [$WANT9], got [$BARRIER_STDERR]"
  [[ "$BARRIER_STDOUT" != *'"status": "blocked"'* && "$BARRIER_STDOUT" != *'"status":"blocked"'* ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "usage error must never emit a blocked status document"
  ok "barrier/wave-label/malformed-after-target/$runtime"
done

# ---------------------------------------------------------------------------
# Scenario 10 — --wave 2-bis resolves Wave 2-bis; --wave 2 resolves Wave 2 and
# does NOT match Wave 2-bis (identity-distinct, grammar: no prefix match).
# Vacuity guard: assert the `wave` field in the JSON document equals the expected
# label — exit 0 alone is insufficient if routing resolves the wrong wave.
# ---------------------------------------------------------------------------
S10="$WORK/s10-bis-identity"
common_dirs "$S10"
cat >"$S10/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Bis-Identity Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 1 — Fixture Wave One

### ML-1A — Fixture ML One
**Status:** ✅
**Critérios de aceite:**
- [x] done

## Wave 2 — Fixture Wave Two

### ML-2A — Fixture ML Two-A
**Status:** ✅
**Critérios de aceite:**
- [x] done

## Wave 2-bis — Fixture Wave Two-Bis

### ML-2Z — Fixture ML Two-Z
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
get_wave_field() {
  python3 -c "import json,sys; print(json.loads(sys.argv[1])['wave'])" "$1"
}
for runtime in go node py; do
  # --wave 2-bis must resolve Wave 2-bis (wave field = "2-bis", not "2")
  run_barrier "$runtime" "$S10" ROADMAP-barrier-fixture --wave 2-bis --json
  [[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/wave-label/bis-identity/$runtime/bis-resolves" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GOT_WAVE=$(get_wave_field "$BARRIER_STDOUT")
  [[ "$GOT_WAVE" == "2-bis" ]] || fail "barrier/wave-label/bis-identity/$runtime/bis-resolves" "expected wave=2-bis, got $GOT_WAVE"
  ok "barrier/wave-label/bis-identity/$runtime/bis-resolves"

  # --wave 2 must resolve Wave 2, not Wave 2-bis (wave field = "2")
  run_barrier "$runtime" "$S10" ROADMAP-barrier-fixture --wave 2 --json
  [[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/wave-label/bis-identity/$runtime/2-not-bis" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GOT_WAVE=$(get_wave_field "$BARRIER_STDOUT")
  [[ "$GOT_WAVE" == "2" ]] || fail "barrier/wave-label/bis-identity/$runtime/2-not-bis" "expected wave=2 (not 2-bis), got $GOT_WAVE"
  ok "barrier/wave-label/bis-identity/$runtime/2-not-bis"
done

# ---------------------------------------------------------------------------
# Scenario 11 — ## Wave 0 is a malformed heading: integer part must be ≥1.
# All three runtimes must detect it in the pre-pass (same code path as "X").
# ---------------------------------------------------------------------------
S11="$WORK/s11-wave-zero"
common_dirs "$S11"
cat >"$S11/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Wave-Zero Fixture

## Wave 0 — Bad Zero Wave

### ML-0A — Bad ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
# ## Wave 0 is at line 3.
WANT11='trackfw barrier: malformed wave heading at line 3: "0" is not a valid wave label'
for runtime in go node py; do
  run_barrier "$runtime" "$S11" ROADMAP-barrier-fixture --wave 1
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/wave-label/wave-zero/$runtime" "expected exit 2, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/wave-zero/$runtime" "stderr is empty — vacuity guard failed"
  [[ "$BARRIER_STDERR" == "$WANT11"$'\n' || "$BARRIER_STDERR" == "$WANT11" ]] || fail "barrier/wave-label/wave-zero/$runtime" "stderr mismatch, want [$WANT11], got [$BARRIER_STDERR]"
  ok "barrier/wave-label/wave-zero/$runtime"
done

# ---------------------------------------------------------------------------
# Scenario 12 — --wave 2-BIS is an invalid argument (fourth pinned exit-2
# message, cli-parity.md §four-pinned-exit-2-messages). Stderr must be
# non-empty and byte-identical across all three runtimes.
# ---------------------------------------------------------------------------
S12="$WORK/s12-invalid-arg"
common_dirs "$S12"
cat >"$S12/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Invalid-Arg Fixture

## Wave 1 — Fixture Wave

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
WANT12='trackfw barrier: invalid --wave "2-BIS" — not a valid wave label'
for runtime in go node py; do
  run_barrier "$runtime" "$S12" ROADMAP-barrier-fixture --wave 2-BIS
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/wave-label/invalid-arg/$runtime" "expected exit 2, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/invalid-arg/$runtime" "stderr is empty — vacuity guard failed"
  [[ "$BARRIER_STDERR" == "$WANT12"$'\n' || "$BARRIER_STDERR" == "$WANT12" ]] || fail "barrier/wave-label/invalid-arg/$runtime" "stderr mismatch, want [$WANT12], got [$BARRIER_STDERR]"
  ok "barrier/wave-label/invalid-arg/$runtime"
done
# Byte-identical parity across runtimes for the fourth exit-2 message.
run_barrier go "$S12" ROADMAP-barrier-fixture --wave 2-BIS
GO_STDERR4="$BARRIER_STDERR"
run_barrier node "$S12" ROADMAP-barrier-fixture --wave 2-BIS
NODE_STDERR4="$BARRIER_STDERR"
run_barrier py "$S12" ROADMAP-barrier-fixture --wave 2-BIS
PY_STDERR4="$BARRIER_STDERR"
[[ "$GO_STDERR4" == "$NODE_STDERR4" ]] || fail "barrier/wave-label/invalid-arg/parity/go-vs-node" "stderr diverges: go=[$GO_STDERR4] node=[$NODE_STDERR4]"
[[ "$GO_STDERR4" == "$PY_STDERR4" ]] || fail "barrier/wave-label/invalid-arg/parity/go-vs-py" "stderr diverges: go=[$GO_STDERR4] py=[$PY_STDERR4]"
ok "barrier/wave-label/invalid-arg/parity/three-runtimes-identical"

echo
echo "All check-barrier.sh scenarios passed."
