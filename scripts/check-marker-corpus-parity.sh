#!/usr/bin/env bash
# check-marker-corpus-parity.sh — AC4 / ADR D7
#
# Compiles the binary at BASE_REF (default: origin/main) and the binary at the
# current tree, then compares the output of `trackfw roadmap show <name> --json`
# and `trackfw validate` over every ROADMAP-*.md under docs/roadmaps/ in the
# versioned corpus.  Any difference → rc=1.
#
# USAGE
#   bash scripts/check-marker-corpus-parity.sh [--self-test]
#   BASE_REF=origin/main bash scripts/check-marker-corpus-parity.sh
#
# INJECTION — skips compilation (useful for self-test, CI, manual contra-braço)
#   TRACKFW_BASE_BIN   path to pre-built base binary
#   TRACKFW_HEAD_BIN   path to pre-built head binary
#   CORPUS_DIR         repo root where git ls-files and both binaries run
#                      (default: git rev-parse --show-toplevel from current dir)
#
# NORMALIZATION: none — all bytes of stdout+stderr must be identical.
# The only legitimately variable quantity is the binary's exit code, which
# is included in the comparison key.
#
# LINKED TO: REQ-2026-11-02 (idioma do produto), ADR D7, Wave 1 ML-1B

set -euo pipefail

SELF=$(basename "${BASH_SOURCE[0]}")
SELF_ABS=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")

