// Package roadmapdoc implements the string-level parser for trackfw roadmap files.
//
// It is a leaf package — it must not import internal/commands or internal/validator.
// Its exported API is consumed by internal/commands (barrier), internal/validator,
// and internal/serve.
//
// All parsing rules are pinned by docs/cli-parity.md
// ("Roadmap parsing rules — string-level — no heuristics").
// Logic is moved byte-for-byte from internal/commands/barrier.go (ML-1A, REQ #392).
package roadmapdoc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ────────────────────────────────────────────────────────────────────────────
// Regexes (pinned by docs/cli-parity.md)
// ────────────────────────────────────────────────────────────────────────────

var (
	// WaveHeadingRe detects any "## Wave <token> " heading, including malformed ones.
	// The captured token is validated separately by WaveLabelRe before being stored.
	WaveHeadingRe = regexp.MustCompile(`^## Wave (\S+) `)
	// WaveLabelRe validates a wave label token in isolation against the grammar pinned in
	// docs/cli-parity.md ("Wave label grammar"): <integer>[-<suffix>] where suffix is [a-zA-Z0-9]+.
	// AC3-ter (REQ #392 / ML-1B): the suffix is matched case-insensitively so that labels like
	// "3-Py" (authored with an upper-case P in four real roadmaps) are accepted. The integer
	// part must still start with a digit — "reaberta" and "abc" remain invalid.
	// ML-1D (REQ #392): the hyphen before the suffix is now optional — "1b" and "1-b" are both
	// valid. The integer-part constraint is unchanged: labels like "abc" or "reaberta" (no
	// leading digit) remain invalid. Counter-example that must still fail: "X", "abc", "reaberta".
	WaveLabelRe = regexp.MustCompile(`^\d+(?:-?[a-zA-Z0-9]+)?$`)
	MLHeadingRe      = regexp.MustCompile(`^### (ML-\S+)`)
	StatusLineRe     = regexp.MustCompile(`^\*\*Status:\*\*(.*)$`)
	CriteriaHeaderRe = regexp.MustCompile(`^\*\*(?:Acceptance criteria|Crit[eé]rios de aceite):\*\*`)
	UnmetCriterionRe = regexp.MustCompile(`^- \[ \]`)
	CriterionLineRe  = regexp.MustCompile(`^- \[.\]`)
	BoldLineRe       = regexp.MustCompile(`^\*\*`)
	GatesHeaderRe    = regexp.MustCompile(`^\*\*Gates da wave:\*\*`)
)

// ────────────────────────────────────────────────────────────────────────────
// Status vocabulary and normalization
// ────────────────────────────────────────────────────────────────────────────

// diacriticsFolder normalizes to NFD and strips combining marks (unicode.Mn),
// folding accented Latin letters to their base form — e.g. "Concluído" →
// "Concluido". Used only by StatusIsComplete (rule 3), never for display.
// SAFE to run only AFTER hasDisallowedCombiningMark has cleared the raw
// (pre-decomposition) token — see that function's doc comment for why order
// matters.
var diacriticsFolder = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// statusVocabulary is the closed set of first-token markers recognised as
// "complete" (ADR decision 3): the checkmark emoji, and the English/Portuguese
// words, matched after case-folding and diacritics-folding. Deliberately
// closed and explicit — "feito", "ok", "finalizado" are out (ADR, Alternatives
// Considered — "accept any non-empty status" is the rejected no-op design).
var statusVocabulary = map[string]bool{
	"✅":         true,
	"done":      true,
	"concluido": true,
}

// vs16 is the single variation selector this package treats as cosmetic
// noise: the one emoji keyboards insert after "✅" to force text-style
// presentation, producing "✅️" — visually identical to "✅" (ADR
// 2026-08-29 decision 9, exception clause). No other variation selector or
// combining mark gets this treatment.
const vs16 rune = 0xFE0F

