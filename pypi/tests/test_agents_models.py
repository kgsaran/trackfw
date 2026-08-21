"""Tests for trackfw agents models (ML-2A).

Locks the warning criterion (looks_like_suspect_model_value) and the
resolve_agent_model resolution table against the acceptance criteria from
ROADMAP-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
"""

from __future__ import annotations

import pytest

from trackfw.integrations.renderers import resolve_agent_model, looks_like_suspect_model_value, _rewrite_frontmatter_model_line


# ---------------------------------------------------------------------------
# looks_like_suspect_model_value — warning criterion (ML-2A)
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "value,expected",
    [
        ("4.6-beta", True),                  # has hyphen, not version, not claude- → warn
        ("4.6", False),                       # bare version string → no warn
        ("5", False),                         # bare version string (major-only) → no warn
        ("1.0.2", False),                     # bare version string → no warn
        ("claude-sonnet-4-5-20250929", False), # starts with claude- → no warn
        ("claude-opus-5", False),             # starts with claude- → no warn
        ("gpt-5", True),                      # wrong namespace → warn
        ("latest", True),                     # not version, not claude- → warn
        # ML-5A: control chars are always suspect — _rewrite_frontmatter_model_line
        # rejects them outright, so this function must agree with the write path.
        ("claude-sonnet-4-6\ntools: Bash", True),       # \n → frontmatter key injection
        ("claude-sonnet-4-6\n---\nINJECTED", True),     # \n---\n → body injection (most severe)
    ],
)
def test_looks_like_suspect_model_value(value, expected):
    assert looks_like_suspect_model_value(value) == expected


# ---------------------------------------------------------------------------
# resolve_agent_model — resolution table
# ---------------------------------------------------------------------------


PINNED = {"sonnet": "4.6", "opus": "5"}
EMPTY: dict[str, str] = {}


@pytest.mark.parametrize(
    "tier,representation,target_id,agent_models,want_resolved,want_present",
    [
        # claude target — no pin → tier alias
        ("sonnet", "subagent", "claude", EMPTY, "sonnet", True),
        ("opus", "subagent", "claude", EMPTY, "opus", True),
        # claude target — with pin → composed model ID
        ("sonnet", "subagent", "claude", PINNED, "claude-sonnet-4-6", True),
        ("opus", "subagent", "claude", PINNED, "claude-opus-5", True),
        # claude target — escape hatch (has hyphen, not version) → literal
        ("sonnet", "subagent", "claude", {"sonnet": "4.6-beta"}, "4.6-beta", True),
        # claude target — escape hatch (claude- prefix) → literal, no warn
        ("sonnet", "subagent", "claude", {"sonnet": "claude-sonnet-4-5-20250929"}, "claude-sonnet-4-5-20250929", True),
        # codex (custom-agent-toml) — mapModelCodex, agentModels ignored
        ("sonnet", "custom-agent-toml", "codex", PINNED, "gpt-5.4-mini", True),
        ("opus", "custom-agent-toml", "codex", PINNED, "gpt-5.4", True),
        # cursor (agent-markdown) — mapModelCursor, agentModels ignored
        ("sonnet", "agent-markdown", "cursor", PINNED, "composer-2.5[fast=true]", True),
        ("opus", "agent-markdown", "cursor", PINNED, "claude-opus-5[effort=high]", True),
        # antigravity (agent-directory) — mapModel, agentModels ignored
        ("sonnet", "agent-directory", "antigravity", PINNED, "flash", True),
        ("opus", "agent-directory", "antigravity", PINNED, "pro", True),
        # amazonq (cli-agent-json) — no model field
        ("sonnet", "cli-agent-json", "amazonq", PINNED, None, False),
        ("opus", "cli-agent-json", "amazonq", PINNED, None, False),
        # opencode (opencode-agent) — model deliberately omitted
        ("sonnet", "opencode-agent", "opencode", PINNED, None, False),
        # kiro cli (agent-json) — no model field
        ("sonnet", "agent-json", "kiro", PINNED, None, False),
        # gemini (agent-markdown) — tier alias, agentModels ignored
        ("sonnet", "agent-markdown", "gemini", PINNED, "sonnet", True),
        ("opus", "agent-markdown", "gemini", PINNED, "opus", True),
        # copilot (custom-agent) — tier alias
        ("sonnet", "custom-agent", "copilot", PINNED, "sonnet", True),
        # windsurf (skill) — tier alias
        ("sonnet", "skill", "windsurf", PINNED, "sonnet", True),
        # kiro ide (agent-markdown) — tier alias
        ("sonnet", "agent-markdown", "kiro", PINNED, "sonnet", True),
    ],
)
def test_resolve_agent_model(tier, representation, target_id, agent_models, want_resolved, want_present):
    resolved, present = resolve_agent_model(tier, representation, target_id, agent_models)
    assert present == want_present, f"present mismatch for {tier}/{representation}/{target_id}"
    if want_present:
        assert resolved == want_resolved, f"resolved mismatch for {tier}/{representation}/{target_id}"


# ---------------------------------------------------------------------------
# Drift gate — resolver must match what render() actually writes
# ---------------------------------------------------------------------------


