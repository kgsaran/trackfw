package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// TestConfigureGuard_SymlinkLeafRefused asserts that pathguard.RejectSymlinks
// refuses a write when trackfw.yaml is itself a symlink pointing outside
// the project root — braço (a) for type (i) relative-path sites.
// AC11: RejectSymlinks(root, filepath.Join(root, "trackfw.yaml")) returns a non-nil
// error when trackfw.yaml is a symlink, confirming the library guard fires before any write.
// Note: this test exercises pathguard.RejectSymlinks directly, not configure.go.
// See TestConfigureCommand_SymlinkLeafRefused for the call-site test.
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

// TestConfigureGuard_LegitimateWritePasses asserts that pathguard.RejectSymlinks
// passes (returns nil) when trackfw.yaml lives under a clean project root
// with no symlinks — braço (b): legitimate operation is not blocked.
// AC11: RejectSymlinks(root, filepath.Join(root, "trackfw.yaml")) returns nil for a
// clean path, confirming the library guard does not break normal use.
// Note: this test exercises pathguard.RejectSymlinks directly, not configure.go.
func TestConfigureGuard_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()
	absYAML := filepath.Join(dir, "trackfw.yaml")
	err := pathguard.RejectSymlinks(dir, absYAML)
	if err != nil {
		t.Fatalf("RejectSymlinks should pass for clean path, got: %v", err)
	}
}

// TestConfigureCommand_SymlinkLeafRefused asserts that configure.go's RunE guard —
// not the pathguard library directly — fires before os.WriteFile when trackfw.yaml
// is a symlink pointing outside the project root.
//
// Reconciliation sentence (AC11 / Regra Dura de Reconciliação): this test affirms
// that the guard block inside configure.go's RunE (pathguard.RejectSymlinks +
// the "refusing write" return path, lines 147-157 of configure.go) fires before
// os.WriteFile, proven by the outside target appearing when that block is removed.
//
// Design note: TERM=dumb makes huh.NewForm use accessible mode (no TTY required —
// see huh/form.go). The symlink points to a dangling (nonexistent) target so that
// os.Stat("trackfw.yaml") at configure.go:26 returns an error, skipping the
// "recreate or cancel" form entirely. In accessible mode the main form reads stdin
// as empty lines and keeps all default field values; execution reaches the guard.
func TestConfigureCommand_SymlinkLeafRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	// dangling symlink: outside target does not exist yet.
	// os.Stat in configure.go:26 follows symlinks; dangling → stat fails → skips
	// the "recreate" confirm form, so we go straight to the main form + guard.
	outsideTarget := filepath.Join(outside, "trackfw.yaml")
	symlinkOrSkipCmd(t, outsideTarget, filepath.Join(dir, "trackfw.yaml"))

	// TERM=dumb enables huh accessible mode, which works without a terminal.
	t.Setenv("TERM", "dumb")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newConfigureCmd()
	// Feed empty input so accessible-mode fields get their Go-initialized defaults.
	cmd.SetIn(strings.NewReader(""))

	execErr := cmd.Execute()

	// The guard must fire and refuse — err must name the refusal explicitly.
	if execErr == nil {
		t.Fatal("configure command must return an error when trackfw.yaml is a symlink outside root, got nil")
	}
	if !strings.Contains(execErr.Error(), "refusing write") {
		t.Fatalf("error must mention 'refusing write', got: %v", execErr)
	}

	// The symlink target outside the project must NOT have been created.
	if _, statErr := os.Stat(outsideTarget); statErr == nil {
		t.Error("containment violated: file was written to symlink target outside project root")
	}
}
