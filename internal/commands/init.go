package commands

import (
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/identity"
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
	cmd.Flags().StringSlice("ai-tools", nil, "AI tools to configure (claude,codex,gemini,antigravity,cursor,copilot,windsurf,amazonq,opencode,kiro)")
	cmd.Flags().String("identity-preset", "none", "Agent identity preset: none, neutral, "+strings.Join(identity.PresetNames(), ", "))
	cmd.Flags().String("forge", "", "Forge platform: "+strings.Join(forge.ValidForges, ", "))
	return cmd
}

// resolveIdentityPreset translates the --identity-preset flag value into a
// Config to persist. "none" and "neutral" mean "do not write anything" — the
// caller must not create ~/.trackfw/identity.json for those values. An
// unknown value is always an error, listing the accepted values.
func resolveIdentityPreset(value string) (cfg identity.Config, shouldSave bool, err error) {
	if value == "none" || value == "neutral" {
		return identity.Config{}, false, nil
	}
	cfg, err = identity.Preset(value)
	if err != nil {
		valid := append([]string{"none", "neutral"}, identity.PresetNames()...)
		return identity.Config{}, false, fmt.Errorf("identity-preset invalido %q (validos: %s)", value, strings.Join(valid, ", "))
	}
	return cfg, true, nil
}

// identityFileExists reports whether ~/.trackfw/identity.json already
// exists, without depending on internal/identity for the path (it is
// intentionally unexported there).
func identityFileExists(home string) bool {
	_, err := os.Stat(filepath.Join(home, ".trackfw", "identity.json"))
	return err == nil
}

func runInit(cmd *cobra.Command, args []string) error {
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("init: nao foi possivel resolver o diretorio home: %w", err)
	}

	presetValue, _ := cmd.Flags().GetString("identity-preset")
	presetChanged := cmd.Flags().Changed("identity-preset")

	// Flag validation and persistence happen unconditionally, above the
	// non-TTY early return below — this is what makes an invalid
	// --identity-preset fail loudly in CI instead of silently no-op'ing.
	if presetChanged {
		cfg, shouldSave, err := resolveIdentityPreset(presetValue)
		if err != nil {
			return err
		}
		if shouldSave {
			if err := identity.Validate(cfg, identity.KnownAgentIDs()); err != nil {
				return fmt.Errorf("init: identidade invalida: %w", err)
			}
			if err := identity.Save(home, cfg); err != nil {
				return fmt.Errorf("init: falha ao gravar identidade: %w", err)
			}
		}
	}

	// --forge: validate unconditionally above the non-TTY early return, so an
	// invalid value fails loudly in CI instead of silently no-op'ing.
	forgeValue, _ := cmd.Flags().GetString("forge")
	forgeChanged := cmd.Flags().Changed("forge")
	if forgeChanged {
		if _, err := forge.Resolve(forge.Input{FlagForge: forgeValue}); err != nil {
			return err
		}
	}

	// Skip the identity wizard entirely when the flag was passed explicitly
	// (already handled above) or when an identity file already exists —
	// re-running init must never silently overwrite a configured identity.
	skipIdentityWizard := presetChanged || identityFileExists(home)

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
		if forgeChanged {
			cfg.Forge = forgeValue
		}
		if err := generators.Scaffold(cfg); err != nil {
			return err
		}
		aiTools, _ := cmd.Flags().GetStringSlice("ai-tools")
		if err := installAITools(aiTools, cwd, "global"); err != nil {
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
	titleForge := i18n.T("init.prompt.forge")
	titleAITools := i18n.T("init.prompt.aiTools")
	titleRequireReq := i18n.T("init.prompt.require_req_in_commit")

	// Detect forge from the current working dir to prefill the wizard default.
	// If the flag was already provided, that value wins (skip wizard question).
	cwd, _ := os.Getwd()
	wizardForge := "" // "" means "auto-detect (omit key)"
	if !forgeChanged {
		if res, err := forge.ResolveFromRepo("", "", cwd); err == nil && res.Source != "none" {
			wizardForge = res.Forge
		}
	} else {
		wizardForge = forgeValue
	}

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

			huh.NewSelect[string]().
				Title(titleForge).
				Options(
					huh.NewOption("Auto-detect (omit key)", ""),
					huh.NewOption("GitHub", "github"),
					huh.NewOption("GitLab", "gitlab"),
					huh.NewOption("Bitbucket", "bitbucket"),
					huh.NewOption("Azure DevOps", "azure"),
				).
				Value(&wizardForge),
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
					huh.NewOption("OpenCode", "opencode"),
					huh.NewOption("Kiro", "kiro"),
				).
				Value(&aiTools),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Identity wizard runs as its own form, right after the main form — the
	// same shared component (ADR D1) that `agents install` uses. This is
	// skipped entirely (no prompt at all) when the flag path already
	// resolved it above, or when an identity file already exists.
	if !skipIdentityWizard {
		catalog, err := integrations.LoadCatalog()
		if err != nil {
			return err
		}
		if _, _, err := identityWizardRunner(catalog, home); err != nil {
			return err
		}
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
	// Resolve final forge value: explicit flag wins, then wizard selection.
	finalForge := wizardForge
	if forgeChanged {
		finalForge = forgeValue
	}
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
		Forge:              finalForge,
	}
	if brownfield {
		cfg.BrownfieldMode = true
		cfg.LenientUntil = time.Now().AddDate(0, 0, 30)
	}

	if err := generators.Scaffold(cfg); err != nil {
		return err
	}

	// D4 — init's wizard also asks for the install scope, only when AI tools
	// were actually selected (asking otherwise would be a prompt about
	// nothing). Sem TTY já foi tratado no early-return acima (default
	// "global"); este ramo só é alcançado quando stdin é um TTY real.
	scope := "global"
	if len(aiTools) > 0 {
		var err error
		scope, err = promptInstallScopeRunner()
		if err != nil {
			return err
		}
	}

	if err := installAITools(aiTools, cwd, scope); err != nil {
		return err
	}

	fmt.Println(i18n.T("init.success"))
	generators.PrintArchitectNextSteps(cwd)
	return nil
}

