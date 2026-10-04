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
//
// Lapsed is a new field (D3, REQ #514 / ML-1A): informational messages for
// acceptance criteria marked Caducou:. Present only on acceptance_evidence when
// at least one lapsed criterion exists; omitempty keeps baseline JSON unchanged.
type barrierCheck struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Commands *[]string `json:"commands,omitempty"`
	Evidence []string  `json:"evidence"`
	Failures []string  `json:"failures"`
	Lapsed   []string  `json:"lapsed,omitempty"`
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

func parseWaves(lines []string, fenced []bool) ([]waveBlock, []roadmapdoc.MalformedWave) {
	waves, malformed := roadmapdoc.ParseWaves(lines, fenced)
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

// acceptanceEvaluateDetail wraps roadmapdoc.AcceptanceEvaluateFull for use inside
// runBarrier. Returns the three-class breakdown (D3, REQ #514 / ML-1A).
func acceptanceEvaluateDetail(lines []string, fenced []bool, ml mlBlock) roadmapdoc.AcceptanceDetail {
	return roadmapdoc.AcceptanceEvaluateFull(lines, fenced, ml)
}

func parseGates(lines []string, waveStart, waveEnd int) ([]string, *barrierUsageError) {
	cmds, err := roadmapdoc.ParseGates(lines, waveStart, waveEnd)
	if err != nil {
		return nil, &barrierUsageError{msg: err.Error()}
	}
	return cmds, nil
}

// parseGatesWithLines wraps roadmapdoc.ParseGatesLines for use inside runBarrier.
// It preserves line-number information needed by the fragment check (ML-2A).
func parseGatesWithLines(lines []string, waveStart, waveEnd int) ([]roadmapdoc.GateCmd, *barrierUsageError) {
	gcmds, err := roadmapdoc.ParseGatesLines(lines, waveStart, waveEnd)
	if err != nil {
		return nil, &barrierUsageError{msg: err.Error()}
	}
	return gcmds, nil
}

// hasOddTrailingBackslashes reports whether text ends with an odd number of
// consecutive backslashes. A trailing odd-\ is a shell line-continuation that
// makes the line incomplete when run in a separate sh -c invocation (rule 5).
// sh -n does not detect this case (measured FN — vault/notes/sh-n-misses-…).
func hasOddTrailingBackslashes(text string) bool {
	count := 0
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '\\' {
			count++
		} else {
			break
		}
	}
	return count%2 == 1
}

