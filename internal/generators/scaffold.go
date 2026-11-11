package generators

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/kgsaran/trackfw/internal/version"
)

// projectRoot returns the resolved (EvalSymlinks) absolute path of the current
// working directory. This is the root argument to pathguard.RejectSymlinks and
// pathguard.GuardedWrite. On macOS, os.Getwd() may return /tmp/... while
// EvalSymlinks resolves it to /private/tmp/... — passing the unresolved path as
// root causes false "escapes root" errors for files that are genuinely inside the
// project. Falls back to the unresolved cwd if EvalSymlinks fails.
// getwdFn is the single seam through which projectRoot reads the working
// directory. os.Getwd failing is the ONLY error path of projectRoot
// (filepath.EvalSymlinks failing falls back to the unresolved cwd and returns
// nil), so injecting this variable is the portable, deterministic way for a test
// to exercise the "root could not be established" branch of every call site.
//
// 🔴 The seam is at os.Getwd and NOT at projectRoot: internal/pathguard's
// analyser keys provenance on the literal identifier `projectRoot` at the call
// site (approvedResolvers / taintSourceCalls). Turning projectRoot itself into a
// variable would strip resolver provenance from every guard root in this package
// and make P2 report them as unresolved. See vault/notes/
// chdir-removeall-nao-forca-getwd-falhar-no-windows-2026-09-17.md for why a
// filesystem trick (chdir + RemoveAll) is not usable here.
var getwdFn = os.Getwd

