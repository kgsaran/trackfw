package integrations

import (
	"fmt"
	"strings"
)

// ValidateAgentModelValue returns an error when the value in agent_models for
// tier is not a safe YAML scalar for the claude write path. An invalid value
// causes Render to return an error and write no file; ValidateAgentModelValue
// exposes the same check so that "trackfw agents models" can report the same
// outcome without invoking the write path (D-1 mirror invariant).
//
// Empty string is not validated — it means "no pin" (the write path no-ops).
func ValidateAgentModelValue(tier, v string) error {
	if v == "" {
		return nil // empty = no-pin, not an error
	}
	if !isValidModelPinValue(v) {
		return fmt.Errorf("agent_models.%s: valor %q inválido (esperado apenas caracteres [a-zA-Z0-9._:@/+-], sem espaço, #, {, }, [, ], ', \")", tier, v)
	}
	return nil
}

// ResolveAgentModel returns the model string that Render would write to the
// model field of an artifact with the given representation and targetID, for
// an agent with the given tier.
//
// present=false means Render omits the model field entirely for this
// (representation, targetID) pair — the caller should display "—" or
// equivalent rather than the tier alias.
//
// agentModels applies only for the "claude" target (ADR-2026-08-21 §4). For
// all other targets the returned value is determined solely by the mapping
// tables already in Render (mapModelCodex, mapModelCursor, mapModel), so
// agent_models configuration has zero effect on those targets — namespace gate,
// not cuidado.
//
// Returns a non-nil error when the agent_models value for the claude target
// fails the write-path allowlist check (D-1 mirror: "trackfw agents models"
// and "trackfw agents install" report identical outcomes for the same value).
//
// This function deliberately mirrors the model-resolution paths of Render so
// that the output of "trackfw agents models" equals what Render would write.
// See models_test.go:TestResolveAgentModelMatchesRender for the drift gate.
func ResolveAgentModel(tier, representation, targetID string, agentModels map[string]string) (resolved string, present bool, err error) {
	switch representation {
	case "custom-agent-toml":
		r, p := mapModelCodex(tier)
		return r, p, nil
	case "cli-agent-json", "agent-json":
		return "", false, nil
	case "agent-directory":
		r, p := mapModel(tier)
		return r, p, nil
	case "opencode-agent":
		return "", false, nil
	default:
		// default branch mirrors Render's default case exactly.
		if targetID == "cursor" {
			r, p := mapModelCursor(tier)
			return r, p, nil
		}
		if targetID == "claude" {
			if version, ok := agentModels[tier]; ok && version != "" {
				if valErr := ValidateAgentModelValue(tier, version); valErr != nil {
					return "", false, valErr
				}
				if isVersionString(version) {
					return composeClaudeModelID(tier, version), true, nil
				}
				return version, true, nil // escape hatch: use literally
			}
			// no pin → tier alias unchanged
		}
		return tier, true, nil
	}
}

// LooksLikeSuspectModelValue reports whether v is an agent_models value that
// will likely produce an invalid model identifier or is rejected by the write
// path. Returns true for:
//   - values with ASCII control characters or Unicode line/paragraph separators
//     (rewriteFrontmatterModelLine rejects these outright);
//   - values that contain inline YAML metacharacters (space, tab, #, {, }, [,
//     ], ', ") — the write path rejects these via isValidModelPinValue, and the
//     warning aligns "trackfw agents models" with what install would do;
//   - values that are not bare version strings, not a Claude model ID prefix,
//     and not a provider-qualified ID (Bedrock "anthropic.X", Vertex "@date",
//     OpenAI "openai/X") — the escape-hatch path would write the value
//     literally, which may produce an invalid model identifier at runtime.
//
// Callers should emit a per-tier warning to stderr when this returns true; the
// warning must fire once per tier, not once per row, to avoid being so noisy
// that it trains the user to ignore it.
//
// D-4 changes from previous version:
//   - Now catches false negatives: "claude-x: danger" and "claude-x #x" (start
//     with "claude-" but contain YAML metacharacters that break install).
//   - Stops false positives: Bedrock IDs like "anthropic.claude-opus-4-5-v1:0"
//     no longer warn (they start with an alpha provider prefix before ".").
//
// Trade-off: false-negatives are still preferred over false-positives.
// "4.6-beta" → warns (not version, not claude-, no provider separator).
// "4.6", "5" → no warn (bare version strings).
// "claude-sonnet-4-5-20250929" → no warn (claude- prefix, no metachar).
// "claude-x: danger" → warns (space-containing; write path rejects it).
// "anthropic.claude-opus-4-5-v1:0" → no warn (alpha-provider prefix + ".").
// "gpt-5" → warns (not version, not claude-, no provider prefix with ".").
func LooksLikeSuspectModelValue(v string) bool {
	// Control chars (and Unicode line/paragraph separators) are always suspect.
	if containsControlChar(v) {
		return true
	}
	// Values with inline YAML metacharacters are always suspect — the write path
	// rejects them via isValidModelPinValue. This catches "claude-x: danger" and
	// "claude-x #x" that the old claude- prefix check missed.
	if containsYAMLInlineMetachar(v) {
		return true
	}
	// Bare version strings are safe (compose path handles them).
	if isVersionString(v) {
		return false
	}
	// Claude model family is safe (standard prefix for Claude Code targets).
	if strings.HasPrefix(v, "claude-") {
		return false
	}
	// Provider-qualified IDs (Bedrock: "anthropic.X", OpenAI: "openai/X",
	// Vertex date-suffix: "claude-opus-5@20250101") use ".", "/" or "@" as
	// separators. Heuristic for "." case: provider prefix starts with a letter
	// (distinguishes "anthropic.X" from "4.6-beta" which starts with a digit).
	if strings.ContainsAny(v, "/@") {
		return false
	}
	if idx := strings.Index(v, "."); idx > 0 && isAlpha(rune(v[0])) {
		_ = idx
		return false
	}
	// Unrecognized format — warn.
	return true
}

// containsYAMLInlineMetachar reports whether v contains characters that would
// make it an invalid or ambiguous YAML plain scalar: space, tab, #, {, }, [,
// ], single-quote, double-quote. These are exactly the characters excluded from
// the write-path allowlist that are not control characters.
func containsYAMLInlineMetachar(v string) bool {
	return strings.ContainsAny(v, " \t#{}[]'\"")
}

// isAlpha reports whether r is an ASCII letter.
func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// AgentTier extracts the tier (canonical model alias, e.g. "sonnet", "opus")
// from a catalog agent's raw markdown source. This is the "model:" field in
// the frontmatter. Returns "sonnet" as the fallback when no model field is
// found — consistent with the catalog default for non-architect agents.
func AgentTier(source []byte) string {
	_, _, tier, _ := markdownParts(source)
	if tier == "" {
		return "sonnet"
	}
	return tier
}

// DefaultAgentSurface returns the default surface for a target under KindAgents:
// the first surface that is neither "legacy" nor "unsupported". ok=false means
// the target has no eligible agents surface (should not happen for catalog
// targets). Mirrors the default-surface selection in BuildPlans / selectedSurfaces.
func DefaultAgentSurface(target Target) (Surface, bool) {
	surfaces, err := selectedSurfaces(target, KindAgents, "", false)
	if err != nil || len(surfaces) == 0 {
		return Surface{}, false
	}
	return surfaces[0], true
}
