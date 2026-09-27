package validator

// ML-1E (REQ-2026-09-09, 2026-09-26) — as três regras de vínculo restantes (`req_has_adr`,
// `wip_has_req`, `blocked_has_req`) passam a ler o vínculo por contentHasStructuredRefValue, o MESMO
// leitor que o ML-1D instalou em `req_has_roadmap`.
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara, em uma frase, qual conclusão do ML-1E
// ele afirma. A medição que essas frases citam está no relatório do ML-1E e no comentário de cada
// regra em validator.go.

import (
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ruleFired reporta se alguma mensagem (violation OU warning) casa needle — a severidade depende do
// trackfw.yaml do projeto sob teste, e o que este ML mede é o VEREDITO da regra, não o canal.
func ruleFired(t *testing.T, needle string) bool {
	t.Helper()
	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, m := range append(append([]string{}, violations...), warnings...) {
		if hasViolation([]string{m}, needle) {
			return true
		}
	}
	return false
}

// writeREQWithADR grava uma REQ com o campo `adr:` do frontmatter e o marcador `ADR:` do corpo
// controlados independentemente. Valor "" ⇒ o campo é gravado como `adr: ""` (a forma que o gerador
// emite) e o corpo recebe um comentário HTML — as duas grafias reais de "não preenchido".
func writeREQWithADR(t *testing.T, dir, name, fmADR, bodyADR string) {
	t.Helper()
	fm := "---\nstatus: Open\ndate: 2026-09-26\nadr: \"" + fmADR + "\"\nroadmap: \"docs/roadmaps/done/ROADMAP-x.md\"\n---\n"
	body := "\n# REQ: Fixture\n\n> Date: 2026-09-26 | Status: Open\n\n## Linked ADR\n"
	if bodyADR != "" {
		body += "ADR: " + bodyADR + "\n"
	} else {
		body += "ADR: <!-- a preencher -->\n"
	}
	writeFile(t, dir, filepath.Join("docs/req", name), fm+body)
}

// ---------------------------------------------------------------------------
// req_has_adr
// ---------------------------------------------------------------------------

// TestValidateREQsHaveADR_FrontmatterPathPassa — afirma a medição de abertura do ML-1E: 7 REQs do
// acervo declaravam `adr: "docs/adr/ADR-….md"` no frontmatter, com o alvo existente no disco, e eram
// acusadas de "has no linked ADR" porque contentHasMarkerValue casava "ADR:" por prefixo
// case-sensitive e nunca via o campo minúsculo do frontmatter. Uma das 7 é a REQ desta campanha.
func TestValidateREQsHaveADR_FrontmatterPathPassa(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithADR(t, dir, "REQ-fm-adr.md", "docs/adr/ADR-2026-09-26-x.md", "")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if ruleFired(t, "no linked ADR") {
		t.Error("frontmatter `adr:` com caminho .md NÃO deve disparar req_has_adr (era a acusação falsa medida no ML-1E)")
	}
}

// TestValidateREQsHaveADR_TravessaoNoFrontmatterContinuaAcusada — contra-braço exigido pelo ML-1E:
// afirma que as 2 REQs do acervo cujo `adr:` é um travessão (`—`) continuam acusadas depois da
// migração; se passassem, teríamos trocado o falso positivo por um falso negativo.
func TestValidateREQsHaveADR_TravessaoNoFrontmatterContinuaAcusada(t *testing.T) {
	for _, placeholder := range []string{"—", "-", "–", "none", "TBD", "N/A"} {
		t.Run(placeholder, func(t *testing.T) {
			dir := buildReqRoadmapDir(t)
			writeREQWithADR(t, dir, "REQ-dash-adr.md", placeholder, "")
			config.Reset()
			chdir(t, dir)
			t.Cleanup(config.Reset)

			if !ruleFired(t, "no linked ADR") {
				t.Errorf("placeholder %q no `adr:` do frontmatter DEVE continuar disparando req_has_adr", placeholder)
			}
		})
	}
}

// TestValidateREQsHaveADR_ProsaNoCorpoPassaAAcusar — afirma a direção OPOSTA e maior da medição do
// ML-1E: 24 REQs satisfaziam a regra com um placeholder em prosa no corpo (`ADR: N/A — …`,
// `ADR: (a decidir …`), nenhuma delas com ADR; o leitor novo exige caminho ".md" e passa a acusá-las.
// É a explicação do delta +24 medido no acervo (128 → 145).
func TestValidateREQsHaveADR_ProsaNoCorpoPassaAAcusar(t *testing.T) {
	for _, prose := range []string{
		"N/A — extensão de um gate de validação já existente (`branch_has_wip_roadmap`)",
		"(a decidir — AC2 e AC4 podem exigir uma decisão registrada sobre qual é o idioma base)",
		"<!-- nenhum: correção de contradição interna e de método de auditoria. A decisão de CI que",
	} {
		t.Run(prose[:12], func(t *testing.T) {
			dir := buildReqRoadmapDir(t)
			writeREQWithADR(t, dir, "REQ-prose-adr.md", "", prose)
			config.Reset()
			chdir(t, dir)
			t.Cleanup(config.Reset)

			if !ruleFired(t, "no linked ADR") {
				t.Errorf("prosa %q no marcador `ADR:` do corpo DEVE disparar req_has_adr", prose)
			}
		})
	}
}

// TestValidateREQsHaveADR_CorpoComCaminhoRealPassa — contra-braço de não-regressão: afirma que a
// migração não estreitou a fonte de verdade, apenas a ampliou — o vínculo declarado só no CORPO
// (a forma canônica que `trackfw req new` gera) continua contando, inclusive entre backticks.
func TestValidateREQsHaveADR_CorpoComCaminhoRealPassa(t *testing.T) {
	for _, value := range []string{"docs/adr/ADR-2026-09-26-x.md", "`docs/adr/ADR-2026-09-26-x.md`"} {
		t.Run(value, func(t *testing.T) {
			dir := buildReqRoadmapDir(t)
			writeREQWithADR(t, dir, "REQ-body-adr.md", "", value)
			config.Reset()
			chdir(t, dir)
			t.Cleanup(config.Reset)

			if ruleFired(t, "no linked ADR") {
				t.Errorf("vínculo de corpo %q NÃO deve disparar req_has_adr", value)
			}
		})
	}
}

// TestValidateREQsHaveADR_CustomLinkFieldHonrado — afirma que a migração preservou a
// configurabilidade de `link_fields.adr`: contentHasStructuredRefValue deriva o field de cada marker
// configurado, sem o ":" final.
func TestValidateREQsHaveADR_CustomLinkFieldHonrado(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "trackfw.yaml", "link_fields:\n  adr:\n    - adr_ref\n")
	writeFile(t, dir, filepath.Join("docs/req", "REQ-custom-adr.md"),
		"---\nstatus: Open\ndate: 2026-09-26\nroadmap: \"docs/roadmaps/done/ROADMAP-x.md\"\n---\n\n# REQ: Fixture\n\nadr_ref: docs/adr/ADR-2026-09-26-x.md\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if ruleFired(t, "no linked ADR") {
		t.Error("marcador customizado (link_fields.adr=[adr_ref]) com valor .md NÃO deve disparar req_has_adr")
	}
}

