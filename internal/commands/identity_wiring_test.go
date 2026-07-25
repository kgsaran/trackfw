package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

// writeGreekIdentity persists the "greek" preset to home/.trackfw/identity.json.
// It is the fixture shared by the tests below, which prove that identity
// propagates end-to-end through each of the four internal/integrations
// BuildPlans callers (ADR ADR-2026-07-25-identidade-personalizavel-de-agentes,
// D7). If any caller stops resolving identity.Load and passing it into
// PlanRequest.Identity, the user's custom identity is silently reverted the
// next time that caller runs — these tests are the only guard against that.
func writeGreekIdentity(t *testing.T, home string) {
	t.Helper()
	cfg, err := identity.Preset("greek")
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Save(home, cfg); err != nil {
		t.Fatal(err)
	}
}

// TestIdentityPropagatesThroughInstallMutationCaller covers caller #1:
// executeIntegrationMutation (internal/commands/integrations_flags.go).
func TestIdentityPropagatesThroughInstallMutationCaller(t *testing.T) {
	project, home := integrationCommandFixture(t)
	writeGreekIdentity(t, home)

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "apolo-tf") {
		t.Fatalf("install mutation caller did not propagate identity:\n%s", data)
	}
}

// TestIdentityPropagatesThroughListCaller covers caller #2:
// executeIntegrationList (internal/commands/integrations_flags.go).
//
// list never writes files — the tell is indirect: it recomputes
// PlannedArtifact.Content in-memory and asks the manager to compare that
// content against what is on disk. If list resolved no identity (or a
// neutral one) while install had already written identity-personalized
// content, the two would disagree and the reported lifecycle State would be
// "stale" instead of "current".
func TestIdentityPropagatesThroughListCaller(t *testing.T) {
	project, home := integrationCommandFixture(t)
	writeGreekIdentity(t, home)
	_ = project

	install := newAgentsCmd()
	install.SetOut(&bytes.Buffer{})
	install.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}

	list := newAgentsCmd()
	var out bytes.Buffer
	list.SetOut(&out)
	list.SetArgs([]string{"list", "--targets", "claude", "--items", "backend", "--scope", "project", "--json"})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	var output lifecycleOutput
	if err := json.Unmarshal(out.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(output.Deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(output.Deployments))
	}
	if output.Deployments[0].State != "current" {
		t.Fatalf("list caller did not propagate identity — content drift reported as %q: %+v", output.Deployments[0].State, output.Deployments[0])
	}
}

// TestIdentityPropagatesThroughInitInstallAITools covers caller #3:
// installAITools (internal/commands/init.go), invoked by `trackfw init`
// after AI tools are chosen.
func TestIdentityPropagatesThroughInitInstallAITools(t *testing.T) {
	project, home := integrationCommandFixture(t)
	writeGreekIdentity(t, home)

	if err := installAITools([]string{"claude"}, project, "project"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "apolo-tf") {
		t.Fatalf("installAITools did not propagate identity:\n%s", data)
	}
}