// checkGateFragments runs the odd-\ rule and sh -n for every gate before any
// gate is executed. It is called only inside trusted paths (after the trust
// check) — untrusted roadmaps never reach this function.
//
// Returns:
//   - "", nil      — every gate is syntactically complete; safe to execute
//   - "blocked", failures — one or more fragments detected; failures has one
//     entry per bad gate in the format pinned by docs/cli-parity.md (rule 5)
//   - "not_evaluated", {shMissingMsg} — sh could not be spawned at all
//
// sh is resolved through $PATH (same as runGateCommand). c.Env is nil so the
// child inherits the process environment — sh -n is a pure syntax check and
// never executes any code, so TRACKFW_BARRIER_STACK propagation is unnecessary.
//
// Transport: the gate text is delivered to sh via stdin (c.Stdin), NOT via
// argv ("-c", text). On Windows, Go's exec.Command applies EscapeArg to every
// argument; MSYS reparsing then converts an unquoted `"` to `\`, so
// `esperado="scaffold.go` arrives as `esperado=\scaffold.go` — a valid
// assignment that exits 0 and is silently approved. Stdin is opaque to
// EscapeArg and reaches sh byte-identical on every OS.
// Parity measured on macOS over 12 vectors: `sh -n` with stdin and with argv
// agree on all 12 (the argv mangling only manifests on Windows).
// See vault/notes/windows-argv-troca-aspa-por-contrabarra-sem-espaco-2026-10-01.md.
func checkGateFragments(gcmds []roadmapdoc.GateCmd) (status string, failures []string) {
	failures = []string{}
	for _, gc := range gcmds {
		// Guard: multi-line gate text — invariant violation.
		// The stdin transport is safe because ParseGatesLines cuts by line and
		// TrimSpace removes \r — each gc.Text is one line in production. With
		// more than one line, a gate that reads stdin (`read x`) would consume
		// the next script line as its input (measured by Lourival, PR #495).
		// Refuse without invoking sh; exit code 2 signals a usage error.
		if strings.ContainsAny(gc.Text, "\n\r") {
			failures = append(failures, fmt.Sprintf(
				"line %d: gate text spans multiple lines — the transport reads one line per gate (rule 5)",
				gc.Line))
			continue
		}

		// Fast path: odd trailing backslash — sh -n passes it (FN, measured),
		// but it is always a fragment per rule 5.
		if hasOddTrailingBackslashes(gc.Text) {
			failures = append(failures, fmt.Sprintf(
				"line %d: incomplete command — each line of the gates block runs as a separate sh -c (rule 5): %s",
				gc.Line, gc.Text))
			continue
		}
		// Deliver the gate text via stdin, not argv — see transport comment above.
		c := exec.Command("sh", "-n")
		c.Stdin = strings.NewReader(gc.Text)
		if err := c.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				// sh could not be spawned: same not_evaluated signal as evalGateCommands.
				return "not_evaluated", []string{shMissingMsg}
			}
			// sh ran and reported a syntax error.
			failures = append(failures, fmt.Sprintf(
				"line %d: incomplete command — each line of the gates block runs as a separate sh -c (rule 5): %s",
				gc.Line, gc.Text))
		}
	}
	if len(failures) > 0 {
		return "blocked", failures
	}
	return "", nil
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

// ────────────────────────────────────────────────────────────────────────────
// Reentrance detection — ML-1A (#485)
//
// TRACKFW_BARRIER_STACK is a JSON array of {"roadmap":"<abs-path>","wave":"<label>"}
// objects. It is injected into the child processes that execute gates, so that a
// nested barrier invocation can detect whether it is being called from within an
// enclosing evaluation of the same (roadmap, wave) pair.
//
// The roadmap key is the EvalSymlinks-resolved absolute path so that a symlink
// pointing at the same file is recognised as the same entry. The wave key is the
// normalised form "<int><suffix>" (no hyphen), so "1b" and "1-b" map to the same
// key. Both normalisations follow the same conventions used by CompareWaveLabels.
//
// Residual (declared in docs/cli-parity.md and the Wave-0 threat model):
// env-clearing environments (env -i, sudo -i, docker run without -e, ssh) discard
// TRACKFW_BARRIER_STACK and bypass both the per-key check and the depth backstop.
// ────────────────────────────────────────────────────────────────────────────

// barrierStackVar is the environment variable name that carries the reentry stack.
const barrierStackVar = "TRACKFW_BARRIER_STACK"

// barrierMaxDepth is the maximum allowed nesting depth of barrier invocations.
// When the stack already has barrierMaxDepth entries, the next call is refused
// even if none of those entries match the current (roadmap, wave) pair — this
// is the backstop against indirect reentrance (e.g. via a chain of different
// roadmaps that forms a cycle). Documented in docs/cli-parity.md (ML-2C, #485).
const barrierMaxDepth = 4

// barrierStackEntry is one element in the reentry stack.
type barrierStackEntry struct {
	Roadmap string `json:"roadmap"`
	Wave    string `json:"wave"`
}

// barrierReentryKey returns the canonical stack entry for the given roadmap path
// and wave label. Uses filepath.Abs before EvalSymlinks so that a relative
// roadmapPath (e.g. from a test that sets cmd.Dir) resolves to the same key as
// an absolute path for the same file.
func barrierReentryKey(roadmapPath, waveLabel string) barrierStackEntry {
	abs, err := filepath.Abs(roadmapPath)
	if err != nil {
		abs = roadmapPath
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// EvalSymlinks may fail if a path component does not exist on disk yet
		// (e.g. inside a TempDir that was just created). Fall back to the abs path.
		resolved = abs
	}
	n, suf := roadmapdoc.SplitWaveLabel(waveLabel)
	return barrierStackEntry{
		Roadmap: resolved,
		Wave:    fmt.Sprintf("%d%s", n, suf),
	}
}

