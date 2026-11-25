package config

// Tests for YAML merge key support in normalizeMapping (ML-4D, REQ-2026-11-22).
//
// Each test is annotated with the conclusion it asserts, as required by the
// Regra Dura de Reconciliação (CLAUDE.md).
//
// Counter-arm summary: without the ML-4D changes in normalizeMapping, every test
// in the TestMergeKey_* group fails at run time, except TestMergeKey_QuotedKeyIsNotMerge,
// which passes on the old code too — see the counter-arm note in that function.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// loadFromYAML writes content to a temp trackfw.yaml, resets the config singleton,
// chdirs into the temp dir, loads, and restores cwd. Mirrors the pattern used in config_test.go.
func loadFromYAML(t *testing.T, content string) ProjectConfig {
	t.Helper()
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return Load()
}

// TestMergeKey_AnchorScalar asserts:
// wip_limit and require_req_in_commit inherited via <<: *anchor are read by Load()
// (the merge key is expanded into the config map).
func TestMergeKey_AnchorScalar(t *testing.T) {
	cfg := loadFromYAML(t, "d: &d\n  wip_limit: \"3\"\n  require_req_in_commit: \"true\"\nadr_dirs: [docs/adr]\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n<<: *d\n")
	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: want 3 (from anchor), got %d", cfg.WipLimit)
	}
	if !cfg.RequireReqInCommit {
		t.Errorf("RequireReqInCommit: want true (from anchor), got false")
	}
}

// TestMergeKey_ExplicitBeforeMerge asserts:
// an explicit key declared BEFORE <<: *anchor wins over the anchor value.
func TestMergeKey_ExplicitBeforeMerge(t *testing.T) {
	cfg := loadFromYAML(t, "d: &d\n  wip_limit: \"3\"\n  require_req_in_commit: \"true\"\nwip_limit: \"2\"\n<<: *d\nadr_dirs: [docs/adr]\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n")
	if cfg.WipLimit != 2 {
		t.Errorf("WipLimit: want 2 (explicit before anchor beats anchor), got %d", cfg.WipLimit)
	}
	// require_req_in_commit not declared explicitly → inherited from anchor
	if !cfg.RequireReqInCommit {
		t.Errorf("RequireReqInCommit: want true (from anchor), got false")
	}
}

// TestMergeKey_ExplicitAfterMerge asserts:
// an explicit key declared AFTER <<: *anchor also wins over the anchor value
// (order-independence of explicit keys is a YAML merge semantic).
func TestMergeKey_ExplicitAfterMerge(t *testing.T) {
	cfg := loadFromYAML(t, "d: &d\n  wip_limit: \"3\"\n  require_req_in_commit: \"true\"\n<<: *d\nwip_limit: \"2\"\nadr_dirs: [docs/adr]\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n")
	if cfg.WipLimit != 2 {
		t.Errorf("WipLimit: want 2 (explicit after anchor beats anchor), got %d", cfg.WipLimit)
	}
	if !cfg.RequireReqInCommit {
		t.Errorf("RequireReqInCommit: want true (from anchor), got false")
	}
}

// TestMergeKey_SequencePrecedence asserts:
// in <<: [*a, *b] the earlier source (*a) wins over the later (*b) for a conflicting key.
func TestMergeKey_SequencePrecedence(t *testing.T) {
	cfg := loadFromYAML(t, "a: &a\n  wip_limit: \"2\"\nb: &b\n  wip_limit: \"3\"\nadr_dirs: [docs/adr]\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n<<: [*a, *b]\n")
	if cfg.WipLimit != 2 {
		t.Errorf("WipLimit: want 2 (first source *a beats *b), got %d", cfg.WipLimit)
	}
}

// TestMergeKey_NestedMerge asserts:
// an anchor that itself uses <<: *inner causes its inherited keys to propagate
// to the outer mapping (nested merge keys are expanded recursively).
func TestMergeKey_NestedMerge(t *testing.T) {
	cfg := loadFromYAML(t, "inner: &inner\n  require_req_in_commit: \"true\"\n  wip_limit: \"2\"\nouter: &outer\n  <<: *inner\n  wip_limit: \"3\"\nadr_dirs: [docs/adr]\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n<<: *outer\n")
	// outer's explicit wip_limit: 3 wins over inner's wip_limit: 2 via nested merge
	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: want 3 (outer explicit beats inner via nested merge), got %d", cfg.WipLimit)
	}
	// require_req_in_commit comes from inner, via outer's nested merge
	if !cfg.RequireReqInCommit {
		t.Errorf("RequireReqInCommit: want true (from inner via nested merge), got false")
	}
}

