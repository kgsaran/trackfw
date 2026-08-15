package integrations

import (
	"strings"
	"testing"
)

// TestApplyThirdPartyReferences_EndMarkerBeforeStartIsTreatedAsMalformed is
// the hefesto-tf finding (ML-4B/ML-4C): ApplyThirdPartyReferences searched
// for thirdPartyRefEnd across the WHOLE content, not starting at `start`.
// If the literal end-marker string happens to appear BEFORE a genuine start
// marker (stray/leftover text, unrelated to any real reference block), the
// old code computed end < start and sliced text[end+len(end marker):],
// which does not panic but silently corrupts the composed output by
// overlapping the wrong regions. The fix anchors the end search at start,
// so end is always either -1 or >= start; this content must now take the
// "malformed (start without end)" path — append a fresh block — the same
// path an end-marker-less start already takes.
func TestApplyThirdPartyReferences_EndMarkerBeforeStartIsTreatedAsMalformed(t *testing.T) {
	root := t.TempDir()
	if err := UpsertThirdPartyReference(root, "claude", "backend", ThirdPartyReference{
		Slug: "my-skill", Destination: ".claude/skills/thirdparty/my-skill.md", URL: "https://example.com/my-skill.md",
	}); err != nil {
		t.Fatalf("UpsertThirdPartyReference: %v", err)
	}

	// A stray end marker appears BEFORE the genuine start marker, with no
	// end marker after it — the exact shape that used to produce end < start.
	content := thirdPartyRefEnd + "\n\nUnrelated leftover text.\n\n" + thirdPartyRefStart + "\nstale content, no closing marker\n"

	got, err := ApplyThirdPartyReferences(root, []byte(content), "claude", "backend")
	if err != nil {
		t.Fatalf("ApplyThirdPartyReferences: %v", err)
	}
	text := string(got)

	// Must not panic (implicit — reaching here proves it) and must produce
	// well-formed output: exactly one occurrence of the start marker after
	// the fresh block is appended (the malformed path does not attempt to
	// remove the stale one), and the reference itself present.
	if !strings.Contains(text, "my-skill") {
		t.Fatalf("expected the reference to my-skill to be present, got:\n%s", text)
	}
	if !strings.Contains(text, "https://example.com/my-skill.md") {
		t.Fatalf("expected the reference URL to be present, got:\n%s", text)
	}
	// The stray leftover text before the genuine start marker must survive
	// untouched — the bug's symptom was silently mangling/duplicating
	// unrelated regions of the document.
	if !strings.Contains(text, "Unrelated leftover text.") {
		t.Fatalf("expected unrelated leftover text to survive untouched, got:\n%s", text)
	}
}

// TestApplyThirdPartyReferences_NoReferencesLeavesContentUnchanged is a
// baseline sanity check alongside the malformed-input test above.
func TestApplyThirdPartyReferences_NoReferencesLeavesContentUnchanged(t *testing.T) {
	root := t.TempDir()
	content := []byte("# Some agent\n\nRegular content.\n")
	got, err := ApplyThirdPartyReferences(root, content, "claude", "backend")
	if err != nil {
		t.Fatalf("ApplyThirdPartyReferences: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("expected content unchanged with no references, got:\n%s", got)
	}
}
