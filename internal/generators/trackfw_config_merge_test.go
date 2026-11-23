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
//   T7 PresentTopLevelKeys_D2a          → yaml.Node parse: quoted key, BOM, space-before-colon, indented sub-key, invalid YAML
//   T8 PresentTopLevelKeys_CounterArm   → absent key not in map; valid parse returns ok=true
//
// ML-1D (REQ-2026-11-22): corretivo — fallback para YAML inválido; reafirmação P1–P3; ponta a ponta
//
//   T9  PresentTopLevelKeys_P1P2P3            → P1/P2/P3 no yaml.Node path (ok=true) e no fallback path (ok=false)
//   T10 WriteTrackfwConfig_D2a_QuotedAdrDirs  → chave "adr_dirs" entre aspas não ganha segundo bloco após merge
//   T11 WriteTrackfwConfig_D2a_SpaceBeforeColon → chave adr_dirs com espaço antes do : não ganha segundo bloco
//   T12 WriteTrackfwConfig_D2a_BOM            → chave adr_dirs com BOM na linha 1 não ganha segundo bloco
//   T13 GenerateLefthookHook_D2a_QuotedPreCommit → "pre-commit" entre aspas não ganha segundo bloco pre-commit:; trackfw-validate instalado; sem aviso "no 'commands:'"
//   T14 GenerateLefthookHook_D2a_InvalidYAMLFallback → lefthook.yml inválido com pre-commit: → fallback encontra a chave, sem bloco duplicado
//
// ML-1E (inserção reconhece chave pelo parse YAML):
//
//   T15 LefthookValidatePresent_D2a_QuotedKey → lefthookValidatePresent retorna true quando pre-commit: está entre aspas duplas
//
// ML-4C (corretivo da Wave 4 — F-9, F-10, R1):
//
//   T16 PresentTopLevelKeys_D2a_MergeKey        → F-9: <<: *anchor e <<: [*a, *b] têm as chaves expandidas; merge aninhado também
//   T17 WriteTrackfwConfig_D2a_MergeKey         → F-9 E2E: chave herdada por âncora não recebe bloco duplicado após init
//   T18 NormalizeYAMLTopLevelKey_EscapeSequence → F-10: escape YAML "pre\x2dcommit" decodificado como "pre-commit"
//   T19 PresentTopLevelKeysFallback_QuotedKey   → R1: fallback usa normalizeYAMLTopLevelKey; chave entre aspas retorna sem aspas

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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

// T7 — yaml.Node parse (D2a, ADR-2026-11-22): quoted key, BOM-prefixed key,
// key with space-before-colon, indented sub-key not at top level, invalid YAML.
//
// Reconciliação: este teste afirma que presentTopLevelKeys (D2a) usa yaml.Node
// e detecta corretamente:
//   - chave entre aspas ("governance_mode": …) → detectada sem as aspas
//   - chave com BOM (\xef\xbb\xbfadr_dirs: …) → detectada como "adr_dirs"
//   - chave com espaço antes do `:` (adr_dirs : …) → detectada como "adr_dirs"
//   - sub-chave indentada não aparece como chave de nível 0
//   - YAML inválido → retorna (empty, false) sem pânico
func TestPresentTopLevelKeys_D2a(t *testing.T) {
	// quoted key: yaml.Node strips the quotes from the key value.
	quotedContent := `"governance_mode": lenient
roadmap_dir: docs/roadmaps
`
	keys, ok := presentTopLevelKeys(quotedContent)
	if !ok {
		t.Fatal("quoted-key fixture must parse as valid YAML")
	}
	if !keys["governance_mode"] {
		t.Error("quoted key \"governance_mode\" must be detected as governance_mode")
	}
	if !keys["roadmap_dir"] {
		t.Error("roadmap_dir must be detected")
	}

	// BOM-prefixed: yaml.v3 strips BOM from key values.
	bomContent := "\xef\xbb\xbfadr_dirs:\n  - docs/adr\ngovernance_mode: lenient\n"
	keys, ok = presentTopLevelKeys(bomContent)
	if !ok {
		t.Fatal("BOM-prefixed fixture must parse as valid YAML")
	}
	if !keys["adr_dirs"] {
		t.Error("BOM-prefixed key must be detected as adr_dirs (BOM stripped by yaml.v3)")
	}
	if !keys["governance_mode"] {
		t.Error("governance_mode must be detected alongside BOM key")
	}

	// space-before-colon: old text implementation kept trailing space in key name;
	// yaml.Node correctly trims it.
	spaceContent := "adr_dirs : [docs/adr]\ngovernance_mode: lenient\n"
	keys, ok = presentTopLevelKeys(spaceContent)
	if !ok {
		t.Fatal("space-before-colon fixture must parse as valid YAML")
	}
	if !keys["adr_dirs"] {
		t.Errorf("space-before-colon key must be detected as adr_dirs; got keys %v", keys)
	}

	// indented sub-key: nested value must NOT appear as a top-level key.
	nestedContent := `frontend:
  framework: react
roadmap_dir: docs/roadmaps
`
	keys, ok = presentTopLevelKeys(nestedContent)
	if !ok {
		t.Fatal("nested fixture must parse as valid YAML")
	}
	if keys["framework"] {
		t.Error("indented sub-key framework must not appear as a top-level key")
	}
	if !keys["frontend"] {
		t.Error("top-level key frontend must be detected")
	}

	// invalid YAML → (empty, false)
	_, ok = presentTopLevelKeys("key: [unclosed")
	if ok {
		t.Error("invalid YAML must return ok=false")
	}
}

