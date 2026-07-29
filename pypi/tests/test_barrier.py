"""
test_barrier.py — Testes unitários adicionais do parser e dos checks de
`trackfw barrier`, complementares ao contrato congelado em
test_barrier_contract.py (docs/cli-parity.md § `trackfw barrier`).

Estes testes cobrem detalhes de implementação (mensagens de erro, resolução em
done/, parsing malformado) que o contrato universal não precisa fixar
literalmente nos três runtimes.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

from .test_barrier_contract import (
    PYPI_ROOT,
    _build_barrier_roadmap,
    _run_barrier_cli,
)


def _setup_dir(state: str = "wip", **kwargs) -> Path:
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in (
        "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
        "docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
    ):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = _build_barrier_roadmap(**kwargs)
    (dir_ / f"docs/roadmaps/{state}/ROADMAP-barrier-fixture.md").write_text(content, encoding="utf-8")
    return dir_


# ────────────────────────────────────────────────────────────────────────────
# Resolução de roadmap
# ────────────────────────────────────────────────────────────────────────────

def test_resolve_roadmap_em_done():
    dir_ = _setup_dir(
        state="done",
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 0, f"stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    assert doc["status"] == "passed"


def test_resolve_roadmap_com_extensao_md_explicita():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture.md", "--wave", "1", "--json")
    assert code == 0, f"stdout={stdout} stderr={stderr}"


def test_wave_inexistente_mensagem_nomeia_wave():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "7", "--json")
    assert code == 2
    assert "7" in stderr


def test_roadmap_inexistente_mensagem_nomeia_roadmap():
    empty_dir = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-empty-"))
    (empty_dir / "docs/roadmaps/wip").mkdir(parents=True, exist_ok=True)
    _, stderr, code = _run_barrier_cli(empty_dir, "ROADMAP-nao-existe", "--wave", "1", "--json")
    assert code == 2
    assert "ROADMAP-nao-existe" in stderr


def test_wave_zero_e_erro_de_uso():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "0", "--json")
    assert code == 2
    assert "wave" in stderr.lower()


def test_wave_nao_numerica_e_erro_de_uso():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "abc", "--json")
    assert code == 2
    assert "wave" in stderr.lower()


def test_wave_flag_ausente_e_erro_de_uso():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, _, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture")
    assert code == 2


# ────────────────────────────────────────────────────────────────────────────
# Parsing malformado (rule 6)
# ────────────────────────────────────────────────────────────────────────────

def test_wave_heading_numero_nao_parseavel_e_erro_de_uso():
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Malformed\n\n"
        "REQ: REQ-x\n\n"
        "## Wave one — Malformed Wave\n> Dependências: nenhuma\n\n"
        "### ML-1A — X\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-malformed.md").write_text(content, encoding="utf-8")
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-malformed", "--wave", "1", "--json")
    assert code == 2
    assert "line" in stderr.lower()


def test_gates_fence_nao_terminada_e_erro_de_uso():
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Malformed\n\n"
        "REQ: REQ-x\n\n"
        "## Wave 1 — Malformed Wave\n> Dependências: nenhuma\n\n"
        "**Gates da wave:**\n```bash\ntrue\n"
        "### ML-1A — X\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-malformed.md").write_text(content, encoding="utf-8")
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-malformed", "--wave", "1", "--json")
    assert code == 2
    assert "fenced" in stderr.lower() or "unterminated" in stderr.lower()


# ────────────────────────────────────────────────────────────────────────────
# Multiplos MLs / gates com múltiplos comandos
# ────────────────────────────────────────────────────────────────────────────

def test_multiplos_mls_um_incompleto_bloqueia_apenas_esse():
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Multi\n\n"
        "REQ: REQ-x\n\n"
        "## Acceptance Criteria\n- [x] fixture\n\n"
        "## Wave 1 — Multi Wave\n> Dependências: nenhuma\n\n"
        "### ML-1A — A\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
        "### ML-1B — B\n**Status:** 🔄 Em andamento\n**Critérios de aceite:**\n- [x] b\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-multi.md").write_text(content, encoding="utf-8")
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-multi", "--wave", "1", "--json")
    assert code == 1, f"stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    mls_check = next(c for c in doc["checks"] if c["name"] == "mls_complete")
    assert mls_check["status"] == "blocked"
    assert any("ML-1A: ✅" == e for e in mls_check["evidence"])
    assert any(f.startswith("ML-1B: not complete") for f in mls_check["failures"])


def test_gates_multiplos_comandos_ordem_preservada():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
        gate_commands=["true", "true", "false"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1
    doc = json.loads(stdout)
    gates_check = next(c for c in doc["checks"] if c["name"] == "gates")
    assert gates_check["commands"] == ["true", "true", "false"]
    assert gates_check["evidence"] == ["true: exit 0", "true: exit 0"]
    assert gates_check["failures"] == ["false: exit 1"]


def test_gates_stdout_nao_polui_documento_json():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
        gate_commands=["echo hello-from-gate"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 0, f"stdout={stdout} stderr={stderr}"
    # stdout deve conter exatamente um documento JSON válido, sem output do gate.
    doc = json.loads(stdout)
    assert doc["status"] == "passed"


# ────────────────────────────────────────────────────────────────────────────
# Modo texto
# ────────────────────────────────────────────────────────────────────────────

def test_modo_texto_sem_json_reporta_status():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1")
    assert code == 0
    assert "passed" in stdout.lower()
    # Modo texto não deve ser JSON válido.
    try:
        json.loads(stdout)
        assert False, "modo texto não deveria produzir JSON válido"
    except json.JSONDecodeError:
        pass


# ────────────────────────────────────────────────────────────────────────────
# Determinismo de contadores de acceptance_evidence
# ────────────────────────────────────────────────────────────────────────────

def test_acceptance_evidence_conta_criterios_atendidos():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] a", "- [x] b", "- [x] c"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 0, f"stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    evidence_check = next(c for c in doc["checks"] if c["name"] == "acceptance_evidence")
    assert evidence_check["evidence"] == ["ML-1A: 3 criteria met"]


def test_acceptance_evidence_conta_nao_atendidos():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] a", "- [ ] b", "- [ ] c"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1
    doc = json.loads(stdout)
    evidence_check = next(c for c in doc["checks"] if c["name"] == "acceptance_evidence")
    assert evidence_check["failures"] == ["ML-1A: 2 unmet acceptance criteria"]
