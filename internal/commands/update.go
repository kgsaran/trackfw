package commands

import (
	"os"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
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
'trackfw agents update' and 'trackfw skills update'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			return generators.Update(cwd)
		},
	}
	cmd.AddCommand(newUpdateHarnessCmd())
	return cmd
}