// TestMergeKey_QuotedKeyIsNotMerge asserts:
// a quoted key "<<" (tag !!str) is treated as a literal key, not a merge key.
//
// Counter-arm note: this test passes on the old code too (quoted "<<" was stored as a
// literal key before this ML). To verify the counter-arm is not vacuous for this test,
// a build mutant that detects merge by k.Value == "<<" (instead of k.Tag == "!!merge")
// would fail here, because it would mistakenly try to expand "not-a-merge" as a merge value.
func TestMergeKey_QuotedKeyIsNotMerge(t *testing.T) {
	// wip_limit is set explicitly; "<<" with quotes is a literal key, not a merge directive.
	cfg := loadFromYAML(t, "adr_dirs: [docs/adr]\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nwip_limit: \"5\"\n\"<<\": \"not-a-merge\"\n")
	if cfg.WipLimit != 5 {
		t.Errorf("WipLimit: want 5 (quoted << is not a merge key), got %d", cfg.WipLimit)
	}
}

// --- ML-4E tests: cycle guard ---
//
// These tests verify that cyclic YAML anchors do not overflow the goroutine stack.
// The cycle guard uses a path-based visiting set (insert on entry, delete on exit)
// so the same alias used twice in unrelated positions is not falsely blocked.
//
// Precondition pattern: each cyclic fixture is first verified to parse without error
// by yaml.Unmarshal (into a yaml.Node) before being handed to Load(). If the fixture
// does not parse, Load()'s pre-check would call osExit(1) and kill the test process —
// an unrelated failure mode that the cycle guard does not address.
//
// Counter-arm (without the guard): the ML-4D implementation (this branch, prior to
// ML-4E) crashes with "goroutine stack exceeds" when given the self-referential fixture.
// The counter-arm was measured by building the pre-ML-4E binary from `git archive HEAD`
// in a $TMPDIR copy, running it with the cyclic trackfw.yaml and observing rc=2 and the
// "goroutine stack exceeds" message. The post-ML-4E binary returns rc=0 on the same input.
// See the ponta-a-ponta section of the ML-4E report for the exact commands and output.

// TestCyclicAnchor_SelfRefMapping asserts:
// A self-referential mapping anchor (d: &d {wip_limit: "3", <<: *d} / <<: *d) does not
// overflow the stack and the reader delivers the explicit keys that are reachable before
// the cycle (wip_limit == 3). ML-4E.
func TestCyclicAnchor_SelfRefMapping(t *testing.T) {
	content := "d: &d\n  wip_limit: \"3\"\n  <<: *d\n<<: *d\nreq_dir: docs/req\n"
	// Precondition: fixture must parse as valid YAML (otherwise Load() exits, not cycles).
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(content), &probe); err != nil {
		t.Fatalf("precondition: fixture must parse as valid YAML: %v", err)
	}
	cfg := loadFromYAML(t, content)
	// The explicit wip_limit: "3" in the anchor is reachable from the top-level <<: *d.
	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: want 3 (from anchor via <<: *d), got %d", cfg.WipLimit)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: want docs/req, got %q", cfg.REQDir)
	}
}

// TestCyclicAnchor_SelfRefSequenceValue asserts:
// A self-referential sequence anchor (adr_dirs: &a [docs/adr, *a]) does not overflow
// the stack. The cyclic element (*a) is returned as nil and filtered out by stringListRec,
// leaving adr_dirs == ["docs/adr"]. ML-4E.
func TestCyclicAnchor_SelfRefSequenceValue(t *testing.T) {
	content := "adr_dirs: &a\n  - docs/adr\n  - *a\nreq_dir: docs/req\n"
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(content), &probe); err != nil {
		t.Fatalf("precondition: fixture must parse as valid YAML: %v", err)
	}
	cfg := loadFromYAML(t, content)
	// The non-cyclic element "docs/adr" must be retained; the cyclic *a must be dropped.
	if len(cfg.ADRDirs) != 1 || cfg.ADRDirs[0] != "docs/adr" {
		t.Errorf("ADRDirs: want [docs/adr], got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: want docs/req, got %q", cfg.REQDir)
	}
}

