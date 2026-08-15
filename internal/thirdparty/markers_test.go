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

func TestCheckMarkers_HTMLCommentStrippedBeforeMatch(t *testing.T) {
	// A marker hidden inside an HTML comment is stripped in step 1 before
	// the heading regex ever sees it — it must never surface as a false
	// match on the surrounding text.
	content := "<!-- ## Git authority -->\n# Benign heading\n"
	matched := CheckMarkers([]byte(content))
	if len(matched) != 0 {
		t.Fatalf("expected no markers for content where the marker was only inside an HTML comment, got %v", matched)
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
