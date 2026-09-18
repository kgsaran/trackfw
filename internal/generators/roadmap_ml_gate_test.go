package generators

// ML-3A (REQ #392): gate on MoveRoadmap that refuses a transition to "done"
// when the roadmap still contains unfinished MLs.  Five test arms:
//   A — falsification: ⬜ Pendente → must refuse and name label+line
//   B — counter-arm:   all ✅ Concluído → must move
//   C — third arm:     ABANDONADO/❌ Cancelado release; ❌ Bloqueado blocks
//   D — fail-safe:     malformed wave heading → refuses
//   E — AC9:           write-failure propagated, not swallowed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupMoveML creates a flat-mode temp repo under t.TempDir() with the given
// roadmap content placed in docs/roadmaps/wip/<filename>.  Returns the root dir.
// It reuses setupMove from roadmap_move_test.go (same package).
// setupMove already calls chdirADR + config.Reset + t.Cleanup(config.Reset).
func setupMoveML(t *testing.T, filename, content string) string {
	t.Helper()
	return setupMove(t, filename, content)
}

// ─────────────────────────────────────────────────────────────────────────────
// Arm A — falsification
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_PendingML_Refuses_ML3A asserts CONCLUSION: MoveRoadmap
// refuses to move a roadmap to "done" when it contains at least one ML whose
// **Status:** is not complete and not terminated.  The refusal message names
// the ML label and its 1-based line number so the user can locate the blocker.
//
// Falsification: this test MUST fail against the pre-ML-3A code (no gate) and
// pass after the gate is added.  Two execution results are reported by the agent.
func TestMoveRoadmapDone_PendingML_Refuses_ML3A(t *testing.T) {
	// Fixture: one wave, one ML with ⬜ Pendente.
	// Line numbers (1-based):
	//   1  ---
	//   2  status: wip
	//   3  date: 2026-09-18
	//   4  ---
	//   5  (blank)
	//   6  # Roadmap: pending test
	//   7  (blank)
	//   8  ## Wave 1 — Implementation
	//   9  (blank)
	//  10  ### ML-1A — implement it      ← ml.Start index=9, line 10
	//  11  **Status:** ⬜ Pendente
	const content = "---\nstatus: wip\ndate: 2026-09-18\n---\n\n# Roadmap: pending test\n\n## Wave 1 — Implementation\n\n### ML-1A — implement it\n**Status:** ⬜ Pendente\n"
	const name = "ROADMAP-pending-ml.md"

	setupMoveML(t, name, content)

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap(..., \"done\") should refuse when ML-1A is ⬜ Pendente, but returned nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ML-1A") {
		t.Errorf("refusal message should contain ML label %q; got: %q", "ML-1A", msg)
	}
	if !strings.Contains(msg, "10") {
		t.Errorf("refusal message should contain the 1-based line number 10; got: %q", msg)
	}
	// Verify the file was NOT moved (gate must fire before os.Rename).
	if _, statErr := os.Stat(filepath.Join("docs", "roadmaps", "done", name)); statErr == nil {
		t.Error("roadmap was moved to done/ despite pending ML — gate fired too late (after Rename)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Arm B — counter-arm (all complete → must move)
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_AllComplete_Moves_ML3A asserts CONCLUSION: a roadmap
// whose every ML carries a completion marker (✅ Concluído) is allowed through
// the done gate without error.  Without this arm, the gate could be an
// unconditional block that breaks all legitimate transitions.
func TestMoveRoadmapDone_AllComplete_Moves_ML3A(t *testing.T) {
	// Wave 0 added (ML-4B, REQ #392): MoveRoadmap("done") requires ## Wave 0.
	const content = "---\nstatus: wip\ndate: 2026-09-18\n---\n\n# Roadmap: complete test\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Done\n\n### ML-1A — finished\n**Status:** ✅ Concluído\n"
	const name = "ROADMAP-complete-ml.md"

	dir := setupMoveML(t, name, content)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap should succeed when all MLs are ✅ Concluído, but got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); statErr != nil {
		t.Errorf("roadmap not found in done/ after successful move: %v", statErr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Arm C — third arm (Terminated releases; ❌ Bloqueado blocks)
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_AbandonadoReleases_ML3A asserts CONCLUSION: a ML whose
// **Status:** is ABANDONADO (StatusTerminated) is treated as explicitly closed
// and does NOT block the transition to done (ADR 2026-09-18 decision 9).
func TestMoveRoadmapDone_AbandonadoReleases_ML3A(t *testing.T) {
	// Wave 0 added (ML-4B, REQ #392): MoveRoadmap("done") requires ## Wave 0.
	const content = "---\nstatus: wip\n---\n\n# Roadmap: abandonado test\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Closed\n\n### ML-1A — abandoned\n**Status:** ABANDONADO\n"
	const name = "ROADMAP-abandonado-ml.md"

	dir := setupMoveML(t, name, content)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("ABANDONADO should release done transition, got error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); statErr != nil {
		t.Errorf("roadmap not found in done/ after ABANDONADO release: %v", statErr)
	}
}

// TestMoveRoadmapDone_CanceladoReleases_ML3A asserts CONCLUSION: "❌ Cancelado"
// (StatusTerminated — the second-token disambiguator fires at StatusCategory line
// 223) releases the done transition, whereas "❌ Bloqueado" (StatusPending) blocks
// it.  Both share the same first emoji; this arm proves the classifier uses the
// second token, not just the first.
func TestMoveRoadmapDone_CanceladoReleases_ML3A(t *testing.T) {
	// ❌ Cancelado → Terminated → releases
	// Wave 0 added (ML-4B, REQ #392): MoveRoadmap("done") requires ## Wave 0.
	const contentCancelado = "---\nstatus: wip\n---\n\n# Roadmap: cancelado test\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Cancelled\n\n### ML-1A — cancelled\n**Status:** ❌ Cancelado\n"
	const nameCancelado = "ROADMAP-cancelado-ml.md"

	dir := setupMoveML(t, nameCancelado, contentCancelado)
	if err := MoveRoadmap(nameCancelado, "done"); err != nil {
		t.Fatalf("❌ Cancelado should release done transition, got error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", nameCancelado)); statErr != nil {
		t.Errorf("roadmap not found in done/ after ❌ Cancelado release: %v", statErr)
	}
}

// TestMoveRoadmapDone_BloqueadoBlocks_ML3A asserts CONCLUSION: "❌ Bloqueado"
// (StatusPending — same first emoji as ❌ Cancelado but different second token)
// must be refused like any other pending status.  A gate that checks only the
// first token would wrongly release it.
func TestMoveRoadmapDone_BloqueadoBlocks_ML3A(t *testing.T) {
	const content = "---\nstatus: wip\n---\n\n# Roadmap: bloqueado test\n\n## Wave 1 — Blocked\n\n### ML-1A — still blocked\n**Status:** ❌ Bloqueado\n"
	const name = "ROADMAP-bloqueado-ml.md"

	setupMoveML(t, name, content)

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap should refuse ❌ Bloqueado (StatusPending) but returned nil")
	}
	if !strings.Contains(err.Error(), "ML-1A") {
		t.Errorf("refusal for ❌ Bloqueado should name ML-1A; got: %q", err.Error())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Arm D — fail-safe: malformed wave heading → refuse
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_MalformedWave_Refuses_ML3A asserts CONCLUSION: a roadmap
// with a malformed wave heading (label that does not satisfy WaveLabelRe) causes
// the done gate to refuse, because the MLs inside that wave are unreachable and
// their completeness cannot be proven — fail-safe closed (ADR decision 11).
func TestMoveRoadmapDone_MalformedWave_Refuses_ML3A(t *testing.T) {
	// "## Wave reaberta " — no leading digit; fails WaveLabelRe; ParseWaves returns it as MalformedWave.
	const content = "---\nstatus: wip\n---\n\n# Roadmap: malformed wave test\n\n## Wave reaberta — invalid label\n\n### ML-1A — inside malformed\n**Status:** ✅ Concluído\n"
	const name = "ROADMAP-malformed-wave.md"

	setupMoveML(t, name, content)

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap should refuse when ParseWaves returns a MalformedWave, but returned nil")
	}
	if !strings.Contains(err.Error(), "malformed") && !strings.Contains(err.Error(), "reaberta") {
		t.Errorf("refusal message should reference the malformed wave; got: %q", err.Error())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Arm E — AC9: write failure is propagated, not swallowed
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_WriteFailurePropagated_ML3A asserts CONCLUSION: when the
// status-sync write (os.WriteFile on the moved file) fails, MoveRoadmap returns
// a non-nil error.  Before ML-3A the call was `_ = os.WriteFile(...)`, silently
// leaving the frontmatter saying "status: wip" after the file landed in done/.
//
// Mechanism: the source file is created with mode 0444 (read-only).  os.Rename
// preserves the mode; os.ReadFile succeeds; rewriteRoadmapStatus returns
// changed=true; os.WriteFile gets EACCES.
//
// Skipped when os.Geteuid() == 0 (root ignores file-mode permission bits).
func TestMoveRoadmapDone_WriteFailurePropagated_ML3A(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — file-mode permission bits are not enforced, skip write-failure arm")
	}

	// All MLs complete so the new done-gate passes before reaching the write.
	// Wave 0 added (ML-4B, REQ #392): MoveRoadmap("done") requires ## Wave 0.
	const content = "---\nstatus: wip\ndate: 2026-09-18\n---\n\n# Roadmap: write-fail test\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Complete\n\n### ML-1A — done\n**Status:** ✅ Concluído\n"
	const name = "ROADMAP-write-fail.md"

	dir := t.TempDir()
	chdirADR(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	wip := filepath.Join(dir, "docs", "roadmaps", "wip")
	if err := os.MkdirAll(wip, 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}
	srcPath := filepath.Join(wip, name)

	// Write with mode 0444 so os.WriteFile on the destination fails after Rename.
	if err := os.WriteFile(srcPath, []byte(content), 0444); err != nil {
		t.Fatalf("WriteFile 0444: %v", err)
	}
	// Verify the mechanism actually produces EACCES on this machine before asserting.
	probe := filepath.Join(dir, "probe.txt")
	if err := os.WriteFile(probe, []byte("probe"), 0444); err != nil {
		t.Fatalf("create probe 0444: %v", err)
	}
	if probeErr := os.WriteFile(probe, []byte("overwrite"), 0444); probeErr == nil {
		t.Skip("os.WriteFile on a 0444 file unexpectedly succeeded on this platform — skip write-failure arm")
	}

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap should propagate the os.WriteFile error (AC9) but returned nil")
	}
	// Three hard assertions that nail the defect precisely:
	// 1. The file must be in done/ — Rename succeeded; the error is downstream (status-sync write).
	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); statErr != nil {
		t.Fatalf("file not found in done/ — the error came from the gate or Rename path, not the status-sync write: %v", err)
	}
	// 2. The error must originate from the sync path (fmt.Errorf("syncing status in %s: …")).
	if !strings.Contains(err.Error(), "syncing status in") {
		t.Fatalf("error did not come from the status-sync path; got: %v", err)
	}
	// 3. The underlying cause must be EACCES (os.ErrPermission).
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected EACCES (os.ErrPermission) from os.WriteFile, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Existing-fixture compatibility check (reporting aid, not a new gate)
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_ScaffoldFixtureWouldFail_ML3A is a documentation test.
// It verifies that a roadmap produced by NewRoadmap (which contains
// "### ML-0A" with "**Status:** ⬜ Pendente") now triggers the done gate.
//
// This test EXISTS to surface the break in TestMoveRoadmap_FrontmatterSync_ValidateAfterMove
// (roadmap_test.go:183), which calls NewRoadmap then immediately moves to "done".
// The remedy is to update that test to complete or clear the scaffold MLs before
// calling MoveRoadmap(..., "done").
//
// CONCLUSION asserted: the gate fires on scaffold-generated content, confirming
// the core defect (ADR 2026-09-18 §Context) is now blocked at the transition.
func TestMoveRoadmapDone_ScaffoldFixtureWouldFail_ML3A(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	if err := NewRoadmap("Gate Test Scaffold"); err != nil {
		t.Fatalf("NewRoadmap: %v", err)
	}
	// Move backlog → wip first, just like the existing test.
	if err := MoveRoadmap("gate-test-scaffold", "wip"); err != nil {
		t.Fatalf("MoveRoadmap wip: %v", err)
	}

	err := MoveRoadmap("gate-test-scaffold", "done")
	if err == nil {
		t.Fatal("scaffold roadmap with ⬜ Pendente ML-0A should be refused at done gate, but was not — gate is not active or scaffold no longer contains pending MLs")
	}
	// Confirm it names ML-0A.
	if !strings.Contains(err.Error(), "ML-0A") {
		t.Errorf("refusal should mention ML-0A; got: %q", err.Error())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Multi-ML refusal: two pending MLs → both named
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_TwoPendingMLs_BothNamed_ML3A asserts CONCLUSION: when
// more than one ML is pending, the refusal message names every one of them
// (document order, not just the first).
func TestMoveRoadmapDone_TwoPendingMLs_BothNamed_ML3A(t *testing.T) {
	const content = "---\nstatus: wip\n---\n\n# Roadmap: two pending\n\n## Wave 1 — Two MLs\n\n### ML-1A — first\n**Status:** ⬜ Pendente\n\n### ML-1B — second\n**Status:** 🔄 Em andamento\n"
	const name = "ROADMAP-two-pending.md"

	setupMoveML(t, name, content)

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap should refuse when multiple MLs are pending, but returned nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ML-1A") {
		t.Errorf("refusal should name ML-1A; got: %q", msg)
	}
	if !strings.Contains(msg, "ML-1B") {
		t.Errorf("refusal should name ML-1B; got: %q", msg)
	}
	// Verify the file was NOT moved.
	if _, statErr := os.Stat(filepath.Join("docs", "roadmaps", "done", name)); statErr == nil {
		t.Error("roadmap was moved to done/ despite two pending MLs")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Non-done transitions are not gated
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapWip_PendingML_NotGated_ML3A asserts CONCLUSION: the pending-ML
// gate only fires for the "done" destination; moving a roadmap with pending MLs
// to "wip" or "blocked" must still succeed.
func TestMoveRoadmapWip_PendingML_NotGated_ML3A(t *testing.T) {
	const content = "---\nstatus: backlog\n---\n\n# Roadmap: wip move\n\n## Wave 1 — In Progress\n\n### ML-1A — not done\n**Status:** ⬜ Pendente\n"
	const name = "ROADMAP-wip-move.md"

	dir := t.TempDir()
	chdirADR(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Place in backlog.
	backlog := filepath.Join(dir, "docs", "roadmaps", "backlog")
	if err := os.MkdirAll(backlog, 0755); err != nil {
		t.Fatalf("mkdir backlog: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backlog, name), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := MoveRoadmap(name, "wip"); err != nil {
		t.Fatalf("Moving to wip with pending MLs should NOT be gated, but got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// AC7-bis (ML-4B, REQ #392): HasWave0 check in MoveRoadmap done transition
// ─────────────────────────────────────────────────────────────────────────────

// TestMoveRoadmapDone_AC7bis_Falsification_ML4B asserts CONCLUSION: a roadmap
// with all MLs ✅ Concluído but WITHOUT a ## Wave 0 heading is refused at the
// done transition.  This is the falsification arm: before ML-4B the check did
// not exist (grep -c HasWave0 internal/generators/roadmap.go == 0), so this
// test would have passed; after ML-4B it must fail.
func TestMoveRoadmapDone_AC7bis_Falsification_ML4B(t *testing.T) {
	const content = "---\nstatus: wip\ndate: 2026-09-18\n---\n\n# Roadmap: no-wave0 test\n\n## Wave 1 — Done\n\n### ML-1A — finished\n**Status:** ✅ Concluído\n"
	const name = "ROADMAP-no-wave0-done.md"

	setupMoveML(t, name, content)

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap(..., \"done\") must refuse when ## Wave 0 is absent (AC7-bis), but returned nil")
	}
	if !strings.Contains(err.Error(), "Wave 0") {
		t.Errorf("refusal message should reference Wave 0; got: %q", err.Error())
	}
	// File must NOT have moved.
	if _, statErr := os.Stat(filepath.Join("docs", "roadmaps", "done", name)); statErr == nil {
		t.Error("roadmap was moved to done/ despite missing Wave 0 heading — gate fired too late")
	}
}

// TestMoveRoadmapDone_AC7bis_CounterArm_ML4B asserts CONCLUSION: a roadmap with
// all MLs ✅ Concluído AND a ## Wave 0 heading IS allowed through the done
// transition.  Without this arm, the Wave 0 check could be an unconditional block.
func TestMoveRoadmapDone_AC7bis_CounterArm_ML4B(t *testing.T) {
	const content = "---\nstatus: wip\ndate: 2026-09-18\n---\n\n# Roadmap: wave0 counter-arm\n\n## Wave 0 — Threat Model\n\n## Wave 1 — Done\n\n### ML-1A — finished\n**Status:** ✅ Concluído\n"
	const name = "ROADMAP-wave0-counter-arm.md"

	dir := setupMoveML(t, name, content)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap should succeed when Wave 0 is present and all MLs are done, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); statErr != nil {
		t.Errorf("roadmap not found in done/ after successful move: %v", statErr)
	}
}

// TestMoveRoadmapDone_AC7bis_Fuga_ML4B asserts CONCLUSION: moving a roadmap from
// blocked/ (where Wave 0 is not checked by the validator) directly to done/ is
// still refused when ## Wave 0 is absent.  This closes the escape route (fuga):
// wip → remove Wave 0 → move to blocked → move to done.
// The done transition gate fires regardless of the source state.
func TestMoveRoadmapDone_AC7bis_Fuga_ML4B(t *testing.T) {
	const name = "ROADMAP-fuga-wave0.md"
	// Roadmap without Wave 0, all MLs Concluído — placed directly in blocked/.
	const content = "---\nstatus: blocked\ndate: 2026-09-18\n---\n\n# Roadmap: fuga test\n\n## Wave 1 — Done\n\n### ML-1A — finished\n**Status:** ✅ Concluído\n"

	dir := t.TempDir()
	chdirADR(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	blocked := filepath.Join(dir, "docs", "roadmaps", "blocked")
	if err := os.MkdirAll(blocked, 0755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blocked, name), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := MoveRoadmap(name, "done")
	if err == nil {
		t.Fatal("MoveRoadmap(..., \"done\") from blocked/ must refuse when Wave 0 is absent (fuga closed), but returned nil")
	}
	if !strings.Contains(err.Error(), "Wave 0") {
		t.Errorf("refusal should reference Wave 0; got: %q", err.Error())
	}
	// File must remain in blocked/.
	if _, statErr := os.Stat(filepath.Join(dir, "docs", "roadmaps", "done", name)); statErr == nil {
		t.Error("roadmap was moved to done/ from blocked/ despite missing Wave 0 — fuga not closed")
	}
}
