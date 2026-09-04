package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/identity"
)

// initFixture sets up an isolated HOME and project cwd for `trackfw init`
// tests, mirroring integrationCommandFixture. go test's stdin is never a
// TTY, so every runInit invocation here exercises the non-interactive
// branch — which is exactly the branch --identity-preset must work through
// without blocking on any prompt.
func initFixture(t *testing.T) (project, home string) {
	t.Helper()
	project = t.TempDir()
	home = t.TempDir()
	oldHome := os.Getenv("HOME")
	oldWD, _ := os.Getwd()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		_ = os.Setenv("HOME", oldHome)
	})
	return project, home
}

func identityJSONPath(home string) string {
	return filepath.Join(home, ".trackfw", "identity.json")
}

func TestInitIdentityPresetInvalidValueListsValidOnes(t *testing.T) {
	_, home := initFixture(t)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--identity-preset", "not-a-real-preset"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --identity-preset value")
	}
	for _, want := range []string{"none", "neutral", "greek", "norse", "egyptian"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must list valid preset %q, got: %v", want, err)
		}
	}
	if _, statErr := os.Stat(identityJSONPath(home)); !os.IsNotExist(statErr) {
		t.Fatalf("invalid preset must not write identity.json: %v", statErr)
	}
}

func TestInitIdentityPresetGreekWritesTwelveAgents(t *testing.T) {
	_, home := initFixture(t)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--identity-preset", "greek"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := identity.Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Agents) != 12 {
		t.Fatalf("expected 12 configured agents, got %d: %+v", len(cfg.Agents), cfg.Agents)
	}
	agent, ok := cfg.Agents["architect"]
	if !ok || agent.Slug != "zeus" {
		t.Fatalf("expected architect to be zeus, got %+v (ok=%v)", agent, ok)
	}
}

func TestInitIdentityPresetNeutralAndNoneWriteNothing(t *testing.T) {
	for _, value := range []string{"neutral", "none"} {
		t.Run(value, func(t *testing.T) {
			_, home := initFixture(t)

			cmd := newInitCmd()
			cmd.SetArgs([]string{"--identity-preset", value})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if _, statErr := os.Stat(identityJSONPath(home)); !os.IsNotExist(statErr) {
				t.Fatalf("--identity-preset %s must not write identity.json: %v", value, statErr)
			}
		})
	}
}

func TestInitNonTTYHonorsIdentityPresetFlagWithoutBlocking(t *testing.T) {
	_, home := initFixture(t)

	done := make(chan error, 1)
	go func() {
		cmd := newInitCmd()
		cmd.SetArgs([]string{"--identity-preset", "greek"})
		done <- cmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("init blocked on a prompt in non-TTY mode")
	}

	if _, statErr := os.Stat(identityJSONPath(home)); statErr != nil {
		t.Fatalf("expected identity.json to be written: %v", statErr)
	}
}

