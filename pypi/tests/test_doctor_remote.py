"""Mirrors internal/commands/doctor_remote_test.go and npm/tests/doctor_remote.test.js — same
scenarios, same non-vacuity guard (asserting the fake was actually called with the expected gh
subcommand, not just inspecting the resulting findings).
"""

from __future__ import annotations

from trackfw.commands.doctor_remote import run_doctor_remote
from trackfw.integrations.doctor import (
    ENFORCE_ADMINS_DISABLED,
    HOOKS_PATH_NEUTRALIZED,
    NOT_EVALUATED,
    REQUIRED_STATUS_CHECKS_MISSING,
)


def _fake_exec(responses=None, errors=None):
    responses = responses or {}
    errors = errors or {}
    calls = []

    def exec_forge_api(name, args, stdin):
        key = " ".join(args)
        calls.append(key)
        if key in errors:
            return ("", errors[key])
        if key in responses:
            return (responses[key], None)
        return ("{}", None)

    exec_forge_api.calls = calls
    return exec_forge_api


def _base_kwargs(hooks_path="", hooks_path_err="not set", exec_forge_api=None, avail_gh=True):
    def exec_git(args):
        joined = " ".join(args)
        if joined == "config --get core.hooksPath":
            if hooks_path_err:
                return ("", hooks_path_err)
            return (hooks_path, None)
        if joined == "remote get-url origin":
            return ("https://github.com/kgsaran/trackfw.git", None)
        return ("", None)

    return dict(
        exec_git=exec_git,
        exec_forge_api=exec_forge_api,
        avail_fn=lambda name: avail_gh if name == "gh" else False,
        config_forge="",
        repo_dir="",
    )


def _has_kind(findings, kind):
    return any(f["finding"] == kind for f in findings)


# -- Falsification direction (a): repo WITHOUT required_status_checks -> finding -----------


def test_no_protection_produces_findings():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":true}}',
        },
        errors={
            "api repos/{owner}/{repo}/branches/main/protection": "gh: Branch not protected (HTTP 404)",
        },
    )
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api))
    assert _has_kind(findings, REQUIRED_STATUS_CHECKS_MISSING)
    assert _has_kind(findings, ENFORCE_ADMINS_DISABLED)
    assert not _has_kind(findings, NOT_EVALUATED)
    assert "api repos/{owner}/{repo}/branches/main/protection" in exec_forge_api.calls


# -- Falsification direction (b), the CONTROL: repo WITH the gate configured -> no finding -


def test_protection_configured_zero_findings_control():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":true}}',
            "api repos/{owner}/{repo}/branches/main/protection": (
                '{"required_status_checks":{"strict":true,"contexts":["governance-go-install"]},'
                '"enforce_admins":{"enabled":true}}'
            ),
        }
    )
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api))
    assert findings == []
    assert "api repos/{owner}/{repo}/branches/main/protection" in exec_forge_api.calls


def test_protection_configured_via_checks_field_zero_findings():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":true}}',
            "api repos/{owner}/{repo}/branches/main/protection": (
                '{"required_status_checks":{"contexts":[],"checks":[{"context":"governance-go-install"}]},'
                '"enforce_admins":{"enabled":true}}'
            ),
        }
    )
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api))
    assert findings == []


# -- The case that decides the ADR: --remote with no credential -> not-evaluated, never ok --


def test_no_credential_not_evaluated_never_ok():
    exec_forge_api = _fake_exec(errors={"auth status": "gh: not logged into any GitHub hosts"})
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api))
    assert len(findings) == 1
    assert findings[0]["finding"] == NOT_EVALUATED
    assert "authenticate" in findings[0]["remedy"]
    assert not any(c.startswith("api repos/{owner}/{repo}/branches/") for c in exec_forge_api.calls)


# -- Token present but insufficient scope -- DISTINCT message from no-credential -----------


def test_insufficient_scope_distinct_from_no_credential():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":false}}',
        }
    )
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api))
    assert len(findings) == 1
    assert findings[0]["finding"] == NOT_EVALUATED
    assert "authenticate first" not in findings[0]["remedy"]
    assert "admin access" in findings[0]["remedy"]
    assert not any(c.startswith("api repos/{owner}/{repo}/branches/") for c in exec_forge_api.calls)


def test_no_gh_cli_not_evaluated():
    exec_forge_api = _fake_exec()
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api, avail_gh=False))
    assert len(findings) == 1
    assert findings[0]["finding"] == NOT_EVALUATED
    assert exec_forge_api.calls == []


def test_non_github_forge_not_evaluated():
    exec_forge_api = _fake_exec()
    kwargs = _base_kwargs(exec_forge_api=exec_forge_api)

    def exec_git(args):
        if " ".join(args) == "remote get-url origin":
            return ("git@gitlab.com:kgsaran/trackfw.git", None)
        return ("", "not set")

    kwargs["exec_git"] = exec_git
    findings = run_doctor_remote(**kwargs)
    assert len(findings) == 1
    assert findings[0]["finding"] == NOT_EVALUATED
    assert exec_forge_api.calls == []


# -- hooksPath: falsification in both directions --------------------------------------------


def test_hooks_path_neutralized_produces_finding():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":true}}',
            "api repos/{owner}/{repo}/branches/main/protection": '{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}',
        }
    )
    findings = run_doctor_remote(**_base_kwargs(hooks_path="/dev/null", hooks_path_err="", exec_forge_api=exec_forge_api))
    assert _has_kind(findings, HOOKS_PATH_NEUTRALIZED)


def test_hooks_path_unset_no_finding_control():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":true}}',
            "api repos/{owner}/{repo}/branches/main/protection": '{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}',
        }
    )
    findings = run_doctor_remote(**_base_kwargs(exec_forge_api=exec_forge_api))
    assert not _has_kind(findings, HOOKS_PATH_NEUTRALIZED)


def test_hooks_path_custom_husky_no_finding():
    exec_forge_api = _fake_exec(
        responses={
            "auth status": "logged in",
            "api repos/{owner}/{repo}": '{"default_branch":"main","permissions":{"admin":true}}',
            "api repos/{owner}/{repo}/branches/main/protection": '{"required_status_checks":{"contexts":["x"]},"enforce_admins":{"enabled":true}}',
        }
    )
    findings = run_doctor_remote(**_base_kwargs(hooks_path=".husky/_", hooks_path_err="", exec_forge_api=exec_forge_api))
    assert not _has_kind(findings, HOOKS_PATH_NEUTRALIZED)