func projectRoot() (string, error) {
	cwd, err := getwdFn()
	if err != nil {
		return "", fmt.Errorf("projectRoot: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return cwd, nil // fallback: EvalSymlinks fails on missing dirs in path
	}
	return resolved, nil
}

type Config struct {
	ProjectType        string // "fullstack" | "frontend" | "backend" | "governance"
	ProjectName        string
	Frontend           string
	Backend            string
	BackendFramework   string
	PkgManager         string
	Hooks              string
	CI                 string
	BrownfieldMode     bool
	LenientUntil       time.Time // zero value = strict
	WipLimit           int       // default: 1
	WipBySquad         bool      // default: false
	RequireReqInCommit bool      // gera hook commit-msg que exige REQ: em feat/* e fix/*
	Forge              string    // forge platform: "github", "gitlab", "bitbucket", "azure", or "" (omit key)
	AgentConventions   string    // agent_conventions: free-text, multi-line, or "" (omit key)
	// RoadmapNamespacing and Agents are used by generators (claudemd, scaffold) to emit the
	// correct `req new` / `roadmap new` command for by_agent projects with 2+ agents (AC13).
	RoadmapNamespacing string   // "flat" (default) | "by_agent"
	Agents             []string // agent namespaces declared in by_agent projects

	// ci_trigger — ADR-2026-08-28 adendo 2026-10-24, REQ-2026-10-24.
	// Accepted values: "pull_request" (default, absent = pull_request) and "manual".
	// CITriggerInvalid mirrors config.UpdateConfig.CITriggerInvalid: when true, callers
	// must refuse to generate CI workflows and return an error.
	CITrigger         string // ci_trigger: "pull_request" | "manual" (default: "" = pull_request)
	CITriggerInvalid  bool   // true when config had an invalid ci_trigger value
	CITriggerRawValue string // raw value for error messages when CITriggerInvalid
}

var govDirs = []string{
	"docs/adr",
	"docs/req",
	"docs/roadmaps/backlog",
	"docs/roadmaps/analyzing",
	"docs/roadmaps/wip",
	"docs/roadmaps/blocked",
	"docs/roadmaps/done",
	"docs/roadmaps/abandoned",
	"vault/notes",
}

// Script path constants — single source of truth shared by the writers and by
// scaffoldWriteTargets. Changing a path here automatically updates both the
// writer and the pre-check list.
const (
	validateScriptPath        = "scripts/trackfw-validate.sh"
	attentionSignalScriptPath = "scripts/trackfw-attention-signal.sh"
	attentionCleanupScriptPath = "scripts/trackfw-attention-cleanup.sh"
	credentialGuardScriptPath = "scripts/trackfw-credential-guard.sh"
	gitBranchGuardScriptPath  = "scripts/trackfw-git-branch-guard.sh"
	vaultIndexPath            = "vault/notes/index.md"
	gitAttributesPath         = ".gitattributes"
	gitIgnorePath             = ".gitignore"
	trackfwYAMLPath           = "trackfw.yaml"
	claudeMDPath              = "CLAUDE.md"
	huskyPreCommitPath        = ".husky/pre-commit"
	huskyCommitMsgPath        = ".husky/commit-msg"
	lefthookYAMLPath          = "lefthook.yml"
	lefthookCommitMsgScript   = ".lefthook/commit-msg/trackfw-req-check.sh"
	pomXMLPath                = "pom.xml"
)

// scaffoldWriteTargets returns all relative paths that Scaffold will write to for
// the given Config. It mirrors Scaffold's conditional logic exactly — a path is
// included if and only if the corresponding writer would be called under this Config.
//
// This list is the single source for the upfront pre-check in Scaffold (T-12,
// AC4): all destinations are verified before any write. It is also used by the
// coverage test to assert that every file Scaffold creates is enumerated here.
func scaffoldWriteTargets(cfg Config) []string {
	// Unconditional govDirs (directories).
	targets := make([]string, 0, 30)
	targets = append(targets, govDirs...)

	// Unconditional file writes.
	targets = append(targets,
		vaultIndexPath,
		gitAttributesPath,
		gitIgnorePath,
		trackfwYAMLPath,
		validateScriptPath,
		attentionSignalScriptPath,
		attentionCleanupScriptPath,
		credentialGuardScriptPath,
		gitBranchGuardScriptPath,
		claudeMDPath,
		ClaudeCommandsDirPath,
	)

	// Conditional on CI.
	switch cfg.CI {
	case "github-actions":
		targets = append(targets, GitHubActionsWorkflowPath)
	case "gitlab-ci":
		targets = append(targets, GitLabCIWorkflowPath)
	}

	// Conditional on Hooks.
	switch cfg.Hooks {
	case "husky":
		targets = append(targets, huskyPreCommitPath)
		if cfg.RequireReqInCommit {
			targets = append(targets, huskyCommitMsgPath)
		}
	case "lefthook":
		targets = append(targets, lefthookYAMLPath)
		if cfg.RequireReqInCommit {
			targets = append(targets, lefthookCommitMsgScript)
		}
	}

	// Conditional on Backend.
	if cfg.Backend == "java" {
		targets = append(targets, pomXMLPath)
	}

	// InjectHooksDetected (.claude/settings.json) is non-fatal by design and
	// never aborts init. Its directory (.claude/) is already covered through
	// ClaudeCommandsDirPath — a symlinked .claude/ fails that check. Excluded
	// from the upfront list for that reason.

	return targets
}

// scaffoldRoot resolves a rootDir argument for the exported scaffold functions.
// When rootDir is "" or ".", it resolves to projectRoot() (EvalSymlinks-canonical).
// When rootDir is already absolute it is returned as-is after EvalSymlinks.
func scaffoldRoot(rootDir string) (string, error) {
	if rootDir == "" || rootDir == "." {
		return projectRoot()
	}
	// ML-8A / #402: Abs + EvalSymlinks-with-fallback is exactly what
	// pathguard.ResolveRoot does; this was the third hand-written copy of it.
	// Delegating keeps one implementation of "what a guard root must be".
	return pathguard.ResolveRoot(rootDir)
}

// rejectScaffoldPath guards a path inside root before any write or MkdirAll.
// It delegates to pathguard.RejectAndReport, which is the single site in the
// binary that emits the containment refusal (ML-7B).
func rejectScaffoldPath(root, absTarget string) error {
	// Thin adapter over pathguard.RejectAndReport: it must NOT re-wrap the error
	// (the message already names the refused path) and must NOT print — the single
	// emission point lives in pathguard (ML-7B).
	return pathguard.RejectAndReport(root, absTarget)
}

func Scaffold(cfg Config) error {
	root, err := projectRoot()
	if err != nil {
		return fmt.Errorf("Scaffold: %w", err)
	}

	// T-12 (AC4, REQ-2026-11-02): upfront pre-check of ALL destinations before
	// the first write. pathguard.RejectAndReport tolerates ENOENT so a fresh
	// project (no files yet) cannot produce a false positive. If any path is
	// under a symlink or reparse point, Scaffold aborts here, before writing
	// anything. The write-time guards in every individual writer remain active
	// as a belt-and-suspenders measure against TOCTOU (T-9).
	for _, rel := range scaffoldWriteTargets(cfg) {
		if err := pathguard.RejectAndReport(root, filepath.Join(root, rel)); err != nil {
			return fmt.Errorf("scaffold pre-check: %w", err)
		}
	}

	for _, dir := range govDirs {
		absDir := filepath.Join(root, dir)
		if err := rejectScaffoldPath(root, absDir); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("  ✓ %s\n", dir)
	}

	if err := generateVaultIndex(); err != nil {
		return err
	}

	if err := generateGitAttributes(); err != nil {
		return err
	}

	// ANTES de writeTrackfwConfig de propósito: writeTrackfwConfig sobrescreve
	// trackfw.yaml com `roadmap_dir: docs/roadmaps` sem ler o valor anterior, então
	// chamar depois dele faria todo projeto brownfield com roadmap_dir próprio
	// receber a linha do layout do mantenedor. Aqui ainda se lê o valor do projeto.
	if err := generateGitIgnore(); err != nil {
		return err
	}

	if err := writeTrackfwConfig(cfg); err != nil {
		return err
	}

	if err := generateValidateScript(cfg); err != nil {
		return err
	}

	if err := GenerateAttentionScripts(""); err != nil {
		return err
	}

	if err := GenerateCredentialGuardScript(""); err != nil {
		return err
	}

	if err := GenerateGitBranchGuardScript(""); err != nil {
		return err
	}

	if err := generateCIWorkflow(cfg); err != nil {
		return err
	}

	if err := generateGitHooks(cfg); err != nil {
		return err
	}

	if err := generateCommitMsgHook(cfg); err != nil {
		return err
	}

	if err := generateClaudeMD(cfg); err != nil {
		return err
	}

	if err := generateClaudeCommands(); err != nil {
		return err
	}

	if cfg.Backend == "java" {
		if err := GeneratePomXML(cfg); err != nil {
			return fmt.Errorf("gerando pom.xml: %w", err)
		}
		fmt.Println("  ✓ pom.xml")
	}

	// Agent hooks (attention signal): injected at init time so a freshly
	// scaffolded project already carries them, matching npm's
	// generators/init.js:scaffold (which calls injectHooksDetected(root) as
	// its last step). Non-fatal like the same call in trackfw update
	// (internal/generators/update.go) — a hook-injection failure must not
	// abort project scaffolding. Ported to close the cross-runtime `init`
	// parity gap surfaced while proving `trackfw update` idempotency
	// byte-identical across Go/Node.js/Python (ML-6H, docs/cli-parity.md
	// "`trackfw update` vs `trackfw update harness`").
	if cwd, err := os.Getwd(); err == nil {
		if err := InjectHooksDetected(cwd); err != nil {
			fmt.Printf("  ⚠ agent hooks: %v\n", err)
		}
	}

	return nil
}

// InstallSkills instala os slash commands no projeto atual e a skill global em ~/.claude/skills/trackfw/.
// Arquivos já existentes não são sobrescritos — idempotente.
func InstallSkills() error {
	return installSkillsInner(false)
}

// ForceInstallSkills re-instala os slash commands e a skill global, sobrescrevendo arquivos existentes.
func ForceInstallSkills() error {
	return installSkillsInner(true)
}

func installSkillsInner(force bool) error {
	if err := generateClaudeCommandsInner(force); err != nil {
		return err
	}
	return installGlobalSkillInner(force)
}

func installGlobalSkillInner(force bool) error {
	// ML-2C (REQ-2026-09-01): use agentconfig.ClaudeConfigDir so that on
	// Windows USERPROFILE (not HOME) is used, matching what Claude Code reads.
	// Honours CLAUDE_CONFIG_DIR when set (ADR-2026-10-14 A1, D6).
	claudeDir, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		return fmt.Errorf("resolving Claude config dir: %w", err)
	}

	// guardRoot is the parent of claudeDir, resolved for symlink-namespace
	// consistency (macOS /var → /private/var). filepath.Dir(claudeDir) is HOME
	// in the standard case and filepath.Dir(CLAUDE_CONFIG_DIR) when the env var
	// is set. Using the parent (not claudeDir itself) keeps the containment
	// semantics identical to today: .claude (or the last CLAUDE_CONFIG_DIR
	// segment) sits BELOW the root and gets Lstat-checked, so a symlinked
	// .claude is still rejected — the same guarantee as ML-8A.
	// This also matches ML-2B's rejectHarnessSymlink(filepath.Dir(claudeDir), …).
	guardRoot, absErr := pathguard.ResolveRoot(filepath.Dir(claudeDir))
	if absErr != nil {
		return fmt.Errorf("resolving guard root: %w", absErr)
	}
	skillPath := filepath.Join(claudeDir, "skills", "trackfw", "SKILL.md")
	skillDir := filepath.Dir(skillPath)
	// Derive the resolved skill dir from guardRoot (already EvalSymlinks-resolved)
	// so the containment check compares paths in the same namespace.
	absSkillDir := filepath.Join(guardRoot, filepath.Base(claudeDir), "skills", "trackfw")
	if err := rejectScaffoldPath(guardRoot, absSkillDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", skillDir, err)
	}

	if _, err := os.Stat(skillPath); err == nil && !force {
		fmt.Printf("  ✓ %s (already exists — not overwritten)\n", skillPath)
		return nil
	}

	// Guard the leaf file — rejectScaffoldPath above covers the directory but not the file
	// itself (RejectSymlinks walks upward from its argument and never descends below it).
	// If SKILL.md already exists as a symlink pointing outside guardRoot, os.WriteFile would
	// follow it silently. Guarding the file catches that case.
	absSkillPath := filepath.Join(guardRoot, filepath.Base(claudeDir), "skills", "trackfw", "SKILL.md")
	if err := rejectScaffoldPath(guardRoot, absSkillPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(skillPath, GlobalClaudeSkillContent(), 0644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}
	fmt.Printf("  ✓ %s\n", skillPath)
	return nil
}

// GlobalADRDir resolves the path of the global-scope ADR directory given a
// home directory. Used by `trackfw adr new/list --scope global` to write/read
// cross-project ADRs outside any single project's `trackfw.yaml`/`adr_dirs`.
// Mirrors GlobalClaudeSkillPath below — same style, same rationale for taking
// home as a parameter (testability with a fixture $HOME).
func GlobalADRDir(home string) string {
	return filepath.Join(home, ".trackfw", "adr")
}

// GlobalClaudeSkillPath resolves the path of the historical, global-scope
// Claude compatibility skill given a home directory. It is not part of the
// catalog-managed integrations manifest — a legacy artifact predating that
// mechanism — so its lifecycle (existence/content) is tracked by direct
// inspection rather than through internal/integrations.
func GlobalClaudeSkillPath(home string) string {
	return filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md")
}

// GlobalClaudeSkillContent returns the current canonical content of the
// historical global Claude compatibility skill.
func GlobalClaudeSkillContent() []byte {
	content := `---
name: trackfw
description: "trackfw — Governed Software Delivery: ADR → REQ → ROADMAP → kanban"
signature: "📦 trackfw - Governed Delivery"
---

# trackfw — Operating Mode

You are operating with **trackfw**, a software delivery governance framework.
The mandatory chain is: **ADR → REQ → ROADMAP → backlog/wip/blocked/done/abandoned**

---

## Non-negotiable rules

1. **Never start an implementation without a REQ and a ROADMAP.** If none exist, create them first with ` + "`/trackfw:req`" + ` and ` + "`/trackfw:roadmap`" + `.
2. **Use ` + "`/trackfw:implement`" + ` as the entry point for any implementation.** This skill orchestrates the full flow automatically.
3. **Only one roadmap in ` + "`wip/`" + ` at a time.** Before starting a new one, complete or move the current one to ` + "`blocked/`" + `.
4. **ML lifecycle — mandatory:**
   - When **starting** a ML: edit the roadmap changing ` + "`**Status:** ⬜ Pending`" + ` → ` + "`**Status:** 🔄 In progress`" + ` and commit the roadmap.
   - When **completing** a ML: edit the roadmap changing ` + "`**Status:** 🔄 In progress`" + ` → ` + "`**Status:** ✅ Done`" + ` and include that change in the ML commit.
   - When **analyzing** a roadmap before starting: move the file from ` + "`backlog/`" + ` to ` + "`analyzing/`" + `; only move to ` + "`wip/`" + ` when actually coding.
5. **Run ` + "`trackfw validate`" + ` before every commit.** Zero violations required.
6. **ADRs before architectural decisions.** Any relevant technical decision must have an ADR (` + "`/trackfw:adr`" + `).
7. **` + GlobalADRsDirective + `**

---

## ML completion protocol

` + "```" + `
1. Implement      → execute actions described in the ML
2. Build          → project build command
3. Tests          → project test command
4. Validate       → trackfw validate
5. Commit         → git commit -m "feat(<scope>): <description>"
6. Push           → git push origin <branch>
7. Roadmap        → mark ML as ✅ Done
` + "```" + `
`
	return []byte(content)
}

// ForceGenerateClaudeCommands re-gera todos os slash commands, sobrescrevendo arquivos existentes.
func ForceGenerateClaudeCommands() error {
	return generateClaudeCommandsInner(true)
}

func generateClaudeCommands() error {
	return generateClaudeCommandsInner(false)
}

// ClaudeCommandsDirPath is the canonical relative path of the directory that holds the
// trackfw slash commands written by init/update. Scaffold doctor (ADR-2026-08-27) uses
// this path to detect the surface and to check individual command files.
const ClaudeCommandsDirPath = ".claude/commands/trackfw"

// claudeCommandsContent returns the template content for every slash command that
// trackfw writes to ClaudeCommandsDirPath. Extracted from generateClaudeCommandsInner
// so scaffold doctor can compare disk content against the current template without
// re-writing anything (ADR-2026-08-27, AC3: no manifest writes).
func claudeCommandsContent() map[string]string {
	return map[string]string{
		"adr.md": `Run the following bash command: ` + "`trackfw adr new \"$ARGUMENTS\"`" + `

If the command fails with ` + "`trackfw: command not found`" + ` or similar, inform the user:

` + "```" + `
trackfw is not installed. Install with one of these options:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"req.md": `Run the following bash command: ` + "`trackfw req new \"$ARGUMENTS\"`" + `

⚠️ In projects with ` + "`roadmap_namespacing: by_agent`" + ` and 2+ agents, use: ` + "`trackfw req new --agent <your-agent> \"$ARGUMENTS\"`" + `

If the command fails with ` + "`trackfw: command not found`" + ` or similar, inform the user:

` + "```" + `
trackfw is not installed. Install with one of these options:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"validate.md": `Run the following bash command: ` + "`trackfw validate`" + `

If the command fails with ` + "`trackfw: command not found`" + ` or similar, inform the user:

` + "```" + `
trackfw is not installed. Install with one of these options:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"status.md": `Run the following bash command: ` + "`trackfw status`" + `

If the command fails with ` + "`trackfw: command not found`" + ` or similar, inform the user:

` + "```" + `
trackfw is not installed. Install with one of these options:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"move.md": `Run the following bash command: ` + "`trackfw roadmap move $ARGUMENTS`" + `

Expected format: ` + "`<roadmap-name> <state>`" + `

Valid states: ` + "`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`" + `

Example: ` + "`/trackfw:move my-roadmap analyzing`" + `

If the command fails with ` + "`trackfw: command not found`" + ` or similar, inform the user:
trackfw is not installed. Install with:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw`,

		"roadmap.md": `Generate an implementation roadmap in microbatches for a project REQ.

## Steps

1. **List available REQs**
   Use Glob to list ` + "`docs/req/*.md`" + `. If no file found, inform:
   > No REQ found in ` + "`docs/req/`" + `. Create one first with ` + "`/trackfw:req`" + `.

2. **Select the REQ**
   - If ` + "`$ARGUMENTS`" + ` was provided: use it as a filter (case-insensitive substring) to find the file
   - If not provided or the filter does not find exactly one: list the available files and ask the user which to use
   - Read the full contents of the selected REQ file

3. **Generate the roadmap**
   Based on the REQ content, generate a roadmap following **strictly** this format:

` + "````markdown" + `
---
status: backlog
date: <YYYY-MM-DD>
req: "docs/req/<selected-file>.md"
squad: ""
---

# Roadmap: <title derived from REQ>

> Created: <YYYY-MM-DD> | Status: backlog

## Context / Diagnosis
<summary of the problem, motivation and scope extracted from the REQ>

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pending
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Wave gates:**
` + "```bash" + `
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
` + "```" + `

## Wave 1 — <descriptive name> (<N> MLs in parallel)
> Dependencies: Independent

### ML-1A — <title>
**Status:** ⬜ Pending
**Files affected:**
- ` + "`exact/path/to/file`" + `
**Actions:**
- Detailed description of the action with exact values, keys and commands
**Acceptance criteria:**
- [ ] build without errors
- [ ] tests green
**Validation commands:** ` + "`<project build and test command>`" + `

### ML-1B — <title> (if independent of ML-1A)
...

## Wave 2 — <name> (depends on Wave 1)
> Dependencies: Wave 1 complete
...
` + "````" + `

   **Mandatory principles:**
   - MLs within the same Wave are **independent** (distinct files, no conflict)
   - Each ML must be detailed enough for execution by an agent without extra context
   - Maximize parallelism: group in parallel everything that does not share files
   - Sequential Waves only when there is a real dependency on a result
   - Measurable acceptance criteria in each ML

4. **Save the file**
   - Compute the slug: title in lowercase, spaces → hyphens, remove special characters
   - Create the file at ` + "`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`" + `
   - Fill ` + "`req:`" + ` with the full relative path of the selected REQ
   - Use today's date

5. **Confirm**
   Report the path of the created file and a summary of Waves and total MLs generated.
`,

		"barrier.md": "You are the `trackfw_architect`, the sole Git authority of this project. This command runs the operational wave-release checklist — no other agent commits, pushes, or releases the next wave.\n" +
			"\n" +
			"## Argument\n" +
			"\n" +
			"`$ARGUMENTS` in the format `<roadmap> <wave>`. If absent or incomplete, ask the user which roadmap (in `docs/roadmaps/wip/`) and which wave number to validate.\n" +
			"\n" +
			"---\n" +
			"\n" +
			"## Deterministic core\n" +
			"\n" +
			"Run first:\n" +
			"```bash\n" +
			"trackfw barrier <roadmap> --wave <n> --trust-local-gates --json\n" +
			"```\n" +
			"\n" +
			"`--trust-local-gates` is required here: WIP roadmaps (modified locally, not yet committed to\n" +
			"`origin/main`) are marked as untrusted by the CLI by default, as protection against running\n" +
			"gates from roadmaps arriving via third-party PRs. The slash command applies this flag because\n" +
			"it represents the legitimate flow of the architect operating in their own repository — not\n" +
			"because the gates are pre-inspected (the diff is still the checklist's responsibility below).\n" +
			"\n" +
			"⚠️ **Do not use `--trust-local-gates` when reviewing a roadmap arriving via a third-party PR** — use the CLI\n" +
			"directly without the flag (`trackfw barrier <roadmap> --wave <n> --json`) so that gates are marked\n" +
			"as `not_evaluated` and not executed.\n" +
			"\n" +
			"This command is **necessary but not sufficient**. It checks completed MLs, evidence and `trackfw validate`, but does not replace the specialized inspections or the diff audit below — none of those are evaluated by the binary. See the `trackfw barrier` section in `docs/cli-parity.md` for the complete contract (states, exit codes, JSON output).\n" +
			"\n" +
			"If the command returns a non-zero exit code (`blocked` or resolution error): stop, report the failure to the user and do not proceed in the checklist until the wave passes.\n" +
			"\n" +
			"---\n" +
			"\n" +
			"## Barrier definition of done — complete checklist\n" +
			"\n" +
			"Before releasing the next wave, confirm each item with concrete evidence — do not assume:\n" +
			"\n" +
			"1. **All wave MLs completed and marked** — each ML in the wave has `**Status:** ✅ Done` in the roadmap.\n" +
			"2. **Applicable unit and E2E tests executed** — run the validation commands declared in each ML.\n" +
			"3. **Applicable build without errors** — run the build command for the affected workspace(s).\n" +
			"4. **Each acceptance criterion inspected with evidence** — read the modified files and confirm against the listed criteria, not just against the tests.\n" +
			"5. **code-quality agent reported conformance, performance, robustness and clarity** — invoke the `code-quality` agent when the change introduces new logic, relevant duplication or maintainability risk.\n" +
			"6. **security agent reported SAST, privileges, access control and applicable layers** — invoke the `security` agent when the change touches authentication, secrets, external input or permissions.\n" +
			"7. **Project-declared pre-commit gates executed** — run the configured hooks/gates (lint, format, contract tests).\n" +
			"8. **`trackfw validate --json` passed** — run and confirm zero violations.\n" +
			"9. **Diff audited against scope** — review the full diff; confirm there are no changes from concurrent agents or files outside the ML scope (e.g. `docs/adr/`, `docs/req/`, `docs/roadmaps/` when not authorized to the specialist).\n" +
			"10. **Result recorded before releasing the next wave** — note in the roadmap or in the response to the user that the wave passed, with evidence for each item above.\n" +
			"\n" +
			"If any item fails: block the next wave, identify the item and the responsible agent, and dispatch a corrective microbatch. Only repeat the checklist after the corrective is completed.\n" +
			"\n" +
			"---\n" +
			"\n" +
			"## Git authority\n" +
			"\n" +
			"Only `trackfw_architect` creates branches, audits diffs, commits and pushes. Specialists deliver work without committing — it is this role's responsibility to review, commit and suggest opening a PR/MR (never opening automatically without user authorization).\n",

		"architect.md": `You are the trackfw architecture guide. Help the user choose the right stack and architect the application in plain language, accessible to non-technical teams.

## Step 1 — Business Discovery

Ask the user the following questions in plain language, one at a time:

1. "What will your application do? Describe it in 2-3 sentences as if explaining to someone outside of IT."
2. "How many people will use this system simultaneously? (< 10 people / 10-100 people / > 100 people)"
3. "Is this system going to real production or is it a prototype to validate an idea?"
4. "Do you need user login/authentication? (Yes / No / Not sure)"
5. "Are there any technology restrictions or company preferences? (e.g. Java only, Microsoft only, etc.)"

---

## Step 2 — Stack Recommendation

Based on the answers, choose **ONE** of the pre-validated combos:

### Combo A — Rapid Prototype
**When to use:** prototyping, idea validation, up to ~10 users, no production pressure.
- **Frontend:** React + Vite
- **Backend:** FastAPI (Python) or Express (Node.js)
- **Database:** SQLite + SQLAlchemy / Prisma
- **Auth:** Simple JWT when needed
- **Docker:** Basic Dockerfile for the backend

### Combo B — Small/Medium Production System
**When to use:** real system, 10-100 users, robustness and maintainability.
- **Frontend:** Next.js (SSR + ready-made routes)
- **Backend:** FastAPI (Python) or NestJS (Node.js)
- **Database:** PostgreSQL + ORM (SQLAlchemy / Prisma / TypeORM)
- **Auth:** OAuth2 with JWT (Supabase Auth or Auth0)
- **Docker:** docker-compose with frontend + backend + database

### Combo C — Enterprise / Java
**When to use:** integration with corporate systems, > 100 users, Java requirement.
- **Frontend:** Angular
- **Backend:** Spring Boot
- **Database:** PostgreSQL + Hibernate
- **Auth:** Spring Security + OAuth2 (Keycloak or Azure AD)
- **Docker:** docker-compose with all services

Present the recommended combo with a simple explanation of the reason.

---

## Step 3 — Layered Architecture (simple explanation)

Explain the architecture with a business metaphor:

"Think of the application as a restaurant:
- **Frontend** = the dining room: what the customer sees and interacts with
- **Backend** = the kitchen: where business rules happen, never exposed directly
- **Database** = the pantry: where data is stored, accessed only by the kitchen"

Reinforce the **Architecture Directives** already injected in this project's CLAUDE.md: 3-layer separation with no in-memory data (always DB + ORM), auth + Docker + .env from day 1, 2-layer validation, OpenAPI contract before coding, security wave in every roadmap, and minimum test coverage (60% prototype / 80% production).

---

## Step 4 — Generate the Stack ADR

Run ` + "`/trackfw:adr`" + ` with the title: ` + "`\"Stack and layered architecture — [project name]\"`" + `

The ADR should record the chosen stack (combo and components), motivation based on the answers, discarded alternatives and adopted architectural principles.

---

## Step 5 — Next Steps

Guide the user:

` + "```" + `
✅ Stack defined. Next steps:

1. Create the first feature REQ with /trackfw:req
2. Generate the roadmap in microbatches with /trackfw:roadmap
3. Start the implementation with /trackfw:implement
` + "```",

		"implement.md": `You are the trackfw implementation orchestrator. Follow the flow below **without skipping steps**.

## Argument

` + "`$ARGUMENTS`" + ` is optional. If provided, it is used as a filter (case-insensitive substring) over the REQ file names.

---

## Step 1 — Select the REQ

Use Glob to list ` + "`docs/req/*.md`" + `.

- If **no file found**: inform that there are no REQs available and suggest creating one with ` + "`/trackfw:req`" + `.
- If **` + "`$ARGUMENTS`" + ` was provided** and filters to exactly one REQ: use it directly.
- In **all other cases** (no argument, or ambiguous argument): present the list of available REQs and ask the user which to implement.

Read the full contents of the selected REQ.

---

## Step 2 — Find or generate the Roadmap

Check whether a roadmap linked to the REQ exists by searching in ` + "`docs/roadmaps/`" + ` (backlog, wip, blocked, done, abandoned) for a file whose name contains the REQ slug.

**If the roadmap does not exist yet:**
- Inform the user: "No roadmap found for this REQ. Generating now..."
- Execute the full generation flow from ` + "`/trackfw:roadmap`" + ` (read the file ` + "`.claude/commands/trackfw/roadmap.md`" + ` to follow the exact instructions), passing the already-selected REQ — do not ask again.
- Save the generated roadmap at ` + "`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`" + `.

**If the roadmap exists and is already in ` + "`done/`" + ` or ` + "`abandoned/`" + `:**
- Inform the user and ask whether to create a new roadmap or stop.

**If the roadmap exists in ` + "`backlog/`" + ` or ` + "`blocked/`" + `:**
- Proceed to Step 3.

**If it is already in ` + "`wip/`" + `:**
- Proceed directly to Step 4 (already in progress).

---

## Step 3 — Move roadmap to WIP

Run:
` + "```bash" + `
trackfw roadmap move <roadmap-name> wip
` + "```" + `

Confirm that the file was moved to ` + "`docs/roadmaps/wip/`" + `.

---

## Step 4 — Read and present the plan

Read the roadmap (now in ` + "`wip/`" + `). Present to the user:
- Roadmap title
- Total Waves and MLs
- Summarized list of MLs per Wave

Confirm: "Starting implementation. I will execute each ML in order and update the roadmap at each completion."

---

## Step 5 — Execute each ML in order

For each Wave (in sequence), execute the Wave's MLs:

### For each ML:

**5a. Announce:** inform which ML is being executed (e.g. "Executing ML-1A — Create client.go").

**5b. Implement:** execute the actions described in the ML using your tools (Read, Write, Edit, Bash). Follow exactly the files affected, actions and acceptance criteria listed in the roadmap.

**5c. Validate:** run the ML's validation commands. If it fails, fix before advancing.

**5d. Update the roadmap:** edit the roadmap file in ` + "`docs/roadmaps/wip/`" + ` replacing the ML status:
- ` + "`**Status:** ⬜ Pending`" + ` → ` + "`**Status:** ✅ Done`" + `

**5e. Commit:**
` + "```bash" + `
git add <ml-files>
trackfw commit -m "feat(<scope>): <ML description>"
` + "```" + `

Only advance to the next Wave after all MLs of the current Wave are ✅.

---

## Step 6 — Finalize

When all MLs are ✅:

**6a.** Run ` + "`trackfw validate`" + ` — must pass with zero violations.

**6b.** Move the roadmap to done:
` + "```bash" + `
trackfw roadmap move <roadmap-name> done
` + "```" + `

**6c.** Make the final commit:
` + "```bash" + `
git add docs/roadmaps/
git commit -m "docs(trackfw): roadmap <name> → done"
` + "```" + `

**6d.** Inform the user:
` + "```" + `
✅ Implementation complete.
Roadmap: docs/roadmaps/done/<name>.md
Next step: open PR with gh pr create
` + "```",
	}
}

func generateClaudeCommandsInner(force bool) error {
	ccRoot, ccErr := projectRoot()
	if ccErr != nil {
		return fmt.Errorf("generateClaudeCommandsInner: %w", ccErr)
	}
	absCommandsDir := filepath.Join(ccRoot, ClaudeCommandsDirPath)
	if err := rejectScaffoldPath(ccRoot, absCommandsDir); err != nil {
		return err
	}
	dir := ClaudeCommandsDirPath
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	commands := claudeCommandsContent()

	created, skipped := 0, 0
	for filename, content := range commands {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil && !force {
			skipped++
			continue
		}
		// Guard the leaf file — directory guard above does not detect a symlinked filename.
		absFilePath := filepath.Join(ccRoot, path)
		if err := rejectScaffoldPath(ccRoot, absFilePath); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
		created++
	}
	if skipped > 0 {
		fmt.Printf("  ✓ %s (%d slash commands created, %d already existed — not overwritten)\n", dir, created, skipped)
	} else {
		fmt.Printf("  ✓ %s (%d slash commands)\n", dir, created)
	}
	return nil
}

// configBlock representa um bloco de configuração extraído do template gerado.
// text inclui: separador blank líder (se houver), linhas de comentário adjacentes
// (P3: comentário = ausente), linha-chave em coluna 0 e corpo (linhas indentadas).
type configBlock struct {
	key  string
	text string
}

// presentTopLevelKeys devolve o conjunto de chaves de nível 0 presentes no
// conteúdo YAML, respeitando as quatro propriedades da ADR:
//
//   - P1: âncora em coluna 0 — chave começa sem espaço/tab.
//   - P2: dois-pontos obrigatórios — compara key+":" para evitar colisão de
//     prefixo (ex: roadmap_dir vs. roadmap_namespacing).
//   - P3: linha comentada = chave ausente — linhas que começam com "#" após
//     TrimSpace são ignoradas.
//
// A função intenciona detectar presença, não ausência — não confundir com
// parseConfigBlocks, que extrai blocos do conteúdo *gerado* (não do existente).
func presentTopLevelKeys(content string) map[string]bool {
	keys := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		// P3: ignora linhas de comentário
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		// P1: âncora em coluna 0 — ignora linhas indentadas e em branco
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		// P2: dois-pontos obrigatórios
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		keys[line[:idx]] = true
	}
	return keys
}

// parseConfigBlocks extrai os blocos de configuração do conteúdo *gerado* pelo
// template. Cada bloco cobre:
//
//  1. Um blank líder (separador visual entre blocos), quando presente.
//  2. Linhas de comentário imediatamente adjacentes (sem blank entre elas e a
//     chave) — precedente de generateGitAttributes: comentário que pertence à
//     chave deve aparecer junto quando a chave for acrescentada.
//  3. A linha-chave em coluna 0.
//  4. O corpo: linhas indentadas até a próxima chave de nível 0 (ou EOF).
//
// O cabeçalho do arquivo (linhas antes do primeiro bloco, separadas por blank)
// não faz parte de nenhum bloco e nunca é acrescentado.
func parseConfigBlocks(content string) []configBlock {
	lines := strings.Split(content, "\n")

	// Encontra os índices de todas as linhas-chave de nível 0.
	var keyIndices []int
	for i, line := range lines {
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		if strings.Contains(line, ":") {
			keyIndices = append(keyIndices, i)
		}
	}
	if len(keyIndices) == 0 {
		return nil
	}

	// Calcula blockStart para cada chave: a linha-chave mais o contexto
	// imediatamente anterior (comentários adjacentes + um blank líder).
	blockStarts := make([]int, len(keyIndices))
	for ki, keyIdx := range keyIndices {
		start := keyIdx
		// Inclui linhas de comentário imediatamente adjacentes (sem blank entre).
		for start > 0 {
			prev := lines[start-1]
			if strings.HasPrefix(strings.TrimSpace(prev), "#") {
				start--
			} else {
				break
			}
		}
		// Inclui um único blank líder (separador visual entre blocos), quando
		// imediatamente anterior às linhas de comentário ou à própria chave.
		// Precedente: generateGitAttributes/generateGitIgnore usam o mesmo guarda.
		if start > 0 && strings.TrimSpace(lines[start-1]) == "" {
			start--
		}
		blockStarts[ki] = start
	}

	blocks := make([]configBlock, 0, len(keyIndices))
	for ki, keyIdx := range keyIndices {
		// blockEnd = blockStart do próximo bloco, ou EOF.
		var blockEnd int
		if ki+1 < len(keyIndices) {
			blockEnd = blockStarts[ki+1]
		} else {
			blockEnd = len(lines)
		}

		key := lines[keyIdx][:strings.Index(lines[keyIdx], ":")]

		text := strings.Join(lines[blockStarts[ki]:blockEnd], "\n")
		// Garante newline final (equivalente ao guarda de generateGitAttributes
		// linhas 2674-2676 e generateGitIgnore linhas 2779-2781).
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}

		blocks = append(blocks, configBlock{key: key, text: text})
	}
	return blocks
}

func writeTrackfwConfig(cfg Config) error {
	wipLimit := cfg.WipLimit
	if wipLimit <= 0 {
		wipLimit = 1
	}
	wipBySquad := "false"
	if cfg.WipBySquad {
		wipBySquad = "true"
	}

	requireReqInCommit := "false"
	if cfg.RequireReqInCommit {
		requireReqInCommit = "true"
	}

	content := fmt.Sprintf(`# trackfw configuration
# generated: %s

frontend: %s
backend: %s
backend_framework: %s
pkg_manager: %s
hooks: %s
ci: %s
wip_limit: %d
wip_by_squad: %s
require_req_in_commit: %s

# validator rules (off / warning / error)
rules:
  branch_has_wip_roadmap: error

# governance paths (edit to match your project structure)
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
`, time.Now().Format("2006-01-02"), cfg.Frontend, cfg.Backend, cfg.BackendFramework, cfg.PkgManager, cfg.Hooks, cfg.CI, wipLimit, wipBySquad, requireReqInCommit)

	if cfg.Forge != "" {
		content += fmt.Sprintf("forge: %s\n", cfg.Forge)
	}

	if cfg.AgentConventions != "" {
		indented := strings.ReplaceAll(cfg.AgentConventions, "\n", "\n  ")
		content += fmt.Sprintf("agent_conventions: |\n  %s\n", indented)
	}

	if cfg.BrownfieldMode {
		content += fmt.Sprintf("governance_mode: lenient\nlenient_until: %s\n", cfg.LenientUntil.Format("2006-01-02"))
	}

	root, cfgRootErr := projectRoot()
	if cfgRootErr != nil {
		return fmt.Errorf("writeTrackfwConfig: %w", cfgRootErr)
	}
	absConfig := filepath.Join(root, "trackfw.yaml")
	if err := rejectScaffoldPath(root, absConfig); err != nil {
		return err
	}

	existing, readErr := os.ReadFile("trackfw.yaml")
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("reading trackfw.yaml: %w", readErr)
		}
		// Arquivo ausente: escreve o template completo (caminho de criação).
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile("trackfw.yaml", []byte(content), 0644); err != nil {
			return fmt.Errorf("writing trackfw.yaml: %w", err)
		}
		fmt.Println("  ✓ trackfw.yaml")
		return nil
	}

	// Arquivo presente: merge textual por chave ausente (ADR-2026-09-28).
	// Preserva todo valor já escrito pelo consumidor; acrescenta apenas as chaves
	// que faltam. Zero diff nas linhas pré-existentes quando não há chave nova.
	existingKeys := presentTopLevelKeys(string(existing))
	blocks := parseConfigBlocks(content)

	var missing []string
	for _, b := range blocks {
		if !existingKeys[b.key] {
			missing = append(missing, b.text)
		}
	}

	if len(missing) == 0 {
		// Todas as chaves já presentes: no-op (garante zero diff).
		return nil
	}

	// Acrescenta blocos ausentes.
	// P4 — guarda de newline final: precedente de generateGitAttributes (linhas
	// 2674-2676) e generateGitIgnore (linhas 2779-2781). Sem este guarda, o
	// primeiro bloco acrescentado grudaria na última linha pré-existente.
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	for _, text := range missing {
		out += text
	}

	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile("trackfw.yaml", []byte(out), 0644); err != nil {
		return fmt.Errorf("writing trackfw.yaml: %w", err)
	}
	fmt.Println("  ✓ trackfw.yaml")
	return nil
}

func generateValidateScript(cfg Config) error {
	vsRoot, vsErr := projectRoot()
	if vsErr != nil {
		return fmt.Errorf("generateValidateScript: %w", vsErr)
	}
	absScripts := filepath.Join(vsRoot, "scripts")
	if err := rejectScaffoldPath(vsRoot, absScripts); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll("scripts", 0755); err != nil {
		return err
	}

	script := buildValidateScript(cfg)
	path := filepath.Join("scripts", "trackfw-validate.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	absValidatePath := filepath.Join(vsRoot, path)
	if err := rejectScaffoldPath(vsRoot, absValidatePath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing validate script: %w", err)
	}
	// AC9 (REQ-2026-08-28): os.WriteFile applies perm only on O_CREATE; for an existing
	// file O_TRUNC rewrites content but leaves the inode mode unchanged. os.Chmod is
	// unconditional and restores 0755 even when the file already existed with 0644.
	// This matches Python's os.chmod behavior (which was already correct) and raises
	// three-runtime parity for the update path.
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("setting execute bit on validate script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

// Legacy script paths remain compatible; native hooks use attention directly.
// No shell parser, Python or jq participates in payload handling.
const attentionSignalScript = `#!/usr/bin/env bash
# trackfw attention signal — compatibility wrapper; attention never blocks tools
command -v trackfw >/dev/null 2>&1 || exit 0
trackfw attention signal "$@" || true
exit 0
`

const attentionCleanupScript = `#!/usr/bin/env bash
# trackfw attention cleanup — compatibility wrapper; attention never blocks tools
command -v trackfw >/dev/null 2>&1 || exit 0
trackfw attention cleanup "$@" || true
exit 0
`

// GenerateAttentionScripts gera os scripts shell de attention signal/cleanup em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual (mesmo
// comportamento de antes da exportação). O conteúdo gerado é idêntico ao produzido
// por `trackfw init`.
func GenerateAttentionScripts(rootDir string) error {
	root, err := scaffoldRoot(rootDir)
	if err != nil {
		return fmt.Errorf("GenerateAttentionScripts: %w", err)
	}
	rootDir = root
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := rejectScaffoldPath(root, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	signalPath := filepath.Join(scriptsDir, "trackfw-attention-signal.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, signalPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(signalPath, []byte(attentionSignalScript), 0755); err != nil {
		return fmt.Errorf("writing attention signal script: %w", err)
	}
	// AC9: restore execute bit unconditionally — see generateValidateScript for rationale.
	if err := os.Chmod(signalPath, 0755); err != nil {
		return fmt.Errorf("setting execute bit on attention signal script: %w", err)
	}
	// Mensagem sempre com caminho relativo "scripts/..." — igual ao literal fixo
	// que o Node.js imprime (npm/src/generators/hooks.js:generateAttentionScripts)
	// — independente de rootDir ser "" (cwd, usado por init/update) ou um caminho
	// absoluto (usado por discover --init via InstallGates).
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-attention-signal.sh"))

	cleanupPath := filepath.Join(scriptsDir, "trackfw-attention-cleanup.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, cleanupPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(cleanupPath, []byte(attentionCleanupScript), 0755); err != nil {
		return fmt.Errorf("writing attention cleanup script: %w", err)
	}
	// AC9: restore execute bit unconditionally.
	if err := os.Chmod(cleanupPath, 0755); err != nil {
		return fmt.Errorf("setting execute bit on attention cleanup script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-attention-cleanup.sh"))

	return nil
}

// GenerateCredentialGuardScript gera o script shell trackfw-credential-guard.sh em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual. Este ML (1A) só cria
// o script — não o injeta em nenhum hooks.json/settings.json de CLI (isso é escopo da Wave 2, ver
// ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md).
//
// O script lê o payload bruto de stdin (mesmo mecanismo usado por trackfw-attention-signal.sh, sem
// jq/python3 — grep/sed simples), procura padrão de JWT ou de AWS access key no payload inteiro
// (cobre tanto tool_input.command em PreToolUse quanto o campo de saída em PostToolUse, sem
// diferenciar o evento) e, se encontrar, decide avisar (`credential_guard.mode: warn`, default) ou
// bloquear (`credential_guard.mode: block`, exit 2) — lido de trackfw.yaml via grep simples, sem
// parser YAML completo. Uma correspondência sem nenhum redirecionamento (ex.: impressa em stdout)
// ou redirecionada para um caminho de arquivo comum sempre alerta; só é ignorada quando TODOS os
// alvos de redirecionamento do payload são efêmeros (/dev/null ou um caminho derivado de mktemp,
// incluindo uma variável atribuída via `VAR=$(mktemp...)` antes do redirecionamento).
func GenerateCredentialGuardScript(rootDir string) error {
	root, err := scaffoldRoot(rootDir)
	if err != nil {
		return fmt.Errorf("GenerateCredentialGuardScript: %w", err)
	}
	rootDir = root
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := rejectScaffoldPath(root, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(credentialGuardScript), 0755); err != nil {
		return fmt.Errorf("writing credential guard script: %w", err)
	}
	// AC9: restore execute bit unconditionally.
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("setting execute bit on credential guard script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-credential-guard.sh"))

	return nil
}

// GenerateGlobalCredentialGuardScript gera o script shell trackfw-credential-guard.sh em escopo
// global, em <home>/.trackfw/scripts/trackfw-credential-guard.sh. Destinado a ser referenciado por
// hooks globais de CLI (~/.claude/settings.json, ~/.codex/hooks.json etc.), instalados via
// `trackfw update harness` (ver ROADMAP-2026-08-06, Wave 2) — não é chamado por `trackfw
// init`/`trackfw update` (escopo de projeto), que continuam usando GenerateCredentialGuardScript.
//
// Diferente da variante de projeto, este script não tem a guarda "só roda dentro de um projeto
// trackfw.yaml" — protege qualquer projeto que o usuário abra com o CLI onde o hook global foi
// instalado. Ver globalCredentialGuardScript para a decisão de design sobre a fonte do modo
// (fallback "block" por padrão, ADR-2026-08-06 emenda 6; respeita credential_guard.mode explícito
// de trackfw.yaml quando presente) e do diretório de attention.
//
// Escreve silenciosamente (sem fmt.Printf) — seu único chamador de produção é UpdateHarness, que
// roda antes de qualquer target por-CLI ser avaliado, inclusive com `--json`; um print aqui vazaria
// texto solto para o stdout antes do JSON e quebraria o parse (mesmo motivo pelo qual
// harnessClaudeSkillTarget escreve via os.WriteFile direto em vez de reusar installGlobalSkillInner,
// que também imprime).
func GenerateGlobalCredentialGuardScript(home string) error {
	if home == "" {
		return fmt.Errorf("home directory vazio")
	}
	// ML-8A / #402: see GenerateGlobalSkill — one assignment from one resolver.
	absHome, err := pathguard.ResolveRoot(home)
	if err != nil {
		return fmt.Errorf("resolving home: %w", err)
	}
	scriptsDir := filepath.Join(absHome, ".trackfw", "scripts")
	if err := rejectScaffoldPath(absHome, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(absHome, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(globalCredentialGuardScript), 0755); err != nil {
		return fmt.Errorf("writing global credential guard script: %w", err)
	}

	return nil
}

// credentialGuardScript é o conteúdo canônico do shim de escopo de projeto (ML-2A,
// ADR-2026-10-04). A lógica de detecção migrou para `trackfw guard credential` (Go). O shim
// sonda o binário e executa o subcomando. Fail-closed (exit 2) se o binário não for encontrado
// ou não tiver o subcomando guard — o invólucro não deve silenciar falhas. A guarda de projeto
// (`[ -f trackfw.yaml ] || exit 0`) é mantida: o escopo de projeto deve ser no-op fora da
// raiz do projeto trackfw, antes da sonda.
const credentialGuardScript = `#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard credential` + "`" + `.
# Script is intentionally a no-op when executed outside the project root
set -euo pipefail
[ -f trackfw.yaml ] || exit 0
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential
`

// globalCredentialGuardScript é o conteúdo canônico do shim de escopo global
// (~/.trackfw/scripts/trackfw-credential-guard.sh, instalado via `trackfw update harness`).
// Sem a guarda de projeto (`[ -f trackfw.yaml ]`): o escopo global protege qualquer projeto,
// com ou sem trackfw.yaml — a lógica de no-op cross-project é responsabilidade do Go binary.
// Fail-closed (exit 2) se o binário não for encontrado ou não tiver o subcomando guard.
const globalCredentialGuardScript = `#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard credential --global` + "`" + `.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential --global
`

// GenerateGitBranchGuardScript gera o script shell trackfw-git-branch-guard.sh em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual. Este ML (1A) só cria o
// script canônico (referência Go) — não o injeta em nenhum hooks.json/settings.json de CLI (isso é
// escopo da Wave 3, ver ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-
// via-deny-hooks-nos-7-runtimes-suportados.md).
//
// Mesmo padrão de GenerateCredentialGuardScript: MkdirAll + WriteFile 0755.
func GenerateGitBranchGuardScript(rootDir string) error {
	root, err := scaffoldRoot(rootDir)
	if err != nil {
		return fmt.Errorf("GenerateGitBranchGuardScript: %w", err)
	}
	rootDir = root
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := rejectScaffoldPath(root, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-git-branch-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(gitBranchGuardScript), 0755); err != nil {
		return fmt.Errorf("writing git branch guard script: %w", err)
	}
	// AC9: restore execute bit unconditionally.
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("setting execute bit on git branch guard script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-git-branch-guard.sh"))

	return nil
}

// GenerateGlobalGitBranchGuardScript gera o script shell trackfw-git-branch-guard.sh em escopo
// global, em <home>/.trackfw/scripts/trackfw-git-branch-guard.sh. Destinado a ser referenciado por
// hooks globais de CLI (~/.claude/settings.json, ~/.gemini/settings.json etc.), instalados via
// `trackfw update harness` (mesmo padrão de GenerateGlobalCredentialGuardScript) — não é chamado
// por `trackfw init`/`trackfw update` (escopo de projeto), que continuam usando
// GenerateGitBranchGuardScript.
//
// O conteúdo do script é idêntico entre escopo de projeto e global — ao contrário do
// credential-guard, o git branch guard não depende de trackfw.yaml (nenhuma leitura de
// credential_guard.mode/roadmap_dir): a detecção de `git commit`/`git push`/`git checkout -b`
// bruto e a mensagem de bloqueio são as mesmas em qualquer diretório. As duas funções existem
// separadamente (em vez de uma única com destino parametrizado) só para espelhar exatamente o
// par Generate*/GenerateGlobal* já estabelecido pelo credential-guard — consistência de padrão
// entre os dois guards, não uma necessidade técnica deste guard específico.
//
// Escreve silenciosamente (sem fmt.Printf), mesmo racional de GenerateGlobalCredentialGuardScript:
// evita vazar texto solto para o stdout de comandos que possam rodar com --json.
func GenerateGlobalGitBranchGuardScript(home string) error {
	if home == "" {
		return fmt.Errorf("home directory vazio")
	}
	// ML-8A / #402: see GenerateGlobalSkill — one assignment from one resolver.
	absHome, err := pathguard.ResolveRoot(home)
	if err != nil {
		return fmt.Errorf("resolving home: %w", err)
	}
	scriptsDir := filepath.Join(absHome, ".trackfw", "scripts")
	if err := rejectScaffoldPath(absHome, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-git-branch-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(absHome, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(gitBranchGuardScript), 0755); err != nil {
		return fmt.Errorf("writing global git branch guard script: %w", err)
	}

	return nil
}

// gitBranchGuardScript é o conteúdo canônico do shim de escopo de projeto/global do git-branch
// guard (ML-2A, ADR-2026-10-04). A lógica de bloqueio (suporte a 3 formatos de entrada, decisão
// de saída, no-op fora de projeto trackfw, etc.) migrou para `trackfw guard git-branch` (Go). O
// shim sonda o binário e executa o subcomando. Fail-closed (exit 2) se o binário não for
// encontrado ou não tiver o subcomando guard.
const gitBranchGuardScript = `#!/usr/bin/env bash
# trackfw-git-branch-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard git-branch` + "`" + `.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
if [ $# -gt 0 ]; then exec trackfw guard git-branch --command "$*"; fi
exec trackfw guard git-branch
`

func buildValidateScript(cfg Config) string {
	base := `#!/usr/bin/env sh
# trackfw governance gate — generated by trackfw init
set -e

echo "→ trackfw: validating governance..."
trackfw validate

`
	switch cfg.Backend {
	case "go":
		base += "echo \"→ build check (go)...\"\ngo build ./...\n"
	case "java":
		base += "echo \"→ build check (maven)...\"\nmvn compile -q\n"
	case "node":
		base += fmt.Sprintf("echo \"→ build check (node)...\"\n%s run build\n", cfg.PkgManager)
	case "python":
		base += "echo \"→ build check (python)...\"\npython3 -c \"import pathlib, py_compile; [py_compile.compile(str(p), doraise=True) for p in pathlib.Path('.').rglob('*.py') if '.venv' not in p.parts and 'venv' not in p.parts]\"\n"
	}

	switch cfg.Frontend {
	case "react", "vue", "angular":
		pm := cfg.PkgManager
		if pm == "none" {
			pm = "npm"
		}
		base += fmt.Sprintf("echo \"→ frontend build check...\"\n%s run build\n", pm)
	}

	base += "\necho \"✓ all checks passed.\"\n"
	return base
}

func generateCIWorkflow(cfg Config) error {
	// AC4: refuse to generate when ci_trigger has an invalid value; rc≠0.
	if cfg.CITriggerInvalid {
		return fmt.Errorf("ci_trigger: %q is invalid — accepted values: pull_request, manual", cfg.CITriggerRawValue)
	}
	switch cfg.CI {
	case "github-actions":
		return generateGitHubActionsWorkflow(cfg)
	case "gitlab-ci":
		return generateGitLabCIWorkflow(cfg)
	}
	return nil
}

// GitHubActionsWorkflowPath is the canonical relative path of the GitHub Actions
// workflow written by trackfw init/update. Used by scaffold doctor (ADR-2026-08-27)
// to identify this artifact by path without reading the manifest.
const GitHubActionsWorkflowPath = ".github/workflows/trackfw-gate.yml"

// GitLabCIWorkflowPath is the canonical relative path of the GitLab CI file written
// when ci: gitlab-ci is configured. Used by scaffold doctor (ADR-2026-08-27).
const GitLabCIWorkflowPath = ".gitlab-ci-trackfw.yml"

// buildGitHubActionsWorkflowContent returns the template content that trackfw writes
// to GitHubActionsWorkflowPath. isProducer is true when the repository IS the trackfw
// project itself (detected via IsProducerGoMod — go.mod declares
// "module github.com/kgsaran/trackfw"), false for all consumer projects.
//
// Producer branch: compiles from source (go build -o ... ./cmd/trackfw) so that the
// required check `governance-install-script` validates the PR's code, not the
// previously published binary. Does not emit TRACKFW_VERSION (meaningless when
// installing from source). Uses go-version-file: go.mod to track the declared toolchain.
//
// Consumer branch: installs via install.sh (unchanged semantics) and pins
// TRACKFW_VERSION to internal/version.Version (ADR-2026-08-28, REQ-2026-08-28
// AC6/AC7). Scaffold doctor compares disk content against this template so a project
// generated by a different binary version is reported as scaffold-divergent (AC10)
// while one generated by the current binary is not (AC11).
//
// Job id is `governance-install-script` (ML-1A, ROADMAP-2026-09-01): trackfw-gate.yml
// and the sibling workflow written by discover --init (trackfw-validate.yml, see
// BuildDiscoverGitHubActionsWorkflowContent in scaffold_doctor.go) both validate the
// exact same property (`trackfw validate` passes) through the same install mechanism
// (install.sh from the pinned release tag, ADR-2026-10-30). trackfw-validate.yml keeps
// its legacy job id governance-go-install for required_status_checks compatibility
// (ADR-2026-10-30 D3) — the name no longer describes the mechanism. Both branches of
// this function emit governance-install-script because the contract with
// required_status_checks is per-job-name.
func buildGitHubActionsWorkflowContent(isProducer bool) string {
	if isProducer {
		return `name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance-install-script:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v7

      - name: ML-1A (severity anchor) — fetch origin/main for trackfw validate
        # ROADMAP-2026-09-17-leniencia-sem-prazo, ML-1A.
        # Without this step, actions/checkout@v7 on pull_request events only fetches the PR's
        # merge commit — origin/main is never written as a local ref. trackfw validate would then
        # see originAnchorRefUnreadable and fail closed (all rules at built-in defaults, disk
        # rules: block ignored). This fetch makes origin/main available so the severity anchor
        # (stricter-of-origin/main-vs-disk) can work as designed.
        #
        # Refspec is config-independent (bare branch name writes only FETCH_HEAD).
        # Cost: 0.09 s measured (1 commit + tree, < 1 MB). fetch-depth: 0 rejected (too costly).
        # Precedent: quality.yml:545-570 (same refspec, same three-branch discrimination).
        #
        # Three-branch discrimination (two-states-one-observable prevention):
        #   Arm 1 — ref absent (fetch failed): ::error:: annotation; trackfw validate will emit
        #           severity-anchor violation (fail closed via Go code).
        #   Arm 2 — ref present, file absent: ::warning:: annotation; disk-only severities.
        #   Arm 3 — ref and file present: anchor active.
        #
        # This step always exits 0 so that trackfw validate always runs (and emits the fail-closed
        # violation if needed). The CI failure surface is trackfw validate, not this fetch step.
        run: |
          FETCH_EXIT=0
          git fetch --depth=1 --no-tags origin "+refs/heads/main:refs/remotes/origin/main" 2>&1 || FETCH_EXIT=$?
          if ! git rev-parse --verify origin/main > /dev/null 2>&1; then
            echo "::error::ML-1A (anchor): origin/main unavailable after fetch (fetch_exit=${FETCH_EXIT}) — trackfw validate will emit severity-anchor violation (fail closed)"
          elif [ -z "$(git ls-tree origin/main -- ./trackfw.yaml 2>/dev/null)" ]; then
            echo "::warning::ML-1A (anchor): trackfw.yaml absent in origin/main — rule severity resolved from disk only (expected for a PR that adds the file for the first time)"
          else
            echo "ML-1A (anchor): origin/main:./trackfw.yaml present — severity anchor active"
          fi

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Build trackfw from source
        run: go build -o /usr/local/bin/trackfw ./cmd/trackfw

      - name: Governance gate
        run: trackfw validate
`
	}
	return `name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance-install-script:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    env:
      TRACKFW_VERSION: "` + version.Version + `"
    steps:
      - uses: actions/checkout@v7

      - name: Install trackfw
        run: |
          curl -sSfL "https://github.com/kgsaran/trackfw/releases/download/v${TRACKFW_VERSION}/install.sh" | sh

      - name: Governance gate
        run: trackfw validate
`
}

// buildGitHubActionsWorkflowContentWithTrigger is the parameterised builder used by
// production callers that have a Config with CITrigger set. The old
// buildGitHubActionsWorkflowContent (below) is kept as a backward-compatible wrapper
// for tests and callers that don't pass a trigger — it always returns the pull_request
// variant. The value of ciTrigger is NEVER interpolated into YAML (T2 mitigation): the
// switch selects between two pre-written fixed blocks.
//
// Accepted values: "pull_request" | "" → pull_request template (default)
//
//	"manual"            → workflow_dispatch template
//
// Callers must validate ciTrigger before calling (CITriggerInvalid=true → return error
// before any write, never call this function with an invalid value).
func buildGitHubActionsWorkflowContentWithTrigger(isProducer bool, ciTrigger string) string {
	base := buildGitHubActionsWorkflowContent(isProducer)
	if ciTrigger == "manual" {
		// Replace the pull_request trigger block with workflow_dispatch.
		// The replaced block is a fixed literal; the value ciTrigger is never in the output.
		return strings.Replace(base,
			"  pull_request:\n    branches: [main]\n",
			"  workflow_dispatch:\n",
			1)
	}
	return base
}

// buildGitLabCIWorkflowContent returns the template content that trackfw writes to
// GitLabCIWorkflowPath. Cfg-independent; ci: gitlab-ci is the gate at the call site.
// NOT version-independent: pins TRACKFW_VERSION to internal/version.Version — see the
// doc comment on buildGitHubActionsWorkflowContent above for the rationale (AC6/AC7,
// AC10, AC11, AC12).
// ci_trigger is honoured: "manual" replaces `only: - merge_requests` with
// `rules: - if: '$CI_PIPELINE_SOURCE == "web"'` (ADR-2026-08-28 adendo §Z decision 2).
func buildGitLabCIWorkflowContent(cfg Config) string {
	base := `# trackfw governance gate
trackfw-gate:
  stage: test
  image: alpine:latest
  timeout: 10 minutes
  variables:
    TRACKFW_VERSION: "` + version.Version + `"
  before_script:
    - apk add --no-cache curl
    - curl -sSfL "https://github.com/kgsaran/trackfw/releases/download/v${TRACKFW_VERSION}/install.sh" | sh
  script:
    - trackfw validate
  only:
    - merge_requests
`
	if cfg.CITrigger == "manual" {
		// Replace only/merge_requests with rules/web equivalent.
		// Neither block contains the value of cfg.CITrigger (T2 mitigation).
		return strings.Replace(base,
			"  only:\n    - merge_requests\n",
			"  rules:\n    - if: '$CI_PIPELINE_SOURCE == \"web\"'\n",
			1)
	}
	return base
}

func generateGitHubActionsWorkflow(cfg Config) error {
	ghRoot, ghErr := projectRoot()
	if ghErr != nil {
		return fmt.Errorf("generateGitHubActionsWorkflow: %w", ghErr)
	}
	absGHDir := filepath.Join(ghRoot, ".github", "workflows")
	if err := rejectScaffoldPath(ghRoot, absGHDir); err != nil {
		return err
	}

	// D2 (ADR-2026-09-29): the product delivers ONE governance workflow per project.
	// If the discover-installed workflow (DiscoverGitHubActionsWorkflowPath,
	// trackfw-validate.yml) already exists as a regular file, skip writing
	// trackfw-gate.yml. This mirrors the cuidado that
	// refreshDiscoverGitHubActionsWorkflowIfPresent (update.go) already has on
	// the other side — the asymmetry was the defect.
	// Uses discoverWorkflowPresent (os.Lstat) so a symlink at that path is NOT
	// treated as present and gate.yml is still written — safe direction.
	if discoverWorkflowPresent(ghRoot) {
		fmt.Printf("  ℹ %s already exists — %s will not be written (ADR-2026-09-29 D2)\n",
			DiscoverGitHubActionsWorkflowPath, GitHubActionsWorkflowPath)
		return nil
	}

	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(".github/workflows", 0755); err != nil {
		return err
	}

	path := GitHubActionsWorkflowPath
	// Guard the leaf file — directory guard above does not detect a symlinked workflow file.
	absWorkflowPath := filepath.Join(ghRoot, path)
	if err := rejectScaffoldPath(ghRoot, absWorkflowPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(buildGitHubActionsWorkflowContentWithTrigger(IsProducerGoMod("."), cfg.CITrigger)), 0644); err != nil {
		return fmt.Errorf("writing CI workflow: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

func generateGitLabCIWorkflow(cfg Config) error {
	glRoot, glErr := projectRoot()
	if glErr != nil {
		return fmt.Errorf("generateGitLabCIWorkflow: %w", glErr)
	}
	absGLCI := filepath.Join(glRoot, GitLabCIWorkflowPath)
	if err := rejectScaffoldPath(glRoot, absGLCI); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(GitLabCIWorkflowPath, []byte(buildGitLabCIWorkflowContent(cfg)), 0644); err != nil {
		return fmt.Errorf("writing GitLab CI: %w", err)
	}
	fmt.Println("  ✓ .gitlab-ci-trackfw.yml")
	return nil
}

func generateCommitMsgHook(cfg Config) error {
	if !cfg.RequireReqInCommit {
		return nil
	}

	cmhRoot, cmhErr := projectRoot()
	if cmhErr != nil {
		return fmt.Errorf("generateCommitMsgHook: %w", cmhErr)
	}
	// Guard the hook directory for the configured hook manager.
	switch cfg.Hooks {
	case "husky":
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, ".husky")); err != nil {
			return err
		}
	case "lefthook":
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, ".lefthook", "commit-msg")); err != nil {
			return err
		}
	}

	script := "#!/bin/sh\n" +
		"# trackfw: require REQ reference in feat/* and fix/* branches\n" +
		"BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo \"\")\n" +
		"case \"$BRANCH\" in\n" +
		"  feat/*|fix/*)\n" +
		"    if ! grep -qE \"^(REQ|req): \" \"$1\"; then\n" +
		"      echo \"ERROR: Commits in feat/* and fix/* branches require a REQ reference.\"\n" +
		"      echo \"  Add to commit body: REQ: REQ-YYYY-MM-DD-your-req-slug\"\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"

	switch cfg.Hooks {
	case "husky":
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.MkdirAll(".husky", 0755); err != nil {
			return fmt.Errorf("creating .husky: %w", err)
		}
		path := ".husky/commit-msg"
		// R-4 (REQ-2026-08-30): replace WriteFile+Chmod with pathguard.GuardedWrite.
		// GuardedWrite calls RejectAndReport internally — the redundant leaf guard
		// (rejectScaffoldPath on ".husky/commit-msg") is removed. Mode is set via
		// fd.Chmod (not path-based), eliminating the TOCTOU window and hardlink sharing.
		if err := pathguard.GuardedWrite(cmhRoot, filepath.Join(cmhRoot, path), []byte(script), 0755); err != nil {
			return fmt.Errorf("writing husky commit-msg hook: %w", err)
		}
		fmt.Printf("  ✓ %s\n", path)
	case "lefthook":
		lefthookPath := "lefthook.yml"
		existing, _ := os.ReadFile(lefthookPath)
		if !strings.Contains(string(existing), "commit-msg:") {
			addition := "\ncommit-msg:\n  scripts:\n    \"trackfw-req-check.sh\":\n      runner: sh\n"
			// Guard lefthook.yml — distinct from the .lefthook/commit-msg directory guard above;
			// that guard never walks lefthook.yml (it is at root level, outside the guarded dir).
			if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, lefthookPath)); err != nil {
				return err
			}
			// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
			if err := os.WriteFile(lefthookPath, append(existing, []byte(addition)...), 0644); err != nil {
				return fmt.Errorf("writing lefthook.yml commit-msg section: %w", err)
			}
		}
		scriptDir := ".lefthook/commit-msg"
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.MkdirAll(scriptDir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", scriptDir, err)
		}
		scriptPath := scriptDir + "/trackfw-req-check.sh"
		// Guard the leaf script file — directory guard does not detect a symlinked script file.
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, scriptPath)); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			return fmt.Errorf("writing lefthook commit-msg script: %w", err)
		}
		fmt.Printf("  ✓ %s\n", scriptPath)
	}
	return nil
}

