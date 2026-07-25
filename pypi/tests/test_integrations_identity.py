"""Tests for identity injection in trackfw.integrations (renderers/manager).

Covers ML-4B acceptance criteria: Rota A / Rota B parity, SET_ARCH kept for
architect regardless of custom name, skills never receive identity, and
name-collision detection with force bypass.
"""

from __future__ import annotations

import pytest

from trackfw.identity import AgentIdentity, Config
from trackfw.integrations.manager import IntegrationError, IntegrationManager
from trackfw.integrations.renderers import render

CLAUDE_SOURCE = (
    "---\n"
    "name: trackfw-architect\n"
    "description: Principal software architect for system design.\n"
    "model: sonnet\n"
    "---\n"
    "# Architect\n\nBody text.\n"
)

ITEM = {"id": "architect", "description": "Principal software architect for system design."}

GREEK_CFG = Config(
    user_nickname="Kleber",
    agents={"architect": AgentIdentity(display_name="Zeus", slug="zeus")},
)


class TestNoIdentityIsByteIdentical:
    def test_default_representation_unchanged_without_identity(self):
        capability = {"representation": "subagent", "support_level": "native"}
        got = render("agents", "claude", "cli", ITEM, CLAUDE_SOURCE, capability, None)
        assert got == CLAUDE_SOURCE.strip() + "\n"

    def test_toml_unchanged_without_identity(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        got = render("agents", "codex", "cli", ITEM, CLAUDE_SOURCE, capability, None)
        assert "trackfw_architect" in got
        assert "Zeus" not in got


class TestRotaBWithIdentity:
    def test_subagent_representation_rewrites_frontmatter_and_body(self):
        capability = {"representation": "subagent", "support_level": "native"}
        got = render("agents", "claude", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "description: Zeus — Principal software architect for system design." in got
        assert "model: sonnet" in got
        assert "Você é Zeus. Trate o usuário como Kleber." in got
        assert "# Architect" in got
        assert "Body text." in got

    def test_agent_markdown_representation_also_rewritten(self):
        # gemini/cursor/kiro-ide use "agent-markdown" — also Rota B (default).
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "gemini", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "Você é Zeus." in got


class TestRotaAWithIdentity:
    def test_toml_name_uses_underscore(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        got = render("agents", "codex", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert 'name = "zeus_tf"' in got
        assert "Zeus" in got
        assert "\\u00ea" not in got  # ensure_ascii=False: "Você" stays literal

    def test_json_representation_uses_slug_name(self):
        capability = {"representation": "cli-agent-json", "support_level": "native"}
        got = render("agents", "amazonq", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        import json

        payload = json.loads(got)
        assert payload["name"] == "zeus-tf"
        assert payload["description"].startswith("Zeus — ")

    def test_agent_directory_uses_slug_name_and_set_arch(self):
        capability = {"representation": "agent-directory", "support_level": "native"}
        got = render("agents", "antigravity", "current", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        # SET_ARCH (14 tools) kept for item id "architect" even with custom name.
        assert "schedule" in got
        assert "invoke_subagent" in got


class TestSetArchByItemIdNotName:
    def test_non_architect_item_id_gets_set_impl_even_with_custom_name(self):
        item = {"id": "backend", "description": "Backend specialist."}
        cfg = Config(agents={"backend": AgentIdentity(display_name="Architect-like", slug="architect-like")})
        capability = {"representation": "agent-directory", "support_level": "native"}
        got = render("agents", "antigravity", "current", item, CLAUDE_SOURCE, capability, cfg)
        assert "schedule" not in got
        assert "send_message" not in got


class TestSkillsNeverGetIdentity:
    def test_skills_kind_short_circuits(self):
        cfg = Config(agents={"architect": AgentIdentity(display_name="Zeus", slug="zeus")})
        capability = {"representation": "skill", "support_level": "native"}
        item = {"id": "architect", "description": "n/a"}
        got = render("skills", "windsurf", "ide", item, CLAUDE_SOURCE, capability, cfg)
        assert got == CLAUDE_SOURCE.strip() + "\n"
        assert "Zeus" not in got


class TestNameCollisionDetection:
    def _plan(self, destination: str, item: str, name: str) -> dict:
        return {
            "destination": destination,
            "claim": {"scope": "project", "target": "claude", "surface": "cli", "kind": "agents", "item": item},
            "content": f"---\nname: {name}\ndescription: x\n---\nbody\n".encode(),
            "catalog_version": "v1",
            "support_level": "native",
        }

    def test_collision_raises_without_force(self, tmp_path):
        project = tmp_path / "project"
        project.mkdir()
        manager = IntegrationManager(project_root=project, home_dir=tmp_path)
        manager.install([self._plan(".claude/agents/a.md", "architect", "zeus-tf")])
        with pytest.raises(IntegrationError):
            manager.install([self._plan(".claude/agents/b.md", "backend", "zeus-tf")])

    def test_collision_bypassed_with_force(self, tmp_path):
        project = tmp_path / "project"
        project.mkdir()
        manager = IntegrationManager(project_root=project, home_dir=tmp_path)
        manager.install([self._plan(".claude/agents/a.md", "architect", "zeus-tf")])
        manager.install([self._plan(".claude/agents/b.md", "backend", "zeus-tf")], force=True)
        assert (project / ".claude" / "agents" / "b.md").is_file()

    def test_no_collision_for_distinct_names(self, tmp_path):
        project = tmp_path / "project"
        project.mkdir()
        manager = IntegrationManager(project_root=project, home_dir=tmp_path)
        manager.install([self._plan(".claude/agents/a.md", "architect", "zeus-tf")])
        manager.install([self._plan(".claude/agents/b.md", "backend", "apolo-tf")])
        assert (project / ".claude" / "agents" / "b.md").is_file()