// stripVS16 removes only U+FE0F occurrences from token. Deliberately narrower
// than "strip every variation selector" or "strip every Mn": VS16 is the one
// documented cosmetic exception (ADR decision 9); anything else of category
// Mn is now a rejection, not a fold (see hasDisallowedCombiningMark).
func stripVS16(token string) string {
	return strings.Map(func(r rune) rune {
		if r == vs16 {
			return -1
		}
		return r
	}, token)
}

// hasDisallowedCombiningMark reports whether token (with VS16 already
// removed) contains any Unicode category Mn (Mark, Nonspacing) codepoint.
// ADR 2026-08-29 decision 9: after the single VS16 exception, any combining
// mark on the first status token is rejected outright — the ML is NOT
// complete — rather than folded away. A vocabulary this small exists to
// refuse ambiguity; silently dobrar combining marks reopens exactly the
// ambiguity it exists to close ("d<U+1DC0>one" must not read as "done").
//
// ORDER IS LOAD-BEARING: this check MUST run on the token BEFORE NFD
// decomposition, not after. "Concluído" in its authored (NFC) form has no
// literal Mn codepoint — the accented "í" is a single precomposed Ll
// codepoint. NFD-decomposing it *produces* a trailing Mn (U+0301, COMBINING
// ACUTE ACCENT) that diacriticsFolder then strips for vocabulary matching.
// Running this rejection check on the already-decomposed string would treat
// that legitimate, vocabulary-sanctioned accent the same as an injected
// combining mark and reject "Concluído" outright, breaking AC15's own
// positive case. Checking the raw, pre-decomposition token instead lets it
// through here (no literal Mn present) while still catching a combining
// mark that was authored directly onto the token, which is category Mn in
// either form, decomposed or not.
func hasDisallowedCombiningMark(token string) bool {
	for _, r := range token {
		if unicode.Is(unicode.Mn, r) {
			return true
		}
	}
	return false
}

// normalizeStatusToken folds diacritics and lower-cases token for vocabulary
// comparison. Never mutates the emoji marker (no combining marks, no case).
// Callers MUST have already rejected via hasDisallowedCombiningMark before
// calling this — see that function's doc comment for why.
func normalizeStatusToken(token string) string {
	folded, _, err := transform.String(diacriticsFolder, token)
	if err != nil {
		folded = token
	}
	return strings.ToLower(folded)
}

// StatusIsComplete implements rule 3 by FIRST TOKEN, not substring (ADR
// decision 3, AC8/AC9/AC14). marker is the already-trimmed remainder of the
// "**Status:**" line. strings.Fields splits on unicode.IsSpace, which treats
// U+00A0 (NBSP) as a separator — matching the accepted "NBSP separator" case —
// while a zero-width character (U+200B, not in unicode.IsSpace) stays glued to
// the token and safely causes rejection (a usability false-negative, not a
// security concern — see the ML-0A threat model, § residual 7).
//
// This is the fix for vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md:
// `strings.Contains(marker, "✅")` would classify "**Status:** ⬜ Pendente ✅" as
// complete (reproduced live against 7.3.0 — ADR decision 8). First-token
// comparison rejects it because the first token is "⬜", not the marker.
//
// ADR decision 9: VS16 is stripped first (the one cosmetic exception), then
// any remaining combining mark on the raw token rejects outright — see
// hasDisallowedCombiningMark for why this must happen before NFD.
func StatusIsComplete(marker string) bool {
	fields := strings.Fields(marker)
	if len(fields) == 0 {
		return false
	}
	first := stripVS16(fields[0])
	if hasDisallowedCombiningMark(first) {
		return false
	}
	return statusVocabulary[normalizeStatusToken(first)]
}

// ────────────────────────────────────────────────────────────────────────────
// AC3-bis: three-category status classification
// ────────────────────────────────────────────────────────────────────────────

// StatusCat is one of the three status categories defined by ADR 2026-09-18.
type StatusCat int

