"""
Testes unitários para pypi/trackfw/generators/req.py
Formato canônico Go/Node — REQ-2026-07-27-convergencia-templates-python.
"""

import os
import tempfile
import unittest
from datetime import date

from trackfw.generators.req import generate_req, move_req, slugify, rewrite_req_status


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

    # Testes de paridade cross-runtime (ML-3B) — mesmos vetores do Go e Node
    def test_slugify_autenticacao_e_sessao(self):
        """Título canônico do gate — ã ç ã."""
        self.assertEqual(slugify("Autenticação e Sessão"), "autenticacao-e-sessao")

    def test_slugify_agudo(self):
        """á é í ó ú → a e i o u."""
        self.assertEqual(slugify("á é í ó ú"), "a-e-i-o-u")

    def test_slugify_cedilha_til_crase(self):
        """ç ã õ à → c a o a."""
        self.assertEqual(slugify("ç ã õ à"), "c-a-o-a")

    def test_slugify_titulo_com_parentese(self):
        """Caracteres não-alfanuméricos são removidos."""
        self.assertEqual(slugify("ADR Config (v2)"), "adr-config-v2")

    def test_slugify_configuracao_avancada(self):
        self.assertEqual(slugify("Configuração Avançada"), "configuracao-avancada")


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
        """Frontmatter canônico: status: Open, date, author, adr, roadmap."""
        path = generate_req("Teste Frontmatter", req_dir=self.req_dir)
        with open(path, encoding="utf-8") as f:
            content = f.read()
        today = date.today().isoformat()
        self.assertIn("status: Open", content)
        self.assertIn(f"date: {today}", content)
        self.assertIn('author: ""', content)
        self.assertIn('adr: ""', content)
        self.assertIn('roadmap: ""', content)

    def test_header_inline_status(self):
        """Header canônico: > Date: <data> | Status: Open."""
        path = generate_req("Header Status", req_dir=self.req_dir)
        with open(path, encoding="utf-8") as f:
            content = f.read()
        today = date.today().isoformat()
        self.assertIn(f"> Date: {today} | Status: Open", content)

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
        """Arquivo gerado contém as seções obrigatórias do template canônico (inglês)."""
        path = generate_req("Mandatory Sections", req_dir=self.req_dir)
        with open(path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("## Motivation", content)
        self.assertIn("## Acceptance Criteria", content)
        self.assertIn("## Linked ADR", content)
        self.assertIn("## Blocked by ADRs", content)
        self.assertIn("## Linked Roadmap", content)
        self.assertIn("# REQ: Mandatory Sections", content)

    def test_move_req_rewrites_status_in_place(self):
        os.makedirs(self.req_dir, exist_ok=True)
        req_path = os.path.join(self.req_dir, "REQ-2026-07-27-fechar.md")
        with open(req_path, "w", encoding="utf-8") as f:
            f.write(
                "---\nstatus: Open\ndate: 2026-07-27\nroadmap: \"docs/roadmaps/done/RM.md\"\n---\n\n"
                "# REQ: Fechar\n\n> Date: 2026-07-27 | Status: Open | Linear Issue: X\n\n"
                "## Notes\nstatus: Open\n| Status: Open\n"
            )

        moved_path = move_req("fechar", "done", req_dir=self.req_dir)

        self.assertEqual(moved_path, req_path)
        with open(req_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: done\n", content)
        self.assertIn("> Date: 2026-07-27 | Status: done | Linear Issue: X", content)
        self.assertIn("## Notes\nstatus: Open\n| Status: Open\n", content)

    def test_rewrite_req_status_crlf_source_produz_resultado_byte_identico_ao_lf(self):
        """ML-5B, falsificação nas duas direções (ADR-2026-09-04-parser-de-frontmatter-
        tolera-crlf-na-fronteira-de-entrada, D1/D3): mesmo defeito do ML-5A, agora no
        2º site de escrita de produto deste CLI: rewrite_req_status. Chamado com CRLF
        DIRETO (bypassando o universal-newlines de open()), que é o que expõe o defeito
        na função em si."""
        lf_src = "---\nstatus: Open\ndate: 2026-07-27\n---\n\n# REQ: X\n\n> Date: 2026-07-27 | Status: Open\n"
        crlf_src = lf_src.replace("\n", "\r\n")

        lf_result, lf_changed = rewrite_req_status(lf_src, "done")
        crlf_result, crlf_changed = rewrite_req_status(crlf_src, "done")

        self.assertTrue(crlf_changed, "CRLF deveria ser reconhecida como frontmatter (D3 não deveria ficar cega)")
        self.assertEqual(lf_result, crlf_result)
        self.assertNotIn("\r", crlf_result)

    def test_move_req_arquivo_crlf_no_disco_ja_e_tolerado_hoje_por_universal_newlines(self):
        """Nuance declarada (mesma do roadmap): move_req lê com
        open(path, "r", encoding="utf-8") — universal newlines já traduz CRLF -> LF nessa
        leitura, então este end-to-end nunca reproduziu o defeito isolado. Mantido como
        controle de integração e de escrita (D2)."""
        os.makedirs(self.req_dir, exist_ok=True)
        req_path = os.path.join(self.req_dir, "REQ-2026-07-27-crlf-disco.md")
        lf_content = (
            "---\nstatus: Open\ndate: 2026-07-27\n---\n\n"
            "# REQ: CRLF Disk\n\n> Date: 2026-07-27 | Status: Open\n"
        )
        with open(req_path, "wb") as f:
            f.write(lf_content.replace("\n", "\r\n").encode("utf-8"))

        moved_path = move_req("crlf-disco", "done", req_dir=self.req_dir)

        with open(moved_path, "rb") as f:
            written = f.read()
        self.assertNotIn(b"\r", written, "escrita deveria ser LF puro (D2), mesmo com fonte CRLF no disco")
        self.assertIn(b"status: done\n", written)
        self.assertIn(b"| Status: done", written)


if __name__ == "__main__":
    unittest.main()
