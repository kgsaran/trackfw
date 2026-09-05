package integrations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	manager, project, _ := testManager(t)
	outside := t.TempDir()
	if !symlinkOrSkip(t, outside, filepath.Join(project, "linked")) {
		return
	}
	parentPlan := testPlan("project", "linked/backend.md", "v1", "x")
	if err := manager.Install([]PlannedArtifact{parentPlan}, false); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink parent error = %v", err)
	}

	target := filepath.Join(project, "real.md")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !symlinkOrSkip(t, target, filepath.Join(project, "link.md")) {
		return
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

func TestResolveWindowsCrossplatform(t *testing.T) {
	// Verifies that resolve() uses POSIX semantics so forward-slash paths from
	// the catalog (e.g. ".claude/agents/x.md") are accepted on all platforms,
	// including Windows where filepath.Clean would convert "/" → "\".
	accept := []string{
		".claude/agents/trackfw-architect.md",
		".amazonq/cli-agents/trackfw-architect.json",
	}
	for _, dest := range accept {
		manager, _, _ := testManager(t)
		plan := testPlan("project", dest, "v1", "content")
		if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
			t.Errorf("Install(%q) rejected valid POSIX path: %v", dest, err)
		}
	}

	reject := []string{
		"..",
		"../x",
		"a/../../x",
		".",
		"./x",
		"",
		"bad\x00name",
	}
	for _, dest := range reject {
		manager, _, _ := testManager(t)
		plan := testPlan("project", dest, "v1", "content")
		if err := manager.Install([]PlannedArtifact{plan}, false); err == nil {
			t.Errorf("Install(%q) accepted unsafe destination", dest)
		}
	}
}

