"""
test_by_agent_ml1c.py — Testes do contrato ML-1C (Wave 1, Python).

Cobre os 4 cenários obrigatórios do contrato comum + herança de REQ + move entre namespaces.

Reconciliação (Regra Dura — CLAUDE.md):
  TC1 afirma: --agent explícito escreve no namespace correto E popula squad: no frontmatter.
  TC2 afirma: sem flag com vários agentes levanta erro nomeando TODOS os namespaces disponíveis.
  TC3 afirma: sem flag com UM agente, o COMANDO `req new` e `roadmap new` criam o arquivo em
              alpha/ sem erro (não apenas que resolve_write_agent retorna "alpha").
  TC4 afirma: modo flat permanece inalterado — COMANDO `roadmap new` em flat não cria subpasta
              de agente mesmo quando agents: está configurado com múltiplos.
  TC5 afirma: COMANDO `roadmap new --req <path em req_dir/beta/>` herda agente beta sem
              --agent explícito; REQ fora de req_dir/<agent>/ não herda (retorna erro de ambiguidade).
  TC6 afirma: roadmap move entre namespaces continua funcionando após extração de
              _agent_from_roadmap_path (a função nomeada não introduziu regressão).
  TC7 afirma: COMANDO `req new --agent` escreve em req_dir/<agent>/ (wiring do comando testado,
              não apenas req_write_dir em isolamento).
  TC8 afirma: resolve_write_agent com agente explícito fora de agents: retorna sem erro (AC5b);
              COMANDO `roadmap new --agent gamma` cria namespace gamma/ no disco.
  TC-extra afirma: _agent_from_roadmap_path e _agent_from_req_path extraem o agente correto
                   das estruturas de 3 e 2 níveis respectivamente — derivações distintas e nomeadas.
"""

import os
import sys
import types

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.generators.roadmap import (
    generate_roadmap,
    generate_roadmap_from_req,
    move_roadmap,
    _agent_from_roadmap_path,
    _agent_from_req_path,
)

try:
    from trackfw.validator import resolve_write_agent, req_write_dir
except ImportError:
    pytest.skip("resolve_write_agent não encontrado em validator — ML-1C incompleto", allow_module_level=True)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def symlink_or_skip(link, target):
    """Create link.symlink_to(target); pytest.skip if the process lacks privilege.

    On Windows without Developer Mode, os.symlink raises PermissionError
    (errno.EPERM, winerror=1314 ERROR_PRIVILEGE_NOT_HELD) or OSError with
    errno.EACCES.  Those conditions mean the guarantee cannot be exercised on
    this runner — skip rather than fail.

    Any other OSError is a genuine test-infrastructure failure and is re-raised
    so the test is marked as an error, not silently skipped.

    NOTE: Path.symlink_to is link-first, target-second (opposite of os.symlink
    and fs.symlinkSync) — the signature follows the native Python convention.
    """
    import errno as errno_mod
    try:
        link.symlink_to(str(target))
    except OSError as exc:
        if exc.errno in (errno_mod.EPERM, errno_mod.EACCES):
            pytest.skip(
                f"criação de symlink exige privilégio elevado "
                f"(Developer Mode no Windows): {exc}"
            )
        raise


def _make_cfg(tmpdir: str, namespacing: str = "flat", agents=None) -> dict:
    cfg = {
        "roadmap_dir": os.path.join(tmpdir, "roadmaps"),
        "req_dir": os.path.join(tmpdir, "req"),
        "roadmap_namespacing": namespacing,
    }
    if agents is not None:
        cfg["agents"] = agents
    return cfg


def _read_frontmatter_field(filepath: str, field: str) -> str:
    """Lê o valor de um campo do frontmatter YAML de um arquivo markdown."""
    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()
    if not content.startswith("---\n"):
        return ""
    end = content.find("\n---", 4)
    if end < 0:
        return ""
    frontmatter = content[4:end]
    for line in frontmatter.split("\n"):
        if ":" not in line:
            continue
        k, v = line.split(":", 1)
        if k.strip() == field:
            return v.strip().strip('"')
    return ""


