package validator

// validator_windows_sh_missing_rule_test.go — tests for windows_sh_missing rule
//
// REQ-IMP-9 (REQ-2026-11-15, ML-3D).
//
// All tests inject goos and lookPath directly into checkWindowsShMissing so they
// run on any host platform without t.Skip. The package-level seams (CurrentGOOS,
// shLookPathFn) are exercised by the public-wrapper tests.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// shAbsent is a lookPath stub that always returns "not found".
func shAbsent(_ string) (string, error) {
	return "", errors.New("not found in PATH")
}

// shPresent is a lookPath stub that always succeeds.
func shPresent(_ string) (string, error) {
	return "/usr/bin/sh", nil
}

// makeShFixture creates a temp directory and places the given POSIX-slash
// relative paths on disk. Returns the absolute root path.
func makeShFixture(t *testing.T, paths ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range paths {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("makeShFixture: mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("makeShFixture: write %s: %v", full, err)
		}
	}
	return root
}

// TestWindowsShMissing_Warning_NamesArtifact asserts:
// goos=windows + sh absent + .sh artifact present → warning that names the artifact.
// (ML-3D conclusion: on Windows with sh absent the rule fires per artifact present.)
func TestWindowsShMissing_Warning_NamesArtifact(t *testing.T) {
	root := makeShFixture(t, "scripts/trackfw-validate.sh")
	msgs := checkWindowsShMissing("windows", shAbsent, root)
	if len(msgs) == 0 {
		t.Fatal("expected warning, got none")
	}
	if !strings.Contains(msgs[0], "scripts/trackfw-validate.sh") {
		t.Errorf("warning must name the artifact path; got: %s", msgs[0])
	}
	if !strings.Contains(msgs[0], "sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL)") {
		t.Errorf("warning must include the sh-missing phrase; got: %s", msgs[0])
	}
}

// TestWindowsShMissing_ShPresent_Silent asserts:
// goos=windows + sh present + .sh artifact present → no warning.
// (ML-3D conclusion: sh in PATH suppresses the rule even on Windows.)
func TestWindowsShMissing_ShPresent_Silent(t *testing.T) {
	root := makeShFixture(t, "scripts/trackfw-validate.sh")
	msgs := checkWindowsShMissing("windows", shPresent, root)
	if len(msgs) != 0 {
		t.Errorf("expected no warning when sh is present, got: %v", msgs)
	}
}

// TestWindowsShMissing_NoArtifacts_BarrierWarning asserts:
// goos=windows + sh absent + no .sh artifacts → warning citing barrier gates.
// (ML-3D2 conclusion: sh absent on Windows always fires the rule; without artifacts,
// the message cites that barrier gates using sh -c will not execute.)
func TestWindowsShMissing_NoArtifacts_BarrierWarning(t *testing.T) {
	root := t.TempDir() // no artifacts
	msgs := checkWindowsShMissing("windows", shAbsent, root)
	if len(msgs) == 0 {
		t.Fatal("expected barrier warning when sh absent on Windows with no artifacts, got none")
	}
	if !strings.Contains(msgs[0], "sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL)") {
		t.Errorf("warning must include the sh-missing phrase; got: %s", msgs[0])
	}
	if !strings.Contains(msgs[0], "trackfw barrier") {
		t.Errorf("warning without artifacts must cite trackfw barrier; got: %s", msgs[0])
	}
}

// TestWindowsShMissing_DarwinSilent asserts:
// goos=darwin + sh absent + .sh artifact present → no warning.
// (ML-3D conclusion: GOOS guard suppresses the rule on non-Windows platforms.)
func TestWindowsShMissing_DarwinSilent(t *testing.T) {
	root := makeShFixture(t, "scripts/trackfw-validate.sh", "scripts/trackfw-attention-signal.sh")
	for _, goos := range []string{"darwin", "linux"} {
		msgs := checkWindowsShMissing(goos, shAbsent, root)
		if len(msgs) != 0 {
			t.Errorf("goos=%s: expected no warning, got: %v", goos, msgs)
		}
	}
}

