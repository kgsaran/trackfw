#!/usr/bin/env bash
# check-platform-matrix-parity.sh — asserts that .goreleaser.yaml build matrix and
# scripts/gen-platform-manifests.sh PLATFORMS array are identical.
#
# D2 (ML-1A v8 ROADMAP-2026-09-12-v8-um-binario-muitos-canais): the two lists diverged —
# gen-platform-manifests.sh listed 6 slugs including win32-arm64, but .goreleaser.yaml had
# ignore: {goos: windows, goarch: arm64} and built only 5. Publishing @trackfw-bin/win32-arm64
# without a binary in it is permanent and produces a silent runtime failure (shim resolves
# the package then fails on spawnSync, instead of aborting with "no package for your platform").
#
# Source A — .goreleaser.yaml: cross-product goos × goarch minus ignore[], translated to npm
#            slug convention (amd64→x64, windows→win32). Parsed with yaml.safe_load.
# Source B — gen-platform-manifests.sh PLATFORMS array: slugs already in npm format (first
#            colon-separated field of each entry). Extracted with awk — no YAML needed.
#
# Comparison: sorted sets. Any item in A∖B or B∖A is a named failure.
# Vacuity guard: either list resolving to zero items → named failure, never silent pass.
#
# Reconciliation (Regra Dura — CLAUDE.md):
#   - Assertion "A == B": the two authoring sources agree on which binaries exist.
#     If they diverge, either goreleaser will publish a binary with no npm package, or
#     an npm package will be published with no binary in it — both are permanent failures.
#
# Falsification:
#   --falsify-goreleaser  Re-inserts ignore:{goos:windows, goarch:arm64} → FAIL naming win32-arm64
#   --falsify-manifest    Adds a 7th entry (fake-arm64) to PLATFORMS → FAIL naming fake-arm64
#
# Bash 3.2 compatible (macOS ships bash 3.2.57): no mapfile, no declare -A.
set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$SCRIPT_DIR/lib-crlf-normalize.sh"

GORELEASER_YAML="$REPO_ROOT/.goreleaser.yaml"
GEN_SCRIPT="$REPO_ROOT/scripts/gen-platform-manifests.sh"

PASS=0
FAIL=0
ok()   { echo "ok: $1";    PASS=$((PASS+1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL+1)); }

# ── Self-test (falsification) ───────────────────────────────────────────────
WORK=""
cleanup() { [[ -n "$WORK" ]] && rm -rf "$WORK" || true; }
trap cleanup EXIT

if [[ "${1:-}" == "--falsify-goreleaser" ]]; then
    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-platform-matrix-parity.XXXXXX")
    # Copy real files into temp dir with the ignore block re-inserted
    cp "$GEN_SCRIPT" "$WORK/gen-platform-manifests.sh"
    python3 - "$GORELEASER_YAML" "$WORK/.goreleaser.yaml" <<'PYEOF'
import sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)
# Re-insert the ignore block
for b in doc.get('builds', []):
    b['ignore'] = [{'goos': 'windows', 'goarch': 'arm64'}]
with open(sys.argv[2], 'w') as f:
    yaml.dump(doc, f, default_flow_style=False)
PYEOF
    # Run ourselves against the modified files
    GORELEASER_YAML="$WORK/.goreleaser.yaml"
    GEN_SCRIPT="$WORK/gen-platform-manifests.sh"
    echo "--- falsify-goreleaser: running check against modified .goreleaser.yaml ---"
    # Note: we continue below and expect a FAIL for win32-arm64
fi

if [[ "${1:-}" == "--falsify-manifest" ]]; then
    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-platform-matrix-parity.XXXXXX")
    cp "$GORELEASER_YAML" "$WORK/.goreleaser.yaml"
    # Add a 7th entry to PLATFORMS
    sed 's/^PLATFORMS=($/PLATFORMS=(/' "$GEN_SCRIPT" > "$WORK/gen-platform-manifests.sh.tmp"
    # Insert a fake entry before the closing paren of PLATFORMS
    awk '/^    "win32-arm64:win32:arm64:trackfw.exe"/ {
        print
        print "    \"fake-arm64:fake:arm64:trackfw\""
        next
    } { print }' "$GEN_SCRIPT" > "$WORK/gen-platform-manifests.sh"
    chmod +x "$WORK/gen-platform-manifests.sh"
    GORELEASER_YAML="$WORK/.goreleaser.yaml"
    GEN_SCRIPT="$WORK/gen-platform-manifests.sh"
    echo "--- falsify-manifest: running check against modified gen-platform-manifests.sh ---"
