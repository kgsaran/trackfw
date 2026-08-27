package commands

import (
	"fmt"
	"os"
	"strings"

	trackversion "github.com/kgsaran/trackfw/internal/version"
	"github.com/spf13/cobra"
)

// newRootCmd builds the full trackfw command tree. It is extracted from
// Execute so tests can inspect the real, registered subcommand set (e.g. to
// prove a command was removed) without depending on os.Exit side effects.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "trackfw",
		Short: "trackfw — governed software delivery framework",
		Long: `trackfw enforces a traceable delivery chain:
ADR → REQ → ROADMAP → backlog/wip/done

Run 'trackfw init' to set up governance in your project.`,
		Version: trackversion.Version,
	}

	helpCmd := newHelpCmd()

	root.SetVersionTemplate("trackfw {{.Version}}\n")

	// Pré-registra a flag "version" sem shorthand, impedindo que o cobra
	// registre automaticamente "--version / -v" via InitDefaultVersionFlag.
	// O cobra só adiciona a flag padrão quando Flags().Lookup("version") == nil,
	// portanto esta declaração reserva o slot sem o atalho -v.
	// Motivação: -v/-−verbose é reservado para modo verboso futuro (cli-parity.md).
	root.Flags().Bool("version", false, "version for trackfw")

	root.AddCommand(
		newInitCmd(),
		newUpdateCmd(),
		newSkillsCmd(),
		newAgentsCmd(),
		newADRCmd(),
		newReqCmd(),
		newRoadmapCmd(),
		newStatusCmd(),
		newValidateCmd(),
		newBaselineCmd(),
		helpCmd,
		newConfigureCmd(),
		newVersionCmd(),
		newLogCmd(),
		NewDiscoverCmd(),
		newServeCmd(),
		newMetricsCmd(),
		newSyncCmd(),
		newContextCmd(),
		newNoteCmd(),
		newShipCmd(),
		newReleaseCmd(),
		newBarrierCmd(),
		newBranchCmd(),
		newCommitCmd(),
		newPushCmd(),
		newChangelogCmd(),
		newDoctorCmd(),
		newAuditSurfaceCmd(),
	)

	// trackfw expõe uma única superfície explícita de ajuda ("help").
	// Sem isto, cobra registra seu próprio comando "help" default além do
	// nosso, duplicando a entrada em `trackfw --help` (Available Commands).
	root.SetHelpCommand(helpCmd)

	return root
}

func Execute() {
	root := newRootCmd()

	// SilenceErrors/SilenceUsage: cobra's ExecuteC prints its own "Error: ..." +
	// usage block by default, guarded ONLY by the root's own flags — a per-command
	// cmd.SilenceErrors/cmd.SilenceUsage (e.g. branch.go's "new", which wants its
	// bare error with no "Error:" prefix and no usage block, matching Node/Python)
	// does not stop cobra's root-level print. That print used to run BEFORE control
	// returned here, then this function printed err a second time — every error was
	// emitted twice (three times for commands that also set their own SilenceErrors).
	// Silencing cobra's print at the root and re-emitting a single message below —
	// which replicates cobra's own per-command SilenceErrors/SilenceUsage semantics
	// exactly, plus the canonical cross-CLI message for "unknown command" — fixes
	// that pre-existing duplication as a side effect of the unknown-command work
	// (ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-
	// terceiro.md).
	root.SilenceErrors = true
	root.SilenceUsage = true

	cmd, err := root.ExecuteC()
	if err != nil {
		if msg, ok := formatUnknownCommandError(root, err); ok {
			fmt.Fprintln(os.Stderr, msg)
		} else {
			// Mirrors cobra's own two independent flags exactly (Command.ExecuteC):
			// a command that opted out of the "Error: " prefix (cmd.SilenceErrors,
			// e.g. branch.go's "new") still needs its bare message on stderr exactly
			// once; usage is gated independently by cmd.SilenceUsage.
			if cmd.SilenceErrors {
				fmt.Fprintln(os.Stderr, err.Error())
			} else {
				fmt.Fprintln(os.Stderr, cmd.ErrPrefix(), err.Error())
			}
			if !cmd.SilenceUsage {
				fmt.Fprintln(os.Stderr, cmd.UsageString())
			}
		}
		os.Exit(1)
	}
}

