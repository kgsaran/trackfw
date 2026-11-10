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
	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/spf13/cobra"
)

// applyCITriggerFromExisting reads ci_trigger from an already-existing trackfw.yaml (if any)
// and applies it to cfg. Option (c) from §Z of ML-1A (REQ-2026-10-24):
//   - absent → no change to cfg (CI template will use default "pull_request" behaviour)
//   - "pull_request" or "manual" → set cfg.CITrigger
//   - any other value → return error with accepted values; init must not overwrite the file
//
// No new flag or wizard question. The template does NOT emit the key; it is preserved only if it
// was already present. config.Load() is safe here because runInit is always called from the CWD
// that is (or will become) the project root.
func applyCITriggerFromExisting(cfg *generators.Config) error {
	existing := config.Load()
	if existing.Update.CITriggerInvalid {
		return fmt.Errorf("init: trackfw.yaml: ci_trigger value %q is not accepted (accepted: \"pull_request\", \"manual\") — correct the file before running init", existing.Update.CITriggerRawValue)
	}
	if existing.Update.CITrigger != "" {
		cfg.CITrigger = existing.Update.CITrigger
	}
	return nil
}

// isPathguardRefusal reports whether err is a containment-refusal error emitted
// by pathguard.RejectAndReport or pathguard.RefuseUnverifiableRoot. It
// delegates to pathguard.IsRefusal, which uses errors.As to traverse any
// wrapping layer — so it works even when an intermediate caller wraps the
// refusal with additional context via fmt.Errorf("%w", ...).
//
// Callers use this predicate to gate the "--scope project" remediation hint
// exclusively on refusal errors, not on unrelated failures such as a corrupt
// integrations-manifest.json file.
func isPathguardRefusal(err error) bool {
	return pathguard.IsRefusal(err)
}

// executeScaffoldAndInstall is the core init sequence shared by both the TTY
// (wizard) and non-TTY (CI/script) paths. Extracting it to a named function
// makes the pre-check reachable from tests without simulating a TTY (T-8,
// REQ-2026-11-02): removing the preCheckAIToolDestinations call from this
// function causes TestExecuteScaffoldAndInstall_PreCheckFiresOnSymlinkClaudeDir
// to fail regardless of which path invoked executeScaffoldAndInstall.
func executeScaffoldAndInstall(cfg generators.Config, aiTools []string, cwd, scope, home string) error {
	if err := preCheckAIToolDestinations(aiTools, cwd, scope, home); err != nil {
		return err
	}
	if err := generators.Scaffold(cfg); err != nil {
		return err
	}
	if err := installAITools(aiTools, cwd, scope); err != nil {
		return err
	}
	return nil
}

