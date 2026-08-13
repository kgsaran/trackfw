"""test_baseline.py — Testes para trackfw baseline e ratchet."""

import json
import os
import shutil
import subprocess
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


def _git(cwd, *args):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True)


def _init_git_repo(cwd):
    _git(cwd, "init")
    _git(cwd, "config", "user.email", "test@test.com")
    _git(cwd, "config", "user.name", "test")


def _commit_trackfw_yaml(cwd, content):
    _write(os.path.join(cwd, "trackfw.yaml"), content)
    _git(cwd, "add", "trackfw.yaml")
    _git(cwd, "commit", "-m", "trackfw.yaml")


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

    def test_baseline_filters_warnings(self):
        """validate() filtra warnings já capturados no baseline (set-difference)."""
        # Cria ADR sem REQ referenciando → gera warning adr_orphan
        _write(os.path.join(self.tmp, "docs/adr/ADR-001.md"),
               "---\nstatus: Accepted\n---\n# ADR-001\n")
        # trackfw.yaml configurando adr_orphan como warning
        _write(os.path.join(self.tmp, "trackfw.yaml"),
               'rules:\n  adr_orphan: warning\n')
        self._chdir()

        # Captura warnings iniciais e salva baseline
        raw = v.validate_unfiltered()
        warn_msgs = [w["message"] for w in raw["warnings"] if "ADR-001" in w["message"]]
        self.assertTrue(warn_msgs, "setup deve gerar warning adr_orphan para ADR-001")

        v.save_baseline(raw["violations"], raw["warnings"])

        # validate() deve filtrar o warning do ADR-001
        result = v.validate()
        warn_result = [
            item["message"] if isinstance(item, dict) else str(item)
            for item in result.get("warnings", [])
        ]
        self.assertFalse(any("ADR-001" in m for m in warn_result),
            f"warnings do baseline devem ser filtrados. msgs: {warn_result}")

    def test_lenient_baseline_no_recreate(self):
        """Em modo lenient, warnings suprimidos pelo baseline não reaparecem."""
        # Cria ADR sem REQ referenciando → gera warning adr_orphan
        _write(os.path.join(self.tmp, "docs/adr/ADR-001.md"),
               "---\nstatus: Accepted\n---\n# ADR-001\n")
        # trackfw.yaml: adr_orphan como warning + lenient
        _write(os.path.join(self.tmp, "trackfw.yaml"),
               'rules:\n  adr_orphan: warning\ngovernance_mode: lenient\n')
        self._chdir()

        # Salva baseline com o warning existente
        raw = v.validate_unfiltered()
        v.save_baseline(raw["violations"], raw["warnings"])

        # validate() em modo lenient: baseline suprime o warning; lenient não o recria
        result = v.validate()
        warn_result = [
            item["message"] if isinstance(item, dict) else str(item)
            for item in result.get("warnings", [])
        ]
        self.assertFalse(any("ADR-001" in m for m in warn_result),
            f"warning baselined nao deve reaparecer em lenient. msgs: {warn_result}")


    def test_baseline_nao_tolera_credential_guard_mode_downgrade(self):
        """ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard,
        ADR-2026-08-12-severidade-das-regras-de-credential-guard-...: carve-out do baseline —
        violation de credential_guard_mode_downgrade continua sendo reportada por validate() mesmo
        depois de "baselined" — .trackfw-baseline.json não é canal válido para tolerá-la, ao
        contrário de qualquer outra regra (provado por test_validate_filtra_violations_do_baseline
        acima).
        """
        _init_git_repo(self.tmp)
        # HEAD: mode: block. Disco: trackfw.yaml deletado → dispara credential_guard_mode_downgrade.
        _commit_trackfw_yaml(self.tmp, "credential_guard:\n  mode: block\n")
        os.remove(os.path.join(self.tmp, "trackfw.yaml"))
        self._chdir()

        raw = v.validate_unfiltered()
        raw_msgs = [item["message"] for item in raw["violations"]]
        self.assertTrue(any("credential_guard.mode: block" in m for m in raw_msgs),
            f"esperado violation de credential_guard_mode_downgrade antes do baseline: {raw_msgs}")

        # Tentar tolerar via baseline — como qualquer outra violation seria.
        v.save_baseline(raw["violations"], raw["warnings"])

        result = v.validate()
        result_msgs = [
            item["message"] if isinstance(item, dict) else str(item)
            for item in result.get("violations", [])
        ]
        self.assertTrue(any("credential_guard.mode: block" in m for m in result_msgs),
            f"violation de credential_guard_mode_downgrade NÃO deveria ser tolerável via baseline: {result_msgs}")


if __name__ == "__main__":
    unittest.main()
