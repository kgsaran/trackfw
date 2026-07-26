package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func integrationCommandFixture(t *testing.T) (string, string) {
	t.Helper()
	project := t.TempDir()
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldWD, _ := os.Getwd()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return false }
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		_ = os.Setenv("HOME", oldHome)
		integrationsStdinIsTTY = oldTTY
	})
	return project, home
}

func TestAgentsAndSkillsExposeLifecycleHelp(t *testing.T) {
	for _, cmd := range []*cobra.Command{newAgentsCmd(), newSkillsCmd()} {
		for _, name := range []string{"list", "install", "uninstall", "update"} {
			if child, _, err := cmd.Find([]string{name}); err != nil || child == cmd {
				t.Fatalf("%s missing %s subcommand", cmd.Name(), name)
			}
		}
		if cmd.RunE == nil || cmd.Run != nil {
			t.Fatalf("%s without subcommand must have help-only behavior", cmd.Name())
		}
	}
}

func TestInstallRequiresTargetsOutsideTTY(t *testing.T) {
	integrationCommandFixture(t)
	cmd := newAgentsCmd()
	cmd.SetArgs([]string{"install"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires --targets in non-interactive mode") {
		t.Fatalf("expected actionable target error, got %v", err)
	}
}

func TestAgentsJSONLifecycleIsCanonical(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	install := newAgentsCmd()
	install.SetArgs([]string{"install", "--targets", "codex", "--items", "backend", "--scope", "project", "--json"})
	var installed bytes.Buffer
	install.SetOut(&installed)
	install.SetErr(&installed)
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}

	var output lifecycleOutput
	if err := json.Unmarshal(installed.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, installed.String())
	}
	if output.Kind != "agents" || output.CatalogVersion == "" || len(output.Items) != 12 || len(output.Deployments) != 1 {
		t.Fatalf("unexpected canonical output: %#v", output)
	}
	deployment := output.Deployments[0]
	if deployment.Target != "codex" || deployment.Surface != "cli" || deployment.Item != "backend" || deployment.State != "current" || !deployment.Managed {
		t.Fatalf("unexpected deployment: %#v", deployment)
	}
	path := filepath.Join(project, ".codex", "agents", "trackfw-backend.toml")
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "developer_instructions =") {
		t.Fatalf("Codex native TOML missing at %s: %v", path, err)
	}

	uninstall := newAgentsCmd()
	uninstall.SetArgs([]string{"uninstall", "--targets", "codex", "--items", "backend", "--scope", "project"})
	if err := uninstall.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("managed artifact still exists after uninstall: %v", err)
	}
}

func TestListWithoutTargetIncludesAllCatalogSurfaces(t *testing.T) {
	integrationCommandFixture(t)
	cmd := newSkillsCmd()
	cmd.SetArgs([]string{"list", "--items", "governance", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var decoded lifecycleOutput
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Items) != 17 {
		t.Fatalf("list must expose complete catalog, got %d items", len(decoded.Items))
	}
	var legacy bool
	for _, deployment := range decoded.Deployments {
		if deployment.Target == "antigravity" && deployment.Surface == "legacy-cli" {
			legacy = true
		}
	}
	if !legacy {
		t.Fatal("unfiltered list must inspect legacy surfaces too")
	}
}

func TestListWithTargetStillIncludesAllCompatibleSurfaces(t *testing.T) {
	integrationCommandFixture(t)
	cmd := newAgentsCmd()
	cmd.SetArgs([]string{"list", "--targets", "antigravity", "--items", "backend", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var decoded lifecycleOutput
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(decoded.Deployments))
	for _, deployment := range decoded.Deployments {
		got = append(got, deployment.Surface)
	}
	if strings.Join(got, ",") != "current,legacy-cli" {
		t.Fatalf("target filter must retain every compatible surface, got %v", got)
	}
}

func TestDeprecatedCursorAliasUsesLifecycleManager(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	cmd := newCursorCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("missing deprecation warning: %s", stderr.String())
	}
	for _, path := range []string{
		filepath.Join(project, ".cursor", "agents", "trackfw-architect.md"),
		filepath.Join(project, ".cursor", "skills", "trackfw-governance", "SKILL.md"),
		filepath.Join(project, ".cursor", "rules", "trackfw.mdc"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("alias did not install %s: %v", path, err)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".trackfw", "integrations-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), filepath.Join(project, ".cursor", "rules", "trackfw.mdc")) {
		t.Fatal("auxiliary legacy rule must not receive lifecycle ownership")
	}
}

func TestInitAIToolsUsesCanonicalManagerAndValidatesTargets(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	if err := installAITools([]string{"kiro", "antigravity"}, project, "project"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(project, ".kiro", "agents", "trackfw-backend.md"),
		filepath.Join(project, ".kiro", "skills", "trackfw-governance", "SKILL.md"),
		filepath.Join(project, ".agents", "agents", "trackfw-backend", "agent.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("canonical init artifact missing: %s: %v", path, err)
		}
	}

	unknownProject := t.TempDir()
	if err := installAITools([]string{"unknown-ai"}, unknownProject, "project"); err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("unknown target must fail actionably, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(unknownProject, ".trackfw", "integrations-manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("unknown target must not create integration manifest: %v", err)
	}
}

func TestInitAIToolsHelpIncludesEveryCatalogTarget(t *testing.T) {
	usage := newInitCmd().Flag("ai-tools").Usage
	for _, target := range []string{"claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "kiro"} {
		if !strings.Contains(usage, target) {
			t.Fatalf("--ai-tools help omits %s: %s", target, usage)
		}
	}
}

// TestInstallExplicitScopeProjectDoesNotPrompt covers ADR D3: an explicit
// --scope project must be respected verbatim and never triggers the install
// scope prompt, even with stdin faked as a TTY (the only situation in which
// the prompt could ever fire). promptInstallScopeRunner is swapped for a spy
// — if resolveScope prompted anyway, the spy would be called and the test
// would fail.
func TestInstallExplicitScopeProjectDoesNotPrompt(t *testing.T) {
	project, home := integrationCommandFixture(t)
	writeGreekIdentity(t, home)

	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldPrompt := promptInstallScopeRunner
	called := false
	promptInstallScopeRunner = func() (string, error) {
		called = true
		return "global", nil
	}
	t.Cleanup(func() { promptInstallScopeRunner = oldPrompt })

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("explicit --scope project must not invoke the install scope prompt")
	}
	path := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("explicit --scope project must write under the project's .claude/: %v", err)
	}
}

