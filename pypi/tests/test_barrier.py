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


def test_wave_zero_nao_presente_no_roadmap_e_erro_de_uso():
    """"--wave 0" é um rótulo válido (Wave 0 threat-model convention, ML-1A de
    ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness), mas a fixture só declara
    "## Wave 1" — então o erro de uso agora vem de "wave not found", não de "invalid
    --wave" (rótulo malformado). Exit 2 nos dois casos; a mensagem muda de causa."""
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "0", "--json")
    assert code == 2
    assert "wave 0" in stderr.lower()
    assert "not found" in stderr.lower()


def test_wave_nao_numerica_e_erro_de_uso():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "abc", "--json")
    assert code == 2
    assert "wave" in stderr.lower()


def test_wave_inexistente_mensagem_pinada_literalmente():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "99", "--json")
    assert code == 2
    assert stderr == 'trackfw barrier: wave 99 not found in roadmap "ROADMAP-barrier-fixture.md"\n'


def test_roadmap_inexistente_mensagem_pinada_literalmente():
    empty_dir = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-empty-"))
    (empty_dir / "docs/roadmaps/wip").mkdir(parents=True, exist_ok=True)
    _, stderr, code = _run_barrier_cli(empty_dir, "ROADMAP-nao-existe", "--wave", "1", "--json")
    assert code == 2
    assert stderr == (
        'trackfw barrier: roadmap "ROADMAP-nao-existe" not found in wip/ nor done/ under docs/roadmaps\n'
    )


def test_wave_sem_mls_produz_mensagem_pinada_literalmente():
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: No ML\n\n"
        "REQ: REQ-x\n\n"
        "## Acceptance Criteria\n- [x] fixture\n\n"
        "## Wave 1 — Sem MLs\n> Dependências: nenhuma\n\n"
        "Some prose, no ML heading at all.\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-no-ml.md").write_text(content, encoding="utf-8")
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-no-ml", "--wave", "1", "--json")
    assert code == 1, f"stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    mls_check = next(c for c in doc["checks"] if c["name"] == "mls_complete")
    assert mls_check["failures"] == ["wave 1: no ML found"]


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


# ────────────────────────────────────────────────────────────────────────────
# Sufixo de wave (ML-2C — gramática <inteiro>[-<sufixo>])
# ────────────────────────────────────────────────────────────────────────────

def test_wave_sufixo_bis_resolve_heading_bis():
    """--wave 2-bis deve resolver ## Wave 2-bis e avaliar a wave com sucesso."""
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Suffix\n\n"
        "REQ: REQ-x\n\n"
        # Bloco de aceite no nível do roadmap — satisfaz wip_acceptance (governança).
        "## Acceptance Criteria\n- [x] fixture criterion\n\n"
        "## Wave 1 — Primeira Wave\n> Dependências: nenhuma\n\n"
        "### ML-1A — X\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
        "## Wave 2-bis — Wave Corretiva\n> Dependências: Wave 1\n\n"
        "### ML-2bisA — Y\n**Status:** ✅\n**Critérios de aceite:**\n- [x] b\n\n"
        "## Wave 3 — Terceira Wave\n> Dependências: Wave 2-bis\n\n"
        "### ML-3A — Z\n**Status:** ✅\n**Critérios de aceite:**\n- [x] c\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-suffix.md").write_text(content, encoding="utf-8")
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-suffix", "--wave", "2-bis", "--json")
    assert code == 0, f"expected exit 0 for --wave 2-bis, got {code}\nstdout={stdout}\nstderr={stderr}"
    doc = json.loads(stdout)
    assert doc["status"] == "passed"
    assert doc["wave"] == "2-bis"


