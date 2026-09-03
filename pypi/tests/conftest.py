"""conftest.py — isola a home ($HOME **e** %USERPROFILE%) para a suíte inteira de testes Python.

ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
independente-de-fiacao, ML-3A: validate_guard_global_script_integrity (pypi/trackfw/validator.py)
agora dispara pela EXISTÊNCIA de ~/.trackfw/scripts/<guard>.sh, não mais só quando algum config
referencia o marker. Sem isolar $HOME aqui, qualquer teste que chame v.validate()/o comando
`validate` sem controlar $HOME explicitamente enxerga o $HOME REAL de quem roda a suíte — e um
$HOME real com o harness instalado e o script desatualizado (o próprio caso que motivou a REQ desta
ML) faz esses testes falharem por um motivo que não tem nada a ver com o que estão testando. Mesmo
precedente do Cenário 46 em scripts/check-gates-falsify.sh, e do TestMain equivalente adicionado em
internal/validator/main_test.go (Go).

No Windows, isolar SÓ a variável HOME não basta — e o motivo NÃO é "a produção ignora HOME lá".
Desde 2026-09-01 a produção resolve a home pelo shim `trackfw.homedir.home_dir()`
(pypi/trackfw/homedir.py:58), que no win32 prefere `os.environ["HOME"]`. O defeito é DIVERGÊNCIA DE
CANAL dentro do mesmo processo: a produção lê HOME pelo shim, enquanto qualquer teste/fixture que
calcule a expectativa pelo primitivo da plataforma (`os.path.expanduser("~")`, que no Windows lê
`%USERPROFILE%` e nunca HOME — ver ntpath.expanduser no stdlib) enxerga OUTRA home. Duas homes, um
processo. Evidência medida no job de Windows (run 33742756936):

    ['C:\\...\\trackfw-pypi-test-home-h4vri31t\\adr'] != ['D:\\a\\_temp\\winhome\\adr']

à esquerda a home da fixture (que a produção achou via HOME), à direita a home do job (que o teste
achou via %USERPROFILE%). Apontar as DUAS variáveis para o MESMO diretório sintético colapsa os dois
canais e não sobre-isola: não há terceiro canal que a produção leia legitimamente. Em POSIX
`%USERPROFILE%` é inerte (nem o stdlib nem o produto o leem), então a variável extra é no-op — o
comportamento em Linux/macOS não muda.

session-scoped e autouse: roda uma vez, antes de qualquer teste. Testes que já isolam $HOME por
conta própria (test_git_branch_guard_validator.py, test_credential_guard_dedup.py etc., via
os.environ['HOME'] = tempfile.mkdtemp() em setUp/tearDown) continuam funcionando sem alteração **em
POSIX** — eles salvam o HOME "original" (que passa a ser este $HOME sintético, não o real) e
restauram para ele ao final, então não há conflito.

🔴 Duas ressalvas que valem SÓ no Windows, ambas FORA do alcance desta fixture (precisam de mudança
nos próprios testes, não aqui):

  1. Isolação por-teste que sobrescreve só HOME reabre a divergência DENTRO daquele teste: a
     produção (shim) passa a ver o tmpdir do teste, enquanto expanduser("~") continua devolvendo
     esta home de sessão. É estritamente melhor que antes (o vazamento cai num diretório
     descartável, não no perfil real do runner), mas não é isolação por-teste correta.
  2. Isolação por PATCH DA FUNÇÃO — monkeypatch.setattr("os.path.expanduser", ...), usada em
     test_identity_wizard.py, test_scope_resolution.py e test_thirdparty.py — fica INALCANÇÁVEL no
     win32: home_dir() devolve os.environ["HOME"] antes de chamar expanduser (homedir.py:58), então
     o patch nunca é exercitado e a produção resolve para esta home de sessão. Não é regressão desta
     fixture (o HOME já era setado aqui antes), mas é o caso pior: o patch falha em silêncio.
"""
import os
import shutil
import tempfile

import pytest


@pytest.fixture(scope="session", autouse=True)
def _isolated_home_for_test_session():
    fake_home = tempfile.mkdtemp(prefix="trackfw-pypi-test-home-")
    os.environ["HOME"] = fake_home
    # USERPROFILE junto com HOME: é o canal que os.path.expanduser("~") usa no Windows.
    # No-op em POSIX (ninguém a lê); no Windows é o que impede a divergência de canal descrita acima.
    os.environ["USERPROFILE"] = fake_home
    try:
        yield
    finally:
        shutil.rmtree(fake_home, ignore_errors=True)
