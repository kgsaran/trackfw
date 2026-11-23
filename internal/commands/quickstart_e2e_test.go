package commands

// TestQuickStartEndToEnd exercises the Quick Start sequence described in the README:
//
//	trackfw init --ai-tools claude --forge github
//	trackfw adr new "User Authentication Strategy"
//	trackfw req new "User Authentication" --adr ADR-<date>-user-authentication-strategy.md
//	trackfw roadmap move ROADMAP-<date>-user-authentication.md wip
//	trackfw validate   →  0 violations, 2 warning(s)
//
// The sequence is derived from quickStartLines (the single-list authority for both
// the execution and the document pin). $(date +%F) is substituted with the actual
// date before tokenising.
//
// The sequence is run in-process inside a t.TempDir() with HOME and XDG_CONFIG_HOME
// redirected so init writes are contained (not serialisable: no t.Parallel).
//
// Counter-arms:
//   - TestQuickStartEndToEnd_CounterArm_NoADRFlagFails: same sequence without --adr
//     → validate emits req_has_adr violation.
//   - TestQuickStartDocsPinned_CounterArm_README: modified README copy → run check fails.
//   - TestQuickStartDocsPinned_CounterArm_SiteGuide: modified site copy → run check fails.
//
// Both execution tests assert: AC4 — Quick start passes end-to-end with --adr;
//
//	without --adr, req_has_adr fires.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// ── Single-list authority (ADR D4) ────────────────────────────────────────────
//
// quickStartLines is the single source of truth for the Quick start commands.
// Every entry is the literal line as it appears in README.md and in
// site/guide/getting-started.md. Two uses:
//  1. TestQuickStartDocsPinned checks that these lines appear as a contiguous
//     block in both documents (after stripping inline shell comments).
//  2. TestQuickStartEndToEnd derives its runCmd arguments by tokenising each
//     entry and substituting $(date +%F) with the actual date.
//
// When quickStartLines changes, both the document pin and the execution change.
var quickStartLines = []string{
	`trackfw init --ai-tools claude --forge github`,
	`trackfw adr new "User Authentication Strategy"`,
	`trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md`,
	`trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip`,
	`trackfw validate`,
}

// repoRoot returns the repository root by walking up from this source file's
// location (internal/commands/ → ../../). Safe when -trimpath is not set;
// this project's Makefile and CI do not set -trimpath.
func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller(0) failed — cannot determine repo root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