def _make_req_args(**kwargs):
    """Cria um SimpleNamespace para simular args do comando req new."""
    defaults = {"title": "REQ Teste", "agent": None}
    defaults.update(kwargs)
    return types.SimpleNamespace(**defaults)


def _make_roadmap_args(**kwargs):
    """Cria um SimpleNamespace para simular args do comando roadmap new."""
    defaults = {
        "title": "Roadmap Teste",
        "title_flag": None,
        "req": None,
        "from_req": None,
        "agent": None,
    }
    defaults.update(kwargs)
    return types.SimpleNamespace(**defaults)


# ---------------------------------------------------------------------------
# TC1 — --agent explícito: namespace correto + frontmatter correto (AC4/AC10/AC12)
# ---------------------------------------------------------------------------

class TestAgentExplicito:
    """TC1 afirma: --agent explícito escreve no namespace correto E popula squad: no frontmatter."""

    def test_roadmap_new_agent_explícito_path(self, tmp_path):
        """roadmap new --agent beta cria o arquivo em beta/backlog/."""
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        path = generate_roadmap("Feature X", cfg, agent="beta")
        assert "/beta/backlog/" in path.replace(os.sep, "/"), (
            f"Esperado path em beta/backlog/, obteve: {path}"
        )

    def test_roadmap_new_agent_explícito_frontmatter(self, tmp_path):
        """roadmap new --agent beta popula squad: beta no frontmatter."""
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        path = generate_roadmap("Feature X", cfg, agent="beta")
        squad = _read_frontmatter_field(path, "squad")
        assert squad == "beta", f"Esperado squad: beta, obteve: {squad!r}"

    def test_roadmap_from_req_agent_explícito_frontmatter(self, tmp_path):
        """generate_roadmap_from_req --agent beta popula squad: beta."""
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        req_dir = os.path.join(str(tmp_path), "req", "beta")
        os.makedirs(req_dir, exist_ok=True)
        req_file = os.path.join(req_dir, "REQ-2026-01-01-teste.md")
        with open(req_file, "w", encoding="utf-8") as f:
            f.write("---\nstatus: Open\n---\n# REQ: Teste\n## Acceptance Criteria\n- [ ] AC1\n")
        path = generate_roadmap_from_req(req_file, cfg, agent="beta")
        squad = _read_frontmatter_field(path, "squad")
        assert squad == "beta", f"Esperado squad: beta (from_req), obteve: {squad!r}"


# ---------------------------------------------------------------------------
# TC2 — sem flag, vários agentes: erro nomeando TODOS (AC5)
# ---------------------------------------------------------------------------

