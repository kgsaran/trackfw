// Package guard — ML-2C corrective tests (REQ-2026-11-23, Wave 2).
// Tests for ADR-2026-11-24 D1 (R-8b/8dot3), D5 (ancestor leases), D6 (normalized frozen hit).
// Each test contains one sentence stating what it asserts (Reconciliation Rule, CLAUDE.md).
//
// findProjectRoot is the 2-return wrapper for findProjectRootSafe; it was removed from
// production code (BAIXO-4, ML-2C) and lives here so that writescope_test.go tests that
// call it by the 2-return form continue to compile.
package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// findProjectRoot walks up from p to find the first ancestor directory containing
// trackfw.yaml. Returns ("", false) if none found.
//
// When a non-regular trackfw.yaml is found (directory, FIFO — D3, ADR-2026-11-24), the
// function returns (dir, true) so that callers which then read the yaml get a deny from
// the read (fail-closed). Production code uses findProjectRootSafe (which surfaces the
// error); this 2-return wrapper is kept for tests that use the old signature.
// BAIXO-4 (ML-2C): moved from production writescope.go to this test helper file.
func findProjectRoot(p string) (string, bool) {
	root, found, _ := findProjectRootSafe(p)
	return root, found
}

// ─────────────────────────────────────────────────────────────
// D1 — R-8b (NTFS alternate data streams) and 8dot3 short names
// ─────────────────────────────────────────────────────────────

