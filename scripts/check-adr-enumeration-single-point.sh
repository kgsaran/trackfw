#!/usr/bin/env bash
# check-adr-enumeration-single-point.sh — anti-reintroduction gate for
# REQ-2026-09-29 (context reporta zero ADRs onde status reporta 145). ML-1B.
#
# WHY THIS EXISTS: Wave 0 (2026-09-29) classified 9 sites that read/count ADR
# files. Three are class-(iii) — they use os.ReadDir (root-only) or filepath.Glob
# (root-only) instead of the single-point primitives walkADRFilePaths /
# ResolveADRFiles (both use filepath.WalkDir, recursive). This caused
# trackfw context to report ADRs(0) in repos whose ADRs live exclusively in
# state subdirectories (docs/adr/zeus/done/, etc.), while trackfw status
# reported the correct count.
#
# Wave 0 evidence (fixture: adr_dirs: [docs/adr/zeus], 4 ADRs in done/wip):
#   trackfw context → "## ADRs (0)"    (os.ReadDir — root-only — WRONG)
#   trackfw status  → "ADRs 4"         (walkADRFilePaths — WalkDir — CORRECT)
#   Same context run: "ADRs (0)" AND names all 4 ADRs in warnings — internal
#   contradiction proved by two implementations of the same question.
#
# ML-1A corrects the three class-(iii) sites:
#   S1  internal/generators/context.go:39   GetContext    os.ReadDir(adrDir)
#   S6  internal/generators/adr.go:189      ListADRs      filepath.Glob(dir+"/*.md")
#   S7  internal/generators/adr.go:316      NewADRDraft   filepath.Glob(adrDir+"ADR-*")
# This gate prevents new violations from being introduced after ML-1A.
#
# WHAT THIS GATE CHECKS (four patterns; all skip comment lines):
#
#   Pattern A (name-based) — os.ReadDir( where the comment-stripped source line
#     contains "adr" (case-insensitive) in the ReadDir argument.
#     Catches: os.ReadDir(adrDir), os.ReadDir(adrPath), os.ReadDir(myADRDir).
#     Does NOT catch: os.ReadDir(dir), os.ReadDir(parent), os.ReadDir(src).
#     Rationale: explicit ADR-named variables are the common mistake.
#
#   Pattern A' (loop-context) — os.ReadDir(LOOPVAR) where LOOPVAR was bound by
#     "for _, LOOPVAR := range ... ADRDirs" in the same file.
#     Catches: for _, d := range cfg.ADRDirs { os.ReadDir(d) }
#     Rationale: short variable names (d, dir) would evade Pattern A.
#     Note: file-level var extraction (not scope-tracked) may give false
#     positives if LOOPVAR is reused for a non-ADR ReadDir elsewhere in the
#     same file. Use adr-single-point-exempt: marker to suppress if needed.
#
#   Pattern B (file-scope for generators/adr.go) — filepath.Glob( in
#     internal/generators/adr.go (any Glob call on a non-comment line).
#     Rationale: after ML-1A all ADR enumeration in that file uses
#     walkADRFilePaths; no filepath.Glob is legitimate there.
#     Exception: add "// adr-single-point-exempt: <reason>" on the line or
#     the line immediately above to declare a legitimate Glob in that file.
#
#   Pattern C (ADR- in Glob argument) — filepath.Glob( where the same source
#     line (comment-stripped) contains the literal string "ADR-".
#     Catches: filepath.Glob(filepath.Join(someDir, "ADR-*.md")).
#     Does NOT catch: filepath.Glob(dir+"/*.md") — no "ADR-" prefix.
#     Rationale: catches regressions in files other than adr.go.
#
# COMMENT FILTERING (both directions, mandatory):
#   Pure comment lines (optional whitespace + //) are skipped entirely.
#   A site that exists only in a comment is NOT a violation.
#   Inline trailing comments are stripped before pattern matching.
#   A comment mentioning WalkDir DOES NOT excuse a real os.ReadDir(adrDir) on
#   the same line: the real call is tested after stripping the comment.
#   Caveat: the inline strip uses ${var%%//*} which also truncates at //
#   inside string literals (e.g., URLs). Acceptable for Go source: ReadDir
#   with a URL-in-argument is not a real pattern.
#
# EXEMPTION MARKER:
#   "// adr-single-point-exempt: <reason>" on the flagged line itself or the
#   line immediately above it suppresses any pattern for that line.
#   The reason text is mandatory and auditable.
#
# EXEMPT SITES (documented; not all require file-skipping — see below):
#   1. internal/serve/api_file.go — buildAllowedDirs/buildRealAllowedDirs
#      iterate cfg.ADRDirs to build a file-serving security containment
#      boundary (access control list for the serve API), NOT to enumerate
#      ADR content. These functions contain NO os.ReadDir or filepath.Glob
#      calls today. Documented explicitly to prevent a future "fix" from
#      adding them under the mistaken belief this gate requires it.
#      *** This is a SECURITY BOUNDARY — do not rewrite it as an enumerator.
#
#   2. internal/generators/update.go — EXEMPTION REMOVED (ML-1C / #450).
#      ensureGlobalADRDirRegistered previously used filepath.Glob root-only;
#      now uses validator.WalkADRFilePaths (recursive). No exemption needed.
#
#   3. internal/discover/discover.go — pre-config scan; does NOT consume
#      cfg.ADRDirs. Uses os.ReadDir(dir) with generic variable names (dir, d).
#      Would not be caught by Patterns A/C anyway; documented for completeness.
#
# ANTI-VACUITY: reports how many Go files were examined. FAILS if zero files
#   are found in internal/ — refuses to report a vacuous pass.
#
# SELF-TEST (--self-test):
#   Run four arms using mktemp fixtures (never mutates the live tree).
#   Arm 1: new file with os.ReadDir(adrDir) → gate FAILS naming file:line.
#   Arm 2: files with only WalkDir/buildAllowedDirs-style code → gate PASSES.
#   Arm 3 (comment — BOTH directions):
#     3a: os.ReadDir(adrDir) ONLY in a comment → gate PASSES.
#     3b: real os.ReadDir(adrDir) with "// use WalkDir" comment above →
#         gate still FAILS (comment does not excuse the real call).
#   Arm 4: filepath.Glob( added to generators/adr.go → gate FAILS.
#
# ENV VAR OVERRIDE: ADR_ENUM_SCAN_DIR — overrides the repo root used for the
#   scan. Default: repo root (parent of scripts/). In self-test, set to a
#   mktemp dir containing synthetic internal/ tree.
#   Note: unlike check-init-preserves-user-config.sh which uses SCAFFOLD_FILE
#   for a single file, this gate overrides the whole scan root. Always use
#   the env var, never a positional argument — positional args are silently
#   ignored and the gate reads the live tree instead.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Scan root — override in self-test via ADR_ENUM_SCAN_DIR.
SCAN_ROOT="${ADR_ENUM_SCAN_DIR:-$REPO_ROOT}"

