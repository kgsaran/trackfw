"""conftest.py — isola $HOME para a suíte inteira de testes Python.

ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
independente-de-fiacao, ML-3A: validate_guard_global_script_integrity (pypi/trackfw/validator.py)
agora dispara pela EXISTÊNCIA de ~/.trackfw/scripts/<guard>.sh, não mais só quando algum config
referencia o marker. Sem isolar $HOME aqui, qualquer teste que chame v.validate()/o comando
`validate` sem controlar $HOME explicitamente enxerga o $HOME REAL de quem roda a suíte — e um
$HOME real com o harness instalado e o script desatualizado (o próprio caso que motivou a REQ desta
ML) faz esses testes falharem por um motivo que não tem nada a ver com o que estão testando. Mesmo
precedente do Cenário 46 em scripts/check-gates-falsify.sh, e do TestMain equivalente adicionado em
internal/validator/main_test.go (Go).

session-scoped e autouse: roda uma vez, antes de qualquer teste. Testes que já isolam $HOME por
conta própria (test_git_branch_guard_validator.py, test_credential_guard_dedup.py etc., via
os.environ['HOME'] = tempfile.mkdtemp() em setUp/tearDown) continuam funcionando sem alteração —
eles salvam o HOME "original" (que passa a ser este $HOME sintético, não o real) e restauram para
ele ao final, então não há conflito.
"""
import os
import shutil
import tempfile

import pytest


@pytest.fixture(scope="session", autouse=True)
def _isolated_home_for_test_session():
    fake_home = tempfile.mkdtemp(prefix="trackfw-pypi-test-home-")
    os.environ["HOME"] = fake_home
    try:
        yield
    finally:
        shutil.rmtree(fake_home, ignore_errors=True)
