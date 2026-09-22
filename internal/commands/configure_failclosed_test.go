package commands

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestConfigureCommand_GetWdFailure_FailClosed asserts that configure.go's write guard
// returns an error and writes nothing when configureGetwdFn returns an error —
// fail-closed behaviour (ML-5A).
//
// Reconciliation sentence (Regra Dura de Reconciliação): this test affirms the
// ML-5A conclusion that "precondition failure must be fail-closed" — the configure
// command must return a non-nil error (containing "cannot verify containment") and
// leave trackfw.yaml uncreated when the CWD cannot be determined.
//
// Old-code proof: before the fix, the `if cwdErr == nil` guard block was skipped
// entirely (guard inside if, WriteFile outside), so os.WriteFile ran even when
// Getwd() failed. This test would have failed against the old code:
//   - want: err != nil (refusing write)
//   - got:  err == nil (file written despite Getwd failure)
func TestConfigureCommand_GetWdFailure_FailClosed(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Temporarily override the Getwd seam to inject a failure.
	origFn := configureGetwdFn
	configureGetwdFn = func() (string, error) {
		return "", errors.New("injected: no working directory")
	}
	t.Cleanup(func() { configureGetwdFn = origFn })

	// TERM=dumb enables huh accessible mode (no TTY required).
	t.Setenv("TERM", "dumb")

	cmd := newConfigureCmd()
	cmd.SetIn(strings.NewReader(""))

	execErr := cmd.Execute()
	if execErr == nil {
		t.Fatal("configure command must return an error when Getwd() fails (fail-closed), got nil")
	}
	if !strings.Contains(execErr.Error(), "cannot verify containment") {
		t.Fatalf("error must mention 'cannot verify containment', got: %v", execErr)
	}

	// trackfw.yaml must NOT have been written.
	if _, statErr := os.Stat("trackfw.yaml"); statErr == nil {
		t.Error("containment violated: trackfw.yaml was written even though Getwd() failed")
	}
}
