package integrations

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoadCatalogHasCanonicalInventory(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if got, want := len(catalog.Agents), 10; got != want {
		t.Fatalf("agents count = %d, want %d", got, want)
	}
	if got, want := len(catalog.Skills), 5; got != want {
		t.Fatalf("skills count = %d, want %d", got, want)
	}
	if got, want := len(catalog.Targets), 9; got != want {
		t.Fatalf("targets count = %d, want %d", got, want)
	}

	assertIDs(t, catalog.Agents, []string{"architect", "backend", "frontend", "qa", "infra", "security", "code-quality", "dba", "ux", "data"})
	assertIDs(t, catalog.Skills, []string{"governance", "plan", "implement", "review", "release"})

	for _, id := range []string{"claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "kiro"} {
		if _, ok := catalog.Target(id); !ok {
			t.Errorf("target %q not found", id)
		}
	}
}

func TestLoadCatalogAssetsAreEmbedded(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ItemKind{KindAgents, KindSkills} {
		for _, item := range catalog.Items(kind) {
			content, err := catalog.ReadAsset(item)
			if err != nil {
				t.Errorf("ReadAsset(%s/%s) error = %v", kind, item.ID, err)
				continue
			}
			if len(content) == 0 || !strings.Contains(string(content), "name: trackfw-") {
				t.Errorf("asset %s/%s is empty or lacks canonical frontmatter", kind, item.ID)
			}
		}
	}
}

func TestCatalogDeclaresFallbackForNonNativeCapabilities(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range catalog.Targets {
		for _, surface := range target.Surfaces {
			for kind, capability := range map[ItemKind]Capability{KindAgents: surface.Capabilities.Agents, KindSkills: surface.Capabilities.Skills} {
				if capability.SupportLevel == "fallback" && capability.FallbackRepresentation == "" {
					t.Errorf("target %s surface %s %s lacks fallback representation", target.ID, surface.ID, kind)
				}
			}
		}
	}
}

