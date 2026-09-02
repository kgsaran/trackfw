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
for runtime in go node py; do
  run_barrier "$runtime" "$S11" ROADMAP-barrier-fixture --wave 0 --json
  [[ "$BARRIER_EXIT" -eq 0 || "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "expected exit 0 or 1 (never 2 — a usage/grammar error), got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GOT_WAVE=$(get_wave_field "$BARRIER_STDOUT")
  [[ "$GOT_WAVE" == "0" ]] || fail "barrier/wave-label/wave-zero-accepted/$runtime" "expected wave=0, got $GOT_WAVE"

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
run_barrier go "$S11" ROADMAP-barrier-fixture --wave 0 --json
GO_STDOUT11="$BARRIER_STDOUT"
run_barrier node "$S11" ROADMAP-barrier-fixture --wave 0 --json
NODE_STDOUT11="$BARRIER_STDOUT"
run_barrier py "$S11" ROADMAP-barrier-fixture --wave 0 --json
PY_STDOUT11="$BARRIER_STDOUT"
# started_at/finished_at are timestamps and legitimately differ per run — strip them before comparing.
STRIP_TS='import json,sys; d=json.loads(sys.argv[1]); d.pop("started_at",None); d.pop("finished_at",None); print(json.dumps(d,sort_keys=True))'
GO_NORM11=$(python3 -c "$STRIP_TS" "$GO_STDOUT11")
NODE_NORM11=$(python3 -c "$STRIP_TS" "$NODE_STDOUT11")
PY_NORM11=$(python3 -c "$STRIP_TS" "$PY_STDOUT11")
[[ "$GO_NORM11" == "$NODE_NORM11" ]] || fail "barrier/wave-label/wave-zero-accepted/parity/go-vs-node" "JSON diverges (timestamps excluded): go=[$GO_NORM11] node=[$NODE_NORM11]"
[[ "$GO_NORM11" == "$PY_NORM11" ]] || fail "barrier/wave-label/wave-zero-accepted/parity/go-vs-py" "JSON diverges (timestamps excluded): go=[$GO_NORM11] py=[$PY_NORM11]"
ok "barrier/wave-label/wave-zero-accepted/parity/three-runtimes-identical"

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
for runtime in go node py; do
  set +e
  case "$runtime" in
  go)   (cd "$S13" && "$GO_BIN" roadmap new --req docs/req/dummy.md --title "Valid Title For AC2 Guard") >/dev/null 2>&1 ;;
  node) (cd "$S13" && node "$NODE_CLI" roadmap new --req docs/req/dummy.md --title "Valid Title For AC2 Guard") >/dev/null 2>&1 ;;
  py)   (cd "$S13" && PYTHONPATH="$PY_ROOT" python3 -m trackfw roadmap new --req docs/req/dummy.md --title "Valid Title For AC2 Guard") >/dev/null 2>&1 ;;
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
for runtime in go node py; do
  set +e
  case "$runtime" in
  go)   OUT13=$(cd "$S13" && "$GO_BIN" roadmap new --req docs/req/dummy.md --title "$FORGED_TITLE" 2>&1); EXIT13=$? ;;
  node) OUT13=$(cd "$S13" && node "$NODE_CLI" roadmap new --req docs/req/dummy.md --title "$FORGED_TITLE" 2>&1); EXIT13=$? ;;
  py)   OUT13=$(cd "$S13" && PYTHONPATH="$PY_ROOT" python3 -m trackfw roadmap new --req docs/req/dummy.md --title "$FORGED_TITLE" 2>&1); EXIT13=$? ;;
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
# "is outside repository at" → barrier fails-open (trusted). Fix: resolve $WORK
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

for runtime in go node py; do
  [[ ! -f "$SENTINEL14" ]] || fail "barrier/trust/not-committed/pre-$runtime" "sentinel existed before $runtime run — previous runtime broke trust check"
  run_barrier "$runtime" "$S14" ROADMAP-trust-fixture --wave 1 --json
  # AC14: sentinel-absence FIRST
  [[ ! -f "$SENTINEL14" ]] || fail "barrier/trust/not-committed/$runtime" "hostile gate EXECUTED — sentinel was created; trust check failed to block execution"
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/trust/not-committed/$runtime" "expected exit 1, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GATES_STATUS14=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT")
  [[ "$GATES_STATUS14" == "not_evaluated" ]] || fail "barrier/trust/not-committed/$runtime" "expected gates.status=not_evaluated, got [$GATES_STATUS14]; stdout: $BARRIER_STDOUT"
  GATES_FAIL14=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c.get('failures',[]) for c in d['checks'] if c['name']=='gates']; print(r[0][0] if r and r[0] else 'MISSING')" "$BARRIER_STDOUT")
  [[ "$GATES_FAIL14" == "$EXPECTED_NOT_COMMITTED" ]] || fail "barrier/trust/not-committed/$runtime" "failure message mismatch; want [$EXPECTED_NOT_COMMITTED] got [$GATES_FAIL14]"
  ok "barrier/trust/not-committed/$runtime"