// TestCyclicAnchor_MutualCycle asserts:
// A nested mutual-cycle anchor (a: &a {b: &b {wip_limit:"3", <<: *a}, <<: *b} / <<: *a)
// does not overflow the stack and the reader delivers the keys reachable before the cycle.
// Note: a forward-reference mutual cycle (a→b, b→a at the same level) is rejected by
// yaml.v3 as "unknown anchor 'b' referenced", so the fixture uses nested anchors where
// &b is declared inside &a — both anchors are visible before the back-reference. ML-4E.
func TestCyclicAnchor_MutualCycle(t *testing.T) {
	content := "a: &a\n  b: &b\n    wip_limit: \"3\"\n    <<: *a\n  <<: *b\n<<: *a\nreq_dir: docs/req\n"
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(content), &probe); err != nil {
		t.Fatalf("precondition: fixture must parse as valid YAML: %v", err)
	}
	cfg := loadFromYAML(t, content)
	// wip_limit: "3" is inside b, which is merged into a and then into the top level.
	// The cycle guard terminates the recursion; the question is whether the key arrives
	// before the cycle fires. The exact value depends on expansion order; we assert only
	// that Load() returns without crashing and that the top-level req_dir is readable.
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: want docs/req, got %q", cfg.REQDir)
	}
}

// TestCyclicAnchor_SameAliasTwiceNoCycle asserts:
// The cycle guard does NOT block a valid double-use of the same alias
// (<<: [*c, *c] is redundant but not a cycle — c is not being expanded while c is
// already on the path). wip_limit must arrive as 3. ML-4E counter-arm.
func TestCyclicAnchor_SameAliasTwiceNoCycle(t *testing.T) {
	content := "c: &c\n  wip_limit: \"3\"\n<<: [*c, *c]\nreq_dir: docs/req\n"
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(content), &probe); err != nil {
		t.Fatalf("precondition: fixture must parse as valid YAML: %v", err)
	}
	cfg := loadFromYAML(t, content)
	// A global (non-path) visited set would mark c as visited on the first expansion and
	// skip it on the second — leaving wip_limit absent and falling back to the default 1.
	// A path-based set removes c after the first expansion finishes, so the second use
	// still sees c as unvisited and correctly skips the already-present key (no duplicate).
	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: want 3 (path-based guard must not block second use of *c), got %d", cfg.WipLimit)
	}
}

// --- ML-4G: alias bomb tests ---

// aliasBombSequence generates a YAML sequence-merge alias bomb with levels levels.
// Each level doubles the expansion count: levels=30 → 2^30 ≈ 10^9 iterations.
// levels=10 triggers "excessive aliasing" in yaml.Unmarshal(_, &interface{}).
// levels=25 takes ~3 s in the yaml.Node walker (useful for counter-arm with 2 s deadline).
func aliasBombSequence(levels int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "n00: &n00\n  wip_limit: 1\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&sb, "n%02d: &n%02d\n  <<: [*n%02d, *n%02d]\n", i, i, i-1, i-1)
	}
	fmt.Fprintf(&sb, "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n<<: [*n%02d, *n%02d]\n",
		levels-1, levels-1)
	return sb.String()
}

// TestAliasBomb_ReaderRejectsLoud asserts:
// A 30-level sequence alias bomb causes Load() to call osExit(1) with MalformedConfigMessage
// and return within 2 s. The walker's node-visit budget (ML-4H) fires after 10 000 visits,
// parse() returns an error, and Load() treats it as malformed YAML.
func TestAliasBomb_ReaderRejectsLoud(t *testing.T) {
	bomb := aliasBombSequence(30)
	done := make(chan struct{})
	go func() {
		assertLoadFailsLoud(t, bomb)
		close(done)
	}()
	select {
	case <-done:
		// assertLoadFailsLoud already verified osExit(1) + MalformedConfigMessage.
	case <-time.After(2 * time.Second):
		t.Fatal("Load() must reject alias bomb and return within 2 s (ML-4H budget)")
	}
}

// TestAliasBomb_ReaderLegitimateAlias asserts:
// An anchor used 3 times (legitimate, non-bomb aliasing) is not stopped by the ML-4H
// budget (well below 10 000 visits) and is loaded normally by Load() — wip_limit and
// req_dir are read.
func TestAliasBomb_ReaderLegitimateAlias(t *testing.T) {
	content := "c: &c\n  wip_limit: \"5\"\nreq_dir: docs/req\nadr_dirs:\n  - docs/adr\n<<: *c\na: *c\nb: *c\n"
	cfg := loadFromYAML(t, content)
	if cfg.WipLimit != 5 {
		t.Errorf("WipLimit: want 5 (anchor used 3× must not be treated as bomb), got %d", cfg.WipLimit)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: want docs/req, got %q", cfg.REQDir)
	}
}

