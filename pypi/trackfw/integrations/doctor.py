"""
doctor.py — classificação e varredura para `trackfw doctor` (ML-2A).

Mirrors internal/integrations/doctor.go (Go, canonical source of truth for
wording/semantics) and npm/src/integrations/doctor.js — see the Go module's
doc comments for the full rationale. Kept in sync deliberately;
docs/cli-parity.md ML-2B is the gate that compares the three CLIs' real
output.
"""

from __future__ import annotations

import os
from typing import Any

from trackfw import config as trackfw_config
from trackfw.identity import load as load_identity

from .catalog import plan_deployments
from .manager import IntegrationManager
# alias: o parametro `home_dir` sombreia o nome importado
from trackfw.homedir import home_dir as _user_home_dir

# The three disk/manifest mismatches doctor reports. They require different
# remedies and must never be merged — see
# docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md,
# ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md,
# and docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md (ML-3A —
# UNKNOWN_CONTENT used to fall silently outside classify_doctor's branches).
UNREGISTERED_WRITE = "unregistered-write"
HAND_MODIFIED = "hand-modified"
# SCAFFOLD_DIVERGENT / SCAFFOLD_MISSING: scaffold artifact (property by path —
# ADR-2026-08-27) exists but content differs from the template, or is absent.
# These findings have no claim (scaffold artifacts are never in the manifest).
# Remedy: trackfw update. Mirrors Go's DoctorScaffoldDivergent/DoctorScaffoldMissing.
SCAFFOLD_DIVERGENT = "scaffold-divergent"
SCAFFOLD_MISSING = "scaffold-missing"
# SCAFFOLD_WRONG_MODE: a scaffold artifact whose content is correct (byte-equal to the
# template) but whose owner-execute bit is missing. Distinct from SCAFFOLD_DIVERGENT
# (content mismatch) — AC3 of REQ-2026-08-28. Only emitted for the 5 scripts the
# generator writes with mode 0o755 (AC11). Uses mode & 0o100 != 0 (not == 0o755) so
# umask-narrowed modes 0o750/0o700 are accepted (AC10). Not emitted on Windows
# (sys.platform == "win32") where the execute bit is not representable (AC5).
SCAFFOLD_WRONG_MODE = "scaffold-wrong-mode"
# UNKNOWN_CONTENT: neither does any manifest entry exist for this
# destination, NOR does the on-disk content match the catalog template
# (registered=False and state="modified"). Genuinely ambiguous between a
# file that simply is not trackfw's occupying a catalog destination, and an
# orphaned trackfw artifact whose bytes drifted once the catalog moved on —
# exactly the state that makes `agents install`'s preflight refuse with
# "unmanaged artifact". The remedy names that refusal literally, with both
# branches, instead of picking a side.
UNKNOWN_CONTENT = "unknown-content"

# --remote modality findings (ADR-2026-09-02, ML-3A) — see
# trackfw.commands.doctor_remote for the full rationale. Mirrors Go's
# DoctorRequiredStatusChecksMissing/DoctorEnforceAdminsDisabled/
# DoctorHooksPathNeutralized/DoctorNotEvaluated.
REQUIRED_STATUS_CHECKS_MISSING = "required-status-checks-missing"
ENFORCE_ADMINS_DISABLED = "enforce-admins-disabled"
HOOKS_PATH_NEUTRALIZED = "hooks-path-neutralized"
NOT_EVALUATED = "not-evaluated"


def _doctor_remedy(destination: str, claim: dict[str, Any], effect: str) -> str:
    return (
        f"trackfw {claim['kind']} install --force --items {claim['item']} "
        f"--targets {claim['target']} --scope {claim['scope']}   # {effect}: {destination}"
    )


