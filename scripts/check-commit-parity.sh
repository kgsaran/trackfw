#!/usr/bin/env bash
# check-commit-parity.sh — proves `trackfw commit -m "<message>"` behaves byte-for-byte
# identically in Go, Node.js, and Python for the three governance-gate scenarios pinned in
# ML-4A of docs/roadmaps/wip/ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-
# subagente-via-deny-hooks-nos-7-runtimes-suportados.md:
#   (a) main/master           → always blocked; same "commit direto em ..." stdout message
#                                and "blocked: commit directly on ..." stderr line, exit 1,
#                                `git commit` never runs.
#   (b) feat/<slug> without a
#       matching roadmap in
#       wip/ or done/          → blocked with the same governance orientation message
#                                (validator.BranchGovernanceOrientation), exit 1, `git commit`
#                                never runs.
#   (c) non-governed branch
#       (e.g. docs/<slug>)
#       with nothing staged    → warns "does not follow feat/fix/refactor" on stdout, then
#                                actually runs `git commit`, which fails with git's own
#                                "nothing to commit" diagnostic (same git binary → identical
#                                output across all three runtimes) — proves the commit is a
#                                real passthrough, not a trackfw-owned second message. This is
#                                the scenario that first caught a real cross-CLI divergence: an
#                                unflushed sys.stdout in pypi/trackfw/commands/commit.py let
#                                git's inherited-stdio output reach the captured file before the
#                                trackfw warning when stdout was not a TTY — see
#                                vault/notes/python-stdout-buffering-reorders-before-inherited-stdio-subprocess-2026-08-14.md
#
# Follows the exact conventions of scripts/check-branch-new-parity.sh: set -euo pipefail,
# mktemp -d fixtures with a cleanup trap, BASH_SOURCE-relative ROOT_DIR, ok()/fail()
# accumulating FAIL=1, byte-level diff -u between runtimes on both stdout and stderr.
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
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-commit-parity.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

# ---------------------------------------------------------------------------
# Resolve the three runtimes — mirrors check-branch-new-parity.sh:
#   GO_BIN unset → build a throwaway binary so the script also works standalone.
#   GO_BIN relative → prefix with ROOT_DIR.
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
  echo "check-commit-parity: Go binary not found/executable at $GO_BIN" >&2
  exit 1
fi
if [[ ! -f "$NODE_CLI" ]]; then
  echo "check-commit-parity: Node CLI not found at $NODE_CLI" >&2
  exit 1
fi

FAIL=0
ok()   { echo "OK   [$1]"; }
fail() { echo "FAIL [$1]: $2" >&2; FAIL=1; }

# run_commit RUNTIME DIR ARGS...
# Runs `trackfw commit ARGS...` from DIR. Sets CM_EXIT and writes stdout/stderr to
# $WORK/<label>.<runtime>.out / .err (label passed by caller via CM_LABEL).
run_commit() {
  local runtime=$1 dir=$2
  shift 2
  local out_file="$WORK/$CM_LABEL.$runtime.out" err_file="$WORK/$CM_LABEL.$runtime.err"
  set +e
  case "$runtime" in
    go)   (cd "$dir" && "$GO_BIN" commit "$@")                              >"$out_file" 2>"$err_file" ;;
    node) (cd "$dir" && node "$NODE_CLI" commit "$@")                       >"$out_file" 2>"$err_file" ;;
    py)   (cd "$dir" && PYTHONPATH="$PY_ROOT" python3 -m trackfw commit "$@") >"$out_file" 2>"$err_file" ;;
    *)    echo "run_commit: unknown runtime '$runtime'" >&2; exit 1 ;;
  esac
  CM_EXIT=$?
  set -e
  CM_OUT_FILE=$out_file
  CM_ERR_FILE=$err_file
}

