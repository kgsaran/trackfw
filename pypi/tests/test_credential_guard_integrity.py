"""
ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
Mirrors internal/validator/validator_credential_guard_integrity_test.go (Go) and
npm/tests/credential_guard_integrity.test.js (Node).
"""

import os
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config
from trackfw import validator
from trackfw.generators.init_gen import _generate_credential_guard_script


def _git(cwd, *args):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True)


def _init_git_repo(cwd):
    _git(cwd, "init")
    _git(cwd, "config", "user.email", "test@test.com")
    _git(cwd, "config", "user.name", "test")
    _git(cwd, "commit", "--allow-empty", "-m", "init")


def _write(base, rel, content):
    full = os.path.join(base, rel)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    with open(full, "w", encoding="utf-8") as f:
        f.write(content)


def _commit_trackfw_yaml(cwd, content):
    _write(cwd, "trackfw.yaml", content)
    _git(cwd, "add", "trackfw.yaml")
    _git(cwd, "commit", "-m", "trackfw.yaml")


def _messages(items):
    return [item["message"] for item in items]


class TestCredentialGuardScriptIntegrity(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    def test_script_ausente_silencio(self):
        msgs = validator.validate_credential_guard_script_integrity(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_script_identico_ao_template_silencio(self):
        _write(self.tmpdir, "scripts/trackfw-credential-guard.sh", validator._CREDENTIAL_GUARD_SCRIPT_REFERENCE)
        msgs = validator.validate_credential_guard_script_integrity(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_script_divergente_dispara_mensagem_neutra(self):
        _write(self.tmpdir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
        msgs = validator.validate_credential_guard_script_integrity(self.tmpdir)
        self.assertEqual(len(msgs), 1)
        text = msgs[0]["message"]
        self.assertIn("scripts/trackfw-credential-guard.sh", text)
        self.assertIn("diverges from the template", text)
        lower = text.lower()
        for forbidden in ("adulterad", "modified by", "tampered"):
            self.assertNotIn(forbidden, lower)

    def test_reference_e_byte_identico_ao_gerador_real(self):
        _generate_credential_guard_script(self.tmpdir)
        with open(os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh"), "r", encoding="utf-8") as f:
            emitted = f.read()
        self.assertEqual(emitted, validator._CREDENTIAL_GUARD_SCRIPT_REFERENCE)


class TestCredentialGuardModeDowngrade(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    def test_sem_git_silencio(self):
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_sem_commits_silencio(self):
        _git(self.tmpdir, "init")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_arquivo_nao_versionado_no_head_silencio(self):
        _init_git_repo(self.tmpdir)
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_head_sem_chave_mode_silencio(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "roadmap_dir: docs/roadmaps\n")
        _write(self.tmpdir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\ncredential_guard:\n  mode: warn\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_head_warn_nunca_dispara_direcional(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: warn\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: block\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_sem_mudanca_silencio(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(msgs, [])

    def test_downgrade_block_para_warn_dispara(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(len(msgs), 1)
        self.assertIn("credential_guard.mode: block", msgs[0]["message"])

    def test_chave_removida_no_disco_dispara(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "roadmap_dir: docs/roadmaps\ncredential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(len(msgs), 1)

    def test_arquivo_deletado_no_disco_dispara(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        os.remove(os.path.join(self.tmpdir, "trackfw.yaml"))
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(len(msgs), 1)


class TestCredentialGuardIntegrityConfiguravel(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    def test_script_integrity_default_warning(self):
        _write(self.tmpdir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertFalse(any("scripts/trackfw-credential-guard.sh" in m for m in _messages(result["violations"])))
        self.assertTrue(any("scripts/trackfw-credential-guard.sh" in m for m in _messages(result["warnings"])))

    def test_script_integrity_rules_error(self):
        _write(self.tmpdir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
        _write(self.tmpdir, "trackfw.yaml", "rules:\n  credential_guard_script_integrity: error\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertTrue(any("scripts/trackfw-credential-guard.sh" in m for m in _messages(result["violations"])))

    def test_mode_downgrade_default_error(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertTrue(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))

    def test_mode_downgrade_rules_warning(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertFalse(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))
        self.assertTrue(any("credential_guard.mode: block" in m for m in _messages(result["warnings"])))

    def test_mode_downgrade_rules_off(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertFalse(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))
        self.assertFalse(any("credential_guard.mode: block" in m for m in _messages(result["warnings"])))


if __name__ == "__main__":
    unittest.main()
