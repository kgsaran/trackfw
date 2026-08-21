"""
release/runner.py — Core implementation of `trackfw release tag`.

Port of internal/commands/release.go — keep in sync. See
ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md for why this exists as
a separate command from `ship`: tag is not a branch operation, and ship's governance gate
("REQ + roadmap in wip/") does not apply to release.

All git/gh operations are injectable for testability. Publishes via two `gh api` calls
(POST git/tags then POST git/refs) — the reference sequence validated in production for
v7.1.0 — which preserves the tag's annotation; a plain `git push origin <tag>` from a
lightweight local tag would lose it, and the git-branch-guard blocks that push form anyway.
"""

import json
import re
import subprocess
import sys
from datetime import datetime, timezone

from trackfw import changelog
from trackfw.forge.resolve import resolve as forge_resolve
from trackfw.forge.adapter import forge_adapter

# ─── Named refusal message builders ────────────────────────────────────────
# Kept byte-identical (by construction) to Go's releaseTag*Fmt constants
# (internal/commands/release.go) and Node's release/runner.js message builders, so the ML-2B
# parity gate can compare all 3 CLIs. Every precondition refusal names what to fix — release
# tag prefers refusing over guessing.


def _dirty_tree_msg(status_out):
    return (
        f"trackfw release tag refuses to run: working tree is not clean.\n{status_out}\n"
        "Commit your changes (trackfw commit) before tagging a release."
    )


def _fetch_failed_msg(err_message):
    return (
        f"trackfw release tag refuses to run: could not fetch origin ({err_message}). "
        "Check your network/credentials and retry."
    )


def _local_branch_stale_msg(base, local_sha, remote_sha):
    return (
        f'trackfw release tag refuses to run: local "{base}" is not up to date with '
        f"origin/{base} (local {local_sha}, remote {remote_sha}). Run: git pull"
    )


def _version_mismatch_msg(label, got, want):
    return (
        f'trackfw release tag refuses to run: {label} has version "{got}", expected "{want}". '
        "Update it to match before tagging."
    )


def _changelog_missing_msg(underlying_message, version):
    return (
        f"trackfw release tag refuses to run: {underlying_message}. "
        f'Add a "## [{version}] - YYYY-MM-DD" section to CHANGELOG.md before tagging.'
    )


def _exists_local_msg(tag_name):
    return (
        f'trackfw release tag refuses to run: tag "{tag_name}" already exists locally. '
        f"Delete it first (git tag -d {tag_name}) or choose a different version."
    )


def _exists_remote_msg(tag_name):
    return (
        f'trackfw release tag refuses to run: tag "{tag_name}" already exists on origin. '
        "Choose a different version."
    )


def _no_forge_cli_msg(tag_name, object_sha):
    return (
        "trackfw release tag requires the GitHub CLI (gh) to publish the tag. No forge CLI is "
        "available for this repository — install and authenticate gh, or push the tag "
        f'manually: git tag -a {tag_name} -m "<CHANGELOG.md section>" {object_sha} && '
        f"git push origin {tag_name}"
    )


def _unsupported_forge_msg(resolved_forge, tag_name, object_sha):
    return (
        f'trackfw release tag currently only supports GitHub (resolved forge: "{resolved_forge}"). '
        f"Publishing tag {tag_name} on this forge is not implemented yet — commit to tag: "
        f"{object_sha}. Create {tag_name} through your forge's web UI, or open an issue "
        "requesting support for this forge."
    )


NO_GIT_IDENTITY_MSG = (
    "trackfw release tag refuses to run: git config user.name and user.email must be set to "
    'create an annotated tag (git config user.name "Your Name" && '
    "git config user.email you@example.com)."
)


# _commit_diverges_msg fires when a LOCAL ref (origin/<forge's default branch>'s resolved sha)
# disagrees with what the forge itself reports for that same branch. This ref is writable inside
# the clone (git update-ref) — the forge is the only source that is not — so a disagreement is
# refused, never silently resolved by picking one side. The BRANCH NAME itself comes from the
# forge unconditionally (no local-vs-forge name check — see the call site) since a fresh/shallow
# clone legitimately has no local opinion on it at all. See ADR-2026-08-19-caminho-governado-
# para-push-forcado-e-tag-de-release.md, Emenda 1.
def _commit_diverges_msg(base, local_sha, forge_base, forge_sha):
    return (
        f"trackfw release tag refuses to run: local origin/{base} ({local_sha}) diverges from "
        f"the forge's {forge_base} tip ({forge_sha}). A local ref can be stale or forged — "
        "investigate before retrying: git fetch origin --prune"
    )


