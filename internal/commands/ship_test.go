package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	branch      string // returned for symbolic-ref --short HEAD
	stagedFiles string // returned for diff --cached --name-only (empty = nothing staged)
	remoteURL   string // returned for "remote get-url origin"
	baseRef     string // returned for symbolic-ref refs/remotes/origin/HEAD (empty = error → fallback "main")
	commitLog   string // returned for `log <base>..HEAD --no-merges --format=%B<sep>`
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

	case strings.HasPrefix(joined, "symbolic-ref refs/remotes/origin/HEAD"):
		if m.baseRef == "" {
			return "", errors.New("no remote-tracking HEAD")
		}
		return m.baseRef, nil

	case strings.HasPrefix(joined, "diff --cached --name-only"):
		return m.stagedFiles, nil

	case strings.HasPrefix(joined, "log "):
		return m.commitLog, nil

	case strings.HasPrefix(joined, "rev-parse --abbrev-ref --symbolic-full-name @{u}"):
		// Simulate no upstream → push -u
		return "", errors.New("no upstream")

	case strings.HasPrefix(joined, "fetch"):
		// Simulate offline — non-blocking
		return "", errors.New("could not connect")

	case joined == "remote get-url origin":
		return m.remoteURL, nil
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
	cases := []string{"feature/foo", "hotfix/bar", "chores/typo", "mybranch"}
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
	validBranches := []string{"feat/my-feature", "fix/bug-123", "refactor/clean-up", "chore/release-x.y.z", "docs/update-readme"}
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
// Doc-only exception — Steps 1 & 2 skip branch-pattern and governance checks
// ────────────────────────────────────────────────────────────────────────────

func TestShip_DocOnlyBranch_NonConformingName_Allowed(t *testing.T) {
	// "docs/foo" does not match feat|fix|refactor/<slug> — normally rejected by isShipBranch,
	// but every staged file is doc-only, so Step 1's branch-pattern check must be skipped.
	violations := []string{`should never be called`}
	d, _ := makeDeps("docs/foo", "docs/some-note.md", violations)
	err := runShip(shipOpts{message: "docs: update note", dryRun: true}, d)
	if err != nil {
		t.Fatalf("doc-only change on non-conforming branch name should not be blocked: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if strings.Contains(out, "does not match the required pattern") {
		t.Fatalf("doc-only change must not trigger the branch-pattern error, got:\n%s", out)
	}
}

func TestShip_DocOnlyBranch_MissingRoadmap_GovernanceSkipped(t *testing.T) {
	// feat/<slug> is a correctly named branch, but governance (checkGovernance) would fail —
	// doc-only staged content must skip governance entirely, never calling checkGovernance.
	called := false
	m := &mockGit{branch: "feat/doc-fix", stagedFiles: "docs/req/REQ-x.md\nvault/notes/note.md"}
	d := shipDeps{
		execGit: m.exec,
		checkGovernance: func() []string {
			called = true
			return []string{"no matching roadmap in wip/ nor done/"}
		},
		out:          &bytes.Buffer{},
		availFn:      func(string) bool { return false },
		execForgeCLI: func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "docs: fix req", dryRun: true}, d)
	if err != nil {
		t.Fatalf("doc-only change must not be blocked by governance: %v", err)
	}
	if called {
		t.Fatal("checkGovernance must not be called at all for a doc-only change")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Governance: skipped (doc-only change)") {
		t.Fatalf("expected doc-only governance skip message in output, got:\n%s", out)
	}
}

func TestShip_MixedDocAndCode_StillBlockedByGovernance(t *testing.T) {
	// One non-doc file staged alongside doc files must NOT trigger the doc-only exception —
	// governance runs exactly as it does today, and the configured violation still blocks.
	violations := []string{`branch "feat/mixed" is a feat/fix/refactor branch but no roadmap is in wip/`}
	d, _ := makeDeps("feat/mixed", "docs/note.md\ninternal/commands/ship.go", violations)
	err := runShip(shipOpts{message: "feat: mixed change"}, d)
	if err == nil {
		t.Fatal("expected governance error for a mixed doc+code change")
	}
	if !strings.Contains(err.Error(), "governance check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if strings.Contains(out, "skipped (doc-only change)") {
		t.Fatalf("mixed doc+code change must not be treated as doc-only, got:\n%s", out)
	}
}

