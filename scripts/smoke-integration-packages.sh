#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-package-smoke.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT HUP INT TERM

"$ROOT_DIR/scripts/check-integration-assets.sh"

mkdir -p "$TMP_ROOT/npm-pack" "$TMP_ROOT/npm-prefix" "$TMP_ROOT/npm-project"
NPM_CONFIG_CACHE="$TMP_ROOT/npm-cache" npm pack --silent --pack-destination "$TMP_ROOT/npm-pack" "$ROOT_DIR/npm" >/dev/null
NPM_TARBALL=$(find "$TMP_ROOT/npm-pack" -type f -name '*.tgz' -print | head -n 1)
test -n "$NPM_TARBALL"
NPM_CONFIG_CACHE="$TMP_ROOT/npm-cache" npm install --no-audit --no-fund --ignore-scripts --prefix "$TMP_ROOT/npm-prefix" "$NPM_TARBALL"
NPM_BIN="$TMP_ROOT/npm-prefix/node_modules/.bin/trackfw"
test -f "$TMP_ROOT/npm-prefix/node_modules/trackfw/src/integrations/assets/catalog.json"
test -f "$TMP_ROOT/npm-prefix/node_modules/trackfw/src/integrations/assets/agents/architect.md"
test -f "$TMP_ROOT/npm-prefix/node_modules/trackfw/src/integrations/assets/skills/governance.md"
(
  cd "$TMP_ROOT/npm-project"
  "$NPM_BIN" agents list --targets codex --items architect --json >/dev/null
  "$NPM_BIN" agents install --targets codex --items architect --json >/dev/null
  "$NPM_BIN" skills install --targets codex --items governance --json >/dev/null
  test -f .codex/agents/trackfw-architect.toml
  test -f .agents/skills/trackfw-governance/SKILL.md
)
echo "npm tarball integration smoke passed"

PYTHON_BIN=${PYTHON_BIN:-python3}
if ! "$PYTHON_BIN" -c 'import build' >/dev/null 2>&1; then
  echo "Python package smoke requires the 'build' module (python -m pip install build)" >&2
  exit 1
fi
mkdir -p "$TMP_ROOT/wheels" "$TMP_ROOT/python-project"
"$PYTHON_BIN" -m build --wheel --outdir "$TMP_ROOT/wheels" "$ROOT_DIR/pypi" >/dev/null
WHEEL=$(find "$TMP_ROOT/wheels" -type f -name '*.whl' -print | head -n 1)
test -n "$WHEEL"
"$PYTHON_BIN" -m venv "$TMP_ROOT/venv"
"$TMP_ROOT/venv/bin/python" -m pip install --quiet --no-deps "$WHEEL"
PY_TRACKFW="$TMP_ROOT/venv/bin/trackfw"
PY_ASSET=$(find "$TMP_ROOT/venv" -type f -path '*/trackfw/integrations/assets/catalog.json' -print | head -n 1)
test -n "$PY_ASSET"
PY_ASSET_DIR=$(dirname "$PY_ASSET")
test -f "$PY_ASSET_DIR/agents/architect.md"
test -f "$PY_ASSET_DIR/skills/governance.md"
(
  cd "$TMP_ROOT/python-project"
  "$PY_TRACKFW" agents list --targets codex --items architect --json >/dev/null
  "$PY_TRACKFW" agents install --targets codex --items architect --json >/dev/null
  "$PY_TRACKFW" skills install --targets codex --items governance --json >/dev/null
  test -f .codex/agents/trackfw-architect.toml
  test -f .agents/skills/trackfw-governance/SKILL.md
)
echo "Python wheel integration smoke passed"
