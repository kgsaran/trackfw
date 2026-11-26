package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/identity"
)

// initFixture sets up an isolated HOME and project cwd for `trackfw init`
// tests, mirroring integrationCommandFixture. go test's stdin is never a
// TTY, so every runInit invocation here exercises the non-interactive
// branch — which is exactly the branch --identity-preset must work through
// without blocking on any prompt.
//
// ML-2D: USERPROFILE is also isolated so that on Windows, where
// agentconfig.ClaudeConfigDir reads USERPROFILE (not HOME), the symlink
// placed at home/.claude is exactly the path the pre-check inspects.
func initFixture(t *testing.T) (project, home string) {
	t.Helper()
	project = t.TempDir()
	home = t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile, hadUserProfile := os.LookupEnv("USERPROFILE")
	oldWD, _ := os.Getwd()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("USERPROFILE", home); err != nil {
		t.Fatal(err)
	}
	// ML-4A4: isolate CODEX_HOME so global-Codex writes land in the temp dir,
	// not the developer's real ~/.codex (memory note: codex-home-env-isolation).
	oldCodexHome, hadCodexHome := os.LookupEnv("CODEX_HOME")
	if err := os.Unsetenv("CODEX_HOME"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		_ = os.Setenv("HOME", oldHome)
		if hadUserProfile {
			_ = os.Setenv("USERPROFILE", oldUserProfile)
		} else {
			_ = os.Unsetenv("USERPROFILE")
		}
		if hadCodexHome {
			_ = os.Setenv("CODEX_HOME", oldCodexHome)
		} else {
			_ = os.Unsetenv("CODEX_HOME")
		}
	})
	return project, home
}

// TestInitFixture_IsolatesUSERPROFILE affirms that after ML-2D initFixture
// sets both HOME and USERPROFILE to the temp home dir, and that both are
// restored to their original values after the sub-test's cleanup runs.
// Rule reconciliation: affirms the ML-2D conclusion that isolating
// USERPROFILE inside initFixture makes the symlink pre-check tests
// (TestInitScopeGlobalPreCheckAbortsOnSymlinkClaudeDir,
// TestInitScopeProjectHintPresentWhenGlobalScope,
// TestExecuteScaffoldAndInstall_PreCheckFiresOnSymlinkClaudeDir) point at
// the correct home directory on Windows.
func TestInitFixture_IsolatesUSERPROFILE(t *testing.T) {
	outerHome := os.Getenv("HOME")
	outerUserProfile, outerHadUserProfile := os.LookupEnv("USERPROFILE")

	t.Run("inside-fixture", func(inner *testing.T) {
		_, home := initFixture(inner)
		if got := os.Getenv("HOME"); got != home {
			inner.Errorf("HOME inside fixture = %q, want %q", got, home)
		}
		if got := os.Getenv("USERPROFILE"); got != home {
			inner.Errorf("USERPROFILE inside fixture = %q, want %q", got, home)
		}
	})
	// After t.Run returns, inner's t.Cleanup callbacks have run.
	if got := os.Getenv("HOME"); got != outerHome {
		t.Errorf("HOME after cleanup = %q, want outer %q", got, outerHome)
	}
	if outerHadUserProfile {
		if got := os.Getenv("USERPROFILE"); got != outerUserProfile {
			t.Errorf("USERPROFILE after cleanup = %q, want outer %q", got, outerUserProfile)
		}
	} else {
		if _, present := os.LookupEnv("USERPROFILE"); present {
			t.Errorf("USERPROFILE must be unset after cleanup, got %q", os.Getenv("USERPROFILE"))
		}
	}
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

// TestInitNonTTYPkgManagerDetectedFromPackageJSON asserts AC3 (#661): without TTY,
// init must NOT hardcode "npm"; it must detect the package manager from the project.
// A project with package.json gets "npm"; a project without it gets "" (empty).
//
// Rule reconciliation: test affirms that PkgManager is inferred from the filesystem,
// not hardcoded — in both the empty and npm cases.
func TestInitNonTTYPkgManagerDetectedFromPackageJSON(t *testing.T) {
	t.Run("no_package_json_gets_empty_pkg_manager", func(t *testing.T) {
		project, _ := initFixture(t)

		cmd := newInitCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(project, "trackfw.yaml"))
		if err != nil {
			t.Fatalf("trackfw.yaml not created: %v", err)
		}
		if strings.Contains(string(content), "pkg_manager: npm") {
			t.Errorf("project without package.json must not get pkg_manager: npm\ngot:\n%s", string(content))
		}
	})

	t.Run("with_package_json_gets_npm", func(t *testing.T) {
		project, _ := initFixture(t)

		// Create a package.json to signal a Node.js project.
		if err := os.WriteFile(filepath.Join(project, "package.json"), []byte(`{"name":"test"}`), 0644); err != nil {
			t.Fatalf("writing package.json: %v", err)
		}

		cmd := newInitCmd()
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(project, "trackfw.yaml"))
		if err != nil {
			t.Fatalf("trackfw.yaml not created: %v", err)
		}
		if !strings.Contains(string(content), "pkg_manager: npm") {
			t.Errorf("project with package.json must get pkg_manager: npm\ngot:\n%s", string(content))
		}
	})
}

