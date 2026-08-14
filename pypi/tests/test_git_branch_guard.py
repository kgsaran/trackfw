"""
Testes de unidade para o script trackfw-git-branch-guard.sh (gerador), sua injeção nos
hooks por runtime e o novo inject_amazonq_hooks — ML-3C de
ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md.

Mirrors pypi/tests/test_credential_guard.py (script generation) and
pypi/tests/test_credential_guard_dedup.py (per-runtime hook wiring idempotency).
"""

import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.generators.init_gen import (
    _generate_git_branch_guard_script,
    generate_global_git_branch_guard_script,
    _GIT_BRANCH_GUARD_SH,
)
from trackfw.generators.hooks import (
    inject_amazonq_hooks,
    inject_claude_hooks,
    inject_codex_hooks,
    inject_copilot_hooks,
    inject_cursor_hooks,
    inject_gemini_hooks,
    inject_kiro_hooks,
)


def _read_json(path):
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)


class TestGitBranchGuardGenerator(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_gera_script_executavel(self):
        _generate_git_branch_guard_script(self.tmpdir)
        script_path = os.path.join(self.tmpdir, 'scripts', 'trackfw-git-branch-guard.sh')
        self.assertTrue(os.path.exists(script_path))
        mode = os.stat(script_path).st_mode
        self.assertTrue(mode & stat.S_IXUSR, 'script deveria ser executável')
        with open(script_path, 'r', encoding='utf-8') as f:
            content = f.read()
        self.assertTrue(content.startswith('#!/usr/bin/env bash'))
        self.assertNotIn('trackfw.yaml', content)  # unlike credential-guard, no config dependency

    def test_script_nao_comeca_com_linha_em_branco(self):
        # .lstrip('\n') deve remover a quebra de linha inicial do raw string.
        self.assertFalse(_GIT_BRANCH_GUARD_SH.lstrip('\n').startswith('\n'))

    def test_conteudo_identico_entre_escopo_projeto_e_global(self):
        _generate_git_branch_guard_script(self.tmpdir)
        home = tempfile.mkdtemp()
        try:
            generate_global_git_branch_guard_script(home)
            with open(os.path.join(self.tmpdir, 'scripts', 'trackfw-git-branch-guard.sh'), encoding='utf-8') as f:
                project_content = f.read()
            with open(os.path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh'), encoding='utf-8') as f:
                global_content = f.read()
            self.assertEqual(project_content, global_content)
        finally:
            shutil.rmtree(home, ignore_errors=True)

    def test_global_requer_home_nao_vazio(self):
        with self.assertRaises(ValueError):
            generate_global_git_branch_guard_script('')

    def test_nao_cria_nenhum_hooks_json_de_cli(self):
        # Este ML só cria o script -- não o injeta em nenhum hooks.json/settings.json de CLI
        # sozinho (isso é feito por generators/hooks.py:inject_hooks_detected).
        _generate_git_branch_guard_script(self.tmpdir)
        for p in [
            '.claude/settings.json',
            '.codex/hooks.json',
            '.gemini/settings.json',
            '.github/hooks/trackfw-attention.json',
            '.cursor/hooks.json',
            '.kiro/hooks/trackfw-attention.json',
            '.amazonq/cli-agents/q_cli_default.json',
        ]:
            self.assertFalse(
                os.path.exists(os.path.join(self.tmpdir, p)),
                f'_generate_git_branch_guard_script não deveria criar {p}',
            )


class TestGitBranchGuardScriptWindsurfStdin(unittest.TestCase):
    """Invoca o script real como subprocesso com o payload `pre_run_command` real do
    Windsurf (`{"tool_info": {"command_line": "..."}}`) -- confirma que a extração via
    `.tool_info.command_line` (adicionada nesta correção, mirror de Go's
    gitBranchGuardScript) bloqueia corretamente, em vez de reimplementar a extração em
    paralelo (mesmo padrão de test_credential_guard.py)."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        _generate_git_branch_guard_script(self.tmpdir)
        self.script_path = os.path.join(self.tmpdir, 'scripts', 'trackfw-git-branch-guard.sh')

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run(self, payload: dict):
        return subprocess.run(
            ['bash', self.script_path],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
        )

    def test_windsurf_command_line_blocks_commit(self):
        proc = self._run({'agent_action_name': 'run_command', 'tool_info': {'command_line': 'git commit -m "x"'}})
        self.assertEqual(proc.returncode, 2)
        self.assertIn('git commit bruto bloqueado', proc.stderr)

    def test_windsurf_command_line_allows_status(self):
        proc = self._run({'agent_action_name': 'run_command', 'tool_info': {'command_line': 'git status'}})
        self.assertEqual(proc.returncode, 0)


class TestGitBranchGuardHookWiringIdempotent(unittest.TestCase):
    """Cada injetor rodado duas vezes deve produzir exatamente a mesma entrada de
    git-branch-guard (sem duplicar) -- o caso mais importante para este ML."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self._orig_home = os.environ.get('HOME')
        os.environ['HOME'] = tempfile.mkdtemp()

    def tearDown(self):
        if self._orig_home is None:
            os.environ.pop('HOME', None)
        else:
            os.environ['HOME'] = self._orig_home
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_claude(self):
        inject_claude_hooks(self.tmp)
        inject_claude_hooks(self.tmp)
        data = _read_json(os.path.join(self.tmp, '.claude', 'settings.json'))
        bash_entry = next(e for e in data['hooks']['PreToolUse'] if e['matcher'] == 'Bash')
        commands = [h['command'] for h in bash_entry['hooks']]
        self.assertEqual(commands.count('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'), 1)
        # PostToolUse[Bash] must not carry the git-branch-guard entry (Pre-only guard).
        post_bash = next((e for e in data['hooks']['PostToolUse'] if e['matcher'] == 'Bash'), None)
        self.assertIsNotNone(post_bash)
        self.assertNotIn('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh', [h['command'] for h in post_bash['hooks']])

    def test_codex(self):
        inject_codex_hooks(self.tmp)
        inject_codex_hooks(self.tmp)
        data = _read_json(os.path.join(self.tmp, '.codex', 'hooks.json'))
        bash_entries = [e for e in data['hooks']['PreToolUse'] if e['matcher'] == 'Bash']
        self.assertEqual(len(bash_entries), 1)
        commands = [h['command'] for h in bash_entries[0]['hooks']]
        expected = '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"'
        self.assertEqual(commands.count(expected), 1)
        apply_patch_entries = [e for e in data['hooks']['PreToolUse'] if e['matcher'] == 'apply_patch']
        self.assertEqual(len(apply_patch_entries), 1)
        self.assertNotIn(expected, [h['command'] for h in apply_patch_entries[0]['hooks']])

    def test_gemini(self):
        inject_gemini_hooks(self.tmp)
        inject_gemini_hooks(self.tmp)
        data = _read_json(os.path.join(self.tmp, '.gemini', 'settings.json'))
        before_entries = [e for e in data['hooks']['BeforeTool'] if e['matcher'] == 'run_shell_command']
        self.assertEqual(len(before_entries), 1)
        commands = [h['command'] for h in before_entries[0]['hooks']]
        expected = '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'
        self.assertEqual(commands.count(expected), 1)
        after_entries = [e for e in data['hooks']['AfterTool'] if e['matcher'] == 'run_shell_command']
        self.assertEqual(len(after_entries), 1)
        self.assertNotIn(expected, [h['command'] for h in after_entries[0]['hooks']])

    def test_kiro_out_of_scope(self):
        # Kiro is NOT one of the roadmap's "7 runtimes" (claude, codex, gemini, copilot,
        # windsurf, amazonq, cursor) -- confirmed against Go's InjectKiroHooks (no
        # git-branch-guard wiring either, verified via check-agent-hooks-parity.sh
        # go-vs-py during this ML). No entry expected.
        inject_kiro_hooks(self.tmp)
        inject_kiro_hooks(self.tmp)
        data = _read_json(os.path.join(self.tmp, '.kiro', 'hooks', 'trackfw-attention.json'))
        names = {h.get('name') for h in data['hooks']}
        self.assertNotIn('trackfw-git-branch-guard-pre', names)

    def test_copilot(self):
        inject_copilot_hooks(self.tmp)
        inject_copilot_hooks(self.tmp)
        data = _read_json(os.path.join(self.tmp, '.github', 'hooks', 'trackfw-attention.json'))
        pre_entries = [
            e for e in data['hooks']['preToolUse']
            if e.get('bash') == 'scripts/trackfw-git-branch-guard.sh' and e.get('matcher') == 'bash'
        ]
        self.assertEqual(len(pre_entries), 1)
        post_entries = [e for e in data['hooks']['postToolUse'] if e.get('bash') == 'scripts/trackfw-git-branch-guard.sh']
        self.assertEqual(len(post_entries), 0)

    def test_cursor(self):
        inject_cursor_hooks(self.tmp)
        inject_cursor_hooks(self.tmp)
        data = _read_json(os.path.join(self.tmp, '.cursor', 'hooks.json'))
        before = [e for e in data['hooks']['beforeShellExecution'] if e.get('command') == 'scripts/trackfw-git-branch-guard.sh']
        self.assertEqual(len(before), 1)
        after = [e for e in data['hooks'].get('afterShellExecution', []) if e.get('command') == 'scripts/trackfw-git-branch-guard.sh']
        self.assertEqual(len(after), 0)

    def test_amazonq(self):
        inject_amazonq_hooks(self.tmp)
        inject_amazonq_hooks(self.tmp)
        path = os.path.join(self.tmp, '.amazonq', 'cli-agents', 'q_cli_default.json')
        self.assertTrue(os.path.isfile(path))
        data = _read_json(path)
        self.assertEqual(data['name'], 'q_cli_default')
        self.assertIn('git branch guard', data['description'])
        self.assertIsNone(data['prompt'])
        self.assertEqual(data['mcpServers'], {})
        self.assertEqual(data['tools'], ['*'])
        self.assertEqual(data['toolAliases'], {})
        self.assertEqual(data['allowedTools'], [])
        self.assertEqual(data['resources'], [])
        self.assertFalse(data['useLegacyMcpJson'])
        pre_entries = [e for e in data['hooks']['preToolUse'] if e['matcher'] == 'execute_bash']
        self.assertEqual(len(pre_entries), 1)
        commands = [h['command'] for h in pre_entries[0]['hooks']]
        self.assertEqual(commands, ['scripts/trackfw-git-branch-guard.sh'])
        denied = data['toolsSettings']['execute_bash']['deniedCommands']
        self.assertEqual(denied, ['^git (commit|push|checkout -b)'])

        # Old (wrong, ML-3C) path must never be written.
        self.assertFalse(os.path.isfile(os.path.join(self.tmp, '.amazonq', 'settings.json')))

    def test_windsurf(self):
        from trackfw.generators.hooks import inject_windsurf_hooks

        windsurfrules = os.path.join(self.tmp, '.windsurfrules')
        with open(windsurfrules, 'w', encoding='utf-8') as f:
            f.write("# Existing rules\n")

        inject_windsurf_hooks(self.tmp)
        inject_windsurf_hooks(self.tmp)

        path = os.path.join(self.tmp, '.windsurf', 'hooks.json')
        self.assertTrue(os.path.isfile(path))
        data = _read_json(path)
        pre_run = [
            e for e in data['hooks']['pre_run_command']
            if e.get('command') == 'bash scripts/trackfw-git-branch-guard.sh'
        ]
        self.assertEqual(len(pre_run), 1)
        self.assertTrue(pre_run[0]['show_output'])

        # Old (wrong, ML-3C) dedicated-file path must never be written.
        self.assertFalse(
            os.path.isfile(os.path.join(self.tmp, '.windsurf', 'hooks', 'trackfw-git-branch-guard.json'))
        )

    def test_windsurf_migrates_legacy_dedicated_file(self):
        from trackfw.generators.hooks import inject_windsurf_hooks

        legacy_dir = os.path.join(self.tmp, '.windsurf', 'hooks')
        os.makedirs(legacy_dir, exist_ok=True)
        legacy_path = os.path.join(legacy_dir, 'trackfw-git-branch-guard.json')
        with open(legacy_path, 'w', encoding='utf-8') as f:
            json.dump({'version': 1, 'hooks': [{'name': 'trackfw-git-branch-guard'}]}, f)

        inject_windsurf_hooks(self.tmp)

        self.assertFalse(os.path.isfile(legacy_path))
        self.assertFalse(os.path.isdir(legacy_dir))
        data = _read_json(os.path.join(self.tmp, '.windsurf', 'hooks.json'))
        commands = [e['command'] for e in data['hooks']['pre_run_command']]
        self.assertIn('bash scripts/trackfw-git-branch-guard.sh', commands)

    def test_windsurf_legacy_dir_with_unrelated_files_is_kept(self):
        from trackfw.generators.hooks import inject_windsurf_hooks

        legacy_dir = os.path.join(self.tmp, '.windsurf', 'hooks')
        os.makedirs(legacy_dir, exist_ok=True)
        with open(os.path.join(legacy_dir, 'trackfw-git-branch-guard.json'), 'w', encoding='utf-8') as f:
            json.dump({}, f)
        with open(os.path.join(legacy_dir, 'unrelated.json'), 'w', encoding='utf-8') as f:
            json.dump({'user': 'data'}, f)

        inject_windsurf_hooks(self.tmp)

        self.assertTrue(os.path.isdir(legacy_dir))
        self.assertTrue(os.path.isfile(os.path.join(legacy_dir, 'unrelated.json')))

    def test_windsurf_preserves_other_events_and_entries(self):
        from trackfw.generators.hooks import inject_windsurf_hooks

        windsurf_dir = os.path.join(self.tmp, '.windsurf')
        os.makedirs(windsurf_dir, exist_ok=True)
        with open(os.path.join(windsurf_dir, 'hooks.json'), 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'pre_run_command': [{'command': 'echo third-party', 'show_output': False}],
                    'post_run_command': [{'command': 'echo other-event'}],
                },
            }, f)

        inject_windsurf_hooks(self.tmp)

        data = _read_json(os.path.join(self.tmp, '.windsurf', 'hooks.json'))
        pre_commands = [e['command'] for e in data['hooks']['pre_run_command']]
        self.assertIn('echo third-party', pre_commands)
        self.assertIn('bash scripts/trackfw-git-branch-guard.sh', pre_commands)
        self.assertEqual(
            [e['command'] for e in data['hooks']['post_run_command']],
            ['echo other-event'],
        )


class TestAmazonQDetection(unittest.TestCase):
    """inject_hooks_detected must detect .amazonq/ and call inject_amazonq_hooks."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self._orig_home = os.environ.get('HOME')
        os.environ['HOME'] = tempfile.mkdtemp()

    def tearDown(self):
        if self._orig_home is None:
            os.environ.pop('HOME', None)
        else:
            os.environ['HOME'] = self._orig_home
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_detects_amazonq_dir(self):
        from trackfw.generators.hooks import inject_hooks_detected
        os.makedirs(os.path.join(self.tmp, '.amazonq'), exist_ok=True)
        inject_hooks_detected(self.tmp)
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.amazonq', 'cli-agents', 'q_cli_default.json')))

    def test_skips_when_no_amazonq_dir(self):
        from trackfw.generators.hooks import inject_hooks_detected
        inject_hooks_detected(self.tmp)
        self.assertFalse(os.path.isfile(os.path.join(self.tmp, '.amazonq', 'cli-agents', 'q_cli_default.json')))


if __name__ == '__main__':
    unittest.main()
