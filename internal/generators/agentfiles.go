package generators

import (
	"encoding/json"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
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
	// ambiguity error in `req new` / `roadmap new` (Wave 1). Using it here ensures the
	// documentation matches the command behaviour exactly.
	step1Req := "`trackfw req new \"title\"`"
	step1Roadmap := "`trackfw roadmap new \"title\"`"
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
			step1Roadmap = "`trackfw roadmap new --agent <agent> \"title\"`"
			step1Note = "\n   ⚠️ `--agent` is required for this project (agents: " + agentList + ")"
		}
	}

	return rulesStart + `
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: ` + "`ADR → REQ → ROADMAP`" + ` · States: ` + "`backlog / analyzing / wip / blocked / done / abandoned`" + `

### Agent Protocol
1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:
   ` + step1Req + ` → ` + step1Roadmap + ` → ` + "`trackfw roadmap move <name> wip`" + ` → ` + "`git checkout -b feat/<branch>`" + step1Note + `
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

	// ML-6B/ML-6C: migrate D2-revised and pre-ML-6C D11 inline forms → D11 revised forms.
	// Must run BEFORE migrateGuardHookMatcher so that legacy commands are already
	// D11 revised when the all-guard renaming logic inspects them.
	for _, matcher := range []string{claudeShellMatcher, "Bash", "Read", "Write|Edit"} {
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	}
	migrateHookCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)

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

	// Dedup (ROADMAP-2026-08-06 Wave 3/ML-3A, extended ADR-2026-08-06 emenda
	// 7/ROADMAP-2026-08-08 Wave 2 to Read/Write|Edit): skip the project-scope
	// credential-guard entry when the global one is already installed
	// (`trackfw update harness --targets claude-credential-guard`), so the
	// guard doesn't run twice per Bash/PowerShell call. attention-signal/cleanup
	// above and below are unaffected — they are inherently project-scope.
	if !globalCredentialGuardInstalledClaude() {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			claudeShellMatcher,
			guardCredentialCmdPSPOSIX,
		)
		// Read/Write/Edit coverage (ADR-2026-08-06 emenda 7, 2026-08-08):
		// extraction via a direct file read, or materialization via write/edit,
		// never went through the hook before.
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Read",
			guardCredentialCmdPSPOSIX,
		)
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Write|Edit",
			guardCredentialCmdPSPOSIX,
		)
	}

	// Git branch guard (ROADMAP-2026-08-14 ML-3A / ML-2A / ML-5A): dedup or emit
	// under claudeShellMatcher so the guard fires on both Bash (POSIX) and
	// PowerShell (Windows).
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets claude-git-branch-guard`), so the
	// guard doesn't fire twice per shell call and print the block message twice.
	if !globalGitBranchGuardInstalledClaude() {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			claudeShellMatcher,
			guardGitBranchCmdPSPOSIX,
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
			claudeShellMatcher,
			guardCredentialCmdPSPOSIX,
		)
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"Read",
			guardCredentialCmdPSPOSIX,
		)
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"Write|Edit",
			guardCredentialCmdPSPOSIX,
		)
	}

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
	guardGitBranchCmdPSPOSIX        = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
	guardCredentialCmdPSPOSIX       = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
	guardCredentialGlobalCmdPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	guardGitBranchCmdCmdExe        = "trackfw guard git-branch || exit 2"
	guardCredentialCmdCmdExe       = "trackfw guard credential || exit 2"
	guardCredentialGlobalCmdCmdExe = "trackfw guard credential --global || exit 2"

	// claudeShellMatcher is the single canonical matcher for Claude Code shell-tool
	// guard hooks (git-branch and credential). Claude Code names its primary shell tool
	// "Bash" on POSIX/macOS and "PowerShell" on Windows; using the combined regex
	// ensures the guard fires on both platforms (ML-5A / REQ-2026-09-05).
	// This is the only place in the codebase where this string is defined — all emit
	// and migration paths consume this constant.
	claudeShellMatcher = "Bash|PowerShell"
)

