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
	"os"
	"path/filepath"
	"testing"
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
