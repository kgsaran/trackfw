package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kgsaran/trackfw/internal/identity"
)

// isVersionString reports whether s is a bare version string (digits and dots
// only, e.g. "5", "4.6", "1.0.2"). Values that don't match — including
// pre-composed IDs like "claude-sonnet-4-5-20250929" (hyphens), "latest"
// (letters), or "" (empty) — return false and the caller uses the value
// literally (escape hatch, ADR-2026-08-21 §3).
//
// Trade-off: a value like "4.6-beta" (version-like but with a suffix) is
// treated as literal and produces an unchanged model line; a purely numeric
// string that happens not to be a real model version (e.g. "99") gets
// composed into "claude-tier-99" (a wrong-but-visible ID). We accept the
// first failure mode over the second: the literal path is recoverable (the
// user sees the exact string they typed); the silent-compose path silently
// produces a non-existent model ID without any indication of what went wrong.
func isVersionString(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// composeClaudeModelID builds a Claude model identifier from tier (e.g.
// "sonnet") and version (e.g. "4.6" or "5"), applying the three composition
// rules from the official Anthropic model documentation
// (ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md §2):
//
//  1. Dots in the version become hyphens: "4.6" → "4-6".
//  2. A major-only version never gets a trailing "-0": "5" → "claude-sonnet-5".
//  3. Pre-4.6 IDs include a date suffix; they must be stored as literal values
//     (escape hatch) and never reach this function.
//
// caller is responsible for calling isVersionString(version) before
// composeClaudeModelID and using the value literally when it returns false.
func composeClaudeModelID(tier, version string) string {
	hyphenated := strings.ReplaceAll(version, ".", "-")
	return "claude-" + tier + "-" + hyphenated
}

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
//     normalized source with the frontmatter still attached. When an
//     identity is configured, its "name:"/"description:" lines are rewritten
//     in place (see rewriteFrontmatterFields), and the last signature line in
//     the body matching `^— <name>, <title>$` is rewritten with the identity
//     display name (see rewriteSignatureLine) — required because Claude
//     Code's subagent selection reads only the frontmatter, never the body.
//
// Both routes must receive the identity injection. When there is no
// identity configured for item.ID, name/description/body are left exactly
// as markdownParts produced them and the default branch returns
// normalizeMarkdown(source) verbatim — the same expression used before
// identity support existed — so the no-identity output is guaranteed
// byte-for-byte unchanged by construction, not by coincidence.
//
// targetID is the canonical ID of the render target (e.g. "cursor",
// "gemini", "kiro", "codex") as declared in the target catalog.
//
// agentModels maps tier names (e.g. "sonnet", "opus") to version strings
// (e.g. "4.6", "5"). A nil or empty map means "no pin" — output is identical
// to before this parameter existed, by construction. Only the "claude" target
// receives composed model IDs; all other targets (Codex, Cursor, Antigravity,
// OpenCode, Gemini, Kiro, etc.) are unaffected even when agentModels is
// populated (ADR-2026-08-21 §4 — namespace boundary is a gate, not a cuidado).
func Render(item Item, kind ItemKind, capability Capability, source []byte, cfg identity.Config, targetID string, agentModels map[string]string) ([]byte, error) {
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
		lines := []string{
			fmt.Sprintf("name = %s", strconv.Quote(strings.ReplaceAll(name, "-", "_"))),
			fmt.Sprintf("description = %s", strconv.Quote(description)),
		}
		if mapped, ok := mapModelCodex(model); ok {
			lines = append(lines, fmt.Sprintf("model = %s", strconv.Quote(mapped)))
		}
		lines = append(lines, fmt.Sprintf("developer_instructions = %s", strconv.Quote(body)))
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	case "cli-agent-json", "agent-json":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]string{
			"name":        name,
			"description": description,
			"prompt":      body,
		}); err != nil {
			return nil, fmt.Errorf("render %s as JSON: %w", item.ID, err)
		}
		// json.Encoder.Encode already appends a trailing '\n' — do not append
		// another one (unlike the previous json.MarshalIndent call, which did
		// not add a trailing newline on its own).
		return buf.Bytes(), nil
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
	case "opencode-agent":
		// Reconstrói o frontmatter para o OpenCode CLI (opencode.ai), seguindo
		// o mesmo padrão de reconstrução-do-zero do case "agent-directory".
		// Decisão registrada na Wave 1 do roadmap
		// ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai (achado
		// #3, pesquisa contra o binário real 1.18.13):
		//   - "tools:" é uma chave RESERVADA no schema de agente do OpenCode
		//     (espera um objeto de overrides por-ferramenta, ex. {bash: false},
		//     não uma lista de nomes estilo Claude Code) — reutilizar o
		//     frontmatter original faz o OpenCode recusar o carregamento
		//     INTEIRO do projeto ("Configuration is invalid"), não só daquele
		//     agente. Por isso "tools:" nunca é emitido aqui.
		//   - sem "mode:" explícito, o OpenCode assume mode "all" (agente
		//     selecionável como persona primária de chat) — os agentes trackfw
		//     devem ser sempre subagentes puros, nunca primários, para
		//     paridade com o comportamento nos demais targets. Por isso
		//     "mode: subagent" é sempre fixo, nunca omitido.
		//   - "model:" é deliberadamente OMITIDO (decisão de produto do
		//     orquestrador, não uma limitação técnica): o OpenCode espera
		//     "provider/model-id" (ex. "anthropic/claude-sonnet-4-5"), não os
		//     aliases curtos do catálogo canônico ("opus"/"sonnet"), e mapear
		//     para um provider fixo contradiria a motivação de negócio do REQ
		//     (permitir que o usuário roteie os agentes trackfw para o modelo
		//     open-source/local que ele já configurou em opencode.json). Omitir
		//     deixa o OpenCode resolver pelo default já configurado pelo
		//     usuário.
		//   - "memory:" também não faz sentido no schema do OpenCode e é
		//     descartado junto com "tools:".
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString("description: " + description + "\n")
		sb.WriteString("mode: subagent\n")
		sb.WriteString("---\n")
		if body != "" {
			sb.WriteString(body + "\n")
		}
		return []byte(sb.String()), nil
	default:
		if targetID == "cursor" {
			if mapped, ok := mapModelCursor(model); ok {
				var rewErr error
				source, rewErr = rewriteFrontmatterModelLine(source, mapped)
				if rewErr != nil {
					return nil, rewErr
				}
			} else {
				source = removeFrontmatterModelLine(source)
			}
		} else if targetID == "claude" && len(agentModels) > 0 {
			// Allowlist: only the "claude" target receives composed model IDs.
			// Codex, Cursor, Antigravity, OpenCode, Gemini, Kiro and any other
			// target are left untouched even when agentModels is populated —
			// ADR-2026-08-21 §4 (gate, not cuidado).
			//
			// Empty version string means "no pin": leave the existing model line
			// as-is (fall back to tier alias). This preserves identical behavior
			// for callers that set agentModels but leave a tier value empty.
			if version, ok := agentModels[model]; ok && version != "" {
				var modelID string
				if isVersionString(version) {
					// Compose: "4.6" → "claude-sonnet-4-6"; "5" → "claude-sonnet-5".
					modelID = composeClaudeModelID(model, version)
				} else {
					// Escape hatch: non-version value (contains hyphens, letters, etc.)
					// is used literally — e.g. "claude-sonnet-4-5-20250929" stays as-is.
					// ADR-2026-08-21 §3.
					modelID = version
				}
				var rewErr error
				source, rewErr = rewriteFrontmatterModelLine(source, modelID)
				if rewErr != nil {
					return nil, rewErr
				}
			}
		}
		if !hasIdentity {
			return normalizeMarkdown(source), nil
		}
		withBody := insertBodyPrefix(source, greeting)
		withFrontmatter := rewriteFrontmatterFields(withBody, name, description)
		withSignature := rewriteSignatureLine(withFrontmatter, id.DisplayName)
		return normalizeMarkdown(withSignature), nil
	}
}

