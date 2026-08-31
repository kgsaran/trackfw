package commands

import (
	"encoding/json"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

type integrationOptions struct {
	targets        []string
	items          []string
	scope          string
	scopeExplicit  bool
	surfaces       []string
	json           bool
	force          bool
	identity       bool
	identityPreset string
}

type deploymentOutput struct {
	Target         string                      `json:"target"`
	Surface        string                      `json:"surface"`
	Scope          string                      `json:"scope"`
	Item           string                      `json:"item"`
	SupportLevel   string                      `json:"support_level"`
	Representation string                      `json:"representation"`
	Destination    string                      `json:"destination"`
	State          integrations.LifecycleState `json:"state"`
	Managed        bool                        `json:"managed"`
}

type lifecycleOutput struct {
	Kind           integrations.ItemKind `json:"kind"`
	CatalogVersion string                `json:"catalog_version"`
	Items          []itemOutput          `json:"items"`
	Deployments    []deploymentOutput    `json:"deployments"`
}

type itemOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

var integrationsStdinIsTTY = func() bool { return cbterm.IsTerminal(uintptr(os.Stdin.Fd())) }

func newIntegrationsLifecycleCmd(kind integrations.ItemKind) *cobra.Command {
	cmd := &cobra.Command{
		Use:   string(kind),
		Short: fmt.Sprintf("Manage trackfw %s across supported AI CLIs", kind),
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(
		newIntegrationListCmd(kind),
		newIntegrationMutationCmd(kind, "install"),
		newIntegrationMutationCmd(kind, "uninstall"),
		newIntegrationMutationCmd(kind, "update"),
		newIntegrationsThirdPartyCmd(kind),
	)
	// "models" is agents-only: skills have no model field, and surfacing this
	// under `trackfw skills models` would mislead users. Mirrors the identity
	// flag gate at integrations_flags.go that keeps --identity out of skills.
	if kind == integrations.KindAgents {
		cmd.AddCommand(newAgentModelsCmd())
	}
	return cmd
}

func newIntegrationListCmd(kind integrations.ItemKind) *cobra.Command {
	opts := integrationOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List available and deployed trackfw %s", kind),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeIntegrationList(cmd, kind, opts)
		},
	}
	addIntegrationFlags(cmd, &opts, false, kind, "list")
	return cmd
}

func newIntegrationMutationCmd(kind integrations.ItemKind, operation string) *cobra.Command {
	opts := integrationOptions{}
	cmd := &cobra.Command{
		Use:   operation,
		Short: fmt.Sprintf("%s trackfw %s", strings.Title(operation), kind), //nolint:staticcheck
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeIntegrationMutation(cmd, kind, operation, &opts)
		},
	}
	addIntegrationFlags(cmd, &opts, true, kind, operation)
	return cmd
}

