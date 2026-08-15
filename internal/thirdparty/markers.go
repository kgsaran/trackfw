package thirdparty

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// literalMarkers is the objective, literal list of headings whose presence
// causes a third-party artifact to be refused by default (D3). This is a
// tripwire, not a filter against a competent adversary — see the ADR's
// "o que este critério NÃO cobre" section: paraphrase, indirection,
// fragmentation and residual homoglyphs outside of NFKC all pass.
var literalMarkers = []string{
	"git authority",
	"mode lock",
	"governance prerequisite",
	"reporting boundary",
	"scope boundary",
	"dispatch contract",
}

// htmlCommentPattern matches HTML comments, removed in step 1 of the D3
// normalization pipeline.
var htmlCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)

// fencePrefixPattern detects a fence-opening/closing line: optional leading
// whitespace followed by three or more backticks or tildes. Go's RE2-based
// regexp package does not support backreferences, so matching a specific
// fence delimiter (``` vs ~~~) against its own closer is done by explicit
// line scanning in removeFencedBlocks rather than a single regex.
var fencePrefixPattern = regexp.MustCompile("^\\s*(```+|~~~+)")

// removeFencedBlocks strips fenced code blocks (``` or ~~~), step 2 of the
// D3 pipeline (architect's amendment to the original hades-tf opinion):
// lines inside a fence are not read as headings, otherwise documentation
// that merely quotes the marker list — such as the opinion document
// itself — would be refused by its own criterion. A fence is closed by a
// line starting with the same delimiter character (backtick or tilde),
// with at least as many repeats as the opener — the CommonMark rule.
func removeFencedBlocks(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	var closer string // fence delimiter run that closes the current block, "" if not in a fence
	for _, line := range lines {
		if closer == "" {
			m := fencePrefixPattern.FindStringSubmatch(line)
			if m != nil {
				closer = m[1]
				continue // drop the opening fence line itself
			}
			out = append(out, line)
			continue
		}
		// Inside a fence: drop the line; check if it closes the block.
		trimmed := strings.TrimSpace(line)
		delimChar := closer[0:1]
		if strings.HasPrefix(trimmed, strings.Repeat(delimChar, len(closer))) &&
			strings.Trim(trimmed, delimChar) == "" {
			closer = ""
		}
	}
	return strings.Join(out, "\n")
}

// headingLinePattern matches a single, already-collapsed Markdown heading
// line (level 1-6): "#" through "######" followed by whitespace and the
// heading body. Applied per-line (not with (?m)) after step 5, on text that
// no longer contains internal runs of whitespace.
var headingLinePattern = regexp.MustCompile(`^#{1,6}\s+(.*)$`)

// whitespacePattern collapses runs of internal whitespace, step 5 of the
// D3 pipeline.
var whitespacePattern = regexp.MustCompile(`\s+`)

// CheckMarkers applies the D3 objective-refusal criterion to content and
// returns the literal marker names (from literalMarkers) that matched as a
// heading. The normalization pipeline, in fixed order per the roadmap's
// ML-1A specification:
//  1. remove HTML comments;
//  2. remove fenced code blocks (``` and ~~~) — content inside a fence is
//     never read as a heading;
//  3. NFKC normalize;
//  4. casefold;
//  5. collapse internal whitespace + strip (applied per line, so newlines
//     — needed to keep the "is this line a heading" question meaningful —
//     are preserved as line separators rather than being collapsed away);
//  6. match only lines matching ^#{1,6}\s+ against the literal marker list.
func CheckMarkers(content []byte) []string {
	text := string(content)

	// 1. Remove HTML comments.
	text = htmlCommentPattern.ReplaceAllString(text, "")

	// 2. Remove fenced code blocks — lines inside a fence are not headings.
	text = removeFencedBlocks(text)

	// 3. NFKC normalize.
	text = norm.NFKC.String(text)

	// 4. Casefold.
	text = strings.ToLower(text)

	var matched []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		// 5. Collapse internal whitespace + strip.
		collapsed := strings.TrimSpace(whitespacePattern.ReplaceAllString(line, " "))

		// 6. Match only heading lines against the literal marker list.
		m := headingLinePattern.FindStringSubmatch(collapsed)
		if m == nil {
			continue
		}
		body := m[1]
		for _, marker := range literalMarkers {
			if body == marker && !seen[marker] {
				matched = append(matched, marker)
				seen[marker] = true
			}
		}
	}
	return matched
}

// Checksum returns the SHA-256 hex digest of the raw bytes, before any
// normalization. This mirrors contentHash in internal/integrations/manager.go
// (unexported there, so replicated here rather than imported) — see D6.
func Checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