// greetingLine builds the first line injected into the agent body when an
// identity is configured for the agent. With no nickname configured, only
// the agent's display name is mentioned.
func greetingLine(id identity.AgentIdentity, nickname string) string {
	if nickname == "" {
		return fmt.Sprintf("You are %s.", id.DisplayName)
	}
	return fmt.Sprintf("You are %s. Address the user as %s.", id.DisplayName, nickname)
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

// rewriteSignatureLine rewrites the last line in the body section of a raw
// markdown source (frontmatter + body) that matches the agent signature
// pattern `^— <name>, <title>$` (em-dash U+2014, space, name, comma, space,
// title). Only the first capture group (the agent name) is replaced with
// displayName; the title (second group) is preserved byte-for-byte.
//
// Scope: the function operates only on the body (everything after the closing
// "---" of the frontmatter). A signature line that happens to appear inside
// the frontmatter block is never touched — the frontmatter boundary detection
// mirrors rewriteFrontmatterFields exactly.
//
// If no line in the body matches the pattern, source is returned unchanged.
// If displayName is empty, source is returned unchanged. The function never
// invents a signature that wasn't already present.
//
// Used by Rota B (the default branch of Render) when hasIdentity == true,
// applied to the output of rewriteFrontmatterFields, so the identity display
// name appears in the signature line in addition to the greeting and frontmatter.
func rewriteSignatureLine(source []byte, displayName string) []byte {
	if displayName == "" {
		return source
	}
	trimmed := strings.TrimSpace(string(source))
	// Locate the start of the body — mirror rewriteFrontmatterFields boundary
	// detection so scoping agrees between the two functions.
	bodyStart := 0
	if strings.HasPrefix(trimmed, "---\n") {
		end := strings.Index(trimmed[4:], "\n---")
		if end >= 0 {
			// bodyStart points to the character immediately after "\n---"
			bodyStart = 4 + end + 4
		}
	}
	head := trimmed[:bodyStart]
	bodySection := trimmed[bodyStart:]

	lines := strings.Split(bodySection, "\n")
	// Walk backward to find the last line matching the signature pattern.
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		// Pattern: em-dash (U+2014) + space + name + ", " + title
		if !strings.HasPrefix(line, "— ") {
			continue
		}
		rest := line[len("— "):] // skip "— "
		commaIdx := strings.Index(rest, ", ")
		if commaIdx < 0 {
			continue
		}
		title := rest[commaIdx+2:]
		if title == "" {
			continue
		}
		lines[i] = "— " + displayName + ", " + title
		return []byte(head + strings.Join(lines, "\n"))
	}
	// No signature line found — return source unchanged.
	return source
}

