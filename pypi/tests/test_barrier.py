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
import shutil
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
# Fixture helper — $PATH curado com um git resolvível (ML-1E)
# ────────────────────────────────────────────────────────────────────────────

def _place_executable_in_path(src: str, bin_dir: str) -> str:
    """Disponibiliza ``src`` dentro de ``bin_dir`` e devolve o caminho criado.

    O nome de destino preserva o basename da origem — no Windows isso mantém o
    ``.exe`` que o PATHEXT exige; um arquivo chamado só ``git`` nunca é
    resolvido pelo CreateProcess, e o subprocess do produto morre com
    FileNotFoundError/ENOENT (a classe corrigida no Go pelo ML-1A).

    O symlink é a forma primária nos dois sistemas. No POSIX porque copiar é
    regressão medida: o ``/usr/bin/git`` do macOS é um shim assinado que morre
    fora do próprio diretório. No Windows porque o processo passa a rodar com o
    caminho do alvo, então o wrapper do Git for Windows continua encontrando sua
    instalação. Hardlink e cópia entram só se o symlink for negado (WinError
    1314 sem Developer Mode; ``os.link`` recusa entre volumes) — e a cópia é o
    único ramo em que o wrapper pode deixar de achar ``mingw64/``, por isso a
    sonda abaixo nomeia o mecanismo.

    Coloca exatamente um arquivo: acrescentar o *diretório* do git ao PATH
    resolveria o PATHEXT, mas no Windows o ``git.exe`` pode estar em
    ``Git/usr/bin``, que também contém ``sh.exe`` — e destruiria em silêncio o
    poder discriminante do teste que exige git presente e sh ausente.
    """
    dst = str(Path(bin_dir) / os.path.basename(src))
    mechanism = "symlink"
    try:
        os.symlink(src, dst)
    except OSError:
        try:
            os.link(src, dst)
            mechanism = "hardlink"
        except OSError:
            shutil.copy2(src, dst)
            mechanism = "copy"
    # Sonda: um destino colocado mas inexecutável faria o produto falhar de um
    # jeito indistinguível do bug que este remendo corrige. A falha tem que
    # nomear a fixture, não o produto.
    try:
        probe = subprocess.run([dst, "--version"], capture_output=True)
        detail = f"returncode={probe.returncode}: {probe.stderr.decode('utf-8', errors='replace')}"
        ok = probe.returncode == 0
    except OSError as exc:  # ENOENT/EACCES ao spawnar o próprio destino
        detail, ok = f"{type(exc).__name__}: {exc}", False
    assert ok, f"fixture: git colocado por {mechanism} em {dst} não executa ({detail})"
    return dst


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


# exit 127 sinaliza "ferramenta ausente dentro do sh" — o sh iniciou e rodou.
# Nunca deve ser confundido com o próprio sh estar ausente (medido no ML-0A).
def test_gates_ferramenta_ausente_dentro_do_sh_e_exit_127_normal_nao_not_evaluated():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
        gate_commands=["nosuchtool-xyz"],
    )
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
    assert code == 1
    doc = json.loads(stdout)
    gates_check = next(c for c in doc["checks"] if c["name"] == "gates")
    assert gates_check["status"] == "blocked"
    assert gates_check["failures"] == ["nosuchtool-xyz: exit 127"]


def test_gates_sh_ausente_do_path_reporta_not_evaluated_com_mensagem_pinada():
    dir_ = _setup_dir(
        linked_req=True,
        ml_status="✅",
        criteria_lines=["- [x] build passes"],
        gate_commands=["true", "false"],
    )
    # Diretório curado com `git` (necessário por _roadmap_trust_for_gates, que
    # não faz parte do escopo deste ML) mas sem `sh` — simula a ausência
    # especificamente de `sh` no $PATH, não do ambiente inteiro.
    curated = tempfile.mkdtemp(prefix="tw-no-sh-")
    # shutil.which honra o PATHEXT: no Windows devolve `git.exe`, que a varredura
    # anterior (procurar um arquivo chamado literalmente "git") nunca encontrava —
    # o teste morria no próprio assert com "git not found in current $PATH".
    git_path = shutil.which("git")
    assert git_path is not None, "git not found in current $PATH — cannot build curated fixture"
    _place_executable_in_path(git_path, curated)
    try:
        stdout, stderr, code = _run_barrier_cli(
            dir_, "ROADMAP-barrier-fixture", "--wave", "1", "--json", curated_path=curated,
        )
        assert code == 1
        doc = json.loads(stdout)
        gates_check = next(c for c in doc["checks"] if c["name"] == "gates")
        assert gates_check["status"] == "not_evaluated"
        assert gates_check["evidence"] == []
        assert gates_check["failures"] == [
            "gates not evaluated: sh not found in PATH — install a POSIX shell "
            "(e.g. Git Bash, WSL) to evaluate gates"
        ]
    finally:
        shutil.rmtree(curated, ignore_errors=True)


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


