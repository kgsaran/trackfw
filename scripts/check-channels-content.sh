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
#   --self-test   runs arms to verify the gate catches regressions:
#     Arm 1: real npm pack → expect pass (no src/)
#     Arm 2: pack with injected src/ dir → expect FAIL ("src/ found")
#     Arm 3: real wheel (if build/wheels/ exists) → expect pass
#     Arm 4: wheel with injected .py → expect FAIL (".py found")
#     Arm 5: PyPI retry positivo — JSON API unavailable 2×, available 3rd → exit 0
#     Arm 6: PyPI retry negativo — JSON API always fails until deadline → exit 1
#     Arm 7: PyPI returns 7 wheels (missing win_arm64) → exit 1, tag named
#     Arm 8: PyPI wheels have wrong content → exit 1, SLEEP never called (no retry)
#     Arm 9: npm retry positivo — npm pack fails 2×, succeeds 3rd → exit 0
#
# Reconciliation for new arms (Regra Dura — CLAUDE.md):
#   Arm 5 asserts: the retry loop eventually fetches all 8 wheels and exits 0 when
#     PYPI_JSON_CMD becomes available on the 3rd attempt.
#   Arm 6 asserts: the retry loop exits 1 (never silent pass) when the deadline is
#     exhausted before PYPI_JSON_CMD responds.
#   Arm 7 asserts: a missing wheel tag (7 instead of 8) is reported as FAIL by name.
#   Arm 8 asserts: a wheel with wrong content fails immediately (exit 1) without
#     triggering the retry loop (VERIFY_CONTENT_SLEEP_CMD is never called).
#   Arm 9 asserts: the npm pack retry loop exits 0 and inspects a valid tarball when
#     NPM_PACK_CMD becomes available on the 3rd attempt.
#
# Vacuity guards: zero files inspected in any category → named fail, never silent pass.
#
# Environment distinctions (--local):
#   Requires: node (npm pack), python3 (zipfile)
#   Skip if node not in PATH — named skip, not fail (environment, not defect).
#
# Injectable commands (for --self-test and CI override):
#   PYPI_JSON_CMD  cmd <full_json_url>  → stdout: PyPI JSON API response
#   FETCH_CMD      cmd <url> <dest>     → downloads url to dest
#   NPM_PACK_CMD   cmd <spec> <destdir> → creates tarball in destdir
#   VERIFY_CONTENT_SLEEP_CMD  cmd <secs>  → delays (default: sleep)
#   VERIFY_CONTENT_NOW_CMD    cmd         → prints epoch seconds (default: date +%s)
#   VERIFY_CONTENT_DEADLINE   seconds     → retry deadline (default: 900)
#   (FAKE_CLOCK_FILE: used by self-test stubs; no effect in normal operation)
#
# Bash 3.2 compatible (macOS ships bash 3.2.57).
set -euo pipefail
export PYTHONIOENCODING=utf-8

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$SCRIPT_DIR/lib-crlf-normalize.sh"

PASS=0
FAIL=0
SKIP=0
ok()   { echo "ok: $1";    PASS=$((PASS+1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL+1)); }
skip() { echo "skip: $1 (environment — not a defect)"; SKIP=$((SKIP+1)); }

MODE="${1:-}"

# ─── Injectable command defaults ─────────────────────────────────────────────
# These can be overridden by environment variables (for self-test without network).
# Each var is a single command word (or "python3 /path/to/stub.py") — no embedded flags.
# The wrappers below add the proper argument contract.

# VERIFY_CONTENT_DEADLINE: total retry window in seconds.
# Default: 900 s — covers CDN max-age=600 s (measured on PyPI Simple Index)
# with ~300 s margin. Empirical data: two runs failed at 165 s and 193 s
# post-upload (v9.3.3 and v9.4.1 on 2026-10-08/09); rerun succeeded at ~208 s
# and ~329 s. 900 s = max-age + 300 s safety margin.
# Source: docs/seguranca/2026-10-09-wave0-verificacao-canais-indice-pypi.md §4
VERIFY_CONTENT_DEADLINE="${VERIFY_CONTENT_DEADLINE:-900}"
if ! printf '%s' "${VERIFY_CONTENT_DEADLINE}" | grep -qE '^[0-9]+$' \
    || [[ "${VERIFY_CONTENT_DEADLINE}" -le 0 ]]; then
    echo "check-channels-content: VERIFY_CONTENT_DEADLINE must be a positive integer" \
         "(got '${VERIFY_CONTENT_DEADLINE}'); using 900" >&2
    VERIFY_CONTENT_DEADLINE=900
