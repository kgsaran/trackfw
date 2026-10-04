package generators

// roadmap_move_wave0_cutoff_test.go — D5 (ADR-2026-10-04, REQ #514 ML-1C).
//
// Tests for the Wave 0 cutoff in the move→done gate (generators/roadmap.go).
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara, em uma frase, qual
// conclusão do ML-1C ele afirma.
//
// Sabotagem coberta: S4 (isenção ignorada no move→done) é derrubado por
// TestMoveDoneWave0Cutoff_PreCutoffNoWave0Succeeds.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMoveDoneWave0 creates a flat-mode temp dir with a roadmap in wip/
// and all MLs ✅ Concluído (so pending-ML gate does not fire).
// Returns the root dir.
func setupMoveDoneWave0(t *testing.T, name, frontmatterDate string) string {
	t.Helper()
	fm := "---\nstatus: wip\n"
	if frontmatterDate != "" {
		fm += "date: " + frontmatterDate + "\n"
	}
	fm += "---\n"
	// Body: Wave 1 with one completed ML, NO ## Wave 0 heading anywhere.
	body := "\n# Roadmap: cutoff fixture\n\n## Wave 1 — Implementation\n\n### ML-1A — finished\n**Status:** ✅ Concluído\n"
	return setupMove(t, name, fm+body)
}

// ─────────────────────────────────────────────────────────────────────────────
// Case 1 — pre-cutoff (by filename date, no frontmatter date)
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveDoneWave0Cutoff_PreCutoffNoWave0Succeeds — AFIRMA a conclusão do ML-1C
// de que um roadmap datado estritamente antes de 2026-09-18 (data do nome, sem
// date: no frontmatter) pode ser movido para done/ sem ## Wave 0, e a saída
// contém "Wave 0 not required" com a data lida.
//
// Sabotagem S4: se o gate ignorar a isenção (sempre exigir Wave 0), este teste
// falha porque MoveRoadmap retornaria erro.
func TestMoveDoneWave0Cutoff_PreCutoffNoWave0Succeeds(t *testing.T) {
	const name = "ROADMAP-2026-09-12-cutoff-pre.md"
	dir := setupMoveDoneWave0(t, name, "") // sem date: no frontmatter

	var moveErr error
	out := captureStdout(t, func() {
		moveErr = MoveRoadmap(name, "done")
	})

	if moveErr != nil {
		t.Fatalf("roadmap pré-cutoff sem Wave 0 deve mover para done/, obteve erro: %v", moveErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); err != nil {
		t.Errorf("roadmap não encontrado em done/ após move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "roadmaps", "wip", name)); err == nil {
		t.Error("roadmap ainda presente em wip/ após move bem-sucedido")
	}
	if !strings.Contains(out, "Wave 0 not required") {
		t.Errorf("saída deve conter 'Wave 0 not required', obteve: %q", out)
	}
	if !strings.Contains(out, "2026-09-12") {
		t.Errorf("saída deve conter a data do roadmap '2026-09-12', obteve: %q", out)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Case 2 — on-cutoff (2026-09-18 in filename, no frontmatter date)
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveDoneWave0Cutoff_OnCutoffNoWave0Refused — AFIRMA a conclusão do ML-1C
// de que um roadmap datado exatamente em 2026-09-18 (o cutoff) sem Wave 0 é
// RECUSADO: o corte é < (estritamente menor que), então a data de entrada não dispensa.
func TestMoveDoneWave0Cutoff_OnCutoffNoWave0Refused(t *testing.T) {
	const name = "ROADMAP-2026-09-18-cutoff-on.md"
	setupMoveDoneWave0(t, name, "") // sem date: no frontmatter

	var moveErr error
	out := captureStdout(t, func() {
		moveErr = MoveRoadmap(name, "done")
	})

	if moveErr == nil {
		t.Fatal("roadmap datado 2026-09-18 (on-cutoff) sem Wave 0 deve ser recusado")
	}
	if !strings.Contains(moveErr.Error(), "Wave 0") {
		t.Errorf("mensagem de recusa deve mencionar 'Wave 0', obteve: %q", moveErr.Error())
	}
	if !strings.Contains(moveErr.Error(), "1 blocker(s)") {
		t.Errorf("mensagem de recusa deve conter '1 blocker(s)', obteve: %q", moveErr.Error())
	}
	if strings.Contains(out, "Wave 0 not required") {
		t.Errorf("saída NÃO deve conter 'Wave 0 not required' para roadmap recusado, obteve: %q", out)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Case 3 — no legible date
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveDoneWave0Cutoff_NoDateFailsClosed — AFIRMA a semântica fail-closed do ML-1C:
// um roadmap sem data legível (sem date: e sem AAAA-MM-DD no nome) sem Wave 0 é
// RECUSADO — a ausência de data não abre bypass de isenção.
func TestMoveDoneWave0Cutoff_NoDateFailsClosed(t *testing.T) {
	const name = "ROADMAP-sem-data.md"
	setupMoveDoneWave0(t, name, "") // sem date: no frontmatter, sem data no nome

	var moveErr error
	out := captureStdout(t, func() {
		moveErr = MoveRoadmap(name, "done")
	})

	if moveErr == nil {
		t.Fatal("roadmap sem data legível sem Wave 0 deve ser recusado (fail-closed)")
	}
	if !strings.Contains(moveErr.Error(), "Wave 0") {
		t.Errorf("mensagem de recusa deve mencionar 'Wave 0', obteve: %q", moveErr.Error())
	}
	if strings.Contains(out, "Wave 0 not required") {
		t.Errorf("saída NÃO deve conter 'Wave 0 not required' para roadmap sem data, obteve: %q", out)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Case 4 — frontmatter date wins over filename date
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveDoneWave0Cutoff_FrontmatterDateWins — AFIRMA a régua D6/T7 do ML-1B
// no gate do move→done: date: do frontmatter vence sobre a data do nome.
// Nome tem 2026-09-12 (pré-cutoff → isenção), mas frontmatter tem 2026-09-20
// (pós-cutoff → cobrado): o move DEVE ser recusado.
//
// Sabotagem S5: se RoadmapCreationDate ignorar o frontmatter (usar só o nome),
// este teste falha porque o roadmap seria erroneamente isento.
func TestMoveDoneWave0Cutoff_FrontmatterDateWins(t *testing.T) {
	const name = "ROADMAP-2026-09-12-fm-wins.md"
	setupMoveDoneWave0(t, name, "2026-09-20") // frontmatter pós-cutoff, nome pré-cutoff

	var moveErr error
	out := captureStdout(t, func() {
		moveErr = MoveRoadmap(name, "done")
	})

	if moveErr == nil {
		t.Fatal("frontmatter pós-cutoff (2026-09-20) deve vencer nome pré-cutoff (2026-09-12) — roadmap deve ser recusado")
	}
	if !strings.Contains(moveErr.Error(), "Wave 0") {
		t.Errorf("mensagem de recusa deve mencionar 'Wave 0', obteve: %q", moveErr.Error())
	}
	if strings.Contains(out, "Wave 0 not required") {
		t.Errorf("saída NÃO deve conter 'Wave 0 not required' quando frontmatter vence com data pós-cutoff, obteve: %q", out)
	}
}
