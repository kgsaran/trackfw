// Package guard — write-scope nested-root tests (ML-1A, REQ-2026-11-23).
// Tests for ADR-2026-11-24 decisions D1, D2, D3.
// Each test contains one sentence stating what it asserts (Reconciliation Rule, CLAUDE.md).
//
// FIFO test lives in writescope_nested_root_fifo_unix_test.go (build tag !windows):
// syscall.Mkfifo does not exist in the Windows syscall package — a runtime.GOOS check
// inside the test body would not stop the file from failing to compile on Windows.
package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────
// D1: component check — subagent must not write trackfw.yaml
// ─────────────────────────────────────────────────────────────

// TestD1_SubagentWrite_TrackfwYAML_Deny asserts that a subagent writing .claude/trackfw.yaml is denied (rc=2) with the D1 message.
// Asserts: D1 closes the sub-variante A re-rooting attack before Step 7.
func TestD1_SubagentWrite_TrackfwYAML_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "trackfw.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "(D1)") {
		t.Fatalf("want D1 message in stderr, got %q", stderr)
	}
}

// TestD1_SubagentWithLease_TrackfwYAML_Deny asserts that D1 applies even when a lease allows the .claude/ directory.
// Asserts: D1 is unconditional for subagents — lease allow cannot override the component check.
func TestD1_SubagentWithLease_TrackfwYAML_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "trackfw.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{filepath.Join(proj, ".claude") + "/"},
		"expires":    futureRFC3339(),
	})
	p := payloadFull("Write", target, "agent-123", "apolo-tf")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "(D1)") {
		t.Fatalf("want D1 message in stderr, got %q", stderr)
	}
}

// TestD1_ComponentInMiddle_Deny asserts that D1 matches trackfw.yaml as an intermediate path component and emits the D1 deny message.
// Asserts: D1 blocks the path x/trackfw.yaml/y, and the deny is pinned to D1 (not D3) via the stderr message.
func TestD1_ComponentInMiddle_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	// Use a path that has trackfw.yaml as a middle component, without creating the directory on disk.
	// D1 runs before Step 6 (path resolution) so the path need not exist.
	target := filepath.Join(proj, "x", "trackfw.yaml", "y")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "(D1)") {
		t.Fatalf("want D1 message in stderr, got %q", stderr)
	}
}

// TestD1_ExactFileLease_TrackfwYAML_Deny asserts that D1 denies even when the lease grants the exact file path.
// Asserts: D1 runs before Step 7 and before the lease check — no lease can override it.
func TestD1_ExactFileLease_TrackfwYAML_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "sub", "trackfw.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{target},
		"expires":    futureRFC3339(),
	})
	p := payloadFull("Write", target, "agent-123", "apolo-tf")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "(D1)") {
		t.Fatalf("want D1 message in stderr, got %q", stderr)
	}
}

// TestD1_CaseInsensitive_Deny asserts that D1 uses EqualFold so TRACKFW.YAML is denied on every OS.
// Asserts: D1 is case-insensitive — a subagent cannot bypass by using uppercase TRACKFW.YAML.
func TestD1_CaseInsensitive_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".claude", "TRACKFW.YAML")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "(D1)") {
		t.Fatalf("want D1 message in stderr, got %q", stderr)
	}
}

// TestD1_OutsideProject_Deny asserts that D1 denies writing trackfw.yaml even outside any project directory.
// Asserts: D1 acts before Step 7 (not-found allows), so the absence of a project root does not rescue the write.
func TestD1_OutsideProject_Deny(t *testing.T) {
	// Use a fresh tempdir with no trackfw.yaml → not-found path, but D1 must still fire.
	dir := t.TempDir()
	target := filepath.Join(dir, "trackfw.yaml")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "(D1)") {
		t.Fatalf("want D1 message in stderr, got %q", stderr)
	}
}