# ---------------------------------------------------------------------------
# --self-test
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--self-test" ]]; then
  _st_tmp=$(mktemp -d)
  printf -v _q_st '%q' "$_st_tmp"
  # shellcheck disable=SC2064  # intentional: value captured at installation via printf -v %q
  trap "rm -rf -- $_q_st" EXIT

  ST_PASS=0
  ST_FAIL=0

  # Run one arm.  Expects: label, expected_rc, expected_grep_pattern,
  # base_bin, head_bin, corpus_dir.
  _st_arm() {
    local label="$1" expect_rc="$2" grep_pat="$3" base="$4" head="$5" corpus="$6"
    local actual_rc=0
    local actual_out=''
    actual_out=$(
      env TRACKFW_BASE_BIN="$base" TRACKFW_HEAD_BIN="$head" CORPUS_DIR="$corpus" \
        bash "$SELF_ABS" 2>&1
    ) || actual_rc=$?
    local ok=1
    if [[ "$actual_rc" -ne "$expect_rc" ]]; then
      printf '  FAIL  %s  (expected rc=%d, got rc=%d)\n' "$label" "$expect_rc" "$actual_rc"
      ok=0
    fi
    if [[ -n "$grep_pat" ]] && ! printf '%s\n' "$actual_out" | grep -F "$grep_pat" >/dev/null; then
      printf '  FAIL  %s  (expected %q in output; got: %s)\n' \
             "$label" "$grep_pat" "$actual_out"
      ok=0
    fi
    if [[ $ok -eq 1 ]]; then
      printf '  PASS  %s\n' "$label"
      (( ST_PASS++ )) || true
    else
      (( ST_FAIL++ )) || true
    fi
  }

  # ---- fixture repo with one roadmap ----
  _fix="$_st_tmp/fixture"
  mkdir -p "$_fix/docs/roadmaps/wip"
  (
    cd "$_fix"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"
    touch "docs/roadmaps/wip/ROADMAP-st-fixture.md"
    git add .
    git commit -q -m "fixture"
  )

  # ---- fixture repo with no roadmaps (vacuity test) ----
  _empty="$_st_tmp/empty"
  mkdir -p "$_empty/docs/roadmaps/wip"
  (
    cd "$_empty"
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"
    touch "docs/roadmaps/wip/NOT-A-ROADMAP.md"
    git add .
    git commit -q -m "empty"
  )

  # ---- fake binaries ----
  # Format for roadmap show <name> --json:
  #   $1=roadmap $2=show $3=<name> $4=--json
  # Path must match what git ls-files returns for the fixture.
  #
  # Binaries used by ST-N1/P1/P2/P3 emit single-line JSON (empty waves — no
  # gates field involved, so no indentation requirement).
  # Binaries used by ST-P4/P5 must emit properly-indented multi-line JSON so
  # that the sed gate-stripper can locate the "gates" block at 6-space indent.

  _bin_base="$_st_tmp/bin-base"
  _bin_head_eq="$_st_tmp/bin-head-eq"      # identical to base
  _bin_head_show_diff="$_st_tmp/bin-head-show-diff"   # different show output
  _bin_head_val_diff="$_st_tmp/bin-head-val-diff"     # different validate output

  cat > "$_bin_base" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap) printf '{"roadmap":"%s.md","path":"docs/roadmaps/wip/%s.md","state":"wip","waves":[]}\n' "$3" "$3" ;;
  validate) echo "0 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_base"

  # equal: same as base
  cp "$_bin_base" "$_bin_head_eq"

  # positive: waves differ in show output
  cat > "$_bin_head_show_diff" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap) printf '{"roadmap":"%s.md","path":"docs/roadmaps/wip/%s.md","state":"wip","waves":[{"label":"1"}]}\n' "$3" "$3" ;;
  validate) echo "0 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_head_show_diff"

  # validate-only diff
  cat > "$_bin_head_val_diff" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap) printf '{"roadmap":"%s.md","path":"docs/roadmaps/wip/%s.md","state":"wip","waves":[]}\n' "$3" "$3" ;;
  validate) echo "1 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_head_val_diff"

  # ST-P4: both binaries emit indented JSON with the gates field, different commands.
  # Base emits "echo base", head emits "echo head" — direct comparison, gates differ → rc=1.
  _bin_p4_base="$_st_tmp/bin-p4-base"
  _bin_p4_head="$_st_tmp/bin-p4-head"

  cat > "$_bin_p4_base" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap)
    cat << BEOF
{
  "roadmap": "$3.md",
  "path": "docs/roadmaps/wip/$3.md",
  "state": "wip",
  "malformed_waves": [],
  "waves": [
    {
      "label": "1",
      "line": 1,
      "gates": {
        "commands": [
          "echo base"
        ]
      },
      "mls": []
    }
  ]
}
BEOF
    ;;
  validate) echo "0 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_p4_base"

  cat > "$_bin_p4_head" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap)
    cat << BEOF
{
  "roadmap": "$3.md",
  "path": "docs/roadmaps/wip/$3.md",
  "state": "wip",
  "malformed_waves": [],
  "waves": [
    {
      "label": "1",
      "line": 1,
      "gates": {
        "commands": [
          "echo head"
        ]
      },
      "mls": []
    }
  ]
}
BEOF
    ;;
  validate) echo "0 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_p4_head"

  # ST-P5 (counter-arm): base lacks the gates field (old binary), head has it.
  # Everything else is identical. After stripping gates from head the outputs
  # match → rc=0, and the summary line is printed.
  _bin_p5_base="$_st_tmp/bin-p5-base"
  _bin_p5_head="$_st_tmp/bin-p5-head"

  cat > "$_bin_p5_base" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap)
    cat << BEOF
{
  "roadmap": "$3.md",
  "path": "docs/roadmaps/wip/$3.md",
  "state": "wip",
  "malformed_waves": [],
  "waves": [
    {
      "label": "1",
      "line": 1,
      "mls": []
    }
  ]
}
BEOF
    ;;
  validate) echo "0 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_p5_base"

  cat > "$_bin_p5_head" << 'BINEOF'
#!/usr/bin/env bash
case "${1:-}" in
  roadmap)
    cat << BEOF
{
  "roadmap": "$3.md",
  "path": "docs/roadmaps/wip/$3.md",
  "state": "wip",
  "malformed_waves": [],
  "waves": [
    {
      "label": "1",
      "line": 1,
      "gates": {
        "commands": [
          "echo something"
        ]
      },
      "mls": []
    }
  ]
}
BEOF
    ;;
  validate) echo "0 warning(s)" ;;
