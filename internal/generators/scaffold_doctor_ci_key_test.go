package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// ─── #484: o aviso do D3 é presença-em-disco, não `ci:` ─────────────────────
//
// RECONCILIAÇÃO (Regra Dura — todo teste novo declara o que afirma):
//
//   - TestRunScaffoldDoctor_DuplicacaoSemChaveCI afirma a conclusão de que o aviso do D3
//     estava amarrado a `cfg.CI` e, por isso, não alcançava o projeto que o comentário
//     vizinho do próprio arquivo descreve — "a project can have discover's workflow
//     without cfg.CI ever being set". É o caso do fork consumidor, onde os dois arquivos
//     chegam por merge e não por `init`.
//   - TestRunScaffoldDoctor_SemChaveCI_SoOGateNaoAvisa é o CONTROLE. Sem ele, um aviso que
//     passasse a disparar sempre que o `ci:` estivesse ausente passaria no primeiro teste.
//
// 🔴 O par é o que dá valor: o primeiro afirma que a condição PASSOU a ser o disco; o
// segundo afirma que ela continua sendo o disco, e não "ausência de ci:".

// trackfw.yaml sem NENHUMA chave ci: — o caso medido no fork consumidor.
const yamlSemCI = "project: sonda\n"

func escreveOsDoisWorkflows(t *testing.T, dir string, comValidate bool) {
	t.Helper()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "trackfw-gate.yml"),
		[]byte(buildGitHubActionsWorkflowContent(false)), 0o644); err != nil {
		t.Fatal(err)
	}
	if !comValidate {
		return
	}
	if err := os.WriteFile(filepath.Join(wfDir, "trackfw-validate.yml"),
		[]byte(BuildDiscoverGitHubActionsWorkflowContent(false)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func avisoDeDuplicacao(t *testing.T, dir string) *integrations.DoctorFinding {
	t.Helper()
	config.Reset()
	t.Cleanup(config.Reset)
	findings, err := RunScaffoldDoctor(dir)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor error: %v", err)
	}
	for i := range findings {
		if findings[i].FindingKind == integrations.DoctorScaffoldWorkflowDuplicated {
			return &findings[i]
		}
	}
	return nil
}

func TestRunScaffoldDoctor_DuplicacaoSemChaveCI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yamlSemCI), 0o644); err != nil {
		t.Fatal(err)
	}
	escreveOsDoisWorkflows(t, dir, true)

	aviso := avisoDeDuplicacao(t, dir)
	if aviso == nil {
		t.Fatal("os DOIS workflows estão no disco e o trackfw.yaml não declara ci:; " +
			"o aviso do D3 tem de sair — a presença dos arquivos não depende daquela chave (#484)")
	}
	if aviso.Destination != DiscoverGitHubActionsWorkflowPath {
		t.Errorf("o aviso deve apontar o %s (o redundante), obtive %q",
			DiscoverGitHubActionsWorkflowPath, aviso.Destination)
	}
}

func TestRunScaffoldDoctor_SemChaveCI_SoOGateNaoAvisa(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yamlSemCI), 0o644); err != nil {
		t.Fatal(err)
	}
	escreveOsDoisWorkflows(t, dir, false) // só o gate.yml

	if aviso := avisoDeDuplicacao(t, dir); aviso != nil {
		t.Fatalf("só o gate.yml está no disco — não há duplicação a avisar; obtive %+v", aviso)
	}
}
