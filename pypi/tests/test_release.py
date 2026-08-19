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

    def read_file(path):
        if path not in files:
            raise FileNotFoundError(f"file not found: {path}")
        return files[path]

    def default_exec_forge_api(name, args, stdin):
        if "git/tags" in args[1]:
            return ('{"sha":"tagobjectsha000"}', None)
        return ("{}", None)

    deps = dict(
        exec_git=git.exec,
        read_file=read_file,
        writeln=out_lines.append,
        write_err=err_lines.append,
        config_forge="",
        repo_dir="",
        avail_fn=avail_fn if avail_fn is not None else (lambda name: True),
        exec_forge_api=exec_forge_api if exec_forge_api is not None else default_exec_forge_api,
    )
    return deps, git, out_lines, err_lines


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
        calls.append((name, args, stdin))
        if "git/tags" in args[1]:
            return ('{"sha":"tagobjectsha000"}', None)
        return ("{}", None)

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
        ref_called["value"] = True
        return ("{}", None)

    deps, _, _, err = make_deps(exec_forge_api=exec_forge_api)
    code = run_release_tag(VERSION, **deps)
    assert code == 1
    assert ref_called["value"] is False
    assert "gh api failed creating the tag object" in err[0]
