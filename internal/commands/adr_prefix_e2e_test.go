package commands

// adr_prefix_e2e_test.go — End-to-end tests for the ADR-prefix identification criterion
// (issue #471, ML-2A of ROADMAP-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr...).
// Covers AC2, AC3, AC5, AC6, AC7 against the real binary.
//
// Counter-arm protocol:
//   TRACKFW_E2E_BIN=<path/to/tfw-44718> go test ./internal/commands/ -run 'ADRPrefixE2E' -count=1 -v
//   go test ./internal/commands/ -run 'ADRPrefixE2E' -count=1 -v
//
// Harness reuse: e2eBinary, e2eCleanEnv, runTFW, gitE2E — all defined in branch_state_e2e_test.go
// (same package). No re-declaration here.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ─── Harness helpers (ADR-prefix scope) ──────────────────────────────────────

// adrPrefixYAML returns a strict-mode trackfw.yaml with adr_dirs: [docs/adr].
func adrPrefixYAML() string {
	return `governance_mode: strict
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: none
forge: none
`
}

// makeADRPrefixRepo creates an isolated git repo with trackfw.yaml that declares
// adr_dirs: [docs/adr]. Returns (repoDir, homeDir).
func makeADRPrefixRepo(t *testing.T) (repoDir, homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	repoDir = t.TempDir()

	gitE2E(t, repoDir, homeDir, "init", "-q", "-b", "main", ".")
	gitE2E(t, repoDir, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, repoDir, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, repoDir, homeDir, "config", "commit.gpgsign", "false")

	for _, d := range []string{
		"docs/adr",
		"docs/req",
		"docs/roadmaps/wip",
		"docs/roadmaps/done",
		"docs/roadmaps/blocked",
		"docs/roadmaps/backlog",
		"docs/roadmaps/abandoned",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, d), 0755); err != nil {
			t.Fatalf("mkdirall %s: %v", d, err)
		}
	}

	if err := os.WriteFile(filepath.Join(repoDir, "trackfw.yaml"),
		[]byte(adrPrefixYAML()), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	sentinel := filepath.Join(repoDir, "docs", ".gitkeep")
	if err := os.WriteFile(sentinel, []byte{}, 0644); err != nil {
		t.Fatalf("write docs/.gitkeep: %v", err)
	}

	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: init adr-prefix test layout")
	return repoDir, homeDir
}

// writeADRFile writes a file to docs/adr inside repoDir.
func writeADRFile(t *testing.T, repoDir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, "docs", "adr", filename),
		[]byte(content), 0644); err != nil {
		t.Fatalf("writeADRFile %s: %v", filename, err)
	}
}

