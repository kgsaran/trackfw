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

	// Migration (ROADMAP-2026-08-11 ML-2A): rewrite any stale relative-path attention-signal
	// command from an older trackfw run before merging the $CLAUDE_PROJECT_DIR-pinned one below,
	// so upgrading doesn't just append a second, still-cwd-fragile entry alongside the fixed one
	// -- same "No such file or directory" bug class, and same migrate-before-merge ordering
	// requirement, as the credential-guard fix a few lines below.
	migrateHookCommand(hooks["PreToolUse"], "AskUserQuestion", "scripts/trackfw-attention-signal.sh", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh")

	hooks["PreToolUse"] = mergeClaudeHookArray(
		hooks["PreToolUse"],
		"AskUserQuestion",
		"$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh",
	)

	// Fix (2026-08-09, reported in production against the CMDB project):
	// the credential-guard command was a bare relative path
	// ("scripts/trackfw-credential-guard.sh"), which Claude Code resolves
	// against the hook's *current* cwd, not the project root — cwd tracks
	// `cd`s the agent runs during the session (confirmed against
	// https://code.claude.com/docs/en/hooks: "Handlers run in the current
	// directory... cwd is dynamic"), so any Bash/Read/Write/Edit call after
	// the agent `cd`s into a subdirectory (e.g. a monorepo package) made the
	// hook fail with "No such file or directory". $CLAUDE_PROJECT_DIR is the
	// env var Claude Code guarantees stays pinned to the project root
	// regardless of cwd drift (same doc) — used here instead, matching the
	// pattern this project's own custom hooks (posttooluse-frontend-gate.sh,
	// pretooluse-rewriter.sh) already relied on successfully. Rewrite any
	// stale relative-path entry from an older trackfw run before merging the
	// fixed command, so upgrading doesn't just append a second, still-broken
	// entry alongside the new one.
	for _, matcher := range []string{"Bash", "Read", "Write|Edit"} {
		migrateHookCommand(hooks["PreToolUse"], matcher, "scripts/trackfw-credential-guard.sh", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh")
		migrateHookCommand(hooks["PostToolUse"], matcher, "scripts/trackfw-credential-guard.sh", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh")
	}

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to Read/Write|Edit): skip the project-scope
	// credential-guard entry when the global one is already installed
	// (`trackfw update harness --targets claude-credential-guard`), so the
	// guard doesn't run twice per Bash call. attention-signal/cleanup above
	// and below are unaffected — they are inherently project-scope.
	if !globalCredentialGuardInstalledClaude() {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Bash",
			"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh",
		)
		// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, 2026-08-08):
		// extraction via a direct file read, or materialization via write/edit,
		// never went through the hook before.
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Read",
			"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh",
		)
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Write|Edit",
			"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh",
		)
	}

	migrateHookCommand(hooks["PostToolUse"], "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh")

	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		"AskUserQuestion",
		"$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh",
	)
	if !globalCredentialGuardInstalledClaude() {
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"Bash",
			"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh",
		)
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"Read",
			"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh",
		)
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"Write|Edit",
			"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh",
		)
	}

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// ROADMAP-2026-08-11 ML-3A: Codex CLI does not expose a project-root env var for
// repo-local hooks (unlike Claude's $CLAUDE_PROJECT_DIR or Gemini's
// $GEMINI_PROJECT_DIR) — the only documented mechanism is shell substitution.
// Per ADR-2026-08-11 ("Codex — alterar, com dependência explícita de shell e
// git"), the command is wrapped in literal double quotes around
// `$(git rev-parse --show-toplevel)`, matching every repo-local hook example in
// the official Codex docs (https://developers.openai.com/codex/config-advanced):
// "For repo-local hooks, prefer resolving from the git root instead of using a
// relative path such as `.codex/hooks/...`."
const codexRoot = `"$(git rev-parse --show-toplevel)`