// rewriteFrontmatterFields replaces the "name:" and "description:" lines of
// a raw markdown source's frontmatter with name and description, preserving
// every other frontmatter line byte-for-byte (order, spacing, quote style)
// and leaving the body untouched. Used by Rota B (the default branch of
// Render) so representations that consume the raw frontmatter — chiefly
// "subagent" (claude, gemini, cursor, copilot, kiro-ide, windsurf) — pick up
// the customized identity: Claude Code's subagent selection reads only
// name/description from the frontmatter, never the body.
//
// The rewrite is scoped strictly to the frontmatter block (between the
// opening "---\n" and the closing "\n---"): a "name:" line that happens to
// appear in the body is never touched. If the frontmatter has no "name:" or
// "description:" line, that key is simply left absent — this function never
// invents a key that wasn't already there. If source has no recognizable
// frontmatter, source is returned unchanged (trimmed), matching the
// behavior Render already has for identity-less rendering.
func rewriteFrontmatterFields(source []byte, name, description string) []byte {
	trimmed := strings.TrimSpace(string(source))
	if !strings.HasPrefix(trimmed, "---\n") {
		return []byte(trimmed)
	}
	end := strings.Index(trimmed[4:], "\n---")
	if end < 0 {
		return []byte(trimmed)
	}
	frontmatter := trimmed[4 : 4+end]
	rest := trimmed[4+end:] // starts with "\n---", followed by the body

	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		var replacement string
		switch strings.TrimSpace(key) {
		case "name":
			replacement = name
		case "description":
			replacement = description
		default:
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		quoted := len(trimmedValue) >= 2 && strings.HasPrefix(trimmedValue, `"`) && strings.HasSuffix(trimmedValue, `"`)
		if quoted {
			lines[i] = key + ": \"" + replacement + "\""
		} else {
			lines[i] = key + ": " + replacement
		}
	}

	return []byte("---\n" + strings.Join(lines, "\n") + rest)
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

// mapModelCodex converte o modelo canônico para o valor aceito pelo Codex CLI.
// Retorna (valor mapeado, true) se mapeável; ("", false) se a linha model deve ser omitida.
func mapModelCodex(model string) (string, bool) {
	switch model {
	case "opus":
		return "gpt-5.4", true
	case "sonnet":
		return "gpt-5.4-mini", true
	default:
		return "", false
	}
}

// mapModelCursor converte o modelo canônico para o valor aceito pela Cursor
// (fonte: cursor.com/docs/subagents, ver ADR ADR-2026-08-14). Retorna (valor
// mapeado, true) se mapeável; ("", false) se a linha "model:" deve ser
// removida do frontmatter (Cursor cai no default "inherit"/Auto).
func mapModelCursor(model string) (string, bool) {
	switch model {
	case "opus":
		return "claude-opus-5[effort=high]", true
	case "sonnet":
		return "composer-2.5[fast=true]", true
	default:
		return "", false
	}
}

// containsControlChar reports whether s contains any ASCII control character
// (U+0000–U+001F) or a Unicode line/paragraph separator (U+2028, U+2029).
// Model IDs never require control characters or Unicode line separators in any
// known or anticipated format; a value that contains them is either malformed
// or a frontmatter-injection payload. Callers must reject, not sanitize.
//
// Implementation note: the rune loop is equivalent to the old byte loop for
// ASCII controls — UTF-8 continuation bytes are always ≥ 0x80, so a byte
// < 0x20 can never be part of a multi-byte sequence. The rune loop adds
// detection of U+2028/U+2029, which yaml.v3 preserves verbatim in the parsed
// Go string (bytes 0xE2 0x80 0xA8 / 0xE2 0x80 0xA9, all ≥ 0x80, invisible to
// the original byte < 0x20 check). ML-5C.
//
// U+0085 (NEL) is intentionally excluded: yaml.v3 normalizes it to a space
// at parse time, so no U+0085 ever reaches this function; the space it
// produces is inert for YAML frontmatter parsers (measured 2026-08-21).
func containsControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == ' ' || r == ' ' {
			return true
		}
	}
	return false
}