// T8 — counter-arm: absent key not in map; valid parse returns ok=true.
//
// Reconciliação: este teste afirma que uma chave ausente do YAML retorna false
// no mapa (não falso-positivo), e que um parse válido retorna ok=true —
// garantindo que o merge acrescenta a chave ausente em vez de silenciosamente
// assumir que ela já existe.
func TestPresentTopLevelKeys_CounterArm(t *testing.T) {
	content := `roadmap_dir: docs/roadmaps
ci: github-actions
`
	keys, ok := presentTopLevelKeys(content)
	if !ok {
		t.Fatal("valid YAML must return ok=true")
	}
	if keys["governance_mode"] {
		t.Error("governance_mode must not be a false-positive when absent from YAML")
	}
	if keys["lenient_until"] {
		t.Error("lenient_until must not be a false-positive when absent from YAML")
	}
	// Present keys must still be found (the counter-arm does not break detection).
	if !keys["roadmap_dir"] {
		t.Error("roadmap_dir must be detected (counter-arm must not break detection)")
	}
}

// --- ML-1D tests ---

// T9 — P1 (column 0), P2 (prefix isolation), P3 (comment = absent) on the
// yaml.Node path (ok=true) and on the text-fallback path (ok=false).
//
// Reconciliation: this test asserts that presentTopLevelKeys correctly enforces
// P1 (indented key is not top-level), P2 (roadmap_dir does not shadow
// roadmap_namespacing), and P3 (commented key = absent) on the yaml.Node path
// (valid YAML, ok=true), and that the same three properties hold on the text-
// fallback path (invalid YAML, ok=false), proving the fallback preserves the
// pre-D2a behavior without regression.
func TestPresentTopLevelKeys_P1P2P3(t *testing.T) {
	// --- yaml.Node path (valid YAML; ok must be true for each fixture) ---

	// P1: indented key is not a top-level key.
	p1Content := "frontend:\n  roadmap_namespacing: flat\nroadmap_dir: docs/roadmaps\nci: github-actions\n"
	keys, ok := presentTopLevelKeys(p1Content)
	if !ok {
		t.Fatal("P1 fixture must parse as valid YAML (ok=true)")
	}
	if keys["roadmap_namespacing"] {
		t.Error("P1 (yaml.Node path): indented roadmap_namespacing must not appear as a top-level key")
	}
	if !keys["roadmap_dir"] {
		t.Error("P1 (yaml.Node path): roadmap_dir must be detected")
	}

	// P3: commented key = absent.
	p3Content := "# governance_mode: lenient\nother_key: value\n"
	keys, ok = presentTopLevelKeys(p3Content)
	if !ok {
		t.Fatal("P3 fixture must parse as valid YAML (ok=true)")
	}
	if keys["governance_mode"] {
		t.Error("P3 (yaml.Node path): governance_mode in comment must not be detected as present")
	}
	if !keys["other_key"] {
		t.Error("P3 (yaml.Node path): other_key must be detected")
	}

	// P2: prefix isolation — roadmap_dir must not shadow roadmap_namespacing.
	p2Content := "roadmap_dir: docs/roadmaps\nci: github-actions\n"
	keys, ok = presentTopLevelKeys(p2Content)
	if !ok {
		t.Fatal("P2 fixture must parse as valid YAML (ok=true)")
	}
	if keys["roadmap_namespacing"] {
		t.Error("P2 (yaml.Node path): roadmap_namespacing must not be a false-positive when only roadmap_dir is present")
	}
	if !keys["roadmap_dir"] {
		t.Error("P2 (yaml.Node path): roadmap_dir must be detected")
	}

	// --- text-fallback path (invalid YAML; ok must be false) ---
	// Uses an explicitly invalid YAML document (unclosed flow sequence on the
	// first line) followed by lines that exercise P1, P2, P3 of the text
	// detector.
	fallbackFixture := "key: [unclosed\n# governance_mode: lenient\nroadmap_dir: docs/roadmaps\n  roadmap_namespacing: flat\nci: github-actions\n"
	keys, ok = presentTopLevelKeys(fallbackFixture)
	if ok {
		t.Fatal("fallback fixture must NOT parse as valid YAML (expected ok=false)")
	}
	// P1 (fallback): indented line is skipped.
	if keys["roadmap_namespacing"] {
		t.Error("P1 (fallback): indented roadmap_namespacing must not be detected")
	}
	if !keys["roadmap_dir"] {
		t.Error("P1 (fallback): roadmap_dir at column 0 must be detected")
	}
	if !keys["ci"] {
		t.Error("P1 (fallback): ci at column 0 must be detected")
	}
	// P3 (fallback): commented line is skipped.
	if keys["governance_mode"] {
		t.Error("P3 (fallback): governance_mode in comment must not be detected")
	}
	// P2 (fallback): roadmap_dir does not create a false positive for roadmap_namespacing.
	if keys["roadmap_namespacing"] {
		t.Error("P2 (fallback): roadmap_namespacing must not be a false-positive when roadmap_dir is present")
	}
}

