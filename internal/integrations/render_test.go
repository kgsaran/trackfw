package integrations

import (
	"encoding/json"
	"os"
	"strconv"
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

// TestRenderJSONRepresentationsDoNotHTMLEscape prova que "cli-agent-json" e
// "agent-json" não aplicam o HTML-escaping padrão de encoding/json (<, >, &
// virando <, >, &) — comportamento que diverge de Node.js
// (JSON.stringify) e Python (json.dumps), nenhum dos quais escapa esses
// caracteres por padrão. Ver check-identity-parity.sh e o "Dispatch contract"
// do papel Architect, cujo placeholder literal "<slug>" expunha a divergência.
func TestRenderJSONRepresentationsDoNotHTMLEscape(t *testing.T) {
	source := []byte("---\n" +
		"name: trackfw-architect\n" +
		"description: Principal software architect for <slug> & friends.\n" +
		"model: opus\n" +
		"---\n\n" +
		"# Architect\n\n" +
		"Dispatch usa o valor de <slug> & outras convenções > baseline.\n")

	item := Item{ID: "architect"}

	for _, representation := range []string{"cli-agent-json", "agent-json"} {
		t.Run(representation, func(t *testing.T) {
			out, err := Render(item, KindAgents, Capability{Representation: representation}, source, identity.Config{})
			if err != nil {
				t.Fatal(err)
			}
			output := string(out)

			for _, unicodeEscape := range []string{"\\u003c", "\\u003e", "\\u0026"} {
				if strings.Contains(output, unicodeEscape) {
					t.Fatalf("saída de %s não deve conter o HTML-escaping %s (comportamento default de encoding/json, ausente em Node.js/Python):\n%s", representation, unicodeEscape, output)
				}
			}
			for _, literal := range []string{"<slug>", "&", ">"} {
				if !strings.Contains(output, literal) {
					t.Fatalf("saída de %s deve conter o caractere literal %q sem escape:\n%s", representation, literal, output)
				}
			}

			var decoded map[string]string
			if err := json.Unmarshal(out, &decoded); err != nil {
				t.Fatalf("saída de %s deve ser JSON válido: %v\n%s", representation, err, output)
			}
			if decoded["description"] != "Principal software architect for <slug> & friends." {
				t.Fatalf("%s: description decodificada divergiu: %q", representation, decoded["description"])
			}
		})
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
//
// Re-congelados em 2026-07-26 pela REQ-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw:
// os 10 assets de agente receberam a camada universal de harness (memory: project, tools:,
// blocos Mode lock / Before you act / Scope boundary / Working context / Knowledge vault e
// linha de assinatura). A propriedade "saída sem identidade == contrato congelado externo"
// foi preservada — os goldens refletem o novo conteúdo deliberadamente revisado, não
// uma cópia automática da saída.
//
// Re-congelados em 2026-07-26 (Wave 2) pela REQ-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw:
// os 10 assets receberam o adendo do orquestrador (Git authority, Parallelization, Workflow,
// Post-microbatch audit) em architect e o adendo do implementador (Governance prerequisite,
// Git boundary, Microbatch completion protocol, Definition of done) nos 6 agents com Edit/Write,
// e Reporting boundary nos 3 read-only (security, code-quality, ux). Todos receberam ## Mission.
//
// Wave 5 (2026-07-26) pela REQ-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw:
// iac.md e tooling.md tiveram descriptions enriquecidas sob a emenda D12-bis (vocabulário de
// domínio como Terraform/Pulumi/MCP); assets architect e backend (escopo dos goldens) inalterados.
// greetingLine migrada de PT-BR ("Você é/Trate o usuário") para EN ("You are/Address the user")
// nos 3 CLIs por coerência com D2 do ADR de convergência. Goldens permanecem válidos.
//
// Re-congelados em 2026-07-29 pela REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador
// (ML-3A): architect.md ganhou o parágrafo revisado de "## Git authority" (agora cobrindo commit
// de código de produto, já que especialistas não commitam mais) e a nova seção "## Barrier
// protocol". backend.md trocou "## Git boundary" (permitia commit/push na branch do orquestrador)
// por "## Git authority" (nenhuma operação Git; atua somente por handoff de trackfw_architect).
//
// Re-congelado em 2026-08-04 pela REQ-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect:
// architect.md ganhou a nova seção "## Dispatch contract" entre "## Workflow" e "## Post-microbatch
// audit", tornando explícito que nomear um especialista em prosa/`squad:` não roteia a chamada da
// Agent tool — todo dispatch exige o parâmetro `subagent_type`, cujo valor correto é o `name:` do
// agente instalado do role-alvo (identity-agnostic, nunca um nome fixo).

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

	if !strings.Contains(output, "You are Zeus. Address the user as chefe.") {
		t.Fatalf("saudação de identidade ausente no corpo da Rota B:\n%s", output)
	}
	// frontmatter reescrito com o name/description customizados: @agent-zeus-tf
	// e o roteamento por linguagem natural dependem disso, pois a seleção de
	// subagent do Claude Code lê exclusivamente name/description do
	// frontmatter (nunca o corpo).
	if !strings.Contains(output, "name: zeus-tf") {
		t.Fatalf("frontmatter não reescrito com o name customizado na Rota B:\n%s", output)
	}
	if strings.Contains(output, "name: trackfw-architect") {
		t.Fatalf("name original vazou no frontmatter da Rota B:\n%s", output)
	}
	if !strings.Contains(output, "description: Zeus — ") {
		t.Fatalf("description não reescrita com prefixo do display name na Rota B:\n%s", output)
	}
	// o modelo original (fora do escopo da identidade) deve permanecer intacto
	if !strings.Contains(output, "model: opus") {
		t.Fatalf("linha model: preservada incorretamente na Rota B:\n%s", output)
	}
	if !strings.Contains(output, "# Architect") {
		t.Fatalf("corpo original perdido na Rota B:\n%s", output)
	}
	if strings.Contains(output, "{{") {
		t.Fatalf("placeholder não substituído vazou na saída:\n%s", output)
	}
}

// TestRenderSubagentRouteFrontmatterRewriteIsScoped prova que a reescrita de
// name/description na Rota B é restrita ao bloco de frontmatter: uma linha
// começando com "name:" dentro do corpo do agente não pode ser alterada, e
// as demais linhas do frontmatter (ex.: model:) devem ser preservadas byte a
// byte, na mesma ordem.
func TestRenderSubagentRouteFrontmatterRewriteIsScoped(t *testing.T) {
	source := []byte("---\n" +
		"name: trackfw-architect\n" +
		"description: Principal software architect.\n" +
		"model: opus\n" +
		"---\n\n" +
		"# Architect\n\n" +
		"Exemplo de convenção:\n" +
		"name: minha-variavel-local\n" +
		"Fim.\n")

	item := Item{ID: "architect"}
	out, err := Render(item, KindAgents, Capability{Representation: "subagent"}, source, zeusIdentity())
	if err != nil {
		t.Fatal(err)
	}
	output := string(out)

	if !strings.Contains(output, "name: zeus-tf") {
		t.Fatalf("name do frontmatter não reescrito:\n%s", output)
	}
	if !strings.Contains(output, "model: opus\n") {
		t.Fatalf("linha model: do frontmatter não preservada:\n%s", output)
	}
	if !strings.Contains(output, "name: minha-variavel-local") {
		t.Fatalf("linha 'name:' do CORPO foi alterada indevidamente:\n%s", output)
	}
	// só deve haver uma ocorrência de "name: zeus-tf" (no frontmatter); a
	// linha do corpo continua com seu valor original, não com o slug.
	if strings.Count(output, "name: zeus-tf") != 1 {
		t.Fatalf("esperada exatamente 1 ocorrência de 'name: zeus-tf' (só no frontmatter):\n%s", output)
	}
}

// TestRenderAllRepresentationsRenderIdentityName é a tabela que garante que
// TODAS as representações — incluindo a Rota B ("subagent" e demais
// superfícies que usam o branch default) — derivam o name renderizado do
// slug da identidade configurada. A transformação "-" → "_" do
// custom-agent-toml é comportamento preexistente e intencional (ADR
// identidade-personalizavel), documentada aqui como esperada, não corrigida.
// Este teste é o guarda contra uma representação futura ficar para trás
// silenciosamente quando uma nova rota for adicionada a Render.
func TestRenderAllRepresentationsRenderIdentityName(t *testing.T) {
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
		representation string
		wantName       string
		extract        func(t *testing.T, out []byte) string
	}{
		{
			representation: "subagent",
			wantName:       "zeus-tf",
			extract: func(t *testing.T, out []byte) string {
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "name:") {
						return strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					}
				}
				return ""
			},
		},
		{
			representation: "agent-directory",
			wantName:       "zeus-tf",
			extract: func(t *testing.T, out []byte) string {
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "name:") {
						return strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					}
				}
				return ""
			},
		},
		{
			representation: "agent-json",
			wantName:       "zeus-tf",
			extract: func(t *testing.T, out []byte) string {
				var decoded map[string]string
				if err := json.Unmarshal(out, &decoded); err != nil {
					t.Fatalf("invalid agent-json: %v", err)
				}
				return decoded["name"]
			},
		},
		{
			representation: "cli-agent-json",
			wantName:       "zeus-tf",
			extract: func(t *testing.T, out []byte) string {
				var decoded map[string]string
				if err := json.Unmarshal(out, &decoded); err != nil {
					t.Fatalf("invalid cli-agent-json: %v", err)
				}
				return decoded["name"]
			},
		},
		{
			representation: "custom-agent-toml",
			// comportamento preexistente e intencional: "-" → "_" no TOML.
			wantName: "zeus_tf",
			extract: func(t *testing.T, out []byte) string {
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "name = ") {
						unquoted, err := strconv.Unquote(strings.TrimPrefix(line, "name = "))
						if err != nil {
							t.Fatalf("failed to unquote toml name: %v", err)
						}
						return unquoted
					}
				}
				return ""
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.representation, func(t *testing.T) {
			out, err := Render(item, KindAgents, Capability{Representation: tc.representation}, source, zeusIdentity())
			if err != nil {
				t.Fatal(err)
			}
			got := tc.extract(t, out)
			if got != tc.wantName {
				t.Fatalf("representation=%s: name renderizado = %q, esperado %q\noutput:\n%s", tc.representation, got, tc.wantName, out)
			}
		})
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

