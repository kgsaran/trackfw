"""
test_validator_ml1a.py — Testes ML-1A (AC9): uma noção de "vinculada" — frontmatter como fonte de
verdade para req_has_roadmap e req list status.

Reconciliação obrigatória (CLAUDE.md): cada teste declara em docstring qual conclusão do ML-1A afirma.
"""

import os
import sys
import unittest
import tempfile
import shutil

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw import validator as v


def _write(path: str, content: str):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _build_project_dir(tmp: str):
    """Cria estrutura mínima de diretórios necessária para validate."""
    for d in [
        "docs/req",
        "docs/roadmaps/wip",
        "docs/roadmaps/done",
        "docs/roadmaps/backlog",
        "docs/adr",
    ]:
        os.makedirs(os.path.join(tmp, d), exist_ok=True)
    _write(os.path.join(tmp, "trackfw.yaml"), "req_dir: docs/req\nroadmap_dir: docs/roadmaps\n")


class TestValidateREQsHaveRoadmapML1A(unittest.TestCase):
    """ML-1A: validate_reqs_have_roadmap usa extractRefPath (frontmatter-first)."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _build_project_dir(self.tmp)
        self.orig_dir = os.getcwd()
        os.chdir(self.tmp)
        _config.reset()

    def tearDown(self):
        os.chdir(self.orig_dir)
        _config.reset()
        shutil.rmtree(self.tmp)

    def test_frontmatter_roadmap_preenchido_passa_req_has_roadmap(self):
        """Afirma: frontmatter `roadmap:` preenchido é suficiente para req_has_roadmap passar.
        Conclusão ML-1A: _extract_ref_path (frontmatter-first) elimina os 21 falsos positivos de órfã."""
        _write(
            os.path.join(self.tmp, "docs/req/REQ-fm-only.md"),
            '---\nstatus: Open\ndate: 2026-09-12\nroadmap: "docs/roadmaps/done/ROADMAP-x.md"\n---\n\n'
            "# REQ: Fixture\n\n> Date: 2026-09-12 | Status: Open\n\n## Linked Roadmap\nRoadmap: <!-- none -->\n",
        )
        cfg = _config.load()
        violations = v.validate_reqs_have_roadmap(cfg)
        has_orphan = any("no linked Roadmap" in (viol.get("message", "") if isinstance(viol, dict) else viol) for viol in violations)
        self.assertFalse(
            has_orphan,
            f"frontmatter roadmap: preenchido NÃO deve disparar req_has_roadmap, obteve: {violations}",
        )

    def test_corpo_roadmap_preenchido_aceito_como_fallback(self):
        """Afirma: corpo `Roadmap:` preenchido (frontmatter vazio) ainda é aceito como fallback.
        Conclusão ML-1A: REQs legadas sem frontmatter continuam funcionando."""
        _write(
            os.path.join(self.tmp, "docs/req/REQ-body-only.md"),
            '---\nstatus: Open\ndate: 2026-09-12\nroadmap: ""\n---\n\n'
            "# REQ: Fixture\n\n> Date: 2026-09-12 | Status: Open\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/done/ROADMAP-x.md\n",
        )
        cfg = _config.load()
        violations = v.validate_reqs_have_roadmap(cfg)
        has_orphan = any("no linked Roadmap" in (viol.get("message", "") if isinstance(viol, dict) else viol) for viol in violations)
        self.assertFalse(
            has_orphan,
            f"corpo Roadmap: preenchido (frontmatter vazio) NÃO deve disparar req_has_roadmap, obteve: {violations}",
        )

    def test_ambos_vazios_dispara_violation(self):
        """Afirma: ausência de ambos os campos produz violation req_has_roadmap.
        Conclusão ML-1A: contra-braço — cheque de órfã não foi removido."""
        _write(
            os.path.join(self.tmp, "docs/req/REQ-neither.md"),
            '---\nstatus: Open\ndate: 2026-09-12\nroadmap: ""\n---\n\n'
            "# REQ: Fixture\n\n> Date: 2026-09-12 | Status: Open\n\n## Linked Roadmap\nRoadmap: <!-- none -->\n",
        )
        cfg = _config.load()
        violations = v.validate_reqs_have_roadmap(cfg)
        has_orphan = any("no linked Roadmap" in (viol.get("message", "") if isinstance(viol, dict) else viol) for viol in violations)
        self.assertTrue(
            has_orphan,
            f"ausência de ambos os campos deve disparar req_has_roadmap violation, obteve: {violations}",
        )


class TestValidateREQRoadmapSyncML1A(unittest.TestCase):
    """ML-1A: validate_req_roadmap_sync detecta basename divergente entre frontmatter e corpo."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _build_project_dir(self.tmp)
        self.orig_dir = os.getcwd()
        os.chdir(self.tmp)
        _config.reset()

    def tearDown(self):
        os.chdir(self.orig_dir)
        _config.reset()
        shutil.rmtree(self.tmp)

    def test_basename_diferente_dispara_warning(self):
        """Afirma: basename diferente entre frontmatter e corpo dispara req_roadmap_sync warning.
        Conclusão ML-1A: warning (não violation) sinaliza divergência sem bloquear usuário."""
        _write(
            os.path.join(self.tmp, "docs/req/REQ-divergent.md"),
            '---\nstatus: Open\ndate: 2026-09-12\nroadmap: "docs/roadmaps/done/ROADMAP-a.md"\n---\n\n'
            "# REQ: Fixture\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/done/ROADMAP-b.md\n",
        )
        cfg = _config.load()
        warnings = v.validate_req_roadmap_sync(cfg)
        has_divergence = any("divergent roadmap" in (w.get("message", "") if isinstance(w, dict) else w) for w in warnings)
        self.assertTrue(
            has_divergence,
            f"basename diferente deve disparar req_roadmap_sync warning, obteve: {warnings}",
        )

    def test_diferenca_estado_mesmo_basename_nao_dispara_warning(self):
        """Afirma: diferença wip vs done com mesmo basename NÃO dispara req_roadmap_sync.
        Conclusão ML-1A: 38 REQs com state-folder diferente (após roadmap move) não geram ruído."""
        _write(
            os.path.join(self.tmp, "docs/req/REQ-state-diff.md"),
            '---\nstatus: Open\ndate: 2026-09-12\nroadmap: "docs/roadmaps/done/ROADMAP-x.md"\n---\n\n'
            "# REQ: Fixture\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ROADMAP-x.md\n",
        )
        cfg = _config.load()
        warnings = v.validate_req_roadmap_sync(cfg)
        has_divergence = any("divergent roadmap" in (w.get("message", "") if isinstance(w, dict) else w) for w in warnings)
        self.assertFalse(
            has_divergence,
            f"state-folder diferente (mesmo basename) NÃO deve disparar req_roadmap_sync, obteve: {warnings}",
        )