// tokenizeQuickStartLine expands $(date +%F) to date, then splits the resulting
// string into shell tokens, handling double-quoted strings. The first token must
// be "trackfw"; it is stripped. Returns an error on unsupported shell syntax
// (backslash, single quotes, or $( other than $(date +%F)) so that a future doc
// edit with unsupported syntax fails loudly instead of splitting wrong.
func tokenizeQuickStartLine(line, date string) ([]string, error) {
	expanded := strings.ReplaceAll(line, "$(date +%F)", date)
	if strings.ContainsAny(expanded, "\\'") {
		return nil, fmt.Errorf("unsupported shell syntax (backslash or single quote) in %q", line)
	}
	if strings.Contains(expanded, "$(") {
		return nil, fmt.Errorf("unsupported shell expansion in %q", line)
	}
	var tokens []string
	var cur strings.Builder
	inQuote := false
	for _, r := range expanded {
		switch {
		case r == '"' && !inQuote:
			inQuote = true
		case r == '"' && inQuote:
			inQuote = false
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	if len(tokens) == 0 || tokens[0] != "trackfw" {
		return nil, fmt.Errorf("expected 'trackfw <cmd> ...', got %v from %q", tokens, line)
	}
	return tokens[1:], nil // strip "trackfw"
}

// normalizeDocLine strips leading/trailing whitespace and an inline shell comment
// (a " #..." suffix). This lets "trackfw validate   # 0 violations, 2 warning(s)..."
// compare equal to the bare "trackfw validate" in quickStartLines.
func normalizeDocLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if idx := strings.Index(trimmed, " #"); idx >= 0 {
		trimmed = strings.TrimRight(trimmed[:idx], " \t")
	}
	return trimmed
}

// findQuickStartRun scans docLines for the five quickStartLines appearing as a
// consecutive block (each document line normalised with normalizeDocLine).
// Returns the start index, or -1 if not found.
func findQuickStartRun(docLines []string) int {
	n := len(quickStartLines)
	for i := 0; i <= len(docLines)-n; i++ {
		match := true
		for j, want := range quickStartLines {
			if normalizeDocLine(docLines[i+j]) != want {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// docChecker is the minimal testing.T subset used by assertDocContainsRun,
// allowing counter-arm tests to pass a recorder that captures failures without
// propagating them to the real *testing.T.
type docChecker interface {
	Helper()
	Errorf(string, ...interface{})
}

// assertDocContainsRun reads the file at path and fails (via dc) if the Quick
// start block is not found as a contiguous run of normalised lines.
func assertDocContainsRun(dc docChecker, path string) {
	dc.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		dc.Errorf("assertDocContainsRun: read %s: %v", path, err)
		return
	}
	lines := strings.Split(string(content), "\n")
	if findQuickStartRun(lines) < 0 {
		dc.Errorf("document %s: Quick start commands not found as a contiguous block\n  want (each normalised):\n    %s",
			filepath.Base(path), strings.Join(quickStartLines, "\n    "))
	}
}

// failRecorder implements docChecker for counter-arm tests: it records whether
// Errorf was called without propagating failures to the real *testing.T.
type failRecorder struct {
	failed bool
}

func (r *failRecorder) Helper() {}
func (r *failRecorder) Errorf(_ string, _ ...interface{}) { r.failed = true }

// ── Core helpers ───────────────────────────────────────────────────────────────

// quickstartSetup creates an isolated project root and HOME, chdirs into the project,
// calls config.Reset(), and registers cleanup. Returns (projectDir, homeDir).
// NOT safe for t.Parallel() — cwd is process-global.
func quickstartSetup(t *testing.T) (projectDir, homeDir string) {
	t.Helper()
	projectDir = t.TempDir()
	homeDir = t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", homeDir)
	}

	// Ensure the trackfw binary is on PATH so that the validator's guardLookupBinary
	// probe (guardBinaryProbeOnce) succeeds in CI environments where trackfw is not
	// installed. guardBinary compiles ./cmd/trackfw once per test run; the resulting
	// binary supports `trackfw guard --help` (exit 0), which is exactly what the probe
	// needs. We prepend the binary's directory so it is found before any system PATH.
	bin := guardBinary(t)
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))

	config.Reset()

	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})
	return projectDir, homeDir
}

// runCmd creates a fresh root command for each invocation (cobra flag state is per-instance)
// and executes it with the given arguments. Returns the error without printing it.
func runCmd(t *testing.T, args ...string) error {
	t.Helper()
	root := newRootCmd()
	root.SilenceErrors = true
	root.SetArgs(args)
	return root.Execute()
}

// ── Tests ──────────────────────────────────────────────────────────────────────