# ────────────────────────────────────────────────────────────────────────────
# _status_is_complete — vocabulário por primeiro token (ADR decisão 3/4/8,
# AC8/AC9/AC14)
# ────────────────────────────────────────────────────────────────────────────

def test_status_is_complete_formas_aceitas():
    """Cobre as seis formas aceitas pinadas pelo ADR, incluindo as duas com
    sufixo que hoje passam só porque o casamento é substring (48 ocorrências
    no corpus) — precisam continuar passando sob primeiro-token."""
    from trackfw.commands.barrier import _status_is_complete

    aceitos = [
        "✅",
        "✅ Concluído",
        "✅ Concluído · **Agente:** `apolo-tf`",
        "✅ concluído (auditado 2026-08-02)",
        "done",
        "Concluído",
        "DONE",
        "concluido",
        "done\t· extra",  # tab após o marcador é separador válido
        "done\u00a0· extra",  # NBSP (U+00A0) após o marcador é separador válido
        "✅️",  # VS16 (U+FE0F) apresentação de emoji estilo texto — a única exceção Mn (ADR decisão 9)
    ]
    for marker in aceitos:
        assert _status_is_complete(marker), f"_status_is_complete({marker!r}) deveria ser True"


def test_status_is_complete_formas_rejeitadas():
    """AC9 falsificado na direção oposta — cada caso é um vetor nomeado
    explicitamente pelo modelo de ameaça da Wave 0. Ampliar o vocabulário sem
    trocar contains()->primeiro-token faria os quatro primeiros passarem
    (vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md)."""
    from trackfw.commands.barrier import _status_is_complete

    rejeitados = [
        "não done",
        "pending (era done)",
        "notdone",
        "done-not-really",
        "⬜ Pendente",
        "🔄 Em andamento",
        "❌ Bloqueado",
        "⬜ Pendente ✅",  # AC14 — posição importa; hoje (contains) isso passa em produção
        "`done`",  # marcador dentro de código inline — as crases grudam no token
        "\u200bdone",  # espaço de largura zero antes do token — não é whitespace, gruda
        "",
        "   ",
        "d᷀one",  # AC15 (ADR decisão 9) — marca combinante (U+1DC0) no primeiro token, rejeitada, não dobrada
        "do᷀ne",  # AC15 — mesma marca, em outro codepoint do token
        "done᷀",  # AC15 — mesma marca, ao final do token
        "✅᷀",  # AC15 — marca combinante sobre o próprio marcador de emoji, ainda rejeitada
    ]
    for marker in rejeitados:
        assert not _status_is_complete(marker), f"_status_is_complete({marker!r}) deveria ser False"


def test_acceptance_header_re_aceita_ingles_e_portugues():
    """AC1/AC2/AC3: o cabeçalho canônico em inglês e o em português devem
    casar, ambos ancorados."""
    from trackfw.commands.barrier import _ACCEPTANCE_HEADER_RE

    aceitos = [
        "**Acceptance criteria:**",
        "**Critérios de aceite:**",
        "**Criterios de aceite:**",
    ]
    for line in aceitos:
        assert _ACCEPTANCE_HEADER_RE.match(line), f"esperava casar {line!r}"

    # A âncora é o discriminante: citar o cabeçalho em prosa NÃO deve casar.
    rejeitados = [
        "o cabeçalho é **Acceptance criteria:**",
        "> **Critérios de aceite:**",
        "prosa citando **Acceptance criteria:** no meio da frase",
    ]
    for line in rejeitados:
        assert not _ACCEPTANCE_HEADER_RE.match(line), f"esperava NÃO casar {line!r} (âncora deve rejeitar citação no meio da linha)"


