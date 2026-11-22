package commands

// Tests for ADR-2026-11-22 D1 and D2 (ML-1A, REQ-2026-11-22).
//
// Rule reconciliation (Regra Dura de Reconciliação, CLAUDE.md):
//
//   TestInitD1_NonTTY_BrownfieldNewProject
//     Affirms: D1 — --brownfield is read on the non-TTY path and sets
//     governance_mode: lenient + lenient_until on a new project (no prior trackfw.yaml).
//
//   TestInitD2_NonTTY_BrownfieldExistingYAML_GovernanceUnchanged
//     Affirms: D2 — when trackfw.yaml is already present, --brownfield does not write
//     governance_mode or lenient_until; the warning is printed; rc=0; agents are installed.
//
//   TestInitD1_TTY_AIToolsFlag_SkipsWizard
//     Affirms: D1 TTY path — when --ai-tools is explicitly passed, the AI-tools wizard
//     group is hidden (WithHideFunc) and agents are installed from the flag value even
//     though the form runner is a no-op (aiTools would otherwise remain empty).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// initFlagsFixture sets up an isolated project root and HOME for init flag tests,
// extending initFixture with CLAUDE_CONFIG_DIR isolation (ADR-2026-11-22, D2).
// NOT safe for t.Parallel() — cwd is process-global.
func initFlagsFixture(t *testing.T) (project, home string) {
	t.Helper()
	project, home = initFixture(t)

	// Also isolate CLAUDE_CONFIG_DIR so writes go to the temp HOME, not the
	// developer's real ~/.claude (memory: codex-home-env-isolation applies to
	// all agent config dirs).
	oldClaudeConfigDir, hadClaudeConfigDir := os.LookupEnv("CLAUDE_CONFIG_DIR")
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadClaudeConfigDir {
			_ = os.Setenv("CLAUDE_CONFIG_DIR", oldClaudeConfigDir)
		} else {
			_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
		}
	})

	config.Reset()
	t.Cleanup(config.Reset)
	return project, home
}

