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
# Pin inventory (34 pins across 6 blocks):
#   Block 1 — ADR/REQ rule-set:         {adr_accepted_when_req_done, blocked_by_draft_adr}
#   Block 2 — branch_has_wip_roadmap:    nomatch/diff message markers + --agent guidance
#                                        + the ML-3A matcher: token overlap accepts (PIN6),
#                                        empty slug refuses (PIN7), written link governs (PIN8)
#   Block 3 — credential_guard:          10 message pins + 5 silence pins
#   Block 4 — git_branch_guard:          1 message pin + 1 silence pin + 3 shared-fixture pins
#   Block 5 — roadmap_unterminated_fence: open fence → violation with line (PIN26),
#                                         closed fence → no violation (PIN27),
#                                         done/ state covered (PIN28 / PIN26 vacuity check)
#   Block 6 — adr_file_without_prefix:   non-prefixed .md with status → warning (PIN28),
#                                         ADR-prefixed .md with status → no warning (PIN29)
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

# Discriminant: GOOS of the binary under test (not the calling shell).
# go version -m reads the build metadata embedded by the Go toolchain — immune
# to the binary filename and to the GOOS environment variable of the shell.
# The || BIN_GOOS="" absorbs pipefail in case go version -m exits nonzero.
BIN_GOOS=$(go version -m "$GO_BIN" 2>/dev/null \
  | awk '$1=="build" && $2 ~ /^GOOS=/{sub(/GOOS=/,"",$2); print $2}') || BIN_GOOS=""
if [[ -z "$BIN_GOOS" ]]; then
  echo "[pin7] ABORT: go version -m '$GO_BIN' returned empty GOOS — toolchain absent or GO_BIN is not a Go binary" >&2
  exit 1
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
#   PIN3 — empty wip/, blocked/ and done/ → "no roadmap is in wip/, blocked/ nor done/"
#   PIN4 — done/ with different slug → "no matching roadmap in wip/, blocked/ nor done/"
#   PIN5 — by_agent + 2 agents + no roadmap → message contains "--agent"
#
# ML-3A (ADR-2026-09-26, REQ-2026-09-09) added three pins to this block. They are an UPDATE of the
# behaviour this gate fixes, not a loosening: each one REPROVES a mutation of the matcher.
#   PIN2B — token overlap accepts the #273 pair (slug is NOT a substring of the roadmap name).
#          Falsified by raising branchRoadmapMinSharedTokens to 3 or deleting the token arm.
#   PIN2C — an EMPTY branch slug matches NOTHING (strings.Contains(x,"") used to accept any corpus).
#          Falsified by removing the empty-slug carve-out.
#   PIN2D — the WRITTEN link (.trackfw-branch-links.json) governs a branch that inference rejects.
#          Falsified by ignoring the link file.
# ---------------------------------------------------------------------------
mkdir -p \
  "$TMP_DIR/bhr-match/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-nomatch/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-diff/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-byagent/docs/roadmaps/zeus"/{wip,done} \
  "$TMP_DIR/bhr-byagent/docs/roadmaps/apolo"/{wip,done} \
  "$TMP_DIR/bhr-tokens/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-empty/docs/roadmaps"/{wip,done} \
  "$TMP_DIR/bhr-link/docs/roadmaps"/{wip,done}

for d in bhr-match bhr-nomatch bhr-diff bhr-tokens bhr-empty bhr-link; do
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

# PIN2B fixture — the real #273 pair. The branch names THE WORK, the roadmap names THE REQ TITLE,
# so neither is a substring of the other; they share exactly two content tokens (divida, acervo).
cat >"$TMP_DIR/bhr-tokens/docs/roadmaps/wip/ROADMAP-2026-09-05-divida-de-governanca-do-acervo-que-nasceu-sem-adr.md" <<'EOF'
---
status: wip
---
# Roadmap: divida de governanca do acervo
EOF

# PIN2C fixture — a corpus with one roadmap, exercised with an EMPTY branch slug ("feat/").
cat >"$TMP_DIR/bhr-empty/docs/roadmaps/wip/ROADMAP-2026-08-20-qualquer-coisa.md" <<'EOF'
---
status: wip
---
# Roadmap: qualquer coisa
EOF

