#!/usr/bin/env bash
# check-req-roadmap-rule-is-status-aware.sh — Anti-reintroduction gate for
# fix/orphan-req-reprova-estado-correto (ML-1C).
#
# PURPOSE: Each rule that decides "should this REQ have a roadmap?" MUST call
# reqStatusIsDone before emitting a violation. This gate verifies the contract
# holds after ML-1A (traceid_orphan_req) and ML-1B (req_has_roadmap).
#
# RULE LIST — explicit and closed:
# Heuristic detection (e.g. "any rule that calls readFileForRule") would match
# unrelated rules and produce false positives. Each rule below is named with a
# reason for inclusion and a reference to its correction ML.
#
#   | Rule                | File                     | ML that adds guard |
#   |---------------------|--------------------------|--------------------|
#   | req_has_roadmap     | validator.go             | ML-1B              |
#   | traceid_orphan_req  | validator_traceid.go     | ML-1A              |
#
# COUPLING NOTE: the anchor is readFileForRule("<rule>") — NOT applyRule("<rule>").
# applyRule("req_has_roadmap") lives at lines 821/1171 in the orchestrator function
# validateAll, which is enormous. Using applyRule as the anchor would extract a
# wrong, giant function and find reqStatusIsDone from a neighboring rule.
#
# EXEMPTIONS — explicit with rationale:
#
#   req_roadmap_lifecycle: already filters via reqStatusIsOpen — only Open REQs
#     trigger. Accusing it would prompt someone to remove the only rule that
#     already has the correct filter.
#
#   ref_targets_exist: measures file existence (os.Stat on the roadmap path),
#     not the link obligation. A Done REQ pointing to a missing file is a
#     legitimate stale reference — different signal, different action. Scope
#     declared in docs/cli-parity.md: wip/blocked/backlog/analyzing only.
#
# COMMENT FILTERING:
#   Both full-line comments (first non-whitespace = "//") and trailing inline
#   comments are stripped before pattern matching. A call that appears only in
#   a comment DOES NOT satisfy this gate. Both comment forms are explicitly
#   falsified in --self-test (arms 3a and 3b).
#
#   This base has been burned by comment confusion four times:
#     check-crlf-normalize-capture.sh         — comment defeated the gate
#     check-init-preserves-user-config.sh     — comment guard (direction 1)
#     check-init-preserves-user-config.sh     — comment write-site (direction 2)
#     check-adr-enumeration-single-point.sh   — fixture contained pattern in comment
#
# EXTRACTION WINDOW — narrow by design:
#   For each rule, the gate only inspects lines between the readFileForRule anchor
#   and the first applyRule("<rule>") after it (or the next ^func if no applyRule
#   exists inside that function). This prevents a decoy reqStatusIsDone in a
#   different rule block within the same function from satisfying the check for
#   traceid_orphan_req — which shares validateTraceId with four other rules.
#   Narrowing is falsified in --self-test arm 4 (decoy in orphan_roadmap block,
#   guard absent from orphan_req block → gate must FAIL).
#
# ANTI-VACUITY:
#   For each rule in the list, at least one non-comment source site must be found.
#   If a rule's readFileForRule call is renamed/removed, the gate fails naming the
#   missing rule. Zero sites examined = gate fails. Site count is printed on every run.
#
# TARGET FILE OVERRIDES (for --self-test, accepted via environment, NOT positional args):
#   GATE_VALIDATOR_GO       — overrides path to internal/validator/validator.go
#   GATE_VALIDATOR_TRACEID  — overrides path to internal/validator/validator_traceid.go
#   (Note: check-init-preserves-user-config.sh uses SCAFFOLD_FILE and ignores $1;
#   this gate uses the same principle with named env vars.)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VALIDATOR_GO="${GATE_VALIDATOR_GO:-$REPO_ROOT/internal/validator/validator.go}"
VALIDATOR_TRACEID="${GATE_VALIDATOR_TRACEID:-$REPO_ROOT/internal/validator/validator_traceid.go}"

# ---------------------------------------------------------------------------
# Exemptions — printed on every run for auditability
# ---------------------------------------------------------------------------
print_exemptions() {
  echo "EXEMPT req_roadmap_lifecycle — already filters via reqStatusIsOpen; only Open REQs trigger"
  echo "EXEMPT ref_targets_exist     — measures file existence (os.Stat), not link obligation; different signal"
}

