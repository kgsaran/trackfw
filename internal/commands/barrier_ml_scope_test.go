package commands

// barrier_ml_scope_test.go — tests for the ml_scope check (D2, ML-2A,
// REQ-2026-10-23). Each test runs the trackfw barrier binary against a real
// git repository in t.TempDir() so that the contra-braço (running against
// HEAD without this file) produces a meaningful failure.
//
// # Reconciliation table (Regra Dura de Reconciliação)
//
// | Test name                              | What it asserts                                                   |
// |----------------------------------------|-------------------------------------------------------------------|
// | TestMLScope_DeclaredFilePasses         | A file listed in **Files affected:** is NOT in failures           |
// | TestMLScope_OutOfScopeFails            | A file NOT listed appears in failures with its path               |
// | TestMLScope_UntrackedOutOfScopeFails   | An untracked (not committed) out-of-scope file appears in failures|
// | TestMLScope_DiskEditDoesNotAmplifyScope| Editing Files affected on disk does not expand the scope          |
// | TestMLScope_OrchestrationSetPasses     | The roadmap file itself passes without being in Files affected    |
// | TestMLScope_DirectoryAndGlobMatch      | Directory prefix and glob patterns match their intended files     |
// | TestMLScope_PreCutoffSkipped           | Roadmap with date before 2026-07-18 exits skipped                 |
// | TestMLScope_WipMissingDeclarationFails | ML in wave without **Files affected:** causes blocked status      |
// | TestMLScopeFilesAffectedParsing        | parseMLScopeFilesAffected extracts backtick paths from single and multi-line declarations |
// | TestMLScopeMatchPath                   | mlScopeMatchPath handles exact, dir, glob and ** patterns         |
// | TestMLScope_NoOriginMain_Blocked               | Absent origin/main → status "blocked" (Decision 1, ML-4C)        |
// | TestMLScope_RoadmapMovedStatePasses_Flat       | Moved roadmap (done/→wip/) old path NOT in failures; different basename deleted on branch IS in failures (flat, ADR-2026-10-29 D2) |
// | TestMLScope_RoadmapMovedStatePasses_ByAgent    | Moved roadmap (agentA/done/→agentA/wip/) old path NOT in failures; same basename under different agent deleted on branch IS in failures (by_agent, ADR-2026-10-29 D2) |
// ── New tests (ML-1A, REQ-2026-10-29) ───────────────────────────────────────────────────────
// | TestMLScopeFilesAffectedList_BreakAtListItemReproducesDefect | List items after the label produce tokens (sabotage: restore break → this test fails)        |
// | TestMLScopeFilesAffectedList_AllTokensExtracted              | All backtick tokens from all list items are extracted (Regra B)                               |
// | TestMLScopeFilesAffectedList_StopsAtBoldField                | Standalone ** stops; contra-braço: "- **" does not stop                                       |
// | TestMLScopeFilesAffectedList_CheckmarkItemStops              | "- [ ]", "- [x]", "- [X]" stop the list before tokens leak                                   |
// | TestMLScopeFilesAffectedList_AnyCriterionMarkerStops         | Any single-char marker "- [~]", "- [-]" stops; multi-char link "- [texto longo](url)" does not |
// | TestMLScopeFilesAffectedHybridInlineAndList                  | Inline tokens on the label line AND list items below are all extracted (§1.4c)                |
// | TestMLScopeGlobBroadPatternBlocked                           | Broad patterns (**, **\/*, *\/**)  → status blocked with rejection message (AC10)            |
// | TestMLScopeGlobPrefixedPatternAccepted                       | Prefixed patterns (site/**, **\/*.go) are accepted and do not produce rejection                |
// | TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope     | File declared by union member B is in scope for target A's barrier (AC4)                      |
// | TestMLScope_UnionIgnoresWorkingTreeRoadmapEdit               | Disk edit of committed union member does not amplify scope (reads from HEAD)                  |
// | TestMLScope_TargetRoadmapExcludedFromMemberLoop              | Target excluded from member loop; broad-pattern rejection appears exactly once (AC5/P-6)      |
// | TestMLScope_UnionStartedWavesOnly_AllPendingContributesNothing | Union member with all-⬜ MLs contributes no patterns (K undefined)                           |
// | TestMLScope_UnionStartedWavesOnly_DoneWaveContributesPatterns | Union member with ✅ Wave 0 contributes Wave 0 patterns (K=0)                                |
// | TestMLScope_UnionAbandonedContributesOrchestrationOnly       | Member in abandoned/ contributes only orchestration, not its Files affected patterns          |
// | TestMLScope_UnionDoesNotIncludeUncommittedRoadmap            | A roadmap not in diffFiles (only on disk) is not a union member (AC5)                        |
// | TestMLScope_UnionDoneAtMergeBaseOrchestrationOnly            | Member in done/ that existed at merge-base contributes only orchestration                     |
// | TestMLScope_UnionDoneMovedInBranchContributesPatterns        | Member moved to done/ on the branch (not at merge-base) contributes patterns                 |
// | TestMLScope_ReqOutsideReqDirDiscarded                        | req: outside req_dir discarded; traversal/sibling rejected; "./" prefix accepted (AC11)       |
// | TestMLScope_AdrOutsideADRDirsDiscarded                       | adr: outside adr_dirs discarded; valid adr: path accepted (AC11)                             |
// | TestMLScope_UnionPreCutoffMemberOrchestrationOnly            | Union member with pre-2026-07-18 date contributes only orchestration                         |
// | TestMLScope_UnionMissingDateTreatedAsPostCutoff              | Union member with no date: field treated as post-cutoff; contributes patterns                |
// ── New tests (ML-1A, REQ-2026-10-31) ───────────────────────────────────────────────────────
// | TestMLScope_StartedWaveAboveN_PassesWhenFilesCommitted        | Files of started wave above N are in scope; --wave N passes (AC1)                            |
// | TestMLScope_StartedWaveAboveN_UndeclaredFileStillFails        | Undeclared file blocks even with started wave above N; named file only (AC2)                 |
// | TestMLScope_UnstartedWaveAmidStartedWavesAboveN_Fails/simple  | Wave ⬜ above N blocks; x.go out of scope (AC3 simple vector)                                |
// | TestMLScope_UnstartedWaveAmidStartedWavesAboveN_Fails/gap     | Wave ⬜ between N and started wave above N blocks x.go; y.go (started wave) passes (AC3 gap) |
// | TestMLScope_UnionMemberUnstartedWaveIsExcluded                | Union member wave ⬜ is not contributed even with started wave above it (AC3b)               |
// | TestMLScope_StartedWaveAboveN_NoDeclFailureForMissingDeclaration/ArmA | No declFailure for ML without **Files affected:** in started wave > N (AC4 arm A)   |
// | TestMLScope_StartedWaveAboveN_NoDeclFailureForMissingDeclaration/ArmB | declFailure still fired for ML without **Files affected:** in wave ≤ N (AC4 arm B)  |
// | TestMLScope_BroadPatternInStartedWaveAboveN_Blocked           | Broad pattern ** in started wave > N is rejected by addPatterns (P-3)                        |
// | TestMLScope_DiskEditOfWaveStatusDoesNotAmplifyScope/disk_edit | Disk edit marking wave ⬜ as 🔄 does not amplify scope (P-4)                                |
// | TestMLScope_DiskEditOfWaveStatusDoesNotAmplifyScope/committed | Control: committed 🔄 in HEAD includes wave's file in scope (P-4 control arm)               |
// | TestMLScopeWaveIsStarted_VS16Variant                          | mlScopeWaveIsStarted returns true for 🔄️ (VS16 emoji variant)                              |
// ── New tests (ML-3A, REQ-2026-10-31) — AC8, AC10 ───────────────────────────────────────────
// | TestMLScope_FractionalWave_UnstartedOutOfScope                | Wave "1b" ⬜ above --wave 1: side.go out of scope; with integer cmp side.go would be in scope (AC8 false-negative fix) |
// | TestMLScope_FractionalWave_UnstartedNoDeclFailure             | Wave "1b" ⬜ with no **Files affected:** above --wave 1: no declFailure (AC8 false-positive fix)                        |
// | TestMLScope_FractionalWave_StartedInScope                     | Wave "1b" 🔄 above --wave 1: side.go in scope (AC8 started variant)                                                   |
// | TestMLScope_FractionalWave_LowerWaveHasDeclFailure            | --wave 1b: wave "1" is ≤ N; ML without **Files affected:** in wave "1" gets declFailure (AC8 reverse direction)       |
// | TestMLScope_BlockedStatusAboveN_InScope                       | Wave 2 above N=1 with only ❌ Bloqueado: wave is started; its file is in scope (AC10, kills M8b)                      |
// | TestMLScope_TerminatedStatusAboveN_InScope                    | Wave 2 above N=1 with only ❌ Cancelado / 🚫 Abandonado: wave is started; its file is in scope (AC10, kills M8c)      |
// ── New tests (ML-3B, REQ-2026-10-31) — AC9 ─────────────────────────────────────────────────
// | TestMLScope_HTMLCommentedStatus_WaveNotStarted                | Wave > N whose only **Status:** is inside <!-- -->: wave is ⬜, file NOT in scope (AC9 scope widening) |
// | TestMLScope_HTMLCommentedStatus_MLSCompleteBlocked            | ML whose only ✅ is inside <!-- -->: mls_complete blocked (AC9 mls_complete)                        |
// ── New tests (ML-3C, REQ-2026-10-31) — AC11 ────────────────────────────────────────────────
// | TestMLScopeFilesAffected_LabelInComment_NotDeclaration        | **Files affected:** label-only inside <!-- -->: no file in scope; real label after comment found (AC11 label masked) |
// | TestMLScopeFilesAffected_ContinuationInComment_Truncated      | List item inside <!-- --> after a real label: the commented item does NOT contribute tokens (AC11 continuation break) |
// ── New tests (ML-3C ação 3, REQ-2026-10-31) — AC11b inline ──────────────────────────────────────
// | TestMLScopeFilesAffected_InlineComment_HidesToken             | Token inside <!-- --> on label line is not extracted; token outside is (AC11b)                          |
// | TestMLScopeFilesAffected_InlineComment_OpenNoClose             | Unclosed <!-- on label line discards tokens after it; continuation via htmlMask still blocked (AC11b)  |
// | TestMLScopeFilesAffected_InlineComment_BetweenTokens           | Closed then unclosed comment on label: only token between spans returned; endsOpen stops continuation |
// | TestMLScopeFilesAffected_InlineComment_ContinuationLine        | Inline <!-- --> on a list continuation line hides its token; token outside is returned (AC11b)         |

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// Git fixture helpers
// ────────────────────────────────────────────────────────────────────────────

// mlScopeGitEnv returns an isolated git environment for the temp directory base.
func mlScopeGitEnv(base string) []string {
	cfgPath := filepath.Join(base, "gitconfig")
	_ = os.WriteFile(cfgPath, []byte(
		"[user]\n\temail = test@trackfw\n\tname = trackfw test\n"+
			"[commit]\n\tgpgsign = false\n"+
			"[core]\n\thooksPath = /dev/null\n\tautocrlf = false\n"+
			"[gc]\n\tauto = 0\n"+
			"[maintenance]\n\tauto = false\n"), 0600)
	return append(os.Environ(),
		"HOME="+base,
		"GIT_CONFIG_GLOBAL="+cfgPath,
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)
}

