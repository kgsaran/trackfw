#!/usr/bin/env bash
# trackfw attention cleanup — compatibility wrapper; attention never blocks tools
command -v trackfw >/dev/null 2>&1 || exit 0
trackfw attention cleanup "$@" || true
exit 0