// TestInitScopeInvalidValueRejectsBeforeAnyWrite asserts AC4 (#657), part 1:
// --scope with an invalid value returns an error before writing any project file.
//
// Rule reconciliation: test affirms that scope validation is the first gate in
// runInit — no trackfw.yaml or any other file is written on invalid scope.
func TestInitScopeInvalidValueRejectsBeforeAnyWrite(t *testing.T) {
	project, _ := initFixture(t)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--scope", "notascope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --scope value")
	}
	for _, want := range []string{"project", "global"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name accepted scope %q, got: %v", want, err)
		}
	}

	// No project file must have been written.
	if _, statErr := os.Stat(filepath.Join(project, "trackfw.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("trackfw.yaml must not be written on invalid scope: %v", statErr)
	}
}

// TestInitScopeGlobalPreCheckAbortsOnSymlinkClaudeDir asserts AC4 (#657), part 2:
// when ~/.claude is a symlink and --ai-tools claude is selected with the default
// global scope, init aborts WITHOUT writing any project files (Scaffold must not run).
// The error must mention --scope project so the user knows the remedy.
//
// Rule reconciliation: test affirms that preCheckAIToolDestinations fires before
// generators.Scaffold, so a symlink destination aborts init in its entirety —
// no partial writes. The default scope is global, so no --scope flag is needed
// to hit this path (Bruno's exact scenario).
func TestInitScopeGlobalPreCheckAbortsOnSymlinkClaudeDir(t *testing.T) {
	project, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Place a real directory as the symlink target.
	target := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	symlinkOrSkipCmd(t, target, claudeDir)

	// No --scope flag: default is "global", which is Bruno's scenario (#657).
	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "claude"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: ~/.claude is a symlink, pre-check must abort init")
	}
	// Error must name the remedy so the user knows what to do next.
	if !strings.Contains(err.Error(), "--scope project") {
		t.Errorf("error must mention '--scope project' remedy, got: %v", err)
	}

	// Scaffold must not have run — NO project files must exist.
	entries, readErr := os.ReadDir(project)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	if len(entries) > 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("project dir must be empty when pre-check aborts init, got files: %v", names)
	}
}

// TestInitPreCheckNonRefusalManagerListErrorLacksHint asserts that a non-refusal
// error from manager.List (here: "decode integration manifest: ..." from a corrupt
// integrations-manifest.json) does NOT receive the "--scope project" hint.
//
// Rule reconciliation: affirms that pathguard.IsRefusal (via isPathguardRefusal)
// correctly identifies non-refusal errors — the conclusion of ML-1B audit
// correction 2: hint only on pathguard refusals, not on unrelated errors that
// --scope project cannot resolve.
func TestInitPreCheckNonRefusalManagerListErrorLacksHint(t *testing.T) {
	_, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Create a corrupt integrations-manifest.json so that loadManifest inside
	// manager.List returns "decode integration manifest: ..." — a genuine
	// non-refusal error (no path containment check fires for it).
	manifestDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatalf("MkdirAll manifest dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "integrations-manifest.json"), []byte("{bad json}"), 0644); err != nil {
		t.Fatalf("WriteFile corrupt manifest: %v", err)
	}

	// Default scope = global, --ai-tools claude: plans are built for claude/global,
	// triggering the manifest decode path in manager.Inspect → manager.List.
	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "claude"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from corrupt manifest, got nil")
	}
	// The error must name the manifest so we know the right path was hit.
	if !strings.Contains(err.Error(), "integration manifest") {
		t.Errorf("expected error to mention 'integration manifest', got: %v", err)
	}
	// A non-refusal error must NOT receive the --scope project hint.
	if strings.Contains(err.Error(), "--scope project") {
		t.Errorf("non-refusal error must not include '--scope project' hint, got: %v", err)
	}
}

// TestInitScopeProjectSucceedsEvenWithSymlinkClaudeDir asserts AC4 (#657), part 3:
// when ~/.claude is a symlink but --scope project is used, init completes because
// AI tools are installed locally and never touch ~/.claude.
//
// Rule reconciliation: test affirms that --scope project is the working remedy for
// Bruno's scenario — the error message's suggestion is actually effective.
func TestInitScopeProjectSucceedsEvenWithSymlinkClaudeDir(t *testing.T) {
	project, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Place a real directory as the symlink target (same setup as the abort test).
	target := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	symlinkOrSkipCmd(t, target, claudeDir)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "claude", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for --scope project with symlink ~/.claude, got: %v", err)
	}

	// trackfw.yaml must exist — Scaffold ran successfully.
	if _, err := os.Stat(filepath.Join(project, "trackfw.yaml")); err != nil {
		t.Fatalf("trackfw.yaml not created: %v", err)
	}
}

// --------------------------------------------------------------------------
// T-12: scope hint absent when --scope project
// --------------------------------------------------------------------------

