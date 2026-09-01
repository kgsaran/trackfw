package commands

// barrier_test.go — unit tests for the string-level parser and per-check evaluation
// logic in barrier.go, additional to the cross-runtime contract fixed in
// barrier_contract_test.go (which drives the real compiled binary end-to-end).
// These tests call the unexported parsing helpers directly and do not build a binary.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseWaves_SingleWave(t *testing.T) {
	content := "# Roadmap\n\n## Wave 1 — Foo\nbody line\n\n### ML-1A — x\n**Status:** ✅\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if waves[0].label != "1" {
		t.Fatalf("expected wave label \"1\", got %q", waves[0].label)
	}
}

func TestParseWaves_MultipleWavesEndAtNextH2(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap",
		"",
		"## Wave 1 — Foo",
		"content of wave 1",
		"",
		"## Wave 2 — Bar",
		"content of wave 2",
	}, "\n")
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if waves[0].label != "1" || waves[1].label != "2" {
		t.Fatalf("unexpected wave labels: %+v", waves)
	}
	// Wave 1 block must end exactly where the "## Wave 2" heading starts.
	if lines[waves[0].end] != "## Wave 2 — Bar" {
		t.Fatalf("expected wave 1 to end at 'Wave 2' heading, got %q", lines[waves[0].end])
	}
}

func TestParseWaves_MalformedLabelIsUsageError(t *testing.T) {
	content := "## Wave x — Foo\nbody\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr == nil {
		t.Fatalf("expected usage error for malformed wave label, got waves=%+v", waves)
	}
	if !strings.Contains(uerr.Error(), "line 1") {
		t.Fatalf("expected error to name line 1, got: %s", uerr.Error())
	}
	if !strings.Contains(uerr.Error(), "wave label") {
		t.Fatalf("expected error to say \"wave label\", got: %s", uerr.Error())
	}
}

// TestParseWaves_BisSuffix verifies that a valid wave label with a suffix is accepted.
func TestParseWaves_BisSuffix(t *testing.T) {
	content := "## Wave 2-bis — Corrective\nbody\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error for valid label 2-bis: %v", uerr)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if waves[0].label != "2-bis" {
		t.Fatalf("expected label \"2-bis\", got %q", waves[0].label)
	}
}

// TestParseWaves_LabelIdentityDistinct verifies that "2" and "2-bis" are distinct identities:
// a document with both headings must produce two separate waveBlocks.
func TestParseWaves_LabelIdentityDistinct(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 2 — Base Wave",
		"body of wave 2",
		"",
		"## Wave 2-bis — Corrective Wave",
		"body of wave 2-bis",
	}, "\n")
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves (distinct labels), got %d: %+v", len(waves), waves)
	}
	if waves[0].label != "2" {
		t.Fatalf("expected first label \"2\", got %q", waves[0].label)
	}
	if waves[1].label != "2-bis" {
		t.Fatalf("expected second label \"2-bis\", got %q", waves[1].label)
	}
	// Wave 2 block must end exactly where "## Wave 2-bis" starts.
	if lines[waves[0].end] != "## Wave 2-bis — Corrective Wave" {
		t.Fatalf("expected wave 2 to end at 'Wave 2-bis' heading, got %q", lines[waves[0].end])
	}
}

func TestParseMLs_MultipleMLsInWave(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"### ML-1A — First",
		"**Status:** ✅",
		"### ML-1B — Second",
		"**Status:** ⬜ Pendente",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, fenceMask(lines), 0, len(lines))
	if len(mls) != 2 {
		t.Fatalf("expected 2 MLs, got %d", len(mls))
	}
	if mls[0].id != "ML-1A" || mls[1].id != "ML-1B" {
		t.Fatalf("unexpected ML ids: %+v", mls)
	}
}