def test_resolve_agent_model_matches_render():
    """ResolveAgentModel must agree with render() on the model field value for
    every (agent, target) combination — this is the drift gate that ensures
    'trackfw agents models' does not lie to the user."""
    from importlib.resources import files
    import json

    from trackfw.integrations.catalog import load_catalog, _surfaces as catalog_surfaces
    from trackfw.integrations.renderers import render, _parts

    catalog = load_catalog()
    agent_models = {"sonnet": "4.6", "opus": "5"}

    for agent in catalog["agents"]:
        asset_path = agent["asset"].removeprefix("assets/")
        source = files("trackfw.integrations").joinpath("assets").joinpath(asset_path).read_text(encoding="utf-8")
        metadata, _ = _parts(source)
        tier = metadata.get("model", "") or "sonnet"

        for target in catalog["targets"]:
            surfaces = catalog_surfaces(target, "agents", {}, False)
            if not surfaces:
                continue
            surface = surfaces[0]
            capability = surface["capabilities"]["agents"]
            representation = capability["representation"]

            rendered = render("agents", target["id"], surface["id"], agent, source, capability, None, agent_models)
            got_resolved, got_present = resolve_agent_model(tier, representation, target["id"], agent_models)
            want_resolved, want_present = _extract_model_from_rendered(rendered, representation)

            assert got_present == want_present, (
                f"agent={agent['id']} target={target['id']} repr={representation}: "
                f"resolve_agent_model.present={got_present} but rendered model present={want_present}"
            )
            if got_present and want_present:
                assert got_resolved == want_resolved, (
                    f"agent={agent['id']} target={target['id']} repr={representation}: "
                    f"resolve_agent_model={got_resolved!r} but rendered model={want_resolved!r}"
                )


def _extract_model_from_rendered(content: str, representation: str) -> tuple[str | None, bool]:
    """Parse the rendered artifact and return (model_value, present).
    present=False when the artifact format omits the model field.
    """
    if representation == "custom-agent-toml":
        for line in content.splitlines():
            line = line.strip()
            if line.startswith("model = "):
                v = line[len("model = "):].strip().strip('"')
                return v, True
        return None, False
    if representation in ("cli-agent-json", "agent-json"):
        return None, False
    if representation == "opencode-agent":
        return None, False
    # YAML frontmatter (agent-directory, subagent, agent-markdown, custom-agent, skill)
    if not content.startswith("---\n"):
        return None, False
    end = content.find("\n---", 4)
    if end < 0:
        return None, False
    frontmatter = content[4:end]
    for line in frontmatter.splitlines():
        if ":" in line:
            k, v = line.split(":", 1)
            if k.strip() == "model":
                return v.strip(), True
    return None, False


# ---------------------------------------------------------------------------
# ML-5A: _rewrite_frontmatter_model_line rejects control characters
# ---------------------------------------------------------------------------

_SOURCE_WITH_MODEL = "---\nname: trackfw-backend\nmodel: sonnet\n---\n\n# Backend\n"


@pytest.mark.parametrize(
    "malicious_value",
    [
        "claude-sonnet-4-6\ntools: Bash",       # \n → injects YAML key into frontmatter
        "claude-sonnet-4-6\n---\nINJECTED",     # \n---\n → closes frontmatter, injects body
    ],
)
def test_rewrite_frontmatter_model_line_rejects_control_chars(malicious_value):
    """_rewrite_frontmatter_model_line must raise ValueError for control chars (ML-5A)."""
    with pytest.raises(ValueError, match="control character"):
        _rewrite_frontmatter_model_line(_SOURCE_WITH_MODEL, malicious_value)


def test_rewrite_frontmatter_model_line_accepts_legitimate_escape_hatch():
    """Legitimate escape-hatch value (dated ID) must continue to work after ML-5A."""
    result = _rewrite_frontmatter_model_line(_SOURCE_WITH_MODEL, "claude-sonnet-4-5-20250929")
    assert "model: claude-sonnet-4-5-20250929" in result

# ---------------------------------------------------------------------------
# ML-5C: Unicode line/paragraph separators rejected (U+2028, U+2029)
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "value,expected",
    [
        # U+2028 LINE SEPARATOR — yaml.v3 preserves verbatim; line-based
        # frontmatter parsers treat it as a line terminator (ML-5C).
        ("claude-sonnet-4-6 tools: Bash", True),
        # U+2029 PARAGRAPH SEPARATOR — same class, same argument.
        ("claude-sonnet-4-6 tools: Bash", True),
        # Accented claude- value: U+00E9 is not a line separator →
        # LooksLikeSuspectModelValue must return False (no warn).
        ("claude-sonnet-4-6-café", False),
    ],
)
def test_looks_like_suspect_model_value_unicode_separators(value, expected):
    """looks_like_suspect_model_value extends to U+2028/U+2029 (ML-5C)."""
    assert looks_like_suspect_model_value(value) == expected


@pytest.mark.parametrize(
    "separator_value",
    [
        "claude-sonnet-4-6 tools: Bash",  # U+2028 LINE SEPARATOR
        "claude-sonnet-4-6 tools: Bash",  # U+2029 PARAGRAPH SEPARATOR
    ],
)
def test_rewrite_frontmatter_model_line_rejects_unicode_separators(separator_value):
    """_rewrite_frontmatter_model_line must reject U+2028/U+2029 (ML-5C)."""
    with pytest.raises(ValueError, match="control character"):
        _rewrite_frontmatter_model_line(_SOURCE_WITH_MODEL, separator_value)


def test_rewrite_frontmatter_model_line_accepts_accented_value():
    """Accented claude- value must not be rejected by the unicode-separator check (ML-5C)."""
    legitimate = "claude-sonnet-4-6-café"
    result = _rewrite_frontmatter_model_line(_SOURCE_WITH_MODEL, legitimate)
    assert f"model: {legitimate}" in result
