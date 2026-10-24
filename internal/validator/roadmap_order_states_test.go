package validator

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestRoadmapFolderStates_SetEqualsCanonical verifies that roadmapFolderStates
// contains exactly the same elements as config.RoadmapStates(). The order IS
// semantic — "wip" appears first so active roadmaps are found quickly in
// validateFolderStatusCoherence and validateFilenameUniqueness — but the set
// must remain complete.
//
// Reconciliação: afirma que roadmapFolderStates tem os mesmos 6 estados que a
// lista canônica; a ordem semântica (wip primeiro, prioridade de busca) é mantida
// na declaração de roadmapFolderStates (validator.go).
func TestRoadmapFolderStates_SetEqualsCanonical(t *testing.T) {
	canonical := config.RoadmapStates()
	if len(roadmapFolderStates) != len(canonical) {
		t.Fatalf("roadmapFolderStates has %d elements, canonical has %d: got=%v want-set=%v",
			len(roadmapFolderStates), len(canonical), roadmapFolderStates, canonical)
	}
	cset := make(map[string]bool, len(canonical))
	for _, s := range canonical {
		cset[s] = true
	}
	for _, s := range roadmapFolderStates {
		if !cset[s] {
			t.Errorf("roadmapFolderStates contains %q which is not in config.RoadmapStates()", s)
		}
	}
}

// TestTraceIdStateDirs_SetEqualsCanonical verifies that traceIdStateDirs
// contains exactly the same elements as config.RoadmapStates(). The order IS
// semantic — "wip" and "done" appear first so the most recent active states are
// found first in collectTraceIdEntries, collectTraceIdEntriesByAgent and
// buildRoadmapReqLinks — but the set must remain complete.
//
// Reconciliação: afirma que traceIdStateDirs tem os mesmos 6 estados que a
// lista canônica; a ordem semântica (wip/done primeiro) é mantida na declaração
// de traceIdStateDirs (validator_traceid.go).
func TestTraceIdStateDirs_SetEqualsCanonical(t *testing.T) {
	canonical := config.RoadmapStates()
	if len(traceIdStateDirs) != len(canonical) {
		t.Fatalf("traceIdStateDirs has %d elements, canonical has %d: got=%v want-set=%v",
			len(traceIdStateDirs), len(canonical), traceIdStateDirs, canonical)
	}
	cset := make(map[string]bool, len(canonical))
	for _, s := range canonical {
		cset[s] = true
	}
	for _, s := range traceIdStateDirs {
		if !cset[s] {
			t.Errorf("traceIdStateDirs contains %q which is not in config.RoadmapStates()", s)
		}
	}
}
