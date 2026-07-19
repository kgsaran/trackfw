package integrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyHashesMatchReleasedCollidingArtifacts(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	claudePlans, err := BuildPlans(catalog, PlanRequest{Kind: KindAgents, Targets: []string{"claude"}, Items: []string{"backend"}, Scope: "global"})
	if err != nil {
		t.Fatal(err)
	}
	legacyClaude, err := os.ReadFile(filepath.Join("..", "generators", "templates", "agents", "trackfw-backend.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !hashIn(contentHash(legacyClaude), claudePlans[0].LegacyHashes) {
		t.Fatal("released Claude global agent hash was not recognized")
	}

	legacyCodexBackend := []byte(`name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope. Preserve public contracts and trackfw traceability.
Run focused tests and report changed files, validation evidence, and remaining risks.
"""
`)
	codexPlans, err := BuildPlans(catalog, PlanRequest{Kind: KindAgents, Targets: []string{"codex"}, Items: []string{"backend"}, Scope: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if !hashIn(contentHash(legacyCodexBackend), codexPlans[0].LegacyHashes) {
		t.Fatal("released Codex project agent hash was not recognized")
	}
	if len(codexPlans[0].LegacyHashes) != 3 {
		t.Fatalf("Codex migration must recognize Go+npm+PyPI bytes, got %v", codexPlans[0].LegacyHashes)
	}
	if hashes := LegacyHashes(Claim{Target: "cursor", Surface: "ide", Scope: "project", Kind: KindAgents, Item: "backend"}); len(hashes) != 0 {
		t.Fatalf("non-colliding legacy path must not be adopted: %v", hashes)
	}
}

func TestLegacyLifecycleAdoptsWithoutOverwriteThenUpdates(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlans(catalog, PlanRequest{Kind: KindAgents, Targets: []string{"claude"}, Items: []string{"backend"}, Scope: "global"})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := os.ReadFile(filepath.Join("..", "generators", "templates", "agents", "trackfw-backend.md"))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(home, ".claude", "agents", "trackfw-backend.md")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := Manager{ProjectRoot: project, HomeDir: home}
	inspection, err := manager.Inspect(plans[0])
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != StateOutdated || inspection.Managed {
		t.Fatalf("legacy list state = %#v, want outdated/unmanaged", inspection)
	}
	if err := manager.Install(plans, false); err != nil {
		t.Fatal(err)
	}
	afterInstall, _ := os.ReadFile(destination)
	if string(afterInstall) != string(legacy) {
		t.Fatal("install overwrote a known legacy artifact")
	}
	inspection, _ = manager.Inspect(plans[0])
	if inspection.State != StateOutdated || !inspection.Managed {
		t.Fatalf("adopted legacy state = %#v, want outdated/managed", inspection)
	}
	if err := manager.Update(plans, false); err != nil {
		t.Fatal(err)
	}
	inspection, _ = manager.Inspect(plans[0])
	if inspection.State != StateCurrent || !inspection.Managed {
		t.Fatalf("updated legacy state = %#v, want current/managed", inspection)
	}
}

func TestLegacyUnknownCannotBeAdoptedByForcedUpdate(t *testing.T) {
	project, home := t.TempDir(), t.TempDir()
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlans(catalog, PlanRequest{Kind: KindAgents, Targets: []string{"codex"}, Items: []string{"backend"}, Scope: "project"})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(project, ".codex", "agents", "trackfw-backend.toml")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	unknown := []byte("user-owned unknown bytes\n")
	if err := os.WriteFile(destination, unknown, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := Manager{ProjectRoot: project, HomeDir: home}
	if err := manager.Update(plans, true); err == nil {
		t.Fatal("forced update adopted unknown unmanaged bytes")
	}
	actual, _ := os.ReadFile(destination)
	if string(actual) != string(unknown) {
		t.Fatal("forced update changed unknown unmanaged bytes")
	}
}
