package commands

// barrier_fence_test.go — testes do ML-2A (#476): `trackfw barrier` deve sair com
// exit 2 e a mensagem canônica da cerca quando o roadmap tiver uma cerca não terminada,
// ANTES de tentar resolver a wave pedida.
//
// Todos os testes usam `barrierBinary` e `runBarrierCLI` de barrier_contract_test.go.
// Nenhum teste usa TRACKFW_BARRIER_STACK. Timeout de 60 s por processo.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── helpers de fixture ─────────────────────────────────────────────────────

// fenceFixtureBase monta o corpo base de um roadmap com Wave 0 terminada e
// Wave 1 com um ML cujo status visível é ✅ Concluído + critérios atendidos.
// O corpo é retornado SEM o final (sem a cerca pendente ou sem o sufixo da cerca
// fechada) — os testes acrescentam o sufixo que querem.
func fenceFixtureBase() string {
	var b strings.Builder
	b.WriteString("# Roadmap: Fence Barrier Fixture\n\n")
	b.WriteString("REQ: docs/req/REQ-2026-07-29-barrier-fixture.md\n\n")
	b.WriteString("## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n")

	// Wave 0 — obrigatória para que o validate não adicione violations espúrias
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model for fixture\n")
	b.WriteString("**Status:** ✅ Concluído\n\n")
	b.WriteString("**Gates da wave:**\n```bash\n")
	b.WriteString("echo \"wave 0 ok\"\n")
	b.WriteString("```\n\n")

	// Wave 1 — ML-1A com status visível e critérios completos
	b.WriteString("## Wave 1 — Test wave\n> Dependências: nenhuma\n\n")
	b.WriteString("**Gates da wave:**\n```bash\necho \"wave 1 ok\"\n```\n\n")
	b.WriteString("### ML-1A — Fixture ML\n")
	b.WriteString("**Status:** ✅ Concluído\n")
	b.WriteString("**Critérios de aceite:**\n- [x] criteria met\n\n")
	return b.String()
}

// setupFenceFixture escreve a árvore de governança mínima + o roadmap com o
// conteúdo passado em roadmapContent e devolve (dir, caminho-absoluto-do-roadmap).
func setupFenceFixture(t *testing.T, roadmapContent string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("setupFenceFixture: mkdirs: %v", err)
		}
	}
	roadmapRel := "docs/roadmaps/wip/ROADMAP-fence-barrier-fixture.md"
	roadmapPath := filepath.Join(dir, roadmapRel)
	if err := os.WriteFile(roadmapPath, []byte(roadmapContent), 0644); err != nil {
		t.Fatalf("setupFenceFixture: write roadmap: %v", err)
	}
	writeBarrierREQFixture(t, dir, roadmapRel)
	return dir, roadmapPath
}

// runFenceBarrier invoca `trackfw barrier ROADMAP-fence-barrier-fixture --wave 1 <extra...>`
// com um timeout de 60 s e devolve stdout, stderr e exit code.
func runFenceBarrier(t *testing.T, dir string, extra ...string) (stdout, stderr string, code int) {
	t.Helper()
	args := append([]string{"ROADMAP-fence-barrier-fixture", "--wave", "1"}, extra...)

	// Timeout guard — barrierBinary runs the real binary; 60 s is generous for a
	// local fence-check that returns in under a second.
	done := make(chan struct{})
	go func() {
		stdout, stderr, code = runBarrierCLI(t, dir, args...)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("runFenceBarrier: timed out after 60 s")
	}
	return
}

// ─── testes ─────────────────────────────────────────────────────────────────

