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

	name, description, body := markdownParts(source)
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
	default:
		return normalizeMarkdown(source), nil
	}
}

func normalizeMarkdown(source []byte) []byte {
	return []byte(strings.TrimSpace(string(source)) + "\n")
}

func markdownParts(source []byte) (name, description, body string) {
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
		}
	}
	return
}