func TestInitRerunWithoutFlagPreservesExistingIdentity(t *testing.T) {
	_, home := initFixture(t)

	first := newInitCmd()
	first.SetArgs([]string{"--identity-preset", "greek"})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(identityJSONPath(home))
	if err != nil {
		t.Fatal(err)
	}

	// Re-run without the flag: must reuse the existing identity file
	// byte-for-byte, not re-prompt (impossible outside a TTY anyway) and
	// not silently reset it to neutral.
	second := newInitCmd()
	second.SetArgs(nil)
	if err := second.Execute(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(identityJSONPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("re-running init without --identity-preset must preserve the existing identity file.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// acceptIdentity is the confirm callback for tests that exercise the
// confirmed path of resolveIdentitySelection without a real huh.Form.
func acceptIdentity(identity.Config) (bool, error) { return true, nil }

// TestResolveIdentitySelectionCustomWritesValidatedConfig covers the
// "Personalizar um a um" wizard path against the live code that
// runIdentityWizard itself calls: resolveIdentitySelection must write a
// Config with one AgentIdentity per known agent id, each Slug matching
// identity.Slugify of the entered display name, plus the nickname.
func TestResolveIdentitySelectionCustomWritesValidatedConfig(t *testing.T) {
	home := t.TempDir()
	ids := identity.KnownAgentIDs()
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = "Agente " + id
	}

	_, outcome, err := resolveIdentitySelection(home, "custom", ids, names, "Kleber", acceptIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != identitySaved {
		t.Fatalf("expected identitySaved, got %v", outcome)
	}

	cfg, err := identity.Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UserNickname != "Kleber" {
		t.Fatalf("expected nickname to be persisted, got %q", cfg.UserNickname)
	}
	if len(cfg.Agents) != len(ids) {
		t.Fatalf("expected %d agents, got %d: %+v", len(ids), len(cfg.Agents), cfg.Agents)
	}
	for i, id := range ids {
		agent, ok := cfg.Agents[id]
		if !ok {
			t.Fatalf("missing agent %q in saved config", id)
		}
		wantSlug, err := identity.Slugify(names[i])
		if err != nil {
			t.Fatal(err)
		}
		if agent.Slug != wantSlug || agent.DisplayName != names[i] {
			t.Fatalf("agent %q mismatch: got %+v, want slug=%q display=%q", id, agent, wantSlug, names[i])
		}
	}
}

// TestResolveIdentitySelectionNeutralWritesNothing covers the "neutral"
// wizard choice and the hidden-group zero value ("") — both mean "do not
// write". The confirm callback must never even be reached.
func TestResolveIdentitySelectionNeutralWritesNothing(t *testing.T) {
	ids := identity.KnownAgentIDs()
	for _, selection := range []string{"neutral", ""} {
		t.Run(selection, func(t *testing.T) {
			home := t.TempDir()
			_, outcome, err := resolveIdentitySelection(home, selection, ids, make([]string, len(ids)), "",
				func(identity.Config) (bool, error) {
					t.Fatalf("selection %q must not reach the confirmation step", selection)
					return false, nil
				})
			if err != nil {
				t.Fatal(err)
			}
			if outcome != identitySkipped {
				t.Fatalf("selection %q: expected identitySkipped, got %v", selection, outcome)
			}
			if _, statErr := os.Stat(identityJSONPath(home)); !os.IsNotExist(statErr) {
				t.Fatalf("selection %q must not write identity.json: %v", selection, statErr)
			}
		})
	}
}

// TestResolveIdentitySelectionCustomCollisionWritesNothing proves the
// ordering the live wizard relies on: identity.Validate must run BEFORE
// identity.Save, so a slug collision between two custom display names
// aborts with an error and leaves no file behind — never a corrupt or
// partially-written identity.json.
func TestResolveIdentitySelectionCustomCollisionWritesNothing(t *testing.T) {
	home := t.TempDir()
	ids := identity.KnownAgentIDs()
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = "Agente " + id
	}
	// Force a slug collision: two different display names that fold to the
	// same slug via identity.Slugify ("Zeus" and "zeus" both -> "zeus").
	names[0] = "Zeus"
	names[1] = "zeus"

	_, _, err := resolveIdentitySelection(home, "custom", ids, names, "", acceptIdentity)
	if err == nil {
		t.Fatal("expected an error for colliding slugs")
	}
	if _, statErr := os.Stat(identityJSONPath(home)); !os.IsNotExist(statErr) {
		t.Fatalf("colliding custom identity must not write identity.json: %v", statErr)
	}
}

// TestInitForgeFlag_Valid verifies that --forge gitlab in non-TTY mode writes
// "forge: gitlab" in the generated trackfw.yaml.
func TestInitForgeFlag_Valid(t *testing.T) {
	project, _ := initFixture(t)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--forge", "gitlab"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for valid --forge, got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(project, "trackfw.yaml"))
	if err != nil {
		t.Fatalf("trackfw.yaml not created: %v", err)
	}
	if !strings.Contains(string(content), "forge: gitlab") {
		t.Errorf("expected 'forge: gitlab' in trackfw.yaml, got:\n%s", string(content))
	}
}

// TestInitGeneratesCredentialGuardScript verifies that `trackfw init` (the real
// command flow, not a direct call to the generator) writes
// scripts/trackfw-credential-guard.sh and that it is executable — regression test
// for the bug where GenerateCredentialGuardScript existed but was never called by
// any real init/update/discover flow, only by tests calling it directly.
func TestInitGeneratesCredentialGuardScript(t *testing.T) {
	project, _ := initFixture(t)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--forge", "github"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	guardPath := filepath.Join(project, "scripts", "trackfw-credential-guard.sh")
	info, err := os.Stat(guardPath)
	if err != nil {
		t.Fatalf("scripts/trackfw-credential-guard.sh not created by trackfw init: %v", err)
	}
	if execBitRepresentavelPara(t, guardPath) {
		if info.Mode()&0o111 == 0 {
			t.Errorf("scripts/trackfw-credential-guard.sh should be executable, mode=%v", info.Mode())
		}
	} else {
		execBitNaoExercitado(t, guardPath)
	}

	signalPath := filepath.Join(project, "scripts", "trackfw-attention-signal.sh")
	if _, err := os.Stat(signalPath); err != nil {
		t.Fatalf("scripts/trackfw-attention-signal.sh not created by trackfw init: %v", err)
	}
}

// TestInitForgeFlag_Invalid verifies that --forge with an unknown value returns
// an error that lists all accepted forge values.
func TestInitForgeFlag_Invalid(t *testing.T) {
	_, _ = initFixture(t)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--forge", "notaforge"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --forge value")
	}
	for _, want := range []string{"github", "gitlab", "bitbucket", "azure"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must list valid forge %q, got: %v", want, err)
		}
	}
}
