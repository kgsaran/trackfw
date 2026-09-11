package generators

// agent_write_test.go — testes do contrato de resolução de agente em req new e roadmap new (ML-1A).
//
// Cada teste declara qual conclusão do ML ele afirma (Regra Dura de Reconciliação, CLAUDE.md).
//
// Cenários obrigatórios do handoff ML-1A (Wave 1, contrato comum):
//   T1 — agents:[alpha,beta] + --agent beta → artefato em beta/, frontmatter beta
//   T2 — agents:[alpha,beta] sem flag → erro, mensagem nomeia alpha E beta
//   T3 — agents:[alpha] sem flag → cria em alpha/, sem erro (contra-braço)
//   T4 — flat → comportamento inalterado
//   T5 — roadmap new --req <REQ em beta/> → roadmap em beta/ (herança AC11)
//
// Cenários adicionais (advisor / AC5b / AC14):
//   T6 — --agent gamma (fora de agents:) → cria gamma/, sem erro (AC5b funciona)
//   T7 — agents:["",zeus] sem flag → cria em zeus/, sem erro (filtragem de vazio)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupByAgent prepara um diretório temporário com trackfw.yaml em modo by_agent.
// agentsYAML é a linha de agentes, ex: "- alpha\n- beta\n".
func setupByAgent(t *testing.T, agentsYAML string) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})
	config.Reset()

	yaml := "roadmap_namespacing: by_agent\nagents:\n" + agentsYAML
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}
	return dir
}

// setupFlat prepara um diretório temporário sem trackfw.yaml (modo flat por omissão).
func setupFlat(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})
	config.Reset()
	return dir
}

// ── T1 ──────────────────────────────────────────────────────────────────────
// Afirma: com vários agentes e --agent beta, req new e roadmap new escrevem em
// beta/ e registram beta no frontmatter; a flag sobreescreve o default silencioso.

func TestReqNew_MultiAgent_WithFlag_WritesToNamespace(t *testing.T) {
	setupByAgent(t, "- alpha\n- beta\n")

	if err := NewREQ(REQContent{Title: "Teste Flag", Agent: "beta"}); err != nil {
		t.Fatalf("NewREQ com --agent beta: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/beta/REQ-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 REQ em docs/req/beta/, obteve %d: %v", len(matches), matches)
	}
	// REQ não tem campo squad — verificar apenas o caminho.
	if !strings.Contains(matches[0], "beta") {
		t.Errorf("caminho não contém 'beta': %q", matches[0])
	}
}

func TestRoadmapNew_MultiAgent_WithFlag_WritesToNamespaceFrontmatter(t *testing.T) {
	dir := setupByAgent(t, "- alpha\n- beta\n")
	_ = dir

	if err := NewRoadmapFromContent(RoadmapContent{Title: "Feature Beta", Agent: "beta"}); err != nil {
		t.Fatalf("NewRoadmapFromContent com Agent=beta: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/beta/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em beta/backlog/, obteve %d", len(matches))
	}
	body, _ := os.ReadFile(matches[0])
	// Afirma: squad: "beta" no frontmatter (efeito 2 de AC4 — caminho e frontmatter do mesmo valor).
	if !strings.Contains(string(body), `squad: "beta"`) {
		t.Errorf("frontmatter não contém squad: \"beta\":\n%s", body)
	}
}

// ── T2 ──────────────────────────────────────────────────────────────────────
// Afirma: com vários agentes e sem flag, req new e roadmap new falham e a mensagem
// de erro nomeia TODOS os namespaces — o silêncio de Agents[0] não existe mais.

