#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CANONICAL="$ROOT_DIR/internal/serve/static"

for target in "$ROOT_DIR/npm/src/serve/static" "$ROOT_DIR/pypi/trackfw/serve/static"; do
  for asset in index.html app.js style.css; do
    if ! cmp -s "$CANONICAL/$asset" "$target/$asset"; then
      echo "Static asset drift: ${target#$ROOT_DIR/}/$asset" >&2
      echo "Canonical source: internal/serve/static/$asset" >&2
      exit 1
    fi
  done
done

echo "Static assets are synchronized"
