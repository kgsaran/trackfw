"""Load the packaged canonical catalog and produce deterministic deployments."""

from __future__ import annotations

import json
from importlib.resources import files
from typing import Any

from trackfw import identity

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
    # `scope` keeps a default value here purely for signature/syntax
    # reasons: `target_ids` and `item_ids` (both optional) precede it
    # positionally, and Python forbids a non-default parameter after
    # default ones without reordering the signature (which would break
    # every positional call site below). The default is NOT meant to be
    # relied upon: ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-
    # agents-e-skills identified a silent "project" fallback as exactly the
    # bug being fixed. Every production caller (integrations/command.py,
    # commands/init.py, commands/update.py) passes `scope` explicitly —
    # resolved via integrations.command.resolve_scope() where the user's
    # choice matters, or as an explicit "project" literal for the
    # trackfw-own codex sync paths that are intentionally out of this
    # ADR's scope. Do not add a new caller that omits `scope`.
    scope: str = "project",
    surfaces: dict[str, str] | None = None,
    all_surfaces: bool = False,
    identity_cfg: "identity.Config | None" = None,
    # project_root, when truthy, lets plan_deployments apply the D5/D9
    # third-party reference extension point (apply_third_party_references,
    # trackfw.thirdparty.references) to every rendered "agents" artifact.
    # Optional and source-compatible: every existing caller that does not
    # set it gets byte-identical output to before this parameter existed
    # (apply_third_party_references no-ops on a falsy root). Mirrors
    # internal/integrations/plan.go's PlanRequest.ProjectRoot.
    project_root: str | None = None,
    # agent_models maps tier names (e.g. "sonnet", "opus") to version strings
    # (e.g. "4.6", "5") read from trackfw.yaml's agent_models field. A None or
    # empty dict leaves rendered output byte-for-byte identical to before this
    # parameter existed. See ADR-2026-08-21.
    agent_models: "dict[str, str] | None" = None,
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
                    rendered = render(kind, target["id"], surface["id"], item, content, capability, identity_cfg, agent_models)
                    rendered_bytes = rendered.encode("utf-8")
                    # D5/D9 extension point: reproduce any persisted
                    # third-party reference block so regenerating this
                    # exact artifact (e.g. a later `trackfw agents
                    # update`) settles at state "current" instead of
                    # treating the attachment as drift. See
                    # trackfw.thirdparty.references' module docstring for
                    # why this cannot live inside render() itself (it does
                    # not know the project root).
                    if kind == "agents" and project_root:
                        from trackfw.thirdparty.references import apply_third_party_references

                        rendered_bytes = apply_third_party_references(
                            project_root, rendered_bytes, target["id"], item["id"]
                        )
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
                            "content": rendered_bytes,
                            "catalog_version": catalog["version"],
                            "support_level": capability["support_level"],
                            "representation": capability["representation"],
                            "legacy_hashes": legacy_hashes(claim),
                        }
                    )
    return catalog, result


def _truncate_before_id_segment(template: str) -> str:
    """Drops the "{{id}}"-bearing path segment and everything after it,
    returning the shared parent directory — mirrors
    internal/integrations/plan.go:truncateBeforeIDSegment and
    npm/src/integrations/catalog.js:truncateBeforeIdSegment."""
    segments = template.split("/")
    for index, segment in enumerate(segments):
        if "{{id}}" in segment:
            return "/".join(segments[:index])
    return template


def global_group_path(catalog: dict[str, Any], tool_id: str, kind: str) -> str:
    """Returns the tilde-abbreviated directory shared by every catalog item
    of (tool_id, kind) at global scope, derived from the catalog's own path
    template rather than any individual plan's destination — so the
    reported path never depends on catalog item iteration order. See
    docs/cli-parity.md, "Declared harness targets — pinned list"."""
    target = next((entry for entry in catalog["targets"] if entry["id"] == tool_id), None)
    if target is None:
        raise ValueError(f"unknown target {tool_id!r}")
    surfaces = _surfaces(target, kind, {}, False)
    surface = surfaces[0]
    install_path = next((entry for entry in surface["paths"][kind] if entry["scope"] == "global"), None)
    if install_path is None:
        raise ValueError(f"target {tool_id} has no global {kind} path")
    return _truncate_before_id_segment(install_path["path"])
