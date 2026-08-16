#!/usr/bin/env bash
# check-serve-address-parity.sh — proves the listen address of `trackfw serve`
# is byte-for-byte/behaviorally identical across Go, Node.js, and Python, by
# actually starting each process and measuring where it listens (lsof), never
# by reading source. Covers REQ-2026-08-16-trackfw-serve-escuta-em-todas-as-
# interfaces-sem-autenticacao-expondo-a-cadeia-de-governanca-na-rede AC5:
#
#   1. Default (no --host): all 3 bind to 127.0.0.1, never to a wildcard
#      (*/0.0.0.0) — this is exactly the security regression this gate exists
#      to catch.
#   2. --host ::1: all 3 bind to [::1].
#   3. --host 0.0.0.0 exposure warning: byte-identical stderr across the 3,
#      normalizing only the port.
#   4. --host 0.0.0.0 printed URL: contains the bound host, never "localhost"
#      — reuses the same run as (3) instead of an unroutable TEST-NET-1
#      address (192.0.2.1), which does not bind on any of the 3 runtimes on a
#      typical machine (confirmed during ML-1B: "Go exit 1, zero occurrences
#      of 'listening'" — see docs/agents-working-context.md) and would make
#      this assertion vacuous by construction.
#
# Follows the conventions of scripts/check-branch-new-parity.sh: set -euo
# pipefail, NO_COLOR/TERM=dumb, mktemp -d work dir with a cleanup trap,
# BASH_SOURCE-relative ROOT_DIR, GO_BIN resolution (builds a throwaway binary
# if unset), ok()/fail() accumulating FAIL, and diff -u for byte-level
# comparisons.
set -euo pipefail

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-serve-address-parity.XXXXXX")

PIDS=()
cleanup() {
  local pid
  for pid in "${PIDS[@]:-}"; do
    [[ -n "${pid:-}" ]] && kill "$pid" >/dev/null 2>&1 || true
  done
  for pid in "${PIDS[@]:-}"; do
    [[ -n "${pid:-}" ]] && wait "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$WORK"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-branch-new-parity.sh:
#   GO_BIN unset → build a throwaway binary so the script also works standalone.
#   GO_BIN relative → prefix with ROOT_DIR.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-serve-address-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-serve-address-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

HAVE_LSOF=1
command -v lsof >/dev/null 2>&1 || HAVE_LSOF=0
if [[ "$HAVE_LSOF" -eq 0 ]]; then
  echo "WARN [serve-address-parity]: lsof not found — degrading to a plain TCP" \
       "connect check. The wildcard-bind assertion (proving the default does" \
       "NOT listen on */0.0.0.0) cannot be verified without lsof/ss/netstat" \
       "and is SKIPPED, not silently passed." >&2
fi

# next_port sets PORT to a fresh value — a plain assignment, not a
# command-substitution function, so the increment survives across calls
# (command substitution would run in a subshell and lose it).
PORT=46199
next_port() {
  PORT=$((PORT + 1))
}

# ---------------------------------------------------------------------------
# start_serve RUNTIME PORT [EXTRA_ARGS...] — starts `trackfw serve` in the
# background. Sets LAST_PID/LAST_OUT/LAST_ERR. Python is started via `exec`
# inside the subshell so LAST_PID is the interpreter's own pid (the one that
# actually holds the socket), not the subshell's.
# ---------------------------------------------------------------------------
start_serve() {
  local runtime=$1 port=$2
  shift 2
  local out="$WORK/$runtime.$port.out" err="$WORK/$runtime.$port.err"
  case "$runtime" in
    go)   "$GO_BIN" serve --port "$port" "$@"                                  >"$out" 2>"$err" & ;;
    node) node "$NODE_CLI" serve --port "$port" --no-open "$@"                 >"$out" 2>"$err" & ;;
    py)   (cd "$PY_ROOT" && exec python3 -u -m trackfw serve --port "$port" --no-open "$@") >"$out" 2>"$err" & ;;
    *)    echo "start_serve: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  LAST_PID=$!
  PIDS+=("$LAST_PID")
  LAST_OUT=$out
  LAST_ERR=$err
}

# wait_ready PID OUT_FILE TIMEOUT_SECONDS — polls until OUT_FILE carries a
# printed URL (proof the listener is actually up, per all 3 runtimes' "print
# only after bind succeeds" contract) or the process exits (bind failed).
# Returns 1 on timeout or early exit.
wait_ready() {
  local pid=$1 out=$2 timeout=$3
  local i=0
  while (( i < timeout * 10 )); do
    if grep -qE 'http://' "$out" 2>/dev/null; then
      return 0
    fi
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      return 1
    fi
    sleep 0.1
    i=$((i + 1))
  done
  return 1
}

# listen_addr_for_pid PID — the bound address as lsof reports it
# (e.g. "127.0.0.1:46200", "[::1]:46201", "*:46202"). Empty if not found.
listen_addr_for_pid() {
  local pid=$1
  lsof -a -p "$pid" -iTCP -sTCP:LISTEN -nP 2>/dev/null | awk 'NR>1 {print $(NF-1)}' | head -1
}

stop_pid() {
  local pid=$1
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
}

