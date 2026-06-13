"""
test_validate_json.py — Testes para a flag --json do comando `trackfw validate`.

Valida:
  - Output com --json é JSON válido e parseable
  - Estrutura do JSON contém campos corretos (summary, violations, warnings)
  - Exit code é idêntico com e sem --json
  - Modo texto permanece inalterado quando --json não é passado
"""

import io
import json
import os
import shutil
import sys
import tempfile
import types
import unittest

# Garante que o pacote pypi/trackfw é importável mesmo sem instalação
_HERE = os.path.dirname(os.path.abspath(__file__))
_PYPI = os.path.dirname(_HERE)
if _PYPI not in sys.path:
    sys.path.insert(0, _PYPI)

from trackfw import config as _config
from trackfw.commands import validate as _validate_cmd


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_file(path: str, content: str = ""):
    """Cria arquivo (e diretórios pai) com o conteúdo dado."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _make_dirs(*paths):
    for p in paths:
        os.makedirs(p, exist_ok=True)


def _make_args(**kwargs) -> types.SimpleNamespace:
    """Cria namespace de args com defaults."""
    defaults = {"json": False}
    defaults.update(kwargs)
    return types.SimpleNamespace(**defaults)


def _run_json(tmp_dir: str):
    """
    Executa run(args) com --json em tmp_dir.
    Captura stdout e o SystemExit.
    Retorna (parsed_dict, exit_code).
    """
    old_cwd = os.getcwd()
    os.chdir(tmp_dir)
    captured = io.StringIO()
    old_stdout = sys.stdout
    sys.stdout = captured
    exit_code = 0
    try:
        _validate_cmd.run(_make_args(json=True))
    except SystemExit as e:
        exit_code = e.code if e.code is not None else 0
    finally:
        sys.stdout = old_stdout
        os.chdir(old_cwd)
        _config.reset()
    output = captured.getvalue()
    return json.loads(output), exit_code


def _run_text(tmp_dir: str):
    """
    Executa run(args) sem --json em tmp_dir.
    Retorna exit_code (0 se não houver SystemExit).
    """
    old_cwd = os.getcwd()
    os.chdir(tmp_dir)
    exit_code = 0
    try:
        _validate_cmd.run(_make_args(json=False))
    except SystemExit as e:
        exit_code = e.code if e.code is not None else 0
    finally:
        os.chdir(old_cwd)
        _config.reset()
    return exit_code


# ---------------------------------------------------------------------------
# Projeto sem violations (dirs vazios)
# ---------------------------------------------------------------------------

class TestValidateJsonSemViolations(unittest.TestCase):
    """Projeto sem violations: --json deve retornar JSON válido com summary correto."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _make_dirs(
            os.path.join(self.tmp, "docs", "adr"),
            os.path.join(self.tmp, "docs", "req"),
            os.path.join(self.tmp, "docs", "roadmaps", "wip"),
            os.path.join(self.tmp, "docs", "roadmaps", "blocked"),
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_json_output_e_valido(self):
        """Output com --json deve ser JSON parseable."""
        result, _ = _run_json(self.tmp)
        self.assertIsInstance(result, dict,
                              "Output deve ser um dict JSON")

    def test_json_contem_summary(self):
        """Output JSON deve conter chave 'summary'."""
        result, _ = _run_json(self.tmp)
        self.assertIn("summary", result)

    def test_json_summary_campos_corretos(self):
        """summary deve ter violations, warnings, mode e exit_code."""
        result, _ = _run_json(self.tmp)
        summary = result["summary"]
        self.assertIn("violations", summary)
        self.assertIn("warnings", summary)
        self.assertIn("mode", summary)
        self.assertIn("exit_code", summary)

    def test_json_contem_violations_e_warnings(self):
        """Output JSON deve conter listas 'violations' e 'warnings'."""
        result, _ = _run_json(self.tmp)
        self.assertIn("violations", result)
        self.assertIn("warnings", result)
        self.assertIsInstance(result["violations"], list)
        self.assertIsInstance(result["warnings"], list)

    def test_json_sem_violations_exit_code_zero(self):
        """Sem violations, exit_code no JSON e exit real devem ser 0."""
        result, exit_code = _run_json(self.tmp)
        self.assertEqual(result["summary"]["exit_code"], 0)
        self.assertEqual(exit_code, 0)

    def test_json_sem_violations_summary_counts(self):
        """Projeto vazio: violations=0 e warnings=0 no summary."""
        result, _ = _run_json(self.tmp)
        self.assertEqual(result["summary"]["violations"], 0)
        self.assertEqual(result["summary"]["warnings"], 0)

    def test_exit_code_identico_com_e_sem_json(self):
        """Exit code deve ser igual independente de --json."""
        _, exit_json = _run_json(self.tmp)
        exit_text = _run_text(self.tmp)
        self.assertEqual(exit_json, exit_text,
                         "Exit code deve ser idêntico com e sem --json")


