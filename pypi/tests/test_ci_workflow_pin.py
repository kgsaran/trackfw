"""
test_ci_workflow_pin.py — ML-2C (REQ-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-
install-sh-honrando-trackfw-version): the Python CLI's CI workflow builders, `update`
target, and scaffold doctor coverage.

Mirrors the Go/Node scaffold_doctor tests for the same REQ's AC6/AC7/AC9/AC10/AC11.
"""

import os
import tempfile
import unittest

from trackfw import __version__ as TRACKFW_VERSION
from trackfw import config as project_config
from trackfw.generators.init_gen import (
    GITHUB_ACTIONS_WORKFLOW_PATH,
    GITLAB_CI_WORKFLOW_PATH,
    build_github_actions_workflow_content,
    build_gitlab_ci_workflow_content,
    generate_ci_workflow,
)
from trackfw.commands.update import CI_WORKFLOW_RELATIVE_PATHS, project_target_ids
from trackfw.integrations.scaffold_doctor import (
    _check_ci_workflow_artifact,
    run_scaffold_doctor,
)
from trackfw.integrations.doctor import SCAFFOLD_DIVERGENT, SCAFFOLD_MISSING


def _write_trackfw_yaml(tmpdir: str, ci: str | None) -> None:
    lines = [
        'req_dir: docs/req',
        'roadmap_dir: docs/roadmaps',
        'roadmap_namespacing: flat',
        'wip_limit: 1',
    ]
    if ci:
        lines.append(f'ci: {ci}')
    with open(os.path.join(tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines) + '\n')


class TestCIWorkflowBuilders(unittest.TestCase):
    """AC6/AC7 — the workflow contains exactly trackfw.__version__, never a literal."""

    def test_github_actions_content_pins_current_version(self):
        content = build_github_actions_workflow_content(None)
        self.assertIn(f'TRACKFW_VERSION: "{TRACKFW_VERSION}"', content)
        self.assertIn('timeout-minutes: 10', content)

    def test_gitlab_ci_content_pins_current_version(self):
        content = build_gitlab_ci_workflow_content(None)
        self.assertIn(f'TRACKFW_VERSION: "{TRACKFW_VERSION}"', content)
        self.assertIn('timeout: 10 minutes', content)

    def test_no_hardcoded_version_literal(self):
        """The pin must come from trackfw.__version__, not a literal string baked
        into the builder. Falsifiable form: the builder's source must reference the
        __version__ symbol AND must not contain the current version as a bare
        string literal (which a hardcoded pin masquerading as '__version__' in a
        comment would still satisfy if we only checked for the substring)."""
        import inspect
        from trackfw.generators import init_gen

        src_gh = inspect.getsource(init_gen.build_github_actions_workflow_content)
        src_gl = inspect.getsource(init_gen.build_gitlab_ci_workflow_content)
        self.assertIn('__version__', src_gh)
        self.assertIn('__version__', src_gl)
        self.assertNotIn(TRACKFW_VERSION, src_gh)
        self.assertNotIn(TRACKFW_VERSION, src_gl)

    def test_idempotent(self):
        self.assertEqual(
            build_github_actions_workflow_content(None),
            build_github_actions_workflow_content(None),
        )
        self.assertEqual(
            build_gitlab_ci_workflow_content(None),
            build_gitlab_ci_workflow_content(None),
        )


