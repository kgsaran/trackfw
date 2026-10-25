package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/guardforms"
	"github.com/kgsaran/trackfw/internal/pathguard"
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

// trackfwRulesBlock generates the governance rules block for AI agent configuration files.
// namespacing and agents are used to emit the correct `req new` / `roadmap new` command for
// projects with by_agent namespacing and 2+ agents (AC13, ML-3A).
func trackfwRulesBlock(agentConventions, namespacing string, agents []string) string {
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

	// Compute the step-1 command sequence, conditional on by_agent 2+.
	// The named predicate is validator.IsMultiAgentByAgent; the same check governs the
	// ambiguity error in `req new` (Wave 1). Using it here ensures the documentation
	// matches the command behaviour exactly.
	//
	// AC8 (REQ-2026-09-09): `req new` already creates the linked roadmap; `roadmap new` is
	// no longer in the protocol step — it is for standalone roadmaps without a new REQ.
	step1Req := "`trackfw req new \"title\"`"
	step1Note := ""

	if namespacing == config.NamespacingByAgent {
		var nonEmpty []string
		for _, a := range agents {
			if a != "" {
				nonEmpty = append(nonEmpty, a)
			}
		}
		if len(nonEmpty) >= 2 {
			agentList := strings.Join(nonEmpty, ", ")
			step1Req = "`trackfw req new --agent <agent> \"title\"`"
			step1Note = "\n   ⚠️ `--agent` is required for this project (agents: " + agentList + ")"
		}
	}

	return rulesStart + `
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: ` + "`ADR → REQ → ROADMAP`" + ` · States: ` + "`backlog / analyzing / wip / blocked / done / abandoned`" + `

### Agent Protocol
1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:
   ` + step1Req + ` → ` + "`trackfw roadmap move <name> wip`" + ` → ` + "`trackfw branch new <type>/<slug>`" + step1Note + `
   ❌ Never create a branch before REQ + ROADMAP are in wip/
   ❌ Never defer REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables
   ✓ ` + "`trackfw validate`" + ` enforces this via ` + "`branch_has_wip_roadmap`" + ` rule (v2.7.0+)
2. **Before starting:** run ` + "`trackfw context`" + ` · read the 10 most recent entries in ` + "`docs/agents-working-context/`" + `
3. **After finishing:** the orchestrating session (` + "`trackfw_architect`" + `) writes a new file in ` + "`docs/agents-working-context/<YYYYMMDDTHHMMSSZ>-<your-slug>-<topic>.md`" + ` with what changed. Subagents never write there; they state what started, what changed and what is pending in their final report.
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
//
// guardRoot must ALREADY be resolved (ML-8A / #402): it is the first operand of
// the containment guard, and filePath is built from it by the caller. Resolving
// it here instead would pair a resolved root with a target the caller built from
// the unresolved one — the false-refusal half of armadilha 3, traded for the
// false-approval half.
func injectOrUpdateRules(filePath, headerIfNew, guardRoot string) error {
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, filePath); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	namespacing, agents := config.ReadNamespacingConfig(guardRoot)
	block := trackfwRulesBlock(config.ReadAgentConventions(guardRoot), namespacing, agents)

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		content := headerIfNew
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + block + "\n"
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		return os.WriteFile(filePath, []byte(content), 0644)
	}

	// Has start marker: replace up to and including end marker
	end := strings.Index(content, rulesEnd)
	if end == -1 {
		// Malformed (start without end): append fresh block
		content += "\n" + block + "\n"
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		return os.WriteFile(filePath, []byte(content), 0644)
	}

	newContent := content[:start] + block + content[end+len(rulesEnd):]
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// InjectRulesForTool injects trackfw governance rules into the config file for the given
// AI tool. tool must be one of: claude, codex, gemini, copilot, windsurf, amazonq, cursor.
// cwd is the project root directory.
func InjectRulesForTool(tool, rootDir string) error {
	relPath, ok := agentFiles[tool]
	if !ok {
		return nil
	}
	header := agentHeaders[tool]
	// ML-8A / #402: resolve the root HERE, where the target is built, so both
	// operands of the guard move into the resolved namespace together.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, relPath), rootErr)
	}
	return injectOrUpdateRules(filepath.Join(guardRoot, relPath), header, guardRoot)
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
func InjectClaudeHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".claude", "settings.json"), rootErr)
	}

	path := filepath.Join(guardRoot, ".claude", "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
	hooks["PreToolUse"] = migrateAttentionHook(hooks["PreToolUse"], "AskUserQuestion", attentionSignalCmd)

	hooks["PreToolUse"] = mergeClaudeHookArray(
		hooks["PreToolUse"],
		"AskUserQuestion",
		attentionSignalCmd,
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
	// ML-2A: migrate any pre-ML-2A credential-guard entries (both bare
	// relative path and $CLAUDE_PROJECT_DIR-pinned form) to the new inline
	// guard command (ADR-2026-10-04 D2 revised).
	// Note: legacy entries only ever used "Bash" as matcher; "Read"/"Write|Edit"
	// migration is kept for completeness but is a no-op on pre-ML-2A configs.
	for _, matcher := range []string{"Bash", "Read", "Write|Edit"} {
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyClaudeCredGuardCmd, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyClaudeCredGuardCmd, guardCredentialCmdPSPOSIX)
	}
	// Git branch guard command migration (must run BEFORE the combined matcher
	// migration below so that legacy path forms are in their current inline
	// form when migrateGuardHookMatcher checks command strings).
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyClaudeGitGuardCmd, guardGitBranchCmdPSPOSIX)

	// ML-6B/ML-6C: migrate D2-revised and pre-ML-6C D11 inline forms → new forms.
	// REQ-2026-10-20: also migrate D11 revised pré-REQ-2026-10-20 (without emulate -R sh prefix) → REQ-2026-10-20 forms.
	// Must run BEFORE migrateGuardHookMatcher so that legacy commands are already
	// in current form when the all-guard renaming logic inspects them.
	for _, matcher := range []string{claudeShellMatcher, "Bash", "Read", "Write|Edit"} {
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11RevCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11RevCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyWABashCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyWABashCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	}
	migrateHookCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], claudeShellMatcher, legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)

	// ML-5A (REQ-2026-09-05): upgrade "Bash" matcher → claudeShellMatcher
	// ("Bash|PowerShell") for all guard hook blocks.
	// Claude Code 2.1.292+ on Windows uses "PowerShell" as the primary shell
	// tool name; hooks with matcher "Bash" never fire there. The combined call
	// passes all known guard commands so that a shared "Bash" block
	// (git-branch + credential in the same entry) is identified as all-guard
	// and simply renamed, rather than split in a mixed-case pass.
	migrateGuardHookMatcher(hooks["PreToolUse"], "Bash", claudeShellMatcher,
		guardGitBranchCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	migrateGuardHookMatcher(hooks["PostToolUse"], "Bash", claudeShellMatcher,
		guardCredentialCmdPSPOSIX)

	// D13 dedup (ADR-2026-10-04 adendo 2026-10-23, REQ-2026-10-23): after all legacy
	// migrations above, collapse any duplicate guard entries (produced when a file had N
	// different legacy forms for the same pair) to exactly one per (event, matcher) pair.
	// Must run AFTER migrateGuardHookMatcher so "Bash" envelopes are already renamed to
	// claudeShellMatcher. We also cover the remaining "Bash" matcher for mixed envelopes
	// that migrateGuardHookMatcher left untouched (user hook co-located with guard).
	for _, m := range []string{claudeShellMatcher, "Bash", "Read", "Write|Edit"} {
		hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], m, guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX)
		hooks["PostToolUse"] = migrateDedupGuardInNestedArray(hooks["PostToolUse"], m, guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX)
	}
	for _, m := range []string{claudeShellMatcher, "Bash"} {
		hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], m, guardGitBranchCmdPSPOSIX, guardforms.GitBranchLegacyPSPOSIX)
	}

	// D15 (ADR-2026-10-04 adendo 2026-10-24, REQ-2026-10-23 #613): project-scope
	// wiring is always emitted regardless of global guard state. No generator
	// consults global to skip or remove project-scope entries. Guard runs twice
	// per event on machines with global installed (~12 ms accepted cost, D15).
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], claudeShellMatcher, guardCredentialCmdPSPOSIX)
	// Read/Write|Edit coverage (ADR-2026-08-06 emenda 7, 2026-08-08): always present.
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], "Read", guardCredentialCmdPSPOSIX)
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], "Write|Edit", guardCredentialCmdPSPOSIX)

	// Git branch guard (ROADMAP-2026-08-14 ML-3A / ML-2A / ML-5A / D15):
	// shell pair only — git branch guard has no Read/Write coverage.
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], claudeShellMatcher, guardGitBranchCmdPSPOSIX)

	hooks["PostToolUse"] = migrateAttentionHook(hooks["PostToolUse"], "AskUserQuestion", attentionCleanupCmd)

	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], "AskUserQuestion", attentionCleanupCmd)
	// D15 PostToolUse: always emit shell-pair credential guard.
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], claudeShellMatcher, guardCredentialCmdPSPOSIX)
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], "Read", guardCredentialCmdPSPOSIX)
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], "Write|Edit", guardCredentialCmdPSPOSIX)

	// Write-scope guard (ADR-2026-10-23 D3, ML-3A): PreToolUse only, matcher
	// Write|Edit|NotebookEdit (list exact — §Z Wave 0; do NOT use Edit.* which is a regex).
	// No PostToolUse, no cmd.exe form, no Read coverage.
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], writeScopeMatcher, guardWriteScopeCmdPSPOSIX)

	root["hooks"] = hooks

	// ML-6B: use marshalJSONNoEscape so that '>' in the D11 hook line
	// (2>${null-/dev/null}) is not HTML-escaped to '>'.
	out, err := marshalJSONNoEscape(root)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// Legacy root token is retained solely for exact migration of previously emitted hooks.
const codexRoot = `"$(git rev-parse --show-toplevel)`

var (
	codexSignalCmd  = attentionSignalCmd
	codexCleanupCmd = attentionCleanupCmd
)

// All attention emitters use native commands; old anchored forms migrate below.
const (
	geminiSignalCmd  = attentionSignalCmd
	geminiCleanupCmd = attentionCleanupCmd
)

// --- Guard hook lines — ADR-2026-10-04 D2 revised (Adendo). ---
//
// Single site for all six emitted strings (ML-2A). Family is a property of
// the CLI vendor, not the OS; the config file is version-controlled and must
// serve a mixed-OS team from the same string.
//
// PS/POSIX family (Claude Code, Codex, Gemini, Cursor, Copilot command field,
// Windsurf): PowerShell wraps the invocation in `powershell -Command "…"`,
// which maps exit 2 → 1 without the "; exit $LASTEXITCODE" suffix.
//
// cmd.exe family (Kiro, Amazon Q): the "; " separator is not valid in cmd.exe
// and would be passed as literal tokens to the binary, so no suffix.
const (
	// D11 revised fail-closed forms (ML-6C / REQ-2026-09-05 / ADR-2026-10-04 D11 revised).
	//
	// PS/POSIX family (Claude Code, Codex, Gemini, Cursor, Copilot, Windsurf):
	//   - In bash/sh: "$LASTEXITCODE=2" is CommandNotFound (bash has no $LASTEXITCODE
	//     variable); stderr is redirected to /dev/null via "${null-/dev/null}" (parameter
	//     expansion: $null is unset → default /dev/null). The arithmetic
	//     "LASTEXITCODE=$((2*!!$?))" normalises any non-zero to 2 via $?, which captures
	//     the exit of `trackfw guard …` (or 127 when absent). "exit $LASTEXITCODE" exits
	//     with that value. The 4th trecho "$LASTEXITCODE=2*!!$LASTEXITCODE 2>..." is
	//     CommandNotFound in bash/sh (inert): it is silenced by the redirect and leaves
	//     $LASTEXITCODE unchanged.
	//   - In PowerShell 5.1: "$LASTEXITCODE=2" seeds the value to 2.
	//     "LASTEXITCODE=$((2*!!$?))" (no leading $) is CommandNotFound — $LASTEXITCODE
	//     stays at whatever trackfw set (or the seed 2 if absent).
	//     The 4th trecho "$LASTEXITCODE=2*!!$LASTEXITCODE" IS a valid PS assignment:
	//     it normalises any non-zero integer to 2 (2*!!N where N!=0 → 2*1 = 2; N=0 → 0).
	//     This fixes the D11 pre-ML-6C defect where trackfw exiting 1 or 3 passed through
	//     unchanged in PowerShell. Measured in ML-6C with a fake trackfw stub.
	//
	// cmd.exe family (Kiro, Amazon Q): "||" is a cmd.exe conditional separator;
	//   when trackfw is absent or exits non-zero, "exit 2" fires.
	// REQ-2026-10-20 (REQ-2026-10-20): prefixing "emulate -R sh 2>${null-/dev/null}; " fixes
	// two zsh breakpoints before the guard runs:
	//   1. EQUALS expansion: "$LASTEXITCODE" is empty on first run → word "=2" → zsh tries
	//      "which 2" → abort exit 1.
	//   2. NOMATCH: after segment 3 sets LASTEXITCODE, segment 4 expands to "N=2*!!N";
	//      the bare "*" is a glob; zero matches → zsh abort exit 1.
	//   "unsetopt EQUALS NOMATCH" was discarded as alternative because: (a) errexit=127 —
	//   in sh/bash/dash with errexit set, CommandNotFound (exit 127) aborts the hook before
	//   the guard runs (fail-open); (b) nounset=1 — "unsetopt nounset" was not in the
	//   candidate, leaving nounset active and causing unset variable references to abort;
	//   (c) cshnullglob — NOMATCH alone was insufficient without also unsetting cshnullglob.
	//   emulate -R sh restores all POSIX sh defaults at once, including disabling these options.
	//   In PS 5.1: emulate and the LASTEXITCODE= seed are CommandNotFound; two lines appear in
	//   stderr per hook execution — accepted cost (ADR-2026-10-04 adendo 2026-10-21).
	//   Measured on macOS: sh/bash/dash/zsh/zsh-f all yield allow→0, deny→2, absent→2, err1→2.
	//   (Wave 0 / ADR-2026-10-04 D11 revised REQ-2026-10-20)
	// D12: current-form aliases — literals live in internal/guardforms/guard_generations.go.
	// Any change to the canonical command string must be made there first.
	guardGitBranchCmdPSPOSIX        = guardforms.GitBranchCurrentPSPOSIX
	guardCredentialCmdPSPOSIX       = guardforms.CredentialCurrentPSPOSIX
	guardCredentialGlobalCmdPSPOSIX = guardforms.CredentialGlobalCurrentPSPOSIX
	guardWriteScopeCmdPSPOSIX       = guardforms.WriteScopeCurrentPSPOSIX

	guardGitBranchCmdCmdExe        = guardforms.GitBranchCurrentCmdExe
	guardCredentialCmdCmdExe       = guardforms.CredentialCurrentCmdExe
	guardCredentialGlobalCmdCmdExe = guardforms.CredentialGlobalCurrentCmdExe

	// writeScopeMatcher is the exact list matcher for write-scope guard hooks.
	// Must match Write, Edit and NotebookEdit — list-exact, not regex (§Z Wave 0).
	// NEVER use "Edit.*": that is a JavaScript regex catching everything starting with "Edit".
	writeScopeMatcher = "Write|Edit|NotebookEdit"

	// claudeShellMatcher is the single canonical matcher for Claude Code shell-tool
	// guard hooks (git-branch and credential). Claude Code names its primary shell tool
	// "Bash" on POSIX/macOS and "PowerShell" on Windows; using the combined regex
	// ensures the guard fires on both platforms (ML-5A / REQ-2026-09-05).
	// This is the only place in the codebase where this string is defined — all emit
	// and migration paths consume this constant.
	claudeShellMatcher = "Bash|PowerShell"
)

// --- Legacy command paths — D12 aliases. Literals in internal/guardforms. ---
//
// These are the exact strings that older trackfw versions wrote into hook
// configs. migrateHookCommand rewrites them to the new guard hook lines above
// when InjectClaudeHooks/InjectCodexHooks/InjectGeminiHooks/InjectAmazonQHooks
// runs.  They are NEVER used as the "new" command in any emit path.
const (
	// git-branch guard legacy paths
	legacyClaudeGitGuardCmd = guardforms.LegacyClaudeGitGuard
	legacyCodexGitGuardCmd  = guardforms.LegacyCodexGitGuard
	legacyGeminiGitGuardCmd = guardforms.LegacyGeminiGitGuard

	// credential guard legacy paths (all CLIs used relative or env-var paths)
	legacyClaudeCredGuardCmd  = guardforms.LegacyClaudeCredGuard
	legacyCodexCredGuardCmd   = guardforms.LegacyCodexCredGuard
	legacyGeminiCredGuardCmd  = guardforms.LegacyGeminiCredGuard
	legacyBareCredGuardCmd    = guardforms.LegacyBareCredGuard
	legacyBareGitGuardCmd     = guardforms.LegacyBareGitGuard
	legacyWindsurfGitGuardCmd = guardforms.LegacyBashGitGuard

	// Amazon Q used the same bare relative path as the generic form
	legacyAmazonQGitGuardCmd = guardforms.LegacyBareGitGuard
)

// D2-revised inline forms — D12 aliases. Literals in internal/guardforms.
const (
	legacyD2GitBranchCmdPSPOSIX        = guardforms.LegacyD2GitBranchPSPOSIX
	legacyD2CredentialCmdPSPOSIX       = guardforms.LegacyD2CredentialPSPOSIX
	legacyD2CredentialGlobalCmdPSPOSIX = guardforms.LegacyD2CredentialGlobalPSPOSIX

	legacyD2GitBranchCmdCmdExe        = guardforms.LegacyD2GitBranchCmdExe
	legacyD2CredentialCmdCmdExe       = guardforms.LegacyD2CredentialCmdExe
	legacyD2CredentialGlobalCmdCmdExe = guardforms.LegacyD2CredentialGlobalCmdExe
)

// D11 pre-ML-6C inline forms — D12 aliases. Literals in internal/guardforms.
const (
	legacyD11GitBranchCmdPSPOSIX        = guardforms.LegacyD11GitBranchPSPOSIX
	legacyD11CredentialCmdPSPOSIX       = guardforms.LegacyD11CredentialPSPOSIX
	legacyD11CredentialGlobalCmdPSPOSIX = guardforms.LegacyD11CredentialGlobalPSPOSIX
)

// D11 revised pre-REQ-2026-10-20 inline forms — D12 aliases. Literals in internal/guardforms.
const (
	legacyD11RevGitBranchCmdPSPOSIX        = guardforms.LegacyD11RevGitBranchPSPOSIX
	legacyD11RevCredentialCmdPSPOSIX       = guardforms.LegacyD11RevCredentialPSPOSIX
	legacyD11RevCredentialGlobalCmdPSPOSIX = guardforms.LegacyD11RevCredentialGlobalPSPOSIX
)

// Workaround bash wrapper forms — D12 aliases. Literals in internal/guardforms.
const (
	legacyWABashGitBranchCmdPSPOSIX        = guardforms.LegacyWABashGitBranchPSPOSIX
	legacyWABashCredentialCmdPSPOSIX       = guardforms.LegacyWABashCredentialPSPOSIX
	legacyWABashCredentialGlobalCmdPSPOSIX = guardforms.LegacyWABashCredentialGlobalPSPOSIX
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
func InjectCodexHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".codex", "hooks.json"), rootErr)
	}

	dir := filepath.Join(guardRoot, ".codex")
	path := filepath.Join(dir, "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
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

	// Migration wiring (ROADMAP-2026-08-11 ML-1A, strings updated in ML-3A):
	// rewrites any stale relative-path entry from before this fix in place, so
	// `trackfw update` doesn't just append the new $(git rev-parse ...) entry
	// alongside the still-cwd-fragile old one.
	hooks["PermissionRequest"] = migrateAttentionHook(hooks["PermissionRequest"], ".*", codexSignalCmd)
	// ML-2A: migrate pre-ML-2A credential-guard entries to the new inline form.
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "apply_patch", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "apply_patch", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	hooks["PostToolUse"] = migrateAttentionHook(hooks["PostToolUse"], ".*", codexCleanupCmd)
	migrateHookCommand(hooks["PostToolUse"], "Bash", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], "Bash", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], "apply_patch", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], "apply_patch", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	// Git branch guard (ROADMAP-2026-08-14 ML-3A / ML-2A): migrate pre-ML-2A entry.
	// B-1 (REQ-2026-10-20): all migrateHookCommand calls moved BEFORE any mergeClaudeHookArray so that
	// a legacy form is rewritten to canonical BEFORE the merge checks for it — prevents duplicate entries.
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyCodexGitGuardCmd, guardGitBranchCmdPSPOSIX)
	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 inline forms → new forms.
	// REQ-2026-10-20: also migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms.
	for _, matcher := range []string{"Bash", "apply_patch"} {
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11RevCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11RevCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyWABashCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyWABashCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	}
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)

	hooks["PermissionRequest"] = mergeClaudeHookArray(hooks["PermissionRequest"], ".*", codexSignalCmd)

	// D13 dedup (REQ-2026-10-23): collapse duplicates produced by multiple migrateHookCommand
	// calls above (one per legacy form × one per matcher). Must run after all migrations.
	for _, m := range []string{"Bash", "apply_patch"} {
		hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], m, guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX)
		hooks["PostToolUse"] = migrateDedupGuardInNestedArray(hooks["PostToolUse"], m, guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX)
	}
	hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], "Bash", guardGitBranchCmdPSPOSIX, guardforms.GitBranchLegacyPSPOSIX)

	// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring always emitted.
	// Global state not consulted (D15).
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], "Bash", guardCredentialCmdPSPOSIX)
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], "apply_patch", guardCredentialCmdPSPOSIX)

	// Git branch guard D15: Bash pair only — always emitted.
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], "Bash", guardGitBranchCmdPSPOSIX)

	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], ".*", codexCleanupCmd)
	// D15 PostToolUse: always emit.
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], "Bash", guardCredentialCmdPSPOSIX)
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], "apply_patch", guardCredentialCmdPSPOSIX)

	root["hooks"] = hooks

	// ML-6B: use marshalJSONNoEscape so '>' in D11 survives as a literal redirect.
	out, err := marshalJSONNoEscape(root)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
func InjectGeminiHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".gemini", "settings.json"), rootErr)
	}

	dir := filepath.Join(guardRoot, ".gemini")
	path := filepath.Join(dir, "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
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

	// Migration wiring (ROADMAP-2026-08-11 ML-1A): old==new is a functional no-op
	// today, but proves the call point exists and runs before the merge below.
	// The wave that changes the Gemini command strings (ML-4A) updates oldCommand
	// here instead of adding this call from scratch — without it, the merge's
	// exact-string dedup would append a duplicate alongside the stale entry.
	hooks["Notification"] = migrateAttentionHook(hooks["Notification"], "ToolPermission", geminiSignalCmd)
	// ML-2A: migrate pre-ML-2A credential-guard entries to the new inline form.
	// The literal "scripts/trackfw-credential-guard.sh" (LegacyBareCredGuard) is in
	// guardforms.CredentialLegacyPSPOSIX and is therefore already covered by the
	// migrateDedupGuardInNestedArray D13 loop below (ML-2C, REQ-2026-10-23 B1 corretivo).
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "read_file|read_many_files", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "write_file|replace", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	hooks["AfterTool"] = migrateAttentionHook(hooks["AfterTool"], "*", geminiCleanupCmd)
	migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "read_file|read_many_files", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "write_file|replace", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	// Git branch guard (ROADMAP-2026-08-14 ML-3A / ML-2A): only
	// "run_shell_command" can ever carry a raw git subcommand.
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyGeminiGitGuardCmd, guardGitBranchCmdPSPOSIX)
	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 inline forms → new forms.
	// REQ-2026-10-20: also migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms.
	// B-1 (REQ-2026-10-20): all migrateHookCommand calls moved BEFORE any mergeClaudeHookArray so that
	// a legacy form is rewritten to canonical BEFORE the merge checks for it — prevents duplicate entries.
	for _, matcher := range []string{"run_shell_command", "read_file|read_many_files", "write_file|replace"} {
		migrateHookCommand(hooks["BeforeTool"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], matcher, legacyD11RevCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], matcher, legacyD11RevCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], matcher, legacyWABashCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], matcher, legacyWABashCredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	}
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)

	hooks["Notification"] = mergeClaudeHookArray(hooks["Notification"], "ToolPermission", geminiSignalCmd)

	// D13 dedup (REQ-2026-10-23): collapse duplicates after all legacy migrations.
	for _, m := range []string{"run_shell_command", "read_file|read_many_files", "write_file|replace"} {
		hooks["BeforeTool"] = migrateDedupGuardInNestedArray(hooks["BeforeTool"], m, guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX)
		hooks["AfterTool"] = migrateDedupGuardInNestedArray(hooks["AfterTool"], m, guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX)
	}
	hooks["BeforeTool"] = migrateDedupGuardInNestedArray(hooks["BeforeTool"], "run_shell_command", guardGitBranchCmdPSPOSIX, guardforms.GitBranchLegacyPSPOSIX)

	// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring always emitted.
	hooks["BeforeTool"] = mergeClaudeHookArray(hooks["BeforeTool"], "run_shell_command", guardCredentialCmdPSPOSIX)
	hooks["BeforeTool"] = mergeClaudeHookArray(hooks["BeforeTool"], "read_file|read_many_files", guardCredentialCmdPSPOSIX)
	hooks["BeforeTool"] = mergeClaudeHookArray(hooks["BeforeTool"], "write_file|replace", guardCredentialCmdPSPOSIX)

	// Git branch guard D15: run_shell_command only — always emitted.
	hooks["BeforeTool"] = mergeClaudeHookArray(hooks["BeforeTool"], "run_shell_command", guardGitBranchCmdPSPOSIX)

	hooks["AfterTool"] = mergeClaudeHookArray(hooks["AfterTool"], "*", geminiCleanupCmd)
	// D15 AfterTool: always emit.
	hooks["AfterTool"] = mergeClaudeHookArray(hooks["AfterTool"], "run_shell_command", guardCredentialCmdPSPOSIX)
	hooks["AfterTool"] = mergeClaudeHookArray(hooks["AfterTool"], "read_file|read_many_files", guardCredentialCmdPSPOSIX)
	hooks["AfterTool"] = mergeClaudeHookArray(hooks["AfterTool"], "write_file|replace", guardCredentialCmdPSPOSIX)

	root["hooks"] = hooks

	// ML-6B: use marshalJSONNoEscape so '>' in D11 survives as a literal redirect.
	out, err := marshalJSONNoEscape(root)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
func InjectKiroHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".kiro", "hooks", "trackfw-attention.json"), rootErr)
	}

	dir := filepath.Join(guardRoot, ".kiro", "hooks")
	path := filepath.Join(dir, "trackfw-attention.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	hooks := []interface{}{
		map[string]interface{}{
			"name":        "trackfw-attention-signal",
			"description": "Signals trackfw board when agent executes a tool",
			"trigger":     "PreToolUse",
			"matcher":     "*",
			"action":      map[string]interface{}{"type": "command", "command": attentionSignalCmd},
		},
		map[string]interface{}{
			"name":        "trackfw-attention-cleanup",
			"description": "Clears trackfw board attention after tool completes",
			"trigger":     "PostToolUse",
			"matcher":     "*",
			"action":      map[string]interface{}{"type": "command", "command": attentionCleanupCmd},
		},
	}

	// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring always emitted
	// regardless of global guard state (D15). ML-2A: Kiro uses cmd.exe family.
	hooks = append(hooks,
		map[string]interface{}{
			"name":        "trackfw-credential-guard-pre",
			"description": "Blocks/warns on possible plaintext credential materialization before a shell command executes",
			"trigger":     "PreToolUse",
			"matcher":     "shell",
			"action":      map[string]interface{}{"type": "command", "command": guardCredentialCmdCmdExe},
		},
		map[string]interface{}{
			"name":        "trackfw-credential-guard-post",
			"description": "Warns on possible plaintext credential materialization after a shell command executes",
			"trigger":     "PostToolUse",
			"matcher":     "shell",
			"action":      map[string]interface{}{"type": "command", "command": guardCredentialCmdCmdExe},
		},
	)
	// Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): "read"
	// and "write" are the documented Kiro tool-category aliases
	// (fs_read/fs_write). Not covered by the global harness → always emit.
	hooks = append(hooks,
		map[string]interface{}{
			"name":        "trackfw-credential-guard-read-pre",
			"description": "Blocks/warns on possible plaintext credential materialization before a file read",
			"trigger":     "PreToolUse",
			"matcher":     "read",
			"action":      map[string]interface{}{"type": "command", "command": guardCredentialCmdCmdExe},
		},
		map[string]interface{}{
			"name":        "trackfw-credential-guard-read-post",
			"description": "Warns on possible plaintext credential materialization after a file read",
			"trigger":     "PostToolUse",
			"matcher":     "read",
			"action":      map[string]interface{}{"type": "command", "command": guardCredentialCmdCmdExe},
		},
		map[string]interface{}{
			"name":        "trackfw-credential-guard-write-pre",
			"description": "Blocks/warns on possible plaintext credential materialization before a file write",
			"trigger":     "PreToolUse",
			"matcher":     "write",
			"action":      map[string]interface{}{"type": "command", "command": guardCredentialCmdCmdExe},
		},
		map[string]interface{}{
			"name":        "trackfw-credential-guard-write-post",
			"description": "Warns on possible plaintext credential materialization after a file write",
			"trigger":     "PostToolUse",
			"matcher":     "write",
			"action":      map[string]interface{}{"type": "command", "command": guardCredentialCmdCmdExe},
		},
	)

	// Git branch guard (ML-2A): Kiro uses cmd.exe family; PreToolUse only
	// (PostToolUse is audit-only, can't block). No global harness target exists
	// for Kiro git-branch-guard, so no dedup check is needed here.
	hooks = append(hooks,
		map[string]interface{}{
			"name":        "trackfw-git-branch-guard",
			"description": "Blocks disallowed git branch operations before a shell command executes",
			"trigger":     "PreToolUse",
			"matcher":     "shell",
			"action":      map[string]interface{}{"type": "command", "command": guardGitBranchCmdCmdExe},
		},
	)

	content := map[string]interface{}{
		"version": "v1",
		"hooks":   hooks,
	}

	// ML-6B: marshalJSONNoEscape for consistency with all other hook emitters.
	out, err := marshalJSONNoEscape(content)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
func InjectCopilotHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".github", "hooks", "trackfw-attention.json"), rootErr)
	}

	dir := filepath.Join(guardRoot, ".github", "hooks")
	path := filepath.Join(dir, "trackfw-attention.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	preToolUse := []interface{}{
		map[string]interface{}{
			"type":       "command",
			"command":    attentionSignalCmd,
			"cwd":        ".",
			"timeoutSec": 10,
		},
	}
	postToolUse := []interface{}{
		map[string]interface{}{
			"type":       "command",
			"command":    attentionCleanupCmd,
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
	// ML-2A: Copilot uses `command` field (not `bash`) per ADR-2026-10-04 D4.
	// PS/POSIX family — includes `; exit $LASTEXITCODE` suffix.
	// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring always emitted.
	preToolUse = append(preToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "bash",
		"command":    guardCredentialCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})
	postToolUse = append(postToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "bash",
		"command":    guardCredentialCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})
	// "view" and "create|edit": not covered by the global harness → always emit.
	preToolUse = append(preToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "view",
		"command":    guardCredentialCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})
	preToolUse = append(preToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "create|edit",
		"command":    guardCredentialCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})
	postToolUse = append(postToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "view",
		"command":    guardCredentialCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})
	postToolUse = append(postToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "create|edit",
		"command":    guardCredentialCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})

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
	// behind) — it does not exempt this entry from unconditional wiring under
	// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope git-branch guard
	// always emitted regardless of global guard state.
	preToolUse = append(preToolUse, map[string]interface{}{
		"type":       "command",
		"matcher":    "bash",
		"command":    guardGitBranchCmdPSPOSIX,
		"cwd":        ".",
		"timeoutSec": 10,
	})

	content := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse":  preToolUse,
			"postToolUse": postToolUse,
		},
	}

	// ML-6B: marshalJSONNoEscape so '>' in D11 survives as a literal redirect.
	out, err := marshalJSONNoEscape(content)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
func InjectCursorHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".cursor", "hooks.json"), rootErr)
	}

	path := filepath.Join(guardRoot, ".cursor", "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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

	// Migrate exact legacy forms in place, preserving matcher/timeout/custom fields.
	for _, pair := range []struct{ event, command string }{{"preToolUse", attentionSignalCmd}, {"postToolUse", attentionCleanupCmd}} {
		for _, old := range legacyAttentionCommands(pair.command) {
			migrateCursorSimpleCommand(hooks[pair.event], old, pair.command, getCmd)
			if legacy, ok := root[pair.event].([]interface{}); ok {
				for _, item := range legacy {
					if getCmd(item) == old {
						obj := item.(map[string]interface{})
						obj["command"] = pair.command
						arr, _ := hooks[pair.event].([]interface{})
						hooks[pair.event] = append(arr, obj)
					}
				}
			}
			removeKnownCommandFromLegacyTopLevelArray(root, pair.event, pair.command, getCmd)
			removeKnownCommandFromLegacyTopLevelArray(root, pair.event, old, getCmd)
		}
		hooks[pair.event] = dedupAttentionSimple(hooks[pair.event], pair.command)
		hooks[pair.event] = mergeSimpleCommandArray(hooks[pair.event], pair.command, makeEntry, getCmd)
	}

	// Credential guard — shell events (D12/D13/D15, ADR-2026-10-04 adendo
	// 2026-10-24): migrate ALL known legacy forms to current in one pass and
	// deduplicate to exactly one entry. D15: always emit regardless of global state.
	credLegacyForms := guardforms.CredentialLegacyPSPOSIX
	hooks["beforeShellExecution"] = migrateDedupGuardInFlatArray(
		hooks["beforeShellExecution"], "", guardCredentialCmdPSPOSIX, credLegacyForms, getCmd)
	hooks["afterShellExecution"] = migrateDedupGuardInFlatArray(
		hooks["afterShellExecution"], "", guardCredentialCmdPSPOSIX, credLegacyForms, getCmd)

	// Credential guard — Read/Write pairs (D15: ALWAYS present).
	// The global harness is wired to shell events only and does NOT fire for
	// Read/Write tool calls. These entries cover file-access operations regardless
	// of global installation status. Uses migrateDedupGuardInFlatArray so that
	// any legacy forms already present in these arrays are rewritten and collapsed
	// to exactly one canonical entry per (event, matcher) pair — fixing issue #599.
	hooks["preToolUse"] = migrateDedupGuardInFlatArray(
		hooks["preToolUse"], "Read", guardCredentialCmdPSPOSIX, credLegacyForms, getCmd)
	hooks["preToolUse"] = migrateDedupGuardInFlatArray(
		hooks["preToolUse"], "Write", guardCredentialCmdPSPOSIX, credLegacyForms, getCmd)
	hooks["postToolUse"] = migrateDedupGuardInFlatArray(
		hooks["postToolUse"], "Read", guardCredentialCmdPSPOSIX, credLegacyForms, getCmd)
	hooks["postToolUse"] = migrateDedupGuardInFlatArray(
		hooks["postToolUse"], "Write", guardCredentialCmdPSPOSIX, credLegacyForms, getCmd)

	// Git branch guard — shell events only (no Read/Write coverage for git guard).
	// D15: always emit; migrate all legacy forms and deduplicate.
	gitLegacyForms := guardforms.GitBranchLegacyPSPOSIX
	hooks["beforeShellExecution"] = migrateDedupGuardInFlatArray(
		hooks["beforeShellExecution"], "", guardGitBranchCmdPSPOSIX, gitLegacyForms, getCmd)

	root["hooks"] = hooks

	// ML-6B: marshalJSONNoEscape so '>' in D11 survives as a literal redirect.
	out, err := marshalJSONNoEscape(root)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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

// migrateGuardHookMatcher rewrites the "matcher" field of any hook group in the
// existing PreToolUse/PostToolUse array whose current matcher is oldMatcher and
// whose inner hooks array contains at least one command from guardCmds (ML-5A,
// REQ-2026-09-05 — Claude Code on Windows uses "PowerShell" as tool name).
//
// Must be called AFTER all migrateHookCommand calls (command migration first) so
// that by the time matcher migration runs, inner command strings are in their
// current form and can be matched against guardCmds.
//
// Two cases are handled in-place:
//   - All-guard block: every inner hook command is in guardCmds → rename matcher.
//   - Mixed block: some are in guardCmds, others not → remove guard entries from
//     the existing block (leaving third-party hooks in the old matcher block);
//     the subsequent mergeClaudeHookArray call will add them under newMatcher.
//   - No-guard block: no inner hook matches guardCmds → untouched.
//
// Idempotent: a second call finds no oldMatcher block containing guard commands
// (they have already been renamed or extracted), so it is a no-op.
func migrateGuardHookMatcher(existing interface{}, oldMatcher, newMatcher string, guardCmds ...string) {
	arr, _ := existing.([]interface{})
	guardSet := make(map[string]bool, len(guardCmds))
	for _, cmd := range guardCmds {
		guardSet[cmd] = true
	}

	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != oldMatcher {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})

		// Categorize inner hooks as guard or non-guard.
		allGuard := len(innerHooks) > 0
		hasGuard := false
		for _, h := range innerHooks {
			hObj, hOk := h.(map[string]interface{})
			if !hOk {
				allGuard = false
				continue
			}
			cmd, _ := hObj["command"].(string)
			if guardSet[cmd] {
				hasGuard = true
			} else {
				allGuard = false
			}
		}
		if !hasGuard {
			continue
		}
		if allGuard {
			// Simple case: every inner hook is a guard — rename the matcher.
			obj["matcher"] = newMatcher
		} else {
			// Mixed case: leave non-guard hooks in the old matcher block;
			// guard hooks are removed and will be re-added under newMatcher
			// by the subsequent mergeClaudeHookArray call.
			var remaining []interface{}
			for _, h := range innerHooks {
				hObj, hOk := h.(map[string]interface{})
				if !hOk {
					remaining = append(remaining, h)
					continue
				}
				cmd, _ := hObj["command"].(string)
				if !guardSet[cmd] {
					remaining = append(remaining, h)
				}
			}
			obj["hooks"] = remaining
		}
	}
}

