package commands

// barrier.go implements `trackfw barrier <roadmap> --wave <n> [--json]`, the deterministic,
// stack-agnostic wave-release barrier described in docs/cli-parity.md (`## trackfw barrier`).
// It never assumes a build tool, test runner or parity rule: every executable check either
// comes from the roadmap itself (gates) or from the in-process validator (validate).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

// ────────────────────────────────────────────────────────────────────────────
// JSON result document — field order and shape pinned by docs/cli-parity.md.
// ────────────────────────────────────────────────────────────────────────────

// barrierCheck is one evaluated check inside the result document.
// Commands uses a pointer so that omitempty only suppresses the field when nil
// (never present) — the gates check always sets a non-nil pointer, even to an
// empty slice, so "commands" is always emitted for it and never for the others.
type barrierCheck struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Commands *[]string `json:"commands,omitempty"`
	Evidence []string  `json:"evidence"`
	Failures []string  `json:"failures"`
}

// barrierResult is the root JSON document emitted by --json.
type barrierResult struct {
	Roadmap    string         `json:"roadmap"`
	Wave       string         `json:"wave"`
	Status     string         `json:"status"`
	StartedAt  string         `json:"started_at"`
	FinishedAt string         `json:"finished_at"`
	Checks     []barrierCheck `json:"checks"`
	Failures   []string       `json:"failures"`
}

// barrierUsageError signals a resolution/parsing error distinct from a
// blocked (but evaluated) barrier — it maps to exit code 2, never 1.
type barrierUsageError struct{ msg string }

func (e *barrierUsageError) Error() string { return e.msg }

// ────────────────────────────────────────────────────────────────────────────
// Command wiring
// ────────────────────────────────────────────────────────────────────────────

