package commands

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/forge"
)

// ────────────────────────────────────────────────────────────────────────────
// test helpers
// ────────────────────────────────────────────────────────────────────────────

// mockGit captures every call to execGit and returns configured responses.
type mockGit struct {
	branch      string   // returned for symbolic-ref --short HEAD
	stagedFiles string   // returned for diff --cached --name-only (empty = nothing staged)
	calls       [][]string
}

func (m *mockGit) exec(args ...string) (string, error) {
	call := make([]string, len(args))
	copy(call, args)
	m.calls = append(m.calls, call)

	joined := strings.Join(args, " ")

	switch {
	case strings.HasPrefix(joined, "symbolic-ref --short"):
		if m.branch == "" {
			return "", errors.New("not a git repo")
		}
		return m.branch, nil

	case strings.HasPrefix(joined, "diff --cached --name-only"):
		return m.stagedFiles, nil

	case strings.HasPrefix(joined, "rev-parse --abbrev-ref --symbolic-full-name @{u}"):
		// Simulate no upstream → push -u
		return "", errors.New("no upstream")

	case strings.HasPrefix(joined, "fetch"):
		// Simulate offline — non-blocking
		return "", errors.New("could not connect")
	}

	return "", nil
}

func makeDeps(branch, staged string, violations []string) (shipDeps, *mockGit) {
	m := &mockGit{branch: branch, stagedFiles: staged}
	d := shipDeps{
		execGit:         m.exec,
		checkGovernance: func() []string { return violations },
		out:             &bytes.Buffer{},
		// Step 7 safe defaults: CLI never invoked, no filesystem access.
		configForge:  "",
		repoDir:      "", // empty → no CI file detection
		availFn:      func(string) bool { return false },
		execForgeCLI: func(string, []string) error { return nil },
	}
	return d, m
}

// ────────────────────────────────────────────────────────────────────────────
// Step 1 — Branch validation
// ────────────────────────────────────────────────────────────────────────────