class TestGenerateCIWorkflow(unittest.TestCase):
    def test_writes_github_actions_workflow(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            generate_ci_workflow(tmpdir, {'ci': 'github-actions'})
            dest = os.path.join(tmpdir, GITHUB_ACTIONS_WORKFLOW_PATH)
            self.assertTrue(os.path.isfile(dest))
            with open(dest, encoding='utf-8') as f:
                self.assertEqual(f.read(), build_github_actions_workflow_content(None))

    def test_writes_gitlab_ci_workflow(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            generate_ci_workflow(tmpdir, {'ci': 'gitlab-ci'})
            dest = os.path.join(tmpdir, GITLAB_CI_WORKFLOW_PATH)
            self.assertTrue(os.path.isfile(dest))
            with open(dest, encoding='utf-8') as f:
                self.assertEqual(f.read(), build_gitlab_ci_workflow_content(None))

    def test_noop_without_ci(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            generate_ci_workflow(tmpdir, {})
            self.assertFalse(os.path.isdir(os.path.join(tmpdir, '.github')))
            self.assertFalse(os.path.isfile(os.path.join(tmpdir, GITLAB_CI_WORKFLOW_PATH)))


class TestProjectTargetIds(unittest.TestCase):
    """ci-workflow appears in the same relative position as Go/Node: right after
    validate-script, before claude-commands."""

    def test_ci_workflow_absent_without_ci(self):
        ids = project_target_ids({})
        self.assertNotIn('ci-workflow', ids)

    def test_ci_workflow_present_and_positioned(self):
        for ci_value in ('github-actions', 'gitlab-ci'):
            ids = project_target_ids({'ci': ci_value})
            self.assertIn('ci-workflow', ids)
            self.assertEqual(
                ids.index('ci-workflow'),
                ids.index('validate-script') + 1,
                f'ci-workflow must immediately follow validate-script (ci={ci_value})',
            )
            self.assertLess(ids.index('ci-workflow'), ids.index('claude-commands'))


class TestDoctorCIWorkflow(unittest.TestCase):
    """AC10/AC11 — scaffold doctor detects a stale pin and accepts a fresh one."""

    def test_no_mismatches_right_after_generation(self):
        """AC11 — doctor reports nothing for a workflow just generated by this binary."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')
            generate_ci_workflow(tmpdir, {'ci': 'github-actions'})
            finding = _check_ci_workflow_artifact(tmpdir, {'ci': 'github-actions'})
            self.assertIsNone(finding)

            findings = run_scaffold_doctor(tmpdir)
            ci_findings = [f for f in findings if f['destination'] == GITHUB_ACTIONS_WORKFLOW_PATH]
            self.assertEqual(ci_findings, [])

    def test_scaffold_divergent_with_pin_swapped_by_hand(self):
        """AC10 — a hand-edited (older/different) pin is reported scaffold-divergent."""
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')
            generate_ci_workflow(tmpdir, {'ci': 'github-actions'})
            dest = os.path.join(tmpdir, GITHUB_ACTIONS_WORKFLOW_PATH)
            with open(dest, encoding='utf-8') as f:
                content = f.read()
            swapped = content.replace(f'"{TRACKFW_VERSION}"', '"0.0.1"')
            self.assertNotEqual(content, swapped, 'sanity: the replace must actually change something')
            with open(dest, 'w', encoding='utf-8') as f:
                f.write(swapped)

            finding = _check_ci_workflow_artifact(tmpdir, {'ci': 'github-actions'})
            self.assertIsNotNone(finding)
            self.assertEqual(finding['finding'], SCAFFOLD_DIVERGENT)
            self.assertEqual(finding['destination'], GITHUB_ACTIONS_WORKFLOW_PATH)

    def test_missing_workflow_reported(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'gitlab-ci')
            finding = _check_ci_workflow_artifact(tmpdir, {'ci': 'gitlab-ci'})
            self.assertIsNotNone(finding)
            self.assertEqual(finding['finding'], SCAFFOLD_MISSING)
            self.assertEqual(finding['destination'], GITLAB_CI_WORKFLOW_PATH)

    def test_no_ci_configured_is_a_noop(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            finding = _check_ci_workflow_artifact(tmpdir, {'ci': None})
            self.assertIsNone(finding)


class TestUpdateBumpsPin(unittest.TestCase):
    """AC9 — a project with an old pin gets bumped and reported `updated` by
    `trackfw update`."""

    def test_update_project_reports_updated_and_rewrites_pin(self):
        from trackfw.commands.update import (
            _run_file_target,
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')
            dest_dir = os.path.join(tmpdir, '.github', 'workflows')
            os.makedirs(dest_dir, exist_ok=True)
            stale = build_github_actions_workflow_content(None).replace(
                f'"{TRACKFW_VERSION}"', '"0.0.1"'
            )
            with open(os.path.join(dest_dir, 'trackfw-gate.yml'), 'w', encoding='utf-8') as f:
                f.write(stale)

            result = _run_file_target(
                'ci-workflow',
                'ci-workflow-display-path',
                tmpdir,
                CI_WORKFLOW_RELATIVE_PATHS,
                lambda root: generate_ci_workflow(root, {'ci': 'github-actions'}),
                False,
                False,
            )
            self.assertEqual(result['state'], 'updated')

            with open(os.path.join(dest_dir, 'trackfw-gate.yml'), encoding='utf-8') as f:
                self.assertIn(f'"{TRACKFW_VERSION}"', f.read())

    def test_update_project_skips_when_already_current(self):
        from trackfw.commands.update import _run_file_target

        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')
            generate_ci_workflow(tmpdir, {'ci': 'github-actions'})

            result = _run_file_target(
                'ci-workflow',
                'ci-workflow-display-path',
                tmpdir,
                CI_WORKFLOW_RELATIVE_PATHS,
                lambda root: generate_ci_workflow(root, {'ci': 'github-actions'}),
                False,
                False,
            )
            self.assertEqual(result['state'], 'skipped')


class TestRunProjectWiring(unittest.TestCase):
    """End-to-end through `_run_project` (the actual `trackfw update --json`
    code path), not just the hash-diff helper — exercises
    _load_update_config → project_target_ids → _resolve_project_targets →
    the "ci-workflow" dispatch branch → the generate_ci_workflow closure.

    `project_config.load()` is a process-global singleton keyed by first
    call, not by cwd (pypi/trackfw/config.py) — every test in this class
    resets it before and after, same convention as
    test_agent_conventions.py/test_baseline.py, or the cfg read here would
    silently be whatever an earlier test's trackfw.yaml cached.
    """

    def setUp(self):
        project_config.reset()

    def tearDown(self):
        project_config.reset()

    def test_ci_workflow_in_report_updated_and_positioned(self):
        import argparse
        from trackfw.commands.update import _run_project

        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, 'github-actions')
            dest_dir = os.path.join(tmpdir, '.github', 'workflows')
            os.makedirs(dest_dir, exist_ok=True)
            stale = build_github_actions_workflow_content(None).replace(
                f'"{TRACKFW_VERSION}"', '"0.0.1"'
            )
            with open(os.path.join(dest_dir, 'trackfw-gate.yml'), 'w', encoding='utf-8') as f:
                f.write(stale)

            cwd = os.getcwd()
            os.chdir(tmpdir)
            try:
                args = argparse.Namespace(dry_run=False, json=True, targets=None, install_missing=False)
                import io
                import contextlib

                buf = io.StringIO()
                with contextlib.redirect_stdout(buf):
                    _run_project(args)
            finally:
                os.chdir(cwd)

            import json as json_mod
            payload = json_mod.loads(buf.getvalue())
            target_ids = [t['id'] for t in payload['targets']]
            self.assertIn('ci-workflow', target_ids)
            self.assertEqual(
                target_ids.index('ci-workflow'),
                target_ids.index('validate-script') + 1,
            )
            ci_target = next(t for t in payload['targets'] if t['id'] == 'ci-workflow')
            self.assertEqual(ci_target['state'], 'updated')

            with open(os.path.join(dest_dir, 'trackfw-gate.yml'), encoding='utf-8') as f:
                self.assertIn(f'"{TRACKFW_VERSION}"', f.read())

    def test_ci_workflow_absent_from_report_without_ci(self):
        import argparse
        import contextlib
        import io
        import json as json_mod
        from trackfw.commands.update import _run_project

        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir, None)
            cwd = os.getcwd()
            os.chdir(tmpdir)
            try:
                args = argparse.Namespace(dry_run=False, json=True, targets=None, install_missing=False)
                buf = io.StringIO()
                with contextlib.redirect_stdout(buf):
                    _run_project(args)
            finally:
                os.chdir(cwd)

            payload = json_mod.loads(buf.getvalue())
            target_ids = [t['id'] for t in payload['targets']]
            self.assertNotIn('ci-workflow', target_ids)


if __name__ == '__main__':
    unittest.main()
