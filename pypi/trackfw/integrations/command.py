"""Shared argparse command contract for agents and skills."""

from __future__ import annotations

import argparse
import json
import os
import sys
from typing import Any, Callable

from trackfw.i18n import t as i18n_t
from trackfw.identity import IdentityError, load as load_identity
from trackfw import config as trackfw_config

from .catalog import plan_deployments
from .manager import IntegrationError, IntegrationManager
from trackfw.generators.init_gen import inject_rules_for_tool

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


SCOPE_PROMPT_TITLE = "Onde instalar os artefatos?"

# Order matters: index 0 is the pre-selected/default option (D2 — `global`
# pre-selected). Shared verbatim with the Go and Node CLIs so the three
# implementations present identical wording (docs/cli-parity.md).
SCOPE_CHOICES: list[tuple[str, str]] = [
    ("global", "Pasta do usuário (~/.claude) — vale para todos os projetos"),
    ("project", "Este projeto (.claude) — apenas neste repositório"),
]


def _prompt_scope() -> str:
    """Real TTY prompt for install scope (D2). Only ever called when
    sys.stdin.isatty() — never blocks a non-interactive run. Empty input
    (bare Enter) accepts the pre-selected default, `global`."""
    print(SCOPE_PROMPT_TITLE, file=sys.stderr)
    for index, (_, label) in enumerate(SCOPE_CHOICES, 1):
        suffix = " (padrão)" if index == 1 else ""
        print(f"  [{index}] {label}{suffix}", file=sys.stderr)
    raw = input("> ").strip()
    if not raw:
        return SCOPE_CHOICES[0][0]
    try:
        index = int(raw) - 1
        if index < 0:
            raise IndexError
        return SCOPE_CHOICES[index][0]
    except (ValueError, IndexError):
        return SCOPE_CHOICES[0][0]


# Indirection so tests can substitute the actual TTY prompt with a spy,
# mirroring identity_wizard.identity_wizard_runner (same module-attribute
# pattern, same reason: callers MUST invoke through this module attribute —
# `command.scope_prompt_runner()` — never via a direct
# `from ... import _prompt_scope`/`scope_prompt_runner`, since the latter
# captures the reference at import time and monkeypatching the module
# attribute afterwards would silently not take effect.
scope_prompt_runner: Callable[[], str] = _prompt_scope


def resolve_scope(scope: str | None, operation: str | None = None) -> str:
    """Resolve the effective install scope for `agents`/`skills`
    install|update|uninstall, and for `trackfw init` (ADR-2026-07-25-
    escopo-de-instalacao-selecionavel-para-agents-e-skills).

    - `scope` is not None (an explicit --scope was parsed, D3): return it
      as-is — argparse's `choices=("project", "global")` already validated
      it, so no further checking is needed here.
    - No TTY and `operation == "uninstall"` (ADR D8): raise instead of
      defaulting. D1's "global" default was approved under the "where to
      install" framing; applying it uniformly to `uninstall` would let a CI
      script that today cleans up `.claude/agents/` in the repo start
      deleting files from the user's home directory instead.
    - No TTY otherwise (D1): default to "global" — the breaking-change
      default this ADR introduces, replacing the old silent "project"
      default.
    - TTY and no explicit scope (D2): ask interactively, `global`
      pre-selected — for every operation, including uninstall, since the
      user sees the choice before anything destructive happens.

    Callers that must NOT prompt (e.g. `list`, a read-only command — D6)
    should not call this function; they should fall back to
    `scope if scope is not None else "global"` directly instead.
    """
    if scope is not None:
        return scope
    if not sys.stdin.isatty():
        if operation == "uninstall":
            raise ValueError("uninstall requires --scope in non-interactive mode")
        return "global"
    return scope_prompt_runner()


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


def _force_help(action: str) -> str:
    """--force help text for a mutation subparser. The three operations grant
    --force different powers, and a single shared string previously
    overstated update/uninstall's reach while never mentioning install's
    ability to adopt unmanaged bytes — that ambiguity is what sent a user
    straight into the "unmanaged artifact ... does not match a trackfw
    template" error on ``update --force`` (see _unmanaged_artifact_error in
    trackfw/integrations/manager.py for the matching remediation). Mirrors
    internal/commands/integrations_flags.go:forceHelp and
    npm/src/commands/integrations.js.
    """
    if action == "install":
        return "Replace a modified managed artifact, or adopt/overwrite an unmanaged file already on disk"
    if action == "uninstall":
        return "Remove a modified managed artifact"
    return "Replace a modified managed artifact; never adopts unmanaged bytes — use 'install --force' for that"


# Larguras de coluna — devem coincidir com Go e Node.js para saída byte-idêntica.
_MODELS_AGENT_WIDTH = 14
_MODELS_TIER_WIDTH = 8
_MODELS_TARGET_WIDTH = 12
_MODELS_NA = "—"


