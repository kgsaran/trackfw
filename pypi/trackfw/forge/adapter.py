"""Adaptadores por forge para o comando trackfw ship.

Cada forge é descrito por um Adapter que informa:
- noun: substantivo do PR/MR (ex: "Pull Request" ou "Merge Request")
- cli_name: nome do executável CLI (vazio = sem CLI)
- cli_args: lista de argumentos para criar o PR/MR
- available: True quando o CLI está no PATH e TRACKFW_DISABLE_EXTERNAL_COMMANDS != "1"

Bitbucket não possui CLI oficial — always falls back to URL.
"""

from __future__ import annotations

import os
import shutil
from dataclasses import dataclass, field
from typing import Callable, List, Optional


# ---------------------------------------------------------------------------
# Verificação de disponibilidade padrão
# ---------------------------------------------------------------------------


def _default_avail_fn(name: str) -> bool:
    """Verifica se *name* existe no PATH, respeitando TRACKFW_DISABLE_EXTERNAL_COMMANDS."""
    if not name:
        return False
    if os.environ.get("TRACKFW_DISABLE_EXTERNAL_COMMANDS") == "1":
        return False
    return shutil.which(name) is not None


# ---------------------------------------------------------------------------
# Conversão de remote URL → base HTTPS
# ---------------------------------------------------------------------------


def remote_https_base(raw_url: str, forge: str) -> str:
    """Converte qualquer formato de remote git para URL base HTTPS sem .git.

    Suporta: git@, ssh://, https://, http://

    Normalização especial para Azure SSH:
        git@ssh.dev.azure.com:v3/org/project/repo
        → https://dev.azure.com/org/project/_git/repo
    """
    raw_url = (raw_url or "").strip()
    if not raw_url:
        return ""

    host = ""
    path_str = ""

    if raw_url.startswith("git@"):
        # git@host:path
        rest = raw_url[4:]
        colon_idx = rest.find(":")
        if colon_idx < 0:
            return ""
        host = rest[:colon_idx].lower()
        path_str = rest[colon_idx + 1:]

    elif raw_url.startswith("ssh://"):
        # ssh://[user@]host/path
        rest = raw_url[6:]
        at_idx = rest.find("@")
        if at_idx >= 0:
            rest = rest[at_idx + 1:]
        slash_idx = rest.find("/")
        if slash_idx < 0:
            host = rest.lower()
        else:
            host = rest[:slash_idx].lower()
            path_str = rest[slash_idx + 1:]

    elif raw_url.startswith("https://") or raw_url.startswith("http://"):
        rest = raw_url.removeprefix("https://").removeprefix("http://")
        at_idx = rest.find("@")
        if at_idx >= 0:
            rest = rest[at_idx + 1:]
        slash_idx = rest.find("/")
        if slash_idx < 0:
            host = rest.lower()
        else:
            host = rest[:slash_idx].lower()
            path_str = rest[slash_idx + 1:]

    else:
        return ""

    path_str = path_str.removesuffix(".git").strip("/")

    # Normalização Azure SSH
    if forge == "azure" and host == "ssh.dev.azure.com":
        host = "dev.azure.com"
        path_str = path_str.removeprefix("v3/")
        parts = path_str.split("/", 2)
        if len(parts) == 3:
            path_str = f"{parts[0]}/{parts[1]}/_git/{parts[2]}"

    if not path_str:
        return f"https://{host}"
    return f"https://{host}/{path_str}"


# ---------------------------------------------------------------------------
# Adapter
# ---------------------------------------------------------------------------


@dataclass
class Adapter:
    forge: str
    noun: str
    cli_name: str
    cli_args: List[str]
    available: bool

    def fallback_url(self, remote_url: str, branch: str) -> str:
        """Retorna a URL para abrir o PR/MR no browser."""
        base = remote_https_base(remote_url, self.forge)
        if not base:
            return ""
        if self.forge == "github":
            return f"{base}/compare/{branch}?expand=1"
        if self.forge == "gitlab":
            return f"{base}/-/merge_requests/new?merge_request[source_branch]={branch}"
        if self.forge == "bitbucket":
            return f"{base}/pull-requests/new?source={branch}"
        if self.forge == "azure":
            return f"{base}/pullrequestcreate?sourceRef={branch}"
        return ""


def forge_adapter(
    forge: str,
    avail_fn: Optional[Callable[[str], bool]] = None,
) -> Adapter:
    """Retorna o Adapter para o forge informado.

    Args:
        forge: identificador do forge ("github", "gitlab", "bitbucket", "azure").
        avail_fn: função de verificação de disponibilidade de CLI (injetável para testes).
                  Quando None, usa shutil.which com respeito a TRACKFW_DISABLE_EXTERNAL_COMMANDS.
    """
    if avail_fn is None:
        avail_fn = _default_avail_fn

    if forge == "github":
        return Adapter(
            forge="github",
            noun="Pull Request",
            cli_name="gh",
            cli_args=["pr", "create"],
            available=avail_fn("gh"),
        )
    if forge == "gitlab":
        return Adapter(
            forge="gitlab",
            noun="Merge Request",
            cli_name="glab",
            cli_args=["mr", "create"],
            available=avail_fn("glab"),
        )
    if forge == "azure":
        return Adapter(
            forge="azure",
            noun="Pull Request",
            cli_name="az",
            cli_args=["repos", "pr", "create"],
            available=avail_fn("az"),
        )
    if forge == "bitbucket":
        # Bitbucket não possui CLI — nunca chama avail_fn
        return Adapter(
            forge="bitbucket",
            noun="Pull Request",
            cli_name="",
            cli_args=[],
            available=False,
        )
    # forge desconhecido
    return Adapter(
        forge=forge,
        noun="Pull Request",
        cli_name="",
        cli_args=[],
        available=False,
    )