// preCheckAIToolDestinations verifies that all destinations for the given AI tools are
// writable (no symlinks/reparse points traversed) before any project files are written by
// generators.Scaffold. It checks two groups:
//
//   - Group 1 (BuildPlans destinations): resolved via Manager.List which calls the same
//     resolve/rejectSymlinks path that manager.Install uses, including ClaudeDir for
//     claude/global plans. This catches ~/.claude being a symlink on Windows (junction) or
//     macOS (symlink proxy).
//
//   - Group 2 (InjectRulesForTool project-local destinations): filepath.Join(resolvedCwd,
//     generators.AgentFilePath(tool)) for each selected tool, using pathguard.ResolveRoot
//     for consistency with the actual writer (generators.InjectRulesForTool).
//
// On refusal (symlink/reparse-point detected by pathguard), the error names the refused
// destination and appends a suggestion: use --scope project or trackfw agents install
// --scope project. Non-refusal errors from Group 1 (e.g. corrupt manifest) are returned
// as-is without the hint, since --scope project does not resolve them.
// This is the pre-check described in P-7 of the Wave 0 threat model (REQ-2026-11-02).
//
// The write-time guards inside installAITools and InjectRulesForTool remain active (T-9,
// TOCTOU guard). This function does not replace them.
func preCheckAIToolDestinations(aiTools []string, cwd, scope, home string) error {
	if len(aiTools) == 0 {
		return nil
	}

	// Group 1: BuildPlans destinations — use Manager.List which mirrors manager.Install's
	// resolve() path, including ClaudeDir for claude/global plans.
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return fmt.Errorf("init: pre-check: loading catalog: %w", err)
	}
	ident, err := identity.Load(home)
	if err != nil {
		return fmt.Errorf("init: pre-check: loading identity: %w", err)
	}

	var plans []integrations.PlannedArtifact
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		// AgentModels is not needed for path resolution (only affects content).
		selected, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
			Kind: kind, Targets: aiTools, Scope: scope, Identity: ident,
		})
		if err != nil {
			return fmt.Errorf("init: pre-check: building plans: %w", err)
		}
		plans = append(plans, selected...)
	}

	manager := integrations.Manager{
		ProjectRoot: cwd,
		HomeDir:     home,
		ClaudeDir: func() (string, error) {
			return agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
		},
	}

	// Manager.List calls Inspect → resolve → rejectSymlinks for every plan.
	// A symlink/junction in the destination path causes resolve to return a refusal
	// error, which is already printed to stderr by pathguard.RejectAndReport.
	// The "--scope project" hint is added only for refusal errors; unrelated
	// errors (e.g. "decode integration manifest: ...") are returned as-is
	// because --scope project does not resolve them. The hint is suppressed when
	// scope is already "project" — the user is already using it, so repeating
	// it is misleading (T-12/c-proj, REQ-2026-11-02).
	if _, err := manager.List(plans); err != nil {
		if isPathguardRefusal(err) && scope != "project" {
			return fmt.Errorf("%w\nTo install AI tools in the project directory only, use --scope project; or run `trackfw agents install --scope project` after init completes", err)
		}
		return err
	}

	// Group 2: InjectRulesForTool project-local destinations.
	// Use ResolveRoot to match the guard root that generators.InjectRulesForTool uses
	// (ML-8A / #402: resolves /var/… → /private/var/… on macOS).
	resolvedCwd, resolveErr := pathguard.ResolveRoot(cwd)
	if resolveErr != nil {
		return pathguard.RefuseUnverifiableRoot(cwd, resolveErr)
	}
	for _, tool := range aiTools {
		relPath, ok := generators.AgentFilePath(tool)
		if !ok {
			continue
		}
		dest := filepath.Join(resolvedCwd, relPath)
		if err := pathguard.RejectAndReport(resolvedCwd, dest); err != nil {
			if scope != "project" {
				return fmt.Errorf("%w\nTo install AI tools in the project directory only, use --scope project; or run `trackfw agents install --scope project` after init completes", err)
			}
			return err
		}
	}
	return nil
}

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
	cmd.Flags().String("scope", "global", "Scope for AI tools installation: project or global")
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
		return fmt.Errorf("init: could not resolve home directory: %w", err)
	}

	// Validate --scope early, before any filesystem writes (AC4, P-7).
	scopeValue, _ := cmd.Flags().GetString("scope")
	scopeChanged := cmd.Flags().Changed("scope")
	if scopeChanged && scopeValue != "project" && scopeValue != "global" {
		return fmt.Errorf("init: --scope %q is not valid (accepted: \"project\", \"global\")", scopeValue)
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
				return fmt.Errorf("init: invalid identity: %w", err)
			}
			if err := identity.Save(home, cfg); err != nil {
				return fmt.Errorf("init: failed to save identity: %w", err)
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

		// AC3 (#661), P-6: detect pkg_manager from the project, don't hardcode "npm".
		// A project with package.json gets "npm"; all others get "" (empty).
		// This mirrors the check in internal/discover/discover.go:188 but is inlined
		// because DiscoveryResult does not expose PkgManager (R-4 of Wave 0).
		pkgMgr := ""
		if _, statErr := os.Stat(filepath.Join(cwd, "package.json")); statErr == nil {
			pkgMgr = "npm"
		}

		cfg := generators.Config{
			ProjectName: filepath.Base(cwd),
			ProjectType: "governance",
			Frontend:    "",
			Backend:     "",
			PkgManager:  pkgMgr,
			Hooks:       "none",
			CI:          "none",
		}
		if forgeChanged {
			cfg.Forge = forgeValue
		}
		if err := applyCITriggerFromExisting(&cfg); err != nil {
			return err
		}

		// Resolve scope for this non-TTY run.
		scope := "global"
		if scopeChanged {
			scope = scopeValue
		}

		// AC4 (#657), P-7: pre-check + scaffold + install in one call (T-8:
		// same function called from both TTY and non-TTY paths — removing the
		// pre-check from executeScaffoldAndInstall fails
		// TestExecuteScaffoldAndInstall_PreCheckFiresOnSymlinkClaudeDir).
		aiTools, _ := cmd.Flags().GetStringSlice("ai-tools")
		if err := executeScaffoldAndInstall(cfg, aiTools, cwd, scope, home); err != nil {
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
		cfg.LenientUntil = time.Now().AddDate(0, 0, config.LenientDefaultDays)
	}

	if err := applyCITriggerFromExisting(&cfg); err != nil {
		return err
	}

	// D4 — resolve install scope BEFORE Scaffold (AC4, P-7).
	// --scope flag wins; otherwise prompt (but only if AI tools were selected).
	scope := "global"
	if scopeChanged {
		scope = scopeValue
	} else if len(aiTools) > 0 {
		var err error
		scope, err = promptInstallScopeRunner()
		if err != nil {
			return err
		}
	}

	// Pre-check + scaffold + install via shared function (T-8: same function as
	// the non-TTY path — a test on executeScaffoldAndInstall covers both).
	if err := executeScaffoldAndInstall(cfg, aiTools, cwd, scope, home); err != nil {
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
		return fmt.Errorf("installing AI tools: invalid identity: %w", err)
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
	fmt.Printf("Destination (%s):\n", scope)
	for _, plan := range plans {
		fmt.Printf("  %s\n", plan.Destination)
	}
	// ML-1D (ADR-2026-10-14 D1/A1): Claude global agents/skills land at
	// agentconfig.ClaudeConfigDir, not at HomeDir/.claude, so the writer
	// and the auditor (validate) agree on the path Claude Code actually reads.
	manager := integrations.Manager{
		ProjectRoot: cwd,
		HomeDir:     home,
		ClaudeDir: func() (string, error) {
			return agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
		},
	}
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
	// Inject guard hooks for every CLI that was just installed. Name-based
	// dispatch (not detection-based) closes two gaps at once:
	//   (a) ordering: Scaffold ran InjectHooksDetected before this function
	//       created the detection signals, so 7 of 8 CLIs were skipped;
	//   (b) kiro global scope: installAITools with scope="global" installs into
	//       ~/.kiro/, never creates .kiro/ in the project — InjectKiroHooks
	//       calls MkdirAll(.kiro/hooks/) itself and closes this gap.
	// Non-fatal: a hook-injection failure must not abort scaffolding (mirrors
	// the same non-fatal call in Scaffold, scaffold.go:197).
	if err := generators.InjectHooksForTools(aiTools, cwd); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ agent hooks: %v\n", err)
	}
	return nil
}