// mlScopeGitRun runs a git command in dir with the given env and fails the test
// on error.
func mlScopeGitRun(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// mlScopeSetupGitRepo creates a minimal git repository in dir with:
//   - a committed trackfw.yaml
//   - the given roadmap content committed at roadmapRel
//   - refs/remotes/origin/main pointing to that commit
//
// Returns the physical path to dir (macOS /var → /private/var resolved).
func mlScopeSetupGitRepo(t *testing.T, dir, roadmapRel, roadmapContent string) string {
	t.Helper()
	// Resolve macOS symlinks so that git toplevel and filepath.Rel agree.
	if phys, err := filepath.EvalSymlinks(dir); err == nil {
		dir = phys
	}
	env := mlScopeGitEnv(dir)

	// Initialise repo.
	mlScopeGitRun(t, dir, env, "init", "-b", "main", dir)

	// Minimal trackfw.yaml.
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"),
		[]byte("req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs: []\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	// Write and commit the roadmap.
	roadmapPath := filepath.Join(dir, roadmapRel)
	if err := os.MkdirAll(filepath.Dir(roadmapPath), 0755); err != nil {
		t.Fatalf("mkdir roadmap dir: %v", err)
	}
	if err := os.WriteFile(roadmapPath, []byte(roadmapContent), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base")

	// Set origin/main to HEAD (no actual remote needed).
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	return dir
}

// mlScopeRunBarrier invokes `trackfw barrier <roadmap> --wave <wave> --json`
// from dir with an isolated environment. Returns (stdout, exit code).
func mlScopeRunBarrier(t *testing.T, dir, roadmapArg, wave string) (string, int) {
	t.Helper()
	bin := barrierBinary(t)
	base := dir
	env := mlScopeGitEnv(base)
	cmd := exec.Command(bin, "barrier", roadmapArg, "--wave", wave, "--json", "--trust-local-gates")
	cmd.Dir = dir
	cmd.Env = env
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("mlScopeRunBarrier: unexpected error: %v\nstderr: %s", err, errBuf.String())
		}
	}
	return outBuf.String(), code
}

// findMLScopeCheck returns the ml_scope check document from a JSON barrier result.
// Fails the test if ml_scope is absent.
func findMLScopeCheck(t *testing.T, stdout string) barrierCheckDoc {
	t.Helper()
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	for _, c := range doc.Checks {
		if c.Name == "ml_scope" {
			return c
		}
	}
	t.Fatalf("ml_scope check not found in barrier output; checks present: %v\nstdout: %s",
		func() []string {
			names := make([]string, len(doc.Checks))
			for i, c := range doc.Checks {
				names[i] = c.Name
			}
			return names
		}(),
		stdout)
	return barrierCheckDoc{}
}

// buildMLScopeRoadmap returns a roadmap string for a Wave 1 fixture.
// filesAffectedLine is the content of the **Files affected:** field (e.g.
// "`foo.txt`"). Pass "" to omit the line entirely (undeclared).
func buildMLScopeRoadmap(date, filesAffectedLine string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nstatus: wip\ndate: %s\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n", date)
	b.WriteString("# Roadmap: ML Scope Test Fixture\n\n")
	b.WriteString("## Acceptance Criteria\n- [x] fixture criterion\n\n")
	// Wave 0 (required by validate)
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	// Wave 1
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0 auditada.\n\n")
	b.WriteString("### ML-1A — The ML\n**Status:** ✅ Concluído\n")
	if filesAffectedLine != "" {
		fmt.Fprintf(&b, "**Files affected:** %s\n", filesAffectedLine)
	}
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	return b.String()
}

// writeMLScopeREQFixture writes a minimal REQ file into the fixture dir so that
// validate remains green (wip_has_req, ref_targets_exist).
func writeMLScopeREQFixture(t *testing.T, dir, roadmapRel string) {
	t.Helper()
	adr := "---\nstatus: Accepted\ndate: 2026-07-20\n---\n\n# ADR: ML Scope Fixture\n\n> Status: Accepted\n"
	adrPath := filepath.Join(dir, "docs/adr/ADR-ml-scope-fixture.md")
	if err := os.WriteFile(adrPath, []byte(adr), 0644); err != nil {
		t.Fatalf("write adr: %v", err)
	}
	req := fmt.Sprintf("---\nstatus: Open\ndate: 2026-07-20\nadr: \"docs/adr/ADR-ml-scope-fixture.md\"\nroadmap: \"%s\"\n---\n\n# REQ: ML Scope Fixture\n\n## Acceptance Criteria\n- [ ] fixture\n", roadmapRel)
	reqPath := filepath.Join(dir, "docs/req/REQ-ml-scope-fixture.md")
	if err := os.WriteFile(reqPath, []byte(req), 0644); err != nil {
		t.Fatalf("write req: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T1 — Declared file passes
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_DeclaredFilePasses asserts that a file explicitly listed in
// **Files affected:** is allowed (in scope) and the ml_scope check passes.
func TestMLScope_DeclaredFilePasses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	content := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	// Commit the REQ/ADR files and add them to origin/main.
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Now create the declared file after origin/main (so it appears in the diff).
	if err := os.MkdirAll(filepath.Join(dir, "internal/commands"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/commands/foo.go"), []byte("package commands\n"), 0644); err != nil {
		t.Fatalf("write foo.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", "internal/commands/foo.go")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add foo.go")

	stdout, code := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	// The barrier may exit 1 due to validate (wip_has_req etc.) but we only check ml_scope.
	_ = code
	if check.Status == "blocked" {
		for _, f := range check.Failures {
			if strings.Contains(f, "internal/commands/foo.go") {
				t.Fatalf("declared file internal/commands/foo.go must not appear in failures; got %q", f)
			}
		}
	}
	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run (not skipped) in a git repo; evidence: %v", check.Evidence)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T2 — Out-of-scope file fails and path appears
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_OutOfScopeFails asserts that a committed file NOT listed in any
// **Files affected:** is reported in Failures with its exact repo-relative path.
func TestMLScope_OutOfScopeFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	// Declare only foo.go; the test will touch bar.go.
	content := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Touch bar.go (not in Files affected).
	if err := os.MkdirAll(filepath.Join(dir, "internal/commands"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/commands/bar.go"), []byte("package commands\n"), 0644); err != nil {
		t.Fatalf("write bar.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", "internal/commands/bar.go")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add bar.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked, got %q; failures: %v", check.Status, check.Failures)
	}
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "internal/commands/bar.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected internal/commands/bar.go in ml_scope failures; got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T3 — Untracked out-of-scope file fails
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UntrackedOutOfScopeFails asserts that a file that exists on disk
// but has NOT been committed also triggers the ml_scope failure when it is
// not in any **Files affected:** declaration.
func TestMLScope_UntrackedOutOfScopeFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	content := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Create untracked.go on disk without git add or commit.
	if err := os.MkdirAll(filepath.Join(dir, "internal/commands"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/commands/untracked.go"), []byte("package commands\n"), 0644); err != nil {
		t.Fatalf("write untracked.go: %v", err)
	}

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked for untracked file, got %q", check.Status)
	}
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "internal/commands/untracked.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected untracked.go in ml_scope failures; got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T4 — Disk edit of roadmap does not amplify scope
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_DiskEditDoesNotAmplifyScope asserts that editing the roadmap on
// disk to add extra.go to **Files affected:** does NOT prevent ml_scope from
// blocking, because the check reads the roadmap from the HEAD commit, not disk.
func TestMLScope_DiskEditDoesNotAmplifyScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	content := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Overwrite the roadmap on disk to include extra.go (NOT committed).
	expanded := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`, `internal/commands/extra.go`")
	roadmapPath := filepath.Join(dir, roadmapRel)
	if err := os.WriteFile(roadmapPath, []byte(expanded), 0644); err != nil {
		t.Fatalf("write expanded roadmap: %v", err)
	}

	// Commit extra.go (which the HEAD roadmap does NOT declare).
	if err := os.MkdirAll(filepath.Join(dir, "internal/commands"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/commands/extra.go"), []byte("package commands\n"), 0644); err != nil {
		t.Fatalf("write extra.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", "internal/commands/extra.go")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add extra.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked (HEAD roadmap does not declare extra.go), got %q\nfailures: %v",
			check.Status, check.Failures)
	}
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "internal/commands/extra.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected extra.go in ml_scope failures (disk edit must not amplify scope); got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T5 — Orchestration set passes
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_OrchestrationSetPasses asserts that the roadmap file itself (always
// in the orchestration set) does not trigger an ml_scope failure, even when no
// **Files affected:** line names it explicitly.
func TestMLScope_OrchestrationSetPasses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	// Declare an unrelated file; the roadmap itself is NOT listed.
	content := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Modify the roadmap itself (it's in orchestration set) and the declared file.
	updatedContent := buildMLScopeRoadmap("2026-08-01", "`internal/commands/foo.go`") + "\n<!-- updated -->\n"
	if err := os.WriteFile(filepath.Join(dir, roadmapRel), []byte(updatedContent), 0644); err != nil {
		t.Fatalf("write updated roadmap: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal/commands"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/commands/foo.go"), []byte("package commands\n"), 0644); err != nil {
		t.Fatalf("write foo.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "update roadmap + foo.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run (not skipped); evidence: %v", check.Evidence)
	}
	for _, f := range check.Failures {
		if strings.Contains(f, roadmapRel) || strings.Contains(f, "ROADMAP-ml-scope-fixture") {
			t.Fatalf("roadmap file must not appear in ml_scope failures (orchestration set); got: %q", f)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T6 — Directory and glob patterns match
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_DirectoryAndGlobMatch asserts that a directory prefix pattern
// (ending in "/") and a glob pattern (containing "*") each match the files
// they are expected to cover.
func TestMLScope_DirectoryAndGlobMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	// Declare a directory and a glob.
	content := buildMLScopeRoadmap("2026-08-01", "`internal/pkg/`, `scripts/*.sh`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Create files matched by the directory prefix and the glob.
	if err := os.MkdirAll(filepath.Join(dir, "internal/pkg"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/pkg/util.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write util.go: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts/deploy.sh"), []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("write deploy.sh: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add matched files")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run, got skipped; evidence: %v", check.Evidence)
	}
	// Neither file should appear in failures.
	for _, f := range check.Failures {
		if strings.Contains(f, "internal/pkg/util.go") {
			t.Fatalf("internal/pkg/util.go must be allowed by directory prefix; failure: %q", f)
		}
		if strings.Contains(f, "scripts/deploy.sh") {
			t.Fatalf("scripts/deploy.sh must be allowed by glob; failure: %q", f)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T7 — Pre-cutoff roadmap is skipped
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_PreCutoffSkipped asserts that a roadmap with a frontmatter date
// before 2026-07-18 causes the ml_scope check to return "skipped" with an
// explicit reason, not "blocked".
func TestMLScope_PreCutoffSkipped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	// Date before the 2026-07-18 cutoff.
	content := buildMLScopeRoadmap("2026-05-10", "`internal/commands/foo.go`")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "skipped" {
		t.Fatalf("expected ml_scope skipped for pre-cutoff roadmap (2026-05-10), got %q\nfailures: %v",
			check.Status, check.Failures)
	}
	// Evidence must mention the reason.
	found := false
	for _, e := range check.Evidence {
		if strings.Contains(e, "2026-05-10") || strings.Contains(e, "cutoff") || strings.Contains(e, "predates") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skip reason in evidence; got: %v", check.Evidence)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T8 — WIP ML missing **Files affected:** fails
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_WipMissingDeclarationFails asserts that a post-cutoff ML in wave N
// with no **Files affected:** line causes ml_scope to report "blocked" and names
// the ML in Failures.
func TestMLScope_WipMissingDeclarationFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	// Empty string means: omit the **Files affected:** line entirely.
	content := buildMLScopeRoadmap("2026-08-01", "")
	dir = mlScopeSetupGitRepo(t, dir, roadmapRel, content)
	writeMLScopeREQFixture(t, dir, roadmapRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked for ML without Files affected, got %q\nevidence: %v\nfailures: %v",
			check.Status, check.Evidence, check.Failures)
	}
	// At least one failure must name the undeclared ML.
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "ML-1A") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ML-1A named in ml_scope failures; got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Unit tests — parseMLScopeFilesAffected
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedParsing asserts that parseMLScopeFilesAffected extracts
// backtick-quoted values from a **Files affected:** line and returns (nil, false)
// when the line is absent.
func TestMLScopeFilesAffectedParsing(t *testing.T) {
	cases := []struct {
		name       string
		lines      []string
		wantPaths  []string
		wantHasDecl bool
	}{
		{
			name:        "single path",
			lines:       []string{"**Files affected:** `foo/bar.go`"},
			wantPaths:   []string{"foo/bar.go"},
			wantHasDecl: true,
		},
		{
			name:        "multiple paths",
			lines:       []string{"**Files affected:** `a.go`, `b.go`, `c/d.go`"},
			wantPaths:   []string{"a.go", "b.go", "c/d.go"},
			wantHasDecl: true,
		},
		{
			name:        "a definir returns empty declared",
			lines:       []string{"**Files affected:** a definir"},
			wantPaths:   []string{},
			wantHasDecl: true,
		},
		{
			name:        "no Files affected line",
			lines:       []string{"**Status:** ✅", "some other line"},
			wantPaths:   nil,
			wantHasDecl: false,
		},
		{
			name:        "glob pattern preserved",
			lines:       []string{"**Files affected:** `scripts/*.sh`"},
			wantPaths:   []string{"scripts/*.sh"},
			wantHasDecl: true,
		},
		{
			name: "multi-line declaration continuation",
			lines: []string{
				"**Files affected:** `internal/generators/agentfiles.go`, some natural language text",
				"deste repositório em `CLAUDE.md`, `AGENTS.md` e `GEMINI.md` (a mesma linha, editada igual).",
			},
			wantPaths:   []string{"internal/generators/agentfiles.go", "CLAUDE.md", "AGENTS.md", "GEMINI.md"},
			wantHasDecl: true,
		},
		{
			name: "continuation stops at blank line",
			lines: []string{
				"**Files affected:** `foo.go`",
				"",
				"`bar.go`",
			},
			wantPaths:   []string{"foo.go"},
			wantHasDecl: true,
		},
		{
			name: "continuation stops at new bold field",
			lines: []string{
				"**Files affected:** `foo.go`",
				"**Acceptance criteria:**",
				"- [x] done",
			},
			wantPaths:   []string{"foo.go"},
			wantHasDecl: true,
		},
		{
			name:        "directory suffix preserved",
			lines:       []string{"**Files affected:** `internal/pkg/`"},
			wantPaths:   []string{"internal/pkg/"},
			wantHasDecl: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fenced := make([]bool, len(tc.lines))
			paths, hasDecl := parseMLScopeFilesAffected(tc.lines, fenced, 0, len(tc.lines))
			if hasDecl != tc.wantHasDecl {
				t.Fatalf("hasDecl: got %v, want %v", hasDecl, tc.wantHasDecl)
			}
			if tc.wantPaths == nil && paths != nil {
				t.Fatalf("expected nil paths, got %v", paths)
			}
			if tc.wantPaths != nil {
				if len(paths) != len(tc.wantPaths) {
					t.Fatalf("paths: got %v, want %v", paths, tc.wantPaths)
				}
				for i, p := range tc.wantPaths {
					if paths[i] != p {
						t.Fatalf("paths[%d]: got %q, want %q", i, paths[i], p)
					}
				}
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Unit tests — mlScopeMatchPath
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeMatchPath asserts the matching semantics for exact paths, directory
// prefixes, single-level globs and ** globs.
func TestMLScopeMatchPath(t *testing.T) {
	cases := []struct {
		pattern string
		file    string
		want    bool
	}{
		// Exact path
		{"foo/bar.go", "foo/bar.go", true},
		{"foo/bar.go", "foo/baz.go", false},
		// Directory prefix
		{"internal/pkg/", "internal/pkg/util.go", true},
		{"internal/pkg/", "internal/pkg/sub/deep.go", true},
		{"internal/pkg/", "internal/pkgx/other.go", false},
		// Directory must not match the .md sibling (FP guard from Wave 0 §3 D3 table)
		{"docs/agents-working-context/", "docs/agents-working-context.md", false},
		// Single-level glob
		{"scripts/*.sh", "scripts/deploy.sh", true},
		{"scripts/*.sh", "scripts/sub/deploy.sh", false},
		{"*.go", "main.go", true},
		{"*.go", "sub/main.go", false},
		// Double-star glob
		{"internal/**/*.go", "internal/commands/barrier.go", true},
		{"internal/**/*.go", "internal/a/b/c.go", true},
		{"internal/**/*.go", "internal/a/b/c.sh", false},
		{"internal/**/*.go", "other/a.go", false},
		// ** with zero segments
		{"docs/**/notes", "docs/notes", true},
		{"docs/**/notes", "docs/foo/notes", true},
		// Edge cases
		{"", "foo.go", false},
		{"foo.go", "", false},
	}
	for _, tc := range cases {
		got := mlScopeMatchPath(tc.pattern, tc.file)
		if got != tc.want {
			t.Errorf("mlScopeMatchPath(%q, %q) = %v, want %v", tc.pattern, tc.file, got, tc.want)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T10 — No origin/main → blocked (Decision 1, ML-4C)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_NoOriginMain_Blocked asserts that when refs/remotes/origin/main is absent,
// ml_scope reports "blocked" (not "skipped").
//
// Rationale (Decision 1, ML-4C): "never pass in silence" — a missing origin/main means the
// diff cannot be computed. Returning "skipped" would silently pass in shallow clones or repos
// without a configured remote, creating a gap in the D2 audit layer. "blocked" is the
// fail-closed choice.
//
// Asserts: Decision 1 (ML-4C) — absent origin/main → status "blocked".
// Contra-braço: PASSES on HEAD (code already returns "blocked" — test pins existing behaviour).
func TestMLScope_NoOriginMain_Blocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	roadmapRel := "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	// Post-cutoff date with a declared file so the only reason to block/skip is origin/main.
	content := buildMLScopeRoadmap("2026-10-01", "`internal/commands/foo.go`")

	// Use a subset of mlScopeSetupGitRepo: initialise repo but do NOT create origin/main.
	if phys, err := filepath.EvalSymlinks(dir); err == nil {
		dir = phys
	}
	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "init", "-b", "main", dir)

	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"),
		[]byte("req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs: []\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	roadmapPath := filepath.Join(dir, roadmapRel)
	if err := os.MkdirAll(filepath.Dir(roadmapPath), 0755); err != nil {
		t.Fatalf("mkdir roadmap dir: %v", err)
	}
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	writeMLScopeREQFixture(t, dir, roadmapRel)

	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base")
	// ← intentionally NO update-ref refs/remotes/origin/main here.

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked when origin/main absent, got %q\nevidence: %v\nfailures: %v",
			check.Status, check.Evidence, check.Failures)
	}
	// The failure message must mention origin/main.
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "origin/main") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'origin/main' in failures; got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T11 — Roadmap moved between states: old path is not out-of-scope (flat mode)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_RoadmapMovedStatePasses_Flat asserts that when a roadmap is moved
// from one state directory to another on the branch (e.g. done/ → wip/ on
// reopen), the deleted old-state path is NOT reported as "out of scope" — it is
// part of the roadmap's orchestration set (same basename, any state in
// roadmap_dir, flat mode).
//
// Counter-arm: docs/roadmaps/done/ROADMAP-ml-scope-moved-other.md (different
// basename) is committed in origin/main and then deleted (git rm) on the branch.
// The deleted path appears in diffFiles but does NOT exist in HEAD, so it is
// not a union member (ADR-2026-10-29 D2). It also has a different basename from
// the target, so it is not in the target's orchestration set — and IS reported
// as out of scope. This proves the diff was evaluated and the fix is basename-exact.
//
// Asserts: orchestration set includes same-basename paths across all roadmap
// states (flat), so a moved roadmap does not produce a spurious "out of scope"
// for the old path.
// Contra-braço: FAILS on HEAD — old code includes only the current path in the
// orchestration set, so done/ROADMAP-ml-scope-moved.md is flagged out of scope.
func TestMLScope_RoadmapMovedStatePasses_Flat(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	if phys, err := filepath.EvalSymlinks(dir); err == nil {
		dir = phys
	}
	env := mlScopeGitEnv(dir)

	const movedBase = "ROADMAP-ml-scope-moved.md"
	const otherBase = "ROADMAP-ml-scope-moved-other.md"
	content := buildMLScopeRoadmap("2026-10-01", "`internal/commands/ml-scope-moved.go`")
	// Distinct content for the counter-arm so that --no-renames never pairs it
	// with movedBase (distinct files, distinct declared patterns).
	otherContent := buildMLScopeRoadmap("2026-10-01", "`internal/commands/ml-scope-moved-other.go`")

	// ── Phase 1: build origin/main with roadmap at done/ ───────────────────
	mlScopeGitRun(t, dir, env, "init", "-b", "main", dir)
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/roadmaps/analyzing",
		"docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"),
		[]byte("req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs: []\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	// Roadmap at done/ for origin/main.
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/done", movedBase), []byte(content), 0644); err != nil {
		t.Fatalf("write done roadmap: %v", err)
	}
	// Counter-arm: different basename at done/ in origin/main.
	// Deleted on the branch; the deleted path is in diffFiles but not in HEAD →
	// not a union member (ADR-2026-10-29 D2). Different basename → not in the
	// target's orchestration set → must appear in failures.
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/done", otherBase), []byte(otherContent), 0644); err != nil {
		t.Fatalf("write other roadmap in done/: %v", err)
	}
	// REQ+ADR fixtures (using the post-move wip/ path as the roadmap ref is fine;
	// ml_scope only reads req: from the roadmap frontmatter to locate the REQ file).
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+movedBase)

	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base: roadmap in done/ + counter-arm other in done/")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// ── Phase 2: move roadmap to wip/, delete counter-arm ──────────────────
	mlScopeGitRun(t, dir, env, "rm", "-q", "docs/roadmaps/done/"+movedBase)
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip", movedBase), []byte(content), 0644); err != nil {
		t.Fatalf("write wip roadmap: %v", err)
	}
	// Delete the counter-arm. The deleted path appears in diffFiles but not in
	// HEAD → not a union member. Different basename → not in orchestration →
	// must appear in failures (ADR-2026-10-29 D2, contra-braço).
	mlScopeGitRun(t, dir, env, "rm", "-q", "docs/roadmaps/done/"+otherBase)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "chore: reopen — move roadmap to wip/")

	// ── Check ───────────────────────────────────────────────────────────────
	stdout, _ := mlScopeRunBarrier(t, dir, movedBase, "1")
	check := findMLScopeCheck(t, stdout)

	// Anti-vacuity: the check must have run.
	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run (not skipped); evidence: %v", check.Evidence)
	}

	// Primary assertion: done/movedBase must NOT appear in failures.
	oldPath := "docs/roadmaps/done/" + movedBase
	for _, f := range check.Failures {
		if strings.Contains(f, oldPath) {
			t.Fatalf("old roadmap path %q must not be in failures after move to wip/; failure: %q\nall failures: %v",
				oldPath, f, check.Failures)
		}
	}

	// Counter-arm: a different basename under done/ MUST appear in failures.
	otherRel := "docs/roadmaps/done/" + otherBase
	foundOther := false
	for _, f := range check.Failures {
		if strings.Contains(f, otherRel) {
			foundOther = true
			break
		}
	}
	if !foundOther {
		t.Fatalf("counter-arm: %q must appear in failures (different basename → out of scope); failures: %v",
			otherRel, check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T12 — Roadmap moved between states: old path is not out-of-scope (by_agent)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_RoadmapMovedStatePasses_ByAgent asserts the same contract as
// TestMLScope_RoadmapMovedStatePasses_Flat in by_agent namespacing mode.
//
// Counter-arm: docs/roadmaps/zeus-tf/done/ROADMAP-ml-scope-byagent-moved.md
// (same basename, different agent) is committed in origin/main and then deleted
// (git rm) on the branch. The deleted path appears in diffFiles but NOT in HEAD
// → not a union member (ADR-2026-10-29 D2). It is outside the target's
// orchestration (different agent path prefix) → IS reported as out of scope.
//
// Asserts: orchestration set includes same-basename paths across all roadmap
// states for the owning agent (by_agent mode), but NOT for other agents.
// Contra-braço: FAILS on HEAD — old code includes only the current path in the
// orchestration set, so agentA/done/ROADMAP-ml-scope-byagent-moved.md is flagged
// out of scope.
func TestMLScope_RoadmapMovedStatePasses_ByAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	if phys, err := filepath.EvalSymlinks(dir); err == nil {
		dir = phys
	}
	env := mlScopeGitEnv(dir)

	const agentA = "apolo-tf"
	const agentB = "zeus-tf"
	const movedBase = "ROADMAP-ml-scope-byagent-moved.md"
	content := buildMLScopeRoadmap("2026-10-01", "`internal/commands/ml-scope-byagent.go`")
	// Distinct content for the counter-arm (different agent's done/) so that
	// --no-renames never silently pairs it with the target via content match.
	otherContent := buildMLScopeRoadmap("2026-10-01", "`internal/commands/ml-scope-byagent-other.go`")

	// ── Phase 1: build origin/main with roadmap at agentA/done/ ────────────
	mlScopeGitRun(t, dir, env, "init", "-b", "main", dir)
	for _, agent := range []string{agentA, agentB} {
		for _, state := range []string{"wip", "done", "backlog", "blocked", "abandoned", "analyzing"} {
			if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps", agent, state), 0755); err != nil {
				t.Fatalf("mkdirs %s/%s: %v", agent, state, err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs/req"), 0755); err != nil {
		t.Fatalf("mkdirs req: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs/adr"), 0755); err != nil {
		t.Fatalf("mkdirs adr: %v", err)
	}
	trackfwYAML := "req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs: []\n" +
		"roadmap_namespacing: by_agent\nagents:\n  - " + agentA + "\n  - " + agentB + "\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(trackfwYAML), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	// Roadmap at agentA/done/ for origin/main.
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps", agentA, "done", movedBase), []byte(content), 0644); err != nil {
		t.Fatalf("write agentA done roadmap: %v", err)
	}
	// Counter-arm: same basename under agentB/done/ at origin/main.
	// Deleted on the branch; the deleted path is in diffFiles but not in HEAD →
	// not a union member (ADR-2026-10-29 D2). Outside agentA's orchestration
	// siblings (different agent path) → must appear in failures.
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps", agentB, "done", movedBase), []byte(otherContent), 0644); err != nil {
		t.Fatalf("write agentB done roadmap in origin/main: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/"+agentA+"/wip/"+movedBase)

	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base: by_agent roadmap in agentA/done/ + counter-arm in agentB/done/")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// ── Phase 2: move roadmap to agentA/wip/, delete counter-arm ──────────
	mlScopeGitRun(t, dir, env, "rm", "-q", "docs/roadmaps/"+agentA+"/done/"+movedBase)
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps", agentA, "wip", movedBase), []byte(content), 0644); err != nil {
		t.Fatalf("write agentA wip roadmap: %v", err)
	}
	// Delete the counter-arm. The deleted path appears in diffFiles but not in
	// HEAD → not a union member. Outside agentA's orchestration → must appear in
	// failures (ADR-2026-10-29 D2, contra-braço).
	mlScopeGitRun(t, dir, env, "rm", "-q", "docs/roadmaps/"+agentB+"/done/"+movedBase)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "chore: reopen — move roadmap to agentA/wip/")

	// ── Check ───────────────────────────────────────────────────────────────
	stdout, _ := mlScopeRunBarrier(t, dir, movedBase, "1")
	check := findMLScopeCheck(t, stdout)

	// Anti-vacuity.
	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run (not skipped); evidence: %v", check.Evidence)
	}

	// Primary assertion: agentA/done/movedBase must NOT appear in failures.
	oldPath := "docs/roadmaps/" + agentA + "/done/" + movedBase
	for _, f := range check.Failures {
		if strings.Contains(f, oldPath) {
			t.Fatalf("old roadmap path %q must not be in failures after move to agentA/wip/; failure: %q\nall failures: %v",
				oldPath, f, check.Failures)
		}
	}

	// Counter-arm: same basename under a different agent MUST appear in failures.
	otherRel := "docs/roadmaps/" + agentB + "/done/" + movedBase
	foundOther := false
	for _, f := range check.Failures {
		if strings.Contains(f, otherRel) {
			foundOther = true
			break
		}
	}
	if !foundOther {
		t.Fatalf("counter-arm: %q must appear in failures (different agent → out of scope); failures: %v",
			otherRel, check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// New test helpers (ML-1A, REQ-2026-10-29)
// ────────────────────────────────────────────────────────────────────────────

// buildMLScopeRoadmapStatus builds a roadmap where ML-1A has the given status
// and filesAffectedLine (empty = omit).
func buildMLScopeRoadmapStatus(date, status, filesAffectedLine string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nstatus: wip\ndate: %s\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n", date)
	b.WriteString("# Roadmap: ML Scope Test Fixture\n\n")
	b.WriteString("## Acceptance Criteria\n- [x] fixture criterion\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0 auditada.\n\n")
	fmt.Fprintf(&b, "### ML-1A — The ML\n**Status:** %s\n", status)
	if filesAffectedLine != "" {
		fmt.Fprintf(&b, "**Files affected:** %s\n", filesAffectedLine)
	}
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	return b.String()
}

// buildMLScopeRoadmapListForm builds a roadmap where ML-1A ✅ declares files in
// list format (one item per element).
func buildMLScopeRoadmapListForm(date string, listItems []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nstatus: wip\ndate: %s\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n", date)
	b.WriteString("# Roadmap: ML Scope Test Fixture\n\n")
	b.WriteString("## Acceptance Criteria\n- [x] fixture criterion\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0 auditada.\n\n")
	b.WriteString("### ML-1A — The ML\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:**\n")
	for _, item := range listItems {
		fmt.Fprintf(&b, "- `%s`\n", item)
	}
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	return b.String()
}

// mlScopeSetupMinimalGitRepo initialises a bare git repository in dir with a
// trackfw.yaml that sets adr_dirs: [docs/adr], all state directories and an
// initial commit, with refs/remotes/origin/main pointing to it. Returns the
// physical path (macOS /var → /private/var resolved).
func mlScopeSetupMinimalGitRepo(t *testing.T, dir string) string {
	t.Helper()
	if phys, err := filepath.EvalSymlinks(dir); err == nil {
		dir = phys
	}
	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "init", "-b", "main", dir)
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/roadmaps/analyzing",
		"docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mlScopeSetupMinimalGitRepo: mkdirs: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"),
		[]byte("req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs: [docs/adr]\n"), 0644); err != nil {
		t.Fatalf("mlScopeSetupMinimalGitRepo: write trackfw.yaml: %v", err)
	}
	return dir
}