# ────────────────────────────────────────────────────────────────────────────
# Consciência de cerca de código (ADR decisão 7, AC13) — _find_mls/_ml_status/
# _ml_acceptance devem ignorar conteúdo dentro de cercas ```. Reproduz
# forged.md e forged3.md do resultado do ML-0A, na íntegra.
# ────────────────────────────────────────────────────────────────────────────

def test_fence_awareness_status_dentro_de_cerca_e_ignorado():
    """forged.md: um exemplo cercado citando "**Status:** done" não pode
    sombrear o "**Status:** pending" real fora da cerca."""
    from trackfw.commands.barrier import _fence_mask, _find_mls, _ml_status, _status_is_complete

    lines = [
        "### ML-1A — probe",
        "Example of the bug we are documenting:",
        "```",
        "**Status:** done",
        "```",
        "**Status:** pending",
    ]
    fenced = _fence_mask(lines)
    mls = _find_mls(lines, fenced, 0, len(lines))
    assert len(mls) == 1
    complete, marker = _ml_status(lines, fenced, mls[0]["start"], mls[0]["end"])
    assert marker == "pending", "esperava o status real, não cercado"
    assert not complete, "o \"done\" cercado não pode vazar para o status real"
    assert not _status_is_complete(marker)


def test_fence_awareness_bloco_de_aceite_dentro_de_cerca_e_ignorado():
    """forged3.md: um exemplo cercado citando "**Critérios de aceite:**" com
    "- [x]" não pode ser lido como o bloco de aceite real do ML quando não há
    bloco real fora dela."""
    from trackfw.commands.barrier import _fence_mask, _find_mls, _ml_acceptance

    lines = [
        "### ML-1A — probe",
        "Example of the bug we are documenting:",
        "```",
        "**Critérios de aceite:**",
        "- [x] fake evidence, nothing built",
        "```",
        "**Status:** ✅",
    ]
    fenced = _fence_mask(lines)
    mls = _find_mls(lines, fenced, 0, len(lines))
    assert len(mls) == 1
    block = _ml_acceptance(lines, fenced, mls[0]["start"], mls[0]["end"])
    assert block is None, "o bloco de aceite citado dentro da cerca não pode contar como evidência real"


def test_fence_awareness_cabecalho_ml_dentro_de_cerca_nao_vira_ml_fantasma():
    """AC13-b: um cabeçalho "### ML-XX" dentro de uma cerca não pode ser
    detectado como um ML real. Reproduzido ao vivo contra o binário 7.3.0
    (ADR: "### ML-9Z ... prosa; o barrier reporta 'ML-9Z: not complete'")."""
    from trackfw.commands.barrier import _fence_mask, _find_mls

    lines = [
        "## Wave 1 — Foo",
        "### ML-1A — Real ML",
        "**Status:** ✅",
        "**Critérios de aceite:**",
        "- [x] real criterion",
        "",
        "Example of a malformed heading inside a fence, cited as documentation:",
        "```markdown",
        "### ML-9Z — phantom, must not be detected",
        "**Status:** ⬜ Pendente",
        "```",
    ]
    fenced = _fence_mask(lines)
    mls = _find_mls(lines, fenced, 0, len(lines))
    assert len(mls) == 1, f"esperava 1 ML (o ML-9Z cercado não pode ser detectado), got {[m['id'] for m in mls]}"
    assert mls[0]["id"] == "ML-1A"


# ────────────────────────────────────────────────────────────────────────────
# Regressões end-to-end com o CLI real (AC1/AC12, AC13)
# ────────────────────────────────────────────────────────────────────────────

def _write_roadmap(dir_: Path, content: str, filename: str = "ROADMAP-regression.md") -> None:
    (dir_ / f"docs/roadmaps/wip/{filename}").write_text(content, encoding="utf-8")


def _setup_regression_dir() -> Path:
    dir_ = Path(tempfile.mkdtemp(prefix="tw-barrier-regression-"))
    for d in (
        "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
        "docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
    ):
        (dir_ / d).mkdir(parents=True, exist_ok=True)
    return dir_


