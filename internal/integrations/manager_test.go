package integrations

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestManagerLifecycleStates(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".claude/agents/trackfw-backend.md", "v1", "first")

	assertState(t, manager, plan, StateNotInstalled)
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	assertState(t, manager, plan, StateCurrent)

	newPlan := plan
	newPlan.Content = []byte("second")
	newPlan.CatalogVersion = "v2"
	assertState(t, manager, newPlan, StateOutdated)
	if err := manager.Update([]PlannedArtifact{newPlan}, false); err != nil {
		t.Fatal(err)
	}
	assertState(t, manager, newPlan, StateCurrent)

	filename := filepath.Join(project, ".claude/agents/trackfw-backend.md")
	if err := os.WriteFile(filename, []byte("custom"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertState(t, manager, newPlan, StateModified)
	if err := manager.Update([]PlannedArtifact{newPlan}, false); err == nil {
		t.Fatal("Update() should protect modified content")
	}
	if got := readFile(t, filename); got != "custom" {
		t.Fatalf("modified content overwritten: %q", got)
	}
	if err := manager.Update([]PlannedArtifact{newPlan}, true); err != nil {
		t.Fatal(err)
	}
	assertState(t, manager, newPlan, StateCurrent)
}

func TestManagerSharedClaimsPreservePhysicalArtifact(t *testing.T) {
	manager, project, _ := testManager(t)
	first := testPlan("project", ".agents/shared.md", "v1", "shared")
	second := first
	second.Claim.Target = "codex"
	second.Claim.Surface = "cli"

	if err := manager.Install([]PlannedArtifact{first, second}, false); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(project, ".agents/shared.md")
	if err := manager.Uninstall([]PlannedArtifact{first}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("shared artifact removed with active claim: %v", err)
	}
	manifest, err := loadManifest(manifestPath(project))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(manifest.Artifacts[filename].Claims); got != 1 {
		t.Fatalf("claims = %d, want 1", got)
	}
	if err := manager.Uninstall([]PlannedArtifact{second}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("artifact remains after last claim: %v", err)
	}
}

func TestManagerLegacyAdoptionAndUpdate(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".gemini/agents/trackfw-backend.md", "v2", "current")
	legacy := []byte("legacy")
	plan.LegacyHashes = []string{contentHash(legacy)}
	filename := filepath.Join(project, plan.Destination)
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	assertState(t, manager, plan, StateOutdated)
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filename); got != "legacy" {
		t.Fatalf("install should adopt legacy content, got %q", got)
	}
	if err := manager.Update([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filename); got != "current" {
		t.Fatalf("legacy update = %q", got)
	}
}

func TestManagerUnmanagedAndModifiedRequireForce(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", "agents/backend.md", "v1", "managed")
	filename := filepath.Join(project, plan.Destination)
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Install([]PlannedArtifact{plan}, false); err == nil {
		t.Fatal("Install() adopted unknown unmanaged content")
	}
	if err := manager.Install([]PlannedArtifact{plan}, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("custom"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall([]PlannedArtifact{plan}, false); err == nil {
		t.Fatal("Uninstall() removed modified content without force")
	}
	if err := manager.Uninstall([]PlannedArtifact{plan}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("forced uninstall left artifact: %v", err)
	}
}

func TestManagerUpdateForceNeverAdoptsUnknownUnmanagedContent(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", "agents/backend.md", "v2", "managed")
	filename := filepath.Join(project, plan.Destination)
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("user-owned bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := manager.Update([]PlannedArtifact{plan}, true); err == nil {
		t.Fatal("Update(force) adopted unknown unmanaged content")
	}
	if got := readFile(t, filename); got != "user-owned bytes" {
		t.Fatalf("Update(force) changed unmanaged bytes to %q", got)
	}
	if _, err := os.Stat(manifestPath(project)); !os.IsNotExist(err) {
		t.Fatalf("Update(force) created ownership manifest: %v", err)
	}
}

func TestManagerUninstallRemovesEmptyAncestorDirectories(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".agents/skills/backend/SKILL.md", "v1", "managed")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("empty managed ancestors remain: %v", err)
	}
	if info, err := os.Stat(project); err != nil || !info.IsDir() {
		t.Fatalf("project root was removed: info=%v err=%v", info, err)
	}
}