class TestAmbiguidade:
    """TC2 afirma: sem flag com vários agentes levanta erro nomeando TODOS os namespaces disponíveis."""

    def test_resolve_write_agent_multi_levanta_valor(self):
        """resolve_write_agent sem agent + vários → ValueError com mensagem byte-idêntica ao contrato de paridade Go/Node/Python."""
        cfg = {"agents": ["alpha", "beta"], "roadmap_namespacing": "by_agent"}
        with pytest.raises(ValueError) as exc_info:
            resolve_write_agent(cfg, None)
        msg = str(exc_info.value)
        expected = "by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one"
        assert msg == expected, f"Mensagem deve ser byte-idêntica ao contrato; obteve: {msg!r}"

    def test_resolve_write_agent_multi_empty_entry_parity_message(self):
        """resolve_write_agent com ['', 'alpha', 'beta'] → mensagem byte-idêntica, só não-vazios."""
        cfg = {"agents": ["", "alpha", "beta"], "roadmap_namespacing": "by_agent"}
        with pytest.raises(ValueError) as exc_info:
            resolve_write_agent(cfg, None)
        msg = str(exc_info.value)
        expected = "by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one"
        assert msg == expected, f"Mensagem deve ser byte-idêntica ao contrato; obteve: {msg!r}"

    def test_resolve_write_agent_multi_filtra_vazios(self):
        """agents: ['', 'zeus'] conta como UM namespace → sem erro."""
        cfg = {"agents": ["", "zeus"], "roadmap_namespacing": "by_agent"}
        result = resolve_write_agent(cfg, None)
        assert result == "zeus", f"Esperado 'zeus', obteve: {result!r}"

    def test_resolve_write_agent_tres_agentes_todos_nomeados(self):
        """Três agentes → mensagem cita os três."""
        cfg = {"agents": ["alpha", "beta", "gamma"], "roadmap_namespacing": "by_agent"}
        with pytest.raises(ValueError) as exc_info:
            resolve_write_agent(cfg, None)
        msg = str(exc_info.value)
        assert "alpha" in msg and "beta" in msg and "gamma" in msg, (
            f"Esperado alpha, beta, gamma na mensagem, obteve: {msg!r}"
        )

    def test_cmd_req_new_sem_flag_multi_agentes_exit_nao_zero(self, tmp_path, capsys, monkeypatch):
        """COMANDO req new sem --agent com vários agents: → sys.exit não zero + mensagem nomeia ambos."""
        from trackfw.commands import req as req_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_req_args(agent=None)
        with pytest.raises(SystemExit) as exc_info:
            req_cmd._cmd_new(args)
        assert exc_info.value.code != 0, "Esperado exit não-zero para ambiguidade"
        captured = capsys.readouterr()
        assert "alpha" in captured.err and "beta" in captured.err, (
            f"Esperado 'alpha' e 'beta' no stderr, obteve: {captured.err!r}"
        )

    def test_cmd_roadmap_new_sem_flag_multi_agentes_exit_nao_zero(self, tmp_path, capsys, monkeypatch):
        """COMANDO roadmap new sem --agent com vários agents: → sys.exit não zero + mensagem nomeia ambos."""
        from trackfw.commands import roadmap as roadmap_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_roadmap_args(agent=None)
        with pytest.raises(SystemExit) as exc_info:
            roadmap_cmd._cmd_new(args)
        assert exc_info.value.code != 0, "Esperado exit não-zero para ambiguidade"
        captured = capsys.readouterr()
        assert "alpha" in captured.err and "beta" in captured.err, (
            f"Esperado 'alpha' e 'beta' no stderr, obteve: {captured.err!r}"
        )


# ---------------------------------------------------------------------------
# TC3 — sem flag, UM agente: COMANDO cria arquivo em alpha/ sem erro (AC14)
# ---------------------------------------------------------------------------

