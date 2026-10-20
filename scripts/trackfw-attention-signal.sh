#!/usr/bin/env bash
# trackfw attention signal — compatibility wrapper; attention never blocks tools
command -v trackfw >/dev/null 2>&1 || exit 0
trackfw attention signal "$@" || true
exit 0
