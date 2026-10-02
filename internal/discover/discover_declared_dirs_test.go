package discover

import (
	"os"
	"path/filepath"
	"testing"
)

// Issue #471: `discover` sondava caminhos convencionais e nunca lia o `trackfw.yaml` que ele
// mesmo acabara de contar em HasTrackfwYAML — então o Governance Score descrevia outro projeto.
//
// Cada teste declara, na primeira linha, qual conclusão da medição ele afirma.

// escreveProjeto monta um projeto temporário com o conteúdo de trackfw.yaml dado (vazio = sem
// arquivo) e os diretórios/arquivos pedidos.
func escreveProjeto(t *testing.T, yaml string, arquivos map[string]string) string {
	t.Helper()
	raiz := t.TempDir()
	if yaml != "" {
		if err := os.WriteFile(filepath.Join(raiz, "trackfw.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatalf("trackfw.yaml: %v", err)
		}
	}
	for rel, conteudo := range arquivos {
		full := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(conteudo), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return raiz
}

const adrReal = "---\nstatus: Accepted\n---\n\n# ADR-001\n\n## Decisao\nSim.\n"

// Afirma o achado central: com `adr_dirs` apontando para fora de `docs/adr`, a contagem era ZERO
// e o Governance Score perdia os 20 pontos da categoria. Medido por efeito antes da correção:
// `status`/`context`/`adr list` viam o ADR e `discover` reportava 20/100 em vez de 40/100.
func TestDetect_ADRDirsDeclaradoForaDoDefault(t *testing.T) {
	raiz := escreveProjeto(t, "adr_dirs:\n  - docs/decisoes\n", map[string]string{
		"docs/decisoes/ADR-001-real.md": adrReal,
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.ADRCount != 1 {
		t.Errorf("ADRCount = %d, esperava 1 (o ADR está em docs/decisoes, declarado)", r.ADRCount)
	}
	if len(r.ADRDirs) != 1 || r.ADRDirs[0] != "docs/decisoes" {
		t.Errorf("ADRDirs = %v, esperava [docs/decisoes]", r.ADRDirs)
	}
}

// Afirma o braço contrário: projeto SEM trackfw.yaml continua detectado pela sonda de disco —
// é o caso de uso do comando (`Scan the repository and auto-detect`), e a correção não pode
// tê-lo quebrado.
func TestDetect_SemYamlMantemASonda(t *testing.T) {
	raiz := escreveProjeto(t, "", map[string]string{
		"docs/adr/ADR-001-real.md":   adrReal,
		"docs/requirements/REQ-1.md": "# req\n",
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.ADRCount != 1 {
		t.Errorf("ADRCount = %d, esperava 1 pela sonda de docs/adr", r.ADRCount)
	}
	if r.REQDir != "docs/requirements" {
		t.Errorf("REQDir = %q, esperava docs/requirements pela sonda de candidatos", r.REQDir)
	}
}

// Afirma a condição "só sobrescreve quando o declarado EXISTE". ParseDirsFromContent devolve
// defaults() para chave ausente, então sem essa condição um projeto com `rules:` e nada mais
// perderia a sonda de 4 candidatos do REQ — regressão silenciosa.
func TestDetect_YamlSemAsChavesMantemASonda(t *testing.T) {
	raiz := escreveProjeto(t, "rules:\n  req_has_adr: error\n", map[string]string{
		"docs/requisições/REQ-1.md": "# req\n",
		"docs/adr/ADR-001-real.md":  adrReal,
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.REQDir != "docs/requisições" {
		t.Errorf("REQDir = %q, esperava docs/requisições pela sonda", r.REQDir)
	}
	if r.ADRCount != 1 {
		t.Errorf("ADRCount = %d, esperava 1 pela sonda", r.ADRCount)
	}
}

// Afirma que o declarado inexistente NÃO cega a sonda: `adr_dirs` apontando para pasta que não
// existe cai no caminho convencional em vez de reportar zero. Controle de remédio excessivo.
func TestDetect_DeclaradoInexistenteCaiNaSonda(t *testing.T) {
	raiz := escreveProjeto(t, "adr_dirs:\n  - docs/nao-existe\nreq_dir: docs/tambem-nao\n", map[string]string{
		"docs/adr/ADR-001-real.md": adrReal,
		"docs/req/REQ-1.md":        "# req\n",
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.ADRCount != 1 {
		t.Errorf("ADRCount = %d, esperava 1 — o declarado não existe, a sonda deve valer", r.ADRCount)
	}
	if r.REQDir != "docs/req" {
		t.Errorf("REQDir = %q, esperava docs/req pela sonda", r.REQDir)
	}
}

// Afirma que a contagem do caminho declarado passa pelo PONTO ÚNICO (validator.ResolveADRFiles,
// D3 da ADR-2026-09-29) e não por uma varredura própria: a dedução é que o critério de
// identificação que a #471 escolher vale aqui sem segunda edição. O observável disponível hoje é
// a DEDUPLICAÇÃO por caminho absoluto, que o countMDFiles local não faz — `adr_dirs` com a mesma
// pasta duas vezes conta 1, não 2.
func TestDetect_ADRDirsDuplicadoContaUmaVez(t *testing.T) {
	raiz := escreveProjeto(t, "adr_dirs:\n  - docs/decisoes\n  - docs/decisoes\n", map[string]string{
		"docs/decisoes/ADR-001-real.md": adrReal,
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.ADRCount != 1 {
		t.Errorf("ADRCount = %d, esperava 1 — ResolveADRFiles deduplica por caminho absoluto", r.ADRCount)
	}
}

// Afirma o terceiro sítio da mesma causa: `roadmap_dir` declarado fora de `docs/roadmaps` era
// ignorado, e o RoadmapCount saía zero junto com os 20 pontos da categoria.
func TestDetect_RoadmapDirDeclaradoForaDoDefault(t *testing.T) {
	raiz := escreveProjeto(t, "roadmap_dir: docs/planos\n", map[string]string{
		"docs/planos/wip/ROADMAP-1.md":  "# r\n",
		"docs/planos/done/ROADMAP-2.md": "# r\n",
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.RoadmapDir != "docs/planos" {
		t.Errorf("RoadmapDir = %q, esperava docs/planos", r.RoadmapDir)
	}
	if r.RoadmapCount != 2 {
		t.Errorf("RoadmapCount = %d, esperava 2", r.RoadmapCount)
	}
	if r.RoadmapNamespacing != "flat" {
		t.Errorf("RoadmapNamespacing = %q, esperava flat", r.RoadmapNamespacing)
	}
}

// Afirma que a detecção de by_agent continua vindo do DISCO mesmo com roadmap_dir declarado —
// o trackfw.yaml declara o diretório, não o namespacing por pasta, e a correção não move essa
// responsabilidade.
func TestDetect_RoadmapDirDeclaradoAindaDetectaByAgent(t *testing.T) {
	raiz := escreveProjeto(t, "roadmap_dir: docs/planos\n", map[string]string{
		"docs/planos/claude/wip/ROADMAP-1.md":  "# r\n",
		"docs/planos/claude/done/ROADMAP-2.md": "# r\n",
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.RoadmapNamespacing != "by_agent" {
		t.Errorf("RoadmapNamespacing = %q, esperava by_agent", r.RoadmapNamespacing)
	}
	if r.RoadmapCount != 2 {
		t.Errorf("RoadmapCount = %d, esperava 2", r.RoadmapCount)
	}
	if len(r.Agents) != 1 || r.Agents[0] != "claude" {
		t.Errorf("Agents = %v, esperava [claude]", r.Agents)
	}
}

// Afirma que o Governance Score — o número que a #471 nomeia como consequência — muda de fato:
// 20 pontos da categoria ADR deixavam de ser creditados a um projeto que TEM ADR.
func TestCalcScore_ADRDeclaradoCreditaACategoria(t *testing.T) {
	raiz := escreveProjeto(t, "adr_dirs:\n  - docs/decisoes\n", map[string]string{
		"docs/decisoes/ADR-001-real.md": adrReal,
	})

	r, err := Scan(raiz)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// trackfw.yaml existe (+20) e a categoria ADR agora conta (+20).
	if r.GovernanceScore < 40 {
		t.Errorf("GovernanceScore = %d, esperava >= 40 (yaml + categoria ADR)", r.GovernanceScore)
	}
	if r.ADRCount == 0 {
		t.Error("ADRCount zero — a categoria não pode ser creditada por acidente")
	}
}
