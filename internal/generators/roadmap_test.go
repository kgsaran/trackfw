package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// testStateDirs retorna os diretórios de estado padrão para uso em testes.
var testStateDirs = []string{
	"docs/roadmaps/backlog",
	"docs/roadmaps/analyzing",
	"docs/roadmaps/wip",
	"docs/roadmaps/blocked",
	"docs/roadmaps/done",
	"docs/roadmaps/abandoned",
}

// chdir muda para dir e restaura ao fim do teste
func chdirRoadmap(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestNewRoadmap_CreatesFile — arquivo criado em docs/roadmaps/backlog/ com conteúdo correto
func TestNewRoadmap_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	if err := NewRoadmap("My Feature"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "My Feature") {
		t.Errorf("arquivo deveria conter 'My Feature', obteve: %q", body)
	}
	if !strings.Contains(body, "REQ:") {
		t.Errorf("arquivo deveria conter 'REQ:', obteve: %q", body)
	}
}

// TestNewRoadmap_StatusLegendAndCanonicalForm — AC11: o template escreve a forma canônica de
// status (⬜ Pendente) e a legenda dos quatro estados, exatamente uma vez; a forma antiga
// "pending" não aparece em lugar nenhum. Falsificação: um regresso que reintroduza "pending" ou
// duplique/remova a legenda deve reprovar este teste.
func TestNewRoadmap_StatusLegendAndCanonicalForm(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	if err := NewRoadmap("Legend Check"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	const legendLine = "⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado"
	if n := strings.Count(body, legendLine); n != 1 {
		t.Errorf("legenda deveria aparecer exatamente 1 vez, obteve %d:\n%s", n, body)
	}
	if !strings.Contains(body, "## Status Legend") {
		t.Errorf("arquivo deveria conter o heading '## Status Legend', obteve:\n%s", body)
	}
	if !strings.Contains(body, "**Status:** ⬜ Pendente") {
		t.Errorf("arquivo deveria conter '**Status:** ⬜ Pendente', obteve:\n%s", body)
	}
	if strings.Contains(body, "**Status:** pending") {
		t.Errorf("forma antiga '**Status:** pending' não deveria aparecer, obteve:\n%s", body)
	}
	if !strings.Contains(body, "**Acceptance criteria:**") {
		t.Errorf("'**Acceptance criteria:**' (forma canônica pelo ADR) deveria continuar presente, obteve:\n%s", body)
	}
	if !strings.Contains(body, "**Gates da wave:**") {
		t.Errorf("'**Gates da wave:**' não deveria ter sido tocado, obteve:\n%s", body)
	}
}

// mkRoadmapDirs cria a estrutura padrão de diretórios de roadmap no diretório corrente.
func mkRoadmapDirs(t *testing.T) {
	t.Helper()
	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}
}

// TestMoveRoadmap_Valid — cria roadmap em backlog, move para wip e verifica frontmatter sincronizado.
func TestMoveRoadmap_Valid(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	if err := NewRoadmap("Move Test"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	if err := MoveRoadmap("move-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	// Deve existir em wip
	wipMatches, err := filepath.Glob("docs/roadmaps/wip/*.md")
	if err != nil {
		t.Fatalf("Glob wip: %v", err)
	}
	if len(wipMatches) != 1 {
		t.Errorf("esperado 1 arquivo em wip, obteve %d: %v", len(wipMatches), wipMatches)
	}

	// Não deve existir mais em backlog
	backlogMatches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(backlogMatches) != 0 {
		t.Errorf("esperado 0 arquivos em backlog após move, obteve %d: %v", len(backlogMatches), backlogMatches)
	}

	// Frontmatter deve ter status: wip (minúsculo, igual ao nome do estado)
	content, err := os.ReadFile(wipMatches[0])
	if err != nil {
		t.Fatalf("ReadFile após move: %v", err)
	}
	if !strings.Contains(string(content), "status: wip") {
		t.Errorf("frontmatter deveria conter 'status: wip', obteve:\n%s", string(content))
	}
	// Cabeçalho também deve ter | Status: wip
	if !strings.Contains(string(content), "| Status: wip") {
		t.Errorf("cabeçalho deveria conter '| Status: wip', obteve:\n%s", string(content))
	}
}

// TestMoveRoadmap_FrontmatterSync_ValidateAfterMove — prova P4: nenhum warning folder_status após move.
// Controle positivo garante que o validador está de fato inspecionando os arquivos.
func TestMoveRoadmap_FrontmatterSync_ValidateAfterMove(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	// Criar e mover um roadmap real: backlog → wip → done
	if err := NewRoadmap("Validate Test"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}
	if err := MoveRoadmap("validate-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap wip: %v", err)
	}
	if err := MoveRoadmap("validate-test", "done"); err != nil {
		t.Fatalf("MoveRoadmap done: %v", err)
	}

	// Controle positivo: escrever manualmente um arquivo em wip com status: backlog → DEVE gerar warning
	controlContent := "---\nstatus: backlog\ndate: 2026-01-01\n---\n# Roadmap: Control\n\n> Created: 2026-01-01 | Status: backlog\n"
	controlPath := "docs/roadmaps/wip/ROADMAP-control.md"
	if err := os.WriteFile(controlPath, []byte(controlContent), 0644); err != nil {
		t.Fatalf("WriteFile controle: %v", err)
	}

	_, warnings, err := validator.ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered(): %v", err)
	}

	// O arquivo movido NÃO deve gerar warning de folder_status
	for _, w := range warnings {
		if strings.Contains(w, "folder_status") && strings.Contains(w, "validate-test") {
			t.Errorf("roadmap movido gerou warning folder_status inesperado: %s", w)
		}
	}

	// O controle positivo DEVE gerar warning de folder_status
	hasControlWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "ROADMAP-control.md") && strings.Contains(w, "folder") {
			hasControlWarning = true
			break
		}
	}
	if !hasControlWarning {
		t.Errorf("controle positivo não gerou warning folder_status — o validador pode não estar inspecionando os arquivos; warnings: %v", warnings)
	}
}

