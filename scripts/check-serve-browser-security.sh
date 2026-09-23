#!/usr/bin/env bash
# check-serve-browser-security.sh — proves `trackfw serve` browser-opening
# code does not have shell injection vulnerabilities, and that host validation
# rejects injection payloads.
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. This gate now asserts the Go binary's behavior alone:
#   - AC4 CLI wiring: invalid --host exits non-zero, stderr names bad host
#   - Zone-ID rejection: scoped host (fe80::1%eth0&...) exits non-zero
#
# Cross-runtime scenarios (injection of openBrowser() in Node.js/Python,
# AST static checks on serve.js/serve.py, _is_valid_host/isValidHost function
# tests) removed by ML-3A (v8) because npm/src/ and pypi/trackfw/ were deleted.
#
# Original gate: scripts/check-serve-browser-security.sh (ROADMAP-2026-...).
set -euo pipefail

export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$ROOT_DIR/scripts/lib-crlf-normalize.sh"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-serve-browser-security.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve Go binary
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

FAIL=0
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }
ok()   { echo "OK   [$1]"; }

BAD_HOST='x"; touch /tmp/INJETADO ; echo "'
ZONE_HOST='fe80::1%eth0&calc.exe&echo'

# ---------------------------------------------------------------------------
# AC4 CLI wiring (Go): `trackfw serve --host '<bad>'`
# exits non-zero, stderr names the bad host.
# ---------------------------------------------------------------------------
GO_STDERR="$WORK/go-serve-stderr.txt"
if "$GO_BIN" serve --host "$BAD_HOST" \
     >"$WORK/go-serve-stdout.txt" 2>"$GO_STDERR"; then
  fail "ac4-wiring/go" "CLI exited 0 for invalid --host — validation is not wired before bind"
else
  if grep -q "invalid --host" "$GO_STDERR" 2>/dev/null; then
    ok "ac4-wiring/go (CLI exits non-zero + stderr names bad host — AC4 is wired)"
  else
    fail "ac4-wiring/go" "CLI exited non-zero but stderr does not contain 'invalid --host' (got: $(head -2 "$GO_STDERR" 2>/dev/null))"
  fi
fi

# ---------------------------------------------------------------------------
# Zone ID rejection (Go CLI wiring): CLI exits non-zero for scoped host.
# ---------------------------------------------------------------------------
GO_ZONE_STDERR="$WORK/go-zone-stderr.txt"
if "$GO_BIN" serve --host "$ZONE_HOST" \
     >"$WORK/go-zone-stdout.txt" 2>"$GO_ZONE_STDERR"; then
  fail "zone-id-rejection/go-wiring" "CLI exited 0 for zone ID host '$ZONE_HOST' — validation is not wired"
else
  ok "zone-id-rejection/go-wiring (CLI exits non-zero for '$ZONE_HOST')"
fi

# ---------------------------------------------------------------------------
# Zone ID parity — Go rejects clean zone ID 'fe80::1%eth0'
# (net.ParseIP rejects zone IDs; confirmed Go-only since v8)
# ---------------------------------------------------------------------------
CLEAN_ZONE='fe80::1%eth0'
GO_CLEAN_ZONE_STDERR="$WORK/go-clean-zone-stderr.txt"
if "$GO_BIN" serve --host "$CLEAN_ZONE" \
     >"$WORK/go-clean-zone-stdout.txt" 2>"$GO_CLEAN_ZONE_STDERR"; then
  fail "zone-id-clean/go" "CLI exited 0 for '$CLEAN_ZONE' — expected rejection (net.ParseIP rejects zone IDs)"
else
  ok "zone-id-clean/go (CLI exits non-zero for '$CLEAN_ZONE' — Go behavioral pin)"
fi

# ---------------------------------------------------------------------------
# list2cmdline vulnerable-arm: prove the attack vector is real
# (zone ID + & as command separator). Pure string operation — no Windows needed.
# ---------------------------------------------------------------------------
ZONE_VECTOR_URL='http://[fe80::1%eth0&calc.exe&echo]:4080'
ZONE_CMDLINE=$(python3 -c "
import subprocess
url = '$ZONE_VECTOR_URL'
argv = ['cmd', '/c', 'start', '', url]
print(subprocess.list2cmdline(argv))
" 2>/dev/null | strip_cr)

if [[ -z "$ZONE_CMDLINE" ]]; then
  fail "zone-id-injection/vulnerable-arm" "list2cmdline check failed — cannot measure the attack vector (fail-closed)"
elif echo "$ZONE_CMDLINE" | grep -q '&' && ! echo "$ZONE_CMDLINE" | grep -q '"&"'; then
  ok "zone-id-injection/vulnerable-arm (list2cmdline leaves & unquoted — attack vector confirmed real, gate discriminates)"
else
  fail "zone-id-injection/vulnerable-arm" "expected unquoted & in list2cmdline output but got: $ZONE_CMDLINE"
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
