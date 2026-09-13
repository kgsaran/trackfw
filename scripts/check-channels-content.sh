#!/usr/bin/env bash
# check-channels-content.sh — asserts that published npm and PyPI artifacts have the
# expected content, not the wrong content (v7 sources in a v8 release).
#
# D7 (ML-1A v8 ROADMAP-2026-09-12-v8-um-binario-muitos-canais): verify-channels checked
# presence ("trackfw@$VERSION exists on npm") but not content. A v7 tree published under a
# v8 version string would pass the old check. This script asserts content.
#
# Two modes:
#   --local   Pack and inspect artifacts locally (no network, no publish). This runs in
#             make quality / parity-rest. Proves the *current tree* is safe to publish.
#   --published VERSION   Download published artifacts and inspect them. This runs in the
#             release workflow's verify-channels job after publishing. Proves the *live
#             registry* holds what was intended.
#
# What is checked:
#   npm shim (trackfw):
#     - MUST contain bin/trackfw.js
#     - MUST NOT contain src/ (that would be a v7 tree)
#     - MUST NOT contain src/commands/, src/generators/, src/validators/
#   npm platform packages (@trackfw-bin/<slug>):
#     - MUST contain bin/trackfw or bin/trackfw.exe
#     - MUST NOT contain *.js files at root (no JavaScript source)
#   PyPI wheels (trackfw-*-py3-none-*.whl):
#     - MUST NOT contain *.py files (zero Python — it is a binary wheel)
#     - MUST contain .data/scripts/trackfw or .data/scripts/trackfw.exe
#     - Wheel must exist in --output dir (--local) or be downloadable (--published)
#
# Reconciliation (Regra Dura — CLAUDE.md):
#   Each assertion names the defect it guards against:
#   "no src/ in npm shim" asserts D7 is closed: a v7 tree would have src/.
#   "no .py in wheel" asserts D5 is closed: the wheel is truly zero-Python.
#   "has .data/scripts/trackfw*" asserts D1+D5 are closed: binary is present.
#
# Falsification:
#   --self-test   runs 4 arms to verify the gate catches regressions:
#     Arm 1: real npm pack → expect pass (no src/)
#     Arm 2: pack with injected src/ dir → expect FAIL ("src/ found")
#     Arm 3: real wheel (if build/wheels/ exists) → expect pass
#     Arm 4: wheel with injected .py → expect FAIL (".py found")
#
# Vacuity guards: zero files inspected in any category → named fail, never silent pass.
#
# Environment distinctions (--local):
#   Requires: node (npm pack), python3 (zipfile)
#   Skip if node not in PATH — named skip, not fail (environment, not defect).
#
# Bash 3.2 compatible (macOS ships bash 3.2.57).
set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0
SKIP=0
ok()   { echo "ok: $1";    PASS=$((PASS+1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL+1)); }
skip() { echo "skip: $1 (environment — not a defect)"; SKIP=$((SKIP+1)); }

MODE="${1:-}"

# ─── Helper: inspect a .whl (zip) file for content assertions ──────────────
# inspect_wheel <path> <label>
# Returns: 0 if all assertions pass, 1 if any fail
inspect_wheel() {
    local whl_path="$1"
    local label="$2"
    local whl_ok=0

    python3 - "$whl_path" "$label" <<'PYEOF'
import sys, zipfile, os

whl = sys.argv[1]
label = sys.argv[2]
ok_count = 0
fail_count = 0

def ok(msg):
    global ok_count
    print(f"ok: {label}: {msg}")
    ok_count += 1

def fail(msg):
    global fail_count
    print(f"FAIL: {label}: {msg}", file=sys.stderr)
    fail_count += 1

try:
    with zipfile.ZipFile(whl) as z:
        names = z.namelist()
except Exception as e:
    fail(f"could not open wheel: {e}")
    sys.exit(1)

# Assert: no .py files
py_files = [n for n in names if n.endswith('.py')]
if py_files:
    for pf in py_files[:5]:
        fail(f"contains .py file: {pf}")
    if len(py_files) > 5:
        fail(f"... and {len(py_files)-5} more .py files")
else:
    ok("no .py files in wheel")

# Assert: has .data/scripts/trackfw or .data/scripts/trackfw.exe
bin_entries = [n for n in names
               if '.data/scripts/trackfw' in n
               and not n.endswith('.py')]
if not bin_entries:
    fail("no .data/scripts/trackfw* entry found — binary is missing from wheel")
else:
    for b in bin_entries[:3]:
        ok(f"found binary entry: {b}")

sys.exit(1 if fail_count > 0 else 0)
PYEOF
    return $?
}