// TestMoveRoadmap_BodyStatusIntact — status: no corpo e | Status: em bloco de código NÃO são tocados.
// Reprova a implementação Python original (re.sub não escopado).
func TestMoveRoadmap_BodyStatusIntact(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	// Roadmap cujo corpo contém 'status: backlog' (em tabela) e '| Status: backlog' (em seção)
	bodyWithStatusLines := "---\nstatus: backlog\ndate: 2026-01-01\n---\n" +
		"# Roadmap: Body Status Test\n\n" +
		"> Created: 2026-01-01 | Status: backlog\n\n" +
		"## Context\n\n" +
		"A tabela abaixo documenta os estados:\n\n" +
		"| Estado | status: backlog |\n" +
		"|--------|----------------|\n" +
		"| Inicial | backlog |\n\n" +
		"Código de exemplo com header:\n\n" +
		"```\n" +
		"> Created: 2026-01-01 | Status: backlog\n" +
		"```\n"

	roadmapPath := "docs/roadmaps/backlog/ROADMAP-body-status-test.md"
	if err := os.WriteFile(roadmapPath, []byte(bodyWithStatusLines), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := MoveRoadmap("body-status-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap(): %v", err)
	}

	wipMatches, _ := filepath.Glob("docs/roadmaps/wip/*.md")
	if len(wipMatches) != 1 {
		t.Fatalf("esperado 1 arquivo em wip, obteve %d", len(wipMatches))
	}
	content, err := os.ReadFile(wipMatches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	// Frontmatter deve ter status: wip
	if !strings.Contains(body, "status: wip") {
		t.Errorf("frontmatter deveria conter 'status: wip'")
	}
	// Cabeçalho deve ter | Status: wip
	if !strings.Contains(body, "| Status: wip") {
		t.Errorf("cabeçalho deveria conter '| Status: wip'")
	}
	// A linha do corpo '| Estado | status: backlog |' NÃO deve ter sido tocada
	if !strings.Contains(body, "| Estado | status: backlog |") {
		t.Errorf("linha do corpo 'status: backlog' foi modificada incorretamente; corpo:\n%s", body)
	}
	// O '| Status: backlog' dentro do bloco de código (após ## ) NÃO deve ter sido tocado
	if !strings.Contains(body, "```\n> Created: 2026-01-01 | Status: backlog\n```") {
		t.Errorf("'| Status: backlog' no bloco de código foi modificado incorretamente; corpo:\n%s", body)
	}
}

// TestMoveRoadmap_NoFrontmatter — arquivo sem frontmatter é movido sem corrupção.
func TestMoveRoadmap_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	// Arquivo sem frontmatter reconhecível
	plainContent := "# Roadmap sem frontmatter\n\nConteúdo simples sem bloco ---.\n"
	roadmapPath := "docs/roadmaps/backlog/ROADMAP-no-frontmatter.md"
	if err := os.WriteFile(roadmapPath, []byte(plainContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := MoveRoadmap("no-frontmatter", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	wipMatches, _ := filepath.Glob("docs/roadmaps/wip/*.md")
	if len(wipMatches) != 1 {
		t.Fatalf("esperado 1 arquivo em wip, obteve %d", len(wipMatches))
	}
	content, err := os.ReadFile(wipMatches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Conteúdo deve ser idêntico ao original (sem chave inventada, sem corrupção)
	if string(content) != plainContent {
		t.Errorf("conteúdo do arquivo sem frontmatter foi alterado;\noriginal: %q\nobteve: %q", plainContent, string(content))
	}
}

func assertMoveRoadmapAnalyzingContract(t *testing.T, byAgent bool) error {
	t.Helper()
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if byAgent {
		yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n"
		if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
			return err
		}
		if err := os.MkdirAll("docs/roadmaps/zeus/backlog", 0755); err != nil {
			return err
		}
		content := "---\nstatus: backlog\ndate: 2026-07-27\nreq: \"docs/req/REQ-demo.md\"\nsquad: \"\"\n---\n\n# Roadmap: Analyze By Agent\n\n> Created: 2026-07-27 | Status: backlog\n"
		if err := os.WriteFile("docs/roadmaps/zeus/backlog/ROADMAP-analyze-by-agent.md", []byte(content), 0644); err != nil {
			return err
		}
		if err := MoveRoadmap("analyze-by-agent", "analyzing"); err != nil {
			return err
		}
		dst := "docs/roadmaps/zeus/analyzing/ROADMAP-analyze-by-agent.md"
		raw, err := os.ReadFile(dst)
		if err != nil {
			return err
		}
		body := string(raw)
		for _, want := range []string{"status: analyzing", "| Status: analyzing"} {
			if !strings.Contains(body, want) {
				return &testExpectationError{message: "roadmap by_agent não sincronizou " + want}
			}
		}
		log, err := os.ReadFile("docs/roadmaps/.trackfw-log")
		if err != nil {
			return err
		}
		if !strings.Contains(string(log), "zeus/ROADMAP-analyze-by-agent.md") || !strings.Contains(string(log), "backlog → analyzing") {
			return &testExpectationError{message: "log by_agent não registrou backlog → analyzing preservando agente"}
		}
		found, err := findRoadmap("analyze-by-agent")
		if err != nil {
			return err
		}
		// findRoadmap devolve separador nativo (o valor alimenta os.Rename/os.ReadDir, uso de
		// filesystem, não artefato autorado — ver ADR-2026-09-04); dst é um literal POSIX de
		// teste. Normalizar found para comparar, não dst: normalizar dst apagaria a asserção.
		if filepath.ToSlash(found) != dst {
			return &testExpectationError{message: "findRoadmap by_agent não encontrou o arquivo em analyzing"}
		}
		if err := ShowRoadmap("analyze-by-agent"); err != nil {
			return err
		}
		if err := ListRoadmaps(); err != nil {
			return err
		}
		return nil
	}

	for _, d := range []string{
		"docs/roadmaps/backlog",
		"docs/roadmaps/analyzing",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	content := "---\nstatus: backlog\ndate: 2026-07-27\nreq: \"docs/req/REQ-demo.md\"\nsquad: \"\"\n---\n\n# Roadmap: Analyze Flat\n\n> Created: 2026-07-27 | Status: backlog\n"
	if err := os.WriteFile("docs/roadmaps/backlog/ROADMAP-analyze-flat.md", []byte(content), 0644); err != nil {
		return err
	}
	if err := MoveRoadmap("analyze-flat", "analyzing"); err != nil {
		return err
	}
	raw, err := os.ReadFile("docs/roadmaps/analyzing/ROADMAP-analyze-flat.md")
	if err != nil {
		return err
	}
	body := string(raw)
	for _, want := range []string{"status: analyzing", "| Status: analyzing"} {
		if !strings.Contains(body, want) {
			return &testExpectationError{message: "roadmap flat não sincronizou " + want}
		}
	}
	log, err := os.ReadFile("docs/roadmaps/.trackfw-log")
	if err != nil {
		return err
	}
	if !strings.Contains(string(log), "ROADMAP-analyze-flat.md") || !strings.Contains(string(log), "backlog → analyzing") {
		return &testExpectationError{message: "log flat não registrou backlog → analyzing"}
	}
	found, err := findRoadmap("analyze-flat")
	if err != nil {
		return err
	}
	// Mesmo raciocínio do branch by_agent acima: found é caminho nativo (uso de filesystem),
	// o literal é POSIX; normaliza-se found, não o literal.
	if filepath.ToSlash(found) != "docs/roadmaps/analyzing/ROADMAP-analyze-flat.md" {
		return &testExpectationError{message: "findRoadmap flat não encontrou o arquivo em analyzing"}
	}
	if err := ShowRoadmap("analyze-flat"); err != nil {
		return err
	}
	if err := ListRoadmaps(); err != nil {
		return err
	}
	return nil
}

func TestMoveRoadmap_AnalyzingFlat(t *testing.T) {
	if err := assertMoveRoadmapAnalyzingContract(t, false); err != nil {
		t.Fatalf("contrato analyzing flat falhou: %v", err)
	}
}

func TestMoveRoadmap_AnalyzingByAgent(t *testing.T) {
	if err := assertMoveRoadmapAnalyzingContract(t, true); err != nil {
		t.Fatalf("contrato analyzing by_agent falhou: %v", err)
	}
}

// TestMoveRoadmap_InvalidState — estado inválido → erro descritivo
func TestMoveRoadmap_InvalidState(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := MoveRoadmap("qualquer-coisa", "inexistente")
	if err == nil {
		t.Fatal("esperado erro para estado inválido, obteve nil")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Errorf("erro deveria mencionar 'invalid state', obteve: %v", err)
	}
}

// TestMoveRoadmap_NotFound — roadmap inexistente → erro descritivo
func TestMoveRoadmap_NotFound(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	// Criar todos os diretórios válidos (vazios)
	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	err := MoveRoadmap("nao-existe", "wip")
	if err == nil {
		t.Fatal("esperado erro para roadmap não encontrado, obteve nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("erro deveria mencionar 'not found', obteve: %v", err)
	}
}

// TestNewRoadmapFromContent_CreatesFile — verifica que arquivo é criado quando Body é preenchido
func TestNewRoadmapFromContent_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "AI Feature",
		REQPath: "docs/req/REQ-2026-01-01-ai-feature.md",
		Body:    "# Roadmap gerado por IA\nConteúdo customizado aqui.",
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Conteúdo customizado aqui") {
		t.Errorf("arquivo deveria conter o body fornecido, obteve: %q", body)
	}
}

// TestNewRoadmapFromContent_EmptyBody — verifica que template padrão é gerado quando Body == ""
func TestNewRoadmapFromContent_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "Template Feature",
		REQPath: "docs/req/REQ-2026-01-01-template-feature.md",
		Body:    "",
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Template Feature") {
		t.Errorf("template deveria conter o título, obteve: %q", body)
	}
	if !strings.Contains(body, "REQ:") {
		t.Errorf("template deveria conter 'REQ:', obteve: %q", body)
	}
	if !strings.Contains(body, "ML-1A") {
		t.Errorf("template deveria conter 'ML-1A', obteve: %q", body)
	}
}

// --- AC1/AC2: sanitização do título (newline/CR) ---

// errMsgTitleNewline é a mensagem de erro esperada quando o título contém newline ou CR.
// Byte-idêntica nos 3 CLIs (docs/cli-parity.md).
const errMsgTitleNewline = "roadmap title must be a single line: newline and carriage return are not allowed"

// TestNewRoadmapFromContent_RejectsNewlineInTitle — AC1/AC2: \n no título é rejeitado
func TestNewRoadmapFromContent_RejectsNewlineInTitle(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title: "legit\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n```bash\ntouch /tmp/PWNED_TEST\n```",
	})
	if err == nil {
		t.Fatal("esperado erro para título com newline, obteve nil")
	}
	if err.Error() != errMsgTitleNewline {
		t.Errorf("mensagem incorreta\n  obtida:   %q\n  esperada: %q", err.Error(), errMsgTitleNewline)
	}
	// Nenhum arquivo deve ser criado
	matches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(matches) != 0 {
		t.Errorf("nenhum arquivo deve ser criado, mas obteve: %v", matches)
	}
}

