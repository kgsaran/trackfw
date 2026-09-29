package generators

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// captureGetContext redireciona os.Stdout para um arquivo temporário, chama GetContext(format)
// e retorna a saída capturada. Não usa os.Pipe (GetContext chama validator.Validate(), que pode
// bloquear uma pipe não consumida em paralelo).
func captureGetContext(t *testing.T, format string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "context-out-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer tmpFile.Close()

	origStdout := os.Stdout
	os.Stdout = tmpFile
	defer func() { os.Stdout = origStdout }()

	if err := GetContext(format); err != nil {
		t.Fatalf("GetContext: %v", err)
	}

	if err := tmpFile.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tmpFile); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	return buf.String()
}

// fixtureADRSubpastas cria um diretório temporário com adr_dirs: [docs/adr/zeus] e
// 4 ADRs em subpastas de estado (3 em done/, 1 em wip/). Sem REQs nem roadmaps.
// Retorna o caminho do dir de fixture.
func fixtureADRSubpastas(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeContextFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr/zeus\n")
	for _, rel := range []string{
		"docs/adr/zeus/done/ADR-2026-09-01-a.md",
		"docs/adr/zeus/done/ADR-2026-09-02-b.md",
		"docs/adr/zeus/done/ADR-2026-09-03-c.md",
		"docs/adr/zeus/wip/ADR-2026-09-04-d.md",
	} {
		writeContextFile(t, dir, rel, "---\nstatus: Accepted\n---\n# ADR: teste\n")
	}
	return dir
}

