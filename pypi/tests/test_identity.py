"""Tests for trackfw.identity — parity port of internal/identity (Go)."""

from __future__ import annotations

import json
import os
import stat
from pathlib import Path

import pytest

from trackfw.identity import (
    AgentIdentity,
    Config,
    IdentityError,
    agent_name,
    known_agent_ids,
    load,
    lookup,
    preset,
    preset_names,
    save,
    slugify,
    validate,
)

FIXTURE = Path(__file__).parent / "fixtures" / "slug_vectors.json"


def _slug_vectors():
    data = json.loads(FIXTURE.read_text(encoding="utf-8"))
    return data["cases"]


class TestSlugifyVectors:
    @pytest.mark.parametrize("case", _slug_vectors())
    def test_vector(self, case):
        if case.get("error"):
            with pytest.raises(IdentityError):
                slugify(case["input"])
        else:
            assert slugify(case["input"]) == case["expect"]


class TestAgentName:
    def test_suffix(self):
        assert agent_name("zeus") == "zeus-tf"


class TestLoadSave:
    def test_load_missing_file_returns_empty_config(self, tmp_path):
        cfg = load(tmp_path)
        assert cfg.schema_version == 1
        assert cfg.user_nickname == ""
        assert cfg.agents == {}

    def test_load_rejects_unsupported_schema_version(self, tmp_path):
        identity_dir = tmp_path / ".trackfw"
        identity_dir.mkdir()
        (identity_dir / "identity.json").write_text(json.dumps({"schema_version": 2}), encoding="utf-8")
        with pytest.raises(IdentityError):
            load(tmp_path)

    def test_save_then_load_roundtrip(self, tmp_path):
        cfg = Config(
            user_nickname="Kleber",
            agents={"architect": AgentIdentity(display_name="Zeus", slug="zeus")},
        )
        save(tmp_path, cfg)
        loaded = load(tmp_path)
        assert loaded.user_nickname == "Kleber"
        assert loaded.agents["architect"].display_name == "Zeus"
        assert loaded.agents["architect"].slug == "zeus"

    def test_save_is_atomic_and_mode_0600(self, tmp_path):
        cfg = Config(agents={"architect": AgentIdentity(display_name="Zeus", slug="zeus")})
        save(tmp_path, cfg)
        filename = tmp_path / ".trackfw" / "identity.json"
        assert filename.is_file()
        mode = stat.S_IMODE(filename.stat().st_mode)
        assert mode == 0o600

    def test_save_writes_indented_json_with_trailing_newline_no_ascii_escape(self, tmp_path):
        cfg = Config(agents={"qa": AgentIdentity(display_name="Ártemis", slug="artemis")})
        save(tmp_path, cfg)
        content = (tmp_path / ".trackfw" / "identity.json").read_text(encoding="utf-8")
        assert content.endswith("\n")
        assert "\\u00c1" not in content
        assert "Ártemis" in content
        assert content == (
            '{\n'
            '  "schema_version": 1,\n'
            '  "agents": {\n'
            '    "qa": {\n'
            '      "display_name": "Ártemis",\n'
            '      "slug": "artemis"\n'
            '    }\n'
            '  }\n'
            '}\n'
        )

    def test_save_top_level_key_order_and_sorted_agent_keys(self, tmp_path):
        cfg = Config(
            user_nickname="Kg",
            agents={
                "qa": AgentIdentity(display_name="Artemis", slug="artemis"),
                "architect": AgentIdentity(display_name="Zeus", slug="zeus"),
            },
        )
        save(tmp_path, cfg)
        content = (tmp_path / ".trackfw" / "identity.json").read_text(encoding="utf-8")
        data = json.loads(content)
        assert list(data.keys()) == ["schema_version", "user_nickname", "agents"]
        assert list(data["agents"].keys()) == ["architect", "qa"]


class TestValidate:
    def test_unknown_agent_id_errors(self):
        cfg = Config(agents={"nope": AgentIdentity(display_name="X", slug="x")})
        with pytest.raises(IdentityError):
            validate(cfg, known_agent_ids())

    def test_empty_display_name_errors(self):
        cfg = Config(agents={"architect": AgentIdentity(display_name="", slug="zeus")})
        with pytest.raises(IdentityError):
            validate(cfg, known_agent_ids())

    def test_invalid_slug_errors(self):
        cfg = Config(agents={"architect": AgentIdentity(display_name="Zeus", slug="Zeus!")})
        with pytest.raises(IdentityError):
            validate(cfg, known_agent_ids())

    def test_duplicate_slug_errors(self):
        cfg = Config(
            agents={
                "architect": AgentIdentity(display_name="Zeus", slug="zeus"),
                "backend": AgentIdentity(display_name="Zeus2", slug="zeus"),
            }
        )
        with pytest.raises(IdentityError):
            validate(cfg, known_agent_ids())

    def test_slug_with_tf_suffix_errors(self):
        cfg = Config(agents={"architect": AgentIdentity(display_name="Zeus", slug="zeus-tf")})
        with pytest.raises(IdentityError) as exc:
            validate(cfg, known_agent_ids())
        assert "zeus-tf" in str(exc.value)
        assert "architect" in str(exc.value)
        assert '-tf' in str(exc.value)

    def test_slugs_not_ending_in_tf_suffix_pass(self):
        for slug in ("zeus", "tf", "meu-tf-agente"):
            cfg = Config(agents={"architect": AgentIdentity(display_name="Zeus", slug=slug)})
            validate(cfg, known_agent_ids())

    def test_valid_config_passes(self):
        cfg = preset("greek")
        validate(cfg, known_agent_ids())


class TestLookup:
    def test_lookup_present(self):
        cfg = preset("greek")
        found = lookup(cfg, "architect")
        assert found is not None
        assert found.display_name == "Zeus"

    def test_lookup_absent(self):
        assert lookup(Config(), "architect") is None


class TestPresets:
    def test_preset_names_order(self):
        assert preset_names() == [
            "greek",
            "norse",
            "potter",
            "thrones",
            "chaves",
            "pioneers",
            "starwars",
            "tolkien",
            "turma",
            "egyptian",
        ]

    def test_all_presets_cover_all_known_agent_ids(self):
        for name in preset_names():
            cfg = preset(name)
            assert set(cfg.agents.keys()) == set(known_agent_ids())
            validate(cfg, known_agent_ids())

    def test_preset_is_a_copy(self):
        cfg1 = preset("greek")
        cfg1.agents["architect"].display_name = "Mutated"
        cfg2 = preset("greek")
        assert cfg2.agents["architect"].display_name == "Zeus"

    def test_unknown_preset_raises(self):
        with pytest.raises(IdentityError):
            preset("does-not-exist")

    def test_greek_matches_go_values(self):
        cfg = preset("greek")
        expected = {
            "architect": ("Zeus", "zeus"),
            "backend": ("Apolo", "apolo"),
            "frontend": ("Afrodite", "afrodite"),
            "qa": ("Ártemis", "artemis"),
            "infra": ("Ares", "ares"),
            "security": ("Hades", "hades"),
            "dba": ("Poseidon", "poseidon"),
            "ux": ("Atena", "atena"),
            "code-quality": ("Hefesto", "hefesto"),
            "data": ("Métis", "metis"),
        }
        for agent_id, (display_name, slug) in expected.items():
            assert cfg.agents[agent_id].display_name == display_name
            assert cfg.agents[agent_id].slug == slug