# PIN2D fixture — the branch slug shares NOTHING with the roadmap name, so inference must reject it;
# the written link recorded by `trackfw branch new` is what governs it.
cat >"$TMP_DIR/bhr-link/docs/roadmaps/wip/ROADMAP-2026-09-26-tema-totalmente-diferente.md" <<'EOF'
---
status: wip
---
# Roadmap: tema totalmente diferente
EOF
cat >"$TMP_DIR/bhr-link/docs/roadmaps/.trackfw-branch-links.json" <<'EOF'
{
  "version": 1,
  "links": {
    "feat/nada-em-comum": "ROADMAP-2026-09-26-tema-totalmente-diferente.md"
  }
}
EOF

run_bhr "$TMP_DIR/bhr-match-go.json"   "$TMP_DIR/bhr-match"   feat/minha-feature
run_bhr "$TMP_DIR/bhr-nomatch-go.json" "$TMP_DIR/bhr-nomatch" feat/sem-roadmap-nenhum
run_bhr "$TMP_DIR/bhr-diff-go.json"    "$TMP_DIR/bhr-diff"    feat/minha-feature
run_bhr "$TMP_DIR/bhr-byagent-go.json" "$TMP_DIR/bhr-byagent" feat/sem-roadmap-byagent
run_bhr "$TMP_DIR/bhr-tokens-go.json"  "$TMP_DIR/bhr-tokens"  feat/adrs-retroativas-da-divida-do-acervo
run_bhr "$TMP_DIR/bhr-empty-go.json"   "$TMP_DIR/bhr-empty"   "feat/"
run_bhr "$TMP_DIR/bhr-link-go.json"    "$TMP_DIR/bhr-link"    feat/nada-em-comum
run_bhr "$TMP_DIR/bhr-link-noinfer-go.json" "$TMP_DIR/bhr-tokens" feat/nada-em-comum

python3 - "$TMP_DIR" <<'PY'
import json, os, sys
tmp = sys.argv[1]
BHR_MARKER = "wip/, blocked/ nor done/"

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

# PIN3: empty wip/, blocked/ and done/ — message contains "no roadmap is in wip/, blocked/ nor done/".
rc, msgs = load_bhr("bhr-nomatch-go.json")
if not msgs:
    raise SystemExit(
        f"PIN3 vacuity: bhr-nomatch: expected branch_has_wip_roadmap violation, "
        f"none found (rc={rc}) — fixture broken or rule regressed"
    )
MARKER_NOMATCH = "no roadmap is in wip/, blocked/ nor done/"
if not all(MARKER_NOMATCH in m for m in msgs):
    raise SystemExit(
        f"PIN3: bhr-nomatch: message lacks {MARKER_NOMATCH!r}: {msgs!r}"
    )
print("OK [validate-rule-pins/pin3-bhr-nomatch-message]")

# PIN4: done/ with different slug — message contains "no matching roadmap in wip/, blocked/ nor done/".
rc, msgs = load_bhr("bhr-diff-go.json")
if not msgs:
    raise SystemExit(
        f"PIN4 vacuity: bhr-diff: expected branch_has_wip_roadmap violation, "
        f"none found (rc={rc}) — fixture broken or rule regressed"
    )
MARKER_DIFF = "no matching roadmap in wip/, blocked/ nor done/"
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

# PIN2B (ML-3A / AC15): token overlap accepts the #273 pair. The slug
# "adrs-retroativas-da-divida-do-acervo" is NOT a substring of
# "ROADMAP-2026-09-05-divida-de-governanca-do-acervo-que-nasceu-sem-adr.md", so the ONLY relation
# that can accept it is the token-overlap arm. Raising the threshold to 3 or deleting the arm makes
# this pin fail.
rc, msgs = load_bhr("bhr-tokens-go.json")
if msgs:
    raise SystemExit(
        f"PIN2B: bhr-tokens: a legitimately governed branch whose slug is NOT a substring of the "
        f"roadmap filename must be accepted via token overlap (AC15 of REQ-2026-09-09, issue #273), "
        f"but got: {msgs!r}"
    )
