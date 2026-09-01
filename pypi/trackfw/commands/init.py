"""Comando ``trackfw init`` para o pacote Python."""

import os
import sys

from trackfw import identity
from trackfw import config as trackfw_config
from trackfw.commands import identity_wizard
from trackfw.generators.init_gen import scaffold
from trackfw.i18n import t as i18n_t
from trackfw.homedir import home_dir
from trackfw.tty import stdin_is_interactive


def _parse_agents(raw: str) -> list[str]:
    return [agent.strip() for agent in raw.split(",") if agent.strip()]


def register(subparsers):
    parser = subparsers.add_parser(
        "init",
        help="Initialize trackfw governance in the current project",
    )
    parser.add_argument("--project-name", default=None, help="Project name")
    parser.add_argument(
        "--namespacing",
        choices=["flat", "by_agent"],
        default="flat",
        help="Roadmap directory layout",
    )
    parser.add_argument(
        "--agents",
        default="",
        help="Comma-separated agents used with --namespacing by_agent",
    )
    parser.add_argument("--wip-limit", type=int, default=1, help="Maximum active roadmaps")
    parser.add_argument(
        "--ai-tools",
        default="",
        help="Comma-separated AI tools to configure",
    )
    parser.add_argument(
        "--identity-preset",
        default=None,
        help="Agent identity preset: none, neutral, " + ", ".join(identity.preset_names()),
    )
    parser.add_argument(
        "--forge",
        default=None,
        help="Forge platform: github, gitlab, bitbucket, azure",
    )
    parser.set_defaults(func=run)
    return parser


def _identity_home() -> str:
    return home_dir()


def _identity_file_exists(home: str) -> bool:
    return identity_wizard.identity_file_exists(home)


def _resolve_identity_preset(value: str) -> tuple["identity.Config | None", bool]:
    """Translate --identity-preset into a Config to persist.

    "none"/"neutral" mean "do not write anything" — the caller must not
    create ~/.trackfw/identity.json for those values. Any other unknown
    value is always an error, listing the accepted values.
    """
    return identity_wizard.resolve_identity_preset(value)


def run(args):
    agents = _parse_agents(args.agents)
    if args.namespacing == "by_agent" and not agents:
        print("--agents is required with --namespacing by_agent", file=sys.stderr)
        sys.exit(2)
    if args.wip_limit < 1:
        print("--wip-limit must be greater than zero", file=sys.stderr)
        sys.exit(2)

    # --forge: validate unconditionally above the non-interactive branch so
    # that an invalid value fails loudly in CI instead of silently no-op'ing.
    forge_value = args.forge  # None if not provided
    if forge_value is not None:
        from trackfw.forge.resolve import VALID_FORGES, resolve
        if forge_value not in VALID_FORGES:
            valid = ", ".join(VALID_FORGES)
            print(f'forge invalido "{forge_value}": valores aceitos: {valid}', file=sys.stderr)
            sys.exit(2)

    home = _identity_home()
    preset_changed = args.identity_preset is not None

    # Flag validation and persistence happen unconditionally, before the
    # non-interactive/TTY branches below — this is what makes an invalid
    # --identity-preset fail loudly in CI instead of silently no-op'ing.
    if preset_changed:
        try:
            cfg, should_save = _resolve_identity_preset(args.identity_preset)
        except identity.IdentityError as error:
            print(str(error), file=sys.stderr)
            sys.exit(2)
        if should_save:
            try:
                identity.validate(cfg, identity.known_agent_ids())
                identity.save(home, cfg)
            except identity.IdentityError as error:
                print(f"init: identidade invalida: {error}", file=sys.stderr)
                sys.exit(2)

    # Skip the identity wizard entirely when the flag was passed explicitly
    # (already handled above) or when an identity file already exists —
    # re-running init must never silently overwrite a configured identity.
    skip_identity_wizard = preset_changed or _identity_file_exists(home)

    if not skip_identity_wizard and stdin_is_interactive():
        from trackfw.integrations.catalog import load_catalog

        try:
            catalog = load_catalog()
            identity_wizard.identity_wizard_runner(catalog, home)
        except identity.IdentityError as error:
            print(f"init: identidade invalida: {error}", file=sys.stderr)
            sys.exit(2)

    cwd = os.getcwd()
    opts = {
        "project_name": args.project_name or os.path.basename(cwd),
        "namespacing": args.namespacing,
        "agents": agents,
        "wip_limit": args.wip_limit,
        "forge": forge_value or "",
    }
    scaffold(cwd, opts)
    ai_tools = _parse_agents(args.ai_tools)
    if ai_tools:
        from trackfw.integrations.catalog import plan_deployments
        from trackfw.integrations.command import resolve_scope
        from trackfw.integrations.manager import IntegrationManager

        # Resolve the persisted identity BEFORE building plans — if this is
        # skipped, PlannedArtifact content silently reverts to neutral names
        # on the next install even though ~/.trackfw/identity.json is present.
        try:
            ident = identity.load(home)
        except identity.IdentityError as error:
            print(f"init: identidade invalida: {error}", file=sys.stderr)
            sys.exit(2)

        # `init` has no --scope flag (ADR D4): resolve_scope(None) always
        # takes the TTY-prompt-or-"global" path below, reusing the exact
        # same prompt (and wording) as `agents`/`skills` install so the two
        # entry points never drift. Sem TTY -> "global".
        scope = resolve_scope(None)
        print(f"Escopo de instalação: {scope}")

        def _on_skip(destination: str, reason: str) -> None:
            print(reason, file=sys.stderr)

        _am, _am_warn = trackfw_config.resolve_agent_models(scope, home, cwd)
        if _am_warn:
            print(_am_warn, file=sys.stderr)
        _, plans = plan_deployments("agents", target_ids=ai_tools, scope=scope, identity_cfg=ident, agent_models=_am)
        print("Destino:")
        for plan in plans:
            print(f"  {plan['destination']}")
        IntegrationManager(cwd, on_skip=_on_skip).install(plans)
        _, plans = plan_deployments("skills", target_ids=ai_tools, scope=scope, identity_cfg=ident, agent_models=_am)
        print("Destino:")
        for plan in plans:
            print(f"  {plan['destination']}")
        IntegrationManager(cwd, on_skip=_on_skip).install(plans)

        # Auxiliary rules files (GEMINI.md, .github/copilot-instructions.md,
        # .windsurfrules, .amazonq/developer/guidelines.md, etc.) are not part
        # of the agents/skills catalog above — they are a separate,
        # tool-specific mechanism (inject_rules_for_tool). Without this call,
        # selecting a tool here silently installs agents/skills but never
        # creates its governance rules file in a brand-new project (mirrors
        # internal/commands/init.go:installAITools, ML-5E/ML-5G of
        # ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador).
        # inject_rules_for_tool no-ops for targets with no rules surface (e.g.
        # antigravity, kiro) and is idempotent for repeated runs.
        from trackfw.generators.init_gen import inject_rules_for_tool

        for tool in ai_tools:
            inject_rules_for_tool(tool, cwd)
    return 0