// --- Legacy command paths — kept for migrateHookCommand calls only. ---
//
// These are the exact strings that older trackfw versions wrote into hook
// configs. migrateHookCommand rewrites them to the new guard hook lines above
// when InjectClaudeHooks/InjectCodexHooks/InjectGeminiHooks/InjectAmazonQHooks
// runs.  They are NEVER used as the "new" command in any emit path.
const (
	// git-branch guard legacy paths
	legacyClaudeGitGuardCmd = "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"
	legacyCodexGitGuardCmd  = codexRoot + `/scripts/trackfw-git-branch-guard.sh"`
	legacyGeminiGitGuardCmd = `$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`

	// credential guard legacy paths (all CLIs used relative or env-var paths)
	legacyClaudeCredGuardCmd = "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"
	legacyCodexCredGuardCmd  = codexRoot + `/scripts/trackfw-credential-guard.sh"`
	legacyGeminiCredGuardCmd = `$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh`
	legacyBareCredGuardCmd   = "scripts/trackfw-credential-guard.sh"
	legacyBareGitGuardCmd    = "scripts/trackfw-git-branch-guard.sh"
	legacyWindsurfGitGuardCmd = "bash scripts/trackfw-git-branch-guard.sh"

	// Amazon Q used a bare relative path (cmd.exe family)
	legacyAmazonQGitGuardCmd = "scripts/trackfw-git-branch-guard.sh"
)

// D2-revised inline forms (pre-ML-6B, ADR-2026-10-04 D2 revised).
// These are the exact strings written by trackfw before the D11 fail-closed
// upgrade. Kept for migrateHookCommand / dedup recognition. Never used as the
// "new" command in any emit path.
const (
	legacyD2GitBranchCmdPSPOSIX        = "trackfw guard git-branch; exit $LASTEXITCODE"
	legacyD2CredentialCmdPSPOSIX       = "trackfw guard credential; exit $LASTEXITCODE"
	legacyD2CredentialGlobalCmdPSPOSIX = "trackfw guard credential --global; exit $LASTEXITCODE"

	legacyD2GitBranchCmdCmdExe        = "trackfw guard git-branch"
	legacyD2CredentialCmdCmdExe       = "trackfw guard credential"
	legacyD2CredentialGlobalCmdCmdExe = "trackfw guard credential --global"
)

