"""
ROADMAP-2026-08-06 Wave 3/ML-3A — inject_x_hooks (project scope) must skip the
credential-guard entry when the corresponding global-scope wiring (installed via
`trackfw update harness --targets <tool>-credential-guard`,
pypi/trackfw/commands/update_harness.py) is already present, and must fail-open
(fall back to the pre-ML-3A behavior of always adding the project-scope entry) if
the global file is missing, unreadable, or unparseable.

Mirrors internal/generators/credential_guard_dedup_test.go (Go) and
npm/tests/credential_guard_dedup.test.js (Node).
"""

import json
import os
import tempfile
import unittest

from trackfw.generators.hooks import (
    inject_claude_hooks,
    inject_codex_hooks,
    inject_copilot_hooks,
    inject_cursor_hooks,
    inject_gemini_hooks,
    inject_kiro_hooks,
)


def _write_json(path, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, indent=2)
        f.write('\n')


def _read_json(path):
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)


def _has_claude_hook(data, event, matcher, command):
    for entry in data.get('hooks', {}).get(event, []):
        if entry.get('matcher') != matcher:
            continue
        for h in entry.get('hooks', []):
            if h.get('command') == command:
                return True
    return False


class DedupTestCase(unittest.TestCase):
    def setUp(self):
        self._orig_home = os.environ.get('HOME')

    def tearDown(self):
        if self._orig_home is None:
            os.environ.pop('HOME', None)
        else:
            os.environ['HOME'] = self._orig_home

    def _isolated_home(self):
        home = tempfile.mkdtemp()
        os.environ['HOME'] = home
        return home

    def _script_path(self, home):
        return os.path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')