func TestMLStatusMarker_MissingLine(t *testing.T) {
	content := "### ML-1A — Foo\nno status line here\n"
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, fenceMask(lines), 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	_, found := mlStatusMarker(lines, fenceMask(lines), mls[0])
	if found {
		t.Fatal("expected found=false when no **Status:** line is present")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// statusIsComplete — first-token vocabulary (ADR decision 3/4/8, AC8/AC9/AC14).
// ────────────────────────────────────────────────────────────────────────────

// TestStatusIsComplete_Accepted covers the six accepted forms pinned by the
// ADR, including the two with a suffix that today pass only because matching
// is substring-based (48 occurrences in the corpus) — they must keep passing
// under first-token matching.
func TestStatusIsComplete_Accepted(t *testing.T) {
	cases := []string{
		"✅",
		"✅ Concluído",
		"✅ Concluído · **Agente:** `apolo-tf`",
		"✅ concluído (auditado 2026-08-02)",
		"done",
		"Concluído",
		"DONE",
		"concluido",
		"done\t· extra", // tab after the marker is a valid separator per unicode.IsSpace
		"done · extra",  // NBSP (U+00A0) after the marker is a valid separator
		"✅️",            // VS16 (U+FE0F) text-style emoji presentation — the single Mn exception (ADR decision 9)
	}
	for _, marker := range cases {
		marker := marker
		t.Run(marker, func(t *testing.T) {
			if !statusIsComplete(marker) {
				t.Errorf("statusIsComplete(%q) = false, want true", marker)
			}
		})
	}
}

// TestStatusIsComplete_Rejected is AC9 falsified in the opposite direction —
// each case is a vector the Wave 0 threat model named explicitly. Ampliar o
// vocabulário sem trocar contains()->first-token faria os quatro primeiros
// passarem (vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md).
func TestStatusIsComplete_Rejected(t *testing.T) {
	cases := []string{
		"não done",
		"pending (era done)",
		"notdone",
		"done-not-really",
		"⬜ Pendente",
		"🔄 Em andamento",
		"❌ Bloqueado",
		"⬜ Pendente ✅", // AC14 — position matters; today (contains) this passes in prod
		"`done`",       // marker inside inline code — backticks glue to the token
		"​done",        // zero-width space before the token — not unicode.IsSpace, stays glued
		"",
		"   ",
		"d᷀one", // AC15 — combining mark (U+1DC0) on the first token, rejected outright, not folded
		"do᷀ne", // AC15 — same, mark on a different codepoint of the token
		"done᷀", // AC15 — same, mark trailing the token
		"✅᷀",    // AC15 — combining mark on the emoji marker itself, still rejected
	}
	for _, marker := range cases {
		marker := marker
		t.Run(marker, func(t *testing.T) {
			if statusIsComplete(marker) {
				t.Errorf("statusIsComplete(%q) = true, want false", marker)
			}
		})
	}
}

// TestCriteriaHeaderRe_AcceptsEnglishAndPortuguese is AC1/AC2/AC3: the
// canonical English header and the Portuguese one must both match, anchored.
func TestCriteriaHeaderRe_AcceptsEnglishAndPortuguese(t *testing.T) {
	accepted := []string{
		"**Acceptance criteria:**",
		"**Critérios de aceite:**",
		"**Criterios de aceite:**", // accentless PT variant already covered by [eé]
	}
	for _, line := range accepted {
		if !criteriaHeaderRe.MatchString(line) {
			t.Errorf("criteriaHeaderRe: expected %q to match", line)
		}
	}
	// The anchor is load-bearing: quoting the header in prose must NOT match.
	rejected := []string{
		"the header is **Acceptance criteria:**",
		"> **Critérios de aceite:**",
		"prose citing **Acceptance criteria:** mid-sentence",
	}
	for _, line := range rejected {
		if criteriaHeaderRe.MatchString(line) {
			t.Errorf("criteriaHeaderRe: expected %q NOT to match (anchor must reject mid-line quotes)", line)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Fence-awareness (ADR decision 7, AC13) — mlStatusMarker/acceptanceEvaluate/
// parseMLs must ignore content inside ``` fences. Reproduces forged.md and
// forged3.md from the ML-0A threat-model result, verbatim.
// ────────────────────────────────────────────────────────────────────────────

// TestFenceAwareness_StatusInsideFenceIsIgnored is forged.md: a fenced example
// citing "**Status:** done" must not shadow the real "**Status:** pending"
// outside the fence.
func TestFenceAwareness_StatusInsideFenceIsIgnored(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — probe",
		"Example of the bug we are documenting:",
		"```",
		"**Status:** done",
		"```",
		"**Status:** pending",
	}, "\n")
	lines := strings.Split(content, "\n")
	fenced := fenceMask(lines)
	mls := parseMLs(lines, fenced, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	marker, found := mlStatusMarker(lines, fenced, mls[0])
	if !found {
		t.Fatal("expected found=true (the real, unfenced **Status:** line)")
	}
	if marker != "pending" {
		t.Fatalf("expected marker %q (the unfenced status), got %q", "pending", marker)
	}
	if statusIsComplete(marker) {
		t.Fatal("expected the real status (\"pending\") to be incomplete — the fenced \"done\" must not leak in")
	}
}

// TestFenceAwareness_AcceptanceInsideFenceIsIgnored is forged3.md: a fenced
// example citing "**Critérios de aceite:**" with "- [x]" must not be read as
// the ML's real acceptance block when there is no real block outside it.
func TestFenceAwareness_AcceptanceInsideFenceIsIgnored(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — probe",
		"Example of the bug we are documenting:",
		"```",
		"**Critérios de aceite:**",
		"- [x] fake evidence, nothing built",
		"```",
		"**Status:** ✅",
	}, "\n")
	lines := strings.Split(content, "\n")
	fenced := fenceMask(lines)
	mls := parseMLs(lines, fenced, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	_, _, hasBlock := acceptanceEvaluate(lines, fenced, mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false — the acceptance block cited inside the fence must not count as real evidence")
	}
}

// TestFenceAwareness_MLHeadingInsideFenceIsNotPhantomML is AC13-b: a
// "### ML-XX" heading inside a fence must not be detected as a real ML.
// Reproduced live against 7.3.0 per the ADR ("### ML-9Z ... prosa; o barrier
// reporta 'ML-9Z: not complete'").
func TestFenceAwareness_MLHeadingInsideFenceIsNotPhantomML(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"### ML-1A — Real ML",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] real criterion",
		"",
		"Example of a malformed heading inside a fence, cited as documentation:",
		"```markdown",
		"### ML-9Z — phantom, must not be detected",
		"**Status:** ⬜ Pendente",
		"```",
	}, "\n")
	lines := strings.Split(content, "\n")
	fenced := fenceMask(lines)
	mls := parseMLs(lines, fenced, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML (the fenced ML-9Z must not be detected), got %d: %+v", len(mls), mls)
	}
	if mls[0].id != "ML-1A" {
		t.Fatalf("expected the single ML to be ML-1A, got %q", mls[0].id)
	}
}

