#!/usr/bin/env bash
# ML-1B Mac — 3 braços (3/3 chamadas). Escreve ~/.gemini/config/hooks.json e apaga no fim.
set -u
P="$HOME/agyprobe"; L="$P/logs"; M="$P/markers"; G="$HOME/.gemini/config/hooks.json"
[ -e "$G" ] && { echo "ABORT: $G já existe"; exit 3; }
trap 'rm -f "$G"; echo "cleanup: $G removido"' EXIT
cd "$P" || exit 1
hooks() { printf '{"%s":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"/bin/sh %s/scripts/probe.sh %s","timeout":10}]}]}}' "$1" "$P" "$2" > "$3"; }
run() { a=$1; rm -f "$M/marker_$a.txt"
  agy -p "Run exactly this shell command and nothing else, then stop: echo MARKER_$a > $M/marker_$a.txt" --dangerously-skip-permissions --output-format stream-json --print-timeout 180s > "$L/agy_$a.ndjson" 2> "$L/agy_$a.err"
  echo "agy_rc=$? marker=$([ -f "$M/marker_$a.txt" ] && echo yes || echo no)"; }
rm -f "$L"/probe_*global*.log "$L"/probe_*ws*.log
echo "[1/3] A1 só global (allow-global), sem hook de projeto"
rm -f "$P/.agents/hooks.json"; hooks g allow-global "$G"; echo "A1 $(run a1) global_fired=$(ls "$L"/probe_allow-global_*.log 2>/dev/null | wc -l | tr -d ' ')"
rm -f "$L"/probe_*global*.log
echo "[2/3] A2 global allow-global + projeto allow-ws"
hooks g allow-global "$G"; hooks w allow-ws "$P/.agents/hooks.json"; echo "A2 $(run a2) global_fired=$(ls "$L"/probe_allow-global_*.log 2>/dev/null | wc -l | tr -d ' ') ws_fired=$(ls "$L"/probe_allow-ws_*.log 2>/dev/null | wc -l | tr -d ' ')"
rm -f "$L"/probe_*global*.log "$L"/probe_*ws*.log
echo "[3/3] A3 global deny-global + projeto allow-ws"
hooks g deny-global "$G"; hooks w allow-ws "$P/.agents/hooks.json"; echo "A3 $(run a3) global_fired=$(ls "$L"/probe_deny-global_*.log 2>/dev/null | wc -l | tr -d ' ') ws_fired=$(ls "$L"/probe_allow-ws_*.log 2>/dev/null | wc -l | tr -d ' ')"
rm -f "$P/.agents/hooks.json"
