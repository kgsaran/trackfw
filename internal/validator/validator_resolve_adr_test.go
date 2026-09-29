package validator

import (
	"os"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// writeADRFixtureFile escreve um arquivo .md mínimo num caminho relativo ao dir.
func writeADRFixtureFile(t *testing.T, base, rel string) {
	t.Helper()
	path := base + "/" + rel
	if err := os.MkdirAll(pathDir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", pathDir(path), err)
	}
	if err := os.WriteFile(path, []byte("---\nstatus: Accepted\n---\n# ADR: teste\n"), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// pathDir retorna o diretório de um caminho (equivalente a filepath.Dir mas sem import extra).
func pathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

// TestResolveADRFiles_LayoutPlano afirma que num layout plano (apenas raiz, sem subpastas),
// ResolveADRFiles devolve os mesmos arquivos que walkADRFilePaths — comportamento legado preservado.
func TestResolveADRFiles_LayoutPlano(t *testing.T) {
	// Reconciliação: este teste mede que a introdução do resolvedor não quebra o layout plano
	// (único layout usado neste repositório), garantido pela Wave 0: WalkDir num dir sem subpastas
	// é idêntico a ReadDir.
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte(""), 0644); err != nil {
		t.Fatalf("trackfw.yaml: %v", err)
	}
	if err := os.MkdirAll("docs/adr", 0755); err != nil {
		t.Fatalf("mkdir docs/adr: %v", err)
	}
	writeADRFixtureFile(t, dir, "docs/adr/ADR-2026-01-01-foo.md")
	writeADRFixtureFile(t, dir, "docs/adr/ADR-2026-01-02-bar.md")

	cfg := config.Load()
	got := ResolveADRFiles(cfg)
	if len(got) != 2 {
		t.Errorf("layout plano: esperado 2 ADRs, obteve %d: %v", len(got), got)
	}
}

// TestResolveADRFiles_Subpastas afirma que ADRs em subpastas de estado (done/, wip/) são
// enumerados quando adr_dirs aponta o diretório pai.
func TestResolveADRFiles_Subpastas(t *testing.T) {
	// Reconciliação: este teste mede a correção do defeito S1 (ADR-2026-09-29): com ADRs
	// exclusivamente em subpastas, o enumerador antigo retornava 0; o resolvedor deve retornar 4.
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yaml := "adr_dirs:\n  - docs/adr/zeus\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("trackfw.yaml: %v", err)
	}
	writeADRFixtureFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-01-a.md")
	writeADRFixtureFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-02-b.md")
	writeADRFixtureFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-03-c.md")
	writeADRFixtureFile(t, dir, "docs/adr/zeus/wip/ADR-2026-09-04-d.md")

	cfg := config.Load()
	got := ResolveADRFiles(cfg)
	if len(got) != 4 {
		t.Errorf("layout subpastas: esperado 4 ADRs, obteve %d: %v", len(got), got)
	}
}

// TestResolveADRFiles_DedupAninhado afirma que com adr_dirs aninhados (pai + subpasta),
// cada arquivo é contado exatamente uma vez — sem double-count.
func TestResolveADRFiles_DedupAninhado(t *testing.T) {
	// Reconciliação: este teste mede o defeito D4 (ADR-2026-09-29): com adr_dirs:
	// [docs/adr/zeus, docs/adr/zeus/done], 4 ADRs reais eram reportados como 7 pelo status.
	// O resolvedor com dedup por caminho absoluto deve reportar exatamente 4.
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// 3 ADRs em done/, 1 em wip/
	writeADRFixtureFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-01-a.md")
	writeADRFixtureFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-02-b.md")
	writeADRFixtureFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-03-c.md")
	writeADRFixtureFile(t, dir, "docs/adr/zeus/wip/ADR-2026-09-04-d.md")

	yaml := "adr_dirs:\n  - docs/adr/zeus\n  - docs/adr/zeus/done\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("trackfw.yaml: %v", err)
	}

	cfg := config.Load()
	got := ResolveADRFiles(cfg)
	if len(got) != 4 {
		t.Errorf("adr_dirs aninhados: esperado 4 ADRs (com dedup), obteve %d: %v", len(got), got)
	}
}

// TestResolveADRFiles_DirsDistintosNaoAninhados afirma que dois arquivos com mesmo basename em
// diretórios distintos e não-aninhados são contados como duas entidades distintas.
func TestResolveADRFiles_DirsDistintosNaoAninhados(t *testing.T) {
	// Reconciliação: este teste falsifica a direção de supressão do dedup — garantindo que a
	// dedup por caminho absoluto não colapsa arquivos em árvores separadas.
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	writeADRFixtureFile(t, dir, "docs/adr/zeus/ADR-2026-09-01-foo.md")
	writeADRFixtureFile(t, dir, "docs/adr/athena/ADR-2026-09-01-foo.md")

	yaml := "adr_dirs:\n  - docs/adr/zeus\n  - docs/adr/athena\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("trackfw.yaml: %v", err)
	}

	cfg := config.Load()
	got := ResolveADRFiles(cfg)
	if len(got) != 2 {
		t.Errorf("dirs distintos não-aninhados: esperado 2 ADRs, obteve %d: %v", len(got), got)
	}
}

// TestResolveADRFiles_HasSuffixSemPrefixo afirma que o critério de identificação é
// strings.HasSuffix(".md") sem filtro de prefixo "ADR-" — comportamento declarado na ADR-2026-09-29
// seção Consequências, resíduo aceito documentado.
//
// Reconciliação: este teste fixa que um arquivo NOTAS.md em adr_dirs É contado — não por engano,
// mas porque este é o comportamento existente de walkADRFilePaths (validator.go:3017) e a ADR
// decidiu explicitamente não mudar o critério de identificação nesta REQ. Mudar este comportamento
// requer REQ própria com medição.
func TestResolveADRFiles_HasSuffixSemPrefixo(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte(""), 0644); err != nil {
		t.Fatalf("trackfw.yaml: %v", err)
	}
	if err := os.MkdirAll("docs/adr", 0755); err != nil {
		t.Fatalf("mkdir docs/adr: %v", err)
	}
	writeADRFixtureFile(t, dir, "docs/adr/ADR-2026-09-01-real.md")
	writeADRFixtureFile(t, dir, "docs/adr/NOTAS.md") // não é ADR, mas DEVE ser contado (resíduo aceito)

	cfg := config.Load()
	got := ResolveADRFiles(cfg)
	if len(got) != 2 {
		t.Errorf("HasSuffix sem prefixo: esperado 2 (incluindo NOTAS.md), obteve %d: %v", len(got), got)
	}
}