func generateGitHooks(cfg Config) error {
	switch cfg.Hooks {
	case "husky":
		return generateHuskyHook()
	case "lefthook":
		return generateLefthookHook()
	}
	return nil
}

func generateHuskyHook() error {
	hhRoot, hhErr := projectRoot()
	if hhErr != nil {
		return fmt.Errorf("generateHuskyHook: %w", hhErr)
	}
	absHusky := filepath.Join(hhRoot, ".husky")
	if err := rejectScaffoldPath(hhRoot, absHusky); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(".husky", 0755); err != nil {
		return err
	}
	content := "#!/usr/bin/env sh\n. \"$(dirname -- \"$0\")/_/husky.sh\"\n\ntrackfw validate\n"
	path := ".husky/pre-commit"
	// R-4 (REQ-2026-08-30): replace WriteFile+Chmod with pathguard.GuardedWrite.
	// GuardedWrite calls RejectAndReport internally — the redundant leaf guard
	// (rejectScaffoldPath on ".husky/pre-commit") is removed. Mode is set via
	// fd.Chmod (not path-based), eliminating the TOCTOU window and hardlink sharing.
	if err := pathguard.GuardedWrite(hhRoot, filepath.Join(hhRoot, path), []byte(content), 0755); err != nil {
		return fmt.Errorf("writing husky hook: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

// generateVaultIndex cria vault/notes/index.md se ainda não existir.
// O arquivo é o ponto de entrada do vault de conhecimento do projeto.
func generateVaultIndex() error {
	viRoot, viErr := projectRoot()
	if viErr != nil {
		return fmt.Errorf("generateVaultIndex: %w", viErr)
	}
	absVaultDir := filepath.Join(viRoot, "vault", "notes")
	if err := rejectScaffoldPath(viRoot, absVaultDir); err != nil {
		return err
	}
	indexPath := filepath.Join("vault", "notes", "index.md")
	if _, err := os.Stat(indexPath); err == nil {
		// já existe — idempotente
		return nil
	}
	content := `# Knowledge Vault

> Project knowledge entry point for agents and people.
> Each note documents a root cause, technical decision or non-obvious constraint.
> Create notes with: trackfw note new "<title>"

## Index

<!-- Notes will be listed below. Example:
- [note-name-YYYY-MM-DD](note-name-YYYY-MM-DD.md)
-->
`
	// Guard the leaf file — directory guard above does not detect a symlinked index file.
	absIndexPath := filepath.Join(viRoot, indexPath)
	if err := rejectScaffoldPath(viRoot, absIndexPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing vault/notes/index.md: %w", err)
	}
	fmt.Println("  ✓ vault/notes/index.md")
	return nil
}

// gitAttributesRuleTarget é o primeiro campo da regra que este gerador mantém.
// A regra casa o BASENAME (padrão sem barra) de propósito: `roadmap_dir` e
// `req_dir` são configuráveis por projeto (trackfw.yaml) e ambos carregam um
// `.trackfw-log` (roadmap.go logPath / req.go appendREQTransitionLog), então um
// caminho fixo nasceria quebrado em quem configurou diretório diferente e
// deixaria o log do req_dir descoberto.
const gitAttributesRuleTarget = ".trackfw-log"

// gitAttributesBlock é o bloco emitido no caminho de criação E no de append.
// Byte-idêntico ao .gitattributes da raiz deste repositório (verificado por
// TestGitAttributesBlock_IgualAoArquivoVersionadoNaRaiz — NÃO alterar o conteúdo
// sem atualizar também o arquivo versionado).
//
// # Por que merge=union continua presente após ML-1A
//
// A partir de ML-1A, novas transições vão para .trackfw-log-entries/<ts>-<h8>.entry;
// dois branches paralelos NUNCA conflitam nesse diretório — arquivos distintos,
// merge automático. O .trackfw-log legado continua sendo LIDO pelo leitor
// unificado (translog.ReadAll) mas nunca mais escrito por este binário.
//
// O atributo merge=union permanece no bloco gerado por dois motivos:
//  1. Binários antigos (< versão que introduziu ML-1A) ainda podem escrever no
//     arquivo legado — a regra protege esses projetos durante a transição.
//  2. Remover o atributo quebraria TestGitAttributesBlock_IgualAoArquivoVersionadoNaRaiz
//     e o .gitattributes versionado na raiz sem coordenação entre os dois.
//
// Nota: forjas como GitHub ignoram merge=union definido em .gitattributes de
// repositório (medido em PR #595/#597); o atributo só é respeitado em merges
// locais via `git merge`. Isso é documentado em
// ADR-2026-10-22-logs-append-only-compartilhados-viram-um-arquivo-por-entrada-*.
const gitAttributesBlock = `# trackfw: .trackfw-log is append-only — every write lands on the last line, so
# two parallel branches conflict on every merge. merge=union keeps the lines
# from both sides (chronological order is not guaranteed). The pattern has no
# slash, so it matches the file in any directory — roadmap_dir and req_dir both
# carry one, and both are configurable per project.
.trackfw-log merge=union
`

// hasGitAttributesRule decide idempotência: verdadeiro quando ALGUMA linha
// não-comentário tem `.trackfw-log` como primeiro campo delimitado por espaço.
// Predicado deliberadamente sobre o CAMPO, não sobre a string literal da linha
// inteira: `.trackfw-log  merge=union` (dois espaços) ou uma regra manual com
// outro atributo já são "a regra existe" — reescrever por cima seria sobrescrever
// decisão do projeto.
func hasGitAttributesRule(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Fields(trimmed)[0] == gitAttributesRuleTarget {
			return true
		}
	}
	return false
}

// generateGitAttributes garante a regra `merge=union` para o `.trackfw-log` no
// `.gitattributes` da raiz do projeto. Três ramos, todos idempotentes:
//   - arquivo ausente  → cria com o bloco
//   - existe sem regra → APPEND do bloco (nunca sobrescreve o arquivo do projeto)
//   - existe com regra → no-op
func generateGitAttributes() error {
	gaRoot, gaErr := projectRoot()
	if gaErr != nil {
		return fmt.Errorf("generateGitAttributes: %w", gaErr)
	}
	absGitAttr := filepath.Join(gaRoot, ".gitattributes")
	if err := rejectScaffoldPath(gaRoot, absGitAttr); err != nil {
		return err
	}
	const path = ".gitattributes"
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading .gitattributes: %w", err)
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile(path, []byte(gitAttributesBlock), 0644); err != nil {
			return fmt.Errorf("writing .gitattributes: %w", err)
		}
		fmt.Println("  ✓ .gitattributes")
		return nil
	}

	if hasGitAttributesRule(string(existing)) {
		return nil
	}

	// Arquivo preexistente sem newline final: emendar o bloco direto grudaria a
	// primeira linha do bloco na última linha do projeto, corrompendo a config
	// dele em silêncio.
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += gitAttributesBlock
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		return fmt.Errorf("appending to .gitattributes: %w", err)
	}
	fmt.Println("  ✓ .gitattributes")
	return nil
}

