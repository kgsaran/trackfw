package integrations

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderNativeAgentFormats(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, _ := catalog.Item(KindAgents, "backend")
	source, _ := catalog.ReadAsset(item)

	toml, err := Render(item, KindAgents, Capability{Representation: "custom-agent-toml"}, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), `name = "trackfw_backend"`) || !strings.Contains(string(toml), "developer_instructions =") {
		t.Fatalf("unexpected Codex TOML:\n%s", toml)
	}

	jsonAgent, err := Render(item, KindAgents, Capability{Representation: "agent-json"}, source)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(jsonAgent, &decoded); err != nil {
		t.Fatalf("invalid native JSON: %v", err)
	}
	if decoded["name"] != "trackfw-backend" || decoded["prompt"] == "" {
		t.Fatalf("unexpected native JSON: %#v", decoded)
	}
}

func TestBuildPlansDefaultsToFirstNonLegacySurface(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlans(catalog, PlanRequest{
		Kind: KindAgents, Targets: []string{"antigravity"}, Items: []string{"architect"}, Scope: "project",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Claim.Surface != "current" {
		t.Fatalf("expected current non-legacy surface, got %#v", plans)
	}
}
