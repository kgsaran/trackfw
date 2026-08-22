"""
test_validator.py — Testes unitários para pypi/trackfw/validator.py
Espelha a cobertura de npm/src/validator/index.test.js.
Usa tempfile.mkdtemp() para isolamento — sem fixtures compartilhadas.
"""

import json
import os
import time
import unittest
import tempfile
import shutil
import pytest
from datetime import datetime

# Garante que importamos a versão local do pacote
import sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw import validator as v


def _write(path: str, content: str = ""):
    """Utilitário: cria arquivo com conteúdo."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


class TestListDir(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def test_retorna_vazio_se_dir_nao_existe(self):
        result = v.list_dir(os.path.join(self.tmp, "nao-existe"))
        self.assertEqual(result, [])

    def test_retorna_apenas_arquivos(self):
        _write(os.path.join(self.tmp, "arquivo.md"), "conteudo")
        os.makedirs(os.path.join(self.tmp, "subdir"))
        result = v.list_dir(self.tmp)
        self.assertIn("arquivo.md", result)
        self.assertNotIn("subdir", result)


class TestResolveWipDirs(unittest.TestCase):
    def test_modo_flat(self):
        cfg = _config.defaults()
        cfg["roadmap_namespacing"] = "flat"
        cfg["roadmap_dir"] = "docs/roadmaps"
        result = v.resolve_wip_dirs(cfg)
        self.assertEqual(result, ["docs/roadmaps/wip"])

    def test_modo_by_agent_com_agents_configurados(self):
        cfg = _config.defaults()
        cfg["roadmap_namespacing"] = "by_agent"
        cfg["roadmap_dir"] = "docs/roadmaps"
        cfg["agents"] = ["apolo", "afrodite"]
        result = v.resolve_wip_dirs(cfg)
        self.assertEqual(result, [
            "docs/roadmaps/apolo/wip",
            "docs/roadmaps/afrodite/wip",
        ])


class TestParseFrontmatter(unittest.TestCase):
    def test_extrai_campos(self):
        content = "---\nstatus: Open\ntitle: Minha REQ\n---\n\nCorpo"
        result = v.parse_frontmatter(content)
        self.assertEqual(result.get("status"), "Open")
        self.assertEqual(result.get("title"), "Minha REQ")

    def test_sem_frontmatter(self):
        content = "# Título\n\nSem frontmatter"
        result = v.parse_frontmatter(content)
        self.assertEqual(result, {})

    def test_chave_com_hifen_vira_underscore(self):
        content = "---\nlinked-adr: ADR-001.md\n---\n"
        result = v.parse_frontmatter(content)
        self.assertIn("linked_adr", result)

    def test_status_com_aspas_externas_e_normalizado(self):
        content = '---\nstatus: "wip"\ntitle: Roadmap\n---\n'
        result = v.parse_frontmatter(content)
        self.assertEqual(result.get("status"), "wip")

    def test_valores_yaml_flat_com_aspas_simples_e_duplas_sao_normalizados(self):
        content = "---\nstatus: 'wip'\ntitle: \"Roadmap\"\n---\n"
        result = v.parse_frontmatter(content)
        self.assertEqual(result.get("status"), "wip")
        self.assertEqual(result.get("title"), "Roadmap")

    def test_valores_yaml_flat_vazios_sao_preservados(self):
        content = '---\nsquad: ""\nowner: \n---\n'
        result = v.parse_frontmatter(content)
        self.assertEqual(result.get("squad"), "")
        self.assertEqual(result.get("owner"), "")

    def test_valores_yaml_flat_preservam_conteudo_interno(self):
        content = "---\ntitle: \"Roadmap 'release'\"\nslug: 'fix/\"release\"'\nraw: \"wip\n---\n"
        result = v.parse_frontmatter(content)
        self.assertEqual(result.get("title"), "Roadmap 'release'")
        self.assertEqual(result.get("slug"), 'fix/"release"')
        self.assertEqual(result.get("raw"), '"wip')

    def test_valor_entre_backticks_no_frontmatter_preserva_os_backticks(self):
        """ML-1D — regressão: normalize_yaml_flat_value (usada por
        parse_frontmatter) NÃO conhece backtick como delimitador; a remoção
        de backtick é responsabilidade exclusiva de _extract_ref_path.
        Backtick não é delimitador de string em YAML — removê-lo aqui em
        parse_frontmatter divergiria de Go/Node, que só tratam backtick em
        extractRefPath (ver ADR / ML-1C)."""
        content = "---\nadr: `docs/adr/X.md`\n---\n"
        result = v.parse_frontmatter(content)
        self.assertEqual(result.get("adr"), "`docs/adr/X.md`")


class TestValidateWipHasReq(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def _cfg(self):
        cfg = _config.defaults()
        cfg["roadmap_dir"] = os.path.join(self.tmp, "docs/roadmaps")
        cfg["req_dir"] = os.path.join(self.tmp, "docs/req")
        return cfg

    def test_sem_violations_wip_vazio(self):
        cfg = self._cfg()
        os.makedirs(os.path.join(self.tmp, "docs/roadmaps/wip"), exist_ok=True)
        result = v.validate_wip_has_req(cfg)
        self.assertEqual(result, [])

    def test_violation_sem_req(self):
        cfg = self._cfg()
        wip_dir = os.path.join(self.tmp, "docs/roadmaps/wip")
        _write(os.path.join(wip_dir, "roadmap-sem-req.md"), "# Roadmap\n\nSem link de REQ")
        result = v.validate_wip_has_req(cfg)
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["type"], "violation")
        self.assertIn("roadmap-sem-req.md", result[0]["message"])

    def test_sem_violation_com_req(self):
        cfg = self._cfg()
        wip_dir = os.path.join(self.tmp, "docs/roadmaps/wip")
        _write(os.path.join(wip_dir, "roadmap-ok.md"), "REQ: REQ-2026-001.md\n# Roadmap")
        result = v.validate_wip_has_req(cfg)
        self.assertEqual(result, [])


class TestValidateWipLimit(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def _cfg(self, wip_limit=1):
        cfg = _config.defaults()
        cfg["roadmap_dir"] = os.path.join(self.tmp, "docs/roadmaps")
        cfg["wip_limit"] = wip_limit
        return cfg

    def test_sem_violations_wip_vazio(self):
        cfg = self._cfg()
        os.makedirs(os.path.join(self.tmp, "docs/roadmaps/wip"), exist_ok=True)
        result = v.validate_wip_limit(cfg)
        self.assertEqual(result["warnings"], [])
        self.assertEqual(result["violations"], [])

    def test_wip_limit_violation_dois_arquivos_limite_um(self):
        cfg = self._cfg(wip_limit=1)
        wip_dir = os.path.join(self.tmp, "docs/roadmaps/wip")
        _write(os.path.join(wip_dir, "roadmap-a.md"), "# A")
        _write(os.path.join(wip_dir, "roadmap-b.md"), "# B")
        result = v.validate_wip_limit(cfg)
        self.assertEqual(len(result["warnings"]), 1)
        self.assertIn("2", result["warnings"][0]["message"])
        self.assertIn("limit: 1", result["warnings"][0]["message"])

    def test_wip_dentro_do_limite(self):
        """
        No modo flat, validate_wip_limit lê o wip_limit do trackfw.yaml no CWD
        (espelhando readWIPConfig do JS). O cfg["wip_limit"] é usado apenas no
        modo by_agent. Para testar o limite=3, escrevemos o yaml em self.tmp.
        """
        cfg = self._cfg(wip_limit=3)
        # Persiste o limite no yaml do tmp para que _read_wip_config o encontre
        _write(os.path.join(self.tmp, "trackfw.yaml"), "wip_limit: 3\n")

        wip_dir = os.path.join(self.tmp, "docs/roadmaps/wip")
        _write(os.path.join(wip_dir, "roadmap-a.md"), "# A")
        _write(os.path.join(wip_dir, "roadmap-b.md"), "# B")

        orig_cwd = os.getcwd()
        try:
            os.chdir(self.tmp)
            result = v.validate_wip_limit(cfg)
        finally:
            os.chdir(orig_cwd)

        self.assertEqual(result["warnings"], [])


class TestValidateStaleWip(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def _cfg(self):
        cfg = _config.defaults()
        cfg["roadmap_dir"] = os.path.join(self.tmp, "docs/roadmaps")
        return cfg

    def test_sem_warnings_wip_vazio(self):
        cfg = self._cfg()
        os.makedirs(os.path.join(self.tmp, "docs/roadmaps/wip"), exist_ok=True)
        result = v.validate_stale_wip(cfg)
        self.assertEqual(result, [])

    def test_stale_wip_warning_arquivo_antigo(self):
        cfg = self._cfg()
        wip_dir = os.path.join(self.tmp, "docs/roadmaps/wip")
        file_path = os.path.join(wip_dir, "roadmap-antigo.md")
        _write(file_path, "# Roadmap antigo")

        # Retrocede o mtime em 10 dias
        old_time = time.time() - (10 * 24 * 60 * 60)
        os.utime(file_path, (old_time, old_time))

        result = v.validate_stale_wip(cfg, days=7)
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["type"], "warning")
        self.assertIn("roadmap-antigo.md", result[0]["message"])
        self.assertIn("10 days", result[0]["message"])

    def test_arquivo_recente_nao_gera_warning(self):
        cfg = self._cfg()
        wip_dir = os.path.join(self.tmp, "docs/roadmaps/wip")
        _write(os.path.join(wip_dir, "roadmap-recente.md"), "# Roadmap recente")
        result = v.validate_stale_wip(cfg, days=7)
        self.assertEqual(result, [])


class TestSemViolationsProjetoVazio(unittest.TestCase):
    """Projeto sem nenhum artefato não deve gerar violations."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def test_sem_violations_projeto_vazio(self):
        # trackfw.yaml mínimo apontando para dirs do tmp
        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        _write(yaml_path, (
            f"roadmap_dir: {os.path.join(self.tmp, 'docs/roadmaps')}\n"
            f"req_dir: {os.path.join(self.tmp, 'docs/req')}\n"
            f"adr_dirs:\n"
            f"  - {os.path.join(self.tmp, 'docs/adr')}\n"
        ))

        # Cria dirs vazios (sem arquivos)
        for d in ["docs/roadmaps/wip", "docs/roadmaps/blocked", "docs/req", "docs/adr"]:
            os.makedirs(os.path.join(self.tmp, d), exist_ok=True)

        result = v.validate(self.tmp)
        self.assertEqual(result["violations"], [])
        # warnings de stale wip também devem ser vazios
        self.assertEqual(result["warnings"], [])


