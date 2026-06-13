"""test_baseline.py — Testes para trackfw baseline e ratchet."""

import json
import os
import shutil
import tempfile
import unittest
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw import validator as v


def _write(path: str, content: str = ""):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


class TestBaseline(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self._orig_dir = os.getcwd()
        _config.reset()
        # Estrutura mínima
        for d in ["docs/roadmaps/wip", "docs/roadmaps/backlog",
                  "docs/roadmaps/blocked", "docs/roadmaps/done",
                  "docs/req", "docs/adr"]:
            os.makedirs(os.path.join(self.tmp, d), exist_ok=True)

    def tearDown(self):
        os.chdir(self._orig_dir)
        _config.reset()
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _chdir(self):
        os.chdir(self.tmp)

    def test_save_baseline_cria_arquivo(self):
        """save_baseline() cria .trackfw-baseline.json com formato correto."""
        self._chdir()
        v.save_baseline(
            [{"type": "violation", "message": "violation 1"}],
            [{"type": "warning", "message": "warning 1"}],
        )
        with open(".trackfw-baseline.json", encoding="utf-8") as f:
            data = json.load(f)
        self.assertEqual(data["violations"], ["violation 1"])
        self.assertEqual(data["warnings"], ["warning 1"])
        self.assertIn("created", data)

    def test_load_baseline_retorna_none_se_nao_existe(self):
        """load_baseline() retorna None se .trackfw-baseline.json não existir."""
        self._chdir()
        result = v.load_baseline()
        self.assertIsNone(result)

    def test_validate_filtra_violations_do_baseline(self):
        """validate() com baseline filtra violations já capturadas."""
        _write(os.path.join(self.tmp, "docs/roadmaps/wip/RM-001.md"),
               "---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n")
        self._chdir()

        # Criar baseline com a violation atual
        raw = v.validate_unfiltered()
        v.save_baseline(raw["violations"], raw["warnings"])

        # validate() deve filtrar a violation do RM-001
        result = v.validate()
        msgs = [
            item["message"] if isinstance(item, dict) else str(item)
            for item in result.get("violations", [])
        ]
        self.assertFalse(any("RM-001" in m for m in msgs),
            f"violations do baseline devem ser filtradas. msgs: {msgs}")

    def test_validate_reporta_violations_novas(self):
        """validate() com baseline reporta violations novas (não no baseline)."""
        self._chdir()
        # Baseline vazio
        v.save_baseline([], [])

        # Criar nova violation
        _write(os.path.join(self.tmp, "docs/roadmaps/wip/RM-002.md"),
               "---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n")

        result = v.validate()
        msgs = [
            item["message"] if isinstance(item, dict) else str(item)
            for item in result.get("violations", [])
        ]
        self.assertTrue(any("RM-002" in m for m in msgs),
            f"nova violation deve aparecer. msgs: {msgs}")


if __name__ == "__main__":
    unittest.main()