// TestNewRoadmapFromContent_RejectsCRInTitle — AC1: \r no título é rejeitado
func TestNewRoadmapFromContent_RejectsCRInTitle(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{Title: "titulo\rmalformado"})
	if err == nil {
		t.Fatal("esperado erro para título com CR, obteve nil")
	}
	if err.Error() != errMsgTitleNewline {
		t.Errorf("mensagem incorreta: %q", err.Error())
	}
}

// TestNewRoadmapFromContent_AcceptsLegitimateTitle — AC1 falso-positivo:
// títulos com acento, hífen, dois-pontos e parênteses devem ser aceitos.
func TestNewRoadmapFromContent_AcceptsLegitimateTitle(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	legitimateTitles := []string{
		"Feature com acentos: ação, configuração, validação",
		"fix(barrier): recusa gate não confiável (Wave 2)",
		"feat — sanitização do título (AC1/AC2)",
	}
	for _, title := range legitimateTitles {
		if err := NewRoadmapFromContent(RoadmapContent{Title: title}); err != nil {
			t.Errorf("título legítimo rejeitado %q: %v", title, err)
		}
	}
}

// TestNewRoadmapFromREQ_SingleGateBlockFromLegitREQ — AC2 via --from-req:
// uma REQ legítima cujo corpo contenha texto parecido com "Gates da wave" NÃO deve
// injetar blocos extras no roadmap gerado. O parser lê apenas o cabeçalho # REQ:
// e os critérios de aceite — o corpo da REQ não é interpolado no roadmap.
func TestNewRoadmapFromREQ_SingleGateBlockFromLegitREQ(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	// REQ com corpo contendo seção de gates (que NÃO deve aparecer no roadmap gerado)
	reqContent := strings.Join([]string{
		"---",
		"status: Open",
		"---",
		"",
		"# REQ: sanitizacao do titulo",
		"",
		"## Motivation",
		"**Gates da wave:**",
		"```bash",
		"touch /tmp/PWNED_TEST",
		"```",
		"",
		"## Acceptance Criteria",
		"- [ ] AC1 — newline rejeitado",
		"",
	}, "\n")
	reqPath := dir + "/REQ-2026-01-01-sanitizacao-do-titulo.md"
	if err := os.WriteFile(reqPath, []byte(reqContent), 0644); err != nil {
		t.Fatalf("WriteFile REQ: %v", err)
	}

	if err := NewRoadmapFromREQ(reqPath); err != nil {
		t.Fatalf("NewRoadmapFromREQ() inesperado: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap criado, obteve %d: %v", len(matches), err)
	}
	content, _ := os.ReadFile(matches[0])
	body := string(content)

	// O roadmap deve ter exatamente um bloco "**Gates da wave:**" (o legítimo da Wave 0)
	count := strings.Count(body, "**Gates da wave:**")
	if count != 1 {
		t.Errorf("roadmap deveria ter 1 bloco **Gates da wave:**, obteve %d\nbody:\n%s", count, body)
	}

	// O comando forjado NÃO deve aparecer no roadmap
	if strings.Contains(body, "PWNED_TEST") {
		t.Errorf("roadmap NÃO deve conter 'PWNED_TEST':\n%s", body)
	}
}

