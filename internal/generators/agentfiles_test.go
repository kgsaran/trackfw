package generators

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func helperWriteJSON(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	if err := os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func helperReadJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile %s: %v", path, err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func helperHasClaudeHook(data map[string]interface{}, event, matcher, command string) bool {
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		return false
	}
	arr, _ := hooks[event].([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				return true
			}
		}
	}
	return false
}

// --- Claude ---

func TestInjectClaudeHooks_Create(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse[AskUserQuestion] → signal.sh missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse[AskUserQuestion] → cleanup.sh missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Bash] → credential-guard.sh missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse[Bash] → credential-guard.sh missing")
	}
}

func TestInjectClaudeHooks_MergeAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	existing := map[string]interface{}{
		"permissions": map[string]interface{}{"defaultMode": "default"},
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/other.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), existing)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("first InjectClaudeHooks failed: %v", err)
	}
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("second InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/other.sh") {
		t.Error("existing Bash hook lost during merge")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse signal hook missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse credential-guard hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse cleanup hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse credential-guard hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pr, _ := hooks["PreToolUse"].([]interface{})
	// A pre-existing "Bash" matcher entry (third-party hook) must be merged with
	// (not duplicated by) the new credential-guard "Bash" entry: 4 entries total
	// -- {Bash: [other.sh, credential-guard.sh]}, {AskUserQuestion: [signal.sh]},
	// {Read: [credential-guard.sh]}, {Write|Edit: [credential-guard.sh]}
	// (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(pr) != 4 {
		t.Errorf("expected 4 PreToolUse entries, got %d", len(pr))
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	if len(post) != 4 {
		t.Errorf("expected 4 PostToolUse entries, got %d", len(post))
	}
}

// TestInjectClaudeHooks_MigratesLegacyRelativeCredentialGuardCommand cobre o bug reportado em
// produção (2026-08-09, projeto CMDB): o comando do credential-guard era um caminho relativo puro
// ("scripts/trackfw-credential-guard.sh"), que o Claude Code resolve contra o cwd *dinâmico* do
// hook (rastreia `cd`s feitos pelo agente durante a sessão), não a raiz do projeto -- qualquer
// chamada Bash/Read/Write/Edit depois de um `cd` para um subdiretório falhava com "No such file or
// directory". Este teste confirma que re-rodar InjectClaudeHooks sobre um settings.json já escrito
// por uma versão antiga REESCREVE o comando legado para a forma fixa
// ($CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh) em vez de só acrescentar um segundo
// hook ao lado do quebrado.
func TestInjectClaudeHooks_MigratesLegacyRelativeCredentialGuardCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	legacy := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"}},
				},
				map[string]interface{}{
					"matcher": "Read",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"}},
				},
			},
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Write|Edit",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), legacy)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	hooks, _ := data["hooks"].(map[string]interface{})

	if helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("stale relative-path PreToolUse[Bash] entry survived the upgrade -- should have been rewritten, not left in place")
	}
	if helperHasClaudeHook(data, "PreToolUse", "Read", "scripts/trackfw-credential-guard.sh") {
		t.Error("stale relative-path PreToolUse[Read] entry survived the upgrade")
	}
	if helperHasClaudeHook(data, "PostToolUse", "Write|Edit", "scripts/trackfw-credential-guard.sh") {
		t.Error("stale relative-path PostToolUse[Write|Edit] entry survived the upgrade")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Bash] was not upgraded to the $CLAUDE_PROJECT_DIR-prefixed command")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Read", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Read] was not upgraded to the $CLAUDE_PROJECT_DIR-prefixed command")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Write|Edit", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse[Write|Edit] was not upgraded to the $CLAUDE_PROJECT_DIR-prefixed command")
	}

	// No duplicate hooks left behind inside the migrated matcher entries: exactly
	// one credential-guard command per matcher after the rewrite, not two (old +
	// new side by side). PreToolUse[Bash] additionally carries the git-branch-guard
	// command (ROADMAP-2026-08-14 ML-3A) merged into the same matcher entry, so its
	// expected count is 2 (credential-guard + git-branch-guard), not 1.
	pre, _ := hooks["PreToolUse"].([]interface{})
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		innerHooks, _ := obj["hooks"].([]interface{})
		switch obj["matcher"] {
		case "Bash":
			if len(innerHooks) != 2 {
				t.Errorf("PreToolUse[Bash] expected exactly 2 hooks (credential-guard + git-branch-guard) after migration, got %d", len(innerHooks))
			}
		case "Read":
			if len(innerHooks) != 1 {
				t.Errorf("PreToolUse[Read] expected exactly 1 hook after migration, got %d", len(innerHooks))
			}
		}
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", claudeGitGuardCmd) {
		t.Error("PreToolUse[Bash] missing the git-branch-guard command")
	}
}

