"""Comando ``trackfw init`` para o pacote Python."""

import os
import sys

from trackfw import identity
from trackfw.generators.init_gen import scaffold
from trackfw.i18n import t as i18n_t


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
    parser.set_defaults(func=run)
    return parser


def _identity_home() -> str:
    return os.path.expanduser("~")


def _identity_file_exists(home: str) -> bool:
    return os.path.isfile(os.path.join(home, ".trackfw", "identity.json"))


def _resolve_identity_preset(value: str) -> tuple["identity.Config | None", bool]:
    """Translate --identity-preset into a Config to persist.

    "none"/"neutral" mean "do not write anything" — the caller must not
    create ~/.trackfw/identity.json for those values. Any other unknown
    value is always an error, listing the accepted values.
    """
    if value in ("none", "neutral"):
        return None, False
    try:
        cfg = identity.preset(value)
    except identity.IdentityError as error:
        valid = ["none", "neutral"] + identity.preset_names()
        raise identity.IdentityError(
            f"identity-preset invalido {value!r} (validos: {', '.join(valid)})"
        ) from error
    return cfg, True


def _prompt_choice(title: str, choices: list[tuple[str, str]]) -> str:
    """Simple TTY prompt for a single choice: prints [n] label, reads a
    number. Only called when sys.stdin.isatty() — never blocks CI."""
    print(title, file=sys.stderr)
    for index, (_, label) in enumerate(choices, 1):
        print(f"  [{index}] {label}", file=sys.stderr)
    raw = input("> ").strip()
    if not raw:
        return ""
    try:
        idx = int(raw) - 1
        if idx < 0:
            raise IndexError
        return choices[idx][0]
    except (ValueError, IndexError):
        return ""


_IDENTITY_PRESET_LABELS: list[tuple[str, str]] = [
    ("greek", "Panteão grego (Zeus, Apolo, Afrodite...)"),
    ("norse", "Mitologia nórdica (Odin, Thor, Freya...)"),
    ("pioneers", "Pioneiros da computação (Turing, Codd, Knuth...)"),
    ("potter", "Harry Potter (Dumbledore, Snape, Luna...)"),
    ("thrones", "Game of Thrones (Tyrion, Jon, Arya...)"),
    ("tolkien", "Senhor dos Anéis (Gandalf, Aragorn, Arwen...)"),
    ("starwars", "Star Wars (Yoda, Leia, Vader...)"),
    ("chaves", "Chaves (Girafales, Madruga, Chiquinha...)"),
    ("turma", "Turma da Mônica (Franjinha, Cebolinha, Mônica...)"),
    ("egyptian", "Panteão egípcio (Thoth, Ísis, Anúbis...)"),
    ("custom", "Personalizar um a um"),
    ("neutral", "Nomes neutros (padrão)"),
]


def _run_identity_wizard(home: str) -> None:
    """Interactive identity wizard: 12 choices (10 presets + custom +
    neutral), followed by a custom-name-per-agent loop and an optional
    nickname prompt. Only called from a TTY — never blocks CI."""
    title = i18n_t("init.prompt.identityPreset")
    select = _prompt_choice(title, _IDENTITY_PRESET_LABELS)
    if select in ("", "neutral"):
        return

    if select == "custom":
        known_ids = identity.known_agent_ids()
        custom_title = i18n_t("init.prompt.identityCustomName")
        agents: dict[str, identity.AgentIdentity] = {}
        slugs_seen: dict[str, str] = {}
        for agent_id in known_ids:
            while True:
                value = input(f"{custom_title} ({agent_id}): ").strip()
                try:
                    slug = identity.slugify(value)
                except identity.IdentityError as error:
                    print(f"  {error}", file=sys.stderr)
                    continue
                if slug in slugs_seen:
                    print(
                        f"  slug {slug!r} duplicado com o agente {slugs_seen[slug]!r}",
                        file=sys.stderr,
                    )
                    continue
                slugs_seen[slug] = agent_id
                agents[agent_id] = identity.AgentIdentity(display_name=value, slug=slug)
                break
        cfg = identity.Config(agents=agents)
    else:
        cfg = identity.preset(select)

    nickname_title = i18n_t("init.prompt.identityNickname")
    cfg.user_nickname = input(f"{nickname_title}: ").strip()

    identity.validate(cfg, identity.known_agent_ids())
    identity.save(home, cfg)


def run(args):
    agents = _parse_agents(args.agents)
    if args.namespacing == "by_agent" and not agents:
        print("--agents is required with --namespacing by_agent", file=sys.stderr)
        sys.exit(2)
    if args.wip_limit < 1:
        print("--wip-limit must be greater than zero", file=sys.stderr)
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

    if not skip_identity_wizard and sys.stdin.isatty():
        try:
            _run_identity_wizard(home)
        except identity.IdentityError as error:
            print(f"init: identidade invalida: {error}", file=sys.stderr)
            sys.exit(2)

    cwd = os.getcwd()
    opts = {
        "project_name": args.project_name or os.path.basename(cwd),
        "namespacing": args.namespacing,
        "agents": agents,
        "wip_limit": args.wip_limit,
    }
    scaffold(cwd, opts)
    ai_tools = _parse_agents(args.ai_tools)
    if ai_tools:
        from trackfw.integrations.catalog import plan_deployments
        from trackfw.integrations.manager import IntegrationManager

        # Resolve the persisted identity BEFORE building plans — if this is
        # skipped, PlannedArtifact content silently reverts to neutral names
        # on the next install even though ~/.trackfw/identity.json is present.
        try:
            ident = identity.load(home)
        except identity.IdentityError as error:
            print(f"init: identidade invalida: {error}", file=sys.stderr)
            sys.exit(2)

        _, plans = plan_deployments("agents", target_ids=ai_tools, scope="project", identity_cfg=ident)
        IntegrationManager(cwd).install(plans)
        _, plans = plan_deployments("skills", target_ids=ai_tools, scope="project", identity_cfg=ident)
        IntegrationManager(cwd).install(plans)
    return 0