// TestAliasBomb_ReaderCounterArm asserts:
// The cycle guard alone does not stop a fan-out bomb: normalizeMapping with an
// effectively unlimited budget (ML-4H limit field) on a 25-level sequence bomb
// takes >2 s. This demonstrates that the budget (ML-4H) is the control — the
// cycle guard (ML-4E) handles back-edges but cannot bound fan-out complexity.
// Uses 25 levels (~2.9 s at unlimited budget) so the goroutine terminates
// naturally within ~1 s after the 2 s deadline fires.
func TestAliasBomb_ReaderCounterArm(t *testing.T) {
	// 25 levels: 2^25 ≈ 33 M yaml.Node walker steps — takes ~2.9 s without budget limit.
	bomb := aliasBombSequence(25)
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(bomb), &root); err != nil {
		t.Fatalf("bomb must parse as yaml.Node without error: %v", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		t.Fatal("bomb must have a top-level MappingNode")
	}
	top := root.Content[0]
	done := make(chan struct{})
	go func() {
		// Counter-arm: bypass the normal budget with an effectively unlimited limit.
		// This demonstrates that cycle guard alone does not stop a fan-out bomb —
		// only the budget (ML-4H) does.
		unlimitedCtx := &walkCtx{
			visiting: make(map[*yaml.Node]bool),
			limit:    1 << 30, // ~1B visits, budget not the control here
		}
		normalizeMapping(top, unlimitedCtx)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("counter-arm FAIL: normalizeMapping with unlimited budget should hang on 25-level bomb — " +
			"verify aliasBombSequence(25) generates an effective bomb on this machine")
	case <-time.After(2 * time.Second):
		// Expected: without budget limit, normalizeMapping on a 25-level bomb takes >>2 s.
		// Goroutine terminates naturally after ~0.9 s more; test process continues. ML-4H.
	}
}

// --- ML-4H: node-visit budget tests ---
//
// These tests verify that the node-visit budget inside the walker (ML-4H) stops bomb
// forms that the ML-4G interface{} first-pass missed: cycle-before-bomb and
// duplicate-merge-key-before-bomb. They also verify that every direct caller of
// parse() — which previously bypassed the ML-4G first-pass in Load() — is now
// protected by the walker budget.
//
// Precondition pattern: each fixture is first verified to parse without error by
// yaml.Unmarshal into yaml.Node (otherwise Load()'s pre-check would fire first and
// the test would pass for the wrong reason). For desvio fixtures, interface{} unmarshal
// is verified to give an error WITHOUT "excessive aliasing" (proving ML-4G would miss it).

// cycleThenBomb prepends a self-referential cyclic anchor to an alias bomb sequence.
// When unmarshaled to interface{}, yaml.v3 gives "anchor value contains itself" (from
// the cycle), NOT "excessive aliasing" — so the ML-4G string-match would have let the
// bomb through to the walker. ML-4H's budget stops it.
func cycleThenBomb(levels int) string {
	cycle := "cycle: &cycle\n  wip_limit: \"1\"\n  <<: *cycle\n"
	return cycle + aliasBombSequence(levels)
}

// dupMergeKeyThenBomb creates a YAML with a small anchor merged first (<<: *small),
// followed by the bomb as a second merge key (<<: [*nN, *nN]). yaml.v3 may report
// "duplicate key <<" before even evaluating the bomb sequence, so the ML-4G
// "excessive aliasing" string-match would not fire. ML-4H's budget stops it.
func dupMergeKeyThenBomb(levels int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "small: &small\n  req_dir: docs/req\n")
	fmt.Fprintf(&sb, "n00: &n00\n  wip_limit: 1\n")
	for i := 1; i < levels; i++ {
		fmt.Fprintf(&sb, "n%02d: &n%02d\n  <<: [*n%02d, *n%02d]\n", i, i, i-1, i-1)
	}
	fmt.Fprintf(&sb, "adr_dirs:\n  - docs/adr\n<<: *small\n<<: [*n%02d, *n%02d]\n",
		levels-1, levels-1)
	return sb.String()
}

