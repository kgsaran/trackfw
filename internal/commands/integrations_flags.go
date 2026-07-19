package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

type integrationOptions struct {
	targets  []string
	items    []string
	scope    string
	surfaces []string
	json     bool
	force    bool
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
	)
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
	addIntegrationFlags(cmd, &opts, false)
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
	addIntegrationFlags(cmd, &opts, true)
	return cmd
}

func addIntegrationFlags(cmd *cobra.Command, opts *integrationOptions, mutation bool) {
	cmd.Flags().StringSliceVar(&opts.targets, "targets", nil, "target CLIs (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.items, "items", nil, "catalog items (comma-separated; default: all)")
	cmd.Flags().StringVar(&opts.scope, "scope", "project", "installation scope: project or global")
	cmd.Flags().StringArrayVar(&opts.surfaces, "surface", nil, "target=surface selection (repeatable)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "print canonical JSON output")
	if mutation {
		cmd.Flags().BoolVar(&opts.force, "force", false, "replace or remove modified managed artifacts")
	}
}

func executeIntegrationMutation(cmd *cobra.Command, kind integrations.ItemKind, operation string, opts *integrationOptions) error {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}
	if opts.scope != "project" && opts.scope != "global" {
		return fmt.Errorf("invalid --scope %q: use project or global", opts.scope)
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
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind: kind, Targets: opts.targets, Items: opts.items, Scope: opts.scope, Surfaces: surfaceMap,
	})
	if err != nil {
		return err
	}
	manager, err := integrationsManager()
	if err != nil {
		return err
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
	if opts.json {
		return printLifecycleOutput(cmd, catalog, kind, plans, manager)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s complete: %d %s artifact(s)\n", operation, len(plans), kind)
	return nil
}

func executeIntegrationList(cmd *cobra.Command, kind integrations.ItemKind, opts integrationOptions) error {
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
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind: kind, Targets: opts.targets, Items: opts.items, Scope: opts.scope,
		Surfaces: surfaceMap, AllSurfaces: true,
	})
	if err != nil {
		return err
	}
	manager, err := integrationsManager()
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
	home, err := os.UserHomeDir()
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

func runDeprecatedIntegrationAlias(cmd *cobra.Command, target string, scopes []string) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "warning: trackfw %s is deprecated; use trackfw agents|skills install --targets %s\n", target, target)
	for _, scope := range scopes {
		for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
			opts := integrationOptions{targets: []string{target}, scope: scope}
			if err := executeIntegrationMutation(cmd, kind, "install", &opts); err != nil {
				return err
			}
		}
	}
	return nil
}