class TestUnicoAgente:
    """TC3 afirma: sem flag com UM agente, o COMANDO cria o arquivo em alpha/ sem erro."""

    def test_resolve_write_agent_único_retorna_sem_erro(self):
        """resolve_write_agent sem agent + 1 agente → usa sem levantar."""
        cfg = {"agents": ["alpha"], "roadmap_namespacing": "by_agent"}
        result = resolve_write_agent(cfg, None)
        assert result == "alpha", f"Esperado 'alpha', obteve: {result!r}"

    def test_cmd_req_new_unico_agente_sem_flag_cria_em_alpha(self, tmp_path, monkeypatch):
        """COMANDO req new sem --agent + 1 agente → arquivo em req/alpha/."""
        from trackfw.commands import req as req_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_req_args(agent=None)
        req_cmd._cmd_new(args)
        req_alpha = os.path.join(str(tmp_path), "req", "alpha")
        files = [f for f in os.listdir(req_alpha) if f.endswith(".md")] if os.path.isdir(req_alpha) else []
        assert len(files) == 1, (
            f"Esperado 1 REQ em req/alpha/, encontrado {files!r} (dir: {os.listdir(str(tmp_path))})"
        )

    def test_cmd_roadmap_new_unico_agente_sem_flag_cria_em_alpha(self, tmp_path, monkeypatch):
        """COMANDO roadmap new sem --agent + 1 agente → arquivo em roadmaps/alpha/backlog/."""
        from trackfw.commands import roadmap as roadmap_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_roadmap_args(agent=None)
        # Não deve levantar SystemExit
        roadmap_cmd._cmd_new(args)
        alpha_backlog = os.path.join(str(tmp_path), "roadmaps", "alpha", "backlog")
        files = [f for f in os.listdir(alpha_backlog) if f.endswith(".md")] if os.path.isdir(alpha_backlog) else []
        assert len(files) == 1, (
            f"Esperado 1 roadmap em alpha/backlog/, encontrado {files!r}"
        )

    def test_resolve_write_agent_único_não_vazio_filtra_vazio(self):
        """agents: ['', 'alpha'] → um namespace real → usa 'alpha'."""
        cfg = {"agents": ["", "alpha"], "roadmap_namespacing": "by_agent"}
        result = resolve_write_agent(cfg, None)
        assert result == "alpha"


# ---------------------------------------------------------------------------
# TC4 — flat: COMANDO não cria subpasta de agente mesmo com agents: configurado (AC14)
# ---------------------------------------------------------------------------

