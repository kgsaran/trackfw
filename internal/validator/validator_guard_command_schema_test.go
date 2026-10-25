package validator

// validator_guard_command_schema_test.go — ML-1A / ML-1A2 (REQ-2026-10-25, D16)
//
// Regra Dura de Reconciliação (CLAUDE.md): cada teste declara em um comentário prefixado
// "Reconciliação:" a conclusão do ML que afirma — em uma frase.
//
// Estes testes verificam que:
// (a) todo arquivo acessível pelos iteradores de global-guard tem um schema de extração
//     declarado em guardGlobalHookLayout;
// (b) todo arquivo em credentialGuardHookFiles tem um schema de extração declarado em
//     guardWiringHookLayout;
// (c) nenhuma função de produção no pacote validator percorre o JSON do hook casando
//     substring em qualquer chave (proibido por D16; detectado por grep na fonte).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuardGlobalHookLayout_CobreTodasAsEntradas afirma: todo caminho acessível por
// globalGuardConfigPath, para qualquer marcador (credentialGuardScriptMarker e
// gitBranchGuardScriptMarker), tem uma entrada correspondente em guardGlobalHookLayout.
//
// Reconciliação: D16 (ADR-2026-10-04) exige que cada arquivo global tenha um schema declarado;
// sem ele, collectCommandsWithMarkerBySchema recebe iota=0 (hookCmdSchemaNested), tornando
// CLIs com schemas diferentes (Cursor=hookCmdSchemaFlat, Copilot=hookCmdSchemaFlatBash,
// Kiro=hookCmdSchemaKiro, Windsurf=hookCmdSchemaFlat) invisíveis ou incorretamente avaliados.
//
// Prova de mordida: remover qualquer entrada de guardGlobalHookLayout → este teste falha para
// esse caminho com a mensagem "has no extraction schema in guardGlobalHookLayout".
func TestGuardGlobalHookLayout_CobreTodasAsEntradas(t *testing.T) {
	markers := []string{credentialGuardScriptMarker, gitBranchGuardScriptMarker}
	for _, gf := range globalGuardConfigFiles {
		for _, marker := range markers {
			relPath := globalGuardConfigPath(gf, marker)
			if _, ok := guardGlobalHookLayout[relPath]; !ok {
				t.Errorf(
					"globalGuardConfigFiles entry %q (CLI=%q, marker=%q) resolves to path %q "+
						"which has no extraction schema in guardGlobalHookLayout — add one (D16)",
					gf.path, gf.cli, marker, relPath,
				)
			}
		}
	}
}

// TestGuardRecursivoAusente_CodigoFonte verifica que nenhum arquivo de produção do pacote
// validator define a função `collectCommandsWithMarker` (a antiga varredura recursiva
// cega-de-chave substituída por D16).
//
// Reconciliação: D16 (ADR-2026-10-04) remove o coletor recursivo que permitia que um marcador
// em qualquer campo de string (e.g. "note", "autoMode.environment") fosse contado como fiação;
// se a função for reintroduzida, a propriedade AC2 (todos os sítios usam coleta restrita) deixa
// de valer — este teste impede isso.
//
// Prova de mordida (contra-braço): origin/main define `collectCommandsWithMarker` em
// validator_credential_guard.go:332; um grep nesse arquivo encontra a definição e o teste
// teria falhado lá (medido na seção 6 do relatório ML-1A2).
//
// Nota (R1, ML-2C): este teste é um pin de nome — detecta apenas a reintrodução com o nome exato
// "collectCommandsWithMarker". Uma reintrodução com outro nome não é detectada aqui. Os testes
// comportamentais AC3/AC4 (TestCredentialGuardPresenceRequired_Windsurf_MarkerSoEmNote_Violation,
// TestCredentialGuardPresenceRequired_AmazonQ_MarkerSoEmNote_Violation,
// TestCredentialGuardHookResolvable_PermissionsAllowNaoGeraViolation) são a barreira real — um
// coletor recursivo reintroduzido sob qualquer nome quebraria esses testes antes de chegar aqui.
func TestGuardRecursivoAusente_CodigoFonte(t *testing.T) {
	// Localiza o diretório do pacote validator relativo ao cwd do teste.
	// Em `go test ./internal/validator/`, o cwd é a raiz do módulo ou a pasta do pacote;
	// usamos filepath.Dir(os.Args[0]) não é confiável em testes, então procuramos a partir
	// de GOPATH/GOMODCACHE não necessários — basta subir até achar o go.mod.
	pkgDir := findValidatorPackageDir(t)

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", pkgDir, err)
	}

	const forbidden = "func collectCommandsWithMarker("
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(pkgDir, name)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("ReadFile(%q): %v", path, readErr)
			continue
		}
		if strings.Contains(string(content), forbidden) {
			t.Errorf(
				"AC2 (D16): file %q defines %q — the old recursive key-blind scanner removed by "+
					"ML-1A; remove this definition and use collectCommandsWithMarkerBySchema instead",
				name, forbidden,
			)
		}
	}
}

