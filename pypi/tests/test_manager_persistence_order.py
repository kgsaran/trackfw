"""ADR-2026-08-18: install/update persist the manifest before writing
artifact bytes; uninstall keeps bytes-before-manifest. This file proves both
halves with executed evidence, mirroring
internal/integrations/manager_persistence_order_test.go:

1. test_install_interrupted_after_manifest_persist_self_heals — genuinely
   interrupts a subprocess at the ADR seam (manager._after_manifest_persist)
   with os._exit (bypassing the finally-based rollback entirely, like
   SIGKILL/power loss would) and proves the resulting disk state is
   manifest-ahead-of-disk ("not-installed") and that a plain install (no
   --force) repairs it. This is also the P4 falsification scenario for
   ML-1A.
2. test_update_write_phase_failure_rolls_back_manifest_and_bytes — proves
   the rollback still restores both manifest and artifact bytes to a
   non-empty pre-batch baseline when a normal error (not a crash) happens in
   the write phase, which now runs after the manifest is persisted.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import pytest

from trackfw.integrations.catalog import plan_deployments
from trackfw.integrations.manager import IntegrationError, IntegrationManager

PYPI_ROOT = Path(__file__).parents[1]


def test_install_interrupted_after_manifest_persist_self_heals(tmp_path):
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()

    script = (
        "import os, sys\n"
        "sys.path.insert(0, sys.argv[3])\n"
        "from trackfw.integrations.catalog import plan_deployments\n"
        "from trackfw.integrations.manager import IntegrationManager\n"
        "manager = IntegrationManager(sys.argv[1], sys.argv[2])\n"
        "manager._after_manifest_persist = lambda: os._exit(7)\n"
        "_, plans = plan_deployments('agents', ['claude'], ['backend'], 'project')\n"
        "manager.install(plans)\n"
        "os._exit(0)\n"
    )
    result = subprocess.run(
        [sys.executable, "-c", script, str(project), str(home), str(PYPI_ROOT)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 7, f"stdout={result.stdout!r} stderr={result.stderr!r}"

    _, plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    plan = plans[0]
    destination = project / plan["destination"]

    # Baseline reproduced for real: artifact bytes absent...
    assert not destination.exists(), "artifact bytes should be absent after the interrupted write"
    # ...while the manifest already declares it (manifest-first ordering).
    manifest = json.loads((project / ".trackfw/integrations-manifest.json").read_text())
    assert str(destination) in manifest["artifacts"]
    assert manifest["artifacts"][str(destination)]["sha256"]

    manager = IntegrationManager(project, home)
    # Detection: "not-installed" (self-repairable), never "modified"/unmanaged.
    assert manager.inspect(plan)["state"] == "not-installed"
    # Self-repair: a later install, with NO force, succeeds and reaches "current".
    manager.install(plans)
    assert manager.inspect(plan)["state"] == "current"


def test_update_write_phase_failure_rolls_back_manifest_and_bytes(tmp_path):
    manager = IntegrationManager(tmp_path)
    _, plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    manager.install(plans)

    plan = plans[0]
    destination = tmp_path / plan["destination"]
    manifest_path = tmp_path / ".trackfw/integrations-manifest.json"
    baseline_bytes = destination.read_bytes()
    baseline_manifest = manifest_path.read_text()

    updated = [{**plan, "content": plan["content"] + b"\nupdated\n", "catalog_version": "9.9.9"}]

    # Force the write phase (which now runs after the manifest has already
    # been persisted) to fail only for the artifact's bytes, not the
    # manifest — proving the failure happens post-manifest-persist, exactly
    # the case the inversion newly makes possible.
    real_write = manager._atomic_write

    def failing_write(target, content, mode):
        if target == destination:
            raise IntegrationError("injected artifact write failure")
        real_write(target, content, mode)

    manager._atomic_write = failing_write
    try:
        with pytest.raises(IntegrationError, match="injected artifact write failure"):
            manager.update(updated)
    finally:
        manager._atomic_write = real_write

    assert destination.read_bytes() == baseline_bytes, "rollback did not restore artifact bytes"
    assert manifest_path.read_text() == baseline_manifest, "rollback did not restore manifest bytes"
    assert manager.inspect(plan)["state"] == "current"


def test_update_batch_rollback_restores_an_already_written_artifact(tmp_path):
    """Mirrors the Go/Node test of the same intent. Closes a gap the
    single-artifact test above cannot: because the injected failure targets
    the ONLY artifact in that batch, its write never lands in the first
    place, so "bytes equal baseline" holds trivially — nothing was ever
    overwritten. Here a batch of two artifacts updates both: the first
    artifact's write succeeds (its bytes really do change to v2 before the
    batch fails), the second artifact's write fails. Rollback must then
    genuinely revert the first artifact's bytes back to v1 — proving the
    restore actually restores. This is also what exercises the per-item
    try/except fix in the rollback loop (manager.py): without it, restoring
    artifact A after artifact B's restore had already failed would never be
    reached.
    """
    manager = IntegrationManager(tmp_path)
    _, plans_a = plan_deployments("agents", ["claude"], ["architect"], "project")
    _, plans_b = plan_deployments("agents", ["claude"], ["backend"], "project")
    plan_a, plan_b = plans_a[0], plans_b[0]
    manager.install([plan_a, plan_b])

    destination_a = tmp_path / plan_a["destination"]
    destination_b = tmp_path / plan_b["destination"]
    manifest_path = tmp_path / ".trackfw/integrations-manifest.json"
    baseline_a = destination_a.read_bytes()
    baseline_b = destination_b.read_bytes()
    baseline_manifest = manifest_path.read_text()

    updated_a = {**plan_a, "content": plan_a["content"] + b"\nupdated-a\n", "catalog_version": "9.9.9"}
    updated_b = {**plan_b, "content": plan_b["content"] + b"\nupdated-b\n", "catalog_version": "9.9.9"}

    real_write = manager._atomic_write

    def failing_write(target, content, mode):
        if target == destination_b:
            raise IntegrationError("injected artifact B write failure")
        real_write(target, content, mode)

    manager._atomic_write = failing_write
    try:
        with pytest.raises(IntegrationError, match="injected artifact B write failure"):
            manager.update([updated_a, updated_b])
    finally:
        manager._atomic_write = real_write

    assert destination_a.read_bytes() == baseline_a, "rollback did not restore already-written artifact A bytes"
    assert destination_b.read_bytes() == baseline_b, "artifact B bytes changed even though its write failed"
    assert manifest_path.read_text() == baseline_manifest, "rollback did not restore manifest bytes"
    assert manager.inspect(plan_a)["state"] == "current"
    assert manager.inspect(plan_b)["state"] == "current"