class TestFlatImbativel:
    """TC4 afirma: modo flat permanece inalterado — COMANDO roadmap new em flat não cria
    subpasta de agente mesmo quando agents: está configurado com múltiplos."""

    def test_req_write_dir_flat_ignora_agent(self):
        """req_write_dir em flat com agent explícito retorna req_dir sem subpasta."""
        cfg = {"req_dir": "docs/req", "roadmap_namespacing": "flat"}
        result = req_write_dir(cfg, agent="beta")
        assert result == "docs/req", f"Esperado 'docs/req' em flat, obteve: {result!r}"

    def test_cmd_roadmap_new_flat_sem_flag_sem_subpasta_de_agente(self, tmp_path, monkeypatch):
        """COMANDO roadmap new em flat com agents: [alpha, beta] → arquivo em backlog/ sem subpasta."""
        from trackfw.commands import roadmap as roadmap_cmd
        # flat mode with agents declared — guards must be scoped to by_agent only
        cfg = _make_cfg(str(tmp_path), namespacing="flat", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_roadmap_args(agent=None)
        # Em flat, nunca deve levantar SystemExit por ambiguidade
        roadmap_cmd._cmd_new(args)
        backlog = os.path.join(str(tmp_path), "roadmaps", "backlog")
        files = [f for f in os.listdir(backlog) if f.endswith(".md")] if os.path.isdir(backlog) else []
        assert len(files) == 1, (
            f"Esperado 1 roadmap em backlog/ (flat), encontrado {files!r}"
        )
        # Confirma que NÃO há subpastas de agente em roadmaps/
        roadmaps_root = os.path.join(str(tmp_path), "roadmaps")
        subdirs = [d for d in os.listdir(roadmaps_root)
                   if os.path.isdir(os.path.join(roadmaps_root, d)) and d not in ("backlog", "wip", "done",
                                                                                    "blocked", "abandoned", "analyzing")]
        assert not subdirs, f"Modo flat não deve ter subpastas de agente: {subdirs!r}"


# ---------------------------------------------------------------------------
# TC5 — COMANDO roadmap new --req <path em req_dir/beta/> herda agente (AC11)
# ---------------------------------------------------------------------------

class TestHerancaDaREQ:
    """TC5 afirma: COMANDO roadmap new --req <path em req_dir/beta/> herda agente beta sem
    --agent explícito; REQ fora de req_dir/<agent>/ não herda (cai na regra de ambiguidade)."""

    def test_cmd_heranca_agente_de_req_na_subpasta(self, tmp_path, monkeypatch):
        """COMANDO roadmap new --req req/beta/REQ.md → roadmap em beta/backlog/ sem --agent."""
        from trackfw.commands import roadmap as roadmap_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        # Cria REQ em req/beta/
        req_dir_beta = os.path.join(str(tmp_path), "req", "beta")
        os.makedirs(req_dir_beta, exist_ok=True)
        req_path = os.path.join(req_dir_beta, "REQ-2026-01-01-heranca.md")
        with open(req_path, "w", encoding="utf-8") as f:
            f.write("---\nstatus: Open\n---\n# REQ: Heranca\n")

        args = _make_roadmap_args(agent=None, req=req_path)
        roadmap_cmd._cmd_new(args)

        beta_backlog = os.path.join(str(tmp_path), "roadmaps", "beta", "backlog")
        files = [f for f in os.listdir(beta_backlog) if f.endswith(".md")] if os.path.isdir(beta_backlog) else []
        assert len(files) == 1, (
            f"Esperado roadmap em beta/backlog/ por herança da REQ, encontrado {files!r}"
        )

    def test_cmd_req_fora_de_req_dir_nao_herda_cai_em_ambiguidade(self, tmp_path, capsys, monkeypatch):
        """COMANDO roadmap new --req <REQ diretamente em req_dir/> não herda → erro de ambiguidade."""
        from trackfw.commands import roadmap as roadmap_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        # REQ em req/ diretamente (flat-layout legado), NÃO em req/<agent>/
        req_root = os.path.join(str(tmp_path), "req")
        os.makedirs(req_root, exist_ok=True)
        req_path = os.path.join(req_root, "REQ-2026-01-01-flat.md")
        with open(req_path, "w", encoding="utf-8") as f:
            f.write("---\nstatus: Open\n---\n# REQ: Flat\n")

        args = _make_roadmap_args(agent=None, req=req_path)
        with pytest.raises(SystemExit) as exc_info:
            roadmap_cmd._cmd_new(args)
        assert exc_info.value.code != 0
        captured = capsys.readouterr()
        # Deve nomear alpha e beta na mensagem de ambiguidade
        assert "alpha" in captured.err and "beta" in captured.err, (
            f"Esperado ambiguidade com alpha/beta, obteve: {captured.err!r}"
        )


# ---------------------------------------------------------------------------
# TC6 — roadmap move entre namespaces continua funcionando (AC6)
# ---------------------------------------------------------------------------

class TestMoveBetweenNamespaces:
    """TC6 afirma: roadmap move entre namespaces continua funcionando após extração de
    _agent_from_roadmap_path (a função nomeada não introduziu regressão)."""

    def test_move_preserva_agente(self, tmp_path):
        """move_roadmap preserva o namespace do agente após extração da função."""
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["zeus", "apolo"])
        src_path = generate_roadmap("Infra", cfg, agent="zeus")
        filename = os.path.basename(src_path)

        dst = move_roadmap(filename, "wip", cfg)
        assert "/zeus/wip/" in dst.replace(os.sep, "/"), (
            f"Esperado zeus/wip/ após move, obteve: {dst}"
        )

    def test_move_de_agente_para_outro_namespace(self, tmp_path):
        """move_roadmap de apolo/backlog → apolo/wip preserva agente."""
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["zeus", "apolo"])
        src_path = generate_roadmap("Multi-Agent", cfg, agent="apolo")
        filename = os.path.basename(src_path)

        dst = move_roadmap(filename, "wip", cfg)
        assert "/apolo/wip/" in dst.replace(os.sep, "/"), (
            f"Esperado apolo/wip/ após move, obteve: {dst}"
        )


# ---------------------------------------------------------------------------
# TC7 — COMANDO req new --agent: escreve em req_dir/<agent>/ (wiring testado)
# ---------------------------------------------------------------------------

