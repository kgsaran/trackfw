"""The ``trackfw <agents|skills> third-party`` two-phase quarantine gate
(D1). Native port of internal/commands/integrations_thirdparty.go.

- Phase 1 — ``fetch <url>``: download, validate, quarantine, emit a review
  artifact. Never installs.
- Phase 2 — ``install --checksum <sha256>``: consume the quarantined
  artifact into its resolved destination(s), requiring a prior
  checksum-linked approval (D8c).
"""

from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import PurePosixPath
from typing import Any, Callable
from urllib.parse import urlparse

from trackfw import identity, thirdparty
from trackfw import config as trackfw_config
from trackfw.integrations.catalog import load_catalog, plan_deployments
from trackfw.integrations.manager import IntegrationError, IntegrationManager
from trackfw.homedir import home_dir
from trackfw.tty import stdin_is_interactive

# thirdPartyProvenanceRule (Go) equivalent — the name of the trackfw
# validate rule that is the real (git-anchored) enforcement behind the
# orchestrator-session guardrail below (Wave 3 of this roadmap, not this
# file). Named as a constant so the guardrail message and any future test
# asserting its wording never drift apart silently.
THIRD_PARTY_PROVENANCE_RULE = "thirdparty_artifact_has_provenance"

# _THIRD_PARTY_GLOBAL_SCOPE_WARNING/_REFUSAL are the D4-bis literal strings
# for --scope global: printed/raised VERBATIM by all 3 CLIs (the roadmap's
# AC requires identical wording). Mirrors
# internal/commands/integrations_thirdparty.go:thirdPartyGlobalScopeWarning/thirdPartyGlobalScopeRefusal
# — keep these two strings byte-identical to the Go constants.
_THIRD_PARTY_GLOBAL_SCOPE_WARNING = (
    "warning: --scope global installs outside the project tree; this artifact will NEVER be verified by "
    f'`trackfw validate` (the "{THIRD_PARTY_PROVENANCE_RULE}" rule only scans the project\'s own manifest — '
    "an artifact under a home directory is invisible to it, per ADR-2026-08-12)."
)
_THIRD_PARTY_GLOBAL_SCOPE_REFUSAL = (
    "install to --scope global requires --yes-global-scope-unverified as its own explicit confirmation "
    "(D4-bis), distinct from --yes-i-trust-this-source: it confirms you understand `trackfw validate` will "
    "never verify this installation"
)

# Indirection so tests can substitute the network fetch instead of hitting
# the real network — mirrors trackfw.integrations.command's
# scope_prompt_runner module-attribute pattern (callers MUST invoke via
# `thirdparty_module.thirdparty_fetch(...)`, never via a captured import,
# so monkeypatching this attribute after import time still takes effect).
thirdparty_fetch: Callable[[str], bytes] = thirdparty.fetch

_SLUG_PATTERN = re.compile(r"^[a-z0-9][a-z0-9._-]{0,63}$")
_CHECKSUM_PATTERN = re.compile(r"^[a-f0-9]{64}$")


def _csv_values(raw: str | None) -> list[str] | None:
    if not raw:
        return None
    values = [value.strip() for value in raw.split(",") if value.strip()]
    return list(dict.fromkeys(values)) or None


def _third_party_entry_kind(kind: str) -> str:
    return "agent" if kind == "agents" else "skill"


def _check_orchestrator_guardrail() -> None:
    """D2: TRACKFW_ORCHESTRATOR_SESSION is a guardrail against accidental
    invocation from a plain terminal, never a security control — it is
    trivially set by anyone with shell access. The real enforcement is the
    THIRD_PARTY_PROVENANCE_RULE check in `trackfw validate` (Wave 3),
    which is git-anchored. This message must never present the env var as
    prevention."""
    if os.environ.get("TRACKFW_ORCHESTRATOR_SESSION"):
        return
    raise IntegrationError(
        "refused: TRACKFW_ORCHESTRATOR_SESSION is not set. This is a guardrail against accidental "
        "invocation from a plain terminal, not a security control — it does not resist anyone who "
        "already has shell access. The real enforcement is the "
        f'"{THIRD_PARTY_PROVENANCE_RULE}" rule checked by `trackfw validate`, which detects any '
        "third-party artifact committed without a matching, checksum-linked provenance entry. "
        "If this is an orchestrated agent session, set TRACKFW_ORCHESTRATOR_SESSION=1"
    )


