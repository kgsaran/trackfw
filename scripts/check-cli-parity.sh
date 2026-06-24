#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}

mkdir -p "$(dirname "$GO_BIN")"
GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$GO_BIN" ./cmd/trackfw

commands=(
  init adr req roadmap validate status log plugins discover update metrics
  sync context baseline help configure serve version
)

check_help() {
  local runtime=$1
  local output=$2
  local command
  for command in "${commands[@]}"; do
    if ! grep -Eq "(^|[[:space:]])${command}([[:space:]]|$)" <<<"$output"; then
      echo "${runtime}: missing command '${command}'" >&2
      return 1
    fi
  done
}

check_help "go" "$("$GO_BIN" --help)"
check_help "node" "$(node "$ROOT_DIR/npm/bin/trackfw" --help)"
check_help "python" "$(PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw --help)"

"$GO_BIN" version | grep -Eq '^trackfw .+'
node "$ROOT_DIR/npm/bin/trackfw" version | grep -Eq '^trackfw .+'
PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw version | grep -Eq '^trackfw .+'

"$GO_BIN" --version | grep -Eq '^trackfw .+'
node "$ROOT_DIR/npm/bin/trackfw" --version | grep -Eq '^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$'
PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw --version | grep -Eq '^trackfw .+'

echo "CLI parity smoke checks passed"
