#!/usr/bin/env bash
# check-update-parity.sh — behavioral pin for `trackfw update` and
# `trackfw update harness` on the Go binary.
#
# ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
# removed. This gate now asserts Go binary behavior alone — exit codes, JSON
# shape, field values, dry-run semantics, skip warnings, and sandbox contract.
#
# Original ML-6G (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-
# orquestrador). Cross-runtime comparison removed by ML-3A (v8) because
# Node.js and Python CLIs no longer exist (npm/src/ and pypi/trackfw/ deleted).
#
# Method: runs `trackfw update` and `trackfw update harness` in isolated
# temporary environments, asserts behavioral contracts per scenario.
# Follows the conventions of check-barrier.sh:
# set -euo pipefail, mktemp -d fixtures with a cleanup trap, a vacuity guard
# before comparing, "OK [scenario/name]" on success, accumulating all
# failures before exiting.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

export NO_COLOR=1
export TERM=dumb

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
GO_BIN=${GO_BIN:-"$ROOT_DIR/bin/trackfw"}
if [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$(pwd)/$GO_BIN"
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-update-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-update-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

FAIL=0

ok() { echo "OK   [$1]"; }
diag() {
  echo "FAIL [$1]: $2" >&2
  FAIL=1
}

# ---------------------------------------------------------------------------
# run_update HOME_DIR PROJECT_DIR ARGS...
# Sets UPDATE_EXIT, UPDATE_STDOUT, UPDATE_STDERR as globals.
# ---------------------------------------------------------------------------
run_update() {
  local home_dir=$1 project_dir=$2
  shift 2
  local out_file="$WORK/out.$$.$RANDOM" err_file="$WORK/err.$$.$RANDOM"
  set +e
  (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" update "$@") >"$out_file" 2>"$err_file"
  UPDATE_EXIT=$?
  set -e
  UPDATE_STDOUT=$(cat "$out_file")
  UPDATE_STDERR=$(cat "$err_file")
  rm -f "$out_file" "$err_file"
}

# normalize_update_json — reparses stdin preserving key order and target
# order (object_pairs_hook=OrderedDict, no sort_keys) and redumps with a
# fixed indent, so only real shape/order/content differences survive.
normalize_update_json() {
  python3 -c "
import json, sys
from collections import OrderedDict
d = json.loads(sys.stdin.read(), object_pairs_hook=OrderedDict)
json.dump(d, sys.stdout, indent=2, ensure_ascii=False)
"
}

# target_ids_json DOC — prints the JSON array of target ids, in declared order.
target_ids_json() {
  python3 -c "
import json, sys
from collections import OrderedDict
d = json.loads(sys.argv[1], object_pairs_hook=OrderedDict)
print(json.dumps([t['id'] for t in d['targets']]))
" "$1"
}

# snapshot_tree DIR — sha256 of every regular file under DIR, path-relative,
# sorted — used to prove --dry-run performs zero writes.
snapshot_tree() {
  local dir=$1
  if [[ -d "$dir" ]]; then
    (cd "$dir" && find . -type f -print0 | sort -z | xargs -0 shasum -a 256 2>/dev/null) || true
  fi
}

# install_claude_agents HOME_DIR — installs the claude agents target into an
# isolated HOME. Runs from a throwaway scratch cwd under $WORK (removed by
# the top-level trap), never the caller's cwd: `agents install --scope
# global` writes CLAUDE.md into the *current project* in addition to the
# redirected HOME, so without an isolated cwd this gate would mutate
# whichever repo invoked it.
install_claude_agents() {
  local home_dir=$1
  mkdir -p "$home_dir"
  local scratch_dir
  scratch_dir=$(mktemp -d "$WORK/agents-install-cwd.XXXXXX")
  (cd "$scratch_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets claude --scope global \
    --identity-preset neutral --json >/dev/null)
}

# install_agent_global HOME_DIR TARGET ITEM
install_agent_global() {
  local home_dir=$1 target=$2 item=$3
  local scratch
  scratch=$(mktemp -d "$WORK/install-global-cwd.XXXXXX")
  (cd "$scratch" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --items "$item" --scope global >/dev/null 2>&1)
}

# install_agent_project PROJECT_DIR HOME_DIR TARGET ITEM
install_agent_project() {
  local project_dir=$1 home_dir=$2 target=$3 item=$4
  (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --items "$item" --scope project >/dev/null 2>&1)
}

# run_agents_install HOME_DIR PROJECT_DIR TARGET ITEM SCOPE
# Sets AGENTS_INSTALL_EXIT, AGENTS_INSTALL_STDERR as globals.
run_agents_install() {
  local home_dir=$1 project_dir=$2 target=$3 item=$4 scope=$5
  local err_file="$WORK/err.$$.$RANDOM"
  set +e
  (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets "$target" --items "$item" --scope "$scope" >/dev/null 2>"$err_file")
  AGENTS_INSTALL_EXIT=$?
  set -e
  AGENTS_INSTALL_STDERR=$(cat "$err_file")
  rm -f "$err_file"
}

# run_init HOME_DIR PROJECT_DIR AI_TOOL
# Sets INIT_EXIT, INIT_STDERR as globals.
run_init() {
  local home_dir=$1 project_dir=$2 tool=$3
  local err_file="$WORK/err.$$.$RANDOM"
  set +e
  (cd "$project_dir" && HOME="$home_dir" "$GO_BIN" init --ai-tools "$tool" >/dev/null 2>"$err_file")
  INIT_EXIT=$?
  set -e
  INIT_STDERR=$(cat "$err_file")
  rm -f "$err_file"
}

# patch_manifest_outdated MANIFEST ARTIFACT SENTINEL
# Overwrites ARTIFACT with SENTINEL (+ newline), then patches MANIFEST so that
# the entry for ARTIFACT has sha256 = sha256(SENTINEL+newline) and an old
# catalog_version. This makes the artifact state outdated+owned.
patch_manifest_outdated() {
  local manifest=$1 artifact=$2 sentinel=$3
  printf '%s\n' "$sentinel" > "$artifact"
  python3 - "$manifest" "$artifact" "$sentinel" <<'PY'
import json, hashlib, sys
manifest_path, artifact_key, sentinel = sys.argv[1], sys.argv[2], sys.argv[3]
new_sha = hashlib.sha256((sentinel + "\n").encode()).hexdigest()
with open(manifest_path) as f:
    data = json.load(f)
if artifact_key not in data["artifacts"]:
    print(f"patch_manifest_outdated: key {artifact_key!r} not in manifest", file=sys.stderr)
    sys.exit(1)
data["artifacts"][artifact_key]["sha256"] = new_sha
data["artifacts"][artifact_key]["catalog_version"] = "simulated-old-0.9.0"
with open(manifest_path, "w") as f:
    json.dump(data, f, indent=2)
PY
}

SKIP_SENTINEL="OUTDATED-SENTINEL-DO-NOT-OVERWRITE-by-check-update-parity"

# ===========================================================================
# Scenario 1 — `update harness --json` on an empty harness.
# Must: exit 0, emit valid JSON, report all non-script targets as 'missing',
# report script targets as 'updated', summary counts match.
# ===========================================================================
S1_PROJECT="$WORK/s1-project"
mkdir -p "$S1_PROJECT" "$WORK/s1-home-go"

run_update "$WORK/s1-home-go" "$S1_PROJECT" harness --json
S1_GO_EXIT=$UPDATE_EXIT; S1_GO_OUT=$UPDATE_STDOUT

if [[ "$S1_GO_EXIT" != "0" ]]; then
  diag "update-harness/empty-harness/exit-zero" "go exited $S1_GO_EXIT on an empty harness (missing must never be an error)"
fi

# Vacuity guard: output must parse as JSON with non-empty targets
count=$(python3 -c "import json,sys; d=json.loads(sys.argv[1]); print(len(d.get('targets',[])))" "$S1_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
if [[ "$count" == "PARSE_ERROR" || "$count" == "0" ]]; then
  diag "update-harness/empty-harness/vacuity-guard" "go produced no parseable/non-empty targets array"
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "update-harness/empty-harness/vacuity-guard"

  # Every target must be `missing` EXCEPT the two script targets
  # ("git-branch-guard-script", "credential-guard-script") which always write
  # on first run without --install-missing (REQ-2026-09-09 ML-1A).
  python3 -c "
import json, sys
d = json.loads(sys.argv[1])
SCRIPT_TARGETS = {'git-branch-guard-script', 'credential-guard-script'}
n = len(d['targets'])
bad = [t['id'] for t in d['targets']
       if t['id'] not in SCRIPT_TARGETS and t['state'] != 'missing']
bad += [t['id'] for t in d['targets']
        if t['id'] in SCRIPT_TARGETS and t['state'] != 'updated']
s = d['summary']
n_scripts = len(SCRIPT_TARGETS)
expected_summary = {'updated': n_scripts, 'skipped': 0, 'missing': n - n_scripts, 'failed': 0}
if bad or s != expected_summary:
    print('go: bad states=%r summary=%r (expected=%r)' % (bad, s, expected_summary))
    sys.exit(1)
" "$S1_GO_OUT" || diag "update-harness/empty-harness/all-missing" "Go: not every target is 'missing' or summary miscounts"

  if [[ "$FAIL" -eq 0 ]]; then
    ok "update-harness/empty-harness/go-behavioral-pin"
  fi
fi

# ===========================================================================
# Scenario 2 — `update harness --json` on a populated harness.
# Must: exit 0, emit valid JSON, at least one non-missing target.
# ===========================================================================
S2_PROJECT="$WORK/s2-project"
mkdir -p "$S2_PROJECT"
install_claude_agents "$WORK/s2-home-go"

run_update "$WORK/s2-home-go" "$S2_PROJECT" harness --json
S2_GO_EXIT=$UPDATE_EXIT; S2_GO_OUT=$UPDATE_STDOUT

if [[ "$S2_GO_EXIT" != "0" ]]; then
  diag "update-harness/populated-harness/exit-zero" "go exited $S2_GO_EXIT on a harness with claude agents installed"
fi

# Vacuity guard: at least one target NOT missing
non_missing=$(python3 -c "
import json, sys
d = json.loads(sys.argv[1])
print(sum(1 for t in d['targets'] if t['state'] != 'missing'))
" "$S2_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
if [[ "$non_missing" == "PARSE_ERROR" || "$non_missing" == "0" ]]; then
  diag "update-harness/populated-harness/vacuity-guard" "go: no non-missing target found after installing claude agents"
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "update-harness/populated-harness/go-behavioral-pin"
fi

# ===========================================================================
# Scenario 3 — `update --json` (project scope).
# Must: exit 0, scope=="project".
# ===========================================================================
mkdir -p "$WORK/s3-home-go" "$WORK/s3-project-go"
(cd "$WORK/s3-project-go" && HOME="$WORK/s3-home-go" "$GO_BIN" init >/dev/null 2>&1)

run_update "$WORK/s3-home-go" "$WORK/s3-project-go" --json
S3_GO_EXIT=$UPDATE_EXIT; S3_GO_OUT=$UPDATE_STDOUT; S3_GO_ERR=$UPDATE_STDERR

if [[ "$S3_GO_EXIT" != "0" ]]; then
  diag "update-project/json/exit-zero" "go: 'trackfw update --json' exited $S3_GO_EXIT (contract requires --json on project update too, per ML-6A); stderr: $S3_GO_ERR"
fi

if [[ "$FAIL" -eq 0 ]]; then
  scope_go=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['scope'])" "$S3_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$scope_go" != "project" ]]; then
    diag "update-project/json/scope-field" "go: expected scope=\"project\", got \"$scope_go\""
  fi
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "update-project/json/go-behavioral-pin"
fi

# ===========================================================================
# Scenario 4 — `--dry-run` on `update harness`:
# (a) zero filesystem writes, (b) exit 0, (c) dry_run=true in JSON.
# ===========================================================================
install_claude_agents "$WORK/s4-home-go"
# Seed a deliberately stale legacy skill file so dry-run has a pending write.
mkdir -p "$WORK/s4-home-go/.claude/skills/trackfw"
echo "stale placeholder content — must never survive --dry-run" \
  >"$WORK/s4-home-go/.claude/skills/trackfw/SKILL.md"

S4_PROJECT="$WORK/s4-project"
mkdir -p "$S4_PROJECT"

before=$(snapshot_tree "$WORK/s4-home-go")
run_update "$WORK/s4-home-go" "$S4_PROJECT" harness --dry-run --json
S4_GO_EXIT=$UPDATE_EXIT
S4_GO_OUT=$UPDATE_STDOUT
after=$(snapshot_tree "$WORK/s4-home-go")

if [[ "$before" != "$after" ]]; then
  diag "update-harness/dry-run/no-writes" "filesystem tree under HOME changed during --dry-run"
fi
if [[ "$FAIL" -eq 0 ]]; then
  ok "update-harness/dry-run/no-writes"
fi

if [[ "$S4_GO_EXIT" != "0" ]]; then
  diag "update-harness/dry-run/exit-zero" "expected exit 0 for --dry-run, got $S4_GO_EXIT"
fi

if [[ "$FAIL" -eq 0 ]]; then
  dry=$(python3 -c "import json,sys; print(json.loads(sys.argv[1])['dry_run'])" "$S4_GO_OUT")
  if [[ "$dry" != "True" ]]; then
    diag "update-harness/dry-run/dry-run-field" "expected dry_run=true in JSON, got $dry"
  fi
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "update-harness/dry-run/go-behavioral-pin"
fi

# ===========================================================================
# Scenario 5 — target list is non-empty (drawn from scenario 1's output).
# ===========================================================================
if [[ -n "${S1_GO_OUT:-}" ]]; then
  ids_go=$(target_ids_json "$S1_GO_OUT" 2>/dev/null || echo "PARSE_ERROR")
  if [[ "$ids_go" == "PARSE_ERROR" || -z "$ids_go" || "$ids_go" == "[]" ]]; then
    diag "update-harness/target-list/non-empty" "Go declared an empty or unparseable target list"
  else
    ok "update-harness/target-list/non-empty (${ids_go})"
  fi
fi

# ===========================================================================
# Scenario 6 — skip warning emitted (exit 0) when artifact is outdated+owned,
# global scope.
# ===========================================================================
S6_PROJ="$WORK/s6-project"
mkdir -p "$S6_PROJ"
home="$WORK/s6-home-go"
install_agent_global "$home" "gemini" "architect"
artifact="$home/.gemini/agents/trackfw-architect.md"
manifest="$home/.trackfw/integrations-manifest.json"
art_key=$(python3 -c "import json; d=json.load(open('$manifest')); print(list(d['artifacts'].keys())[0])")
patch_manifest_outdated "$manifest" "$art_key" "$SKIP_SENTINEL"

run_agents_install "$home" "$S6_PROJ" "gemini" "architect" "global"
S6_GO_EXIT=$AGENTS_INSTALL_EXIT
S6_GO_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)

if [[ "$S6_GO_EXIT" != "0" ]]; then
  diag "skip-parity/global-scope/exit-zero" "go exited $S6_GO_EXIT — install over outdated+owned must be exit 0"
fi

if [[ -z "${S6_GO_WARN:-}" ]]; then
  diag "skip-parity/global-scope/vacuity-guard" "Go emitted no skip warning — fixture may be broken"
else
  ok "skip-parity/global-scope/go-behavioral-pin"
fi

# ===========================================================================
# Scenario 7 — skip warning emitted (exit 0), project scope.
# ===========================================================================
S7_PROJ="$WORK/s7-proj-go"
S7_HOME="$WORK/s7-home-go"
mkdir -p "$S7_PROJ" "$S7_HOME"
install_agent_project "$S7_PROJ" "$S7_HOME" "claude" "architect"
manifest="$S7_PROJ/.trackfw/integrations-manifest.json"
art_key=$(python3 -c "import json; d=json.load(open('$manifest')); print(list(d['artifacts'].keys())[0])")
artifact="$art_key"  # manifest key IS the absolute artifact path for project scope
patch_manifest_outdated "$manifest" "$artifact" "$SKIP_SENTINEL"

run_agents_install "$S7_HOME" "$S7_PROJ" "claude" "architect" "project"
S7_GO_EXIT=$AGENTS_INSTALL_EXIT
S7_GO_WARN=$(grep '^warning: skipping outdated artifact ' <<<"$AGENTS_INSTALL_STDERR" || true)

if [[ "$S7_GO_EXIT" != "0" ]]; then
  diag "skip-parity/project-scope/exit-zero" "go exited $S7_GO_EXIT — install over outdated+owned must be exit 0"
fi

if [[ -z "${S7_GO_WARN:-}" ]]; then
  diag "skip-parity/project-scope/vacuity-guard" "Go emitted no skip warning — fixture may be broken"
else
  ok "skip-parity/project-scope/go-behavioral-pin"
fi

# ===========================================================================
# Scenario 8 — E2E: init with outdated global artifact.
# Must: exit 0, scaffold complete (trackfw.yaml), bytes preserved (sentinel
# untouched), sibling artifact written (proves skip ≠ abort), skip warning
# in stderr.
# ===========================================================================
home="$WORK/s8-home-go"
proj="$WORK/s8-proj-go"
mkdir -p "$home" "$proj"
install_agent_global "$home" "gemini" "architect"
manifest="$home/.trackfw/integrations-manifest.json"
art_key=$(python3 -c "import json; d=json.load(open('$manifest')); print(list(d['artifacts'].keys())[0])")
patch_manifest_outdated "$manifest" "$art_key" "$SKIP_SENTINEL"
architect="$home/.gemini/agents/trackfw-architect.md"
backend="$home/.gemini/agents/trackfw-backend.md"

run_init "$home" "$proj" "gemini"
if [[ "$INIT_EXIT" -ne 0 ]]; then
  diag "e2e/init-outdated-global/exit-zero" "go: init exited $INIT_EXIT — expected 0 with outdated+owned global artifact"
else
  if [[ ! -f "$proj/trackfw.yaml" ]]; then
    diag "e2e/init-outdated-global/scaffold" "trackfw.yaml missing — scaffold incomplete"
  fi
  if [[ ! -f "$architect" ]]; then
    diag "e2e/init-outdated-global/bytes-preserved" "architect artifact was removed"
  elif ! grep -qF "$SKIP_SENTINEL" "$architect"; then
    diag "e2e/init-outdated-global/bytes-preserved" "sentinel bytes overwritten — skip did not preserve artifact"
  fi
  if [[ ! -f "$backend" ]]; then
    diag "e2e/init-outdated-global/sibling-written" "trackfw-backend.md was not written — init may have aborted"
  fi
  warn_line=$(grep '^warning: skipping outdated artifact ' <<<"$INIT_STDERR" || true)
  if [[ -z "$warn_line" ]]; then
    diag "e2e/init-outdated-global/warning-in-stderr" "no skip warning in stderr"
  fi
  if [[ "$FAIL" -eq 0 ]]; then
    ok "e2e/init-outdated-global/go-behavioral-pin"
  fi
fi

# ===========================================================================
# Scenario 9 — Sandbox by inclusion: dangling symlink OUTSIDE the declared
# set (.venv/bin/python → nonexistent) does NOT abort --dry-run.
# Vacuity guard: JSON output must have dry_run=true.
# ===========================================================================
S9_PROJ="$WORK/s9-proj"
mkdir -p "$S9_PROJ/.venv/bin"
ln -sf /nonexistent-python3.99 "$S9_PROJ/.venv/bin/python"
cat > "$S9_PROJ/trackfw.yaml" << 'S9YAML'
name: s9
S9YAML

run_update "$WORK/s9-home-go" "$S9_PROJ" --dry-run --json
s9_dry_flag=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(str(d.get('dry_run', 'MISSING')).lower())
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")
if [[ "$s9_dry_flag" != "true" ]]; then
  diag "sandbox/dangling-outside-set/vacuity" "dry_run field not true ($s9_dry_flag) — fixture broken or output unparseable"
fi
if [[ "$UPDATE_EXIT" != "0" ]]; then
  diag "sandbox/dangling-outside-set/exit-zero" "go exited $UPDATE_EXIT — dangling symlink outside declared set must not abort --dry-run"
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/dangling-outside-set"
fi

# ===========================================================================
# Scenario 10 — Sandbox by inclusion: dangling symlink INSIDE the declared
# set (CLAUDE.md → nonexistent) is treated as absent, not as an error.
# Must: exit 0, state=missing.
# ===========================================================================
S10_PROJ="$WORK/s10-proj"
mkdir -p "$S10_PROJ"
cat > "$S10_PROJ/trackfw.yaml" << 'S10YAML'
name: s10
S10YAML
ln -sf /nonexistent-claude "$S10_PROJ/CLAUDE.md"

run_update "$WORK/s10-home-go" "$S10_PROJ" --dry-run --json --targets agent-rules
s10_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")
if [[ "$s10_state" == "PARSE_ERROR"* ]]; then
  diag "sandbox/dangling-inside-set/vacuity" "output unparseable: $s10_state"
elif [[ "$s10_state" != "missing" ]]; then
  diag "sandbox/dangling-inside-set/state" "go: expected state=missing for CLAUDE.md broken symlink, got '$s10_state'"
fi
if [[ "$UPDATE_EXIT" != "0" ]]; then
  diag "sandbox/dangling-inside-set/exit-zero" "go exited $UPDATE_EXIT — broken symlink inside declared set must not abort"
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/dangling-inside-set"
fi

# ===========================================================================
# Scenario 11 — Gap E: trackfw.yaml with agent_conventions — dry-run and
# real run report the SAME state for agent-rules.
# Vacuity guard: real-run state must be 'updated'.
# ===========================================================================
S11_SEED="$WORK/s11-seed"
mkdir -p "$S11_SEED"
echo "# S11 Seed" > "$S11_SEED/CLAUDE.md"
cat > "$S11_SEED/trackfw.yaml" << 'S11SEED'
name: s11-seed
S11SEED
(cd "$S11_SEED" && HOME="$WORK/s11-seed-home" "$GO_BIN" update --targets agent-rules >/dev/null 2>&1) || true

if [[ ! -f "$S11_SEED/CLAUDE.md" ]]; then
  diag "sandbox/gap-e/seed-setup" "s11 seed: Go update did not write CLAUDE.md — fixture cannot be built"
  S11_SEED_OK=0
else
  S11_SEED_OK=1
fi

if [[ "$S11_SEED_OK" -eq 1 ]]; then
  S11_DRY="$WORK/s11-dry-go"
  S11_REAL="$WORK/s11-real-go"
  mkdir -p "$S11_DRY" "$S11_REAL"
  cp "$S11_SEED/CLAUDE.md" "$S11_DRY/CLAUDE.md"
  cp "$S11_SEED/CLAUDE.md" "$S11_REAL/CLAUDE.md"
  cat > "$S11_DRY/trackfw.yaml" << 'S11YAML'
name: s11
agent_conventions: "Always commit tests alongside code"
S11YAML
  cp "$S11_DRY/trackfw.yaml" "$S11_REAL/trackfw.yaml"

  run_update "$WORK/s11-home-dry-go" "$S11_DRY" --dry-run --json --targets agent-rules
  s11_dry_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  run_update "$WORK/s11-home-real-go" "$S11_REAL" --json --targets agent-rules
  s11_real_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  if [[ "$s11_real_state" != "updated" ]]; then
    diag "sandbox/gap-e/vacuity" "real-run state='$s11_real_state' (expected 'updated') — fixture may be broken"
  fi

  if [[ "$s11_dry_state" != "$s11_real_state" ]]; then
    diag "sandbox/gap-e/dry-vs-real" "dry=$s11_dry_state real=$s11_real_state — dry-run diverged from real run for agent_conventions fixture; trackfw.yaml may be missing from sandbox"
  fi

  if [[ "$FAIL" -eq 0 ]]; then
    ok "sandbox/gap-e/dry-vs-real"
  fi
fi

# ===========================================================================
# Scenario 12 — Gap C: .github/copilot-instructions.md present — dry-run and
# real run agree on the agent-hooks target state.
# Vacuity guard: .github/hooks/trackfw-attention.json must be written.
# ===========================================================================
S12_DRY="$WORK/s12-dry-go"
S12_REAL="$WORK/s12-real-go"
mkdir -p "$S12_DRY/.github" "$S12_DRY/.claude" "$S12_REAL/.github" "$S12_REAL/.claude"

cat > "$S12_DRY/trackfw.yaml" << 'S12YAML'
name: s12
S12YAML
touch "$S12_DRY/.github/copilot-instructions.md"
echo '{}' > "$S12_DRY/.claude/settings.json"

cp "$S12_DRY/trackfw.yaml" "$S12_REAL/trackfw.yaml"
touch "$S12_REAL/.github/copilot-instructions.md"
echo '{}' > "$S12_REAL/.claude/settings.json"

run_update "$WORK/s12-home-dry-go" "$S12_DRY" --dry-run --json --targets agent-hooks
s12_dry_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

run_update "$WORK/s12-home-real-go" "$S12_REAL" --json --targets agent-hooks
s12_real_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

if [[ ! -f "$S12_REAL/.github/hooks/trackfw-attention.json" ]]; then
  diag "sandbox/gap-c/vacuity" ".github/hooks/trackfw-attention.json not created in real run — fixture may be broken"
fi

if [[ "$s12_dry_state" != "$s12_real_state" ]]; then
  diag "sandbox/gap-c/dry-vs-real" "dry=$s12_dry_state real=$s12_real_state — copilot detection signal (.github/copilot-instructions.md) may be missing from sandbox"
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/gap-c/dry-vs-real"
fi

# ===========================================================================
# Scenario 13 — Gap A/B: .windsurf/hooks.json and .amazonq/cli-agents/
# q_cli_default.json appear in the agent-hooks declared path list.
# Vacuity guard: state must not be 'missing'.
# ===========================================================================
S13_PROJ="$WORK/s13-proj"
mkdir -p "$S13_PROJ/.amazonq" "$S13_PROJ/.claude"
cat > "$S13_PROJ/trackfw.yaml" << 'S13YAML'
name: s13
S13YAML
touch "$S13_PROJ/.windsurfrules"
echo '{}' > "$S13_PROJ/.claude/settings.json"

run_update "$WORK/s13-home-go" "$S13_PROJ" --dry-run --json --targets agent-hooks
s13_path=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['path'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")
s13_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

if [[ "$s13_state" == "missing" ]]; then
  diag "sandbox/gap-ab/vacuity" "agent-hooks state=missing — fixture may be broken"
fi

if [[ "$s13_path" == "PARSE_ERROR"* ]]; then
  diag "sandbox/gap-ab/parse" "output unparseable: $s13_path"
else
  if ! echo "$s13_path" | grep -qF '.windsurf/hooks.json'; then
    diag "sandbox/gap-a/declared-path" ".windsurf/hooks.json absent from agent-hooks path field: $s13_path"
  fi
  if ! echo "$s13_path" | grep -qF '.amazonq/cli-agents/q_cli_default.json'; then
    diag "sandbox/gap-b/declared-path" ".amazonq/cli-agents/q_cli_default.json absent from agent-hooks path field: $s13_path"
  fi
fi

if [[ "$FAIL" -eq 0 ]]; then
  ok "sandbox/gap-ab/declared-path"
fi

# ===========================================================================
# Scenario 14 — R-novo-1 fix: declared directory already correct.
# Vacuity guard 1: .claude/commands/trackfw non-empty after prime.
# Vacuity guard 2: real state == skipped.
# Main assertion: dry == real.
# ===========================================================================
S14_FAIL_BEFORE="$FAIL"
S14_PROJ="$WORK/s14-proj-go"
mkdir -p "$S14_PROJ"
cat > "$S14_PROJ/trackfw.yaml" << 'S14YAML'
name: s14
S14YAML

# Prime: real run with --install-missing to populate .claude/commands/trackfw.
run_update "$WORK/s14-home-go" "$S14_PROJ" --targets claude-commands --install-missing

if [[ ! -d "$S14_PROJ/.claude/commands/trackfw" ]]; then
  diag "sandbox/dir-already-correct/vacuity-dir-exists" ".claude/commands/trackfw not created by prime real run — target broken or fixture invalid"
elif [[ -z "$(ls -A "$S14_PROJ/.claude/commands/trackfw" 2>/dev/null)" ]]; then
  diag "sandbox/dir-already-correct/vacuity-dir-nonempty" ".claude/commands/trackfw is empty after prime — claude-commands wrote no files"
else
  run_update "$WORK/s14-home-go" "$S14_PROJ" --dry-run --json --targets claude-commands
  s14_dry_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  run_update "$WORK/s14-home-go" "$S14_PROJ" --json --targets claude-commands
  s14_real_state=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    print(d['targets'][0]['state'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
" "$UPDATE_STDOUT" 2>/dev/null || echo "PARSE_ERROR")

  if [[ "$s14_real_state" != "skipped" ]]; then
    diag "sandbox/dir-already-correct/vacuity-real-skipped" "real-run state='$s14_real_state' (expected 'skipped') — prime did not produce correct content, or target is non-idempotent"
  fi

  if [[ "$s14_dry_state" != "$s14_real_state" ]]; then
    diag "sandbox/dir-already-correct/dry-vs-real" "dry=$s14_dry_state real=$s14_real_state — dry-run diverged from real for already-correct claude-commands directory; copyPath may not be recursing directory contents into sandbox"
  fi
fi

if [[ "$FAIL" -eq "$S14_FAIL_BEFORE" ]]; then
  ok "sandbox/dir-already-correct/dry-vs-real"
fi

# ---------------------------------------------------------------------------
if [[ "$FAIL" -ne 0 ]]; then
  echo "check-update-parity: behavioral pin failures detected — see FAIL lines above." >&2
  exit 1
fi

echo "All check-update-parity.sh scenarios passed (Go binary only — v8 single-runtime)."
