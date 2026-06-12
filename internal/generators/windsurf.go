package generators

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/windsurf
var windsurfTemplates embed.FS

// InstallWindsurf installs Windsurf rules and workflows in .windsurf/ of the current directory,
// and appends trackfw governance to the global Windsurf rules (idempotent).
func InstallWindsurf() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("obtendo diretório corrente: %w", err)
	}

	installed, skipped := 0, 0

	// rules → .windsurf/rules/
	rulesDir := filepath.Join(cwd, ".windsurf", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", rulesDir, err)
	}
	ruleEntries, err := fs.ReadDir(windsurfTemplates, "templates/windsurf/rules")
	if err != nil {
		return fmt.Errorf("lendo templates windsurf/rules: %w", err)
	}
	for _, entry := range ruleEntries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(rulesDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ .windsurf/rules/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := windsurfTemplates.ReadFile("templates/windsurf/rules/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template rules/%s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ .windsurf/rules/%s\n", entry.Name())
		installed++
	}

	// workflows → .windsurf/workflows/
	workflowsDir := filepath.Join(cwd, ".windsurf", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", workflowsDir, err)
	}
	workflowEntries, err := fs.ReadDir(windsurfTemplates, "templates/windsurf/workflows")
	if err != nil {
		return fmt.Errorf("lendo templates windsurf/workflows: %w", err)
	}
	for _, entry := range workflowEntries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(workflowsDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ .windsurf/workflows/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := windsurfTemplates.ReadFile("templates/windsurf/workflows/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template workflows/%s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ .windsurf/workflows/%s\n", entry.Name())
		installed++
	}

	// global_rules.md → append to ~/.codeium/windsurf/memories/global_rules.md
	if err := appendWindsurfGlobalRules(); err != nil {
		return err
	}

	fmt.Printf("\n%d arquivo(s) instalado(s), %d já existia(m) e não foram alterados.\n", installed, skipped)
	return nil
}

func appendWindsurfGlobalRules() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}
	globalPath := filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md")

	appendContent, err := windsurfTemplates.ReadFile("templates/windsurf/global_rules_append.md")
	if err != nil {
		return fmt.Errorf("lendo global_rules_append.md: %w", err)
	}

	existing, readErr := os.ReadFile(globalPath)
	if readErr == nil && strings.Contains(string(existing), "trackfw") {
		fmt.Printf("  ✓ ~/.codeium/windsurf/memories/global_rules.md (trackfw já presente — não modificado)\n")
		return nil
	}

	if readErr != nil {
		// File doesn't exist — create directory and write from scratch
		if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
			return fmt.Errorf("criando diretório global rules: %w", err)
		}
		if err := os.WriteFile(globalPath, appendContent, 0644); err != nil {
			return fmt.Errorf("escrevendo global_rules.md: %w", err)
		}
	} else {
		// File exists but doesn't contain trackfw — append with separator
		f, err := os.OpenFile(globalPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("abrindo global_rules.md para append: %w", err)
		}
		defer f.Close()
		separator := "\n\n---\n\n"
		if _, err := f.WriteString(separator + string(appendContent)); err != nil {
			return fmt.Errorf("fazendo append em global_rules.md: %w", err)
		}
	}

	fmt.Printf("  ✅ ~/.codeium/windsurf/memories/global_rules.md (trackfw adicionado)\n")
	return nil
}
