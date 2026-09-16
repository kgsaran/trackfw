#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-package-smoke.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT HUP INT TERM

"$ROOT_DIR/scripts/check-integration-assets.sh"

# ── npm shim (v8) ────────────────────────────────────────────────────────────
# In v8 the npm package ships only the shim (bin/trackfw.js). The integration
# assets (catalog, agents, skills) are embedded in the Go binary via go:embed
# (internal/integrations/catalog.go:15 //go:embed assets). The binary is
# delivered via the @trackfw-bin/<platform> optional package, not by npm/src/.
#
# This smoke:
#   (1) Packs the shim tarball from npm/ and verifies v8 structure
#       (shim present, no src/ directory — would indicate a v7 tree).
#   (2) Stages a local platform package in $TMP_ROOT with the built binary
#       and NO `bin` field (AC13) — proves AC1+AC13 together.
#   (3) Installs both tarballs together, then runs agents/skills commands
#       through the shim, which delegates to the Go binary (which embeds assets).
#
# Hard-fail if binary absent — a named skip here would be the same vacuity
# class fixed in check-shim-byte-identity.sh and check-install-restriction.sh.
# package-smoke depends on `make build` (see Makefile).
#
# Reconciliation: replacing lines 16-18 (src/integrations/assets/ assertions)
# with (1)+(2)+(3). The protected property — "the installed npm package can
# serve catalog and agents" — holds because the Go binary embeds the assets.
# The assertion moves from "assets in src/ tarball" to "shim delegates to
# binary which serves assets via go:embed". Falsification: binary absent →
# named hard-fail at line ~55; src/ present in tarball → test ! -d assertion.

OS_NAME=$(node -e "const os=require('os');process.stdout.write(os.platform())")
ARCH_NAME=$(node -e "const os=require('os');process.stdout.write(os.arch())")
PLATFORM="${OS_NAME}-${ARCH_NAME}"
if [ "$OS_NAME" = "win32" ]; then
  BIN_FILENAME="trackfw.exe"
else
  BIN_FILENAME="trackfw"
fi
LOCAL_BIN="$ROOT_DIR/bin/$BIN_FILENAME"

# Hard-fail: binary must exist. package-smoke: build check-integration-assets.
if [ ! -f "$LOCAL_BIN" ]; then
  echo "FAIL: $LOCAL_BIN not found — package-smoke requires 'make build' (Makefile dependency)" >&2
  exit 1
fi

mkdir -p "$TMP_ROOT/npm-pack" "$TMP_ROOT/npm-prefix" "$TMP_ROOT/npm-project"

# Pack the shim tarball.
NPM_CONFIG_CACHE="$TMP_ROOT/npm-cache" npm pack --silent --pack-destination "$TMP_ROOT/npm-pack" "$ROOT_DIR/npm" >/dev/null
NPM_TARBALL=$(find "$TMP_ROOT/npm-pack" -type f -name 'trackfw-*.tgz' -print | head -n 1)
test -n "$NPM_TARBALL" || { echo "FAIL: npm pack produced no tarball" >&2; exit 1; }

# Stage a local platform package with NO `bin` field (AC13).
# Staged in $TMP_ROOT to keep the repo tree clean — not in build/npm-platform/.
PLAT_DIR="$TMP_ROOT/platform-pkg/@trackfw-bin/$PLATFORM"
mkdir -p "$PLAT_DIR/bin"
cp "$LOCAL_BIN" "$PLAT_DIR/bin/$BIN_FILENAME"
# Write package.json: no `bin` field (AC13) — shim resolves via subpath, not bin map.
cat > "$PLAT_DIR/package.json" << PKGJSON
{
  "name": "@trackfw-bin/$PLATFORM",
  "version": "0.0.1-smoke",
  "description": "smoke-test local platform package — no bin field (AC13)",
  "os": ["$OS_NAME"],
  "cpu": ["$ARCH_NAME"],
  "files": ["bin/$BIN_FILENAME"]
}
PKGJSON

NPM_CONFIG_CACHE="$TMP_ROOT/npm-cache" npm pack --silent --pack-destination "$TMP_ROOT/npm-pack" "$PLAT_DIR" >/dev/null
PLATFORM_TARBALL=$(find "$TMP_ROOT/npm-pack" -type f -name "trackfw-bin-*.tgz" -print | head -n 1)
test -n "$PLATFORM_TARBALL" || { echo "FAIL: npm pack produced no tarball for platform package" >&2; exit 1; }

# Install shim + platform together in one npm install (proves AC1+AC13 simultaneously).
NPM_CONFIG_CACHE="$TMP_ROOT/npm-cache" npm install --no-audit --no-fund --ignore-scripts --prefix "$TMP_ROOT/npm-prefix" "$NPM_TARBALL" "$PLATFORM_TARBALL"

# v8 structural assertions (replaces v7 src/ asset assertions).
# Reconciliation: "shim is present in installed package" asserts the bin field in
# npm/package.json correctly points to bin/trackfw.js and `files` includes it.
test -f "$TMP_ROOT/npm-prefix/node_modules/trackfw/bin/trackfw.js" \
  || { echo "FAIL: shim bin/trackfw.js not in installed package" >&2; exit 1; }
