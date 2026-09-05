"""
ROADMAP-2026-08-17 Wave 2/ML-2C -- _hook_array_has_command/_simple_array_has_value
must compare hook "command" paths after normalizing incidental formatting
(double slashes, trailing slash), not as raw strings. Root cause: macOS's
$TMPDIR ends in "/", so a $HOME built under it can contain "//" once
concatenated with a literal path segment; the writer computes script_path via
os.path.join (which normalizes), while a value written by hand or captured
before normalization does not -- a byte-for-byte compare then silently fails
to dedup.

Mirrors internal/generators/guard_path_normalize_test.go (Go) and
npm/tests/guard_path_normalize.test.js (Node).
"""

import unittest

from trackfw.generators.hooks import (
    _has_valid_unc_prefix,
    _normalize_guard_path,
    _same_path_command,
)


class TestNormalizeGuardPath(unittest.TestCase):
    def test_table(self):
        cases = [
            ('', ''),
            ('/a/b', '/a/b'),
            ('//a/b', '/a/b'),
            ('/a//b', '/a/b'),
            ('/a/b/', '/a/b'),
            ('/a/b//', '/a/b'),
            ('//', '/'),
            ('/', '/'),
            ('/a/./b', '/a/./b'),  # deliberately NOT resolved -- see doc comment
            ('/a/b/../b', '/a/b/../b'),  # deliberately NOT resolved
        ]
        for input_path, want in cases:
            with self.subTest(input_path=input_path):
                self.assertEqual(_normalize_guard_path(input_path), want)


class TestSamePathCommand(unittest.TestCase):
    def test_tolerates_double_slash_and_trailing_slash(self):
        a = '/var/folders/xx/yy/T//trackfw-falsify.ABC/s67-fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh'
        b = '/var/folders/xx/yy/T/trackfw-falsify.ABC/s67-fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh'
        self.assertTrue(_same_path_command(a, b))

    def test_does_not_match_different_paths(self):
        """Non-regression half of risk #2 ('normalizar demais é perigoso'):
        two genuinely different scripts, or the same guard installed for two
        different users, must never compare equal."""
        cases = [
            ('/home/alice/.trackfw/scripts/trackfw-git-branch-guard.sh', '/home/bob/.trackfw/scripts/trackfw-git-branch-guard.sh'),
            ('/a/b/trackfw-git-branch-guard.sh', '/a/bb/trackfw-git-branch-guard.sh'),
            ('/a/b/trackfw-git-branch-guard.sh', '/a/b/trackfw-credential-guard.sh'),
            ('/a/b', '/a/b/c'),
        ]
        for a, b in cases:
            with self.subTest(a=a, b=b):
                self.assertFalse(_same_path_command(a, b))