// formatUnknownCommandError recognizes cobra's own "unknown command %q for %q..."
// error (produced when the root command receives an argument that matches no
// registered subcommand — the exact vector the removed plugin fallback used to
// swallow, see ADR D3) and reformats it into the canonical, cross-CLI message:
//
//	Error: unknown command "x" for "trackfw"
//	Did you mean "validate"?
//	Run 'trackfw --help' for usage.
//
// The "Did you mean" line is emitted only when a close-enough command exists, via
// suggestCommand — the same distance/prefix algorithm reimplemented identically in
// npm/src/commands/index.js and pypi/trackfw/cli.py so the three CLIs always agree
// on whether/what to suggest (docs/cli-parity.md, check-unknown-command-parity.sh).
// Cobra's own SuggestionsFor is deliberately NOT used here: it would include the
// auto-registered "completion" command, which has no Node.js/Python equivalent and
// would make the suggestion set diverge across runtimes.
func formatUnknownCommandError(root *cobra.Command, err error) (string, bool) {
	typed, ok := parseUnknownCommandError(err.Error(), root.CommandPath())
	if !ok {
		return "", false
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Error: unknown command %q for %q\n", typed, root.CommandPath())
	if suggestion, found := suggestCommand(typed, unknownCommandCandidates(root)); found {
		fmt.Fprintf(&sb, "Did you mean %q?\n", suggestion)
	}
	fmt.Fprintf(&sb, "Run '%s --help' for usage.", root.CommandPath())
	return sb.String(), true
}

// parseUnknownCommandError extracts the typed (unrecognized) command name from
// cobra's error text for the exact top-level shape
// `unknown command "<typed>" for "<cmdPath>"` (args.go), ignoring any suggestion
// text cobra may have appended after it — that text is discarded and replaced with
// our own. Only matches when cmdPath is the root's own command path (top-level
// dispatch); a subcommand receiving an unrecognized nested argument is unaffected
// and keeps cobra's existing behavior, out of scope for this change.
func parseUnknownCommandError(errText, cmdPath string) (string, bool) {
	const prefix = `unknown command "`
	suffix := fmt.Sprintf(`" for %q`, cmdPath)

	if !strings.HasPrefix(errText, prefix) {
		return "", false
	}
	rest := errText[len(prefix):]
	idx := strings.Index(rest, suffix)
	if idx < 0 {
		return "", false
	}
	return rest[:idx], true
}

// unknownCommandCandidates returns the top-level command names eligible for
// suggestion — every registered subcommand except cobra's auto-added
// "completion" (Go-only, pre-existing divergence unrelated to plugins removal;
// documented in docs/cli-parity.md).
func unknownCommandCandidates(root *cobra.Command) []string {
	var names []string
	for _, c := range root.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		if c.Name() == "completion" {
			continue
		}
		names = append(names, c.Name())
	}
	return names
}

// suggestCommand picks a single closest candidate for typed, or reports none.
// Shared cross-CLI criterion (reimplemented identically in npm and pypi):
//   - a candidate is eligible when its case-insensitive Levenshtein distance to
//     typed is <= 2, OR it is a case-insensitive prefix of typed's target (i.e.
//     the candidate starts with the typed text);
//   - among eligible candidates, the winner is the one with the lowest distance,
//     tie-broken alphabetically, for a deterministic single suggestion.
func suggestCommand(typed string, candidates []string) (string, bool) {
	lowerTyped := strings.ToLower(typed)

	bestDist := -1
	best := ""
	for _, c := range candidates {
		lowerC := strings.ToLower(c)
		dist := levenshteinDistance(lowerTyped, lowerC)
		prefixMatch := lowerTyped != "" && strings.HasPrefix(lowerC, lowerTyped)
		if dist > 2 && !prefixMatch {
			continue
		}
		if bestDist == -1 || dist < bestDist || (dist == bestDist && c < best) {
			bestDist = dist
			best = c
		}
	}
	return best, bestDist != -1
}

// levenshteinDistance computes the plain (no-transposition) Levenshtein edit
// distance between a and b. Deliberately the textbook algorithm — no framework
// helper — so the same distance function can be reproduced verbatim in
// JavaScript and Python without depending on library-specific edit-distance
// variants (e.g. commander's Damerau-Levenshtein-based suggestSimilar), which is
// what makes the suggestion criterion agree across all three CLIs.
func levenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			min := d[i-1][j] + 1 // deletion
			if v := d[i][j-1] + 1; v < min {
				min = v // insertion
			}
			if v := d[i-1][j-1] + cost; v < min {
				min = v // substitution
			}
			d[i][j] = min
		}
	}
	return d[la][lb]
}