// TestInjectClaudeHooks_MigratesLegacyRelativeAttentionSignalCleanupCommand cobre o ROADMAP-2026-08-11
// ML-2A: assim como o credential-guard (ML anterior, teste acima), a checagem invoca o injector real
// contra um fixture com a string relativa antiga e assevera que a entrada é reescrita in-place --
// não duplicada -- para $CLAUDE_PROJECT_DIR/scripts/trackfw-attention-{signal,cleanup}.sh.
func TestInjectClaudeHooks_MigratesLegacyRelativeAttentionSignalCleanupCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	legacy := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "AskUserQuestion",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/trackfw-attention-signal.sh"}},
				},
			},
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "AskUserQuestion",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/trackfw-attention-cleanup.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), legacy)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	hooks, _ := data["hooks"].(map[string]interface{})

	if helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("stale relative-path PreToolUse[AskUserQuestion] signal entry survived the upgrade -- should have been rewritten, not left in place")
	}
	if helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("stale relative-path PostToolUse[AskUserQuestion] cleanup entry survived the upgrade")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse[AskUserQuestion] was not upgraded to the $CLAUDE_PROJECT_DIR-prefixed signal command")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse[AskUserQuestion] was not upgraded to the $CLAUDE_PROJECT_DIR-prefixed cleanup command")
	}

	// No duplicate hooks left behind inside the migrated matcher entries: exactly one command per
	// matcher after the rewrite, not two (old + new side by side).
	pre, _ := hooks["PreToolUse"].([]interface{})
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] != "AskUserQuestion" {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		if len(innerHooks) != 1 {
			t.Errorf("PreToolUse[AskUserQuestion] expected exactly 1 hook after migration, got %d", len(innerHooks))
		}
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	for _, item := range post {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] != "AskUserQuestion" {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		if len(innerHooks) != 1 {
			t.Errorf("PostToolUse[AskUserQuestion] expected exactly 1 hook after migration, got %d", len(innerHooks))
		}
	}
}

// TestInjectClaudeHooks_ReadWriteEditMatchersRegisteredForCredentialGuard cobre o cenário (b) da
// REQ-2026-08-08 (ML-3A): um payload de tool call Read/Write/Edit (não Bash) contendo JWT/AWS key
// no tool_input só é interceptado pelo credential-guard se o hook estiver REGISTRADO para esses
// matchers -- este teste confirma o wiring (structural), não a execução do script (já coberta
// pelos cenários (a)/(c) em credential_guard_test.go, já que o script escaneia o payload cru
// independente do tool_name).
func TestInjectClaudeHooks_ReadWriteEditMatchersRegisteredForCredentialGuard(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", "Read", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Read] → credential-guard.sh missing (Read tool calls never reach the guard without this entry)")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Read", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse[Read] → credential-guard.sh missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Write|Edit", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Write|Edit] → credential-guard.sh missing (Write/Edit tool calls never reach the guard without this entry)")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Write|Edit", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse[Write|Edit] → credential-guard.sh missing")
	}
}

// TestInjectGeminiHooks_ReadWriteMatchersRegisteredForCredentialGuard é a contraparte Gemini do
// cenário (b) -- matchers read_file|read_many_files / write_file|replace, tabela da ADR-2026-08-06
// emenda 7.
func TestInjectGeminiHooks_ReadWriteMatchersRegisteredForCredentialGuard(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))

	if !helperHasClaudeHook(data, "BeforeTool", "read_file|read_many_files", geminiGuardCmd) {
		t.Error("BeforeTool[read_file|read_many_files] → credential-guard.sh missing")
	}
	if !helperHasClaudeHook(data, "AfterTool", "read_file|read_many_files", geminiGuardCmd) {
		t.Error("AfterTool[read_file|read_many_files] → credential-guard.sh missing")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "write_file|replace", geminiGuardCmd) {
		t.Error("BeforeTool[write_file|replace] → credential-guard.sh missing")
	}
	if !helperHasClaudeHook(data, "AfterTool", "write_file|replace", geminiGuardCmd) {
		t.Error("AfterTool[write_file|replace] → credential-guard.sh missing")
	}
}

// --- Codex ---

func TestInjectCodexHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks failed: %v", err)
	}
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("second InjectCodexHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	if !helperHasClaudeHook(data, "PermissionRequest", ".*", codexSignalCmd) {
		t.Error("Codex PermissionRequest hook missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", codexGuardCmd) {
		t.Error("Codex PreToolUse[Bash] credential-guard hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", ".*", codexCleanupCmd) {
		t.Error("Codex PostToolUse hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", codexGuardCmd) {
		t.Error("Codex PostToolUse[Bash] credential-guard hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["PreToolUse"].([]interface{})
	// 2 entries: {matcher:"Bash", hooks:[credential-guard.sh]}, {matcher:"apply_patch",
	// hooks:[credential-guard.sh]} (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2;
	// Codex has no read-tool matcher — see InjectCodexHooks doc comment).
	if len(pre) != 2 {
		t.Errorf("expected 2 PreToolUse entries (Bash + apply_patch, no idempotency dup), got %d", len(pre))
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	// 3 entries: {matcher:".*", hooks:[cleanup.sh]}, {matcher:"Bash", hooks:[credential-guard.sh]},
	// {matcher:"apply_patch", hooks:[credential-guard.sh]}
	if len(post) != 3 {
		t.Errorf("expected 3 PostToolUse entries, got %d", len(post))
	}
}

func TestInjectCodexHooks_PreservesExistingBashEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/other.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".codex", "hooks.json"), existing)

	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks failed: %v", err)
	}
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("second InjectCodexHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/other.sh") {
		t.Error("existing Bash hook lost during merge")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", codexGuardCmd) {
		t.Error("PreToolUse[Bash] credential-guard hook missing after merge")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["PreToolUse"].([]interface{})
	// 2 entries: {matcher:"Bash", hooks:[other.sh, credential-guard.sh]} (merged),
	// {matcher:"apply_patch", hooks:[credential-guard.sh]}.
	if len(pre) != 2 {
		t.Errorf("expected 2 PreToolUse entries (Bash merged + apply_patch), got %d", len(pre))
	}
}

// TestInjectCodexHooks_MigrationWiringRewritesInPlaceNotDuplicate covers the ML-1A migration
// wiring added to InjectCodexHooks (migrateHookCommand, called before mergeClaudeHookArray for
// every trackfw-owned matcher), now exercised as a genuine migration (ROADMAP-2026-08-11 ML-3A):
// this fixture pre-populates every trackfw-owned matcher with the pre-ML-3A relative-path command,
// exactly as an older trackfw run would have left it, and asserts the injector rewrites each entry
// to the new $(git rev-parse --show-toplevel)-pinned command in place instead of appending a
// second, still-cwd-fragile entry alongside it.
func TestInjectCodexHooks_MigrationWiringRewritesInPlaceNotDuplicate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	mk := func(matcher, command string) map[string]interface{} {
		return map[string]interface{}{
			"matcher": matcher,
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": command}},
		}
	}
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PermissionRequest": []interface{}{mk(".*", "scripts/trackfw-attention-signal.sh")},
			"PreToolUse": []interface{}{
				mk("Bash", "scripts/trackfw-credential-guard.sh"),
				mk("apply_patch", "scripts/trackfw-credential-guard.sh"),
			},
			"PostToolUse": []interface{}{
				mk(".*", "scripts/trackfw-attention-cleanup.sh"),
				mk("Bash", "scripts/trackfw-credential-guard.sh"),
				mk("apply_patch", "scripts/trackfw-credential-guard.sh"),
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".codex", "hooks.json"), existing)

	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	hooks, _ := data["hooks"].(map[string]interface{})

	// wantHooks: PreToolUse[Bash] also carries the git-branch-guard command
	// (ROADMAP-2026-08-14 ML-3A), merged into the same matcher entry, so its
	// expected hook count is 2 instead of 1 for every other matcher here.
	checkOne := func(event, matcher, command string, wantHooks int) {
		arr, _ := hooks[event].([]interface{})
		count := 0
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["matcher"] != matcher {
				continue
			}
			count++
			innerHooks, _ := obj["hooks"].([]interface{})
			if len(innerHooks) != wantHooks {
				t.Errorf("%s[%s]: expected exactly %d hook(s), got %d", event, matcher, wantHooks, len(innerHooks))
			}
		}
		if count != 1 {
			t.Errorf("%s[%s]: expected exactly 1 matcher entry (no duplicate), got %d", event, matcher, count)
		}
		if !helperHasClaudeHook(data, event, matcher, command) {
			t.Errorf("%s[%s]: expected command %q missing", event, matcher, command)
		}
	}
	checkOne("PermissionRequest", ".*", codexSignalCmd, 1)
	checkOne("PreToolUse", "Bash", codexGuardCmd, 2)
	checkOne("PreToolUse", "apply_patch", codexGuardCmd, 1)
	checkOne("PostToolUse", ".*", codexCleanupCmd, 1)
	checkOne("PostToolUse", "Bash", codexGuardCmd, 1)
	checkOne("PostToolUse", "apply_patch", codexGuardCmd, 1)
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", codexGitGuardCmd) {
		t.Error("PreToolUse[Bash]: expected git-branch-guard command missing")
	}
}

