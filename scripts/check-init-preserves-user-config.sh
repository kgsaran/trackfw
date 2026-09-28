#!/usr/bin/env bash
# check-init-preserves-user-config.sh — anti-reintroduction gate for
# ROADMAP-2026-09-28-trackfw-init-reexecutado-destroi-configuracao-do-consumidor (ML-1B).
#
# WHY THIS EXISTS: trackfw init executes unconditional os.WriteFile calls on files that the
# consumer authors. trackfw.yaml is the governance config written by the consumer (or by
# trackfw discover); trackfw validate reads it, and overwriting it silently resets the
# consumer's customisations, causing validate to reject a correctly-configured repository.
# lefthook.yml is the consumer's lefthook configuration; overwriting it destroys hooks the
# consumer added after the initial scaffolding. Wave 0 (2026-09-28) classified 22 write sites
# in internal/generators/scaffold.go — exactly two are class (a) (consumer-authored config,
# unconditional overwrite, no read-before-write):
#   line 864  writeTrackfwConfig    → trackfw.yaml
#   line 2806 generateLefthookHook → lefthook.yml
# A third site (line 2498, generateCommitMsgHook, via variable lefthookPath) is class (c):
# it reads lefthook.yml before writing and appends only what is absent.  It is examined by
# this gate and passes because of that ReadFile guard.
#
# WHAT THIS GATE CHECKS (dual predicate — either condition is sufficient):
#   1. os.ReadFile precedes the os.WriteFile within the enclosing function body.
#      Rationale: the (c) merge site already follows this pattern; no annotation needed.
#   2. An inline marker "consumer-config-merge-allowed: <reason>" appears on the write line
#      itself or the line immediately above it.
#      Rationale: when the merge is delegated to a helper (ReadFile outside the function),
#      condition 1 fails; the marker makes the intentional guard explicit and auditable.
# A site that satisfies neither condition is an unconditional overwrite → FAIL.
#
# WHY NOT REUSE write-containment-allowed: That marker asserts symlink-containment (pathguard
# scope).  Reusing it conflates two orthogonal safety properties and would mark every site in
# the file as "justified", rendering this gate vacuous on arrival.
#
# ANTI-VACUITY: for EACH consumer-authored target, at least one write site must be found.
# If the file is refactored and the literal target strings disappear, the gate fails naming
# the missing target — never a silent green.  ("métrica por artefato, não por regra")
#
# CONSUMER-AUTHORED FILES (config owned by the consumer, not the product):
#   trackfw.yaml — governance config; overwriting resets customisations, breaks validate.
#   lefthook.yml — lefthook config; overwriting destroys consumer hooks.
#
# NOT consumer-authored (product-generated — unconditional truncation IS the update mechanism):
#   scripts/trackfw-*.sh, .github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml,
#   .husky/*, .lefthook/commit-msg/trackfw-req-check.sh, ~/.claude/skills/trackfw/SKILL.md.
#   (Wave 0 class (b), 14 sites — this gate does not examine them.)
#
# COMMENT FILTERING (both write-site detection and ReadFile guard detection):
#   Lines that are pure Go comments (optional whitespace + //) are excluded from both the
#   grep that finds write sites and the awk that finds ReadFile guards.  A real call that
#   appears only in a comment does NOT count as a guard and does NOT count as a write site.
#   Inline trailing comments are also stripped before pattern-matching in the guard awk.
#   Caveat: the inline strip uses sub(/[[:space:]]*\/\/.*$/, "") which also truncates at //
#   inside string literals (e.g. URLs).  This is acceptable for this gate: os.ReadFile after
#   a URL in the same source line is not a credible real-world pattern in scaffold.go.
#   A comment-only guard is INDISTINGUISHABLE from an absent guard — by design.
#   For consistency, inline end-of-line comments are also excluded: `foo() // os.ReadFile`
#   does NOT satisfy condition 1; the guard must be executable code.
#
# SELF-TEST (--self-test flag):
#   Arm 1 (braço 1): synthetic scaffold with unmarked writes to consumer files → gate FAILS,
#     naming each site.  Demonstrates the gate catches the defect.
#   Arm 2 (braço 2): synthetic scaffold where consumer writes carry the marker AND an intact
#     (b) site (generateValidateScript, unconditional os.WriteFile to scripts/trackfw-validate.sh)
#     is present → gate PASSES.  Demonstrates the gate does NOT flag product-generated writes,
#     which would block the mechanism by which script fixes (e.g. CRLF, PR #353) reach consumers.
#   Arm 3 (braço 3): fixture with two lefthook.yml write sites in separate functions —
#     one guarded by a REAL os.ReadFile call (no marker), one where os.ReadFile appears only
#     in a comment.  Gate must FAIL naming the comment-only site AND emit OK for the real-guard
#     site.  Demonstrates that condition 1 is live after the fix (not silently dead) and that
#     a comment mentioning os.ReadFile cannot satisfy condition 1.
#
# LIMITE CONHECIDO (Wave 2, 2026-09-28): este gate é ESTRUTURAL, não semântico.
# Ele detecta a AUSÊNCIA de leitura antes da escrita; não verifica que o resultado
# da leitura é USADO no merge. Um os.ReadFile decorativo — p.ex.
#   existing, _ := os.ReadFile("trackfw.yaml"); _ = len(existing)
#   return os.WriteFile("trackfw.yaml", template, 0644)   // trunca
# satisfaz o gate. Medido com decoy na auditoria independente.
# Isto é limite da técnica (análise textual), não defeito a corrigir.
# 🔴 NÃO "resolva" tornando o marcador consumer-config-merge-allowed: obrigatório:
# o decoy passaria igual, bastando colar o marcador — que é afirmação do mesmo
# autor que escreveu o truncamento. O ML-1C removeu esse marcador POR MEDIÇÃO
# (falsificação provou que a remoção fortaleceu o gate).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Override via env var for self-test arms; default is the real scaffold file.
SCAFFOLD_FILE="${SCAFFOLD_FILE:-$REPO_ROOT/internal/generators/scaffold.go}"

