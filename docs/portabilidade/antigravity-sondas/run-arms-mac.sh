#!/usr/bin/env bash
# ML-1A (REQ-2026-11-17) — sonda do contrato de hook do Antigravity no macOS.
# Uso: bash run-arms-mac.sh            (7 chamadas de modelo: 7/7)
# Só escreve em ~/agyprobe. Não toca em ~/.gemini.
set -u
P="$HOME/agyprobe"; L="$P/logs"; M="$P/markers"
mkdir -p "$P/.agents" "$P/scripts" "$L" "$M"
cd "$P" || exit 1
[ -d .git ] || git init -q

cat > "$P/scripts/probe.sh" <<'PROBE'
#!/bin/sh
ARM="${1:-unknown}"
LOG="$HOME/agyprobe/logs/probe_${ARM}_$$.log"
STDIN_DATA=$(cat)
{
  printf '=== ARM: %s ===\n--- STDIN ---\n%s\n' "$ARM" "$STDIN_DATA"
  printf -- '--- ENV ---\nZERO=%s\nCWD=%s\nHOME=%s\nPATH=%s\n' "$0" "$(pwd)" "$HOME" "$PATH"
  printf -- '--- PROCESS ---\n'
  ps -o pid,ppid,command -p $$ 2>/dev/null
  PP=$(ps -o ppid= -p $$ 2>/dev/null | tr -d ' ')
  [ -n "$PP" ] && ps -o pid,ppid,command -p "$PP" 2>/dev/null
  [ -n "$PP" ] && GP=$(ps -o ppid= -p "$PP" 2>/dev/null | tr -d ' ') && [ -n "$GP" ] && ps -o pid,ppid,command -p "$GP" 2>/dev/null
  printf '=== END ===\n'
} >> "$LOG"
case "$ARM" in
  allow*)  printf '{"decision":"allow"}';;
  deny*)   printf '{"decision":"deny","reason":"probe deny arm"}';;
  exit2)   echo "probe exit2 stderr" >&2; exit 2;;
  exit1)   exit 1;;
  empty)   : ;;
  badjson) printf 'NOT_JSON';;
  *)       printf '{"decision":"allow"}';;
esac
PROBE
chmod +x "$P/scripts/probe.sh"

write_hooks() {  # $1 = arm
  cat > "$P/.agents/hooks.json" <<EOF
{
  "probe": {
    "PreToolUse": [{"matcher": "run_command", "hooks": [
      {"type": "command", "command": "/bin/sh $P/scripts/probe.sh $1", "timeout": 10}
    ]}]
  }
}
EOF
}

OUT="$P/RESULT-mac.txt"
{ echo "agy: $(agy --version 2>&1)"; echo "date: $(date -u +%FT%TZ)"; echo; } > "$OUT"
n=0; total=7
for a in allow deny exit2 empty badjson exit1 shellid; do
  n=$((n+1))
  rm -f "$M/marker_$a.txt" "$L"/probe_"$a"_*.log "$L/shellid.txt"
  if [ "$a" = shellid ]; then
    cat > "$P/.agents/hooks.json" <<'EOF'
{"probe":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"echo \"0=$0 BASH=$BASH_VERSION ZSH=$ZSH_VERSION PPID=$PPID\" > $HOME/agyprobe/logs/shellid.txt; printf '{\"decision\":\"allow\"}'","timeout":10}]}]}}
EOF
  else
    write_hooks "$a"
  fi
  echo "[$n/$total] braço $a ..."
  agy -p "Run exactly this shell command and nothing else, then stop: echo MARKER_$a > $M/marker_$a.txt" \
      --dangerously-skip-permissions --output-format stream-json --print-timeout 180s \
      > "$L/agy_$a.ndjson" 2> "$L/agy_$a.err"
  rc=$?
  mk=no; [ -f "$M/marker_$a.txt" ] && mk=yes
  lg=$(ls "$L"/probe_"$a"_*.log 2>/dev/null | wc -l | tr -d ' ')
  [ "$a" = shellid ] && lg=$([ -f "$L/shellid.txt" ] && echo 1 || echo 0)
  printf '%-8s agy_rc=%s hook_fired=%s marker=%s\n' "$a" "$rc" "$lg" "$mk" | tee -a "$OUT"
done
echo; echo "Resultado: $OUT  |  logs: $L"