// --- Gemini ---

func TestInjectGeminiHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("second InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	if !helperHasClaudeHook(data, "Notification", "ToolPermission", geminiSignalCmd) {
		t.Error("Gemini Notification hook missing")
	}
	if !helperHasClaudeHook(data, "AfterTool", "*", geminiCleanupCmd) {
		t.Error("Gemini AfterTool[*] cleanup hook missing")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", geminiGuardCmd) {
		t.Error("Gemini BeforeTool[run_shell_command] credential-guard hook missing")
	}
	if !helperHasClaudeHook(data, "AfterTool", "run_shell_command", geminiGuardCmd) {
		t.Error("Gemini AfterTool[run_shell_command] credential-guard hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	before, _ := hooks["BeforeTool"].([]interface{})
	// 3 entries: {matcher:"run_shell_command", ...}, {matcher:"read_file|read_many_files", ...},
	// {matcher:"write_file|replace", ...} (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(before) != 3 {
		t.Errorf("expected 3 BeforeTool entries (run_shell_command + read + write, no idempotency dup), got %d", len(before))
	}
	after, _ := hooks["AfterTool"].([]interface{})
	// 4 entries: {matcher:"*", hooks:[cleanup.sh]}, {matcher:"run_shell_command", ...},
	// {matcher:"read_file|read_many_files", ...}, {matcher:"write_file|replace", ...}
	if len(after) != 4 {
		t.Errorf("expected 4 AfterTool entries, got %d", len(after))
	}
}

func TestInjectGeminiHooks_PreservesExistingBeforeToolEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"BeforeTool": []interface{}{
				map[string]interface{}{
					"matcher": "run_shell_command",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/other.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), existing)

	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("second InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", "scripts/other.sh") {
		t.Error("existing BeforeTool[run_shell_command] hook lost during merge")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", geminiGuardCmd) {
		t.Error("BeforeTool[run_shell_command] credential-guard hook missing after merge")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	before, _ := hooks["BeforeTool"].([]interface{})
	// 3 entries: {matcher:"run_shell_command", hooks:[other.sh, credential-guard.sh]} (merged),
	// {matcher:"read_file|read_many_files", ...}, {matcher:"write_file|replace", ...}.
	if len(before) != 3 {
		t.Errorf("expected 3 BeforeTool entries (run_shell_command merged + read + write), got %d", len(before))
	}
}

// TestInjectGeminiHooks_MigrationWiringRewritesInPlaceNotDuplicate is the Gemini counterpart of
// TestInjectCodexHooks_MigrationWiringRewritesInPlaceNotDuplicate. ML-4A flipped
// migrateHookCommand's oldCommand argument to the pre-ML-4A relative literal, so this fixture (an
// old settings.json written by a pre-ML-4A trackfw) now exercises a genuine migration: the old
// relative-path entries must be rewritten in place to $GEMINI_PROJECT_DIR/... form, not duplicated.
func TestInjectGeminiHooks_MigrationWiringRewritesInPlaceNotDuplicate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	mk := func(matcher, command string) map[string]interface{} {
		return map[string]interface{}{
			"matcher": matcher,
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": command}},
		}
	}
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"Notification": []interface{}{mk("ToolPermission", "scripts/trackfw-attention-signal.sh")},
			"BeforeTool": []interface{}{
				mk("run_shell_command", "scripts/trackfw-credential-guard.sh"),
				mk("read_file|read_many_files", "scripts/trackfw-credential-guard.sh"),
				mk("write_file|replace", "scripts/trackfw-credential-guard.sh"),
			},
			"AfterTool": []interface{}{
				mk("*", "scripts/trackfw-attention-cleanup.sh"),
				mk("run_shell_command", "scripts/trackfw-credential-guard.sh"),
				mk("read_file|read_many_files", "scripts/trackfw-credential-guard.sh"),
				mk("write_file|replace", "scripts/trackfw-credential-guard.sh"),
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), existing)

	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	hooks, _ := data["hooks"].(map[string]interface{})

	// wantHooks: BeforeTool[run_shell_command] also carries the
	// git-branch-guard command (ROADMAP-2026-08-14 ML-3A), merged into the
	// same matcher entry, so its expected hook count is 2 instead of 1.
	checkOne := func(event, matcher, command string, wantHooks int) {
		arr, _ := hooks[event].([]interface{})
		count := 0
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["matcher"] != matcher {
				continue
			}
			count++
			innerHooks, _ := obj["hooks"].([]interface{})
			if len(innerHooks) != wantHooks {
				t.Errorf("%s[%s]: expected exactly %d hook(s), got %d", event, matcher, wantHooks, len(innerHooks))
			}
		}
		if count != 1 {
			t.Errorf("%s[%s]: expected exactly 1 matcher entry (no duplicate), got %d", event, matcher, count)
		}
		if !helperHasClaudeHook(data, event, matcher, command) {
			t.Errorf("%s[%s]: expected command %q missing", event, matcher, command)
		}
	}
	checkOne("Notification", "ToolPermission", geminiSignalCmd, 1)
	checkOne("BeforeTool", "run_shell_command", geminiGuardCmd, 2)
	checkOne("BeforeTool", "read_file|read_many_files", geminiGuardCmd, 1)
	checkOne("BeforeTool", "write_file|replace", geminiGuardCmd, 1)
	checkOne("AfterTool", "*", geminiCleanupCmd, 1)
	checkOne("AfterTool", "run_shell_command", geminiGuardCmd, 1)
	checkOne("AfterTool", "read_file|read_many_files", geminiGuardCmd, 1)
	checkOne("AfterTool", "write_file|replace", geminiGuardCmd, 1)
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", geminiGitGuardCmd) {
		t.Error("BeforeTool[run_shell_command]: expected git-branch-guard command missing")
	}
}

