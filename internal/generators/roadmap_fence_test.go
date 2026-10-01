package generators

// roadmap_fence_test.go — ML-2C (REQ #476)
//
// Testa as três superfícies que o ML-2B/ML-2C fecha:
//   (A) `roadmap move ... done`  — recusado com cerca aberta, aceito com fechada
//   (B) `roadmap show`           — retorna *UsageError + stdout vazio
//   (C) `roadmap show --json`    — retorna *UsageError + stdout vazio
//
// Os testes (B) e (C) são in-process: chamam ShowRoadmap / ShowRoadmapJSON
// diretamente; o exit 2 fica no handler de commands/ (ML-2C). O stdout é capturado
// com captureStdout (roadmap_test.go, mesmo pacote) para confirmar que nenhum
// documento parcial é emitido antes do retorno.
//
// Cada teste declara explicitamente a conclusão que afirma (Regra Dura de
// Reconciliação, CLAUDE.md).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
// (B) roadmap show — texto (in-process, ML-2C)
// ─────────────────────────────────────────────────────────────────────────────

// TestFenceShowUsageErrorOnOpenFence
//
// AFIRMA: ShowRoadmap com cerca aberta retorna *UsageError cuja mensagem contém
// a string canônica ("unterminated code fence starting at line 15"), e o stdout
// fica vazio — nenhum documento parcial é emitido antes do retorno.
func TestFenceShowUsageErrorOnOpenFence(t *testing.T) {
	const name = "ROADMAP-fence-open-show.md"
	setupMoveML(t, name, fenceOpenContent)

	var err error
	out := captureStdout(t, func() {
		err = ShowRoadmap("fence-open-show")
	})

	if err == nil {
		t.Fatal("ShowRoadmap deveria retornar erro com cerca aberta, retornou nil")
	}
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("esperava *UsageError, obteve %T: %v", err, err)
	}
	want := fmt.Sprintf("unterminated code fence starting at line %d", fenceOpenLine)
	if !strings.Contains(ue.Error(), want) {
		t.Errorf("UsageError.Msg deveria conter %q; got: %q", want, ue.Error())
	}
	if out != "" {
		t.Errorf("stdout deve ser vazio com cerca aberta; obteve: %q", out)
	}
}

// TestFenceShowPassthroughOnClosedFence
//
// AFIRMA: ShowRoadmap com cerca fechada retorna nil e emite o cabeçalho canônico
// no stdout. Sem este braço, um gate incondicional passaria no teste acima.
func TestFenceShowPassthroughOnClosedFence(t *testing.T) {
	const name = "ROADMAP-fence-closed-show.md"
	setupMoveML(t, name, fenceClosedContent)

	var err error
	out := captureStdout(t, func() {
		err = ShowRoadmap("fence-closed-show")
	})

	if err != nil {
		t.Fatalf("ShowRoadmap deveria aceitar cerca fechada, retornou: %v", err)
	}
	if !strings.Contains(out, "── "+name+" ──") {
		t.Errorf("stdout deve conter o cabeçalho do show; got: %q", out)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// (C) roadmap show --json (in-process, ML-2C)
// ─────────────────────────────────────────────────────────────────────────────

// TestFenceShowJSONUsageErrorOnOpenFence
//
// AFIRMA: ShowRoadmapJSON com cerca aberta retorna *UsageError cuja mensagem
// contém a string canônica, e o stdout fica vazio — nenhum JSON parcial é emitido.
func TestFenceShowJSONUsageErrorOnOpenFence(t *testing.T) {
	const name = "ROADMAP-fence-open-json.md"
	setupMoveML(t, name, fenceOpenContent)

	var err error
	out := captureStdout(t, func() {
		err = ShowRoadmapJSON("fence-open-json")
	})

	if err == nil {
		t.Fatal("ShowRoadmapJSON deveria retornar erro com cerca aberta, retornou nil")
	}
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("esperava *UsageError, obteve %T: %v", err, err)
	}
	want := fmt.Sprintf("unterminated code fence starting at line %d", fenceOpenLine)
	if !strings.Contains(ue.Error(), want) {
		t.Errorf("UsageError.Msg deveria conter %q; got: %q", want, ue.Error())
	}
	if out != "" {
		t.Errorf("stdout deve ser vazio com cerca aberta; obteve: %q", out)
	}
}

// TestFenceShowJSONPassthroughOnClosedFence
//
// AFIRMA: ShowRoadmapJSON com cerca fechada retorna nil e emite JSON válido no
// stdout (começa com '{'). Sem este braço, um gate incondicional passaria no
// teste acima.
func TestFenceShowJSONPassthroughOnClosedFence(t *testing.T) {
	const name = "ROADMAP-fence-closed-json.md"
	setupMoveML(t, name, fenceClosedContent)

	var err error
	out := captureStdout(t, func() {
		err = ShowRoadmapJSON("fence-closed-json")
	})

	if err != nil {
		t.Fatalf("ShowRoadmapJSON deveria aceitar cerca fechada, retornou: %v", err)
	}
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "{") {
		t.Errorf("stdout deve ser JSON (começa com '{'); got: %q", out)
	}
}
