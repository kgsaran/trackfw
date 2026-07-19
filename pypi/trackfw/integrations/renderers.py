"""Native renderers for catalog assets."""

from __future__ import annotations

import json
from typing import Any

# ---------------------------------------------------------------------------
# Constantes para renderização no formato agent-directory (Antigravity IDE/CLI)
# ---------------------------------------------------------------------------

# Mapa de modelos canônicos para valores aceitos pelo Antigravity CLI.
# opus→pro, sonnet→flash; flash_lite/flash/pro mantêm-se; demais são omitidos.
_MODEL_MAP: dict[str, str] = {
    "opus": "pro",
    "sonnet": "flash",
    "flash_lite": "flash_lite",
    "flash": "flash",
    "pro": "pro",
}

# SET_IMPL — conjunto base de 10 ferramentas (agentes não-architect)
_SET_IMPL: list[str] = [
    "view_file",
    "list_dir",
    "grep_search",
    "search_web",
    "read_url_content",
    "write_to_file",
    "replace_file_content",
    "run_command",
    "command_status",
    "generate_image",
]

# SET_ARCH — SET_IMPL + 4 ferramentas de orquestração (agente architect)
_SET_ARCH: list[str] = _SET_IMPL + [
    "send_message",
    "define_subagent",
    "invoke_subagent",
    "schedule",
]


def _map_model(model: str) -> str | None:
    """Converte modelo canônico para valor aceito pelo Antigravity CLI.

    Retorna o modelo mapeado ou None se a linha model deve ser omitida.
    """
    return _MODEL_MAP.get(model)


def _agent_tools(name: str) -> list[str]:
    """Retorna SET_ARCH se o nome termina com 'architect', caso contrário SET_IMPL."""
    if name.endswith("architect"):
        return _SET_ARCH
    return _SET_IMPL


# ---------------------------------------------------------------------------
# Parser de frontmatter
# ---------------------------------------------------------------------------


def _parts(source: str) -> tuple[dict[str, str], str]:
    metadata: dict[str, str] = {}
    if not source.startswith("---\n"):
        return metadata, source
    marker = source.find("\n---\n", 4)
    if marker < 0:
        return metadata, source
    for line in source[4:marker].splitlines():
        if ":" in line:
            key, value = line.split(":", 1)
            metadata[key.strip()] = value.strip()
    return metadata, source[marker + 5 :].lstrip()


# ---------------------------------------------------------------------------
# Renderer principal
# ---------------------------------------------------------------------------


def render(
    kind: str,
    target: str,
    surface: str,
    item: dict[str, Any],
    source: str,
    capability: dict[str, str],
) -> str:
    metadata, body = _parts(source)
    description = metadata.get("description", item["description"])
    name = metadata.get("name", f"trackfw-{item['id']}")

    if kind == "agents" and target == "codex":
        return "\n".join(
            [
                f"name = {json.dumps(name.replace('-', '_'))}",
                f"description = {json.dumps(description)}",
                f"developer_instructions = {json.dumps(body.rstrip())}",
                "",
            ]
        )
    if kind == "agents" and (
        target == "amazonq" or (target == "kiro" and surface == "cli") or (target == "antigravity" and surface == "legacy-cli")
    ):
        # Go's encoding/json sorts map keys; keep byte-stable parity with the
        # canonical renderer as well as semantic JSON compatibility.
        payload = {"description": description, "name": name, "prompt": body.rstrip()}
        return json.dumps(payload, indent=2, ensure_ascii=False) + "\n"
    if kind == "agents" and target == "antigravity" and surface == "current":
        # Reconstrói o frontmatter para o Antigravity IDE/CLI (representação agent-directory).
        # O agy rejeita agentes com model: opus|sonnet — mapeamos para pro|flash.
        # Injeta tools: SET_ARCH para architect, SET_IMPL para os demais.
        model = metadata.get("model", "")
        lines = ["---", f"name: {name}", f"description: {description}"]
        mapped = _map_model(model)
        if mapped is not None:
            lines.append(f"model: {mapped}")
        lines.append("tools:")
        for tool in _agent_tools(name):
            lines.append(f"  - {tool}")
        lines.append("---")
        result = "\n".join(lines) + "\n"
        stripped_body = body.rstrip()
        if stripped_body:
            result += stripped_body + "\n"
        return result
    return source.strip() + "\n"