// gitIgnoreRuleTarget é o BASENAME do arquivo que este gerador garante ignorado.
// O predicado de idempotência casa o basename (e não o caminho literal emitido)
// porque `roadmap_dir` é configurável: um projeto que reconfigure o diretório e
// rode `init` de novo receberia uma segunda linha para o MESMO arquivo se o
// reconhecimento fosse por string exata. Mesma lição do hasGitAttributesRule.
const gitIgnoreRuleTarget = validator.BranchLinkFileName

// gitIgnoreLeasesEntry é o padrão exato emitido para ignorar o diretório de leases.
// Caminhos absolutos da máquina do arquiteto não devem ser versionados.
// ML-3C (ADR-2026-10-23): o diretório .trackfw/leases/ é state-per-checkout, como o branch-links.
const gitIgnoreLeasesEntry = ".trackfw/leases/"

// gitIgnoreLeasesBlock é o bloco emitido para garantir que .trackfw/leases/ seja ignorado.
func gitIgnoreLeasesBlock() string {
	return `# trackfw: per-agent write-scope leases (D4, ADR-2026-10-23) — contain absolute paths
# of the dispatching machine; committing them would govern writes on a different checkout.
.trackfw/leases/
`
}

// hasGitIgnoreLeasesRule decide idempotência para o diretório de leases: verdadeiro quando
// ALGUMA linha não-comentário já cobre .trackfw/leases/ (inclusive .trackfw/).
func hasGitIgnoreLeasesRule(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Strip negation prefix and trailing slash for comparison.
		pattern := strings.TrimSuffix(strings.TrimPrefix(trimmed, "!"), "/")
		// Exact match: .trackfw/leases
		if pattern == ".trackfw/leases" {
			return true
		}
		// Parent directory: .trackfw (covers all of .trackfw/, including leases/).
		if pattern == ".trackfw" {
			return true
		}
	}
	return false
}

