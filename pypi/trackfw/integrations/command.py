"""Shared argparse command contract for agents and skills."""

from __future__ import annotations

import argparse
import json
import os
import sys
from typing import Any

from trackfw.i18n import t as i18n_t
from trackfw.identity import IdentityError, load as load_identity

from .catalog import plan_deployments
from .manager import IntegrationError, IntegrationManager

# trackfw.commands.identity_wizard is imported lazily inside run(), as a
# defensive measure: this module lives in trackfw.integrations, and
# trackfw.commands.init also imports trackfw.integrations.catalog lazily for
# the same reason — keeping the identity_wizard import function-local here
# avoids depending on cross-package import order at module-load time.


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


def _select_one(label: str, entries: list[tuple[str, str]]) -> str:
    print(f"Select {label}:", file=sys.stderr)
    for index, (_, name) in enumerate(entries, 1):
        print(f"  [{index}] {name}", file=sys.stderr)
    raw = input("> ").strip()
    try:
        index = int(raw)
        if index < 1:
            raise IndexError
        return entries[index - 1][0]
    except (ValueError, IndexError):
        raise ValueError(f"invalid selection {raw!r}") from None


def _prompt_ambiguous_surfaces(catalog, kind: str, targets: list[str], selected: dict[str, str]) -> None:
    targets_by_id = {target["id"]: target for target in catalog["targets"]}
    for target_id in targets:
        if target_id in selected:
            continue
        target = targets_by_id[target_id]
        choices = [
            (surface["id"], surface["name"])
            for surface in target["surfaces"]
            if surface["capabilities"][kind]["support_level"] not in {"legacy", "unsupported"}
        ]
        if len(choices) > 1:
            selected[target_id] = _select_one(f"surface for {target['name']}", choices)


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
        mutation = action != "list"
        if mutation:
            child.add_argument("--force", action="store_true", help="Replace/remove modified managed files")
        # Identity flags are agents-only (ADR D5): skills have no identity,
        # and this lifecycle command is shared between `agents` and
        # `skills` — without this kind gate, `trackfw skills install
        # --identity` would silently accept a flag with no effect at all.
        if mutation and kind == "agents":
            from trackfw import identity as _identity

            child.add_argument(
                "--identity",
                action="store_true",
                help="Reconfigure agent identity even if ~/.trackfw/identity.json already exists",
            )
            child.add_argument(
                "--identity-preset",
                default=None,
                help="Agent identity preset (non-interactive): none, neutral, "
                + ", ".join(_identity.preset_names()),
            )
        child.set_defaults(func=lambda args, selected_kind=kind: run(args, selected_kind))
    return parser


def run(args: argparse.Namespace, kind: str) -> int:
    try:
        home = os.path.expanduser("~")
        # Identity must be resolved from disk before plan_deployments — skipping
        # this silently reverts custom agent names to the neutral defaults.
        ident = load_identity(home)
        catalog, _ = plan_deployments(kind, scope=args.scope, identity_cfg=ident)
        targets = csv_values(args.targets)
        items = csv_values(args.items)
        mutation = args.action != "list"

        from trackfw.commands import identity_wizard

        # --identity-preset is validated and persisted unconditionally, above
        # every TTY-dependent branch below — mirrors init's --identity-preset
        # handling so an invalid value always fails loudly instead of
        # silently no-op'ing in a non-interactive CI run.
        preset_changed = kind == "agents" and mutation and getattr(args, "identity_preset", None) is not None
        if preset_changed:
            identity_wizard.apply_identity_preset_flag(args.identity_preset, args.action, home)

        if mutation and not targets:
            if sys.stdin.isatty():
                targets = _select("target CLIs", [(entry["id"], entry["name"]) for entry in catalog["targets"]])
            else:
                raise ValueError(f"--targets is required for non-interactive {args.action}")
        if mutation and not items and sys.stdin.isatty():
            items = _select(kind, [(entry["id"], entry["name"]) for entry in catalog[kind]])
        selected_surfaces = surface_values(args.surface)
        if mutation and sys.stdin.isatty():
            _prompt_ambiguous_surfaces(catalog, kind, targets or [], selected_surfaces)

        # Identity wizard trigger (ADR ADR-2026-07-25-wizard-unificado-de-
        # identidade-no-agents-install, D2): shown only when the flag path
        # above did not already resolve identity for this run, and only for
        # agents (never skills, D5). Runs after target/surface selection and
        # before the final plan_deployments call below so the wizard's
        # freshly-saved identity is what gets rendered into the plans.
        if kind == "agents" and mutation and not preset_changed:
            identity_exists = identity_wizard.identity_file_exists(home)
            force_flag = bool(getattr(args, "identity", False))
            if identity_wizard.should_prompt_identity(kind, sys.stdin.isatty(), identity_exists, force_flag):
                identity_wizard.identity_wizard_runner(catalog, home)
            elif identity_exists and not args.json:
                existing = load_identity(home)
                print(i18n_t("identity.inUse", count=str(len(existing.agents))))

        # Identity must be resolved from disk before the final
        # plan_deployments call — reload here because the wizard or
        # --identity-preset above may have just written a new config; using
        # the stale `ident` loaded at the top of this function would
        # silently revert custom agent names to the neutral defaults.
        ident = load_identity(home)

        catalog, plans = plan_deployments(
            kind,
            target_ids=targets,
            item_ids=items,
            scope=args.scope,
            surfaces=selected_surfaces,
            all_surfaces=not mutation,
            identity_cfg=ident,
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
            print(f"Available {kind} (catalog {catalog['version']}):")
            for item in payload["items"]:
                print(f"  {item['id']:<14} {item['name']} — {item['description']}")
            print("\nDeployments:")
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
