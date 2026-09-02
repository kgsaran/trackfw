#!/usr/bin/env bash
# check-harness-hooks-parity.sh — proves the GLOBAL-scope credential-guard AND
# git-branch-guard hook files written by `trackfw update harness --targets
# <tool>-credential-guard,<tool>-git-branch-guard --install-missing`
# (~/.claude/settings.json, ~/.codex/hooks.json, ~/.gemini/settings.json,
# ~/.cursor/hooks.json, ~/.copilot/settings.json,
# ~/.kiro/hooks/trackfw-credential-guard.json,
# ~/.kiro/hooks/trackfw-git-branch-guard.json) are STRUCTURALLY identical
# across Go, Node.js and Python, for each of the 6 native-wave CLIs.
#
# Windsurf and Amazon Q are deliberately NOT covered by this gate, and never
# can be without a product change: `HarnessTargetIDs`/`buildHarnessTargetIDs`
# (internal/generators/update.go) only pairs credential-guard/git-branch-guard
# targets with claude/codex/gemini/cursor/copilot/kiro — there is no
# `windsurf-credential-guard`, `windsurf-git-branch-guard`, `amazonq-
# credential-guard` or `amazonq-git-branch-guard` target in any of the 3
# CLIs (confirmed by grep across update.go/update-harness.js/
# update_harness.py, ROADMAP-2026-08-20/ML-1B); the same doc comment says
# "Windsurf has no native hook mechanism and stays out per the ADR", and
# Amazon Q was simply never given a harness-scope (global, ~/.amazonq) pair
# either — both only get project-scope wiring via `discover --init`
# (InjectWindsurfHooks/InjectAmazonQHooks), which check-agent-hooks-parity.sh
# covers. There is no global ~/.amazonq or ~/.windsurf hook file this gate
# could compare; extending CLIS here would require inventing a harness
# target that does not exist, which is a product-behavior change out of
# ML-1B's scope, not a gate-coverage gap. See docs/cli-parity.md's "Git
# branch guard por runtime" section for the correction to the previously
# false claim that this gate already covered them.
#
# git-branch-guard wiring (ROADMAP-2026-08-17 Wave 2/ML-2A) reuses the exact
# same merge helpers credential-guard does — for the 5 merge-based CLIs
# (claude/codex/gemini/cursor/copilot) both guards' entries coexist in the
# SAME hook file; only Kiro splits into two dedicated files (its
# credential-guard writer rewrites its document wholesale, so sharing one
# file with a second wholesale writer would make the two targets flap
# between each other's desired state — see internal/generators/update.go:
# harnessGitBranchGuardTargetKiro's doc comment).
#
# Sibling of check-agent-hooks-parity.sh (PR #141), which covers the exact
# same 6 CLIs but for PROJECT-scope hook files written by `discover --init`.
# Neither gate subsumes the other: the per-project InjectXHooks and the
# per-home harnessCredentialGuardTarget<Tool>/harnessGitBranchGuardTarget
# <Tool>/credentialGuardTarget<Tool>/gitBranchGuardTarget<Tool>/
# _credential_guard_<tool>_result/_git_branch_guard_<tool>_result
# implementations are independent code paths in all 3 stacks
# (ROADMAP-2026-08-06 Wave 2, ROADMAP-2026-08-17 Wave 2), and ML-3A's dedup
# logic reads the global file the harness gate here exercises but never
# writes it.
#
# Method: same structural (parsed-JSON) deep-compare as
# check-agent-hooks-parity.sh — NOT byte-identical, since JSON indentation/
# key-insertion-order differs between the Go/Node.js/Python serializers and
# is never semantically significant here (unlike GitHub Copilot's own
# project-scope array-order contract, none of these 6 hook shapes has more
# than one entry per event array in a fresh `--install-missing` run, so array
# order is moot for THIS gate specifically — still compared for safety, at no
# extra cost, via the same diff() used by check-agent-hooks-parity.sh).
#
# Real invocation, not internal generator/function calls: each stack runs its
# own real `update harness --targets <tool>-credential-guard --install-
# missing` entry point (Go binary / `node npm/bin/trackfw` / `python3 -m
# trackfw`) once per CLI, each against its OWN isolated $HOME fixture
# directory (HOME env var — never the real $HOME of whoever runs this gate;
# `trackfw update harness` never touches anything outside $HOME, see
# internal/commands/update_harness.go's own doc comment).
#
# Absolute-path normalization: every one of the 6 hook files embeds the
# ABSOLUTE path of ~/.trackfw/scripts/trackfw-credential-guard.sh (a global
# hook must resolve from any project's cwd, so a relative path is not an
# option — see internal/generators/update.go:harnessCredentialGuardTarget-
# Claude's own doc comment). Because each stack runs against its OWN $HOME
# fixture directory (mirroring check-agent-hooks-parity.sh's one-work-dir-
# per-runtime layout, not a single shared fixture — see below for why), that
# absolute path is textually different per runtime even when every stack
# resolves it correctly, and a naive structural diff would report false
# drift on every single field that embeds it. Fixed by textually replacing
# each runtime's own $HOME fixture path with a common "<HOME>" placeholder
# in the raw file content BEFORE parsing as JSON (see normalize_home below) —
# the same style of pre-parse normalization already used by
# check-barrier.sh's normalize_barrier_json for a different reason
# (whitespace). A single shared $HOME fixture across all 3 runtimes was
# considered and rejected: `trackfw update harness --install-missing` is
# idempotent-merge, so a second and third runtime writing into a $HOME
# already populated by an earlier runtime would report `state: skipped`
# instead of `state: updated` for two of the three stacks, silently
# weakening this gate's most basic guarantee — that each stack's OWN
# from-scratch write path produces the same structure, not just that stacks
# agree once someone else has already written the file.
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
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-harness-hooks-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
# Squash any double-slash introduced by a trailing slash in $TMPDIR (macOS
# sets TMPDIR with a trailing "/"): filepath.Join (Go)/path.join (Node)/
# os.path.join (Python) all normalize it away when building the embedded
# absolute script path, but the raw $WORK string here would not — which
# would silently break the textual "$HOME" normalization in compare_json
# below (the literal never matches, so no drift is ever normalized away).
WORK=$(cd "$WORK" && pwd)

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-agent-hooks-parity.sh.
# ---------------------------------------------------------------------------
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw)
elif [[ "$GO_BIN" != /* ]]; then
  GO_BIN="$ROOT_DIR/$GO_BIN"
fi
NODE_CLI="$ROOT_DIR/npm/bin/trackfw"
PY_ROOT="${PY_ROOT:-$ROOT_DIR/pypi}"

if [[ ! -x "$GO_BIN" ]]; then
  echo "check-harness-hooks-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-harness-hooks-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# Per-(CLI, guard) table: target id passed to --targets and the relative (to
# $HOME) path of the global hook file each harness target writes. Both
# guards share ONE file for the 5 merge-based CLIs; Kiro splits into two
# dedicated files (see file header).
# ---------------------------------------------------------------------------
CLIS="claude codex gemini cursor copilot kiro"
GUARDS="credential-guard git-branch-guard"

target_for() {
  echo "$1-$2"
}

hookfile_for() {
  local cli=$1 guard=$2
  case "$cli" in
    claude)  echo ".claude/settings.json" ;;
    codex)   echo ".codex/hooks.json" ;;
    gemini)  echo ".gemini/settings.json" ;;
    cursor)  echo ".cursor/hooks.json" ;;
    copilot) echo ".copilot/settings.json" ;;
    kiro)    echo ".kiro/hooks/trackfw-$guard.json" ;;
    *) echo "hookfile_for: unknown cli '$cli'" >&2; exit 1 ;;
  esac
}

# ---------------------------------------------------------------------------
# Run `update harness --targets <all 12 ids> --install-missing` once per
# runtime, each against its own fresh $HOME fixture directory (never the real
# $HOME of whoever runs this gate).
# ---------------------------------------------------------------------------
ALL_TARGETS=""
for cli in $CLIS; do
  for guard in $GUARDS; do
    id=$(target_for "$cli" "$guard")
    ALL_TARGETS="${ALL_TARGETS:+$ALL_TARGETS,}$id"
  done
done

run_update_harness() {
  local runtime=$1 home_dir=$2
  mkdir -p "$home_dir"
  case "$runtime" in
    go)   HOME="$home_dir" "$GO_BIN" update harness --targets "$ALL_TARGETS" --install-missing                              >/dev/null 2>"$WORK/$runtime.err" ;;
    node) HOME="$home_dir" node "$NODE_CLI" update harness --targets "$ALL_TARGETS" --install-missing                       >/dev/null 2>"$WORK/$runtime.err" ;;
    py)   HOME="$home_dir" PYTHONPATH="$PY_ROOT" python3 -m trackfw update harness --targets "$ALL_TARGETS" --install-missing >/dev/null 2>"$WORK/$runtime.err" ;;
    *)    echo "run_update_harness: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
}

set +e
for runtime in go node py; do
  home_dir="$WORK/$runtime-home"
  run_update_harness "$runtime" "$home_dir"
  status=$?
  if [[ "$status" -ne 0 ]]; then
    fail "harness-hooks-parity/$runtime/update-harness" \
      "update harness exited $status; stderr: $(cat "$WORK/$runtime.err" 2>/dev/null)"
  fi
done
set -e

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-harness-hooks-parity.sh: one or more scenarios FAILED (setup)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# P2 vacuity guards, two of them (mirrors check-agent-hooks-parity.sh):
#   1. the hook file exists and is non-empty for all three runtimes, per
#      (cli, guard) — for the 5 merge-based CLIs this is the SAME file
#      checked twice (once per guard), which is fine: it re-asserts
#      non-emptiness under a distinct label per guard;
#   2. the hook file actually references trackfw-<guard>.sh at least once,
#      per runtime — a regression that dropped either guard's entry from all
#      three stacks identically would otherwise still "pass" a pure
#      cross-stack equality check.
# ---------------------------------------------------------------------------
for cli in $CLIS; do
  for guard in $GUARDS; do
    hookfile=$(hookfile_for "$cli" "$guard")
    for runtime in go node py; do
      path="$WORK/$runtime-home/$hookfile"
      if [[ ! -s "$path" ]]; then
        fail "harness-hooks-parity/$cli/$guard/$runtime/$hookfile" "missing or empty: $path"
        continue
      fi
      if ! grep -q "trackfw-$guard.sh" "$path"; then
        fail "harness-hooks-parity/$cli/$guard/$runtime/present" \
          "trackfw-$guard.sh not referenced anywhere in $path"
      fi
    done
  done
done

if [[ "$FAIL" -ne 0 ]]; then
  echo
  echo "check-harness-hooks-parity.sh: one or more scenarios FAILED (vacuity guard)." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Structural (parsed-JSON) diff, go-vs-node and go-vs-py, per CLI. Each
# runtime's own $HOME fixture absolute path is textually normalized to a
# common "<HOME>" placeholder before parsing (see file header) so the
# embedded absolute script path never produces false drift.
# ---------------------------------------------------------------------------
normalize_home() {
  local file=$1 home_dir=$2
  python3 -c "
import sys
path, home = sys.argv[1], sys.argv[2]
with open(path, encoding='utf-8') as f:
    content = f.read()
content = content.replace(home, '<HOME>')
sys.stdout.write(content)
" "$file" "$home_dir"
}

compare_json() {
  local label=$1 go_file=$2 go_home=$3 other_file=$4 other_home=$5
  local out
  if out=$(python3 -c "
import json
import sys

a_text, b_text = sys.argv[1], sys.argv[2]
a = json.loads(a_text)
b = json.loads(b_text)


def diff(path, x, y, out):
    if type(x) is not type(y) and not (isinstance(x, (int, float)) and isinstance(y, (int, float))):
        out.append(f'{path}: type {type(x).__name__} (go) vs {type(y).__name__} (other) -- go={x!r} other={y!r}')
        return
    if isinstance(x, dict):
        xk, yk = set(x.keys()), set(y.keys())
        for k in sorted(xk - yk):
            out.append(f'{path}.{k}: present in go, missing in other')
        for k in sorted(yk - xk):
            out.append(f'{path}.{k}: missing in go, present in other')
        for k in sorted(xk & yk):
            diff(f'{path}.{k}', x[k], y[k], out)
    elif isinstance(x, list):
        if len(x) != len(y):
            out.append(f'{path}: array length {len(x)} (go) vs {len(y)} (other)')
        for i, (xi, yi) in enumerate(zip(x, y)):
            diff(f'{path}[{i}]', xi, yi, out)
    else:
        if x != y:
            out.append(f'{path}: value {x!r} (go) vs {y!r} (other)')


diffs = []
diff('\$', a, b, diffs)
if diffs:
    print('\n'.join(diffs))
    sys.exit(1)
" "$(normalize_home "$go_file" "$go_home")" "$(normalize_home "$other_file" "$other_home")" 2>&1); then
    ok "$label"
  else
    fail "$label" "structural drift:
$out"
  fi
}

# Compare once per DISTINCT file per cli: the 5 merge-based CLIs write both
# guards into the same file, so credential-guard's hookfile_for output
# already covers git-branch-guard too — comparing it a second time under the
# same path would be a no-op. Kiro is the exception (two distinct dedicated
# files), so it gets a second, guard-labeled comparison for the
# git-branch-guard file. The un-suffixed "$cli/go-vs-node"/"$cli/go-vs-py"
# labels are preserved unchanged (never renamed) so existing scenario
# assertions that target them (e.g. Cenário 45's kiro credential-guard
# sabotage) keep matching.
for cli in $CLIS; do
  primary_hookfile=$(hookfile_for "$cli" "credential-guard")
  go_home="$WORK/go-home"
  node_home="$WORK/node-home"
  py_home="$WORK/py-home"
  go_file="$go_home/$primary_hookfile"
  node_file="$node_home/$primary_hookfile"
  py_file="$py_home/$primary_hookfile"

  compare_json "harness-hooks-parity/$cli/go-vs-node" "$go_file" "$go_home" "$node_file" "$node_home"
  compare_json "harness-hooks-parity/$cli/go-vs-py" "$go_file" "$go_home" "$py_file" "$py_home"

  secondary_hookfile=$(hookfile_for "$cli" "git-branch-guard")
  if [[ "$secondary_hookfile" != "$primary_hookfile" ]]; then
    go_file2="$go_home/$secondary_hookfile"
    node_file2="$node_home/$secondary_hookfile"
    py_file2="$py_home/$secondary_hookfile"

    compare_json "harness-hooks-parity/$cli/git-branch-guard/go-vs-node" "$go_file2" "$go_home" "$node_file2" "$node_home"
    compare_json "harness-hooks-parity/$cli/git-branch-guard/go-vs-py" "$go_file2" "$go_home" "$py_file2" "$py_home"
  fi
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-harness-hooks-parity.sh scenarios passed."
else
  echo "check-harness-hooks-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