// TestInitScopeProjectHintAbsentOnSymlinkProjectClaudeDir asserts that when
// --scope project is used and <proj>/.claude is a symlink, the error does NOT
// suggest "--scope project" (the user is already using it — the hint would be
// misleading). Reconciliation: affirms that the hint is gated on scope != "project"
// in both Group 1 and Group 2 of preCheckAIToolDestinations.
func TestInitScopeProjectHintAbsentOnSymlinkProjectClaudeDir(t *testing.T) {
	project, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	_ = home

	// Make the project's .claude a symlink (Group 1 would catch it via plan
	// destinations when scope=project).
	target := t.TempDir()
	claudeDir := filepath.Join(project, ".claude")
	symlinkOrSkipCmd(t, target, claudeDir)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "claude", "--scope", "project"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: project .claude is a symlink, Scaffold pre-check must fire")
	}
	// The hint must NOT appear — user already uses --scope project.
	if strings.Contains(err.Error(), "--scope project") {
		t.Errorf("hint '--scope project' must be absent when scope is already project, got: %v", err)
	}
}

// TestInitScopeProjectHintPresentWhenGlobalScope is a positive control: with
// global scope and ~/.claude symlinked, the hint "--scope project" must appear.
// Without this control, TestInitScopeProjectHintAbsentOnSymlinkProjectClaudeDir
// would be vacuous. Reconciliation: affirms that the hint appears for global scope.
func TestInitScopeProjectHintPresentWhenGlobalScope(t *testing.T) {
	_, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	target := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	symlinkOrSkipCmd(t, target, claudeDir)

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "claude"}) // default scope = global
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: ~/.claude is a symlink, pre-check must abort")
	}
	if !strings.Contains(err.Error(), "--scope project") {
		t.Errorf("hint '--scope project' must appear for global scope refusal, got: %v", err)
	}
}

// --------------------------------------------------------------------------
// T-8: executeScaffoldAndInstall covers both TTY and non-TTY pre-check paths
// --------------------------------------------------------------------------

// TestExecuteScaffoldAndInstall_PreCheckFiresOnSymlinkClaudeDir asserts that
// executeScaffoldAndInstall fires preCheckAIToolDestinations before Scaffold:
// with ~/.claude as a symlink and --ai-tools claude/global, the call must fail
// and leave the project directory untouched. Reconciliation: affirms that the
// pre-check is inside executeScaffoldAndInstall — removing it from that
// function would let Scaffold run and create project files, failing this test.
// Because both TTY and non-TTY paths call executeScaffoldAndInstall, this test
// covers both paths.
func TestExecuteScaffoldAndInstall_PreCheckFiresOnSymlinkClaudeDir(t *testing.T) {
	project, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	target := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	symlinkOrSkipCmd(t, target, claudeDir)

	cfg := generators.Config{
		ProjectName: "test",
		ProjectType: "governance",
		Hooks:       "none",
		CI:          "none",
	}
	err := executeScaffoldAndInstall(cfg, []string{"claude"}, project, "global", home)
	if err == nil {
		t.Fatal("expected error: ~/.claude is a symlink, pre-check must abort")
	}

	// No project files must have been written.
	entries, _ := os.ReadDir(project)
	if len(entries) > 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("project must be empty after pre-check abort, got: %v", names)
	}
}

// --------------------------------------------------------------------------
// T-12: Scaffold pre-check on symlinked project directories
// --------------------------------------------------------------------------

// TestInitScaffoldPreCheckAbortsOnSymlinkProjectClaudeNoAITools asserts that
// even without --ai-tools, init aborts without partial writes when <proj>/.claude
// is a symlink. Reconciliation: affirms that the Scaffold upfront pre-check
// (T-12) fires for ClaudeCommandsDirPath regardless of the --ai-tools flag —
// preCheckAIToolDestinations returns early when aiTools is empty, so the only
// guard is the Scaffold pre-check.
// D1 note (ADR-2026-11-26): git init runs AFTER scaffold succeeds; because
// scaffold aborts here on the symlink pre-check, git init is never called and
// no .git/ is created — only the placed symlink exists after the abort.
func TestInitScaffoldPreCheckAbortsOnSymlinkProjectClaudeNoAITools(t *testing.T) {
	project, _ := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	target := t.TempDir()
	claudeDir := filepath.Join(project, ".claude")
	symlinkOrSkipCmd(t, target, claudeDir)

	cmd := newInitCmd()
	// No --ai-tools flag — preCheckAIToolDestinations returns nil early.
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: project .claude is a symlink, Scaffold pre-check must abort")
	}

	// Only the placed symlink (.claude) may exist — no scaffold files and no .git/.
	entries, _ := os.ReadDir(project)
	for _, e := range entries {
		if e.Name() == ".claude" {
			continue // the symlink we placed
		}
		t.Errorf("unexpected project file after pre-check abort: %s", e.Name())
	}
}

