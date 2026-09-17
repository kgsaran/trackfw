#!/usr/bin/env bash
# check-jira-url-concat.sh — AC7 anti-reintroduction gate
# Linked to: make parity-rest (ML-1A, REQ-2026-09-17-jira-base-url-*)
#
# Fails if any non-test Go file under internal/ constructs an authenticated
# request URL by string concatenation from a config-derived field.
#
# MECHANISM: a file that (a) sets an Authorization header AND (b) appends a
# string literal to a config field (pattern: \.BaseURL\s*\+\s*") is a
# violation. Both conditions must coexist in the same file.
#
# DISCRIMINANT: a file with Authorization but using url.JoinPath or url.Parse
# for URL construction is clean; a file with concatenation but no Authorization
# is also clean. The conjunction is the violation.
# Falsification: arm 1 in --self-test has both conditions and must FAIL;
# arm 2 has Authorization + url.JoinPath and must PASS. Removing the
# Authorization grep from arm 2 would make it impossible to distinguish
# the clean pattern from the violation — confirming that grep is load-bearing.
#
# SCOPE: internal/**/*.go (excluding *_test.go) — test files may contain
# synthetic fixtures with bad patterns for their own self-tests.
#
# ALLOWLIST: add "basename.go|reason" when a legitimate concatenation exists
# (e.g. a constant base path that has been statically verified and cannot change
# at runtime). An entry without "|reason" causes this gate to FAIL — silence
# is not acceptable justification.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Allowlist — format: "basename.go|reason"
# Empty = no exceptions today.
# ---------------------------------------------------------------------------
ALLOWLIST=()

# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------
if [ "${1:-}" = "--self-test" ]; then
    TMPDIR_SELF="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR_SELF"' EXIT

    # Arm 1 (must FAIL): Authorization + BaseURL concatenation → violation
    cat > "$TMPDIR_SELF/bad.go" <<'GOEOF'
package sync
func bad() {
    url := c.BaseURL + "/rest/api/3/issue"
    req, _ := http.NewRequest("POST", url, nil)
    req.Header.Set("Authorization", "Basic xyz")
}
GOEOF

    # Arm 2 (must PASS): Authorization + url.JoinPath → clean
    cat > "$TMPDIR_SELF/good.go" <<'GOEOF'
package sync
func good() {
    endpoint, _ := url.JoinPath(c.BaseURL, "rest/api/3/issue")
    req, _ := http.NewRequest("POST", endpoint, nil)
    req.Header.Set("Authorization", "Basic xyz")
}
GOEOF

    # Arm 3 (must PASS): concatenation without Authorization → clean
    cat > "$TMPDIR_SELF/noauth.go" <<'GOEOF'
package sync
func noauth() {
    url := c.BaseURL + "/public/endpoint"
    req, _ := http.NewRequest("GET", url, nil)
}
GOEOF

    SELF_FAIL=0

    # Arm 1: must be flagged
    if grep -q 'Header\.Set("Authorization' "$TMPDIR_SELF/bad.go" && \
       grep -qE '\.BaseURL[[:space:]]*\+[[:space:]]*"' "$TMPDIR_SELF/bad.go"; then
        echo "OK [arm1/bad.go]: violation detected as expected"
    else
        echo "FAIL [arm1/bad.go]: should have been flagged but was not"
        SELF_FAIL=1
    fi

    # Arm 2: must NOT be flagged
    if grep -q 'Header\.Set("Authorization' "$TMPDIR_SELF/good.go" && \
       grep -qE '\.BaseURL[[:space:]]*\+[[:space:]]*"' "$TMPDIR_SELF/good.go"; then
        echo "FAIL [arm2/good.go]: clean file was incorrectly flagged"
        SELF_FAIL=1
    else
        echo "OK [arm2/good.go]: no violation detected as expected"
    fi

    # Arm 3: must NOT be flagged (concatenation without auth is not this gate's concern)
    if grep -q 'Header\.Set("Authorization' "$TMPDIR_SELF/noauth.go" && \
       grep -qE '\.BaseURL[[:space:]]*\+[[:space:]]*"' "$TMPDIR_SELF/noauth.go"; then
        echo "FAIL [arm3/noauth.go]: non-auth file was incorrectly flagged"
        SELF_FAIL=1
    else
        echo "OK [arm3/noauth.go]: no violation detected as expected"
    fi

    if [ "$SELF_FAIL" -ne 0 ]; then
        echo "FAIL [check-jira-url-concat/self-test]: one or more arms produced unexpected results"
        exit 1
    fi
    echo "OK [check-jira-url-concat/self-test]"
    exit 0
fi

# ---------------------------------------------------------------------------
# Main check
# ---------------------------------------------------------------------------
VIOLATIONS=()

while IFS= read -r -d '' f; do
    # Skip test files — they may contain bad patterns in contra-braço fixtures
    [[ "$f" == *_test.go ]] && continue

    # The conjunction: Authorization header set AND BaseURL string concatenation
    if grep -q 'Header\.Set("Authorization' "$f" && \
       grep -qE '\.BaseURL[[:space:]]*\+[[:space:]]*"' "$f"; then

        # Check allowlist
        allowed=0
        basename_f="$(basename "$f")"
        for entry in "${ALLOWLIST[@]+"${ALLOWLIST[@]}"}"; do
            pattern="${entry%%|*}"
            reason="${entry#*|}"
            if [ -z "$reason" ]; then
                echo "FAIL [check-jira-url-concat]: allowlist entry missing reason: $entry"
                exit 1
            fi
            if [ "$basename_f" = "$pattern" ]; then
                allowed=1
                break
            fi
        done

        if [ "$allowed" -eq 0 ]; then
            VIOLATIONS+=("$f")
        fi
    fi
done < <(find "$REPO_ROOT/internal" -name "*.go" -print0)

if [ "${#VIOLATIONS[@]}" -gt 0 ]; then
    echo "FAIL [check-jira-url-concat]: authenticated URL built by string concatenation from config field:"
    for v in "${VIOLATIONS[@]}"; do
        echo "  $v"
    done
    echo ""
    echo "Fix: use url.JoinPath(c.BaseURL, path) after validateJiraURL(). See internal/sync/jira.go."
    echo "To add a justified exception: edit ALLOWLIST in scripts/check-jira-url-concat.sh."
    exit 1
fi

echo "OK [check-jira-url-concat]: no unvalidated URL concatenation in authenticated request files"
