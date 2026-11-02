package commands

// TestQuickStartEndToEnd exercises the Quick Start sequence described in the README:
//
//	trackfw init --ai-tools claude --forge github
//	trackfw adr new "User Authentication Strategy"
//	trackfw req new "User Authentication" --adr ADR-<date>-user-authentication-strategy.md
//	trackfw roadmap move ROADMAP-<date>-user-authentication.md wip
//	trackfw validate   →  0 violations
//
// The sequence is run in-process inside a t.TempDir() with HOME and XDG_CONFIG_HOME
// redirected so init writes are contained (not serializable: no t.Parallel).
//
// Counter-arm (TestQuickStartEndToEnd_CounterArm_NoADRFlagFails):
//
//	same sequence but without --adr → validate emits req_has_adr violation.
//
// Both tests assert: AC4 — Quick start passes end-to-end with --adr;
//
//	without --adr, req_has_adr fires.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

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

// TestQuickStartEndToEnd — the complete Quick Start sequence ends with 0 violations.
// Asserts: AC4 — req new --adr links the ADR so that validate passes without req_has_adr.
func TestQuickStartEndToEnd(t *testing.T) {
	quickstartSetup(t)

	// Step 1: init
	if err := runCmd(t, "init", "--ai-tools", "claude", "--forge", "github"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// config.Reset() after init so subsequent commands read the just-created trackfw.yaml.
	config.Reset()

	// Step 2: adr new
	if err := runCmd(t, "adr", "new", "User Authentication Strategy"); err != nil {
		t.Fatalf("adr new: %v", err)
	}

	// Discover the ADR basename created in this test run.
	adrMatches, err := filepath.Glob(filepath.Join("docs", "adr", "ADR-*.md"))
	if err != nil || len(adrMatches) == 0 {
		t.Fatalf("expected ADR file after adr new, found %d (err=%v)", len(adrMatches), err)
	}
	adrBasename := filepath.Base(adrMatches[0])

	// Step 3: req new --adr
	if err := runCmd(t, "req", "new", "User Authentication", "--adr", adrBasename); err != nil {
		t.Fatalf("req new --adr: %v", err)
	}

	// Discover the roadmap basename.
	roadmapMatches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "ROADMAP-*.md"))
	if err != nil || len(roadmapMatches) == 0 {
		t.Fatalf("expected roadmap after req new, found %d (err=%v)", len(roadmapMatches), err)
	}
	roadmapBasename := filepath.Base(roadmapMatches[0])

	// Step 4: roadmap move wip
	if err := runCmd(t, "roadmap", "move", roadmapBasename, "wip"); err != nil {
		t.Fatalf("roadmap move wip: %v", err)
	}

	// Step 5: validate — must pass with 0 violations.
	violations, _, err := validator.ValidateTagged()
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
