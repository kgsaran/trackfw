package commands

import "github.com/spf13/cobra"

func newCopilotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copilot",
		Short: "Install trackfw instructions and prompts in .github/ of the current project",
		Long: `Installs GitHub Copilot configuration in .github/:
  .github/copilot-instructions.md       Global governance instructions
  .github/instructions/trackfw-*.instructions.md  10 role-specific instructions (applyTo: **)
  .github/prompts/trackfw-*.prompt.md   10 invocable prompts (/trackfw-architect, etc.)

Roles covered: architect, backend, frontend, qa, infra, security, code-quality, dba, ux, data.
Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeprecatedIntegrationAlias(cmd, "copilot", []string{"project"})
		},
	}
}
