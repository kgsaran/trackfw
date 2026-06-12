package generators

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed templates/copilot
var copilotTemplates embed.FS

// InstallCopilot installs GitHub Copilot instruction files in .github/ of the current directory.
// Existing files are never overwritten (idempotent).
func InstallCopilot() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("obtendo diretório corrente: %w", err)
	}

	installed, skipped := 0, 0

	// copilot-instructions.md → .github/copilot-instructions.md
	githubDir := filepath.Join(cwd, ".github")
	if err := os.MkdirAll(githubDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", githubDir, err)
	}
	dest := filepath.Join(githubDir, "copilot-instructions.md")
	if _, err := os.Stat(dest); err == nil {
		fmt.Printf("  ✓ .github/copilot-instructions.md (já existe — não sobrescrito)\n")
		skipped++
	} else {
		content, err := copilotTemplates.ReadFile("templates/copilot/copilot-instructions.md")
		if err != nil {
			return fmt.Errorf("lendo template copilot-instructions.md: %w", err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo copilot-instructions.md: %w", err)
		}
		fmt.Printf("  ✅ .github/copilot-instructions.md\n")
		installed++
	}

	// .instructions.md files → .github/instructions/
	instructionsDir := filepath.Join(githubDir, "instructions")
	if err := os.MkdirAll(instructionsDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", instructionsDir, err)
	}
	instrEntries, err := fs.ReadDir(copilotTemplates, "templates/copilot/instructions")
	if err != nil {
		return fmt.Errorf("lendo templates copilot/instructions: %w", err)
	}
	for _, entry := range instrEntries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(instructionsDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ .github/instructions/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := copilotTemplates.ReadFile("templates/copilot/instructions/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template instructions/%s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ .github/instructions/%s\n", entry.Name())
		installed++
	}

	// .prompt.md files → .github/prompts/
	promptsDir := filepath.Join(githubDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", promptsDir, err)
	}
	promptEntries, err := fs.ReadDir(copilotTemplates, "templates/copilot/prompts")
	if err != nil {
		return fmt.Errorf("lendo templates copilot/prompts: %w", err)
	}
	for _, entry := range promptEntries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(promptsDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ .github/prompts/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := copilotTemplates.ReadFile("templates/copilot/prompts/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template prompts/%s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ .github/prompts/%s\n", entry.Name())
		installed++
	}

	fmt.Printf("\n%d arquivo(s) instalado(s), %d já existia(m) e não foram alterados.\n", installed, skipped)
	return nil
}