fi

# ── Source A: parse .goreleaser.yaml ──────────────────────────────────────
if ! python3 -c "import yaml" 2>/dev/null; then
    echo "check-platform-matrix-parity: python3 yaml module not available — skip" >&2
    echo "check-platform-matrix-parity: $PASS passed, $FAIL failed (skipped — yaml unavailable)"
    exit 0
fi

GORELEASER_SLUGS=$(python3 - "$GORELEASER_YAML" <<'PYEOF' | strip_cr
import sys, yaml

GOOS_TO_NPM  = {'linux': 'linux', 'darwin': 'darwin', 'windows': 'win32'}
GOARCH_TO_NPM = {'amd64': 'x64', 'arm64': 'arm64'}

with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)

slugs = []
for build in doc.get('builds', []):
    goos_list  = build.get('goos', [])
    goarch_list = build.get('goarch', [])
    ignores    = {(e.get('goos', ''), e.get('goarch', ''))
                  for e in build.get('ignore', [])}
    for goos in goos_list:
        for goarch in goarch_list:
            if (goos, goarch) in ignores:
                continue
            npm_os   = GOOS_TO_NPM.get(goos)
            npm_arch = GOARCH_TO_NPM.get(goarch)
            if not npm_os or not npm_arch:
                print(f"check-platform-matrix-parity: unknown translation for {goos}/{goarch}", file=sys.stderr)
                sys.exit(1)
            slugs.append(f"{npm_os}-{npm_arch}")

for s in sorted(slugs):
    print(s)
PYEOF
)

if [[ -z "$GORELEASER_SLUGS" ]]; then
    fail "vacuity guard: .goreleaser.yaml produced zero platform slugs — check builds section"
    echo ""
    echo "check-platform-matrix-parity: $PASS passed, $FAIL failed"
    exit 1
fi

GORELEASER_COUNT=$(echo "$GORELEASER_SLUGS" | wc -l | tr -d ' ')
ok ".goreleaser.yaml: parsed $GORELEASER_COUNT platform slugs"

# ── Source B: extract PLATFORMS from gen-platform-manifests.sh ──────────
# Each PLATFORMS entry is of the form "<slug>:<os>:<cpu>:<binary>" — first field is the slug.
MANIFEST_SLUGS=$(awk '
    /^PLATFORMS=\(/ { in_array=1; next }
    in_array && /^\)/ { in_array=0; next }
    in_array {
        # Strip leading whitespace and quotes
        line = $0
        gsub(/^[[:space:]]*"/, "", line)
        gsub(/".*$/, "", line)
        if (length(line) == 0) next
        n = split(line, parts, ":")
        if (n >= 1) print parts[1]
    }
' "$GEN_SCRIPT" | sort)

if [[ -z "$MANIFEST_SLUGS" ]]; then
    fail "vacuity guard: gen-platform-manifests.sh PLATFORMS array produced zero slugs"
    echo ""
    echo "check-platform-matrix-parity: $PASS passed, $FAIL failed"
    exit 1
fi

MANIFEST_COUNT=$(echo "$MANIFEST_SLUGS" | wc -l | tr -d ' ')
ok "gen-platform-manifests.sh: parsed $MANIFEST_COUNT platform slugs"

# ── Compare: A \ B (in goreleaser but not in manifests) ──────────────────
FAIL_COUNT_BEFORE=$FAIL
while IFS= read -r slug; do
    if ! echo "$MANIFEST_SLUGS" | grep -qxF "$slug"; then
        fail "goreleaser builds \"$slug\" but gen-platform-manifests.sh does not list it — add it to PLATFORMS"
    fi
done <<EOF
$GORELEASER_SLUGS
EOF

# ── Compare: B \ A (in manifests but not in goreleaser) ──────────────────
while IFS= read -r slug; do
    if ! echo "$GORELEASER_SLUGS" | grep -qxF "$slug"; then
        fail "gen-platform-manifests.sh lists \"$slug\" but .goreleaser.yaml does not build it — publishing this package would have no binary"
    fi
done <<EOF
$MANIFEST_SLUGS
EOF

if [[ "$FAIL_COUNT_BEFORE" -eq "$FAIL" ]]; then
    ok "platform lists agree: $GORELEASER_COUNT slugs match in both sources"
fi

echo ""
echo "check-platform-matrix-parity: $PASS passed, $FAIL failed"
if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