// TestInitScaffoldPreCheckAbortsOnSymlinkScriptsDir asserts that init aborts
// without partial writes when <proj>/scripts is a symlink. Reconciliation:
// affirms that the Scaffold pre-check covers the scripts/ directory via
// validateScriptPath — partial writes happen without the pre-check.
// D1 note (ADR-2026-11-26): git init runs AFTER scaffold succeeds; because
// scaffold aborts here on the symlink pre-check, git init is never called and
// no .git/ is created — only the placed symlink exists after the abort.
func TestInitScaffoldPreCheckAbortsOnSymlinkScriptsDir(t *testing.T) {
	project, _ := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	target := t.TempDir()
	scriptsDir := filepath.Join(project, "scripts")
	symlinkOrSkipCmd(t, target, scriptsDir)

	cmd := newInitCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: project scripts/ is a symlink, Scaffold pre-check must abort")
	}

	// Only the placed symlink (scripts/) may exist — no scaffold files and no .git/.
	entries, _ := os.ReadDir(project)
	for _, e := range entries {
		if e.Name() == "scripts" {
			continue
		}
		t.Errorf("unexpected project file after pre-check abort: %s", e.Name())
	}
}

// TestInitScaffoldPreCheckAbortsOnSymlinkVaultDir asserts that init aborts
// without partial writes when <proj>/vault is a symlink. Reconciliation:
// affirms that the Scaffold pre-check covers vault/ via vaultIndexPath —
// without the pre-check, govDirs (docs/) would be written before the error.
// D1 note (ADR-2026-11-26): git init runs AFTER scaffold succeeds; because
// scaffold aborts here on the symlink pre-check, git init is never called and
// no .git/ is created — only the placed symlink exists after the abort.
func TestInitScaffoldPreCheckAbortsOnSymlinkVaultDir(t *testing.T) {
	project, _ := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	target := t.TempDir()
	vaultDir := filepath.Join(project, "vault")
	symlinkOrSkipCmd(t, target, vaultDir)

	cmd := newInitCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: project vault/ is a symlink, Scaffold pre-check must abort")
	}

	// Only the placed symlink (vault/) may exist — no scaffold files and no .git/.
	entries, _ := os.ReadDir(project)
	for _, e := range entries {
		if e.Name() == "vault" {
			continue
		}
		t.Errorf("unexpected project file after pre-check abort: %s", e.Name())
	}
}

// TestInitScaffoldPreCheckAbortsOnSymlinkDocsDir asserts that init aborts
// without partial writes when <proj>/docs is a symlink. Reconciliation:
// affirms that the Scaffold pre-check covers docs/ via govDirs — without the
// pre-check, some govDirs would succeed before docs/ is refused.
// D1 note (ADR-2026-11-26): git init runs AFTER scaffold succeeds; because
// scaffold aborts here on the symlink pre-check, git init is never called and
// no .git/ is created — only the placed symlink exists after the abort.
func TestInitScaffoldPreCheckAbortsOnSymlinkDocsDir(t *testing.T) {
	project, _ := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	target := t.TempDir()
	docsDir := filepath.Join(project, "docs")
	symlinkOrSkipCmd(t, target, docsDir)

	cmd := newInitCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: project docs/ is a symlink, Scaffold pre-check must abort")
	}

	// Only the placed symlink (docs/) may exist — no scaffold files and no .git/.
	entries, _ := os.ReadDir(project)
	for _, e := range entries {
		if e.Name() == "docs" {
			continue
		}
		t.Errorf("unexpected project file after pre-check abort: %s", e.Name())
	}
}

// TestInitGlobalCodexRespectsCodexHome asserts ML-4A4 R6 for the init path:
// installAITools (site 621 in init.go) must set CodexDir so that a global Codex
// agent install lands at $CODEX_HOME/agents/, not at $HOME/.codex/agents/.
// Rule reconciliation: affirms that the CodexDir wiring at site 621 is what routes
// the write; removing it makes the install land in $HOME/.codex/ instead
// (counter-arm embedded in the test).
func TestInitGlobalCodexRespectsCodexHome(t *testing.T) {
	project, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	cfg := generators.Config{
		ProjectName: "test",
		ProjectType: "governance",
		Hooks:       "none",
		CI:          "none",
	}
	if err := executeScaffoldAndInstall(cfg, []string{"codex"}, project, "global", home); err != nil {
		t.Fatalf("executeScaffoldAndInstall failed: %v", err)
	}

	// Primary assertion: agent landed at CODEX_HOME/agents/, not HOME/.codex/agents/.
	wantPath := filepath.Join(codexHome, "agents", "trackfw-backend.toml")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("ML-4A4 R6: init global codex must write at CODEX_HOME/agents/, got error: %v", err)
	}
	absentPath := filepath.Join(home, ".codex", "agents", "trackfw-backend.toml")
	if _, err := os.Stat(absentPath); err == nil {
		t.Fatalf("ML-4A4 R6: init global codex must NOT write at HOME/.codex/agents/ when CODEX_HOME is set")
	}
}

