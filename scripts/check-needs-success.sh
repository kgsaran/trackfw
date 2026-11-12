#!/usr/bin/env bash
# check-needs-success.sh — ADR D2: approve needs-job results only when all report
# result == "success" (allowlist, not a denylist of known-bad values).
# Linked to: make parity-rest  (REQ-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins...)
#
# ADR: docs/adr/ADR-2026-11-12-agregador-de-ci-exige-success-dos-needs-em-vez-de-reprovar-lista-de-valores-ruins.md
#
# USAGE
#   NEEDS_JSON=<json>  scripts/check-needs-success.sh <job> [<job>...]
#   NEEDS_JSON=<json>  scripts/check-needs-success.sh --self-test
#
# CONTRACT (rc=0 iff ALL of):
#   - At least one job argument given
#   - NEEDS_JSON is set, non-empty, valid JSON object at the top level
#   - Set of JSON keys == set of job arguments (extra or missing keys fail)
#   - Every job's "result" value is exactly the string "success"
#     (no case folding, no whitespace trimming, null/absent/non-string all fail)
#   - python3 is available on PATH
#
# On failure: one ::error:: annotation per job/reason (GitHub Actions format).
# Never prints "outputs" values.
#
# BASH: compatible with /bin/bash 3.2 (no mapfile, no declare -A, no ${var,,}).

set -euo pipefail
export PYTHONIOENCODING=utf-8

