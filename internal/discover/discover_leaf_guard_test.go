package discover

// discover_leaf_guard_test.go — load-bearing tests for ML-4C leaf-symlink gap.
//
// These four tests prove that the (B) gap de folha in discover.go is real
// (they FAIL on old code without the leaf guard) and closed (they PASS after
// the fix).  Each test is described by a one-sentence assertion per the
// Regra Dura de Reconciliação.
//
// symlinkOrSkip and isSymlinkPrivilegeError live in symlink_helper_test.go
// (same package).  TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 prevents npm/npx
// calls in installHusky / installHuskyNPX.

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteValidateScript_LeafSymlinkRefused asserts that writeValidateScript
// refuses when scripts/trackfw-validate.sh is a symlink pointing outside the
// project root — the directory guard (root, "scripts") does not walk below the
// directory, so without a leaf guard the write would follow the symlink.
// ML-4C conclusion: (B) gap at discover.go:108 is load-bearing; leaf guard closes it.
func TestWriteValidateScript_LeafSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	// Create scripts/ as a REAL directory (so the directory guard passes).
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}

	// Place a victim file outside the project.
	victim := filepath.Join(outside, "vitima.txt")
	const originalContent = "CONTEUDO ORIGINAL — NAO DEVE SER SOBRESCRITO\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Place the leaf file as a symlink pointing to the victim.
	leaf := filepath.Join(dir, "scripts", "trackfw-validate.sh")
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above
	symlinkOrSkip(t, victim, leaf)

	err := writeValidateScript(dir)
	if err == nil {
		t.Fatal("writeValidateScript must return an error when the leaf is a symlink, got nil")
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatalf("could not read victim after refused write: %v", readErr)
	}
	if string(got) != originalContent {
		t.Fatalf("containment violated: victim was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// TestInstallHook_HuskyLeafSymlinkRefused asserts that installHook("husky", ...)
// refuses when .husky/pre-commit is a symlink pointing outside the project
// root — the directory guard (root, ".husky") does not walk below the
// directory, so without a leaf guard the write would follow the symlink.
// ML-4C conclusion: (B) gap at discover.go:160 is load-bearing; leaf guard closes it.
func TestInstallHook_HuskyLeafSymlinkRefused(t *testing.T) {
	t.Setenv("TRACKFW_DISABLE_EXTERNAL_COMMANDS", "1")

	dir := t.TempDir()
	outside := t.TempDir()

	// Create .husky/ as a REAL directory (so the directory guard passes).
	if err := os.MkdirAll(filepath.Join(dir, ".husky"), 0755); err != nil {
		t.Fatal(err)
	}

	// Place a victim file outside the project.
	victim := filepath.Join(outside, "vitima.txt")
	const originalContent = "CONTEUDO ORIGINAL — NAO DEVE SER SOBRESCRITO\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Place .husky/pre-commit as a symlink pointing to the victim.
	leaf := filepath.Join(dir, ".husky", "pre-commit")
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above
	symlinkOrSkip(t, victim, leaf)

	err := installHook("husky", dir, io.Discard)
	if err == nil {
		t.Fatal("installHook(husky) must return an error when .husky/pre-commit is a symlink, got nil")
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatalf("could not read victim after refused write: %v", readErr)
	}
	if string(got) != originalContent {
		t.Fatalf("containment violated: victim was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// TestInstallHusky_LeafSymlinkRefused asserts that installHusky refuses when
// .husky/pre-commit is a symlink pointing outside the project root.
// ML-4C conclusion: (B) gap at discover.go:269 is load-bearing; leaf guard closes it.
func TestInstallHusky_LeafSymlinkRefused(t *testing.T) {
	t.Setenv("TRACKFW_DISABLE_EXTERNAL_COMMANDS", "1")

	dir := t.TempDir()
	outside := t.TempDir()

	// Create .husky/ as a REAL directory (so the directory guard passes).
	if err := os.MkdirAll(filepath.Join(dir, ".husky"), 0755); err != nil {
		t.Fatal(err)
	}

	// Place a victim file outside the project.
	victim := filepath.Join(outside, "vitima.txt")
	const originalContent = "CONTEUDO ORIGINAL — NAO DEVE SER SOBRESCRITO\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Place .husky/pre-commit as a symlink pointing to the victim.
	leaf := filepath.Join(dir, ".husky", "pre-commit")
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above
	symlinkOrSkip(t, victim, leaf)

	err := installHusky(dir, io.Discard)
	if err == nil {
		t.Fatal("installHusky must return an error when .husky/pre-commit is a symlink, got nil")
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatalf("could not read victim after refused write: %v", readErr)
	}
	if string(got) != originalContent {
		t.Fatalf("containment violated: victim was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// TestInstallHuskyNPX_LeafSymlinkRefused asserts that installHuskyNPX refuses when
// .husky/pre-commit is a symlink pointing outside the project root.
// ML-4C conclusion: (B) gap at discover.go:307 is load-bearing; leaf guard closes it.
func TestInstallHuskyNPX_LeafSymlinkRefused(t *testing.T) {
	t.Setenv("TRACKFW_DISABLE_EXTERNAL_COMMANDS", "1")

	dir := t.TempDir()
	outside := t.TempDir()

	// Create .husky/ as a REAL directory (so the directory guard passes).
	if err := os.MkdirAll(filepath.Join(dir, ".husky"), 0755); err != nil {
		t.Fatal(err)
	}

	// Place a victim file outside the project.
	victim := filepath.Join(outside, "vitima.txt")
	const originalContent = "CONTEUDO ORIGINAL — NAO DEVE SER SOBRESCRITO\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Place .husky/pre-commit as a symlink pointing to the victim.
	leaf := filepath.Join(dir, ".husky", "pre-commit")
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above
	symlinkOrSkip(t, victim, leaf)

	err := installHuskyNPX(dir, io.Discard)
	if err == nil {
		t.Fatal("installHuskyNPX must return an error when .husky/pre-commit is a symlink, got nil")
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatalf("could not read victim after refused write: %v", readErr)
	}
	if string(got) != originalContent {
		t.Fatalf("containment violated: victim was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}
