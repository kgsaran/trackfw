"""
test_generators_roadmap.py — Testes unitários para generators/roadmap.py
"""

import os
import datetime
import tempfile
import unittest

from trackfw import config as cfg_module
from trackfw.generators.roadmap import (
    slugify,
    generate_roadmap,
    move_roadmap,
    VALID_STATES,
)


def _make_cfg(tmpdir: str, namespacing: str = "flat", agents=None) -> dict:
    """Cria config mínimo apontando para tmpdir."""
    cfg = cfg_module.defaults()
    cfg["roadmap_dir"] = os.path.join(tmpdir, "docs", "roadmaps")
    cfg["roadmap_namespacing"] = namespacing
    if agents is not None:
        cfg["agents"] = agents
    return cfg


class TestSlugify(unittest.TestCase):
    def test_lowercase(self):
        self.assertEqual(slugify("Hello World"), "hello-world")

    def test_special_chars(self):
        self.assertEqual(slugify("Feature: Auth & Login"), "feature-auth-login")

    def test_leading_trailing_hyphens(self):
        self.assertEqual(slugify("--test--"), "test")


class TestGenerateFlat(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_generate_flat(self):
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Minha Feature", cfg)

        self.assertTrue(os.path.isfile(path))

        # Deve estar em roadmap_dir/backlog/
        backlog_dir = os.path.join(cfg["roadmap_dir"], "backlog")
        self.assertEqual(os.path.dirname(path), backlog_dir)

        # Nome do arquivo contém slug e data
        basename = os.path.basename(path)
        today = datetime.date.today().isoformat()
        self.assertIn(today, basename)
        self.assertIn("minha-feature", basename)
        self.assertTrue(basename.endswith(".md"))

        # Conteúdo contém frontmatter e seção de wave
        with open(path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: Backlog", content)
        self.assertIn("# Roadmap: Minha Feature", content)
        self.assertIn("## Wave 1", content)
        self.assertIn("ML-1A", content)


class TestGenerateByAgent(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_generate_by_agent(self):
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        path = generate_roadmap("Auth Redesign", cfg, agent="zeus")

        self.assertTrue(os.path.isfile(path))

        # Deve estar em roadmap_dir/zeus/backlog/
        expected_dir = os.path.join(cfg["roadmap_dir"], "zeus", "backlog")
        self.assertEqual(os.path.dirname(path), expected_dir)

    def test_generate_by_agent_usa_primeiro_agente_configurado(self):
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["apolo", "zeus"])
        path = generate_roadmap("API Gateway", cfg)

        # Sem agent explícito, usa o primeiro da lista
        expected_dir = os.path.join(cfg["roadmap_dir"], "apolo", "backlog")
        self.assertEqual(os.path.dirname(path), expected_dir)


class TestMoveBacklogParaWip(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_move_backlog_para_wip(self):
        cfg = _make_cfg(self.tmpdir)

        # Cria roadmap em backlog
        src_path = generate_roadmap("Deploy Pipeline", cfg)
        basename = os.path.basename(src_path)

        # Move para wip
        dst_path = move_roadmap(basename, "wip", cfg)

        # Arquivo de destino existe
        self.assertTrue(os.path.isfile(dst_path))
        # Arquivo de origem não existe mais
        self.assertFalse(os.path.isfile(src_path))

        # Está em wip/
        wip_dir = os.path.join(cfg["roadmap_dir"], "wip")
        self.assertEqual(os.path.dirname(dst_path), wip_dir)

        # Frontmatter atualizado
        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: WIP", content)

    def test_move_estado_invalido_levanta_exception(self):
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("X", cfg)
        basename = os.path.basename(src_path)

        with self.assertRaises(ValueError):
            move_roadmap(basename, "inexistente", cfg)

    def test_move_arquivo_nao_encontrado_levanta_exception(self):
        cfg = _make_cfg(self.tmpdir)

        with self.assertRaises(FileNotFoundError):
            move_roadmap("nao-existe.md", "wip", cfg)

    def test_log_gravado_apos_move(self):
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("Log Test", cfg)
        basename = os.path.basename(src_path)

        move_roadmap(basename, "done", cfg)

        log_path = os.path.join(cfg["roadmap_dir"], ".trackfw-log")
        self.assertTrue(os.path.isfile(log_path))
        with open(log_path, encoding="utf-8") as f:
            log_content = f.read()
        self.assertIn("backlog", log_content)
        self.assertIn("done", log_content)
        self.assertIn(basename, log_content)


class TestMoveBuscaEmTodosAgentes(unittest.TestCase):
    """
    Em modo by_agent, move_roadmap deve encontrar o arquivo mesmo sem
    saber em qual agente ele está.
    """

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_move_busca_em_todos_agentes(self):
        cfg = _make_cfg(
            self.tmpdir,
            namespacing="by_agent",
            agents=["zeus", "apolo"],
        )

        # Cria roadmap no agente zeus/backlog
        src_path = generate_roadmap("Infra Refactor", cfg, agent="zeus")
        basename = os.path.basename(src_path)

        # Move para wip sem especificar agente — deve encontrar em zeus/backlog
        dst_path = move_roadmap(basename, "wip", cfg)

        self.assertTrue(os.path.isfile(dst_path))
        self.assertFalse(os.path.isfile(src_path))

        # Deve estar em zeus/wip/ (preserva o agente)
        expected_dir = os.path.join(cfg["roadmap_dir"], "zeus", "wip")
        self.assertEqual(os.path.dirname(dst_path), expected_dir)

        # Frontmatter atualizado
        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: WIP", content)


if __name__ == "__main__":
    unittest.main()
