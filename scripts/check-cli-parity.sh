#!/usr/bin/env bash
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

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
  init adr req roadmap validate status log discover update metrics
  sync context baseline help configure serve version agents skills note ship push
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

# Version output — unified assertion across all three runtimes and both surfaces.
# Regex pinned in docs/cli-parity.md § "Gate assertion — pinned, and why the old one was vacuous".
# The previous gate used '^trackfw .+' (loose) for Go/Python and a Node.js-specific
# regex '^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$' that encoded format divergence as
# expected behaviour. Both are replaced here with the same strict assertion plus
# byte-by-byte comparison across runtimes and surfaces.
_VERSION_RE='^trackfw [0-9]+\.[0-9]+\.[0-9]+$'

_GO_VER=$("$GO_BIN" version) \
  || { echo "check-cli-parity: go version exited non-zero" >&2; exit 1; }
_NODE_VER=$(node "$ROOT_DIR/npm/bin/trackfw" version) \
  || { echo "check-cli-parity: node version exited non-zero" >&2; exit 1; }
_PY_VER=$(PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw version) \
  || { echo "check-cli-parity: python version exited non-zero" >&2; exit 1; }
_GO_FLAG=$("$GO_BIN" --version) \
  || { echo "check-cli-parity: go --version exited non-zero" >&2; exit 1; }
_NODE_FLAG=$(node "$ROOT_DIR/npm/bin/trackfw" --version) \
  || { echo "check-cli-parity: node --version exited non-zero" >&2; exit 1; }
_PY_FLAG=$(PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw --version) \
  || { echo "check-cli-parity: python --version exited non-zero" >&2; exit 1; }

# Vacuity guard: all six outputs must be non-empty before format and byte checks.
[[ -n "$_GO_VER" ]]    || { echo "check-cli-parity: go version output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_NODE_VER" ]]  || { echo "check-cli-parity: node version output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_PY_VER" ]]    || { echo "check-cli-parity: python version output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_GO_FLAG" ]]   || { echo "check-cli-parity: go --version output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_NODE_FLAG" ]] || { echo "check-cli-parity: node --version output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_PY_FLAG" ]]   || { echo "check-cli-parity: python --version output empty — vacuity guard failed" >&2; exit 1; }

# Single-line guard: contract requires exactly one line on stdout (no warning preamble).
# Command substitution strips the trailing newline, so a single-line output has
# `wc -l` == 0; two or more lines have `wc -l` >= 1.
[[ $(printf '%s' "$_GO_VER"    | wc -l) -eq 0 ]] || { echo "check-cli-parity: go version emitted more than one line (got: '$_GO_VER')" >&2; exit 1; }
[[ $(printf '%s' "$_NODE_VER"  | wc -l) -eq 0 ]] || { echo "check-cli-parity: node version emitted more than one line (got: '$_NODE_VER')" >&2; exit 1; }
[[ $(printf '%s' "$_PY_VER"    | wc -l) -eq 0 ]] || { echo "check-cli-parity: python version emitted more than one line (got: '$_PY_VER')" >&2; exit 1; }
[[ $(printf '%s' "$_GO_FLAG"   | wc -l) -eq 0 ]] || { echo "check-cli-parity: go --version emitted more than one line (got: '$_GO_FLAG')" >&2; exit 1; }
[[ $(printf '%s' "$_NODE_FLAG" | wc -l) -eq 0 ]] || { echo "check-cli-parity: node --version emitted more than one line (got: '$_NODE_FLAG')" >&2; exit 1; }
[[ $(printf '%s' "$_PY_FLAG"   | wc -l) -eq 0 ]] || { echo "check-cli-parity: python --version emitted more than one line (got: '$_PY_FLAG')" >&2; exit 1; }

# Format assertion — same strict regex for all six (three runtimes × two surfaces).
grep -Eq "$_VERSION_RE" <<<"$_GO_VER"    || { echo "check-cli-parity: go version format invalid (got: '$_GO_VER')" >&2; exit 1; }
grep -Eq "$_VERSION_RE" <<<"$_NODE_VER"  || { echo "check-cli-parity: node version format invalid (got: '$_NODE_VER')" >&2; exit 1; }
grep -Eq "$_VERSION_RE" <<<"$_PY_VER"    || { echo "check-cli-parity: python version format invalid (got: '$_PY_VER')" >&2; exit 1; }
grep -Eq "$_VERSION_RE" <<<"$_GO_FLAG"   || { echo "check-cli-parity: go --version format invalid (got: '$_GO_FLAG')" >&2; exit 1; }
grep -Eq "$_VERSION_RE" <<<"$_NODE_FLAG" || { echo "check-cli-parity: node --version format invalid (got: '$_NODE_FLAG')" >&2; exit 1; }
grep -Eq "$_VERSION_RE" <<<"$_PY_FLAG"   || { echo "check-cli-parity: python --version format invalid (got: '$_PY_FLAG')" >&2; exit 1; }

