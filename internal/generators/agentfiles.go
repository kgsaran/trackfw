package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const rulesStart = "<!-- trackfw:rules:start -->"
const rulesEnd = "<!-- trackfw:rules:end -->"

var agentFiles = map[string]string{
	"claude":   "CLAUDE.md",
	"codex":    "AGENTS.md",
	"gemini":   "GEMINI.md",
	"copilot":  ".github/copilot-instructions.md",
	"windsurf": ".windsurfrules",
	"amazonq":  ".amazonq/developer/guidelines.md",
	"cursor":   ".cursor/rules/trackfw.mdc",
}

var agentHeaders = map[string]string{
	"claude":   "# Project Instructions\n",
	"codex":    "# Project Instructions\n",
	"gemini":   "# Project Instructions\n",
	"copilot":  "# GitHub Copilot Instructions\n",
	"windsurf": "# Windsurf Rules\n",
	"amazonq":  "# Amazon Q Developer Guidelines\n",
	"cursor":   "---\ndescription: trackfw governance rules\nglob: \"**/*\"\nalwaysApply: true\n---\n",
}

func trackfwRulesBlock() string {
	return rulesStart + `
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: ` + "`ADR → REQ → ROADMAP`" + ` · States: ` + "`backlog / analyzing / wip / blocked / done / abandoned`" + `

### Agent Protocol
1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:
   ` + "`trackfw req new \"title\"`" + ` → ` + "`trackfw roadmap new \"title\"`" + ` → ` + "`trackfw roadmap move <name> wip`" + ` → ` + "`git checkout -b feat/<branch>`" + `
   ❌ Never create a branch before REQ + ROADMAP are in wip/
   ❌ Never defer REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables
   ✓ ` + "`trackfw validate`" + ` enforces this via ` + "`branch_has_wip_roadmap`" + ` rule (v2.7.0+)
2. **Before starting:** run ` + "`trackfw context`" + ` · read ` + "`docs/agents-working-context.md`" + `
3. **After finishing:** update ` + "`docs/agents-working-context.md`" + ` with what changed
4. **Before PR:** ` + "`trackfw validate`" + ` must pass
5. **ML lifecycle — mandatory:**
   - Starting a ML: edit roadmap ` + "`**Status:** ⬜ Pendente`" + ` → ` + "`**Status:** 🔄 Em andamento`" + ` + commit.
   - Completing a ML: edit roadmap → ` + "`**Status:** ✅ Concluído`" + ` + include in ML commit.
   - Analyzing a roadmap: move from ` + "`backlog/`" + ` to ` + "`analyzing/`" + `; to ` + "`wip/`" + ` only when coding starts.
6. **` + GlobalADRsDirective + `**

### Attention Signal (when you need user input during a task)
Write ` + "`docs/roadmaps/.trackfw-attention.json`" + `:
` + "```" + `json
{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}
` + "```" + `
Delete the file when resolved. Visible as a live banner in ` + "`trackfw serve`" + `.

> **Windsurf users:** before asking the user a question or requesting approval, write
> ` + "`<roadmap_dir>/.trackfw-attention.json`" + ` manually — there is no automatic hook for this.
> Delete the file after the user responds.

### Architecture Directives (mandatory)
- **3-layer separation:** frontend / backend / database — never mix concerns
- **No in-memory data:** always database + ORM (never arrays/globals for persistence)
- **Auth from day 1:** never defer — refactoring auth later is very costly
- **Docker + .env from day 1:** containerize early; all config via env vars
- **2-layer validation:** frontend (UX) + backend (security) — never only one
- **API-first:** define OpenAPI contract before coding frontend/backend integration
- **Security wave:** include a red-team review wave in every feature roadmap
- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)
- Use ` + "`/trackfw:architect`" + ` to define stack before the first REQ

### Key Commands
- ` + "`trackfw context`" + ` — current governance state (always run first)
- ` + "`trackfw status`" + ` — all artifacts and states
- ` + "`trackfw validate`" + ` — governance consistency check
- ` + "`trackfw roadmap move <name> <state>`" + ` — transition roadmap state
- ` + "`trackfw serve`" + ` — live Kanban board at http://localhost:4080
` + rulesEnd
}

