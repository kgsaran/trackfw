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

func TestInitIdentityPresetGreekWritesTenAgents(t *testing.T) {
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
	if len(cfg.Agents) != 10 {
		t.Fatalf("expected 10 configured agents, got %d: %+v", len(cfg.Agents), cfg.Agents)
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
