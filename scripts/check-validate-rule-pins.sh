#!/usr/bin/env bash
# check-validate-rule-pins.sh — behavioral pins on `trackfw validate` (Go only)
# extracted from check-validate-parity.sh before Wave 3 deletes the Node.js and
# Python CLI implementations (ML-3C, ROADMAP-2026-09-12-v8-um-binario-muitos-canais).
#
# The cross-runtime comparison assertions from check-validate-parity.sh die when
# Node/Python are removed — these pins survive because they assert Go-only properties:
# specific rule names, message substrings, exit codes.  Nothing here invokes
# `node npm/bin/trackfw` or `python3 -m trackfw`.
#
# Pin inventory (25 pins across 4 blocks):
#   Block 1 — ADR/REQ rule-set:         {adr_accepted_when_req_done, blocked_by_draft_adr}
#   Block 2 — branch_has_wip_roadmap:    nomatch/diff message markers + --agent guidance
#   Block 3 — credential_guard:          10 message pins + 5 silence pins
#   Block 4 — git_branch_guard:          1 message pin + 1 silence pin + 3 shared-fixture pins
#
# Falsification evidence is recorded after `make quality` run.
set -euo pipefail

export PYTHONIOENCODING=utf-8
export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-validate-rule-pins.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT

# Isolate $HOME — prevent real developer global guards from interfering.
export GOPATH="${GOPATH:-$(go env GOPATH)}"
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
export HOME="$TMP_DIR/home"
mkdir -p "$HOME"

# GO_BIN: same convention as check-validate-parity.sh.
if [[ -z "${GO_BIN:-}" ]]; then
  GOCACHE=${GOCACHE:-/tmp/trackfw-go-cache} go build -o "$TMP_DIR/trackfw-go" ./cmd/trackfw
  GO_BIN="$TMP_DIR/trackfw-go"
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi

# Normalize TMP_DIR path (macOS $TMPDIR may have trailing /) — same reasoning as
# check-validate-parity.sh (Go collapses via filepath.Join; Python does not).
CG_TMP=$(printf '%s' "$TMP_DIR" | sed 's#//*#/#g')

# ---------------------------------------------------------------------------
# BLOCK 1: Rule-set pin
#
# Go must report BOTH adr_accepted_when_req_done AND blocked_by_draft_adr for
# the ADR-Proposed/REQ-Done/REQ-Blocked fixture (exit code 1, non-empty list).
# A total-count check alone would pass if one rule silently dropped — this
# asserts both names explicitly (equivalent to check-validate-parity.sh L196-215,
# "P2 vacuity guard, per-rule").
# ---------------------------------------------------------------------------
mkdir -p \
  "$TMP_DIR/p1/docs/adr" \
  "$TMP_DIR/p1/docs/req" \
  "$TMP_DIR/p1/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}

cat >"$TMP_DIR/p1/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

cat >"$TMP_DIR/p1/docs/roadmaps/wip/RM.md" <<'EOF'
---
status: WIP
---
# Roadmap without required governance links
EOF

cat >"$TMP_DIR/p1/docs/adr/ADR-proposed-fixture.md" <<'EOF'
---
status: Proposed
date: 2026-08-01
author: ""
---

# ADR: fixture

> Date: 2026-08-01 | Status: Proposed

## Context
ctx

## Decision
decision
EOF

cat >"$TMP_DIR/p1/docs/req/REQ-done-fixture.md" <<'EOF'
---
status: Done
date: 2026-08-01
author: ""
adr: "docs/adr/ADR-proposed-fixture.md"
roadmap: ""
---

# REQ: fixture

> Date: 2026-08-01 | Status: Done

## Motivation
motivo

## Acceptance Criteria
- [x] feito

## Linked ADR
ADR: docs/adr/ADR-proposed-fixture.md

## Linked Roadmap
Roadmap:
EOF

cat >"$TMP_DIR/p1/docs/req/REQ-blocked-fixture.md" <<'EOF'
---
status: Open
date: 2026-08-01
author: ""
adr: ""
roadmap: ""
---

# REQ: bloqueada

> Date: 2026-08-01 | Status: Open

## Motivation
motivo

## Acceptance Criteria
- [ ] pendente

## Linked ADR
ADR:

## Blocked by ADRs
- ADR-proposed-fixture.md (Proposed)

## Linked Roadmap
Roadmap:
EOF

set +e
(cd "$TMP_DIR/p1" && "$GO_BIN" validate --json) >"$TMP_DIR/p1.json" 2>"$TMP_DIR/p1.stderr"
P1_RC=$?
set -e

python3 - "$TMP_DIR/p1.json" "$P1_RC" <<'PY'
import json, sys
path, rc = sys.argv[1], int(sys.argv[2])
# Vacuity guard: must exit 1 (violations present).
if rc != 1:
    raise SystemExit(
        f"PIN1 vacuity: expected exit code 1, got {rc} — "
        "fixture broken or validate regressed to exit 0 with violations"
    )
with open(path, encoding="utf-8") as f:
    payload = json.load(f)
violations = payload.get("violations", [])
if not violations:
    raise SystemExit(
        "PIN1 vacuity: violations list is empty — fixture broken "
        "(no governance violations triggered)"
    )
got_rules = {item.get("rule") for item in violations}
expected = {"adr_accepted_when_req_done", "blocked_by_draft_adr"}
missing = expected - got_rules
if missing:
    raise SystemExit(
        f"PIN1: validate missing rule(s) {sorted(missing)} — "
        f"rule regressed or fixture broken. Got rules: {sorted(r for r in got_rules if r)}"
    )
print("OK [validate-rule-pins/pin1-rule-set]  "
      "{adr_accepted_when_req_done, blocked_by_draft_adr}")
PY

# ---------------------------------------------------------------------------
# BLOCK 2: branch_has_wip_roadmap message pins
#
# Pins:
#   PIN2 — done/ with matching slug → no violation (acceptance behavior)
#   PIN3 — empty wip/ and done/ → "no roadmap is in wip/ nor done/"
#   PIN4 — done/ with different slug → "no matching roadmap in wip/ nor done/"
#   PIN5 — by_agent + 2 agents + no roadmap → message contains "--agent"
# ---------------------------------------------------------------------------
mkdir -p \
  "$TMP_DIR/bhr-match/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-nomatch/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-diff/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-byagent/docs/roadmaps/zeus"/{wip,done} \
  "$TMP_DIR/bhr-byagent/docs/roadmaps/apolo"/{wip,done}

for d in bhr-match bhr-nomatch bhr-diff; do
  cat >"$TMP_DIR/$d/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
done

cat >"$TMP_DIR/bhr-byagent/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
  - zeus
  - apolo
EOF

cat >"$TMP_DIR/bhr-match/docs/roadmaps/done/ROADMAP-2026-08-20-minha-feature.md" <<'EOF'
---
status: done
---
# Roadmap: minha feature
EOF

cat >"$TMP_DIR/bhr-diff/docs/roadmaps/done/ROADMAP-2026-08-20-outra-coisa.md" <<'EOF'
---
status: done
---
# Roadmap: outra coisa
EOF

run_bhr() {
  local out="$1" dir="$2" branch="$3"
  set +e
  ( cd "$dir" && TRACKFW_BRANCH="$branch" "$GO_BIN" validate --json ) \
    >"$out" 2>"$out.stderr"
  echo $? >"$out.exit"
  set -e
}

run_bhr "$TMP_DIR/bhr-match-go.json"   "$TMP_DIR/bhr-match"   feat/minha-feature
run_bhr "$TMP_DIR/bhr-nomatch-go.json" "$TMP_DIR/bhr-nomatch" feat/sem-roadmap-nenhum
run_bhr "$TMP_DIR/bhr-diff-go.json"    "$TMP_DIR/bhr-diff"    feat/minha-feature
run_bhr "$TMP_DIR/bhr-byagent-go.json" "$TMP_DIR/bhr-byagent" feat/sem-roadmap-byagent

python3 - "$TMP_DIR" <<'PY'
import json, os, sys
tmp = sys.argv[1]
BHR_MARKER = "wip/ nor done/"

def load_bhr(name):
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    with open(path + ".exit") as f:
        rc = int(f.read().strip())
    matching = [
        item for item in payload.get("violations", [])
        if BHR_MARKER in item.get("message", "")
    ]
    return rc, sorted(item["message"] for item in matching)