const (
	// StatusPending covers markers that block a done transition:
	// ⬜, 🔄, pending, PENDENTE, ❌ Bloqueado.
	StatusPending StatusCat = iota
	// StatusComplete covers markers that satisfy StatusIsComplete:
	// ✅, done, Concluído, CONCLUIDO, etc.
	StatusComplete
	// StatusTerminated covers explicitly terminated (not forgotten) MLs:
	// ABANDONADO, 🚫 Abandonado, ❌ Cancelado.
	StatusTerminated
)

// terminatedVocabulary covers explicit termination markers (first token, lower-cased
// and diacritics-folded). These release a done transition because the omission was
// deliberate and recorded, not forgotten.
var terminatedVocabulary = map[string]bool{
	"abandonado": true,
	"🚫":         true,
}

// canceladoToken is the second-token disambiguator for the ❌ first-token case.
// "❌ Cancelado" → Terminated; "❌ Bloqueado" → Pending.
// Both strings are compared after normalizeStatusToken.
const canceladoNormalized = "cancelado"

// StatusCategory returns the three-way classification of a "**Status:**" marker.
// It does NOT alter StatusIsComplete — that function remains the binary gate
// the barrier uses (ADR decision 3). This function exists for the move-to-done
// gate (ML-3A) which needs to distinguish a deliberately terminated ML from a
// merely pending one.
//
// Classification:
//   - Complete   — StatusIsComplete(marker) returns true.
//   - Terminated — first token (after VS16 strip) normalizes to "abandonado" or "🚫",
//     OR first token is "❌" and second token normalizes to "cancelado".
//   - Pending    — everything else, including "❌ Bloqueado".
func StatusCategory(marker string) StatusCat {
	if StatusIsComplete(marker) {
		return StatusComplete
	}
	fields := strings.Fields(marker)
	if len(fields) == 0 {
		return StatusPending
	}
	first := stripVS16(fields[0])
	// hasDisallowedCombiningMark check not needed here: even if it fires, the
	// marker isn't complete (StatusIsComplete already returned false) and we fall
	// through to Pending, which is the safe default.
	firstNorm := normalizeStatusToken(first)

	if terminatedVocabulary[firstNorm] {
		return StatusTerminated
	}
	// ❌ is ambiguous by first token: check second token.
	if first == "❌" && len(fields) >= 2 {
		secondNorm := normalizeStatusToken(fields[1])
		if secondNorm == canceladoNormalized {
			return StatusTerminated
		}
	}
	return StatusPending
}

// ────────────────────────────────────────────────────────────────────────────
// Line splitting and fence masking
// ────────────────────────────────────────────────────────────────────────────

