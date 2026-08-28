"""
test_discover_ci_workflow_pin.py — ROADMAP-2026-08-28 ML-2F (corrective, Wave 2): the
SECOND install mechanism — `go install github.com/kgsaran/trackfw/cmd/trackfw@latest`,
written to .github/workflows/trackfw-validate.yml by `trackfw discover --init`
(install_gates) — was never pinned by ML-2A/2B/2C, which only covered
trackfw-gate.yml (the install.sh mechanism, trackfw.generators.init_gen).

Mirrors internal/discover/discover_test.go (Go, canonical source of truth for this
second surface) and npm/tests/discover_ci_workflow_version_pin.test.js.
"""

import os
import tempfile
import unittest
from unittest import mock

from trackfw import __version__ as TRACKFW_VERSION
from trackfw.commands.discover import (
    DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
    build_discover_github_actions_workflow_content,
    install_gates,
)
from trackfw.integrations.scaffold_doctor import run_scaffold_doctor
from trackfw.integrations.doctor import SCAFFOLD_DIVERGENT


def _write_trackfw_yaml(tmpdir: str) -> None:
    with open(os.path.join(tmpdir, 'trackfw.yaml'), 'w', encoding='utf-8') as f:
        f.write('backend: go\n')


def _empty_discover_result() -> dict:
    return {
        'hook_framework': 'none',
        'ci_system': 'github-actions',
    }


def _workflow_path(tmpdir: str) -> str:
    return os.path.join(tmpdir, '.github', 'workflows', 'trackfw-validate.yml')


class TestDiscoverCIWorkflowPin(unittest.TestCase):
    def test_install_gates_pins_go_install_to_binary_version(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            install_gates(_empty_discover_result(), tmpdir)
            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                content = f.read()
            self.assertIn(
                f'go install github.com/kgsaran/trackfw/cmd/trackfw@v{TRACKFW_VERSION}',
                content,
            )
            self.assertNotIn('trackfw/cmd/trackfw@latest', content)

    def test_install_gates_output_matches_builder_byte_for_byte(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            install_gates(_empty_discover_result(), tmpdir)
            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                on_disk = f.read()
            self.assertEqual(on_disk, build_discover_github_actions_workflow_content())

    def test_go_install_pin_is_not_hardcoded(self):
        # Falsifies the specific regression the ADR warns against: a template with
        # `@v7.3.0` typed literally into the generator source would pass the assertion
        # in test_install_gates_pins_go_install_to_binary_version today, because
        # TRACKFW_VERSION happens to equal "7.3.0" right now. `from trackfw import
        # __version__` inside build_discover_github_actions_workflow_content reads the
        # module attribute live on every call, so patching trackfw.__version__ and
        # re-generating is the only way to prove the pin tracks the attribute, not a
        # literal.
        with tempfile.TemporaryDirectory() as tmpdir:
            with mock.patch('trackfw.__version__', '9.9.9-stub'):
                install_gates(_empty_discover_result(), tmpdir)
                with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                    content = f.read()
            self.assertIn('trackfw@v9.9.9-stub', content)
            self.assertNotIn(TRACKFW_VERSION, content)

    def test_builder_is_idempotent_across_calls(self):
        first = build_discover_github_actions_workflow_content()
        second = build_discover_github_actions_workflow_content()
        self.assertEqual(first, second)

    def test_install_gates_does_not_overwrite_existing_file(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            workflows_dir = os.path.join(tmpdir, '.github', 'workflows')
            os.makedirs(workflows_dir, exist_ok=True)
            with open(_workflow_path(tmpdir), 'w', encoding='utf-8') as f:
                f.write('# existing\n')
            install_gates(_empty_discover_result(), tmpdir)
            with open(_workflow_path(tmpdir), encoding='utf-8') as f:
                self.assertEqual(f.read(), '# existing\n')

    def test_doctor_no_mismatch_right_after_generation(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir)
            install_gates(_empty_discover_result(), tmpdir)
            findings = run_scaffold_doctor(tmpdir)
            hits = [f for f in findings if f['destination'] == DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH]
            self.assertEqual(hits, [], f'expected no finding, got: {hits}')

    def test_doctor_scaffold_divergent_when_pin_manually_changed(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir)
            install_gates(_empty_discover_result(), tmpdir)
            path = _workflow_path(tmpdir)
            with open(path, encoding='utf-8') as f:
                original = f.read()
            tampered = original.replace(
                f'trackfw@v{TRACKFW_VERSION}', 'trackfw@v0.0.1-stale'
            )
            self.assertNotEqual(tampered, original, 'tamper substitution did not change content')
            with open(path, 'w', encoding='utf-8') as f:
                f.write(tampered)

            findings = run_scaffold_doctor(tmpdir)
            hits = [f for f in findings if f['destination'] == DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH]
            self.assertEqual(len(hits), 1, f'expected exactly one finding, got: {findings}')
            self.assertEqual(hits[0]['finding'], SCAFFOLD_DIVERGENT)

    def test_doctor_reports_nothing_when_discover_workflow_absent(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            _write_trackfw_yaml(tmpdir)
            findings = run_scaffold_doctor(tmpdir)
            hits = [f for f in findings if f['destination'] == DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH]
            self.assertEqual(hits, [])


if __name__ == '__main__':
    unittest.main()