// ────────────────────────────────────────────────────────────────────────────
// T13 — List items after the label produce tokens (AC1, sabotage arm)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedList_BreakAtListItemReproducesDefect asserts that a
// **Files affected:** declaration followed by list items produces the tokens from
// those items. Sabotage: restoring the original `break` in the `- ` branch of
// parseMLScopeFilesAffected causes this test to fail — proving it detects the defect.
func TestMLScopeFilesAffectedList_BreakAtListItemReproducesDefect(t *testing.T) {
	lines := []string{
		"**Files affected:**",
		"- `npm/README.md` e `pypi/README.md`: removidos do git",
		"- `scripts/install.sh`",
	}
	fenced := make([]bool, len(lines))
	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatal("expected hasDecl=true")
	}
	wantAll := []string{"npm/README.md", "pypi/README.md", "scripts/install.sh"}
	for _, w := range wantAll {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q must be in patterns; got %v (list items broken — break in '- ' branch reintroduced?)", w, patterns)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T14 — All backtick tokens from all list items are extracted (Regra B)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedList_AllTokensExtracted asserts that all backtick-quoted
// tokens from every list item below the label are collected (Regra B: same primitive
// as the inline parser, applied per-item).
func TestMLScopeFilesAffectedList_AllTokensExtracted(t *testing.T) {
	lines := []string{
		"**Files affected:**",
		"- `a.go` (novo)",
		"- `b/c.go`: updated",
		"- `d/e.go`, `f.go`",
	}
	fenced := make([]bool, len(lines))
	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatal("expected hasDecl=true")
	}
	want := []string{"a.go", "b/c.go", "d/e.go", "f.go"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want %q in patterns; got %v", w, patterns)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T15 — Standalone bold field stops the list; "- **" does not (AC2)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedList_StopsAtBoldField asserts that a standalone bold
// field line ("**Actions:**") terminates the list.
// Contra-braço: a list item that STARTS with "- **bold**" is still a list item
// and must NOT stop the parser.
func TestMLScopeFilesAffectedList_StopsAtBoldField(t *testing.T) {
	// Primary: bold field stops the list.
	lines := []string{
		"**Files affected:**",
		"- `a.go`",
		"**Actions:**",
		"- `b.go`",
	}
	fenced := make([]bool, len(lines))
	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatal("expected hasDecl=true")
	}
	if len(patterns) != 1 || patterns[0] != "a.go" {
		t.Errorf("expected only [a.go]; got %v (list must stop at bold field)", patterns)
	}

	// Contra-braço: "- **bold** `path`" is a list item, not a bold field — must NOT stop.
	lines2 := []string{
		"**Files affected:**",
		"- **`pypi/pyproject.toml` fica.** Consumidores: `scripts/foo.sh`",
		"- `bar.go`",
	}
	fenced2 := make([]bool, len(lines2))
	patterns2, hasDecl2 := parseMLScopeFilesAffected(lines2, fenced2, 0, len(lines2))
	if !hasDecl2 {
		t.Fatal("contra-braço: expected hasDecl=true")
	}
	foundPypi := false
	foundBar := false
	for _, p := range patterns2 {
		if p == "pypi/pyproject.toml" {
			foundPypi = true
		}
		if p == "bar.go" {
			foundBar = true
		}
	}
	if !foundPypi || !foundBar {
		t.Errorf("contra-braço: '- **bold** `path`' list item must not stop list; got %v", patterns2)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T16 — Checkmark items stop the list (AC2)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedList_CheckmarkItemStops asserts that checkmark items
// ("- [ ]", "- [x]", "- [X]") stop the list so that acceptance-criteria items
// after a **Files affected:** block do not leak into scope.
func TestMLScopeFilesAffectedList_CheckmarkItemStops(t *testing.T) {
	for _, stopLine := range []string{"- [ ] criterion", "- [x] done", "- [X] Done"} {
		stopLine := stopLine
		t.Run(stopLine, func(t *testing.T) {
			lines := []string{
				"**Files affected:**",
				"- `a.go`",
				stopLine,
				"- `b.go`",
			}
			fenced := make([]bool, len(lines))
			patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
			if !hasDecl {
				t.Fatalf("expected hasDecl=true")
			}
			// b.go must NOT be present (past the checkmark stop).
			for _, p := range patterns {
				if p == "b.go" {
					t.Errorf("b.go must not be in patterns (list must stop at %q); got %v", stopLine, patterns)
				}
			}
			// a.go MUST be present (before the stop).
			found := false
			for _, p := range patterns {
				if p == "a.go" {
					found = true
				}
			}
			if !found {
				t.Errorf("a.go must be in patterns; got %v", patterns)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T16b — Any single-char criterion marker stops the list (ML-2B, Achado 3)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedList_AnyCriterionMarkerStops asserts that any line
// matching CriterionLineRe (^- \[.\]) stops the list, including variant markers
// such as "- [~]" and "- [-]" that were not covered by the previous three-form
// check. It also asserts that a link whose text is longer than one character
// ("- [texto longo](docs/x.md) `real.go`") does NOT stop the list, so that
// real.go is collected. A link with exactly one character in its text stops the
// list — this is the accepted cost of using CriterionLineRe (documented in
// barrier.go and cli-parity.md).
func TestMLScopeFilesAffectedList_AnyCriterionMarkerStops(t *testing.T) {
	// Sub-case 1: "- [~]" stops the list — leaked-tilde.go and b.go must not appear.
	t.Run("tilde_marker_stops", func(t *testing.T) {
		lines := []string{
			"**Files affected:**",
			"- `a.go`",
			"- [~] `leaked-tilde.go` variant criterion caducado",
			"- `b.go`",
		}
		fenced := make([]bool, len(lines))
		patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
		if !hasDecl {
			t.Fatalf("expected hasDecl=true")
		}
		for _, p := range patterns {
			if p == "leaked-tilde.go" || p == "b.go" {
				t.Errorf("token %q must not be in patterns (list must stop at - [~]); got %v", p, patterns)
			}
		}
		found := false
		for _, p := range patterns {
			if p == "a.go" {
				found = true
			}
		}
		if !found {
			t.Errorf("a.go must be in patterns (before the stop); got %v", patterns)
		}
	})

	// Sub-case 2: "- [-]" stops the list — leaked-dash.go and b.go must not appear.
	t.Run("dash_marker_stops", func(t *testing.T) {
		lines := []string{
			"**Files affected:**",
			"- `a.go`",
			"- [-] `leaked-dash.go` variant criterion",
			"- `b.go`",
		}
		fenced := make([]bool, len(lines))
		patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
		if !hasDecl {
			t.Fatalf("expected hasDecl=true")
		}
		for _, p := range patterns {
			if p == "leaked-dash.go" || p == "b.go" {
				t.Errorf("token %q must not be in patterns (list must stop at - [-]); got %v", p, patterns)
			}
		}
		found := false
		for _, p := range patterns {
			if p == "a.go" {
				found = true
			}
		}
		if !found {
			t.Errorf("a.go must be in patterns (before the stop); got %v", patterns)
		}
	})

	// Sub-case 3: a link with multi-char text does NOT stop the list — real.go must appear.
	t.Run("long_link_text_does_not_stop", func(t *testing.T) {
		lines := []string{
			"**Files affected:**",
			"- `a.go`",
			"- [texto longo](docs/x.md) `real.go`",
			"- `b.go`",
		}
		fenced := make([]bool, len(lines))
		patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
		if !hasDecl {
			t.Fatalf("expected hasDecl=true")
		}
		wantPresent := []string{"a.go", "real.go", "b.go"}
		for _, w := range wantPresent {
			found := false
			for _, p := range patterns {
				if p == w {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("want %q in patterns (multi-char link text must not stop list); got %v", w, patterns)
			}
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T17 — Hybrid inline + list (§1.4c)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeFilesAffectedHybridInlineAndList asserts that tokens on the label
// line AND tokens in list items below are all extracted (§1.4c hybrid form).
func TestMLScopeFilesAffectedHybridInlineAndList(t *testing.T) {
	lines := []string{
		"**Files affected:** `inline.go`",
		"- `list-a.go`",
		"- `list-b.go`",
	}
	fenced := make([]bool, len(lines))
	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatal("expected hasDecl=true")
	}
	want := []string{"inline.go", "list-a.go", "list-b.go"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want %q in patterns; got %v (hybrid inline+list must capture all tokens)", w, patterns)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T18 — Broad glob patterns are blocked (AC10)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeGlobBroadPatternBlocked asserts that tokens with no literal path
// segment (**, **\/*, *\/**)  cause the ml_scope check to return "blocked" with
// the rejection message from ADR D3.
func TestMLScopeGlobBroadPatternBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	vectors := []string{"**", "**/*", "*/**", "**/**", "*", "**/", "/"}

	// Build a single roadmap with all broad patterns in one Files affected line.
	var parts []string
	for _, v := range vectors {
		parts = append(parts, "`"+v+"`")
	}
	patternsLine := strings.Join(parts, ", ")

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	content := buildMLScopeRoadmap("2026-08-01", patternsLine)
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, content)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit a file so the diff is non-empty.
	if err := os.MkdirAll(filepath.Join(dir, "internal"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/foo.go"), []byte("package internal\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", "internal/foo.go")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add file")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked for broad patterns %v, got %q\nevidence: %v\nfailures: %v",
			vectors, check.Status, check.Evidence, check.Failures)
	}
	// Each broad-pattern token must produce a failure that names the token (quoted)
	// and the ML id (ML-1A), confirming the new message format includes context.
	for _, v := range vectors {
		found := false
		for _, f := range check.Failures {
			if strings.Contains(f, `"`+v+`"`) && strings.Contains(f, "ML-1A") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected failure containing token %q and ML-1A for broad pattern %q; got: %v",
				v, v, check.Failures)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T19 — Prefixed glob patterns are accepted (AC10 negative arm)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeGlobPrefixedPatternAccepted asserts that patterns with a literal
// prefix (site/**, **\/*.go) do NOT trigger the broad-pattern rejection.
func TestMLScopeGlobPrefixedPatternAccepted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
	content := buildMLScopeRoadmap("2026-08-01", "`site/**`, `**/*.go`")
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, content)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	if err := os.MkdirAll(filepath.Join(dir, "site/en"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "site/en/index.html"), []byte("<html/>\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal/commands"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/commands/foo.go"), []byte("package commands\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add matched files")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	for _, f := range check.Failures {
		// Use a suffix of the rejection message that survives the new format
		// ("<rdmPath> <mlID>: rejected: pattern \"<tok>\" without literal path segments …").
		if strings.Contains(f, "without literal path segments") {
			t.Fatalf("prefixed patterns site/** and **/*.go must not trigger broad rejection; failure: %q", f)
		}
		if strings.Contains(f, "site/en/index.html") {
			t.Fatalf("site/en/index.html must be allowed by site/**; failure: %q", f)
		}
		if strings.Contains(f, "internal/commands/foo.go") {
			t.Fatalf("internal/commands/foo.go must be allowed by **/*.go; failure: %q", f)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T20 — Union: file from member roadmap B is in scope for target A (AC4)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope asserts that when two
// roadmaps are in the diff, the barrier of roadmap A accepts files declared by
// roadmap B (union scope, ADR D2).
func TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmB = "ROADMAP-ml-scope-union-b.md"

	// Commit rdmA and REQ/ADR at origin/main; rdmB is added on the branch so it
	// appears in diffFiles and is detected as a union member.
	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB on the branch (after origin/main → in diffFiles → union member).
	contentB := buildMLScopeRoadmap("2026-08-01", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmB), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmB")

	// Commit a.go and b.go so both appear in the diff.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add a.go b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go is declared by union member rdmB — must NOT be in failures.
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			t.Fatalf("b.go is declared by union member %s — must not be in failures; got: %q\nall: %v",
				rdmB, f, check.Failures)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T21 — Union reads member from HEAD, not from disk
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionIgnoresWorkingTreeRoadmapEdit asserts that editing a union
// member's **Files affected:** on disk (without committing) does NOT expand the
// scope — the union logic reads the member from HEAD, not from disk.
func TestMLScope_UnionIgnoresWorkingTreeRoadmapEdit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmB = "ROADMAP-ml-scope-union-b.md"

	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB on the branch (after origin/main → in diffFiles → union member).
	contentB := buildMLScopeRoadmap("2026-08-01", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmB), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB: %v", err)
	}
	// Commit a.go and rdmB together so both are on the branch.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmB + a.go")

	// Expand rdmB on disk to grant extra.go — NOT committed.
	expandedB := buildMLScopeRoadmap("2026-08-01", "`b.go`, `extra.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmB), []byte(expandedB), 0644); err != nil {
		t.Fatalf("write expanded rdmB: %v", err)
	}
	// Commit extra.go (which the HEAD rdmB does NOT declare).
	if err := os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write extra.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", "extra.go")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add extra.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked (HEAD rdmB does not declare extra.go), got %q\nfailures: %v",
			check.Status, check.Failures)
	}
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "extra.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("extra.go must be in failures (disk edit of union member must not amplify scope); got: %v",
			check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T22 — AC5: Target roadmap excluded from union member loop (rewritten)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_TargetRoadmapExcludedFromMemberLoop asserts that the target roadmap
// is never treated as a union member (guard: `if diffPath == relPath { continue }`
// in Step 9.5). Fixture is constructed so that the target path appears in diffFiles
// (a roadmap edit is committed after origin/main), making the guard observable.
//
// Discriminator: Wave 2 🔄 of the target declares `**` (broad pattern). With the
// guard present, the broad-pattern rejection message appears ONCE (Step 9 direct
// path). Without the guard the target would also be processed as a member, causing
// the message to appear TWICE. The test asserts exactly one occurrence.
//
// Reconciliation: this test afirma que o target é excluído do member loop — com
// fixture que o coloca em diffFiles, a mensagem de rejeição de broad pattern aparece
// exatamente uma vez.
func TestMLScope_TargetRoadmapExcludedFromMemberLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Target roadmap: Wave 0 ✅, Wave 1 🔄 (wave1-file.go), Wave 2 🔄 (**).
	// Using ** in Wave 2 as the discriminator: with the guard, the rejection
	// message appears once; without it, twice (Step 9 + Step 9.5 member path).
	targetV1 := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: Target Excluded Test\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `wave1-file.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n" +
		"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
		"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `**`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	// targetV2: same content, Wave 2 status updated (minor edit so the path appears in diffFiles).
	targetV2 := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: Target Excluded Test\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `wave1-file.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n" +
		"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
		"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n<!-- wave 2 started -->\n" +
		"**Files affected:** `**`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-target-excl.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetV1)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit an edit to the target roadmap (targetV2) so its path appears in diffFiles.
	// This is the fix for the vacuous fixture (P-6): the target must be in diffFiles
	// for the guard to be observable.
	if err := os.WriteFile(filepath.Join(dir, rdmRel), []byte(targetV2), 0644); err != nil {
		t.Fatalf("write targetV2: %v", err)
	}
	// Commit wave1-file.go.
	if err := os.WriteFile(filepath.Join(dir, "wave1-file.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1-file.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "edit roadmap + add wave1-file.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-target-excl", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// The check must be blocked due to the broad-pattern rejection from Wave 2 ** .
	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked (** in Wave 2 rejected); got %q\nfailures: %v",
			check.Status, check.Failures)
	}
	// Count how many failure entries mention the broad-pattern rejection.
	// With the guard (target excluded from member loop): exactly 1.
	// Without the guard: the target would be processed again as a member → 2.
	const broadPatternMsg = "rejected: pattern"
	count := 0
	for _, f := range check.Failures {
		if strings.Contains(f, broadPatternMsg) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 broad-pattern rejection failure (target excluded from member loop); got %d\nfailures: %v",
			count, check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T23 — Union member with all-⬜ MLs contributes no patterns (K undefined)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionStartedWavesOnly_AllPendingContributesNothing asserts that a
// union member whose every ML is ⬜ Pendente (K undefined) contributes only
// orchestration paths and no pattern scope.
func TestMLScope_UnionStartedWavesOnly_AllPendingContributesNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmB = "ROADMAP-ml-scope-union-pending.md"

	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB (all-⬜) on the branch so it is in diffFiles and detected as a member.
	// K=-1 because no ML is started/done → no patterns contributed.
	contentB := buildMLScopeRoadmapStatus("2026-08-01", "⬜ Pendente", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmB), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmB+a.go+b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go is declared by all-pending member — must be in failures (K=-1 → no patterns).
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("b.go must be in failures (member rdmB has all-⬜ MLs → K=-1 → no patterns); got: %v",
			check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T24 — Union member with ✅ Wave 0 contributes Wave 0 patterns (K=0)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionStartedWavesOnly_DoneWaveContributesPatterns asserts that a
// member with Wave 0 ✅ and Wave 1 ⬜ sets K=0 and contributes only Wave 0 patterns.
func TestMLScope_UnionStartedWavesOnly_DoneWaveContributesPatterns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmB = "ROADMAP-ml-scope-union-k0.md"

	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB (Wave 0 ✅, Wave 1 ⬜ → K=0) on the branch so it is in diffFiles.
	var bb strings.Builder
	bb.WriteString("---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n")
	bb.WriteString("# Roadmap: K=0 Member\n\n## Acceptance Criteria\n- [x] ok\n\n")
	bb.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	bb.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	bb.WriteString("**Files affected:** `docs/seguranca/wave0-b.md`\n")
	bb.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	bb.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n")
	bb.WriteString("### ML-1A — Impl\n**Status:** ⬜ Pendente\n")
	bb.WriteString("**Files affected:** `b.go`\n")
	bb.WriteString("**Critérios de aceite:**\n- [ ] pending\n\n")
	contentB := bb.String()

	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmB), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs/seguranca"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs/seguranca/wave0-b.md"), []byte("# wave0-b\n"), 0644); err != nil {
		t.Fatalf("write wave0-b.md: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmB+a.go+b.go+wave0-b.md")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go is Wave 1 ⬜ in member B (K=0) — must be in failures.
	bInFailures := false
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			bInFailures = true
			break
		}
	}
	if !bInFailures {
		t.Fatalf("b.go must be in failures (member K=0 → Wave 1 patterns not contributed); got: %v",
			check.Failures)
	}
	// wave0-b.md is Wave 0 ✅ in member B (K=0) — must NOT be in failures.
	for _, f := range check.Failures {
		if strings.Contains(f, "wave0-b.md") {
			t.Fatalf("docs/seguranca/wave0-b.md must not be in failures (member Wave 0 ✅ → K=0 → Wave 0 patterns contributed); got: %q\nall: %v",
				f, check.Failures)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T25 — Union member in abandoned/ contributes only orchestration
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionAbandonedContributesOrchestrationOnly asserts that a union
// member in the abandoned/ state contributes only orchestration paths (its own
// roadmap path, REQ, ADR), not the patterns from its Files affected.
func TestMLScope_UnionAbandonedContributesOrchestrationOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmBAbandon = "ROADMAP-ml-scope-abandoned-b.md"

	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB (abandoned/) on the branch so it is in diffFiles and is detected as a
	// union member — but abandoned/ → orchestration only, no patterns.
	contentB := buildMLScopeRoadmap("2026-08-01", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/abandoned/"+rdmBAbandon), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmB-abandoned+a.go+b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go declared by abandoned member — must be in failures (abandoned → no patterns).
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("b.go must be in failures (member in abandoned/ → orchestration only, no patterns); got: %v",
			check.Failures)
	}
	// The abandoned roadmap path itself must NOT be in failures (it's in orchestration).
	for _, f := range check.Failures {
		if strings.Contains(f, rdmBAbandon) {
			t.Fatalf("abandoned roadmap %q must not be in failures (in orchestration set); got: %q\nall: %v",
				rdmBAbandon, f, check.Failures)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T26 — Uncommitted roadmap does not contribute to union (AC5)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionDoesNotIncludeUncommittedRoadmap asserts that a roadmap that
// exists only on disk (not in diffFiles) is not a union member, and its declared
// files remain out of scope.
func TestMLScope_UnionDoesNotIncludeUncommittedRoadmap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmBUncommitted = "ROADMAP-ml-scope-uncommitted.md"

	// Only rdmA is committed before origin/main.
	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Write rdmB on disk only — never git add.
	contentB := buildMLScopeRoadmap("2026-08-01", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmBUncommitted), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB (disk only): %v", err)
	}
	// Commit a.go; leave b.go untracked (statusFiles but not diffFiles).
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", "a.go")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add a.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go is declared only by uncommitted rdmB — must be in failures.
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("b.go must be in failures (rdmB not in diff → not a member); got: %v",
			check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T27 — Member in done/ at merge-base → orchestration only
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionDoneAtMergeBaseOrchestrationOnly asserts that a roadmap that
// was already in done/ at merge-base (origin/main) contributes only orchestration
// when it is modified on the branch, not its pattern scope.
func TestMLScope_UnionDoneAtMergeBaseOrchestrationOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmBDone = "ROADMAP-ml-scope-done-at-mb.md"

	// Commit rdmA (wip/) and rdmB (done/) at origin/main.
	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	contentB := buildMLScopeRoadmap("2026-08-01", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/done/"+rdmBDone), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB done: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA+rdmBDone")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// On branch: touch rdmB (keep in done/) and commit b.go.
	touchedB := contentB + "\n<!-- touched -->\n"
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/done/"+rdmBDone), []byte(touchedB), 0644); err != nil {
		t.Fatalf("write touched rdmB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "touch rdmB + add b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go declared by rdmB which was in done/ at merge-base → orchestration only → b.go in failures.
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("b.go must be in failures (rdmB was in done/ at merge-base → orchestration only); got: %v",
			check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T28 — Member moved to done/ on branch contributes patterns
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionDoneMovedInBranchContributesPatterns asserts that a roadmap
// moved from wip/ to done/ on the branch (i.e., done/ file was NOT at merge-base)
// contributes its patterns to the union scope.
func TestMLScope_UnionDoneMovedInBranchContributesPatterns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmBBase = "ROADMAP-ml-scope-moved-to-done.md"

	// At origin/main: rdmA in wip/, rdmB in wip/ (declares b.go).
	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	contentB := buildMLScopeRoadmap("2026-08-01", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmBBase), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB wip: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA+rdmBwip")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// On branch: move rdmB from wip/ to done/.
	mlScopeGitRun(t, dir, env, "rm", "-q", "docs/roadmaps/wip/"+rdmBBase)
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/done/"+rdmBBase), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB done: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "move rdmB to done + add a.go b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go declared by rdmB (moved to done/ on branch, NOT at merge-base) — must NOT be in failures.
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			t.Fatalf("b.go must not be in failures (rdmB moved to done/ on branch → contributes patterns); got: %q\nall: %v",
				f, check.Failures)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T29 — req: outside req_dir discarded (AC11)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_ReqOutsideReqDirDiscarded asserts that:
//   - req: "go.mod", req: "docs/req-x/f.md", req: "docs/req/../../go.mod" are
//     silently discarded (path traversal / outside req_dir);
//   - req: "./docs/req/REQ-x.md" IS accepted (normalises to "docs/req/REQ-x.md").
func TestMLScope_ReqOutsideReqDirDiscarded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Case A: req: "go.mod" — must not grant go.mod as orchestration.
	t.Run("req_outside_req_dir", func(t *testing.T) {
		content := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"go.mod\"\n---\n\n" +
			"# Roadmap: Fixture\n\n## Acceptance Criteria\n- [x] ok\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A\n**Status:** ✅ Concluído\n**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A\n**Status:** ✅ Concluído\n**Files affected:** `a.go`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n"

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, content)
		env := mlScopeGitEnv(dir)
		// Write a real go.mod so it would be reachable if wrongly granted.
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+go.mod")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		// Now commit go.mod change and a.go on the branch.
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0644); err != nil {
			t.Fatalf("update go.mod: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write a.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "bump go.mod + add a.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		// go.mod must be in failures: req: "go.mod" is outside req_dir → discarded → not orchestration.
		found := false
		for _, f := range check.Failures {
			if strings.Contains(f, "go.mod") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("go.mod must be in failures (req: outside req_dir → discarded); got: %v",
				check.Failures)
		}
	})

	// Case B: req: "./docs/req/REQ-x.md" — "./" prefix normalises, must be accepted.
	t.Run("req_with_dotslash_accepted", func(t *testing.T) {
		content := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"./docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: Fixture\n\n## Acceptance Criteria\n- [x] ok\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A\n**Status:** ✅ Concluído\n**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A\n**Status:** ✅ Concluído\n**Files affected:** `a.go`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n"

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, content)
		writeMLScopeREQFixture(t, dir, rdmRel)
		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write a.go: %v", err)
		}
		// Also commit the REQ file (to put it in the diff so its scope is exercised).
		reqContent := "---\nstatus: Open\ndate: 2026-07-20\nadr: \"\"\nroadmap: \"" + rdmRel + "\"\n---\n\n# REQ\n\n## Acceptance Criteria\n- [ ] ok\n"
		if err := os.WriteFile(filepath.Join(dir, "docs/req/REQ-ml-scope-fixture.md"), []byte(reqContent), 0644); err != nil {
			t.Fatalf("update req: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add a.go + update req")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		// REQ file must NOT be in failures (./docs/req/REQ-x.md normalises to docs/req/REQ-x.md → accepted).
		for _, f := range check.Failures {
			if strings.Contains(f, "REQ-ml-scope-fixture.md") {
				t.Fatalf("REQ file must not be in failures (req: './docs/req/...' normalises and is accepted); got: %q\nall: %v",
					f, check.Failures)
			}
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T30 — adr: outside adr_dirs discarded (AC11)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_AdrOutsideADRDirsDiscarded asserts that an ADR path outside
// adr_dirs (e.g., "go.mod") is discarded; a valid path under adr_dirs is accepted.
func TestMLScope_AdrOutsideADRDirsDiscarded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// The REQ will reference adr: "go.mod".
	makeContent := func(adrPath string) string {
		return "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: Fixture\n\n## Acceptance Criteria\n- [x] ok\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A\n**Status:** ✅ Concluído\n**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A\n**Status:** ✅ Concluído\n**Files affected:** `a.go`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"_adrPath placeholder: " + adrPath + "_\n"
	}

	// Case A: adr: "go.mod" — must not grant go.mod.
	t.Run("adr_outside_discarded", func(t *testing.T) {
		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, makeContent("go.mod"))
		env := mlScopeGitEnv(dir)

		// Write REQ with adr: "go.mod".
		adr := "---\nstatus: Accepted\ndate: 2026-07-20\n---\n\n# ADR: Fixture\n"
		if err := os.WriteFile(filepath.Join(dir, "docs/adr/ADR-ml-scope-fixture.md"), []byte(adr), 0644); err != nil {
			t.Fatalf("write adr: %v", err)
		}
		req := fmt.Sprintf("---\nstatus: Open\ndate: 2026-07-20\nadr: \"go.mod\"\nroadmap: \"%s\"\n---\n\n# REQ: Fixture\n\n## Acceptance Criteria\n- [ ] ok\n", rdmRel)
		if err := os.WriteFile(filepath.Join(dir, "docs/req/REQ-ml-scope-fixture.md"), []byte(req), 0644); err != nil {
			t.Fatalf("write req: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+go.mod")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		// Commit a.go and go.mod change on branch.
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0644); err != nil {
			t.Fatalf("update go.mod: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write a.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "bump go.mod + add a.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		// go.mod must be in failures: adr: "go.mod" outside adr_dirs → discarded.
		found := false
		for _, f := range check.Failures {
			if strings.Contains(f, "go.mod") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("go.mod must be in failures (adr: outside adr_dirs → discarded); got: %v", check.Failures)
		}
	})

	// Case B: adr: "docs/adr/ADR-ml-scope-fixture.md" with adr_dirs: [docs/adr] — must be accepted.
	t.Run("adr_inside_accepted", func(t *testing.T) {
		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"
		// Use mlScopeSetupMinimalGitRepo which sets adr_dirs: [docs/adr].
		dir = mlScopeSetupMinimalGitRepo(t, dir)
		env := mlScopeGitEnv(dir)
		if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-ml-scope-fixture.md"),
			[]byte(makeContent("docs/adr/ADR-ml-scope-fixture.md")), 0644); err != nil {
			t.Fatalf("write roadmap: %v", err)
		}
		writeMLScopeREQFixture(t, dir, rdmRel)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+roadmap+req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write a.go: %v", err)
		}
		// Modify the ADR to put it in the diff.
		adrContent := "---\nstatus: Accepted\ndate: 2026-07-20\n---\n\n# ADR: ML Scope Fixture\n\n> Status: Accepted\n<!-- updated -->\n"
		if err := os.WriteFile(filepath.Join(dir, "docs/adr/ADR-ml-scope-fixture.md"), []byte(adrContent), 0644); err != nil {
			t.Fatalf("update adr: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add a.go + update adr")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-fixture", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		// ADR file must NOT be in failures (docs/adr/ADR-ml-scope-fixture.md ∈ adr_dirs → accepted).
		for _, f := range check.Failures {
			if strings.Contains(f, "ADR-ml-scope-fixture.md") {
				t.Fatalf("ADR file must not be in failures (adr: inside adr_dirs → accepted); got: %q\nall: %v",
					f, check.Failures)
			}
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T31 — Union member with pre-cutoff date contributes only orchestration
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionPreCutoffMemberOrchestrationOnly asserts that a union member
// whose roadmap date is before 2026-07-18 contributes only orchestration paths,
// not its pattern scope.
func TestMLScope_UnionPreCutoffMemberOrchestrationOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmBPre = "ROADMAP-ml-scope-precutouff.md"

	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB (pre-cutoff date) on the branch so it is in diffFiles and detected as a
	// union member — but pre-cutoff → orchestration only, no patterns.
	contentB := buildMLScopeRoadmap("2026-05-10", "`b.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmBPre), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB precutouff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmBPre+a.go+b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go declared by pre-cutoff member — must be in failures (orchestration only, no patterns).
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("b.go must be in failures (member rdmBPre has pre-cutoff date → orchestration only); got: %v",
			check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T32 — Union member with no date: treated as post-cutoff (contributes patterns)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionMissingDateTreatedAsPostCutoff asserts that a union member
// whose roadmap frontmatter has no date: field is treated as post-cutoff and
// contributes its pattern scope normally.
func TestMLScope_UnionMissingDateTreatedAsPostCutoff(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-union-a.md"
	const rdmBNoDate = "ROADMAP-ml-scope-nodate.md"

	contentA := buildMLScopeRoadmap("2026-08-01", "`a.go`")
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add rdmB (no date: field → treated as post-cutoff) on the branch so it is in
	// diffFiles and detected as a union member — contributes patterns normally.
	contentBNoDate := "---\nstatus: wip\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: No Date Fixture\n\n## Acceptance Criteria\n- [x] ok\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — The ML\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `b.go`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n"

	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmBNoDate), []byte(contentBNoDate), 0644); err != nil {
		t.Fatalf("write rdmB nodate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmBNoDate+a.go+b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// b.go declared by no-date member → treated as post-cutoff → contributes patterns → must NOT be in failures.
	for _, f := range check.Failures {
		if strings.Contains(f, "b.go") {
			t.Fatalf("b.go must not be in failures (member with no date: → post-cutoff → contributes patterns); got: %q\nall: %v",
				f, check.Failures)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T26 — AC1: started wave above N contributes files to scope
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_StartedWaveAboveN_PassesWhenFilesCommitted asserts that files
// declared in a started wave above N (wave 2 🔄 when N=1) are in scope and
// do not appear in failures.
//
// Reconciliation: AC1 afirma que arquivos de waves iniciadas acima de N estão
// em escopo e não reprovam o barrier da wave N.
func TestMLScope_StartedWaveAboveN_PassesWhenFilesCommitted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Roadmap: Wave 0 ✅, Wave 1 ✅ (wave1-file.go), Wave 2 🔄 (wave2-file.go).
	targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: AC1 Test\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `wave1-file.go`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
		"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `wave2-file.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac1.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave1-file.go and wave2-file.go (both in scope: wave1 ≤ N, wave2 started > N).
	if err := os.WriteFile(filepath.Join(dir, "wave1-file.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1-file.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wave2-file.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave2-file.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1-file.go wave2-file.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac1", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status != "passed" {
		t.Fatalf("expected ml_scope passed (wave2-file.go is in scope via started wave 2 > N=1); got %q\nfailures: %v",
			check.Status, check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T27 — AC2: undeclared file still fails with started wave above N
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_StartedWaveAboveN_UndeclaredFileStillFails asserts that a file
// not declared in any wave still appears in failures even when there is a started
// wave above N — the started wave does not unlock undeclared files.
//
// Reconciliation: AC2 afirma que arquivos não declarados em nenhuma wave reprovam
// mesmo quando há wave iniciada acima de N, e só esse arquivo aparece nas falhas.
func TestMLScope_StartedWaveAboveN_UndeclaredFileStillFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Same roadmap as AC1: Wave 0 ✅, Wave 1 ✅ (wave1-file.go), Wave 2 🔄 (wave2-file.go).
	targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: AC2 Test\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `wave1-file.go`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
		"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `wave2-file.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac2.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave1-file.go, wave2-file.go (both in scope) and extra.go (not declared).
	if err := os.WriteFile(filepath.Join(dir, "wave1-file.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1-file.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wave2-file.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave2-file.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write extra.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add files")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac2", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked (extra.go not declared); got %q\nfailures: %v",
			check.Status, check.Failures)
	}
	// extra.go must appear in failures, and must be the only out-of-scope entry.
	extraFound := false
	for _, f := range check.Failures {
		if strings.Contains(f, "extra.go") {
			extraFound = true
		}
		// wave1-file.go and wave2-file.go must NOT appear in failures.
		if strings.Contains(f, "wave1-file.go") {
			t.Fatalf("wave1-file.go must not be in failures; got: %q\nall: %v", f, check.Failures)
		}
		if strings.Contains(f, "wave2-file.go") {
			t.Fatalf("wave2-file.go must not be in failures (started wave 2 > N=1 contributes it); got: %q\nall: %v", f, check.Failures)
		}
	}
	if !extraFound {
		t.Fatalf("extra.go must be in failures (not declared in any wave); got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T28 — AC3: unstarted wave ⬜ above N is not in scope even with started wave above it
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnstartedWaveAmidStartedWavesAboveN_Fails asserts two vectors:
//   - simple: Wave 2 ⬜ alone above N=1 → x.go blocks
//   - gap:    Wave 2 ⬜ and Wave 3 🔄 above N=1 → x.go blocks, y.go passes
//
// Reconciliation: AC3 afirma que arquivo de wave ⬜ entre N e wave iniciada
// reprova, e que a wave iniciada acima dela (wave 3 🔄) não inclui a ⬜ abaixo.
func TestMLScope_UnstartedWaveAmidStartedWavesAboveN_Fails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	t.Run("simple", func(t *testing.T) {
		// Wave 0 ✅, Wave 1 ✅, Wave 2 ⬜ (x.go). No wave above 2.
		targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: AC3 Simple\n\n## Acceptance Criteria\n- [x] test\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A — Impl\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `wave1-file.go`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
			"### ML-2A — Red team\n**Status:** ⬜ Pendente\n" +
			"**Files affected:** `x.go`\n" +
			"**Critérios de aceite:**\n- [ ] pending\n\n"

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac3-simple.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
		writeMLScopeREQFixture(t, dir, rdmRel)

		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write x.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add x.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac3-simple", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status != "blocked" {
			t.Fatalf("expected blocked (x.go in unstarted wave 2 ⬜ > N=1); got %q\nfailures: %v",
				check.Status, check.Failures)
		}
		found := false
		for _, f := range check.Failures {
			if strings.Contains(f, "x.go") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("x.go must be in failures (wave 2 ⬜ not started); got: %v", check.Failures)
		}
	})

	t.Run("gap", func(t *testing.T) {
		// Wave 0 ✅, Wave 1 ✅, Wave 2 ⬜ (x.go), Wave 3 🔄 (y.go).
		// x.go should block (wave 2 ⬜ not started); y.go should pass (wave 3 🔄 started).
		targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: AC3 Gap\n\n## Acceptance Criteria\n- [x] test\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A — Impl\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `wave1-file.go`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
			"### ML-2A — Red team\n**Status:** ⬜ Pendente\n" +
			"**Files affected:** `x.go`\n" +
			"**Critérios de aceite:**\n- [ ] pending\n\n" +
			"## Wave 3 — Post review\n> Dependências: Wave 2.\n\n" +
			"### ML-3A — Post review\n**Status:** 🔄 Em andamento\n" +
			"**Files affected:** `y.go`\n" +
			"**Critérios de aceite:**\n- [ ] pending\n\n"

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac3-gap.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
		writeMLScopeREQFixture(t, dir, rdmRel)

		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write x.go: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "y.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write y.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add x.go y.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac3-gap", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status != "blocked" {
			t.Fatalf("expected blocked (x.go in unstarted wave 2 ⬜ between started waves); got %q\nfailures: %v",
				check.Status, check.Failures)
		}
		// x.go must be in failures (wave 2 ⬜ not started, even though wave 3 🔄 is started).
		xFound := false
		for _, f := range check.Failures {
			if strings.Contains(f, "x.go") {
				xFound = true
			}
			// y.go must NOT be in failures (wave 3 🔄 is started → in scope).
			if strings.Contains(f, "y.go") {
				t.Fatalf("y.go must not be in failures (declared in started wave 3 🔄 > N=1); got: %q\nall: %v",
					f, check.Failures)
			}
		}
		if !xFound {
			t.Fatalf("x.go must be in failures (wave 2 ⬜ not started); got: %v", check.Failures)
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T29 — AC3b: union member unstarted wave ⬜ is excluded
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_UnionMemberUnstartedWaveIsExcluded asserts that a wave ⬜ in a
// union member is not included in the member's scope contribution, even when
// a started wave above it exists (opção B: only started waves contribute).
//
// Reconciliation: AC3b afirma que arquivo de wave ⬜ de um membro da união não
// entra no escopo, mesmo quando há wave iniciada acima dela no mesmo membro.
func TestMLScope_UnionMemberUnstartedWaveIsExcluded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	dir := t.TempDir()
	dir = mlScopeSetupMinimalGitRepo(t, dir)
	env := mlScopeGitEnv(dir)

	const rdmA = "ROADMAP-ml-scope-ac3b-target.md"
	const rdmB = "ROADMAP-ml-scope-ac3b-member.md"

	// Target A: Wave 0 ✅, Wave 1 🔄 (a.go).
	contentA := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: AC3b Target\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0-a.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `a.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	// Member B: Wave 0 ✅, Wave 1 ✅, Wave 2 ⬜ (x.go), Wave 3 🔄 (y.go).
	contentB := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: AC3b Member\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0-b.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `b1.go`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
		"### ML-2A — Red team\n**Status:** ⬜ Pendente\n" +
		"**Files affected:** `x.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n" +
		"## Wave 3 — Post review\n> Dependências: Wave 2.\n\n" +
		"### ML-3A — Post review\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `y.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmA), []byte(contentA), 0644); err != nil {
		t.Fatalf("write rdmA: %v", err)
	}
	writeMLScopeREQFixture(t, dir, "docs/roadmaps/wip/"+rdmA)
	if err := os.MkdirAll(filepath.Join(dir, "docs/seguranca"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "base+rdmA")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Add member B and implementation files after origin/main.
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/"+rdmB), []byte(contentB), 0644); err != nil {
		t.Fatalf("write rdmB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write x.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "y.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write y.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add rdmB+a.go+x.go+y.go")

	stdout, _ := mlScopeRunBarrier(t, dir, rdmA, "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// a.go (target wave 1 🔄 ≤ N=1) → in scope → not in failures.
	// y.go (member wave 3 🔄 → started) → in scope → not in failures.
	// x.go (member wave 2 ⬜ → not started) → out of scope → must be in failures.
	xFound := false
	for _, f := range check.Failures {
		if strings.Contains(f, "x.go") {
			xFound = true
		}
		if strings.Contains(f, "a.go") {
			t.Fatalf("a.go must not be in failures (target wave 1 ≤ N=1); got: %q\nall: %v", f, check.Failures)
		}
		if strings.Contains(f, "y.go") {
			t.Fatalf("y.go must not be in failures (member wave 3 🔄 started); got: %q\nall: %v", f, check.Failures)
		}
	}
	if !xFound {
		t.Fatalf("x.go must be in failures (member wave 2 ⬜ not started); got: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T30 — AC4/P-2: no declFailure for missing declaration in started wave above N
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_StartedWaveAboveN_NoDeclFailureForMissingDeclaration asserts two arms:
//   - Arm A (positive): ML in started wave > N without **Files affected:** does NOT
//     produce a declFailure — the barrier of wave N passes.
//   - Arm B (control/contra-braço): ML in wave ≤ N without **Files affected:** still
//     produces a declFailure and blocks.
//
// Reconciliation (Arm A): afirma que declFailures não são emitidos para MLs de waves
// > N mesmo quando a wave tem K match-positive.
// Reconciliation (Arm B): afirma que MLs em waves ≤ N sem declaração ainda geram
// declFailure.
func TestMLScope_StartedWaveAboveN_NoDeclFailureForMissingDeclaration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	t.Run("ArmA_no_decl_failure_for_wave_above_N", func(t *testing.T) {
		// Wave 0 ✅, Wave 1 ✅ (wave1-file.go), Wave 2 🔄 WITHOUT **Files affected:**.
		// --wave 1 should pass: ML-2A missing declaration does not block wave 1.
		targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: AC4 Arm A\n\n## Acceptance Criteria\n- [x] test\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A — Impl\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `wave1-file.go`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
			"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n" +
			// No **Files affected:** line for ML-2A.
			"**Critérios de aceite:**\n- [ ] pending\n\n"

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac4a.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
		writeMLScopeREQFixture(t, dir, rdmRel)

		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		if err := os.WriteFile(filepath.Join(dir, "wave1-file.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write wave1-file.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1-file.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac4a", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		if check.Status != "passed" {
			t.Fatalf("expected ml_scope passed (ML-2A missing declaration in wave 2 🔄 > N=1 must not block wave 1); got %q\nfailures: %v",
				check.Status, check.Failures)
		}
		// Confirm ML-2A does not appear in any failure.
		for _, f := range check.Failures {
			if strings.Contains(f, "ML-2A") {
				t.Fatalf("ML-2A must not appear in failures (wave 2 > N=1 → no declFailure); got: %q\nall: %v",
					f, check.Failures)
			}
		}
	})

	t.Run("ArmB_decl_failure_for_wave_le_N", func(t *testing.T) {
		// Wave 0 ✅, Wave 1 🔄 WITHOUT **Files affected:**, Wave 2 🔄 (wave2-file.go).
		// --wave 1 should block: ML-1A missing declaration in wave 1 ≤ N generates declFailure.
		targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: AC4 Arm B\n\n## Acceptance Criteria\n- [x] test\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A — Impl\n**Status:** 🔄 Em andamento\n" +
			// No **Files affected:** line for ML-1A.
			"**Critérios de aceite:**\n- [ ] pending\n\n" +
			"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
			"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n" +
			"**Files affected:** `wave2-file.go`\n" +
			"**Critérios de aceite:**\n- [ ] pending\n\n"

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac4b.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
		writeMLScopeREQFixture(t, dir, rdmRel)

		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		if err := os.WriteFile(filepath.Join(dir, "wave2-file.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write wave2-file.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave2-file.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac4b", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		if check.Status != "blocked" {
			t.Fatalf("expected ml_scope blocked (ML-1A missing declaration in wave 1 ≤ N=1 must generate declFailure); got %q\nfailures: %v",
				check.Status, check.Failures)
		}
		// ML-1A must appear in failures with the declFailure message.
		ml1aFound := false
		for _, f := range check.Failures {
			if strings.Contains(f, "ML-1A") {
				ml1aFound = true
				break
			}
		}
		if !ml1aFound {
			t.Fatalf("ML-1A must appear in failures (wave 1 ≤ N=1 → declFailure); got: %v", check.Failures)
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T31 — P-3: broad pattern in started wave above N is rejected
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_BroadPatternInStartedWaveAboveN_Blocked asserts that a broad pattern
// (e.g. **) in a started wave above N is still rejected by addPatterns and blocks
// the barrier.
//
// Reconciliation: P-3 afirma que broad patterns em waves > N iniciadas ainda são
// rejeitados pelo mecanismo addPatterns, da mesma forma que em waves ≤ N. Este
// teste também falha sob o recorte 0..N pois wave 2 nunca seria processada.
func TestMLScope_BroadPatternInStartedWaveAboveN_Blocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Wave 0 ✅, Wave 1 🔄 (wave1-file.go), Wave 2 🔄 (**).
	targetContent := "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
		"# Roadmap: P3 Test\n\n## Acceptance Criteria\n- [x] test\n\n" +
		"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
		"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
		"**Files affected:** `docs/seguranca/wave0.md`\n" +
		"**Critérios de aceite:**\n- [x] done\n\n" +
		"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
		"### ML-1A — Impl\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `wave1-file.go`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n" +
		"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
		"### ML-2A — Red team\n**Status:** 🔄 Em andamento\n" +
		"**Files affected:** `**`\n" +
		"**Critérios de aceite:**\n- [ ] pending\n\n"

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-p3.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, targetContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	if err := os.WriteFile(filepath.Join(dir, "wave1-file.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1-file.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1-file.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-p3", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status != "blocked" {
		t.Fatalf("expected ml_scope blocked (** in started wave 2 > N=1 must be rejected); got %q\nfailures: %v",
			check.Status, check.Failures)
	}
	// Must contain the broad-pattern rejection message from wave 2 (ML-2A).
	rejectFound := false
	for _, f := range check.Failures {
		if strings.Contains(f, "rejected: pattern") {
			rejectFound = true
			break
		}
	}
	if !rejectFound {
		t.Fatalf("broad-pattern rejection message must be in failures (** in started wave 2 > N=1); got: %v",
			check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T32 — P-4: disk edit of wave status does not amplify scope
// ────────────────────────────────────────────────────────────────────────────

// TestMLScope_DiskEditOfWaveStatusDoesNotAmplifyScope asserts that editing the
// roadmap on disk to mark a future wave as 🔄 (without committing) does NOT
// include that wave's files in scope — the check reads wave statuses from HEAD.
//
// The test includes a control arm where the wave is committed as 🔄 in HEAD,
// confirming that HEAD status is what governs scope, not the disk state.
//
// Reconciliation: P-4 afirma que uma edição de disco que eleva K não altera o
// escopo da wave N; o braço de controle afirma que quando o HEAD tem a wave
// como 🔄 commitada, o arquivo da wave entra em escopo.
func TestMLScope_DiskEditOfWaveStatusDoesNotAmplifyScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// buildRoadmapP4 builds a roadmap where Wave 2 has the given status in HEAD.
	// wave2Status: "⬜ Pendente" (not started) or "🔄 Em andamento" (started).
	buildRoadmapP4 := func(wave2Status string) string {
		return "---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n" +
			"# Roadmap: P4 Test\n\n## Acceptance Criteria\n- [x] test\n\n" +
			"## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n" +
			"### ML-0A — Threat model\n**Status:** ✅ Concluído\n" +
			"**Files affected:** `docs/seguranca/wave0.md`\n" +
			"**Critérios de aceite:**\n- [x] done\n\n" +
			"## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n" +
			"### ML-1A — Impl\n**Status:** 🔄 Em andamento\n" +
			"**Files affected:** `w1.go`\n" +
			"**Critérios de aceite:**\n- [ ] pending\n\n" +
			"## Wave 2 — Red team\n> Dependências: Wave 1.\n\n" +
			"### ML-2A — Red team\n**Status:** " + wave2Status + "\n" +
			"**Files affected:** `z.go`\n" +
			"**Critérios de aceite:**\n- [ ] pending\n\n"
	}

	t.Run("disk_edit_does_not_amplify", func(t *testing.T) {
		// HEAD has Wave 2 ⬜. Disk edit marks Wave 2 as 🔄. z.go is committed.
		// --wave 1 must block on z.go (HEAD says wave 2 is ⬜ → not started → not in scope).
		headContent := buildRoadmapP4("⬜ Pendente")
		diskContent := buildRoadmapP4("🔄 Em andamento")

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-p4-edit.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, headContent)
		writeMLScopeREQFixture(t, dir, rdmRel)

		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		// Edit the roadmap on disk to mark Wave 2 as 🔄 (NOT committed).
		if err := os.WriteFile(filepath.Join(dir, rdmRel), []byte(diskContent), 0644); err != nil {
			t.Fatalf("write disk content: %v", err)
		}
		// Commit z.go (declared in wave 2, which is ⬜ in HEAD).
		if err := os.WriteFile(filepath.Join(dir, "z.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write z.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", "z.go")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add z.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-p4-edit", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		if check.Status != "blocked" {
			t.Fatalf("expected blocked (z.go in wave 2 ⬜ in HEAD, disk edit to 🔄 must not amplify scope); got %q\nfailures: %v",
				check.Status, check.Failures)
		}
		found := false
		for _, f := range check.Failures {
			if strings.Contains(f, "z.go") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("z.go must be in failures (disk edit of wave status does not amplify scope); got: %v", check.Failures)
		}
	})

	t.Run("committed_wave_amplifies", func(t *testing.T) {
		// Control arm: HEAD has Wave 2 🔄 (committed). z.go is committed.
		// --wave 1 must pass (HEAD says wave 2 is 🔄 → started → z.go in scope).
		headContent := buildRoadmapP4("🔄 Em andamento")

		dir := t.TempDir()
		const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-p4-ctrl.md"
		dir = mlScopeSetupGitRepo(t, dir, rdmRel, headContent)
		writeMLScopeREQFixture(t, dir, rdmRel)

		env := mlScopeGitEnv(dir)
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
		mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

		// Commit z.go (declared in wave 2 🔄 in HEAD → in scope).
		if err := os.WriteFile(filepath.Join(dir, "z.go"), []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write z.go: %v", err)
		}
		mlScopeGitRun(t, dir, env, "add", ".")
		mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add z.go")

		stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-p4-ctrl", "1")
		check := findMLScopeCheck(t, stdout)

		if check.Status == "skipped" {
			t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
		}
		if check.Status != "passed" {
			t.Fatalf("expected passed (z.go in wave 2 🔄 committed in HEAD → in scope); got %q\nfailures: %v",
				check.Status, check.Failures)
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T33 — VS16 variant: wave with 🔄️ (U+FE0F) counts as started (in-process)
// ────────────────────────────────────────────────────────────────────────────

// TestMLScopeWaveIsStarted_VS16Variant asserts that mlScopeWaveIsStarted returns
// true for a wave whose only ML has status "🔄️ Em andamento" (with VS16 U+FE0F
// appended to the emoji), confirming that VS16 normalisation in mlScopeWaveIsStarted
// handles the variant form identically to the plain form.
//
// Reconciliation: afirma que o predicado mlScopeWaveIsStarted normaliza variantes
// VS16 de 🔄, retornando true para uma wave com único ML marcado 🔄️.
func TestMLScopeWaveIsStarted_VS16Variant(t *testing.T) {
	// "🔄️ Em andamento" — 🔄 followed by VS16 (U+FE0F).
	lines := []string{
		"## Wave 1 — Implementation",
		"### ML-1A — The ML",
		"**Status:** 🔄️ Em andamento",
		"**Files affected:** `foo.go`",
		"**Critérios de aceite:**",
		"- [ ] pending",
	}
	fenced := make([]bool, len(lines))
	wave := waveBlock{Label: "1", Start: 0, End: len(lines)}
	if !mlScopeWaveIsStarted(lines, fenced, wave) {
		t.Fatalf("mlScopeWaveIsStarted must return true for 🔄️ (VS16 variant of 🔄 Em andamento)")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T34 — AC8: fractional wave labels ordered by CompareWaveLabels, not integer
// ────────────────────────────────────────────────────────────────────────────

// buildFractionalRoadmap returns a roadmap with Wave 0 ✅, Wave 1 ✅ (wave1.go
// declared), and Wave 1b in the given status with side.go declared.
// If wave1bStatus is "" the Wave 1b ML has no **Files affected:** line.
func buildFractionalRoadmap(wave1bStatus, wave1bFilesAffected string) string {
	var b strings.Builder
	b.WriteString("---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n")
	b.WriteString("# Roadmap: AC8 Fractional Wave Test\n\n## Acceptance Criteria\n- [x] test\n\n")
	// Wave 0
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	// Wave 1
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n")
	b.WriteString("### ML-1A — Impl\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `wave1.go`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	// Wave 1b
	fmt.Fprintf(&b, "## Wave 1b — Corrective\n> Dependências: Wave 1.\n\n")
	fmt.Fprintf(&b, "### ML-1b-A — Corrective\n**Status:** %s\n", wave1bStatus)
	if wave1bFilesAffected != "" {
		fmt.Fprintf(&b, "**Files affected:** %s\n", wave1bFilesAffected)
	}
	b.WriteString("**Critérios de aceite:**\n- [ ] pending\n\n")
	return b.String()
}

// TestMLScope_FractionalWave_UnstartedOutOfScope asserts that a file declared in
// wave "1b" ⬜ is OUT of scope for --wave 1, because CompareWaveLabels("1b","1") > 0
// classifies "1b" as above N and it is not started.
//
// Contra-braço: with the integer comparison (waveInt > currentWaveInt ↔ 1 > 1 = false),
// wave "1b" is never filtered, side.go would be silently in scope, and ml_scope would
// pass — a false-negative. With CompareWaveLabels the test reproves that path.
//
// Reconciliation: AC8 afirma que a wave fracionária ⬜ "1b" está acima de N=1 por
// CompareWaveLabels e não entra no escopo; com comparação inteira ela entraria.
func TestMLScope_FractionalWave_UnstartedOutOfScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	rdmContent := buildFractionalRoadmap("⬜ Pendente", "`side.go`")

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac8a.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit side.go — declared only in wave "1b" ⬜, which must be excluded for --wave 1.
	if err := os.WriteFile(filepath.Join(dir, "side.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write side.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add side.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac8a", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status == "passed" {
		t.Fatalf("expected ml_scope blocked (side.go in wave 1b ⬜ must be out of scope for --wave 1); got passed\nevidence: %v", check.Evidence)
	}
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "side.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("side.go must appear in ml_scope failures; got: %v", check.Failures)
	}
}

// TestMLScope_FractionalWave_UnstartedNoDeclFailure asserts that a ML in wave "1b" ⬜
// without a **Files affected:** declaration does NOT produce a declFailure for --wave 1,
// because CompareWaveLabels("1b","1") > 0 classifies "1b" as above N, and declFailures
// are only emitted for waves ≤ N.
//
// Contra-braço: with the integer comparison (waveInt <= currentWaveInt ↔ 1 <= 1 = true),
// wave "1b" would be classified as ≤ N and a declFailure would fire for ML-1b-A.
//
// Reconciliation: AC8 afirma que ML sem declaração em wave fracionária ⬜ acima de N
// não gera declFailure; com comparação inteira, geraria.
func TestMLScope_FractionalWave_UnstartedNoDeclFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Wave 1b ⬜ with NO **Files affected:** — would trigger declFailure under old code.
	rdmContent := buildFractionalRoadmap("⬜ Pendente", "" /* no Files affected */)

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac8b.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave1.go (declared in wave 1 ✅ — definitely in scope).
	if err := os.WriteFile(filepath.Join(dir, "wave1.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac8b", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// ml_scope must NOT have a declFailure for ML-1b-A.
	for _, f := range check.Failures {
		if strings.Contains(f, "ML-1b-A") && strings.Contains(f, "Files affected") {
			t.Fatalf("unexpected declFailure for ML-1b-A in wave 1b ⬜ (above N=1); got: %v", check.Failures)
		}
	}
}

// TestMLScope_FractionalWave_StartedInScope asserts that a file declared in wave "1b" 🔄
// IS in scope for --wave 1, because CompareWaveLabels("1b","1") > 0 but the wave IS
// started — the started-wave path contributes its patterns to scope.
//
// Reconciliation: AC8 afirma que wave fracionária 🔄 acima de N=1 está no escopo
// (CompareWaveLabels > 0 AND started); side.go deve ser aceito sem falha.
func TestMLScope_FractionalWave_StartedInScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	rdmContent := buildFractionalRoadmap("🔄 Em andamento", "`side.go`")

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac8c.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit both wave1.go and side.go — both should be in scope.
	if err := os.WriteFile(filepath.Join(dir, "wave1.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "side.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write side.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1.go side.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac8c", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status != "passed" {
		t.Fatalf("expected ml_scope passed (side.go in wave 1b 🔄 is started above N=1 → in scope); got %q\nfailures: %v",
			check.Status, check.Failures)
	}
}

// TestMLScope_FractionalWave_LowerWaveHasDeclFailure asserts that --wave 1b treats
// wave "1" as ≤ N (CompareWaveLabels("1","1b") < 0), so a ML in wave "1" without
// **Files affected:** produces a declFailure.
//
// Reconciliation: AC8 afirma que --wave 1b classifica wave "1" como ≤ N e exige
// declaração; um ML sem declaração nessa wave gera declFailure.
func TestMLScope_FractionalWave_LowerWaveHasDeclFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Wave 1 has ML-1A WITHOUT **Files affected:** (will be ≤ N for --wave 1b).
	// Wave 1b 🔄 has **Files affected:** wave1b.go.
	var b strings.Builder
	b.WriteString("---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n")
	b.WriteString("# Roadmap: AC8 Reverse Direction Test\n\n## Acceptance Criteria\n- [x] test\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	// Wave 1 — ≤ N for --wave 1b, ML without declaration
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n")
	b.WriteString("### ML-1A — Impl\n**Status:** ✅ Concluído\n")
	// NO **Files affected:** line — should trigger declFailure under --wave 1b
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	// Wave 1b — the target wave
	b.WriteString("## Wave 1b — Corrective\n> Dependências: Wave 1.\n\n")
	b.WriteString("### ML-1b-A — Corrective\n**Status:** 🔄 Em andamento\n")
	b.WriteString("**Files affected:** `wave1b.go`\n")
	b.WriteString("**Critérios de aceite:**\n- [ ] pending\n\n")
	rdmContent := b.String()

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac8d.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave1b.go (in scope via wave 1b 🔄).
	if err := os.WriteFile(filepath.Join(dir, "wave1b.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1b.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1b.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac8d", "1b")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	// ml_scope must have a declFailure for ML-1A (wave "1" ≤ N for --wave 1b).
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "ML-1A") && strings.Contains(f, "Files affected") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected declFailure for ML-1A (wave 1 is ≤ N for --wave 1b); got failures: %v", check.Failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T35 — AC10: ❌ Bloqueado and ❌ Cancelado / 🚫 Abandonado above N
// ────────────────────────────────────────────────────────────────────────────

// buildHTMLCommentStatusRoadmap returns a roadmap with Wave 0 ✅, Wave 1 ✅ (wave1.go),
// Wave 2 where the ML's **Status:** line appears only inside an HTML comment block
// (so the live status is missing → ⬜ fail-safe), and wave2.go is declared.
// For the mls_complete test the live status of wave 1 ML can be suppressed via the
// wave1Status parameter.
func buildHTMLCommentStatusRoadmap(wave2Status, wave1Status string) string {
	var b strings.Builder
	b.WriteString("---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n")
	b.WriteString("# Roadmap: AC9 HTML Comment Status Test\n\n## Acceptance Criteria\n- [x] test\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n")
	// Wave 1: ML-1A with configurable live status (or all status inside comment).
	if wave1Status == "comment" {
		b.WriteString("### ML-1A — Impl\n<!--\n**Status:** ✅ Concluído\n-->\n")
	} else {
		fmt.Fprintf(&b, "### ML-1A — Impl\n**Status:** %s\n", wave1Status)
	}
	b.WriteString("**Files affected:** `wave1.go`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	// Wave 2: **Status:** inside HTML comment only; real status is missing (⬜ fail-safe).
	b.WriteString("## Wave 2 — Red team\n> Dependências: Wave 1.\n\n")
	fmt.Fprintf(&b, "### ML-2A — Check\n<!--\n**Status:** %s\n-->\n", wave2Status)
	b.WriteString("**Files affected:** `wave2.go`\n")
	b.WriteString("**Critérios de aceite:**\n- [ ] pending\n\n")
	return b.String()
}

// buildBlockedStatusRoadmap returns a roadmap with Wave 0 ✅, Wave 1 ✅ (wave1.go),
// Wave 2 with only one ML in the given status and wave2.go declared.
func buildBlockedStatusRoadmap(wave2Status string) string {
	var b strings.Builder
	b.WriteString("---\nstatus: wip\ndate: 2026-08-01\nreq: \"docs/req/REQ-ml-scope-fixture.md\"\n---\n\n")
	b.WriteString("# Roadmap: AC10 Status Test\n\n## Acceptance Criteria\n- [x] test\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `docs/seguranca/wave0.md`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	b.WriteString("## Wave 1 — Implementation\n> Dependências: Wave 0.\n\n")
	b.WriteString("### ML-1A — Impl\n**Status:** ✅ Concluído\n")
	b.WriteString("**Files affected:** `wave1.go`\n")
	b.WriteString("**Critérios de aceite:**\n- [x] done\n\n")
	fmt.Fprintf(&b, "## Wave 2 — Red team\n> Dependências: Wave 1.\n\n")
	fmt.Fprintf(&b, "### ML-2A — Check\n**Status:** %s\n", wave2Status)
	b.WriteString("**Files affected:** `wave2.go`\n")
	b.WriteString("**Critérios de aceite:**\n- [ ] pending\n\n")
	return b.String()
}

// TestMLScope_BlockedStatusAboveN_InScope asserts that a wave above N=1 containing
// only a ML with status "❌ Bloqueado" is treated as started (first token ❌ in a
// StatusPending ML), and its declared file is in scope for --wave 1.
//
// Kills M8b: removing ❌ from the token set in mlScopeWaveIsStarted makes the wave
// unstarted, so wave2.go would be out of scope and this test fails.
//
// Reconciliation: AC10 afirma que ❌ Bloqueado acima de N inicia a wave; wave2.go
// deve estar em escopo (matar M8b).
func TestMLScope_BlockedStatusAboveN_InScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	rdmContent := buildBlockedStatusRoadmap("❌ Bloqueado")

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac10a.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave1.go (in scope via wave 1 ✅) and wave2.go (in scope via wave 2 ❌ Bloqueado).
	if err := os.WriteFile(filepath.Join(dir, "wave1.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wave2.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave2.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1.go wave2.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac10a", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status != "passed" {
		t.Fatalf("expected ml_scope passed (wave2.go in wave 2 ❌ Bloqueado → started → in scope); got %q\nfailures: %v",
			check.Status, check.Failures)
	}
}

// TestMLScope_TerminatedStatusAboveN_InScope asserts that a wave above N=1 containing
// only a ML with terminated status (❌ Cancelado or 🚫 Abandonado) is treated as
// started (StatusTerminated), and its declared file is in scope for --wave 1.
//
// Kills M8c: removing the StatusTerminated branch in mlScopeWaveIsStarted makes the
// wave unstarted, so wave2.go would be out of scope and this test fails.
//
// Reconciliation: AC10 afirma que ❌ Cancelado e 🚫 Abandonado acima de N iniciam a
// wave; wave2.go deve estar em escopo (matar M8c).
func TestMLScope_TerminatedStatusAboveN_InScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	for _, status := range []string{"❌ Cancelado", "🚫 Abandonado"} {
		status := status
		t.Run(status, func(t *testing.T) {
			rdmContent := buildBlockedStatusRoadmap(status)

			dir := t.TempDir()
			rdmRel := "docs/roadmaps/wip/ROADMAP-ml-scope-ac10b-" + strings.ReplaceAll(status, " ", "-") + ".md"
			dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
			writeMLScopeREQFixture(t, dir, rdmRel)

			env := mlScopeGitEnv(dir)
			mlScopeGitRun(t, dir, env, "add", ".")
			mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
			mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

			// Commit wave1.go and wave2.go — both should be in scope.
			if err := os.WriteFile(filepath.Join(dir, "wave1.go"), []byte("package main\n"), 0644); err != nil {
				t.Fatalf("write wave1.go: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "wave2.go"), []byte("package main\n"), 0644); err != nil {
				t.Fatalf("write wave2.go: %v", err)
			}
			mlScopeGitRun(t, dir, env, "add", ".")
			mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1.go wave2.go")

			// Use the basename of the roadmap file (without .md) as the argument.
			rdmBasename := strings.TrimSuffix(filepath.Base(rdmRel), ".md")
			stdout, _ := mlScopeRunBarrier(t, dir, rdmBasename, "1")
			check := findMLScopeCheck(t, stdout)

			if check.Status == "skipped" {
				t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
			}
			if check.Status != "passed" {
				t.Fatalf("expected ml_scope passed (wave2.go in wave 2 %q → started → in scope); got %q\nfailures: %v",
					status, check.Status, check.Failures)
			}
		})
	}
}

// TestMLScope_HTMLCommentedStatus_WaveNotStarted asserts that a wave above N=1 whose
// only **Status:** line is inside an HTML comment block is treated as not started
// (live status = missing → fail-safe ⬜), so its declared file is NOT in scope for
// --wave 1.
//
// Contra-braço: without htmlCommentMask in MLStatusMarker the commented status
// would be visible as 🔄, the wave would be considered started, and wave2.go would
// be in scope — a scope-widening false-negative. With the mask this test reproves
// that path.
//
// Reconciliation: AC9 afirma que **Status:** dentro de comentário HTML não inicia
// a wave; wave2.go deve estar fora de escopo.
func TestMLScope_HTMLCommentedStatus_WaveNotStarted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Wave 2 has "**Status:** 🔄 Em andamento" only inside <!-- -->; live status is missing.
	rdmContent := buildHTMLCommentStatusRoadmap("🔄 Em andamento", "✅ Concluído")

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac9a.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave2.go — declared only in wave 2 whose status is inside a comment.
	if err := os.WriteFile(filepath.Join(dir, "wave2.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave2.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave2.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac9a", "1")
	check := findMLScopeCheck(t, stdout)

	if check.Status == "skipped" {
		t.Fatalf("expected ml_scope to run; evidence: %v", check.Evidence)
	}
	if check.Status == "passed" {
		t.Fatalf("expected ml_scope blocked (wave2.go in wave 2 whose status is inside HTML comment → wave not started → out of scope); got passed\nevidence: %v", check.Evidence)
	}
	found := false
	for _, f := range check.Failures {
		if strings.Contains(f, "wave2.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("wave2.go must appear in ml_scope failures (out of scope); got: %v", check.Failures)
	}
}

// TestMLScope_HTMLCommentedStatus_MLSCompleteBlocked asserts that mls_complete is
// blocked when an ML's only ✅ marker is inside an HTML comment block — the live
// status is missing (fail-safe ⬜) so the check fails.
//
// Contra-braço: without htmlCommentMask in MLStatusMarker the commented ✅ would
// satisfy mls_complete and the check would pass — a false-pass.
//
// Reconciliation: AC9 afirma que ✅ dentro de comentário HTML não satisfaz
// mls_complete; o check deve ficar blocked.
func TestMLScope_HTMLCommentedStatus_MLSCompleteBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ml_scope uses git commands; windows path handling tested separately")
	}

	// Wave 1: ML-1A has its ✅ only inside a comment; live status is missing.
	rdmContent := buildHTMLCommentStatusRoadmap("🔄 Em andamento", "comment")

	dir := t.TempDir()
	const rdmRel = "docs/roadmaps/wip/ROADMAP-ml-scope-ac9b.md"
	dir = mlScopeSetupGitRepo(t, dir, rdmRel, rdmContent)
	writeMLScopeREQFixture(t, dir, rdmRel)

	env := mlScopeGitEnv(dir)
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add req+adr")
	mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Commit wave1.go — it's in scope but mls_complete must fail because ML-1A has no live ✅.
	if err := os.WriteFile(filepath.Join(dir, "wave1.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write wave1.go: %v", err)
	}
	mlScopeGitRun(t, dir, env, "add", ".")
	mlScopeGitRun(t, dir, env, "commit", "-q", "-m", "add wave1.go")

	stdout, _ := mlScopeRunBarrier(t, dir, "ROADMAP-ml-scope-ac9b", "1")

	// Parse all checks to find mls_complete.
	var result struct {
		Checks []barrierCheck `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal barrier output: %v\nstdout: %s", err, stdout)
	}
	var mlsCheck *barrierCheck
	for i := range result.Checks {
		if result.Checks[i].Name == "mls_complete" {
			mlsCheck = &result.Checks[i]
			break
		}
	}
	if mlsCheck == nil {
		t.Fatalf("mls_complete check not found in barrier output; checks: %v", result.Checks)
	}
	if mlsCheck.Status == "passed" {
		t.Fatalf("expected mls_complete blocked (ML-1A has ✅ only inside HTML comment → missing live status); got passed\nevidence: %v", mlsCheck.Evidence)
	}
}

// ── ML-3C tests — AC11 (parseMLScopeFilesAffected, HTML comment masking) ────

// TestMLScopeFilesAffected_LabelInComment_NotDeclaration asserts that a
// **Files affected:** label line whose column 0 is inside an HTML comment
// (<!-- … -->) is silently skipped, and a real declaration following the
// comment is found and its tokens returned (AC11, ML-3C, REQ-2026-10-31).
//
// Contra-braço: without HTMLCommentMask in parseMLScopeFilesAffected the
// commented label would be treated as the real one; evil.go would be added
// to scope and this test would fail (hasDecl true, patterns contains evil.go).
//
// Reconciliation (Regra Dura): the function skips label lines inside <!-- -->
// and continues scanning; the real label outside the comment is returned.
func TestMLScopeFilesAffected_LabelInComment_NotDeclaration(t *testing.T) {
	// Document:
	//  line 0: ## Wave 1 — Test
	//  line 1: ### ML-1A — scope
	//  line 2: <!--
	//  line 3: **Files affected:** `evil.go`    ← inside comment, must be skipped
	//  line 4: -->
	//  line 5: **Files affected:** `real.go`    ← real label, outside comment
	doc := "## Wave 1 — Test\n" +
		"### ML-1A — scope\n" +
		"<!--\n" +
		"**Files affected:** `evil.go`\n" +
		"-->\n" +
		"**Files affected:** `real.go`\n"

	lines := splitRoadmapLines(doc)
	fenced := fenceMask(lines)

	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatalf("expected hasDecl=true (real label at line 6 must be found)")
	}
	if len(patterns) != 1 || patterns[0] != "real.go" {
		t.Errorf("expected patterns=[real.go], got %v (evil.go must not appear)", patterns)
	}
}

// TestMLScopeFilesAffected_ContinuationInComment_Truncated asserts that a list
// item continuation line inside an HTML comment block ends the declaration
// (treated like a code-fence line — "como cerca"), so the commented token is
// NOT added to scope (AC11, ML-3C, REQ-2026-10-31).
//
// Contra-braço: without the htmlMask[j] break in the continuation loop the
// commented item would be scanned past the closing --> line, and evil.go would
// appear in patterns. The test fails because patterns does not contain evil.go.
//
// Reconciliation (Regra Dura): a continuation line inside <!-- --> stops the
// declaration collection; only tokens before the comment appear in scope.
func TestMLScopeFilesAffected_ContinuationInComment_Truncated(t *testing.T) {
	// Document:
	//  line 0: **Files affected:** `real.go`    ← real label
	//  line 1: <!--                              ← opens comment
	//  line 2: - `evil.go`                       ← inside comment, must NOT be a token
	//  line 3: -->
	doc := "**Files affected:** `real.go`\n" +
		"<!--\n" +
		"- `evil.go`\n" +
		"-->\n"

	lines := splitRoadmapLines(doc)
	fenced := fenceMask(lines)

	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatalf("expected hasDecl=true (real label at line 1)")
	}
	for _, p := range patterns {
		if p == "evil.go" {
			t.Errorf("evil.go must NOT be in scope (it is inside HTML comment); patterns: %v", patterns)
		}
	}
	found := false
	for _, p := range patterns {
		if p == "real.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("real.go must be in scope; patterns: %v", patterns)
	}
}
// ── ML-3C ação 3 tests — AC11b (parseMLScopeFilesAffected, inline HTML comment stripping) ─────

// TestMLScopeFilesAffected_InlineComment_HidesToken asserts that a backtick-quoted
// token inside an inline HTML comment (<!-- `token` -->) on the label line is NOT
// added to scope, while a token outside the comment IS returned (AC11b, ML-3C ação 3).
//
// Contra-braço: without StripInlineHTMLComments applied to the label line,
// extractTokens sees the raw line and returns both real.go and evil.go; the test
// fails because evil.go appears in patterns.
//
// Reconciliation (Regra Dura): inline complete <!-- --> on the label line removes the
// backtick-quoted token inside it; only tokens outside comments are extracted.
func TestMLScopeFilesAffected_InlineComment_HidesToken(t *testing.T) {
	// Label: **Files affected:** `real.go` <!-- `evil.go` -->
	// After strip: **Files affected:** `real.go`  (comment replaced with space)
	lines := []string{
		"**Files affected:** `real.go` <!-- `evil.go` -->",
	}
	fenced := make([]bool, len(lines))

	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatal("expected hasDecl=true")
	}
	for _, p := range patterns {
		if p == "evil.go" {
			t.Errorf("evil.go must NOT be in scope (inside inline comment); patterns: %v", patterns)
		}
	}
	found := false
	for _, p := range patterns {
		if p == "real.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("real.go must be in scope (outside comment); patterns: %v", patterns)
	}
}

// TestMLScopeFilesAffected_InlineComment_OpenNoClose asserts that an unclosed <!--
// on the label line discards tokens that follow it on that line; a continuation line
// whose column 0 is inside the resulting multi-line comment is blocked by the
// existing htmlCommentMask ("o comportamento de máscara multi-linha que você já fez
// continua", AC11b, ML-3C ação 3).
//
// Contra-braço: without StripInlineHTMLComments, extractTokens returns evil.go from
// the raw label line (between <!-- and EOL); the test fails because evil.go appears.
//
// Reconciliation (Regra Dura): unclosed <!-- truncates the label line; tokens after
// it are invisible; continuation lines inside the multi-line comment are blocked by
// htmlCommentMask as before.
func TestMLScopeFilesAffected_InlineComment_OpenNoClose(t *testing.T) {
	// Document:
	//  line 0: **Files affected:** `real.go` <!-- `evil.go`   ← unclosed comment
	//  line 1: - `evil-cont.go`                               ← inside multi-line comment (htmlMask)
	//  line 2: -->
	doc := "**Files affected:** `real.go` <!-- `evil.go`\n" +
		"- `evil-cont.go`\n" +
		"-->\n"

	lines := splitRoadmapLines(doc)
	fenced := fenceMask(lines)

	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatalf("expected hasDecl=true")
	}
	found := false
	for _, p := range patterns {
		if p == "real.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("real.go must be in scope (before <!--); patterns: %v", patterns)
	}
	for _, p := range patterns {
		if p == "evil.go" || p == "evil-cont.go" {
			t.Errorf("%q must NOT be in scope; patterns: %v", p, patterns)
		}
	}
}

// TestMLScopeFilesAffected_InlineComment_BetweenTokens asserts that when a label
// line contains a closed inline comment followed by an unclosed one, only the token
// between the two comments is returned, and the endsOpen=true from the unclosed
// comment prevents any continuation lines from being collected (AC11b, ML-3C ação 3).
//
// Contra-braço: without StripInlineHTMLComments applied to the label, evil-a.go and
// evil-b.go inside the comment spans would appear in patterns. Without endsOpen
// breaking the continuation loop, other.go from the continuation would also appear.
//
// Reconciliation (Regra Dura): multiple inline comments on the label line are all
// stripped left-to-right; only tokens outside every span are extracted; an unclosed
// "<!--" stops continuation collection.
func TestMLScopeFilesAffected_InlineComment_BetweenTokens(t *testing.T) {
	// Document:
	//  line 0: **Files affected:** <!-- `evil-a.go` --> `x.go` <!-- `evil-b.go`  ← closed then unclosed
	//  line 1: `other.go`                                                          ← continuation; endsOpen stops loop before reaching it
	doc := "**Files affected:** <!-- `evil-a.go` --> `x.go` <!-- `evil-b.go`\n" +
		"`other.go`\n"

	lines := splitRoadmapLines(doc)
	fenced := fenceMask(lines)

	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatalf("expected hasDecl=true")
	}
	if len(patterns) != 1 || patterns[0] != "x.go" {
		t.Errorf("expected patterns=[x.go], got %v (evil-a.go, evil-b.go, other.go must not appear)", patterns)
	}
}

// TestMLScopeFilesAffected_InlineComment_ContinuationLine asserts that an inline
// HTML comment on a list-item continuation line hides the token inside it, while a
// token outside the comment on the same line is still returned (AC11b, ML-3C ação 3).
//
// Contra-braço: without StripInlineHTMLComments applied to continuation lines,
// extractTokens returns evil.go from the raw continuation; the test fails.
//
// Reconciliation (Regra Dura): inline stripping is applied to both the label line
// and continuation lines; tokens inside comments are invisible regardless of which
// line they appear on.
func TestMLScopeFilesAffected_InlineComment_ContinuationLine(t *testing.T) {
	// Document:
	//  line 0: **Files affected:** `real.go`          ← label (no comment)
	//  line 1: - `ok.go` <!-- `evil.go` -->            ← list item with inline comment
	lines := []string{
		"**Files affected:** `real.go`",
		"- `ok.go` <!-- `evil.go` -->",
	}
	fenced := make([]bool, len(lines))

	patterns, hasDecl := parseMLScopeFilesAffected(lines, fenced, 0, len(lines))
	if !hasDecl {
		t.Fatal("expected hasDecl=true")
	}
	for _, p := range patterns {
		if p == "evil.go" {
			t.Errorf("evil.go must NOT be in scope (inside inline comment on continuation); patterns: %v", patterns)
		}
	}
	wantIn := []string{"real.go", "ok.go"}
	for _, w := range wantIn {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q must be in scope; patterns: %v", w, patterns)
		}
	}
}