MARKER="consumer-config-merge-allowed:"

# ---- Consumer-authored targets (parallel arrays, bash 3.2-compatible) --------------------
# Each entry covers one write pattern targeting a consumer-authored file.
# String-literal patterns catch the two (a) sites.
# Variable pattern (lefthookPath) catches the (c) merge site and any future variable writes.
TARGET_NAMES=(
    "trackfw.yaml"
    "lefthook.yml[literal]"
    "lefthook.yml[via-lefthookPath]"
)
TARGET_PATTERNS=(
    'os\.WriteFile\("trackfw\.yaml"'
    'os\.WriteFile\("lefthook\.yml"'
    'os\.WriteFile\(lefthookPath'
)

# ---- check_scaffold <file> ----------------------------------------------------------------
# Returns 0 (PASS) or 1 (FAIL).  Prints a labelled line per site and a summary.
check_scaffold() {
    local scaffold="$1"
    local total_examined=0
    local total_violations=0

    if [[ ! -f "$scaffold" ]]; then
        echo "FAIL: scaffold file not found: $scaffold" >&2
        return 1
    fi

    local i
    for i in "${!TARGET_NAMES[@]}"; do
        local name="${TARGET_NAMES[$i]}"
        local pattern="${TARGET_PATTERNS[$i]}"
        local sites_for_target=0

        while IFS=: read -r lineno _rest; do
            [[ -z "${lineno:-}" ]] && continue
            sites_for_target=$((sites_for_target + 1))
            total_examined=$((total_examined + 1))

            # Find the start of the enclosing function: last '^func ' before lineno.
            # '/^func /' matches plain functions AND method receivers (func (r *T) name()).
            local func_start func_name
            func_start=$(awk -v wline="$lineno" \
                '/^func / { start = NR } NR == wline { print start+0; exit }' "$scaffold")

            if [[ -z "${func_start:-}" || "$func_start" -eq 0 ]]; then
                echo "FAIL [$name] line $lineno — could not locate enclosing function; treating as violation"
                total_violations=$((total_violations + 1))
                continue
            fi

            func_name=$(sed -n "${func_start}p" "$scaffold" \
                | grep -oE 'func [A-Za-z0-9_]+' | head -1 | cut -d' ' -f2 || echo "<unknown>")

            # ---- Condition 1: os.ReadFile in enclosing function, before the write ----
            # Comment filtering: pure comment lines (^[[:space:]]*// ...) are skipped.
            # Inline trailing comments are stripped before matching so that
            #   existing, _ := os.ReadFile(f) // some note
            # still counts as a guard, but
            #   // old: existing, _ := os.ReadFile(f)    ← comment-only, NOT a guard
            # does not.  Caveat documented in the script header.
            local has_read
            has_read=$(awk -v fstart="$func_start" -v wline="$lineno" '
                NR > fstart && NR < wline {
                    line = $0
                    # Skip pure comment lines
                    if (line ~ /^[[:space:]]*\/\//) next
                    # Strip inline trailing comment before matching
                    sub(/[[:space:]]*\/\/.*$/, "", line)
                    if (line ~ /os\.ReadFile/) found=1
                }
                END { print (found ? "yes" : "no") }' \
                "$scaffold")

            # ---- Condition 2: inline marker on the write line or the line above ----
            local write_line prev_line prevno
            write_line=$(sed -n "${lineno}p" "$scaffold")
            prevno=$((lineno - 1))
            prev_line=""
            [[ $prevno -ge 1 ]] && prev_line=$(sed -n "${prevno}p" "$scaffold")

            local has_marker=0
            if echo "$write_line" | grep -qF "$MARKER" || echo "$prev_line" | grep -qF "$MARKER"; then
                has_marker=1
            fi

            if [[ "$has_read" == "yes" ]]; then
                echo "OK   [$name] line $lineno in $func_name — os.ReadFile precedes write in same function"
            elif [[ $has_marker -eq 1 ]]; then
                echo "OK   [$name] line $lineno in $func_name — $MARKER present"
            else
                echo "FAIL [$name] line $lineno in $func_name — unconditional write to consumer-authored file (no ReadFile guard, no '$MARKER' marker)"
                total_violations=$((total_violations + 1))
            fi
        done < <(grep -nE "$pattern" "$scaffold" | grep -Ev '^[0-9]+:[[:space:]]*//' || true)

        # Anti-vacuity: every target must have at least one site
        if [[ $sites_for_target -eq 0 ]]; then
            echo "FAIL [anti-vacuity] [$name] — no write site found for pattern '$pattern'" \
                 "(file refactored? literal target string changed?)" >&2
            total_violations=$((total_violations + 1))
        fi
    done

    echo ""
    echo "Sites examined: $total_examined"

    if [[ $total_violations -gt 0 ]]; then
        echo "FAIL: $total_violations violation(s) — unconditional write(s) to consumer-owned config"
        return 1
    fi
    echo "PASS: all $total_examined consumer-config write site(s) are guarded"
    return 0
}

# ---- Self-test (--self-test) --------------------------------------------------------------
if [[ "${1:-}" == "--self-test" ]]; then
    TMPDIR_ST=$(mktemp -d)
    trap 'rm -rf "$TMPDIR_ST"' EXIT
    overall_rc=0

    # ---- Arm 1 (braço 1): unmarked writes to consumer files → gate MUST FAIL ----
    # Three sites, none guarded: two (a) literals + one variable write.
    # Gate must name each site and exit non-zero.
    cat > "$TMPDIR_ST/arm1.go" <<'GOEOF'
package generators

import "os"

// (a) site — unconditional overwrite of trackfw.yaml, no guard, no marker: the primary defect
func writeTrackfwConfig() error {
	if err := os.WriteFile("trackfw.yaml", []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}

// (a) site — unconditional overwrite of lefthook.yml, no guard, no marker
func generateLefthookHook() error {
	if err := os.WriteFile("lefthook.yml", []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}

// variable write — no ReadFile precedes it, no marker: would be a defect if introduced here
func generateCommitMsgHook() error {
	lefthookPath := "lefthook.yml"
	if err := os.WriteFile(lefthookPath, []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}
GOEOF

    echo "=== Arm 1 (braço 1): 3 unmarked consumer writes — gate must FAIL naming each site ==="
    arm1_rc=0
    arm1_out=$(SCAFFOLD_FILE="$TMPDIR_ST/arm1.go" bash "${BASH_SOURCE[0]}" 2>&1) || arm1_rc=$?
    echo "$arm1_out"
    # Gate must exit non-zero AND name trackfw.yaml
    if [[ $arm1_rc -ne 0 ]] && echo "$arm1_out" | grep -q "FAIL \[trackfw.yaml\]"; then
        echo "SELF-TEST arm1: PASS (gate correctly failed naming trackfw.yaml; rc=$arm1_rc)"
    else
        echo "SELF-TEST arm1: FAIL — expected gate to fail naming trackfw.yaml (rc=$arm1_rc)" >&2
        overall_rc=1
    fi
    echo ""

    # ---- Arm 2 (braço 2): fixed consumer writes (marker) + intact (b) site → gate MUST PASS ----
    # The (b) site generateValidateScript is an unconditional os.WriteFile to
    # scripts/trackfw-validate.sh — product-generated, not consumer-authored.  Gate must NOT
    # flag it: that write is the mechanism by which script fixes reach consumers (PR #353).
    # All consumer-config writes carry the marker on the line immediately above.
    cat > "$TMPDIR_ST/arm2.go" <<'GOEOF'
package generators

import "os"

// (b) site — unconditional truncating write to PRODUCT-generated file (scripts/trackfw-validate.sh).
// This is class (b): NOT consumer-authored; gate must not examine or flag it.
// It is the mechanism by which script-level fixes (e.g. CRLF normalisation, PR #353) reach
// consumers: trackfw init re-runs and delivers the updated script.
func generateValidateScript() error {
	if err := os.WriteFile("scripts/trackfw-validate.sh", []byte("script"), 0755); err != nil {
		return err
	}
	return nil
}

// (a) site corrected — marker on line above confirms intentional merge ramp
func writeTrackfwConfig() error {
	// consumer-config-merge-allowed: reads existing trackfw.yaml, merges missing top-level keys only
	if err := os.WriteFile("trackfw.yaml", []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}

// (a) site corrected — marker on line above confirms intentional merge ramp
func generateLefthookHook() error {
	// consumer-config-merge-allowed: reads existing lefthook.yml, appends pre-commit block only if absent
	if err := os.WriteFile("lefthook.yml", []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}

// (c) variable write — marker on line above confirms intentional merge (read precedes write)
func generateCommitMsgHook() error {
	lefthookPath := "lefthook.yml"
	existing, _ := os.ReadFile(lefthookPath)
	// consumer-config-merge-allowed: reads existing lefthook.yml, appends commit-msg section only if absent
	if err := os.WriteFile(lefthookPath, append(existing, []byte("add")...), 0644); err != nil {
		return err
	}
	return nil
}
GOEOF

    echo "=== Arm 2 (braço 2): marked consumer writes + (b) site intact — gate must PASS ==="
    arm2_rc=0
    arm2_out=$(SCAFFOLD_FILE="$TMPDIR_ST/arm2.go" bash "${BASH_SOURCE[0]}" 2>&1) || arm2_rc=$?
    echo "$arm2_out"
    if [[ $arm2_rc -eq 0 ]]; then
        echo "SELF-TEST arm2: PASS (gate correctly passed; (b) site not flagged; rc=0)"
    else
        echo "SELF-TEST arm2: FAIL — expected gate to pass (rc=$arm2_rc)" >&2
        overall_rc=1
    fi
    echo ""

    # ---- Arm 3 (braço 3): one real ReadFile guard + one comment-only "guard" → gate MUST FAIL
    #      naming the comment-only site AND emit OK for the real-guard site.
    #      Two-sided assertion: proves condition 1 is live (real guard accepted) AND that a
    #      comment mentioning os.ReadFile cannot satisfy condition 1 (comment-only rejected).
    cat > "$TMPDIR_ST/arm3.go" <<'GOEOF'
package generators

import "os"

// trackfw.yaml — marker guard so anti-vacuity passes for that target
func writeTrackfwConfig() error {
	// consumer-config-merge-allowed: merges missing top-level keys only
	if err := os.WriteFile("trackfw.yaml", []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}

// lefthook.yml site A — REAL os.ReadFile guard (no marker): condition 1 satisfied → OK
func funcWithRealGuard() error {
	existing, _ := os.ReadFile("lefthook.yml")
	if err := os.WriteFile("lefthook.yml", existing, 0644); err != nil {
		return err
	}
	return nil
}

// lefthook.yml site B — os.ReadFile appears ONLY in a comment: condition 1 NOT satisfied → FAIL
func funcWithCommentOnlyGuard() error {
	// old: existing, _ := os.ReadFile("lefthook.yml") — removed by refactor
	var existing []byte
	_ = existing
	if err := os.WriteFile("lefthook.yml", []byte("content"), 0644); err != nil {
		return err
	}
	return nil
}

// lefthook.yml via variable — real ReadFile guard for anti-vacuity of lefthook.yml[via-lefthookPath]
func generateCommitMsgHook() error {
	lefthookPath := "lefthook.yml"
	existing, _ := os.ReadFile(lefthookPath)
	if err := os.WriteFile(lefthookPath, existing, 0644); err != nil {
		return err
	}
	return nil
}
GOEOF

    echo "=== Arm 3 (braço 3): real ReadFile guard + comment-only 'guard' — gate must FAIL naming comment site, OK real site ==="
    arm3_rc=0
    arm3_out=$(SCAFFOLD_FILE="$TMPDIR_ST/arm3.go" bash "${BASH_SOURCE[0]}" 2>&1) || arm3_rc=$?
    echo "$arm3_out"
    # Gate must exit non-zero AND name lefthook.yml[literal] as FAIL AND also emit OK for lefthook.yml[literal]
    if [[ $arm3_rc -ne 0 ]] \
        && echo "$arm3_out" | grep -q "FAIL \[lefthook.yml\[literal\]\]" \
        && echo "$arm3_out" | grep -q "OK   \[lefthook.yml\[literal\]\]"; then
        echo "SELF-TEST arm3: PASS (gate failed naming comment-only site; real-guard site accepted; rc=$arm3_rc)"
    else
        echo "SELF-TEST arm3: FAIL — expected FAIL for comment-only site AND OK for real-guard site (rc=$arm3_rc)" >&2
        overall_rc=1
    fi
    echo ""

    if [[ $overall_rc -eq 0 ]]; then
        echo "SELF-TEST: PASS (all 3 arms)"
    else
        echo "SELF-TEST: FAIL" >&2
    fi
    exit $overall_rc
fi

# ---- Real run -----------------------------------------------------------------------------
echo "--- Gate: init-preserves-user-config ---"
echo "Scaffold: $SCAFFOLD_FILE"
echo ""
check_scaffold "$SCAFFOLD_FILE"
