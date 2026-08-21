"""
test_release.py — Testes para trackfw.release.runner (`trackfw release tag`)

Cobre os mesmos casos que Go e Node.js:
  - Precondition 1: arvore de trabalho limpa
  - Precondition 2: branch default atualizada com o remoto
  - Precondition 3: os 4 arquivos de versao (5 checagens) batendo com a versao pedida
  - Precondition 4: CHANGELOG.md com a secao da versao
  - Precondition 5: tag ainda nao existente, local nem remota
  - Precondition 6: CLI de forge disponivel, apenas GitHub
  - Identidade git (user.name/user.email)
  - Caminho de sucesso: publica via duas chamadas gh api, preservando a anotacao
"""

import json

from trackfw.release.runner import run_release_tag

VERSION = "9.9.9"
TAG = "v9.9.9"
SHA = "abc123def456"


def valid_files(version):
    return {
        "internal/version/version.go": f'package version\n\nvar Version = "{version}"\n',
        "npm/package.json": json.dumps({"name": "trackfw", "version": version}),
        "pypi/pyproject.toml": f'[project]\nname = "trackfw"\nversion = "{version}"\n',
        "pypi/trackfw/__init__.py": (
            "try:\n    from importlib.metadata import version\n"
            f'    __version__ = version("trackfw") or "{version}"\n'
            f'except Exception:\n    __version__ = "{version}"\n'
        ),
        "CHANGELOG.md": f"# Changelog\n\n## [{version}] - 2026-08-19\n\n### Added\n- x\n",
    }


class MockGit:
    def __init__(self, responses=None, errors=None):
        self.responses = {
            "status --porcelain": "",
            "fetch origin --prune": "",
            "symbolic-ref refs/remotes/origin/HEAD": "refs/remotes/origin/main",
            f"rev-parse origin/main": SHA,
            "remote get-url origin": "https://github.com/kgsaran/trackfw.git",
            "config user.name": "Test User",
            "config user.email": "test@example.com",
            f"ls-remote --tags origin refs/tags/{TAG}": "",
        }
        self.errors = {
            "rev-parse -q --verify refs/heads/main": "no such branch",
            f"rev-parse -q --verify refs/tags/{TAG}": "no such tag",
        }
        if responses:
            self.responses.update(responses)
        if errors:
            self.errors.update(errors)
        self.calls = []

    def exec(self, args):
        self.calls.append(list(args))
        key = " ".join(args)
        if key in self.errors:
            return ("", self.errors[key])
        if key in self.responses:
            return (self.responses[key], None)
        return ("", None)


def make_deps(file_overrides=None, git_responses=None, git_errors=None,
              avail_fn=None, exec_forge_api=None):
    files = valid_files(VERSION)
    if file_overrides:
        files.update(file_overrides)
    git = MockGit(responses=git_responses, errors=git_errors)
    out_lines = []
    err_lines = []

    def read_at_commit(sha, path):
        # reads from the files map, keyed by path (ignoring sha — tests control both
        # the sha the forge mock returns and the content in the files map).
        if path not in files:
            return ("", f"object {sha}:{path} not found")
        return (files[path], None)

    deps = dict(
        exec_git=git.exec,
        read_at_commit=read_at_commit,
        writeln=out_lines.append,
        write_err=err_lines.append,
        config_forge="",
        repo_dir="",
        avail_fn=avail_fn if avail_fn is not None else (lambda name: True),
        exec_forge_api=exec_forge_api if exec_forge_api is not None else default_exec_forge_api,
    )
    return deps, git, out_lines, err_lines


# default_exec_forge_api answers the four gh api calls release tag makes:
#   - repos/{owner}/{repo}                       -> default_branch: "main" (agrees with the
#     fixture's symref-derived base, so no divergence fires by default)
#   - repos/{owner}/{repo}/commits/main           -> sha: SHA (agrees with the fixture's local
#     origin/main, so no divergence fires by default)
#   - repos/{owner}/{repo}/git/tags  (POST)       -> sha: "tagobjectsha000"
#   - repos/{owner}/{repo}/git/refs  (POST)       -> {}
def default_exec_forge_api(name, args, stdin):
    endpoint = args[1]
    if "git/tags" in endpoint:
        return ('{"sha":"tagobjectsha000"}', None)
    if "git/refs" in endpoint:
        return ("{}", None)
    if "/commits/" in endpoint:
        return (json.dumps({"sha": SHA}), None)
    if endpoint == "repos/{owner}/{repo}":
        return ('{"default_branch":"main"}', None)
    return ("{}", None)