def test_barrier_cli_cabecalho_ingles_e_status_por_palavra_passam_e2e():
    """Regressão ponta a ponta AC1/AC12: um roadmap escrito exatamente como o
    `roadmap new` escreve hoje (cabeçalho de aceite em inglês, status por
    palavra) deve passar mls_complete e acceptance_evidence com o binário
    real, sem editar o cabeçalho à mão."""
    dir_ = _setup_regression_dir()
    content = (
        "# Roadmap: English dialect fixture\n\n"
        "REQ: REQ-2026-08-29-barrier-fixture\n\n"
        "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"
        "## Wave 1 — Fixture Wave\n> Dependencies: none\n\n"
        "### ML-1A — Fixture ML\n"
        "**Status:** done\n"
        "**Acceptance criteria:**\n"
        "- [x] build passes\n"
    )
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 0, f"stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["status"] == "passed", checks["mls_complete"]
    assert checks["acceptance_evidence"]["status"] == "passed", checks["acceptance_evidence"]


def test_barrier_cli_conteudo_forjado_em_cerca_nao_libera_wave_e2e():
    """Regressão ponta a ponta para ADR decisão 7 (AC13): uma wave cujo único
    ML tem o status real, não cercado, "pending" (não concluído) e o único
    bloco de aceite cercado (forjado) deve continuar bloqueada no binário
    real — o "done" cercado e o "- [x]" cercado não podem vazar para
    mls_complete / acceptance_evidence."""
    dir_ = _setup_regression_dir()
    content = (
        "# Roadmap: Forged fence fixture\n\n"
        "REQ: REQ-2026-08-29-barrier-fixture\n\n"
        "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"
        "## Wave 1 — Fixture Wave\n> Dependencies: none\n\n"
        "### ML-1A — Fixture ML\n"
        "Example of the bug we are documenting:\n"
        "```\n"
        "**Status:** done\n"
        "**Critérios de aceite:**\n"
        "- [x] fake evidence, nothing built\n"
        "```\n"
        "**Status:** pending\n"
    )
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["status"] == "blocked"
    assert checks["mls_complete"]["failures"] == ["ML-1A: not complete (status: pending)"]
    assert checks["acceptance_evidence"]["status"] == "blocked"
    assert checks["acceptance_evidence"]["failures"] == ["ML-1A: no acceptance block"]


# ────────────────────────────────────────────────────────────────────────────
# ML-1B achado 1 — _fence_mask deve reconhecer ~~~ e cercas de 4+ crases, por
# CommonMark (3+ do MESMO caractere, fechada por uma corrida do mesmo
# caractere com comprimento >= o da abertura).
# ────────────────────────────────────────────────────────────────────────────

def test_fence_mask_tio_de_3_mais_masca_igual_a_crases():
    """~~~ (3+ tis) deve mascarar exatamente como ``` (ML-1B achado 1)."""
    from trackfw.commands.barrier import _fence_mask

    lines = ["before", "~~~", "inside", "~~~", "after"]
    assert _fence_mask(lines) == [False, False, True, False, False]


def test_fence_mask_4_crases_masca_interior_com_cerca_de_3_aninhada():
    """Uma cerca de 4 crases mascara o interior inteiro, inclusive um bloco
    de 3 crases aninhado (ML-1B achado 1)."""
    from trackfw.commands.barrier import _fence_mask

    lines = [
        "before",
        "````",
        "outer",
        "```",
        "nested (corrida mais curta, deve continuar mascarada como interior)",
        "```",
        "still outer",
        "````",
        "after",
    ]
    assert _fence_mask(lines) == [False, False, True, True, True, True, True, False, False]


def test_fence_mask_fechamento_exige_mesmo_caractere_e_comprimento_maior_ou_igual():
    """Uma linha ``` dentro de uma cerca ~~~ não a fecha (caractere diferente)."""
    from trackfw.commands.barrier import _fence_mask

    lines = ["~~~", "```", "still inside", "~~~"]
    assert _fence_mask(lines) == [False, True, True, False]


def test_fence_mask_fechamento_mais_longo_do_mesmo_caractere_fecha():
    """Uma corrida de fechamento MAIS LONGA do mesmo caractere fecha a cerca
    (comprimento >= o de abertura, per CommonMark)."""
    from trackfw.commands.barrier import _fence_mask

    lines = ["before", "```", "inside", "`````", "after"]
    assert _fence_mask(lines) == [False, False, True, False, False]