// TestD1_NormalizePathComponent_Unit is a unit test for normalizePathComponent.
// Asserts: cutting at the first ':' and stripping trailing '.' and ' ' converts
// ADS and trailing-dot variants to the base filename; .bak suffix survives.
func TestD1_NormalizePathComponent_Unit(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"trackfw.yaml", "trackfw.yaml"},
		{"trackfw.yaml::$DATA", "trackfw.yaml"},
		{"trackfw.yaml:", "trackfw.yaml"},
		{"trackfw.yaml:x:$DATA", "trackfw.yaml"},
		// trailing dot + ADS: strip dot first, then cut at colon
		{"trackfw.yaml. ::$DATA", "trackfw.yaml"},
		// 8dot3 — cut at colon (no colon), strip trailing chars: none to strip
		{"TRACKF~1.YAM", "TRACKF~1.YAM"},
		// .bak must NOT be stripped
		{"trackfw.yaml.bak", "trackfw.yaml.bak"},
		{"trackf~1.yaml.bak", "trackf~1.yaml.bak"},
		// drive letter C: — only component cut is at first colon; result is "C"
		{"C:", "C"},
	}
	for _, tc := range cases {
		got := normalizePathComponent(tc.in)
		if got != tc.want {
			t.Errorf("normalizePathComponent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestD1_HasTrackfwYAMLComponent_Unit is a unit test for hasTrackfwYAMLComponent.
// Asserts: ADS variants, 8dot3, and uppercase are recognized as trackfw.yaml components;
// .bak suffix and 8dot3-of-.yaml are not.
func TestD1_HasTrackfwYAMLComponent_Unit(t *testing.T) {
	proj := t.TempDir()
	type kase struct {
		path string
		want bool
	}
	cases := []kase{
		// exact match
		{filepath.Join(proj, "trackfw.yaml"), true},
		// ADS suffix
		{filepath.Join(proj, "trackfw.yaml::$DATA"), true},
		{filepath.Join(proj, "trackfw.yaml:"), true},
		{filepath.Join(proj, "trackfw.yaml:x:$DATA"), true},
		// trailing dot + ADS
		{filepath.Join(proj, "trackfw.yaml. ::$DATA"), true},
		// 8dot3 of trackfw.yaml
		{filepath.Join(proj, "TRACKF~1.YAM"), true},
		{filepath.Join(proj, "trackf~2.yam"), true},
		// these must NOT match
		{filepath.Join(proj, "trackfw.yaml.bak"), false},
		{filepath.Join(proj, "trackf~1.yaml.bak"), false},
		{filepath.Join(proj, "trackfw.yml"), false},
	}
	for _, tc := range cases {
		got := hasTrackfwYAMLComponent(tc.path)
		if got != tc.want {
			t.Errorf("hasTrackfwYAMLComponent(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestD1_ADSTrackfwYAML_Deny asserts that a subagent writing a path whose component
// is "trackfw.yaml::$DATA" (Windows NTFS alternate data stream) is denied.
// Asserts: D1 — normalizePathComponent strips the ADS suffix before EqualFold,
// so trackfw.yaml::$DATA is blocked the same way as trackfw.yaml.
func TestD1_ADSTrackfwYAML_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	// target has an ADS suffix — on NTFS this writes the base file; blocked by D1.
	target := filepath.Join(proj, ".claude", "trackfw.yaml::$DATA")
	if err := os.MkdirAll(filepath.Join(proj, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("ADS trackfw.yaml::$DATA: want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD1_PartialADS_Deny asserts that a subagent writing a path whose component is
// "trackfw.yaml:" (bare colon ADS prefix) is denied.
// Asserts: D1 — normalizePathComponent cuts at the first colon.
func TestD1_PartialADS_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "trackfw.yaml:")
	if err := os.MkdirAll(filepath.Join(proj, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("ADS trackfw.yaml: (bare colon): want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD1_ADSWithStreamName_Deny asserts that "trackfw.yaml:x:$DATA" is denied.
// Asserts: D1 — only the part before the first colon is kept by normalizePathComponent.
func TestD1_ADSWithStreamName_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "trackfw.yaml:x:$DATA")
	if err := os.MkdirAll(filepath.Join(proj, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("ADS trackfw.yaml:x:$DATA: want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD1_8dot3TrackfwYAML_Deny asserts that a path whose component is the 8dot3
// short name TRACKF~1.YAM is denied for subagents.
// Asserts: D1 — hasTrackfwYAMLComponent recognises the 8dot3 pattern TRACKF~<n>.YAM
// (EqualFold) so a subagent cannot rewrite an existing trackfw.yaml via its short name.
func TestD1_8dot3TrackfwYAML_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "TRACKF~1.YAM")
	if err := os.MkdirAll(filepath.Join(proj, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("8dot3 TRACKF~1.YAM: want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD1_YAMLBak_Allow asserts that "trackfw.yaml.bak" is allowed for subagents
// (D1 counter-example: .bak suffix is not stripped by normalizePathComponent).
// Asserts: D1 only denies exact trackfw.yaml match after normalization; a .bak suffix
// is unrelated and must not be caught as a false positive.
func TestD1_YAMLBak_Allow(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "trackfw.yaml.bak")
	p := payload("Write", target, "agent-123")
	rc, _ := runWS(t, p)
	if rc != 0 {
		t.Fatalf("trackfw.yaml.bak: want rc=0 (not a frozen component), got %d", rc)
	}
}

// ─────────────────────────────────────────────────────────────
// D6 — normalized component matching in frozen path check
// ─────────────────────────────────────────────────────────────

// TestD6_TrailingDotSettings_Deny asserts that a subagent writing
// ".claude/settings.json." (trailing dot) is denied.
// Asserts: D6 — frozenHit compares also the path with each component normalized
// (trailing-dot stripped), so ".claude/settings.json." maps to the frozen
// ".claude/settings.json" and is denied even when the file does not yet exist.
func TestD6_TrailingDotSettings_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "settings.json.")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("settings.json. (trailing dot): want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD6_ADSSettings_Deny asserts that a subagent writing ".claude/settings.json::$DATA"
// is denied even when the file does not yet exist.
// Asserts: D6 — normalizePathComponent strips the ADS suffix from the target component,
// making it match the frozen entry ".claude/settings.json".
func TestD6_ADSSettings_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "settings.json::$DATA")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("settings.json::$DATA: want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD6_TrailingSpaceSettings_Deny asserts that a subagent writing
// ".claude/settings.json " (trailing space) is denied.
// Asserts: D6 — same normalization strips trailing spaces, matching the frozen entry.
func TestD6_TrailingSpaceSettings_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "settings.json ")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("settings.json (trailing space): want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD6_ExactSettingsJson_Deny is a baseline arm for the D6 tests.
// Asserts: the exact frozen path ".claude/settings.json" is denied before and after the
// fix; the before rc=0 on D6 cases above comes from the ADS/trailing spelling, not from
// a fixture mismatch.
func TestD6_ExactSettingsJson_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "settings.json")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("settings.json (exact): want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD6_MainSession_TrailingDot_NoFreeze asserts that a main session (no agent_id)
// writing ".claude/settings.json." is allowed (rc=0).
// Asserts: D6 normalized frozen check applies to subagents; the main session does not
// have .claude/settings.json in its frozen list, so the normalized spelling is also allowed.
// Reconciliation: this test proves that D6 does not extend subagent-frozen paths to the
// main session — ".claude/settings.json." is not frozen for the main session and must pass.
func TestD6_MainSession_TrailingDot_NoFreeze(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "settings.json.")
	// main session: no agent_id in payload
	p := payload("Write", target, "")
	rc, stderr := runWS(t, p)
	// main session does not have settings.json frozen → rc=0 regardless of normalization
	if rc != 0 {
		t.Fatalf("D6 main session: settings.json. (trailing dot): want rc=0, got %d (stderr=%q)", rc, stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// D5 — ancestor leases
// ─────────────────────────────────────────────────────────────

// futureRFC3339ML2C returns a future RFC 3339 timestamp (now + 1 hour).
// Named with the ML2C suffix to avoid collision with the helper in writescope_test.go.
func futureRFC3339ML2C() string { return time.Now().Add(time.Hour).Format(time.RFC3339) }

// makeLeaseFileML2C writes <projectRoot>/.trackfw/leases/<agentType>.json.
// Named with the ML2C suffix to avoid collision with the helper in writescope_test.go.
func makeLeaseFileML2C(t *testing.T, projectRoot, agentType string, lease map[string]interface{}) {
	t.Helper()
	leasesDir := filepath.Join(projectRoot, ".trackfw", "leases")
	if err := os.MkdirAll(leasesDir, 0700); err != nil {
		t.Fatalf("mkdir leases: %v", err)
	}
	leasePath := filepath.Join(leasesDir, agentType+".json")
	data, err := json.Marshal(lease)
	if err != nil {
		t.Fatalf("marshal lease: %v", err)
	}
	if err := os.WriteFile(leasePath, data, 0600); err != nil {
		t.Fatalf("write lease: %v", err)
	}
}

// TestD5_AncestorLeaseConsulted_Deny asserts that when a subagent's cwd and target
// are both inside a nested project (svc/), and an outer project has a restrictive lease
// (allow=[<outer>/docs/x.md]), the outer lease is consulted and denies the target
// outside its allow list.
// Asserts: D5 — collectLeaseRoots now includes every ancestor of the target that has
// trackfw.yaml, so the outer lease gates access even when cwd resolves to the inner root.
func TestD5_AncestorLeaseConsulted_Deny(t *testing.T) {
	outer := t.TempDir()
	// Use EvalSymlinks to get the canonical path (macOS /var → /private/var).
	outer, _ = filepath.EvalSymlinks(outer)

	// Outer project yaml
	if err := os.WriteFile(filepath.Join(outer, "trackfw.yaml"), []byte("project_name: outer\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Allowed path in outer lease (absolute)
	allowedPath := filepath.Join(outer, "docs", "x.md")
	if err := os.MkdirAll(filepath.Dir(allowedPath), 0700); err != nil {
		t.Fatal(err)
	}

	// Outer lease: allow only outer/docs/x.md
	makeLeaseFileML2C(t, outer, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"expires":    futureRFC3339ML2C(),
		"allow":      []string{allowedPath},
	})

	// Inner project (svc/) with its own yaml
	svc := filepath.Join(outer, "svc")
	if err := os.MkdirAll(svc, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc, "trackfw.yaml"), []byte("project_name: svc\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target: svc/main.go — outside outer's allow list
	target := filepath.Join(svc, "main.go")

	// cwd = svc (inner project)
	p := payloadFullWithCwd("Write", target, "agent-123", "apolo-tf", svc)
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("D5 arm: outer lease restricts svc/main.go: want rc=2, got %d (stderr=%q)", rc, stderr)
	}
}

// TestD5_AncestorLeaseConsulted_AllowCounterArm asserts that when D5's ancestor walk
// reaches outer/ and the outer lease explicitly allows the target, the write is permitted.
// Asserts: D5 — collectLeaseRoots walks up from the target's project root (svc/) to outer/,
// reads the outer lease, and honours the allow entry for svc/main.go; without the ancestor
// walk, the outer lease would not be consulted at all (svc/ has no lease of its own).
func TestD5_AncestorLeaseConsulted_AllowCounterArm(t *testing.T) {
	outer := t.TempDir()
	outer, _ = filepath.EvalSymlinks(outer)

	if err := os.WriteFile(filepath.Join(outer, "trackfw.yaml"), []byte("project_name: outer\n"), 0600); err != nil {
		t.Fatal(err)
	}

	svc := filepath.Join(outer, "svc")
	if err := os.MkdirAll(svc, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc, "trackfw.yaml"), []byte("project_name: svc\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target is svc/main.go — it lives inside the inner project (svc/).
	// The outer lease (not the inner one) explicitly allows this exact path.
	// svc/ has no lease of its own, so without the D5 walk, no lease would be found.
	target := filepath.Join(svc, "main.go")
	makeLeaseFileML2C(t, outer, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"expires":    futureRFC3339ML2C(),
		"allow":      []string{target},
	})

	// cwd = svc (inner project); target = svc/main.go
	p := payloadFullWithCwd("Write", target, "agent-123", "apolo-tf", svc)
	rc, stderr := runWS(t, p)
	if rc != 0 {
		t.Fatalf("D5 counter-arm: outer allows svc/main.go, ancestor walk active: want rc=0, got %d (stderr=%q)", rc, stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// D3 — collectLeaseRoots fail-closed on non-regular ancestor yaml
// ─────────────────────────────────────────────────────────────

// TestD3_CollectLeaseRoots_NonRegularAncestorYAML_Error asserts that collectLeaseRoots
// returns an error when an ancestor directory contains a directory (not a file) named
// "trackfw.yaml", instead of silently skipping it.
// Asserts: D3 (ADR-2026-11-24) — a non-regular trackfw.yaml in an ancestor is not
// "absent"; collectLeaseRoots must return an error so the caller can deny (rc=2).
// Reconciliation: this test proves that the fail-closed D3 policy is enforced inside
// collectLeaseRoots itself, not only in buildFrozenList — the break-silently path that
// violated "only absent is absent" has been replaced by an explicit error return.
func TestD3_CollectLeaseRoots_NonRegularAncestorYAML_Error(t *testing.T) {
	// Build fixture: outer/ has a *directory* named trackfw.yaml; inner/ is a valid project.
	outer := t.TempDir()
	outer, _ = filepath.EvalSymlinks(outer)

	// Plant a directory named "trackfw.yaml" in the ancestor — simulates a subagent planting a
	// directory to mask the real yaml and bypass the ancestor lease.
	if err := os.MkdirAll(filepath.Join(outer, "trackfw.yaml"), 0700); err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	// inner/ has a valid (regular) trackfw.yaml — it is the targetRoot.
	if err := os.WriteFile(filepath.Join(inner, "trackfw.yaml"), []byte("project_name: inner\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := collectLeaseRoots(inner, "")
	if err == nil {
		t.Fatal("D3: collectLeaseRoots with directory-named trackfw.yaml in ancestor: want error, got nil")
	}
}

// ─────────────────────────────────────────────────────────────
// D1 8dot3 regex — package-level compile check
// ─────────────────────────────────────────────────────────────

// TestD1_8dot3Regex_CompileAndMatch checks that the production variable trackfwYAML8dot3RE
// matches the expected inputs, including the hash form Windows generates after collisions.
// Asserts: trackfwYAML8dot3RE matches TRACKF~1.YAM, trackf~999.yam, and the hash form
// TR1A2B~1.YAM, but not trackfw.yaml, trackfw.yaml.bak, or notes~1.yaml (wrong extension).
func TestD1_8dot3Regex_CompileAndMatch(t *testing.T) {
	hits := []string{"TRACKF~1.YAM", "trackf~2.yam", "trackf~999.yam", "TR1A2B~1.YAM"}
	misses := []string{"trackfw.yaml", "trackfw.yaml.bak", "trackf~1.yaml", "TRACKFW.YAML", "notes~1.yaml"}
	for _, s := range hits {
		if !trackfwYAML8dot3RE.MatchString(s) {
			t.Errorf("8dot3 regex: expected match for %q", s)
		}
	}
	for _, s := range misses {
		if trackfwYAML8dot3RE.MatchString(s) {
			t.Errorf("8dot3 regex: expected no match for %q", s)
		}
	}
}