// findValidatorPackageDir resolves the on-disk path of internal/validator by walking up
// from the current working directory to find the go.mod file, then appending the package path.
func findValidatorPackageDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk upward until we find go.mod.
	dir := cwd
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return filepath.Join(dir, "internal", "validator")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod starting from %q", cwd)
		}
		dir = parent
	}
}

// --------------------------------------------------------------------------
// AC3 — Project false-positive: marker outside hooks does not generate
// violation or warning in validateGuardHookResolvable (D16)
// --------------------------------------------------------------------------

// projectClaudeSettingsWithPermissionsAllow builds a .claude/settings.json where the
// credential-guard subcommand marker appears only in permissions.allow — not in hooks.
func projectClaudeSettingsWithPermissionsAllow() string {
	return `{
  "permissions": {
    "allow": [
      "Bash(` + credentialGuardSubcmdMarker + ` --check)",
      "Bash(echo hello)"
    ]
  }
}`
}

// projectClaudeSettingsWithEnvMarker builds a .claude/settings.json where the
// credential-guard script marker appears in an env map value — not in hooks.
func projectClaudeSettingsWithEnvMarker() string {
	return `{
  "env": {
    "GUARD_NOTE": "this project uses ` + credentialGuardScriptMarker + ` for credentials"
  }
}`
}

// projectClaudeSettingsWithCurrentHookAndPermissionsDecoy builds a .claude/settings.json
// with a correct current-form hook AND the script marker in permissions.allow (decoy).
func projectClaudeSettingsWithCurrentHookAndPermissionsDecoy() string {
	// Use the nested schema that Claude Code expects:
	// hooks.PreToolUse[{matcher, hooks:[{type:command, command:<line>}]}]
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|PowerShell",
        "hooks": [
          {
            "type": "command",
            "command": "` + credentialGuardSubcmdMarker + ` || exit 2; exit $LASTEXITCODE"
          }
        ]
      }
    ]
  },
  "permissions": {
    "allow": [
      "Bash(` + credentialGuardScriptMarker + ` --audit)"
    ]
  }
}`
}

// projectClaudeSettingsWithRealHookAndPermissionsDecoy builds a .claude/settings.json
// where the hook command contains the credential-guard script marker (triggering a
// script-not-found violation) AND the marker is also in permissions.allow (decoy).
// Used to prove the real hook is still detected: the violation fires from the hook command,
// not from the decoy in permissions.allow.
func projectClaudeSettingsWithRealHookAndPermissionsDecoy() string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|PowerShell",
        "hooks": [
          {
            "type": "command",
            "command": "scripts/` + credentialGuardScriptMarker + `"
          }
        ]
      }
    ]
  },
  "permissions": {
    "allow": [
      "Bash(` + credentialGuardScriptMarker + ` --audit)"
    ]
  }
}`
}

