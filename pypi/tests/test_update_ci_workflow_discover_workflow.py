"""
test_update_ci_workflow_discover_workflow.py — ML-2G (ROADMAP-2026-08-28-gate-de-ci-
pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version) — REQ-2026-08-28 AC17:
the doctor's remedy for a stale .github/workflows/trackfw-validate.yml (`trackfw
update`) was inert — no `update` target touched that file. `ci-workflow` now manages it
too, with four rules:
  (a) existence on disk (not cfg["ci"]) is the inclusion/refresh criterion — same
      criterion the doctor (ML-2F) already uses.
  (b) `update` NEVER creates the file for a project that doesn't have it.
  (c) the target is declared when EITHER cfg["ci"] opts into github-actions/gitlab-ci
      OR trackfw-validate.yml exists on disk.
  (d) idempotent — a second run with the same binary does not report "updated" again.

Mirrors internal/generators/update.go's TestUpdateCiWorkflow* (Go, canonical reference)
and npm/tests/update_ci_workflow_target_discover_workflow.test.js.
"""

import argparse
import contextlib
import io
import json
import os
import tempfile
import unittest

from trackfw import config as project_config
from trackfw.commands.discover import (
    DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
    build_discover_github_actions_workflow_content,
    install_gates,
)
from trackfw.commands.update import _run, _run_project
from trackfw.integrations.doctor import SCAFFOLD_DIVERGENT
from trackfw.integrations.scaffold_doctor import run_scaffold_doctor


def _write_trackfw_yaml(tmpdir: str, ci: str | None) -> None:
    lines = ['backend: go']
    if ci:
        lines.append(f'ci: {ci}')
    with open(os.path.join(tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines) + '\n')


def _empty_discover_result() -> dict:
    return {'hook_framework': 'none', 'ci_system': 'github-actions'}


def _workflow_path(tmpdir: str) -> str:
    return os.path.join(tmpdir, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)


def _run_update(tmpdir: str, install_missing: bool = False) -> dict:
    cwd = os.getcwd()
    os.chdir(tmpdir)
    try:
        args = argparse.Namespace(dry_run=False, json=True, targets=None, install_missing=install_missing)
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            _run_project(args)
        return json.loads(buf.getvalue())
    finally:
        os.chdir(cwd)


def _ci_target(payload: dict):
    return next((t for t in payload['targets'] if t['id'] == 'ci-workflow'), None)


def _run_bare_update(tmpdir: str) -> None:
    """Runs `_run` — the code path a bare `trackfw update` (no flags) actually
    takes, per `_dispatch` in trackfw.commands.update. `_run` never reads
    `args`, so a bare Namespace stands in for the CLI-parsed one. Distinct
    from `_run_update`/`_run_project` above (the --targets/--json path,
    ML-2G) — this is the path the doctor's printed remedy ("trackfw update")
    resolves to."""
    cwd = os.getcwd()
    os.chdir(tmpdir)
    try:
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            _run(argparse.Namespace())
    finally:
        os.chdir(cwd)