// --- Kiro ---

func TestInjectKiroHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks failed: %v", err)
	}
	file := filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json")
	content1, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("second InjectKiroHooks failed: %v", err)
	}
	content2, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("second ReadFile failed: %v", err)
	}

	if !bytes.Equal(content1, content2) {
		t.Fatalf("expected Kiro config content to be identical after 2nd injection")
	}

	data := helperReadJSON(t, file)
	if v, _ := data["version"].(string); v != "v1" {
		t.Fatalf("expected version \"v1\", got %v", data["version"])
	}
	hooks, _ := data["hooks"].([]interface{})
	// 8 entries: signal, cleanup, credential-guard shell pre/post, read pre/post,
	// write pre/post (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(hooks) != 8 {
		t.Fatalf("expected 8 hooks in Kiro config (signal, cleanup, credential-guard shell/read/write pre/post), got %d", len(hooks))
	}

	sawGuardPre, sawGuardPost := false, false
	for _, h := range hooks {
		entry, _ := h.(map[string]interface{})
		if entry == nil {
			continue
		}
		if _, hasEvent := entry["event"]; hasEvent {
			t.Fatalf("hook entry uses legacy \"event\" field, expected \"trigger\": %v", entry)
		}
		trigger, _ := entry["trigger"].(string)
		if trigger == "" {
			t.Fatalf("hook entry missing \"trigger\": %v", entry)
		}
		if _, isObject := entry["matcher"].(map[string]interface{}); isObject {
			t.Fatalf("hook entry uses object matcher, expected plain regex string: %v", entry)
		}
		name, _ := entry["name"].(string)
		switch name {
		case "trackfw-credential-guard-pre":
			sawGuardPre = true
			if trigger != "PreToolUse" {
				t.Fatalf("expected credential-guard-pre trigger PreToolUse, got %q", trigger)
			}
			if m, _ := entry["matcher"].(string); m != "shell" {
				t.Fatalf("expected credential-guard-pre matcher \"shell\", got %q", m)
			}
		case "trackfw-credential-guard-post":
			sawGuardPost = true
			if trigger != "PostToolUse" {
				t.Fatalf("expected credential-guard-post trigger PostToolUse, got %q", trigger)
			}
		}
	}
	if !sawGuardPre || !sawGuardPost {
		t.Fatalf("expected both credential-guard pre and post hooks, got pre=%v post=%v", sawGuardPre, sawGuardPost)
	}
}

// --- Copilot ---

func TestInjectCopilotHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks failed: %v", err)
	}
	file := filepath.Join(dir, ".github", "hooks", "trackfw-attention.json")
	content1, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("second InjectCopilotHooks failed: %v", err)
	}
	content2, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("second ReadFile failed: %v", err)
	}

	if !bytes.Equal(content1, content2) {
		t.Fatalf("expected Copilot config content to be identical after 2nd injection")
	}

	data := helperReadJSON(t, file)
	if data["version"] != float64(1) {
		t.Fatalf("expected version 1, got %v", data["version"])
	}
	hooks, ok := data["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected hooks to be an object keyed by event, got %v", data["hooks"])
	}

	// preToolUse: 5 entries -- signal + credential-guard "bash"/"view"/"create|edit"
	// (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2) + git-branch-guard "bash"
	// (ROADMAP-2026-08-14 ML-3A). postToolUse stays at 4 (git-branch-guard is
	// PreToolUse-only, see InjectCopilotHooks doc comment).
	pre, ok := hooks["preToolUse"].([]interface{})
	if !ok || len(pre) != 5 {
		t.Fatalf("expected preToolUse array of size 5, got %v", hooks["preToolUse"])
	}
	post, ok := hooks["postToolUse"].([]interface{})
	if !ok || len(post) != 4 {
		t.Fatalf("expected postToolUse array of size 4, got %v", hooks["postToolUse"])
	}

	helperFindCopilotEntry := func(arr []interface{}, bash string) map[string]interface{} {
		for _, item := range arr {
			obj, ok := item.(map[string]interface{})
			if ok && obj["bash"] == bash {
				return obj
			}
		}
		return nil
	}

	signal := helperFindCopilotEntry(pre, "scripts/trackfw-attention-signal.sh")
	if signal == nil {
		t.Fatal("preToolUse missing attention-signal entry")
	}
	if signal["matcher"] != nil {
		t.Errorf("attention-signal entry should not have a matcher, got %v", signal["matcher"])
	}

	guardPre := helperFindCopilotEntry(pre, "scripts/trackfw-credential-guard.sh")
	if guardPre == nil {
		t.Fatal("preToolUse missing credential-guard entry")
	}
	if guardPre["matcher"] != "bash" {
		t.Errorf("credential-guard preToolUse entry should have matcher=bash, got %v", guardPre["matcher"])
	}

	cleanup := helperFindCopilotEntry(post, "scripts/trackfw-attention-cleanup.sh")
	if cleanup == nil {
		t.Fatal("postToolUse missing attention-cleanup entry")
	}

	guardPost := helperFindCopilotEntry(post, "scripts/trackfw-credential-guard.sh")
	if guardPost == nil {
		t.Fatal("postToolUse missing credential-guard entry")
	}
	if guardPost["matcher"] != "bash" {
		t.Errorf("credential-guard postToolUse entry should have matcher=bash, got %v", guardPost["matcher"])
	}
}

// --- Cursor ---

func TestInjectCursorHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("second InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	if _, ok := data["preToolUse"]; ok {
		t.Errorf("expected no top-level preToolUse key (legacy schema), got %v", data["preToolUse"])
	}
	if _, ok := data["postToolUse"]; ok {
		t.Errorf("expected no top-level postToolUse key (legacy schema), got %v", data["postToolUse"])
	}

	if data["version"] != float64(1) {
		t.Errorf("expected version=1, got %v", data["version"])
	}
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatalf("expected top-level hooks object, got none")
	}

	pre, _ := hooks["preToolUse"].([]interface{})
	post, _ := hooks["postToolUse"].([]interface{})
	// 3 entries each: attention-signal/cleanup (unfiltered) + credential-guard
	// scoped to matcher "Read" + credential-guard scoped to matcher "Write"
	// (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(pre) != 3 || len(post) != 3 {
		t.Fatalf("expected 3 hooks.preToolUse and 3 hooks.postToolUse entries, got %d pre, %d post", len(pre), len(post))
	}
	if pre[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("hooks.preToolUse[0] should be the attention-signal script, got %v", pre[0])
	}
	if post[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-cleanup.sh" {
		t.Errorf("hooks.postToolUse[0] should be the attention-cleanup script, got %v", post[0])
	}

	before, _ := hooks["beforeShellExecution"].([]interface{})
	after, _ := hooks["afterShellExecution"].([]interface{})
	// beforeShellExecution: 2 entries -- credential-guard + git-branch-guard
	// (ROADMAP-2026-08-14 ML-3A, PreToolUse-only, see InjectCursorHooks doc
	// comment). afterShellExecution stays at 1 (credential-guard only).
	if len(before) != 2 || len(after) != 1 {
		t.Fatalf("expected 2 beforeShellExecution and 1 afterShellExecution entry, got %d before, %d after", len(before), len(after))
	}
	if before[0].(map[string]interface{})["command"] != "scripts/trackfw-credential-guard.sh" {
		t.Errorf("beforeShellExecution[0] should be the credential-guard script, got %v", before[0])
	}
	foundGitGuard := false
	for _, item := range before {
		if item.(map[string]interface{})["command"] == "scripts/trackfw-git-branch-guard.sh" {
			foundGitGuard = true
		}
	}
	if !foundGitGuard {
		t.Error("beforeShellExecution missing the git-branch-guard entry")
	}
	if after[0].(map[string]interface{})["command"] != "scripts/trackfw-credential-guard.sh" {
		t.Errorf("afterShellExecution[0] should be the credential-guard script, got %v", after[0])
	}
}

