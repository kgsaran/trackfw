"""
tests/test_generators_init.py — testes para generators/init_gen.py
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from trackfw.generators.init_gen import scaffold


class TestScaffoldFlat(unittest.TestCase):
    """Verifica criação de estrutura flat."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_scaffold_flat(self):
        opts = {
            'project_name': 'meu-projeto',
            'namespacing': 'flat',
            'wip_limit': 1,
        }
        scaffold(self.tmp, opts)

        dirs_esperados = [
            'docs/adr',
            'docs/req',
            'docs/roadmaps/backlog',
            'docs/roadmaps/wip',
            'docs/roadmaps/blocked',
            'docs/roadmaps/done',
            'docs/roadmaps/abandoned',
        ]
        for d in dirs_esperados:
            full = os.path.join(self.tmp, d)
            self.assertTrue(os.path.isdir(full), f'Diretório ausente: {d}')

    def test_scaffold_flat_nao_cria_dirs_por_agente(self):
        """No modo flat não deve criar subpastas de agente dentro de docs/adr."""
        opts = {
            'project_name': 'meu-projeto',
            'namespacing': 'flat',
            'wip_limit': 1,
        }
        scaffold(self.tmp, opts)

        adr_dir = os.path.join(self.tmp, 'docs', 'adr')
        # No modo flat, não devem existir subdirs dentro de docs/adr além do ADR exemplo
        subdirs = [e for e in os.listdir(adr_dir) if os.path.isdir(os.path.join(adr_dir, e))]
        self.assertEqual(subdirs, [], f'Subdirs inesperados em docs/adr: {subdirs}')


class TestScaffoldByAgent(unittest.TestCase):
    """Verifica criação de estrutura by_agent com múltiplos agentes."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_scaffold_by_agent(self):
        opts = {
            'project_name': 'meu-projeto',
            'namespacing': 'by_agent',
            'agents': ['zeus', 'apolo'],
            'wip_limit': 2,
        }
        scaffold(self.tmp, opts)

        # docs/adr/<agent>
        for agent in ['zeus', 'apolo']:
            d = os.path.join(self.tmp, 'docs', 'adr', agent)
            self.assertTrue(os.path.isdir(d), f'Diretório ausente: docs/adr/{agent}')

        # docs/req (sempre flat)
        self.assertTrue(os.path.isdir(os.path.join(self.tmp, 'docs', 'req')))

        # docs/roadmaps/<agent>/<state>
        for agent in ['zeus', 'apolo']:
            for state in ['backlog', 'wip', 'blocked', 'done', 'abandoned']:
                d = os.path.join(self.tmp, 'docs', 'roadmaps', agent, state)
                self.assertTrue(os.path.isdir(d), f'Diretório ausente: docs/roadmaps/{agent}/{state}')

    def test_scaffold_by_agent_sem_agentes(self):
        """by_agent com lista vazia não cria nenhum subdir de agente."""
        opts = {
            'project_name': 'meu-projeto',
            'namespacing': 'by_agent',
            'agents': [],
            'wip_limit': 1,
        }
        # Não deve lançar exceção
        scaffold(self.tmp, opts)

        # docs/req deve existir (é sempre criado)
        self.assertTrue(os.path.isdir(os.path.join(self.tmp, 'docs', 'req')))


class TestTrackfwYamlGerado(unittest.TestCase):
    """Verifica conteúdo do trackfw.yaml para ambos os modos."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def _yaml_content(self):
        with open(os.path.join(self.tmp, 'trackfw.yaml'), encoding='utf-8') as f:
            return f.read()

    def test_trackfw_yaml_flat(self):
        opts = {
            'project_name': 'proj',
            'namespacing': 'flat',
            'wip_limit': 3,
        }
        scaffold(self.tmp, opts)
        content = self._yaml_content()

        self.assertIn('adr_dirs:', content)
        self.assertIn('- docs/adr', content)
        self.assertIn('req_dir: docs/req', content)
        self.assertIn('roadmap_dir: docs/roadmaps', content)
        self.assertIn('roadmap_namespacing: flat', content)
        self.assertIn('wip_limit: 3', content)
        # Não deve conter seção de agents no modo flat
        self.assertNotIn('agents:', content)

    def test_trackfw_yaml_by_agent(self):
        opts = {
            'project_name': 'proj',
            'namespacing': 'by_agent',
            'agents': ['zeus', 'apolo'],
            'wip_limit': 1,
        }
        scaffold(self.tmp, opts)
        content = self._yaml_content()

        self.assertIn('adr_dirs:', content)
        self.assertIn('- docs/adr/zeus', content)
        self.assertIn('- docs/adr/apolo', content)
        self.assertIn('req_dir: docs/req', content)
        self.assertIn('roadmap_dir: docs/roadmaps', content)
        self.assertIn('roadmap_namespacing: by_agent', content)
        self.assertIn('agents:', content)
        self.assertIn('- zeus', content)
        self.assertIn('- apolo', content)
        self.assertIn('wip_limit: 1', content)

    def test_trackfw_yaml_campos_obrigatorios(self):
        """Verifica que todos os campos obrigatórios estão presentes."""
        opts = {
            'project_name': 'qualquer',
            'namespacing': 'flat',
            'wip_limit': 1,
        }
        scaffold(self.tmp, opts)
        content = self._yaml_content()

        campos = ['adr_dirs:', 'req_dir:', 'roadmap_dir:', 'roadmap_namespacing:', 'wip_limit:']
        for campo in campos:
            self.assertIn(campo, content, f'Campo ausente no YAML: {campo}')


class TestIdempotente(unittest.TestCase):
    """Chamar scaffold duas vezes não deve falhar."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_idempotente(self):
        opts = {
            'project_name': 'proj',
            'namespacing': 'flat',
            'wip_limit': 1,
        }
        # Primeira chamada
        scaffold(self.tmp, opts)
        # Segunda chamada — não deve lançar exceção
        scaffold(self.tmp, opts)

        # Verificar que o YAML ainda existe e está correto
        yaml_path = os.path.join(self.tmp, 'trackfw.yaml')
        self.assertTrue(os.path.isfile(yaml_path))
        with open(yaml_path, encoding='utf-8') as f:
            content = f.read()
        self.assertIn('roadmap_namespacing: flat', content)

    def test_idempotente_by_agent(self):
        opts = {
            'project_name': 'proj',
            'namespacing': 'by_agent',
            'agents': ['zeus'],
            'wip_limit': 1,
        }
        scaffold(self.tmp, opts)
        scaffold(self.tmp, opts)  # Não deve falhar

        d = os.path.join(self.tmp, 'docs', 'adr', 'zeus')
        self.assertTrue(os.path.isdir(d))


class TestExemploADR(unittest.TestCase):
    """Verifica criação do ADR exemplo."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_adr_exemplo_flat(self):
        opts = {'project_name': 'p', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)

        adr_path = os.path.join(self.tmp, 'docs', 'adr', 'ADR-001-inicio-do-projeto.md')
        self.assertTrue(os.path.isfile(adr_path), 'ADR exemplo não criado no modo flat')

        with open(adr_path, encoding='utf-8') as f:
            content = f.read()
        self.assertIn('status: Proposed', content)
        self.assertIn('# ADR-001:', content)

    def test_adr_exemplo_by_agent(self):
        opts = {
            'project_name': 'p',
            'namespacing': 'by_agent',
            'agents': ['zeus', 'apolo'],
            'wip_limit': 1,
        }
        scaffold(self.tmp, opts)

        # ADR exemplo deve estar no diretório do primeiro agente
        adr_path = os.path.join(
            self.tmp, 'docs', 'adr', 'zeus', 'ADR-001-inicio-do-projeto.md'
        )
        self.assertTrue(os.path.isfile(adr_path), 'ADR exemplo não criado no modo by_agent')

    def test_adr_exemplo_nao_sobrescreve(self):
        """Segunda execução não deve sobrescrever ADR já existente."""
        opts = {'project_name': 'p', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)

        # Modifica o arquivo
        adr_path = os.path.join(self.tmp, 'docs', 'adr', 'ADR-001-inicio-do-projeto.md')
        with open(adr_path, 'w', encoding='utf-8') as f:
            f.write('conteudo modificado')

        # Segunda execução
        scaffold(self.tmp, opts)

        with open(adr_path, encoding='utf-8') as f:
            content = f.read()
        self.assertEqual(content, 'conteudo modificado', 'ADR foi sobrescrito indevidamente')