func addIntegrationFlags(cmd *cobra.Command, opts *integrationOptions, mutation bool, kind integrations.ItemKind, operation string) {
	cmd.Flags().StringSliceVar(&opts.targets, "targets", nil, "target CLIs (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.items, "items", nil, "catalog items (comma-separated; default: all)")
	cmd.Flags().StringVar(&opts.scope, "scope", "", "installation scope: project or global (default: global; asks interactively)")
	cmd.Flags().StringArrayVar(&opts.surfaces, "surface", nil, "target=surface selection (repeatable)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "print canonical JSON output")
	if mutation {
		cmd.Flags().BoolVar(&opts.force, "force", false, forceHelp(operation))
	}
	// Identity flags are agents-only (ADR D5): skills have no identity, and
	// newIntegrationsLifecycleCmd is shared between `agents` and `skills` —
	// without this kind gate, `trackfw skills install --identity` would
	// silently accept a flag with no effect at all.
	if mutation && kind == integrations.KindAgents {
		cmd.Flags().BoolVar(&opts.identity, "identity", false, "reconfigure agent identity even if ~/.trackfw/identity.json already exists")
		cmd.Flags().StringVar(&opts.identityPreset, "identity-preset", "", "agent identity preset (non-interactive): none, neutral, "+strings.Join(identity.PresetNames(), ", "))
	}
}

// forceHelp returns the --force help text for a mutation subcommand. The
// three operations grant --force different powers, and a single shared
// string previously overstated update/uninstall's reach while never
// mentioning install's ability to adopt unmanaged bytes — that ambiguity is
// what sent a user straight into the "unmanaged artifact ... does not match
// a trackfw template" error on `update --force` (see
// unmanagedArtifactError in internal/integrations/manager.go for the
// matching remediation). Mirrors npm/src/commands/integrations.js and
// pypi/trackfw/integrations/command.py.
func forceHelp(operation string) string {
	switch operation {
	case "install":
		return "replace a modified managed artifact, or adopt/overwrite an unmanaged file already on disk"
	case "uninstall":
		return "remove a modified managed artifact"
	default: // "update"
		return "replace a modified managed artifact; never adopts unmanaged bytes — use 'install --force' for that"
	}
}

func executeIntegrationMutation(cmd *cobra.Command, kind integrations.ItemKind, operation string, opts *integrationOptions) error {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}
	// resolveScope is a gate independent from target/item selection (ADR D2):
	// it must run — and may prompt — even when --targets was already
	// supplied below, which is the most common invocation shape.
	if err := resolveScope(cmd, opts, operation); err != nil {
		return err
	}

	// --identity-preset is validated and persisted unconditionally, above
	// every TTY-dependent branch below — mirrors init's --identity-preset
	// handling so an invalid value always fails loudly instead of silently
	// no-op'ing in a non-interactive CI run.
	presetChanged := kind == integrations.KindAgents && cmd.Flags().Changed("identity-preset")
	if presetChanged {
		if err := applyIdentityPresetFlag(opts.identityPreset, operation); err != nil {
			return err
		}
	}

	if len(opts.targets) == 0 {
		if !integrationsStdinIsTTY() {
			return fmt.Errorf("%s requires --targets in non-interactive mode", operation)
		}
		if err := promptIntegrationSelection(catalog, kind, opts); err != nil {
			return err
		}
		if len(opts.targets) == 0 {
			return fmt.Errorf("select at least one target CLI")
		}
	}
	surfaceMap, err := parseSurfaceFlags(opts.surfaces)
	if err != nil {
		return err
	}
	if integrationsStdinIsTTY() {
		if err := promptAmbiguousSurfaces(catalog, kind, opts.targets, surfaceMap); err != nil {
			return err
		}
	}
	manager, err := integrationsManager()
	if err != nil {
		return err
	}

	// Identity wizard trigger (ADR ADR-2026-07-25-wizard-unificado-de-
	// identidade-no-agents-install, D2): shown only when the flag path above
	// did not already resolve identity for this run, and only for agents
	// (never skills, D5). Runs after target/surface selection and before
	// BuildPlans so the wizard's freshly-saved identity is what gets
	// rendered into the plans below.
	if kind == integrations.KindAgents && !presetChanged {
		identityExists := identityFileExists(manager.HomeDir)
		if shouldPromptIdentity(kind, integrationsStdinIsTTY(), identityExists, opts.identity) {
			if _, _, err := identityWizardRunner(catalog, manager.HomeDir); err != nil {
				return err
			}
		} else if identityExists && !opts.json {
			existing, err := identity.Load(manager.HomeDir)
			if err != nil {
				return fmt.Errorf("%s: identidade invalida: %w", operation, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", i18n.T("identity.inUse", "count", strconv.Itoa(len(existing.Agents))))
		}
	}

	// Identity must be resolved from disk before BuildPlans — skipping this
	// silently reverts custom agent names to the neutral defaults.
	ident, err := identity.Load(manager.HomeDir)
	if err != nil {
		return fmt.Errorf("%s: identidade invalida: %w", operation, err)
	}
	agentModels, warnMsg := config.ResolveAgentModels(opts.scope, manager.HomeDir, manager.ProjectRoot)
	if warnMsg != "" {
		fmt.Fprintln(os.Stderr, warnMsg)
	}
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind: kind, Targets: opts.targets, Items: opts.items, Scope: opts.scope, Surfaces: surfaceMap, Identity: ident,
		ProjectRoot: manager.ProjectRoot,
		AgentModels: agentModels,
	})
	if err != nil {
		return err
	}
	// D5 — transparency: no extra confirmation, but the resolved destination
	// paths are always printed before anything is written to disk.
	if !opts.json {
		fmt.Fprintf(cmd.OutOrStdout(), "Destino (%s):\n", opts.scope)
		for _, plan := range plans {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", plan.Destination)
		}
	}
	if operation == "install" {
		manager.OnSkip = func(_ string, reason string) {
			fmt.Fprintln(os.Stderr, reason)
		}
	}
	switch operation {
	case "install":
		err = manager.Install(plans, opts.force)
	case "update":
		err = manager.Update(plans, opts.force)
	case "uninstall":
		err = manager.Uninstall(plans, opts.force)
	default:
		return fmt.Errorf("unsupported lifecycle operation %q", operation)
	}
	if err != nil {
		return err
	}
	// Auxiliary rules files (GEMINI.md, .github/copilot-instructions.md,
	// .windsurfrules, .amazonq/developer/guidelines.md, etc.) are outside the
	// agents/skills catalog managed by Manager above — they are a separate,
	// tool-specific mechanism (generators.InjectRulesForTool), and this is
	// the canonical catalog-based install path (`trackfw agents|skills
	// install --targets <tool>`). Restores the behavior the removed
	// deprecated CLI aliases used to provide (ML-5E of ROADMAP-2026-07-29-
	// barrier-governanca-e-autoridade-do-orquestrador). Scoped to "install"
	// only, mirroring the one-shot semantics the old aliases had.
	// InjectRulesForTool no-ops for targets without a rules surface (e.g.
	// antigravity, kiro) and is idempotent for repeated runs.
	if operation == "install" {
		for _, target := range opts.targets {
			if err := generators.InjectRulesForTool(target, manager.ProjectRoot); err != nil {
				return fmt.Errorf("install %s auxiliary rules: %w", target, err)
			}
		}
	}
	if opts.json {
		return printLifecycleOutput(cmd, catalog, kind, plans, manager)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s complete: %d %s artifact(s)\n", operation, len(plans), kind)
	return nil
}

// applyIdentityPresetFlag resolves and persists --identity-preset for
// `agents install|update|uninstall`, reusing resolveIdentityPreset (init.go)
// so both commands accept exactly the same preset names and reject invalid
// ones with the exact same error shape.
func applyIdentityPresetFlag(presetValue, operation string) error {
	cfg, shouldSave, err := resolveIdentityPreset(presetValue)
	if err != nil {
		return err
	}
	if !shouldSave {
		return nil
	}
	home, err := homedir.Dir()
	if err != nil {
		return err
	}
	if err := identity.Validate(cfg, identity.KnownAgentIDs()); err != nil {
		return fmt.Errorf("%s: identidade invalida: %w", operation, err)
	}
	if err := identity.Save(home, cfg); err != nil {
		return fmt.Errorf("%s: falha ao gravar identidade: %w", operation, err)
	}
	return nil
}

func executeIntegrationList(cmd *cobra.Command, kind integrations.ItemKind, opts integrationOptions) error {
	// D6: list is a read-only command and never prompts, but adopts the same
	// "global" default as install/update/uninstall so it never reports
	// deployments for a scope the mutating commands wouldn't have written to.
	if opts.scope == "" {
		opts.scope = "global"
	}
	if opts.scope != "project" && opts.scope != "global" {
		return fmt.Errorf("invalid --scope %q: use project or global", opts.scope)
	}
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}
	surfaceMap, err := parseSurfaceFlags(opts.surfaces)
	if err != nil {
		return err
	}
	manager, err := integrationsManager()
	if err != nil {
		return err
	}
	ident, err := identity.Load(manager.HomeDir)
	if err != nil {
		return fmt.Errorf("list: identidade invalida: %w", err)
	}
	listAgentModels, listWarnMsg := config.ResolveAgentModels(opts.scope, manager.HomeDir, manager.ProjectRoot)
	if listWarnMsg != "" {
		fmt.Fprintln(os.Stderr, listWarnMsg)
	}
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind: kind, Targets: opts.targets, Items: opts.items, Scope: opts.scope,
		Surfaces: surfaceMap, AllSurfaces: true, Identity: ident,
		ProjectRoot: manager.ProjectRoot,
		AgentModels: listAgentModels,
	})
	if err != nil {
		return err
	}
	if opts.json {
		return printLifecycleOutput(cmd, catalog, kind, plans, manager)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Available %s (catalog %s):\n", kind, catalog.Version)
	for _, item := range catalog.Items(kind) {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-14s %s — %s\n", item.ID, item.Name, item.Description)
	}
	inspections, err := manager.List(plans)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nDeployments:")
	for index, plan := range plans {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %-12s %-14s %-13s %s\n", plan.Claim.Target, plan.Claim.Surface, plan.Claim.Item, inspections[index].State, plan.Destination)
	}
	return nil
}

