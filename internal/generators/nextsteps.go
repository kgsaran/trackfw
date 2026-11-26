package generators

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type toolEntry struct {
	file string
	cmd  string
}

// PrintArchitectNextSteps exibe instruções de próximo passo após init/update.
func PrintArchitectNextSteps(cwd string) {
	candidates := []toolEntry{
		{"CLAUDE.md", "claude"},
		{".cursor/rules/trackfw.mdc", "cursor ."},
		{".windsurfrules", "windsurf ."},
		{".github/copilot-instructions.md", "code . (Copilot)"},
		{".amazonq/developer/guidelines.md", "code . (Amazon Q)"},
		{"GEMINI.md", "gemini"},
		{"AGENTS.md", "codex"},
	}

	var detected []toolEntry
	for _, t := range candidates {
		if _, err := os.Stat(filepath.Join(cwd, t.file)); err == nil {
			detected = append(detected, t)
		}
	}
	if len(detected) == 0 {
		detected = []toolEntry{{"", "claude"}}
	}

	fmt.Println()
	fmt.Println("Next step — start with the architecture guide:")
	fmt.Println()
	for _, t := range detected {
		fmt.Printf("  %s\n", t.cmd)
	}
	fmt.Println()
	fmt.Println("  Run /trackfw:architect in your AI assistant chat.")
	fmt.Println()
}

// PrintBootstrapSteps prints the bootstrap steps for a freshly initialized
// repository to w. Called by init when git init was just run. Repos that
// already had a work tree do not receive this output.
// D5 (ADR-2026-11-26): every surface that agents read describes the bootstrap.
//
// paths contains the top-level paths reported by `git status --porcelain` after
// scaffold completed — init passes the real list so the agent sees exact paths.
// When paths is empty (e.g. the mock tests that stub initGitExecFn), the
// placeholder form is used instead.
func PrintBootstrapSteps(w io.Writer, paths []string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Bootstrap steps — commit the scaffold and publish the default branch:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  git status --short")
	if len(paths) > 0 {
		fmt.Fprintf(w, "  git add %s\n", strings.Join(paths, " "))
	} else {
		fmt.Fprintln(w, "  git add <each path listed by git status --short>")
	}
	fmt.Fprintln(w, "  trackfw commit -m \"chore(scaffold): initial\"")
	fmt.Fprintln(w, "  trackfw push")
	fmt.Fprintln(w)
}
