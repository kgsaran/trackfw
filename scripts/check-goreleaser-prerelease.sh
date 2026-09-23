#!/usr/bin/env bash
# check-goreleaser-prerelease.sh — asserts that .goreleaser.yaml declares
# prerelease: auto in its release: block.
#
# Without this key, GoReleaser defaults to prerelease: false, meaning any tag
# with a pre-release suffix (e.g. v8.0.0-rc1) is published as a stable GitHub
# Release and becomes the `latest` release via the releases/latest API endpoint.
# scripts/install.sh resolves the version via that endpoint, so it installs the
# pre-release as if it were stable.
#
# Measured 2026-09-16: v8.0.0-rc1 served as `latest` for 3 days because
# this key was absent. The release was corrected manually by setting
# prerelease: true via the GitHub API, but the root cause was the missing
# configuration in .goreleaser.yaml — it would have recurred on rc2.
#
# Value choice — `auto`, not `true`:
#   `true` marks every release as pre-release, which would demote stable tags
#   (e.g. v8.0.0) and break the stable channel. `auto` lets GoReleaser derive
#   the flag from the tag suffix: tags matching its semver pre-release pattern
#   publish as pre-releases; clean semver tags (v8.0.0) publish as stable.
#   Consistent with `brews.skip_upload: auto` already in use in the same file.
#
# Vacuity guards:
#   - .goreleaser.yaml missing → FAIL (not silent pass)
#   - release: block absent → FAIL (named)
#   - prerelease key absent within release: → FAIL (named)
#
# Self-test (--self-test): runs both falsification directions:
#   Direction A — prerelease key absent or wrong value → gate FAILS, naming the key
#   Direction B — correct config passes; file-absent and block-absent → gate FAILS
#
# Reconciliation (Regra Dura — CLAUDE.md):
#   This gate asserts: .goreleaser.yaml is configured so GoReleaser treats tags
#   with pre-release suffixes as pre-releases, not stable releases.
#
# ML-3E (ROADMAP-2026-09-12-v8-um-binario-muitos-canais)

set -euo pipefail
export PYTHONIOENCODING=utf-8
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=scripts/lib-crlf-normalize.sh
. "$SCRIPT_DIR/lib-crlf-normalize.sh"

GORELEASER_YAML="${GORELEASER_YAML:-$REPO_ROOT/.goreleaser.yaml}"

PASS=0
FAIL=0
ok()   { echo "OK   [$1]"; PASS=$((PASS+1)); }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=$((FAIL+1)); }

# ── Core check ─────────────────────────────────────────────────────────────
check_prerelease() {
    local yaml_file="$1" scenario="$2"

    if [[ ! -f "$yaml_file" ]]; then
        fail "$scenario" ".goreleaser.yaml not found: $yaml_file"
        return
    fi

    local result
    result=$(python3 - "$yaml_file" <<'PYEOF' | strip_cr
import sys, yaml
with open(sys.argv[1]) as f:
    # yaml.safe_load returns None for an empty file; `or {}` makes it a dict
    doc = yaml.safe_load(f) or {}
# If the release: key is not present at all, report NO_RELEASE_BLOCK
if 'release' not in doc:
    print('NO_RELEASE_BLOCK')
    sys.exit(0)
# release: bare key returns None in YAML (not a dict); treat as absent block
rel = doc['release']
if not isinstance(rel, dict):
    print('NO_RELEASE_BLOCK')
    sys.exit(0)
v = rel.get('prerelease', '__ABSENT__')
print(str(v))
PYEOF
    )

    case "$result" in
        NO_RELEASE_BLOCK)
            fail "$scenario" "release: block is absent or not a mapping — GoReleaser will default prerelease to false"
            ;;
        __ABSENT__)
            fail "$scenario" "release.prerelease is absent — GoReleaser defaults to false; pre-release tags will publish as stable"
            ;;
        auto)
            ok "$scenario"
            ;;
        *)
            fail "$scenario" "release.prerelease is '$result', expected 'auto'"
            ;;
    esac
}

