package validator

// ML-2A (ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-
// que-o-pr-edita): tests for AC2, AC3, AC4, Ação 0, Ação 1 and AC8(a).
// Every test below declares, in its reconciliation comment, which ML-2A conclusion it asserts.

import (
	"os/exec"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// ── AC2 / AC8(a) ────────────────────────────────────────────────────────────────────────────────

// TestIsLenientFor asserts the pure leniency decision function (extracted for clock-free testing).
// Reconciliação: este teste prova que isLenientFor implementa as três rejeições do AC2 —
// ausência de prazo → strict; prazo no futuro → leniente; prazo além do horizonte → strict como
// ausente — e o contra-braço do AC8(a): o input (time.Time zero) que antes retornava true
// agora retorna false, confirmando que a correção é um estado inversão e não uma regressão.
func TestIsLenientFor(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		mode  string
		until time.Time
		want  bool
	}{
		// Arm 0: non-lenient mode is always strict regardless of date.
		{"strict_mode_ignores_date", "strict", now.AddDate(0, 0, 30), false},

		// Arm 1 (AC8a arm 1 / AC2): absent lenient_until → strict.
		// AC8(a) counter-arm: pre-fix IsLenient() returned true on this exact input (zero
		// time) because the code read `if gm.LenientUntil.IsZero() { return true }`.
		// The fix inverts this: zero time → false. The test is the runnable counter-arm.
		{"lenient_no_date_is_strict", "lenient", time.Time{}, false},

		// Arm 2: past deadline → expired, strict.
		{"lenient_expired", "lenient", now.Add(-24 * time.Hour), false},

		// Arm 3 (AC8a arm 2): valid future date within horizon → lenient.
		{"lenient_valid_future", "lenient", now.AddDate(0, 0, 30), true},

		// Arm 4 (AC2 ceiling): date exactly at horizon boundary (not before horizon) → rejected.
		{"lenient_at_horizon_exact", "lenient", now.AddDate(0, 0, config.LenientHorizonDays), false},

		// Arm 5 (AC2 ceiling): date beyond horizon → treated same as absent → strict.
		{"lenient_beyond_horizon", "lenient", time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), false},

		// Arm 6: date one day before the horizon boundary → still accepted.
		{"lenient_one_day_before_horizon", "lenient", now.AddDate(0, 0, config.LenientHorizonDays-1), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLenientFor(tc.mode, tc.until, now)
			if got != tc.want {
				t.Errorf("isLenientFor(%q, %v, now) = %v, want %v", tc.mode, tc.until, got, tc.want)
			}
		})
	}
}

// TestLenientDefaultDays verifies that the shared constant matches the brownfield default written
// by `trackfw init`. If they diverge, projects onboarded via discover get a different horizon
// than projects onboarded via init — a contract inconsistency.
// Reconciliação: este teste prova que config.LenientDefaultDays é o único ponto de definição
// do prazo padrão; alterá-lo aqui quebra init e discover simultaneamente, forçando consistência.
func TestLenientDefaultDays(t *testing.T) {
	if config.LenientDefaultDays != 30 {
		t.Errorf("LenientDefaultDays = %d, want 30 (must match trackfw init brownfield default)",
			config.LenientDefaultDays)
	}
}

// isLenientPreFix reproduces the pre-ML-2A decision verbatim (validator.go before this wave):
//
//	if gm.LenientUntil.IsZero() { return true }
//
// It exists ONLY as the AC8(a) counter-arm in TestAC8a_ThreeArmFalsification: it proves that the
// old code classified "no deadline" as lenient, so the new behaviour is a real inversion, not a
// no-op. Never use this function outside of that test.
func isLenientPreFix(mode string, until time.Time, now time.Time) bool {
	if mode != "lenient" {
		return false
	}
	if until.IsZero() {
		return true // pre-fix bug: absent deadline was treated as lenient-forever
	}
	return now.Before(until)
}

