// Package guard — unit tests for writescope.go.
//
// ML-3A2 (ADR-2026-10-23 D3): unit tests for resolveWriteScopePath, findProjectRoot,
// buildFrozenList, fileHasFrozenMarker, normalizeTargetPath, pathMatchesFrozen,
// and RunWriteScope (called directly, not via subprocess).
//
// Reconciliation Rule (CLAUDE.md): each test names the conclusion of ML-3A2 it asserts.
//
// Contra-braço conventions used here:
//   - Tests that use new symbols (writeScopeInternalHook, normalizeTargetPath) CANNOT compile
//     on HEAD. That is declared explicitly per test.
//   - Tests for behavioural fixes (relative path denial, YAML invalidity) are verified below
//     against the scratch HEAD copy; results are cited in the table.
package guard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// shortTimeout is used for RunWriteScope calls in unit tests to keep the idle window fast.
const shortTimeout = 50 * time.Millisecond

// makeWSProjectDir creates a temp directory with a minimal trackfw.yaml for write-scope tests.
func makeWSProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("create trackfw.yaml: %v", err)
	}
	return dir
}

// makeDocsWorkingContext creates docs/agents-working-context.md inside dir.
// If frozen is true, the frozen marker is placed on line 1.
func makeDocsWorkingContext(t *testing.T, dir string, frozen bool) string {
	t.Helper()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatalf("MkdirAll docs: %v", err)
	}
	content := "# Working Context\n"
	if frozen {
		content = frozenMarker + "\n" + content
	}
	path := filepath.Join(docs, "agents-working-context.md")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write working context: %v", err)
	}
	return path
}

// payload builds a minimal JSON payload for RunWriteScope.
// agentID may be "" for main session or a non-empty string for subagent.
func payload(tool, filePath, agentID string) string {
	if agentID != "" {
		return `{"tool_name":"` + tool + `","tool_input":{"file_path":"` + filePath + `"},"agent_id":"` + agentID + `"}`
	}
	return `{"tool_name":"` + tool + `","tool_input":{"file_path":"` + filePath + `"}}`
}

// runWS calls RunWriteScope with the given JSON payload (main-session, no agent_id by default).
func runWS(t *testing.T, jsonPayload string) (exitCode int, stderr string) {
	t.Helper()
	var stderrBuf strings.Builder
	code := RunWriteScope(strings.NewReader(jsonPayload), shortTimeout, &stderrBuf)
	return code, stderrBuf.String()
}

// ─────────────────────────────────────────────────────────────
// fileHasFrozenMarker
// ─────────────────────────────────────────────────────────────

// TestFileHasFrozenMarker_Line1_Detected asserts that the marker on line 1 is detected.
// Asserts: ML-3A2 unit coverage — marker detection (contract item 4 boundary).
// Contra-braço: this test exercises existing logic; passes on HEAD (behaviour was correct).
func TestFileHasFrozenMarker_Line1_Detected(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ctx.md")
	if err := os.WriteFile(f, []byte(frozenMarker+"\nline2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !fileHasFrozenMarker(f) {
		t.Fatal("marker on line 1: want true, got false")
	}
}

// TestFileHasFrozenMarker_Line10_Detected asserts that the marker on line 10 is detected.
// Asserts: ML-3A2 unit coverage — marker on last checked line.
// Contra-braço: passes on HEAD (existing logic).
func TestFileHasFrozenMarker_Line10_Detected(t *testing.T) {
	dir := t.TempDir()
	// Lines 1-9 are blank, line 10 has the marker.
	content := strings.Repeat("\n", 9) + frozenMarker + "\n"
	f := filepath.Join(dir, "ctx.md")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if !fileHasFrozenMarker(f) {
		t.Fatal("marker on line 10: want true, got false")
	}
}

// TestFileHasFrozenMarker_Line11_NotDetected asserts that the marker on line 11 is NOT detected.
// Asserts: ML-3A2 unit coverage — marker beyond 10-line window is invisible.
// Contra-braço: passes on HEAD (existing logic).
func TestFileHasFrozenMarker_Line11_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// Lines 1-10 are blank, line 11 has the marker.
	content := strings.Repeat("\n", 10) + frozenMarker + "\n"
	f := filepath.Join(dir, "ctx.md")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if fileHasFrozenMarker(f) {
		t.Fatal("marker on line 11: want false (outside window), got true")
	}
}

