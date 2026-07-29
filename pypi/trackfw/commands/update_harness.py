"""
commands/update_harness.py — trackfw update harness (Python CLI).

Refreshes trackfw-managed artifacts already installed in the user's global
harness (``~/.claude``, ``~/.codex``, ``~/.gemini`` and equivalents for the
other catalog targets). Never touches the current project and never
requires ``trackfw.yaml`` or a project cwd — this command runs from
anywhere. Contract: docs/cli-parity.md, section "`trackfw update` vs
`trackfw update harness`".

Declared harness targets (fixed order — this is the order `targets` in the
JSON document follows, not filesystem order):

  1. ``claude-skill``            — legacy global compatibility skill,
                                    ``~/.claude/skills/trackfw/SKILL.md``.
     For every catalog target ``<tool>`` (in catalog.json declaration
     order: claude, codex, gemini, antigravity, cursor, copilot, windsurf,
     amazonq, kiro), two targets follow:
  N. ``<tool>-agents``           — every catalog *agent* item already
                                    deployed for ``<tool>`` at global scope.
  N. ``<tool>-skills``           — every catalog *skill* item already
                                    deployed for ``<tool>`` at global scope.

``<tool>-agents``/``<tool>-skills`` are a roll-up over every catalog item
for that (tool, kind) pair — mirroring the one directory-level example row
in the contract (``codex-agents`` / ``~/.codex/agents``), rather than one
row per catalog item (which `trackfw agents update --targets <tool>`
already reports at that granularity). Roll-up precedence when items
disagree: any item ``failed`` wins over ``updated``, which wins over
``skipped``. A group where every catalog item is not-installed reports
``missing``; a group with at least one installed item never reports
``missing`` for the itself (see the ambiguity note in the ML-6D report —
this precedence is a documented assumption, not part of the pinned
contract).
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from typing import Any

from trackfw.identity import IdentityError, load as load_identity
from trackfw.integrations.catalog import plan_deployments
from trackfw.integrations.manager import IntegrationError, IntegrationManager

STATE_UPDATED = "updated"
STATE_SKIPPED = "skipped"
STATE_MISSING = "missing"
STATE_FAILED = "failed"

_CATALOG_TARGET_ORDER = [
    "claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "kiro",
]
_CATALOG_KIND_ORDER = ["agents", "skills"]

LEGACY_SKILL_RELATIVE = os.path.join(".claude", "skills", "trackfw", "SKILL.md")

LEGACY_SKILL_CONTENT = (
    "---\n"
    "name: trackfw\n"
    'description: "trackfw — Governed Software Delivery: ADR → REQ → ROADMAP → kanban"\n'
    'signature: "\U0001F4E6 trackfw - Governed Delivery"\n'
    "---\n\n"
    "# trackfw — Modo de Operação\n\n"
    "Você está operando com o **trackfw**, um framework de governança de entrega de software.\n"
    "Cadeia: **ADR → REQ → ROADMAP** · Estados: `backlog / wip / blocked / done / abandoned`\n\n"
    "## Comandos principais\n\n"
    "- `trackfw context` — contexto de trabalho atual (sempre execute primeiro)\n"
    "- `trackfw status` — todos os artefatos e estados\n"
    "- `trackfw validate` — valida consistência de governança\n"
    "- `trackfw roadmap move <nome> <estado>` — transição de estado\n"
    "- `trackfw serve` — board Kanban em http://localhost:4080\n\n"
    "## Protocolo de agente\n\n"
    "1. Antes de iniciar: `trackfw context` + ler `docs/agents-working-context.md`\n"
    "2. Após concluir: atualizar `docs/agents-working-context.md`\n"
    "3. Antes de PR: `trackfw validate` deve passar com zero violations\n"
)


def _resolve_destination(raw: str, home: str) -> str:
    """Renders the `~/`-relative destination template plan_deployments
    returns into an absolute path rooted at `home`, mirroring
    IntegrationManager._resolve's global-scope branch (that method is
    private, so this is a read-only, side-effect-free re-derivation used
    only to compute the reported `path` field)."""
    if raw.startswith("~/"):
        return os.path.normpath(os.path.join(home, raw[2:]))
    return os.path.normpath(raw)


def declared_target_ids() -> list[str]:
    ids = ["claude-skill"]
    for tool in _CATALOG_TARGET_ORDER:
        for kind in _CATALOG_KIND_ORDER:
            ids.append(f"{tool}-{kind}")
    return ids


def register(update_actions) -> None:
    parser = update_actions.add_parser(
        "harness",
        help="Update trackfw rules, agents and skills already installed in the user's global harness",
    )
    parser.add_argument("--dry-run", action="store_true", help="Compute and report states without writing anything")
    parser.add_argument("--json", action="store_true", help="Emit the result document instead of the text report")
    parser.add_argument("--targets", help="Comma-separated subset of harness target ids")
    parser.add_argument(
        "--install-missing",
        action="store_true",
        help="Allow missing targets to be installed instead of merely reported",
    )
    parser.set_defaults(func=_run)


def _resolve_targets(raw: str | None) -> list[str]:
    declared = declared_target_ids()
    if not raw:
        return declared
    requested = [value.strip() for value in raw.split(",") if value.strip()]
    unknown = [value for value in requested if value not in declared]
    if unknown:
        print(
            f"trackfw update harness: unknown target id(s): {', '.join(unknown)}",
            )
        raise SystemExit(2)
    # Preserve declared order, not the order the user typed --targets in —
    # consistent with "targets follows the declared target order" (contract).
    selected = set(requested)
    return [target_id for target_id in declared if target_id in selected]


def _legacy_skill_result(home: str, dry_run: bool, install_missing: bool) -> dict[str, Any]:
    path = os.path.join(home, LEGACY_SKILL_RELATIVE)
    desired = LEGACY_SKILL_CONTENT.encode("utf-8")
    try:
        existing = Path(path).read_bytes()
    except FileNotFoundError:
        if not install_missing:
            return {"id": "claude-skill", "state": STATE_MISSING, "path": path}
        if dry_run:
            return {"id": "claude-skill", "state": STATE_UPDATED, "path": path}
        try:
            Path(path).parent.mkdir(parents=True, exist_ok=True)
            Path(path).write_bytes(desired)
        except OSError as error:
            return {"id": "claude-skill", "state": STATE_FAILED, "path": path, "message": str(error)}
        return {"id": "claude-skill", "state": STATE_UPDATED, "path": path}
    if existing == desired:
        return {"id": "claude-skill", "state": STATE_SKIPPED, "path": path}
    if dry_run:
        return {"id": "claude-skill", "state": STATE_UPDATED, "path": path}
    try:
        Path(path).write_bytes(desired)
    except OSError as error:
        return {"id": "claude-skill", "state": STATE_FAILED, "path": path, "message": str(error)}
    return {"id": "claude-skill", "state": STATE_UPDATED, "path": path}


def _catalog_group_result(
    tool: str,
    kind: str,
    home: str,
    manager: IntegrationManager,
    identity_cfg,
    dry_run: bool,
    install_missing: bool,
) -> dict[str, Any]:
    target_id = f"{tool}-{kind}"
    try:
        _, plans = plan_deployments(kind, target_ids=[tool], scope="global", identity_cfg=identity_cfg)
    except ValueError as error:
        return {"id": target_id, "state": STATE_FAILED, "path": "", "message": str(error)}

    if not plans:
        return {"id": target_id, "state": STATE_MISSING, "path": ""}

    directory = os.path.dirname(_resolve_destination(plans[0]["destination"], home))
    inspections = manager.list(plans)

    installed = [(plan, inspection) for plan, inspection in zip(plans, inspections) if inspection["state"] != "not-installed"]
    not_installed = [plan for plan, inspection in zip(plans, inspections) if inspection["state"] == "not-installed"]

    results: list[tuple[str, str | None]] = []

    for plan, inspection in installed:
        pre_state = inspection["state"]
        if pre_state == "current":
            results.append((STATE_SKIPPED, None))
            continue
        if dry_run:
            if pre_state == "modified":
                results.append((STATE_FAILED, f"artifact {plan['destination']} is modified; use force"))
            else:
                results.append((STATE_UPDATED, None))
            continue
        try:
            manager.update([plan])
            results.append((STATE_UPDATED, None))
        except IntegrationError as error:
            results.append((STATE_FAILED, str(error)))

    if install_missing and not_installed:
        for plan in not_installed:
            if dry_run:
                results.append((STATE_UPDATED, None))
                continue
            try:
                manager.install([plan])
                results.append((STATE_UPDATED, None))
            except IntegrationError as error:
                results.append((STATE_FAILED, str(error)))

    if not results:
        # Nothing installed for this (tool, kind) at all, and --install-missing
        # was not requested — "missing never installs" (contract).
        return {"id": target_id, "state": STATE_MISSING, "path": directory}

    states = [state for state, _ in results]
    if STATE_FAILED in states:
        message = next(message for state, message in results if state == STATE_FAILED)
        return {"id": target_id, "state": STATE_FAILED, "path": directory, "message": message}
    if STATE_UPDATED in states:
        return {"id": target_id, "state": STATE_UPDATED, "path": directory}
    return {"id": target_id, "state": STATE_SKIPPED, "path": directory}


def _run(args: argparse.Namespace) -> None:
    home = os.path.expanduser("~")

    try:
        identity_cfg = load_identity(home)
    except IdentityError as error:
        print(f"update harness: identidade invalida: {error}")
        raise SystemExit(2) from error

    target_ids = _resolve_targets(args.targets)
    manager = IntegrationManager(project_root=os.getcwd(), home_dir=home)

    targets: list[dict[str, Any]] = []
    for target_id in target_ids:
        if target_id == "claude-skill":
            targets.append(_legacy_skill_result(home, args.dry_run, args.install_missing))
            continue
        tool, kind = target_id.rsplit("-", 1)
        targets.append(_catalog_group_result(tool, kind, home, manager, identity_cfg, args.dry_run, args.install_missing))

    summary = {
        STATE_UPDATED: sum(1 for target in targets if target["state"] == STATE_UPDATED),
        STATE_SKIPPED: sum(1 for target in targets if target["state"] == STATE_SKIPPED),
        STATE_MISSING: sum(1 for target in targets if target["state"] == STATE_MISSING),
        STATE_FAILED: sum(1 for target in targets if target["state"] == STATE_FAILED),
    }

    payload = {
        "scope": "harness",
        "dry_run": bool(args.dry_run),
        "targets": targets,
        "summary": summary,
    }

    if args.json:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    else:
        print("trackfw update harness — atualizando harness global...\n")
        for target in targets:
            suffix = f" — {target['message']}" if target["state"] == STATE_FAILED and "message" in target else ""
            print(f"  {target['id']:<16} {target['state']:<8} ({target['path']}){suffix}")
        print(
            f"\nupdated={summary[STATE_UPDATED]} skipped={summary[STATE_SKIPPED]} "
            f"missing={summary[STATE_MISSING]} failed={summary[STATE_FAILED]}"
        )

    if summary[STATE_FAILED] > 0:
        raise SystemExit(1)