# ---------------------------------------------------------------------------
# strip_comments FILENAME
# Outputs lines with: (a) full-line comments removed, (b) trailing inline
# comments stripped. Blank lines after stripping are also suppressed.
#
# Caveat (inherited from check-init-preserves-user-config.sh): sub(// ...) also
# truncates at // inside string literals (e.g. URLs). Acceptable here: a
# reqStatusIsDone or readFileForRule call following a URL on the same line is
# not a plausible real-world pattern.
# ---------------------------------------------------------------------------
strip_comments_from_file() {
  local file="$1"
  awk '
  {
    line = $0
    # Strip trailing inline comment (keeps code before //)
    sub(/[[:space:]]*\/\/.*$/, "", line)
    # Skip lines that were pure comments or are now empty
    stripped = line
    gsub(/^[[:space:]]+/, "", stripped)
    if (stripped == "") next
    print line
  }
  ' "$file"
}

# ---------------------------------------------------------------------------
# find_anchor RULE FILE
# Prints the line number of the first non-comment readFileForRule("<rule>") call.
# Returns 1 if not found.
# ---------------------------------------------------------------------------
find_anchor() {
  local rule="$1"
  local file="$2"
  awk -v rule="$rule" '
  {
    line = $0
    sub(/[[:space:]]*\/\/.*$/, "", line)
    stripped = line
    gsub(/^[[:space:]]+/, "", stripped)
    if (stripped == "") next
    if (index(line, "readFileForRule(\"" rule "\"") > 0) {
      print NR
      exit
    }
  }
  ' "$file"
}

