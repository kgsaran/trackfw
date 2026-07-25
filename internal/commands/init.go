package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
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
	cmd.Flags().StringSlice("ai-tools", nil, "AI tools to configure (claude,codex,gemini,antigravity,cursor,copilot,windsurf,amazonq,kiro)")
	cmd.Flags().String("identity-preset", "none", "Agent identity preset: none, neutral, "+strings.Join(identity.PresetNames(), ", "))
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
	home, err := os.UserHomeDir()
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
		identitySelect     string
		userNickname       string
	)

	knownAgentIDs := identity.KnownAgentIDs()
	customDisplayNames := make([]string, len(knownAgentIDs))

	titleProjectName := i18n.T("init.prompt.projectName")
	titleProjectType := i18n.T("init.prompt.projectType")
	titleFrontendStack := i18n.T("init.prompt.frontendStack")
	titlePkgManager := i18n.T("init.prompt.pkgManager")
	titleBackendLang := i18n.T("init.prompt.backendLang")
	titleGitHooks := i18n.T("init.prompt.gitHooks")
	titleCI := i18n.T("init.prompt.ci")
	titleAITools := i18n.T("init.prompt.aiTools")
	titleRequireReq := i18n.T("init.prompt.require_req_in_commit")
	titleIdentityPreset := i18n.T("init.prompt.identityPreset")
	titleIdentityCustomName := i18n.T("init.prompt.identityCustomName")
	titleIdentityNickname := i18n.T("init.prompt.identityNickname")

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

		// Grupo 6 — identidade dos agentes (oculto se resolvida via flag ou já existente)
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(titleIdentityPreset).
				Options(
					huh.NewOption("Panteão grego (Zeus, Apolo, Afrodite...)", "greek"),
					huh.NewOption("Mitologia nórdica (Odin, Thor, Freya...)", "norse"),
					huh.NewOption("Pioneiros da computação (Turing, Codd, Knuth...)", "pioneers"),
					huh.NewOption("Harry Potter (Dumbledore, Snape, Luna...)", "potter"),
					huh.NewOption("Game of Thrones (Tyrion, Jon, Arya...)", "thrones"),
					huh.NewOption("Senhor dos Anéis (Gandalf, Aragorn, Arwen...)", "tolkien"),
					huh.NewOption("Star Wars (Yoda, Leia, Vader...)", "starwars"),
					huh.NewOption("Chaves (Girafales, Madruga, Chiquinha...)", "chaves"),
					huh.NewOption("Turma da Mônica (Franjinha, Cebolinha, Mônica...)", "turma"),
					huh.NewOption("Panteão egípcio (Thoth, Ísis, Anúbis...)", "egyptian"),
					huh.NewOption("Personalizar um a um", "custom"),
					huh.NewOption("Nomes neutros (padrão)", "neutral"),
				).
				Value(&identitySelect),
		).WithHideFunc(func() bool {
			return skipIdentityWizard
		}),

		// Grupo 7 — nomes customizados um a um (somente se identitySelect == "custom")
		buildCustomIdentityGroup(titleIdentityCustomName, knownAgentIDs, customDisplayNames, func() bool {
			return skipIdentityWizard || identitySelect != "custom"
		}),

		// Grupo 8 — apelido do usuário (oculto para "neutral" e quando resolvida via flag/já existente)
		huh.NewGroup(
			huh.NewInput().
				Title(titleIdentityNickname).
				Value(&userNickname),
		).WithHideFunc(func() bool {
			return skipIdentityWizard || identitySelect == "" || identitySelect == "neutral"
		}),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if !skipIdentityWizard {
		if err := saveWizardIdentity(home, identitySelect, knownAgentIDs, customDisplayNames, userNickname); err != nil {
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
	home, err := os.UserHomeDir()
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
	var plans []integrations.PlannedArtifact
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		selected, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
			Kind: kind, Targets: aiTools, Scope: "project", Identity: ident,
		})
		if err != nil {
			return fmt.Errorf("configurando AI tools: %w", err)
		}
		plans = append(plans, selected...)
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

// buildCustomIdentityGroup builds the huh group for the "Personalizar um a
// um" identity path: one input per known agent id, each validated inline via
// identity.Slugify and checked for slug collisions against the fields
// entered so far.
func buildCustomIdentityGroup(title string, ids []string, values []string, hide func() bool) *huh.Group {
	fields := make([]huh.Field, 0, len(ids))
	for index, id := range ids {
		index := index
		id := id
		fields = append(fields, huh.NewInput().
			Title(fmt.Sprintf("%s (%s)", title, id)).
			Value(&values[index]).
			Validate(func(value string) error {
				slug, err := identity.Slugify(value)
				if err != nil {
					return err
				}
				for j := 0; j < index; j++ {
					if values[j] == "" {
						continue
					}
					otherSlug, err := identity.Slugify(values[j])
					if err != nil {
						continue
					}
					if otherSlug == slug {
						return fmt.Errorf("slug %q duplicado com o agente %q", slug, ids[j])
					}
				}
				return nil
			}))
	}
	return huh.NewGroup(fields...).WithHideFunc(hide)
}

// saveWizardIdentity persists the identity chosen through the interactive
// wizard. "neutral" (and the zero-value select left untouched because the
// group was hidden) mean "do not write anything".
func saveWizardIdentity(home, identitySelect string, knownAgentIDs []string, customDisplayNames []string, userNickname string) error {
	if identitySelect == "" || identitySelect == "neutral" {
		return nil
	}

	var cfg identity.Config
	if identitySelect == "custom" {
		cfg = identity.Config{Agents: make(map[string]identity.AgentIdentity, len(knownAgentIDs))}
		for index, id := range knownAgentIDs {
			slug, err := identity.Slugify(customDisplayNames[index])
			if err != nil {
				return fmt.Errorf("init: identidade customizada invalida para %q: %w", id, err)
			}
			cfg.Agents[id] = identity.AgentIdentity{DisplayName: customDisplayNames[index], Slug: slug}
		}
	} else {
		preset, err := identity.Preset(identitySelect)
		if err != nil {
			return err
		}
		cfg = preset
	}
	cfg.UserNickname = userNickname

	if err := identity.Validate(cfg, identity.KnownAgentIDs()); err != nil {
		return fmt.Errorf("init: identidade invalida: %w", err)
	}
	if err := identity.Save(home, cfg); err != nil {
		return fmt.Errorf("init: falha ao gravar identidade: %w", err)
	}
	return nil
}
