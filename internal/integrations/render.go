package integrations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Render converts a canonical catalog item to the native representation
// declared by a target surface.
func Render(item Item, kind ItemKind, capability Capability, source []byte) ([]byte, error) {
	if kind == KindSkills {
		return normalizeMarkdown(source), nil
	}

	name, description, model, body := markdownParts(source)
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
		// - injeta tools: SET_IMPL ou SET_ARCH dependendo do nome do agente
		// - omite campos não suportados pelo agy
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString("name: " + name + "\n")
		sb.WriteString("description: " + description + "\n")
		if mapped, ok := mapModel(model); ok {
			sb.WriteString("model: " + mapped + "\n")
		}
		sb.WriteString("tools:\n")
		for _, tool := range agentTools(name) {
			sb.WriteString("  - " + tool + "\n")
		}
		sb.WriteString("---\n")
		if body != "" {
			sb.WriteString(body + "\n")
		}
		return []byte(sb.String()), nil
	default:
		return normalizeMarkdown(source), nil
	}
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
// Agentes cujo nome termina com "architect" recebem SET_ARCH (14 tools),
// os demais recebem SET_IMPL (10 tools).
// IDs proibidos (edit_file, read_file, find, view_code_item, view_file_outline,
// call_mcp_tool) nunca são emitidos.
func agentTools(name string) []string {
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
	if strings.HasSuffix(name, "architect") {
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