# assert_three_way LABEL — diffs go vs node and go vs py for both stdout and stderr
# previously captured under $WORK/<label>.<runtime>.{out,err}, plus exit codes recorded in
# $WORK/<label>.<runtime>.exit. Fails with a labeled diagnostic and the unified diff on
# divergence (never silently degrades — P2).
assert_three_way() {
  local label=$1
  local diverged=0
  local stream
  for stream in out err; do
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.node.$stream" >"$WORK/$label.diff.go-node.$stream" 2>&1; then
      fail "commit-parity/$label/go-vs-node/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-node.$stream")"
      diverged=1
    fi
    if ! diff -u "$WORK/$label.go.$stream" "$WORK/$label.py.$stream" >"$WORK/$label.diff.go-py.$stream" 2>&1; then
      fail "commit-parity/$label/go-vs-py/$stream" "stdout/stderr diverges:
$(cat "$WORK/$label.diff.go-py.$stream")"
      diverged=1
    fi
  done
  local go_exit node_exit py_exit
  go_exit=$(cat "$WORK/$label.go.exit")
  node_exit=$(cat "$WORK/$label.node.exit")
  py_exit=$(cat "$WORK/$label.py.exit")
  if [[ "$go_exit" != "$node_exit" || "$go_exit" != "$py_exit" ]]; then
    fail "commit-parity/$label/exit-code" "exit codes diverge: go=$go_exit node=$node_exit py=$py_exit"
    diverged=1
  fi
  if [[ "$diverged" -eq 0 ]]; then
    ok "commit-parity/$label"
  fi
}

# ---------------------------------------------------------------------------
# Shared fixture scaffolding — flat layout, minimal governance tree, real git repo (commit
# reads the current branch via `git rev-parse --abbrev-ref HEAD`, so a real repo is required
# in every scenario, unlike branch-new's scenarios (a)/(b) which stay outside git).
# ---------------------------------------------------------------------------
make_repo() {
  local dir=$1
  mkdir -p "$dir/docs/roadmaps/wip" "$dir/docs/roadmaps/done" "$dir/docs/req" "$dir/docs/adr"
  cat >"$dir/trackfw.yaml" <<'EOF'
req_dir: docs/req
roadmap_dir: docs/roadmaps
adr_dir: docs/adr
EOF
  (
    cd "$dir"
    git init -q
    git config user.email test@example.com
    git config user.name "trackfw parity gate"
    echo x >file.txt
    git add file.txt trackfw.yaml
    git commit -qm init
    git checkout -q main 2>/dev/null || git checkout -q master 2>/dev/null || git branch -m main
  )
}

# make_repo_with_staged_change DIR — same as make_repo, plus a staged, uncommitted change.
# Required by scenarios (a)/(b): without something staged, a regressed block (e.g. a missing
# return/exit that falls through to execGitCommit) would still die on git's own "nothing to
# commit" and land on the SAME log_count=1 as the correctly-blocked case, making the
# post-condition guard below inert — it must be able to observe a real leaked commit.
make_repo_with_staged_change() {
  local dir=$1
  make_repo "$dir"
  (cd "$dir" && echo staged >staged.txt && git add staged.txt)
}

# ---------------------------------------------------------------------------
# Scenario (a) — main/master: always blocked, `git commit` never runs.
# Vacuity guard: stdout must contain the Portuguese governance message ("não é permitido"),
# proving the runtime actually reached the protected-branch check.
# ---------------------------------------------------------------------------
CM_LABEL="main-master-blocked"
for runtime in go node py; do
  fixture="$WORK/a-$runtime"
  make_repo_with_staged_change "$fixture"
  run_commit "$runtime" "$fixture" -m "test: should never land"
  echo "$CM_EXIT" >"$WORK/$CM_LABEL.$runtime.exit"
  if [[ "$CM_EXIT" -eq 0 ]]; then
    fail "commit-parity/$CM_LABEL/$runtime" "expected non-zero exit (blocked), got 0"
    continue
  fi
  if ! grep -qF 'não é permitido' "$CM_OUT_FILE"; then
    fail "commit-parity/$CM_LABEL/$runtime" "vacuity guard: stdout missing governance message; stdout: $(cat "$CM_OUT_FILE")"
    continue
  fi
  # Confirms `git commit` never ran: HEAD stays at the fixture's single "init" commit. The
  # fixture carries a staged-but-uncommitted change (make_repo_with_staged_change) so that a
  # regressed block (e.g. a missing return that falls through to execGitCommit) would actually
  # produce a second commit here instead of dying on git's own "nothing to commit" and
  # masking the regression behind the same log_count=1.
  log_count=$(cd "$fixture" && git log --oneline | wc -l | tr -d ' ')
  if [[ "$log_count" != "1" ]]; then
    fail "commit-parity/$CM_LABEL/$runtime" "git commit must never run on main/master, but HEAD has $log_count commits"
    continue
  fi
