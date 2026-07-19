"""Native renderers for catalog assets."""

from __future__ import annotations

import json
from typing import Any


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
                f"name = {json.dumps(name.removeprefix('trackfw-'))}",
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
    return source.strip() + "\n"