// effectiveRoadmapDir devolve o `roadmap_dir` do trackfw.yaml do diretório
// corrente, ou o default quando o arquivo está ausente/ilegível.
//
// 🔴 Deliberadamente NÃO usa config.Load(): Load é singleton `once.Do` e cacheia
// o primeiro trackfw.yaml lido no processo — num gerador chamado após chdir (e nos
// testes, que rodam vários cwd no mesmo binário) isso devolveria a config de outro
// projeto. config.ParseDirsFromContent é o mesmo parser sem o cache, e já tem o
// contrato de "chave ausente cai no default".
func effectiveRoadmapDir() string {
	data, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		_, roadmapDir, _ := config.ParseDirsFromContent("")
		return roadmapDir
	}
	_, roadmapDir, _ := config.ParseDirsFromContent(string(data))
	return roadmapDir
}

// gitIgnoreBlock é o bloco emitido no caminho de criação E no de append, com o
// `roadmap_dir` EFETIVO do projeto — nunca `docs/roadmaps` fixo (o produto
// presumir o layout do mantenedor é a família de defeito do #396).
func gitIgnoreBlock(roadmapDir string) string {
	return fmt.Sprintf(`# trackfw: branch<->roadmap link written by `+"`trackfw branch new`"+` — PER-CHECKOUT state,
# not versioned. A clone, a fork and CI never have it by design: inference by branch
# name is the fallback for that population. Committing it would let the link recorded
# on one machine govern another, possibly on a different branch.
%s/%s
`, strings.TrimSuffix(roadmapDir, "/"), gitIgnoreRuleTarget)
}