// rewriteFrontmatterModelLine replaces the "model:" line of a raw markdown
// source's frontmatter with value, preserving every other frontmatter line
// and the body byte-for-byte. If the frontmatter has no "model:" line, one is
// appended as the last line of the frontmatter block. If source has no
// recognizable frontmatter, source is returned unchanged (trimmed) — mirrors
// the boundary detection used by rewriteFrontmatterFields, scoped to the
// single "model" key.
//
// Returns an error if value contains any ASCII control character (U+0000–U+001F).
// Model IDs never require control characters; any such value is rejected to
// prevent frontmatter injection (CVE-class: escape hatch writes value literally;
// a newline closes the frontmatter block or adds attacker-controlled YAML keys).
func rewriteFrontmatterModelLine(source []byte, value string) ([]byte, error) {
	if containsControlChar(value) {
		return nil, fmt.Errorf("model value contains control character and was rejected: model IDs never require newlines or other control characters (got %q)", value)
	}
	trimmed := strings.TrimSpace(string(source))
	if !strings.HasPrefix(trimmed, "---\n") {
		return []byte(trimmed), nil
	}
	end := strings.Index(trimmed[4:], "\n---")
	if end < 0 {
		return []byte(trimmed), nil
	}
	frontmatter := trimmed[4 : 4+end]
	rest := trimmed[4+end:] // starts with "\n---", followed by the body

	lines := strings.Split(frontmatter, "\n")
	found := false
	for i, line := range lines {
		key, value2, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "model" {
			continue
		}
		trimmedValue := strings.TrimSpace(value2)
		quoted := len(trimmedValue) >= 2 && strings.HasPrefix(trimmedValue, `"`) && strings.HasSuffix(trimmedValue, `"`)
		if quoted {
			lines[i] = "model: \"" + value + "\""
		} else {
			lines[i] = "model: " + value
		}
		found = true
		break
	}
	if !found {
		lines = append(lines, "model: "+value)
	}

	return []byte("---\n" + strings.Join(lines, "\n") + rest), nil
}

