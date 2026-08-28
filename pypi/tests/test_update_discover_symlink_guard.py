"""
test_update_discover_symlink_guard.py — corrective falsifier for the symlink-follow
arbitrary-write reported by hades-tf's final barrier review (2026-08-28):
`trackfw update` and `trackfw discover --init` decided the presence of
.github/workflows/trackfw-validate.yml with os.path.isfile (follows symlinks). A LIVE
symlink at that path pointing OUTSIDE the project let `trackfw update` overwrite the
linked-to file even in a `ci: none` project (which never asked for CI management at
all); a DANGLING symlink let `trackfw discover --init` CREATE a file at whatever path
the attacker chose. Fixed by deciding presence with os.path.islink (checked before
os.path.isfile) and refusing (loudly, on stderr) to write through a symlink either way.

Mirrors internal/generators/update_test.go's
TestUpdateNeverWritesThroughSymlinkAtDiscoverWorkflowPath /
TestUpdateNeverWritesThroughDanglingSymlinkAtDiscoverWorkflowPath and
internal/discover/discover_test.go's TestWriteCIWorkflow_NeverWritesThroughLiveSymlink /
TestWriteCIWorkflow_NeverWritesThroughDanglingSymlink (Go, canonical reference) and
npm/tests/update_discover_symlink_guard.test.js.
"""

import argparse
import contextlib
import io
import os
import tempfile
import unittest

from trackfw import config as project_config
from trackfw.commands.discover import (
    DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
    _write_ci_workflow,
)
from trackfw.commands.update import _run, _run_project