# ROADMAP-2026-09-03 ML-7B -- drive-letter anchored input gets "\"
# canonicalized to "/" (closing the seam ML-7A measured: ntpath.join always
# emits "\" on Windows, so a hand- or concat-built command with "/" never
# compared equal to the computed one), while UNC and every degenerate/spoof
# form are provably UNCHANGED. Pure string cases -- zero OS calls in the
# function, so valid on any host. Mirrors internal/generators/
# guard_path_normalize_test.go (TestNormalizeGuardPath_WindowsSeparators) and
# npm/tests/guard_path_normalize.test.js.
class TestNormalizeGuardPathWindowsSeparators(unittest.TestCase):
    def test_table(self):
        cases = [
            # --- drive-letter anchored: canonicalized ---
            ('backslash drive path', 'C:\\Users\\x\\guard.sh', 'C:/Users/x/guard.sh'),
            ('forward-slash drive path unchanged shape', 'C:/Users/x/guard.sh', 'C:/Users/x/guard.sh'),
            ('mixed separators, the exact ML-7A trigger',
             'C:\\Users\\RUNNER~1\\AppData\\Local\\Temp\\TestGBGDedup1234567//.trackfw/scripts/trackfw-git-branch-guard.sh',
             'C:/Users/RUNNER~1/AppData/Local/Temp/TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh'),
            ('computed via join, all backslash',
             'C:\\Users\\RUNNER~1\\AppData\\Local\\Temp\\TestGBGDedup1234567\\.trackfw\\scripts\\trackfw-git-branch-guard.sh',
             'C:/Users/RUNNER~1/AppData/Local/Temp/TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh'),
            ("doc-comment's own $HOME-trailing-slash trigger",
             'C:\\Users\\foo\\\\.trackfw\\scripts\\trackfw-git-branch-guard.sh',
             'C:/Users/foo/.trackfw/scripts/trackfw-git-branch-guard.sh'),
            ('lowercase drive letter', 'd:\\a\\b', 'd:/a/b'),

            # --- valid UNC: byte-for-byte unchanged, including its backslashes ---
            ('valid UNC', '\\\\servidor\\share\\guard.sh', '\\\\servidor\\share\\guard.sh'),
            ('valid UNC, different server', '\\\\other\\share\\guard.sh', '\\\\other\\share\\guard.sh'),

            # --- POSIX-typed UNC-equivalent: pre-existing "//" collapse, untouched by this ML ---
            ('POSIX-typed UNC equivalent collapses like any // input', '//servidor/share/guard.sh', '/servidor/share/guard.sh'),

            # --- degenerate / invalid UNC: not anchored, no translation, no change ---
            ('bare double backslash', '\\\\', '\\\\'),
            ('double backslash no share segment', '\\\\x', '\\\\x'),
            ('server "." is not a hostname', '\\\\.\\x', '\\\\.\\x'),
            ('server ".." is not a hostname', '\\\\..\\evil', '\\\\..\\evil'),
            ('doubled backslash mid-string produces empty share', '\\\\..\\\\evil', '\\\\..\\\\evil'),

            # --- adversarial corpus from the hades-tf ML-3A/3B barrier ---
            ('homoglyph fullwidth C, not ASCII', '\uff43:\\Users\\x\\guard.sh', '\uff43:\\Users\\x\\guard.sh'),
            ('zero-width space before drive letter', '\u200bC:\\Users\\x\\guard.sh', '\u200bC:\\Users\\x\\guard.sh'),
            ('digit before colon, not a drive letter', '1:\\Users\\x\\guard.sh', '1:\\Users\\x\\guard.sh'),
            ('leading space before drive letter', ' C:\\Users\\x\\guard.sh', ' C:\\Users\\x\\guard.sh'),
            ('drive-relative, no separator after colon', 'C:foo\\bar', 'C:foo\\bar'),
            ('embedded newline, only backslash bytes change', 'C:\\x\ny\\z', 'C:/x\ny/z'),

            # --- plain POSIX / relative input with a literal backslash byte: must NOT be touched ---
            ('literal backslash in a POSIX segment name is a filename byte, not a separator',
             '/home/alice/weird\\name/guard.sh', '/home/alice/weird\\name/guard.sh'),
            ('relative path with backslash, no drive letter, untouched', 'scripts\\guard.sh', 'scripts\\guard.sh'),
        ]
        for name, input_path, want in cases:
            with self.subTest(name=name, input_path=input_path):
                self.assertEqual(_normalize_guard_path(input_path), want)


class TestHasValidUNCPrefix(unittest.TestCase):
    def test_table(self):
        cases = [
            ('\\\\servidor\\share\\guard.sh', True),
            ('\\\\servidor\\share', True),
            ('\\\\', False),
            ('\\\\x', False),
            ('\\\\.\\x', False),
            ('\\\\..\\evil', False),
            ('\\\\..\\\\evil', False),
            ('//servidor/share', False),
            ('C:\\Users\\x', False),
            ('', False),
        ]
        for input_path, want in cases:
            with self.subTest(input_path=input_path):
                self.assertEqual(_has_valid_unc_prefix(input_path), want)


class TestSamePathCommandWindowsSeparatorsDoNotOverMatch(unittest.TestCase):
    """Non-loosening control specific to ML-7B: the new equalities introduced
    by canonicalizing "\\" must be exactly "same drive-anchored path,
    different separator style" -- nothing else."""

    def test_positive_match(self):
        self.assertTrue(_same_path_command('C:\\Users\\x\\guard.sh', 'C:/Users/x/guard.sh'))

    def test_does_not_over_match(self):
        cases = [
            ('\\\\servidor\\share\\guard.sh', '/servidor/share/guard.sh'),
            ('\\\\servidor\\share\\guard.sh', '\\servidor\\share\\guard.sh'),
            ('\\\\servidor\\share\\guard.sh', '//servidor/share/guard.sh'),
            ('\\\\servidor\\share\\guard.sh', '\\\\otherserver\\share\\guard.sh'),
            ('C:\\Users\\alice\\guard.sh', 'C:\\Users\\bob\\guard.sh'),
            ('/home/alice/weird\\name/guard.sh', '/home/alice/weird/name/guard.sh'),
        ]
        for a, b in cases:
            with self.subTest(a=a, b=b):
                self.assertFalse(_same_path_command(a, b))


if __name__ == '__main__':
    unittest.main()
