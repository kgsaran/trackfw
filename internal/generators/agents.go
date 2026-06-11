package generators

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed templates/agents
var agentTemplates embed.FS

// InstallAgents instala os 10 agentes da constelação trackfw em ~/.claude/agents/.
// Arquivos já existentes não são sobrescritos (idempotente).
func InstallAgents() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}
	agentsDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("criando %s: %w", agentsDir, err)
	}

	entries, err := fs.ReadDir(agentTemplates, "templates/agents")
	if err != nil {
		return fmt.Errorf("lendo templates de agentes: %w", err)
	}

	installed := 0
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(agentsDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  ✓ ~/.claude/agents/%s (já existe — não sobrescrito)\n", entry.Name())
			skipped++
			continue
		}
		content, err := agentTemplates.ReadFile("templates/agents/" + entry.Name())
		if err != nil {
			return fmt.Errorf("lendo template %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("escrevendo %s: %w", dest, err)
		}
		fmt.Printf("  ✅ ~/.claude/agents/%s\n", entry.Name())
		installed++
	}

	fmt.Printf("\n%d agente(s) instalado(s), %d já existia(m) e não foram alterados.\n", installed, skipped)
	return nil
}