// sameRoadmapFile reports whether paths a and b refer to the same underlying file.
// It is used for roadmap identity in the reentry stack comparison (F2 fix, ML-2C #485).
//
// String equality is the fast path: if the two EvalSymlinks-resolved paths are
// identical, the files are the same without any syscall. When they differ, we fall
// back to os.Stat + os.SameFile which compares inode+device on Unix and
// volume+file-index on Windows. This catches:
//   - hardlinks (two names, one inode)
//   - APFS/HFS+ case-insensitive aliases (EvalSymlinks preserves the provided case,
//     not the canonical on-disk case; os.SameFile uses the inode, not the path string)
//
// If os.Stat fails for either path (e.g. the file was deleted since the key was
// built), sameRoadmapFile returns false and the caller falls through to the string
// comparison already embedded in the fast path — meaning the entry is not treated
// as a match, which is safe (worst case: one extra level before backstop triggers).
func sameRoadmapFile(a, b string) bool {
	if a == b {
		return true // fast path: identical strings
	}
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

// buildChildEnv constructs the environment to pass to gate child processes:
// os.Environ() minus any existing TRACKFW_BARRIER_STACK entries (case-insensitive
// on the variable name, for Windows compatibility), plus the updated stack that
// includes the current (roadmap, wave) entry.
//
// The current barrier process does NOT call os.Setenv — the stack is only visible
// to child processes that execute gates.
func buildChildEnv(stack []barrierStackEntry) []string {
	raw, err := json.Marshal(stack)
	if err != nil {
		// json.Marshal only fails on unmarshalable types; barrierStackEntry is
		// plain strings, so this is unreachable in practice.
		raw = []byte("[]")
	}
	newEntry := barrierStackVar + "=" + string(raw)

	var childEnv []string
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if strings.EqualFold(name, barrierStackVar) {
			continue // drop all existing TRACKFW_BARRIER_STACK entries
		}
		childEnv = append(childEnv, e)
	}
	childEnv = append(childEnv, newEntry)
	return childEnv
}