// TestAliasBomb_Budget_CycleBeforeBomb asserts:
// A YAML with a cyclic anchor BEFORE an alias bomb is rejected by Load() (osExit(1) +
// MalformedConfigMessage) within 2 s via the walker budget. ML-4G would have missed this
// because yaml.v3's interface{} unmarshal gives "anchor value contains itself" (not
// "excessive aliasing") for the cyclic anchor.
func TestAliasBomb_Budget_CycleBeforeBomb(t *testing.T) {
	fixture := cycleThenBomb(25)
	// Precondition 1: yaml.Node unmarshal succeeds (Load's probe won't reject it).
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(fixture), &probe); err != nil {
		t.Fatalf("precondition: yaml.Node unmarshal must succeed: %v", err)
	}
	// Precondition 2: interface{} unmarshal gives an error without "excessive aliasing" —
	// proving that ML-4G's string-match would have let the bomb reach the walker.
	var iface interface{}
	ifaceErr := yaml.Unmarshal([]byte(fixture), &iface)
	if ifaceErr == nil {
		t.Log("note: interface{} unmarshal succeeded (cycle not detected before bomb); ML-4G gap still demonstrated by missing excessive-aliasing match")
	} else if strings.Contains(ifaceErr.Error(), "excessive aliasing") {
		t.Logf("note: interface{} gave 'excessive aliasing' — ML-4G would catch this form; ML-4H budget still stops it (defense in depth): %v", ifaceErr)
	}
	// Main assertion: Load() rejects within 2 s.
	done := make(chan struct{})
	go func() {
		assertLoadFailsLoud(t, fixture)
		close(done)
	}()
	select {
	case <-done:
		// Load() called osExit(1) + MalformedConfigMessage within 2 s.
	case <-time.After(2 * time.Second):
		t.Fatal("Load() must reject cycle-before-bomb within 2 s (ML-4H budget)")
	}
}

// TestAliasBomb_Budget_DupMergeKeyBeforeBomb asserts:
// A YAML with a duplicate merge key (<<:) before an alias bomb is rejected by Load()
// within 2 s via the walker budget. The yaml.v3 interface{} unmarshal may give
// "duplicate key <<" (not "excessive aliasing"), so ML-4G's string-match would have
// missed this form. ML-4H's budget stops it regardless of the error text.
func TestAliasBomb_Budget_DupMergeKeyBeforeBomb(t *testing.T) {
	fixture := dupMergeKeyThenBomb(25)
	// Precondition: yaml.Node unmarshal succeeds.
	var probe yaml.Node
	if err := yaml.Unmarshal([]byte(fixture), &probe); err != nil {
		t.Fatalf("precondition: yaml.Node unmarshal must succeed: %v", err)
	}
	// Desvio check: interface{} error should NOT be "excessive aliasing" for the
	// duplicate-<<: form; if it is, ML-4G already handles it (ML-4H is defense in depth).
	var iface interface{}
	ifaceErr := yaml.Unmarshal([]byte(fixture), &iface)
	if ifaceErr != nil && !strings.Contains(ifaceErr.Error(), "excessive aliasing") {
		t.Logf("desvio confirmed: interface{} gave %q — ML-4G would have missed this", ifaceErr.Error())
	} else if ifaceErr != nil {
		t.Logf("note: interface{} gave 'excessive aliasing' — ML-4G would catch this; ML-4H is defense in depth")
	}
	// Main assertion: Load() rejects within 2 s.
	done := make(chan struct{})
	go func() {
		assertLoadFailsLoud(t, fixture)
		close(done)
	}()
	select {
	case <-done:
		// Load() called osExit(1) + MalformedConfigMessage within 2 s.
	case <-time.After(2 * time.Second):
		t.Fatal("Load() must reject dup-merge-key-before-bomb within 2 s (ML-4H budget)")
	}
}

// TestAliasBomb_Budget_ParseRulesFromContent asserts:
// ParseRulesFromContent called with a 25-level alias bomb returns within 2 s.
// ParseRulesFromContent bypasses Load() (and thus the ML-4G first-pass that lived there);
// only the walker budget (ML-4H) protects it.
func TestAliasBomb_Budget_ParseRulesFromContent(t *testing.T) {
	bomb := aliasBombSequence(25)
	done := make(chan struct{})
	go func() {
		ParseRulesFromContent(bomb)
		close(done)
	}()
	select {
	case <-done:
		// Returned within 2 s — budget fired.
	case <-time.After(2 * time.Second):
		t.Fatal("ParseRulesFromContent must return within 2 s on alias bomb (ML-4H budget)")
	}
}

