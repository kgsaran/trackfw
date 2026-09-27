package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// Testes do ML-4A (AC1 + AC10 da REQ-2026-09-09): criar REQ e roadmap deixa de exigir dois
// comandos, E criar REQ sem roadmap continua possível quando é deliberado.
//
// Cada teste declara no próprio comentário qual conclusão do ML ele afirma (Regra Dura de
// Reconciliação — CLAUDE.md).
//
// Nota de ambiente: `go test` roda com stdin que NÃO é terminal, logo todo teste aqui exercita
// exatamente o caminho não-interativo (CI/agente) — o mesmo em que um prompt não rodaria.

func chdirML4A(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	config.Reset()
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})
	return dir
}

// reqHasRoadmapNames diz se a regra `req_has_roadmap` do validate REAL acusa a REQ de basename
// reqBase. Filtra por regra E por basename porque "a regra apareceu na saída" é falso verde: o
// validate desta árvore emite avisos de escopo global (~/.trackfw) que nada têm a ver com a
// fixture. Lê violações E avisos porque a SEVERIDADE da regra é escopo do ML-4B, em paralelo —
// este teste afirma o vínculo, não o grau.
func reqHasRoadmapNames(t *testing.T, reqBase string) bool {
	t.Helper()
	violations, warnings, err := validator.ValidateTagged()
	if err != nil {
		t.Fatalf("ValidateTagged: %v", err)
	}
	for _, m := range append(append([]validator.TaggedMsg{}, violations...), warnings...) {
		if m.Rule == "req_has_roadmap" && strings.Contains(m.Msg, reqBase) {
			return true
		}
	}
	return false
}

func onlyREQML4A(t *testing.T) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 1 {
		t.Fatalf("esperava 1 REQ, encontrou %d: %v", len(matches), matches)
	}
	return matches[0]
}

func roadmapsML4A(t *testing.T) []string {
	t.Helper()
	var all []string
	for _, state := range []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"} {
		m, _ := filepath.Glob(filepath.Join("docs", "roadmaps", state, "ROADMAP-*.md"))
		all = append(all, m...)
	}
	return all
}

// Reconciliação: afirma A conclusão central do ML-4A — `req new` sozinho produz uma REQ que o
// validate REAL não acusa de órfã, e `req new --no-roadmap` produz uma que ele acusa. Os dois braços
// vivem no MESMO teste, com a MESMA fixture e o MESMO título, diferindo só no flag: é isso que impede
// o teste de passar com a integração inerte (o modo de falha medido no ML-1B — código que compila,
// roda e não propaga o caminho). Numa integração inerte o braço padrão também acusaria, e o teste
// reprova.
func TestRunReqNew_IntegratedPathIsNotOrphanAndNoRoadmapArmIs(t *testing.T) {
	const title = "Cobranca recorrente por assinatura"

	t.Run("padrao — sem flag, vinculada", func(t *testing.T) {
		chdirML4A(t)
		if err := runReqNew(nil, []string{title}, "", false); err != nil {
			t.Fatalf("runReqNew padrão: %v", err)
		}
		reqPath := onlyREQML4A(t)
		if rms := roadmapsML4A(t); len(rms) != 1 {
			t.Fatalf("caminho integrado deveria criar exatamente 1 roadmap, criou %d: %v", len(rms), rms)
		}
		if reqHasRoadmapNames(t, filepath.Base(reqPath)) {
			data, _ := os.ReadFile(reqPath)
			t.Errorf("validate ainda acusa %s de órfã — caminho integrado não fechou o elo:\n%s",
				filepath.Base(reqPath), data)
		}
	})

	t.Run("deliberado — --no-roadmap, acusada", func(t *testing.T) {
		chdirML4A(t)
		if err := runReqNew(nil, []string{title}, "", true); err != nil {
			t.Fatalf("runReqNew --no-roadmap não deve falhar: %v", err)
		}
		reqPath := onlyREQML4A(t)
		if !reqHasRoadmapNames(t, filepath.Base(reqPath)) {
			t.Errorf("braço de controle vacuoso: o validate NÃO acusa %s mesmo sem roadmap — "+
				"o braço padrão deste teste passaria com a integração desligada",
				filepath.Base(reqPath))
		}
	})
}

