"""
doctor.py — Comando `trackfw doctor` (ML-2A).

Detects artifacts on disk missing from the manifest, and distinguishes them
from hand-modified artifacts. Mirrors internal/commands/doctor.go and
npm/src/commands/doctor.js.
"""

import json

from trackfw.integrations.doctor import HAND_MODIFIED, UNREGISTERED_WRITE, run_doctor


def _print_report(findings: list) -> str:
    if not findings:
        return "trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact."
    unregistered = sum(1 for finding in findings if finding["finding"] == UNREGISTERED_WRITE)
    hand_modified = sum(1 for finding in findings if finding["finding"] == HAND_MODIFIED)
    lines = [
        f"trackfw doctor: {len(findings)} finding(s) -- {unregistered} unregistered-write, {hand_modified} hand-modified",
        "",
    ]
    for finding in findings:
        lines.append(f"[{finding['finding']}] {finding['destination']}")
        lines.append(f"  remedy: {finding['remedy']}")
        lines.append("")
    return "\n".join(lines).rstrip("\n")


def run(args) -> int:
    findings = run_doctor()
    if getattr(args, "json", False):
        print(json.dumps(findings, indent=2))
        return 0
    print(_print_report(findings))
    return 0


def register(subparsers):
    """Registra o subcomando 'doctor' no parser principal."""
    parser = subparsers.add_parser(
        "doctor",
        help="Detect artifacts on disk missing from the manifest, and distinguish them from hand-modified artifacts",
    )
    parser.add_argument("--json", action="store_true", help="Emit findings as a JSON array instead of the text report")
    parser.set_defaults(func=run)
    return parser
