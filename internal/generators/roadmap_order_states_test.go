package generators

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// sameStateSet returns true when a and b contain exactly the same strings (ignoring order).
// Both directions are checked: every element of a is in b, and every element of b is in a.
func sameStateSet(a, b []string) (bool, string) {
	if len(a) != len(b) {
		return false, "length mismatch"
	}
	aset := make(map[string]bool, len(a))
	for _, s := range a {
		aset[s] = true
	}
	bset := make(map[string]bool, len(b))
	for _, s := range b {
		bset[s] = true
	}
	for _, s := range b {
		if !aset[s] {
			return false, s + " in b but not in a"
		}
	}
	for _, s := range a {
		if !bset[s] {
			return false, s + " in a but not in b"
		}
	}
	return true, ""
}

// TestRoadmapStateOrder_SetEqualsCanonical verifies that roadmapStateOrder
// contains exactly the same elements as config.RoadmapStates(). The order
// IS semantic — "analyzing" and "wip" appear first so roadmap searches
// prioritise the most active states — but the set must remain complete.
//
// Reconciliação: afirma que roadmapStateOrder tem os mesmos 6 estados que a
// lista canônica (checagem bidirecional: a ∈ b e b ∈ a); a ordem semântica é
// mantida na declaração de roadmapStateOrder.
func TestRoadmapStateOrder_SetEqualsCanonical(t *testing.T) {
	canonical := config.RoadmapStates()
	ok, msg := sameStateSet(roadmapStateOrder, canonical)
	if !ok {
		t.Errorf("roadmapStateOrder and config.RoadmapStates() differ: %s\ngot=%v\nwant-set=%v",
			msg, roadmapStateOrder, canonical)
	}
}

// TestContextRoadmapStateOrder_SetEqualsCanonical verifies that contextRoadmapStateOrder
// contains exactly the same elements as config.RoadmapStates(). The order is semantic
// (wip first) but the set must stay complete, including "analyzing" added in ML-6D.
//
// Reconciliação: afirma que contextRoadmapStateOrder tem os mesmos 6 estados que a lista
// canônica (checagem bidirecional), garantindo que nenhum estado seja omitido silenciosamente
// do `trackfw context` — o defeito que motivou ML-6D (analyzing/ era omitido antes).
func TestContextRoadmapStateOrder_SetEqualsCanonical(t *testing.T) {
	canonical := config.RoadmapStates()
	ok, msg := sameStateSet(contextRoadmapStateOrder, canonical)
	if !ok {
		t.Errorf("contextRoadmapStateOrder and config.RoadmapStates() differ: %s\ngot=%v\nwant-set=%v",
			msg, contextRoadmapStateOrder, canonical)
	}
}