// hasGitIgnoreRule decide idempotência: verdadeiro quando ALGUMA linha
// não-comentário tem um padrão cujo basename é o arquivo de vínculo — qualquer
// caminho, e inclusive a forma negada (`!...`), que é decisão explícita do projeto
// e não deve ser contrariada por append.
func hasGitIgnoreRule(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		pattern := strings.TrimSuffix(strings.TrimPrefix(trimmed, "!"), "/")
		if path.Base(pattern) == gitIgnoreRuleTarget {
			return true
		}
	}
	return false
}

// gitIgnoreAgentMemoryEntry is the exact pattern emitted to ignore .claude/agent-memory/
// directories created in subdirectories when Claude Code is dispatched with a cwd other
// than the repository root (issue #646). The */**/ prefix requires at least one path
// component before .claude/, so the legitimate root .claude/agent-memory/ is not matched.
const gitIgnoreAgentMemoryEntry = "*/**/.claude/agent-memory/"

// gitIgnoreAgentMemoryBlock returns the block emitted to guard against stray agent-memory
// directories created by the Claude Code harness when the dispatch cwd is not the repo root.
// The pattern line is derived from gitIgnoreAgentMemoryEntry so that detection and emission
// never diverge.
func gitIgnoreAgentMemoryBlock() string {
	return fmt.Sprintf(`# trackfw: agent-memory directories created in subdirectories when Claude Code is
# dispatched from a cwd other than the repository root (issue #646). These are
# harness residue or lost memory — never version-controlled. The root
# .claude/agent-memory/ (legitimate, versioned) is NOT matched by this pattern.
%s
`, gitIgnoreAgentMemoryEntry)
}

