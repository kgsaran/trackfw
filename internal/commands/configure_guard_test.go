package commands

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

// symlinkOrSkipCmd creates a symlink and skips if privilege is unavailable.
// Copied from internal/generators/update_test.go (symlinkOrSkip) — the detection
// is on the CONDITION (a privilege failure), not on runtime.GOOS.
func symlinkOrSkipCmd(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if isSymlinkPrivilegeErrorCmd(err) {
		t.Skipf("guarda de escrita através de symlink não exercitada: criação de symlink exige Developer Mode (ou processo elevado) neste Windows: %v", err)
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
}

func isSymlinkPrivilegeErrorCmd(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == 1314 {
		return true
	}
	return false
}

// TestConfigureGuard_SymlinkLeafRefused asserts that the containment guard used by
// configure.go rejects a write when trackfw.yaml is itself a symlink pointing outside
// the project root — braço (a) for type (i) relative-path sites.
// AC11: RejectSymlinks(root, filepath.Join(root, "trackfw.yaml")) returns a non-nil
// error when trackfw.yaml is a symlink, confirming the guard fires before any write.
func TestConfigureGuard_SymlinkLeafRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	// trackfw.yaml → symlink pointing outside project
	symlinkOrSkipCmd(t, filepath.Join(outside, "trackfw.yaml"), filepath.Join(dir, "trackfw.yaml"))

	absYAML := filepath.Join(dir, "trackfw.yaml")
	err := pathguard.RejectSymlinks(dir, absYAML)
	if err == nil {
		t.Fatal("RejectSymlinks should refuse symlink leaf pointing outside project root, got nil")
	}
	// Nothing should be written outside
	if _, statErr := os.Stat(filepath.Join(outside, "trackfw.yaml")); statErr == nil {
		t.Error("containment violated: file exists at symlink target outside project root")
	}
}

// TestConfigureGuard_LegitimateWritePasses asserts that the containment guard used by
// configure.go passes (returns nil) when trackfw.yaml lives under a clean project root
// with no symlinks — braço (b): legitimate operation is not blocked.
// AC11: RejectSymlinks(root, filepath.Join(root, "trackfw.yaml")) returns nil for a
// clean path, confirming the guard does not break normal configure execution.
func TestConfigureGuard_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()
	absYAML := filepath.Join(dir, "trackfw.yaml")
	err := pathguard.RejectSymlinks(dir, absYAML)
	if err != nil {
		t.Fatalf("RejectSymlinks should pass for clean path, got: %v", err)
	}
}
