"""
Testes de unidade para a forma flow-style (inline) de lista YAML em pypi/trackfw/config.py.

Contrato (ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis):

 # | Entrada                                                | Resultado
 1 | [a, b]                                                 | [a, b]
 2 | [a,b]                                                  | [a, b]
 3 | [ a , b ]                                               | [a, b]
 4 | ["a", "b"]                                             | [a, b]
 5 | ['a', 'b']                                             | [a, b]
 6 | [a]                                                     | [a]
 7 | []                                                      | lista vazia, não default
 8 | ["a, b", "c"]                                          | dois itens: "a, b" e "c"
 9 | ["## Acceptance Criteria", "## Critérios de Aceite"]   | os dois marcadores
"""

import os
import sys
import tempfile
import shutil
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config

CASES = [
    ("1_espaco_simples", "[a, b]", ["a", "b"]),
    ("2_sem_espaco", "[a,b]", ["a", "b"]),
    ("3_espacos_extras", "[ a , b ]", ["a", "b"]),
    ("4_aspas_duplas", '["a", "b"]', ["a", "b"]),
    ("5_aspas_simples", "['a', 'b']", ["a", "b"]),
    ("6_item_unico", "[a]", ["a"]),
    ("7_lista_vazia", "[]", []),
    ("8_virgula_dentro_de_aspas", '["a, b", "c"]', ["a, b", "c"]),
    (
        "9_marcadores_reais",
        '["## Acceptance Criteria", "## Critérios de Aceite"]',
        ["## Acceptance Criteria", "## Critérios de Aceite"],
    ),
]


class TestConfigInlineList(unittest.TestCase):

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

    def _load(self, content):
        config.reset()
        self._write_yaml(content)
        return config.load(cwd=self.tmpdir)


def _make_test(key_yaml, cfg_path, input_val, want):
    def test(self):
        cfg = self._load(f"{key_yaml}: {input_val}\n")
        got = cfg
        for part in cfg_path:
            got = got[part]
        self.assertEqual(got, want)
    return test


for name, input_val, want in CASES:
    setattr(TestConfigInlineList, f"test_adr_dirs_{name}",
            _make_test("adr_dirs", ["adr_dirs"], input_val, want))
    setattr(TestConfigInlineList, f"test_agents_{name}",
            _make_test("agents", ["agents"], input_val, want))
    setattr(TestConfigInlineList, f"test_acceptance_markers_{name}",
            _make_test("acceptance_markers", ["acceptance_markers"], input_val, want))


class TestConfigInlineListLinkFields(unittest.TestCase):

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


def _make_link_fields_test(sub_key, input_val, want):
    def test(self):
        config.reset()
        self._write_yaml(f"link_fields:\n  {sub_key}: {input_val}\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["link_fields"][sub_key], want)
    return test


for name, input_val, want in CASES:
    for sub_key in ("req", "adr", "roadmap"):
        setattr(
            TestConfigInlineListLinkFields,
            f"test_link_fields_{sub_key}_{name}",
            _make_link_fields_test(sub_key, input_val, want),
        )


class TestConfigInlineListEdgeCases(unittest.TestCase):

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

    def test_adr_dirs_inline_expande_til(self):
        self._write_yaml("adr_dirs: [~/adr, docs/adr]\n")
        cfg = config.load(cwd=self.tmpdir)
        home = os.path.expanduser("~")
        self.assertEqual(cfg["adr_dirs"], [os.path.join(home, "adr"), "docs/adr"])

    def test_inline_nao_abre_contexto_de_bloco(self):
        self._write_yaml("agents: [zeus, apolo]\nreq_dir: docs/req\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["agents"], ["zeus", "apolo"])
        self.assertEqual(cfg["req_dir"], "docs/req")


if __name__ == "__main__":
    unittest.main()