// migrateCursorSimpleCommand rewrites a single stale command string to a new
// one in a flat `[]interface{}` array where each item is a map with a "command"
// key — the format used by Cursor's beforeShellExecution/afterShellExecution/
// pre_run_command hook arrays and Windsurf's pre_run_command. Analogous to
// migrateHookCommand but for the flat-array (no matcher envelope) shape used
// by those CLIs' hook events. Must be called before the corresponding
// mergeSimpleCommandArray call to avoid a duplicate stale+new entry.
func migrateCursorSimpleCommand(existing interface{}, oldCommand, newCommand string, getCmd func(interface{}) string) {
	arr, _ := existing.([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if getCmd(item) == oldCommand {
			obj["command"] = newCommand
		}
	}
}

// windsurfGitGuardCmd is the command entry trackfw registers under
// `hooks.pre_run_command` in `.windsurf/hooks.json`. Windsurf executes
// pre_run_command via a shell, so the guard can be called as an inline
// `trackfw guard` subcommand (ADR-2026-10-04 D2 / ML-2A). PS/POSIX family.
const windsurfGitGuardCmd = guardGitBranchCmdPSPOSIX

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
func InjectWindsurfHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".windsurf", "hooks.json"), rootErr)
	}

	if err := InjectRulesForTool("windsurf", guardRoot); err != nil {
		return err
	}

	// Migration: remove the incorrect, previously-written dedicated hook file
	// from an older (buggy) trackfw run, so it doesn't linger as a dead,
	// never-consumed artifact once the correct .windsurf/hooks.json exists.
	legacyPath := filepath.Join(guardRoot, ".windsurf", "hooks", legacyWindsurfHooksFile)
	if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Best-effort cleanup of the now-possibly-empty legacy directory; ignore
	// failure (non-empty dir, e.g. holding unrelated user files, or already
	// gone) — never fatal.
	_ = os.Remove(filepath.Join(guardRoot, ".windsurf", "hooks"))

	dir := filepath.Join(guardRoot, ".windsurf")
	path := filepath.Join(dir, "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
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

	windsurfGetCmd := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		s, _ := obj["command"].(string)
		return s
	}
	windsurfMakeEntry := func(cmd string) interface{} {
		return map[string]interface{}{
			"command":     cmd,
			"show_output": true,
		}
	}
	// D13 dedup for git-branch guard (REQ-2026-10-23): replace N individual
	// migrateCursorSimpleCommand calls (one per legacy form) with a single
	// dedupFlatGuardEntries call that migrates all legacy forms and collapses to
	// exactly one entry. mergeSimpleCommandArray then handles the "not yet present"
	// case with the correct {command, show_output:true} entry shape.
	hooks["pre_run_command"] = dedupFlatGuardEntries(hooks["pre_run_command"], "", windsurfGitGuardCmd, guardforms.GitBranchLegacyPSPOSIX, windsurfGetCmd)
	hooks["pre_run_command"] = mergeSimpleCommandArray(hooks["pre_run_command"], windsurfGitGuardCmd, windsurfMakeEntry, windsurfGetCmd)

	// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring always emitted.
	// pre_run_command: credential guard in addition to git-branch guard.
	// Credential guard merged AFTER git-branch for stable byte order.
	hooks["pre_run_command"] = dedupFlatGuardEntries(hooks["pre_run_command"], "", guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX, windsurfGetCmd)
	hooks["pre_run_command"] = mergeSimpleCommandArray(hooks["pre_run_command"], guardCredentialCmdPSPOSIX, windsurfMakeEntry, windsurfGetCmd)
	// pre_write_code: credential guard only (git-branch irrelevant for write events).
	// Wave 0 ML-0B: pre_write_code carries edits[*].new_string — Vector H confirmed: RC=2.
	// NÃO instalar em pre_read_code: payload is file_path only, not content (R2).
	hooks["pre_write_code"] = dedupFlatGuardEntries(hooks["pre_write_code"], "", guardCredentialCmdPSPOSIX, guardforms.CredentialLegacyPSPOSIX, windsurfGetCmd)
	hooks["pre_write_code"] = mergeSimpleCommandArray(hooks["pre_write_code"], guardCredentialCmdPSPOSIX, windsurfMakeEntry, windsurfGetCmd)
	root["hooks"] = hooks

	// ML-6B: marshalJSONNoEscape so '>' in D11 survives as a literal redirect.
	out, err := marshalJSONNoEscape(root)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