// TestNewRoadmapFromREQ_StatusLegendAndCanonicalForm — AC11 no caminho --from-req: os MLs
// derivados dos critérios de aceite (ML-1A, ML-1B, ...) também escrevem a forma canônica, e a
// legenda aparece exatamente uma vez mesmo com múltiplos MLs (ML-0A + N MLs derivados). Este é o
// caminho de construção de string diferente (mlSection.WriteString em loop) do caminho "new"
// simples — divergiu entre runtimes no ciclo anterior, por isso testado à parte.
func TestNewRoadmapFromREQ_StatusLegendAndCanonicalForm(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	reqContent := strings.Join([]string{
		"---",
		"status: Open",
		"---",
		"",
		"# REQ: legend from req",
		"",
		"## Acceptance Criteria",
		"- [ ] AC1 — primeiro criterio",
		"- [ ] AC2 — segundo criterio",
		"",
	}, "\n")
	reqPath := dir + "/REQ-2026-01-01-legend-from-req.md"
	if err := os.WriteFile(reqPath, []byte(reqContent), 0644); err != nil {
		t.Fatalf("WriteFile REQ: %v", err)
	}

	if err := NewRoadmapFromREQ(reqPath); err != nil {
		t.Fatalf("NewRoadmapFromREQ() inesperado: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap criado, obteve %d: %v", len(matches), err)
	}
	content, _ := os.ReadFile(matches[0])
	body := string(content)

	const legendLine = "⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado"
	if n := strings.Count(body, legendLine); n != 1 {
		t.Errorf("legenda deveria aparecer exatamente 1 vez mesmo com 2 MLs derivados, obteve %d:\n%s", n, body)
	}
	// ML-0A (Wave 0) + ML-1A + ML-1B derivados dos 2 critérios = 3 ocorrências da forma canônica.
	if n := strings.Count(body, "**Status:** ⬜ Pendente"); n != 3 {
		t.Errorf("esperava 3 ocorrências de '**Status:** ⬜ Pendente' (ML-0A, ML-1A, ML-1B), obteve %d:\n%s", n, body)
	}
	if strings.Contains(body, "**Status:** pending") {
		t.Errorf("forma antiga '**Status:** pending' não deveria aparecer, obteve:\n%s", body)
	}
}

// TestNewRoadmapFromREQ_AcceptsCRLFLineEndings — falso-positivo: REQ com CRLF deve funcionar.
// O scanner de Go strips trailing \r de cada linha, então o título extraído não tem \r.
func TestNewRoadmapFromREQ_AcceptsCRLFLineEndings(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	// REQ com CRLF (apenas fim de linha, não CR embutido no título)
	reqContent := "---\r\nstatus: Open\r\n---\r\n\r\n# REQ: CRLF Title\r\n"
	reqPath := dir + "/REQ-2026-01-01-crlf.md"
	if err := os.WriteFile(reqPath, []byte(reqContent), 0644); err != nil {
		t.Fatalf("WriteFile REQ CRLF: %v", err)
	}

	err := NewRoadmapFromREQ(reqPath)
	if err != nil {
		t.Errorf("REQ com CRLF rejeitada inesperadamente: %v", err)
	}
}

// TestListRoadmaps_GroupedByState — verifica agrupamento correto por estado
func TestListRoadmaps_GroupedByState(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}

	// Criar um arquivo em backlog e um em done
	if err := os.WriteFile("docs/roadmaps/backlog/ROADMAP-2026-01-01-feature-a.md", []byte("# A"), 0644); err != nil {
		t.Fatalf("WriteFile backlog: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/done/ROADMAP-2026-01-01-feature-b.md", []byte("# B"), 0644); err != nil {
		t.Fatalf("WriteFile done: %v", err)
	}

	// ListRoadmaps não deve retornar erro
	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro: %v", err)
	}
}

// TestListRoadmaps_Empty — nenhum roadmap → mensagem amigável, sem erro
func TestListRoadmaps_Empty(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro esperando nil: %v", err)
	}
}

// TestListRoadmaps_ByAgent — modo by_agent lista roadmaps agrupados por agente/estado
func TestListRoadmaps_ByAgent(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Criar trackfw.yaml com by_agent + agentes zeus e apolo
	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n- apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	// Criar estrutura de diretórios e arquivos
	if err := os.MkdirAll("docs/roadmaps/zeus/wip", 0755); err != nil {
		t.Fatalf("mkdir zeus/wip: %v", err)
	}
	if err := os.MkdirAll("docs/roadmaps/apolo/backlog", 0755); err != nil {
		t.Fatalf("mkdir apolo/backlog: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/zeus/wip/ROADMAP-2026-01-01-zeus-test.md", []byte("# Zeus"), 0644); err != nil {
		t.Fatalf("escrever arquivo zeus: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/apolo/backlog/ROADMAP-2026-01-01-apolo-test.md", []byte("# Apolo"), 0644); err != nil {
		t.Fatalf("escrever arquivo apolo: %v", err)
	}

	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro: %v", err)
	}
}

// TestMoveRoadmap_ByAgent — move roadmap dentro do namespace do agente em modo by_agent
func TestMoveRoadmap_ByAgent(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Criar trackfw.yaml com by_agent
	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	// Criar roadmap em zeus/backlog
	if err := os.MkdirAll("docs/roadmaps/zeus/backlog", 0755); err != nil {
		t.Fatalf("mkdir zeus/backlog: %v", err)
	}
	const roadmapFile = "docs/roadmaps/zeus/backlog/ROADMAP-test.md"
	if err := os.WriteFile(roadmapFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("escrever arquivo: %v", err)
	}

	if err := MoveRoadmap("ROADMAP-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	// Deve existir em zeus/wip
	if _, err := os.Stat("docs/roadmaps/zeus/wip/ROADMAP-test.md"); err != nil {
		t.Errorf("arquivo não encontrado em zeus/wip: %v", err)
	}

	// Não deve existir mais em zeus/backlog
	if _, err := os.Stat(roadmapFile); err == nil {
		t.Error("arquivo ainda existe em zeus/backlog após move")
	}
}

// TestContainsIgnoreCase — função privada testada diretamente via white-box
func TestContainsIgnoreCase(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"ROADMAP-My-Feature.md", "my-feature", true},
		{"roadmap-my-feature.md", "MY-FEATURE", true},
		{"ROADMAP-Other.md", "my-feature", false},
		{"", "sub", false},
		{"something", "", true}, // strings.Contains("something", "") == true
	}

	for _, c := range cases {
		got := containsIgnoreCase(c.s, c.sub)
		if got != c.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, quer %v", c.s, c.sub, got, c.want)
		}
	}
}