func TestInjectCursorHooks_MigratesLegacyTopLevelArrays(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{
  "preToolUse": [{"command": "scripts/trackfw-attention-signal.sh"}, {"command": "./my-custom-hook.sh"}],
  "postToolUse": [{"command": "scripts/trackfw-attention-cleanup.sh"}]
}`
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(legacy), 0644); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}

	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))

	// The known trackfw entry must be migrated out of preToolUse, but the
	// unrelated user entry must survive untouched at the top level.
	pre, _ := data["preToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Fatalf("expected 1 surviving unrelated entry in top-level preToolUse, got %d: %v", len(pre), pre)
	}
	if pre[0].(map[string]interface{})["command"] != "./my-custom-hook.sh" {
		t.Errorf("expected unrelated user entry to survive, got %v", pre[0])
	}

	// postToolUse had only the known trackfw entry, so the key must be gone entirely.
	if _, ok := data["postToolUse"]; ok {
		t.Errorf("expected top-level postToolUse to be removed once empty, got %v", data["postToolUse"])
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	hPre, _ := hooks["preToolUse"].([]interface{})
	hPost, _ := hooks["postToolUse"].([]interface{})
	// 3 entries each after migration: the migrated attention-signal/cleanup entry
	// plus the two matcher-scoped credential-guard entries (Read/Write) added by
	// this ML — see TestInjectCursorHooks.
	if len(hPre) != 3 || hPre[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("expected hooks.preToolUse to contain the migrated attention-signal entry, got %v", hPre)
	}
	if len(hPost) != 3 || hPost[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-cleanup.sh" {
		t.Errorf("expected hooks.postToolUse to contain the migrated attention-cleanup entry, got %v", hPost)
	}
}

func TestInjectCursorHooks_PreservesUserVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(`{"version": 2, "hooks": {}}`), 0644); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}

	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	if data["version"] != float64(2) {
		t.Errorf("expected pre-existing version=2 to be preserved, got %v", data["version"])
	}
}

// --- Windsurf ---

func TestInjectWindsurfHooks(t *testing.T) {
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".windsurfrules"))
	if err != nil {
		t.Fatalf("readFile .windsurfrules: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, "Windsurf users:") || !strings.Contains(str, "trackfw-attention.json") {
		t.Errorf(".windsurfrules missing attention instructions: %s", str)
	}
}

// --- Git branch guard wiring (ROADMAP-2026-08-14 ML-3A) ---
//
// The tests below cover the 7-runtime deny/hook wiring for
// scripts/trackfw-git-branch-guard.sh, following the same "call the injector
// twice, assert no duplicate" pattern already used above for
// credential-guard.

func TestInjectWindsurfHooks_WritesGitBranchGuardHook(t *testing.T) {
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("first InjectWindsurfHooks failed: %v", err)
	}
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("second InjectWindsurfHooks failed: %v", err)
	}

	path := filepath.Join(dir, ".windsurf", "hooks.json")
	data := helperReadJSON(t, path)
	hooksMap, ok := data["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected top-level \"hooks\" object, got %v", data["hooks"])
	}
	pre, ok := hooksMap["pre_run_command"].([]interface{})
	if !ok || len(pre) != 1 {
		t.Fatalf("expected exactly 1 pre_run_command entry (idempotent across 2 runs), got %v", hooksMap["pre_run_command"])
	}
	entry, _ := pre[0].(map[string]interface{})
	if entry["command"] != "bash scripts/trackfw-git-branch-guard.sh" {
		t.Errorf("expected command to be the git-branch-guard script, got %v", entry["command"])
	}
	if entry["show_output"] != true {
		t.Errorf("expected show_output=true, got %v", entry["show_output"])
	}
}

func TestInjectWindsurfHooks_MigratesLegacyHookFile(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".windsurf", "hooks", "trackfw-git-branch-guard.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"version":1,"hooks":[]}`), 0644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("expected legacy hook file to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".windsurf", "hooks.json")); err != nil {
		t.Errorf("expected .windsurf/hooks.json to be written, got: %v", err)
	}
}

