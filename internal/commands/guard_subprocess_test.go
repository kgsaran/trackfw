package commands

// guard_subprocess_test.go — subprocess tests for `trackfw guard` parent command.
//
// C1 fix (ADR-2026-10-04 D7): newGuardCmd() now has Args + RunE so that an
// unknown positional arg (e.g. cmd.exe tokenises "trackfw guard git-branch;" as
// argv: guard, "git-branch;") exits with code 2, not 0.
//
// These tests build the real binary and assert the exact exit code.
// Mut-3 from the red team (exitCode 2 → 1 in root.go) is killed by any test
// asserting rc == 2 exactly (not just rc != 0).

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	guardBinaryOnce sync.Once
	guardBinaryPath string
	guardBinaryErr  error
)

// guardBinary builds the trackfw binary once per test run and returns its path.
func guardBinary(t *testing.T) string {
	t.Helper()
	guardBinaryOnce.Do(func() {
		projRoot := barrierFindProjectRoot(t) // reuses barrier_contract_test.go helper
		binDir, err := os.MkdirTemp("", "trackfw-guard-bin-")
		if err != nil {
			guardBinaryErr = err
			return
		}
		bin := filepath.Join(binDir, testBinaryName("trackfw"))
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/trackfw")
		cmd.Dir = projRoot
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			guardBinaryErr = &buildBinaryError{msg: string(out), cause: buildErr}
			return
		}
		guardBinaryPath = bin
	})
	if guardBinaryErr != nil {
		t.Fatalf("could not build trackfw binary: %v", guardBinaryErr)
	}
	return guardBinaryPath
}

// buildBinaryError carries the combined output of a failed go build.
type buildBinaryError struct {
	msg   string
	cause error
}

func (e *buildBinaryError) Error() string {
	return "go build failed: " + e.cause.Error() + "\n" + e.msg
}

// guardExitCode runs the binary with the given args (no stdin) and returns the exit code.
func guardExitCode(t *testing.T, args ...string) int {
	t.Helper()
	bin := guardBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Stdin = nil
	err := cmd.Run()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	t.Fatalf("unexpected error running %v: %v", args, err)
	return -1
}

// TestGuardParentCmd_UnknownSubcommand_ExitsTwo asserts that an unknown positional
// arg to `trackfw guard` exits with code 2, not 0 (C1 fix).
// Assertion: newGuardCmd() Args rejects unknown positional args as *guardError,
// which root.go routes to exit 2 via isCommandUnderGuard — not cobra's default
// exit 0 (swallowed flag.ErrHelp) that existed before the fix.
func TestGuardParentCmd_UnknownSubcommand_ExitsTwo(t *testing.T) {
	rc := guardExitCode(t, "guard", "nao-existe")
	if rc != 2 {
		t.Fatalf("trackfw guard nao-existe: want exit 2 (C1 fix), got %d", rc)
	}
}

// TestGuardParentCmd_SemicolonArg_ExitsTwo asserts that `trackfw guard 'git-branch;'`
// exits with code 2. This is the cmd.exe tokenisation scenario: the hook line
// "trackfw guard git-branch; exit $LASTEXITCODE" is split by cmd.exe into argv
// [guard, "git-branch;", exit, $LASTEXITCODE]; the second argv is "git-branch;"
// which is an unknown subcommand.
// Assertion: C1 fix blocks the cmd.exe tokenisation fallback — the parent guard
// rejects "git-branch;" as a positional arg and exits 2, not 0.
func TestGuardParentCmd_SemicolonArg_ExitsTwo(t *testing.T) {
	rc := guardExitCode(t, "guard", "git-branch;")
	if rc != 2 {
		t.Fatalf("trackfw guard 'git-branch;': want exit 2 (C1 cmd.exe scenario), got %d", rc)
	}
}

// TestGuardParentCmd_NoSubcommand_ExitsTwo asserts that `trackfw guard` without a
// subcommand exits with code 2 (RunE now returns a *guardError).
// Assertion: before the fix cobra printed help and exited 0; with RunE the parent
// guard command requires a subcommand and exits 2 via isCommandUnderGuard.
func TestGuardParentCmd_NoSubcommand_ExitsTwo(t *testing.T) {
	rc := guardExitCode(t, "guard")
	if rc != 2 {
		t.Fatalf("trackfw guard (no subcommand): want exit 2, got %d", rc)
	}
}

// TestGuardParentCmd_HelpFlag_ExitsZero asserts that `trackfw guard --help` exits 0.
// Assertion: --help is handled by cobra before RunE/Args; the sonda guardRunProbe
// (guard --help → 0) that the validator uses to detect the guard capability must
// remain unaffected by the C1 fix.
func TestGuardParentCmd_HelpFlag_ExitsZero(t *testing.T) {
	rc := guardExitCode(t, "guard", "--help")
	if rc != 0 {
		t.Fatalf("trackfw guard --help: want exit 0 (probe must stay alive), got %d", rc)
	}
}

// TestGuardParentCmd_ExitCodeIsExactlyTwo_KillsMut3 asserts that the exit code for
// an unknown subcommand is exactly 2, not 1. This kills Mut-3 from the red team
// (changing exitCode = 2 to exitCode = 1 in root.go).
// Assertion: root.go's isCommandUnderGuard branch sets exitCode = 2 (not 1);
// rc == 2 exactly discriminates the guard deny code from a generic error (rc == 1).
func TestGuardParentCmd_ExitCodeIsExactlyTwo_KillsMut3(t *testing.T) {
	rc := guardExitCode(t, "guard", "any-unknown")
	if rc == 0 {
		t.Fatal("expected non-zero exit for unknown subcommand, got 0")
	}
	if rc != 2 {
		t.Fatalf("exit code must be exactly 2 (D7 / Mut-3 gate), got %d", rc)
	}
}
