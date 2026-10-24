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
// | TestMLScope_NoOriginMain_Blocked       | Absent origin/main → status "blocked" (Decision 1, ML-4C)        |

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
			"[core]\n\thooksPath = /dev/null\n\tautocrlf = false\n"), 0600)
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
