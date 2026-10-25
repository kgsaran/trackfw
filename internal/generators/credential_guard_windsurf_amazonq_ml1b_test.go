package generators

// credential_guard_windsurf_amazonq_ml1b_test.go — ML-1B (REQ-2026-10-06)
//
// Tests for the Windsurf + Amazon Q credential-guard wiring introduced by ML-1B:
//   Generator: pre_write_code / fs_write get credential guard; pre_read_code /
//              fs_read do NOT; migration of git-branch-only files; idempotence.
//   Harness:   windsurf-credential-guard target creates ~/.codeium/windsurf/hooks.json
//              with credential guard in pre_run_command + pre_write_code; idempotence.
//
// Regra de Reconciliação (CLAUDE.md — Regra Dura de Reconciliação):
// cada teste tem, em seu comentário de abertura, a conclusão do ML que afirma.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// Generator: Windsurf credential guard events (ML-1B)
// ────────────────────────────────────────────────────────────────────────────

// Reconciliação: afirma que InjectWindsurfHooks adiciona o credential guard em
// pre_write_code (evento de escrita de arquivo), além do pre_run_command já coberto
// pelo git-branch guard.
func TestInjectWindsurfHooks_ML1B_PreWriteCodeGetsCredentialGuard(t *testing.T) {
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, ok := data["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected top-level \"hooks\" object, got %v", data["hooks"])
	}

	pre, ok := hooksMap["pre_write_code"].([]interface{})
	if !ok || len(pre) == 0 {
		t.Fatalf("expected pre_write_code to be non-empty after InjectWindsurfHooks, got %v", hooksMap["pre_write_code"])
	}

	found := false
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == guardCredentialCmdPSPOSIX {
			found = true
			if obj["show_output"] != true {
				t.Errorf("expected show_output=true for credential guard in pre_write_code, got %v", obj["show_output"])
			}
		}
	}
	if !found {
		t.Errorf("credential guard command not found in pre_write_code; full event: %v", pre)
	}
}

// Reconciliação: afirma que InjectWindsurfHooks NÃO adiciona nenhum guard em
// pre_read_code — evento que só carrega caminho de arquivo no payload, não conteúdo,
// logo instalar ali seria falsa proteção (Wave 0 residual R2).
func TestInjectWindsurfHooks_ML1B_PreReadCodeGetsNoGuard(t *testing.T) {
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, _ := data["hooks"].(map[string]interface{})
	if v, ok := hooksMap["pre_read_code"]; ok {
		arr, _ := v.([]interface{})
		if len(arr) > 0 {
			t.Errorf("expected pre_read_code to be absent or empty (Wave 0 residual R2), got %v", arr)
		}
	}
}

// Reconciliação: afirma que InjectWindsurfHooks é idempotente em relação ao
// credential guard em pre_write_code — rodar duas vezes não duplica a entrada.
func TestInjectWindsurfHooks_ML1B_PreWriteCodeIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("first InjectWindsurfHooks failed: %v", err)
	}
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("second InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooksMap["pre_write_code"].([]interface{})
	count := 0
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == guardCredentialCmdPSPOSIX {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 credential guard entry in pre_write_code (idempotent), got %d", count)
	}
}

// Reconciliação: afirma que um arquivo .windsurf/hooks.json com apenas o
// git-branch guard (gerado por versão anterior ao ML-1B) recebe o credential
// guard em pre_run_command e pre_write_code após InjectWindsurfHooks, sem perder
// a entrada do git-branch.
func TestInjectWindsurfHooks_ML1B_MigratesGitBranchOnlyFile(t *testing.T) {
	dir := t.TempDir()
	// Simula arquivo gerado antes do ML-1B: só git-branch em pre_run_command.
	helperWriteJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{
				map[string]interface{}{"command": guardGitBranchCmdPSPOSIX, "show_output": true},
			},
		},
	})

	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, _ := data["hooks"].(map[string]interface{})

	// pre_run_command must now have git-branch + credential (order: git-branch first).
	pre, _ := hooksMap["pre_run_command"].([]interface{})
	if len(pre) != 2 {
		t.Fatalf("expected 2 pre_run_command entries (git-branch + credential), got %d: %v", len(pre), pre)
	}
	obj0, _ := pre[0].(map[string]interface{})
	if obj0["command"] != guardGitBranchCmdPSPOSIX {
		t.Errorf("expected git-branch as pre_run_command[0], got %v", obj0["command"])
	}
	obj1, _ := pre[1].(map[string]interface{})
	if obj1["command"] != guardCredentialCmdPSPOSIX {
		t.Errorf("expected credential as pre_run_command[1], got %v", obj1["command"])
	}

	// pre_write_code must have credential.
	write, _ := hooksMap["pre_write_code"].([]interface{})
	found := false
	for _, item := range write {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == guardCredentialCmdPSPOSIX {
			found = true
		}
	}
	if !found {
		t.Errorf("expected credential guard in pre_write_code after migration, not found")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Generator: Amazon Q credential guard events (ML-1B)
// ────────────────────────────────────────────────────────────────────────────

// Reconciliação: afirma que InjectAmazonQHooks adiciona o credential guard no
// matcher fs_write de preToolUse — o evento que carrega o conteúdo do arquivo no
// payload (Layer 1 detecta JWT nele, RC=2).
func TestInjectAmazonQHooks_ML1B_FsWriteGetsCredentialGuard(t *testing.T) {
	dir := t.TempDir()
	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("InjectAmazonQHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"))
	if !helperHasClaudeHook(data, "preToolUse", "fs_write", guardCredentialCmdCmdExe) {
		t.Errorf("hooks.preToolUse[fs_write] missing the credential-guard command (ML-1B); preToolUse: %v", data["hooks"])
	}
}

