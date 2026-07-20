"""
test_validator.py — Testes unitários para pypi/trackfw/validator.py
Espelha a cobertura de npm/src/validator/index.test.js.
Usa tempfile.mkdtemp() para isolamento — sem fixtures compartilhadas.
"""

import os
import time
import unittest
import tempfile
import shutil

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

    def test_validate_ref_targets_accepts_generated_basenames(self):
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
        self.assertEqual(validate_ref_targets_exist(cfg), [])

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


if __name__ == "__main__":
    unittest.main()