# _object_absent_msg fires when git show <sha>:<path> fails (object absent locally after the
# fetch that Precondition 2 already ran). Names the path and the sha so the user knows
# exactly what is missing. Never falls back to the working tree. See ADR-2026-08-21-
# release-tag-le-versao-e-changelog-do-commit-ancorado.md.
def _object_absent_msg(path, sha, err_message):
    return f"trackfw release tag refuses to run: could not read {path} at commit {sha}: {err_message}"


# ─── Version file extraction ───────────────────────────────────────────────

_GO_VERSION_RE = re.compile(r'Version\s*=\s*"([^"]+)"')
_PYPROJECT_VERSION_RE = re.compile(r'^version\s*=\s*"([^"]+)"', re.MULTILINE)
# Matches the try-block fallback in `__version__ = version("trackfw") or "7.1.0"`.
_INIT_TRY_VERSION_RE = re.compile(r'or\s+"([^"]+)"')
# Matches the except-block's `__version__ = "7.1.0"` — distinct from the try-block line, which
# never starts with `__version__ = "` directly (it starts with `__version__ = version(...)`).
_INIT_EXCEPT_VERSION_RE = re.compile(r'__version__\s*=\s*"([^"]+)"')


def _extract_go_version(content):
    m = _GO_VERSION_RE.search(content)
    if not m:
        raise ValueError('could not find Version = "..." in internal/version/version.go')
    return m.group(1)


def _extract_npm_version(content):
    try:
        pkg = json.loads(content)
    except json.JSONDecodeError as e:
        raise ValueError(f"could not parse npm/package.json: {e}")
    version = pkg.get("version", "")
    if not version:
        raise ValueError('npm/package.json has no "version" field')
    return version


def _extract_pyproject_version(content):
    m = _PYPROJECT_VERSION_RE.search(content)
    if not m:
        raise ValueError('could not find version = "..." in pypi/pyproject.toml')
    return m.group(1)


def _extract_init_try_version(content):
    m = _INIT_TRY_VERSION_RE.search(content)
    if not m:
        raise ValueError(
            "could not find the importlib.metadata fallback version in pypi/trackfw/__init__.py"
        )
    return m.group(1)


def _extract_init_except_version(content):
    m = _INIT_EXCEPT_VERSION_RE.search(content)
    if not m:
        raise ValueError(
            "could not find the except fallback version in pypi/trackfw/__init__.py"
        )
    return m.group(1)


RELEASE_VERSION_FILES = [
    ("internal/version/version.go", "internal/version/version.go", _extract_go_version),
    ("npm/package.json", "npm/package.json", _extract_npm_version),
    ("pypi/pyproject.toml", "pypi/pyproject.toml", _extract_pyproject_version),
    (
        "pypi/trackfw/__init__.py (importlib.metadata fallback)",
        "pypi/trackfw/__init__.py",
        _extract_init_try_version,
    ),
    (
        "pypi/trackfw/__init__.py (except fallback)",
        "pypi/trackfw/__init__.py",
        _extract_init_except_version,
    ),
]


def normalize_release_version(v):
    """Strips an optional leading "v"/"V"."""
    if v and v[0] in ("v", "V"):
        return v[1:]
    return v


# ─── Default dependency implementations ────────────────────────────────────


