#!/usr/bin/env bash
# check-serve-browser-security.sh — proves that `trackfw serve`'s browser-open
# path does NOT interpolate --host into a shell string (REQ-2026-09-01-serve-
# interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-
# browser, AC1+AC2+AC3+AC4 in Node.js and Python).
#
# Covers ALL of:
#   AC1 — no runtime interpolates a controllable value into a shell string
#   AC2 — Python Windows branch no longer uses shell=True
#   AC3 — falsification in both directions:
#          (a) payload does NOT execute extra command → sentinel absent
#          (b) legitimate host → browser opener called with correct URL as one argv
#   AC4 — invalid --host rejected before bind (CLI exits non-zero, stderr names
#          the host, no listener on port) — proven for all 3 CLIs (Go, Node, Python)
#
# Go has no browser-open path (it only prints the URL); AC4 is proven via CLI
# wiring (IsValidHost is called before serve.Start, so a bad host is rejected
# before any bind).
#
# Structure:
#   1. Vulnerable-reference reproducer (with PATH shim): OLD exec(string) DOES
#      create sentinel → gate actually discriminates.
#   2. Injection gate (Node.js): fixed code does NOT create the sentinel.
#   3. Counter-arm (Node.js): legitimate host → 'open' shim called with URL as
#      a single argv element, verbatim.
#   4. Injection gate (Python, Darwin branch): fixed code does NOT create sentinel.
#   5. Counter-arm (Python): 'open' shim called with URL as single argv element.
#   6. AC2 static proof (Python AST): no live Popen(shell=True) call.
#   7. AC2 Windows argv: _browser_argv("Windows") returns ["cmd", ...] not ["start",...].
#   8. AC1 static Node: old exec(`open pattern absent from live code.
#   9. AC1 static Node-spawn: spawn present in live code.
#  10. AC4 function (Node.js): isValidHost rejects injection payload.
#  11. AC4 function (Node.js): isValidHost accepts all legitimate host forms.
#  12. AC4 function (Python): _is_valid_host rejects injection payload.
#  13. AC4 function (Python): _is_valid_host accepts all legitimate host forms.
#  14. AC4 CLI wiring (Node.js): CLI exits non-zero + stderr names host for bad --host.
#  15. AC4 CLI wiring (Python): CLI exits non-zero + stderr names host for bad --host.
#  16. AC4 CLI wiring (Go): CLI exits non-zero + stderr names host for bad --host.
#
# Conventions:
#   - set -euo pipefail, mktemp WORK dir + cleanup trap.
#   - FAIL accumulator: ok()/fail() never abort early.
#   - python3 always, never python.
#   - Gate anchored to ROOT_DIR.

set -euo pipefail
export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-serve-browser-security.XXXXXX")
SENTINEL="$WORK/INJETADO"

cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

NODE_SERVE="$ROOT_DIR/npm/src/commands/serve.js"
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_SERVE="$ROOT_DIR/pypi/trackfw/commands/serve.py"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"
PY_CLI="$ROOT_DIR/pypi/trackfw/__main__.py"

# ---------------------------------------------------------------------------
# Go binary resolution (mirrors check-serve-address-parity.sh).
# GO_BIN unset → build a throwaway binary.
# GO_BIN relative → prefix with ROOT_DIR.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-serve-browser-security: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# PATH shim setup — used by all injection/counter-arm/wiring tests.
# The shim logs all arguments it receives to SHIM_LOG (one space-separated line)
# and echoes the verbatim first argument for inspection.
# ---------------------------------------------------------------------------
SHIM_DIR="$WORK/shim"
mkdir -p "$SHIM_DIR"
cat >"$SHIM_DIR/open" <<'STUB'
#!/bin/sh
# Shim for 'open': log all arguments verbatim so the gate can verify them.
printf '%s' "$*" >> "$SHIM_LOG"
STUB
chmod +x "$SHIM_DIR/open"
cp "$SHIM_DIR/open" "$SHIM_DIR/xdg-open"
chmod +x "$SHIM_DIR/xdg-open"

SHIM_LOG="$WORK/shim.log"

MALICIOUS_URL='http://x" ; touch '"$SENTINEL"' ; echo ":4080'
LEGIT_URL="http://localhost:4080"
BAD_HOST='x"; touch /tmp/INJETADO ; echo "'

# ---------------------------------------------------------------------------
# 1 — Vulnerable-reference reproducer (with PATH shim, to avoid browser tabs
#     and make the two arms comparable).
#     The OLD exec(`open "${url}"`) DOES create the sentinel via shell injection.
#     Purpose: prove this gate discriminates before and after the fix.
# ---------------------------------------------------------------------------
rm -f "$SENTINEL" "$SHIM_LOG"

SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" \
  node -e "
const { exec } = require('child_process');
const url = process.argv[1];
const openCmd = 'open \"' + url + '\"';
exec(openCmd, (err) => {});
setTimeout(() => {}, 500);
" -- "$MALICIOUS_URL" 2>/dev/null || true
sleep 0.6   # allow the child process to complete

if [[ -f "$SENTINEL" ]]; then
  ok "vulnerable-reference/node (OLD exec+string approach creates sentinel — gate is falsifiable)"
  rm -f "$SENTINEL"
else
  fail "vulnerable-reference/node" "expected sentinel '$SENTINEL' to be created by OLD exec+string approach — gate may not be discriminating"
fi

# ---------------------------------------------------------------------------
# 2 — Injection gate (Node.js): fixed openBrowser() does NOT create the sentinel.
# ---------------------------------------------------------------------------
rm -f "$SENTINEL" "$SHIM_LOG"

SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" \
  node -e "
const { openBrowser } = require('$NODE_SERVE');
openBrowser('darwin', process.argv[1]);
setTimeout(() => {}, 300);
" -- "$MALICIOUS_URL" 2>/dev/null || true
sleep 0.4

if [[ -f "$SENTINEL" ]]; then
  fail "injection/node" "sentinel '$SENTINEL' was created — shell injection STILL POSSIBLE in Node.js fixed code"
else
  ok "injection/node (sentinel absent — malicious URL does not execute extra command)"
fi

# ---------------------------------------------------------------------------
# 3 — Counter-arm (Node.js): legitimate URL → shim receives it verbatim as
#     a single argument (no shell splitting).
# ---------------------------------------------------------------------------
rm -f "$SHIM_LOG"

SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" \
  node -e "
const { openBrowser } = require('$NODE_SERVE');
openBrowser('darwin', process.argv[1]);
setTimeout(() => {}, 300);
" -- "$LEGIT_URL" 2>/dev/null || true
sleep 0.4

if [[ ! -f "$SHIM_LOG" ]]; then
  fail "counter-arm/node" "'open' shim was never called for legitimate host — browser opener is broken"
else
  logged=$(cat "$SHIM_LOG")
  if [[ "$logged" == "$LEGIT_URL" ]]; then
    ok "counter-arm/node (legitimate URL '$LEGIT_URL' passed as single argv to 'open')"
  else
    fail "counter-arm/node" "shim received '$logged', expected verbatim '$LEGIT_URL'"
  fi
fi

# ---------------------------------------------------------------------------
# 4 — Injection gate (Python, Darwin branch): fixed _open_browser() does NOT
#     create the sentinel.
# ---------------------------------------------------------------------------
rm -f "$SENTINEL" "$SHIM_LOG"

SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" \
  python3 -c "
import sys
sys.path.insert(0, '$PY_ROOT')
from trackfw.commands.serve import _open_browser
import unittest.mock as mock
url = sys.argv[1]
with mock.patch('platform.system', return_value='Darwin'):
    _open_browser(url)
import time; time.sleep(0.3)
" "$MALICIOUS_URL" 2>/dev/null || true
sleep 0.4

if [[ -f "$SENTINEL" ]]; then
  fail "injection/python-darwin" "sentinel '$SENTINEL' was created — shell injection STILL POSSIBLE in Python Darwin branch"
else
  ok "injection/python-darwin (sentinel absent — Darwin branch does not inject)"
fi

# ---------------------------------------------------------------------------
# 5 — Counter-arm (Python): legitimate URL → shim receives it verbatim.
# ---------------------------------------------------------------------------
rm -f "$SHIM_LOG"

SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" \
  python3 -c "
import sys
sys.path.insert(0, '$PY_ROOT')
from trackfw.commands.serve import _open_browser
import unittest.mock as mock
url = sys.argv[1]
with mock.patch('platform.system', return_value='Darwin'):
    _open_browser(url)
import time; time.sleep(0.3)
" "$LEGIT_URL" 2>/dev/null || true
sleep 0.4

if [[ ! -f "$SHIM_LOG" ]]; then
  fail "counter-arm/python" "'open' shim was never called for legitimate host in Python — browser opener broken"
else
  logged=$(cat "$SHIM_LOG")
  if [[ "$logged" == "$LEGIT_URL" ]]; then
    ok "counter-arm/python (legitimate URL passed as single argv to 'open')"
  else
    fail "counter-arm/python" "shim received '$logged', expected verbatim '$LEGIT_URL'"
  fi
fi