// ─── helpers para testes de sincronização de REQ ──────────────────────────────

// captureStdout executa fn e retorna tudo que foi escrito em os.Stdout durante a execução.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("captureStdout: os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("captureStdout: io.Copy: %v", err)
	}
	return buf.String()
}

// mkReqDir cria docs/req/ no diretório corrente e retorna o caminho.
func mkReqDir(t *testing.T) string {
	t.Helper()
	d := "docs/req"
	if err := os.MkdirAll(d, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", d, err)
	}
	return d
}

// writeREQ escreve um arquivo REQ com frontmatter roadmap: apontando para roadmapPath.
// Usa backtick no corpo, espelhando o template real gerado por trackfw req new.
func writeREQ(t *testing.T, reqPath, roadmapPath string) {
	t.Helper()
	content := "---\nstatus: Open\ndate: 2026-07-30\nauthor: \"\"\nadr: \"\"\nroadmap: \"" + roadmapPath + "\"\n---\n\n# REQ: Sync Test\n\n> Date: 2026-07-30 | Status: Open\n\n## Linked Roadmap\nRoadmap: `" + roadmapPath + "`\n"
	if err := os.WriteFile(reqPath, []byte(content), 0644); err != nil {
		t.Fatalf("writeREQ %s: %v", reqPath, err)
	}
}

// mkMinimalRoadmap escreve um roadmap mínimo válido em roadmapPath.
func mkMinimalRoadmap(t *testing.T, roadmapPath, state string) {
	t.Helper()
	body := "---\nstatus: " + state + "\ndate: 2026-07-30\nreq: \"\"\nsquad: \"\"\n---\n\n# Roadmap: Sync Test\n\n> Created: 2026-07-30 | Status: " + state + "\n"
	if err := os.WriteFile(roadmapPath, []byte(body), 0644); err != nil {
		t.Fatalf("mkMinimalRoadmap %s: %v", roadmapPath, err)
	}
}

// ─── testes de cardinalidade — contrato de docs/cli-parity.md ─────────────────

// TestSyncREQ_ZeroREQs: zero REQs apontando → no-op, sem output, exit 0.
func TestSyncREQ_ZeroREQs(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkReqDir(t)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-nonexistent.md", "docs/roadmaps/wip/ROADMAP-nonexistent.md"); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	if out != "" {
		t.Errorf("cardinalidade zero deve produzir output vazio; obteve: %q", out)
	}
}

// TestSyncREQ_OneREQ: uma REQ apontando → reescrita, linha de output correta.
func TestSyncREQ_OneREQ(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-one.md")
	oldPath := "docs/roadmaps/backlog/ROADMAP-one.md"
	newPath := "docs/roadmaps/wip/ROADMAP-one.md"
	writeREQ(t, reqPath, oldPath)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-one.md", newPath); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	wantLine := "✓ synced REQ-one.md → " + newPath + "\n"
	if out != wantLine {
		t.Errorf("output incorreto\nqueria:  %q\nobteve: %q", wantLine, out)
	}

	content, _ := os.ReadFile(reqPath)
	s := string(content)
	if !strings.Contains(s, "roadmap: \""+newPath+"\"") {
		t.Errorf("frontmatter não atualizado;\nconteúdo:\n%s", s)
	}
	if !strings.Contains(s, "Roadmap: `"+newPath+"`") {
		t.Errorf("corpo não atualizado com backticks;\nconteúdo:\n%s", s)
	}
}

// TestSyncREQ_SeveralREQs: várias REQs apontando → todas reescritas, uma linha cada.
func TestSyncREQ_SeveralREQs(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	oldPath := "docs/roadmaps/backlog/ROADMAP-multi.md"
	newPath := "docs/roadmaps/wip/ROADMAP-multi.md"

	names := []string{"REQ-A.md", "REQ-B.md", "REQ-C.md"}
	for _, n := range names {
		writeREQ(t, filepath.Join(reqDir, n), oldPath)
	}

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-multi.md", newPath); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	for _, n := range names {
		wantLine := "✓ synced " + n + " → " + newPath
		if !strings.Contains(out, wantLine) {
			t.Errorf("linha %q ausente no output:\n%s", wantLine, out)
		}
		content, _ := os.ReadFile(filepath.Join(reqDir, n))
		if !strings.Contains(string(content), "roadmap: \""+newPath+"\"") {
			t.Errorf("frontmatter de %s não atualizado", n)
		}
	}
}

// TestSyncREQ_PointsAtOtherRoadmap: REQ apontando para outro roadmap → não tocada.
func TestSyncREQ_PointsAtOtherRoadmap(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-other.md")
	otherPath := "docs/roadmaps/wip/ROADMAP-other.md"
	writeREQ(t, reqPath, otherPath)

	origContent, _ := os.ReadFile(reqPath)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-moved.md", "docs/roadmaps/done/ROADMAP-moved.md"); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	if out != "" {
		t.Errorf("REQ apontando para outro roadmap não deve gerar output; obteve: %q", out)
	}

	newContent, _ := os.ReadFile(reqPath)
	if !bytes.Equal(origContent, newContent) {
		t.Errorf("REQ apontando para outro roadmap foi modificada (bytes divergem)")
	}
}

// TestSyncREQ_AlreadyCorrect: referência já correta → nenhuma escrita (byte-level idempotente).
func TestSyncREQ_AlreadyCorrect(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-correct.md")
	newPath := "docs/roadmaps/wip/ROADMAP-correct.md"
	writeREQ(t, reqPath, newPath) // já aponta para o estado correto

	origContent, _ := os.ReadFile(reqPath)
	origMtime, _ := os.Stat(reqPath)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-correct.md", newPath); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	if out != "" {
		t.Errorf("referência já correta não deve gerar output; obteve: %q", out)
	}

	newContent, _ := os.ReadFile(reqPath)
	if !bytes.Equal(origContent, newContent) {
		t.Errorf("bytes da REQ alterados mesmo com referência já correta")
	}

	newInfo, _ := os.Stat(reqPath)
	if newInfo.ModTime() != origMtime.ModTime() {
		t.Errorf("mtime da REQ alterado mesmo sem escrita (arquivo foi regravado)")
	}
}

