package generators

// ML-1C (REQ-2026-09-09) — recusar o nome vazio e o nome ambíguo em `roadmap move`
// e `req move`.
//
// Cada teste deste arquivo afirma UMA conclusão da seção de medição do relatório do
// ML-1C (medida em 2026-09-26 sobre o corpus real: 228 roadmaps em docs/roadmaps/**
// e 231 REQs em docs/req):
//
//	query vazia                  → 228 roadmaps / 231 REQs casados, primeiro-vence
//	query = basename sem ".md"   → 225 únicos, 3 ambíguos (stem é prefixo de irmão maior)
//	query = 20 primeiros chars   → 151 únicos, 77 ambíguos (hoje escolhidos em silêncio)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

const ml1cBody = "# r\n\n## Wave 0 — Threat Model\n"

// writeRoadmapFile escreve um roadmap mínimo em dir, criando dir se preciso.
func writeRoadmapFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(ml1cBody), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", p, err)
	}
	return p
}

// TestMoveRoadmap_EmptyName_RefusesAndMovesNothing
//
// Reconciliação: afirma que a query vazia, que casava os 228 roadmaps do corpus real
// e movia o PRIMEIRO deles (ocorreu em 2026-09-12), agora move ZERO — falsificado
// pelo arquivo no disco, não pelo erro retornado.
func TestMoveRoadmap_EmptyName_RefusesAndMovesNothing(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	names := []string{"ROADMAP-alpha.md", "ROADMAP-beta.md", "ROADMAP-gama.md"}
	for _, n := range names {
		writeRoadmapFile(t, "docs/roadmaps/backlog", n)
	}

	for _, empty := range []string{"", "   "} {
		err := MoveRoadmap(empty, "wip")
		if err == nil {
			t.Fatalf("MoveRoadmap(%q) deveria recusar, retornou nil", empty)
		}
		if !strings.Contains(err.Error(), "empty name") {
			t.Errorf("erro deve nomear o problema (nome vazio); got: %v", err)
		}
	}

	// Falsificação no disco: nada saiu de backlog, nada entrou em wip.
	for _, n := range names {
		if _, statErr := os.Stat(filepath.Join("docs/roadmaps/backlog", n)); statErr != nil {
			t.Errorf("%s deveria continuar em backlog: %v", n, statErr)
		}
	}
	wip, _ := filepath.Glob("docs/roadmaps/wip/*.md")
	if len(wip) != 0 {
		t.Errorf("wip deveria estar vazio, obteve %v", wip)
	}
}

// TestMoveRoadmap_EmptyName_RefusesInByAgentLayout
//
// Reconciliação: afirma que a recusa do nome vazio vale também no layout by_agent —
// a segunda cópia do laço primeiro-vence (antiga linha :791), que o corpus real deste
// projeto (`roadmap_namespacing: flat`) não exercita e por isso poderia ficar com o
// defeito vivo e o `make quality` verde.
func TestMoveRoadmap_EmptyName_RefusesInByAgentLayout(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n- athena\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}
	writeRoadmapFile(t, "docs/roadmaps/zeus/backlog", "ROADMAP-zeus-um.md")
	writeRoadmapFile(t, "docs/roadmaps/athena/backlog", "ROADMAP-athena-um.md")

	if err := MoveRoadmap("", "wip"); err == nil {
		t.Fatal("MoveRoadmap(\"\") em by_agent deveria recusar, retornou nil")
	} else if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("erro deve nomear o nome vazio; got: %v", err)
	}
	for _, p := range []string{
		"docs/roadmaps/zeus/backlog/ROADMAP-zeus-um.md",
		"docs/roadmaps/athena/backlog/ROADMAP-athena-um.md",
	} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Errorf("%s deveria continuar onde estava: %v", p, statErr)
		}
	}
}

// TestMoveRoadmap_ExactAndUniquePartialNames_StillMove
//
// Reconciliação (contra-braço duplo): afirma que os 225 de 228 nomes completos únicos
// e os 151 de 228 fragmentos de 20 caracteres únicos do corpus real continuam movendo —
// com e sem o sufixo ".md". Sem este teste, "nada move mais" satisfaria o braço negativo.
func TestMoveRoadmap_ExactAndUniquePartialNames_StillMove(t *testing.T) {
	cases := []struct {
		label string
		query string
	}{
		{"exato com .md", "ROADMAP-2026-09-09-req-nasce-orfa.md"},
		{"exato sem .md", "ROADMAP-2026-09-09-req-nasce-orfa"},
		{"parcial único", "nasce-orfa"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			dir := t.TempDir()
			chdirRoadmap(t, dir)
			config.Reset()
			t.Cleanup(config.Reset)
			mkRoadmapDirs(t)

			writeRoadmapFile(t, "docs/roadmaps/backlog", "ROADMAP-2026-09-09-req-nasce-orfa.md")
			writeRoadmapFile(t, "docs/roadmaps/backlog", "ROADMAP-2026-08-31-guarda-de-folha.md")

			if err := MoveRoadmap(c.query, "wip"); err != nil {
				t.Fatalf("MoveRoadmap(%q) deveria mover, erro: %v", c.query, err)
			}
			if _, statErr := os.Stat("docs/roadmaps/wip/ROADMAP-2026-09-09-req-nasce-orfa.md"); statErr != nil {
				t.Errorf("arquivo não chegou em wip: %v", statErr)
			}
			if _, statErr := os.Stat("docs/roadmaps/backlog/ROADMAP-2026-08-31-guarda-de-folha.md"); statErr != nil {
				t.Errorf("o roadmap não nomeado foi afetado: %v", statErr)
			}
		})
	}
}

