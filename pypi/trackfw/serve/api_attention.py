"""Attention signal API used by the local dashboard."""

import json
import os


def get_attention(cfg):
    attention_path = os.path.join(
        cfg.get("roadmap_dir", "docs/roadmaps"),
        ".trackfw-attention.json",
    )
    try:
        with open(attention_path, encoding="utf-8") as stream:
            payload = json.load(stream)
    except (OSError, ValueError, TypeError):
        return {"active": False}

    payload["active"] = True
    return payload
