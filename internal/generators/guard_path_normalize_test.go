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

// TestNormalizeGuardPath_WindowsSeparators is the ML-7B table: drive-letter
// anchored input gets "\" canonicalized to "/" (closing the seam ML-7A
// measured: filepath.Join always emits "\" on Windows, so a hand- or
// concat-built command with "/" never compared equal to the computed one),
// while UNC and every degenerate/spoof form are provably UNCHANGED. These
// are pure string cases -- the function makes zero OS calls, so they are as
// valid on macOS/Linux as on a Windows runner (verifiable by reading
// hasWindowsDriveLetterPrefix/hasValidUNCPrefix: neither calls anything in
// "path/filepath").
func TestNormalizeGuardPath_WindowsSeparators(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		// --- drive-letter anchored: canonicalized ---
		{"backslash drive path", `C:\Users\x\guard.sh`, "C:/Users/x/guard.sh"},
		{"forward-slash drive path unchanged shape", "C:/Users/x/guard.sh", "C:/Users/x/guard.sh"},
		{"mixed separators, the exact ML-7A trigger", `C:\Users\RUNNER~1\AppData\Local\Temp\TestGBGDedup1234567//.trackfw/scripts/trackfw-git-branch-guard.sh`, "C:/Users/RUNNER~1/AppData/Local/Temp/TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh"},
		{"computed via Join, all backslash", `C:\Users\RUNNER~1\AppData\Local\Temp\TestGBGDedup1234567\.trackfw\scripts\trackfw-git-branch-guard.sh`, "C:/Users/RUNNER~1/AppData/Local/Temp/TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh"},
		{"doc-comment's own $HOME-trailing-slash trigger", `C:\Users\foo\\.trackfw\scripts\trackfw-git-branch-guard.sh`, "C:/Users/foo/.trackfw/scripts/trackfw-git-branch-guard.sh"},
		{"lowercase drive letter", `d:\a\b`, "d:/a/b"},

		// --- valid UNC: byte-for-byte unchanged, including its backslashes ---
		{"valid UNC", `\\servidor\share\guard.sh`, `\\servidor\share\guard.sh`},
		{"valid UNC, different server", `\\other\share\guard.sh`, `\\other\share\guard.sh`},

		// --- POSIX-typed UNC-equivalent: pre-existing "//" collapse, untouched by this ML ---
		{"POSIX-typed UNC equivalent collapses like any // input", "//servidor/share/guard.sh", "/servidor/share/guard.sh"},

		// --- degenerate / invalid UNC: not anchored, no translation, no change ---
		{"bare double backslash", `\\`, `\\`},
		{"double backslash no share segment", `\\x`, `\\x`},
		{`server "." is not a hostname`, `\\.\x`, `\\.\x`},
		{`server ".." is not a hostname`, `\\..\evil`, `\\..\evil`},
		{"doubled backslash mid-string produces empty share", `\\..\\evil`, `\\..\\evil`},

		// --- adversarial corpus from the hades-tf ML-3A/3B barrier: must NOT match the drive-letter arm ---
		{"homoglyph fullwidth C, not ASCII", "ｃ:\\Users\\x\\guard.sh", "ｃ:\\Users\\x\\guard.sh"},
		{"zero-width space before drive letter", "\u200bC:\\Users\\x\\guard.sh", "\u200bC:\\Users\\x\\guard.sh"},
		{"digit before colon, not a drive letter", `1:\Users\x\guard.sh`, `1:\Users\x\guard.sh`},
		{"leading space before drive letter", ` C:\Users\x\guard.sh`, ` C:\Users\x\guard.sh`},
		{"drive-relative, no separator after colon", `C:foo\bar`, `C:foo\bar`},
		{"embedded newline, only backslash bytes change", "C:\\x\ny\\z", "C:/x\ny/z"},

		// --- plain POSIX / relative input with a literal backslash byte: must NOT be touched ---
		{"literal backslash in a POSIX segment name is a filename byte, not a separator", `/home/alice/weird\name/guard.sh`, `/home/alice/weird\name/guard.sh`},
		{"relative path with backslash, no drive letter, untouched", `scripts\guard.sh`, `scripts\guard.sh`},
	}
	for _, c := range cases {
		if got := normalizeGuardPath(c.in); got != c.want {
			t.Errorf("%s: normalizeGuardPath(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestHasValidUNCPrefix_Table pins the predicate ML-7B relies on to leave
// UNC untouched -- named and tested independently so the decision is
// verifiable without re-deriving it from normalizeGuardPath's output.
func TestHasValidUNCPrefix_Table(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`\\servidor\share\guard.sh`, true},
		{`\\servidor\share`, true},
		{`\\`, false},
		{`\\x`, false},
		{`\\.\x`, false},
		{`\\..\evil`, false},
		{`\\..\\evil`, false},
		{`//servidor/share`, false}, // POSIX-typed form -- not this predicate's concern
		{`C:\Users\x`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := hasValidUNCPrefix(c.in); got != c.want {
			t.Errorf("hasValidUNCPrefix(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestSamePathCommand_WindowsSeparatorsDoNotOverMatch is the non-loosening
// control specific to ML-7B: the new equalities introduced by canonicalizing
// "\" must be exactly "same drive-anchored path, different separator style"
// -- nothing else. A UNC path must never dedup-match a differently-anchored
// path that happens to normalize to the same string under the OLD (pre-ML-7B)
// "//" -> "/" collapse rule alone.
func TestSamePathCommand_WindowsSeparatorsDoNotOverMatch(t *testing.T) {
	// Positive: the actual fix -- same script, different separator style.
	if !samePathCommand(`C:\Users\x\guard.sh`, "C:/Users/x/guard.sh") {
		t.Error("same drive-anchored script should match regardless of separator style")
	}
	// Negative: UNC must not collapse into the single-leading-backslash /
	// POSIX-collapsed form that a naive translate-then-collapse would produce.
	cases := []struct{ a, b string }{
		{`\\servidor\share\guard.sh`, "/servidor/share/guard.sh"},
		{`\\servidor\share\guard.sh`, `\servidor\share\guard.sh`},
		{`\\servidor\share\guard.sh`, "//servidor/share/guard.sh"},
		{`\\servidor\share\guard.sh`, `\\otherserver\share\guard.sh`},
		{`C:\Users\alice\guard.sh`, `C:\Users\bob\guard.sh`},
		{`/home/alice/weird\name/guard.sh`, `/home/alice/weird/name/guard.sh`},
	}
	for _, c := range cases {
		if samePathCommand(c.a, c.b) {
			t.Errorf("samePathCommand(%q, %q) should be false -- these are different paths", c.a, c.b)
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