// TestBarrierCLI_EnglishHeaderAndWordStatusPass is the end-to-end AC1/AC12
// regression: a roadmap written exactly the way `roadmap new` writes it today
// (English acceptance header, word status) must pass mls_complete and
// acceptance_evidence with the real compiled binary, without editing the
// header by hand.
func TestBarrierCLI_EnglishHeaderAndWordStatusPass(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"docs/roadmaps/wip", "docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	content := strings.Join([]string{
		"# Roadmap: English dialect fixture",
		"",
		"REQ: REQ-2026-08-29-barrier-fixture",
		"",
		"## Acceptance Criteria",
		"- [x] fixture roadmap-level criterion",
		"",
		"## Wave 1 — Fixture Wave",
		"> Dependencies: none",
		"",
		"### ML-1A — Fixture ML",
		"**Status:** done",
		"**Acceptance criteria:**",
		"- [x] build passes",
	}, "\n")
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-english-fixture.md")
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-english-fixture", "--wave", "1", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0 (passed), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	for _, name := range []string{"mls_complete", "acceptance_evidence"} {
		for _, c := range doc.Checks {
			if c.Name == name && c.Status != "passed" {
				t.Fatalf("expected %s=passed, got %q (failures: %v)", name, c.Status, c.Failures)
			}
		}
	}
}

// TestBarrierCLI_ForgedFenceContentDoesNotLiberateWave is the end-to-end
// regression for ADR decision 7 (AC13): a wave whose only ML has its real,
// unfenced status as "pending" (not complete) and its only acceptance block
// fenced (forged) must stay blocked on the real binary — the fenced "done"
// and fenced "- [x]" must not leak into mls_complete / acceptance_evidence.
func TestBarrierCLI_ForgedFenceContentDoesNotLiberateWave(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"docs/roadmaps/wip", "docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	content := strings.Join([]string{
		"# Roadmap: Forged fence fixture",
		"",
		"REQ: REQ-2026-08-29-barrier-fixture",
		"",
		"## Acceptance Criteria",
		"- [x] fixture roadmap-level criterion",
		"",
		"## Wave 1 — Fixture Wave",
		"> Dependencies: none",
		"",
		"### ML-1A — Fixture ML",
		"Example of the bug we are documenting:",
		"```",
		"**Status:** done",
		"**Critérios de aceite:**",
		"- [x] fake evidence, nothing built",
		"```",
		"**Status:** pending",
	}, "\n")
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-forged-fence-fixture.md")
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-forged-fence-fixture", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1 (blocked — the forged fenced content must not liberate the wave), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	for _, c := range doc.Checks {
		if c.Name == "mls_complete" {
			if c.Status != "blocked" {
				t.Fatalf("expected mls_complete=blocked (real status is \"pending\"), got %q", c.Status)
			}
			if len(c.Failures) != 1 || !strings.Contains(c.Failures[0], "status: pending") {
				t.Fatalf("expected mls_complete failure to name the real status \"pending\", got %v", c.Failures)
			}
		}
		if c.Name == "acceptance_evidence" {
			if c.Status != "blocked" {
				t.Fatalf("expected acceptance_evidence=blocked (fenced block must not count), got %q", c.Status)
			}
			if len(c.Failures) != 1 || !strings.Contains(c.Failures[0], "no acceptance block") {
				t.Fatalf("expected acceptance_evidence failure \"no acceptance block\", got %v", c.Failures)
			}
		}
	}
}

func TestAcceptanceEvaluate_AllMet(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] build passes",
		"- [x] tests pass",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, fenceMask(lines), 0, len(lines))
	met, unmet, hasBlock := acceptanceEvaluate(lines, fenceMask(lines), mls[0])
	if !hasBlock {
		t.Fatal("expected hasBlock=true")
	}
	if unmet != 0 {
		t.Fatalf("expected 0 unmet, got %d", unmet)
	}
	if met != 2 {
		t.Fatalf("expected 2 met, got %d", met)
	}
}

