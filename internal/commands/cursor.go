package commands

import "github.com/spf13/cobra"

func newCursorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cursor",
		Short: "Install trackfw rules in .cursor/rules/ of the current project",
		Long: `Installs 10 specialized rule files in .cursor/rules/:
  trackfw-architect.mdc     Principal Software Architect
  trackfw-backend.mdc       Backend Senior Specialist (Go / Java)
  trackfw-frontend.mdc      Frontend i18n Senior Specialist (React/Next.js)
  trackfw-qa.mdc            Quality Assurance Senior Specialist
  trackfw-infra.mdc         Infrastructure Senior Specialist (K8s/AWS/GitOps)
  trackfw-security.mdc      DevSecOps Security Specialist
  trackfw-code-quality.mdc  Code Quality Senior Specialist
  trackfw-dba.mdc           Database Senior Specialist
  trackfw-ux.mdc            UX/UI Design Senior Specialist
  trackfw-data.mdc          Data Engineering & Data Science Senior Specialist

Rules use alwaysApply: false (Agent Requested mode) so Cursor activates them on demand.
Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeprecatedIntegrationAlias(cmd, "cursor", []string{"project"})
		},
	}
}
