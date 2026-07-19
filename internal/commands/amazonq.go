package commands

import "github.com/spf13/cobra"

func newAmazonQCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "amazonq",
		Short: "Install trackfw rules in .amazonq/rules/ of the current project",
		Long: `Installs 10 specialized rule files in .amazonq/rules/:
  trackfw-architect.md     Principal Software Architect
  trackfw-backend.md       Backend Senior Specialist (Go / Java)
  trackfw-frontend.md      Frontend i18n Senior Specialist (React/Next.js)
  trackfw-qa.md            Quality Assurance Senior Specialist
  trackfw-infra.md         Infrastructure Senior Specialist (K8s/AWS/GitOps)
  trackfw-security.md      DevSecOps Security Specialist
  trackfw-code-quality.md  Code Quality Senior Specialist
  trackfw-dba.md           Database Senior Specialist
  trackfw-ux.md            UX/UI Design Senior Specialist
  trackfw-data.md          Data Engineering & Data Science Senior Specialist

Rules are plain Markdown (no frontmatter) as required by Amazon Q Developer.
Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeprecatedIntegrationAlias(cmd, "amazonq", []string{"project"})
		},
	}
}
