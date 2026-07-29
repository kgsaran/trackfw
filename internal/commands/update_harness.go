package commands

// update_harness.go implements `trackfw update harness`, the global-scope
// counterpart to `trackfw update` described in docs/cli-parity.md
// ("## `trackfw update` vs `trackfw update harness`"). It never requires
// trackfw.yaml or a project working directory, and it never touches anything
// outside the user's home directory.

import (
	"encoding/json"
	"fmt"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

// updateSummaryDoc mirrors generators.UpdateSummary with the four counters
// always emitted (no omitempty) — pinned by contract.
type updateSummaryDoc struct {
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Missing int `json:"missing"`
	Failed  int `json:"failed"`
}

// updateTargetDoc mirrors generators.TargetResult. Message is last and
// omitempty so a target with no failure serializes identically to the
// contract's example, which never shows a "message" key.
type updateTargetDoc struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Path    string `json:"path"`
	Message string `json:"message,omitempty"`
}

// updateResultDoc is the root JSON document emitted by --json, for both
// `trackfw update` and `trackfw update harness`. Field order is pinned by
// contract: scope, dry_run, targets, summary.
type updateResultDoc struct {
	Scope   string            `json:"scope"`
	DryRun  bool              `json:"dry_run"`
	Targets []updateTargetDoc `json:"targets"`
	Summary updateSummaryDoc  `json:"summary"`
}

func toUpdateResultDoc(report generators.UpdateReport) updateResultDoc {
	targets := make([]updateTargetDoc, 0, len(report.Targets))
	for _, t := range report.Targets {
		targets = append(targets, updateTargetDoc{
			ID:      t.ID,
			State:   string(t.State),
			Path:    t.Path,
			Message: t.Message,
		})
	}
	s := report.Summary()
	return updateResultDoc{
		Scope:   report.Scope,
		DryRun:  report.DryRun,
		Targets: targets,
		Summary: updateSummaryDoc{Updated: s.Updated, Skipped: s.Skipped, Missing: s.Missing, Failed: s.Failed},
	}
}

func newUpdateHarnessCmd() *cobra.Command {
	var dryRun bool
	var jsonOut bool
	var targets []string
	var installMissing bool

	cmd := &cobra.Command{
		Use:   "harness",
		Short: "Update trackfw-managed artifacts already installed in the user's global harness",
		Long: `trackfw update harness re-applies current trackfw templates to every
already-installed global-scope artifact in the user's home directory
(the historical Claude compatibility skill, and globally installed
Codex agent/skill deployments).

It never requires trackfw.yaml or a project working directory, and it never
touches anything inside the current repository — that is the job of
'trackfw update'. A target that is not installed is reported as "missing"
and left alone unless --install-missing is passed explicitly; a harness with
nothing installed reports every target "missing" and exits 0.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := generators.UpdateOptions{DryRun: dryRun, Targets: targets, InstallMissing: installMissing}
			report, err := generators.UpdateHarness(opts)
			if err != nil {
				// Unknown --targets id is a usage error — leave usage visible.
				return err
			}

			if jsonOut {
				out, marshalErr := json.Marshal(toUpdateResultDoc(report))
				if marshalErr != nil {
					cmd.SilenceUsage = true
					return marshalErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			} else {
				printUpdateReportText(cmd, report)
			}

			cmd.SilenceUsage = true
			if report.Summary().Failed > 0 {
				return fmt.Errorf("trackfw update harness: %d target(s) failed", report.Summary().Failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Compute and report target states without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit the result document as JSON instead of the text report")
	cmd.Flags().StringSliceVar(&targets, "targets", nil, "Comma-separated subset of target ids (unknown id is a usage error)")
	cmd.Flags().BoolVar(&installMissing, "install-missing", false, "Also install targets currently reported as missing")

	return cmd
}

// printUpdateReportText renders a human-readable report of an update result
// (project or harness scope).
func printUpdateReportText(cmd *cobra.Command, report generators.UpdateReport) {
	out := cmd.OutOrStdout()
	label := "trackfw update"
	if report.Scope == "harness" {
		label = "trackfw update harness"
	}
	if report.DryRun {
		label += " (dry-run)"
	}
	fmt.Fprintf(out, "%s\n", label)
	for _, t := range report.Targets {
		symbol := "•"
		switch t.State {
		case generators.TargetUpdated:
			symbol = "✓"
		case generators.TargetFailed:
			symbol = "✗"
		}
		fmt.Fprintf(out, "%s %s: %s (%s)\n", symbol, t.ID, t.State, t.Path)
		if t.Message != "" {
			fmt.Fprintf(out, "    - %s\n", t.Message)
		}
	}
	s := report.Summary()
	fmt.Fprintf(out, "\nupdated=%d skipped=%d missing=%d failed=%d\n", s.Updated, s.Skipped, s.Missing, s.Failed)
}
