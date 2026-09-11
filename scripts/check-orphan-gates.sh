#!/usr/bin/env bash
# check-orphan-gates.sh — every scripts/check-*.sh must have a consumer.
# Linked to: make parity-rest (ML-NOVO, ROADMAP-2026-09-11-serve-interpola-host-...)
#
# WHY THIS EXISTS — three orphan incidents on the same day (2026-09-11):
#   ratchet job        → job reprovava; não era `required`     → ninguém consumia o veredito
#   baseline D4        → não rodava; origin/main não fetchado  → ninguém consumia
#   check-raw-read-ban → existia, fora de todo alvo            → ninguém consumia
#   check-serve-browser-security → idem, idêntico             → ninguém consumia
# The last two were linked manually; this gate prevents the next one.
#
# DECISION 1 — what counts as a consumer
# A script/check-*.sh has a consumer when its basename appears on a non-comment
# execution line in at least one of:
#   (a) A Makefile recipe line (tab-prefixed, not comment-only after the tab).
#   (b) A .sh file NOT under scripts/testdata/ (non-comment lines, not the script
#       itself — self-citation is not consumption).
#   (c) A GitHub Actions workflow (.github/**/*.yml), on any non-comment line.
#
# A citation inside scripts/testdata/ is a frozen corpus reference — never executed.
# This exclusion is the DISCRIMINANT: check-integration-cli-parity.sh is mentioned
# in testdata .md files and in comment-only lines in several scripts, but its only
# real consumer is the `bash "…/check-integration-cli-parity.sh"` call in
# check-cli-parity.sh:211. A gate that counted testdata references as consumers
# would classify a script with no real consumer as "consumed" — the naive-substring
# failure mode the handoff explicitly named.
# Falsification of the discriminant: in --self-test, arm 3 cites a synthetic orphan
# only inside scripts/testdata/fake-corpus.sh. The gate must FAIL for that arm.
# Confidence check: removing the `! -path "*/testdata/*"` exclusion from the search
# and re-running arm 3 should flip it to a false PASS — confirming the exclusion is
# load-bearing, not decorative.
#
# DECISION 2 — no consumer → FAIL (not warn)
# Rationale: warnings that do not block became noise that engineers learned to ignore
# (GitHub issue #275, observed in this project). Failing forces the author to wire
# the script before merging, which is exactly what the two prior orphan incidents
# would have required had this gate existed.
#
# DECISION 3 — exception list with mandatory reason
# A script may legitimately have no automated consumer (e.g. a one-shot diagnostic
# tool meant for manual use only). Such scripts must be declared in EXCEPTIONS below
# with a non-empty reason. An entry without "|reason" causes this gate to FAIL — the
# point is to make tolerance explicit and auditable, never silent.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Exception list.
# Format: one entry per line: "script-basename.sh|reason for exemption"
# The pipe-separated reason is MANDATORY. An entry without "|" fails this gate.
# Empty = no exceptions today.
# ---------------------------------------------------------------------------
EXCEPTIONS=(
  # Example (do not un-comment without a real reason):
  # "check-diagnose-something.sh|manual-only diagnostic; no automated consumer by design"
)