class TestLenientMode(unittest.TestCase):
    """Em modo lenient, violations devem ser movidas para warnings."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def test_lenient_mode_violations_viram_warnings(self):
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")
        req_dir = os.path.join(self.tmp, "docs/req")
        adr_dir = os.path.join(self.tmp, "docs/adr")

        # trackfw.yaml com governance_mode: lenient (sem lenient_until → nunca expira)
        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        _write(yaml_path, (
            f"roadmap_dir: {roadmap_dir}\n"
            f"req_dir: {req_dir}\n"
            f"adr_dirs:\n"
            f"  - {adr_dir}\n"
            "governance_mode: lenient\n"
            "wip_limit: 10\n"
        ))

        # Cria um roadmap em wip/ sem REQ → normalmente seria violation
        wip_dir = os.path.join(roadmap_dir, "wip")
        _write(os.path.join(wip_dir, "roadmap-sem-req.md"), "# Roadmap sem REQ\n\nSem link")

        result = v.validate(self.tmp)

        # Em modo lenient: violations = [] e a mensagem vai para warnings
        self.assertEqual(result["violations"], [])
        msgs = [w["message"] for w in result["warnings"]]
        self.assertTrue(
            any("roadmap-sem-req.md" in m for m in msgs),
            f"Esperava mensagem sobre roadmap-sem-req.md em warnings. Obtido: {msgs}"
        )

    def test_strict_mode_gera_violations(self):
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")
        req_dir = os.path.join(self.tmp, "docs/req")
        adr_dir = os.path.join(self.tmp, "docs/adr")

        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        _write(yaml_path, (
            f"roadmap_dir: {roadmap_dir}\n"
            f"req_dir: {req_dir}\n"
            f"adr_dirs:\n"
            f"  - {adr_dir}\n"
            "wip_limit: 10\n"
        ))

        wip_dir = os.path.join(roadmap_dir, "wip")
        _write(os.path.join(wip_dir, "roadmap-sem-req.md"), "# Roadmap sem REQ\n\nSem link")

        result = v.validate(self.tmp)

        # Em modo strict: deve haver violation
        msgs = [viol["message"] for viol in result["violations"]]
        self.assertTrue(
            any("roadmap-sem-req.md" in m for m in msgs),
            f"Esperava violation para roadmap-sem-req.md. Obtido: {msgs}"
        )


class TestWipLimitViolation(unittest.TestCase):
    """Cenário explícito: 2 arquivos em wip/ com wip_limit=1 → 1 warning."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def test_dois_arquivos_limite_um(self):
        roadmap_dir = os.path.join(self.tmp, "docs/roadmaps")
        req_dir = os.path.join(self.tmp, "docs/req")
        adr_dir = os.path.join(self.tmp, "docs/adr")

        yaml_path = os.path.join(self.tmp, "trackfw.yaml")
        _write(yaml_path, (
            f"roadmap_dir: {roadmap_dir}\n"
            f"req_dir: {req_dir}\n"
            f"adr_dirs:\n"
            f"  - {adr_dir}\n"
            "wip_limit: 1\n"
        ))

        # Cria 2 roadmaps válidos (com REQ: e Acceptance Criteria) para não ter outras violations
        wip_dir = os.path.join(roadmap_dir, "wip")
        body = "REQ: REQ-001.md\n## Critérios de Aceite\n- [ ] ok\n"
        _write(os.path.join(wip_dir, "roadmap-a.md"), body)
        _write(os.path.join(wip_dir, "roadmap-b.md"), body)

        result = v.validate(self.tmp)

        # Deve ter exatamente 1 warning de wip_limit
        wip_warnings = [
            w for w in result["warnings"]
            if "limit:" in w["message"] and "roadmaps in wip/" in w["message"]
        ]
        self.assertEqual(len(wip_warnings), 1)
        self.assertIn("2", wip_warnings[0]["message"])


class TestValidateReqHasAdr(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp)
        _config.reset()

    def _cfg(self):
        cfg = _config.defaults()
        cfg["req_dir"] = os.path.join(self.tmp, "docs/req")
        cfg["roadmap_dir"] = os.path.join(self.tmp, "docs/roadmaps")
        cfg["adr_dirs"] = [os.path.join(self.tmp, "docs/adr")]
        return cfg

    def test_req_sem_adr_gera_violation(self):
        cfg = self._cfg()
        _write(os.path.join(self.tmp, "docs/req", "REQ-001.md"), "# REQ\n\nSem ADR")
        result = v.validate_reqs_have_adr(cfg)
        self.assertEqual(len(result), 1)
        self.assertIn("REQ-001.md", result[0]["message"])

    def test_req_com_adr_sem_violation(self):
        cfg = self._cfg()
        _write(os.path.join(self.tmp, "docs/req", "REQ-001.md"), "ADR: ADR-001.md\n# REQ")
        result = v.validate_reqs_have_adr(cfg)
        self.assertEqual(result, [])


class TestValidatorImprovements(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_walk_dir_md_finds_in_subdirs(self):
        """_walk_dir_md deve encontrar .md em subpastas."""
        from trackfw.validator import _walk_dir_md
        done_dir = os.path.join(self.tmp, "done")
        os.makedirs(done_dir)
        with open(os.path.join(done_dir, "ADR-001.md"), "w") as f:
            f.write("---\nstatus: Accepted\n---\n# ADR\n")
        wip_dir = os.path.join(self.tmp, "wip")
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, "ADR-002.md"), "w") as f:
            f.write("---\nstatus: Draft\n---\n# ADR\n")
        results = _walk_dir_md(self.tmp)
        self.assertIn("ADR-001.md", results)
        self.assertIn("ADR-002.md", results)

    def test_find_adr_file_in_subdir(self):
        """_find_adr_file deve encontrar arquivo em subpasta."""
        from trackfw.validator import _find_adr_file
        sub = os.path.join(self.tmp, "done")
        os.makedirs(sub)
        adr_path = os.path.join(sub, "ADR-001.md")
        with open(adr_path, "w") as f:
            f.write("---\nstatus: Accepted\n---\n")
        result = _find_adr_file("ADR-001.md", [self.tmp])
        self.assertEqual(result, adr_path)

    def test_find_adr_file_not_found(self):
        from trackfw.validator import _find_adr_file
        result = _find_adr_file("nao-existe.md", [self.tmp])
        self.assertEqual(result, "")

    def test_extract_ref_path_basic(self):
        from trackfw.validator import _extract_ref_path
        content = "REQ: docs/req/foo.md\n"
        self.assertEqual(_extract_ref_path(content, "REQ"), "docs/req/foo.md")

    def test_extract_ref_path_em_dash(self):
        from trackfw.validator import _extract_ref_path
        content = "REQ: —\n"
        self.assertEqual(_extract_ref_path(content, "REQ"), "")

    def test_extract_ref_path_no_md(self):
        from trackfw.validator import _extract_ref_path
        content = "REQ: algum texto sem extensao\n"
        self.assertEqual(_extract_ref_path(content, "REQ"), "")

    def test_extract_ref_path_backtick_com_prosa(self):
        """ML-1C — backtick pareado ao redor do caminho, com prosa após: o
        primeiro token (antes do espaço) é o caminho entre backticks; deve
        resolver para o .md sem os delimitadores."""
        from trackfw.validator import _extract_ref_path
        content = (
            "ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` "
            "(P1–P4; esta REQ é ...)\n"
        )
        self.assertEqual(
            _extract_ref_path(content, "ADR"),
            "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md",
        )

    def test_extract_ref_path_resolve_reqs_reais_com_backtick(self):
        """ML-1C — as 3 REQs reais do repositório cujo campo ADR usa backtick
        (sem `adr:` no frontmatter) devem ter o ADR resolvido pelo extrator."""
        from trackfw.validator import _extract_ref_path

        repo_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
        reqs = [
            "docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md",
            "docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md",
            "docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md",
        ]
        for rel_path in reqs:
            full_path = os.path.join(repo_root, rel_path)
            with open(full_path, "r", encoding="utf-8") as fh:
                content = fh.read()
            resolved = _extract_ref_path(content, "ADR")
            self.assertTrue(
                resolved.endswith(".md"),
                f"{rel_path}: esperava resolver ADR .md, obteve {resolved!r}",
            )
            self.assertEqual(
                resolved,
                "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md",
                f"{rel_path}: ADR resolvido inesperado: {resolved!r}",
            )

    def test_extract_ref_path_delimitador_nao_pareado(self):
        """ML-1A — delimitador aberto com aspa dupla e fechado com aspa
        simples (`ADR: "docs/adr/X.md'`) deve resolver como Go
        (strings.Trim(v, "\\"'`")) e Node (regex de borda única): remove um
        delimitador de cada ponta, mesmo sem par casado. Antes desta
        correção, normalize_yaml_flat_value (que exige par casado) fazia o
        Python devolver ''."""
        from trackfw.validator import _extract_ref_path
        content = "ADR: \"docs/adr/X.md'\n"
        self.assertEqual(_extract_ref_path(content, "ADR"), "docs/adr/X.md")

    def test_extract_ref_path_tabela_oito_entradas(self):
        """ML-1A — tabela completa do ADR de convergência: os 8 casos devem
        resolver de forma idêntica a Go/Node."""
        from trackfw.validator import _extract_ref_path
        casos = [
            ("ADR: `docs/adr/X.md`", "docs/adr/X.md"),
            ('ADR: "docs/adr/X.md"', "docs/adr/X.md"),
            ("ADR: 'docs/adr/X.md'", "docs/adr/X.md"),
            ("ADR: docs/adr/X.md", "docs/adr/X.md"),
            ("ADR: `docs/adr/X.md` (prosa)", "docs/adr/X.md"),
            ("ADR: \"docs/adr/X.md'", "docs/adr/X.md"),
            ("ADR:", ""),
            ("ADR: —", ""),
        ]
        for linha, esperado in casos:
            with self.subTest(linha=linha):
                self.assertEqual(_extract_ref_path(linha, "ADR"), esperado)

    def test_validate_ref_targets_exist_warning(self):
        """Ref a arquivo inexistente gera warning."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_ref_targets_exist
        cfg_mod.reset()

        # Criar estrutura mínima
        req_dir = os.path.join(self.tmp, "docs", "req")
        roadmap_wip = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(req_dir)
        os.makedirs(roadmap_wip)

        # Roadmap com REQ inexistente
        with open(os.path.join(roadmap_wip, "my-roadmap.md"), "w") as f:
            f.write("---\nstatus: WIP\n---\nREQ: docs/req/nao-existe.md\n")

        cfg = {
            "adr_dirs": ["docs/adr"],
            "req_dir": req_dir,
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }

        import os as _os
        orig_cwd = _os.getcwd()
        _os.chdir(self.tmp)
        try:
            warnings = validate_ref_targets_exist(cfg)
        finally:
            _os.chdir(orig_cwd)

        self.assertTrue(any("nao-existe.md" in w["message"] for w in warnings))

    def test_validate_ref_targets_rejects_generated_basenames(self):
        from trackfw.validator import validate_ref_targets_exist

        req_dir = os.path.join(self.tmp, "docs", "req")
        roadmap_wip = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(req_dir)
        os.makedirs(roadmap_wip)
        with open(os.path.join(req_dir, "REQ-001.md"), "w") as f:
            f.write("# REQ\nRoadmap: ROADMAP-001.md\n")
        with open(os.path.join(roadmap_wip, "ROADMAP-001.md"), "w") as f:
            f.write("# Roadmap\nREQ: REQ-001.md\n")

        cfg = {
            "adr_dirs": [os.path.join(self.tmp, "docs", "adr")],
            "req_dir": req_dir,
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_ref_targets_exist(cfg)
        self.assertTrue(any("ROADMAP-001.md" in w["message"] for w in warnings))

    def test_validate_ref_targets_rejects_frontmatter_basename_req(self):
        """
        Regressão: req: "<basename>" no frontmatter deve continuar reprovando
        mesmo quando o corpo (REQ: <caminho completo>) aponta para uma REQ
        que de fato existe. O extrator lê o primeiro campo casado (frontmatter
        precede corpo), então basename no frontmatter nunca pode ser tolerado
        por _reference_exists — ver ADR-2026-08-01.
        """
        from trackfw.validator import validate_ref_targets_exist

        req_dir = os.path.join(self.tmp, "docs", "req")
        roadmap_wip = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(req_dir)
        os.makedirs(roadmap_wip)

        req_basename = "REQ-2026-08-01-fonte.md"
        req_full_path = os.path.join(req_dir, req_basename)
        with open(req_full_path, "w") as f:
            f.write("# REQ: Fonte\n")

        with open(os.path.join(roadmap_wip, "ROADMAP-002.md"), "w") as f:
            f.write(
                "---\n"
                f'req: "{req_basename}"\n'
                "---\n"
                "# Roadmap\n"
                f"REQ: {req_full_path}\n"
            )

        cfg = {
            "adr_dirs": [os.path.join(self.tmp, "docs", "adr")],
            "req_dir": req_dir,
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_ref_targets_exist(cfg)
        self.assertTrue(
            any(req_basename in w["message"] for w in warnings),
            f"esperava warning citando basename '{req_basename}'; warnings={warnings}",
        )

    def test_validate_folder_status_coherence_warning(self):
        """Arquivo em wip/ com status: Done gera warning."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_folder_status_coherence
        cfg_mod.reset()

        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, "my-roadmap.md"), "w") as f:
            f.write("---\nstatus: Done\n---\n# Roadmap\n")

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_folder_status_coherence(cfg)
        self.assertTrue(any('status declares "Done"' in w["message"] for w in warnings))

    def test_validate_folder_status_coherence_no_warning_when_match(self):
        """Arquivo em wip/ com status: WIP não gera warning."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_folder_status_coherence
        cfg_mod.reset()

        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, "my-roadmap.md"), "w") as f:
            f.write("---\nstatus: WIP\n---\n# Roadmap\n")

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_folder_status_coherence(cfg)
        self.assertEqual(warnings, [])

    def test_validate_folder_status_coherence_no_warning_when_quoted_wip(self):
        """Arquivo em wip/ com status: "wip" deve ser equivalente a status: wip."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_folder_status_coherence
        cfg_mod.reset()

        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, "quoted-wip.md"), "w") as f:
            f.write('---\nstatus: "wip"\n---\n# Roadmap\n')

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_folder_status_coherence(cfg)
        messages = [w["message"] for w in warnings]
        self.assertFalse(
            any("quoted-wip.md" in message for message in messages),
            f'status: "wip" não deve gerar warning folder_status; warnings={messages}',
        )

    def test_validate_folder_status_coherence_no_warning_when_single_quoted_wip(self):
        """Arquivo em wip/ com status: 'wip' deve ser equivalente a status: wip."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_folder_status_coherence
        cfg_mod.reset()

        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, "single-quoted-wip.md"), "w") as f:
            f.write("---\nstatus: 'wip'\n---\n# Roadmap\n")

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_folder_status_coherence(cfg)
        messages = [w["message"] for w in warnings]
        self.assertFalse(
            any("single-quoted-wip.md" in message for message in messages),
            f"status: 'wip' não deve gerar warning folder_status; warnings={messages}",
        )

    def test_validate_folder_status_coherence_ignora_status_vazio_aspeado(self):
        """Arquivo com status vazio não deve gerar mismatch artificial."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_folder_status_coherence
        cfg_mod.reset()

        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        os.makedirs(wip_dir)
        with open(os.path.join(wip_dir, "empty-status.md"), "w") as f:
            f.write('---\nstatus: ""\n---\n# Roadmap\n')

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        warnings = validate_folder_status_coherence(cfg)
        messages = [w["message"] for w in warnings]
        self.assertFalse(
            any("empty-status.md" in message for message in messages),
            f'status: "" deve continuar sem mismatch; warnings={messages}',
        )

    def test_validate_filename_uniqueness_violation(self):
        """Mesmo filename em wip/ e backlog/ gera violation."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_filename_uniqueness
        cfg_mod.reset()

        for state in ["wip", "backlog"]:
            d = os.path.join(self.tmp, "docs", "roadmaps", state)
            os.makedirs(d)
            with open(os.path.join(d, "duplicado.md"), "w") as f:
                f.write("---\nstatus: WIP\n---\n")

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        violations = validate_filename_uniqueness(cfg)
        self.assertTrue(any("duplicado.md" in v["message"] for v in violations))

    def test_validate_filename_uniqueness_no_violation(self):
        """Filenames únicos por estado não geram violation."""
        from trackfw import config as cfg_mod
        from trackfw.validator import validate_filename_uniqueness
        cfg_mod.reset()

        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        backlog_dir = os.path.join(self.tmp, "docs", "roadmaps", "backlog")
        os.makedirs(wip_dir)
        os.makedirs(backlog_dir)
        with open(os.path.join(wip_dir, "feat-a.md"), "w") as f:
            f.write("---\nstatus: WIP\n---\n")
        with open(os.path.join(backlog_dir, "feat-b.md"), "w") as f:
            f.write("---\nstatus: Backlog\n---\n")

        cfg = {
            "roadmap_dir": os.path.join(self.tmp, "docs", "roadmaps"),
            "roadmap_namespacing": "flat",
            "agents": [],
        }
        violations = validate_filename_uniqueness(cfg)
        self.assertEqual(violations, [])


class TestValidatorEvolution(unittest.TestCase):
    """Testes para F2 (field mapping) e F3 (severity per rule) — v2.4."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()
        self._orig_dir = os.getcwd()
        # Criar estrutura mínima
        for d in ["docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
                  "docs/roadmaps/done", "docs/req", "docs/adr"]:
            os.makedirs(os.path.join(self.tmp, d), exist_ok=True)

    def tearDown(self):
        os.chdir(self._orig_dir)
        _config.reset()
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _write(self, rel, content=""):
        path = os.path.join(self.tmp, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)

    def _chdir(self):
        os.chdir(self.tmp)

    def _violations_messages(self, violations):
        """Extrai mensagem de uma lista de violations (str ou dict)."""
        result = []
        for v in violations:
            if isinstance(v, dict):
                result.append(v.get("message", str(v)))
            else:
                result.append(str(v))
        return result

    def test_field_mapping_req_id_satisfies_wip_has_req(self):
        """req_id como link_fields.req satisfaz a validação de REQ em wip."""
        self._write("trackfw.yaml",
            "link_fields:\n  req:\n    - req_id\n")
        self._write("docs/roadmaps/wip/RM-001.md",
            "---\nstatus: WIP\nreq_id: docs/req/REQ-001.md\n---\n## Acceptance Criteria\n- [ ] done\n")
        self._chdir()
        result = v.validate()
        msgs = self._violations_messages(result.get("violations", []))
        self.assertFalse(any("no linked REQ" in m for m in msgs),
            f"req_id deve satisfazer wip_has_req. violations: {msgs}")

    def test_severity_off_adr_orphan_silenciado(self):
        """adr_orphan: off → ADR órfão não aparece em violations nem warnings."""
        self._write("trackfw.yaml", "rules:\n  adr_orphan: off\n")
        self._write("docs/adr/ADR-001.md",
            "---\nstatus: Accepted\n---\n# ADR-001\n")
        self._chdir()
        result = v.validate()
        all_msgs = (
            self._violations_messages(result.get("violations", []))
            + self._violations_messages(result.get("warnings", []))
        )
        self.assertFalse(any("not referenced" in m for m in all_msgs),
            f"adr_orphan: off deve suprimir tudo. msgs: {all_msgs}")

    def test_severity_warning_wip_has_req(self):
        """wip_has_req: warning → aparece em warnings, não em violations."""
        self._write("trackfw.yaml", "rules:\n  wip_has_req: warning\n")
        self._write("docs/roadmaps/wip/RM-001.md",
            "---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n")
        self._chdir()
        result = v.validate()
        v_msgs = self._violations_messages(result.get("violations", []))
        w_msgs = self._violations_messages(result.get("warnings", []))
        self.assertFalse(any("no linked REQ" in m for m in v_msgs),
            f"wip_has_req: warning não deve estar em violations. violations: {v_msgs}")
        self.assertTrue(any("no linked REQ" in m for m in w_msgs),
            f"wip_has_req: warning deve aparecer em warnings. warnings: {w_msgs}")

    def test_acceptance_markers_customizados(self):
        """Marcador customizado ## Done When satisfaz verificação de acceptance criteria."""
        self._write("trackfw.yaml",
            'acceptance_markers:\n  - "## Done When"\n  - "## Critérios"\n')
        self._write("docs/roadmaps/wip/RM-001.md",
            "---\nstatus: WIP\nREQ: docs/req/REQ-001.md\n---\n## Done When\n- [ ] done\n")
        self._chdir()
        result = v.validate()
        msgs = self._violations_messages(result.get("violations", []))
        self.assertFalse(any("no acceptance criteria" in m for m in msgs),
            f"## Done When deve satisfazer acceptance criteria. violations: {msgs}")


class TestExpandTildeAdrDirs(unittest.TestCase):
    """Testes unitários para expansão de til (~) em adr_dirs no validator."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_find_adr_file_com_tilde(self):
        """_find_adr_file localiza arquivo ADR em adr_dir especificado com ~/."""
        home = os.path.expanduser("~")
        test_dir_name = f".tmp_trackfw_test_{int(time.time())}"
        test_dir = os.path.join(home, test_dir_name)
        os.makedirs(test_dir, exist_ok=True)
        try:
            adr_path = os.path.join(test_dir, "ADR-0001-global.md")
            _write(adr_path, "---\nstatus: Accepted\n---\n# Global ADR")
            found = v._find_adr_file("ADR-0001-global.md", [f"~/{test_dir_name}"])
            self.assertEqual(found, adr_path)
        finally:
            shutil.rmtree(test_dir, ignore_errors=True)

    def test_validate_adrs_are_referenced_com_tilde(self):
        """validate_adrs_are_referenced expande ~/ em adr_dirs ao verificar referências."""
        home = os.path.expanduser("~")
        test_dir_name = f".tmp_trackfw_test_ref_{int(time.time())}"
        test_dir = os.path.join(home, test_dir_name)
        os.makedirs(test_dir, exist_ok=True)
        try:
            _write(os.path.join(test_dir, "ADR-0002.md"), "---\nstatus: Accepted\n---\n# ADR 2")
            req_dir = os.path.join(self.tmp, "docs/req")
            _write(os.path.join(req_dir, "REQ-001.md"), "---\nstatus: Open\n---\nADR: ADR-0002.md\n")

            cfg = _config.defaults()
            cfg["adr_dirs"] = [f"~/{test_dir_name}"]
            cfg["req_dir"] = req_dir

            violations = v.validate_adrs_are_referenced(cfg)
            self.assertEqual(violations, [])
        finally:
            shutil.rmtree(test_dir, ignore_errors=True)


class TestStrictCIPathsAndInexistentAdrDirs(unittest.TestCase):
    """Testes unitários ML-2C: tratamento de diretórios adr_dirs inexistentes e strict_ci_paths."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_adr_dir_inexistente_gera_warning_por_padrao(self):
        """Diretório em adr_dirs inexistente com strict_ci_paths=False gera Warning em warnings."""
        cfg = _config.defaults()
        cfg["adr_dirs"] = [os.path.join(self.tmp, "docs/adr_inexistente")]
        cfg["strict_ci_paths"] = False

        res = v.validate_adr_dirs_exist(cfg)
        self.assertEqual(res["violations"], [])
        self.assertEqual(len(res["warnings"]), 1)
        self.assertIn("does not exist", res["warnings"][0]["message"])
        self.assertEqual(res["warnings"][0]["type"], "warning")

    def test_adr_dir_inexistente_gera_violation_quando_strict_ci_paths_true(self):
        """Diretório em adr_dirs inexistente com strict_ci_paths=True gera Violation em violations."""
        cfg = _config.defaults()
        cfg["adr_dirs"] = [os.path.join(self.tmp, "docs/adr_inexistente")]
        cfg["strict_ci_paths"] = True

        res = v.validate_adr_dirs_exist(cfg)
        self.assertEqual(res["warnings"], [])
        self.assertEqual(len(res["violations"]), 1)
        self.assertIn("does not exist", res["violations"][0]["message"])
        self.assertEqual(res["violations"][0]["type"], "violation")


def _guard_entry_claude_settings(script_cmd: str) -> str:
    return json.dumps({
        "hooks": {
            "PreToolUse": [
                {"matcher": "Bash", "hooks": [{"command": script_cmd, "type": "command"}]}
            ]
        }
    })


