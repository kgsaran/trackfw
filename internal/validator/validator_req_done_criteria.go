package validator

// validator_req_done_criteria.go — D4 (ADR-2026-10-04, REQ #514 ML-1B).
//
// Rule req_done_open_criteria: a REQ with Status: Done that has at least one
// acceptance-criteria checkbox "- [ ]" WITHOUT a valid Caducou: continuation
// (see roadmapdoc.LapsedContinuationRe) fires a WARNING.
//
// Cutoff: 2026-10-04, the date this rule was introduced. REQs created strictly
// before the cutoff are exempt and counted in a single aggregated notice line
// (same "isenção visível" pattern as req_has_roadmap / reqRoadmapGrandfatherNotice).
//
// 🔴 Direction of the cutoff is the OPPOSITE of req_has_roadmap:
//   - req_has_roadmap: rule existed before, cutoff grants AMNESTY to the old backlog
//     → oldest date that zeroes the backlog (2026-09-03).
//   - req_done_open_criteria: RULE IS NEW. Charging REQs closed before this rule existed
//     is retroactive. Cutoff is the ENTRY DATE (2026-10-04), not the oldest pre-rule date.
//
// Measured (2026-10-04): 126 Done REQs with open boxes in their AC section, all
// dated before 2026-10-04 → 0 individual warnings, 1 aggregated notice with count 126.
//
// Severity: "warning" (registered in ruleDefaults in validator.go).
//
// How open criteria in a REQ are counted:
//   1. Find the AC section heading (## Acceptance Criteria or ## Critérios de aceite).
//   2. Find the end boundary (next ## heading or EOF).
//   3. Build a synthetic slice with "**Acceptance criteria:**" prepended, so
//      roadmapdoc.AcceptanceEvaluateFull can find the header via CriteriaHeaderRe.
//   4. Call AcceptanceEvaluateFull(sub, subFenced, MLBlock{Start:0, End:len(sub)}).
//      detail.Unmet is the count of open criteria (lapsed ones are NOT open).
//
// This reuses the HTML-comment mask and fence logic inside AcceptanceEvaluateFull
// without adding new code to the roadmapdoc package.

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/roadmapdoc"
)

// ---------------------------------------------------------------------------
// Cutoff constant
// ---------------------------------------------------------------------------

// reqDoneOpenCriteriaCutoff is the date of introduction of req_done_open_criteria.
// REQs created strictly before this date are exempt.
// 🔴 This is a FORWARD cutoff: the rule is new, retroactive charging is prohibited.
const reqDoneOpenCriteriaCutoff = "2026-10-04"

func reqDoneOpenCriteriaCutoffDate() time.Time {
	d, err := time.Parse("2006-01-02", reqDoneOpenCriteriaCutoff)
	if err != nil {
		return time.Time{}
	}
	return d
}

// reqDoneOpenCriteriaExempt returns true if the REQ was created strictly before
// the cutoff (i.e. under the old regime, before this rule existed).
// Data ilegível → false (fail-closed: charged, not exempt).
func reqDoneOpenCriteriaExempt(content, path string) bool {
	d, ok := reqCreationDate(content, path)
	if !ok {
		return false
	}
	return d.Before(reqDoneOpenCriteriaCutoffDate())
}

// ---------------------------------------------------------------------------
// AC section detector
// ---------------------------------------------------------------------------

// reqACHeadingRe matches the ## Acceptance Criteria (and Portuguese variants)
// heading in REQ files. The § character and parenthetical suffixes are allowed.
var reqACHeadingRe = regexp.MustCompile(`(?i)^## (?:Acceptance Criteria|Crit[eé]rios de [Aa]ceite)`)

// nextH2Re matches any level-2 heading (end of a section boundary).
var nextH2Re = regexp.MustCompile(`^## `)