class TestCiWorkflowManagesDiscoverWorkflow(unittest.TestCase):
    # trackfw.config.load() is a per-process singleton (see trackfw/config.py) —
    # without resetting it between tests, a trackfw.yaml read by an earlier test
    # in this class would leak into the next one, since each test uses a fresh
    # tmpdir but the same cached config object.
    def setUp(self):
        project_config.reset()

    def tearDown(self):
        project_config.reset()

    def test_refreshes_stale_discover_workflow_even_with_ci_none(self):
        """AC17(a)/(c) — existence on disk is the criterion, even for ci: none."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            install_gates(_empty_discover_result(), tmpdir)
            stale = (
                'name: trackfw validate\non: [push, pull_request]\njobs:\n'
                '  governance:\n    runs-on: ubuntu-latest\n    steps:\n'
                '      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n'
                '      - run: trackfw validate\n'
            )
            with open(_workflow_path(tmpdir), 'w', encoding='utf-8') as f:
                f.write(stale)

            payload = _run_update(tmpdir)
            target = _ci_target(payload)
            self.assertIsNotNone(target, 'ci-workflow must be declared for ci:none with an existing trackfw-validate.yml')
            self.assertEqual(target['state'], 'updated')

            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                self.assertEqual(f.read(), build_discover_github_actions_workflow_content())

    def test_never_creates_discover_workflow_for_project_that_never_had_it(self):
        """AC17(b) — even with ci: github-actions and --install-missing, `update` must
        not create trackfw-validate.yml if it was never installed by discover."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')

            payload = _run_update(tmpdir, install_missing=True)
            target = _ci_target(payload)
            self.assertIsNotNone(target, 'ci-workflow must be declared for ci: github-actions')
            self.assertFalse(
                os.path.isfile(_workflow_path(tmpdir)),
                'AC17(b) violated: trackfw update created trackfw-validate.yml for a project that never had it',
            )

    def test_not_declared_without_ci_or_discover_workflow(self):
        """AC17(c) negative control — ci: none and no trackfw-validate.yml on disk."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            payload = _run_update(tmpdir)
            self.assertIsNone(_ci_target(payload))

    def test_idempotent(self):
        """AC17(d) — a second run with the same binary reports skipped, not updated."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            install_gates(_empty_discover_result(), tmpdir)

            first = _ci_target(_run_update(tmpdir))
            self.assertEqual(first['state'], 'skipped', 'install_gates already wrote the current template')

            second = _ci_target(_run_update(tmpdir))
            self.assertEqual(second['state'], 'skipped')

    def test_closes_doctor_finding_end_to_end(self):
        """End-to-end proof the remedy stopped being inert: a stale
        trackfw-validate.yml produces a scaffold-divergent doctor finding; after
        `trackfw update` (the exact remedy doctor prints — the BARE command, no
        flags, which `_dispatch` routes to `_run`, not `_run_project`) the same
        project reports no mismatches for that path.

        ML-2G proved this same claim through `_run_project` (the --targets/
        --json path) instead and declared the remedy closed without ever
        exercising `_run` — the path `trackfw update` with no flags, and the
        path the doctor's own remedy text points at, actually takes. That gap
        is why the bare command stayed inert after ML-2G shipped; asserting
        through `_run_bare_update` here is what makes this test fail again if
        that regresses."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            install_gates(_empty_discover_result(), tmpdir)
            stale = (
                'name: trackfw validate\non: [push, pull_request]\njobs:\n'
                '  governance:\n    runs-on: ubuntu-latest\n    steps:\n'
                '      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n'
                '      - run: trackfw validate\n'
            )
            with open(_workflow_path(tmpdir), 'w', encoding='utf-8') as f:
                f.write(stale)

            before = run_scaffold_doctor(tmpdir)
            before_finding = next(
                (f for f in before if f['destination'] == DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH), None
            )
            self.assertIsNotNone(before_finding, 'doctor did not flag the stale workflow before update')
            self.assertEqual(before_finding['finding'], SCAFFOLD_DIVERGENT)

            _run_bare_update(tmpdir)

            after = run_scaffold_doctor(tmpdir)
            after_finding = next(
                (f for f in after if f['destination'] == DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH), None
            )
            self.assertIsNone(after_finding, 'doctor still flags trackfw-validate.yml after trackfw update')


class TestBareUpdateManagesDiscoverWorkflow(unittest.TestCase):
    """AC17 through the bare `trackfw update` path (`_run`), not the
    --targets/--json path (`_run_project`, covered above). ML-2G's fix only
    reached `_run_project`; these are the tests that would have caught it."""

    def setUp(self):
        project_config.reset()

    def tearDown(self):
        project_config.reset()

    def test_bare_update_refreshes_stale_discover_workflow_even_with_ci_none(self):
        """AC17(a)/(c) via the bare command."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            install_gates(_empty_discover_result(), tmpdir)
            stale = (
                'name: trackfw validate\non: [push, pull_request]\njobs:\n'
                '  governance:\n    runs-on: ubuntu-latest\n    steps:\n'
                '      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n'
                '      - run: trackfw validate\n'
            )
            with open(_workflow_path(tmpdir), 'w', encoding='utf-8') as f:
                f.write(stale)

            _run_bare_update(tmpdir)

            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                self.assertEqual(f.read(), build_discover_github_actions_workflow_content())

    def test_bare_update_never_creates_discover_workflow_for_project_that_never_had_it(self):
        """AC17(b) via the bare command — the armadilha case: trackfw-gate.yml
        present, trackfw-validate.yml absent, must stay absent."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')

            _run_bare_update(tmpdir)

            self.assertFalse(
                os.path.isfile(_workflow_path(tmpdir)),
                'AC17(b) violated: bare trackfw update created trackfw-validate.yml for a project that never had it',
            )

    def test_bare_update_is_idempotent(self):
        """AC17(d) via the bare command — running twice with the same binary
        must not change the file on the second run."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            install_gates(_empty_discover_result(), tmpdir)

            _run_bare_update(tmpdir)
            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                first = f.read()

            _run_bare_update(tmpdir)
            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                second = f.read()

            self.assertEqual(first, second)


if __name__ == '__main__':
    unittest.main()
