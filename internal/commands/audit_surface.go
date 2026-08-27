package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/auditsurface"
	"github.com/spf13/cobra"
)

func newAuditSurfaceCmd() *cobra.Command {
	var jsonOut bool
	var base string

	cmd := &cobra.Command{
		Use:   "audit-surface <ref>",
		Short: "Report the executable surface of a git ref without checking it out",
		Long: `trackfw audit-surface reads a git ref's hook wiring and instruction files
directly from the git object database — no checkout, no worktree modification.
This lets the maintainer audit a PR ref BEFORE making it part of the local tree,
closing the window from "checkout → first agent tool use" to zero.

The report covers:
  · hook wiring in all 8 project-scope agent runtimes (claude, codex, gemini,
    copilot, cursor, kiro, windsurf, amazonq) — always scanned; absence is
    information, not an exclusion
  · the (trigger, matcher, script-path, script-digest) tuple for each hook
    entry — any component changing is a surface change (AC14)
  · instruction files (CLAUDE.md, AGENTS.md, slash commands) — labelled
    distinct from shell scripts because they instruct, not execute (AC15)
  · lifecycle hooks (npm preinstall/postinstall/prepare, .husky/pre-commit)

The command NEVER judges whether a script is hostile. It names what executes;
the maintainer decides whether it is safe.

Example:
  git fetch origin pull/42/head
  trackfw audit-surface FETCH_HEAD`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			gitRoot, err := findGitRoot()
			if err != nil {
				return fmt.Errorf("audit-surface: not inside a git repository: %w", err)
			}

			// Validate ref resolves.
			if err := validateRef(ref, gitRoot); err != nil {
				return fmt.Errorf("audit-surface: ref %q does not resolve: %w", ref, err)
			}

			opts := auditsurface.Options{
				Ref:     ref,
				Base:    base,
				GitRoot: gitRoot,
			}
			report, err := auditsurface.RunAuditSurface(opts)
			if err != nil {
				return fmt.Errorf("audit-surface: %w", err)
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(report)
			}

			fmt.Fprint(cmd.OutOrStdout(), auditsurface.FormatText(report))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit report as JSON instead of text")
	cmd.Flags().StringVar(&base, "base", "", "Base ref for Makefile/CI diff (optional; e.g. HEAD, main)")
	return cmd
}

// findGitRoot returns the absolute path of the git repository root.
func findGitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// validateRef checks that the ref resolves to a commit object in this repository.
// Using ^{commit} ensures a 40-hex SHA from another repository is rejected (F3 fix).
func validateRef(ref, gitRoot string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", ref+"^{commit}")
	cmd.Dir = gitRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// findGitRootFromPath walks up from cwd looking for .git.
// Used as fallback when git is not in PATH (test environments).
func findGitRootFromPath(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