class TestGlobalADRsRuleDirective(unittest.TestCase):
    """Verifica que a diretiva de ADRs globais está presente no bloco de regras."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_rules_block_contains_global_adrs_directive(self):
        from trackfw.generators.init_gen import _trackfw_rules_block, inject_rules_for_tool

        block = _trackfw_rules_block()
        expected = (
            "Obrigatório: Inspecione e respeite todos os ADRs globais "
            "nos diretórios listados em adr_dirs (inclusive caminhos ~/...) "
            "antes de propor alterações de arquitetura."
        )
        self.assertIn(expected, block, "Diretiva de ADRs globais ausente do _trackfw_rules_block()")

        # Testar também a injeção em arquivo de agente (ex: CLAUDE.md)
        inject_rules_for_tool("claude", self.tmp)
        claude_md = os.path.join(self.tmp, "CLAUDE.md")
        self.assertTrue(os.path.isfile(claude_md))
        with open(claude_md, encoding="utf-8") as f:
            content = f.read()
        self.assertIn(expected, content, "Diretiva de ADRs globais ausente do CLAUDE.md gerado")


class TestGenerateClaudeMDHarnessSections(unittest.TestCase):
    """Verifica que generate_claude_md escreve as 9 seções de harness e preserva as pré-existentes."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_generate_claude_md_creates_harness_sections(self):
        from trackfw.generators.init_gen import generate_claude_md

        opts = {'project_name': 'test-harness-project', 'namespacing': 'flat', 'wip_limit': 1}
        generate_claude_md(self.tmp, opts)

        claude_path = os.path.join(self.tmp, 'CLAUDE.md')
        self.assertTrue(os.path.isfile(claude_path), 'CLAUDE.md não foi criado')

        with open(claude_path, encoding='utf-8') as f:
            content = f.read()

        harness_sections = [
            '## Branch strategy',
            '## Definition of done',
            '## Requirement scope',
            '## State requirements',
            '## Roadmap format',
            '## When governance is not required',
            '## Production incidents',
            '## Iterative prototyping',
            '## Autopilot',
        ]
        for section in harness_sections:
            self.assertIn(section, content, f'CLAUDE.md não contém a seção de harness: {section!r}')

        harness_snippets = [
            'One active branch at a time',
            'squash-merged',
            'Green build and tests do not close a microbatch',
            'explicit negative scope',
            '`blocked` requires a reason and an owner',
            'waves of microbatches',
            'closed list of exemptions',
            'This section takes precedence',
            'Inspect the live environment before proposing a fix',
            'disposable, isolated prototype',
            'Ask everything you need before starting',
        ]
        for snippet in harness_snippets:
            self.assertIn(snippet, content, f'CLAUDE.md não contém o trecho de harness: {snippet!r}')

        self.assertIn(
            '| `/trackfw:barrier` | Run the wave-release checklist before liberating the next wave |',
            content,
            'CLAUDE.md não anuncia o slash command /trackfw:barrier na tabela',
        )

    def test_generate_claude_md_architect_responses_section(self):
        from trackfw.generators.init_gen import generate_claude_md

        opts = {'project_name': 'verbosity-test', 'namespacing': 'flat', 'wip_limit': 1}
        generate_claude_md(self.tmp, opts)

        claude_path = os.path.join(self.tmp, 'CLAUDE.md')
        with open(claude_path, encoding='utf-8') as f:
            content = f.read()

        checks = [
            '## Architect responses',
            'Default: what changed',
            'what was decided',
            'what is needed from you. Three to five lines.',
            'a **blocker** that stops the next wave',
            'a **pending user decision** that cannot be inferred from context',
            'an **error the architect made** that cannot be self-corrected',
            'Never cut, even when short: measured evidence',
            'Cut: restating what an executor already reported',
            'Depth is on demand from the user.',
        ]
        for expected in checks:
            self.assertIn(
                expected, content,
                f'CLAUDE.md ## Architect responses não contém o trecho esperado: {expected!r}',
            )

    def test_generate_claude_md_preserves_pre_existing_sections(self):
        from trackfw.generators.init_gen import generate_claude_md

        opts = {'project_name': 'test-project', 'namespacing': 'flat', 'wip_limit': 1}
        generate_claude_md(self.tmp, opts)

        claude_path = os.path.join(self.tmp, 'CLAUDE.md')
        with open(claude_path, encoding='utf-8') as f:
            content = f.read()

        pre_existing = [
            '## Governance chain',
            '## Agent rules (mandatory)',
            '## Slash commands (Claude Code)',
            '## CLI commands (terminal / CI)',
            '## Architecture Directives (mandatory)',
            '## Pre-commit checklist',
            '## Git hooks',
            '## CI gate',
        ]
        for section in pre_existing:
            self.assertIn(section, content, f'CLAUDE.md perdeu a seção pré-existente: {section!r}')

    def test_scaffold_generates_claude_md_with_harness_sections(self):
        """scaffold() deve gerar CLAUDE.md com as 9 seções de harness."""
        opts = {'project_name': 'scaffold-harness-test', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)

        claude_path = os.path.join(self.tmp, 'CLAUDE.md')
        self.assertTrue(os.path.isfile(claude_path), 'CLAUDE.md não foi criado pelo scaffold')

        with open(claude_path, encoding='utf-8') as f:
            content = f.read()

        for section in ['## Branch strategy', '## Autopilot', '## When governance is not required']:
            self.assertIn(section, content, f'scaffold não gerou a seção de harness: {section!r}')


