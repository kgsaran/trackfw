package generators

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttentionSixEmittersNativeAndIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	cases := []struct {
		name, path string
		inject     func(string) error
	}{
		{"claude", ".claude/settings.json", InjectClaudeHooks}, {"codex", ".codex/hooks.json", InjectCodexHooks}, {"gemini", ".gemini/settings.json", InjectGeminiHooks}, {"kiro", ".kiro/hooks/trackfw-attention.json", InjectKiroHooks}, {"copilot", ".github/hooks/trackfw-attention.json", InjectCopilotHooks}, {"cursor", ".cursor/hooks.json", InjectCursorHooks},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tc.inject(dir); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, filepath.FromSlash(tc.path))
			first, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.inject(dir); err != nil {
				t.Fatal(err)
			}
			second, _ := os.ReadFile(path)
			if !bytes.Equal(first, second) {
				t.Fatal("second update changed hooks")
			}
			var config interface{}
			if err := json.Unmarshal(second, &config); err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			var walk func(interface{})
			walk = func(v interface{}) {
				switch x := v.(type) {
				case map[string]interface{}:
					for key, item := range x {
						if cmd, ok := item.(string); ok && (key == "command" || key == "bash") {
							if strings.Contains(cmd, "trackfw-attention-") {
								t.Fatalf("legacy attention emitted %q", cmd)
							}
							if strings.HasPrefix(cmd, "trackfw attention ") {
								if key != "command" {
									t.Fatal("native attention must use command field")
								}
								counts[cmd]++
							}
						}
						walk(item)
					}
				case []interface{}:
					for _, item := range x {
						walk(item)
					}
				}
			}
			walk(config)
			if counts[attentionSignalCmd] != 1 || counts[attentionCleanupCmd] != 1 {
				t.Fatalf("attention counts: %#v", counts)
			}
		})
	}
}

func TestAttentionMigrationPreservesCustomMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	cases := []struct {
		path, event, matcher string
		inject               func(string) error
	}{
		{".claude/settings.json", "PreToolUse", "AskUserQuestion", InjectClaudeHooks}, {".codex/hooks.json", "PermissionRequest", ".*", InjectCodexHooks}, {".gemini/settings.json", "Notification", "ToolPermission", InjectGeminiHooks},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(tc.path))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			custom := "echo scripts/trackfw-attention-signal.sh custom"
			entries := []interface{}{}
			for _, cmd := range append(legacyAttentionCommands(attentionSignalCmd), attentionSignalCmd, custom) {
				entries = append(entries, map[string]interface{}{"type": "command", "command": cmd, "timeout": 17})
			}
			seed := map[string]interface{}{"customKey": true, "hooks": map[string]interface{}{tc.event: []interface{}{map[string]interface{}{"matcher": tc.matcher, "hooks": entries}, map[string]interface{}{"matcher": "CUSTOM_MATCHER", "hooks": []interface{}{map[string]interface{}{"type": "command", "command": custom, "timeout": 29}}}}}}
			data, _ := json.Marshal(seed)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := tc.inject(dir); err != nil {
					t.Fatal(err)
				}
			}
			data, _ = os.ReadFile(path)
			var got map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got["customKey"] != true {
				t.Fatal("custom key lost")
			}
			hooks := got["hooks"].(map[string]interface{})
			groups := hooks[tc.event].([]interface{})
			count, customCount := 0, 0
			for _, v := range groups {
				group := v.(map[string]interface{})
				for _, h := range group["hooks"].([]interface{}) {
					hook := h.(map[string]interface{})
					if hook["command"] == attentionSignalCmd {
						count++
						if hook["timeout"] != float64(17) {
							t.Fatal("timeout lost")
						}
					}
					if hook["command"] == custom {
						customCount++
					}
				}
			}
			if count != 1 || customCount != 2 {
				t.Fatalf("counts native=%d custom=%d", count, customCount)
			}
		})
	}
}

func TestAttentionCursorMigrationNestedAndTopLevel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	custom := "echo scripts/trackfw-attention-signal.sh"
	entries := []interface{}{}
	for _, cmd := range append(legacyAttentionCommands(attentionSignalCmd), attentionSignalCmd) {
		entries = append(entries, map[string]interface{}{"command": cmd, "timeout": 17})
	}
	entries = append(entries, map[string]interface{}{"command": custom, "timeout": 42})
	config := map[string]interface{}{"preToolUse": entries, "hooks": map[string]interface{}{"preToolUse": []interface{}{map[string]interface{}{"command": attentionSignalCmd}}}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := InjectCursorHooks(dir); err != nil {
			t.Fatal(err)
		}
	}
	data, _ = os.ReadFile(path)
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	native := 0
	for _, e := range config["hooks"].(map[string]interface{})["preToolUse"].([]interface{}) {
		obj := e.(map[string]interface{})
		if obj["command"] == attentionSignalCmd {
			native++
			if obj["timeout"] != float64(17) {
				t.Fatal("timeout lost")
			}
		}
	}
	if native != 1 {
		t.Fatalf("%d native entries", native)
	}
	top := config["preToolUse"].([]interface{})
	if len(top) != 1 || top[0].(map[string]interface{})["command"] != custom {
		t.Fatal("custom top-level lost")
	}
}

func TestAttentionShellCommandsAndFailSoftWrappers(t *testing.T) {
	binary := compiledGuardBinary(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := GenerateAttentionScripts(dir); err != nil {
		t.Fatal(err)
	}
	for _, shell := range []string{"sh", "bash", "zsh"} {
		shellPath, err := exec.LookPath(shell)
		if err != nil {
			t.Logf("%s unavailable", shell)
			continue
		}
		t.Run(shell, func(t *testing.T) {
			for _, action := range []string{"signal", "cleanup"} {
				cmd := exec.Command(shellPath, "-c", "trackfw attention "+action)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(binary))
				cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"question":"á 😀 $HOME $(touch NEVER)"}}`)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%s: %v %s", action, err, out)
				}
				file := filepath.Join(dir, "docs", "roadmaps", ".trackfw-attention.json")
				if action == "signal" {
					data, err := os.ReadFile(file)
					if err != nil || !bytes.Contains(data, []byte("á 😀")) {
						t.Fatalf("signal failed %v %s", err, data)
					}
				} else if _, err := os.Stat(file); !os.IsNotExist(err) {
					t.Fatal("cleanup failed")
				}
			}
		})
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"missing", "failing"} {
		binDir := t.TempDir()
		if state == "failing" {
			if err := os.WriteFile(filepath.Join(binDir, "trackfw"), []byte("#!/bin/sh\nexit 27\n"), 0755); err != nil {
				t.Fatal(err)
			}
		}
		for _, action := range []string{"signal", "cleanup"} {
			cmd := exec.Command(bash, filepath.Join(dir, "scripts", "trackfw-attention-"+action+".sh"))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+binDir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s wrapper %s blocked: %v %s", state, action, err, out)
			}
		}
	}
}
