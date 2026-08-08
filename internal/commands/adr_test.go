package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// setHomeAndCwd isola HOME e cwd em diretórios de fixture (t.TempDir()), restaurando ambos
// ao fim do teste. Nunca usa o $HOME real da máquina — mesmo padrão de
// internal/generators/scaffold_test.go (TestInstallSkills_*).
func setHomeAndCwd(t *testing.T) (cwd, home string) {
	t.Helper()
	cwd = t.TempDir()
	home = t.TempDir()

	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	origHome := os.Getenv("HOME")

	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("setenv HOME: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origCwd)
		_ = os.Setenv("HOME", origHome)
	})

	return cwd, home
}

// TestADRNew_ScopeGlobal_NoProjectRequired — `adr new --scope global` cria o arquivo em
// $HOME/.trackfw/adr/ mesmo sem trackfw.yaml no cwd (mesmo padrão de UpdateHarness).
func TestADRNew_ScopeGlobal_NoProjectRequired(t *testing.T) {
	_, home := setHomeAndCwd(t)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetIn(bytes.NewReader(nil)) // stdin não-TTY: wizard interativo não roda
	root.SetArgs([]string{"adr", "new", "Decisao Global de Teste", "--scope", "global"})
	if err := root.Execute(); err != nil {
		t.Fatalf("adr new --scope global: erro inesperado: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(home, ".trackfw", "adr", "*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em $HOME/.trackfw/adr, obteve %d: %v", len(matches), matches)
	}

	// Confirma que nada foi escrito em docs/adr (escopo project) do cwd.
	if _, err := os.Stat("docs/adr"); err == nil {
		t.Errorf("docs/adr não deveria existir no cwd quando --scope global é usado")
	}

	// Confirma que trackfw.yaml nunca foi exigido/criado no cwd.
	if _, err := os.Stat("trackfw.yaml"); err == nil {
		t.Errorf("trackfw.yaml não deveria existir/ser exigido no cwd para --scope global")
	}
}

// TestADRNew_ScopeProject_Default — sem --scope (ou --scope project explícito), o
// comportamento é idêntico ao anterior à introdução da flag: escreve em docs/adr no cwd.
func TestADRNew_ScopeProject_Default(t *testing.T) {
	setHomeAndCwd(t)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetIn(bytes.NewReader(nil))
	root.SetArgs([]string{"adr", "new", "Decisao de Projeto"})
	if err := root.Execute(); err != nil {
		t.Fatalf("adr new (default scope): erro inesperado: %v", err)
	}

	matches, err := filepath.Glob("docs/adr/*.md")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/adr, obteve %d: %v", len(matches), matches)
	}
}

// TestADRNew_ScopeProject_Explicit — --scope project explícito é idêntico ao default.
func TestADRNew_ScopeProject_Explicit(t *testing.T) {
	setHomeAndCwd(t)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetIn(bytes.NewReader(nil))
	root.SetArgs([]string{"adr", "new", "Decisao de Projeto Explicita", "--scope", "project"})
	if err := root.Execute(); err != nil {
		t.Fatalf("adr new --scope project: erro inesperado: %v", err)
	}

	matches, err := filepath.Glob("docs/adr/*.md")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/adr, obteve %d: %v", len(matches), matches)
	}
}

// TestADRNew_ScopeInvalid_ErroClaro — valor inválido de --scope retorna erro claro,
// sem escrever nenhum arquivo.
func TestADRNew_ScopeInvalid_ErroClaro(t *testing.T) {
	setHomeAndCwd(t)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetIn(bytes.NewReader(nil))
	root.SetArgs([]string{"adr", "new", "Decisao Invalida", "--scope", "banana"})
	err := root.Execute()
	if err == nil {
		t.Fatal("esperava erro para --scope inválido, obteve nil")
	}

	matches, _ := filepath.Glob("docs/adr/*.md")
	if len(matches) != 0 {
		t.Errorf("nenhum ADR deveria ter sido criado com --scope inválido, obteve: %v", matches)
	}
}

// TestADRList_ScopeGlobal — `adr list --scope global` lista os ADRs de $HOME/.trackfw/adr,
// não os de docs/adr do cwd.
func TestADRList_ScopeGlobal(t *testing.T) {
	_, home := setHomeAndCwd(t)

	globalDir := filepath.Join(home, ".trackfw", "adr")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	globalFile := filepath.Join(globalDir, "ADR-2026-08-08-teste-global.md")
	if err := os.WriteFile(globalFile, []byte("# ADR: Teste Global\n\n> Date: 2026-08-08 | Status: Proposed\n"), 0644); err != nil {
		t.Fatalf("write global adr: %v", err)
	}

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"adr", "list", "--scope", "global"})
	if err := root.Execute(); err != nil {
		t.Fatalf("adr list --scope global: erro inesperado: %v", err)
	}
}