// TestAC8a_ThreeArmFalsification is the runnable AC8(a) three-arm falsification test.
//   - Arm (a1): no deadline + post-fix code → strict (the fix)
//   - Arm (a2): valid future deadline + post-fix code → lenient (positive confirmation)
//   - Arm (a3): no deadline + pre-fix code → lenient (counter-arm; t.Fatal if vacuous)
//
// The t.Fatal on arm (a3) is the load-bearing assertion: if the pre-fix function no longer
// returns true for this input the counter-arm has gone vacuous and arm (a1) proves nothing.
// Reconciliação: este teste prova, com código executável, que o input (mode="lenient",
// until=zero) produzia dois resultados opostos antes e depois da correção — confirmando que
// a mudança é uma inversão de comportamento, não uma refatoração sem efeito observável.
func TestAC8a_ThreeArmFalsification(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	noDeadline := time.Time{}
	futureDeadline := now.AddDate(0, 0, 30)

	// Arm (a1): post-fix, no deadline → strict.
	if isLenientFor("lenient", noDeadline, now) {
		t.Error("arm a1 failed: post-fix isLenientFor with no deadline must be strict")
	}

	// Arm (a2): post-fix, valid future deadline → lenient.
	if !isLenientFor("lenient", futureDeadline, now) {
		t.Error("arm a2 failed: post-fix isLenientFor with valid future deadline must be lenient")
	}

	// Arm (a3) COUNTER-ARM: pre-fix, no deadline → lenient.
	// t.Fatal here (not t.Error): if the counter-arm goes vacuous (pre-fix function returns
	// false), arm (a1) no longer proves anything — it could be comparing two identical behaviours.
	if !isLenientPreFix("lenient", noDeadline, now) {
		t.Fatal("arm a3 (counter-arm) vacuous: pre-fix isLenientPreFix no longer returns true " +
			"for no-deadline input — the counter-arm must always return true or it proves nothing")
	}
}

// ── AC4 — lenient carve-out ──────────────────────────────────────────────────────────────────────

// TestApplyLenientWithCarveout_KeepsCarveoutViolations verifies that rules in the named closed
// carve-out set remain violations in lenient mode; non-carve-out violations become warnings.
// Reconciliação: este teste prova que applyLenientWithCarveout separa corretamente o conjunto
// fechado {req_roadmap_lifecycle, ref_targets_exist} de qualquer outra regra, incluindo violações
// sem tag (Rule=""), que são movidas para warnings — o comportamento documentado no código.
func TestApplyLenientWithCarveout_KeepsCarveoutViolations(t *testing.T) {
	violations := []TaggedMsg{
		{Rule: "req_roadmap_lifecycle", Msg: "Open REQ with done roadmap"},
		{Rule: "ref_targets_exist", Msg: "referenced roadmap does not exist"},
		{Rule: "wip_limit", Msg: "WIP limit exceeded"},
		{Rule: "", Msg: "frontmatter: missing required field"},
	}
	warnings := []TaggedMsg{}

	gotV, gotW := applyLenientWithCarveout(violations, warnings)

	// Carve-out rules must remain violations.
	if len(gotV) != 2 {
		t.Errorf("expected 2 carve-out violations, got %d: %v", len(gotV), gotV)
	}
	for _, v := range gotV {
		if !lenientCarveoutRules[v.Rule] {
			t.Errorf("non-carve-out rule %q leaked into violations", v.Rule)
		}
	}

	// Non-carve-out violations (wip_limit and Rule="") must become warnings.
	if len(gotW) != 2 {
		t.Errorf("expected 2 warnings (wip_limit + untagged), got %d: %v", len(gotW), gotW)
	}
}

// TestLenientCarveoutRules_ClosedSet verifies the exact membership of the carve-out set.
// Reconciliação: este teste prova que o conjunto fechado tem exatamente dois membros —
// req_roadmap_lifecycle e ref_targets_exist — correspondendo às 8 violações ativas / 0 históricas
// calibradas em 2026-09-17. Adicionar uma regra ao conjunto exige atualizar este teste.
func TestLenientCarveoutRules_ClosedSet(t *testing.T) {
	wantRules := []string{"req_roadmap_lifecycle", "ref_targets_exist"}
	if len(lenientCarveoutRules) != len(wantRules) {
		t.Errorf("lenientCarveoutRules has %d members, want %d (%v)",
			len(lenientCarveoutRules), len(wantRules), wantRules)
	}
	for _, r := range wantRules {
		if !lenientCarveoutRules[r] {
			t.Errorf("rule %q must be in lenientCarveoutRules", r)
		}
	}
}

