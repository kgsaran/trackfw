package generators

// ML-1A (REQ-2026-09-28-trackfw-init-reexecutado): testes do merge textual de
// writeTrackfwConfig. Cada teste é acompanhado da frase de reconciliação exigida
// pela Regra Dura de Reconciliação (CLAUDE.md):
//
//   T1 AbsenteEscreve                  → arquivo ausente produz template completo
//   T2 PresenteTodasChavesNoOp         → AC principal: zero diff nas linhas pré-existentes (incl. comentários e chaves do consumidor)
//   T3 ChaveFaltandoAcrescentada       → chave ausente é acrescentada sem tocar conteúdo pré-existente
//   T4 BlocoMultilinhaAcrescentado     → bloco multi-linha (chave + corpo indentado) não emite linha-órfã
//   T5 ChaveComentadaEhAusente         → P3: chave comentada conta como ausente e é acrescentada
//   T6 PrefixoColisaoPrevenida         → P2: roadmap_dir presente não impede acréscimo de roadmap_namespacing
//   T7 PresentTopLevelKeys_P1P2P3      → unidade: P1 (coluna 0), P2 (dois-pontos), P3 (comentário)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureAllKeys retorna um trackfw.yaml mínimo que contém TODAS as chaves
// geradas por writeTrackfwConfig(Config{}) mais chaves de consumidor:
//   - governance_mode / lenient_until  (adicionadas pelo consumidor, não geradas por Config{})
//   - agent_models com comentário de justificativa (chave 100% do consumidor)
//
// A fixture foi construída de modo que presentTopLevelKeys(fixture) ⊇ chaves
// de parseConfigBlocks(template de Config{}), garantindo que o merge seja
// no-op para essa configuração.
const fixtureAllKeys = `# trackfw configuration — gerado por trackfw discover
# governance_mode: lenient permite validação não-bloqueante durante onboarding

governance_mode: lenient
lenient_until: "2027-12-31"

adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: github-actions
forge: github

# Versão do modelo por tier — justificativa de cota de API aprovada em 2026-09.
# Manter este bloco: o orquestrador usa orchestrator; implementadores usam implementer.
agent_models:
  orchestrator: claude-opus-4
  implementer: claude-sonnet-4-6

frontend: ""
backend: go
backend_framework: ""
pkg_manager: ""
wip_limit: 1
wip_by_squad: false
require_req_in_commit: false

# validator rules (off / warning / error)
rules:
  branch_has_wip_roadmap: error
`

// T1 — arquivo ausente → escreve template completo.
//
// Reconciliação: este teste afirma que o caminho de criação (arquivo ausente)
// preserva o comportamento original — writeTrackfwConfig escreve o template
// completo quando trackfw.yaml não existe.
func TestWriteTrackfwConfig_AbsenteEscreve(t *testing.T) {
	dir := chdirTemp(t)
	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "trackfw.yaml"))
	if err != nil {
		t.Fatalf("ler trackfw.yaml: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("trackfw.yaml criado está vazio")
	}
	// Verifica chaves essenciais presentes no template.
	for _, key := range []string{"frontend:", "rules:", "adr_dirs:", "roadmap_dir:"} {
		if !strings.Contains(string(data), key) {
			t.Errorf("chave %q ausente no template criado", key)
		}
	}
}

// T2 — AC PRINCIPAL: arquivo presente com todas as chaves → no-op byte-a-byte.
//
// Reconciliação: este teste afirma a conclusão central do ML-1A — quando
// trackfw.yaml já contém todas as chaves geradas pelo template, writeTrackfwConfig
// é um no-op completo: o arquivo é byte-idêntico antes e depois, preservando
// comentários, chaves do consumidor (agent_models com comentário de justificativa)
// e as chaves governance_mode/lenient_until. Como o arquivo não muda, a saída de
// `trackfw validate` seria necessariamente idêntica antes e depois, que é o
// critério de aceite principal desta REQ.
func TestWriteTrackfwConfig_PresenteTodasChavesNoOp(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")
	if err := os.WriteFile(path, []byte(fixtureAllKeys), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler trackfw.yaml: %v", err)
	}
	if string(got) != fixtureAllKeys {
		t.Fatalf("arquivo alterado — deveria ser no-op:\ngot:\n%s\nwant:\n%s", string(got), fixtureAllKeys)
	}

	// Segunda execução: também deve ser no-op.
	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig (2ª): %v", err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != fixtureAllKeys {
		t.Fatal("segunda execução alterou o arquivo — idempotência quebrada")
	}
}

// T3 — chave ausente é acrescentada sem tocar conteúdo pré-existente.
//
// Reconciliação: este teste afirma que quando uma chave está ausente do arquivo
// existente, ela é acrescentada ao final sem modificar nenhuma linha pré-existente
// — garantindo "zero diff" no conteúdo original e o correto funcionamento do
// caminho de atualização pós-upgrade.
func TestWriteTrackfwConfig_ChaveFaltandoAcrescentada(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// Fixture sem roadmap_namespacing.
	fixture := `adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
hooks: none
ci: github-actions
frontend: ""
backend: go
backend_framework: ""
pkg_manager: ""
wip_limit: 1
wip_by_squad: false
require_req_in_commit: false
rules:
  branch_has_wip_roadmap: error
`
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler trackfw.yaml: %v", err)
	}

	// Conteúdo original deve ser preservado byte-a-byte no início.
	if !strings.HasPrefix(string(got), fixture) {
		t.Fatalf("conteúdo pré-existente foi alterado:\ngot prefix:\n%s\nwant prefix:\n%s",
			string(got)[:min(len(fixture)+20, len(got))], fixture)
	}

	// A chave ausente deve ter sido acrescentada.
	if !strings.Contains(string(got), "roadmap_namespacing:") {
		t.Fatal("roadmap_namespacing não foi acrescentada")
	}
}