# ────────────────────────────────────────────────────────────────────────────
# ML-1B achado 2 — marcadores (status, cabeçalho de aceite, itens de
# critério, cabeçalho de gates) devem ser casados contra a linha CRUA
# (coluna 0), nunca uma linha stripada por linha — alinhando o Python (que já
# exige coluna 0 via `^`) e garantindo que o Node também exija.
# ────────────────────────────────────────────────────────────────────────────

def test_ml_status_linha_indentada_nao_e_reconhecida():
    from trackfw.commands.barrier import _fence_mask, _find_mls, _ml_status

    lines = ["### ML-1A — Real ML", "  **Status:** done"]
    fenced = _fence_mask(lines)
    mls = _find_mls(lines, fenced, 0, len(lines))
    assert len(mls) == 1
    complete, marker = _ml_status(lines, fenced, mls[0]["start"], mls[0]["end"])
    assert complete is False
    assert marker is None


def test_ml_acceptance_cabecalho_e_criterios_indentados_nao_sao_reconhecidos():
    from trackfw.commands.barrier import _fence_mask, _find_mls, _ml_acceptance

    lines = [
        "### ML-1A — Real ML",
        "  **Critérios de aceite:**",
        "  - [x] indented criterion",
    ]
    fenced = _fence_mask(lines)
    mls = _find_mls(lines, fenced, 0, len(lines))
    assert len(mls) == 1
    block = _ml_acceptance(lines, fenced, mls[0]["start"], mls[0]["end"])
    assert block is None


def test_barrier_cli_cerca_de_til_forjada_nao_libera_wave_e2e():
    """Teste de evasão (ML-1B): um ML fantasma escondido dentro de uma cerca
    ~~~ não pode existir nem contar como conteúdo real."""
    dir_ = _setup_regression_dir()
    content = (
        "# Roadmap: Tilde fence fixture\n\n"
        "REQ: REQ-2026-08-29-barrier-fixture\n\n"
        "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"
        "## Wave 1 — Fixture Wave\n> Dependencies: none\n\n"
        "### ML-1A — Real ML\n"
        "**Status:** ⬜ Pendente\n"
        "**Critérios de aceite:**\n"
        "- [ ] real unmet criterion\n\n"
        "Example of a phantom ML hidden inside a tilde fence:\n"
        "~~~\n"
        "### ML-9Z — phantom\n"
        "**Status:** done\n"
        "**Critérios de aceite:**\n"
        "- [x] fake\n"
        "~~~\n"
    )
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["failures"] == ["ML-1A: not complete (status: ⬜ Pendente)"], \
        "o ML-9Z fantasma não pode existir"


def test_barrier_cli_cerca_de_4_crases_com_bloco_de_3_aninhado_nao_libera_wave_e2e():
    """Teste de evasão (ML-1B): um ML fantasma aninhado em um bloco de 3
    crases dentro de uma cerca de 4 crases não pode existir nem contar."""
    dir_ = _setup_regression_dir()
    content = (
        "# Roadmap: Nested fence fixture\n\n"
        "REQ: REQ-2026-08-29-barrier-fixture\n\n"
        "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"
        "## Wave 1 — Fixture Wave\n> Dependencies: none\n\n"
        "### ML-1A — Real ML\n"
        "**Status:** ⬜ Pendente\n"
        "**Critérios de aceite:**\n"
        "- [ ] real unmet criterion\n\n"
        "Example nesting a 3-backtick fence inside a 4-backtick fence:\n"
        "````\n"
        "outer fence, then a nested doc block:\n"
        "```\n"
        "### ML-9Z — nested phantom\n"
        "**Status:** done\n"
        "**Critérios de aceite:**\n"
        "- [x] fake\n"
        "```\n"
        "still inside the outer fence\n"
        "````\n"
    )
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["failures"] == ["ML-1A: not complete (status: ⬜ Pendente)"], \
        "o ML-9Z fantasma não pode existir"


