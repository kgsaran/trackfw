package generators

import (
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// #451 — o mesmo `trackfw validate` rodando em triplicata por push.
//
// A decomposição medida num PR real (branch do próprio repositório):
//
//	governance-go-install      2 execuções   ← trackfw-validate.yml, push + pull_request
//	governance-install-script  1 execução    ← trackfw-gate.yml, pull_request
//	                           3 no total
//
// A coexistência dos DOIS arquivos é decidida e está escrita (ADR-2026-08-28, citada
// em scaffold_doctor.go); não se mexe nela aqui. O que este teste protege é a outra
// metade, que mora dentro de um arquivo só: o gatilho em lista faz o MESMO commit
// disparar dois eventos.
//
// Reconciliação (Regra Dura): a frase de cada teste está no seu comentário.
// ────────────────────────────────────────────────────────────────────────────

// AFIRMA: as DUAS variantes do template (produtor e consumidor) deixaram de usar o
// gatilho em lista, que é o que fazia o mesmo SHA ser validado duas vezes. Testar uma
// só deixaria metade do defeito viva com aparência de corrigida.
func TestDiscoverWorkflow_DoesNotDoubleRunOnTheSameCommit(t *testing.T) {
	for _, tc := range []struct {
		nome       string
		isProducer bool
	}{
		{"produtor", true},
		{"consumidor", false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			got := BuildDiscoverGitHubActionsWorkflowContent(tc.isProducer)

			if strings.Contains(got, "on: [push, pull_request]") {
				t.Fatal("o gatilho em lista voltou: push e pull_request disparam o mesmo commit duas vezes")
			}

			// O push precisa continuar existindo, restrito à main — senão a main deixa
			// de ser validada, e a correção do desperdício teria aberto um buraco.
			if !strings.Contains(got, "  push:\n    branches: [main]\n") {
				t.Fatalf("o push restrito à main sumiu do template; got:\n%s", got)
			}

			// E o pull_request precisa continuar — é ele que guarda o PR.
			if !strings.Contains(got, "  pull_request:\n") {
				t.Fatalf("o gatilho de pull_request sumiu do template; got:\n%s", got)
			}
		})
	}
}

// AFIRMA: o id do job continua o mesmo nas duas variantes. Ele é contrato de
// required_status_checks — mudá-lo sem querer faria o check exigido desaparecer da
// branch protection e o portão abriria em silêncio, que é pior que o desperdício que
// este PR corrige.
func TestDiscoverWorkflow_JobIDIsUnchanged(t *testing.T) {
	for _, isProducer := range []bool{true, false} {
		got := BuildDiscoverGitHubActionsWorkflowContent(isProducer)
		if !strings.Contains(got, "  governance-go-install:\n") {
			t.Fatalf("job id governance-go-install ausente (isProducer=%v):\n%s", isProducer, got)
		}
	}
}