// --- Testes de rewriteSignatureLine ---

// TestRewriteSignatureLineBasic verifica a substituição do nome na última
// linha de assinatura que casa com o padrão.
func TestRewriteSignatureLineBasic(t *testing.T) {
	source := []byte("---\nname: trackfw-architect\n---\n\n# Corpo\n\nAlgum texto.\n\n— Architect, Principal Software Architect\n")
	got := rewriteSignatureLine(source, "Zeus")
	want := "— Zeus, Principal Software Architect"
	if !strings.Contains(string(got), want) {
		t.Fatalf("esperado %q na saída:\n%s", want, got)
	}
	if strings.Contains(string(got), "— Architect, Principal Software Architect") {
		t.Fatalf("nome original não foi substituído:\n%s", got)
	}
	// título preservado byte a byte
	if !strings.Contains(string(got), "Principal Software Architect") {
		t.Fatalf("título não preservado:\n%s", got)
	}
}

// TestRewriteSignatureLineNoMatch verifica que source é retornado inalterado
// quando nenhuma linha casa com o padrão de assinatura.
func TestRewriteSignatureLineNoMatch(t *testing.T) {
	source := []byte("---\nname: trackfw-architect\n---\n\n# Corpo\n\nNenhuma assinatura aqui.\n")
	got := rewriteSignatureLine(source, "Zeus")
	if string(got) != string(source) {
		t.Fatalf("sem assinatura: source deve ser retornado inalterado\ngot: %q\nwant: %q", got, source)
	}
}