// fixtureBaseKeysWithout returns a minimal set of all template keys produced by
// writeTrackfwConfig(Config{}), with adr_dirs omitted so callers can supply it
// in the desired format. The user value for adr_dirs is "docs/decisions" to
// distinguish it from the template value "docs/adr".
const fixtureBaseKeysNoAdrDirs = `req_dir: docs/req
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

// T10 — end-to-end: existing trackfw.yaml with quoted "adr_dirs": key does not
// gain a second adr_dirs block after writeTrackfwConfig.
//
// Reconciliation: this test asserts that the yaml.Node parser detects "adr_dirs"
// (quoted) as the key adr_dirs, so writeTrackfwConfig treats the key as present
// and performs a no-op for that key — measured by confirming that no unquoted
// adr_dirs: line is added and the user's value (docs/decisions) is preserved.
func TestWriteTrackfwConfig_D2a_QuotedAdrDirs(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	fixture := "\"adr_dirs\":\n  - docs/decisions\n" + fixtureBaseKeysNoAdrDirs
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
	content := string(got)

	// No unquoted adr_dirs: line should have been added.
	for _, line := range strings.Split(content, "\n") {
		if line == "adr_dirs:" {
			t.Fatalf("duplicate unquoted adr_dirs: block was added after merge (D2a failed for quoted key)\ncontent:\n%s", content)
		}
	}
	// User's value must be preserved.
	if !strings.Contains(content, "docs/decisions") {
		t.Fatalf("user's adr_dirs value (docs/decisions) was lost\ncontent:\n%s", content)
	}
}

// T11 — end-to-end: existing trackfw.yaml with adr_dirs : (space before colon)
// does not gain a second adr_dirs block after writeTrackfwConfig.
//
// Reconciliation: this test asserts that the yaml.Node parser detects "adr_dirs"
// from "adr_dirs : [docs/decisions]" (key with trailing space stripped by yaml.v3),
// so writeTrackfwConfig treats the key as present and performs a no-op for that
// key — measured by confirming that no adr_dirs: line is added and the user's
// value (docs/decisions) is preserved.
func TestWriteTrackfwConfig_D2a_SpaceBeforeColon(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	fixture := "adr_dirs : [docs/decisions]\n" + fixtureBaseKeysNoAdrDirs
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
	content := string(got)

	// No exact adr_dirs: line should have been added (original uses "adr_dirs :").
	for _, line := range strings.Split(content, "\n") {
		if line == "adr_dirs:" {
			t.Fatalf("duplicate adr_dirs: block was added after merge (D2a failed for space-before-colon)\ncontent:\n%s", content)
		}
	}
	// User's value must be preserved.
	if !strings.Contains(content, "docs/decisions") {
		t.Fatalf("user's adr_dirs value (docs/decisions) was lost\ncontent:\n%s", content)
	}
}

// T12 — end-to-end: existing trackfw.yaml with BOM on line 1 does not gain a
// second adr_dirs block after writeTrackfwConfig.
//
// Reconciliation: this test asserts that the yaml.Node parser strips the UTF-8
// BOM and detects "adr_dirs" correctly, so writeTrackfwConfig treats the key as
// present and performs a no-op for that key — measured by confirming that no
// unquoted adr_dirs: line is added and the user's value (docs/decisions) is
// preserved.
func TestWriteTrackfwConfig_D2a_BOM(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// BOM (\xef\xbb\xbf) prepended to adr_dirs: on line 1.
	bom := "\xef\xbb\xbf"
	fixture := bom + "adr_dirs:\n  - docs/decisions\n" + fixtureBaseKeysNoAdrDirs
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
	content := string(got)

	// Count lines that EXACTLY equal "adr_dirs:" (the template block header).
	// The original BOM line is "\xef\xbb\xbfadr_dirs:" — not equal — so only a
	// newly appended duplicate would match.
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if line == "adr_dirs:" {
			count++
		}
	}
	if count != 0 {
		t.Fatalf("duplicate adr_dirs: block was added after merge (D2a failed for BOM key); count=%d\ncontent:\n%s", count, content)
	}
	// User's value must be preserved.
	if !strings.Contains(content, "docs/decisions") {
		t.Fatalf("user's adr_dirs value (docs/decisions) was lost\ncontent:\n%s", content)
	}
}

// T13 — end-to-end: existing lefthook.yml with "pre-commit": (quoted) does not
// gain a second pre-commit: block after generateGitHooks; trackfw-validate is
// inserted under the existing block; no "no 'commands:'" warning is emitted.
//
// Reconciliation: this test asserts that normalizeYAMLTopLevelKey lets both the
// key-detection path (presentTopLevelKeys) and the insertion-path section tracker
// (generateLefthookHook) recognize "pre-commit" written as "pre-commit": — so
// generateLefthookHook inserts trackfw-validate under the existing quoted block
// instead of (a) appending a duplicate block or (b) printing the "no commands:"
// warning and skipping the insertion (ML-1E).
func TestGenerateLefthookHook_D2a_QuotedPreCommit(t *testing.T) {
	dir := chdirTemp(t)

	// Fixture: pre-commit: written as "pre-commit": (quoted key) with a commands: section.
	fixture := "\"pre-commit\":\n  commands:\n    lint:\n      run: golangci-lint run\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(fixture), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	stdout := captureStdout(t, func() {
		if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
			t.Fatalf("generateGitHooks: %v", err)
		}
	})

	got, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml: %v", err)
	}
	content := string(got)

	// No unquoted top-level pre-commit: line should have been added.
	for _, line := range strings.Split(content, "\n") {
		if line == "pre-commit:" {
			t.Fatalf("duplicate unquoted pre-commit: block was added after merge (D2a failed for quoted key)\ncontent:\n%s", content)
		}
	}

	// trackfw-validate must be installed under the quoted pre-commit block (ML-1E).
	if !strings.Contains(content, "trackfw-validate:") {
		t.Fatalf("trackfw-validate: not inserted under quoted pre-commit block (ML-1E insertion-path fix missing)\ncontent:\n%s", content)
	}

	// No "no 'commands:'" warning should appear — the insertion succeeded (ML-1E).
	if strings.Contains(stdout, "commands:") && strings.Contains(stdout, "pre-commit") {
		t.Fatalf("unexpected 'no commands:' warning printed (ML-1E insertion-path fix missing)\nstdout: %s", stdout)
	}
}

// T14 — end-to-end: existing lefthook.yml with invalid YAML that contains
// pre-commit: does not gain a second pre-commit: block after generateGitHooks
// (guaranteed by the text-fallback introduced in ML-1D).
//
// Reconciliation: this test asserts that when yaml.Unmarshal fails on the
// lefthook.yml content, presentTopLevelKeys falls back to the text detector
// which finds pre-commit:, allowing generateLefthookHook to insert
// trackfw-validate under the existing block instead of appending a full
// duplicate block — measured by confirming that exactly one pre-commit: line
// exists and trackfw-validate: is present. Without the fallback (D2a yaml-only),
// yaml parse failure returns empty keys, the if-branch appends a full block, and
// the count is two.
func TestGenerateLefthookHook_D2a_InvalidYAMLFallback(t *testing.T) {
	dir := chdirTemp(t)

	// Fixture: invalid YAML (unclosed flow on line 1) with pre-commit: at column 0.
	invalidFixture := "{unclosed\npre-commit:\n  commands:\n    lint:\n      run: golangci-lint run\n"

	// Precondition: fixture must be invalid YAML (otherwise the fallback is not exercised).
	_, ok := presentTopLevelKeys(invalidFixture)
	if ok {
		t.Fatal("precondition: invalidFixture must not parse as valid YAML — adjust the fixture if yaml.v3 accepts it")
	}

	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(invalidFixture), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml: %v", err)
	}
	content := string(got)

	// Exactly one pre-commit: line must exist (original; no duplicate appended).
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if line == "pre-commit:" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 pre-commit: line, got %d (fallback must prevent duplicate)\ncontent:\n%s", count, content)
	}
	// trackfw-validate: must have been inserted by the fallback path.
	if !strings.Contains(content, "trackfw-validate:") {
		t.Fatalf("trackfw-validate: not inserted (fallback must find pre-commit: and insert under it)\ncontent:\n%s", content)
	}
}

// T15 — unit: lefthookValidatePresent returns true when trackfw-validate: lives
// under a pre-commit block whose key is written between double quotes.
//
// Reconciliation: this test asserts that normalizeYAMLTopLevelKey lets
// lefthookValidatePresent recognize "pre-commit": as the pre-commit section, so
// that a file with the quoted spelling is treated as already having the hook
// installed — preventing a spurious second insertion (ML-1E detection-path fix).
func TestLefthookValidatePresent_D2a_QuotedKey(t *testing.T) {
	content := "\"pre-commit\":\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n"
	if !lefthookValidatePresent(content) {
		t.Fatalf("lefthookValidatePresent returned false for quoted pre-commit key (ML-1E detection-path fix missing)\ncontent:\n%s", content)
	}
}

// --- ML-4C tests ---

// T16 — presentTopLevelKeys expands YAML merge keys (<<: *anchor and <<: [*a, *b]),
// including nested merge keys inside the anchor target.
//
// Reconciliation: this test asserts that presentTopLevelKeys (F-9, ML-4C) enumerates
// keys inherited via YAML merge (<<) so that writeTrackfwConfig treats them as present
// and does not append template defaults that would silently downgrade user-configured
// values (e.g. require_req_in_commit: true → false).
func TestPresentTopLevelKeys_D2a_MergeKey(t *testing.T) {
	// Single alias: <<: *anchor
	singleAlias := `defaults: &d
  require_req_in_commit: true
  wip_limit: 3