class TestCredentialGuardHookResolvable(unittest.TestCase):
    """ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_dispara_quando_script_ausente(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("does not exist" in m["message"] and ".claude/settings.json" in m["message"] for m in msgs),
            f"esperado violation de script ausente, obteve: {msgs}",
        )

    def test_dispara_quando_script_nao_executavel(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o644)  # sem bit +x

        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("not executable" in m["message"] for m in msgs),
            f"esperado violation de script não executável, obteve: {msgs}",
        )

    def test_nao_dispara_sem_entrada_de_guard(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            json.dumps({
                "hooks": {
                    "PostToolUse": [{"matcher": "AskUserQuestion", "hooks": [
                        {"command": "scripts/trackfw-attention-cleanup.sh", "type": "command"}]}],
                    "PreToolUse": [{"matcher": "AskUserQuestion", "hooks": [
                        {"command": "scripts/trackfw-attention-signal.sh", "type": "command"}]}],
                }
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"sem entrada de guard não deve haver violations, obteve: {msgs}")

    def test_nao_dispara_formato_desconhecido(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("$SOME_OTHER_VAR/scripts/trackfw-credential-guard.sh"),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"formato desconhecido não deve violar, obteve: {msgs}")

    def test_resolve_forma_codex_aspas_literais(self):
        _write(
            os.path.join(self.tmp, ".codex/hooks.json"),
            json.dumps({
                "hooks": {
                    "PreToolUse": [{"matcher": ".*", "hooks": [
                        {"command": '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"',
                         "type": "command"}]}],
                }
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("does not exist" in m["message"] and ".codex/hooks.json" in m["message"] for m in msgs),
            f"esperado violation resolvendo a forma do Codex, obteve: {msgs}",
        )

        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o755)

        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"com script existente e executável não deve haver violations, obteve: {msgs}")

    def test_resolve_caminho_relativo_puro(self):
        _write(
            os.path.join(self.tmp, ".cursor/hooks.json"),
            json.dumps({
                "version": 1,
                "hooks": {"beforeShellExecution": [{"command": "scripts/trackfw-credential-guard.sh"}]},
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("does not exist" in m["message"] and ".cursor/hooks.json" in m["message"] for m in msgs),
            f"esperado violation resolvendo caminho relativo puro, obteve: {msgs}",
        )

    def test_arquivo_ausente_e_pulado(self):
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [])

    def test_configuravel_via_rules(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"),
        )

        # default error
        cfg = _config.defaults()
        violations, warnings = [], []
        v._apply_rule("credential_guard_hook_resolvable",
                       v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp),
                       violations, warnings, cfg)
        self.assertTrue(any("trackfw-credential-guard.sh" in m["message"] for m in violations))

        # warning — cwd=self.tmp é obrigatório para _apply_rule porque
        # credential_guard_hook_resolvable é uma _CREDENTIAL_GUARD_ANCHORED_RULE: sem cwd, a
        # severidade é lida da git HEAD do repositório real (onde a regra não está configurada
        # como warning), sobrepondo o cfg do disco. self.tmp não é um worktree git, então
        # _head_trackfw_yaml retorna ok=False e o fallback é o disco.
        cfg = _config.defaults()
        cfg["rules"] = {"credential_guard_hook_resolvable": "warning"}
        violations, warnings = [], []
        v._apply_rule("credential_guard_hook_resolvable",
                       v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp),
                       violations, warnings, cfg, cwd=self.tmp)
        self.assertEqual(violations, [])
        self.assertTrue(any("trackfw-credential-guard.sh" in m["message"] for m in warnings))

        # off — mesmo raciocínio: cwd=self.tmp para isolar do git HEAD real.
        cfg = _config.defaults()
        cfg["rules"] = {"credential_guard_hook_resolvable": "off"}
        violations, warnings = [], []
        v._apply_rule("credential_guard_hook_resolvable",
                       v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp),
                       violations, warnings, cfg, cwd=self.tmp)
        self.assertEqual(violations, [])
        self.assertEqual(warnings, [])

    def test_dispara_forma_relativa_antiga_em_claude_ac1(self):
        """AC1 (ROADMAP-2026-08-21 ML-1B): Claude settings com forma relativa antiga
        ("scripts/...") e script PRESENTE e executável deve gerar violation "bare relative path".
        Prova que a violação vem da forma do comando, não da ausência do script."""
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("scripts/trackfw-credential-guard.sh"),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o755)

        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("bare relative path" in m["message"] and ".claude/settings.json" in m["message"]
                for m in msgs),
            f"AC1: esperado violation de forma relativa antiga em Claude, obteve: {msgs}",
        )
        self.assertTrue(
            any("trackfw update" in m["message"] for m in msgs),
            f"AC4: mensagem deve nomear trackfw update, obteve: {msgs}",
        )

    def test_nao_dispara_forma_relativa_em_cursor_ac3(self):
        """AC3 não-vácuo (ROADMAP-2026-08-21 ML-1B): Cursor com caminho relativo puro e script
        PRESENTE e executável não deve gerar violação — requiresVarOrShellPrefix=False para
        Cursor por construção (falso-positivo eliminado por construção)."""
        _write(
            os.path.join(self.tmp, ".cursor/hooks.json"),
            json.dumps({
                "version": 1,
                "hooks": {"beforeShellExecution": [{"command": "scripts/trackfw-credential-guard.sh"}]},
            }),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o755)

        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(
            msgs, [],
            f"AC3: Cursor com relativo deve estar limpo (falso-positivo eliminado por construção), obteve: {msgs}",
        )

    # ------------------------------------------------------------------
    # ADR-2026-08-22 ML-1A — classificação por ancoragem
    # ------------------------------------------------------------------

    def test_classify_hook_anchorage_classe1_ancorado(self):
        """classifyHookAnchorage retorna classe 1 para formas ancoradas (incluindo ~/… sem aspas)."""
        cases = [
            ("$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh", False),
            ("$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh", False),
            ("$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh", False),
            ("/opt/scripts/trackfw-credential-guard.sh", False),
            ("/absolute/path/guard.sh", False),
            # ~/… sem aspas: tilde expande para $HOME em qualquer shell POSIX — ancorado.
            ("~/scripts/trackfw-credential-guard.sh", False),
            ("~/.trackfw/scripts/trackfw-credential-guard.sh", False),
        ]
        for raw, was_quoted in cases:
            self.assertEqual(
                v._classify_hook_anchorage(raw, was_quoted), v._HOOK_ANCHORAGE_CLASS_ANCHORED,
                f"esperava classe 1 para: {raw!r} (was_quoted={was_quoted})",
            )

    def test_classify_hook_anchorage_classe2_cwd_dependent(self):
        """classifyHookAnchorage retorna classe 2 para formas dependentes do cwd."""
        cases = [
            ("$PWD/scripts/trackfw-credential-guard.sh", False),
            ("${PWD}/scripts/trackfw-credential-guard.sh", False),
            ("./scripts/trackfw-credential-guard.sh", False),
            ("../scripts/trackfw-credential-guard.sh", False),
            ("scripts/trackfw-credential-guard.sh", False),
            ("sh scripts/trackfw-credential-guard.sh", False),
            # "~/…" com aspas: tilde NÃO expande dentro de aspas duplas — classe 2.
            ("~/scripts/trackfw-credential-guard.sh", True),
            ("~/.trackfw/scripts/trackfw-credential-guard.sh", True),
        ]
        for raw, was_quoted in cases:
            self.assertEqual(
                v._classify_hook_anchorage(raw, was_quoted), v._HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT,
                f"esperava classe 2 para: {raw!r} (was_quoted={was_quoted})",
            )

    def test_classify_hook_anchorage_classe3_indecidivel(self):
        """classifyHookAnchorage retorna classe 3 para variáveis próprias do usuário."""
        cases = [
            ("$SOME_OTHER_VAR/scripts/trackfw-credential-guard.sh", False),
            ("$MY_CUSTOM_DIR/guard.sh", False),
            ("$UNDEFINED/trackfw-credential-guard.sh", False),
        ]
        for raw, was_quoted in cases:
            self.assertEqual(
                v._classify_hook_anchorage(raw, was_quoted), v._HOOK_ANCHORAGE_CLASS_UNDECIDABLE,
                f"esperava classe 3 para: {raw!r} (was_quoted={was_quoted})",
            )

    def test_hook_value_was_quoted(self):
        """_hook_value_was_quoted detecta aspas externas."""
        self.assertTrue(v._hook_value_was_quoted('"$PWD/scripts/guard.sh"'))
        self.assertTrue(v._hook_value_was_quoted('"~/scripts/guard.sh"'))
        self.assertFalse(v._hook_value_was_quoted("~/scripts/guard.sh"))
        self.assertFalse(v._hook_value_was_quoted("$PWD/scripts/guard.sh"))
        self.assertFalse(v._hook_value_was_quoted('"'))
        self.assertTrue(v._hook_value_was_quoted('""'))
        self.assertFalse(v._hook_value_was_quoted('"abc'))

    def test_cwd_dependent_reason_pwd_em_qualquer_posicao(self):
        """_cwd_dependent_reason retorna mensagem do $PWD para qualquer forma contendo $PWD."""
        pwd_cases = [
            "$PWD/scripts/guard.sh",
            "${PWD}/scripts/guard.sh",
            'sh -c "$PWD/scripts/guard.sh"',
            "env FOO=x $PWD/scripts/guard.sh",
        ]
        for raw in pwd_cases:
            reason = v._cwd_dependent_reason(raw)
            self.assertIn("$PWD path", reason, f"esperava '$PWD path' para: {raw!r}")
        bare_cases = [
            "./scripts/guard.sh",
            "../scripts/guard.sh",
            "scripts/guard.sh",
            "~/scripts/guard.sh",
        ]
        for raw in bare_cases:
            reason = v._cwd_dependent_reason(raw)
            self.assertIn("bare relative path", reason, f"esperava 'bare relative path' para: {raw!r}")

    def test_strip_outer_quotes_for_classify(self):
        """_strip_outer_quotes_for_classify remove aspas duplas envolventes."""
        cases = [
            ('"$PWD/scripts/guard.sh"', "$PWD/scripts/guard.sh"),
            ('"$(git rev-parse --show-toplevel)/scripts/guard.sh"',
             "$(git rev-parse --show-toplevel)/scripts/guard.sh"),
            ("$CLAUDE_PROJECT_DIR/scripts/guard.sh", "$CLAUDE_PROJECT_DIR/scripts/guard.sh"),
            ("scripts/guard.sh", "scripts/guard.sh"),
            ('"', '"'),
            ('""', ""),
            ('"abc', '"abc'),
        ]
        for raw, want in cases:
            self.assertEqual(
                v._strip_outer_quotes_for_classify(raw), want,
                f"strip({raw!r})",
            )

    # ------------------------------------------------------------------
    # ML-4A (ROADMAP-2026-08-22) — ~/…, ${PWD}/…, mensagem certa por forma
    # ------------------------------------------------------------------

    def test_tilde_sem_aspas_silencioso(self):
        """ML-4A: ~/… sem aspas é classe 1 (tilde expande para $HOME — ancorado). Não deve
        gerar violação (falso-positivo confirmado pela barreira ML-3A/Hades)."""
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("~/scripts/trackfw-credential-guard.sh"),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(
            msgs, [],
            f"ML-4A: ~/… sem aspas (classe 1) deve ser silencioso, obteve: {msgs}",
        )

    def test_tilde_com_aspas_acusado(self):
        """ML-4A: \"~/…\" aspeado é classe 2 (tilde NÃO expande dentro de aspas duplas).
        Deve gerar violação com mensagem 'bare relative path'."""
        cmd_value = '"~/scripts/trackfw-credential-guard.sh"'
        content = _guard_entry_claude_settings(cmd_value)
        parsed = json.loads(content)
        cmd_in_json = parsed["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
        self.assertTrue(
            cmd_in_json.startswith('"') and cmd_in_json.endswith('"'),
            f"valor command deve ter aspas literais, obteve: {cmd_in_json!r}",
        )
        _write(os.path.join(self.tmp, ".claude/settings.json"), content)
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("bare relative path" in m["message"] for m in msgs),
            f"ML-4A: \"~/…\" aspeado deve ser acusado com 'bare relative path', obteve: {msgs}",
        )

    def test_pwd_chaveado_acusado(self):
        """ML-4A: ${PWD}/… é classe 2 (mesma semântica de $PWD/…). Deve gerar violação
        com mensagem do $PWD."""
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("${PWD}/scripts/trackfw-credential-guard.sh"),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("$PWD path" in m["message"] for m in msgs),
            f"ML-4A: ${'{'}PWD{'}'}/… deve ser acusado com mensagem do $PWD, obteve: {msgs}",
        )

    def test_sh_c_pwd_mensagem_pwd(self):
        """ML-4A: sh -c \"$PWD/…\" deve ser acusado com mensagem do $PWD, não 'bare relative
        path', pois $PWD está presente no comando."""
        cmd_value = 'sh -c "$PWD/scripts/trackfw-credential-guard.sh"'
        content = _guard_entry_claude_settings(cmd_value)
        parsed = json.loads(content)
        self.assertIsNotNone(parsed, "fixture JSON deve ser válido")
        _write(os.path.join(self.tmp, ".claude/settings.json"), content)
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("$PWD path" in m["message"] for m in msgs),
            f"ML-4A: sh -c \"$PWD/…\" deve usar mensagem do $PWD, obteve: {msgs}",
        )
        self.assertFalse(
            any("bare relative path" in m["message"] for m in msgs),
            f"ML-4A: sh -c \"$PWD/…\" não deve dizer 'bare relative path', obteve: {msgs}",
        )

    def test_dispara_pwd_em_claude_ac2(self):
        """AC2: Claude settings com $PWD/… e script presente deve gerar violation explicando
        que $PWD não ancora."""
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("$PWD/scripts/trackfw-credential-guard.sh"),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o755)

        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("$PWD path" in m["message"] and ".claude/settings.json" in m["message"]
                for m in msgs),
            f"AC2: esperado violation de $PWD em Claude, obteve: {msgs}",
        )
        self.assertTrue(
            any("current working directory" in m["message"] for m in msgs),
            f"AC2: mensagem deve explicar que $PWD não ancora, obteve: {msgs}",
        )
        self.assertTrue(
            any("trackfw update" in m["message"] for m in msgs),
            f"AC2: mensagem deve citar trackfw update, obteve: {msgs}",
        )

    def test_dispara_pwd_entre_aspas_em_claude_d3(self):
        """Achado D.3: \"$PWD/…\" entre aspas (valor JSON com aspas literais) também acusado
        após strip de aspas externas. _guard_entry_claude_settings usa json.dumps, que serializa
        as aspas corretamente."""
        # Passa a string Python com chars de aspas duplas; json.dumps as serializa como \" no JSON.
        cmd_value = '"$PWD/scripts/trackfw-credential-guard.sh"'
        content = _guard_entry_claude_settings(cmd_value)
        # Sanidade: o valor no JSON tem aspas duplas como primeiro e último char.
        parsed = json.loads(content)
        cmd_in_json = parsed["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
        self.assertTrue(
            cmd_in_json.startswith('"') and cmd_in_json.endswith('"'),
            f"valor command deve ter aspas literais, obteve: {cmd_in_json!r}",
        )
        _write(os.path.join(self.tmp, ".claude/settings.json"), content)
        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o755)

        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("$PWD path" in m["message"] and ".claude/settings.json" in m["message"]
                for m in msgs),
            f"D.3: esperado violation de $PWD entre aspas em Claude, obteve: {msgs}",
        )

    def test_caminho_absoluto_silencioso_classe1(self):
        """Classe 1: caminho absoluto não deve gerar violação (wiring legítimo, falso-positivo
        dominante do ADR-2026-08-22)."""
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("/opt/scripts/trackfw-credential-guard.sh"),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"classe 1 (absoluto) deve ser silenciosa, obteve: {msgs}")

    def test_outra_var_silenciosa_classe3(self):
        """Classe 3: $OUTRA_VAR/… não deve gerar violação (indecidível, silêncio declarado)."""
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _guard_entry_claude_settings("$MY_CUSTOM_DIR/scripts/trackfw-credential-guard.sh"),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"classe 3 ($OUTRA_VAR) deve ser silenciosa, obteve: {msgs}")

    def test_forma_codex_aspas_silenciosa_classe1(self):
        """Forma do Codex com aspas e git rev-parse (classe 1) continua silenciosa após strip."""
        script_path = os.path.join(self.tmp, "scripts", "trackfw-credential-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o755)
        _write(
            os.path.join(self.tmp, ".codex/hooks.json"),
            json.dumps({
                "hooks": {"PreToolUse": [{"matcher": ".*", "hooks": [
                    {"command": '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"',
                     "type": "command"}
                ]}]}
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"forma Codex (classe 1 com aspas) deve ser silenciosa, obteve: {msgs}")

    def test_dispara_pwd_em_codex_ac2(self):
        """AC2 para Codex: $PWD/… também acusado."""
        _write(
            os.path.join(self.tmp, ".codex/hooks.json"),
            json.dumps({
                "hooks": {"PreToolUse": [{"matcher": ".*", "hooks": [
                    {"command": "$PWD/scripts/trackfw-credential-guard.sh", "type": "command"}
                ]}]}
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("$PWD path" in m["message"] and ".codex/hooks.json" in m["message"]
                for m in msgs),
            f"AC2 Codex: esperado violation de $PWD, obteve: {msgs}",
        )

    def test_dispara_pwd_em_gemini_ac2(self):
        """AC2 para Gemini: $PWD/… também acusado."""
        _write(
            os.path.join(self.tmp, ".gemini/settings.json"),
            json.dumps({
                "hooks": {"PreToolUse": [{"matcher": ".*", "hooks": [
                    {"command": "$PWD/scripts/trackfw-credential-guard.sh", "type": "command"}
                ]}]}
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_credential_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("$PWD path" in m["message"] and ".gemini/settings.json" in m["message"]
                for m in msgs),
            f"AC2 Gemini: esperado violation de $PWD, obteve: {msgs}",
        )


class TestAdrOrphanExemptOutsideCwd(unittest.TestCase):
    """Testes unitários ML-2C: isenção de adr_orphan para arquivos fora de cwd."""

    def setUp(self):
        self.cwd = tempfile.mkdtemp()
        self.external_dir = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.cwd, ignore_errors=True)
        shutil.rmtree(self.external_dir, ignore_errors=True)
        _config.reset()

    def test_adr_orphan_isenta_arquivos_fora_de_cwd(self):
        """ADR contida em diretório fora de cwd não deve ser reportada como adr_orphan."""
        # Cria uma ADR no diretório externo
        ext_adr = os.path.join(self.external_dir, "ADR-0099-global.md")
        _write(ext_adr, "---\nstatus: Accepted\n---\n# Global ADR")

        cfg = _config.defaults()
        cfg["adr_dirs"] = [self.external_dir]
        cfg["req_dir"] = os.path.join(self.cwd, "docs/req")
        os.makedirs(cfg["req_dir"], exist_ok=True)

        violations = v.validate_adrs_are_referenced(cfg, cwd=self.cwd)
        self.assertEqual(violations, [], "ADR em diretório externo a cwd deve ser isenta de adr_orphan")

    def test_adr_orphan_reporta_arquivos_dentro_de_cwd(self):
        """ADR contida dentro de cwd e não referenciada por nenhuma REQ gera violation."""
        internal_adr_dir = os.path.join(self.cwd, "docs/adr")
        int_adr = os.path.join(internal_adr_dir, "ADR-0001-local.md")
        _write(int_adr, "---\nstatus: Accepted\n---\n# Local ADR")

        cfg = _config.defaults()
        cfg["adr_dirs"] = [internal_adr_dir]
        cfg["req_dir"] = os.path.join(self.cwd, "docs/req")
        os.makedirs(cfg["req_dir"], exist_ok=True)

        violations = v.validate_adrs_are_referenced(cfg, cwd=self.cwd)
        self.assertEqual(len(violations), 1)
        self.assertIn("ADR-0001-local.md", violations[0]["message"])

    def test_adr_orphan_isenta_arquivo_individual_externo_via_symlink(self):
        """ADR individual cujo caminho absoluto resolvido está fora de CWD é isento por-arquivo."""
        internal_adr_dir = os.path.join(self.cwd, "docs", "adr")
        os.makedirs(internal_adr_dir, exist_ok=True)
        ext_file = os.path.join(self.external_dir, "ADR-0100-external-symlink.md")
        _write(ext_file, "---\nstatus: Accepted\n---\n# External ADR")
        symlink_path = os.path.join(internal_adr_dir, "ADR-0100-external-symlink.md")
        try:
            os.symlink(ext_file, symlink_path)
        except (OSError, AttributeError, NotImplementedError):
            return

        cfg = _config.defaults()
        cfg["adr_dirs"] = [internal_adr_dir]
        cfg["req_dir"] = os.path.join(self.cwd, "docs/req")
        os.makedirs(cfg["req_dir"], exist_ok=True)

        violations = v.validate_adrs_are_referenced(cfg, cwd=self.cwd)
        self.assertEqual(violations, [], "Arquivo com caminho resolvido fora do CWD deve ser isento por-arquivo")


class TestValidateBranchHasWIPRoadmap(unittest.TestCase):
    """4 cenários obrigatórios (P4 do ADR): a regra não afrouxou."""

    def _cfg(self, roadmap_dir: str) -> dict:
        return {
            "roadmap_dir": roadmap_dir,
            "roadmap_namespacing": "flat",
            "agents": [],
        }

    def test_cenario1_wip_com_slug_passa(self):
        """Cenário 1 — roadmap em wip/ com slug da branch → sem violação (comportamento preservado)."""
        import os as _os
        from trackfw.validator import validate_branch_has_wip_roadmap
        tmp = tempfile.mkdtemp()
        try:
            wip_dir = os.path.join(tmp, "docs", "roadmaps", "wip")
            os.makedirs(wip_dir)
            _write(os.path.join(wip_dir, "ROADMAP-my-feature.md"), "REQ: REQ-001\n")
            cfg = self._cfg(os.path.join(tmp, "docs", "roadmaps"))
            orig = _os.environ.get("TRACKFW_BRANCH")
            _os.environ["TRACKFW_BRANCH"] = "feat/my-feature"
            try:
                result = validate_branch_has_wip_roadmap(cfg)
                self.assertEqual(result, [], f"roadmap em wip/ com slug deve passar, obteve: {result}")
            finally:
                if orig is None:
                    _os.environ.pop("TRACKFW_BRANCH", None)
                else:
                    _os.environ["TRACKFW_BRANCH"] = orig
        finally:
            import shutil; shutil.rmtree(tmp, ignore_errors=True)

    def test_cenario2_done_com_slug_passa(self):
        """Cenário 2 — roadmap em done/ com slug da branch → sem violação (novo comportamento)."""
        import os as _os
        from trackfw.validator import validate_branch_has_wip_roadmap
        tmp = tempfile.mkdtemp()
        try:
            os.makedirs(os.path.join(tmp, "docs", "roadmaps", "wip"))
            done_dir = os.path.join(tmp, "docs", "roadmaps", "done")
            os.makedirs(done_dir)
            _write(os.path.join(done_dir, "ROADMAP-my-feature.md"), "REQ: REQ-001\n")
            cfg = self._cfg(os.path.join(tmp, "docs", "roadmaps"))
            orig = _os.environ.get("TRACKFW_BRANCH")
            _os.environ["TRACKFW_BRANCH"] = "feat/my-feature"
            try:
                result = validate_branch_has_wip_roadmap(cfg)
                self.assertEqual(result, [], f"roadmap em done/ com slug deve passar, obteve: {result}")
            finally:
                if orig is None:
                    _os.environ.pop("TRACKFW_BRANCH", None)
                else:
                    _os.environ["TRACKFW_BRANCH"] = orig
        finally:
            import shutil; shutil.rmtree(tmp, ignore_errors=True)

    def test_cenario3_sem_roadmap_reprova(self):
        """Cenário 3 — nenhum roadmap em wip/ nem done/ → continua reprovando."""
        import os as _os
        from trackfw.validator import validate_branch_has_wip_roadmap
        tmp = tempfile.mkdtemp()
        try:
            os.makedirs(os.path.join(tmp, "docs", "roadmaps", "wip"))
            cfg = self._cfg(os.path.join(tmp, "docs", "roadmaps"))
            orig = _os.environ.get("TRACKFW_BRANCH")
            _os.environ["TRACKFW_BRANCH"] = "feat/my-feature"
            try:
                result = validate_branch_has_wip_roadmap(cfg)
                self.assertTrue(len(result) > 0, "sem roadmap em wip/ nem done/ deve reprovar")
                self.assertIn("no roadmap is in wip/ nor done/", result[0])
            finally:
                if orig is None:
                    _os.environ.pop("TRACKFW_BRANCH", None)
                else:
                    _os.environ["TRACKFW_BRANCH"] = orig
        finally:
            import shutil; shutil.rmtree(tmp, ignore_errors=True)

    def test_cenario4_done_slug_diferente_reprova(self):
        """Cenário 4 — roadmap em done/ com slug DIFERENTE → continua reprovando (casamento obrigatório)."""
        import os as _os
        from trackfw.validator import validate_branch_has_wip_roadmap
        tmp = tempfile.mkdtemp()
        try:
            done_dir = os.path.join(tmp, "docs", "roadmaps", "done")
            os.makedirs(done_dir)
            _write(os.path.join(done_dir, "ROADMAP-outra-coisa.md"), "REQ: REQ-001\n")
            cfg = self._cfg(os.path.join(tmp, "docs", "roadmaps"))
            orig = _os.environ.get("TRACKFW_BRANCH")
            _os.environ["TRACKFW_BRANCH"] = "feat/my-feature"
            try:
                result = validate_branch_has_wip_roadmap(cfg)
                self.assertTrue(len(result) > 0, "slug diferente em done/ deve reprovar")
                self.assertIn("no matching roadmap in wip/ nor done/", result[0])
            finally:
                if orig is None:
                    _os.environ.pop("TRACKFW_BRANCH", None)
                else:
                    _os.environ["TRACKFW_BRANCH"] = orig
        finally:
            import shutil; shutil.rmtree(tmp, ignore_errors=True)


# ---------------------------------------------------------------------------
# Testes P2/P3 — adicionados pelo ML-2A (REQ-2026-07-26-robustez-gates)
# ---------------------------------------------------------------------------

class TestContentHasMarkerCRLF(unittest.TestCase):
    """P3: contentHasMarker deve detectar campos vazios em arquivos CRLF."""

    def test_campo_vazio_crlf_nao_conta_como_presente(self):
        from trackfw.validator import _content_has_marker
        content = "# Roadmap\r\nREQ: \r\n## Seção\r\n"
        self.assertFalse(_content_has_marker(content, ["REQ:"]),
                         "campo vazio com CRLF não deve ser tratado como presente")

    def test_campo_preenchido_crlf_conta_como_presente(self):
        from trackfw.validator import _content_has_marker
        content = "# Roadmap\r\nREQ: REQ-001-titulo.md\r\n## Seção\r\n"
        self.assertTrue(_content_has_marker(content, ["REQ:"]),
                        "campo preenchido com CRLF deve ser tratado como presente")

    def test_campo_vazio_lf_nao_conta_como_presente(self):
        from trackfw.validator import _content_has_marker
        content = "# Roadmap\nREQ: \n## Seção\n"
        self.assertFalse(_content_has_marker(content, ["REQ:"]),
                         "campo vazio com LF não deve ser tratado como presente")


class TestFolderStatusDirNaoLegivel(unittest.TestCase):
    """P2: pasta de estado que EXISTE mas não pode ser lida deve gerar warning."""

    def test_enotdir_gera_warning(self):
        from trackfw.validator import validate_folder_status_coherence
        tmp = tempfile.mkdtemp()
        try:
            os.makedirs(os.path.join(tmp, "docs", "roadmaps"))
            # "analyzing" como arquivo regular — NotADirectoryError ao listar
            _write(os.path.join(tmp, "docs", "roadmaps", "analyzing"),
                   "eu sou um arquivo, nao um diretorio")
            cfg = {"roadmap_dir": os.path.join(tmp, "docs", "roadmaps")}
            warnings = validate_folder_status_coherence(cfg)
            msgs = [w["message"] for w in warnings]
            self.assertTrue(any("could not read directory" in m for m in msgs),
                            f"esperado warning sobre diretório ilegível, obteve: {msgs}")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


class TestFilenameUniquenessDirNaoLegivel(unittest.TestCase):
    """P2: pasta de estado que EXISTE mas não pode ser lida deve gerar violation."""

    def test_enotdir_gera_violation(self):
        from trackfw.validator import validate_filename_uniqueness
        tmp = tempfile.mkdtemp()
        try:
            os.makedirs(os.path.join(tmp, "docs", "roadmaps"))
            # "wip" como arquivo regular — NotADirectoryError ao listar
            _write(os.path.join(tmp, "docs", "roadmaps", "wip"),
                   "eu sou um arquivo, nao um diretorio")
            cfg = {"roadmap_dir": os.path.join(tmp, "docs", "roadmaps")}
            violations = validate_filename_uniqueness(cfg)
            msgs = [v["message"] for v in violations]
            self.assertTrue(any("could not read directory" in m for m in msgs),
                            f"esperado violation sobre diretório ilegível, obteve: {msgs}")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


class TestFilenameUniquenessOrdemDeterministica(unittest.TestCase):
    """P3: estados na mensagem devem estar em ordem alfabética."""

    def test_estados_em_ordem_alfabetica(self):
        from trackfw.validator import validate_filename_uniqueness
        tmp = tempfile.mkdtemp()
        try:
            wip_dir = os.path.join(tmp, "docs", "roadmaps", "wip")
            done_dir = os.path.join(tmp, "docs", "roadmaps", "done")
            os.makedirs(wip_dir)
            os.makedirs(done_dir)
            _write(os.path.join(wip_dir, "ROADMAP-duplicado.md"), "# Dup\n")
            _write(os.path.join(done_dir, "ROADMAP-duplicado.md"), "# Dup\n")
            cfg = {"roadmap_dir": os.path.join(tmp, "docs", "roadmaps")}
            violations = validate_filename_uniqueness(cfg)
            self.assertEqual(len(violations), 1,
                             f"esperado 1 violation, obteve {len(violations)}: {violations}")
            msg = violations[0]["message"]
            self.assertIn("['done', 'wip']", msg,
                          f"estados devem estar em ordem alfabética, obteve: {msg}")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


# ---------------------------------------------------------------------------
# Testes P3+P4 — adicionados pelo ML-3A (REQ-2026-07-26-robustez-gates)
# ---------------------------------------------------------------------------

class TestBranchHasWIPRoadmapTruncaMensagem(unittest.TestCase):
    """P3+P4: 4 candidatos devem gerar mensagem truncada em 3 + 'e mais 1', em ordem alfabética."""

    def test_trunca_em_3_mais_contagem(self):
        from trackfw.validator import validate_branch_has_wip_roadmap
        import os as _os
        tmp = tempfile.mkdtemp()
        try:
            wip_dir = os.path.join(tmp, "docs", "roadmaps", "wip")
            os.makedirs(wip_dir)
            # 4 roadmaps sem slug da branch → todos são candidatos, nenhum casa
            for name in ["ROADMAP-alpha.md", "ROADMAP-bravo.md", "ROADMAP-charlie.md", "ROADMAP-delta.md"]:
                _write(os.path.join(wip_dir, name), "REQ: REQ-001\n")
            cfg = {"roadmap_dir": os.path.join(tmp, "docs", "roadmaps")}
            orig = _os.environ.get("TRACKFW_BRANCH")
            _os.environ["TRACKFW_BRANCH"] = "feat/minha-feature"
            try:
                result = validate_branch_has_wip_roadmap(cfg)
                self.assertTrue(len(result) > 0, "esperava violation com 4 candidatos sem slug correspondente")
                want = "ROADMAP-alpha.md, ROADMAP-bravo.md, ROADMAP-charlie.md, e mais 1"
                self.assertIn(want, result[0],
                              f'mensagem truncada deve conter "{want}", obteve: {result[0]}')
            finally:
                if orig is None:
                    _os.environ.pop("TRACKFW_BRANCH", None)
                else:
                    _os.environ["TRACKFW_BRANCH"] = orig
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


class TestWIPHasREQCRLFIntegracao(unittest.TestCase):
    """P3+P4: roadmap CRLF com REQ vazio deve emitir violation de wip_has_req (integração)."""

    def test_crlf_vazio_emite_violation(self):
        from trackfw.validator import validate_wip_has_req
        tmp = tempfile.mkdtemp()
        try:
            wip_dir = os.path.join(tmp, "docs", "roadmaps", "wip")
            os.makedirs(wip_dir)
            # Arquivo CRLF: REQ: seguido de espaço + \r\n — campo vazio
            with open(os.path.join(wip_dir, "ROADMAP-crlf.md"), "wb") as f:
                f.write(b"REQ: \r\n## Acceptance Criteria\r\n- [ ] ok\r\n")
            cfg = {"roadmap_dir": os.path.join(tmp, "docs", "roadmaps")}
            violations = validate_wip_has_req(cfg)
            msgs = [v if isinstance(v, str) else v.get("message", str(v)) for v in violations]
            self.assertTrue(any("wip but has no linked REQ" in m for m in msgs),
                            f"esperava violation de REQ vazio com CRLF, obteve: {violations}")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


# ---------------------------------------------------------------------------
# ML-1A — REQ-2026-07-27-integridade-referencias — testes negativos xfail
# Semântica strict: cada teste executa o corpo e falha contra o código atual.
# Se o defeito for corrigido sem reativação do teste, pytest reporta XPASS e
# falha a suíte por causa de strict=True.
# ---------------------------------------------------------------------------


def _ml1a_base(tmp_path, monkeypatch):
    for rel in [
        "docs/req",
        "docs/roadmaps/backlog",
        "docs/roadmaps/blocked",
        "docs/roadmaps/done",
        "docs/roadmaps/wip",
        "docs/adr",
    ]:
        (tmp_path / rel).mkdir(parents=True, exist_ok=True)
    (tmp_path / "trackfw.yaml").write_text(
        "req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n",
        encoding="utf-8",
    )
    monkeypatch.chdir(tmp_path)
    _config.reset()


def test_ml2a_escape1_frontmatter_roadmap_validado(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    (tmp_path / "docs/req/REQ-XFAIL-ESCAPE1.md").write_text(
        "---\nstatus: Open\nroadmap: \"docs/roadmaps/wip/NAO-EXISTE-ESCAPE-1.md\"\n---\n\n"
        "# REQ: Escape 1\n\n> Date: 2026-07-27 | Status: Open\n\n"
        "## Linked Roadmap\n",
        encoding="utf-8",
    )

    warnings = v.validate_ref_targets_exist(_config.load())

    assert any(
        "NAO-EXISTE-ESCAPE-1" in item["message"] for item in warnings
    ), f"esperava warning para roadmap: inexistente no frontmatter; warnings={warnings}"


def test_ml2a_escape2_fallback_basename_removido(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    (tmp_path / "docs/roadmaps/done/ESCAPE2-ROADMAP.md").write_text(
        "# Roadmap\n## Acceptance Criteria\n- [x] done\n",
        encoding="utf-8",
    )
    (tmp_path / "docs/req/REQ-XFAIL-ESCAPE2.md").write_text(
        "---\nstatus: Open\n---\n\n# REQ: Escape 2\n\n"
        "> Date: 2026-07-27 | Status: Open\n\n"
        "## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ESCAPE2-ROADMAP.md\n",
        encoding="utf-8",
    )

    warnings = v.validate_ref_targets_exist(_config.load())

    assert any(
        "ESCAPE2-ROADMAP" in item["message"] for item in warnings
    ), f"esperava warning para caminho errado wip/ vs done/; warnings={warnings}"


def test_ml3a_escape3_ref_targets_exist_default_error_reprova_gate(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    (tmp_path / "docs/req/REQ-XFAIL-ESCAPE3.md").write_text(
        "---\nstatus: Open\n---\n\n# REQ: Escape 3\n\n"
        "> Date: 2026-07-27 | Status: Open\n\n"
        "## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ESCAPE3-TRULY-MISSING.md\n",
        encoding="utf-8",
    )

    result = v.validate_unfiltered()

    assert any(
        "ESCAPE3-TRULY-MISSING" in item["message"]
        for item in result["violations"]
    ), f"esperava violation para referencia quebrada com severidade default error; result={result}"


def test_ml2b_defeito2_req_open_com_roadmap_done(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    (tmp_path / "docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md").write_text(
        "---\nstatus: Done\ndate: 2026-07-01\n---\n"
        "# Roadmap concluido\n## Acceptance Criteria\n- [x] done\n",
        encoding="utf-8",
    )
    (tmp_path / "docs/req/REQ-XFAIL-DEFEITO2.md").write_text(
        "---\nstatus: Open\ndate: 2026-07-01\n"
        "roadmap: \"docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md\"\n---\n\n"
        "# REQ: Defeito 2\n\n> Date: 2026-07-01 | Status: Open\n\n"
        "## Linked Roadmap\nRoadmap: docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md\n",
        encoding="utf-8",
    )

    result = v.validate_unfiltered()
    all_messages = [
        item["message"] for item in result["violations"] + result["warnings"]
    ]

    assert any(
        "DONE-ROADMAP-DEFEITO2" in message for message in all_messages
    ), f"esperava mensagem sobre REQ Open com roadmap done; result={result}"


def test_ml2a_stale_wip_usa_entrada_log_de_wip(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    roadmap = tmp_path / "docs/roadmaps/wip/ROADMAP-old-wip.md"
    roadmap.write_text(
        "---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n",
        encoding="utf-8",
    )
    (tmp_path / "docs/roadmaps/.trackfw-log").write_text(
        "2026-07-10 10:00  ROADMAP-old-wip.md                                backlog → wip\n",
        encoding="utf-8",
    )
    now = time.time()
    os.utime(roadmap, (now, now))

    warnings = v.validate_stale_wip(
        _config.load(),
        days=7,
        now=datetime(2026, 7, 27, 12, 0).timestamp(),
    )
    messages = [item["message"] for item in warnings]

    assert any(
        "ROADMAP-old-wip.md" in message for message in messages
    ), f"esperava stale_wip pela entrada antiga do .trackfw-log; warnings={warnings}"


def test_ml2a_stale_wip_usa_latest_transition_e_limite_configuravel(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    (tmp_path / "trackfw.yaml").write_text(
        "roadmap_dir: docs/roadmaps\nstale_wip_days: 2\n",
        encoding="utf-8",
    )
    _config.reset()

    roadmap = tmp_path / "docs/roadmaps/wip/ROADMAP-boundary.md"
    roadmap.write_text(
        "---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n",
        encoding="utf-8",
    )
    old_mtime = datetime(2026, 6, 1, 8, 0).timestamp()
    os.utime(roadmap, (old_mtime, old_mtime))
    (tmp_path / "docs/roadmaps/.trackfw-log").write_text(
        "2026-07-01 10:00  ROADMAP-boundary.md                               backlog → wip\n"
        "2026-07-20 10:00  ROADMAP-boundary.md                               wip → blocked\n"
        "2026-07-26 10:01  ROADMAP-boundary.md                               blocked → wip\n",
        encoding="utf-8",
    )

    cfg = _config.load()
    before_boundary = v.validate_stale_wip(
        cfg,
        now=datetime(2026, 7, 28, 10, 0, 59).timestamp(),
    )
    at_boundary = v.validate_stale_wip(
        cfg,
        now=datetime(2026, 7, 28, 10, 1).timestamp(),
    )

    assert before_boundary == []
    assert any(
        "ROADMAP-boundary.md" in item["message"] for item in at_boundary
    ), f"esperava warning exatamente no limite configurado; warnings={at_boundary}"


def test_ml2a_stale_wip_fallback_mtime_quando_log_ausente(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    (tmp_path / "trackfw.yaml").write_text(
        "roadmap_dir: docs/roadmaps\nstale_wip_days: 3\n",
        encoding="utf-8",
    )
    _config.reset()

    roadmap = tmp_path / "docs/roadmaps/wip/ROADMAP-fallback.md"
    roadmap.write_text(
        "---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n",
        encoding="utf-8",
    )
    old_mtime = datetime(2026, 7, 20, 9, 0).timestamp()
    os.utime(roadmap, (old_mtime, old_mtime))

    warnings = v.validate_stale_wip(
        _config.load(),
        now=datetime(2026, 7, 24, 9, 0).timestamp(),
    )

    assert any(
        "ROADMAP-fallback.md" in item["message"] for item in warnings
    ), f"esperava fallback para mtime quando .trackfw-log está ausente; warnings={warnings}"


def test_ml2b_stale_wip_diagnostica_erro_de_walk(tmp_path, monkeypatch):
    _ml1a_base(tmp_path, monkeypatch)
    shutil.rmtree(tmp_path / "docs/roadmaps/wip")
    (tmp_path / "docs/roadmaps/wip").write_text("not a directory\n", encoding="utf-8")

    warnings = v.validate_stale_wip(_config.load(), days=7)
    messages = [item["message"] for item in warnings]

    assert any(
        "wip" in message for message in messages
    ), f"esperava diagnostico para erro de walk/ENOTDIR em wip/; warnings={warnings}"


# ---------------------------------------------------------------------------
# ML-2A — REQ-2026-07-27-convergencia-templates-python (reativado)
# Após convergência dos templates Python, as regras detectam artefatos canônicos.
# Os testes chamam os geradores diretamente — se o template regredir, o teste falha.
# ---------------------------------------------------------------------------


def test_adr_draft_validador_detecta_formato_canonico():
    """ML-2A (reativado): ADR gerado pelo CLI Python agora emite
    '> Date: … | Status: Draft' no header.
    _adr_is_draft() faz Contains('Status: Draft') → True →
    blocked_by_draft_adr dispara.

    Fixture: ADR criado pelo generate_adr() (formato canônico após ML-2A) +
             REQ canônica que o referencia.
    Se o template Python regredir, o ADR não terá 'Status: Draft' inline
    e o teste falhará.
    """
    import tempfile
    import shutil
    from trackfw.validator import validate_reqs_not_blocked_by_draft_adrs, _adr_is_draft
    from trackfw.generators.adr import generate_adr

    tmp = tempfile.mkdtemp()
    try:
        req_dir = os.path.join(tmp, "docs", "req")
        adr_dir = os.path.join(tmp, "docs", "adr")
        os.makedirs(req_dir)
        os.makedirs(adr_dir)

        # ADR criado pelo gerador Python (formato canônico após ML-2A).
        # Se o template regredir para o formato antigo, o teste quebra aqui.
        adr_filepath = generate_adr(
            title="auth strategy",
            status="Draft",
            adr_dirs=[adr_dir],
            cwd=tmp,
        )
        adr_basename = os.path.basename(adr_filepath)

        # REQ canônica que referencia o ADR na seção ## Blocked by ADRs.
        req_canonical_content = f"""\
# REQ: Login

> Date: 2026-07-27 | Status: Open

## Motivation

## Acceptance Criteria

- [ ] criterio

## Linked ADR
ADR:

## Blocked by ADRs
- {adr_basename} (Draft)

## Linked Roadmap
Roadmap:
"""
        _write(os.path.join(req_dir, "REQ-2026-07-27-login.md"), req_canonical_content)

        cfg = {
            "req_dir": req_dir,
            "adr_dirs": [adr_dir],
        }

        # Pré-condição: ADR existe e é detectado como Draft
        assert os.path.exists(adr_filepath), f"pré-condição: ADR não encontrado em {adr_filepath}"
        assert _adr_is_draft(adr_basename, cfg), (
            f"pré-condição: _adr_is_draft deve retornar True para ADR canônico. "
            f"Conteúdo: {open(adr_filepath).read()[:200]}"
        )

        violations = validate_reqs_not_blocked_by_draft_adrs(cfg)

        # DEVE disparar violation — ADR canônico tem 'Status: Draft' inline
        assert len(violations) > 0, (
            "regressão: blocked_by_draft_adr não detectou ADR Draft no formato canônico. "
            f"ADR gerado em {adr_filepath}. violations: {violations}"
        )
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def test_req_open_validador_detecta_formato_canonico():
    """ML-2A (reativado): REQ gerada pelo CLI Python agora emite
    '> Date: … | Status: Open' no header.
    validate_reqs_not_blocked_by_draft_adrs() detecta a REQ →
    verifica o ADR bloqueante → dispara violation.

    Fixture: ADR canônico Draft + REQ criada pelo generate_req() +
             seção ## Blocked by ADRs com o ADR referenciado.
    Se o template Python regredir, a REQ não terá 'Status: Open' inline
    e o teste falhará.
    """
    import tempfile
    import shutil
    from trackfw.validator import validate_reqs_not_blocked_by_draft_adrs, _adr_is_draft
    from trackfw.generators.req import generate_req

    tmp = tempfile.mkdtemp()
    try:
        req_dir = os.path.join(tmp, "docs", "req")
        adr_dir = os.path.join(tmp, "docs", "adr")
        os.makedirs(req_dir)
        os.makedirs(adr_dir)

        # ADR no formato canônico Go/Node: tem "> Date: … | Status: Draft"
        adr_canonical_content = """\
# ADR: Auth

> Date: 2026-07-27 | Status: Draft

## Context
context
"""
        adr_path = os.path.join(adr_dir, "ADR-2026-07-27-auth-draft.md")
        _write(adr_path, adr_canonical_content)

        # REQ criada pelo gerador Python (formato canônico após ML-2A).
        # Gera com '> Date: … | Status: Open' — detectado pelo guard inicial.
        # ## Blocked by ADRs vem com '<!-- none -->' e é atualizado abaixo
        # para incluir o ADR bloqueante (andaime de teste).
        req_filepath = generate_req("login", req_dir=req_dir, cwd=tmp)
        with open(req_filepath, encoding="utf-8") as f:
            req_content = f.read()
        # Substitui o placeholder pela referência ao ADR bloqueante
        req_content = req_content.replace(
            "<!-- none -->",
            "- ADR-2026-07-27-auth-draft.md (Draft)",
        )
        with open(req_filepath, "w", encoding="utf-8") as f:
            f.write(req_content)

        cfg = {
            "req_dir": req_dir,
            "adr_dirs": [adr_dir],
        }

        # Pré-condição: ADR canônico deve ser detectado como Draft
        adr_is_draft = _adr_is_draft("ADR-2026-07-27-auth-draft.md", cfg)
        assert adr_is_draft, (
            "pré-condição falhou: _adr_is_draft deve retornar True para ADR canônico com 'Status: Draft'"
        )

        violations = validate_reqs_not_blocked_by_draft_adrs(cfg)

        # DEVE disparar violation — REQ canônica tem 'Status: Open' inline
        assert len(violations) > 0, (
            "regressão: blocked_by_draft_adr não detectou REQ Open no formato canônico. "
            "REQ gerada em formato canônico deve ter '> Date: … | Status: Open' detectado. "
            f"violations: {violations}"
        )
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


# ---------------------------------------------------------------------------
# ML-1C — adr_accepted_when_req_done + helper canônico _adr_not_accepted
# REQ-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida
# ---------------------------------------------------------------------------


class TestAdrNotAcceptedHelper(unittest.TestCase):
    """Cobre o helper canônico _adr_not_accepted: Draft ou Proposed -> True;
    qualquer outro status (Accepted, Superseded, Deprecated, Rejected) -> False,
    por exclusão (sem allowlist fechada)."""

    def test_draft_e_proposed_sao_nao_aceitos(self):
        from trackfw.validator import _adr_not_accepted

        self.assertTrue(_adr_not_accepted("> Date: 2026-08-01 | Status: Draft\n"))
        self.assertTrue(_adr_not_accepted("> Date: 2026-08-01 | Status: Proposed\n"))

    def test_aceito_por_exclusao(self):
        from trackfw.validator import _adr_not_accepted

        for status in ("Accepted", "Superseded", "Deprecated", "Rejected"):
            content = f"> Date: 2026-08-01 | Status: {status}\n"
            self.assertFalse(
                _adr_not_accepted(content),
                f"status {status} deveria contar como aceito (definição por exclusão)",
            )

    def test_frontmatter_tem_precedencia_sobre_cabecalho(self):
        """Frontmatter status: é a fonte estruturada; se presente, prevalece sobre
        a linha de cabeçalho (que pode estar dessincronizada em edição manual)."""
        from trackfw.validator import _adr_not_accepted

        content = (
            "---\n"
            "status: Proposed\n"
            "date: 2026-08-01\n"
            "---\n\n"
            "> Date: 2026-08-01 | Status: Accepted\n"
        )
        self.assertTrue(_adr_not_accepted(content))

    def test_frontmatter_sem_linha_de_cabecalho(self):
        """ML-1D (2026-08-01) — divergência A da auditoria de paridade: um ADR pode
        ter frontmatter `status:` sem NENHUMA linha de cabeçalho '| Status: X'. É o
        caso que discriminava o Node (que lia só o cabeçalho) do Go e do Python."""
        from trackfw.validator import _adr_not_accepted

        content = "---\nstatus: Proposed\ndate: 2026-08-01\n---\n\n# ADR: sem cabecalho\n\n## Context\nctx\n"
        self.assertTrue(_adr_not_accepted(content))

        accepted_content = "---\nstatus: Accepted\ndate: 2026-08-01\n---\n\n# ADR: sem cabecalho\n\n## Context\nctx\n"
        self.assertFalse(_adr_not_accepted(accepted_content))

    def test_cabecalho_como_fallback_sem_frontmatter(self):
        """ADRs legados (ex.: ADR-001) sem bloco frontmatter continuam detectáveis
        via a linha de cabeçalho '> Date: ... | Status: X'."""
        from trackfw.validator import _adr_not_accepted

        content = "**Status:** Draft\n\n> Date: 2026-08-01 | Status: Draft\n"
        self.assertTrue(_adr_not_accepted(content))

    def test_cabecalho_trunca_no_proximo_pipe(self):
        """ML-1D (2026-08-01) — paridade com Go/Node: o fallback de cabeçalho deve
        truncar o valor no próximo ' |' (ex.: '| Status: Draft | Owner: kg'). Sem
        truncar, _extract_adr_status devolveria 'Draft | Owner: kg', que não bate com
        nem 'draft' nem 'proposed' após lower() -> falso-negativo divergente do Go/Node."""
        from trackfw.validator import _adr_not_accepted, _extract_adr_status

        content = "# ADR: legado\n\n> Date: 2026-08-01 | Status: Draft | Owner: kg\n"
        self.assertEqual(_extract_adr_status(content), "Draft")
        self.assertTrue(_adr_not_accepted(content))

    def test_prosa_com_status_draft_nao_engana_quando_frontmatter_aceito(self):
        """Regressão do falso-positivo por substring livre: um ADR com frontmatter
        Accepted cuja prosa cita literalmente 'Status: Draft'/'Status: Proposed' não
        deve ser classificado como não aceito (ver Go
        TestAdrStatusIsNotAccepted_FrontmatterPrecedeProse e o teste de anchoring do
        Node)."""
        from trackfw.validator import _adr_not_accepted

        content = (
            "---\n"
            "status: Accepted\n"
            "date: 2026-08-01\n"
            "---\n\n"
            "# ADR: x\n\n"
            "> Date: 2026-08-01 | Status: Accepted\n\n"
            "## Context\n"
            "Este ADR substitui uma proposta anterior que ficou em Status: Draft "
            "por meses, e chegou a ser Status: Proposed antes disso.\n"
        )
        self.assertFalse(_adr_not_accepted(content))


def _write_adr(adr_dir: str, basename: str, status: str) -> str:
    path = os.path.join(adr_dir, basename)
    _write(path, f"""\
---
status: {status}
date: 2026-08-01
---

# ADR: Fixture

> Date: 2026-08-01 | Status: {status}
""")
    return path


def _write_req_done(req_dir: str, basename: str, adr_basename: str, req_status: str = "Done") -> str:
    path = os.path.join(req_dir, basename)
    _write(path, f"""\
---
status: {req_status}
date: 2026-08-01
---

# REQ: Fixture

> Date: 2026-08-01 | Status: {req_status}

## Linked ADR
ADR: docs/adr/{adr_basename}
""")
    return path


def _write_req_done_backtick(req_dir: str, basename: str, adr_basename: str, req_status: str = "Done") -> str:
    """Igual a _write_req_done, mas com o ADR referenciado entre backticks e
    prosa após — o formato real presente em REQs do repositório (ML-1C)."""
    path = os.path.join(req_dir, basename)
    _write(path, f"""\
---
status: {req_status}
date: 2026-08-01
---

# REQ: Fixture

> Date: 2026-08-01 | Status: {req_status}

## Linked ADR
ADR: `docs/adr/{adr_basename}` (P1–P4; esta REQ é derivada deste ADR)
""")
    return path


class TestAdrAcceptedWhenReqDone(unittest.TestCase):
    """validate_adr_accepted_when_req_done: REQ Done referenciando ADR Draft/Proposed
    -> violation citando ambos. Superseded/Deprecated/Rejected e REQ não-Done não disparam."""

    def _cfg(self, req_dir, adr_dir):
        return {"req_dir": req_dir, "adr_dirs": [adr_dir]}

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.req_dir = os.path.join(self.tmp, "docs", "req")
        self.adr_dir = os.path.join(self.tmp, "docs", "adr")
        os.makedirs(self.req_dir)
        os.makedirs(self.adr_dir)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_req_done_adr_proposed_dispara_e_cita_ambos(self):
        from trackfw.validator import validate_adr_accepted_when_req_done

        _write_adr(self.adr_dir, "ADR-x.md", "Proposed")
        _write_req_done(self.req_dir, "REQ-x.md", "ADR-x.md", req_status="Done")

        violations = validate_adr_accepted_when_req_done(self._cfg(self.req_dir, self.adr_dir))

        self.assertEqual(len(violations), 1)
        msg = violations[0]["message"]
        self.assertIn("REQ-x.md", msg)
        self.assertIn("ADR-x.md", msg)

    def test_req_done_adr_draft_dispara(self):
        from trackfw.validator import validate_adr_accepted_when_req_done

        _write_adr(self.adr_dir, "ADR-y.md", "Draft")
        _write_req_done(self.req_dir, "REQ-y.md", "ADR-y.md", req_status="Done")

        violations = validate_adr_accepted_when_req_done(self._cfg(self.req_dir, self.adr_dir))

        self.assertEqual(len(violations), 1)
        self.assertIn("REQ-y.md", violations[0]["message"])
        self.assertIn("ADR-y.md", violations[0]["message"])

    def test_req_done_adr_superseded_nao_dispara(self):
        from trackfw.validator import validate_adr_accepted_when_req_done

        _write_adr(self.adr_dir, "ADR-z.md", "Superseded")
        _write_req_done(self.req_dir, "REQ-z.md", "ADR-z.md", req_status="Done")

        violations = validate_adr_accepted_when_req_done(self._cfg(self.req_dir, self.adr_dir))

        self.assertEqual(violations, [])

    def test_req_open_adr_proposed_nao_dispara_regra_nova(self):
        from trackfw.validator import validate_adr_accepted_when_req_done

        _write_adr(self.adr_dir, "ADR-w.md", "Proposed")
        _write_req_done(self.req_dir, "REQ-w.md", "ADR-w.md", req_status="Open")

        violations = validate_adr_accepted_when_req_done(self._cfg(self.req_dir, self.adr_dir))

        self.assertEqual(violations, [])

    def test_rule_registrada_como_error_no_default(self):
        from trackfw import config as cfg_mod

        cfg_mod.reset()
        defaults = cfg_mod.defaults()
        self.assertEqual(defaults["rules"].get("adr_accepted_when_req_done"), "error")

    def test_req_done_adr_entre_backticks_dispara_teste_discriminante(self):
        """ML-1C — teste discriminante: REQ Done referenciando um ADR Proposed
        através de um campo `ADR:` com o caminho entre backticks (formato real
        presente em REQs do repositório) DEVE disparar a violação. Antes desta
        mudança, normalize_yaml_flat_value não removia o par de backticks, o
        token não terminava em '.md', e _extract_ref_path retornava '' em
        silêncio — nenhuma violação era reportada para esse formato."""
        from trackfw.validator import validate_adr_accepted_when_req_done

        _write_adr(self.adr_dir, "ADR-bt.md", "Proposed")
        _write_req_done_backtick(self.req_dir, "REQ-bt.md", "ADR-bt.md", req_status="Done")

        violations = validate_adr_accepted_when_req_done(self._cfg(self.req_dir, self.adr_dir))

        self.assertEqual(len(violations), 1)
        self.assertIn("REQ-bt.md", violations[0]["message"])
        self.assertIn("ADR-bt.md", violations[0]["message"])


class TestBlockedByDraftAdrDeixaDeSerCegaAProposed(unittest.TestCase):
    """A correção da cegueira do ADR: REQ Open bloqueada por ADR Proposed agora
    dispara blocked_by_draft_adr (regressão de AC2 sem renomear a regra)."""

    def test_req_open_bloqueada_por_adr_proposed_dispara(self):
        from trackfw.validator import validate_reqs_not_blocked_by_draft_adrs

        tmp = tempfile.mkdtemp()
        try:
            req_dir = os.path.join(tmp, "docs", "req")
            adr_dir = os.path.join(tmp, "docs", "adr")
            os.makedirs(req_dir)
            os.makedirs(adr_dir)

            _write_adr(adr_dir, "ADR-proposed.md", "Proposed")
            _write(os.path.join(req_dir, "REQ-blocked.md"), """\
# REQ: Fixture

> Date: 2026-08-01 | Status: Open

## Blocked by ADRs
- ADR-proposed.md (Proposed)
""")

            cfg = {"req_dir": req_dir, "adr_dirs": [adr_dir]}
            violations = validate_reqs_not_blocked_by_draft_adrs(cfg)

            self.assertEqual(len(violations), 1)
            self.assertIn("REQ-blocked.md", violations[0]["message"])
            self.assertIn("ADR-proposed.md", violations[0]["message"])
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
