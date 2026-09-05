package integrations

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDetectNameCollision_ENOTDIRIsPlatformDependent is a characterization
// test for the ENOTDIR branch of detectNameCollision (issue #276,
// manager.go:477), not a regression guard for the os.IsNotExist ->
// errors.Is(err, fs.ErrNotExist) swap — both predicates behave identically
// here (see below). It builds a genuine ENOTDIR condition (a real file where
// a directory component is expected — blocker.txt/agent.md) and asserts
// what was actually measured, which differs by platform:
//
// Read from GOROOT (src/os/error.go, src/syscall/syscall_{unix,windows}.go,
// src/syscall/zerrors_windows.go): os.IsNotExist(err) is
// underlyingErrorIs(err, ErrNotExist), which for a single-level *PathError
// unwraps exactly once and calls the same syscall.Errno.Is(fs.ErrNotExist)
// that errors.Is(err, fs.ErrNotExist) calls — for this exact error shape
// (os.ReadDir's PathError-wrapped Errno, unwrapped no further before the
// check) the two predicates are provably identical, on every platform Go
// supports, not just the ones measured here.
//
// On POSIX, ENOTDIR is a distinct errno from ENOENT, and
// syscall.Errno.Is(fs.ErrNotExist) does not match it — so
// detectNameCollision reports an error: "blocked by a file" is
// distinguishable from "doesn't exist yet".
//
// On Windows, GOROOT reading (zerrors_windows.go) shows `ENOTDIR Errno =
// ERROR_PATH_NOT_FOUND` — the same numeric code Windows returns for a
// genuinely absent parent directory, so `Errno.Is(fs.ErrNotExist)` would
// match it. What CI run 33991655271 actually measured, narrower than that
// GOROOT chain, is: os.ReadDir on this exact plain-file-as-directory setup
// returns an error for which errors.Is(err, fs.ErrNotExist) is true on
// windows-latest, so detectNameCollision takes the `return nil` branch —
// the collision scan is silently suppressed, exactly as it would be for a
// truly-absent directory. The Windows branch below asserts both the raw
// os.ReadDir error (the measured fact) and the detectNameCollision outcome
// it implies, so a future divergence between the two would fail loudly
// instead of the test just going green for the wrong reason. This is not a
// bug in detectNameCollision; it is the Windows API conflating the two
// conditions, and neither predicate, nor the issue's own proposed
// errors.Is(err, syscall.ENOENT), can separate them at this call site
// without inspecting the offending path component directly (out of scope —
// see the vault note below).
//
// This asymmetry is only real if a Windows CI run actually exercises the
// Windows branch below: this repository's CI runs this package's suite on
// windows-latest (.github/workflows/quality.yml, job
// windows-full-suites), which is the only execution of this assertion
// against real Windows syscalls available to this project — a green run on
// macOS/Linux proves only the POSIX branch.
//
// See vault/notes/go-isnotexist-e-errors-is-fs-errnotexist-sao-o-mesmo-predicado-para-pathError-de-um-nivel-2026-09-05.md.
func TestDetectNameCollision_ENOTDIRIsPlatformDependent(t *testing.T) {
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

	if runtime.GOOS == "windows" {
		// Assert the mechanism, not just the outcome: confirm os.ReadDir on
		// this plain-file-as-directory really produced an fs.ErrNotExist-
		// matching error on this Windows run, before trusting that
		// detectNameCollision's nil return is caused by that (and not by,
		// e.g., os.ReadDir unexpectedly succeeding on a file handle).
		_, rawErr := os.ReadDir(blocker)
		if rawErr == nil {
			t.Fatalf("setup: os.ReadDir(%q) succeeded on Windows; the ENOTDIR condition this test depends on was never built", blocker)
		}
		if !errors.Is(rawErr, fs.ErrNotExist) {
			t.Fatalf("os.ReadDir(file-as-directory) = %v, want errors.Is(err, fs.ErrNotExist) == true on Windows (ENOTDIR expected to collide with ERROR_PATH_NOT_FOUND) — if this fires, the ML-1C/ML-1A measurement for this call site is wrong and detectNameCollision's platform-dependent branch above needs re-deriving, not just re-skipping", rawErr)
		}
		t.Logf("windows: raw os.ReadDir error = %v (errors.Is(fs.ErrNotExist) = true)", rawErr)

		// Measured: given the raw error confirmed above, detectNameCollision
		// takes its errors.Is(err, fs.ErrNotExist) branch and suppresses the
		// scan exactly like a genuinely absent directory. Asserting err ==
		// nil here is the documented platform limitation, not an aspiration
		// to fix it.
		if err != nil {
			t.Fatalf("detectNameCollision(ENOTDIR) = %v, want nil on Windows (ENOTDIR collides with ERROR_PATH_NOT_FOUND, so it is classified as absence — see the doc comment above)", err)
		}
		t.Logf("ENOTDIR silently suppressed on Windows as expected (ENOTDIR == ERROR_PATH_NOT_FOUND == fs.ErrNotExist): %v", err)
		return
	}

	if err == nil {
		t.Fatalf("detectNameCollision(ENOTDIR) = nil, want a reported error on %s (ENOTDIR must not be classified as absence)", runtime.GOOS)
	}
	if !strings.Contains(err.Error(), "scan") || !strings.Contains(err.Error(), "name collisions") {
		t.Fatalf("detectNameCollision(ENOTDIR) = %q, want the %q wrapper naming the scanned directory", err.Error(), "scan %q for name collisions")
	}
	t.Logf("ENOTDIR reported as expected on %s: %v", runtime.GOOS, err)
}

// TestDetectNameCollision_TrulyAbsentDirectoryIsSuppressed is the control
// case: a directory that genuinely does not exist (ENOENT, not ENOTDIR)
// must still be silently accepted — that branch is intentional (a brand
// new destination directory that hasn't been created yet is not a
// collision). It affirms the ML-1C decision that the
// errors.Is(err, fs.ErrNotExist) suppression branch in detectNameCollision
// is deliberate for this case (e.g. `~/.claude/agents/` not yet created on
// a clean install), not something ML-1A's reconciliation should touch.
//
// On Windows, this test and TestDetectNameCollision_ENOTDIRIsPlatformDependent
// converge to the same nil outcome — that pair stops discriminating "blocked
// by a file" (ENOTDIR) from "doesn't exist yet" (ENOENT) there, which is
// exactly the platform limitation documented above, not a gap this test can
// close on its own.
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
