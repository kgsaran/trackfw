#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SOURCE="$ROOT_DIR/internal/integrations/assets"
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-assets-check.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT HUP INT TERM

if [ ! -f "$SOURCE/catalog.json" ]; then
  echo "Canonical integration assets are missing: internal/integrations/assets" >&2
  exit 1
fi

(cd "$SOURCE" && find . -type f -print | LC_ALL=C sort) > "$TMP_ROOT/canonical-files"

check_destination() {
  destination=$1
  label=${destination#"$ROOT_DIR"/}
  if [ ! -d "$destination" ]; then
    echo "Integration asset destination is missing: $label" >&2
    echo "Run scripts/sync-integration-assets.sh" >&2
    return 1
  fi

  (cd "$destination" && find . -type f -print | LC_ALL=C sort) > "$TMP_ROOT/destination-files"
  if ! diff -u "$TMP_ROOT/canonical-files" "$TMP_ROOT/destination-files"; then
    echo "Integration asset file-list drift detected in $label" >&2
    echo "Run scripts/sync-integration-assets.sh" >&2
    return 1
  fi

  while IFS= read -r relative; do
    relative=${relative#./}
    if ! cmp -s "$SOURCE/$relative" "$destination/$relative"; then
      echo "Integration asset byte drift: $label/$relative" >&2
      echo "Canonical checksum: $(cksum < "$SOURCE/$relative")" >&2
      echo "Generated checksum: $(cksum < "$destination/$relative")" >&2
      echo "Run scripts/sync-integration-assets.sh" >&2
      return 1
    fi
  done < "$TMP_ROOT/canonical-files"
}

check_destination "$ROOT_DIR/npm/src/integrations/assets"
check_destination "$ROOT_DIR/pypi/trackfw/integrations/assets"

if ! grep -Fq '"src/"' "$ROOT_DIR/npm/package.json"; then
  echo "npm package files must include src/ so integration assets are published" >&2
  exit 1
fi
for pattern in \
  '"integrations/assets/catalog.json"' \
  '"integrations/assets/agents/*.md"' \
  '"integrations/assets/skills/*.md"'
do
  if ! grep -Fq "$pattern" "$ROOT_DIR/pypi/pyproject.toml"; then
    echo "Python package-data is missing required pattern: $pattern" >&2
    exit 1
  fi
done
echo "Integration assets are synchronized (file lists and bytes match)"