def test_wave_inteiro_nao_casa_heading_bis():
    """--wave 2 NÃO deve casar com ## Wave 2-bis — labels são identidades distintas."""
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Suffix\n\n"
        "REQ: REQ-x\n\n"
        "## Wave 2-bis — Wave Corretiva\n> Dependências: nenhuma\n\n"
        "### ML-2bisA — X\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-suffix.md").write_text(content, encoding="utf-8")
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-suffix", "--wave", "2", "--json")
    assert code == 2, f"expected exit 2 (wave not found), got {code}\nstdout={stdout}\nstderr={stderr}"
    assert "wave 2 not found" in stderr


def test_wave_heading_malformada_aborta_documento_inteiro():
    """Regressão ADR decisão 16: heading fora da gramática aborta o documento inteiro,
    mesmo quando --wave pede uma wave válida que existe DEPOIS da malformada.
    Isso impede que uma wave seja avaliada vacuamente sem auditoria dos MLs malformados.

    Posição: malformada ANTES da wave alvo (posição clássica). Complementado por
    test_wave_heading_malformada_depois_da_wave_alvo_aborta_documento (posição "depois"),
    que cobre o early-break corrigido no ML-2D.
    """
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Abort\n\n"
        "REQ: REQ-x\n\n"
        # Heading malformada ANTES da wave solicitada — deve abortar.
        "## Wave X — Heading Invalida\n> Dependências: nenhuma\n\n"
        "### ML-XA — Bad\n**Status:** ✅\n**Critérios de aceite:**\n- [x] x\n\n"
        "## Wave 1 — Wave Valida\n> Dependências: Wave X\n\n"
        "### ML-1A — Good\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-abort.md").write_text(content, encoding="utf-8")
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-abort", "--wave", "1", "--json")
    assert code == 2, (
        f"expected exit 2 (malformed heading aborts document), got {code}\nstderr={stderr}"
    )
    # Mensagem pinada byte-a-byte: aspas duplas, token capturado, número de linha 1-based
    assert stderr == 'trackfw barrier: malformed wave heading at line 5: "X" is not a valid wave label\n', (
        f"stderr mismatch: got [{stderr!r}]"
    )


def test_wave_heading_malformada_depois_da_wave_alvo_aborta_documento():
    """Regressão ML-2D (pré-passo completo): heading malformada DEPOIS da wave alvo
    também aborta o documento com exit 2 — o pré-passo não deve ter break antecipado.

    Antes da correção do ML-2D, o Python retornava exit 1 'blocked' porque
    _find_wave saía do laço ao encontrar a wave pedida, deixando a heading
    posterior nunca visitada. Isso violava ADR decisões 16 (abort é feature)
    e 12 (roadmap malformado nunca deve ser lido como 'wave reprovada').

    Nota: esta é a posição complementar ao test_wave_heading_malformada_aborta_documento_inteiro.
    Ambas devem estar presentes — teste de uma só posição é vacuoso quanto ao early-break.
    """
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Abort After\n\n"
        "REQ: REQ-x\n\n"
        # Wave alvo válida PRIMEIRO — se o early-break sobreviveu, a malformada
        # abaixo seria ignorada e o exit seria 1 (blocked por validação de governança),
        # não 2. O pré-passo completo deve detectá-la antes de qualquer exit 1.
        "## Wave 1 — Wave Valida\n> Dependências: nenhuma\n\n"
        "### ML-1A — Good\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
        # Heading malformada DEPOIS da wave solicitada — deve abortar mesmo assim.
        "## Wave X — Heading Malformada Depois\n> Posição: depois da wave alvo\n\n"
        "### ML-XA — Bad\n**Status:** ✅\n**Critérios de aceite:**\n- [x] x\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-abort-after.md").write_text(content, encoding="utf-8")
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-abort-after", "--wave", "1", "--json")
    assert code == 2, (
        f"expected exit 2 (malformed heading after target aborts document), got {code}\nstderr={stderr}"
    )
    # Mensagem pinada byte-a-byte — número de linha 1-based da heading malformada.
    # A heading '## Wave X — ...' é a 13ª linha do documento (linha 13 = índice 12).
    assert stderr == 'trackfw barrier: malformed wave heading at line 13: "X" is not a valid wave label\n', (
        f"stderr mismatch: got [{stderr!r}]"
    )