// installAITools installs agents and skills for the selected AI tools at the
// given scope ("project" or "global"). scope is resolved by the caller —
// runInit prompts for it (D4) when AI tools were selected and stdin is a
// TTY, and defaults to "global" (D1) in every non-interactive path.
func installAITools(aiTools []string, cwd string, scope string) error {
	if len(aiTools) == 0 {
		return nil
	}
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}
	home, err := homedir.Dir()
	if err != nil {
		return err
	}
	// Resolve the persisted identity BEFORE building plans — if this caller
	// is skipped, PlannedArtifact content silently reverts to neutral names
	// on the next install even though ~/.trackfw/identity.json is present.
	ident, err := identity.Load(home)
	if err != nil {
		return fmt.Errorf("instalando AI tools: identidade invalida: %w", err)
	}
	initAgentModels, initWarnMsg := config.ResolveAgentModels(scope, home, cwd)
	if initWarnMsg != "" {
		fmt.Fprintln(os.Stderr, initWarnMsg)
	}
	var plans []integrations.PlannedArtifact
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		selected, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
			Kind: kind, Targets: aiTools, Scope: scope, Identity: ident,
			AgentModels: initAgentModels,
		})
		if err != nil {
			return fmt.Errorf("configurando AI tools: %w", err)
		}
		plans = append(plans, selected...)
	}
	// D5 — transparency: print resolved destinations before writing anything.
	fmt.Printf("Destino (%s):\n", scope)
	for _, plan := range plans {
		fmt.Printf("  %s\n", plan.Destination)
	}
	manager := integrations.Manager{ProjectRoot: cwd, HomeDir: home}
	manager.OnSkip = func(_ string, reason string) {
		fmt.Fprintln(os.Stderr, reason)
	}
	if err := manager.Install(plans, false); err != nil {
		return fmt.Errorf("instalando AI tools: %w", err)
	}
	// Auxiliary rules files (GEMINI.md, .github/copilot-instructions.md,
	// .windsurfrules, .amazonq/developer/guidelines.md, etc.) are not part of
	// the agents/skills catalog above — they are a separate, tool-specific
	// mechanism (generators.InjectRulesForTool). Without this call, selecting
	// a tool here silently installs agents/skills but never creates its
	// governance rules file in a brand-new project (regression fixed in
	// ML-5E of ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-
	// orquestrador). InjectRulesForTool no-ops for targets with no rules
	// surface (e.g. antigravity, kiro) and is idempotent for repeated runs.
	for _, tool := range aiTools {
		if err := generators.InjectRulesForTool(tool, cwd); err != nil {
			return fmt.Errorf("instalando AI tools: regras auxiliares de %s: %w", tool, err)
		}
	}
	for _, tool := range aiTools {
		fmt.Printf("  ✓ %s agents and skills\n", tool)
	}
	return nil
}