def _prompt_confirm(title: str) -> bool:
    print(title, file=sys.stderr)
    raw = input("Confirm? [y/N] ").strip().lower()
    return raw in ("y", "yes")


confirm_prompt_runner = _prompt_confirm


def _derive_slug(raw_url: str) -> str:
    """Produces a filesystem-safe slug from a quarantined artifact's
    source URL when --slug is not given. Mirrors Go's deriveSlug."""
    parsed = urlparse(raw_url)
    base = PurePosixPath(parsed.path).name
    dot = base.rfind(".")
    if dot > 0:
        base = base[:dot]
    base = base.lower()
    chars: list[str] = []
    for char in base:
        if ("a" <= char <= "z") or ("0" <= char <= "9"):
            chars.append(char)
        elif char in "-_.":
            chars.append(char)
        else:
            chars.append("-")
    slug = "".join(chars).strip("-._")
    if not slug or not _SLUG_PATTERN.match(slug):
        raise IntegrationError(f"cannot derive a safe slug from URL {raw_url!r}; pass --slug explicitly")
    return slug


def add_thirdparty_parser(actions: argparse._SubParsersAction, kind: str) -> None:
    """Registers the `third-party` subcommand (fetch/install) under the
    kind's ("agents"/"skills") existing action subparsers (D1)."""
    parser = actions.add_parser(
        "third-party",
        help=f"Fetch and install third-party {kind} content under a two-phase quarantine gate",
    )
    sub = parser.add_subparsers(dest="thirdparty_action", required=True)

    fetch_parser = sub.add_parser(
        "fetch", help="Download third-party content into quarantine for review (never installs)"
    )
    fetch_parser.add_argument("url")
    fetch_parser.add_argument(
        "--targets",
        help="Comma-separated target CLIs this artifact is intended for (recorded for review only; "
        "confirmed again at install)",
    )
    fetch_parser.add_argument(
        "--force-thirdparty-markers",
        action="store_true",
        help="override refusal on boundary-redefinition markers (D3); recorded, never silent",
    )
    fetch_parser.set_defaults(func=lambda args, selected_kind=kind: execute_fetch(selected_kind, args))

    install_parser = sub.add_parser(
        "install",
        help="Consume a quarantined artifact into its resolved destination(s), requiring a prior "
        "checksum-linked approval",
    )
    install_parser.add_argument("--checksum", required=True, help="SHA-256 checksum of the quarantined artifact")
    install_parser.add_argument("--slug", default=None, help="destination file slug (default: derived from the quarantined URL)")
    install_parser.add_argument("--targets", required=True, help="target CLIs to install the skill file into")
    install_parser.add_argument(
        "--apply-to",
        default=None,
        help="catalog agent item IDs whose rendered file gets a reference to this artifact (optional; "
        "never inferred silently — AC3)",
    )
    install_parser.add_argument(
        "--scope",
        choices=("project", "global"),
        default=None,
        help="installation scope: project or global (default: project — D4)",
    )
    install_parser.add_argument(
        "--yes-i-trust-this-source",
        action="store_true",
        help="required in non-interactive mode (AC1)",
    )
    install_parser.add_argument(
        "--yes-global-scope-unverified",
        action="store_true",
        help="required, in addition to --yes-i-trust-this-source, for --scope global: confirms this "
        "installation will never be verified by `trackfw validate` (D4-bis)",
    )
    install_parser.set_defaults(func=lambda args, selected_kind=kind: execute_install(selected_kind, args))


# --- Fase 1: fetch ---