func printLifecycleOutput(cmd *cobra.Command, catalog *integrations.Catalog, kind integrations.ItemKind, plans []integrations.PlannedArtifact, manager integrations.Manager) error {
	inspections, err := manager.List(plans)
	if err != nil {
		return err
	}
	output := lifecycleOutput{Kind: kind, CatalogVersion: catalog.Version}
	for _, item := range catalog.Items(kind) {
		output.Items = append(output.Items, itemOutput{ID: item.ID, Name: item.Name, Description: item.Description})
	}
	for index, plan := range plans {
		target, _ := catalog.Target(plan.Claim.Target)
		surface, capability := surfaceCapability(target, plan.Claim.Surface, kind)
		output.Deployments = append(output.Deployments, deploymentOutput{
			Target: plan.Claim.Target, Surface: surface.ID, Scope: plan.Claim.Scope, Item: plan.Claim.Item,
			SupportLevel: plan.SupportLevel, Representation: capability.Representation,
			Destination: plan.Destination, State: inspections[index].State, Managed: inspections[index].Managed,
		})
	}
	sort.Slice(output.Deployments, func(i, j int) bool {
		a, b := output.Deployments[i], output.Deployments[j]
		return a.Target < b.Target || a.Target == b.Target && (a.Surface < b.Surface || a.Surface == b.Surface && a.Item < b.Item)
	})
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func integrationsManager() (integrations.Manager, error) {
	project, err := os.Getwd()
	if err != nil {
		return integrations.Manager{}, err
	}
	home, err := homedir.Dir()
	if err != nil {
		return integrations.Manager{}, err
	}
	return integrations.Manager{ProjectRoot: project, HomeDir: home}, nil
}

func parseSurfaceFlags(values []string) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		target, surface, ok := strings.Cut(value, "=")
		if !ok || target == "" || surface == "" {
			return nil, fmt.Errorf("invalid --surface %q: expected target=surface", value)
		}
		if _, duplicate := result[target]; duplicate {
			return nil, fmt.Errorf("duplicate --surface for target %s", target)
		}
		result[target] = surface
	}
	return result, nil
}