func newBarrierCmd() *cobra.Command {
	var waveStr string
	var jsonOut bool
	var trustLocalGates bool

	cmd := &cobra.Command{
		Use:   "barrier <roadmap> --wave <n>",
		Short: "Deterministic wave-release barrier (mls_complete, acceptance_evidence, gates, validate)",
		Long: `trackfw barrier evaluates a single wave of a roadmap against four built-in,
stack-agnostic checks: every ML in the wave is complete, every ML has met acceptance
evidence, every gate command declared by the wave exits 0, and 'trackfw validate' reports
zero violations. It never invents a gate and never assumes a build tool.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			if !cmd.Flags().Changed("wave") || strings.TrimSpace(waveStr) == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "trackfw barrier: --wave is required")
				os.Exit(2)
			}
			waveLabel := strings.TrimSpace(waveStr)
			if !waveLabelRe.MatchString(waveLabel) {
				fmt.Fprintf(cmd.ErrOrStderr(), "trackfw barrier: invalid --wave %q — not a valid wave label\n", waveStr)
				os.Exit(2)
			}
			// Integer part must be >= 0 (grammar: integer value constraint, not enforced by regex).
			// 0 is a valid wave label — the Wave 0 threat-model convention (docs/cli-parity.md
			// § "Wave label grammar"). The regex cannot reject negatives on its own (\d+ has no
			// sign), so this is still the only place enforcing the lower bound.
			waveInt, _ := splitWaveLabel(waveLabel)
			if waveInt < 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "trackfw barrier: invalid --wave %q — not a valid wave label\n", waveStr)
				os.Exit(2)
			}

			runBarrier(cmd, args[0], waveLabel, jsonOut, trustLocalGates)
			return nil
		},
	}

	cmd.Flags().StringVar(&waveStr, "wave", "", "Wave label to evaluate (required, grammar: <integer>[-<suffix>], e.g. 2 or 2-bis)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit the result document as JSON instead of the text report")
	cmd.Flags().BoolVar(&trustLocalGates, "trust-local-gates", false, "Trust the local roadmap content for gate execution without comparing to origin/main (used by the /trackfw:barrier slash command for WIP roadmaps)")
	return cmd
}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap resolution — basename with or without .md, wip/ then done/,
// supporting both flat and by_agent roadmap_namespacing layouts.
// ────────────────────────────────────────────────────────────────────────────

func resolveBarrierRoadmap(name string) (string, error) {
	cfg := config.Load()
	base := strings.TrimSuffix(filepath.Base(name), ".md")
	filename := base + ".md"

	var wipDirs, doneDirs []string
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, err := os.ReadDir(cfg.RoadmapDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
		sort.Strings(agents)
		for _, agent := range agents {
			wipDirs = append(wipDirs, filepath.Join(cfg.RoadmapDir, agent, "wip"))
			doneDirs = append(doneDirs, filepath.Join(cfg.RoadmapDir, agent, "done"))
		}
	} else {
		wipDirs = []string{filepath.Join(cfg.RoadmapDir, "wip")}
		doneDirs = []string{filepath.Join(cfg.RoadmapDir, "done")}
	}

	for _, dir := range append(wipDirs, doneDirs...) {
		candidate := filepath.Join(dir, filename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("roadmap %q not found in wip/ nor done/ under %s", base, cfg.RoadmapDir)
}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap parsing — string-level rules pinned by docs/cli-parity.md
// ("Roadmap parsing rules — string-level — no heuristics").
// ────────────────────────────────────────────────────────────────────────────

var (
	// waveHeadingRe detects any "## Wave <token> " heading, including malformed ones.
	// The captured token is validated separately by waveLabelRe before being stored.
	waveHeadingRe = regexp.MustCompile(`^## Wave (\S+) `)
	// waveLabelRe validates a wave label token in isolation against the grammar pinned in
	// docs/cli-parity.md ("Wave label grammar"): <integer>[-<suffix>] where suffix is [a-z0-9]+.
	// Equivalent to the capture group of the heading regex ^## Wave (\d+(?:-[a-z0-9]+)?) .
	waveLabelRe      = regexp.MustCompile(`^\d+(?:-[a-z0-9]+)?$`)
	mlHeadingRe      = regexp.MustCompile(`^### (ML-\S+)`)
	statusLineRe     = regexp.MustCompile(`^\*\*Status:\*\*(.*)$`)
	criteriaHeaderRe = regexp.MustCompile(`^\*\*Crit[eé]rios de aceite:\*\*`)
	unmetCriterionRe = regexp.MustCompile(`^- \[ \]`)
	criterionLineRe  = regexp.MustCompile(`^- \[.\]`)
	boldLineRe       = regexp.MustCompile(`^\*\*`)
	gatesHeaderRe    = regexp.MustCompile(`^\*\*Gates da wave:\*\*`)
)

// waveBlock delimits one "## Wave <label> ..." section: [start, end) line indices (0-based).
// label is the wave label string (e.g. "1", "2-bis") per the grammar in docs/cli-parity.md.
type waveBlock struct {
	label string
	start int
	end   int
}

// mlBlock delimits one "### ML-..." section within a wave: [start, end) line indices.
type mlBlock struct {
	id    string
	start int
	end   int
}

// parseWaves splits the roadmap into wave blocks (rule 1). Returns a *barrierUsageError
// (never a plain error) when a wave heading's label is outside the grammar (rule 6).
// A heading outside the grammar aborts the entire document — intentionally (ADR decision 16).
func parseWaves(lines []string) ([]waveBlock, *barrierUsageError) {
	var waves []waveBlock
	n := len(lines)
	for i := 0; i < n; i++ {
		m := waveHeadingRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		token := m[1]
		// Validate label against the grammar pinned in docs/cli-parity.md.
		// Using literal quotes (not %q) so the token is emitted verbatim across runtimes.
		if !waveLabelRe.MatchString(token) {
			return nil, &barrierUsageError{
				msg: fmt.Sprintf("malformed wave heading at line %d: \"%s\" is not a valid wave label", i+1, token),
			}
		}
		// Integer part must be >= 0 — 0 is a valid wave label (Wave 0 threat-model convention,
		// docs/cli-parity.md § "Wave label grammar"). Mirrors the flag-validation constraint above.
		intVal, _ := splitWaveLabel(token)
		if intVal < 0 {
			return nil, &barrierUsageError{
				msg: fmt.Sprintf("malformed wave heading at line %d: \"%s\" is not a valid wave label", i+1, token),
			}
		}
		end := n
		for j := i + 1; j < n; j++ {
			if strings.HasPrefix(lines[j], "## ") {
				end = j
				break
			}
		}
		waves = append(waves, waveBlock{label: token, start: i, end: end})
	}
	return waves, nil
}

// splitWaveLabel splits a valid wave label into its integer and optional suffix parts.
// For "2-bis" it returns (2, "bis"); for "3" it returns (3, "").
// The label must already be valid per waveLabelRe; behaviour on invalid input is undefined.
func splitWaveLabel(label string) (integer int, suffix string) {
	if idx := strings.Index(label, "-"); idx >= 0 {
		integer, _ = strconv.Atoi(label[:idx])
		suffix = label[idx+1:]
	} else {
		integer, _ = strconv.Atoi(label)
	}
	return
}

// compareWaveLabels returns -1, 0, or 1 comparing two wave labels per the ordering defined
// in docs/cli-parity.md (§ "Wave label grammar"):
//  1. Compare integer parts numerically.
//  2. On a tie, no-suffix precedes with-suffix.
//  3. On a tie between two suffixes, compare lexicographically.
//
// So "2" < "2-bis" < "2-hotfix" < "3". Used where waves must be listed or compared.
func compareWaveLabels(a, b string) int {
	aInt, aSuf := splitWaveLabel(a)
	bInt, bSuf := splitWaveLabel(b)
	if aInt != bInt {
		if aInt < bInt {
			return -1
		}
		return 1
	}
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

// parseMLs splits a wave block into ML blocks (rule 2).
func parseMLs(lines []string, waveStart, waveEnd int) []mlBlock {
	var mls []mlBlock
	for i := waveStart; i < waveEnd; i++ {
		m := mlHeadingRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		end := waveEnd
		for j := i + 1; j < waveEnd; j++ {
			if strings.HasPrefix(lines[j], "### ") || strings.HasPrefix(lines[j], "## ") {
				end = j
				break
			}
		}
		mls = append(mls, mlBlock{id: m[1], start: i, end: end})
	}
	return mls
}

// mlStatusMarker returns the trimmed remainder of the ML's "**Status:**" line, if any (rule 3).
func mlStatusMarker(lines []string, ml mlBlock) (marker string, found bool) {
	for i := ml.start; i < ml.end; i++ {
		if m := statusLineRe.FindStringSubmatch(lines[i]); m != nil {
			return strings.TrimSpace(m[1]), true
		}
	}
	return "", false
}

// acceptanceEvaluate implements rule 4. hasBlock is false both when the
// "**Critérios de aceite:**" header is absent and when it is present but the body between
// it and the next "**" line (or ML boundary) contains zero "- [...]" criterion lines —
// an empty block is not vacuously passed, per the contract.
func acceptanceEvaluate(lines []string, ml mlBlock) (met, unmet int, hasBlock bool) {
	headerLine := -1
	for i := ml.start; i < ml.end; i++ {
		if criteriaHeaderRe.MatchString(lines[i]) {
			headerLine = i
			break
		}
	}
	if headerLine < 0 {
		return 0, 0, false
	}
	blockEnd := ml.end
	for j := headerLine + 1; j < ml.end; j++ {
		if boldLineRe.MatchString(lines[j]) {
			blockEnd = j
			break
		}
	}

	total := 0
	unmetCount := 0
	for i := headerLine + 1; i < blockEnd; i++ {
		line := lines[i]
		if unmetCriterionRe.MatchString(line) {
			total++
			unmetCount++
			continue
		}
		if criterionLineRe.MatchString(line) {
			total++
		}
	}
	if total == 0 {
		return 0, 0, false
	}
	return total - unmetCount, unmetCount, true
}

// parseGates implements rule 5. A wave with no "**Gates da wave:**" block returns an
// empty, non-nil slice — zero gates is legal and the barrier never invents one.
func parseGates(lines []string, waveStart, waveEnd int) ([]string, *barrierUsageError) {
	for i := waveStart; i < waveEnd; i++ {
		if !gatesHeaderRe.MatchString(lines[i]) {
			continue
		}
		j := i + 1
		for j < waveEnd && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= waveEnd || strings.TrimSpace(lines[j]) != "```bash" {
			return nil, &barrierUsageError{
				msg: fmt.Sprintf("malformed gates block at line %d: expected a ```bash fence immediately after '**Gates da wave:**'", i+1),
			}
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
			return nil, &barrierUsageError{
				msg: fmt.Sprintf("unterminated gates fence starting at line %d", fenceStart+1),
			}
		}
		if cmds == nil {
			cmds = []string{}
		}
		return cmds, nil
	}
	return []string{}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap trust check (AC11, AC12 — docs/cli-parity.md § Trust and --trust-local-gates)
// ────────────────────────────────────────────────────────────────────────────

// gatesTrustVerdict is returned by roadmapTrustForGates.
type gatesTrustVerdict struct {
	// trusted is true when gates may be executed from the local roadmap content.
	trusted bool
	// failureMsg is the pinned message to record in the gates check failures
	// when trusted is false. It must be byte-identical across all three runtimes.
	failureMsg string
}

// roadmapTrustForGates determines whether the gates declared in a roadmap can
// be trusted for execution without --trust-local-gates.
//
// Decision (AC4, AC11): the discriminant is git — a roadmap whose content
// differs from origin/main, or that is absent from origin/main, is untrusted.
//
// Fail-open cases (trust granted, residual declared in docs/cli-parity.md):
//   - roadmapPath is not inside a git repository
//   - origin/main reference is not resolvable (no remote, not fetched)
//   - any git invocation fails for reasons other than "path absent from origin/main"
//
// These fail-open cases preserve the check-barrier.sh fixtures (temp dirs, no
// git repo) and fresh clones without a fetched remote, without compromising the
// PR-review protection. The residuals are named in docs/cli-parity.md.
func roadmapTrustForGates(roadmapPath string) gatesTrustVerdict {
	roadmapDir := filepath.Dir(roadmapPath)

	// Step 1: check if we are inside a git repository.
	revParseCmd := exec.Command("git", "rev-parse", "--git-dir")
	revParseCmd.Dir = roadmapDir
	if err := revParseCmd.Run(); err != nil {
		// Not a git repo → fail-open.
		return gatesTrustVerdict{trusted: true}
	}

	// Step 2: get the repository toplevel so we can compute a repo-relative path.
	topCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	topCmd.Dir = roadmapDir
	topOut, err := topCmd.Output()
	if err != nil {
		return gatesTrustVerdict{trusted: true}
	}
	topLevel := strings.TrimSpace(string(topOut))

	// Step 3: compute path relative to the toplevel (git uses forward slashes).
	absRoadmap, err := filepath.Abs(roadmapPath)
	if err != nil {
		return gatesTrustVerdict{trusted: true}
	}
	relPath, err := filepath.Rel(topLevel, absRoadmap)
	if err != nil {
		return gatesTrustVerdict{trusted: true}
	}
	relPath = filepath.ToSlash(relPath)

	// Step 4: retrieve the file at origin/main.
	showCmd := exec.Command("git", "show", "origin/main:"+relPath)
	showCmd.Dir = topLevel
	mainContent, err := showCmd.Output()
	if err != nil {
		// If the path specifically does not exist in origin/main, it is a
		// PR-added file → untrusted.
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "does not exist in") ||
				strings.Contains(stderr, "exists on disk, but not in") {
				return gatesTrustVerdict{
					trusted:    false,
					failureMsg: "gates not evaluated: roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates",
				}
			}
		}
		// Any other failure (origin not configured, ref not fetched) → fail-open.
		return gatesTrustVerdict{trusted: true}
	}

	// Step 5: compare content byte-for-byte.
	localContent, err := os.ReadFile(roadmapPath)
	if err != nil {
		return gatesTrustVerdict{trusted: true}
	}
	if string(mainContent) != string(localContent) {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates",
		}
	}
	return gatesTrustVerdict{trusted: true}
}