// TestLenientMode_Validate_CarveoutViolationSurvives tests that req_roadmap_lifecycle violations
// survive lenient mode through the full Validate() path (not just the unit helper above).
// Uses a mktemp-dir fixture with governance_mode: lenient + valid future lenient_until to avoid
// touching this repository's trackfw.yaml (Wave 3 scope).
// Reconciliação: este teste prova que a integração de AC2 (IsLenient com prazo) + AC4 (carve-out)
// funciona no caminho completo de Validate() — uma violação de req_roadmap_lifecycle com
// lenient ativo ainda aparece em violations, não em warnings.
func TestLenientMode_Validate_CarveoutViolationSurvives(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/roadmaps/done", "docs/adr")

	// Set a future lenient_until so IsLenient() returns true.
	futureDate := time.Now().AddDate(0, 0, 60).Format("2006-01-02")

	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n"+
			"governance_mode: lenient\nlenient_until: "+futureDate+"\n")

	// Done roadmap: forces a lifecycle violation — Open REQ with done roadmap.
	writeFile(t, dir, "docs/roadmaps/done/DONE-ROADMAP-lifecycle.md",
		"---\nstatus: done\ndate: 2026-01-01\n---\n# Roadmap done\n## Acceptance Criteria\n- [x] x\n")

	// Open REQ pointing to the done roadmap — this is the lifecycle contradiction.
	writeFile(t, dir, "docs/req/REQ-open.md",
		"---\nstatus: Open\nroadmap: \"docs/roadmaps/done/DONE-ROADMAP-lifecycle.md\"\n---\n\n"+
			"# REQ: Open\n\n> Date: 2026-01-01 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: `docs/roadmaps/done/DONE-ROADMAP-lifecycle.md`\n")

	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	// The lifecycle violation must remain a violation even in lenient mode (AC4 carve-out).
	// Message pattern: `req "REQ-open.md" is Open but linked Roadmap "..." is in done/`
	lifecycleViolation := hasViolation(violations, "is Open but linked Roadmap")
	if !lifecycleViolation {
		t.Errorf("expected req_roadmap_lifecycle violation (pattern 'is Open but linked Roadmap') "+
			"to survive lenient mode (AC4 carve-out)\nviolations=%v\nwarnings=%v",
			violations, warnings)
	}

	// Must NOT appear in warnings (it must be a real violation, not silenced by lenient).
	if hasWarning(warnings, "is Open but linked Roadmap") {
		t.Errorf("lifecycle finding must not be in warnings when carve-out is active; "+
			"warnings=%v", warnings)
	}
}

// ── Ação 0 — deriveOriginDefaultBranch filter ───────────────────────────────────────────────────

// TestDeriveOriginDefaultBranch_OriginHEADIsFiltered verifies that when origin/HEAD is present
// (as a symbolic ref to the default branch), the %(refname:short) output "origin" is correctly
// filtered and the single real branch is still derived unambiguously.
// Reconciliação: este teste prova que o filtro corrigido (line == "origin") elimina o output
// "origin" de %(refname:short) para refs/remotes/origin/HEAD, de modo que um repositório com
// um único branch não-main ainda resolve via derivação unambígua.
func TestDeriveOriginDefaultBranch_OriginHEADIsFiltered(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	originDir := t.TempDir()
	runIn := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = d
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %s", args, d, out)
		}
	}

	// Create a remote with branch "trunk" (non-main, non-master) so the single-branch
	// path is exercised — it must be derived correctly after filtering "origin".
	runIn(originDir, "init", "-b", "trunk")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	writeFile(t, originDir, "trackfw.yaml", "req_dir: docs/req\n")
	runIn(originDir, "add", "trackfw.yaml")
	runIn(originDir, "config", "commit.gpgsign", "false")
	runIn(originDir, "commit", "-m", "init")

	// Add as "origin" and fetch, then set origin/HEAD (symbolic ref).
	// After this, `git for-each-ref --format=%(refname:short) refs/remotes/origin/` emits:
	//   origin        ← the %(refname:short) of refs/remotes/origin/HEAD (pre-fix: not filtered)
	//   origin/trunk  ← the real branch
	runIn(dir, "remote", "add", "origin", originDir)
	runIn(dir, "fetch", "--no-tags", "origin", "+refs/heads/trunk:refs/remotes/origin/trunk")
	runIn(dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")

	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	branch, ok := deriveOriginDefaultBranch()
	if !ok {
		t.Fatalf("deriveOriginDefaultBranch() returned ok=false; expected to derive 'origin/trunk' "+
			"from single real branch after filtering 'origin' (origin/HEAD short form)")
	}
	if branch != "origin/trunk" {
		t.Errorf("deriveOriginDefaultBranch() = %q, want %q", branch, "origin/trunk")
	}
}

