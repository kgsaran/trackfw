package validator

// Tests for ResolveBranchRoadmapForExisting (D2 of ADR-2026-10-01) and related gates.
//
// All tests use real git repositories in t.TempDir() — no mock of the matcher.
// Where an "origin" is needed, a bare repository is used as the remote.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// initBareOrigin creates a bare git repository at bareDir and configures the
// working repository at workDir to use it as "origin".
// It commits the given files (map[relPath]content) to the bare repo so that
// deriveOriginDefaultBranch() can resolve "origin/main".
func initBareOrigin(t *testing.T, workDir, bareDir string, files map[string]string) {
	t.Helper()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %s", args, dir, out)
		}
	}

	// Init the bare repo.
	run(bareDir, "init", "--bare", bareDir)

	// Commit the initial files in a temporary clone.
	tmpClone := t.TempDir()
	run(tmpClone, "init")
	run(tmpClone, "config", "user.email", "test@test.com")
	run(tmpClone, "config", "user.name", "test")
	for rel, content := range files {
		path := filepath.Join(tmpClone, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writeFile: %v", err)
		}
		run(tmpClone, "add", rel)
	}
	run(tmpClone, "commit", "--allow-empty", "-m", "init")
	run(tmpClone, "remote", "add", "origin", bareDir)
	run(tmpClone, "push", "origin", "HEAD:main")

	// Add the bare repo as origin to the working directory.
	run(workDir, "remote", "add", "origin", bareDir)
	run(workDir, "fetch", "origin")
}

// newExistingBranchFixture sets up a working directory with:
//   - a git repo on the given feat/fix/refactor branch
//   - trackfw.yaml pointing to docs/roadmaps
//
// The caller places roadmaps via writeFile after calling this.
func newExistingBranchFixture(t *testing.T, branch string) (dir string, cfg config.ProjectConfig) {
	t.Helper()
	dir = t.TempDir()
	initGitRepo(t, dir, branch)
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	cfg = config.Load()
	return
}

// ────────────────────────────────────────────────────────────────────────────
// D4: blocked/ governs via written link (link not stale)
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchRoadmapForExisting_BlockedLinkNotStale affirms: a written link pointing to a
// roadmap in blocked/ is InScope (not stale) after D4 expands the scope to wip∪blocked∪done.
func TestResolveBranchRoadmapForExisting_BlockedLinkNotStale(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const roadmap = "ROADMAP-2026-10-01-cache-de-sessao.md"
	writeFile(t, dir, "docs/roadmaps/blocked/"+roadmap, "# r\n")
	// Write a link pointing to the roadmap in blocked/.
	writeFile(t, dir, "docs/roadmaps/"+BranchLinkFileName,
		`{"version":1,"links":{"fix/cache-de-sessao":"`+roadmap+`"}}`)

	wipDirs := ResolveWIPDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)
	link := BranchLinkFor(cfg, "fix/cache-de-sessao", wipDirs, doneDirs)

	if !link.Present {
		t.Fatalf("link must be present")
	}
	if !link.InScope {
		// D4: blocked dir must be in scope so the link is not stale.
		t.Errorf("link for roadmap in blocked/ must be InScope after D4 scope expansion, got stale")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D2 item 2: blocked/ governs via inference (without written link)
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchRoadmapForExisting_BlockedGovernsInference affirms: a roadmap in blocked/ that
// matches the branch slug governs via inference — the branch is not "ungoverned" just because
// it was moved to blocked/ (issue #490).
func TestResolveBranchRoadmapForExisting_BlockedGovernsInference(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	writeFile(t, dir, "docs/roadmaps/blocked/ROADMAP-cache-de-sessao.md", "# r\n")
	// No link file — pure inference path.

	t.Setenv("TRACKFW_BRANCH", "fix/cache-de-sessao")

	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)

	if !res.Matched {
		t.Errorf("roadmap in blocked/ must govern by inference (D2 item 2), got: %+v", res)
	}
	if res.Source != "inference" {
		t.Errorf("source must be inference, got %q", res.Source)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("no warnings expected for a clean blocked/ match, got: %v", res.Warnings)
	}
}

