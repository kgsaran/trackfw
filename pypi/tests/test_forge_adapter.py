"""Testes para pypi/trackfw/forge/adapter.py"""

import pytest
from trackfw.forge.adapter import forge_adapter, remote_https_base

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

avail_true  = lambda _: True   # noqa: E731
avail_false = lambda _: False  # noqa: E731


def make_spy(calls: list, ret: bool):
    """Retorna uma avail_fn que registra chamadas em *calls* e retorna *ret*."""
    def spy(name: str) -> bool:
        calls.append(name)
        return ret
    return spy


# ---------------------------------------------------------------------------
# Nouns e cli_name
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("forge,want_noun,want_cli", [
    ("github",    "Pull Request",  "gh"),
    ("gitlab",    "Merge Request", "glab"),
    ("azure",     "Pull Request",  "az"),
    ("bitbucket", "Pull Request",  ""),
])
def test_nouns_and_cli_name(forge, want_noun, want_cli):
    a = forge_adapter(forge, avail_true)
    assert a.noun == want_noun,     f"{forge}: noun={a.noun!r} want {want_noun!r}"
    assert a.cli_name == want_cli,  f"{forge}: cli_name={a.cli_name!r} want {want_cli!r}"


# ---------------------------------------------------------------------------
# cli_args
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("forge,want_args", [
    ("github",    ["pr", "create"]),
    ("gitlab",    ["mr", "create"]),
    ("azure",     ["repos", "pr", "create"]),
    ("bitbucket", []),
])
def test_cli_args(forge, want_args):
    a = forge_adapter(forge, avail_true)
    assert a.cli_args == want_args


# ---------------------------------------------------------------------------
# Disponibilidade via avail_fn injetada
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("forge", ["github", "gitlab", "azure"])
def test_available_true(forge):
    a = forge_adapter(forge, avail_true)
    assert a.available is True, f"{forge}: available deve ser True"


@pytest.mark.parametrize("forge", ["github", "gitlab", "azure"])
def test_available_false(forge):
    a = forge_adapter(forge, avail_false)
    assert a.available is False, f"{forge}: available deve ser False"


# ---------------------------------------------------------------------------
# Bitbucket nunca chama avail_fn
# ---------------------------------------------------------------------------

def test_bitbucket_never_calls_avail_fn():
    calls: list = []
    a = forge_adapter("bitbucket", make_spy(calls, True))
    assert a.available is False, "bitbucket: available deve ser sempre False"
    assert calls == [], f"avail_fn não deve ser chamada; chamada com {calls}"


def test_github_calls_avail_fn_with_gh():
    calls: list = []
    forge_adapter("github", make_spy(calls, True))
    assert calls == ["gh"]


def test_gitlab_calls_avail_fn_with_glab():
    calls: list = []
    forge_adapter("gitlab", make_spy(calls, False))
    assert calls == ["glab"]


def test_azure_calls_avail_fn_with_az():
    calls: list = []
    forge_adapter("azure", make_spy(calls, False))
    assert calls == ["az"]


# ---------------------------------------------------------------------------
# Forge desconhecido
# ---------------------------------------------------------------------------

def test_unknown_forge():
    a = forge_adapter("unknown-forge", avail_true)
    assert a.available is False
    assert a.cli_name == ""


# ---------------------------------------------------------------------------
# fallback_url — casos principais
# ---------------------------------------------------------------------------

BRANCH = "feat/my-feature"

@pytest.mark.parametrize("forge,remote_url,want", [
    # GitHub — HTTPS
    ("github", "https://github.com/org/repo.git",
     "https://github.com/org/repo/compare/feat/my-feature?expand=1"),
    # GitHub — SSH
    ("github", "git@github.com:org/repo.git",
     "https://github.com/org/repo/compare/feat/my-feature?expand=1"),
    # GitHub — self-hosted
    ("github", "https://git.company.com/org/repo.git",
     "https://git.company.com/org/repo/compare/feat/my-feature?expand=1"),
    # GitLab — HTTPS
    ("gitlab", "https://gitlab.com/org/repo.git",
     "https://gitlab.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature"),
    # GitLab — SSH
    ("gitlab", "git@gitlab.com:org/repo.git",
     "https://gitlab.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature"),
    # GitLab — self-hosted
    ("gitlab", "https://gitlab.company.com/org/repo.git",
     "https://gitlab.company.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature"),
    # Bitbucket — HTTPS
    ("bitbucket", "https://bitbucket.org/org/repo.git",
     "https://bitbucket.org/org/repo/pull-requests/new?source=feat/my-feature"),
    # Bitbucket — SSH
    ("bitbucket", "git@bitbucket.org:org/repo.git",
     "https://bitbucket.org/org/repo/pull-requests/new?source=feat/my-feature"),
    # Azure — HTTPS
    ("azure", "https://dev.azure.com/org/project/_git/repo",
     "https://dev.azure.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature"),
    # Azure — SSH (normalização)
    ("azure", "git@ssh.dev.azure.com:v3/org/project/repo",
     "https://dev.azure.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature"),
    # Azure — self-hosted
    ("azure", "https://azdo.company.com/org/project/_git/repo",
     "https://azdo.company.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature"),
])
def test_fallback_url(forge, remote_url, want):
    a = forge_adapter(forge, avail_false)
    got = a.fallback_url(remote_url, BRANCH)
    assert got == want, f"\nforge={forge}\nremote={remote_url}\ngot:  {got}\nwant: {want}"


# ---------------------------------------------------------------------------
# fallback_url — casos de borda
# ---------------------------------------------------------------------------

def test_fallback_url_empty_remote():
    a = forge_adapter("github", avail_false)
    assert a.fallback_url("", "main") == ""


def test_fallback_url_unknown_forge():
    a = forge_adapter("unknown", avail_false)
    assert a.fallback_url("https://example.com/org/repo.git", "main") == ""
