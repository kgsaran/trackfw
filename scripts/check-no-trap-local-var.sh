#!/usr/bin/env bash
# check-no-trap-local-var.sh — trap com body em aspas simples não pode citar variável
# declarada local na mesma função; a variável é desvinculada antes do EXIT trap disparar
# (bash 5) ou na saída normal da função (ambas as versões), causando 'unbound variable'
# e vazamento do diretório temporário.
#
# Linked to: make parity-rest (REQ-2026-11-07, #671)
#
# WHY THIS EXISTS — REQ-2026-11-07, Wave 0:
#   Under 'set -u', 'trap '"'"'...$VAR...'"'"' EXIT' inside a function fails when VAR is
#   declared with 'local': the local frame is destroyed before the EXIT trap fires (bash 5)
#   or when the function returns normally (both versions). The fix: capture the value at
#   install time with  local q; printf -v q '"'"'%q'"'"' "$VAR"; trap "rm -rf $q" EXIT
#
# RESIDUALS (not detected by this gate):
#   R2: trap body calls a helper function that uses VAR via dynamic scope (body has no $VAR)
#   R3: ${VAR:-fallback} with non-empty fallback — the gate flags ${VAR:-} (empty fallback)
#       but NOT ${VAR:-other}, which still leaks silently; declare in code review
#   R4: trap '"'"'...$VAR...'"'"' RETURN — same mechanism; not in corpus; declared residual
#
# USAGE
#   bash scripts/check-no-trap-local-var.sh           # scan git ls-files '*.sh'
#   bash scripts/check-no-trap-local-var.sh --self-test
#   bash scripts/check-no-trap-local-var.sh FILE...   # scan specific files

set -uo pipefail

SELF=$(basename "${BASH_SOURCE[0]}")

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
MODE=scan
if [[ "${1:-}" == "--self-test" ]]; then
  MODE=self-test
  shift
