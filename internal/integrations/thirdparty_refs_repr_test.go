package integrations

// thirdparty_refs_repr_test.go — ML-4D corretivo (T-4: third-party references and structured representations).
//
// TestThirdPartyRefs_SkipBlockKeepValid asserts CONCLUSION of ML-4D corretivo:
//   BuildPlans does NOT error when third-party references are registered for a
//   structured-representation target; the artifact is valid TOML or JSON and
//   does not contain the HTML-comment block.
//
// TestThirdPartyRefs_StaleRegistryDoesNotBreakOtherTargets asserts CONCLUSION of ML-4D corretivo:
//   A stale registry entry for codex does not cause BuildPlans to fail for any target,
//   and does not suppress the reference block for markdown targets that also have entries.
//
// TestThirdPartyRefs_NoRefs_StructuredArtifactValid asserts CONCLUSION of ML-4D:
//   Without registered references, BuildPlans produces a valid TOML or JSON artifact
//   for structured-representation targets — the fix does not break the no-refs path.
//
// TestThirdPartyRefs_MarkdownRepresentation_GetsBlock asserts CONCLUSION of ML-4D:
//   For markdown (subagent/claude) representations, the HTML-comment reference block
//   is still appended when refs are registered — the fix does not remove existing
//   behavior for markdown targets.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

// structuredRepTargetCases covers all three structured representations emitted
// by the catalog (non-legacy surfaces only).  surfaceID is non-empty when a
// specific surface must be requested (kiro's default surface is "ide" which
// uses "agent-markdown"; the "cli" surface uses "agent-json" — that is the
// one under test here).
var structuredRepTargetCases = []struct {
	targetID       string
	surfaceID      string // empty = use default surface selection
	representation string
}{
	{"codex", "", "custom-agent-toml"},
	{"kiro", "cli", "agent-json"},
	{"amazonq", "", "cli-agent-json"},
}

// TestThirdPartyRefs_StructuredRepresentations_SkipBlockKeepValid asserts:
// When third-party references are registered for a structured-representation
// target, BuildPlans does NOT fail: the artifact is valid TOML or JSON and
// does not contain the thirdparty-skills HTML-comment block.
//
// On pre-corretivo code (plan.go with return nil, fmt.Errorf) this test FAILS:
// BuildPlans returns a non-nil error and no plans.
func TestThirdPartyRefs_StructuredRepresentations_SkipBlockKeepValid(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("catalog: backend item not found")
	}

	for _, tc := range structuredRepTargetCases {
		tc := tc
		t.Run(tc.targetID, func(t *testing.T) {
			root := t.TempDir()
			if err := UpsertThirdPartyReference(root, tc.targetID, item.ID, ThirdPartyReference{
				Slug:        "test-skill",
				Destination: ".agents/skills/thirdparty/test-skill.md",
				URL:         "https://example.com/skill.md",
			}); err != nil {
				t.Fatalf("UpsertThirdPartyReference: %v", err)
			}

			req := PlanRequest{
				Kind:        KindAgents,
				Targets:     []string{tc.targetID},
				Items:       []string{item.ID},
				Scope:       "project",
				ProjectRoot: root,
			}
			if tc.surfaceID != "" {
				req.Surfaces = map[string]string{tc.targetID: tc.surfaceID}
			}
			plans, buildErr := BuildPlans(catalog, req)
			if buildErr != nil {
				t.Fatalf("BuildPlans for %q (%s) with refs: expected nil error (skip), got: %v",
					tc.targetID, tc.representation, buildErr)
			}
			if len(plans) == 0 {
				t.Fatalf("BuildPlans for %q (%s): no plans returned", tc.targetID, tc.representation)
			}
			content := string(plans[0].Content)

			// The artifact must not contain the HTML-comment block.
			if strings.Contains(content, thirdPartyRefStart) {
				t.Fatalf("structured artifact for %q (%s) must not contain reference block; content snippet:\n%s",
					tc.targetID, tc.representation, truncateSnippet(content, 300))
			}

			// The artifact must be valid in its format.
			switch tc.representation {
			case "custom-agent-toml":
				for i, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
					if line == "" {
						continue
					}
					_, val, found := strings.Cut(line, " = ")
					if !found {
						t.Errorf("TOML line %d: not a key = value pair: %q", i+1, line)
						continue
					}
					if _, decErr := decodeTomlBasicString(val); decErr != nil {
						t.Errorf("TOML line %d: decoder rejected %q: %v", i+1, val, decErr)
					}
				}
			case "agent-json", "cli-agent-json":
				var obj map[string]string
				if unmarshalErr := json.Unmarshal([]byte(content), &obj); unmarshalErr != nil {
					t.Fatalf("json.Unmarshal: %v\ncontent:\n%s", unmarshalErr, content)
				}
			}
		})
	}
}

