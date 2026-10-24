package config

import (
	"testing"
)

// TestRoadmapStates_ExactLifecycleOrder verifies that RoadmapStates() returns
// exactly the six canonical roadmap state names in lifecycle order.
//
// The order is contract: it appears in `trackfw status` output (via inventoryBlock)
// and governs all per-state directory scans. Changing it changes what users see.
//
// Reconciliação: afirma que a lista canônica tem os 6 estados na ordem do ciclo
// de vida (backlog→analyzing→wip→blocked→done→abandoned) — a ordem é contrato
// porque aparece na saída do `trackfw status` (inventoryBlock).
func TestRoadmapStates_ExactLifecycleOrder(t *testing.T) {
	want := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}

	got := RoadmapStates()
	if len(got) != len(want) {
		t.Fatalf("RoadmapStates() returned %d elements, want %d: %v", len(got), len(want), got)
	}
	for i, s := range got {
		if s != want[i] {
			t.Errorf("RoadmapStates()[%d] = %q, want %q (lifecycle order is contract)", i, s, want[i])
		}
	}
}

// TestRoadmapStates_ReturnsCopy verifies that mutating the returned slice does
// not affect subsequent calls.
//
// Reconciliação: afirma que RoadmapStates() devolve uma cópia independente —
// nenhum sítio consumidor pode mutar a lista compartilhada por acidente.
func TestRoadmapStates_ReturnsCopy(t *testing.T) {
	first := RoadmapStates()
	first[0] = "MUTATED"

	second := RoadmapStates()
	for _, s := range second {
		if s == "MUTATED" {
			t.Error("RoadmapStates() returned the same slice on consecutive calls (mutation leaked)")
		}
	}
}
