"""
test_barrier_contract.py — Contrato universal de `trackfw barrier`, congelado em
docs/cli-parity.md (seção `## trackfw barrier`). Estes testes NAO implementam producao:
eles fixam, nos tres runtimes, os oito cenarios obrigatorios definidos pelo ML-1A do
roadmap ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.

Mecanismo de pendencia: cada teste recebe @pytest.mark.xfail(strict=True, reason=...).
Diferente de Go/Node, o xfail EXECUTA o corpo do teste — o comando "barrier" ainda nao
registrado no argparse faz as assercoes falharem, o que e o comportamento esperado ate
o ML-2C remover a marcacao. O ML-2C deve REMOVER o xfail, nao reescrever o teste.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

import pytest

PYPI_ROOT = Path(__file__).resolve().parents[1]
XFAIL_REASON = "pendente até ML-2C: trackfw barrier ainda não implementado"


# ────────────────────────────────────────────────────────────────────────────
# Fixture builder — reproduz as regras de parsing string-level da seção
# "Roadmap parsing rules" de docs/cli-parity.md.
# ────────────────────────────────────────────────────────────────────────────

def _build_barrier_roadmap(
    linked_req: bool = True,
    ml_status: str = "✅",
    criteria_lines=None,
    omit_criteria_block: bool = False,
    gate_commands=None,
) -> str:
    criteria_lines = criteria_lines or []
    out = "# Roadmap: Barrier Contract Fixture\n\n"
    if linked_req:
        out += "REQ: REQ-2026-07-29-barrier-fixture\n\n"
    # Bloco de aceite em nível de roadmap — satisfaz wip_acceptance (governança),
    # distinto do bloco por-ML usado pela barrier (rule 4).
    out += "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"

    out += "## Wave 1 — Fixture Wave\n> Dependências: nenhuma\n\n"
    if gate_commands is not None:
        out += "**Gates da wave:**\n```bash\n"
        for c in gate_commands:
            out += c + "\n"
        out += "```\n\n"

    out += "### ML-1A — Fixture ML\n"
    out += "**Status:** " + ml_status + "\n"
    if not omit_criteria_block:
        out += "**Critérios de aceite:**\n"
        for line in criteria_lines:
            out += line + "\n"
    out += "\n"
    return out


def _setup_barrier_fixture(**kwargs) -> Path:
    """Escreve a árvore de governança + o roadmap de fixture em um diretório
    temporário e devolve o caminho do diretório."""
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-"))
    for d in (
        "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
        "docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
    ):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    (dir_ / "docs/roadmaps/wip/ROADMAP-barrier-fixture.md").write_text(
        _build_barrier_roadmap(**kwargs), encoding="utf-8"
    )
    return dir_


def _run_barrier_cli(cwd: Path, *args: str):
    """Invoca `python -m trackfw barrier <args>` em cwd e devolve
    (stdout, stderr, returncode)."""
    env = dict(os.environ)
    env["PYTHONPATH"] = str(PYPI_ROOT)
    result = subprocess.run(
        [sys.executable, "-m", "trackfw", "barrier", *args],
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        # encoding explicito: o CLI escreve UTF-8 nos tres runtimes (ver
        # _force_utf8_output em trackfw/cli.py). text=True sozinho decodifica
        # pelo locale — cp1252 no Windows — e transforma o travessao U+2014
        # desta mensagem pinada em mojibake.
        encoding="utf-8",
        check=False,
    )
    return result.stdout, result.stderr, result.returncode


# ────────────────────────────────────────────────────────────────────────────
# 1 — wave_verde_passa
# ────────────────────────────────────────────────────────────────────────────

def test_wave_verde_passa():
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes", "- [x] tests pass"],
        gate_commands=None,  # sem bloco de gates — zero gates é legal
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 0, f"expected exit 0, got {code}\nstdout: {stdout}\nstderr: {stderr}"

    doc = json.loads(stdout)
    assert doc["status"] == "passed"

    gates_check = next((c for c in doc["checks"] if c["name"] == "gates"), None)
    assert gates_check is not None, "expected a 'gates' check in the result document"
    assert gates_check["status"] == "passed"
    assert gates_check["commands"] == []


# ────────────────────────────────────────────────────────────────────────────
# 2 — ml_pendente_bloqueia
# ────────────────────────────────────────────────────────────────────────────

def test_ml_pendente_bloqueia():
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="⬜ Pendente",
        criteria_lines=["- [x] build passes"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1, f"expected exit 1, got {code}\nstdout: {stdout}\nstderr: {stderr}"

    doc = json.loads(stdout)
    assert doc["status"] == "blocked"

    mls_check = next((c for c in doc["checks"] if c["name"] == "mls_complete"), None)
    assert mls_check is not None, "expected a 'mls_complete' check in the result document"
    assert mls_check["status"] == "blocked"


# ────────────────────────────────────────────────────────────────────────────
# 3 — evidencia_ausente_bloqueia
# ────────────────────────────────────────────────────────────────────────────

def test_evidencia_ausente_bloqueia():
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes", "- [ ] tests pass"],  # ao menos um não marcado
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1, f"expected exit 1, got {code}\nstdout: {stdout}\nstderr: {stderr}"

    doc = json.loads(stdout)
    assert doc["status"] == "blocked"

    evidence_check = next((c for c in doc["checks"] if c["name"] == "acceptance_evidence"), None)
    assert evidence_check is not None, "expected an 'acceptance_evidence' check in the result document"
    assert evidence_check["status"] == "blocked"


# ────────────────────────────────────────────────────────────────────────────
# 4 — ml_sem_bloco_de_criterios_bloqueia (caso anti-vacuidade)
# ────────────────────────────────────────────────────────────────────────────

def test_ml_sem_bloco_de_criterios_bloqueia():
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="✅",
        omit_criteria_block=True,  # nenhum bloco "**Critérios de aceite:**" — não pode passar vacuamente
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1, f"expected exit 1 (anti-vacuity), got {code}\nstdout: {stdout}\nstderr: {stderr}"

    doc = json.loads(stdout)
    assert doc["status"] == "blocked"

    evidence_check = next((c for c in doc["checks"] if c["name"] == "acceptance_evidence"), None)
    assert evidence_check is not None, "expected an 'acceptance_evidence' check in the result document"
    assert evidence_check["status"] == "blocked"


# ────────────────────────────────────────────────────────────────────────────
# 5 — gate_falho_bloqueia
# ────────────────────────────────────────────────────────────────────────────

def test_gate_falho_bloqueia():
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
        gate_commands=["false"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1, f"expected exit 1, got {code}\nstdout: {stdout}\nstderr: {stderr}"

    doc = json.loads(stdout)
    assert doc["status"] == "blocked"

    gates_check = next((c for c in doc["checks"] if c["name"] == "gates"), None)
    assert gates_check is not None, "expected a 'gates' check in the result document"
    assert gates_check["status"] == "blocked"
    assert "false" in gates_check["commands"]


# ────────────────────────────────────────────────────────────────────────────
# 6 — validate_falho_bloqueia
# ────────────────────────────────────────────────────────────────────────────

def test_validate_falho_bloqueia():
    # Wave/ML/gates estão inteiramente verdes; a única falha é de governança
    # (roadmap em wip sem REQ vinculada), que só o check "validate" deve capturar.
    dir_ = _setup_barrier_fixture(
        linked_req=False,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1, f"expected exit 1, got {code}\nstdout: {stdout}\nstderr: {stderr}"

    doc = json.loads(stdout)
    assert doc["status"] == "blocked"

    validate_check = next((c for c in doc["checks"] if c["name"] == "validate"), None)
    assert validate_check is not None, "expected a 'validate' check in the result document"
    assert validate_check["status"] == "blocked"

    # Os demais checks devem permanecer verdes — prova que o fixture isola a falha.
    for c in doc["checks"]:
        if c["name"] != "validate":
            assert c["status"] == "passed", f"expected only 'validate' to be blocked, but {c['name']} is {c['status']}"


# ────────────────────────────────────────────────────────────────────────────
# 7 — roadmap_ou_wave_inexistente_e_erro_de_uso
# ────────────────────────────────────────────────────────────────────────────

def test_roadmap_ou_wave_inexistente_e_erro_de_uso():
    # Sub-caso 1 — wave inexistente
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "99", "--json")
    assert code == 2, f"expected exit 2 (usage error) for nonexistent wave, got {code}"
    assert '"status":"blocked"' not in stdout and '"status": "blocked"' not in stdout, (
        f"a usage error must never emit a status=blocked result document, got: {stdout}"
    )
    assert "wave" in stderr.lower() or "99" in stderr, (
        f"usage error must explicitly name the unresolved wave, got stderr: {stderr}"
    )

    # Sub-caso 2 — roadmap inexistente
    empty_dir = Path(tempfile.mkdtemp(prefix="tw-barrier-empty-"))
    (empty_dir / "docs/roadmaps/wip").mkdir(parents=True, exist_ok=True)
    stdout, stderr, code = _run_barrier_cli(empty_dir, "ROADMAP-does-not-exist", "--wave", "1", "--json")
    assert code == 2, f"expected exit 2 (usage error) for nonexistent roadmap, got {code}"
    assert '"status":"blocked"' not in stdout and '"status": "blocked"' not in stdout, (
        f"a usage error must never emit a status=blocked result document, got: {stdout}"
    )
    assert "roadmap" in stderr.lower() or "does-not-exist" in stderr.lower(), (
        f"usage error must explicitly name the unresolved roadmap, got stderr: {stderr}"
    )


# ────────────────────────────────────────────────────────────────────────────
# 8 — json_deterministico
# ────────────────────────────────────────────────────────────────────────────

def test_json_deterministico():
    dir_ = _setup_barrier_fixture(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
        gate_commands=["true"],
    )
    want_order = ["mls_complete", "acceptance_evidence", "gates", "validate"]

    for run in range(2):
        stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
        assert code == 0, f"run {run}: expected exit 0, got {code}\nstdout: {stdout}\nstderr: {stderr}"

        doc = json.loads(stdout)
        assert len(doc["checks"]) == len(want_order), f"run {run}: expected {len(want_order)} checks"

        for i, name in enumerate(want_order):
            check = doc["checks"][i]
            assert check["name"] == name, f"run {run}: expected checks[{i}].name={name}, got {check['name']}"
            assert isinstance(check["evidence"], list), f"run {run}: checks[{i}].evidence must always be a list"
            assert isinstance(check["failures"], list), f"run {run}: checks[{i}].failures must always be a list"
            if name == "gates":
                assert "commands" in check, f"run {run}: 'commands' must be present on the gates check"
            else:
                assert "commands" not in check, (
                    f"run {run}: 'commands' must be present only on the gates check, found on {name}"
                )

        assert isinstance(doc["failures"], list), f"run {run}: top-level failures must always be a list"
