import os
import tempfile
import unittest

from trackfw.generators.init_gen import inject_rules_for_tool


class TestAgentRules(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def _read(self, rel_path: str) -> str:
        with open(os.path.join(self.tmp, rel_path), encoding="utf-8") as f:
            return f.read()

    def _assert_idempotent(self, tool: str, rel_path: str, expected_snippets: list[str]) -> None:
        inject_rules_for_tool(tool, self.tmp)
        before = self._read(rel_path)
        for snippet in expected_snippets:
            self.assertIn(snippet, before)

        inject_rules_for_tool(tool, self.tmp)
        after = self._read(rel_path)
        self.assertEqual(before, after)

    def test_cursor_rules(self):
        self._assert_idempotent(
            "cursor",
            os.path.join(".cursor", "rules", "trackfw.mdc"),
            ["---", "alwaysApply: true", "trackfw governance"],
        )

    def test_windsurf_rules(self):
        self._assert_idempotent(
            "windsurf",
            ".windsurfrules",
            ["# Windsurf Rules", "AI-native delivery governance"],
        )

    def test_amazonq_rules(self):
        self._assert_idempotent(
            "amazonq",
            os.path.join(".amazonq", "developer", "guidelines.md"),
            ["# Amazon Q Developer Guidelines", "AI-native delivery governance"],
        )


if __name__ == "__main__":
    unittest.main()
