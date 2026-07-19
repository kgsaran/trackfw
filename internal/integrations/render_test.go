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

func TestRenderAgentDirectory(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	cap := Capability{Representation: "agent-directory"}

	// IDs proibidos — nunca devem aparecer no output
	forbidden := []string{
		"edit_file", "read_file", "find",
		"view_code_item", "view_file_outline", "call_mcp_tool",
	}

	t.Run("architect usa SET_ARCH e mapeia opus→pro", func(t *testing.T) {
		item, ok := catalog.Item(KindAgents, "architect")
		if !ok {
			t.Fatal("agente 'architect' não encontrado no catalog")
		}
		source, err := catalog.ReadAsset(item)
		if err != nil {
			t.Fatal(err)
		}

		out, err := Render(item, KindAgents, cap, source)
		if err != nil {
			t.Fatal(err)
		}
		output := string(out)

		// model mapeado corretamente
		if !strings.Contains(output, "model: pro") {
			t.Errorf("esperado 'model: pro', output:\n%s", output)
		}
		// modelo original não deve aparecer
		if strings.Contains(output, "opus") {
			t.Errorf("'opus' não deve aparecer no output:\n%s", output)
		}

		// SET_ARCH: todos os 14 tools
		archTools := []string{
			"view_file", "list_dir", "grep_search", "search_web",
			"read_url_content", "write_to_file", "replace_file_content",
			"run_command", "command_status", "generate_image",
			"send_message", "define_subagent", "invoke_subagent", "schedule",
		}
		for _, tool := range archTools {
			if !strings.Contains(output, "  - "+tool) {
				t.Errorf("tool '%s' ausente no output do architect:\n%s", tool, output)
			}
		}

		// IDs proibidos
		for _, id := range forbidden {
			if strings.Contains(output, id) {
				t.Errorf("ID proibido '%s' presente no output:\n%s", id, output)
			}
		}
	})

	t.Run("backend usa SET_IMPL e mapeia sonnet→flash", func(t *testing.T) {
		item, ok := catalog.Item(KindAgents, "backend")
		if !ok {
			t.Fatal("agente 'backend' não encontrado no catalog")
		}
		source, err := catalog.ReadAsset(item)
		if err != nil {
			t.Fatal(err)
		}

		out, err := Render(item, KindAgents, cap, source)
		if err != nil {
			t.Fatal(err)
		}
		output := string(out)

		// model mapeado corretamente
		if !strings.Contains(output, "model: flash") {
			t.Errorf("esperado 'model: flash', output:\n%s", output)
		}
		// modelo original não deve aparecer
		if strings.Contains(output, "sonnet") {
			t.Errorf("'sonnet' não deve aparecer no output:\n%s", output)
		}

		// SET_IMPL: 10 tools
		implTools := []string{
			"view_file", "list_dir", "grep_search", "search_web",
			"read_url_content", "write_to_file", "replace_file_content",
			"run_command", "command_status", "generate_image",
		}
		for _, tool := range implTools {
			if !strings.Contains(output, "  - "+tool) {
				t.Errorf("tool '%s' ausente no output do backend:\n%s", tool, output)
			}
		}

		// define_subagent não deve aparecer no SET_IMPL
		if strings.Contains(output, "define_subagent") {
			t.Errorf("'define_subagent' não deve aparecer no output do backend:\n%s", output)
		}

		// IDs proibidos
		for _, id := range forbidden {
			if strings.Contains(output, id) {
				t.Errorf("ID proibido '%s' presente no output:\n%s", id, output)
			}
		}
	})
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