def default_exec_git(args):
    """Production git executor. Returns (stdout_str, error_str_or_None)."""
    try:
        result = subprocess.run(["git"] + args, capture_output=True, text=True)
        if result.returncode != 0:
            return (
                "",
                result.stderr.strip()
                or f"git {' '.join(args)} exited with {result.returncode}",
            )
        return (result.stdout.strip(), None)
    except FileNotFoundError:
        return ("", "git not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


def default_read_at_commit(sha, path):
    """
    Reads a file from a specific commit object (git show <sha>:<path>) and returns the content
    verbatim — stdout is NOT stripped because callers rely on byte-exact content (CHANGELOG
    sections, version strings with newlines). On any failure the error surfaces git's real
    stderr; there is NO fallback to the working tree. See ADR-2026-08-21-release-tag-le-
    versao-e-changelog-do-commit-ancorado.md.

    Returns (content_str, error_str_or_None).
    """
    try:
        result = subprocess.run(
            ["git", "--no-replace-objects", "show", f"{sha}:{path}"], capture_output=True, text=True
        )
        if result.returncode != 0:
            msg = result.stderr.strip() or f"git show {sha}:{path} exited with {result.returncode}"
            return ("", msg)
        # NOT stripped — content must be preserved byte-for-byte.
        return (result.stdout, None)
    except FileNotFoundError:
        return ("", "git not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


def default_exec_forge_api(name, args, stdin):
    """
    Runs a forge CLI command feeding stdin and capturing stdout, so the JSON response can be
    parsed. Returns (stdout_str, error_str_or_None). On failure, surfaces the CLI's real
    stderr text.
    """
    try:
        result = subprocess.run(
            [name] + args, input=stdin, capture_output=True, text=True
        )
        if result.returncode != 0:
            msg = result.stderr.strip() or f"{name} {' '.join(args)} exited with {result.returncode}"
            return (result.stdout.strip(), msg)
        return (result.stdout.strip(), None)
    except FileNotFoundError:
        return ("", f"{name} not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


# _GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX — see ship/runner.py's identical constant/rationale.
_GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX = "refs/remotes/origin/"


def _default_base_branch(exec_git):
    """
    Resolves the repository's default branch, mirroring Go's defaultBaseBranch (ship.go)
    exactly: tries symbolic-ref refs/remotes/origin/HEAD, falls back to "main". This is a
    LOCAL, gravable ref — release tag treats its result as a cross-check candidate only, never
    as the source of truth for the tag's commit target.
    """
    out, err = exec_git(["symbolic-ref", "refs/remotes/origin/HEAD"])
    if err:
        return "main"
    out = out.strip()
    if not out.startswith(_GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX):
        return "main"
    name = out[len(_GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX):]
    return name if name else "main"


def run_release_tag(
    version_arg,
    exec_git=None,
    read_at_commit=None,
    writeln=None,
    write_err=None,
    config_forge="",
    repo_dir=".",
    avail_fn=None,
    exec_forge_api=None,
):
    """
    Implements `trackfw release tag <version>`. Every precondition below is checked before any
    write — the risk this command carries is publishing a wrong tag to a public repository, so
    it always refuses rather than guesses.

    Returns
    -------
    int
        Exit code: 0 = success, 1 = failure.
    """
    if exec_git is None:
        exec_git = default_exec_git
    # read_at_commit reads a file from a specific commit object (git show <sha>:<path>).
    # Content is returned verbatim — NOT stripped. Absent object → error; no fallback to local.
    if read_at_commit is None:
        read_at_commit = default_read_at_commit
    if writeln is None:
        writeln = lambda s: print(s)  # noqa: E731
    if write_err is None:
        write_err = lambda s: print(f"Error: {s}", file=sys.stderr)  # noqa: E731
    if exec_forge_api is None:
        exec_forge_api = default_exec_forge_api

    version = normalize_release_version(str(version_arg).strip())
    tag_name = f"v{version}"

    # ─── Precondition 1: clean working tree ──────────────────────────────
    status_out, err = exec_git(["status", "--porcelain"])
    if err:
        write_err(f"could not determine working tree status: {err}")
        return 1
    if status_out.strip() != "":
        write_err(_dirty_tree_msg(status_out))
        return 1

    # ─── Precondition 2: default branch up to date with origin ──────────
    # base is symref-derived — a LOCAL, gravable ref. Used below only as (a) the value the
    # forge's default_branch must agree with, and (b) input to the local-branch-staleness
    # check, unrelated to the forge and unaffected by it.
    _, err = exec_git(["fetch", "origin", "--prune"])
    if err:
        write_err(_fetch_failed_msg(err))
        return 1

    base = _default_base_branch(exec_git)

    # local_sha (origin/<base>'s local tracking ref) is best-effort and non-fatal: a cross-check
    # candidate against the forge, never the source of the commit target. A failure to resolve
    # it must not block reaching the forge resolution below.
    local_sha = ""
    origin_sha, origin_err = exec_git(["rev-parse", f"origin/{base}"])
    if not origin_err:
        local_sha = origin_sha.strip()

        _, local_verify_err = exec_git(["rev-parse", "-q", "--verify", f"refs/heads/{base}"])
        if not local_verify_err:
            local_branch_sha, lerr = exec_git(["rev-parse", f"refs/heads/{base}"])
            if not lerr:
                local_branch_sha = local_branch_sha.strip()
                if local_branch_sha != local_sha:
                    write_err(_local_branch_stale_msg(base, local_branch_sha, local_sha))
                    return 1

    # ─── Precondition 5: tag must not already exist, local or remote ────
    _, local_tag_err = exec_git(["rev-parse", "-q", "--verify", f"refs/tags/{tag_name}"])
    if not local_tag_err:
        write_err(_exists_local_msg(tag_name))
        return 1
    remote_tag_out, _ = exec_git(["ls-remote", "--tags", "origin", f"refs/tags/{tag_name}"])
    if remote_tag_out.strip() != "":
        write_err(_exists_remote_msg(tag_name))
        return 1

    # ─── Precondition 6: forge CLI available — GitHub only, for now ─────
    remote_url, _ = exec_git(["remote", "get-url", "origin"])
    remote_url = (remote_url or "").strip()

    try:
        resolution = forge_resolve(
            config_forge=config_forge, remote_url=remote_url, repo_dir=repo_dir
        )
    except ValueError as e:
        write_err(str(e))
        return 1

    if resolution.forge != "github":
        # No forge to ask — local_sha is shown purely as an informational hint for the manual
        # fallback text below; the command never publishes on this path.
        write_err(_unsupported_forge_msg(resolution.forge, tag_name, local_sha))
        return 1

    adapter = forge_adapter(resolution.forge, avail_fn)
    if not adapter.available:
        # Same reasoning as above: no forge CLI to ask, local_sha is informational only.
        write_err(_no_forge_cli_msg(tag_name, local_sha))
        return 1

    # ─── The commit-target comes from the forge, never from a local ref ──
    # The forge's default_branch is authoritative for the BRANCH NAME — unconditionally, with no
    # refusal if it disagrees with the local symref-derived base (a fresh/shallow clone may have
    # no origin/HEAD symref at all, _default_base_branch then falls back to "main"; refusing on
    # that mismatch would be a false refusal against a legitimate repo, not a security check).
    # Only the forge's SHA is cross-checked against a local ref — resolved fresh, keyed to the
    # forge's own branch name, never to the (possibly-forged) local base. See ADR-2026-08-19-
    # caminho-governado-para-push-forcado-e-tag-de-release.md, Emenda 1.
    repo_info_resp, repo_info_err = exec_forge_api(
        "gh", ["api", "repos/{owner}/{repo}"], ""
    )
    if repo_info_err:
        write_err(
            "trackfw release tag: gh api failed resolving the repository's default branch "
            f"from the forge: {repo_info_err}"
        )
        return 1
    try:
        repo_info = json.loads(repo_info_resp)
    except (json.JSONDecodeError, TypeError):
        repo_info = {}
    default_branch = repo_info.get("default_branch") if isinstance(repo_info, dict) else None
    if not default_branch:
        write_err(
            "trackfw release tag: could not parse default_branch from the forge's "
            f"repository response: {repo_info_resp}"
        )
        return 1

    # forge_local_sha is resolved fresh against the forge's own branch name — deliberately NOT
    # reusing local_sha above, which was keyed to the symref-derived base and may name a
    # different branch (stale symref, or a fresh clone with no symref at all). Best-effort/
    # non-fatal, same reasoning as local_sha.
    forge_local_sha = ""
    forge_origin_sha, forge_origin_err = exec_git(["rev-parse", f"origin/{default_branch}"])
    if not forge_origin_err:
        forge_local_sha = forge_origin_sha.strip()

    commit_resp, commit_err = exec_forge_api(
        "gh", ["api", f"repos/{{owner}}/{{repo}}/commits/{default_branch}"], ""
    )
    if commit_err:
        write_err(
            "trackfw release tag: gh api failed resolving the forge's tip commit for "
            f"{default_branch}: {commit_err}"
        )
        return 1
    try:
        commit_obj = json.loads(commit_resp)
    except (json.JSONDecodeError, TypeError):
        commit_obj = {}
    forge_sha = commit_obj.get("sha") if isinstance(commit_obj, dict) else None
    if not forge_sha:
        write_err(
            "trackfw release tag: could not parse the forge's commit response for "
            f"{default_branch}: {commit_resp}"
        )
        return 1

    if forge_local_sha and forge_local_sha != forge_sha:
        write_err(_commit_diverges_msg(default_branch, forge_local_sha, default_branch, forge_sha))
        return 1

    # object_sha is now authoritative — resolved from the forge, cross-checked (not sourced)
    # against the local ref above.
    object_sha = forge_sha

    # ─── Precondition 3: version files in the commit-target must all match ─
    # Content is read from object_sha via git show, NOT from the working tree. Objects are
    # content-addressed: given a sha from the forge, the content is cyptographically determined —
    # a local edit that was not committed cannot influence the tag message.
    # Absent object → refuse naming sha+path; never fall back to local. See ADR-2026-08-21.
    for label, path, extract in RELEASE_VERSION_FILES:
        content, rerr = read_at_commit(object_sha, path)
        if rerr:
            write_err(_object_absent_msg(path, object_sha, rerr))
            return 1
        try:
            got = extract(content)
        except Exception as e:  # noqa: BLE001
            write_err(f"trackfw release tag refuses to run: {e}")
            return 1
        if got != version:
            write_err(_version_mismatch_msg(label, got, version))
            return 1

    # ─── Precondition 4: CHANGELOG.md in the commit-target has the version's section ──
    # Same anchoring as P3: content comes from object_sha, never from the working tree.
    changelog_content, changelog_err = read_at_commit(object_sha, "CHANGELOG.md")
    if changelog_err:
        write_err(_object_absent_msg("CHANGELOG.md", object_sha, changelog_err))
        return 1
    sections = changelog.parse_sections(changelog_content)
    try:
        section = changelog.find_version(sections, version)
    except ValueError as e:
        write_err(_changelog_missing_msg(str(e), version))
        return 1
    tag_message = changelog.format_section(section)

    # ─── Tagger identity ─────────────────────────────────────────────────
    name, _ = exec_git(["config", "user.name"])
    email, _ = exec_git(["config", "user.email"])
    name = (name or "").strip()
    email = (email or "").strip()
    if not name or not email:
        write_err(NO_GIT_IDENTITY_MSG)
        return 1

    # ─── Publish: two gh api calls, preserving the annotation ───────────
    tag_payload = json.dumps(
        {
            "tag": tag_name,
            "message": tag_message,
            "object": object_sha,
            "type": "commit",
            "tagger": {
                "name": name,
                "email": email,
                "date": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
        }
    )

    tag_resp, tag_err = exec_forge_api(
        "gh", ["api", "repos/{owner}/{repo}/git/tags", "--method", "POST", "--input", "-"], tag_payload
    )
    if tag_err:
        write_err(f"trackfw release tag: gh api failed creating the tag object: {tag_err}")
        return 1

    try:
        tag_obj = json.loads(tag_resp)
    except (json.JSONDecodeError, TypeError):
        tag_obj = {}
    tag_object_sha = tag_obj.get("sha") if isinstance(tag_obj, dict) else None
    if not tag_object_sha:
        write_err(
            f"trackfw release tag: could not parse the tag object response from gh api: {tag_resp}"
        )
        return 1

    ref_payload = json.dumps({"ref": f"refs/tags/{tag_name}", "sha": tag_object_sha})
    _, ref_err = exec_forge_api(
        "gh", ["api", "repos/{owner}/{repo}/git/refs", "--method", "POST", "--input", "-"], ref_payload
    )
    if ref_err:
        write_err(f"trackfw release tag: gh api failed creating the tag ref: {ref_err}")
        return 1

    writeln(f"Tag published: {tag_name}")
    writeln(f"  tag object: {tag_object_sha}")
    writeln(f"  commit:     {object_sha}")
    writeln("")
    writeln("release tag complete.")
    return 0
