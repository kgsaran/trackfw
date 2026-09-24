package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// projetoByAgent monta um projeto by_agent com a REQ no layout CANONICO
// (req_dir/<agente>/*.md, um nivel — ADR-2026-09-03 D2) e status inicial done.
func projetoByAgent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_namespacing: by_agent\nagents:\n- claude\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("docs", "req", "claude"), 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join("docs", "req", "claude", "REQ-2026-09-23-cobaia-de-move.md")
	body := "---\nstatus: done\ndate: 2026-09-23\n---\n\n# REQ: Cobaia de move\n\n> Date: 2026-09-23 | Status: done\n"
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMoveREQ_ReabreNoLayoutCanonico afirma a conclusao central da medicao da #327:
// no layout canonico de by_agent nada se MOVE — o comando so reescreve o campo —, e
// por isso o vocabulario de estado de roadmap nao se aplica. Sem isto, uma REQ fechada
// no layout que o proprio `req new` grava nao tinha como voltar a Open.
func TestMoveREQ_ReabreNoLayoutCanonico(t *testing.T) {
	p := projetoByAgent(t)

	if err := MoveREQ("cobaia-de-move", "Open"); err != nil {
		t.Fatalf("MoveREQ(…, \"Open\") no layout canonico: %v", err)
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("a REQ deveria continuar no caminho original: %v", err)
	}
	texto := string(raw)
	if !strings.Contains(texto, "status: Open") {
		t.Fatalf("frontmatter nao reaberto:\n%s", texto)
	}
	if !strings.Contains(texto, "Status: Open") {
		t.Fatalf("cabecalho nao reaberto:\n%s", texto)
	}
	if _, err := os.Stat(filepath.Join("docs", "req", "claude", "Open")); !os.IsNotExist(err) {
		t.Fatal("nao deveria ter criado pasta docs/req/claude/Open — nada se move neste layout")
	}
}

// TestMoveREQ_LayoutCanonicoAceitaEstadoDeRoadmap — controle na outra direcao: o
// vocabulario de roadmap continua aceito onde ja era, e tambem sem mover.
func TestMoveREQ_LayoutCanonicoAceitaEstadoDeRoadmap(t *testing.T) {
	p := projetoByAgent(t)

	if err := MoveREQ("cobaia-de-move", "wip"); err != nil {
		t.Fatalf("MoveREQ(…, \"wip\"): %v", err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("a REQ deveria continuar no caminho original: %v", err)
	}
	if !strings.Contains(string(raw), "status: wip") {
		t.Fatalf("frontmatter nao atualizado:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join("docs", "req", "claude", "wip")); !os.IsNotExist(err) {
		t.Fatal("nao deveria ter criado pasta docs/req/claude/wip")
	}
}

// TestMoveREQ_PastaDeEstadoAindaExigeVocabulario — o controle que impede a correcao de
// virar "aceita qualquer coisa": no layout LEGADO com pasta de estado o destino e um
// diretorio, e um valor arbitrario criaria docs/req/claude/status-invalido-xyz/.
// Espelha TestMoveREQ_RejectsInvalidStateInByAgentLayout, que continua valendo.
func TestMoveREQ_PastaDeEstadoAindaExigeVocabulario(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_namespacing: by_agent\nagents:\n- claude\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("docs", "req", "claude", "wip"), 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join("docs", "req", "claude", "wip", "REQ-2026-09-23-cobaia-legado.md")
	if err := os.WriteFile(p, []byte("---\nstatus: wip\ndate: 2026-09-23\n---\n\n# REQ: Legado\n\n> Date: 2026-09-23 | Status: wip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := MoveREQ("cobaia-legado", "Open")
	if err == nil {
		t.Fatal("no layout com pasta de estado, 'Open' deveria continuar recusado")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("erro deveria mencionar 'invalid state', obteve: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join("docs", "req", "claude", "Open")); !os.IsNotExist(statErr) {
		t.Fatal("nao deveria ter criado pasta docs/req/claude/Open")
	}
}