# ---------------------------------------------------------------------------
# Projeto com violation (roadmap em wip sem REQ)
# ---------------------------------------------------------------------------

class TestValidateJsonComViolation(unittest.TestCase):
    """Projeto com violation: --json deve refletir exit_code=1 e violations populadas."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        _make_dirs(wip_dir)
        # Roadmap sem REQ → violation "wip_has_req"
        _make_file(
            os.path.join(wip_dir, "roadmap-sem-req.md"),
            "# Roadmap sem REQ\n\nConteúdo sem link de REQ.\n",
        )
        _make_dirs(
            os.path.join(self.tmp, "docs", "adr"),
            os.path.join(self.tmp, "docs", "req"),
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_json_com_violation_exit_code_um(self):
        """Com violations, exit_code no JSON e exit real devem ser 1."""
        result, exit_code = _run_json(self.tmp)
        self.assertEqual(result["summary"]["exit_code"], 1)
        self.assertEqual(exit_code, 1)

    def test_json_violations_nao_vazio(self):
        """Lista 'violations' no JSON não deve estar vazia."""
        result, _ = _run_json(self.tmp)
        self.assertGreater(len(result["violations"]), 0)

    def test_json_violations_tem_campo_message(self):
        """Cada item de violations deve ter campo 'message'."""
        result, _ = _run_json(self.tmp)
        for item in result["violations"]:
            self.assertIn("message", item,
                          f"Item de violation sem 'message': {item}")

    def test_json_summary_violations_count(self):
        """summary.violations deve bater com len(violations)."""
        result, _ = _run_json(self.tmp)
        self.assertEqual(
            result["summary"]["violations"],
            len(result["violations"]),
        )

    def test_exit_code_identico_com_e_sem_json(self):
        """Exit code deve ser igual independente de --json (ambos 1)."""
        _, exit_json = _run_json(self.tmp)
        exit_text = _run_text(self.tmp)
        self.assertEqual(exit_json, exit_text,
                         "Exit code deve ser idêntico com e sem --json")


# ---------------------------------------------------------------------------
# Projeto em modo lenient
# ---------------------------------------------------------------------------

class TestValidateJsonLenient(unittest.TestCase):
    """Modo lenient: violations viram warnings; exit_code=0 e mode='lenient' no JSON."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        _make_dirs(wip_dir)
        _make_file(
            os.path.join(wip_dir, "roadmap-sem-req.md"),
            "# Roadmap sem REQ\n\nConteúdo sem link de REQ.\n",
        )
        _make_dirs(
            os.path.join(self.tmp, "docs", "adr"),
            os.path.join(self.tmp, "docs", "req"),
        )
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\ngovernance_mode: lenient\n",
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_json_lenient_mode_no_summary(self):
        """summary.mode deve ser 'lenient'."""
        result, _ = _run_json(self.tmp)
        self.assertEqual(result["summary"]["mode"], "lenient")

    def test_json_lenient_exit_code_zero(self):
        """Modo lenient: violations → warnings; exit_code=0."""
        result, exit_code = _run_json(self.tmp)
        self.assertEqual(result["summary"]["violations"], 0)
        self.assertEqual(result["summary"]["exit_code"], 0)
        self.assertEqual(exit_code, 0)

    def test_exit_code_identico_com_e_sem_json_lenient(self):
        """Em modo lenient, exit code deve ser 0 tanto com quanto sem --json."""
        _, exit_json = _run_json(self.tmp)
        exit_text = _run_text(self.tmp)
        self.assertEqual(exit_json, exit_text)


if __name__ == "__main__":
    unittest.main()
