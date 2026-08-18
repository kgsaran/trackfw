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

from trackfw.identity import load as load_identity

from .catalog import plan_deployments
from .manager import IntegrationManager

# The two disk/manifest mismatches doctor reports. They require different
# remedies and must never be merged — see
# docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md
# and ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md.
UNREGISTERED_WRITE = "unregistered-write"
HAND_MODIFIED = "hand-modified"


def _doctor_remedy(destination: str, claim: dict[str, Any], effect: str) -> str:
    return (
        f"trackfw {claim['kind']} install --force --items {claim['item']} "
        f"--targets {claim['target']} --scope {claim['scope']}   # {effect}: {destination}"
    )


def classify_doctor(statuses: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Separates the two disk/manifest mismatches doctor reports from every
    other lifecycle state. Deliberately narrow: current-and-registered,
    outdated (handled by `update`), not-installed, and unmanaged content that
    does not match the catalog (not trackfw's) are never reported — flagging
    any of those would be the false positive that is this command's
    dominant risk.

    Keys off "registered", not "managed": managed additionally requires this
    exact claim to own the manifest entry, so a destination registered under
    a *different* claim reads managed=False while still being registered.
    Treating that as an "unregistered write" would be exactly the dominant
    false-positive doctor exists to avoid.
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
    home_dir = home_dir or os.path.expanduser("~")
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
                kind, scope=scope, all_surfaces=True, identity_cfg=ident, project_root=project_root
            )
            if not plans:
                continue
            statuses = manager.list_full(plans)
            findings.extend(classify_doctor(statuses))
    return _sort_doctor_findings(findings)