// Reconciliação: afirma que InjectAmazonQHooks NÃO adiciona nenhum guard no
// matcher fs_read — esse evento só carrega o caminho do arquivo no payload, não o
// conteúdo, logo instalar ali seria falsa proteção (Wave 0 residual R2).
func TestInjectAmazonQHooks_ML1B_FsReadGetsNoGuard(t *testing.T) {
	dir := t.TempDir()
	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("InjectAmazonQHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["preToolUse"].([]interface{})
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] == "fs_read" {
			t.Errorf("fs_read matcher should not be present in preToolUse (Wave 0 residual R2), got: %v", obj)
		}
	}
}

// Reconciliação: afirma que InjectAmazonQHooks é idempotente em relação ao
// credential guard em fs_write — rodar duas vezes não duplica a entrada.
func TestInjectAmazonQHooks_ML1B_FsWriteIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("first InjectAmazonQHooks failed: %v", err)
	}
	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("second InjectAmazonQHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["preToolUse"].([]interface{})
	fsWriteCount := 0
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] == "fs_write" {
			fsWriteCount++
			inner, _ := obj["hooks"].([]interface{})
			credCount := 0
			for _, h := range inner {
				hObj, _ := h.(map[string]interface{})
				if hObj["command"] == guardCredentialCmdCmdExe {
					credCount++
				}
			}
			if credCount != 1 {
				t.Errorf("expected exactly 1 credential guard in fs_write inner hooks (idempotent), got %d", credCount)
			}
		}
	}
	if fsWriteCount != 1 {
		t.Errorf("expected exactly 1 fs_write matcher entry (idempotent across 2 runs), got %d", fsWriteCount)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Harness: windsurf-credential-guard target (ML-1B)
// ────────────────────────────────────────────────────────────────────────────

// Reconciliação: afirma que o target windsurf-credential-guard retorna TargetMissing
// quando ~/.codeium/windsurf/hooks.json não existe e InstallMissing=false.
func TestUpdateHarnessCredentialGuardWindsurfMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"windsurf-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".codeium", "windsurf", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("windsurf-credential-guard was installed without --install-missing: %v", err)
	}
}

// Reconciliação: afirma que o target windsurf-credential-guard cria
// ~/.codeium/windsurf/hooks.json com credential guard em pre_run_command e
// pre_write_code quando InstallMissing=true.
func TestUpdateHarnessCredentialGuardWindsurfInstallsWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"windsurf-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	if report.Targets[0].Path != "~/.codeium/windsurf/hooks.json" {
		t.Fatalf("path = %q, want ~/.codeium/windsurf/hooks.json", report.Targets[0].Path)
	}

	hookPath := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("~/.codeium/windsurf/hooks.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}

	hooks, _ := doc["hooks"].(map[string]interface{})
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		arr, _ := hooks[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["command"] == guardCredentialGlobalCmdPSPOSIX {
				found = true
			}
		}
		if !found {
			t.Errorf("credential global guard not found in %s; full hooks: %v", event, hooks[event])
		}
	}
}

// Reconciliação: afirma que o target windsurf-credential-guard é idempotente —
// rodar duas vezes com InstallMissing=true produz TargetSkipped na segunda vez e
// não duplica entradas.
func TestUpdateHarnessCredentialGuardWindsurfIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"windsurf-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	firstContent, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"windsurf-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already current)", report.Targets[0].State)
	}
	secondContent, _ := os.ReadFile(hookPath)
	if string(firstContent) != string(secondContent) {
		t.Error("file changed on second run (idempotence violated)")
	}
}

// D15: ML-1D tests (globalCredentialGuardInstalledWindsurf hardening,
// writeWindsurfGlobalHooksML1D helper, and the three tests that used it)
// have been removed. Under D15, InjectWindsurfHooks always emits both
// pre_run_command and pre_write_code unconditionally; the global file is
// never consulted for project wiring decisions.
