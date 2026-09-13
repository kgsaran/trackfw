#!/usr/bin/env bash
# check-manifest-version-gate.sh — gate for AC3 + full #338 resolution.
#
# Asserts four things in sequence:
#   1. gen-platform-manifests.sh runs without error (generation happened).
#   2. Every generated @trackfw-bin/<platform>/package.json carries the version from
#      internal/version/version.go — not a hand-written number.
#   3. That version matches the latest versioned section in CHANGELOG.md — the cross-check
#      that issue #338 identified as missing from the pre-release pipeline.
#   4. npm/package.json and pypi/pyproject.toml agree with internal/version/version.go.
#
# WHY SOME FILES ARE GENERATED AND OTHERS ARE MONITORED:
#   - The 6 @trackfw-bin/<platform>/package.json manifests are generated (not committed)
#     because their sole purpose is to declare the binary path per platform; they carry
#     no other information. Generating them from the Go source eliminates drift by
#     construction and costs nothing at runtime.
#   - npm/package.json and pypi/pyproject.toml MUST remain hand-written in the tree:
#     `npm ci` reads npm/package.json directly and cannot work from a generated file;
#     package metadata (description, keywords, scripts, devDependencies) lives alongside
#     the version field and is meaningfully maintained by humans. Generating these files
#     would break `npm ci` and obscure intentional metadata edits. The right answer for
#     these two is continuous monitoring, not generation.
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
#   - Assertion 4 proves: npm/package.json and pypi/pyproject.toml agree with version.go —
#     the two hand-written version sites that remain after Wave 3 are now continuously
#     monitored, closing issue #338 fully.
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

# ── Step 4: Verify hand-written version sites agree with version.go (#338) ──
# npm/package.json and pypi/pyproject.toml must be kept in sync with
# internal/version/version.go by the developer. This gate enforces that
# agreement continuously rather than only at 'trackfw release tag' runtime.

NPM_PKG="$REPO_ROOT/npm/package.json"
if [[ ! -f "$NPM_PKG" ]]; then
    fail "npm/package.json not found at $NPM_PKG"
else
    NPM_VERSION=$(python3 -c "
import json, sys
try:
    d = json.load(open(sys.argv[1]))
    print(d.get('version', ''))
except Exception:
    sys.exit(1)
" "$NPM_PKG" 2>/dev/null) || NPM_VERSION=""
    if [[ -z "$NPM_VERSION" ]]; then
        fail "npm/package.json: could not read version field"
    elif [[ "$NPM_VERSION" != "$GO_VERSION" ]]; then
        fail "npm/package.json: version is \"$NPM_VERSION\", expected \"$GO_VERSION\" (from internal/version/version.go)"
    else
        ok "npm/package.json: version $NPM_VERSION matches Go source"
    fi
fi

PYPROJECT="$REPO_ROOT/pypi/pyproject.toml"
if [[ ! -f "$PYPROJECT" ]]; then
    fail "pypi/pyproject.toml not found at $PYPROJECT"
else
    PYPI_VERSION=$(awk -F'"' '/^version[[:space:]]*=/ { print $2; exit }' "$PYPROJECT")
    if [[ -z "$PYPI_VERSION" ]]; then
        fail "pypi/pyproject.toml: could not read version field"
    elif [[ "$PYPI_VERSION" != "$GO_VERSION" ]]; then
        fail "pypi/pyproject.toml: version is \"$PYPI_VERSION\", expected \"$GO_VERSION\" (from internal/version/version.go)"
    else
        ok "pypi/pyproject.toml: version $PYPI_VERSION matches Go source"
    fi
fi

# ── Step 5: Conditional check for pypi/trackfw/__init__.py literals ──────────
# WHY CONDITIONAL:
#   This file carries two hardcoded version literals (the importlib.metadata
#   fallback and the bare except fallback). Wave 3 (ML-3A) of the v8 roadmap
#   deletes the entire pypi/trackfw/ directory. Between now and Wave 3 execution
#   those literals are not covered by any other gate, creating a window where a
#   version bump could leave them stale. A plain check would fail when Wave 3
#   runs; a missing check leaves a silent window.
#
#   The conditional pattern resolves both without future maintenance:
#   - File present: verify both literals against internal/version/version.go.
#   - File absent: print an informational line (not silent) and continue.
#   The informational else-branch is mandatory — a missing else is
#   indistinguishable from a check that never ran, which is the failure mode
#   this project has seen in four other gates.
#
#   Other gates should use this same pattern for any file that Wave 3 removes:
#   guard with [ -f ], emit a named informational message in the else branch,
#   and document which wave makes the file absent.
#
# Reconciliation:
#   - Arm 1 (file present, literal matches): both hardcoded literals agree with version.go.
#   - Arm 2 (file present, literal diverges): a version bump did not update __init__.py.
#   - Arm 3 (file absent): Wave 3 has run; this check is inapplicable; gate continues.

INIT_PY="$REPO_ROOT/pypi/trackfw/__init__.py"
if [[ -f "$INIT_PY" ]]; then
    # Extract the try-branch fallback: __version__ = version("trackfw") or "X.Y.Z"
    INIT_TRY_VERSION=$(awk -F'"' '/version\("trackfw"\)[[:space:]]*or[[:space:]]*"/ { print $4; exit }' "$INIT_PY")
    # Extract the except-branch literal: __version__ = "X.Y.Z"
    INIT_EXCEPT_VERSION=$(awk -F'"' '/^[[:space:]]*__version__[[:space:]]*=[[:space:]]*"[0-9]/ { print $2; exit }' "$INIT_PY")

    if [[ -z "$INIT_TRY_VERSION" ]]; then
        fail "pypi/trackfw/__init__.py: could not extract try-branch fallback literal"
    elif [[ "$INIT_TRY_VERSION" != "$GO_VERSION" ]]; then
        fail "pypi/trackfw/__init__.py: try-branch fallback is \"$INIT_TRY_VERSION\", expected \"$GO_VERSION\" (from internal/version/version.go)"
    else
        ok "pypi/trackfw/__init__.py: try-branch fallback \"$INIT_TRY_VERSION\" matches Go source"
    fi

    if [[ -z "$INIT_EXCEPT_VERSION" ]]; then
        fail "pypi/trackfw/__init__.py: could not extract except-branch literal"
    elif [[ "$INIT_EXCEPT_VERSION" != "$GO_VERSION" ]]; then
        fail "pypi/trackfw/__init__.py: except-branch literal is \"$INIT_EXCEPT_VERSION\", expected \"$GO_VERSION\" (from internal/version/version.go)"
    else
        ok "pypi/trackfw/__init__.py: except-branch literal \"$INIT_EXCEPT_VERSION\" matches Go source"
    fi
else
    echo "info: pypi/trackfw/__init__.py absent — Wave 3 (ML-3A) has run; __init__.py literals check does not apply"
fi

echo ""
echo "manifest-version-gate: $PASS passed, $FAIL failed"
if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