// removeFrontmatterModelLine removes the "model:" line from a raw markdown
// source's frontmatter, if present, preserving every other frontmatter line
// and the body byte-for-byte. If source has no "model:" line or no
// recognizable frontmatter, source is returned unchanged (trimmed).
func removeFrontmatterModelLine(source []byte) []byte {
	trimmed := strings.TrimSpace(string(source))
	if !strings.HasPrefix(trimmed, "---\n") {
		return []byte(trimmed)
	}
	end := strings.Index(trimmed[4:], "\n---")
	if end < 0 {
		return []byte(trimmed)
	}
	frontmatter := trimmed[4 : 4+end]
	rest := trimmed[4+end:] // starts with "\n---", followed by the body

	lines := strings.Split(frontmatter, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		key, _, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "model" {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == len(lines) {
		return []byte(trimmed)
	}

	return []byte("---\n" + strings.Join(kept, "\n") + rest)
}

// --- D5 extension point: third-party skill reference composition ---
//
// ThirdPartyReference records one link from a catalog agent's rendered file
// to a third-party skill artifact installed at Destination (D5). Entries are
// persisted per-project (never per-home; third-party scope defaults to
// "project" — D4) under thirdPartyReferencesPath, keyed by
// "<targetID>/<agentItemID>".
//
// Why this exists: internal/commands/integrations_thirdparty.go's `install`
// (fase 2) cannot simply append a reference block to an already-rendered
// catalog agent file and write it once — the *next* plain `trackfw agents
// update` would call BuildPlans → Render again, which re-derives content
// straight from the catalog asset and knows nothing about the appended
// block. That regenerated content would differ from what's on disk, so
// Manager would either silently overwrite (wiping the reference) or skip
// with a warning, depending on ownership state — see manager.go's
// preflight/applyMutation. Persisting the reference here and having
// BuildPlans call ApplyThirdPartyReferences after every Render (see
// plan.go) makes the *canonical* render reproduce the block, so repeated
// `agents update` runs settle at StateCurrent instead of fighting the
// third-party attachment.
type ThirdPartyReference struct {
	Slug        string `json:"slug"`
	Destination string `json:"destination"`
	URL         string `json:"url"`
}

const thirdPartyReferencesSchemaVersion = 1

type thirdPartyReferenceRegistry struct {
	SchemaVersion int                              `json:"schema_version"`
	Entries       map[string][]ThirdPartyReference `json:"entries"`
}

func thirdPartyReferencesPath(root string) string {
	return filepath.Join(root, ".trackfw", "thirdparty-references.json")
}

func loadThirdPartyReferenceRegistry(root string) (thirdPartyReferenceRegistry, error) {
	filename := thirdPartyReferencesPath(root)
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return thirdPartyReferenceRegistry{SchemaVersion: thirdPartyReferencesSchemaVersion, Entries: map[string][]ThirdPartyReference{}}, nil
	}
	if err != nil {
		return thirdPartyReferenceRegistry{}, fmt.Errorf("read thirdparty reference registry: %w", err)
	}
	var reg thirdPartyReferenceRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return thirdPartyReferenceRegistry{}, fmt.Errorf("decode thirdparty reference registry: %w", err)
	}
	if reg.SchemaVersion != thirdPartyReferencesSchemaVersion {
		return thirdPartyReferenceRegistry{}, fmt.Errorf("unsupported thirdparty reference registry schema %d", reg.SchemaVersion)
	}
	if reg.Entries == nil {
		reg.Entries = map[string][]ThirdPartyReference{}
	}
	return reg, nil
}

func writeThirdPartyReferenceRegistry(root string, reg thirdPartyReferenceRegistry) error {
	reg.SchemaVersion = thirdPartyReferencesSchemaVersion
	if reg.Entries == nil {
		reg.Entries = map[string][]ThirdPartyReference{}
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode thirdparty reference registry: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(thirdPartyReferencesPath(root), data, 0o600); err != nil {
		return fmt.Errorf("write thirdparty reference registry: %w", err)
	}
	return nil
}

// UpsertThirdPartyReference records (or replaces, keyed by ref.Slug) a
// reference from the rendered catalog agent artifact (targetID,
// agentItemID) to a third-party skill, and persists it under root
// (project root — D4). Idempotent: calling it twice with the same Slug
// replaces the prior entry instead of duplicating it.
func UpsertThirdPartyReference(root, targetID, agentItemID string, ref ThirdPartyReference) error {
	reg, err := loadThirdPartyReferenceRegistry(root)
	if err != nil {
		return err
	}
	key := targetID + "/" + agentItemID
	refs := reg.Entries[key]
	replaced := false
	for i, existing := range refs {
		if existing.Slug == ref.Slug {
			refs[i] = ref
			replaced = true
			break
		}
	}
	if !replaced {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Slug < refs[j].Slug })
	reg.Entries[key] = refs
	return writeThirdPartyReferenceRegistry(root, reg)
}

// thirdPartyRefStart/thirdPartyRefEnd are the composition markers (D5),
// dedicated and distinct from generators.rulesStart/rulesEnd (which govern
// a different subsystem — auxiliary rules files, not catalog agent
// artifacts). Mirrors the idempotent marker-replace pattern of
// injectOrUpdateRules in internal/generators/agentfiles.go.
const thirdPartyRefStart = "<!-- trackfw:thirdparty-skills:start -->"
const thirdPartyRefEnd = "<!-- trackfw:thirdparty-skills:end -->"

