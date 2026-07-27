"""
Testes unitários para pypi/trackfw/generators/adr.py
Formato canônico Go/Node — REQ-2026-07-27-convergencia-templates-python.
"""

import os
import re
import tempfile
import unittest
from datetime import date

from trackfw.generators.adr import slugify, generate_adr


class TestSlugify(unittest.TestCase):

    def test_slugify_acento(self):
        """Acentos devem ser removidos e espaços viram hifens."""
        result = slugify('Minha Decisão Técnica')
        self.assertEqual(result, 'minha-decisao-tecnica')

    def test_slugify_simples(self):
        result = slugify('Authentication Strategy')
        self.assertEqual(result, 'authentication-strategy')

    def test_slugify_caracteres_especiais(self):
        """Caracteres não-alfanuméricos exceto hífen devem ser removidos."""
        result = slugify('My Decision (v2)!')
        self.assertEqual(result, 'my-decision-v2')

    def test_slugify_lowercase(self):
        result = slugify('ALL CAPS TITLE')
        self.assertEqual(result, 'all-caps-title')

    def test_slugify_hifens_multiplos(self):
        """Hifens múltiplos consecutivos devem ser colapsados."""
        result = slugify('foo  bar')
        self.assertEqual(result, 'foo-bar')


class TestGenerateAdr(unittest.TestCase):

    def test_generate_adr_cria_arquivo(self):
        """generate_adr deve criar o arquivo com nome baseado em data e frontmatter canônico."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            filepath = generate_adr(
                title='Minha Decisão Técnica',
                status='Draft',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )

            self.assertTrue(os.path.isfile(filepath))

            # Nome do arquivo: ADR-YYYY-MM-DD-<slug>.md
            basename = os.path.basename(filepath)
            self.assertRegex(basename, r'^ADR-\d{4}-\d{2}-\d{2}-minha-decisao-tecnica\.md$')

            with open(filepath, encoding='utf-8') as f:
                content = f.read()

            today = date.today().isoformat()
            # Frontmatter canônico
            self.assertIn('status: Draft', content)
            self.assertIn(f'date: {today}', content)
            self.assertIn('author: ""', content)
            # Header inline
            self.assertIn(f'> Date: {today} | Status: Draft', content)
            # H1 canônico
            self.assertIn('# ADR: Minha Decisão Técnica', content)
            # Seções canônicas
            self.assertIn('## Context', content)
            self.assertIn('## Decision', content)
            self.assertIn('## Consequences', content)
            self.assertIn('## Alternatives Considered', content)

    def test_generate_adr_nomes_com_data(self):
        """Dois ADRs gerados no mesmo diretório devem ter nomes baseados em data."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            today = date.today().isoformat()

            path1 = generate_adr(
                title='Primeira Decisão',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            path2 = generate_adr(
                title='Segunda Decisão',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )

            name1 = os.path.basename(path1)
            name2 = os.path.basename(path2)

            # Ambos devem conter a data atual no nome
            self.assertIn(today, name1)
            self.assertIn(today, name2)
            # Slugs distintos
            self.assertIn('primeira-decisao', name1)
            self.assertIn('segunda-decisao', name2)

    def test_generate_adr_status_padrao_proposed(self):
        """Status padrão deve ser 'Proposed' (canônico Go/Node)."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            filepath = generate_adr(
                title='Decisão Sem Status',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            with open(filepath, encoding='utf-8') as f:
                content = f.read()
            self.assertIn('status: Proposed', content)

    def test_generate_adr_cria_dir_se_inexistente(self):
        """O diretório de ADRs deve ser criado automaticamente."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr', 'subdir')
            filepath = generate_adr(
                title='Test Dir Creation',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            self.assertTrue(os.path.isfile(filepath))


if __name__ == '__main__':
    unittest.main()
