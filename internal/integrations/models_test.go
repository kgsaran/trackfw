package integrations

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

// TestLooksLikeSuspectModelValue locks the warning heuristic (ML-2A AC):
// warns on "4.6-beta", silent on "4.6", "5", and "claude-sonnet-4-5-20250929".
func TestLooksLikeSuspectModelValue(t *testing.T) {
	cases := []struct {
		value   string
		suspect bool
	}{
		{"4.6-beta", true},               // escape hatch + not claude- → warn
		{"4.6", false},                   // bare version string → no warn
		{"5", false},                     // bare version string (major-only) → no warn
		{"claude-sonnet-4-5-20250929", false}, // starts with claude- → no warn
		{"claude-opus-5", false},          // starts with claude- → no warn
		{"gpt-5", true},                   // not version, not claude- → warn
		{"latest", true},                  // not version, not claude- → warn
		{"", true},                        // empty → isVersionString=false, no prefix → warn
		// ML-5A: control chars are always suspect — rewriteFrontmatterModelLine
		// rejects them outright, so the command must agree with the write path.
		{"claude-sonnet-4-6\ntools: Bash", true},         // \n → frontmatter injection
		{"claude-sonnet-4-6\n---\nINJECTED", true},       // \n---\n → body injection (most severe)
		// ML-5C: Unicode line/paragraph separators — same argument as control
		// chars; model IDs never need line separators.
		{"claude-sonnet-4-6\u2028tools: Bash", true},    // U+2028 LINE SEPARATOR
		{"claude-sonnet-4-6\u2029tools: Bash", true},    // U+2029 PARAGRAPH SEPARATOR
		// ML-5C: accented value starts with claude-, NOT a version string, NOT a
		// line separator → LooksLikeSuspectModelValue = false (no warn).
		{"claude-sonnet-4-6-caf\u00e9", false},
	}
	for _, tc := range cases {
		got := LooksLikeSuspectModelValue(tc.value)
		if got != tc.suspect {
			t.Errorf("LooksLikeSuspectModelValue(%q) = %v, want %v", tc.value, got, tc.suspect)
		}
	}
}

// TestResolveAgentModel verifies the resolution table for canonical tiers
// across every representation type, with and without agent_models configured.
func TestResolveAgentModel(t *testing.T) {
	pinned := map[string]string{"sonnet": "4.6", "opus": "5"}
	empty := map[string]string{}

	cases := []struct {
		tier           string
		representation string
		targetID       string
		agentModels    map[string]string
		wantResolved   string
		wantPresent    bool
	}{
		// claude + no pin → tier alias
		{"sonnet", "subagent", "claude", empty, "sonnet", true},
		{"opus", "subagent", "claude", empty, "opus", true},
		// claude + pin → composed model ID
		{"sonnet", "subagent", "claude", pinned, "claude-sonnet-4-6", true},
		{"opus", "subagent", "claude", pinned, "claude-opus-5", true},
		// claude + escape hatch (non-version, non-claude-) → literal
		{"sonnet", "subagent", "claude", map[string]string{"sonnet": "4.6-beta"}, "4.6-beta", true},
		// claude + escape hatch (claude- prefix) → literal, no warn
		{"sonnet", "subagent", "claude", map[string]string{"sonnet": "claude-sonnet-4-5-20250929"}, "claude-sonnet-4-5-20250929", true},
		// codex → mapModelCodex (agentModels ignored)
		{"sonnet", "custom-agent-toml", "codex", pinned, "gpt-5.4-mini", true},
		{"opus", "custom-agent-toml", "codex", pinned, "gpt-5.4", true},
		// cursor → mapModelCursor (agentModels ignored)
		{"sonnet", "agent-markdown", "cursor", pinned, "composer-2.5[fast=true]", true},
		{"opus", "agent-markdown", "cursor", pinned, "claude-opus-5[effort=high]", true},
		// antigravity → mapModel (agentModels ignored)
		{"sonnet", "agent-directory", "antigravity", pinned, "flash", true},
		{"opus", "agent-directory", "antigravity", pinned, "pro", true},
		// amazonq → no model field
		{"sonnet", "cli-agent-json", "amazonq", pinned, "", false},
		{"opus", "cli-agent-json", "amazonq", pinned, "", false},
		// opencode → model deliberately omitted
		{"sonnet", "opencode-agent", "opencode", pinned, "", false},
		// agent-json (antigravity legacy, kiro-cli) → no model field
		{"sonnet", "agent-json", "kiro", pinned, "", false},
		// gemini → default branch, not cursor/claude → tier alias (agentModels ignored)
		{"sonnet", "agent-markdown", "gemini", pinned, "sonnet", true},
		{"opus", "agent-markdown", "gemini", pinned, "opus", true},
		// copilot (custom-agent) → default branch → tier alias
		{"sonnet", "custom-agent", "copilot", pinned, "sonnet", true},
		// windsurf (skill) → default branch → tier alias
		{"sonnet", "skill", "windsurf", pinned, "sonnet", true},
		// kiro-ide (agent-markdown) → default branch → tier alias
		{"sonnet", "agent-markdown", "kiro", pinned, "sonnet", true},
	}

	for _, tc := range cases {
		resolved, present := ResolveAgentModel(tc.tier, tc.representation, tc.targetID, tc.agentModels)
		if resolved != tc.wantResolved || present != tc.wantPresent {
			t.Errorf(
				"ResolveAgentModel(tier=%q repr=%q target=%q) = (%q, %v), want (%q, %v)",
				tc.tier, tc.representation, tc.targetID,
				resolved, present, tc.wantResolved, tc.wantPresent,
			)
		}
	}
}