// TestThirdPartyRefs_StaleRegistryDoesNotBreakOtherTargets asserts:
// A stale registry entry for a structured-representation target (codex) does
// not cause BuildPlans to fail for any target in a multi-target request, and
// does not suppress the reference block for markdown targets (claude) that
// also have registered entries.
//
// On pre-corretivo code this test FAILS because BuildPlans returns an error
// the moment it encounters the codex entry, aborting the whole plan.
func TestThirdPartyRefs_StaleRegistryDoesNotBreakOtherTargets(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("catalog: backend item not found")
	}

	t.Run("codex-only stale entry, claude unaffected", func(t *testing.T) {
		root := t.TempDir()
		// Register a stale entry for codex only.
		if err := UpsertThirdPartyReference(root, "codex", item.ID, ThirdPartyReference{
			Slug:        "stale-skill",
			Destination: ".agents/skills/thirdparty/stale-skill.md",
			URL:         "https://example.com/stale.md",
		}); err != nil {
			t.Fatalf("UpsertThirdPartyReference: %v", err)
		}

		// BuildPlans for claude (markdown) with no registered claude entry must succeed.
		claudePlans, err := BuildPlans(catalog, PlanRequest{
			Kind:        KindAgents,
			Targets:     []string{"claude"},
			Items:       []string{item.ID},
			Scope:       "project",
			ProjectRoot: root,
		})
		if err != nil {
			t.Fatalf("BuildPlans for claude with stale codex entry: unexpected error: %v", err)
		}
		if len(claudePlans) == 0 {
			t.Fatal("BuildPlans for claude: no plans returned")
		}
		// Claude artifact must NOT contain the block (no claude entry in registry).
		if strings.Contains(string(claudePlans[0].Content), thirdPartyRefStart) {
			t.Fatalf("claude artifact unexpectedly contains reference block")
		}

		// BuildPlans for codex (stale) must also succeed and produce valid TOML.
		codexPlans, err := BuildPlans(catalog, PlanRequest{
			Kind:        KindAgents,
			Targets:     []string{"codex"},
			Items:       []string{item.ID},
			Scope:       "project",
			ProjectRoot: root,
		})
		if err != nil {
			t.Fatalf("BuildPlans for codex with stale entry: unexpected error: %v", err)
		}
		if len(codexPlans) == 0 {
			t.Fatal("BuildPlans for codex: no plans returned")
		}
		codexContent := string(codexPlans[0].Content)
		if strings.Contains(codexContent, thirdPartyRefStart) {
			t.Fatalf("codex artifact must not contain reference block; snippet:\n%s", truncateSnippet(codexContent, 300))
		}
	})

	t.Run("codex stale + claude registered: claude gets block", func(t *testing.T) {
		root := t.TempDir()
		// Stale codex entry.
		if err := UpsertThirdPartyReference(root, "codex", item.ID, ThirdPartyReference{
			Slug:        "stale-skill",
			Destination: ".agents/skills/thirdparty/stale-skill.md",
			URL:         "https://example.com/stale.md",
		}); err != nil {
			t.Fatalf("UpsertThirdPartyReference (codex): %v", err)
		}
		// Real claude entry.
		claudeRef := ThirdPartyReference{
			Slug:        "real-skill",
			Destination: ".claude/skills/thirdparty/real-skill.md",
			URL:         "https://example.com/real.md",
		}
		if err := UpsertThirdPartyReference(root, "claude", item.ID, claudeRef); err != nil {
			t.Fatalf("UpsertThirdPartyReference (claude): %v", err)
		}

		// claude BuildPlans: no error, artifact has the block with the real-skill slug.
		claudePlans, err := BuildPlans(catalog, PlanRequest{
			Kind:        KindAgents,
			Targets:     []string{"claude"},
			Items:       []string{item.ID},
			Scope:       "project",
			ProjectRoot: root,
		})
		if err != nil {
			t.Fatalf("BuildPlans for claude: unexpected error: %v", err)
		}
		if len(claudePlans) == 0 {
			t.Fatal("BuildPlans for claude: no plans returned")
		}
		claudeContent := string(claudePlans[0].Content)
		if !strings.Contains(claudeContent, thirdPartyRefStart) {
			t.Fatalf("claude artifact missing reference block; snippet:\n%s", truncateSnippet(claudeContent, 300))
		}
		if !strings.Contains(claudeContent, claudeRef.Slug) {
			t.Fatalf("claude artifact missing skill slug %q; snippet:\n%s", claudeRef.Slug, truncateSnippet(claudeContent, 300))
		}

		// codex BuildPlans: no error, valid TOML, no block.
		codexPlans, err := BuildPlans(catalog, PlanRequest{
			Kind:        KindAgents,
			Targets:     []string{"codex"},
			Items:       []string{item.ID},
			Scope:       "project",
			ProjectRoot: root,
		})
		if err != nil {
			t.Fatalf("BuildPlans for codex: unexpected error: %v", err)
		}
		if len(codexPlans) == 0 {
			t.Fatal("BuildPlans for codex: no plans returned")
		}
		codexContent := string(codexPlans[0].Content)
		if strings.Contains(codexContent, thirdPartyRefStart) {
			t.Fatalf("codex artifact must not contain reference block; snippet:\n%s", truncateSnippet(codexContent, 300))
		}
		for i, line := range strings.Split(strings.TrimRight(codexContent, "\n"), "\n") {
			if line == "" {
				continue
			}
			_, val, found := strings.Cut(line, " = ")
			if !found {
				t.Errorf("TOML line %d: not a key = value pair: %q", i+1, line)
				continue
			}
			if _, decErr := decodeTomlBasicString(val); decErr != nil {
				t.Errorf("TOML line %d: decoder rejected %q: %v", i+1, val, decErr)
			}
		}
	})
}