# ---------------------------------------------------------------------------
# find_window_end RULE ANCHOR_LINE FILE
# Prints the last line of the narrow extraction window:
#   - first applyRule("<rule>") or applyRuleTagged("<rule>") after anchor, OR
#   - line before the next ^func after anchor, OR
#   - EOF.
# ---------------------------------------------------------------------------
find_window_end() {
  local rule="$1"
  local anchor="$2"
  local file="$3"

  # Try applyRule/applyRuleTagged first (tight boundary for traceid_orphan_req)
  local apply_line
  apply_line=$(awk -v rule="$rule" -v start="$anchor" '
  NR <= start { next }
  {
    line = $0
    sub(/[[:space:]]*\/\/.*$/, "", line)
    stripped = line
    gsub(/^[[:space:]]+/, "", stripped)
    if (stripped == "") next
    if (index(line, "applyRule(\"" rule "\"") > 0 || index(line, "applyRuleTagged(\"" rule "\"") > 0) {
      print NR
      exit
    }
  }
  ' "$file")

  if [ -n "$apply_line" ]; then
    echo "$apply_line"
    return 0
  fi

  # Fall back to line before next ^func
  local next_func
  next_func=$(awk -v start="$anchor" 'NR > start && /^func / { print NR; exit }' "$file")
  if [ -n "$next_func" ]; then
    echo "$((next_func - 1))"
    return 0
  fi

  # Fall back to EOF
  wc -l < "$file" | tr -d ' '
}

# ---------------------------------------------------------------------------
# check_rule RULE FILE LABEL
# Returns 0 (PASS) or 1 (FAIL). Prints a single result line.
# ---------------------------------------------------------------------------
check_rule() {
  local rule="$1"
  local file="$2"
  local label="$3"

  if [ ! -f "$file" ]; then
    echo "FAIL [anti-vacuity] rule \"$rule\": target file not found: $label"
    return 1
  fi

  # Step 1: find anchor
  local anchor
  anchor=$(find_anchor "$rule" "$file")
  if [ -z "$anchor" ]; then
    echo "FAIL [anti-vacuity] rule \"$rule\": readFileForRule(\"$rule\") not found on any non-comment line in $label"
    return 1
  fi

  # Step 2: find window end
  local wend
  wend=$(find_window_end "$rule" "$anchor" "$file")

  # Step 3: extract window, strip comments, check for reqStatusIsDone
  local found
  found=$(sed -n "${anchor},${wend}p" "$file" | awk '
  {
    line = $0
    sub(/[[:space:]]*\/\/.*$/, "", line)
    stripped = line
    gsub(/^[[:space:]]+/, "", stripped)
    if (stripped == "") next
    if (index(line, "reqStatusIsDone") > 0) { print NR; exit }
  }
  ')

  if [ -n "$found" ]; then
    echo "OK   rule \"$rule\" in $label — reqStatusIsDone on non-comment line (anchor=${label}:${anchor}, window=${anchor}-${wend}, sites=1)"
    return 0
  else
    echo "FAIL rule \"$rule\" in $label:${anchor} — window ${anchor}-${wend} has no reqStatusIsDone on non-comment line"
    return 1
  fi
}

# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------
self_test() {
  local WORK
  WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-req-roadmap-rule-status-aware-self-test.XXXXXX")
  trap 'rm -rf "$WORK"' EXIT

  local FAIL=0

  # --- Synthetic go file: req_has_roadmap WITHOUT reqStatusIsDone (arm 1 fixture) ---
  local go_no_guard="$WORK/validator_no_guard.go"
  cat > "$go_no_guard" << 'GOEOF'
package validator

func validateREQsHaveRoadmap() (enforced []string, exempt []string, scanned int, err error) {
	content, ok := readFileForRule("req_has_roadmap", "path", &enforced)
	if !ok {
		return
	}
	if !contentHasStructuredRefValue(string(content), nil) {
		enforced = append(enforced, "msg")
	}
	return enforced, exempt, scanned, nil
}
func nextFunc() {}
GOEOF

  # --- Synthetic go file: req_has_roadmap WITH reqStatusIsDone (arm 2 fixture) ---
  local go_with_guard="$WORK/validator_with_guard.go"
  cat > "$go_with_guard" << 'GOEOF'
package validator

func validateREQsHaveRoadmap() (enforced []string, exempt []string, scanned int, err error) {
	content, ok := readFileForRule("req_has_roadmap", "path", &enforced)
	if !ok {
		return
	}
	if !reqStatusIsDone(string(content)) {
		return
	}
	if !contentHasStructuredRefValue(string(content), nil) {
		enforced = append(enforced, "msg")
	}
	return enforced, exempt, scanned, nil
}
func nextFunc() {}
GOEOF

  # --- Synthetic go file: reqStatusIsDone only in TRAILING comment (arm 3a fixture) ---
  local go_trailing_comment="$WORK/validator_trailing_comment.go"
  cat > "$go_trailing_comment" << 'GOEOF'
package validator

func validateREQsHaveRoadmap() (enforced []string, exempt []string, scanned int, err error) {
	content, ok := readFileForRule("req_has_roadmap", "path", &enforced) // reqStatusIsDone
	if !contentHasStructuredRefValue(string(content), nil) {
		enforced = append(enforced, "msg")
	}
	return enforced, exempt, scanned, nil
}
func nextFunc() {}
GOEOF

  # --- Synthetic go file: readFileForRule only in FULL-LINE comment (arm 3b fixture) ---
  local go_fullline_comment="$WORK/validator_fullline_comment.go"
  cat > "$go_fullline_comment" << 'GOEOF'
package validator

func validateREQsHaveRoadmap() (enforced []string, exempt []string, scanned int, err error) {
	// content, ok := readFileForRule("req_has_roadmap", "path", &enforced)
	if !contentHasStructuredRefValue(nil, nil) {
		enforced = append(enforced, "msg")
	}
	return enforced, exempt, scanned, nil
}
func nextFunc() {}
GOEOF

  # --- Synthetic traceid file: traceid_orphan_req WITH reqStatusIsDone (correct) ---
  local traceid_correct="$WORK/traceid_correct.go"
  cat > "$traceid_correct" << 'GOEOF'
package validator

func validateTraceId() {
	var orphanReqMsgs []string
	for _, e := range reqIndex {
		content, ok := readFileForRule("traceid_orphan_req", e.path, &warnings)
		if !ok {
			continue
		}
		if !reqStatusIsDone(string(content)) {
			continue
		}
		orphanReqMsgs = append(orphanReqMsgs, "msg")
	}
	applyRule("traceid_orphan_req", orphanReqMsgs, &violations, &warnings)
}
func nextFunc() {}
GOEOF

  # --- Synthetic traceid file: DECOY reqStatusIsDone in orphan_roadmap, ABSENT from orphan_req ---
  # This arm proves the extraction window is narrow (not whole-function).
  # A gate that checked the whole validateTraceId function would find the decoy and
  # report PASS. The narrow window makes it correctly FAIL.
  local traceid_decoy="$WORK/traceid_decoy.go"
  cat > "$traceid_decoy" << 'GOEOF'
package validator

func validateTraceId() {
	var orphanRoadmapMsgs []string
	for _, e := range roadmapIndex {
		content, ok := readFileForRule("traceid_orphan_roadmap", e.path, &warnings)
		if !ok {
			continue
		}
		if !reqStatusIsDone(string(content)) { // decoy: guard is here, NOT in orphan_req below
			continue
		}
		orphanRoadmapMsgs = append(orphanRoadmapMsgs, "msg")
	}
	applyRule("traceid_orphan_roadmap", orphanRoadmapMsgs, &violations, &warnings)

	var orphanReqMsgs []string
	for _, e := range reqIndex {
		content, ok := readFileForRule("traceid_orphan_req", e.path, &warnings)
		if !ok {
			continue
		}
		// reqStatusIsDone intentionally absent — this is the defect being detected
		orphanReqMsgs = append(orphanReqMsgs, "msg")
	}
	applyRule("traceid_orphan_req", orphanReqMsgs, &violations, &warnings)
}
func nextFunc() {}
GOEOF

  # ---- Arm 1: req_has_roadmap missing guard → gate FAILS, names the rule ----
  local arm1_out arm1_rc
  arm1_rc=0
  arm1_out=$(GATE_VALIDATOR_GO="$go_no_guard" GATE_VALIDATOR_TRACEID="$traceid_correct" \
    bash "$SCRIPT_DIR/check-req-roadmap-rule-is-status-aware.sh" 2>&1) || arm1_rc=$?
  if [ "$arm1_rc" -ne 0 ] && echo "$arm1_out" | grep -q 'req_has_roadmap'; then
    echo "OK   [self-test/arm1] gate FAILs naming req_has_roadmap (no guard): rc=$arm1_rc"
  else
    echo "FAIL [self-test/arm1] expected FAIL naming req_has_roadmap; got rc=$arm1_rc"
    echo "     output: $arm1_out"
    FAIL=1
  fi

  # ---- Arm 2: both rules have guard → gate PASSES ----
  local arm2_out arm2_rc
  arm2_rc=0
  arm2_out=$(GATE_VALIDATOR_GO="$go_with_guard" GATE_VALIDATOR_TRACEID="$traceid_correct" \
    bash "$SCRIPT_DIR/check-req-roadmap-rule-is-status-aware.sh" 2>&1) || arm2_rc=$?
  if [ "$arm2_rc" -eq 0 ]; then
    echo "OK   [self-test/arm2] gate PASSes when both rules have guard: rc=$arm2_rc"
  else
    echo "FAIL [self-test/arm2] expected PASS; got rc=$arm2_rc"
    echo "     output: $arm2_out"
    FAIL=1
  fi

  # ---- Arm 3a: reqStatusIsDone only in TRAILING comment → gate FAILS ----
  # This is the primary comment-direction falsification.
  # (check-init-preserves-user-config.sh burned here: comment guard satisfied the check)
  local arm3a_out arm3a_rc
  arm3a_rc=0
  arm3a_out=$(GATE_VALIDATOR_GO="$go_trailing_comment" GATE_VALIDATOR_TRACEID="$traceid_correct" \
    bash "$SCRIPT_DIR/check-req-roadmap-rule-is-status-aware.sh" 2>&1) || arm3a_rc=$?
  if [ "$arm3a_rc" -ne 0 ] && echo "$arm3a_out" | grep -q 'req_has_roadmap'; then
    echo "OK   [self-test/arm3a] trailing-comment reqStatusIsDone correctly rejected: rc=$arm3a_rc"
  else
    echo "FAIL [self-test/arm3a] expected FAIL for trailing-comment-only reqStatusIsDone; got rc=$arm3a_rc"
    echo "     output: $arm3a_out"
    FAIL=1
  fi

  # ---- Arm 3b: readFileForRule only in FULL-LINE comment → anti-vacuity FAILS ----
  # This is the second comment direction: rule name in comment → anchor not found.
  local arm3b_out arm3b_rc
  arm3b_rc=0
  arm3b_out=$(GATE_VALIDATOR_GO="$go_fullline_comment" GATE_VALIDATOR_TRACEID="$traceid_correct" \
    bash "$SCRIPT_DIR/check-req-roadmap-rule-is-status-aware.sh" 2>&1) || arm3b_rc=$?
  if [ "$arm3b_rc" -ne 0 ] && echo "$arm3b_out" | grep -qi 'anti-vacuity\|not found'; then
    echo "OK   [self-test/arm3b] full-line-comment readFileForRule triggers anti-vacuity: rc=$arm3b_rc"
  else
    echo "FAIL [self-test/arm3b] expected anti-vacuity FAIL for commented readFileForRule; got rc=$arm3b_rc"
    echo "     output: $arm3b_out"
    FAIL=1
  fi

  # ---- Arm 4: NARROWING — decoy reqStatusIsDone before anchor → gate FAILS ----
  # Proves the extraction window is narrow (anchor→applyRule, not whole function).
  # A whole-function check would find the decoy in orphan_roadmap and report PASS.
  # The narrow window correctly FAILs because only the orphan_req block is checked.
  local arm4_out arm4_rc
  arm4_rc=0
  arm4_out=$(GATE_VALIDATOR_GO="$go_with_guard" GATE_VALIDATOR_TRACEID="$traceid_decoy" \
    bash "$SCRIPT_DIR/check-req-roadmap-rule-is-status-aware.sh" 2>&1) || arm4_rc=$?
  if [ "$arm4_rc" -ne 0 ] && echo "$arm4_out" | grep -q 'traceid_orphan_req'; then
    echo "OK   [self-test/arm4] narrowing is load-bearing: decoy in orphan_roadmap block does not satisfy orphan_req check: rc=$arm4_rc"
  else
    echo "FAIL [self-test/arm4] expected FAIL for decoy-before-anchor (narrowing check); got rc=$arm4_rc"
    echo "     output: $arm4_out"
    FAIL=1
  fi

  echo ""
  if [ "$FAIL" -ne 0 ]; then
    echo "check-req-roadmap-rule-is-status-aware --self-test: FAIL"
    exit 1
  fi
  echo "check-req-roadmap-rule-is-status-aware --self-test: OK (4 arms pass)"
  exit 0
}

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
RUN_SELF_TEST=0
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test)
      RUN_SELF_TEST=1
      shift
      ;;
    *)
      echo "check-req-roadmap-rule-is-status-aware: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [ "$RUN_SELF_TEST" -eq 1 ]; then
  self_test
