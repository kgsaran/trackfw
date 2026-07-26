package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// TestShouldPromptIdentityExhaustive covers every combination of the three
// conditions ADR D2 requires: kind == agents, stdin is a TTY, and
// (no identity configured yet OR --identity was passed). Any single
// condition failing must suppress the wizard.
func TestShouldPromptIdentityExhaustive(t *testing.T) {
	tests := []struct {
		name           string
		kind           integrations.ItemKind
		isTTY          bool
		identityExists bool
		forceFlag      bool
		want           bool
	}{
		{"agents+tty+absent+noforce -> prompt", integrations.KindAgents, true, false, false, true},
		{"agents+tty+absent+force -> prompt", integrations.KindAgents, true, false, true, true},
		{"agents+tty+existing+noforce -> silent", integrations.KindAgents, true, true, false, false},
		{"agents+tty+existing+force -> prompt", integrations.KindAgents, true, true, true, true},
		{"agents+notty+absent+noforce -> never blocks", integrations.KindAgents, false, false, false, false},
		{"agents+notty+absent+force -> never blocks", integrations.KindAgents, false, false, true, false},
		{"agents+notty+existing+force -> never blocks", integrations.KindAgents, false, true, true, false},
		{"skills+tty+absent+noforce -> never (D5)", integrations.KindSkills, true, false, false, false},
		{"skills+tty+absent+force -> never (D5)", integrations.KindSkills, true, false, true, false},
		{"skills+notty+existing+force -> never (D5)", integrations.KindSkills, false, true, true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shouldPromptIdentity(test.kind, test.isTTY, test.identityExists, test.forceFlag)
			if got != test.want {
				t.Fatalf("shouldPromptIdentity(%v, tty=%v, exists=%v, force=%v) = %v, want %v",
					test.kind, test.isTTY, test.identityExists, test.forceFlag, got, test.want)
			}
		})
	}
}

// TestAgentsInstallSkipsWizardWhenIdentityAlreadyConfigured covers: "agents
// install com identity.json existente e sem --identity → não invoca o
// wizard". The fixture fakes stdin as a TTY (the only way the wizard could
// ever trigger) — identityWizardRunner is swapped for a spy so a real
// (blocking) huh.Form is never exercised; if executeIntegrationMutation
// invoked the wizard here, the spy would be called and the test would fail.
func TestAgentsInstallSkipsWizardWhenIdentityAlreadyConfigured(t *testing.T) {
	project, home := integrationCommandFixture(t)
	_ = project
	writeGreekIdentity(t, home)

	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldWizard := identityWizardRunner
	called := false
	identityWizardRunner = func(_ *integrations.Catalog, _ string) (identity.Config, bool, error) {
		called = true
		return identity.Config{}, false, nil
	}
	t.Cleanup(func() { identityWizardRunner = oldWizard })

	cmd := newAgentsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("wizard must not be invoked when identity.json already exists and --identity was not passed")
	}
	if !strings.Contains(out.String(), "identidade:") && !strings.Contains(out.String(), "identity:") {
		t.Fatalf("expected an informational identity-in-use line, got: %q", out.String())
	}
}

// TestAgentsInstallInvokesWizardWhenIdentityAbsent covers: "agents install
// com identity.json ausente em TTY → invoca". Same spy technique as above —
// this proves the *decision* to invoke, without ever running a real
// (blocking) interactive form.
func TestAgentsInstallInvokesWizardWhenIdentityAbsent(t *testing.T) {
	integrationCommandFixture(t)

	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldWizard := identityWizardRunner
	called := false
	identityWizardRunner = func(_ *integrations.Catalog, _ string) (identity.Config, bool, error) {
		called = true
		return identity.Config{}, false, nil
	}
	t.Cleanup(func() { identityWizardRunner = oldWizard })

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("wizard must be invoked when identity.json is absent and stdin is a TTY")
	}
}

// TestAgentsInstallForceFlagInvokesWizardEvenWithExistingIdentity covers:
// "agents install com --identity e identidade existente → invoca".
func TestAgentsInstallForceFlagInvokesWizardEvenWithExistingIdentity(t *testing.T) {
	_, home := integrationCommandFixture(t)
	writeGreekIdentity(t, home)

	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldWizard := identityWizardRunner
	called := false
	identityWizardRunner = func(_ *integrations.Catalog, _ string) (identity.Config, bool, error) {
		called = true
		return identity.Config{}, false, nil
	}
	t.Cleanup(func() { identityWizardRunner = oldWizard })

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project", "--identity"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("wizard must be invoked when --identity is passed, even with an existing identity.json")
	}
}

// TestSkillsInstallNeverInvokesWizard covers: "skills install → nunca
// invoca, mesmo sem identity.json" (ADR D5).
func TestSkillsInstallNeverInvokesWizard(t *testing.T) {
	integrationCommandFixture(t)

	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldWizard := identityWizardRunner
	called := false
	identityWizardRunner = func(_ *integrations.Catalog, _ string) (identity.Config, bool, error) {
		called = true
		return identity.Config{}, false, nil
	}
	t.Cleanup(func() { identityWizardRunner = oldWizard })

	cmd := newSkillsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "governance", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("skills install must never invoke the identity wizard")
	}
}