var (
	codexSignalCmd  = codexRoot + `/scripts/trackfw-attention-signal.sh"`
	codexCleanupCmd = codexRoot + `/scripts/trackfw-attention-cleanup.sh"`
	codexGuardCmd   = codexRoot + `/scripts/trackfw-credential-guard.sh"`
)

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
//
// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2,
// 2026-08-08): Codex has NO dedicated, interceptable read-tool matcher —
// confirmed against https://learn.chatgpt.com/docs/hooks — so no read matcher
// is added here; this is a documented limitation (also called out in
// docs/cli-parity.md), not a workaround. Write/edit materialization IS
// covered via the "apply_patch" matcher (documented aliases Edit/Write).
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

	// Migration wiring (ROADMAP-2026-08-11 ML-1A, strings updated in ML-3A):
	// rewrites any stale relative-path entry from before this fix in place, so
	// `trackfw update` doesn't just append the new $(git rev-parse ...) entry
	// alongside the still-cwd-fragile old one.
	migrateHookCommand(hooks["PermissionRequest"], ".*", "scripts/trackfw-attention-signal.sh", codexSignalCmd)
	migrateHookCommand(hooks["PreToolUse"], "Bash", "scripts/trackfw-credential-guard.sh", codexGuardCmd)
	migrateHookCommand(hooks["PreToolUse"], "apply_patch", "scripts/trackfw-credential-guard.sh", codexGuardCmd)
	migrateHookCommand(hooks["PostToolUse"], ".*", "scripts/trackfw-attention-cleanup.sh", codexCleanupCmd)
	migrateHookCommand(hooks["PostToolUse"], "Bash", "scripts/trackfw-credential-guard.sh", codexGuardCmd)
	migrateHookCommand(hooks["PostToolUse"], "apply_patch", "scripts/trackfw-credential-guard.sh", codexGuardCmd)

	hooks["PermissionRequest"] = mergeClaudeHookArray(
		hooks["PermissionRequest"],
		".*",
		codexSignalCmd,
	)

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to apply_patch): skip the project-scope
	// credential-guard entry when the global one is already installed
	// (`trackfw update harness --targets codex-credential-guard`).
	skipCodexCG := globalCredentialGuardInstalledCodex()
	if !skipCodexCG {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Bash",
			codexGuardCmd,
		)
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"apply_patch",
			codexGuardCmd,
		)
	}

	hooks["PostToolUse"] = mergeClaudeHookArray(
		hooks["PostToolUse"],
		".*",
		codexCleanupCmd,
	)
	if !skipCodexCG {
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"Bash",
			codexGuardCmd,
		)
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"apply_patch",
			codexGuardCmd,
		)
	}

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
// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2,
// 2026-08-08): the Gemini CLI tools table (https://geminicli.com/docs/reference/tools)
// documents read_file/read_many_files as the file-read tools and write_file/replace
// as the file-write/edit tools — matcher below follows the same regex-over-tool_name
// convention already used for run_shell_command.
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

	// Migration wiring (ROADMAP-2026-08-11 ML-1A): old==new is a functional no-op
	// today, but proves the call point exists and runs before the merge below.
	// The wave that changes the Gemini command strings (ML-4A) updates oldCommand
	// here instead of adding this call from scratch — without it, the merge's
	// exact-string dedup would append a duplicate alongside the stale entry.
	migrateHookCommand(hooks["Notification"], "ToolPermission", "scripts/trackfw-attention-signal.sh", "scripts/trackfw-attention-signal.sh")
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", "scripts/trackfw-credential-guard.sh", "scripts/trackfw-credential-guard.sh")
	migrateHookCommand(hooks["BeforeTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", "scripts/trackfw-credential-guard.sh")
	migrateHookCommand(hooks["BeforeTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", "scripts/trackfw-credential-guard.sh")
	migrateHookCommand(hooks["AfterTool"], "*", "scripts/trackfw-attention-cleanup.sh", "scripts/trackfw-attention-cleanup.sh")
	migrateHookCommand(hooks["AfterTool"], "run_shell_command", "scripts/trackfw-credential-guard.sh", "scripts/trackfw-credential-guard.sh")
	migrateHookCommand(hooks["AfterTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", "scripts/trackfw-credential-guard.sh")
	migrateHookCommand(hooks["AfterTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", "scripts/trackfw-credential-guard.sh")

	hooks["Notification"] = mergeClaudeHookArray(
		hooks["Notification"],
		"ToolPermission",
		"scripts/trackfw-attention-signal.sh",
	)

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to read_file|read_many_files /
	// write_file|replace): skip the project-scope credential-guard entry when
	// the global one is already installed
	// (`trackfw update harness --targets gemini-credential-guard`).
	skipGeminiCG := globalCredentialGuardInstalledGemini()
	if !skipGeminiCG {
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"run_shell_command",
			"scripts/trackfw-credential-guard.sh",
		)
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"read_file|read_many_files",
			"scripts/trackfw-credential-guard.sh",
		)
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"write_file|replace",
			"scripts/trackfw-credential-guard.sh",
		)
	}

	hooks["AfterTool"] = mergeClaudeHookArray(
		hooks["AfterTool"],
		"*",
		"scripts/trackfw-attention-cleanup.sh",
	)
	if !skipGeminiCG {
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"run_shell_command",
			"scripts/trackfw-credential-guard.sh",
		)
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"read_file|read_many_files",
			"scripts/trackfw-credential-guard.sh",
		)
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"write_file|replace",
			"scripts/trackfw-credential-guard.sh",
		)
	}

	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// InjectKiroHooks injects Kiro attention + credential-guard hooks into .kiro/hooks/trackfw-attention.json.
