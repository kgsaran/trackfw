"""
test_commands_roadmap_discover.py — Testes para commands/roadmap.py e commands/discover.py
"""

import os
import sys
import tempfile
import unittest
import argparse
import pytest
from contextlib import contextmanager

from trackfw import config as cfg_module
from trackfw.generators.roadmap import generate_roadmap, move_roadmap
from trackfw.commands import roadmap as roadmap_cmd
from trackfw.commands import discover as discover_cmd


# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------

def _make_cfg(tmpdir: str, namespacing: str = "flat", agents=None) -> dict:
    cfg = cfg_module.defaults()
    cfg["roadmap_dir"] = os.path.join(tmpdir, "docs", "roadmaps")
    cfg["roadmap_namespacing"] = namespacing
    if agents is not None:
        cfg["agents"] = agents
    return cfg


# ---------------------------------------------------------------------------
# tests roadmap new
# ---------------------------------------------------------------------------

def _find_subparser(parser: argparse.ArgumentParser, name: str) -> argparse.ArgumentParser:
    for action in parser._actions:
        if isinstance(action, argparse._SubParsersAction):
            return action.choices[name]
    raise AssertionError(f"subparser {name!r} not found")


@contextmanager
def _cwd(path: str):
    old = os.getcwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(old)


def _roadmap_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="trackfw")
    subparsers = parser.add_subparsers(dest="command", metavar="COMMAND")
    roadmap_cmd.register(subparsers)
    return parser


def _write_req(root: str, filename: str = "REQ-demo.md") -> str:
    req_dir = os.path.join(root, "docs", "req")
    os.makedirs(req_dir, exist_ok=True)
    req_path = os.path.join(req_dir, filename)
    with open(req_path, "w", encoding="utf-8") as f:
        f.write(
            "---\nstatus: Open\n---\n\n"
            "# REQ: Checkout seguro\n\n"
            "**ADR:** docs/adr/ADR-demo.md\n\n"
            "## Acceptance Criteria\n\n"
            "- [ ] Cart validates payment token\n"
            "- [x] Receipt is persisted\n"
        )
    return req_path


def test_roadmap_new_help_exposes_go_node_parity_flags():
    parser = _roadmap_parser()

    roadmap_parser = _find_subparser(parser, "roadmap")
    new_parser = _find_subparser(roadmap_parser, "new")
    help_text = new_parser.format_help()

    for flag in ("--title", "--req", "--from-req"):
        assert flag in help_text, f"Python roadmap new help missing parity flag {flag}; help:\n{help_text}"


def test_roadmap_new_accepts_valid_title_req_and_from_req_arguments(capsys):
    parser = _roadmap_parser()

    args = parser.parse_args(["roadmap", "new", "Checkout", "--req", "docs/req/REQ-demo.md"])
    assert args.title == "Checkout"
    assert args.req == "docs/req/REQ-demo.md"

    args = parser.parse_args(["roadmap", "new", "--title", "Billing", "--from-req", "docs/req/REQ-demo.md"])
    assert args.title_flag == "Billing"
    assert args.from_req == "docs/req/REQ-demo.md"


def test_roadmap_new_req_flag_generates_artifact_with_link(capsys):
    with tempfile.TemporaryDirectory() as tmpdir, _cwd(tmpdir):
        cfg_module.reset()
        parser = _roadmap_parser()
        args = parser.parse_args(["roadmap", "new", "--title", "Billing Flow", "--req", "docs/req/REQ-billing.md"])

        args.func(args)

        out = capsys.readouterr().out
        assert "Roadmap criado:" in out
        path = os.path.join(tmpdir, "docs", "roadmaps", "backlog")
        files = os.listdir(path)
        assert files == [f for f in files if f.endswith(".md")]
        assert len(files) == 1
        with open(os.path.join(path, files[0]), encoding="utf-8") as f:
            content = f.read()
        assert "# Roadmap: Billing Flow" in content
        assert "REQ: docs/req/REQ-billing.md" in content
        cfg_module.reset()


def test_roadmap_new_title_conflict_uses_positional_before_title_flag(capsys):
    with tempfile.TemporaryDirectory() as tmpdir, _cwd(tmpdir):
        cfg_module.reset()
        parser = _roadmap_parser()
        args = parser.parse_args(["roadmap", "new", "Positional Title", "--title", "Flag Title"])

        args.func(args)

        files = os.listdir(os.path.join(tmpdir, "docs", "roadmaps", "backlog"))
        with open(os.path.join(tmpdir, "docs", "roadmaps", "backlog", files[0]), encoding="utf-8") as f:
            content = f.read()
        assert "# Roadmap: Positional Title" in content
        assert "Flag Title" not in content
        cfg_module.reset()