func TestManagerUninstallPreservesSiblingAndItsAncestors(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".agents/skills/backend/SKILL.md", "v1", "managed")
	sibling := filepath.Join(project, ".agents", "skills", "user.md")
	if err := os.MkdirAll(filepath.Dir(sibling), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, sibling); got != "user" {
		t.Fatalf("sibling changed to %q", got)
	}
	if info, err := os.Stat(filepath.Dir(sibling)); err != nil || !info.IsDir() {
		t.Fatalf("sibling ancestor removed: info=%v err=%v", info, err)
	}
}

func TestManagerRejectsTraversalAbsoluteMismatchAndNUL(t *testing.T) {
	manager, _, home := testManager(t)
	cases := []PlannedArtifact{
		testPlan("project", "../outside.md", "v1", "x"),
		testPlan("project", filepath.Join(home, "outside.md"), "v1", "x"),
		testPlan("global", "/tmp/outside-trackfw.md", "v1", "x"),
		testPlan("project", "bad\x00name.md", "v1", "x"),
	}
	for _, plan := range cases {
		if err := manager.Install([]PlannedArtifact{plan}, false); err == nil {
			t.Errorf("Install(%q, %s) accepted unsafe destination", plan.Destination, plan.Claim.Scope)
		}
	}
}

func TestManagerRejectsSymlinkFileAndParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is privilege-dependent on Windows")
	}
	manager, project, _ := testManager(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(project, "linked")); err != nil {
		t.Fatal(err)
	}
	parentPlan := testPlan("project", "linked/backend.md", "v1", "x")
	if err := manager.Install([]PlannedArtifact{parentPlan}, false); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink parent error = %v", err)
	}

	target := filepath.Join(project, "real.md")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(project, "link.md")); err != nil {
		t.Fatal(err)
	}
	filePlan := testPlan("project", "link.md", "v1", "x")
	if _, err := manager.Inspect(filePlan); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink file error = %v", err)
	}
}

func TestManagerSeparatesProjectAndGlobalManifests(t *testing.T) {
	manager, project, home := testManager(t)
	projectPlan := testPlan("project", ".agents/project.md", "v1", "project")
	globalPlan := testPlan("global", "~/.agents/global.md", "v1", "global")
	if err := manager.Install([]PlannedArtifact{projectPlan, globalPlan}, false); err != nil {
		t.Fatal(err)
	}
	projectManifest, err := loadManifest(manifestPath(project))
	if err != nil {
		t.Fatal(err)
	}
	globalManifest, err := loadManifest(manifestPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if len(projectManifest.Artifacts) != 1 || len(globalManifest.Artifacts) != 1 {
		t.Fatalf("manifest sizes = project %d, global %d", len(projectManifest.Artifacts), len(globalManifest.Artifacts))
	}
}

func TestManagerPreflightRollsBackBatch(t *testing.T) {
	manager, project, home := testManager(t)
	valid := testPlan("project", "agents/valid.md", "v1", "valid")
	invalid := testPlan("project", filepath.Join(home, "escape.md"), "v1", "invalid")
	if err := manager.Install([]PlannedArtifact{valid, invalid}, false); err == nil {
		t.Fatal("batch with invalid destination succeeded")
	}
	if _, err := os.Stat(filepath.Join(project, valid.Destination)); !os.IsNotExist(err) {
		t.Fatalf("partial artifact remains: %v", err)
	}
	if _, err := os.Stat(manifestPath(project)); !os.IsNotExist(err) {
		t.Fatalf("partial manifest remains: %v", err)
	}
}

func testManager(t *testing.T) (Manager, string, string) {
	t.Helper()
	project := t.TempDir()
	home := t.TempDir()
	return Manager{ProjectRoot: project, HomeDir: home}, project, home
}

func testPlan(scope, destination, version, content string) PlannedArtifact {
	return PlannedArtifact{
		Claim:       Claim{Target: "claude", Surface: "code", Scope: scope, Kind: KindAgents, Item: "backend"},
		Destination: destination, Content: []byte(content), CatalogVersion: version, SupportLevel: "native",
	}
}

func assertState(t *testing.T, manager Manager, plan PlannedArtifact, want LifecycleState) {
	t.Helper()
	inspection, err := manager.Inspect(plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != want {
		t.Fatalf("state = %q, want %q", inspection.State, want)
	}
}

func readFile(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