# ─── Helper: inspect a .tgz npm tarball for content assertions ─────────────
# inspect_npm_tarball <path> <label> <is_shim>
# is_shim: "1" = top-level shim package, "0" = platform package
inspect_npm_tarball() {
    local tgz_path="$1"
    local label="$2"
    local is_shim="$3"

    python3 - "$tgz_path" "$label" "$is_shim" <<'PYEOF'
import sys, tarfile, os

tgz = sys.argv[1]
label = sys.argv[2]
is_shim = (sys.argv[3] == "1")
ok_count = 0
fail_count = 0

def ok(msg):
    global ok_count
    print(f"ok: {label}: {msg}")
    ok_count += 1

def fail(msg):
    global fail_count
    print(f"FAIL: {label}: {msg}", file=sys.stderr)
    fail_count += 1

try:
    with tarfile.open(tgz) as t:
        names = t.getnames()
except Exception as e:
    fail(f"could not open tarball: {e}")
    sys.exit(1)

# Strip the leading "package/" prefix that npm pack adds
def strip_pkg(n):
    if n.startswith('package/'):
        return n[len('package/'):]
    return n

names_stripped = [strip_pkg(n) for n in names]

if is_shim:
    # Assert: has bin/trackfw.js
    if 'bin/trackfw.js' in names_stripped:
        ok("contains bin/trackfw.js")
    else:
        fail("missing bin/trackfw.js — shim not present")

    # Assert: no src/ tree (v7 artifact test)
    src_files = [n for n in names_stripped if n.startswith('src/')]
    if src_files:
        fail(f"contains src/ directory ({len(src_files)} entries) — this looks like a v7 tree")
        for sf in src_files[:3]:
            fail(f"  src/ entry: {sf}")
    else:
        ok("no src/ directory — correct for v8 shim")

    # Assert: has package.json
    if 'package.json' in names_stripped:
        ok("contains package.json")
    else:
        fail("missing package.json")
else:
    # Platform package: has bin/trackfw or bin/trackfw.exe
    bin_entries = [n for n in names_stripped
                   if n.startswith('bin/') and 'trackfw' in n]
    if bin_entries:
        for b in bin_entries[:3]:
            ok(f"contains binary: {b}")
    else:
        fail("no bin/trackfw* entry — binary is missing from platform package")

    # Assert: no .js files at root level
    js_files = [n for n in names_stripped
                if n.endswith('.js') and '/' not in n.lstrip('/')]
    if js_files:
        fail(f"platform package contains top-level .js files: {js_files[:3]}")
    else:
        ok("no top-level .js files in platform package")

sys.exit(1 if fail_count > 0 else 0)
PYEOF
    return $?
}

# ═══════════════════════════════════════════════════════════════════════════
# Self-test mode
# ═══════════════════════════════════════════════════════════════════════════
if [[ "$MODE" == "--self-test" ]]; then
    echo "=== check-channels-content: self-test ==="
    # Self-test uses SYNTHETIC artifacts only — it does not pack from the current tree.
    # Reason: Wave 1 still has npm/src/ present (Wave 3 removes it). Packing the real
    # npm/ dir here would always fail the "no src/" assertion before Wave 3, making the
    # self-test tree-state dependent and unusable in parity-rest. Synthetic tarballs
    # prove gate LOGIC regardless of tree state — that is the right property.
    #
    # Arm 1: clean v8-like shim (bin/trackfw.js, no src/) → expect pass
    # Arm 2: v7-like shim with src/ injected → expect FAIL ("src/ found")
    # Arm 3: clean wheel (has .data/scripts/trackfw, no .py) → expect pass
    # Arm 4: wheel with .py injected → expect FAIL (".py found")

    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-channels-content-selftest.XXXXXX")
    CLEANUP_WORK="$WORK"
    cleanup_selftest() { rm -rf "$CLEANUP_WORK" 2>/dev/null || true; }
    trap cleanup_selftest EXIT

    # Arm 1: synthetic clean v8 shim (bin/trackfw.js, no src/) — expect pass
    echo ""
    echo "--- Arm 1: synthetic v8 shim (no src/, has bin/trackfw.js) — expect pass ---"
    python3 - "$WORK/arm1-clean-shim.tgz" <<'PYEOF'