# ────────────────────────────────────────────────────────────────────────────
# Precondition 1 — clean working tree
# ────────────────────────────────────────────────────────────────────────────

def test_dirty_tree_aborts():
    deps, _, _, err = make_deps(git_responses={"status --porcelain": " M some/file.py\n"})
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "working tree is not clean" in err[0]


# ────────────────────────────────────────────────────────────────────────────
# Precondition 2 — default branch up to date with origin
# ────────────────────────────────────────────────────────────────────────────

def test_fetch_fails_aborts():
    deps, _, _, err = make_deps(git_errors={"fetch origin --prune": "could not connect"})
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "could not fetch origin" in err[0]


def test_local_main_stale_aborts():
    deps, _, _, err = make_deps(
        git_errors={"rev-parse -q --verify refs/heads/main": None},
        git_responses={
            "rev-parse -q --verify refs/heads/main": "",
            "rev-parse refs/heads/main": "stalesha000",
        },
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "not up to date with origin/main" in err[0]


def test_local_main_matches_origin_not_blocked():
    deps, _, _, err = make_deps(
        git_errors={"rev-parse -q --verify refs/heads/main": None},
        git_responses={
            "rev-parse -q --verify refs/heads/main": "",
            "rev-parse refs/heads/main": SHA,
        },
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 0, err


def test_no_local_main_not_blocked():
    deps, _, _, err = make_deps()
    code = run_release_tag(VERSION, **deps)
    assert code == 0, err


# ────────────────────────────────────────────────────────────────────────────
# Precondition 3 — the 4 version files must all match
# ────────────────────────────────────────────────────────────────────────────

def test_mismatched_go_version_names_the_file():
    deps, _, _, err = make_deps(
        file_overrides={"internal/version/version.go": 'package version\n\nvar Version = "0.0.1"\n'}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "internal/version/version.go" in err[0]
    assert '"0.0.1"' in err[0]
    assert VERSION in err[0]


def test_mismatched_npm_version_names_the_file():
    deps, _, _, err = make_deps(
        file_overrides={"npm/package.json": json.dumps({"name": "trackfw", "version": "0.0.1"})}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "npm/package.json" in err[0]


def test_mismatched_pyproject_version_names_the_file():
    deps, _, _, err = make_deps(
        file_overrides={"pypi/pyproject.toml": '[project]\nversion = "0.0.1"\n'}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "pypi/pyproject.toml" in err[0]


def test_mismatched_init_py_try_fallback_names_it():
    deps, _, _, err = make_deps(
        file_overrides={
            "pypi/trackfw/__init__.py": (
                "try:\n    from importlib.metadata import version\n"
                '    __version__ = version("trackfw") or "0.0.1"\n'
                f'except Exception:\n    __version__ = "{VERSION}"\n'
            )
        }
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "importlib.metadata fallback" in err[0]


def test_mismatched_init_py_except_fallback_names_it():
    deps, _, _, err = make_deps(
        file_overrides={
            "pypi/trackfw/__init__.py": (
                "try:\n    from importlib.metadata import version\n"
                f'    __version__ = version("trackfw") or "{VERSION}"\n'
                'except Exception:\n    __version__ = "0.0.1"\n'
            )
        }
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "except fallback" in err[0]


def test_v_prefix_arg_normalized_against_bare_file_versions():
    deps, _, _, err = make_deps()
    code = run_release_tag(f"v{VERSION}", **deps)
    assert code == 0, err


# ────────────────────────────────────────────────────────────────────────────
# Precondition 4 — CHANGELOG.md must have the version's section
# ────────────────────────────────────────────────────────────────────────────

def test_changelog_missing_section_aborts():
    deps, _, _, err = make_deps(
        file_overrides={"CHANGELOG.md": "# Changelog\n\n## [1.0.0] - 2020-01-01\n\n### Added\n- x\n"}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert VERSION in err[0]
    assert "not found in CHANGELOG.md" in err[0]


# ────────────────────────────────────────────────────────────────────────────
# Precondition 5 — tag must not already exist, local or remote
# ────────────────────────────────────────────────────────────────────────────

def test_local_tag_exists_aborts():
    deps, _, _, err = make_deps(
        git_errors={f"rev-parse -q --verify refs/tags/{TAG}": None},
        git_responses={f"rev-parse -q --verify refs/tags/{TAG}": SHA},
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert TAG in err[0]
    assert "already exists locally" in err[0]


def test_remote_tag_exists_aborts():
    deps, _, _, err = make_deps(
        git_responses={f"ls-remote --tags origin refs/tags/{TAG}": f"{SHA}\trefs/tags/{TAG}"}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert TAG in err[0]
    assert "already exists on origin" in err[0]


# ────────────────────────────────────────────────────────────────────────────
# Precondition 6 — forge CLI available, GitHub only
# ────────────────────────────────────────────────────────────────────────────

def test_no_forge_cli_aborts():
    deps, _, _, err = make_deps(avail_fn=lambda name: False)
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "requires the GitHub CLI (gh)" in err[0]
    assert f"git tag -a {TAG}" in err[0]


def test_unsupported_forge_aborts():
    deps, _, _, err = make_deps(
        git_responses={"remote get-url origin": "git@gitlab.com:kgsaran/trackfw.git"}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "currently only supports GitHub" in err[0]
    assert "gitlab" in err[0]


def test_manual_forge_aborts():
    deps, _, _, err = make_deps(
        git_responses={"remote get-url origin": "git@example.internal:kgsaran/trackfw.git"}
    )
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert 'resolved forge: "manual"' in err[0]


# ────────────────────────────────────────────────────────────────────────────
# Git identity
# ────────────────────────────────────────────────────────────────────────────

def test_no_git_identity_aborts():
    deps, _, _, err = make_deps(git_responses={"config user.name": ""})
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "git config user.name" in err[0]


# ────────────────────────────────────────────────────────────────────────────
# Success path — verifies the annotated-tag publish sequence
# ────────────────────────────────────────────────────────────────────────────

def test_success_publishes_annotated_tag():
    calls = []

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1] or "git/refs" in args[1]:
            calls.append((name, args, stdin))
        return default_exec_forge_api(name, args, stdin)

    deps, _, out, err = make_deps(exec_forge_api=exec_forge_api)
    code = run_release_tag(VERSION, **deps)
    assert code == 0, err
    assert len(calls) == 2

    name0, args0, body0 = calls[0]
    assert "git/tags" in args0[1]
    tag_payload = json.loads(body0)
    assert tag_payload["tag"] == TAG
    assert tag_payload["object"] == SHA
    assert tag_payload["type"] == "commit"
    assert VERSION in tag_payload["message"]
    assert tag_payload["tagger"]["name"] == "Test User"
    assert tag_payload["tagger"]["email"] == "test@example.com"

    name1, args1, body1 = calls[1]
    assert "git/refs" in args1[1]
    ref_payload = json.loads(body1)
    assert ref_payload["ref"] == f"refs/tags/{TAG}"
    assert ref_payload["sha"] == "tagobjectsha000"

    assert TAG in "\n".join(out)


def test_tag_object_call_failure_never_reaches_ref_call():
    ref_called = {"value": False}

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            return ("", "401 Unauthorized")
        if "git/refs" in args[1]:
            ref_called["value"] = True
        return default_exec_forge_api(name, args, stdin)

    deps, _, _, err = make_deps(exec_forge_api=exec_forge_api)
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert ref_called["value"] is False
    assert "gh api failed creating the tag object" in err[0]


# ────────────────────────────────────────────────────────────────────────────
# Commit target anchored on the forge (ADR-2026-08-19, Emenda 1) — the forge's
# default_branch/commit sha are authoritative; local refs are cross-checked only, never trusted.
# ────────────────────────────────────────────────────────────────────────────


def test_repointed_local_symref_is_neutralized_forge_branch_name_wins():
    # The local, symref-derived base resolves to "chore/other" (attacker-writable, purely
    # local), while the forge reports "main". The forge's branch name is authoritative
    # unconditionally — no local-vs-forge name comparison exists (a fresh/shallow clone
    # legitimately has no local opinion at all). The repoint is neutralized: publish uses the
    # forge's branch/sha, ignoring it.
    deps, _, _, _ = make_deps(
        git_responses={
            "symbolic-ref refs/remotes/origin/HEAD": "refs/remotes/origin/chore/other",
            "rev-parse origin/chore/other": "shaonchoreother00",
        },
        git_errors={"rev-parse -q --verify refs/heads/chore/other": "no such branch"},
    )
    tags_body = {"value": None}

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            tags_body["value"] = stdin
            return ('{"sha":"tagobjectsha000"}', None)
        return default_exec_forge_api(name, args, stdin)

    deps["exec_forge_api"] = exec_forge_api
    code = run_release_tag(VERSION, **deps)
    assert code == 0
    assert SHA in tags_body["value"]


def test_absent_local_symref_is_not_a_false_divergence():
    # No origin/HEAD symref at all — _default_base_branch falls back to "main". The forge's
    # real default branch is "master". There is no local opinion to disagree with the forge
    # here, just an absent one — must not refuse.
    deps, _, _, _ = make_deps(
        git_errors={
            "symbolic-ref refs/remotes/origin/HEAD": "not a symbolic ref",
            "rev-parse origin/main": "unknown revision",
        },
        git_responses={"rev-parse origin/master": SHA},
    )

    def exec_forge_api(name, args, stdin):
        if args[1] == "repos/{owner}/{repo}":
            return ('{"default_branch":"master"}', None)
        return default_exec_forge_api(name, args, stdin)

    deps["exec_forge_api"] = exec_forge_api
    code = run_release_tag(VERSION, **deps)
    assert code == 0


def test_forge_commit_diverges_refuses_naming_divergence():
    deps, _, _, err = make_deps(git_responses={"rev-parse origin/main": "forgedlocalsha000"})
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "forgedlocalsha000" in err[0]
    assert SHA in err[0]
    assert "diverges" in err[0]


def test_publish_always_uses_forge_sha_never_local():
    deps, _, _, _ = make_deps(git_errors={"rev-parse origin/main": "unknown revision"})
    tags_body = {"value": None}

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            tags_body["value"] = stdin
            return ('{"sha":"tagobjectsha000"}', None)
        return default_exec_forge_api(name, args, stdin)

    deps["exec_forge_api"] = exec_forge_api
    code = run_release_tag(VERSION, **deps)
    assert code == 0
    assert SHA in tags_body["value"]


# ────────────────────────────────────────────────────────────────────────────
# ML-2A: Object anchoring — P3/P4 read from the commit-target, not the
# working tree. See ADR-2026-08-21-release-tag-le-versao-e-changelog-do-
# commit-ancorado.md.
# ────────────────────────────────────────────────────────────────────────────


def test_object_absent_version_file_refuses_naming_sha_and_path():
    """When read_at_commit cannot find a version file, refuse naming path+sha, never publish."""
    deps, _, _, err = make_deps()
    publish_called = {"value": False}

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            publish_called["value"] = True
        return default_exec_forge_api(name, args, stdin)

    files = valid_files(VERSION)

    def read_at_commit(sha, path):
        if path == "internal/version/version.go":
            return ("", f"path '{path}' does not exist in '{sha}'")
        return (files[path], None) if path in files else ("", f"object {sha}:{path} not found")

    deps["exec_forge_api"] = exec_forge_api
    deps["read_at_commit"] = read_at_commit
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "internal/version/version.go" in err[0]
    assert SHA in err[0]
    assert "refuses to run" in err[0]
    assert not publish_called["value"]


def test_object_absent_changelog_refuses_naming_sha_and_path():
    """When read_at_commit cannot find CHANGELOG.md, refuse naming path+sha, never publish."""
    deps, _, _, err = make_deps()
    publish_called = {"value": False}

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            publish_called["value"] = True
        return default_exec_forge_api(name, args, stdin)

    files = valid_files(VERSION)

    def read_at_commit(sha, path):
        if path == "CHANGELOG.md":
            return ("", f"path '{path}' does not exist in '{sha}'")
        return (files[path], None) if path in files else ("", f"object {sha}:{path} not found")

    deps["exec_forge_api"] = exec_forge_api
    deps["read_at_commit"] = read_at_commit
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert "CHANGELOG.md" in err[0]
    assert SHA in err[0]
    assert "refuses to run" in err[0]
    assert not publish_called["value"]


def test_tag_message_sourced_from_commit_blob_not_hypothetical_local():
    """
    The tag payload message must come from read_at_commit (the commit-anchored blob), not from
    any hypothetical local source. read_at_commit delivers a CHANGELOG body with a unique
    discriminant line; the test asserts the payload contains it.
    """
    deps, _, _, err = make_deps()
    tags_body = {"value": None}

    def exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            tags_body["value"] = stdin
        return default_exec_forge_api(name, args, stdin)

    files = valid_files(VERSION)

    def read_at_commit(sha, path):
        if path == "CHANGELOG.md":
            return (
                f"# Changelog\n\n## [{VERSION}] - 2026-08-21\n\n### Added\n- from-commit-object-anchor\n",
                None,
            )
        return (files[path], None) if path in files else ("", f"object {sha}:{path} not found")

    deps["exec_forge_api"] = exec_forge_api
    deps["read_at_commit"] = read_at_commit
    code = run_release_tag(VERSION, **deps)
    assert code == 0, f"unexpected failure: {err}"
    assert "from-commit-object-anchor" in tags_body["value"]
    assert "from-working-tree-NOT-anchored" not in tags_body["value"]
