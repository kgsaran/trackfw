"""
tests/test_generators_init.py — testes para generators/init_gen.py
"""

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


if __name__ == '__main__':
    unittest.main()