// SplitRoadmapLines is the single boundary where the raw file content becomes
// the []string every marker regex operates on. It normalizes CRLF line
// endings by stripping a trailing "\r" from each line produced by splitting
// on "\n" — every downstream marker (ML heading, "**Status:**", acceptance
// header, criterion lines, "**Gates da wave:**", the fence delimiter) then
// sees the same content it would see for an LF-only file. It does NOT handle
// a lone-CR (old-Mac-style) file: splitting on "\n" alone leaves such a file
// as one giant line, in both this function and its Node.js counterpart.
// Python's universal-newlines read (see pypi/trackfw/commands/barrier.py
// _split_roadmap_lines) does handle lone CR — a known, accepted asymmetry:
// issue #216 (the defect motivating this fix) is CRLF specifically, and nothing
// in this REQ's scope produces or is known to produce lone-CR roadmaps.
//
// This runtime does not currently depend on this normalization to pass a
// CRLF roadmap end-to-end — statusLineRe's "(.*)$" already matches a trailing
// "\r" (Go's RE2 "." excludes only "\n"), and every downstream comparison
// goes through strings.TrimSpace, which treats "\r" as whitespace. That is an
// accident of the specific combination of primitives used today, not a
// contract: a future marker added with an exact-equality comparison (no
// TrimSpace) or a "." used together with a stricter multiline mode would
// reintroduce the defect this function exists to prevent once, at the
// boundary, instead of at every regex site (mirrors npm/src/commands/barrier.js
// splitRoadmapLines and the universal-newline read in pypi/trackfw/commands/barrier.py).
//
// Only the trailing "\r" immediately before the split point is stripped —
// this must never be confused with per-line indentation trimming, which
// ML-1B deliberately removed: leading whitespace on a marker line still fails
// to match (markers are anchored at column 0, untouched by this function).
func SplitRoadmapLines(data string) []string {
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// DetectFenceMarker inspects a whitespace-trimmed line and reports whether it
// opens or closes a CommonMark-style fence: a run of 3+ identical backtick
// (`) or tilde (~) characters at the start of the line. Returns the fence
// character and the length of the run. ADR decision (ML-1B, achado 1):
// CommonMark defines a fence as 3+ of the SAME character (backtick or tilde),
// closed by a run of the same character with length >= the opening run —
// masking only "```" left both "~~~" fences and 4+-backtick fences (whose
// interior can nest a 3-backtick block) unmasked, which is the escape route a
// hostile roadmap would use.
func DetectFenceMarker(trimmed string) (ch byte, length int, ok bool) {
	if trimmed == "" {
		return 0, 0, false
	}
	first := trimmed[0]
	if first != '`' && first != '~' {
		return 0, 0, false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == first {
		i++
	}
	if i < 3 {
		return 0, 0, false
	}
	return first, i, true
}

// FenceMask returns, for each line index, whether that line lies strictly
// inside a fenced code block (``` ... ``` or ~~~ ... ~~~, per CommonMark: 3+
// of the same fence character, closed by a run of the same character with
// length >= the opening run's length). A line that is itself a fence
// delimiter is never reported as "inside" — only the lines between an opening
// and a closing delimiter are masked. ADR decision 7 / AC13: MLHeadingRe,
// StatusLineRe and CriteriaHeaderRe must ignore documentation/examples inside
// a cerca — otherwise a roadmap that cites those literals (as this very
// roadmap, its REQ and its ADR do, repeatedly) is read as real ML content.
// ParseGates already has its own, independent fence-matching for the
// "```bash ... ```" gates block and is untouched by this mask.
func FenceMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	fenced := false
	var fenceChar byte
	var fenceLen int
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		ch, length, isFence := DetectFenceMarker(trimmed)
		if !fenced {
			if isFence {
				fenced = true
				fenceChar = ch
				fenceLen = length
				continue
			}
			continue
		}
		// Currently inside a fence: only a marker of the SAME character with
		// length >= the opening run closes it (a nested shorter/different
		// marker stays masked as interior content) — AND, per CommonMark,
		// a closing fence line contains NOTHING besides the fence run and
		// (already-stripped) surrounding whitespace: length == len(trimmed).
		// This is the fix for the hades-tf security review (2026-08-29,
		// achado #1 / vault/notes/barrier-fence-closing-trailing-content-bypass-2026-08-29.md):
		// a line like "```trailing-junk" found INSIDE an open fence does not
		// close it in real CommonMark — it stays interior content — but the
		// prior check treated any run of length >= fenceLen as a valid close
		// regardless of trailing text, letting a forged "example" fence close
		// early and expose the rest of the example as real ML content. The
		// opening branch above is intentionally unchanged: CommonMark allows
		// an info string after the opening run (` ```bash `).
		if isFence && ch == fenceChar && length >= fenceLen && length == len(trimmed) {
			fenced = false
			continue
		}
		mask[i] = true
	}
	return mask
}

// ────────────────────────────────────────────────────────────────────────────
// Wave and ML block types
// ────────────────────────────────────────────────────────────────────────────

// WaveBlock delimits one "## Wave <label> ..." section: [Start, End) line indices (0-based).
// Label is the wave label string (e.g. "1", "2-bis", "1b") per the grammar in docs/cli-parity.md.
type WaveBlock struct {
	Label string
	Start int
	End   int
}

