package commands

import (
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newWindsurfCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "windsurf",
		Short: "Install trackfw rules and workflows in .windsurf/ of the current project",
		Long: `Installs Windsurf configuration:
  .windsurf/rules/trackfw-*.md       10 role rules (trigger: model_decision)
  .windsurf/workflows/trackfw-*.md   3 governance workflows (ADR, REQ, implement)
  ~/.codeium/windsurf/memories/global_rules.md  Appended with trackfw governance summary

Roles: architect, backend, frontend, qa, infra, security, code-quality, dba, ux, data.
The global rules file is never overwritten — trackfw content is appended only if not present.
Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.InstallWindsurf()
		},
	}
}
