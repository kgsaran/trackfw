package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rulesMarker mirrors generators.rulesStart, which is unexported. It is
// duplicated here (rather than exported) because these tests only need to
// prove the marker's presence/count, not its content, and the tests
// documented below regress if the two ever drift.
const rulesMarker = "<!-- trackfw:rules:start -->"

// catalogRulesTarget maps a catalog target ID to the auxiliary rules file it
// must produce via generators.InjectRulesForTool, plus a header snippet
// unique to that tool. This is exactly the set of targets affected by the
// ML-5E regression: the catalog-based install path (`trackfw agents|skills
// install --targets <tool>`) built agents/skills from the catalog but never
// created these standalone rules files in a brand-new project.
type catalogRulesTarget struct {
	tool         string
	relPath      string
	headerSubstr string
}

var catalogRulesTargets = []catalogRulesTarget{
	{tool: "gemini", relPath: "GEMINI.md", headerSubstr: "# Project Instructions"},
	{tool: "copilot", relPath: filepath.Join(".github", "copilot-instructions.md"), headerSubstr: "# GitHub Copilot Instructions"},
	{tool: "windsurf", relPath: ".windsurfrules", headerSubstr: "# Windsurf Rules"},
	{tool: "amazonq", relPath: filepath.Join(".amazonq", "developer", "guidelines.md"), headerSubstr: "# Amazon Q Developer Guidelines"},
}

// TestAgentsInstallCreatesRulesFileFromEmptyProject proves the regression
// fix from an empty project (t.TempDir()): a project that starts with none
// of the four auxiliary rules files must have them created by `trackfw
// agents install --targets <tool>`, the canonical catalog-based install
// path. A test starting from a project where the file already exists would
// not prove this — that was exactly the gap that let the regression through
// (see handoff for ML-5E of ROADMAP-2026-07-29-barrier-governanca-e-
// autoridade-do-orquestrador).
func TestAgentsInstallCreatesRulesFileFromEmptyProject(t *testing.T) {
	for _, target := range catalogRulesTargets {
		target := target
		t.Run(target.tool, func(t *testing.T) {
			project, _ := integrationCommandFixture(t)

			destination := filepath.Join(project, target.relPath)
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatalf("precondition failed: %s must not exist before install", destination)
			}

			cmd := newAgentsCmd()
			cmd.SetArgs([]string{"install", "--targets", target.tool, "--scope", "project"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("agents install --targets %s failed: %v", target.tool, err)
			}

			data, err := os.ReadFile(destination)
			if err != nil {
				t.Fatalf("expected rules file %s to be created, got: %v", destination, err)
			}
			content := string(data)
			if !strings.Contains(content, target.headerSubstr) {
				t.Errorf("rules file %s missing header %q:\n%s", destination, target.headerSubstr, content)
			}
			if !strings.Contains(content, rulesMarker) {
				t.Errorf("rules file %s missing trackfw rules marker:\n%s", destination, content)
			}
		})
	}
}

// TestAgentsInstallRulesFileIdempotent proves that installing the same
// target twice does not duplicate the trackfw rules block — the existing
// injectOrUpdateRules mechanism must be reused, not reimplemented, and this
// is the test that would catch a reimplementation that regresses
// idempotency.
func TestAgentsInstallRulesFileIdempotent(t *testing.T) {
	for _, target := range catalogRulesTargets {
		target := target
		t.Run(target.tool, func(t *testing.T) {
			project, _ := integrationCommandFixture(t)
			destination := filepath.Join(project, target.relPath)

			for i := 0; i < 2; i++ {
				cmd := newAgentsCmd()
				cmd.SetArgs([]string{"install", "--targets", target.tool, "--scope", "project"})
				if err := cmd.Execute(); err != nil {
					t.Fatalf("run %d: agents install --targets %s failed: %v", i+1, target.tool, err)
				}
			}

			data, err := os.ReadFile(destination)
			if err != nil {
				t.Fatalf("expected rules file %s to exist: %v", destination, err)
			}
			content := string(data)
			if count := strings.Count(content, rulesMarker); count != 1 {
				t.Fatalf("expected exactly 1 trackfw rules block in %s after 2 installs, found %d:\n%s", destination, count, content)
			}
		})
	}
}

// TestSkillsInstallAlsoCreatesRulesFile proves the fix applies to `trackfw
// skills install` too, not just `trackfw agents install` — both commands
// share executeIntegrationMutation and a user may run either one first.
func TestSkillsInstallAlsoCreatesRulesFile(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	destination := filepath.Join(project, "GEMINI.md")

	cmd := newSkillsCmd()
	cmd.SetArgs([]string{"install", "--targets", "gemini", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skills install --targets gemini failed: %v", err)
	}

	if _, err := os.ReadFile(destination); err != nil {
		t.Fatalf("expected %s to be created by skills install: %v", destination, err)
	}
}

// TestAgentsUpdateDoesNotCreateRulesFile guards the deliberate scope
// boundary: rules-file creation is restored only for "install" (mirroring
// the one-shot semantics the removed deprecated aliases had), not
// "update"/"uninstall". A project with no rules file that only ever runs
// `update` should not have one materialize as a side effect of update.
func TestAgentsUpdateDoesNotCreateRulesFile(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	destination := filepath.Join(project, "GEMINI.md")

	cmd := newAgentsCmd()
	cmd.SetArgs([]string{"update", "--targets", "gemini", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agents update --targets gemini failed: %v", err)
	}

	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("update must not create %s (install-only behavior)", destination)
	}
}

// TestInitAiToolsFlagCreatesRulesFile proves the second catalog-based call
// site — `trackfw init --ai-tools <tool>` (internal/commands/init.go
// installAITools) — got the same fix as `trackfw agents install`. Both
// paths call integrations.Manager.Install independently; fixing only one
// would leave `init` regressed.
func TestInitAiToolsFlagCreatesRulesFile(t *testing.T) {
	project, _ := initFixture(t)
	destination := filepath.Join(project, "GEMINI.md")

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "gemini", "--identity-preset", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --ai-tools gemini failed: %v", err)
	}

	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("expected %s to be created by init --ai-tools: %v", destination, err)
	}
	if !strings.Contains(string(data), rulesMarker) {
		t.Errorf("expected trackfw rules marker in %s:\n%s", destination, data)
	}
}

// TestCursorDirectoryTriggerBehaviorUnchanged proves the existing
// directory-triggered Cursor behavior (InjectRulesDetected: rules injected
// whenever .cursor/ exists, even without an explicit AI-tools selection) did
// not regress. This is guarded separately from the four fixed targets
// because the fix must not alter this pre-existing mechanism.
func TestCursorDirectoryTriggerBehaviorUnchanged(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	if err := os.MkdirAll(filepath.Join(project, ".cursor"), 0755); err != nil {
		t.Fatal(err)
	}

	// discover --init is the existing entrypoint for InjectRulesDetected; a
	// plain discover run (no AI tool explicitly requested) must still create
	// the Cursor rules file purely because .cursor/ exists, exactly as
	// before this fix.
	discover := NewDiscoverCmd()
	discover.SetArgs([]string{"--init"})
	discover.SetOut(&strings.Builder{})
	if err := discover.Execute(); err != nil {
		t.Fatalf("discover --init failed: %v", err)
	}

	destination := filepath.Join(project, ".cursor", "rules", "trackfw.mdc")
	if _, err := os.ReadFile(destination); err != nil {
		t.Fatalf("expected Cursor directory-trigger to still create %s: %v", destination, err)
	}
}
