package commands

import (
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newGeminiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gemini",
		Short: "Install trackfw skills and commands for Gemini CLI",
		Long: `Installs Gemini CLI configuration:
  ~/.gemini/GEMINI.md                           Global governance instructions
  GEMINI.md (current directory)                 Project-scoped instructions
  ~/.gemini/skills/trackfw-<role>/SKILL.md      10 role skills
  ~/.gemini/commands/trackfw-*.toml             3 governance commands (/trackfw-adr, etc.)

Roles: architect, backend, frontend, qa, infra, security, code-quality, dba, ux, data.
Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.InstallGemini()
		},
	}
}
