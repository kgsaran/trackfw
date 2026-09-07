"""
Testes unitários para pypi/trackfw/serve/api_chain.py (ML-3D).

ML-3D: reconciliação — cada teste afirma qual conclusão deste ML ele mede.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.serve.api_chain import get_chain


def _make_md(path, content):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


class TestChainCanonicalReqRoadmapLink:
    def test_req_with_body_only_roadmap_ref_produces_edge(self, tmp_path, monkeypatch):
        """
        AFIRMA: o formato canônico gravado por `trackfw req new`
        (frontmatter `roadmap: ""` sempre vazio, valor real em
        "## Linked Roadmap / Roadmap: <path>" no corpo) produz a aresta
        REQ->Roadmap no /api/chain. Antes do ML-3D, get_chain nem sequer
        tentava resolver REQ->Roadmap (a construção de arestas cobria só
        REQ->ADR e ROADMAP->REQ/ADR) — a aresta não existia por
        construção, não por falha de resolução.
        """
        req_dir = tmp_path / "req"
        roadmap_dir = tmp_path / "roadmaps"
        wip_dir = roadmap_dir / "wip"
        _make_md(str(wip_dir / "ROADMAP-canon.md"), "# Roadmap canonico\n")

        roadmap_ref = str(wip_dir / "ROADMAP-canon.md")
        req_content = (
            "---\n"
            "status: Open\n"
            'adr: ""\n'
            'roadmap: ""\n'
            "---\n"
            "# REQ canonica\n\n"
            "## Linked Roadmap\n"
            f"Roadmap: {roadmap_ref}\n"
        )
        _make_md(str(req_dir / "REQ-canon.md"), req_content)

        cfg = {
            "adr_dirs": [str(tmp_path / "adr")],
            "req_dir": str(req_dir),
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }
        monkeypatch.chdir(str(tmp_path))
        result = get_chain(cfg)

        roadmap_node = next((n for n in result["nodes"] if n["type"] == "roadmap"), None)
        assert roadmap_node is not None, f"nó do roadmap não encontrado; nodes={result['nodes']}"

        found = any(e["to"] == roadmap_node["id"] for e in result["edges"])
        assert found, f"aresta REQ->Roadmap não encontrada; edges={result['edges']}"

    def test_unresolvable_roadmap_ref_does_not_invent_node(self, tmp_path, monkeypatch):
        """
        AFIRMA: um vínculo `Roadmap:` cujo basename não existe em estado
        algum não produz aresta para nó nenhum — guarda de vacuidade.
        """
        req_dir = tmp_path / "req"
        roadmap_dir = tmp_path / "roadmaps"
        wip_dir = roadmap_dir / "wip"
        os.makedirs(str(wip_dir), exist_ok=True)

        missing_ref = str(wip_dir / "ROADMAP-nunca-existiu.md")
        req_content = (
            "---\n"
            "status: Open\n"
            'adr: ""\n'
            'roadmap: ""\n'
            "---\n"
            "# REQ orfa\n\n"
            "## Linked Roadmap\n"
            f"Roadmap: {missing_ref}\n"
        )
        _make_md(str(req_dir / "REQ-orfa.md"), req_content)

        cfg = {
            "adr_dirs": [str(tmp_path / "adr")],
            "req_dir": str(req_dir),
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }
        monkeypatch.chdir(str(tmp_path))
        result = get_chain(cfg)

        node_ids = {n["id"] for n in result["nodes"]}
        for e in result["edges"]:
            assert e["to"] in node_ids, f"aresta aponta para nó inventado: {e['to']}"
