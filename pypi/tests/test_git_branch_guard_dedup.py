"""
ROADMAP-2026-08-17 Wave 2/ML-2B — inject_x_hooks (project scope) must skip the
git-branch-guard entry when the corresponding global-scope wiring (installed via
`trackfw update harness --targets <tool>-git-branch-guard`, ML-2A) is already
present, so the guard doesn't fire (and print its block message) twice per Bash
call — and must fail-open (fall back to always adding the project-scope entry)
if the global file is missing, unreadable, or unparseable.

Mirrors internal/generators/git_branch_guard_dedup_test.go (Go) and
npm/tests/git_branch_guard_dedup.test.js (Node).
"""

import json
import os
import subprocess
import tempfile
import unittest

# ML-0C: bash por caminho absoluto PROVADO (`GNU bash` no --version). Nome nu resolve para
# System32\bash.exe (stub do WSL) no Windows -- ver pypi/tests/bash_path.py.
from .bash_path import bash_cmd

from trackfw.generators.hooks import (
    inject_claude_hooks,
    inject_codex_hooks,
    inject_copilot_hooks,
    inject_cursor_hooks,
    inject_gemini_hooks,
)
from trackfw.generators.init_gen import (
    _generate_git_branch_guard_script,
    generate_global_git_branch_guard_script,
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


def _claude_hook_commands(data, event, matcher):
    out = []
    for entry in data.get('hooks', {}).get(event, []):
        if entry.get('matcher') != matcher:
            continue
        for h in entry.get('hooks', []):
            cmd = h.get('command')
            if isinstance(cmd, str):
                out.append(cmd)
    return out


def _claude_hook_commands_with_type(data, event, matcher):
    """ROADMAP-2026-08-17 ML-4B counterpart of _claude_hook_commands: only
    returns commands from entries that ALSO carry "type":"command" -- i.e.
    models what a real Claude Code runtime would actually execute, not
    merely what textually references a script."""
    out = []
    for entry in data.get('hooks', {}).get(event, []):
        if entry.get('matcher') != matcher:
            continue
        for h in entry.get('hooks', []):
            if h.get('type') != 'command':
                continue
            cmd = h.get('command')
            if isinstance(cmd, str):
                out.append(cmd)
    return out


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
        return os.path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')


class TestGBGDedupSkipsProjectEntry(DedupTestCase):
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
        self.assertFalse(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
        # credential-guard's own global was NOT installed by this fixture -- its entry
        # must still be added, proving the two guards dedup independently.
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))

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
        codex_git_guard_cmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"'
        codex_guard_cmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"'
        self.assertFalse(_has_claude_hook(data, 'PreToolUse', 'Bash', codex_git_guard_cmd))
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'Bash', codex_guard_cmd))

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
        gemini_git_guard_cmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'
        gemini_guard_cmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'
        self.assertFalse(_has_claude_hook(data, 'BeforeTool', 'run_shell_command', gemini_git_guard_cmd))
        self.assertTrue(_has_claude_hook(data, 'BeforeTool', 'run_shell_command', gemini_guard_cmd))

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
        before = data['hooks'].get('beforeShellExecution', [])
        self.assertEqual(len(before), 1)
        self.assertEqual(before[0]['command'], 'scripts/trackfw-credential-guard.sh')

    def test_cursor_both_globally_installed_key_absent_not_empty(self):
        home = self._isolated_home()
        cred_path = os.path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
        gbg_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.cursor', 'hooks.json'),
            {'version': 1, 'hooks': {'beforeShellExecution': [{'command': cred_path}, {'command': gbg_path}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_cursor_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.cursor', 'hooks.json'))
        # Both dedups skip: the key must be ABSENT, not a present-but-empty array --
        # matches Go's InjectCursorHooks, which check-agent-hooks-parity.sh's structural
        # comparator treats as significant.
        self.assertNotIn('beforeShellExecution', data['hooks'])

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
        pre = data['hooks']['preToolUse']
        self.assertFalse(any(e.get('bash') == 'scripts/trackfw-git-branch-guard.sh' for e in pre))
        self.assertTrue(any(e.get('bash') == 'scripts/trackfw-credential-guard.sh' for e in pre))


    def test_claude_tolerates_double_slash_in_stored_command(self):
        """ROADMAP-2026-08-17 ML-2C: reproduces the root cause directly at the
        dedup level -- the "command" value stored in ~/.claude/settings.json
        is built with raw string concatenation (as a hand-edited config, or a
        $HOME captured with a trailing slash before normalization, would
        produce) instead of os.path.join, so it textually differs from what
        _global_git_branch_guard_script_path() computes today even though it
        names the SAME file."""
        home = self._isolated_home()
        raw_stored_command = home + '//' + '.trackfw/scripts/trackfw-git-branch-guard.sh'
        _write_json(
            os.path.join(home, '.claude', 'settings.json'),
            {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': raw_stored_command}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertFalse(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))


class TestGBGDedupReWiresOnMalformedGlobalEntry(DedupTestCase):
    """ROADMAP-2026-08-17 ML-4B -- hades-tf ML-4A barrier finding: a global
    entry with the CORRECT command but MISSING "type":"command" (hand-edited
    config, older trackfw version, another tool's merge) is silently never
    executed by Claude Code/GitHub Copilot CLI. Before this ML,
    _hook_array_has_command/_simple_array_has_value still read such an entry
    as "installed", so the project-scope entry was skipped in favor of a
    global entry that never fires -- "nenhum dos dois escopos protege".
    Cursor is the exception: its schema never carries a "type" field, so a
    missing "type" there is normal, not malformed -- see
    test_claude_tolerates_double_slash_in_stored_command above, whose
    fixture already has no "type" field for Cursor-shaped entries and must
    keep skipping. Mirrors
    TestGBGDedup_Claude_ReWiresProjectEntryWhenGlobalEntryMissingType /
    TestGBGDedup_Copilot_ReWiresProjectEntryWhenGlobalEntryMissingType (Go).
    """

    def test_claude(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.claude', 'settings.json'),
            # Deliberately missing "type":"command" -- the ML-4A barrier
            # finding's exact malformed shape.
            {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'command': script_path}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertTrue(
            _has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'),
            'the malformed global entry (missing "type") is never executed by Claude Code, so the project-scope entry must be re-wired',
        )

    def test_copilot(self):
        home = self._isolated_home()
        script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.copilot', 'settings.json'),
            # Deliberately missing "type":"command" -- same ML-4A finding,
            # Copilot's own schema.
            {'hooks': {'preToolUse': [{'matcher': 'bash', 'bash': script_path, 'cwd': '.', 'timeoutSec': 10}]}},
        )

        project_dir = tempfile.mkdtemp()
        inject_copilot_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.github', 'hooks', 'trackfw-attention.json'))
        pre = data['hooks']['preToolUse']
        self.assertTrue(any(e.get('bash') == 'scripts/trackfw-git-branch-guard.sh' for e in pre))


class TestGBGDedupFailOpen(DedupTestCase):
    def test_no_global_file(self):
        self._isolated_home()  # empty $HOME, no global files at all

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))

    def test_corrupted_global_file(self):
        home = self._isolated_home()
        settings_path = os.path.join(home, '.claude', 'settings.json')
        os.makedirs(os.path.dirname(settings_path), exist_ok=True)
        with open(settings_path, 'w', encoding='utf-8') as f:
            f.write('{not valid json')

        project_dir = tempfile.mkdtemp()
        inject_claude_hooks(project_dir)

        data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        self.assertTrue(_has_claude_hook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))


# ---------------------------------------------------------------------------
# "Message once" -- proved by EXECUTING the generated hook entries (not by
# counting JSON entries), per the architect's explicit AC3 wording ("prove
# executando, não por contagem de entradas no JSON"). Mirrors
# TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled in Go.
# ---------------------------------------------------------------------------

def _run_entries(project_dir, script_paths):
    blocked = 0
    for script in script_paths:
        proc = subprocess.run(
            bash_cmd(script, 'git', 'push'),
            cwd=project_dir,
            capture_output=True,
            text=True,
        )
        if proc.returncode == 0:
            continue  # allow, not counted
        if proc.returncode == 2 and 'git push bruto bloqueado' in proc.stderr:
            blocked += 1
        else:
            raise AssertionError(f'unexpected script outcome for {script}: rc={proc.returncode} stderr={proc.stderr}')
    return blocked


class TestGBGDedupMessageOnce(DedupTestCase):
    def test_message_appears_once_when_both_scopes_installed(self):
        home = self._isolated_home()
        generate_global_git_branch_guard_script(home)
        global_script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.claude', 'settings.json'),
            {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': global_script_path}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        with open(os.path.join(project_dir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('roadmap_dir: docs/roadmaps\n')
        _generate_git_branch_guard_script(project_dir)

        inject_claude_hooks(project_dir)

        project_data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        global_data = _read_json(os.path.join(home, '.claude', 'settings.json'))
        script_paths = (
            _claude_hook_commands(project_data, 'PreToolUse', 'Bash')
            + _claude_hook_commands(global_data, 'PreToolUse', 'Bash')
        )
        git_guard_entries = [p for p in script_paths if 'trackfw-git-branch-guard.sh' in p]
        self.assertEqual(len(git_guard_entries), 1, f'expected exactly 1 git-branch-guard entry across project+global, got {git_guard_entries}')

        resolved = [p.replace('$CLAUDE_PROJECT_DIR', project_dir) for p in git_guard_entries]
        blocked = _run_entries(project_dir, resolved)
        self.assertEqual(blocked, 1)

    def test_malformed_global_entry_does_not_defeat_protection(self):
        """ROADMAP-2026-08-17 ML-4B -- proves the fix end-to-end via
        EXECUTION, not just JSON presence. _claude_hook_commands_with_type
        filters on type == 'command' (unlike _claude_hook_commands above),
        modeling what a real Claude Code runtime would actually fire -- the
        malformed global entry is present in the combined hook set but must
        be excluded from what executes. Before this ML: the malformed entry
        made the dedup skip the project entry AND the malformed entry itself
        never fires -- 0 blocks. After: the project entry is re-wired and,
        being structurally valid, executes -- 1 block. Mirrors
        TestGBGDedup_MalformedGlobalEntry_ProjectStillProtects (Go)."""
        home = self._isolated_home()
        generate_global_git_branch_guard_script(home)
        global_script_path = self._script_path(home)
        _write_json(
            os.path.join(home, '.claude', 'settings.json'),
            {'hooks': {'PreToolUse': [{'matcher': 'Bash', 'hooks': [{'command': global_script_path}]}]}},
        )

        project_dir = tempfile.mkdtemp()
        with open(os.path.join(project_dir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('roadmap_dir: docs/roadmaps\n')
        _generate_git_branch_guard_script(project_dir)

        inject_claude_hooks(project_dir)

        project_data = _read_json(os.path.join(project_dir, '.claude', 'settings.json'))
        global_data = _read_json(os.path.join(home, '.claude', 'settings.json'))
        executable = (
            _claude_hook_commands_with_type(project_data, 'PreToolUse', 'Bash')
            + _claude_hook_commands_with_type(global_data, 'PreToolUse', 'Bash')
        )
        resolved = [
            p.replace('$CLAUDE_PROJECT_DIR', project_dir)
            for p in executable if 'trackfw-git-branch-guard.sh' in p
        ]

        blocked = _run_entries(project_dir, resolved)
        self.assertEqual(
            blocked, 1,
            f'expected exactly 1 block (the re-wired, structurally-valid project entry); executable entries: {resolved}',
        )

    def test_non_vacuous_both_entries_wired_message_twice(self):
        home = self._isolated_home()
        generate_global_git_branch_guard_script(home)
        global_script_path = self._script_path(home)

        project_dir = tempfile.mkdtemp()
        with open(os.path.join(project_dir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('roadmap_dir: docs/roadmaps\n')
        _generate_git_branch_guard_script(project_dir)
        project_script_path = os.path.join(project_dir, 'scripts', 'trackfw-git-branch-guard.sh')

        blocked = _run_entries(project_dir, [project_script_path, global_script_path])
        self.assertEqual(blocked, 2, 'non-vacuity check failed: the test harness itself is broken')


if __name__ == '__main__':
    unittest.main()