// TestGetContext_ADRSubpastas_NaoZero afirma que com ADRs exclusivamente em subpastas de estado,
// GetContext não imprime "## ADRs (0)".
//
// Reconciliação: este teste mede o sintoma original do #450 (T1 da Wave 0): o mesmo comando
// declarava zero ADRs e nomeava ADRs nos warnings simultaneamente. Após o fix, a contagem
// deve ser > 0 quando existem ADRs no disco.
func TestGetContext_ADRSubpastas_NaoZero(t *testing.T) {
	dir := fixtureADRSubpastas(t)
	chdirContext(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	out := captureGetContext(t, "md")

	if strings.Contains(out, "## ADRs (0)") {
		t.Errorf("GetContext imprimiu '## ADRs (0)' com ADRs em subpastas; saída:\n%s", out)
	}
	if !strings.Contains(out, "## ADRs (4)") {
		t.Errorf("GetContext esperado '## ADRs (4)', não encontrado; saída:\n%s", out)
	}
}

// TestGetContext_ADRSubpastas_ScoreDelta20 afirma que o score com ADRs em subpastas é exatamente
// 20 pontos maior do que seria sem ADRs — confirmando que apenas a categoria ADR muda.
//
// Reconciliação: este teste mede T2 da Wave 0 (score deflacionado): a fixture tem ADRs mas nenhuma
// REQ e nenhum roadmap; o score deve ser 60/100 (40 clean + 20 ADRs), não 40/100 como antes do fix.
// Delta = 20 pontos, exatamente a categoria ADR.
func TestGetContext_ADRSubpastas_ScoreDelta20(t *testing.T) {
	dir := fixtureADRSubpastas(t)
	chdirContext(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	out := captureGetContext(t, "md")

	// Score esperado: +40 (clean) + 20 (ADRs) + 0 (REQs) + 0 (roadmaps) = 60/100
	if !strings.Contains(out, "**Governance score:** 60/100") {
		t.Errorf("score esperado 60/100, saída:\n%s", out)
	}
}

// TestGetContext_ADRSubpastas_SemContradição afirma que a saída não contém simultaneamente
// "## ADRs (0)" e um warning nomeando um arquivo ADR — contradição interna que era o sintoma
// original do issue #450.
//
// Reconciliação: este teste mede a contradição literal da mesma execução ("declara zero ADRs e
// nomeia um ADR nos warnings"). Após o fix, a contradição deve ser impossível por construção.
func TestGetContext_ADRSubpastas_SemContradicao(t *testing.T) {
	dir := fixtureADRSubpastas(t)
	chdirContext(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	out := captureGetContext(t, "md")

	hasZero := strings.Contains(out, "## ADRs (0)")
	hasADRWarning := strings.Contains(out, "ADR-") && strings.Contains(out, "## Warnings")

	if hasZero && hasADRWarning {
		t.Errorf("contradição interna: saída declara ADRs (0) e nomeia ADRs em warnings:\n%s", out)
	}
}

// TestListADRs_Subpastas afirma que ListADRs enumera ADRs em subpastas de estado —
// corrigindo S6 (ADR-2026-09-29): filepath.Glob raiz-only produzia "No ADRs found" mesmo com
// ADRs no disco.
//
// Reconciliação: este teste mede T3 da Wave 0: com 4 ADRs em subpastas, ListADRs devia retornar
// "No ADRs found" antes do fix; após o fix deve listar os 4 arquivos.
func TestListADRs_Subpastas(t *testing.T) {
	dir := t.TempDir()
	adrDir := filepath.Join(dir, "docs/adr/zeus")

	for _, sub := range []string{"done", "wip"} {
		if err := os.MkdirAll(filepath.Join(adrDir, sub), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	adrFiles := []string{
		filepath.Join(adrDir, "done/ADR-2026-09-01-a.md"),
		filepath.Join(adrDir, "done/ADR-2026-09-02-b.md"),
		filepath.Join(adrDir, "done/ADR-2026-09-03-c.md"),
		filepath.Join(adrDir, "wip/ADR-2026-09-04-d.md"),
	}
	for _, f := range adrFiles {
		if err := os.WriteFile(f, []byte("---\nstatus: Accepted\n---\n# ADR: teste\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	// Captura stdout
	tmpFile, err := os.CreateTemp(t.TempDir(), "list-out-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer tmpFile.Close()

	origStdout := os.Stdout
	os.Stdout = tmpFile
	listErr := ListADRs(adrDir)
	os.Stdout = origStdout

	if listErr != nil {
		t.Fatalf("ListADRs: %v", listErr)
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tmpFile); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "No ADRs found") {
		t.Errorf("ListADRs retornou 'No ADRs found' com 4 ADRs em subpastas; saída:\n%s", out)
	}
	for _, f := range adrFiles {
		base := filepath.Base(f)
		if !strings.Contains(out, base) {
			t.Errorf("ListADRs: arquivo %s não aparece na saída:\n%s", base, out)
		}
	}
}

// TestNewADRDraft_NaoCriaDuplicadoSubpasta afirma que NewADRDraft não cria rascunho duplicado
// quando um ADR com o mesmo slug já existe em subpasta de estado — corrigindo S7 (ADR-2026-09-29):
// filepath.Glob raiz-only não encontrava o twin e criava um segundo arquivo.
//
// Reconciliação: este teste mede T4 da Wave 0: antes do fix, o twin em wip/ era invisível ao
// Glob; após o fix, o scan recursivo encontra o twin e retorna "skipped".
func TestNewADRDraft_NaoCriaDuplicadoSubpasta(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	adrDir := filepath.Join(dir, "docs/adr/zeus")
	// Criar twin em subpasta wip/
	if err := os.MkdirAll(filepath.Join(adrDir, "wip"), 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}
	twinPath := filepath.Join(adrDir, "wip/ADR-2026-09-01-minha-decisao.md")
	if err := os.WriteFile(twinPath, []byte("---\nstatus: Draft\n---\n# ADR: minha decisao\n"), 0644); err != nil {
		t.Fatalf("write twin: %v", err)
	}

	// Chamar NewADRDraft com o mesmo slug — deve detectar o twin e não criar novo
	basename, err := NewADRDraft("minha-decisao", adrDir)
	if err != nil {
		t.Fatalf("NewADRDraft: %v", err)
	}

	// Deve retornar o basename do twin (sem criar arquivo novo na raiz)
	if basename == "" {
		t.Fatal("NewADRDraft retornou basename vazio esperando skip")
	}

	// Verificar que NÃO foi criado novo arquivo na raiz de adrDir
	rootMatches, _ := filepath.Glob(filepath.Join(adrDir, "ADR-*-minha-decisao.md"))
	if len(rootMatches) > 0 {
		t.Errorf("NewADRDraft criou arquivo duplicado na raiz: %v", rootMatches)
	}
}
