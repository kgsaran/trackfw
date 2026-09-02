package generators

import (
	"os"
	"path/filepath"
	"testing"
)

// ML-1A (ROADMAP-2026-09-02-gitattributes-com-merge-union-para-o-trackfw-log-nos-3-clis):
// os três ramos de generateGitAttributes. O gate scripts/check-artifact-parity.sh
// cobre só o ramo de CRIAÇÃO cross-runtime; o de APPEND (projeto que já tem
// .gitattributes) só existe aqui e nos equivalentes npm/pypi.

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

func TestGenerateGitAttributes_CriaQuandoAusenteEIdempotente(t *testing.T) {
	dir := chdirTemp(t)
	if err := generateGitAttributes(); err != nil {
		t.Fatalf("generateGitAttributes: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil {
		t.Fatalf("ler .gitattributes: %v", err)
	}
	if string(first) != gitAttributesBlock {
		t.Fatalf("conteúdo criado diverge do bloco canônico:\n%q", string(first))
	}
	// Segunda execução: no-op, byte a byte.
	if err := generateGitAttributes(); err != nil {
		t.Fatalf("generateGitAttributes (2ª): %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if string(second) != string(first) {
		t.Fatalf("init duas vezes duplicou/alterou o arquivo:\n%q", string(second))
	}
}

func TestGenerateGitAttributes_AppendPreservaArquivoPreexistenteSemNewlineFinal(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitattributes")
	// Sem newline final de propósito: emendar direto grudaria a primeira linha do
	// bloco na última linha do projeto.
	if err := os.WriteFile(path, []byte("* text=auto"), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitAttributes(); err != nil {
		t.Fatalf("generateGitAttributes: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "* text=auto\n" + gitAttributesBlock
	if string(got) != want {
		t.Fatalf("append incorreto:\ngot:  %q\nwant: %q", string(got), want)
	}
	if err := generateGitAttributes(); err != nil {
		t.Fatalf("generateGitAttributes (2ª): %v", err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != want {
		t.Fatalf("segunda execução duplicou a regra:\n%q", string(again))
	}
}

func TestGenerateGitAttributes_NaoSobrescreveRegraPreexistenteComOutroEspacamento(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitattributes")
	existing := ".trackfw-log  merge=union\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitAttributes(); err != nil {
		t.Fatalf("generateGitAttributes: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != existing {
		t.Fatalf("regra preexistente foi alterada:\n%q", string(got))
	}
}

func TestGenerateGitAttributes_ComentarioNaoContaComoRegra(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(path, []byte("# .trackfw-log merge=union\n"), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitAttributes(); err != nil {
		t.Fatalf("generateGitAttributes: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "# .trackfw-log merge=union\n" + gitAttributesBlock
	if string(got) != want {
		t.Fatalf("comentário foi tratado como regra ativa:\n%q", string(got))
	}
}

func TestGitAttributesBlock_IgualAoArquivoVersionadoNaRaiz(t *testing.T) {
	// Mesmo precedente do teste do slash command roadmap.md: o arquivo deste
	// repositório e o que o `init` gera não podem divergir.
	versioned, err := os.ReadFile(filepath.Join("..", "..", ".gitattributes"))
	if err != nil {
		t.Fatalf("ler .gitattributes versionado: %v", err)
	}
	if string(versioned) != gitAttributesBlock {
		t.Fatalf(".gitattributes da raiz diverge do bloco gerado pelo init:\ngot:  %q\nwant: %q", string(versioned), gitAttributesBlock)
	}
}
