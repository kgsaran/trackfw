#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to `trackfw guard credential`.
# Script is intentionally a no-op when executed outside any project (no trackfw.yaml in any ancestor)
set -euo pipefail
d=$PWD
while :; do
  if [ -f "${d%/}/trackfw.yaml" ]; then break; fi
  if [ "$d" = / ]; then exit 0; fi
  d=$(dirname "$d")
done
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential
