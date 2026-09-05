package integrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectNameCollision_ENOTDIRIsReportedNotSwallowed is a characterization
// test for the ENOTDIR branch of detectNameCollision (issue #276,
// manager.go:477), not a regression guard for the os.IsNotExist ->
// errors.Is(err, fs.ErrNotExist) swap. It builds a genuine ENOTDIR condition
// (a real file where a directory component is expected — blocker.txt/agent.md)
// and asserts the scan surfaces an error instead of silently returning nil.
//
// Both predicates pass this test identically. Read from GOROOT
// (src/os/error.go, src/syscall/syscall_{unix,windows}.go,
// src/syscall/zerrors_windows.go): os.IsNotExist(err) is
// underlyingErrorIs(err, ErrNotExist), which for a single-level *PathError
// unwraps exactly once and calls the same syscall.Errno.Is(fs.ErrNotExist)
// that errors.Is(err, fs.ErrNotExist) calls — for this exact error shape
// (os.ReadDir's PathError-wrapped Errno, unwrapped no further before the
// check) the two predicates are provably identical, on every platform Go
// supports, not just the ones measured here. On Windows,
// `ENOTDIR Errno = ERROR_PATH_NOT_FOUND` (zerrors_windows.go) — the same
// numeric code Windows returns for a genuinely absent parent directory — so
// neither predicate, nor the issue's own proposed errors.Is(err,
// syscall.ENOENT), can separate "blocked by a file" from "doesn't exist yet"
// at this call site on Windows without inspecting the offending path
// component directly. See
// vault/notes/go-isnotexist-vs-errors-is-fs-errnotexist-single-level-unwrap-2026-09-05.md.
func TestDetectNameCollision_ENOTDIRIsReportedNotSwallowed(t *testing.T) {
	root := t.TempDir()

	// Create a plain file where the destination expects a directory
	// component: root/blocker.txt/agent.md — "blocker.txt" is a file, not a
	// directory, so filepath.Dir(destination) resolves to a path that
	// cannot exist as a directory. Reading it yields ENOTDIR.
	blocker := filepath.Join(root, "blocker.txt")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("setup: write blocker file: %v", err)
	}

	destination := filepath.Join(blocker, "agent.md")
	content := []byte("---\nname: \"zeus\"\n---\nbody\n")

	item := resolvedPlan{
		plan: PlannedArtifact{
			Claim:   Claim{Kind: KindAgents},
			Content: content,
		},
		destination: destination,
	}

	err := detectNameCollision(item, false)
	if err == nil {
		t.Fatalf("detectNameCollision(ENOTDIR) = nil, want a reported error (ENOTDIR must not be classified as absence)")
	}
	if !strings.Contains(err.Error(), "scan") || !strings.Contains(err.Error(), "name collisions") {
		t.Fatalf("detectNameCollision(ENOTDIR) = %q, want the %q wrapper naming the scanned directory", err.Error(), "scan %q for name collisions")
	}
	t.Logf("ENOTDIR reported as expected: %v", err)
}

// TestDetectNameCollision_TrulyAbsentDirectoryIsSuppressed is the control
// case: a directory that genuinely does not exist (ENOENT, not ENOTDIR)
// must still be silently accepted — that branch is intentional (a brand
// new destination directory that hasn't been created yet is not a
// collision).
func TestDetectNameCollision_TrulyAbsentDirectoryIsSuppressed(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "does-not-exist-yet", "agent.md")
	content := []byte("---\nname: \"zeus\"\n---\nbody\n")

	item := resolvedPlan{
		plan: PlannedArtifact{
			Claim:   Claim{Kind: KindAgents},
			Content: content,
		},
		destination: destination,
	}

	if err := detectNameCollision(item, false); err != nil {
		t.Fatalf("detectNameCollision(truly absent dir) = %v, want nil (ENOENT must still be suppressed)", err)
	}
}