# Byte-comparison: all six outputs must be identical (version ≡ --version, within and across runtimes).
[[ "$_GO_VER" == "$_NODE_VER" ]]   || { echo "check-cli-parity: version byte mismatch — go vs node/version (got: '$_GO_VER' vs '$_NODE_VER')" >&2; exit 1; }
[[ "$_GO_VER" == "$_PY_VER" ]]     || { echo "check-cli-parity: version byte mismatch — go vs python/version (got: '$_GO_VER' vs '$_PY_VER')" >&2; exit 1; }
[[ "$_GO_VER" == "$_GO_FLAG" ]]    || { echo "check-cli-parity: version byte mismatch — go/version vs go/--version (got: '$_GO_VER' vs '$_GO_FLAG')" >&2; exit 1; }
[[ "$_GO_VER" == "$_NODE_FLAG" ]]  || { echo "check-cli-parity: version byte mismatch — go/version vs node/--version (got: '$_GO_VER' vs '$_NODE_FLAG')" >&2; exit 1; }
[[ "$_GO_VER" == "$_PY_FLAG" ]]    || { echo "check-cli-parity: version byte mismatch — go/version vs python/--version (got: '$_GO_VER' vs '$_PY_FLAG')" >&2; exit 1; }

# -v flag — reserved for verbose; must be rejected (non-zero exit + no version
# output) in all three runtimes. Two assertions per runtime:
#   1. Exit code is non-zero  — proves the flag is not silently accepted.
#   2. Output does not match _VERSION_RE — proves -v did not bind to --version.
# Together they characterize genuine rejection; each alone has a false-positive case:
#   exit-code alone does not distinguish "flag rejected" from "flag accepted but
#   something else went wrong"; format-check alone is vacuous when output is empty.
# Pinned in docs/cli-parity.md § "-v is reserved for verbose — never bound to --version".
#
# Note: exit codes for unknown flags are NOT unified across runtimes (cobra=1,
# commander=1, argparse=2 — framework divergence, deliberately preserved).
# The assertion uses -ne 0 rather than -eq N to remain runtime-agnostic.

set +e
_GO_V_OUT=$("$GO_BIN" -v 2>&1);                                       _GO_V_EXIT=$?
_NODE_V_OUT=$(node "$ROOT_DIR/npm/bin/trackfw" -v 2>&1);               _NODE_V_EXIT=$?
_PY_V_OUT=$(PYTHONPATH="$ROOT_DIR/pypi" python3 -m trackfw -v 2>&1);  _PY_V_EXIT=$?
set -e

# Vacuity guard: each runtime must produce non-empty output so the format
# assertion is not trivially satisfied by an empty string that never matched.
[[ -n "$_GO_V_OUT" ]]   || { echo "check-cli-parity: go -v output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_NODE_V_OUT" ]] || { echo "check-cli-parity: node -v output empty — vacuity guard failed" >&2; exit 1; }
[[ -n "$_PY_V_OUT" ]]   || { echo "check-cli-parity: python -v output empty — vacuity guard failed" >&2; exit 1; }

# Assertion 1: exit code must be non-zero (flag must be rejected).
[[ $_GO_V_EXIT   -ne 0 ]] || { echo "check-cli-parity: go -v exited 0 — -v must be rejected (non-zero exit)" >&2;     exit 1; }
[[ $_NODE_V_EXIT -ne 0 ]] || { echo "check-cli-parity: node -v exited 0 — -v must be rejected (non-zero exit)" >&2;   exit 1; }
[[ $_PY_V_EXIT   -ne 0 ]] || { echo "check-cli-parity: python -v exited 0 — -v must be rejected (non-zero exit)" >&2; exit 1; }

# Assertion 2: output must not match the version format.
# grep runs line-by-line, so a single matching line in a multi-line block fails.
if grep -Eq "$_VERSION_RE" <<<"$_GO_V_OUT"; then
  echo "check-cli-parity: go -v printed version string — -v must not bind to --version (got: '$_GO_V_OUT')" >&2; exit 1
fi
if grep -Eq "$_VERSION_RE" <<<"$_NODE_V_OUT"; then
  echo "check-cli-parity: node -v printed version string — -v must not bind to --version (got: '$_NODE_V_OUT')" >&2; exit 1
fi
if grep -Eq "$_VERSION_RE" <<<"$_PY_V_OUT"; then
  echo "check-cli-parity: python -v printed version string — -v must not bind to --version (got: '$_PY_V_OUT')" >&2; exit 1
fi

GO_BIN="$GO_BIN" bash "$ROOT_DIR/scripts/check-integration-cli-parity.sh"

echo "CLI parity smoke checks passed"
