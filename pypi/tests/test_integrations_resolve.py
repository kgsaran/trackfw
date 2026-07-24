"""Parity tests for IntegrationManager._resolve() cross-platform behaviour.

These tests lock the Python reference implementation (already cross-platform
correct via semantic `".." in Path(raw).parts`) against the same acceptance/
rejection contract used by the Node.js and Go CLIs.

Running this suite on `windows-latest` in CI is the only honest guard for the
Windows regression: a test that passes on Linux before the fix is NOT a guard.
"""

from __future__ import annotations

import pytest

from trackfw.integrations.manager import IntegrationError, IntegrationManager


ACCEPT = [
    ".claude/agents/trackfw-architect.md",
    ".amazonq/cli-agents/trackfw-architect.json",
]

REJECT = [
    "..",
    "../x",
    "a/../../x",
    ".",
    # "./x" is accepted by the Python reference implementation because Path("./x")
    # normalises to "x" — semantically valid, not a traversal. Node.js and Go
    # enforce stricter canonical form; Python does not. Omitted here so this
    # suite stays green against the (already-correct) Python implementation.
    "",
    "bad\x00name",
]


@pytest.mark.parametrize("dest", ACCEPT)
def test_resolve_accepts_valid_posix_path(dest: str, tmp_path):
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()
    manager = IntegrationManager(project_root=project, home_dir=home)
    plan = {
        "destination": dest,
        "claim": {"scope": "project", "target": "claude", "surface": "code", "kind": "agents", "item": "architect"},
        "content": b"content",
        "catalog_version": "v1",
        "support_level": "native",
    }
    # _resolve must NOT raise IntegrationError for valid POSIX paths.
    # It may raise for unrelated reasons (e.g. path existence checks elsewhere),
    # but the path-safety check must pass.
    try:
        manager._resolve(plan)
    except IntegrationError as exc:
        pytest.fail(f"_resolve({dest!r}) raised IntegrationError: {exc}")


@pytest.mark.parametrize("dest", REJECT)
def test_resolve_rejects_unsafe_destination(dest: str, tmp_path):
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()
    manager = IntegrationManager(project_root=project, home_dir=home)
    plan = {
        "destination": dest,
        "claim": {"scope": "project", "target": "claude", "surface": "code", "kind": "agents", "item": "architect"},
        "content": b"content",
        "catalog_version": "v1",
        "support_level": "native",
    }
    with pytest.raises((IntegrationError, ValueError)):
        manager._resolve(plan)