// countREQOpenCriteria counts the number of acceptance-criteria boxes in the
// REQ document that are "- [ ]" WITHOUT a valid Caducou: continuation.
// Returns 0 if the document has no AC section or has only met/lapsed criteria.
//
// Implementation: locate the AC section, build a synthetic slice with a bold
// CriteriaHeaderRe-matching line prepended, and delegate to AcceptanceEvaluateFull.
// This reuses the HTML-comment mask and fence-skip logic without adding new code
// to the roadmapdoc package.
func countREQOpenCriteria(content string) int {
	lines := roadmapdoc.SplitRoadmapLines(content)
	fenced := roadmapdoc.FenceMask(lines)

	// Locate the AC section heading (unfenced only).
	headingIdx := -1
	for i, line := range lines {
		if fenced[i] {
			continue
		}
		if reqACHeadingRe.MatchString(line) {
			headingIdx = i
			break
		}
	}
	if headingIdx < 0 {
		return 0
	}

	// Find the section end: next unfenced ## heading after headingIdx, or EOF.
	sectionEnd := len(lines)
	for j := headingIdx + 1; j < len(lines); j++ {
		if fenced[j] {
			continue
		}
		if nextH2Re.MatchString(lines[j]) {
			sectionEnd = j
			break
		}
	}

	// Build synthetic slice: prepend a bold header line so AcceptanceEvaluateFull
	// can find CriteriaHeaderRe at index 0.
	sub := make([]string, 0, 1+(sectionEnd-(headingIdx+1)))
	sub = append(sub, "**Acceptance criteria:**")
	sub = append(sub, lines[headingIdx+1:sectionEnd]...)

	subFenced := make([]bool, len(sub))
	// subFenced[0] = false (the synthetic header is never fenced)
	copy(subFenced[1:], fenced[headingIdx+1:sectionEnd])

	ml := roadmapdoc.MLBlock{Start: 0, End: len(sub)}
	detail := roadmapdoc.AcceptanceEvaluateFull(sub, subFenced, ml)
	return detail.Unmet
}

// ---------------------------------------------------------------------------
// Validator
// ---------------------------------------------------------------------------

// validateREQDoneOpenCriteria implements rule req_done_open_criteria (D4).
// Returns:
//   - violations: per-REQ messages for REQs that are on/after the cutoff and have
//     open criteria. (These are fed to applyRule as warnings, not errors, because
//     the rule is registered in ruleDefaults with severity "warning".)
//   - exemptCount: REQs skipped because they pre-date the cutoff.
//   - enforcedCount: REQs that fired (len(violations)).
//   - scannedCount: total Done REQs inspected.
//   - err: configuration or filesystem error.
func validateREQDoneOpenCriteria() (violations []string, exemptCount, enforcedCount, scannedCount int, err error) {
	cfg := config.Load()
	files, filesErr := resolveREQFiles(cfg)
	if filesErr != nil {
		return nil, 0, 0, 0, filesErr
	}

	for _, path := range files {
		rawBytes, ok := readFileForRule("req_done_open_criteria", path, &violations)
		if !ok {
			continue
		}
		s := string(rawBytes)
		if !reqStatusIsDone(s) {
			continue
		}
		scannedCount++

		openCount := countREQOpenCriteria(s)
		if openCount == 0 {
			// No open criteria — rule would not fire regardless. Skip.
			continue
		}

		if reqDoneOpenCriteriaExempt(s, path) {
			// Has open criteria but pre-dates the rule — exempt.
			exemptCount++
			continue
		}

		violations = append(violations, fmt.Sprintf(
			"REQ %q has Status: Done but contains open acceptance criteria (- [ ] without Caducou:); mark as met or add a Caducou: justification (req_done_open_criteria, D4 ADR-2026-10-04)",
			filepath.Base(path),
		))
		enforcedCount++
	}
	return violations, exemptCount, enforcedCount, scannedCount, nil
}

// reqDoneOpenCriteriaNotice is the aggregated single-line notice that makes
// the exemption VISIBLE. Emitted unconditionally after any Done REQ scan
// (even with 0 exempt), so the denominator (scanned) prevents vacuous silence.
func reqDoneOpenCriteriaNotice(exempt, enforced, scanned int) string {
	return fmt.Sprintf(
		"req_done_open_criteria: %d Done REQ(s) with open criteria exempt as created before cutoff %s, %d enforced, %d Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)",
		exempt, reqDoneOpenCriteriaCutoff, enforced, scanned,
	)
}

// reqDoneOpenCriteriaAlwaysWarn generates the aggregated notice that is ALWAYS
// emitted via applyRuleWarnOnly when at least one Done REQ was scanned.
// Returns nil (empty) if no Done REQs were scanned — then there is nothing to report.
func reqDoneOpenCriteriaAlwaysWarn(violations []string, exempt, enforced, scanned int) []string {
	if scanned == 0 {
		return nil
	}
	return []string{reqDoneOpenCriteriaNotice(exempt, enforced, scanned)}
}

// reqDoneOpenCriteriaNoticeSubstr is a unique substring of the notice line, used
// by tests to distinguish the aggregate from per-item messages.
const reqDoneOpenCriteriaNoticeSubstr = "req_done_open_criteria:"
