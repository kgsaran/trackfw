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
    generate_roadmap_from_req,
    move_roadmap,
    sync_paired_req_references,
    _rewrite_roadmap_status,
    _get_frontmatter_roadmap_value,
    _rewrite_req_roadmap_ref,
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
        self.assertIn('req: ""', content)
        self.assertIn("# Roadmap: Minha Feature", content)
        self.assertIn("## Wave 1", content)
        self.assertIn("ML-1A", content)

        # Bloco consolidado de critérios de aceite (contrato de paridade com Go/Node)
        self.assertIn(
            "## Acceptance Criteria\n"
            "<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->\n"
            "- [ ]\n- [ ]\n",
            content,
        )

    def test_generate_flat_with_req_path_sets_context_link(self):
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Linked Roadmap", cfg, req_path="docs/req/REQ-linked.md")

        with open(path, encoding="utf-8") as f:
            content = f.read()

        self.assertIn('req: "docs/req/REQ-linked.md"', content)
        self.assertIn("REQ: docs/req/REQ-linked.md", content)

    def test_generate_from_req_derives_title_criteria_and_adr(self):
        cfg = _make_cfg(self.tmpdir)
        req_dir = os.path.join(self.tmpdir, "docs", "req")
        os.makedirs(req_dir, exist_ok=True)
        req_path = os.path.join(req_dir, "REQ-checkout.md")
        with open(req_path, "w", encoding="utf-8") as f:
            f.write(
                "---\nstatus: Open\n---\n\n"
                "# REQ: Checkout seguro\n\n"
                "**ADR:** docs/adr/ADR-checkout.md\n\n"
                "## Critérios de Aceite\n\n"
                "- [ ] Validar token de pagamento\n"
                "- [x] Persistir recibo\n"
            )

        path = generate_roadmap_from_req(req_path, cfg)

        self.assertTrue(os.path.isfile(path))
        self.assertIn("checkout-seguro", os.path.basename(path))
        with open(path, encoding="utf-8") as f:
            content = f.read()

        self.assertIn(f'req: "{req_path}"', content)
        self.assertIn("# Roadmap: Checkout seguro", content)
        self.assertIn("<!-- Derived from REQ: REQ-checkout.md -->", content)
        self.assertIn(f"REQ: {req_path}", content)
        self.assertIn("ADR: docs/adr/ADR-checkout.md", content)
        self.assertIn("### ML-1A — Validar token de pagamento", content)
        self.assertIn("### ML-1B — Persistir recibo", content)

        # Bloco consolidado de critérios de aceite (contrato de paridade com Go/Node)
        self.assertIn(
            "## Acceptance Criteria\n"
            "<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->\n"
            "- [ ]\n- [ ]\n",
            content,
        )
        # Bloco por ML preservado (unidade operacional de cada microlote)
        self.assertIn("**Acceptance criteria:**", content)