// TestInitPreCheckRejectsSymlinkInCodexHome asserts ML-4A4 R6 for the pre-check
// path: preCheckAIToolDestinations (site 125 in init.go) must set CodexDir so that
// a symlink at $CODEX_HOME triggers an abort before Scaffold writes any project
// file. Rule reconciliation: without the CodexDir wiring, the pre-check inspects
// $HOME/.codex (which is not a symlink here) and passes — leaving executeScaffoldAndInstall
// free to write project files even though the target path is compromised.
func TestInitPreCheckRejectsSymlinkInCodexHome(t *testing.T) {
	project, home := initFixture(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Place the symlink at a path that does not yet exist so os.Symlink succeeds.
	// parent is a real dir; codexHomeLink is the path CODEX_HOME will point at,
	// and it does not exist on disk yet — matching the ~/.claude pattern used by
	// TestExecuteScaffoldAndInstall_PreCheckFiresOnSymlinkClaudeDir.
	parent := t.TempDir()
	realTarget := t.TempDir()
	codexHomeLink := filepath.Join(parent, "codex-home")
	symlinkOrSkipCmd(t, realTarget, codexHomeLink)
	t.Setenv("CODEX_HOME", codexHomeLink)

	cfg := generators.Config{
		ProjectName: "test",
		ProjectType: "governance",
		Hooks:       "none",
		CI:          "none",
	}
	err := executeScaffoldAndInstall(cfg, []string{"codex"}, project, "global", home)
	if err == nil {
		t.Fatal("expected error: CODEX_HOME is a symlink, pre-check must abort")
	}

	// No project files must have been written.
	entries, _ := os.ReadDir(project)
	for _, e := range entries {
		t.Errorf("project must be empty after pre-check abort, found: %s", e.Name())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D1 (ADR-2026-11-26): runGitInitForBootstrap — four-state table
// ────────────────────────────────────────────────────────────────────────────

// TestRunGitInitForBootstrap_FreshDir affirms the main D1 path: rc=128 with
// "not a git repository" in stderr triggers `git init -b main`, returns nil.
// Rule reconciliation: affirms the conclusion that rc=128/"not a git repository"
// is the correct discriminator for a directory that needs git init — the arm
// that runs initGitExecFn confirms init was called; the counter-arm uses
// rc=0/true to confirm the "already in a repo" path skips init.
func TestRunGitInitForBootstrap_FreshDir(t *testing.T) {
	// Inject: simulate fresh directory (not a git repository).
	oldCheck := initGitCheck
	oldExec := initGitExecFn
	oldStatus := initGitStatusFn
	t.Cleanup(func() { initGitCheck = oldCheck; initGitExecFn = oldExec; initGitStatusFn = oldStatus })

	initGitCheck = func() (string, string, int) {
		return "", "fatal: not a git repository (or any of the parent directories): .git", 128
	}

	var execArgs [][]string
	initGitExecFn = func(args ...string) (string, error) {
		execArgs = append(execArgs, append([]string{}, args...))
		return "", nil
	}
	// Stub status so the test does not query the real repo's working tree.
	initGitStatusFn = func() string { return "" }

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil error for fresh dir, got: %v", err)
	}
	// At least one git exec call must have been made (git init -b main or fallback).
	if len(execArgs) == 0 {
		t.Error("expected initGitExecFn to be called for fresh dir, got 0 calls")
	}
	firstArgs := execArgs[0]
	if len(firstArgs) < 1 || firstArgs[0] != "init" {
		t.Errorf("expected first git call to be 'init', got %v", firstArgs)
	}
}

// TestRunGitInitForBootstrap_AlreadyInsideWorkTree affirms that rc=0/stdout=="true"
// skips git init and returns nil (D1 state 1).
// Rule reconciliation: affirms the conclusion that an existing work tree is
// detected and git init is not invoked — the counter-arm (FreshDir) confirms
// init IS invoked when rc=128.
func TestRunGitInitForBootstrap_AlreadyInsideWorkTree(t *testing.T) {
	oldCheck := initGitCheck
	oldExec := initGitExecFn
	t.Cleanup(func() { initGitCheck = oldCheck; initGitExecFn = oldExec })

	initGitCheck = func() (string, string, int) {
		return "true", "", 0
	}
	var execCalled bool
	initGitExecFn = func(args ...string) (string, error) {
		execCalled = true
		return "", nil
	}

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil error when already in work tree, got: %v", err)
	}
	if execCalled {
		t.Error("initGitExecFn must not be called when already inside a work tree")
	}
}

// TestRunGitInitForBootstrap_InsideBareOrGitDir affirms that rc=0/stdout=="false"
// returns a non-nil error (D1 state 2: inside .git/ or a bare repo).
// Rule reconciliation: affirms M-X9 conclusion that git init inside a gitdir or
// bare repo would create a nested repo — this case must abort before any write.
func TestRunGitInitForBootstrap_InsideBareOrGitDir(t *testing.T) {
	oldCheck := initGitCheck
	t.Cleanup(func() { initGitCheck = oldCheck })

	initGitCheck = func() (string, string, int) {
		return "false", "", 0
	}

	err := runGitInitForBootstrap(t.TempDir())
	if err == nil {
		t.Fatal("expected non-nil error for rc=0/false (inside .git/ or bare repo), got nil")
	}
	if !strings.Contains(err.Error(), "refusing") {
		t.Errorf("error must mention 'refusing', got: %v", err)
	}
}

// TestRunGitInitForBootstrap_DubiousOwnership affirms that rc=128 with stderr
// NOT containing "not a git repository" (e.g., "dubious ownership") warns but
// returns nil so scaffold proceeds (D1 state 4).
// Rule reconciliation: affirms the conclusion that ownership/permission errors
// are non-fatal for init — git init is skipped but scaffold proceeds (fail-open).
func TestRunGitInitForBootstrap_DubiousOwnership(t *testing.T) {
	oldCheck := initGitCheck
	oldExec := initGitExecFn
	t.Cleanup(func() { initGitCheck = oldCheck; initGitExecFn = oldExec })

	initGitCheck = func() (string, string, int) {
		return "", "fatal: detected dubious ownership in repository", 128
	}
	var execCalled bool
	initGitExecFn = func(args ...string) (string, error) {
		execCalled = true
		return "", nil
	}

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil (warn + skip) for dubious ownership, got: %v", err)
	}
	if execCalled {
		t.Error("initGitExecFn must not be called for dubious ownership (non-fatal skip)")
	}
}

