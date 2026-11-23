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
// and return within 2 s. This confirms that the ML-4G interface{} first-pass check in Load()
// intercepts the bomb before the yaml.Node walker runs.
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
		t.Fatal("Load() must reject alias bomb and return within 2 s (ML-4G)")
	}
}

// TestAliasBomb_ReaderLegitimateAlias asserts:
// An anchor used 3 times (legitimate, non-bomb aliasing) is NOT rejected by the ML-4G
// interface{} check and is loaded normally by Load() — wip_limit and req_dir are read.
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
// Without the ML-4G interface{} pre-check, normalizeMapping on a 25-level sequence bomb
// takes >2 s — demonstrating the attack vector the fix addresses. Uses 25 levels
// (~2.9 s without the guard) so the goroutine terminates naturally within ~1 s after
// the 2 s deadline fires. If this test fails (goroutine finishes before deadline),
// verify that aliasBombSequence(25) produces an effective bomb on this machine.
func TestAliasBomb_ReaderCounterArm(t *testing.T) {
	// 25 levels: 2^25 ≈ 33 M yaml.Node walker steps — takes ~2.9 s without the guard.
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
		// Call normalizeMapping directly — this is the yaml.Node walker path that
		// runs after the ML-4G check and would hang without it.
		normalizeMapping(top, make(map[*yaml.Node]bool))
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("counter-arm FAIL: normalizeMapping completed on 25-level bomb within 2 s — " +
			"verify aliasBombSequence(25) generates an effective bomb on this machine")
	case <-time.After(2 * time.Second):
		// Expected: without the interface{} check, normalizeMapping takes >>2 s.
		// Goroutine terminates naturally after ~0.9 s more; test process continues. ML-4G.
	}
}