// TestQuickStartEndToEnd — the complete Quick Start sequence ends with 0 violations.
// Args are derived from quickStartLines (the single-list authority); $(date +%F) is
// substituted with the actual date before tokenising.
// Asserts: AC4 — req new --adr links the ADR so that validate passes without req_has_adr.
func TestQuickStartEndToEnd(t *testing.T) {
	quickstartSetup(t)

	// quickStartLines is the authority for the execution below AND for the document
	// pin in TestQuickStartDocsPinned. Changing the doc lines without updating
	// quickStartLines causes TestQuickStartDocsPinned to fail; changing quickStartLines
	// without updating the docs causes TestQuickStartDocsPinned to fail too.
	date := time.Now().Format("2006-01-02")

	mustTokenize := func(idx int) []string {
		t.Helper()
		args, err := tokenizeQuickStartLine(quickStartLines[idx], date)
		if err != nil {
			t.Fatalf("tokenize quickStartLines[%d]: %v", idx, err)
		}
		return args
	}

	// Step 1: init — derived from quickStartLines[0]
	if err := runCmd(t, mustTokenize(0)...); err != nil {
		t.Fatalf("init: %v", err)
	}
	// config.Reset() after init so subsequent commands read the just-created trackfw.yaml.
	config.Reset()

	// Step 2: adr new — derived from quickStartLines[1]
	if err := runCmd(t, mustTokenize(1)...); err != nil {
		t.Fatalf("adr new: %v", err)
	}

	// Verify the expected ADR basename matches the README's `ADR-$(date +%F)-user-authentication-strategy.md`.
	adrBasename := "ADR-" + date + "-user-authentication-strategy.md"
	if _, err := os.Stat(filepath.Join("docs", "adr", adrBasename)); err != nil {
		t.Fatalf("expected ADR file %s to exist after adr new: %v", adrBasename, err)
	}

	// Step 3: req new --adr — derived from quickStartLines[2]
	if err := runCmd(t, mustTokenize(2)...); err != nil {
		t.Fatalf("req new --adr: %v", err)
	}

	// Step 4: roadmap move wip — derived from quickStartLines[3]
	roadmapArgs := mustTokenize(3) // ["roadmap", "move", "ROADMAP-<date>-user-authentication.md", "wip"]
	roadmapBasenameDerived := roadmapArgs[2]

	// Cross-check: glob to verify the file req new created matches the name the doc predicts.
	roadmapMatches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "ROADMAP-*.md"))
	if err != nil || len(roadmapMatches) == 0 {
		t.Fatalf("expected roadmap after req new, found %d (err=%v)", len(roadmapMatches), err)
	}
	if got := filepath.Base(roadmapMatches[0]); got != roadmapBasenameDerived {
		t.Errorf("roadmap name mismatch: quickStartLines predicts %s, req new created %s",
			roadmapBasenameDerived, got)
	}

	if err := runCmd(t, roadmapArgs...); err != nil {
		t.Fatalf("roadmap move wip: %v", err)
	}

	// Determine the REQ basename (REQ-<date>-user-authentication.md).
	reqBasename := "REQ-" + date + "-user-authentication.md"

	// Step 5: validate — quickStartLines[4] = "trackfw validate"
	// Executed in-process via ValidateTagged for precise assertion of violations/warnings.
	// Rule reconciliation (Regra Dura de Reconciliação): this test affirms that
	// freshly-scaffolded ADR and REQ files both trigger adr_req_has_content warnings,
	// one per file, naming the specific file — which is the conclusion from ML-1A
	// (rule fires on template content, is silent on filled content, names the file).
	validateArgs := mustTokenize(4)
	if len(validateArgs) != 1 || validateArgs[0] != "validate" {
		t.Fatalf("quickStartLines[4]: expected [validate], got %v (the doc line must be bare 'trackfw validate')", validateArgs)
	}

	violations, warnings, err := validator.ValidateTagged()
	if err != nil {
		t.Fatalf("ValidateTagged error: %v", err)
	}
	if len(violations) != 0 {
		msgs := make([]string, len(violations))
		for i, v := range violations {
			msgs[i] = v.Rule + ": " + v.Msg
		}
		t.Errorf("expected 0 violations after quick start with --adr, got %d:\n  %s",
			len(violations), strings.Join(msgs, "\n  "))
	}
	// Exactly 2 adr_req_has_content warnings — one naming the ADR, one naming the REQ.
	// Two warnings for the same file, or a warning for an unexpected file, is a regression.
	const wantWarnRule = "adr_req_has_content"
	if len(warnings) != 2 {
		wmsgs := make([]string, len(warnings))
		for i, w := range warnings {
			wmsgs[i] = w.Rule + ": " + w.Msg
		}
		t.Errorf("expected exactly 2 %s warnings (one for ADR, one for REQ), got %d:\n  %s",
			wantWarnRule, len(warnings), strings.Join(wmsgs, "\n  "))
	}
	for _, w := range warnings {
		if w.Rule != wantWarnRule {
			t.Errorf("unexpected warning rule %q (want %q): %s", w.Rule, wantWarnRule, w.Msg)
		}
	}
	// Each expected basename must appear in exactly one warning message.
	for _, basename := range []string{adrBasename, reqBasename} {
		var hits int
		for _, w := range warnings {
			if strings.Contains(w.Msg, basename) {
				hits++
			}
		}
		if hits != 1 {
			wmsgs := make([]string, len(warnings))
			for i, w := range warnings {
				wmsgs[i] = w.Msg
			}
			t.Errorf("expected exactly 1 warning mentioning %q, got %d:\n  %s",
				basename, hits, strings.Join(wmsgs, "\n  "))
		}
	}
}

