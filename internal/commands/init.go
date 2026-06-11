package commands

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize trackfw governance in the current project",
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	var (
		projectName string
		projectType string
		frontend    string
		backend     string
		pkgManager  string
		hooks       string
		ci          string
	)

	form := huh.NewForm(
		// Grupo 1 — sempre mostrado
		huh.NewGroup(
			huh.NewInput().
				Title("Project name?").
				Value(&projectName),

			huh.NewSelect[string]().
				Title("Project type?").
				Options(
					huh.NewOption("Full-stack (frontend + backend)", "fullstack"),
					huh.NewOption("Frontend only", "frontend"),
					huh.NewOption("Backend only", "backend"),
					huh.NewOption("Governance only (no build stack)", "governance"),
				).
				Value(&projectType),
		),

		// Grupo 2 — Frontend (oculto se backend ou governance)
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Frontend stack?").
				Options(
					huh.NewOption("React / Next.js", "react"),
					huh.NewOption("Vue", "vue"),
					huh.NewOption("Angular", "angular"),
				).
				Value(&frontend),

			huh.NewSelect[string]().
				Title("Package manager?").
				Options(
					huh.NewOption("npm", "npm"),
					huh.NewOption("pnpm", "pnpm"),
					huh.NewOption("yarn", "yarn"),
					huh.NewOption("bun", "bun"),
				).
				Value(&pkgManager),
		).WithHideFunc(func() bool {
			return projectType == "backend" || projectType == "governance"
		}),

		// Grupo 3 — Backend (oculto se frontend ou governance)
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Backend stack?").
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("Java / Spring Boot", "java"),
					huh.NewOption("Node.js", "node"),
					huh.NewOption("Python", "python"),
				).
				Value(&backend),
		).WithHideFunc(func() bool {
			return projectType == "frontend" || projectType == "governance"
		}),

		// Grupo 4 — sempre mostrado
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Git hooks?").
				Options(
					huh.NewOption("husky", "husky"),
					huh.NewOption("lefthook", "lefthook"),
					huh.NewOption("None", "none"),
				).
				Value(&hooks),

			huh.NewSelect[string]().
				Title("CI system?").
				Options(
					huh.NewOption("GitHub Actions", "github-actions"),
					huh.NewOption("GitLab CI", "gitlab-ci"),
					huh.NewOption("None", "none"),
				).
				Value(&ci),
		),

	)

	if err := form.Run(); err != nil {
		return err
	}

	cfg := generators.Config{
		ProjectType: projectType,
		ProjectName: projectName,
		Frontend:    frontend,
		Backend:     backend,
		PkgManager:  pkgManager,
		Hooks:       hooks,
		CI:          ci,
	}

	if err := generators.Scaffold(cfg); err != nil {
		return err
	}

	fmt.Println("\n✓ trackfw initialized — run 'trackfw status' to see your governance state.")
	return nil
}