def _run_models(_args: argparse.Namespace) -> int:
    """Implementação do subcomando `trackfw agents models`.

    Lista, por agente e por alvo, o modelo efetivamente resolvido que
    install/update escreveria no campo model: do artefato gerado. Espelha
    internal/commands/agents_models.go:executeAgentModels e
    npm/src/commands/integrations.js:createAgentModelsCommand.
    """
    from .catalog import load_catalog
    from .renderers import resolve_agent_model, looks_like_suspect_model_value
    from .catalog import _surfaces as _catalog_surfaces

    # AC5 + AC11: read agent_models from the global config (~/.trackfw/trackfw.yaml),
    # not from the cwd singleton. Show origin before the table.
    home = os.path.expanduser("~")
    cwd = os.getcwd()
    agent_models, models_source = trackfw_config.load_global_agent_models(home, cwd)
    agent_models = agent_models or {}

    # Source line (AC5): show origin before the table; advisory to stderr when not resolved.
    _models_source_lines = {
        "global": "source: ~/.trackfw/trackfw.yaml",
        "none": "source: não configurado",
        "project_only": "source: trackfw.yaml do projeto (não vale para escopo global)",
        "global_malformed": "source: arquivo global malformado",
    }
    print(_models_source_lines.get(models_source, _models_source_lines["none"]))
    if models_source == "none":
        print(trackfw_config.GLOBAL_AGENT_MODELS_NONE_MESSAGE, file=sys.stderr)
    elif models_source == "project_only":
        print(trackfw_config.GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE, file=sys.stderr)
    elif models_source == "global_malformed":
        print(trackfw_config.MALFORMED_GLOBAL_CONFIG_MESSAGE, file=sys.stderr)
    catalog = load_catalog()

    # Avisos: um por tier suspeito, ordenados, para stderr (antes da tabela).
    suspect_tiers = sorted(tier for tier, v in agent_models.items() if looks_like_suspect_model_value(v))
    for tier in suspect_tiers:
        print(
            f'WARN: agent_models.{tier} = {json.dumps(agent_models[tier])} — not a version string and not a claude- model ID; will be written literally and may produce an invalid model identifier',
            file=sys.stderr,
        )

    aw, tw, gw = _MODELS_AGENT_WIDTH, _MODELS_TIER_WIDTH, _MODELS_TARGET_WIDTH

    def row(a: str, b: str, c: str, d: str) -> str:
        return f"{a:<{aw}} {b:<{tw}} {c:<{gw}} {d}"

    print(row("AGENT", "TIER", "TARGET", "RESOLVED"))

    for agent in catalog["agents"]:
        asset_path = agent["asset"].removeprefix("assets/")
        from importlib.resources import files
        source = files("trackfw.integrations").joinpath("assets").joinpath(asset_path).read_text(encoding="utf-8")
        from .renderers import _parts
        metadata, _ = _parts(source)
        tier = metadata.get("model", "") or "sonnet"

        for target in catalog["targets"]:
            # Selecionar a surface padrão: primeira não-legacy e não-unsupported.
            surfaces = _catalog_surfaces(target, "agents", {}, False)
            if not surfaces:
                continue
            surface = surfaces[0]
            representation = surface["capabilities"]["agents"]["representation"]
            resolved, present = resolve_agent_model(tier, representation, target["id"], agent_models)
            display = resolved if present else _MODELS_NA
            print(row(agent["id"], tier, target["id"], display))

    return 0


def add_lifecycle_parser(subparsers, kind: str):
    parser = subparsers.add_parser(kind, help=f"List and manage trackfw {kind}")
    actions = parser.add_subparsers(dest="action", required=True)
    for action in ("list", "install", "uninstall", "update"):
        child = actions.add_parser(action, help=f"{action.title()} trackfw {kind}")
        child.add_argument("--targets", help="Comma-separated target CLIs")
        child.add_argument("--items", help=f"Comma-separated {kind} IDs")
        # default=None (not "project"/"global") is load-bearing: it is what
        # lets resolve_scope() below distinguish "user did not pass --scope"
        # from an explicit `--scope project`. Comparing against a sentinel
        # string value instead would make an explicit `--scope project`
        # indistinguishable from the default and re-trigger the prompt for
        # users who already chose it (ADR-2026-07-25-escopo-de-instalacao-
        # selecionavel-para-agents-e-skills, "Armadilha crítica").
        child.add_argument(
            "--scope",
            choices=("project", "global"),
            default=None,
            help="installation scope: project or global (default: global; asks interactively)",
        )
        child.add_argument("--surface", action="append", help="Select target surface as target=surface")
        child.add_argument("--json", action="store_true", help="Print deterministic JSON")
        mutation = action != "list"
        if mutation:
            child.add_argument("--force", action="store_true", help=_force_help(action))
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

    # --- third-party (D1) — two-phase quarantine gate, reachable from both
    # `trackfw agents third-party` and `trackfw skills third-party`.
    # Imported lazily: trackfw.commands.thirdparty imports
    # trackfw.integrations.catalog/manager, and this module (command.py)
    # is imported BY trackfw.commands.agents/skills at trackfw.cli
    # module-load time — a top-level import here would risk a load-time
    # cycle the same way trackfw.commands.identity_wizard is imported
    # lazily inside run() above.
    from trackfw.commands.thirdparty import add_thirdparty_parser

    add_thirdparty_parser(actions, kind)

    # "models" é exclusivo de agents: skills não têm campo model, e expor
    # `trackfw skills models` enganaria o usuário. Espelha o gate de kind
    # das flags de identidade no mesmo arquivo e nos CLIs Go e Node.js.
    if kind == "agents":
        models_cmd = actions.add_parser(
            "models",
            help="Show the resolved model each agent uses per target",
        )
        models_cmd.set_defaults(func=lambda args: _run_models(args))

    return parser


