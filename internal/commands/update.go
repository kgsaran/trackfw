package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var dryRun bool
	var jsonOut bool
	var targets []string
	var installMissing bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update trackfw-managed artifacts in the current project",
		Long: `Re-applies current trackfw templates to a project that was previously
initialized with 'trackfw init' or 'trackfw discover --init'.

trackfw update is a project-scope operation and never mutates global state
(the user's home directory). Updates:
  - trackfw rules block in all detected agent config files (CLAUDE.md, GEMINI.md, etc.)
  - scripts/trackfw-validate.sh
  - CI workflow (.github/workflows/trackfw-gate.yml or .gitlab-ci-trackfw.yml)
  - existing Codex agent/skill deployments in this project (without installing missing items)
  - historical Claude slash commands (.claude/commands/trackfw/)
  - Git hooks (surgical: ensures 'trackfw validate' is present)

The historical global Claude compatibility skill and globally installed
Codex agent/skill deployments are updated by 'trackfw update harness'
instead — it runs once per machine and does not require a project.

Other agent and skill integrations are updated explicitly with
'trackfw agents update' and 'trackfw skills update'.

--dry-run, --json, --targets and --install-missing report the same
four-state model (updated/skipped/missing/failed) as 'trackfw update
harness', over a "scope": "project" JSON document — see docs/cli-parity.md.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()

			if !dryRun && !jsonOut && len(targets) == 0 && !installMissing {
				return generators.Update(cwd)
			}

			opts := generators.UpdateOptions{DryRun: dryRun, Targets: targets, InstallMissing: installMissing}
			var report generators.UpdateReport
			var err error
			if jsonOut {
				// The reused generators (generateValidateScript,
				// InjectRulesDetected, ...) print human progress lines as a
				// side effect of writing; with --json, stdout must carry
				// only the result document.
				silenceErr := silenceStdout(func() error {
					report, err = generators.UpdateProject(cwd, opts)
					return nil
				})
				if silenceErr != nil {
					return silenceErr
				}
			} else {
				report, err = generators.UpdateProject(cwd, opts)
			}
			if err != nil {
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
				return fmt.Errorf("trackfw update: %d target(s) failed", report.Summary().Failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Compute and report target states without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit the result document as JSON instead of the text report")
	cmd.Flags().StringSliceVar(&targets, "targets", nil, "Comma-separated subset of target ids (unknown id is a usage error)")
	cmd.Flags().BoolVar(&installMissing, "install-missing", false, "Also install targets currently reported as missing")

	cmd.AddCommand(newUpdateHarnessCmd())
	return cmd
}

// silenceStdout runs fn with os.Stdout temporarily redirected to /dev/null.
// generators.UpdateProject reuses generator functions (generateValidateScript,
// InjectRulesDetected, ForceGenerateClaudeCommands, ...) that print human
// progress lines as a side effect of writing; --json must emit only the
// result document. Mirrors npm's silenceConsole (npm/src/lib/update-engine.js).
func silenceStdout(fn func() error) error {
	devnull, openErr := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if openErr != nil {
		return fn()
	}
	defer devnull.Close() //nolint:errcheck
	orig := os.Stdout
	os.Stdout = devnull
	defer func() { os.Stdout = orig }()
	return fn()
}