// MalformedWave records a wave heading whose label token failed grammar validation.
// ParseWaves returns these instead of aborting: the wave is isolated (not included in the
// returned WaveBlock slice), but parsing continues for the rest of the document (ML-1D/ML-1E,
// REQ #392 — emends ADR-2026-07-29 decision 16). The MalformedWave implements error so
// callers can format messages uniformly (e.g. "trackfw barrier: " + mw.Error()).
//
// The barrier translates every MalformedWave into a failure entry in the wave_headings check
// (ML-1E, REQ #392). This ensures no "passed" verdict is emitted while the document contains
// an unaudited wave — preserving ADR-2026-07-29 decision 16's principle ("reprovar alto") while
// allowing the valid waves to continue being evaluated (remedy change: check that blocks, not
// document abort). MLs inside malformed waves are also covered by HasUnfinishedMLs, which
// returns true whenever len(malformed) > 0 (fail-safe closed, belt-and-suspenders).
type MalformedWave struct {
	Line  int    // 1-based line number of the ## Wave heading
	Token string // the literal label token that failed grammar validation
}

// Error implements error. Format is the same as the old ParseWaves error so that any
// tooling that pins the message string continues to work.
func (m MalformedWave) Error() string {
	return fmt.Sprintf("malformed wave heading at line %d: \"%s\" is not a valid wave label", m.Line, m.Token)
}

// MLBlock delimits one "### ML-..." section within a wave: [Start, End) line indices.
type MLBlock struct {
	ID    string
	Start int
	End   int
}

// ────────────────────────────────────────────────────────────────────────────
// Wave and ML parsing
// ────────────────────────────────────────────────────────────────────────────

// ParseWaves splits the roadmap into wave blocks (rule 1).
//
// Malformed headings are ISOLATED, not aborted (ML-1D/ML-1E, REQ #392 — emends ADR-2026-07-29
// decision 16). Each invalid label is recorded in the returned []MalformedWave slice and
// parsing continues for the rest of the document. The safe basis is that every "## Wave …"
// heading is an H2; the block-end scan (strings.HasPrefix(lines[j], "## ")) still closes the
// preceding valid wave at the malformed heading, so valid-wave boundaries are never corrupted.
//
// 🔴 Never fail open: malformed waves are NOT added to the WaveBlock slice. The barrier
// translates each MalformedWave into a failure in the wave_headings check (ML-1E), blocking
// the overall verdict. HasUnfinishedMLs also returns true whenever len(malformed) > 0 as a
// belt-and-suspenders fail-safe.
func ParseWaves(lines []string) ([]WaveBlock, []MalformedWave) {
	var waves []WaveBlock
	var malformed []MalformedWave
	n := len(lines)
	for i := 0; i < n; i++ {
		m := WaveHeadingRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		token := m[1]
		// Validate label against the grammar pinned in docs/cli-parity.md.
		// Using literal quotes (not %q) so the token is emitted verbatim across runtimes.
		if !WaveLabelRe.MatchString(token) {
			malformed = append(malformed, MalformedWave{Line: i + 1, Token: token})
			continue
		}
		// Integer part must be >= 0 — 0 is a valid wave label (Wave 0 threat-model convention,
		// docs/cli-parity.md § "Wave label grammar"). Mirrors the flag-validation constraint above.
		intVal, _ := SplitWaveLabel(token)
		if intVal < 0 {
			malformed = append(malformed, MalformedWave{Line: i + 1, Token: token})
			continue
		}
		end := n
		for j := i + 1; j < n; j++ {
			if strings.HasPrefix(lines[j], "## ") {
				end = j
				break
			}
		}
		waves = append(waves, WaveBlock{Label: token, Start: i, End: end})
	}
	return waves, malformed
}