// Reconciliação (contra-braço): afirma que o caminho deliberado do ML-4A continua DISPONÍVEL — com
// `--no-roadmap` a REQ é criada, nenhum roadmap nasce, e o comando termina SEM erro. É o teste que
// separa "atrito onde é engano" de "imposição": se `--no-roadmap` virasse erro, a saída previsível
// seria escrever o .md à mão, fora do comando, onde nem o vínculo nem a validação existem.
func TestRunReqNew_NoRoadmapArmStaysAvailableAndIsNotAnError(t *testing.T) {
	chdirML4A(t)
	if err := runReqNew(nil, []string{"Exportar relatorio fiscal"}, "", true); err != nil {
		t.Fatalf("--no-roadmap deveria ser caminho válido, retornou erro: %v", err)
	}
	if _, err := os.Stat(onlyREQML4A(t)); err != nil {
		t.Fatalf("REQ deveria existir no caminho deliberado: %v", err)
	}
	if rms := roadmapsML4A(t); len(rms) != 0 {
		t.Errorf("--no-roadmap não deveria criar roadmap, criou %d: %v", len(rms), rms)
	}
}

// Reconciliação: afirma a idempotência do caminho integrado do ML-4A — rodar duas vezes não cria
// dois roadmaps nem duplica o vínculo na REQ.
func TestRunReqNew_IntegratedPathIsIdempotent(t *testing.T) {
	chdirML4A(t)
	const title = "Fila de notificacoes assincronas"

	if err := runReqNew(nil, []string{title}, "", false); err != nil {
		t.Fatalf("1a rodada: %v", err)
	}
	if err := runReqNew(nil, []string{title}, "", false); err != nil {
		t.Fatalf("2a rodada: %v", err)
	}

	if rms := roadmapsML4A(t); len(rms) != 1 {
		t.Errorf("duas rodadas deveriam deixar 1 roadmap, deixaram %d: %v", len(rms), rms)
	}
	reqPath := onlyREQML4A(t)
	data, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}
	got := string(data)
	if n := strings.Count(got, "roadmap: "); n != 1 {
		t.Errorf("frontmatter deveria ter 1 campo roadmap:, tem %d:\n%s", n, got)
	}
	if n := strings.Count(got, "Roadmap: docs/roadmaps/"); n != 1 {
		t.Errorf("corpo deveria ter 1 marcador Roadmap: preenchido, tem %d:\n%s", n, got)
	}
	// Anti-vacuidade: "não duplicou" é satisfeito trivialmente por "nunca vinculou".
	if reqHasRoadmapNames(t, filepath.Base(reqPath)) {
		t.Errorf("idempotência medida sobre REQ órfã — teste vacuoso:\n%s", got)
	}
}