// TestInstallWithoutScopeOutsideTTYDefaultsToGlobal covers ADR D1: with no
// TTY and no --scope, the resolved scope is "global" (~/.claude/...), not
// the previous "project" default.
func TestInstallWithoutScopeOutsideTTYDefaultsToGlobal(t *testing.T) {
	_, home := integrationCommandFixture(t)

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no TTY and no --scope must default to global (~/.claude/): %v", err)
	}
}

// TestInstallWithTargetsButNoScopeStillPromptsInTTY covers ADR D2: the scope
// prompt is a gate independent from target selection — it must fire even
// when --targets was already supplied, which is the most common invocation
// shape. promptInstallScopeRunner is swapped for a spy so a real (blocking)
// huh.Form is never exercised.
func TestInstallWithTargetsButNoScopeStillPromptsInTTY(t *testing.T) {
	project, home := integrationCommandFixture(t)
	writeGreekIdentity(t, home)

	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldPrompt := promptInstallScopeRunner
	called := false
	promptInstallScopeRunner = func() (string, error) {
		called = true
		return "project", nil
	}
	t.Cleanup(func() { promptInstallScopeRunner = oldPrompt })

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("scope prompt must fire even when --targets was already supplied")
	}
	// The prompt's answer must actually be honored, not merely invoked — a
	// regression where resolveScope calls the runner and drops its return
	// value would still leave `called` true.
	path := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("prompt-chosen scope (project) must be honored: %v", err)
	}
}

// TestInstallScopeSelectDefaultsToGlobal covers ADR D2's "global pre-selected"
// requirement directly against the real huh.Select field built by
// installScopeSelect — not against promptInstallScopeRunner, which every
// other test in this file stubs out entirely (roadmap divergence #5,
// ROADMAP-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills).
// Select.RunAccessible reads a bare Enter from a fake reader and, because the
// field's accessor is a *PointerAccessor bound to a variable pre-set to
// "global", falls back to the option matching that value — this is huh's own
// non-interactive fallback path, not a stub, so it fails if the pre-selected
// option is ever changed away from "global" without updating this test.
func TestInstallScopeSelectDefaultsToGlobal(t *testing.T) {
	scope := "global"
	sel := installScopeSelect(&scope)
	if err := sel.RunAccessible(&bytes.Buffer{}, strings.NewReader("\n")); err != nil {
		t.Fatalf("RunAccessible with bare Enter must accept the pre-selected default: %v", err)
	}
	if scope != "global" {
		t.Fatalf("expected pre-selected scope to be %q on bare Enter, got %q", "global", scope)
	}
}

// TestListWithoutScopeReportsGlobalDestinations covers ADR D6: `list` never
// prompts (it is a read-only command), but adopts the same "global" default
// as install/update/uninstall so it does not report deployments at a scope
// the mutating commands wouldn't have written to.
func TestListWithoutScopeReportsGlobalDestinations(t *testing.T) {
	_, home := integrationCommandFixture(t)

	cmd := newAgentsCmd()
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	list := newAgentsCmd()
	var out bytes.Buffer
	list.SetOut(&out)
	list.SetArgs([]string{"list", "--targets", "claude", "--items", "backend", "--json"})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	var output lifecycleOutput
	if err := json.Unmarshal(out.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(output.Deployments) != 1 || output.Deployments[0].Scope != "global" {
		t.Fatalf("list without --scope must report global destinations: %+v", output.Deployments)
	}
	if !strings.HasPrefix(output.Deployments[0].Destination, "~/") {
		t.Fatalf("list without --scope must report a home-relative (~/) destination: %+v", output.Deployments[0])
	}
	path := filepath.Join(home, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("install without --scope must have written under home: %v", err)
	}
}

func TestDeprecatedAliasesPreserveAuxiliaryRulesWithoutOwnership(t *testing.T) {
	tests := []struct {
		name     string
		command  func() *cobra.Command
		rulePath string
	}{
		{"gemini", newGeminiCmd, "GEMINI.md"},
		{"copilot", newCopilotCmd, filepath.Join(".github", "copilot-instructions.md")},
		{"windsurf", newWindsurfCmd, ".windsurfrules"},
		{"amazonq", newAmazonQCmd, filepath.Join(".amazonq", "developer", "guidelines.md")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project, _ := integrationCommandFixture(t)
			cmd := test.command()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			rule := filepath.Join(project, test.rulePath)
			if _, err := os.Stat(rule); err != nil {
				t.Fatalf("legacy alias rule missing: %s: %v", rule, err)
			}
			manifest, err := os.ReadFile(filepath.Join(project, ".trackfw", "integrations-manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(manifest), rule) {
				t.Fatalf("auxiliary rule unexpectedly owned by lifecycle: %s", rule)
			}
		})
	}
}
