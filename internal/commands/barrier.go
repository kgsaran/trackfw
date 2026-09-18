package commands

// barrier.go implements `trackfw barrier <roadmap> --wave <n> [--json]`, the deterministic,
// stack-agnostic wave-release barrier described in docs/cli-parity.md (`## trackfw barrier`).
// It never assumes a build tool, test runner or parity rule: every executable check either
// comes from the roadmap itself (gates) or from the in-process validator (validate).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/roadmapdoc"
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
		Short: "Deterministic wave-release barrier (wave_headings, mls_complete, acceptance_evidence, gates, validate)",
		Long: `trackfw barrier evaluates a single wave of a roadmap against five built-in,
stack-agnostic checks: the document has no malformed wave headings, every ML in the wave
is complete, every ML has met acceptance evidence, every gate command declared by the wave
exits 0, and 'trackfw validate' reports zero violations. It never invents a gate and never
assumes a build tool.`,
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

	// Reusa o resolvedor canônico do validator (validator.ResolveWIPDirs/ResolveDoneDirs, que
	// delega a resolveStateDirs → resolveAgentNamespaces) em vez de reimplementar a resolução de
	// namespace aqui — duplicar essa lógica foi apontado pelo ML-0A como um dos dois pontos de
	// divergência arquitetural do sweep (REQ-2026-08-29).
	wipDirs := validator.ResolveWIPDirs(cfg)
	doneDirs := validator.ResolveDoneDirs(cfg)

	for _, dir := range append(wipDirs, doneDirs...) {
		candidate := filepath.Join(dir, filename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("roadmap %q not found in wip/ nor done/ under %s", base, cfg.RoadmapDir)
}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap parsing — delegated to internal/roadmapdoc (ML-1A, REQ #392).
// All string-level rules are pinned by docs/cli-parity.md.
// barrier.go retains only the cobra-dependent wrappers and the usage error type.
// ────────────────────────────────────────────────────────────────────────────

// waveLabelRe is still needed here for the --wave flag validation
// in newBarrierCmd (which runs before parseWaves). The canonical copy lives in
// roadmapdoc; this is forwarded for the command-layer guard.
var waveLabelRe = roadmapdoc.WaveLabelRe

// criteriaHeaderRe and compareWaveLabels are forwarded from roadmapdoc so that
// tests in package commands can still reference them without importing roadmapdoc directly.
var criteriaHeaderRe = roadmapdoc.CriteriaHeaderRe

func compareWaveLabels(a, b string) int {
	return roadmapdoc.CompareWaveLabels(a, b)
}

// parseWaves, splitWaveLabel, fenceMask, splitRoadmapLines, parseMLs,
// mlStatusMarker, statusIsComplete, acceptanceEvaluate, parseGates are all
// delegated to roadmapdoc. Local shims preserve the original call sites and
// wrap roadmapdoc errors into barrierUsageError.

func splitRoadmapLines(data string) []string {
	return roadmapdoc.SplitRoadmapLines(data)
}

func fenceMask(lines []string) []bool {
	return roadmapdoc.FenceMask(lines)
}

type waveBlock = roadmapdoc.WaveBlock
type mlBlock = roadmapdoc.MLBlock

func parseWaves(lines []string) ([]waveBlock, []roadmapdoc.MalformedWave) {
	waves, malformed := roadmapdoc.ParseWaves(lines)
	return waves, malformed
}

func splitWaveLabel(label string) (int, string) {
	return roadmapdoc.SplitWaveLabel(label)
}

func parseMLs(lines []string, fenced []bool, waveStart, waveEnd int) []mlBlock {
	return roadmapdoc.ParseMLs(lines, fenced, waveStart, waveEnd)
}

func mlStatusMarker(lines []string, fenced []bool, ml mlBlock) (string, bool) {
	return roadmapdoc.MLStatusMarker(lines, fenced, ml)
}

func statusIsComplete(marker string) bool {
	return roadmapdoc.StatusIsComplete(marker)
}

func acceptanceEvaluate(lines []string, fenced []bool, ml mlBlock) (int, int, bool) {
	return roadmapdoc.AcceptanceEvaluate(lines, fenced, ml)
}

func parseGates(lines []string, waveStart, waveEnd int) ([]string, *barrierUsageError) {
	cmds, err := roadmapdoc.ParseGates(lines, waveStart, waveEnd)
	if err != nil {
		return nil, &barrierUsageError{msg: err.Error()}
	}
	return cmds, nil
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
// Posture (AC1): CLOSED by default. Gates execute only when the function can
// PROVE that the roadmap is present in refs/remotes/origin/main byte-for-byte.
// Absence of proof is absence of trust — every error path returns trusted:false
// with a named reason (AC2). There is exactly one trusted:true return, at the
// very end, after all five proofs succeed.
//
// AC3: the discriminant never parses git stderr. Steps 4 and 5 use
// "git rev-parse --verify" and "git cat-file -e" whose exit codes answer the
// questions directly. The fully-qualified ref refs/remotes/origin/main is used
// throughout so that a local branch or tag named "origin/main" cannot satisfy
// the trust anchor.
//
// Residual (declared in docs/cli-parity.md): Windows users with
// core.autocrlf=true may receive "content differs" for an otherwise-identical
// roadmap (LF vs CRLF). The check fails closed, so the residual is safe.
// roadmapTrustForGates determines whether the gates declared in a roadmap can
// be trusted for execution without --trust-local-gates.
//
// localContent is the byte slice already read by the caller (the same buffer
// used to parse gate commands). Comparing it here ensures the proof covers the
// exact bytes that will be executed — F1 invariant: what is proved = what executes.
// The only remaining route from unverified bytes to sh -c is --trust-local-gates,
// which requires explicit operator consent.
func roadmapTrustForGates(roadmapPath string, localContent []byte) gatesTrustVerdict {
	roadmapDir := filepath.Dir(roadmapPath)

	// Step 1: check if we are inside a git repository.
	// Two distinct failure reasons are separated here (F5):
	//   spawn failure  → git binary not found in PATH
	//   exit non-zero  → not inside a git repository
	revParseCmd := exec.Command("git", "rev-parse", "--git-dir")
	revParseCmd.Dir = roadmapDir
	if err := revParseCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// Spawn failure: git binary is not installed or not in PATH.
			return gatesTrustVerdict{
				trusted:    false,
				failureMsg: "gates not evaluated: git not found in PATH — install git to evaluate local gates",
			}
		}
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: not a git repository — pass --trust-local-gates to evaluate local gates",
		}
	}

	// Step 2: get the repository toplevel so we can compute a repo-relative path.
	topCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	topCmd.Dir = roadmapDir
	topOut, err := topCmd.Output()
	if err != nil {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: cannot resolve git repository root — pass --trust-local-gates to evaluate local gates",
		}
	}
	topLevel := strings.TrimSpace(string(topOut))

	// Step 3: compute path relative to the toplevel (git uses forward slashes).
	absRoadmap, err := filepath.Abs(roadmapPath)
	if err != nil {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: cannot compute relative path to roadmap — pass --trust-local-gates to evaluate local gates",
		}
	}
	relPath, err := filepath.Rel(topLevel, absRoadmap)
	if err != nil {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: cannot compute relative path to roadmap — pass --trust-local-gates to evaluate local gates",
		}
	}
	relPath = filepath.ToSlash(relPath)

	// Step 4: verify that refs/remotes/origin/main resolves to a commit (AC3 —
	// exit code only, no stderr parsing). Failure means: no remote named origin,
	// origin/main never fetched, or wrong default branch name. All are untrusted.
	verifyCmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
	verifyCmd.Dir = topLevel
	if err := verifyCmd.Run(); err != nil {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: origin/main ref not available — pass --trust-local-gates to evaluate local gates",
		}
	}

	// Step 5: check whether the roadmap path exists in refs/remotes/origin/main
	// (AC3 — "git cat-file -e" exits non-zero when the object does not exist).
	refPath := "refs/remotes/origin/main:" + relPath
	catFileCmd := exec.Command("git", "cat-file", "-e", refPath)
	catFileCmd.Dir = topLevel
	if err := catFileCmd.Run(); err != nil {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates",
		}
	}

	// Step 6: retrieve the file content from refs/remotes/origin/main.
	showCmd := exec.Command("git", "show", refPath)
	showCmd.Dir = topLevel
	mainContent, err := showCmd.Output()
	if err != nil {
		// The cat-file check already confirmed the object exists; this is an
		// unexpected failure (e.g. transient I/O). Still fail closed.
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: cannot read roadmap from origin/main — pass --trust-local-gates to evaluate local gates",
		}
	}

	// Step 7: compare content byte-for-byte.
	// localContent (the parameter) is the buffer the caller already read — it
	// is the same slice used to parse gate commands. No second read is performed
	// here; the proof covers exactly what executes (F1 invariant).
	//
	// Step 5 triple note (F5 — declared unseparable): git cat-file -e exits
	// non-zero for three distinct reasons: (a) roadmap absent from origin/main,
	// (b) relPath computed incorrectly (e.g. macOS symlink divergence, mitigated
	// by WORK_PHYS in check-barrier.sh), (c) unexpected I/O error. Separating
	// them without additional git invocations would expand the attack surface and
	// all three paths are equally fail-closed. Declared unseparable by design.
	if string(mainContent) != string(localContent) {
		return gatesTrustVerdict{
			trusted:    false,
			failureMsg: "gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates",
		}
	}

	// Proven: roadmap is present in refs/remotes/origin/main and byte-identical.
	// The buffer compared is the same one parsed for gate commands.
	return gatesTrustVerdict{trusted: true}
}

