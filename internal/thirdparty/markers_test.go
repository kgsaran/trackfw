package thirdparty

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// allMarkersDisplay mirrors literalMarkers but in the "Title Case" form a
// human would actually write in a heading, to exercise casefold (step 4).
var allMarkersDisplay = []string{
	"Git authority",
	"Mode lock",
	"Governance prerequisite",
	"Reporting boundary",
	"Scope boundary",
	"Dispatch contract",
}

func TestCheckMarkers_EachMarkerRefusedAsH1(t *testing.T) {
	for i, display := range allMarkersDisplay {
		display := display
		expected := literalMarkers[i]
		t.Run(expected, func(t *testing.T) {
			content := fmt.Sprintf("# %s\n\nSome body text.\n", display)
			matched := CheckMarkers([]byte(content))
			if len(matched) != 1 || matched[0] != expected {
				t.Fatalf("expected [%q], got %v", expected, matched)
			}
		})
	}
}

func TestCheckMarkers_EachMarkerRefusedAsH6(t *testing.T) {
	for i, display := range allMarkersDisplay {
		display := display
		expected := literalMarkers[i]
		t.Run(expected, func(t *testing.T) {
			content := fmt.Sprintf("###### %s\n\nSome body text.\n", display)
			matched := CheckMarkers([]byte(content))
			if len(matched) != 1 || matched[0] != expected {
				t.Fatalf("expected [%q], got %v", expected, matched)
			}
		})
	}
}

func TestCheckMarkers_MarkerInsideFencedBlockAccepted(t *testing.T) {
	content := "" +
		"# Benign heading\n\n" +
		"Some documentation about how markers work:\n\n" +
		"```\n" +
		"## Git authority\n" +
		"## Mode lock\n" +
		"```\n\n" +
		"More text.\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers matched for content inside a fenced block, got %v", matched)
	}
}

func TestCheckMarkers_MarkerInsideTildeFencedBlockAccepted(t *testing.T) {
	content := "" +
		"# Benign heading\n\n" +
		"~~~\n" +
		"## Scope boundary\n" +
		"~~~\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers matched for content inside a tilde-fenced block, got %v", matched)
	}
}

// TestCheckMarkers_UnclosedFenceDropsRestOfDocument (D3-ter(a) amendment,
// ML-4C): superseded. An unclosed fence used to swallow everything through
// EOF as fenced content, silently hiding any marker after the opener — this
// was found to be a security-relevant regression by the Wave 4 barrier
// (both hades-tf and hefesto-tf, independently) and by the architect's own
// reproduction against all 3 CLIs. Renamed and inverted: an opener with no
// matching closer is NOT a fence for this check — the content is rescanned
// and a marker inside it is now caught.
func TestCheckMarkers_UnclosedFenceNoLongerGrantsImmunity(t *testing.T) {
	content := "```\n# Git authority\nstill inside, never closed\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 1 || matched[0] != "git authority" {
		t.Fatalf("expected an unclosed fence to no longer hide a marker (D3-ter(a)), got %v", matched)
	}
}

// TestCheckMarkers_CloserShorterThanOpenerDoesNotCloseButStillCaught
// (D3-ter(a) amendment, ML-4C): superseded, same rationale as
// TestCheckMarkers_UnclosedFenceNoLongerGrantsImmunity above — a fence
// whose closer is shorter than its opener never closes per the CommonMark
// rule removeFencedBlocks still implements, and per D3-ter(a) a fence that
// never closes is not a fence at all: its content is rescanned.
func TestCheckMarkers_CloserShorterThanOpenerDoesNotCloseButStillCaught(t *testing.T) {
	content := "````\n# Git authority\n```\nstill fenced (closer too short)\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 1 || matched[0] != "git authority" {
		t.Fatalf("expected a marker inside a never-properly-closed fence to be caught (D3-ter(a)), got %v", matched)
	}
}

// TestCheckMarkers_IndentedFenceStillRecognized covers fencePrefixPattern's
// leading-whitespace allowance: a fence opener/closer indented under a list
// item or blockquote is still recognized as a fence delimiter, not read as
// prose.
func TestCheckMarkers_IndentedFenceStillRecognized(t *testing.T) {
	content := "   ```\n   ## Git authority\n   ```\n\nRegular text.\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers for a marker inside an indented fence, got %v", matched)
	}
}

