package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// ML-1E, ação 2 (REQ-2026-09-09) — A INSTÂNCIA DO WIZARD, CONSTRUÍDA.
//
// O handoff pedia: "construa uma instância antes de corrigir, ou declare a ausência no contrato. Não
// corrija o que não consegue reproduzir." Este teste é a instância — e o veredito dela é o que o
// contrato (docs/cli-parity.md, seção "`req new` — o elo com o ADR do wizard NÃO é escrito") declara.
//
// O que o wizard faz (internal/commands/req.go:138): para cada probe respondida, chama
// generators.NewADRDraft e acumula o basename em content.DependsOnADRs. NewREQ (internal/generators/
// req.go:87-131) usa essa lista para (a) o contador da linha de status e (b) a seção
// "## Blocked by ADRs" — e grava `adr: ""` no frontmatter e `ADR: ` vazio no corpo, SEMPRE.
//
// Consequência, agora medida em vez de suposta: uma REQ criada pelo caminho interativo com probes
// respondidas nasce acusada por `req_has_adr`, mesmo tendo um ADR real criado no mesmo ato.
//
// 🔴 Por que este ML NÃO escreve o elo (decisão a ratificar pelo arquiteto, registrada no contrato):
// o ADR que o wizard cria está em status **Draft**, e a seção onde ele é listado ("Blocked by ADRs")
// significa o OPOSTO de "vínculo resolvido" — o próprio wizard imprime "Resolve these ADRs (set
// Status: Accepted) before creating a roadmap". Preencher `adr:` com um Draft faria `req_has_adr`
// passar a chamar de "vinculada" uma REQ cuja decisão ainda não existe, o que é trocar um falso
// positivo por um falso negativo — exatamente o que o contra-braço deste ML proíbe. A alternativa
// (preencher o elo só quando o ADR sair de Draft) é mudança de comportamento do wizard, com superfície
// própria, e o ML-1E já carrega uma mudança de veredito em 31 artefatos.
//
// Reconciliação: este teste afirma a conclusão "o wizard não escreve o elo `adr:`, e a ausência é
// DECLARADA, não corrigida" — é o pin que faz a declaração do contrato falsificável.
func TestREQWizardShape_ADRDraftNaoPreencheOElo_ML1E(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"docs/req", "docs/adr", "docs/roadmaps/wip", "docs/roadmaps/backlog",
		"docs/roadmaps/blocked", "docs/roadmaps/done", "docs/roadmaps/abandoned"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"),
		[]byte("adr_dirs:\n  - docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	config.Reset()
	t.Cleanup(config.Reset)

	// Passo 1 — o que o wizard faz por probe respondida.
	adrBase, err := NewADRDraft("decisao-pendente-do-wizard", "docs/adr")
	if err != nil {
		t.Fatalf("NewADRDraft: %v", err)
	}

	// Passo 2 — o que o wizard passa a NewREQ: os drafts vão em DependsOnADRs, e SÓ ali.
	if err := NewREQ(REQContent{
		Title:         "instancia do wizard do ml1e",
		Criteria:      "- [ ] alguma coisa",
		DependsOnADRs: []string{adrBase},
	}); err != nil {
		t.Fatalf("NewREQ: %v", err)
	}

	matches, err := filepath.Glob("docs/req/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperava exatamente 1 REQ gerada, obteve %v (err=%v)", matches, err)
	}
	generated, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read REQ: %v", err)
	}
	body := string(generated)

	// Afirmação 1 — o ADR real aparece SOMENTE como bloqueio, nunca como vínculo.
	if !strings.Contains(body, "## Blocked by ADRs") || !strings.Contains(body, adrBase) {
		t.Fatalf("esperava o ADR %q listado em '## Blocked by ADRs', obtive:\n%s", adrBase, body)
	}
	if !strings.Contains(body, `adr: ""`) {
		t.Errorf("o frontmatter da REQ gerada deveria manter `adr: \"\"` (o wizard não escreve o elo); obtive:\n%s", body)
	}

	// Afirmação 2 — o veredito do validate sobre essa forma: acusada. É a ausência declarada.
	violations, warnings, err := validator.ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	fired := false
	for _, m := range append(append([]string{}, violations...), warnings...) {
		if strings.Contains(m, "has no linked ADR") {
			fired = true
		}
	}
	if !fired {
		t.Error("contrato desatualizado: a REQ na forma do wizard (ADR Draft só em 'Blocked by ADRs') NÃO é mais acusada por req_has_adr — se o elo passou a ser escrito, docs/cli-parity.md precisa mudar junto")
	}
}
