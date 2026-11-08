package validator

// validator_roadmap_gates_ml1c_test.go — ML-1C (REQ de idioma, ADR-2026-11-08):
// Wave0GateDuplicate messages and bilingual header citation in validate.
//
// AC11 reconciliation (Regra Dura de Reconciliação): one sentence per test.
//
//   TestValidateRoadmapGateCoverage_D3_DuplicateHeadersWave0
//     Affirms: a wip roadmap whose Wave 0 contains two valid gate headers outside fences
//     triggers roadmap_gate_coverage with a message that contains "duplicate" and names
//     both line numbers — not the "is not followed by a fence" message that Wave0GateMalformed
//     would emit (this is the specific regression that ML-1C fixes).
//
//   TestValidateRoadmapGateCoverage_MalformedMessage_CitesEnAndPt
//     Affirms: a wip roadmap whose Wave 0 gate header has no ```bash fence before the next
//     heading emits a roadmap_gate_coverage message that cites both '**Wave gates:**' and
//     '**Gates da wave:**' — the bilingual update of ML-1C.
//
//   TestValidateRoadmapGateCoverage_AbsentMessage_CitesEnAndPt
//     Affirms: a wip roadmap whose Wave 0 has no gate header at all emits a
//     roadmap_gate_coverage message that cites both '**Wave gates:**' and
//     '**Gates da wave:**'.

import (
	"os"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// roadmapWithDuplicateGateHeaders is a wip roadmap with two gate headers in Wave 0.
// Line counting (1-based, after SplitRoadmapLines which splits on \n):
//
//	 1: ---
//	 2: status: wip
//	 3: ---
//	 4: (blank)
//	 5: # Roadmap: Duplicate Gates Test
//	 6: (blank)
//	 7: ## Wave 0 — Threat Model
//	 8: (blank)
//	 9: ### ML-0A — work
//	10: **Status:** ✅ Concluído
//	11: (blank)
//	12: **Gates da wave:**
//	13: ```bash
//	14: bash scripts/check.sh
//	15: ```
//	16: (blank)
//	17: **Wave gates:**
//	18: ```bash
//	19: bash scripts/other.sh
//	20: ```
//	21: (blank)
//
// ParseGatesLines receives [waveStart, waveEnd) relative to the document lines.
// Wave 0 starts at line 7 (0-based: index 6), so the first gate header
// "**Gates da wave:**" is at document line 12 (0-based: index 11), i.e. 1-based = 12.
// The second gate header "**Wave gates:**" is at document line 17 (0-based: index 16),
// i.e. 1-based = 17.
// The D3 error must name lines 12 and 17 (relative to the whole document).
// Wave start in ParseWaves is the index of the "## Wave 0" line, so waveStart = 6 (0-based).
// Inside ParseGatesLines: validHeaderLines[0]+1 = (11)+1 = 12; validHeaderLines[1]+1 = (16)+1 = 17.
const roadmapWithDuplicateGateHeaders = `---
status: wip
---

# Roadmap: Duplicate Gates Test

## Wave 0 — Threat Model

### ML-0A — work
**Status:** ✅ Concluído

**Gates da wave:**
` + "```bash\n" + `bash scripts/check.sh
` + "```\n" + `
**Wave gates:**
` + "```bash\n" + `bash scripts/other.sh
` + "```\n"

// roadmapWithGateHeaderNoFence is a wip roadmap whose Wave 0 gate header has no
// ```bash fence before the next heading.
const roadmapWithGateHeaderNoFence = `---
status: wip
---

# Roadmap: Malformed Gate Test

## Wave 0 — Threat Model

### ML-0A — work
**Status:** ✅ Concluído

**Wave gates:**

> Some prose but no fence follows.

`

func TestValidateRoadmapGateCoverage_D3_DuplicateHeadersWave0(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/analyzing", "docs/req", "docs/adr")
	if err := os.WriteFile(dir+"/trackfw.yaml", []byte(minimalTrackfwYaml), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.WriteFile(dir+"/docs/roadmaps/wip/ROADMAP-duplicate-gates.md", []byte(roadmapWithDuplicateGateHeaders), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	// Find the gate coverage message in both slices.
	all := append(violations, warnings...)
	var found string
	for _, msg := range all {
		if strings.Contains(msg, "ROADMAP-duplicate-gates") && strings.Contains(msg, "duplicate gates header") {
			found = msg
			break
		}
	}
	if found == "" {
		t.Fatalf("expected roadmap_gate_coverage violation with 'duplicate gates header' for D3 fixture, got violations=%v warnings=%v", violations, warnings)
	}
	// The message must name the two line numbers from the parser error.
	if !strings.Contains(found, "lines") {
		t.Errorf("D3 message should name line numbers, got: %q", found)
	}
	// Regression guard: the old Wave0GateMalformed message must NOT appear for this case.
	for _, msg := range all {
		if strings.Contains(msg, "ROADMAP-duplicate-gates") && strings.Contains(msg, "is not followed by") {
			t.Errorf("D3 case must not emit the malformed-fence message, got: %q", msg)
		}
	}
}

func TestValidateRoadmapGateCoverage_MalformedMessage_CitesEnAndPt(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/analyzing", "docs/req", "docs/adr")
	if err := os.WriteFile(dir+"/trackfw.yaml", []byte(minimalTrackfwYaml), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.WriteFile(dir+"/docs/roadmaps/wip/ROADMAP-malformed-gate.md", []byte(roadmapWithGateHeaderNoFence), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	all := append(violations, warnings...)
	var found string
	for _, msg := range all {
		if strings.Contains(msg, "ROADMAP-malformed-gate") && strings.Contains(msg, "not followed by") {
			found = msg
			break
		}
	}
	if found == "" {
		t.Fatalf("expected roadmap_gate_coverage violation for malformed gate header, got violations=%v warnings=%v", violations, warnings)
	}
	if !strings.Contains(found, "**Wave gates:**") {
		t.Errorf("malformed message must cite '**Wave gates:**' (EN form), got: %q", found)
	}
	if !strings.Contains(found, "**Gates da wave:**") {
		t.Errorf("malformed message must cite '**Gates da wave:**' (PT form), got: %q", found)
	}
}

func TestValidateRoadmapGateCoverage_AbsentMessage_CitesEnAndPt(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/analyzing", "docs/req", "docs/adr")
	if err := os.WriteFile(dir+"/trackfw.yaml", []byte(minimalTrackfwYaml), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.WriteFile(dir+"/docs/roadmaps/wip/ROADMAP-absent-gate.md", []byte(roadmapWithWave0NoGateBlock), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	all := append(violations, warnings...)
	var found string
	for _, msg := range all {
		if strings.Contains(msg, "ROADMAP-absent-gate") && strings.Contains(msg, "has no gate block") {
			found = msg
			break
		}
	}
	if found == "" {
		t.Fatalf("expected roadmap_gate_coverage violation for absent gate block, got violations=%v warnings=%v", violations, warnings)
	}
	if !strings.Contains(found, "**Wave gates:**") {
		t.Errorf("absent message must cite '**Wave gates:**' (EN form), got: %q", found)
	}
	if !strings.Contains(found, "**Gates da wave:**") {
		t.Errorf("absent message must cite '**Gates da wave:**' (PT form), got: %q", found)
	}
}
