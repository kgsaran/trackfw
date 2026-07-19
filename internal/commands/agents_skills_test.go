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
	if output.Kind != "agents" || output.CatalogVersion == "" || len(output.Items) != 10 || len(output.Deployments) != 1 {
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
	uninstall.SetArgs([]string{"uninstall", "--targets", "codex", "--items", "backend"})
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
	if len(decoded.Items) != 5 {
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
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("alias did not install %s: %v", path, err)
		}
	}
}