<<: *d
adr_dirs:
  - docs/adr
`
	keys, ok := presentTopLevelKeys(singleAlias)
	if !ok {
		t.Fatal("single-alias fixture must parse as valid YAML (ok=true)")
	}
	if !keys["require_req_in_commit"] {
		t.Error("F-9: require_req_in_commit inherited via <<: *d must be detected as present")
	}
	if !keys["wip_limit"] {
		t.Error("F-9: wip_limit inherited via <<: *d must be detected as present")
	}
	if !keys["adr_dirs"] {
		t.Error("adr_dirs (explicit key) must still be detected")
	}

	// Sequence form: <<: [*a, *b]
	seqForm := `a: &a
  foo: 1
b: &b
  bar: 2
<<: [*a, *b]
baz: 3
`
	keys, ok = presentTopLevelKeys(seqForm)
	if !ok {
		t.Fatal("sequence-alias fixture must parse as valid YAML (ok=true)")
	}
	if !keys["foo"] {
		t.Error("F-9: foo inherited via <<: [*a, ...] must be detected as present")
	}
	if !keys["bar"] {
		t.Error("F-9: bar inherited via <<: [..., *b] must be detected as present")
	}
	if !keys["baz"] {
		t.Error("baz (explicit key) must still be detected")
	}

	// Nested merge: anchor that itself has a <<: *inner merge key.
	nestedMerge := `base: &base
  x: 1
extended: &ext
  <<: *base
  y: 2