// T4 — bloco multi-linha é acrescentado como unidade (sem linha-órfã).
//
// Reconciliação: este teste afirma que quando `rules:` está ausente, o bloco
// inteiro (chave + corpo indentado `  branch_has_wip_roadmap: error`) é
// acrescentado — nunca o corpo isolado, o que produziria YAML inválido.
func TestWriteTrackfwConfig_BlocoMultilinhaAcrescentado(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// Fixture sem `rules:`.
	fixture := `adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: github-actions
forge: github
frontend: ""
backend: go
backend_framework: ""
pkg_manager: ""
wip_limit: 1
wip_by_squad: false
require_req_in_commit: false
`
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := writeTrackfwConfig(Config{Forge: "github"}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler trackfw.yaml: %v", err)
	}
	content := string(got)

	// A chave rules: deve estar presente.
	if !strings.Contains(content, "rules:") {
		t.Fatal("rules: não foi acrescentada")
	}

	// O corpo deve aparecer DEPOIS de rules:, não antes (sem linha-órfã).
	rulesIdx := strings.Index(content, "rules:")
	bodyIdx := strings.Index(content, "  branch_has_wip_roadmap:")
	if bodyIdx < 0 {
		t.Fatal("corpo de rules: não encontrado no arquivo")
	}
	if bodyIdx < rulesIdx {
		t.Fatalf("linha-órfã: corpo indentado aparece antes da chave rules:\nbodyIdx=%d rulesIdx=%d", bodyIdx, rulesIdx)
	}

	// Conteúdo original preservado.
	if !strings.HasPrefix(content, fixture) {
		t.Fatal("conteúdo pré-existente foi alterado")
	}
}

// T5 — P3: chave comentada conta como ausente.
//
// Reconciliação: este teste afirma a propriedade P3 da spec — `# governance_mode:
// lenient` em comentário NÃO satisfaz a presença de `governance_mode`, portanto
// a chave é acrescentada quando o template a gera (BrownfieldMode).
func TestWriteTrackfwConfig_ChaveComentadaEhAusente(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// governance_mode aparece só em comentário; lenient_until ausente.
	fixture := `# governance_mode: lenient
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: github-actions
frontend: ""
backend: go
backend_framework: ""
pkg_manager: ""
wip_limit: 1
wip_by_squad: false
require_req_in_commit: false
rules:
  branch_has_wip_roadmap: error
`
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	lenientDate := time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)
	if err := writeTrackfwConfig(Config{BrownfieldMode: true, LenientUntil: lenientDate}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler trackfw.yaml: %v", err)
	}
	content := string(got)

	// governance_mode deve ter sido acrescentada como chave real.
	// Verifica presença fora de comentário: procura linha que começa com "governance_mode:".
	found := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") && strings.HasPrefix(line, "governance_mode:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("governance_mode: não foi acrescentada como chave real (P3 quebrado)")
	}
}

// T6 — P2: roadmap_dir presente não impede acréscimo de roadmap_namespacing.
//
// Reconciliação: este teste afirma a propriedade P2 da spec — a verificação de
// presença usa key+":" (não apenas key como prefixo), evitando que `roadmap_dir`
// seja confundido com `roadmap_namespacing`.
func TestWriteTrackfwConfig_PrefixoColisaoPrevenida(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// Fixture tem roadmap_dir mas NÃO roadmap_namespacing.
	fixture := `adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
hooks: none
ci: github-actions
frontend: ""
backend: go
backend_framework: ""
pkg_manager: ""
wip_limit: 1
wip_by_squad: false
require_req_in_commit: false
rules:
  branch_has_wip_roadmap: error
`
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler trackfw.yaml: %v", err)
	}
	if !strings.Contains(string(got), "roadmap_namespacing:") {
		t.Fatal("roadmap_namespacing não foi acrescentada — P2 possivelmente quebrado (colisão de prefixo)")
	}
}

// T7 — unidade de presentTopLevelKeys: P1 (coluna 0), P2 (dois-pontos), P3 (comentário).
//
// Reconciliação: este teste afirma que presentTopLevelKeys detecta corretamente
// chaves presentes e ausentes de acordo com as propriedades P1-P3, que são os
// predicados que governam se um bloco é acrescentado ou ignorado no merge.
func TestPresentTopLevelKeys_P1P2P3(t *testing.T) {
	content := `# roadmap_dir: docs/roadmaps
roadmap_dir: docs/roadmaps
  roadmap_namespacing: flat
ci: github-actions
`
	keys := presentTopLevelKeys(content)

	// P3: comentado — deve ser detectado como presente porque roadmap_dir aparece
	// também como chave real na linha seguinte.
	if !keys["roadmap_dir"] {
		t.Error("roadmap_dir deveria estar presente (linha real, não apenas comentário)")
	}
	// P1: indentado — roadmap_namespacing NÃO deve ser detectado como chave de nível 0.
	if keys["roadmap_namespacing"] {
		t.Error("roadmap_namespacing indentado não deveria ser detectado como chave P1")
	}
	// P2: ci: presente.
	if !keys["ci"] {
		t.Error("ci deveria estar presente")
	}

	// P3 puro: apenas comentário.
	contentOnlyComment := `# governance_mode: lenient
other_key: value
`
	keys2 := presentTopLevelKeys(contentOnlyComment)
	if keys2["governance_mode"] {
		t.Error("governance_mode apenas em comentário não deveria ser detectado como presente (P3)")
	}
}

// min é um auxiliar para evitar índice fora de intervalo em mensagens de erro.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