# ── Self-test (falsification) ───────────────────────────────────────────────
if [[ "${1:-}" == "--self-test" ]]; then
    WORK=$(mktemp -d "${TMPDIR:-/tmp}/check-goreleaser-prerelease.XXXXXX")
    trap 'rm -rf "$WORK"' EXIT

    # ── Direction A: key absent → FAIL naming the key ──────────────────────

    # A1: prerelease line removed (realistic regression: dropped in a merge)
    sed '/^  prerelease:/d' "$GORELEASER_YAML" > "$WORK/absent.yaml"
    if cmp -s "$WORK/absent.yaml" "$GORELEASER_YAML"; then
        fail "goreleaser-prerelease/direction-a-absent" \
            "fixture is identical to source — sed did not remove any prerelease line; check .goreleaser.yaml"
    else
        OUT_A1=$(GORELEASER_YAML="$WORK/absent.yaml" "$SCRIPT_DIR/check-goreleaser-prerelease.sh" 2>&1 || true)
        if echo "$OUT_A1" | grep -qF "release.prerelease is absent"; then
            ok "goreleaser-prerelease/direction-a-absent"
        else
            fail "goreleaser-prerelease/direction-a-absent" \
                "gate did not emit expected 'release.prerelease is absent'; output: $OUT_A1"
        fi
    fi

    # A2: wrong value (false) — note Python str(False) == 'False', not 'false'
    python3 - "$GORELEASER_YAML" "$WORK/false_val.yaml" <<'PYEOF'
import sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f) or {}
doc.setdefault('release', {})['prerelease'] = False
with open(sys.argv[2], 'w') as f:
    yaml.dump(doc, f, default_flow_style=False, allow_unicode=True)
PYEOF
    if cmp -s "$WORK/false_val.yaml" "$GORELEASER_YAML"; then
        fail "goreleaser-prerelease/direction-a-false-value" \
            "fixture is identical to source — mutation did not change prerelease value"
    else
        OUT_A2=$(GORELEASER_YAML="$WORK/false_val.yaml" "$SCRIPT_DIR/check-goreleaser-prerelease.sh" 2>&1 || true)
        if echo "$OUT_A2" | grep -qF "release.prerelease is 'False'"; then
            ok "goreleaser-prerelease/direction-a-false-value"
        else
            fail "goreleaser-prerelease/direction-a-false-value" \
                "gate did not emit expected \"release.prerelease is 'False'\"; output: $OUT_A2"
        fi
    fi

    # ── Direction B: correct config passes; vacuity guards fire ────────────

    # B1: real .goreleaser.yaml passes
    OUT_B1=$(GORELEASER_YAML="$GORELEASER_YAML" "$SCRIPT_DIR/check-goreleaser-prerelease.sh" 2>&1 || true)
    if echo "$OUT_B1" | grep -q "^OK"; then
        ok "goreleaser-prerelease/direction-b-correct-config"
    else
        fail "goreleaser-prerelease/direction-b-correct-config" \
            "gate FAILED for correct config; output: $OUT_B1"
    fi

    # B2: file absent → FAIL (not silent pass)
    OUT_B2=$(GORELEASER_YAML="$WORK/nonexistent.yaml" "$SCRIPT_DIR/check-goreleaser-prerelease.sh" 2>&1 || true)
    if echo "$OUT_B2" | grep -q "FAIL"; then
        ok "goreleaser-prerelease/direction-b-file-absent"
    else
        fail "goreleaser-prerelease/direction-b-file-absent" \
            "gate passed silently for missing file; output: $OUT_B2"
    fi

    # B3: release: block absent → FAIL (named, distinct message from A1)
    python3 - "$GORELEASER_YAML" "$WORK/no_release_block.yaml" <<'PYEOF'
import sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f) or {}
doc.pop('release', None)
with open(sys.argv[2], 'w') as f:
    yaml.dump(doc, f, default_flow_style=False, allow_unicode=True)
PYEOF
    OUT_B3=$(GORELEASER_YAML="$WORK/no_release_block.yaml" "$SCRIPT_DIR/check-goreleaser-prerelease.sh" 2>&1 || true)
    if echo "$OUT_B3" | grep -qF "release: block is absent"; then
        ok "goreleaser-prerelease/direction-b-block-absent"
    else
        fail "goreleaser-prerelease/direction-b-block-absent" \
            "gate did not emit expected 'release: block is absent'; output: $OUT_B3"
    fi

    echo ""
    echo "goreleaser-prerelease (self-test): $PASS passed, $FAIL failed"
    [[ $FAIL -eq 0 ]] || exit 1
    exit 0
fi

# ── Normal mode ─────────────────────────────────────────────────────────────
check_prerelease "$GORELEASER_YAML" "goreleaser-prerelease/release-block"

echo ""
echo "goreleaser-prerelease: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]] || exit 1
