#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \(.tool_name // "unknown")") | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

TOOL_ESC=$(echo "$TOOL" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$TOOL_ESC" \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
