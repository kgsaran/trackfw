package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ML-3D (ROADMAP-2026-09-09-req-nasce-orfa-…): os ramos de generateGitIgnore.
// O ML-3A criou <roadmap_dir>/.trackfw-branch-links.json — estado POR CHECKOUT — e
// o ignorou apenas no .gitignore DESTE repositório. No consumidor o arquivo nascia
// rastreado, e o vínculo de uma máquina passava a governar outra. Estes testes
// afirmam que `trackfw init` fecha isso sem tocar no .gitignore do projeto.
//
// chdirTemp vem de gitattributes_test.go (mesmo pacote).

// Reconciliação: afirma o AC1 (o `init` acrescenta a linha, com o roadmap_dir
// default quando não há trackfw.yaml) e o contra-braço 2 (rodar duas vezes não
// duplica a linha).
func TestGenerateGitIgnore_CriaQuandoAusenteEIdempotente(t *testing.T) {
	dir := chdirTemp(t)
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ler .gitignore: %v", err)
	}
	if !strings.Contains(string(first), "docs/roadmaps/.trackfw-branch-links.json\n") {
		t.Fatalf("linha de ignore ausente no arquivo criado:\n%q", string(first))
	}
	// Segunda execução: no-op, byte a byte.
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore (2ª): %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if string(second) != string(first) {
		t.Fatalf("init duas vezes duplicou/alterou o arquivo:\n%q", string(second))
	}
	if n := strings.Count(string(second), ".trackfw-branch-links.json"); n != 1 {
		t.Fatalf("esperava 1 menção ao arquivo de vínculo, achei %d:\n%q", n, string(second))
	}
}

// Reconciliação: afirma que o caminho emitido vem do `roadmap_dir` EFETIVO do
// trackfw.yaml do projeto, e não de `docs/roadmaps` fixo — é o único teste que
// falsifica o hardcode (família de defeito do #396). Sem trackfw.yaml não-default,
// o teste passaria idêntico nas duas implementações.
func TestGenerateGitIgnore_UsaRoadmapDirEfetivoNaoODoMantenedor(t *testing.T) {
	dir := chdirTemp(t)
	cfg := "req_dir: governance/reqs\nroadmap_dir: governance/plans\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatalf("preparar trackfw.yaml: %v", err)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ler .gitignore: %v", err)
	}
	if !strings.Contains(string(got), "governance/plans/.trackfw-branch-links.json\n") {
		t.Fatalf("não usou o roadmap_dir do projeto:\n%q", string(got))
	}
	if strings.Contains(string(got), "docs/roadmaps") {
		t.Fatalf("emitiu o layout do mantenedor num projeto com roadmap_dir próprio:\n%q", string(got))
	}
}

// Reconciliação: afirma o contra-braço 1 — o .gitignore preexistente do projeto
// não é sobrescrito, e o append não gruda o bloco na última linha quando o arquivo
// não termina em newline (que corromperia o padrão do projeto em silêncio).
func TestGenerateGitIgnore_AppendPreservaArquivoPreexistenteSemNewlineFinal(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("/node_modules\n*.log"), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "/node_modules\n*.log\n" + gitIgnoreBlock("docs/roadmaps")
	if string(got) != want {
		t.Fatalf("append incorreto:\ngot:  %q\nwant: %q", string(got), want)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore (2ª): %v", err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != want {
		t.Fatalf("segunda execução duplicou a regra:\n%q", string(again))
	}
}

// Reconciliação: afirma que o reconhecimento é por BASENAME, não pelo caminho
// literal emitido — um projeto que já ignora o arquivo em outro caminho (ou que
// mudou de roadmap_dir depois) não recebe uma segunda linha para o mesmo arquivo.
func TestGenerateGitIgnore_NaoDuplicaQuandoOutroCaminhoJaNomeiaOArquivo(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitignore")
	existing := "# ignorado à mão pelo projeto\nlegacy/plans/.trackfw-branch-links.json\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != existing {
		t.Fatalf("regra preexistente foi alterada ou duplicada:\ngot:  %q\nwant: %q", string(got), existing)
	}
}