// TestSyncREQ_Idempotency_ByteLevel: mover duas vezes não altera bytes da REQ.
// Prova: primeira chamada atualiza; segunda é no-op byte-a-byte.
func TestSyncREQ_Idempotency_ByteLevel(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-idempotent.md")
	oldPath := "docs/roadmaps/backlog/ROADMAP-idempotent.md"
	newPath := "docs/roadmaps/wip/ROADMAP-idempotent.md"
	writeREQ(t, reqPath, oldPath)
	mkMinimalRoadmap(t, oldPath, "backlog")

	// Primeira sincronização: atualiza backlog → wip
	if err := syncREQReferences("ROADMAP-idempotent.md", newPath); err != nil {
		t.Fatalf("primeira sincronização falhou: %v", err)
	}
	bytesAfterFirst, _ := os.ReadFile(reqPath)

	// Segunda sincronização com o mesmo alvo: não deve alterar bytes
	if err := syncREQReferences("ROADMAP-idempotent.md", newPath); err != nil {
		t.Fatalf("segunda sincronização falhou: %v", err)
	}
	bytesAfterSecond, _ := os.ReadFile(reqPath)

	if !bytes.Equal(bytesAfterFirst, bytesAfterSecond) {
		t.Errorf("idempotência violada: bytes diferem após segunda sincronização\napós 1ª:\n%s\napós 2ª:\n%s",
			bytesAfterFirst, bytesAfterSecond)
	}
}

// TestSyncREQ_ByAgent: REQ em req_dir/<agente>/<estado>/ é encontrada no modo by_agent.
func TestSyncREQ_ByAgent(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Configurar by_agent com um agente explícito
	yaml := "roadmap_namespacing: by_agent\nagents:\n- apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}
	config.Reset() // recarregar config com o yaml recém-escrito
	t.Cleanup(config.Reset)

	// REQ em docs/req/apolo/wip/
	reqDir := "docs/req/apolo/wip"
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", reqDir, err)
	}
	reqPath := filepath.Join(reqDir, "REQ-byagent.md")
	oldPath := "docs/roadmaps/apolo/backlog/ROADMAP-byagent.md"
	newPath := "docs/roadmaps/apolo/wip/ROADMAP-byagent.md"
	writeREQ(t, reqPath, oldPath)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-byagent.md", newPath); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	if !strings.Contains(out, "✓ synced REQ-byagent.md → "+newPath) {
		t.Errorf("REQ em by_agent não foi sincronizada; output: %q", out)
	}

	content, _ := os.ReadFile(reqPath)
	if !strings.Contains(string(content), "roadmap: \""+newPath+"\"") {
		t.Errorf("frontmatter da REQ by_agent não atualizado:\n%s", content)
	}
}

// TestSyncREQ_ByAgent_OrderByBasename: fixture discriminante — dois agentes com REQs
// cujos basenames são invertidos em relação à ordem de caminho completo.
//
// Fixture:
//   docs/req/apolo/done/REQ-zzz.md   → aponta para o roadmap
//   docs/req/zeus/backlog/REQ-aaa.md  → aponta para o roadmap
//   agents: [zeus, apolo]             → ordem de varredura natural: zeus/…aaa, apolo/…zzz
//
// Por caminho completo: apolo/…zzz < zeus/…aaa → zzz, aaa (errado).
// Por basename:          aaa < zzz                → aaa, zzz (correto — contrato pinado).
//
// O teste asserta a SEQUÊNCIA exata das linhas de output, não apenas o conjunto.
func TestSyncREQ_ByAgent_OrderByBasename(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// by_agent com dois agentes — ordem intencional invertida (zeus antes de apolo)
	// para que a varredura natural produza a ordem errada se não houver sort.
	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n- apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}
	config.Reset()
	t.Cleanup(config.Reset)

	oldPath := "docs/roadmaps/zeus/backlog/ROADMAP-order.md"
	newPath := "docs/roadmaps/zeus/wip/ROADMAP-order.md"

	// REQ-zzz.md em apolo/done/
	reqDirApolo := filepath.Join("docs", "req", "apolo", "done")
	if err := os.MkdirAll(reqDirApolo, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", reqDirApolo, err)
	}
	reqZzz := filepath.Join(reqDirApolo, "REQ-zzz.md")
	writeREQ(t, reqZzz, oldPath)

	// REQ-aaa.md em zeus/backlog/
	reqDirZeus := filepath.Join("docs", "req", "zeus", "backlog")
	if err := os.MkdirAll(reqDirZeus, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", reqDirZeus, err)
	}
	reqAaa := filepath.Join(reqDirZeus, "REQ-aaa.md")
	writeREQ(t, reqAaa, oldPath)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-order.md", newPath); err != nil {
			t.Errorf("syncREQReferences: %v", err)
		}
	})

	// Asserta a SEQUÊNCIA: aaa deve aparecer ANTES de zzz
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("esperado 2 linhas de output; obteve %d:\n%s", len(lines), out)
	}
	wantFirst := "✓ synced REQ-aaa.md → " + newPath
	wantSecond := "✓ synced REQ-zzz.md → " + newPath
	if lines[0] != wantFirst {
		t.Errorf("linha 0: esperado %q, obteve %q\noutput completo:\n%s", wantFirst, lines[0], out)
	}
	if lines[1] != wantSecond {
		t.Errorf("linha 1: esperado %q, obteve %q\noutput completo:\n%s", wantSecond, lines[1], out)
	}

	// Ambas as REQs devem ter sido atualizadas
	for _, p := range []string{reqAaa, reqZzz} {
		content, _ := os.ReadFile(p)
		if !strings.Contains(string(content), "roadmap: \""+newPath+"\"") {
			t.Errorf("frontmatter de %s não atualizado:\n%s", filepath.Base(p), content)
		}
	}
}