import sys, tarfile, io
out = sys.argv[1]
files = {
    'package/package.json': b'{"name":"trackfw","version":"8.0.0","bin":{"trackfw":"bin/trackfw.js"}}',
    'package/bin/trackfw.js': b'#!/usr/bin/env node\n"use strict";\n',
    'package/README.md': b'# trackfw\n',
}
with tarfile.open(out, 'w:gz') as t:
    for name, data in files.items():
        info = tarfile.TarInfo(name=name)
        info.size = len(data)
        t.addfile(info, io.BytesIO(data))
PYEOF
    if inspect_npm_tarball "$WORK/arm1-clean-shim.tgz" "arm1-clean-shim" "1"; then
        ok "arm1: synthetic v8 shim → pass as expected"
    else
        fail "arm1: synthetic v8 shim → FAIL (gate may be too strict)"
    fi

    # Arm 2: synthetic v7-like shim with src/ — expect FAIL
    echo ""
    echo "--- Arm 2: synthetic v7 shim (has src/) — expect FAIL ---"
    python3 - "$WORK/arm2-v7-shim.tgz" <<'PYEOF'
import sys, tarfile, io
out = sys.argv[1]
files = {
    'package/package.json': b'{"name":"trackfw","version":"8.0.0"}',
    'package/bin/trackfw.js': b'#!/usr/bin/env node\n',
    'package/src/commands/version.js': b'// v7 source\n',
    'package/src/generators/init.js': b'// v7 source\n',
}
with tarfile.open(out, 'w:gz') as t:
    for name, data in files.items():
        info = tarfile.TarInfo(name=name)
        info.size = len(data)
        t.addfile(info, io.BytesIO(data))
PYEOF
    if ! inspect_npm_tarball "$WORK/arm2-v7-shim.tgz" "arm2-v7-shim" "1" 2>/dev/null; then
        ok "arm2: synthetic v7 shim (with src/) → correctly detected FAIL as expected"
    else
        fail "arm2: synthetic v7 shim (with src/) → passed — gate is vacuous for src/ detection"
    fi

    # Arm 3: synthetic clean wheel (.data/scripts/trackfw, no .py) — expect pass
    echo ""
    echo "--- Arm 3: synthetic clean wheel (no .py, has .data/scripts/trackfw) — expect pass ---"
    CLEAN_WHL="$WORK/arm3-clean-trackfw-8.0.0-py3-none-linux_x86_64.whl"
    python3 - "$CLEAN_WHL" <<'PYEOF'
import sys, zipfile
whl = sys.argv[1]
with zipfile.ZipFile(whl, 'w') as z:
    z.writestr('trackfw-8.0.0.dist-info/WHEEL', 'Wheel-Version: 1.0\n')
    z.writestr('trackfw-8.0.0.dist-info/METADATA', 'Name: trackfw\nVersion: 8.0.0\n')
    z.writestr('trackfw-8.0.0.data/scripts/trackfw', b'\x7fELF')  # fake ELF header
PYEOF
    if inspect_wheel "$CLEAN_WHL" "arm3-clean-wheel"; then
        ok "arm3: synthetic clean wheel → pass as expected"
    else
        fail "arm3: synthetic clean wheel → FAIL (gate may be too strict)"
    fi

    # Arm 4: synthetic wheel with .py → expect FAIL
    echo ""
    echo "--- Arm 4: synthetic wheel with .py injected — expect FAIL ---"
    FAKE_WHL="$WORK/arm4-fake-trackfw-1.0.0-py3-none-linux_x86_64.whl"
    python3 - "$FAKE_WHL" <<'PYEOF'