<<: *ext
z: 3
`
	keys, ok = presentTopLevelKeys(nestedMerge)
	if !ok {
		t.Fatal("nested-merge fixture must parse as valid YAML (ok=true)")
	}
	if !keys["x"] {
		t.Error("F-9: x inherited transitively via nested merge must be detected as present")
	}
	if !keys["y"] {
		t.Error("F-9: y inherited via <<: *ext must be detected as present")
	}
	if !keys["z"] {
		t.Error("z (explicit key) must still be detected")
	}

	// Counter-arm: a key genuinely absent must still be absent.
	if keys["absent_key"] {
		t.Error("absent_key must not be a false-positive (counter-arm)")
	}
}

// T17 — end-to-end: trackfw.yaml with governance keys inherited via YAML merge key
// (<<: *anchor) keeps the user's values after writeTrackfwConfig.
//
// Reconciliation: this test asserts that presentTopLevelKeys (F-9, ML-4C) prevents
// writeTrackfwConfig from appending template defaults for keys that exist only via
// merge-key expansion — confirming the downgrade scenario (require_req_in_commit:
// true → false, wip_limit: 3 → 1) cannot occur after the fix.
//
// config.Load() is NOT used to assert values: the yaml.Node reader does not expand
// merge keys, so it always returns defaults for anchor-inherited values regardless
// of whether the file was modified. The assertion is by absence of appended defaults
// and by yaml.Unmarshal-into-map, which does expand merge keys (confirmed by probe).
func TestWriteTrackfwConfig_D2a_MergeKey(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// Fixture: governance keys defined in an anchor and merged at top level.
	// require_req_in_commit: true and wip_limit: 3 are only present via <<: *d.
	fixture := `defaults: &d
  require_req_in_commit: true
  wip_limit: 3
<<: *d
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
wip_by_squad: false
rules:
  branch_has_wip_roadmap: error