def test_barrier_cli_marcadores_indentados_nao_reconhecidos_e2e():
    """Regressão ponta a ponta (ML-1B achado 2): marcadores indentados por 2
    espaços não são reconhecidos — o veredito é o estrito (bloqueado)."""
    dir_ = _setup_regression_dir()
    content = (
        "# Roadmap: Indented marker fixture\n\n"
        "REQ: REQ-2026-08-29-barrier-fixture\n\n"
        "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"
        "## Wave 1 — Fixture Wave\n> Dependencies: none\n\n"
        "### ML-1A — Real ML\n"
        "  **Status:** done\n"
        "  **Critérios de aceite:**\n"
        "  - [x] indented criterion\n"
    )
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["status"] == "blocked"
    assert checks["mls_complete"]["failures"] == ["ML-1A: not complete (status: missing)"]
    assert checks["acceptance_evidence"]["status"] == "blocked"
    assert checks["acceptance_evidence"]["failures"] == ["ML-1A: no acceptance block"]


def test_find_gates_cabecalho_e_casamento_por_prefixo():
    """_GATES_HEADER_RE já é casamento por PREFIXO (não igualdade de linha
    inteira) — um cabeçalho seguido de prosa na mesma linha continua sendo
    reconhecido. Fecha a cobertura de paridade para o defeito que o Node
    introduziu brevemente ao remover seu .trim() por linha (ML-1B)."""
    from trackfw.commands.barrier import _find_gates

    lines = [
        "## Wave 1 — X",
        "**Gates da wave:** (obrigatórios)",
        "```bash",
        "make build",
        "```",
        "## Wave 2 — Y",
    ]
    commands = _find_gates(lines, 0, 5)
    assert commands == ["make build"]


def test_barrier_cli_cabecalho_de_gates_com_prosa_final_ainda_executa_o_gate_e2e():
    """Regressão ponta a ponta (ML-1B): '**Gates da wave:**' seguido de prosa
    na mesma linha continua sendo reconhecido — o gate `false` deve rodar e
    bloquear a wave, não ser silenciosamente ignorado."""
    dir_ = _setup_regression_dir()
    content = (
        "# Roadmap: Gates header prefix fixture\n\n"
        "REQ: REQ-2026-08-29-barrier-fixture\n\n"
        "## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n"
        "## Wave 1 — Fixture Wave\n\n"
        "**Gates da wave:** (obrigatórios)\n"
        "```bash\n"
        "false\n"
        "```\n\n"
        "### ML-1A — Real ML\n"
        "**Status:** done\n"
        "**Critérios de aceite:**\n"
        "- [x] build passes\n"
    )
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["gates"]["status"] == "blocked", checks["gates"]
    assert checks["gates"]["commands"] == ["false"]


# ────────────────────────────────────────────────────────────────────────────
# ML-3C — roadmaps CRLF. Achado em auditoria: um roadmap salvo com fins de
# linha CRLF reportava mls_complete: passed em Go e Python, mas blocked
# ("status: missing") em Node — o "." do regex de JS exclui "\r" (é um
# LineTerminator no ECMAScript), então `/^\*\*Status:\*\*(.*)$/` nunca casava
# com "**Status:** ✅ Concluído\r". Corrigido normalizando CRLF uma única vez,
# no limite onde o arquivo vira lista de linhas (_split_roadmap_lines), em vez
# de remendar cada regex de marcador.
#
# Python não dependia dessa normalização para passar um roadmap CRLF de ponta
# a ponta — `open(path, "r", encoding="utf-8")` já roda a tradução universal
# de newlines (newline=None por padrão) antes do content.split("\n"). A
# normalização explícita em _split_roadmap_lines é defensiva (mantém os três
# runtimes simétricos), não é o que faz este runtime passar hoje — por isso os
# testes abaixo exercitam o comportamento observável do CLI (que já
# funcionava), e o teste unitário isolado de _split_roadmap_lines documenta o
# contrato da função em si.
# ────────────────────────────────────────────────────────────────────────────

def test_split_roadmap_lines_remove_apenas_o_r_final_de_cada_linha():
    from trackfw.commands.barrier import _split_roadmap_lines

    got = _split_roadmap_lines("  **Status:** done\r\n**Status:** done\r\nlast\r")
    assert got == ["  **Status:** done", "**Status:** done", "last"]