// mustExtractADRCountStatus extracts the integer after "ADRs" from trackfw status output.
// Fatal if not found — prevents vacuous pass.
func mustExtractADRCountStatus(t *testing.T, out, arm string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*ADRs\s+(\d+)\s*$`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("mustExtractADRCountStatus [%s]: could not find 'ADRs <n>' line in output:\n%s", arm, out)
	}
	return m[1]
}

// mustExtractContextScore extracts the governance score from trackfw context output.
// Fatal if not found.
func mustExtractContextScore(t *testing.T, out, arm string) string {
	t.Helper()
	re := regexp.MustCompile(`\*\*Governance score:\*\* (\d+)/100`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("mustExtractContextScore [%s]: could not find '**Governance score:** n/100' in output:\n%s", arm, out)
	}
	return m[1]
}

// mustExtractDiscoverScore extracts the governance score from trackfw discover output.
// Fatal if not found.
func mustExtractDiscoverScore(t *testing.T, out, arm string) string {
	t.Helper()
	re := regexp.MustCompile(`Governance Score: (\d+)/100`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("mustExtractDiscoverScore [%s]: could not find 'Governance Score: n/100' in output:\n%s", arm, out)
	}
	return m[1]
}

// ─── Test 1: AC2 — three arms (empty / NOTAS.md / ADR-…) ────────────────────
//
// Reconciliation sentence: the test affirms that NOTAS.md (no ADR- prefix) is
// invisible to status/context/discover/adr-list/validate — identical to empty adr_dirs —
// while a real ADR-2026-01-01-x.md changes the status ADR count to 1.
//
// Counter-arm (old binary, 44718ffc): any .md counts → arm(b) shows ADRs≠0, context score
// differs from arm(a), discover shows "ADRs found: 1", validate emits frontmatter violation.
// Expected: FAIL.
func TestADRPrefixE2E_AC2_ThreeArms(t *testing.T) {
	// ─── arm (a): empty docs/adr ─────────────────────────────────────────────
	repoA, homeA := makeADRPrefixRepo(t)

	statusA, _ := runTFW(t, repoA, homeA, "status")
	t.Logf("[arm-a] status:\n%s", statusA)
	contextA, _ := runTFW(t, repoA, homeA, "context")
	t.Logf("[arm-a] context:\n%s", contextA)
	discoverA, _ := runTFW(t, repoA, homeA, "discover")
	t.Logf("[arm-a] discover:\n%s", discoverA)
	adrListA, _ := runTFW(t, repoA, homeA, "adr", "list")
	t.Logf("[arm-a] adr list:\n%s", adrListA)

	countStatusA := mustExtractADRCountStatus(t, statusA, "arm-a")
	scoreContextA := mustExtractContextScore(t, contextA, "arm-a")
	scoreDiscoverA := mustExtractDiscoverScore(t, discoverA, "arm-a")

	// ─── arm (b): only NOTAS.md ───────────────────────────────────────────────
	repoB, homeB := makeADRPrefixRepo(t)
	writeADRFile(t, repoB, "NOTAS.md", "# notas\n")

	statusB, _ := runTFW(t, repoB, homeB, "status")
	t.Logf("[arm-b] status:\n%s", statusB)
	contextB, _ := runTFW(t, repoB, homeB, "context")
	t.Logf("[arm-b] context:\n%s", contextB)
	discoverB, _ := runTFW(t, repoB, homeB, "discover")
	t.Logf("[arm-b] discover:\n%s", discoverB)
	adrListB, _ := runTFW(t, repoB, homeB, "adr", "list")
	t.Logf("[arm-b] adr list:\n%s", adrListB)
	validateB, _ := runTFW(t, repoB, homeB, "validate")
	t.Logf("[arm-b] validate:\n%s", validateB)

	countStatusB := mustExtractADRCountStatus(t, statusB, "arm-b")
	scoreContextB := mustExtractContextScore(t, contextB, "arm-b")
	scoreDiscoverB := mustExtractDiscoverScore(t, discoverB, "arm-b")

	// status: arm(b) == arm(a) in ADR count; count must be 0
	if countStatusA != countStatusB {
		t.Errorf("AC2 status: arm(a) ADRs=%s, arm(b) ADRs=%s — NOTAS.md must not count", countStatusA, countStatusB)
	}
	if countStatusB != "0" {
		t.Errorf("AC2 status arm(b): expected ADRs=0, got %s (NOTAS.md must not count)", countStatusB)
	}

	// context: arm(b) score == arm(a) score; "## ADRs (0)" in both
	if scoreContextA != scoreContextB {
		t.Errorf("AC2 context score: arm(a)=%s, arm(b)=%s — NOTAS.md must not change score", scoreContextA, scoreContextB)
	}
	if !strings.Contains(contextA, "## ADRs (0)") {
		t.Errorf("AC2 context arm(a): expected '## ADRs (0)'\n%s", contextA)
	}
	if !strings.Contains(contextB, "## ADRs (0)") {
		t.Errorf("AC2 context arm(b): expected '## ADRs (0)' — NOTAS.md must not count\n%s", contextB)
	}

	// discover: arm(b) score == arm(a) score; both show "No ADRs found"
	if scoreDiscoverA != scoreDiscoverB {
		t.Errorf("AC2 discover score: arm(a)=%s, arm(b)=%s — NOTAS.md must not affect score", scoreDiscoverA, scoreDiscoverB)
	}
	if !strings.Contains(discoverA, "No ADRs found") {
		t.Errorf("AC2 discover arm(a): expected 'No ADRs found'\n%s", discoverA)
	}
	if !strings.Contains(discoverB, "No ADRs found") {
		t.Errorf("AC2 discover arm(b): expected 'No ADRs found' — NOTAS.md must not count\n%s", discoverB)
	}

	// adr list: neither arm lists NOTAS.md; both show "No ADRs found"
	if !strings.Contains(adrListA, "No ADRs found") {
		t.Errorf("AC2 adr list arm(a): expected 'No ADRs found'\n%s", adrListA)
	}
	if !strings.Contains(adrListB, "No ADRs found") {
		t.Errorf("AC2 adr list arm(b): expected 'No ADRs found' — NOTAS.md must not be listed\n%s", adrListB)
	}
	if strings.Contains(adrListB, "NOTAS") {
		t.Errorf("AC2 adr list arm(b): must NOT mention NOTAS.md\n%s", adrListB)
	}

	// validate: arm(b) must not report frontmatter violation for NOTAS.md
	// Old binary: "adr \"NOTAS.md\" has no frontmatter block" (NOTAS.md counted as ADR).
	if strings.Contains(validateB, "NOTAS.md") {
		t.Errorf("AC2 validate arm(b): validate must NOT mention NOTAS.md (not an ADR)\n%s", validateB)
	}

	// ─── arm (c): real ADR — non-vacuity control ──────────────────────────────
	repoC, homeC := makeADRPrefixRepo(t)
	writeADRFile(t, repoC, "ADR-2026-01-01-x.md",
		"---\nstatus: Accepted\ndate: 2026-01-01\nauthor: \"\"\n---\n# ADR: X\n\n> Date: 2026-01-01 | Status: Accepted\n")

	statusC, _ := runTFW(t, repoC, homeC, "status")
	t.Logf("[arm-c] status:\n%s", statusC)

	countStatusC := mustExtractADRCountStatus(t, statusC, "arm-c")

	// arm(c) must differ from arm(a): real ADR is counted
	if countStatusC == countStatusA {
		t.Errorf("AC2 status arm(c): ADRs=%s same as arm(a)=%s — real ADR must be counted", countStatusC, countStatusA)
	}
	if countStatusC != "1" {
		t.Errorf("AC2 status arm(c): expected ADRs=1, got %s", countStatusC)
	}
}

// ─── Test 2: AC3 — lowercase prefix counts ───────────────────────────────────
//
// Reconciliation sentence: the test affirms that adr-001-x.md (lowercase adr- prefix)
// is identified as an ADR by isADRFileName (strings.ToUpper(name) → "ADR-…"), so the
// criterion is case-insensitive and lowercase names are not silently dropped.
//
// Counter-arm (old binary): any .md counts, so adr-001-x.md was already counted.
// This is a regression guard in the restrictive direction. Expected: PASS on both.
func TestADRPrefixE2E_AC3_LowercasePrefixCounts(t *testing.T) {
	repoDir, homeDir := makeADRPrefixRepo(t)
	writeADRFile(t, repoDir, "adr-001-x.md",
		"---\nstatus: Accepted\ndate: 2026-01-01\nauthor: \"\"\n---\n# ADR: X\n\n> Date: 2026-01-01 | Status: Accepted\n")

	out, rc := runTFW(t, repoDir, homeDir, "status")
	t.Logf("AC3 status:\n%s", out)
	if rc != 0 {
		t.Errorf("AC3: trackfw status failed rc=%d", rc)
	}

	count := mustExtractADRCountStatus(t, out, "AC3")
	if count != "1" {
		t.Errorf("AC3: expected ADRs=1 for adr-001-x.md (lowercase prefix must count), got %s", count)
	}

	// adr list must show the file
	listOut, _ := runTFW(t, repoDir, homeDir, "adr", "list")
	t.Logf("AC3 adr list:\n%s", listOut)
	if !strings.Contains(listOut, "adr-001-x.md") {
		t.Errorf("AC3: adr list must show 'adr-001-x.md'\n%s", listOut)
	}
}

// ─── Test 3: AC5 — discover probe (no trackfw.yaml) ignores NOTAS.md ─────────
//
// Reconciliation sentence: the test affirms that the fallback probe in Scan (flat docs/adr
// without trackfw.yaml) uses validator.WalkADRFilePaths — so NOTAS.md produces Governance
// Score identical to an empty docs/adr, instead of crediting ADR count via countMDFiles.
//
// Counter-arm (old binary): fallback probe used countMDFiles → NOTAS.md adds +20 to score.
// Expected: FAIL (scores differ).
func TestADRPrefixE2E_AC5_DiscoverFallbackIgnoresNOTAS(t *testing.T) {
	// Repo WITHOUT trackfw.yaml. discover probes docs/adr directly (flat branch, ~:494).
	makeNoYAMLRepo := func(t *testing.T) (repoDir, homeDir string) {
		t.Helper()
		homeDir = t.TempDir()
		repoDir = t.TempDir()

		gitE2E(t, repoDir, homeDir, "init", "-q", "-b", "main", ".")
		gitE2E(t, repoDir, homeDir, "config", "user.email", "e2e@localhost")
		gitE2E(t, repoDir, homeDir, "config", "user.name", "E2E Test")
		gitE2E(t, repoDir, homeDir, "config", "commit.gpgsign", "false")

		// Create docs/adr (flat, no subdirs) and no trackfw.yaml.
		if err := os.MkdirAll(filepath.Join(repoDir, "docs", "adr"), 0755); err != nil {
			t.Fatalf("mkdirall docs/adr: %v", err)
		}
		gitE2E(t, repoDir, homeDir, "init", "-q", ".")
		return repoDir, homeDir
	}

	// Baseline: empty docs/adr, no trackfw.yaml
	repoEmpty, homeEmpty := makeNoYAMLRepo(t)
	discoverEmpty, _ := runTFW(t, repoEmpty, homeEmpty, "discover")
	t.Logf("[empty] discover:\n%s", discoverEmpty)
	scoreEmpty := mustExtractDiscoverScore(t, discoverEmpty, "empty")

	// Probe: only NOTAS.md in docs/adr, no trackfw.yaml
	repoNotas, homeNotas := makeNoYAMLRepo(t)
	if err := os.WriteFile(
		filepath.Join(repoNotas, "docs", "adr", "NOTAS.md"),
		[]byte("# notas\n"), 0644); err != nil {
		t.Fatalf("write NOTAS.md: %v", err)
	}
	discoverNotas, _ := runTFW(t, repoNotas, homeNotas, "discover")
	t.Logf("[notas] discover:\n%s", discoverNotas)
	scoreNotas := mustExtractDiscoverScore(t, discoverNotas, "notas")

	// Both must show "No ADRs found" and identical score.
	if !strings.Contains(discoverEmpty, "No ADRs found") {
		t.Errorf("AC5 empty: discover must show 'No ADRs found'\n%s", discoverEmpty)
	}
	if !strings.Contains(discoverNotas, "No ADRs found") {
		t.Errorf("AC5 notas: discover must show 'No ADRs found' — NOTAS.md must not credit ADR\n%s", discoverNotas)
	}
	if scoreEmpty != scoreNotas {
		t.Errorf("AC5: discover Governance Score differs: empty=%s, notas=%s — NOTAS.md must not affect score",
			scoreEmpty, scoreNotas)
	}
}

// ─── Test 4: AC6 — adr_file_without_prefix fires for .md with status:, not README ─
//
// Reconciliation sentence: the test affirms that the adr_file_without_prefix warning
// fires for decisao.md (has status: Draft frontmatter, no ADR- prefix) and does NOT
// fire for README.md (no status frontmatter), confirming the guard checks status
// presence and not merely the filename.
//
// Counter-arm (old binary): the rule adr_file_without_prefix did not exist. Expected: FAIL
// (no warning emitted for decisao.md).
func TestADRPrefixE2E_AC6_WithoutPrefixRuleFiresCorrectly(t *testing.T) {
	// Sub-case A: decisao.md with status: Draft → warning must fire
	repoDecisao, homeDecisao := makeADRPrefixRepo(t)
	writeADRFile(t, repoDecisao, "decisao.md",
		"---\nstatus: Draft\ndate: 2026-10-01\nauthor: \"\"\n---\n# Decisão\n\n> Date: 2026-10-01 | Status: Draft\n")

	validateDecisao, _ := runTFW(t, repoDecisao, homeDecisao, "validate")
	t.Logf("AC6 validate (decisao.md with status:Draft):\n%s", validateDecisao)

	if !strings.Contains(validateDecisao, "adr_file_without_prefix") {
		t.Errorf("AC6: validate must emit 'adr_file_without_prefix' for decisao.md (has status: Draft, no ADR- prefix)\n%s", validateDecisao)
	}
	if !strings.Contains(validateDecisao, "decisao.md") {
		t.Errorf("AC6: validate adr_file_without_prefix message must name 'decisao.md'\n%s", validateDecisao)
	}

	// Sub-case B: README.md without status frontmatter → warning must NOT fire
	repoReadme, homeReadme := makeADRPrefixRepo(t)
	writeADRFile(t, repoReadme, "README.md", "# Architecture Decisions\n\nIndex of ADRs.\n")

	validateReadme, _ := runTFW(t, repoReadme, homeReadme, "validate")
	t.Logf("AC6 validate (README.md without status:):\n%s", validateReadme)

	if strings.Contains(validateReadme, "adr_file_without_prefix") {
		t.Errorf("AC6: validate must NOT emit 'adr_file_without_prefix' for README.md (no status frontmatter)\n%s", validateReadme)
	}
	if strings.Contains(validateReadme, "README.md") && strings.Contains(validateReadme, "prefix") {
		t.Errorf("AC6: validate must NOT report any prefix violation for README.md\n%s", validateReadme)
	}
}

// ─── Test 5: AC7 — adr new creates date-slug file; NOTAS.md is not a counter ─
//
// Reconciliation sentence: the test affirms that trackfw adr new uses date-based naming
// (ADR-YYYY-MM-DD-slug.md), independent of the enumeration criterion — so ADR-001/002 and
// NOTAS.md in docs/adr do not shift the generated filename; the new file is identified as
// an ADR (prefix ADR-) while NOTAS.md remains uncounted.
//
// Counter-arm (old binary): same mechanism (NewADR was unchanged). Expected: PASS on both.
// NOTE: This is a regression guard, not a bite proof. The test holds vacuously on both
// binaries. Reported to the architect as an AC7 contract gap: "creates number 003" does
// not match the date-slug naming in generators/adr.go — the correct claim is date-slug
// independence from NOTAS.md.
func TestADRPrefixE2E_AC7_AdrNewUsesDateSlug(t *testing.T) {
	repoDir, homeDir := makeADRPrefixRepo(t)

	// Pre-populate docs/adr with two ADR files (numbered style) and a NOTAS.md.
	adrContent := func(n, date string) string {
		return "---\nstatus: Accepted\ndate: " + date + "\nauthor: \"\"\n---\n# ADR: " + n + "\n"
	}
	writeADRFile(t, repoDir, "ADR-001-first.md", adrContent("First", "2026-01-01"))
	writeADRFile(t, repoDir, "ADR-002-second.md", adrContent("Second", "2026-01-02"))
	writeADRFile(t, repoDir, "NOTAS.md", "# notas\n")

	// Verify baseline: 2 ADRs (not 3, NOTAS.md must not count).
	statusBefore, _ := runTFW(t, repoDir, homeDir, "status")
	t.Logf("AC7 status before:\n%s", statusBefore)
	countBefore := mustExtractADRCountStatus(t, statusBefore, "AC7-before")
	if countBefore != "2" {
		t.Errorf("AC7: expected ADRs=2 before adr new, got %s (NOTAS.md must not count)", countBefore)
	}

	// Run trackfw adr new — not a TTY, wizard skipped; uses just the title arg.
	newOut, newRC := runTFW(t, repoDir, homeDir, "adr", "new", "Third ADR")
	t.Logf("AC7 adr new output:\n%s (rc=%d)", newOut, newRC)
	if newRC != 0 {
		t.Fatalf("AC7: trackfw adr new failed rc=%d\n%s", newRC, newOut)
	}

	// Output must say "created" and contain "ADR-".
	if !strings.Contains(newOut, "created") {
		t.Errorf("AC7: adr new output must contain 'created'\n%s", newOut)
	}
	if !strings.Contains(newOut, "ADR-") {
		t.Errorf("AC7: adr new output must name the created file (starting with ADR-)\n%s", newOut)
	}

	// Verify: the created file matches ADR-*-third-adr.md in docs/adr/.
	adrDir := filepath.Join(repoDir, "docs", "adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("AC7: ReadDir docs/adr: %v", err)
	}
	var newADRs []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(strings.ToUpper(name), "ADR-") && strings.HasSuffix(name, ".md") &&
			name != "ADR-001-first.md" && name != "ADR-002-second.md" {
			newADRs = append(newADRs, name)
		}
	}
	if len(newADRs) != 1 {
		t.Errorf("AC7: expected exactly 1 new ADR-*.md, found: %v", newADRs)
	} else {
		newName := newADRs[0]
		t.Logf("AC7: created file = %s", newName)
		// Must be date-based: ADR-YYYY-MM-DD-third-adr.md (not ADR-003-…).
		if strings.HasPrefix(newName, "ADR-003") {
			t.Errorf("AC7: file must use date-based naming (ADR-YYYY-MM-DD-…), not sequential (got %s)", newName)
		}
		if !strings.Contains(newName, "third-adr") {
			t.Errorf("AC7: expected slug 'third-adr' in filename, got %s", newName)
		}
	}

	// After creation: status must show 3 ADRs (not 4; NOTAS.md still not counted).
	statusAfter, _ := runTFW(t, repoDir, homeDir, "status")
	t.Logf("AC7 status after:\n%s", statusAfter)
	countAfter := mustExtractADRCountStatus(t, statusAfter, "AC7-after")
	if countAfter != "3" {
		t.Errorf("AC7: expected ADRs=3 after adr new (2 original + 1 new), got %s (NOTAS.md must still not count)", countAfter)
	}
}