func TestInjectWindsurfHooks_PreservesOtherEvents(t *testing.T) {
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"post_run_command": []interface{}{
				map[string]interface{}{"command": "echo done", "show_output": false},
			},
			"pre_run_command": []interface{}{
				map[string]interface{}{"command": "some-other-tool-hook", "show_output": true},
			},
		},
	})

	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, _ := data["hooks"].(map[string]interface{})
	post, _ := hooksMap["post_run_command"].([]interface{})
	if len(post) != 1 {
		t.Errorf("expected pre-existing post_run_command entry to survive, got %v", post)
	}
	pre, _ := hooksMap["pre_run_command"].([]interface{})
	if len(pre) != 2 {
		t.Fatalf("expected 2 pre_run_command entries (pre-existing + git-guard), got %v", pre)
	}
	foundExisting, foundNew := false, false
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		switch obj["command"] {
		case "some-other-tool-hook":
			foundExisting = true
		case "bash scripts/trackfw-git-branch-guard.sh":
			foundNew = true
		}
	}
	if !foundExisting {
		t.Error("pre-existing pre_run_command entry was lost")
	}
	if !foundNew {
		t.Error("git-branch-guard pre_run_command entry was not added")
	}
}

func TestInjectAmazonQHooks_CreateAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("first InjectAmazonQHooks failed: %v", err)
	}
	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("second InjectAmazonQHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"))

	if data["name"] != "q_cli_default" {
		t.Errorf("expected name=q_cli_default, got %v", data["name"])
	}
	tools, _ := data["tools"].([]interface{})
	if len(tools) != 1 || tools[0] != "*" {
		t.Errorf("expected tools=[\"*\"], got %v", data["tools"])
	}

	if !helperHasClaudeHook(data, "preToolUse", "execute_bash", "scripts/trackfw-git-branch-guard.sh") {
		t.Error("hooks.preToolUse[execute_bash] missing the git-branch-guard command")
	}
	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["preToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Errorf("expected exactly 1 preToolUse matcher entry (idempotent across 2 runs), got %d", len(pre))
	}
	obj, _ := pre[0].(map[string]interface{})
	inner, _ := obj["hooks"].([]interface{})
	if len(inner) != 1 {
		t.Errorf("expected exactly 1 command inside preToolUse[execute_bash] (idempotent across 2 runs), got %d", len(inner))
	}

	toolsSettings, _ := data["toolsSettings"].(map[string]interface{})
	execBash, _ := toolsSettings["execute_bash"].(map[string]interface{})
	denied, _ := execBash["deniedCommands"].([]interface{})
	if len(denied) != 1 || denied[0] != `^git (commit|push|checkout -b)` {
		t.Errorf("expected exactly 1 deniedCommands entry (idempotent across 2 runs), got %v", denied)
	}
}

func TestInjectAmazonQHooks_PreservesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"), map[string]interface{}{
		"someOtherSetting": "keep-me",
		"name":             "q_cli_default",
		"toolsSettings": map[string]interface{}{
			"execute_bash": map[string]interface{}{
				"deniedCommands": []interface{}{"^rm -rf /"},
			},
		},
	})

	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("InjectAmazonQHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"))
	if data["someOtherSetting"] != "keep-me" {
		t.Errorf("expected unrelated setting to survive, got %v", data["someOtherSetting"])
	}
	toolsSettings, _ := data["toolsSettings"].(map[string]interface{})
	execBash, _ := toolsSettings["execute_bash"].(map[string]interface{})
	denied, _ := execBash["deniedCommands"].([]interface{})
	if len(denied) != 2 {
		t.Fatalf("expected 2 deniedCommands entries (pre-existing + git-guard), got %v", denied)
	}
	foundExisting, foundNew := false, false
	for _, d := range denied {
		switch d {
		case "^rm -rf /":
			foundExisting = true
		case `^git (commit|push|checkout -b)`:
			foundNew = true
		}
	}
	if !foundExisting {
		t.Error("pre-existing deniedCommands entry was lost")
	}
	if !foundNew {
		t.Error("git-branch-guard deniedCommands entry was not added")
	}
}

func TestInjectHooksDetected_DispatchesAmazonQWhenDirExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".amazonq"), 0755); err != nil {
		t.Fatalf("mkdir .amazonq: %v", err)
	}
	if err := InjectHooksDetected(dir); err != nil {
		t.Fatalf("InjectHooksDetected failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json")); err != nil {
		t.Errorf("expected .amazonq/cli-agents/q_cli_default.json to be written by InjectHooksDetected, got: %v", err)
	}
}
