#!/usr/bin/env bash
set -euo pipefail

# Disable ANSI colour output across all runtimes invoked in this process tree.
# Python 3.13+ colorises argparse help by default; without NO_COLOR the grep
# patterns in check_help fail because ANSI escapes wrap the matched word
# (e.g. ESC[1;32minit ESC[0m — the character before "init" is "m", not a space).
export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}

mkdir -p "$(dirname "$GO_BIN")"
GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$GO_BIN" ./cmd/trackfw

commands=(
  init adr req roadmap validate status log plugins discover update metrics
  sync context baseline help configure serve version agents skills note ship
)

check_help() {
  local runtime=$1
  # Strip any remaining ANSI escape sequences before grep so the check is
  # immune to runtimes that honour NO_COLOR inconsistently or not at all.
  local output
  output=$(printf '%s' "$2" | sed 's/\x1b\[[0-9;]*m//g')
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

GO_BIN="$GO_BIN" bash "$ROOT_DIR/scripts/check-integration-cli-parity.sh"

echo "CLI parity smoke checks passed"
