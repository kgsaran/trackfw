"""
Testes de unidade para os namespaces update/sync de pypi/trackfw/config.py (ML-1A).

AC2/AC3 da REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis:
os onze campos historicamente lidos pelos scanners artesanais (_read_config_field de sync.py e o
leitor inexistente de update.py) resolvem como string via o único caminho config.load(), expostos
em cfg["update"] e cfg["sync"]. As chaves continuam planas na raiz do YAML — só o dict em memória
é namespaced.
"""

import os
import sys
import tempfile
import shutil
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config


class TestConfigUpdateSync(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_onze_campos_resolvidos_como_string(self):
        yaml_content = (
            "hooks: husky\n"
            "ci: github\n"
            "backend: go\n"
            "frontend: react\n"
            "pkg_manager: npm\n"
            "linear_api_key: lin_api_abc123\n"
            "linear_team_id: TEAM-1\n"
            'jira_base_url: "https://x.atlassian.net:443"\n'
            "jira_email: bot@example.com\n"
            "jira_token: jira_tok_xyz\n"
            "jira_project: PROJ\n"
        )
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write(yaml_content)

        cfg = config.load(cwd=self.tmpdir)

        self.assertEqual(cfg["update"]["hooks"], "husky")
        self.assertEqual(cfg["update"]["ci"], "github")
        self.assertEqual(cfg["update"]["backend"], "go")
        self.assertEqual(cfg["update"]["frontend"], "react")
        self.assertEqual(cfg["update"]["pkg_manager"], "npm")
        self.assertEqual(cfg["sync"]["linear_api_key"], "lin_api_abc123")
        self.assertEqual(cfg["sync"]["linear_team_id"], "TEAM-1")
        self.assertEqual(cfg["sync"]["jira_base_url"], "https://x.atlassian.net:443")
        self.assertEqual(cfg["sync"]["jira_email"], "bot@example.com")
        self.assertEqual(cfg["sync"]["jira_token"], "jira_tok_xyz")
        self.assertEqual(cfg["sync"]["jira_project"], "PROJ")

    def test_campos_ausentes_default_para_string_vazia(self):
        cfg = config.load(cwd=self.tmpdir)

        self.assertEqual(cfg["update"]["hooks"], "")
        self.assertEqual(cfg["update"]["ci"], "")
        self.assertEqual(cfg["update"]["backend"], "")
        self.assertEqual(cfg["update"]["frontend"], "")
        self.assertEqual(cfg["update"]["pkg_manager"], "")
        self.assertEqual(cfg["sync"]["linear_api_key"], "")
        self.assertEqual(cfg["sync"]["linear_team_id"], "")
        self.assertEqual(cfg["sync"]["jira_base_url"], "")
        self.assertEqual(cfg["sync"]["jira_email"], "")
        self.assertEqual(cfg["sync"]["jira_token"], "")
        self.assertEqual(cfg["sync"]["jira_project"], "")


if __name__ == "__main__":
    unittest.main()
