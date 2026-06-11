package commands

import (
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skills",
		Short: "Install Claude Code slash commands (/trackfw:*) in the current project",
		Long: `Creates .claude/commands/trackfw/ with the 7 slash commands that integrate
trackfw with Claude Code: /trackfw:adr, /trackfw:req, /trackfw:roadmap,
/trackfw:implement, /trackfw:validate, /trackfw:status, /trackfw:move.

Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.InstallSkills()
		},
	}
}
