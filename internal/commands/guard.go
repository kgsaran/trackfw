package commands

import (
	"os"

	"github.com/kgsaran/trackfw/internal/guard"
	"github.com/spf13/cobra"
)

// newGuardCmd returns the `trackfw guard` parent command.
// Guard commands exit with code 2 on deny (ADR-2026-10-04 D7).
// Exit-2 is enforced by Execute() in root.go via isCommandUnderGuard.
func newGuardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guard",
		Short: "Hook guards enforced by trackfw",
		Long: `Guard commands are designed to be installed as pre-tool-use hooks.
They read the tool invocation from stdin (JSON payload), extract the command,
and deny operations that are blocked by trackfw governance rules.

Denied commands exit with code 2 (ADR-2026-10-04 D7).`,
	}
	cmd.AddCommand(newGuardGitBranchCmd())
	return cmd
}

// newGuardGitBranchCmd returns the `trackfw guard git-branch` subcommand.
// It is a faithful Go port of scripts/trackfw-git-branch-guard.sh.
func newGuardGitBranchCmd() *cobra.Command {
	var commandFlag string

	cmd := &cobra.Command{
		Use:   "git-branch",
		Short: "Guard that blocks raw git branch/commit/push operations",
		Long: `git-branch reads a hook payload from stdin, extracts the command, and
denies git operations that bypass trackfw governance (commit, push, checkout -b,
switch -c, branch <name>, stash, reset --hard, clean -f, etc.).

Install as a pre-tool-use hook (see docs/cli-parity.md for the hook line).

Allowed  → exit 0 (nothing printed)
Denied   → exit 2, JSON to stdout, reason to stderr
No-op    → exit 0 (outside a trackfw project, or no command found)

Positional arguments are not accepted; pass the command via --command.`,
		// D8: positional args are an error (guard accepts no positional args).
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &guardError{"trackfw guard git-branch: use --command \"<cmd>\" — positional arguments are not accepted (ADR-2026-10-04 D8)"}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := guard.RunGitBranch(
				os.Stdin,
				guard.StdInIdleTimeout,
				commandFlag,
				os.Getenv,
				os.Stdout,
				os.Stderr,
			)
			if code != 0 {
				// Exit immediately with the guard exit code (2 = deny).
				// We use os.Exit directly here rather than returning an error
				// because cobra would otherwise print "Error: exit status 2"
				// to stderr, which must stay clean for the JSON protocol.
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&commandFlag, "command", "", "command to guard (bypasses stdin)")
	return cmd
}

// guardError is a sentinel error type for guard cobra errors (D7: unknown flag,
// unknown subcommand, positional arg passed) so Execute() can exit with 2.
type guardError struct{ msg string }

func (e *guardError) Error() string { return e.msg }