func TestShip_MainBranch_Aborts(t *testing.T) {
	d, _ := makeDeps("main", "file.go", nil)
	err := runShip(shipOpts{message: "feat: x"}, d)
	if err == nil {
		t.Fatal("expected error for main branch")
	}
	if !strings.Contains(err.Error(), "cannot run on") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShip_MasterBranch_Aborts(t *testing.T) {
	d, _ := makeDeps("master", "file.go", nil)
	err := runShip(shipOpts{message: "feat: x"}, d)
	if err == nil {
		t.Fatal("expected error for master branch")
	}
	if !strings.Contains(err.Error(), "cannot run on") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShip_WrongPattern_Aborts(t *testing.T) {
	cases := []string{"feature/foo", "hotfix/bar", "docs/update", "mybranch"}
	for _, branch := range cases {
		d, _ := makeDeps(branch, "file.go", nil)
		err := runShip(shipOpts{message: "feat: x"}, d)
		if err == nil {
			t.Fatalf("expected error for branch %q", branch)
		}
		if !strings.Contains(err.Error(), "does not match the required pattern") {
			t.Fatalf("branch %q: unexpected error: %v", branch, err)
		}
	}
}

func TestShip_ValidBranchPatterns_NotRejectedByStep1(t *testing.T) {
	validBranches := []string{"feat/my-feature", "fix/bug-123", "refactor/clean-up"}
	for _, branch := range validBranches {
		d, _ := makeDeps(branch, "file.go", nil)
		err := runShip(shipOpts{message: "feat(scope): desc"}, d)
		// May fail after step 1 (commit/push mock), but must NOT fail at branch validation.
		if err != nil && strings.Contains(err.Error(), "does not match the required pattern") {
			t.Fatalf("branch %q should be valid but was rejected: %v", branch, err)
		}
		if err != nil && strings.Contains(err.Error(), "cannot run on") {
			t.Fatalf("branch %q should not trigger main/master check: %v", branch, err)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 2 — Governance
// ────────────────────────────────────────────────────────────────────────────

func TestShip_NoWIPRoadmap_Aborts(t *testing.T) {
	violations := []string{`branch "feat/foo" is a feat/fix/refactor branch but no roadmap is in wip/`}
	d, _ := makeDeps("feat/foo", "file.go", violations)
	err := runShip(shipOpts{message: "feat: x"}, d)
	if err == nil {
		t.Fatal("expected governance error")
	}
	if !strings.Contains(err.Error(), "governance check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	outStr := d.out.(*bytes.Buffer).String()
	for _, cmd := range []string{"trackfw req new", "trackfw roadmap new", "trackfw roadmap move"} {
		if !strings.Contains(outStr, cmd) {
			t.Fatalf("output must mention remediation command %q", cmd)
		}
	}
	if !strings.Contains(outStr, "lenient") {
		t.Fatalf("output must mention lenient mode so users understand why validate passes but ship aborts")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 4 — Nothing staged
// ────────────────────────────────────────────────────────────────────────────

func TestShip_NothingStaged_Aborts(t *testing.T) {
	d, _ := makeDeps("feat/my-feature", "" /* nothing staged */, nil)
	err := runShip(shipOpts{message: "feat: x"}, d)
	if err == nil {
		t.Fatal("expected error when nothing is staged")
	}
	if !strings.Contains(err.Error(), "nothing is staged") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 5 — Missing commit message
// ────────────────────────────────────────────────────────────────────────────

func TestShip_NoMessage_Aborts(t *testing.T) {
	d, _ := makeDeps("feat/my-feature", "file.go", nil)
	err := runShip(shipOpts{message: "" /* no -m */}, d)
	if err == nil {
		t.Fatal("expected error when -m is absent")
	}
	if !strings.Contains(err.Error(), "commit message is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// --dry-run: no write commands forwarded to execGit
// ────────────────────────────────────────────────────────────────────────────

func TestShip_DryRun_NoWriteCommandsExecuted(t *testing.T) {
	m := &mockGit{branch: "feat/my-feature", stagedFiles: "file.go"}
	d := shipDeps{
		execGit:         m.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		availFn:         func(string) bool { return false },
		execForgeCLI:    func(string, []string) error { return nil },
	}

	err := runShip(shipOpts{message: "feat(scope): dry run test", dryRun: true}, d)
	if err != nil {
		t.Fatalf("dry-run should not fail: %v", err)
	}

	// execGit must not have been called with any write command
	for _, call := range m.calls {
		if len(call) == 0 {
			continue
		}
		if gitWriteCommands[call[0]] {
			t.Fatalf("dry-run must not execute write command via execGit: git %s", strings.Join(call, " "))
		}
	}

	// Output must contain [dry-run] markers
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run]") {
		t.Fatal("dry-run output must contain '[dry-run]' markers")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Source-level guarantee: git add . / git add -A must not appear in ship.go
// ────────────────────────────────────────────────────────────────────────────

func TestShip_SourceHasNoGitAddAll(t *testing.T) {
	// Locate ship.go relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("runtime.Caller unavailable")
	}
	shipFile := filepath.Join(filepath.Dir(thisFile), "ship.go")
	src, err := os.ReadFile(shipFile)
	if err != nil {
		t.Skipf("could not read ship.go: %v", err)
	}
	content := string(src)

	// Patterns that would indicate actual git add calls (not user-facing doc strings).
	// We check for the two-argument form used in Go slice/function calls: "add", "."
	// Single-quoted occurrences like 'git add .' in error messages are not matched here.
	forbidden := []string{`"add", "."`, `"add", "-A"`}
	for _, bad := range forbidden {
		if strings.Contains(content, bad) {
			t.Fatalf("ship.go contains forbidden pattern %q — git add . / git add -A must never appear", bad)
		}
	}
}

// TestShip_ExecNeverReceivesGitAddAll verifies at runtime that execGit is never
// called with "add ." or "add -A" arguments, even transitively.
func TestShip_ExecNeverReceivesGitAddAll(t *testing.T) {
	m := &mockGit{branch: "feat/safe-check", stagedFiles: "internal/x.go"}
	d := shipDeps{
		execGit:         m.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		availFn:         func(string) bool { return false },
		execForgeCLI:    func(string, []string) error { return nil },
	}

	_ = runShip(shipOpts{message: "feat: safe check", dryRun: true}, d)

	for _, call := range m.calls {
		if len(call) < 2 {
			continue
		}
		if call[0] == "add" && (call[1] == "." || call[1] == "-A") {
			t.Fatalf("execGit received forbidden call: git %s", strings.Join(call, " "))
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// isShipBranch unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestIsShipBranch(t *testing.T) {
	valid := []string{"feat/foo", "feat/a-very-long-slug", "fix/123", "refactor/clean-up"}
	for _, b := range valid {
		if !isShipBranch(b) {
			t.Errorf("isShipBranch(%q) should be true", b)
		}
	}

	invalid := []string{"main", "master", "feature/foo", "hotfix/bar", "feat/", "refactor/"}
	for _, b := range invalid {
		if isShipBranch(b) {
			t.Errorf("isShipBranch(%q) should be false", b)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 7 — forge resolution and PR/MR opening
// ────────────────────────────────────────────────────────────────────────────

// mockForgeCLI captures execForgeCLI calls.
type mockForgeCLI struct {
	calls []struct {
		name string
		args []string
	}
	err error
}

func (m *mockForgeCLI) exec(name string, args []string) error {
	m.calls = append(m.calls, struct {
		name string
		args []string
	}{name, args})
	return m.err
}

// makeStep7Deps returns deps ready to reach Step 7 (valid branch, staged file, no violations).
func makeStep7Deps(configForge string, forgeFlag string, availFn func(string) bool) (shipOpts, shipDeps, *mockGit, *mockForgeCLI) {
	g := &mockGit{branch: "feat/my-feature", stagedFiles: "file.go"}
	cli := &mockForgeCLI{}
	if availFn == nil {
		availFn = func(string) bool { return false }
	}
	d := shipDeps{
		execGit:         g.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		configForge:     configForge,
		repoDir:         "",
		availFn:         availFn,
		execForgeCLI:    cli.exec,
	}
	opts := shipOpts{message: "feat(x): test step7", forge: forgeFlag}
	return opts, d, g, cli
}

func TestShip_GitLab_SaysMergeRequest(t *testing.T) {
	opts, d, _, _ := makeStep7Deps("gitlab", "", func(string) bool { return false })
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Merge Request") {
		t.Fatalf("expected 'Merge Request' in output for gitlab, got:\n%s", out)
	}
}

func TestShip_GitHub_SaysPullRequest(t *testing.T) {
	opts, d, _, _ := makeStep7Deps("github", "", func(string) bool { return false })
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Pull Request") {
		t.Fatalf("expected 'Pull Request' in output for github, got:\n%s", out)
	}
}

func TestShip_CLIUnavailable_Exit0WithURL(t *testing.T) {
	// GitHub with CLI unavailable — should print URL and return nil.
	opts, d, _, cli := makeStep7Deps("github", "", func(string) bool { return false })
	d.repoDir = ""
	// Override execGit to return a real-looking remote URL so FallbackURL works.
	g := &mockGit{branch: "feat/my-feature", stagedFiles: "file.go"}
	origExec := g.exec
	d.execGit = func(args ...string) (string, error) {
		if len(args) == 3 && args[0] == "remote" && args[1] == "get-url" {
			return "https://github.com/org/repo.git", nil
		}
		return origExec(args...)
	}
	if err := runShip(opts, d); err != nil {
		t.Fatalf("should not return error when CLI unavailable: %v", err)
	}
	if len(cli.calls) > 0 {
		t.Fatalf("execForgeCLI must not be called when CLI is unavailable")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "github.com") {
		t.Fatalf("expected fallback URL in output, got:\n%s", out)
	}
}

func TestShip_ManualForge_Exit0(t *testing.T) {
	// No forge flag, no config, no remoteURL → manual.
	opts, d, _, cli := makeStep7Deps("", "", nil)
	if err := runShip(opts, d); err != nil {
		t.Fatalf("manual forge should not cause error: %v", err)
	}
	if len(cli.calls) > 0 {
		t.Fatalf("execForgeCLI must not be called for manual forge")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "ship complete") {
		t.Fatalf("expected 'ship complete' in manual output, got:\n%s", out)
	}
}

func TestShip_NoPR_SkipsStep7(t *testing.T) {
	opts, d, _, cli := makeStep7Deps("github", "", func(string) bool { return true })
	opts.noPR = true
	if err := runShip(opts, d); err != nil {
		t.Fatalf("--no-pr should not cause error: %v", err)
	}
	if len(cli.calls) > 0 {
		t.Fatalf("execForgeCLI must not be called with --no-pr")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "--no-pr") {
		t.Fatalf("expected '--no-pr' message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "ship complete") {
		t.Fatalf("expected 'ship complete' in output with --no-pr, got:\n%s", out)
	}
}

func TestShip_ForgeFlag_Overrides(t *testing.T) {
	// --forge github with configForge="" → flag takes precedence.
	opts, d, _, _ := makeStep7Deps("", "github", func(string) bool { return false })
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Forge:     github (source: flag)") {
		t.Fatalf("expected forge=github source=flag in output, got:\n%s", out)
	}
}

func TestShip_DryRun_NoForgeCLI(t *testing.T) {
	opts, d, _, cli := makeStep7Deps("github", "", func(string) bool { return true })
	opts.dryRun = true
	if err := runShip(opts, d); err != nil {
		t.Fatalf("dry-run should not fail: %v", err)
	}
	if len(cli.calls) > 0 {
		t.Fatalf("execForgeCLI must not be called in dry-run mode")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run]") && !strings.Contains(out, "would open") {
		t.Fatalf("expected dry-run marker for step 7, got:\n%s", out)
	}
}

func TestShip_SourceMentionedInOutput(t *testing.T) {
	// configForge set → source should be "config".
	opts, d, _, _ := makeStep7Deps("gitlab", "", func(string) bool { return false })
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "source: config") {
		t.Fatalf("expected 'source: config' in output, got:\n%s", out)
	}
}

func TestShip_CLIAvailable_InvokesExecForgeCLI(t *testing.T) {
	opts, d, _, cli := makeStep7Deps("github", "", func(string) bool { return true })
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cli.calls) != 1 {
		t.Fatalf("expected 1 execForgeCLI call, got %d", len(cli.calls))
	}
	call := cli.calls[0]
	if call.name != "gh" {
		t.Fatalf("expected CLI name 'gh', got %q", call.name)
	}
	// Args must contain --title
	found := false
	for _, a := range call.args {
		if a == "--title" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --title in CLI args, got %v", call.args)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// buildForgeCreateArgs unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestBuildForgeCreateArgs_GitHubUsesBody(t *testing.T) {
	adapter := forge.NewAdapter("github", func(string) bool { return false })
	args := buildForgeCreateArgs(adapter, "my title", "my body")
	want := []string{"pr", "create", "--title", "my title", "--body", "my body"}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", args, want)
	}
}

func TestBuildForgeCreateArgs_AzureUsesDescription(t *testing.T) {
	adapter := forge.NewAdapter("azure", func(string) bool { return false })
	args := buildForgeCreateArgs(adapter, "my title", "my body")
	// Must use --description, not --body.
	for i, a := range args {
		if a == "--body" {
			t.Fatalf("azure args must not contain --body (got %v)", args)
		}
		if a == "--description" && i+1 < len(args) && args[i+1] == "my body" {
			return // found correctly
		}
	}
	t.Fatalf("azure args must contain --description my body, got %v", args)
}

func TestBuildForgeCreateArgs_NeverMutatesAdapterSlice(t *testing.T) {
	adapter := forge.NewAdapter("gitlab", func(string) bool { return false })
	original := make([]string, len(adapter.CLIArgs))
	copy(original, adapter.CLIArgs)
	buildForgeCreateArgs(adapter, "t1", "b1")
	buildForgeCreateArgs(adapter, "t2", "b2")
	for i, v := range adapter.CLIArgs {
		if v != original[i] {
			t.Fatalf("adapter.CLIArgs was mutated at index %d: got %q, want %q", i, v, original[i])
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// isGitWriteCmd unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestIsGitWriteCmd(t *testing.T) {
	writes := [][]string{
		{"commit", "-m", "msg"},
		{"push", "origin", "feat/foo"},
		{"push", "-u", "origin", "feat/foo"},
		{"fetch", "origin", "--prune"},
	}
	for _, args := range writes {
		if !isGitWriteCmd(args) {
			t.Errorf("isGitWriteCmd(%v) should be true", args)
		}
	}

	reads := [][]string{
		{"status", "--short"},
		{"diff", "--cached", "--stat"},
		{"diff", "--cached", "--name-only"},
		{"branch", "-r", "--no-merged"},
		{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"},
		{"symbolic-ref", "--short", "HEAD"},
		{"log", "-1"},
	}
	for _, args := range reads {
		if isGitWriteCmd(args) {
			t.Errorf("isGitWriteCmd(%v) should be false (read-only)", args)
		}
	}
}
