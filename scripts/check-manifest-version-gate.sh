#!/usr/bin/env bash
# check-manifest-version-gate.sh — gate for AC3 + partial #338 resolution.
#
# Asserts three things in sequence:
#   1. gen-platform-manifests.sh runs without error (generation happened).
#   2. Every generated @trackfw-bin/<platform>/package.json carries the version from
#      internal/version/version.go — not a hand-written number.
#   3. That version matches the latest versioned section in CHANGELOG.md — the cross-check
#      that issue #338 identified as missing from the pre-release pipeline.
#
# Vacuity guard: zero manifests found after generation → reprova.
# This gate never passes silently: every verification emits ok/FAIL, and a non-zero FAIL
# count forces exit 1.
#
# Reconciliation (inviolável — CLAUDE.md regra dura):
#   - Assertion 1 proves: gen-platform-manifests.sh ran and produced output.
#   - Assertion 2 proves: generated manifests carry the same version as Go's single source.
#   - Assertion 3 proves: the Go version and CHANGELOG top section agree — the #338 check
#     now fires at gate time, not only at 'trackfw release tag' runtime.
set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0

ok()   { echo "ok: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL + 1)); }

# ── Step 1: Generate manifests ─────────────────────────────────────────────
if ! "$SCRIPT_DIR/gen-platform-manifests.sh"; then
    fail "gen-platform-manifests.sh failed — cannot verify manifests"
    echo ""
    echo "manifest-version-gate: $PASS passed, $FAIL failed"
    exit 1
fi

# ── Extract Go version (single source of truth) ────────────────────────────
GO_VERSION=$(awk -F'"' '/^var Version/ { print $2 }' "$REPO_ROOT/internal/version/version.go")
if [[ -z "$GO_VERSION" ]]; then
    fail "could not extract version from internal/version/version.go"
    echo ""
    echo "manifest-version-gate: $PASS passed, $FAIL failed"
    exit 1
fi

# ── Step 2: Verify each generated manifest's version ──────────────────────
OUT_DIR="$REPO_ROOT/build/npm-platform/@trackfw-bin"
if [[ ! -d "$OUT_DIR" ]]; then
    fail "vacuity guard: $OUT_DIR does not exist after generation"
    echo ""
    echo "manifest-version-gate: $PASS passed, $FAIL failed"
    exit 1
fi

MANIFEST_COUNT=0
while IFS= read -r -d '' pkg_json; do
    MANIFEST_COUNT=$((MANIFEST_COUNT + 1))
    manifest_version=$(python3 -c "
import json, sys
try:
    d = json.load(open(sys.argv[1]))
    print(d.get('version', ''))
except Exception as e:
    print('', end='')
    sys.exit(1)
" "$pkg_json" 2>/dev/null) || manifest_version=""
    if [[ -z "$manifest_version" ]]; then
        fail "$pkg_json: could not read version field"
        continue
    fi
    if [[ "$manifest_version" != "$GO_VERSION" ]]; then
        fail "$pkg_json: version is \"$manifest_version\", expected \"$GO_VERSION\" (from internal/version/version.go)"
    else
        ok "$pkg_json: version $manifest_version matches Go source"
    fi
done < <(find "$OUT_DIR" -name "package.json" -print0 | sort -z)

# Vacuity guard: generation claimed success but no manifests are readable.
if [[ "$MANIFEST_COUNT" -eq 0 ]]; then
    fail "vacuity guard: no package.json files found under $OUT_DIR after generation"
    echo ""
    echo "manifest-version-gate: $PASS passed, $FAIL failed"
    exit 1
fi

# ── Step 3: Verify version matches CHANGELOG.md top section (#338) ─────────
CHANGELOG="$REPO_ROOT/CHANGELOG.md"
if [[ ! -f "$CHANGELOG" ]]; then
    fail "CHANGELOG.md not found at $CHANGELOG"
else
    # Extract first '## [X.Y.Z] - YYYY-MM-DD' line; skip '## [Unreleased]' if present.
    CHANGELOG_VERSION=$(grep '^## \[' "$CHANGELOG" | grep -v '^\#\# \[Unreleased\]' | head -1 | sed 's/^## \[\([^]]*\)\].*/\1/')
    if [[ -z "$CHANGELOG_VERSION" ]]; then
        fail "CHANGELOG.md has no '## [X.Y.Z]' section (excluding [Unreleased])"
    elif [[ "$CHANGELOG_VERSION" != "$GO_VERSION" ]]; then
        fail "CHANGELOG.md top section is [$CHANGELOG_VERSION] but internal/version/version.go is \"$GO_VERSION\" — update one before release"
    else
        ok "CHANGELOG.md top section [$CHANGELOG_VERSION] matches Go source v$GO_VERSION"
    fi
fi

echo ""
echo "manifest-version-gate: $PASS passed, $FAIL failed"
if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