// D11 pre-ML-6C inline forms (the D11 form WITHOUT the 4th PowerShell normalisation trecho).
// These were written by trackfw v9.3.x before the ML-6C upgrade. Kept for
// migrateHookCommand / dedup recognition. Never used as the "new" command in any emit path.
const (
	legacyD11GitBranchCmdPSPOSIX        = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`
	legacyD11CredentialCmdPSPOSIX       = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`
	legacyD11CredentialGlobalCmdPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`
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
	migrateHookCommand(hooks["PermissionRequest"], ".*", "scripts/trackfw-attention-signal.sh", codexSignalCmd)
	// ML-2A: migrate pre-ML-2A credential-guard entries to the new inline form.
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "apply_patch", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "apply_patch", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], ".*", "scripts/trackfw-attention-cleanup.sh", codexCleanupCmd)
	migrateHookCommand(hooks["PostToolUse"], "Bash", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], "Bash", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], "apply_patch", legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["PostToolUse"], "apply_patch", legacyCodexCredGuardCmd, guardCredentialCmdPSPOSIX)

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
			guardCredentialCmdPSPOSIX,
		)
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"apply_patch",
			guardCredentialCmdPSPOSIX,
		)
	}

	// Git branch guard (ROADMAP-2026-08-14 ML-3A / ML-2A): migrate
	// pre-ML-2A entry then emit or dedup.
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyCodexGitGuardCmd, guardGitBranchCmdPSPOSIX)
	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 inline forms → D11 revised fail-closed forms.
	for _, matcher := range []string{"Bash", "apply_patch"} {
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	}
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets codex-git-branch-guard`).
	if !globalGitBranchGuardInstalledCodex() {
		hooks["PreToolUse"] = mergeClaudeHookArray(
			hooks["PreToolUse"],
			"Bash",
			guardGitBranchCmdPSPOSIX,
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
			guardCredentialCmdPSPOSIX,
		)
		hooks["PostToolUse"] = mergeClaudeHookArray(
			hooks["PostToolUse"],
			"apply_patch",
			guardCredentialCmdPSPOSIX,
		)
	}

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
	migrateHookCommand(hooks["Notification"], "ToolPermission", "scripts/trackfw-attention-signal.sh", geminiSignalCmd)
	// ML-2A: migrate pre-ML-2A credential-guard entries to the new inline form.
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "read_file|read_many_files", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "write_file|replace", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "*", "scripts/trackfw-attention-cleanup.sh", geminiCleanupCmd)
	migrateHookCommand(hooks["AfterTool"], "run_shell_command", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "read_file|read_many_files", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "read_file|read_many_files", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "write_file|replace", "scripts/trackfw-credential-guard.sh", guardCredentialCmdPSPOSIX)
	migrateHookCommand(hooks["AfterTool"], "write_file|replace", legacyGeminiCredGuardCmd, guardCredentialCmdPSPOSIX)

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
			guardCredentialCmdPSPOSIX,
		)
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"read_file|read_many_files",
			guardCredentialCmdPSPOSIX,
		)
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"write_file|replace",
			guardCredentialCmdPSPOSIX,
		)
	}

	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 inline forms → D11 revised fail-closed forms.
	for _, matcher := range []string{"run_shell_command", "read_file|read_many_files", "write_file|replace"} {
		migrateHookCommand(hooks["BeforeTool"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], matcher, legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], matcher, legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX)
	}
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)

	// Git branch guard (ROADMAP-2026-08-14 ML-3A / ML-2A): only
	// "run_shell_command" can ever carry a raw git subcommand.
	migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyGeminiGitGuardCmd, guardGitBranchCmdPSPOSIX)
	// Dedup (ROADMAP-2026-08-17 Wave 2/ML-2B): skip the project-scope
	// git-branch-guard entry when the global one is already installed
	// (`trackfw update harness --targets gemini-git-branch-guard`).
	if !globalGitBranchGuardInstalledGemini() {
		hooks["BeforeTool"] = mergeClaudeHookArray(
			hooks["BeforeTool"],
			"run_shell_command",
			guardGitBranchCmdPSPOSIX,
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
			guardCredentialCmdPSPOSIX,
		)
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"read_file|read_many_files",
			guardCredentialCmdPSPOSIX,
		)
		hooks["AfterTool"] = mergeClaudeHookArray(
			hooks["AfterTool"],
			"write_file|replace",
			guardCredentialCmdPSPOSIX,
		)
	}

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
	// ML-2A: Kiro uses cmd.exe family — no `; exit $LASTEXITCODE` suffix.
	if !globalCredentialGuardInstalledKiro() {
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
			// Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): "read"
			// and "write" are the documented Kiro tool-category aliases
			// (fs_read/fs_write), same pattern as "shell" above.
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
	}

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
	// ML-2A: Copilot uses `command` field (not `bash`) per ADR-2026-10-04 D4.
	// PS/POSIX family — includes `; exit $LASTEXITCODE` suffix.
	if !globalCredentialGuardInstalledCopilot() {
		preToolUse = append(preToolUse, map[string]interface{}{
			"type":       "command",
			"matcher":    "bash",
			"command":    guardCredentialCmdPSPOSIX,
			"cwd":        ".",
			"timeoutSec": 10,
		})
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
			"matcher":    "bash",
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
			"command":    guardGitBranchCmdPSPOSIX,
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
	// ML-2A: migrate pre-ML-2A Cursor credential-guard entries to the new inline form.
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX, getCmd)
	migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyBareCredGuardCmd, guardCredentialCmdPSPOSIX, getCmd)
	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 inline forms → D11 revised fail-closed forms.
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX, getCmd)
	migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD2CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX, getCmd)
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX, getCmd)
	migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD11CredentialCmdPSPOSIX, guardCredentialCmdPSPOSIX, getCmd)
	if !globalCredentialGuardInstalledCursor() {
		hooks["beforeShellExecution"] = mergeSimpleCommandArray(hooks["beforeShellExecution"], guardCredentialCmdPSPOSIX, makeEntry, getCmd)
		hooks["afterShellExecution"] = mergeSimpleCommandArray(hooks["afterShellExecution"], guardCredentialCmdPSPOSIX, makeEntry, getCmd)

		// Read/Write coverage (ADR-2026-08-06 emenda 7, 2026-08-08): wired via
		// the generic preToolUse/postToolUse events (distinct from
		// beforeShellExecution/afterShellExecution, which only ever fire for
		// Shell) with an explicit "matcher", so these entries never fire for
		// the same tool call the unfiltered attention-signal/cleanup entries
		// already handle above in this same array. mergeSimpleCommandArray
		// (command-only dedup) is not enough here — both the unfiltered
		// signal entry and these matcher-scoped guard entries share the same
		// array, so dedup must also check "matcher".
		hooks["preToolUse"] = mergeCursorGuardMatcherEntry(hooks["preToolUse"], "Read", guardCredentialCmdPSPOSIX)
		hooks["preToolUse"] = mergeCursorGuardMatcherEntry(hooks["preToolUse"], "Write", guardCredentialCmdPSPOSIX)
		hooks["postToolUse"] = mergeCursorGuardMatcherEntry(hooks["postToolUse"], "Read", guardCredentialCmdPSPOSIX)
		hooks["postToolUse"] = mergeCursorGuardMatcherEntry(hooks["postToolUse"], "Write", guardCredentialCmdPSPOSIX)
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
	// ML-2A: migrate pre-ML-2A Cursor git-branch-guard entry to the new inline form.
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyBareGitGuardCmd, guardGitBranchCmdPSPOSIX, getCmd)
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyWindsurfGitGuardCmd, guardGitBranchCmdPSPOSIX, getCmd)
	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 git-branch inline form → D11 revised fail-closed form.
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, getCmd)
	migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, getCmd)
	if !globalGitBranchGuardInstalledCursor() {
		hooks["beforeShellExecution"] = mergeSimpleCommandArray(hooks["beforeShellExecution"], guardGitBranchCmdPSPOSIX, makeEntry, getCmd)
	}

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
	// ML-2A: migrate pre-ML-2A Windsurf git-branch-guard entry to the new inline form.
	migrateCursorSimpleCommand(hooks["pre_run_command"], legacyWindsurfGitGuardCmd, windsurfGitGuardCmd, windsurfGetCmd)
	// ML-6B/ML-6C: migrate D2 and pre-ML-6C D11 inline form → D11 revised fail-closed form.
	migrateCursorSimpleCommand(hooks["pre_run_command"], legacyD2GitBranchCmdPSPOSIX, windsurfGitGuardCmd, windsurfGetCmd)
	migrateCursorSimpleCommand(hooks["pre_run_command"], legacyD11GitBranchCmdPSPOSIX, windsurfGitGuardCmd, windsurfGetCmd)
	hooks["pre_run_command"] = mergeSimpleCommandArray(
		hooks["pre_run_command"],
		windsurfGitGuardCmd,
		windsurfMakeEntry,
		windsurfGetCmd,
	)
	// ML-1B (REQ-2026-10-06): wire credential guard.
	// globalCredentialGuardInstalledWindsurf checks ~/.codeium/windsurf/hooks.json
	// for the global form (guardCredentialGlobalCmdPSPOSIX). If the global harness is
	// already installed, the project-scope wiring is skipped to avoid double-firing —
	// same dedup pattern as Cursor/Claude/Codex/Gemini/Copilot.
	if !globalCredentialGuardInstalledWindsurf() {
		// pre_run_command: credential guard in addition to git-branch guard (already wired above).
		// Credential guard is merged AFTER git-branch so that a fresh init and an update of a
		// git-branch-only file produce identical byte order: [git-branch, credential] in both cases.
		hooks["pre_run_command"] = mergeSimpleCommandArray(
			hooks["pre_run_command"],
			guardCredentialCmdPSPOSIX,
			windsurfMakeEntry,
			windsurfGetCmd,
		)
		// pre_write_code: credential guard only (git-branch is irrelevant for write events).
		// Wave 0 ML-0B: pre_write_code carries edits[*].new_string — the most critical event for
		// credential materialization. Layer 1 (brute scan) detects JWTs in the payload (Vector H
		// confirmed: RC=2). NÃO instalar em pre_read_code: payload contains only file_path, not
		// content — installing there would be false protection (Wave 0 residual R2).
		hooks["pre_write_code"] = mergeSimpleCommandArray(
			hooks["pre_write_code"],
			guardCredentialCmdPSPOSIX,
			windsurfMakeEntry,
			windsurfGetCmd,
		)
	}
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
	hooks["preToolUse"] = mergeClaudeHookArray(
		hooks["preToolUse"],
		"execute_bash",
		guardGitBranchCmdCmdExe,
	)
	// ML-1B (REQ-2026-10-06): wire credential guard.
	// execute_bash: credential guard in addition to git-branch guard (already wired above).
	// Credential guard is merged AFTER git-branch so that a fresh init and an update of a
	// git-branch-only file produce identical byte order: git-branch entry first, credential
	// second inside the same matcher's inner hooks array.
	hooks["preToolUse"] = mergeClaudeHookArray(
		hooks["preToolUse"],
		"execute_bash",
		guardCredentialCmdCmdExe,
	)
	// fs_write: credential guard only (git-branch guard is irrelevant for write events).
	// Wave 0 ML-0B: fs_write carries file_text/new_str content variants — Layer 1 detects
	// JWTs in the payload (Vector G confirmed: RC=2). NÃO instalar em fs_read: payload contains
	// only path, not content — false protection (Wave 0 residual R2).
	// Bypass EE4 (JWT + "> /dev/null" in new_str → credIsAllEphemeral → RC=0) is a known gap
	// documented in Wave 0 and to be addressed by ML-1C.
	hooks["preToolUse"] = mergeClaudeHookArray(
		hooks["preToolUse"],
		"fs_write",
		guardCredentialCmdCmdExe,
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

// globalCredentialGuardInstalledClaude checks ~/.claude/settings.json for
// the PreToolUse[matcher:claudeShellMatcher] entry harnessCredentialGuardTargetClaude
// writes. Also accepts the legacy "Bash"-only matcher for configs not yet migrated.
// Fail-open: any read/parse error → false.
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-5A: also accept the new claudeShellMatcher ("Bash|PowerShell") in addition
	// to the legacy "Bash" matcher so that a global config already migrated by
	// `trackfw update harness` still dedups the project-scope entry.
	// ML-6B: also accept D2 revised form so that a global config not yet migrated
	// to D11 still dedups the project-scope entry.
	// ML-6C: also accept pre-ML-6C D11 (without 4th PS trecho) so that a global
	// config not yet migrated to D11 revised still dedups the project-scope entry.
	return hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", guardCredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], claudeShellMatcher, guardCredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD2CredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD2CredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD11CredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD11CredentialGlobalCmdPSPOSIX)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", guardCredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD2CredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD11CredentialGlobalCmdPSPOSIX)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", scriptPath) ||
		hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", guardCredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", legacyD2CredentialGlobalCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", legacyD11CredentialGlobalCmdPSPOSIX)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return simpleArrayHasValue(hooks["beforeShellExecution"], "command", scriptPath, false) ||
		simpleArrayHasValue(hooks["beforeShellExecution"], "command", guardCredentialGlobalCmdPSPOSIX, false) ||
		simpleArrayHasValue(hooks["beforeShellExecution"], "command", legacyD2CredentialGlobalCmdPSPOSIX, false) ||
		simpleArrayHasValue(hooks["beforeShellExecution"], "command", legacyD11CredentialGlobalCmdPSPOSIX, false)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return simpleArrayHasValue(hooks["preToolUse"], "bash", scriptPath, true) ||
		simpleArrayHasValue(hooks["preToolUse"], "bash", guardCredentialGlobalCmdPSPOSIX, true) ||
		simpleArrayHasValue(hooks["preToolUse"], "bash", legacyD2CredentialGlobalCmdPSPOSIX, true) ||
		simpleArrayHasValue(hooks["preToolUse"], "bash", legacyD11CredentialGlobalCmdPSPOSIX, true)
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