done

# Cross-CLI JSON parity for not_evaluated (normalized — timestamps stripped)
STRIP_TS_TRUST='import json,sys; d=json.loads(sys.argv[1]); d.pop("started_at",None); d.pop("finished_at",None); print(json.dumps(d,sort_keys=True))'
run_barrier go   "$S14" ROADMAP-trust-fixture --wave 1 --json; GO_S14=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT")
run_barrier node "$S14" ROADMAP-trust-fixture --wave 1 --json; NODE_S14=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT")
run_barrier py   "$S14" ROADMAP-trust-fixture --wave 1 --json; PY_S14=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT")
[[ "$GO_S14" == "$NODE_S14" ]] || fail "barrier/trust/not-committed/parity/go-vs-node" "JSON diverges: go=[$GO_S14] node=[$NODE_S14]"
[[ "$GO_S14" == "$PY_S14" ]]   || fail "barrier/trust/not-committed/parity/go-vs-py"   "JSON diverges: go=[$GO_S14] py=[$PY_S14]"
ok "barrier/trust/not-committed/parity/three-runtimes-identical"

# ---------------------------------------------------------------------------
# Scenario 15 — trust check: same fixture (not committed), with --trust-local-gates.
# barrier must execute the gate. The sentinel IS created, proving --trust-local-gates
# bypasses the trust check (AC12, AC15). This is the vacuity guard for Scenario 14:
# if the gate itself were broken, the sentinel would never appear regardless of trust.
# ---------------------------------------------------------------------------
S15="$WORK_PHYS/s15-trust-local-gates"
SENTINEL15="$WORK_PHYS/s15-sentinel"
make_barrier_git_fixture "$S15" "$SENTINEL15"

for runtime in go node py; do
  rm -f "$SENTINEL15"
  run_barrier "$runtime" "$S15" ROADMAP-trust-fixture --wave 1 --json --trust-local-gates
  # Sentinel-presence proves the gate executed. Do NOT assert exit 0: the
  # validate check inside barrier may legitimately fail on the minimal git
  # fixture (branch_has_wip_roadmap etc.) — that is orthogonal to trust.
  [[ -f "$SENTINEL15" ]] || fail "barrier/trust/trust-local-gates/$runtime" "gate did NOT execute — sentinel was NOT created; --trust-local-gates may be broken"
  GATES_STATUS15=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT")
  [[ "$GATES_STATUS15" == "passed" ]] || fail "barrier/trust/trust-local-gates/$runtime" "expected gates.status=passed, got [$GATES_STATUS15]"
  ok "barrier/trust/trust-local-gates/$runtime"
done

# Cross-CLI JSON parity for --trust-local-gates gates check specifically
# (strip timestamps; also strip validate block which may diverge on the
# minimal fixture — parity claim is about the gates check, not validate).
STRIP_TS_GATES='import json,sys; d=json.loads(sys.argv[1]); d.pop("started_at",None); d.pop("finished_at",None); d["checks"]=[c for c in d.get("checks",[]) if c["name"]=="gates"]; print(json.dumps(d,sort_keys=True))'
rm -f "$SENTINEL15"
run_barrier go   "$S15" ROADMAP-trust-fixture --wave 1 --json --trust-local-gates; GO_S15=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT")
rm -f "$SENTINEL15"
run_barrier node "$S15" ROADMAP-trust-fixture --wave 1 --json --trust-local-gates; NODE_S15=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT")
rm -f "$SENTINEL15"
run_barrier py   "$S15" ROADMAP-trust-fixture --wave 1 --json --trust-local-gates; PY_S15=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT")
[[ "$GO_S15" == "$NODE_S15" ]] || fail "barrier/trust/trust-local-gates/parity/go-vs-node" "gates-check JSON diverges: go=[$GO_S15] node=[$NODE_S15]"
[[ "$GO_S15" == "$PY_S15" ]]   || fail "barrier/trust/trust-local-gates/parity/go-vs-py"   "gates-check JSON diverges: go=[$GO_S15] py=[$PY_S15]"
ok "barrier/trust/trust-local-gates/parity/three-runtimes-identical"

