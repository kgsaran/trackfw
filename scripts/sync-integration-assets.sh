#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SOURCE="$ROOT_DIR/internal/integrations/assets"

if [ ! -f "$SOURCE/catalog.json" ]; then
  echo "Canonical integration assets are missing: $SOURCE" >&2
  exit 1
fi

sync_destination() {
  destination=$1
  mkdir -p "$destination"

  # Remove generated files and empty directories without rsync. Destinations
  # are fixed below and never derived from user input.
  find "$destination" -type f -exec rm -f {} \;
  find "$destination" -type l -exec rm -f {} \;
  find "$destination" -depth -type d -empty ! -path "$destination" -exec rmdir {} \;

  cp -R "$SOURCE"/. "$destination"/
  echo "Synchronized ${destination#"$ROOT_DIR"/}"
}

sync_destination "$ROOT_DIR/npm/src/integrations/assets"
sync_destination "$ROOT_DIR/pypi/trackfw/integrations/assets"
