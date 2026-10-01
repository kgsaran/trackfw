package commands

// roadmap_show_fence_test.go — ML-2C (REQ #476)
//
// Testa o binário completo: `trackfw roadmap show` e `roadmap show --json`
// com cerca aberta (exit 2) e cerca fechada (exit 0).
//
// Usa barrierBinary(t) de barrier_contract_test.go (mesmo pacote) para obter
// o binário compilado uma única vez por suite. O diretório de trabalho passado
// via cmd.Dir contém uma fixture mínima: docs/roadmaps/wip/<name>.md.
//
// Cada teste declara a conclusão que afirma (Regra Dura de Reconciliação).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixture
// ─────────────────────────────────────────────────────────────────────────────

const showFenceOpenContent = "---\nstatus: wip\ndate: 2026-09-30\n---\n\n# Roadmap: show fence\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Implementação\n\n### ML-1A — feito\n**Status:** ✅ Concluído\n\n```bash\necho hello\n"
const showFenceClosedContent = "---\nstatus: wip\ndate: 2026-09-30\n---\n\n# Roadmap: show fence\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Implementação\n\n### ML-1A — feito\n**Status:** ✅ Concluído\n\n```bash\necho hello\n```\n"

// setupShowFenceDir cria um diretório temporário com docs/roadmaps/wip/<name>.
func setupShowFenceDir(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	wipDir := filepath.Join(dir, "docs", "roadmaps", "wip")
	if err := os.MkdirAll(wipDir, 0755); err != nil {
		t.Fatalf("setupShowFenceDir: mkdirall: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wipDir, name), []byte(content), 0644); err != nil {
		t.Fatalf("setupShowFenceDir: write: %v", err)
	}
	return dir
}

// runRoadmapShowCLI invoca `trackfw roadmap show [--json] <fragment>` a partir
// de dir e devolve stdout, stderr e o exit code.
func runRoadmapShowCLI(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := barrierBinary(t)
	fullArgs := append([]string{"roadmap", "show"}, args...)
	cmd := exec.Command(bin, fullArgs...)
	cmd.Dir = dir
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run trackfw roadmap show: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// ─────────────────────────────────────────────────────────────────────────────
// Testes
// ─────────────────────────────────────────────────────────────────────────────

// TestRoadmapShowBinary_OpenFence_Exit2
//
// AFIRMA: `trackfw roadmap show` com cerca aberta sai com exit 2; o stderr
// contém "trackfw roadmap:" e a mensagem canônica "unterminated code fence
// starting at line 15"; o stdout está vazio.
func TestRoadmapShowBinary_OpenFence_Exit2(t *testing.T) {
	const name = "ROADMAP-show-open.md"
	dir := setupShowFenceDir(t, name, showFenceOpenContent)

	stdout, stderr, code := runRoadmapShowCLI(t, dir, "show-open")

	if code != 2 {
		t.Fatalf("exit esperado 2, obteve %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout deve ser vazio com cerca aberta; obteve: %q", stdout)
	}
	if !strings.Contains(stderr, "trackfw roadmap:") {
		t.Errorf("stderr deve conter 'trackfw roadmap:'; got: %q", stderr)
	}
	if !strings.Contains(stderr, "unterminated code fence starting at line 15") {
		t.Errorf("stderr deve conter a mensagem canônica; got: %q", stderr)
	}
}

// TestRoadmapShowBinary_ClosedFence_Exit0
//
// AFIRMA: `trackfw roadmap show` com cerca fechada sai com exit 0 e emite o
// cabeçalho canônico no stdout. Sem este braço, um bloqueio incondicional
// passaria no teste acima.
func TestRoadmapShowBinary_ClosedFence_Exit0(t *testing.T) {
	const name = "ROADMAP-show-closed.md"
	dir := setupShowFenceDir(t, name, showFenceClosedContent)

	stdout, _, code := runRoadmapShowCLI(t, dir, "show-closed")

	if code != 0 {
		t.Fatalf("exit esperado 0, obteve %d", code)
	}
	if !strings.Contains(stdout, "── "+name+" ──") {
		t.Errorf("stdout deve conter o cabeçalho; got: %q", stdout)
	}
}

// TestRoadmapShowJSONBinary_OpenFence_Exit2
//
// AFIRMA: `trackfw roadmap show --json` com cerca aberta sai com exit 2; o
// stderr contém "trackfw roadmap:" e a mensagem canônica; o stdout está vazio.
func TestRoadmapShowJSONBinary_OpenFence_Exit2(t *testing.T) {
	const name = "ROADMAP-showjson-open.md"
	dir := setupShowFenceDir(t, name, showFenceOpenContent)

	stdout, stderr, code := runRoadmapShowCLI(t, dir, "--json", "showjson-open")

	if code != 2 {
		t.Fatalf("exit esperado 2, obteve %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout deve ser vazio com cerca aberta; obteve: %q", stdout)
	}
	if !strings.Contains(stderr, "trackfw roadmap:") {
		t.Errorf("stderr deve conter 'trackfw roadmap:'; got: %q", stderr)
	}
	if !strings.Contains(stderr, "unterminated code fence starting at line 15") {
		t.Errorf("stderr deve conter a mensagem canônica; got: %q", stderr)
	}
}

// TestRoadmapShowJSONBinary_ClosedFence_Exit0
//
// AFIRMA: `trackfw roadmap show --json` com cerca fechada sai com exit 0 e
// emite JSON válido no stdout (começa com '{'). Sem este braço, um bloqueio
// incondicional passaria no teste acima.
func TestRoadmapShowJSONBinary_ClosedFence_Exit0(t *testing.T) {
	const name = "ROADMAP-showjson-closed.md"
	dir := setupShowFenceDir(t, name, showFenceClosedContent)

	stdout, _, code := runRoadmapShowCLI(t, dir, "--json", "showjson-closed")

	if code != 0 {
		t.Fatalf("exit esperado 0, obteve %d", code)
	}
	trimmed := strings.TrimSpace(stdout)
	if !strings.HasPrefix(trimmed, "{") {
		t.Errorf("stdout deve ser JSON (começa com '{'); got: %q", stdout)
	}
}