# ---------------------------------------------------------------------------
# Scenario 16 — trust check: roadmap committed and IDENTICAL to origin/main.
# barrier must execute gates (trusted). Sentinel IS created.
# ---------------------------------------------------------------------------
S16="$WORK_PHYS/s16-trusted-identical"
SENTINEL16="$WORK_PHYS/s16-sentinel"
make_barrier_git_fixture "$S16" "$SENTINEL16"
commit_roadmap_to_origin "$S16"

for runtime in go node py; do
  rm -f "$SENTINEL16"
  run_barrier "$runtime" "$S16" ROADMAP-trust-fixture --wave 1 --json
  # Sentinel-presence proves the gate executed (roadmap trusted).
  # Do NOT assert exit 0: validate inside barrier may fail on the minimal git
  # fixture — that is orthogonal to the trust check being exercised here.
  [[ -f "$SENTINEL16" ]] || fail "barrier/trust/trusted-identical/$runtime" "gate did NOT execute — sentinel was NOT created; trust check may be over-refusing"
  GATES_STATUS16=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT")
  [[ "$GATES_STATUS16" == "passed" ]] || fail "barrier/trust/trusted-identical/$runtime" "expected gates.status=passed, got [$GATES_STATUS16]"
  ok "barrier/trust/trusted-identical/$runtime"
done

# Cross-CLI JSON parity for trusted-identical gates check specifically
rm -f "$SENTINEL16"
run_barrier go   "$S16" ROADMAP-trust-fixture --wave 1 --json; GO_S16=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT")
rm -f "$SENTINEL16"
run_barrier node "$S16" ROADMAP-trust-fixture --wave 1 --json; NODE_S16=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT")
rm -f "$SENTINEL16"
run_barrier py   "$S16" ROADMAP-trust-fixture --wave 1 --json; PY_S16=$(python3 -c "$STRIP_TS_GATES" "$BARRIER_STDOUT")
[[ "$GO_S16" == "$NODE_S16" ]] || fail "barrier/trust/trusted-identical/parity/go-vs-node" "gates-check JSON diverges: go=[$GO_S16] node=[$NODE_S16]"
[[ "$GO_S16" == "$PY_S16" ]]   || fail "barrier/trust/trusted-identical/parity/go-vs-py"   "gates-check JSON diverges: go=[$GO_S16] py=[$PY_S16]"
ok "barrier/trust/trusted-identical/parity/three-runtimes-identical"

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

for runtime in go node py; do
  [[ ! -f "$SENTINEL17" ]] || fail "barrier/trust/content-differs/pre-$runtime" "sentinel existed before $runtime run"
  run_barrier "$runtime" "$S17" ROADMAP-trust-fixture --wave 1 --json
  [[ ! -f "$SENTINEL17" ]] || fail "barrier/trust/content-differs/$runtime" "hostile gate EXECUTED — sentinel was created; content-differs check failed"
  [[ "$BARRIER_EXIT" -eq 1 ]] || fail "barrier/trust/content-differs/$runtime" "expected exit 1, got $BARRIER_EXIT; stderr: $BARRIER_STDERR"
  GATES_STATUS17=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c['status'] for c in d['checks'] if c['name']=='gates']; print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT")
  [[ "$GATES_STATUS17" == "not_evaluated" ]] || fail "barrier/trust/content-differs/$runtime" "expected gates.status=not_evaluated, got [$GATES_STATUS17]"
  GATES_FAIL17=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); r=[c.get('failures',[]) for c in d['checks'] if c['name']=='gates']; print(r[0][0] if r and r[0] else 'MISSING')" "$BARRIER_STDOUT")
  [[ "$GATES_FAIL17" == "$EXPECTED_DIFFERS" ]] || fail "barrier/trust/content-differs/$runtime" "failure message mismatch; want [$EXPECTED_DIFFERS] got [$GATES_FAIL17]"
  ok "barrier/trust/content-differs/$runtime"
done

# Cross-CLI JSON parity for content-differs path
run_barrier go   "$S17" ROADMAP-trust-fixture --wave 1 --json; GO_S17=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT")
run_barrier node "$S17" ROADMAP-trust-fixture --wave 1 --json; NODE_S17=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT")
run_barrier py   "$S17" ROADMAP-trust-fixture --wave 1 --json; PY_S17=$(python3 -c "$STRIP_TS_TRUST" "$BARRIER_STDOUT")
[[ "$GO_S17" == "$NODE_S17" ]] || fail "barrier/trust/content-differs/parity/go-vs-node" "JSON diverges: go=[$GO_S17] node=[$NODE_S17]"
[[ "$GO_S17" == "$PY_S17" ]]   || fail "barrier/trust/content-differs/parity/go-vs-py"   "JSON diverges: go=[$GO_S17] py=[$PY_S17]"
ok "barrier/trust/content-differs/parity/three-runtimes-identical"

echo
echo "All check-barrier.sh scenarios passed."