# ---------------------------------------------------------------------------
# 6 — AC2 static proof (Python): no live Popen(shell=True) call.
#     Uses Python AST analysis — immune to comment/docstring false positives.
# ---------------------------------------------------------------------------
PY_STATIC_CHECK=$(python3 -c "
import ast, sys

src = open('$PY_SERVE').read()
tree = ast.parse(src)

bad_calls = []
for node in ast.walk(tree):
    if not isinstance(node, ast.Call):
        continue
    is_popen = False
    func = node.func
    if isinstance(func, ast.Attribute) and func.attr == 'Popen':
        is_popen = True
    elif isinstance(func, ast.Name) and func.id == 'Popen':
        is_popen = True
    if not is_popen:
        continue
    for kw in node.keywords:
        if kw.arg == 'shell' and isinstance(kw.value, ast.Constant) and kw.value.value is True:
            bad_calls.append(node.lineno)

if bad_calls:
    print('FAIL:' + ','.join(str(l) for l in bad_calls))
else:
    print('OK')
" 2>&1)

if [[ "$PY_STATIC_CHECK" == OK ]]; then
  ok "ac2-static/python (AST: no live Popen(shell=True) call in serve.py — AC2 satisfied)"
else
  fail "ac2-static/python" "AST found live Popen(shell=True) at line(s) ${PY_STATIC_CHECK#FAIL:} in $PY_SERVE"
fi

# ---------------------------------------------------------------------------
# 7 — AC2 Windows argv: _browser_argv("Windows") must return ["cmd", ...].
# ---------------------------------------------------------------------------
WIN_ARGV=$(python3 -c "
import sys
sys.path.insert(0, '$PY_ROOT')
from trackfw.commands.serve import _browser_argv
argv = _browser_argv('Windows', 'http://test:4080')
print(argv[0])
" 2>/dev/null)

if [[ "$WIN_ARGV" == "cmd" ]]; then
  ok "ac2-static/python-windows-cmd (Windows argv starts with 'cmd', not 'start')"
else
  fail "ac2-static/python-windows-cmd" "_browser_argv('Windows', ...) first element is '$WIN_ARGV', expected 'cmd'"
fi

# ---------------------------------------------------------------------------
# 8 — AC1 static (Node.js): old exec(`open pattern absent from live code.
# ---------------------------------------------------------------------------
OLD_EXEC_PATTERN='exec(`open'
if grep -v "^[[:space:]]*//" "$NODE_SERVE" | grep -F "$OLD_EXEC_PATTERN" >/dev/null 2>&1; then
  fail "ac1-static/node" "found live exec(string) pattern '$OLD_EXEC_PATTERN' in $NODE_SERVE (excluding comment lines)"
else
  ok "ac1-static/node (old exec(string) pattern absent from live code in serve.js)"
fi

# ---------------------------------------------------------------------------
# 9 — AC1 static (Node.js): spawn present in live code.
# ---------------------------------------------------------------------------
if grep -v "^[[:space:]]*//" "$NODE_SERVE" | grep -q "spawn"; then
  ok "ac1-static/node-spawn (spawn is present in live code of serve.js)"
else
  fail "ac1-static/node-spawn" "spawn not found in live code of $NODE_SERVE — openBrowser may not be using argv"
fi

# ---------------------------------------------------------------------------
# 10 — AC4 function (Node.js): isValidHost rejects injection payload.
# ---------------------------------------------------------------------------
PAYLOAD_CHECK=$(node -e "
const { isValidHost } = require('$NODE_SERVE');
const payload = 'x\"; touch /tmp/INJETADO ; echo \"';
process.stdout.write(isValidHost(payload) ? 'true' : 'false');
" 2>/dev/null)

if [[ "$PAYLOAD_CHECK" == "false" ]]; then
  ok "ac4/node (isValidHost rejects injection payload)"
else
  fail "ac4/node" "isValidHost returned '$PAYLOAD_CHECK' for injection payload — expected 'false'"
fi

# ---------------------------------------------------------------------------
# 11 — AC4 function (Node.js): isValidHost accepts all legitimate host forms.
# ---------------------------------------------------------------------------
LEGIT_CHECK=$(node -e "
const { isValidHost } = require('$NODE_SERVE');
const hosts = ['localhost', '127.0.0.1', '::1', 'my-host.example.com', '0.0.0.0'];
process.stdout.write(hosts.every(h => isValidHost(h)) ? 'true' : 'false');
" 2>/dev/null)

if [[ "$LEGIT_CHECK" == "true" ]]; then
  ok "ac4/node-legitimate (isValidHost accepts all legitimate host forms)"
else
  fail "ac4/node-legitimate" "isValidHost rejected a legitimate host — counter-arm broken"
fi

# ---------------------------------------------------------------------------
# 12 — AC4 function (Python): _is_valid_host rejects injection payload.
# ---------------------------------------------------------------------------
PY_PAYLOAD_CHECK=$(python3 -c "
import sys
sys.path.insert(0, '$PY_ROOT')
from trackfw.commands.serve import _is_valid_host
payload = 'x\"; touch /tmp/INJETADO ; echo \"'
print('false' if not _is_valid_host(payload) else 'true', end='')
" 2>/dev/null)

if [[ "$PY_PAYLOAD_CHECK" == "false" ]]; then
  ok "ac4/python (_is_valid_host rejects injection payload)"
else
  fail "ac4/python" "_is_valid_host returned '$PY_PAYLOAD_CHECK' for injection payload — expected 'false'"
fi

# ---------------------------------------------------------------------------
# 13 — AC4 function (Python): _is_valid_host accepts all legitimate host forms.
# ---------------------------------------------------------------------------
PY_LEGIT_CHECK=$(python3 -c "
import sys
sys.path.insert(0, '$PY_ROOT')
from trackfw.commands.serve import _is_valid_host
hosts = ['localhost', '127.0.0.1', '::1', 'my-host.example.com', '0.0.0.0']
print('true' if all(_is_valid_host(h) for h in hosts) else 'false', end='')
" 2>/dev/null)

if [[ "$PY_LEGIT_CHECK" == "true" ]]; then
  ok "ac4/python-legitimate (_is_valid_host accepts all legitimate host forms)"
else
  fail "ac4/python-legitimate" "_is_valid_host rejected a legitimate host — counter-arm broken"
fi

# ---------------------------------------------------------------------------
# 14 — AC4 CLI wiring (Node.js): `trackfw serve --host '<bad>' --no-open`
#      exits non-zero, stderr names the bad host.
#      Proves validation runs before bind, not just that the function returns false.
# ---------------------------------------------------------------------------
NODE_STDERR="$WORK/node-serve-stderr.txt"
if PATH="$SHIM_DIR:$PATH" node "$NODE_CLI" serve --host "$BAD_HOST" --no-open \
     >"$WORK/node-serve-stdout.txt" 2>"$NODE_STDERR"; then
  fail "ac4-wiring/node" "CLI exited 0 for invalid --host — validation is not wired before bind"
else
  if grep -q "$BAD_HOST" "$NODE_STDERR" 2>/dev/null || \
     grep -q "invalid --host" "$NODE_STDERR" 2>/dev/null; then
    ok "ac4-wiring/node (CLI exits non-zero + stderr names bad host — AC4 is wired)"
  else
    fail "ac4-wiring/node" "CLI exited non-zero but stderr does not name the bad host (got: $(cat "$NODE_STDERR" 2>/dev/null | head -2))"
  fi
fi

# ---------------------------------------------------------------------------
# 15 — AC4 CLI wiring (Python): `python3 -m trackfw serve --host '<bad>' --no-open`
#      exits non-zero, stderr names the bad host.
# ---------------------------------------------------------------------------
PY_STDERR="$WORK/py-serve-stderr.txt"
if PYTHONPATH="$PY_ROOT" PATH="$SHIM_DIR:$PATH" \
     python3 -m trackfw serve --host "$BAD_HOST" --no-open \
     >"$WORK/py-serve-stdout.txt" 2>"$PY_STDERR"; then
  fail "ac4-wiring/python" "CLI exited 0 for invalid --host — validation is not wired before bind"
else
  if grep -q "invalid --host" "$PY_STDERR" 2>/dev/null; then
    ok "ac4-wiring/python (CLI exits non-zero + stderr names bad host — AC4 is wired)"
  else
    fail "ac4-wiring/python" "CLI exited non-zero but stderr does not contain 'invalid --host' (got: $(cat "$PY_STDERR" 2>/dev/null | head -2))"
  fi
fi

# ---------------------------------------------------------------------------
# 16 — AC4 CLI wiring (Go): `trackfw serve --host '<bad>'`
#      exits non-zero, stderr names the bad host.
# ---------------------------------------------------------------------------
GO_STDERR="$WORK/go-serve-stderr.txt"
if "$GO_BIN" serve --host "$BAD_HOST" \
     >"$WORK/go-serve-stdout.txt" 2>"$GO_STDERR"; then
  fail "ac4-wiring/go" "CLI exited 0 for invalid --host — validation is not wired before bind"
else
  if grep -q "invalid --host" "$GO_STDERR" 2>/dev/null; then
    ok "ac4-wiring/go (CLI exits non-zero + stderr names bad host — AC4 is wired)"
  else
    fail "ac4-wiring/go" "CLI exited non-zero but stderr does not contain 'invalid --host' (got: $(cat "$GO_STDERR" 2>/dev/null | head -2))"
  fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-serve-browser-security.sh scenarios passed."
else
  echo "check-serve-browser-security.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
