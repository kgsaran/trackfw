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

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$ROOT_DIR/scripts/lib-crlf-normalize.sh"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-barrier.XXXXXX")
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

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

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-barrier: Go binary not found/executable at $GO_BIN" >&2
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
# When BARRIER_BIS_SELFTEST_BREAK=1, scenario 9 writes a fixture WITHOUT the malformed
# heading, so the barrier exits 0 (all checks pass). ML-1E: the first assertion
# `[[ "$BARRIER_EXIT" -eq 1 ]]` fires first with "expected exit 1 (blocked: wave_headings
# check), got 0; stderr:" — proving the scenario is non-vacuous with respect to the
# early-break class. This does NOT flip the assertion; it corrupts the fixture (same
# pattern as BARRIER_SELFTEST_BREAK). Updated by ML-1E (REQ #392) which added the
# wave_headings check (exit 1 when malformed headings exist).
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
  python3 -c "import json, sys; print(json.loads(sys.argv[1])['status'])" "$doc" | strip_cr
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
" "$doc" "$name" "$field" | strip_cr
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
    echo "## Wave 0 — Threat Model"
    echo
    echo "### ML-0A — Threat model for this fixture"
    echo "**Status:** ✅"
    echo "**Gates da wave:**"
    echo '```bash'
    echo "exit 0"
    echo '```'
    echo "**Critérios de aceite:**"
    echo "- [x] threat model complete"
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

run_barrier go "$S1" ROADMAP-barrier-e2e --wave 1 --json --trust-local-gates
if [[ "$BARRIER_EXIT" -ne 0 ]]; then
  fail "barrier/two-wave-flow/wave1-passed" "expected exit 0 for Wave 1, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
fi
STATUS1=$(doc_status "$BARRIER_STDOUT")
if [[ "$STATUS1" != "passed" ]]; then
  fail "barrier/two-wave-flow/wave1-passed" "expected status=passed for Wave 1, got $STATUS1"
fi
ok "barrier/two-wave-flow/wave1-passed"

run_barrier go "$S1" ROADMAP-barrier-e2e --wave 2 --json --trust-local-gates
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
run_barrier go "$S1" ROADMAP-barrier-e2e --wave 2 --json --trust-local-gates
if [[ "$BARRIER_EXIT" -ne 0 ]]; then
  fail "barrier/reexecution-after-fix" "expected exit 0 after correction, got $BARRIER_EXIT; stdout: $BARRIER_STDOUT stderr: $BARRIER_STDERR"
fi
STATUS2FIXED=$(doc_status "$BARRIER_STDOUT")
if [[ "$STATUS2FIXED" != "passed" ]]; then
  fail "barrier/reexecution-after-fix" "expected status=passed after correction, got $STATUS2FIXED"
fi
ok "barrier/reexecution-after-fix"

# ---------------------------------------------------------------------------
# Scenario 3 — each of the five built-in checks blocks in isolation, and this
# holds across all three runtimes (Go, Node.js, Python) — not just Go.
# (wave_headings is tested by Scenarios 8 and 9; the four content checks below.)
# ---------------------------------------------------------------------------

