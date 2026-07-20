"""
tests/test_generators_init.py — testes para generators/init_gen.py
"""

import json
import os
import tempfile
import unittest

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


class TestAttentionScripts(unittest.TestCase):
    """Verifica geração dos scripts de atenção trackfw-attention-signal.sh e cleanup.sh."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_scaffold_generates_attention_scripts(self):
        opts = {'project_name': 'test-proj', 'namespacing': 'flat', 'wip_limit': 1}
        scaffold(self.tmp, opts)

        signal_path = os.path.join(self.tmp, 'scripts', 'trackfw-attention-signal.sh')
        cleanup_path = os.path.join(self.tmp, 'scripts', 'trackfw-attention-cleanup.sh')

        self.assertTrue(os.path.isfile(signal_path), 'trackfw-attention-signal.sh não foi criado')
        self.assertTrue(os.path.isfile(cleanup_path), 'trackfw-attention-cleanup.sh não foi criado')

        # Permissão de execução no Unix
        if os.name == 'posix':
            self.assertTrue(os.stat(signal_path).st_mode & 0o111 != 0, 'signal script não é executável')
            self.assertTrue(os.stat(cleanup_path).st_mode & 0o111 != 0, 'cleanup script não é executável')

        with open(signal_path, encoding='utf-8') as f:
            signal_content = f.read()
        self.assertIn('# trackfw attention signal — PreToolUse/BeforeTool hook', signal_content)

        with open(cleanup_path, encoding='utf-8') as f:
            cleanup_content = f.read()
        self.assertIn('# trackfw attention cleanup — PostToolUse/AfterTool hook', cleanup_content)


class TestAttentionHooksInjectors(unittest.TestCase):
    """Testes unitários para injeção idempotente de hooks de atenção nos 7 CLIs."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

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

        # 2. Idempotência
        inject_claude_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['PreToolUse']), 1)
        self.assertEqual(len(data2['hooks']['PostToolUse']), 1)

    def test_inject_codex_hooks_create_and_merge(self):
        from trackfw.generators.hooks import inject_codex_hooks
        inject_codex_hooks(self.tmp)
        path = os.path.join(self.tmp, '.codex', 'hooks.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertIn('PermissionRequest', data.get('hooks', {}))
        self.assertIn('PostToolUse', data.get('hooks', {}))

        # Idempotência
        inject_codex_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['PermissionRequest']), 1)
        self.assertEqual(len(data2['hooks']['PostToolUse']), 1)

    def test_inject_gemini_hooks_create_and_merge(self):
        from trackfw.generators.hooks import inject_gemini_hooks
        inject_gemini_hooks(self.tmp)
        path = os.path.join(self.tmp, '.gemini', 'settings.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertIn('Notification', data.get('hooks', {}))
        self.assertIn('AfterTool', data.get('hooks', {}))

        # Idempotência
        inject_gemini_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['hooks']['Notification']), 1)
        self.assertEqual(len(data2['hooks']['AfterTool']), 1)

    def test_inject_kiro_hooks(self):
        from trackfw.generators.hooks import inject_kiro_hooks
        inject_kiro_hooks(self.tmp)
        path = os.path.join(self.tmp, '.kiro', 'hooks', 'trackfw-attention.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertEqual(len(data.get('hooks', [])), 2)

        # Idempotência
        inject_kiro_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2.get('hooks', [])), 2)

    def test_inject_copilot_hooks(self):
        from trackfw.generators.hooks import inject_copilot_hooks
        inject_copilot_hooks(self.tmp)
        path = os.path.join(self.tmp, '.github', 'hooks', 'trackfw-attention.json')
        self.assertTrue(os.path.isfile(path))
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        self.assertIn('preToolUse', data.get('hooks', {}))
        self.assertIn('postToolUse', data.get('hooks', {}))

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
        self.assertIn('preToolUse', data)
        self.assertIn('postToolUse', data)

        # Idempotência
        inject_cursor_hooks(self.tmp)
        with open(path, 'r', encoding='utf-8') as f:
            data2 = json.load(f)
        self.assertEqual(len(data2['preToolUse']), 1)
        self.assertEqual(len(data2['postToolUse']), 1)

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


if __name__ == '__main__':
    unittest.main()



