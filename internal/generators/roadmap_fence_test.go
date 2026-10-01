package generators

// roadmap_fence_test.go — ML-2B (REQ #476)
//
// Testa as três superfícies que o ML-2B fecha:
//   (A) `roadmap move ... done`  — recusado com cerca aberta, aceito com fecha­da
//   (B) `roadmap show`           — exit 2 + stderr canônico + stdout vazio
//   (C) `roadmap show --json`    — exit 2 + stderr canônico + stdout vazio
//
// Os testes (B) e (C) usam o padrão de re-exec: o binário de testes é invocado
// como subprocesso com uma variável de ambiente que ativa o modo "helper".  No
// modo helper o subprocesso chama ShowRoadmap / ShowRoadmapJSON diretamente e
// sai; o processo pai verifica o exit code, stderr e stdout.
//
// Cada teste declara explicitamente a conclusão que afirma (Regra Dura de
// Reconciliação, CLAUDE.md).

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────────────────────────────────────

// fenceOpenContent: roadmap com cerca aberta na linha 15 (1-based).
// Todos os MLs concluídos, Wave 0 presente — apenas a cerca impede a transição.
//
// Contagem de linhas (1-based):
//  1 ---
//  2 status: wip
//  3 date: 2026-09-30
//  4 ---
//  5 (blank)
//  6 # Roadmap: cerca aberta
//  7 (blank)
//  8 ## Wave 0 — Threat Model
//  9 (blank)
// 10 ## Wave 1 — Implementação
// 11 (blank)
// 12 ### ML-1A — feito
// 13 **Status:** ✅ Concluído
// 14 (blank)
// 15 ```bash          ← abertura não fechada
// 16 echo hello
const fenceOpenContent = "---\nstatus: wip\ndate: 2026-09-30\n---\n\n# Roadmap: cerca aberta\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Implementação\n\n### ML-1A — feito\n**Status:** ✅ Concluído\n\n```bash\necho hello\n"

// fenceClosedContent: o mesmo roadmap com a cerca fechada (adiciona ``` após echo).
const fenceClosedContent = "---\nstatus: wip\ndate: 2026-09-30\n---\n\n# Roadmap: cerca aberta\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Implementação\n\n### ML-1A — feito\n**Status:** ✅ Concluído\n\n```bash\necho hello\n```\n"

// fenceOpenLine é a linha 1-based da abertura não fechada em fenceOpenContent.
const fenceOpenLine = 15

// ─────────────────────────────────────────────────────────────────────────────
// (A) move→done
// ─────────────────────────────────────────────────────────────────────────────

// TestFenceMoveBlockedOnOpenFence
//
// AFIRMA: `roadmap move ... done` recusa a transição quando o arquivo tem cerca
// aberta, mesmo que todos os MLs estejam concluídos e o Wave 0 esteja presente.
// A mensagem de recusa contém a mensagem canônica ("unterminated code fence
// starting at line 15") e NÃO contém "missing ## Wave 0" nem "ML-1A" — esses
// seriam artefatos das predicados fail-closed (AC do ML-1A), não achados reais.
// O arquivo permanece em wip/.
func TestFenceMoveBlockedOnOpenFence(t *testing.T) {
	const name = "ROADMAP-fence-open.md"
	setupMoveML(t, name, fenceOpenContent)

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap deveria recusar cerca aberta, retornou nil")
	}

	msg := err.Error()

	// Mensagem canônica presente.
	want := fmt.Sprintf("unterminated code fence starting at line %d", fenceOpenLine)
	if !strings.Contains(msg, want) {
		t.Errorf("mensagem deveria conter %q; got: %q", want, msg)
	}

	// Artefatos de fail-closed ausentes — se aparecerem, o check está na ordem errada.
	if strings.Contains(msg, "missing ## Wave 0") {
		t.Errorf("mensagem NÃO deveria conter 'missing ## Wave 0' (artefato fail-closed); got: %q", msg)
	}
	if strings.Contains(msg, "ML-1A") {
		t.Errorf("mensagem NÃO deveria conter 'ML-1A' (artefato fail-closed); got: %q", msg)
	}

	// Arquivo permanece em wip/.
	if _, statErr := os.Stat(filepath.Join("docs", "roadmaps", "done", name)); statErr == nil {
		t.Error("arquivo foi movido para done/ apesar da cerca aberta — gate tardio")
	}
}

