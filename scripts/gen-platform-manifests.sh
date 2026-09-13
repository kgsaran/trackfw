#!/usr/bin/env bash
# gen-platform-manifests.sh — generates @trackfw-bin/<platform>/package.json artefacts
# from the single version source in internal/version/version.go.
#
# Output: build/npm-platform/@trackfw-bin/<platform>/package.json  (gitignored)
#
# These are build artefacts consumed by the release workflow to publish platform packages.
# They are NEVER committed — committing them would recreate the #338 problem (N+1 version
# sites to keep in sync). Wave 3 (ML-3A) removes pypi/trackfw/; after that, this script and
# internal/version/version.go are the only version authorities.
#
# Platforms match the goreleaser build matrix (.goreleaser.yaml).
#
# Vacuity guard: if the PLATFORMS array is empty or zero manifests are written, exits 1.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION_FILE="$REPO_ROOT/internal/version/version.go"
if [[ ! -f "$VERSION_FILE" ]]; then
    echo "gen-platform-manifests: $VERSION_FILE not found" >&2
    exit 1
fi

# Extract version from `var Version = "X.Y.Z"` using awk — portable across BSD and GNU.
VERSION=$(awk -F'"' '/^var Version/ { print $2 }' "$VERSION_FILE")
if [[ -z "$VERSION" ]]; then
    echo "gen-platform-manifests: could not extract version from $VERSION_FILE" >&2
    exit 1
fi

OUT_DIR="$REPO_ROOT/build/npm-platform/@trackfw-bin"

# Platform triples: <slug>:<os>:<cpu>:<binary-name>
# On Windows the binary carries the .exe extension; all others do not.
PLATFORMS=(
    "linux-x64:linux:x64:trackfw"
    "linux-arm64:linux:arm64:trackfw"
    "darwin-x64:darwin:x64:trackfw"
    "darwin-arm64:darwin:arm64:trackfw"
    "win32-x64:win32:x64:trackfw.exe"
    "win32-arm64:win32:arm64:trackfw.exe"
)

COUNT=0
for entry in "${PLATFORMS[@]}"; do
    IFS=: read -r slug os_name cpu_name binary_name <<<"$entry"
    pkg_dir="$OUT_DIR/$slug"
    mkdir -p "$pkg_dir/bin"
    cat >"$pkg_dir/package.json" <<EOF
{
  "name": "@trackfw-bin/$slug",
  "version": "$VERSION",
  "description": "trackfw binary for $os_name $cpu_name",
  "os": ["$os_name"],
  "cpu": ["$cpu_name"],
  "files": ["bin/$binary_name"]
}
EOF
    COUNT=$((COUNT + 1))
done

# Vacuity guard: zero manifests means the PLATFORMS list was empty or all iterations failed.
if [[ "$COUNT" -eq 0 ]]; then
    echo "gen-platform-manifests: no manifests generated — PLATFORMS list is empty" >&2
    exit 1
fi

echo "gen-platform-manifests: generated $COUNT platform manifests for v$VERSION in $OUT_DIR"