// TestD1_MainSession_TrackfwYAML_Allow asserts that the main session (no agent_id) may write trackfw.yaml.
// Asserts: D1 is subagent-only — main session writes are not blocked by the component check.
func TestD1_MainSession_TrackfwYAML_Allow(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "sub", "trackfw.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	// main session: no agent_id
	p := payload("Write", target, "")
	rc, _ := runWS(t, p)
	if rc != 0 {
		t.Fatalf("main session: want rc=0, got %d", rc)
	}
}

// ─────────────────────────────────────────────────────────────
// D1 unit: normalizePathComponent and hasTrackfwYAMLComponent
// ─────────────────────────────────────────────────────────────

// TestD1_NormalizePathComponent asserts the Windows normalization strips trailing dots and spaces but not unrelated suffixes.
// Asserts: normalizePathComponent("trackfw.yaml.") == "trackfw.yaml" and "trackfw.yaml.bak" is unchanged (D1, Windows component form).
func TestD1_NormalizePathComponent(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"trackfw.yaml", "trackfw.yaml"},
		{"trackfw.yaml.", "trackfw.yaml"},
		{"trackfw.yaml ", "trackfw.yaml"},
		{"trackfw.yaml. ", "trackfw.yaml"},
		{"TRACKFW.YAML", "TRACKFW.YAML"},
		{"trackfw.yaml.bak", "trackfw.yaml.bak"}, // must NOT match
	}
	for _, c := range cases {
		got := normalizePathComponent(c.in)
		if got != c.want {
			t.Errorf("normalizePathComponent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestD1_HasTrackfwYAMLComponent_Cases asserts detection of trackfw.yaml in various component positions and case forms.
// Asserts: hasTrackfwYAMLComponent correctly identifies paths with trackfw.yaml component, including case variants and Windows normalizations, and correctly rejects unrelated components.
func TestD1_HasTrackfwYAMLComponent_Cases(t *testing.T) {
	yes := []string{
		"trackfw.yaml",
		"a/trackfw.yaml",
		"a/trackfw.yaml/b",
		"TRACKFW.YAML",
		"a/TRACKFW.YAML/b",
		"trackfw.yaml.",  // Windows trailing-dot form
		"trackfw.yaml ",  // Windows trailing-space form
	}
	no := []string{
		"trackfw.yaml.bak",
		"my_trackfw.yaml",
		"trackfw_yaml",
		"a/b/c",
	}
	for _, p := range yes {
		if !hasTrackfwYAMLComponent(p) {
			t.Errorf("hasTrackfwYAMLComponent(%q) = false, want true", p)
		}
	}
	for _, p := range no {
		if hasTrackfwYAMLComponent(p) {
			t.Errorf("hasTrackfwYAMLComponent(%q) = true, want false", p)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// D2: monotonic union of frozen lists
// ─────────────────────────────────────────────────────────────

// TestD2_MonotonicFrozen_OuterYAMLWithoutWriteScope_Deny asserts that the full frozen list from every ancestor is used even when the outer yaml lacks write_scope.
// Asserts: D2 uses the FULL buildFrozenList of each ancestor (fixed frozen paths + yaml frozen_paths), not only frozen_paths — a nested yaml that lacks write_scope does not remove ancestor protections.
func TestD2_MonotonicFrozen_OuterYAMLWithoutWriteScope_Deny(t *testing.T) {
	// outer project (no write_scope section) → subagent frozen list includes .claude/settings.json
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// nested project inside outer/.claude/
	inner := filepath.Join(outer, ".claude")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// target: outer/.claude/settings.json (frozen because .claude/settings.json is a subagent-frozen path of outer)
	target := filepath.Join(inner, "settings.json")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "frozen path") {
		t.Fatalf("want frozen path message in stderr, got %q", stderr)
	}
}

// TestD2_CaseInsensitiveFS_NestedYAML_Deny asserts that TRACKFW.YAML in a nested directory triggers D2 on case-insensitive filesystems.
// Asserts: D2 reads the YAML file via ReadRegularFile (which uses the actual path), and the outer frozen list protects .claude/settings.json regardless of YAML filename case.
func TestD2_CaseInsensitiveFS_NestedYAML_Deny(t *testing.T) {
	// Skip on case-sensitive filesystems by probing at runtime.
	probe := t.TempDir()
	if err := os.WriteFile(filepath.Join(probe, "PROBE.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(probe, "probe.txt"))
	if os.IsNotExist(err) {
		t.Skip("case-sensitive filesystem — TRACKFW.YAML is a distinct name")
	}
	// outer project with regular trackfw.yaml
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// nested yaml using uppercase — on case-insensitive FS this IS trackfw.yaml
	inner := filepath.Join(outer, ".claude")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "TRACKFW.YAML"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(inner, "settings.json")
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "frozen path") {
		t.Fatalf("want frozen path message in stderr, got %q", stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// D3: ErrNotRegularFile denies
// ─────────────────────────────────────────────────────────────

// TestD3_DirectoryNamedTrackfwYAML_Deny asserts that a directory named trackfw.yaml nested inside a valid project triggers deny when a subagent writes in that subdirectory.
// Asserts: D3 — when findProjectRootSafe encounters a directory named trackfw.yaml, it returns ErrNotRegularFile and the guard denies; before the fix (Forma V), os.IsNotExist was used for all errors and the directory was treated as absent, changing the root to the outer project.
func TestD3_DirectoryNamedTrackfwYAML_Deny(t *testing.T) {
	// outer project (regular yaml)
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// Plant a DIRECTORY named trackfw.yaml inside .claude/
	nestedDir := filepath.Join(outer, ".claude")
	if err := os.MkdirAll(nestedDir, 0700); err != nil {
		t.Fatal(err)
	}
	yamlDir := filepath.Join(nestedDir, "trackfw.yaml")
	if err := os.Mkdir(yamlDir, 0700); err != nil {
		t.Fatal(err)
	}
	// target: outer/.claude/settings.json
	// D3 mechanism: findProjectRootSafe walks up from nestedDir (.claude/).
	// At nestedDir it calls fileutil.ReadRegularFile("…/.claude/trackfw.yaml") — this is a
	// directory, so ReadRegularFile returns ErrNotRegularFile (not os.IsNotExist).
	// Pre-fix (Forma V): os.IsNotExist was the only "absent" test; ErrNotRegularFile was
	// treated as an unexpected error, silenced, and the walk continued up to outer/, where
	// the outer project root was used — allowing the write.
	// Post-fix (D3): any non-IsNotExist error from ReadRegularFile causes findProjectRootSafe
	// to return ("", false, err); RunWriteScope propagates the error as deny → rc=2.
	target := filepath.Join(nestedDir, "settings.json")
	p := payload("Write", target, "agent-123")
	rc, _ := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
}

// TestD3_SymlinkToRegularFile_DoesNotBreakFrozen asserts that a symlink named trackfw.yaml pointing to a valid regular file is accepted by ReadRegularFile and frozen paths work correctly.
// Asserts: D3 — ReadRegularFile follows symlinks to regular files, so a symlink-based trackfw.yaml does not erroneously deny the project root lookup.
func TestD3_SymlinkToRegularFile_DoesNotBreakFrozen(t *testing.T) {
	proj := t.TempDir()
	realYAML := filepath.Join(proj, "real_trackfw.yaml")
	if err := os.WriteFile(realYAML, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	yamlLink := filepath.Join(proj, "trackfw.yaml")
	// Use symlinkOrSkipInGuard to skip on Windows without Developer Mode.
	if !symlinkOrSkipInGuard(t, realYAML, yamlLink) {
		return
	}
	// target: .claude/settings.json — should be frozen for subagent
	target := filepath.Join(proj, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, stderr := runWS(t, p)
	// .claude/settings.json is a subagent frozen path → rc=2.
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
	if !strings.Contains(stderr, "frozen path") {
		t.Fatalf("want frozen path message in stderr, got %q", stderr)
	}
	// Allow arm: a non-frozen path in the same project → rc=0.
	targetOK := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(targetOK), 0700); err != nil {
		t.Fatal(err)
	}
	p2 := payload("Write", targetOK, "agent-123")
	rc2, stderr2 := runWS(t, p2)
	if rc2 != 0 {
		t.Fatalf("allow arm: want rc=0, got %d (stderr=%q)", rc2, stderr2)
	}
}

// ─────────────────────────────────────────────────────────────
// No regression: worktree and monorepo
// ─────────────────────────────────────────────────────────────

// TestD2_Worktree_MonotonicDoesNotRegressWorktreeContract asserts that the monotonic frozen list does not break the worktree contract and allows non-frozen writes.
// Asserts: D2 monotonic union adds protection but does not change the worktree root-resolution contract from ADR-2026-10-23; non-frozen paths in the main checkout remain writable.
func TestD2_Worktree_MonotonicDoesNotRegressWorktreeContract(t *testing.T) {
	// main checkout with trackfw.yaml
	main := makeWSProjectDir(t)
	// worktree: separate directory also with trackfw.yaml
	wt := makeWSProjectDir(t)

	// deny arm: main/.claude/settings.json — frozen for subagent
	target := filepath.Join(main, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	p := payloadFullWithCwd("Write", target, "agent-wt", "apolo-tf", wt)
	rc, _ := runWS(t, p)
	if rc != 2 {
		t.Fatalf("worktree deny arm: want rc=2, got %d", rc)
	}

	// allow arm: main/src/main.go — not frozen
	targetOK := filepath.Join(main, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(targetOK), 0700); err != nil {
		t.Fatal(err)
	}
	makeLeaseFile(t, main, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{filepath.Join(main, "src") + "/"},
		"expires":    futureRFC3339(),
	})
	p2 := payloadFullWithCwd("Write", targetOK, "agent-wt", "apolo-tf", wt)
	rc2, stderr2 := runWS(t, p2)
	if rc2 != 0 {
		t.Fatalf("worktree allow arm: want rc=0, got %d (stderr=%q)", rc2, stderr2)
	}
}

// TestD2_Monorepo_BothProjectsDenied asserts that pre-existing nested yamls in a monorepo still deny writes to frozen paths in both projects and allow non-frozen writes.
// Asserts: D2 monotonic union does not regress monorepo scenarios — each nested yaml's frozen list is included, and non-frozen paths remain accessible.
func TestD2_Monorepo_BothProjectsDenied(t *testing.T) {
	// outer project
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// inner project (pre-existing monorepo nesting)
	inner := filepath.Join(outer, "services", "api")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	// deny arm: inner/.claude/settings.json
	targetInner := filepath.Join(inner, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(targetInner), 0700); err != nil {
		t.Fatal(err)
	}
	p1 := payload("Write", targetInner, "agent-123")
	rc1, _ := runWS(t, p1)
	if rc1 != 2 {
		t.Fatalf("inner project deny: want rc=2, got %d", rc1)
	}

	// deny arm: outer/.claude/settings.json
	targetOuter := filepath.Join(outer, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(targetOuter), 0700); err != nil {
		t.Fatal(err)
	}
	p2 := payload("Write", targetOuter, "agent-456")
	rc2, _ := runWS(t, p2)
	if rc2 != 2 {
		t.Fatalf("outer project deny: want rc=2, got %d", rc2)
	}

	// allow arm: inner/src/main.go — not frozen
	targetOK := filepath.Join(inner, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(targetOK), 0700); err != nil {
		t.Fatal(err)
	}
	// No lease → main session write is allowed
	p3 := payload("Write", targetOK, "")
	rc3, stderr3 := runWS(t, p3)
	if rc3 != 0 {
		t.Fatalf("monorepo allow arm: want rc=0, got %d (stderr=%q)", rc3, stderr3)
	}
}