// TestThirdPartyRefs_NoRefs_StructuredArtifactValid asserts: without any
// registered references, BuildPlans produces a valid TOML or JSON artifact
// for structured-representation targets.  Uses decodeTomlBasicString (from
// render_escape_test.go, same package — not duplicated) for TOML field
// validation and json.Unmarshal for JSON.
func TestThirdPartyRefs_NoRefs_StructuredArtifactValid(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("catalog: backend item not found")
	}

	for _, tc := range structuredRepTargetCases {
		tc := tc
		t.Run(tc.targetID, func(t *testing.T) {
			root := t.TempDir() // empty registry — no refs
			req := PlanRequest{
				Kind:        KindAgents,
				Targets:     []string{tc.targetID},
				Items:       []string{item.ID},
				Scope:       "project",
				ProjectRoot: root,
			}
			if tc.surfaceID != "" {
				req.Surfaces = map[string]string{tc.targetID: tc.surfaceID}
			}
			plans, err := BuildPlans(catalog, req)
			if err != nil {
				t.Fatalf("BuildPlans (no refs): unexpected error: %v", err)
			}
			if len(plans) == 0 {
				t.Fatal("BuildPlans: no plans returned")
			}
			content := string(plans[0].Content)
			switch tc.representation {
			case "custom-agent-toml":
				// Parse each non-empty line as  key = "<encoded-value>"  using
				// the inline TOML 1.0 basic-string decoder (defined in
				// render_escape_test.go — same package, not duplicated).
				for i, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
					if line == "" {
						continue
					}
					_, val, found := strings.Cut(line, " = ")
					if !found {
						t.Errorf("TOML line %d: not a key = value pair: %q", i+1, line)
						continue
					}
					if _, decErr := decodeTomlBasicString(val); decErr != nil {
						t.Errorf("TOML line %d: decoder rejected %q: %v", i+1, val, decErr)
					}
				}
			case "agent-json", "cli-agent-json":
				var obj map[string]string
				if unmarshalErr := json.Unmarshal([]byte(content), &obj); unmarshalErr != nil {
					t.Fatalf("json.Unmarshal: %v\ncontent:\n%s", unmarshalErr, content)
				}
			}
		})
	}
}

// TestThirdPartyRefs_MarkdownRepresentation_GetsBlock asserts: the markdown
// (subagent/claude) representation still receives the HTML-comment reference
// block when refs are registered — the fix must not regress existing behavior
// for markdown targets.
func TestThirdPartyRefs_MarkdownRepresentation_GetsBlock(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("catalog: backend item not found")
	}

	root := t.TempDir()
	ref := ThirdPartyReference{
		Slug:        "test-skill",
		Destination: ".claude/agents/skills/thirdparty/test-skill.md",
		URL:         "https://example.com/skill.md",
	}
	if err := UpsertThirdPartyReference(root, "claude", item.ID, ref); err != nil {
		t.Fatalf("UpsertThirdPartyReference: %v", err)
	}

	plans, err := BuildPlans(catalog, PlanRequest{
		Kind:        KindAgents,
		Targets:     []string{"claude"},
		Items:       []string{item.ID},
		Scope:       "project",
		ProjectRoot: root,
	})
	if err != nil {
		t.Fatalf("BuildPlans (claude/subagent with refs): unexpected error: %v", err)
	}
	if len(plans) == 0 {
		t.Fatal("BuildPlans: no plans returned")
	}
	content := string(plans[0].Content)
	if !strings.Contains(content, thirdPartyRefStart) {
		t.Fatalf("markdown artifact missing HTML-comment start marker; content snippet:\n%s",
			truncateSnippet(content, 300))
	}
	if !strings.Contains(content, ref.Slug) {
		t.Fatalf("markdown artifact missing skill slug %q; content snippet:\n%s",
			ref.Slug, truncateSnippet(content, 300))
	}
	if !strings.Contains(content, ref.URL) {
		t.Fatalf("markdown artifact missing skill URL %q; content snippet:\n%s",
			ref.URL, truncateSnippet(content, 300))
	}
}

