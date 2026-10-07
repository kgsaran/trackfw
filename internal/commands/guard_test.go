package commands

import (
	"strings"
	"testing"
)

// TestGuardCmd_Registered asserts that `trackfw guard`, `trackfw guard git-branch`,
// and `trackfw guard credential` are registered in the command tree.
// Assertion: newGuardCmd() is wired into newRootCmd() with both subcommands.
func TestGuardCmd_Registered(t *testing.T) {
	root := newRootCmd()
	var guardCmd, gitBranchCmd, credentialCmd *string
	for _, c := range root.Commands() {
		if c.Name() == "guard" {
			name := c.Name()
			guardCmd = &name
			for _, gc := range c.Commands() {
				switch gc.Name() {
				case "git-branch":
					sub := gc.Name()
					gitBranchCmd = &sub
				case "credential":
					sub := gc.Name()
					credentialCmd = &sub
				}
			}
		}
	}
	if guardCmd == nil {
		t.Fatal("root command does not have 'guard' subcommand")
	}
	if gitBranchCmd == nil {
		t.Fatal("guard command does not have 'git-branch' subcommand")
	}
	if credentialCmd == nil {
		t.Fatal("guard command does not have 'credential' subcommand")
	}
}

// TestGuardCmd_PositionalArg_IsGuardError asserts that passing a positional arg to
// `trackfw guard git-branch` yields a *guardError (D7 gate i / D8).
// Assertion: D8 — positional arg → guardError, so Execute() will exit with 2 not 1.
func TestGuardCmd_PositionalArg_IsGuardError(t *testing.T) {
	root := newRootCmd()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"guard", "git-branch", "some-positional-arg"})
	cmd, err := root.ExecuteC()
	if err == nil {
		t.Fatal("expected error for positional arg, got nil")
	}
	if !isCommandUnderGuard(cmd) {
		t.Fatalf("command %q should be recognized as under guard", cmd.CommandPath())
	}
	if _, ok := err.(*guardError); !ok {
		t.Fatalf("expected *guardError, got %T: %v", err, err)
	}
}

// TestIsCommandUnderGuard_GuardSubtree asserts that commands under "trackfw guard"
// return true from isCommandUnderGuard.
// Assertion: D7 exit-2 is applied exactly to the guard subtree.
func TestIsCommandUnderGuard_GuardSubtree(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Name() == "guard" {
			if !isCommandUnderGuard(c) {
				t.Errorf("guard cmd should be under guard, path=%q", c.CommandPath())
			}
			for _, gc := range c.Commands() {
				if !isCommandUnderGuard(gc) {
					t.Errorf("guard subcommand %q should be under guard", gc.CommandPath())
				}
			}
		}
	}
}

// TestIsCommandUnderGuard_NonGuard asserts that non-guard commands return false.
// Assertion: exit-1 is preserved for all commands outside the guard subtree.
func TestIsCommandUnderGuard_NonGuard(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Name() == "guard" {
			continue
		}
		if isCommandUnderGuard(c) {
			t.Errorf("command %q should NOT be under guard", c.CommandPath())
		}
	}
	if isCommandUnderGuard(root) {
		t.Error("root should not be under guard")
	}
}

// TestGuardCredentialCmd_PositionalArg_IsGuardError asserts that passing a positional
// arg to `trackfw guard credential` yields a *guardError (D7 gate / D8).
// Assertion: D8 — positional arg to `credential` → guardError → Execute() exits with 2.
func TestGuardCredentialCmd_PositionalArg_IsGuardError(t *testing.T) {
	root := newRootCmd()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"guard", "credential", "some-positional-arg"})
	cmd, err := root.ExecuteC()
	if err == nil {
		t.Fatal("expected error for positional arg, got nil")
	}
	if !isCommandUnderGuard(cmd) {
		t.Fatalf("command %q should be recognized as under guard", cmd.CommandPath())
	}
	if _, ok := err.(*guardError); !ok {
		t.Fatalf("expected *guardError, got %T: %v", err, err)
	}
}

// TestGuardCredentialCmd_GlobalFlagRegistered asserts that the `credential` subcommand
// has a --global boolean flag registered.
// Assertion: newGuardCredentialCmd() registers --global so cobra routes it without error.
func TestGuardCredentialCmd_GlobalFlagRegistered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Name() != "guard" {
			continue
		}
		for _, gc := range c.Commands() {
			if gc.Name() != "credential" {
				continue
			}
			f := gc.Flags().Lookup("global")
			if f == nil {
				t.Fatal("credential subcommand missing --global flag")
			}
			if f.Value.Type() != "bool" {
				t.Fatalf("--global flag type=%q, want bool", f.Value.Type())
			}
			return
		}
	}
	t.Fatal("credential subcommand not found")
}

// TestGuardErrorType asserts that guardError implements the error interface
// and carries its message verbatim.
// Assertion: guardError is the sentinel used by Execute() to detect D7 exits.
func TestGuardErrorType(t *testing.T) {
	msg := "test guard error"
	err := &guardError{msg: msg}
	if err.Error() != msg {
		t.Fatalf("guardError.Error() = %q, want %q", err.Error(), msg)
	}
	// Ensure it satisfies the error interface (compile-time check via interface).
	var _ error = err
	_ = strings.Contains(err.Error(), "guard")
}
