package commands

import (
	"os"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update trackfw-managed artifacts to the current version",
		Long: `Re-applies current trackfw templates to a project that was previously
initialized with 'trackfw init' or 'trackfw discover --init'.

Updates:
  - trackfw rules block in all detected agent config files (CLAUDE.md, GEMINI.md, etc.)
  - scripts/trackfw-validate.sh
  - CI workflow (.github/workflows/trackfw-gate.yml or .gitlab-ci-trackfw.yml)
  - existing Codex agent/skill deployments (without installing missing items)
  - historical Claude slash commands and global compatibility skill
  - Git hooks (surgical: ensures 'trackfw validate' is present)

Other agent and skill integrations are updated explicitly with
'trackfw agents update' and 'trackfw skills update'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			return generators.Update(cwd)
		},
	}
}
