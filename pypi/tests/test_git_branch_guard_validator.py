"""
ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
ML-2B. Mirrors internal/validator/validator_git_branch_guard_test.go (Go).
"""

import json
import os
import shutil
import stat
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw import validator as v
from trackfw.generators.init_gen import _generate_git_branch_guard_script


def _write(path: str, content: str = ""):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _messages(items):
    return [item["message"] for item in items]


def _git_branch_guard_entry_claude_settings(script_cmd: str) -> str:
    return json.dumps({
        "hooks": {
            "PreToolUse": [
                {"matcher": "Bash", "hooks": [{"command": script_cmd, "type": "command"}]}
            ]
        }
    })


# ---- git_branch_guard_hook_resolvable (projeto) ----

class TestGitBranchGuardHookResolvable(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_dispara_script_ausente(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _git_branch_guard_entry_claude_settings("$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"),
        )
        # scripts/trackfw-git-branch-guard.sh NÃO é criado — ausência proposital.
        cfg = _config.defaults()
        msgs = v.validate_git_branch_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any(
                "does not exist" in m["message"]
                and ".claude/settings.json" in m["message"]
                and "trackfw-git-branch-guard.sh" in m["message"]
                for m in msgs
            ),
            f"esperado violation de script ausente, obteve: {msgs}",
        )

    def test_dispara_script_nao_executavel(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _git_branch_guard_entry_claude_settings("$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-git-branch-guard.sh")
        _write(script_path, "#!/bin/sh\nexit 0\n")
        os.chmod(script_path, 0o644)  # sem bit +x

        cfg = _config.defaults()
        msgs = v.validate_git_branch_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertTrue(
            any("not executable" in m["message"] for m in msgs),
            f"esperado violation de script não executável, obteve: {msgs}",
        )

    def test_nao_dispara_sem_entrada(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            json.dumps({
                "hooks": {
                    "PostToolUse": [{"matcher": "AskUserQuestion", "hooks": [
                        {"command": "scripts/trackfw-attention-cleanup.sh", "type": "command"}]}],
                }
            }),
        )
        cfg = _config.defaults()
        msgs = v.validate_git_branch_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(msgs, [], f"esperado zero violations sem entrada de guard, obteve: {msgs}")

    def test_nao_dispara_script_presente_e_executavel(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            _git_branch_guard_entry_claude_settings("$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-git-branch-guard.sh")
        _write(script_path, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
        os.chmod(script_path, 0o755)

        cfg = _config.defaults()
        msgs = v.validate_git_branch_guard_hook_resolvable(cfg, cwd=self.tmp)
        self.assertEqual(
            msgs, [], f"esperado zero violations com script presente e executável, obteve: {msgs}"
        )


class TestGitBranchGuardHookResolvableMalformedType(unittest.TestCase):
    """ROADMAP-2026-08-17 ML-4B -- hades-tf ML-4A barrier finding reproduced at PROJECT scope: a
    hook entry with the CORRECT command but MISSING "type":"command" (script present and
    íntegro) makes neither the dedup NOR this rule notice anything wrong."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_dispara_entrada_sem_type(self):
        _write(
            os.path.join(self.tmp, ".claude/settings.json"),
            json.dumps({
                "hooks": {
                    "PreToolUse": [
                        {"matcher": "Bash", "hooks": [
                            {"command": "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"}]}
                    ]
                }
            }),
        )
        script_path = os.path.join(self.tmp, "scripts", "trackfw-git-branch-guard.sh")
        _write(script_path, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
        os.chmod(script_path, 0o755)

        cfg = _config.defaults()
        msgs = v.validate_git_branch_guard_hook_resolvable(cfg, cwd=self.tmp)
        texts = [m["message"] for m in msgs]
        self.assertTrue(
            any(
                'missing "type":"command"' in t and ".claude/settings.json" in t and "Claude Code" in t
                for t in texts
            ),
            f"esperado violation de entrada estruturalmente malformada (type ausente), obteve: {texts}",
        )
        self.assertFalse(any("but the script does not exist" in t for t in texts))
        self.assertFalse(any("but the script is not executable" in t for t in texts))


# ---- git_branch_guard_script_integrity (projeto) ----

class TestGitBranchGuardScriptIntegrity(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_script_ausente_silencio(self):
        # scripts/trackfw-git-branch-guard.sh NÃO existe — cobertura de ausência é
        # git_branch_guard_hook_resolvable, não esta regra.
        msgs = v.validate_git_branch_guard_script_integrity(self.tmp)
        self.assertEqual(msgs, [])

    def test_script_identico_ao_template_silencio(self):
        _write(
            os.path.join(self.tmp, "scripts/trackfw-git-branch-guard.sh"),
            v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE,
        )
        msgs = v.validate_git_branch_guard_script_integrity(self.tmp)
        self.assertEqual(msgs, [])

    def test_um_byte_alterado_dispara(self):
        tampered = v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE[:-1] + "X"
        _write(os.path.join(self.tmp, "scripts/trackfw-git-branch-guard.sh"), tampered)

        msgs = v.validate_git_branch_guard_script_integrity(self.tmp)
        self.assertEqual(len(msgs), 1)
        text = msgs[0]["message"]
        self.assertIn("scripts/trackfw-git-branch-guard.sh", text)
        self.assertIn("diverges from the template", text)

    def test_severity_default_warning(self):
        _write(os.path.join(self.tmp, "scripts/trackfw-git-branch-guard.sh"), "#!/usr/bin/env bash\nexit 0\n")
        result = v.validate_unfiltered(self.tmp)
        self.assertFalse(
            any("trackfw-git-branch-guard.sh" in m for m in _messages(result["violations"]))
        )
        self.assertTrue(
            any("trackfw-git-branch-guard.sh" in m for m in _messages(result["warnings"]))
        )

    def test_reference_e_byte_identico_ao_gerador_real(self):
        _generate_git_branch_guard_script(self.tmp)
        with open(os.path.join(self.tmp, "scripts", "trackfw-git-branch-guard.sh"), "r", encoding="utf-8") as f:
            emitted = f.read()
        self.assertEqual(emitted, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)


# ---- Escopo global (credential-guard e git-branch-guard) ----

def _global_guard_home(test_case):
    """Cria um $HOME isolado (tempfile.mkdtemp) e aponta os.path.expanduser("~") pra lá via a
    variável de ambiente HOME, isolando os testes de escopo global do $HOME real da máquina."""
    home = tempfile.mkdtemp()
    saved = os.environ.get("HOME")
    os.environ["HOME"] = home

    def _restore():
        if saved is None:
            os.environ.pop("HOME", None)
        else:
            os.environ["HOME"] = saved
        shutil.rmtree(home, ignore_errors=True)

    test_case.addCleanup(_restore)
    return home


def _global_claude_settings_with_command(script_abs_path: str) -> str:
    """Monta ~/.claude/settings.json com uma entrada global PreToolUse[Bash] apontando para
    script_abs_path — mesma forma que os geradores de harness global escrevem."""
    return json.dumps({
        "hooks": {
            "PreToolUse": [
                {"matcher": "Bash", "hooks": [{"command": script_abs_path, "type": "command"}]}
            ]
        }
    })


def _global_claude_settings_with_command_no_type(script_abs_path: str) -> str:
    """ROADMAP-2026-08-17 ML-4B counterpart of _global_claude_settings_with_command:
    "type":"command" is deliberately OMITTED -- the exact malformed shape hades-tf's ML-4A
    barrier finding reproduced."""
    return json.dumps({
        "hooks": {
            "PreToolUse": [
                {"matcher": "Bash", "hooks": [{"command": script_abs_path}]}
            ]
        }
    })


def _global_cursor_hooks_with_command(script_abs_path: str) -> str:
    """Cursor's schema never carries a "type" field at all -- non-regression control proving
    requires_command_type=False for Cursor is not over-tightened by ML-4B."""
    return json.dumps({
        "version": 1,
        "hooks": {
            "beforeShellExecution": [{"command": script_abs_path}]
        }
    })


class TestGuardGlobalHookResolvable(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_sem_entrada_global_silencio(self):
        _global_guard_home(self)

        msgs = v.validate_credential_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(msgs, [], f"esperado zero violations sem entrada global, obteve: {msgs}")

        gmsgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(
            gmsgs, [],
            f"esperado zero violations sem entrada global (git-branch-guard), obteve: {gmsgs}",
        )

    def test_global_instalado_e_integro_silencio(self):
        # O gap principal que este ML fecha: hook de PROJETO ausente (dedup) + global instalado E
        # íntegro → silêncio (dedup preservado).
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
        _write(global_script_path, v._CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE)
        os.chmod(global_script_path, 0o755)
        _write(
            os.path.join(home, ".claude", "settings.json"),
            _global_claude_settings_with_command(global_script_path),
        )

        hook_msgs = v.validate_credential_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(
            hook_msgs, [], f"esperado zero violations com global instalado e executável, obteve: {hook_msgs}"
        )

        integrity_msgs = v.validate_credential_guard_global_script_integrity(self.tmp)
        self.assertEqual(
            integrity_msgs, [],
            f"esperado zero violations com script global íntegro, obteve: {integrity_msgs}",
        )

    def test_global_instalado_mas_script_ausente_dispara(self):
        # Hook de PROJETO ausente + global REGISTRADO em ~/.claude/settings.json mas o script
        # global não existe no disco → antes deste ML, `trackfw validate` silenciava; agora deve
        # violar.
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
        # Script global NÃO é criado — ausência proposital, apesar de estar registrado.
        _write(
            os.path.join(home, ".claude", "settings.json"),
            _global_claude_settings_with_command(global_script_path),
        )

        msgs = v.validate_credential_guard_global_hook_resolvable(self.tmp)
        self.assertTrue(
            any(
                "does not exist" in m["message"]
                and "global scope" in m["message"]
                and "trackfw update harness" in m["message"]
                for m in msgs
            ),
            f"esperado violation de script global ausente, obteve: {msgs}",
        )

    def test_global_registrado_sem_type_dispara(self):
        # ROADMAP-2026-08-17 ML-4B -- hades-tf ML-4A barrier finding: script global presente e
        # íntegro, mas a entrada de config está sem "type":"command" -- Claude Code nunca a
        # executa em silêncio. Antes desta ML: silêncio. Depois: violação, e NÃO a mensagem de
        # "does not exist" (o script existe, o problema é a forma estrutural).
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        _write(global_script_path, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
        os.chmod(global_script_path, 0o755)
        _write(
            os.path.join(home, ".claude", "settings.json"),
            _global_claude_settings_with_command_no_type(global_script_path),
        )

        msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        texts = [m["message"] for m in msgs]
        self.assertTrue(
            any(
                'missing "type":"command"' in t and "global scope" in t and "Claude Code" in t
                and "trackfw update harness" in t
                for t in texts
            ),
            f"esperado violation de entrada estruturalmente malformada (type ausente), obteve: {texts}",
        )
        self.assertFalse(any("but the script does not exist" in t for t in texts))
        self.assertFalse(any("but the script is not executable" in t for t in texts))

    def test_global_cursor_sem_type_e_normal_silencio(self):
        # Non-regression control: Cursor's schema never carries a "type" field, so its absence is
        # normal, not malformed.
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        _write(global_script_path, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
        os.chmod(global_script_path, 0o755)
        _write(
            os.path.join(home, ".cursor", "hooks.json"),
            _global_cursor_hooks_with_command(global_script_path),
        )

        msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(msgs, [], f"esperado zero violations (Cursor não carrega \"type\"), obteve: {msgs}")

    def test_global_instalado_mas_script_corrompido_dispara(self):
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
        _write(global_script_path, "#!/usr/bin/env bash\nexit 0\n")
        os.chmod(global_script_path, 0o755)
        _write(
            os.path.join(home, ".claude", "settings.json"),
            _global_claude_settings_with_command(global_script_path),
        )

        msgs = v.validate_credential_guard_global_script_integrity(self.tmp)
        self.assertTrue(
            any(
                "diverges from the template" in m["message"]
                and "global scope" in m["message"]
                and "trackfw update harness" in m["message"]
                for m in msgs
            ),
            f"esperado violation de integridade global, obteve: {msgs}",
        )

    def test_git_branch_guard_global_sem_wiring_hoje_silencio(self):
        # Atualizado no ML-3B (idêntico ao Go, ver
        # TestGitBranchGuardGlobal_SemWiringGlobalHoje_Silencio em
        # internal/validator/validator_git_branch_guard_test.go): a fiação global do
        # git-branch-guard EXISTE desde a Wave 2 (ML-2A), mas este teste não a exercita -- nenhum
        # dos arquivos de config globais é escrito no fixture. Prova o caso "script global
        # presente, nenhum config o referencia": deve permanecer em silêncio -- hook_resolvable é
        # condicionado à fiação por desenho.
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        _write(global_script_path, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
        os.chmod(global_script_path, 0o755)
        # Nenhum ~/.claude/settings.json (ou equivalente) referencia trackfw-git-branch-guard.sh hoje.

        msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(msgs, [], f"esperado silêncio (sem wiring global hoje), obteve: {msgs}")


# ---- git_branch_guard_script_integrity / credential_guard_script_integrity (escopo GLOBAL,
# disparo por EXISTÊNCIA do artefato — ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-e-integridade-independente-de-fiacao, ML-3A). Mirrors
# TestGuardGlobalScriptIntegrity_* em internal/validator/validator_git_branch_guard_test.go (Go).

class TestGuardGlobalScriptIntegrityByExistence(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_dispara_sem_nenhuma_fiacao(self):
        # O discriminante central deste ML: o script global existe e diverge do template, mas
        # ZERO arquivo de config referencia o marker. Antes deste ML o laço antigo nunca entrava.
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        _write(global_script_path, "#!/usr/bin/env bash\nexit 0\n")
        os.chmod(global_script_path, 0o755)
        # Nenhum arquivo de config é escrito neste $HOME.

        msgs = v.validate_git_branch_guard_global_script_integrity(self.tmp)
        self.assertTrue(
            any(
                "diverges from the template" in m["message"] and global_script_path in m["message"]
                for m in msgs
            ),
            f"esperado violation de integridade global mesmo sem fiação, obteve: {msgs}",
        )

    def test_ausencia_do_artefato_silencio(self):
        _global_guard_home(self)

        msgs = v.validate_git_branch_guard_global_script_integrity(self.tmp)
        self.assertEqual(msgs, [], f"esperado silêncio com script global ausente, obteve: {msgs}")

        cmsgs = v.validate_credential_guard_global_script_integrity(self.tmp)
        self.assertEqual(
            cmsgs, [], f"esperado silêncio (credential-guard) com script global ausente, obteve: {cmsgs}"
        )

    def test_nao_duplica_com_dois_configs_referenciando_o_mesmo_script(self):
        home = _global_guard_home(self)

        global_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        _write(global_script_path, "#!/usr/bin/env bash\nexit 0\n")
        os.chmod(global_script_path, 0o755)
        _write(
            os.path.join(home, ".claude", "settings.json"),
            _global_claude_settings_with_command(global_script_path),
        )
        _write(
            os.path.join(home, ".codex", "hooks.json"),
            _global_claude_settings_with_command(global_script_path),
        )

        msgs = v.validate_git_branch_guard_global_script_integrity(self.tmp)
        self.assertEqual(
            len(msgs), 1,
            f"esperado exatamente 1 mensagem (script referenciado por 2 configs), obteve {len(msgs)}: {msgs}",
        )


def _kiro_global_guard_fixture(hook_name_prefix: str, script_abs_path: str) -> str:
    """Monta o documento que os writers reais (harness_credential_guard_target_kiro/
    harness_git_branch_guard_target_kiro) escrevem -- {"version":"v1","hooks":[{...pre...},
    {...post...}]}, cada hook com action.command == script_abs_path."""
    return json.dumps({
        "version": "v1",
        "hooks": [
            {
                "name": f"{hook_name_prefix}-global-pre",
                "description": "global pre hook",
                "trigger": "PreToolUse",
                "matcher": "shell",
                "action": {"type": "command", "command": script_abs_path},
            },
            {
                "name": f"{hook_name_prefix}-global-post",
                "description": "global post hook",
                "trigger": "PostToolUse",
                "matcher": "shell",
                "action": {"type": "command", "command": script_abs_path},
            },
        ],
    })


# ---- git_branch_guard_hook_resolvable / credential_guard_hook_resolvable (escopo GLOBAL,
# arquivo DEDICADO do Kiro -- ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
# integridade-independente-de-fiacao, ML-3B). Mirrors internal/validator/
# validator_git_branch_guard_test.go's Test*Kiro* (Go).

class TestGuardGlobalHookResolvableKiroDedicatedFile(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_git_branch_guard_kiro_dedicado_script_ausente_dispara(self):
        # Discriminante central do ML-3B: antes dele, o arquivo dedicado do Kiro para
        # git-branch-guard (~/.kiro/hooks/trackfw-git-branch-guard.json, escrito desde a Wave 2)
        # nunca era lido -- um hook Kiro apontando para script ausente passava limpo.
        home = _global_guard_home(self)

        script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        # script_path NÃO é criado -- ausência proposital.
        _write(
            os.path.join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json"),
            _kiro_global_guard_fixture("trackfw-git-branch-guard", script_path),
        )

        msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertTrue(
            any(
                "does not exist" in m["message"]
                and "trackfw-git-branch-guard.json" in m["message"]
                and "Kiro" in m["message"]
                for m in msgs
            ),
            f"esperado violation do arquivo dedicado do Kiro para git-branch-guard, obteve: {msgs}",
        )

    def test_git_branch_guard_kiro_dedicado_script_presente_e_executavel_silencio(self):
        home = _global_guard_home(self)

        script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        _write(script_path, v._GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
        os.chmod(script_path, 0o755)
        _write(
            os.path.join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json"),
            _kiro_global_guard_fixture("trackfw-git-branch-guard", script_path),
        )

        msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(
            msgs, [], f"esperado zero violations com script Kiro presente e executável, obteve: {msgs}"
        )

    def test_kiro_dois_arquivos_dedicados_nao_regride_nao_duplica(self):
        # Não-regressão: com os dois arquivos dedicados do Kiro presentes simultaneamente, cada
        # um referenciando um script ausente distinto, cada regra deve reportar exatamente 1
        # violation -- nunca 0 (regressão) nem 2+ (dupla contagem entre os dois arquivos/guards).
        home = _global_guard_home(self)

        cred_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
        gbg_script_path = os.path.join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
        # Nenhum dos dois scripts é criado -- ambos ausentes propositalmente.
        _write(
            os.path.join(home, ".kiro", "hooks", "trackfw-credential-guard.json"),
            _kiro_global_guard_fixture("trackfw-credential-guard", cred_script_path),
        )
        _write(
            os.path.join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json"),
            _kiro_global_guard_fixture("trackfw-git-branch-guard", gbg_script_path),
        )

        cred_msgs = v.validate_credential_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(
            len(cred_msgs), 1,
            f"esperado exatamente 1 violation (credential-guard do Kiro), obteve {len(cred_msgs)}: {cred_msgs}",
        )

        gbg_msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(
            len(gbg_msgs), 1,
            f"esperado exatamente 1 violation (git-branch-guard do Kiro), obteve {len(gbg_msgs)}: {gbg_msgs}",
        )

    def test_git_branch_guard_kiro_sem_arquivo_dedicado_silencio(self):
        _global_guard_home(self)

        msgs = v.validate_git_branch_guard_global_hook_resolvable(self.tmp)
        self.assertEqual(msgs, [], f"esperado silêncio sem arquivo dedicado do Kiro, obteve: {msgs}")


if __name__ == "__main__":
    unittest.main()
