package thirdparty

// resolved_root_test.go — ML-8A / #402: armadilha 3 of Decisão 2 of
// REQ-2026-08-31, falsified on a real write site.
//
// # The trap
//
// "Destino resolvido contra root não resolvido → falso positivo; medido
// /tmp → /private/tmp no macOS." RejectSymlinks walks the target upwards
// comparing each component against root, and stops on string equality with
// root. If the two live in different namespaces the walk never reaches the
// stop condition — it either refuses a write that is genuinely inside the
// tree, or (with the ancestor symlink being the root's own) refuses on the
// root itself. filepath.Clean(root), which is what this site used before
// ML-8A, normalises TEXT and never resolves a symlink, so it could not
// produce the namespace the target lives in.
//
// # One sentence per test (Regra Dura de Reconciliação)
//
//   - TestWriteQuarantineAcceptsARootReachedThroughASymlink affirms ML-8A's
//     conclusion that deriving the guard root from a resolver closes armadilha
//     3: a root handed in through a symlinked path now writes successfully, and
//     the bytes land inside the real tree — the same call refuses before the fix.
//   - TestWriteQuarantineStillRefusesASymlinkedAncestorUnderASymlinkedRoot
//     affirms ML-8A's conclusion that the acceptance above is NOT a disabled
//     guard: with the root still reached through a symlink, a symlinked ancestor
//     inside it is still refused, and nothing is written.
//   - TestWriteQuarantineStillBoundsTheScopeToTheGivenRoot affirms ML-8A's
//     conclusion that the guard scope is still the ROOT it was given and not
//     something wider: a destination that escapes the root by traversal is
//     refused even though the root was resolved.
//
// 🔴 The second and third tests are the "controle degradado" arm the AC of
// ML-8A demands: a test that only asserts rc=0 and "the file is there" passes
// with the guard switched off, which is the degeneration ML-7A already paid for
// once in this REQ.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// symlinkOrSkip — intentional per-package copy; _test.go symbols are not
// importable across Go packages. Same logic as internal/discover,
// internal/generators and internal/integrations; any divergence is a bug.
func symlinkOrSkip(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeError(err) {
		t.Skipf("symlink guard not exercised: creating a symlink requires Developer Mode (or an elevated process) on this Windows: %v", err)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeError detects WinError 1314 (ERROR_PRIVILEGE_NOT_HELD).
// 🔴 It is kept as a SEPARATE function immediately below the t.Fatalf above on
// purpose: scripts/check-symlink-privilege-guard.sh requires the guard token
// (symlinkOrSkip|isSymlinkPrivilegeError) within ±5 lines of every match of
// `os.Symlink(`, and the format string in that t.Fatalf is itself a match.
func isSymlinkPrivilegeError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == 1314 {
		return true
	}
	return false
}

const probeChecksum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func probeEntry(checksum string) QuarantineEntry {
	return QuarantineEntry{ChecksumSHA256: checksum}
}

// symlinkedRoot builds  <tmp>/real  and  <tmp>/link → <tmp>/real  and returns
// both. `link` is the /tmp → /private/tmp shape of the measurement, expressed
// portably: a root the caller names through a symlink.
func symlinkedRoot(t *testing.T) (real, link string) {
	t.Helper()
	base := t.TempDir()
	real = filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("creating the real root: %v", err)
	}
	link = filepath.Join(base, "link")
	if !symlinkOrSkip(t, real, link) {
		t.FailNow()
	}
	return real, link
}

func TestWriteQuarantineAcceptsARootReachedThroughASymlink(t *testing.T) {
	real, link := symlinkedRoot(t)

	if err := WriteQuarantine(link, probeEntry(probeChecksum)); err != nil {
		t.Fatalf("WriteQuarantine refused a root reached through a symlink: %v\n"+
			"this is armadilha 3 of Decisão 2: the guard root was in the logical namespace (filepath.Clean never resolves) while the walk it performs is over the resolved one, so the ancestor that IS the root reads as a symlink and the write is refused", err)
	}

	// The bytes must be inside the REAL tree — "no error" alone is satisfied by a
	// write that went somewhere else entirely.
	written := filepath.Join(real, ".trackfw", "thirdparty-quarantine", probeChecksum+".json")
	if _, err := os.Stat(written); err != nil {
		t.Fatalf("no quarantine record at %s: %v — rc=0 without the artifact inside the root is not containment, it is a write the test failed to locate", written, err)
	}
}

func TestWriteQuarantineStillRefusesASymlinkedAncestorUnderASymlinkedRoot(t *testing.T) {
	real, link := symlinkedRoot(t)

	// The escape target: a directory OUTSIDE the root, reached by planting a
	// symlink at the ancestor the write must traverse.
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("creating the outside dir: %v", err)
	}
	if !symlinkOrSkip(t, outside, filepath.Join(real, ".trackfw")) {
		return
	}

	err := WriteQuarantine(link, probeEntry(probeChecksum))
	if err == nil {
		t.Fatalf("WriteQuarantine ACCEPTED a write through a symlinked ancestor — the acceptance of the previous test would then mean the guard is off, not that the root is now resolved")
	}
	if !strings.Contains(err.Error(), "refusing") {
		t.Errorf("the refusal does not come from the single emitter's grammar: %v", err)
	}
	// And nothing may have been created through the link.
	escaped := filepath.Join(outside, "thirdparty-quarantine", probeChecksum+".json")
	if _, statErr := os.Stat(escaped); statErr == nil {
		t.Fatalf("the write escaped to %s — the refusal above happened after the bytes were out", escaped)
	}
}

func TestWriteQuarantineStillBoundsTheScopeToTheGivenRoot(t *testing.T) {
	real, link := symlinkedRoot(t)

	// A checksum carrying traversal: QuarantinePath joins it under the root, so a
	// guard whose scope silently widened (to the parent, or to "/") would let it
	// through. The scope must still be the root that was handed in.
	traversing := filepath.Join("..", "..", "..", "escaped")
	err := WriteQuarantine(link, probeEntry(traversing))
	if err == nil {
		t.Fatalf("WriteQuarantine ACCEPTED a destination that escapes the root by traversal — resolving the root must not widen the scope of the guard")
	}
	if !strings.Contains(err.Error(), "escapes root") {
		t.Errorf("the refusal is not the containment one: %v", err)
	}
	if entries, readErr := os.ReadDir(filepath.Dir(real)); readErr == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "escaped") {
				t.Fatalf("a file named %q was created beside the root — the traversal was refused only after the write", entry.Name())
			}
		}
	}
}