# PIN2: done/ with matching slug — no branch_has_wip_roadmap violation.
rc, msgs = load_bhr("bhr-match-go.json")
if msgs:
    raise SystemExit(
        f"PIN2: bhr-match: roadmap in done/ with matching slug must NOT trigger "
        f"branch_has_wip_roadmap, but got: {msgs!r}"
    )
print("OK [validate-rule-pins/pin2-bhr-match-accepted]")

# PIN3: empty wip/ and done/ — message contains "no roadmap is in wip/ nor done/".
rc, msgs = load_bhr("bhr-nomatch-go.json")
if not msgs:
    raise SystemExit(
        f"PIN3 vacuity: bhr-nomatch: expected branch_has_wip_roadmap violation, "
        f"none found (rc={rc}) — fixture broken or rule regressed"
    )
MARKER_NOMATCH = "no roadmap is in wip/ nor done/"
if not all(MARKER_NOMATCH in m for m in msgs):
    raise SystemExit(
        f"PIN3: bhr-nomatch: message lacks {MARKER_NOMATCH!r}: {msgs!r}"
    )
print("OK [validate-rule-pins/pin3-bhr-nomatch-message]")

# PIN4: done/ with different slug — message contains "no matching roadmap in wip/ nor done/".
rc, msgs = load_bhr("bhr-diff-go.json")
if not msgs:
    raise SystemExit(
        f"PIN4 vacuity: bhr-diff: expected branch_has_wip_roadmap violation, "
        f"none found (rc={rc}) — fixture broken or rule regressed"
    )
MARKER_DIFF = "no matching roadmap in wip/ nor done/"
if not all(MARKER_DIFF in m for m in msgs):
    raise SystemExit(
        f"PIN4: bhr-diff: message lacks {MARKER_DIFF!r}: {msgs!r}"
    )
print("OK [validate-rule-pins/pin4-bhr-diff-message]")

# PIN5: by_agent + 2 agents + no roadmap — message contains "--agent".
rc, msgs = load_bhr("bhr-byagent-go.json")
if not msgs:
    raise SystemExit(
        f"PIN5 vacuity: bhr-byagent: expected branch_has_wip_roadmap violation, "
        f"none found (rc={rc}) — fixture broken or rule regressed"
    )
if not any("--agent" in m for m in msgs):
    raise SystemExit(
        f"PIN5: bhr-byagent: orientation message must contain '--agent' for by_agent "
        f"project with 2 agents (zeus, apolo). Got: {msgs!r}"
    )
print("OK [validate-rule-pins/pin5-byagent-agent-guidance]")
PY

# ---------------------------------------------------------------------------
# BLOCK 3: credential_guard_hook_resolvable message pins
#
# 15 fixture cases → 10 expect-violation pins + 5 expect-silence pins.
# ---------------------------------------------------------------------------
make_cg_base() {
  local fix="$1"
  mkdir -p "$CG_TMP/$fix/docs/roadmaps"/{wip,done}
  cat >"$CG_TMP/$fix/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
}

for fix in cg-claude-absent cg-claude-present cg-claude-noexec cg-claude-notype \
           cg-claude-relativo cg-claude-pwd cg-claude-absoluto cg-claude-windows-drive \
           cg-claude-invalid-json cg-claude-unreadable cg-claude-utf16 \
           cg-claude-tilde-quoted cg-claude-tilde-user \
           cg-cursor-present cg-copilot-relativo-present; do
  make_cg_base "$fix"
done

# Claude settings.json with $CLAUDE_PROJECT_DIR/... command
for fix in cg-claude-absent cg-claude-present cg-claude-noexec; do
  mkdir -p "$CG_TMP/$fix/.claude"
  cat >"$CG_TMP/$fix/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"}]}]}}
EOF
done

# cg-claude-present: script present and executable → no violation.
mkdir -p "$CG_TMP/cg-claude-present/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/cg-claude-present/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP/cg-claude-present/scripts/trackfw-credential-guard.sh"

# cg-claude-noexec: script present but not executable (chmod 644) → "not executable".
mkdir -p "$CG_TMP/cg-claude-noexec/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/cg-claude-noexec/scripts/trackfw-credential-guard.sh"
chmod 644 "$CG_TMP/cg-claude-noexec/scripts/trackfw-credential-guard.sh"

