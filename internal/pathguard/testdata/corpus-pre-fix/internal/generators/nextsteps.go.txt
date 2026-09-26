package generators

import (
	"fmt"
	"os"
	"path/filepath"
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
	fmt.Println("Próximo passo — inicie com o guia de arquitetura:")
	fmt.Println()
	for _, t := range detected {
		fmt.Printf("  %s\n", t.cmd)
	}
	fmt.Println()
	fmt.Println("  Execute /trackfw:architect no chat do seu assistente de IA.")
	fmt.Println()
}