// TestQuickStartEndToEnd_CounterArm_NoADRFlagFails — the same sequence without --adr
// leaves the REQ unlinked and validate fires req_has_adr for the created REQ.
// Asserts: AC4 counter-arm — --adr is necessary for validate to pass; omitting it fails.
func TestQuickStartEndToEnd_CounterArm_NoADRFlagFails(t *testing.T) {
	quickstartSetup(t)

	if err := runCmd(t, "init", "--ai-tools", "claude", "--forge", "github"); err != nil {
		t.Fatalf("init: %v", err)
	}
	config.Reset()

	if err := runCmd(t, "adr", "new", "User Authentication Strategy"); err != nil {
		t.Fatalf("adr new: %v", err)
	}

	// req new WITHOUT --adr
	if err := runCmd(t, "req", "new", "User Authentication"); err != nil {
		t.Fatalf("req new (no --adr): %v", err)
	}

	roadmapMatches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "ROADMAP-*.md"))
	if err != nil || len(roadmapMatches) == 0 {
		t.Fatalf("expected roadmap, found %d (err=%v)", len(roadmapMatches), err)
	}
	if err := runCmd(t, "roadmap", "move", filepath.Base(roadmapMatches[0]), "wip"); err != nil {
		t.Fatalf("roadmap move wip: %v", err)
	}

	// validate must report req_has_adr for the created REQ.
	reqMatches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(reqMatches) == 0 {
		t.Fatal("no REQ file found")
	}
	reqBase := filepath.Base(reqMatches[0])

	violations, warnings, err := validator.ValidateTagged()
	if err != nil {
		t.Fatalf("ValidateTagged: %v", err)
	}
	found := false
	all := append(append([]validator.TaggedMsg{}, violations...), warnings...)
	for _, m := range all {
		if m.Rule == "req_has_adr" && strings.Contains(m.Msg, reqBase) {
			found = true
			break
		}
	}
	if !found {
		msgs := make([]string, len(all))
		for i, m := range all {
			msgs[i] = m.Rule + ": " + m.Msg
		}
		t.Errorf("expected req_has_adr violation for %s without --adr, got:\n  %s",
			reqBase, strings.Join(msgs, "\n  "))
	}
}

// ── ADR D4 — doc-pinning tests ─────────────────────────────────────────────────

// assertSiteGuideHas2Warnings reads the site guide at path, finds the Quick start
// run block, then checks that the output fence immediately after it contains a line
// whose trimmed text equals "2 warning(s)".
//
// Using a trimmed-line equality (not strings.Contains over the whole file) ensures
// that "2 warning(s)" appearing only in prose (lines 62/323) or inside an HTML
// comment does not cause a false pass when the actual output fence is wrong (F-8,
// ML-4C).
//
// Uses dc for reporting so counter-arm tests can capture failures without
// propagating to the real *testing.T.
func assertSiteGuideHas2Warnings(dc docChecker, path string) {
	dc.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		dc.Errorf("assertSiteGuideHas2Warnings: read %s: %v", filepath.Base(path), err)
		return
	}
	lines := strings.Split(string(content), "\n")
	runIdx := findQuickStartRun(lines)
	if runIdx < 0 {
		dc.Errorf("assertSiteGuideHas2Warnings: Quick start run not found in %s — cannot check output fence",
			filepath.Base(path))
		return
	}
	// Scan forward from after the last command line.
	// The first ``` encountered is the closing fence of the command block; skip it.
	// The second ``` opens the output block; scan inside it for "2 warning(s)".
	skipFences := 1 // closing fence of the command block
	inFence := false
	found := false
	for li := runIdx + len(quickStartLines); li < len(lines); li++ {
		line := lines[li]
		if strings.HasPrefix(line, "```") {
			if skipFences > 0 {
				skipFences--
				continue
			}
			if !inFence {
				inFence = true
				continue
			}
			break // end of output fence; stop
		}
		if inFence && strings.TrimSpace(line) == "2 warning(s)" {
			found = true
		}
	}
	if !found {
		dc.Errorf("%s: output fence after Quick start run must contain a line \"2 warning(s)\""+
			" (F-8, ML-4C: scoped to output fence, not the whole page — prose references do not count)",
			filepath.Base(path))
	}
}