// Overwriting this file is intentional as trackfw-attention.json is a dedicated file owned exclusively by trackfw.
//
// Format confirmed against https://kiro.dev/docs/hooks/ , https://kiro.dev/docs/hooks/types and
// https://kiro.dev/docs/hooks/actions/ (retrieved 2026-08-05, via curl -L against the RSC/HTML page
// since WebFetch/WebSearch were unavailable in this session):
//
//   - Top-level schema is {"version": "v1", "hooks": [...]} — "version" is the string "v1", not an
//     integer. Each entry is {"name", "description"?, "trigger", "matcher"?, "action", "timeout"?,
//     "enabled"?}. The field is "trigger" (PascalCase event name), NOT "event" as this function and its
//     Node/Python siblings previously emitted — "event" does not appear anywhere in the documented
//     schema. This ML also realigns the pre-existing trackfw-attention-signal/cleanup entries to the
//     correct field name (this file is fully generated/overwritten by trackfw, not merged with
//     user content, so there is no legacy entry to preserve byte-for-byte — same situation as the
//     GitHub Copilot fix in ML-2D).
//   - "matcher" is a plain regex string evaluated against tool name (per the field reference table:
//     "Regex pattern to filter which events fire this hook. For PreToolUse/PostToolUse, matches tool
//     name."), NOT an object like {"tool_name": ".*"} as previously emitted. "*" (a literal asterisk,
//     documented explicitly as "all tools (built-in and MCP)") is used here instead of the invalid
//     ".*" this function used to emit — ".*" is not a documented matcher value (the vocabulary is:
//     canonical tool names like "execute_bash"/"fs_read"/"fs_write"/"use_aws", their aliases
//     "shell"/"read"/"write"/"aws", category wildcards "read"/"write"/"shell"/"web"/"spec", "@"-prefix
//     regex filters, or the literal "*"/no matcher for "all tools").
//   - PreToolUse ("Triggers when the agent is about to invoke a tool. Can validate and block tool
//     usage.") is a real, distinct trigger from PostFileSave/file-save events — confirmed by the
//     "Available triggers" table (PreToolUse: "Before a tool is about to execute", Can block: Yes) and
//     by the dedicated "Pre Tool Use" section of hooks/types. This resolves the open question from the
//     ADR: Kiro's hook system does intercept tool invocations (including shell) before execution, not
//     only IDE/file events.
//   - Blocking contract (hooks/actions, "CLI" tab): "If the command returns an exit code of 0
//     indicating success, the stdout output ... is added to the agent's context. If the command
//     returns any other exit code, the stderr output ... is sent to the agent ... Additionally, in the
//     case of the Pre Tool Use hook, the tool invocation is blocked." This is a stricter contract than
//     Claude Code/Codex/Gemini (which key specifically on exit code 2) — Kiro blocks on ANY non-zero
//     exit from a PreToolUse command hook. trackfw-credential-guard.sh was audited against this: every
//     exit path is an explicit `exit 0` or `exit 2` (block mode); the only unguarded failure surface is
//     an unexpected environment failure under `set -euo pipefail` (e.g. `mkdir -p` failing), which is a
//     generic script-authoring risk shared by every trigger, not a normal-operation fail-closed hazard
//     specific to Kiro's exit-code semantics.
//   - Shell tool name for the matcher: hooks/types documents the canonical name "execute_bash" with
//     alias "shell" ("all built-in shell command-related tools" — broader than the single-tool
//     canonical name, and the choice made here for trackfw-credential-guard.sh's own matcher, since the
//     guard must see every shell invocation, not just one canonical tool identifier).
//   - PreToolUse/PostToolUse STDIN payload is JSON: {"hook_event_name", "cwd", "session_id",
//     "tool_name", "tool_input"} — trackfw-credential-guard.sh scans the raw payload for JWT/AWS-key
//     patterns regardless of field names (ML-1A), so it works under this shape without changes.
func InjectKiroHooks(cwd string) error {
	dir := filepath.Join(cwd, ".kiro", "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "trackfw-attention.json")

	hooks := []interface{}{
		map[string]interface{}{
			"name":        "trackfw-attention-signal",
			"description": "Signals trackfw board when agent executes a tool",
			"trigger":     "PreToolUse",
			"matcher":     "*",
			"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-attention-signal.sh"},
		},
		map[string]interface{}{
			"name":        "trackfw-attention-cleanup",
			"description": "Clears trackfw board attention after tool completes",
			"trigger":     "PostToolUse",
			"matcher":     "*",
			"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-attention-cleanup.sh"},
		},
	}

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to read/write): skip the project-scope
	// credential-guard entries when the global one is already installed
	// (`trackfw update harness --targets kiro-credential-guard`,
	// ~/.kiro/hooks/trackfw-credential-guard.json).
	if !globalCredentialGuardInstalledKiro() {
		hooks = append(hooks,
			map[string]interface{}{
				"name":        "trackfw-credential-guard-pre",
				"description": "Blocks/warns on possible plaintext credential materialization before a shell command executes",
				"trigger":     "PreToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"},
			},
			map[string]interface{}{
				"name":        "trackfw-credential-guard-post",
				"description": "Warns on possible plaintext credential materialization after a shell command executes",
				"trigger":     "PostToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"},
			},
			// Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): "read"
			// and "write" are the documented Kiro tool-category aliases
			// (fs_read/fs_write), same pattern as "shell" above.
			map[string]interface{}{
				"name":        "trackfw-credential-guard-read-pre",
				"description": "Blocks/warns on possible plaintext credential materialization before a file read",
				"trigger":     "PreToolUse",
				"matcher":     "read",
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"},
			},
			map[string]interface{}{
				"name":        "trackfw-credential-guard-read-post",
				"description": "Warns on possible plaintext credential materialization after a file read",
				"trigger":     "PostToolUse",
				"matcher":     "read",
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"},
			},
			map[string]interface{}{
				"name":        "trackfw-credential-guard-write-pre",
				"description": "Blocks/warns on possible plaintext credential materialization before a file write",
				"trigger":     "PreToolUse",
				"matcher":     "write",
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"},
			},
			map[string]interface{}{
				"name":        "trackfw-credential-guard-write-post",
				"description": "Warns on possible plaintext credential materialization after a file write",
				"trigger":     "PostToolUse",
				"matcher":     "write",
				"action":      map[string]interface{}{"type": "command", "command": "scripts/trackfw-credential-guard.sh"},
			},
		)
	}

	content := map[string]interface{}{
		"version": "v1",
		"hooks":   hooks,
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

	preToolUse := []interface{}{
		map[string]interface{}{
			"type":       "command",
			"bash":       "scripts/trackfw-attention-signal.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		},
	}
	postToolUse := []interface{}{
		map[string]interface{}{
			"type":       "command",
			"bash":       "scripts/trackfw-attention-cleanup.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		},
	}

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to view / create|edit): skip the
	// project-scope credential-guard entries when the global one is already
	// installed (`trackfw update harness --targets copilot-credential-guard`).
	//
	// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, 2026-08-08):
	// https://docs.github.com/en/copilot/reference/hooks-reference confirms
	// the camelCase preToolUse/postToolUse toolName mapping `view -> Read`,
	// `create -> Write`, `edit -> Edit` — "view" is the read matcher,
	// "create|edit" the write/edit matcher, same lowercase-runtime-name
	// convention already used for "bash" above.
	if !globalCredentialGuardInstalledCopilot() {
		preToolUse = append(preToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "bash",
			"bash":       "scripts/trackfw-credential-guard.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		})
		preToolUse = append(preToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "view",
			"bash":       "scripts/trackfw-credential-guard.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		})
		preToolUse = append(preToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "create|edit",
			"bash":       "scripts/trackfw-credential-guard.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		})
		postToolUse = append(postToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "bash",
			"bash":       "scripts/trackfw-credential-guard.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		})
		postToolUse = append(postToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "view",
			"bash":       "scripts/trackfw-credential-guard.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		})
		postToolUse = append(postToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "create|edit",
			"bash":       "scripts/trackfw-credential-guard.sh",
			"cwd":        ".",
			"timeoutSec": 10,
		})
	}

	content := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse":  preToolUse,
			"postToolUse": postToolUse,
		},
	}

	out, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// InjectCursorHooks injects Cursor attention hooks into .cursor/hooks.json.
//
// Two independent things are wired here, both nested under the real Cursor
// hook config `{"version": 1, "hooks": {"<eventName>": [...] }}`:
//   - hooks.preToolUse + hooks.postToolUse (migrated by this ML) —
//     attention-signal/cleanup. Prior to this ML these were written to
//     top-level preToolUse/postToolUse arrays, which did not match any
//     documented Cursor event (confirmed 2026-08-05, see docs/cli-parity.md
//     "Cursor wiring (ML-2E)"). Re-fetching https://cursor.com/docs/hooks on
//     2026-08-06 (the /docs/agent/hooks URL now 308-redirects there) shows
//     Cursor's docs were updated in the interim to add three new generic
//     events: preToolUse/postToolUse/postToolUseFailure, "fires for all tool
//     types (Shell, Read, Write, MCP, Task, etc.)". preToolUse's documented
//     input is `{"tool_name","tool_input":{...},"tool_use_id","cwd",...}`
//     and postToolUse's is the same shape plus `tool_output`/`duration` —
//     structurally identical to Claude Code's PreToolUse/PostToolUse payload
//     (`tool_name`/`tool_input`), which is exactly the shape
//     scripts/trackfw-attention-signal.sh and trackfw-attention-cleanup.sh
//     already parse (`.tool_name`, `.tool_input.question // .tool_input.command`).
//     No script changes were needed. Per-hook `matcher` filters by tool type
//     (e.g. "Shell|Read|Write") and is optional; intentionally omitted here,
//     same reasoning as beforeShellExecution below — the attention signal
//     must fire for every tool use, not a filtered subset.
//   - hooks.beforeShellExecution + hooks.afterShellExecution (ML-2E, prior
//     cycle) — credential-guard. beforeShellExecution is the real,
//     Bash-specific, pre-execution event: input is `{"command","cwd","sandbox"}`,
//     response (stdout JSON, only read on exit code 0) is
//     `{"permission":"allow"|"deny"|"ask","user_message":"...",
//     "agent_message":"..."}`. Per the documented "Exit code behavior": exit 0 uses the
//     JSON output (or defaults to allow if stdout has none — confirmed by the doc's own
//     minimal example hook, which exits 0 with no stdout at all), exit 2 blocks the
//     action ("equivalent to returning permission: \"deny\""), any other exit code
//     fail-opens (hook failed, action proceeds). This is already exactly
//     trackfw-credential-guard.sh's existing contract (block mode → exit 2 + stderr, warn
//     mode → exit 0), so no script changes were needed to wire Cursor. afterShellExecution
//     is a post-execution audit-only event (input adds "output"/"duration", no
//     allow/deny/ask response defined) — added in parallel for symmetry with the
//     PostToolUse wiring already used for the other CLIs in this wave, so the guard also
//     gets a chance to flag credentials that only appear in captured command output.
//     Concurrency between hooks registered on the same event was not documented on the
//     page retrieved for this investigation (unlike Codex, which explicitly documents
//     concurrent execution); not assumed either way. Not a blocker here regardless: this
//     event array only ever contains the single credential-guard entry added by trackfw.
//
// Backward compatibility: a `.cursor/hooks.json` written by a pre-migration
// trackfw still has the legacy top-level preToolUse/postToolUse arrays. This
// function migrates known trackfw entries out of those top-level arrays into
// the nested hooks.preToolUse/hooks.postToolUse location, and drops the
// top-level key entirely once it is empty — but never touches or deletes
// unrelated entries a user may have added there themselves (those keys are
// inert either way — Cursor never read the top-level location — so leaving
// them is harmless and avoids destroying unrelated user data on a guess).
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

	if _, ok := root["version"]; !ok {
		root["version"] = 1
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	// Migrate any legacy top-level preToolUse/postToolUse trackfw entries
	// (written by trackfw before this ML) into the nested, real hooks.
	hooks["preToolUse"] = mergeSimpleCommandArray(hooks["preToolUse"], "scripts/trackfw-attention-signal.sh", makeEntry, getCmd)
	hooks["postToolUse"] = mergeSimpleCommandArray(hooks["postToolUse"], "scripts/trackfw-attention-cleanup.sh", makeEntry, getCmd)
	removeKnownCommandFromLegacyTopLevelArray(root, "preToolUse", "scripts/trackfw-attention-signal.sh", getCmd)
	removeKnownCommandFromLegacyTopLevelArray(root, "postToolUse", "scripts/trackfw-attention-cleanup.sh", getCmd)

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to Read/Write via the generic
	// preToolUse/postToolUse events): skip the project-scope credential-guard
	// entries when the global one is already installed
	// (`trackfw update harness --targets cursor-credential-guard`).
	if !globalCredentialGuardInstalledCursor() {
		hooks["beforeShellExecution"] = mergeSimpleCommandArray(hooks["beforeShellExecution"], "scripts/trackfw-credential-guard.sh", makeEntry, getCmd)
		hooks["afterShellExecution"] = mergeSimpleCommandArray(hooks["afterShellExecution"], "scripts/trackfw-credential-guard.sh", makeEntry, getCmd)

		// Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): wired via
		// the generic preToolUse/postToolUse events (distinct from
		// beforeShellExecution/afterShellExecution, which only ever fire for
		// Shell) with an explicit "matcher", so these entries never fire for
		// the same tool call the unfiltered attention-signal/cleanup entries
		// already handle above in this same array. mergeSimpleCommandArray
		// (command-only dedup) is not enough here — both the unfiltered
		// signal entry and these matcher-scoped guard entries share the same
		// array, so dedup must also check "matcher".
		hooks["preToolUse"] = mergeCursorGuardMatcherEntry(hooks["preToolUse"], "Read", "scripts/trackfw-credential-guard.sh")
		hooks["preToolUse"] = mergeCursorGuardMatcherEntry(hooks["preToolUse"], "Write", "scripts/trackfw-credential-guard.sh")
		hooks["postToolUse"] = mergeCursorGuardMatcherEntry(hooks["postToolUse"], "Read", "scripts/trackfw-credential-guard.sh")
		hooks["postToolUse"] = mergeCursorGuardMatcherEntry(hooks["postToolUse"], "Write", "scripts/trackfw-credential-guard.sh")
	}
	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// removeKnownCommandFromLegacyTopLevelArray drops a single known trackfw
// entry (matched by command) from a legacy top-level array in root[key], and
// removes the key entirely once empty. Any other entries in the array (not
// matching command) are left untouched — see InjectCursorHooks doc comment.
func removeKnownCommandFromLegacyTopLevelArray(root map[string]interface{}, key, command string, getCmd func(interface{}) string) {
	arr, ok := root[key].([]interface{})
	if !ok {
		return
	}
	kept := arr[:0]
	for _, item := range arr {
		if getCmd(item) == command {
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == 0 {
		delete(root, key)
		return
	}
	root[key] = kept
}

// migrateHookCommand rewrites a legacy hook command to a new one, in place,
// for every entry matching the given matcher inside a "matcher + hooks[].command"
// shaped array — the format shared by Claude, Codex and Gemini's merge-based
// settings files (PreToolUse/PostToolUse/PermissionRequest/Notification/
// BeforeTool/AfterTool). Used to fix settings files already written by an
// older trackfw before a command string changes — without this, re-running
// `trackfw init`/`update` only ever appends the new (fixed) command alongside
// the stale one (merge dedup in mergeClaudeHookArray keys on the exact
// command string, so it can't tell "same guard, new path" from "a different
// hook"), leaving the broken entry in place to keep firing and failing
// forever. Originally written for Claude only (hence the doc comment history
// below); generalized (ROADMAP-2026-08-11 ML-1A) so Codex/Gemini injectors
// can call it too, ahead of the mechanism-specific string changes those CLIs'
// waves make. Must always be called before the corresponding
// mergeClaudeHookArray call for the same matcher, or the merge's exact-string
// dedup will append a duplicate instead of rewriting in place.
func migrateHookCommand(existing interface{}, matcher, oldCommand, newCommand string) {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == oldCommand {
				hObj["command"] = newCommand
			}
		}
	}
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

// mergeCursorGuardMatcherEntry appends {"command": command, "matcher": matcher}
// to a Cursor preToolUse/postToolUse array unless an entry with that exact
// (command, matcher) pair already exists. Distinct from mergeSimpleCommandArray
// (which dedups on command alone) because these arrays also hold the
// unfiltered attention-signal/cleanup entries — see InjectCursorHooks'
// Read/Write wiring comment (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2).
func mergeCursorGuardMatcherEntry(existing interface{}, matcher, command string) []interface{} {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if obj["command"] == command && obj["matcher"] == matcher {
			return arr
		}
	}
	return append(arr, map[string]interface{}{"command": command, "matcher": matcher})
}

// --- Global credential-guard dedup (ROADMAP-2026-08-06 Wave 3/ML-3A) ---
//
// InjectClaudeHooks/InjectCodexHooks/InjectGeminiHooks/InjectCursorHooks/
// InjectCopilotHooks/InjectKiroHooks each check, read-only, whether the
// user already has the global-scope credential-guard wiring installed for
// that CLI (via `trackfw update harness --targets <tool>-credential-guard`,
// internal/generators/update.go) before adding the project-scope
// credential-guard entry. If the global entry is already present, the
// project-scope entry is skipped entirely (never running the guard twice
// per command) — attention-signal/cleanup entries are unaffected, since
// those are inherently project-scoped (ADR-2026-08-06, Decision #4).
//
// Fail-open is mandatory: any failure to resolve $HOME, read the global
// file, or parse its JSON is treated as "not installed globally" and the
// project-scope entry is added exactly as before this ML. This function
// never writes to the global file — read-only by construction (no
// os.WriteFile call anywhere in this section).

// globalCredentialGuardScriptPath resolves the absolute path the global
// credential-guard wiring would point at (~/.trackfw/scripts/trackfw-
// credential-guard.sh), matching harnessCredentialGuardTargetClaude/Codex/
// Gemini/Cursor/Copilot/Kiro (internal/generators/update.go) exactly. Returns
// ok=false if $HOME cannot be resolved (fail-open: caller treats this as
// "not installed globally").
func globalCredentialGuardScriptPath() (path string, ok bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh"), true
}

// readGlobalHookJSON reads and parses a JSON object at $HOME/<relParts...>.
// Returns ok=false on any failure (file missing, unreadable, not valid JSON,
// or $HOME unresolvable) — the fail-open contract for every caller in this
// section.
func readGlobalHookJSON(relParts ...string) (root map[string]interface{}, ok bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, false
	}
	parts := append([]string{home}, relParts...)
	raw, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		return nil, false
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, false
	}
	return root, true
}

// hookArrayHasCommand reports whether a Claude/Codex/Gemini-shaped hook
// array (matcher → {"hooks":[{"command"}]}) already contains command under
// matcher. Read-only counterpart of mergeClaudeHookArray.
func hookArrayHasCommand(existing interface{}, matcher, command string) bool {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		inner, _ := obj["hooks"].([]interface{})
		for _, h := range inner {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				return true
			}
		}
	}
	return false
}

// simpleArrayHasValue reports whether a flat hook array (Cursor's
// {"command":...} or Copilot's {"bash":...} shape) already has an entry
// with field == value. Read-only counterpart of mergeSimpleCommandArray.
func simpleArrayHasValue(existing interface{}, field, value string) bool {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if ok && obj[field] == value {
			return true
		}
	}
	return false
}