// promptInstallScopeRunner is the indirection callers use to invoke the
// install-scope prompt, mirroring the identityWizardRunner package-var
// pattern already used in this package. Tests that need to prove a caller
// *would* invoke the prompt (without actually blocking on a real huh.Form,
// which requires a terminal) substitute this var instead of calling
// promptInstallScope directly.
var promptInstallScopeRunner = promptInstallScope

// installScopeSelect builds the huh.Select field used by promptInstallScope,
// extracted so tests can exercise the real pre-selected default via
// Select.RunAccessible (no TTY required) instead of stubbing
// promptInstallScopeRunner — see TestInstallScopeSelectDefaultsToGlobal in
// agents_skills_test.go. Options() runs before Value(scope) binds the
// accessor, so huh re-syncs the cursor to *scope's initial value ("global")
// when Value is called (see huh's Accessor/selectValue).
func installScopeSelect(scope *string) *huh.Select[string] {
	return huh.NewSelect[string]().
		Title("Onde instalar os artefatos?").
		Options(
			huh.NewOption("Pasta do usuário (~/.claude) — vale para todos os projetos", "global"),
			huh.NewOption("Este projeto (.claude) — apenas neste repositório", "project"),
		).
		Value(scope)
}

// promptInstallScope asks the user where trackfw should install agents and
// skills artifacts. It is shared between `agents|skills install|update|
// uninstall` (via resolveScope) and `trackfw init`'s wizard so both surfaces
// present the exact same options and wording (ADR D2, D4).
func promptInstallScope() (string, error) {
	scope := "global"
	err := huh.NewForm(huh.NewGroup(installScopeSelect(&scope))).Run()
	return scope, err
}

