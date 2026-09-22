package generators

// scaffold_leaf_guard_test.go — ML-3A: proves that scaffold functions guard
// the LEAF FILE and not only the parent directory.
//
// Defect: installGlobalSkillInner (and other scaffold writers) called
// rejectScaffoldPath on the DIRECTORY before writing a file inside it.
// RejectSymlinks walks upward from its argument — it never descends, so a
// file that is itself a symlink inside a clean directory was not caught.
//
// Fix: every write site now calls rejectScaffoldPath(root, fileAbsPath)
// immediately before os.WriteFile, in addition to (not instead of) the
// existing directory guard.
//
// Tests in this file:
//   - TestGenerateAttentionScripts_SymlinkLeafRefused — arm (a): leaf file is
//     a symlink → write refused, victim unchanged.  Affirms: the new leaf guard
//     fires when the FILE (not the directory) is a symlink pointing outside
//     root.
//   - TestGenerateAttentionScripts_LeafGuardCleanPass — arm (b): clean temp
//     tree, no symlinks → call succeeds.  Affirms: the leaf guard does not
//     super-fire on legitimate writes (the fix does not break the happy path).

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateAttentionScripts_SymlinkLeafRefused asserts that ML-3A leaf-level
// containment guards reject GenerateAttentionScripts when the target script
// FILE is itself a symlink pointing outside the project root — even when the
// parent scripts/ directory is clean.
//
// Affirms: the new rejectScaffoldPath(root, signalPath) guard fires when
// trackfw-attention-signal.sh is a symlink, and the victim file outside root
// remains byte-identical after the refused call.
func TestGenerateAttentionScripts_SymlinkLeafRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	// Create a victim file outside the project root.
	victimContent := []byte("victim-content")
	victimPath := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victimPath, victimContent, 0644); err != nil {
		t.Fatalf("creating victim: %v", err)
	}

	// Create the scripts/ directory inside the project root (clean, NOT a symlink).
	scriptsDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("creating scripts dir: %v", err)
	}

	// Place the leaf file trackfw-attention-signal.sh as a symlink pointing to
	// the victim.  This is the attack vector the ML-3A fix closes.
	// symlinkOrSkip detects by condition (os.IsPermission / errno 1314), not by
	// GOOS, and skips the test when symlink creation requires elevated privilege
	// on this platform.
	leafPath := filepath.Join(scriptsDir, "trackfw-attention-signal.sh")
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above the t.Fatalf below
	symlinkOrSkip(t, victimPath, leafPath)

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	err := GenerateAttentionScripts("")
	if err == nil {
		t.Fatal("GenerateAttentionScripts() should refuse when signal script leaf is a symlink, got nil")
	}

	// Victim must remain byte-identical — write must not have gone through the symlink.
	got, readErr := os.ReadFile(victimPath)
	if readErr != nil {
		t.Fatalf("reading victim after refused call: %v", readErr)
	}
	if string(got) != string(victimContent) {
		t.Errorf("containment violated: victim was overwritten\ngot:  %q\nwant: %q", got, victimContent)
	}
}

// TestGenerateAttentionScripts_LeafGuardCleanPass asserts that the leaf-level
// guard introduced in ML-3A does not reject a legitimate write when the target
// tree is clean (no symlinks anywhere).
//
// Affirms: rejectScaffoldPath on the leaf file passes when the file does not
// exist yet or is a regular file — the guard does not super-fire on the happy
// path.
func TestGenerateAttentionScripts_LeafGuardCleanPass(t *testing.T) {
	dir := t.TempDir()

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("GenerateAttentionScripts() in clean tree should succeed, got: %v", err)
	}

	// Both files must have been created.
	for _, name := range []string{"trackfw-attention-signal.sh", "trackfw-attention-cleanup.sh"} {
		p := filepath.Join(dir, "scripts", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist after successful call, got: %v", name, err)
		}
	}
}