func TestShip_MixedDocAndCode_NonConformingBranch_StillBlocked(t *testing.T) {
	// Same mixed-content guarantee, but on a branch name outside the ship vocabulary entirely
	// (feat/fix/refactor/chore/docs) — must still fail Step 1's branch-pattern check.
	d, _ := makeDeps("hotfix/mixed", "docs/note.md\ninternal/commands/ship.go", nil)
	err := runShip(shipOpts{message: "fix: mixed change"}, d)
	if err == nil {
		t.Fatal("expected branch-pattern error for a mixed doc+code change on a non-conforming branch")
	}
	if !strings.Contains(err.Error(), "does not match the required pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// chore/docs branch-type exception — Step 2 skips governance regardless of staged content
// ────────────────────────────────────────────────────────────────────────────

func TestShip_ChoreBranch_MixedContent_GovernanceSkipped(t *testing.T) {
	// "chore/release-x.y.z" carries a non-doc file staged (not doc-only) — proves the skip is
	// keyed on branch type, not on the pre-existing doc-only staged-content exception.
	called := false
	m := &mockGit{branch: "chore/release-x.y.z", stagedFiles: "internal/commands/ship.go"}
	d := shipDeps{
		execGit: m.exec,
		checkGovernance: func() []string {
			called = true
			return []string{"should never be called"}
		},
		out:          &bytes.Buffer{},
		availFn:      func(string) bool { return false },
		execForgeCLI: func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "chore: release x.y.z", dryRun: true}, d)
	if err != nil {
		t.Fatalf("chore branch must not be blocked by governance: %v", err)
	}
	if called {
		t.Fatal("checkGovernance must not be called at all for a chore/docs branch")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Governance: skipped (chore/docs branch)") {
		t.Fatalf("expected chore/docs branch-type skip message in output, got:\n%s", out)
	}
}

func TestShip_DocsBranch_MixedContent_GovernanceSkipped(t *testing.T) {
	// Same as above for "docs/", with mixed doc+code staged content.
	called := false
	m := &mockGit{branch: "docs/update-readme", stagedFiles: "docs/note.md\ninternal/commands/ship.go"}
	d := shipDeps{
		execGit: m.exec,
		checkGovernance: func() []string {
			called = true
			return []string{"should never be called"}
		},
		out:          &bytes.Buffer{},
		availFn:      func(string) bool { return false },
		execForgeCLI: func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "docs: update readme", dryRun: true}, d)
	if err != nil {
		t.Fatalf("docs branch must not be blocked by governance: %v", err)
	}
	if called {
		t.Fatal("checkGovernance must not be called at all for a chore/docs branch")
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Governance: skipped (chore/docs branch)") {
		t.Fatalf("expected chore/docs branch-type skip message in output, got:\n%s", out)
	}
}