// ---------------------------------------------------------------------------
// wip_has_req / blocked_has_req
// ---------------------------------------------------------------------------

func writeRoadmapWithREQ(t *testing.T, dir, state, name, fmREQ, bodyREQ string) {
	t.Helper()
	content := "---\nstatus: " + state + "\nreq: \"" + fmREQ + "\"\n---\n\n# Roadmap: Fixture\n\n"
	if bodyREQ != "" {
		content += "REQ: " + bodyREQ + "\n\n"
	}
	content += "## Acceptance Criteria\n- [ ] ok\n"
	writeFile(t, dir, filepath.Join("docs/roadmaps", state, name), content)
}

// TestValidateWIPHasREQ_FrontmatterPathPassa — afirma a conclusão estrutural do ML-1E para o campo
// `req:`: o campo que `roadmap new` escreve no frontmatter do roadmap (`req: "docs/req/….md"`) era
// invisível para wip_has_req (casamento case-sensitive de "REQ:") e agora satisfaz a regra. Medição
// própria: 0 instâncias no acervo deste repositório — o defeito é de mecanismo, não de contagem.
func TestValidateWIPHasREQ_FrontmatterPathPassa(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeRoadmapWithREQ(t, dir, "wip", "ROADMAP-fm-req.md", "docs/req/REQ-x.md", "")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if ruleFired(t, "is in wip but has no linked REQ") {
		t.Error("frontmatter `req:` com caminho .md NÃO deve disparar wip_has_req")
	}
}