fi

# ---------------------------------------------------------------------------
# Main scan
# ---------------------------------------------------------------------------
echo "=== check-req-roadmap-rule-is-status-aware ==="
echo ""
echo "Rule list (closed — 2 rules under contract):"
echo "  req_has_roadmap      → $VALIDATOR_GO"
echo "  traceid_orphan_req   → $VALIDATOR_TRACEID"
echo ""
print_exemptions
echo ""

FAIL=0
SITES=0

# Rule 1: req_has_roadmap in validator.go
if check_rule "req_has_roadmap" "$VALIDATOR_GO" "validator.go"; then
  SITES=$((SITES + 1))
else
  FAIL=1
  SITES=$((SITES + 1))
fi

# Rule 2: traceid_orphan_req in validator_traceid.go
if check_rule "traceid_orphan_req" "$VALIDATOR_TRACEID" "validator_traceid.go"; then
  SITES=$((SITES + 1))
else
  FAIL=1
  SITES=$((SITES + 1))
fi

echo ""
echo "Sites examined: $SITES / 2"
echo ""

if [ "$FAIL" -ne 0 ]; then
  echo "check-req-roadmap-rule-is-status-aware: FAIL"
  exit 1
fi
echo "check-req-roadmap-rule-is-status-aware: OK"
exit 0