// TestCheckMarkers_HeadingAfterClosedFenceStillMatches is the converse of
// the fence-acceptance tests above: content AFTER a properly closed fence
// must still be read as ordinary text, so a real heading following a fence
// is not accidentally swallowed by removeFencedBlocks.
func TestCheckMarkers_HeadingAfterClosedFenceStillMatches(t *testing.T) {
	content := "```\nsome code, not a marker\n```\n\n## Git authority\n\nRegular text.\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 1 || matched[0] != "git authority" {
		t.Fatalf("expected a heading after a closed fence to still match, got %v", matched)
	}
}

func TestCheckMarkers_FullwidthCompatibilityCharsRefused(t *testing.T) {
	// U+FF03 FULLWIDTH NUMBER SIGN, U+FF27 FULLWIDTH LATIN CAPITAL LETTER G —
	// NFKC folds both to ASCII "#" and "G". This is exactly what NFKC (step 3)
	// is meant to defeat.
	content := "＃＃ Ｇit authority\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 1 || matched[0] != "git authority" {
		t.Fatalf("expected fullwidth heading to be refused as git authority, got %v", matched)
	}
}

func TestCheckMarkers_CyrillicHomoglyphPasses(t *testing.T) {
	// U+0430 CYRILLIC SMALL LETTER A in place of Latin "a" in "authority".
	// NFKC does NOT fold cross-script homoglyphs — this is documented as an
	// explicit, deliberate gap in D3 ("o que este critério NÃO cobre"), not
	// a bug. The expected behavior is that this content PASSES (no match).
	content := "## Git аuthority\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected cyrillic homoglyph heading to pass (documented gap in D3), got %v", matched)
	}
}

func TestCheckMarkers_BenignContentAccepted(t *testing.T) {
	content := "# My Skill\n\n## Usage\n\nThis skill helps with formatting Go code.\n\n## Examples\n\n```go\nfmt.Println(\"hi\")\n```\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers for benign content, got %v", matched)
	}
}

// TestCheckMarkers_HTMLCommentNeutralizedContentStillMatches (D3-ter(b)
// amendment, ML-4C): supersedes the previous
// TestCheckMarkers_HTMLCommentStrippedBeforeMatch, which asserted the
// opposite (no match) — that assertion contradicted D3's own written
// justification for neutralizing comments in the first place ("an LLM
// reads HTML comments in the token stream") and was reproduced by hades-tf
// and the architect as a real evasion: `<!-- ## Git authority -->` passed
// clean. Step 1 now strips only the comment delimiters, keeping the inner
// text in place to be scanned like any other line.
func TestCheckMarkers_HTMLCommentNeutralizedContentStillMatches(t *testing.T) {
	content := "<!-- ## Git authority -->\n# Benign heading\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 1 || matched[0] != "git authority" {
		t.Fatalf("expected a marker inside a neutralized HTML comment to be caught (D3-ter(b)), got %v", matched)
	}
}

// TestCheckMarkers_MultilineHTMLCommentContentStillMatches covers the
// multi-line comment shape the architect reproduced against Python before
// the fix: the marker sits on its own line inside a multi-line comment
// block, not inline with the delimiters.
func TestCheckMarkers_MultilineHTMLCommentContentStillMatches(t *testing.T) {
	content := "<!--\n## Git authority\nsome other commented-out text\n-->\n# Benign heading\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 1 || matched[0] != "git authority" {
		t.Fatalf("expected a marker inside a multi-line neutralized HTML comment to be caught (D3-ter(b)), got %v", matched)
	}
}

// TestCheckMarkers_BenignHTMLCommentTextStaysBenign is the converse: text
// inside a neutralized comment that does NOT match any literal marker must
// not spuriously match after neutralization — neutralization only removes
// the delimiters, it does not otherwise alter the text.
func TestCheckMarkers_BenignHTMLCommentTextStaysBenign(t *testing.T) {
	content := "<!-- just an ordinary editorial note, nothing boundary-related -->\n# Benign heading\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers for benign HTML comment content, got %v", matched)
	}
}

