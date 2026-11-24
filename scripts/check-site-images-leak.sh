#!/usr/bin/env bash
# check-site-images-leak.sh — scan every file in site/public/img/ with `strings`
# for personal data: home paths, email addresses (non-allowlisted), and tokens.
#
# WHAT THIS DETECTS
#   - Absolute home paths: /Users/<x>, /home/<x>, C:\Users\<x>
#   - Git/GitHub credential tokens: ghp_, gho_, github_pat_, Bearer
#   - Email addresses with common real TLDs (.com .io .dev .org .net .sh .app
#     .co .me .edu .gov .br .de .fr .uk .nl .us .ca .au) — 2-letter ccTLDs and
#     well-known gTLDs. The .invalid TLD used by demo@example.invalid is 7 chars
#     and does NOT match the [a-z]{2,6} suffix constraint.
#
# WHAT THIS CANNOT DETECT
#   - Rasterized text in image pixels (requires OCR — e.g. Tesseract)
#   - Text inside video/gif frames encoded as pixel data
#   The control for those is the isolated generation environment (C1 in the
#   threat model): PS1='$ ', HOME isolated, GIT_AUTHOR/COMMITTER identity set.
#
# ALLOWLIST
#   github.com/kgsaran/trackfw — project URL, not a personal path
#   demo@example.invalid        — .invalid TLD (7 chars) is outside the email
#                                 pattern's [a-z]{2,6} range; passes naturally
#
# SELF-TEST (always runs before the real scan)
#   Canary A: file containing /Users/x — scanner must FAIL it
#             (asserts: scanner catches absolute home paths)
#   Canary B: file containing github.com/kgsaran/trackfw only — scanner must PASS it
#             (asserts: allowlist works — project URL is not a leak)
#   Canary C: file containing both /Users/x and github.com/kgsaran/trackfw — must FAIL
#             (asserts: allowlist strips the URL substring, not the whole line)
#   Canary D: file containing demo@example.invalid — scanner must PASS it
#             (asserts: .invalid TLD, 7 chars, is outside [a-z]{2,6}; not caught)
#   Canary E: file containing someone@example.com — scanner must FAIL it
#             (asserts: real-TLD email is detected as a potential personal data leak)
#   Vacuity : zero files in img/ — scanner must FAIL
#
# Linked to: scripts/site-images/generate.sh (called after image generation)
#
# USAGE
#   bash scripts/check-site-images-leak.sh           # scan site/public/img/
#   bash scripts/check-site-images-leak.sh --self-test

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
IMG_DIR="$REPO_ROOT/site/public/img"

# Patterns that constitute a leak (after allowlist substitution)
# Note: | grep -q is prohibited by check-no-grep-q-under-pipefail.sh;
# use | grep ... >/dev/null instead.
#
# Email pattern: [local]@[domain].[explicit-real-tld]
#   - Uses an explicit TLD list (not a character-count range) to avoid matching
#     prefixes of longer pseudo-TLDs like .invalid.
#   - Covers common developer/personal email TLDs: com io dev org net sh app
#     co me edu gov br de fr uk nl us ca au info biz tech cloud ai
#   - Does NOT match .invalid — not in the list by design
#   - Allowlist (github.com/kgsaran/trackfw) has no @ so it is unaffected
_EMAIL_TLDS='com|io|dev|org|net|sh|app|co|me|edu|gov|br|de|fr|uk|nl|us|ca|au|info|biz|tech|cloud|ai'
# Base patterns in single quotes (preserves backslashes for C:\Users\)
_LEAK_BASE='/Users/[a-zA-Z]|/home/[a-zA-Z]|C:\\Users\\[a-zA-Z]|ghp_[A-Za-z0-9]+|gho_[A-Za-z0-9]+|github_pat_[A-Za-z0-9]+|Bearer [A-Za-z0-9]'
# Email pattern appended via concatenation; \. is literal dot in ERE
LEAK_RE="${_LEAK_BASE}|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.(${_EMAIL_TLDS})"

# The allowlist substitution string: remove this substring before checking
ALLOWLIST_SUB='github.com/kgsaran/trackfw'

# ---------------------------------------------------------------------------
# scan_file: scan a single file for leaks.
# Returns 0 if clean, 1 if a leak is found.
# ---------------------------------------------------------------------------
scan_file() {
  local f="$1"
  local raw

  # strings: extracts printable character sequences from binary files
  # || true: strings exits non-zero if the file is empty; don't abort
  raw=$( { strings "$f" || true; } )

  # Remove the allowlisted project URL before checking
  # (sed with global replacement on the entire string)
  local cleaned
  cleaned=$(printf '%s\n' "$raw" | sed "s|${ALLOWLIST_SUB}||g")

  if printf '%s\n' "$cleaned" | grep -E "$LEAK_RE" >/dev/null 2>&1; then
    return 1  # leak found
  fi
  return 0
}

