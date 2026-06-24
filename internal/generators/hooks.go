package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InjectHooksDetected detecta quais CLIs estão presentes no cwd e injeta os attention hooks
// em cada um. Erros são coletados e retornados como string joined (não para na primeira falha).
func InjectHooksDetected(cwd string) error {
	type detector struct {
		fn     func(string) error
		detect func(string) bool
	}

	detections := map[string]detector{
		"claude": {
			fn: injectClaudeHooks,
			detect: func(cwd string) bool {
				_, err1 := os.Stat(filepath.Join(cwd, ".claude"))
				_, err2 := os.Stat(filepath.Join(cwd, "CLAUDE.md"))
				return err1 == nil || err2 == nil
			},
		},
		"codex": {
			fn: injectCodexHooks,
			detect: func(cwd string) bool {
				_, err1 := os.Stat(filepath.Join(cwd, "AGENTS.md"))
				_, err2 := os.Stat(filepath.Join(cwd, ".codex"))
				return err1 == nil || err2 == nil
			},
		},
		"gemini": {
			fn: injectGeminiHooks,
			detect: func(cwd string) bool {
				_, err1 := os.Stat(filepath.Join(cwd, "GEMINI.md"))
				_, err2 := os.Stat(filepath.Join(cwd, ".gemini"))
				return err1 == nil || err2 == nil
			},
		},
		"kiro": {
			fn: injectKiroHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".kiro"))
				return err == nil
			},
		},
		"copilot": {
			fn: injectCopilotHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".github", "copilot-instructions.md"))
				return err == nil
			},
		},
		"cursor": {
			fn: injectCursorHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".cursor"))
				return err == nil
			},
		},
	}

	var errs []string
	for name, d := range detections {
		if d.detect(cwd) {
			if err := d.fn(cwd); err != nil {
				errs = append(errs, name+": "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("partial: %s", strings.Join(errs, "; "))
	}
	return nil
}

// --- Claude ---

func injectClaudeHooks(cwd string) error {
	path := filepath.Join(cwd, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	// PreToolUse
	hooks["PreToolUse"] = mergeClaudeHookArray(
		hooks["PreToolUse"],
		"AskUserQuestion",
		"scripts/trackfw-attention-signal.sh",
	)

	// PostToolUse
	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		"AskUserQuestion",
		"scripts/trackfw-attention-cleanup.sh",
	)

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// mergeClaudeHookArray garante que o array de hooks Claude contém a entry com matcher + command.
// Não duplica se já existir.
func mergeClaudeHookArray(existing interface{}, matcher, command string) []interface{} {
	arr, _ := existing.([]interface{})

	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if obj["matcher"] != matcher {
			continue
		}
		// Verificar se o command já existe dentro de hooks[]
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				return arr // já existe, sem mudança
			}
		}
	}

	// Adicionar nova entry
	entry := map[string]interface{}{
		"matcher": matcher,
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": command,
			},
		},
	}
	return append(arr, entry)
}

// --- Codex ---

func injectCodexHooks(cwd string) error {
	dir := filepath.Join(cwd, ".codex")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "hooks.json")

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	hooks["PermissionRequest"] = mergeSimpleCommandArray(
		hooks["PermissionRequest"],
		"scripts/trackfw-attention-signal.sh",
		func(command string) interface{} {
			return map[string]interface{}{"matcher": ".*", "hooks": []interface{}{map[string]interface{}{"type": "command", "command": command}}}
		},
		func(item interface{}) string {
			// Para Codex, verificar command dentro de hooks[0].command
			obj, ok := item.(map[string]interface{})
			if !ok {
				return ""
			}
			innerHooks, _ := obj["hooks"].([]interface{})
			if len(innerHooks) == 0 {
				return ""
			}
			h, ok := innerHooks[0].(map[string]interface{})
			if !ok {
				return ""
			}
			cmd, _ := h["command"].(string)
			return cmd
		},
	)

	hooks["PostToolUse"] = mergeSimpleCommandArray(
		hooks["PostToolUse"],
		"scripts/trackfw-attention-cleanup.sh",
		func(command string) interface{} {
			return map[string]interface{}{"matcher": ".*", "hooks": []interface{}{map[string]interface{}{"type": "command", "command": command}}}
		},
		func(item interface{}) string {
			obj, ok := item.(map[string]interface{})
			if !ok {
				return ""
			}
			innerHooks, _ := obj["hooks"].([]interface{})
			if len(innerHooks) == 0 {
				return ""
			}
			h, ok := innerHooks[0].(map[string]interface{})
			if !ok {
				return ""
			}
			cmd, _ := h["command"].(string)
			return cmd
		},
	)

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// --- Gemini ---

func injectGeminiHooks(cwd string) error {
	dir := filepath.Join(cwd, ".gemini")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "settings.json")

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	hooks["Notification"] = mergeClaudeHookArray(
		hooks["Notification"],
		"ToolPermission",
		"scripts/trackfw-attention-signal.sh",
	)
	hooks["AfterTool"] = mergeClaudeHookArray(
		hooks["AfterTool"],
		"*",
		"scripts/trackfw-attention-cleanup.sh",
	)

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// --- Kiro ---

func injectKiroHooks(cwd string) error {
	dir := filepath.Join(cwd, ".kiro", "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "trackfw-attention.json")

	// Arquivo dedicado trackfw-owned — overwrite seguro
	content := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"name":        "trackfw-attention-signal",
				"description": "Signals trackfw board when agent executes a tool",
				"event":       "PreToolUse",
				"matcher":     map[string]interface{}{"tool_name": ".*"},
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-attention-signal.sh"},
			},
			map[string]interface{}{
				"name":        "trackfw-attention-cleanup",
				"description": "Clears trackfw board attention after tool completes",
				"event":       "PostToolUse",
				"matcher":     map[string]interface{}{"tool_name": ".*"},
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-attention-cleanup.sh"},
			},
		},
	}

	out, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// --- Copilot ---

func injectCopilotHooks(cwd string) error {
	dir := filepath.Join(cwd, ".github", "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "trackfw-attention.json")

	// Arquivo dedicado trackfw-owned — overwrite seguro
	content := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type":       "command",
					"bash":       "scripts/trackfw-attention-signal.sh",
					"cwd":        ".",
					"timeoutSec": 10,
				},
			},
			"postToolUse": []interface{}{
				map[string]interface{}{
					"type":       "command",
					"bash":       "scripts/trackfw-attention-cleanup.sh",
					"cwd":        ".",
					"timeoutSec": 10,
				},
			},
		},
	}

	out, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// --- Cursor ---

func injectCursorHooks(cwd string) error {
	path := filepath.Join(cwd, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	makeEntry := func(command string) interface{} {
		return map[string]interface{}{"command": command}
	}
	getCmd := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		cmd, _ := obj["command"].(string)
		return cmd
	}

	root["preToolUse"] = mergeSimpleCommandArray(root["preToolUse"], "scripts/trackfw-attention-signal.sh", makeEntry, getCmd)
	root["postToolUse"] = mergeSimpleCommandArray(root["postToolUse"], "scripts/trackfw-attention-cleanup.sh", makeEntry, getCmd)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// --- helpers ---

// mergeSimpleCommandArray garante deduplicação por command em arrays simples.
// makeEntry constrói a entrada JSON a partir do command.
// getCmd extrai o command de uma entrada existente para dedup.
func mergeSimpleCommandArray(
	existing interface{},
	command string,
	makeEntry func(string) interface{},
	getCmd func(interface{}) string,
) []interface{} {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		if getCmd(item) == command {
			return arr
		}
	}
	return append(arr, makeEntry(command))
}