class TestParseREQStatusML1A(unittest.TestCase):
    """ML-1A: parse_req_status usa frontmatter como fonte de verdade para req list (issue #306)."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def test_frontmatter_status_e_fonte_de_verdade(self):
        """Afirma: parse_req_status retorna status do frontmatter quando presente.
        Conclusão ML-1A: issue #306 — req list não usa mais apenas o corpo para status."""
        from trackfw.generators.req import parse_req_status

        req_file = os.path.join(self.tmp, "REQ-status-test.md")
        # Frontmatter diz Done, corpo diz WIP — deve retornar Done
        with open(req_file, "w", encoding="utf-8") as f:
            f.write(
                "---\nstatus: Done\ndate: 2026-09-12\nroadmap: \"\"\n---\n\n"
                "# REQ: Test\n\n> Date: 2026-09-12 | Status: WIP\n"
            )
        status = parse_req_status(req_file)
        self.assertEqual(
            status,
            "Done",
            f"parse_req_status deve retornar status do frontmatter (Done) quando presente, obteve: {status!r}",
        )

    def test_body_fallback_quando_frontmatter_ausente(self):
        """Afirma: parse_req_status cai para body quando frontmatter sem status.
        Conclusão ML-1A: REQs legadas sem frontmatter ainda retornam status corretamente."""
        from trackfw.generators.req import parse_req_status

        req_file = os.path.join(self.tmp, "REQ-legacy.md")
        # Sem frontmatter, só linha de cabeçalho
        with open(req_file, "w", encoding="utf-8") as f:
            f.write("# REQ: Legacy\n\n> Date: 2026-09-12 | Status: WIP\n")
        status = parse_req_status(req_file)
        self.assertEqual(
            status,
            "WIP",
            f"parse_req_status deve cair para body quando frontmatter ausente, obteve: {status!r}",
        )

    def test_frontmatter_vazio_usa_body(self):
        """Afirma: frontmatter com status vazio não suprime o fallback do body.
        Conclusão ML-1A: frontmatter vazio != frontmatter preenchido."""
        from trackfw.generators.req import parse_req_status

        req_file = os.path.join(self.tmp, "REQ-empty-fm.md")
        # Frontmatter com status vazio, corpo diz WIP
        with open(req_file, "w", encoding="utf-8") as f:
            f.write(
                '---\nstatus: ""\ndate: 2026-09-12\nroadmap: ""\n---\n\n'
                "# REQ: Fixture\n\n> Date: 2026-09-12 | Status: WIP\n"
            )
        status = parse_req_status(req_file)
        self.assertEqual(
            status,
            "WIP",
            f"frontmatter com status vazio deve cair para body, obteve: {status!r}",
        )


if __name__ == "__main__":
    unittest.main()