// injectOrUpdateRules injects or updates the trackfw governance rules block in filePath.
//   - File doesn't exist: creates with headerIfNew + rules block
//   - File exists, no marker: appends rules block at end
//   - File exists, has marker: replaces content between markers (idempotent update)
func injectOrUpdateRules(filePath, headerIfNew string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	block := trackfwRulesBlock()

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		content := headerIfNew
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + block + "\n"
		return os.WriteFile(filePath, []byte(content), 0644)
	}
	if err != nil {
		return err
	}

	content := string(data)

	start := strings.Index(content, rulesStart)
	if start == -1 {
		// No marker: append
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + block + "\n"
		return os.WriteFile(filePath, []byte(content), 0644)
	}

	// Has start marker: replace up to and including end marker
	end := strings.Index(content, rulesEnd)
	if end == -1 {
		// Malformed (start without end): append fresh block
		content += "\n" + block + "\n"
		return os.WriteFile(filePath, []byte(content), 0644)
	}

	newContent := content[:start] + block + content[end+len(rulesEnd):]
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// InjectRulesForTool injects trackfw governance rules into the config file for the given
// AI tool. tool must be one of: claude, codex, gemini, copilot, windsurf, amazonq, cursor.
// cwd is the project root directory.
func InjectRulesForTool(tool, cwd string) error {
	relPath, ok := agentFiles[tool]
	if !ok {
		return nil
	}
	header := agentHeaders[tool]
	return injectOrUpdateRules(filepath.Join(cwd, relPath), header)
}