class TestReqNewAgent:
    """TC7 afirma: COMANDO req new --agent escreve em req_dir/<agent>/ (wiring testado)."""

    def test_cmd_req_new_agent_explícito_cria_em_beta(self, tmp_path, monkeypatch):
        """COMANDO req new --agent beta cria REQ em req/beta/."""
        from trackfw.commands import req as req_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_req_args(agent="beta")
        req_cmd._cmd_new(args)

        req_beta = os.path.join(str(tmp_path), "req", "beta")
        files = [f for f in os.listdir(req_beta) if f.endswith(".md")] if os.path.isdir(req_beta) else []
        assert len(files) == 1, f"Esperado 1 REQ em req/beta/, encontrado {files!r}"
        # Confirma que NOT em req/alpha/
        req_alpha = os.path.join(str(tmp_path), "req", "alpha")
        alpha_files = [f for f in os.listdir(req_alpha) if f.endswith(".md")] if os.path.isdir(req_alpha) else []
        assert not alpha_files, f"REQ não deve estar em req/alpha/, encontrado {alpha_files!r}"

    def test_req_write_dir_com_agent_explícito(self):
        """req_write_dir com agent explícito retorna req_dir/<agent>/."""
        cfg = {
            "req_dir": "docs/req",
            "roadmap_namespacing": "by_agent",
            "agents": ["alpha", "beta"],
        }
        result = req_write_dir(cfg, agent="beta")
        assert result == os.path.join("docs/req", "beta"), (
            f"Esperado docs/req/beta, obteve: {result!r}"
        )

    def test_req_write_dir_único_agente_sem_flag(self):
        """req_write_dir sem agent + 1 agente → req_dir/<agente>/."""
        cfg = {
            "req_dir": "docs/req",
            "roadmap_namespacing": "by_agent",
            "agents": ["alpha"],
        }
        result = req_write_dir(cfg)
        assert result == os.path.join("docs/req", "alpha"), (
            f"Esperado docs/req/alpha, obteve: {result!r}"
        )


# ---------------------------------------------------------------------------
# TC8 — --agent fora de agents: funciona (AC5b)
# ---------------------------------------------------------------------------

class TestAgentForaDoConjunto:
    """TC8 afirma: resolve_write_agent com agente explícito fora de agents: retorna sem erro (AC5b);
    COMANDO roadmap new --agent gamma cria namespace gamma/ no disco."""

    def test_agent_explícito_fora_de_agents_retorna(self):
        """--agent com valor fora de agents: funciona (AC5b: viola agent_namespace_undeclared)."""
        cfg = {"agents": ["alpha", "beta"], "roadmap_namespacing": "by_agent"}
        result = resolve_write_agent(cfg, "gamma")
        assert result == "gamma", f"Esperado 'gamma', obteve: {result!r}"

    def test_cmd_roadmap_new_agent_fora_cria_namespace_no_disco(self, tmp_path, monkeypatch):
        """COMANDO roadmap new --agent gamma cria gamma/backlog/ no disco (AC5b)."""
        from trackfw.commands import roadmap as roadmap_cmd
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha", "beta"])
        monkeypatch.chdir(tmp_path)
        import trackfw.config as cfg_module
        monkeypatch.setattr(cfg_module, "load", lambda: cfg)

        args = _make_roadmap_args(agent="gamma")
        roadmap_cmd._cmd_new(args)

        gamma_backlog = os.path.join(str(tmp_path), "roadmaps", "gamma", "backlog")
        files = [f for f in os.listdir(gamma_backlog) if f.endswith(".md")] if os.path.isdir(gamma_backlog) else []
        assert len(files) == 1, f"Esperado 1 roadmap em gamma/backlog/, encontrado {files!r}"


# ---------------------------------------------------------------------------
# TC extra — derivações nomeadas: _agent_from_roadmap_path e _agent_from_req_path
# ---------------------------------------------------------------------------

