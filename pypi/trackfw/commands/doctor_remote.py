"""
doctor_remote.py — the --remote modality of `trackfw doctor` (ADR-2026-09-02, ML-3A).

Mirrors internal/commands/doctor_remote.go (Go, canonical source of truth for wording/
semantics) — see that file's doc comments for the full rationale: GitHub branch protection
(required_status_checks, enforce_admins) plus local core.hooksPath neutralization. Never runs
unless explicitly requested via --remote.
"""

import json
import subprocess

from trackfw.forge.adapter import forge_adapter
from trackfw.forge.resolve import resolve as resolve_forge
from trackfw.integrations.doctor import (
    ENFORCE_ADMINS_DISABLED,
    HOOKS_PATH_NEUTRALIZED,
    NOT_EVALUATED,
    REQUIRED_STATUS_CHECKS_MISSING,
)

# Values that discard every git hook invocation on each OS git supports as a hooksPath target.
# Anything else (including unset) is left alone — a custom husky/lefthook directory is
# legitimate and must never be flagged.
_NEUTRALIZED_HOOKS_PATH_VALUES = {"/dev/null", "NUL"}


def default_exec_git(args):
    """Production git executor. Returns (stdout_str, error_str_or_None)."""
    try:
        result = subprocess.run(["git"] + args, capture_output=True, text=True)
        if result.returncode != 0:
            return (
                "",
                result.stderr.strip() or f"git {' '.join(args)} exited with {result.returncode}",
            )
        return (result.stdout.strip(), None)
    except FileNotFoundError:
        return ("", "git not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


def default_exec_forge_api(name, args, stdin):
    """Production forge CLI executor. Returns (stdout_str, error_str_or_None)."""
    try:
        result = subprocess.run([name] + args, input=stdin, capture_output=True, text=True)
        if result.returncode != 0:
            msg = result.stderr.strip() or f"{name} {' '.join(args)} exited with {result.returncode}"
            return (result.stdout.strip(), msg)
        return (result.stdout.strip(), None)
    except FileNotFoundError:
        return ("", f"{name} not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


def run_doctor_remote(
    exec_git=None,
    exec_forge_api=None,
    avail_fn=None,
    config_forge="",
    repo_dir="",
):
    """
    Implements the --remote modality: every branch either produces a genuine finding
    (evaluated, and wrong) or a NOT_EVALUATED finding (could not evaluate) — never silence,
    which would read as "ok" to a report that treats an empty finding list as a clean bill of
    health.

    Every dependency is injectable so the whole check runs deterministically offline in tests.
    """
    exec_git = exec_git or default_exec_git
    exec_forge_api = exec_forge_api or default_exec_forge_api

    findings = []

    # -- Local check: core.hooksPath neutralized (no network needed) --------------------
    hooks_path, hooks_path_err = exec_git(["config", "--get", "core.hooksPath"])
    if not hooks_path_err and hooks_path.strip() in _NEUTRALIZED_HOOKS_PATH_VALUES:
        findings.append(
            {
                "finding": HOOKS_PATH_NEUTRALIZED,
                "destination": "git:core.hooksPath",
                "remedy": (
                    f'git config --unset core.hooksPath   # currently "{hooks_path.strip()}" '
                    "discards every hook invocation; unset to restore .git/hooks, or point it "
                    "at your real hooks directory"
                ),
            }
        )

    # -- Forge resolution: only GitHub is evaluated; every other forge is not applicable --
    remote_url, remote_url_err = exec_git(["remote", "get-url", "origin"])
    remote_url = "" if remote_url_err else remote_url.strip()
    try:
        resolution = resolve_forge(config_forge=config_forge, remote_url=remote_url, repo_dir=repo_dir)
    except ValueError:
        findings.append(_not_applicable_finding("unknown"))
        return findings

    if resolution.forge != "github":
        findings.append(_not_applicable_finding(resolution.forge))
        return findings

    # -- gh CLI availability ---------------------------------------------------------------
    adapter = forge_adapter("github", avail_fn)
    if not adapter.available:
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": "branch-protection",
                "remedy": (
                    "install the GitHub CLI (gh) to evaluate branch protection remotely: "
                    "https://cli.github.com, then retry with --remote"
                ),
            }
        )
        return findings

    # -- Credential presence: distinct from credential SCOPE below (ADR-2026-09-02) -------
    _, auth_err = exec_forge_api("gh", ["auth", "status"], "")
    if auth_err:
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": "branch-protection",
                "remedy": (
                    "GitHub CLI has no credential — authenticate first: gh auth login "
                    "(or set GITHUB_TOKEN/GH_TOKEN), then retry with --remote"
                ),
            }
        )
        return findings

    # -- Repository info: default branch + whether this credential has admin access -------
    repo_info_resp, repo_info_err = exec_forge_api("gh", ["api", "repos/{owner}/{repo}"], "")
    if repo_info_err:
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": "branch-protection",
                "remedy": (
                    f"could not reach the GitHub API to resolve this repository: {repo_info_err}. "
                    "Check network connectivity and retry with --remote"
                ),
            }
        )
        return findings

    try:
        repo_info = json.loads(repo_info_resp)
    except (ValueError, TypeError):
        repo_info = None
    default_branch = (repo_info or {}).get("default_branch") if isinstance(repo_info, dict) else None
    if not default_branch:
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": "branch-protection",
                "remedy": (
                    f"could not parse the repository response from the GitHub API: "
                    f"{repo_info_resp}. Retry with --remote"
                ),
            }
        )
        return findings

    # Credential SCOPE: reading branch protection requires admin access to the repository.
    # Distinct remedy from "no credential" above — one is fixed by authenticating, this one
    # by being granted admin access (or using a token for a repo you administer).
    permissions = repo_info.get("permissions") or {}
    if not permissions.get("admin"):
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": "branch-protection",
                "remedy": (
                    "the authenticated GitHub credential lacks admin access to this repository "
                    "— reading branch protection requires it. Ask a repository admin to grant "
                    "access, or authenticate as an account that has it, then retry with --remote"
                ),
            }
        )
        return findings

    # -- Branch protection itself -----------------------------------------------------------
    protection_resp, protection_err = exec_forge_api(
        "gh", ["api", f"repos/{{owner}}/{{repo}}/branches/{default_branch}/protection"], ""
    )
    if protection_err:
        if "(HTTP 404)" in protection_err:
            # Evaluated (admin confirmed above): the branch genuinely has no protection at
            # all, which means both checks fail — GitHub does not return the two settings
            # separately when there is no rule to read them from.
            findings.append(_required_checks_missing_finding(default_branch))
            findings.append(_enforce_admins_disabled_finding(default_branch))
            return findings
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": f"branch-protection:{default_branch}",
                "remedy": (
                    f"could not read branch protection from the GitHub API: {protection_err}. "
                    "This may be transient (rate limit, network) — retry with --remote"
                ),
            }
        )
        return findings

    try:
        protection = json.loads(protection_resp)
    except (ValueError, TypeError):
        findings.append(
            {
                "finding": NOT_EVALUATED,
                "destination": f"branch-protection:{default_branch}",
                "remedy": (
                    f"could not parse the branch protection response from the GitHub API: "
                    f"{protection_resp}. Retry with --remote"
                ),
            }
        )
        return findings

    rsc = protection.get("required_status_checks") or {}
    contexts = rsc.get("contexts") or []
    checks = rsc.get("checks") or []
    if not protection.get("required_status_checks") or (not contexts and not checks):
        findings.append(_required_checks_missing_finding(default_branch))

    enforce_admins = protection.get("enforce_admins") or {}
    if not enforce_admins.get("enabled"):
        findings.append(_enforce_admins_disabled_finding(default_branch))

    return findings


def _not_applicable_finding(forge_name):
    return {
        "finding": NOT_EVALUATED,
        "destination": "branch-protection",
        "remedy": (
            "not applicable: branch protection is checked only on GitHub, and this "
            f'repository\'s forge resolved to "{forge_name}". Not a failure — no action '
            "needed unless this repository is actually hosted on GitHub, in which case set "
            "forge: github in trackfw.yaml or add a github.com origin remote."
        ),
    }


def _required_checks_missing_finding(default_branch):
    return {
        "finding": REQUIRED_STATUS_CHECKS_MISSING,
        "destination": f"branch-protection:{default_branch}:required_status_checks",
        "remedy": (
            f"configure required status checks: GitHub repo Settings > Branches > Branch "
            f"protection rules > {default_branch} > Require status checks to pass before merging"
        ),
    }


def _enforce_admins_disabled_finding(default_branch):
    return {
        "finding": ENFORCE_ADMINS_DISABLED,
        "destination": f"branch-protection:{default_branch}:enforce_admins",
        "remedy": f"gh api repos/{{owner}}/{{repo}}/branches/{default_branch}/protection/enforce_admins --method POST",
    }