// TestRunGitInitForBootstrap_NoGitBinary affirms that rc==-1 (git not in PATH)
// warns but returns nil so scaffold proceeds (D1 state 5 / rc=-1 branch).
// Rule reconciliation: affirms the ADR-2026-11-26 decision that a missing git
// binary is non-fatal for init — the user must run git init manually.
func TestRunGitInitForBootstrap_NoGitBinary(t *testing.T) {
	oldCheck := initGitCheck
	oldExec := initGitExecFn
	t.Cleanup(func() { initGitCheck = oldCheck; initGitExecFn = oldExec })

	initGitCheck = func() (string, string, int) {
		return "", "", -1
	}
	var execCalled bool
	initGitExecFn = func(args ...string) (string, error) {
		execCalled = true
		return "", nil
	}

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil (warn + skip) when git is missing, got: %v", err)
	}
	if execCalled {
		t.Error("initGitExecFn must not be called when git binary is missing")
	}
}

// TestRunGitInitForBootstrap_FallbackSymbolicRef affirms that when `git init -b main`
// fails (old git without -b), the fallback runs `git init` then `git symbolic-ref HEAD
// refs/heads/main` and still returns nil.
// Rule reconciliation: affirms the backward-compatibility path for git < 2.28
// — without it, init would silently leave the default branch as 'master' on old git.
func TestRunGitInitForBootstrap_FallbackSymbolicRef(t *testing.T) {
	oldCheck := initGitCheck
	oldExec := initGitExecFn
	oldStatus := initGitStatusFn
	t.Cleanup(func() { initGitCheck = oldCheck; initGitExecFn = oldExec; initGitStatusFn = oldStatus })

	initGitCheck = func() (string, string, int) {
		return "", "fatal: not a git repository (or any of the parent directories): .git", 128
	}

	var execArgs [][]string
	initGitExecFn = func(args ...string) (string, error) {
		execArgs = append(execArgs, append([]string{}, args...))
		// First call: `git init -b main` fails (old git).
		if len(execArgs) == 1 {
			return "", fmt.Errorf("unknown switch `b'")
		}
		return "", nil
	}
	// Stub status so the test does not query the real repo's working tree.
	initGitStatusFn = func() string { return "" }

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil from fallback path, got: %v", err)
	}
	// Expect at least: ["init", "-b", "main"] + ["init"] + ["symbolic-ref", ...]
	if len(execArgs) < 3 {
		t.Fatalf("expected at least 3 git calls (init -b, init, symbolic-ref), got %v", execArgs)
	}
	var foundSymbolicRef bool
	for _, a := range execArgs {
		if len(a) >= 1 && a[0] == "symbolic-ref" {
			foundSymbolicRef = true
		}
	}
	if !foundSymbolicRef {
		t.Errorf("expected symbolic-ref call in fallback, calls were: %v", execArgs)
	}
}

// TestRunGitInitForBootstrap_FreshDir_PrintsBootstrapSteps is the arm: when git init
// runs (D1 state 3, rc=128 "not a git repository"), the bootstrap steps are written to
// initBootstrapOut so the user knows how to stage and push the initial commit.
//
// Rule reconciliation: affirms D5 (ADR-2026-11-26) — the bootstrap surface printed by
// init must guide the agent through the first commit and push without using whole-tree
// staging ('git add .' / 'git add -A'), which the guard denies.
// Counter-arm: TestRunGitInitForBootstrap_AlreadyInsideWorkTree_NoBootstrapSteps.
func TestRunGitInitForBootstrap_FreshDir_PrintsBootstrapSteps(t *testing.T) {
	oldCheck := initGitCheck
	oldExec := initGitExecFn
	oldOut := initBootstrapOut
	oldStatus := initGitStatusFn
	t.Cleanup(func() {
		initGitCheck = oldCheck
		initGitExecFn = oldExec
		initBootstrapOut = oldOut
		initGitStatusFn = oldStatus
	})

	initGitCheck = func() (string, string, int) {
		return "", "fatal: not a git repository (or any of the parent directories): .git", 128
	}
	initGitExecFn = func(args ...string) (string, error) { return "", nil }
	// Stub status to empty: mock path does not have real files, so we expect
	// the generic "each path listed" placeholder, not an actual file list.
	initGitStatusFn = func() string { return "" }
	var buf bytes.Buffer
	initBootstrapOut = &buf

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil error for fresh dir, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "trackfw push") {
		t.Errorf("bootstrap output must contain 'trackfw push', got: %q", out)
	}
	if !strings.Contains(out, "git status --short") {
		t.Errorf("bootstrap output must contain 'git status --short', got: %q", out)
	}
	// Whole-tree staging must not appear: 'git add .' where dot is followed by whitespace
	// or end-of-string. This regex does NOT match 'git add .gitignore' (dot + letter).
	wholeTreeRe := regexp.MustCompile(`\bgit add \.(\s|\z)`)
	if wholeTreeRe.MatchString(out) {
		t.Errorf("bootstrap output must not contain whole-tree 'git add .', got: %q", out)
	}
	if strings.Contains(out, "git add -A") {
		t.Errorf("bootstrap output must not contain 'git add -A', got: %q", out)
	}
}