// hasGitIgnoreAgentMemoryRule reports whether content already contains the exact
// pattern */**/.claude/agent-memory/ on a non-comment, non-negated, non-empty line.
//
// Exact-line match only: basename checks, Contains, and broader patterns such as
// **/.claude/agent-memory/ produce different ignore semantics and are intentionally
// not treated as present — they must not silently suppress the correct, narrower rule.
// Lines starting with ! (explicit project negation) are skipped: the caller (generateGitIgnore)
// is responsible for detecting negation lines and printing a warning instead of appending.
func hasGitIgnoreAgentMemoryRule(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		if trimmed == gitIgnoreAgentMemoryEntry {
			return true
		}
	}
	return false
}

// gitIgnoreAgentMemoryNegationLine returns the first negation line (starting with !)
// whose pattern — with the leading ! and any trailing / stripped — ends with
// .claude/agent-memory AND is not anchored to the repository root. Returns "" if none
// is found or only root-anchored negations are present.
//
// Root-anchored forms (.claude/agent-memory and /.claude/agent-memory) only un-ignore
// the top-level directory and do NOT conflict with */**/.claude/agent-memory/, which
// targets subdirectories. Treating them as opt-outs would silently skip the rule in
// projects that version their root-level memory while wanting subdirectory instances
// ignored (issue #646, A-1).
//
// Non-root forms such as lib/.claude/agent-memory, **/.claude/agent-memory, or
// */.claude/agent-memory can match subdirectories and therefore do represent an
// explicit consumer opt-out. Appending */**/.claude/agent-memory/ after any such
// negation would override it under gitignore's last-match-wins rule (vault note:
// gitignore-negation-override-by-append-2026-11-01.md). generateGitIgnore calls this
// function to detect the opt-out, skip the append, and warn the user.
func gitIgnoreAgentMemoryNegationLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "!") {
			continue
		}
		// Strip leading ! and trailing /.
		pattern := strings.TrimPrefix(trimmed, "!")
		pattern = strings.TrimSuffix(pattern, "/")
		if !strings.HasSuffix(pattern, ".claude/agent-memory") {
			continue
		}
		// Skip root-anchored forms: bare ".claude/agent-memory" (no path separator
		// before the pattern) and "/.claude/agent-memory" (explicit root anchor).
		// These only un-ignore the top-level directory and are compatible with
		// */**/.claude/agent-memory/.
		if pattern == ".claude/agent-memory" || pattern == "/.claude/agent-memory" {
			continue
		}
		return trimmed
	}
	return ""
}