// TestRewriteSignatureLineInFrontmatter garante que uma linha com padrão de
// assinatura dentro do frontmatter NÃO é tocada — apenas o corpo é varrido.
func TestRewriteSignatureLineInFrontmatter(t *testing.T) {
	// A linha "— Architect, Principal Software Architect" está no frontmatter
	// (entre os delimitadores ---); deve ser ignorada.
	source := []byte("---\nname: trackfw-architect\ndescription: — Architect, Principal Software Architect\n---\n\n# Corpo sem assinatura.\n")
	got := rewriteSignatureLine(source, "Zeus")
	// source inalterado (sem assinatura no corpo)
	if string(got) != string(source) {
		t.Fatalf("linha no frontmatter não deve ser reescrita:\ngot: %q\nwant: %q", got, source)
	}
}

// TestRewriteSignatureLineLastWins verifica que quando há múltiplas linhas
// candidatas no corpo, APENAS a última é reescrita.
func TestRewriteSignatureLineLastWins(t *testing.T) {
	source := []byte("---\nname: trackfw-architect\n---\n\n— Architect, Senior Role\n\nTexto.\n\n— Architect, Principal Software Architect\n")
	got := rewriteSignatureLine(source, "Zeus")
	output := string(got)
	// A última linha foi reescrita
	if !strings.Contains(output, "— Zeus, Principal Software Architect") {
		t.Fatalf("última assinatura não reescrita:\n%s", output)
	}
	// A primeira linha permanece inalterada
	if !strings.Contains(output, "— Architect, Senior Role") {
		t.Fatalf("primeira assinatura não deve ser alterada:\n%s", output)
	}
}