def execute_fetch(kind: str, args: argparse.Namespace) -> None:
    _check_orchestrator_guardrail()
    raw = thirdparty_fetch(args.url)
    matched = thirdparty.check_markers(raw)
    if matched and not args.force_thirdparty_markers:
        raise IntegrationError(
            f"refused: content matches boundary-redefinition marker(s) {matched} (D3); pass "
            "--force-thirdparty-markers to quarantine it anyway (recorded in marker_check, never "
            "installed without approval)"
        )

    project_root = os.getcwd()
    targets = _csv_values(args.targets)
    entry = thirdparty.new_quarantine_entry(args.url, raw, matched, _third_party_entry_kind(kind), targets)
    thirdparty.write_quarantine(project_root, entry)

    checksum = entry["checksum_sha256"]
    print(f"quarantined: {thirdparty.quarantine_path(project_root, checksum)}")
    print(f"checksum: {checksum}")
    if matched:
        print(f"warning: marker check failed (matched={matched}); --force-thirdparty-markers was used")
    print(
        "next: obtain a favorable hades-tf review, record its provenance entry keyed by the resolved "
        f"destination(s), then run `{kind} third-party install --checksum {checksum} --targets "
        "<t1,t2> [--apply-to <agent-id,...>]`"
    )


# --- Fase 2: install ---


def _resolve_third_party_scope(args: argparse.Namespace) -> str:
    """Deliberately separate from trackfw.integrations.command.resolve_scope:
    third-party's default is "project" (D4), the opposite of the
    catalog's "global" default. --scope's argparse default=None (not
    "project") is what lets this distinguish "user did not pass --scope"
    from an explicit `--scope project` — argparse's choices=(...) already
    validated the value when present, so no further checking is needed
    here."""
    if args.scope is not None:
        return args.scope
    return "project"


