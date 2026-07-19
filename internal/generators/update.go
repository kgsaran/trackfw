package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/integrations"
)

// ReadUpdateConfig lê hooks/ci/backend/frontend/pkg_manager de trackfw.yaml.
// Sem dependências externas — parse linha a linha.
func ReadUpdateConfig(cwd string) Config {
	data, err := os.ReadFile(filepath.Join(cwd, "trackfw.yaml"))
	if err != nil {
		return Config{}
	}
	cfg := Config{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := splitKVupdate(line)
		if !ok {
			continue
		}
		switch key {
		case "hooks":
			cfg.Hooks = val
		case "ci":
			cfg.CI = val
		case "backend":
			cfg.Backend = val
		case "frontend":
			cfg.Frontend = val
		case "pkg_manager":
			cfg.PkgManager = val
		}
	}
	return cfg
}

func splitKVupdate(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	if ci := strings.Index(val, " #"); ci >= 0 {
		val = strings.TrimSpace(val[:ci])
	}
	return key, val, key != ""
}

// Update re-aplica todos os templates atuais do trackfw ao projeto em cwd.
func Update(cwd string) error {
	if _, err := os.Stat(filepath.Join(cwd, "trackfw.yaml")); err != nil {
		return fmt.Errorf("trackfw.yaml não encontrado — execute trackfw init primeiro")
	}

	cfg := ReadUpdateConfig(cwd)

	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		return fmt.Errorf("não foi possível mudar para %s: %w", cwd, err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	fmt.Println("trackfw update — re-aplicando templates atuais...")
	fmt.Println()

	// 1. Regras de agente (categoria 1 — marker-delimited)
	if err := InjectRulesDetected(cwd); err != nil {
		fmt.Printf("  ⚠ agent rules: %v\n", err)
	} else {
		fmt.Println("  ✓ agent rules atualizadas")
	}

	// 1b. Agent hooks (attention signal)
	if err := InjectHooksDetected(cwd); err != nil {
		fmt.Printf("  ⚠ agent hooks: %v\n", err)
	} else {
		fmt.Println("  ✓ agent hooks atualizados")
	}
	_, agentsErr := os.Stat(filepath.Join(cwd, "AGENTS.md"))
	_, codexErr := os.Stat(filepath.Join(cwd, ".codex"))
	if agentsErr == nil || codexErr == nil {
		updateDetectedCodexIntegrations(cwd)
	}
	// 2. Validate script (categoria 2 — trackfw-owned, overwrite seguro)
	if err := generateValidateScript(cfg); err != nil {
		fmt.Printf("  ⚠ validate script: %v\n", err)
	}

	if err := generateAttentionScripts(); err != nil {
		fmt.Printf("  ⚠ attention scripts: %v\n", err)
	}

	// 3. CI workflow (categoria 2 — trackfw-owned, overwrite seguro)
	if err := generateCIWorkflow(cfg); err != nil {
		fmt.Printf("  ⚠ CI workflow: %v\n", err)
	} else if cfg.CI != "" && cfg.CI != "none" {
		fmt.Println("  ✓ CI workflow atualizado")
	}

	// 4. Git hooks — cirúrgico (categoria 3 — shared user files)
	updateHooksSurgical(cfg)

	// 5. Historical Claude auxiliaries remain backward compatible. Canonical
	// agents/skills themselves are managed only by their lifecycle commands.
	if err := ForceGenerateClaudeCommands(); err != nil {
		fmt.Printf("  ⚠ Claude commands: %v\n", err)
	} else {
		fmt.Println("  ✓ .claude/commands/trackfw/ atualizado")
	}
	if err := ForceInstallSkills(); err != nil {
		fmt.Printf("  ⚠ legacy Claude skill: %v\n", err)
	} else {
		fmt.Println("  ✓ legacy Claude skill global atualizada")
	}

	fmt.Println("\n✓ trackfw update concluído")
	PrintArchitectNextSteps(cwd)
	return nil
}

func updateDetectedCodexIntegrations(cwd string) {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		fmt.Printf("  ⚠ Codex integration catalog: %v\n", err)
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("  ⚠ Codex integration home: %v\n", err)
		return
	}
	manager := integrations.Manager{ProjectRoot: cwd, HomeDir: home}
	updated := 0
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		plans, planErr := integrations.BuildPlans(catalog, integrations.PlanRequest{Kind: kind, Targets: []string{"codex"}, Scope: "project"})
		if planErr != nil {
			fmt.Printf("  ⚠ Codex %s plans: %v\n", kind, planErr)
			continue
		}
		for _, plan := range plans {
			inspection, inspectErr := manager.Inspect(plan)
			if inspectErr != nil {
				fmt.Printf("  ⚠ Codex %s/%s inspect: %v\n", kind, plan.Claim.Item, inspectErr)
				continue
			}
			if inspection.State == integrations.StateNotInstalled {
				continue
			}
			if updateErr := manager.Update([]integrations.PlannedArtifact{plan}, false); updateErr != nil {
				fmt.Printf("  ⚠ Codex %s/%s preservado: %v\n", kind, plan.Claim.Item, updateErr)
				continue
			}
			updated++
		}
	}
	if updated > 0 {
		fmt.Printf("  ✓ %d Codex agent/skill artifact(s) migrated or updated\n", updated)
	}
}

// updateHooksSurgical garante que 'trackfw validate' está presente nos hooks sem sobrescrever conteúdo do usuário.
func updateHooksSurgical(cfg Config) {
	switch cfg.Hooks {
	case "husky":
		path := filepath.Join(".husky", "pre-commit")
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "trackfw validate") {
			fmt.Println("  ✓ .husky/pre-commit — trackfw validate já presente")
			return
		}
		os.MkdirAll(".husky", 0755) //nolint:errcheck
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			fmt.Printf("  ⚠ .husky/pre-commit: %v\n", err)
			return
		}
		defer f.Close()
		fmt.Fprintln(f, "\ntrackfw validate")
		fmt.Println("  ✓ .husky/pre-commit — trackfw validate injetado")

	case "lefthook":
		path := "lefthook.yml"
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "trackfw-validate:") || strings.Contains(string(data), "trackfw validate") {
			fmt.Println("  ✓ lefthook.yml — trackfw já presente")
			return
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("  ⚠ lefthook.yml: %v\n", err)
			return
		}
		defer f.Close()
		fmt.Fprintln(f, "\npre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate")
		fmt.Println("  ✓ lefthook.yml — trackfw-validate injetado")
	}
}