// ── Ação 1 — req_roadmap_lifecycle routing ──────────────────────────────────────────────────────

// TestReqRoadmapLifecycle_RoutedThroughApplyRule verifies that after the fix, a lifecycle
// mismatch appears in violations (not only warnings) when the rule defaults to "error" (which it
// does because req_roadmap_lifecycle is absent from ruleDefaults, so it falls through to "error").
// Reconciliação: este teste prova que req_roadmap_lifecycle agora passa por applyRule/
// applyRuleTagged em vez de ser anexado diretamente a warnings — a regra pode se tornar
// violação sob configuração error (seu default), algo impossível antes da correção.
func TestReqRoadmapLifecycle_RoutedThroughApplyRule(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/roadmaps/done", "docs/adr")

	// Done roadmap + open REQ pointing to it → lifecycle contradiction.
	writeFile(t, dir, "docs/roadmaps/done/DONE-ROADMAP-lc.md",
		"---\nstatus: done\ndate: 2026-01-01\n---\n# Done\n## Acceptance Criteria\n- [x] ok\n")
	writeFile(t, dir, "docs/req/REQ-open-lc.md",
		"---\nstatus: Open\nroadmap: \"docs/roadmaps/done/DONE-ROADMAP-lc.md\"\n---\n\n"+
			"# REQ: Lifecycle\n\n> Date: 2026-01-01 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: `docs/roadmaps/done/DONE-ROADMAP-lc.md`\n")

	// No governance_mode or rules: config — defaults apply. req_roadmap_lifecycle is NOT in
	// ruleDefaults so it falls through to "error". After the fix, it must appear in violations.
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")

	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}

	// After fix: req_roadmap_lifecycle must appear in violations (not only warnings).
	// Message pattern: `req "..." is Open but linked Roadmap "..." is in done/`
	if !hasViolation(violations, "is Open but linked Roadmap") {
		t.Errorf("req_roadmap_lifecycle mismatch must appear in violations after Ação 1 fix "+
			"(pattern 'is Open but linked Roadmap'); violations=%v warnings=%v",
			violations, warnings)
	}
}

// TestReqRoadmapLifecycle_WarningSeverityRespected verifies that setting
// rules: {req_roadmap_lifecycle: warning} moves the finding to warnings instead of violations —
// confirming that the routing now passes through ruleSeverity() and the config is respected.
// Reconciliação: este teste prova que a configuração rules: {req_roadmap_lifecycle: warning}
// tem efeito agora que a regra passa por applyRule — antes da correção, a regra sempre ia para
// warnings independente de qualquer configuração.
func TestReqRoadmapLifecycle_WarningSeverityRespected(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/roadmaps/done", "docs/adr")

	writeFile(t, dir, "docs/roadmaps/done/DONE-ROADMAP-w.md",
		"---\nstatus: done\ndate: 2026-01-01\n---\n# Done\n## Acceptance Criteria\n- [x] ok\n")
	writeFile(t, dir, "docs/req/REQ-open-w.md",
		"---\nstatus: Open\nroadmap: \"docs/roadmaps/done/DONE-ROADMAP-w.md\"\n---\n\n"+
			"# REQ: Lifecycle W\n\n> Date: 2026-01-01 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: `docs/roadmaps/done/DONE-ROADMAP-w.md`\n")

	// Explicitly set req_roadmap_lifecycle to warning — must be obeyed.
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n"+
			"rules:\n  req_roadmap_lifecycle: warning\n")

	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}

	if !hasWarning(warnings, "is Open but linked Roadmap") {
		t.Errorf("rules: {req_roadmap_lifecycle: warning} must route finding to warnings; "+
			"violations=%v warnings=%v", violations, warnings)
	}
	if hasViolation(violations, "is Open but linked Roadmap") {
		t.Errorf("finding must not appear in violations when severity is warning; "+
			"violations=%v", violations)
	}
}