`
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(got)

	// No explicit require_req_in_commit or wip_limit line should have been appended.
	// The template writes "require_req_in_commit: false" and "wip_limit: 1" when it
	// believes the keys are absent. If present here, the fix failed.
	for _, line := range strings.Split(content, "\n") {
		if line == "require_req_in_commit: false" {
			t.Fatalf("F-9: require_req_in_commit: false was appended (merge key not expanded)\ncontent:\n%s", content)
		}
		if line == "wip_limit: 1" {
			t.Fatalf("F-9: wip_limit: 1 was appended (merge key not expanded)\ncontent:\n%s", content)
		}
	}

	// Absence of appended defaults is sufficient: writeTrackfwConfig is a no-op for
	// present keys, so if neither default line appeared, the user's anchor-merged
	// values are intact. config.Load() is not used here because yaml.Node (which
	// config.go uses internally) does not expand merge keys — it always returns
	// defaults for anchor-inherited values regardless of whether the file changed.
	// Confirmed by measurement in probe_mergekey_test (session 2026-11-22).
}

// T18 — normalizeYAMLTopLevelKey decodes YAML escape sequences in quoted keys.
//
// Reconciliation: this test asserts that the YAML-parser primary path in
// normalizeYAMLTopLevelKey (F-10, ML-4C) decodes "pre\x2dcommit" (a YAML
// double-quoted scalar with escape \x2d = '-') as "pre-commit", which the
// text-based fallback cannot do (it returns the literal "pre\x2dcommit").
func TestNormalizeYAMLTopLevelKey_EscapeSequence(t *testing.T) {
	// Raw string: contains the literal bytes "pre\x2dcommit": (backslash, x, 2, d).
	// In YAML double-quoted string semantics, \x2d is the Unicode escape for '-',
	// so the parsed key is "pre-commit".
	line := `"pre\x2dcommit":`
	got := normalizeYAMLTopLevelKey(line)
	if got != "pre-commit" {
		t.Errorf("F-10: normalizeYAMLTopLevelKey(%q) = %q, want \"pre-commit\""+
			" (YAML \\x2d escape must be decoded by the YAML-parser primary path)", line, got)
	}
}

// T19 — presentTopLevelKeysFallback uses normalizeYAMLTopLevelKey so that a quoted
// key ("pre-commit":) in an otherwise-invalid YAML file is stored without quotes.
//
// Reconciliation: this test asserts that presentTopLevelKeysFallback (R1, ML-4C)
// calls normalizeYAMLTopLevelKey on each column-0 line instead of using line[:idx].
// Without R1, the key is stored as '"pre-commit"' (with quotes); the subsequent
// caller looks up "pre-commit" (without quotes) and gets false, triggering a
// duplicate block insertion.
func TestPresentTopLevelKeysFallback_QuotedKey(t *testing.T) {
	// Invalid YAML overall (unclosed flow) but "pre-commit": is a valid column-0 line.
	content := "{unclosed\n\"pre-commit\":\n  commands:\n    lint:\n      run: golangci-lint run\n"

	// Precondition: fixture must be invalid YAML (the fallback path is exercised).
	_, ok := presentTopLevelKeys(content)
	if ok {
		t.Fatal("precondition: fixture must NOT parse as valid YAML — adjust if yaml.v3 accepts it")
	}

	keys := presentTopLevelKeysFallback(content)
	if !keys["pre-commit"] {
		t.Errorf(`R1: presentTopLevelKeysFallback must return keys["pre-commit"]=true for a quoted key; `+
			`keys=%v — without normalizeYAMLTopLevelKey, the key would be stored as '"pre-commit"' (with quotes)`, keys)
	}
	// Keys with the raw quoted form must not appear.
	if keys[`"pre-commit"`] {
		t.Errorf(`R1: keys['"pre-commit"'] must be false; the quoted form must not persist as a key`)
	}
}

// --- ML-4E tests: cycle guard for the writer's key detector ---
//
// presentTopLevelKeys (via expandTopLevelKeys / expandMergeValue) can reach a cycle
// when the top-level mapping has a <<: that transitively points back to itself.
// The fixtures below all include a top-level <<: *anchor so that expandMergeValue is
// called on the cyclic node — without that, the writer's traversal never enters the
// anchor's content and the cycle is not reachable from the writer path.
//
// Counter-arm (without the guard): the ML-4D implementation crashes with
// "goroutine stack exceeds" for the self-referential mapping fixture. Measured by
// running the pre-ML-4E test binary (from `git archive HEAD` in $TMPDIR) with
// -run TestPresentTopLevelKeys_Cyclic -timeout 10s and observing rc!=0.
//
// T20 PresentTopLevelKeys_CyclicSelfRefMapping    — d: &d / <<: *d doesn't overflow; explicit keys returned
// T21 PresentTopLevelKeys_CyclicSequence          — <<: *x where x is a self-ref sequence doesn't overflow
// T22 PresentTopLevelKeys_CyclicMutualNested      — nested mutual cycle doesn't overflow; explicit keys returned
// T23 PresentTopLevelKeys_SameAliasTwice          — same alias used twice (no cycle) is not blocked by the guard
// T24 WriteTrackfwConfig_CyclicMergeKey           — E2E: cyclic trackfw.yaml does not trigger a duplicate key append

// T20 — presentTopLevelKeys on a self-referential mapping anchor does not overflow the
// stack and returns all keys reachable before the cycle.
//
// Reconciliation: this test asserts that the cycle guard in expandTopLevelKeys (ML-4E)
// breaks the d→d back-edge, preventing infinite recursion, and that explicit keys
// declared before the cycle (d, req_dir) are still detected as present.
func TestPresentTopLevelKeys_CyclicSelfRefMapping(t *testing.T) {
	// Fixture: d has explicit wip_limit and a self-referential merge; top level merges d in.
	// The top-level <<: *d is required to exercise the cycle in the writer path.
	content := "d: &d\n  wip_limit: \"3\"\n  <<: *d\n<<: *d\nreq_dir: docs/req\n"
	// Precondition: valid YAML (otherwise yaml.Unmarshal fails and expandTopLevelKeys is not called).
	keys, ok := presentTopLevelKeys(content)
	if !ok {
		t.Fatal("cyclic fixture must parse as valid YAML (ok=true)")
	}
	// Keys reachable before the cycle must be detected.
	if !keys["d"] {
		t.Error("T20: explicit key 'd' must be detected")
	}
	if !keys["wip_limit"] {
		t.Error("T20: wip_limit inherited via <<: *d must be detected (cycle guard must not block first visit)")
	}
	if !keys["req_dir"] {
		t.Error("T20: explicit key 'req_dir' must be detected")
	}
}

// T21 — presentTopLevelKeys on a self-referential sequence anchor (<<: *x where x=[*x])
// does not overflow the stack.
//
// Reconciliation: this test asserts that the cycle guard in expandMergeValue (ML-4E)
// breaks the sequence→sequence back-edge, preventing infinite recursion when the
// merge-value sequence contains an alias to itself.
func TestPresentTopLevelKeys_CyclicSequence(t *testing.T) {
	// x is a sequence containing an alias to itself; <<: *x at top level exercises
	// the SequenceNode path in expandMergeValue.
	content := "x: &x\n  - *x\n<<: *x\nreq_dir: docs/req\n"
	keys, ok := presentTopLevelKeys(content)
	if !ok {
		t.Fatal("cyclic sequence fixture must parse as valid YAML (ok=true)")
	}
	// The writer does not add sequence-element values as keys; it only enumerates
	// top-level key names. The important assertion is that the call returns at all.
	if !keys["x"] {
		t.Error("T21: explicit key 'x' must be detected")
	}
	if !keys["req_dir"] {
		t.Error("T21: explicit key 'req_dir' must be detected")
	}
}

// T22 — presentTopLevelKeys on a nested mutual cycle does not overflow the stack
// and returns the explicit keys.
//
// Reconciliation: this test asserts that the cycle guard in expandTopLevelKeys (ML-4E)
// handles the nested a→b→a cycle, terminating at the back-edge and returning the keys
// that were reachable on the first traversal of the cycle.
func TestPresentTopLevelKeys_CyclicMutualNested(t *testing.T) {
	// a: &a {b: &b {wip_limit: "3", <<: *a}, <<: *b} / <<: *a
	// &b is declared inside &a so there is no forward-anchor reference.
	content := "a: &a\n  b: &b\n    wip_limit: \"3\"\n    <<: *a\n  <<: *b\n<<: *a\nreq_dir: docs/req\n"
	keys, ok := presentTopLevelKeys(content)
	if !ok {
		t.Fatal("nested mutual cycle fixture must parse as valid YAML (ok=true)")
	}
	if !keys["a"] {
		t.Error("T22: explicit key 'a' must be detected")
	}
	if !keys["req_dir"] {
		t.Error("T22: explicit key 'req_dir' must be detected")
	}
}

// T23 — same alias used twice without a cycle is not blocked by the path-based guard.
//
// Reconciliation: this test asserts that the path-based cycle guard (ML-4E) does not
// over-block: when the same alias is used twice in <<: [*c, *c] without any back-edge,
// the guard removes *c from the visiting set after the first use, so the second use is
// processed normally. For key-set enumeration, this means keys["wip_limit"] is true.
// Note: a global (non-path) visited set would leave wip_limit absent after the first
// expansion and would still set it on the second (since we're only accumulating keys,
// not preventing re-reads). So the path/global distinction is observable here only via
// the reader test (TestCyclicAnchor_SameAliasTwiceNoCycle) where the actual value matters.
// This test confirms the writer also sees the key.
func TestPresentTopLevelKeys_SameAliasTwice(t *testing.T) {
	content := "c: &c\n  wip_limit: \"3\"\n<<: [*c, *c]\nreq_dir: docs/req\n"
	keys, ok := presentTopLevelKeys(content)
	if !ok {
		t.Fatal("double-alias fixture must parse as valid YAML (ok=true)")
	}
	if !keys["wip_limit"] {
		t.Error("T23: wip_limit inherited via <<: [*c, *c] must be detected as present")
	}
	if !keys["req_dir"] {
		t.Error("T23: req_dir must be detected")
	}
}

// T24 — end-to-end: writeTrackfwConfig on a cyclic trackfw.yaml does not append a
// duplicate wip_limit line, because presentTopLevelKeys detects wip_limit as already
// present via the cycle-guarded merge expansion.
//
// Reconciliation: this test asserts that the writer's key detector (ML-4E) handles a
// cyclic trackfw.yaml without crashing AND correctly identifies wip_limit as present —
// so writeTrackfwConfig performs a no-op for that key instead of appending
// "wip_limit: 1" (the template default) which would silently downgrade the user value.
func TestWriteTrackfwConfig_CyclicMergeKey(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "trackfw.yaml")

	// Fixture: wip_limit is set via a cyclic anchor and the top-level <<: *d.
	// All template keys except wip_limit are present explicitly.
	fixture := "d: &d\n  wip_limit: \"3\"\n  <<: *d\n<<: *d\nadr_dirs:\n  - docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\nhooks: none\nci: github-actions\nfrontend: \"\"\nbackend: go\nbackend_framework: \"\"\npkg_manager: \"\"\nwip_by_squad: false\nrequire_req_in_commit: false\nrules:\n  branch_has_wip_roadmap: error\n"
	if err := os.WriteFile(path, []byte(fixture), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := writeTrackfwConfig(Config{}); err != nil {
		t.Fatalf("writeTrackfwConfig on cyclic YAML: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(got)

	// No "wip_limit: 1" (template default) should have been appended.
	for _, line := range strings.Split(content, "\n") {
		if line == "wip_limit: 1" {
			t.Fatalf("T24: wip_limit: 1 was appended (cycle guard failed to detect wip_limit as present)\ncontent:\n%s", content)
		}
	}
}

// min é um auxiliar para evitar índice fora de intervalo em mensagens de erro.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- ML-4G: alias bomb tests (writers side) ---

// aliasBombSequenceWriter generates a YAML sequence-merge alias bomb with levels levels.
// Equivalent to the reader-side generator in config_merge_key_test.go; duplicated here
// because the generators package does not import the config test helpers.
func aliasBombSequenceWriter(levels int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "n00: &n00\n  wip_limit: 1\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&sb, "n%02d: &n%02d\n  <<: [*n%02d, *n%02d]\n", i, i, i-1, i-1)
	}
	fmt.Fprintf(&sb, "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n<<: [*n%02d, *n%02d]\n",
		levels-1, levels-1)
	return sb.String()
}

// TestAliasBomb_WriterFallback asserts:
// presentTopLevelKeys on a 30-level sequence alias bomb returns ok=false (fallback path)
// within 2 s. The walker's node-visit budget (ML-4H) fires, presentTopLevelKeys falls
// back to the text detector. req_dir and adr_dirs (at column 0) must still be detected.
func TestAliasBomb_WriterFallback(t *testing.T) {
	bomb := aliasBombSequenceWriter(30)
	type res struct {
		keys map[string]bool
		ok   bool
	}
	ch := make(chan res, 1)
	go func() {
		k, ok := presentTopLevelKeys(bomb)
		ch <- res{k, ok}
	}()
	select {
	case r := <-ch:
		if r.ok {
			t.Fatal("bomb must return ok=false (fallback triggered by ML-4H budget)")
		}
		// The fallback text detector scans lines at column 0; req_dir is at column 0.
		if !r.keys["req_dir"] {
			t.Error("fallback must detect req_dir (column-0 key in bomb fixture)")
		}
		if !r.keys["adr_dirs"] {
			t.Error("fallback must detect adr_dirs (column-0 key in bomb fixture)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("presentTopLevelKeys must handle alias bomb within 2 s (ML-4H budget)")
	}
}

// TestAliasBomb_WriterLegitimateAlias asserts:
// presentTopLevelKeys on a YAML where the same anchor is used 3 times (legitimate
// aliasing) returns ok=true (yaml.Node path, not fallback) and detects both the
// explicit keys and the key inherited via <<: *c. Legitimate use stays below the
// ML-4H budget (3 uses ≪ 10 000 visits).
func TestAliasBomb_WriterLegitimateAlias(t *testing.T) {
	content := "c: &c\n  wip_limit: \"5\"\nreq_dir: docs/req\nadr_dirs:\n  - docs/adr\n<<: *c\na: *c\nb: *c\n"
	keys, ok := presentTopLevelKeys(content)
	if !ok {
		t.Fatal("anchor used 3× must not be treated as bomb (ok must be true, yaml.Node path)")
	}
	if !keys["req_dir"] {
		t.Error("req_dir must be detected")
	}
	if !keys["wip_limit"] {
		t.Error("wip_limit inherited via <<: *c must be detected")
	}
}

// TestAliasBomb_WriterCounterArm asserts:
// The cycle guard alone does not stop a fan-out bomb: expandTopLevelKeys with an
// effectively unlimited budget (ML-4H limit field) on a 25-level sequence bomb
// takes >2 s. This demonstrates that the budget (ML-4H) is the control.
func TestAliasBomb_WriterCounterArm(t *testing.T) {
	// 25 levels: 2^25 ≈ 33 M yaml.Node walker steps — takes ~2.9 s without budget limit.
	bomb := aliasBombSequenceWriter(25)
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(bomb), &root); err != nil {
		t.Fatalf("bomb must parse as yaml.Node without error: %v", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		t.Fatal("bomb must have a top-level MappingNode")
	}
	top := root.Content[0]
	done := make(chan struct{})
	go func() {
		// Counter-arm: bypass the normal budget with an effectively unlimited limit.
		// This demonstrates that cycle guard alone does not stop a fan-out bomb —
		// only the budget (ML-4H) does.
		unlimitedCtx := &scaffoldCtx{
			visiting: make(map[*yaml.Node]bool),
			limit:    1 << 30, // ~1B visits, budget not the control here
		}
		expandTopLevelKeys(top, make(map[string]bool), unlimitedCtx)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("counter-arm FAIL: expandTopLevelKeys with unlimited budget should hang on 25-level bomb — " +
			"verify aliasBombSequenceWriter(25) generates an effective bomb on this machine")
	case <-time.After(2 * time.Second):
		// Expected: without budget limit, expandTopLevelKeys on a 25-level bomb takes >>2 s.
		// Goroutine terminates naturally after ~0.9 s more; test process continues. ML-4H.
	}
}

// --- ML-4H: writer budget tests ---

// cycleWriterThenBomb prepends a cyclic anchor to an alias bomb sequence (writer-side).
// This is the same desvio as the reader: interface{} gives "anchor value contains itself",
// not "excessive aliasing", so the ML-4G first-pass would have missed it.
func cycleWriterThenBomb(levels int) string {
	cycle := "cycle: &cycle\n  req_dir: docs/req\n  <<: *cycle\n"
	return cycle + aliasBombSequenceWriter(levels)
}

// dupMergeKeyWriterThenBomb creates a YAML with a small anchor merged first, followed
// by the bomb as a second merge key. See the reader-side equivalent for the desvio
// explanation.
func dupMergeKeyWriterThenBomb(levels int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "small: &small\n  req_dir: docs/req\n")
	fmt.Fprintf(&sb, "n00: &n00\n  wip_limit: 1\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&sb, "n%02d: &n%02d\n  <<: [*n%02d, *n%02d]\n", i, i, i-1, i-1)
	}
	fmt.Fprintf(&sb, "adr_dirs:\n  - docs/adr\n<<: *small\n<<: [*n%02d, *n%02d]\n",
		levels-1, levels-1)
	return sb.String()
}

// TestAliasBomb_WriterBudget_CycleBeforeBomb asserts:
// presentTopLevelKeys on a YAML with a cyclic anchor before an alias bomb returns
// ok=false (fallback) within 2 s. The ML-4H walker budget fires. The fallback text
// detector finds the column-0 keys.
func TestAliasBomb_WriterBudget_CycleBeforeBomb(t *testing.T) {
	fixture := cycleWriterThenBomb(25)
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(fixture), &probe); err != nil {
		t.Fatalf("precondition: yaml.Node must succeed: %v", err)
	}
	type res struct {
		keys map[string]bool
		ok   bool
	}
	ch := make(chan res, 1)
	go func() {
		k, ok := presentTopLevelKeys(fixture)
		ch <- res{k, ok}
	}()
	select {
	case r := <-ch:
		if r.ok {
			t.Fatal("cycle-before-bomb must return ok=false (ML-4H budget or yaml.Node error)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("presentTopLevelKeys must handle cycle-before-bomb within 2 s (ML-4H budget)")
	}
}

// TestAliasBomb_WriterBudget_DupMergeKeyBeforeBomb asserts:
// presentTopLevelKeys on a YAML with a duplicate <<: before an alias bomb returns
// ok=false (fallback) within 2 s. ML-4H budget fires regardless of interface{} error.
func TestAliasBomb_WriterBudget_DupMergeKeyBeforeBomb(t *testing.T) {
	fixture := dupMergeKeyWriterThenBomb(25)
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(fixture), &probe); err != nil {
		t.Fatalf("precondition: yaml.Node must succeed: %v", err)
	}
	type res struct {
		keys map[string]bool
		ok   bool
	}
	ch := make(chan res, 1)
	go func() {
		k, ok := presentTopLevelKeys(fixture)
		ch <- res{k, ok}
	}()
	select {
	case r := <-ch:
		if r.ok {
			t.Fatal("dup-merge-key-before-bomb must return ok=false (ML-4H budget or yaml.Node error)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("presentTopLevelKeys must handle dup-merge-key-before-bomb within 2 s (ML-4H budget)")
	}
}
