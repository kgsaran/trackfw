package generators

import "testing"

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-17 Wave 2/ML-2C — hookArrayHasCommand/simpleArrayHasValue
// must compare hook "command" paths after normalizing incidental formatting
// (double slashes, trailing slash), not as raw strings. Root cause: macOS's
// $TMPDIR ends in "/", so a $HOME built under it ("$WORK/sub") can contain
// "//" once concatenated with a literal path segment; the writer computes
// scriptPath via filepath.Join (which normalizes), while a value written by
// hand or captured before normalization does not — a byte-for-byte compare
// then silently fails to dedup. Mirrored in npm/tests/
// guard_path_normalize.test.js and pypi/tests/test_guard_path_normalize.py.
// ---------------------------------------------------------------------------

func TestNormalizeGuardPath_Table(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/a/b", "/a/b"},
		{"//a/b", "/a/b"},
		{"/a//b", "/a/b"},
		{"/a/b/", "/a/b"},
		{"/a/b//", "/a/b"},
		{"//", "/"},
		{"/", "/"},
		{"/a/./b", "/a/./b"},       // deliberately NOT resolved -- see doc comment
		{"/a/b/../b", "/a/b/../b"}, // deliberately NOT resolved
	}
	for _, c := range cases {
		if got := normalizeGuardPath(c.in); got != c.want {
			t.Errorf("normalizeGuardPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSamePathCommand_ToleratesDoubleSlashAndTrailingSlash(t *testing.T) {
	a := "/var/folders/xx/yy/T//trackfw-falsify.ABC/s67-fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh"
	b := "/var/folders/xx/yy/T/trackfw-falsify.ABC/s67-fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh"
	if !samePathCommand(a, b) {
		t.Errorf("samePathCommand should tolerate the double-slash formatting produced by a trailing-slash $TMPDIR: %q vs %q", a, b)
	}
}

// TestSamePathCommand_DifferentPathsDoNotMatch is the non-regression half of
// risk #2 in the roadmap ("normalizar demais é perigoso"): two genuinely
// different scripts, or the same guard installed for two different users,
// must never compare equal. A passing "//" test alone would not catch an
// over-normalization bug (e.g. accidentally resolving ".." or symlinks).
func TestSamePathCommand_DifferentPathsDoNotMatch(t *testing.T) {
	cases := []struct{ a, b string }{
		{"/home/alice/.trackfw/scripts/trackfw-git-branch-guard.sh", "/home/bob/.trackfw/scripts/trackfw-git-branch-guard.sh"},
		{"/a/b/trackfw-git-branch-guard.sh", "/a/bb/trackfw-git-branch-guard.sh"},
		{"/a/b/trackfw-git-branch-guard.sh", "/a/b/trackfw-credential-guard.sh"},
		{"/a/b", "/a/b/c"},
	}
	for _, c := range cases {
		if samePathCommand(c.a, c.b) {
			t.Errorf("samePathCommand(%q, %q) should be false -- these are different paths", c.a, c.b)
		}
	}
}