// truncateSnippet returns up to n bytes of s, appending "…" if truncated.
// Used only in test failure messages — no allocation pressure.
func truncateSnippet(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---------------------------------------------------------------------------
// TestStructuredRepCatalogCoverage — ML-4F (R3 coverage)
// ---------------------------------------------------------------------------

// TestStructuredRepCatalogCoverage asserts CONCLUSION of ML-4F (R3):
// IsStructuredAgentRepresentation agrees with the actual Render output for every
// representation in the catalog, and structuredRepTargetCases covers every
// representation that Render classifies as structured.
//
// Classification rule: Render output that does NOT start with "---\n" is
// structured (TOML or JSON); output that starts with "---\n" is markdown.
// This matches actual render.go behaviour for all known representations:
//   - "custom-agent-toml" → "name = ..." (TOML, no leading "---\n")
//   - "agent-json"        → "{" (JSON, no leading "---\n")
//   - "cli-agent-json"    → "{" (JSON, no leading "---\n")
//   - everything else     → "---\n..." (YAML frontmatter / markdown)
//
// Unlike the previous version of this test, classification is driven by the
// output of Render — not by IsStructuredAgentRepresentation itself — so a new
// structured representation that the function does not yet know about causes
// the test to fail instead of being silently skipped.
func TestStructuredRepCatalogCoverage(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	architectItem, ok := catalog.Item(KindAgents, "architect")
	if !ok {
		t.Fatal("catalog: architect item not found")
	}
	architectSource, readErr := catalog.ReadAsset(architectItem)
	if readErr != nil {
		t.Fatalf("ReadAsset(architect): %v", readErr)
	}
	architectSource = NormalizeCRLF(architectSource)

	markdownPrefix := []byte("---\n")

	// renderStructured collects every representation whose Render output does
	// not start with "---\n" (i.e. is TOML or JSON, not markdown/YAML).
	renderStructured := make(map[string]bool)

	for _, target := range catalog.Targets {
		for _, surface := range target.Surfaces {
			cap := surface.Capabilities.Agents
			rep := cap.Representation
			if rep == "" || cap.SupportLevel == "unsupported" {
				continue
			}

			out, renderErr := Render(architectItem, KindAgents, cap, architectSource, identity.Config{}, target.ID, nil)
			if renderErr != nil {
				t.Errorf("Render(architect, target=%s surface=%s rep=%q): unexpected error: %v",
					target.ID, surface.ID, rep, renderErr)
				continue
			}

			isMarkdown := bytes.HasPrefix(out, markdownPrefix)
			isStructuredFn := IsStructuredAgentRepresentation(rep)

			// The function must agree with the actual output: structured output
			// means IsStructuredAgentRepresentation must return true, and vice versa.
			if isStructuredFn != !isMarkdown {
				t.Errorf("target=%s surface=%s rep=%q: Render output isMarkdown=%v but IsStructuredAgentRepresentation=%v — they disagree; update IsStructuredAgentRepresentation or the representation",
					target.ID, surface.ID, rep, isMarkdown, isStructuredFn)
			}

			if !isMarkdown {
				renderStructured[rep] = true
			}
		}
	}

	// structuredRepTargetCases must cover every representation that Render
	// classified as structured.
	coveredReps := make(map[string]bool)
	for _, tc := range structuredRepTargetCases {
		coveredReps[tc.representation] = true
	}

	for rep := range renderStructured {
		if !coveredReps[rep] {
			t.Errorf("catalog has structured representation %q (confirmed by Render output) but structuredRepTargetCases has no test for it — add a case",
				rep)
		}
	}

	// structuredRepTargetCases must not declare representations whose Render
	// output is not actually structured (stale entries or wrong representation
	// string in the test table).
	for _, tc := range structuredRepTargetCases {
		if !renderStructured[tc.representation] {
			t.Errorf("structuredRepTargetCases entry %q: not found in catalog as a non-markdown representation — update the test table or IsStructuredAgentRepresentation",
				tc.representation)
		}
	}
}