// runGateCommand executes one gate command from the repository root (the process's
// current working directory) via `sh -c`. `sh` is resolved through $PATH
// (exec.LookPath, the same as Go has always done) — NOT a fixed /bin/sh path.
//
// env is the explicit environment to pass to the child process; it must include
// the updated TRACKFW_BARRIER_STACK so that nested barrier invocations can detect
// reentrance. If env is nil, the child inherits the current process environment.
//
// 🔴 Inside runBarrier, env == nil is PROHIBITED after the reentry stack is built:
// passing nil silently bypasses TRACKFW_BARRIER_STACK propagation, allowing nested
// barriers to miss the stack and fail to detect reentrance. nil is only correct for
// direct unit tests of runGateCommand itself, outside of runBarrier.
//
// Returns the exit code and spawnFailed, which is true only when the process
// never started at all (e.g. `sh` missing from $PATH). This is distinct from the
// gate command itself failing inside a running `sh`: `sh -c 'nosuchtool'` returns
// exit 127 with spawnFailed=false — sh started and ran, then reported that its
// child command doesn't exist. 127 is a normal (if unusual) exit code, never a
// signal for "sh is missing" (measured in ML-0A).
//
// Transport: the gate text is delivered via stdin (c.Stdin = strings.NewReader(command)),
// NOT via argv. This is the same reason as checkGateFragments: on Windows, Go's
// EscapeArg + MSYS reparse silently converts `esperado="scaffold.go` (a fragment) into
// `esperado=\scaffold.go` (a valid assignment that exits 0), making a malformed gate
// appear to pass. Stdin is byte-identical on all OSes.
// Parity with the former `sh -c <argv>` form measured on macOS over 12 vectors: all
// exit codes identical (stdin chosen; env-eval diverges on 5 vectors — exits 1 vs 2
// for fragments). See vault/notes/windows-argv-troca-aspa-por-contrabarra-sem-espaco-2026-10-01.md.
func runGateCommand(command string, env []string) (exitCode int, spawnFailed bool) {
	// Guard: a gate text with embedded newlines spans multiple lines.
	// The stdin transport is safe because ParseGatesLines (roadmapdoc) cuts by
	// line and TrimSpace removes trailing \r — so each gate text is guaranteed to
	// be one line in production. With more than one line, a gate that reads stdin
	// (`read x`) would consume the NEXT line of the script as its input, as
	// measured by Lourival in PR #495 (reproduced: `lido=[SEGUNDA_LINHA…]` and
	// `command not found` for the line that sh tried to read as a command).
	// Refuse without spawning sh; exit code 2 signals a usage/invariant error.
	if strings.ContainsAny(command, "\n\r") {
		return 2, false
	}

	c := exec.Command("sh")
	c.Stdin = strings.NewReader(command)
	c.Env = env
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
//
// env is passed to each gate child process (see runGateCommand).
func evalGateCommands(gateCommands []string, env []string) (status string, evidence []string, failures []string) {
	evidence = []string{}
	failures = []string{}
	for _, gcmd := range gateCommands {
		exitCode, spawnFailed := runGateCommand(gcmd, env)
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
	// ML-2A (#476): reject documents with an unterminated code fence before
	// attempting to parse waves. A fence opened before a wave heading hides the
	// heading entirely, so the user would receive "wave X not found" — the wrong
	// message — instead of the underlying formatting error. This check takes
	// precedence over all wave-level processing, but comes after file resolution
	// and reading, matching the same fail-fast principle used by ParseGates.
	if _, err := roadmapdoc.FenceMaskCheck(lines); err != nil {
		usageExit(cmd, "%s", err.Error())
		return
	}
	fenced := fenceMask(lines)

	waves, malformed := parseWaves(lines, fenced)
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

	// ── reentrance detection (ML-1A, #485) ──────────────────────────────────
	// Read and validate TRACKFW_BARRIER_STACK before evaluating any check.
	// "wave not found" (above) has precedence; all other errors come after.
	var stack []barrierStackEntry
	if raw := os.Getenv(barrierStackVar); raw != "" {
		if err := json.Unmarshal([]byte(raw), &stack); err != nil {
			usageExit(cmd, "TRACKFW_BARRIER_STACK is malformed: %s", err.Error())
			return
		}
	}

	// F1 fix (ML-2C, #485): use target.Label (the section header as parsed), not
	// waveLabel (the CLI argument). CompareWaveLabels normalises case when finding
	// the wave, so "--wave 1B" resolves to a header "## Wave 1b" whose Label is "1b".
	// Using waveLabel would produce key "1B", which differs from the outer's key "1b"
	// and allow one extra level before detection. The error message still shows the
	// CLI argument (waveLabel) so the user sees exactly what they typed.
	currentKey := barrierReentryKey(roadmapPath, target.Label)
	for _, entry := range stack {
		if sameRoadmapFile(entry.Roadmap, currentKey.Roadmap) && entry.Wave == currentKey.Wave {
			usageExit(cmd, "reentrant call — %s wave %s is already being evaluated by an enclosing barrier",
				filepath.Base(roadmapPath), waveLabel)
			return
		}
	}
	if len(stack) >= barrierMaxDepth {
		usageExit(cmd, "evaluation depth limit exceeded (%d nested barriers) — possible reentrant call via indirection",
			len(stack))
		return
	}

	// Build child env: current stack + current key, propagated to gate processes.
	childStack := append(append([]barrierStackEntry{}, stack...), currentKey)
	childEnv := buildChildEnv(childStack)
	// ────────────────────────────────────────────────────────────────────────

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
		detail := acceptanceEvaluateDetail(lines, fenced, ml)
		switch {
		case !detail.HasBlock:
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: no acceptance block", ml.ID))

		case detail.Unmet > 0:
			// Pending (and unrecognized) criteria block the wave. Lapsed are shown
			// separately even in a blocked ML, for visibility (T2, REQ #514).
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: %d unmet acceptance criteria", ml.ID, detail.Unmet))
			for _, ln := range detail.UnrecognizedLines {
				accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: unrecognized checkbox at line %d", ml.ID, ln))
			}
			if detail.Lapsed > 0 {
				accCheck.Lapsed = append(accCheck.Lapsed, fmt.Sprintf("%s: %d lapsed acceptance criteria", ml.ID, detail.Lapsed))
			}

		case detail.Met == 0 && detail.Lapsed > 0:
			// T8 (REQ #514 / ML-1A): all criteria lapsed — no real evidence. Blocked.
			accOK = false
			accCheck.Failures = append(accCheck.Failures, fmt.Sprintf("%s: all acceptance criteria lapsed", ml.ID))
			accCheck.Lapsed = append(accCheck.Lapsed, fmt.Sprintf("%s: %d lapsed acceptance criteria", ml.ID, detail.Lapsed))

		default:
			// At least one criterion met; lapsed ones are informational.
			accCheck.Evidence = append(accCheck.Evidence, fmt.Sprintf("%s: %d criteria met", ml.ID, detail.Met))
			if detail.Lapsed > 0 {
				accCheck.Lapsed = append(accCheck.Lapsed, fmt.Sprintf("%s: %d lapsed acceptance criteria", ml.ID, detail.Lapsed))
			}
		}
	}
	if accOK {
		accCheck.Status = "passed"
	} else {
		accCheck.Status = "blocked"
	}

	// ── check: gates ──────────────────────────────────────────────────────────
	gcmds, gerr := parseGatesWithLines(lines, target.Start, target.End)
	if gerr != nil {
		usageExit(cmd, "%s", gerr.Error())
		return
	}
	// Build the flat text slice for the Commands field (pinned contract: []string).
	gatesCmds := make([]string, len(gcmds))
	for i, gc := range gcmds {
		gatesCmds[i] = gc.Text
	}
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
		// Explicit consent: check fragments (ML-2A) before executing.
		if fragStatus, fragFails := checkGateFragments(gcmds); fragStatus != "" {
			gatesCheck.Status = fragStatus
			gatesCheck.Failures = fragFails
		} else {
			status, evidence, failures := evalGateCommands(gatesCmds, childEnv)
			gatesCheck.Status = status
			gatesCheck.Evidence = evidence
			gatesCheck.Failures = failures
		}
	} else {
		verdict := roadmapTrustForGates(roadmapPath, data)
		if !verdict.trusted {
			// Roadmap is not trusted: do not execute gates (AC3, AC14).
			// Report as not_evaluated — distinct from passed and blocked (AC6).
			// checkGateFragments is NOT called for untrusted roadmaps (ML-2A).
			gatesCheck.Status = "not_evaluated"
			gatesCheck.Failures = append(gatesCheck.Failures, verdict.failureMsg)
		} else {
			// Trusted: check fragments (ML-2A) before executing.
			if fragStatus, fragFails := checkGateFragments(gcmds); fragStatus != "" {
				gatesCheck.Status = fragStatus
				gatesCheck.Failures = fragFails
			} else {
				status, evidence, failures := evalGateCommands(gatesCmds, childEnv)
				gatesCheck.Status = status
				gatesCheck.Evidence = evidence
				gatesCheck.Failures = failures
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
		// Lapsed criteria are informational ("~" prefix, distinct from "- " failures).
		for _, l := range c.Lapsed {
			fmt.Fprintf(out, "    ~ %s\n", l)
		}
	}
	fmt.Fprintf(out, "\nresult: %s\n", result.Status)
}