def test_wave_argumento_invalido_mensagem_pinada_literalmente():
    """Quarta mensagem de exit-2 pinada (docs/cli-parity.md §trackfw barrier):
    'trackfw barrier: invalid --wave "<value>" — not a valid wave label'
    O separador é travessão U+2014, não hífen. <value> é o argumento exato.
    """
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
    )
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "2-BIS", "--json")
    assert code == 2, f"expected exit 2, got {code}\nstderr={stderr}"
    assert stderr == 'trackfw barrier: invalid --wave "2-BIS" — not a valid wave label\n', (
        f"stderr mismatch: got [{stderr!r}]"
    )


def test_wave_heading_malformada_uppercase_aborta():
    """Sufixo em maiúsculas é inválido — ## Wave 2-BIS deve abortar o documento."""
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Uppercase\n\n"
        "REQ: REQ-x\n\n"
        "## Wave 2-BIS — Invalida Uppercase\n> Dependências: nenhuma\n\n"
        "### ML-1A — X\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-uppercase.md").write_text(content, encoding="utf-8")
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-uppercase", "--wave", "1", "--json")
    assert code == 2
    assert '"2-BIS" is not a valid wave label' in stderr


def test_wave_heading_malformada_mensagem_tem_aspas_duplas():
    """A mensagem de heading malformada deve usar aspas duplas ao redor do token,
    não aspas simples do !r. Pinado em docs/cli-parity.md § exit-2 messages."""
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-unit-"))
    for d in ("docs/roadmaps/wip", "docs/req", "docs/adr"):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    content = (
        "# Roadmap: Quotes\n\n"
        "REQ: REQ-x\n\n"
        "## Wave abc — Invalida\n> Dependências: nenhuma\n\n"
        "### ML-1A — X\n**Status:** ✅\n**Critérios de aceite:**\n- [x] a\n\n"
    )
    (dir_ / "docs/roadmaps/wip/ROADMAP-quotes.md").write_text(content, encoding="utf-8")
    _, stderr, code = _run_barrier_cli(dir_, "ROADMAP-quotes", "--wave", "1", "--json")
    assert code == 2
    # Aspas duplas — NÃO aspas simples do !r ('abc')
    assert '"abc" is not a valid wave label' in stderr
    assert "'abc'" not in stderr


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


# ────────────────────────────────────────────────────────────────────────────
# Tabela completa da gramática de rótulo de wave (ML-3A, pinned)
# ────────────────────────────────────────────────────────────────────────────

def test_is_valid_wave_label_tabela_completa():
    """Testa _is_valid_wave_label contra a tabela completa de docs/cli-parity.md.

    Node.js já tinha este teste de tabela via isValidWaveLabel (barrier.test.js).
    Go cobre via TestWaveLabelGrammar_ValidAndInvalid usando parseWaves.
    Este teste fecha a mesma lacuna em Python: verifica a função diretamente,
    garantindo que a implementação nativa (re.fullmatch + int>=0) corresponde
    ao contrato em todos os cinco rótulos inválidos e seis válidos da tabela.

    "0" é a convenção da Wave 0 de modelo de ameaça (docs/cli-parity.md §
    "Wave label grammar"; ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-
    harness, ML-1A) — passou de inválido para válido.
    """
    from trackfw.commands.barrier import _is_valid_wave_label

    valid = ["0", "1", "2", "2-bis", "2-hotfix", "10-a2"]
    invalid = ["X", "2-BIS", "-bis", "2-", "2-bis-ter"]

    for lbl in valid:
        assert _is_valid_wave_label(lbl), (
            f"_is_valid_wave_label({lbl!r}) deve retornar True (rótulo válido per contrato)"
        )

    for lbl in invalid:
        assert not _is_valid_wave_label(lbl), (
            f"_is_valid_wave_label({lbl!r}) deve retornar False (rótulo inválido per contrato)"
        )
