package integrations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kgsaran/trackfw/internal/identity"
)

// Render converts a canonical catalog item to the native representation
// declared by a target surface. When cfg carries a customized identity for
// item.ID, the rendered name/description/body are personalized (ADR
// ADR-2026-07-25-identidade-personalizavel-de-agentes, seções D1/D2/D6).
//
// Render has two exit routes:
//   - Rota A: "custom-agent-toml", "cli-agent-json", "agent-json" and
//     "agent-directory" work from name/description/body already split out of
//     the frontmatter by markdownParts.
//   - Rota B: the default branch, used by the "subagent" representation
//     (claude, gemini, cursor, copilot, kiro-ide, windsurf), returns the raw
//     normalized source with the frontmatter still attached.
//
// Both routes must receive the identity injection. When there is no
// identity configured for item.ID, name/description/body are left exactly
// as markdownParts produced them and the default branch returns
// normalizeMarkdown(source) verbatim — the same expression used before
// identity support existed — so the no-identity output is guaranteed
// byte-for-byte unchanged by construction, not by coincidence.
func Render(item Item, kind ItemKind, capability Capability, source []byte, cfg identity.Config) ([]byte, error) {
	if kind == KindSkills {
		return normalizeMarkdown(source), nil
	}

	id, hasIdentity := identity.Lookup(cfg, item.ID)

	name, description, model, body := markdownParts(source)
	var greeting string
	if hasIdentity {
		greeting = greetingLine(id, cfg.UserNickname)
		name = identity.AgentName(id.Slug)
		description = id.DisplayName + " — " + description
		body = greeting + "\n\n" + body
	}

	switch capability.Representation {
	case "custom-agent-toml":
		return []byte(fmt.Sprintf("name = %s\ndescription = %s\ndeveloper_instructions = %s\n",
			strconv.Quote(strings.ReplaceAll(name, "-", "_")), strconv.Quote(description), strconv.Quote(body))), nil
	case "cli-agent-json", "agent-json":
		data, err := json.MarshalIndent(map[string]string{
			"name":        name,
			"description": description,
			"prompt":      body,
		}, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("render %s as JSON: %w", item.ID, err)
		}
		return append(data, '\n'), nil
	case "agent-directory":
		// Reconstrói o frontmatter para o Antigravity CLI:
		// - mapeia model canônico para o valor aceito (opus→pro, sonnet→flash)
		// - injeta tools: SET_IMPL ou SET_ARCH dependendo do item.ID (não do
		//   nome renderizado, que pode ser customizado pela identidade — ADR D8)
		// - omite campos não suportados pelo agy
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString("name: " + name + "\n")
		sb.WriteString("description: " + description + "\n")
		if mapped, ok := mapModel(model); ok {
			sb.WriteString("model: " + mapped + "\n")
		}
		sb.WriteString("tools:\n")
		for _, tool := range agentTools(item.ID) {
			sb.WriteString("  - " + tool + "\n")
		}
		sb.WriteString("---\n")
		if body != "" {
			sb.WriteString(body + "\n")
		}
		return []byte(sb.String()), nil
	default:
		if !hasIdentity {
			return normalizeMarkdown(source), nil
		}
		return normalizeMarkdown(insertBodyPrefix(source, greeting)), nil
	}
}

// greetingLine builds the first line injected into the agent body when an
// identity is configured for the agent. With no nickname configured, only
// the agent's display name is mentioned.
func greetingLine(id identity.AgentIdentity, nickname string) string {
	if nickname == "" {
		return fmt.Sprintf("Você é %s.", id.DisplayName)
	}
	return fmt.Sprintf("Você é %s. Trate o usuário como %s.", id.DisplayName, nickname)
}

// insertBodyPrefix inserts prefix as the new first line of the body section
// of a raw markdown source (frontmatter + body), followed by a blank line.
// If source has no recognizable frontmatter, prefix is inserted at the very
// top. This reuses the same frontmatter-boundary detection as markdownParts
// so Rota A and Rota B agree on where the body starts.
func insertBodyPrefix(source []byte, prefix string) []byte {
	trimmed := strings.TrimSpace(string(source))
	if prefix == "" {
		return []byte(trimmed)
	}
	if !strings.HasPrefix(trimmed, "---\n") {
		return []byte(prefix + "\n\n" + trimmed)
	}
	end := strings.Index(trimmed[4:], "\n---")
	if end < 0 {
		return []byte(prefix + "\n\n" + trimmed)
	}
	insertAt := 4 + end + 4
	head := trimmed[:insertAt]
	rest := strings.TrimLeft(trimmed[insertAt:], "\n")
	if rest == "" {
		return []byte(head + "\n\n" + prefix)
	}
	return []byte(head + "\n\n" + prefix + "\n\n" + rest)
}

func normalizeMarkdown(source []byte) []byte {
	return []byte(strings.TrimSpace(string(source)) + "\n")
}

// markdownParts extrai name, description, model e body do frontmatter YAML delimitado por ---.
func markdownParts(source []byte) (name, description, model, body string) {
	text := strings.TrimSpace(string(source))
	name = "trackfw-agent"
	description = "trackfw specialist"
	body = text
	if !strings.HasPrefix(text, "---\n") {
		return
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return
	}
	frontmatter := text[4 : 4+end]
	body = strings.TrimSpace(text[4+end+4:])
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			description = value
		case "model":
			model = value
		}
	}
	return
}

// frontmatterName extrai apenas o campo "name" de um frontmatter YAML
// delimitado por ---, sem os valores default aplicados por markdownParts.
// Retorna ok=false quando o arquivo não tem frontmatter reconhecível ou não
// declara "name". Usado pela detecção de colisão em manager.go.
func frontmatterName(source []byte) (name string, ok bool) {
	text := strings.TrimSpace(string(source))
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", false
	}
	frontmatter := text[4 : 4+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, cut := strings.Cut(line, ":")
		if !cut {
			continue
		}
		if strings.TrimSpace(key) != "name" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}

// mapModel converte o modelo canônico para o valor aceito pelo Antigravity CLI.
// Retorna (valor mapeado, true) se mapeável; ("", false) se a linha model deve ser omitida.
func mapModel(model string) (string, bool) {
	switch model {
	case "opus":
		return "pro", true
	case "sonnet":
		return "flash", true
	case "flash_lite", "flash", "pro":
		return model, true
	default:
		return "", false
	}
}

// agentTools retorna o conjunto de ferramentas para o agente.
// A decisão é feita pelo item.ID canônico do catálogo (ex.: "architect"),
// não pelo nome renderizado — que pode ser customizado por identidade
// (ex.: "zeus-tf") e não deve influenciar a seleção do toolset (ADR D8).
// O ID "architect" recebe SET_ARCH (14 tools); os demais recebem SET_IMPL
// (10 tools). IDs proibidos (edit_file, read_file, find, view_code_item,
// view_file_outline, call_mcp_tool) nunca são emitidos.
func agentTools(itemID string) []string {
	// SET_IMPL — conjunto base de 10 ferramentas
	setImpl := []string{
		"view_file",
		"list_dir",
		"grep_search",
		"search_web",
		"read_url_content",
		"write_to_file",
		"replace_file_content",
		"run_command",
		"command_status",
		"generate_image",
	}
	if itemID == "architect" {
		// SET_ARCH — SET_IMPL + 4 ferramentas de orquestração
		return append(setImpl,
			"send_message",
			"define_subagent",
			"invoke_subagent",
			"schedule",
		)
	}
	return setImpl
}
