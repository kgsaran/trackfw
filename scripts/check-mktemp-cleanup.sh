#!/usr/bin/env bash
# check-mktemp-cleanup.sh — ADR D2: every V=$(mktemp …) in a tracked .sh file must have
# guaranteed EXIT cleanup in the same scope: a trap … EXIT (or a function called by one)
# that removes V directly, via alias, via containment under a covered dir, or via a
# one-level-deep function that cites V or an outer-scope alias of V.
#
# Linked to: make parity-rest  (REQ-2026-11-11-mktemp-sem-limpeza-garantida, #676)
#
# WHY THIS EXISTS — ADR D2:
#   mktemp without a guaranteed EXIT trap leaks temporary files when the script exits
#   between the mktemp call and the explicit rm in the normal path (set -euo pipefail
#   makes this common).  A bare rm in the normal path is NOT guaranteed cleanup.
#
# COVERAGE MECHANISMS (ADR D2):
#   DIRECT   — trap body cites $V, ${V}, ${V:-} or ${V:-x}
#   ALIAS    — printf -v A '%q' "$V" or A=$V or A="$V" before trap; trap cites $A
#   FUNCTION — trap calls function whose body cites $V or an outer-scope alias of V
#   CONTAINED— V created as mktemp … "$W/…" and W is covered
#
# LAST-TRAP RULE (ADR Consequences):
#   In a function: only the last trap … EXIT in that function scope matters.  A covering
#   trap superseded by a non-covering one is treated as UNCOVERED (fail-trap-replaced-fn).
#   When the function has ANY EXIT trap it supersedes all inherited global traps — bash
#   `trap … EXIT` in a function replaces the inherited handler.
#   At global scope: the last global EXIT trap wins, with one exception: if a global-scope
#   `exit` statement falls between an earlier covering trap and a later superseding trap,
#   the earlier trap is treated as effective (exit-boundary exception for multi-path
#   scripts with mutually exclusive self-test vs main paths).
#
# RESIDUALS (not detected by this gate):
#   R1  trap inside a subshell function called as d=$(fn) — cleanup fires before the
#       caller captures the value; no static discriminant (FN-9)
#   R2  loop reassignment — only the last creation is covered (FN-10)
#   R3  exec CMD after a trapped var replaces the shell without firing EXIT (FN-7)
#   R4  two-level function indirection: trap → wrapper() → cleanup() → $V (FN-4b)
#   R5  case-branch `exit` between global covering and superseding traps: only a bare
#       `exit` at the start of a line in global scope is detected as a path boundary
#
# USAGE
#   bash scripts/check-mktemp-cleanup.sh           # scan git ls-files '*.sh'
#   bash scripts/check-mktemp-cleanup.sh --self-test
#   bash scripts/check-mktemp-cleanup.sh FILE...   # scan specific files

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
EXPLICIT_FILES=()
while [[ $# -gt 0 ]]; do
  EXPLICIT_FILES+=("$1"); shift
done

# ---------------------------------------------------------------------------
# AWK program — single pass, per-file deferred resolution
# ---------------------------------------------------------------------------
read -r -d '' AWK_PROG << 'AWKEOF' || true
BEGIN {
    sq = "'"
    violations = 0
    total_candidates = 0
}

# ---- On first line of a new file: flush previous file, reset state ----
FNR == 1 {
    if (NR > 1) flush_file()
    reset_file()
    cur_file = FILENAME
}

# ---- Multi-line single-quote string: skip, but track SQ state correctly ----
# The whole line is inside a SQ argument (e.g., a multi-line fixture passed to run_case).
# We must still compute the correct SQ state at end-of-line using update_sq() so that
# lines with '"'"' quoting sequences (SQ-close, DQ-open, literal-sq, DQ-close, SQ-open)
# leave in_sq at the right value for subsequent lines — otherwise the lines following
# the first '"'"' in a fixture would be processed as real code (FP source: §check-no-trap-local-var).
in_sq {
    in_sq = update_sq($0, in_sq)
    # Still track brace depth in case function scope closes mid-SQ (rare but correct)
    if (in_func) {
        brace_depth += brace_net($0)
        if (brace_depth <= 0) func_close()
    }
    next
}

{
    line_start_sq = in_sq   # SQ state at start of this line (used for in_sq_at checks)
    orig_line = $0

    # ---- Heredoc skip ----
    # Do NOT call update_sq here: heredoc text is literal (like a SQ string) and must
    # not affect the outer shell SQ state. An apostrophe inside a heredoc body (e.g.,
    # "Don't") would otherwise flip in_sq and silently suppress mktemp detection on
    # subsequent lines.
    if (in_heredoc) {
        test_line = $0
        if (heredoc_strip) sub(/^\t*/, "", test_line)
        if (test_line == heredoc_delim) in_heredoc = 0
        next
    }

    # ---- Pure comment lines ----
    if (/^[[:space:]]*#/) {
        in_sq = update_sq($0, in_sq)
        next
    }

    # ---- Detect heredoc start ----
    detect_heredoc($0)

    # ---- Pending function (waiting for opening { on its own line) ----
    if (pending_func) {
        stripped = $0
        sub(/^[[:space:]]*/, "", stripped)
        if (substr(stripped, 1, 1) == "{") {
            in_func = 1; brace_depth = 1; cur_scope = pending_func_name
            pending_func = 0; pending_func_name = ""
        } else if (stripped != "") {
            pending_func = 0; pending_func_name = ""
        }
        in_sq = update_sq($0, in_sq)
        next
    }

    # ---- Function scope processing ----
    if (in_func) {
        process_scope_line($0, cur_scope, line_start_sq)
        brace_depth += brace_net($0)
        if (brace_depth <= 0) func_close()
    } else {
        # Global scope: check for function header first
        if (try_func_start($0, line_start_sq)) {
            # Function started; if it was a one-liner, it's already closed
        } else {
            process_scope_line($0, "global", line_start_sq)
        }
    }

    in_sq = update_sq($0, in_sq)
}

END {
    flush_file()
    if (total_candidates < min_candidates) {
        printf "ERROR: vacuity guard — only %d mktemp candidate(s) found (floor=%d)\n",
               total_candidates, min_candidates > "/dev/stderr"
        exit 1
    }
    if (violations > 0) exit 1
}

# ============================================================
# HELPER FUNCTIONS
# ============================================================

# Update in_sq state after processing one line
function update_sq(line, cur_sq,    n, i, c, in_dq) {
    n = length(line); in_dq = 0
    for (i = 1; i <= n; i++) {
        c = substr(line, i, 1)
        if (cur_sq) {
            if (c == sq) cur_sq = 0
        } else if (in_dq) {
            if (c == "\\") i++
            else if (c == "\"") in_dq = 0
        } else if (c == "#") {
            break
        } else if (c == sq) {
            cur_sq = 1
        } else if (c == "\"") {
            in_dq = 1
        }
    }
    return cur_sq
}

# Return 1 if position pos (1-based) in line is inside a single-quoted string
# initial_sq: sq state at start of line
function in_sq_at(line, pos, initial_sq,    cur, in_dq, i, c) {
    cur = initial_sq; in_dq = 0
    for (i = 1; i < pos; i++) {
        c = substr(line, i, 1)
        if (cur) {
            if (c == sq) cur = 0
        } else if (in_dq) {
            if (c == "\\") i++
            else if (c == "\"") in_dq = 0
        } else if (c == "#") {
            return cur
        } else if (c == sq) {
            cur = 1
        } else if (c == "\"") {
            in_dq = 1
        }
    }
    return cur
}

# Count net { - } in line, respecting quotes
function brace_net(s,    n, i, c, in_sq2, in_dq, net) {
    n = length(s); in_sq2 = 0; in_dq = 0; net = 0
    for (i = 1; i <= n; i++) {
        c = substr(s, i, 1)
        if (in_sq2) {
            if (c == sq) in_sq2 = 0
        } else if (in_dq) {
            if (c == "\\") i++
            else if (c == "\"") in_dq = 0
        } else if (c == "#") {
            break
        } else if (c == sq) {
            in_sq2 = 1
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

# Check if body string cites variable v: $v, ${v}, ${v:...}
function body_refs_var(body, v,    n, vlen, i, c, next_c) {
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

# Detect heredoc start: set in_heredoc, heredoc_delim, heredoc_strip
function detect_heredoc(line,    tmp2, m_pos, m_len, after, p, stripped_flag) {
    if (in_heredoc) return
    tmp2 = line
    gsub(/<<</, "HHH", tmp2)
    if (!match(tmp2, /<<-?/)) return
    m_pos = RSTART; m_len = RLENGTH
    stripped_flag = (substr(tmp2, m_pos, m_len) == "<<-")
    after = substr(line, m_pos + m_len)
    sub(/^[[:space:]]*/, "", after)
    heredoc_delim = ""
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
    if (heredoc_delim != "") {
        in_heredoc = 1
        heredoc_strip = stripped_flag
    }
}

# Try to detect and start a function; return 1 if function started
function try_func_start(line, line_sq,    fname, rest, bd) {
    # Pattern 1: name()
    if (match(line, /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*\(\)/)) {
        if (in_sq_at(line, RSTART, line_sq)) return 0
        fname = substr(line, RSTART, RLENGTH)
        sub(/[[:space:]]*\(\)/, "", fname)
        sub(/^[[:space:]]*/, "", fname)
        rest = substr(line, RSTART + RLENGTH)
        sub(/^[[:space:]]*/, "", rest)
        if (substr(rest, 1, 1) == "(") {
            # Subshell func f() (...) — skip, vars in subshell not relevant to parent
            return 0
        }
        if (index(line, "{") > 0) {
            in_func = 1; cur_scope = fname; brace_depth = 0
            brace_depth = brace_net(line)
            if (brace_depth <= 0) {
                # One-liner: process body, then close
                process_scope_line(line, fname, line_sq)
                func_close()
            }
        } else {
            pending_func = 1; pending_func_name = fname
        }
        return 1
    }
    # Pattern 2: function name {
    if (match(line, /^[[:space:]]*function[[:space:]]+[A-Za-z_][A-Za-z0-9_]*/)) {
        if (in_sq_at(line, RSTART, line_sq)) return 0
        fname = substr(line, RSTART, RLENGTH)
        sub(/^[[:space:]]*function[[:space:]]+/, "", fname)
        in_func = 1; cur_scope = fname
        brace_depth = brace_net(line)
        if (brace_depth <= 0) {
            process_scope_line(line, fname, line_sq)
            func_close()
        }
        return 1
    }
    return 0
}

# Close current function scope
function func_close() {
    in_func = 0; cur_scope = "global"; brace_depth = 0
}

# Process a line within scope (global or function name)
function process_scope_line(line, scope, lsq) {
    # 1. Collect function body text (for FUNCTION coverage detection)
    if (scope != "global") {
        func_body[scope] = func_body[scope] "\n" line
    }

    # 2. Check for mktemp assignment: VAR=$(mktemp ...) or VAR="$(mktemp ...)"
    try_store_mktemp(line, scope, lsq)

    # 3. Check for alias: printf -v A '%q' "$V" or A=$V or A="$V"
    try_store_alias_printf(line, scope, lsq)
    try_store_alias_assign(line, scope, lsq)

    # 4. Check for trap EXIT
    try_store_trap(line, scope, lsq)

    # 5. Track global-scope `exit` statements (needed for exit-boundary analysis in
    #    global_covered()).  Only at global scope; not inside functions.
    if (scope == "global") {
        if (match(line, /^[[:space:]]*exit/)) {
            ec = substr(line, RSTART + RLENGTH, 1)
            if (ec == "" || ec !~ /[A-Za-z0-9_]/) {
                n_global_exits++
                global_exit_line[n_global_exits] = FNR
            }
        }
    }
}

# Detect and store mktemp assignment in current scope
function try_store_mktemp(line, scope, lsq,    pos, matched, varname, rest, ctr) {
    # Pattern: NAME=$(mktemp...) or NAME="$(mktemp...)"
    # Must have NAME= before $(mktemp; FP-4(a): line must contain $(
    if (index(line, "$(") == 0) return
    if (!match(line, /[A-Za-z_][A-Za-z0-9_]*[[:space:]]*=[[:space:]]*"?\$\(mktemp/)) return
    pos = RSTART
    if (in_sq_at(line, pos, lsq)) return

    matched = substr(line, RSTART, RLENGTH)
    varname = matched
    sub(/[[:space:]]*=.*/, "", varname)
    sub(/^[[:space:]]*/, "", varname)

    # Detect containment: mktemp arg contains $VAR/ or -p $VAR
    rest = substr(line, RSTART + RLENGTH)   # text after "mktemp"
    # Find closing ) for mktemp args
    ctr = 0; args = ""
    for (ci = 1; ci <= length(rest); ci++) {
        ch = substr(rest, ci, 1)
        if (ch == "(") ctr++
        else if (ch == ")") { if (ctr == 0) break; ctr-- }
        args = args ch
    }

    container = ""
    # Pattern: $VAR/ or ${VAR}/ or $VAR (with -p flag)
    if (match(args, /\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?\//)) {
        cname = substr(args, RSTART+1, RLENGTH-2)
        sub(/[{]/, "", cname); sub(/[}\/]/, "", cname)
        # Exclude TMPDIR (system-level variable)
        if (match(args, /\$\{?[A-Za-z_][A-Za-z0-9_]*\}?\//)) {
            tmp_cname = substr(args, RSTART+1, RLENGTH-2)
            gsub(/[{}\/]/, "", tmp_cname)
            if (tmp_cname != "TMPDIR") container = tmp_cname
        }
    }
    if (container == "" && match(args, /-p[[:space:]]+"?\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?/)) {
        pname = substr(args, RSTART, RLENGTH)
        sub(/^-p[[:space:]]+"?\$\{?/, "", pname); sub(/\}?"?.*/, "", pname)
        if (pname != "TMPDIR") container = pname
    }

    n_sites++
    site_var[n_sites] = varname
    site_scope[n_sites] = scope
    site_line[n_sites] = FNR
    site_container[n_sites] = container
    total_candidates++
}

# Detect and store printf -v A '%q' "$V" alias
function try_store_alias_printf(line, scope, lsq,    pos, alias_v, rest, src_v) {
    if (!match(line, /printf[[:space:]]+-v[[:space:]]+/)) return
    pos = RSTART
    if (in_sq_at(line, pos, lsq)) return
    rest = substr(line, pos + RLENGTH)
    # Parse alias variable name
    if (!match(rest, /^[A-Za-z_][A-Za-z0-9_]*/)) return
    alias_v = substr(rest, 1, RLENGTH)
    rest = substr(rest, RLENGTH + 1)
    # Skip format string
    sub(/^[[:space:]]+/, "", rest)
    if (substr(rest, 1, 1) == sq) {
        p = index(substr(rest, 2), sq)
        if (p <= 0) return
        rest = substr(rest, p + 2)
    } else if (substr(rest, 1, 1) == "\"") {
        p = index(substr(rest, 2), "\"")
        if (p <= 0) return
        rest = substr(rest, p + 2)
    } else return
    # Parse source variable $VAR
    sub(/^[[:space:]]+/, "", rest)
    if (substr(rest, 1, 1) == "\"") rest = substr(rest, 2)
    if (substr(rest, 1, 1) != "$") return
    rest = substr(rest, 2)
    if (substr(rest, 1, 1) == "{") rest = substr(rest, 2)
    if (!match(rest, /^[A-Za-z_][A-Za-z0-9_]*/)) return
    src_v = substr(rest, 1, RLENGTH)
    n_aliases++
    alias_name[n_aliases] = alias_v
    alias_src[n_aliases] = src_v
    alias_scope[n_aliases] = scope
    alias_line_no[n_aliases] = FNR
}

# Detect and store A=$V or A="$V" alias (simple assignment, not mktemp)
function try_store_alias_assign(line, scope, lsq,    pos, name, after_eq, src_v) {
    if (index(line, "mktemp") > 0) return  # skip mktemp lines
    if (index(line, "$(") > 0) return      # skip subshell lines
    if (!match(line, /[A-Za-z_][A-Za-z0-9_]*[[:space:]]*=[[:space:]]*"?\$[A-Za-z_][A-Za-z0-9_]*/)) return
    pos = RSTART
    if (in_sq_at(line, pos, lsq)) return
    matched = substr(line, RSTART, RLENGTH)
    # Extract name: everything before =
    name = matched
    sub(/[[:space:]]*=.*/, "", name)
    # Extract src_v: after = and optional " and $
    after_eq = matched
    sub(/^[^=]*=[[:space:]]*"?\$/, "", after_eq)
    if (!match(after_eq, /^[A-Za-z_][A-Za-z0-9_]*/)) return
    src_v = substr(after_eq, 1, RLENGTH)
    if (name != src_v && name != "" && src_v != "") {
        n_aliases++
        alias_name[n_aliases] = name
        alias_src[n_aliases] = src_v
        alias_scope[n_aliases] = scope
        alias_line_no[n_aliases] = FNR
    }
}

# Detect and store trap EXIT in current scope
function try_store_trap(line, scope, lsq,    pos, rest, body, is_dq, fn_nm, is_reset, after, sigs, n_sigs, i, sig, has_exit) {
    if (!match(line, /trap[[:space:]]+/)) return
    pos = RSTART
    if (pos > 1 && substr(line, pos-1, 1) ~ /[A-Za-z0-9_]/) return
    if (in_sq_at(line, pos, lsq)) return
    rest = substr(line, pos + RLENGTH)
    # Optional end-of-options separator
    if (substr(rest, 1, 2) == "--") sub(/^--[[:space:]]+/, "", rest)
    body = ""; is_dq = 0; fn_nm = ""; is_reset = 0
    if (substr(rest, 1, 1) == sq) {
        # Single-quoted body
        p = index(substr(rest, 2), sq)
        if (p <= 0) return
        body = substr(rest, 2, p - 1)
        rest = substr(rest, p + 2)
    } else if (substr(rest, 1, 1) == "\"") {
        # Double-quoted body
        is_dq = 1
        i = 2; body = ""
        while (i <= length(rest)) {
            ch = substr(rest, i, 1)
            if (ch == "\\") { i += 2; continue }
            if (ch == "\"") break
            body = body ch
            i++
        }
        rest = substr(rest, i + 1)
    } else {
        # Bare: either function name, "-", or empty string ''
        if (substr(rest, 1, 1) == "-") {
            is_reset = 1; body = "-"
            rest = substr(rest, 2)
        } else if (match(rest, /^[A-Za-z_][A-Za-z0-9_]*/)) {
            fn_nm = substr(rest, 1, RLENGTH)
            rest = substr(rest, RLENGTH + 1)
            body = fn_nm
        } else {
            return  # unrecognized form
        }
    }
    # Check for EXIT or 0 in signal list
    sub(/^[[:space:]]+/, "", rest)
    n_sigs = split(rest, sigs, /[[:space:]]+/)
    has_exit = 0
    for (i = 1; i <= n_sigs; i++) {
        sig = sigs[i]
        sub(/[^A-Za-z0-9].*$/, "", sig)
        if (sig == "EXIT" || sig == "0") { has_exit = 1; break }
    }
    if (!has_exit) return
    n_traps++
    trap_body[n_traps] = body
    trap_is_dq[n_traps] = is_dq
    trap_fn_name[n_traps] = fn_nm
    trap_is_reset[n_traps] = is_reset
    trap_scope[n_traps] = scope
    trap_line_no[n_traps] = FNR
}

# ---- Coverage check ----

# Check if function fn_name's body cites var v or any alias of v in scope
function fn_covers_var(fn_nm, v, scope,    body_text, i) {
    body_text = func_body[fn_nm]
    if (body_refs_var(body_text, v)) return 1
    # Check aliases of v in given scope
    for (i = 1; i <= n_aliases; i++) {
        if (alias_src[i] == v && alias_scope[i] == scope) {
            if (body_refs_var(body_text, alias_name[i])) return 1
        }
    }
    return 0
}

# Check if trap at index ti covers variable v using the given alias scope
function trap_covers(ti, v, alias_scope_name,    body, fn_nm, i) {
    if (trap_is_reset[ti]) return 0
    body = trap_body[ti]
    fn_nm = trap_fn_name[ti]
    # 1. DIRECT
    if (body_refs_var(body, v)) return 1
    # 2. ALIAS: trap body mentions alias A of v (alias defined before trap in scope)
    for (i = 1; i <= n_aliases; i++) {
        if (alias_src[i] != v) continue
        if (alias_scope[i] != alias_scope_name) continue
        # For function scope: alias must be defined before this trap
        if (alias_scope_name != "global" && alias_line_no[i] > trap_line_no[ti]) continue
        if (body_refs_var(body, alias_name[i])) return 1
    }
    # 3. FUNCTION: trap body is a bare function call
    if (fn_nm != "" && fn_nm != "-") {
        if (fn_covers_var(fn_nm, v, alias_scope_name)) return 1
    }
    return 0
}

# Main coverage check: returns 1 if covered, 0 if not
# scope: the scope where V was created (function name or "global")
# Global scope coverage: the LAST global EXIT trap supersedes all earlier ones, because
# bash `trap … EXIT` replaces the previous EXIT handler.  Exception: if a global-scope
# `exit` statement falls between an EARLIER covering trap and the LAST (non-covering)
# trap, the earlier trap is still the effective handler on the path that takes the exit.
# This correctly models scripts with mutually exclusive code paths (e.g. self-test vs
# main) where each path installs its own trap and exits before the other path's trap.
function global_covered(v,    i, j, last_gt, last_gl) {
    last_gt = -1; last_gl = -1
    for (i = 1; i <= n_traps; i++) {
        if (trap_scope[i] != "global") continue
        if (trap_line_no[i] > last_gl) { last_gt = i; last_gl = trap_line_no[i] }
    }
    if (last_gt < 0) return 0
    if (!trap_is_reset[last_gt] && trap_covers(last_gt, v, "global")) return 1
    # Last trap doesn't cover v. Check if any earlier covering trap is protected by
    # a global-scope exit between it and the last trap.
    for (i = 1; i <= n_traps; i++) {
        if (trap_scope[i] != "global") continue
        if (trap_line_no[i] >= last_gl) continue
        if (!trap_covers(i, v, "global")) continue
        for (j = 1; j <= n_global_exits; j++) {
            if (global_exit_line[j] > trap_line_no[i] && global_exit_line[j] < last_gl) {
                return 1
            }
        }
    }
    return 0
}

# Main coverage check.
# When the function has ANY EXIT trap, its last trap supersedes all global traps — bash
# `trap … EXIT` in a function replaces the inherited global handler.  Containment is
# checked through the same effective-trap selection so that a function's `trap - EXIT`
# also invalidates global coverage of a container variable.
function covered(v, container, scope,    i, fn_has_trap, last_fn_trap, last_fn_line) {
    # --- Determine function's EXIT traps (scope != "global" only) ---
    fn_has_trap = 0; last_fn_trap = -1; last_fn_line = -1
    if (scope != "global") {
        for (i = 1; i <= n_traps; i++) {
            if (trap_scope[i] != scope) continue
            fn_has_trap = 1
            if (trap_line_no[i] > last_fn_line) {
                last_fn_trap = i; last_fn_line = trap_line_no[i]
            }
        }
    }

    # --- Check containment ---
    if (container != "") {
        if (fn_has_trap) {
            # Function has EXIT traps: only the function's last trap can cover the container.
            if (last_fn_trap >= 0 && !trap_is_reset[last_fn_trap]) {
                if (trap_covers(last_fn_trap, container, scope)) return 1
            }
        } else {
            # No function EXIT traps (or global scope): check global traps.
            if (global_covered(container)) return 1
        }
    }

    # --- Check v directly ---
    if (fn_has_trap) {
        if (last_fn_trap >= 0 && !trap_is_reset[last_fn_trap]) {
            if (trap_covers(last_fn_trap, v, scope)) return 1
        }
    } else {
        if (global_covered(v)) return 1
    }
    return 0
}

# ---- Per-file flush: check all sites and report violations ----
function flush_file(    i, v, cont, scope) {
    for (i = 1; i <= n_sites; i++) {
        v     = site_var[i]
        cont  = site_container[i]
        scope = site_scope[i]
        if (!covered(v, cont, scope)) {
            printf "%s:%d: variable %s created by mktemp without guaranteed cleanup (ADR D2)\n",
                   cur_file, site_line[i], v
            violations++
        }
    }
}

# ---- Reset per-file data ----
function reset_file(    k) {
    n_sites = 0; n_aliases = 0; n_traps = 0
    for (k in func_body)   delete func_body[k]
    for (k in site_var)    delete site_var[k]
    for (k in site_scope)  delete site_scope[k]
    for (k in site_line)   delete site_line[k]
    for (k in site_container) delete site_container[k]
    for (k in alias_name)  delete alias_name[k]
    for (k in alias_src)   delete alias_src[k]
    for (k in alias_scope) delete alias_scope[k]
    for (k in alias_line_no) delete alias_line_no[k]
    for (k in trap_body)   delete trap_body[k]
    for (k in trap_is_dq)  delete trap_is_dq[k]
    for (k in trap_fn_name) delete trap_fn_name[k]
    for (k in trap_is_reset) delete trap_is_reset[k]
    for (k in trap_scope)  delete trap_scope[k]
    for (k in trap_line_no) delete trap_line_no[k]
    # Parse state
    in_func = 0; cur_scope = "global"; brace_depth = 0
    in_sq = 0; in_heredoc = 0; heredoc_delim = ""; heredoc_strip = 0
    pending_func = 0; pending_func_name = ""
    line_start_sq = 0
    n_global_exits = 0
    for (k in global_exit_line) delete global_exit_line[k]
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

  # run_case NAME CONTENT EXPECT_VIOLATIONS [vars...]
  # EXPECT_VIOLATIONS=0 → expect rc=0 (no violations)
  # EXPECT_VIOLATIONS=1 → expect rc=1 (at least one violation mentioning any of vars)
  run_case() {
    local name="$1" expect_v="$2"
    shift 2
    local expected_vars=("$@")
    local f="$tmpdir/${name}.sh"
    # Content is read from stdin
    cat > "$f"
    local out rc=0
    out=$(awk -v sq="'" -v min_candidates=1 "$AWK_PROG" "$f" 2>&1) || rc=$?
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
        if ! printf '%s\n' "$out" | grep -F "variable $v" >/dev/null; then
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
  }

  echo "--- self-test: check-mktemp-cleanup ---"

  # ---- PASSING ARMS (must NOT be flagged) ----

  # pass-direct: V covered by SQ trap '$V'
  # Affirms: ADR D2 DIRECT — SQ trap body citing $V directly is covered
  run_case "pass-direct" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
V=$(mktemp -d)
trap 'rm -rf "$V"' EXIT
FIX

  # pass-direct-brace-default: V covered via ${V:-} in trap body
  # Affirms: ADR D2 DIRECT — ${V:-} form also counts as a valid citation (FP-1 fix)
  run_case "pass-direct-brace-default" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
FENCE_TMP=$(mktemp -d)
trap 'rm -rf "${FENCE_TMP:-}"' EXIT
FIX

  # pass-alias-printf-v: V covered via printf -v A '%q' "$V" + DQ trap "rm $A"
  # Affirms: ADR D2 ALIAS — printf-v form; trap cites alias var, not V directly (FP-2 fix)
  run_case "pass-alias-printf-v" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
printf -v q '%q' "$WORK"
trap "rm -rf -- $q" EXIT
FIX

  # pass-alias-assign-direct: V covered via A="$V" simple assignment + SQ trap 'rm -rf "$A"'
  # Affirms: ADR D2 ALIAS assignment form — A=$V creates an alias; trap citing A covers V directly
  run_case "pass-alias-assign-direct" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
CLEANUP_WORK="$WORK"
trap 'rm -rf "$CLEANUP_WORK"' EXIT
FIX

  # pass-function-alias: V covered via A="$V" + function that cites A + trap calling function
  # Affirms: ADR D2 FUNCTION+ALIAS — function indirection with assignment-form alias covers V
  run_case "pass-function-alias" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
CLEANUP_WORK="$WORK"
cleanup_fn() { rm -rf "$CLEANUP_WORK" 2>/dev/null || true; }
trap cleanup_fn EXIT
FIX

  # pass-function: V covered via trap calling function that cites V
  # Affirms: ADR D2 FUNCTION — one-level-deep function indirection (R2 explains limit)
  run_case "pass-function" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT
FIX

  # pass-function-oneliner: one-liner function definition (FN-3 fix)
  # Affirms: ADR D2 FUNCTION — one-liner function body is correctly collected
  run_case "pass-function-oneliner" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
cleanup() { rm -rf "$WORK" 2>/dev/null || true; }
trap cleanup EXIT
FIX

  # pass-contained: V created under covered container W
  # Affirms: ADR D2 CONTAINED — mktemp under covered dir counts as covered (FP-3 / FP-5/6 fix)
  run_case "pass-contained" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
scratch=$(mktemp -d "$WORK/scratch.XXXXXX")
FIX

  # pass-local-fn-alias: in a function, alias covers V (last-trap rule satisfied)
  # Affirms: ADR D2 ALIAS within function scope; last trap covers V via alias
  run_case "pass-local-fn-alias" 0 << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
run_it() {
  local tmp
  tmp=$(mktemp -d)
  local q; printf -v q '%q' "$tmp"
  trap "rm -rf -- $q" EXIT
  echo "work"
}
run_it
FIX

  # pass-sq-fixture-skip: mktemp inside a SQ string is NOT a real candidate (FN-2 / FP-4b fix)
  # Affirms: SQ-state tracking prevents fixture strings from being treated as assignments;
  # one real covered mktemp is included so the vacuity guard has ≥1 candidate to count
  run_case "pass-sq-fixture-skip" 0 << 'FIX'
#!/usr/bin/env bash
REAL=$(mktemp -d)
trap 'rm -rf "$REAL"' EXIT
sentinel_trap() { trap 'echo ok' EXIT; }
run_case "N1-example" \
'V=$(mktemp -d)
trap '"'"'rm -rf "$V"'"'"' EXIT' 0
FIX

  # pass-no-dollar-paren-not-candidate: line with mktemp but no $( not a candidate (FP-4a fix)
  # Affirms: FP-4(a) criterion — line without $( is never a mktemp candidate;
  # one real covered mktemp is included so the vacuity guard has ≥1 candidate to count
  run_case "pass-no-dollar-paren-not-candidate" 0 << 'FIX'
#!/usr/bin/env bash
REAL=$(mktemp -d)
trap 'rm -rf "$REAL"' EXIT
sentinel_trap() { trap 'echo ok' EXIT; }
# utility list (no $( on this line):
for util in sh mktemp rm; do echo "$util"; done
FIX

  # ---- FAILING ARMS (must be flagged) ----

  # fail-no-trap: V with no EXIT trap at all
  # Affirms: ADR D2 baseline — no trap means no guaranteed cleanup; gate must reprove
  run_case "fail-no-trap" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
V=$(mktemp -d)
echo "work in $V"
FIX

  # fail-trap-no-citation: trap exists but does not cite V
  # Affirms: ADR D2 — trap that covers a different var does not cover V (T1 from threat model)
  run_case "fail-trap-no-citation" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
OTHER=$(mktemp -d)
trap 'rm -rf "$OTHER"' EXIT
V=$(mktemp -d)
FIX

  # fail-rm-normal-path-only: only rm in normal path, no trap (D1 — not guaranteed)
  # Affirms: ADR D1 — rm in normal flow is not guaranteed cleanup; trap required
  run_case "fail-rm-normal-path-only" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
OTHER=$(mktemp -d)
trap 'rm -rf "$OTHER"' EXIT
V=$(mktemp -d)
echo "work"
rm -rf "$V"
FIX

  # fail-trap-replaced-fn: in function scope, covering trap superseded by non-covering trap (FN-5)
  # Affirms: last-trap rule in function scope — if the final trap does not cover V, V is UNCOVERED
  run_case "fail-trap-replaced-fn" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
do_work() {
  local V
  V=$(mktemp -d)
  trap 'rm -rf "$V"' EXIT   # covers V ...
  local W
  W=$(mktemp -d)
  trap 'rm -rf "$W"' EXIT   # ... but this REPLACES the previous trap; V now uncovered
  echo "work"
}
do_work
FIX

  # fail-trap-reset-fn: trap - EXIT removes coverage in function scope (FN-8, function scope)
  # Affirms: last-trap rule — a reset trap leaves V uncovered even if an earlier trap covered it
  run_case "fail-trap-reset-fn" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
do_work() {
  local V
  V=$(mktemp -d)
  trap 'rm -rf "$V"' EXIT
  trap - EXIT   # removes all EXIT traps; V is now uncovered
  echo "work"
}
do_work
FIX

  # fail-contained-under-uncovered: V is contained under W, but W is uncovered (FN-6)
  # Affirms: CONTAINED only covers V when the container W is itself covered
  run_case "fail-contained-under-uncovered" 1 "sub" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
OTHER=$(mktemp -d)
trap 'rm -rf "$OTHER"' EXIT
W=$(mktemp -d)   # W has no trap
sub=$(mktemp -d "$W/sub.XXXXXX")
FIX

  # fail-fn1-template-with-quote-in-args: mktemp with "${TMPDIR:-/tmp}/x.XXXXXX" must be detected
  # Affirms: FN-1 fix — mktemp args containing " must still be matched; VAR=$(mktemp …) is a site
  run_case "fail-fn1-template-args" 1 "EMPTY_EXC" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
EMPTY_EXC=$(mktemp "${TMPDIR:-/tmp}/checknul-exc.XXXXXX")
FIX

  # fail-fn12-case-branch: mktemp inside a case branch with no trap (FN-12)
  # Affirms: FN-12 — mktemp in a case-branch result is detected; AWK tracks $( not case pattern text
  run_case "fail-fn12-case-branch" 1 "d" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
OTHER=$(mktemp -d)
trap 'rm -rf "$OTHER"' EXIT
arg="${1:-default}"
case "$arg" in
  alpha)
    d=$(mktemp -d)
    echo "alpha with $d"
    ;;
esac
FIX

  # fail-trap-replaced-global: global covering trap superseded by later non-covering trap
  # Affirms: global last-trap rule — bash trap EXIT replaces the previous EXIT handler;
  # a later trap for W leaves V uncovered (no exit-boundary between the two traps)
  run_case "fail-trap-replaced-global" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
V=$(mktemp -d)
trap 'rm -rf "$V"' EXIT
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
FIX

  # fail-heredoc-apostrophe: mktemp after a heredoc whose body has an apostrophe must be caught
  # Affirms: heredoc content does not corrupt in_sq state — gate detects V even when the
  # preceding heredoc body contains a bare apostrophe ("Don't")
  run_case "fail-heredoc-apostrophe" 1 "V" << 'FIX'
#!/usr/bin/env bash
set -euo pipefail
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
cat << 'HDOC'
Don't worry about this.
HDOC
V=$(mktemp -d)
FIX

  # ---- VACUITY GUARD ----
  # vac-no-candidates: file with no mktemp assignments → gate must fail (vacuity)
  # Affirms: vacuity guard prevents silent success on a corpus with no candidates
  vac_f="$tmpdir/vac_no_candidates.sh"
  printf '#!/usr/bin/env bash\necho hello\ntrap '"'"'echo bye'"'"' EXIT\n' > "$vac_f"
  vac_rc=0
  awk -v sq="'" -v min_candidates=1 "$AWK_PROG" "$vac_f" >/dev/null 2>&1 || vac_rc=$?
  if [[ $vac_rc -ne 0 ]]; then
    echo "  PASS  vac-no-candidates (rc=$vac_rc)"
    (( PASS++ )) || true
  else
    echo "  FAIL  vac-no-candidates (expected rc!=0 from vacuity guard, got rc=0)"
    (( FAIL++ )) || true
  fi

  echo "---"
  printf "Results: %d passed, %d failed\n" "$PASS" "$FAIL"
  if [[ $FAIL -gt 0 ]]; then exit 1; fi
  # Verify gate does not flag itself
  self_rc=0
  self_out=$(awk -v sq="'" -v min_candidates=1 "$AWK_PROG" "${BASH_SOURCE[0]}" 2>&1) || self_rc=$?
  if [[ -n "$self_out" ]]; then
    printf "FAIL  self-scan: gate flagged itself:\n%s\n" "$self_out"
    exit 1
  fi
  echo "PASS  self-scan: gate does not flag itself"
  exit 0
fi

# ---------------------------------------------------------------------------
# Main: scan files for mktemp-without-cleanup violations
# ---------------------------------------------------------------------------
if [[ ${#EXPLICIT_FILES[@]} -gt 0 ]]; then
  SCAN_FILES=("${EXPLICIT_FILES[@]}")
  FLOOR=1
else
  SCAN_FILES=()
  while IFS= read -r f; do
    SCAN_FILES+=("$f")
  done < <(git ls-files '*.sh' 2>/dev/null)
  # Floor measured at gate creation (2026-11-11, HEAD after ML-1A):
  # 72 real candidates (97 raw grep hits - 25 fixture strings in sq-strings).
  # Set conservatively below measurement to tolerate minor corpus changes.
  FLOOR=65
fi

if [[ ${#SCAN_FILES[@]} -eq 0 ]]; then
  printf '%s: ERROR: no .sh files to scan\n' "$SELF" >&2
  exit 2
fi

VIOLATIONS=0
AWK_OUT=""
AWK_RC=0
AWK_OUT=$(awk -v sq="'" -v min_candidates="$FLOOR" "$AWK_PROG" "${SCAN_FILES[@]}" 2>&1) || AWK_RC=$?

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
  printf '%s: FAIL — %d mktemp cleanup violation(s) found (ADR D2)\n' "$SELF" "$VIOLATIONS" >&2
  exit 1
fi

printf '%s: OK — all mktemp assignments have guaranteed EXIT cleanup\n' "$SELF"
exit 0