# ---------------------------------------------------------------------------
# self_test: run canary checks BEFORE scanning real files.
# A broken scanner must abort before touching any real file.
# ---------------------------------------------------------------------------
self_test() {
  local tmpdir
  tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-leak-selftest.XXXXXX")
  local q
  printf -v q '%q' "$tmpdir"
  trap "rm -rf $q" EXIT

  local pass=0 fail=0

  # Canary A — /Users/x must FAIL (asserts: scanner catches absolute home paths)
  printf '/Users/testuser/project/secret\n' > "$tmpdir/canary-a.txt"
  if scan_file "$tmpdir/canary-a.txt"; then
    echo "SELF-TEST FAIL: canary A (/Users/x) was NOT caught — scanner is broken" >&2
    fail=$((fail + 1))
  else
    echo "  SELF-TEST OK: canary A (/Users/x) correctly caught"
    pass=$((pass + 1))
  fi

  # Canary B — github.com/kgsaran/trackfw only must PASS (asserts: allowlist works)
  printf 'github.com/kgsaran/trackfw\n' > "$tmpdir/canary-b.txt"
  if scan_file "$tmpdir/canary-b.txt"; then
    echo "  SELF-TEST OK: canary B (allowlisted URL) correctly passed"
    pass=$((pass + 1))
  else
    echo "SELF-TEST FAIL: canary B (allowlisted URL) was incorrectly caught" >&2
    fail=$((fail + 1))
  fi

  # Canary C — both /Users/x and allowlisted URL on same content must FAIL
  # (asserts: allowlist removes the substring, not the entire line)
  printf 'github.com/kgsaran/trackfw /Users/testuser/secret\n' > "$tmpdir/canary-c.txt"
  if scan_file "$tmpdir/canary-c.txt"; then
    echo "SELF-TEST FAIL: canary C (URL + /Users/x) was NOT caught — allowlist filters whole lines" >&2
    fail=$((fail + 1))
  else
    echo "  SELF-TEST OK: canary C (URL + /Users/x) correctly caught"
    pass=$((pass + 1))
  fi

  # Canary D — demo@example.invalid must PASS (.invalid = 7 chars, outside [a-z]{2,6})
  printf 'demo@example.invalid\n' > "$tmpdir/canary-d.txt"
  if scan_file "$tmpdir/canary-d.txt"; then
    echo "  SELF-TEST OK: canary D (demo@example.invalid) correctly passed"
    pass=$((pass + 1))
  else
    echo "SELF-TEST FAIL: canary D (demo@example.invalid) was incorrectly caught" >&2
    fail=$((fail + 1))
  fi

  # Canary E — someone@example.com must FAIL (real TLD, potential personal data)
  printf 'someone@example.com\n' > "$tmpdir/canary-e.txt"
  if scan_file "$tmpdir/canary-e.txt"; then
    echo "SELF-TEST FAIL: canary E (someone@example.com) was NOT caught — email detection broken" >&2
    fail=$((fail + 1))
  else
    echo "  SELF-TEST OK: canary E (someone@example.com) correctly caught"
    pass=$((pass + 1))
  fi

  echo "  Self-test: $pass passed, $fail failed"

  if [ "$fail" -gt 0 ]; then
    echo "ERROR: self-test failed — aborting real scan" >&2
    return 1
  fi
  return 0
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
MODE="${1:-scan}"

if [ "$MODE" = "--self-test" ]; then
  echo "=== check-site-images-leak.sh: self-test only ==="
  self_test
  echo "check-site-images-leak.sh: self-test passed"
  exit 0
fi

echo "=== check-site-images-leak.sh: running self-test before real scan ==="
self_test

echo ""
echo "=== check-site-images-leak.sh: scanning $IMG_DIR ==="

# Vacuity guard: refuse to report clean if no files exist
if [ ! -d "$IMG_DIR" ]; then
  echo "ERROR: $IMG_DIR does not exist — no files to scan" >&2
  exit 1
fi

FILES=()
while IFS= read -r f; do
  FILES+=("$f")
done < <(find "$IMG_DIR" -type f ! -name ".gitkeep" | sort)

if [ "${#FILES[@]}" -eq 0 ]; then
  echo "ERROR: no files found in $IMG_DIR (vacuity guard) — run generate.sh first" >&2
  exit 1
fi

echo "  Scanning ${#FILES[@]} file(s)..."

FAIL=0
for f in "${FILES[@]}"; do
  bname="$(basename "$f")"
  if scan_file "$f"; then
    echo "  OK   $bname"
  else
    echo "  LEAK $bname"
    FAIL=$((FAIL + 1))
  fi
done

echo ""
if [ "$FAIL" -gt 0 ]; then
  echo "check-site-images-leak.sh: FAIL — $FAIL file(s) contain potential leaks"
  exit 1
fi

echo "check-site-images-leak.sh: OK — all ${#FILES[@]} file(s) clean"
exit 0
