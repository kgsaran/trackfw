package commands

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// identityWizardRunner is the indirection callers use to invoke the wizard,
// mirroring the integrationsStdinIsTTY package-var pattern already used in
// this package. Tests that need to prove a caller *would* invoke the wizard
// (without actually blocking on a real huh.Form, which requires a terminal)
// substitute this var instead of calling runIdentityWizard directly.
var identityWizardRunner = runIdentityWizard

// runIdentityWizard is the single interactive identity wizard consumed by
// both `trackfw init` and `trackfw agents install`
// (ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install, D1).
//
// It shows, in order: preset/custom selection, free-text names (custom mode
// only), the user nickname, and a confirmation screen (D3, D6). If the user
// declines the confirmation, the wizard loops back to preset selection
// without persisting anything — it never returns having written a partial
// or rejected config to disk.
//
// The returned bool reports whether cfg was persisted via identity.Save.
// "neutral" (or leaving the hidden-group zero value untouched) is the
// "write nothing" path and returns (zero Config, false, nil). A non-nil
// error means the wizard itself failed (e.g. the huh form was aborted); the
// caller does not need to do any additional cleanup in that case either.
func runIdentityWizard(catalog *integrations.Catalog, home string) (identity.Config, bool, error) {
	knownAgentIDs := identity.KnownAgentIDs()

	titleIdentityPreset := i18n.T("init.prompt.identityPreset")
	titleIdentityCustomName := i18n.T("init.prompt.identityCustomName")
	titleIdentityNickname := i18n.T("init.prompt.identityNickname")

	for {
		var (
			identitySelect string
			userNickname   string
		)
		customDisplayNames := make([]string, len(knownAgentIDs))

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(titleIdentityPreset).
					Options(identityPresetOptions()...).
					Value(&identitySelect),
			),
			buildCustomIdentityGroup(catalog, titleIdentityCustomName, knownAgentIDs, customDisplayNames, func() bool {
				return identitySelect != "custom"
			}),
			huh.NewGroup(
				huh.NewInput().
					Title(titleIdentityNickname).
					Value(&userNickname),
			).WithHideFunc(func() bool {
				return identitySelect == "" || identitySelect == "neutral"
			}),
		)
		if err := form.Run(); err != nil {
			return identity.Config{}, false, err
		}

		cfg, outcome, err := resolveIdentitySelection(home, identitySelect, knownAgentIDs, customDisplayNames, userNickname,
			func(candidate identity.Config) (bool, error) {
				return confirmIdentitySelection(catalog, knownAgentIDs, candidate)
			})
		if err != nil {
			return identity.Config{}, false, err
		}
		switch outcome {
		case identitySkipped:
			return identity.Config{}, false, nil
		case identityDeclined:
			// D3: declining returns to preset selection without persisting
			// anything at all.
			continue
		default:
			return cfg, true, nil
		}
	}
}

// identityOutcome distinguishes the three terminal states of a single wizard
// round. A plain bool cannot: "neutral" (write nothing and finish) and
// "user declined the confirmation" (write nothing and re-ask) are both
// "nothing was saved", but the caller must return in the first case and loop
// in the second.
type identityOutcome int

const (
	// identitySkipped: "neutral" or the hidden-group zero value — the wizard
	// writes nothing at all and the caller is done.
	identitySkipped identityOutcome = iota
	// identityDeclined: the config was built and validated, but the user
	// rejected the confirmation screen; nothing was written.
	identityDeclined
	// identitySaved: the config was validated, confirmed and persisted.
	identitySaved
)

// resolveIdentitySelection is the entire non-interactive half of the wizard:
// the "write nothing" predicate, the Config construction, validation,
// confirmation and persistence — in that exact order. runIdentityWizard is
// its only production caller, so this sequence lives in exactly one place and
// tests exercising it are testing the shipped path.
//
// The confirm callback is the sole interactive step (rendered by
// confirmIdentitySelection in production); it is invoked only after
// identity.Validate has already accepted cfg, and identity.Save runs only
// after confirm returns true. That ordering is the whole point of the
// extraction: a slug collision must abort before anything reaches disk.
func resolveIdentitySelection(home, identitySelect string, knownAgentIDs []string, customDisplayNames []string, userNickname string, confirm func(identity.Config) (bool, error)) (identity.Config, identityOutcome, error) {
	if identitySelect == "" || identitySelect == "neutral" {
		return identity.Config{}, identitySkipped, nil
	}

	cfg, err := buildIdentityConfig(identitySelect, knownAgentIDs, customDisplayNames, userNickname)
	if err != nil {
		return identity.Config{}, identitySkipped, fmt.Errorf("identidade invalida: %w", err)
	}
	if err := identity.Validate(cfg, knownAgentIDs); err != nil {
		return identity.Config{}, identitySkipped, fmt.Errorf("identidade invalida: %w", err)
	}

	confirmed, err := confirm(cfg)
	if err != nil {
		return identity.Config{}, identitySkipped, err
	}
	if !confirmed {
		return identity.Config{}, identityDeclined, nil
	}

	if err := identity.Save(home, cfg); err != nil {
		return identity.Config{}, identitySkipped, fmt.Errorf("falha ao gravar identidade: %w", err)
	}
	return cfg, identitySaved, nil
}