// TestBarrierFence_OpenFencePendingMLInTail asserts that when a roadmap has an
// unterminated code fence that masks a pending ML (branch D of the sonde),
// `trackfw barrier` exits 2 and prints the canonical fence message with the
// correct 1-based line number, instead of silently treating the ML as complete.
func TestBarrierFence_OpenFencePendingMLInTail(t *testing.T) {
	base := fenceFixtureBase()
	// Fence opens at the last line of the base block (line = len(lines)+1).
	// A second ML with ⬜ Pendente is hidden in the masked tail.
	roadmap := base +
		"```bash\necho \"fence opens here\"\n" +
		"### ML-2A — Hidden pending ML\n" +
		"**Status:** ⬜ Pendente\n" +
		"**Critérios de aceite:**\n- [ ] hidden criterion\n"

	// Count the 1-based line of the fence opener to assert in the error message.
	baseLines := strings.Count(base, "\n") + 1 // +1 because base ends without a trailing newline
	fenceOpenerLine := baseLines                // the "```bash" line immediately after base

	dir, _ := setupFenceFixture(t, roadmap)
	_, stderr, code := runFenceBarrier(t, dir, "--trust-local-gates")

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	want := "unterminated code fence starting at line "
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr %q does not contain %q", stderr, want)
	}
	wantLine := strings.Contains(stderr, want+strings.TrimRight(
		strings.TrimLeft(strings.Split(stderr, want)[len(strings.Split(stderr, want))-1], ""), "\n "))
	_ = wantLine
	// Assert the exact line number.
	wantMsg := want + itoa(fenceOpenerLine)
	if !strings.Contains(stderr, wantMsg) {
		t.Errorf("stderr %q does not contain expected line reference %q (fence opens at line %d)",
			stderr, wantMsg, fenceOpenerLine)
	}
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// TestBarrierFence_FenceBeforeWaveHeaderGivesFenceMessage asserts that when an
// unterminated fence opens BEFORE the requested wave heading, the error message
// is the fence diagnostic, NOT "wave not found" — because the fence hides the
// wave heading and "wave not found" would be the wrong message for the same
// exit code.
func TestBarrierFence_FenceBeforeWaveHeaderGivesFenceMessage(t *testing.T) {
	// Build a roadmap where the fence opens inside the Wave 0 section (before
	// the Wave 1 heading), so that Wave 1 is entirely masked.
	var b strings.Builder
	b.WriteString("# Roadmap: Fence Barrier Fixture\n\n")
	b.WriteString("REQ: docs/req/REQ-2026-07-29-barrier-fixture.md\n\n")
	b.WriteString("## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model for fixture\n")
	b.WriteString("**Status:** ✅ Concluído\n\n")
	// Fence opens here — Wave 1 heading comes after, masked entirely.
	b.WriteString("```bash\necho \"fence before wave 1\"\n")
	b.WriteString("## Wave 1 — Test wave\n> Dependências: nenhuma\n\n")
	b.WriteString("### ML-1A — Fixture ML\n")
	b.WriteString("**Status:** ✅ Concluído\n")
	b.WriteString("**Critérios de aceite:**\n- [x] criteria met\n")

	dir, _ := setupFenceFixture(t, b.String())
	_, stderr, code := runFenceBarrier(t, dir)

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if strings.Contains(stderr, "not found") {
		t.Errorf("stderr contains 'not found' when it should show the fence message: %q", stderr)
	}
	if !strings.Contains(stderr, "unterminated code fence starting at line") {
		t.Errorf("stderr %q does not contain fence message", stderr)
	}
}

// TestBarrierFence_WellFormedRoadmapPassesThrough asserts that a well-formed
// roadmap (all fences closed) still produces the normal barrier behavior (AC3):
// the fence check does not block valid documents.
func TestBarrierFence_WellFormedRoadmapPassesThrough(t *testing.T) {
	roadmap := fenceFixtureBase()
	// No unclosed fence — the roadmap ends cleanly after the complete ML block.
	dir, _ := setupFenceFixture(t, roadmap)
	_, stderr, code := runFenceBarrier(t, dir, "--trust-local-gates")

	if strings.Contains(stderr, "unterminated code fence") {
		t.Errorf("well-formed roadmap produced fence error: %q", stderr)
	}
	// The barrier should exit 0 (passed) for a fully complete wave.
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
}

// TestBarrierFence_JsonFlagExits2WithoutDocument asserts that when an
// unterminated fence is detected, `--json` still causes exit 2 and the stdout
// contains no JSON document — only the fence diagnostic on stderr.
func TestBarrierFence_JsonFlagExits2WithoutDocument(t *testing.T) {
	base := fenceFixtureBase()
	roadmap := base + "```bash\necho \"unclosed fence\"\npending content here\n"

	dir, _ := setupFenceFixture(t, roadmap)
	stdout, stderr, code := runFenceBarrier(t, dir, "--json", "--trust-local-gates")

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout should be empty with fence error + --json, got: %q", stdout)
	}
	if !strings.Contains(stderr, "unterminated code fence starting at line") {
		t.Errorf("stderr %q does not contain fence message", stderr)
	}
}
