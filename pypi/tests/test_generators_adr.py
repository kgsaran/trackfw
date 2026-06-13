"""
Testes unitários para pypi/trackfw/generators/adr.py
"""

import os
import re
import tempfile
import unittest

from trackfw.generators.adr import next_adr_number, slugify, generate_adr


class TestNextAdrNumber(unittest.TestCase):

    def test_next_number_dir_vazio(self):
        """Diretório vazio deve retornar 1."""
        with tempfile.TemporaryDirectory() as tmpdir:
            result = next_adr_number(tmpdir)
            self.assertEqual(result, 1)

    def test_next_number_com_arquivos(self):
        """Diretório com ADR-001 e ADR-003 deve retornar 4."""
        with tempfile.TemporaryDirectory() as tmpdir:
            # Cria arquivos simulando ADRs existentes
            open(os.path.join(tmpdir, 'ADR-001-primeiro.md'), 'w').close()
            open(os.path.join(tmpdir, 'ADR-003-terceiro.md'), 'w').close()
            result = next_adr_number(tmpdir)
            self.assertEqual(result, 4)

    def test_next_number_dir_inexistente(self):
        """Diretório inexistente deve retornar 1."""
        result = next_adr_number('/tmp/trackfw-dir-que-nao-existe-xyz')
        self.assertEqual(result, 1)

    def test_next_number_ignora_arquivos_nao_adr(self):
        """Arquivos não-ADR no diretório não devem influenciar a numeração."""
        with tempfile.TemporaryDirectory() as tmpdir:
            open(os.path.join(tmpdir, 'README.md'), 'w').close()
            open(os.path.join(tmpdir, 'ADR-002-segundo.md'), 'w').close()
            result = next_adr_number(tmpdir)
            self.assertEqual(result, 3)


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
        """generate_adr deve criar o arquivo com nome e frontmatter corretos."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            filepath = generate_adr(
                title='Minha Decisão Técnica',
                status='Draft',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )

            # Arquivo deve existir
            self.assertTrue(os.path.isfile(filepath))

            # Nome do arquivo deve conter o número e o slug
            basename = os.path.basename(filepath)
            self.assertRegex(basename, r'^ADR-001-minha-decisao-tecnica\.md$')

            # Conteúdo deve ter frontmatter
            with open(filepath, encoding='utf-8') as f:
                content = f.read()

            self.assertIn('name: ADR-001-minha-decisao-tecnica', content)
            self.assertIn('title: "Minha Decisão Técnica"', content)
            self.assertIn('status: Draft', content)
            self.assertIn('## Status', content)
            self.assertIn('## Context', content)
            self.assertIn('## Decision', content)
            self.assertIn('## Consequences', content)

    def test_generate_adr_numero_sequencial(self):
        """Dois ADRs gerados no mesmo diretório devem ter números 1 e 2."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')

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

            self.assertIn('ADR-001', name1)
            self.assertIn('ADR-002', name2)

    def test_generate_adr_status_padrao_draft(self):
        """Status padrão deve ser 'Draft'."""
        with tempfile.TemporaryDirectory() as tmpdir:
            adr_dir = os.path.join(tmpdir, 'docs', 'adr')
            filepath = generate_adr(
                title='Decisão Sem Status',
                adr_dirs=[adr_dir],
                cwd=tmpdir,
            )
            with open(filepath, encoding='utf-8') as f:
                content = f.read()
            self.assertIn('status: Draft', content)

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
