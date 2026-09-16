#!/usr/bin/env bash
# check-thirdparty-parity.sh — pins the third-party artifact gate behavior
# (ADR-2026-08-15-gate-de-duas-fases-...): Go binary only.
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. npm/src/ and pypi/trackfw/ deleted. This gate now asserts the Go
# binary's behavior only — marker corpus in Go tests, D9 install round trip
# via Go CLI, D2 branch (i) violation message via Go CLI, and D10.1 refusal
# message via Go CLI.
#
# Original: ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-
# agentes-especialistas, ML-3A. Cross-runtime comparison removed by ML-3A (v8).
set -euo pipefail

export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
case "$GO_BIN" in
  /*) ;;
  *) GO_BIN="$ROOT_DIR/${GO_BIN#./}" ;;
esac
if [[ ! -x "$GO_BIN" ]]; then
  echo "check-thirdparty-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-thirdparty-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT INT TERM

export GOCACHE="$WORK/go-cache"
export PYTHONDONTWRITEBYTECODE=1
export TRACKFW_ORCHESTRATOR_SESSION=1

fail=0
ok() { echo "OK: $1"; }
bad() { echo "FAIL: $1" >&2; fail=1; }

# ── PART A — marker corpus coverage: the tricky cases must exist in Go tests ──
# The roadmap names these specific cases as the set where a naive regex would
# diverge from the CommonMark-style line-scanner (D3 amendment). Only Go tests
# are checked — Node.js/Python CLIs no longer exist (v8 single-runtime).
declare -A corpus_grep=(
  [heading_h1]='EachMarkerRefusedAsH1'
  [heading_h6]='EachMarkerRefusedAsH6'
  [fence_backtick]='MarkerInsideFencedBlockAccepted'
  [fence_tilde]='MarkerInsideTildeFencedBlockAccepted'
  [fence_unclosed]='UnclosedFenceNoLongerGrantsImmunity'
  [fence_short_close]='CloserShorterThanOpenerDoesNotCloseButStillCaught'
  [fence_indented]='IndentedFenceStillRecognized'
  [fence_then_heading]='HeadingAfterClosedFenceStillMatches'
  [fullwidth]='FullwidthCompatibilityCharsRefused'
  [cyrillic_pass]='CyrillicHomoglyphPasses'
  [html_comment]='HTMLCommentNeutralizedContentStillMatches'
  [casefold]='CasefoldIsSimpleLowercaseNotFullCasefold'
  [security_doc_nonregression]='SecurityOpinionDocumentDoesNotRefuseItself'
  [benign]='BenignContentAccepted'
)
GO_MARKERS_TEST="$ROOT_DIR/internal/thirdparty/markers_test.go"
for case_name in "${!corpus_grep[@]}"; do
  pattern=${corpus_grep[$case_name]}
  if ! grep -Eq "$pattern" "$GO_MARKERS_TEST"; then
    bad "marker corpus case '$case_name' not found in go ($GO_MARKERS_TEST)"
  fi
done
[[ "$fail" -eq 0 ]] && ok "marker corpus cases present in Go stack (v8 single-runtime)"

# ── PART B — D9 round trip (network-free, Go only) ─────────────────────────
# Hand-author quarantine + provenance records, run `third-party install` via
# Go CLI, and verify stdout, installed skill file, integrations-manifest.json
# and thirdparty-references.json.
#
# CONTENT is deliberately non-canonical (trailing blank line) so that
# INSTALLED_SHA256 (normalized) differs from CHECKSUM (raw), exercising D2-bis.
CONTENT='# Example Third-Party Skill

Some helpful, benign content for the agent to consume.

'
CHECKSUM=$(printf '%s' "$CONTENT" | python3 -c 'import sys,hashlib; sys.stdout.write(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())')
CONTENT_B64=$(printf '%s' "$CONTENT" | python3 -c 'import sys,base64; sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())')

INSTALLED_SHA256=$(printf '%s' "$CONTENT" | python3 -c '
import sys, hashlib
raw = sys.stdin.buffer.read()
normalized = raw.strip() + b"\n"
assert normalized != raw, "fixture CONTENT must NOT be canonical, or installed_sha256 cannot be distinguished from checksum_sha256 in this gate"
sys.stdout.write(hashlib.sha256(normalized).hexdigest())
')
if [[ "$INSTALLED_SHA256" == "$CHECKSUM" ]]; then
  bad "fixture setup: INSTALLED_SHA256 must differ from CHECKSUM (raw vs normalized), got the same value"
fi

write_quarantine_and_provenance() {
  local project=$1
  mkdir -p "$project/.trackfw/thirdparty-quarantine"
  cat >"$project/.trackfw/thirdparty-quarantine/$CHECKSUM.json" <<EOF
{
  "schema_version": 1,
  "url": "https://example.com/skills/my-skill.md",
  "checksum_sha256": "$CHECKSUM",
  "fetched_at": "2026-08-15T00:00:00Z",
  "content_base64": "$CONTENT_B64",
  "marker_check": {"result": "pass", "matched_markers": []},
  "kind": "skill",
  "requested_targets": ["claude"]
}
EOF
  cat >"$project/.trackfw/thirdparty-provenance.json" <<EOF
{
  "schema_version": 2,
  "entries": {
    ".claude/skills/thirdparty/my-skill.md": {
      "url": "https://example.com/skills/my-skill.md",
      "checksum_sha256": "$CHECKSUM",
      "installed_at": "2026-08-15T00:00:00Z",
      "approved_by": "hades-tf",
      "review_reference": "docs/seguranca/example.md",
      "scope": "project",
      "marker_override": false
    }
  }
}
EOF
}

project="$WORK/go/project"
home="$WORK/go/home"
mkdir -p "$project" "$home"
project=$(cd "$project" && pwd -P)
home=$(cd "$home" && pwd -P)

(cd "$project" && HOME="$home" "$GO_BIN" agents install --targets claude --items backend --scope project \
  >"$WORK/go-agent-install.out" 2>&1)

write_quarantine_and_provenance "$project"

set +e
(cd "$project" && HOME="$home" "$GO_BIN" skills third-party install \
  --checksum "$CHECKSUM" --targets claude --apply-to backend --yes-i-trust-this-source \
  >"$WORK/go-install.out" 2>&1)
install_status=$?
set -e
if [[ $install_status -ne 0 ]]; then
  bad "go: third-party install failed (exit $install_status):"
  cat "$WORK/go-install.out" >&2
else
  ok "go: third-party install succeeded (D9 round trip)"
fi

# Verify installed artifacts exist
[[ -f "$project/.claude/skills/thirdparty/my-skill.md" ]] \
  && ok "go: installed skill file found" \
  || bad "go: installed skill file not found"
[[ -f "$project/.trackfw/integrations-manifest.json" ]] \
  && ok "go: integrations-manifest.json found" \
  || bad "go: integrations-manifest.json not found"
[[ -f "$project/.trackfw/thirdparty-references.json" ]] \
  && ok "go: thirdparty-references.json found (D9 schema 3)" \
  || bad "go: thirdparty-references.json not found"

# D2-bis: installed_sha256 must be the normalized hash, not the raw hash
installed_sha256=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['entries']['.claude/skills/thirdparty/my-skill.md']['installed_sha256'])" \
  "$project/.trackfw/thirdparty-provenance.json" 2>/dev/null || true)
if [[ "$installed_sha256" == "$INSTALLED_SHA256" ]]; then
  ok "go: thirdparty-provenance.json installed_sha256 matches normalized content hash (D2-bis)"
else
  bad "go: thirdparty-provenance.json installed_sha256 = '$installed_sha256', want '$INSTALLED_SHA256' (D2-bis)"
fi

# D11: integrations-manifest.json must carry origin=thirdparty claim
python3 - "$project/.trackfw/integrations-manifest.json" "$project" <<'PY' \
  && ok "go: integrations-manifest.json claim origin=thirdparty present (D11)" \
  || bad "go: integrations-manifest.json missing origin=thirdparty claim (D11)"
import json, sys
path, project = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as fh:
    doc = json.loads(fh.read().replace(project, "<PROJECT>"))
dest = "<PROJECT>/.claude/skills/thirdparty/my-skill.md"
artifact = doc["artifacts"].get(dest)
assert artifact is not None, f"expected artifact at {dest}, got keys {list(doc['artifacts'])}"
claims = artifact["claims"]
assert any(c.get("origin") == "thirdparty" for c in claims), f"origin=thirdparty not found in {claims}"
PY

# D2-bis end-to-end: legitimate install must produce ZERO validate violations
set +e
(cd "$project" && HOME="$home" "$GO_BIN" validate --json >"$WORK/go-clean-validate.json" 2>"$WORK/go-clean-validate.stderr")
set -e
violation_count=$(python3 -c "
import json, sys
with open(sys.argv[1], encoding='utf-8') as fh:
    payload = json.load(fh)
print(sum(1 for v in payload.get('violations', []) if v.get('rule') == 'thirdparty_artifact_has_provenance'))
" "$WORK/go-clean-validate.json")
if [[ "$violation_count" == "0" ]]; then
  ok "D2-bis: legitimate install of non-canonical content produces zero validate violations (go)"
else
  bad "go: legitimate non-canonical install produced $violation_count thirdparty_artifact_has_provenance violation(s), want 0 (D2-bis end-to-end)"
  cat "$WORK/go-clean-validate.json" >&2
fi

# ── PART C — D2 branch (i) violation message (Go only) ─────────────────────
rm -f "$project/.trackfw/thirdparty-provenance.json"
set +e
(cd "$project" && HOME="$home" "$GO_BIN" validate --json >"$WORK/go-validate.json" 2>"$WORK/go-validate.stderr")
set -e
python3 - "$WORK/go-validate.json" "$project" "$WORK/go-branch-i.msg" <<'PY'
import json, sys
path, project, out_path = sys.argv[1:4]
with open(path, encoding="utf-8") as fh:
    payload = json.load(fh)
msgs = [
    item["message"].replace(project, "<PROJECT>")
    for item in payload.get("violations", [])
    if item.get("rule") == "thirdparty_artifact_has_provenance"
]
with open(out_path, "w", encoding="utf-8") as fh:
    fh.write("\n".join(sorted(msgs)) + "\n")
PY
if [[ -s "$WORK/go-branch-i.msg" ]]; then
  ok "D2 branch (i): validate produces thirdparty_artifact_has_provenance violation message (go — behavioral pin)"
else
  bad "D2 branch (i): validate produced no thirdparty_artifact_has_provenance violation message (go)"
  cat "$WORK/go-validate.json" >&2
fi

# ── PART D — D10.1 --apply-to scope-mismatch refusal message (Go only) ─────
project_d10="$WORK/d10/go/project"
home_d10="$WORK/d10/go/home"
mkdir -p "$project_d10" "$home_d10"
project_d10=$(cd "$project_d10" && pwd -P)
home_d10=$(cd "$home_d10" && pwd -P)
write_quarantine_and_provenance "$project_d10"
set +e
(cd "$project_d10" && HOME="$home_d10" "$GO_BIN" skills third-party install \
  --checksum "$CHECKSUM" --targets claude --apply-to backend --yes-i-trust-this-source \
  >"$WORK/go-d10.out" 2>&1)
status=$?
set -e
if [[ $status -ne 0 ]]; then
  ok "go: D10.1 refusal fired (agent not installed) — exit non-zero"
else
  bad "go: expected D10.1 refusal (agent not installed) but install succeeded"
fi
if grep -q 'cannot attach reference:' "$WORK/go-d10.out"; then
  ok "D10.1 remediation message present in go output (behavioral pin)"
else
  bad "D10.1 remediation message 'cannot attach reference:' not found in go output"
  cat "$WORK/go-d10.out" >&2
fi

if [[ "$fail" -ne 0 ]]; then
  echo "check-thirdparty-parity: FAILED" >&2
  exit 1
fi
echo "Third-party artifact gate checks passed (Go only — v8 single-runtime; D9 install, D2-bis, D11, D2 branch i, D10.1)"
