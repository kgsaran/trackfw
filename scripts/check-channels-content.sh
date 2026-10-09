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
#     Arm 7: PyPI returns 7 wheels (missing win_arm64) — retries until deadline → exit 1, tag named
#     Arm 8: PyPI wheels have wrong content (.py) — sha256 matches, inspect_wheel catches it → exit 1, SLEEP never called
#     Arm 9: npm retry positivo — npm pack fails 2×, succeeds 3rd → exit 0
#    Arm 10: PyPI JSON returns wrong sha256 — mismatch detected immediately → exit 1, SLEEP never called
#    Arm 11: PyPI JSON returns URL from wrong host → FAIL immediately, no retry
#    Arm 12: PyPI JSON returns 4 of 8 wheels on first 2 calls, 8 on 3rd → exit 0 (A2 retry)
#    Arm 13: PyPI JSON returns 9 wheels (extra tag linux_riscv64) → exit 1, extra tag named (A4)
#    Arm 14: injectable vars set in --published mode → warned and ignored; injected stubs NOT called
#
# Reconciliation for new arms (Regra Dura — CLAUDE.md):
#   Arm 5 asserts: the retry loop eventually fetches all 8 wheels and exits 0 when
#     PYPI_JSON_CMD becomes available on the 3rd attempt.
#   Arm 6 asserts: the retry loop exits 1 (never silent pass) when the deadline is
#     exhausted before PYPI_JSON_CMD responds.
#   Arm 7 asserts: a partial wheel list (7 of 8) causes the retry loop to run until
#     deadline, then reports missing tag win_arm64 by name.
#   Arm 8 asserts: a wheel with .py content is caught by inspect_wheel after download
#     (sha256 matches manifest); the retry loop is never triggered
#     (VERIFY_CONTENT_SLEEP_CMD is never called).
#   Arm 9 asserts: the npm pack retry loop exits 0 and inspects a valid tarball when
#     NPM_PACK_CMD becomes available on the 3rd attempt.
#   Arm 10 asserts: sha256 mismatch between JSON API digest and downloaded file causes
#     immediate FAIL (return 2) without triggering the retry loop.
#   Arm 11 asserts: a wheel URL from a non-files.pythonhosted.org/ host (including
#     lookalike subdomain) causes immediate FAIL (return 2) without retry.
#   Arm 12 asserts: when JSON API returns fewer than EXPECTED_WHEEL_COUNT wheels, the
#     function retries instead of failing immediately; exits 0 when full list arrives.
#   Arm 13 asserts: a wheel tag not in EXPECTED_WHEEL_TAGS causes FAIL with the
#     unexpected tag named (not a silent pass).
#   Arm 14 asserts: in --published mode, injectable command overrides are warned about
#     and unset; the injected stubs are never called and real system commands are used.
#
# Vacuity guards: zero files inspected in any category → named fail, never silent pass.
#
# Environment distinctions (--local):
#   Requires: node (npm pack), python3 (zipfile)
#   Skip if node not in PATH — named skip, not fail (environment, not defect).
#
# Injectable commands (for --self-test only — ignored in --published with WARN):
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
# These can be overridden in --self-test mode only.
# In --published mode they are WARNED about and unset (A3 fix).

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
# In real mode: --proto =https prevents non-https; -L --max-redirs 0 rejects any
# redirect (exits 47) so a CDN redirect cannot silently replace the expected file.
# sha256 is verified by the caller after return.
_do_fetch() {
    local url="$1"
    local dest="$2"
    if [[ -n "${FETCH_CMD:-}" ]]; then
        $FETCH_CMD "$url" "$dest"
    else
        curl -fSL --proto =https --max-redirs 0 -o "$dest" "$url"
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
# Uses the PyPI JSON API to get wheel URLs and downloads each directly from
# files.pythonhosted.org — bypasses the CDN-cached Simple Index.
# Retries the entire cycle (JSON fetch + all downloads) on any failure.
# Content inspection is NOT done here — caller inspects after return 0.
#
# Returns:
#   0 — all wheels fetched and sha256-verified
#   1 — deadline exhausted (temporary — propagation still in progress)
#   2 — security failure: disallowed URL host or sha256 mismatch (permanent, no retry)
#
# Security checks per-URL (return 2 immediately on any violation):
#   - Host must be exactly https://files.pythonhosted.org/ (with trailing slash)
#   - sha256 must be present in JSON and be exactly 64 lowercase hex characters
#   - sha256 of downloaded file must equal the JSON-supplied digest
#
# A2 fix: if JSON returns fewer URLs than EXPECTED_WHEEL_COUNT, treats the
# attempt as "still propagating" — retries instead of failing immediately.
_fetch_pypi_wheels_with_retry() {
    local py_version="$1"
    local dest="$2"
    local deadline="$VERIFY_CONTENT_DEADLINE"
    local interval=10
    local max_interval=60
    local start
    start=$(_do_now)
    local last_valid_entries=""

    echo "  PyPI wheels for trackfw==$py_version via JSON API (deadline: ${deadline}s)"
    echo "  (JSON API bypasses CDN Simple Index; files from files.pythonhosted.org)"

    while true; do
        # ── Step 0: fetch JSON ────────────────────────────────────────────────
        local json_out
        json_out=$(_do_pypi_json "$py_version" 2>/dev/null) || json_out=""

        local wheel_entries=""
        if [[ -n "$json_out" ]]; then
            wheel_entries=$(printf '%s' "$json_out" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for u in data.get('urls', []):
        if u.get('packagetype') == 'bdist_wheel':
            url = u.get('url', '')
            sha = u.get('digests', {}).get('sha256', '')
            if url:
                print(url + '|||' + sha)
except Exception:
    sys.exit(1)
" 2>/dev/null | strip_cr) || wheel_entries=""
        fi

        # ── Step 1: validate each entry — host and sha256 ────────────────────
        # Any violation → return 2 immediately (no retry).
        local security_fail=0
        local valid_count=0
        local valid_entries_buf=""

        if [[ -n "$wheel_entries" ]]; then
            while IFS= read -r _wline; do
                [[ -n "$_wline" ]] || continue
                local _wurl="${_wline%|||*}"
                local _wsha="${_wline##*|||}"

                # Host must be exactly https://files.pythonhosted.org/ (with slash).
                # The trailing slash prevents the lookalike https://files.pythonhosted.org.evil.example/
                case "$_wurl" in
                    https://files.pythonhosted.org/*) ;;
                    *)
                        echo "FAIL: wheel URL from disallowed host: ${_wurl}" >&2
                        echo "  expected prefix: https://files.pythonhosted.org/" >&2
                        security_fail=1
                        break
                        ;;
                esac

                # sha256 must be present and exactly 64 lowercase hex characters.
                if ! printf '%s' "$_wsha" | grep -qE '^[0-9a-f]{64}$'; then
                    echo "FAIL: missing or invalid sha256 for $(basename "$_wurl") (got: '${_wsha:-empty}')" >&2
                    security_fail=1
                    break
                fi

                valid_count=$((valid_count + 1))
                valid_entries_buf="${valid_entries_buf}${_wline}
"
            done <<< "$wheel_entries"
        fi
        unset _wline _wurl _wsha 2>/dev/null || true

        if [[ "$security_fail" -eq 1 ]]; then
            return 2
        fi

        # ── Step 2: count check — retry if fewer wheels than expected ─────────
        # Covers the case where the JSON API reflects a partially-uploaded release
        # (twine uploads 8 wheels in sequence; all must be indexed before proceeding).
        if [[ "$valid_count" -lt "$EXPECTED_WHEEL_COUNT" ]]; then
            if [[ -n "$valid_entries_buf" ]]; then
                last_valid_entries="$valid_entries_buf"
            fi
            # Not enough wheels yet — fall through to retry/deadline check.
        else
            # ── Step 3: download each wheel and verify sha256 ─────────────────
            # Clear dest dir first (avoid stale wheels from an aborted attempt).
            rm -f "$dest"/*.whl 2>/dev/null || true

            local download_ok=1
            while IFS= read -r _wline; do
                [[ -n "$_wline" ]] || continue
                local _wurl="${_wline%|||*}"
                local _wsha="${_wline##*|||}"
                local _wfn
                _wfn=$(basename "$_wurl")

                if ! _do_fetch "$_wurl" "$dest/$_wfn" 2>/dev/null; then
                    download_ok=0
                    break
                fi

                # Verify sha256 of downloaded file against JSON API digest.
                local _actual_sha
                _actual_sha=$(python3 -c "
import sys, hashlib
with open(sys.argv[1],'rb') as f: d=f.read()
print(hashlib.sha256(d).hexdigest())
" "$dest/$_wfn" 2>/dev/null | strip_cr) || _actual_sha=""

                if [[ "$_actual_sha" != "$_wsha" ]]; then
                    echo "FAIL: sha256 mismatch for $_wfn" >&2
                    echo "  expected (JSON API): $_wsha" >&2
                    echo "  actual   (download): ${_actual_sha:-error computing hash}" >&2
                    unset _wline _wurl _wsha _wfn _actual_sha 2>/dev/null || true
                    return 2
                fi
            done <<< "$valid_entries_buf"
            unset _wline _wurl _wsha _wfn _actual_sha 2>/dev/null || true

            if [[ "$download_ok" -eq 1 ]]; then
                return 0
            fi
        fi

        # ── Retry / deadline logic ─────────────────────────────────────────────
        local elapsed=$(( $(_do_now) - start ))
        if [[ "$elapsed" -ge "$deadline" ]]; then
            # Report which tags are missing (from the last known partial list if any)
            local _tags_src="${last_valid_entries:-}"
            if [[ -n "$_tags_src" ]]; then
                printf '%s' "$_tags_src" | python3 -c "
import sys, os
lines = [l for l in sys.stdin.read().strip().split('\n') if l]
actual = set()
for l in lines:
    url = l.split('|||')[0]
    bn = os.path.basename(url)
    parts = bn.replace('.whl','').split('-')
    if len(parts) >= 5:
        actual.add(parts[-1])
expected = set(t for t in sys.argv[1].split(',') if t)
for t in sorted(expected - actual):
    print(f'  missing wheel tag: {t}', file=sys.stderr)
" "$EXPECTED_WHEEL_TAGS" 2>&1 | cat >&2 || true
            fi
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
    # Arms 5–9 and 12–14 call --self-test-published (internal argv-only mode that
    # bypasses the A3 injectable guard). --published unconditionally warns and unsets
    # any injectable vars; --self-test-published allows them.

    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-channels-content-selftest.XXXXXX")
    CLEANUP_WORK="$WORK"
    cleanup_selftest() { rm -rf "$CLEANUP_WORK" 2>/dev/null || true; }
    trap cleanup_selftest EXIT

    # ── Fixture setup: deterministic wheel files + sha256 manifests ──────────
    # Creates:
    #   $WORK/fixture-good/   — 9 valid wheels (8 expected + linux_riscv64 for arm 13)
    #   $WORK/fixture-bad/    — 8 wheels with injected .py (for arm 8)
    #   $WORK/manifest-good-8.json  — sha256 of the 8 expected good wheels
    #   $WORK/manifest-good-9.json  — sha256 of all 9 good wheels (incl linux_riscv64)
    #   $WORK/manifest-bad-8.json   — sha256 of the 8 bad wheels
    python3 - "$WORK/fixture-good" "$WORK/fixture-bad" \
               "$WORK/manifest-good-8.json" "$WORK/manifest-good-9.json" \
               "$WORK/manifest-bad-8.json" <<'PYEOF'
import sys, os, zipfile, hashlib, json

GOOD_TAGS = ['macosx_10_9_x86_64','macosx_11_0_arm64','manylinux_2_17_aarch64',
             'manylinux_2_17_x86_64','musllinux_1_2_aarch64','musllinux_1_2_x86_64',
             'win_amd64','win_arm64']
EXTRA_TAG = 'linux_riscv64'  # used by arm 13

good_dir, bad_dir, mf_g8, mf_g9, mf_b8 = sys.argv[1:6]
os.makedirs(good_dir, exist_ok=True)
os.makedirs(bad_dir,  exist_ok=True)

EPOCH = (1980, 1, 1, 0, 0, 0)
VER = '1.0.0'
NAME = 'trackfw'

def write_wheel(path, tag, inject_py=False):
    entries = [
        (f'{NAME}-{VER}.dist-info/WHEEL',    b'Wheel-Version: 1.0\n'),
        (f'{NAME}-{VER}.dist-info/METADATA', f'Name: {NAME}\nVersion: {VER}\nPlatform: {tag}\n'.encode()),
        (f'{NAME}-{VER}.data/scripts/{NAME}', b'\x7fELF\x00' + tag.encode()),
    ]
    if inject_py:
        entries.append((f'{NAME}/__init__.py', b'# injected .py - arm8 bad-content\n'))
    with zipfile.ZipFile(path, 'w', compression=zipfile.ZIP_STORED) as z:
        for fname, data in entries:
            zi = zipfile.ZipInfo(fname, date_time=EPOCH)
            z.writestr(zi, data)

def sha256f(path):
    with open(path, 'rb') as f:
        return hashlib.sha256(f.read()).hexdigest()

g8 = {}
g9 = {}
b8 = {}

for tag in GOOD_TAGS + [EXTRA_TAG]:
    fn   = f'{NAME}-{VER}-py3-none-{tag}.whl'
    path = os.path.join(good_dir, fn)
    write_wheel(path, tag)
    h = sha256f(path)
    g9[fn] = h
    if tag != EXTRA_TAG:
        g8[fn] = h

for tag in GOOD_TAGS:
    fn   = f'{NAME}-{VER}-py3-none-{tag}.whl'
    path = os.path.join(bad_dir, fn)
    write_wheel(path, tag, inject_py=True)
    b8[fn] = sha256f(path)

for mf, data in [(mf_g8, g8), (mf_g9, g9), (mf_b8, b8)]:
    with open(mf, 'w') as f:
        json.dump(data, f)
PYEOF

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

    # ── Arms 5–14: retry, security, and content behavior ─────────────────────
    # These arms call bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 as a subprocess
    # with injected command stubs, and verify exit code + output.
    # --self-test-published bypasses the A3 injectable guard (argv-only, not env-settable).

    echo ""
    echo "=== Arms 5–14: retry, security, and content behavior (subprocess + stubs) ==="

    # ── Shared stubs ──────────────────────────────────────────────────────────

    # fake-now.py — reads $FAKE_CLOCK_FILE (defaults to real time)
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

    # fake-sleep.py — increments $FAKE_CLOCK_FILE by argv[1]
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

    # pass-npm.py — always creates valid npm tarball in argv[2]
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

    # pass-pypi.py — reads WHEEL_MANIFEST_FILE; returns all entries as JSON API response
    cat > "$WORK/pass-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys, os
manifest_file = os.environ.get('WHEEL_MANIFEST_FILE', '')
base_url = 'https://files.pythonhosted.org/packages/ST/'
if not manifest_file:
    sys.exit(1)
with open(manifest_file) as f:
    manifest = json.load(f)
urls = [{'url': base_url + fn, 'packagetype': 'bdist_wheel',
         'digests': {'sha256': sha256}}
        for fn, sha256 in manifest.items()]
print(json.dumps({'urls': urls}))
PYEOF

    # pass-fetch.py — copies fixture wheel from FIXTURE_WHEEL_DIR by URL basename
    cat > "$WORK/pass-fetch.py" << 'PYEOF'
#!/usr/bin/env python3
import sys, os, shutil
url      = sys.argv[1] if len(sys.argv) > 1 else ''
dest     = sys.argv[2] if len(sys.argv) > 2 else '/tmp/x.whl'
fix_dir  = os.environ.get('FIXTURE_WHEEL_DIR', '')
basename = os.path.basename(url)
os.makedirs(os.path.dirname(os.path.abspath(dest)) or '.', exist_ok=True)
src = os.path.join(fix_dir, basename) if fix_dir else ''
if src and os.path.exists(src):
    shutil.copy2(src, dest)
else:
    # fallback: minimal deterministic wheel (for arms that never reach download)
    import zipfile
    EPOCH = (1980, 1, 1, 0, 0, 0)
    fn   = os.path.basename(dest).replace('.whl', '')
    parts = fn.split('-')
    name = parts[0] if parts else 'trackfw'
    ver  = parts[1] if len(parts) > 1 else '0'
    with zipfile.ZipFile(dest, 'w', compression=zipfile.ZIP_STORED) as z:
        for e_name, data in [
            (f'{name}-{ver}.dist-info/WHEEL',    b'Wheel-Version: 1.0\n'),
            (f'{name}-{ver}.dist-info/METADATA', f'Name: {name}\nVersion: {ver}\n'.encode()),
            (f'{name}-{ver}.data/scripts/{name}', b'\x7fELF'),
        ]:
            zi = zipfile.ZipInfo(e_name, date_time=EPOCH)
            z.writestr(zi, data)
PYEOF

    # arm-fail-pypi.py — always fails
    cat > "$WORK/arm-fail-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import sys; sys.exit(1)
PYEOF

    # arm5-pypi.py — fails first 2 calls, then returns full list from manifest
    cat > "$WORK/arm5-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys, os
cf = os.environ.get('STUB_COUNTER_FILE', '')
manifest_file = os.environ.get('WHEEL_MANIFEST_FILE', '')
base_url = 'https://files.pythonhosted.org/packages/ST/'
n = 0
if cf:
    try:
        with open(cf) as f: n = int(f.read().strip())
    except Exception: n = 0
    n += 1
    with open(cf, 'w') as f: f.write(str(n))
if n < 3:
    sys.exit(1)
with open(manifest_file) as f:
    manifest = json.load(f)
urls = [{'url': base_url + fn, 'packagetype': 'bdist_wheel',
         'digests': {'sha256': sha256}}
        for fn, sha256 in manifest.items()]
print(json.dumps({'urls': urls}))
PYEOF

    # arm7-pypi.py — returns manifest minus win_arm64 (7 wheels, always)
    cat > "$WORK/arm7-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys, os
manifest_file = os.environ.get('WHEEL_MANIFEST_FILE', '')
base_url = 'https://files.pythonhosted.org/packages/ST/'
with open(manifest_file) as f:
    manifest = json.load(f)
urls = [{'url': base_url + fn, 'packagetype': 'bdist_wheel',
         'digests': {'sha256': sha256}}
        for fn, sha256 in manifest.items()
        if 'win_arm64' not in fn]
print(json.dumps({'urls': urls}))
PYEOF

    # arm9-npm.py — fails first 2 calls, creates valid tarball on 3rd
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

    # arm10-pypi.py — correct URLs from manifest but with WRONG sha256 (all 'a')
    cat > "$WORK/arm10-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys, os
manifest_file = os.environ.get('WHEEL_MANIFEST_FILE', '')
base_url = 'https://files.pythonhosted.org/packages/ST/'
WRONG_SHA = 'a' * 64
with open(manifest_file) as f:
    manifest = json.load(f)
urls = [{'url': base_url + fn, 'packagetype': 'bdist_wheel',
         'digests': {'sha256': WRONG_SHA}}
        for fn in manifest]
print(json.dumps({'urls': urls}))
PYEOF

    # arm11-pypi.py — returns URL from a wrong host (lookalike subdomain)
    cat > "$WORK/arm11-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys, os
manifest_file = os.environ.get('WHEEL_MANIFEST_FILE', '')
# Lookalike: has files.pythonhosted.org as a subdomain prefix, tests that the
# check requires a *trailing slash* after the exact hostname.
BAD_HOST = 'https://files.pythonhosted.org.evil.example/packages/ST/'
with open(manifest_file) as f:
    manifest = json.load(f)
first_fn = list(manifest.keys())[0]
sha256   = list(manifest.values())[0]
urls = [{'url': BAD_HOST + first_fn, 'packagetype': 'bdist_wheel',
         'digests': {'sha256': sha256}}]
print(json.dumps({'urls': urls}))
PYEOF

    # arm12-pypi.py — returns 4 wheels on first 2 calls, all 8 on 3rd+
    cat > "$WORK/arm12-pypi.py" << 'PYEOF'
#!/usr/bin/env python3
import json, sys, os
cf = os.environ.get('STUB_COUNTER_FILE', '')
manifest_file = os.environ.get('WHEEL_MANIFEST_FILE', '')
base_url = 'https://files.pythonhosted.org/packages/ST/'
n = 0
if cf:
    try:
        with open(cf) as f: n = int(f.read().strip())
    except Exception: n = 0
    n += 1
    with open(cf, 'w') as f: f.write(str(n))
with open(manifest_file) as f:
    manifest = json.load(f)
entries = list(manifest.items())
if n < 3:
    entries = entries[:4]   # partial list — only 4 of 8
urls = [{'url': base_url + fn, 'packagetype': 'bdist_wheel',
         'digests': {'sha256': sha256}}
        for fn, sha256 in entries]
print(json.dumps({'urls': urls}))
PYEOF

    # arm14-stub.py — recorder stub for all 5 injectable vars; exits 1 if called
    cat > "$WORK/arm14-stub.py" << 'PYEOF'
#!/usr/bin/env python3
import os, sys
rec = os.environ.get('ARM14_STUB_REC', '')
if rec:
    with open(rec, 'a') as f:
        f.write(f'stub called: {" ".join(sys.argv)}\n')
sys.exit(1)
PYEOF

    # fake curl for arm14 — records invocation, exits 1 (no real network)
    cat > "$WORK/curl" << 'STUBEOF'
#!/bin/sh
printf 'curl_call\n' >> "${ARM14_CURL_REC:-/dev/null}"
exit 1
STUBEOF
    chmod +x "$WORK/curl"

    # fake npm for arm14
    cat > "$WORK/npm" << 'STUBEOF'
#!/bin/sh
printf 'npm_call\n' >> "${ARM14_CURL_REC:-/dev/null}"
exit 1
STUBEOF
    chmod +x "$WORK/npm"

    # ── Arm 5: PyPI retry positivo ─────────────────────────────────────────
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
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-8.json" \
        FIXTURE_WHEEL_DIR="$WORK/fixture-good" \
        PYPI_JSON_CMD="python3 $WORK/arm5-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
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
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
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

    # ── Arm 7: 7 wheels → retry until deadline, then FAIL with tag named ──
    echo ""
    echo "--- Arm 7: 7 wheels (missing win_arm64) — expect exit 1 with tag named ---"
    CLOCK7="$WORK/arm7-clock"
    printf '0\n' > "$CLOCK7"
    set +e
    arm7_out=$(
        FAKE_CLOCK_FILE="$CLOCK7" \
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-8.json" \
        PYPI_JSON_CMD="python3 $WORK/arm7-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
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

    # ── Arm 8: bad wheel content (.py) → FAIL without retry ──────────────
    # Uses bad manifest so sha256 matches the bad wheel; inspect_wheel catches .py.
    echo ""
    echo "--- Arm 8: bad wheel content (.py, sha256 matches bad manifest) — expect exit 1, sleep never called ---"
    CLOCK8="$WORK/arm8-clock"
    printf '0\n' > "$CLOCK8"
    set +e
    arm8_out=$(
        FAKE_CLOCK_FILE="$CLOCK8" \
        WHEEL_MANIFEST_FILE="$WORK/manifest-bad-8.json" \
        FIXTURE_WHEEL_DIR="$WORK/fixture-bad" \
        PYPI_JSON_CMD="python3 $WORK/pass-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
    )
    arm8_exit=$?
    set -e
    arm8_clock=$(cat "$CLOCK8" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm8_exit" -ne 0 ]] && [[ "${arm8_clock:-0}" -eq 0 ]]; then
        ok "arm8: bad wheel content (.py) → exit 1 without retry (sleep never called, clock=0)"
    elif [[ "$arm8_exit" -ne 0 ]] && [[ "${arm8_clock:-0}" -ne 0 ]]; then
        fail "arm8: bad wheel content → exit 1 but sleep was called (clock=${arm8_clock}) — content check appears to be inside the retry loop"
    else
        fail "arm8: bad wheel content → expected exit 1, got $arm8_exit (inspect_wheel not catching .py)"
        printf '%s\n' "$arm8_out" | head -5 | sed 's/^/  /'
    fi

    # ── Arm 9: npm retry positivo ─────────────────────────────────────────
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
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-8.json" \
        FIXTURE_WHEEL_DIR="$WORK/fixture-good" \
        PYPI_JSON_CMD="python3 $WORK/pass-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/arm9-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
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

    # ── Arm 10: sha256 mismatch → FAIL immediately, no retry ─────────────
    # arm10-pypi.py returns correct URLs but wrong sha256 (all 'a').
    # sha256 of downloaded file ≠ 'aaa…' → return 2 → FAIL, sleep never called.
    echo ""
    echo "--- Arm 10: sha256 mismatch (wrong digest in JSON) — expect exit 1, sleep never called ---"
    CLOCK10="$WORK/arm10-clock"
    printf '0\n' > "$CLOCK10"
    set +e
    arm10_out=$(
        FAKE_CLOCK_FILE="$CLOCK10" \
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-8.json" \
        FIXTURE_WHEEL_DIR="$WORK/fixture-good" \
        PYPI_JSON_CMD="python3 $WORK/arm10-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
    )
    arm10_exit=$?
    set -e
    arm10_clock=$(cat "$CLOCK10" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm10_exit" -ne 0 ]] && [[ "${arm10_clock:-0}" -eq 0 ]] \
       && printf '%s' "$arm10_out" | grep -qi "sha256"; then
        ok "arm10: sha256 mismatch → exit 1, 'sha256' in output, sleep never called (clock=0)"
    elif [[ "$arm10_exit" -ne 0 ]] && [[ "${arm10_clock:-0}" -eq 0 ]]; then
        fail "arm10: sha256 mismatch → exit 1, no sleep, but 'sha256' not found in output"
        printf '%s\n' "$arm10_out" | head -5 | sed 's/^/  /'
    elif [[ "$arm10_exit" -ne 0 ]]; then
        fail "arm10: sha256 mismatch → exit 1 but sleep was called (clock=${arm10_clock}) — mismatch did not cause immediate fail"
    else
        fail "arm10: sha256 mismatch → expected exit 1, got $arm10_exit (sha256 check not enforced)"
        printf '%s\n' "$arm10_out" | head -5 | sed 's/^/  /'
    fi

    # ── Arm 11: URL from wrong host → FAIL immediately ─────────────────
    # arm11-pypi.py returns a URL with lookalike host (subdomain of pythonhosted.org.evil.example).
    # Host check catches it before download → return 2 → FAIL, sleep never called.
    echo ""
    echo "--- Arm 11: URL from wrong host (lookalike subdomain) — expect exit 1, sleep never called ---"
    CLOCK11="$WORK/arm11-clock"
    printf '0\n' > "$CLOCK11"
    set +e
    arm11_out=$(
        FAKE_CLOCK_FILE="$CLOCK11" \
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-8.json" \
        PYPI_JSON_CMD="python3 $WORK/arm11-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
    )
    arm11_exit=$?
    set -e
    arm11_clock=$(cat "$CLOCK11" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm11_exit" -ne 0 ]] && [[ "${arm11_clock:-0}" -eq 0 ]] \
       && printf '%s' "$arm11_out" | grep -qi "disallowed host\|not allowed\|host"; then
        ok "arm11: wrong host URL → exit 1, 'host' in output, sleep never called (clock=0)"
    elif [[ "$arm11_exit" -ne 0 ]] && [[ "${arm11_clock:-0}" -eq 0 ]]; then
        fail "arm11: wrong host URL → exit 1, no sleep, but host mention not found in output"
        printf '%s\n' "$arm11_out" | head -5 | sed 's/^/  /'
    elif [[ "$arm11_exit" -ne 0 ]]; then
        fail "arm11: wrong host URL → exit 1 but sleep was called (clock=${arm11_clock}) — host check did not cause immediate fail"
    else
        fail "arm11: wrong host URL → expected exit 1, got $arm11_exit (host check not enforced)"
        printf '%s\n' "$arm11_out" | head -5 | sed 's/^/  /'
    fi

    # ── Arm 12: partial list (4 of 8) → retry → pass ──────────────────────
    # arm12-pypi.py returns 4 wheels on calls 1–2, all 8 on call 3.
    # A2 fix: partial list → retry (not fail); exits 0 when full list arrives.
    echo ""
    echo "--- Arm 12: partial list (4 of 8 first 2 calls, 8 on 3rd) — expect exit 0 ---"
    COUNTER12="$WORK/arm12-counter"
    CLOCK12="$WORK/arm12-clock"
    printf '' > "$COUNTER12"
    printf '0\n' > "$CLOCK12"
    set +e
    arm12_out=$(
        STUB_COUNTER_FILE="$COUNTER12" \
        FAKE_CLOCK_FILE="$CLOCK12" \
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-8.json" \
        FIXTURE_WHEEL_DIR="$WORK/fixture-good" \
        PYPI_JSON_CMD="python3 $WORK/arm12-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=60 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
    )
    arm12_exit=$?
    set -e
    arm12_calls=$(cat "$COUNTER12" 2>/dev/null | tr -d '[:space:]' || echo 0)
    if [[ "$arm12_exit" -eq 0 ]] && [[ "${arm12_calls:-0}" -ge 3 ]]; then
        ok "arm12: partial list → retry → exit 0 on 3rd call (${arm12_calls} JSON fetches)"
    elif [[ "$arm12_exit" -eq 0 ]] && [[ "${arm12_calls:-0}" -lt 3 ]]; then
        fail "arm12: partial list → exit 0 but only ${arm12_calls} JSON fetches (expected ≥3 for partial→full path)"
    else
        fail "arm12: partial list → expected exit 0, got $arm12_exit (A2 retry not working)"
        printf '%s\n' "$arm12_out" | head -8 | sed 's/^/  /'
    fi

    # ── Arm 13: extra tag (linux_riscv64) → FAIL ──────────────────────────
    # pass-pypi.py with 9-entry manifest returns 9 wheels; tag checker finds
    # linux_riscv64 not in EXPECTED_WHEEL_TAGS → FAIL (A4 fix: was warn only).
    echo ""
    echo "--- Arm 13: extra wheel tag linux_riscv64 — expect exit 1 with tag named ---"
    CLOCK13="$WORK/arm13-clock"
    printf '0\n' > "$CLOCK13"
    set +e
    arm13_out=$(
        FAKE_CLOCK_FILE="$CLOCK13" \
        WHEEL_MANIFEST_FILE="$WORK/manifest-good-9.json" \
        FIXTURE_WHEEL_DIR="$WORK/fixture-good" \
        PYPI_JSON_CMD="python3 $WORK/pass-pypi.py" \
        FETCH_CMD="python3 $WORK/pass-fetch.py" \
        NPM_PACK_CMD="python3 $WORK/pass-npm.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/fake-now.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/fake-sleep.py" \
        VERIFY_CONTENT_DEADLINE=30 \
        bash "${BASH_SOURCE[0]}" --self-test-published 1.0.0 2>&1
    )
    arm13_exit=$?
    set -e
    if [[ "$arm13_exit" -ne 0 ]] && printf '%s' "$arm13_out" | grep -q "linux_riscv64"; then
        ok "arm13: extra tag linux_riscv64 → exit 1 with unexpected tag named"
    elif [[ "$arm13_exit" -ne 0 ]]; then
        fail "arm13: extra tag → exit 1 but 'linux_riscv64' not named in output"
        printf '%s\n' "$arm13_out" | grep -i "FAIL\|warn\|tag\|riscv" | head -5 | sed 's/^/  /'
    else
        fail "arm13: extra tag linux_riscv64 → expected exit 1, got $arm13_exit (unexpected-tag guard not firing)"
    fi

    # ── Arm 14: injectable vars in --published → warned, stubs not called ──
    # Calls --published (not --self-test-published) with all 5 injectable vars set to
    # recorder stubs. The A3 guard warns about each and unsets them; real system
    # commands (shimmed curl/npm) run instead. Proves the guard is enforced.
    echo ""
    echo "--- Arm 14: injectable vars in --published → warned and ignored, stubs not called ---"
    ARM14_STUB_REC="$WORK/arm14-stub-rec"
    ARM14_CURL_REC="$WORK/arm14-curl-rec"
    printf '' > "$ARM14_STUB_REC"
    printf '' > "$ARM14_CURL_REC"
    set +e
    arm14_out=$(
        ARM14_STUB_REC="$ARM14_STUB_REC" \
        ARM14_CURL_REC="$ARM14_CURL_REC" \
        PYPI_JSON_CMD="python3 $WORK/arm14-stub.py" \
        FETCH_CMD="python3 $WORK/arm14-stub.py" \
        NPM_PACK_CMD="python3 $WORK/arm14-stub.py" \
        VERIFY_CONTENT_SLEEP_CMD="python3 $WORK/arm14-stub.py" \
        VERIFY_CONTENT_NOW_CMD="python3 $WORK/arm14-stub.py" \
        VERIFY_CONTENT_DEADLINE=1 \
        PATH="$WORK:$PATH" \
        bash "${BASH_SOURCE[0]}" --published 0.0.0-never 2>&1
    )
    arm14_exit=$?
    set -e
    arm14_warn_count=$(printf '%s' "$arm14_out" | grep -c "WARN:.*ignorada fora do --self-test" || true)
    arm14_stub_lines=$(wc -l < "$ARM14_STUB_REC" | tr -d ' ')
    arm14_curl_lines=$(wc -l < "$ARM14_CURL_REC" | tr -d ' ')
    if [[ "$arm14_exit" -ne 0 ]] \
       && [[ "${arm14_warn_count:-0}" -ge 5 ]] \
       && [[ "${arm14_stub_lines:-1}" -eq 0 ]] \
       && [[ "${arm14_curl_lines:-0}" -ge 1 ]]; then
        ok "arm14: injectable vars in --published → ${arm14_warn_count} WARN lines, stubs not called (${arm14_stub_lines} lines), real curl used (${arm14_curl_lines} calls), exit $arm14_exit"
    else
        fail "arm14: injectable guard misbehaved (exit=${arm14_exit} warns=${arm14_warn_count:-?} stub_calls=${arm14_stub_lines:-?} curl_calls=${arm14_curl_lines:-?})"
        printf '%s\n' "$arm14_out" | grep "WARN\|FAIL" | head -8 | sed 's/^/  /'
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
# --self-test-published (internal mode — used by self-test arms 5–9, 12–14)
# Identical to --published but bypasses the A3 injectable guard so stub-based
# arms work. This mode is argv-only and is NOT listed in the usage message.
# It cannot be set via $GITHUB_ENV, so it does not reopen the A3 surface.
# ═══════════════════════════════════════════════════════════════════════════
_PUBLISHED_ALLOW_STUBS=0
if [[ "$MODE" == "--self-test-published" ]]; then
    _PUBLISHED_ALLOW_STUBS=1
fi

# ═══════════════════════════════════════════════════════════════════════════
# Published mode (--published VERSION): download and inspect live artifacts
# ═══════════════════════════════════════════════════════════════════════════
if [[ "$MODE" == "--published" || "$MODE" == "--self-test-published" ]]; then
    VERSION="${2:-}"
    if [[ -z "$VERSION" ]]; then
        echo "check-channels-content: --published requires a VERSION argument" >&2
        exit 1
    fi

    # ── A3: injectable var guard ──────────────────────────────────────────
    # In --published mode, injectable command overrides are NOT allowed.
    # They could be set accidentally via $GITHUB_ENV (supply-chain surface).
    # Warn about each and unset them so real commands are used unconditionally.
    # Bypassed only in --self-test-published (argv-only, not env-settable).
    if [[ "$_PUBLISHED_ALLOW_STUBS" -eq 0 ]]; then
        for _g3var in PYPI_JSON_CMD FETCH_CMD NPM_PACK_CMD VERIFY_CONTENT_SLEEP_CMD VERIFY_CONTENT_NOW_CMD; do
            _g3val="${!_g3var:-}"
            if [[ -n "$_g3val" ]]; then
                echo "WARN: $_g3var ignorada fora do --self-test" >&2
                unset "$_g3var"
            fi
        done
        unset _g3var _g3val 2>/dev/null || true
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
    _fetch_rc=0
    _fetch_pypi_wheels_with_retry "$PY_VERSION" "$WHEEL_WORK" || _fetch_rc=$?

    if [[ "$_fetch_rc" -eq 0 ]]; then
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
            # An UNEXPECTED tag means EXPECTED_WHEEL_TAGS and the platform matrix are
            # out of sync — update them together.
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
        print(f"FAIL: unexpected wheel tag: {t} — update EXPECTED_WHEEL_TAGS and the platform matrix together", file=sys.stderr)
    exit_code = 1
if not missing and not extra:
    print(f"ok: all {len(expected)} expected wheel tags present: {', '.join(sorted(actual))}")
sys.exit(exit_code)
PYEOF
            then
                ok "wheel count: $WHEEL_COUNT of $EXPECTED_WHEEL_COUNT expected wheels present"
            else
                fail "wheel count: $WHEEL_COUNT wheels downloaded but tag check failed — see above"
            fi
        fi
    elif [[ "$_fetch_rc" -eq 2 ]]; then
        fail "PyPI wheels: security failure (disallowed URL host or sha256 mismatch) — see above"
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