done
assert_three_way "$CM_LABEL"

# ---------------------------------------------------------------------------
# Scenario (b) — feat/<slug> without a matching roadmap in wip/ or done/: blocked, `git
# commit` never runs.
# Vacuity guard: stdout must contain the orientation message ("no roadmap is in wip/ nor done/").
# ---------------------------------------------------------------------------
CM_LABEL="feat-no-roadmap-blocked"
for runtime in go node py; do
  fixture="$WORK/b-$runtime"
  make_repo_with_staged_change "$fixture"
  (cd "$fixture" && git checkout -q -b feat/parity-no-roadmap)
  run_commit "$runtime" "$fixture" -m "test: should never land"
  echo "$CM_EXIT" >"$WORK/$CM_LABEL.$runtime.exit"
  if [[ "$CM_EXIT" -eq 0 ]]; then
    fail "commit-parity/$CM_LABEL/$runtime" "expected non-zero exit (blocked), got 0"
    continue
  fi
  if ! grep -qF 'no roadmap is in wip/ nor done/' "$CM_OUT_FILE"; then
    fail "commit-parity/$CM_LABEL/$runtime" "vacuity guard: stdout missing orientation message; stdout: $(cat "$CM_OUT_FILE")"
    continue
  fi
  # Same rationale as scenario (a): the fixture carries a staged-but-uncommitted change so a
  # leaked commit would actually land and be observable here.
  log_count=$(cd "$fixture" && git log --oneline | wc -l | tr -d ' ')
  if [[ "$log_count" != "1" ]]; then
    fail "commit-parity/$CM_LABEL/$runtime" "git commit must never run without a matching roadmap, but HEAD has $log_count commits"
    continue
  fi
done
assert_three_way "$CM_LABEL"

# ---------------------------------------------------------------------------
# Scenario (c) — non-governed branch (outside feat/fix/refactor), nothing staged: warns on
# stdout, then actually runs `git commit`, which fails with git's own "nothing to commit"
# diagnostic — the one scenario that exercises the real execGitCommit passthrough end-to-end.
# ---------------------------------------------------------------------------
CM_LABEL="non-governed-nothing-staged"
for runtime in go node py; do
  fixture="$WORK/c-$runtime"
  make_repo "$fixture"
  (cd "$fixture" && git checkout -q -b docs/parity-housekeeping)
  run_commit "$runtime" "$fixture" -m "docs: should fail on git's own nothing-to-commit"
  echo "$CM_EXIT" >"$WORK/$CM_LABEL.$runtime.exit"
  if [[ "$CM_EXIT" -eq 0 ]]; then
    fail "commit-parity/$CM_LABEL/$runtime" "expected non-zero exit (git's own 'nothing to commit'), got 0"
    continue
  fi
  if ! grep -qF 'does not follow feat/fix/refactor' "$CM_OUT_FILE"; then
    fail "commit-parity/$CM_LABEL/$runtime" "vacuity guard: stdout missing non-governed-branch warning; stdout: $(cat "$CM_OUT_FILE")"
    continue
  fi
  if ! grep -qi 'nothing to commit' "$CM_OUT_FILE" "$CM_ERR_FILE"; then
    fail "commit-parity/$CM_LABEL/$runtime" "vacuity guard: neither stdout nor stderr carries git's own 'nothing to commit' diagnostic; stdout: $(cat "$CM_OUT_FILE") stderr: $(cat "$CM_ERR_FILE")"
    continue
  fi
done
assert_three_way "$CM_LABEL"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo
if [[ "$FAIL" -eq 0 ]]; then
  echo "All check-commit-parity.sh scenarios passed."
else
  echo "check-commit-parity.sh: one or more scenarios FAILED." >&2
fi
exit "$FAIL"