print("OK [validate-rule-pins/pin2b-token-overlap-accepted]")

# PIN2C (ML-3A / AC12): an EMPTY branch slug matches nothing. strings.Contains(x, "") is always true,
# so before ML-3A the branch "feat/" was reported as governed by ANY corpus.
rc, msgs = load_bhr("bhr-empty-go.json")
if not msgs:
    raise SystemExit(
        f"PIN2C: bhr-empty: an empty branch slug must match NO roadmap — Contains(x, \"\") is always "
        f"true and used to accept any corpus vacuously (rc={rc})"
    )
MARKER_EMPTY = "no matching roadmap in wip/, blocked/ nor done/"
if not all(MARKER_EMPTY in m for m in msgs):
    raise SystemExit(f"PIN2C: bhr-empty: message lacks {MARKER_EMPTY!r}: {msgs!r}")
print("OK [validate-rule-pins/pin2c-empty-slug-refused]")

# PIN2D (ML-3A / AC12, D1): the WRITTEN link governs a branch that inference rejects.
# Contra-arm in the same pin: the SAME branch name against a corpus with no link recorded must be
# rejected — otherwise PIN2D would pass for a reason other than the link.
rc, msgs = load_bhr("bhr-link-go.json")
if msgs:
    raise SystemExit(
        f"PIN2D: bhr-link: the written branch↔roadmap link must govern the branch (D1 of "
        f"ADR-2026-09-26), but got: {msgs!r}"
    )
rc, msgs = load_bhr("bhr-link-noinfer-go.json")
if not msgs:
    raise SystemExit(
        f"PIN2D vacuity: bhr-link-noinfer: the same branch name WITHOUT a recorded link must be "
        f"rejected — otherwise PIN8 does not prove the link is what accepted it (rc={rc})"
    )
print("OK [validate-rule-pins/pin2d-written-link-governs]")
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
           cg-cursor-present cg-copilot-relativo-present \
           cg-claude-guard-subcmd; do
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
# 🔴 O caminho vai como ARGUMENTO, nunca interpolado dentro do programa.
# No MSYS/Git-bash o argumento que parece caminho POSIX e convertido para caminho do
# Windows antes de chegar ao python; o texto dentro de `-c` NAO e. O python do Windows
# nao resolve /tmp, e o open() morre com FileNotFoundError num diretorio que o mkdir
# acabou de criar. Medido no Windows, issue #363.
python3 - "$CG_TMP/cg-claude-pwd/.claude/settings.json" <<'PY'
import json, sys
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '$PWD/scripts/trackfw-credential-guard.sh'}]}]}}
with open(sys.argv[1], 'w') as f:
    json.dump(d, f)
PY

# cg-claude-absoluto: absolute path (class 1) → silent.
mkdir -p "$CG_TMP/cg-claude-absoluto/.claude"
# 🔴 O caminho vai como ARGUMENTO, nunca interpolado dentro do programa.
# No MSYS/Git-bash o argumento que parece caminho POSIX e convertido para caminho do
# Windows antes de chegar ao python; o texto dentro de `-c` NAO e. O python do Windows
# nao resolve /tmp, e o open() morre com FileNotFoundError num diretorio que o mkdir
# acabou de criar. Medido no Windows, issue #363.
python3 - "$CG_TMP/cg-claude-absoluto/.claude/settings.json" <<'PY'
import json, sys
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '/opt/trackfw/scripts/trackfw-credential-guard.sh'}]}]}}
with open(sys.argv[1], 'w') as f:
    json.dump(d, f)
PY

# cg-claude-windows-drive: Windows drive letter (class 1 by union) → silent.
mkdir -p "$CG_TMP/cg-claude-windows-drive/.claude"
# 🔴 O caminho vai como ARGUMENTO, nunca interpolado dentro do programa.
# No MSYS/Git-bash o argumento que parece caminho POSIX e convertido para caminho do
# Windows antes de chegar ao python; o texto dentro de `-c` NAO e. O python do Windows
# nao resolve /tmp, e o open() morre com FileNotFoundError num diretorio que o mkdir
# acabou de criar. Medido no Windows, issue #363.
python3 - "$CG_TMP/cg-claude-windows-drive/.claude/settings.json" <<'PY'
import json, sys
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': r'C:\Users\kg\scripts\trackfw-credential-guard.sh'}]}]}}
with open(sys.argv[1], 'w') as f:
    json.dump(d, f)