// InjectRulesDetected scans cwd for existing AI agent config files and injects
// trackfw governance rules into each one found.
// For Cursor: also injects when .cursor/ directory exists (even if trackfw.mdc doesn't yet).
// Errors are collected and returned as a single error; processing continues for all files.
func InjectRulesDetected(cwd string) error {
	var errs []string

	for tool, relPath := range agentFiles {
		// Cursor: inject whenever .cursor/ dir exists
		if tool == "cursor" {
			if _, statErr := os.Stat(filepath.Join(cwd, ".cursor")); statErr == nil {
				if err := InjectRulesForTool(tool, cwd); err != nil {
					errs = append(errs, tool+": "+err.Error())
				}
			}
			continue
		}

		// All other tools: only inject if their config file already exists
		if _, statErr := os.Stat(filepath.Join(cwd, relPath)); statErr == nil {
			if err := InjectRulesForTool(tool, cwd); err != nil {
				errs = append(errs, tool+": "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("partial: %s", strings.Join(errs, "; "))
	}
	return nil
}

// --- Attention Hook Injectors ---

// InjectClaudeHooks injects Claude Code attention hooks into .claude/settings.json.
func InjectClaudeHooks(cwd string) error {
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

	hooks["PreToolUse"] = mergeClaudeHookArray(
		hooks["PreToolUse"],
		"AskUserQuestion",
		"scripts/trackfw-attention-signal.sh",
	)
	hooks["PreToolUse"] = mergeClaudeHookArray(
		hooks["PreToolUse"],
		"Bash",
		"scripts/trackfw-credential-guard.sh",
	)

	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		"AskUserQuestion",
		"scripts/trackfw-attention-cleanup.sh",
	)
	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		"Bash",
		"scripts/trackfw-credential-guard.sh",
	)

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// InjectCodexHooks injects Codex CLI attention hooks into .codex/hooks.json.
//
// Two independent hook events are wired here:
//   - PermissionRequest (matcher ".*") — existing attention-signal, only fires when
//     Codex is about to prompt for approval (shell escalation / managed-network
//     approval). Does not fire for commands that don't need approval.
//   - PreToolUse (matcher "Bash") + PostToolUse (matcher "Bash") — credential-guard,
//     fires for every Bash tool call regardless of approval requirement. Confirmed
//     against https://developers.openai.com/codex/hooks (2026-08-05): hooks are
//     enabled by default in Codex CLI (no `[features] hooks = true`/`codex_hooks`
//     opt-in needed — that flag exists only to turn hooks OFF), and PreToolUse
//     blocking uses exit code 2 + stderr (matching trackfw-credential-guard.sh's
//     existing "block" mode).
func InjectCodexHooks(cwd string) error {
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

	hooks["PermissionRequest"] = mergeClaudeHookArray(
		hooks["PermissionRequest"],
		".*",
		"scripts/trackfw-attention-signal.sh",
	)

	hooks["PreToolUse"] = mergeClaudeHookArray(
		hooks["PreToolUse"],
		"Bash",
		"scripts/trackfw-credential-guard.sh",
	)

	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		".*",
		"scripts/trackfw-attention-cleanup.sh",
	)
	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		"Bash",
		"scripts/trackfw-credential-guard.sh",
	)

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// InjectGeminiHooks injects Gemini CLI attention hooks into .gemini/settings.json.
//
// Three independent hook events are wired here:
//   - Notification (matcher "ToolPermission") — existing attention-signal, only fires
//     when Gemini CLI is about to prompt for permission, not for every tool call.
//   - BeforeTool (matcher "run_shell_command") + AfterTool (matcher "run_shell_command") —
//     credential-guard, fires for every shell tool call regardless of whether a
//     permission prompt is needed. Confirmed against
//     https://geminicli.com/docs/hooks/reference (retrieved 2026-08-05): BeforeTool
//     "Fires before a tool is invoked. Used for argument validation, security checks,
//     and parameter rewriting" and supports "Exit Code 2 (Block Tool): Prevents
//     execution. Uses stderr as the reason" — matching trackfw-credential-guard.sh's
//     existing "block" mode. The shell tool's canonical name is "run_shell_command"
//     (doc: "you can match any built-in tool (for example, read_file,
//     run_shell_command)"); matcher is a regex evaluated against tool_name.
//   - AfterTool (matcher "*") — pre-existing attention-cleanup, unrelated to the new
//     credential-guard wiring above (different matcher, added as a separate array
//     entry so the two coexist without merging into one hooks group).
//
// Concurrency note: the doc's `sequential` field only orders hooks *within* one
// matcher group ("If true, hooks in this group run one after another"); it says
// nothing about ordering across two different matching groups for the same event
// (e.g. AfterTool["*"] vs AfterTool["run_shell_command"] both firing for a shell
// call). That cross-group model is undocumented, so no ordering is assumed here.
// It does not matter for this wiring because credential-guard's "warn" mode writes
// to its own dedicated $ROADMAP_DIR/.trackfw-credential-guard.json (see ML-1A),
// never touching the .trackfw-attention.json file that trackfw-attention-cleanup.sh
// deletes — the same fix that neutralized the equivalent race confirmed for Codex
// in ML-2B applies here regardless of Gemini's actual concurrency model.
func InjectGeminiHooks(cwd string) error {
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

	hooks["BeforeTool"] = mergeClaudeHookArray(
		hooks["BeforeTool"],
		"run_shell_command",
		"scripts/trackfw-credential-guard.sh",
	)

	hooks["AfterTool"] = mergeClaudeHookArray(
		hooks["AfterTool"],
		"*",
		"scripts/trackfw-attention-cleanup.sh",
	)
	hooks["AfterTool"] = mergeClaudeHookArray(
		hooks["AfterTool"],
		"run_shell_command",
		"scripts/trackfw-credential-guard.sh",
	)

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// InjectKiroHooks injects Kiro attention hooks into .kiro/hooks/trackfw-attention.json.
// Overwriting this file is intentional as trackfw-attention.json is a dedicated file owned exclusively by trackfw.
func InjectKiroHooks(cwd string) error {
	dir := filepath.Join(cwd, ".kiro", "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "trackfw-attention.json")

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

// InjectCopilotHooks injects GitHub Copilot attention hooks into .github/hooks/trackfw-attention.json.
// Overwriting this file is intentional as trackfw-attention.json is a dedicated file owned exclusively by trackfw.
//
// Format confirmed against https://docs.github.com/en/copilot/reference/hooks-reference (retrieved
// 2026-08-05): repository-level hook files live at .github/hooks/*.json (a directory of files that are
// all loaded and combined), each using the schema {"version": 1, "hooks": {"<event>": [<command entry>,
// ...]}}, where a command entry is {"type": "command", "bash": "...", "cwd": "...", "timeoutSec": N}.
// This is the format `inject_copilot_hooks` (Python) already used; the {"hooks": [{"event", "run"}]}
// shape this Go function and its Node sibling previously emitted does not match any format documented
// by GitHub -- Go/Node were wrong, Python was right, and this ML aligns Go/Node to it.
//
// Matcher: the doc's matcher-filtering table lists `preToolUse -> toolName` and `postToolUse ->
// toolName` (a regex, anchored `^(?:PATTERN)$`), and shows a worked `"matcher"` field inline on a
// postToolUse command entry. The Command-hooks field table itself does not list `matcher` explicitly,
// but per the doc's own malformed-item handling ("only that item is dropped and logged"), a rejected
// field would silently drop the whole entry rather than error loudly -- so this is used defensively:
// even if `matcher` were ignored by some Copilot version, trackfw-credential-guard.sh already filters
// on its own raw-payload scan (ML-1A) and is a safe no-op when the match doesn't hit, so restricting
// scope here is a hardening layer, not the sole line of defense.
//
// Tool name for matching: with camelCase event names (preToolUse/postToolUse, used here and by the
// pre-existing signal/cleanup entries), the doc specifies the *runtime* tool name is reported in
// `toolName`, and the shell tool's runtime name is "bash" (lowercase) -- distinct from the PascalCase
// event/VS Code-compatible payload shape, which would report the Claude-mapped name "Bash". The script
// itself scans the raw JSON payload for JWT/AWS-key patterns regardless of field names, so it works
// under either payload shape; the matcher below is only a scope-narrowing optimization, not something
// the script's own detection logic depends on.
//
// Concurrency: "If multiple hooks of the same type are configured, they execute in order" (same
// section) -- Copilot hooks run serially, in configured order, for the same event. This makes the
// postToolUse cleanup/guard ordering deterministic here (unlike Codex's confirmed-concurrent or
// Gemini's undocumented cross-group model); the ML-1A fix (credential-guard's "warn" mode writes to
// its own dedicated $ROADMAP_DIR/.trackfw-credential-guard.json, never touching the shared
// .trackfw-attention.json that trackfw-attention-cleanup.sh deletes) makes this moot regardless.
func InjectCopilotHooks(cwd string) error {
	dir := filepath.Join(cwd, ".github", "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "trackfw-attention.json")

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
				map[string]interface{}{
					"type":       "command",
					"matcher":    "bash",
					"bash":       "scripts/trackfw-credential-guard.sh",
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
				map[string]interface{}{
					"type":       "command",
					"matcher":    "bash",
					"bash":       "scripts/trackfw-credential-guard.sh",
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

// InjectCursorHooks injects Cursor attention hooks into .cursor/hooks.json.
func InjectCursorHooks(cwd string) error {
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

// InjectWindsurfHooks updates .windsurfrules with the attention instruction.
func InjectWindsurfHooks(cwd string) error {
	return InjectRulesForTool("windsurf", cwd)
}

// --- helpers ---

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
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				return arr
			}
		}
		// Matcher already present but this command isn't yet: merge the new
		// command into the existing entry instead of appending a duplicate
		// matcher entry (keeps parity with npm/pypi's merge behavior and
		// avoids two separate {"matcher":"Bash",...} blocks in the output).
		obj["hooks"] = append(innerHooks, map[string]interface{}{
			"type":    "command",
			"command": command,
		})
		return arr
	}

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