// TestWindowsShMissing_GuardNeverCited asserts:
// guard scripts (trackfw-credential-guard.sh, trackfw-git-branch-guard.sh)
// are never named in the warning, even when they are present on disk.
// (ML-3D conclusion: the guard exclusion holds — guards are Go subcommands.)
func TestWindowsShMissing_GuardNeverCited(t *testing.T) {
	// Place every convenience artifact AND both guard scripts.
	all := append(shConvenienceArtifacts,
		"scripts/trackfw-credential-guard.sh",
		"scripts/trackfw-git-branch-guard.sh",
	)
	root := makeShFixture(t, all...)
	msgs := checkWindowsShMissing("windows", shAbsent, root)
	if len(msgs) == 0 {
		t.Fatal("expected at least one warning (convenience artifacts present), got none")
	}
	for _, msg := range msgs {
		if strings.Contains(msg, "trackfw-credential-guard.sh") {
			t.Errorf("credential guard must not appear in warning; got: %s", msg)
		}
		if strings.Contains(msg, "trackfw-git-branch-guard.sh") {
			t.Errorf("git-branch guard must not appear in warning; got: %s", msg)
		}
	}
}

// TestWindowsShMissing_AllArtifactsListed asserts:
// when all three .sh convenience artifacts are present, all are listed in a
// single warning message.
// (ML-3D conclusion: one warning message per rule invocation names all present artifacts;
// ML-4A2 conclusion: git hook scripts are excluded from the list — only the three
// scripts/ artifacts remain.)
func TestWindowsShMissing_AllArtifactsListed(t *testing.T) {
	artifacts := []string{
		"scripts/trackfw-attention-signal.sh",
		"scripts/trackfw-attention-cleanup.sh",
		"scripts/trackfw-validate.sh",
	}
	root := makeShFixture(t, artifacts...)
	msgs := checkWindowsShMissing("windows", shAbsent, root)
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 warning message, got %d: %v", len(msgs), msgs)
	}
	for _, rel := range artifacts {
		if !strings.Contains(msgs[0], rel) {
			t.Errorf("warning must list %q; full message: %s", rel, msgs[0])
		}
	}
}

// TestWindowsShMissing_GitHookArtifactsNotListed asserts:
// goos=windows + sh absent + only git-hook-managed artifacts present →
// barrier-only variant (no "the following scripts…"), and none of the hook
// file names appear in the message.
// Counter-arm: adding scripts/trackfw-validate.sh makes the scripts list appear
// (the hook names still must not appear).
// (ML-4A2 conclusion: git hook scripts (.husky/pre-commit, .husky/commit-msg,
// .lefthook/commit-msg/trackfw-req-check.sh) are excluded from shConvenienceArtifacts
// because Git for Windows augments the hook PATH with usr\bin, so sh is always
// available to hooks regardless of the user's interactive PATH.)
func TestWindowsShMissing_GitHookArtifactsNotListed(t *testing.T) {
	gitHookArtifacts := []string{
		".husky/pre-commit",
		".husky/commit-msg",
		".lefthook/commit-msg/trackfw-req-check.sh",
	}
	root := makeShFixture(t, gitHookArtifacts...)

	// Arm: hooks only → barrier-only variant, no hook names in message.
	msgs := checkWindowsShMissing("windows", shAbsent, root)
	if len(msgs) == 0 {
		t.Fatal("expected barrier warning, got none")
	}
	if strings.Contains(msgs[0], "the following scripts") {
		t.Errorf("arm: barrier-only variant must not contain 'the following scripts'; got: %s", msgs[0])
	}
	for _, hookPath := range gitHookArtifacts {
		if strings.Contains(msgs[0], hookPath) {
			t.Errorf("arm: git hook artifact %q must not appear in warning; got: %s", hookPath, msgs[0])
		}
	}
	if !strings.Contains(msgs[0], "trackfw barrier") {
		t.Errorf("arm: barrier-only variant must cite trackfw barrier; got: %s", msgs[0])
	}

	// Counter-arm: add a scripts/ artifact → list variant appears; hook names still absent.
	fullScript := makeShFixture(t, append(gitHookArtifacts, "scripts/trackfw-validate.sh")...)
	counterMsgs := checkWindowsShMissing("windows", shAbsent, fullScript)
	if len(counterMsgs) == 0 {
		t.Fatal("counter-arm: expected list warning, got none")
	}
	if !strings.Contains(counterMsgs[0], "the following scripts") {
		t.Errorf("counter-arm: expected list variant; got: %s", counterMsgs[0])
	}
	if !strings.Contains(counterMsgs[0], "scripts/trackfw-validate.sh") {
		t.Errorf("counter-arm: scripts/trackfw-validate.sh must appear in list; got: %s", counterMsgs[0])
	}
	for _, hookPath := range gitHookArtifacts {
		if strings.Contains(counterMsgs[0], hookPath) {
			t.Errorf("counter-arm: git hook artifact %q must not appear in warning even with scripts/ present; got: %s", hookPath, counterMsgs[0])
		}
	}
}

