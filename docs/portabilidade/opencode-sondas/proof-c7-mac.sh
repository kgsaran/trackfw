#!/usr/bin/env bash
# ML-5B C7 (REQ-2026-11-17) — git commit cru sob o OpenCode deve ser bloqueado pelo guard de git-branch.
# 1 chamada de modelo (1/1). Reusa ~/ocprobe/proof criado pelo proof-mac.sh. Uso:
#   OC_MODEL=anthropic/claude-haiku-5-5 bash proof-c7-mac.sh
set -u
REPO="${REPO:-$HOME/Sistemas/Desenvolvimento/workspace/trackfw}"
P="$HOME/ocprobe/proof"; L="$P/logs"
[ -d "$P/.git" ] || { echo "ABORT: rode antes o proof-mac.sh"; exit 3; }
mkdir -p "$P/.opencode/plugins" "$L"
cp "$REPO/docs/portabilidade/opencode-sondas/guard-probe-plugin.js" "$P/.opencode/plugins/trackfw-guard-probe.js"
cp "$REPO/bin/trackfw" "$P/bin/trackfw"
export PATH="$P/bin:$PATH"
cd "$P" || exit 1
before=$(git rev-list --count HEAD)
export TRACKFW_PROBE_LOG="$L/plugin_C7.log"; rm -f "$TRACKFW_PROBE_LOG"
MODEL_ARGS=(); [ -n "${OC_MODEL:-}" ] && MODEL_ARGS=(--model "$OC_MODEL")
opencode run --auto --format json --standalone ${MODEL_ARGS[@]+"${MODEL_ARGS[@]}"} \
  "Run exactly this shell command and nothing else, then stop: git commit --allow-empty -m probe" > "$L/oc_C7.json" 2> "$L/oc_C7.err"
echo "C7 oc_rc=$? branch=$(git branch --show-current) commits antes=$before depois=$(git rev-list --count HEAD)   (esperado: iguais)"
echo "  plugin_log=$(cat "$TRACKFW_PROBE_LOG" 2>/dev/null || echo '(vazio)')"
grep -q '"type":"error"' "$L/oc_C7.json" && echo "  INCONCLUSIVO: erro de modelo em oc_C7.json"
