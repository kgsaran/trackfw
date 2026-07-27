"""
Testes de paridade de fuso horário para os geradores de artefato Python.

Invariante: date.today().isoformat() deve retornar a DATA LOCAL, não UTC.
Python já faz isso corretamente; estes testes travam a semântica como
regressão — se alguém introduzir datetime.utcnow().date(), o teste detecta.

Estratégia determinística (sem mock, sem nova dependência):
  Pacific/Kiritimati = UTC+14, sem DST → sempre 14h à frente do UTC.
  Pacific/Midway     = UTC-11, sem DST → sempre 11h atrás do UTC.
  Span total = 25 horas → as duas datas locais NUNCA coincidem.

Com implementação correta (hora local): kiri ≠ midway → assertNotEqual → PASS
Com implementação quebrada (UTC):       kiri == midway → assertNotEqual → FAIL

Os fusos são aplicados via os.environ["TZ"] + time.tzset() (Unix-only).
Nos casos em que tzset() não está disponível (Windows), o teste é pulado.

REQ: REQ-2026-07-27-convergencia-templates-python
"""

import os
import tempfile
import time
import unittest
from contextlib import contextmanager
from datetime import date


@contextmanager
def timezone(tz_name):
    """Context manager que aplica TZ e restaura o original ao sair (Unix-only)."""
    if not hasattr(time, 'tzset'):
        raise unittest.SkipTest('time.tzset() não disponível (não-Unix)')
    orig = os.environ.get('TZ')
    os.environ['TZ'] = tz_name
    time.tzset()
    try:
        yield
    finally:
        if orig is None:
            os.environ.pop('TZ', None)
        else:
            os.environ['TZ'] = orig
        time.tzset()


class TestGeneratorsLocalDateNotUTC(unittest.TestCase):
    """Verifica que os geradores usam hora local, não UTC."""

    def setUp(self):
        if not hasattr(time, 'tzset'):
            self.skipTest('time.tzset() não disponível (não-Unix)')
        self.tmpdir = tempfile.mkdtemp()

    def _assert_tz_parity(self, make_generator_call, label):
        """
        Executa a chamada ao gerador em UTC+14 e UTC-11 e afirma que:
        1. As datas locais são diferentes entre si (loop-breaker: UTC quebraria isso).
        2. A data do arquivo gerado coincide com date.today() no mesmo TZ.

        Args:
            make_generator_call: callable que aceita um argumento `subdir` (str)
                                  e retorna o path do artefato criado.
                                  O TZ já está configurado pelo context manager externo.
            label: nome do gerador para mensagens de erro.
        """
        with timezone('Pacific/Kiritimati'):
            expected14 = date.today().isoformat()
            path14 = make_generator_call('kiri')

        with timezone('Pacific/Midway'):
            expected11 = date.today().isoformat()
            path11 = make_generator_call('midway')

        self.assertNotEqual(
            expected14, expected11,
            f'{label}: UTC+14 ({expected14}) == UTC-11 ({expected11}) — '
            'span de 25h nunca permite isso, erro de setup',
        )

        basename14 = os.path.basename(path14)
        basename11 = os.path.basename(path11)

        self.assertIn(
            expected14, basename14,
            f'{label} UTC+14: esperado {expected14!r} no basename, obteve {basename14!r}',
        )
        self.assertIn(
            expected11, basename11,
            f'{label} UTC-11: esperado {expected11!r} no basename, obteve {basename11!r}',
        )

    def test_req_usa_hora_local(self):
        """generate_req usa data local — UTC+14 e UTC-11 produzem datas diferentes."""
        from trackfw.generators.req import generate_req

        def run(subdir):
            req_dir = os.path.join(self.tmpdir, 'req', subdir)
            return generate_req('TZ Parity Test', req_dir=req_dir)

        self._assert_tz_parity(run, 'generate_req')

    def test_adr_usa_hora_local(self):
        """generate_adr usa data local — UTC+14 e UTC-11 produzem datas diferentes."""
        from trackfw.generators.adr import generate_adr

        def run(subdir):
            # generate_adr(title, status, adr_dirs, cwd): resolve docs/adr/ relativo ao cwd
            adr_cwd = os.path.join(self.tmpdir, 'adr', subdir)
            os.makedirs(adr_cwd, exist_ok=True)
            return generate_adr('TZ Parity ADR', cwd=adr_cwd)

        self._assert_tz_parity(run, 'generate_adr')

    def test_note_usa_hora_local(self):
        """new_note usa data local — UTC+14 e UTC-11 produzem datas diferentes."""
        from trackfw.generators.note import new_note

        def run(subdir):
            # new_note(title, cwd): cria vault/notes/ relativo ao cwd
            note_cwd = os.path.join(self.tmpdir, 'note', subdir)
            os.makedirs(note_cwd, exist_ok=True)
            return new_note('TZ Parity Note', cwd=note_cwd)

        self._assert_tz_parity(run, 'new_note')


if __name__ == '__main__':
    unittest.main()
