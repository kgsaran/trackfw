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
	"runtime"
	"strings"
	"testing"
	"time"

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

	// Step 2: adr new — record date before the call so the expected filename uses the same day.
	adrDate := time.Now().Format("2006-01-02")
	if err := runCmd(t, "adr", "new", "User Authentication Strategy"); err != nil {
		t.Fatalf("adr new: %v", err)
	}

	// Verify the expected ADR basename matches the README's `ADR-$(date +%F)-user-authentication-strategy.md`.
	adrBasename := "ADR-" + adrDate + "-user-authentication-strategy.md"
	if _, err := os.Stat(filepath.Join("docs", "adr", adrBasename)); err != nil {
		t.Fatalf("expected ADR file %s to exist after adr new: %v", adrBasename, err)
	}

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

	// Determine the REQ basename (REQ-<date>-user-authentication.md).
	// Same date as adrDate — both files are created during the same test run.
	reqBasename := "REQ-" + adrDate + "-user-authentication.md"

	// Step 5: validate — must pass with 0 violations and exactly 2 warnings,
	// both adr_req_has_content: the freshly-created ADR and REQ contain only
	// model/template content (no user-supplied text yet). Any other warning or
	// any violation is a regression.
	//
	// Rule reconciliation (Regra Dura de Reconciliação): this test affirms that
	// freshly-scaffolded ADR and REQ files both trigger adr_req_has_content warnings,
	// one per file, naming the specific file — which is the conclusion from ML-1A
	// (rule fires on template content, is silent on filled content, names the file).
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