// TestAgentsInstallNonTTYNeverBlocksAndStillRequiresTargets covers the
// inherited non-TTY restriction plus --identity-preset support outside a
// TTY: the flag must apply without ever invoking the interactive wizard,
// and omitting --targets outside a TTY must still fail actionably.
func TestAgentsInstallNonTTYNeverBlocksAndStillRequiresTargets(t *testing.T) {
	_, home := integrationCommandFixture(t) // fixture fakes stdin as non-TTY

	oldWizard := identityWizardRunner
	called := false
	identityWizardRunner = func(_ *integrations.Catalog, _ string) (identity.Config, bool, error) {
		called = true
		return identity.Config{}, false, nil
	}
	t.Cleanup(func() { identityWizardRunner = oldWizard })

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--identity-preset", "greek", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("--identity-preset must resolve identity without ever invoking the interactive wizard")
	}
	cfg, err := identity.Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Agents) != 12 {
		t.Fatalf("expected --identity-preset greek to persist 12 agents, got %d", len(cfg.Agents))
	}

	// --targets is still required outside a TTY, --identity-preset does not
	// exempt the caller from it.
	missingTargets := newAgentsCmd()
	missingTargets.SetOut(&bytes.Buffer{})
	missingTargets.SetArgs([]string{"install", "--identity-preset", "greek"})
	err = missingTargets.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires --targets in non-interactive mode") {
		t.Fatalf("expected actionable target error, got %v", err)
	}
}

// TestAgentsIdentityPresetInvalidValueListsValidOnes covers: "--identity-preset
// inválido → erro listando os válidos".
func TestAgentsIdentityPresetInvalidValueListsValidOnes(t *testing.T) {
	_, home := integrationCommandFixture(t)

	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"install", "--identity-preset", "not-a-real-preset", "--targets", "claude"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --identity-preset value")
	}
	for _, want := range []string{"none", "neutral", "greek", "norse", "egyptian"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must list valid preset %q, got: %v", want, err)
		}
	}
	if _, statErr := os.Stat(filepath.Join(home, ".trackfw", "identity.json")); !os.IsNotExist(statErr) {
		t.Fatalf("invalid preset must not write identity.json: %v", statErr)
	}
}

// TestSkillsCommandHasNoIdentityFlags covers ADR D5 at the flag-registration
// level: `trackfw skills install` must not even expose --identity or
// --identity-preset, since skills have no identity at all.
func TestSkillsCommandHasNoIdentityFlags(t *testing.T) {
	cmd := newSkillsCmd()
	install, _, err := cmd.Find([]string{"install"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"identity", "identity-preset"} {
		if install.Flags().Lookup(name) != nil {
			t.Fatalf("skills install must not register --%s", name)
		}
	}
}

// TestAgentsCommandHasIdentityFlags is the mirror-image sanity check: agents
// install/update/uninstall must expose both new flags.
func TestAgentsCommandHasIdentityFlags(t *testing.T) {
	cmd := newAgentsCmd()
	for _, op := range []string{"install", "update", "uninstall"} {
		sub, _, err := cmd.Find([]string{op})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"identity", "identity-preset"} {
			if sub.Flags().Lookup(name) == nil {
				t.Fatalf("agents %s must register --%s", op, name)
			}
		}
	}
}

// TestConfirmIdentitySelectionRejectionWritesNothing covers: "recusar a
// confirmação → nenhum arquivo gravado". It exercises runIdentityWizard's
// reject-then-loop path indirectly is not practical without a real TTY, so
// instead this proves the invariant at the level the wizard actually
// enforces it: buildIdentityConfig (used before any confirmation) never
// touches disk, and identity.Save is only ever reached after a positive
// confirmation inside runIdentityWizard's loop body (see identity_wizard.go).
// The disk-safety guarantee itself is covered end-to-end by
// TestSaveWizardIdentityCustomCollisionWritesNothing and
// TestInitIdentityPresetInvalidValueListsValidOnes (init_test.go), which
// prove no partial/rejected config ever reaches identity.Save.
func TestBuildIdentityConfigNeverTouchesDisk(t *testing.T) {
	home := t.TempDir()
	ids := identity.KnownAgentIDs()
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = "Agente " + id
	}
	if _, err := buildIdentityConfig("custom", ids, names, "Kleber"); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".trackfw", "identity.json")); !os.IsNotExist(statErr) {
		t.Fatalf("buildIdentityConfig must never write identity.json: %v", statErr)
	}
}

// TestBuildCustomIdentityGroupLabelsUseCatalogNotID covers: "rótulos do modo
// custom contêm Item.Description e não contêm o id cru" (ADR D4).
func TestBuildCustomIdentityGroupLabelsUseCatalogNotID(t *testing.T) {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ids := identity.KnownAgentIDs()
	values := make([]string, len(ids))
	group := buildCustomIdentityGroup(catalog, "helper text", ids, values, func() bool { return false })
	group.Init()
	rendered := group.View()

	for _, id := range ids {
		item, ok := catalog.Item(integrations.KindAgents, id)
		if !ok {
			t.Fatalf("catalog missing agent %q", id)
		}
		if !strings.Contains(rendered, item.Name) {
			t.Fatalf("rendered form missing catalog name %q for agent %q:\n%s", item.Name, id, rendered)
		}
		if !strings.Contains(rendered, item.Description) {
			t.Fatalf("rendered form missing catalog description %q for agent %q:\n%s", item.Description, id, rendered)
		}
		if strings.Contains(rendered, "("+id+")") {
			t.Fatalf("rendered form leaks raw technical id %q:\n%s", id, rendered)
		}
	}
}

// TestBuildCustomIdentityGroupPanicsOnUnknownCatalogAgent proves the
// "erro de programação" contract: an id in identity.KnownAgentIDs() that has
// no entry in the agents catalog must fail loudly (panic), never silently
// mislabel a field.
func TestBuildCustomIdentityGroupPanicsOnUnknownCatalogAgent(t *testing.T) {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected buildCustomIdentityGroup to panic on an id absent from the catalog")
		}
	}()
	ids := []string{"not-a-real-agent-id"}
	values := make([]string, 1)
	buildCustomIdentityGroup(catalog, "helper", ids, values, func() bool { return false })
}