// runGateCommand executes one gate command from the repository root (the process's
// current working directory) via the shell, returning its exit code.
func runGateCommand(command string) int {
	c := exec.Command("sh", "-c", command)
	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// ────────────────────────────────────────────────────────────────────────────
// Evaluation
// ────────────────────────────────────────────────────────────────────────────

// usageExit prints msg naming the unresolved entity to stderr and exits 2.
// Exit code 2 is distinct from exit 1 ("blocked"): a barrier that could not be
// evaluated is not the same as one that evaluated to a failure.
func usageExit(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(cmd.ErrOrStderr(), "trackfw barrier: "+format+"\n", args...)
	os.Exit(2)
}

// runBarrier evaluates <roadmap> --wave <label> and prints the report (text or JSON),
// exiting 0 (passed), 1 (blocked) or 2 (usage/resolution error, handled via usageExit).
func runBarrier(cmd *cobra.Command, roadmapArg string, waveLabel string, jsonOut bool, trustLocalGates bool) {
	startedAt := time.Now().UTC()

	roadmapPath, err := resolveBarrierRoadmap(roadmapArg)
	if err != nil {
		usageExit(cmd, "%s", err.Error())
		return
	}

	data, err := os.ReadFile(roadmapPath)
	if err != nil {
		usageExit(cmd, "could not read roadmap %q: %s", roadmapPath, err.Error())
		return
	}
	lines := strings.Split(string(data), "\n")

	waves, uerr := parseWaves(lines)
	if uerr != nil {
		usageExit(cmd, "%s", uerr.Error())
		return
	}

	var target *waveBlock
	for i := range waves {
		if waves[i].label == waveLabel {
			target = &waves[i]
			break
		}
	}
	if target == nil {
		usageExit(cmd, "wave %s not found in roadmap %q", waveLabel, filepath.Base(roadmapPath))
		return
	}

	mls := parseMLs(lines, target.start, target.end)

	// ── check: mls_complete ──────────────────────────────────────────────────
	mlsCheck := barrierCheck{Name: "mls_complete", Evidence: []string{}, Failures: []string{}}
	if len(mls) == 0 {
		mlsCheck.Status = "blocked"
		mlsCheck.Failures = append(mlsCheck.Failures, fmt.Sprintf("wave %s: no ML found", waveLabel))
	} else {
		ok := true
		for _, ml := range mls {
			marker, found := mlStatusMarker(lines, ml)
			if found && strings.Contains(marker, "✅") {
				mlsCheck.Evidence = append(mlsCheck.Evidence, fmt.Sprintf("%s: ✅", ml.id))
				continue
			}
			ok = false
			status := marker
			if !found {
				status = "missing"
			}
			mlsCheck.Failures = append(mlsCheck.Failures, fmt.Sprintf("%s: not complete (status: %s)", ml.id, status))
		}
		if ok {
			mlsCheck.Status = "passed"
		} else {
			mlsCheck.Status = "blocked"
		}
	}

	// ── check: acceptance_evidence ───────────────────────────────────────────
	accCheck := barrierCheck{Name: "acceptance_evidence", Evidence: []string{}, Failures: []string{}}
	accOK := len(mls) > 0
	for _, ml := range mls {
		met, unmet, hasBlock := acceptanceEvaluate(lines, ml)
		switch {
		case !hasBlock:
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: no acceptance block", ml.id))
		case unmet > 0:
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: %d unmet acceptance criteria", ml.id, unmet))
		default:
			accCheck.Evidence = append(accCheck.Evidence, fmt.Sprintf("%s: %d criteria met", ml.id, met))
		}
	}
	if accOK {
		accCheck.Status = "passed"
	} else {
		accCheck.Status = "blocked"
	}

	// ── check: gates ──────────────────────────────────────────────────────────
	gateCommands, gerr := parseGates(lines, target.start, target.end)
	if gerr != nil {
		usageExit(cmd, "%s", gerr.Error())
		return
	}
	gatesCmds := gateCommands
	gatesCheck := barrierCheck{
		Name:     "gates",
		Evidence: []string{},
		Failures: []string{},
		Commands: &gatesCmds,
	}
	gatesOK := true

	// Trust check (AC11, AC12): determine whether this roadmap's gates may be
	// executed from local content, or whether the roadmap is untrusted (PR vector).
	// --trust-local-gates bypasses the check (injected by the /trackfw:barrier
	// slash command for the WIP flow — AC12, AC15).
	if trustLocalGates {
		// Explicit consent: evaluate gates from local content.
		for _, gcmd := range gateCommands {
			exitCode := runGateCommand(gcmd)
			if exitCode == 0 {
				gatesCheck.Evidence = append(gatesCheck.Evidence, fmt.Sprintf("%s: exit 0", gcmd))
			} else {
				gatesOK = false
				gatesCheck.Failures = append(gatesCheck.Failures, fmt.Sprintf("%s: exit %d", gcmd, exitCode))
			}
		}
		if gatesOK {
			gatesCheck.Status = "passed"
		} else {
			gatesCheck.Status = "blocked"
		}
	} else {
		verdict := roadmapTrustForGates(roadmapPath)
		if !verdict.trusted {
			// Roadmap is not trusted: do not execute gates (AC3, AC14).
			// Report as not_evaluated — distinct from passed and blocked (AC6).
			gatesCheck.Status = "not_evaluated"
			gatesCheck.Failures = append(gatesCheck.Failures, verdict.failureMsg)
			gatesOK = false
		} else {
			// Trusted (fail-open): evaluate gates.
			for _, gcmd := range gateCommands {
				exitCode := runGateCommand(gcmd)
				if exitCode == 0 {
					gatesCheck.Evidence = append(gatesCheck.Evidence, fmt.Sprintf("%s: exit 0", gcmd))
				} else {
					gatesOK = false
					gatesCheck.Failures = append(gatesCheck.Failures, fmt.Sprintf("%s: exit %d", gcmd, exitCode))
				}
			}
			if gatesOK {
				gatesCheck.Status = "passed"
			} else {
				gatesCheck.Status = "blocked"
			}
		}
	}

	// ── check: validate ──────────────────────────────────────────────────────
	violations, warnings, verr := validator.ValidateTagged()
	validateCheck := barrierCheck{Name: "validate", Evidence: []string{}, Failures: []string{}}
	if verr != nil {
		validateCheck.Status = "blocked"
		validateCheck.Failures = append(validateCheck.Failures, verr.Error())
	} else {
		summary := fmt.Sprintf("%d violations, %d warnings", len(violations), len(warnings))
		if len(violations) == 0 {
			validateCheck.Status = "passed"
			validateCheck.Evidence = append(validateCheck.Evidence, summary)
		} else {
			validateCheck.Status = "blocked"
			validateCheck.Failures = append(validateCheck.Failures, summary)
		}
	}

	checks := []barrierCheck{mlsCheck, accCheck, gatesCheck, validateCheck}
	overallStatus := "passed"
	failures := []string{}
	for _, c := range checks {
		if c.Status != "passed" {
			overallStatus = "blocked"
		}
		for _, f := range c.Failures {
			failures = append(failures, fmt.Sprintf("%s: %s", c.Name, f))
		}
	}

	finishedAt := time.Now().UTC()
	result := barrierResult{
		Roadmap:    filepath.Base(roadmapPath),
		Wave:       waveLabel,
		Status:     overallStatus,
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Checks:     checks,
		Failures:   failures,
	}

	if jsonOut {
		out, _ := json.Marshal(result)
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
	} else {
		printBarrierText(cmd, result)
	}

	if overallStatus == "blocked" {
		os.Exit(1)
	}
}

// printBarrierText renders a human-readable report of the barrier result.
func printBarrierText(cmd *cobra.Command, result barrierResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "trackfw barrier — %s — wave %s\n", result.Roadmap, result.Wave)
	for _, c := range result.Checks {
		symbol := "✓"
		if c.Status != "passed" {
			symbol = "✗"
		}
		fmt.Fprintf(out, "%s %s: %s\n", symbol, c.Name, c.Status)
		for _, f := range c.Failures {
			fmt.Fprintf(out, "    - %s\n", f)
		}
	}
	fmt.Fprintf(out, "\nresult: %s\n", result.Status)
}