class TestDedupSkipsProjectEntry(DedupTestCase):
    def test_claude(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.claude', 'settings.json'),
            {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': script_path}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertFalse(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))
        self.assertFalse(_has_claude_hook(data, 'PostToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'AskUserQuestion', '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh'))
        self.assertTrue(_has_claude_hook(data, 'PostToolUse', 'AskUserQuestion', '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'))

    def test_codex(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.codex', 'hooks.json'),
            {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': script_path}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_codex_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.codex', 'hooks.json'))
        # ROADMAP-2026-08-11 ML-3A: Codex commands now wrap $(git rev-parse --show-toplevel) in
        # literal quotes (ADR-2026-08-11).
        codex_guard_cmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"'
        codex_signal_cmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-signal.sh"'
        codex_cleanup_cmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-cleanup.sh"'
        self.assertFalse(_has_claude_hook(data, 'PreToolUse', 'Bash', codex_guard_cmd))
        self.assertFalse(_has_claude_hook(data, 'PostToolUse', 'Bash', codex_guard_cmd))
        self.assertTrue(_has_claude_hook(data, 'PermissionRequest', '.*', codex_signal_cmd))
        self.assertTrue(_has_claude_hook(data, 'PostToolUse', '.*', codex_cleanup_cmd))

    def test_gemini(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.gemini', 'settings.json'),
            {'hooks': {'BeforeTool': [{'matcher': 'run_shell_command', 'hooks': [{'type': 'command', 'command': script_path}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_gemini_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.gemini', 'settings.json'))
        # ROADMAP-2026-08-11 ML-4A: Gemini commands now use $GEMINI_PROJECT_DIR (ADR-2026-08-11).
        gemini_guard_cmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'
        gemini_signal_cmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-signal.sh'
        gemini_cleanup_cmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'
        self.assertFalse(_has_claude_hook(data, 'BeforeTool', 'run_shell_command', gemini_guard_cmd))
        self.assertFalse(_has_claude_hook(data, 'AfterTool', 'run_shell_command', gemini_guard_cmd))
        self.assertTrue(_has_claude_hook(data, 'Notification', 'ToolPermission', gemini_signal_cmd))
        self.assertTrue(_has_claude_hook(data, 'AfterTool', '*', gemini_cleanup_cmd))

    def test_cursor(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.cursor', 'hooks.json'),
            {'version': 1, 'hooks': {'beforeShellExecution': [{'command': script_path}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_cursor_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.cursor', 'hooks.json'))
        # ROADMAP-2026-08-17 Wave 2/ML-2B: git-branch-guard now has its own global dedup
        # (_global_git_branch_guard_installed_cursor), but this fixture only wired the
        # GLOBAL credential-guard entry, not git-branch-guard's -- so the git-branch-guard
        # dedup check finds no matching command and fails open, keeping its project-scope
        # entry. See test_git_branch_guard_dedup.py for the case where the git-branch-guard
        # global IS installed.
        self.assertEqual(len(data['hooks'].get('beforeShellExecution', [])), 1)
        self.assertEqual(data['hooks']['beforeShellExecution'][0]['command'], 'scripts/trackfw-git-branch-guard.sh')
        self.assertEqual(len(data['hooks'].get('afterShellExecution', [])), 0)
        self.assertEqual(len(data['hooks']['preToolUse']), 1)
        self.assertEqual(data['hooks']['preToolUse'][0]['command'], 'scripts/trackfw-attention-signal.sh')
        self.assertEqual(len(data['hooks']['postToolUse']), 1)
        self.assertEqual(data['hooks']['postToolUse'][0]['command'], 'scripts/trackfw-attention-cleanup.sh')

    def test_copilot(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.copilot', 'settings.json'),
            {'hooks': {'preToolUse': [{'type': 'command', 'matcher': 'bash', 'bash': script_path, 'cwd': '.', 'timeoutSec': 10}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_copilot_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.github', 'hooks', 'trackfw-attention.json'))
        # ROADMAP-2026-08-17 Wave 2/ML-2B: git-branch-guard now has its own global dedup
        # (_global_git_branch_guard_installed_copilot), but this fixture only wired the
        # GLOBAL credential-guard entry, not git-branch-guard's -- so the git-branch-guard
        # dedup check finds no matching command and fails open, keeping its project-scope
        # entry alongside the always-on attention-signal entry.
        self.assertEqual(len(data['hooks']['preToolUse']), 2)
        self.assertEqual(data['hooks']['preToolUse'][0]['bash'], 'scripts/trackfw-attention-signal.sh')
        self.assertEqual(data['hooks']['preToolUse'][1]['bash'], 'scripts/trackfw-git-branch-guard.sh')
        self.assertEqual(len(data['hooks']['postToolUse']), 1)
        self.assertEqual(data['hooks']['postToolUse'][0]['bash'], 'scripts/trackfw-attention-cleanup.sh')

    def test_kiro(self):
        home = self._isolated_home()
        global_kiro_path = os.path.join(home, '.kiro', 'hooks', 'trackfw-credential-guard.json')
        _write_json(global_kiro_path, {'version': 'v1', 'hooks': []})

        project_dir = tempfile.mkdtemp()
        inject_kiro_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.kiro', 'hooks', 'trackfw-attention.json'))
        # ML-3C (ROADMAP-2026-08-14): Kiro is not one of the roadmap's "7 runtimes" -- no
        # git-branch-guard wiring for Kiro, matching Go's InjectKiroHooks.
        self.assertEqual(len(data['hooks']), 2)
        names = {h.get('name') for h in data['hooks']}
        self.assertNotIn('trackfw-credential-guard-pre', names)
        self.assertNotIn('trackfw-credential-guard-post', names)
        self.assertNotIn('trackfw-git-branch-guard-pre', names)


class TestDedupFailOpen(DedupTestCase):
    def test_no_global_file(self):
        self._isolated_home()  # empty $HOME, no global files at all

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))

    def test_corrupted_global_file(self):
        home = self._isolated_home()
        settings_path = os.path.join(home, '.claude', 'settings.json')
        os.makedirs(os.path.dirname(settings_path), exist_ok=True)
        with open(settings_path, 'w', encoding='utf-8') as f:
            f.write('{not valid json')

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))


if __name__ == '__main__':
    unittest.main()