// TestResolveBranchRoadmapForExisting_BlockedGovernsWithLink affirms: a roadmap in blocked/ that
// has a written link also governs (written-link source, not stale).
func TestResolveBranchRoadmapForExisting_BlockedGovernsWithLink(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const roadmap = "ROADMAP-cache-de-sessao.md"
	writeFile(t, dir, "docs/roadmaps/blocked/"+roadmap, "# r\n")
	writeFile(t, dir, "docs/roadmaps/"+BranchLinkFileName,
		`{"version":1,"links":{"fix/cache-de-sessao":"`+roadmap+`"}}`)

	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)

	if !res.Matched {
		t.Errorf("roadmap in blocked/ with link must govern, got: %+v", res)
	}
	if res.Source != "written-link" {
		t.Errorf("source must be written-link, got %q", res.Source)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D2 item 3: done/ governs only if moved by this branch
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchRoadmapForExisting_DoneMovedByBranchGoverns affirms: a roadmap in done/ that
// is ABSENT from done/ in origin/main governs (the branch moved it → Definition of Done path).
func TestResolveBranchRoadmapForExisting_DoneMovedByBranchGoverns(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const roadmap = "ROADMAP-cache-de-sessao.md"
	writeFile(t, dir, "docs/roadmaps/done/"+roadmap, "# r\n")

	// Origin/main has OTHER files in done/ but NOT the one above — so it looks "moved by branch".
	bareDir := t.TempDir()
	initBareOrigin(t, dir, bareDir, map[string]string{
		"docs/roadmaps/done/ROADMAP-outro-trabalho.md": "# outro\n",
		"trackfw.yaml": "roadmap_dir: docs/roadmaps\n",
	})

	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)

	if !res.Matched {
		t.Errorf("roadmap absent from done/ in origin/main must govern (moved by branch), got: %+v", res)
	}
	if res.Source != "inference" {
		t.Errorf("source must be inference, got %q", res.Source)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("no warnings expected when origin is resolvable, got: %v", res.Warnings)
	}
}

// TestResolveBranchRoadmapForExisting_DonePresentInBaseNoGovernance affirms: a roadmap in done/
// that IS present in done/ in origin/main does NOT govern — it was done before this branch.
func TestResolveBranchRoadmapForExisting_DonePresentInBaseNoGovernance(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const roadmap = "ROADMAP-cache-de-sessao.md"
	writeFile(t, dir, "docs/roadmaps/done/"+roadmap, "# r\n")

	// Origin/main already has the SAME roadmap in done/ — branch didn't move it.
	bareDir := t.TempDir()
	initBareOrigin(t, dir, bareDir, map[string]string{
		"docs/roadmaps/done/" + roadmap: "# pre-existing\n",
		"trackfw.yaml":                  "roadmap_dir: docs/roadmaps\n",
	})

	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)

	if res.Matched {
		t.Errorf("roadmap already in done/ in origin/main must NOT govern (not moved by branch), got matched=true: %+v", res)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("no warnings expected when origin is resolvable, got: %v", res.Warnings)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D3: no origin → unverifiable base → accept + warn, never violation
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchRoadmapForExisting_NoOriginWarnAndAccept affirms: when origin is not
// configured, the done/ scope is unverifiable (D3) → the match is accepted with a warning named
// branch_done_scope_unverifiable, and no violation is emitted.
func TestResolveBranchRoadmapForExisting_NoOriginWarnAndAccept(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const roadmap = "ROADMAP-cache-de-sessao.md"
	writeFile(t, dir, "docs/roadmaps/done/"+roadmap, "# r\n")
	// No origin configured — deriveOriginDefaultBranch() will return ok=false.

	t.Setenv("TRACKFW_BRANCH", "fix/cache-de-sessao")

	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)

	if !res.Matched {
		t.Errorf("D3: without origin, match in done/ must be accepted, got: %+v", res)
	}
	if !hasWarning(res.Warnings, "branch_done_scope_unverifiable") {
		t.Errorf("D3: must emit branch_done_scope_unverifiable warning, got: %v", res.Warnings)
	}
}