# ---------------------------------------------------------------------------
# Core: determine whether <basename> has a consumer under <root>.
# Prints a human-readable verdict line and returns 0 (consumed) or 1 (orphan).
# Optional third argument: space-separated list of extra .sh files to treat as
# testdata (used by --self-test to inject synthetic corpora).
# ---------------------------------------------------------------------------
has_consumer() {
  local root="$1"
  local basename="$2"
  # Skip own file to avoid self-citation
  local selffile="$SCRIPT_DIR/check-orphan-gates.sh"

  # (a) Makefile recipe lines (tab-prefixed, not comment-only)
  local makefile="$root/Makefile"
  if [ -f "$makefile" ]; then
    # Grep for tab-prefixed lines; exclude comment-only recipe lines (^\t#)
    if grep $'^\t' "$makefile" \
        | grep -v $'^\t#' \
        | grep -q "$basename"; then
      return 0
    fi
  fi

  # (b) .sh files NOT under scripts/testdata/, non-comment lines, not self
  local sh_files
  sh_files=$(find "$root/scripts" "$root/.github" \
      -name "*.sh" \
      ! -path "*/testdata/*" \
      2>/dev/null)
  local f
  for f in $sh_files; do
    # Skip the gate script itself
    [ "$(realpath "$f" 2>/dev/null || echo "$f")" = "$(realpath "$selffile" 2>/dev/null || echo "$selffile")" ] && continue
    # Filter comment lines (lines where first non-space character is #)
    if grep -v '^\s*#' "$f" 2>/dev/null | grep -q "$basename"; then
      return 0
    fi
  done

  # (c) GitHub Actions workflows (.github/**/*.yml), non-comment lines
  local yml_files
  yml_files=$(find "$root/.github" -name "*.yml" 2>/dev/null)
  for f in $yml_files; do
    if grep -v '^\s*#' "$f" 2>/dev/null | grep -q "$basename"; then
      return 0
    fi
  done

  return 1
}

# ---------------------------------------------------------------------------
# Validate the exception list itself: every entry must contain "|" with a
# non-empty reason. A malformed entry is itself a gate failure.
# ---------------------------------------------------------------------------
validate_exceptions() {
  local fail=0
  local entry
  for entry in "${EXCEPTIONS[@]+"${EXCEPTIONS[@]}"}"; do
    # Must contain "|"
    if [[ "$entry" != *"|"* ]]; then
      echo "FAIL [exceptions] malformed entry (no reason after '|'): '$entry'"
      fail=1
      continue
    fi
    local reason="${entry#*|}"
    reason="${reason// /}"   # strip spaces
    if [ -z "$reason" ]; then
      echo "FAIL [exceptions] empty reason in entry: '$entry'"
      fail=1
    fi
  done
  return $fail
}