// TestRunGitInitForBootstrap_AlreadyInsideWorkTree_NoBootstrapSteps is the counter-arm:
// when already inside a work tree (D1 state 1, rc=0/"true"), no bootstrap steps are
// printed — the repo already exists and the user does not need onboarding.
//
// Rule reconciliation: affirms D5 (ADR-2026-11-26) — the bootstrap steps are emitted
// only once, when git init actually runs. An existing repo must not receive them again.
// Arm: TestRunGitInitForBootstrap_FreshDir_PrintsBootstrapSteps.
func TestRunGitInitForBootstrap_AlreadyInsideWorkTree_NoBootstrapSteps(t *testing.T) {
	oldCheck := initGitCheck
	oldOut := initBootstrapOut
	t.Cleanup(func() {
		initGitCheck = oldCheck
		initBootstrapOut = oldOut
	})

	initGitCheck = func() (string, string, int) {
		return "true", "", 0
	}
	var buf bytes.Buffer
	initBootstrapOut = &buf

	if err := runGitInitForBootstrap(t.TempDir()); err != nil {
		t.Fatalf("expected nil error when already in work tree, got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no bootstrap output for existing repo, got: %q", buf.String())
	}
}

// TestRunInit_RealGit_ClaudeScope_PrintsActualPaths is a real-git integration test
// for the init bootstrap output (ML-1F, ADR-2026-11-26). When init runs in a fresh
// directory (no git repo) with --ai-tools claude --scope project, it runs real git
// init, writes the scaffold, then queries git status --porcelain to build the git add
// line. The printed line must contain ".claude" and "CLAUDE.md" (created by the claude
// scope), and when executed, it must leave git status --porcelain without any "??" entries.
//
// Rule reconciliation: affirms D5+ML-1F — init does not use a fixed path list; it
// queries the real post-scaffold state. The arm fails on the fixed-list implementation
// because ".claude" and "CLAUDE.md" are absent from the fixed list.
// Counter-arm: TestRunGitInitForBootstrap_AlreadyInsideWorkTree_NoBootstrapSteps
// (mock) confirms an existing repo does not print bootstrap steps.
func TestRunInit_RealGit_ClaudeScope_PrintsActualPaths(t *testing.T) {
	project, home := initFixture(t)
	// Isolate Claude and Codex config dirs so the test does not write to the
	// developer's real ~/.claude or ~/.codex (memory notes: codex-home-env-isolation,
	// claude-config-dir-isolation).
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	// Reset the config singleton so it picks up the fresh project dir.
	config.Reset()
	t.Cleanup(config.Reset)

	// Capture bootstrap output (initBootstrapOut is the injection point used by
	// runGitInitForBootstrap — it receives the printed bootstrap steps).
	var buf bytes.Buffer
	oldOut := initBootstrapOut
	initBootstrapOut = &buf
	t.Cleanup(func() { initBootstrapOut = oldOut })

	// Do NOT inject initGitCheck or initGitExecFn or initGitStatusFn — this test
	// exercises the real paths: git init runs for real, scaffold writes real files,
	// and git status --porcelain queries the real filesystem.
	cfg := generators.Config{
		ProjectName: "test-project",
		ProjectType: "governance",
		Hooks:       "none",
		CI:          "none",
	}
	if err := executeScaffoldAndInstall(cfg, []string{"claude"}, project, "project", home); err != nil {
		t.Fatalf("executeScaffoldAndInstall failed: %v", err)
	}

	out := buf.String()

	// The printed git add line must contain ".claude" and "CLAUDE.md" (the agent
	// config directory and rules file created for the claude scope).
	if !strings.Contains(out, ".claude") {
		t.Errorf("bootstrap output must contain '.claude' (created by --scope project), got: %q", out)
	}
	if !strings.Contains(out, "CLAUDE.md") {
		t.Errorf("bootstrap output must contain 'CLAUDE.md' (created by --scope project), got: %q", out)
	}

	// Parse the "git add ..." line and execute it in the project dir.
	var addLine string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "git add ") {
			addLine = trimmed
			break
		}
	}
	if addLine == "" {
		t.Fatal("bootstrap output must contain a 'git add ...' line, got: " + out)
	}

	// Set required git identity in the project so git add can run (git add itself
	// does not need identity, but we set it to keep the repo in a valid state).
	for _, args := range [][]string{
		{"config", "user.email", "t@localhost"},
		{"config", "user.name", "T"},
	} {
		gitCmd := exec.Command("git", args...)
		gitCmd.Dir = project
		if out, err := gitCmd.CombinedOutput(); err != nil {
			t.Fatalf("git config failed: %v\n%s", err, out)
		}
	}

	// Execute the git add line via sh -c so that POSIX single-quoted paths
	// (produced by D1-6 fix, ML-2C) are interpreted correctly by the shell.
	// Using strings.Fields + exec.Command would leave the single quotes as
	// part of the path argument and fail to find the files.
	addCmd := exec.Command("sh", "-c", addLine)
	addCmd.Dir = project
	if addOut, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("executing %q failed: %v\n%s", addLine, err, addOut)
	}

	// After the git add, there must be no untracked (??) files in the index.
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = project
	statusOut, err := statusCmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain failed: %v", err)
	}
	for _, line := range strings.Split(string(statusOut), "\n") {
		if strings.HasPrefix(line, "?? ") {
			t.Errorf("after executing the printed git add, untracked file remains: %q (full status: %q)", line, string(statusOut))
		}
	}
}