// SplitWaveLabel splits a valid wave label into its integer and optional suffix parts.
// For "2-bis" it returns (2, "bis"); for "3" it returns (3, ""); for "1b" it returns (1, "b").
//
// ML-1D (REQ #392): the suffix may be attached directly to the integer without a hyphen
// ("1b") or separated by a hyphen ("1-b"). Both forms are valid per WaveLabelRe after
// the ML-1D grammar fix. SplitWaveLabel normalises both to (integer, suffix) without the
// hyphen — so CompareWaveLabels("1b", "1-b") == 0.
//
// The label must already be valid per WaveLabelRe; behaviour on invalid input is undefined.
func SplitWaveLabel(label string) (integer int, suffix string) {
	// Walk past the leading digit run.
	i := 0
	for i < len(label) && label[i] >= '0' && label[i] <= '9' {
		i++
	}
	integer, _ = strconv.Atoi(label[:i])
	if i < len(label) {
		rest := label[i:]
		// Strip leading hyphen if present (e.g. "1-b" → rest "-b" → "b").
		if rest[0] == '-' {
			rest = rest[1:]
		}
		suffix = rest
	}
	return
}

// CompareWaveLabels returns -1, 0, or 1 comparing two wave labels per the ordering defined
// in docs/cli-parity.md (§ "Wave label grammar"):
//  1. Compare integer parts numerically.
//  2. On a tie, no-suffix precedes with-suffix.
//  3. On a tie between two suffixes, compare lexicographically (case-insensitive).
//
// So "2" < "2-bis" < "2-hotfix" < "3". Used where waves must be listed or compared.
//
// AC3-ter (REQ #392 / ML-1B): suffixes are lower-cased before comparison so that
// "3-Py" and "3-py" sort identically — the casing of the authored label must not
// affect ordering.
func CompareWaveLabels(a, b string) int {
	aInt, aSuf := SplitWaveLabel(a)
	bInt, bSuf := SplitWaveLabel(b)
	if aInt != bInt {
		if aInt < bInt {
			return -1
		}
		return 1
	}
	// Normalize suffix to lower-case before ordering (AC3-ter).
	aSuf = strings.ToLower(aSuf)
	bSuf = strings.ToLower(bSuf)
	// integers equal — no-suffix before with-suffix
	if aSuf == "" && bSuf != "" {
		return -1
	}
	if aSuf != "" && bSuf == "" {
		return 1
	}
	// both have a suffix (or both have none): compare lexicographically
	if aSuf < bSuf {
		return -1
	}
	if aSuf > bSuf {
		return 1
	}
	return 0
}

// ParseMLs splits a wave block into ML blocks (rule 2). fenced marks, per
// line index into lines, whether that line lies inside a fenced code block
// (FenceMask) — a "### ML-XX" heading inside a cerca is documentation, not a
// real ML, and must not become a phantom ML (ADR decision 7, AC13-b).
func ParseMLs(lines []string, fenced []bool, waveStart, waveEnd int) []MLBlock {
	var mls []MLBlock
	for i := waveStart; i < waveEnd; i++ {
		if fenced[i] {
			continue
		}
		m := MLHeadingRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		end := waveEnd
		for j := i + 1; j < waveEnd; j++ {
			if fenced[j] {
				continue
			}
			if strings.HasPrefix(lines[j], "### ") || strings.HasPrefix(lines[j], "## ") {
				end = j
				break
			}
		}
		mls = append(mls, MLBlock{ID: m[1], Start: i, End: end})
	}
	return mls
}

// MLStatusMarker returns the trimmed remainder of the ML's "**Status:**" line,
// if any (rule 3). Lines inside a fenced code block are ignored — a "**Status:**"
// cited inside a cerca (e.g. as documentation of this very defect) is not the
// ML's real status (ADR decision 7, AC13-a).
func MLStatusMarker(lines []string, fenced []bool, ml MLBlock) (marker string, found bool) {
	for i := ml.Start; i < ml.End; i++ {
		if fenced[i] {
			continue
		}
		if m := StatusLineRe.FindStringSubmatch(lines[i]); m != nil {
			return strings.TrimSpace(m[1]), true
		}
	}
	return "", false
}

