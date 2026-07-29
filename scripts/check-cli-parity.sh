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

# Floor: minimum set of cross-runtime commands that must always be present.
# Used as a vacuity guard — if parsing Go's "Available Commands:" block
# produces fewer commands than the floor (indicating a parser breakage), we
# exit 1 rather than silently running a vacuous check.
floor_commands=(
  init adr req roadmap validate status log plugins discover update metrics
  sync context baseline help configure serve version agents skills note ship
)

# Go-only commands: documented in docs/cli-parity.md as exceptions to the
# cross-runtime parity contract. These exist in the Go binary for historical
# compatibility and must NOT be required of the Node.js and Python CLIs.
#  · completion — cobra built-in shell-completion helper, not cross-runtime
go_only_commands=(completion)

# Derive the canonical command set from the Go CLI (the reference implementation).
# P1: never hardcode the command list; derive it so the gate stays accurate
# automatically when new commands are added to the Go CLI.
# Strip ANSI before parsing in case any colour slips through despite NO_COLOR.
_go_help=$("$GO_BIN" --help 2>&1 | sed 's/\x1b\[[0-9;]*m//g')

# All commands the Go CLI advertises (deduped; cobra may list "help" twice).
all_go_commands=()
while IFS= read -r _cmd; do
  [[ -n "$_cmd" ]] && all_go_commands+=("$_cmd")
done < <(
  awk '/^Available Commands:/{f=1;next}
       f && /^[[:space:]]{2,}[a-zA-Z]/{print $1}
       f && /^[[:space:]]*$/{exit}' <<< "$_go_help" \
  | awk '!seen[$0]++'
)

# Vacuity guard: a parse failure must be visible, not a silent vacuous pass.
if [[ ${#all_go_commands[@]} -lt ${#floor_commands[@]} ]]; then
  echo "check-cli-parity: Go help parsing yielded only ${#all_go_commands[@]} commands (floor=${#floor_commands[@]})" >&2
  echo "  Check that 'Available Commands:' block format has not changed." >&2
  exit 1
fi

# Cross-runtime commands: everything Go has, minus the documented Go-only set.
commands=()
for _cmd in "${all_go_commands[@]}"; do
  _is_go_only=0
  for _exc in "${go_only_commands[@]}"; do
    [[ "$_cmd" == "$_exc" ]] && _is_go_only=1 && break
  done
  [[ $_is_go_only -eq 0 ]] && commands+=("$_cmd")
done

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

# Node and Python must expose everything in the cross-runtime command set
# (all Go commands minus the documented Go-only exceptions).
check_help "node" "$(node "$ROOT_DIR/npm/bin/trackfw" --help)"
check_help "python" "$(PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw --help)"

check_roadmap_new_flags() {
  local runtime=$1
  local output
  output=$(printf '%s' "$2" | sed 's/\x1b\[[0-9;]*m//g')
  local flag
  for flag in "--title" "--req" "--from-req"; do
    if ! grep -qF -- "$flag" <<<"$output"; then
      echo "${runtime}: roadmap new help missing ${flag}" >&2
      return 1
    fi
  done
}

check_roadmap_new_flags "go" "$("$GO_BIN" roadmap new --help)"
check_roadmap_new_flags "node" "$(node "$ROOT_DIR/npm/bin/trackfw" roadmap new --help)"
check_roadmap_new_flags "python" "$(PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw roadmap new --help)"

"$GO_BIN" version | grep -Eq '^trackfw .+'
node "$ROOT_DIR/npm/bin/trackfw" version | grep -Eq '^trackfw .+'
PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw version | grep -Eq '^trackfw .+'

"$GO_BIN" --version | grep -Eq '^trackfw .+'
node "$ROOT_DIR/npm/bin/trackfw" --version | grep -Eq '^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$'
PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw --version | grep -Eq '^trackfw .+'

GO_BIN="$GO_BIN" bash "$ROOT_DIR/scripts/check-integration-cli-parity.sh"

echo "CLI parity smoke checks passed"