// runCmdCaptureOut creates a fresh root command, redirects its output writer to
// a buffer, executes with the given args, and returns (captured stdout, error).
func runCmdCaptureOut(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	root := newRootCmd()
	root.SilenceErrors = true
	root.SetOut(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// TestInitD1_NonTTY_BrownfieldNewProject — new project + --brownfield non-TTY
// → governance_mode: lenient and lenient_until are written to trackfw.yaml.
//
// Affirms: D1 — --brownfield is read on the non-TTY path and sets
// governance_mode: lenient + lenient_until on a new project (no prior trackfw.yaml).
func TestInitD1_NonTTY_BrownfieldNewProject(t *testing.T) {
	_, _ = initFlagsFixture(t)
	// stdin is never a TTY in go test: initStdinIsTTY() returns false → non-TTY path.

	if err := runCmd(t, "init", "--brownfield"); err != nil {
		t.Fatalf("init --brownfield: %v", err)
	}
	config.Reset()

	data, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		t.Fatalf("reading trackfw.yaml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "governance_mode: lenient") {
		t.Errorf("trackfw.yaml missing governance_mode: lenient:\n%s", content)
	}
	if !strings.Contains(content, "lenient_until:") {
		t.Errorf("trackfw.yaml missing lenient_until:\n%s", content)
	}

	// Also verify via config.Load() that the governance mode is correctly parsed.
	cfg := config.Load()
	if cfg.GovernanceMode != "lenient" {
		t.Errorf("config.GovernanceMode = %q, want %q", cfg.GovernanceMode, "lenient")
	}
}

// TestInitD2_NonTTY_BrownfieldExistingYAML_GovernanceUnchanged — existing
// trackfw.yaml without governance_mode + --brownfield --ai-tools claude non-TTY
// → governance_mode NOT added, warning printed, agents installed, rc=0.
//
// Affirms: D2 — when trackfw.yaml is already present, --brownfield does not write
// governance_mode or lenient_until; the warning is printed; rc=0; agents are installed.
func TestInitD2_NonTTY_BrownfieldExistingYAML_GovernanceUnchanged(t *testing.T) {
	_, home := initFlagsFixture(t)

	// Pre-populate a trackfw.yaml without governance_mode.
	existingYAML := `# trackfw configuration
frontend: ""
backend: go
backend_framework: ""
pkg_manager: ""
hooks: none
ci: none
wip_limit: 1
wip_by_squad: false
require_req_in_commit: false
rules:
  branch_has_wip_roadmap: error
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
`
	if err := os.WriteFile("trackfw.yaml", []byte(existingYAML), 0644); err != nil {
		t.Fatalf("writing pre-existing trackfw.yaml: %v", err)
	}

	out, err := runCmdCaptureOut(t, "init", "--brownfield", "--ai-tools", "claude")
	if err != nil {
		t.Fatalf("init --brownfield --ai-tools claude: %v", err)
	}

	// D2: governance_mode must NOT have been added.
	data, readErr := os.ReadFile("trackfw.yaml")
	if readErr != nil {
		t.Fatalf("reading trackfw.yaml after init: %v", readErr)
	}
	content := string(data)

	govCount := strings.Count(content, "governance_mode")
	if govCount != 0 {
		t.Errorf("governance_mode found %d time(s) in trackfw.yaml after D2 path; want 0:\n%s",
			govCount, content)
	}

	// D2: warning must be printed.
	if !strings.Contains(out, "governance keys not modified") {
		t.Errorf("D2 warning not found in captured output %q", out)
	}

	// rc=0 is implied by no error above.

	// Validate must NOT show LENIENT governance mode.
	config.Reset()
	cfg := config.Load()
	if cfg.GovernanceMode == "lenient" {
		t.Errorf("config.GovernanceMode = %q after D2; must not be lenient (governance not modified)",
			cfg.GovernanceMode)
	}

	// Agents installed: check that the claude config dir exists under HOME.
	claudeDir := filepath.Join(home, ".claude")
	if _, statErr := os.Stat(claudeDir); os.IsNotExist(statErr) {
		t.Errorf("claude dir %s not created — agents were not installed", claudeDir)
	}
}

// TestInitD1_TTY_AIToolsFlag_SkipsWizard — TTY path with --ai-tools claude
// → AI-tools wizard group is hidden, agents are installed from the flag.
//
// Affirms: D1 TTY path — when --ai-tools is explicitly passed, the AI-tools wizard
// group is hidden (WithHideFunc) and agents are installed from the flag value even
// though the form runner is a no-op (aiTools would otherwise remain empty).
func TestInitD1_TTY_AIToolsFlag_SkipsWizard(t *testing.T) {
	_, home := initFlagsFixture(t)

	// Override TTY detection to force the TTY path.
	origIsTTY := initStdinIsTTY
	t.Cleanup(func() { initStdinIsTTY = origIsTTY })
	initStdinIsTTY = func() bool { return true }

	// Stub form runner as no-op: all wizard vars stay at zero values.
	// If D1 were not implemented, aiTools would remain empty after the form
	// and no agents would be installed.
	origFormRunner := initFormRunner
	t.Cleanup(func() { initFormRunner = origFormRunner })
	initFormRunner = func(_ *huh.Form) error { return nil }

	// Stub identityWizardRunner to avoid interactive prompt.
	origIdentityWizard := identityWizardRunner
	t.Cleanup(func() { identityWizardRunner = origIdentityWizard })
	identityWizardRunner = func(_ *integrations.Catalog, _ string) (identity.Config, bool, error) {
		return identity.Config{}, false, nil
	}

	// Stub promptInstallScopeRunner to avoid interactive prompt.
	origScopeRunner := promptInstallScopeRunner
	t.Cleanup(func() { promptInstallScopeRunner = origScopeRunner })
	promptInstallScopeRunner = func() (string, error) { return "global", nil }

	// D1 TTY: --ai-tools claude explicitly passed → agents installed, wizard skipped.
	if err := runCmd(t, "init", "--ai-tools", "claude"); err != nil {
		t.Fatalf("init --ai-tools claude (TTY path): %v", err)
	}

	// Agents must be installed even though the form runner was a no-op.
	// If D1 were absent, aiTools would be empty and no agents would be written.
	claudeDir := filepath.Join(home, ".claude")
	if _, statErr := os.Stat(claudeDir); os.IsNotExist(statErr) {
		t.Errorf("claude dir %s not created — agents not installed on TTY path with --ai-tools flag", claudeDir)
	}
}
