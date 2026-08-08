package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunReqNew_NoTTY_BehaviorUnchanged — sem TTY (ambiente de teste, stdin não é terminal),
// runReqNew deve seguir o caminho generators.NewREQ(content) direto, sem nenhum prompt novo
// (nem o de escopo de ADR introduzido nesta ML) e sem gerar ADR drafts. ROADMAP-2026-08-08 ML-2A.
func TestRunReqNew_NoTTY_BehaviorUnchanged(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := runReqNew(nil, []string{"Estrategia de Autenticacao"}); err != nil {
		t.Fatalf("runReqNew erro inesperado (sem TTY não deveria acionar wizard): %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperava 1 REQ criada, encontrou %d", len(matches))
	}

	// Sem TTY não deve haver geração de ADR drafts (nenhuma probe é processada nesse caminho).
	adrMatches, _ := filepath.Glob(filepath.Join("docs", "adr", "ADR-*.md"))
	if len(adrMatches) != 0 {
		t.Errorf("não esperava ADR drafts sem TTY, encontrou %d", len(adrMatches))
	}
}