func TestReqNew_MultiAgent_NoFlag_ReturnsErrorNamingAllAgents(t *testing.T) {
	setupByAgent(t, "- alpha\n- beta\n")

	err := NewREQ(REQContent{Title: "Ambíguo"})
	if err == nil {
		t.Fatal("esperava erro de ambiguidade, obteve nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "alpha") || !strings.Contains(msg, "beta") {
		t.Errorf("mensagem de erro deve nomear 'alpha' e 'beta', obteve: %q", msg)
	}
}

func TestRoadmapNew_MultiAgent_NoFlag_ReturnsErrorNamingAllAgents(t *testing.T) {
	setupByAgent(t, "- alpha\n- beta\n")

	err := NewRoadmapFromContent(RoadmapContent{Title: "Ambíguo"})
	if err == nil {
		t.Fatal("esperava erro de ambiguidade, obteve nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "alpha") || !strings.Contains(msg, "beta") {
		t.Errorf("mensagem de erro deve nomear 'alpha' e 'beta', obteve: %q", msg)
	}
}

// ── T3 ──────────────────────────────────────────────────────────────────────
// Afirma (contra-braço obrigatório): com UM único agente e sem flag, req new e
// roadmap new criam em alpha/ SEM erro — a guarda só dispara em ambiguidade real.

func TestReqNew_SingleAgent_NoFlag_SucceedsInNamespace(t *testing.T) {
	setupByAgent(t, "- alpha\n")

	if err := NewREQ(REQContent{Title: "Single Agent"}); err != nil {
		t.Fatalf("NewREQ com um agente e sem flag deveria ter sucesso: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/alpha/REQ-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 REQ em docs/req/alpha/, obteve %d: %v", len(matches), matches)
	}
}

func TestRoadmapNew_SingleAgent_NoFlag_SucceedsInNamespace(t *testing.T) {
	setupByAgent(t, "- alpha\n")

	if err := NewRoadmapFromContent(RoadmapContent{Title: "Single Agent Roadmap"}); err != nil {
		t.Fatalf("NewRoadmapFromContent com um agente e sem flag deveria ter sucesso: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/alpha/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em alpha/backlog/, obteve %d", len(matches))
	}
}

// ── T4 ──────────────────────────────────────────────────────────────────────
// Afirma: em modo flat (sem trackfw.yaml), req new e roadmap new continuam
// gravando em docs/req/ e docs/roadmaps/backlog/ — nenhuma regressão em flat.

func TestReqNew_FlatMode_UnchangedBehavior(t *testing.T) {
	setupFlat(t)

	if err := NewREQ(REQContent{Title: "Flat REQ"}); err != nil {
		t.Fatalf("NewREQ em flat deveria ter sucesso: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/REQ-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 REQ em docs/req/ (flat), obteve %d: %v", len(matches), matches)
	}
}

func TestRoadmapNew_FlatMode_UnchangedBehavior(t *testing.T) {
	setupFlat(t)

	if err := NewRoadmapFromContent(RoadmapContent{Title: "Flat Roadmap"}); err != nil {
		t.Fatalf("NewRoadmapFromContent em flat deveria ter sucesso: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog/ (flat), obteve %d", len(matches))
	}
}

// ── T5 ──────────────────────────────────────────────────────────────────────
// Afirma: herança de agente por caminho da REQ funciona nos DOIS caminhos de geração:
//   - NewRoadmapFromREQ (--from-req): já funcionava antes deste fix.
//   - NewRoadmapFromContent com REQPath (--req): corrigido pelo AC11-fix (ML-1A-fix).
// REQ flat (diretamente em req_dir/) não herda — cai na guarda de ambiguidade.
// --agent explícito vence a herança em ambos os caminhos.

// TestRoadmapFromREQ_InheritsAgentFromREQPath testa a herança via NewRoadmapFromREQ (--from-req).
// Afirma: --from-req com REQ em beta/ sem --agent cria roadmap em beta/backlog/ (AC11).
func TestRoadmapFromREQ_InheritsAgentFromREQPath(t *testing.T) {
	dir := setupByAgent(t, "- alpha\n- beta\n")

	// Criar REQ no namespace beta
	reqDir := filepath.Join(dir, "docs", "req", "beta")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reqPath := filepath.Join(reqDir, "REQ-2026-01-01-heranca-via-from-req.md")
	reqContent := "---\nstatus: Open\ndate: 2026-01-01\n---\n# REQ: Heranca via from-req\n\n## Acceptance Criteria\n- [ ] AC1\n"
	if err := os.WriteFile(reqPath, []byte(reqContent), 0644); err != nil {
		t.Fatalf("escrever REQ: %v", err)
	}

	// NewRoadmapFromREQ sem agente explícito deve herdar 'beta' do caminho.
	if err := NewRoadmapFromREQ(reqPath, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ deveria herdar beta do caminho da REQ: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/beta/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado roadmap em beta/backlog/, obteve %d arquivos (%v)", len(matches), matches)
	}
	body, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(body), `squad: "beta"`) {
		t.Errorf("frontmatter não contém squad: \"beta\":\n%s", body)
	}
}

// ── T5b ─────────────────────────────────────────────────────────────────────
// Afirma: NewRoadmapFromContent com REQPath preenchido e Agent="" herda o agente
// do namespace da REQ (AC11) — o caminho --req agora tem o mesmo comportamento
// que --from-req. Este é o caso que reprovou o ML anterior.

// TestRoadmapFromContent_InheritsAgentFromREQPath testa a herança via NewRoadmapFromContent
// (caminho --req). Afirma: REQPath em beta/ sem Agent explícito → roadmap em beta/backlog/.
func TestRoadmapFromContent_InheritsAgentFromREQPath(t *testing.T) {
	dir := setupByAgent(t, "- alpha\n- beta\n")

	reqDir := filepath.Join(dir, "docs", "req", "beta")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reqPath := filepath.Join(reqDir, "REQ-2026-01-01-heranca-via-req.md")
	if err := os.WriteFile(reqPath, []byte("---\nstatus: Open\ndate: 2026-01-01\n---\n# REQ: Heranca via req\n"), 0644); err != nil {
		t.Fatalf("escrever REQ: %v", err)
	}

	// NewRoadmapFromContent sem Agent explícito deve herdar 'beta' do REQPath.
	if err := NewRoadmapFromContent(RoadmapContent{Title: "rm", REQPath: reqPath}); err != nil {
		t.Fatalf("NewRoadmapFromContent deveria herdar beta do REQPath: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/beta/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado roadmap em beta/backlog/, obteve %d (%v)", len(matches), matches)
	}
	body, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(body), `squad: "beta"`) {
		t.Errorf("frontmatter não contém squad: \"beta\":\n%s", body)
	}
}

