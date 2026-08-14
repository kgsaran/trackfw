"""Tests for identity injection in trackfw.integrations (renderers/manager).

Covers ML-4B acceptance criteria: Rota A / Rota B parity, SET_ARCH kept for
architect regardless of custom name, skills never receive identity, and
name-collision detection with force bypass.
"""

from __future__ import annotations

import pytest

from trackfw.identity import AgentIdentity, Config
from trackfw.integrations.manager import IntegrationError, IntegrationManager
from trackfw.integrations.renderers import render, _rewrite_signature_line

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


class TestTomlModelMapping:
    """ML-2C: model tier no branch custom-agent-toml (Codex), espelhando Go/Node.js."""

    ARCHITECT_OPUS_SOURCE = (
        "---\n"
        "name: trackfw-architect\n"
        "description: Principal software architect for system design.\n"
        "model: opus\n"
        "---\n"
        "# Architect\n\nBody text.\n"
    )

    BACKEND_ITEM = {"id": "backend", "description": "Backend specialist."}
    BACKEND_SONNET_SOURCE = (
        "---\n"
        "name: trackfw-backend\n"
        "description: Backend specialist.\n"
        "model: sonnet\n"
        "---\n"
        "# Backend\n\nBody text.\n"
    )

    def test_architect_opus_maps_to_gpt_5_4(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        got = render("agents", "codex", "cli", ITEM, self.ARCHITECT_OPUS_SOURCE, capability, None)
        assert 'model = "gpt-5.4"' in got

    def test_backend_sonnet_maps_to_gpt_5_4_mini(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        got = render("agents", "codex", "cli", self.BACKEND_ITEM, self.BACKEND_SONNET_SOURCE, capability, None)
        assert 'model = "gpt-5.4-mini"' in got

    def test_model_line_omitted_for_unmapped_value(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        item = {"id": "architect", "description": "Principal software architect."}
        source = (
            "---\n"
            "name: trackfw-architect\n"
            "description: Principal software architect.\n"
            "---\n"
            "# Architect\n\nBody text.\n"
        )
        got = render("agents", "codex", "cli", item, source, capability, None)
        assert "model =" not in got

    def test_model_line_positioned_between_description_and_developer_instructions(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        got = render("agents", "codex", "cli", ITEM, self.ARCHITECT_OPUS_SOURCE, capability, None)
        lines = got.splitlines()
        description_idx = next(i for i, line in enumerate(lines) if line.startswith("description ="))
        model_idx = next(i for i, line in enumerate(lines) if line.startswith("model ="))
        instructions_idx = next(i for i, line in enumerate(lines) if line.startswith("developer_instructions ="))
        assert description_idx < model_idx < instructions_idx


class TestRotaBWithIdentity:
    def test_subagent_representation_rewrites_frontmatter_and_body(self):
        capability = {"representation": "subagent", "support_level": "native"}
        got = render("agents", "claude", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "description: Zeus — Principal software architect for system design." in got
        assert "model: sonnet" in got
        assert "You are Zeus. Address the user as Kleber." in got
        assert "# Architect" in got
        assert "Body text." in got

    def test_agent_markdown_representation_also_rewritten(self):
        # gemini/cursor/kiro-ide use "agent-markdown" — also Rota B (default).
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "gemini", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "You are Zeus." in got


class TestRotaAWithIdentity:
    def test_toml_name_uses_underscore(self):
        capability = {"representation": "custom-agent-toml", "support_level": "native"}
        got = render("agents", "codex", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert 'name = "zeus_tf"' in got
        assert "Zeus" in got
        assert "\\u00ea" not in got  # ensure_ascii=False: non-ASCII chars in display names stay literal, not JSON-escaped

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


class TestRenderOpenCodeAgent:
    """Prova que a representação "opencode-agent" reconstrói o frontmatter
    do zero (mesmo estilo do ramo "agent-directory") de um jeito que o
    OpenCode real (1.18.13) aceita: description presente, "mode: subagent"
    sempre fixo, e "model:"/"tools:"/"memory:" AUSENTES — achado #3 da Wave 1
    do roadmap ROADMAP-2026-08-04-compatibilidade-com-opencode: "tools:" é
    chave reservada no schema do OpenCode (recusa TODO o carregamento do
    projeto se receber a lista estilo Claude Code) e "model:" é omitido por
    decisão de produto (deixar o OpenCode resolver pelo default já
    configurado pelo usuário em opencode.json).

    Espelha internal/integrations/render_test.go:TestRenderOpenCodeAgent.
    """

    def test_description_and_mode_present_model_tools_memory_absent(self):
        capability = {"representation": "opencode-agent", "support_level": "native"}
        got = render("agents", "opencode", "cli", ITEM, CLAUDE_SOURCE, capability, None)

        assert got.startswith("---\n")
        assert "description:" in got
        assert "mode: subagent\n" in got
        for forbidden in ("model:", "tools:", "memory:"):
            assert forbidden not in got
        assert "# Architect" in got
        assert "Body text." in got

    def test_identity_is_applied_to_description(self):
        capability = {"representation": "opencode-agent", "support_level": "native"}
        got = render("agents", "opencode", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)

        assert "description: Zeus — Principal software architect for system design." in got
        assert "mode: subagent\n" in got
        for forbidden in ("model:", "tools:", "memory:"):
            assert forbidden not in got


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


# ---------------------------------------------------------------------------
# Testes unitários de _rewrite_signature_line
# ---------------------------------------------------------------------------


class TestRewriteSignatureLine:
    def test_substitui_nome_na_ultima_assinatura(self):
        source = "---\nname: trackfw-architect\n---\n\n# Corpo\n\nAlgum texto.\n\n— Architect, Principal Software Architect\n"
        got = _rewrite_signature_line(source, "Zeus")
        assert "— Zeus, Principal Software Architect" in got
        assert "— Architect, Principal Software Architect" not in got

    def test_sem_assinatura_retorna_source_inalterado(self):
        source = "---\nname: trackfw-architect\n---\n\n# Corpo\n\nSem assinatura.\n"
        got = _rewrite_signature_line(source, "Zeus")
        assert got == source

    def test_assinatura_no_frontmatter_nao_e_tocada(self):
        source = "---\nname: trackfw-architect\ndescription: — Architect, Principal Software Architect\n---\n\n# Corpo sem assinatura.\n"
        got = _rewrite_signature_line(source, "Zeus")
        assert got == source

    def test_multiplas_candidatas_apenas_ultima_reescrita(self):
        source = "---\nname: trackfw-architect\n---\n\n— Architect, Senior Role\n\nTexto.\n\n— Architect, Principal Software Architect\n"
        got = _rewrite_signature_line(source, "Zeus")
        assert "— Zeus, Principal Software Architect" in got
        assert "— Architect, Senior Role" in got
        assert "— Zeus, Senior Role" not in got

    def test_display_name_vazio_retorna_source_inalterado(self):
        source = "---\nname: trackfw-architect\n---\n\n# Corpo\n\n— Architect, Principal Software Architect\n"
        got = _rewrite_signature_line(source, "")
        assert got == source


class TestTargetParameterBaseline:
    """ML-1C (roteamento de model tier): baseline de regressão.

    Python já recebe `target` posicional em `render()` (ver assinatura em
    renderers.py e o call site em catalog.py, linha ~108) — diferente de Go
    e Node.js, que precisaram de ML-1A/1B para adicionar o parâmetro. Este
    teste documenta o comportamento atual (target aceito, mas sem efeito na
    saída) para servir de ponto de comparação antes das Waves 2/3 passarem a
    usar `target` para mapear `model` no output.
    """

    def test_render_accepts_target_positional_for_cursor(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "cursor", "ide", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "You are Zeus." in got

    def test_render_accepts_target_positional_for_gemini(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "gemini", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "You are Zeus." in got

    def test_different_target_values_no_longer_match_after_wave_3(self):
        # Wave 3 (ML-3C) faz `cursor` divergir de `gemini`/`kiro` dentro da
        # mesma representação "agent-markdown": só `cursor` reescreve a linha
        # "model:". CLAUDE_SOURCE declara "model: sonnet" -> mapeado para
        # "composer-2.5[fast=true]" apenas no branch cursor.
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got_cursor = render("agents", "cursor", "ide", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        got_gemini = render("agents", "gemini", "cli", ITEM, CLAUDE_SOURCE, capability, GREEK_CFG)
        assert got_cursor != got_gemini
        assert "model: composer-2.5[fast=true]" in got_cursor
        assert "model: sonnet" in got_gemini


class TestCursorModelMapping:
    """ML-3C: model tier no branch agent-markdown (Cursor), espelhando Go/Node.js.

    Espelha internal/integrations/render_test.go (ML-3A) e o cenário Go
    equivalente para o mapeamento opus/sonnet e a regressão gemini/kiro.
    """

    ARCHITECT_OPUS_SOURCE = (
        "---\n"
        "name: trackfw-architect\n"
        "description: Principal software architect for system design.\n"
        "model: opus\n"
        "---\n"
        "# Architect\n\nBody text.\n"
    )

    BACKEND_ITEM = {"id": "backend", "description": "Backend specialist."}
    BACKEND_SONNET_SOURCE = (
        "---\n"
        "name: trackfw-backend\n"
        "description: Backend specialist.\n"
        "model: sonnet\n"
        "---\n"
        "# Backend\n\nBody text.\n"
    )

    def test_architect_opus_maps_to_claude_opus_5(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "cursor", "ide", ITEM, self.ARCHITECT_OPUS_SOURCE, capability, None)
        assert "model: claude-opus-5[effort=high]" in got

    def test_backend_sonnet_maps_to_composer_2_5(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render(
            "agents", "cursor", "ide", self.BACKEND_ITEM, self.BACKEND_SONNET_SOURCE, capability, None
        )
        assert "model: composer-2.5[fast=true]" in got

    def test_model_line_removed_for_unmapped_value(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        item = {"id": "architect", "description": "Principal software architect."}
        source = (
            "---\n"
            "name: trackfw-architect\n"
            "description: Principal software architect.\n"
            "---\n"
            "# Architect\n\nBody text.\n"
        )
        got = render("agents", "cursor", "ide", item, source, capability, None)
        assert "model:" not in got

    def test_gemini_output_byte_identical_to_pre_wave_3(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "gemini", "cli", ITEM, self.ARCHITECT_OPUS_SOURCE, capability, None)
        assert got == self.ARCHITECT_OPUS_SOURCE.strip() + "\n"
        assert "model: opus" in got
        assert "claude-opus-5" not in got

    def test_kiro_output_byte_identical_to_pre_wave_3(self):
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "kiro", "ide", ITEM, self.ARCHITECT_OPUS_SOURCE, capability, None)
        assert got == self.ARCHITECT_OPUS_SOURCE.strip() + "\n"
        assert "model: opus" in got
        assert "claude-opus-5" not in got

    def test_composes_with_custom_identity(self):
        # target == "cursor" reescreve "model:" ANTES da injeção de identidade
        # (name:/description:/greeting/assinatura) — as duas transformações
        # devem compor sem se pisar.
        capability = {"representation": "agent-markdown", "support_level": "native"}
        got = render("agents", "cursor", "ide", ITEM, self.ARCHITECT_OPUS_SOURCE, capability, GREEK_CFG)
        assert "name: zeus-tf" in got
        assert "description: Zeus — Principal software architect for system design." in got
        assert "model: claude-opus-5[effort=high]" in got
        assert "You are Zeus. Address the user as Kleber." in got


class TestRotaBRewritesSignature:
    """Teste de integração: Rota B reescreve assinatura quando há identidade."""

    SOURCE_WITH_SIG = (
        "---\n"
        "name: trackfw-architect\n"
        "description: Principal software architect.\n"
        "model: opus\n"
        "---\n\n"
        "# Architect\n\n"
        "Corpo do agente.\n\n"
        "— Architect, Principal Software Architect\n"
    )
    ITEM = {"id": "architect", "description": "Principal software architect."}
    CFG = Config(
        user_nickname="chefe",
        agents={"architect": AgentIdentity(display_name="Zeus", slug="zeus")},
    )

    def test_subagent_reescreve_assinatura_com_identidade(self):
        capability = {"representation": "subagent", "support_level": "native"}
        got = render("agents", "claude", "cli", self.ITEM, self.SOURCE_WITH_SIG, capability, self.CFG)
        assert "— Zeus, Principal Software Architect" in got
        assert "— Architect, Principal Software Architect" not in got
        # título preservado
        assert "Principal Software Architect" in got