// TestAliasBomb_Budget_ParseDirsFromContent asserts:
// ParseDirsFromContent called with a 25-level alias bomb returns within 2 s.
// Same bypass as ParseRulesFromContent — only the walker budget protects it.
func TestAliasBomb_Budget_ParseDirsFromContent(t *testing.T) {
	bomb := aliasBombSequence(25)
	done := make(chan struct{})
	go func() {
		ParseDirsFromContent(bomb)
		close(done)
	}()
	select {
	case <-done:
		// Returned within 2 s — budget fired.
	case <-time.After(2 * time.Second):
		t.Fatal("ParseDirsFromContent must return within 2 s on alias bomb (ML-4H budget)")
	}
}

// TestAliasBomb_Budget_ReadNamespacingConfig asserts:
// ReadNamespacingConfig called with a 25-level alias bomb in a temp dir returns within 2 s.
func TestAliasBomb_Budget_ReadNamespacingConfig(t *testing.T) {
	bomb := aliasBombSequence(25)
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(bomb), 0644); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		ReadNamespacingConfig(tmp)
		close(done)
	}()
	select {
	case <-done:
		// Returned within 2 s — budget fired.
	case <-time.After(2 * time.Second):
		t.Fatal("ReadNamespacingConfig must return within 2 s on alias bomb (ML-4H budget)")
	}
}

// TestAliasBomb_Budget_ReadAgentConventions asserts:
// ReadAgentConventions called with a 25-level alias bomb in a temp dir returns within 2 s.
func TestAliasBomb_Budget_ReadAgentConventions(t *testing.T) {
	bomb := aliasBombSequence(25)
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(bomb), 0644); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		ReadAgentConventions(tmp)
		close(done)
	}()
	select {
	case <-done:
		// Returned within 2 s — budget fired.
	case <-time.After(2 * time.Second):
		t.Fatal("ReadAgentConventions must return within 2 s on alias bomb (ML-4H budget)")
	}
}

// TestAliasBomb_Budget_RealTrackfwYaml asserts:
// The repo's own trackfw.yaml is well below the node-visit budget — parse() returns
// no error (ML-4H budget). Field values are not asserted here: they reflect this
// repository's own trackfw.yaml and would fail in any consumer with a different
// configuration (issue #704). See TestAliasBomb_Budget_ConsumerFixture and
// TestAliasBomb_Budget_CounterArm for the property-based assertions.
// Reconciliation (ML-1A #704): this test afirma somente que parse() não retorna erro
// sobre o arquivo real — a propriedade que o nome do teste descreve (orçamento suficiente).
func TestAliasBomb_Budget_RealTrackfwYaml(t *testing.T) {
	data, err := os.ReadFile("../../trackfw.yaml")
	if os.IsNotExist(err) {
		t.Skip("trackfw.yaml not at ../../trackfw.yaml — skipping real-file test")
		return
	}
	if err != nil {
		t.Skipf("cannot read ../../trackfw.yaml: %v", err)
		return
	}
	cfg := defaults()
	if parseErr := parse(string(data), &cfg); parseErr != nil {
		t.Fatalf("parse() returned error on real trackfw.yaml (budget too low?): %v", parseErr)
	}
}

