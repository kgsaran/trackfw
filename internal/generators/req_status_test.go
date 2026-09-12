package generators

// Tests for ML-1A (AC9): parseREQMeta deve ler status do frontmatter primeiro.
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara em comentário qual conclusão
// do ML-1A ele afirma.

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseREQMeta_FrontmatterFirst — afirma que parseREQMeta retorna o status do frontmatter
// quando presente, ignorando o que a linha de cabeçalho diz.
// Conclusão afirmada: ML-1A — frontmatter `status:` é a fonte de verdade para `req list`;
// a linha `| Status: WIP` no corpo é ignorada quando o frontmatter declara "Done" (issue #306).
func TestParseREQMeta_FrontmatterFirst(t *testing.T) {
	dir := t.TempDir()
	// Frontmatter diz Done, corpo diz WIP — deve retornar Done
	content := "---\nstatus: Done\ndate: 2026-09-12\nroadmap: \"\"\n---\n\n# REQ: Fixture\n\n> Date: 2026-09-12 | Status: WIP\n"
	path := filepath.Join(dir, "REQ-test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, status := parseREQMeta(path)
	if status != "Done" {
		t.Errorf("parseREQMeta deve retornar status do frontmatter (Done) quando presente, obteve %q", status)
	}
}

// TestParseREQMeta_BodyFallback — afirma que parseREQMeta cai para o corpo quando o frontmatter
// não tem `status:` (legados sem frontmatter ainda funcionam).
// Conclusão afirmada: ML-1A — body `| Status:` é fallback válido para REQs legadas sem frontmatter.
func TestParseREQMeta_BodyFallback(t *testing.T) {
	dir := t.TempDir()
	// Sem frontmatter: apenas linha de cabeçalho no corpo
	content := "# REQ: Sem Frontmatter\n\n> Date: 2026-09-12 | Status: WIP\n"
	path := filepath.Join(dir, "REQ-legacy.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, status := parseREQMeta(path)
	if status != "WIP" {
		t.Errorf("parseREQMeta deve cair para o corpo quando frontmatter ausente, obteve %q", status)
	}
}

// TestParseREQMeta_FrontmatterEmptyBodyPresent — afirma que quando o frontmatter tem `status: ""`
// (vazio), o corpo é usado como fallback (frontmatter vazio != frontmatter preenchido).
// Conclusão afirmada: ML-1A — frontmatter empty não suprime o fallback do corpo.
func TestParseREQMeta_FrontmatterEmptyBodyPresent(t *testing.T) {
	dir := t.TempDir()
	// Frontmatter com status vazio, corpo diz WIP
	content := "---\nstatus: \"\"\ndate: 2026-09-12\nroadmap: \"\"\n---\n\n# REQ: Fixture\n\n> Date: 2026-09-12 | Status: WIP\n"
	path := filepath.Join(dir, "REQ-empty-fm.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, status := parseREQMeta(path)
	if status != "WIP" {
		t.Errorf("frontmatter com status vazio deve cair para o corpo, obteve %q", status)
	}
}
