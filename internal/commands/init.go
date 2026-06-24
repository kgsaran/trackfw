package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: i18n.T("init.description"),
		RunE:  runInit,
	}
	cmd.Flags().Bool("brownfield", false, "Adopt governance gradually (lenient mode for 30 days)")
	cmd.Flags().StringSlice("ai-tools", nil, "AI tools to configure (codex,claude,gemini,cursor,copilot,windsurf,amazonq)")
	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	// Non-TTY: use defaults and skip wizard (matches npm CLI behavior)
	if !cbterm.IsTerminal(uintptr(os.Stdin.Fd())) {
		cwd, _ := os.Getwd()
		cfg := generators.Config{
			ProjectName: filepath.Base(cwd),
			ProjectType: "governance",
			Frontend:    "",
			Backend:     "",
			PkgManager:  "npm",
			Hooks:       "none",
			CI:          "none",
		}
		if err := generators.Scaffold(cfg); err != nil {
			return err
		}
		aiTools, _ := cmd.Flags().GetStringSlice("ai-tools")
		if err := installAITools(aiTools, cwd); err != nil {
			return err
		}
		fmt.Println(i18n.T("init.success"))
		return nil
	}

	var (
		projectName        string
		projectType        string
		frontend           string
		backend            string
		backendFramework   string
		pkgManager         string
		hooks              string
		ci                 string
		aiTools            []string
		requireReqInCommit bool
	)

	titleProjectName := i18n.T("init.prompt.projectName")
	titleProjectType := i18n.T("init.prompt.projectType")
	titleFrontendStack := i18n.T("init.prompt.frontendStack")
	titlePkgManager := i18n.T("init.prompt.pkgManager")
	titleBackendLang := i18n.T("init.prompt.backendLang")
	titleGitHooks := i18n.T("init.prompt.gitHooks")
	titleCI := i18n.T("init.prompt.ci")
	titleAITools := i18n.T("init.prompt.aiTools")
	titleRequireReq := i18n.T("init.prompt.require_req_in_commit")

	form := huh.NewForm(
		// Grupo 1 — sempre mostrado
		huh.NewGroup(
			huh.NewInput().
				Title(titleProjectName).
				Value(&projectName),

			huh.NewSelect[string]().
				Title(titleProjectType).
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
				Title(titleFrontendStack).
				Options(
					huh.NewOption("React / Next.js", "react"),
					huh.NewOption("Vue", "vue"),
					huh.NewOption("Angular", "angular"),
				).
				Value(&frontend),

			huh.NewSelect[string]().
				Title(titlePkgManager).
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
				Title(titleBackendLang).
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
				Title(titleGitHooks).
				Options(
					huh.NewOption("husky", "husky"),
					huh.NewOption("lefthook", "lefthook"),
					huh.NewOption("None", "none"),
				).
				Value(&hooks),

			huh.NewSelect[string]().
				Title(titleCI).
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
				Title(titleAITools).
				Options(
					huh.NewOption("Claude Code", "claude"),
					huh.NewOption("OpenAI Codex", "codex"),
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

	// Pergunta condicional: require_req_in_commit (somente quando hooks != "none")
	if hooks != "none" {
		reqForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(titleRequireReq).
					Value(&requireReqInCommit),
			),
		)
		if err := reqForm.Run(); err != nil {
			return err
		}
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
			titleBackendFramework := i18n.T("init.prompt.backendFramework")
			frameworkForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(titleBackendFramework).
						Options(choices...).
						Value(&backendFramework),
				),
			)
			if err := frameworkForm.Run(); err != nil {
				return err
			}
		}
	}

	brownfield, _ := cmd.Flags().GetBool("brownfield")
	cfg := generators.Config{
		ProjectType:        projectType,
		ProjectName:        projectName,
		Frontend:           frontend,
		Backend:            backend,
		BackendFramework:   backendFramework,
		PkgManager:         pkgManager,
		Hooks:              hooks,
		CI:                 ci,
		RequireReqInCommit: requireReqInCommit,
	}
	if brownfield {
		cfg.BrownfieldMode = true
		cfg.LenientUntil = time.Now().AddDate(0, 0, 30)
	}

	if err := generators.Scaffold(cfg); err != nil {
		return err
	}

	cwd, _ := os.Getwd()

	if err := installAITools(aiTools, cwd); err != nil {
		return err
	}

	fmt.Println(i18n.T("init.success"))
	generators.PrintArchitectNextSteps(cwd)
	return nil
}

func installAITools(aiTools []string, cwd string) error {
	for _, tool := range aiTools {
		switch tool {
		case "codex":
			if err := generators.InstallCodex(cwd); err != nil {
				return fmt.Errorf("instalando Codex: %w", err)
			}
		case "claude":
			if err := generators.InstallAgents(); err != nil {
				return fmt.Errorf("instalando agentes Claude: %w", err)
			}
			// CLAUDE.md já tratado por generateClaudeMD dentro de Scaffold
		case "gemini":
			if err := generators.InstallGemini(); err != nil {
				return fmt.Errorf("instalando Gemini: %w", err)
			}
			if err := generators.InjectRulesForTool("gemini", cwd); err != nil {
				fmt.Printf("  ⚠ gemini rules inject: %v\n", err)
			} else {
				fmt.Println("  ✓ trackfw rules → GEMINI.md")
			}
		case "cursor":
			if err := generators.InstallCursor(); err != nil {
				return fmt.Errorf("instalando Cursor: %w", err)
			}
			if err := generators.InjectRulesForTool("cursor", cwd); err != nil {
				fmt.Printf("  ⚠ cursor rules inject: %v\n", err)
			} else {
				fmt.Println("  ✓ trackfw rules → .cursor/rules/trackfw.mdc")
			}
		case "copilot":
			if err := generators.InstallCopilot(); err != nil {
				return fmt.Errorf("instalando Copilot: %w", err)
			}
			if err := generators.InjectRulesForTool("copilot", cwd); err != nil {
				fmt.Printf("  ⚠ copilot rules inject: %v\n", err)
			} else {
				fmt.Println("  ✓ trackfw rules → .github/copilot-instructions.md")
			}
		case "windsurf":
			if err := generators.InstallWindsurf(); err != nil {
				return fmt.Errorf("instalando Windsurf: %w", err)
			}
			if err := generators.InjectRulesForTool("windsurf", cwd); err != nil {
				fmt.Printf("  ⚠ windsurf rules inject: %v\n", err)
			} else {
				fmt.Println("  ✓ trackfw rules → .windsurfrules")
			}
		case "amazonq":
			if err := generators.InstallAmazonQ(); err != nil {
				return fmt.Errorf("instalando Amazon Q: %w", err)
			}
			if err := generators.InjectRulesForTool("amazonq", cwd); err != nil {
				fmt.Printf("  ⚠ amazonq rules inject: %v\n", err)
			} else {
				fmt.Println("  ✓ trackfw rules → .amazonq/developer/guidelines.md")
			}
		}
	}
	return nil
}