// TestMoveRoadmap_AmbiguousPartial_RefusesNamingCandidates
//
// Reconciliação: afirma que os 77 de 228 fragmentos de 20 caracteres que hoje casam
// mais de um roadmap e escolhem UM em silêncio passam a recusar NOMEANDO os candidatos.
func TestMoveRoadmap_AmbiguousPartial_RefusesNamingCandidates(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	writeRoadmapFile(t, "docs/roadmaps/backlog", "ROADMAP-2026-07-19-global-adrs-um.md")
	writeRoadmapFile(t, "docs/roadmaps/done", "ROADMAP-2026-07-19-global-adrs-dois.md")

	err := MoveRoadmap("global-adrs", "wip")
	if err == nil {
		t.Fatal("nome ambíguo deveria recusar, retornou nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"multiple roadmaps match",
		filepath.Join("docs/roadmaps/backlog", "ROADMAP-2026-07-19-global-adrs-um.md"),
		filepath.Join("docs/roadmaps/done", "ROADMAP-2026-07-19-global-adrs-dois.md"),
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("erro deveria conter %q; got: %v", want, msg)
		}
	}
	if wip, _ := filepath.Glob("docs/roadmaps/wip/*.md"); len(wip) != 0 {
		t.Errorf("nenhum arquivo deveria ter sido movido, obteve %v", wip)
	}
}

// TestMoveRoadmap_ExactNameWinsOverLongerSibling
//
// Reconciliação: afirma o caso dos 3 roadmaps ambíguos medidos no corpus real
// (`...global-adrs-governance` contra `...global-adrs-governance-ML-1B.md`): o nome
// completo deixa de ser escolha arbitrária de ordem de varredura e passa a resolver
// para o arquivo exato — é por isso que a mudança adiciona ZERO recusas novas para
// queries de nome completo.
func TestMoveRoadmap_ExactNameWinsOverLongerSibling(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	// O irmão maior vive em backlog (varrido ANTES de done na ordem canônica), logo
	// sob primeiro-vence ele venceria a query exata do arquivo que está em done.
	writeRoadmapFile(t, "docs/roadmaps/backlog", "ROADMAP-global-adrs-governance-ML-1B.md")
	writeRoadmapFile(t, "docs/roadmaps/done", "ROADMAP-global-adrs-governance.md")

	if err := MoveRoadmap("ROADMAP-global-adrs-governance", "wip"); err != nil {
		t.Fatalf("nome exato deveria resolver, erro: %v", err)
	}
	if _, statErr := os.Stat("docs/roadmaps/wip/ROADMAP-global-adrs-governance.md"); statErr != nil {
		t.Errorf("o arquivo exato não foi movido: %v", statErr)
	}
	if _, statErr := os.Stat("docs/roadmaps/backlog/ROADMAP-global-adrs-governance-ML-1B.md"); statErr != nil {
		t.Errorf("o irmão maior foi movido no lugar do arquivo exato: %v", statErr)
	}
}

// TestRoadmapCandidateFiles_IgnoresNonMarkdownAndDirectories
//
// Reconciliação: afirma a medição que autorizou o filtro `.md` — os únicos arquivos
// não-`.md` do corpus real (`.trackfw-log`, `.DS_Store`) vivem na raiz de
// docs/roadmaps e não nas pastas de estado; com a coleta-de-todos, um entry estranho
// dentro da pasta de estado transformaria um casamento único em recusa.
func TestRoadmapCandidateFiles_IgnoresNonMarkdownAndDirectories(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)

	writeRoadmapFile(t, "docs/roadmaps/backlog", "ROADMAP-alvo.md")
	if err := os.WriteFile("docs/roadmaps/backlog/ROADMAP-alvo.md.swp", []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs/roadmaps/backlog/ROADMAP-alvo-dir", 0755); err != nil {
		t.Fatal(err)
	}

	got := roadmapCandidateFiles(config.Load())
	if len(got) != 1 || filepath.Base(got[0]) != "ROADMAP-alvo.md" {
		t.Fatalf("candidatos deveriam ser apenas o .md regular, obteve %v", got)
	}
	// E o casamento parcial continua único apesar do lixo ao lado.
	if err := MoveRoadmap("alvo", "wip"); err != nil {
		t.Fatalf("casamento único deveria sobreviver ao lixo não-.md, erro: %v", err)
	}
}