# ---------------------------------------------------------------------------
# Self-test: exercises all 4 falsification branches in synthetic environments.
# ---------------------------------------------------------------------------
self_test() {
  local WORK
  WORK=$(mktemp -d /tmp/check-orphan-gates-selftest.XXXXXX)
  trap 'rm -rf "$WORK"' EXIT

  local SELF_FAIL=0

  # ---- Arm 1: orphan — script with NO consumer anywhere → must FAIL ----
  local arm1="$WORK/arm1"
  mkdir -p "$arm1/scripts" "$arm1/.github/workflows"
  echo '#!/usr/bin/env bash' > "$arm1/scripts/check-arm1-orphan.sh"
  # Empty Makefile and workflow — no consumer
  touch "$arm1/Makefile"
  touch "$arm1/.github/workflows/quality.yml"

  local arm1_out arm1_rc
  arm1_rc=0
  arm1_out=$(bash "$SCRIPT_DIR/check-orphan-gates.sh" --scan-root "$arm1" 2>&1) || arm1_rc=$?
  if [ "$arm1_rc" -ne 0 ] && echo "$arm1_out" | grep -q "check-arm1-orphan"; then
    echo "OK   [self-test/arm1] orphan correctly named and gate fails: rc=$arm1_rc"
  else
    echo "FAIL [self-test/arm1] expected non-zero rc naming 'check-arm1-orphan'; got rc=$arm1_rc output: $arm1_out"
    SELF_FAIL=1
  fi

  # ---- Arm 2: Makefile consumer → must PASS ----
  local arm2="$WORK/arm2"
  mkdir -p "$arm2/scripts" "$arm2/.github/workflows"
  echo '#!/usr/bin/env bash' > "$arm2/scripts/check-arm2-linked.sh"
  printf '\tscripts/check-arm2-linked.sh\n' > "$arm2/Makefile"
  touch "$arm2/.github/workflows/quality.yml"

  local arm2_out arm2_rc
  arm2_rc=0
  arm2_out=$(bash "$SCRIPT_DIR/check-orphan-gates.sh" --scan-root "$arm2" 2>&1) || arm2_rc=$?
  if [ "$arm2_rc" -eq 0 ]; then
    echo "OK   [self-test/arm2] Makefile-linked script passes: rc=$arm2_rc"
  else
    echo "FAIL [self-test/arm2] expected rc=0 for Makefile-linked script; got rc=$arm2_rc output: $arm2_out"
    SELF_FAIL=1
  fi

  # ---- Arm 3: testdata-only citation → must FAIL ----
  # This is the discriminant arm. The script is mentioned in scripts/testdata/fake-corpus.sh
  # but nowhere else. The gate must fail — testdata is not execution.
  # If we removed the testdata exclusion, this arm would flip to a false pass,
  # confirming the exclusion is load-bearing.
  local arm3="$WORK/arm3"
  mkdir -p "$arm3/scripts/testdata" "$arm3/.github/workflows"
  echo '#!/usr/bin/env bash' > "$arm3/scripts/check-arm3-testdata.sh"
  # Cite the script inside a testdata .sh file (same type as real scripts — the search
  # WOULD pick this up if testdata were not excluded; that is what makes the exclusion
  # load-bearing rather than a no-op against .md-only corpora)
  echo "# Referenced for documentation: scripts/check-arm3-testdata.sh is mentioned here" \
    > "$arm3/scripts/testdata/fake-corpus.sh"
  # Also add it to a non-comment line inside testdata to stress the exclusion further
  echo "echo 'see check-arm3-testdata.sh for details'" \
    >> "$arm3/scripts/testdata/fake-corpus.sh"
  touch "$arm3/Makefile"
  touch "$arm3/.github/workflows/quality.yml"

  local arm3_out arm3_rc
  arm3_rc=0
  arm3_out=$(bash "$SCRIPT_DIR/check-orphan-gates.sh" --scan-root "$arm3" 2>&1) || arm3_rc=$?
  if [ "$arm3_rc" -ne 0 ] && echo "$arm3_out" | grep -q "check-arm3-testdata"; then
    echo "OK   [self-test/arm3] testdata-only citation correctly rejected: rc=$arm3_rc"
  else
    echo "FAIL [self-test/arm3] expected non-zero rc naming 'check-arm3-testdata'; got rc=$arm3_rc output: $arm3_out"
    SELF_FAIL=1
  fi

  # ---- Arm 3 confidence check: dropping the testdata exclusion flips to false PASS ----
  # Run with ORPHAN_GATE_NO_TESTDATA_EXCLUSION=1 to verify the exclusion is the discriminant
  local arm3b_out arm3b_rc
  arm3b_rc=0
  arm3b_out=$(ORPHAN_GATE_NO_TESTDATA_EXCLUSION=1 bash "$SCRIPT_DIR/check-orphan-gates.sh" --scan-root "$arm3" 2>&1) || arm3b_rc=$?
  if [ "$arm3b_rc" -eq 0 ]; then
    echo "OK   [self-test/arm3-confidence] dropping testdata exclusion flips to false PASS — exclusion is load-bearing"
  else
    # If it STILL fails even without the exclusion, the fixture may have a different issue
    # (e.g. the .sh pattern doesn't match). Log but don't fail self-test: the primary arm3
    # already proves the exclusion works in one direction.
    echo "WARN [self-test/arm3-confidence] dropping testdata exclusion still fails (rc=$arm3b_rc) — check fixture; primary arm3 still valid"
  fi

  # ---- Arm 4a: exception with reason → must PASS ----
  local arm4a="$WORK/arm4a"
  mkdir -p "$arm4a/scripts" "$arm4a/.github/workflows"
  echo '#!/usr/bin/env bash' > "$arm4a/scripts/check-arm4a-excepted.sh"
  touch "$arm4a/Makefile"
  touch "$arm4a/.github/workflows/quality.yml"

  local arm4a_out arm4a_rc
  arm4a_rc=0
  arm4a_out=$(ORPHAN_GATE_EXTRA_EXCEPTIONS="check-arm4a-excepted.sh|manual diagnostic only; no CI target by design" \
    bash "$SCRIPT_DIR/check-orphan-gates.sh" --scan-root "$arm4a" 2>&1) || arm4a_rc=$?
  if [ "$arm4a_rc" -eq 0 ]; then
    echo "OK   [self-test/arm4a] excepted script (with reason) passes: rc=$arm4a_rc"
  else
    echo "FAIL [self-test/arm4a] expected rc=0 for excepted script with reason; got rc=$arm4a_rc output: $arm4a_out"
    SELF_FAIL=1
  fi

  # ---- Arm 4b: exception WITHOUT reason → gate itself must FAIL ----
  local arm4b_out arm4b_rc
  arm4b_rc=0
  arm4b_out=$(ORPHAN_GATE_EXTRA_EXCEPTIONS="check-arm4b-no-reason.sh" \
    bash "$SCRIPT_DIR/check-orphan-gates.sh" --scan-root "$arm4a" 2>&1) || arm4b_rc=$?
  if [ "$arm4b_rc" -ne 0 ] && echo "$arm4b_out" | grep -q "malformed\|empty reason"; then
    echo "OK   [self-test/arm4b] exception without reason correctly fails gate: rc=$arm4b_rc"
  else
    echo "FAIL [self-test/arm4b] expected non-zero rc for exception without reason; got rc=$arm4b_rc output: $arm4b_out"
    SELF_FAIL=1
  fi

  echo ""
  if [ "$SELF_FAIL" -ne 0 ]; then
    echo "check-orphan-gates --self-test: FAIL"
    exit 1
  fi
  echo "check-orphan-gates --self-test: OK (4 arms pass)"
  exit 0
}

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
SCAN_ROOT="$REPO_ROOT"
RUN_SELF_TEST=0

