package generators

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed templates/cursor
var cursorTemplates embed.FS

// InstallCursor installs 10 trackfw rule files in .cursor/rules/ of the current directory.
// Existing files are never overwritten (idempotent).
func InstallCursor() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("obtendo diretório corrente: %w", err)
	}
	rulesDir := filepath.Join(cwd, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", rulesDir, err)
	}

	entries, err := fs.ReadDir(cursorTemplates, "templates/cursor")
	if err != nil {
		return fmt.Errorf("lendo templates cursor: %w", err)
	}

	installed, skipped := 0, 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(rulesDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ .cursor/rules/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := cursorTemplates.ReadFile("templates/cursor/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ .cursor/rules/%s\n", entry.Name())
		installed++
	}

	fmt.Printf("\n%d regra(s) instalada(s), %d já existia(m) e não foram alteradas.\n", installed, skipped)
	return nil
}
