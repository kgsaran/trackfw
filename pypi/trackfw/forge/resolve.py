"""
forge/resolve.py — Forge resolution for trackfw ship.

Precedence (highest → lowest):
  1. --forge flag (explicit override)
  2. forge: field in trackfw.yaml
  3. Host extracted from git remote get-url origin
  4. CI configuration files in repo root (.gitlab-ci.yml / .github/workflows/)
  5. "manual" — never an error
"""

import os
import subprocess
from dataclasses import dataclass
from typing import Optional

# Accepted values for the --forge flag and forge: config field.
VALID_FORGES = ["github", "gitlab", "bitbucket", "azure"]


@dataclass
class Resolution:
    """Holds the resolved forge and the source that decided it."""
    forge: str   # "github", "gitlab", "bitbucket", "azure", or "manual"
    source: str  # "flag", "config", "remote", "ci", or "none"


def resolve(
    flag_forge: str = "",
    config_forge: str = "",
    remote_url: str = "",
    repo_dir: str = "",
) -> Resolution:
    """
    Determine the active forge using the ADR precedence.

    Parameters are injected for full testability (no subprocess inside).

    Raises:
        ValueError: if flag_forge or config_forge is not in VALID_FORGES.

    Returns:
        Resolution with forge ∈ {"github","gitlab","bitbucket","azure","manual"}
        and source ∈ {"flag","config","remote","ci","none"}.
        Resolution{forge="manual", source="none"} is never an error.
    """
    # 1. Explicit flag wins everything.
    if flag_forge:
        _validate_forge(flag_forge)
        return Resolution(forge=flag_forge, source="flag")

    # 2. Config field.
    if config_forge:
        _validate_forge(config_forge)
        return Resolution(forge=config_forge, source="config")

    # 3. Remote URL — known hosts only.
    if remote_url:
        forge = _forge_from_remote_url(remote_url)
        if forge:
            return Resolution(forge=forge, source="remote")

    # 4. CI files — desempate for self-hosted / unknown host.
    if repo_dir:
        forge = _forge_from_ci(repo_dir)
        if forge:
            return Resolution(forge=forge, source="ci")

    # 5. Manual — not an error.
    return Resolution(forge="manual", source="none")


def resolve_from_repo(
    flag_forge: str = "",
    config_forge: str = "",
    repo_dir: str = "",
) -> Resolution:
    """
    Production entry point. Runs 'git remote get-url origin' in repo_dir,
    then calls resolve().
    """
    remote_url = _git_remote_url(repo_dir)
    return resolve(
        flag_forge=flag_forge,
        config_forge=config_forge,
        remote_url=remote_url,
        repo_dir=repo_dir,
    )


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _validate_forge(forge: str) -> None:
    """Raises ValueError if forge is not in VALID_FORGES."""
    if forge not in VALID_FORGES:
        valid = ", ".join(VALID_FORGES)
        raise ValueError(
            f'invalid forge "{forge}": accepted values are {valid}'
        )


def _forge_from_remote_url(raw_url: str) -> str:
    """Returns forge name for known hosts; empty string for unknown / self-hosted."""
    host = _extract_host(raw_url)
    return _host_to_forge(host)


def _extract_host(raw_url: str) -> str:
    """Extracts the lowercase hostname from HTTPS, SSH (git@) or ssh:// URLs."""
    raw_url = raw_url.strip()

    # SSH short form: git@github.com:org/repo.git
    if raw_url.startswith("git@"):
        rest = raw_url[4:]  # strip "git@"
        colon_idx = rest.find(":")
        if colon_idx >= 0:
            return rest[:colon_idx].lower()
        return ""

    # SSH long form: ssh://git@github.com/org/repo.git
    if raw_url.startswith("ssh://"):
        rest = raw_url[6:]  # strip "ssh://"
        at_idx = rest.find("@")
        if at_idx >= 0:
            rest = rest[at_idx + 1:]  # strip user@
        slash_idx = rest.find("/")
        if slash_idx >= 0:
            return rest[:slash_idx].lower()
        return rest.lower()

    # HTTPS / HTTP
    if raw_url.startswith("https://") or raw_url.startswith("http://"):
        rest = raw_url.removeprefix("https://").removeprefix("http://")
        at_idx = rest.find("@")
        if at_idx >= 0:
            rest = rest[at_idx + 1:]  # strip optional user:pass@
        slash_idx = rest.find("/")
        if slash_idx >= 0:
            return rest[:slash_idx].lower()
        return rest.lower()

    return ""


def _host_to_forge(host: str) -> str:
    """
    Maps a known hostname to its forge identifier.
    Azure DevOps uses dev.azure.com (HTTPS) and ssh.dev.azure.com (SSH).
    Returns '' for unknown / self-hosted hosts.
    """
    if not host:
        return ""
    if host == "github.com":
        return "github"
    if host == "gitlab.com":
        return "gitlab"
    if host == "bitbucket.org":
        return "bitbucket"
    if (
        host == "dev.azure.com"
        or host.endswith(".dev.azure.com")
        or host.endswith(".visualstudio.com")
    ):
        return "azure"
    return ""


def _forge_from_ci(repo_dir: str) -> str:
    """
    Inspects CI indicator files in repo_dir.
    Priority: .gitlab-ci.yml → gitlab; .github/workflows/ → github.
    """
    if os.path.isfile(os.path.join(repo_dir, ".gitlab-ci.yml")):
        return "gitlab"
    workflows = os.path.join(repo_dir, ".github", "workflows")
    if os.path.isdir(workflows):
        return "github"
    return ""


def _git_remote_url(repo_dir: str) -> str:
    """
    Runs 'git remote get-url origin' in repo_dir.
    Returns '' on any error.
    """
    try:
        result = subprocess.run(
            ["git", "remote", "get-url", "origin"],
            cwd=repo_dir or None,
            capture_output=True,
            text=True,
            check=True,
        )
        return result.stdout.strip()
    except Exception:
        return ""