esac
BINEOF
  chmod +x "$_bin_p5_head"

  echo "--- self-test: check-marker-corpus-parity ---"

  # ST-N1: identical outputs → rc=0
  # Affirms: when base and head produce identical show+validate output for all roadmaps, the script exits rc=0.
  _st_arm "ST-N1-identical-outputs" 0 "roadmaps compared" \
          "$_bin_base" "$_bin_head_eq" "$_fix"

  # ST-P1: show output diverges → rc=1 and diff output names the roadmap
  # Affirms: when head binary returns different roadmap show --json output than base, the script exits rc=1 and prints a diff.
  _st_arm "ST-P1-show-divergence" 1 "ROADMAP-st-fixture" \
          "$_bin_base" "$_bin_head_show_diff" "$_fix"

  # ST-P2: validate output diverges → rc=1
  # Affirms: when head binary returns different validate output than base, the script exits rc=1.
  _st_arm "ST-P2-validate-divergence" 1 "DIFF validate" \
          "$_bin_base" "$_bin_head_val_diff" "$_fix"

  # ST-P3: zero roadmaps in corpus → rc=1 (vacuity guard)
  # Affirms: when no ROADMAP-*.md files exist in the corpus, the script exits rc=1 to prevent silent success.
  _st_arm "ST-P3-vacuity-zero-roadmaps" 1 "vacuity" \
          "$_bin_base" "$_bin_head_eq" "$_empty"

  # ST-P4: both sides emit the gates field with different commands → rc=1.
  # Affirms: a divergence only in gates.commands (both sides have the field) is
  # detected as a real difference — the script does NOT strip when base has gates.
  _st_arm "ST-P4-gates-divergence-both-sides" 1 "ROADMAP-st-fixture" \
          "$_bin_p4_base" "$_bin_p4_head" "$_fix"

  # ST-P5: base lacks gates (old binary), head has it, rest identical → rc=0.
  # Affirms: a head-only gates field does not trigger a false failure when base
  # is an old binary — the gates block is stripped before comparison, and the
  # summary line naming the count is printed.
  _st_arm "ST-P5-old-base-gates-stripped" 0 "gates summary" \
          "$_bin_p5_base" "$_bin_p5_head" "$_fix"

  echo "---"
  printf "Results: %d passed, %d failed\n" "$ST_PASS" "$ST_FAIL"
  [[ "$ST_FAIL" -eq 0 ]]
  exit $?
fi

# ---------------------------------------------------------------------------
# Main: determine repo root and corpus directory
# ---------------------------------------------------------------------------
REPO_ROOT=$(git -C "${CORPUS_DIR:-.}" rev-parse --show-toplevel)
CORPUS_DIR="${CORPUS_DIR:-$REPO_ROOT}"

# ---------------------------------------------------------------------------
# Build binaries if not injected; set up cleanup trap
# ---------------------------------------------------------------------------
tmp=$(mktemp -d)
printf -v _q_tmp '%q' "$tmp"
printf -v _q_root '%q' "$REPO_ROOT"

# Initial trap — covers failures before worktree is created
# shellcheck disable=SC2064  # intentional: value captured at installation via printf -v %q
trap "rm -rf -- $_q_tmp" EXIT

