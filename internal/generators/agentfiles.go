package generators

import (
	"encoding/json"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
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

func trackfwRulesBlock(agentConventions string) string {
	conventionsSection := ""
	if strings.TrimSpace(agentConventions) != "" {
		conventionsSection = `

### Project Conventions
> Declared by the team in ` + "`trackfw.yaml`" + `'s ` + "`agent_conventions`" + ` field — NOT
> inferred automatically. trackfw does not impose an architectural standard; it only
> propagates what the project has already decided.

` + strings.TrimSpace(agentConventions) + `
`
	}

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
- **Threat model waves:** every feature roadmap opens with a Wave 0 threat model (before implementation) and closes with a red-team review wave (before release)
- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)
- Use ` + "`/trackfw:architect`" + ` to define stack before the first REQ
` + conventionsSection + `
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
func injectOrUpdateRules(filePath, headerIfNew, cwd string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	block := trackfwRulesBlock(config.ReadAgentConventions(cwd))

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
	return injectOrUpdateRules(filepath.Join(cwd, relPath), header, cwd)
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

	// Git branch guard (ROADMAP-2026-08-14 ML-3A): blocks raw `git commit`/
	// `git push`/`git checkout -b` for Bash calls.
	// migrateHookCommand is a deliberate old==new no-op today (see its doc
	// comment / the Gemini injector for the same pattern): it proves the call
	// point exists and runs before the merge, so a future ML that changes
	// this command string only needs to update oldCommand here instead of
	// adding the call from scratch. Always runs regardless of dedup state
	// below (a migration must fire even when the project-scope entry is
	// about to be skipped, so a stale entry never lingers unmigrated).
	migrateHookCommand(hooks["PreToolUse"], "Bash", claudeGitGuardCmd, claudeGitGuardCmd)
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets claude-git-branch-guard`), so the
	// guard doesn't fire twice per Bash call and print the block message
	// twice.
	if !globalGitBranchGuardInstalledClaude() {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Bash",
			claudeGitGuardCmd,
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

// ROADMAP-2026-08-11 ML-4A: Gemini CLI documents $GEMINI_PROJECT_DIR (distinct
// from the session-following $GEMINI_CWD) and uses it in 100% of its official
// hook command examples (ADR-2026-08-11, "Gemini CLI — alterar, por argumento
// de assimetria"). Unlike Codex's $(git rev-parse …), this is an env var
// expanded by the Gemini CLI runtime itself — no shell substitution needed, no
// literal quotes required.
const (
	geminiSignalCmd  = `$GEMINI_PROJECT_DIR/scripts/trackfw-attention-signal.sh`
	geminiCleanupCmd = `$GEMINI_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh`
	geminiGuardCmd   = `$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh`
)

// --- Git branch guard command paths (ROADMAP-2026-08-14 ML-3A) ---
//
// Wires scripts/trackfw-git-branch-guard.sh (generated by
// GenerateGitBranchGuardScript, internal/generators/scaffold.go) into every
// runtime's PreToolUse-equivalent hook, using the exact same project-root
// resolution mechanism already proven for the credential-guard above (per
// runtime: $CLAUDE_PROJECT_DIR, `$(git rev-parse --show-toplevel)`,
// $GEMINI_PROJECT_DIR). Unlike credential-guard, this guard is block-only
// (no "warn" mode) and only needs to run *before* the tool call — no
// PostToolUse/afterShellExecution wiring is added for it.
const (
	claudeGitGuardCmd = "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"
	codexGitGuardCmd  = codexRoot + `/scripts/trackfw-git-branch-guard.sh"`
	geminiGitGuardCmd = `$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`
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

	// Git branch guard (ROADMAP-2026-08-14 ML-3A): wired via the same
	// PreToolUse/Bash hook mechanism already proven stable for
	// credential-guard above (this file's own doc comment on InjectCodexHooks
	// confirms Codex hooks are enabled by default, not opt-in/experimental —
	// so the roadmap's "Rules vs experimental hook" fork resolves to "hook",
	// consistent with the rest of this function; see docs/cli-parity.md for
	// the recorded decision). Only "Bash" matters here (apply_patch never
	// carries a raw git subcommand). migrateHookCommand always runs
	// regardless of the dedup check below (ROADMAP-2026-08-17 Wave 2/ML-2B).
	migrateHookCommand(hooks["PreToolUse"], "Bash", codexGitGuardCmd, codexGitGuardCmd)
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets codex-git-branch-guard`).
	if !globalGitBranchGuardInstalledCodex() {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Bash",
			codexGitGuardCmd,
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
	migrateHookCommand(hooks["Notification"], "ToolPermission", "scripts/trackfw-attention-signal.sh", geminiSignalCmd)
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", "scripts/trackfw-credential-guard.sh", geminiGuardCmd)
	migrateHookCommand(hooks["BeforeTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", geminiGuardCmd)
	migrateHookCommand(hooks["BeforeTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", geminiGuardCmd)
	migrateHookCommand(hooks["AfterTool"], "*", "scripts/trackfw-attention-cleanup.sh", geminiCleanupCmd)
	migrateHookCommand(hooks["AfterTool"], "run_shell_command", "scripts/trackfw-credential-guard.sh", geminiGuardCmd)
	migrateHookCommand(hooks["AfterTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", geminiGuardCmd)
	migrateHookCommand(hooks["AfterTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", geminiGuardCmd)

	hooks["Notification"] = mergeClaudeHookArray(
		hooks["Notification"],
		"ToolPermission",
		geminiSignalCmd,
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
			geminiGuardCmd,
		)
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"read_file|read_many_files",
			geminiGuardCmd,
		)
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"write_file|replace",
			geminiGuardCmd,
		)
	}

	// Git branch guard (ROADMAP-2026-08-14 ML-3A): only "run_shell_command"
	// can ever carry a raw git subcommand, so unlike credential-guard this is
	// not also wired to the read_file/write_file matchers.
	//
	// Native subagent toolset restriction (REQ acceptance criterion — Gemini
	// CLI supports per-agent restricted toolsets, keeping the architect
	// unrestricted): NOT implemented here. No generator for Gemini custom
	// subagent definitions (`.gemini/agents` or equivalent) exists anywhere
	// in this codebase (confirmed via grep across internal/generators before
	// writing this function) — building one from scratch is out of scope for
	// this ML per the roadmap's own instruction ("não invente um gerador de
	// subagentes do zero, fora de escopo"). This hook therefore applies
	// uniformly to every Gemini agent, architect included, exactly like the
	// pre-existing credential-guard hook above. See docs/cli-parity.md.
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", geminiGitGuardCmd, geminiGitGuardCmd)
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets gemini-git-branch-guard`).
	if !globalGitBranchGuardInstalledGemini() {
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"run_shell_command",
			geminiGitGuardCmd,
		)
	}

	hooks["AfterTool"] = mergeClaudeHookArray(
		hooks["AfterTool"],
		"*",
		geminiCleanupCmd,
	)
	if !skipGeminiCG {
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"run_shell_command",
			geminiGuardCmd,
		)
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"read_file|read_many_files",
			geminiGuardCmd,
		)
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"write_file|replace",
			geminiGuardCmd,
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

	// Git branch guard (ROADMAP-2026-08-14 ML-3A): the roadmap describes this
	// mechanism as `--deny-tool='shell(git commit)'`-style CLI flags in a
	// permissions-config.json/settings.json file, but no such file/flag is
	// referenced anywhere else in this codebase — Copilot's only established
	// deny-adjacent mechanism here is this same preToolUse/postToolUse hooks
	// file already used for credential-guard above. Following that precedent
	// instead of inventing a new config surface; documented as a deliberate
	// divergence from the roadmap's literal wording in docs/cli-parity.md.
	// This file is overwritten wholesale every run (doc comment above), but
	// that only means there is no MIGRATION concern (nothing stale to leave
	// behind) — it does not exempt this entry from the dedup-against-global
	// check (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets copilot-git-branch-guard`), same
	// reasoning as the credential-guard dedup above.
	if !globalGitBranchGuardInstalledCopilot() {
		preToolUse = append(preToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "bash",
			"bash":       "scripts/trackfw-git-branch-guard.sh",
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

	// Git branch guard (ROADMAP-2026-08-14 ML-3A): wired via
	// beforeShellExecution, the same event already used for
	// credential-guard. No script change was needed for Cursor's
	// `permission: "deny"` contract — this file's own doc comment above
	// confirms exit code 2 alone is "equivalent to returning
	// permission: \"deny\"", and stdout JSON is only consulted on exit 0;
	// trackfw-git-branch-guard.sh always exits 2 on a match, so the existing
	// script output is sufficient without adding a `permission` field to the
	// byte-parity-tested gitBranchGuardScript constant (scaffold.go).
	//
	// The roadmap also asks for a static `Shell(git:commit)`/`Shell(git:push)`
	// deny layer in `.cursor/rules` as defense-in-depth. NOT added here: this
	// codebase has no established mechanism for tool-specific supplementary
	// directives inside `.cursor/rules/trackfw.mdc` (that file only carries
	// the shared, cross-tool trackfwRulesBlock() text — adding a
	// Cursor-specific deny clause there would require either a new templating
	// mechanism or leaking a Cursor-only concept into the shared block used
	// by all 7 runtimes) and the Cursor `.mdc` frontmatter format documents
	// no declarative shell-deny field to confirm. Documented as a known gap
	// in docs/cli-parity.md rather than guessed at.
	//
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets cursor-git-branch-guard`). The key
	// is intentionally only touched inside this conditional (never a
	// standalone `hooks["beforeShellExecution"] = hooks["beforeShellExecution"]`
	// outside it) so that when BOTH credential-guard and git-branch-guard are
	// deduped away, the key stays absent from the emitted JSON rather than
	// becoming a present-but-empty array — matching the shape the
	// credential-guard dedup above already produces in the equivalent case,
	// which check-agent-hooks-parity.sh's structural comparator treats as
	// significant (absent key vs empty array is drift, not noise).
	if !globalGitBranchGuardInstalledCursor() {
		hooks["beforeShellExecution"] = mergeSimpleCommandArray(hooks["beforeShellExecution"], "scripts/trackfw-git-branch-guard.sh", makeEntry, getCmd)
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

// windsurfGitGuardCmd is the command entry trackfw registers under
// `hooks.pre_run_command` in `.windsurf/hooks.json`. Windsurf invokes the
// hook via a shell (see InjectWindsurfHooks doc), so the guard script is
// wrapped in `bash <path>` rather than invoked directly.
const windsurfGitGuardCmd = "bash scripts/trackfw-git-branch-guard.sh"

// legacyWindsurfHooksFile is the path this same function wrote to before the
// path/schema fix documented below (ROADMAP-2026-08-14 ML-3A originally
// invented this path without confirming it against official docs).
const legacyWindsurfHooksFile = "trackfw-git-branch-guard.json"

// InjectWindsurfHooks updates .windsurfrules with the attention instruction,
// and registers the `pre_run_command` git-branch-guard hook in Windsurf's
// real hooks file.
//
// Path/schema correction (apolo-tf, 2026-08-14, post-ML-3A audit): the
// original ML-3A implementation wrote a dedicated, wholly-owned file at
// `.windsurf/hooks/trackfw-git-branch-guard.json` with an invented payload
// shape (`{"version":1,"hooks":[{"name":...,"trigger":"pre_run_command",
// "action":{...}}]}`) that was flagged in its own doc comment as UNCONFIRMED
// against official documentation. A verification pass against
// https://docs.devin.ai/desktop/cascade/hooks confirmed both the path and
// the shape were wrong:
//   - Windsurf reads hooks from a single fixed-name file, `.windsurf/hooks.json`
//     — NOT a directory of per-hook files under `.windsurf/hooks/`.
//   - The schema is `{"hooks": {"<event>": [{"command": "...", "show_output":
//     bool}]}}` — an object keyed by event name (e.g. "pre_run_command",
//     "post_run_command"), each mapping to an ARRAY of hook defs. There is no
//     "name"/"trigger"/"action" envelope.
//   - The hook script receives its context via stdin as JSON, including
//     `tool_info.command_line` — gitBranchGuardScript (scaffold.go) now tries
//     this field explicitly (in addition to the generic `.command`/
//     `.tool_input.command`/`.hook_input.command` fields it already handled).
//
// Merge is idempotent and shaped like every other multi-tool settings file in
// this package: existing `pre_run_command` entries from other tools (or a
// prior trackfw run) are preserved; only an entry with our exact command
// string is deduped via mergeSimpleCommandArray. Other events already present
// (e.g. a user- or third-party-authored `post_run_command`) are left
// untouched.
//
// Migration: if the stale `.windsurf/hooks/trackfw-git-branch-guard.json`
// file from the incorrect ML-3A version exists on disk, it is removed here
// (never left orphaned) — same "migrate before merge" discipline as
// migrateHookCommand elsewhere in this file, just at the file level since the
// whole file (not just one entry) moved.
//
// `windsurf.cascadeCommandsAllowList` (an IDE *user settings* key, not a
// project-local file) remains out of scope — same reasoning as before this
// fix: trackfw has no established, confirmed mechanism for rewriting IDE user
// settings safely, and inventing one on a guess repeats the exact mistake
// this fix corrects. Documented as an open gap in docs/cli-parity.md.
func InjectWindsurfHooks(cwd string) error {
	if err := InjectRulesForTool("windsurf", cwd); err != nil {
		return err
	}

	// Migration: remove the incorrect, previously-written dedicated hook file
	// from an older (buggy) trackfw run, so it doesn't linger as a dead,
	// never-consumed artifact once the correct .windsurf/hooks.json exists.
	legacyPath := filepath.Join(cwd, ".windsurf", "hooks", legacyWindsurfHooksFile)
	if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Best-effort cleanup of the now-possibly-empty legacy directory; ignore
	// failure (non-empty dir, e.g. holding unrelated user files, or already
	// gone) — never fatal.
	_ = os.Remove(filepath.Join(cwd, ".windsurf", "hooks"))

	dir := filepath.Join(cwd, ".windsurf")
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

	hooks["pre_run_command"] = mergeSimpleCommandArray(
		hooks["pre_run_command"],
		windsurfGitGuardCmd,
		func(cmd string) interface{} {
			return map[string]interface{}{
				"command":     cmd,
				"show_output": true,
			}
		},
		func(item interface{}) string {
			obj, ok := item.(map[string]interface{})
			if !ok {
				return ""
			}
			s, _ := obj["command"].(string)
			return s
		},
	)
	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// amazonQCliAgentsDir / amazonQDefaultAgentFile identify the Amazon Q
// Developer CLI custom-agent file trackfw manages
// (.amazonq/cli-agents/q_cli_default.json).
const amazonQCliAgentsDir = "cli-agents"
const amazonQDefaultAgentFile = "q_cli_default.json"

// InjectAmazonQHooks injects Amazon Q Developer CLI git branch guard wiring
// into a custom agent file, .amazonq/cli-agents/q_cli_default.json.
//
// Path correction (apolo-tf, 2026-08-14, post-ML-3A audit): the original
// ML-3A implementation wrote `hooks`/`toolsSettings` to `.amazonq/settings.json`
// (flagged in its own doc comment as unconfirmed against official docs). A
// verification pass against
// https://docs.aws.amazon.com/amazonq/latest/qdeveloper-ug/command-line-custom-agents-configuration.html
// and
// https://docs.aws.amazon.com/amazonq/latest/qdeveloper-ug/command-line-agents-default-behavior.html
// confirmed there is no `.amazonq/settings.json` for this purpose: `hooks`
// and `toolsSettings` are top-level fields of a named **custom agent**
// file under `.amazonq/cli-agents/<name>.json`, not of a shared settings
// file. This is a BREAKING CHANGE of a development-only path from an
// unreleased roadmap (this REQ has not shipped yet, so there are no real
// users on the old `.amazonq/settings.json` path to migrate) — the stale
// file, if present from a prior run of the buggy version, is intentionally
// left untouched rather than auto-migrated (it is a genuinely different file
// now, not a rename).
//
// File name (`q_cli_default.json`, not an arbitrary name like
// `trackfw-guard.json`): a custom agent only takes effect if it is the
// *active* agent (`q chat --agent <name>`, the `chat.defaultAgent` setting,
// or — the closest thing to "activates automatically without a manual flag"
// — being named `q_cli_default.json`). Known limitation, documented rather
// than worked around: AWS has an open bug where this default-name override is
// not always honored (github.com/aws/amazon-q-developer-cli#2922) — a custom
// agent named `q_cli_default.json` is not guaranteed to always be picked up
// automatically depending on CLI version/config.
//
// Two guard mechanisms are wired, per REQ-2026-08-14's confirmed Amazon Q
// contract ("hook `preToolUse` confirmado", "`deniedCommands` com regex,
// avaliado antes do allow") — internal shape unchanged from the original
// ML-3A implementation, only the target file moved:
//   - hooks.preToolUse[matcher:"execute_bash"] → the guard script, same
//     matcher+hooks[].command shape already used by Claude/Codex/Gemini in
//     this file (reuses mergeClaudeHookArray for idempotent merge).
//   - toolsSettings.execute_bash.deniedCommands → a regex denylist evaluated
//     before allow, independent of and in addition to the hook (defense in
//     depth, same reasoning as Cursor's static Shell(git:commit) layer).
//
// Native custom-agent toolset restriction (REQ acceptance criterion — Amazon
// Q supports `tools`/`allowedTools` on custom agents, keeping the architect
// unrestricted): still NOT implemented here — this ML only wires the
// guard/deny fields on the one default agent file; per-specialist-agent
// toolset restriction is out of scope, same limitation already accepted for
// Gemini above. `tools: ["*"]` is written on first creation so the default
// agent keeps today's unrestricted tool access (this fix does not narrow
// what any agent can do, only where the deny wiring lives).
func InjectAmazonQHooks(cwd string) error {
	path := filepath.Join(cwd, ".amazonq", amazonQCliAgentsDir, amazonQDefaultAgentFile)
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

	// Root-level fields written on first creation, kept deliberately minimal:
	// this ML confirmed the *file path* (.amazonq/cli-agents/<name>.json) and
	// the *shape of hooks/toolsSettings* against
	// command-line-custom-agents-configuration.html, but did NOT have network
	// access to fetch and cross-check the doc's complete custom-agent JSON
	// schema in this session — so only the fields load-bearing for this ML's
	// purpose are written: `name` (required for the file to identify itself
	// as an agent and for the "activates by filename" behavior documented
	// above) and `tools: ["*"]` (preserves today's unrestricted tool access —
	// dropping it would silently narrow what the default agent can do).
	// `description` is included as a harmless free-text field. Optional
	// fields seen in other custom-agent examples elsewhere (`prompt`,
	// `mcpServers`, `toolAliases`, `allowedTools`, `resources`,
	// `useLegacyMcpJson`, a `$schema` pointer) are deliberately NOT written
	// here: an extra field the real schema doesn't expect risks failing
	// validation, whereas an absent optional field usually doesn't. Flagged
	// for the auditing agent: verify this defaults set against the live doc
	// (or a real `q chat --agent` run) before treating it as final — only
	// set for fields not already present, so re-running against a
	// hand-edited or previously-generated file never clobbers user
	// customization, same "preserve existing settings" contract as every
	// other merge-based injector in this file.
	defaults := map[string]interface{}{
		"name":        "q_cli_default",
		"description": "trackfw-managed default agent — wires the git branch guard hook/denylist. See docs/cli-parity.md.",
		"tools":       []interface{}{"*"},
	}
	for k, v := range defaults {
		if _, exists := root[k]; !exists {
			root[k] = v
		}
	}

	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	const amazonQGitGuardCmd = "scripts/trackfw-git-branch-guard.sh"

	migrateHookCommand(hooks["preToolUse"], "execute_bash", amazonQGitGuardCmd, amazonQGitGuardCmd)
	hooks["preToolUse"] = mergeClaudeHookArray(
		hooks["preToolUse"],
		"execute_bash",
		amazonQGitGuardCmd,
	)
	root["hooks"] = hooks

	toolsSettings, _ := root["toolsSettings"].(map[string]interface{})
	if toolsSettings == nil {
		toolsSettings = make(map[string]interface{})
	}
	execBash, _ := toolsSettings["execute_bash"].(map[string]interface{})
	if execBash == nil {
		execBash = make(map[string]interface{})
	}
	const gitDenyPattern = `^git (commit|push|checkout -b)`
	denied, _ := execBash["deniedCommands"].([]interface{})
	found := false
	for _, d := range denied {
		if s, ok := d.(string); ok && s == gitDenyPattern {
			found = true
			break
		}
	}
	if !found {
		denied = append(denied, gitDenyPattern)
	}
	execBash["deniedCommands"] = denied
	toolsSettings["execute_bash"] = execBash
	root["toolsSettings"] = toolsSettings

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
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
	home, err := homedir.Dir()
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
	home, err := homedir.Dir()
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

// normalizeGuardPath collapses run of consecutive slashes ("//" -> "/", at
// any position, including leading) and strips a trailing slash, so that two
// on-disk forms of the SAME script path compare equal regardless of
// incidental formatting (e.g. $HOME resolving with a trailing slash, as
// happens with macOS's $TMPDIR, or a hand-edited config file). It does NOT
// resolve "." / ".." segments or symlinks — those transforms would let
// unrelated paths compare equal (turning a missing/renamed guard into a
// false "already installed" and silently disarming the dedup — the more
// dangerous failure mode here) and symlink resolution errors on a path that
// does not exist yet, which every caller in this file must fail OPEN on,
// producing the exact silent-false this whole ML exists to close. Hand-rolled
// instead of filepath.Clean/path.normalize/os.path.normpath because those
// three disagree with each other on leading "//" and trailing "/" handling
// (measured) — this algorithm is mirrored byte-for-byte in npm/src/generators/
// hooks.js and pypi/trackfw/generators/hooks.py to keep the three CLIs
// deciding identically. Never call this with anything other than a script
// path — it is not a general string normalizer.
//
// ROADMAP-2026-09-03 ML-7B — Windows separator canonicalization, gated on
// anchoring, NOT a blanket "\\" -> "/" translate. On POSIX "\\" is a legal
// filename byte, so translating it unconditionally would make two
// genuinely different paths (one with a literal backslash in a segment
// name, one with an extra path separator there) compare equal — the exact
// dangerous loosening this function's own doc comment warns against, and
// the risk this ML was told to treat explicitly. The decision, per input
// shape (mirrors the UNC/drive-letter arms of
// internal/validator/pathIsAnchoredForHookConfig, reimplemented locally —
// not imported, different question, and this package must not import
// internal/validator):
//
//   - "C:\Users\x\guard.sh" / "C:/Users/x/guard.sh" (ASCII drive letter,
//     ":", then "\" or "/") — CANONICALIZED: every "\" is translated to "/"
//     before the collapse below runs, so both forms land on the same
//     "C:/Users/x/guard.sh". This is the case ML-7A measured as the actual
//     trigger (a Join()-computed Windows path vs. a hand-concatenated one).
//   - "\\servidor\share\guard.sh" (valid UNC: non-empty SERVER not "." or
//     "..", followed by a non-empty SHARE not itself starting with "\") —
//     UNCHANGED, byte-for-byte, including its backslashes. Translating it
//     would collapse "\\server\share" into "//server/share" and then this
//     function's own "//" -> "/" collapse would eat the second slash,
//     producing "/server/share/..." — indistinguishable from a
//     single-leading-backslash, drive-root-relative path ("\server\share\..."
//     means something else on Windows) or from a same-named POSIX path.
//     That collision is precisely a false "already installed": a
//     network-share guard would dedup-match a local one. Left untouched, a
//     UNC command never cross-matches a non-UNC one — no new equality is
//     introduced, so there is nothing to falsify beyond what already held
//     before this ML.
//   - "//servidor/share/guard.sh" (the POSIX-typed equivalent of UNC) —
//     UNCHANGED behavior from before this ML: it does not start with "\",
//     so it never enters the new branch above; it still collapses via the
//     existing "//" -> "/" rule below, same as any other POSIX path. It is
//     intentionally NOT unified with the "\\servidor\share\..." form above
//     (that asymmetry pre-dates this ML and is out of its scope).
//   - "\\" and "\\x" alone (no SHARE segment) — NOT valid UNC by the
//     predicate above, so they fall through unchanged: no drive letter, no
//     valid UNC, no translation. Same behavior as before this ML (they
//     contain no "/", so the collapse below is a no-op on them too).
//   - "C:foo" (drive-relative, no separator after ":") and homoglyph/
//     zero-width-prefixed strings (e.g. "ｃ:\...", "\u200bC:\...") — do NOT
//     match the ASCII-only, position-0 drive-letter check, so no
//     translation happens. Same anti-spoofing posture as the validator's
//     isASCIIDriveLetter.
//
// Known residual, documented not fixed (hades-tf ML-7B barrier review,
// 2026-09-05 parecer): three real Windows-with-"\" shapes have no drive
// letter at position 0, so hasWindowsDriveLetterPrefix's gate leaves them
// uncanonicalized -- the same pre-ML-7A defect survives for them. Direction
// is always TIGHTENS (possible duplicate hook entry), never loosens (the
// guard is never silently skipped) -- same safe direction as every other
// case in this comment:
//
//   - "\\?\C:\Users\x\guard.sh" (the Win32 long-path prefix; a real form
//     Windows/long-path APIs produce automatically, not a hypothetical).
//   - A relative path containing "\" (e.g. "guard\scripts\hook.sh"). Low
//     practical risk: today both sides of the real comparison always come
//     from filepath.Join with an absolute home, so a relative command
//     should not reach this function -- but nothing in the comparator
//     itself prevents it.
//   - A home resolved via a network-profile UNC path (e.g.
//     "\\fileserver\homes\kg\.trackfw\scripts\..."). It never enters
//     the drive-letter branch and is intentionally left untouched (see the
//     UNC bullet above), so the original defect persists for both UNC
//     spellings, not just between UNC and drive-letter forms.
//
// Not fixed here on purpose: closing any of the three would touch the
// drive-letter gate this barrier just approved as conservative, trading a
// duplicate-entry nuisance for the more expensive failure mode (collapsing
// genuinely different paths into a false "already installed"). Do not treat
// rediscovering these three as a new finding.
func normalizeGuardPath(p string) string {
	if p == "" {
		return p
	}
	if hasWindowsDriveLetterPrefix(p) {
		p = strings.ReplaceAll(p, `\`, "/")
	}
	var b strings.Builder
	prevSlash := false
	for _, r := range p {
		if r == '/' {
			if prevSlash {
				continue
			}
			prevSlash = true
		} else {
			prevSlash = false
		}
		b.WriteRune(r)
	}
	out := b.String()
	if len(out) > 1 && strings.HasSuffix(out, "/") {
		out = strings.TrimRight(out, "/")
		if out == "" {
			out = "/"
		}
	}
	return out
}

// hasValidUNCPrefix reports whether p begins with a Windows UNC prefix
// ("\\server\share...") with a non-empty SERVER segment (not "." or "..")
// followed by a non-empty SHARE segment that does not itself start with
// another backslash. Mirrors the UNC arm of
// internal/validator/pathIsAnchoredForHookConfig — reimplemented here (not
// imported: that predicate answers "is this anchored for a hook config
// value", a different question, and this package must not import
// internal/validator). "\\", "\\x" (no share segment) and "\\.\x" /
// "\\..\evil" (server "." or "..") are NOT valid UNC — same call the
// validator made in ROADMAP-2026-08-21 ML-3B. Currently unused by
// normalizeGuardPath itself (a valid-UNC string never has a drive-letter
// prefix, so the two checks are already mutually exclusive by construction)
// — kept as a named, tested predicate so the "UNC stays untouched" decision
// in the doc comment above is verifiable by name, not just by absence of a
// call.
func hasValidUNCPrefix(p string) bool {
	if len(p) < 2 || p[0] != '\\' || p[1] != '\\' {
		return false
	}
	server, share, found := strings.Cut(p[2:], `\`)
	return found && server != "" && server != "." && server != ".." && share != "" && share[0] != '\\'
}

// hasWindowsDriveLetterPrefix reports whether p begins with an ASCII drive
// letter followed by ":" and a path separator ("C:\..." or "C:/..."). Byte
// check only (mirrors internal/validator's isASCIIDriveLetter) — a Windows
// drive letter is always ASCII; this deliberately does NOT match a
// homoglyph ("ｃ:\..."), a leading zero-width space, a digit before ":", or
// a bare "C:" with no following separator ("C:foo" is drive-relative, not
// anchored — it must NOT be canonicalized here, same call the validator
// makes for hook-config anchoring).
func hasWindowsDriveLetterPrefix(p string) bool {
	if len(p) < 3 {
		return false
	}
	c := p[0]
	isASCIILetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	return isASCIILetter && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

// samePathCommand reports whether a and b denote the same script command
// path after normalizeGuardPath. Use for command-field comparisons only.
func samePathCommand(a, b string) bool {
	return normalizeGuardPath(a) == normalizeGuardPath(b)
}

// hookArrayHasCommand reports whether a Claude/Codex/Gemini-shaped hook
// array (matcher → {"hooks":[{"type":"command","command"}]}) already
// contains command under matcher AND wired with a structurally valid entry
// that this CLI will actually execute. Read-only counterpart of
// mergeClaudeHookArray. Compares command paths via samePathCommand
// (normalized), not raw string equality — see its doc comment.
//
// ROADMAP-2026-08-17 ML-4B: also requires the sibling "type" field to equal
// "command" — mergeClaudeHookArray (this file) always writes
// {"type":"command","command":...}, and Claude/Codex/Gemini all silently
// ignore a hook entry missing "type":"command" (measured, hades-tf ML-4A
// barrier finding). Before this ML, an entry with the correct command but a
// missing/wrong "type" (hand-edited config, older trackfw version, another
// tool's merge) made this function return true — dedup then skipped the
// project-scope entry in favor of a global entry that never actually runs,
// leaving BOTH scopes silently unprotected while `trackfw validate` stayed
// green. Requiring "type":"command" here closes that gap: a malformed
// global entry is now treated as "not installed", so the project-scope
// entry gets re-wired instead of being skipped.
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
			if !ok {
				continue
			}
			if hObj["type"] != "command" {
				continue
			}
			hCommand, ok := hObj["command"].(string)
			if ok && samePathCommand(hCommand, command) {
				return true
			}
		}
	}
	return false
}

// simpleArrayHasValue reports whether a flat hook array (Cursor's
// {"command":...} or Copilot's {"type":"command","bash":...} shape) already
// has an entry with field == value. Read-only counterpart of
// mergeSimpleCommandArray. Compares via samePathCommand (normalized) —
// every caller of this function passes a script path, never an arbitrary
// field value; if that ever changes, do not reuse this helper for non-path
// fields.
//
// ROADMAP-2026-08-17 ML-4B: requireCommandType controls whether a sibling
// "type" field equal to "command" is also required, matching what each
// CLI's own schema demands — mergeCredentialGuardCopilotHooks
// (internal/generators/update.go) always writes "type":"command" and
// Copilot ignores an entry without it (same hades-tf ML-4A finding as
// hookArrayHasCommand above), so Copilot callers pass true. Cursor's
// mergeCredentialGuardCursorHooks entries ({"command":...}) never carry a
// "type" field at all — it is not part of Cursor's schema — so requiring it
// there would make simpleArrayHasValue always return false for a perfectly
// valid, executing Cursor entry; Cursor callers pass false. Do NOT
// uniformize this across CLIs — see the risk note on ML-4B in the roadmap.
func simpleArrayHasValue(existing interface{}, field, value string, requireCommandType bool) bool {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if requireCommandType && obj["type"] != "command" {
			continue
		}
		v, ok := obj[field].(string)
		if ok && samePathCommand(v, value) {
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
	return simpleArrayHasValue(hooks["beforeShellExecution"], "command", scriptPath, false)
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
	return simpleArrayHasValue(hooks["preToolUse"], "bash", scriptPath, true)
}

// globalCredentialGuardInstalledKiro checks whether
// ~/.kiro/hooks/trackfw-credential-guard.json exists and is non-empty — this
// file is 100% dedicated to the global credential-guard wiring
// (harnessCredentialGuardTargetKiro overwrites it wholesale, never merges),
// so presence + non-empty content is sufficient, matching the roadmap's
// explicit instruction for Kiro. Fail-open: any stat error → false.
func globalCredentialGuardInstalledKiro() bool {
	home, err := homedir.Dir()
	if err != nil || home == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json"))
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// --- git-branch-guard global-installed dedup (ROADMAP-2026-08-17 Wave 2/
// ML-2B) ---
//
// Mirrors the globalCredentialGuardInstalled<Tool> family above exactly,
// pointed at ~/.trackfw/scripts/trackfw-git-branch-guard.sh instead of
// trackfw-credential-guard.sh. The global-scope git-branch-guard targets
// added by ML-2A (harnessGitBranchGuardTarget<Tool>, internal/generators/
// update.go) reuse the SAME merge helpers as their credential-guard
// counterparts (mergeCredentialGuardClaudeHooks/GeminiHooks/CursorHooks/
// CopilotHooks, parametrized by scriptPath) — same hooks key, same matcher,
// only the scriptPath argument differs — so the read side below checks the
// exact same hooks key/matcher as globalCredentialGuardInstalled<Tool>, just
// against the git-branch-guard scriptPath.
//
// Only 5 of the 6 credential-guard dedup targets have a git-branch-guard
// counterpart: Kiro's project-scope injector (InjectKiroHooks) never wires
// git-branch-guard at all (it is not one of the CLIs InjectKiroHooks covers
// for this guard — see its doc comment), so there is nothing to dedup there
// and no globalGitBranchGuardInstalledKiro function exists. Windsurf/AmazonQ
// wire git-branch-guard at project scope but have no global-scope target
// (ML-2A only added targets for the 6 CLIs above) and no credential-guard
// dedup precedent either — consistent, not a gap.

// globalGitBranchGuardScriptPath resolves the absolute path the global
// git-branch-guard wiring would point at (~/.trackfw/scripts/trackfw-git-
// branch-guard.sh), matching harnessGitBranchGuardTargetClaude/Codex/Gemini/
// Cursor/Copilot (internal/generators/update.go) exactly. Returns ok=false
// if $HOME cannot be resolved (fail-open: caller treats this as "not
// installed globally").
func globalGitBranchGuardScriptPath() (path string, ok bool) {
	home, err := homedir.Dir()
	if err != nil || home == "" {
		return "", false
	}
	return filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh"), true
}

// globalGitBranchGuardInstalledClaude checks ~/.claude/settings.json for the
// PreToolUse[matcher:"Bash"] entry harnessGitBranchGuardTargetClaude writes.
// Fail-open: any read/parse error → false.
func globalGitBranchGuardInstalledClaude() bool {
	scriptPath, ok := globalGitBranchGuardScriptPath()
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

// globalGitBranchGuardInstalledCodex checks ~/.codex/hooks.json for the
// PreToolUse[matcher:"Bash"] entry harnessGitBranchGuardTargetCodex writes.
// Fail-open: any read/parse error → false.
func globalGitBranchGuardInstalledCodex() bool {
	scriptPath, ok := globalGitBranchGuardScriptPath()
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

// globalGitBranchGuardInstalledGemini checks ~/.gemini/settings.json for the
// BeforeTool[matcher:"run_shell_command"] entry harnessGitBranchGuardTargetGemini
// writes. Fail-open: any read/parse error → false.
func globalGitBranchGuardInstalledGemini() bool {
	scriptPath, ok := globalGitBranchGuardScriptPath()
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

// globalGitBranchGuardInstalledCursor checks ~/.cursor/hooks.json for the
// hooks.beforeShellExecution entry harnessGitBranchGuardTargetCursor writes.
// Fail-open: any read/parse error → false.
func globalGitBranchGuardInstalledCursor() bool {
	scriptPath, ok := globalGitBranchGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".cursor", "hooks.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return simpleArrayHasValue(hooks["beforeShellExecution"], "command", scriptPath, false)
}

// globalGitBranchGuardInstalledCopilot checks ~/.copilot/settings.json for
// the hooks.preToolUse[bash] entry harnessGitBranchGuardTargetCopilot writes.
// Fail-open: any read/parse error → false.
func globalGitBranchGuardInstalledCopilot() bool {
	scriptPath, ok := globalGitBranchGuardScriptPath()
	if !ok {
		return false
	}
	root, ok := readGlobalHookJSON(".copilot", "settings.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return simpleArrayHasValue(hooks["preToolUse"], "bash", scriptPath, true)
}