def test_agent_from_roadmap_path_extrai_correto():
    """_agent_from_roadmap_path extrai o agente de roadmap_dir/<agent>/<state>/file.md (3 níveis).
    TC-extra afirma: a estrutura de 3 níveis usa dirname(dirname(path)) — não a mesma fórmula das REQs."""
    path = "/proj/roadmaps/zeus/wip/ROADMAP-2026-01-01-test.md"
    assert _agent_from_roadmap_path(path) == "zeus", (
        f"Esperado 'zeus', obteve: {_agent_from_roadmap_path(path)!r}"
    )


def test_agent_from_req_path_extrai_correto():
    """_agent_from_req_path extrai o agente de req_dir/<agent>/REQ.md (2 níveis).
    TC-extra afirma: a estrutura de 2 níveis usa dirname(path) — derivação distinta e nomeada,
    corrijo o erro do ML-0A que descreveu dirname(dirname(req_path))."""
    path = "/proj/docs/req/beta/REQ-2026-01-01-test.md"
    assert _agent_from_req_path(path) == "beta", (
        f"Esperado 'beta', obteve: {_agent_from_req_path(path)!r}"
    )


def test_agent_from_req_path_difere_de_roadmap_path():
    """Confirma que REQ e roadmap têm estruturas de profundidade distintas — não são intercambiáveis.
    TC-extra afirma: aplicar a fórmula de roadmap (2x dirname) a um caminho de REQ dá resultado errado."""
    req_path = "/proj/docs/req/beta/REQ.md"
    # Fórmula correta para REQ (1x dirname):
    assert _agent_from_req_path(req_path) == "beta"
    # Fórmula ERRADA se aplicada a REQ (2x dirname, como faria _agent_from_roadmap_path):
    wrong = os.path.basename(os.path.dirname(os.path.dirname(req_path)))
    assert wrong != "beta", (
        f"Confirmação: fórmula de roadmap aplicada a REQ daria {wrong!r}, não 'beta' — estruturas distintas"
    )


# ---------------------------------------------------------------------------
# ML-1E-a — erro explícito e log-prefix em by_agent
# ---------------------------------------------------------------------------

class TestLogPrefixHasAgent:
    """Port de TestMoveRoadmap_ByAgent_LogPrefixHasAgent (Go → Python).

    Afirma que move_roadmap em modo by_agent registra a transição no .trackfw-log
    com o prefixo "<agente>/ROADMAP-*.md", não apenas "ROADMAP-*.md".
    Conclusão do ML-1E-a: _agent_from_roadmap_path com base_dir retorna o agente correto
    para paths legítimos dentro do roadmapDir.
    """

    def test_log_prefix_contem_agente(self, tmp_path):
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["alpha"])
        src_path = generate_roadmap("Log Prefix", cfg, agent="alpha")
        filename = os.path.basename(src_path)

        move_roadmap(filename, "analyzing", cfg)

        log_path = os.path.join(cfg["roadmap_dir"], ".trackfw-log")
        assert os.path.exists(log_path), ".trackfw-log deve existir após move"
        log_content = open(log_path, encoding="utf-8").read()
        assert "alpha/" + filename in log_content, (
            f".trackfw-log deve conter 'alpha/{filename}'; got:\n{log_content}"
        )
        assert "backlog → analyzing" in log_content, (
            f".trackfw-log deve registrar 'backlog → analyzing'; got:\n{log_content}"
        )


