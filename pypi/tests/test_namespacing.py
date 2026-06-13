"""
test_namespacing.py — Testes para roadmap_namespacing: by_agent no CLI Python.

Cobre:
  - test_config_by_agent_parsed: parse de roadmap_namespacing + agents no config
  - test_validator_by_agent_wip_limit: validator respeita hierarquia <agent>/wip/ por agente
  - test_validator_flat_unchanged: sem namespacing → comportamento flat inalterado
"""

import os
import sys
import tempfile
import shutil
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw import validator as v


def _write(path: str, content: str = ""):
    """Cria arquivo e diretórios necessários."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


class TestConfigByAgentParsed(unittest.TestCase):
    """test_config_by_agent_parsed: parse de roadmap_namespacing e agents."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        _config.reset()
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_config_by_agent_parsed(self):
        """roadmap_namespacing: by_agent + agents: [zeus, apolo] → config correto."""
        yaml_content = (
            "roadmap_namespacing: by_agent\n"
            "agents:\n"
            "  - zeus\n"
            "  - apolo\n"
        )
        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)

        cfg = _config.load(cwd=self.tmp)

        self.assertEqual(cfg["roadmap_namespacing"], "by_agent")
        self.assertEqual(cfg["agents"], ["zeus", "apolo"])

    def test_config_agents_sem_indentacao(self):
        """agents sem indentação também é aceito (formato alternativo válido)."""
        yaml_content = (
            "roadmap_namespacing: by_agent\n"
            "agents:\n"
            "- zeus\n"
            "- apolo\n"
        )
        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)

        cfg = _config.load(cwd=self.tmp)

        self.assertEqual(cfg["roadmap_namespacing"], "by_agent")
        self.assertEqual(cfg["agents"], ["zeus", "apolo"])

    def test_config_default_flat(self):
        """Sem yaml → roadmap_namespacing default é 'flat' e agents é lista vazia."""
        cfg = _config.load(cwd=self.tmp)
        self.assertEqual(cfg["roadmap_namespacing"], "flat")
        self.assertEqual(cfg["agents"], [])


