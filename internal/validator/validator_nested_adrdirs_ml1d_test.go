package validator

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestADROrphanNaoduplicaComADRDirsAninhadas afirma que com adr_dirs aninhados
// (ex: [docs/adr/zeus, docs/adr/zeus/done]), N ADRs reais produzem exatamente N warnings
// adr_orphan — não 2N — porque validateADRsAreReferenced agora usa ResolveADRFiles (dedup).
//
// Reconciliação: mede diretamente a redução de 6→3 warnings que o achado do ML-1D documentou
// sobre a fixture canônica (3 ADRs em zeus/done com adr_dirs listando zeus E zeus/done).
func TestADROrphanNaoDuplicaComADRDirsAninhadas(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr/zeus/done", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	yaml := "adr_dirs:\n  - docs/adr/zeus\n  - docs/adr/zeus/done\n"
	writeFile(t, dir, "trackfw.yaml", yaml)

	// 3 ADRs em zeus/done — com adr_dirs aninhados, o loop antigo produzia 6 warnings
	writeFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-01-t.md",
		"---\nstatus: Accepted\n---\n# ADR 1\n")
	writeFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-02-t.md",
		"---\nstatus: Accepted\n---\n# ADR 2\n")
	writeFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-03-t.md",
		"---\nstatus: Accepted\n---\n# ADR 3\n")

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}

	orphanWarnings := filterOrphanWarnings(warnings)
	if len(orphanWarnings) != 3 {
		t.Errorf("adr_dirs aninhados: esperado 3 warnings de adr_orphan, obteve %d: %v",
			len(orphanWarnings), orphanWarnings)
	}
}

// TestADROrphanPreservaBasenameIgualEmDirsDistintos afirma que dois ADRs com mesmo basename em
// diretórios distintos e NÃO aninhados produzem exatamente 2 warnings — o dedup por caminho
// absoluto não colapsa arquivos em árvores separadas.
//
// Reconciliação: mede a direção de supressão — um dedup por basename suprimiria 1 dos 2 avisos
// legítimos; o teste garante que isso NÃO acontece.
func TestADROrphanPreservaBasenameIgualEmDirsDistintos(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr/zeus", "docs/adr/athena", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	yaml := "adr_dirs:\n  - docs/adr/zeus\n  - docs/adr/athena\n"
	writeFile(t, dir, "trackfw.yaml", yaml)

	// Mesmo basename em duas árvores distintas e não-aninhadas
	writeFile(t, dir, "docs/adr/zeus/ADR-001-t.md",
		"---\nstatus: Accepted\n---\n# ADR Zeus\n")
	writeFile(t, dir, "docs/adr/athena/ADR-001-t.md",
		"---\nstatus: Accepted\n---\n# ADR Athena\n")

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}

	orphanWarnings := filterOrphanWarnings(warnings)
	if len(orphanWarnings) != 2 {
		t.Errorf("dirs distintos não-aninhados: esperado 2 warnings de adr_orphan, obteve %d: %v",
			len(orphanWarnings), orphanWarnings)
	}
}

// TestFrontmatterPresenceNaoDuplicaComADRDirsAninhadas afirma que com adr_dirs aninhados,
// N ADRs sem frontmatter produzem exatamente N violations — não 2N — porque
// validateFrontmatterPresence agora usa ResolveADRFiles (dedup).
//
// Reconciliação: mede o mesmo mecanismo de dedup que TestADROrphanNaoDuplicaComADRDirsAninhadas,
// mas para a regra frontmatter_presence — confirma que ambas as regras foram corrigidas.
func TestFrontmatterPresenceNaoDuplicaComADRDirsAninhadas(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr/zeus/done", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	yaml := "adr_dirs:\n  - docs/adr/zeus\n  - docs/adr/zeus/done\n"
	writeFile(t, dir, "trackfw.yaml", yaml)

	// 2 ADRs SEM frontmatter em zeus/done
	writeFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-01-nofm.md",
		"# ADR sem frontmatter\n\nConteúdo.\n")
	writeFile(t, dir, "docs/adr/zeus/done/ADR-2026-09-02-nofm.md",
		"# Outro ADR sem frontmatter\n\nConteúdo.\n")

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}

	fmViolations := filterFrontmatterViolations(violations)
	if len(fmViolations) != 2 {
		t.Errorf("frontmatter_presence aninhado: esperado 2 violations, obteve %d: %v",
			len(fmViolations), fmViolations)
	}
}

// filterOrphanWarnings filtra os warnings de adr_orphan da lista geral.
func filterOrphanWarnings(warnings []string) []string {
	var out []string
	for _, w := range warnings {
		if strings.Contains(w, "is not referenced by any REQ") {
			out = append(out, w)
		}
	}
	return out
}

// filterFrontmatterViolations filtra as violations de frontmatter_presence da lista geral.
func filterFrontmatterViolations(violations []string) []string {
	var out []string
	for _, v := range violations {
		if strings.Contains(v, "has no frontmatter block") {
			out = append(out, v)
		}
	}
	return out
}