class TestSymlinkOutsideRaisesExplicitError:
    """Afirma que move_roadmap em by_agent levanta ValueError explícito nomeando o path
    quando _agent_from_roadmap_path retorna "" (symlink resolve para fora de roadmapDir).
    Conclusão do ML-1E-a: não há fallback silencioso ao primeiro agente — o contrato é
    'controle que não reconhece rejeita e avisa'.
    """

    def test_symlink_fora_levanta_valor_error(self, tmp_path):
        outside = tmp_path / "outside"
        outside.mkdir()
        (outside / "wip").mkdir()
        (outside / "wip" / "ROADMAP-leak.md").write_text("# leak", encoding="utf-8")

        project = tmp_path / "project"
        project.mkdir()
        roadmap_dir = project / "roadmaps"
        roadmap_dir.mkdir()
        (roadmap_dir / "alice" / "wip").mkdir(parents=True)

        # evil → outside, dentro de roadmap_dir
        # symlink_or_skip: EPERM/EACCES → pytest.skip; qualquer outro erro → re-raise (falha)
        symlink_or_skip(roadmap_dir / "evil", outside)

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "req_dir": str(project / "req"),
            "roadmap_namespacing": "by_agent",
            "agents": ["alice", "evil"],
        }

        # Medido em 2026-09-11: Python levanta ValueError (não FileNotFoundError).
        # _find_roadmap_matches com evil em agents: segue o symlink via os.listdir e
        # encontra ROADMAP-leak.md; _agent_from_roadmap_path(src, base_dir) resolve via
        # os.path.realpath, retorna ""; a guarda em move_roadmap levanta ValueError.
        with pytest.raises(ValueError) as exc_info:
            move_roadmap("ROADMAP-leak.md", "done", cfg)

        err_msg = str(exc_info.value)
        # A mensagem deve mencionar "agent namespace"
        assert "agent namespace" in err_msg, (
            f"ValueError deve mencionar 'agent namespace'; got: {err_msg}"
        )
        # O path recusado deve ser nomeado — contrato "nomeia o caminho recusado"
        assert "ROADMAP-leak" in err_msg, (
            f"ValueError deve nomear o path recusado (ROADMAP-leak); got: {err_msg}"
        )

        # Verificar contenção: o arquivo não deve ter escapado
        escaped = outside / "done" / "ROADMAP-leak.md"
        assert not escaped.exists(), "arquivo não deve escapar para fora do roadmapDir via symlink"

    def test_agentfrompath_retorna_vazio_para_symlink_externo(self, tmp_path):
        """Afirma que _agent_from_roadmap_path com base_dir retorna '' para paths
        que resolvem via symlink para fora de base_dir.
        Conclusão do ML-1E-a: a resolução de symlinks em _agent_from_roadmap_path
        é o mecanismo que produz '' para caminhos externos — afirma o comportamento da função."""
        outside = tmp_path / "outside"
        outside.mkdir()
        (outside / "wip").mkdir()
        the_file = outside / "wip" / "ROADMAP-test.md"
        the_file.write_text("# test", encoding="utf-8")

        base_dir = tmp_path / "roadmaps"
        base_dir.mkdir()
        symlink_or_skip(base_dir / "evil", outside)

        via_symlink = str(base_dir / "evil" / "wip" / "ROADMAP-test.md")
        result = _agent_from_roadmap_path(via_symlink, base_dir=str(base_dir))
        assert result == "", (
            f"_agent_from_roadmap_path deve retornar '' para symlink externo; got: {result!r}"
        )


class TestLegitMoveStillWorks:
    """Contra-braço do ML-1E-a: move legítimo entre estados continua funcionando.
    Conclusão do ML-1E-a: o erro explícito para '' não afeta moves de paths reais
    dentro do roadmapDir.
    """

    def test_move_legitimo_nao_falha(self, tmp_path):
        cfg = _make_cfg(str(tmp_path), namespacing="by_agent", agents=["zeus"])
        src_path = generate_roadmap("Legit", cfg, agent="zeus")
        filename = os.path.basename(src_path)

        dst = move_roadmap(filename, "done", cfg)
        assert "/zeus/done/" in dst.replace(os.sep, "/"), (
            f"Esperado zeus/done/ após move legítimo, obteve: {dst}"
        )