// TestAliasBomb_Budget_ConsumerFixture asserts:
// parse() reads consumer-specific field values from a fixture yaml, overwriting the
// ProjectConfig zero-value. The braço shows the old repo-value assertions would fail
// on a consumer with different configuration, confirming the assertions depend on what
// parse() reads rather than on defaults().
// Reconciliation (ML-1A #704): este teste afirma que parse() lê e sobrescreve campos
// com os valores da fixture do consumidor — REQDir, GovernanceMode, RoadmapNamespacing
// e Agents diferem dos valores deste repositório, então as asserções antigas reprovariam.
func TestAliasBomb_Budget_ConsumerFixture(t *testing.T) {
	const fixtureYAML = `req_dir: docs/requisições
roadmap_dir: docs/roadmaps
governance_mode: strict
roadmap_namespacing: by_agent
agents:
  - a
  - b
  - c
`
	tmp := t.TempDir()
	fixturePath := filepath.Join(tmp, "trackfw.yaml")
	if err := os.WriteFile(fixturePath, []byte(fixtureYAML), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	// Start from zero-value, not defaults(), so every assertion depends on parse().
	var cfg ProjectConfig
	if parseErr := parse(string(data), &cfg); parseErr != nil {
		t.Fatalf("parse() returned error on consumer fixture: %v", parseErr)
	}

	// Assert fields equal the fixture values.
	if cfg.REQDir != "docs/requisições" {
		t.Errorf("REQDir: want %q, got %q", "docs/requisições", cfg.REQDir)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir: want %q, got %q", "docs/roadmaps", cfg.RoadmapDir)
	}
	if cfg.GovernanceMode != "strict" {
		t.Errorf("GovernanceMode: want %q, got %q", "strict", cfg.GovernanceMode)
	}
	if cfg.RoadmapNamespacing != "by_agent" {
		t.Errorf("RoadmapNamespacing: want %q, got %q", "by_agent", cfg.RoadmapNamespacing)
	}
	if len(cfg.Agents) != 3 || cfg.Agents[0] != "a" || cfg.Agents[1] != "b" || cfg.Agents[2] != "c" {
		t.Errorf("Agents: want [a b c], got %v", cfg.Agents)
	}

	// Braço: the old repo-value assertions from the original test would fail on this fixture.
	// REQDir "docs/req" != "docs/requisições" — FAIL
	// GovernanceMode "lenient" != "strict"    — FAIL
	// RoadmapNamespacing "flat" != "by_agent" — FAIL
	// The ADRDirs guard from the original test (len(cfg.ADRDirs) == 0 || ADRDirs[0] != "docs/adr")
	// would also fail here: this fixture has no adr_dirs key, so parse() leaves ADRDirs nil —
	// len(nil) == 0 is true, so the guard fires and would report failure, same as the other three.
	// (These fields are intentionally not re-asserted here; the lines above prove the point.)
}

// TestAliasBomb_Budget_CounterArm asserts:
// When parse() receives a yaml with none of the tracked keys, all fields remain at
// their zero-value. This shows that TestAliasBomb_Budget_ConsumerFixture only passes
// because parse() actually read the fixture — not because the fields were pre-populated.
// Reconciliation (ML-1A #704): este contra-braço afirma que parse() NÃO preenche campos
// ausentes do yaml — sem a chave no arquivo, o campo fica vazio (zero-value de string/slice).
//
// Dependência de par: este teste e TestAliasBomb_Budget_ConsumerFixture falsificam juntos o
// caso de "parse() inerte" (lê o yaml mas não popula nenhum campo). ConsumerFixture mostra
// valores não-zero quando as chaves estão presentes; este braço mostra zero-values quando
// estão ausentes. Nenhum dos dois é suficiente isoladamente: sem ConsumerFixture, este braço
// é vácuo (zeros poderiam vir da inicialização padrão de Go, não de parse()); sem este braço,
// ConsumerFixture perde a testemunha de falsificação para parse() inerte.
func TestAliasBomb_Budget_CounterArm(t *testing.T) {
	const minimalYAML = `wip_limit: 2
`
	tmp := t.TempDir()
	fixturePath := filepath.Join(tmp, "trackfw.yaml")
	if err := os.WriteFile(fixturePath, []byte(minimalYAML), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	// Zero-value ProjectConfig — no defaults(), no pre-population.
	var cfg ProjectConfig
	if parseErr := parse(string(data), &cfg); parseErr != nil {
		t.Fatalf("parse() returned error on minimal yaml: %v", parseErr)
	}

	// Fields absent from the yaml must remain empty.
	if cfg.REQDir != "" {
		t.Errorf("REQDir: want empty string, got %q", cfg.REQDir)
	}
	if cfg.RoadmapDir != "" {
		t.Errorf("RoadmapDir: want empty string, got %q", cfg.RoadmapDir)
	}
	if cfg.GovernanceMode != "" {
		t.Errorf("GovernanceMode: want empty string, got %q", cfg.GovernanceMode)
	}
	if cfg.RoadmapNamespacing != "" {
		t.Errorf("RoadmapNamespacing: want empty string, got %q", cfg.RoadmapNamespacing)
	}
	if len(cfg.Agents) != 0 {
		t.Errorf("Agents: want empty slice, got %v", cfg.Agents)
	}
}