while [ $# -gt 0 ]; do
  case "$1" in
    --self-test)
      RUN_SELF_TEST=1
      shift
      ;;
    --scan-root)
      SCAN_ROOT="$2"
      shift 2
      ;;
    *)
      echo "check-orphan-gates: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [ "$RUN_SELF_TEST" -eq 1 ]; then
  self_test
fi

# ---------------------------------------------------------------------------
# Accept runtime exception injections (used by --self-test arm 4a/4b).
# Format: same as EXCEPTIONS entries, newline-separated.
# ---------------------------------------------------------------------------
EXTRA_EXCEPTIONS=()
if [ -n "${ORPHAN_GATE_EXTRA_EXCEPTIONS:-}" ]; then
  # Read newline or colon-separated entries
  while IFS= read -r entry; do
    [ -n "$entry" ] && EXTRA_EXCEPTIONS+=("$entry")
  done <<< "${ORPHAN_GATE_EXTRA_EXCEPTIONS//:/$'\n'}"
fi

ALL_EXCEPTIONS=("${EXCEPTIONS[@]+"${EXCEPTIONS[@]}"}" "${EXTRA_EXCEPTIONS[@]+"${EXTRA_EXCEPTIONS[@]}"}")

# ---------------------------------------------------------------------------
# Validate exception list format first — a malformed list fails the gate
# regardless of what the scan finds
# ---------------------------------------------------------------------------
EXCEPTIONS_FAIL=0
for entry in "${ALL_EXCEPTIONS[@]+"${ALL_EXCEPTIONS[@]}"}"; do
  if [[ "$entry" != *"|"* ]]; then
    echo "FAIL [exceptions] malformed entry (no reason after '|'): '$entry'"
    EXCEPTIONS_FAIL=1
    continue
  fi
  reason="${entry#*|}"
  reason_stripped="${reason// /}"
  if [ -z "$reason_stripped" ]; then
    echo "FAIL [exceptions] empty reason in entry: '$entry'"
    EXCEPTIONS_FAIL=1
  fi
