package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// TestUpdateDetectedCodexIntegrationsPropagatesIdentity covers caller #4:
// updateDetectedCodexIntegrations (internal/generators/update.go), invoked by
// Update when a Codex install is detected in the project.
//
// It installs a Codex agent while the "greek" identity preset is active,
// switches the persisted identity to "norse", and asserts that re-running
// Update rewrites the managed artifact with the new identity — proving this
// caller resolves identity.Load fresh on every run instead of caching a
// stale (or neutral-default) copy.
func TestUpdateDetectedCodexIntegrationsPropagatesIdentity(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Existing instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	greek, err := identity.Preset("greek")
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Save(home, greek); err != nil {
		t.Fatal(err)
	}

	catalog, err := integrations.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind: integrations.KindAgents, Targets: []string{"codex"}, Items: []string{"backend"}, Scope: "project", Identity: greek,
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := integrations.Manager{ProjectRoot: root, HomeDir: home}
	if err := manager.Install(plans, false); err != nil {
		t.Fatal(err)
	}

	backendPath := filepath.Join(root, ".codex", "agents", "trackfw-backend.toml")
	initial, err := os.ReadFile(backendPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initial), "apolo_tf") {
		t.Fatalf("fixture setup did not render the greek identity:\n%s", initial)
	}

	norse, err := identity.Preset("norse")
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Save(home, norse); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(backendPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "thor_tf") {
		t.Fatalf("updateDetectedCodexIntegrations did not propagate the updated identity:\n%s", updated)
	}
	if strings.Contains(string(updated), "apolo_tf") {
		t.Fatalf("stale greek identity still present after update:\n%s", updated)
	}
}
