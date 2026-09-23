#!/usr/bin/env bash
# check-crlf-normalize-capture.sh — anti-reintroduction gate for ML-1B / ML-1C
# (ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md)
#
# WHY THIS EXISTS:
#   After ML-1A corrected 19 python3 capture sites by routing them through
#   strip_cr(), this gate prevents the 20th site from being introduced without
#   normalization.  On Windows, python3 translates \n → \r\n in stdout (text
#   mode).  A bash capture that does not pipe through strip_cr receives values
#   with a trailing \r — the failure mode that silently broke the Windows
#   census (issue #353).
#
# STRUCTURAL DISCRIMINANT (census ML-0A, Seção 2):
#   Category (a) = three simultaneous conditions:
#     1. Capture  — bash receives python3 stdout as data via $(python3 ...) or
#                   $("$PY_BIN" ...) / $($PY_BIN ...)
#     2. Emission — Python emits \n (print(), writelines, sys.stdout.write+\n,
#                   os.linesep)
#     3. No norm  — nothing strips \r between python3 and subsequent use
#
#   The gate exempts a capture when condition 1 or 2 is FALSE — computed
#   from the source text, not asserted by the author:
#
#   Condition 1 fails (not an interpreter invocation):
#     $(command -v python3) — captures the binary path, not program output.
#
#   Condition 2 fails (no newline emitter in block):
#     Block contains none of: print(  writelines  os.linesep
#                             sys.stdout.write( followed by \n in argument
#     Covers sys.stdout.write(hashlib/base64 data) which never emits \n.
#     (Measured 2026-09-23: check-agent-models-parity.sh:927,928 and
#     check-thirdparty-parity.sh:84,85,87 match this exemption.)
#
# FORMS COVERED BY DISCRIMINANT (condition 1):
#   $(python3 ...)            — direct literal invocation (primary)
#   $("$PY_BIN" ...)          — invocation via PY_BIN (double-quoted form)
#   $($PY_BIN ...)            — invocation via PY_BIN (unquoted form)
#
# NON-COVERED FORMS (declared residuals — "not measured" is not acceptable;
#                    these are measured and the limit is documented):
#   < <(python3 ...)          — process substitution feeds a file descriptor,
#                               not a shell variable; the one existing site
#                               (check-no-literal-nul-in-source.sh) was fixed
#                               inside the helper function; a new direct site
#                               would require a second scanner pass.
#   $(func_calling_python3)   — indirect capture via helper function; the four
#                               known helpers (check_field_json,
#                               normalize_barrier_json, target_ids_json,
#                               doc_check_json) were corrected in ML-1C to
#                               normalize internally; detecting a new helper
#                               that calls python3 without normalizing requires
#                               call-graph analysis beyond static line scanning.
#   $("${PY_BIN}" ...)        — brace-quoted PY_BIN; not in use; the unquoted
#                               and double-quoted forms cover all existing sites.
#   eval "$CMD" (python3)     — eval expansion; no existing sites.
#
# NON-VACUITY:
#   Floor re-measured 2026-09-23 (ML-1C) after extending discriminant to cover
#   $("$PY_BIN" ...) / $($PY_BIN ...) forms. Two new candidates from
#   check-gates-falsify.sh (PROSE_PAYLOAD and T65_BIG_PAYLOAD) now appear in
#   the scan; both have strip_cr confirmed.
#   Command: CRLF_GATE_MIN_CAPTURES=0 bash scripts/check-crlf-normalize-capture.sh
#   Result: 68 candidates, 6 exempt (5 condition-2 + 1 condition-1).
#   MIN_CAPTURES set to 50 (≈74% of measured — guards against empty-corpus
#   silent pass). Previous floor was 50 at 66 candidates (≈76%); floor is
#   unchanged — still comfortably above the 74% threshold.
#   Override with CRLF_GATE_MIN_CAPTURES env var (set to 1 in synthetic trees).
#
# DIAGNOSTIC STRINGS (must stay distinct — assert_fails_with matches on them):
#   Violation : "python3 capture without strip_cr"
#   Vacuity   : "vacuity guard tripped"
#
# SELF-REFERENCE:
#   This script is excluded from its own scan — see "Skip self" below.
#   The gate's own source references patterns as ERE sequences (\$\(), not as
#   the literal $(python3 text the scanner looks for.

set -uo pipefail

# Dead mention, not an invocation: `python3` appears below only in ERE patterns
# and comments. This export satisfies check-output-encoding-declared.sh whose
# discriminant is intentionally file-level and conservative (trade-off
# documented there, lines ~207-212).
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Configuration (overridable for synthetic-corpus tests)
# ---------------------------------------------------------------------------
SCAN_ROOT="${CRLF_GATE_SCAN_ROOT:-$REPO_ROOT}"
MIN_CAPTURES="${CRLF_GATE_MIN_CAPTURES:-50}"

# How many lines ahead to look when strip_cr is not on the opening line.
# All known multi-line patterns in this repo close within 12 lines.
LOOKAHEAD_LINES=20

FAIL=0
TOTAL_CANDIDATES=0
TOTAL_EXEMPT=0
SCANNED_FILES=0

# ---------------------------------------------------------------------------
# PY_DETECT_RE: matches command substitutions that invoke python3 or $PY_BIN.
#   Forms covered: $(python3 ...) | $("$PY_BIN" ...) | $($PY_BIN <space>...)
#   Stored in a variable so ERE alternation (|) works in [[ =~ ]] without
#   quoting the variable — bash 4+ guarantees this behaviour.
# ---------------------------------------------------------------------------
PY_DETECT_RE='\$\([^\)]*python3|\$\("?\$PY_BIN[" ]'