// shMissingMsg is the pinned failure string for a `gates` check that could not be
// evaluated because `sh` is not on $PATH (AC3, AC4). All three runtimes (Go, Node,
// Python) must emit this byte-for-byte — see docs/cli-parity.md
// "Pinned failure strings for not_evaluated".
const shMissingMsg = "gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates"

// runGateCommand executes one gate command from the repository root (the process's
// current working directory) via `sh -c`. `sh` is resolved through $PATH
// (exec.LookPath, the same as Go has always done) — NOT a fixed /bin/sh path.
//
// Returns the exit code and spawnFailed, which is true only when the process
// never started at all (e.g. `sh` missing from $PATH). This is distinct from the
// gate command itself failing inside a running `sh`: `sh -c 'nosuchtool'` returns
// exit 127 with spawnFailed=false — sh started and ran, then reported that its
// child command doesn't exist. 127 is a normal (if unusual) exit code, never a
// signal for "sh is missing" (measured in ML-0A).
func runGateCommand(command string) (exitCode int, spawnFailed bool) {
	c := exec.Command("sh", "-c", command)
	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), false
		}
		// Not an *exec.ExitError: the process never started (e.g. LookPath
		// failed to find "sh" in $PATH) — sh's own exit code was never observed.
		return 1, true
	}
	return 0, false
}