if [[ -z "${TRACKFW_BASE_BIN:-}" || -z "${TRACKFW_HEAD_BIN:-}" ]]; then
  _base_ref="${BASE_REF:-origin/main}"
  if [[ -z "$_base_ref" ]]; then
    printf '%s: ERROR: BASE_REF is empty; specify a valid git ref\n' "$SELF" >&2
    exit 1
  fi

  if ! git -C "$REPO_ROOT" rev-parse --verify --quiet "$_base_ref^{commit}" >/dev/null 2>&1; then
    printf '%s: ERROR: BASE_REF=%s does not resolve to a commit\n' "$SELF" "$_base_ref" >&2
    exit 1
  fi

  printf '%s: building base binary from %s ...\n' "$SELF" "$_base_ref"
  if ! git -C "$REPO_ROOT" worktree add --detach "$tmp/base" "$_base_ref" >/dev/null 2>&1; then
    printf '%s: ERROR: could not create worktree for %s\n' "$SELF" "$_base_ref" >&2
    exit 1
  fi

  # Reinstall trap now that worktree exists (worktree remove must precede rm -rf)
  printf -v _q_wt '%q' "$tmp/base"
  # shellcheck disable=SC2064  # intentional: values captured at installation via printf -v %q
  trap "git -C $_q_root worktree remove --force -- $_q_wt >/dev/null 2>&1 || true; rm -rf -- $_q_tmp" EXIT

  if ! (cd "$tmp/base" && go build -o "$tmp/trackfw-base" ./cmd/trackfw 2>&1); then
    printf '%s: ERROR: go build failed for base (%s)\n' "$SELF" "$_base_ref" >&2
    exit 1
  fi

  if ! (cd "$REPO_ROOT" && go build -o "$tmp/trackfw-head" ./cmd/trackfw 2>&1); then
    printf '%s: ERROR: go build failed for head (working tree)\n' "$SELF" >&2
    exit 1
  fi

  TRACKFW_BASE_BIN="$tmp/trackfw-base"
  TRACKFW_HEAD_BIN="$tmp/trackfw-head"

  _base_sha=$(git -C "$REPO_ROOT" rev-parse --short "$_base_ref")
  _head_sha=$(git -C "$REPO_ROOT" rev-parse --short HEAD)
  printf '%s: base=%s (%s)  head=%s (%s)\n' \
         "$SELF" "$_base_ref" "$_base_sha" "HEAD" "$_head_sha"
else
  printf '%s: injected: base=%s  head=%s\n' "$SELF" "$TRACKFW_BASE_BIN" "$TRACKFW_HEAD_BIN"
fi

# ---------------------------------------------------------------------------
# Find roadmaps
# ---------------------------------------------------------------------------
ROADMAPS=()
while IFS= read -r _f; do
  # Only files whose basename starts with ROADMAP-
  case "${_f##*/}" in ROADMAP-*) ROADMAPS+=("$_f") ;; esac
done < <(git -C "$CORPUS_DIR" ls-files 'docs/roadmaps/*.md')

if [[ "${#ROADMAPS[@]}" -eq 0 ]]; then
  printf '%s: ERROR: no ROADMAP-*.md files found — vacuity guard triggered\n' "$SELF" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Compare show --json for each roadmap
# ---------------------------------------------------------------------------
FAIL=0
TOTAL=0

# Gates-field awareness counters (ML-2D, REQ-2026-11-02).
# When base binary pre-dates the gates field (old binary), the gates block is
# stripped from head output before comparison.  _OLD_BASE_SEEN flips to 1 on
# the first roadmap where base lacks the field; the per-wave stats accumulate
# across all roadmaps for the end-of-run summary.
_OLD_BASE_SEEN=0
_WAVES_WITH_CMDS=0
_WAVES_WITH_ERRORS=0