# cg-claude-notype: hook without "type":"command" → 'missing "type":"command"'.
mkdir -p "$CG_TMP/cg-claude-notype/.claude"
cat >"$CG_TMP/cg-claude-notype/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"}]}]}}
EOF

# cg-claude-relativo: bare relative path (class 2) → "with a bare relative path".
mkdir -p "$CG_TMP/cg-claude-relativo/.claude" "$CG_TMP/cg-claude-relativo/scripts"
cat >"$CG_TMP/cg-claude-relativo/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"scripts/trackfw-credential-guard.sh"}]}]}}
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/cg-claude-relativo/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP/cg-claude-relativo/scripts/trackfw-credential-guard.sh"

# cg-claude-pwd: $PWD path (class 2) → "with a $PWD path".
mkdir -p "$CG_TMP/cg-claude-pwd/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\$PWD/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP/cg-claude-pwd/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"

# cg-claude-absoluto: absolute path (class 1) → silent.
mkdir -p "$CG_TMP/cg-claude-absoluto/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '/opt/trackfw/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP/cg-claude-absoluto/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"

# cg-claude-windows-drive: Windows drive letter (class 1 by union) → silent.
mkdir -p "$CG_TMP/cg-claude-windows-drive/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': r'C:\Users\kg\scripts\trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP/cg-claude-windows-drive/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"

# cg-claude-invalid-json: malformed JSON → "is not valid JSON".
mkdir -p "$CG_TMP/cg-claude-invalid-json/.claude"
printf '{"hooks": {,}}' >"$CG_TMP/cg-claude-invalid-json/.claude/settings.json"

# cg-claude-unreadable: directory in place of file → "could not be read".
mkdir -p "$CG_TMP/cg-claude-unreadable/.claude/settings.json"

# cg-claude-utf16: UTF-16 encoded JSON → "is not valid UTF-8".
mkdir -p "$CG_TMP/cg-claude-utf16/.claude"
python3 -c '
import sys
with open(sys.argv[1], "wb") as f:
    f.write("{\"hooks\":{}}".encode("utf-16"))
' "$CG_TMP/cg-claude-utf16/.claude/settings.json"

# cg-claude-tilde-quoted: "~/..." with outer quotes (class 2) → "with a quoted tilde path".
mkdir -p "$CG_TMP/cg-claude-tilde-quoted/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '\"' + r'~/scripts/trackfw-credential-guard.sh' + '\"'}]}]}}
with open('$CG_TMP/cg-claude-tilde-quoted/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"

# cg-claude-tilde-user: ~alice/... (class 2) → "with a named-user tilde path".
mkdir -p "$CG_TMP/cg-claude-tilde-user/.claude"
python3 -c "
import json
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '~alice/scripts/trackfw-credential-guard.sh'}]}]}}
with open('$CG_TMP/cg-claude-tilde-user/.claude/settings.json', 'w') as f:
    json.dump(d, f)
"

# cg-cursor-present: Cursor with script present → silent.
mkdir -p "$CG_TMP/cg-cursor-present/.cursor" "$CG_TMP/cg-cursor-present/scripts"
cat >"$CG_TMP/cg-cursor-present/.cursor/hooks.json" <<'EOF'
{"version":1,"hooks":{"beforeShellExecution":[{"command":"scripts/trackfw-credential-guard.sh"}]}}
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/cg-cursor-present/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP/cg-cursor-present/scripts/trackfw-credential-guard.sh"

# cg-copilot-relativo-present: Copilot with relative path + script present → silent.
mkdir -p "$CG_TMP/cg-copilot-relativo-present/.github/hooks" \
         "$CG_TMP/cg-copilot-relativo-present/scripts"
cat >"$CG_TMP/cg-copilot-relativo-present/.github/hooks/trackfw-attention.json" <<'EOF'
{"type":"command","bash":"scripts/trackfw-credential-guard.sh"}
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/cg-copilot-relativo-present/scripts/trackfw-credential-guard.sh"
chmod +x "$CG_TMP/cg-copilot-relativo-present/scripts/trackfw-credential-guard.sh"

# Run Go validate for each credential_guard fixture.
run_cg() {
  local out="$1" dir="$2"
  set +e
  ( cd "$dir" && "$GO_BIN" validate --json ) >"$out" 2>"$out.stderr"
  echo $? >"$out.exit"
  set -e
}

