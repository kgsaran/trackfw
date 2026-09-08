"""
test_update_json_path_forward_slash.py — corrective falsifier for issue #292:
`trackfw update --json` on Windows emitted native `\\` in the "path" field for
the agent-rules, validate-script and claude-commands targets — only in the
Python runtime; Go and Node always emit `/` (REQ-2026-08-30, REABERTA
2026-09-08, ML-R1).

Root cause: AGENT_RULES_RELATIVE_PATHS, VALIDATE_SCRIPT_RELATIVE_PATH and
CLAUDE_COMMANDS_RELATIVE_PATH (pypi/trackfw/commands/update.py) were built
with os.path.join(...), which resolves via os.sep — "\\" on Windows. These
three constants are DUAL-purpose: they also feed _run_project's display_path
argument directly, i.e. they ARE the "path" field of the --json contract for
their targets (unlike AGENT_HOOKS_RELATIVE_PATHS / CI_WORKFLOW_RELATIVE_PATHS,
whose display strings are separate hardcoded literals and were never
affected — left unchanged, see update.py's comments at their definitions).

Non-vacuity (the ML's critical point): a test that merely asserts "these
strings contain no backslash" on the CURRENT host is vacuously satisfied on
Linux/macOS — os.path.join(".github", "x") already returns ".github/x" there
(posixpath.join uses "/" regardless of the *fix* being present), so a
revert (bringing back os.path.join in the three declarations) would NOT
reproduce the failure locally or in the Linux CI job.

To make the assertion fail on revert REGARDLESS of host OS, this test
reloads pypi/trackfw/commands/update.py with the stdlib `os.path` module
temporarily REPLACED by `ntpath` (Windows path semantics) before import —
so any `os.path.join(...)` call the module performs at import time is
forced through ntpath.join, exactly as it would run on a real Windows host,
independent of what platform the test suite is actually executing on. If
the three constants still use os.path.join, this reload reproduces the
'\\' leak deterministically on Linux/macOS/CI too; with the literal "/"
strings from this ML, the reload changes nothing, because no os.path.join
call is made for them at all.
"""

import importlib
import ntpath
import sys
import unittest
from unittest import mock


def _reload_update_under_ntpath():
    """Reloads trackfw.commands.update with os.path forced to ntpath for
    the duration of the import, then restores the real os.path and the
    original module object in sys.modules so this test never leaks a
    Windows-flavored module into the rest of the suite."""
    import trackfw.commands.update as update_module

    original_update_module = sys.modules.get("trackfw.commands.update")
    with mock.patch("os.path", ntpath):
        reloaded = importlib.reload(update_module)
        # Snapshot the values while os.path is still patched — the
        # constants are plain strings, so the snapshot survives the
        # patch being undone below.
        agent_rules = list(reloaded.AGENT_RULES_RELATIVE_PATHS)
        validate_script = reloaded.VALIDATE_SCRIPT_RELATIVE_PATH
        claude_commands = reloaded.CLAUDE_COMMANDS_RELATIVE_PATH

    # Restore the module to its real (posixpath-joined-or-literal, per host)
    # form so every other test in the suite sees the normal module.
    if original_update_module is not None:
        importlib.reload(original_update_module)

    return agent_rules, validate_script, claude_commands


class TestUpdateJsonPathIsPlatformIndependentForwardSlash(unittest.TestCase):
    """Asserts the ML-R1 conclusion: AGENT_RULES_RELATIVE_PATHS,
    VALIDATE_SCRIPT_RELATIVE_PATH and CLAUDE_COMMANDS_RELATIVE_PATH use a
    literal "/" — never os.path.join/os.sep — by reproducing Windows path
    semantics (ntpath) at import time regardless of the host OS running
    this test."""

    def test_agent_rules_relative_paths_have_no_backslash_even_under_ntpath(self):
        agent_rules, _validate_script, _claude_commands = _reload_update_under_ntpath()
        for rel in agent_rules:
            self.assertNotIn(
                "\\",
                rel,
                f"{rel!r} would leak a native Windows separator into the --json "
                "'path' field if these constants used os.path.join (issue #292) "
                "— reproduced here via ntpath regardless of host OS",
            )
        self.assertEqual(
            agent_rules,
            [
                "CLAUDE.md",
                "AGENTS.md",
                "GEMINI.md",
                ".github/copilot-instructions.md",
                ".windsurfrules",
                ".amazonq/developer/guidelines.md",
                ".cursor/rules/trackfw.mdc",
            ],
        )

    def test_validate_script_relative_path_has_no_backslash_even_under_ntpath(self):
        _agent_rules, validate_script, _claude_commands = _reload_update_under_ntpath()
        self.assertNotIn("\\", validate_script)
        self.assertEqual(validate_script, "scripts/trackfw-validate.sh")

    def test_claude_commands_relative_path_has_no_backslash_even_under_ntpath(self):
        _agent_rules, _validate_script, claude_commands = _reload_update_under_ntpath()
        self.assertNotIn("\\", claude_commands)
        self.assertEqual(claude_commands, ".claude/commands/trackfw")


if __name__ == "__main__":
    unittest.main()