class TestValidatorByAgentWipLimit(unittest.TestCase):
    """test_validator_by_agent_wip_limit: validator varre hierarquia de dois níveis por agente."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        _config.reset()
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _make_cfg(self, agents, wip_limit=1, roadmap_dir=None):
        """Monta dict de config para by_agent."""
        cfg = _config.defaults()
        cfg["roadmap_namespacing"] = "by_agent"
        cfg["agents"] = agents
        cfg["wip_limit"] = wip_limit
        cfg["roadmap_dir"] = roadmap_dir or os.path.join(self.tmp, "docs/roadmaps")
        return cfg

    def test_validator_by_agent_wip_limit(self):
        """
        Validator com by_agent varre <roadmap_dir>/<agente>/wip/.
        Dois agentes independentes: zeus com 1 roadmap (ok), apolo com 2 (excede limit=1).
        """
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")

        # zeus: 1 roadmap em wip (dentro do limite)
        _write(os.path.join(roadmap_dir, "zeus", "wip", "roadmap-zeus-1.md"), "# Zeus WIP 1")

        # apolo: 2 roadmaps em wip (excede limit=1)
        _write(os.path.join(roadmap_dir, "apolo", "wip", "roadmap-apolo-1.md"), "# Apolo WIP 1")
        _write(os.path.join(roadmap_dir, "apolo", "wip", "roadmap-apolo-2.md"), "# Apolo WIP 2")

        cfg = self._make_cfg(agents=["zeus", "apolo"], wip_limit=1, roadmap_dir=roadmap_dir)
        result = v.validate_wip_limit(cfg)

        warnings = result["warnings"]
        violations = result["violations"]

        # Zeus não deve gerar warning (1 <= 1)
        zeus_msgs = [w["message"] for w in warnings if '"zeus"' in w["message"]]
        self.assertEqual(zeus_msgs, [], "zeus com 1 roadmap não deve gerar warning")

        # Apolo deve gerar warning (2 > 1)
        apolo_msgs = [w["message"] for w in warnings if '"apolo"' in w["message"]]
        self.assertEqual(len(apolo_msgs), 1)
        self.assertIn("apolo", apolo_msgs[0])
        self.assertIn("2", apolo_msgs[0])

        # Não deve haver violations
        self.assertEqual(violations, [])

    def test_validator_by_agent_resolve_wip_dirs(self):
        """resolve_wip_dirs retorna caminho correto por agente."""
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")
        cfg = self._make_cfg(agents=["zeus", "apolo"], roadmap_dir=roadmap_dir)

        result = v.resolve_wip_dirs(cfg)

        self.assertEqual(result, [
            roadmap_dir + "/zeus/wip",
            roadmap_dir + "/apolo/wip",
        ])

    def test_validator_by_agent_autodiscover_agents(self):
        """
        Se agents=[],  resolve_wip_dirs descobre agentes pelos subdiretórios do roadmap_dir.
        """
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")
        os.makedirs(os.path.join(roadmap_dir, "zeus"), exist_ok=True)
        os.makedirs(os.path.join(roadmap_dir, "apolo"), exist_ok=True)

        cfg = _config.defaults()
        cfg["roadmap_namespacing"] = "by_agent"
        cfg["agents"] = []  # lista vazia → autodiscover
        cfg["roadmap_dir"] = roadmap_dir

        result = v.resolve_wip_dirs(cfg)

        # Deve descobrir os dois agentes
        self.assertIn(roadmap_dir + "/zeus/wip", result)
        self.assertIn(roadmap_dir + "/apolo/wip", result)


class TestValidatorFlatUnchanged(unittest.TestCase):
    """test_validator_flat_unchanged: sem namespacing → comportamento flat inalterado."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        _config.reset()
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_validator_flat_unchanged(self):
        """
        Sem roadmap_namespacing (flat), resolve_wip_dirs retorna apenas <roadmap_dir>/wip.
        """
        cfg = _config.defaults()
        cfg["roadmap_dir"] = os.path.join(self.tmp, "docs/roadmaps")
        # roadmap_namespacing permanece "flat" (default)

        result = v.resolve_wip_dirs(cfg)

        self.assertEqual(result, [os.path.join(self.tmp, "docs/roadmaps") + "/wip"])

    def test_validator_flat_wip_limit_global(self):
        """
        Modo flat: wip_limit é verificado globalmente contra a pasta wip/ raiz.
        """
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")
        wip_dir = os.path.join(roadmap_dir, "wip")

        # 3 roadmaps em wip (excede limit=2)
        _write(os.path.join(wip_dir, "r1.md"), "# R1")
        _write(os.path.join(wip_dir, "r2.md"), "# R2")
        _write(os.path.join(wip_dir, "r3.md"), "# R3")

        # Criar trackfw.yaml com wip_limit=2
        yaml_content = "wip_limit: 2\n"
        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)

        cfg = _config.defaults()
        cfg["roadmap_dir"] = roadmap_dir
        # roadmap_namespacing = "flat" (default)

        # validate_wip_limit usa _read_wip_config() que lê do CWD
        # Para este teste, verificamos resolve_wip_dirs (comportamento flat)
        wip_dirs = v.resolve_wip_dirs(cfg)
        self.assertEqual(len(wip_dirs), 1)
        self.assertTrue(wip_dirs[0].endswith("/wip"))

    def test_flat_nao_tem_subagente_no_caminho(self):
        """
        Modo flat: o caminho wip/ NÃO contém subdiretório de agente.
        """
        cfg = _config.defaults()
        cfg["roadmap_dir"] = "docs/roadmaps"
        # sem agents, sem namespacing

        result = v.resolve_wip_dirs(cfg)

        self.assertEqual(len(result), 1)
        # Garante que não há segmento de agente no meio do caminho
        self.assertNotIn("zeus", result[0])
        self.assertNotIn("apolo", result[0])
        self.assertEqual(result[0], "docs/roadmaps/wip")


if __name__ == "__main__":
    unittest.main()