def execute_install(kind: str, args: argparse.Namespace) -> None:
    _check_orchestrator_guardrail()
    checksum = args.checksum
    if not checksum:
        raise IntegrationError("install requires --checksum")
    if not _CHECKSUM_PATTERN.match(checksum):
        raise IntegrationError(f"invalid --checksum {checksum!r}: expected a 64-character lowercase SHA-256 hex digest")

    targets = _csv_values(args.targets)
    if not targets:
        raise IntegrationError("install requires --targets")

    scope = _resolve_third_party_scope(args)
    # D4-bis — print the warning as early as possible, before any other
    # output, so it is visible even if a later step aborts the command.
    if scope == "global":
        print(_THIRD_PARTY_GLOBAL_SCOPE_WARNING)

    project_root = os.getcwd()
    home = home_dir()

    entry = thirdparty.read_quarantine(project_root, checksum)
    content = thirdparty.decode_content(entry)

    # D8c / TOCTOU close: the quarantine record's filename IS its
    # checksum, but that alone does not prove content_base64 hasn't been
    # edited in place since approval. Recompute over the decoded bytes and
    # require both the record's own field and the caller-supplied
    # --checksum to agree with it.
    recomputed = thirdparty.checksum(content)
    if recomputed != checksum or entry.get("checksum_sha256") != checksum:
        raise IntegrationError(
            f"refused: quarantined content for {entry.get('url')!r} no longer matches checksum "
            f"{checksum} (TOCTOU guard, D8c)"
        )

    slug = args.slug
    if not slug:
        slug = _derive_slug(entry["url"])
    elif not _SLUG_PATTERN.match(slug):
        raise IntegrationError(f"invalid --slug {slug!r}: use lowercase alphanumerics, '.', '_' or '-'")

    catalog = load_catalog()

    resolved_targets: list[dict[str, str]] = []
    for target_id in targets:
        destination, surface_id, representation = thirdparty.resolve_third_party_skill_destination(
            catalog, target_id, scope, slug
        )
        resolved_targets.append(
            {
                "target_id": target_id,
                "surface_id": surface_id,
                "destination": destination,
                "representation": representation,
            }
        )

    manager = IntegrationManager(project_root, home_dir=home)

    # D5/AC3 preconditions for --apply-to are validated here, BEFORE any
    # write happens (including the skill file below). Fail everything up
    # front instead of leaving partial state on a precondition failure.
    apply_to = _csv_values(args.apply_to)
    _tp_agent_models, _tp_warn = trackfw_config.resolve_agent_models(scope, home, project_root)
    if _tp_warn:
        print(_tp_warn, file=sys.stderr)
    ident: identity.Config | None = None
    if apply_to:
        ident = identity.load(home)
        known_agent_ids = {item["id"] for item in catalog["agents"]}
        for agent_id in apply_to:
            if agent_id not in known_agent_ids:
                raise IntegrationError(f"unknown agent item \"{agent_id}\"")
            for rt in resolved_targets:
                _, agent_plans = plan_deployments(
                    "agents",
                    target_ids=[rt["target_id"]],
                    item_ids=[agent_id],
                    scope=scope,
                    identity_cfg=ident,
                    project_root=project_root,
                    agent_models=_tp_agent_models,
                )
                if not agent_plans:
                    raise IntegrationError(
                        f"target {rt['target_id']} has no supported agents surface for item \"{agent_id}\""
                    )
                inspection = manager.inspect(agent_plans[0])
                # ADR imprecision found and resolved here (reported, not
                # silently worked around): D5/D8 never say which scope the
                # referencing agent artifact must be at, and D4 gives
                # third-party a DIFFERENT default scope (project) than the
                # catalog default (global) — so in the common case there
                # is no project-scoped agent artifact to inject into, only
                # a global (home-scoped) one, and a project-relative skill
                # path injected into a global agent file would be broken
                # for every other project sharing that home-scoped file.
                # Resolution: require the agent to already be installed,
                # owned, and NOT hand-modified at the SAME scope as the
                # skill; fail loudly with the exact remediation instead of
                # silently skipping or installing at a mismatched scope.
                if not inspection["managed"] or inspection["state"] == "not-installed":
                    raise IntegrationError(
                        f"cannot attach reference: agent \"{agent_id}\" is not installed at --scope "
                        f"{scope} for target {rt['target_id']}; run `trackfw agents install --scope "
                        f"{scope} --targets {rt['target_id']} --items {agent_id}` first"
                    )
                if inspection["state"] == "modified":
                    raise IntegrationError(
                        f"cannot attach reference: agent \"{agent_id}\" at --scope {scope} for target "
                        f"{rt['target_id']} was modified outside trackfw; run `trackfw agents update "
                        f"--scope {scope} --targets {rt['target_id']} --items {agent_id} --force` first"
                    )

    # AC1 — always show content and every resolved destination before
    # writing anything, in both interactive and non-interactive mode.
    print(f"URL: {entry['url']}\nChecksum: {checksum}\n\n--- content ---")
    print(content.decode("utf-8", errors="replace"))
    print("--- end content ---\n")
    print(f"Resolved destination(s) (scope={scope}):")
    for rt in resolved_targets:
        print(f"  {rt['target_id']}: {rt['destination']}")

    if not stdin_is_interactive():
        if not args.yes_i_trust_this_source:
            raise IntegrationError("install requires --yes-i-trust-this-source in non-interactive mode (AC1)")
    elif not args.yes_i_trust_this_source:
        confirmed = confirm_prompt_runner("Install this third-party content at the destination(s) shown above?")
        if not confirmed:
            raise IntegrationError("install cancelled")

    # D4-bis — global scope requires ITS OWN explicit confirmation, beyond
    # --yes-i-trust-this-source (decision by KG, 2026-08-15, superseding
    # the ML-3A choice to collapse both into --yes-i-trust-this-source).
    if scope == "global" and not getattr(args, "yes_global_scope_unverified", False):
        raise IntegrationError(_THIRD_PARTY_GLOBAL_SCOPE_REFUSAL)

    # D8c — the TOCTOU-closing approval check, verified per resolved
    # destination (provenance is keyed by destination, not by checksum
    # alone).
    for rt in resolved_targets:
        try:
            thirdparty.verify_approval(project_root, checksum, rt["destination"])
        except RuntimeError as error:
            raise IntegrationError(f"not approved for {rt['destination']}: {error}") from error

    # D3 — a failed marker check is not fatal to fetch
    # (--force-thirdparty-markers already overrode that refusal), but
    # install requires the approver to have knowingly recorded
    # marker_override in the provenance entry for each destination.
    if entry.get("marker_check", {}).get("result") == "fail":
        prov = thirdparty.load_provenance(project_root)
        for rt in resolved_targets:
            prov_entry = prov["entries"].get(rt["destination"], {})
            if not prov_entry.get("marker_override"):
                raise IntegrationError(
                    f"refused: {rt['destination']} failed the D3 boundary marker check "
                    f"(matched={entry['marker_check'].get('matched_markers')}) and its provenance "
                    "entry lacks marker_override=true"
                )

    # Write the skill file(s) — always through IntegrationManager, never a
    # raw file write. Claim.kind is always "skills" regardless of which
    # lifecycle ("skills"/"agents") invoked this command: the artifact
    # always lives under "skills/thirdparty/" per D5.
    normalized = thirdparty.normalize_third_party_content(content)
    plans: list[dict[str, Any]] = []
    for rt in resolved_targets:
        plans.append(
            {
                "claim": {
                    "target": rt["target_id"],
                    "surface": rt["surface_id"],
                    "scope": scope,
                    "kind": "skills",
                    "item": f"thirdparty-{slug}",
                    # ADR-2026-08-15 D11 — marks this destination for the
                    # thirdparty_artifact_has_provenance validate rule.
                    # Never set for catalog claims (see catalog.py's
                    # plan_deployments), which keeps old manifests
                    # (no "origin" key at all) reading as catalog claims.
                    "origin": "thirdparty",
                },
                "destination": rt["destination"],
                "content": normalized,
                "catalog_version": f"thirdparty:{checksum[:12]}",
                "support_level": "native",
                "representation": rt["representation"],
            }
        )
    manager.install(plans, force=False)

    # D2-bis — record installed_sha256 (SHA-256 of the NORMALIZED bytes
    # just installed) on each destination's existing provenance entry, now
    # that the install actually succeeded. checksum_sha256 (the raw-bytes
    # D8c approval anchor, written externally by the approver) is left
    # untouched — only installed_sha256 is added/overwritten.
    # rt["destination"] is already the provenance key: the
    # project-root-relative (pre-resolve) string
    # resolve_third_party_skill_destination returns, the same value
    # verify_approval was just called with above.
    installed_sha256 = thirdparty.checksum(normalized)
    for rt in resolved_targets:
        prov = thirdparty.load_provenance(project_root)
        prov_entry = dict(prov["entries"].get(rt["destination"], {}))
        prov_entry["installed_sha256"] = installed_sha256
        thirdparty.upsert_provenance_entry(project_root, rt["destination"], prov_entry)

    # D5 — attach references to the requested catalog agent artifacts, at
    # the SAME scope as the skill file just installed. Preconditions were
    # already validated above, before the skill file was written — this
    # loop only performs writes.
    if apply_to:
        for agent_id in apply_to:
            for rt in resolved_targets:
                thirdparty.upsert_third_party_reference(
                    project_root,
                    rt["target_id"],
                    agent_id,
                    {"slug": slug, "destination": rt["destination"], "url": entry["url"]},
                )
                # Re-render now so the on-disk artifact reflects the new
                # reference immediately — not only on the next `agents
                # update`. plan_deployments picks up the registry entry
                # just written via apply_third_party_references.
                _, agent_plans = plan_deployments(
                    "agents",
                    target_ids=[rt["target_id"]],
                    item_ids=[agent_id],
                    scope=scope,
                    identity_cfg=ident,
                    project_root=project_root,
                    agent_models=_tp_agent_models,
                )
                manager.update(agent_plans, force=False)

    print(f"installed: {len(plans)} destination(s)")
