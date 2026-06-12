package generators

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/gemini
var geminiTemplates embed.FS

// InstallGemini installs Gemini CLI configuration:
//   - ~/.gemini/GEMINI.md (global governance)
//   - GEMINI.md in current directory (project context)
//   - ~/.gemini/skills/trackfw-<role>/SKILL.md (10 skills)
//   - ~/.gemini/commands/trackfw-*.toml (3 commands)
//
// Existing files are never overwritten (idempotent).
func InstallGemini() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("obtendo diretório corrente: %w", err)
	}

	installed, skipped := 0, 0

	// Global GEMINI.md → ~/.gemini/GEMINI.md
	geminiHomeDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiHomeDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", geminiHomeDir, err)
	}
	globalDest := filepath.Join(geminiHomeDir, "GEMINI.md")
	if _, err := os.Stat(globalDest); err == nil {
		fmt.Printf("  ✓ ~/.gemini/GEMINI.md (já existe — não sobrescrito)\n")
		skipped++
	} else {
		content, err := geminiTemplates.ReadFile("templates/gemini/GEMINI.md")
		if err != nil {
			return fmt.Errorf("lendo template GEMINI.md: %w", err)
		}
		if err := os.WriteFile(globalDest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo ~/.gemini/GEMINI.md: %w", err)
		}
		fmt.Printf("  ✅ ~/.gemini/GEMINI.md\n")
		installed++
	}

	// Project GEMINI.md → $PWD/GEMINI.md
	projectDest := filepath.Join(cwd, "GEMINI.md")
	if _, err := os.Stat(projectDest); err == nil {
		fmt.Printf("  ✓ GEMINI.md (já existe — não sobrescrito)\n")
		skipped++
	} else {
		content, err := geminiTemplates.ReadFile("templates/gemini/GEMINI-project.md")
		if err != nil {
			return fmt.Errorf("lendo template GEMINI-project.md: %w", err)
		}
		if err := os.WriteFile(projectDest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo GEMINI.md: %w", err)
		}
		fmt.Printf("  ✅ GEMINI.md (projeto)\n")
		installed++
	}

	// Skills → ~/.gemini/skills/trackfw-<role>/SKILL.md
	skillEntries, err := fs.ReadDir(geminiTemplates, "templates/gemini/skills")
	if err != nil {
		return fmt.Errorf("lendo templates gemini/skills: %w", err)
	}
	for _, skillDir := range skillEntries {
		if !skillDir.IsDir() {
			continue
		}
		roleName := skillDir.Name()
		skillDestDir := filepath.Join(geminiHomeDir, "skills", roleName)
		if err := os.MkdirAll(skillDestDir, 0755); err != nil {
			return fmt.Errorf("criando %s: %w", skillDestDir, err)
		}
		dest := filepath.Join(skillDestDir, "SKILL.md")
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ ~/.gemini/skills/%s/SKILL.md (já existe — não sobrescrito)\n", roleName)
			skipped++
			continue
		}
		content, err := geminiTemplates.ReadFile("templates/gemini/skills/" + roleName + "/SKILL.md")
		if err != nil {
			return fmt.Errorf("lendo template skills/%s/SKILL.md: %w", roleName, err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ ~/.gemini/skills/%s/SKILL.md\n", roleName)
		installed++
	}

	// Commands → ~/.gemini/commands/trackfw-*.toml
	commandsDir := filepath.Join(geminiHomeDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", commandsDir, err)
	}
	cmdEntries, err := fs.ReadDir(geminiTemplates, "templates/gemini/commands")
	if err != nil {
		return fmt.Errorf("lendo templates gemini/commands: %w", err)
	}
	for _, entry := range cmdEntries {
		if entry.IsDir() {
			continue
		}
		// Only install trackfw-prefixed commands
		if !strings.HasPrefix(entry.Name(), "trackfw-") {
			continue
		}
		dest := filepath.Join(commandsDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ ~/.gemini/commands/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := geminiTemplates.ReadFile("templates/gemini/commands/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template commands/%s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ ~/.gemini/commands/%s\n", entry.Name())
		installed++
	}

	fmt.Printf("\n%d arquivo(s) instalado(s), %d já existia(m) e não foram alterados.\n", installed, skipped)
	return nil
}
