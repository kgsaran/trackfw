package commands

// barrier_lapsed_test.go — end-to-end tests for lapsed acceptance criteria
// (D2/D3/D6/T8, REQ #514 / ML-1A).
//
// These tests drive the real compiled binary (same approach as barrier_contract_test.go).
// Each test asserts one rule and includes the reconciliation sentence required by the
// "Regra Dura de Reconciliação" (CLAUDE.md).

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// helper: build a roadmap with two MLs, each with custom criteria lines
// ─────────────────────────────────────────────────────────────────────────────

// buildLapsedRoadmap builds a minimal two-ML roadmap:
//   - ML-1A has criteriaA lines
//   - ML-1B has criteriaB lines
//
// Both MLs are marked "✅ Concluído". The roadmap has a Wave 0 and a Wave 1.
func buildLapsedRoadmap(criteriaA, criteriaB []string) string {
	var b strings.Builder
	b.WriteString("# Roadmap: Lapsed Fixture\n\n")
	b.WriteString("REQ: " + barrierFixtureREQRel + "\n\n")
	b.WriteString("## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n")
	b.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-0A — Threat model for fixture\n")
	b.WriteString("**Status:** ✅ Concluído\n\n")
	b.WriteString("**Gates da wave:**\n```bash\n")
	b.WriteString("echo \"wave 0 accepted\"\n")
	b.WriteString("```\n\n")

	b.WriteString("## Wave 1 — Fixture Wave\n> Dependências: nenhuma.\n\n")
	b.WriteString("### ML-1A — Fixture A\n")
	b.WriteString("**Status:** ✅ Concluído\n")
	b.WriteString("**Critérios de aceite:**\n")
	for _, line := range criteriaA {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString("### ML-1B — Fixture B\n")
	b.WriteString("**Status:** ✅ Concluído\n")
	b.WriteString("**Critérios de aceite:**\n")
	for _, line := range criteriaB {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// setupLapsedFixture writes the fixture to a temp dir and returns (dir, roadmapPath).
func setupLapsedFixture(t *testing.T, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(dir+"/"+d, 0755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
	roadmapRel := "docs/roadmaps/wip/ROADMAP-lapsed-fixture.md"
	if err := os.WriteFile(dir+"/"+roadmapRel, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}
	writeBarrierREQFixture(t, dir, roadmapRel)
	return dir, dir + "/" + roadmapRel
}

// ─────────────────────────────────────────────────────────────────────────────
// D3: lapsed does not block when at least one criterion is met
// ─────────────────────────────────────────────────────────────────────────────

// TestBarrierLapsed_PassedWhenMetAndLapsed asserts that the barrier reports
// acceptance_evidence=passed when ML-1A has one "[x]" criterion and one "[ ]" with a valid
// Caducou: continuation — lapsed criteria do not block a wave that has real evidence (D3).
func TestBarrierLapsed_PassedWhenMetAndLapsed(t *testing.T) {
	criteriaA := []string{
		"- [x] build passes",
		"- [ ] suíte pypi sem regressão",
		"  Caducou: a v8 removeu o CLI Python (#365)",
	}
	criteriaB := []string{"- [x] tests pass"}
	content := buildLapsedRoadmap(criteriaA, criteriaB)
	dir, _ := setupLapsedFixture(t, content)

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--json", "--trust-local-gates")
	if code != 0 {
		t.Fatalf("expected exit 0 (passed), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}

	for _, c := range doc.Checks {
		if c.Name != "acceptance_evidence" {
			continue
		}
		if c.Status != "passed" {
			t.Fatalf("expected acceptance_evidence=passed, got %q (failures: %v)", c.Status, c.Failures)
		}
		// Evidence must mention the met criterion count.
		foundMet := false
		for _, e := range c.Evidence {
			if strings.Contains(e, "ML-1A") && strings.Contains(e, "1 criteria met") {
				foundMet = true
			}
		}
		if !foundMet {
			t.Fatalf("expected evidence to contain 'ML-1A: 1 criteria met', got %v", c.Evidence)
		}
		// Lapsed must be reported in the new lapsed field.
		foundLapsed := false
		for _, l := range c.Lapsed {
			if strings.Contains(l, "ML-1A") && strings.Contains(l, "lapsed") {
				foundLapsed = true
			}
		}
		if !foundLapsed {
			t.Fatalf("expected lapsed field to contain 'ML-1A: ... lapsed', got %v", c.Lapsed)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// D6/T3: unrecognized checkbox blocked + "unrecognized checkbox at line N"
// ─────────────────────────────────────────────────────────────────────────────

// TestBarrierLapsed_UnrecognizedCheckboxBlocked asserts that the barrier reports
// acceptance_evidence=blocked when ML-1B contains "- [~]" — unrecognized checkbox
// characters count as pending (D6/T3), and the barrier names the line.
func TestBarrierLapsed_UnrecognizedCheckboxBlocked(t *testing.T) {
	criteriaA := []string{"- [x] build passes"}
	criteriaB := []string{"- [~] partial work done"}
	content := buildLapsedRoadmap(criteriaA, criteriaB)
	dir, _ := setupLapsedFixture(t, content)

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--json", "--trust-local-gates")
	if code != 1 {
		t.Fatalf("expected exit 1 (blocked — [~] must count as pending), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}

	for _, c := range doc.Checks {
		if c.Name != "acceptance_evidence" {
			continue
		}
		if c.Status != "blocked" {
			t.Fatalf("expected acceptance_evidence=blocked, got %q", c.Status)
		}
		// Must have "N unmet acceptance criteria" failure for ML-1B.
		foundUnmet := false
		for _, f := range c.Failures {
			if strings.Contains(f, "ML-1B") && strings.Contains(f, "unmet acceptance criteria") {
				foundUnmet = true
			}
		}
		if !foundUnmet {
			t.Fatalf("expected failure 'ML-1B: N unmet acceptance criteria', got %v", c.Failures)
		}
		// Must also have "unrecognized checkbox at line N" for ML-1B.
		foundUnrecognized := false
		for _, f := range c.Failures {
			if strings.Contains(f, "ML-1B") && strings.Contains(f, "unrecognized checkbox at line") {
				foundUnrecognized = true
			}
		}
		if !foundUnrecognized {
			t.Fatalf("expected failure 'ML-1B: unrecognized checkbox at line N', got %v", c.Failures)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// D6/T8: all-lapsed ML is blocked
// ─────────────────────────────────────────────────────────────────────────────

// TestBarrierLapsed_TextModePrintsLapsedLine asserts that the text-mode output
// (without --json) contains "~ ML-1A: N lapsed acceptance criteria" for an ML with
// one met criterion and one lapsed criterion (D3: "Mostra à parte, no texto e no JSON").
func TestBarrierLapsed_TextModePrintsLapsedLine(t *testing.T) {
	criteriaA := []string{
		"- [x] build passes",
		"- [ ] suíte pypi",
		"  Caducou: v8 removeu o CLI Python",
	}
	criteriaB := []string{"- [x] tests pass"}
	content := buildLapsedRoadmap(criteriaA, criteriaB)
	dir, _ := setupLapsedFixture(t, content)

	// No --json flag: text mode output.
	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--trust-local-gates")
	if code != 0 {
		t.Fatalf("expected exit 0 (passed), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// The text output must contain the tilde prefix for the lapsed message.
	wantLine := "~ ML-1A: 1 lapsed acceptance criteria"
	if !strings.Contains(stdout, wantLine) {
		t.Fatalf("expected text output to contain %q\nstdout: %s", wantLine, stdout)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ML-1D: justification text appears in text output and JSON (Ajuste 1, Wave 2)
// ─────────────────────────────────────────────────────────────────────────────

// TestBarrierLapsed_JustificationInTextAndJSON asserts that the barrier prints the
// Caducou: justification text in both the text-mode output (indented "line N: Caducou:
// <text>") and in the JSON lapsed_details field (ML-1D, REQ #514).
//
// Reconciliation: this test asserts that the Ajuste 1 gap (Wave 2 security review) is
// closed — the reviewer can see the lapsed justification without opening the roadmap.
func TestBarrierLapsed_JustificationInTextAndJSON(t *testing.T) {
	const justification = "a v8 removeu o CLI Python (#365)"
	criteriaA := []string{
		"- [x] build passes",
		"- [ ] suíte pypi sem regressão",
		"  Caducou: " + justification,
	}
	criteriaB := []string{"- [x] tests pass"}
	content := buildLapsedRoadmap(criteriaA, criteriaB)
	dir, _ := setupLapsedFixture(t, content)

	// ── text mode: justification must appear as "line N: Caducou: <text>" ────
	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--trust-local-gates")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	wantFragment := "Caducou: " + justification
	if !strings.Contains(stdout, wantFragment) {
		t.Fatalf("text output must contain justification %q\nstdout: %s", wantFragment, stdout)
	}
	// The line must be indented with "      line N:" prefix.
	if !strings.Contains(stdout, "      line ") {
		t.Fatalf("text output must contain '      line N: Caducou: ...' indented block\nstdout: %s", stdout)
	}

	// ── JSON mode: lapsed_details must contain line and text ─────────────────
	jsonOut, _, code2 := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--json", "--trust-local-gates")
	if code2 != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s", code2, jsonOut)
	}
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonOut)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, jsonOut)
	}
	for _, c := range doc.Checks {
		if c.Name != "acceptance_evidence" {
			continue
		}
		if len(c.LapsedDetails) == 0 {
			t.Fatalf("expected lapsed_details to be populated, got empty\nJSON: %s", jsonOut)
		}
		found := false
		for _, d := range c.LapsedDetails {
			if d.Line > 0 && strings.Contains(d.Text, justification) {
				found = true
			}
		}
		if !found {
			t.Fatalf("lapsed_details must contain {line>0, text containing %q}, got %+v", justification, c.LapsedDetails)
		}
	}
}

// TestBarrierLapsed_JustificationTruncatedAt120 asserts that a Caducou: justification
// longer than 120 runes is truncated with "…" in both text and JSON output (ML-1D, REQ #514).
//
// Reconciliation: this test asserts that the 120-rune limit in extractLapsedReason prevents
// excessively long justifications from polluting the barrier output.
func TestBarrierLapsed_JustificationTruncatedAt120(t *testing.T) {
	// Build a justification of exactly 125 runes (first 120 visible + 5 cut).
	longText := strings.Repeat("a", 120) + "XXXXX" // 125 ASCII chars
	criteriaA := []string{
		"- [x] build passes",
		"- [ ] long justification test",
		"  Caducou: " + longText,
	}
	criteriaB := []string{"- [x] tests pass"}
	content := buildLapsedRoadmap(criteriaA, criteriaB)
	dir, _ := setupLapsedFixture(t, content)

	// ── text mode ─────────────────────────────────────────────────────────────
	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--trust-local-gates")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// The text must NOT contain the 5 trailing 'X' characters (truncated).
	if strings.Contains(stdout, "XXXXX") {
		t.Fatalf("text output must not contain the truncated suffix 'XXXXX'\nstdout: %s", stdout)
	}
	// The text MUST contain the truncation marker "…".
	if !strings.Contains(stdout, "…") {
		t.Fatalf("text output must contain truncation marker '…'\nstdout: %s", stdout)
	}

	// ── JSON mode ─────────────────────────────────────────────────────────────
	jsonOut, _, code2 := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--json", "--trust-local-gates")
	if code2 != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s", code2, jsonOut)
	}
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonOut)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, jsonOut)
	}
	for _, c := range doc.Checks {
		if c.Name != "acceptance_evidence" {
			continue
		}
		if len(c.LapsedDetails) == 0 {
			t.Fatalf("expected lapsed_details to be populated, got empty\nJSON: %s", jsonOut)
		}
		for _, d := range c.LapsedDetails {
			if strings.Contains(d.Text, "XXXXX") {
				t.Fatalf("lapsed_details text must not contain truncated suffix 'XXXXX', got %q", d.Text)
			}
			if strings.Contains(d.Text, longText[:10]) && !strings.HasSuffix(d.Text, "…") {
				t.Fatalf("lapsed_details text must end with '…' when truncated, got %q", d.Text)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// D6/T8: all-lapsed ML is blocked
// ─────────────────────────────────────────────────────────────────────────────

// TestBarrierLapsed_AllLapsedBlocked asserts that the barrier reports
// acceptance_evidence=blocked with "all acceptance criteria lapsed" when an ML has
// Met=0 and Lapsed≥1 (T8: delivering requires at least one verified criterion).
func TestBarrierLapsed_AllLapsedBlocked(t *testing.T) {
	criteriaA := []string{"- [x] build passes"}
	criteriaB := []string{
		"- [ ] suíte pypi",
		"  Caducou: v8 removeu o CLI Python",
		"- [ ] versão no help",
		"  Caducou: número caducou",
	}
	content := buildLapsedRoadmap(criteriaA, criteriaB)
	dir, _ := setupLapsedFixture(t, content)

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-lapsed-fixture", "--wave", "1", "--json", "--trust-local-gates")
	if code != 1 {
		t.Fatalf("expected exit 1 (blocked — all criteria lapsed in ML-1B), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}

	for _, c := range doc.Checks {
		if c.Name != "acceptance_evidence" {
			continue
		}
		if c.Status != "blocked" {
			t.Fatalf("expected acceptance_evidence=blocked (all ML-1B criteria lapsed), got %q", c.Status)
		}
		// Must have "all acceptance criteria lapsed" failure for ML-1B.
		foundAllLapsed := false
		for _, f := range c.Failures {
			if strings.Contains(f, "ML-1B") && strings.Contains(f, "all acceptance criteria lapsed") {
				foundAllLapsed = true
			}
		}
		if !foundAllLapsed {
			t.Fatalf("expected failure 'ML-1B: all acceptance criteria lapsed', got %v", c.Failures)
		}
		// ML-1A must still pass.
		foundA := false
		for _, e := range c.Evidence {
			if strings.Contains(e, "ML-1A") && strings.Contains(e, "criteria met") {
				foundA = true
			}
		}
		if !foundA {
			t.Fatalf("expected evidence for ML-1A, got %v", c.Evidence)
		}
	}
}
