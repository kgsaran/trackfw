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
    # newline="\n" mirrors production writers (generators/*.py all pass it explicitly,
    # enforced by scripts/check-python-writes-lf.sh) -- without it, Python's text-mode
    # open() on Windows translates every embedded "\n" in `content` to os.linesep
    # ("\r\n"), so a fixture written from an LF-only string constant (e.g.
    # validator._CREDENTIAL_GUARD_SCRIPT_REFERENCE) lands on disk with CRLF. ML-1H
    # (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-
    # silencio) measured this live on Windows ARM64: it made
    # test_script_identico_ao_template_silencio fail, because the guard-script
    # *_script_integrity family compares raw bytes against the reference (correctly, by
    # design -- CRLF in a generated .sh corrupts its shebang) and a CRLF-corrupted
    # fixture is genuinely not identical to the LF template. Fixed here at the fixture,
    # not by loosening the byte-exact comparison in production code.
    with open(full, "w", encoding="utf-8", newline="\n") as f:
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

    def test_script_crlf_dispara_divergencia(self):
        """ML-1H: afirma que _read_regular_file (guard-script family) NUNCA folds CRLF->LF --
        um script CRLF-corrompido continua reportado como divergente do template, mesmo sendo
        byte-a-byte igual ao template modulo terminador de linha. Guarda contra a tentação de
        "resolver" test_script_identico_ao_template_silencio normalizando a comparação em vez
        de corrigir a fixture: CRLF num .sh gerado quebra o shebang em POSIX ("bad
        interpreter") -- é conteúdo divergente de fato, não estilo de EOL tolerável."""
        crlf_script = validator._CREDENTIAL_GUARD_SCRIPT_REFERENCE.replace("\n", "\r\n")
        full = os.path.join(self.tmpdir, "scripts/trackfw-credential-guard.sh")
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "wb") as f:
            f.write(crlf_script.encode("utf-8"))
        msgs = validator.validate_credential_guard_script_integrity(self.tmpdir)
        self.assertEqual(len(msgs), 1, f"esperava divergencia para script CRLF-corrompido, obteve: {msgs}")
        self.assertIn("diverges from the template", msgs[0]["message"])

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


# ---------------------------------------------------------------------------
# ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C.
# Mirrors internal/validator/validator_credential_guard_integrity_test.go's
# TestCredentialGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar and
# TestCredentialGuardScriptIntegrity_FIFO_NaoTrava. Python's script_integrity has always
# `except OSError: return []` on ANY read error, not just absence -- hades-tf's ML-1B barrier
# found this silenced EACCES exactly like the pre-ML-1A config-file `continue` did. Distinct from
# Go (which ABORTED instead of silencing) -- Python's failure mode here was always fail-open, not
# a crash, so there is no "was it a crash" assertion to make, only "was it silenced".
# ---------------------------------------------------------------------------

class TestCredentialGuardScriptIntegrityUnreadable(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        config.reset()

    @unittest.skipIf(sys.platform == "win32", "bits de permissão POSIX não se aplicam no Windows")
    @unittest.skipIf(hasattr(os, "geteuid") and os.geteuid() == 0, "chmod 000 não bloqueia leitura para root")
    def test_script_ilegivel_vira_violation_nao_silencio(self):
        _write(self.tmpdir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
        script_path = os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh")
        os.chmod(script_path, 0o000)
        try:
            msgs = _messages(validator.validate_credential_guard_script_integrity(self.tmpdir))
            self.assertTrue(any("could not be read" in m for m in msgs), msgs)
        finally:
            os.chmod(script_path, 0o644)

    @unittest.skipIf(sys.platform == "win32", "mkfifo não existe no Windows")
    def test_fifo_no_lugar_do_script_nao_trava(self):
        import threading

        os.makedirs(os.path.join(self.tmpdir, "scripts"), exist_ok=True)
        fifo_path = os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh")
        os.mkfifo(fifo_path)

        watchdog = threading.Timer(5.0, lambda: os._exit(1))
        watchdog.daemon = True
        watchdog.start()
        try:
            msgs = _messages(validator.validate_credential_guard_script_integrity(self.tmpdir))
            self.assertTrue(any("could not be read" in m for m in msgs), msgs)
        finally:
            watchdog.cancel()

    def test_byte_invalido_utf8_no_script_nao_crasha_e_classifica_como_divergencia(self):
        """Regressão específica do Python: ler o script em modo texto estrito (encoding="utf-8")
        podia levantar UnicodeDecodeError não capturado num script sobrescrito com bytes inválidos
        -- esta função nunca foi tocada pelo ML-1B (que corrigiu só os arquivos de CONFIG JSON), e
        continuava com o mesmo bug de classe. Ler como bytes (ML-1C) elimina o crash inteiramente."""
        script_path = os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh")
        os.makedirs(os.path.dirname(script_path), exist_ok=True)
        with open(script_path, "wb") as f:
            f.write(b"#!/bin/bash\n\xff\xfe invalid utf8 \xff")

        msgs = _messages(validator.validate_credential_guard_script_integrity(self.tmpdir))
        self.assertEqual(len(msgs), 1)
        self.assertIn("diverges from the template", msgs[0])


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

    def test_leitura_falha_nao_enoent_usa_texto_fixo_nao_str_erro_cru(self):
        """ML-1G (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-
        silencio): erro de leitura não-ENOENT (diretório no lugar do arquivo) usa mensagem de
        texto FIXO, não str(OSError) cru (que diverge de Go %v e Node err.message para a mesma
        falha), e não reusa o texto de downgrade CONFIRMADO. Mirrors
        TestCredentialGuardModeDowngrade_LeituraFalhaNaoENOENT_ViolationSemAbortarEComTextoFixo (Go)
        e o teste homônimo em npm/tests/credential_guard_integrity.test.js (Node)."""
        _init_git_repo(self.tmpdir)
        _commit_trackfw_yaml(self.tmpdir, "credential_guard:\n  mode: block\n")
        os.remove(os.path.join(self.tmpdir, "trackfw.yaml"))
        os.mkdir(os.path.join(self.tmpdir, "trackfw.yaml"))
        msgs = validator.validate_credential_guard_mode_downgrade(self.tmpdir)
        self.assertEqual(len(msgs), 1)
        self.assertEqual(
            msgs[0]["message"],
            "trackfw.yaml could not be read — trackfw cannot tell whether credential_guard.mode "
            "is still block; fix the file, or run `trackfw update` to regenerate it",
        )
        self.assertNotIn("credential_guard.mode: block", msgs[0]["message"])


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
