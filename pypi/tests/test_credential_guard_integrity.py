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

    # ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard,
    # ADR-2026-08-12-severidade-das-regras-de-credential-guard-...: as duas subtests abaixo COMMITAM
    # a mudança de rules: junto com mode: block (a âncora do HEAD, que não pode ser removida, senão
    # a regra silencia por falta de âncora — outro teste). Antes deste ADR, este teste escrevia
    # "rules: <nome>: warning|off" só em disco, SEM commit — exatamente o auto-silenciamento sem
    # rastro que o ADR fecha; ver "*_nao_commitado_ainda_dispara" abaixo para o canal fechado.

    def test_mode_downgrade_rules_warning_commitado(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\nrules:\n  credential_guard_mode_downgrade: warning\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertFalse(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))
        self.assertTrue(any("credential_guard.mode: block" in m for m in _messages(result["warnings"])))

    def test_mode_downgrade_rules_off_commitado(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\nrules:\n  credential_guard_mode_downgrade: off\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertFalse(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))
        self.assertFalse(any("credential_guard.mode: block" in m for m in _messages(result["warnings"])))

    def test_mode_downgrade_rules_warning_nao_commitado_ainda_dispara(self):
        _init_git_repo(self.tmpdir)
        # HEAD só tem mode: block — SEM rules:. Disco rebaixa mode E desliga a regra na MESMA
        # edição, nunca commitada. Ataque combinado que o ADR fecha.
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertTrue(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))

    def test_mode_downgrade_rules_off_nao_commitado_ainda_dispara(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n")
        result = validator.validate_unfiltered(self.tmpdir)
        self.assertTrue(any("credential_guard.mode: block" in m for m in _messages(result["violations"])))


class TestRuleSeverityZeroDeltaParaRegrasNaoGuard(unittest.TestCase):
    """Espelha TestRuleSeverity_ZeroDeltaParaRegrasNaoGuard (Go): ruleSeverity()/_rule_severity()
    para qualquer regra fora de _CREDENTIAL_GUARD_ANCHORED_RULES continua resolvendo só pelo
    disco — o critério de aceite "zero delta para as outras ~38 regras" do
    ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard.
    """

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    def test_zero_delta(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "")
        _write(self.tmpdir, "trackfw.yaml", "rules:\n  wip_limit: warning\n  adr_orphan: off\n")
        cfg = config.load(self.tmpdir)
        self.assertEqual(validator._rule_severity("wip_limit", cfg, self.tmpdir), "warning")
        self.assertEqual(validator._rule_severity("adr_orphan", cfg, self.tmpdir), "off")
        self.assertEqual(validator._rule_severity("filename_uniqueness", cfg, self.tmpdir), "error")


class TestCredentialGuardRuleSeveritySemHead(unittest.TestCase):
    """Espelha TestCredentialGuardRuleSeverity_SemHead_CaiNoDisco (Go): sem HEAD utilizável, a
    resolução das regras de credential-guard cai no disco puro.
    """

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    def test_sem_head_cai_no_disco(self):
        # nem sequer é git worktree
        _write(self.tmpdir, "trackfw.yaml", "rules:\n  credential_guard_mode_downgrade: warning\n")
        cfg = config.load(self.tmpdir)
        self.assertEqual(
            validator._rule_severity("credential_guard_mode_downgrade", cfg, self.tmpdir),
            "warning",
        )


class TestGitExecEnvIsolation(unittest.TestCase):
    """ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1B.
    Mirrors internal/validator/validator_git_exec_test.go (Go) and the ML-1B section of
    npm/tests/credential_guard_integrity.test.js (Node).
    """

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    def _set_env(self, overrides):
        saved = {}
        for key, value in overrides.items():
            saved[key] = os.environ.get(key)
            os.environ[key] = value
        self.addCleanup(lambda: self._restore_env(saved))

    def _restore_env(self, saved):
        for key, value in saved.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value

    def test_clean_git_env_remove_apenas_prefixo_git(self):
        self._set_env({
            "GIT_DIR": "/tmp/whatever",
            "GIT_CONFIG_COUNT": "abc",
            "MY_GIT_DIR_LOOKALIKE": "kept",
        })
        cleaned = validator._clean_git_env()
        for key in cleaned:
            self.assertFalse(key.startswith("GIT_"), f"_clean_git_env() não deveria manter {key}")
        self.assertEqual(cleaned.get("MY_GIT_DIR_LOOKALIKE"), "kept")

    def test_mode_downgrade_git_dir_work_tree_redirecionados_continua_detectando(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")

        other = tempfile.mkdtemp()
        self.addCleanup(lambda: shutil.rmtree(other, ignore_errors=True))
        _init_git_repo(other)

        self._set_env({
            "GIT_DIR": os.path.join(other, ".git"),
            "GIT_WORK_TREE": other,
        })

        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(
            len(msgs), 1,
            f"GIT_DIR/GIT_WORK_TREE redirecionados NÃO deveriam silenciar a detecção, obteve: {msgs}",
        )
        self.assertIn("credential_guard.mode: block", msgs[0]["message"])

    def test_mode_downgrade_git_config_count_malformado_continua_detectando(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        _write(self.tmpdir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")

        self._set_env({"GIT_CONFIG_COUNT": "abc"})

        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(
            len(msgs), 1,
            f"GIT_CONFIG_COUNT malformado NÃO deveria silenciar a detecção, obteve: {msgs}",
        )
        self.assertIn("credential_guard.mode: block", msgs[0]["message"])

    def test_mode_downgrade_git_config_count_malformado_prova_nao_vacuidade(self):
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")

        env = dict(os.environ)
        env["GIT_CONFIG_COUNT"] = "abc"
        result = subprocess.run(
            ["git", "-C", self.tmpdir, "rev-parse", "--verify", "HEAD"],
            env=env, capture_output=True, text=True,
        )
        self.assertNotEqual(
            result.returncode, 0,
            "esperava que git falhasse com GIT_CONFIG_COUNT=abc herdado sem limpeza — "
            "não falhou, o fixture não prova nada",
        )

    def test_is_git_worktree_linked_worktree_legitimo_continua_funcionando(self):
        main_dir = tempfile.mkdtemp()
        self.addCleanup(lambda: shutil.rmtree(main_dir, ignore_errors=True))
        _init_git_repo(main_dir)
        _commit_trackfw_yaml(main_dir, "credential_guard:\n  mode: block\n")

        linked_parent = tempfile.mkdtemp()
        self.addCleanup(lambda: shutil.rmtree(linked_parent, ignore_errors=True))
        linked_dir = os.path.join(linked_parent, "linked")
        _git(main_dir, "worktree", "add", "-b", "feat/linked-worktree-test-python", linked_dir)

        # Sem downgrade — disco na worktree ainda resolve para block, idêntico ao HEAD.
        self.assertEqual(validator.validate_credential_guard_mode_downgrade(linked_dir), [])

        # Downgrade introduzido dentro da worktree — deve disparar normalmente.
        _write(linked_dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
        msgs = validator.validate_credential_guard_mode_downgrade(linked_dir)
        self.assertEqual(len(msgs), 1)
        self.assertIn("credential_guard.mode: block", msgs[0]["message"])


if __name__ == "__main__":
    unittest.main()