// ApplyThirdPartyReferences injects or updates the third-party reference
// block in content for (targetID, agentItemID), based on entries persisted
// by UpsertThirdPartyReference. When root is empty or there are no entries
// for this key, content is returned byte-for-byte unchanged — this is the
// guarantee that every agent artifact which never received a third-party
// attachment renders exactly as it did before D5 was introduced, by
// construction (mirrors the doc comment on Render about the no-identity
// path being unchanged "not by coincidence").
//
// content is expected to already be normalizeMarkdown's output (a single
// trailing "\n", no surrounding extra whitespace) — true for every caller,
// since this is only invoked right after Render produces the "subagent"
// markdown representation (plan.go, BuildPlans).
func ApplyThirdPartyReferences(root string, content []byte, targetID, agentItemID string) ([]byte, error) {
	if root == "" {
		return content, nil
	}
	reg, err := loadThirdPartyReferenceRegistry(root)
	if err != nil {
		return nil, err
	}
	refs := reg.Entries[targetID+"/"+agentItemID]
	if len(refs) == 0 {
		return content, nil
	}

	var block strings.Builder
	block.WriteString(thirdPartyRefStart + "\n")
	for _, ref := range refs {
		fmt.Fprintf(&block, "- Third-party skill %q: %s (source: %s)\n", ref.Slug, ref.Destination, ref.URL)
	}
	block.WriteString(thirdPartyRefEnd)

	text := string(content)
	start := strings.Index(text, thirdPartyRefStart)
	if start == -1 {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n" + block.String() + "\n"
		return []byte(text), nil
	}
	// Search for the end marker starting at start, not from the beginning
	// of text (hefesto-tf finding, ML-4C): searching the whole text could
	// find an END marker that appears BEFORE start — e.g. leftover,
	// unrelated content containing the literal end-marker string ahead of
	// a genuine start marker — producing end < start. Slicing
	// text[end+len(thirdPartyRefEnd):] in that case does not panic (both
	// indices are valid), but silently corrupts the composed output by
	// overlapping the wrong regions. Anchoring the search at start makes
	// end < start impossible: end is either -1 or >= start by construction.
	relEnd := strings.Index(text[start:], thirdPartyRefEnd)
	if relEnd == -1 {
		// Malformed (start without end): append a fresh block rather than
		// guess at repair — mirrors injectOrUpdateRules' handling of the
		// same malformed case.
		text += "\n" + block.String() + "\n"
		return []byte(text), nil
	}
	end := start + relEnd
	newText := text[:start] + block.String() + text[end+len(thirdPartyRefEnd):]
	return []byte(strings.TrimRight(newText, "\n") + "\n"), nil
}

// NormalizeThirdPartyContent applies the same normalization Render uses for
// managed catalog skill content (TrimSpace + single trailing newline) to raw
// third-party bytes before they are written through Manager.Install, so
// third-party artifacts are stored with the same on-disk convention as
// catalog artifacts.
func NormalizeThirdPartyContent(content []byte) []byte {
	return normalizeMarkdown(content)
}

// ResolveThirdPartySkillDestination computes where a third-party artifact's
// content should live for (targetID, scope), per D5: the shared parent
// directory of the target's own canonical project/global Skills install
// path template (e.g. ".claude/skills/trackfw-{{id}}/SKILL.md" truncates to
// ".claude/skills"), followed by "/thirdparty/<slug>.md". The directory
// always comes from the catalog's own path template — never a hardcoded
// ".claude" — so every target the catalog declares a Skills capability for
// is supported without per-target special-casing (D5's explicit
// requirement).
func ResolveThirdPartySkillDestination(catalog *Catalog, targetID, scope, slug string) (destination, surfaceID string, err error) {
	target, ok := catalog.Target(targetID)
	if !ok {
		return "", "", fmt.Errorf("unknown target %q", targetID)
	}
	surfaces, err := selectedSurfaces(target, KindSkills, "", false)
	if err != nil {
		return "", "", err
	}
	surface := surfaces[0]
	installPath, ok := pathForScope(surface.Paths.Skills, scope)
	if !ok {
		return "", "", fmt.Errorf("target %s surface %s has no %s skills path", targetID, surface.ID, scope)
	}
	baseDir := truncateBeforeIDSegment(installPath.Path)
	return baseDir + "/thirdparty/" + slug + ".md", surface.ID, nil
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
