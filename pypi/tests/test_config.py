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


class TestConfigEvolution(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _write_yaml(self, content):
        path = os.path.join(self.tmpdir, "trackfw.yaml")
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)

    def test_defaults_novos_campos(self):
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["link_fields"]["req"], ["REQ:"])
        self.assertEqual(cfg["link_fields"]["adr"], ["ADR:"])
        self.assertEqual(cfg["link_fields"]["roadmap"], ["Roadmap:"])
        self.assertEqual(cfg["acceptance_markers"], ["## Acceptance Criteria", "## Critérios de Aceite"])
        self.assertEqual(cfg["rules"]["wip_has_req"], "error")
        self.assertEqual(cfg["rules"]["stale_wip"], "warning")

    def test_link_fields_customizado(self):
        self._write_yaml(
            "link_fields:\n"
            "  req:\n"
            '    - "REQ:"\n'
            "    - req_id\n"
            "  adr:\n"
            '    - "ADR:"\n'
            "  roadmap:\n"
            '    - "Roadmap:"\n'
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["link_fields"]["req"], ["REQ:", "req_id"])
        self.assertEqual(cfg["link_fields"]["adr"], ["ADR:"])
        self.assertEqual(cfg["link_fields"]["roadmap"], ["Roadmap:"])

    def test_acceptance_markers_customizado(self):
        self._write_yaml(
            "acceptance_markers:\n"
            '  - "## Done"\n'
            '  - "## Concluído"\n'
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["acceptance_markers"], ["## Done", "## Concluído"])

    def test_rules_parcial_merge_com_defaults(self):
        self._write_yaml(
            "rules:\n"
            "  stale_wip: error\n"
            "  adr_orphan: off\n"
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["rules"]["stale_wip"], "error")
        self.assertEqual(cfg["rules"]["adr_orphan"], "off")
        self.assertEqual(cfg["rules"]["wip_has_req"], "error")  # default mantido

    def test_sparse_novos_campos_usam_defaults(self):
        self._write_yaml("wip_limit: 3\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["wip_limit"], 3)
        self.assertEqual(cfg["link_fields"]["req"], ["REQ:"])
        self.assertEqual(cfg["rules"]["wip_has_req"], "error")

    def test_retrocompat_yaml_v23(self):
        self._write_yaml(
            "adr_dirs:\n"
            "  - docs/adr\n"
            "wip_limit: 2\n"
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["adr_dirs"], ["docs/adr"])
        self.assertEqual(cfg["wip_limit"], 2)
        self.assertEqual(cfg["link_fields"]["req"], ["REQ:"])  # default

    def test_rules_value_with_double_quotes(self):
        """Valores de rules com aspas duplas devem ser armazenados sem aspas."""
        self._write_yaml(
            "rules:\n"
            '  adr_orphan: "off"\n'
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["rules"]["adr_orphan"], "off")

    def test_rules_value_with_single_quotes(self):
        """Valores de rules com aspas simples devem ser armazenados sem aspas."""
        self._write_yaml(
            "rules:\n"
            "  stale_wip: 'warning'\n"
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["rules"]["stale_wip"], "warning")


class TestConfigPaths(unittest.TestCase):
    """Testes ML-2C: paths configuráveis adr_dirs, req_dir, roadmap_dir."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _write_yaml(self, content):
        path = os.path.join(self.tmpdir, "trackfw.yaml")
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)

    def test_config_adr_dirs_list(self):
        """adr_dirs com dois itens → lista correta."""
        self._write_yaml(
            "adr_dirs:\n"
            "  - docs/adr/zeus\n"
            "  - docs/adr/apolo\n"
        )
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["adr_dirs"], ["docs/adr/zeus", "docs/adr/apolo"])
        self.assertIsInstance(cfg["adr_dirs"], list)
        self.assertEqual(len(cfg["adr_dirs"]), 2)

    def test_config_req_dir_custom(self):
        """req_dir: docs/requisições → valor UTF-8 correto."""
        self._write_yaml("req_dir: docs/requisições\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["req_dir"], "docs/requisições")

    def test_config_roadmap_dir_custom(self):
        """roadmap_dir: docs/rm → valor correto."""
        self._write_yaml("roadmap_dir: docs/rm\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["roadmap_dir"], "docs/rm")

    def test_config_paths_defaults(self):
        """Sem campos de path no yaml → defaults corretos."""
        self._write_yaml("wip_limit: 2\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["adr_dirs"], ["docs/adr"])
        self.assertEqual(cfg["req_dir"], "docs/req")
        self.assertEqual(cfg["roadmap_dir"], "docs/roadmaps")


if __name__ == "__main__":
    unittest.main()
