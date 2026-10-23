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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
// Uses encoding/json so Windows backslash paths are escaped correctly.
func payload(tool, filePath, agentID string) string {
	m := map[string]any{
		"tool_name":  tool,
		"tool_input": map[string]any{"file_path": filePath},
	}
	if agentID != "" {
		m["agent_id"] = agentID
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic("payload: json.Marshal: " + err.Error())
	}
	return string(b)
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

// symlinkOrSkipInGuard creates a symlink from link to target.
// If creation fails due to a privilege error (Windows without Developer Mode,
// or permission-denied), the calling test is skipped.
// Any other error is a t.Fatalf.
// This is the capability guard required by scripts/check-symlink-privilege-guard.sh.
func symlinkOrSkipInGuard(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeErrorInGuard(err) {
		t.Skipf("guarda de symlink não exercitada: criação de symlink exige privilégio: %v", err)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeErrorInGuard reports whether err is a privilege error for symlink creation.
// Matches os.IsPermission and Windows WinError 1314 (ERROR_PRIVILEGE_NOT_HELD).
func isSymlinkPrivilegeErrorInGuard(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	if errors.As(err, &e) && e == 1314 {
		return true
	}
	return false
}

// TestRunWriteScope_Symlink_Directory asserts that a symlink DIRECTORY inside the project that
// points to docs/ causes writes to agents-working-context.md via the symlink to be denied.
// Asserts: ML-3A2 AC5 — symlink directory inside project → resolves to frozen path → deny.
// Contra-braço: passes on HEAD (EvalSymlinks was already there; the test adds coverage).
func TestRunWriteScope_Symlink_Directory(t *testing.T) {
	dir := makeWSProjectDir(t)
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0700); err != nil {
		t.Fatal(err)
	}
	makeDocsWorkingContext(t, dir, true /* frozen */)

	// Create a symlink: dir/link → dir/docs
	link := filepath.Join(dir, "link")
	if !symlinkOrSkipInGuard(t, docs, link) {
		return
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

// ─────────────────────────────────────────────────────────────
// D4 — permission by dispatch (ML-3B)
//
// Helper: makeLeaseFile writes .trackfw/leases/<agentType>.json.
// Helper: payloadFull builds a JSON payload using json.Marshal
//         (safe for Windows paths that contain backslashes).
// ─────────────────────────────────────────────────────────────

// makeLeaseFile creates a .trackfw/leases/<agentType>.json with the given content.
func makeLeaseFile(t *testing.T, projectRoot, agentType string, lease map[string]interface{}) string {
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
	return leasePath
}

// futureRFC3339 returns a future RFC 3339 timestamp (now + 1 hour).
func futureRFC3339() string { return time.Now().Add(time.Hour).Format(time.RFC3339) }

// pastRFC3339 returns a past RFC 3339 timestamp (now - 1 hour).
func pastRFC3339() string { return time.Now().Add(-time.Hour).Format(time.RFC3339) }

// payloadFull builds a JSON payload using json.Marshal to handle Windows backslashes safely.
func payloadFull(tool, filePath, agentID, agentType string) string {
	m := map[string]interface{}{
		"tool_name":  tool,
		"tool_input": map[string]string{"file_path": filePath},
	}
	if agentID != "" {
		m["agent_id"] = agentID
	}
	if agentType != "" {
		m["agent_type"] = agentType
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// payloadFullWithCwd builds a JSON payload that includes the cwd field.
// ML-3C: cwd is the session working directory; its project root is the second lease search root.
func payloadFullWithCwd(tool, filePath, agentID, agentType, cwd string) string {
	m := map[string]interface{}{
		"tool_name":  tool,
		"tool_input": map[string]string{"file_path": filePath},
	}
	if agentID != "" {
		m["agent_id"] = agentID
	}
	if agentType != "" {
		m["agent_type"] = agentType
	}
	if cwd != "" {
		m["cwd"] = cwd
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// TestRunWriteScope_D4_TargetInAllowDir_Allow asserts that a target inside a directory allow entry → 0.
// Asserts: ML-3B AC4 — alvo dentro do allow (directory entry) → 0.
// Contra-braço: on HEAD, D4 doesn't exist; RunWriteScope skips the lease check and allows the write
//
//	(exit 0) for the same reason — this test PASSES on HEAD for the wrong reason (no D4 check).
//	The test's value comes from the failing arms that confirm D4 was not present on HEAD.
func TestRunWriteScope_D4_TargetInAllowDir_Allow(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Allow entry: proj/ (directory, trailing slash).
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{proj + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 0 {
		t.Fatalf("target in allow dir: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_TargetInAllowFile_Allow asserts that a file-exact allow entry → 0.
// Asserts: ML-3B AC4 — file-exact allow entry matches the target → 0.
// Contra-braço: PASSES on HEAD (D4 absent → same exit 0 for wrong reason; see above).
func TestRunWriteScope_D4_TargetInAllowFile_Allow(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "lib.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package lib"), 0600); err != nil {
		t.Fatal(err)
	}

	// Allow entry: exact file path (no trailing slash).
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{target},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 0 {
		t.Fatalf("target matches file-exact allow: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_SiblingFileNotAllowed asserts that a sibling of the allow file is denied.
// Asserts: ML-3B — file-exact allow entry does NOT grant siblings → 2.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0; after fix → exit 2).
func TestRunWriteScope_D4_SiblingFileNotAllowed(t *testing.T) {
	proj := makeWSProjectDir(t)
	allowedFile := filepath.Join(proj, "src", "lib.go")
	if err := os.MkdirAll(filepath.Dir(allowedFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowedFile, []byte("package lib"), 0600); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(proj, "src", "other.go")
	if err := os.WriteFile(sibling, []byte("package lib"), 0600); err != nil {
		t.Fatal(err)
	}

	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{allowedFile},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", sibling, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("sibling not in allow: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "allow") && !strings.Contains(stderr, "not in") {
		t.Fatalf("deny message: want 'allow' or 'not in' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_TargetOutsideAllow_Deny asserts that a target not in any allow → 2 with reason.
// Asserts: ML-3B AC4 — alvo fora do allow → 2, com motivo.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_TargetOutsideAllow_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)

	// Allowed dir: a subdirectory.
	allowedDir := filepath.Join(proj, "src")
	if err := os.MkdirAll(allowedDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Target: a different subdirectory, outside allowedDir.
	target := filepath.Join(proj, "docs", "plan.md")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("plan"), 0600); err != nil {
		t.Fatal(err)
	}

	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{allowedDir + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("target outside allow: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	// Denial reason should name the target (or lease file).
	if !strings.Contains(stderr, "allow") && !strings.Contains(stderr, "not in") {
		t.Fatalf("deny message: want 'allow' or 'not in' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_NoLease_D3Only_Allow asserts that when no lease exists, only D3 applies.
// Asserts: ML-3B AC4 — sem permissão → só a D3 (non-frozen target allowed).
// Contra-braço: PASSES on HEAD (D4 absent → same D3-only behavior → exit 0).
func TestRunWriteScope_D4_NoLease_D3Only_Allow(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	// No lease file created.

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 0 {
		t.Fatalf("no lease → D3 only: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_Expired_Deny asserts that an expired lease → 2 with "expir" in stderr.
// Asserts: ML-3B AC4 — permissão expirada → 2.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_Expired_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease valid except that expires is in the past.
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{proj + "/"},
		"expires":    pastRFC3339(), // expired
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("expired lease: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "expir") {
		t.Fatalf("expired deny message: want 'expir' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_InvalidJSON_Deny asserts that an invalid JSON lease → 2.
// Asserts: ML-3B AC4 — JSON inválido → 2.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_InvalidJSON_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Write an invalid JSON lease.
	leasesDir := filepath.Join(proj, ".trackfw", "leases")
	if err := os.MkdirAll(leasesDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leasesDir, "apolo-tf.json"), []byte("{not valid json"), 0600); err != nil {
		t.Fatal(err)
	}

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("invalid JSON lease: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_LeaseIsDirectory_Deny asserts that when the lease path is a directory → 2.
// Asserts: ML-3B — lease ilegível (diretório) → 2 (D4-C).
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_LeaseIsDirectory_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create the lease path as a directory instead of a file.
	leasesDir := filepath.Join(proj, ".trackfw", "leases", "apolo-tf.json")
	if err := os.MkdirAll(leasesDir, 0700); err != nil {
		t.Fatal(err)
	}

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("lease is directory: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_VersionNotOne_Deny asserts that an unsupported lease version → 2.
// Asserts: ML-3B contract item 3 — version != 1 → 2.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_VersionNotOne_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    2, // unsupported
		"agent_type": "apolo-tf",
		"allow":      []string{proj + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("version 2 lease: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "version") {
		t.Fatalf("version deny message: want 'version' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_AgentTypeDivergent_Deny asserts that a divergent agent_type in the file → 2.
// Asserts: ML-3B AC4 — agent_type divergente → 2.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_AgentTypeDivergent_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// File agent_type "hades-tf" ≠ payload agent_type "apolo-tf".
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "hades-tf", // divergent
		"allow":      []string{proj + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("agent_type mismatch: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "agent_type") {
		t.Fatalf("mismatch deny message: want 'agent_type' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_RelativeAllow_Deny asserts that a relative allow entry → 2 even if it would match.
// Asserts: ML-3B AC4 — allow relativo → 2.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_RelativeAllow_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// One relative, one absolute (that covers the target).
	// The relative entry should trigger denial before the absolute entry is consulted.
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{"relative/path", proj + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("relative allow entry: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_FrozenInsideAllow_Deny asserts that a frozen path inside the allow list is still denied.
// Asserts: ML-3B AC4 — congelado dentro do allow → 2 (D3 runs before D4).
// Contra-braço: PASSES on HEAD (D3 already denies; D4 is absent but D3 denies first → exit 2).
func TestRunWriteScope_D4_FrozenInsideAllow_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)
	// Create frozen working context.
	makeDocsWorkingContext(t, proj, true /* frozen */)

	// Allow entry covers the whole project (including the frozen file).
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{proj + "/"},
		"expires":    futureRFC3339(),
	})

	target := filepath.Join(proj, "docs", "agents-working-context.md")
	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("frozen path inside allow: want exit 2 (deny by D3), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "frozen") {
		t.Fatalf("frozen deny message: want 'frozen' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_MainSession_NoAgentID_LeaseIgnored asserts that the main session ignores D4.
// Asserts: ML-3B AC4 — sessão principal (sem agent_id) ignora a permissão.
// A lease exists that would deny the target, but agent_id is absent → D3 only → allow.
// Contra-braço: PASSES on HEAD (D4 absent → D3 only → same exit 0).
func TestRunWriteScope_D4_MainSession_NoAgentID_LeaseIgnored(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease exists with allow pointing elsewhere — would deny if D4 applied.
	elsewhere := filepath.Join(proj, "other")
	if err := os.MkdirAll(elsewhere, 0700); err != nil {
		t.Fatal(err)
	}
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{elsewhere + "/"},
		"expires":    futureRFC3339(),
	})

	// No agent_id → main session.
	p := payloadFull("Write", target, "", "apolo-tf")
	rc, stderr := runWS(t, p)
	if rc != 0 {
		t.Fatalf("main session (no agent_id): want exit 0 (allow, D4 ignored), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_InvalidSlug_D3Only asserts that an agent_type with "/" → D3 only (not D4).
// Asserts: ML-3B AC4 — agent_type fora do padrão → só a D3 (D4-A slug validation).
// Contra-braço: PASSES on HEAD (D4 absent → D3 only → same exit 0 for non-frozen target).
func TestRunWriteScope_D4_InvalidSlug_D3Only(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// A lease exists for a slug that could be accessed via path traversal if slug validation failed.
	// With validation, "x/../apolo-tf" fails the slug check → D3 only.
	rc, stderr := runWS(t, payloadFull("Write", target, "x-id", "x/../apolo-tf"))
	if rc != 0 {
		t.Fatalf("invalid slug agent_type: want exit 0 (D3 only), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_DirBoundary_NoCross asserts /a/b/ does NOT match /a/bc/x.
// Asserts: ML-3B — directory allow entry uses separator-terminated prefix; /a/b/ doesn't match /a/bc.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0 for any non-frozen target in a project;
//
//	but the test is meaningful with D4: it verifies the separator boundary).
func TestRunWriteScope_D4_DirBoundary_NoCross(t *testing.T) {
	proj := makeWSProjectDir(t)

	// allowedDir = proj/src
	allowedDir := filepath.Join(proj, "src")
	if err := os.MkdirAll(allowedDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create proj/srcmore/file.go — starts with the same prefix as "src/" but is a sibling.
	srcmore := filepath.Join(proj, "srcmore")
	if err := os.MkdirAll(srcmore, 0700); err != nil {
		t.Fatal(err)
	}
	crossFile := filepath.Join(srcmore, "file.go")
	if err := os.WriteFile(crossFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{allowedDir + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", crossFile, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("directory boundary: want exit 2 (deny; proj/srcmore not in proj/src/), got %d (stderr=%q)", rc, stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// D4 — Caso de 2026-10-23: worktrees paralelos
//
// Projeto A = checkout principal, Projeto B = worktree.
// Cada um tem trackfw.yaml. A permissão é lida da raiz do ALVO.
// ─────────────────────────────────────────────────────────────

// TestRunWriteScope_D4_Worktree_VariantA_Deny asserts Variant A: lease in A, allow=[B/], target in A → 2.
// Asserts: ML-3B test of "caso de 2026-10-23" — lease in A allows only B, write targets A → denied.
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestRunWriteScope_D4_Worktree_VariantA_Deny(t *testing.T) {
	projA := makeWSProjectDir(t) // simulates main checkout
	projB := makeWSProjectDir(t) // simulates worktree

	// Target in A (non-frozen).
	targetInA := filepath.Join(projA, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(targetInA), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetInA, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease in A: allow = [projB/] — the agent is permitted to write in B, not A.
	makeLeaseFile(t, projA, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projB + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", targetInA, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("variant A (lease in A, allow=B, target=A): want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_Worktree_VariantB_Deny asserts Variant B (ML-3C): lease in B (cwd project),
// allow=[B/], target in A, cwd=B → 2.
// With ML-3C two-root search, the lease in B (the cwd project) is found. The target is in A, which
// is NOT in the allow list (only B/ is allowed), so the write is denied.
//
// Asserts: ML-3C contract item 2 — the case of 2026-10-23 (lease in cwd, allow=worktree, target
// in main checkout) is now denied. The cwd root is searched in addition to the target root.
//
// Contra-braço: FAILS on HEAD (cwd root not searched → lease in B never found → D3 only → exit 0).
func TestRunWriteScope_D4_Worktree_VariantB_Deny(t *testing.T) {
	projA := makeWSProjectDir(t) // simulates main checkout (target project)
	projB := makeWSProjectDir(t) // simulates worktree (cwd project, where lease lives)

	// Target in A (non-frozen).
	targetInA := filepath.Join(projA, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(targetInA), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetInA, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease in B (the cwd project). A has no lease.
	// The allow list only covers projB — the agent is supposed to write in B, not A.
	makeLeaseFile(t, projB, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projB + "/"},
		"expires":    futureRFC3339(),
	})

	// Writing to A with cwd=B: ML-3C searches B (cwd root) and finds the lease.
	// Target (in A) is not in allow=[B/] → denied.
	rc, stderr := runWS(t, payloadFullWithCwd("Write", targetInA, "apolo-tf-id", "apolo-tf", projB))
	if rc != 2 {
		t.Fatalf("variant B (lease in B, cwd=B, target=A): want exit 2 (deny — target not in allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_CwdLeaseAllowTarget_Allow asserts that a lease only in the cwd root,
// with the target inside its allow list, results in exit 0.
// Asserts: ML-3C AC — permissão só na raiz do cwd, alvo dentro do allow → 0.
// Contra-braço: PASSES on HEAD (no D4 logic → D3 only → allow for non-frozen target).
// This is a fixation test: it documents the correct behavior and ensures ML-3C doesn't break it.
func TestRunWriteScope_D4_CwdLeaseAllowTarget_Allow(t *testing.T) {
	projA := makeWSProjectDir(t) // target project (no lease)
	projB := makeWSProjectDir(t) // cwd project (has lease)

	// Target in A — inside a subdirectory that the lease allows.
	targetInA := filepath.Join(projA, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(targetInA), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetInA, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease in B (cwd project): allow = [projA/] — the agent is permitted to write anywhere in A.
	makeLeaseFile(t, projB, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projA + "/"},
		"expires":    futureRFC3339(),
	})

	// Writing to A with cwd=B: lease in B allows projA/ → target is in allow → exit 0.
	rc, stderr := runWS(t, payloadFullWithCwd("Write", targetInA, "apolo-tf-id", "apolo-tf", projB))
	if rc != 0 {
		t.Fatalf("cwd-lease allow target: want exit 0 (allow — target in cwd lease), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_InvalidLeaseInCwd_ValidInTarget_Deny asserts that an invalid lease in
// the cwd root denies even when a valid lease in the target root would allow the write.
// Asserts: ML-3C AC — permissão inválida no cwd e válida no alvo → 2.
// Uses an expired lease (not bad JSON) to exercise the validate-all-before-match path.
// Contra-braço: FAILS on HEAD (no cwd root search → only target root, valid lease → exit 0).
func TestRunWriteScope_D4_InvalidLeaseInCwd_ValidInTarget_Deny(t *testing.T) {
	projA := makeWSProjectDir(t) // target project (has valid lease)
	projB := makeWSProjectDir(t) // cwd project (has expired/invalid lease)

	target := filepath.Join(projA, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Valid lease in A (target project): allows the target.
	makeLeaseFile(t, projA, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projA + "/"},
		"expires":    futureRFC3339(),
	})

	// EXPIRED lease in B (cwd project): invalid because it has expired.
	// Using an expired lease (not bad JSON) so it passes load but fails validation.
	makeLeaseFile(t, projB, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projA + "/"},
		"expires":    pastRFC3339(), // expired
	})

	// Writing to A with cwd=B: expired lease in B → deny (validate-all-before-match).
	rc, stderr := runWS(t, payloadFullWithCwd("Write", target, "apolo-tf-id", "apolo-tf", projB))
	if rc != 2 {
		t.Fatalf("invalid lease in cwd + valid in target: want exit 2 (deny — expired cwd lease), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "expir") {
		t.Fatalf("deny message: want 'expir' in stderr, got %q", stderr)
	}
}

// TestRunWriteScope_D4_CwdAbsent_TargetRootOnly asserts that when cwd is absent from the payload,
// only the target project root is searched for leases.
// Asserts: ML-3C AC — cwd ausente → só a raiz do alvo.
// Contra-braço: PASSES on HEAD (same single-root behavior → no change in outcome).
// Fixation test: documents that missing cwd does not break existing D4 behavior.
func TestRunWriteScope_D4_CwdAbsent_TargetRootOnly(t *testing.T) {
	projA := makeWSProjectDir(t) // target project (has lease allowing the target)

	target := filepath.Join(projA, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Valid lease in A: allows the target directory.
	makeLeaseFile(t, projA, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projA + "/"},
		"expires":    futureRFC3339(),
	})

	// Payload has no cwd field → only target root searched → lease in A found → target allowed → 0.
	rc, stderr := runWS(t, payloadFull("Write", target, "apolo-tf-id", "apolo-tf"))
	if rc != 0 {
		t.Fatalf("cwd absent: want exit 0 (allow — target in A's lease), got %d (stderr=%q)", rc, stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// Decision 2 (ML-4C) — Hardlink to frozen file is denied
// ─────────────────────────────────────────────────────────────

// hardlinkOrSkip creates a hardlink from link to target.
// Skips the test if the OS/FS does not support hardlinks (different devices, permission).
func hardlinkOrSkip(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Link(target, link)
	if err == nil {
		return true
	}
	// Cross-device link, or FS that does not support hardlinks → skip.
	// os.IsPermission covers permission issues; syscall.EXDEV covers cross-device.
	if os.IsPermission(err) || errors.Is(err, syscall.EXDEV) {
		t.Skipf("hardlink not supported on this FS: %v", err)
		return false
	}
	t.Fatalf("os.Link(%q, %q): %v", target, link, err)
	return false
}

// TestRunWriteScope_Hardlink_FrozenFile_Deny asserts that a hardlink pointing to the frozen
// docs/agents-working-context.md is denied for the main session (when the marker is present).
//
// Attack vector: attacker creates a hardlink at an arbitrary path. The two paths share an inode;
// EvalSymlinks resolves symlinks but returns the hardlink's own path — the string comparison fails.
// os.SameFile(fi1, fi2) detects the shared inode.
//
// Asserts: Decision 2 (ML-4C) — os.SameFile used in pathMatchesFrozen → hardlink denied.
// Contra-braço: FAILS on HEAD (no os.SameFile → exit 0, hardlink bypasses frozen-path check).
func TestRunWriteScope_Hardlink_FrozenFile_Deny(t *testing.T) {
	dir := makeWSProjectDir(t)
	makeDocsWorkingContext(t, dir, true /* frozen */)

	// Create a hardlink to docs/agents-working-context.md at a different path.
	original := filepath.Join(dir, "docs", "agents-working-context.md")
	alias := filepath.Join(dir, "docs", "frozen-alias.md")
	if !hardlinkOrSkip(t, original, alias) {
		return
	}

	// Target the alias path — not the frozen path by name.
	rc, stderr := runWS(t, payload("Write", alias, ""))
	if rc != 2 {
		t.Fatalf("hardlink to frozen file: want exit 2 (deny — same inode), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_Hardlink_FrozenFile_Deny_Subagent asserts the same hardlink denial for a
// subagent (agent_id present), where docs/agents-working-context.md is always frozen.
//
// Asserts: Decision 2 (ML-4C) — hardlink denied for subagent arm (no frozen marker needed).
// Contra-braço: FAILS on HEAD (no os.SameFile → exit 0).
func TestRunWriteScope_Hardlink_FrozenFile_Deny_Subagent(t *testing.T) {
	dir := makeWSProjectDir(t)
	// For subagents, agents-working-context.md is always frozen (no marker needed).
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(dir, "docs", "agents-working-context.md")
	if err := os.WriteFile(original, []byte("no marker\n"), 0600); err != nil {
		t.Fatal(err)
	}

	alias := filepath.Join(dir, "docs", "hl-alias.md")
	if !hardlinkOrSkip(t, original, alias) {
		return
	}

	rc, stderr := runWS(t, payload("Write", alias, "subagent-id"))
	if rc != 2 {
		t.Fatalf("hardlink to frozen file (subagent): want exit 2, got %d (stderr=%q)", rc, stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// Decision 3 (ML-4C) — All leases must allow (attack 2E)
// ─────────────────────────────────────────────────────────────

// TestRunWriteScope_D4_2E_FakeProjectPermissive_Deny reproduces attack 2E (Decision 3, ML-4C):
// A fake cwd project F has a permissive lease (allow=[<parent>/]). The real target project A
// has a restrictive lease (allow=[A/src/allowed.go]). The target is A/src/x.go (not allowed).
//
// Under the old "at least one lease allows" rule: F's permissive lease would grant access → exit 0.
// Under the new "all leases must allow" rule: A's restrictive lease denies → exit 2.
//
// Asserts: Decision 3 (ML-4C) — all found leases must authorize the target.
// Contra-braço: FAILS on HEAD (old "at least one" rule → F's permissive lease is enough → exit 0).
func TestRunWriteScope_D4_2E_FakeProjectPermissive_Deny(t *testing.T) {
	projA := makeWSProjectDir(t) // checkout principal — has restrictive lease
	projF := makeWSProjectDir(t) // fake cwd project — has permissive lease

	// Target in A (non-frozen).
	targetInA := filepath.Join(projA, "src", "x.go")
	if err := os.MkdirAll(filepath.Dir(targetInA), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetInA, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease in A: only allows src/allowed.go — NOT src/x.go.
	makeLeaseFile(t, projA, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{filepath.Join(projA, "src", "allowed.go")},
		"expires":    futureRFC3339(),
	})

	// Lease in F: permissive — allows the entire parent of projA.
	// Uses the parent directory of projA's parent to cover both projA and projF.
	commonParent := filepath.Dir(projA) + string(filepath.Separator)
	makeLeaseFile(t, projF, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{commonParent},
		"expires":    futureRFC3339(),
	})

	// Agent runs in F (fake cwd) and tries to write to A/src/x.go.
	// Old rule: F's permissive lease covers A/src/x.go → allow (bypass!).
	// New rule: A's restrictive lease does NOT cover A/src/x.go → deny.
	rc, stderr := runWS(t, payloadFullWithCwd("Write", targetInA, "apolo-sess-001", "apolo-tf", projF))
	if rc != 2 {
		t.Fatalf("attack 2E: want exit 2 (deny — A's restrictive lease not satisfied), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_2E_BothLeasesAllow_Allow asserts the benign case of Decision 3:
// when both A and F have leases that authorize the target, the write is allowed.
//
// Asserts: Decision 3 (ML-4C) — both leases authorize → allow.
// Contra-braço: PASSES on HEAD (old rule: at least one allows → exit 0 for same reason).
func TestRunWriteScope_D4_2E_BothLeasesAllow_Allow(t *testing.T) {
	projA := makeWSProjectDir(t)
	projF := makeWSProjectDir(t)

	targetInA := filepath.Join(projA, "src", "x.go")
	if err := os.MkdirAll(filepath.Dir(targetInA), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetInA, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Both leases allow the target directory in A.
	makeLeaseFile(t, projA, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projA + "/"},
		"expires":    futureRFC3339(),
	})
	makeLeaseFile(t, projF, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{projA + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFullWithCwd("Write", targetInA, "apolo-sess-001", "apolo-tf", projF))
	if rc != 0 {
		t.Fatalf("both leases allow: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// ─────────────────────────────────────────────────────────────
// Decision 5 (ML-4C) — Trailing slash does not match same-named file
// ─────────────────────────────────────────────────────────────

// TestRunWriteScope_D4_TrailingSlash_DoesNotMatchFile_Deny asserts that an allow entry
// ending in "/" does NOT match the file at the same cleaned path.
//
// Example: allow entry "/path/src/" must NOT match the file "/path/src" (a file with that name).
// It only matches files INSIDE the directory (prefix matching).
//
// This prevents an accidental or malicious allow entry like "/path/file.go/" from granting
// access to "/path/file.go" as an exact-file match via filepath.Clean stripping the slash.
//
// Asserts: Decision 5 (ML-4C) — trailing-slash entry matches only content inside the directory.
// Contra-braço: FAILS on HEAD (old code: pathEqual(resolved, allowDir) matches file with dir name → exit 0).
func TestRunWriteScope_D4_TrailingSlash_DoesNotMatchFile_Deny(t *testing.T) {
	proj := makeWSProjectDir(t)

	// Create a file at proj/src (no extension — simulates a file accidentally named as dir).
	srcFile := filepath.Join(proj, "src")
	if err := os.WriteFile(srcFile, []byte("not-a-dir\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease allows "proj/src/" — trailing slash means "inside the src directory".
	// The file proj/src is NOT inside proj/src/ — it IS proj/src.
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{srcFile + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", srcFile, "apolo-tf-id", "apolo-tf"))
	if rc != 2 {
		t.Fatalf("trailing slash must not match file of same name: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestRunWriteScope_D4_TrailingSlash_MatchesContentInside_Allow asserts that a trailing-slash
// allow entry correctly matches files INSIDE the directory.
//
// Asserts: Decision 5 (ML-4C) — trailing-slash entry still grants access to content inside the dir.
// Contra-braço: PASSES on HEAD (directory content matching was already correct).
func TestRunWriteScope_D4_TrailingSlash_MatchesContentInside_Allow(t *testing.T) {
	proj := makeWSProjectDir(t)

	srcDir := filepath.Join(proj, "src")
	if err := os.MkdirAll(srcDir, 0700); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(srcDir, "main.go")
	if err := os.WriteFile(targetFile, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease allows "proj/src/" — content inside src/ should be allowed.
	makeLeaseFile(t, proj, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{srcDir + "/"},
		"expires":    futureRFC3339(),
	})

	rc, stderr := runWS(t, payloadFull("Write", targetFile, "apolo-tf-id", "apolo-tf"))
	if rc != 0 {
		t.Fatalf("trailing slash must match content inside dir: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}
