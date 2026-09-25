package pathguard

// resolve_root_test.go — ML-8A / #402: the contract of ResolveRoot, asserted.
//
// ResolveRoot became the single chokepoint every fixed site depends on: ~19 call
// sites now turn its error into pathguard.RefuseUnverifiableRoot. Its three
// behaviours are therefore load-bearing and none of them was asserted by name
// until this file existed.
//
// 🔴 The middle one is the one that must not drift. "Tidying" ResolveRoot to
// return an error when filepath.EvalSymlinks fails would make every one of those
// refusal branches fire for any destination whose directory does not exist yet —
// which is the normal case for a CLI that CREATES files. That is a fail-closed
// regression across the whole tree, and the T5 residual ML-9A declared says this
// fallback is deliberately unchanged.
//
// One sentence per test (Regra Dura de Reconciliação):
//
//   - TestResolveRootContract affirms ML-8A's conclusion that ResolveRoot moves a
//     root into the resolved namespace WITHOUT changing the T5 fallback: an empty
//     root is refused, a root whose path does not exist returns its absolute form
//     with a NIL error, and an existing root reached through a symlink comes back
//     resolved.
//   - TestResolveRootIsWhatTheGuardNeeds affirms ML-8A's conclusion that the value
//     ResolveRoot returns is the one RejectSymlinks can actually contain: a target
//     filepath.Join'ed onto it passes, and the same target spelled from the
//     unresolved root — the pre-ML-8A shape — is refused.

import (
	"os"
	"path/filepath"
	"testing"
)

// This file is in package pathguard (internal), so it uses
// symlinkOrSkipInPathguard — the twin in pathguard_test.go lives in the EXTERNAL
// test package and is not reachable from here.

func TestResolveRootContract(t *testing.T) {
	t.Run("empty-root-is-refused-never-substituted", func(t *testing.T) {
		// filepath.Abs("") returns the process working directory. Accepting it
		// would silently guard a root nobody asked for.
		got, err := ResolveRoot("")
		if err == nil {
			t.Fatalf("ResolveRoot(\"\") returned %q with a nil error — an empty root must never fall back to the process cwd", got)
		}
		if got != "" {
			t.Errorf("ResolveRoot(\"\") returned %q alongside its error — callers pass the result straight into a guard", got)
		}
	})

	t.Run("non-existent-root-falls-back-with-a-NIL-error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does", "not", "exist")
		got, err := ResolveRoot(missing)
		if err != nil {
			t.Fatalf("ResolveRoot refused a root that merely does not exist yet: %v\n"+
				"this is the T5 fallback ML-9A declared and ML-8A deliberately did not change; turning it into an error makes every RefuseUnverifiableRoot branch added by ML-8A fire on the normal create-a-new-file path", err)
		}
		if got != missing {
			t.Errorf("ResolveRoot(%q) = %q, want the absolute form unchanged", missing, got)
		}
	})

	t.Run("existing-root-behind-a-symlink-comes-back-resolved", func(t *testing.T) {
		base := t.TempDir()
		real := filepath.Join(base, "real")
		if err := os.MkdirAll(real, 0o755); err != nil {
			t.Fatalf("creating the real root: %v", err)
		}
		link := filepath.Join(base, "link")
		if !symlinkOrSkipInPathguard(t, real, link) {
			return
		}
		got, err := ResolveRoot(link)
		if err != nil {
			t.Fatalf("ResolveRoot(%q): %v", link, err)
		}
		want, err := filepath.EvalSymlinks(real)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q): %v", real, err)
		}
		if got != want {
			t.Fatalf("ResolveRoot(%q) = %q, want %q — a root still spelled through the symlink is exactly the argument #402 is about", link, got, want)
		}
	})
}

func TestResolveRootIsWhatTheGuardNeeds(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("creating the real root: %v", err)
	}
	link := filepath.Join(base, "link")
	if !symlinkOrSkipInPathguard(t, real, link) {
		return
	}

	resolved, err := ResolveRoot(link)
	if err != nil {
		t.Fatalf("ResolveRoot(%q): %v", link, err)
	}
	if guardErr := RejectSymlinks(resolved, filepath.Join(resolved, "docs", "note.md")); guardErr != nil {
		t.Fatalf("the guard refused a target derived from the resolved root: %v — then ResolveRoot is not producing what RejectSymlinks needs", guardErr)
	}

	// Control: the pre-ML-8A pairing. filepath.Clean of the symlinked root is
	// what 16 sites passed, and the walk trips on the root itself.
	if guardErr := RejectSymlinks(filepath.Clean(link), filepath.Join(link, "docs", "note.md")); guardErr == nil {
		t.Fatalf("RejectSymlinks accepted the filepath.Clean(root) pairing — if that shape already worked there would be nothing for ML-8A to fix, and this test would be asserting nothing")
	}
}