func InjectAmazonQHooks(rootDir string) error {
	// ML-8A / #402: the guard root must live in the RESOLVED namespace and every
	// path below must be derived FROM it. filepath.Clean(cwd) only normalised
	// text, so a root of /tmp/p never contained a target under /private/tmp/p on
	// macOS. filepath.Join stays textual, so the target's own components remain
	// unresolved and RejectSymlinks still Lstats every one of them.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, ".amazonq", amazonQCliAgentsDir, amazonQDefaultAgentFile), rootErr)
	}

	path := filepath.Join(guardRoot, ".amazonq", amazonQCliAgentsDir, amazonQDefaultAgentFile)
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
		"description": "trackfw-managed default agent — wires the credential guard and git branch guard hooks/denylist. See docs/cli-parity.md.",
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

	// ML-2A: Amazon Q uses cmd.exe family — no `; exit $LASTEXITCODE` suffix.
	// Migrate pre-ML-2A entry (script path) to the new inline form.
	migrateHookCommand(hooks["preToolUse"], "execute_bash", legacyAmazonQGitGuardCmd, guardGitBranchCmdCmdExe)
	// ML-6B: migrate D2 cmd.exe form → fail-closed D11 form.
	migrateHookCommand(hooks["preToolUse"], "execute_bash", legacyD2GitBranchCmdCmdExe, guardGitBranchCmdCmdExe)
	// Credential guard: migrate all known legacy forms to the cmd.exe canonical.
	for _, matcher := range []string{"execute_bash", "fs_write"} {
		migrateHookCommand(hooks["preToolUse"], matcher, legacyBareCredGuardCmd, guardCredentialCmdCmdExe)
		migrateHookCommand(hooks["preToolUse"], matcher, legacyD2CredentialCmdCmdExe, guardCredentialCmdCmdExe)
	}
	// D13 dedup (REQ-2026-10-23): collapse duplicates after legacy migrations.
	// Both git-branch and credential include POSIX legacy forms so any entry
	// written by an old POSIX-family trackfw version that installed in
	// execute_bash or fs_write is also migrated to the canonical CmdExe form
	// (Amazon Q's confirmed shell family, ML-2A).
	credentialLegacyAll := append(append([]string{}, guardforms.CredentialLegacyCmdExe...), guardforms.CredentialLegacyPSPOSIX...)
	hooks["preToolUse"] = migrateDedupGuardInNestedArray(hooks["preToolUse"], "execute_bash", guardGitBranchCmdCmdExe, append(append([]string{}, guardforms.GitBranchLegacyCmdExe...), guardforms.GitBranchLegacyPSPOSIX...))
	hooks["preToolUse"] = migrateDedupGuardInNestedArray(hooks["preToolUse"], "execute_bash", guardCredentialCmdCmdExe, credentialLegacyAll)
	hooks["preToolUse"] = migrateDedupGuardInNestedArray(hooks["preToolUse"], "fs_write", guardCredentialCmdCmdExe, credentialLegacyAll)
	// Emit: git-branch guard then credential guard in execute_bash; credential in fs_write.
	hooks["preToolUse"] = mergeClaudeHookArray(hooks["preToolUse"], "execute_bash", guardGitBranchCmdCmdExe)
	// execute_bash: credential guard after git-branch for stable byte order.
	hooks["preToolUse"] = mergeClaudeHookArray(hooks["preToolUse"], "execute_bash", guardCredentialCmdCmdExe)
	// fs_write: credential guard only (git-branch irrelevant for write events).
	// Wave 0 ML-0B: fs_write carries file_text/new_str — Vector G confirmed: RC=2.
	// NÃO instalar em fs_read: payload is path only, not content (R2).
	// Bypass EE4 (JWT + "> /dev/null" in new_str → credIsAllEphemeral → RC=0) documented in Wave 0.
	hooks["preToolUse"] = mergeClaudeHookArray(hooks["preToolUse"], "fs_write", guardCredentialCmdCmdExe)
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
	// gitAddImplicitDenyPattern covers implicit-scope staging forms
	// (ADR-2026-09-05, D1): git add/stage with no explicit path, or with
	// only whole-tree root tokens (. ./ .. ../ :/ :(top)).
	// False-positive check: `git add -A docs/` does NOT match because `docs/`
	// is not in the alternatives, so the pattern fails after consuming `-A`.
	// `git add file.go` also does not match. `git add -p` does not match.
	const gitAddImplicitDenyPattern = `^git (add|stage)( +(-A|--all|-u|--update|--|\.|\./|\.\.|\.\./|:/|:\(top\)))* *$`
	denied, _ := execBash["deniedCommands"].([]interface{})
	foundBase := false
	foundAdd := false
	for _, d := range denied {
		s, ok := d.(string)
		if !ok {
			continue
		}
		if s == gitDenyPattern {
			foundBase = true
		}
		if s == gitAddImplicitDenyPattern {
			foundAdd = true
		}
	}
	if !foundBase {
		denied = append(denied, gitDenyPattern)
	}
	if !foundAdd {
		denied = append(denied, gitAddImplicitDenyPattern)
	}
	execBash["deniedCommands"] = denied
	toolsSettings["execute_bash"] = execBash
	root["toolsSettings"] = toolsSettings

	// ML-6B: marshalJSONNoEscape for consistency (cmd.exe D11 uses ||, no >, but
	// keeping the same helper across all emitters prevents future regressions).
	out, err := marshalJSONNoEscape(root)
	if err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
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
// internal/pathanchor.IsAnchored (ROADMAP-2026-09-03 Wave reaberta ML-R1, 2026-09-08: the
// predicate that used to live in internal/validator as pathIsAnchoredForHookConfig moved to the
// leaf package internal/pathanchor, now also consumed by internal/integrations), reimplemented
// locally here — not imported, different question: that predicate classifies whether a STRING is
// anchored; this one decides whether to TRANSLATE separators in a string already known to be a
// drive-letter form):
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
// internal/pathanchor.IsAnchored (moved from internal/validator's
// pathIsAnchoredForHookConfig, ROADMAP-2026-09-03 Wave reaberta ML-R1, 2026-09-08) — reimplemented
// here (not imported: that predicate answers "is this anchored for a destination/hook-config
// string", a different question from "should this drive-letter form be separator-canonicalized").
// "\\", "\\x" (no share segment) and "\\.\x" /
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

