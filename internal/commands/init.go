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
		projectName      string
		projectType      string
		frontend         string
		backend          string
		backendFramework string
		pkgManager       string
		hooks            string
		ci               string
		aiTools          []string
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
				Title("Backend language?").
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

		// Grupo 5 — seleção de ferramentas de IA
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which AI assistants do you use?").
				Options(
					huh.NewOption("Claude Code", "claude"),
					huh.NewOption("Gemini CLI", "gemini"),
					huh.NewOption("Cursor", "cursor"),
					huh.NewOption("GitHub Copilot", "copilot"),
					huh.NewOption("Windsurf", "windsurf"),
					huh.NewOption("Amazon Q Developer", "amazonq"),
				).
				Value(&aiTools),
		),

	)

	if err := form.Run(); err != nil {
		return err
	}

	if backend != "" {
		frameworkChoices := map[string][]huh.Option[string]{
			"go": {
				huh.NewOption("Gin", "gin"),
				huh.NewOption("Echo", "echo"),
				huh.NewOption("Fiber", "fiber"),
				huh.NewOption("Standard library (net/http)", "stdlib"),
			},
			"java": {
				huh.NewOption("Spring Boot", "spring-boot"),
				huh.NewOption("Quarkus", "quarkus"),
				huh.NewOption("Micronaut", "micronaut"),
			},
			"node": {
				huh.NewOption("Express", "express"),
				huh.NewOption("Fastify", "fastify"),
				huh.NewOption("NestJS", "nestjs"),
				huh.NewOption("Koa", "koa"),
			},
			"python": {
				huh.NewOption("FastAPI", "fastapi"),
				huh.NewOption("Django", "django"),
				huh.NewOption("Flask", "flask"),
			},
		}
		choices := frameworkChoices[backend]
		if len(choices) > 0 {
			frameworkForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Backend framework?").
						Options(choices...).
						Value(&backendFramework),
				),
			)
			if err := frameworkForm.Run(); err != nil {
				return err
			}
		}
	}

	cfg := generators.Config{
		ProjectType:      projectType,
		ProjectName:      projectName,
		Frontend:         frontend,
		Backend:          backend,
		BackendFramework: backendFramework,
		PkgManager:       pkgManager,
		Hooks:            hooks,
		CI:               ci,
	}

	if err := generators.Scaffold(cfg); err != nil {
		return err
	}

	for _, tool := range aiTools {
		switch tool {
		case "claude":
			if err := generators.InstallAgents(); err != nil {
				return fmt.Errorf("instalando agentes Claude: %w", err)
			}
		case "gemini":
			if err := generators.InstallGemini(); err != nil {
				return fmt.Errorf("instalando Gemini: %w", err)
			}
		case "cursor":
			if err := generators.InstallCursor(); err != nil {
				return fmt.Errorf("instalando Cursor: %w", err)
			}
		case "copilot":
			if err := generators.InstallCopilot(); err != nil {
				return fmt.Errorf("instalando Copilot: %w", err)
			}
		case "windsurf":
			if err := generators.InstallWindsurf(); err != nil {
				return fmt.Errorf("instalando Windsurf: %w", err)
			}
		case "amazonq":
			if err := generators.InstallAmazonQ(); err != nil {
				return fmt.Errorf("instalando Amazon Q: %w", err)
			}
		}
	}

	fmt.Println("\n✓ trackfw initialized — run 'trackfw status' to see your governance state.")
	return nil
}