func TestCatalogModelsOfficialMultiSurfaceContracts(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	assertSurfacePath(t, catalog, "gemini", "cli", KindAgents, "project", ".gemini/agents/trackfw-{{id}}.md", "native")
	assertSurfacePath(t, catalog, "cursor", "ide", KindSkills, "project", ".cursor/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "antigravity", "current", KindAgents, "global", "~/.gemini/config/agents/trackfw-{{id}}/agent.md", "native")
	assertSurfacePath(t, catalog, "antigravity", "current", KindAgents, "project", ".agents/agents/trackfw-{{id}}/agent.md", "native")
	assertSurfacePath(t, catalog, "antigravity", "current", KindSkills, "global", "~/.gemini/config/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "antigravity", "legacy-cli", KindAgents, "global", "~/.gemini/antigravity-cli/agents/trackfw-{{id}}/agent.json", "legacy")
	assertSurfacePath(t, catalog, "antigravity", "legacy-cli", KindAgents, "project", ".agents/agents/trackfw-{{id}}/agent.json", "legacy")
	assertSurfacePath(t, catalog, "antigravity", "legacy-cli", KindSkills, "global", "~/.gemini/antigravity-cli/skills/trackfw-{{id}}/SKILL.md", "legacy")
	assertSurfacePath(t, catalog, "amazonq", "cli", KindAgents, "project", ".amazonq/cli-agents/trackfw-{{id}}.json", "native")
	assertSurfacePath(t, catalog, "amazonq", "cli", KindSkills, "project", ".amazonq/rules/trackfw-{{id}}.md", "fallback")
	assertSurfacePath(t, catalog, "windsurf", "ide", KindSkills, "project", ".windsurf/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "windsurf", "ide", KindAgents, "global", "~/.codeium/windsurf/skills/trackfw-agent-{{id}}/SKILL.md", "fallback")
	assertSurfacePath(t, catalog, "windsurf", "ide", KindSkills, "global", "~/.codeium/windsurf/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "copilot", "ide", KindAgents, "global", "~/.copilot/agents/trackfw-{{id}}.agent.md", "native")
	assertSurfacePath(t, catalog, "copilot", "ide", KindSkills, "global", "~/.copilot/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "kiro", "ide", KindAgents, "project", ".kiro/agents/trackfw-{{id}}.md", "native")
	assertSurfacePath(t, catalog, "kiro", "ide", KindAgents, "global", "~/.kiro/agents/trackfw-{{id}}.md", "native")
	assertSurfacePath(t, catalog, "kiro", "ide", KindSkills, "global", "~/.kiro/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "kiro", "cli", KindAgents, "project", ".kiro/agents/trackfw-{{id}}.json", "native")
	assertSurfacePath(t, catalog, "kiro", "cli", KindSkills, "global", "~/.kiro/skills/trackfw-{{id}}/SKILL.md", "native")
	assertSurfacePath(t, catalog, "kiro", "cli", KindSkills, "project", ".kiro/skills/trackfw-{{id}}/SKILL.md", "native")
}

func TestCatalogRejectsDuplicateTarget(t *testing.T) {
	catalog := clonedCatalog(t)
	catalog.Targets = append(catalog.Targets, catalog.Targets[0])
	assertValidationError(t, catalog, "duplicate target id")
}

func TestCatalogRejectsDuplicateItem(t *testing.T) {
	catalog := clonedCatalog(t)
	catalog.Skills[0].ID = catalog.Agents[0].ID
	assertValidationError(t, catalog, "duplicate item id")
}

func TestCatalogRejectsDuplicatePath(t *testing.T) {
	catalog := clonedCatalog(t)
	surface := &catalog.Targets[0].Surfaces[0]
	surface.Paths.Skills[0].Scope = surface.Paths.Agents[0].Scope
	surface.Paths.Skills[0].Path = surface.Paths.Agents[0].Path
	assertValidationError(t, catalog, "duplicate install path")
}

func TestCatalogRejectsUnsafePath(t *testing.T) {
	catalog := clonedCatalog(t)
	catalog.Targets[0].Surfaces[0].Paths.Agents[0].Path = "~/../trackfw-{{id}}.md"
	assertValidationError(t, catalog, "unsafe destination")
}

func assertSurfacePath(t *testing.T, catalog *Catalog, targetID, surfaceID string, kind ItemKind, scope, expectedPath, level string) {
	t.Helper()
	target, ok := catalog.Target(targetID)
	if !ok {
		t.Fatalf("target %q not found", targetID)
	}
	for _, surface := range target.Surfaces {
		if surface.ID != surfaceID {
			continue
		}
		capability := surface.Capabilities.Agents
		paths := surface.Paths.Agents
		if kind == KindSkills {
			capability = surface.Capabilities.Skills
			paths = surface.Paths.Skills
		}
		if capability.SupportLevel != level {
			t.Errorf("%s/%s %s support_level = %q, want %q", targetID, surfaceID, kind, capability.SupportLevel, level)
		}
		for _, installPath := range paths {
			if installPath.Scope == scope && installPath.Path == expectedPath {
				return
			}
		}
		t.Errorf("%s/%s %s path %q (%s) not found", targetID, surfaceID, kind, expectedPath, scope)
		return
	}
	t.Errorf("target %s surface %s not found", targetID, surfaceID)
}

func assertIDs(t *testing.T, items []Item, expected []string) {
	t.Helper()
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		seen[item.ID] = true
	}
	for _, id := range expected {
		if !seen[id] {
			t.Errorf("item %q not found", id)
		}
	}
}

func clonedCatalog(t *testing.T) *Catalog {
	t.Helper()
	original, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var clone Catalog
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return &clone
}

func assertValidationError(t *testing.T, catalog *Catalog, want string) {
	t.Helper()
	err := catalog.Validate()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() error = %v, want containing %q", err, want)
	}
}