// Exact equality prevents rewriting a user's command containing a similar path.
const attentionSignalCmd = "trackfw attention signal"
const attentionCleanupCmd = "trackfw attention cleanup"

func legacyAttentionCommands(command string) []string {
	name := "signal"
	if command == attentionCleanupCmd {
		name = "cleanup"
	}
	path := "scripts/trackfw-attention-" + name + ".sh"
	return []string{path, "$CLAUDE_PROJECT_DIR/" + path, "$GEMINI_PROJECT_DIR/" + path, codexRoot + "/" + path + `"`}
}

func migrateAttentionHook(existing interface{}, matcher, command string) []interface{} {
	for _, old := range legacyAttentionCommands(command) {
		migrateHookCommand(existing, matcher, old, command)
	}
	arr, _ := existing.([]interface{})
	kept := make([]interface{}, 0, len(arr))
	var first map[string]interface{}
	for _, item := range arr {
		group, ok := item.(map[string]interface{})
		if !ok || group["matcher"] != matcher {
			kept = append(kept, item)
			continue
		}
		entries, _ := group["hooks"].([]interface{})
		next := make([]interface{}, 0, len(entries))
		for _, entry := range entries {
			hook, ok := entry.(map[string]interface{})
			if !ok || hook["command"] != command {
				next = append(next, entry)
				continue
			}
			if first == nil {
				first = hook
				next = append(next, entry)
			} else {
				for k, v := range hook {
					if _, exists := first[k]; !exists {
						first[k] = v
					}
				}
			}
		}
		group["hooks"] = next
		if len(next) > 0 {
			kept = append(kept, item)
		}
	}
	return kept
}

func dedupAttentionSimple(existing interface{}, command string) []interface{} {
	arr, _ := existing.([]interface{})
	kept := make([]interface{}, 0, len(arr))
	seen := map[string]map[string]interface{}{}
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["command"] != command {
			kept = append(kept, item)
			continue
		}
		key, _ := obj["matcher"].(string)
		if first := seen[key]; first != nil {
			for k, v := range obj {
				if _, exists := first[k]; !exists {
					first[k] = v
				}
			}
		} else {
			seen[key] = obj
			kept = append(kept, item)
		}
	}
	return kept
}
