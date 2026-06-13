"""
Testes unitários para pypi/trackfw/generators/req.py
"""

import os
import tempfile
import unittest
from datetime import date

from trackfw.generators.req import generate_req, slugify


class TestSlugify(unittest.TestCase):
    def test_slugify_com_acentos(self):
        """'Minha Requisição' deve gerar 'minha-requisicao'."""
        result = slugify("Minha Requisição")
        self.assertEqual(result, "minha-requisicao")

    def test_slugify_lowercase(self):
        result = slugify("Feature Nova")
        self.assertEqual(result, "feature-nova")

    def test_slugify_sem_acentos(self):
        result = slugify("autenticacao")
        self.assertEqual(result, "autenticacao")


class TestGenerateReq(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.req_dir = os.path.join(self.tmpdir, "docs", "req")

    def test_generate_req_cria_arquivo(self):
        """Arquivo criado com nome correto (REQ-YYYY-MM-DD-<slug>.md)."""
        path = generate_req("Minha Feature", req_dir=self.req_dir)
        today = date.today().isoformat()
        expected_filename = f"REQ-{today}-minha-feature.md"
        self.assertTrue(os.path.isfile(path))
        self.assertEqual(os.path.basename(path), expected_filename)

    def test_frontmatter_correto(self):
        """Frontmatter contém status: Open e linked_adr: —."""
        path = generate_req("Teste Frontmatter", req_dir=self.req_dir)
        with open(path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: Open", content)
        self.assertIn("linked_adr: —", content)

    def test_cria_req_dir_se_nao_existir(self):
        """req_dir inexistente é criado automaticamente."""
        novo_dir = os.path.join(self.tmpdir, "novo", "subdir", "req")
        self.assertFalse(os.path.exists(novo_dir))
        path = generate_req("Criar Dir", req_dir=novo_dir)
        self.assertTrue(os.path.isdir(novo_dir))
        self.assertTrue(os.path.isfile(path))

    def test_retorna_path_absoluto(self):
        """generate_req deve retornar o path absoluto do arquivo criado."""
        path = generate_req("Path Test", req_dir=self.req_dir)
        self.assertTrue(os.path.isabs(path))

    def test_conteudo_template(self):
        """Arquivo gerado contém as seções obrigatórias do template."""
        path = generate_req("Seções Obrigatórias", req_dir=self.req_dir)
        with open(path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("## Motivação", content)
        self.assertIn("## Critérios de Aceite", content)
        self.assertIn("## Fora de Escopo", content)
        self.assertIn("# REQ: Seções Obrigatórias", content)


if __name__ == "__main__":
    unittest.main()