# ---------------------------------------------------------------------------
# scan_file <file>
#   Reads the file into an array and walks every non-comment line looking for
#   $(python3 ...) or $("$PY_BIN" ...) patterns. For each opening found:
#     · Applies exemption conditions 1 and 2.
#     · If not exempt: requires strip_cr within the block (same line or
#       LOOKAHEAD_LINES ahead). Missing strip_cr → FAIL.
#   Exhausting the lookahead window without finding strip_cr also → FAIL.
# ---------------------------------------------------------------------------
scan_file() {
  local file="$1"
  local -a lines
  mapfile -t lines < "$file"
  local n=${#lines[@]}
  local fname
  fname=$(basename "$file")
  local i=0

  while [ $i -lt $n ]; do
    local line="${lines[$i]}"
    local lineno=$((i + 1))

    # Skip comment lines (first non-whitespace char is #)
    if [[ "$line" =~ ^[[:space:]]*# ]]; then
      ((i++))
      continue
    fi

    # Detect opening of a python3 command substitution (direct or via $PY_BIN).
    # PY_DETECT_RE covers:
    #   $(python3 ...)         — literal invocation
    #   $("$PY_BIN" ...)       — PY_BIN double-quoted
    #   $($PY_BIN <space>...)  — PY_BIN unquoted (trailing space required to
    #                            avoid matching variable names like $PY_BINARY)
    if ! [[ "$line" =~ $PY_DETECT_RE ]]; then
      ((i++))
      continue
    fi

    # Condition 1 exemption: $(command -v python3) — not an interpreter call.
    if [[ "$line" =~ \$\(command[[:space:]]+-v[[:space:]]+python3 ]]; then
      echo "OK   [exempt/condition-1/$fname:$lineno] command -v: not an interpreter invocation"
      ((TOTAL_EXEMPT++))
      ((i++))
      continue
    fi

    # Collect the block: opening line + up to LOOKAHEAD_LINES following lines.
    ((TOTAL_CANDIDATES++))
    local block=""
    local j=$i
    local limit=$((i + LOOKAHEAD_LINES))
    [ $limit -ge $n ] && limit=$((n - 1))
    while [ $j -le $limit ]; do
      block+="${lines[$j]}"$'\n'
      ((j++))
    done

    # Primary check: strip_cr in block → normalized (structural proof).
    # This check runs BEFORE condition-2 exemption so that captures that use
    # Python code in a variable ($STRIP_TS etc.) are not incorrectly exempted
    # (their block lacks literal "print(" but strip_cr is still in the pipeline).
    if grep -qF 'strip_cr' <<<"$block"; then
      echo "OK   [$fname:$lineno] python3 capture normalized (strip_cr in block)"
      ((i++))
      continue
    fi

    # Condition 2 exemption: no newline emitter in block.
    # Only reached when strip_cr is NOT in the block.
    # If block contains none of: print(  writelines  os.linesep
    #                            sys.stdout.write( with \n in argument
    # → condition 2 fails → Python cannot produce \r\n → exempt.
    # Covers sys.stdout.write(hashlib/base64 data) which never emits \n.
    # NOTE: sys.stdout.write('x\n') IS a newline emitter — detected by the
    # ERE 'sys\.stdout\.write\(.*\\n' (literal backslash-n in argument).
    local emits_newline=1
    if ! grep -qF 'print(' <<<"$block" && \
       ! grep -qF 'writelines' <<<"$block" && \
       ! grep -qF 'os.linesep' <<<"$block" && \
       ! grep -qE 'sys\.stdout\.write\(.*\\n' <<<"$block"; then
      emits_newline=0
    fi

    if [ $emits_newline -eq 0 ]; then
      echo "OK   [exempt/condition-2/$fname:$lineno] no newline emitter (print(/writelines/os.linesep/sys.stdout.write+\\n absent in block)"
      ((TOTAL_EXEMPT++))
      ((i++))
      continue
    fi

    # No strip_cr, emits newline → violation.
    echo "FAIL [$fname:$lineno] python3 capture without strip_cr: $line"
    FAIL=1

    ((i++))
  done

  ((SCANNED_FILES++))
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --scan-root)
      SCAN_ROOT="$2"
      shift 2
      ;;
    *)
      echo "check-crlf-normalize-capture: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

echo "=== check-crlf-normalize-capture: scanning scripts/*.sh under $SCAN_ROOT ==="
echo ""

for f in "$SCAN_ROOT/scripts/"*.sh; do
  [ -e "$f" ] || continue
  # Skip self to avoid scanning this gate's own source code.
  [ "$(basename "$f")" = "check-crlf-normalize-capture.sh" ] && continue
  scan_file "$f"
done

# ---------------------------------------------------------------------------
# Vacuity guard
# ---------------------------------------------------------------------------
echo ""
echo "=== vacuity guard ==="
echo "Candidates (conditions 1+2 true): $TOTAL_CANDIDATES"
echo "Exempt     (condition 1 or 2 false): $TOTAL_EXEMPT"
echo "Files scanned: $SCANNED_FILES"
echo "MIN_CAPTURES floor: $MIN_CAPTURES"

if [ "$TOTAL_CANDIDATES" -lt "$MIN_CAPTURES" ]; then
  echo "FAIL vacuity guard tripped: $TOTAL_CANDIDATES candidate(s) found, minimum floor is $MIN_CAPTURES — corpus may be empty or pruned; set CRLF_GATE_MIN_CAPTURES to override for synthetic trees"
  FAIL=1
else
  echo "OK   vacuity: $TOTAL_CANDIDATES >= $MIN_CAPTURES"
fi

echo ""
if [ "$FAIL" -ne 0 ]; then
  echo "check-crlf-normalize-capture: FAIL"
  exit 1
fi
echo "check-crlf-normalize-capture: OK — all python3 captures normalized via strip_cr"
exit 0