SELF=${BASH_SOURCE[0]##*/}

# ---------------------------------------------------------------------------
# Argument handling
# ---------------------------------------------------------------------------
MODE=run
if [ "${1:-}" = "--self-test" ]; then
  MODE=self-test
  shift
fi

# ---------------------------------------------------------------------------
# Main path
# ---------------------------------------------------------------------------
if [ "$MODE" = "run" ]; then

  if [ $# -eq 0 ]; then
    printf '%s: ::error::no job names given -- usage: NEEDS_JSON=<json> %s <job> [<job>...]\n' \
      "$SELF" "$SELF" >&2
    exit 1
  fi

  if [ -z "${NEEDS_JSON+x}" ]; then
    printf '::error::NEEDS_JSON is not set\n' >&2
    exit 1
  fi

  if ! command -v python3 >/dev/null 2>&1; then
    printf '::error::python3 is not available; cannot parse NEEDS_JSON\n' >&2
    exit 1
  fi

  python3 - "$@" <<'PY'
import sys, os, json

def esc(s):
    s = str(s)
    s = s.replace('%', '%25')
    s = s.replace('\r', '%0D')
    s = s.replace('\n', '%0A')
    return s

def reject_dupes(pairs):
    d = {}
    for k, v in pairs:
        if k in d:
            raise ValueError("duplicate key in JSON object: " + str(k))
        d[k] = v
    return d

expected = set(sys.argv[1:])
raw = os.environ.get("NEEDS_JSON", "")

try:
    needs = json.loads(raw, object_pairs_hook=reject_dupes)
except (ValueError, TypeError) as e:
    sys.stderr.write("::error::NEEDS_JSON is not valid JSON: " + esc(str(e)) + "\n")
    sys.exit(1)

if not isinstance(needs, dict):
    sys.stderr.write(
        "::error::NEEDS_JSON top-level is not a JSON object (got: "
        + esc(type(needs).__name__) + ")\n"
    )
    sys.exit(1)

actual = set(needs.keys())
failed = []

for job in sorted(expected - actual):
    sys.stderr.write("::error::job " + esc(job) + " expected but missing from NEEDS_JSON\n")
    failed.append(job)

for job in sorted(actual - expected):
    sys.stderr.write("::error::job " + esc(job) + " present in NEEDS_JSON but not in argument list\n")
    failed.append(job)

for job in sorted(actual & expected):
    data = needs[job]
    if not isinstance(data, dict):
        r = None
    else:
        r = data.get("result")
    if type(r) is not str or r != "success":  # allowlist-check
        sys.stderr.write(
            "::error::job " + esc(job) + " result=" + esc(r)
            + " (expected: success)\n"
        )
        failed.append(job)

if failed:
    sys.exit(1)

# All jobs approved -- print evidence for the CI log (AC6)
for job in sorted(actual):
    data = needs[job]
    r = data.get("result") if isinstance(data, dict) else None
    sys.stdout.write("OK  " + esc(job) + "  result=" + esc(r) + "\n")
sys.exit(0)
PY

fi  # end MODE=run

# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------
if [ "$MODE" = "self-test" ]; then

  SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  SCRIPT="$SCRIPT_DIR/${BASH_SOURCE[0]##*/}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  PASS=0
  FAIL=0
  ARMS=0
  EXPECTED_ARMS=27

  # check_arm NAME EXPECT_OK RC OUT [ABSENT [PRESENT]]
  # EXPECT_OK: 1=expect rc=0, 0=expect rc!=0
  check_arm() {
    local name="$1"
    local expect_ok="$2"
    local actual_rc="$3"
    local actual_out="$4"
    local want_absent="${5:-}"
    local want_present="${6:-}"
    ARMS=$((ARMS + 1))
    local ok=1
    if [ "$expect_ok" = "1" ] && [ "$actual_rc" -ne 0 ]; then
      printf 'FAIL %s: expected rc=0, got rc=%d\n' "$name" "$actual_rc"
      printf '  output: %s\n' "$actual_out"
      ok=0
    fi
    if [ "$expect_ok" = "0" ] && [ "$actual_rc" -eq 0 ]; then
      printf 'FAIL %s: expected rc!=0, got rc=0\n' "$name"
      printf '  output: %s\n' "$actual_out"
      ok=0
    fi
    if [ -n "$want_absent" ]; then
      if printf '%s\n' "$actual_out" | grep -F -- "$want_absent" >/dev/null; then
        printf 'FAIL %s: found forbidden substring [%s]\n' "$name" "$want_absent"
        ok=0
      fi
    fi
    if [ -n "$want_present" ]; then
      if ! printf '%s\n' "$actual_out" | grep -F -- "$want_present" >/dev/null; then
        printf 'FAIL %s: missing expected substring [%s]\n' "$name" "$want_present"
        ok=0
      fi
    fi
    if [ "$ok" = "1" ]; then
      printf 'PASS %s\n' "$name"
      PASS=$((PASS + 1))
    else
      FAIL=$((FAIL + 1))
    fi
  }

  # --- Arm 1: all-success-1job
  # Affirms: a single job with result=="success" and extra outputs is approved (V12 false-negative proof)
  rc=0
  out=$(NEEDS_JSON='{"j":{"result":"success","outputs":{}}}' \
    "$BASH" "$SCRIPT" j 2>&1) || rc=$?
  check_arm "all-success-1job" 1 "$rc" "$out" "" "OK  j  result=success"

  # --- Arm 2: all-success-3jobs
  # Affirms: multi-line, indented, three-job JSON is accepted -- json.loads does not require compactness
  json3='{
  "parity-falsify-shard": {"result": "success"},
  "parity-other-gates": {"result": "success"},
  "check-required-checks": {"result": "success"}
}'
  rc=0
  out=$(NEEDS_JSON="$json3" \
    "$BASH" "$SCRIPT" parity-falsify-shard parity-other-gates check-required-checks 2>&1) || rc=$?
  check_arm "all-success-3jobs" 1 "$rc" "$out" "" "parity-falsify-shard"

  # --- Arm 3: compact-json
  # Affirms: compact single-line JSON without spaces is accepted (V16 false-negative: parser handles both forms)
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "compact-json" 1 "$rc" "$out" "" "OK  a  result=success"

  # --- Arm 4: abandoned
  # Affirms: "abandoned" is not "success" -- the value that triggered issue #664 is correctly rejected
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"abandoned"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "abandoned" 0 "$rc" "$out" "" "abandoned"

  # --- Arm 5: failure
  # Affirms: "failure" is not "success" -- the classic failure value is rejected (not a false denylist pass)
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"failure"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "failure" 0 "$rc" "$out" "" "failure"

  # --- Arm 6: cancelled
  # Affirms: "cancelled" is not "success" -- job cancellations are rejected
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"cancelled"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "cancelled" 0 "$rc" "$out" "" "cancelled"

  # --- Arm 7: skipped
  # Affirms: "skipped" is not "success" -- the value that slipped through the old denylist (PR #365) is rejected
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"skipped"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "skipped" 0 "$rc" "$out" "" "skipped"

  # --- Arm 8: invented-value
  # Affirms: any value not in the allowlist fails -- a future undocumented GitHub value is rejected by construction
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"pending"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "invented-value" 0 "$rc" "$out" "" "pending"

  # --- Arm 9: empty-object
  # Affirms: {} with no jobs rejects when job names were expected -- guards against an empty toJSON(needs)
  rc=0
  out=$(NEEDS_JSON='{}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "empty-object" 0 "$rc" "$out" "" "a"

  # --- Arm 10: empty-string
  # Affirms: NEEDS_JSON="" is not valid JSON and is rejected -- empty env var from misconfigured step
  rc=0
  out=$(NEEDS_JSON='' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "empty-string" 0 "$rc" "$out" "" "not valid JSON"

  # --- Arm 11: invalid-json
  # Affirms: non-JSON input is rejected -- malformed NEEDS_JSON does not silently approve
  rc=0
  out=$(NEEDS_JSON='not-json' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "invalid-json" 0 "$rc" "$out" "" "not valid JSON"

  # --- Arm 12: null-toplevel
  # Affirms: JSON null at top level is rejected -- toJSON(needs) for a job with no needs: might return null
  rc=0
  out=$(NEEDS_JSON='null' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "null-toplevel" 0 "$rc" "$out" "" "not a JSON object"

  # --- Arm 13: array-toplevel
  # Affirms: a JSON array at top level is rejected -- the script requires an object
  rc=0
  out=$(NEEDS_JSON='[]' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "array-toplevel" 0 "$rc" "$out" "" "not a JSON object"

  # --- Arm 14: string-toplevel
  # Affirms: a JSON string at top level is rejected -- guards against scalar inputs
  rc=0
  out=$(NEEDS_JSON='"success"' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "string-toplevel" 0 "$rc" "$out" "" "not a JSON object"

  # --- Arm 15: missing-result-key
  # Affirms: a job dict without the "result" key is rejected (data.get("result") returns None, which != "success")
  rc=0
  out=$(NEEDS_JSON='{"a":{}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "missing-result-key" 0 "$rc" "$out" "" "a"

  # --- Arm 16: null-result
  # Affirms: result=null (JSON null -> Python None) is rejected (type(None) is not str)
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":null}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "null-result" 0 "$rc" "$out" "" "a"

  # --- Arm 17: uppercase-success
  # Affirms: "Success" (capital S) is not accepted -- comparison is case-sensitive, no .lower() applied
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"Success"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "uppercase-success" 0 "$rc" "$out" "" "Success"

  # --- Arm 18: trailing-space
  # Affirms: "success " (trailing space) is not accepted -- comparison is exact, no .strip() applied
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success "}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "trailing-space" 0 "$rc" "$out" "" "success "

  # --- Arm 19: mixed
  # Affirms: one successful job does not mask a failing sibling -- ALL jobs must be success, not ANY
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success"},"b":{"result":"abandoned"}}' \
    "$BASH" "$SCRIPT" a b 2>&1) || rc=$?
  check_arm "mixed" 0 "$rc" "$out" "" "abandoned"

  # --- Arm 20: outputs-injection
  # Affirms: "outputs" values are never printed -- an injected ::error:: in outputs cannot forge a CI annotation
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success","outputs":{"k":"\n::error::forged"}}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "outputs-injection" 1 "$rc" "$out" "::error::forged" ""

  # --- Arm 21: jobname-injection-newline (handled manually: need line-start grep)
  # Affirms: a newline in a job key is escaped (%0A) -- ::warning:: cannot be injected into a fresh CI annotation line
  ARMS=$((ARMS + 1))
  job_with_nl=$'x\n::warning::forged'
  rc21=0
  out21=$(NEEDS_JSON='{"x\n::warning::forged":{"result":"abandoned"}}' \
    "$BASH" "$SCRIPT" "$job_with_nl" 2>&1) || rc21=$?
  arm21_ok=1
  if [ "$rc21" -eq 0 ]; then
    printf 'FAIL jobname-injection-newline: expected rc!=0, got rc=0\n'
    printf '  output: %s\n' "$out21"
    arm21_ok=0
  fi
  if printf '%s\n' "$out21" | grep "^::warning::forged" >/dev/null; then
    printf 'FAIL jobname-injection-newline: unescaped ::warning::forged line found\n'
    arm21_ok=0
  fi
  if ! printf '%s\n' "$out21" | grep -F "%0A::warning::forged" >/dev/null; then
    printf 'FAIL jobname-injection-newline: escaped form %%0A::warning::forged not found\n'
    arm21_ok=0
  fi
  if [ "$arm21_ok" = "1" ]; then
    printf 'PASS jobname-injection-newline\n'
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
  fi

  # --- Arm 22: unset
  # Affirms: a completely absent NEEDS_JSON is rejected before python3 is called -- guards misconfigured steps
  rc=0
  out=$( (unset NEEDS_JSON; "$BASH" "$SCRIPT" a) 2>&1) || rc=$?
  check_arm "unset" 0 "$rc" "$out" "" "not set"

  # --- Arm 23: no-interpreter
  # Affirms: python3 absence is fail-closed -- the script names the missing interpreter in the error
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success"}}' PATH=/dev/null \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "no-interpreter" 0 "$rc" "$out" "" "python3"

  # --- Arm 24: missing-expected-job (R2 correction from ML-0A audit)
  # Affirms: a job listed as argument but absent from NEEDS_JSON is rejected -- shrinking needs: without
  #          updating the script call fails closed (guards against silent coverage drop)
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success"}}' \
    "$BASH" "$SCRIPT" a b 2>&1) || rc=$?
  check_arm "missing-expected-job" 0 "$rc" "$out" "" "b"

  # --- Arm 25: unexpected-job (R2 correction from ML-0A audit)
  # Affirms: a job present in NEEDS_JSON but not in the argument list is rejected -- guards against
  #          extra jobs silently not being verified when needs: grows
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success"},"b":{"result":"success"}}' \
    "$BASH" "$SCRIPT" a 2>&1) || rc=$?
  check_arm "unexpected-job" 0 "$rc" "$out" "" "b"

  # --- Arm 26: no-args
  # Affirms: calling the script with no job arguments is rejected -- prevents vacuous approval
  rc=0
  out=$(NEEDS_JSON='{"a":{"result":"success"}}' \
    "$BASH" "$SCRIPT" 2>&1) || rc=$?
  check_arm "no-args" 0 "$rc" "$out" "" "no job names"

  # --- Arm 27: counter-arm-denylist (AC5 sabotage proof)
  # Affirms: replacing the allowlist check with a denylist causes "abandoned" to be approved --
  #          proving that the original allowlist is the only guard catching the #664 vector
  ARMS=$((ARMS + 1))
  sabotado="$tmpdir/sabotado.sh"
  orig="$SCRIPT"
  sed 's/^    if type(r) is not str or r != .success.:  # allowlist-check$/    if type(r) is str and r in ("failure","cancelled","skipped"):/' \
    "$orig" > "$sabotado"
  arm27_ok=1
  if cmp -s "$orig" "$sabotado"; then
    printf 'FAIL counter-arm-denylist: sed did not change the script -- sabotage has no effect (line literal mismatch)\n'
    arm27_ok=0
  fi
  if [ "$arm27_ok" = "1" ]; then
    chmod +x "$sabotado"
    # sabotaged script must APPROVE abandoned (denylist misses it)
    rc_ab=0
    out_ab=$(NEEDS_JSON='{"a":{"result":"abandoned"}}' \
      "$BASH" "$sabotado" a 2>&1) || rc_ab=$?
    if [ "$rc_ab" -ne 0 ]; then
      printf 'FAIL counter-arm-denylist: sabotaged script rejected "abandoned" (rc=%d) -- denylist regression not proved\n' "$rc_ab"
      printf '  output: %s\n' "$out_ab"
      arm27_ok=0
    fi
    # sabotaged script must REJECT failure (denylist catches it -- proving it is a denylist, not approve-all)
    rc_fl=0
    out_fl=$(NEEDS_JSON='{"a":{"result":"failure"}}' \
      "$BASH" "$sabotado" a 2>&1) || rc_fl=$?
    if [ "$rc_fl" -eq 0 ]; then
      printf 'FAIL counter-arm-denylist: sabotaged script approved "failure" (rc=0) -- not a real denylist\n'
      arm27_ok=0
    fi
  fi
  if [ "$arm27_ok" = "1" ]; then
    printf 'PASS counter-arm-denylist\n'
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
  fi

  printf '\n--- self-test results: %d passed, %d failed (of %d arms, floor=%d) ---\n' \
    "$PASS" "$FAIL" "$ARMS" "$EXPECTED_ARMS"

  if [ "$ARMS" -lt "$EXPECTED_ARMS" ]; then
    printf 'FAIL vacuity: only %d arms executed, expected at least %d\n' "$ARMS" "$EXPECTED_ARMS" >&2
    exit 1
  fi
  if [ "$FAIL" -gt 0 ]; then
    exit 1
  fi
  exit 0

fi  # end MODE=self-test
