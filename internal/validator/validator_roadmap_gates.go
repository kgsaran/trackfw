package validator

// validator_roadmap_gates.go — AC7, AC7-bis, AC8 (REQ #392, ML-3B/ML-4B) and
// roadmap_unterminated_fence (REQ #476, ML-3B).
//
// Four rules, state-sensitive:
//
//   roadmap_wave0_required      (AC7-bis) — roadmap in wip/ must have ## Wave 0.
//   roadmap_gate_coverage       (AC7)     — Wave 0 gate must be real (not placeholder, not absent).
//   roadmap_duplicate_label     (AC8)     — no duplicate Wave or ML labels.
//   roadmap_unterminated_fence  (#476)    — no unterminated code fence in any state.
//
// roadmap_wave0_required and roadmap_gate_coverage apply to wip/ only.
// roadmap_duplicate_label applies to wip and blocked (duplicate labels are ambiguous
// regardless of convention age; blocked/ can still have structural errors).
// roadmap_unterminated_fence applies to ALL states (backlog, analyzing, wip, blocked,
// done, abandoned): an unterminated fence masks content regardless of workflow state.
//
// None of wave0_required/gate_coverage applies to backlog/analyzing (ADR-2026-09-18
// decision 6-bis: 6 backlog roadmaps legitimately carry the scaffold placeholder).
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
// Default severity: "error" for all four (absent from ruleDefaults — falls through to error).
// All pre-existing violations of the first three were cleaned by ML-4A/ML-4B (REQ #392).
// The two pre-existing unterminated-fence files were closed by ML-1A (REQ #476).

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
// for the three gate-coverage rules.  Returns four slices: wave0Msgs, wave0ExemptNotice,
// gateMsgs, dupMsgs. Each is independently routed through applyRule/applyRuleTagged by the
// caller; wave0ExemptNotice is routed through applyRuleWarnOnly.
//
// State coverage per rule:
//
//	roadmap_wave0_required  → wip/ only  (see header comment)
//	roadmap_gate_coverage   → wip/ only  (see header comment)
//	roadmap_duplicate_label → wip/ + blocked/
//
// D5 (ADR-2026-10-04, REQ #514 ML-1B): roadmaps dated strictly before
// roadmapWave0Cutoff (2026-09-18) are exempt from roadmap_wave0_required.
// Isenção visível: counted in a single wave0ExemptNotice string.
//
// The function never returns a non-nil error: read errors become diagnostic messages
// inside the relevant slice (following the pattern of other validator functions here).
func validateRoadmapGatesCoverage() (wave0Msgs []string, wave0ExemptNotice []string, gateMsgs []string, dupMsgs []string) {
	cfg := config.Load()
	wave0ExemptCount := 0

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

			// AC7-bis: Wave 0 heading must exist in wip — unless the roadmap predates
			// the Wave 0 requirement (D5, ADR-2026-10-04: cutoff 2026-09-18).
			if !roadmapdoc.HasWave0(data) {
				if d, ok := RoadmapCreationDate(data, path); ok && d.Before(RoadmapWave0CutoffDate()) {
					// Pre-cutoff: exempt, count for the aggregated notice.
					wave0ExemptCount++
				} else {
					wave0Msgs = append(wave0Msgs, fmt.Sprintf(
						"roadmap %q (wip) has no ## Wave 0 heading; wip roadmaps must have a Wave 0 threat-model section (ADR-2026-09-18 decision 8)",
						base,
					))
				}
			}

			// AC7: Wave 0 gate must be real — not a placeholder, not absent.
			// If Wave 0 is absent the gate predicate returns false (AC7-bis handles it above).
			// #460: a causa vem junto, porque as tres levam a acoes OPOSTAS —
			// reescrever o comando, escrever o bloco, ou mover a cerca. A mensagem
			// antiga nomeava duas causas e instruia sobre uma so.
			switch roadmapdoc.Wave0GateDiagnosis(data) {
			case roadmapdoc.Wave0GatePlaceholder:
				gateMsgs = append(gateMsgs, fmt.Sprintf(
					"roadmap %q (wip) Wave 0 gate is still the placeholder; replace the exit 1 command with a real gate (AC7, ADR-2026-09-18 decision 4)",
					base,
				))
			case roadmapdoc.Wave0GateAbsent:
				gateMsgs = append(gateMsgs, fmt.Sprintf(
					"roadmap %q (wip) Wave 0 has no gate block; add '**Gates da wave:**' followed by a ```bash fence with a real gate command (AC7, ADR-2026-09-18 decision 4)",
					base,
				))
			case roadmapdoc.Wave0GateMalformed:
				gateMsgs = append(gateMsgs, fmt.Sprintf(
					"roadmap %q (wip) Wave 0 gate block was not found: '**Gates da wave:**' is not followed by a ```bash fence before the next heading (AC7, ADR-2026-09-18 decision 4)",
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

	// D5: emit the aggregated exemption notice if any wip roadmaps were exempted.
	if wave0ExemptCount > 0 {
		wave0ExemptNotice = []string{roadmapWave0ExemptNotice(wave0ExemptCount)}
	}
	return
}

// validateRoadmapUnterminatedFence checks ALL roadmap states for unterminated code fences.
// An unterminated fence silently masks content to the end of the file, which can hide
// governance-significant lines (ML status, acceptance criteria, gate blocks). Unlike the
// other three rules in this file, coverage is intentionally universal: a masking defect is
// equally harmful in done/, backlog/ or wip/.
//
// Returns one message per offending file, in the form
//
//	"<basename>: unterminated code fence starting at line <n>"
//
// which is the canonical format shared by all surfaces (barrier, roadmap show, serve, validate).
// In governance_mode: lenient the caller routes it through applyRule, which demotes to warning —
// no special handling needed here.
func validateRoadmapUnterminatedFence() []string {
	cfg := config.Load()
	allStates := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}

	var msgs []string
	for _, state := range allStates {
		for _, dir := range resolveStateDirs(cfg, state) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				if !os.IsNotExist(err) {
					msgs = append(msgs, inspectionDiagnostic("roadmap_unterminated_fence", dir, err))
				}
				continue
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				path := filepath.Join(dir, e.Name())
				rawBytes, ok := readFileForRule("roadmap_unterminated_fence", path, &msgs)
				if !ok {
					continue
				}
				lines := strings.Split(string(rawBytes), "\n")
				if _, err := roadmapdoc.FenceMaskCheck(lines); err != nil {
					msgs = append(msgs, fmt.Sprintf("%s: %s", e.Name(), err.Error()))
				}
			}
		}
	}

	sort.Strings(msgs)
	return msgs
}