def test_roadmap_new_from_req_takes_precedence_over_other_inputs(capsys):
    with tempfile.TemporaryDirectory() as tmpdir, _cwd(tmpdir):
        req_path = _write_req(tmpdir)
        cfg_module.reset()
        parser = _roadmap_parser()
        args = parser.parse_args([
            "roadmap",
            "new",
            "Ignored Positional",
            "--title",
            "Ignored Flag",
            "--req",
            "docs/req/REQ-other.md",
            "--from-req",
            req_path,
        ])

        args.func(args)

        files = os.listdir(os.path.join(tmpdir, "docs", "roadmaps", "backlog"))
        with open(os.path.join(tmpdir, "docs", "roadmaps", "backlog", files[0]), encoding="utf-8") as f:
            content = f.read()
        assert "# Roadmap: Checkout seguro" in content
        assert "REQ-other.md" not in content
        assert "Cart validates payment token" in content
        assert "Receipt is persisted" in content
        cfg_module.reset()


class TestRoadmapNew(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_roadmap_new_flat(self):
        """Cria roadmap em backlog flat via generator (sem CLI parse)."""
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Nova Feature", cfg)

        self.assertTrue(os.path.isfile(path))
        backlog_dir = os.path.join(cfg["roadmap_dir"], "backlog")
        self.assertEqual(os.path.dirname(path), backlog_dir)
        self.assertTrue(os.path.basename(path).endswith(".md"))
        self.assertIn("nova-feature", os.path.basename(path))

        with open(path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: backlog", content)
        self.assertIn("# Roadmap: Nova Feature", content)

    def test_roadmap_new_by_agent(self):
        """Cria roadmap com --agent zeus em modo by_agent."""
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        path = generate_roadmap("Auth Refactor", cfg, agent="zeus")

        self.assertTrue(os.path.isfile(path))
        expected_dir = os.path.join(cfg["roadmap_dir"], "zeus", "backlog")
        self.assertEqual(os.path.dirname(path), expected_dir)

    def test_roadmap_new_by_agent_sem_agent_usa_primeiro(self):
        """Modo by_agent sem agente explícito usa o primeiro da lista."""
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["apolo", "zeus"])
        path = generate_roadmap("Pipeline Deploy", cfg)

        expected_dir = os.path.join(cfg["roadmap_dir"], "apolo", "backlog")
        self.assertEqual(os.path.dirname(path), expected_dir)


# ---------------------------------------------------------------------------
# tests roadmap move
# ---------------------------------------------------------------------------

class TestRoadmapMove(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_roadmap_move(self):
        """Move roadmap de backlog para wip."""
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("Move Test", cfg)
        basename = os.path.basename(src_path)

        dst_path = move_roadmap(basename, "wip", cfg)

        self.assertTrue(os.path.isfile(dst_path))
        self.assertFalse(os.path.isfile(src_path))

        wip_dir = os.path.join(cfg["roadmap_dir"], "wip")
        self.assertEqual(os.path.dirname(dst_path), wip_dir)

        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: wip", content)

    def test_roadmap_move_estado_invalido(self):
        """Move com estado inválido levanta ValueError."""
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("X", cfg)
        basename = os.path.basename(src_path)

        with self.assertRaises(ValueError):
            move_roadmap(basename, "inexistente", cfg)

    def test_roadmap_move_arquivo_nao_encontrado(self):
        """Move de arquivo inexistente levanta FileNotFoundError."""
        cfg = _make_cfg(self.tmpdir)
        with self.assertRaises(FileNotFoundError):
            move_roadmap("nao-existe.md", "wip", cfg)


# ---------------------------------------------------------------------------
# tests roadmap list (integração via _list_flat e _list_by_agent)
# ---------------------------------------------------------------------------

class TestRoadmapList(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_list_flat_retorna_entradas(self):
        """_list_flat retorna entradas para roadmaps criados."""
        cfg = _make_cfg(self.tmpdir)
        generate_roadmap("Alpha", cfg)
        generate_roadmap("Beta", cfg)

        roadmap_dir = cfg["roadmap_dir"]
        entries = roadmap_cmd._list_flat(roadmap_dir)

        self.assertEqual(len(entries), 2)
        states = {e[0] for e in entries}
        self.assertIn("backlog", states)

    def test_list_by_agent_retorna_entradas(self):
        """_list_by_agent retorna entradas agrupadas por agente."""
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus", "apolo"])
        generate_roadmap("RM1", cfg, agent="zeus")
        generate_roadmap("RM2", cfg, agent="apolo")

        roadmap_dir = cfg["roadmap_dir"]
        entries = roadmap_cmd._list_by_agent(roadmap_dir, agents=["zeus", "apolo"])

        self.assertEqual(len(entries), 2)
        agents_found = {e[1] for e in entries}
        self.assertIn("zeus", agents_found)
        self.assertIn("apolo", agents_found)

    def test_list_flat_filtro_por_estado(self):
        """_list_flat com filter_state retorna apenas roadmaps naquele estado."""
        cfg = _make_cfg(self.tmpdir)
        src = generate_roadmap("WipItem", cfg)
        move_roadmap(os.path.basename(src), "wip", cfg)
        generate_roadmap("BacklogItem", cfg)

        roadmap_dir = cfg["roadmap_dir"]
        entries_wip = roadmap_cmd._list_flat(roadmap_dir, filter_state="wip")
        entries_backlog = roadmap_cmd._list_flat(roadmap_dir, filter_state="backlog")

        self.assertEqual(len(entries_wip), 1)
        self.assertEqual(len(entries_backlog), 1)

    def test_list_flat_encontra_analyzing(self):
        """_list_flat encontra roadmaps em analyzing/."""
        cfg = _make_cfg(self.tmpdir)
        src = generate_roadmap("Analyze Item", cfg)
        move_roadmap(os.path.basename(src), "analyzing", cfg)

        entries = roadmap_cmd._list_flat(cfg["roadmap_dir"], filter_state="analyzing")

        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0][0], "analyzing")
        self.assertIn("analyze-item", entries[0][2])

    def test_list_by_agent_encontra_analyzing(self):
        """_list_by_agent encontra roadmaps em analyzing/ preservando agente."""
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        src = generate_roadmap("Analyze Agent", cfg, agent="zeus")
        move_roadmap(os.path.basename(src), "analyzing", cfg)

        entries = roadmap_cmd._list_by_agent(
            cfg["roadmap_dir"],
            filter_state="analyzing",
            agents=["zeus"],
        )

        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0][0], "analyzing")
        self.assertEqual(entries[0][1], "zeus")