def run(args: argparse.Namespace, kind: str) -> int:
    try:
        home = os.path.expanduser("~")
        mutation = args.action != "list"

        # Scope resolution is a gate independent of --targets/--items (ADR
        # D2 — "Nenhum dos CLIs possui prompt de escopo [...] o caso mais
        # comum (trackfw agents install --targets claude) não passa por
        # prompt algum"). It must run before targets/items are resolved and
        # before the catalog is scoped below, and it must NOT prompt for
        # the read-only `list` action (D6) — `list` only adopts the same
        # `global` default so it never reports deployments that diverge
        # from what `install` just wrote.
        if mutation:
            resolved_scope = resolve_scope(args.scope, operation=args.action)
        else:
            resolved_scope = args.scope if args.scope is not None else "global"

        # Identity must be resolved from disk before plan_deployments — skipping
        # this silently reverts custom agent names to the neutral defaults.
        ident = load_identity(home)
        run_agent_models, run_warn_msg = trackfw_config.resolve_agent_models(resolved_scope, home, os.getcwd())
        if run_warn_msg:
            print(run_warn_msg, file=sys.stderr)
        catalog, _ = plan_deployments(kind, scope=resolved_scope, identity_cfg=ident, agent_models=run_agent_models)
        targets = csv_values(args.targets)
        items = csv_values(args.items)

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
            scope=resolved_scope,
            surfaces=selected_surfaces,
            all_surfaces=not mutation,
            identity_cfg=ident,
            # D5/D9 — lets a plain `trackfw agents update` (this is the
            # canonical BuildPlans call site) reproduce any persisted
            # third-party reference block instead of treating a prior
            # `third-party install --apply-to` attachment as drift.
            # Mirrors internal/commands/integrations_flags.go passing
            # manager.ProjectRoot to BuildPlans at both of its call sites.
            project_root=os.getcwd(),
            agent_models=run_agent_models,
        )
        def _on_skip(destination: str, reason: str) -> None:
            print(reason, file=sys.stderr)

        manager = IntegrationManager(os.getcwd(), on_skip=_on_skip)
        # D5 — transparency without an extra confirmation step: print the
        # resolved destinations before writing anything, so the effect of
        # the command is auditable. Only for mutating actions (install/
        # update/uninstall) and only outside --json, which is meant to be
        # machine-parseable output.
        if mutation and not args.json:
            print(f"Destino ({resolved_scope}):")
            for plan in plans:
                print(f"  {plan['destination']}")
        if args.action == "install":
            manager.install(plans, force=args.force)
        elif args.action == "update":
            manager.update(plans, force=args.force)
        elif args.action == "uninstall":
            manager.uninstall(plans, force=args.force)

        # Auxiliary rules files (GEMINI.md, .github/copilot-instructions.md,
        # .windsurfrules, .amazonq/developer/guidelines.md, etc.) are outside
        # the agents/skills catalog managed by IntegrationManager above — they
        # are a separate, tool-specific mechanism (inject_rules_for_tool), and
        # this is the canonical catalog-based install path (`trackfw
        # agents|skills install --targets <tool>`). Mirrors
        # internal/commands/integrations_flags.go:executeIntegrationMutation
        # (ML-5E/ML-5G of ROADMAP-2026-07-29-barrier-governanca-e-autoridade-
        # do-orquestrador). Scoped to "install" only, mirroring the one-shot
        # semantics the removed deprecated CLI aliases had.
        # inject_rules_for_tool no-ops for targets without a rules surface
        # (e.g. antigravity, kiro) and is idempotent for repeated runs.
        if args.action == "install":
            for target_id in targets:
                inject_rules_for_tool(target_id, str(manager.project_root))

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
