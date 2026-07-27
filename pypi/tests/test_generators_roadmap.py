"""
test_generators_roadmap.py — Testes unitários para generators/roadmap.py
"""

import os
import datetime
import tempfile
import unittest

from trackfw import config as cfg_module
from trackfw.generators.roadmap import (
    slugify,
    generate_roadmap,
    move_roadmap,
    _rewrite_roadmap_status,
    VALID_STATES,
)
from trackfw.validator import validate_folder_status_coherence


def _make_cfg(tmpdir: str, namespacing: str = "flat", agents=None) -> dict:
    """Cria config mínimo apontando para tmpdir."""
    cfg = cfg_module.defaults()
    cfg["roadmap_dir"] = os.path.join(tmpdir, "docs", "roadmaps")
    cfg["roadmap_namespacing"] = namespacing
    if agents is not None:
        cfg["agents"] = agents
    return cfg


class TestSlugify(unittest.TestCase):
    def test_lowercase(self):
        self.assertEqual(slugify("Hello World"), "hello-world")

    def test_special_chars(self):
        self.assertEqual(slugify("Feature: Auth & Login"), "feature-auth-login")

    def test_leading_trailing_hyphens(self):
        self.assertEqual(slugify("--test--"), "test")


class TestGenerateFlat(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_generate_flat(self):
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Minha Feature", cfg)

        self.assertTrue(os.path.isfile(path))

        # Deve estar em roadmap_dir/backlog/
        backlog_dir = os.path.join(cfg["roadmap_dir"], "backlog")
        self.assertEqual(os.path.dirname(path), backlog_dir)

        # Nome do arquivo contém slug e data
        basename = os.path.basename(path)
        today = datetime.date.today().isoformat()
        self.assertIn(today, basename)
        self.assertIn("minha-feature", basename)
        self.assertTrue(basename.endswith(".md"))

        # Conteúdo contém frontmatter e seção de wave
        with open(path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: backlog", content)
        self.assertIn("# Roadmap: Minha Feature", content)
        self.assertIn("## Wave 1", content)
        self.assertIn("ML-1A", content)


class TestGenerateByAgent(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_generate_by_agent(self):
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        path = generate_roadmap("Auth Redesign", cfg, agent="zeus")

        self.assertTrue(os.path.isfile(path))

        # Deve estar em roadmap_dir/zeus/backlog/
        expected_dir = os.path.join(cfg["roadmap_dir"], "zeus", "backlog")
        self.assertEqual(os.path.dirname(path), expected_dir)

    def test_generate_by_agent_usa_primeiro_agente_configurado(self):
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["apolo", "zeus"])
        path = generate_roadmap("API Gateway", cfg)

        # Sem agent explícito, usa o primeiro da lista
        expected_dir = os.path.join(cfg["roadmap_dir"], "apolo", "backlog")
        self.assertEqual(os.path.dirname(path), expected_dir)


class TestMoveBacklogParaWip(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_move_backlog_para_wip(self):
        cfg = _make_cfg(self.tmpdir)

        # Cria roadmap em backlog
        src_path = generate_roadmap("Deploy Pipeline", cfg)
        basename = os.path.basename(src_path)

        # Move para wip
        dst_path = move_roadmap(basename, "wip", cfg)

        # Arquivo de destino existe
        self.assertTrue(os.path.isfile(dst_path))
        # Arquivo de origem não existe mais
        self.assertFalse(os.path.isfile(src_path))

        # Está em wip/
        wip_dir = os.path.join(cfg["roadmap_dir"], "wip")
        self.assertEqual(os.path.dirname(dst_path), wip_dir)

        # Frontmatter atualizado
        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: wip", content)

    def test_move_estado_invalido_levanta_exception(self):
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("X", cfg)
        basename = os.path.basename(src_path)

        with self.assertRaises(ValueError):
            move_roadmap(basename, "inexistente", cfg)

    def test_move_arquivo_nao_encontrado_levanta_exception(self):
        cfg = _make_cfg(self.tmpdir)

        with self.assertRaises(FileNotFoundError):
            move_roadmap("nao-existe.md", "wip", cfg)

    def test_log_gravado_apos_move(self):
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("Log Test", cfg)
        basename = os.path.basename(src_path)

        move_roadmap(basename, "done", cfg)

        log_path = os.path.join(cfg["roadmap_dir"], ".trackfw-log")
        self.assertTrue(os.path.isfile(log_path))
        with open(log_path, encoding="utf-8") as f:
            log_content = f.read()
        self.assertIn("backlog", log_content)
        self.assertIn("done", log_content)
        self.assertIn(basename, log_content)


class TestMoveBuscaEmTodosAgentes(unittest.TestCase):
    """
    Em modo by_agent, move_roadmap deve encontrar o arquivo mesmo sem
    saber em qual agente ele está.
    """

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_move_busca_em_todos_agentes(self):
        cfg = _make_cfg(
            self.tmpdir,
            namespacing="by_agent",
            agents=["zeus", "apolo"],
        )

        # Cria roadmap no agente zeus/backlog
        src_path = generate_roadmap("Infra Refactor", cfg, agent="zeus")
        basename = os.path.basename(src_path)

        # Move para wip sem especificar agente — deve encontrar em zeus/backlog
        dst_path = move_roadmap(basename, "wip", cfg)

        self.assertTrue(os.path.isfile(dst_path))
        self.assertFalse(os.path.isfile(src_path))

        # Deve estar em zeus/wip/ (preserva o agente)
        expected_dir = os.path.join(cfg["roadmap_dir"], "zeus", "wip")
        self.assertEqual(os.path.dirname(dst_path), expected_dir)

        # Frontmatter atualizado
        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: wip", content)


class TestRewriteRoadmapStatus(unittest.TestCase):
    """Testes unitários para _rewrite_roadmap_status."""

    def test_sem_frontmatter_retorna_inalterado(self):
        src = "# Roadmap sem frontmatter\n\nTexto simples.\n"
        result, changed = _rewrite_roadmap_status(src, "wip")
        self.assertFalse(changed)
        self.assertEqual(result, src)

    def test_sem_chave_status_retorna_inalterado(self):
        src = "---\ndate: 2026-01-01\n---\n# Roadmap\n"
        result, changed = _rewrite_roadmap_status(src, "wip")
        self.assertFalse(changed)
        self.assertEqual(result, src)

    def test_reescreve_status_minusculo(self):
        src = "---\nstatus: backlog\ndate: 2026-01-01\n---\n# Roadmap\n\n> Created: 2026-01-01 | Status: backlog\n"
        result, changed = _rewrite_roadmap_status(src, "wip")
        self.assertTrue(changed)
        self.assertIn("status: wip", result)
        self.assertIn("| Status: wip", result)

    def test_preserva_aspas(self):
        src = '---\nstatus: "backlog"\ndate: 2026-01-01\n---\n# Roadmap\n'
        result, changed = _rewrite_roadmap_status(src, "wip")
        self.assertTrue(changed)
        self.assertIn('status: "wip"', result)

    def test_status_no_corpo_nao_e_tocado(self):
        src = (
            "---\nstatus: backlog\ndate: 2026-01-01\n---\n"
            "# Roadmap\n\n"
            "> Created: 2026-01-01 | Status: backlog\n\n"
            "## Context\n\n"
            "| Campo | status: backlog |\n"
            "|-------|----------------|\n\n"
            "```\n"
            "> Created: 2026-01-01 | Status: backlog\n"
            "```\n"
        )
        result, changed = _rewrite_roadmap_status(src, "wip")
        self.assertTrue(changed)
        # Frontmatter atualizado
        self.assertIn("status: wip", result)
        # Cabeçalho antes do ## atualizado
        self.assertIn("| Status: wip", result)
        # Tabela no corpo intocada
        self.assertIn("| Campo | status: backlog |", result)
        # Bloco de código (após ##) intocado
        self.assertIn("```\n> Created: 2026-01-01 | Status: backlog\n```", result)


class TestMoveRoadmapFrontmatterSync(unittest.TestCase):
    """Testes que verificam que move_roadmap sincroniza o frontmatter corretamente."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_move_sincroniza_status_minusculo(self):
        """status: no frontmatter deve ficar minúsculo após move (não 'WIP', não 'Done')."""
        cfg = _make_cfg(self.tmpdir)
        src_path = generate_roadmap("Frontmatter Sync Test", cfg)
        basename = os.path.basename(src_path)

        dst_path = move_roadmap(basename, "wip", cfg)

        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        # Deve ser minúsculo (bytes idênticos nos 3 CLIs)
        self.assertIn("status: wip", content)
        # Cabeçalho também deve ter | Status: wip
        self.assertIn("| Status: wip", content)

    def test_move_backlog_wip_done_sem_warning_folder_status_p4(self):
        """P4: validate após move backlog→wip→done não gera warning folder_status."""
        cfg = _make_cfg(self.tmpdir)

        # Criar e mover roadmap real
        src_path = generate_roadmap("P4 Validate Test", cfg)
        basename = os.path.basename(src_path)
        wip_path = move_roadmap(basename, "wip", cfg)
        done_path = move_roadmap(basename, "done", cfg)

        # Controle positivo: arquivo em wip com status: backlog DEVE gerar warning
        wip_dir = os.path.join(cfg["roadmap_dir"], "wip")
        control_content = "---\nstatus: backlog\ndate: 2026-01-01\n---\n# Roadmap: Control\n\n> Created: 2026-01-01 | Status: backlog\n"
        control_path = os.path.join(wip_dir, "ROADMAP-control.md")
        with open(control_path, "w", encoding="utf-8") as f:
            f.write(control_content)

        warnings = validate_folder_status_coherence(cfg)
        warning_msgs = [w["message"] if isinstance(w, dict) else w for w in warnings]

        # O roadmap movido NÃO deve gerar warning
        moved_warnings = [m for m in warning_msgs if "p4-validate-test" in m or os.path.basename(done_path) in m]
        self.assertEqual(len(moved_warnings), 0,
            f"roadmap movido gerou warning folder_status inesperado: {moved_warnings}")

        # O controle positivo DEVE gerar warning (garante que o validador está inspecionando)
        control_warnings = [m for m in warning_msgs if "ROADMAP-control.md" in m and "folder" in m]
        self.assertGreater(len(control_warnings), 0,
            f"controle positivo não gerou warning — validador pode não estar inspecionando; warnings: {warning_msgs}")

    def test_move_arquivo_sem_frontmatter_conteudo_intacto(self):
        """Arquivo sem frontmatter: move funciona, nenhuma chave inventada, conteúdo inalterado."""
        cfg = _make_cfg(self.tmpdir)
        backlog_dir = os.path.join(cfg["roadmap_dir"], "backlog")
        os.makedirs(backlog_dir, exist_ok=True)

        plain_content = "# Roadmap sem frontmatter\n\nConteúdo simples sem bloco ---.\n"
        road_path = os.path.join(backlog_dir, "ROADMAP-no-fm.md")
        with open(road_path, "w", encoding="utf-8") as f:
            f.write(plain_content)

        dst_path = move_roadmap("ROADMAP-no-fm.md", "wip", cfg)

        with open(dst_path, encoding="utf-8") as f:
            content = f.read()
        self.assertEqual(content, plain_content,
            "conteúdo do arquivo sem frontmatter foi alterado após move")

    def test_move_corpo_com_status_no_corpo_intacto(self):
        """status: no corpo e | Status: em seção após ## não são tocados."""
        cfg = _make_cfg(self.tmpdir)
        backlog_dir = os.path.join(cfg["roadmap_dir"], "backlog")
        os.makedirs(backlog_dir, exist_ok=True)

        body = (
            "---\nstatus: backlog\ndate: 2026-01-01\n---\n"
            "# Roadmap: Body Scope Test\n\n"
            "> Criado em: 2026-01-01 | Status: ⬜ Backlog\n\n"
            "## Context\n\n"
            "| Campo | status: backlog |\n"
            "|-------|----------------|\n\n"
            "```\n"
            "> Created: 2026-01-01 | Status: backlog\n"
            "```\n"
        )
        road_path = os.path.join(backlog_dir, "ROADMAP-body-scope.md")
        with open(road_path, "w", encoding="utf-8") as f:
            f.write(body)

        dst_path = move_roadmap("ROADMAP-body-scope.md", "wip", cfg)

        with open(dst_path, encoding="utf-8") as f:
            content = f.read()

        # Frontmatter sincronizado
        self.assertIn("status: wip", content)
        # Cabeçalho PT-BR sincronizado (antes do ## )
        self.assertIn("| Status: wip", content)
        # Tabela no corpo intocada
        self.assertIn("| Campo | status: backlog |", content)
        # Bloco de código (após ## ) intocado
        self.assertIn("```\n> Created: 2026-01-01 | Status: backlog\n```", content)


if __name__ == "__main__":
    unittest.main()