# ---------------------------------------------------------------------------
# tests roadmap show
# ---------------------------------------------------------------------------

class TestRoadmapShow(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_find_file_flat(self):
        """_find_file encontra arquivo pelo nome exato em modo flat."""
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Show Me", cfg)
        basename = os.path.basename(path)

        found = roadmap_cmd._find_file(
            basename,
            cfg["roadmap_dir"],
            "flat",
        )
        self.assertIsNotNone(found)
        self.assertEqual(found, path)

    def test_find_file_partial_match(self):
        """_find_file aceita match parcial de nome."""
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Partial Match Feature", cfg)

        found = roadmap_cmd._find_file(
            "partial-match",
            cfg["roadmap_dir"],
            "flat",
        )
        self.assertIsNotNone(found)

    def test_find_file_nao_encontrado(self):
        """_find_file retorna None quando nao ha match."""
        cfg = _make_cfg(self.tmpdir)
        generate_roadmap("Something Else", cfg)

        found = roadmap_cmd._find_file(
            "inexistente-xyz",
            cfg["roadmap_dir"],
            "flat",
        )
        self.assertIsNone(found)

    def test_find_file_flat_em_analyzing(self):
        """_find_file encontra roadmap em analyzing/ no modo flat."""
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Show Analyze", cfg)
        moved = move_roadmap(os.path.basename(path), "analyzing", cfg)

        found = roadmap_cmd._find_file(
            "show-analyze",
            cfg["roadmap_dir"],
            "flat",
        )

        self.assertEqual(found, moved)

    def test_find_file_by_agent_em_analyzing(self):
        """_find_file encontra roadmap em analyzing/ no modo by_agent."""
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        path = generate_roadmap("Show Analyze Agent", cfg, agent="zeus")
        moved = move_roadmap(os.path.basename(path), "analyzing", cfg)

        found = roadmap_cmd._find_file(
            "show-analyze-agent",
            cfg["roadmap_dir"],
            "by_agent",
            agents=["zeus"],
        )

        self.assertEqual(found, moved)


# ---------------------------------------------------------------------------
# tests discover scan
# ---------------------------------------------------------------------------

class TestDiscoverScan(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def test_discover_scan_detecta_estrutura(self):
        """Scan em dir com docs/adr/ e docs/roadmaps/ retorna counts corretos."""
        # Cria estrutura de docs
        adr_dir = os.path.join(self.tmpdir, "docs", "adr")
        os.makedirs(adr_dir, exist_ok=True)
        open(os.path.join(adr_dir, "ADR-001.md"), "w").close()
        open(os.path.join(adr_dir, "ADR-002.md"), "w").close()

        req_dir = os.path.join(self.tmpdir, "docs", "req")
        os.makedirs(req_dir, exist_ok=True)
        open(os.path.join(req_dir, "REQ-001.md"), "w").close()

        roadmap_backlog = os.path.join(self.tmpdir, "docs", "roadmaps", "backlog")
        os.makedirs(roadmap_backlog, exist_ok=True)
        open(os.path.join(roadmap_backlog, "ROADMAP-001.md"), "w").close()

        r = discover_cmd.scan(self.tmpdir)

        self.assertEqual(r["adr_count"], 2)
        self.assertEqual(r["req_count"], 1)
        self.assertEqual(r["roadmap_count"], 1)
        self.assertEqual(r["roadmap_namespacing"], "flat")
        self.assertEqual(r["req_dir"], "docs/req")
        self.assertIn("docs/adr", r["adr_dirs"])

    def test_discover_scan_by_agent(self):
        """Scan detecta by_agent quando subdirs de agente têm pastas de estado."""
        zeus_backlog = os.path.join(self.tmpdir, "docs", "roadmaps", "zeus", "backlog")
        apolo_wip = os.path.join(self.tmpdir, "docs", "roadmaps", "apolo", "wip")
        os.makedirs(zeus_backlog, exist_ok=True)
        os.makedirs(apolo_wip, exist_ok=True)
        open(os.path.join(zeus_backlog, "RM-001.md"), "w").close()

        r = discover_cmd.scan(self.tmpdir)

        self.assertEqual(r["roadmap_namespacing"], "by_agent")
        self.assertIn("zeus", r["agents"])
        self.assertIn("apolo", r["agents"])
        self.assertEqual(r["roadmap_count"], 1)

    def test_discover_scan_score_zerado_sem_artefatos(self):
        """Score 0 quando não há nenhum artefato."""
        r = discover_cmd.scan(self.tmpdir)
        self.assertEqual(r["governance_score"], 0)

    def test_discover_scan_score_parcial(self):
        """Score 40 quando há ADRs e REQs."""
        adr_dir = os.path.join(self.tmpdir, "docs", "adr")
        req_dir = os.path.join(self.tmpdir, "docs", "req")
        os.makedirs(adr_dir, exist_ok=True)
        os.makedirs(req_dir, exist_ok=True)
        open(os.path.join(adr_dir, "ADR-001.md"), "w").close()
        open(os.path.join(req_dir, "REQ-001.md"), "w").close()

        r = discover_cmd.scan(self.tmpdir)
        self.assertEqual(r["governance_score"], 40)

    def test_discover_scan_detecta_github_actions(self):
        """Scan detecta .github/workflows como github-actions."""
        workflows = os.path.join(self.tmpdir, ".github", "workflows")
        os.makedirs(workflows, exist_ok=True)

        r = discover_cmd.scan(self.tmpdir)
        self.assertEqual(r["ci_system"], "github-actions")

    def test_discover_scan_detecta_lefthook(self):
        """Scan detecta lefthook.yml como hook framework."""
        open(os.path.join(self.tmpdir, "lefthook.yml"), "w").close()

        r = discover_cmd.scan(self.tmpdir)
        self.assertEqual(r["hook_framework"], "lefthook")


# ---------------------------------------------------------------------------
# tests discover --init
# ---------------------------------------------------------------------------

class TestDiscoverInit(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def test_discover_init_gera_yaml(self):
        """--init cria trackfw.yaml no diretório raiz."""
        adr_dir = os.path.join(self.tmpdir, "docs", "adr")
        os.makedirs(adr_dir, exist_ok=True)
        open(os.path.join(adr_dir, "ADR-001.md"), "w").close()

        r = discover_cmd.scan(self.tmpdir)
        yaml_content = discover_cmd.generate_yaml(r)

        yaml_path = os.path.join(self.tmpdir, "trackfw.yaml")
        with open(yaml_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)

        self.assertTrue(os.path.isfile(yaml_path))
        with open(yaml_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("governance_mode: lenient", content)
        self.assertIn("adr_dirs:", content)
        self.assertIn("roadmap_namespacing:", content)

    def test_generate_yaml_conteudo_correto(self):
        """generate_yaml produz YAML com todos os campos esperados."""
        r = discover_cmd.scan(self.tmpdir)
        r["adr_dirs"] = ["docs/adr/zeus", "docs/adr/apolo"]
        r["req_dir"] = "docs/req"
        r["roadmap_dir"] = "docs/roadmaps"
        r["roadmap_namespacing"] = "by_agent"
        r["agents"] = ["zeus", "apolo"]
        r["hook_framework"] = "lefthook"
        r["ci_system"] = "github-actions"

        yaml_str = discover_cmd.generate_yaml(r)

        self.assertIn("governance_mode: lenient", yaml_str)
        self.assertIn("  - docs/adr/zeus", yaml_str)
        self.assertIn("  - docs/adr/apolo", yaml_str)
        self.assertIn("req_dir: docs/req", yaml_str)
        self.assertIn("roadmap_dir: docs/roadmaps", yaml_str)
        self.assertIn("roadmap_namespacing: by_agent", yaml_str)
        self.assertIn("  - zeus", yaml_str)
        self.assertIn("  - apolo", yaml_str)
        self.assertIn("hooks: lefthook", yaml_str)
        self.assertIn("ci: github-actions", yaml_str)


# ---------------------------------------------------------------------------
# tests discover bootstrap_log
# ---------------------------------------------------------------------------

class TestDiscoverBootstrapLog(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def test_generate_bootstrap_log_flat(self):
        """generate_bootstrap_log gera entradas para arquivos em done/ flat."""
        done_dir = os.path.join(self.tmpdir, "docs", "roadmaps", "done")
        os.makedirs(done_dir, exist_ok=True)
        open(os.path.join(done_dir, "ROADMAP-2026-01-01-feature-x.md"), "w").close()
        open(os.path.join(done_dir, "ROADMAP-2026-02-01-feature-y.md"), "w").close()

        r = {
            "roadmap_dir": "docs/roadmaps",
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        log = discover_cmd.generate_bootstrap_log(r, self.tmpdir)

        self.assertIn("backlog -> done", log)
        self.assertIn("ROADMAP-2026-01-01-feature-x.md", log)
        self.assertIn("ROADMAP-2026-02-01-feature-y.md", log)

    def test_generate_bootstrap_log_by_agent(self):
        """generate_bootstrap_log gera entradas com prefixo de agente."""
        zeus_done = os.path.join(self.tmpdir, "docs", "roadmaps", "zeus", "done")
        os.makedirs(zeus_done, exist_ok=True)
        open(os.path.join(zeus_done, "ROADMAP-2026-01-10-auth.md"), "w").close()

        r = {
            "roadmap_dir": "docs/roadmaps",
            "roadmap_namespacing": "by_agent",
            "agents": ["zeus"],
        }
        log = discover_cmd.generate_bootstrap_log(r, self.tmpdir)

        self.assertIn("zeus/ROADMAP-2026-01-10-auth.md", log)
        self.assertIn("backlog -> done", log)

    def test_generate_bootstrap_log_sem_done_retorna_vazio(self):
        """generate_bootstrap_log retorna string vazia quando done/ não existe."""
        roadmap_root = os.path.join(self.tmpdir, "docs", "roadmaps")
        os.makedirs(roadmap_root, exist_ok=True)

        r = {
            "roadmap_dir": "docs/roadmaps",
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        log = discover_cmd.generate_bootstrap_log(r, self.tmpdir)
        self.assertEqual(log, "")


# ---------------------------------------------------------------------------
# test registro do argparse
# ---------------------------------------------------------------------------

class TestRegister(unittest.TestCase):
    def test_roadmap_register(self):
        """register() adiciona subparser 'roadmap' sem erro."""
        parser = argparse.ArgumentParser()
        subparsers = parser.add_subparsers(dest="command")
        roadmap_cmd.register(subparsers)

        args = parser.parse_args(["roadmap", "list"])
        self.assertEqual(args.command, "roadmap")
        self.assertEqual(args.roadmap_cmd, "list")

    def test_discover_register(self):
        """register() adiciona subparser 'discover' com flags corretas."""
        parser = argparse.ArgumentParser()
        subparsers = parser.add_subparsers(dest="command")
        discover_cmd.register(subparsers)

        args = parser.parse_args(["discover", "--init"])
        self.assertEqual(args.command, "discover")
        self.assertTrue(args.init)
        self.assertFalse(args.bootstrap_log)

    def test_discover_register_bootstrap_log(self):
        """Flag --bootstrap-log é mapeada para args.bootstrap_log."""
        parser = argparse.ArgumentParser()
        subparsers = parser.add_subparsers(dest="command")
        discover_cmd.register(subparsers)

        args = parser.parse_args(["discover", "--bootstrap-log"])
        self.assertTrue(args.bootstrap_log)


if __name__ == "__main__":
    unittest.main()