for fix in cg-claude-absent cg-claude-present cg-claude-noexec cg-claude-notype \
           cg-claude-relativo cg-claude-pwd cg-claude-absoluto cg-claude-windows-drive \
           cg-claude-invalid-json cg-claude-unreadable cg-claude-utf16 \
           cg-claude-tilde-quoted cg-claude-tilde-user \
           cg-cursor-present cg-copilot-relativo-present; do
  run_cg "$CG_TMP/$fix-go.json" "$CG_TMP/$fix"
done

python3 - "$CG_TMP" <<'PY'
import json, os, sys
tmp = sys.argv[1]
CG_RULE = "credential_guard_hook_resolvable"

def load_cg(name):
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    with open(path + ".exit") as f:
        rc = int(f.read().strip())
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [item for item in all_items if item.get("rule") == CG_RULE]
    return rc, sorted(item["message"] for item in matching)

# PIN6-PIN15: expect violations with specific message markers.
# Each tuple: (fixture-go-json, pin-label, message-marker)
expect_violation = [
    ("cg-claude-absent-go.json",       "pin6-absent",        "but the script does not exist"),
    ("cg-claude-noexec-go.json",       "pin7-noexec",        "not executable"),
    ("cg-claude-notype-go.json",       "pin8-notype",        'missing "type":"command"'),
    ("cg-claude-relativo-go.json",     "pin9-relativo",      "with a bare relative path"),
    ("cg-claude-pwd-go.json",          "pin10-pwd",          "with a $PWD path"),
    ("cg-claude-tilde-quoted-go.json", "pin11-tilde-quoted", "with a quoted tilde path"),
    ("cg-claude-tilde-user-go.json",   "pin12-tilde-user",   "with a named-user tilde path"),
    ("cg-claude-invalid-json-go.json", "pin13-invalid-json", "is not valid JSON"),
    ("cg-claude-unreadable-go.json",   "pin14-unreadable",   "could not be read"),
    ("cg-claude-utf16-go.json",        "pin15-utf16",        "is not valid UTF-8"),
]

for name, label, marker in expect_violation:
    rc, msgs = load_cg(name)
    if not msgs:
        raise SystemExit(
            f"[{label}] vacuity: {name}: expected {CG_RULE!r} violation, "
            f"none found (rc={rc}) — fixture broken or rule regressed"
        )
    if not all(marker in m for m in msgs):
        raise SystemExit(
            f"[{label}]: {name}: message does not contain {marker!r}: {msgs!r}"
        )
    print(f"OK [validate-rule-pins/{label}]  {marker!r}")

# PIN16-PIN20: expect silence (no violation from this rule).
expect_silent = [
    ("cg-claude-present-go.json",          "pin16-present-silent"),
    ("cg-claude-absoluto-go.json",          "pin17-absoluto-silent"),
    ("cg-claude-windows-drive-go.json",     "pin18-windows-drive-silent"),
    ("cg-cursor-present-go.json",           "pin19-cursor-present-silent"),
    ("cg-copilot-relativo-present-go.json", "pin20-copilot-relativo-silent"),
]

for name, label in expect_silent:
    rc, msgs = load_cg(name)
    if msgs:
        raise SystemExit(
            f"[{label}]: {name}: no {CG_RULE!r} violation expected, but got: {msgs!r}"
        )
    print(f"OK [validate-rule-pins/{label}]")
PY

# ---------------------------------------------------------------------------
# BLOCK 4: git_branch_guard_hook_resolvable pins
#
# Pins:
#   PIN21 — gbg-claude-relativo: "with a bare relative path"
#   PIN22 — gbg-cursor-relativo-present: silent (Cursor relative is correct)
#   PIN23 — gbg-claude-invalid-json: "is not valid JSON" (reuses Block 3 fixture)
#   PIN24 — gbg-claude-unreadable: "could not be read" (reuses Block 3 fixture)
#   PIN25 — gbg-claude-utf16: "is not valid UTF-8" (reuses Block 3 fixture)
#
# PIN23-25 reuse the cg outputs — the SAME corrupted file blinds both
# credential_guard and git_branch_guard simultaneously, proving both rules
# fail-closed rather than silencing on unreadable config.
# ---------------------------------------------------------------------------
GBG_RULE="git_branch_guard_hook_resolvable"

