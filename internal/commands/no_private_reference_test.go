package commands

// no_private_reference_test.go — Camada C-commands (ML-1A, REQ-2026-11-02).
//
// Asserts that the Long help texts of newBranchPruneCmd() and newAgentModelsCmd()
// do not contain § after ML-1A removed the CLAUDE.md §1 and ADR-2026-08-21 §4
// references. These two functions expose user-visible text that is compiled into
// the binary and therefore in scope of AC2.

import (
	"strings"
	"testing"
)

// TestBranchPruneLong_NoPrivateSectionReference (Camada C-commands, site b)
// asserts that newBranchPruneCmd().Long does not contain § after ML-1A removed
// the "CLAUDE.md §1" reference from branch_prune.go:336.
//
// Reconciliation: this test asserts that the Long: help text of branch prune
// no longer cites a private document section. A revert or accidental re-introduction
// of any § in that text fails immediately.
func TestBranchPruneLong_NoPrivateSectionReference(t *testing.T) {
	cmd := newBranchPruneCmd()
	if strings.Contains(cmd.Long, "§") {
		t.Errorf("newBranchPruneCmd().Long contains § symbol — private document reference must be removed\ngot Long: %q", cmd.Long)
	}
}

// TestAgentModelsLong_NoPrivateSectionReference (Camada C-commands, site c)
// asserts that newAgentModelsCmd().Long does not contain § after ML-1A removed
// the "ADR-2026-08-21 §4" reference from agents_models.go:56.
//
// Reconciliation: this test asserts that the Long: help text of agent models
// no longer cites a private ADR section. A revert or accidental re-introduction
// of any § in that text fails immediately.
func TestAgentModelsLong_NoPrivateSectionReference(t *testing.T) {
	cmd := newAgentModelsCmd()
	if strings.Contains(cmd.Long, "§") {
		t.Errorf("newAgentModelsCmd().Long contains § symbol — private document reference must be removed\ngot Long: %q", cmd.Long)
	}
}