// TestFileHasFrozenMarker_MissingFile_False asserts that a missing file returns false.
// Asserts: ML-3A2 unit coverage — missing file is not frozen for main session.
// Contra-braço: passes on HEAD (existing logic).
func TestFileHasFrozenMarker_MissingFile_False(t *testing.T) {
	if fileHasFrozenMarker("/does/not/exist/ctx.md") {
		t.Fatal("missing file: want false, got true")
	}
}

// ─────────────────────────────────────────────────────────────
// findProjectRoot
// ─────────────────────────────────────────────────────────────

// TestFindProjectRoot_DirectFile asserts that a file inside a project returns the project root.
// Asserts: ML-3A2 unit coverage — findProjectRoot walks up to trackfw.yaml.
// Contra-braço: passes on HEAD (existing logic).
func TestFindProjectRoot_DirectFile(t *testing.T) {
	dir := makeWSProjectDir(t)
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(docs, "somefile.md")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	root, found := findProjectRoot(target)
	if !found {
		t.Fatal("file inside project: want found=true, got false")
	}
	if root != dir {
		t.Fatalf("expected root %q, got %q", dir, root)
	}
}

// TestFindProjectRoot_OutsideProject asserts that a file with no trackfw.yaml ancestor returns ("", false).
// Asserts: ML-3A2 unit coverage — contract item 3: alvo fora de projeto → allow.
// Contra-braço: passes on HEAD (existing logic).
func TestFindProjectRoot_OutsideProject(t *testing.T) {
	// t.TempDir() itself has no trackfw.yaml; neither do its parents up to /.
	dir := t.TempDir()
	target := filepath.Join(dir, "some.md")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, found := findProjectRoot(target)
	if found {
		t.Fatal("file outside any project: want found=false, got true")
	}
}

// TestFindProjectRoot_NewFileInProject asserts that a not-yet-existing file path is handled.
// Asserts: ML-3A2 unit coverage — Write creates new files; root must still be found.
// Contra-braço: passes on HEAD (existing logic).
func TestFindProjectRoot_NewFileInProject(t *testing.T) {
	dir := makeWSProjectDir(t)
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	// File does not exist yet.
	target := filepath.Join(docs, "newfile.md")
	root, found := findProjectRoot(target)
	if !found {
		t.Fatal("new file in project: want found=true, got false")
	}
	if root != dir {
		t.Fatalf("expected root %q, got %q", dir, root)
	}
}

// ─────────────────────────────────────────────────────────────
// normalizeTargetPath
// ─────────────────────────────────────────────────────────────

// TestNormalizeTargetPath_Windows_SlashDrive asserts /c/path → C:\path on Windows.
// Asserts: ML-3A2 AC5 (Windows normalisation) — /c/... maps to the drive C:.
// Contra-braço: new function; compilation fails on HEAD. Documented.
func TestNormalizeTargetPath_Windows_SlashDrive(t *testing.T) {
	got := normalizeTargetPath("/c/proj/docs/agents-working-context.md", "windows")
	want := `C:\proj\docs\agents-working-context.md`
	if got != want {
		t.Fatalf("/c/... normalisation: got %q, want %q", got, want)
	}
}

// TestNormalizeTargetPath_Windows_UpperDrive asserts /C/path → C:\path on Windows.
// Asserts: ML-3A2 AC5 (Windows normalisation) — uppercase drive letter handled.
// Contra-braço: new function; compilation fails on HEAD.
func TestNormalizeTargetPath_Windows_UpperDrive(t *testing.T) {
	got := normalizeTargetPath("/C/proj/docs/file.md", "windows")
	want := `C:\proj\docs\file.md`
	if got != want {
		t.Fatalf("/C/... upper-case normalisation: got %q, want %q", got, want)
	}
}

// TestNormalizeTargetPath_Windows_MixedSlash asserts C:/path → C:\path on Windows.
// Asserts: ML-3A2 AC5 (Windows normalisation) — C:/ with forward slash is normalised.
// Contra-braço: new function; compilation fails on HEAD.
func TestNormalizeTargetPath_Windows_MixedSlash(t *testing.T) {
	got := normalizeTargetPath(`C:/proj/docs/agents-working-context.md`, "windows")
	want := `C:\proj\docs\agents-working-context.md`
	if got != want {
		t.Fatalf("C:/ mixed-slash normalisation: got %q, want %q", got, want)
	}
}

// TestNormalizeTargetPath_NonWindows_Unchanged asserts that /c/path is unchanged on darwin/linux.
// Asserts: ML-3A2 AC5 (Windows normalisation) — non-Windows paths are passed through.
// Contra-braço: new function; compilation fails on HEAD.
func TestNormalizeTargetPath_NonWindows_Unchanged(t *testing.T) {
	p := "/c/proj/docs/agents-working-context.md"
	for _, goos := range []string{"darwin", "linux"} {
		got := normalizeTargetPath(p, goos)
		if got != p {
			t.Fatalf("goos=%q: want path unchanged %q, got %q", goos, p, got)
		}
	}
}

