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
// Upstream inheritance decomposition (issue #542, REQ-2026-10-08, ML-1A):
// When the repository has a remote "upstream" pointing to a different repository,
// the aggregated notice reports how many of the exempt REQs have the same basename
// as REQs committed in upstream/<branch> — making inherited debt visible to the
// fork's owner without changing any enforcement decision.
//
// How open criteria in a REQ are counted:
//  1. Find the AC section heading (## Acceptance Criteria or ## Critérios de aceite).
//  2. Find the end boundary (next ## heading or EOF).
//  3. Build a synthetic slice with "**Acceptance criteria:**" prepended, so
//     roadmapdoc.AcceptanceEvaluateFull can find the header via CriteriaHeaderRe.
//  4. Call AcceptanceEvaluateFull(sub, subFenced, MLBlock{Start:0, End:len(sub)}).
//     detail.Unmet is the count of open criteria (lapsed ones are NOT open).
//
// This reuses the HTML-comment mask and fence logic inside AcceptanceEvaluateFull
// without adding new code to the roadmapdoc package.

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
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
// Upstream inheritance detection (issue #542, REQ-2026-10-08)
// ---------------------------------------------------------------------------
//
// All git calls go through gitCommand() (validator_git_exec.go), never via
// exec.Command("git", ...) directly — see that file for the rationale.
//
// Algorithm (5 local git calls, O(1) per validate invocation):
//  1. git config remote.upstream.url  → absent: no parenthetical (AC3)
//  2. git config remote.origin.url    → equal: no parenthetical (T1 guard)
//  3. For ref in [refs/remotes/upstream/main, refs/remotes/upstream/master]:
//     a. rev-parse --verify --quiet <ref>^{commit}  → resolves: proceed
//     b. show <ref>:trackfw.yaml  → parse req_dir (fallback "docs/req")
//     c. ls-tree -r -z --name-only <ref> -- <req_dir>  → upstream basenames
//     d. count intersection with local exempt basenames; return parenthetical
//  4. Try refs/remotes/upstream/HEAD as last fallback.
//  5. Nothing resolved → "(upstream tried main, master: ref unresolvable)"
//
// K counts only exempt REQs that are inherited; post-cutoff (enforced) REQs
// with shared basenames are NOT counted in K (§5.5 of Wave 0 threat model).