// TestSyncREQ_BackticksPreservedInBody: backticks no corpo da REQ são preservados após reescrita.
func TestSyncREQ_BackticksPreservedInBody(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-backtick.md")
	oldPath := "docs/roadmaps/backlog/ROADMAP-backtick.md"
	newPath := "docs/roadmaps/wip/ROADMAP-backtick.md"

	// Conteúdo com backtick explícito no corpo
	content := "---\nstatus: Open\ndate: 2026-07-30\nroadmap: \"" + oldPath + "\"\n---\n\n# REQ: Backtick Test\n\n## Linked Roadmap\nRoadmap: `" + oldPath + "`\n"
	if err := os.WriteFile(reqPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := syncREQReferences("ROADMAP-backtick.md", newPath); err != nil {
		t.Fatalf("syncREQReferences: %v", err)
	}

	result, _ := os.ReadFile(reqPath)
	s := string(result)

	if !strings.Contains(s, "Roadmap: `"+newPath+"`") {
		t.Errorf("backticks não foram preservados no corpo;\nconteúdo:\n%s", s)
	}
	if strings.Contains(s, oldPath) {
		t.Errorf("caminho antigo ainda presente no conteúdo:\n%s", s)
	}
}

// TestSyncREQ_ErrorContinues: falha de escrita em uma REQ não interrompe as demais;
// o erro é reportado em stderr e syncREQReferences retorna erro não-nulo.
func TestSyncREQ_ErrorContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 0444 não bloqueia escrita como root")
	}

	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	oldPath := "docs/roadmaps/backlog/ROADMAP-err.md"
	newPath := "docs/roadmaps/wip/ROADMAP-err.md"

	// REQ não-gravável (unwritable)
	reqUnwritable := filepath.Join(reqDir, "REQ-A-unwritable.md")
	writeREQ(t, reqUnwritable, oldPath)
	if err := os.Chmod(reqUnwritable, 0444); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(reqUnwritable, 0644) })

	// REQ normal — deve ser atualizada mesmo com a outra falhando
	reqOK := filepath.Join(reqDir, "REQ-B-ok.md")
	writeREQ(t, reqOK, oldPath)

	out := captureStdout(t, func() {
		err := syncREQReferences("ROADMAP-err.md", newPath)
		if err == nil {
			t.Errorf("esperado erro não-nulo pela REQ não-gravável")
		}
	})

	// A REQ gravável deve ter sido atualizada
	if !strings.Contains(out, "✓ synced REQ-B-ok.md") {
		t.Errorf("REQ gravável não foi sincronizada; output: %q", out)
	}
	okContent, _ := os.ReadFile(reqOK)
	if !strings.Contains(string(okContent), newPath) {
		t.Errorf("REQ gravável não foi atualizada após falha da outra")
	}
}

// TestMoveRoadmap_SyncsREQFrontmatter: integração completa — move via MoveRoadmap e verifica
// que a REQ pareada é atualizada (frontmatter + corpo com backticks).
func TestMoveRoadmap_SyncsREQFrontmatter(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	reqDir := mkReqDir(t)
	roadmapBase := "ROADMAP-integration.md"
	backlogPath := "docs/roadmaps/backlog/" + roadmapBase
	wipPath := "docs/roadmaps/wip/" + roadmapBase

	mkMinimalRoadmap(t, backlogPath, "backlog")

	reqPath := filepath.Join(reqDir, "REQ-integration.md")
	writeREQ(t, reqPath, backlogPath)

	out := captureStdout(t, func() {
		if err := MoveRoadmap("integration", "wip"); err != nil {
			t.Fatalf("MoveRoadmap: %v", err)
		}
	})

	// Output deve ter ✓ moved seguido de ✓ synced
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("esperado pelo menos 2 linhas de output; obteve: %q", out)
	}
	if !strings.HasPrefix(lines[0], "✓ moved") {
		t.Errorf("primeira linha deve ser '✓ moved', obteve: %q", lines[0])
	}
	if !strings.Contains(lines[1], "✓ synced REQ-integration.md → "+wipPath) {
		t.Errorf("segunda linha deve ser '✓ synced'; obteve: %q", lines[1])
	}

	// Roadmap deve estar em wip
	if _, err := os.Stat(wipPath); err != nil {
		t.Errorf("roadmap não encontrado em wip: %v", err)
	}

	// REQ deve apontar para wip
	content, _ := os.ReadFile(reqPath)
	s := string(content)
	if !strings.Contains(s, "roadmap: \""+wipPath+"\"") {
		t.Errorf("frontmatter da REQ não atualizado;\nconteúdo:\n%s", s)
	}
	if !strings.Contains(s, "Roadmap: `"+wipPath+"`") {
		t.Errorf("corpo da REQ não atualizado (backticks);\nconteúdo:\n%s", s)
	}
}

// ─── ML-1A/ML-1B — separador portável (item 10 do issue #216) ─────────────────

// TestNormalizeRefSeparator_ConvertsNativeToPortable e TestNormalizeRefSeparator_ControlLeavesCleanValueUnchanged
// cobrem a função isoladamente: converte "\" para "/", e não altera um valor já portável.
func TestNormalizeRefSeparator_ConvertsNativeToPortable(t *testing.T) {
	dirty := `docs\roadmaps\wip\ROADMAP-x.md`
	want := "docs/roadmaps/wip/ROADMAP-x.md"
	if got := normalizeRefSeparator(dirty); got != want {
		t.Errorf("queria %q, obteve %q", want, got)
	}
}

func TestNormalizeRefSeparator_ControlLeavesCleanValueUnchanged(t *testing.T) {
	clean := "docs/roadmaps/wip/ROADMAP-x.md"
	if got := normalizeRefSeparator(clean); got != clean {
		t.Errorf("valor já portável não deveria mudar; queria %q, obteve %q", clean, got)
	}
}