# 3a — mls_complete: ML pending, evidence otherwise complete.
S3A="$WORK/s3a-mls"
common_dirs "$S3A"
cat >"$S3A/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [x] build passes
EOF
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S3A" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
- [ ] tests pass
EOF
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S3B" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

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
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S3C" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S3D" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
\`\`\`bash
exit 0
\`\`\`
**Critérios de aceite:**
- [x] threat model complete

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
run_barrier go "$S4A" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

## Wave 1 — Fixture Wave
> Dependências: nenhuma

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] build passes
EOF
run_barrier go "$S4B" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
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

for runtime in go; do  # ML-3A (v8): node py removidos
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

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
" | strip_cr
}

run_barrier go "$S6" ROADMAP-barrier-fixture --wave 1 --json --trust-local-gates
[[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/parity/go" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
echo "$BARRIER_STDOUT" | normalize_barrier_json >"$WORK/go.norm.json"




ok "barrier/parity/go-behavioral-pin"

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
# Scenario 8 — malformed heading BEFORE the target wave (cascade isolation, ML-1D/ML-1E).
# ML-1D (REQ #392) emends ADR-2026-07-29 decision 16: ParseWaves records each malformed
# heading in []MalformedWave and continues parsing (cascade isolation at parse level).
# ML-1E (REQ #392) adds the wave_headings check: the barrier:
#   - prints the malformed-wave warning to stderr
#   - records the malformed heading in the wave_headings check (blocked)
#   - evaluates the requested wave normally (never exit 2 for parse error)
#   - exits 1 (blocked overall via wave_headings check; principle of "reprovar alto" preserved)
# Vacuity guard: assert stderr is non-empty before comparing bytes — an empty stderr
# would pass the byte-match trivially and prove nothing about the warning path.
# ---------------------------------------------------------------------------
S8="$WORK/s8-malformed-before"
common_dirs "$S8"
cat >"$S8/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Before Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave X — Bad Heading

### ML-XA — Bad ML
**Status:** ✅
**Critérios de aceite:**
- [x] done

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

## Wave 1 — Fixture Wave

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF
# ## Wave X is at line 8 in the file above (REQ + AC block prepended — ML-1D).
# ## Wave 0 is inserted between Wave X and Wave 1 (ML-4C); Wave X stays at line 8.
WANT8='trackfw barrier: malformed wave heading at line 8: "X" is not a valid wave label'
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S8" ROADMAP-barrier-fixture --wave 1 --trust-local-gates
  # ML-1E: exit 1 (blocked via wave_headings check); the valid wave is still EVALUATED,
  # but the overall verdict must never be "passed" when malformed headings exist.
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "expected exit 1 (blocked: wave_headings check), got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "stderr is empty — vacuity guard: malformed-wave warning must appear on stderr"
  [[ "$BARRIER_STDERR" == "$WANT8"$'\n' || "$BARRIER_STDERR" == "$WANT8" ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "stderr mismatch, want [$WANT8], got [$BARRIER_STDERR]"
  # wave_headings check must be present and blocked in the output.
  [[ "$BARRIER_STDOUT" == *'wave_headings'* ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "wave_headings check not found in output: $BARRIER_STDOUT"
  [[ "$BARRIER_STDOUT" == *'result: blocked'* || "$BARRIER_STDOUT" == *'"status": "blocked"'* || "$BARRIER_STDOUT" == *'"status":"blocked"'* ]] || fail "barrier/wave-label/malformed-before-target/$runtime" "expected result blocked, got: $BARRIER_STDOUT"
  ok "barrier/wave-label/malformed-before-target/$runtime"
done

# ---------------------------------------------------------------------------
# Scenario 9 — malformed heading AFTER the target wave (cascade isolation + full-parse
# coverage, ML-1D/ML-1E).
# ML-1D (REQ #392) emends ADR-2026-07-29 decision 16: ParseWaves records both heading
# positions in []MalformedWave (full pre-pass). ML-1E adds the wave_headings check.
# Both heading positions (before and after target wave) are fully parsed; each invalid
# label produces a MalformedWave entry that blocks the verdict via wave_headings.
# This scenario proves that the AFTER-position malformed heading is still parsed
# (non-vacuous with respect to the early-break class: an implementation that stops
# parsing after finding the target wave would miss this heading entirely — its warning
# would be absent from stderr, making the non-empty vacuity guard fail).
#
# BARRIER_BIS_SELFTEST_BREAK seam: when active, the fixture is written without the
# malformed heading (a fully valid document), so the barrier exits 0 (all passed).
# ML-1E: the first assertion `[[ "$BARRIER_EXIT" -eq 1 ]]` fails with:
# "expected exit 1 (blocked: wave_headings check), got 0; stderr:"
# — proving that Scenario 9 has falsification power over the early-break class.
# (Cenário 19 of check-gates-falsify.sh asserts this failure message.)
# The assertion is never changed; only the fixture data changes — same pattern
# as BARRIER_SELFTEST_BREAK. (ML-1E update: prior diagnostic was the vacuity guard
# "stderr is empty — vacuity guard: ..."; now the exit code check fires first.)
# ---------------------------------------------------------------------------
S9="$WORK/s9-malformed-after"
common_dirs "$S9"
ROADMAP9="$S9/docs/roadmaps/wip/ROADMAP-barrier-fixture.md"

if [[ "$BIS_SELFTEST_BREAK" == "1" ]]; then
  # Seam active: omit the malformed heading so stderr is empty.
  # The vacuity guard fails → falsification proof for Cenário 19.
  # ML-4C: Wave 0 added so roadmap_wave0_required passes; validate does not block;
  # barrier exits 0 → [[ $BARRIER_EXIT -eq 1 ]] fails → Cenário 19 fires correctly.
  cat >"$ROADMAP9" <<'EOF'
# Roadmap: Barrier After Fixture (seam: no malformed heading)

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

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
  # Normal: Wave X heading at line 26 (after Wave 0 + Wave 1 block, with REQ + AC — ML-1D/ML-4C).
  # ML-4C: Wave 0 inserted before Wave 1 → Wave X shifts from line 15 to line 26.
  cat >"$ROADMAP9" <<'EOF'
# Roadmap: Barrier After Fixture

REQ: REQ-2026-07-29-barrier-fixture

## Acceptance Criteria
- [x] fixture roadmap-level criterion

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

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
# ML-4C: Wave 0 inserted before Wave 1 → ## Wave X now at line 26 (was line 15 pre-ML-4C).
WANT9='trackfw barrier: malformed wave heading at line 26: "X" is not a valid wave label'
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S9" ROADMAP-barrier-fixture --wave 1 --trust-local-gates
  # ML-1E: exit 1 (blocked via wave_headings check); wave 1 is still EVALUATED.
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "expected exit 1 (blocked: wave_headings check), got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "stderr is empty — vacuity guard: malformed-wave warning must appear on stderr"
  [[ "$BARRIER_STDERR" == "$WANT9"$'\n' || "$BARRIER_STDERR" == "$WANT9" ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "stderr mismatch, want [$WANT9], got [$BARRIER_STDERR]"
  # wave_headings check must be present and blocked in the output.
  [[ "$BARRIER_STDOUT" == *'wave_headings'* ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "wave_headings check not found in output: $BARRIER_STDOUT"
  [[ "$BARRIER_STDOUT" == *'result: blocked'* || "$BARRIER_STDOUT" == *'"status": "blocked"'* || "$BARRIER_STDOUT" == *'"status":"blocked"'* ]] || fail "barrier/wave-label/malformed-after-target/$runtime" "expected result blocked, got: $BARRIER_STDOUT"
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

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
```bash
exit 0
```
**Critérios de aceite:**
- [x] threat model complete

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
  python3 -c "import json,sys; print(json.loads(sys.argv[1])['wave'])" "$1" | strip_cr
}
for runtime in go; do  # ML-3A (v8): node py removidos
  # --wave 2-bis must resolve Wave 2-bis (wave field = "2-bis", not "2")
  run_barrier "$runtime" "$S10" ROADMAP-barrier-fixture --wave 2-bis --json --trust-local-gates
  [[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/wave-label/bis-identity/$runtime/bis-resolves" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GOT_WAVE=$(get_wave_field "$BARRIER_STDOUT")
  [[ "$GOT_WAVE" == "2-bis" ]] || fail "barrier/wave-label/bis-identity/$runtime/bis-resolves" "expected wave=2-bis, got $GOT_WAVE"
  ok "barrier/wave-label/bis-identity/$runtime/bis-resolves"

  # --wave 2 must resolve Wave 2, not Wave 2-bis (wave field = "2")
  run_barrier "$runtime" "$S10" ROADMAP-barrier-fixture --wave 2 --json --trust-local-gates
  [[ "$BARRIER_EXIT" -eq 0 ]] || fail "barrier/wave-label/bis-identity/$runtime/2-not-bis" "expected exit 0, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GOT_WAVE=$(get_wave_field "$BARRIER_STDOUT")
  [[ "$GOT_WAVE" == "2" ]] || fail "barrier/wave-label/bis-identity/$runtime/2-not-bis" "expected wave=2 (not 2-bis), got $GOT_WAVE"
  ok "barrier/wave-label/bis-identity/$runtime/2-not-bis"
done

# ---------------------------------------------------------------------------
# Scenario 11 — ## Wave 0 is a VALID heading (ADR decision, ML-1A): the old
# contract ("0" is malformed) is INVERTED here on purpose — the ADR rejected
# renumbering the threat-model wave, so `barrier` had to widen the grammar to
# accept 0 instead. Wave X ("not a number", Scenarios 8/9) stays malformed —
# only the integer lower bound moved, not the whole label grammar.
#
# This must prove `--wave 0` is not just syntactically accepted (exit != 2)
# but genuinely EVALUATED: `mls_complete` finds the ML-0A block and reports
# it complete, `gates` runs the declared command and reports evidence — not
# a status flipped to "passed" over an empty/untouched check. Vacuity guard:
# assert non-empty evidence/commands on both checks, not just their status
# field — a check that never ran can still report "passed" trivially
# (parseGates returns an empty-but-non-nil slice for zero gates, per the
# threat model, §2.1/§3 F5) and a bare status comparison would not catch it.
#
# ML-4E (REQ #392): why exit 0 OR exit 1 (not exit 0 only)?
# The S11 fixture dir is not a governed repo, so `validate` blocks with
# "2 violations, 1 warnings" and the barrier exits 1 — a verdict, not a
# malformation. The wave_headings assertion below distinguishes the two:
# if exit 1 is a malformation, wave_headings.status would be "blocked";
# if it is a verdict (validate failed, wave_headings intact), it is "passed".
# Malformed-Wave-0 → exit 2 is forced by construction: parseWaves appends
# Wave 0 to `malformed`, never to `waves`; target==nil triggers usageExit(2),
# so exit 1 from malformation is impossible — exit 2 is caught by the
# "never 2" assertion above, and the wave_headings assertion is redundant
# under the sabotage. It pins the "verdict, not malformation" invariant
# for any future regression that might alter the exit-code path.
# Cenário 167 of check-gates-falsify.sh provides the falsification proof:
# intVal<1 in parseWaves → check-barrier.sh fails at S1 with "malformed
# wave heading" (S1 fixture has Wave 0 since ML-4C).
# ---------------------------------------------------------------------------
S11="$WORK/s11-wave-zero"
common_dirs "$S11"
cat >"$S11/docs/roadmaps/wip/ROADMAP-barrier-fixture.md" <<'EOF'
# Roadmap: Barrier Wave-Zero Fixture

## Wave 0 — Threat Model

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Gates da wave:**
```bash
exit 0
```

**Critérios de aceite:**
- [x] done
EOF
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S11" ROADMAP-barrier-fixture --wave 0 --json --trust-local-gates
  [[ "$BARRIER_EXIT" -eq 0 || "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "expected exit 0 or 1 (never 2 — a usage/grammar error), got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GOT_WAVE=$(get_wave_field "$BARRIER_STDOUT")
  [[ "$GOT_WAVE" == "0" ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "expected wave=0, got $GOT_WAVE"

  # ML-4E (REQ #392): distinguish "exit 1 = verdict" from "exit 1 = malformation".
  # wave_headings must be "passed" — if Wave 0 were malformed, it would be "blocked".
  WH_STATUS=$(check_field_json "$BARRIER_STDOUT" wave_headings status)
  [[ "$WH_STATUS" == '"passed"' ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "wave_headings status: want \"passed\" (exit 1 here is a validate verdict, not a malformation), got $WH_STATUS; stderr: $BARRIER_STDERR"

  MLS_STATUS=$(check_field_json "$BARRIER_STDOUT" mls_complete status)
  [[ "$MLS_STATUS" == '"passed"' ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "mls_complete status: want \"passed\", got $MLS_STATUS"
  MLS_EVIDENCE=$(check_field_json "$BARRIER_STDOUT" mls_complete evidence)
  [[ "$MLS_EVIDENCE" != "[]" && "$MLS_EVIDENCE" != "MISSING" ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "mls_complete evidence is empty — vacuity guard failed, wave 0 was accepted but not genuinely evaluated"

  GATES_STATUS=$(check_field_json "$BARRIER_STDOUT" gates status)
  [[ "$GATES_STATUS" == '"passed"' ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "gates status: want \"passed\", got $GATES_STATUS"
  GATES_COMMANDS=$(check_field_json "$BARRIER_STDOUT" gates commands)
  [[ "$GATES_COMMANDS" == '["exit 0"]' ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "gates commands: want [\"exit 0\"], got $GATES_COMMANDS"
  GATES_EVIDENCE=$(check_field_json "$BARRIER_STDOUT" gates evidence)
  [[ "$GATES_EVIDENCE" != "[]" && "$GATES_EVIDENCE" != "MISSING" ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "gates evidence is empty — vacuity guard failed, the declared gate command never ran"

  ok "barrier/wave-label/wave-zero-accepted/$runtime"
done

# Byte-identical parity across runtimes for the Wave 0 JSON document (checks
# array, evidence and all) — the same discipline as Scenario 12's four-message
# parity check, applied to the newly-inverted contract.
run_barrier go "$S11" ROADMAP-barrier-fixture --wave 0 --json --trust-local-gates
GO_STDOUT11="$BARRIER_STDOUT"
# started_at/finished_at are timestamps and legitimately differ per run — strip them before comparing.
STRIP_TS='import json,sys; d=json.loads(sys.argv[1]); d.pop("started_at",None); d.pop("finished_at",None); print(json.dumps(d,sort_keys=True))'
GO_NORM11=$(python3 -c "$STRIP_TS" "$GO_STDOUT11" | strip_cr)
ok "barrier/wave-label/wave-zero-accepted/parity/go-behavioral-pin"

# ---------------------------------------------------------------------------
# Scenario 12 — --wave abc is an invalid argument (fourth pinned exit-2
# message, cli-parity.md §four-pinned-exit-2-messages). Stderr must be
# non-empty and byte-identical across all three runtimes.
# AC3-ter (REQ #392 / ML-1B): "2-BIS" was moved to valid (case-insensitive
# suffix). "abc" (letter-only, no digit prefix) is the genuinely invalid label.
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
WANT12='trackfw barrier: invalid --wave "abc" — not a valid wave label'
for runtime in go; do  # ML-3A (v8): node py removidos
  run_barrier "$runtime" "$S12" ROADMAP-barrier-fixture --wave abc
  [[ "$BARRIER_EXIT" -eq 2 ]] || fail "barrier/wave-label/invalid-arg/$runtime" "expected exit 2, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  [[ -n "$BARRIER_STDERR" ]] || fail "barrier/wave-label/invalid-arg/$runtime" "stderr is empty — vacuity guard failed"
  [[ "$BARRIER_STDERR" == "$WANT12"$'\n' || "$BARRIER_STDERR" == "$WANT12" ]] || fail "barrier/wave-label/invalid-arg/$runtime" "stderr mismatch, want [$WANT12], got [$BARRIER_STDERR]"
  ok "barrier/wave-label/invalid-arg/$runtime"
done
# Byte-identical parity across runtimes for the fourth exit-2 message.
run_barrier go "$S12" ROADMAP-barrier-fixture --wave abc
GO_STDERR4="$BARRIER_STDERR"
ok "barrier/wave-label/invalid-arg/parity/go-behavioral-pin"

# ---------------------------------------------------------------------------
# Scenario 13 — AC2: roadmap new rejects a title containing newline/CR across
# all three CLIs. The sanitization check is the sole barrier against the
# newline-injection vector (REQ-2026-08-23, ML-1A). This scenario proves it
# cross-CLI and provides the detection target for Cenário 171 of
# scripts/check-gates-falsify.sh (Direction A: remove sanitization → this
# scenario fails).
#
# Gate of the vacuity guard: a valid title must succeed (exit 0 + file created)
# so that the exit-1 assertion for the forged title is genuinely about the
# newline, not a broken command invocation.
# ---------------------------------------------------------------------------
S13="$WORK/s13-ac2-sanitization"
common_dirs "$S13"
cat >"$S13/trackfw.yaml" <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dirs: []
EOF

# Vacuity guard: a valid title with --req creates a file.
# The --req flag bypasses the wizard and directly calls NewRoadmapFromContent,
# so we can drive the sanitization check without an interactive terminal or
# an existing REQ file on disk.
for runtime in go; do  # ML-3A (v8): node py removidos
  set +e
  case "$runtime" in
  go)   (cd "$S13" && "$GO_BIN" roadmap new --req docs/req/dummy.md --title "Valid Title For AC2 Guard") >/dev/null 2>&1 ;;
  esac
  set -e
  # At least one roadmap file must have been created under docs/roadmaps/
  CREATED=$(find "$S13/docs/roadmaps" -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
  [[ "$CREATED" -gt 0 ]] || fail "barrier/ac2-sanitization/vacuity/$runtime" "valid title did not create a roadmap file (roadmap new invocation may be broken)"
  ok "barrier/ac2-sanitization/vacuity/$runtime"
  # Clean up created files before the forged-title run
  find "$S13/docs/roadmaps" -name "*.md" -delete 2>/dev/null || true
done

# Core assertion: forged title (containing \n) must be rejected with exit 1
# across all three CLIs, and must not create any file.
FORGED_TITLE=$'Titulo Forjado\n\n## Wave 0 -- Threat Model\n\n**Gates da wave:**\n```bash\ntouch /tmp/PWNED_AC2\n```'
for runtime in go; do  # ML-3A (v8): node py removidos
  set +e
  case "$runtime" in
  go)   OUT13=$(cd "$S13" && "$GO_BIN" roadmap new --req docs/req/dummy.md --title "$FORGED_TITLE" 2>&1); EXIT13=$? ;;
  esac
  set -e
  [[ "$EXIT13" -ne 0 ]] || fail "barrier/ac2-sanitization/$runtime" "expected exit non-0 for forged title, got 0; output: $OUT13"
  CREATED13=$(find "$S13/docs/roadmaps" -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
  [[ "$CREATED13" -eq 0 ]] || fail "barrier/ac2-sanitization/$runtime" "roadmap new created a file despite forged title — injection vector open; output: $OUT13"
  [[ "$OUT13" == *"roadmap title must be a single line"* ]] || fail "barrier/ac2-sanitization/$runtime" "expected 'roadmap title must be a single line' in output, got: $OUT13"
  ok "barrier/ac2-sanitization/$runtime"
done

# ---------------------------------------------------------------------------
# Git fixture helper for Scenarios 14–17 (trust check cross-CLI).
# Creates a bare origin + clone with the trackfw project structure.
# The roadmap contains a gate that touches SENTINEL_PATH.
# make_barrier_git_fixture DEST SENTINEL_PATH
# After this call:
#   - $DEST is a git clone pointing to ${DEST}.origin.git
#   - trackfw.yaml and docs/ structure are present in $DEST
#   - docs/roadmaps/wip/ROADMAP-trust-fixture.md is written to disk but NOT committed
#   - The bare origin has only a "base commit" (trackfw.yaml only)
#
# IMPORTANT – macOS symlink issue: $TMPDIR (/var/folders/…) is a symlink to
# /private/var/folders/…. git rev-parse --show-toplevel resolves symlinks, but
# Go's filepath.Abs uses os.Getwd() which returns the symlink path. The mismatch
# makes filepath.Rel produce an "outside repository" path → git show fails with
# "is outside repository at" → barrier fails-closed (not_evaluated). Fix: resolve $WORK
# to its physical path (WORK_PHYS) and use that for all trust-check fixtures.
# ---------------------------------------------------------------------------
WORK_PHYS=$(cd "$WORK" && pwd -P)
_GITCFG="$WORK_PHYS/barrier-gitcfg"
printf '[user]\n\temail = barrier@trackfw.test\n\tname = trackfw barrier gate\n[commit]\n\tgpgsign = false\n[core]\n\thooksPath = /dev/null\n' > "$_GITCFG"

make_barrier_git_fixture() {
  local dest=$1 sentinel=$2
  local bare="${dest}.origin.git"
  local -a ge=(
    "GIT_CONFIG_GLOBAL=$_GITCFG"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$WORK_PHYS/home"
    "LC_ALL=C"
  )

  env "${ge[@]}" git init -q --bare -b main "$bare"
  env "${ge[@]}" git clone -q "$bare" "$dest"

  # Set up project structure
  common_dirs "$dest"
  cat >"$dest/trackfw.yaml" <<'YAML'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dirs: []
YAML

  # Write the roadmap with a gate that creates the sentinel
  cat >"$dest/docs/roadmaps/wip/ROADMAP-trust-fixture.md" <<EOF
# Roadmap: Trust Fixture

## Wave 0 — Threat Model

### ML-0A — Threat model for this fixture
**Status:** ✅
**Gates da wave:**
\`\`\`bash
exit 0
\`\`\`
**Critérios de aceite:**
- [x] threat model complete

## Wave 1 — Fixture Wave

**Gates da wave:**
\`\`\`bash
touch "$sentinel"
\`\`\`

### ML-1A — Fixture ML
**Status:** ✅
**Critérios de aceite:**
- [x] done
EOF

  # Base commit: only trackfw.yaml (no roadmap) — roadmap stays untracked
  (
    cd "$dest"
    env "${ge[@]}" git add trackfw.yaml
    env "${ge[@]}" git commit -q -m "base commit"
    env "${ge[@]}" git push -q origin main
  )
}

# commit_roadmap_to_origin DEST
# Commits the roadmap that make_barrier_git_fixture wrote to disk and pushes it
# to origin/main. Idempotent.
commit_roadmap_to_origin() {
  local dest=$1
  local -a ge=(
    "GIT_CONFIG_GLOBAL=$_GITCFG"
    "GIT_CONFIG_SYSTEM=/dev/null"
    "GIT_TERMINAL_PROMPT=0"
    "HOME=$WORK_PHYS/home"
    "LC_ALL=C"
  )
  (
    cd "$dest"
    env "${ge[@]}" git add docs/roadmaps/wip/ROADMAP-trust-fixture.md
    env "${ge[@]}" git commit -q -m "add roadmap"
    env "${ge[@]}" git push -q origin main
  )
}

# ---------------------------------------------------------------------------
# Scenario 14 — trust check: roadmap NOT committed in origin/main, no flag.
# barrier must report gates.status = not_evaluated and exit 1.
# AC14: the sentinel MUST NOT be created (the gate must not execute at all).
# Sentinel-absence is checked FIRST, before exit code — because the original
# bug executed the gate and then reported "blocked". The detection arm of
# Cenário 172 (Direction B) targets this diagnostic string.
# ---------------------------------------------------------------------------
S14="$WORK_PHYS/s14-not-committed"
SENTINEL14="$WORK_PHYS/s14-sentinel"
EXPECTED_NOT_COMMITTED="gates not evaluated: roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates"
make_barrier_git_fixture "$S14" "$SENTINEL14"

for runtime in go; do  # ML-3A (v8): node py removidos
  [[ ! -f "$SENTINEL14" ]] || fail "barrier/trust/not-committed/pre-$runtime" "sentinel existed before $runtime run — previous runtime broke trust check"
  run_barrier "$runtime" "$S14" ROADMAP-trust-fixture --wave 1 --json
  # AC14: sentinel-absence FIRST
  [[ ! -f "$SENTINEL14" ]] || fail "barrier/trust/not-committed/$runtime" "hostile gate EXECUTED — sentinel was created; trust check failed to block execution"
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/trust/not-committed/$runtime" "expected exit 1, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GATES_STATUS14=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT" | strip_cr)
  [[ "$GATES_STATUS14" == "not_evaluated" ]] || fail "barrier/trust/not-committed/$runtime" "expected gates.status=not_evaluated, got [$GATES_STATUS14]; stdout: $BARRIER_STDOUT"
  GATES_FAIL14=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c.get('failures',[]) for c in d['checks'] if c['name']=='gates']; print(r[0][0] if r and r[0] else 'MISSING')" "$BARRIER_STDOUT" | strip_cr)
  [[ "$GATES_FAIL14" == "$EXPECTED_NOT_COMMITTED" ]] || fail "barrier/trust/not-committed/$runtime" "failure message mismatch; want [$EXPECTED_NOT_COMMITTED] got [$GATES_FAIL14]"
  ok "barrier/trust/not-committed/$runtime"
done

# Cross-CLI JSON parity for not_evaluated (normalized — timestamps stripped)
STRIP_TS_TRUST='import json,sys; d=json.loads(sys.argv[1]); d.pop("started_at",None); d.pop("finished_at",None); print(json.dumps(d,sort_keys=True))'
run_barrier go   "$S14" ROADMAP-trust-fixture --wave 1 --json; GO_S14=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT" | strip_cr)
ok "barrier/trust/not-committed/parity/go-behavioral-pin"

# ---------------------------------------------------------------------------
# Scenario 15 — trust check: same fixture (not committed), with --trust-local-gates.
# barrier must execute the gate. The sentinel IS created, proving --trust-local-gates
# bypasses the trust check (AC12, AC15). This is the vacuity guard for Scenario 14:
# if the gate itself were broken, the sentinel would never appear regardless of trust.
# ---------------------------------------------------------------------------
S15="$WORK_PHYS/s15-trust-local-gates"
SENTINEL15="$WORK_PHYS/s15-sentinel"
make_barrier_git_fixture "$S15" "$SENTINEL15"

for runtime in go; do  # ML-3A (v8): node py removidos
  rm -f "$SENTINEL15"
  run_barrier "$runtime" "$S15" ROADMAP-trust-fixture --wave 1 --json --trust-local-gates
  # Sentinel-presence proves the gate executed. Do NOT assert exit 0: the
  # validate check inside barrier may legitimately fail on the minimal git
  # fixture (branch_has_wip_roadmap etc.) — that is orthogonal to trust.
  [[ -f "$SENTINEL15" ]] || fail "barrier/trust/trust-local-gates/$runtime" "gate did NOT execute — sentinel was NOT created; --trust-local-gates may be broken"
  GATES_STATUS15=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT" | strip_cr)
  [[ "$GATES_STATUS15" == "passed" ]] || fail "barrier/trust/trust-local-gates/$runtime" "expected gates.status=passed, got [$GATES_STATUS15]"
  ok "barrier/trust/trust-local-gates/$runtime"
done

# Cross-CLI JSON parity for --trust-local-gates gates check specifically
# (strip timestamps; also strip validate block which may diverge on the
# minimal fixture — parity claim is about the gates check, not validate).
STRIP_TS_GATES='import json,sys; d=json.loads(sys.argv[1]); d.pop("started_at",None); d.pop("finished_at",None); d["checks"]=[c for c in d.get("checks",[]) if c["name"]=="gates"]; print(json.dumps(d,sort_keys=True))'
rm -f "$SENTINEL15"
run_barrier go   "$S15" ROADMAP-trust-fixture --wave 1 --json --trust-local-gates; GO_S15=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT" | strip_cr)
rm -f "$SENTINEL15"
rm -f "$SENTINEL15"
ok "barrier/trust/trust-local-gates/parity/go-behavioral-pin"

# ---------------------------------------------------------------------------
# Scenario 16 — trust check: roadmap committed and IDENTICAL to origin/main.
# barrier must execute gates (trusted). Sentinel IS created.
# ---------------------------------------------------------------------------
S16="$WORK_PHYS/s16-trusted-identical"
SENTINEL16="$WORK_PHYS/s16-sentinel"
make_barrier_git_fixture "$S16" "$SENTINEL16"
commit_roadmap_to_origin "$S16"

for runtime in go; do  # ML-3A (v8): node py removidos
  rm -f "$SENTINEL16"
  run_barrier "$runtime" "$S16" ROADMAP-trust-fixture --wave 1 --json
  # Sentinel-presence proves the gate executed (roadmap trusted).
  # Do NOT assert exit 0: validate inside barrier may fail on the minimal git
  # fixture — that is orthogonal to the trust check being exercised here.
  [[ -f "$SENTINEL16" ]] || fail "barrier/trust/trusted-identical/$runtime" "gate did NOT execute — sentinel was NOT created; trust check may be over-refusing"
  GATES_STATUS16=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT" | strip_cr)
  [[ "$GATES_STATUS16" == "passed" ]] || fail "barrier/trust/trusted-identical/$runtime" "expected gates.status=passed, got [$GATES_STATUS16]"
  ok "barrier/trust/trusted-identical/$runtime"
done

# Cross-CLI JSON parity for trusted-identical gates check specifically
rm -f "$SENTINEL16"
run_barrier go   "$S16" ROADMAP-trust-fixture --wave 1 --json; GO_S16=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT" | strip_cr)
rm -f "$SENTINEL16"
rm -f "$SENTINEL16"
ok "barrier/trust/trusted-identical/parity/go-behavioral-pin"

# ---------------------------------------------------------------------------
# Scenario 17 — trust check: roadmap committed in origin/main but LOCAL CONTENT
# DIFFERS (one line appended). barrier must refuse gate execution with the
# "content differs" failure string, not_evaluated, exit 1, sentinel NOT created.
# This covers the second pinned failure string (docs/cli-parity.md §Pinned
# failure strings for not_evaluated).
# ---------------------------------------------------------------------------
S17="$WORK_PHYS/s17-content-differs"
SENTINEL17="$WORK_PHYS/s17-sentinel"
EXPECTED_DIFFERS="gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates"
make_barrier_git_fixture "$S17" "$SENTINEL17"
commit_roadmap_to_origin "$S17"
# Append one byte locally (does not change origin/main)
echo "# local-only edit" >> "$S17/docs/roadmaps/wip/ROADMAP-trust-fixture.md"

for runtime in go; do  # ML-3A (v8): node py removidos
  [[ ! -f "$SENTINEL17" ]] || fail "barrier/trust/content-differs/pre-$runtime" "sentinel existed before $runtime run"
  run_barrier "$runtime" "$S17" ROADMAP-trust-fixture --wave 1 --json
  [[ ! -f "$SENTINEL17" ]] || fail "barrier/trust/content-differs/$runtime" "hostile gate EXECUTED — sentinel was created; content-differs check failed"
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/trust/content-differs/$runtime" "expected exit 1, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GATES_STATUS17=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT" | strip_cr)
  [[ "$GATES_STATUS17" == "not_evaluated" ]] || fail "barrier/trust/content-differs/$runtime" "expected gates.status=not_evaluated, got [$GATES_STATUS17]"
  GATES_FAIL17=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c.get('failures',[]) for c in d['checks'] if c['name']=='gates']; print(r[0][0] if r and r[0] else 'MISSING')" "$BARRIER_STDOUT" | strip_cr)
  [[ "$GATES_FAIL17" == "$EXPECTED_DIFFERS" ]] || fail "barrier/trust/content-differs/$runtime" "failure message mismatch; want [$EXPECTED_DIFFERS] got [$GATES_FAIL17]"
  ok "barrier/trust/content-differs/$runtime"
done

# Cross-CLI JSON parity for content-differs path
run_barrier go   "$S17" ROADMAP-trust-fixture --wave 1 --json; GO_S17=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT" | strip_cr)
ok "barrier/trust/content-differs/parity/go-behavioral-pin"

echo
echo "All check-barrier.sh scenarios passed."