// TestShowRoadmap_EmptyName_Refuses
//
// Reconciliação: afirma o achado da varredura de mesma classe — com UM único roadmap
// no corpus, o glob `*<vazio>*.md` de ShowRoadmap imprimia esse arquivo pelo mesmo
// mecanismo "vazio casa tudo" (com os 228 do corpus real ele já caía na recusa de
// ambiguidade, o que escondia a classe).
func TestShowRoadmap_EmptyName_Refuses(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	mkRoadmapDirs(t)
	writeRoadmapFile(t, "docs/roadmaps/backlog", "ROADMAP-unico.md")

	if err := ShowRoadmap(""); err == nil {
		t.Fatal("ShowRoadmap(\"\") deveria recusar, retornou nil")
	} else if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("erro deve nomear o nome vazio; got: %v", err)
	}
	// Contra-braço: nome existente continua imprimindo.
	if err := ShowRoadmap("unico"); err != nil {
		t.Errorf("ShowRoadmap com nome válido deveria funcionar: %v", err)
	}
}

// ─── REQ (Família 2 — mesma causa, Regra Dura de Causa Raiz) ──────────────────

func writeREQFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	body := "---\nstatus: backlog\ndate: 2026-09-26\n---\n\n# REQ: " + name + "\n\n> Date: 2026-09-26 | Status: backlog\n"
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", p, err)
	}
	return p
}

// TestMoveREQ_EmptyName_RefusesAndRewritesNothing
//
// Reconciliação: afirma o sítio PIOR medido na varredura de mesma classe — a query
// vazia casava os 231 REQs do corpus real e MoveREQ REESCREVIA o status dentro do
// primeiro arquivo, não apenas o movia; o guard de vazio que MoveREQ já tinha era
// sobre o `status`, nunca sobre o `name`.
func TestMoveREQ_EmptyName_RefusesAndRewritesNothing(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	first := writeREQFile(t, "docs/req/backlog", "REQ-aaa-primeiro.md")
	second := writeREQFile(t, "docs/req/backlog", "REQ-bbb-segundo.md")
	before, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}

	if err := MoveREQ("", "wip"); err == nil {
		t.Fatal("MoveREQ(\"\") deveria recusar, retornou nil")
	} else if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("erro deve nomear o nome vazio; got: %v", err)
	}

	after, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("o primeiro REQ deveria continuar em backlog: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("conteúdo do primeiro REQ foi reescrito:\nantes:\n%s\ndepois:\n%s", before, after)
	}
	if _, statErr := os.Stat(second); statErr != nil {
		t.Errorf("o segundo REQ deveria continuar em backlog: %v", statErr)
	}
	if moved, _ := filepath.Glob("docs/req/wip/*.md"); len(moved) != 0 {
		t.Errorf("nenhum REQ deveria ter sido movido, obteve %v", moved)
	}
}

// TestMoveREQ_AmbiguousPartial_RefusesNamingCandidates
//
// Reconciliação: afirma os 36 de 231 fragmentos de 18 caracteres do corpus real de
// REQs que casam mais de um arquivo — hoje escolhidos em silêncio, agora recusados
// com os candidatos nomeados.
func TestMoveREQ_AmbiguousPartial_RefusesNamingCandidates(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	a := writeREQFile(t, "docs/req/backlog", "REQ-2026-07-19-global-adrs-um.md")
	b := writeREQFile(t, "docs/req/backlog", "REQ-2026-07-19-global-adrs-dois.md")

	err := MoveREQ("global-adrs", "wip")
	if err == nil {
		t.Fatal("nome ambíguo deveria recusar, retornou nil")
	}
	for _, want := range []string{"multiple REQs match", a, b} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro deveria conter %q; got: %v", want, err)
		}
	}
	if moved, _ := filepath.Glob("docs/req/wip/*.md"); len(moved) != 0 {
		t.Errorf("nenhum REQ deveria ter sido movido, obteve %v", moved)
	}
}

// TestMoveREQ_ExactNameWinsOverLongerSibling
//
// Reconciliação: afirma os 3 pares medidos no corpus real de REQs em que o stem é
// prefixo de um irmão maior (`REQ-2026-07-19-global-adrs-governance` contra
// `...-ML-1B.md`) — nome completo resolve exato, logo nenhuma recusa nova para
// queries de nome completo (228 de 231 já eram únicas).
func TestMoveREQ_ExactNameWinsOverLongerSibling(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	sibling := writeREQFile(t, "docs/req/backlog", "REQ-global-adrs-governance-ML-1B.md")
	writeREQFile(t, "docs/req/backlog", "REQ-global-adrs-governance.md")

	if err := MoveREQ("REQ-global-adrs-governance", "wip"); err != nil {
		t.Fatalf("nome exato deveria resolver, erro: %v", err)
	}
	if _, statErr := os.Stat("docs/req/wip/REQ-global-adrs-governance.md"); statErr != nil {
		t.Errorf("o REQ exato não foi movido: %v", statErr)
	}
	if _, statErr := os.Stat(sibling); statErr != nil {
		t.Errorf("o irmão maior foi movido no lugar do REQ exato: %v", statErr)
	}
}