// TestSyncREQ_HealsDirtyBackslashReference — ML-1B: uma REQ cujo frontmatter roadmap: já foi
// gravado com separador nativo ("\", simulando um commit feito no Windows antes do fix de
// escrita) é curada por um roadmap move subsequente: o predicado de descoberta normaliza o
// valor antes de comparar o basename, então a REQ suja É encontrada e reescrita com "/".
// Sem essa normalização, filepath.Base(fmVal) em Linux/macOS devolve o valor sujo inteiro (não
// há "/" para separar), nunca bate com roadmapBasename, e a REQ nunca é curada.
func TestSyncREQ_HealsDirtyBackslashReference(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-dirty.md")
	newPath := "docs/roadmaps/wip/ROADMAP-dirty.md"
	dirtyRef := strings.ReplaceAll(newPath, "/", `\`)
	writeREQ(t, reqPath, dirtyRef)

	out := captureStdout(t, func() {
		if err := syncREQReferences("ROADMAP-dirty.md", newPath); err != nil {
			t.Errorf("esperado nil, obteve: %v", err)
		}
	})

	wantLine := "✓ synced REQ-dirty.md → " + newPath + "\n"
	if out != wantLine {
		t.Errorf("output incorreto (referência suja deveria ser curada)\nqueria:  %q\nobteve: %q", wantLine, out)
	}

	content, _ := os.ReadFile(reqPath)
	s := string(content)
	if !strings.Contains(s, "roadmap: \""+newPath+"\"") {
		t.Errorf("frontmatter não curado para separador portável;\nconteúdo:\n%s", s)
	}
	if strings.Contains(s, dirtyRef) {
		t.Errorf("valor sujo com \"\\\\\" ainda presente após a cura;\nconteúdo:\n%s", s)
	}
}

// TestSyncREQ_ControlDoesNotTouchUnrelatedBackslashInBody — limite duro do parecer de ameaça:
// prosa/código no corpo da REQ com "\" legítimo, cujo basename NÃO coincide com o roadmap
// movido, não é tocado pela sincronização. A linha citada precisa começar com "Roadmap:" —
// esse é o único formato que rewriteREQRoadmapRef examina no corpo; uma linha sem esse prefixo
// (ex.: comentário Python com "#") nunca chega ao comparador de basename, normalizado ou não, e
// por isso não seria um controle real do limite duro (ver advisor review: a primeira versão
// desta função usava uma linha sem ":", que já era ignorada por strings.Cut antes mesmo desta
// mudança — não provava nada). Este teste usa uma linha "Roadmap:" de verdade, com um basename
// DIFERENTE do roadmap movido, dentro de uma cerca de código.
func TestSyncREQ_ControlDoesNotTouchUnrelatedBackslashInBody(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-with-prose.md")
	oldPath := "docs/roadmaps/backlog/ROADMAP-real.md"
	newPath := "docs/roadmaps/wip/ROADMAP-real.md"

	// Basename diferente do roadmap movido (ROADMAP-outro.md != ROADMAP-real.md) — mesmo após
	// normalizar o separador, o basename não bate, então esta linha não deveria ser tocada.
	unrelatedFencedLine := "Roadmap: `docs\\roadmaps\\wip\\ROADMAP-outro.md`"
	unrelatedProse := "```\n" + unrelatedFencedLine + "\n```\n"
	content := "---\nstatus: Open\ndate: 2026-07-30\nroadmap: \"" + oldPath + "\"\n---\n\n" +
		"# REQ: With prose\n\n## Exemplo (não tocar)\n" + unrelatedProse +
		"\n## Linked Roadmap\nRoadmap: `" + oldPath + "`\n"
	if err := os.WriteFile(reqPath, []byte(content), 0644); err != nil {
		t.Fatalf("write REQ: %v", err)
	}

	if err := syncREQReferences("ROADMAP-real.md", newPath); err != nil {
		t.Fatalf("syncREQReferences: %v", err)
	}

	updated, _ := os.ReadFile(reqPath)
	s := string(updated)
	if !strings.Contains(s, unrelatedFencedLine) {
		t.Errorf("linha \"Roadmap:\" de basename diferente foi alterada; conteúdo:\n%s", s)
	}
	if !strings.Contains(s, "roadmap: \""+newPath+"\"") {
		t.Errorf("frontmatter real não foi atualizado; conteúdo:\n%s", s)
	}
}

// TestSyncREQ_KnownWideningRewritesFencedExampleWithMatchingBasename documenta um efeito
// colateral aceito, não um limite duro: rewriteREQRoadmapRef só examina a PRIMEIRA linha
// "Roadmap:" do corpo (sem distinguir se está dentro de uma cerca de código), e a normalização
// do ML-1B amplia o conjunto de linhas que ela reconhece como "aponta para o roadmap movido" —
// antes, uma linha "Roadmap: `docs\roadmaps\wip\ROADMAP-x.md`" suja nunca batia por basename
// (filepath.Base do valor sujo inteiro != roadmapBasename) e por isso nunca era tocada, mesmo
// citando de propósito o roadmap certo; depois desta mudança, ela é reconhecida e reescrita —
// mesmo se essa linha estiver dentro de um bloco de exemplo/prosa. Verificado no repositório
// real (grep por 'Roadmap:.*\\' em docs/req/) que não há ocorrência hoje — o risco é latente,
// não um incidente ativo. Reportado ao arquiteto; não é o limite duro #2 do parecer de ameaça
// (esse cobre "\" em prosa cujo basename NÃO coincide, que é o teste acima).
func TestSyncREQ_KnownWideningRewritesFencedExampleWithMatchingBasename(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	reqDir := mkReqDir(t)
	reqPath := filepath.Join(reqDir, "REQ-with-matching-fence.md")
	oldPath := "docs/roadmaps/backlog/ROADMAP-real.md"
	newPath := "docs/roadmaps/wip/ROADMAP-real.md"

	// Mesmo basename do roadmap movido (ROADMAP-real.md), citado suja dentro de uma cerca —
	// é a única linha "Roadmap:" do corpo, então é ela quem é reescrita (rewriteREQRoadmapRef
	// só processa a primeira ocorrência).
	fencedExampleLine := "Roadmap: `docs\\roadmaps\\wip\\ROADMAP-real.md`"
	content := "---\nstatus: Open\ndate: 2026-07-30\nroadmap: \"" + oldPath + "\"\n---\n\n" +
		"# REQ: With matching fence\n\n## Exemplo\n```\n" + fencedExampleLine + "\n```\n"
	if err := os.WriteFile(reqPath, []byte(content), 0644); err != nil {
		t.Fatalf("write REQ: %v", err)
	}

	if err := syncREQReferences("ROADMAP-real.md", newPath); err != nil {
		t.Fatalf("syncREQReferences: %v", err)
	}

	updated, _ := os.ReadFile(reqPath)
	s := string(updated)
	if strings.Contains(s, fencedExampleLine) {
		t.Errorf("comportamento documentado mudou: exemplo com basename coincidente deixou de ser reescrito; conteúdo:\n%s", s)
	}
}

// TestMoveRoadmap_SyncsREQFrontmatterWithPortableSeparator — controle de regressão: o valor
// escrito no frontmatter da REQ pareada nunca contém "\", mesmo indiretamente (nesta máquina
// filepath.Join já produz "/", então isto documenta o comportamento esperado sem depender de
// rodar em Windows — a prova cross-plataforma real é o gate de CI da Wave 2).
func TestMoveRoadmap_SyncsREQFrontmatterWithPortableSeparator(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	reqDir := mkReqDir(t)
	roadmapBase := "ROADMAP-portable.md"
	backlogPath := "docs/roadmaps/backlog/" + roadmapBase

	mkMinimalRoadmap(t, backlogPath, "backlog")

	reqPath := filepath.Join(reqDir, "REQ-portable.md")
	writeREQ(t, reqPath, backlogPath)

	if err := MoveRoadmap("portable", "wip"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	content, _ := os.ReadFile(reqPath)
	s := string(content)
	if strings.Contains(s, `\`) {
		t.Errorf("frontmatter da REQ contém separador nativo \"\\\\\"; conteúdo:\n%s", s)
	}
}
