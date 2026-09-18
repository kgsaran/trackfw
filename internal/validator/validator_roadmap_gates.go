package validator

// validator_roadmap_gates.go — AC7, AC7-bis, AC8 (REQ #392, ML-3B/ML-4B).
//
// Three rules, state-sensitive:
//
//   roadmap_wave0_required  (AC7-bis) — roadmap in wip/ must have ## Wave 0.
//   roadmap_gate_coverage   (AC7)     — Wave 0 gate must be real (not placeholder, not absent).
//   roadmap_duplicate_label (AC8)     — no duplicate Wave or ML labels.
//
// roadmap_wave0_required and roadmap_gate_coverage apply to wip/ only.
// roadmap_duplicate_label applies to wip and blocked (duplicate labels are ambiguous
// regardless of convention age; blocked/ can still have structural errors).
//
// None of the three applies to backlog/analyzing (ADR-2026-09-18 decision 6-bis:
// 6 backlog roadmaps legitimately carry the scaffold placeholder).
// None applies retroactively to done/ (decision 7/8: 154 of 192 done/ roadmaps lack
// Wave 0; applying the rule there would produce 154 violations, worse than the 27
// ML-pending violations that decision 7 rejected for the same reason).
//
// Why wave0_required and gate_coverage do NOT cover blocked/ (ML-4B, REQ #392):
// "blocked" means work paused — possibly before Wave 0 was required or before the
// gate convention existed (ADR-2026-09-18 postdates those roadmaps).  Measured:
// 1 roadmap in blocked/ (fechar-os-grupos-de-falha-de-windows) would fire immediately
// with the Wave 0 gates block absent.  Same retroactivity argument that excludes done/.
// The asymmetry is deliberate; it is written here so it is not "fixed" later.
//
// Default severity: "error" for all three (absent from ruleDefaults — falls through to error).
// All pre-existing violations were cleaned by ML-4A/ML-4B (REQ #392).

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/roadmapdoc"
)

// validateRoadmapGatesCoverage checks roadmaps in wip/ (and blocked/ for duplicate labels)
// for the three gate-coverage rules.  Returns three slices: wave0Msgs, gateMsgs, dupMsgs.
// Each is independently routed through applyRule/applyRuleTagged by the caller.
//
// State coverage per rule:
//   roadmap_wave0_required  → wip/ only  (see header comment)
//   roadmap_gate_coverage   → wip/ only  (see header comment)
//   roadmap_duplicate_label → wip/ + blocked/
//
// The function never returns a non-nil error: read errors become diagnostic messages
// inside the relevant slice (following the pattern of other validator functions here).
func validateRoadmapGatesCoverage() (wave0Msgs []string, gateMsgs []string, dupMsgs []string) {
	cfg := config.Load()

	// roadmap_wave0_required and roadmap_gate_coverage: wip/ only.
	for _, dir := range resolveStateDirs(cfg, "wip") {
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

			// AC7-bis: Wave 0 heading must exist in wip.
			if !roadmapdoc.HasWave0(data) {
				wave0Msgs = append(wave0Msgs, fmt.Sprintf(
					"roadmap %q (wip) has no ## Wave 0 heading; wip roadmaps must have a Wave 0 threat-model section (ADR-2026-09-18 decision 8)",
					base,
				))
			}

			// AC7: Wave 0 gate must be real — not a placeholder, not absent.
			// If Wave 0 is absent the gate predicate returns false (AC7-bis handles it above).
			if roadmapdoc.Wave0HasPlaceholderOrMissingGate(data) {
				gateMsgs = append(gateMsgs, fmt.Sprintf(
					"roadmap %q (wip) Wave 0 gate is placeholder or absent; replace the exit 1 placeholder with a real gate command (AC7, ADR-2026-09-18 decision 4)",
					base,
				))
			}
		}
	}

	// roadmap_duplicate_label: wip/ + blocked/ (structural error, not convention-age-sensitive).
	for _, state := range []string{"wip", "blocked"} {
		for _, dir := range resolveStateDirs(cfg, state) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				if !os.IsNotExist(err) {
					dupMsgs = append(dupMsgs, inspectionDiagnostic("roadmap_duplicate_label", dir, err))
				}
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				path := filepath.Join(dir, e.Name())
				rawBytes, ok := readFileForRule("roadmap_duplicate_label", path, &dupMsgs)
				if !ok {
					continue
				}
				data := string(rawBytes)
				base := e.Name()

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