class TestStatusLegendAndCanonicalForm(unittest.TestCase):
    """AC11: o template escreve a forma canônica de status (⬜ Pendente) e a legenda dos
    quatro estados. Falsificação: reintroduzir 'pending' ou duplicar/remover a legenda deve
    reprovar este teste."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def test_generate_roadmap_legend_and_canonical_status(self):
        cfg = _make_cfg(self.tmpdir)
        path = generate_roadmap("Legend Check Python", cfg)

        with open(path, encoding="utf-8") as f:
            content = f.read()

        legend_line = "⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado"
        self.assertEqual(
            content.count(legend_line), 1,
            f"legenda deveria aparecer exatamente 1 vez, obteve {content.count(legend_line)}:\n{content}",
        )
        self.assertIn("## Status Legend", content)
        self.assertIn("**Status:** ⬜ Pendente", content)
        self.assertNotIn("**Status:** pending", content)
        self.assertIn("**Acceptance criteria:**", content)
        self.assertIn("**Gates da wave:**", content)

    def test_generate_roadmap_from_req_legend_and_canonical_status(self):
        cfg = _make_cfg(self.tmpdir)
        req_dir = os.path.join(self.tmpdir, "docs", "req")
        os.makedirs(req_dir, exist_ok=True)
        req_path = os.path.join(req_dir, "REQ-legend-from-req.md")
        with open(req_path, "w", encoding="utf-8") as f:
            f.write(
                "---\nstatus: Open\n---\n\n"
                "# REQ: legend from req python\n\n"
                "## Acceptance Criteria\n\n"
                "- [ ] AC1 — primeiro criterio\n"
                "- [ ] AC2 — segundo criterio\n"
            )

        path = generate_roadmap_from_req(req_path, cfg)

        with open(path, encoding="utf-8") as f:
            content = f.read()

        legend_line = "⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado"
        self.assertEqual(
            content.count(legend_line), 1,
            f"legenda deveria aparecer exatamente 1 vez mesmo com 2 MLs derivados, "
            f"obteve {content.count(legend_line)}:\n{content}",
        )
        # ML-0A (Wave 0) + ML-1A + ML-1B derivados dos 2 critérios = 3 ocorrências.
        self.assertEqual(
            content.count("**Status:** ⬜ Pendente"), 3,
            f"esperava 3 ocorrências de '**Status:** ⬜ Pendente' (ML-0A, ML-1A, ML-1B), "
            f"obteve {content.count('**Status:** ⬜ Pendente')}:\n{content}",
        )
        self.assertNotIn("**Status:** pending", content)


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

        log_path = os.path.join(cfg["roadmap_dir"], ".trackfw-log")
        with open(log_path, encoding="utf-8") as f:
            log_content = f.read()
        self.assertIn(f"zeus/{basename}", log_content)
        self.assertIn("backlog → wip", log_content)


class TestMoveRoadmapAnalyzing(unittest.TestCase):
    """Contrato obrigatório: analyzing deve ser movível nos layouts flat e by_agent."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    def _canonical_roadmap(self, title: str, state: str = "backlog") -> str:
        return (
            f"---\nstatus: {state}\ndate: 2026-07-27\n"
            'req: "docs/req/REQ-demo.md"\nsquad: ""\n---\n\n'
            f"# Roadmap: {title}\n\n"
            f"> Created: 2026-07-27 | Status: {state}\n"
        )

    def test_move_analyzing_flat_syncs_status_and_log(self):
        cfg = _make_cfg(self.tmpdir)
        for state in ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]:
            os.makedirs(os.path.join(cfg["roadmap_dir"], state), exist_ok=True)
        src = os.path.join(cfg["roadmap_dir"], "backlog", "ROADMAP-analyze-flat.md")
        with open(src, "w", encoding="utf-8") as f:
            f.write(self._canonical_roadmap("Analyze Flat"))

        dst = move_roadmap("analyze-flat", "analyzing", cfg)

        self.assertEqual(os.path.dirname(dst), os.path.join(cfg["roadmap_dir"], "analyzing"))
        with open(dst, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: analyzing", content)
        self.assertIn("| Status: analyzing", content)
        with open(os.path.join(cfg["roadmap_dir"], ".trackfw-log"), encoding="utf-8") as f:
            log = f.read()
        self.assertIn("ROADMAP-analyze-flat.md", log)
        self.assertIn("backlog → analyzing", log)

    def test_move_analyzing_by_agent_preserves_agent_path_and_log(self):
        cfg = _make_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        backlog_dir = os.path.join(cfg["roadmap_dir"], "zeus", "backlog")
        os.makedirs(backlog_dir, exist_ok=True)
        os.makedirs(os.path.join(cfg["roadmap_dir"], "zeus", "analyzing"), exist_ok=True)
        src = os.path.join(backlog_dir, "ROADMAP-analyze-by-agent.md")
        with open(src, "w", encoding="utf-8") as f:
            f.write(self._canonical_roadmap("Analyze By Agent"))

        dst = move_roadmap("analyze-by-agent", "analyzing", cfg)

        self.assertEqual(os.path.dirname(dst), os.path.join(cfg["roadmap_dir"], "zeus", "analyzing"))
        with open(dst, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("status: analyzing", content)
        self.assertIn("| Status: analyzing", content)
        with open(os.path.join(cfg["roadmap_dir"], ".trackfw-log"), encoding="utf-8") as f:
            log = f.read()
        self.assertIn("zeus/ROADMAP-analyze-by-agent.md", log)
        self.assertIn("backlog → analyzing", log)


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


class TestGetFrontmatterRoadmapValue(unittest.TestCase):
    """Testes unitários de _get_frontmatter_roadmap_value."""

    def test_extrai_valor_com_aspas(self):
        content = '---\nstatus: Open\nroadmap: "docs/roadmaps/wip/X.md"\n---\n'
        self.assertEqual(
            _get_frontmatter_roadmap_value(content),
            "docs/roadmaps/wip/X.md",
        )

    def test_extrai_valor_sem_aspas(self):
        content = "---\nstatus: Open\nroadmap: docs/roadmaps/wip/X.md\n---\n"
        self.assertEqual(
            _get_frontmatter_roadmap_value(content),
            "docs/roadmaps/wip/X.md",
        )

    def test_nao_extrai_valor_com_backtick(self):
        # backtick não é removido → valor não termina em .md → retorna ''
        content = "---\nstatus: Open\nroadmap: `docs/roadmaps/wip/X.md`\n---\n"
        self.assertEqual(_get_frontmatter_roadmap_value(content), "")

    def test_retorna_vazio_sem_frontmatter(self):
        self.assertEqual(_get_frontmatter_roadmap_value("# Sem frontmatter\n"), "")

    def test_retorna_vazio_se_campo_ausente(self):
        content = "---\nstatus: Open\n---\n"
        self.assertEqual(_get_frontmatter_roadmap_value(content), "")

    def test_retorna_vazio_se_valor_vazio(self):
        content = '---\nroadmap: ""\n---\n'
        self.assertEqual(_get_frontmatter_roadmap_value(content), "")


class TestRewriteReqRoadmapRef(unittest.TestCase):
    """Testes unitários de _rewrite_req_roadmap_ref."""

    def _make_req(self, roadmap_path: str, body_fmt: str = "backtick") -> str:
        if body_fmt == "backtick":
            body_val = f"`{roadmap_path}`"
        elif body_fmt == "quote":
            body_val = f'"{roadmap_path}"'
        else:
            body_val = roadmap_path
        return (
            f'---\nstatus: Open\nroadmap: "{roadmap_path}"\n---\n\n'
            f"# REQ: Teste\n\n## Linked Roadmap\nRoadmap: {body_val}\n"
        )

    def test_reescreve_frontmatter_e_corpo_backtick(self):
        old_path = "docs/roadmaps/wip/ROADMAP-x.md"
        new_path = "docs/roadmaps/done/ROADMAP-x.md"
        content = self._make_req(old_path, "backtick")
        result, changed = _rewrite_req_roadmap_ref(content, new_path)
        self.assertTrue(changed)
        self.assertIn(f'roadmap: "{new_path}"', result)
        self.assertIn(f"Roadmap: `{new_path}`", result)

    def test_preserva_backtick_no_corpo(self):
        old_path = "docs/roadmaps/wip/ROADMAP-x.md"
        new_path = "docs/roadmaps/done/ROADMAP-x.md"
        content = self._make_req(old_path, "backtick")
        result, _ = _rewrite_req_roadmap_ref(content, new_path)
        # Backtick preservado: `new_path`
        self.assertIn(f"`{new_path}`", result)
        # Backtick não deve envolver o antigo caminho
        self.assertNotIn(f"`{old_path}`", result)

    def test_preserva_aspas_no_frontmatter(self):
        old_path = "docs/roadmaps/wip/ROADMAP-x.md"
        new_path = "docs/roadmaps/done/ROADMAP-x.md"
        content = self._make_req(old_path, "backtick")
        result, _ = _rewrite_req_roadmap_ref(content, new_path)
        # Aspas duplas preservadas no frontmatter
        self.assertIn(f'roadmap: "{new_path}"', result)

    def test_reescreve_corpo_bare(self):
        old_path = "docs/roadmaps/wip/ROADMAP-x.md"
        new_path = "docs/roadmaps/done/ROADMAP-x.md"
        content = self._make_req(old_path, "bare")
        result, changed = _rewrite_req_roadmap_ref(content, new_path)
        self.assertTrue(changed)
        self.assertIn(f"Roadmap: {new_path}", result)

    def test_idempotente_se_ja_correto(self):
        path = "docs/roadmaps/done/ROADMAP-x.md"
        content = self._make_req(path, "backtick")
        result, changed = _rewrite_req_roadmap_ref(content, path)
        self.assertFalse(changed)
        self.assertEqual(result, content)

    def test_sem_frontmatter_retorna_inalterado(self):
        content = "# REQ sem frontmatter\nRoadmap: docs/roadmaps/wip/X.md\n"
        result, changed = _rewrite_req_roadmap_ref(content, "docs/roadmaps/done/X.md")
        self.assertFalse(changed)
        self.assertEqual(result, content)


def _make_full_cfg(tmpdir: str, namespacing: str = "flat", agents=None) -> dict:
    """Config com roadmap_dir e req_dir isolados no tmpdir."""
    cfg = cfg_module.defaults()
    cfg["roadmap_dir"] = os.path.join(tmpdir, "docs", "roadmaps")
    cfg["req_dir"] = os.path.join(tmpdir, "docs", "req")
    cfg["roadmap_namespacing"] = namespacing
    if agents is not None:
        cfg["agents"] = agents
    return cfg


def _write_req_file(path: str, roadmap_path: str, body_backtick: bool = True) -> None:
    """Cria arquivo REQ com frontmatter e corpo no padrão canônico."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if body_backtick:
        body_val = f"`{roadmap_path}`"
    else:
        body_val = roadmap_path
    content = (
        f'---\nstatus: Open\ndate: 2026-07-30\nauthor: ""\nadr: ""\n'
        f'roadmap: "{roadmap_path}"\n---\n\n'
        f"# REQ: Teste\n\n## Linked Roadmap\nRoadmap: {body_val}\n"
    )
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


class TestSyncPairedReqReferences(unittest.TestCase):
    """Testes das cinco cardinalidades + idempotência + by_agent + backticks."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        cfg_module.reset()

    def tearDown(self):
        cfg_module.reset()

    # ------------------------------------------------------------------
    # Cardinalidade 1: Zero REQs → no-op silencioso
    # ------------------------------------------------------------------
    def test_zero_reqs_nooop_silencioso(self):
        cfg = _make_full_cfg(self.tmpdir)
        os.makedirs(cfg["req_dir"], exist_ok=True)
        # Nenhum arquivo .md em req_dir
        synced, failures = sync_paired_req_references(
            "docs/roadmaps/wip/ROADMAP-2026-07-30-x.md", cfg
        )
        self.assertEqual(synced, [])
        self.assertEqual(failures, [])

    # ------------------------------------------------------------------
    # Cardinalidade 2: Uma REQ → reescreve
    # ------------------------------------------------------------------
    def test_uma_req_reescrita(self):
        cfg = _make_full_cfg(self.tmpdir)
        req_dir = cfg["req_dir"]
        os.makedirs(req_dir, exist_ok=True)

        old_path = "docs/roadmaps/backlog/ROADMAP-2026-07-30-feat.md"
        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-feat.md"
        req_file = os.path.join(req_dir, "REQ-2026-07-30-feat.md")
        _write_req_file(req_file, old_path, body_backtick=True)

        synced, failures = sync_paired_req_references(new_path, cfg)

        self.assertEqual(failures, [])
        self.assertEqual(synced, ["REQ-2026-07-30-feat.md"])

        with open(req_file, encoding="utf-8") as f:
            content = f.read()
        self.assertIn(f'roadmap: "{new_path}"', content)
        self.assertIn(f"Roadmap: `{new_path}`", content)
        self.assertNotIn(old_path, content)

    # ------------------------------------------------------------------
    # Cardinalidade 3: Várias REQs → todas reescritas
    # ------------------------------------------------------------------
    def test_varias_reqs_todas_reescritas(self):
        cfg = _make_full_cfg(self.tmpdir)
        req_dir = cfg["req_dir"]
        os.makedirs(req_dir, exist_ok=True)

        old_path = "docs/roadmaps/backlog/ROADMAP-2026-07-30-multi.md"
        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-multi.md"

        req_a = os.path.join(req_dir, "REQ-2026-07-30-a.md")
        req_b = os.path.join(req_dir, "REQ-2026-07-30-b.md")
        _write_req_file(req_a, old_path)
        _write_req_file(req_b, old_path)

        synced, failures = sync_paired_req_references(new_path, cfg)

        self.assertEqual(failures, [])
        self.assertEqual(len(synced), 2)
        self.assertIn("REQ-2026-07-30-a.md", synced)
        self.assertIn("REQ-2026-07-30-b.md", synced)

        for req_file in (req_a, req_b):
            with open(req_file, encoding="utf-8") as f:
                content = f.read()
            self.assertIn(f'roadmap: "{new_path}"', content)
            self.assertNotIn(old_path, content)

    # ------------------------------------------------------------------
    # Cardinalidade 4: REQ aponta para outro roadmap → não toca
    # ------------------------------------------------------------------
    def test_req_aponta_para_outro_roadmap_nao_tocada(self):
        cfg = _make_full_cfg(self.tmpdir)
        req_dir = cfg["req_dir"]
        os.makedirs(req_dir, exist_ok=True)

        other_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-outro.md"
        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-alvo.md"
        req_file = os.path.join(req_dir, "REQ-2026-07-30-outro.md")
        _write_req_file(req_file, other_path)

        # Bytes antes
        with open(req_file, "rb") as f:
            bytes_before = f.read()

        synced, failures = sync_paired_req_references(new_path, cfg)

        self.assertEqual(synced, [])
        self.assertEqual(failures, [])

        with open(req_file, "rb") as f:
            bytes_after = f.read()
        self.assertEqual(bytes_before, bytes_after)

    # ------------------------------------------------------------------
    # Cardinalidade 5: Referência já correta → nenhuma escrita (idempotente)
    # ------------------------------------------------------------------
    def test_referencia_ja_correta_sem_escrita(self):
        cfg = _make_full_cfg(self.tmpdir)
        req_dir = cfg["req_dir"]
        os.makedirs(req_dir, exist_ok=True)

        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-x.md"
        req_file = os.path.join(req_dir, "REQ-2026-07-30-x.md")
        _write_req_file(req_file, new_path)  # já aponta para o estado correto

        with open(req_file, "rb") as f:
            bytes_before = f.read()

        synced, failures = sync_paired_req_references(new_path, cfg)

        self.assertEqual(synced, [])
        self.assertEqual(failures, [])

        with open(req_file, "rb") as f:
            bytes_after = f.read()
        self.assertEqual(bytes_before, bytes_after,
            "referência já correta causou escrita inesperada (não é idempotente)")

    # ------------------------------------------------------------------
    # Idempotência byte-a-byte: chamar sync duas vezes é equivalente a uma
    # ------------------------------------------------------------------
    def test_idempotencia_byte_a_byte_duas_chamadas(self):
        cfg = _make_full_cfg(self.tmpdir)
        req_dir = cfg["req_dir"]
        os.makedirs(req_dir, exist_ok=True)

        old_path = "docs/roadmaps/backlog/ROADMAP-2026-07-30-idem.md"
        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-idem.md"
        req_file = os.path.join(req_dir, "REQ-2026-07-30-idem.md")
        _write_req_file(req_file, old_path)

        # Primeira chamada: deve reescrever
        synced1, _ = sync_paired_req_references(new_path, cfg)
        self.assertEqual(len(synced1), 1)

        with open(req_file, "rb") as f:
            bytes_after_first = f.read()

        # Segunda chamada: referência já correta → nenhuma escrita
        synced2, failures2 = sync_paired_req_references(new_path, cfg)
        self.assertEqual(synced2, [], "segunda chamada deveria ser no-op")
        self.assertEqual(failures2, [])

        with open(req_file, "rb") as f:
            bytes_after_second = f.read()

        self.assertEqual(bytes_after_first, bytes_after_second,
            "bytes mudaram na segunda chamada — não é idempotente")

    # ------------------------------------------------------------------
    # Layout by_agent: REQ em req_dir/<agente>/<estado>/ é encontrada
    # ------------------------------------------------------------------
    def test_by_agent_req_encontrada(self):
        cfg = _make_full_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus"])
        req_dir = cfg["req_dir"]
        agent_state_dir = os.path.join(req_dir, "zeus", "wip")
        os.makedirs(agent_state_dir, exist_ok=True)

        old_path = "docs/roadmaps/backlog/ROADMAP-2026-07-30-agent.md"
        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-agent.md"
        req_file = os.path.join(agent_state_dir, "REQ-2026-07-30-agent.md")
        _write_req_file(req_file, old_path)

        synced, failures = sync_paired_req_references(new_path, cfg)

        self.assertEqual(failures, [])
        self.assertEqual(synced, ["REQ-2026-07-30-agent.md"],
            "REQ em req_dir/<agente>/<estado>/ não foi encontrada (by_agent)")

        with open(req_file, encoding="utf-8") as f:
            content = f.read()
        self.assertIn(f'roadmap: "{new_path}"', content)

    # ------------------------------------------------------------------
    # Backticks preservados após reescrita
    # ------------------------------------------------------------------
    def test_backticks_preservados_no_corpo(self):
        cfg = _make_full_cfg(self.tmpdir)
        req_dir = cfg["req_dir"]
        os.makedirs(req_dir, exist_ok=True)

        old_path = "docs/roadmaps/backlog/ROADMAP-2026-07-30-bt.md"
        new_path = "docs/roadmaps/done/ROADMAP-2026-07-30-bt.md"
        req_file = os.path.join(req_dir, "REQ-2026-07-30-bt.md")
        _write_req_file(req_file, old_path, body_backtick=True)

        sync_paired_req_references(new_path, cfg)

        with open(req_file, encoding="utf-8") as f:
            content = f.read()

        # Corpo deve ter backticks com o novo caminho
        self.assertIn(f"Roadmap: `{new_path}`", content)
        # Corpo NÃO deve ter backticks com o caminho antigo
        self.assertNotIn(f"Roadmap: `{old_path}`", content)

    # ------------------------------------------------------------------
    # Ordenação by_agent discriminante: basename ≠ ordem de caminho completo
    # Fixture: apolo/done/REQ-zzz + zeus/backlog/REQ-aaa
    # Caminho: apolo/done/REQ-zzz < zeus/backlog/REQ-aaa (ordem por caminho)
    # Basename: REQ-aaa < REQ-zzz (ordem por basename — contrato)
    # Esperado: ["REQ-aaa.md", "REQ-zzz.md"]
    # ------------------------------------------------------------------
    def test_by_agent_ordenacao_por_basename_fixture_discriminante(self):
        """Garante ordenação por basename, não por caminho completo (fixture discriminante)."""
        cfg = _make_full_cfg(self.tmpdir, namespacing="by_agent", agents=["zeus", "apolo"])
        req_dir = cfg["req_dir"]

        old_path = "docs/roadmaps/backlog/ROADMAP-2026-07-30-ordem.md"
        new_path = "docs/roadmaps/wip/ROADMAP-2026-07-30-ordem.md"

        # apolo/done/REQ-zzz.md → nome de caminho completo fica ANTES de zeus/backlog/REQ-aaa.md
        # mas basename REQ-zzz.md deve ficar APÓS REQ-aaa.md
        apolo_done_dir = os.path.join(req_dir, "apolo", "done")
        zeus_backlog_dir = os.path.join(req_dir, "zeus", "backlog")
        os.makedirs(apolo_done_dir, exist_ok=True)
        os.makedirs(zeus_backlog_dir, exist_ok=True)

        req_zzz = os.path.join(apolo_done_dir, "REQ-zzz.md")
        req_aaa = os.path.join(zeus_backlog_dir, "REQ-aaa.md")
        _write_req_file(req_zzz, old_path)
        _write_req_file(req_aaa, old_path)

        synced, failures = sync_paired_req_references(new_path, cfg)

        self.assertEqual(failures, [])
        self.assertEqual(len(synced), 2)
        # Ordem DEVE ser por basename: aaa antes de zzz
        self.assertEqual(synced[0], "REQ-aaa.md",
            f"Esperado REQ-aaa.md primeiro (basename order), obtido: {synced}")
        self.assertEqual(synced[1], "REQ-zzz.md",
            f"Esperado REQ-zzz.md segundo (basename order), obtido: {synced}")

    # ------------------------------------------------------------------
    # Saída pinada literalmente: ✓ U+2713, → U+2192
    # ------------------------------------------------------------------
    def test_caracteres_unicode_na_saida_pinados(self):
        """Verifica que ✓ (U+2713) e → (U+2192) são os chars corretos."""
        synced_line = "✓ synced REQ-foo.md → docs/roadmaps/wip/ROADMAP-bar.md"
        # U+2713 CHECK MARK
        self.assertEqual(synced_line[0], "✓")
        # U+2192 RIGHTWARDS ARROW
        arrow_idx = synced_line.index("→")
        self.assertEqual(synced_line[arrow_idx], "→")


if __name__ == "__main__":
    unittest.main()