# Reconciliation: "no src/ in installed package" asserts `files` excludes npm/src/ —
# the v7 tree would include it; the v8 shim must not.
test ! -d "$TMP_ROOT/npm-prefix/node_modules/trackfw/src" \
  || { echo "FAIL: src/ present in installed shim package — v7 tree leaked into tarball" >&2; exit 1; }

NPM_BIN="$TMP_ROOT/npm-prefix/node_modules/.bin/trackfw"

# --scope project is required, not redundant: since
# ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills the
# non-interactive default is `global` (~/.codex/...), so without the flag the
# `test -f .codex/...` assertions below would look in the wrong place. This is
# exactly the CI migration the CHANGELOG breaking-change note prescribes.
(
  cd "$TMP_ROOT/npm-project"
  "$NPM_BIN" agents list --targets codex --items architect --json >/dev/null
  "$NPM_BIN" agents install --targets codex --items architect --scope project --json >/dev/null
  "$NPM_BIN" skills install --targets codex --items governance --scope project --json >/dev/null
  test -f .codex/agents/trackfw-architect.toml
  test -f .agents/skills/trackfw-governance/SKILL.md
)
echo "npm tarball integration smoke passed"

PYTHON_BIN=${PYTHON_BIN:-python3}
# ── Python wheel (v8 binary wheel) ─────────────────────────────────────────
# v8 delivers a binary-only wheel (no Python source). pypi/scripts/build_wheel.py
# assembles the wheel directly from the compiled Go binary, using the same
# mechanism as the release pipeline. This smoke:
#   (1) Detects the local platform tag from OS_NAME/ARCH_NAME (already resolved above).
#   (2) Reads the package version from npm/package.json (single source of truth).
#   (3) Builds the binary wheel via pypi/scripts/build_wheel.py.
#       Requires only `packaging` (already installed by the CI job).
#   (4) Installs the wheel in an isolated venv.
#   (5) Runs functional commands through the installed binary to prove that
#       the wheel delivers a working trackfw executable.
#
# Reconciliation (ML-4C): replaced `python -m build --wheel pypi/` (which
# required pypi/trackfw/ as a Python source package, deleted in ML-3A) with
# build_wheel.py. catalog.json and agent/skill file assertions removed: in v8
# the integration assets are embedded in the Go binary via go:embed; they are
# not Python package data files installed into site-packages.
# build_wheel.py requires only `packaging`, already in the CI pip install.
#
# Falsification: binary absent → hard-fail at line ~46 above;
# wheel not produced → `test -n "$WHEEL"` hard-fail;
# binary not installed → `test -x "$PY_TRACKFW"` hard-fail;
# functional commands fail → subshell exits non-zero.
PYPI_PLATFORM=$(case "${OS_NAME}-${ARCH_NAME}" in
  darwin-arm64)  echo "macosx_11_0_arm64" ;;
  darwin-x64)    echo "macosx_10_9_x86_64" ;;
  linux-x64)     echo "manylinux_2_17_x86_64" ;;
  linux-arm64)   echo "manylinux_2_17_aarch64" ;;
  win32-x64)     echo "win_amd64" ;;
  win32-arm64)   echo "win_arm64" ;;
  *)             echo "manylinux_2_17_x86_64" ;;
esac)
PYPI_VERSION=$(node -e "process.stdout.write(require('$ROOT_DIR/npm/package.json').version)")
mkdir -p "$TMP_ROOT/wheels" "$TMP_ROOT/python-project"
"$PYTHON_BIN" "$ROOT_DIR/pypi/scripts/build_wheel.py" \
    --binary "$LOCAL_BIN" \
    --version "$PYPI_VERSION" \
    --platform "$PYPI_PLATFORM" \
    --output "$TMP_ROOT/wheels"
WHEEL=$(find "$TMP_ROOT/wheels" -type f -name '*.whl' -print | head -n 1)
test -n "$WHEEL" || { echo "FAIL: build_wheel.py produced no wheel" >&2; exit 1; }
"$PYTHON_BIN" -m venv "$TMP_ROOT/venv"
"$TMP_ROOT/venv/bin/python" -m pip install --quiet "$WHEEL"
PY_TRACKFW="$TMP_ROOT/venv/bin/trackfw"
test -x "$PY_TRACKFW" || { echo "FAIL: trackfw not executable in venv bin" >&2; exit 1; }
(
  cd "$TMP_ROOT/python-project"
  "$PY_TRACKFW" agents list --targets codex --items architect --json >/dev/null
  "$PY_TRACKFW" agents install --targets codex --items architect --scope project --json >/dev/null
  "$PY_TRACKFW" skills install --targets codex --items governance --scope project --json >/dev/null
  test -f .codex/agents/trackfw-architect.toml
  test -f .agents/skills/trackfw-governance/SKILL.md
)
echo "Python wheel integration smoke passed"
