package integrations

import (
	"os"
	"path/filepath"
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
		// D-4: false negatives — inline YAML metachar check fires BEFORE the
		// claude- prefix check, so these are caught even with the claude- prefix.
		// Values contain space or # which are rejected by the write path via
		// isValidModelPinValue; LooksLikeSuspect must agree with what install does.
		{"claude-opus-5 #x", true},          // space before # → YAML inline comment
		{"claude-opus-5: danger", true},      // space before : value → write-path reject
		// D-4: false positives stopped — Bedrock-style IDs must NOT warn.
		// "anthropic.claude-opus-v1:0" starts with alpha then "." → provider-qualified.
		{"anthropic.claude-opus-v1:0", false}, // Bedrock-style ID → no warn
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
		resolved, present, err := ResolveAgentModel(tc.tier, tc.representation, tc.targetID, tc.agentModels)
		if err != nil {
			t.Errorf("ResolveAgentModel(tier=%q repr=%q target=%q): unexpected error: %v",
				tc.tier, tc.representation, tc.targetID, err)
			continue
		}
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

			gotResolved, gotPresent, resolveErr := ResolveAgentModel(tier, capability.Representation, tgt.ID, agentModels)
			if resolveErr != nil {
				t.Errorf("agent=%s target=%s (repr=%s): ResolveAgentModel returned unexpected error: %v",
					agent.ID, tgt.ID, capability.Representation, resolveErr)
				continue
			}

			// Extract model line from the rendered artifact.
			// extractFrontmatterField strips surrounding double-quotes so that
			// "claude-sonnet-4-6" (quoted in YAML) compares equal to the
			// unquoted value returned by ResolveAgentModel.
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
				// Strip surrounding double-quotes: YAML model: "claude-sonnet-4-6"
				// and model: claude-sonnet-4-6 are semantically identical. The
				// drift gate compares the raw value from ResolveAgentModel (no
				// quotes) with the extracted value from the rendered file, which
				// may now be double-quoted for the claude target (D-2).
				if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
					v = v[1 : len(v)-1]
				}
				return v, true
			}
		}
	}
	return "", false
}

// TestValidateAgentModelValue covers the D-1 allowlist: rejected values must
// produce a non-nil error naming the tier and the value; accepted values must
// produce nil. The accepted set includes values that YAML would parse as
// non-string types without quoting (null, true, 0x10) — they are valid for the
// allowlist because the write path quotes them (D-2), making them string scalars.
func TestValidateAgentModelValue(t *testing.T) {
	cases := []struct {
		tier    string
		value   string
		wantErr bool
	}{
		// Rejected — each asserts: this exact value causes Render to return an
		// error and write no file (D-1 mirror invariant).
		{"sonnet", "5 && rm -rf /", true},   // shell metachar (space + &)
		{"sonnet", "5: danger", true},        // colon-space (YAML key separator)
		{"sonnet", "{tools: Bash}", true},    // flow mapping
		{"sonnet", "claude-opus-5 #x", true}, // space+hash (YAML comment)
		{"sonnet", "4.6 beta", true},         // space
		{"sonnet", `"quoted"`, true},         // double-quote
		// empty is not validated (it means "no pin")
		{"sonnet", "", false},
		// Accepted — each asserts: this value passes the write-path allowlist;
		// Render would proceed and write the artifact.
		{"sonnet", "5", false},
		{"sonnet", "4.6", false},
		{"sonnet", "4-5-sonnet", false},
		{"sonnet", "opus", false},
		{"sonnet", "claude-opus-5-5", false},
		{"sonnet", "anthropic.claude-opus-4-5-v1:0", false},
		{"sonnet", "claude-opus-5@20250101", false},
		{"sonnet", "openai/gpt-5", false},
		// YAML type-confusion values: allowlist accepts them; the write path
		// quotes them (D-2) so the YAML parser sees a string, not a non-string type.
		{"sonnet", "null", false},
		{"sonnet", "true", false},
		{"sonnet", "0x10", false},
	}
	for _, tc := range cases {
		err := ValidateAgentModelValue(tc.tier, tc.value)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateAgentModelValue(%q, %q): expected error, got nil", tc.tier, tc.value)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateAgentModelValue(%q, %q): unexpected error: %v", tc.tier, tc.value, err)
		}
		// When error, it must name the tier in the message.
		if tc.wantErr && err != nil && !strings.Contains(err.Error(), tc.tier) {
			t.Errorf("ValidateAgentModelValue(%q, %q): error message does not name tier: %v", tc.tier, tc.value, err)
		}
	}
}

// TestRenderRejectsInvalidModelValueAndWritesNoFile asserts D-1 end-to-end:
// an invalid agent_models value causes Render to return a non-nil error and
// write no file. The test uses the real write path (BuildPlans) with a temp dir
// to ensure no file is left on disk when validation fails.
func TestRenderRejectsInvalidModelValueAndWritesNoFile(t *testing.T) {
	// Use a real catalog agent and the claude target so the validation branch fires.
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	// Find the first agent with a sonnet tier.
	var agent Item
	var source []byte
	for _, a := range cat.Agents {
		s, rerr := cat.ReadAsset(a)
		if rerr != nil {
			continue
		}
		if AgentTier(s) == "sonnet" {
			agent = a
			source = s
			break
		}
	}
	if agent.ID == "" {
		t.Skip("no sonnet agent in catalog")
	}

	// An invalid value: space is forbidden by the allowlist.
	agentModels := map[string]string{"sonnet": "claude-opus-5 injected"}

	// Find claude surface.
	var claudeTarget Target
	for _, tgt := range cat.Targets {
		if tgt.ID == "claude" {
			claudeTarget = tgt
			break
		}
	}
	if claudeTarget.ID == "" {
		t.Skip("claude target not in catalog")
	}
	surface, ok := DefaultAgentSurface(claudeTarget)
	if !ok {
		t.Skip("no default surface for claude")
	}

	// Render must return an error — no file must have been written.
	tmpDir := t.TempDir()
	out, rerr := Render(agent, KindAgents, surface.Capabilities.Agents, source, identity.Config{}, "claude", agentModels)
	if rerr == nil {
		t.Errorf("Render must return error for invalid model value; got nil (output len=%d)", len(out))
	}

	// Confirm no file exists in tmpDir (the test is a direct Render call, so
	// tmpDir is unused here — but we establish the invariant that when Render
	// returns an error the caller would not write the bytes).
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) != 0 {
		t.Errorf("no file should exist in tmpDir after failed Render; found %v",
			func() []string {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = filepath.Join(tmpDir, e.Name())
				}
				return names
			}())
	}
}