func TestAcceptanceEvaluate_EmptyBlockIsNotVacuouslyPassed(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"**Files affected:**",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, fenceMask(lines), 0, len(lines))
	_, _, hasBlock := acceptanceEvaluate(lines, fenceMask(lines), mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false for an empty acceptance block (anti-vacuity)")
	}
}

func TestAcceptanceEvaluate_NoHeaderAtAll(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, fenceMask(lines), 0, len(lines))
	_, _, hasBlock := acceptanceEvaluate(lines, fenceMask(lines), mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false when no **Critérios de aceite:** header exists")
	}
}

func TestParseGates_NoBlockYieldsEmptyNonNilSlice(t *testing.T) {
	content := "## Wave 1 — Foo\nno gates here\n"
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if cmds == nil {
		t.Fatal("expected non-nil empty slice for a wave with no gates block")
	}
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands, got %v", cmds)
	}
}

func TestParseGates_ParsesCommandsIgnoringBlankAndComment(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"```bash",
		"go build ./...",
		"",
		"# a comment",
		"go test ./...",
		"```",
	}, "\n")
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	want := []string{"go build ./...", "go test ./..."}
	if len(cmds) != len(want) {
		t.Fatalf("expected %v, got %v", want, cmds)
	}
	for i := range want {
		if cmds[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, cmds)
		}
	}
}

// TestParseGates_HeaderIsAPrefixMatchNotFullLineEquality is ML-1B: the
// '**Gates da wave:**' header must be recognised as a PREFIX (gatesHeaderRe),
// not full-line equality — a header followed by trailing prose on the same
// line must still be recognised. This is the contract Node.js briefly
// regressed while removing its per-line .trim() (see the cross-runtime
// scenario "falsify/gates-header-prefix-match-with-trailing-prose-cross-runtime"
// in scripts/check-roadmap-barrier-contract.sh, ML-3F).
func TestParseGates_HeaderIsAPrefixMatchNotFullLineEquality(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:** (obrigatórios)",
		"```bash",
		"make build",
		"```",
	}, "\n")
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	want := []string{"make build"}
	if len(cmds) != 1 || cmds[0] != want[0] {
		t.Fatalf("expected %v, got %v", want, cmds)
	}
}

func TestParseGates_UnterminatedFenceIsUsageError(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"```bash",
		"go build ./...",
	}, "\n")
	lines := strings.Split(content, "\n")
	_, uerr := parseGates(lines, 0, len(lines))
	if uerr == nil {
		t.Fatal("expected usage error for unterminated fence")
	}
}

func TestParseGates_MissingFenceRightAfterHeaderIsUsageError(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"go build ./...",
	}, "\n")
	lines := strings.Split(content, "\n")
	_, uerr := parseGates(lines, 0, len(lines))
	if uerr == nil {
		t.Fatal("expected usage error when no ```bash fence immediately follows the gates header")
	}
}

// TestBarrierCheck_JSONKeyOrderMatchesCliParityContract asserts the literal key
// order of a serialized barrierCheck — not just presence of keys. This is the
// regression coverage for the Go-vs-Node/Python "gates" check key-order
// divergence described in vault/notes/barrier-gates-check-key-order-divergence-go-2026-07-29.md:
// encoding/json serializes struct fields in declaration order, so reordering the
// barrierCheck struct silently changes the emitted JSON. docs/cli-parity.md pins
// "commands" as the third key (right after "status") for the gates check.
func TestBarrierCheck_JSONKeyOrderMatchesCliParityContract(t *testing.T) {
	cmds := []string{"go build ./..."}
	check := barrierCheck{
		Name:     "gates",
		Status:   "passed",
		Commands: &cmds,
		Evidence: []string{"go build ./...: exit 0"},
		Failures: []string{},
	}
	out, err := json.Marshal(check)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	wantOrder := []string{`"name"`, `"status"`, `"commands"`, `"evidence"`, `"failures"`}
	var positions []int
	for _, key := range wantOrder {
		pos := strings.Index(string(out), key)
		if pos < 0 {
			t.Fatalf("expected key %s to be present in %s", key, out)
		}
		positions = append(positions, pos)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] >= positions[i] {
			t.Fatalf("expected key order %v, got JSON with wrong order: %s", wantOrder, out)
		}
	}

	// A check without gates (Commands == nil) must omit "commands" entirely —
	// never emit it as null.
	nonGatesCheck := barrierCheck{
		Name:     "mls_complete",
		Status:   "passed",
		Evidence: []string{},
		Failures: []string{},
	}
	nonGatesOut, err := json.Marshal(nonGatesCheck)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(nonGatesOut), "commands") {
		t.Fatalf("expected non-gates check to omit \"commands\" entirely, got: %s", nonGatesOut)
	}
}

