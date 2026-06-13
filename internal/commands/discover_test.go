package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers compartilhados

func mustMkdirCmd(t *testing.T, base, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, rel), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
}

func mustWriteFileCmd(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// runDiscover executa o comando discover num diretório temporário com os flags fornecidos.
// Faz chdir para tmpDir e restaura o diretório original ao finalizar.
func runDiscover(t *testing.T, tmpDir string, flags ...string) (string, error) {
	t.Helper()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	cmd := NewDiscoverCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	args := append([]string{}, flags...)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return buf.String(), err
}

// TestDiscoverReport verifica que o relatório inclui os caminhos e contagens detectadas.
func TestDiscoverReport(t *testing.T) {
	dir := t.TempDir()

	// estrutura com ADRs, REQs e Roadmaps
	mustMkdirCmd(t, dir, "docs/adr")
	mustMkdirCmd(t, dir, "docs/req")
	mustMkdirCmd(t, dir, "docs/roadmaps/wip")
	mustMkdirCmd(t, dir, "docs/roadmaps/done")

	mustWriteFileCmd(t, filepath.Join(dir, "docs/adr/ADR-001.md"), "# ADR 001")
	mustWriteFileCmd(t, filepath.Join(dir, "docs/adr/ADR-002.md"), "# ADR 002")
	mustWriteFileCmd(t, filepath.Join(dir, "docs/req/REQ-001.md"), "# REQ 001")
	mustWriteFileCmd(t, filepath.Join(dir, "docs/roadmaps/done/ROADMAP-001.md"), "# Roadmap")

	out, err := runDiscover(t, dir)
	if err != nil {
		t.Fatalf("discover command failed: %v", err)
	}

	// verificar que o relatório menciona o caminho ADR
	if !strings.Contains(out, "docs/adr") {
		t.Errorf("output should mention ADR dir 'docs/adr'; got:\n%s", out)
	}

	// verificar que a contagem de ADRs está correta (2)
	if !strings.Contains(out, "2") {
		t.Errorf("output should mention ADR count 2; got:\n%s", out)
	}

	// verificar que o caminho REQ está presente
	if !strings.Contains(out, "docs/req") {
		t.Errorf("output should mention REQ dir 'docs/req'; got:\n%s", out)
	}

	// verificar que o caminho roadmap está presente
	if !strings.Contains(out, "docs/roadmaps") {
		t.Errorf("output should mention roadmap dir 'docs/roadmaps'; got:\n%s", out)
	}

	// verificar que o score aparece no relatório
	if !strings.Contains(out, "Governance Score:") {
		t.Errorf("output should contain Governance Score; got:\n%s", out)
	}
}

// TestDiscoverInit verifica que trackfw.yaml é gerado com os campos corretos
// e que não sobrescreve um arquivo existente.
func TestDiscoverInit(t *testing.T) {
	t.Run("gera yaml novo com by_agent e requisicoes", func(t *testing.T) {
		dir := t.TempDir()

		// estrutura by_agent com docs/requisições
		mustMkdirCmd(t, dir, "docs/requisições")
		mustMkdirCmd(t, dir, "docs/roadmaps/zeus/wip")
		mustMkdirCmd(t, dir, "docs/roadmaps/apolo/done")
		mustMkdirCmd(t, dir, "docs/adr")

		mustWriteFileCmd(t, filepath.Join(dir, "docs/requisições/REQ-001.md"), "# REQ")
		mustWriteFileCmd(t, filepath.Join(dir, "docs/roadmaps/zeus/wip/R-001.md"), "# R")

		out, err := runDiscover(t, dir, "--init")
		if err != nil {
			t.Fatalf("discover --init failed: %v", err)
		}

		// verificar mensagem de sucesso
		if !strings.Contains(out, "trackfw.yaml generated") {
			t.Errorf("output should confirm generation; got:\n%s", out)
		}

		// verificar conteúdo do yaml gerado
		yamlPath := filepath.Join(dir, "trackfw.yaml")
		content, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("trackfw.yaml not created: %v", err)
		}

		yamlStr := string(content)
		if !strings.Contains(yamlStr, "governance_mode: lenient") {
			t.Errorf("yaml should contain 'governance_mode: lenient'; got:\n%s", yamlStr)
		}
		if !strings.Contains(yamlStr, "docs/requisições") {
			t.Errorf("yaml should contain req_dir with 'docs/requisições'; got:\n%s", yamlStr)
		}
		if !strings.Contains(yamlStr, "roadmap_namespacing: by_agent") {
			t.Errorf("yaml should contain 'roadmap_namespacing: by_agent'; got:\n%s", yamlStr)
		}
	})

	t.Run("nao sobrescreve yaml existente", func(t *testing.T) {
		dir := t.TempDir()

		// pré-criar trackfw.yaml com conteúdo sentinela
		yamlPath := filepath.Join(dir, "trackfw.yaml")
		sentinel := "# conteúdo original — não deve ser sobrescrito\n"
		mustWriteFileCmd(t, yamlPath, sentinel)

		out, err := runDiscover(t, dir, "--init")
		if err != nil {
			t.Fatalf("discover --init failed: %v", err)
		}

		// deve avisar que o arquivo já existe
		if !strings.Contains(out, "already exists") {
			t.Errorf("output should warn that trackfw.yaml already exists; got:\n%s", out)
		}

		// o conteúdo original não deve ter sido alterado
		content, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("trackfw.yaml should still exist: %v", err)
		}
		if string(content) != sentinel {
			t.Errorf("trackfw.yaml should not be overwritten; got:\n%s", string(content))
		}
	})
}

// TestDiscoverBootstrapLog verifica que .trackfw-log é criado com entradas dos arquivos done/
// e que execuções repetidas não duplicam entradas.
func TestDiscoverBootstrapLog(t *testing.T) {
	dir := t.TempDir()

	// estrutura flat com 2 roadmaps em done/
	mustMkdirCmd(t, dir, "docs/roadmaps/done")
	mustWriteFileCmd(t, filepath.Join(dir, "docs/roadmaps/done/ROADMAP-001.md"), "# Roadmap 001")
	mustWriteFileCmd(t, filepath.Join(dir, "docs/roadmaps/done/ROADMAP-002.md"), "# Roadmap 002")

	// primeira execução — deve criar o log com 2 entradas
	out, err := runDiscover(t, dir, "--bootstrap-log")
	if err != nil {
		t.Fatalf("discover --bootstrap-log failed (1ª execução): %v", err)
	}
	if !strings.Contains(out, "bootstrap log written") {
		t.Errorf("output should confirm log was written; got:\n%s", out)
	}

	logPath := filepath.Join(dir, "docs/roadmaps/.trackfw-log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf(".trackfw-log not created: %v", err)
	}

	lines := countNonEmptyLines(string(content))
	if lines != 2 {
		t.Errorf("expected 2 log entries after 1st run, got %d; content:\n%s", lines, string(content))
	}

	// segunda execução — não deve duplicar (dedup)
	_, err = runDiscover(t, dir, "--bootstrap-log")
	if err != nil {
		t.Fatalf("discover --bootstrap-log failed (2ª execução): %v", err)
	}

	content2, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf(".trackfw-log not found after 2nd run: %v", err)
	}

	lines2 := countNonEmptyLines(string(content2))
	if lines2 != 2 {
		t.Errorf("expected 2 log entries after 2nd run (no duplicates), got %d; content:\n%s", lines2, string(content2))
	}
}

// countNonEmptyLines conta linhas não-vazias em uma string.
func countNonEmptyLines(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