// evalGateCommands runs each gate command in order via runGateCommand. If `sh`
// cannot be spawned at all, the whole check becomes not_evaluated (AC3, AC4) —
// "could not measure" is distinct from "measured and failed" — and evaluation
// stops immediately: gates after the spawn failure were never observed, so they
// must not appear in evidence or failures.
func evalGateCommands(gateCommands []string) (status string, evidence []string, failures []string) {
	evidence = []string{}
	failures = []string{}
	for _, gcmd := range gateCommands {
		exitCode, spawnFailed := runGateCommand(gcmd)
		if spawnFailed {
			return "not_evaluated", []string{}, []string{shMissingMsg}
		}
		if exitCode == 0 {
			evidence = append(evidence, fmt.Sprintf("%s: exit 0", gcmd))
		} else {
			failures = append(failures, fmt.Sprintf("%s: exit %d", gcmd, exitCode))
		}
	}
	if len(failures) == 0 {
		return "passed", evidence, failures
	}
	return "blocked", evidence, failures
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
	lines := splitRoadmapLines(string(data))
	fenced := fenceMask(lines)

	waves, malformed := parseWaves(lines)
	// Report each malformed wave heading to stderr AND record in the wave_headings check
	// (ML-1E, REQ #392). The stderr warning is kept so that users without --json also see
	// the problem immediately; the check entry ensures the verdict is never "passed" while
	// any malformed heading exists (ADR-2026-07-29 decision 16, as emended: principle
	// "reprovar alto" preserved; remedy changed from "abort document" to "check that blocks").
	for _, mw := range malformed {
		fmt.Fprintf(cmd.ErrOrStderr(), "trackfw barrier: %s\n", mw.Error())
	}

	// ── check: wave_headings ─────────────────────────────────────────────────
	// Every heading that matched the broad wave detector but failed the strict label grammar
	// is a failure. An empty document (no malformed headings) passes.
	whCheck := barrierCheck{Name: "wave_headings", Evidence: []string{}, Failures: []string{}}
	if len(malformed) == 0 {
		whCheck.Status = "passed"
	} else {
		whCheck.Status = "blocked"
		for _, mw := range malformed {
			whCheck.Failures = append(whCheck.Failures,
				fmt.Sprintf("line %d: %q is not a valid wave label", mw.Line, mw.Token))
		}
	}

	var target *waveBlock
	for i := range waves {
		// Use CompareWaveLabels for equality so that "1b" and "1-b" resolve to the same
		// wave (ML-1D, REQ #392): SplitWaveLabel normalises both to (1, "b") before compare.
		if roadmapdoc.CompareWaveLabels(waves[i].Label, waveLabel) == 0 {
			target = &waves[i]
			break
		}
	}
	if target == nil {
		usageExit(cmd, "wave %s not found in roadmap %q", waveLabel, filepath.Base(roadmapPath))
		return
	}

	mls := parseMLs(lines, fenced, target.Start, target.End)

	// ── check: mls_complete ──────────────────────────────────────────────────
	mlsCheck := barrierCheck{Name: "mls_complete", Evidence: []string{}, Failures: []string{}}
	if len(mls) == 0 {
		mlsCheck.Status = "blocked"
		mlsCheck.Failures = append(mlsCheck.Failures, fmt.Sprintf("wave %s: no ML found", waveLabel))
	} else {
		ok := true
		for _, ml := range mls {
			marker, found := mlStatusMarker(lines, fenced, ml)
			if found && statusIsComplete(marker) {
				mlsCheck.Evidence = append(mlsCheck.Evidence, fmt.Sprintf("%s: ✅", ml.ID))
				continue
			}
			ok = false
			status := marker
			if !found {
				status = "missing"
			}
			mlsCheck.Failures = append(mlsCheck.Failures, fmt.Sprintf("%s: not complete (status: %s)", ml.ID, status))
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
		met, unmet, hasBlock := acceptanceEvaluate(lines, fenced, ml)
		switch {
		case !hasBlock:
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: no acceptance block", ml.ID))
		case unmet > 0:
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: %d unmet acceptance criteria", ml.ID, unmet))
		default:
			accCheck.Evidence = append(accCheck.Evidence, fmt.Sprintf("%s: %d criteria met", ml.ID, met))
		}
	}
	if accOK {
		accCheck.Status = "passed"
	} else {
		accCheck.Status = "blocked"
	}

	// ── check: gates ──────────────────────────────────────────────────────────
	gateCommands, gerr := parseGates(lines, target.Start, target.End)
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
	// Trust check (AC11, AC12): determine whether this roadmap's gates may be
	// executed from local content, or whether the roadmap is untrusted (PR vector).
	// --trust-local-gates bypasses the check (injected by the /trackfw:barrier
	// slash command for the WIP flow — AC12, AC15).
	if trustLocalGates {
		// Explicit consent: evaluate gates from local content.
		status, evidence, failures := evalGateCommands(gateCommands)
		gatesCheck.Status = status
		gatesCheck.Evidence = evidence
		gatesCheck.Failures = failures
	} else {
		verdict := roadmapTrustForGates(roadmapPath, data)
		if !verdict.trusted {
			// Roadmap is not trusted: do not execute gates (AC3, AC14).
			// Report as not_evaluated — distinct from passed and blocked (AC6).
			gatesCheck.Status = "not_evaluated"
			gatesCheck.Failures = append(gatesCheck.Failures, verdict.failureMsg)
		} else {
			// Trusted (fail-open): evaluate gates.
			status, evidence, failures := evalGateCommands(gateCommands)
			gatesCheck.Status = status
			gatesCheck.Evidence = evidence
			gatesCheck.Failures = failures
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

	checks := []barrierCheck{whCheck, mlsCheck, accCheck, gatesCheck, validateCheck}
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