// TestQuickStartDocsPinned — ADR D4: each Quick start command line appears as a
// contiguous block in README.md and in site/guide/getting-started.md; the README
// validate line promises "2 warning(s)" and does not promise "✓ No violations found.";
// the site guide contains "2 warning(s)" inside the output fence that immediately
// follows the Quick start command fence (F-8, ML-4C: scoped to the fence, not the page).
//
// Asserts (ML-3A): documentation matches the commands the binary executes; a doc
// edit without a matching test edit is detected at test time.
func TestQuickStartDocsPinned(t *testing.T) {
	root := repoRoot()

	// --- README ---
	readmeBytes, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readmeLines := strings.Split(string(readmeBytes), "\n")
	readmeIdx := findQuickStartRun(readmeLines)
	if readmeIdx < 0 {
		t.Errorf("README.md: Quick start block not found as contiguous run\n  want (each normalised):\n    %s",
			strings.Join(quickStartLines, "\n    "))
	} else {
		// The validate line (run[4]) must promise the right output.
		// The README uses an inline comment on the validate line itself.
		validateLine := readmeLines[readmeIdx+len(quickStartLines)-1]
		if !strings.Contains(validateLine, "2 warning(s)") {
			t.Errorf("README.md: validate line must contain \"2 warning(s)\", got:\n  %s", validateLine)
		}
		if strings.Contains(validateLine, "✓ No violations found.") {
			t.Errorf("README.md: validate line must not contain \"✓ No violations found.\", got:\n  %s", validateLine)
		}
	}

	// --- Site guide ---
	siteBytes, err := os.ReadFile(filepath.Join(root, "site", "guide", "getting-started.md"))
	if err != nil {
		t.Fatalf("read site/guide/getting-started.md: %v", err)
	}
	siteLines := strings.Split(string(siteBytes), "\n")
	siteRunIdx := findQuickStartRun(siteLines)
	if siteRunIdx < 0 {
		t.Errorf("site/guide/getting-started.md: Quick start block not found as contiguous run\n  want (each normalised):\n    %s",
			strings.Join(quickStartLines, "\n    "))
	}
	// F-8 (ML-4C): "2 warning(s)" must appear inside the output fence immediately
	// after the command block, not just anywhere in the page.
	assertSiteGuideHas2Warnings(t, filepath.Join(root, "site", "guide", "getting-started.md"))
}

// TestQuickStartDocsPinned_CounterArm_README — removing or editing a Quick start
// line in README makes TestQuickStartDocsPinned fail.
//
// Asserts (ML-3A counter-arm): the doc pin is not vacuous for README; a stale
// README is detected even when the same line appears elsewhere in the file.
func TestQuickStartDocsPinned_CounterArm_README(t *testing.T) {
	root := repoRoot()
	src, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	// Find the run to edit the first line by index (not by ReplaceAll, which
	// could also replace duplicates outside the Quick start block).
	idx := findQuickStartRun(lines)
	if idx < 0 {
		t.Fatal("counter-arm setup: Quick start run not found in README.md — test fixture is broken")
	}
	lines[idx] = strings.ReplaceAll(lines[idx], "--forge github", "--forge gitlab")

	tmpFile := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(tmpFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write modified README to temp: %v", err)
	}

	rec := &failRecorder{}
	assertDocContainsRun(rec, tmpFile)
	if !rec.failed {
		t.Errorf("counter-arm: modified README.md passed the run check — the doc pin is not detecting changes")
	}
}

