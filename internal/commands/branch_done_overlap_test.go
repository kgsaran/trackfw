package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// Cenários da issue #494: `branch new` aceitava um roadmap de done/ alcançado apenas por
// sobreposição de palavras, criando a branch sem nada em wip/.
//
// Cada teste declara, na primeira linha, qual conclusão da medição ele afirma — a Regra Dura de
// Reconciliação do projeto exige a frase, não só que o teste passe.

// escreveAcervo cria wip/ e done/ num diretório temporário e devolve as duas listas de diretórios.
func escreveAcervo(t *testing.T, wip, done []string) (wipDirs, doneDirs []string) {
	t.Helper()
	raiz := t.TempDir()
	wipDir := filepath.Join(raiz, "wip")
	doneDir := filepath.Join(raiz, "done")
	for _, d := range []string{wipDir, doneDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	for _, par := range []struct {
		dir   string
		nomes []string
	}{{wipDir, wip}, {doneDir, done}} {
		for _, n := range par.nomes {
			if err := os.WriteFile(filepath.Join(par.dir, n), []byte("# roadmap\n"), 0o644); err != nil {
				t.Fatalf("write %s: %v", n, err)
			}
		}
	}
	return []string{wipDir}, []string{doneDir}
}

func depsComAcervo(t *testing.T, wip, done []string, criadas *[]string, saida *bytes.Buffer) branchNewDeps {
	t.Helper()
	wipDirs, doneDirs := escreveAcervo(t, wip, done)
	return branchNewDeps{
		loadConfig:        func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:    func(config.ProjectConfig) []string { return wipDirs },
		resolveDoneDirs:   func(config.ProjectConfig) []string { return doneDirs },
		matchSlug:         validator.BranchSlugMatchesRoadmap,
		matchSlugDetailed: validator.MatchRoadmapsForBranchSlugDetailed,
		execGitCheckout: func(b string) error {
			*criadas = append(*criadas, b)
			return nil
		},
		out: saida,
	}
}

// Afirma a conclusão central da medição: o caso reportado na #494 — um slug novo que divide duas
// palavras do vocabulário com roadmaps CONCLUÍDOS de outro assunto — criava a branch, e passa a
// reprovar. Medido no acervo real: `barrier-executa-cada-linha-do-bloco-de-gates` casava com dois
// roadmaps do trust check do barrier, nenhum deles o seu assunto.
func TestBranchNew_SoConcluidoPorSobreposicao_Bloqueia(t *testing.T) {
	var criadas []string
	var saida bytes.Buffer
	deps := depsComAcervo(t, nil, []string{
		"ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel.md",
		"ROADMAP-2026-09-10-barrier-executa-gate-de-roadmap-nao-confiavel-porque-falha-aberto.md",
	}, &criadas, &saida)

	err := runBranchNew("fix/barrier-executa-cada-linha-do-bloco-de-gates", false, false, deps)
	if err == nil {
		t.Fatal("esperava bloqueio: todos os casamentos são de done/ e só por sobreposição")
	}
	if len(criadas) != 0 {
		t.Fatalf("git checkout -b NÃO deveria ter rodado, rodou para %v", criadas)
	}
	got := saida.String()
	for _, esperado := range []string{"only CONCLUDED roadmaps", "--allow-done", "roadmap move"} {
		if !strings.Contains(got, esperado) {
			t.Errorf("mensagem não contém %q:\n%s", esperado, got)
		}
	}
	// A mensagem nomeia os casados: a #494 pede isso explicitamente, porque um slug chegou a casar
	// com nove roadmaps e uma lista sem nomes não permite decidir.
	if !strings.Contains(got, "ROADMAP-2026-08-23-barrier-nao-executa") {
		t.Errorf("mensagem não nomeia o roadmap casado:\n%s", got)
	}
}

// Afirma o braço contrário OBRIGATÓRIO declarado na issue: a correção tardia de uma REQ concluída
// continua criável — com a confirmação explícita. Sem este teste, a correção seria um bloqueio
// cego, que é o remédio excessivo que a issue proíbe.
func TestBranchNew_SoConcluidoPorSobreposicao_AllowDoneCria(t *testing.T) {
	var criadas []string
	var saida bytes.Buffer
	deps := depsComAcervo(t, nil, []string{
		"ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel.md",
	}, &criadas, &saida)

	if err := runBranchNew("fix/barrier-executa-cada-linha-do-bloco-de-gates", false, true, deps); err != nil {
		t.Fatalf("--allow-done deveria criar, deu erro: %v (saída: %s)", err, saida.String())
	}
	if len(criadas) != 1 || criadas[0] != "fix/barrier-executa-cada-linha-do-bloco-de-gates" {
		t.Fatalf("esperava uma branch criada, veio %v", criadas)
	}
}

// Afirma a FALSIFICAÇÃO da direção (a) da issue — exigir contenção para done/. Medido: 142 de 160
// branches gated mescladas do repositório casam por contenção contra done/, e 23 de 23 num fork
// consumidor. Se este caso pedisse confirmação, 88,8% do uso diário passaria a pedir uma flag.
func TestBranchNew_ConcluidoPorContencao_CriaSemFlag(t *testing.T) {
	var criadas []string
	var saida bytes.Buffer
	deps := depsComAcervo(t, nil, []string{
		"ROADMAP-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio.md",
	}, &criadas, &saida)

	// O slug está contido no nome normalizado do roadmap: é o caso ordinário.
	if err := runBranchNew("fix/cerca-nao-terminada-mascara-ate-o-fim-do-arquivo", false, false, deps); err != nil {
		t.Fatalf("contenção contra done/ não deveria pedir flag: %v (saída: %s)", err, saida.String())
	}
	if len(criadas) != 1 {
		t.Fatalf("esperava uma branch criada, veio %v", criadas)
	}
}

// Afirma que a correção não alcança o caso com governança viva: havendo roadmap em wip/, a
// sobreposição continua bastando. É o que o ML-3A da #273 decidiu ao adotar a sobreposição, e esta
// correção não o revoga — ela só nega o que vem SÓ de done/.
func TestBranchNew_SobreposicaoComWip_CriaSemFlag(t *testing.T) {
	var criadas []string
	var saida bytes.Buffer
	deps := depsComAcervo(t,
		[]string{"ROADMAP-2026-10-01-barrier-executa-gate-por-linha-e-o-contrato-nao-diz.md"},
		[]string{"ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel.md"},
		&criadas, &saida)

	if err := runBranchNew("fix/barrier-executa-cada-linha-do-bloco-de-gates", false, false, deps); err != nil {
		t.Fatalf("com roadmap em wip/ a sobreposição deveria bastar: %v (saída: %s)", err, saida.String())
	}
	if len(criadas) != 1 {
		t.Fatalf("esperava uma branch criada, veio %v", criadas)
	}
}

// Afirma que o tipo de housekeeping continua fora do gate — metade do braço contrário da issue já
// estava satisfeita por construção, e esta correção não pode tê-la quebrado: `chore/fecha-req-*`
// nunca consulta done/. Medido com o binário antes da mudança: mesmo slug, chore rc=0, fix rc=1.
func TestBranchNew_ChoreNaoConsultaDone(t *testing.T) {
	var criadas []string
	var saida bytes.Buffer
	deps := depsComAcervo(t, nil, []string{
		"ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel.md",
	}, &criadas, &saida)

	if err := runBranchNew("chore/fecha-req-491-nada-a-ver-com-o-acervo", false, false, deps); err != nil {
		t.Fatalf("chore/ não passa pelo gate: %v (saída: %s)", err, saida.String())
	}
	if len(criadas) != 1 {
		t.Fatalf("esperava uma branch criada, veio %v", criadas)
	}
}

// Afirma que o bloqueio NÃO depende da flag nova para o caso que já reprovava antes: slug sem
// casamento nenhum continua caindo na mensagem antiga, não na nova. Controle de remédio excessivo:
// sem ele, a mensagem nova poderia ter engolido o caminho antigo sem ninguém notar.
func TestBranchNew_SemCasamento_MensagemAntiga(t *testing.T) {
	var criadas []string
	var saida bytes.Buffer
	deps := depsComAcervo(t, nil, []string{
		"ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel.md",
	}, &criadas, &saida)

	err := runBranchNew("fix/xyz-nada-a-ver-qwerty-plumbus", false, false, deps)
	if err == nil {
		t.Fatal("esperava bloqueio por ausência de casamento")
	}
	got := saida.String()
	if strings.Contains(got, "--allow-done") {
		t.Errorf("slug sem casamento não deveria sugerir --allow-done:\n%s", got)
	}
	if !strings.Contains(got, "no matching roadmap") {
		t.Errorf("esperava a mensagem antiga:\n%s", got)
	}
}

// Afirma que a ORDENAÇÃO da mensagem é por força do casamento, não por nome nem por ordem de
// leitura do diretório. A medição encontrou uma branch real casando com NOVE roadmaps concluídos;
// sem ordem, o aviso é uma lista e não uma sugestão.
func TestBranchOnlyConcludedByOverlapMessage_OrdenaPorForca(t *testing.T) {
	msg := validator.BranchOnlyConcludedByOverlapMessage("fix/x", []validator.RoadmapMatch{
		{Name: "ROADMAP-fraco.md", SharedTokens: 2, SlugTokens: 8},
		{Name: "ROADMAP-forte.md", SharedTokens: 7, SlugTokens: 8},
		{Name: "ROADMAP-medio.md", SharedTokens: 4, SlugTokens: 8},
	})
	iF := strings.Index(msg, "ROADMAP-forte.md")
	iM := strings.Index(msg, "ROADMAP-medio.md")
	iW := strings.Index(msg, "ROADMAP-fraco.md")
	if iF < 0 || iM < 0 || iW < 0 {
		t.Fatalf("os três deveriam aparecer:\n%s", msg)
	}
	if !(iF < iM && iM < iW) {
		t.Errorf("esperava forte < medio < fraco, veio %d %d %d:\n%s", iF, iM, iW, msg)
	}
}

// Afirma que a decisão nova NÃO alterou a relação branch↔roadmap: `BranchSlugMatchesRoadmap`, que
// `validate` e `commit` usam, devolve o mesmo veredito de antes para o caso da issue. D3 da
// ADR-2026-09-26 decide uma implementação só da relação; expor o MOTIVO não pode mudar o resultado.
func TestBranchSlugMatchesRoadmap_InalteradoPelaCorrecao(t *testing.T) {
	wipDirs, doneDirs := escreveAcervo(t, nil, []string{
		"ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel.md",
	})
	slug := validator.NormalizeBranchSlug("barrier-executa-cada-linha-do-bloco-de-gates")
	matched, candidates := validator.BranchSlugMatchesRoadmap(slug, wipDirs, doneDirs)
	if !matched {
		t.Error("validate/commit continuam vendo casamento aqui — o gate novo é só do branch new")
	}
	if len(candidates) != 1 {
		t.Errorf("esperava 1 candidato, veio %d", len(candidates))
	}
}