// TestValidateWIPHasREQ_IdPeladoPassaAAcusar — afirma o estreitamento que o ML-1E introduz no campo
// `req:` e que o relatório entrega ao arquiteto como ponto de ratificação: um ID pelado
// ("REQ-2026-07-29-fixture", sem ".md") deixou de contar como vínculo, porque extractRefPath exige um
// caminho de artefato. Foi o que quebrou 7 testes de barrier/ship e 11 sítios de scripts/.
func TestValidateWIPHasREQ_IdPeladoPassaAAcusar(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeRoadmapWithREQ(t, dir, "wip", "ROADMAP-bare-id.md", "", "REQ-2026-07-29-fixture")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if !ruleFired(t, "is in wip but has no linked REQ") {
		t.Error("ID pelado (sem .md) no marcador `REQ:` DEVE disparar wip_has_req depois do ML-1E")
	}
}

// TestValidateBlockedHasREQ_FrontmatterPathPassa — afirma que blocked_has_req recebeu o MESMO
// tratamento de wip_has_req (e não foi presumido equivalente): o `req:` do frontmatter satisfaz a
// regra em blocked/ também. Medição própria em blocked/: 0 violações antes, 0 depois.
func TestValidateBlockedHasREQ_FrontmatterPathPassa(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeRoadmapWithREQ(t, dir, "blocked", "ROADMAP-fm-req-blocked.md", "docs/req/REQ-x.md", "")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if ruleFired(t, "is in blocked but has no linked REQ") {
		t.Error("frontmatter `req:` com caminho .md NÃO deve disparar blocked_has_req")
	}
}

// TestValidateBlockedHasREQ_PlaceholderContinuaAcusado — contra-braço de blocked_has_req: afirma que
// a regra não virou permissiva no estado blocked/ — placeholder continua acusado.
func TestValidateBlockedHasREQ_PlaceholderContinuaAcusado(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeRoadmapWithREQ(t, dir, "blocked", "ROADMAP-placeholder-blocked.md", "none", "TBD")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if !ruleFired(t, "is in blocked but has no linked REQ") {
		t.Error("placeholder no `req:`/`REQ:` DEVE disparar blocked_has_req")
	}
}

// TestValidateREQsHaveRoadmap_ML1DNaoDesfeito — afirma o critério de aceite "ML-1B/1C/1D não são
// desfeitos" na parte que este ML podia quebrar: o leitor compartilhado continua servindo
// req_has_roadmap, então placeholder no campo `roadmap:` segue acusado e caminho real segue passando.
func TestValidateREQsHaveRoadmap_ML1DNaoDesfeito(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-ml1d-placeholder.md", "none", "none")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	if !ruleFired(t, "no linked Roadmap") {
		t.Error("ML-1D desfeito: `roadmap: none` voltou a contar como vínculo")
	}
}