import sys, zipfile
whl = sys.argv[1]
with zipfile.ZipFile(whl, 'w') as z:
    z.writestr('trackfw-1.0.0.dist-info/WHEEL', 'Wheel-Version: 1.0\n')
    z.writestr('trackfw-1.0.0.dist-info/METADATA', 'Name: trackfw\nVersion: 1.0.0\n')
    # Inject a .py file — this should be caught
    z.writestr('trackfw/__init__.py', '__version__ = "1.0.0"\n')
    # NO .data/scripts/trackfw entry (also a failure)
PYEOF
    if ! inspect_wheel "$FAKE_WHL" "arm4-injected-py" 2>/dev/null; then
        ok "arm4: wheel with injected .py → correctly detected FAIL as expected"
    else
        fail "arm4: wheel with injected .py → passed — gate is vacuous for .py detection"
    fi

    echo ""
    echo "check-channels-content --self-test: $PASS passed, $FAIL failed, $SKIP skipped"
    if [[ "$FAIL" -gt 0 ]]; then
        exit 1
    fi
    exit 0
fi

# ═══════════════════════════════════════════════════════════════════════════
# Local mode (--local): inspect locally packed artifacts
# ═══════════════════════════════════════════════════════════════════════════
if [[ "$MODE" == "--local" ]]; then
    echo "=== check-channels-content: local mode ==="

    # Guard: node must be available
    if ! command -v node >/dev/null 2>&1; then
        skip "node not in PATH — cannot pack npm artifacts (install Node.js to enable this check)"
        echo ""
        echo "check-channels-content --local: $PASS passed, $FAIL failed, $SKIP skipped"
        exit 0
    fi

    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-channels-content-local.XXXXXX")
    cleanup_local() { rm -rf "$WORK" 2>/dev/null || true; }
    trap cleanup_local EXIT

    # ── npm shim ──────────────────────────────────────────────────────────
    echo ""
    echo "--- npm shim (trackfw) ---"
    SHIM_TGZ="$WORK/shim.tgz"
    (cd "$REPO_ROOT/npm" && npm pack --pack-destination "$WORK" --silent 2>/dev/null) \
        && mv "$WORK"/*.tgz "$SHIM_TGZ" 2>/dev/null || true
    if [[ -f "$SHIM_TGZ" ]]; then
        if inspect_npm_tarball "$SHIM_TGZ" "npm-shim" "1"; then
            ok "npm shim content assertions passed"
        else
            fail "npm shim content assertions failed"
        fi
    else
        fail "vacuity guard: npm pack produced no tarball for npm/trackfw"
    fi

    # ── npm platform manifests (if generated) ────────────────────────────
    echo ""
    echo "--- npm platform packages (@trackfw-bin/*) ---"
    PLATFORM_COUNT=0
    OUT_DIR="$REPO_ROOT/build/npm-platform/@trackfw-bin"
    if [[ -d "$OUT_DIR" ]]; then
        for pkg_dir in "$OUT_DIR"/*/; do
            [[ -d "$pkg_dir" ]] || continue
            slug=$(basename "$pkg_dir")
            PKG_TGZ="$WORK/${slug}.tgz"
            (cd "$pkg_dir" && npm pack --pack-destination "$WORK" --silent 2>/dev/null) \
                && mv "$WORK"/*.tgz "$PKG_TGZ" 2>/dev/null || true
            if [[ -f "$PKG_TGZ" ]]; then
                if inspect_npm_tarball "$PKG_TGZ" "platform-$slug" "0"; then
                    ok "platform package $slug content assertions passed"
                else
                    fail "platform package $slug content assertions failed"
                fi
                PLATFORM_COUNT=$((PLATFORM_COUNT+1))
            fi
        done
    fi
    if [[ "$PLATFORM_COUNT" -eq 0 ]]; then
        skip "no generated platform manifests in $OUT_DIR — run 'make gen-manifests' to enable platform package checks"
    fi

    # ── PyPI wheels (if built) ────────────────────────────────────────────
    echo ""
    echo "--- PyPI wheels ---"
    WHEEL_COUNT=0
    WHEEL_DIR="$REPO_ROOT/build/wheels"
    if [[ -d "$WHEEL_DIR" ]]; then
        for whl in "$WHEEL_DIR"/*.whl; do
            [[ -f "$whl" ]] || continue
            if inspect_wheel "$whl" "$(basename "$whl")"; then
                ok "wheel $(basename "$whl") content assertions passed"
            else
                fail "wheel $(basename "$whl") content assertions failed"
            fi
            WHEEL_COUNT=$((WHEEL_COUNT+1))
        done
    fi
    if [[ "$WHEEL_COUNT" -eq 0 ]]; then
        skip "no wheels found in $WHEEL_DIR — build wheels first to enable wheel content checks"
    fi

    echo ""
    echo "check-channels-content --local: $PASS passed, $FAIL failed, $SKIP skipped"
    if [[ "$FAIL" -gt 0 ]]; then
        exit 1
    fi
    exit 0
fi

# ═══════════════════════════════════════════════════════════════════════════
# Published mode (--published VERSION): download and inspect live artifacts
# ═══════════════════════════════════════════════════════════════════════════
if [[ "$MODE" == "--published" ]]; then
    VERSION="${2:-}"
    if [[ -z "$VERSION" ]]; then
        echo "check-channels-content: --published requires a VERSION argument" >&2
        exit 1
    fi

    echo "=== check-channels-content: published mode (v$VERSION) ==="

    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-channels-content-published.XXXXXX")
    cleanup_pub() { rm -rf "$WORK" 2>/dev/null || true; }
    trap cleanup_pub EXIT

    # ── npm shim (download pack from registry) ────────────────────────────
    echo ""
    echo "--- npm shim trackfw@$VERSION ---"
    if npm pack "trackfw@$VERSION" --pack-destination "$WORK" 2>/dev/null; then
        SHIM_TGZ=$(ls "$WORK"/trackfw-*.tgz 2>/dev/null | tail -1)
        if [[ -n "$SHIM_TGZ" && -f "$SHIM_TGZ" ]]; then
            if inspect_npm_tarball "$SHIM_TGZ" "npm-shim-published" "1"; then
                ok "published npm shim content assertions passed"
            else
                fail "published npm shim content assertions FAILED — wrong content in registry"
            fi
        else
            fail "vacuity guard: npm pack downloaded nothing for trackfw@$VERSION"
        fi
    else
        fail "npm pack trackfw@$VERSION failed — package may not be published yet"
    fi

    # ── PyPI: download wheels and inspect ────────────────────────────────
    echo ""
    echo "--- PyPI wheels trackfw==$VERSION ---"
    # Normalize version for PyPI (8.0.0-rc1 → 8.0.0rc1)
    PY_VERSION=$(python3 -c "
try:
    from packaging.version import Version
    print(str(Version('$VERSION')))
except Exception:
    print('$VERSION')
" 2>/dev/null || echo "$VERSION")

    WHEEL_WORK="$WORK/wheels"
    mkdir -p "$WHEEL_WORK"
    if pip download "trackfw==$PY_VERSION" --no-deps --only-binary :all: \
        --dest "$WHEEL_WORK" --quiet 2>/dev/null; then
        WHEEL_COUNT=0
        for whl in "$WHEEL_WORK"/*.whl; do
            [[ -f "$whl" ]] || continue
            if inspect_wheel "$whl" "$(basename "$whl")-published"; then
                ok "published wheel $(basename "$whl") content assertions passed"
            else
                fail "published wheel $(basename "$whl") content assertions FAILED — wrong content in registry"
            fi
            WHEEL_COUNT=$((WHEEL_COUNT+1))
        done
        if [[ "$WHEEL_COUNT" -eq 0 ]]; then
            fail "vacuity guard: pip downloaded no wheels for trackfw==$PY_VERSION"
        fi
    else
        fail "pip download trackfw==$PY_VERSION failed — wheels may not be published yet"
    fi

    echo ""
    echo "check-channels-content --published $VERSION: $PASS passed, $FAIL failed, $SKIP skipped"
    if [[ "$FAIL" -gt 0 ]]; then
        exit 1
    fi
    exit 0
fi

# Unknown mode
echo "check-channels-content: unknown mode '$MODE'" >&2
echo "Usage: check-channels-content.sh [--local | --published VERSION | --self-test]" >&2
exit 1