// TestRewriteSignatureLineEmptyDisplayName verifica que source é retornado
// inalterado quando displayName é vazio.
func TestRewriteSignatureLineEmptyDisplayName(t *testing.T) {
	source := []byte("---\nname: trackfw-architect\n---\n\n# Corpo\n\n— Architect, Principal Software Architect\n")
	got := rewriteSignatureLine(source, "")
	if string(got) != string(source) {
		t.Fatalf("displayName vazio: source deve ser retornado inalterado\ngot: %q\nwant: %q", got, source)
	}
}

// TestRenderSubagentRouteRewritesSignatureLine é o teste de integração que
// prova que a Rota B (representation "subagent") reescreve a assinatura quando
// há identidade configurada. Usa source inline para não depender dos assets
// reais, que ainda não têm linha de assinatura (ela será adicionada no ML-1A).
func TestRenderSubagentRouteRewritesSignatureLine(t *testing.T) {
	source := []byte("---\n" +
		"name: trackfw-architect\n" +
		"description: Principal software architect.\n" +
		"model: opus\n" +
		"---\n\n" +
		"# Architect\n\n" +
		"Corpo do agente.\n\n" +
		"— Architect, Principal Software Architect\n")

	item := Item{ID: "architect"}
	out, err := Render(item, KindAgents, Capability{Representation: "subagent"}, source, zeusIdentity())
	if err != nil {
		t.Fatal(err)
	}
	output := string(out)

	if !strings.Contains(output, "— Zeus, Principal Software Architect") {
		t.Fatalf("assinatura não reescrita com identidade configurada:\n%s", output)
	}
	if strings.Contains(output, "— Architect, Principal Software Architect") {
		t.Fatalf("assinatura original vazou na saída:\n%s", output)
	}
	// título preservado
	if !strings.Contains(output, "Principal Software Architect") {
		t.Fatalf("título da assinatura não preservado:\n%s", output)
	}
}
