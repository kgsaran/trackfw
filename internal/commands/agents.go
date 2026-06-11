package commands

import (
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newAgentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "Install trackfw agent constellation in ~/.claude/agents/",
		Long: `Installs 10 specialized agents in ~/.claude/agents/:
  trackfw-zeus        Principal Software Architect (orchestrator)
  trackfw-apolo       Backend Senior Specialist (Go / Java)
  trackfw-afrodite    Frontend i18n Senior Specialist (React/Next.js)
  trackfw-artemis     Quality Assurance Senior Specialist (Playwright/Vitest)
  trackfw-ares        Infrastructure Senior Specialist (K8s/AWS/GitOps)
  trackfw-hades       DevSecOps Security Specialist (SAST/DAST/Zero Trust)
  trackfw-hephaestus  Code Quality Senior Specialist (SonarQube/Semgrep)
  trackfw-poseidon    Database Senior Specialist (PostgreSQL/ArangoDB/vectors)
  trackfw-athena      UX/UI Design Senior Specialist (Figma/WCAG 2.2)
  trackfw-metis       Data Engineering & Data Science Senior Specialist

Agents use the trackfw- prefix to avoid colliding with existing agents.
Safe to run multiple times — existing files are never overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.InstallAgents()
		},
	}
}
