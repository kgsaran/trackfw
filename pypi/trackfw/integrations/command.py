"""Shared argparse command contract for agents and skills."""

from __future__ import annotations

import argparse
import json
import os
import sys
from typing import Any

from .catalog import plan_deployments
from .manager import IntegrationError, IntegrationManager


def csv_values(raw: str | None) -> list[str] | None:
    if not raw:
        return None
    values = [value.strip() for value in raw.split(",") if value.strip()]
    return list(dict.fromkeys(values)) or None


def surface_values(raw_values: list[str] | None) -> dict[str, str]:
    result: dict[str, str] = {}
    for raw in raw_values or []:
        if "=" not in raw:
            raise ValueError(f"invalid --surface {raw!r}; expected target=surface")
        target, surface = raw.split("=", 1)
        if not target.strip() or not surface.strip():
            raise ValueError(f"invalid --surface {raw!r}; expected target=surface")
        result[target.strip()] = surface.strip()
    return result


def _select(label: str, entries: list[tuple[str, str]]) -> list[str]:
    print(f"Select {label} (comma-separated numbers):", file=sys.stderr)
    for index, (_, name) in enumerate(entries, 1):
        print(f"  [{index}] {name}", file=sys.stderr)
    raw = input("> ").strip()
    selected: list[str] = []
    for token in raw.split(","):
        try:
            index = int(token.strip()) - 1
            selected.append(entries[index][0])
        except (ValueError, IndexError):
            raise ValueError(f"invalid selection {token!r}") from None
    return list(dict.fromkeys(selected))


def add_lifecycle_parser(subparsers, kind: str):
    parser = subparsers.add_parser(kind, help=f"List and manage trackfw {kind}")
    actions = parser.add_subparsers(dest="action", required=True)
    for action in ("list", "install", "uninstall", "update"):
        child = actions.add_parser(action, help=f"{action.title()} trackfw {kind}")
        child.add_argument("--targets", help="Comma-separated target CLIs")
        child.add_argument("--items", help=f"Comma-separated {kind} IDs")
        child.add_argument("--scope", choices=("project", "global"), default="project")
        child.add_argument("--surface", action="append", help="Select target surface as target=surface")
        child.add_argument("--json", action="store_true", help="Print deterministic JSON")
        child.add_argument("--force", action="store_true", help="Replace/remove modified managed files")
        child.set_defaults(func=lambda args, selected_kind=kind: run(args, selected_kind))
    return parser


def run(args: argparse.Namespace, kind: str) -> int:
    try:
        catalog, _ = plan_deployments(kind, scope=args.scope)
        targets = csv_values(args.targets)
        items = csv_values(args.items)
        mutation = args.action != "list"
        if mutation and not targets:
            if sys.stdin.isatty():
                targets = _select("target CLIs", [(entry["id"], entry["name"]) for entry in catalog["targets"]])
            else:
                raise ValueError(f"--targets is required for non-interactive {args.action}")
        if mutation and not items and sys.stdin.isatty():
            items = _select(kind, [(entry["id"], entry["name"]) for entry in catalog[kind]])
        catalog, plans = plan_deployments(
            kind,
            target_ids=targets,
            item_ids=items,
            scope=args.scope,
            surfaces=surface_values(args.surface),
        )
        manager = IntegrationManager(os.getcwd())
        if args.action == "install":
            manager.install(plans, force=args.force)
        elif args.action == "update":
            manager.update(plans, force=args.force)
        elif args.action == "uninstall":
            manager.uninstall(plans, force=args.force)
        deployments = manager.list(plans)
        deployments.sort(key=lambda deployment: (deployment["target"], deployment["surface"], deployment["item"]))
        payload: dict[str, Any] = {
            "kind": kind,
            "catalog_version": catalog["version"],
            "items": [
                {"id": item["id"], "name": item["name"], "description": item["description"]}
                for item in catalog[kind]
            ],
            "deployments": deployments,
        }
        if args.json:
            print(json.dumps(payload, ensure_ascii=False, indent=2))
        else:
            for deployment in deployments:
                print(
                    f"{deployment['target']}/{deployment['surface']} "
                    f"{deployment['scope']} {deployment['item']}: {deployment['state']} "
                    f"({deployment['destination']})"
                )
        return 0
    except (IntegrationError, OSError, ValueError) as error:
        print(f"trackfw {kind} {args.action}: {error}", file=sys.stderr)
        raise SystemExit(2) from error
