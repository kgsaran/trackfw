"""
test_forge_resolve.py — Testes pytest para pypi/trackfw/forge/resolve.py

Cobre os mesmos casos que Go e Node.js:
  - Precedência (flag > config > remote > CI > manual)
  - SSH e HTTPS equivalentes para os 4 hosts conhecidos
  - Azure: dev.azure.com e *.visualstudio.com
  - Self-hosted + .gitlab-ci.yml → gitlab (source: ci)
  - Self-hosted + .github/workflows/ → github (source: ci)
  - Self-hosted sem sinal → manual (source: none), sem erro
  - Sem remote (repo novo) → não entra em pânico; cai para CI ou manual
  - manual é resultado válido, nunca erro
  - Valor inválido em forge: → ValueError com valores aceitos listados
"""

import os
import pytest

from trackfw.forge.resolve import (
    resolve,
    _extract_host,
    _host_to_forge,
    _forge_from_ci,
    VALID_FORGES,
)


# ---------------------------------------------------------------------------
# Precedence
# ---------------------------------------------------------------------------

def test_flag_wins_over_config():
    res = resolve(flag_forge="github", config_forge="gitlab")
    assert res.forge == "github"
    assert res.source == "flag"


def test_flag_wins_over_remote():
    res = resolve(flag_forge="github", remote_url="https://gitlab.com/org/repo.git")
    assert res.forge == "github"
    assert res.source == "flag"


def test_flag_wins_over_ci(tmp_path):
    (tmp_path / ".gitlab-ci.yml").write_text("")
    res = resolve(flag_forge="bitbucket", repo_dir=str(tmp_path))
    assert res.forge == "bitbucket"
    assert res.source == "flag"


def test_config_wins_over_remote():
    res = resolve(config_forge="bitbucket", remote_url="https://github.com/org/repo.git")
    assert res.forge == "bitbucket"
    assert res.source == "config"


def test_config_wins_over_ci(tmp_path):
    (tmp_path / ".gitlab-ci.yml").write_text("")
    res = resolve(config_forge="azure", repo_dir=str(tmp_path))
    assert res.forge == "azure"
    assert res.source == "config"


def test_remote_wins_over_ci(tmp_path):
    (tmp_path / ".gitlab-ci.yml").write_text("")
    res = resolve(remote_url="https://github.com/org/repo.git", repo_dir=str(tmp_path))
    assert res.forge == "github"
    assert res.source == "remote"


# ---------------------------------------------------------------------------
# SSH and HTTPS equivalence for the 4 known hosts
# ---------------------------------------------------------------------------

KNOWN_HOSTS = [
    ("github",    "https://github.com/org/repo.git",                  "git@github.com:org/repo.git"),
    ("gitlab",    "https://gitlab.com/org/repo.git",                  "git@gitlab.com:org/repo.git"),
    ("bitbucket", "https://bitbucket.org/org/repo.git",               "git@bitbucket.org:org/repo.git"),
    ("azure",     "https://dev.azure.com/org/project/_git/repo",      "git@ssh.dev.azure.com:v3/org/project/repo"),
]


@pytest.mark.parametrize("forge,https_url,ssh_url", KNOWN_HOSTS)
def test_https_resolves(forge, https_url, ssh_url):
    res = resolve(remote_url=https_url)
    assert res.forge == forge
    assert res.source == "remote"


@pytest.mark.parametrize("forge,https_url,ssh_url", KNOWN_HOSTS)
def test_ssh_resolves(forge, https_url, ssh_url):
    res = resolve(remote_url=ssh_url)
    assert res.forge == forge
    assert res.source == "remote"


# ---------------------------------------------------------------------------
# Azure: dev.azure.com and *.visualstudio.com
# ---------------------------------------------------------------------------

def test_azure_dev_azure_com():
    res = resolve(remote_url="https://dev.azure.com/myorg/_git/myrepo")
    assert res.forge == "azure"


def test_azure_visualstudio_com():
    res = resolve(remote_url="https://foo.visualstudio.com/DefaultCollection/_git/repo")
    assert res.forge == "azure"


def test_azure_ssh_dev_azure_com():
    res = resolve(remote_url="ssh://git@ssh.dev.azure.com/v3/org/project/repo")
    assert res.forge == "azure"
    assert res.source == "remote"


# ---------------------------------------------------------------------------
# Self-hosted + CI desempate
# ---------------------------------------------------------------------------

def test_self_hosted_gitlab_ci(tmp_path):
    (tmp_path / ".gitlab-ci.yml").write_text("")
    res = resolve(
        remote_url="https://git.empresa.com.br/org/repo.git",
        repo_dir=str(tmp_path),
    )
    assert res.forge == "gitlab"
    assert res.source == "ci"


def test_self_hosted_github_workflows(tmp_path):
    workflows = tmp_path / ".github" / "workflows"
    workflows.mkdir(parents=True)
    res = resolve(
        remote_url="https://git.empresa.com.br/org/repo.git",
        repo_dir=str(tmp_path),
    )
    assert res.forge == "github"
    assert res.source == "ci"


def test_self_hosted_no_ci_is_manual(tmp_path):
    res = resolve(
        remote_url="https://git.empresa.com.br/org/repo.git",
        repo_dir=str(tmp_path),
    )
    assert res.forge == "manual"
    assert res.source == "none"


# ---------------------------------------------------------------------------
# No remote (new repo)
# ---------------------------------------------------------------------------

def test_no_remote_ci_decides(tmp_path):
    (tmp_path / ".gitlab-ci.yml").write_text("")
    res = resolve(remote_url="", repo_dir=str(tmp_path))
    assert res.forge == "gitlab"
    assert res.source == "ci"


def test_no_remote_no_ci_is_manual(tmp_path):
    res = resolve(remote_url="", repo_dir=str(tmp_path))
    assert res.forge == "manual"
    assert res.source == "none"


# ---------------------------------------------------------------------------
# Manual is a valid result, never an error
# ---------------------------------------------------------------------------

def test_manual_is_not_an_error():
    res = resolve()
    assert res.forge == "manual"
    assert res.source == "none"


# ---------------------------------------------------------------------------
# Invalid forge value → ValueError with valid values listed
# ---------------------------------------------------------------------------

def test_invalid_flag_forge_raises():
    with pytest.raises(ValueError) as exc_info:
        resolve(flag_forge="notaforge")
    msg = str(exc_info.value)
    assert "notaforge" in msg
    for v in VALID_FORGES:
        assert v in msg, f"error should mention valid value {v!r}"


def test_invalid_config_forge_raises():
    with pytest.raises(ValueError) as exc_info:
        resolve(config_forge="svn")
    assert "svn" in str(exc_info.value)


# ---------------------------------------------------------------------------
# _extract_host edge cases
# ---------------------------------------------------------------------------

def test_extract_host_empty():
    assert _extract_host("") == ""


def test_extract_host_https_with_credentials():
    assert _extract_host("https://user:pass@github.com/org/repo.git") == "github.com"


def test_extract_host_ssh_long_form():
    assert _extract_host("ssh://git@github.com/org/repo.git") == "github.com"
    assert _extract_host("ssh://git@gitlab.com/org/repo.git") == "gitlab.com"
    assert _extract_host("ssh://git@bitbucket.org/org/repo.git") == "bitbucket.org"