// buildIdentityConfig builds the in-memory Config for the chosen wizard
// path, without touching disk, so the Config shape is built from the wizard
// inputs in exactly one place.
func buildIdentityConfig(identitySelect string, knownAgentIDs []string, customDisplayNames []string, userNickname string) (identity.Config, error) {
	var cfg identity.Config
	if identitySelect == "custom" {
		cfg = identity.Config{Agents: make(map[string]identity.AgentIdentity, len(knownAgentIDs))}
		for index, id := range knownAgentIDs {
			slug, err := identity.Slugify(customDisplayNames[index])
			if err != nil {
				return identity.Config{}, fmt.Errorf("identidade customizada invalida para %q: %w", id, err)
			}
			cfg.Agents[id] = identity.AgentIdentity{DisplayName: customDisplayNames[index], Slug: slug}
		}
	} else {
		preset, err := identity.Preset(identitySelect)
		if err != nil {
			return identity.Config{}, err
		}
		cfg = preset
	}
	cfg.UserNickname = userNickname
	return cfg, nil
}

// identityPresetOptions is the fixed list of themed identity presets offered
// by the wizard, shared verbatim between init and agents install so neither
// consumer can drift from the other.
func identityPresetOptions() []huh.Option[string] {
	return []huh.Option[string]{
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
	}
}

// buildCustomIdentityGroup builds the huh group for the "Personalizar um a
// um" identity path: one input per known agent id, each labeled with the
// agent's specialty — Item.Name and Item.Description from the catalog — and
// never with the raw technical id (ADR D4). helperText is shown as
// secondary Description text under the specialty label.
//
// Every id in ids is expected to exist in the agents catalog (identity's
// KnownAgentIDs and the embedded catalog are meant to be kept in lockstep);
// if one is missing, that is a programming error and buildCustomIdentityGroup
// fails loudly via panic rather than silently mislabeling a field.
func buildCustomIdentityGroup(catalog *integrations.Catalog, helperText string, ids []string, values []string, hide func() bool) *huh.Group {
	fields := make([]huh.Field, 0, len(ids))
	for index, id := range ids {
		index := index
		id := id
		item, ok := catalog.Item(integrations.KindAgents, id)
		if !ok {
			panic(fmt.Sprintf("identity wizard: agent id %q from identity.KnownAgentIDs() has no entry in the agents catalog", id))
		}
		label := fmt.Sprintf("%s — %s", item.Name, item.Description)
		fields = append(fields, huh.NewInput().
			Title(label).
			Description(helperText).
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

// confirmIdentitySelection renders the specialty → display name mapping
// (ADR D3) and asks for confirmation via huh.NewConfirm. The mapping itself
// is printed with plain fmt (not through a huh field) so its column
// alignment matches the ADR's example layout exactly.
func confirmIdentitySelection(catalog *integrations.Catalog, knownAgentIDs []string, cfg identity.Config) (bool, error) {
	nicknameLabel := i18n.T("identity.wizard.nicknameRowLabel")
	labelWidth := len(nicknameLabel)
	labels := make([]string, len(knownAgentIDs))
	for i, id := range knownAgentIDs {
		item, ok := catalog.Item(integrations.KindAgents, id)
		if !ok {
			panic(fmt.Sprintf("identity wizard: agent id %q from identity.KnownAgentIDs() has no entry in the agents catalog", id))
		}
		labels[i] = item.Description
		if len(labels[i]) > labelWidth {
			labelWidth = len(labels[i])
		}
	}

	fmt.Println()
	fmt.Println(i18n.T("identity.wizard.confirmHeader"))
	for i, id := range knownAgentIDs {
		agent := cfg.Agents[id]
		fmt.Printf("  %-*s   →  %s\n", labelWidth, labels[i], agent.DisplayName)
	}
	if cfg.UserNickname != "" {
		fmt.Printf("  %-*s   %s\n", labelWidth, nicknameLabel, cfg.UserNickname)
	}
	fmt.Println()

	var confirmed bool
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(i18n.T("identity.wizard.confirmQuestion")).
			Value(&confirmed),
	)).Run()
	if err != nil {
		return false, err
	}
	return confirmed, nil
}

// shouldPromptIdentity decides whether `trackfw agents install` (and the
// update/uninstall operations, which share executeIntegrationMutation)
// should show the interactive identity wizard. All conditions must hold
// simultaneously (ADR D2):
//
//  1. kind is agents — skills have no identity and must never prompt (D5).
//  2. stdin is a TTY — never block a non-interactive run.
//  3. either no identity is configured yet, or the user explicitly asked to
//     reconfigure it via --identity.
//
// With an identity already configured and no --identity flag, the wizard
// must NOT reappear — a wizard that shows up on every install becomes an
// incentive to script around it, which quietly kills the feature.
func shouldPromptIdentity(kind integrations.ItemKind, isTTY bool, identityExists bool, forceFlag bool) bool {
	if kind != integrations.KindAgents {
		return false
	}
	if !isTTY {
		return false
	}
	return !identityExists || forceFlag
}
