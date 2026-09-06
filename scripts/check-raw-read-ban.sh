#!/usr/bin/env bash
# check-raw-read-ban.sh — anti-reintroduction gate for
# ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1G.
#
# Why this exists: ML-1G swept internal/validator/*.go (Go), npm/src/validator/index.js (Node) and
# pypi/trackfw/validator.py (Python) so that every config/governance-artifact read goes through a
# fail-safe helper (readFileForRule/readRegularFile in Go, readFileForRule/readRegularFileSync in
# Node, _read_file_for_rule/_read_regular_file in Python) instead of a raw os.ReadFile /
# fs.readFileSync / open() — which either hangs forever on a FIFO, aborts `trackfw validate`
# entirely on a non-ENOENT error, or silently skips the file (the whole "fail-open" class this REQ
# exists to close). This gate is what stops the NEXT ml from reintroducing a raw call one site at a
# time, unnoticed — which is exactly how the original 26 sites accumulated.
#
# Design: ban the raw primitive in each validator entrypoint file, with a small, explicit,
# inline-justified allowlist for the handful of sites that are legitimately raw (the fail-safe
# helper's OWN implementation, which must call the primitive once to build the abstraction; a WRITE,
# which is a different concern than the read class this gate bans). A raw match is accepted ONLY if
# the offending line OR the line immediately above it carries the literal marker
# "raw-read-allowed:" followed by a reason — an allowlist that can't drift silently, because
# appending a line without a reason fails the gate.
#
# Non-negotiable per this REQ's history of vacuous gates: if a target file scans to zero raw AND
# zero routed occurrences (i.e. the file is empty, missing, or the scan produced no lines at all),
# this gate FAILS LOUD instead of reporting a false "0 raw sites" pass.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

FAIL=0
SCANNED_ANY=0

# check_file <label> <pattern (portable ERE — no PCRE lookbehind: BSD grep on macOS has no -P)> <file> [file...]
# Scans each file for raw-primitive occurrences. Every match must carry "raw-read-allowed:" on its
# own line or the line directly above; anything else is an unjustified raw read → FAIL.
# Vacuity is guarded at the AGGREGATE level (total lines scanned across the whole file set for a
# runtime), not per file — a runtime's validator logic is legitimately split across many files, some
# small (e.g. Go's regularfile_windows.go, 33 lines); failing each one individually would be a false
# positive, not a real vacuity signal. Per-runtime minimum: 200 lines total.
check_file() {
  local label="$1" pattern="$2"
  shift 2
  local file
  local runtime_total_lines=0
  for file in "$@"; do
    if [ ! -s "$file" ]; then
      echo "FAIL [$label] $file is missing or empty — vacuous scan, refusing to report a silent pass"
      FAIL=1
      continue
    fi
    local total_lines
    total_lines=$(wc -l < "$file")
    runtime_total_lines=$((runtime_total_lines + total_lines))

    local matches
    matches=$(grep -anE "$pattern" "$file" || true)
    if [ -z "$matches" ]; then
      echo "OK   [$label] 0 raw sites in $file ($total_lines lines scanned)"
      continue
    fi

    local line lineno content prevno prevline justified
    while IFS= read -r line; do
      lineno="${line%%:*}"
      content="${line#*:}"
      prevno=$((lineno - 1))
      prevline=""
      if [ "$prevno" -ge 1 ]; then
        prevline=$(sed -n "${prevno}p" "$file")
      fi
      justified=0
      if printf '%s' "$content" | grep -q "raw-read-allowed:"; then
        justified=1
      elif printf '%s' "$prevline" | grep -q "raw-read-allowed:"; then
        justified=1
      fi
      if [ "$justified" -eq 1 ]; then
        echo "OK   [$label] raw site at $file:$lineno — justified inline"
      else
        echo "FAIL [$label] unjustified raw read at $file:$lineno: $(printf '%s' "$content" | sed 's/^[[:space:]]*//')"
        FAIL=1
      fi
    done <<< "$matches"
  done

  if [ "$runtime_total_lines" -lt 200 ]; then
    echo "FAIL [$label] only $runtime_total_lines total lines scanned across all files — vacuous-scan guard tripped"
    FAIL=1
  else
    SCANNED_ANY=1
  fi
}

echo "=== check-raw-read-ban: Go (internal/validator/*.go, non-test) ==="
GO_FILES=$(find internal/validator -maxdepth 1 -name '*.go' ! -name '*_test.go')
check_file "go" '\<os\.ReadFile\(' $GO_FILES

echo
echo "=== check-raw-read-ban: Node (npm/src/validator/index.js) ==="
# grep -a is MANDATORY here: `file(1)` classifies this file as binary (Unicode text with certain
# byte sequences trips libmagic's heuristic), so a plain `grep` silently scans ZERO lines and always
# reports "clean" no matter what raw calls are reintroduced — this exact trap already produced two
# false-premise REQs earlier in this campaign.
check_file "node" '\<fs\.readFileSync\(' npm/src/validator/index.js

echo
echo "=== check-raw-read-ban: Python (pypi/trackfw/validator.py) ==="
# Matches only STATEMENT-POSITION open( — preceded by "with", "=" or "return" plus whitespace, i.e.
# actual code that reads a file — not `os.open(`/`fdopen(` (different call) and not prose in a
# docstring/comment that merely mentions "open(...)" (e.g. "replaces open(path, ...).read()"),
# which this pattern does NOT match. Earlier version of this gate used a lookbehind-shaped standalone-
# open( pattern that DID match prose, and the fix was to reword 7 archaeological ML-1B/1C comments
# from "open(" to "open (" just to dodge the regex — backwards: it broke prose to satisfy the gate,
# and the next agent writing "open(" in a comment hits the same wall. Anchoring on statement shape
# instead makes prose a structural non-match, no rewording needed; the declared residual is a raw
# `open(` nested inside another call with no assignment/with/return on the same line (e.g.
# `foo(open(p))`), which this pattern would miss — not exercised by this codebase, called out here
# rather than silently accepted.
# Written without a (?<!...) lookbehind on purpose: macOS's BSD grep has NO -P flag at all (verified
# live: `grep -P` errors "invalid option -- P" and, worse, the `|| true` around the grep call in
# check_file swallowed that error silently, reporting "0 raw sites" no matter what the file actually
# contained — a vacuous gate on the exact runtime this campaign already got bitten by twice for a
# different reason). -E is portable to both BSD and GNU grep and needs no lookbehind for this shape.
check_file "python" '(with|=|return)[[:space:]]+open\(' pypi/trackfw/validator.py

echo
echo "=== vacuity guard ==="
if [ "$SCANNED_ANY" -ne 1 ]; then
  echo "FAIL vacuity guard: no file was actually scanned end to end — the gate would silently pass on nothing"
  FAIL=1
else
  echo "OK   at least one file was scanned with a non-trivial line count in all 3 runtimes"
fi

echo
if [ "$FAIL" -ne 0 ]; then
  echo "check-raw-read-ban: FAIL"
  exit 1
fi
echo "check-raw-read-ban: OK — no unjustified raw reads in internal/validator, npm/src/validator/index.js, pypi/trackfw/validator.py"
exit 0