// resolveScope decides opts.scope for `agents|skills install|update|
// uninstall`, as a gate independent from target/item selection (ADR D2): it
// must run — and may prompt — even when --targets was already supplied,
// which is the most common invocation shape.
//
// Precedence (ADR D3): an explicit --scope (detected via cmd.Flags().
// Changed, never by comparing opts.scope's value against "project" — that
// comparison cannot distinguish an explicit `--scope project` from the flag's
// unset default) or a deprecated-alias-assigned scope (opts.scopeExplicit)
// always wins and is only validated, never re-prompted. Otherwise: no TTY →
// "global" (D1) for install/update; for uninstall, no TTY and no explicit
// --scope is a hard error (ADR D8) — a destructive operation must never
// silently default to a scope the caller didn't choose, since a CI script
// removing `.claude/agents/trackfw-*.md` from the repo would otherwise start
// deleting files from the user's home directory instead. TTY → interactive
// prompt with "global" pre-selected (D2), for every operation including
// uninstall.
func resolveScope(cmd *cobra.Command, opts *integrationOptions, operation string) error {
	if opts.scopeExplicit || cmd.Flags().Changed("scope") {
		if opts.scope != "project" && opts.scope != "global" {
			return fmt.Errorf("invalid --scope %q: use project or global", opts.scope)
		}
		return nil
	}
	if !integrationsStdinIsTTY() {
		if operation == "uninstall" {
			return fmt.Errorf("uninstall requires --scope in non-interactive mode")
		}
		opts.scope = "global"
		return nil
	}
	scope, err := promptInstallScopeRunner()
	if err != nil {
		return err
	}
	opts.scope = scope
	return nil
}

func promptIntegrationSelection(catalog *integrations.Catalog, kind integrations.ItemKind, opts *integrationOptions) error {
	targetOptions := make([]huh.Option[string], 0, len(catalog.Targets))
	for _, target := range catalog.Targets {
		targetOptions = append(targetOptions, huh.NewOption(target.Name, target.ID))
	}
	itemOptions := make([]huh.Option[string], 0, len(catalog.Items(kind)))
	for _, item := range catalog.Items(kind) {
		itemOptions = append(itemOptions, huh.NewOption(item.Name, item.ID))
	}
	return huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("Target CLIs").Options(targetOptions...).Value(&opts.targets),
		huh.NewMultiSelect[string]().Title(fmt.Sprintf("%s to manage", strings.Title(string(kind)))).Options(itemOptions...).Value(&opts.items), //nolint:staticcheck
	)).Run()
}

func promptAmbiguousSurfaces(catalog *integrations.Catalog, kind integrations.ItemKind, targets []string, selected map[string]string) error {
	for _, targetID := range targets {
		if selected[targetID] != "" {
			continue
		}
		target, ok := catalog.Target(targetID)
		if !ok {
			continue
		}
		var options []huh.Option[string]
		for _, surface := range target.Surfaces {
			_, capability := surfaceCapability(target, surface.ID, kind)
			if capability.SupportLevel != "legacy" && capability.SupportLevel != "unsupported" {
				options = append(options, huh.NewOption(surface.Name, surface.ID))
			}
		}
		if len(options) > 1 {
			var value string
			if err := huh.NewSelect[string]().Title("Surface for " + target.Name).Options(options...).Value(&value).Run(); err != nil {
				return err
			}
			selected[targetID] = value
		}
	}
	return nil
}

func surfaceCapability(target integrations.Target, surfaceID string, kind integrations.ItemKind) (integrations.Surface, integrations.Capability) {
	for _, surface := range target.Surfaces {
		if surface.ID == surfaceID {
			if kind == integrations.KindSkills {
				return surface, surface.Capabilities.Skills
			}
			return surface, surface.Capabilities.Agents
		}
	}
	return integrations.Surface{}, integrations.Capability{}
}