// Reconciliação: afirma que o guard de idempotência do ML-4A é por VÍNCULO e não por nome de
// arquivo — com o roadmap já promovido para wip/ (onde o nome não colide mais com o que backlog/
// geraria), uma reexecução NÃO cria um segundo roadmap, e o trabalho escrito à mão dentro dele
// sobrevive. Sem este braço, a idempotência do teste anterior seria explicada por sobrescrita
// (mesmo caminho, bytes iguais), que preserva a contagem e destrói o conteúdo.
func TestRunReqNew_RerunAfterPromotionDoesNotDuplicateAndPreservesEdits(t *testing.T) {
	chdirML4A(t)
	const title = "Importador de extrato bancario"

	if err := runReqNew(nil, []string{title}, "", false); err != nil {
		t.Fatalf("1a rodada: %v", err)
	}
	created := roadmapsML4A(t)
	if len(created) != 1 {
		t.Fatalf("esperava 1 roadmap após a 1a rodada, obteve %d: %v", len(created), created)
	}
	if err := generators.MoveRoadmap(filepath.Base(created[0]), "wip"); err != nil {
		t.Fatalf("MoveRoadmap wip: %v", err)
	}
	moved := roadmapsML4A(t)
	if len(moved) != 1 || !strings.Contains(moved[0], string(os.PathSeparator)+"wip"+string(os.PathSeparator)) {
		t.Fatalf("esperava 1 roadmap em wip/, obteve %v", moved)
	}
	const handWritten = "\n### ML-9Z — escrito a mao, precisa sobreviver\n"
	f, err := os.OpenFile(moved[0], os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile roadmap: %v", err)
	}
	if _, err := f.WriteString(handWritten); err != nil {
		t.Fatalf("append: %v", err)
	}
	_ = f.Close()

	if err := runReqNew(nil, []string{title}, "", false); err != nil {
		t.Fatalf("2a rodada: %v", err)
	}

	after := roadmapsML4A(t)
	if len(after) != 1 {
		t.Errorf("reexecução após promoção não deveria criar um segundo roadmap, há %d: %v", len(after), after)
	}
	body, err := os.ReadFile(moved[0])
	if err != nil {
		t.Fatalf("ReadFile roadmap: %v", err)
	}
	if !strings.Contains(string(body), handWritten) {
		t.Errorf("o ML escrito à mão foi destruído pela reexecução:\n%s", body)
	}
	// E a REQ, que `req new` reescreveu do zero na 2a rodada, volta a apontar para o roadmap
	// promovido — o guard não pode trocar "roadmap duplicado" por "REQ órfã".
	reqPath := onlyREQML4A(t)
	if reqHasRoadmapNames(t, filepath.Base(reqPath)) {
		data, _ := os.ReadFile(reqPath)
		t.Errorf("reexecução deixou %s órfã:\n%s", filepath.Base(reqPath), data)
	}
}

// Reconciliação: afirma que o caminho integrado do ML-4A lê a CONFIG do projeto em vez de assumir
// `docs/req`/`docs/roadmaps` — num t.TempDir() sem trackfw.yaml os dois seriam indistinguíveis
// (achado do ML-3D), então a fixture declara diretórios customizados e mede onde os artefatos caem.
func TestRunReqNew_IntegratedPathHonorsCustomDirs(t *testing.T) {
	chdirML4A(t)
	if err := os.WriteFile("trackfw.yaml",
		[]byte("req_dir: docs/requisicoes\nroadmap_dir: docs/planos\n"), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}
	config.Reset()

	if err := runReqNew(nil, []string{"Catalogo de produtos"}, "", false); err != nil {
		t.Fatalf("runReqNew: %v", err)
	}

	reqs, _ := filepath.Glob(filepath.Join("docs", "requisicoes", "REQ-*.md"))
	if len(reqs) != 1 {
		t.Fatalf("esperava 1 REQ em docs/requisicoes, obteve %d", len(reqs))
	}
	rms, _ := filepath.Glob(filepath.Join("docs", "planos", "backlog", "ROADMAP-*.md"))
	if len(rms) != 1 {
		t.Fatalf("esperava 1 roadmap em docs/planos/backlog, obteve %d", len(rms))
	}
	if stray, _ := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "*.md")); len(stray) != 0 {
		t.Errorf("roadmap escrito no diretório default em vez do configurado: %v", stray)
	}
	data, err := os.ReadFile(reqs[0])
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}
	// filepath.ToSlash: o produto escreve separador portável; filepath.Glob devolve nativo no Windows.
	portableRM := filepath.ToSlash(rms[0])
	if !strings.Contains(string(data), `roadmap: "`+portableRM+`"`) {
		t.Errorf("REQ deveria apontar para %q, obteve:\n%s", portableRM, data)
	}
	// ADR-2026-09-04 D1: separador portável "/" nos artefatos autorados.
	// Reconciliação: afirma que o produto grava separador POSIX no frontmatter roadmap: mesmo com
	// diretórios customizados (cfg.RoadmapDir="docs/planos" em trackfw.yaml).
	if i := strings.Index(string(data), `roadmap: "`); i >= 0 {
		line := string(data)[i:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if strings.ContainsRune(line, '\\') {
			t.Errorf("produto gravou separador nativo em artefato versionado — viola ADR-2026-09-04 D1: %s", strings.TrimSpace(line))
		}
	}
}