def _write_trackfw_yaml(tmpdir: str, ci: str | None) -> None:
    lines = ['backend: go']
    if ci:
        lines.append(f'ci: {ci}')
    with open(os.path.join(tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
        f.write('\n'.join(lines) + '\n')


def _workflow_path(tmpdir: str) -> str:
    return os.path.join(tmpdir, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)


def _run_bare_update(tmpdir: str) -> str:
    """Runs `_run` — the code path a bare `trackfw update` (no flags) actually
    takes. Returns whatever it printed to stderr."""
    cwd = os.getcwd()
    os.chdir(tmpdir)
    try:
        stdout_buf = io.StringIO()
        stderr_buf = io.StringIO()
        with contextlib.redirect_stdout(stdout_buf), contextlib.redirect_stderr(stderr_buf):
            _run(argparse.Namespace())
        return stderr_buf.getvalue()
    finally:
        os.chdir(cwd)


class TestUpdateNeverWritesThroughSymlink(unittest.TestCase):
    def setUp(self):
        project_config.reset()

    def tearDown(self):
        project_config.reset()

    def test_ci_none_never_writes_through_live_symlink_outside_project(self):
        with tempfile.TemporaryDirectory() as tmpdir, tempfile.TemporaryDirectory() as outside:
            _write_trackfw_yaml(tmpdir, None)
            victim = os.path.join(outside, 'vitima.txt')
            original_content = 'CONTEUDO ORIGINAL DA VITIMA\n'
            with open(victim, 'w', encoding='utf-8') as f:
                f.write(original_content)

            workflow_path = _workflow_path(tmpdir)
            os.makedirs(os.path.dirname(workflow_path), exist_ok=True)
            os.symlink(victim, workflow_path)

            _run_bare_update(tmpdir)

            with open(victim, encoding='utf-8') as f:
                self.assertEqual(f.read(), original_content, 'symlink-follow arbitrary write: victim file outside the project was overwritten')
            self.assertTrue(os.path.islink(workflow_path), 'trackfw-validate.yml symlink should remain untouched')

    def test_ci_github_actions_warns_on_stderr_and_refuses_live_symlink(self):
        with tempfile.TemporaryDirectory() as tmpdir, tempfile.TemporaryDirectory() as outside:
            _write_trackfw_yaml(tmpdir, 'github-actions')
            victim = os.path.join(outside, 'vitima.txt')
            original_content = 'CONTEUDO ORIGINAL DA VITIMA\n'
            with open(victim, 'w', encoding='utf-8') as f:
                f.write(original_content)

            workflow_path = _workflow_path(tmpdir)
            os.makedirs(os.path.dirname(workflow_path), exist_ok=True)
            os.symlink(victim, workflow_path)

            stderr = _run_bare_update(tmpdir)

            with open(victim, encoding='utf-8') as f:
                self.assertEqual(f.read(), original_content)
            self.assertIn(DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH, stderr)
            self.assertIn('symlink', stderr)

    def test_ci_workflow_target_not_declared_for_live_symlink_only(self):
        """A live symlink is not "manageable" by update, so — for a ci:none
        project — the ci-workflow target must not be declared on its account
        via the --targets/--json path (_run_project)."""
        with tempfile.TemporaryDirectory() as tmpdir, tempfile.TemporaryDirectory() as outside:
            _write_trackfw_yaml(tmpdir, None)
            victim = os.path.join(outside, 'vitima.txt')
            with open(victim, 'w', encoding='utf-8') as f:
                f.write('CONTEUDO ORIGINAL DA VITIMA\n')

            workflow_path = _workflow_path(tmpdir)
            os.makedirs(os.path.dirname(workflow_path), exist_ok=True)
            os.symlink(victim, workflow_path)

            cwd = os.getcwd()
            os.chdir(tmpdir)
            try:
                import json
                args = argparse.Namespace(dry_run=False, json=True, targets=None, install_missing=False)
                buf = io.StringIO()
                with contextlib.redirect_stdout(buf):
                    _run_project(args)
                payload = json.loads(buf.getvalue())
            finally:
                os.chdir(cwd)

            target = next((t for t in payload['targets'] if t['id'] == 'ci-workflow'), None)
            self.assertIsNone(target, 'ci-workflow should not be declared for a ci:none project whose only trackfw-validate.yml is a symlink')


class TestDiscoverInitNeverWritesThroughDanglingSymlink(unittest.TestCase):
    def test_write_ci_workflow_never_creates_file_through_dangling_symlink(self):
        with tempfile.TemporaryDirectory() as tmpdir, tempfile.TemporaryDirectory() as outside:
            dangling_target = os.path.join(outside, 'does-not-exist-yet')
            workflows_dir = os.path.join(tmpdir, '.github', 'workflows')
            os.makedirs(workflows_dir, exist_ok=True)
            link = os.path.join(workflows_dir, 'trackfw-validate.yml')
            os.symlink(dangling_target, link)

            _write_ci_workflow(tmpdir)

            self.assertFalse(
                os.path.exists(dangling_target),
                'dangling-symlink arbitrary write: _write_ci_workflow created a file outside the project',
            )

    def test_write_ci_workflow_warns_on_stderr_for_live_symlink(self):
        with tempfile.TemporaryDirectory() as tmpdir, tempfile.TemporaryDirectory() as outside:
            victim = os.path.join(outside, 'vitima.txt')
            original_content = 'CONTEUDO ORIGINAL DA VITIMA\n'
            with open(victim, 'w', encoding='utf-8') as f:
                f.write(original_content)

            workflows_dir = os.path.join(tmpdir, '.github', 'workflows')
            os.makedirs(workflows_dir, exist_ok=True)
            link = os.path.join(workflows_dir, 'trackfw-validate.yml')
            os.symlink(victim, link)

            stderr_buf = io.StringIO()
            with contextlib.redirect_stderr(stderr_buf):
                _write_ci_workflow(tmpdir)

            with open(victim, encoding='utf-8') as f:
                self.assertEqual(f.read(), original_content)
            stderr = stderr_buf.getvalue()
            self.assertIn(DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH, stderr)
            self.assertIn('symlink', stderr)


if __name__ == '__main__':
    unittest.main()
