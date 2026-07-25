"""Shared interactive identity wizard — parity port of
``internal/commands/identity_wizard.go``
(ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install, D1).

Consumed by both ``trackfw init`` and ``trackfw agents install`` (and the
``update``/``uninstall`` operations that share the same lifecycle command).
A single implementation — two would drift at the first UX tweak.

Flow (D6): preset/custom selection → free-text names (custom mode only) →
nickname → confirmation screen (D3) → persist. Declining the confirmation
loops back to preset selection without persisting anything at all.
"""

from __future__ import annotations

import os
import sys
from typing import Any, Callable

from trackfw import identity
from trackfw.i18n import t as i18n_t

# Fixed list of themed identity presets offered by the wizard, shared
# verbatim with `trackfw init`'s legacy prompt table so neither consumer can
# drift from the other. Mirrors identityPresetOptions() (Go).
IDENTITY_PRESET_LABELS: list[tuple[str, str]] = [
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


def _catalog_item(catalog: dict[str, Any], kind: str, item_id: str) -> dict[str, Any]:
    """Look up a single catalog item by kind+id.

    Every id in identity.known_agent_ids() is expected to exist in the
    agents catalog (identity's known ids and the embedded catalog are meant
    to be kept in lockstep). If one is missing, that is a programming error
    and this fails loudly instead of silently mislabeling a field — mirrors
    the panic in buildCustomIdentityGroup/confirmIdentitySelection (Go).
    """
    for item in catalog[kind]:
        if item["id"] == item_id:
            return item
    raise AssertionError(
        f"identity wizard: agent id {item_id!r} from identity.known_agent_ids() "
        "has no entry in the agents catalog"
    )


def _prompt_choice(title: str, choices: list[tuple[str, str]]) -> str:
    """Simple TTY prompt for a single choice: prints [n] label, reads a
    number. Only ever called when sys.stdin.isatty() — never blocks CI."""
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


def _prompt_custom_names(catalog: dict[str, Any], known_ids: list[str], helper_text: str) -> list[str]:
    """Prompt one display name per known agent id, labeled with the agent's
    specialty (item.name + item.description from the catalog) — never the
    raw technical id (ADR D4). Rejects empty/duplicate slugs in a loop."""
    values = [""] * len(known_ids)
    slugs_seen: dict[str, str] = {}
    for index, agent_id in enumerate(known_ids):
        item = _catalog_item(catalog, "agents", agent_id)
        label = f"{item['name']} — {item['description']}"
        while True:
            print(label, file=sys.stderr)
            print(f"  {helper_text}", file=sys.stderr)
            value = input("> ").strip()
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
            values[index] = value
            break
    return values


def _build_identity_config(
    identity_select: str,
    known_ids: list[str],
    custom_display_names: list[str],
    user_nickname: str,
) -> identity.Config:
    """Build the in-memory Config for the chosen wizard path, without
    touching disk. Mirrors buildIdentityConfig (Go) so both the wizard and
    any flag-driven caller build the exact same Config shape from the exact
    same inputs."""
    if identity_select == "custom":
        agents: dict[str, identity.AgentIdentity] = {}
        for index, agent_id in enumerate(known_ids):
            slug = identity.slugify(custom_display_names[index])
            agents[agent_id] = identity.AgentIdentity(
                display_name=custom_display_names[index], slug=slug
            )
        cfg = identity.Config(agents=agents)
    else:
        cfg = identity.preset(identity_select)
    cfg.user_nickname = user_nickname
    return cfg


def _confirm_identity_selection(
    catalog: dict[str, Any], known_ids: list[str], cfg: identity.Config
) -> bool:
    """Render the specialty → display name mapping (ADR D3) and ask for
    confirmation. Returns True iff the user confirmed."""
    nickname_label = i18n_t("identity.wizard.nicknameRowLabel")
    label_width = len(nickname_label)
    labels: list[str] = []
    for agent_id in known_ids:
        item = _catalog_item(catalog, "agents", agent_id)
        labels.append(item["description"])
        label_width = max(label_width, len(item["description"]))

    print()
    print(i18n_t("identity.wizard.confirmHeader"))
    for label, agent_id in zip(labels, known_ids):
        agent = cfg.agents.get(agent_id)
        display_name = agent.display_name if agent else ""
        print(f"  {label:<{label_width}}   →  {display_name}")
    if cfg.user_nickname:
        print(f"  {nickname_label:<{label_width}}   {cfg.user_nickname}")
    print()

    question = i18n_t("identity.wizard.confirmQuestion")
    raw = input(f"? {question} (S/n) ").strip().lower()
    return raw in ("", "s", "sim", "y", "yes")


def run_identity_wizard(catalog: dict[str, Any], home: str) -> tuple[identity.Config, bool]:
    """The single interactive identity wizard consumed by both `trackfw
    init` and `trackfw agents install`
    (ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install, D1).

    Shows, in order: preset/custom selection, free-text names (custom mode
    only), the user nickname, and a confirmation screen (D3, D6). If the
    user declines the confirmation, the wizard loops back to preset
    selection without persisting anything — it never returns having written
    a partial or rejected config to disk.

    Returns (cfg, persisted). "neutral" (or an empty/blank selection) is the
    "write nothing" path and returns (empty Config, False).
    """
    known_ids = identity.known_agent_ids()

    title_preset = i18n_t("init.prompt.identityPreset")
    title_custom_name = i18n_t("init.prompt.identityCustomName")
    title_nickname = i18n_t("init.prompt.identityNickname")

    while True:
        identity_select = _prompt_choice(title_preset, IDENTITY_PRESET_LABELS)

        if identity_select in ("", "neutral"):
            return identity.Config(), False

        custom_display_names: list[str] = [""] * len(known_ids)
        if identity_select == "custom":
            custom_display_names = _prompt_custom_names(catalog, known_ids, title_custom_name)

        user_nickname = input(f"{title_nickname}: ").strip()

        cfg = _build_identity_config(identity_select, known_ids, custom_display_names, user_nickname)
        identity.validate(cfg, known_ids)

        if not _confirm_identity_selection(catalog, known_ids, cfg):
            # D3: declining returns to preset selection without persisting
            # anything at all.
            continue

        identity.save(home, cfg)
        return cfg, True


# identity_wizard_runner is the indirection callers use to invoke the
# wizard, mirroring Go's `var identityWizardRunner = runIdentityWizard`
# package-var pattern. Tests that need to prove a caller *would* invoke the
# wizard (without actually blocking on real input()) substitute this module
# attribute instead of calling run_identity_wizard directly. Callers MUST
# call through the module attribute (`identity_wizard.identity_wizard_runner(...)`),
# never via a direct `from ... import identity_wizard_runner` — the latter
# captures the function reference at import time and monkeypatching the
# module attribute afterwards would silently not take effect.
identity_wizard_runner: Callable[[dict[str, Any], str], tuple[identity.Config, bool]] = run_identity_wizard


def identity_file_exists(home: str) -> bool:
    """Report whether <home>/.trackfw/identity.json already exists."""
    return os.path.isfile(os.path.join(home, ".trackfw", "identity.json"))


def resolve_identity_preset(value: str) -> tuple[identity.Config | None, bool]:
    """Translate a --identity-preset flag value into a Config to persist.

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


def apply_identity_preset_flag(preset_value: str, operation: str, home: str) -> None:
    """Resolve and persist --identity-preset for `agents install|update|
    uninstall`, reusing resolve_identity_preset so all callers accept the
    exact same preset names and reject invalid ones with the exact same
    error shape."""
    cfg, should_save = resolve_identity_preset(preset_value)
    if not should_save:
        return
    try:
        identity.validate(cfg, identity.known_agent_ids())
    except identity.IdentityError as error:
        raise identity.IdentityError(f"{operation}: identidade invalida: {error}") from error
    try:
        identity.save(home, cfg)
    except identity.IdentityError as error:
        raise identity.IdentityError(f"{operation}: falha ao gravar identidade: {error}") from error


def should_prompt_identity(kind: str, is_tty: bool, identity_exists: bool, force_flag: bool) -> bool:
    """Decide whether `trackfw agents install` (and the update/uninstall
    operations, which share the same lifecycle command) should show the
    interactive identity wizard. All conditions must hold simultaneously
    (ADR D2):

      1. kind is "agents" — skills have no identity and must never prompt
         (D5).
      2. is_tty — never block a non-interactive run.
      3. either no identity is configured yet, or the user explicitly asked
         to reconfigure it via --identity.

    With an identity already configured and no --identity flag, the wizard
    must NOT reappear — a wizard that shows up on every install becomes an
    incentive to script around it, which quietly kills the feature.
    """
    if kind != "agents":
        return False
    if not is_tty:
        return False
    return (not identity_exists) or force_flag
