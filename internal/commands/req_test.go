package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunReqNew_NoTTY_NoWizardNoADRDrafts — sem TTY (ambiente de teste, stdin não é terminal),
// runReqNew não abre wizard nenhum (nem o prompt de escopo de ADR) e não gera ADR drafts.
// ROADMAP-2026-08-08 ML-2A.
//
// 🔴 Renomeado no ML-4A (REQ-2026-09-09): o nome anterior era "BehaviorUnchanged", e a partir deste
// ML o comportamento MUDOU de propósito — `req new` passa a criar o roadmap no mesmo ato. O que este
// teste sempre mediu, e continua medindo, é a ausência de interação sem TTY; a asserção de roadmap
// vive em req_chain_ml4a_test.go. Este teste passa `--no-roadmap` para isolar o que ele afirma.
func TestRunReqNew_NoTTY_NoWizardNoADRDrafts(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := runReqNew(nil, []string{"Estrategia de Autenticacao"}, "", true); err != nil {
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
