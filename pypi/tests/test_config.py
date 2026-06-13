"""
Testes de unidade para pypi/trackfw/config.py
Usa unittest (stdlib) — sem dependências externas.
"""

import os
import sys
import tempfile
import shutil
import unittest

# Garante que o pacote pypi/trackfw seja importável
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config


class TestConfig(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    # ------------------------------------------------------------------
    # test_defaults_sem_yaml
    # ------------------------------------------------------------------
    def test_defaults_sem_yaml(self):
        """load() em dir sem trackfw.yaml retorna valores padrão."""
        cfg = config.load(cwd=self.tmpdir)
        expected = config.defaults()
        self.assertEqual(cfg, expected)

    # ------------------------------------------------------------------
    # test_lê_campos_escalares
    # ------------------------------------------------------------------
    def test_le_campos_escalares(self):
        """load() com yaml contendo campos escalares retorna valores corretos."""
        yaml_content = (
            "req_dir: docs/requisições\n"
            "roadmap_dir: docs/roadmaps/custom\n"
            "roadmap_namespacing: by_agent\n"
            "governance_mode: strict\n"
            "lenient_until: 2026-12-31\n"
            "wip_limit: 3\n"
            "wip_by_squad: true\n"
            "require_req_in_commit: true\n"
        )
        yaml_path = os.path.join(self.tmpdir, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)

        cfg = config.load(cwd=self.tmpdir)

        self.assertEqual(cfg["req_dir"], "docs/requisições")
        self.assertEqual(cfg["roadmap_dir"], "docs/roadmaps/custom")
        self.assertEqual(cfg["roadmap_namespacing"], "by_agent")
        self.assertEqual(cfg["governance_mode"], "strict")
        self.assertEqual(cfg["lenient_until"], "2026-12-31")
        self.assertEqual(cfg["wip_limit"], 3)
        self.assertTrue(cfg["wip_by_squad"])
        self.assertTrue(cfg["require_req_in_commit"])

    # ------------------------------------------------------------------
    # test_lê_adr_dirs
    # ------------------------------------------------------------------
    def test_le_adr_dirs(self):
        """load() com lista adr_dirs no yaml faz parse correto."""
        yaml_content = (
            "adr_dirs:\n"
            "  - docs/adr/zeus\n"
            "  - docs/adr/apolo\n"
        )
        yaml_path = os.path.join(self.tmpdir, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)

        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["adr_dirs"], ["docs/adr/zeus", "docs/adr/apolo"])

    # ------------------------------------------------------------------
    # test_singleton
    # ------------------------------------------------------------------
    def test_singleton(self):
        """Duas chamadas a load() retornam o mesmo objeto."""
        cfg1 = config.load(cwd=self.tmpdir)
        cfg2 = config.load(cwd=self.tmpdir)
        self.assertIs(cfg1, cfg2)

    # ------------------------------------------------------------------
    # test_reset
    # ------------------------------------------------------------------
    def test_reset(self):
        """Após reset(), load() relê o arquivo (novo objeto)."""
        cfg1 = config.load(cwd=self.tmpdir)
        config.reset()

        # Cria yaml com valor diferente após reset
        yaml_path = os.path.join(self.tmpdir, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write("wip_limit: 5\n")

        cfg2 = config.load(cwd=self.tmpdir)
        self.assertIsNot(cfg1, cfg2)
        self.assertEqual(cfg2["wip_limit"], 5)


if __name__ == "__main__":
    unittest.main()