fi

# Expected wheel platform tags — from release.yml publish-pypi job lines 342–347:
#   linux amd64  → manylinux_2_17_x86_64  musllinux_1_2_x86_64
#   linux arm64  → manylinux_2_17_aarch64 musllinux_1_2_aarch64
#   darwin amd64 → macosx_10_9_x86_64
#   darwin arm64 → macosx_11_0_arm64
#   win   amd64  → win_amd64
#   win   arm64  → win_arm64
# Total: 8. Update here AND in release.yml if the platform matrix changes.
EXPECTED_WHEEL_TAGS="macosx_10_9_x86_64,macosx_11_0_arm64,manylinux_2_17_aarch64,manylinux_2_17_x86_64,musllinux_1_2_aarch64,musllinux_1_2_x86_64,win_amd64,win_arm64"
EXPECTED_WHEEL_COUNT=8

# ─── Injectable command wrappers ─────────────────────────────────────────────

# _do_pypi_json <py_version>  → stdout: full JSON from PyPI JSON API
_do_pypi_json() {
    local py_version="$1"
    local url="https://pypi.org/pypi/trackfw/${py_version}/json"
    if [[ -n "${PYPI_JSON_CMD:-}" ]]; then
        $PYPI_JSON_CMD "$url"
    else
        curl -sf "$url"
    fi
}

# _do_fetch <url> <dest>  → downloads url to dest file
_do_fetch() {
    local url="$1"
    local dest="$2"
    if [[ -n "${FETCH_CMD:-}" ]]; then
        $FETCH_CMD "$url" "$dest"
    else
        curl -fSL -o "$dest" "$url"
    fi
}

# _do_npm_pack <spec> <destdir>  → creates npm tarball in destdir
_do_npm_pack() {
    local spec="$1"
    local destdir="$2"
    if [[ -n "${NPM_PACK_CMD:-}" ]]; then
        $NPM_PACK_CMD "$spec" "$destdir"
    else
        npm pack "$spec" --pack-destination "$destdir"
    fi
}

# _do_sleep <secs>  → delays (injectable for tests)
_do_sleep() {
    local secs="$1"
    if [[ -n "${VERIFY_CONTENT_SLEEP_CMD:-}" ]]; then
        $VERIFY_CONTENT_SLEEP_CMD "$secs"
    else
        sleep "$secs"
    fi
}

# _do_now  → prints current epoch seconds (injectable for tests)
_do_now() {
    if [[ -n "${VERIFY_CONTENT_NOW_CMD:-}" ]]; then
        $VERIFY_CONTENT_NOW_CMD 2>/dev/null || date +%s
    else
        date +%s
    fi
}