// upstreamGitConfigGet reads a git config key in projectRoot.
// Returns ("", error) when the key is absent or git exits non-zero.
func upstreamGitConfigGet(projectRoot, key string) (string, error) {
	out, err := gitCommand(projectRoot, "config", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// upstreamRefExistsAsCommit returns true when <ref>^{commit} resolves without error.
func upstreamRefExistsAsCommit(projectRoot, ref string) bool {
	_, err := gitCommand(projectRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Output()
	return err == nil
}

// upstreamSymrefShort tries to dereference a symbolic ref and return its short name
// (e.g. "upstream/main"). Falls back to shortening the symbolic ref itself on error.
func upstreamSymrefShort(projectRoot, symref string) string {
	out, err := gitCommand(projectRoot, "symbolic-ref", "--short", symref).Output()
	if err == nil {
		if s := strings.TrimRight(string(out), "\n"); s != "" {
			return s
		}
	}
	// Fallback: strip "refs/remotes/" prefix if present.
	const pfx = "refs/remotes/"
	if strings.HasPrefix(symref, pfx) {
		return symref[len(pfx):]
	}
	return symref
}

// upstreamReqBasenames returns the set of .md basenames committed under the
// upstream's req_dir in the given ref. It reads req_dir from the upstream's
// trackfw.yaml (fallback "docs/req" when the file is absent or unparseable).
func upstreamReqBasenames(projectRoot, ref string) map[string]struct{} {
	result := make(map[string]struct{})

	// Read req_dir from upstream's trackfw.yaml.
	reqDir := "docs/req"
	showOut, err := gitCommand(projectRoot, "show", ref+":trackfw.yaml").Output()
	if err == nil {
		if parsed, _, _ := config.ParseDirsFromContent(string(showOut)); parsed != "" {
			reqDir = parsed
		}
	}

	// List all files under req_dir in the upstream ref.
	// --literal-pathspecs guards against pathspec-magic in directory names.
	lsOut, err := gitCommand(projectRoot, "--literal-pathspecs", "ls-tree",
		"-r", "-z", "--name-only", ref, "--", reqDir).Output()
	if err != nil {
		return result
	}

	for _, entry := range bytes.Split(lsOut, []byte{0}) {
		name := string(entry)
		if name == "" {
			continue
		}
		if filepath.Ext(name) == ".md" {
			result[filepath.Base(name)] = struct{}{}
		}
	}
	return result
}

// upstreamInheritedInfo computes the upstream parenthetical and the count K of
// exempt REQs whose basename appears in the upstream repository.
//
// Returns ("", 0) when the repository has no upstream remote or when upstream
// equals origin (T1 guard). Returns a non-empty parenthetical with the resolved
// ref name (e.g. "(3 inherited from upstream/main)") on success.
//
// Uses "." as projectRoot (the current working directory, anchored by gitCommand).
func upstreamInheritedInfo(exemptBasenames map[string]struct{}) (parenthetical string, k int) {
	const projectRoot = "."

	// Step 1: check for remote.upstream.url — absent means plain clone (AC3).
	upstreamURL, err := upstreamGitConfigGet(projectRoot, "remote.upstream.url")
	if err != nil || upstreamURL == "" {
		return "", 0
	}

	// Step 2: T1 guard — upstream pointing at itself.
	originURL, err := upstreamGitConfigGet(projectRoot, "remote.origin.url")
	if err == nil && upstreamURL == originURL {
		return "", 0
	}

	// Step 3: try main, then master.
	for _, branch := range []string{"main", "master"} {
		refFull := "refs/remotes/upstream/" + branch
		if !upstreamRefExistsAsCommit(projectRoot, refFull) {
			continue
		}
		upBasenames := upstreamReqBasenames(projectRoot, refFull)
		k = countBaselineIntersection(upBasenames, exemptBasenames)
		shortRef := "upstream/" + branch
		return fmt.Sprintf("(%d inherited from %s)", k, shortRef), k
	}

	// Step 4: fall back to refs/remotes/upstream/HEAD.
	headRef := "refs/remotes/upstream/HEAD"
	if upstreamRefExistsAsCommit(projectRoot, headRef) {
		shortRef := upstreamSymrefShort(projectRoot, headRef)
		upBasenames := upstreamReqBasenames(projectRoot, headRef)
		k = countBaselineIntersection(upBasenames, exemptBasenames)
		return fmt.Sprintf("(%d inherited from %s)", k, shortRef), k
	}

	// No resolvable ref found.
	return "(upstream tried main, master: ref unresolvable)", 0
}

// countBaselineIntersection counts how many keys in `b` also appear in `a`.
func countBaselineIntersection(a, b map[string]struct{}) int {
	n := 0
	for key := range b {
		if _, ok := a[key]; ok {
			n++
		}
	}
	return n
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
//   - parenthetical: upstream inheritance annotation, e.g. "(3 inherited from upstream/main)".
//     Empty when the repository has no upstream remote (AC3) or upstream == origin (T1).
//   - err: configuration or filesystem error.
//
// issue #542 / REQ-2026-10-08: parenthetical is computed once per invocation via
// upstreamInheritedInfo, which performs 5 local git calls (~37ms total).
func validateREQDoneOpenCriteria() (violations []string, exemptCount, enforcedCount, scannedCount int, parenthetical string, err error) {
	cfg := config.Load()
	files, filesErr := resolveREQFiles(cfg)
	if filesErr != nil {
		return nil, 0, 0, 0, "", filesErr
	}

	// exemptBasenames collects the basenames of exempt REQs so that
	// upstreamInheritedInfo can intersect them with upstream basenames.
	exemptBasenames := make(map[string]struct{})

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
			exemptBasenames[filepath.Base(path)] = struct{}{}
			continue
		}

		violations = append(violations, fmt.Sprintf(
			"REQ %q has Status: Done but contains open acceptance criteria (- [ ] without Caducou:); mark as met or add a Caducou: justification (req_done_open_criteria, D4 ADR-2026-10-04)",
			filepath.Base(path),
		))
		enforcedCount++
	}

	parenthetical, _ = upstreamInheritedInfo(exemptBasenames)
	return violations, exemptCount, enforcedCount, scannedCount, parenthetical, nil
}

// reqDoneOpenCriteriaNotice is the aggregated single-line notice that makes
// the exemption VISIBLE. Emitted unconditionally after any Done REQ scan
// (even with 0 exempt), so the denominator (scanned) prevents vacuous silence.
//
// parenthetical is the upstream inheritance annotation, e.g. "(3 inherited from
// upstream/main)". It is inserted after the cutoff date and before the comma.
// An empty parenthetical produces the byte-identical pre-upstream format (AC3).
// No double-quotes appear in the parenthetical (extractFile constraint in result.go).
func reqDoneOpenCriteriaNotice(exempt, enforced, scanned int, parenthetical string) string {
	exemptPart := fmt.Sprintf(
		"%d Done REQ(s) with open criteria exempt as created before cutoff %s",
		exempt, reqDoneOpenCriteriaCutoff,
	)
	if parenthetical != "" {
		exemptPart += " " + parenthetical
	}
	return fmt.Sprintf(
		"req_done_open_criteria: %s, %d enforced, %d Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)",
		exemptPart, enforced, scanned,
	)
}

// reqDoneOpenCriteriaAlwaysWarn generates the aggregated notice that is ALWAYS
// emitted via applyRuleWarnOnly when at least one Done REQ was scanned.
// Returns nil (empty) if no Done REQs were scanned — then there is nothing to report.
func reqDoneOpenCriteriaAlwaysWarn(violations []string, exempt, enforced, scanned int, parenthetical string) []string {
	if scanned == 0 {
		return nil
	}
	return []string{reqDoneOpenCriteriaNotice(exempt, enforced, scanned, parenthetical)}
}

// reqDoneOpenCriteriaNoticeSubstr is a unique substring of the notice line, used
// by tests to distinguish the aggregate from per-item messages.
const reqDoneOpenCriteriaNoticeSubstr = "req_done_open_criteria:"