// TestCredentialGuardHookResolvable_PermissionsAllowNaoGeraViolation — AC3 project false-positive
//
// Reconciliação: D16 schema-aware search — the credential-guard subcommand marker in
// permissions.allow of a project .claude/settings.json does NOT generate a violation;
// only the command field of a hook entry in the hooks tree counts as wiring.
//
// Prova de mordida: the old collectCommandsWithMarker traversed all JSON strings; a marker
// in permissions.allow triggered a validation of the command path, producing a violation.
func TestCredentialGuardHookResolvable_PermissionsAllowNaoGeraViolation(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".claude/settings.json", projectClaudeSettingsWithPermissionsAllow())

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The marker is in permissions.allow — not a hook command. Expected: no violation.
	// If a violation referencing .claude/settings.json appears, D16 is not enforced.
	for _, m := range msgs {
		if strings.Contains(m, ".claude/settings.json") {
			// Only fail if it is a "hook not resolvable" or "references" message,
			// not an unrelated rule (e.g. malformed JSON).
			if strings.Contains(m, "references") || strings.Contains(m, "not found") ||
				strings.Contains(m, "not executable") || strings.Contains(m, "resolved to") {
				t.Errorf(
					"AC3: unexpected violation for .claude/settings.json when marker is only "+
						"in permissions.allow — D16 should ignore non-hook fields: %s", m,
				)
			}
		}
	}
}

// TestCredentialGuardHookResolvable_EnvMarkerNaoGeraViolation — AC3 project false-positive
//
// Reconciliação: D16 schema-aware search — the credential-guard script marker in an env map
// value of a project .claude/settings.json does NOT generate a hook-resolvable violation.
func TestCredentialGuardHookResolvable_EnvMarkerNaoGeraViolation(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".claude/settings.json", projectClaudeSettingsWithEnvMarker())

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, ".claude/settings.json") &&
			(strings.Contains(m, "references") || strings.Contains(m, "not found") ||
				strings.Contains(m, "not executable") || strings.Contains(m, "resolved to")) {
			t.Errorf(
				"AC3: unexpected violation for .claude/settings.json when marker is only "+
					"in env map — D16 should ignore non-hook fields: %s", m,
			)
		}
	}
}

// TestCredentialGuardHookResolvable_PermissionsAllowDecoyRealHookAindaDetectado — AC3 positive arm
//
// Reconciliação: D16 schema-aware search — with the real guard script marker in the hook command
// (triggering a script-not-found or cwd-dependent violation) AND the same marker as a decoy in
// permissions.allow, the validator still detects the real hook issue; the decoy does not mask it.
//
// This uses a differential assertion:
//
//	arm A — correct subcommand hook + permissions decoy → zero violations from the hook
//	arm B — relative .sh hook (cwd-dependent) + permissions decoy → violation fires from hook
//
// In both arms, the decoy in permissions.allow is NOT the source of any violation.
func TestCredentialGuardHookResolvable_PermissionsAllowDecoyRealHookAindaDetectado(t *testing.T) {
	t.Run("correct subcommand hook + permissions decoy = zero hook violations", func(t *testing.T) {
		isolateHome(t)
		dir := t.TempDir()
		chdir(t, dir)
		stubProbeOK(t)
		writeFile(t, dir, ".claude/settings.json", projectClaudeSettingsWithCurrentHookAndPermissionsDecoy())

		msgs, err := validateCredentialGuardHookResolvable()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, m := range msgs {
			if strings.Contains(m, ".claude/settings.json") &&
				(strings.Contains(m, "references") || strings.Contains(m, "not found") ||
					strings.Contains(m, "not executable") || strings.Contains(m, "resolved to")) {
				t.Errorf("AC3 positive arm: unexpected violation with correct subcommand hook + decoy: %s", m)
			}
		}
	})

	t.Run("relative .sh hook + permissions decoy = violation from hook fires, not from decoy", func(t *testing.T) {
		isolateHome(t)
		dir := t.TempDir()
		chdir(t, dir)
		stubProbeOK(t)
		// The hook command is "scripts/<marker>" — a relative (cwd-dependent) path.
		// This triggers a violation from validateGuardHookResolvable (cwd-dependent path).
		// The decoy in permissions.allow must NOT contribute an additional violation.
		writeFile(t, dir, ".claude/settings.json", projectClaudeSettingsWithRealHookAndPermissionsDecoy())

		msgs, err := validateCredentialGuardHookResolvable()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Expect a violation from the hook command (cwd-dependent relative path).
		// hasViolation checks by substring, so any violation citing .claude/settings.json qualifies.
		if !hasViolation(msgs, ".claude/settings.json") {
			t.Errorf(
				"AC3 positive arm: expected violation for .claude/settings.json when hook has "+
					"relative .sh path (real detection still works); got: %v", msgs,
			)
		}
	})
}