PY

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
# 🔴 O caminho vai como ARGUMENTO, nunca interpolado dentro do programa.
# No MSYS/Git-bash o argumento que parece caminho POSIX e convertido para caminho do
# Windows antes de chegar ao python; o texto dentro de `-c` NAO e. O python do Windows
# nao resolve /tmp, e o open() morre com FileNotFoundError num diretorio que o mkdir
# acabou de criar. Medido no Windows, issue #363.
python3 - "$CG_TMP/cg-claude-tilde-quoted/.claude/settings.json" <<'PY'
import json, sys
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '"' + r'~/scripts/trackfw-credential-guard.sh' + '"'}]}]}}
with open(sys.argv[1], 'w') as f:
    json.dump(d, f)
PY

# cg-claude-tilde-user: ~alice/... (class 2) → "with a named-user tilde path".
mkdir -p "$CG_TMP/cg-claude-tilde-user/.claude"
# 🔴 O caminho vai como ARGUMENTO, nunca interpolado dentro do programa.
# No MSYS/Git-bash o argumento que parece caminho POSIX e convertido para caminho do
# Windows antes de chegar ao python; o texto dentro de `-c` NAO e. O python do Windows
# nao resolve /tmp, e o open() morre com FileNotFoundError num diretorio que o mkdir
# acabou de criar. Medido no Windows, issue #363.
python3 - "$CG_TMP/cg-claude-tilde-user/.claude/settings.json" <<'PY'
import json, sys
d = {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': '~alice/scripts/trackfw-credential-guard.sh'}]}]}}
with open(sys.argv[1], 'w') as f:
    json.dump(d, f)
PY

# cg-claude-guard-subcmd: new "trackfw guard credential" subcommand form → no Windows migration warning.
mkdir -p "$CG_TMP/cg-claude-guard-subcmd/.claude"
cat >"$CG_TMP/cg-claude-guard-subcmd/.claude/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"trackfw guard credential; exit $LASTEXITCODE"}]}]}}
EOF

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
           cg-cursor-present cg-copilot-relativo-present \
           cg-claude-guard-subcmd; do
  run_cg "$CG_TMP/$fix-go.json" "$CG_TMP/$fix"
done

python3 - "$CG_TMP" "$BIN_GOOS" <<'PY'
import json, os, sys
tmp = sys.argv[1]
BIN_GOOS = sys.argv[2]
CG_RULE = "credential_guard_hook_resolvable"
CG_WINDOWS_SH_MARKER = "does not execute on Windows outside Git Bash"

def load_cg(name):
    """Returns (rc, violation_messages) for CG_RULE — violations only, not warnings.
    Used by expect_violation and expect_silent loops: violations drive pass/fail,
    warnings (e.g. the Windows migration hint) are not mixed in."""
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    with open(path + ".exit") as f:
        rc = int(f.read().strip())
    matching = [item for item in payload.get("violations", []) if item.get("rule") == CG_RULE]
    return rc, sorted(item["message"] for item in matching)

def load_cg_any(name):
    """Returns (rc, combined violation+warning messages) for CG_RULE.
    Used where silence means neither a violation nor a warning from this rule
    (e.g. Windows pin7-noexec-guarded branch)."""
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    with open(path + ".exit") as f:
        rc = int(f.read().strip())
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [item for item in all_items if item.get("rule") == CG_RULE]
    return rc, sorted(item["message"] for item in matching)

def load_cg_non_legacy(name):
    """Returns (rc, messages) for CG_RULE — violations PLUS warnings that are NOT
    the legacy Windows migration hint. Preserves the original silence-pin semantics
    (no real issue allowed) while ignoring the always-on legacy advisory that fires
    for any .sh reference regardless of GOOS. Used by expect_silent and Windows pin7
    arm (b)."""
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    with open(path + ".exit") as f:
        rc = int(f.read().strip())
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [
        item for item in all_items
        if item.get("rule") == CG_RULE
        and CG_WINDOWS_SH_MARKER not in item.get("message", "")
    ]
    return rc, sorted(item["message"] for item in matching)

def load_cg_warnings(name):
    """Returns warning_messages for CG_RULE only (no violations)."""
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    matching = [item for item in payload.get("warnings", []) if item.get("rule") == CG_RULE]
    return sorted(item["message"] for item in matching)

# PIN6-PIN15: expect violations with specific message markers.
# Each tuple: (fixture-go-json, pin-label, message-marker)
# pin7-noexec is removed from this list on Windows and handled separately below.
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
# On Windows the rule declines exec-bit checks by design; pin7 is affirmed separately.
if BIN_GOOS == "windows":
    expect_violation = [t for t in expect_violation if t[1] != "pin7-noexec"]

for name, label, marker in expect_violation:
    rc, msgs = load_cg(name)
    if not msgs:
        raise SystemExit(
            f"[{label}] vacuity: {name}: expected {CG_RULE!r} violation, "
            f"none found (rc={rc}) — fixture broken, rule regressed, "
            f"or the rule is guarded on this platform — see the platform-guarded pins"
        )
    if not all(marker in m for m in msgs):
        raise SystemExit(
            f"[{label}]: {name}: message does not contain {marker!r}: {msgs!r}"
        )
    print(f"OK [validate-rule-pins/{label}]  {marker!r}")

# PIN7 — platform-guarded branch.
# On Windows, credential_guard_hook_resolvable declines exec-bit checks by design
# (internal/validator/goos.go). Affirm the guarded behavior: zero violations for noexec.
if BIN_GOOS == "windows":
    # (c) Anti-vacuity: confirm that the absent fixture (pin6) produced at least one
    # violation of this rule in its violations list. This proves the rule is live and
    # that the JSON output schema is valid before asserting silence for noexec.
    absent_path = os.path.join(tmp, "cg-claude-absent-go.json")
    with open(absent_path, encoding="utf-8") as f:
        absent_payload = json.load(f)
    absent_violations = [v for v in absent_payload.get("violations", [])
                         if v.get("rule") == CG_RULE]
    if not absent_violations:
        raise SystemExit(
            f"[pin7-noexec-windows-guarded] anti-vacuity: pin6 (absent) produced no violations "
            f"of {CG_RULE!r} in the violations list — rule may be inoperative"
        )

    # (a) noexec fixture: rc==0 and JSON has 'violations' as list.
    noexec_path = os.path.join(tmp, "cg-claude-noexec-go.json")
    with open(noexec_path, encoding="utf-8") as f:
        noexec_payload = json.load(f)
    if not isinstance(noexec_payload.get("violations"), list):
        raise SystemExit(
            f"[pin7-noexec-windows-guarded] JSON schema: expected 'violations' as list, "
            f"got: {noexec_payload!r}"
        )
    with open(noexec_path + ".exit") as f:
        noexec_rc = int(f.read().strip())
    if noexec_rc != 0:
        raise SystemExit(
            f"[pin7-noexec-windows-guarded] expected rc=0 for noexec fixture on windows, "
            f"got rc={noexec_rc}"
        )

    # (b) No violation or non-legacy warning from this rule for the noexec fixture.
    # The .sh form emits the Windows migration advisory regardless of GOOS; that legacy
    # warning is not about exec-bit and must not mask the guarded-silence assertion.
    _, noexec_msgs = load_cg_non_legacy("cg-claude-noexec-go.json")
    if noexec_msgs:
        raise SystemExit(
            f"[pin7-noexec-windows-guarded] expected no non-legacy {CG_RULE!r} violations/warnings "
            f"on windows for noexec fixture, but got: {noexec_msgs!r}"
        )

    print("OK [validate-rule-pins/pin7-noexec-windows-guarded]"
          "  rule declines exec-bit check on windows by design (internal/validator/goos.go)")

# PIN16-PIN20: expect silence — no violation AND no non-legacy warning from this rule.
# The legacy Windows migration advisory (CG_WINDOWS_SH_MARKER) is tested separately
# by pin20b/pin20c and is excluded here so the .sh-form presence check doesn't mask
# real silent-path violations (e.g., pin17 absoluto, pin18 windows-drive).
expect_silent = [
    ("cg-claude-present-go.json",          "pin16-present-silent"),
    ("cg-claude-absoluto-go.json",          "pin17-absoluto-silent"),
    ("cg-claude-windows-drive-go.json",     "pin18-windows-drive-silent"),
    ("cg-cursor-present-go.json",           "pin19-cursor-present-silent"),
    ("cg-copilot-relativo-present-go.json", "pin20-copilot-relativo-silent"),
]

for name, label in expect_silent:
    rc, msgs = load_cg_non_legacy(name)
    if msgs:
        raise SystemExit(
            f"[{label}]: {name}: no non-legacy {CG_RULE!r} violation/warning expected, "
            f"but got: {msgs!r}"
        )
    print(f"OK [validate-rule-pins/{label}]")

# PIN20B (ML-4C corretivo): Windows migration warning — .sh form emits it, subcommand form does not.
# The validator emits a CG_RULE warning when the command references the legacy .sh script,
# regardless of whether the script exists, to prompt migration to `trackfw guard credential`.
# The subcommand form must NOT trigger this warning.
warn_sh = load_cg_warnings("cg-claude-absent-go.json")
if not any(CG_WINDOWS_SH_MARKER in w for w in warn_sh):
    raise SystemExit(
        f"[pin20b-windows-sh-warning] vacuity: cg-claude-absent: expected {CG_RULE!r} warning "
        f"containing {CG_WINDOWS_SH_MARKER!r}, got warnings: {warn_sh!r}"
    )
print(f"OK [validate-rule-pins/pin20b-windows-sh-warning]  {CG_WINDOWS_SH_MARKER!r}")

# PIN20C (ML-4C corretivo, contra-braço): "trackfw guard credential; exit $LASTEXITCODE"
# must NOT emit the Windows migration warning — it IS the migration target.
warn_subcmd = load_cg_warnings("cg-claude-guard-subcmd-go.json")
if any(CG_WINDOWS_SH_MARKER in w for w in warn_subcmd):
    raise SystemExit(
        f"[pin20c-guard-subcmd-no-warning]: cg-claude-guard-subcmd must NOT emit the Windows "
        f"migration warning, but got: {warn_subcmd!r}"
    )
print(f"OK [validate-rule-pins/pin20c-guard-subcmd-no-warning]  no {CG_WINDOWS_SH_MARKER!r}")
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
GBG_WINDOWS_SH_MARKER = "does not execute on Windows outside Git Bash"

def load_gbg(name):
    """Returns violation_messages for GBG_RULE only (not warnings).
    The Windows migration warning fires for any .sh reference and must not be
    mixed into violation marker checks."""
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    matching = [item for item in payload.get("violations", []) if item.get("rule") == GBG_RULE]
    return sorted(item["message"] for item in matching)

def load_gbg_non_legacy(name):
    """Returns messages for GBG_RULE — violations PLUS warnings that are NOT the
    legacy Windows migration hint. Used by pin22 to preserve the original silence
    semantics (no real issue from this rule)."""
    path = os.path.join(tmp, name)
    with open(path, encoding="utf-8") as f:
        payload = json.load(f)
    all_items = payload.get("violations", []) + payload.get("warnings", [])
    matching = [
        item for item in all_items
        if item.get("rule") == GBG_RULE
        and GBG_WINDOWS_SH_MARKER not in item.get("message", "")
    ]
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
# Uses load_gbg_non_legacy: the Cursor fixture uses the .sh form, which emits the
# Windows migration advisory; that legacy warning is pinned by the CG block (pin20b)
# and is excluded here so a real non-legacy issue would still be caught.
msgs = load_gbg_non_legacy("gbg-cursor-relativo-present-go.json")
if msgs:
    raise SystemExit(
        f"PIN22: gbg-cursor-relativo-present: no non-legacy {GBG_RULE!r} violation/warning "
        f"expected, got: {msgs!r}"
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

# ---------------------------------------------------------------------------
# BLOCK 5: roadmap_unterminated_fence
#
# Pins:
#   PIN26 — open fence in wip/ → violation naming file and line number
#   PIN27 — closed fence in wip/ → no violation
#   done/ state is covered by the same fixture as PIN26 (vacuity checked by PIN26 setup)
# ---------------------------------------------------------------------------
FENCE_TMP=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-fence-pins.XXXXXX")
trap 'rm -rf "$FENCE_TMP"' EXIT

# PIN26 fixture: roadmap with an unterminated fence starting at line 5.
mkdir -p "$FENCE_TMP/p26/docs/roadmaps/wip" "$FENCE_TMP/p26/docs/roadmaps/done"
cat >"$FENCE_TMP/p26/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
# Lines 1-4: frontmatter + blank; line 5: ```bash (opens fence, never closed)
printf -- '---\nstatus: wip\n---\n\n```bash\necho hello\n' \
  >"$FENCE_TMP/p26/docs/roadmaps/wip/ROADMAP-open.md"
# Also test done/ to prove universal state coverage.
printf -- '---\nstatus: done\n---\n\n```bash\necho hello\n' \
  >"$FENCE_TMP/p26/docs/roadmaps/done/ROADMAP-done-open.md"

P26_JSON="$FENCE_TMP/p26.json"
P26_RC=0
( cd "$FENCE_TMP/p26" && "$GO_BIN" validate --json ) >"$P26_JSON" 2>"$FENCE_TMP/p26.stderr" || P26_RC=$?

# PIN27 fixture: roadmap with all fences closed → no violation.
mkdir -p "$FENCE_TMP/p27/docs/roadmaps/wip"
cat >"$FENCE_TMP/p27/trackfw.yaml" <<'EOF'
roadmap_dir: docs/roadmaps
EOF
printf -- '---\nstatus: wip\n---\n\n```bash\necho hello\n```\n' \
  >"$FENCE_TMP/p27/docs/roadmaps/wip/ROADMAP-closed.md"

P27_JSON="$FENCE_TMP/p27.json"
P27_RC=0
( cd "$FENCE_TMP/p27" && "$GO_BIN" validate --json ) >"$P27_JSON" 2>"$FENCE_TMP/p27.stderr" || P27_RC=$?

FENCE_RULE="roadmap_unterminated_fence"

python3 - "$P26_JSON" "$P26_RC" "$P27_JSON" "$P27_RC" "$FENCE_RULE" <<'PY'
import json, sys
p26_path, p26_rc, p27_path, p27_rc, rule = \
    sys.argv[1], int(sys.argv[2]), sys.argv[3], int(sys.argv[4]), sys.argv[5]

# PIN26: open fence → violation with file name and line number.
with open(p26_path, encoding="utf-8") as f:
    p26 = json.load(f)
all_msgs = [v.get("message","") for v in p26.get("violations",[])] + \
           [w.get("message","") for w in p26.get("warnings",[])]
fence_msgs = [m for m in all_msgs if rule in p26.get("violations",[{}])[0].get("rule","") or
              any(r.get("rule") == rule for r in p26.get("violations",[]))]
# Re-collect by rule
fence_violations = [v for v in p26.get("violations",[]) if v.get("rule") == rule]
fence_warnings   = [w for w in p26.get("warnings",  []) if w.get("rule") == rule]
fence_findings   = fence_violations + fence_warnings

if not fence_findings:
    raise SystemExit(
        f"PIN26 vacuity: expected {rule!r} finding for open fence, got none — "
        f"rule regressed or fixture broken. rc={p26_rc}, "
        f"violations={[v.get('rule') for v in p26.get('violations',[])]}"
    )
msgs_text = " | ".join(f.get("message","") for f in fence_findings)
if "ROADMAP-open.md" not in msgs_text and "ROADMAP-done-open.md" not in msgs_text:
    raise SystemExit(
        f"PIN26: violation message must name the offending file: {msgs_text!r}"
    )
if "unterminated code fence starting at line 5" not in msgs_text:
    raise SystemExit(
        f"PIN26: violation message must include line number: {msgs_text!r}"
    )
print(f"OK [validate-rule-pins/pin26-unterminated-fence-violation]  {msgs_text[:80]!r}")

# PIN27: closed fence → no violation from this rule.
with open(p27_path, encoding="utf-8") as f:
    p27 = json.load(f)
fence_p27 = [v for v in p27.get("violations",[]) if v.get("rule") == rule] + \
            [w for w in p27.get("warnings",  []) if w.get("rule") == rule]
if fence_p27:
    raise SystemExit(
        f"PIN27: no {rule!r} finding expected for closed fence, got: {fence_p27!r}"
    )
print(f"OK [validate-rule-pins/pin27-closed-fence-silent]")
PY

# ---------------------------------------------------------------------------
# BLOCK 6: adr_file_without_prefix message pins
#
# PIN28 — non-prefixed .md with frontmatter status: → warning with rule name
# PIN29 — ADR-prefixed .md with status: → no warning (silence)
# ---------------------------------------------------------------------------
mkdir -p \
  "$TMP_DIR/p6/docs/adr" \
  "$TMP_DIR/p6/docs/req" \
  "$TMP_DIR/p6/docs/roadmaps"/{backlog,wip,blocked,done,abandoned}

cat >"$TMP_DIR/p6/trackfw.yaml" <<'EOF'
governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
EOF

# Non-prefixed .md with status: Draft — should trigger warning
cat >"$TMP_DIR/p6/docs/adr/decisao.md" <<'EOF'
---
status: Draft
date: 2026-10-02
author: ""
---

# Decisão: sem prefixo

> Date: 2026-10-02 | Status: Draft

## Context
ctx
EOF

# ADR-prefixed .md with status: Draft — must NOT trigger the rule
cat >"$TMP_DIR/p6/docs/adr/ADR-2026-10-02-prefixed.md" <<'EOF'
---
status: Draft
date: 2026-10-02
author: ""
---

# ADR: com prefixo

> Date: 2026-10-02 | Status: Draft

## Context
ctx

## Decision
d
EOF

set +e
(cd "$TMP_DIR/p6" && "$GO_BIN" validate --json) >"$TMP_DIR/p6.json" 2>"$TMP_DIR/p6.stderr"
P6_RC=$?
set -e

python3 - "$TMP_DIR/p6.json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    payload = json.load(f)
warnings = payload.get("warnings", [])
rule = "adr_file_without_prefix"

# PIN28: non-prefixed decisao.md with status: must produce warning
found_decisao = any(
    (item.get("rule") == rule or item.get("message", "").startswith(rule + ":"))
    and "decisao.md" in item.get("message", "")
    for item in warnings
)
if not found_decisao:
    raise SystemExit(
        f"PIN28: {rule!r} warning not found for decisao.md — "
        f"rule not firing or message format changed. warnings: {warnings!r}"
    )
print(f"OK [validate-rule-pins/pin28-adr-without-prefix-fires]")

# PIN29: ADR-prefixed file must NOT appear in the rule's warnings
found_prefixed = any(
    (item.get("rule") == rule or item.get("message", "").startswith(rule + ":"))
    and "ADR-2026-10-02-prefixed.md" in item.get("message", "")
    for item in warnings
)
if found_prefixed:
    raise SystemExit(
        f"PIN29: {rule!r} fired for ADR-prefixed file — rule must not flag files with ADR- prefix"
    )
print(f"OK [validate-rule-pins/pin29-adr-prefixed-silent]")
PY

echo "validate-rule-pins: all 34 pins pass"
echo "  Block 1 (rule-set):          pin1"
echo "  Block 2 (bhr-messages):      pin2-pin5 + pin2b/pin2c/pin2d (ML-3A matcher)"
echo "  Block 3 (credential-guard):  pin6-pin20 + pin20b/pin20c (ML-4C Windows migration warning)"
echo "  Block 4 (git-branch-guard):  pin21-pin25"
echo "  Block 5 (unterminated-fence): pin26-pin27"
echo "  Block 6 (adr-without-prefix): pin28-pin29"
