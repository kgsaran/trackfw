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
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: i18n.T("init.description"),
		RunE:  runInit,
	}
	cmd.Flags().Bool("brownfield", false, "Adopt governance gradually (lenient mode for 30 days)")
	cmd.Flags().StringSlice("ai-tools", nil, "AI tools to configure (claude,codex,gemini,antigravity,cursor,copilot,windsurf,amazonq,kiro)")
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
					huh.NewOption("Google Antigravity", "antigravity"),
					huh.NewOption("Cursor", "cursor"),
					huh.NewOption("GitHub Copilot", "copilot"),
					huh.NewOption("Windsurf", "windsurf"),
					huh.NewOption("Amazon Q Developer", "amazonq"),
					huh.NewOption("Kiro", "kiro"),
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
	if len(aiTools) == 0 {
		return nil
	}
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}
	var plans []integrations.PlannedArtifact
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		selected, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
			Kind: kind, Targets: aiTools, Scope: "project",
		})
		if err != nil {
			return fmt.Errorf("configurando AI tools: %w", err)
		}
		plans = append(plans, selected...)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager := integrations.Manager{ProjectRoot: cwd, HomeDir: home}
	if err := manager.Install(plans, false); err != nil {
		return fmt.Errorf("instalando AI tools: %w", err)
	}
	for _, tool := range aiTools {
		fmt.Printf("  ✓ %s agents and skills\n", tool)
	}
	return nil
}