# ---------------------------------------------------------------------------
# 1 — Default bind (no --host): all 3 listen on loopback, never on a wildcard.
# ---------------------------------------------------------------------------
for runtime in go node py; do
  next_port; port=$PORT
  start_serve "$runtime" "$port"
  if ! wait_ready "$LAST_PID" "$LAST_OUT" 8; then
    fail "default-bind/$runtime" "process did not become ready in time; stderr: $(cat "$LAST_ERR" 2>/dev/null)"
    stop_pid "$LAST_PID"
    continue
  fi
  if [[ "$HAVE_LSOF" -eq 1 ]]; then
    addr=$(listen_addr_for_pid "$LAST_PID")
    stop_pid "$LAST_PID"
    if [[ "$addr" == "127.0.0.1:$port" ]]; then
      ok "default-bind/$runtime (127.0.0.1:$port)"
    else
      fail "default-bind/$runtime" "expected lsof to show 127.0.0.1:$port, got '$addr' — a wildcard/non-loopback default is the exact security regression this gate exists to catch"
    fi
  else
    if (echo >"/dev/tcp/127.0.0.1/$port") >/dev/null 2>&1; then
      ok "default-bind/$runtime (127.0.0.1:$port reachable, wildcard exclusion unverified without lsof)"
    else
      fail "default-bind/$runtime" "127.0.0.1:$port did not accept a connection"
    fi
    stop_pid "$LAST_PID"
  fi
done

# ---------------------------------------------------------------------------
# 2 — --host ::1: all 3 listen on [::1].
# ---------------------------------------------------------------------------
for runtime in go node py; do
  next_port; port=$PORT
  start_serve "$runtime" "$port" --host ::1
  if ! wait_ready "$LAST_PID" "$LAST_OUT" 8; then
    fail "host-ipv6-loopback/$runtime" "process did not become ready in time; stderr: $(cat "$LAST_ERR" 2>/dev/null)"
    stop_pid "$LAST_PID"
    continue
  fi
  if [[ "$HAVE_LSOF" -eq 1 ]]; then
    addr=$(listen_addr_for_pid "$LAST_PID")
    stop_pid "$LAST_PID"
    if [[ "$addr" == "[::1]:$port" ]]; then
      ok "host-ipv6-loopback/$runtime ([::1]:$port)"
    else
      fail "host-ipv6-loopback/$runtime" "expected lsof to show [::1]:$port, got '$addr'"
    fi
  else
    if (echo >"/dev/tcp/::1/$port") >/dev/null 2>&1; then
      ok "host-ipv6-loopback/$runtime ([::1]:$port reachable)"
    else
      fail "host-ipv6-loopback/$runtime" "[::1]:$port did not accept a connection"
    fi
    stop_pid "$LAST_PID"
  fi
done

# ---------------------------------------------------------------------------
# 3/4 — --host 0.0.0.0: exposure warning byte-identical (port normalized) +
# printed URL carries the bound host, never "localhost". Reuses the same run
# for both assertions (the 4th action of ML-1C, done here with 0.0.0.0
# instead of the unroutable 192.0.2.1 — see file header).
# ---------------------------------------------------------------------------
declare -A EXPOSURE_ERR
declare -A EXPOSURE_URL
for runtime in go node py; do
  next_port; port=$PORT
  start_serve "$runtime" "$port" --host 0.0.0.0
  if ! wait_ready "$LAST_PID" "$LAST_OUT" 8; then
    fail "host-wildcard-exposure/$runtime" "process did not become ready in time; stderr: $(cat "$LAST_ERR" 2>/dev/null)"
    stop_pid "$LAST_PID"
    continue
  fi
  stop_pid "$LAST_PID"

  url=$(grep -oE 'http://\S+' "$LAST_OUT" | head -1)
  if [[ -z "$url" ]]; then
    fail "host-wildcard-exposure/$runtime/url" "no URL found in stdout: $(cat "$LAST_OUT")"
  elif [[ "$url" != "http://0.0.0.0:$port" ]] || [[ "$url" == *localhost* ]]; then
    fail "host-wildcard-exposure/$runtime/url" "expected printed URL 'http://0.0.0.0:$port' (not localhost), got '$url'"
  else
    ok "host-wildcard-exposure/$runtime/url ($url)"
  fi
  EXPOSURE_URL[$runtime]=$url

  # Normalize the port so the byte-diff below compares only the fixed prose,
  # not the (deliberately distinct, to avoid port collisions) per-runtime port.
  sed "s/:$port /:PORT /g; s/:$port\$/:PORT/g" "$LAST_ERR" >"$WORK/$runtime.exposure.err.norm"
  EXPOSURE_ERR[$runtime]="$WORK/$runtime.exposure.err.norm"
done

if [[ -n "${EXPOSURE_ERR[go]:-}" && -n "${EXPOSURE_ERR[node]:-}" && -n "${EXPOSURE_ERR[py]:-}" ]]; then
  diverged=0
  if ! diff -u "${EXPOSURE_ERR[go]}" "${EXPOSURE_ERR[node]}" >"$WORK/exposure.diff.go-node" 2>&1; then
    fail "host-wildcard-exposure/warning/go-vs-node" "stderr diverges:
$(cat "$WORK/exposure.diff.go-node")"
    diverged=1
  fi
  if ! diff -u "${EXPOSURE_ERR[go]}" "${EXPOSURE_ERR[py]}" >"$WORK/exposure.diff.go-py" 2>&1; then
    fail "host-wildcard-exposure/warning/go-vs-py" "stderr diverges:
$(cat "$WORK/exposure.diff.go-py")"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "host-wildcard-exposure/warning (byte-identical across go/node/py)"
  fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-serve-address-parity.sh scenarios passed."
else
  echo "check-serve-address-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
