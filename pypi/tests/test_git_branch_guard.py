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
        # ML-1A (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
        # integridade-independente-de-fiacao.md): ao contrário do estado anterior a este ML, o
        # script agora DEPENDE de trackfw.yaml -- vira no-op fora de projeto trackfw. Ver
        # TestGitBranchGuardNoOpOutsideProject abaixo para o comportamento em runtime.
        self.assertIn('trackfw.yaml', content)

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
        # ML-1A: o guard só bloqueia DENTRO de projeto trackfw -- estes testes exercitam
        # comportamento de bloqueio, precisam de trackfw.yaml na raiz do cwd (que é
        # explicitamente self.tmpdir via cwd= abaixo, nunca ambient).
        with open(os.path.join(self.tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('project_name: fixture\n')

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run(self, payload: dict):
        return subprocess.run(
            ['bash', self.script_path],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
            cwd=self.tmpdir,
        )

    def test_windsurf_command_line_blocks_commit(self):
        proc = self._run({'agent_action_name': 'run_command', 'tool_info': {'command_line': 'git commit -m "x"'}})
        self.assertEqual(proc.returncode, 2)
        self.assertIn('git commit bruto bloqueado', proc.stderr)

    def test_windsurf_command_line_allows_status(self):
        proc = self._run({'agent_action_name': 'run_command', 'tool_info': {'command_line': 'git status'}})
        self.assertEqual(proc.returncode, 0)


class TestGitBranchGuardManualE2ERegressions(unittest.TestCase):
    """Regressão dos 3 bugs reais achados por teste manual end-to-end (ML-4A): o parser
    original detectava o alvo (`git commit`/`git push`/`git checkout -b`) por busca de
    substring/regex em QUALQUER lugar da string do comando, em vez de analisar o comando
    por segmentos reais com "git" como primeiro token. Ver doc comment de match_subcommand
    em _GIT_BRANCH_GUARD_SH (init_gen.py) para a causa raiz unificada e o fix."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        _generate_git_branch_guard_script(self.tmpdir)
        self.script_path = os.path.join(self.tmpdir, 'scripts', 'trackfw-git-branch-guard.sh')
        # ML-1A: o guard só bloqueia DENTRO de projeto trackfw -- estes testes exercitam
        # comportamento de bloqueio, precisam de trackfw.yaml na raiz do cwd (que é
        # explicitamente self.tmpdir via cwd= abaixo, nunca ambient).
        with open(os.path.join(self.tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('project_name: fixture\n')

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run(self, command: str):
        return subprocess.run(
            ['bash', self.script_path],
            input=json.dumps({'tool_input': {'command': command}}),
            capture_output=True,
            text=True,
            cwd=self.tmpdir,
        )

    def test_bug1_chained_command_blocks_second_segment_push(self):
        # Falso negativo original: o parser só coletava tokens depois da PRIMEIRA
        # ocorrência de "git" na string inteira, então o `push` do segundo comando
        # (depois do `;`) nunca era analisado.
        proc = self._run('git status; git push origin HEAD')
        self.assertEqual(proc.returncode, 2)
        self.assertIn('git push bruto bloqueado', proc.stderr)

    def test_bug2_absolute_path_git_blocks_commit(self):
        # Falso negativo original: comparação `[ "$tok" = "git" ]` era igualdade exata de
        # string, então `/usr/bin/git` nunca ativava a detecção.
        proc = self._run('/usr/bin/git commit -m x')
        self.assertEqual(proc.returncode, 2)
        self.assertIn('git commit bruto bloqueado', proc.stderr)

    def test_bug3_prose_mentioning_git_commit_does_not_block_legit_trackfw_commit(self):
        # Falso positivo original (crítico): uma chamada legítima a `trackfw commit`
        # cuja mensagem menciona "git commit"/"git push" em algum lugar (ex.: dentro de
        # um heredoc) era bloqueada porque o parser buscava o padrão livremente na string
        # inteira, sem exigir que "git" fosse o primeiro token de um segmento real.
        proc = self._run('bin/trackfw commit -m "test message mentioning git commit inside"')
        self.assertEqual(proc.returncode, 0)
        self.assertEqual(proc.stdout, '')
        self.assertEqual(proc.stderr, '')


class TestGitBranchGuardML1AFalsePositiveAndSwitchC(unittest.TestCase):
    """ML-1A (ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-
    release-7-0-0.md): item 1 (falso-positivo por prosa que COMEÇA a linha) + item 2 (brecha
    `git switch -c`)."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        _generate_git_branch_guard_script(self.tmpdir)
        self.script_path = os.path.join(self.tmpdir, 'scripts', 'trackfw-git-branch-guard.sh')
        # ML-1A: o guard só bloqueia DENTRO de projeto trackfw -- estes testes exercitam
        # comportamento de bloqueio, precisam de trackfw.yaml na raiz do cwd (que é
        # explicitamente self.tmpdir via cwd= abaixo, nunca ambient).
        with open(os.path.join(self.tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('project_name: fixture\n')

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run(self, command: str):
        return subprocess.run(
            ['bash', self.script_path],
            input=json.dumps({'tool_input': {'command': command}}),
            capture_output=True,
            text=True,
            cwd=self.tmpdir,
        )

    def test_commit_message_line_starting_with_git_checkout_dash_b_does_not_block(self):
        # Reprodução literal do incidente real (vault/notes/git-branch-guard-falso-positivo-
        # em-linha-de-mensagem-de-commit-2026-08-16.md): via heredoc (`-m "$(cat <<'EOF' ...
        # EOF)"`, convenção deste próprio CLAUDE.md), a PRIMEIRA linha do corpo começa com
        # "git checkout -b" -- diferente de test_bug3 (que testa "git commit" no MEIO de uma
        # frase), aqui "git" é o PRIMEIRO token da linha.
        cmd = (
            "bin/trackfw commit -m \"$(cat <<'EOF'\n"
            "  git checkout -b            -> bloqueado pelo guard\n"
            "  trackfw branch new chore/  -> recusado\n"
            "EOF\n"
            ")\""
        )
        proc = self._run(cmd)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(proc.stdout, '')

    def test_quoted_message_then_real_chained_command_still_blocks(self):
        # Não-regressão crítica: -m fechado seguido de git push real encadeado por ';' ou '&&'
        # tem que continuar bloqueando.
        for cmd in ('git commit -m "x"; git push', 'git commit -m "x" && git push'):
            with self.subTest(cmd=cmd):
                proc = self._run(cmd)
                self.assertEqual(proc.returncode, 2, proc.stderr)
                self.assertIn('git commit bruto bloqueado', proc.stderr)

    def test_unterminated_heredoc_before_real_push_still_blocks(self):
        # Fallback de segurança de strip_heredoc_bodies: heredoc mal-formado não pode esconder
        # um git push real.
        cmd = "git status <<'EOF'\nwhatever\nNOTEOF\ngit push origin main"
        proc = self._run(cmd)
        self.assertEqual(proc.returncode, 2, proc.stderr)
        self.assertIn('git push bruto bloqueado', proc.stderr)

    def test_switch_dash_c_blocks(self):
        proc = self._run('git switch -c feat/x')
        self.assertEqual(proc.returncode, 2, proc.stderr)
        self.assertIn('git switch -c bruto bloqueado', proc.stderr)

    def test_switch_dash_c_flag_before_create_blocks(self):
        proc = self._run('git switch --track -c feat/x')
        self.assertEqual(proc.returncode, 2, proc.stderr)

    def test_switch_without_create_flag_allows(self):
        proc = self._run('git switch main')
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(proc.stdout, '')


class TestGitBranchGuardNoOpOutsideProject(unittest.TestCase):
    """ML-1A (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
    independente-de-fiacao.md): fora de projeto trackfw (sem trackfw.yaml em nenhum ancestral),
    o guard vira no-op (exit 0), sem inspecionar o comando."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        _generate_git_branch_guard_script(self.tmpdir)
        self.script_path = os.path.join(self.tmpdir, 'scripts', 'trackfw-git-branch-guard.sh')

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run(self, command: str, cwd: str):
        return subprocess.run(
            ['bash', self.script_path],
            input=json.dumps({'tool_input': {'command': command}}),
            capture_output=True,
            text=True,
            cwd=cwd,
        )

    def test_fixture_has_no_trackfw_yaml_ancestor(self):
        # Não-vacuidade: prova (em vez de presumir) que tempfile.mkdtemp() não tem
        # trackfw.yaml em nenhum ancestral.
        d = self.tmpdir
        while True:
            self.assertFalse(
                os.path.exists(os.path.join(d, 'trackfw.yaml')),
                f'premissa violada: trackfw.yaml em {d}',
            )
            parent = os.path.dirname(d)
            if parent == d:
                break
            d = parent

    def test_git_push_without_trackfw_yaml_is_noop(self):
        proc = self._run('git push', cwd=self.tmpdir)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(proc.stdout, '')

    def test_commit_checkout_branch_switch_without_trackfw_yaml_are_noop(self):
        for cmd in ('git commit -m "x"', 'git checkout -b feat/x', 'git branch nova', 'git switch -c feat/x'):
            with self.subTest(cmd=cmd):
                proc = self._run(cmd, cwd=self.tmpdir)
                self.assertEqual(proc.returncode, 0, proc.stderr)

    def test_git_push_with_trackfw_yaml_still_blocks(self):
        # Reverse-vacuity: MESMO script, só que rodado com trackfw.yaml no cwd -- prova que o
        # 0 acima veio do no-op, não de um build quebrado.
        with_yaml = tempfile.mkdtemp()
        try:
            with open(os.path.join(with_yaml, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
                f.write('project_name: fixture\n')
            proc = self._run('git push', cwd=with_yaml)
            self.assertEqual(proc.returncode, 2, proc.stderr)
        finally:
            shutil.rmtree(with_yaml, ignore_errors=True)

    def test_trackfw_yaml_in_ancestor_subdirectory_still_blocks(self):
        # A raiz do projeto é encontrada SUBINDO diretórios.
        root = tempfile.mkdtemp()
        try:
            with open(os.path.join(root, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
                f.write('project_name: fixture\n')
            sub = os.path.join(root, 'a', 'b', 'c')
            os.makedirs(sub)
            proc = self._run('git push', cwd=sub)
            self.assertEqual(proc.returncode, 2, proc.stderr)
        finally:
            shutil.rmtree(root, ignore_errors=True)


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
        self.assertEqual(data['tools'], ['*'])
        # ML-1A-bis (ROADMAP-2026-08-20): Go is canonical; only name/description/tools
        # are written on first creation. `prompt`/`mcpServers`/`toolAliases`/
        # `allowedTools`/`resources`/`useLegacyMcpJson` are deliberately NOT written --
        # an extra field the real schema doesn't expect risks failing validation,
        # while an absent optional field usually doesn't (docs/cli-parity.md).
        self.assertNotIn('prompt', data)
        self.assertNotIn('mcpServers', data)
        self.assertNotIn('toolAliases', data)
        self.assertNotIn('allowedTools', data)
        self.assertNotIn('resources', data)
        self.assertNotIn('useLegacyMcpJson', data)
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