// TestResolveAgentModelMatchesRender is the drift gate: it asserts that
// ResolveAgentModel returns the same model value that Render would actually
// write in the generated artifact. Any divergence here would mean the
// "agents models" command lies to the user — which defeats its purpose.
//
// The test loads the real catalog and renders each agent × target combination,
// extracts the model line from the rendered output, and compares it against
// ResolveAgentModel's answer. present=false must correspond to an absent model
// line in the rendered output (not a tier alias written in its place).
func TestResolveAgentModelMatchesRender(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	agentModels := map[string]string{"sonnet": "4.6", "opus": "5"}

	for _, agent := range cat.Agents {
		source, err := cat.ReadAsset(agent)
		if err != nil {
			t.Fatalf("read asset %s: %v", agent.ID, err)
		}
		tier := AgentTier(source)

		for _, tgt := range cat.Targets {
			surface, ok := DefaultAgentSurface(tgt)
			if !ok {
				continue
			}
			capability := surface.Capabilities.Agents

			rendered, err := Render(agent, KindAgents, capability, source, identity.Config{}, tgt.ID, agentModels)
			if err != nil {
				t.Fatalf("render %s/%s: %v", agent.ID, tgt.ID, err)
			}

			gotResolved, gotPresent := ResolveAgentModel(tier, capability.Representation, tgt.ID, agentModels)

			// Extract model line from the rendered artifact.
			wantResolved, wantPresent := extractModelFromRendered(t, string(rendered), capability.Representation)

			if gotPresent != wantPresent {
				t.Errorf(
					"agent=%s target=%s (repr=%s): ResolveAgentModel.present=%v but rendered model present=%v",
					agent.ID, tgt.ID, capability.Representation, gotPresent, wantPresent,
				)
			}
			if gotPresent && wantPresent && gotResolved != wantResolved {
				t.Errorf(
					"agent=%s target=%s (repr=%s): ResolveAgentModel=%q but rendered model=%q",
					agent.ID, tgt.ID, capability.Representation, gotResolved, wantResolved,
				)
			}
		}
	}
}

// extractModelFromRendered parses the rendered artifact content and returns
// (modelValue, true) when a model field is present, or ("", false) when absent.
// The extraction strategy depends on the artifact representation.
func extractModelFromRendered(t *testing.T, content, representation string) (string, bool) {
	t.Helper()
	switch representation {
	case "custom-agent-toml":
		// model = "gpt-5.4-mini" — TOML quoted string
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "model = ") {
				v := strings.TrimPrefix(line, "model = ")
				v = strings.Trim(v, `"`)
				return v, true
			}
		}
		return "", false
	case "cli-agent-json", "agent-json":
		// JSON format — no model key
		return "", false
	case "opencode-agent":
		return "", false
	case "agent-directory":
		// YAML frontmatter inside ---..--- block
		return extractFrontmatterField(content, "model")
	default:
		// Markdown frontmatter
		return extractFrontmatterField(content, "model")
	}
}

func extractFrontmatterField(content, key string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return "", false
	}
	frontmatter := content[4 : 4+end]
	for _, line := range strings.Split(frontmatter, "\n") {
		if sep := strings.Index(line, ":"); sep >= 0 {
			k := strings.TrimSpace(line[:sep])
			if k == key {
				v := strings.TrimSpace(line[sep+1:])
				return v, true
			}
		}
	}
	return "", false
}