fi
FILES=()
while [[ $# -gt 0 ]]; do
  FILES+=("$1"); shift
done

# ---------------------------------------------------------------------------
# AWK program
# single-quote is passed via  -v sq="'"
#
# Design: deferred trap checking.
#   Traps are stored when encountered inside a function.
#   At function exit, all stored traps are checked against the FULL locals set.
#   This correctly handles locals declared either before or after the trap.
# ---------------------------------------------------------------------------
read -r -d '' AWK_PROG << 'AWKEOF' || true
BEGIN { violations = 0; all_traps = 0 }

FNR == 1 {
    in_func      = 0
    brace_depth  = 0
    in_heredoc   = 0
    heredoc_delim = ""
    heredoc_strip = 0
    for (k in lvars)   delete lvars[k]
    for (k in t_line)  delete t_line[k]
    for (k in t_body)  delete t_body[k]
    n_traps = 0
    pending_func = 0
}

# --- Heredoc skip ---
in_heredoc == 1 {
    testline = $0
    if (heredoc_strip) sub(/^\t*/, "", testline)
    if (testline == heredoc_delim) in_heredoc = 0
    next
}

# --- Pure comment lines ---
/^[[:space:]]*#/ { next }

{
    line = $0

    # Count ALL trap occurrences in file (vacuity guard — any scope)
    tmp_line = line
    if (match(tmp_line, /trap[[:space:]]/)) {
        prev_c = (RSTART > 1) ? substr(tmp_line, RSTART-1, 1) : ""
        if (prev_c !~ /[A-Za-z0-9_]/) all_traps++
    }

    # Detect heredoc start: << not followed by < (avoid <<<)
    if (!in_heredoc) {
        tmp2 = line
        gsub(/<<</, "HHH", tmp2)
        if (match(tmp2, /<<-?/)) {
            hd_m = substr(tmp2, RSTART, RLENGTH)
            heredoc_strip = (hd_m == "<<-")
            after = substr(line, RSTART + RLENGTH)
            sub(/^[[:space:]]*/, "", after)
            if (substr(after, 1, 1) == sq) {
                p = index(substr(after, 2), sq)
                if (p > 0) heredoc_delim = substr(after, 2, p - 1)
            } else if (substr(after, 1, 1) == "\"") {
                p = index(substr(after, 2), "\"")
                if (p > 0) heredoc_delim = substr(after, 2, p - 1)
            } else {
                if (match(after, /^[A-Za-z_][A-Za-z0-9_]*/))
                    heredoc_delim = substr(after, RSTART, RLENGTH)
            }
            in_heredoc = 1
        }
    }

    # --- Pending function: waiting for opening { ---
    if (pending_func && !in_func) {
        if (match(line, /^[[:space:]]*\{/)) {
            in_func = 1; brace_depth = 1
            for (k in lvars) delete lvars[k]
            for (k in t_line) delete t_line[k]
            for (k in t_body) delete t_body[k]
            n_traps = 0
            pending_func = 0
        } else if (line !~ /^[[:space:]]*$/) {
            pending_func = 0
        }
        next
    }

    # --- Function header detection (only when not inside a function) ---
    if (!in_func) {
        is_func = 0; is_subshell = 0
        if (match(line, /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*\(\)/)) {
            is_func = 1
            after_paren = substr(line, RSTART + RLENGTH)
            sub(/^[[:space:]]*/, "", after_paren)
            if (substr(after_paren, 1, 1) == "(") is_subshell = 1
        } else if (match(line, /^[[:space:]]*function[[:space:]]+[A-Za-z_][A-Za-z0-9_]*/)) {
            is_func = 1
        }
        if (is_func && !is_subshell) {
            if (index(line, "{") > 0) {
                in_func = 1; brace_depth = 0
                for (k in lvars) delete lvars[k]
                for (k in t_line) delete t_line[k]
                for (k in t_body) delete t_body[k]
                n_traps = 0
            } else {
                pending_func = 1
            }
        }
        if (!in_func) next
    }

    # --- Within function ---
    brace_depth += brace_net(line)
    if (brace_depth <= 0) {
        # Function exit: check all deferred traps against full locals set
        flush_traps(FILENAME)
        in_func = 0; brace_depth = 0
        for (k in lvars)  delete lvars[k]
        for (k in t_line) delete t_line[k]
        for (k in t_body) delete t_body[k]
        n_traps = 0
        next
    }

    collect_lvars(line)
    store_trap(line, FNR)
}

END {
    if (all_traps == 0) {
        printf "ERROR: no trap statements found — vacuity guard triggered\n" > "/dev/stderr"
        exit 1
    }
}

# Count net { - } in line, ignoring content inside quotes and # comments
function brace_net(s,    n, i, c, in_sq, in_dq, net) {
    n = length(s); in_sq = 0; in_dq = 0; net = 0
    for (i = 1; i <= n; i++) {
        c = substr(s, i, 1)
        if (in_sq) {
            if (c == sq) in_sq = 0
        } else if (in_dq) {
            if (c == "\\") { i++ }
            else if (c == "\"") in_dq = 0
        } else if (c == "#") {
            break
        } else if (c == sq) {
            in_sq = 1
        } else if (c == "\"") {
            in_dq = 1
        } else if (c == "{") {
            net++
        } else if (c == "}") {
            net--
        }
    }
    return net
}

# Parse local/declare/typeset declarations and add var names to lvars[]
function collect_lvars(line,    flags_r, n, i, t, v) {
    # Match keyword at start of line (optional whitespace) or after semicolon
    if (match(line, /^[[:space:]]*(local|declare|typeset)[[:space:]]+/)) {
        flags_r = substr(line, RSTART + RLENGTH)
    } else if (match(line, /;[[:space:]]*(local|declare|typeset)[[:space:]]+/)) {
        flags_r = substr(line, RSTART + RLENGTH)
    } else {
        return
    }

    # Skip if has -g option (global) in any option cluster
    while (match(flags_r, /^-[A-Za-z]+[[:space:]]*/)) {
        flag_cl = substr(flags_r, 1, RLENGTH)
        if (index(flag_cl, "g") > 0) return
        flags_r = substr(flags_r, RSTART + RLENGTH)
    }

    # Split remaining text on whitespace to get name=value tokens
    n = split(flags_r, toks, /[[:space:]]+/)
    for (i = 1; i <= n; i++) {
        t = toks[i]
        if (substr(t, 1, 1) == "#") break
        if (match(t, /^[A-Za-z_][A-Za-z0-9_]*/)) {
            v = substr(t, 1, RLENGTH)
            if (v != "" && v != "local" && v != "declare" && v != "typeset")
                lvars[v] = 1
        }
    }
}

# Store a single-quoted EXIT/0 trap for deferred checking at function exit
function store_trap(line, lineno,    rest, bdy_end, bdy, after, sigs, n, i, has_exit) {
    if (!match(line, /trap[[:space:]]+/)) return
    pos = RSTART
    if (pos > 1 && substr(line, pos-1, 1) ~ /[A-Za-z0-9_]/) return
    rest = substr(line, pos + RLENGTH)
    if (substr(rest, 1, 1) != sq) return

    bdy_end = index(substr(rest, 2), sq)
    if (bdy_end == 0) return
    bdy = substr(rest, 2, bdy_end - 1)
    after = substr(rest, bdy_end + 2)
    sub(/^[[:space:]]+/, "", after)

    n = split(after, sigs, /[[:space:]]+/)
    has_exit = 0
    for (i = 1; i <= n; i++) {
        if (sigs[i] == "EXIT" || sigs[i] == "0") { has_exit = 1; break }
    }
    if (!has_exit) return

    n_traps++
    t_line[n_traps] = lineno
    t_body[n_traps] = bdy
}

# At function exit: check all deferred traps against full locals set
function flush_traps(fname,    j, v) {
    for (j = 1; j <= n_traps; j++) {
        for (v in lvars) {
            if (body_refs_var(t_body[j], v)) {
                printf "%s:%d:%s\n", fname, t_line[j], v
                violations++
            }
        }
    }
}

# Returns 1 if body contains $V or ${V} or ${V:-} with word boundary after V
function body_refs_var(body, v,    n, i, c, vlen, next_c) {
    n = length(body); vlen = length(v)
    for (i = 1; i <= n; i++) {
        c = substr(body, i, 1)
        if (c != "$") continue
        if (substr(body, i+1, 1) == "{") {
            if (substr(body, i+2, vlen) == v) {
                next_c = substr(body, i+2+vlen, 1)
                if (next_c == "}" || next_c == ":") return 1
            }
        } else {
            if (substr(body, i+1, vlen) == v) {
                next_c = substr(body, i+1+vlen, 1)
                if (next_c !~ /[A-Za-z0-9_]/) return 1
            }
        }
    }
    return 0
}
AWKEOF

# ---------------------------------------------------------------------------
# --self-test: fixture-based tests
# ---------------------------------------------------------------------------
if [[ "$MODE" == "self-test" ]]; then
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  PASS=0
  FAIL=0

  run_case() {
    local name="$1" content="$2" expect_v="$3"
    shift 3
    local expected_vars=("$@")
    local f="$tmpdir/${name}.sh"
    printf '%s\n' "$content" > "$f"
    local out rc=0
    out=$(bash "${BASH_SOURCE[0]}" "$f" 2>&1) || rc=$?
    local ok=1
    if [[ "$expect_v" -eq 0 ]]; then
      if [[ $rc -ne 0 ]]; then
        echo "  FAIL  $name  (expected rc=0, got rc=$rc; output: $out)"
        ok=0
      fi
    else
      if [[ $rc -eq 0 ]]; then
        echo "  FAIL  $name  (expected violations, got rc=0)"
        ok=0
      fi
      for v in "${expected_vars[@]}"; do
        if ! printf '%s\n' "$out" | grep -- ":${v}$" >/dev/null; then
          echo "  FAIL  $name  (expected var '$v' in output; got: $out)"
          ok=0
        fi
      done
    fi
    if [[ $ok -eq 1 ]]; then
      echo "  PASS  $name"
      (( PASS++ )) || true
    else
      (( FAIL++ )) || true
    fi
    rm -f "$f"
  }

  echo "--- self-test: check-no-trap-local-var ---"

  # --- POSITIVE cases (must be flagged) ---

  # P1: basic $VAR form
  # Reconciliation: affirms the fundamental case that causes unbound variable error (Wave 0 §2 Caminho 2a/2b)
  run_case "P1-basic-dollar-var" \
'f() {
  local tmp
  tmp=$(mktemp -d)
  trap '"'"'rm -rf "$tmp"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "tmp"

  # P2: ${VAR} form with braces
  # Reconciliation: affirms threat model FN-1 — ${VAR} form also triggers the unbound error
  run_case "P2-brace-var" \
'f() {
  local DIR
  DIR=$(mktemp -d)
  trap '"'"'rm -rf "${DIR}"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "DIR"

  # P3: ${VAR:-} empty default form
  # Reconciliation: affirms threat model R3 — ${VAR:-} produces silent leak (rm -rf "" is no-op on macOS)
  run_case "P3-var-default-empty" \
'f() {
  local D
  D=$(mktemp -d)
  trap '"'"'rm -rf "${D:-}"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "D"

  # P4: signal 0 (alias for EXIT)
  # Reconciliation: affirms threat model §5 — signal 0 is functionally identical to EXIT
  run_case "P4-signal-zero" \
'g() {
  local W
  W=$(mktemp -d)
  trap '"'"'rm -rf "$W"'"'"' 0
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "W"

  # P5: signal list including EXIT
  # Reconciliation: affirms threat model §5 — EXIT anywhere in a signal list is detected
  run_case "P5-signal-list" \
'g() {
  local W
  W=$(mktemp -d)
  trap '"'"'rm -rf "$W"'"'"' EXIT HUP INT TERM
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "W"

  # P6: local -r VAR (read-only)
  # Reconciliation: affirms threat model FN-2 — local -r has identical frame lifetime to local
  run_case "P6-local-r" \
'h() {
  local -r FENCE
  FENCE=$(mktemp -d)
  trap '"'"'rm -rf "$FENCE"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "FENCE"

  # P7: declare VAR without -g
  # Reconciliation: affirms threat model FN-3 — declare without -g creates function-local variable
  run_case "P7-declare" \
'h() {
  declare MYVAR
  MYVAR=$(mktemp -d)
  trap '"'"'rm -rf "$MYVAR"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "MYVAR"

  # P8: typeset VAR
  # Reconciliation: affirms threat model FN-4 — typeset is a synonym for declare; same local lifetime
  run_case "P8-typeset" \
'h() {
  typeset TV
  TV=$(mktemp -d)
  trap '"'"'rm -rf "$TV"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "TV"

  # P9: function keyword style
  # Reconciliation: affirms threat model FN-5 — both name() and function name styles are detected
  run_case "P9-function-keyword" \
'function myfunc {
  local VAR
  VAR=$(mktemp -d)
  trap '"'"'rm -rf "$VAR"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "VAR"

  # P10: local declared AFTER trap (deferred check catches it)
  # Reconciliation: affirms threat model FN-6 — bash local scope is function-wide (not ordered);
  # deferred checking at function exit uses the full locals set
  run_case "P10-local-after-trap" \
'f() {
  trap '"'"'rm -rf "$LATEVAR"'"'"' EXIT
  local LATEVAR
  LATEVAR=$(mktemp -d)
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "LATEVAR"

  # P11: word boundary — $s must NOT match inside $saved_github_output
  # Reconciliation: word-boundary check prevents false positives from prefix matches
  run_case "P11-word-boundary-not-prefix" \
'f() {
  local s
  local saved_other
  s=$(mktemp -d)
  saved_other=$(mktemp -d)
  trap '"'"'rm -rf "$s" "$saved_other"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "s"

  # P12: trap after a one-liner nested function (outer scope persists, outer brace_depth unchanged)
  # Reconciliation: net brace count for one-liner inner functions is 0;
  # the outer scope (and its trap violation) is correctly detected via deferred check
  run_case "P12-nested-oneliner-then-trap" \
'run_self_test() {
  local tmp
  tmp=$(mktemp -d)
  inner() { echo ok; }
  trap '"'"'rm -rf "$tmp"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "tmp"

  # P13: trap after a heredoc inside the function (heredoc braces must not confuse scope)
  # Reconciliation: the gate skips heredoc content; {/} inside heredoc do not affect brace_depth
  run_case "P13-trap-after-heredoc" \
'f() {
  local D
  D=$(mktemp -d)
  cat > /dev/null << '"'"'EOF'"'"'
this has { and } inside
func g() { return; }
EOF
  trap '"'"'rm -rf "$D"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 1 "D"

  # --- NEGATIVE cases (must NOT be flagged) ---

  # N1: trap at script level — global var is safe; vacuity guard must NOT fire (trap IS in corpus)
  # Reconciliation: gate only tracks locals inside functions; script-level vars are not local
  run_case "N1-script-level-trap" \
'GTMP=$(mktemp -d)
trap '"'"'rm -rf "$GTMP"'"'"' EXIT' 0

  # N2: trap with double-quoted body (expansion at install time)
  # Reconciliation: double-quoted body expands when trap is installed; value is captured correctly
  run_case "N2-double-quote-body" \
'f() {
  local q
  printf -v q '"'"'%q'"'"' "$(mktemp -d)"
  trap "rm -rf -- $q" EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 0

  # N3: declare -g VAR (explicitly global)
  # Reconciliation: -g flag makes the variable global; it persists after function returns
  run_case "N3-declare-g" \
'f() {
  declare -g GVAR
  GVAR=$(mktemp -d)
  trap '"'"'rm -rf "$GVAR"'"'"' EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 0

  # N4: subshell function f() ( ... ) — EXIT trap fires inside subshell with intact scope
  # Reconciliation: subshell functions run in a child process; local vars persist until subshell exit;
  # the gate does not enter tracking for subshell functions
  run_case "N4-subshell-func" \
'f() (
  local S
  S=$(mktemp -d)
  trap '"'"'rm -rf "$S"'"'"' EXIT
)
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 0

  # N5: global var with same name as a local in a different function — no cross-contamination
  # Reconciliation: gate resets local var tracking per function; no false positive from earlier scope
  run_case "N5-global-same-name-different-func" \
'WORK=$(mktemp -d)
trap '"'"'rm -rf "$WORK"'"'"' EXIT
other() {
  local WORK
  WORK=$(mktemp -d)
}' 0

  # N6: trap with ERR signal (not EXIT/0) — residual R4
  # Reconciliation: gate targets only EXIT and 0 signals; ERR is not in scope
  run_case "N6-trap-err-signal" \
'f() {
  local E
  E=$(mktemp -d)
  trap '"'"'rm -rf "$E"'"'"' ERR
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 0

  # N7: the fix form itself — printf %q captures value at install time (double-quoted body)
  # Reconciliation: the fixed form is correctly not flagged; it is the recommended replacement
  run_case "N7-fix-form-printf-q" \
'run_self_test() {
  local tmp
  tmp=$(mktemp -d)
  local q
  printf -v q '"'"'%q'"'"' "$tmp"
  trap "rm -rf -- $q" EXIT
}
sentinel_trap() { trap '"'"'echo ok'"'"' EXIT; }' 0

  # --- VACUITY GUARD ---
  # Vac1: corpus with no trap statements → gate fails
  # Reconciliation: vacuity guard prevents silent success on a corpus that has no traps to check
  vac_f="$tmpdir/vac_no_trap.sh"
  printf '#!/usr/bin/env bash\necho hello\n' > "$vac_f"
  vac_rc=0
  bash "${BASH_SOURCE[0]}" "$vac_f" >/dev/null 2>&1 || vac_rc=$?
  if [[ $vac_rc -ne 0 ]]; then
    echo "  PASS  Vac1-no-trap-corpus (vacuity rc=$vac_rc)"
    (( PASS++ )) || true
  else
    echo "  FAIL  Vac1-no-trap-corpus (expected rc!=0 from vacuity guard)"
    (( FAIL++ )) || true
  fi
  rm -f "$vac_f"

  echo "---"
  printf "Results: %d passed, %d failed\n" "$PASS" "$FAIL"
  if [[ $FAIL -gt 0 ]]; then exit 1; fi
  exit 0
fi

# ---------------------------------------------------------------------------
# Main: scan files for trap-local-var violations
# ---------------------------------------------------------------------------
if [[ ${#FILES[@]} -gt 0 ]]; then
  SCAN_FILES=("${FILES[@]}")
else
  SCAN_FILES=()
  while IFS= read -r f; do
    SCAN_FILES+=("$f")
  done < <(git ls-files '*.sh' 2>/dev/null)
fi

if [[ ${#SCAN_FILES[@]} -eq 0 ]]; then
  printf '%s: ERROR: no .sh files to scan\n' "$SELF" >&2
  exit 2
fi

VIOLATIONS=0
AWK_OUT=""
AWK_RC=0
AWK_OUT=$(awk -v sq="'" "$AWK_PROG" "${SCAN_FILES[@]}" 2>&1) || AWK_RC=$?

if [[ $AWK_RC -ne 0 ]]; then
  printf '%s\n' "$AWK_OUT" >&2
  exit 1
fi

if [[ -n "$AWK_OUT" ]]; then
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    echo "$line"
    (( VIOLATIONS++ )) || true
  done <<< "$AWK_OUT"
fi

if [[ $VIOLATIONS -gt 0 ]]; then
  printf '%s: FAIL — %d trap-local-var violation(s) found\n' "$SELF" "$VIOLATIONS" >&2
  exit 1
fi

printf '%s: OK — no trap-local-var violations found\n' "$SELF"
exit 0