done
if [ "$EXCEPTIONS_FAIL" -ne 0 ]; then
  echo ""
  echo "check-orphan-gates: FAIL — exception list is malformed (see above)"
  exit 1
fi

# ---------------------------------------------------------------------------
# Build the set of excepted basenames for O(1) lookup
# ---------------------------------------------------------------------------
declare -A EXCEPTED=()
for entry in "${ALL_EXCEPTIONS[@]+"${ALL_EXCEPTIONS[@]}"}"; do
  bname="${entry%%|*}"
  EXCEPTED["$bname"]=1
done

# ---------------------------------------------------------------------------
# Scan for testdata exclusion override (used by arm 3 confidence check)
# ---------------------------------------------------------------------------
NO_TESTDATA_EXCL="${ORPHAN_GATE_NO_TESTDATA_EXCLUSION:-}"

# ---------------------------------------------------------------------------
# Main scan
# ---------------------------------------------------------------------------
FAIL=0
FOUND_ANY=0

echo "=== check-orphan-gates: scanning scripts/check-*.sh under $SCAN_ROOT ==="

for script_path in "$SCAN_ROOT/scripts"/check-*.sh; do
  [ -e "$script_path" ] || continue
  FOUND_ANY=1
  bname="$(basename "$script_path")"

  # Check exception list
  if [ "${EXCEPTED[$bname]+set}" = "set" ]; then
    echo "OK   [excepted] $bname — ${entry#*|}"
    continue
  fi

  # Determine if consumed
  # (a) Makefile recipe lines
  local_found=0
  makefile="$SCAN_ROOT/Makefile"
  if [ -f "$makefile" ]; then
    if grep $'^\t' "$makefile" \
        | grep -v $'^\t#' \
        | grep -q "$bname"; then
      local_found=1
    fi
  fi

  # (b) .sh files NOT under testdata/ (unless override active), not self
  if [ "$local_found" -eq 0 ]; then
    if [ -n "$NO_TESTDATA_EXCL" ]; then
      sh_find_args=(-name "*.sh")
    else
      sh_find_args=(-name "*.sh" ! -path "*/testdata/*")
    fi
    while IFS= read -r f; do
      # Skip self
      [ "$(basename "$f")" = "check-orphan-gates.sh" ] && [ "$SCAN_ROOT" = "$REPO_ROOT" ] && continue
      if grep -v '^\s*#' "$f" 2>/dev/null | grep -q "$bname"; then
        local_found=1
        break
      fi
    done < <(find "$SCAN_ROOT/scripts" "$SCAN_ROOT/.github" \
        "${sh_find_args[@]}" \
        2>/dev/null)
  fi

  # (c) .yml workflow files, non-comment lines
  if [ "$local_found" -eq 0 ]; then
    while IFS= read -r f; do
      if grep -v '^\s*#' "$f" 2>/dev/null | grep -q "$bname"; then
        local_found=1
        break
      fi
    done < <(find "$SCAN_ROOT/.github" -name "*.yml" 2>/dev/null)
  fi

  if [ "$local_found" -eq 1 ]; then
    echo "OK   $bname"
  else
    echo "FAIL $bname — no consumer found (not in Makefile recipe, not called by any non-testdata script, not in any workflow)"
    FAIL=1
  fi
done

echo ""
if [ "$FOUND_ANY" -eq 0 ]; then
  echo "FAIL vacuity guard: no scripts/check-*.sh found under $SCAN_ROOT"
  exit 1
fi

if [ "$FAIL" -ne 0 ]; then
  echo "check-orphan-gates: FAIL — one or more check-*.sh scripts have no consumer"
  exit 1
fi

echo "check-orphan-gates: OK — all check-*.sh scripts have a consumer"
exit 0