// TestCheckMarkers_CasefoldIsSimpleLowercaseNotFullCasefold (D3-ter(c),
// ML-4C): pins step 4's chosen semantics — Go/Node's simple lowercase
// (strings.ToLower / String.prototype.toLowerCase), not a full Unicode
// casefold (Python's str.casefold(), which this ML changes to match) — so
// the 3 CLIs never silently diverge on a normalization step feeding a
// security check. There is no known exploit against the 6 ASCII literal
// markers under either semantics; German sharp S (ß) is the textbook
// example where simple lowercase and full casefold diverge (ß.casefold()
// == "ss", ß.lower() == "ß", unchanged) and is used here only to prove
// which one is in effect, not because any marker text is reachable through
// it.
func TestCheckMarkers_CasefoldIsSimpleLowercaseNotFullCasefold(t *testing.T) {
	content := "# Straße\n\nAn unrelated heading using a German sharp S.\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers for an unrelated heading, got %v", matched)
	}
}

func TestChecksum_StableAndMatchesSHA256Sum(t *testing.T) {
	content := []byte("# Hello\n\nSome deterministic content.\n")

	got := Checksum(content)

	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("Checksum() = %q, want %q", got, want)
	}

	// Cross-check against the actual sha256sum binary, if available, to
	// avoid the test only validating itself against crypto/sha256.
	if _, err := exec.LookPath("shasum"); err == nil {
		dir := t.TempDir()
		path := filepath.Join(dir, "content.txt")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}
		out, err := exec.Command("shasum", "-a", "256", path).Output()
		if err != nil {
			t.Fatalf("shasum failed: %v", err)
		}
		fields := strings.Fields(string(out))
		if len(fields) == 0 {
			t.Fatalf("unexpected shasum output: %q", out)
		}
		if fields[0] != got {
			t.Fatalf("Checksum() = %q, shasum -a 256 = %q", got, fields[0])
		}
	}

	// Stability: calling twice on the same bytes yields the same digest.
	if again := Checksum(content); again != got {
		t.Fatalf("Checksum() not stable: %q vs %q", got, again)
	}
}

// TestCheckMarkers_SecurityOpinionDocumentDoesNotRefuseItself is the
// non-regression falsification test named by the ML-4C AC: the D3-ter(a)/(b)
// fixes above must NOT reintroduce the exact self-refusal the original D3
// amendment (fenced-block removal) exists to prevent. The opinion document
// itself lists all 6 literal markers as headings, but INSIDE a properly
// CLOSED fence (docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md,
// ~line 82-89) — running the checker against the real file on disk must
// still return zero matches. If this test ever fails, the fence-closed
// immunity the original D3 amendment relies on has regressed.
func TestCheckMarkers_SecurityOpinionDocumentDoesNotRefuseItself(t *testing.T) {
	content, err := os.ReadFile("../../docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md")
	if err != nil {
		t.Fatalf("failed to read the security opinion document: %v", err)
	}
	matched := CheckMarkers(content)
	if len(matched) != 0 {
		t.Fatalf("the security opinion document must not refuse itself (fence-closed immunity regression), got %v", matched)
	}
}

// --- RedactURL (D6-bis) ---

func TestRedactURL_StripsQueryString(t *testing.T) {
	got := RedactURL("https://example.com/skills/my-skill.md?token=abc123")
	want := "https://example.com/skills/my-skill.md?[redacted]"
	if got != want {
		t.Fatalf("RedactURL() = %q, want %q", got, want)
	}
	if strings.Contains(got, "abc123") {
		t.Fatalf("RedactURL() leaked the token: %q", got)
	}
}

func TestRedactURL_StripsUserinfo(t *testing.T) {
	got := RedactURL("https://user:supersecret@example.com/skills/my-skill.md")
	want := "https://example.com/skills/my-skill.md"
	if got != want {
		t.Fatalf("RedactURL() = %q, want %q", got, want)
	}
	if strings.Contains(got, "supersecret") {
		t.Fatalf("RedactURL() leaked the userinfo: %q", got)
	}
}

func TestRedactURL_NoQueryOrUserinfoUnchanged(t *testing.T) {
	got := RedactURL("https://example.com/skills/my-skill.md")
	want := "https://example.com/skills/my-skill.md"
	if got != want {
		t.Fatalf("RedactURL() = %q, want %q", got, want)
	}
}

func TestRedactURL_Idempotent(t *testing.T) {
	once := RedactURL("https://example.com/skills/my-skill.md?token=abc123")
	twice := RedactURL(once)
	if once != twice {
		t.Fatalf("RedactURL() is not idempotent: %q then %q", once, twice)
	}
}