def test_barrier_cli_crlf_roadmap_com_ml_completo_passa_e2e():
    dir_ = _setup_regression_dir()
    content = "\r\n".join([
        "# Roadmap: CRLF Fixture",
        "",
        "REQ: REQ-2026-08-29-barrier-fixture",
        "",
        "## Acceptance Criteria",
        "- [x] fixture roadmap-level criterion",
        "",
        "## Wave 1 — Fixture Wave",
        "",
        "### ML-1A — Real ML",
        "**Status:** ✅ Concluído",
        "**Critérios de aceite:**",
        "- [x] real met criterion",
        "",
    ])
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 0, f"esperava exit 0 (passed), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    assert doc["status"] == "passed"
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["status"] == "passed"
    assert checks["mls_complete"]["failures"] == []


def test_barrier_cli_crlf_roadmap_com_ml_pendente_continua_bloqueando_e2e():
    """Prova que a correção não é permissiva: um ML genuinamente pendente,
    num roadmap CRLF, continua bloqueando."""
    dir_ = _setup_regression_dir()
    content = "\r\n".join([
        "# Roadmap: CRLF Fixture",
        "",
        "REQ: REQ-2026-08-29-barrier-fixture",
        "",
        "## Acceptance Criteria",
        "- [x] fixture roadmap-level criterion",
        "",
        "## Wave 1 — Fixture Wave",
        "",
        "### ML-1A — Real ML",
        "**Status:** ⬜ Pendente",
        "**Critérios de aceite:**",
        "- [ ] real unmet criterion",
        "",
    ])
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["failures"] == ["ML-1A: not complete (status: ⬜ Pendente)"]


def test_barrier_cli_crlf_roadmap_cerca_e_marcador_indentado_continuam_corretos_e2e():
    """Roadmap CRLF combinando um ML fantasma dentro de cerca (deve ficar
    mascarado) e um marcador indentado (ML-1B — não pode ser reconhecido)."""
    dir_ = _setup_regression_dir()
    content = "\r\n".join([
        "# Roadmap: CRLF Fixture",
        "",
        "REQ: REQ-2026-08-29-barrier-fixture",
        "",
        "## Acceptance Criteria",
        "- [x] fixture roadmap-level criterion",
        "",
        "## Wave 1 — Fixture Wave",
        "",
        "### ML-1A — Real ML",
        "**Status:** ⬜ Pendente",
        "**Critérios de aceite:**",
        "- [ ] real unmet criterion",
        "",
        "Exemplo de ML fantasma dentro de uma cerca:",
        "```",
        "### ML-9Z — phantom",
        "**Status:** done",
        "**Critérios de aceite:**",
        "- [x] fake",
        "```",
        "",
        "### ML-1B — marcador indentado não pode contar",
        "  **Status:** done",
        "  **Critérios de aceite:**",
        "  - [x] indented criterion",
        "",
    ])
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["mls_complete"]["failures"] == [
        "ML-1A: not complete (status: ⬜ Pendente)",
        "ML-1B: not complete (status: missing)",
    ]


def test_barrier_cli_crlf_roadmap_gates_da_wave_e_reconhecido_e_comando_roda_e2e():
    dir_ = _setup_regression_dir()
    content = "\r\n".join([
        "# Roadmap: CRLF Fixture",
        "",
        "REQ: REQ-2026-08-29-barrier-fixture",
        "",
        "## Acceptance Criteria",
        "- [x] fixture roadmap-level criterion",
        "",
        "## Wave 1 — Fixture Wave",
        "",
        "**Gates da wave:**",
        "```bash",
        "false",
        "```",
        "",
        "### ML-1A — Real ML",
        "**Status:** done",
        "**Critérios de aceite:**",
        "- [x] build passes",
        "",
    ])
    _write_roadmap(dir_, content)
    stdout, stderr, code = _run_barrier_cli(dir_, "ROADMAP-regression", "--wave", "1", "--json")
    assert code == 1, f"esperava exit 1 (blocked pelo gate), stdout={stdout} stderr={stderr}"
    doc = json.loads(stdout)
    checks = {c["name"]: c for c in doc["checks"]}
    assert checks["gates"]["status"] == "blocked", checks["gates"]
    assert checks["gates"]["commands"] == ["false"]
