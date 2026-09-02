"""
doctor.py — Comando `trackfw doctor` (ML-2A).

Detects artifacts on disk missing from the manifest, and distinguishes them
from hand-modified artifacts. Mirrors internal/commands/doctor.go and
npm/src/commands/doctor.js.
"""

import json
import os

from trackfw.config import load as load_config
from trackfw.commands.doctor_remote import run_doctor_remote
from trackfw.integrations.doctor import (
    ENFORCE_ADMINS_DISABLED,
    HAND_MODIFIED,
    HOOKS_PATH_NEUTRALIZED,
    NOT_EVALUATED,
    REQUIRED_STATUS_CHECKS_MISSING,
    SCAFFOLD_DIVERGENT,
    SCAFFOLD_MISSING,
    SCAFFOLD_WRONG_MODE,
    UNKNOWN_CONTENT,
    UNREGISTERED_WRITE,
    run_doctor,
)
from trackfw.integrations.scaffold_doctor import run_scaffold_doctor


def _print_report(findings: list) -> str:
    if not findings:
        return "trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact and all scaffold templates."
    unregistered = sum(1 for finding in findings if finding["finding"] == UNREGISTERED_WRITE)
    hand_modified = sum(1 for finding in findings if finding["finding"] == HAND_MODIFIED)
    unknown_content = sum(1 for finding in findings if finding["finding"] == UNKNOWN_CONTENT)
    scaffold_divergent = sum(1 for finding in findings if finding["finding"] == SCAFFOLD_DIVERGENT)
    scaffold_missing = sum(1 for finding in findings if finding["finding"] == SCAFFOLD_MISSING)
    scaffold_wrong_mode = sum(1 for finding in findings if finding["finding"] == SCAFFOLD_WRONG_MODE)
    required_checks_missing = sum(1 for finding in findings if finding["finding"] == REQUIRED_STATUS_CHECKS_MISSING)
    enforce_admins_disabled = sum(1 for finding in findings if finding["finding"] == ENFORCE_ADMINS_DISABLED)
    hooks_path_neutralized = sum(1 for finding in findings if finding["finding"] == HOOKS_PATH_NEUTRALIZED)
    not_evaluated = sum(1 for finding in findings if finding["finding"] == NOT_EVALUATED)
    lines = [
        f"trackfw doctor: {len(findings)} finding(s) -- {unregistered} unregistered-write, {hand_modified} hand-modified, {unknown_content} unknown-content, {scaffold_divergent} scaffold-divergent, {scaffold_missing} scaffold-missing, {scaffold_wrong_mode} scaffold-wrong-mode, {required_checks_missing} required-status-checks-missing, {enforce_admins_disabled} enforce-admins-disabled, {hooks_path_neutralized} hooks-path-neutralized, {not_evaluated} not-evaluated",
        "",
    ]
    for finding in findings:
        lines.append(f"[{finding['finding']}] {finding['destination']}")
        lines.append(f"  remedy: {finding['remedy']}")
        lines.append("")
    return "\n".join(lines).rstrip("\n")


def run(args) -> int:
    project_root = os.getcwd()
    catalog_findings = run_doctor()
    # Scaffold coverage (ADR-2026-08-27): compare scaffold artifacts on disk against
    # the templates the current binary would generate, using the project's own
    # trackfw.yaml. No manifest entry is written or read (AC3).
    # Note: Python scaffold doctor excludes validate-script and CI workflows —
    # see pypi/trackfw/integrations/scaffold_doctor.py module docstring and
    # docs/cli-parity.md "Reduced surface — Python."
    scaffold_findings = run_scaffold_doctor(project_root)
    findings = catalog_findings + scaffold_findings
    # Remote modality (ADR-2026-09-02, ML-3A): opt-in only — never runs without --remote, so
    # doctor's offline default is unchanged.
    if getattr(args, "remote", False):
        config_forge = load_config().get("forge", "") or ""
        findings = findings + run_doctor_remote(config_forge=config_forge, repo_dir=project_root)
    if getattr(args, "json", False):
        print(json.dumps(findings, indent=2))
        return 0
    print(_print_report(findings))
    return 0


def register(subparsers):
    """Registra o subcomando 'doctor' no parser principal."""
    parser = subparsers.add_parser(
        "doctor",
        help="Detect artifacts on disk missing from the manifest, distinguishing hand-modified artifacts from unknown content",
    )
    parser.add_argument("--json", action="store_true", help="Emit findings as a JSON array instead of the text report")
    parser.add_argument(
        "--remote",
        action="store_true",
        help=(
            "Also check GitHub branch protection (required_status_checks, enforce_admins) and "
            "local core.hooksPath neutralization. Requires network + a GitHub credential with "
            "admin access; absence of either is reported as not-evaluated, never as a pass "
            "(ADR-2026-09-02)."
        ),
    )
    parser.set_defaults(func=run)
    return parser
