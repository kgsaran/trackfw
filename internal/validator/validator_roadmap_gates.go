package validator

// validator_roadmap_gates.go — AC7, AC7-bis, AC8 (REQ #392, ML-3B).
//
// Three rules, state-sensitive:
//
//   roadmap_wave0_required  (AC7-bis) — roadmap in wip/blocked must have ## Wave 0.
//   roadmap_gate_coverage   (AC7)     — Wave 0 gate must be real (not placeholder, not absent).
//   roadmap_duplicate_label (AC8)     — no duplicate Wave or ML labels.
//
// All three apply to wip and blocked.  None applies to backlog/analyzing (ADR-2026-09-18
// decision 6-bis: 6 backlog roadmaps legitimately carry the scaffold placeholder).
// None applies retroactively to done/ (decision 7/8: 154 of 192 done/ roadmaps lack
// Wave 0; applying the rule there would produce 154 violations, worse than the 27
// ML-pending violations that decision 7 rejected for the same reason).
// The asymmetry is deliberate; it is written here so it is not "fixed" later.
//
// Default severity: "warning" for all three (see ruleDefaults in validator.go).
// Rationale: the three blocked/ roadmaps that already violate these rules are pre-existing;
// promoting them to "error" would break trackfw validate on the current corpus before ML-4A
// cleans them (AC10).  Operators can promote to "error" via rules: in trackfw.yaml.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/roadmapdoc"
)

// validateRoadmapGatesCoverage checks every roadmap in wip/ and blocked/ for the three
// gate-coverage rules.  Returns three slices: wave0Msgs, gateMsgs, dupMsgs.
// Each is independently routed through applyRule/applyRuleTagged by the caller.
//
// The function never returns a non-nil error: read errors become diagnostic messages
// inside the relevant slice (following the pattern of other validator functions here).
func validateRoadmapGatesCoverage() (wave0Msgs []string, gateMsgs []string, dupMsgs []string) {
	cfg := config.Load()
	for _, state := range []string{"wip", "blocked"} {
		for _, dir := range resolveStateDirs(cfg, state) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				if !os.IsNotExist(err) {
					gateMsgs = append(gateMsgs, inspectionDiagnostic("roadmap_gate_coverage", dir, err))
				}
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				path := filepath.Join(dir, e.Name())
				rawBytes, ok := readFileForRule("roadmap_gate_coverage", path, &gateMsgs)
				if !ok {
					continue
				}
				data := string(rawBytes)
				base := e.Name()

				// AC7-bis: Wave 0 heading must exist.
				if !roadmapdoc.HasWave0(data) {
					wave0Msgs = append(wave0Msgs, fmt.Sprintf(
						"roadmap %q (%s) has no ## Wave 0 heading; wip/blocked roadmaps must have a Wave 0 threat-model section (ADR-2026-09-18 decision 8)",
						base, state,
					))
				}

				// AC7: Wave 0 gate must be real — not a placeholder, not absent.
				// If Wave 0 is absent the gate predicate returns false (AC7-bis handles it above).
				if roadmapdoc.Wave0HasPlaceholderOrMissingGate(data) {
					gateMsgs = append(gateMsgs, fmt.Sprintf(
						"roadmap %q (%s) Wave 0 gate is placeholder or absent; replace the exit 1 placeholder with a real gate command (AC7, ADR-2026-09-18 decision 4)",
						base, state,
					))
				}

				// AC8: no duplicate Wave or ML labels.
				for _, msg := range roadmapdoc.DuplicateWaveOrMLLabels(data) {
					dupMsgs = append(dupMsgs, fmt.Sprintf("roadmap %q (%s): %s", base, state, msg))
				}
			}
		}
	}
	sort.Strings(wave0Msgs)
	sort.Strings(gateMsgs)
	sort.Strings(dupMsgs)
	return
}