for _rel in "${ROADMAPS[@]}"; do
  _name="${_rel##*/}"   # basename with .md
  _name="${_name%.md}"  # strip .md extension

  _base_rc=0
  _base_out=$( (cd "$CORPUS_DIR" && "$TRACKFW_BASE_BIN" roadmap show "$_name" --json) 2>&1 ) \
    || _base_rc=$?

  _head_rc=0
  _head_out=$( (cd "$CORPUS_DIR" && "$TRACKFW_HEAD_BIN" roadmap show "$_name" --json) 2>&1 ) \
    || _head_rc=$?

  # Path verification: only when base resolved to JSON (rc=0) and contains "path" field
  if [[ "$_base_rc" -eq 0 ]] && printf '%s\n' "$_base_out" | grep '"path"' >/dev/null; then
    _base_path=$(printf '%s\n' "$_base_out" \
                 | { grep -o '"path": *"[^"]*"' || true; } \
                 | sed 's/"path": *"\([^"]*\)"/\1/')
    if [[ "$_base_path" != "$_rel" ]]; then
      printf '%s: ERROR: show returned path=%s, expected %s (partial-match confusion?)\n' \
             "$SELF" "$_base_path" "$_rel" >&2
      (( FAIL++ )) || true
      (( TOTAL++ )) || true
      continue
    fi
  fi

  # Build comparison keys: "rc=N\n<output>".
  # Gates-field handling: when base output contains the gates field, both sides
  # have it and any difference is real — compare as-is.  When base lacks the
  # field (old binary), strip the gates block from head before comparing so
  # that a head-only gates addition is not flagged as a regression.
  #
  # The sed range relies on waveJSON struct ordering (see roadmap_show_json.go):
  #   "gates": {    appears at 6-space indent (between "line" and "mls")
  #   },            closing at 6-space (comma present because "mls" always follows)
  if [[ "$_base_rc" -eq 0 ]] && printf '%s\n' "$_base_out" | grep -F '"gates": {' >/dev/null; then
    # Both sides have the gates field: direct comparison, any diff is real.
    _base_key=$(printf 'rc=%d\n%s' "$_base_rc" "$_base_out")
    _head_key=$(printf 'rc=%d\n%s' "$_head_rc" "$_head_out")
  else
    # Old base (no gates field): strip the gates block from head, then compare.
    _OLD_BASE_SEEN=1
    _head_stripped_rc=0
    _head_stripped=$(printf '%s\n' "$_head_out" \
        | sed '/^      "gates": {/,/^      },/d') || _head_stripped_rc=$?
    if [[ "$_head_stripped_rc" -ne 0 ]]; then
      _head_stripped="$_head_out"
    fi
    _base_key=$(printf 'rc=%d\n%s' "$_base_rc" "$_base_out")
    _head_key=$(printf 'rc=%d\n%s' "$_head_rc" "$_head_stripped")

    # Accumulate per-wave gates stats from head output for the summary report.
    # grep -c returns rc=1 when there are no matches; the '|| true' guard inside
    # the braces prevents set -e from aborting — the value "0" is the correct result.
    _cnt_all=$( { printf '%s\n' "$_head_out" | grep -cF '"commands": [' || true; } )
    _cnt_empty=$( { printf '%s\n' "$_head_out" | grep -cF '"commands": []' || true; } )
    _cnt_err=$( { printf '%s\n' "$_head_out" | grep -cF '"error":' || true; } )
    (( _WAVES_WITH_CMDS += _cnt_all - _cnt_empty )) || true
    (( _WAVES_WITH_ERRORS += _cnt_err )) || true
  fi

  if [[ "$_base_key" != "$_head_key" ]]; then
    printf '=== DIFF show --json: %s ===\n' "$_rel"
    if ! diff -u \
        <(printf '%s\n' "$_base_key") \
        <(printf '%s\n' "$_head_key"); then
      true
    fi
    (( FAIL++ )) || true
  fi

  (( TOTAL++ )) || true
done

# ---------------------------------------------------------------------------
# Compare validate (once)
# ---------------------------------------------------------------------------
_val_base_rc=0
_val_base_out=$( (cd "$CORPUS_DIR" && "$TRACKFW_BASE_BIN" validate) 2>&1 ) || _val_base_rc=$?

_val_head_rc=0
_val_head_out=$( (cd "$CORPUS_DIR" && "$TRACKFW_HEAD_BIN" validate) 2>&1 ) || _val_head_rc=$?

_val_base_key=$(printf 'rc=%d\n%s' "$_val_base_rc" "$_val_base_out")
_val_head_key=$(printf 'rc=%d\n%s' "$_val_head_rc" "$_val_head_out")

if [[ "$_val_base_key" != "$_val_head_key" ]]; then
  printf '=== DIFF validate ===\n'
  if ! diff -u \
      <(printf '%s\n' "$_val_base_key") \
      <(printf '%s\n' "$_val_head_key"); then
    true
  fi
  (( FAIL++ )) || true
fi

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------
if [[ "$FAIL" -gt 0 ]]; then
  printf '%s: FAIL — %d difference(s) found (%d roadmaps compared)\n' "$SELF" "$FAIL" "$TOTAL" >&2
  exit 1
fi

printf '%s: %d roadmaps compared, validate identical\n' "$SELF" "$TOTAL"
if [[ "$_OLD_BASE_SEEN" -eq 1 ]]; then
  printf '%s: head gates summary (base lacked gates field): %d wave(s) with commands, %d wave(s) with errors\n' \
         "$SELF" "$_WAVES_WITH_CMDS" "$_WAVES_WITH_ERRORS"
fi
exit 0