// TestCheckShipGovernance_PropagatesWarning affirms: CheckShipGovernance propagates the
// branch_done_scope_unverifiable warning from validateBranchHasWIPRoadmap (A2 of ML-0A) — the
// returned GovernanceViolation has Warnings set and Missing empty when governance passes with
// degraded confidence (no origin, match in done/ only, wip/ empty).
func TestCheckShipGovernance_PropagatesWarning(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "fix/cache-de-sessao")
	// done/ has a roadmap matching the branch slug; wip/ is empty so validateWIPHasREQ is silent.
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-cache-de-sessao.md", "# r\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps/wip"), 0o755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	// No origin configured — deriveOriginDefaultBranch() returns ok=false → D3 fires.
	t.Setenv("TRACKFW_BRANCH", "fix/cache-de-sessao")

	gv := CheckShipGovernance()

	// Governance must pass (no violations) but carry the D3 warning.
	if gv == nil {
		t.Fatal("CheckShipGovernance must return non-nil when warnings exist (A2)")
	}
	if len(gv.Missing) != 0 {
		t.Errorf("no violations expected, got: %v", gv.Missing)
	}
	if !hasWarning(gv.Warnings, "branch_done_scope_unverifiable") {
		t.Errorf("must propagate branch_done_scope_unverifiable warning, got: %v", gv.Warnings)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D2 item 3 + written link: done/ with link governs
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchRoadmapForExisting_DoneMovedByBranchGovernsWithLink affirms: a written link
// pointing to a roadmap in done/ that is absent from origin/main governs (source="written-link").
func TestResolveBranchRoadmapForExisting_DoneMovedByBranchGovernsWithLink(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const roadmap = "ROADMAP-cache-de-sessao.md"
	writeFile(t, dir, "docs/roadmaps/done/"+roadmap, "# r\n")
	writeFile(t, dir, "docs/roadmaps/"+BranchLinkFileName,
		`{"version":1,"links":{"fix/cache-de-sessao":"`+roadmap+`"}}`)

	// Origin/main has OTHER files but NOT this roadmap — so the link target was moved by branch.
	bareDir := t.TempDir()
	initBareOrigin(t, dir, bareDir, map[string]string{
		"docs/roadmaps/done/ROADMAP-outro-trabalho.md": "# outro\n",
		"trackfw.yaml": "roadmap_dir: docs/roadmaps\n",
	})

	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)

	if !res.Matched {
		t.Errorf("written link to done/ roadmap absent from base must govern, got: %+v", res)
	}
	if res.Source != "written-link" {
		t.Errorf("source must be written-link, got %q", res.Source)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("no warnings expected when origin is resolvable and link is in scope, got: %v", res.Warnings)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D1: RecordBranchLink resolves only against wip/ (not done/)
// ────────────────────────────────────────────────────────────────────────────

// TestRecordBranchLink_DoneOnlyNoLink affirms: RecordBranchLink does NOT write a link when the
// only matching roadmap is in done/ — D1 requires inference against wip/ only at creation time,
// so a done/-only match returns without writing.
func TestRecordBranchLink_DoneOnlyNoLink(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	// Roadmap exists only in done/ (matches slug).
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-cache-de-sessao.md", "# r\n")
	// wip/ is empty — no matching roadmap for RecordBranchLink.

	if err := RecordBranchLink(cfg, "fix/cache-de-sessao"); err != nil {
		t.Fatalf("RecordBranchLink error: %v", err)
	}

	// No link file should have been written.
	linkPath := BranchLinkPath(cfg)
	if _, err := os.Stat(linkPath); err == nil {
		t.Errorf("RecordBranchLink must not write a link when match is only in done/ (D1), but %s exists", linkPath)
	}
}

// TestRecordBranchLink_WipAndDonePicksWip affirms: when a roadmap matches the slug in both wip/
// and done/, RecordBranchLink writes the link pointing to the wip/ roadmap (D1 — wip/ only).
func TestRecordBranchLink_WipAndDonePicksWip(t *testing.T) {
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	const wipRoadmap = "ROADMAP-cache-de-sessao.md"
	const doneRoadmap = "ROADMAP-sessao-de-cache.md" // also matches: tokens "cache" + "sessao"
	writeFile(t, dir, "docs/roadmaps/wip/"+wipRoadmap, "# r\n")
	writeFile(t, dir, "docs/roadmaps/done/"+doneRoadmap, "# r\n")

	if err := RecordBranchLink(cfg, "fix/cache-de-sessao"); err != nil {
		t.Fatalf("RecordBranchLink error: %v", err)
	}

	// Link must name the wip/ roadmap, not the done/ one.
	link := BranchLinkFor(cfg, "fix/cache-de-sessao", ResolveWIPDirs(cfg), ResolveDoneDirs(cfg))
	if !link.Present {
		t.Fatalf("link must be written when exactly one wip/ match exists")
	}
	if link.Roadmap != wipRoadmap {
		t.Errorf("link must point to wip/ roadmap %q, got %q", wipRoadmap, link.Roadmap)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// A1: accented roadmap filename in done/ — recognized with -z, rejected without
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchRoadmapForExisting_AccentedFilenameRecognized affirms: a roadmap in done/ whose
// filename contains non-ASCII characters (e.g. "ã") is correctly identified as PRESENT in the base
// tree, so the branch does NOT claim it as "moved by this branch" (Matched=false, Warnings=[]).
// Without -z, git ls-tree quotes the filename under core.quotepath=true and filepath.Base returns
// the escaped form, causing the file to appear absent — the fix (-z + NUL-split) prevents it.
func TestResolveBranchRoadmapForExisting_AccentedFilenameRecognized(t *testing.T) {
	// Roadmap: ASCII tokens "cache"+"sessao" match branch slug; "revisão" has ã (U+00E3).
	const accentedRoadmap = "ROADMAP-cache-de-sessao-revisão.md"
	dir, cfg := newExistingBranchFixture(t, "fix/cache-de-sessao")

	writeFile(t, dir, "docs/roadmaps/done/"+accentedRoadmap, "# r\n")

	// Origin/main already has the accented roadmap in done/ — this branch did NOT move it.
	bareDir := t.TempDir()
	initBareOrigin(t, dir, bareDir, map[string]string{
		"docs/roadmaps/done/" + accentedRoadmap: "# pre-existing\n",
		"trackfw.yaml":                          "roadmap_dir: docs/roadmaps\n",
	})

	// Force core.quotepath=true so git ls-tree quotes non-ASCII filenames.
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	runGit("config", "core.quotepath", "true")

	doneDirs := ResolveDoneDirs(cfg)

	ref, ok := deriveOriginDefaultBranch()
	if !ok {
		t.Skip("origin/main not resolvable — test requires origin setup")
	}

	// PRECONDITION: the old reader (no -z) must NOT find the accented file.
	// If it does, core.quotepath is not active in this environment and the test cannot
	// reproduce the quoting bug — the fixture is invalid.
	oldStyleSet := oldMdBasenamesInGitTree(ref, doneDirs[0])
	if oldStyleSet[accentedRoadmap] {
		t.Fatalf("fixture does not reproduce quoting: old-style ls-tree (no -z) found %q even with core.quotepath=true — cannot validate the A1 fix on this system", accentedRoadmap)
	}

	// New reader (-z + NUL-split) must find the real filename.
	baseSet, err := mdBasenamesInGitTreeWithError(ref, doneDirs[0])
	if err != nil {
		t.Fatalf("mdBasenamesInGitTreeWithError failed: %v", err)
	}
	if !baseSet[accentedRoadmap] {
		t.Errorf("A1: accented roadmap %q must be recognized in base set with -z; got set: %v",
			accentedRoadmap, mapKeys(baseSet))
	}

	// With A1 fix, ResolveBranchRoadmapForExisting must recognise the roadmap as present in base →
	// not moved by this branch → Matched=false, no warnings.
	wipDirs := ResolveWIPDirs(cfg)
	blockedDirs := ResolveBlockedDirs(cfg)

	res := ResolveBranchRoadmapForExisting(cfg, "fix/cache-de-sessao", wipDirs, blockedDirs, doneDirs)
	if res.Matched {
		t.Errorf("A1: accented roadmap present in origin/main must NOT govern (present in base), got: %+v", res)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("A1: no warnings expected when origin is resolvable, got: %v", res.Warnings)
	}
}

// oldMdBasenamesInGitTree replicates the pre-A1 implementation (no -z, newline split) for
// comparison in TestResolveBranchRoadmapForExisting_AccentedFilenameRecognized.
func oldMdBasenamesInGitTree(ref, dirPrefix string) map[string]bool {
	out, err := gitCommand(".", "ls-tree", "-r", "--name-only", ref, "--", dirPrefix).Output()
	set := make(map[string]bool)
	if err != nil {
		return set
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if filepath.Ext(line) == ".md" {
			set[filepath.Base(line)] = true
		}
	}
	return set
}

// mapKeys returns the keys of a map[string]bool for diagnostic output.
func mapKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// D5: ForCreation hint — 4 done matches → 3 named + "e mais 1"
// ────────────────────────────────────────────────────────────────────────────

// TestBranchGovernanceOrientationForCreation_DoneHint affirms: BranchGovernanceOrientationForCreation
// with 4 done/ matches produces a hint that (1) contains the first 3 sorted names, (2) "e mais 1",
// (3) the phrase "do not govern a new branch", (4) the literal "<name>" placeholder, and
// (5) never contains "trackfw roadmap move ROADMAP-" (no ready-to-run command with a concrete name).
// nil doneMatches produces no hint.
func TestBranchGovernanceOrientationForCreation_DoneHint(t *testing.T) {
	cfg := config.ProjectConfig{}
	branch := "feat/cache-de-sessao"

	// nil → no hint
	msg := BranchGovernanceOrientationForCreation(branch, cfg, nil)
	if strings.Contains(msg, "do not govern a new branch") {
		t.Errorf("nil doneMatches must produce no hint, got: %s", msg)
	}

	// 4 matches → 3 listed, "e mais 1"; new safe format, no concrete roadmap command
	doneMatches := []string{
		"ROADMAP-delta.md",
		"ROADMAP-alpha.md",
		"ROADMAP-gamma.md",
		"ROADMAP-beta.md",
	}
	msg = BranchGovernanceOrientationForCreation(branch, cfg, doneMatches)
	// affirms: hint announces that concluded roadmaps do not govern a new branch
	if !strings.Contains(msg, "do not govern a new branch") {
		t.Errorf("expected 'do not govern a new branch' in hint, got: %s", msg)
	}
	// affirms: overflow summarised correctly
	if !strings.Contains(msg, "e mais 1") {
		t.Errorf("expected 'e mais 1' for 4 matches, got: %s", msg)
	}
	// sorted order: alpha < beta < delta < gamma; first 3: alpha, beta, delta; gamma is elided
	if !strings.Contains(msg, "ROADMAP-alpha.md") || !strings.Contains(msg, "ROADMAP-beta.md") || !strings.Contains(msg, "ROADMAP-delta.md") {
		t.Errorf("expected first 3 sorted names (alpha, beta, delta), got: %s", msg)
	}
	if strings.Contains(msg, "ROADMAP-gamma.md") {
		t.Errorf("4th match (gamma) must be elided, got: %s", msg)
	}
	// affirms: hint uses literal <name> placeholder, not a ready-to-run command with a concrete roadmap name
	if !strings.Contains(msg, "trackfw roadmap move <name> wip") {
		t.Errorf("hint must include literal '<name>' placeholder, got: %s", msg)
	}
	// affirms: no concrete roadmap command that could direct an agent to reopen the wrong roadmap
	if strings.Contains(msg, "trackfw roadmap move ROADMAP-") {
		t.Errorf("hint must NOT contain ready-to-run command with concrete roadmap name, got: %s", msg)
	}
}