class TestAttentionScripts(unittest.TestCase):
    """Verifica geração dos scripts de atenção trackfw-attention-signal.sh e cleanup.sh."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_scaffold_generates_attention_scripts(self):
        opts = {'project_name': 'test-proj', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)

        signal_path = os.path.join(self.tmp, 'scripts', 'trackfw-attention-signal.sh')
        cleanup_path = os.path.join(self.tmp, 'scripts', 'trackfw-attention-cleanup.sh')
        guard_path = os.path.join(self.tmp, 'scripts', 'trackfw-credential-guard.sh')

        self.assertTrue(os.path.isfile(signal_path), 'trackfw-attention-signal.sh não foi criado')
        self.assertTrue(os.path.isfile(cleanup_path), 'trackfw-attention-cleanup.sh não foi criado')
        # trackfw init (via scaffold()) deve gerar o script de credential guard no
        # mesmo ciclo de vida dos scripts de attention — regressão do bug onde o
        # gerador existia mas nunca era chamado por nenhum fluxo real.
        self.assertTrue(os.path.isfile(guard_path), 'trackfw-credential-guard.sh não foi criado por scaffold()')

        # Permissão de execução no Unix
        if os.name == 'posix':
            self.assertTrue(os.stat(signal_path).st_mode & 0o111 != 0, 'signal script não é executável')
            self.assertTrue(os.stat(cleanup_path).st_mode & 0o111 != 0, 'cleanup script não é executável')
            self.assertTrue(os.stat(guard_path).st_mode & 0o111 != 0, 'credential guard script não é executável')

        with open(signal_path, encoding='utf-8') as f:
            signal_content = f.read()
        self.assertIn('# trackfw attention signal — PreToolUse/BeforeTool hook', signal_content)

        with open(cleanup_path, encoding='utf-8') as f:
            cleanup_content = f.read()
        self.assertIn('# trackfw attention cleanup — PostToolUse/AfterTool hook', cleanup_content)


# ROADMAP-2026-08-11 ML-3A: Codex has no project-root env var, so the command is wrapped in
# literal double quotes around `$(git rev-parse --show-toplevel)` per ADR-2026-08-11 -- matches
# _SIGNAL_CMD_CODEX/_GUARD_CMD_CODEX/_CLEANUP_CMD_CODEX in trackfw/generators/hooks.py.
_CODEX_SIGNAL_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-signal.sh"'
_CODEX_CLEANUP_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-cleanup.sh"'
_CODEX_GUARD_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"'
# ML-3C (ROADMAP-2026-08-14): same $(git rev-parse --show-toplevel) wrapper, git-branch-guard
# script -- matches _GIT_GUARD_CMD_CODEX in trackfw/generators/hooks.py.
_CODEX_GIT_GUARD_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"'

# ROADMAP-2026-08-11 ML-4A: Gemini documents and uses $GEMINI_PROJECT_DIR in 100% of its official
# hook command examples (ADR-2026-08-11, "Gemini CLI — alterar, por argumento de assimetria") --
# matches _SIGNAL_CMD_GEMINI/_CLEANUP_CMD_GEMINI/_GUARD_CMD_GEMINI in trackfw/generators/hooks.py.
_GEMINI_SIGNAL_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-signal.sh'
_GEMINI_CLEANUP_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'
_GEMINI_GUARD_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'
# ML-3C (ROADMAP-2026-08-14): matches _GIT_GUARD_CMD_GEMINI in trackfw/generators/hooks.py.
_GEMINI_GIT_GUARD_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'


class TestAttentionHooksInjectors(unittest.TestCase):
    """Testes unitários para injeção idempotente de hooks de atenção nos 7 CLIs."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        # Isolate the global credential-guard dedup check (ML-3A,
        # trackfw.generators.hooks._global_credential_guard_installed_*) from the
        # real $HOME -- none of these tests should depend on the developer's
        # actual home dir.
        self._orig_home = os.environ.get('HOME')
        os.environ['HOME'] = tempfile.mkdtemp()

    def tearDown(self):
        if self._orig_home is None:
            os.environ.pop('HOME', None)
        else:
            os.environ['HOME'] = self._orig_home

    def test_inject_claude_hooks_create_and_merge(self):
        from trackfw.generators.hooks import inject_claude_hooks
        # 1. Criação do zero
        inject_claude_hooks(self.tmp)
        path = os.path.join(self.tmp, '.claude', 'settings.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertIn('PreToolUse', data.get('hooks', {}))
        self.assertIn('PostToolUse', data.get('hooks', {}))
        pre_matchers = {e.get('matcher') for e in data['hooks']['PreToolUse']}
        post_matchers = {e.get('matcher') for e in data['hooks']['PostToolUse']}
        # ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): PreToolUse/PostToolUse each gain
        # two new credential-guard entries (Read, Write|Edit) alongside the pre-existing Bash one.
        self.assertEqual(pre_matchers, {'AskUserQuestion', 'Bash', 'Read', 'Write|Edit'})
        self.assertEqual(post_matchers, {'AskUserQuestion', 'Bash', 'Read', 'Write|Edit'})

        # 2. Idempotência
        inject_claude_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['PreToolUse']), 4)
        self.assertEqual(len(data2['hooks']['PostToolUse']), 4)

    def test_inject_claude_hooks_preserves_third_party_matcher(self):
        """PreToolUse/PostToolUse com um matcher de terceiro (ex.: 'CustomTool')
        deve ser preservado ao lado de AskUserQuestion + Bash, sem duplicar
        entradas do mesmo matcher em execuções repetidas (ML-2A)."""
        from trackfw.generators.hooks import inject_claude_hooks

        settings_dir = os.path.join(self.tmp, '.claude')
        os.makedirs(settings_dir, exist_ok=True)
        settings_path = os.path.join(settings_dir, 'settings.json')
        with open(settings_path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'PreToolUse': [
                        {'matcher': 'CustomTool', 'hooks': [{'type': 'command', 'command': 'custom.sh'}]}
                    ]
                }
            }, f)

        inject_claude_hooks(self.tmp)
        inject_claude_hooks(self.tmp)  # idempotência

        with open(settings_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        pre = data['hooks']['PreToolUse']
        self.assertEqual(len(pre), 5)
        matchers = {e['matcher'] for e in pre}
        self.assertEqual(matchers, {'CustomTool', 'AskUserQuestion', 'Bash', 'Read', 'Write|Edit'})

        bash_entry = next(e for e in pre if e['matcher'] == 'Bash')
        # ML-3C (ROADMAP-2026-08-14): Bash matcher also carries the unconditional
        # git-branch-guard entry now, alongside credential-guard -- no new matcher is
        # added (git-branch-guard reuses the existing "Bash" matcher), so `matchers`
        # above is unchanged.
        self.assertEqual(
            [h['command'] for h in bash_entry['hooks']],
            [
                '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh',
                '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh',
            ],
        )

        post = data['hooks']['PostToolUse']
        self.assertEqual(len(post), 4)
        post_matchers = {e['matcher'] for e in post}
        self.assertEqual(post_matchers, {'AskUserQuestion', 'Bash', 'Read', 'Write|Edit'})

    def test_inject_claude_hooks_migrates_legacy_relative_path_command(self):
        """Regressão do bug reportado em produção (2026-08-09, projeto CMDB): o comando do
        credential-guard era um caminho relativo puro, que o Claude Code resolve contra o cwd
        *dinâmico* do hook (rastreia cd's do agente), não a raiz do projeto -- qualquer chamada
        depois de um cd para um subdiretório falhava com "No such file or directory". Confirma
        que re-injetar sobre um settings.json já escrito por uma versão antiga REESCREVE o
        comando legado em vez de só acrescentar um segundo hook ao lado do quebrado."""
        from trackfw.generators.hooks import inject_claude_hooks

        settings_dir = os.path.join(self.tmp, '.claude')
        os.makedirs(settings_dir, exist_ok=True)
        settings_path = os.path.join(settings_dir, 'settings.json')
        with open(settings_path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'PreToolUse': [
                        {'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': 'scripts/trackfw-credential-guard.sh'}]},
                        {'matcher': 'Read', 'hooks': [{'type': 'command', 'command': 'scripts/trackfw-credential-guard.sh'}]},
                    ],
                    'PostToolUse': [
                        {'matcher': 'Write|Edit', 'hooks': [{'type': 'command', 'command': 'scripts/trackfw-credential-guard.sh'}]},
                    ],
                }
            }, f)

        inject_claude_hooks(self.tmp)

        with open(settings_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        bash_entry = next(e for e in data['hooks']['PreToolUse'] if e['matcher'] == 'Bash')
        read_entry = next(e for e in data['hooks']['PreToolUse'] if e['matcher'] == 'Read')
        write_edit_entry = next(e for e in data['hooks']['PostToolUse'] if e['matcher'] == 'Write|Edit')

        # ML-3C (ROADMAP-2026-08-14): PreToolUse[Bash] now also carries the unconditional
        # git-branch-guard entry alongside credential-guard (Bash is the only matcher this
        # new guard touches -- Read/Write|Edit are untouched, since git commands never
        # reach a subagent through those tools).
        self.assertEqual(
            [h['command'] for h in bash_entry['hooks']],
            [
                '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh',
                '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh',
            ],
        )
        for entry in (read_entry, write_edit_entry):
            self.assertEqual(
                [h['command'] for h in entry['hooks']],
                ['$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'],
            )

    def test_inject_claude_hooks_migrates_legacy_relative_path_attention_signal_cleanup(self):
        """ROADMAP-2026-08-11 ML-2A: mesma classe de bug de resolução de cwd do teste do
        credential-guard acima, aplicada ao attention-signal/cleanup -- confirma que re-injetar
        sobre um settings.json já escrito por uma versão antiga REESCREVE o comando legado em vez
        de só acrescentar um segundo hook ao lado do quebrado."""
        from trackfw.generators.hooks import inject_claude_hooks

        settings_dir = os.path.join(self.tmp, '.claude')
        os.makedirs(settings_dir, exist_ok=True)
        settings_path = os.path.join(settings_dir, 'settings.json')
        with open(settings_path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'PreToolUse': [
                        {'matcher': 'AskUserQuestion', 'hooks': [{'type': 'command', 'command': 'scripts/trackfw-attention-signal.sh'}]},
                    ],
                    'PostToolUse': [
                        {'matcher': 'AskUserQuestion', 'hooks': [{'type': 'command', 'command': 'scripts/trackfw-attention-cleanup.sh'}]},
                    ],
                }
            }, f)

        inject_claude_hooks(self.tmp)

        with open(settings_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        signal_entry = next(e for e in data['hooks']['PreToolUse'] if e['matcher'] == 'AskUserQuestion')
        cleanup_entry = next(e for e in data['hooks']['PostToolUse'] if e['matcher'] == 'AskUserQuestion')

        self.assertEqual(
            [h['command'] for h in signal_entry['hooks']],
            ['$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh'],
        )
        self.assertEqual(
            [h['command'] for h in cleanup_entry['hooks']],
            ['$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'],
        )

    def test_inject_codex_hooks_create_and_merge(self):
        from trackfw.generators.hooks import inject_codex_hooks
        inject_codex_hooks(self.tmp)
        path = os.path.join(self.tmp, '.codex', 'hooks.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertIn('PermissionRequest', data.get('hooks', {}))
        self.assertIn('PreToolUse', data.get('hooks', {}))
        self.assertIn('PostToolUse', data.get('hooks', {}))

        pre_matcher = data['hooks']['PreToolUse'][0]['matcher']
        self.assertEqual(pre_matcher, 'Bash')
        pre_command = data['hooks']['PreToolUse'][0]['hooks'][0]['command']
        self.assertEqual(pre_command, _CODEX_GUARD_CMD)
        # ADR-2026-08-06 emenda 7: Codex has no dedicated read matcher (documented
        # limitation) -- only apply_patch (write/edit) is added alongside Bash.
        self.assertEqual(data['hooks']['PreToolUse'][1]['matcher'], 'apply_patch')
        self.assertEqual(
            data['hooks']['PreToolUse'][1]['hooks'][0]['command'],
            _CODEX_GUARD_CMD,
        )

        post_matchers = {e['matcher'] for e in data['hooks']['PostToolUse']}
        self.assertEqual(post_matchers, {'.*', 'Bash', 'apply_patch'})

        # Idempotência
        inject_codex_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['PermissionRequest']), 1)
        self.assertEqual(len(data2['hooks']['PreToolUse']), 2)
        self.assertEqual(len(data2['hooks']['PostToolUse']), 3)

    def test_inject_codex_hooks_preserves_existing_bash_entry(self):
        """Um matcher 'Bash' pré-existente em PreToolUse (hook de terceiro) deve
        ser mesclado com o novo comando do credential-guard, sem duplicar a
        entrada do matcher (mesmo padrão do merge do Claude Code, ML-2A)."""
        from trackfw.generators.hooks import inject_codex_hooks

        hooks_dir = os.path.join(self.tmp, '.codex')
        os.makedirs(hooks_dir, exist_ok=True)
        hooks_path = os.path.join(hooks_dir, 'hooks.json')
        with open(hooks_path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'PreToolUse': [
                        {'matcher': 'Bash', 'hooks': [{'type': 'command', 'command': 'scripts/other.sh'}]}
                    ]
                }
            }, f)

        inject_codex_hooks(self.tmp)
        inject_codex_hooks(self.tmp)  # idempotência

        with open(hooks_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        pre = data['hooks']['PreToolUse']
        # ADR-2026-08-06 emenda 7: apply_patch is now added alongside Bash. ML-3C
        # (ROADMAP-2026-08-14): git-branch-guard also targets the Bash matcher (no new
        # matcher entry -- same reuse pattern as Claude Code above).
        self.assertEqual(len(pre), 2)
        self.assertEqual(pre[0]['matcher'], 'Bash')
        commands = {h['command'] for h in pre[0]['hooks']}
        self.assertEqual(commands, {'scripts/other.sh', _CODEX_GUARD_CMD, _CODEX_GIT_GUARD_CMD})
        self.assertEqual(pre[1]['matcher'], 'apply_patch')

    def test_inject_codex_hooks_migration_wiring_rewrites_in_place_not_duplicate(self):
        """ML-1A migration wiring, now exercised as a genuine migration (ROADMAP-2026-08-11
        ML-3A): _migrate_hook_command is called before the merge for every trackfw-owned matcher
        in inject_codex_hooks. This fixture pre-populates every trackfw-owned matcher with the
        pre-ML-3A relative-path command, exactly as an older trackfw run would have left it, and
        asserts the injector rewrites each entry to the new
        $(git rev-parse --show-toplevel)-pinned command in place instead of appending a second,
        still-cwd-fragile entry alongside it."""
        from trackfw.generators.hooks import inject_codex_hooks

        def mk(matcher, command):
            return {'matcher': matcher, 'hooks': [{'type': 'command', 'command': command}]}

        hooks_dir = os.path.join(self.tmp, '.codex')
        os.makedirs(hooks_dir, exist_ok=True)
        hooks_path = os.path.join(hooks_dir, 'hooks.json')
        with open(hooks_path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'PermissionRequest': [mk('.*', 'scripts/trackfw-attention-signal.sh')],
                    'PreToolUse': [
                        mk('Bash', 'scripts/trackfw-credential-guard.sh'),
                        mk('apply_patch', 'scripts/trackfw-credential-guard.sh'),
                    ],
                    'PostToolUse': [
                        mk('.*', 'scripts/trackfw-attention-cleanup.sh'),
                        mk('Bash', 'scripts/trackfw-credential-guard.sh'),
                        mk('apply_patch', 'scripts/trackfw-credential-guard.sh'),
                    ],
                }
            }, f)

        inject_codex_hooks(self.tmp)

        with open(hooks_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        def check_one(event, matcher, command):
            entries = [e for e in data['hooks'][event] if e['matcher'] == matcher]
            self.assertEqual(len(entries), 1, f'{event}[{matcher}]: expected exactly 1 matcher entry (no duplicate)')
            self.assertEqual(len(entries[0]['hooks']), 1, f'{event}[{matcher}]: expected exactly 1 hook')
            self.assertEqual(entries[0]['hooks'][0]['command'], command, f'{event}[{matcher}]: unexpected command')

        def check_commands(event, matcher, commands):
            entries = [e for e in data['hooks'][event] if e['matcher'] == matcher]
            self.assertEqual(len(entries), 1, f'{event}[{matcher}]: expected exactly 1 matcher entry (no duplicate)')
            self.assertEqual(
                [h['command'] for h in entries[0]['hooks']], commands, f'{event}[{matcher}]: unexpected commands'
            )

        check_one('PermissionRequest', '.*', _CODEX_SIGNAL_CMD)
        # ML-3C (ROADMAP-2026-08-14): PreToolUse[Bash] also carries the unconditional
        # git-branch-guard entry now (Bash-only -- no PostToolUse/apply_patch
        # counterpart, see the design-note block above _GIT_GUARD_CMD_CLAUDE in
        # trackfw/generators/hooks.py).
        check_commands('PreToolUse', 'Bash', [_CODEX_GUARD_CMD, _CODEX_GIT_GUARD_CMD])
        check_one('PreToolUse', 'apply_patch', _CODEX_GUARD_CMD)
        check_one('PostToolUse', '.*', _CODEX_CLEANUP_CMD)
        check_one('PostToolUse', 'Bash', _CODEX_GUARD_CMD)
        check_one('PostToolUse', 'apply_patch', _CODEX_GUARD_CMD)

    def test_inject_gemini_hooks_create_and_merge(self):
        from trackfw.generators.hooks import inject_gemini_hooks
        inject_gemini_hooks(self.tmp)
        path = os.path.join(self.tmp, '.gemini', 'settings.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertIn('Notification', data.get('hooks', {}))
        self.assertIn('AfterTool', data.get('hooks', {}))
        self.assertIn('BeforeTool', data.get('hooks', {}))

        before = data['hooks']['BeforeTool']
        # ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): read_file|read_many_files and
        # write_file|replace credential-guard entries alongside run_shell_command.
        self.assertEqual(len(before), 3)
        self.assertEqual(before[0]['matcher'], 'run_shell_command')
        self.assertEqual(before[0]['hooks'][0]['command'], _GEMINI_GUARD_CMD)
        before_matchers = {e['matcher'] for e in before}
        self.assertEqual(before_matchers, {'run_shell_command', 'read_file|read_many_files', 'write_file|replace'})

        after = data['hooks']['AfterTool']
        after_matchers = {e['matcher'] for e in after}
        self.assertEqual(after_matchers, {'*', 'run_shell_command', 'read_file|read_many_files', 'write_file|replace'})

        # Idempotência
        inject_gemini_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['Notification']), 1)
        self.assertEqual(len(data2['hooks']['AfterTool']), 4)
        self.assertEqual(len(data2['hooks']['BeforeTool']), 3)

    def test_inject_gemini_hooks_preserves_existing_before_tool_entry(self):
        from trackfw.generators.hooks import inject_gemini_hooks
        path = os.path.join(self.tmp, '.gemini', 'settings.json')
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'BeforeTool': [
                        {'matcher': 'run_shell_command', 'hooks': [{'type': 'command', 'command': 'scripts/other.sh'}]},
                    ],
                },
            }, f)

        inject_gemini_hooks(self.tmp)
        inject_gemini_hooks(self.tmp)

        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        before = data['hooks']['BeforeTool']
        # ADR-2026-08-06 emenda 7: read_file|read_many_files and write_file|replace entries
        # are added alongside run_shell_command. ML-3C (ROADMAP-2026-08-14): git-branch-guard
        # also targets the run_shell_command matcher (Bash-only equivalent, no new matcher).
        self.assertEqual(len(before), 3)
        commands = {h['command'] for h in before[0]['hooks']}
        self.assertEqual(commands, {'scripts/other.sh', _GEMINI_GUARD_CMD, _GEMINI_GIT_GUARD_CMD})

    def test_inject_gemini_hooks_migration_wiring_rewrites_in_place_not_duplicate(self):
        """ML-4A: Gemini counterpart of test_inject_codex_hooks_migration_wiring_rewrites_in_place_not_duplicate.
        The fixture below is an old settings.json written by a pre-ML-4A trackfw (relative-path
        commands); inject_gemini_hooks must rewrite each entry in place to $GEMINI_PROJECT_DIR/...
        form rather than duplicating it."""
        from trackfw.generators.hooks import inject_gemini_hooks

        def mk(matcher, command):
            return {'matcher': matcher, 'hooks': [{'type': 'command', 'command': command}]}

        path = os.path.join(self.tmp, '.gemini', 'settings.json')
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, 'w', encoding='utf-8') as f:
            json.dump({
                'hooks': {
                    'Notification': [mk('ToolPermission', 'scripts/trackfw-attention-signal.sh')],
                    'BeforeTool': [
                        mk('run_shell_command', 'scripts/trackfw-credential-guard.sh'),
                        mk('read_file|read_many_files', 'scripts/trackfw-credential-guard.sh'),
                        mk('write_file|replace', 'scripts/trackfw-credential-guard.sh'),
                    ],
                    'AfterTool': [
                        mk('*', 'scripts/trackfw-attention-cleanup.sh'),
                        mk('run_shell_command', 'scripts/trackfw-credential-guard.sh'),
                        mk('read_file|read_many_files', 'scripts/trackfw-credential-guard.sh'),
                        mk('write_file|replace', 'scripts/trackfw-credential-guard.sh'),
                    ],
                },
            }, f)

        inject_gemini_hooks(self.tmp)

        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        def check_one(event, matcher, command):
            entries = [e for e in data['hooks'][event] if e['matcher'] == matcher]
            self.assertEqual(len(entries), 1, f'{event}[{matcher}]: expected exactly 1 matcher entry (no duplicate)')
            self.assertEqual(len(entries[0]['hooks']), 1, f'{event}[{matcher}]: expected exactly 1 hook')
            self.assertEqual(entries[0]['hooks'][0]['command'], command, f'{event}[{matcher}]: unexpected command')

        def check_commands(event, matcher, commands):
            entries = [e for e in data['hooks'][event] if e['matcher'] == matcher]
            self.assertEqual(len(entries), 1, f'{event}[{matcher}]: expected exactly 1 matcher entry (no duplicate)')
            self.assertEqual(
                [h['command'] for h in entries[0]['hooks']], commands, f'{event}[{matcher}]: unexpected commands'
            )

        check_one('Notification', 'ToolPermission', _GEMINI_SIGNAL_CMD)
        # ML-3C (ROADMAP-2026-08-14): BeforeTool[run_shell_command] also carries the
        # unconditional git-branch-guard entry now (run_shell_command-only -- no
        # read_file|read_many_files/write_file|replace/AfterTool counterpart, see the
        # design-note block above _GIT_GUARD_CMD_CLAUDE in trackfw/generators/hooks.py).
        check_commands('BeforeTool', 'run_shell_command', [_GEMINI_GUARD_CMD, _GEMINI_GIT_GUARD_CMD])
        check_one('BeforeTool', 'read_file|read_many_files', _GEMINI_GUARD_CMD)
        check_one('BeforeTool', 'write_file|replace', _GEMINI_GUARD_CMD)
        check_one('AfterTool', '*', _GEMINI_CLEANUP_CMD)
        check_one('AfterTool', 'run_shell_command', _GEMINI_GUARD_CMD)
        check_one('AfterTool', 'read_file|read_many_files', _GEMINI_GUARD_CMD)
        check_one('AfterTool', 'write_file|replace', _GEMINI_GUARD_CMD)

    def test_inject_kiro_hooks(self):
        from trackfw.generators.hooks import inject_kiro_hooks
        inject_kiro_hooks(self.tmp)
        path = os.path.join(self.tmp, '.kiro', 'hooks', 'trackfw-attention.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertEqual(data.get('version'), 'v1')
        hooks = data.get('hooks', [])
        # ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): +4 credential-guard entries
        # (read-pre/read-post/write-pre/write-post) alongside the pre-existing shell pre/post.
        # ML-3C (ROADMAP-2026-08-14): Kiro is NOT one of the roadmap's "7 runtimes" -- no
        # git-branch-guard entry is added here, matching Go's InjectKiroHooks.
        self.assertEqual(len(hooks), 8)
        for entry in hooks:
            self.assertNotIn('event', entry, 'legacy "event" field must not be emitted')
            self.assertIn('trigger', entry)
            self.assertNotIsInstance(entry.get('matcher'), dict, 'matcher must be a plain regex string')

        by_name = {h['name']: h for h in hooks}
        self.assertEqual(by_name['trackfw-attention-signal']['trigger'], 'PreToolUse')
        self.assertEqual(by_name['trackfw-attention-cleanup']['trigger'], 'PostToolUse')
        guard_pre = by_name['trackfw-credential-guard-pre']
        self.assertEqual(guard_pre['trigger'], 'PreToolUse')
        self.assertEqual(guard_pre['matcher'], 'shell')
        self.assertEqual(guard_pre['action']['command'], 'scripts/trackfw-credential-guard.sh')
        guard_post = by_name['trackfw-credential-guard-post']
        self.assertEqual(guard_post['trigger'], 'PostToolUse')
        self.assertEqual(guard_post['matcher'], 'shell')

        guard_read_pre = by_name['trackfw-credential-guard-read-pre']
        self.assertEqual(guard_read_pre['trigger'], 'PreToolUse')
        self.assertEqual(guard_read_pre['matcher'], 'read')
        guard_read_post = by_name['trackfw-credential-guard-read-post']
        self.assertEqual(guard_read_post['trigger'], 'PostToolUse')
        self.assertEqual(guard_read_post['matcher'], 'read')
        guard_write_pre = by_name['trackfw-credential-guard-write-pre']
        self.assertEqual(guard_write_pre['trigger'], 'PreToolUse')
        self.assertEqual(guard_write_pre['matcher'], 'write')
        guard_write_post = by_name['trackfw-credential-guard-write-post']
        self.assertEqual(guard_write_post['trigger'], 'PostToolUse')
        self.assertEqual(guard_write_post['matcher'], 'write')

        # ML-3C (ROADMAP-2026-08-14): Kiro is not one of the roadmap's "7 runtimes" --
        # no git-branch-guard entry expected.
        self.assertNotIn('trackfw-git-branch-guard-pre', by_name)

        # Idempotência
        inject_kiro_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(data, data2)

    def test_inject_copilot_hooks(self):
        from trackfw.generators.hooks import inject_copilot_hooks
        inject_copilot_hooks(self.tmp)
        path = os.path.join(self.tmp, '.github', 'hooks', 'trackfw-attention.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertEqual(data.get('version'), 1)
        self.assertIn('preToolUse', data.get('hooks', {}))
        self.assertIn('postToolUse', data.get('hooks', {}))
        # ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): +2 credential-guard entries
        # (view, create|edit) alongside the pre-existing bash one, in each of
        # preToolUse/postToolUse. ML-3C (ROADMAP-2026-08-14): +1 unconditional
        # git-branch-guard entry (preToolUse only, matcher "bash").
        self.assertEqual(len(data['hooks']['preToolUse']), 5)
        self.assertEqual(len(data['hooks']['postToolUse']), 4)

        def find_by_bash(entries, bash):
            return next((e for e in entries if e.get('bash') == bash), None)

        def find_by_matcher(entries, bash, matcher):
            return next((e for e in entries if e.get('bash') == bash and e.get('matcher') == matcher), None)

        signal = find_by_bash(data['hooks']['preToolUse'], 'scripts/trackfw-attention-signal.sh')
        self.assertIsNotNone(signal, 'preToolUse missing attention-signal entry')
        self.assertNotIn('matcher', signal)

        guard_pre = find_by_matcher(data['hooks']['preToolUse'], 'scripts/trackfw-credential-guard.sh', 'bash')
        self.assertIsNotNone(guard_pre, 'preToolUse missing credential-guard bash entry')

        guard_pre_view = find_by_matcher(data['hooks']['preToolUse'], 'scripts/trackfw-credential-guard.sh', 'view')
        self.assertIsNotNone(guard_pre_view, 'preToolUse missing credential-guard view entry')

        guard_pre_edit = find_by_matcher(data['hooks']['preToolUse'], 'scripts/trackfw-credential-guard.sh', 'create|edit')
        self.assertIsNotNone(guard_pre_edit, 'preToolUse missing credential-guard create|edit entry')

        cleanup = find_by_bash(data['hooks']['postToolUse'], 'scripts/trackfw-attention-cleanup.sh')
        self.assertIsNotNone(cleanup, 'postToolUse missing attention-cleanup entry')

        guard_post = find_by_matcher(data['hooks']['postToolUse'], 'scripts/trackfw-credential-guard.sh', 'bash')
        self.assertIsNotNone(guard_post, 'postToolUse missing credential-guard bash entry')

        guard_post_view = find_by_matcher(data['hooks']['postToolUse'], 'scripts/trackfw-credential-guard.sh', 'view')
        self.assertIsNotNone(guard_post_view, 'postToolUse missing credential-guard view entry')

        guard_post_edit = find_by_matcher(data['hooks']['postToolUse'], 'scripts/trackfw-credential-guard.sh', 'create|edit')
        self.assertIsNotNone(guard_post_edit, 'postToolUse missing credential-guard create|edit entry')

        # ML-3C (ROADMAP-2026-08-14): git-branch-guard, preToolUse only.
        git_guard_pre = find_by_matcher(data['hooks']['preToolUse'], 'scripts/trackfw-git-branch-guard.sh', 'bash')
        self.assertIsNotNone(git_guard_pre, 'preToolUse missing git-branch-guard bash entry')
        git_guard_post = find_by_matcher(data['hooks']['postToolUse'], 'scripts/trackfw-git-branch-guard.sh', 'bash')
        self.assertIsNone(git_guard_post, 'postToolUse must not carry a git-branch-guard entry')

        # Idempotência
        inject_copilot_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(data, data2)

    def test_inject_cursor_hooks(self):
        from trackfw.generators.hooks import inject_cursor_hooks
        inject_cursor_hooks(self.tmp)
        path = os.path.join(self.tmp, '.cursor', 'hooks.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        # Legacy top-level schema must not be written anymore.
        self.assertNotIn('preToolUse', data)
        self.assertNotIn('postToolUse', data)

        self.assertEqual(data.get('version'), 1)
        self.assertIn('hooks', data)
        # ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): +2 credential-guard entries
        # (matcher Read, matcher Write) added to the generic preToolUse/postToolUse events
        # alongside the unfiltered attention-signal/cleanup entry already there.
        self.assertEqual(len(data['hooks']['preToolUse']), 3)
        self.assertEqual(
            data['hooks']['preToolUse'][0]['command'],
            'scripts/trackfw-attention-signal.sh',
        )
        self.assertEqual(len(data['hooks']['postToolUse']), 3)
        self.assertEqual(
            data['hooks']['postToolUse'][0]['command'],
            'scripts/trackfw-attention-cleanup.sh',
        )
        # ML-3C (ROADMAP-2026-08-14): beforeShellExecution also carries the unconditional
        # git-branch-guard entry now (beforeShellExecution-only -- no afterShellExecution
        # counterpart, see the design-note block above _GIT_GUARD_CMD_CLAUDE in
        # trackfw/generators/hooks.py).
        self.assertEqual(len(data['hooks']['beforeShellExecution']), 2)
        self.assertEqual(
            data['hooks']['beforeShellExecution'][0]['command'],
            'scripts/trackfw-credential-guard.sh',
        )
        self.assertEqual(
            data['hooks']['beforeShellExecution'][1]['command'],
            'scripts/trackfw-git-branch-guard.sh',
        )
        self.assertEqual(len(data['hooks']['afterShellExecution']), 1)
        self.assertEqual(
            data['hooks']['afterShellExecution'][0]['command'],
            'scripts/trackfw-credential-guard.sh',
        )

        def find_guard(entries, matcher):
            return next(
                (e for e in entries if e.get('command') == 'scripts/trackfw-credential-guard.sh' and e.get('matcher') == matcher),
                None,
            )

        self.assertIsNotNone(find_guard(data['hooks']['preToolUse'], 'Read'), 'preToolUse missing credential-guard Read entry')
        self.assertIsNotNone(find_guard(data['hooks']['preToolUse'], 'Write'), 'preToolUse missing credential-guard Write entry')
        self.assertIsNotNone(find_guard(data['hooks']['postToolUse'], 'Read'), 'postToolUse missing credential-guard Read entry')
        self.assertIsNotNone(find_guard(data['hooks']['postToolUse'], 'Write'), 'postToolUse missing credential-guard Write entry')

        # Idempotência
        inject_cursor_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['preToolUse']), 3)
        self.assertEqual(len(data2['hooks']['postToolUse']), 3)
        self.assertEqual(len(data2['hooks']['beforeShellExecution']), 2)
        self.assertEqual(len(data2['hooks']['afterShellExecution']), 1)

    def test_inject_cursor_hooks_preserves_existing_version(self):
        from trackfw.generators.hooks import inject_cursor_hooks
        cursor_dir = os.path.join(self.tmp, '.cursor')
        os.makedirs(cursor_dir, exist_ok=True)
        path = os.path.join(cursor_dir, 'hooks.json')
        with open(path, 'w', encoding='utf-8') as f:
            json.dump({'version': 2, 'hooks': {}}, f)

        inject_cursor_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertEqual(data['version'], 2)

    def test_inject_cursor_hooks_migrates_legacy_top_level_arrays(self):
        from trackfw.generators.hooks import inject_cursor_hooks
        cursor_dir = os.path.join(self.tmp, '.cursor')
        os.makedirs(cursor_dir, exist_ok=True)
        path = os.path.join(cursor_dir, 'hooks.json')
        legacy = {
            'preToolUse': [
                {'command': 'scripts/trackfw-attention-signal.sh'},
                {'command': 'user-pre.sh'},
            ],
            'postToolUse': [
                {'command': 'scripts/trackfw-attention-cleanup.sh'},
            ],
        }
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(legacy, f)

        inject_cursor_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        # Known trackfw entry removed from top level; unrelated entry survives.
        self.assertEqual(len(data['preToolUse']), 1)
        self.assertEqual(data['preToolUse'][0]['command'], 'user-pre.sh')
        # postToolUse only had the known trackfw entry -> key removed entirely.
        self.assertNotIn('postToolUse', data)

        # Entries migrated to the real, nested location.
        self.assertEqual(data['hooks']['preToolUse'][0]['command'], 'scripts/trackfw-attention-signal.sh')
        self.assertEqual(data['hooks']['postToolUse'][0]['command'], 'scripts/trackfw-attention-cleanup.sh')

    def test_inject_hooks_detected(self):
        from trackfw.generators.hooks import inject_hooks_detected
        # Simular presença dos CLIs
        os.makedirs(os.path.join(self.tmp, '.claude'), exist_ok=True)
        os.makedirs(os.path.join(self.tmp, '.codex'), exist_ok=True)
        os.makedirs(os.path.join(self.tmp, '.gemini'), exist_ok=True)
        os.makedirs(os.path.join(self.tmp, '.kiro'), exist_ok=True)
        os.makedirs(os.path.join(self.tmp, '.github'), exist_ok=True)
        with open(os.path.join(self.tmp, '.github', 'copilot-instructions.md'), 'w') as f:
            f.write('# Copilot')
        os.makedirs(os.path.join(self.tmp, '.cursor'), exist_ok=True)

        inject_hooks_detected(self.tmp)

        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.claude', 'settings.json')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.codex', 'hooks.json')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.gemini', 'settings.json')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.kiro', 'hooks', 'trackfw-attention.json')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.github', 'hooks', 'trackfw-attention.json')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.cursor', 'hooks.json')))

    def test_windsurf_instruction_in_rules(self):
        from trackfw.generators.init_gen import _trackfw_rules_block
        block = _trackfw_rules_block()
        self.assertIn('Windsurf users:', block)
        self.assertIn('<roadmap_dir>/.trackfw-attention.json', block)

    def test_update_command_injects_attention_hooks(self):
        from trackfw.commands.update import _run
        import argparse
        import os

        # Criar projeto fake com trackfw.yaml e .claude/
        with open(os.path.join(self.tmp, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('backend: python\nroadmap_dir: docs/roadmaps\n')
        os.makedirs(os.path.join(self.tmp, '.claude'), exist_ok=True)

        old_cwd = os.getcwd()
        try:
            os.chdir(self.tmp)
            _run(argparse.Namespace())
        finally:
            os.chdir(old_cwd)

        # Verificar se hooks de atenção e scripts foram criados
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, '.claude', 'settings.json')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, 'scripts', 'trackfw-attention-signal.sh')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, 'scripts', 'trackfw-attention-cleanup.sh')))
        self.assertTrue(os.path.isfile(os.path.join(self.tmp, 'scripts', 'trackfw-credential-guard.sh')))

    def test_update_command_upgrade_scenario_backfills_credential_guard(self):
        """
        Cenário de upgrade: projeto que já rodou `trackfw init`/`update` ANTES desta
        REQ tem scripts/trackfw-attention-signal.sh mas ainda não tem
        scripts/trackfw-credential-guard.sh. `trackfw update` deve gerar o script
        que falta, sem quebrar nada existente.
        """
        from trackfw.commands.update import _run
        import argparse
        import os

        with open(os.path.join(self.tmp, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
            f.write('backend: python\nroadmap_dir: docs/roadmaps\n')
        os.makedirs(os.path.join(self.tmp, '.claude'), exist_ok=True)
        os.makedirs(os.path.join(self.tmp, 'scripts'), exist_ok=True)

        signal_path = os.path.join(self.tmp, 'scripts', 'trackfw-attention-signal.sh')
        with open(signal_path, 'w', encoding='utf-8') as f:
            f.write('#!/usr/bin/env bash\necho "old signal script"\n')
        os.chmod(signal_path, 0o755)

        guard_path = os.path.join(self.tmp, 'scripts', 'trackfw-credential-guard.sh')
        self.assertFalse(os.path.isfile(guard_path), 'pré-condição do teste: credential-guard não deve existir ainda')

        old_cwd = os.getcwd()
        try:
            os.chdir(self.tmp)
            _run(argparse.Namespace())
        finally:
            os.chdir(old_cwd)

        self.assertTrue(os.path.isfile(guard_path), 'update não gerou o script de credential guard faltante')
        if os.name == 'posix':
            self.assertTrue(os.stat(guard_path).st_mode & 0o111 != 0, 'credential guard script não é executável')
        # attention-signal.sh preexistente continua presente e não foi removido
        self.assertTrue(os.path.isfile(signal_path))


class TestGenerateClaudeCommands(unittest.TestCase):
    """Verifica geração dos slash commands do Claude, em especial architect.md."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_generate_claude_commands_creates_architect_md(self):
        from trackfw.generators.init_gen import generate_claude_commands

        generate_claude_commands(self.tmp)

        cmd_dir = os.path.join(self.tmp, '.claude', 'commands', 'trackfw')
        self.assertTrue(os.path.isdir(cmd_dir), '.claude/commands/trackfw não foi criado')

        architect_file = os.path.join(cmd_dir, 'architect.md')
        self.assertTrue(os.path.isfile(architect_file), 'architect.md não foi criado')

        with open(architect_file, encoding='utf-8') as f:
            content = f.read()

        self.assertIn('guia de arquitetura do trackfw', content)
        self.assertIn('Passo 1 — Descoberta de Negócio', content)
        self.assertIn('Passo 2 — Recomendação de Stack', content)
        self.assertIn('Passo 3 — Arquitetura em Camadas', content)
        self.assertIn('Passo 4 — Gerar o ADR de Stack', content)
        self.assertIn('Passo 5 — Próximos Passos', content)

    def test_generate_claude_commands_creates_barrier_md(self):
        from trackfw.generators.init_gen import generate_claude_commands

        generate_claude_commands(self.tmp)

        cmd_dir = os.path.join(self.tmp, '.claude', 'commands', 'trackfw')
        barrier_file = os.path.join(cmd_dir, 'barrier.md')
        self.assertTrue(os.path.isfile(barrier_file), 'barrier.md não foi criado')

        with open(barrier_file, encoding='utf-8') as f:
            content = f.read()

        self.assertIn('trackfw_architect', content)
        self.assertIn('trackfw barrier <roadmap> --wave <n> --trust-local-gates --json', content)
        self.assertIn('Todos os MLs da wave concluídos e marcados', content)
        self.assertIn('Agente code-quality reportou', content)
        self.assertIn('Agente security reportou', content)
        self.assertIn('Resultado registrado antes de liberar a próxima wave', content)

    def test_slash_roadmap_command_requires_canonical_frontmatter(self):
        from trackfw.generators.init_gen import generate_claude_commands

        generate_claude_commands(self.tmp)

        roadmap_file = os.path.join(self.tmp, '.claude', 'commands', 'trackfw', 'roadmap.md')
        with open(roadmap_file, encoding='utf-8') as f:
            content = f.read()

        required = [
            '```markdown\n   ---',
            'status: backlog',
            'date: <YYYY-MM-DD>',
            'req: "docs/req/<arquivo-selecionado>.md"',
            'squad: ""',
            '---\n\n   # Roadmap:',
            '> Created: <YYYY-MM-DD> | Status: backlog',
            'docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md',
            'Preencha `req:` com o caminho relativo completo da REQ selecionada',
            '### ML-1B — <título> (se independente de ML-1A)',
            '## Wave 2 — <nome> (depende de Wave 1)',
            '> Dependências: Wave 1 completa',
        ]
        for snippet in required:
            self.assertIn(snippet, content, f"roadmap.md deveria conter trecho canônico: {snippet}")

        versioned = Path(__file__).resolve().parents[2] / '.claude' / 'commands' / 'trackfw' / 'roadmap.md'
        self.assertEqual(content, versioned.read_text(encoding='utf-8'))

    def test_scaffold_creates_all_slash_commands(self):
        opts = {'project_name': 'test-proj', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)

        cmd_dir = os.path.join(self.tmp, '.claude', 'commands', 'trackfw')
        expected_commands = [
            'adr.md', 'req.md', 'validate.md', 'status.md',
            'move.md', 'roadmap.md', 'implement.md', 'architect.md', 'barrier.md'
        ]
        for cmd in expected_commands:
            cmd_path = os.path.join(cmd_dir, cmd)
            self.assertTrue(os.path.isfile(cmd_path), f'Slash command {cmd} não foi criado')

    def test_rules_block_contains_architecture_directives(self):
        from trackfw.generators.init_gen import _trackfw_rules_block, GLOBAL_ADR_DIRECTIVE

        block = _trackfw_rules_block()
        self.assertIn('### Architecture Directives (mandatory)', block)
        self.assertIn('3-layer separation:', block)
        self.assertIn('No in-memory data:', block)
        self.assertIn('Auth from day 1:', block)
        self.assertIn('Docker + .env from day 1:', block)
        self.assertIn('2-layer validation:', block)
        self.assertIn('API-first:', block)
        self.assertIn('Threat model waves:', block)
        self.assertIn('Test coverage:', block)
        self.assertIn('Use `/trackfw:architect` to define stack before the first REQ', block)
        self.assertIn(GLOBAL_ADR_DIRECTIVE, block)

    def test_global_adr_directive_constant(self):
        from trackfw.generators.init_gen import GLOBAL_ADR_DIRECTIVE
        self.assertIn("Obrigatório: Inspecione e respeite todos os ADRs globais", GLOBAL_ADR_DIRECTIVE)


class TestAttentionScriptsExecutionAndHardening(unittest.TestCase):
    """Testes de contrato e hardening dos scripts shell de atenção (C1, C4, C5)."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        opts = {'project_name': 'test-proj', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)
        self.signal_script = os.path.join(self.tmp, 'scripts', 'trackfw-attention-signal.sh')
        self.cleanup_script = os.path.join(self.tmp, 'scripts', 'trackfw-attention-cleanup.sh')

    def test_script_execution_without_roadmap_dir_in_yaml(self):
        """(C1) Script executa com sucesso quando trackfw.yaml não possui roadmap_dir:."""
        yaml_path = os.path.join(self.tmp, 'trackfw.yaml')
        with open(yaml_path, 'w', encoding='utf-8') as f:
            f.write("backend: python\n")

        payload = json.dumps({
            "tool_name": "bash",
            "tool_input": {"command": "echo hello"}
        })

        res = subprocess.run(
            [self.signal_script],
            input=payload,
            capture_output=True,
            text=True,
            cwd=self.tmp,
            shell=False
        )
        self.assertEqual(res.returncode, 0, f"Signal script falhou com stdout: {res.stdout}, stderr: {res.stderr}")

        attention_file = os.path.join(self.tmp, 'docs', 'roadmaps', '.trackfw-attention.json')
        self.assertTrue(os.path.isfile(attention_file), "Attention file não foi criado no fallback docs/roadmaps")

        res_clean = subprocess.run(
            [self.cleanup_script],
            capture_output=True,
            text=True,
            cwd=self.tmp,
            shell=False
        )
        self.assertEqual(res_clean.returncode, 0)
        self.assertFalse(os.path.exists(attention_file), "Attention file não foi removido pelo cleanup script")

    def test_path_containment_in_roadmap_dir(self):
        """(C4) roadmap_dir apontando para fora do CWD é contido ao fallback docs/roadmaps."""
        yaml_path = os.path.join(self.tmp, 'trackfw.yaml')
        with open(yaml_path, 'w', encoding='utf-8') as f:
            f.write("roadmap_dir: ../../outside\n")

        payload = json.dumps({
            "tool_name": "test_tool",
            "tool_input": {"question": "Are you sure?"}
        })

        res = subprocess.run(
            [self.signal_script],
            input=payload,
            capture_output=True,
            text=True,
            cwd=self.tmp,
            shell=False
        )
        self.assertEqual(res.returncode, 0)

        outside_file = os.path.abspath(os.path.join(self.tmp, '..', '..', 'outside', '.trackfw-attention.json'))
        self.assertFalse(os.path.exists(outside_file), "Escreveu fora do CWD em roadmap_dir malicioso")

        inside_file = os.path.join(self.tmp, 'docs', 'roadmaps', '.trackfw-attention.json')
        self.assertTrue(os.path.isfile(inside_file), "Arquivo de atenção não foi salvo no fallback contido")

    def test_json_escaping_with_quotes_slashes_and_newlines(self):
        """(C5) tool_name/message contendo aspas, barras invertidas e newlines gera JSON válido."""
        payload = json.dumps({
            "tool_name": 'tool\\"name\\foo',
            "tool_input": {"question": "Line 1\nLine 2 \"quoted\" \\slash\\"}
        })

        res = subprocess.run(
            [self.signal_script],
            input=payload,
            capture_output=True,
            text=True,
            cwd=self.tmp,
            shell=False
        )
        self.assertEqual(res.returncode, 0)

        attention_file = os.path.join(self.tmp, 'docs', 'roadmaps', '.trackfw-attention.json')
        self.assertTrue(os.path.isfile(attention_file))

        with open(attention_file, 'r', encoding='utf-8') as f:
            data = json.load(f)

        self.assertIn("Line1Line2", data["message"].replace(" ", ""))
        self.assertEqual(data["level"], "action_required")
        self.assertIn("timestamp", data)

    def test_control_character_sanitization(self):
        """(Q2+Q4) Caracteres de controle (U+0000-U+001F) são sanitizados do JSON."""
        payload = json.dumps({
            "tool_name": "tool\u0007name\u001f",
            "tool_input": {"question": "Hello\u0000\tWorld\r\nTest"}
        })

        res = subprocess.run(
            [self.signal_script],
            input=payload,
            capture_output=True,
            text=True,
            cwd=self.tmp,
            shell=False
        )
        self.assertEqual(res.returncode, 0)

        attention_file = os.path.join(self.tmp, 'docs', 'roadmaps', '.trackfw-attention.json')
        self.assertTrue(os.path.isfile(attention_file))

        with open(attention_file, 'r', encoding='utf-8') as f:
            data = json.load(f)

        self.assertEqual(data["tool"], "toolname")
        self.assertEqual(data["message"], "HelloWorldTest")

    def test_tolerant_roadmap_dir_parsing_with_comments(self):
        """(Q6) roadmap_dir com comentários inline e/ou sem espaço é parseado corretamente."""
        yaml_path = os.path.join(self.tmp, 'trackfw.yaml')
        with open(yaml_path, 'w', encoding='utf-8') as f:
            f.write("roadmap_dir:custom/roadmaps # comentario inline\n")

        payload = json.dumps({
            "tool_name": "bash",
            "tool_input": {"command": "echo test"}
        })

        res = subprocess.run(
            [self.signal_script],
            input=payload,
            capture_output=True,
            text=True,
            cwd=self.tmp,
            shell=False
        )
        self.assertEqual(res.returncode, 0)

        custom_file = os.path.join(self.tmp, 'custom', 'roadmaps', '.trackfw-attention.json')
        self.assertTrue(os.path.isfile(custom_file), "Attention file não foi criado no roadmap_dir customizado parseado")

    def test_fallback_without_jq(self):
        """(Q5) Execução do signal script sem jq no PATH utiliza o fallback python3 e gera JSON válido."""
        env = dict(os.environ)
        path_dirs = env.get("PATH", "").split(os.path.pathsep)

        fake_bin = tempfile.mkdtemp()
        filtered_dirs = []

        for d in path_dirs:
            if not os.path.isdir(d):
                continue
            if os.path.exists(os.path.join(d, "jq")) or os.path.exists(os.path.join(d, "jq.exe")):
                for item in os.listdir(d):
                    if item in ("jq", "jq.exe"):
                        continue
                    src = os.path.join(d, item)
                    dst = os.path.join(fake_bin, item)
                    if not os.path.exists(dst):
                        try:
                            os.symlink(src, dst)
                        except OSError:
                            pass
            else:
                filtered_dirs.append(d)

        filtered_dirs.insert(0, fake_bin)

        if not any(os.path.exists(os.path.join(d, "python3")) for d in filtered_dirs):
            py_bin = os.path.join(fake_bin, "python3")
            if not os.path.exists(py_bin):
                try:
                    os.symlink(sys.executable, py_bin)
                except OSError:
                    pass

        env["PATH"] = os.path.pathsep.join(filtered_dirs)

        payload = json.dumps({
            "tool_name": "bash",
            "tool_input": {"command": "echo fallback_no_jq"}
        })

        res = subprocess.run(
            [self.signal_script],
            input=payload,
            capture_output=True,
            text=True,
            cwd=self.tmp,
            env=env,
            shell=False
        )
        self.assertEqual(res.returncode, 0, f"Signal script falhou no fallback sem jq. Stderr: {res.stderr}")

        attention_file = os.path.join(self.tmp, 'docs', 'roadmaps', '.trackfw-attention.json')
        self.assertTrue(os.path.isfile(attention_file), "Attention file não foi gerado no fallback sem jq")

        with open(attention_file, 'r', encoding='utf-8') as f:
            data = json.load(f)

        self.assertEqual(data.get("tool"), "bash")
        self.assertEqual(data.get("message"), "echo fallback_no_jq")
        self.assertEqual(data.get("level"), "action_required")
        self.assertIn("timestamp", data)


class TestWindsurfHooks(unittest.TestCase):
    """Testes para injeção de hooks no CLI Windsurf (C8)."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_inject_windsurf_hooks_updates_rules(self):
        from trackfw.generators.hooks import inject_windsurf_hooks

        windsurfrules = os.path.join(self.tmp, '.windsurfrules')
        with open(windsurfrules, 'w', encoding='utf-8') as f:
            f.write("# Existing rules\n")

        inject_windsurf_hooks(self.tmp)

        with open(windsurfrules, 'r', encoding='utf-8') as f:
            content = f.read()

        self.assertIn("<!-- trackfw:rules:start -->", content)
        self.assertIn("Governance Rules", content)

    def test_inject_hooks_detected_detects_windsurf(self):
        from trackfw.generators.hooks import inject_hooks_detected

        windsurfrules = os.path.join(self.tmp, '.windsurfrules')
        with open(windsurfrules, 'w', encoding='utf-8') as f:
            f.write("# Rules\n")

        inject_hooks_detected(self.tmp)

        with open(windsurfrules, 'r', encoding='utf-8') as f:
            content = f.read()

        self.assertIn("trackfw — Governance Rules", content)


if __name__ == '__main__':
    unittest.main()
