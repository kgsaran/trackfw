#!/usr/bin/env bash
# verify-npm-channels.sh — verify npm shim + 6 platform packages are live after publish.
#
# Retry with exponential backoff and a hard deadline.
# Measured 2026-09-14 (rc2 release): packages propagated staggered; all 6 live after 717s.
# Deadline is 900s (717s + ~26% margin). Exhausting the deadline is a FAIL — never a pass.
#
# Message format distinguishes two root causes:
#   "not propagated within deadline" — publish reported success but CDN not caught up
#   "publish failed"                 — the upstream publish job did not succeed
# These require different actions and must not be conflated.
#
# Env vars (can be set by workflow via needs context):
#   PUBLISH_NPM_RESULT  — result of the publish-npm-shim job ("success"|"failure"|"skipped"|"cancelled")
#   VERIFY_DEADLINE     — override deadline in seconds (default: 900)
#   VERIFY_POLL_INITIAL — initial poll interval in seconds (default: 30)
#   VERIFY_POLL_MAX     — max poll interval in seconds (default: 60)
#
# Exit codes: 0 = all live; 1 = any package not confirmed live within deadline or publish failed.
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Usage: verify-npm-channels.sh <version>" >&2
  exit 1
fi

# --- Configurable parameters -------------------------------------------------
# Deadline: 900s measured against 717s real propagation (+26% margin, 2026-09-14).
DEADLINE="${VERIFY_DEADLINE:-900}"
POLL_INTERVAL="${VERIFY_POLL_INITIAL:-30}"
POLL_MAX="${VERIFY_POLL_MAX:-60}"

PUBLISH_NPM_RESULT="${PUBLISH_NPM_RESULT:-}"

START_TS=$(date +%s)

# --- Check publish-side outcome first ----------------------------------------
# If the upstream job failed or was skipped, the package may not exist at all.
# Report that as a distinct cause from CDN propagation delay.
if [ -n "$PUBLISH_NPM_RESULT" ] && [ "$PUBLISH_NPM_RESULT" != "success" ]; then
  echo "FAIL: npm publish job result: '$PUBLISH_NPM_RESULT' — packages may not have been published" >&2
  echo "  Action: check publish-npm-shim / publish-npm-platforms job logs for the root cause." >&2
  exit 1
fi

# --- Package list to verify --------------------------------------------------
SHIM_PKG="trackfw"
declare -a PLATFORM_SLUGS=(linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64 win32-arm64)

# --- Verify a single package via the registry JSON API -----------------------
# Uses the registry JSON API, not `npm view`, to avoid npm's local HTTP cache returning
# a stale cache miss inside a retry loop. Measured endpoint:
#   https://registry.npmjs.org/@trackfw-bin%2f<slug>/<version>
#
# Exit 0 if the version is live; exit 1 if 404 or network error.
check_pkg() {
  local pkg="$1"
  local ver="$2"
  local encoded
  # URL-encode the scope separator '/' → '%2f' (replace all occurrences)
  encoded="${pkg//\//%2f}"
  local url="https://registry.npmjs.org/${encoded}/${ver}"
  # --fail: exit non-zero on HTTP error (including 404); --silent: no progress meter
  if curl --fail --silent --max-time 15 "$url" -o /dev/null 2>/dev/null; then
    return 0
  fi
  return 1
}

# --- Retry loop: only re-poll missing packages each round --------------------
declare -a STILL_MISSING=()
# Initialise with all platform slugs; shim is checked in the first round below.

SHIM_LIVE=false

echo "Verifying npm shim and platform packages for trackfw@$VERSION"
echo "Deadline: ${DEADLINE}s (measured propagation: 717s on 2026-09-14 rc2 release)"

while true; do
  NOW=$(date +%s)
  ELAPSED=$(( NOW - START_TS ))

  if [ "$ELAPSED" -ge "$DEADLINE" ]; then
    echo "" >&2
    echo "FAIL: deadline exhausted after ${ELAPSED}s — the following npm package(s) did not" >&2
    echo "      propagate within ${DEADLINE}s. This is a CDN propagation timeout, NOT a" >&2
    echo "      confirmed publish failure. Check the registry manually:" >&2
    if ! $SHIM_LIVE; then
      echo "  - trackfw@${VERSION}" >&2
    fi
    for slug in "${STILL_MISSING[@]+"${STILL_MISSING[@]}"}"; do
      echo "  - @trackfw-bin/${slug}@${VERSION}" >&2
    done
    echo "" >&2
    echo "  Real elapsed: ${ELAPSED}s (deadline: ${DEADLINE}s)" >&2
    echo "  If packages are live now, the deadline was too tight for today's CDN conditions." >&2
    exit 1
  fi

  # Check shim if not yet confirmed
  if ! $SHIM_LIVE; then
    if check_pkg "$SHIM_PKG" "$VERSION"; then
      echo "ok:   npm trackfw@$VERSION is live  (+${ELAPSED}s)"
      SHIM_LIVE=true
    else
      echo "wait: npm trackfw@$VERSION not yet live  (+${ELAPSED}s)"
    fi
  fi

  # Determine the set of platform slugs to check this round:
  # first round = all slugs; subsequent rounds = only the still-missing ones.
  declare -a TO_CHECK=()
  if [ "${FIRST_ROUND_DONE:-false}" = "false" ]; then
    FIRST_ROUND_DONE=true
    TO_CHECK=("${PLATFORM_SLUGS[@]}")
  else
    # ${STILL_MISSING[@]+"..."} expands to elements if non-empty, to nothing if empty.
    # This is safe under set -u (unlike [@] or [@]:-) and does not produce ghost elements.
    TO_CHECK=(${STILL_MISSING[@]+"${STILL_MISSING[@]}"})
  fi

  declare -a NEXT_MISSING=()
  for slug in "${TO_CHECK[@]+"${TO_CHECK[@]}"}"; do
    if check_pkg "@trackfw-bin/${slug}" "$VERSION"; then
      echo "ok:   npm @trackfw-bin/${slug}@${VERSION} is live  (+${ELAPSED}s)"
    else
      echo "wait: npm @trackfw-bin/${slug}@${VERSION} not yet live  (+${ELAPSED}s)"
      NEXT_MISSING+=("$slug")
    fi
  done
  # Replace STILL_MISSING with the packages that are still not live.
  STILL_MISSING=(${NEXT_MISSING[@]+"${NEXT_MISSING[@]}"})

  # Success: shim + all platform packages confirmed
  if $SHIM_LIVE && [ "${#STILL_MISSING[@]}" -eq 0 ]; then
    NOW=$(date +%s)
    ELAPSED=$(( NOW - START_TS ))
    echo ""
    echo "ok: all npm packages live after ${ELAPSED}s"
    exit 0
  fi

  # Sleep with backoff before next round, but don't sleep past the deadline
  REMAINING=$(( DEADLINE - ELAPSED ))
  ACTUAL_SLEEP="$POLL_INTERVAL"
  if [ "$ACTUAL_SLEEP" -gt "$REMAINING" ]; then
    ACTUAL_SLEEP="$REMAINING"
  fi
  if [ "$ACTUAL_SLEEP" -gt 0 ]; then
    echo "  polling again in ${ACTUAL_SLEEP}s..."
    sleep "$ACTUAL_SLEEP"
  fi
  # Backoff: double interval up to max
  POLL_INTERVAL=$(( POLL_INTERVAL * 2 ))
  if [ "$POLL_INTERVAL" -gt "$POLL_MAX" ]; then
    POLL_INTERVAL="$POLL_MAX"
  fi
done
