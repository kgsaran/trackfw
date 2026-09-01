"""Oraculo de fim de linha: nenhum artefato gerado pelo Python sai com CRLF.

`open(path, "w")` do Python usa `newline=None`, que traduz `LF` para `os.linesep`
— CRLF no Windows. Go e Node escrevem bytes direto. Sem `newline` explicito os
tres runtimes produzem artefato diferente byte a byte para a mesma entrada, e os
`scripts/*.sh` gerados saem com CR no shebang, que falha em POSIX com
"bad interpreter".

LEITURA BINARIA E O MECANISMO; A ASSERCAO E O ORACULO. `test_generators_roadmap.py`
ja abre os arquivos gerados em "rb", mas so compara idempotencia
(`bytes_before == bytes_after`) — o que nunca acusa CRLF, porque as duas leituras
saem igualmente erradas. Este arquivo acrescenta a assercao que faltava.

Num Linux/macOS `os.linesep` ja e LF e estes testes passam com ou sem a correcao:
ali eles valem como guarda de regressao, nao como reproducao. Em Windows eles
nascem vermelhos sem a correcao.
"""

import os
import tempfile
import unittest

from trackfw import config as cfg_module
from trackfw.generators.adr import generate_adr
from trackfw.generators.req import generate_req
from trackfw.generators.roadmap import generate_roadmap
from trackfw.generators.note import new_note

CRLF = bytes([13, 10])


def _bytes(path):
    with open(path, "rb") as handle:
        return handle.read()


class TestGeneratorsWriteLF(unittest.TestCase):
    """Cada gerador escreve o artefato em LF, em qualquer plataforma."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self._cwd = os.getcwd()
        os.chdir(self.tmp)
        cfg_module.reset()

    def tearDown(self):
        os.chdir(self._cwd)
        cfg_module.reset()

    def _cfg(self):
        cfg = cfg_module.defaults()
        cfg["roadmap_dir"] = os.path.join(self.tmp, "docs", "roadmaps")
        cfg["req_dir"] = os.path.join(self.tmp, "docs", "req")
        cfg["roadmap_namespacing"] = "flat"
        return cfg

    def _assert_lf(self, path, rotulo):
        raw = _bytes(path)
        self.assertNotIn(
            CRLF,
            raw,
            "%s saiu com CRLF: %r" % (rotulo, raw[:120]),
        )
        self.assertIn(bytes([10]), raw, "%s nao tem nenhuma quebra de linha" % rotulo)

    def test_adr_sai_em_lf(self):
        path = generate_adr("titulo de teste", cwd=self.tmp)
        self._assert_lf(os.path.join(self.tmp, path) if not os.path.isabs(path) else path, "ADR")

    def test_req_sai_em_lf(self):
        path = generate_req("titulo de teste", cwd=self.tmp)
        self._assert_lf(os.path.join(self.tmp, path) if not os.path.isabs(path) else path, "REQ")

    def test_roadmap_sai_em_lf(self):
        path = generate_roadmap("titulo de teste", self._cfg())
        self._assert_lf(os.path.join(self.tmp, path) if not os.path.isabs(path) else path, "ROADMAP")

    def test_note_sai_em_lf(self):
        path = new_note("titulo de teste", cwd=self.tmp)
        full = os.path.join(self.tmp, path) if not os.path.isabs(path) else path
        self._assert_lf(full, "nota")
        indice = os.path.join(self.tmp, "vault", "notes", "index.md")
        if os.path.exists(indice):
            self._assert_lf(indice, "indice do vault")


if __name__ == "__main__":
    unittest.main()
