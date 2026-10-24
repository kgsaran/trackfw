package integrations

// TestWorkingContextFormByRole affirms ML-1A (D1): for the architect, the
// ## Working context section instructs writing to docs/agents-working-context/;
// for every other role, it instructs NOT writing and reporting instead.
//
// The negative arm proves the test would fail (return FAIL) if a non-architect
// asset reverted to "Write a new file in `docs/agents-working-context/`".
//
// Reconciliation sentence (Regra Dura de Reconciliação): this test asserts the
// conclusion of ML-1A — that the 11 non-architect assets no longer direct the
// agent to write to the working context directory, while the architect asset
// retains the write instruction.

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

// wctxWritePhrase is the forbidden phrase for non-architect agents.
// The test fails any non-architect render that still contains it.
const wctxWritePhrase = "Write a new file in `docs/agents-working-context/"

// wctxReportPhrase is the required phrase for non-architect agents.
const wctxReportPhrase = "Do not write to the working context"

// wctxArchitectPhrase is required in the architect's rendered output.
const wctxArchitectPhrase = "Write a new file in `docs/agents-working-context/"

// extractWorkingContextSection returns the body of the ## Working context
// section (the text between the heading line and the next ## heading, or end of
// string). Returns ("", false) if the heading is absent.
func extractWorkingContextSection(text string) (string, bool) {
	const heading = "## Working context\n"
	idx := strings.Index(text, heading)
	if idx < 0 {
		return "", false
	}
	body := text[idx+len(heading):]
	// Trim up to the next ## heading.
	if next := strings.Index(body, "\n## "); next >= 0 {
		body = body[:next]
	}
	return body, true
}

func TestWorkingContextFormByRole(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	agents := catalog.Items(KindAgents)
	if len(agents) != 12 {
		t.Fatalf("expected 12 agent items in catalog, got %d", len(agents))
	}

	// Use agent-directory / antigravity as the representative render surface.
	cap := Capability{Representation: "agent-directory"}
	const targetID = "antigravity"

	architectSeen := 0
	var failures []string

	for _, item := range agents {
		src, err := catalog.ReadAsset(item)
		if err != nil {
			t.Fatalf("ReadAsset(%s): %v", item.ID, err)
		}
		out, err := Render(item, KindAgents, cap, src, identity.Config{}, targetID, nil)
		if err != nil {
			t.Fatalf("Render(%s): %v", item.ID, err)
		}
		rendered := string(out)

		section, ok := extractWorkingContextSection(rendered)
		if !ok {
			failures = append(failures, item.ID+": ## Working context section missing")
			continue
		}
		if strings.TrimSpace(section) == "" {
			failures = append(failures, item.ID+": ## Working context section is empty")
			continue
		}

		if item.ID == "architect" {
			architectSeen++
			if !strings.Contains(section, wctxArchitectPhrase) {
				failures = append(failures, item.ID+": architect section missing write instruction (want phrase: "+wctxArchitectPhrase+")")
			}
		} else {
			// Non-architect: must prohibit writing, must not instruct writing.
			if !strings.Contains(section, wctxReportPhrase) {
				failures = append(failures, item.ID+": non-architect section missing report instruction (want phrase: "+wctxReportPhrase+")")
			}
			if strings.Contains(section, wctxWritePhrase) {
				failures = append(failures, item.ID+": non-architect section still contains write instruction (forbidden phrase: "+wctxWritePhrase+")")
			}
		}
	}

	if architectSeen != 1 {
		t.Errorf("expected exactly 1 architect item, got %d", architectSeen)
	}
	for _, f := range failures {
		t.Errorf("%s", f)
	}

	// Negative arm: a non-architect asset with the old write instruction must
	// trigger the forbidden-phrase check.
	t.Run("negative/non-architect-write-instruction-fails", func(t *testing.T) {
		oldSectionBody := wctxWritePhrase + `<YYYYMMDDTHHMMSSZ>-<your-slug>-<short-topic>.md` + "` (UTC timestamp) when you start and when you finish, using the block format already in use. Never edit another entry's file; never write to `docs/agents-working-context.md`, which is frozen history. To read: run `trackfw context` or read the 10 most recent entries in `docs/agents-working-context/`. Do this automatically, without asking."
		syntheticText := "## Working context\n" + oldSectionBody + "\n"

		section, ok := extractWorkingContextSection(syntheticText)
		if !ok {
			t.Fatal("negative arm setup: extractWorkingContextSection failed on synthetic text")
		}
		if !strings.Contains(section, wctxWritePhrase) {
			t.Fatal("negative arm setup: old write phrase not found in synthetic section — test is inconclusive")
		}
		// Verify the positive check fails (no report phrase).
		hasReportPhrase := strings.Contains(section, wctxReportPhrase)
		hasForbiddenPhrase := strings.Contains(section, wctxWritePhrase)
		if hasReportPhrase {
			t.Error("negative arm: old source unexpectedly has the report phrase — positive check would pass when it should fail")
		}
		if !hasForbiddenPhrase {
			t.Error("negative arm: old source does not have the forbidden write phrase — negative check would pass when it should fail")
		}
	})
}