for fix in gbg-claude-relativo gbg-cursor-relativo-present; do
  mkdir -p "$CG_TMP/$fix/docs/roadmaps"/{wip,done}
  cat >"$CG_TMP/$fix/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
done

mkdir -p "$CG_TMP/gbg-claude-relativo/.claude" "$CG_TMP/gbg-claude-relativo/scripts"
cat >"$CG_TMP/gbg-claude-relativo/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"scripts/trackfw-git-branch-guard.sh"}]}]}}
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/gbg-claude-relativo/scripts/trackfw-git-branch-guard.sh"
chmod +x "$CG_TMP/gbg-claude-relativo/scripts/trackfw-git-branch-guard.sh"

mkdir -p "$CG_TMP/gbg-cursor-relativo-present/.cursor" "$CG_TMP/gbg-cursor-relativo-present/scripts"
cat >"$CG_TMP/gbg-cursor-relativo-present/.cursor/hooks.json" <<'EOF'
{"version":1,"hooks":{"beforeShellExecution":[{"command":"scripts/trackfw-git-branch-guard.sh"}]}}
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$CG_TMP/gbg-cursor-relativo-present/scripts/trackfw-git-branch-guard.sh"
chmod +x "$CG_TMP/gbg-cursor-relativo-present/scripts/trackfw-git-branch-guard.sh"

run_cg "$CG_TMP/gbg-claude-relativo-go.json"         "$CG_TMP/gbg-claude-relativo"
run_cg "$CG_TMP/gbg-cursor-relativo-present-go.json"  "$CG_TMP/gbg-cursor-relativo-present"

python3 - "$CG_TMP" "$GBG_RULE" <<'PY'
import json, os, sys
tmp, GBG_RULE = sys.argv[1], sys.argv[2]

def load_gbg(name):
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [item for item in all_items if item.get("rule") == GBG_RULE]
    return sorted(item["message"] for item in matching)

# PIN21: gbg-claude-relativo — "with a bare relative path".
msgs = load_gbg("gbg-claude-relativo-go.json")
if not msgs:
    raise SystemExit(
        "PIN21 vacuity: gbg-claude-relativo: expected git_branch_guard_hook_resolvable "
        "violation, none found — fixture broken or rule regressed"
    )
if not all("with a bare relative path" in m for m in msgs):
    raise SystemExit(
        f"PIN21: gbg-claude-relativo: message lacks 'with a bare relative path': {msgs!r}"
    )
print("OK [validate-rule-pins/pin21-gbg-relativo]")

# PIN22: gbg-cursor-relativo-present — silent (Cursor relative is the correct form).
msgs = load_gbg("gbg-cursor-relativo-present-go.json")
if msgs:
    raise SystemExit(
        f"PIN22: gbg-cursor-relativo-present: no violation expected, got: {msgs!r}"
    )
print("OK [validate-rule-pins/pin22-gbg-cursor-silent]")

# PIN23-25: Reuse cg-* outputs — the corrupted file blinds BOTH rules.
for fixture, marker, pin in [
    ("cg-claude-invalid-json-go.json", "is not valid JSON",  "pin23-gbg-invalid-json"),
    ("cg-claude-unreadable-go.json",   "could not be read",  "pin24-gbg-unreadable"),
    ("cg-claude-utf16-go.json",        "is not valid UTF-8", "pin25-gbg-utf16"),
]:
    msgs = load_gbg(fixture)
    if not msgs:
        raise SystemExit(
            f"[{pin}] vacuity: {fixture}: expected {GBG_RULE!r} violation, none — "
            "fixture broken or rule regressed"
        )
    if not all(marker in m for m in msgs):
        raise SystemExit(
            f"[{pin}]: {fixture}: message lacks {marker!r}: {msgs!r}"
        )
    print(f"OK [validate-rule-pins/{pin}]  {marker!r}")
PY

echo "validate-rule-pins: all 25 pins pass"
echo "  Block 1 (rule-set):          pin1"
echo "  Block 2 (bhr-messages):      pin2-pin5"
echo "  Block 3 (credential-guard):  pin6-pin20"
echo "  Block 4 (git-branch-guard):  pin21-pin25"
