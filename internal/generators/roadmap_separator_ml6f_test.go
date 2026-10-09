package generators

// roadmap_separator_ml6f_test.go — testes do ML-6F (REQ-2026-09-09):
// o vínculo REQ↔roadmap é gravado com "/" em qualquer SO.
//
// ADR-2026-09-01 D1: caminhos dentro de artefato versionado usam sempre "/".
// Defeito corrigido: NewRoadmapFromContent e NewRoadmapFromREQ gravavam
// content.REQPath / reqPath diretamente no template; no Windows esses valores
// chegam com "\" (filepath.Join nativo).
//
// Falsificação: remover o portableREQPath / portableReqPath dos dois Sprintf e
// usar content.REQPath / reqPath diretamente → TestNewRoadmapFromContent_BackslashREQPathNormalized
// reprova porque o roadmap escrito contém "\" no campo req:.
//
// Cada teste declara, no próprio comentário, qual conclusão do ML ele afirma
// (Regra Dura de Reconciliação — CLAUDE.md).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupSep6FDir cria um projeto temporário flat com trackfw.yaml e muda o cwd.
func setupSep6FDir(t *testing.T) string {
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
	config.Reset()
	t.Cleanup(config.Reset)
	return dir
}

// TestNewRoadmapFromContent_BackslashREQPathNormalized: o template de NewRoadmapFromContent
// normaliza content.REQPath com normalizeRefSeparator antes de escrever o roadmap; quando
// content.REQPath chega com "\" (como ocorre no Windows via filepath.Join), os campos
// req: (frontmatter) e REQ: (corpo) do arquivo gravado contêm apenas "/".
//
// Reconciliação ML-6F: afirma que o caminho de criação do roadmap grava separador portável
// independente do SO — o fix central do ML-6F.
func TestNewRoadmapFromContent_BackslashREQPathNormalized(t *testing.T) {
	dir := setupSep6FDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", "backlog"), 0o755); err != nil {
		t.Fatalf("mkdir backlog: %v", err)
	}

	// Injeta caminho com "\" simulando o que filepath.Join produz no Windows.
	// NewRoadmapFromContent não lê o arquivo da REQ (só escreve o roadmap), então
	// não precisamos que o arquivo exista; linkREQToRoadmap vai falhar silenciosamente
	// e não alterar o resultado do teste.
	dirtyREQPath := `docs\req\REQ-2026-10-09-separador.md`

	content := RoadmapContent{
		Title:   "separador portavel",
		REQPath: dirtyREQPath,
	}
	if err := NewRoadmapFromContent(content); err != nil {
		t.Fatalf("NewRoadmapFromContent: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}
	got, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile roadmap: %v", err)
	}
	body := string(got)

	// req: no frontmatter deve usar "/"
	if i := strings.Index(body, `req: "`); i >= 0 {
		line := body[i:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if strings.ContainsRune(line, '\\') {
			t.Errorf("req: frontmatter contém separador nativo; esperado só '/': %s", strings.TrimSpace(line))
		}
		wantSlash := `req: "docs/req/REQ-2026-10-09-separador.md"`
		if !strings.Contains(body, wantSlash) {
			t.Errorf("req: frontmatter esperado %q, corpo:\n%s", wantSlash, body)
		}
	} else {
		t.Errorf("campo req: não encontrado no roadmap gravado:\n%s", body)
	}

	// REQ: no corpo deve usar "/"
	if i := strings.Index(body, "REQ: "); i >= 0 {
		line := body[i:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if strings.ContainsRune(line, '\\') {
			t.Errorf("REQ: corpo contém separador nativo; esperado só '/': %s", strings.TrimSpace(line))
		}
	} else {
		t.Errorf("marcador REQ: não encontrado no corpo do roadmap:\n%s", body)
	}
}

// TestNewRoadmapFromContent_BackslashIsNormalizedToSlash_WindowsIntegration:
// integração via t.TempDir() — exercita o caminho completo (criação + verificação do disco)
// com separador nativo. No macOS filepath.Join produz "/" e o teste confirma que a normalização
// não altera o valor já correto (idempotente). Na VM Windows confirma que filepath.Join
// produz "\" e o arquivo gravado tem apenas "/".
//
// Reconciliação ML-6F: afirma que normalizeRefSeparator é idempotente para entradas com "/"
// e corretora para entradas com "\".
func TestNewRoadmapFromContent_BackslashIsNormalizedToSlash_WindowsIntegration(t *testing.T) {
	dir := setupSep6FDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", "backlog"), 0o755); err != nil {
		t.Fatalf("mkdir backlog: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("mkdir docs/req: %v", err)
	}

	// filepath.Join produz "\" no Windows, "/" no macOS/Linux — caminho nativo real.
	nativeREQPath := filepath.Join("docs", "req", "REQ-2026-10-09-native.md")
	reqBody := `---
status: Open
date: 2026-10-09
adr: ""
roadmap: ""
---

# REQ: separador nativo

## Acceptance Criteria
- [ ] AC1

## Linked Roadmap
Roadmap:
`
	if err := os.WriteFile(filepath.Join(dir, nativeREQPath), []byte(reqBody), 0o644); err != nil {
		t.Fatalf("WriteFile REQ: %v", err)
	}

	content := RoadmapContent{
		Title:   "separador nativo integration",
		REQPath: nativeREQPath,
	}
	if err := NewRoadmapFromContent(content); err != nil {
		t.Fatalf("NewRoadmapFromContent: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}
	got, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile roadmap: %v", err)
	}
	body := string(got)

	if strings.ContainsRune(body, '\\') {
		// Só falha em Windows onde filepath.Join produz "\"; no macOS é sempre "/" e passa.
		t.Errorf("roadmap gravado contém separador nativo (esperado somente '/'):\n%s", body)
	}
	portablePath := filepath.ToSlash(nativeREQPath)
	wantReq := `req: "` + portablePath + `"`
	if !strings.Contains(body, wantReq) {
		t.Errorf("req: esperado %q no roadmap, corpo:\n%s", wantReq, body)
	}
}

// TestLegacyBackslashInRoadmapRef_RecognizedByLinkREQToRoadmap: uma REQ com
// roadmap: "docs\roadmaps\backlog\ROADMAP-x.md" (separador Windows herdado)
// é reconhecida pelo predicado sameBasename de linkREQToRoadmap como já apontando
// para o mesmo roadmap — a linha "already links … not overwritten" não aparece e
// a REQ é curada: o campo roadmap: passa a ter "/".
//
// Reconciliação ML-6F: afirma que a leitura de vínculos legados com "\" continua
// funcionando — regressão do lado de leitura seria partir o vínculo em vez de
// curar.
func TestLegacyBackslashInRoadmapRef_RecognizedByLinkREQToRoadmap(t *testing.T) {
	dir := setupSep6FDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("mkdir docs/req: %v", err)
	}

	// REQ com separador Windows (como gerado por versões antigas do produto).
	reqName := "REQ-2026-10-09-legado.md"
	reqRel := filepath.Join("docs", "req", reqName)
	legacyRoadmapRef := `docs\roadmaps\backlog\ROADMAP-2026-10-09-legado.md`
	reqBody := "---\nstatus: Open\ndate: 2026-10-09\nadr: \"\"\nroadmap: \"" + legacyRoadmapRef + "\"\n---\n\n# REQ: legado\n\n## Linked Roadmap\nRoadmap: " + legacyRoadmapRef + "\n"
	if err := os.WriteFile(filepath.Join(dir, reqRel), []byte(reqBody), 0o644); err != nil {
		t.Fatalf("WriteFile REQ: %v", err)
	}

	// Chama linkREQToRoadmap com a forma portable do mesmo basename.
	// A função deve reconhecer que a REQ já aponta para o mesmo basename
	// (via normalizeRefSeparator no predicado sameBasename) e NÃO imprimir
	// "already links … not overwritten".
	portableRoadmapPath := "docs/roadmaps/backlog/ROADMAP-2026-10-09-legado.md"

	// Captura stderr para verificar ausência do warning de "already links".
	// Usa a API de captura de output já usada em outros testes do pacote.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	linkREQToRoadmap(reqRel, portableRoadmapPath)

	_ = w.Close()
	os.Stderr = origStderr
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, _ := r.Read(tmp)
		if n == 0 {
			break
		}
		buf.Write(tmp[:n])
	}
	stderrOut := buf.String()

	if strings.Contains(stderrOut, "already links") {
		t.Errorf("linkREQToRoadmap não reconheceu backslash legado como mesmo basename; stderr: %s", stderrOut)
	}

	// Após a chamada, o arquivo deve ter sido curado: roadmap: agora usa "/".
	after, err := os.ReadFile(filepath.Join(dir, reqRel))
	if err != nil {
		t.Fatalf("ReadFile REQ após link: %v", err)
	}
	gotBody := string(after)

	// O campo roadmap: no frontmatter deve ter o caminho portável.
	wantFM := `roadmap: "` + portableRoadmapPath + `"`
	if !strings.Contains(gotBody, wantFM) {
		t.Errorf("REQ não foi curada: esperado %q, frontmatter:\n%s", wantFM, gotBody)
	}
	// Não deve ter "\" no campo roadmap: depois da cura.
	if i := strings.Index(gotBody, `roadmap: "`); i >= 0 {
		line := gotBody[i:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if strings.ContainsRune(line, '\\') {
			t.Errorf("campo roadmap: ainda contém separador nativo após cura: %s", strings.TrimSpace(line))
		}
	}
}
