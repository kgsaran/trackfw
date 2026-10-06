package commands

import (
	"fmt"
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
		// D7: unknown subcommand passed as positional arg (e.g. cmd.exe tokenises
		// "trackfw guard git-branch;" as argv: guard, "git-branch;", ...).
		// Without Args + RunE, cobra swallows flag.ErrHelp and exits 0 silently.
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &guardError{fmt.Sprintf(
					"trackfw guard: unknown subcommand %q — use `trackfw guard git-branch` or `trackfw guard credential`",
					args[0],
				)}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return &guardError{
				"trackfw guard: requires a subcommand (git-branch or credential)",
			}
		},
	}
	cmd.AddCommand(newGuardGitBranchCmd())
	cmd.AddCommand(newGuardCredentialCmd())
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

// newGuardCredentialCmd returns the `trackfw guard credential` subcommand.
// It is a faithful Go port of scripts/trackfw-credential-guard.sh (project scope).
// With --global it ports globalCredentialGuardScript (DEFAULT_MODE=block, fixed
// docs/roadmaps, no trackfw.yaml required).
func newGuardCredentialCmd() *cobra.Command {
	var globalFlag bool

	cmd := &cobra.Command{
		Use:   "credential",
		Short: "Guard that blocks credential patterns in tool payloads",
		Long: `credential reads a hook payload from stdin, scans for JWT or AWS access key
patterns, and either blocks (exit 2) or emits a warning (exit 0 with attention signal)
depending on credential_guard.mode in trackfw.yaml.

Install as a pre-tool-use or post-tool-use hook (see docs/cli-parity.md).

Allowed/warn  → exit 0 (nothing printed on stdout)
Blocked       → exit 2, message to stderr
No-op         → exit 0 (no credential found; project scope: also outside a trackfw project)

Use --global for the global scope variant:
  - runs in any directory (no trackfw.yaml required)
  - default mode is "block" (vs "warn" in project scope)
  - writes attention JSON only to docs/roadmaps if that directory already exists

Positional arguments are not accepted.`,
		// D8: positional args are an error (guard accepts no positional args).
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &guardError{"trackfw guard credential: positional arguments are not accepted (ADR-2026-10-04 D8)"}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			var code int
			if globalFlag {
				code = guard.RunCredentialGlobal(os.Stdin, os.Stdout, os.Stderr)
			} else {
				code = guard.RunCredential(os.Stdin, os.Stdout, os.Stderr)
			}
			if code != 0 {
				// Exit immediately with the guard exit code (2 = deny).
				// We use os.Exit directly here rather than returning an error
				// because cobra would otherwise print "Error: exit status 2"
				// to stderr, which must stay clean for the credential guard protocol.
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&globalFlag, "global", false, "run as global scope guard (no trackfw.yaml required, default mode block, fixed docs/roadmaps dir)")
	return cmd
}

// guardError is a sentinel error type for guard cobra errors (D7: unknown flag,
// unknown subcommand, positional arg passed) so Execute() can exit with 2.
type guardError struct{ msg string }

func (e *guardError) Error() string { return e.msg }
