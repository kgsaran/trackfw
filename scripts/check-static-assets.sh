#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CANONICAL="$ROOT_DIR/internal/serve/static"
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-static-assets.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT

if [[ ! -d "$CANONICAL" ]]; then
  echo "Static assets: canonical source missing: $CANONICAL" >&2
  exit 1
fi

# P1: derive the file list from the canonical source — never hardcode asset names.
# This ensures that adding a new static asset to internal/serve/static is
# automatically caught if it is not mirrored to npm and pypi.
(cd "$CANONICAL" && find . -maxdepth 1 -type f -print | LC_ALL=C sort) \
  > "$TMP_ROOT/canonical-files"

# P2 vacuity guard: an empty canonical list would make every destination look
# synchronized — silence that is worse than a failure.
if [[ ! -s "$TMP_ROOT/canonical-files" ]]; then
  echo "Static assets: no files found in canonical source $CANONICAL" >&2
  exit 1
fi

check_destination() {
  local destination=$1
  local label="${destination#"$ROOT_DIR"/}"

  if [[ ! -d "$destination" ]]; then
    echo "Static asset destination missing: $label" >&2
    echo "Canonical source: $CANONICAL" >&2
    return 1
  fi

  # Build destination file list with the same format for a clean diff.
  (cd "$destination" && find . -maxdepth 1 -type f -print | LC_ALL=C sort) \
    > "$TMP_ROOT/destination-files"

  # Check both directions: missing files AND extra files in the destination
  # are both caught by the diff below.
  if ! diff -u "$TMP_ROOT/canonical-files" "$TMP_ROOT/destination-files" >&2; then
    echo "Static asset file-list drift in $label" >&2
    echo "Run scripts/sync-integration-assets.sh (or manually mirror changes)" >&2
    return 1
  fi

  # Byte-for-byte content check for every file that exists in both.
  while IFS= read -r relative; do
    relative="${relative#./}"
    if ! cmp -s "$CANONICAL/$relative" "$destination/$relative"; then
      echo "Static asset byte drift: $label/$relative" >&2
      echo "Canonical source: internal/serve/static/$relative" >&2
      return 1
    fi
  done < "$TMP_ROOT/canonical-files"
}

check_destination "$ROOT_DIR/npm/src/serve/static"
check_destination "$ROOT_DIR/pypi/trackfw/serve/static"

echo "Static assets are synchronized"