func TestShip_FeatBranch_NoRoadmap_NonRegression(t *testing.T) {
	// Non-regression: feat/fix/refactor branches must still be hard-gated on governance —
	// loosening the gate for chore/docs must not loosen it for feat/fix/refactor.
	violations := []string{`branch "feat/no-roadmap" is a feat/fix/refactor branch but no roadmap is in wip/`}
	d, _ := makeDeps("feat/no-roadmap", "file.go", violations)
	err := runShip(shipOpts{message: "feat: x", dryRun: true}, d)
	if err == nil {
		t.Fatal("expected governance error — feat/fix/refactor must still be gated")
	}
	if !strings.Contains(err.Error(), "governance check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if strings.Contains(out, "Governance: skipped") {
		t.Fatalf("feat branch must never print a governance-skipped message, got:\n%s", out)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// allDocOnly unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestAllDocOnly(t *testing.T) {
	docOnlyCases := [][]string{
		{"docs/req/REQ-x.md"},
		{"vault/notes/note.md"},
		{"README.md"},
		{"docs/req/REQ-x.md", "vault/notes/note.md", "CHANGELOG.md"},
	}
	for _, files := range docOnlyCases {
		if !allDocOnly(files) {
			t.Errorf("allDocOnly(%v) should be true", files)
		}
	}

	notDocOnlyCases := [][]string{
		nil,
		{},
		{"internal/commands/ship.go"},
		{"docs/req/REQ-x.md", "internal/commands/ship.go"},
		{"go.mod"},
	}
	for _, files := range notDocOnlyCases {
		if allDocOnly(files) {
			t.Errorf("allDocOnly(%v) should be false", files)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// defaultBaseBranch unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestDefaultBaseBranch_SymbolicRefSucceeds(t *testing.T) {
	exec := func(args ...string) (string, error) {
		return "refs/remotes/origin/develop", nil
	}
	if got := defaultBaseBranch(exec); got != "develop" {
		t.Fatalf("expected %q, got %q", "develop", got)
	}
}

func TestDefaultBaseBranch_SymbolicRefFails_FallsBackToMain(t *testing.T) {
	exec := func(args ...string) (string, error) {
		return "", errors.New("no remote-tracking HEAD")
	}
	if got := defaultBaseBranch(exec); got != "main" {
		t.Fatalf("expected fallback %q, got %q", "main", got)
	}
}

func TestDefaultBaseBranch_EmptyOutput_FallsBackToMain(t *testing.T) {
	exec := func(args ...string) (string, error) {
		return "", nil
	}
	if got := defaultBaseBranch(exec); got != "main" {
		t.Fatalf("expected fallback %q, got %q", "main", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// buildPRBody unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestBuildPRBody_ZeroOrOneCommit_MinimalBody(t *testing.T) {
	// Not a regression: 0 or 1 non-merge commit keeps today's minimal body.
	for _, commits := range [][]string{nil, {"feat: single commit"}} {
		body := buildPRBody("feat/my-feature", commits)
		want := "Branch: feat/my-feature\n\nCreated by trackfw ship."
		if body != want {
			t.Fatalf("commits=%v: got %q, want %q", commits, body, want)
		}
	}
}

func TestBuildPRBody_MultipleCommits_AggregatesHistory(t *testing.T) {
	commits := []string{
		"feat(ship): add doc-only exception\n\nSkips governance for docs/vault/md-only staged files.",
		"fix(ship): correct base branch fallback",
		"docs: update roadmap status",
	}
	body := buildPRBody("feat/my-feature", commits)

	if !strings.Contains(body, "## Commits") {
		t.Fatalf("expected '## Commits' heading, got:\n%s", body)
	}
	for _, subject := range []string{
		"- feat(ship): add doc-only exception",
		"- fix(ship): correct base branch fallback",
		"- docs: update roadmap status",
	} {
		if !strings.Contains(body, subject) {
			t.Fatalf("expected subject line %q in body, got:\n%s", subject, body)
		}
	}
	if !strings.Contains(body, "## Detalhes") {
		t.Fatalf("expected '## Detalhes' heading for the commit with a body, got:\n%s", body)
	}
	if !strings.Contains(body, "Skips governance for docs/vault/md-only staged files.") {
		t.Fatalf("expected full commit body under '## Detalhes', got:\n%s", body)
	}
	if !strings.Contains(body, "---\nBranch: feat/my-feature") {
		t.Fatalf("expected trailing 'Branch: feat/my-feature' footer, got:\n%s", body)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// gitCommitsSince unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestGitCommitsSince_ParsesSeparatedCommits(t *testing.T) {
	m := &mockGit{
		branch:    "feat/my-feature",
		commitLog: "feat: first" + commitMessageSep + "fix: second\n\nwith a body" + commitMessageSep,
	}
	commits := gitCommitsSince("main", m.exec)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", len(commits), commits)
	}
	if commits[0] != "feat: first" {
		t.Fatalf("unexpected first commit: %q", commits[0])
	}
	if commits[1] != "fix: second\n\nwith a body" {
		t.Fatalf("unexpected second commit: %q", commits[1])
	}
}

func TestGitCommitsSince_EmptyRange_ReturnsNil(t *testing.T) {
	m := &mockGit{branch: "feat/my-feature", commitLog: ""}
	commits := gitCommitsSince("main", m.exec)
	if commits != nil {
		t.Fatalf("expected nil for empty range, got %v", commits)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// End-to-end: --dry-run PR body reflects real branch commit history
// ────────────────────────────────────────────────────────────────────────────

func TestShip_DryRun_PRBodyAggregatesCommitHistory(t *testing.T) {
	m := &mockGit{
		branch:      "feat/my-feature",
		stagedFiles: "file.go",
		remoteURL:   "https://github.com/org/repo.git",
		baseRef:     "refs/remotes/origin/main",
		commitLog:   "feat(x): third commit" + commitMessageSep + "feat(x): second commit" + commitMessageSep,
	}
	d := shipDeps{
		execGit:         m.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		configForge:     "github",
		availFn:         func(string) bool { return false },
		execForgeCLI:    func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "feat(x): first commit (this ship call)", dryRun: true}, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] Title: feat(x): first commit (this ship call)") {
		t.Fatalf("expected dry-run title line, got:\n%s", out)
	}
	if !strings.Contains(out, "## Commits") || !strings.Contains(out, "feat(x): third commit") {
		t.Fatalf("expected aggregated commit history in dry-run body, got:\n%s", out)
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
	valid := []string{"feat/foo", "feat/a-very-long-slug", "fix/123", "refactor/clean-up", "chore/x", "docs/x"}
	for _, b := range valid {
		if !isShipBranch(b) {
			t.Errorf("isShipBranch(%q) should be true", b)
		}
	}

	invalid := []string{"main", "master", "feature/foo", "hotfix/bar", "feat/", "refactor/", "chore/", "docs/"}
	for _, b := range invalid {
		if isShipBranch(b) {
			t.Errorf("isShipBranch(%q) should be false", b)
		}
	}
}

func TestIsGatedShipBranch(t *testing.T) {
	gated := []string{"feat/foo", "fix/123", "refactor/clean-up"}
	for _, b := range gated {
		if !isGatedShipBranch(b) {
			t.Errorf("isGatedShipBranch(%q) should be true", b)
		}
	}

	notGated := []string{"chore/x", "docs/x", "main", "feature/foo", "chore/", "docs/"}
	for _, b := range notGated {
		if isGatedShipBranch(b) {
			t.Errorf("isGatedShipBranch(%q) should be false", b)
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
	// dry-run output must contain either the would-open marker or the not-available marker.
	hasDryRunMarker := strings.Contains(out, "[dry-run]") && (strings.Contains(out, "would open") || strings.Contains(out, "not available"))
	if !hasDryRunMarker {
		t.Fatalf("expected dry-run step-7 marker in output, got:\n%s", out)
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

// ────────────────────────────────────────────────────────────────────────────
// SilenceUsage — cobra shows usage on flag errors; hides it on runtime errors
// ────────────────────────────────────────────────────────────────────────────

// TestShip_SilenceUsage_FlagErrorShowsUsage verifies that an unknown flag still
// triggers cobra's usage output. SilenceUsage is only set inside RunE, which is
// never reached when cobra itself rejects the flag.
func TestShip_SilenceUsage_FlagErrorShowsUsage(t *testing.T) {
	cmd := newShipCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--bogus-unknown-flag-xyz"})
	_ = cmd.Execute()
	out := buf.String()
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("flag error must show 'Usage:' so users know the right syntax; got:\n%s", out)
	}
}

// TestShip_SilenceUsage_SetInRunE verifies at the source level that
// "cmd.SilenceUsage = true" appears in ship.go's RunE, proving runtime errors
// will not emit cobra's usage block.
func TestShip_SilenceUsage_SetInRunE(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("runtime.Caller unavailable")
	}
	shipFile := filepath.Join(filepath.Dir(thisFile), "ship.go")
	src, err := os.ReadFile(shipFile)
	if err != nil {
		t.Skipf("could not read ship.go: %v", err)
	}
	if !strings.Contains(string(src), "cmd.SilenceUsage = true") {
		t.Fatal("ship.go RunE must set cmd.SilenceUsage = true so runtime errors do not show cobra Usage:")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Forge matrix — 4 forges × CLI present/absent × known/self-hosted host
// ────────────────────────────────────────────────────────────────────────────

// forgeMatrixCase describes one cell of the 4×2×2 test matrix.
type forgeMatrixCase struct {
	name string
	// Inputs to shipDeps
	configForge string
	remoteURL   string
	cliAvail    bool // ignored for bitbucket (always Available=false)
	// Expected outputs
	wantForge  string
	wantSource string // "remote" or "config"
	wantNoun   string // "Pull Request" or "Merge Request"
	wantMROnly bool   // if true, assert "Merge Request" in output (gitlab only)
	wantNotMR  bool   // if true, assert "Merge Request" NOT in output
	wantURL    bool   // if true, URL must appear (CLI absent / bitbucket)
	wantCLI    bool   // if true, execForgeCLI must be called (CLI present, non-bitbucket)
}

func TestShip_ForgeMatrix(t *testing.T) {
	cases := []forgeMatrixCase{
		// ── GitHub ──────────────────────────────────────────────────────────
		{
			name: "github/known-host/cli-absent",
			configForge: "", remoteURL: "https://github.com/org/repo.git", cliAvail: false,
			wantForge: "github", wantSource: "remote", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			name: "github/known-host/cli-present",
			configForge: "", remoteURL: "https://github.com/org/repo.git", cliAvail: true,
			wantForge: "github", wantSource: "remote", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: false, wantCLI: true,
		},
		{
			name: "github/self-hosted/cli-absent",
			configForge: "github", remoteURL: "https://git.company.com/org/repo.git", cliAvail: false,
			wantForge: "github", wantSource: "config", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			name: "github/self-hosted/cli-present",
			configForge: "github", remoteURL: "https://git.company.com/org/repo.git", cliAvail: true,
			wantForge: "github", wantSource: "config", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: false, wantCLI: true,
		},
		// ── GitLab ──────────────────────────────────────────────────────────
		{
			name: "gitlab/known-host/cli-absent",
			configForge: "", remoteURL: "https://gitlab.com/org/repo.git", cliAvail: false,
			wantForge: "gitlab", wantSource: "remote", wantNoun: "Merge Request",
			wantMROnly: true, wantURL: true, wantCLI: false,
		},
		{
			name: "gitlab/known-host/cli-present",
			configForge: "", remoteURL: "https://gitlab.com/org/repo.git", cliAvail: true,
			wantForge: "gitlab", wantSource: "remote", wantNoun: "Merge Request",
			wantMROnly: true, wantURL: false, wantCLI: true,
		},
		{
			name: "gitlab/self-hosted/cli-absent",
			configForge: "gitlab", remoteURL: "https://gitlab.company.com/org/repo.git", cliAvail: false,
			wantForge: "gitlab", wantSource: "config", wantNoun: "Merge Request",
			wantMROnly: true, wantURL: true, wantCLI: false,
		},
		{
			name: "gitlab/self-hosted/cli-present",
			configForge: "gitlab", remoteURL: "https://gitlab.company.com/org/repo.git", cliAvail: true,
			wantForge: "gitlab", wantSource: "config", wantNoun: "Merge Request",
			wantMROnly: true, wantURL: false, wantCLI: true,
		},
		// ── Bitbucket — no official CLI; always falls back to URL ────────────
		{
			name: "bitbucket/known-host/cli-absent",
			configForge: "", remoteURL: "https://bitbucket.org/org/repo.git", cliAvail: false,
			wantForge: "bitbucket", wantSource: "remote", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			// bitbucket adapter never calls availFn — Available is always false.
			name: "bitbucket/known-host/cli-present",
			configForge: "", remoteURL: "https://bitbucket.org/org/repo.git", cliAvail: true,
			wantForge: "bitbucket", wantSource: "remote", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			name: "bitbucket/self-hosted/cli-absent",
			configForge: "bitbucket", remoteURL: "https://bitbucket.company.com/org/repo.git", cliAvail: false,
			wantForge: "bitbucket", wantSource: "config", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			name: "bitbucket/self-hosted/cli-present",
			configForge: "bitbucket", remoteURL: "https://bitbucket.company.com/org/repo.git", cliAvail: true,
			wantForge: "bitbucket", wantSource: "config", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		// ── Azure ────────────────────────────────────────────────────────────
		{
			name: "azure/known-host/cli-absent",
			configForge: "", remoteURL: "https://dev.azure.com/org/project/_git/repo", cliAvail: false,
			wantForge: "azure", wantSource: "remote", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			name: "azure/known-host/cli-present",
			configForge: "", remoteURL: "https://dev.azure.com/org/project/_git/repo", cliAvail: true,
			wantForge: "azure", wantSource: "remote", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: false, wantCLI: true,
		},
		{
			name: "azure/self-hosted/cli-absent",
			configForge: "azure", remoteURL: "https://azdo.company.com/org/project/_git/repo", cliAvail: false,
			wantForge: "azure", wantSource: "config", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: true, wantCLI: false,
		},
		{
			name: "azure/self-hosted/cli-present",
			configForge: "azure", remoteURL: "https://azdo.company.com/org/project/_git/repo", cliAvail: true,
			wantForge: "azure", wantSource: "config", wantNoun: "Pull Request",
			wantNotMR: true, wantURL: false, wantCLI: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cliCalls := &mockForgeCLI{}

			g := &mockGit{
				branch:      "feat/matrix-test",
				stagedFiles: "file.go",
				remoteURL:   tc.remoteURL,
			}
			d := shipDeps{
				execGit:         g.exec,
				checkGovernance: func() []string { return nil },
				out:             &bytes.Buffer{},
				configForge:     tc.configForge,
				repoDir:         "", // no CI file detection
				availFn:         func(string) bool { return tc.cliAvail },
				execForgeCLI:    cliCalls.exec,
			}
			opts := shipOpts{message: "feat(matrix): test", forge: ""}
			err := runShip(opts, d)
			if err != nil {
				t.Fatalf("runShip returned unexpected error: %v", err)
			}

			out := d.out.(*bytes.Buffer).String()

			// Forge line: "Forge:     <forge> (source: <source>)"
			wantForgeLine := fmt.Sprintf("Forge:     %s (source: %s)", tc.wantForge, tc.wantSource)
			if !strings.Contains(out, wantForgeLine) {
				t.Errorf("want forge line %q in output, got:\n%s", wantForgeLine, out)
			}

			// Noun in output
			if !strings.Contains(out, tc.wantNoun) {
				t.Errorf("want noun %q in output, got:\n%s", tc.wantNoun, out)
			}

			// Negative: non-gitlab forges must not say "Merge Request"
			if tc.wantNotMR && strings.Contains(out, "Merge Request") {
				t.Errorf("non-gitlab forge %q must not output 'Merge Request', got:\n%s", tc.wantForge, out)
			}

			// URL assertion (fallback path / bitbucket)
			if tc.wantURL {
				// Extract expected URL host from remoteURL
				urlHost := extractURLHost(tc.remoteURL)
				if !strings.Contains(out, urlHost) {
					t.Errorf("want fallback URL containing %q in output, got:\n%s", urlHost, out)
				}
			}

			// CLI invocation assertion
			if tc.wantCLI {
				if len(cliCalls.calls) != 1 {
					t.Errorf("want execForgeCLI called once, got %d calls", len(cliCalls.calls))
				}
			} else {
				if len(cliCalls.calls) != 0 {
					t.Errorf("want execForgeCLI NOT called, got %d calls", len(cliCalls.calls))
				}
			}
		})
	}
}

// extractURLHost extracts the hostname from a URL for assertion purposes.
// e.g. "https://github.com/org/repo.git" → "github.com"
func extractURLHost(rawURL string) string {
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	if idx := strings.IndexByte(rawURL, '/'); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
}

// ────────────────────────────────────────────────────────────────────────────
// Integration test — real binary with clean PATH (proves graceful degradation)
// ────────────────────────────────────────────────────────────────────────────

// findProjectRoot walks up from this test file's location until it finds go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller unavailable")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found: could not determine project root")
		}
		dir = parent
	}
}

// TestShip_Integration_GracefulDegradation_RealBinary builds the trackfw binary and
// runs it with a clean PATH that contains only git — proving that when gh/glab/az are
// absent, ship still exits 0 and prints a browser URL.
//
// Uses --dry-run to avoid requiring network or a real git push target; --dry-run
// exercises the CLI availability check (Step 7) because NewAdapter is called before
// the dry-run guard.
func TestShip_Integration_GracefulDegradation_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: skipped in short mode (-short)")
	}

	projRoot := findProjectRoot(t)

	// ── Build binary ──────────────────────────────────────────────────────────
	binaryDir := t.TempDir()
	binary := filepath.Join(binaryDir, "trackfw")
	buildCmd := exec.Command("go", "build", "-o", binary, "./cmd/trackfw")
	buildCmd.Dir = projRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build trackfw: %v\n%s", err, out)
	}

	// ── Create tmpbin with ONLY git ───────────────────────────────────────────
	tmpBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(tmpBin, 0755); err != nil {
		t.Fatalf("mkdir tmpbin: %v", err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found in PATH — cannot run integration test")
	}
	if err := os.Symlink(gitPath, filepath.Join(tmpBin, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	cleanPATH := tmpBin

	// ── Verify gh/glab/az are NOT in cleanPATH ───────────────────────────────
	for _, cli := range []string{"gh", "glab", "az"} {
		// Temporarily set PATH to clean version and check.
		orig := os.Getenv("PATH")
		_ = os.Setenv("PATH", cleanPATH)
		_, lErr := exec.LookPath(cli)
		_ = os.Setenv("PATH", orig)
		if lErr == nil {
			t.Logf("note: %s found in cleanPATH despite our setup — test might not prove degradation", cli)
		}
	}

	// ── Create temp git repo ──────────────────────────────────────────────────
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = repoDir
		cmd.Env = []string{
			"PATH=" + cleanPATH,
			"HOME=" + t.TempDir(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-b", "feat/ship-integration-test")
	runGit("remote", "add", "origin", "https://github.com/org/repo.git")

	// ── Create governance artifacts (default RoadmapDir = "docs/roadmaps") ───
	wipDir := filepath.Join(repoDir, "docs", "roadmaps", "wip")
	if err := os.MkdirAll(wipDir, 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}
	roadmapContent := "REQ: REQ-ship-integration-test\n\n# Roadmap: Integration Test\n\nTest roadmap for graceful degradation proof.\n"
	roadmapPath := filepath.Join(wipDir, "ROADMAP-2026-07-26-ship-integration-test.md")
	if err := os.WriteFile(roadmapPath, []byte(roadmapContent), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	// ── Stage a file ──────────────────────────────────────────────────────────
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write test.txt: %v", err)
	}
	runGit("add", "test.txt")

	// ── Run binary with clean PATH ────────────────────────────────────────────
	shipCmd := exec.Command(binary, "ship", "--dry-run", "-m", "feat: integration test")
	shipCmd.Dir = repoDir
	shipCmd.Env = []string{
		"PATH=" + cleanPATH,
		"HOME=" + t.TempDir(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	}

	var stdout, stderr bytes.Buffer
	shipCmd.Stdout = &stdout
	shipCmd.Stderr = &stderr

	if err := shipCmd.Run(); err != nil {
		t.Fatalf("trackfw ship --dry-run must exit 0 when gh is absent:\nstdout: %s\nstderr: %s\nerror:  %v",
			stdout.String(), stderr.String(), err)
	}

	combined := stdout.String() + "\n" + stderr.String()

	// The dry-run now reports availability — expect the fallback URL.
	if !strings.Contains(combined, "github.com") {
		t.Fatalf("expected github.com fallback URL in dry-run output (proves graceful degradation), got:\n%s", combined)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ML-2A — detectPendingSquashMerges reuses evaluateBranchIntegration
// ────────────────────────────────────────────────────────────────────────────

// TestDetectPendingSquashMerges_RealGitRepo_StaleIntegratedVsGenuinelyPending is the P4
// falsification scenario for ML-2A (REQ-2026-08-18): reproduces the PR #181/#182 incident in a
// real, disposable git repository. origin/feat/a is squash-merged into origin/main, which then
// advances further (origin/feat/b), leaving origin/feat/a's naive bidirectional diff non-empty
// even though every file it touched is already on main. origin/feat/pending never merges
// anywhere and must still warn.
//
// Baseline (P4): asserts the naive check IS non-empty for origin/feat/a first — proving the test
// discriminates against the pre-fix behavior instead of passing vacuously. Detection: the fixed
// detectPendingSquashMerges (calling evaluateBranchIntegration) must NOT warn about feat/a and
// MUST warn about feat/pending.
func TestDetectPendingSquashMerges_RealGitRepo_StaleIntegratedVsGenuinelyPending(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")

	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+filepath.Join(work, "empty-gitconfig"),
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_TERMINAL_PROMPT=0",
			"HOME="+work,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v (dir=%s) failed: %v\n%s", args, dir, err, out)
		}
		return string(out)
	}

	if err := os.WriteFile(filepath.Join(work, "empty-gitconfig"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "main")

	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "user.email", "falsify@trackfw.test")
	run(cloneDir, "config", "user.name", "trackfw falsify")
	run(cloneDir, "config", "commit.gpgsign", "false")
	run(cloneDir, "config", "core.hooksPath", "/dev/null")

	writeFile := func(name, content string) {
		if err := os.WriteFile(filepath.Join(cloneDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("base.txt", "base\n")
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")
	run(cloneDir, "push", "-q", "origin", "main")

	// feat/a — pushed to origin, then squash-merged into main. Ancestry never records the merge
	// (the git branch -d false negative), so it stays in `branch -r --no-merged origin/main`.
	run(cloneDir, "checkout", "-q", "-b", "feat/a")
	writeFile("a.txt", "a\n")
	run(cloneDir, "add", "a.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/a work")
	run(cloneDir, "push", "-q", "origin", "feat/a")
	run(cloneDir, "checkout", "-q", "main")
	run(cloneDir, "merge", "-q", "--squash", "feat/a")
	run(cloneDir, "commit", "-q", "-m", "squash-merge feat/a (PR #181)")

	// main advances further (PR #182) — this is what makes the naive bidirectional diff non-empty
	// for the already-integrated feat/a.
	writeFile("b.txt", "b\n")
	run(cloneDir, "add", "b.txt")
	run(cloneDir, "commit", "-q", "-m", "unrelated follow-up (PR #182)")
	run(cloneDir, "push", "-q", "origin", "main")

	// feat/pending — pushed to origin, genuinely never merged anywhere.
	run(cloneDir, "checkout", "-q", "-b", "feat/pending")
	writeFile("c.txt", "c\n")
	run(cloneDir, "add", "c.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/pending work, never merged")
	run(cloneDir, "push", "-q", "origin", "feat/pending")

	run(cloneDir, "checkout", "-q", "main")
	run(cloneDir, "fetch", "-q", "origin")

	gitExec := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+filepath.Join(work, "empty-gitconfig"),
			"GIT_CONFIG_SYSTEM=/dev/null",
			"HOME="+work,
		)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	// P4 baseline: prove the naive bidirectional check (the pre-fix behavior) IS non-empty for
	// origin/feat/a — this is the exact false positive the PR #181/#182 incident reported.
	naiveDiff, err := gitExec("diff", "origin/main", "origin/feat/a", "--stat")
	if err != nil {
		t.Fatalf("naive diff failed: %v", err)
	}
	if strings.TrimSpace(naiveDiff) == "" {
		t.Fatal("test setup invalid: naive diff origin/main origin/feat/a --stat must be non-empty to reproduce the #181/#182 false positive")
	}

	// P4 detection: the fixed detectPendingSquashMerges must not warn about feat/a (stale but
	// integrated) and must still warn about feat/pending (genuinely unmerged).
	out := &bytes.Buffer{}
	detectPendingSquashMerges("main", gitExec, out)
	got := out.String()

	if strings.Contains(got, `"feat/a"`) {
		t.Fatalf("detectPendingSquashMerges must NOT warn about feat/a (stale-but-integrated squash-merge) — this is the AC7 discriminant. Output:\n%s", got)
	}
	if !strings.Contains(got, `"feat/pending"`) {
		t.Fatalf("detectPendingSquashMerges must still warn about feat/pending (genuinely unmerged) — non-regression. Output:\n%s", got)
	}
}

// TestDetectPendingSquashMerges_CallsSharedEvaluateBranchIntegration is a narrower, fast
// (fake-gitExec) test proving detectPendingSquashMerges routes through evaluateBranchIntegration
// instead of maintaining its own bidirectional diff: for a candidate whose merge-base call fails,
// no warning fires — a raw diff-based implementation would still have attempted `git diff
// origin/main <candidate> --stat` and could have warned regardless of merge-base failing.
func TestDetectPendingSquashMerges_CallsSharedEvaluateBranchIntegration(t *testing.T) {
	calls := map[string]int{}
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		calls[key]++
		switch {
		case key == "branch -r --no-merged origin/main":
			return "  origin/feat/unrelated-history\n", nil
		case strings.HasPrefix(key, "merge-base origin/main"):
			return "", fmt.Errorf("fatal: no merge base")
		case strings.HasPrefix(key, "diff origin/main"):
			// A raw bidirectional-diff implementation would call this directly; the shared
			// evaluateBranchIntegration only reaches its own -z diffs after a successful
			// merge-base, which never happens here.
			return "some.file | 1 +\n", nil
		default:
			return "", nil
		}
	}

	out := &bytes.Buffer{}
	detectPendingSquashMerges("main", gitExec, out)

	if out.Len() != 0 {
		t.Fatalf("expected no warning when merge-base fails (routed through evaluateBranchIntegration -> no_merge_base), got:\n%s", out.String())
	}
	if calls["diff origin/main origin/feat/unrelated-history --stat"] > 0 {
		t.Fatal("detectPendingSquashMerges must not run its own bidirectional diff --stat anymore — it must delegate entirely to evaluateBranchIntegration")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// --force-with-lease — ML-1A
// ────────────────────────────────────────────────────────────────────────────

// makeForceLeaseDeps returns opts/deps ready to reach the force-with-lease gate: valid gated
// branch, no governance violations, github forge resolved via config, CLI available.
func makeForceLeaseDeps(staged string, checkPROpen func(forge.Adapter, string) (bool, error)) (shipOpts, shipDeps, *mockGit, *mockForgeCLI) {
	g := &mockGit{branch: "fix/rebase-test", stagedFiles: staged, remoteURL: "https://github.com/org/repo.git"}
	cli := &mockForgeCLI{}
	d := shipDeps{
		execGit:         g.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		configForge:     "github",
		repoDir:         "",
		availFn:         func(string) bool { return true },
		execForgeCLI:    cli.exec,
		checkPROpen:     checkPROpen,
	}
	opts := shipOpts{message: "fix: rebase", forceWithLease: true}
	return opts, d, g, cli
}

func TestShip_ForceWithLease_OpenPR_Succeeds(t *testing.T) {
	opts, d, g, cli := makeForceLeaseDeps("file.go", func(adapter forge.Adapter, branch string) (bool, error) {
		if branch != "fix/rebase-test" {
			t.Fatalf("unexpected branch passed to checkPROpen: %q", branch)
		}
		return true, nil
	})
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cli.calls) > 0 {
		t.Fatalf("execForgeCLI must not be called to create a PR when force-with-lease already confirmed one is open, got %d calls", len(cli.calls))
	}

	var pushCall []string
	for _, c := range g.calls {
		if len(c) > 0 && c[0] == "push" {
			pushCall = c
		}
	}
	if pushCall == nil {
		t.Fatal("expected a push call")
	}
	want := []string{"push", "--force-with-lease", "-u", "origin", "fix/rebase-test"}
	if strings.Join(pushCall, " ") != strings.Join(want, " ") {
		t.Fatalf("push args = %v, want %v", pushCall, want)
	}

	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "already open — skipping creation (--force-with-lease)") {
		t.Fatalf("expected skip-creation message, got:\n%s", out)
	}
}

func TestShip_ForceWithLease_PushOnly_WhenNothingStaged(t *testing.T) {
	opts, d, g, _ := makeForceLeaseDeps("", func(forge.Adapter, string) (bool, error) { return true, nil })
	opts.message = "" // no -m required in push-only mode
	if err := runShip(opts, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range g.calls {
		if len(c) > 0 && c[0] == "commit" {
			t.Fatalf("commit must not be called when nothing is staged, got call: %v", c)
		}
	}
	out := d.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "pushes existing commits only") {
		t.Fatalf("expected push-only notice, got:\n%s", out)
	}
}

func TestShip_ForceWithLease_NothingStaged_NoFlag_StillAborts(t *testing.T) {
	// Non-regression: without --force-with-lease, nothing staged still aborts exactly as before.
	g := &mockGit{branch: "fix/rebase-test", stagedFiles: ""}
	d := shipDeps{
		execGit:         g.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		availFn:         func(string) bool { return false },
		execForgeCLI:    func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "fix: x"}, d)
	if err == nil || !strings.Contains(err.Error(), "nothing is staged") {
		t.Fatalf("expected 'nothing is staged' error, got: %v", err)
	}
}

func TestShip_ForceWithLease_NoOpenPR_Refuses(t *testing.T) {
	opts, d, g, cli := makeForceLeaseDeps("file.go", func(forge.Adapter, string) (bool, error) { return false, nil })
	err := runShip(opts, d)
	if err == nil {
		t.Fatal("expected refusal when no PR is open")
	}
	if !strings.Contains(err.Error(), "no open pull/merge request") {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range g.calls {
		if len(c) > 0 && (c[0] == "commit" || c[0] == "push") {
			t.Fatalf("refusal must happen before any write — got call: %v", c)
		}
	}
	if len(cli.calls) > 0 {
		t.Fatalf("execForgeCLI must not be called, got %d calls", len(cli.calls))
	}
}

func TestShip_ForceWithLease_NoForgeCLI_RefusesWithoutDegrading(t *testing.T) {
	g := &mockGit{branch: "fix/rebase-test", stagedFiles: "file.go", remoteURL: "https://github.com/org/repo.git"}
	d := shipDeps{
		execGit:         g.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		configForge:     "github",
		availFn:         func(string) bool { return false }, // CLI absent
		execForgeCLI:    func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "fix: rebase", forceWithLease: true}, d)
	if err == nil {
		t.Fatal("expected refusal when forge CLI is unavailable")
	}
	if !strings.Contains(err.Error(), "requires a forge CLI") {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range g.calls {
		if len(c) > 0 && (c[0] == "commit" || c[0] == "push") {
			t.Fatalf("refusal must happen before any write — got call: %v", c)
		}
	}
}

func TestShip_ForceWithLease_ManualForge_Refuses(t *testing.T) {
	// No forge flag, no config, no remote URL → manual — must refuse, never degrade.
	g := &mockGit{branch: "fix/rebase-test", stagedFiles: "file.go"}
	d := shipDeps{
		execGit:         g.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		availFn:         func(string) bool { return true },
		execForgeCLI:    func(string, []string) error { return nil },
	}
	err := runShip(shipOpts{message: "fix: rebase", forceWithLease: true}, d)
	if err == nil {
		t.Fatal("expected refusal for manual forge")
	}
	if !strings.Contains(err.Error(), "requires a forge CLI") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShip_ForceWithLease_CannotVerify_Refuses(t *testing.T) {
	opts, d, g, _ := makeForceLeaseDeps("file.go", func(forge.Adapter, string) (bool, error) {
		return false, fmt.Errorf("gh: authentication required")
	})
	err := runShip(opts, d)
	if err == nil {
		t.Fatal("expected refusal when PR status cannot be verified")
	}
	if !strings.Contains(err.Error(), "could not verify") {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range g.calls {
		if len(c) > 0 && (c[0] == "commit" || c[0] == "push") {
			t.Fatalf("refusal must happen before any write — got call: %v", c)
		}
	}
}

func TestShip_ForceWithLease_DryRun_StillRunsGate(t *testing.T) {
	called := false
	opts, d, _, _ := makeForceLeaseDeps("file.go", func(forge.Adapter, string) (bool, error) {
		called = true
		return false, nil
	})
	opts.dryRun = true
	err := runShip(opts, d)
	if err == nil {
		t.Fatal("expected refusal even in dry-run when no PR is open")
	}
	if !called {
		t.Fatal("checkPROpen must run in dry-run mode too — it is read-only")
	}
}

func TestShip_ForceWithLease_NormalPush_Unaffected(t *testing.T) {
	// Non-regression: --force-with-lease not set → push args are exactly as before.
	g := &mockGit{branch: "fix/normal", stagedFiles: "file.go"}
	d := shipDeps{
		execGit:         g.exec,
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
		availFn:         func(string) bool { return false },
		execForgeCLI:    func(string, []string) error { return nil },
	}
	if err := runShip(shipOpts{message: "fix: x"}, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var pushCall []string
	for _, c := range g.calls {
		if len(c) > 0 && c[0] == "push" {
			pushCall = c
		}
	}
	want := []string{"push", "-u", "origin", "fix/normal"}
	if strings.Join(pushCall, " ") != strings.Join(want, " ") {
		t.Fatalf("push args = %v, want %v", pushCall, want)
	}
}

func TestShip_ForceFlagDoesNotExist(t *testing.T) {
	cmd := newShipCmd()
	if flag := cmd.Flags().Lookup("force"); flag != nil {
		t.Fatalf("raw --force flag must never be registered on ship — found: %+v", flag)
	}
	if flag := cmd.Flags().Lookup("force-with-lease"); flag == nil {
		t.Fatal("expected --force-with-lease flag to be registered")
	}
}
