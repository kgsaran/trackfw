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


if __name__ == "__main__":
    unittest.main()