// AcceptanceEvaluate implements rule 4. hasBlock is false both when the
// acceptance header (English or Portuguese) is absent and when it is present but
// the body between it and the next "**" line (or ML boundary) contains zero
// "- [...]" criterion lines — an empty block is not vacuously passed, per the
// contract. Lines inside a fenced code block are ignored throughout — for the
// header search, for the "**" block-end boundary, and for counting criterion
// lines — otherwise a cerca citing "**Critérios de aceite:**"/"- [x]" as an
// example would forge acceptance evidence (ADR decision 7, AC13-a).
func AcceptanceEvaluate(lines []string, fenced []bool, ml MLBlock) (met, unmet int, hasBlock bool) {
	headerLine := -1
	for i := ml.Start; i < ml.End; i++ {
		if fenced[i] {
			continue
		}
		if CriteriaHeaderRe.MatchString(lines[i]) {
			headerLine = i
			break
		}
	}
	if headerLine < 0 {
		return 0, 0, false
	}
	blockEnd := ml.End
	for j := headerLine + 1; j < ml.End; j++ {
		if fenced[j] {
			continue
		}
		if BoldLineRe.MatchString(lines[j]) {
			blockEnd = j
			break
		}
	}

	total := 0
	unmetCount := 0
	for i := headerLine + 1; i < blockEnd; i++ {
		if fenced[i] {
			continue
		}
		line := lines[i]
		if UnmetCriterionRe.MatchString(line) {
			total++
			unmetCount++
			continue
		}
		if CriterionLineRe.MatchString(line) {
			total++
		}
	}
	if total == 0 {
		return 0, 0, false
	}
	return total - unmetCount, unmetCount, true
}

// ParseGates implements rule 5. A wave with no "**Gates da wave:**" block returns an
// empty, non-nil slice — zero gates is legal and the barrier never invents one.
func ParseGates(lines []string, waveStart, waveEnd int) ([]string, error) {
	for i := waveStart; i < waveEnd; i++ {
		if !GatesHeaderRe.MatchString(lines[i]) {
			continue
		}
		j := i + 1
		for j < waveEnd && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= waveEnd || strings.TrimSpace(lines[j]) != "```bash" {
			return nil, fmt.Errorf("malformed gates block at line %d: expected a ```bash fence immediately after '**Gates da wave:**'", i+1)
		}
		fenceStart := j
		var cmds []string
		k := j + 1
		closed := false
		for k < waveEnd {
			if strings.TrimSpace(lines[k]) == "```" {
				closed = true
				break
			}
			line := strings.TrimSpace(lines[k])
			if line != "" && !strings.HasPrefix(line, "#") {
				cmds = append(cmds, line)
			}
			k++
		}
		if !closed {
			return nil, fmt.Errorf("unterminated gates fence starting at line %d", fenceStart+1)
		}
		if cmds == nil {
			cmds = []string{}
		}
		return cmds, nil
	}
	return []string{}, nil
}

// HasUnfinishedMLs reports whether a roadmap (given as raw file content) has
// any ML whose status does not satisfy StatusIsComplete and is not Terminated.
// This is the predicate for AC3: a roadmap that "reached done without a gate
// that rejects it" is one where this function returns true.
//
// The function reads all waves in the document (not just one); it is meant to
// be applied to the roadmap as a whole, not to a single wave.
func HasUnfinishedMLs(data string) bool {
	lines := SplitRoadmapLines(data)
	fenced := FenceMask(lines)
	waves, malformed := ParseWaves(lines)
	if len(malformed) > 0 {
		// Malformed wave heading: treat as having unfinished content (fail safe).
		// The MLs inside malformed waves are unreachable by wave-scoped barrier calls
		// and their completeness cannot be proven — so we fail closed.
		return true
	}
	for _, wave := range waves {
		mls := ParseMLs(lines, fenced, wave.Start, wave.End)
		for _, ml := range mls {
			marker, found := MLStatusMarker(lines, fenced, ml)
			if !found {
				return true
			}
			cat := StatusCategory(marker)
			if cat == StatusPending {
				return true
			}
		}
	}
	return false
}