func TestRunGateCommand_ExitCodes(t *testing.T) {
	if code, spawnFailed := runGateCommand("true"); code != 0 || spawnFailed {
		t.Fatalf("expected exit 0/spawnFailed=false for 'true', got %d/%v", code, spawnFailed)
	}
	if code, spawnFailed := runGateCommand("false"); code != 1 || spawnFailed {
		t.Fatalf("expected exit 1/spawnFailed=false for 'false', got %d/%v", code, spawnFailed)
	}
	if code, spawnFailed := runGateCommand("exit 7"); code != 7 || spawnFailed {
		t.Fatalf("expected exit 7/spawnFailed=false for 'exit 7', got %d/%v", code, spawnFailed)
	}
	// exit 127 signals "tool not found inside sh" — sh itself started and ran.
	// This must NEVER be confused with spawnFailed (ML-0A measurement).
	if code, spawnFailed := runGateCommand("nosuchtool-xyz"); code != 127 || spawnFailed {
		t.Fatalf("expected exit 127/spawnFailed=false for a missing tool inside sh, got %d/%v", code, spawnFailed)
	}
}

// TestRunGateCommand_ShMissing_SpawnFailed proves the spawn-failure signal by
// curating a $PATH that genuinely lacks `sh` — not by asserting on exit code 127
// (which is the "tool inside sh missing" case, proven not to collide above).
func TestRunGateCommand_ShMissing_SpawnFailed(t *testing.T) {
	curated := t.TempDir()
	t.Setenv("PATH", curated)
	code, spawnFailed := runGateCommand("true")
	if !spawnFailed {
		t.Fatalf("expected spawnFailed=true when sh is absent from $PATH, got code=%d spawnFailed=%v", code, spawnFailed)
	}
}