// TestWindowsShMissing_PublicWrapperUsesSeams asserts that the public
// validateWindowsShMissing wrapper respects CurrentGOOS and ShLookPathFn seams.
// (ML-3D conclusion: the public entry point honours the same guards as the inner function.)
func TestWindowsShMissing_PublicWrapperUsesSeams(t *testing.T) {
	origGOOS := CurrentGOOS
	origFn := ShLookPathFn
	t.Cleanup(func() {
		CurrentGOOS = origGOOS
		ShLookPathFn = origFn
	})

	// Create fixture first, then chdir — so TempDir RemoveAll is registered
	// before the directory-restore cleanup; reverse execution order means we
	// leave the directory before Windows can't-delete-open-dir errors fire.
	root := makeShFixture(t, "scripts/trackfw-validate.sh")
	chdir(t, root)

	CurrentGOOS = "windows"
	ShLookPathFn = shAbsent
	msgs := validateWindowsShMissing()
	if len(msgs) == 0 {
		t.Error("expected warning via public wrapper when GOOS=windows and sh absent")
	}

	CurrentGOOS = runtime.GOOS // reset to real value → rule goes quiet on non-windows hosts
	msgs = validateWindowsShMissing()
	if runtime.GOOS != "windows" && len(msgs) != 0 {
		t.Errorf("expected no warning on real GOOS=%s, got: %v", runtime.GOOS, msgs)
	}
}

// TestWindowsShMissing_FiacaoValidateUnfiltered — prova que a regra está fiada nos dois
// sítios de validator.go (ValidateUnfiltered e validateUnfilteredTagged).
// Reconciliação (ML-3D2): afirma que applyRuleWarnOnly/applyRuleWarnOnlyTagged em
// validator.go roteia windows_sh_missing para o chamador via o stack completo do validator.
// Contra-braço: comentar qualquer um dos dois sítios de fiação em validator.go faz o
// sub-teste correspondente reprovar.
func TestWindowsShMissing_FiacaoValidateUnfiltered(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	writeFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr/\n")
	writeFile(t, dir, "scripts/trackfw-validate.sh", "#!/bin/sh\n")

	origGOOS := CurrentGOOS
	origFn := ShLookPathFn
	chdir(t, dir)
	t.Cleanup(func() {
		CurrentGOOS = origGOOS
		ShLookPathFn = origFn
		currentOriginMain = originMainAnchor{}
	})
	CurrentGOOS = "windows"
	ShLookPathFn = shAbsent

	t.Run("ValidateUnfiltered", func(t *testing.T) {
		config.Reset()
		_, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered: %v", err)
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "windows_sh_missing") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidateUnfiltered must include windows_sh_missing warning; got warnings: %v", warnings)
		}
	})

	t.Run("validateUnfilteredTagged", func(t *testing.T) {
		config.Reset()
		_, warningsTagged, err := validateUnfilteredTagged()
		if err != nil {
			t.Fatalf("validateUnfilteredTagged: %v", err)
		}
		found := false
		for _, w := range warningsTagged {
			if w.Rule == "windows_sh_missing" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("validateUnfilteredTagged must include windows_sh_missing warning; got: %v", warningsTagged)
		}
	})
}
