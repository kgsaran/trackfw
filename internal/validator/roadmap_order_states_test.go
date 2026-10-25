package validator

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// sameStateSet returns true when a and b contain exactly the same strings
// (ignoring order, rejecting duplicates). Both directions are checked:
// every element of a is in b (a⊆b) AND every element of b is in a (b⊆a).
// A duplicate entry in a would inflate len(a) relative to the unique set,
// but even without a length difference it is detected when the missing
// element from the other side is reported.
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

// TestRoadmapFolderStates_SetEqualsCanonical verifies that roadmapFolderStates
// contains exactly the same elements as config.RoadmapStates(). The order IS
// semantic — "wip" appears first so active roadmaps are found quickly in
// validateFolderStatusCoherence and validateFilenameUniqueness — but the set
// must remain complete.
//
// Reconciliação: afirma que roadmapFolderStates tem os mesmos 6 estados que a
// lista canônica; checagem bidirecional (roadmapFolderStates⊆canonical e
// canonical⊆roadmapFolderStates) para que duplicatas ou estados ausentes sejam
// detectados. A ordem semântica (wip primeiro, prioridade de busca) é mantida
// na declaração de roadmapFolderStates (validator.go).
func TestRoadmapFolderStates_SetEqualsCanonical(t *testing.T) {
	canonical := config.RoadmapStates()
	ok, msg := sameStateSet(roadmapFolderStates, canonical)
	if !ok {
		t.Errorf("roadmapFolderStates and config.RoadmapStates() differ: %s\ngot=%v\nwant-set=%v",
			msg, roadmapFolderStates, canonical)
	}
}

// TestTraceIdStateDirs_SetEqualsCanonical verifies that traceIdStateDirs
// contains exactly the same elements as config.RoadmapStates(). The order IS
// semantic — "wip" and "done" appear first so the most recent active states are
// found first in collectTraceIdEntries, collectTraceIdEntriesByAgent and
// buildRoadmapReqLinks — but the set must remain complete.
//
// Reconciliação: afirma que traceIdStateDirs tem os mesmos 6 estados que a
// lista canônica; checagem bidirecional (traceIdStateDirs⊆canonical e
// canonical⊆traceIdStateDirs) para que duplicatas ou estados ausentes sejam
// detectados. A ordem semântica (wip/done primeiro) é mantida na declaração
// de traceIdStateDirs (validator_traceid.go).
func TestTraceIdStateDirs_SetEqualsCanonical(t *testing.T) {
	canonical := config.RoadmapStates()
	ok, msg := sameStateSet(traceIdStateDirs, canonical)
	if !ok {
		t.Errorf("traceIdStateDirs and config.RoadmapStates() differ: %s\ngot=%v\nwant-set=%v",
			msg, traceIdStateDirs, canonical)
	}
}
