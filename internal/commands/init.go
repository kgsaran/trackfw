package commands

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/trackfw/trackfw/internal/generators"
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
		frontend   string
		backend    string
		pkgManager string
		hooks      string
		ci         string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Frontend stack?").
				Options(
					huh.NewOption("React / Next.js", "react"),
					huh.NewOption("Vue", "vue"),
					huh.NewOption("Angular", "angular"),
					huh.NewOption("None", "none"),
				).
				Value(&frontend),

			huh.NewSelect[string]().
				Title("Backend stack?").
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("Java / Spring Boot", "java"),
					huh.NewOption("Node.js", "node"),
					huh.NewOption("Python", "python"),
					huh.NewOption("None", "none"),
				).
				Value(&backend),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Package manager? (for frontend)").
				Options(
					huh.NewOption("npm", "npm"),
					huh.NewOption("pnpm", "pnpm"),
					huh.NewOption("yarn", "yarn"),
					huh.NewOption("bun", "bun"),
					huh.NewOption("N/A", "none"),
				).
				Value(&pkgManager),

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
		Frontend:   frontend,
		Backend:    backend,
		PkgManager: pkgManager,
		Hooks:      hooks,
		CI:         ci,
	}

	if err := generators.Scaffold(cfg); err != nil {
		return err
	}

	fmt.Println("\n✓ trackfw initialized — run 'trackfw status' to see your governance state.")
	return nil
}