# Exemption marker text.
EXEMPT_MARKER="adr-single-point-exempt:"

# Files exempt from Pattern C (filepath.Glob with "ADR-" in arg).
# These have a documented legitimate reason; see header.
# Format: path relative to SCAN_ROOT/internal/
PATTERN_C_EXEMPT=(
    # generators/update.go exemption removed in ML-1C (#450): now uses
    # validator.WalkADRFilePaths (recursive) instead of filepath.Glob.
)

# ── helpers ─────────────────────────────────────────────────────────────────

is_pattern_c_exempt() {
    local abspath="$1"
    local relpath="${abspath#"$SCAN_ROOT/internal/"}"
    local entry
    for entry in "${PATTERN_C_EXEMPT[@]}"; do
        [[ "$relpath" == "$entry" ]] && return 0
    done
    return 1
}

# strip_inline_comment <text>
# Returns the text with everything from the first "//" stripped.
# This is a side-effect-free helper so that the same stripping logic
# is used consistently; the caveat (// in string literals) is accepted.
strip_inline_comment() {
    local s="$1"
    echo "${s%%//*}"
}

# is_pure_comment <text>
# Returns 0 (true) if the text is a pure comment line (optional whitespace + //).
is_pure_comment() {
    local stripped="${1#"${1%%[![:space:]]*}"}"   # ltrim
    [[ "$stripped" == //* ]]
}

# has_exempt_marker <file> <lineno>
# Returns 0 (true) if line LINENO or line LINENO-1 in FILE contains EXEMPT_MARKER.
has_exempt_marker() {
    local file="$1"
    local lineno="$2"
    local line
    line=$(sed -n "${lineno}p" "$file")
    if echo "$line" | grep -qF "$EXEMPT_MARKER"; then
        return 0
    fi
    if [[ $lineno -gt 1 ]]; then
        local prev
        prev=$(sed -n "$((lineno - 1))p" "$file")
        echo "$prev" | grep -qF "$EXEMPT_MARKER" && return 0
    fi
    return 1
}

# ── main scan ────────────────────────────────────────────────────────────────

run_scan() {
    local root="$1"
    local violations=0
    local files_examined=0

    # Collect Go production files (non-test).
    local go_files=()
    while IFS= read -r f; do
        go_files+=("$f")
    done < <( { find "$root/internal" -name '*.go' ! -name '*_test.go' -print 2>/dev/null | sort; } || true)

    if [[ ${#go_files[@]} -eq 0 ]]; then
        echo "FAIL: nenhum arquivo Go de producao encontrado em $root/internal — corpus vazio, recusando aprovacao silenciosa" >&2
        return 1
    fi

    for f in "${go_files[@]}"; do
        files_examined=$((files_examined + 1))
        local is_adr_go=0
        [[ "$f" == */generators/adr.go ]] && is_adr_go=1

        # ── Pattern A (name-based): os.ReadDir with "adr" in argument ──────
        # grep lines containing os.ReadDir(, then filter by comment and "adr".
        local readdir_hits=()
        while IFS= read -r hit; do
            [[ -n "$hit" ]] && readdir_hits+=("$hit")
        done < <( { grep -n 'os\.ReadDir(' "$f" || true; } )

        for hit in "${readdir_hits[@]+"${readdir_hits[@]}"}"; do
            local lineno="${hit%%:*}"
            local content="${hit#*:}"
            is_pure_comment "$content" && continue
            local clean
            clean=$(strip_inline_comment "$content")
            # Check for "adr" (case-insensitive) in the ReadDir argument
            if echo "$clean" | grep -qiE 'os\.ReadDir\([^)]*[Aa][Dd][Rr]'; then
                has_exempt_marker "$f" "$lineno" && continue
                echo "FAIL $f:$lineno [Pattern A] os.ReadDir with ADR-dir argument — use walkADRFilePaths or ResolveADRFiles"
                echo "     line: $content"
                violations=$((violations + 1))
            fi
        done

        # ── Pattern A' (loop-context): os.ReadDir(LOOPVAR) ─────────────────
        # Collect loop variables bound by "for _, VAR := range.*ADRDirs".
        # File-level extraction (not scope-tracked): if VAR is reused for a
        # non-ADR ReadDir, use adr-single-point-exempt: to suppress.
        local loop_vars=()
        while IFS= read -r lv; do
            [[ -n "$lv" ]] && loop_vars+=("$lv")
        done < <( { \
            sed -n 's/.*for[[:space:]]*_[[:space:]]*,[[:space:]]*\([A-Za-z_][A-Za-z0-9_]*\)[[:space:]]*:=[[:space:]]*range[[:space:]].*ADRDirs.*/\1/p' "$f" \
            | sort -u; \
        } || true)

        for lv in "${loop_vars[@]+"${loop_vars[@]}"}"; do
            local lv_hits=()
            while IFS= read -r hit; do
                [[ -n "$hit" ]] && lv_hits+=("$hit")
            done < <( { grep -n "os\.ReadDir($lv)" "$f" || true; } )

            for hit in "${lv_hits[@]+"${lv_hits[@]}"}"; do
                local lineno="${hit%%:*}"
                local content="${hit#*:}"
                is_pure_comment "$content" && continue
                local clean
                clean=$(strip_inline_comment "$content")
                # Only flag if clean line still contains the ReadDir call
                if echo "$clean" | grep -qE "os\.ReadDir\\($lv\\)"; then
                    # Skip if already caught by Pattern A (avoid double-counting)
                    if echo "$clean" | grep -qiE 'os\.ReadDir\([^)]*[Aa][Dd][Rr]'; then
                        continue
                    fi
                    has_exempt_marker "$f" "$lineno" && continue
                    echo "FAIL $f:$lineno [Pattern A'] os.ReadDir($lv) where $lv is a loop var from range.*ADRDirs — use walkADRFilePaths"
                    echo "     line: $content"
                    violations=$((violations + 1))
                fi
            done
        done

        # ── Pattern B (file-scope for generators/adr.go): any Glob ─────────
        if [[ $is_adr_go -eq 1 ]]; then
            local glob_hits=()
            while IFS= read -r hit; do
                [[ -n "$hit" ]] && glob_hits+=("$hit")
            done < <( { grep -n 'filepath\.Glob(' "$f" || true; } )

            for hit in "${glob_hits[@]+"${glob_hits[@]}"}"; do
                local lineno="${hit%%:*}"
                local content="${hit#*:}"
                is_pure_comment "$content" && continue
                has_exempt_marker "$f" "$lineno" && continue
                echo "FAIL $f:$lineno [Pattern B] filepath.Glob in generators/adr.go — use walkADRFilePaths instead"
                echo "     line: $content"
                violations=$((violations + 1))
            done
        fi

        # ── Pattern C (ADR- in Glob arg): filepath.Glob with "ADR-" ────────
        is_pattern_c_exempt "$f" && continue
        [[ $is_adr_go -eq 1 ]] && continue   # Pattern B already covers adr.go

        local glob_c_hits=()
        while IFS= read -r hit; do
            [[ -n "$hit" ]] && glob_c_hits+=("$hit")
        done < <( { grep -n 'filepath\.Glob(' "$f" || true; } )

        for hit in "${glob_c_hits[@]+"${glob_c_hits[@]}"}"; do
            local lineno="${hit%%:*}"
            local content="${hit#*:}"
            is_pure_comment "$content" && continue
            local clean
            clean=$(strip_inline_comment "$content")
            if echo "$clean" | grep -q 'ADR-'; then
                has_exempt_marker "$f" "$lineno" && continue
                echo "FAIL $f:$lineno [Pattern C] filepath.Glob with ADR- pattern — use walkADRFilePaths instead"
                echo "     line: $content"
                violations=$((violations + 1))
            fi
        done
    done

    echo ""
    echo "Files examined: $files_examined"

    if [[ $violations -gt 0 ]]; then
        echo "FAIL: $violations violation(s) — ADR enumeration outside single point (walkADRFilePaths/ResolveADRFiles)"
        return 1
    fi
    echo "PASS: no ADR enumeration violations"
    return 0
}

# ── self-test ────────────────────────────────────────────────────────────────

if [[ "${1:-}" == "--self-test" ]]; then
    TMPDIR_ST=$(mktemp -d)
    trap 'rm -rf "$TMPDIR_ST"' EXIT
    overall_rc=0

    # ── Arm 1: new os.ReadDir(adrDir) in a new file → gate MUST FAIL ────────
    mkdir -p "$TMPDIR_ST/arm1/internal/generators"
    cat > "$TMPDIR_ST/arm1/internal/generators/enumerate_test_fixture.go" <<'GOEOF'
package generators

import (
	"os"
	"path/filepath"
)

// BAD: root-only ReadDir over cfg.ADRDirs — the class-(iii) defect.
func badEnumerateADRs(cfg someConfig) []string {
	var result []string
	for _, adrDir := range cfg.ADRDirs {
		entries, _ := os.ReadDir(adrDir)   // Pattern A: adrDir in argument
		for _, e := range entries {
			if !e.IsDir() {
				result = append(result, filepath.Join(adrDir, e.Name()))
			}
		}
	}
	return result
}
GOEOF

    echo "=== Arm 1: os.ReadDir(adrDir) → gate MUST FAIL naming file:line ==="
    arm1_rc=0
    arm1_out=$(ADR_ENUM_SCAN_DIR="$TMPDIR_ST/arm1" bash "${BASH_SOURCE[0]}" 2>&1) || arm1_rc=$?
    echo "$arm1_out"
    if [[ $arm1_rc -ne 0 ]] && echo "$arm1_out" | grep -q "FAIL.*enumerate_test_fixture.go"; then
        echo "SELF-TEST arm1: PASS (gate correctly failed naming the file; rc=$arm1_rc)"
    else
        echo "SELF-TEST arm1: FAIL — expected gate to fail naming enumerate_test_fixture.go (rc=$arm1_rc)" >&2
        overall_rc=1
    fi
    echo ""

    # ── Arm 2: correct code only → gate MUST PASS ───────────────────────────
    mkdir -p "$TMPDIR_ST/arm2/internal/serve"
    mkdir -p "$TMPDIR_ST/arm2/internal/generators"
    # Correct enumerator using WalkDir
    cat > "$TMPDIR_ST/arm2/internal/generators/context_ok.go" <<'GOEOF'
package generators

import (
	"io/fs"
	"path/filepath"
)

// CORRECT: recursive WalkDir over adrDir — the single-point pattern.
func correctEnumerateADRs(adrDir string) []string {
	var result []string
	_ = filepath.WalkDir(adrDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result
}
GOEOF
    # buildAllowedDirs-style: iterates cfg.ADRDirs but builds paths, no ReadDir/Glob
    cat > "$TMPDIR_ST/arm2/internal/serve/api_file_ok.go" <<'GOEOF'
package serve

import (
	"path/filepath"
)

type fakeConfig struct {
	ADRDirs []string
}

// CORRECT: iterates cfg.ADRDirs to build a security boundary — no ReadDir/Glob.
func buildAllowedDirs(workDir string, cfg fakeConfig) []string {
	var dirs []string
	for _, d := range cfg.ADRDirs {
		dirs = append(dirs, filepath.Join(workDir, filepath.Clean(d)))
	}
	return dirs
}
GOEOF

    echo "=== Arm 2: WalkDir + buildAllowedDirs-style code only → gate MUST PASS ==="
    arm2_rc=0
    arm2_out=$(ADR_ENUM_SCAN_DIR="$TMPDIR_ST/arm2" bash "${BASH_SOURCE[0]}" 2>&1) || arm2_rc=$?
    echo "$arm2_out"
    if [[ $arm2_rc -eq 0 ]]; then
        echo "SELF-TEST arm2: PASS (gate correctly passed; rc=0)"
    else
        echo "SELF-TEST arm2: FAIL — expected gate to pass (rc=$arm2_rc)" >&2
        overall_rc=1
    fi
    echo ""

    # ── Arm 3: comment directions ────────────────────────────────────────────
    # 3a: os.ReadDir(adrDir) only in a comment → gate MUST PASS
    # 3b: real os.ReadDir(adrDir) with "// use WalkDir" comment above → gate MUST FAIL
    mkdir -p "$TMPDIR_ST/arm3/internal/generators"
    cat > "$TMPDIR_ST/arm3/internal/generators/comment_fixture.go" <<'GOEOF'
package generators

import (
	"io/fs"
	"path/filepath"
)

// COMMENT-ONLY: the old defective approach — do not use
// os.ReadDir(adrDir) is wrong because it only reads the root directory.
func correctWithDocComment(adrDir string) []string {
	var result []string
	// OLD: entries, _ := os.ReadDir(adrDir)  ← this was wrong, replaced by WalkDir
	_ = filepath.WalkDir(adrDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result
}

// REAL CALL: real os.ReadDir(adrDir) below, with WalkDir mentioned only in a comment.
// The comment does NOT excuse the real call — gate must still FAIL.
func badWithWalkdirComment(adrDir string) []string {
	// NOTE: we should use WalkDir(adrDir) here to recurse into subdirs
	entries, _ := os.ReadDir(adrDir)   // real call — must be flagged
	var result []string
	for _, e := range entries {
		_ = e
	}
	return result
}
GOEOF

    echo "=== Arm 3a: os.ReadDir(adrDir) only in comment → gate MUST PASS ==="
    echo "=== Arm 3b: real os.ReadDir(adrDir) with WalkDir comment above → gate MUST FAIL ==="
    arm3_rc=0
    arm3_out=$(ADR_ENUM_SCAN_DIR="$TMPDIR_ST/arm3" bash "${BASH_SOURCE[0]}" 2>&1) || arm3_rc=$?
    echo "$arm3_out"

    # 3a: gate must fail (because arm3 has the REAL call too), and the FAIL line must
    # name comment_fixture.go. More importantly, the comment-only function must NOT appear
    # on its own — we verify by checking that "correctWithDocComment" is NOT in the FAIL lines.
    # 3b: the real call in badWithWalkdirComment MUST be named.
    arm3a_ok=0
    arm3b_ok=0

    # 3a: "OLD:" comment line must NOT produce a FAIL
    if ! echo "$arm3_out" | grep "FAIL" | grep -q "OLD:"; then
        arm3a_ok=1
    fi
    # 3b: the real call must be named
    if [[ $arm3_rc -ne 0 ]] && echo "$arm3_out" | grep "FAIL" | grep -q "comment_fixture.go"; then
        arm3b_ok=1
    fi

    if [[ $arm3a_ok -eq 1 && $arm3b_ok -eq 1 ]]; then
        echo "SELF-TEST arm3: PASS (comment-only NOT flagged [3a]; real call flagged [3b]; rc=$arm3_rc)"
    else
        if [[ $arm3a_ok -eq 0 ]]; then
            echo "SELF-TEST arm3: FAIL [3a] — comment-only line was flagged, or comment content appeared in FAIL output" >&2
        fi
        if [[ $arm3b_ok -eq 0 ]]; then
            echo "SELF-TEST arm3: FAIL [3b] — real os.ReadDir(adrDir) was not flagged (rc=$arm3_rc)" >&2
        fi
        overall_rc=1
    fi
    echo ""

    # ── Arm 4: filepath.Glob in generators/adr.go → gate MUST FAIL ──────────
    mkdir -p "$TMPDIR_ST/arm4/internal/generators"
    cat > "$TMPDIR_ST/arm4/internal/generators/adr.go" <<'GOEOF'
package generators

import (
	"path/filepath"
)

// BAD: filepath.Glob in generators/adr.go — Pattern B.
func badListADRs(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	return matches, nil
}
GOEOF

    echo "=== Arm 4: filepath.Glob in generators/adr.go → gate MUST FAIL ==="
    arm4_rc=0
    arm4_out=$(ADR_ENUM_SCAN_DIR="$TMPDIR_ST/arm4" bash "${BASH_SOURCE[0]}" 2>&1) || arm4_rc=$?
    echo "$arm4_out"
    if [[ $arm4_rc -ne 0 ]] && echo "$arm4_out" | grep -q "FAIL.*adr.go"; then
        echo "SELF-TEST arm4: PASS (gate correctly failed naming adr.go; rc=$arm4_rc)"
    else
        echo "SELF-TEST arm4: FAIL — expected gate to fail naming adr.go (rc=$arm4_rc)" >&2
        overall_rc=1
    fi
    echo ""

    if [[ $overall_rc -eq 0 ]]; then
        echo "SELF-TEST: PASS (all 4 arms)"
    else
        echo "SELF-TEST: FAIL" >&2
    fi
    exit $overall_rc
fi

# ── real run ─────────────────────────────────────────────────────────────────
echo "--- Gate: adr-enumeration-single-point ---"
echo "Scan root: $SCAN_ROOT"
echo ""
run_scan "$SCAN_ROOT"