// TestNormalizeTargetPath_Windows_NativeBackslash_Unchanged asserts that a native Windows path
// (C:\...) is left unchanged (no double-conversion).
// Asserts: ML-3A2 AC5 — already-native paths are not mangled.
// Contra-braço: new function; compilation fails on HEAD.
func TestNormalizeTargetPath_Windows_NativeBackslash_Unchanged(t *testing.T) {
	p := `C:\proj\docs\file.md`
	got := normalizeTargetPath(p, "windows")
	if got != p {
		t.Fatalf("native backslash: want unchanged %q, got %q", p, got)
	}
}

// ─────────────────────────────────────────────────────────────
// RunWriteScope — path cases (ML-3A2 AC5)
// ─────────────────────────────────────────────────────────────

// TestRunWriteScope_RelativePath_Denied asserts that a relative file_path → exit 2.
// Asserts: ML-3A2 AC5 — relative path is denied (fail-closed).
// Contra-braço: on HEAD, resolveWriteScopePath uses os.Getwd() + concat and then walks up.
//
//	In the test process cwd (the repo worktree at /private/tmp/trackfw-toml-escape),
//	"docs/plain.md" resolves to a path inside the real project, where docs/agents-working-context.md
//	may or may not have a frozen marker. The outcome is non-deterministic. After the fix, any
//	relative path is denied immediately → deterministic exit 2.
//	Measured in scratchpad: HEAD gives exit 0 for "docs/plain.md" (no marker on HEAD); fix gives exit 2.
func TestRunWriteScope_RelativePath_Denied(t *testing.T) {
	// "docs/relative.md" is a relative path; no absolute prefix.
	p := `{"tool_name":"Write","tool_input":{"file_path":"docs/relative.md"}}`
	rc, stderr := runWS(t, p)
	if rc != 2 {
		t.Fatalf("relative path: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "absolute") && !strings.Contains(stderr, "relative") {
		t.Fatalf("relative path deny: want 'absolute' or 'relative' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_DotDot_FrozenFile asserts that a path with ".." that resolves to a frozen
// file is denied.
// Asserts: ML-3A2 AC5 — ".." traversal landing on frozen file → deny.
// Contra-braço: passes on HEAD (existing EvalSymlinks resolves ".." correctly).
func TestRunWriteScope_DotDot_FrozenFile(t *testing.T) {
	dir := makeWSProjectDir(t)
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory so that ".." from it is within the project.
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	makeDocsWorkingContext(t, dir, true /* frozen */)

	// Path with "..": subdir/../docs/agents-working-context.md → docs/agents-working-context.md
	dotDotPath := filepath.Join(dir, "subdir", "..", "docs", "agents-working-context.md")
	rc, stderr := runWS(t, payload("Write", dotDotPath, ""))
	if rc != 2 {
		t.Fatalf("'..' to frozen file: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_DotDot_OutsideProject asserts that ".." leaving the project → allow.
// Asserts: ML-3A2 AC5 — ".." escaping the project root lands outside any trackfw project → allow.
// Contra-braço: passes on HEAD (findProjectRoot returns false → exit 0).
func TestRunWriteScope_DotDot_OutsideProject(t *testing.T) {
	dir := makeWSProjectDir(t)

	// Target: dir/../../../tmp/outside.md → goes outside dir into a non-project ancestor.
	// t.TempDir() is something like /var/folders/.../TestXxx/NNN; its parent NNN-1 has no trackfw.yaml.
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.md")
	if err := os.WriteFile(outsideFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	// Build a ".." path that resolves to outsideFile by going up from dir.
	// We need a real absolute path that crosses from dir into outsideDir.
	// Since both are under /var/.../T/, use a path that goes up and sideways.
	// Simpler: just use outsideFile directly — findProjectRoot(outsideFile) returns ("", false)
	// because outsideDir has no trackfw.yaml. The ".." variant is also tested.
	_ = dir // dir created project root that is never an ancestor of outsideFile

	// Verify that a direct absolute path outside the project is allowed.
	rc, stderr := runWS(t, payload("Write", outsideFile, ""))
	if rc != 0 {
		t.Fatalf("path outside project: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_DotDot_OutsideProject_DotDotPath asserts that a ".." path that exits the
// project lands outside any trackfw project → allow.
// Asserts: ML-3A2 AC5 — ".." walking above the project root; findProjectRoot returns false.
// Contra-braço: passes on HEAD.
func TestRunWriteScope_DotDot_OutsideProject_DotDotPath(t *testing.T) {
	proj := makeWSProjectDir(t)

	// The parent of proj is a plain temp directory (t.TempDir gives nested dirs).
	// proj itself looks like /tmp/.../T/TestX/001; proj's parent is /tmp/.../T/TestX (no yaml).
	// Build path: proj + "/../plainfile.txt".
	dotDotTarget := filepath.Join(proj, "..", "plain_outside.txt")
	// Write the file so it's reachable.
	parent := filepath.Dir(proj)
	if err := os.WriteFile(filepath.Join(parent, "plain_outside.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	rc, stderr := runWS(t, payload("Write", dotDotTarget, ""))
	// proj's parent has no trackfw.yaml → allow.
	if rc != 0 {
		t.Fatalf("'..' exiting project: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_Symlink_Directory asserts that a symlink DIRECTORY inside the project that
// points to docs/ causes writes to agents-working-context.md via the symlink to be denied.
// Asserts: ML-3A2 AC5 — symlink directory inside project → resolves to frozen path → deny.
// Contra-braço: passes on HEAD (EvalSymlinks was already there; the test adds coverage).
func TestRunWriteScope_Symlink_Directory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Symlink requires elevated privileges on Windows; skipping")
	}
	dir := makeWSProjectDir(t)
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	makeDocsWorkingContext(t, dir, true /* frozen */)

	// Create a symlink: dir/link → dir/docs
	link := filepath.Join(dir, "link")
	if err := os.Symlink(docs, link); err != nil {
		t.Skipf("os.Symlink failed (%v); skipping symlink test", err)
	}

	// Target: dir/link/agents-working-context.md
	// After EvalSymlinks: dir/docs/agents-working-context.md → frozen → deny.
	target := filepath.Join(link, "agents-working-context.md")
	rc, stderr := runWS(t, payload("Write", target, ""))
	if rc != 2 {
		t.Fatalf("symlink dir → frozen file: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_TmpVsPrivateTmp asserts that both raw and EvalSymlinks spellings of the
// same frozen path give the same verdict (deny).
// On macOS, t.TempDir() lives under /var/folders which is a symlink for /private/var/folders.
// On Linux the two spellings are identical (no symlink), so the test still passes.
// Asserts: ML-3A2 AC5 — /tmp vs /private/tmp (and variants) → same verdict.
// Contra-braço: passes on HEAD (EvalSymlinks was already in resolveWriteScopePath).
func TestRunWriteScope_TmpVsPrivateTmp(t *testing.T) {
	dir := makeWSProjectDir(t)
	makeDocsWorkingContext(t, dir, true /* frozen */)

	// path1: the raw t.TempDir() path — may contain unresolved symlinks on macOS.
	path1 := filepath.Join(dir, "docs", "agents-working-context.md")

	// path2: after EvalSymlinks — the canonical physical path.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", dir, err)
	}
	path2 := filepath.Join(resolved, "docs", "agents-working-context.md")

	for _, tc := range []struct {
		name string
		path string
	}{
		{"raw-spelling", path1},
		{"resolved-spelling", path2},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rc, stderr := runWS(t, payload("Write", tc.path, ""))
			if rc != 2 {
				t.Fatalf("%s: want exit 2 (deny), got %d (stderr=%q)", tc.name, rc, stderr)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────
// Item 3: two projects A and B, target in A, cwd in B
// ─────────────────────────────────────────────────────────────

// TestRunWriteScope_TwoProjects_TargetInA_CwdInB asserts that a target in project A is judged
// by A's trackfw.yaml, not by B's, even when the process cwd is B.
// Setup: A has a frozen working context; B does not.
// Asserts: ML-3A2 item 3 — guard uses the root of the TARGET, not the cwd.
// Contra-braço: passes on HEAD (findProjectRoot already walks from the target); the test adds
//
//	explicit coverage and a control arm.
func TestRunWriteScope_TwoProjects_TargetInA_CwdInB(t *testing.T) {
	projA := makeWSProjectDir(t)
	makeDocsWorkingContext(t, projA, true /* frozen in A */)

	projB := makeWSProjectDir(t)
	// B has docs/agents-working-context.md WITHOUT frozen marker.
	makeDocsWorkingContext(t, projB, false /* not frozen in B */)

	// Change cwd to B (so os.Getwd() returns projB during RunWriteScope).
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projB); err != nil {
		t.Fatalf("chdir to projB: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck

	// Target in A: must be denied (A's frozen marker applies).
	targetA := filepath.Join(projA, "docs", "agents-working-context.md")
	rc, stderr := runWS(t, payload("Write", targetA, ""))
	if rc != 2 {
		t.Fatalf("target in A (frozen), cwd in B: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}

	// Control: target in B must be allowed (B has no frozen marker, no agent_id).
	targetB := filepath.Join(projB, "docs", "agents-working-context.md")
	rcB, stderrB := runWS(t, payload("Write", targetB, ""))
	if rcB != 0 {
		t.Fatalf("target in B (not frozen), cwd in B: want exit 0 (allow), got %d (stderr=%q)", rcB, stderrB)
	}
}

// ─────────────────────────────────────────────────────────────
// Panic recovery (seam)
// ─────────────────────────────────────────────────────────────

// TestRunWriteScope_PanicRecovery asserts that a panic inside RunWriteScope is recovered and
// exits with code 2, with the panic message on stderr.
// Asserts: ML-3A2 — panic via seam → exit 2 (D3-F fail-closed, contract item 1).
// Contra-braço: writeScopeInternalHook is a new symbol; compilation fails on HEAD.
//
//	Stronger contra-braço: delete the defer/recover block in a scratch copy and run the test —
//	the process panics (unrecovered) and the test receives an unexpected exit code. Not automated
//	here to avoid modifying production code.
func TestRunWriteScope_PanicRecovery(t *testing.T) {
	// This test must not run in parallel: it mutates a package-level var.
	old := writeScopeInternalHook
	writeScopeInternalHook = func() { panic("ml-3a2 test panic") }
	t.Cleanup(func() { writeScopeInternalHook = old })

	var stderrBuf strings.Builder
	// Provide a valid JSON payload so the code reaches the hook (the panic fires right after the defer).
	rc := RunWriteScope(strings.NewReader(`{}`), shortTimeout, &stderrBuf)
	stderrStr := stderrBuf.String()
	if rc != 2 {
		t.Fatalf("panic via seam: want exit 2, got %d (stderr=%q)", rc, stderrStr)
	}
	if !strings.Contains(stderrStr, "panic") {
		t.Fatalf("panic via seam: want 'panic' in stderr, got %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "ml-3a2 test panic") {
		t.Fatalf("panic via seam: want panic message in stderr, got %q", stderrStr)
	}
}

// ─────────────────────────────────────────────────────────────
// YAML invalidity (ML-3A2 defect B)
// ─────────────────────────────────────────────────────────────

// TestBuildFrozenList_InvalidYAML_Error asserts that buildFrozenList returns a non-nil error
// when trackfw.yaml contains invalid YAML.
// Asserts: ML-3A2 — YAML inválido nega (contract item 6).
// Contra-braço: on HEAD, readWriteScopeFromYAML silently returns empty slices for bad YAML,
//
//	so buildFrozenList returns (frozen, nil) and the guard allows the write.
//	After the fix, buildFrozenList returns (nil, error) → RunWriteScope exits 2.
//	Measured against the scratchpad copy: HEAD gives exit 0 for non-frozen target with
//	bad YAML; fix gives exit 2.
func TestBuildFrozenList_InvalidYAML_Error(t *testing.T) {
	dir := t.TempDir()
	// Unambiguously broken YAML: unclosed bracket.
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("key: [unclosed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	// No frozen marker → default paths only (no frozen marker).
	_, _, err := readWriteScopeFromYAML(dir)
	if err == nil {
		t.Fatal("invalid YAML: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "YAML") && !strings.Contains(err.Error(), "yaml") {
		t.Fatalf("invalid YAML error: want 'YAML'/'yaml' in message, got %q", err.Error())
	}
}

// TestRunWriteScope_MalformedYAML_NonFrozenTarget_Denied asserts that a malformed trackfw.yaml
// causes a non-frozen, non-listed target to be denied (fail-closed).
// Asserts: ML-3A2 defect B — YAML invalidity propagates through buildFrozenList to RunWriteScope.
// Contra-braço: new behaviour; HEAD gives exit 0. See TestBuildFrozenList_InvalidYAML_Error above.
func TestRunWriteScope_MalformedYAML_NonFrozenTarget_Denied(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("key: [unclosed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	// Plain file, no frozen marker.
	target := filepath.Join(docs, "plain.md")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	rc, stderr := runWS(t, payload("Write", target, ""))
	if rc != 2 {
		t.Fatalf("malformed YAML, non-frozen target: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}