// ── T5c ─────────────────────────────────────────────────────────────────────
// Contra-braço: REQ flat (diretamente em req_dir/) não herda namespace —
// cai na guarda de ambiguidade e retorna erro listando os agentes disponíveis.
// Afirma: REQ flat com múltiplos agentes → erro de ambiguidade (comportamento correto).

func TestRoadmapFromContent_FlatREQDoesNotInheritAgent(t *testing.T) {
	dir := setupByAgent(t, "- alpha\n- beta\n")

	// REQ flat: diretamente em req_dir/, sem subpasta de agente.
	reqDir := filepath.Join(dir, "docs", "req")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("mkdir req_dir: %v", err)
	}
	reqPath := filepath.Join(reqDir, "REQ-2026-01-01-flat.md")
	if err := os.WriteFile(reqPath, []byte("---\nstatus: Open\n---\n# REQ: flat\n"), 0644); err != nil {
		t.Fatalf("escrever REQ: %v", err)
	}

	// Deve falhar com erro de ambiguidade — não deve herdar nenhum namespace de agente.
	err := NewRoadmapFromContent(RoadmapContent{Title: "rm-flat", REQPath: reqPath})
	if err == nil {
		t.Fatal("esperado erro de ambiguidade para REQ flat com múltiplos agentes, mas obteve nil")
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("erro de ambiguidade deveria nomear alpha e beta, obteve: %v", err)
	}
}

// ── T5d ─────────────────────────────────────────────────────────────────────
// Contra-braço: --agent explícito vence a herança do REQPath.
// Afirma: NewRoadmapFromContent com REQPath em beta/ mas Agent="alpha" → roadmap em alpha/backlog/.