func TestManagerRejectsNameCollisionAmongAgentMarkdownArtifacts(t *testing.T) {
	manager, project, _ := testManager(t)
	// Um arquivo pré-existente (não gerenciado pelo trackfw) no mesmo
	// diretório de destino já declara o mesmo name que o artefato planejado.
	existing := filepath.Join(project, ".claude/agents/other.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("---\nname: zeus-tf\n---\n\nOther agent\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	plan := testPlan("project", ".claude/agents/trackfw-architect.md", "v1", "---\nname: zeus-tf\n---\n\nArchitect\n")
	if err := manager.Install([]PlannedArtifact{plan}, false); err == nil {
		t.Fatal("Install() should reject a colliding declared name without force")
	}
	if _, err := os.Stat(filepath.Join(project, plan.Destination)); !os.IsNotExist(err) {
		t.Fatalf("colliding artifact should not have been written: %v", err)
	}

	if err := manager.Install([]PlannedArtifact{plan}, true); err != nil {
		t.Fatalf("Install(force) should proceed past a name collision: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, plan.Destination)); err != nil {
		t.Fatalf("Install(force) should have written the artifact: %v", err)
	}
}

func TestManagerNameCollisionIgnoresOwnDestination(t *testing.T) {
	manager, _, _ := testManager(t)
	plan := testPlan("project", ".claude/agents/trackfw-architect.md", "v1", "---\nname: zeus-tf\n---\n\nArchitect\n")
	// Instalar e depois atualizar o próprio artefato não deve ser tratado
	// como colisão, mesmo declarando o mesmo name que ele mesmo já tinha.
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	updated := plan
	updated.Content = []byte("---\nname: zeus-tf\n---\n\nArchitect updated\n")
	updated.CatalogVersion = "v2"
	if err := manager.Update([]PlannedArtifact{updated}, false); err != nil {
		t.Fatalf("Update() falsely detected collision with its own destination: %v", err)
	}
}

func TestManagerNameCollisionOnlyAppliesToAgentsMarkdown(t *testing.T) {
	manager, project, _ := testManager(t)
	existing := filepath.Join(project, ".codex/agents/other.toml")
	if err := os.MkdirAll(filepath.Dir(existing), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte(`name = "zeus_tf"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Mesmo "name" declarado, mas em formato .toml — a varredura de colisão
	// é limitada a .md (ver comentário em detectNameCollision), então não
	// deve haver erro.
	plan := testPlan("project", ".codex/agents/trackfw-architect.toml", "v1", `name = "zeus_tf"`+"\n")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("Install() should not scan .toml siblings for collisions: %v", err)
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

// TestManagerInstallSkipsOwnedOutdatedPreservesBytes verifies the contract
// from docs/cli-parity.md §"install sobre artefato gerenciado desatualizado":
//
//  1. Install v1 of an artifact → artifact is owned+current in manifest.
//  2. Attempt Install of v2 (same claim, new content/version) in a batch that
//     also contains a second artifact at a different destination.
//  3. The skipped artifact's bytes are preserved (v1content), the second
//     artifact is applied normally, and Install returns nil.
//  4. OnSkip is called exactly once with a tilde-abbreviated destination and
//     a warning line byte-identical to the pinned contract string.
//  5. OnSkip=nil does not panic.
func TestManagerInstallSkipsOwnedOutdatedPreservesBytes(t *testing.T) {
	manager, project, home := testManager(t)

	// --- global scope: install v1 → outdated+owned skip scenario ---
	planV1 := PlannedArtifact{
		Claim:          Claim{Target: "gemini", Surface: "cli", Scope: "global", Kind: KindAgents, Item: "architect"},
		Destination:    "~/.gemini/agents/trackfw-architect.md",
		Content:        []byte("v1content"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}
	// Install v1: file is written, claim is recorded in manifest.
	if err := manager.Install([]PlannedArtifact{planV1}, false); err != nil {
		t.Fatalf("Install v1 failed: %v", err)
	}
	globalDest := filepath.Join(home, ".gemini/agents/trackfw-architect.md")
	if got := readFile(t, globalDest); got != "v1content" {
		t.Fatalf("after Install v1, file = %q, want %q", got, "v1content")
	}

	// planV2: new version, different content — file is now outdated+owned.
	planV2 := PlannedArtifact{
		Claim:          planV1.Claim,
		Destination:    planV1.Destination,
		Content:        []byte("v2content"),
		CatalogVersion: "v2",
		SupportLevel:   "native",
		// v1content in LegacyHashes matches the handoff description; for a
		// managed artifact inspectResolved does not use LegacyHashes, so this
		// is informational only and harmless.
		LegacyHashes: []string{contentHash([]byte("v1content"))},
	}

	// Second artifact in the same batch: project-scoped, different destination.
	secondPlan := PlannedArtifact{
		Claim:          Claim{Target: "claude", Surface: "code", Scope: "project", Kind: KindAgents, Item: "backend"},
		Destination:    ".claude/agents/trackfw-backend.md",
		Content:        []byte("backend-content"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}

	var skipDestinations []string
	var skipReasons []string
	manager.OnSkip = func(destination, reason string) {
		skipDestinations = append(skipDestinations, destination)
		skipReasons = append(skipReasons, reason)
	}

	if err := manager.Install([]PlannedArtifact{planV2, secondPlan}, false); err != nil {
		t.Fatalf("Install batch failed: %v", err)
	}

	// (a) bytes of skipped artifact preserved.
	if got := readFile(t, globalDest); got != "v1content" {
		t.Fatalf("skipped artifact bytes changed: got %q, want %q", got, "v1content")
	}

	// (b) second artifact applied normally.
	secondDest := filepath.Join(project, ".claude/agents/trackfw-backend.md")
	if got := readFile(t, secondDest); got != "backend-content" {
		t.Fatalf("second artifact = %q, want %q", got, "backend-content")
	}

	// (c) OnSkip called exactly once.
	if len(skipDestinations) != 1 {
		t.Fatalf("OnSkip called %d times, want 1; destinations=%v", len(skipDestinations), skipDestinations)
	}

	// (c) destination is tilde-abbreviated.
	wantDest := "~/.gemini/agents/trackfw-architect.md"
	if skipDestinations[0] != wantDest {
		t.Fatalf("OnSkip destination = %q, want %q", skipDestinations[0], wantDest)
	}

	// (c) reason is byte-identical to the pinned contract string (global scope →
	// 'trackfw update harness').
	wantReason := "warning: skipping outdated artifact ~/.gemini/agents/trackfw-architect.md; run 'trackfw update harness' to refresh it"
	if skipReasons[0] != wantReason {
		t.Fatalf("OnSkip reason = %q\nwant            %q", skipReasons[0], wantReason)
	}

	// (d) second artifact absent from skipped destinations.
	for _, d := range skipDestinations {
		if d == secondPlan.Destination || strings.HasSuffix(d, "trackfw-backend.md") {
			t.Fatalf("second artifact should not have been skipped: %v", skipDestinations)
		}
	}

	// --- project scope: verify tilde abbreviation and remediation command ---
	manager2, project2, home2 := testManager(t)
	_ = home2
	projectPlanV1 := PlannedArtifact{
		Claim:          Claim{Target: "claude", Surface: "code", Scope: "project", Kind: KindAgents, Item: "architect"},
		Destination:    ".claude/agents/trackfw-architect.md",
		Content:        []byte("p-v1content"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}
	if err := manager2.Install([]PlannedArtifact{projectPlanV1}, false); err != nil {
		t.Fatalf("project Install v1 failed: %v", err)
	}
	projectPlanV2 := projectPlanV1
	projectPlanV2.Content = []byte("p-v2content")
	projectPlanV2.CatalogVersion = "v2"

	var projectSkipDests []string
	var projectSkipReasons []string
	manager2.OnSkip = func(destination, reason string) {
		projectSkipDests = append(projectSkipDests, destination)
		projectSkipReasons = append(projectSkipReasons, reason)
	}
	if err := manager2.Install([]PlannedArtifact{projectPlanV2}, false); err != nil {
		t.Fatalf("project Install v2 failed: %v", err)
	}
	if len(projectSkipDests) != 1 {
		t.Fatalf("project OnSkip called %d times, want 1", len(projectSkipDests))
	}
	wantProjectDest := ".claude/agents/trackfw-architect.md"
	if projectSkipDests[0] != wantProjectDest {
		t.Fatalf("project OnSkip destination = %q, want %q", projectSkipDests[0], wantProjectDest)
	}
	wantProjectReason := "warning: skipping outdated artifact .claude/agents/trackfw-architect.md; run 'trackfw update' to refresh it"
	if projectSkipReasons[0] != wantProjectReason {
		t.Fatalf("project OnSkip reason = %q\nwant               %q", projectSkipReasons[0], wantProjectReason)
	}
	projectArtifact := filepath.Join(project2, ".claude/agents/trackfw-architect.md")
	if got := readFile(t, projectArtifact); got != "p-v1content" {
		t.Fatalf("project skipped artifact bytes changed: got %q, want %q", got, "p-v1content")
	}
}

// TestManagerInstallSkipOnSkipNilNoPanic verifies that Install does not panic
// when Manager.OnSkip is nil and an artifact is skipped (outdated+owned).
func TestManagerInstallSkipOnSkipNilNoPanic(t *testing.T) {
	manager, _, _ := testManager(t)
	planV1 := PlannedArtifact{
		Claim:          Claim{Target: "claude", Surface: "code", Scope: "project", Kind: KindAgents, Item: "architect"},
		Destination:    ".claude/agents/trackfw-architect.md",
		Content:        []byte("v1"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}
	if err := manager.Install([]PlannedArtifact{planV1}, false); err != nil {
		t.Fatal(err)
	}
	planV2 := planV1
	planV2.Content = []byte("v2")
	planV2.CatalogVersion = "v2"
	manager.OnSkip = nil // explicitly nil — must not panic
	if err := manager.Install([]PlannedArtifact{planV2}, false); err != nil {
		t.Fatalf("Install with nil OnSkip failed: %v", err)
	}
}

// TestManagerInstallSkipMixedScopeBatch mirrors Node.js agents-skills.test.js:208
// and Python test_agents_skills.py:298. It proves that the remediation command
// is derived from plan.Claim.Scope per artifact, NOT from a uniform batch-scope
// closure — a closure would be correct by accident for uniform-scope batches;
// only a mixed-scope batch (global + project in the same Install call)
// distinguishes per-artifact derivation from per-batch derivation.
func TestManagerInstallSkipMixedScopeBatch(t *testing.T) {
	manager, _, _ := testManager(t)

	// Install v1 for a global artifact and a project artifact.
	globalV1 := PlannedArtifact{
		Claim:          Claim{Target: "gemini", Surface: "cli", Scope: "global", Kind: KindAgents, Item: "architect"},
		Destination:    "~/.gemini/agents/trackfw-architect.md",
		Content:        []byte("global-v1"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}
	projectV1 := PlannedArtifact{
		Claim:          Claim{Target: "claude", Surface: "code", Scope: "project", Kind: KindAgents, Item: "backend"},
		Destination:    ".claude/agents/trackfw-backend.md",
		Content:        []byte("project-v1"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}
	if err := manager.Install([]PlannedArtifact{globalV1, projectV1}, false); err != nil {
		t.Fatalf("Install v1 batch failed: %v", err)
	}

	// v2 plans: bump CatalogVersion → both become outdated+owned.
	globalV2 := globalV1
	globalV2.Content = []byte("global-v2")
	globalV2.CatalogVersion = "v2"
	projectV2 := projectV1
	projectV2.Content = []byte("project-v2")
	projectV2.CatalogVersion = "v2"

	type skipRecord struct{ dest, reason string }
	var skips []skipRecord
	manager.OnSkip = func(destination, reason string) {
		skips = append(skips, skipRecord{destination, reason})
	}

	// Mixed-scope batch: both outdated+owned → both must be skipped, each with
	// the remediation correct for its own scope (derived from plan.Claim.Scope,
	// not from a shared batch closure).
	if err := manager.Install([]PlannedArtifact{globalV2, projectV2}, false); err != nil {
		t.Fatalf("Install mixed-scope batch failed: %v", err)
	}

	if len(skips) != 2 {
		t.Fatalf("OnSkip called %d times, want 2; skips=%v", len(skips), skips)
	}

	// Index by destination for order-independent assertion.
	byDest := make(map[string]string, 2)
	for _, s := range skips {
		byDest[s.dest] = s.reason
	}

	wantGlobalDest := "~/.gemini/agents/trackfw-architect.md"
	wantProjectDest := ".claude/agents/trackfw-backend.md"

	globalReason, ok := byDest[wantGlobalDest]
	if !ok {
		t.Fatalf("no skip for global artifact %q; byDest=%v", wantGlobalDest, byDest)
	}
	projectReason, ok := byDest[wantProjectDest]
	if !ok {
		t.Fatalf("no skip for project artifact %q; byDest=%v", wantProjectDest, byDest)
	}

	wantGlobal := "warning: skipping outdated artifact ~/.gemini/agents/trackfw-architect.md; run 'trackfw update harness' to refresh it"
	if globalReason != wantGlobal {
		t.Fatalf("global reason =  %q\nwant             %q", globalReason, wantGlobal)
	}
	wantProject := "warning: skipping outdated artifact .claude/agents/trackfw-backend.md; run 'trackfw update' to refresh it"
	if projectReason != wantProject {
		t.Fatalf("project reason = %q\nwant            %q", projectReason, wantProject)
	}
}

// TestManagerInstallOwnedModifiedRemainsError verifies that an owned artifact
// with user-modified bytes still returns an error on Install without --force.
// This guards against accidentally simetrizing the Modified and Outdated cases
// in preflight (the contract explicitly requires them to be asymmetric).
func TestManagerInstallOwnedModifiedRemainsError(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".claude/agents/trackfw-architect.md", "v1", "managed")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(project, plan.Destination)
	// Overwrite with user bytes — artifact is now owned+modified.
	if err := os.WriteFile(filename, []byte("user-modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Install without --force must error.
	if err := manager.Install([]PlannedArtifact{plan}, false); err == nil {
		t.Fatal("Install() should error on owned+modified artifact without --force")
	}
	// Bytes must be preserved.
	if got := readFile(t, filename); got != "user-modified" {
		t.Fatalf("modified bytes overwritten: got %q", got)
	}
	// Install with --force must succeed.
	if err := manager.Install([]PlannedArtifact{plan}, true); err != nil {
		t.Fatalf("Install(force) on owned+modified failed: %v", err)
	}
}

// ML-1A de ROADMAP-2026-09-05-fechar-os-tres-defeitos-mecanicos-dos-issues-do-consumidor-externo:
// porta para este pacote o MESMO idioma ja estabelecido em
// internal/generators/update_test.go:symlinkOrSkip (introduzido no #221,
// anterior ao ML-4A) -- detecao pela CONDICAO (WinError 1314,
// ERROR_PRIVILEGE_NOT_HELD), nao por runtime.GOOS: num Windows com
// Developer Mode habilitado, ou em Linux/macOS, os.Symlink tem sucesso e o
// chamador roda normalmente.
//
// Ao contrario da sonda de bit de execucao (execBitRepresentavelPara), aqui
// nao ha "resto do teste" independente do symlink -- o proprio symlink e a
// condicao sob teste, entao a supressao continua sendo t.Skip (nao um
// Stderr + return silencioso disfarcado de PASS): SKIP e mais honesto que
// um PASS que nao verificou nada.

// symlinkOrSkip cria um symlink em link apontando para target. Se a criacao
// falhar por falta do privilegio que o Windows exige (Developer Mode ou
// processo elevado), pula o teste chamador nomeando a garantia nao
// exercitada e devolve false. Qualquer outro erro e um t.Fatalf.
func symlinkOrSkip(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeError(err) {
		t.Skipf("guarda de symlink nao exercitada: criacao de symlink exige Developer Mode (ou processo elevado) neste Windows: %v", err)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeError reporta se err e a falha "processo sem privilegio
// para criar symlink" -- WinError 1314 no Windows sem Developer
// Mode/elevacao, ou permission-denied generico em qualquer plataforma. Nao
// casa por GOOS/plataforma, so pelo erro subjacente.
func isSymlinkPrivilegeError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == 1314 {
		return true
	}
	return false
}