# ─── Helper: inspect a .whl (zip) file for content assertions ──────────────
# inspect_wheel <path> <label>
# Returns: 0 if all assertions pass, 1 if any fail
inspect_wheel() {
    local whl_path="$1"
    local label="$2"

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

    # Assert: no bin/trackfw (the v7 Node entry, without .js extension)
    # "conteúdo v7 sob nome v8" is the accident this REQ exists to prevent — ML-1B.
    if 'bin/trackfw' in names_stripped:
        fail("contains bin/trackfw (v7 Node entry without .js) — old implementation must not be in v8 tarball")
    else:
        ok("no bin/trackfw v7 entry — correct for v8 shim")

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

# ─── Retry: download npm tarball from registry with backoff ──────────────────
# _retry_npm_pack <spec> <destdir>
# Retries _do_npm_pack with backoff until deadline. Content check is NOT here.
# Returns: 0 on success, 1 if deadline exhausted.
_retry_npm_pack() {
    local spec="$1"
    local dest="$2"
    local deadline="$VERIFY_CONTENT_DEADLINE"
    local interval=10
    local max_interval=60
    local start
    start=$(_do_now)
    echo "  npm pack $spec (deadline: ${deadline}s, backoff: ${interval}s→${max_interval}s)"

    while true; do
        # Attempt first, check deadline after
        if _do_npm_pack "$spec" "$dest" 2>/dev/null; then
            return 0
        fi
        local elapsed=$(( $(_do_now) - start ))
        if [[ "$elapsed" -ge "$deadline" ]]; then
            echo "FAIL: deadline ${deadline}s exhausted after ${elapsed}s — npm pack $spec" >&2
            return 1
        fi
        local remaining=$(( deadline - elapsed ))
        local sleep_time
        if [[ "$interval" -lt "$remaining" ]]; then
            sleep_time="$interval"
        else
            sleep_time="$remaining"
        fi
        if [[ "$sleep_time" -le 0 ]]; then
            sleep_time=1
        fi
        elapsed=$(( $(_do_now) - start ))
        echo "  npm pack: retry in ${sleep_time}s... (+${elapsed}s)"
        _do_sleep "$sleep_time"
        interval=$(( interval * 2 ))
        if [[ "$interval" -gt "$max_interval" ]]; then
            interval="$max_interval"
        fi
    done
}

# ─── Retry: fetch all wheels from PyPI JSON API with backoff ─────────────────
# _fetch_pypi_wheels_with_retry <py_version> <destdir>
# Uses the PyPI JSON API (/pypi/trackfw/<ver>/json) to get wheel URLs and downloads
# each directly from files.pythonhosted.org — bypasses the CDN-cached Simple Index
# (/simple/) that caused the 165 s and 193 s failures in v9.3.3 and v9.4.1.
# Retries the entire cycle (JSON fetch + all downloads) on any failure.
# Content inspection is NOT done here — caller inspects after return 0.
# Returns: 0 when all wheel URLs resolved and downloaded, 1 if deadline exhausted.
_fetch_pypi_wheels_with_retry() {
    local py_version="$1"
    local dest="$2"
    local deadline="$VERIFY_CONTENT_DEADLINE"
    local interval=10
    local max_interval=60
    local start
    start=$(_do_now)

    echo "  PyPI wheels for trackfw==$py_version via JSON API (deadline: ${deadline}s)"
    echo "  (JSON API bypasses CDN Simple Index; files from files.pythonhosted.org)"

    while true; do
        # Attempt: fetch JSON → parse wheel URLs → download all
        local json_out
        json_out=$(_do_pypi_json "$py_version" 2>/dev/null) || json_out=""
        local attempt_ok=0

        if [[ -n "$json_out" ]]; then
            local wheel_urls
            wheel_urls=$(printf '%s' "$json_out" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for u in data.get('urls', []):
        if u.get('packagetype') == 'bdist_wheel':
            print(u['url'])
except Exception:
    sys.exit(1)
" 2>/dev/null | strip_cr) || wheel_urls=""

            if [[ -n "$wheel_urls" ]]; then
                local all_dl_ok=1
                while IFS= read -r url; do
                    # [[ -n ]] || continue: always exits 0 under set -e (safe form of skip-if-empty)
                    [[ -n "$url" ]] || continue
                    local filename
                    filename=$(basename "$url")
                    if ! _do_fetch "$url" "$dest/$filename" 2>/dev/null; then
                        all_dl_ok=0
                        break
                    fi
                done <<< "$wheel_urls"
                if [[ "$all_dl_ok" -eq 1 ]]; then
                    attempt_ok=1
                fi
            fi
        fi

        if [[ "$attempt_ok" -eq 1 ]]; then
            return 0
        fi

        # Check deadline after attempt
        local elapsed=$(( $(_do_now) - start ))
        if [[ "$elapsed" -ge "$deadline" ]]; then
            echo "FAIL: deadline ${deadline}s exhausted after ${elapsed}s — PyPI wheels for trackfw==$py_version" >&2
            return 1
        fi
        local remaining=$(( deadline - elapsed ))
        local sleep_time
        if [[ "$interval" -lt "$remaining" ]]; then
            sleep_time="$interval"
        else
            sleep_time="$remaining"
        fi
        if [[ "$sleep_time" -le 0 ]]; then
            sleep_time=1
        fi
        elapsed=$(( $(_do_now) - start ))
        echo "  PyPI wheels: retry in ${sleep_time}s... (+${elapsed}s)"
        _do_sleep "$sleep_time"
        interval=$(( interval * 2 ))
        if [[ "$interval" -gt "$max_interval" ]]; then
            interval="$max_interval"
        fi
    done
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
    # Arms 5-9: retry and content arms — see header comment for detail.

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

    # ── Arms 5–9: retry and content arms ─────────────────────────────────────
    # These arms call bash "${BASH_SOURCE[0]}" --published 1.0.0 as a subprocess
    # with injected command stubs, and verify exit code + counter/clock output.
    # All stubs live in $WORK (not under scripts/) to avoid tripping the orphan gate.

    echo ""
    echo "=== Arms 5–9: retry and content behavior (subprocess + stubs) ==="

    # Shared stub: fake-now.py — reads $FAKE_CLOCK_FILE (defaults to real time)
    cat > "$WORK/fake-now.py" << 'PYEOF'
#!/usr/bin/env python3
import os, sys, time
cf = os.environ.get('FAKE_CLOCK_FILE', '')
if not cf:
    print(int(time.time()))
    sys.exit(0)
try:
    with open(cf) as f:
        print(int(f.read().strip()))
except Exception:
    print(0)
PYEOF

    # Shared stub: fake-sleep.py — increments $FAKE_CLOCK_FILE by argv[1]
    cat > "$WORK/fake-sleep.py" << 'PYEOF'
#!/usr/bin/env python3
import os, sys
secs = int(sys.argv[1]) if len(sys.argv) > 1 else 0
cf = os.environ.get('FAKE_CLOCK_FILE', '')
if not cf:
    import time; time.sleep(secs); sys.exit(0)
try:
    with open(cf) as f: curr = int(f.read().strip())
except Exception:
    curr = 0
with open(cf, 'w') as f:
    f.write(str(curr + secs))
PYEOF

    # Shared stub: pass-npm.py — always creates valid npm tarball in argv[2]
    cat > "$WORK/pass-npm.py" << 'PYEOF'
#!/usr/bin/env python3
import sys, tarfile, io, os, json as _json
spec    = sys.argv[1] if len(sys.argv) > 1 else 'trackfw@0.0.0'
destdir = sys.argv[2] if len(sys.argv) > 2 else '/tmp'
ver = spec.split('@')[1] if '@' in spec else '0.0.0'
files = {
    'package/package.json': _json.dumps({'name':'trackfw','version':ver,
        'bin':{'trackfw':'bin/trackfw.js'}}).encode(),
    'package/bin/trackfw.js': b'#!/usr/bin/env node\n"use strict";\n',
    'package/README.md': b'# trackfw\n',
}
fn  = f'trackfw-{ver}.tgz'
out = os.path.join(destdir, fn)
os.makedirs(destdir, exist_ok=True)
with tarfile.open(out, 'w:gz') as t:
    for name, data in files.items():
        info = tarfile.TarInfo(name=name)
        info.size = len(data)
        t.addfile(info, io.BytesIO(data))
PYEOF

    # Shared stub: pass-pypi.py — always returns 8 wheel URLs on first call
    cat > "$WORK/pass-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys
TAGS = ['macosx_10_9_x86_64','macosx_11_0_arm64','manylinux_2_17_aarch64',
        'manylinux_2_17_x86_64','musllinux_1_2_aarch64','musllinux_1_2_x86_64',
        'win_amd64','win_arm64']
data = {'urls': [{'url': f'FAKE/trackfw-ST-py3-none-{t}.whl',
                  'packagetype': 'bdist_wheel'} for t in TAGS]}
print(json.dumps(data))
PYEOF

    # Shared stub: pass-fetch.py — creates a valid wheel at argv[2]
    cat > "$WORK/pass-fetch.py" << 'PYEOF'
#!/usr/bin/env python3
import sys, zipfile, os
dest = sys.argv[2] if len(sys.argv) > 2 else '/tmp/x.whl'
os.makedirs(os.path.dirname(os.path.abspath(dest)) or '.', exist_ok=True)
bn    = os.path.basename(dest).replace('.whl', '')
parts = bn.split('-')
name  = parts[0] if parts else 'trackfw'
ver   = parts[1] if len(parts) > 1 else '0'
with zipfile.ZipFile(dest, 'w') as z:
    z.writestr(f'{name}-{ver}.dist-info/WHEEL',    'Wheel-Version: 1.0\n')
    z.writestr(f'{name}-{ver}.dist-info/METADATA', f'Name: {name}\nVersion: {ver}\n')
    z.writestr(f'{name}-{ver}.data/scripts/trackfw', b'\x7fELF')
PYEOF

    # Arm-specific stub: arm-fail-pypi.py — always fails
    cat > "$WORK/arm-fail-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import sys; sys.exit(1)
PYEOF

    # Arm-specific stub: arm5-pypi.py — fails first 2 calls, returns 8 wheels on 3rd
    cat > "$WORK/arm5-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import sys, json, os
cf = os.environ.get('STUB_COUNTER_FILE', '')
n = 0
if cf:
    try:
        with open(cf) as f: n = int(f.read().strip())
    except Exception: n = 0
    n += 1
    with open(cf, 'w') as f: f.write(str(n))
if n < 3:
    sys.exit(1)
TAGS = ['macosx_10_9_x86_64','macosx_11_0_arm64','manylinux_2_17_aarch64',
        'manylinux_2_17_x86_64','musllinux_1_2_aarch64','musllinux_1_2_x86_64',
        'win_amd64','win_arm64']
data = {'urls': [{'url': f'FAKE/trackfw-ST-py3-none-{t}.whl',
                  'packagetype': 'bdist_wheel'} for t in TAGS]}
print(json.dumps(data))
PYEOF

    # Arm-specific stub: arm7-pypi.py — returns 7 wheels (missing win_arm64)
    cat > "$WORK/arm7-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys
TAGS = ['macosx_10_9_x86_64','macosx_11_0_arm64','manylinux_2_17_aarch64',
        'manylinux_2_17_x86_64','musllinux_1_2_aarch64','musllinux_1_2_x86_64',
        'win_amd64']
# win_arm64 intentionally omitted to test missing-tag detection
data = {'urls': [{'url': f'FAKE/trackfw-ST-py3-none-{t}.whl',
                  'packagetype': 'bdist_wheel'} for t in TAGS]}
print(json.dumps(data))
PYEOF

    # Arm-specific stub: arm8-fetch.py — always creates a wheel with .py injected (bad content)
    cat > "$WORK/arm8-fetch.py" << 'PYEOF'
#!/usr/bin/env python3
import sys, zipfile, os
dest = sys.argv[2] if len(sys.argv) > 2 else '/tmp/x.whl'
os.makedirs(os.path.dirname(os.path.abspath(dest)) or '.', exist_ok=True)
bn    = os.path.basename(dest).replace('.whl', '')
parts = bn.split('-')
name  = parts[0] if parts else 'trackfw'
ver   = parts[1] if len(parts) > 1 else '0'
with zipfile.ZipFile(dest, 'w') as z:
    z.writestr(f'{name}-{ver}.dist-info/WHEEL',    'Wheel-Version: 1.0\n')
    z.writestr(f'{name}-{ver}.dist-info/METADATA', f'Name: {name}\nVersion: {ver}\n')
    z.writestr(f'{name}-{ver}.data/scripts/trackfw', b'\x7fELF')
    # Injected .py file -- proves inspect_wheel catches this without retrying
    z.writestr(f'{name}/__init__.py', b'# injected .py -- arm8 bad-content stub\n')
PYEOF

    # Arm-specific stub: arm9-npm.py — fails first 2 calls, creates valid tarball on 3rd
    cat > "$WORK/arm9-npm.py" << 'PYEOF'
#!/usr/bin/env python3
import sys, tarfile, io, os, json as _json
spec    = sys.argv[1] if len(sys.argv) > 1 else 'trackfw@0.0.0'
destdir = sys.argv[2] if len(sys.argv) > 2 else '/tmp'
cf = os.environ.get('STUB_COUNTER_FILE', '')
n = 0
if cf:
    try:
        with open(cf) as f: n = int(f.read().strip())
    except Exception: n = 0
    n += 1
    with open(cf, 'w') as f: f.write(str(n))
if n < 3:
    sys.exit(1)
ver = spec.split('@')[1] if '@' in spec else '0.0.0'
files = {
    'package/package.json': _json.dumps({'name':'trackfw','version':ver,
        'bin':{'trackfw':'bin/trackfw.js'}}).encode(),
    'package/bin/trackfw.js': b'#!/usr/bin/env node\n"use strict";\n',
    'package/README.md': b'# trackfw\n',
}
fn  = f'trackfw-{ver}.tgz'
out = os.path.join(destdir, fn)
os.makedirs(destdir, exist_ok=True)
with tarfile.open(out, 'w:gz') as t:
    for name, data in files.items():
        info = tarfile.TarInfo(name=name)
        info.size = len(data)
        t.addfile(info, io.BytesIO(data))
PYEOF

    # ── Arm 5: PyPI retry positivo ─────────────────────────────────────────
    # Asserts: retry loop eventually fetches all 8 wheels and exits 0 when
    # PYPI_JSON_CMD becomes available on the 3rd attempt.
    echo ""
    echo "--- Arm 5: PyPI retry positivo (PYPI_JSON_CMD fails 2×, passes 3rd) — expect exit 0 ---"
    COUNTER5="$WORK/arm5-counter"
    CLOCK5="$WORK/arm5-clock"
    printf '' > "$COUNTER5"
    printf '0\n' > "$CLOCK5"
    set +e
    arm5_out=$(
        STUB_COUNTER_FILE="$COUNTER5" \
        FAKE_CLOCK_FILE="$CLOCK5" \
        PYPI_JSON_CMD="python3 $WORK/arm5-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --published 1.0.0 2>&1
    )
    arm5_exit=$?
    set -e
    arm5_calls=$(cat "$COUNTER5" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm5_exit" -eq 0 ]] && [[ "${arm5_calls:-0}" -ge 3 ]]; then
        ok "arm5: PyPI retry positivo — exit 0, PYPI_JSON_CMD called ${arm5_calls} times (≥3)"
    elif [[ "$arm5_exit" -eq 0 ]] && [[ "${arm5_calls:-0}" -lt 3 ]]; then
        fail "arm5: PyPI retry positivo — exit 0 but only ${arm5_calls} calls to PYPI_JSON_CMD (expected ≥3)"
    else
        fail "arm5: PyPI retry positivo — expected exit 0, got $arm5_exit"
        printf '%s\n' "$arm5_out" | head -8 | sed 's/^/  /'
    fi

    # ── Arm 6: PyPI retry negativo (deadline exhausted) ───────────────────
    # Asserts: retry loop exits 1 (never silent pass) when deadline exhausted.
    echo ""
    echo "--- Arm 6: PyPI retry negativo (PYPI_JSON_CMD always fails, deadline=1) — expect exit 1 ---"
    CLOCK6="$WORK/arm6-clock"
    printf '0\n' > "$CLOCK6"
    set +e
    arm6_out=$(
        FAKE_CLOCK_FILE="$CLOCK6" \
        PYPI_JSON_CMD="python3 $WORK/arm-fail-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=1 \
        bash "${BASH_SOURCE[0]}" --published 1.0.0 2>&1
    )
    arm6_exit=$?
    set -e
    if [[ "$arm6_exit" -ne 0 ]] && printf '%s' "$arm6_out" | grep -qi "deadline"; then
        ok "arm6: PyPI retry negativo — exit 1 with 'deadline' message"
    elif [[ "$arm6_exit" -ne 0 ]]; then
        fail "arm6: PyPI retry negativo — exit 1 but no 'deadline' in output (wrong failure path?)"
        printf '%s\n' "$arm6_out" | head -5 | sed 's/^/  /'
    else
        fail "arm6: PyPI retry negativo — expected exit 1, got $arm6_exit (deadline not enforced)"
    fi

    # ── Arm 7: 7 wheels → FAIL with missing tag named ────────────────────
    # Asserts: a missing wheel tag (7 instead of 8) is reported as FAIL by name.
    echo ""
    echo "--- Arm 7: 7 wheels (missing win_arm64) — expect exit 1 with tag named ---"
    CLOCK7="$WORK/arm7-clock"
    printf '0\n' > "$CLOCK7"
    set +e
    arm7_out=$(
        FAKE_CLOCK_FILE="$CLOCK7" \
        PYPI_JSON_CMD="python3 $WORK/arm7-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --published 1.0.0 2>&1
    )
    arm7_exit=$?
    set -e
    if [[ "$arm7_exit" -ne 0 ]] && printf '%s' "$arm7_out" | grep -q "win_arm64"; then
        ok "arm7: 7 wheels → exit 1, missing tag 'win_arm64' named"
    elif [[ "$arm7_exit" -ne 0 ]]; then
        fail "arm7: 7 wheels → exit 1 but missing tag not named in output"
        printf '%s\n' "$arm7_out" | grep -i "FAIL\|miss\|tag" | head -5 | sed 's/^/  /'
    else
        fail "arm7: 7 wheels → expected exit 1, got $arm7_exit (missing-tag guard not firing)"
    fi

    # ── Arm 8: bad wheel content → FAIL immediately, no retry ────────────
    # Asserts: a wheel with wrong content fails immediately without retry
    # (VERIFY_CONTENT_SLEEP_CMD never called — fake clock stays at 0).
    echo ""
    echo "--- Arm 8: bad wheel content (.py injected) — expect exit 1, sleep never called ---"
    CLOCK8="$WORK/arm8-clock"
    printf '0\n' > "$CLOCK8"
    set +e
    arm8_out=$(
        FAKE_CLOCK_FILE="$CLOCK8" \
        PYPI_JSON_CMD="python3 $WORK/pass-pypi.py" \
        FETCH_CMD="python3 $WORK/arm8-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --published 1.0.0 2>&1
    )
    arm8_exit=$?
    set -e
    arm8_clock=$(cat "$CLOCK8" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm8_exit" -ne 0 ]] && [[ "${arm8_clock:-0}" -eq 0 ]]; then
        ok "arm8: bad wheel content → exit 1 without retry (sleep never called, clock=0)"
    elif [[ "$arm8_exit" -ne 0 ]] && [[ "${arm8_clock:-0}" -ne 0 ]]; then
        fail "arm8: bad wheel content → exit 1 but sleep was called (clock=${arm8_clock}) — content check appears to be inside the retry loop"
    else
        fail "arm8: bad wheel content → expected exit 1, got $arm8_exit (inspect_wheel not catching .py)"
        printf '%s\n' "$arm8_out" | head -5 | sed 's/^/  /'
    fi

    # ── Arm 9: npm retry positivo ─────────────────────────────────────────
    # Asserts: npm pack retry loop exits 0 and inspects a valid tarball when
    # NPM_PACK_CMD becomes available on the 3rd attempt.
    echo ""
    echo "--- Arm 9: npm retry positivo (NPM_PACK_CMD fails 2×, passes 3rd) — expect exit 0 ---"
    COUNTER9="$WORK/arm9-counter"
    CLOCK9="$WORK/arm9-clock"
    printf '' > "$COUNTER9"
    printf '0\n' > "$CLOCK9"
    set +e
    arm9_out=$(
        STUB_COUNTER_FILE="$COUNTER9" \
        FAKE_CLOCK_FILE="$CLOCK9" \
        PYPI_JSON_CMD="python3 $WORK/pass-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/arm9-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --published 1.0.0 2>&1
    )
    arm9_exit=$?
    set -e
    arm9_calls=$(cat "$COUNTER9" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm9_exit" -eq 0 ]] && [[ "${arm9_calls:-0}" -ge 3 ]]; then
        ok "arm9: npm retry positivo — exit 0, NPM_PACK_CMD called ${arm9_calls} times (≥3)"
    elif [[ "$arm9_exit" -eq 0 ]] && [[ "${arm9_calls:-0}" -lt 3 ]]; then
        fail "arm9: npm retry positivo — exit 0 but only ${arm9_calls} calls to NPM_PACK_CMD (expected ≥3)"
    else
        fail "arm9: npm retry positivo — expected exit 0, got $arm9_exit"
        printf '%s\n' "$arm9_out" | head -8 | sed 's/^/  /'
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

    # ── npm shim (download pack from registry, with retry) ────────────────
    echo ""
    echo "--- npm shim trackfw@$VERSION ---"
    NPM_WORK="$WORK/npm"
    mkdir -p "$NPM_WORK"
    if _retry_npm_pack "trackfw@$VERSION" "$NPM_WORK"; then
        SHIM_TGZ=$(ls "$NPM_WORK"/trackfw-*.tgz 2>/dev/null | tail -1)
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
        fail "npm pack trackfw@$VERSION unavailable after ${VERIFY_CONTENT_DEADLINE}s deadline"
    fi

    # ── PyPI: fetch wheel list via JSON API (with retry) and inspect ──────
    # Fetches from /pypi/trackfw/<ver>/json — NOT the Simple Index /simple/
    # which is CDN-cached with max-age=600 and caused failures in v9.3.3/v9.4.1.
    echo ""
    echo "--- PyPI wheels trackfw==$VERSION ---"
    # Normalize version for PyPI (8.0.0-rc1 → 8.0.0rc1)
    PY_VERSION=$(python3 -c "
try:
    from packaging.version import Version
    print(str(Version('$VERSION')))
except Exception:
    print('$VERSION')
" 2>/dev/null | strip_cr || echo "$VERSION")

    WHEEL_WORK="$WORK/wheels"
    mkdir -p "$WHEEL_WORK"
    if _fetch_pypi_wheels_with_retry "$PY_VERSION" "$WHEEL_WORK"; then
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
            fail "vacuity guard: no wheels downloaded for trackfw==$PY_VERSION"
        else
            # Check that all expected platform tags are present (not just count).
            # A missing tag means one platform was not published or not yet propagated.
            if python3 - "$WHEEL_WORK" "$EXPECTED_WHEEL_TAGS" <<'PYEOF'
import sys, os
wheel_dir   = sys.argv[1]
# comma-separated expected platform tags
expected    = set(t for t in sys.argv[2].split(',') if t)
actual      = set()
for fn in os.listdir(wheel_dir):
    if fn.endswith('.whl'):
        parts = fn.replace('.whl', '').split('-')
        # wheel filename: name-ver-pytag-abitag-platformtag.whl (>=5 parts)
        if len(parts) >= 5:
            actual.add(parts[-1])
missing = expected - actual
extra   = actual   - expected
exit_code = 0
if missing:
    for t in sorted(missing):
        print(f"FAIL: missing expected wheel tag: {t}", file=sys.stderr)
    exit_code = 1
if extra:
    for t in sorted(extra):
        print(f"warn: unexpected wheel tag (not in expected set): {t}", file=sys.stderr)
if not missing:
    print(f"ok: all {len(expected)} expected wheel tags present: {', '.join(sorted(actual))}")
sys.exit(exit_code)
PYEOF
            then
                ok "wheel count: $WHEEL_COUNT of $EXPECTED_WHEEL_COUNT expected wheels present"
            else
                fail "wheel count: $WHEEL_COUNT wheels downloaded but expected tags missing — see above"
            fi
        fi
    else
        fail "PyPI wheels for trackfw==$PY_VERSION unavailable after ${VERIFY_CONTENT_DEADLINE}s deadline"
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
