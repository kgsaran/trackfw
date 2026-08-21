package integrations

import "strings"

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
// This function deliberately mirrors the model-resolution paths of Render so
// that the output of "trackfw agents models" equals what Render would write.
// See models_test.go:TestResolveAgentModelMatchesRender for the drift gate.
func ResolveAgentModel(tier, representation, targetID string, agentModels map[string]string) (resolved string, present bool) {
	switch representation {
	case "custom-agent-toml":
		return mapModelCodex(tier)
	case "cli-agent-json", "agent-json":
		return "", false
	case "agent-directory":
		return mapModel(tier)
	case "opencode-agent":
		return "", false
	default:
		// default branch mirrors Render's default case exactly.
		if targetID == "cursor" {
			return mapModelCursor(tier)
		}
		if targetID == "claude" {
			if version, ok := agentModels[tier]; ok && version != "" {
				if isVersionString(version) {
					return composeClaudeModelID(tier, version), true
				}
				return version, true // escape hatch: use literally
			}
			// no pin → tier alias unchanged
		}
		return tier, true
	}
}

// LooksLikeSuspectModelValue reports whether v is an agent_models value that
// will trigger the escape-hatch path and likely produce an invalid model
// identifier in the rendered artifact.
//
// Returns true when v is not a bare version string (digits and dots only) AND
// does not look like a Claude model ID (prefix "claude-"), OR when v contains
// any ASCII control character (U+0000–U+001F). Values with control characters
// are always suspect: rewriteFrontmatterModelLine rejects them outright, so
// an agent_models entry containing \n or \r will fail at install time. Flagging
// them here aligns the "trackfw agents models" inspection command with the
// behavior of "trackfw agents install/update" — a command that reports a
// different outcome than the write path would is worse than no command.
//
// Callers should emit a per-tier warning to stderr when this returns true; the
// warning must fire once per tier, not once per row, to avoid being so noisy
// that it trains the user to ignore it.
//
// Trade-off: false-negatives are preferred over false-positives.
// "4.6-beta" → warns (has hyphen, not version, not claude-).
// "4.6", "5"  → no warn (bare version strings).
// "claude-sonnet-4-5-20250929" → no warn (looks like a Claude model ID).
// "gpt-5"     → warns (not a version, not claude-; wrong namespace for claude target).
// "claude-sonnet-4-6\ntools: Bash" → warns (control char; install would reject).
func LooksLikeSuspectModelValue(v string) bool {
	return containsControlChar(v) || (!isVersionString(v) && !strings.HasPrefix(v, "claude-"))
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
