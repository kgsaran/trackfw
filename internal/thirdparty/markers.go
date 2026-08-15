package thirdparty

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
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

// htmlCommentPattern matches HTML comments; step 1 of the D3 normalization
// pipeline NEUTRALIZES them (strips only the "<!--"/"-->" delimiters,
// keeping the inner text in place to be scanned) rather than removing the
// comment wholesale — see neutralizeHTMLComments' doc comment (D3-ter(b)).
var htmlCommentPattern = regexp.MustCompile(`(?s)<!--(.*?)-->`)

// neutralizeHTMLComments strips only the HTML comment delimiters ("<!--"
// and "-->"), keeping whatever text was between them in place to be
// scanned by the later steps of the pipeline.
//
// D3-ter(b) amendment: the original D3 text apagava o comentário inteiro
// antes de escanear, o que contradizia a própria justificativa escrita do
// critério — "um agente LLM lê comentário HTML no fluxo de tokens" — ao
// remover exatamente o conteúdo que o agente vai ler. `<!-- ## Git
// authority -->` passava limpo. Reproduzido pelo arquiteto contra os 3
// CLIs antes da correção.
func neutralizeHTMLComments(text string) string {
	return htmlCommentPattern.ReplaceAllString(text, "$1")
}

// fencePrefixPattern detects a fence-opening/closing line: optional leading
// whitespace followed by three or more backticks or tildes. Go's RE2-based
// regexp package does not support backreferences, so matching a specific
// fence delimiter (``` vs ~~~) against its own closer is done by explicit
// line scanning in removeFencedBlocks rather than a single regex.
var fencePrefixPattern = regexp.MustCompile("^\\s*(```+|~~~+)")

// removeFencedBlocks strips fenced code blocks (``` or ~~~), step 2 of the
// D3 pipeline (architect's amendment to the original hades-tf opinion):
// lines inside a CLOSED fence are not read as headings, otherwise
// documentation that merely quotes the marker list — such as the opinion
// document itself — would be refused by its own criterion. A fence is
// closed by a line starting with the same delimiter character (backtick or
// tilde), with at least as many repeats as the opener — the CommonMark
// rule.
//
// D3-ter(a) amendment: an opener with NO matching closer before EOF is not
// a fence for the purpose of this check — the buffered lines (including
// the opener itself) are replayed as ordinary text instead of being
// dropped. Before this amendment, a bare "```" with no closing fence
// swallowed the rest of the document as "fenced" content, silently
// defeating the check for anything after it — reproduced by the architect
// against all 3 CLIs before the fix (content started by an unclosed fence
// and containing "## Git authority" returned no matches). Only a
// PROPERLY-CLOSED fence still grants immunity; that is the original D3
// amendment and is unchanged (without it, the opinion document that lists
// the 6 markers inside a closed fence would refuse itself).
func removeFencedBlocks(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	var buffered []string // lines consumed since the current fence opener; replayed verbatim if it never closes
	var closer string     // fence delimiter run that closes the current block, "" if not in a fence
	for _, line := range lines {
		if closer == "" {
			m := fencePrefixPattern.FindStringSubmatch(line)
			if m != nil {
				closer = m[1]
				buffered = []string{line} // keep the opener in case this fence never closes
				continue
			}
			out = append(out, line)
			continue
		}
		// Inside a (possibly-never-closing) fence: buffer the line, then
		// check if it closes the block.
		buffered = append(buffered, line)
		trimmed := strings.TrimSpace(line)
		delimChar := closer[0:1]
		if strings.HasPrefix(trimmed, strings.Repeat(delimChar, len(closer))) &&
			strings.Trim(trimmed, delimChar) == "" {
			closer = ""
			buffered = nil // closed properly: the buffered fenced content is discarded, as before
		}
	}
	if closer != "" {
		// Reached EOF still "inside" a fence that never closed (D3-ter(a)):
		// not a fence at all — replay every buffered line, including the
		// opener, as ordinary text to be scanned.
		out = append(out, buffered...)
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
// ML-1A specification (amended by D3-ter):
//  1. neutralize HTML comments (strip delimiters, keep inner content —
//     D3-ter(b));
//  2. remove PROPERLY-CLOSED fenced code blocks (``` and ~~~) — content
//     inside a closed fence is never read as a heading; an unclosed fence
//     is NOT a fence for this purpose and its content is scanned normally
//     (D3-ter(a));
//  3. NFKC normalize;
//  4. casefold — deliberately Go's strings.ToLower (simple lowercase), not
//     a full Unicode casefold: unified across the 3 CLIs by D3-ter(c) so
//     none of them silently diverges on a normalization step that feeds a
//     security check. There is no known exploit against the 6 ASCII
//     literal markers either way; this is a consistency fix, not a
//     vulnerability fix — see
//     TestCheckMarkers_CasefoldIsSimpleLowercaseNotFullCasefold;
//  5. collapse internal whitespace + strip (applied per line, so newlines
//     — needed to keep the "is this line a heading" question meaningful —
//     are preserved as line separators rather than being collapsed away);
//  6. match only lines matching ^#{1,6}\s+ against the literal marker list.
func CheckMarkers(content []byte) []string {
	text := string(content)

	// 1. Neutralize HTML comments — strip only the delimiters, keep the
	// inner content in place to be scanned (D3-ter(b); see
	// neutralizeHTMLComments' doc comment for why this changed from a
	// wholesale removal).
	text = neutralizeHTMLComments(text)

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

// RedactURL returns rawURL with its query string — and userinfo, if
// present — replaced by the literal marker "[redacted]" (D6-bis). Used
// before persisting a third-party artifact's source URL to disk (the
// quarantine record and the provenance entry): a pre-signed URL can carry a
// bearer token in its query string, which would otherwise become a
// permanent secret in the git history the moment either file is committed.
// The full, unredacted URL is used only in memory, for the network fetch
// itself (D7) — never for anything persisted.
//
// If rawURL fails to parse, it is returned unchanged: by the time any
// caller of this function runs, url.Parse has already succeeded once
// upstream (Fetch validates the scheme before the first request), so a
// parse failure here would indicate a caller bug, not adversarial input
// this function needs to defend against.
func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = nil
	if parsed.RawQuery != "" {
		parsed.RawQuery = "[redacted]"
	}
	return parsed.String()
}
