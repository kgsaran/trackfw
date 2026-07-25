package integrations

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

func TestRenderNativeAgentFormats(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, _ := catalog.Item(KindAgents, "backend")
	source, _ := catalog.ReadAsset(item)

	toml, err := Render(item, KindAgents, Capability{Representation: "custom-agent-toml"}, source, identity.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toml), `name = "trackfw_backend"`) || !strings.Contains(string(toml), "developer_instructions =") {
		t.Fatalf("unexpected Codex TOML:\n%s", toml)
	}

	jsonAgent, err := Render(item, KindAgents, Capability{Representation: "agent-json"}, source, identity.Config{})
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

		out, err := Render(item, KindAgents, cap, source, identity.Config{})
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

		out, err := Render(item, KindAgents, cap, source, identity.Config{})
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

// --- Goldens congelados (internal/integrations/testdata/) ---
//
// Estes goldens foram capturados ANTES da injeção de identidade existir
// (commit 5fe5cb9 e npm/tests/agents-skills.test.js) e são lidos do disco de
// forma independente do asset embedado que Render também consome. Isso evita
// a lacuna de "Render(x) == Render(x)" — a suite compara Render contra um
// contrato externo congelado, não contra si mesma.

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return data
}

// zeusIdentity retorna uma identidade customizada para "architect" (Zeus/zeus)
// com apelido "chefe" para o usuário, usada pelos testes de injeção abaixo.
func zeusIdentity() identity.Config {
	return identity.Config{
		SchemaVersion: 1,
		UserNickname:  "chefe",
		Agents: map[string]identity.AgentIdentity{
			"architect": {DisplayName: "Zeus", Slug: "zeus"},
		},
	}
}

func TestRenderWithoutIdentityMatchesFrozenGoldens(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		itemID     string
		capability Capability
		golden     string
	}{
		{"subagent/architect", "architect", Capability{Representation: "subagent"}, "architect.subagent.golden.md"},
		{"custom-agent-toml/backend", "backend", Capability{Representation: "custom-agent-toml"}, "backend.codex-toml.golden.toml"},
		{"agent-directory/architect", "architect", Capability{Representation: "agent-directory"}, "architect.agent-directory.golden.md"},
		{"agent-directory/backend", "backend", Capability{Representation: "agent-directory"}, "backend.agent-directory.golden.md"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item, ok := catalog.Item(KindAgents, tc.itemID)
			if !ok {
				t.Fatalf("item %q não encontrado no catalog", tc.itemID)
			}
			source, err := catalog.ReadAsset(item)
			if err != nil {
				t.Fatal(err)
			}
			out, err := Render(item, KindAgents, tc.capability, source, identity.Config{})
			if err != nil {
				t.Fatal(err)
			}
			want := readGolden(t, tc.golden)
			if string(out) != string(want) {
				t.Fatalf("Render sem identidade diverge do golden congelado %s:\n--- got ---\n%s\n--- want ---\n%s", tc.golden, out, want)
			}
		})
	}
}

// TestRenderSubagentRouteInjectsIdentity é o teste que a tentativa anterior
// não tinha: prova que a Rota B (branch default, representation "subagent",
// usada pela superfície claude) recebe a injeção de identidade — e não
// apenas a Rota A (custom-agent-toml/cli-agent-json/agent-json/agent-directory).
func TestRenderSubagentRouteInjectsIdentity(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := catalog.Item(KindAgents, "architect")
	if !ok {
		t.Fatal("agente 'architect' não encontrado no catalog")
	}
	source, err := catalog.ReadAsset(item)
	if err != nil {
		t.Fatal(err)
	}

	out, err := Render(item, KindAgents, Capability{Representation: "subagent"}, source, zeusIdentity())
	if err != nil {
		t.Fatal(err)
	}
	output := string(out)

	if !strings.Contains(output, "Você é Zeus. Trate o usuário como chefe.") {
		t.Fatalf("saudação de identidade ausente no corpo da Rota B:\n%s", output)
	}
	// frontmatter original preservado, corpo original ("# Architect...") ainda presente
	if !strings.Contains(output, "name: trackfw-architect") {
		t.Fatalf("frontmatter original não preservado na Rota B:\n%s", output)
	}
	if !strings.Contains(output, "# Architect") {
		t.Fatalf("corpo original perdido na Rota B:\n%s", output)
	}
	if strings.Contains(output, "{{") {
		t.Fatalf("placeholder não substituído vazou na saída:\n%s", output)
	}
}