// globalCredentialGuardInstalledClaude checks ~/.claude/settings.json for
// the PreToolUse[matcher:"Bash"] entry harnessCredentialGuardTargetClaude
// writes. Fail-open: any read/parse error → false.
func globalCredentialGuardInstalledClaude() bool {
	scriptPath, ok := globalCredentialGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".claude", "settings.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath)
}

// globalCredentialGuardInstalledCodex checks ~/.codex/hooks.json for the
// PreToolUse[matcher:"Bash"] entry harnessCredentialGuardTargetCodex writes.
// Fail-open: any read/parse error → false.
func globalCredentialGuardInstalledCodex() bool {
	scriptPath, ok := globalCredentialGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".codex", "hooks.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath)
}

// globalCredentialGuardInstalledGemini checks ~/.gemini/settings.json for
// the BeforeTool[matcher:"run_shell_command"] entry
// harnessCredentialGuardTargetGemini writes. Fail-open: any read/parse
// error → false.
func globalCredentialGuardInstalledGemini() bool {
	scriptPath, ok := globalCredentialGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".gemini", "settings.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", scriptPath)
}

// globalCredentialGuardInstalledCursor checks ~/.cursor/hooks.json for the
// hooks.beforeShellExecution entry harnessCredentialGuardTargetCursor
// writes. Fail-open: any read/parse error → false.
func globalCredentialGuardInstalledCursor() bool {
	scriptPath, ok := globalCredentialGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".cursor", "hooks.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return simpleArrayHasValue(hooks["beforeShellExecution"], "command", scriptPath)
}

// globalCredentialGuardInstalledCopilot checks ~/.copilot/settings.json for
// the hooks.preToolUse[bash] entry harnessCredentialGuardTargetCopilot
// writes. Fail-open: any read/parse error → false.
func globalCredentialGuardInstalledCopilot() bool {
	scriptPath, ok := globalCredentialGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".copilot", "settings.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return simpleArrayHasValue(hooks["preToolUse"], "bash", scriptPath)
}

// globalCredentialGuardInstalledKiro checks whether
// ~/.kiro/hooks/trackfw-credential-guard.json exists and is non-empty — this
// file is 100% dedicated to the global credential-guard wiring
// (harnessCredentialGuardTargetKiro overwrites it wholesale, never merges),
// so presence + non-empty content is sufficient, matching the roadmap's
// explicit instruction for Kiro. Fail-open: any stat error → false.
func globalCredentialGuardInstalledKiro() bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json"))
	if err != nil {
		return false
	}
	return info.Size() > 0
}
