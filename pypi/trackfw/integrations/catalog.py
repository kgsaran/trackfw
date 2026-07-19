"""Load the packaged canonical catalog and produce deterministic deployments."""

from __future__ import annotations

import json
from importlib.resources import files
from typing import Any

from .renderers import render
from .legacy import legacy_hashes


def _asset_root():
    return files("trackfw.integrations").joinpath("assets")


def load_catalog() -> dict[str, Any]:
    with _asset_root().joinpath("catalog.json").open("r", encoding="utf-8") as stream:
        return json.load(stream)


CATALOG_VERSION = load_catalog()["version"]


def _surfaces(
    target: dict[str, Any],
    kind: str,
    requested: dict[str, str],
    all_surfaces: bool,
) -> list[dict[str, Any]]:
    selected = requested.get(target["id"])
    if selected:
        for surface in target["surfaces"]:
            if surface["id"] == selected:
                return [surface]
        raise ValueError(f"unknown surface {target['id']}={selected}")
    compatible = [
        surface
        for surface in target["surfaces"]
        if surface["capabilities"][kind]["support_level"] != "unsupported"
    ]
    if all_surfaces:
        return compatible
    for surface in target["surfaces"]:
        level = surface["capabilities"][kind]["support_level"]
        if level not in {"legacy", "unsupported"}:
            return [surface]
    if compatible:
        return [compatible[0]]
    raise ValueError(f"target {target['id']} has no supported {kind} surface")


def plan_deployments(
    kind: str,
    target_ids: list[str] | None = None,
    item_ids: list[str] | None = None,
    scope: str = "project",
    surfaces: dict[str, str] | None = None,
    all_surfaces: bool = False,
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    if kind not in {"agents", "skills"}:
        raise ValueError(f"unsupported integration kind {kind!r}")
    if scope not in {"project", "global"}:
        raise ValueError(f"unsupported scope {scope!r}")
    catalog = load_catalog()
    selected_targets = set(target_ids or [target["id"] for target in catalog["targets"]])
    selected_items = set(item_ids or [item["id"] for item in catalog[kind]])
    known_targets = {target["id"] for target in catalog["targets"]}
    known_items = {item["id"] for item in catalog[kind]}
    unknown_targets = selected_targets - known_targets
    unknown_items = selected_items - known_items
    if unknown_targets:
        raise ValueError(f"unknown targets: {', '.join(sorted(unknown_targets))}")
    if unknown_items:
        raise ValueError(f"unknown {kind}: {', '.join(sorted(unknown_items))}")

    result: list[dict[str, Any]] = []
    surface_selection = surfaces or {}
    for target in catalog["targets"]:
        if target["id"] not in selected_targets:
            continue
        for surface in _surfaces(target, kind, surface_selection, all_surfaces):
            capability = surface["capabilities"][kind]
            install_paths = [entry for entry in surface["paths"][kind] if entry["scope"] == scope]
            for item in catalog[kind]:
                if item["id"] not in selected_items:
                    continue
                asset_path = item["asset"].removeprefix("assets/")
                content = _asset_root().joinpath(asset_path).read_text(encoding="utf-8")
                for install_path in install_paths:
                    destination = install_path["path"].replace("{{id}}", item["id"])
                    rendered = render(kind, target["id"], surface["id"], item, content, capability)
                    claim = {
                        "target": target["id"],
                        "surface": surface["id"],
                        "scope": scope,
                        "kind": kind,
                        "item": item["id"],
                    }
                    result.append(
                        {
                            "claim": claim,
                            "destination": destination,
                            "content": rendered.encode("utf-8"),
                            "catalog_version": catalog["version"],
                            "support_level": capability["support_level"],
                            "representation": capability["representation"],
                            "legacy_hashes": legacy_hashes(claim),
                        }
                    )
    return catalog, result