// TestD1_6_RealGit_ShellInjectionSafe (ML-2C, D1-6) affirms that filenames
// containing shell metacharacters ($(touch PWNED) and -A) are safely handled
// by the POSIX-single-quoted git add line emitted by PrintBootstrapSteps.
//
// When the printed line is executed with sh -c in the project directory:
//  1. PWNED must NOT be created ($(touch PWNED) is not executed as a command).
//  2. `git diff --cached --name-only` must contain the two literal filenames.
//
// Rule reconciliation: affirms D1-6 (ML-2C, ADR-2026-11-26) — the POSIX
// single-quoted git add line suppresses $() command substitution and passes
// -A as a literal pathspec (not the --all flag), so that the agent staging
// the scaffold cannot be tricked by maliciously named files placed in the
// project directory before trackfw init ran.
func TestD1_6_RealGit_ShellInjectionSafe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX single-quoting is a POSIX shell feature — skipped on Windows")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available — skipping shell-injection test")
	}

	dir := t.TempDir()
	gitRepoIn(t, dir,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
	)

	// Create the two dangerous files BEFORE querying git status.
	dangerousName := "$(touch PWNED)"
	dashAName := "-A"
	for _, name := range []string{dangerousName, dashAName} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}

	// Query git status -z to build the path list (same as production code).
	statusCmd := exec.Command("git", "status", "--porcelain", "-z", "--untracked-files=normal")
	statusCmd.Dir = dir
	var statusBuf bytes.Buffer
	statusCmd.Stdout = &statusBuf
	if err := statusCmd.Run(); err != nil {
		t.Fatalf("git status -z: %v", err)
	}
	paths := parseGitStatusPorcelain(statusBuf.String())
	if len(paths) == 0 {
		t.Fatal("expected at least 2 untracked files from git status -z, got none")
	}

	// Build the git add line via PrintBootstrapSteps.
	var buf bytes.Buffer
	generators.PrintBootstrapSteps(&buf, paths)
	out := buf.String()

	// Extract the git add line.
	var addLine string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "git add -- ") {
			addLine = trimmed
			break
		}
	}
	if addLine == "" {
		t.Fatalf("bootstrap output must contain 'git add -- ...'; got: %q", out)
	}

	// Execute the git add line via sh -c in the project directory.
	addCmd := exec.Command("sh", "-c", addLine)
	addCmd.Dir = dir
	if addOut, addErr := addCmd.CombinedOutput(); addErr != nil {
		t.Fatalf("sh -c %q failed: %v\n%s", addLine, addErr, addOut)
	}

	// 1. PWNED must NOT exist — $(touch PWNED) was not executed.
	if _, statErr := os.Stat(filepath.Join(dir, "PWNED")); statErr == nil {
		t.Error("D1-6: PWNED was created — shell injection via $(touch PWNED) was not prevented")
	}

	// 2. Both dangerous filenames must be staged.
	cachedCmd := exec.Command("git", "diff", "--cached", "--name-only")
	cachedCmd.Dir = dir
	cachedOut, cachedErr := cachedCmd.Output()
	if cachedErr != nil {
		t.Fatalf("git diff --cached --name-only: %v", cachedErr)
	}
	staged := string(cachedOut)
	if !strings.Contains(staged, dangerousName) {
		t.Errorf("D1-6: file %q must be staged; staged files: %q", dangerousName, staged)
	}
	if !strings.Contains(staged, dashAName) {
		t.Errorf("D1-6: file %q must be staged as literal path (not flag); staged files: %q", dashAName, staged)
	}
}