func TestEvalGateCommands_ShMissing_NotEvaluated(t *testing.T) {
	curated := t.TempDir()
	t.Setenv("PATH", curated)
	status, evidence, failures := evalGateCommands([]string{"true", "false"})
	if status != "not_evaluated" {
		t.Fatalf("expected status not_evaluated, got %q", status)
	}
	if len(evidence) != 0 {
		t.Fatalf("expected empty evidence, got %v", evidence)
	}
	if len(failures) != 1 || failures[0] != shMissingMsg {
		t.Fatalf("expected exactly one pinned failure %q, got %v", shMissingMsg, failures)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Wave label ordering tests (compareWaveLabels, splitWaveLabel).
// ────────────────────────────────────────────────────────────────────────────

func TestWaveLabelOrdering(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Numeric ordering (the discriminating case: 10 > 2, not lexicographic)
		{"10", "2", 1},
		{"2", "10", -1},
		{"3", "3", 0},
		// No-suffix before with-suffix on same integer
		{"2", "2-bis", -1},
		{"2-bis", "2", 1},
		// Lexicographic between suffixes
		{"2-bis", "2-hotfix", -1},
		{"2-hotfix", "2-bis", 1},
		{"2-bis", "2-bis", 0},
		// Full contract example: 2 < 2-bis < 2-hotfix < 3
		{"2", "3", -1},
		{"2-hotfix", "3", -1},
	}
	for _, tc := range cases {
		got := compareWaveLabels(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("compareWaveLabels(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSplitWaveLabel(t *testing.T) {
	cases := []struct {
		label   string
		wantInt int
		wantSuf string
	}{
		{"1", 1, ""},
		{"2", 2, ""},
		{"2-bis", 2, "bis"},
		{"2-hotfix", 2, "hotfix"},
		{"10-a2", 10, "a2"},
	}
	for _, tc := range cases {
		gotInt, gotSuf := splitWaveLabel(tc.label)
		if gotInt != tc.wantInt || gotSuf != tc.wantSuf {
			t.Errorf("splitWaveLabel(%q) = (%d, %q), want (%d, %q)",
				tc.label, gotInt, gotSuf, tc.wantInt, tc.wantSuf)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Regression: malformed heading aborts ENTIRE document — ADR decision 16.
// This end-to-end test is the evidence that prevents anyone from "fixing"
// the abort into a scoped skip for the requested wave.
// ────────────────────────────────────────────────────────────────────────────

// TestParseWaves_MalformedHeadingAbortsEntireDocument_Regression proves that a
// malformed wave heading (`## Wave X — ...`) aborts the whole document even when
// the requested wave (`--wave 1`) is fully green and appears before the bad heading.
// This is the regression guard for ADR decision 16.
func TestParseWaves_MalformedHeadingAbortsEntireDocument_Regression(t *testing.T) {
	// Build a multi-wave roadmap where Wave 1 is fully green and Wave X is malformed.
	// Line positions are fixed so we can assert the exact pinned error message.
	content := strings.Join([]string{
		"# Roadmap: Malformed Wave Regression", // line 1
		"",                                     // line 2
		"REQ: REQ-regression-test",             // line 3
		"",                                     // line 4
		"## Acceptance Criteria",               // line 5
		"- [x] fixture criterion",              // line 6
		"",                                     // line 7
		"## Wave 1 — Green Wave",               // line 8
		"> Dependências: nenhuma",              // line 9
		"",                                     // line 10
		"### ML-1A — Green ML",                 // line 11
		"**Status:** ✅ Concluído",              // line 12
		"**Critérios de aceite:**",             // line 13
		"- [x] criterion met",                  // line 14
		"",                                     // line 15
		"## Wave X — Malformed Label",          // line 16 ← bad heading
		"> body of malformed wave",             // line 17
		"",                                     // line 18
		"### ML-X1 — Irrelevant ML",            // line 19
		"**Status:** ✅",                        // line 20
		"**Critérios de aceite:**",             // line 21
		"- [x] whatever",                       // line 22
	}, "\n")

	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr == nil {
		t.Fatalf("expected parse error for malformed heading, got %d waves", len(waves))
	}

	// The error must name the line AND carry the pinned fragment.
	wantFrag := `"X" is not a valid wave label`
	if !strings.Contains(uerr.Error(), "line 16") {
		t.Errorf("expected error to name line 16, got: %s", uerr.Error())
	}
	if !strings.Contains(uerr.Error(), wantFrag) {
		t.Errorf("expected error to contain %q, got: %s", wantFrag, uerr.Error())
	}

	// End-to-end: even requesting --wave 1 (the green wave) must exit 2, not 0.
	// This is the regression guard that prevents "fix" the abort into a scoped skip.
	dir := t.TempDir()
	for _, d := range []string{"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-malformed-regression.md")
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-malformed-regression", "--wave", "1")
	if code != 2 {
		t.Fatalf("expected exit 2 (abort on malformed heading), got %d\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	// Assert pinned stderr byte-for-byte.
	wantStderr := "trackfw barrier: malformed wave heading at line 16: \"X\" is not a valid wave label\n"
	if stderr != wantStderr {
		t.Fatalf("stderr mismatch:\nwant: %q\ngot:  %q", wantStderr, stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Regression tests — ML-2D. These two cases had zero coverage before ML-2D,
// which is exactly why the cross-runtime divergence (Go vs Node.js vs Python)
// slipped through the per-runtime suites in the Wave 2 barrier run.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierRegression_WaveWithNoMLProducesPinnedFailureMessage(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"docs/roadmaps/wip", "docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	content := "# Roadmap: No ML\n\nREQ: REQ-x\n\n" +
		"## Acceptance Criteria\n- [x] fixture\n\n" +
		"## Wave 1 — Sem MLs\n> Dependências: nenhuma\n\n" +
		"Some prose, no ML heading at all.\n"
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-no-ml.md")
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-no-ml", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1 (blocked), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	var mlsCheck *barrierCheckDoc
	for i := range doc.Checks {
		if doc.Checks[i].Name == "mls_complete" {
			mlsCheck = &doc.Checks[i]
		}
	}
	if mlsCheck == nil {
		t.Fatalf("mls_complete check not found in: %+v", doc.Checks)
	}
	want := []string{"wave 1: no ML found"}
	if len(mlsCheck.Failures) != 1 || mlsCheck.Failures[0] != want[0] {
		t.Fatalf("expected mls_complete failures %v, got %v", want, mlsCheck.Failures)
	}
}

func TestBarrierRegression_Exit2MessagesArePinnedLiterally(t *testing.T) {
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
	})

	_, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "99", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d, stderr=%s", code, stderr)
	}
	wantWave := "trackfw barrier: wave 99 not found in roadmap \"ROADMAP-barrier-fixture.md\"\n"
	if stderr != wantWave {
		t.Fatalf("wave-not-found message mismatch:\nwant: %q\ngot:  %q", wantWave, stderr)
	}

	_, stderr2, code2 := runBarrierCLI(t, dir, "ROADMAP-nao-existe", "--wave", "1", "--json")
	if code2 != 2 {
		t.Fatalf("expected exit 2, got %d, stderr=%s", code2, stderr2)
	}
	wantRoadmap := "trackfw barrier: roadmap \"ROADMAP-nao-existe\" not found in wip/ nor done/ under docs/roadmaps\n"
	if stderr2 != wantRoadmap {
		t.Fatalf("roadmap-not-found message mismatch:\nwant: %q\ngot:  %q", wantRoadmap, stderr2)
	}
}

// TestWaveLabelGrammar_ValidAndInvalid is a table test of the full contract table
// from docs/cli-parity.md §wave-label-grammar (pinned in ML-3A, extended in
// ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness ML-1A). It exercises
// the composite predicate used in the heading pre-pass: waveLabelRe (regex) +
// int≥0 guard in parseWaves. "0" is now valid (the Wave 0 threat-model
// convention) — it used to be the only label that passed the regex but failed
// the (then int≥1) guard; parseWaves remains the correct surface to test it on.
func TestWaveLabelGrammar_ValidAndInvalid(t *testing.T) {
	valid := []string{"0", "1", "2", "2-bis", "2-hotfix", "10-a2"}
	invalid := []string{"X", "2-BIS", "-bis", "2-", "2-bis-ter"}

	for _, lbl := range valid {
		lbl := lbl
		t.Run("valid/"+lbl, func(t *testing.T) {
			if !waveLabelRe.MatchString(lbl) {
				t.Errorf("waveLabelRe: expected %q to match (valid per contract), but it did not", lbl)
			}
			// Composite check: heading pre-pass must also accept it.
			content := "## Wave " + lbl + " — Test Heading\nbody\n"
			lines := strings.Split(content, "\n")
			_, uerr := parseWaves(lines)
			if uerr != nil {
				t.Errorf("parseWaves: expected no error for valid label %q, got: %v", lbl, uerr)
			}
		})
	}

	for _, lbl := range invalid {
		lbl := lbl
		t.Run("invalid/"+lbl, func(t *testing.T) {
			// Use parseWaves (not waveLabelRe alone) to exercise the full composite
			// predicate.
			content := "## Wave " + lbl + " — Bad Heading\nbody\n"
			lines := strings.Split(content, "\n")
			_, uerr := parseWaves(lines)
			if uerr == nil {
				t.Errorf("parseWaves: expected usage error for invalid label %q, got nil", lbl)
			}
		})
	}
}

// TestBarrierRegression_FourthExitTwoMessage proves the fourth pinned exit-2
// message (cli-parity.md §four-pinned-exit-2-messages) is emitted byte-for-byte
// when --wave receives a syntactically invalid label. This is the message class
// that distinguished from the third message (malformed heading) — same exit code,
// different trigger surface (argument, not document). Node.js had complete coverage
// of this path before ML-3A; this test closes the gap in Go.
func TestBarrierRegression_FourthExitTwoMessage(t *testing.T) {
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
	})

	_, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "2-BIS")
	if code != 2 {
		t.Fatalf("expected exit 2 for invalid --wave label, got %d; stderr=%s", code, stderr)
	}
	want := "trackfw barrier: invalid --wave \"2-BIS\" — not a valid wave label\n"
	if stderr != want {
		t.Fatalf("fourth pinned message mismatch:\nwant: %q\ngot:  %q", want, stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Trust check tests (AC11, AC12, AC3, AC6 — ML-2A)
// ────────────────────────────────────────────────────────────────────────────

// TestRoadmapTrustForGates_FailOpenWhenNotGitRepo verifies that when the roadmap
// is in a directory that is not a git repository, the trust verdict is open
// (trusted). This is the fail-open residual for non-git environments such as the
// check-barrier.sh fixtures in temp dirs.
func TestRoadmapTrustForGates_FailOpenWhenNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	roadmapPath := filepath.Join(dir, "ROADMAP.md")
	if err := os.WriteFile(roadmapPath, []byte("# Roadmap: test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	verdict := roadmapTrustForGates(roadmapPath)
	if !verdict.trusted {
		t.Fatalf("expected trusted=true for non-git-repo path, got failureMsg=%q", verdict.failureMsg)
	}
}

// TestBarrierTrustLocalGatesFlag verifies that --trust-local-gates causes the
// barrier to evaluate gates from local content without a trust check. The gate
// command "true" must exit 0 and be recorded in the gates check.
func TestBarrierTrustLocalGatesFlag(t *testing.T) {
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
		gateCommands:  []string{"true"},
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1",
		"--trust-local-gates", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0 with --trust-local-gates, got %d\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	found := false
	for _, c := range doc.Checks {
		if c.Name == "gates" {
			found = true
			if c.Status != "passed" {
				t.Fatalf("expected gates=passed with --trust-local-gates, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("gates check not found in result document")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ML-1B achado 1 — fenceMask must recognise ~~~ and 4+-backtick fences per
// CommonMark (3+ of the SAME character, closed by a run of the same character
// with length >= the opening run's length).
// ────────────────────────────────────────────────────────────────────────────

func TestFenceMask_TildeFenceMasksSameAsBacktick(t *testing.T) {
	lines := []string{"before", "~~~", "inside", "~~~", "after"}
	got := fenceMask(lines)
	want := []bool{false, false, true, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fenceMask(~~~) = %v, want %v", got, want)
	}
}

func TestFenceMask_FourBacktickFenceMasksNestedThreeBacktickBlock(t *testing.T) {
	lines := []string{
		"before",
		"````",
		"outer",
		"```",
		"nested (shorter run, must stay masked as interior)",
		"```",
		"still outer",
		"````",
		"after",
	}
	got := fenceMask(lines)
	want := []bool{false, false, true, true, true, true, true, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fenceMask(4-backtick nesting 3-backtick) = %v, want %v", got, want)
	}
}

func TestFenceMask_ClosingRequiresSameCharacterAndLengthGE(t *testing.T) {
	// A ``` line inside a ~~~ fence does not close it (different character).
	lines := []string{"~~~", "```", "still inside", "~~~"}
	got := fenceMask(lines)
	want := []bool{false, true, true, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fenceMask(mismatched char) = %v, want %v", got, want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ML-1B achado 2 — status/acceptance-header/criterion-item markers must be
// matched against the RAW line (column 0), never a per-line-trimmed line.
// Go was already strict; these tests pin that contract explicitly so a future
// change cannot silently regress it.
// ────────────────────────────────────────────────────────────────────────────

func TestMLStatusMarker_IndentedStatusLineIsNotRecognised(t *testing.T) {
	content := "### ML-1A — Real ML\n  **Status:** done\n"
	lines := strings.Split(content, "\n")
	fenced := fenceMask(lines)
	mls := parseMLs(lines, fenced, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	_, found := mlStatusMarker(lines, fenced, mls[0])
	if found {
		t.Fatal("expected found=false — an indented \"**Status:**\" line must not be recognised")
	}
}

func TestAcceptanceEvaluate_IndentedHeaderAndCriteriaAreNotRecognised(t *testing.T) {
	content := "### ML-1A — Real ML\n  **Critérios de aceite:**\n  - [x] indented criterion\n"
	lines := strings.Split(content, "\n")
	fenced := fenceMask(lines)
	mls := parseMLs(lines, fenced, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	_, _, hasBlock := acceptanceEvaluate(lines, fenced, mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false — an indented acceptance header must not be recognised")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ML-1B achado 1 — closing-fence positive case: a LONGER closing run of the
// same character must close the fence (length >= opening, per CommonMark).
// The two adjacent negatives (different char; shorter nested run) are already
// pinned by TestFenceMask_ClosingRequiresSameCharacterAndLengthGE and
// TestFenceMask_FourBacktickFenceMasksNestedThreeBacktickBlock.
// ────────────────────────────────────────────────────────────────────────────

func TestFenceMask_LongerClosingRunOfSameCharacterCloses(t *testing.T) {
	lines := []string{"before", "```", "inside", "`````", "after"}
	got := fenceMask(lines)
	want := []bool{false, false, true, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fenceMask(open 3, close 5) = %v, want %v", got, want)
	}
}

// TestSplitRoadmapLines_StripsTrailingCROnlyAtBoundary is the unit-level
// falsification of splitRoadmapLines itself: it must strip exactly a
// trailing "\r" per line and nothing else — no leading-whitespace trimming
// (that would reintroduce the ML-1B regression) and no merging of lines.
//
// KNOWN GAP, reported rather than hidden: this test only fails if the
// FUNCTION is deleted or changed, not if runBarrier stops CALLING it. In Go
// specifically, the call-site revert (lines := strings.Split(string(data),
// "\n") instead of splitRoadmapLines(string(data))) is behaviorally a no-op —
// verified empirically: statusLineRe's "." already matches "\r" (Go's RE2 "."
// excludes only "\n", unlike JS), mlHeadingRe/waveHeadingRe/criterionLineRe/
// boldLineRe are all prefix-anchored (unaffected by a trailing "\r"), and
// every downstream comparison in mlStatusMarker/parseGates goes through
// strings.TrimSpace, which treats "\r" as whitespace regardless. No CRLF
// fixture — including every CRLF cross-runtime scenario in
// scripts/check-roadmap-barrier-contract.sh (ML-3C/ML-3F) — can distinguish
// "Go calls splitRoadmapLines" from "Go still splits on \n alone", because
// the two are equivalent for every marker this parser currently recognises.
// A call-site assertion (asserting runBarrier invokes this exact function)
// was deliberately rejected — see ML-3A's own note on why testing through
// the internal call graph, instead of observable CLI behavior, is how the
// ML-2G defect escaped audit. The call is still made, for symmetry with
// Node.js (where it IS load-bearing) and as a guard against a FUTURE marker
// that does not go through TrimSpace; it is simply, today, unfalsifiable in
// Go by a black-box test. Python has the same gap for a different reason:
// os.ReadFile-equivalent open(path, "r", encoding="utf-8") already runs
// universal-newlines translation, so _split_roadmap_lines never even sees a
// "\r" in production. Node.js is the one runtime where this normalization is
// genuinely load-bearing and where its removal is caught — see the
// "crlf/full-roadmap-passes-cross-runtime" scenario in
// scripts/check-roadmap-barrier-contract.sh, empirically confirmed by
// reverting npm/src/commands/barrier.js's call site during ML-3C and
// observing the scenario fail with the exact original defect.
func TestSplitRoadmapLines_StripsTrailingCROnlyAtBoundary(t *testing.T) {
	got := splitRoadmapLines("  **Status:** done\r\n**Status:** done\r\nlast\r")
	want := []string{"  **Status:** done", "**Status:** done", "last"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitRoadmapLines = %#v, want %#v", got, want)
	}
}
