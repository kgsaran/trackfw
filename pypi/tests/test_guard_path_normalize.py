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

from trackfw.generators.hooks import _normalize_guard_path, _same_path_command


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


if __name__ == '__main__':
    unittest.main()