func TestRenderInjectsCustomNameAndDescription(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := catalog.Item(KindAgents, "architect")
	if !ok {
		t.Fatal("agente 'architect' não encontrado no catalog")
	}
	source, err := catalog.ReadAsset(item)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		capability Capability
	}{
		{"custom-agent-toml", Capability{Representation: "custom-agent-toml"}},
		{"cli-agent-json", Capability{Representation: "cli-agent-json"}},
		{"agent-directory", Capability{Representation: "agent-directory"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Render(item, KindAgents, tc.capability, source, zeusIdentity())
			if err != nil {
				t.Fatal(err)
			}
			output := string(out)
			wantName := "zeus-tf"
			if tc.capability.Representation == "custom-agent-toml" {
				// custom-agent-toml substitui "-" por "_" no name (comportamento
				// preexistente, preservado para nomes customizados).
				wantName = "zeus_tf"
			}
			if !strings.Contains(output, wantName) {
				t.Errorf("nome customizado %q ausente:\n%s", wantName, output)
			}
			if !strings.Contains(output, "Zeus — ") {
				t.Errorf("descrição não prefixada com 'Zeus — ':\n%s", output)
			}
			if strings.Contains(output, "{{") {
				t.Errorf("placeholder não substituído vazou na saída:\n%s", output)
			}
		})
	}
}

func TestRenderCustomNameDoesNotAffectArchitectToolset(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := catalog.Item(KindAgents, "architect")
	if !ok {
		t.Fatal("agente 'architect' não encontrado no catalog")
	}
	source, err := catalog.ReadAsset(item)
	if err != nil {
		t.Fatal(err)
	}

	out, err := Render(item, KindAgents, Capability{Representation: "agent-directory"}, source, zeusIdentity())
	if err != nil {
		t.Fatal(err)
	}
	output := string(out)

	if !strings.Contains(output, "name: zeus-tf") {
		t.Fatalf("esperado name customizado 'zeus-tf' no agent-directory:\n%s", output)
	}
	archTools := []string{
		"view_file", "list_dir", "grep_search", "search_web",
		"read_url_content", "write_to_file", "replace_file_content",
		"run_command", "command_status", "generate_image",
		"send_message", "define_subagent", "invoke_subagent", "schedule",
	}
	for _, tool := range archTools {
		if !strings.Contains(output, "  - "+tool) {
			t.Errorf("tool '%s' ausente — o toolset SET_ARCH não deveria depender do name customizado:\n%s", tool, output)
		}
	}
}

func TestRenderNoLeakedPlaceholders(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("agente 'backend' não encontrado no catalog")
	}
	source, err := catalog.ReadAsset(item)
	if err != nil {
		t.Fatal(err)
	}

	representations := []string{"custom-agent-toml", "cli-agent-json", "agent-json", "agent-directory", "subagent"}
	for _, representation := range representations {
		for _, cfg := range []identity.Config{{}, zeusIdentityFor("backend")} {
			out, err := Render(item, KindAgents, Capability{Representation: representation}, source, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(out), "{{") {
				t.Fatalf("placeholder vazou em representation=%s cfg=%#v:\n%s", representation, cfg, out)
			}
		}
	}
}

// zeusIdentityFor constrói uma identidade customizada mínima para o item
// informado, reutilizando o mesmo nickname de zeusIdentity().
func zeusIdentityFor(itemID string) identity.Config {
	return identity.Config{
		SchemaVersion: 1,
		UserNickname:  "chefe",
		Agents: map[string]identity.AgentIdentity{
			itemID: {DisplayName: "Zeus", Slug: "zeus"},
		},
	}
}