// generateGitIgnore garante que `<roadmap_dir>/.trackfw-branch-links.json`, `.trackfw/leases/`
// e `*/**/.claude/agent-memory/` estejam ignorados no `.gitignore` da raiz do projeto. Cada
// regra é gerenciada independentemente para que projetos que já têm algumas regras recebam
// as demais sem duplicação.
//
// Três ramos por regra, todos idempotentes — mesmo desenho de generateGitAttributes:
//   - arquivo ausente  → cria com o bloco completo (branch-links + leases + agent-memory)
//   - existe sem regra → APPEND do bloco correspondente (nunca sobrescreve o arquivo do projeto)
//   - existe com regra → no-op para aquela regra específica
func generateGitIgnore() error {
	giRoot, giErr := projectRoot()
	if giErr != nil {
		return fmt.Errorf("generateGitIgnore: %w", giErr)
	}
	absGitIgnore := filepath.Join(giRoot, ".gitignore")
	if err := rejectScaffoldPath(giRoot, absGitIgnore); err != nil {
		return err
	}
	const gitIgnorePath = ".gitignore"
	branchLinksBlock := gitIgnoreBlock(effectiveRoadmapDir())
	leasesBlock := gitIgnoreLeasesBlock()
	agentMemoryBlock := gitIgnoreAgentMemoryBlock()

	existing, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading .gitignore: %w", err)
		}
		// File absent: create with all three blocks.
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile(gitIgnorePath, []byte(branchLinksBlock+leasesBlock+agentMemoryBlock), 0644); err != nil {
			return fmt.Errorf("writing .gitignore: %w", err)
		}
		fmt.Println("  ✓ .gitignore")
		return nil
	}

	// File exists: append only the missing rules.
	content := string(existing)
	needsBranchLinks := !hasGitIgnoreRule(content)
	needsLeases := !hasGitIgnoreLeasesRule(content)
	needsAgentMemory := !hasGitIgnoreAgentMemoryRule(content)
	if needsAgentMemory {
		if negLine := gitIgnoreAgentMemoryNegationLine(content); negLine != "" {
			fmt.Printf("  ⚠ .gitignore: negation %q respected — */**/.claude/agent-memory/ rule not added\n", negLine)
			needsAgentMemory = false
		}
	}

	if !needsBranchLinks && !needsLeases && !needsAgentMemory {
		return nil
	}

	// Arquivo preexistente sem newline final: emendar o bloco direto grudaria a
	// primeira linha do bloco no último padrão do projeto, transformando os dois
	// numa linha só e corrompendo o .gitignore dele em silêncio.
	out := content
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if needsBranchLinks {
		out += branchLinksBlock
	}
	if needsLeases {
		out += leasesBlock
	}
	if needsAgentMemory {
		out += agentMemoryBlock
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(gitIgnorePath, []byte(out), 0644); err != nil {
		return fmt.Errorf("appending to .gitignore: %w", err)
	}
	fmt.Println("  ✓ .gitignore")
	return nil
}

// lefthookValidatePresent reports whether a trackfw-validate: entry already lives
// inside the top-level pre-commit: block of a lefthook.yml file.
//
// It uses the same section-tracking logic as the merge path in generateLefthookHook:
// iterate lines, track inPreCommit by watching column-0 non-comment keys, and look for
// "trackfw-validate:" only while inside that section. This avoids false positives from:
//   - the key appearing under a different top-level block (e.g. pre-push:)
//   - the key appearing inside a comment (# trackfw-validate:)
//
// Inline YAML comments (space + "#" sequence after the key) are stripped before
// comparison so that "trackfw-validate: # installed by trackfw" is treated as present.
// Caveat: if trackfw-validate: ever carried a quoted value containing " #" (e.g.
// trackfw-validate: "run # something"), this truncation would incorrectly strip part
// of the value. Since this key carries no quoted value in practice — only optional
// trailing comments — the truncation is safe. Declaring rather than hiding the edge
// case (mirrors the equivalent caveat in scripts/check-init-preserves-user-config.sh).
func lefthookValidatePresent(content string) bool {
	inPreCommit := false
	for _, line := range strings.Split(content, "\n") {
		// Track top-level sections (column-0, non-whitespace, non-comment).
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			inPreCommit = line == "pre-commit:"
		}
		// Strip inline YAML comment (" #" sequence) before comparing.
		// A leading '#' (full-line comment) is already excluded by the section-tracking
		// guard above. Here we only need to handle trailing inline comments.
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, " #"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if inPreCommit && trimmed == "trackfw-validate:" {
			return true
		}
	}
	return false
}

func generateLefthookHook() error {
	lhRoot, lhErr := projectRoot()
	if lhErr != nil {
		return fmt.Errorf("generateLefthookHook: %w", lhErr)
	}
	absLH := filepath.Join(lhRoot, "lefthook.yml")
	if err := rejectScaffoldPath(lhRoot, absLH); err != nil {
		return err
	}

	const validateEntry = "    trackfw-validate:\n      run: trackfw validate\n"
	const fullBlock = "pre-commit:\n  commands:\n" + validateEntry

	// Read before write — guard that preserves consumer-authored lefthook.yml content.
	existing, _ := os.ReadFile("lefthook.yml")
	existingStr := string(existing)

	// Idempotent: trackfw-validate already installed inside pre-commit: → no-op.
	// Uses the same section-tracking logic as the merge path below to avoid false
	// positives from the key appearing in comments or under a different top-level
	// block (e.g. pre-push:).
	if lefthookValidatePresent(existingStr) {
		return nil
	}

	var out string
	if !presentTopLevelKeys(existingStr)["pre-commit"] {
		// No pre-commit block yet: append the full block with a blank-line separator.
		out = existingStr
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		out += fullBlock
	} else {
		// pre-commit block exists: insert trackfw-validate under its commands: entry.
		lines := strings.Split(existingStr, "\n")
		result := make([]string, 0, len(lines)+2)
		inPreCommit := false
		inserted := false
		for _, line := range lines {
			// Track top-level sections (column-0, non-comment).
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
				inPreCommit = line == "pre-commit:"
			}
			result = append(result, line)
			if inPreCommit && !inserted && strings.TrimSpace(line) == "commands:" {
				result = append(result, "    trackfw-validate:")
				result = append(result, "      run: trackfw validate")
				inserted = true
			}
		}
		if !inserted {
			// pre-commit exists but has no commands: section — warn and skip to avoid
			// corrupting a file with an unknown layout.
			fmt.Println("  ⚠ lefthook.yml: pre-commit block has no 'commands:' section — trackfw-validate not added")
			return nil
		}
		out = strings.Join(result, "\n")
	}

	// Guarda real: leitura de lefthook.yml acima — o merge só acrescenta o que falta.
	// Não há marcador de isenção declarativa aqui: com a leitura detectável na mesma
	// função, a isenção seria redundante e mascararia a perda da guarda se o bloco
	// de leitura fosse extraído para um helper em refatores futuros.
	// Distinção de gates: write-containment-allowed pertence ao check-write-containment/pathguard
	// (contenção de escrita — este sítio). O marcador de preservação de config do consumidor
	// (check-init-preserves-user-config) NÃO deve estar aqui — sua presença enfraquecia aquele gate
	// porque a detecção real é via os.ReadFile, não via marcador declarativo (ML-1C/ML-1D).
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile("lefthook.yml", []byte(out), 0644); err != nil {
		return fmt.Errorf("writing lefthook config: %w", err)
	}
	fmt.Println("  ✓ lefthook.yml")
	return nil
}