func TestRoadmapFromContent_ExplicitAgentWinsOverREQPathInheritance(t *testing.T) {
	dir := setupByAgent(t, "- alpha\n- beta\n")

	reqDir := filepath.Join(dir, "docs", "req", "beta")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reqPath := filepath.Join(reqDir, "REQ-2026-01-01-beta.md")
	if err := os.WriteFile(reqPath, []byte("---\nstatus: Open\n---\n# REQ: beta req\n"), 0644); err != nil {
		t.Fatalf("escrever REQ: %v", err)
	}

	// --agent alpha explícito deve vencer a herança de beta via REQPath.
	if err := NewRoadmapFromContent(RoadmapContent{Title: "rm-explicit", REQPath: reqPath, Agent: "alpha"}); err != nil {
		t.Fatalf("NewRoadmapFromContent com --agent alpha deveria ter sucesso: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/alpha/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado roadmap em alpha/backlog/ (--agent vence herança), obteve %d (%v)", len(matches), matches)
	}
	// Garante que NÃO foi criado em beta/.
	betaMatches, _ := filepath.Glob("docs/roadmaps/beta/backlog/ROADMAP-*.md")
	if len(betaMatches) != 0 {
		t.Errorf("roadmap não deveria ter sido criado em beta/, obteve %d", len(betaMatches))
	}
}

// ── T6 ──────────────────────────────────────────────────────────────────────
// Afirma: --agent gamma com agents:[alpha,beta] cria namespace gamma/ sem erro (AC5b).
// A violação agent_namespace_undeclared vem do validate, não do new.

func TestReqNew_AgentOutsideAgentsList_CreatesNamespace(t *testing.T) {
	setupByAgent(t, "- alpha\n- beta\n")

	if err := NewREQ(REQContent{Title: "Gamma REQ", Agent: "gamma"}); err != nil {
		t.Fatalf("--agent gamma fora de agents: deveria ter sucesso (AC5b): %v", err)
	}

	matches, _ := filepath.Glob("docs/req/gamma/REQ-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 REQ em docs/req/gamma/, obteve %d", len(matches))
	}
}

func TestRoadmapNew_AgentOutsideAgentsList_CreatesNamespace(t *testing.T) {
	setupByAgent(t, "- alpha\n- beta\n")

	if err := NewRoadmapFromContent(RoadmapContent{Title: "Gamma Roadmap", Agent: "gamma"}); err != nil {
		t.Fatalf("--agent gamma fora de agents: deveria ter sucesso (AC5b): %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/gamma/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em gamma/backlog/, obteve %d", len(matches))
	}
}

// ── T7 ──────────────────────────────────────────────────────────────────────
// Afirma: agents:["",zeus] sem flag filtra o nome vazio e cria em zeus/ sem erro.
// Prova que agentStateDir e ResolveWriteAgent têm a mesma noção de "vazio não é nome".

func TestReqNew_EmptyAgentFiltered_UsesFirstNonEmpty(t *testing.T) {
	setupByAgent(t, "- \"\"\n- zeus\n")

	if err := NewREQ(REQContent{Title: "Zeus Filtered"}); err != nil {
		t.Fatalf("com agents:['',zeus] sem flag deveria usar zeus: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/zeus/REQ-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 REQ em docs/req/zeus/, obteve %d: %v", len(matches), matches)
	}
}

func TestRoadmapNew_EmptyAgentFiltered_UsesFirstNonEmpty(t *testing.T) {
	setupByAgent(t, "- \"\"\n- zeus\n")

	if err := NewRoadmapFromContent(RoadmapContent{Title: "Zeus Filtered Roadmap"}); err != nil {
		t.Fatalf("com agents:['',zeus] sem flag deveria usar zeus: %v", err)
	}

	matches, _ := filepath.Glob("docs/roadmaps/zeus/backlog/ROADMAP-*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em zeus/backlog/, obteve %d", len(matches))
	}
}
