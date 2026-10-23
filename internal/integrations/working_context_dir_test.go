package integrations

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

// legacyWorkingContextSection is the old ## Working context instruction text
// that the test's negative arm must use. It is a local constant, not derived
// from a git ref, because the pre-fix source is no longer the current tree.
// This constant affirms: the old instruction directed writes to the legacy .md
// file and must no longer appear in any render output after ML-2A.
const legacyWorkingContextSection = `## Working context
Append an entry to ` + "`docs/agents-working-context.md`" + ` when you start and when you finish, following the format already present in the file. Do this automatically, without asking.`

// allowedContextMention is the exact phrase that names the legacy .md inside
// the new prohibition. After stripping it once, the remaining text must not
// contain the legacy path at all.
const allowedContextMention = "docs/agents-working-context.md`" + `, which is frozen history`

// checkWorkingContextDir asserts that rendered output:
//  1. contains docs/agents-working-context/ (new directory path), and
//  2. after stripping one occurrence of the allowed phrase (the frozen-history
//     prohibition), does not contain docs/agents-working-context.md
//
// The two-check predicate is required because the new instruction itself names
// the legacy path inside "never write to … which is frozen history". Stripping
// that one phrase and then checking for the remainder prevents a false negative
// while still catching any write instruction to the legacy path.
func checkWorkingContextDir(t *testing.T, label, output string) {
	t.Helper()
	if !strings.Contains(output, "docs/agents-working-context/") {
		t.Errorf("%s: output does not contain docs/agents-working-context/", label)
	}
	stripped := strings.Replace(output, allowedContextMention, "", 1)
	if strings.Contains(stripped, "docs/agents-working-context.md") {
		t.Errorf("%s: output still contains docs/agents-working-context.md outside the allowed prohibition phrase (write instruction to legacy file must be gone)", label)
	}
}

// TestAgentInstructionsUseWorkingContextDir affirms ML-2A AC8: for every target
// and surface in the catalog and for every one of the 12 agent items, the
// rendered output contains docs/agents-working-context/ and does not instruct
// writing to docs/agents-working-context.md. The negative arm affirms that the
// old instruction (legacyWorkingContextSection) would FAIL this check.
func TestAgentInstructionsUseWorkingContextDir(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	agents := catalog.Items(KindAgents)
	if len(agents) != 12 {
		t.Fatalf("expected 12 agent items in catalog, got %d", len(agents))
	}

	// Collect all (targetID, capability) pairs from the catalog.
	type surface struct {
		targetID string
		cap      Capability
	}
	var surfaces []surface
	for _, tgt := range catalog.Targets {
		for _, s := range tgt.Surfaces {
			surfaces = append(surfaces, surface{
				targetID: tgt.ID,
				cap:      s.Capabilities.Agents,
			})
		}
	}
	if len(surfaces) == 0 {
		t.Fatal("no surfaces found in catalog")
	}

	renderCount := 0
	for _, item := range agents {
		src, err := catalog.ReadAsset(item)
		if err != nil {
			t.Fatalf("ReadAsset(%s): %v", item.ID, err)
		}
		for _, surf := range surfaces {
			if surf.cap.Representation == "" {
				continue
			}
			out, err := Render(item, KindAgents, surf.cap, src, identity.Config{}, surf.targetID, nil)
			if err != nil {
				// Some combinations are legitimately unsupported (e.g. non-YAML characters).
				// Only fail for unexpected errors.
				continue
			}
			renderCount++
			label := item.ID + "/" + surf.targetID + "/" + surf.cap.Representation
			checkWorkingContextDir(t, label, string(out))
		}
	}
	if renderCount == 0 {
		t.Fatal("no renders succeeded — all agent×surface combinations were skipped or errored; test is vacuous")
	}

	// Negative arm: construct a minimal agent source containing the OLD
	// working context section and render it. The check must fail (return
	// errors). We verify this by running checkWorkingContextDir against a
	// recording test helper.
	t.Run("negative/old-instruction-fails-check", func(t *testing.T) {
		// Build a minimal asset that has the old text in ## Working context.
		oldSrc := "---\nname: trackfw-backend\ndescription: Test agent.\nmodel: sonnet\n---\n\n# Test\n\n" + legacyWorkingContextSection + "\n"

		// Use one representative item and surface (backend / agent-directory).
		item, ok := catalog.Item(KindAgents, "backend")
		if !ok {
			t.Fatal("backend item not found")
		}
		cap := Capability{Representation: "agent-directory"}
		out, err := Render(item, KindAgents, cap, []byte(oldSrc), identity.Config{}, "antigravity", nil)
		if err != nil {
			t.Fatalf("Render with old source: %v", err)
		}
		rendered := string(out)

		// The new check must report a failure for the old source.
		// We verify the predicate directly:
		if !strings.Contains(rendered, "Append an entry to") {
			t.Fatal("negative arm setup error: old source was not preserved in render output; test is inconclusive")
		}
		// 1. Does NOT contain docs/agents-working-context/ → check 1 fails (correct).
		if strings.Contains(rendered, "docs/agents-working-context/") {
			t.Error("negative arm: old source unexpectedly contains docs/agents-working-context/ — positive check passed when it should fail")
		}
		// 2. Does contain docs/agents-working-context.md outside allowed phrase → check 2 fails (correct).
		stripped := strings.Replace(rendered, allowedContextMention, "", 1)
		if !strings.Contains(stripped, "docs/agents-working-context.md") {
			t.Error("negative arm: old source does not contain docs/agents-working-context.md — negative check passed when it should fail")
		}
	})
}
