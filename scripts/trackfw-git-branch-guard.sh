#!/usr/bin/env bash
# trackfw-git-branch-guard.sh — thin wrapper; delegates to `trackfw guard git-branch`.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
if [ $# -gt 0 ]; then exec trackfw guard git-branch --command "$*"; fi
exec trackfw guard git-branch