// TestQuickStartDocsPinned_CounterArm_SiteGuide — same as the README counter-arm
// but for site/guide/getting-started.md, which has a second occurrence of the init
// line (line 89) outside the Quick start block. The contiguous-run check correctly
// rejects a copy where only the block's init line is changed.
//
// Asserts (ML-3A counter-arm): the doc pin is not vacuous for the site guide;
// contiguous-run matching avoids false passes from repeated lines elsewhere.
func TestQuickStartDocsPinned_CounterArm_SiteGuide(t *testing.T) {
	root := repoRoot()
	src, err := os.ReadFile(filepath.Join(root, "site", "guide", "getting-started.md"))
	if err != nil {
		t.Fatalf("read site/guide/getting-started.md: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	// Find the Quick start block and edit its first line by index.
	// The site guide has a second "--forge github" at line 89 (1-indexed); editing
	// only the block's copy proves the contiguous-run check catches it despite the duplicate.
	idx := findQuickStartRun(lines)
	if idx < 0 {
		t.Fatal("counter-arm setup: Quick start run not found in site guide — test fixture is broken")
	}
	lines[idx] = strings.ReplaceAll(lines[idx], "--forge github", "--forge gitlab")

	tmpFile := filepath.Join(t.TempDir(), "getting-started.md")
	if err := os.WriteFile(tmpFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write modified site guide to temp: %v", err)
	}

	rec := &failRecorder{}
	assertDocContainsRun(rec, tmpFile)
	if !rec.failed {
		t.Errorf("counter-arm: modified site guide passed the run check — the doc pin is not detecting changes")
	}
}

// TestQuickStartDocsPinned_CounterArm_SiteGuideOutputFence — replacing "2 warning(s)"
// in the output fence with "0 warning(s)" while leaving prose intact must cause
// assertSiteGuideHas2Warnings to fail.
//
// This counter-arm specifically tests the F-8 fix (ML-4C): the old check used
// strings.Contains over the whole page, so any surviving "2 warning(s)" in prose
// (lines 62/323) would cause a false pass. The new fence-scoped check catches it.
//
// Asserts (F-8 counter-arm, ML-4C): the output-fence scope is not vacuous; prose
// references at lines 62/323 do not satisfy the check when the fence is wrong.
func TestQuickStartDocsPinned_CounterArm_SiteGuideOutputFence(t *testing.T) {
	root := repoRoot()
	src, err := os.ReadFile(filepath.Join(root, "site", "guide", "getting-started.md"))
	if err != nil {
		t.Fatalf("read site/guide/getting-started.md: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	// Find the Quick start run and then the output fence (second ``` after the run).
	runIdx := findQuickStartRun(lines)
	if runIdx < 0 {
		t.Fatal("counter-arm setup: Quick start run not found in site guide — fixture is broken")
	}

	// Locate the output fence boundaries using the same logic as assertSiteGuideHas2Warnings.
	skipFences := 1
	inFence := false
	fenceStart := -1
	fenceEnd := -1
	for li := runIdx + len(quickStartLines); li < len(lines); li++ {
		line := lines[li]
		if strings.HasPrefix(line, "```") {
			if skipFences > 0 {
				skipFences--
				continue
			}
			if !inFence {
				inFence = true
				fenceStart = li
				continue
			}
			fenceEnd = li
			break
		}
	}
	if fenceStart < 0 || fenceEnd < 0 {
		t.Fatal("counter-arm setup: output fence not found after Quick start run")
	}

	// Replace "2 warning(s)" with "0 warning(s)" only inside the output fence.
	for li := fenceStart; li <= fenceEnd; li++ {
		if strings.TrimSpace(lines[li]) == "2 warning(s)" {
			lines[li] = "0 warning(s)"
		}
	}

	// Verify "2 warning(s)" still appears in prose (line 62-ish) so the counter-arm
	// isolates the fence-scope failure rather than a total absence.
	proseFound := false
	for _, l := range lines {
		if strings.Contains(l, "2 warning(s)") && !strings.HasPrefix(l, "```") {
			proseFound = true
			break
		}
	}
	if !proseFound {
		t.Fatal("counter-arm setup: '2 warning(s)' not found in prose after editing fence — adjust fixture")
	}

	tmpFile := filepath.Join(t.TempDir(), "getting-started.md")
	if err := os.WriteFile(tmpFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write modified site guide to temp: %v", err)
	}

	rec := &failRecorder{}
	assertSiteGuideHas2Warnings(rec, tmpFile)
	if !rec.failed {
		t.Errorf("counter-arm: modified site guide (output fence says '0 warning(s)') passed the fence check — F-8 fix is not detecting the wrong output")
	}
}