def classify_doctor(statuses: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Separates the three disk/manifest mismatches doctor reports from every
    other lifecycle state. Deliberately narrow: current-and-registered,
    outdated (handled by `update`), not-installed, and registered under a
    claim OTHER than the one under inspection (managed=False,
    registered=True, regardless of state) are never reported — flagging any
    of those would be the false positive that is this command's dominant
    risk.

    Content at a catalog destination that matches neither the desired bytes
    nor any manifest entry (registered=False and state="modified") IS
    reported, as UNKNOWN_CONTENT — see that constant's doc comment.

    Keys off "registered", not "managed": managed additionally requires this
    exact claim to own the manifest entry, so a destination registered under
    a *different* claim reads managed=False while still being registered.
    Treating that as an "unregistered write" (or, symmetrically, as
    "unknown-content") would be exactly the dominant false-positive doctor
    exists to avoid.
    """
    findings: list[dict[str, Any]] = []
    for status in statuses:
        claim = status.get("claim") or {
            "target": status["target"],
            "surface": status["surface"],
            "scope": status["scope"],
            "kind": status.get("kind"),
            "item": status["item"],
        }
        destination = status.get("resolved_destination", status["destination"])
        if not status["registered"] and status["state"] == "current":
            findings.append(
                {
                    "finding": UNREGISTERED_WRITE,
                    "claim": claim,
                    "destination": destination,
                    "remedy": _doctor_remedy(
                        destination,
                        claim,
                        "adopts it — content already matches the catalog template, only the manifest entry is missing",
                    ),
                }
            )
        elif not status["registered"] and status["state"] == "modified":
            findings.append(
                {
                    "finding": UNKNOWN_CONTENT,
                    "claim": claim,
                    "destination": destination,
                    "remedy": _doctor_remedy(
                        destination,
                        claim,
                        "is ambiguous — content matches neither the catalog template nor a manifest entry, so install will refuse this destination with \"unmanaged artifact\"; if this file is yours, remove or move it; if it is trackfw's and it drifted from the catalog template, this replaces it",
                    ),
                }
            )
        elif status["managed"] and status["state"] == "modified":
            findings.append(
                {
                    "finding": HAND_MODIFIED,
                    "claim": claim,
                    "destination": destination,
                    "remedy": _doctor_remedy(
                        destination,
                        claim,
                        "overwrites it with the catalog template — you will lose the hand edit",
                    ),
                }
            )
    return _sort_doctor_findings(findings)


def _sort_doctor_findings(findings: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Orders by a total key — destination alone is not total when a single
    destination carries more than one claim (ML-2B's gate needs
    deterministic order across three independent CLI implementations)."""
    return sorted(
        findings,
        key=lambda finding: (
            finding["destination"],
            finding["claim"]["kind"] or "",
            finding["claim"]["item"],
            finding["claim"]["target"],
            finding["claim"]["surface"],
            finding["claim"]["scope"],
        ),
    )


def run_doctor(
    project_root: str | None = None,
    home_dir: str | None = None,
    identity_cfg: Any = None,
) -> list[dict[str, Any]]:
    """Sweeps every catalog kind (agents, skills) in both scopes (project,
    global) and returns every classify_doctor finding. plan_deployments
    already skips a surface that does not support the requested scope
    (`install_paths = [entry for entry in surface["paths"][kind] if
    entry["scope"] == scope]` in catalog.py) — unlike the Go BuildPlans,
    which errors on that case by design for explicit install/update
    requests — so no extra per-surface filtering is needed here.
    """
    project_root = project_root or os.getcwd()
    home_dir = home_dir or _user_home_dir()
    # Identity must be resolved from disk before plan_deployments — skipping
    # this step would silently revert custom agent names to the neutral
    # defaults, manufacturing a hash mismatch and a false positive. Mirrors
    # integrations/command.py:run().
    ident = identity_cfg if identity_cfg is not None else load_identity(home_dir)
    manager = IntegrationManager(project_root, home_dir)

    findings: list[dict[str, Any]] = []
    for kind in ("agents", "skills"):
        for scope in ("project", "global"):
            _catalog, plans = plan_deployments(
                kind, scope=scope, all_surfaces=True, identity_cfg=ident, project_root=project_root,
                agent_models=trackfw_config.load(project_root).get("agent_models", {}),
            )
            if not plans:
                continue
            statuses = manager.list_full(plans)
            findings.extend(classify_doctor(statuses))
    return _sort_doctor_findings(findings)