// globalCredentialGuardInstalledWindsurf checks ~/.codeium/windsurf/hooks.json
// for the hooks.pre_run_command AND hooks.pre_write_code entries that
// harnessCredentialGuardTargetWindsurf writes (ML-1B, REQ-2026-10-06).
//
// ML-1D fix: both events are required. The old behavior (checking only
// pre_run_command) allowed a partially-initialized global file (one that only
// got pre_run_command, e.g. from a partial `trackfw update harness` run) to
// cause InjectWindsurfHooks to skip project-scope wiring for BOTH events,
// leaving pre_write_code uncovered in project and global alike.
// harnessCredentialGuardTargetWindsurf always writes both events atomically;
// a global file with only one is partially initialized and InjectWindsurfHooks
// must fill in the project file.
// Fail-open: any read/parse error → false.
func globalCredentialGuardInstalledWindsurf() bool {
	root, ok := readGlobalHookJSON(".codeium", "windsurf", "hooks.json")
	if !ok {
		return false
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	return simpleArrayHasValue(hooks["pre_run_command"], "command", guardCredentialGlobalCmdPSPOSIX, false) &&
		simpleArrayHasValue(hooks["pre_write_code"], "command", guardCredentialGlobalCmdPSPOSIX, false)
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
// counterpart: Kiro's project-scope injector (InjectKiroHooks) wires
// git-branch-guard at project scope (agentfiles.go:1027–1038), but has no
// GLOBAL harness target (no ~/.kiro/hooks/ harness was added by ML-2A) — so
// no globalGitBranchGuardInstalledKiro function exists and no dedup is needed.
// Windsurf/AmazonQ wire git-branch-guard at project scope but have no global-scope target
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
// PreToolUse[matcher:claudeShellMatcher] entry harnessGitBranchGuardTargetClaude
// writes. Also accepts the legacy "Bash"-only matcher for configs not yet migrated.
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-5A: also accept the new claudeShellMatcher ("Bash|PowerShell") in addition
	// to the legacy "Bash" matcher so that a global config already migrated by
	// `trackfw update harness` still dedups the project-scope entry.
	// ML-6B: also accept D2 revised form.
	return hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", guardGitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], claudeShellMatcher, guardGitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD2GitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], claudeShellMatcher, legacyD11GitBranchCmdPSPOSIX)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return hookArrayHasCommand(hooks["PreToolUse"], "Bash", scriptPath) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", guardGitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", scriptPath) ||
		hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", guardGitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", legacyD2GitBranchCmdPSPOSIX) ||
		hookArrayHasCommand(hooks["BeforeTool"], "run_shell_command", legacyD11GitBranchCmdPSPOSIX)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return simpleArrayHasValue(hooks["beforeShellExecution"], "command", scriptPath, false) ||
		simpleArrayHasValue(hooks["beforeShellExecution"], "command", guardGitBranchCmdPSPOSIX, false) ||
		simpleArrayHasValue(hooks["beforeShellExecution"], "command", legacyD2GitBranchCmdPSPOSIX, false) ||
		simpleArrayHasValue(hooks["beforeShellExecution"], "command", legacyD11GitBranchCmdPSPOSIX, false)
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
	// ML-2A: accept both old abs .sh path and new inline command form.
	// ML-6B: also accept D2 revised form.
	// ML-6C: also accept pre-ML-6C D11 form.
	return simpleArrayHasValue(hooks["preToolUse"], "bash", scriptPath, true) ||
		simpleArrayHasValue(hooks["preToolUse"], "bash", guardGitBranchCmdPSPOSIX, true) ||
		simpleArrayHasValue(hooks["preToolUse"], "bash", legacyD2GitBranchCmdPSPOSIX, true) ||
		simpleArrayHasValue(hooks["preToolUse"], "bash", legacyD11GitBranchCmdPSPOSIX, true)
}