// TestFenceMoveAcceptedOnClosedFence
//
// AFIRMA: quando a cerca está fechada, todos os MLs concluídos e o Wave 0
// presente, `roadmap move ... done` aceita a transição sem erro.  Sem este
// braço, o gate poderia ser um bloqueio incondicional.
func TestFenceMoveAcceptedOnClosedFence(t *testing.T) {
	const name = "ROADMAP-fence-closed.md"
	dir := setupMoveML(t, name, fenceClosedContent)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap deveria aceitar cerca fechada, retornou: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); statErr != nil {
		t.Errorf("arquivo não encontrado em done/ após move legítimo: %v", statErr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Re-exec helpers — (B) show e (C) show --json
// ─────────────────────────────────────────────────────────────────────────────

// fenceHelperEnv é a variável de ambiente que ativa o modo subprocesso.
// Valores: "show_open", "show_open_json", "show_closed", "show_closed_json".
const fenceHelperEnv = "TRACKFW_TEST_FENCE_HELPER"

// fenceHelperDir é a variável que passa o diretório de trabalho para o subprocesso.
const fenceHelperDir = "TRACKFW_TEST_FENCE_DIR"

// fenceHelperName é a variável que passa o nome do roadmap para o subprocesso.
const fenceHelperName = "TRACKFW_TEST_FENCE_NAME"

// init detecta o modo helper e age como tal ANTES que qualquer TestXxx rode.
// O padrão é: se a variável de ambiente está definida, o subprocesso
// realiza a operação pedida e termina — o test runner do pacote não chega a
// rodar nenhum teste de verdade nesse processo.
//
// Usamos init() em vez de TestMain para não alterar o comportamento padrão de
// todos os testes do pacote; outros testes não têm TestMain e não deveriam
// precisar de um só por causa deste arquivo.
func init() {
	mode := os.Getenv(fenceHelperEnv)
	if mode == "" {
		return
	}

	dir := os.Getenv(fenceHelperDir)
	name := os.Getenv(fenceHelperName)

	if dir == "" || name == "" {
		fmt.Fprintln(os.Stderr, "fence helper: TRACKFW_TEST_FENCE_DIR e TRACKFW_TEST_FENCE_NAME são obrigatórios")
		os.Exit(3)
	}

	if err := os.Chdir(dir); err != nil {
		fmt.Fprintf(os.Stderr, "fence helper: chdir: %v\n", err)
		os.Exit(3)
	}
	config.Reset()

	switch mode {
	case "show_open", "show_closed":
		_ = ShowRoadmap(name) // exit 2 no show_open; retorna nil no show_closed
		os.Exit(0)
	case "show_open_json", "show_closed_json":
		_ = ShowRoadmapJSON(name) // exit 2 no show_open_json; retorna nil no show_closed_json
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "fence helper: modo desconhecido %q\n", mode)
		os.Exit(3)
	}
}

// runFenceSubprocess executa o binário de teste como subprocesso com o modo
// dado, a fixture no dir informado e o nome do roadmap.
// Devolve stdout, stderr e o exit code.
func runFenceSubprocess(t *testing.T, testName, mode, dir, name string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(),
		fenceHelperEnv+"="+mode,
		fenceHelperDir+"="+dir,
		fenceHelperName+"="+name,
	)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("erro ao executar subprocesso: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// setupFenceFixture cria a estrutura de diretórios e escreve o conteúdo do
// roadmap em docs/roadmaps/wip/<name> dentro de um t.TempDir().
func setupFenceFixture(t *testing.T, name, content string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	wipDir := filepath.Join(dir, "docs", "roadmaps", "wip")
	if err := os.MkdirAll(wipDir, 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wipDir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// (B) roadmap show — texto
// ─────────────────────────────────────────────────────────────────────────────

// TestFenceShowExits2OnOpenFence
//
// AFIRMA: `roadmap show` com cerca aberta sai com exit 2, a mensagem canônica
// vai para stderr, e o stdout fica vazio (nenhum documento parcial emitido).
func TestFenceShowExits2OnOpenFence(t *testing.T) {
	const name = "ROADMAP-fence-open.md"
	dir := setupFenceFixture(t, name, fenceOpenContent)
	// resolveRoadmapMatches usa glob *<name>*.md — sem ".md" no fragmento
	const nameFragment = "fence-open"

	stdout, stderr, code := runFenceSubprocess(t, "TestFenceShowExits2OnOpenFence", "show_open", dir, nameFragment)

	if code != 2 {
		t.Fatalf("exit esperado 2, obteve %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout deve ser vazio com cerca aberta, obteve: %q", stdout)
	}
	want := fmt.Sprintf("unterminated code fence starting at line %d", fenceOpenLine)
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr deve conter %q; got: %q", want, stderr)
	}
}

// TestFenceShowWellFormedPassthrough
//
// AFIRMA: `roadmap show` com cerca fechada sai com exit 0 e emite o cabeçalho
// no stdout.  Sem este braço, um gate incondicional passaria nos testes acima.
func TestFenceShowWellFormedPassthrough(t *testing.T) {
	const name = "ROADMAP-fence-closed.md"
	dir := setupFenceFixture(t, name, fenceClosedContent)
	const nameFragment = "fence-closed"

	stdout, stderr, code := runFenceSubprocess(t, "TestFenceShowWellFormedPassthrough", "show_closed", dir, nameFragment)

	if code != 0 {
		t.Fatalf("exit esperado 0, obteve %d (stderr: %s)", code, stderr)
	}
	// O cabeçalho canônico do show deve estar no stdout.
	if !strings.Contains(stdout, "── "+name+" ──") {
		t.Errorf("stdout deve conter o cabeçalho do show; got: %q", stdout)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// (C) roadmap show --json
// ─────────────────────────────────────────────────────────────────────────────

// TestFenceShowJSONExits2OnOpenFence
//
// AFIRMA: `roadmap show --json` com cerca aberta sai com exit 2, a mensagem
// canônica vai para stderr, e o stdout fica vazio (nenhum JSON parcial emitido).
func TestFenceShowJSONExits2OnOpenFence(t *testing.T) {
	const name = "ROADMAP-fence-open.md"
	dir := setupFenceFixture(t, name, fenceOpenContent)
	const nameFragment = "fence-open"

	stdout, stderr, code := runFenceSubprocess(t, "TestFenceShowJSONExits2OnOpenFence", "show_open_json", dir, nameFragment)

	if code != 2 {
		t.Fatalf("exit esperado 2, obteve %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout deve ser vazio com cerca aberta, obteve: %q", stdout)
	}
	want := fmt.Sprintf("unterminated code fence starting at line %d", fenceOpenLine)
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr deve conter %q; got: %q", want, stderr)
	}
}

// TestFenceShowJSONWellFormedPassthrough
//
// AFIRMA: `roadmap show --json` com cerca fechada sai com exit 0 e emite JSON
// válido no stdout (começa com '{').  Sem este braço, um gate incondicional
// passaria nos testes acima.
func TestFenceShowJSONWellFormedPassthrough(t *testing.T) {
	const name = "ROADMAP-fence-closed.md"
	dir := setupFenceFixture(t, name, fenceClosedContent)
	const nameFragment = "fence-closed"

	stdout, stderr, code := runFenceSubprocess(t, "TestFenceShowJSONWellFormedPassthrough", "show_closed_json", dir, nameFragment)

	if code != 0 {
		t.Fatalf("exit esperado 0, obteve %d (stderr: %s)", code, stderr)
	}
	// JSON deve começar com '{'.
	trimmed := strings.TrimSpace(stdout)
	if !strings.HasPrefix(trimmed, "{") {
		t.Errorf("stdout deve ser JSON (começa com '{'); got: %q", stdout)
	}
}
