package generators

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/guardforms"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/kgsaran/trackfw/internal/validator"
)

// loadUpdateConfig converts the Update namespace resolved by the single config
// loader (config.Load(), see internal/config/config.go and
// ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-tipados.md) into the
// generators.Config shape this file's writers expect. Replaces the former artisanal scanner
// ReadUpdateConfig — config.Load() reads relative to the process' current working directory, so
// callers must invoke this only after chdir'ing into the target project root.
func loadUpdateConfig() Config {
	u := config.Load().Update
	return Config{
		Hooks:             u.Hooks,
		CI:                u.CI,
		Backend:           u.Backend,
		Frontend:          u.Frontend,
		PkgManager:        u.PkgManager,
		CITrigger:         u.CITrigger,
		CITriggerInvalid:  u.CITriggerInvalid,
		CITriggerRawValue: u.CITriggerRawValue,
	}
}

// Update re-aplica todos os templates atuais do trackfw ao projeto em cwd.
func Update(cwd string) error {
	if _, err := os.Stat(filepath.Join(cwd, "trackfw.yaml")); err != nil {
		return fmt.Errorf("trackfw.yaml não encontrado — execute trackfw init primeiro")
	}

	if err := ensureGlobalADRDirRegistered(cwd); err != nil {
		fmt.Printf("  ⚠ adr_dirs: %v\n", err)
	}

	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		return fmt.Errorf("não foi possível mudar para %s: %w", cwd, err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	cfg := loadUpdateConfig()

	// AC4 (REQ-2026-10-24): refuse early when ci_trigger is invalid so no write happens
	// and the existing file is left untouched. rc≠0 propagated by cobra.
	if cfg.CITriggerInvalid {
		return fmt.Errorf("ci_trigger: %q é inválido — valores aceitos: pull_request, manual", cfg.CITriggerRawValue)
	}

	fmt.Println("trackfw update — re-aplicando templates atuais...")
	fmt.Println()

	// 1. Regras de agente (categoria 1 — marker-delimited)
	if err := InjectRulesDetected(cwd); err != nil {
		fmt.Printf("  ⚠ agent rules: %v\n", err)
	} else {
		fmt.Println("  ✓ agent rules atualizadas")
	}

	// 1b. Agent hooks (attention signal)
	if err := InjectHooksDetected(cwd); err != nil {
		fmt.Printf("  ⚠ agent hooks: %v\n", err)
	} else {
		fmt.Println("  ✓ agent hooks atualizados")
	}
	_, agentsErr := os.Stat(filepath.Join(cwd, "AGENTS.md"))
	_, codexErr := os.Stat(filepath.Join(cwd, ".codex"))
	if agentsErr == nil || codexErr == nil {
		if err := updateDetectedCodexIntegrations(cwd); err != nil {
			return fmt.Errorf("codex integration update: %w", err)
		}
	}
	// 2. Validate script (categoria 2 — trackfw-owned, overwrite seguro)
	if err := generateValidateScript(cfg); err != nil {
		fmt.Printf("  ⚠ validate script: %v\n", err)
	}

	if err := GenerateAttentionScripts(""); err != nil {
		fmt.Printf("  ⚠ attention scripts: %v\n", err)
	}

	if err := GenerateCredentialGuardScript(""); err != nil {
		fmt.Printf("  ⚠ credential guard script: %v\n", err)
	}

	if err := GenerateGitBranchGuardScript(""); err != nil {
		fmt.Printf("  ⚠ git branch guard script: %v\n", err)
	}

	// 2b. .gitignore: garante que <roadmap_dir>/.trackfw-branch-links.json esteja
	// ignorado (ML-3D). Este é o caminho do consumidor JÁ ONBOARDADO, que não vai
	// rodar `init` de novo — sem ele, a correção só alcançaria projetos novos.
	// Idempotente e nunca sobrescreve: append quando falta a regra, no-op quando
	// alguma linha já nomeia o arquivo.
	if err := generateGitIgnore(); err != nil {
		fmt.Printf("  ⚠ .gitignore: %v\n", err)
	}

	// 3. CI workflow (categoria 2 — trackfw-owned, overwrite seguro)
	if err := generateCIWorkflow(cfg); err != nil {
		fmt.Printf("  ⚠ CI workflow: %v\n", err)
	} else if cfg.CI != "" && cfg.CI != "none" {
		fmt.Println("  ✓ CI workflow atualizado")
	}

	// 3b. discover-installed CI workflow (.github/workflows/trackfw-validate.yml),
	// present regardless of cfg.CI (AC17(c), REQ-2026-08-28) — same shared writer
	// the "ci-workflow" project target uses (runProjectTarget, below), so the
	// simple `trackfw update` path and the `--targets ci-workflow` path can never
	// drift apart on what "refreshed" means. No-ops when the file isn't present
	// (AC17(b) — update never installs it, only `trackfw discover --init` does).
	if err := refreshDiscoverGitHubActionsWorkflowIfPresent(cwd); err != nil {
		fmt.Printf("  ⚠ CI workflow (discover): %v\n", err)
	} else if discoverWorkflowPresent(cwd) {
		fmt.Println("  ✓ CI workflow (discover) atualizado")
	}

	// 4. Git hooks — cirúrgico (categoria 3 — shared user files)
	updateHooksSurgical(cwd, cfg)

	// 5. Historical Claude slash commands are a project-scope auxiliary and
	// remain backward compatible here. The historical global Claude
	// compatibility skill (~/.claude/skills/trackfw/SKILL.md) is global state
	// and is intentionally NOT touched by this project-scope command anymore
	// — see 'trackfw update harness', which owns every global-scope target.
	if err := ForceGenerateClaudeCommands(); err != nil {
		fmt.Printf("  ⚠ Claude commands: %v\n", err)
	} else {
		fmt.Println("  ✓ .claude/commands/trackfw/ atualizado")
	}

	fmt.Println("\n✓ trackfw update concluído")
	PrintArchitectNextSteps(cwd)
	return nil
}

// updateDetectedCodexIntegrations re-applies managed Codex agent/skill
// artifacts already installed in the project, using the identity currently
// persisted at ~/.trackfw/identity.json.
//
// Only identity.Load failing aborts the whole update (returns an error): an
// error there means we cannot tell whether the user has a customized
// identity, and silently falling back to the neutral default would revert it
// without warning. Every other failure here (catalog, home, per-kind
// planning, per-artifact inspection) keeps its original warn-and-continue
// behavior — those are unrelated to identity and must not turn a single
// unreadable Codex artifact into a reason to skip the rest of `trackfw
// update` (CI workflow, git hooks, .claude/commands, legacy skill, ...).
func updateDetectedCodexIntegrations(cwd string) error {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		fmt.Printf("  ⚠ Codex integration catalog: %v\n", err)
		return nil
	}
	home, err := homedir.Dir()
	if err != nil {
		fmt.Printf("  ⚠ Codex integration home: %v\n", err)
		return nil
	}
	ident, err := identity.Load(home)
	if err != nil {
		return fmt.Errorf("codex integration identity: %w", err)
	}
	// ML-1D: ClaudeDir is intentionally not set — this Manager handles Codex
	// project-scope artifacts only (Targets: []string{"codex"}, Scope: "project").
	// No claude/global plans reach it, so ClaudeDir is behaviour-neutral here.
	manager := integrations.Manager{ProjectRoot: cwd, HomeDir: home}
	updated := 0
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		plans, planErr := integrations.BuildPlans(catalog, integrations.PlanRequest{Kind: kind, Targets: []string{"codex"}, Scope: "project", Identity: ident, AgentModels: config.Load().AgentModels})
		if planErr != nil {
			fmt.Printf("  ⚠ Codex %s plans: %v\n", kind, planErr)
			continue
		}
		for _, plan := range plans {
			inspection, inspectErr := manager.Inspect(plan)
			if inspectErr != nil {
				fmt.Printf("  ⚠ Codex %s/%s inspect: %v\n", kind, plan.Claim.Item, inspectErr)
				continue
			}
			if inspection.State == integrations.StateNotInstalled {
				continue
			}
			if updateErr := manager.Update([]integrations.PlannedArtifact{plan}, false); updateErr != nil {
				fmt.Printf("  ⚠ Codex %s/%s preservado: %v\n", kind, plan.Claim.Item, updateErr)
				continue
			}
			updated++
		}
	}
	if updated > 0 {
		fmt.Printf("  ✓ %d Codex agent/skill artifact(s) migrated or updated\n", updated)
	}
	return nil
}

// updateHooksSurgical garante que 'trackfw validate' está presente nos hooks sem sobrescrever conteúdo do usuário.
// rootDir must be the absolute path of the project root as the caller knows it;
// the guard root and every path below are derived from its RESOLVED form
// (ML-8A / #402).
//
// NON-FATAL by decision: a hook-file failure must never abort `trackfw update`.
// The decision lives HERE, in the wrapper that discards the error — and only
// here. updateHooksSurgicalEntry below returns the refusal like any other
// guarded write, which is what lets an unverifiable root refuse with a plain
// `return pathguard.RefuseUnverifiableRoot(...)` instead of a discarded call
// whose error nothing consumes (same shape as appendTransitionLog, ML-9A).
func updateHooksSurgical(rootDir string, cfg Config) {
	_ = updateHooksSurgicalEntry(rootDir, cfg)
}

func updateHooksSurgicalEntry(rootDir string, cfg Config) error {
	// The relative name is decided BEFORE the root is resolved so that a refusal
	// can name the file the user was expecting to see updated.
	var relPath string
	switch cfg.Hooks {
	case "husky":
		relPath = filepath.Join(".husky", "pre-commit")
	case "lefthook":
		relPath = "lefthook.yml"
	default:
		return nil
	}
	// ML-8A / #402: filepath.Clean(cwd) normalised text and never resolved a
	// symlink, so the guard root could not contain a target derived from the
	// resolved tree. Both operands now come from the resolved root.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, relPath), rootErr)
	}

	switch cfg.Hooks {
	case "husky":
		path := filepath.Join(guardRoot, ".husky", "pre-commit")
		// Guard: reject if any ancestor of path is a symlink.
		if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
			fmt.Printf("  ⚠ .husky/pre-commit: %v\n", guardErr)
			return guardErr
		}
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "trackfw validate") {
			fmt.Println("  ✓ .husky/pre-commit — trackfw validate já presente")
			return nil
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		os.MkdirAll(filepath.Join(guardRoot, ".husky"), 0755) //nolint:errcheck
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			fmt.Printf("  ⚠ .husky/pre-commit: %v\n", err)
			return err
		}
		defer f.Close()
		fmt.Fprintln(f, "\ntrackfw validate")
		fmt.Println("  ✓ .husky/pre-commit — trackfw validate injetado")

	case "lefthook":
		path := filepath.Join(guardRoot, "lefthook.yml")
		// Guard: reject if any ancestor of path is a symlink.
		if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
			fmt.Printf("  ⚠ lefthook.yml: %v\n", guardErr)
			return guardErr
		}
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "trackfw-validate:") || strings.Contains(string(data), "trackfw validate") {
			fmt.Println("  ✓ lefthook.yml — trackfw já presente")
			return nil
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("  ⚠ lefthook.yml: %v\n", err)
			return err
		}
		defer f.Close()
		fmt.Fprintln(f, "\npre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate")
		fmt.Println("  ✓ lefthook.yml — trackfw-validate injetado")
	}
	return nil
}

// ensureGlobalADRDirRegistered registers ~/.trackfw/adr in trackfw.yaml's
// adr_dirs list, surgically (text-level edit, never config.Load()+re-
// serialize, which would lose the user's comments/formatting) and
// idempotently, but only when there is something to gain from it: the global
// ADR directory must exist AND contain at least one ADR-*.md file. This keeps
// `trackfw update` from cluttering every project's trackfw.yaml with a
// pointer to a directory that is empty or doesn't exist on this machine.
func ensureGlobalADRDirRegistered(cwd string) error {
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	globalDir := GlobalADRDir(home)
	if _, statErr := os.Stat(globalDir); statErr != nil {
		return nil // global ADR dir doesn't exist — no-op
	}
	// Use the recursive point-of-truth (ML-1C / #450): Glob root-only missed ADRs
	// that live only in subdirectories (e.g. ~/.trackfw/adr/done/ADR-*.md).
	if len(validator.WalkADRFilePaths(globalDir)) == 0 {
		return nil // global ADR dir has no ADRs yet — no-op
	}

	// ML-8A / #402: the guard root and yamlPath must both live in the resolved
	// namespace — filepath.Clean(cwd) normalised text only and never resolved a
	// symlink, so on macOS a cwd of /tmp/p could not contain /private/tmp/p/....
	guardRoot, rootErr := pathguard.ResolveRoot(cwd)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(cwd, "trackfw.yaml"), rootErr)
	}
	yamlPath := filepath.Join(guardRoot, "trackfw.yaml")
	data, readErr := os.ReadFile(yamlPath)
	if readErr != nil {
		return fmt.Errorf("reading %s: %w", yamlPath, readErr)
	}
	content := string(data)

	absGlobalDir := filepath.Join(home, ".trackfw", "adr")
	if adrDirsEntryPresent(content, absGlobalDir) {
		return nil // already registered (literal "~/.trackfw/adr" or the expanded absolute path)
	}

	updated, insertErr := insertGlobalADRDirEntry(content)
	if insertErr != nil {
		return insertErr
	}
	// Guard: reject writes through symlinks before mutating trackfw.yaml
	// (ADR-2026-09-18 / ML-1B). Root is the RESOLVED project directory.
	if guardErr := pathguard.RejectAndReport(guardRoot, yamlPath); guardErr != nil {
		return guardErr
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(yamlPath, []byte(updated), 0o644); writeErr != nil {
		return fmt.Errorf("writing %s: %w", yamlPath, writeErr)
	}
	fmt.Println("  ✓ adr_dirs: ~/.trackfw/adr registrado")
	return nil
}

// adrDirsEntryPresent reports whether content's adr_dirs block (if any)
// already has an item resolving to the global ADR dir — matching both the
// literal "~/.trackfw/adr" form and the expanded absolute-path form so the
// two textual spellings of the same entry are never treated as distinct.
func adrDirsEntryPresent(content, absGlobalDir string) bool {
	lines := strings.Split(content, "\n")
	inADRDirs := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasPrefix(strings.TrimLeft(trimmed, " "), "adr_dirs:") {
			inADRDirs = true
			continue
		}
		if inADRDirs {
			itemLine := strings.TrimLeft(trimmed, " ")
			if !strings.HasPrefix(itemLine, "-") {
				break // list ended
			}
			value := strings.TrimSpace(strings.TrimPrefix(itemLine, "-"))
			if value == "~/.trackfw/adr" || value == absGlobalDir {
				return true
			}
		}
	}
	return false
}

// insertGlobalADRDirEntry returns content with "  - ~/.trackfw/adr" inserted
// as the last item of the existing adr_dirs list, or — if content has no
// adr_dirs key at all (implying the loader's implicit "docs/adr" default) —
// with a new adr_dirs block appended at the end preserving that default
// explicitly alongside the new global entry.
func insertGlobalADRDirEntry(content string) (string, error) {
	lines := strings.Split(content, "\n")
	adrDirsIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(strings.TrimRight(line, " \t"), " "), "adr_dirs:") {
			adrDirsIdx = i
			break
		}
	}
	if adrDirsIdx == -1 {
		// No adr_dirs key present — append a new explicit block.
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if !strings.HasSuffix(content, "\n\n") && content != "\n" {
			content += "\n"
		}
		content += "adr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n"
		return content, nil
	}

	// Find the indentation used by existing list items and the index of the
	// last item line so the new entry can be inserted right after it, with
	// matching indentation.
	itemIndent := "  "
	lastItemIdx := adrDirsIdx
	for i := adrDirsIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], " \t")
		leftTrimmed := strings.TrimLeft(trimmed, " ")
		if !strings.HasPrefix(leftTrimmed, "-") {
			break
		}
		itemIndent = trimmed[:len(trimmed)-len(leftTrimmed)]
		lastItemIdx = i
	}

	newLine := itemIndent + "- ~/.trackfw/adr"
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:lastItemIdx+1]...)
	out = append(out, newLine)
	out = append(out, lines[lastItemIdx+1:]...)
	return strings.Join(out, "\n"), nil
}

// ────────────────────────────────────────────────────────────────────────────
// trackfw update harness — global-scope update, contract in docs/cli-parity.md
// ("## `trackfw update` vs `trackfw update harness`"). Everything below this
// point mutates only the user's home directory, never a project working tree.
// ────────────────────────────────────────────────────────────────────────────

// TargetState is one of the four pinned states of the update harness contract.
type TargetState string

const (
	TargetUpdated TargetState = "updated"
	TargetSkipped TargetState = "skipped"
	TargetMissing TargetState = "missing"
	TargetFailed  TargetState = "failed"
)

// TargetResult is one evaluated harness target.
type TargetResult struct {
	ID      string
	State   TargetState
	Path    string
	Message string // set when State == TargetFailed (error detail) or for informational notices (e.g. D5 inert-remainder, ML-1B)
}

// UpdateSummary carries all four counters, including zeros — pinned by contract.
type UpdateSummary struct {
	Updated int
	Skipped int
	Missing int
	Failed  int
}

// UpdateReport is the scope-agnostic result of running trackfw update or
// trackfw update harness. Scope is "project" or "harness".
type UpdateReport struct {
	Scope   string
	DryRun  bool
	Targets []TargetResult
}

// Summary tallies Targets into the four pinned counters.
func (r UpdateReport) Summary() UpdateSummary {
	var s UpdateSummary
	for _, t := range r.Targets {
		switch t.State {
		case TargetUpdated:
			s.Updated++
		case TargetSkipped:
			s.Skipped++
		case TargetMissing:
			s.Missing++
		case TargetFailed:
			s.Failed++
		}
	}
	return s
}

// UpdateOptions carries the four flags shared by trackfw update and
// trackfw update harness.
type UpdateOptions struct {
	DryRun         bool
	Targets        []string // subset of declared target ids; empty selects all
	InstallMissing bool
}

// harnessCatalogTargetOrder is the fixed catalog target order the pinned
// harness target list is built from (docs/cli-parity.md, "Declared harness
// targets — pinned list"): claude-skill, then <tool>-agents/<tool>-skills
// for each of these ten catalog.json targets, in this exact order.
var harnessCatalogTargetOrder = []string{
	"claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "opencode", "kiro",
}

// HarnessTargetIDs is the fixed, declared order of `trackfw update harness`
// targets: 34 ids — "claude-skill", then "claude-credential-guard" and
// "claude-git-branch-guard" (global hook wiring for Claude Code — placed
// immediately after claude-skill since all three are Claude-Code-scoped
// global artifacts), then for each catalog target in
// harnessCatalogTargetOrder its "<tool>-agents"/"<tool>-skills" pair, with
// "codex-credential-guard"/"codex-git-branch-guard" inserted immediately
// BEFORE "codex-agents"/"codex-skills" (ROADMAP-2026-08-06 Wave 2/ML-2B,
// ROADMAP-2026-08-17 Wave 2/ML-2A) and "gemini-credential-guard"/
// "gemini-git-branch-guard" inserted immediately BEFORE "gemini-agents"/
// "gemini-skills" (ML-2C), and "cursor-credential-guard"/
// "cursor-git-branch-guard" inserted immediately BEFORE "cursor-agents"/
// "cursor-skills" (ML-2D), and "copilot-credential-guard"/
// "copilot-git-branch-guard" inserted immediately BEFORE "copilot-agents"/
// "copilot-skills" (ML-2E), "windsurf-credential-guard" inserted immediately
// BEFORE "windsurf-agents"/"windsurf-skills" (ML-1B, REQ-2026-10-06 — global
// credential guard for Windsurf at ~/.codeium/windsurf/hooks.json; Windsurf
// now has pre_run_command and pre_write_code hooks so the ADR-2026-08-05
// premise that excluded Windsurf has been superseded — see adendo 2026-10-08;
// NO windsurf-git-branch-guard global target: Windsurf does not have a
// documented global git-branch-guard hook path equivalent to the project-scope
// .windsurf/hooks.json), and "kiro-credential-guard"/
// "kiro-git-branch-guard" inserted immediately BEFORE "kiro-agents"/
// "kiro-skills" (ML-2F). Within each tool, credential-guard always precedes
// git-branch-guard (where it exists), which always precedes that tool's own
// agents/skills pair, never follows it. Order here is authoritative for both
// JSON output and iteration — it must never be derived from the filesystem or
// from what happens to be installed on a given machine (see docs/cli-parity.md,
// "targets follows the declared target order, not filesystem order").
var HarnessTargetIDs = buildHarnessTargetIDs()

func buildHarnessTargetIDs() []string {
	ids := make([]string, 0, 7+2*len(harnessCatalogTargetOrder))
	// Script targets precede every wiring target that references them so that
	// even a --targets-filtered run that only requests a wiring target still
	// finds the script already in place from a prior full run. The two script
	// targets are 100% trackfw-owned and do not require --install-missing.
	ids = append(ids, "claude-skill", "git-branch-guard-script", "credential-guard-script",
		"claude-credential-guard", "claude-git-branch-guard")
	for _, tool := range harnessCatalogTargetOrder {
		if tool == "codex" {
			ids = append(ids, "codex-credential-guard", "codex-git-branch-guard")
		}
		if tool == "gemini" {
			ids = append(ids, "gemini-credential-guard", "gemini-git-branch-guard")
		}
		if tool == "cursor" {
			ids = append(ids, "cursor-credential-guard", "cursor-git-branch-guard")
		}
		if tool == "copilot" {
			ids = append(ids, "copilot-credential-guard", "copilot-git-branch-guard")
		}
		if tool == "windsurf" {
			ids = append(ids, "windsurf-credential-guard")
		}
		if tool == "kiro" {
			ids = append(ids, "kiro-credential-guard", "kiro-git-branch-guard")
		}
		ids = append(ids, tool+"-agents", tool+"-skills")
	}
	return ids
}

// UnknownHarnessTargetError is returned by UpdateHarness when --targets names
// an id outside HarnessTargetIDs. Per contract this is a usage error.
type UnknownHarnessTargetError struct{ ID string }

func (e *UnknownHarnessTargetError) Error() string {
	return fmt.Sprintf("unknown target %q", e.ID)
}

// UpdateHarness evaluates (and, unless DryRun, applies) every declared harness
// target already installed in the user's home directory. It never requires
// trackfw.yaml or a project working directory.
func UpdateHarness(opts UpdateOptions) (UpdateReport, error) {
	selected, err := selectDeclaredTargets(HarnessTargetIDs, opts.Targets)
	if err != nil {
		return UpdateReport{}, err
	}

	home, homeErr := homedir.Dir()
	if homeErr != nil {
		return UpdateReport{}, fmt.Errorf("resolving home directory: %w", homeErr)
	}
	// filepath.Clean normalises trailing slashes and "." segments before home
	// is used as the RejectSymlinks root — the walk's "current == root"
	// termination condition is a string equality, so a trailing slash or "."
	// would cause the walk to overshoot root and return "escapes root" for
	// every target (ADR-2026-09-18 / ML-1B precondition).
	//
	// 🔴 ML-8A / #402 deliberately does NOT resolve home here, and the reason is
	// measured, not assumed. This `home` is not only the guard root: it is
	// embedded VERBATIM in the hook command paths written into
	// .claude/settings.json, .codex/hooks.json, .gemini/settings.json,
	// .cursor/hooks.json and friends. Swapping it for the EvalSymlinks form
	// changes the CONTENT of every generated artifact (/var/... →
	// /private/var/... on macOS) — 15 tests pin the logical form — and that is a
	// product decision about what trackfw writes into the user's config, not the
	// argument fix this ML owns.
	//
	// Containment is NOT degraded by leaving it logical: every harness target is
	// built by filepath.Join from this same `home`, so the guard root and the
	// guarded target live in the SAME namespace. Armadilha 3 is the MISMATCH
	// between the two namespaces, not the choice of namespace. The residual is
	// that P2 still reports rejectHarnessSymlink()'s `home` parameter as
	// unresolved; that finding is pinned by name in containment_live_test.go
	// with this reason.
	home = filepath.Clean(home)

	// ML-1A (REQ-2026-09-01): Claude Code's config dir may differ from home/.claude on Windows
	// (USERPROFILE ≠ HOME in Git Bash) or when CLAUDE_CONFIG_DIR is set. Compute it once here
	// and pass it to the two Claude-specific target functions — ADR-2026-10-14 D3.
	claudeDir, claudeDirErr := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if claudeDirErr == nil {
		claudeDir = filepath.Clean(claudeDir)
	}

	// NOTE: the script files are now first-class targets ("git-branch-guard-script",
	// "credential-guard-script") positioned before the wiring targets in
	// HarnessTargetIDs. The pre-loop unconditional writes that used to live here
	// have been removed — the target functions handle creation and idempotency.
	// (REQ-2026-09-09, ML-1A)

	catalog, catalogErr := integrations.LoadCatalog()
	if catalogErr != nil {
		return UpdateReport{}, fmt.Errorf("loading integration catalog: %w", catalogErr)
	}

	results := make([]TargetResult, 0, len(selected))
	for _, id := range selected {
		if id == "claude-skill" {
			results = append(results, harnessClaudeSkillTarget(home, claudeDir, claudeDirErr, opts))
			continue
		}
		if id == "git-branch-guard-script" {
			results = append(results, harnessGitBranchGuardScriptTarget(home, opts))
			continue
		}
		if id == "credential-guard-script" {
			results = append(results, harnessCredentialGuardScriptTarget(home, opts))
			continue
		}
		if id == "claude-credential-guard" {
			results = append(results, harnessCredentialGuardTargetClaude(home, claudeDir, claudeDirErr, opts))
			continue
		}
		if id == "claude-git-branch-guard" {
			results = append(results, harnessGitBranchGuardTargetClaude(home, claudeDir, claudeDirErr, opts))
			continue
		}
		if id == "codex-credential-guard" {
			results = append(results, harnessCredentialGuardTargetCodex(home, opts))
			continue
		}
		if id == "codex-git-branch-guard" {
			results = append(results, harnessGitBranchGuardTargetCodex(home, opts))
			continue
		}
		if id == "gemini-credential-guard" {
			results = append(results, harnessCredentialGuardTargetGemini(home, opts))
			continue
		}
		if id == "gemini-git-branch-guard" {
			results = append(results, harnessGitBranchGuardTargetGemini(home, opts))
			continue
		}
		if id == "cursor-credential-guard" {
			results = append(results, harnessCredentialGuardTargetCursor(home, opts))
			continue
		}
		if id == "cursor-git-branch-guard" {
			results = append(results, harnessGitBranchGuardTargetCursor(home, opts))
			continue
		}
		if id == "copilot-credential-guard" {
			results = append(results, harnessCredentialGuardTargetCopilot(home, opts))
			continue
		}
		if id == "copilot-git-branch-guard" {
			results = append(results, harnessGitBranchGuardTargetCopilot(home, opts))
			continue
		}
		if id == "windsurf-credential-guard" {
			results = append(results, harnessCredentialGuardTargetWindsurf(home, opts))
			continue
		}
		if id == "kiro-credential-guard" {
			results = append(results, harnessCredentialGuardTargetKiro(home, opts))
			continue
		}
		if id == "kiro-git-branch-guard" {
			results = append(results, harnessGitBranchGuardTargetKiro(home, opts))
			continue
		}
		tool, kind, ok := splitHarnessCatalogTargetID(id)
		if !ok {
			continue
		}
		results = append(results, harnessCatalogTarget(catalog, id, tool, kind, home, claudeDir, claudeDirErr, opts))
	}
	return UpdateReport{Scope: "harness", DryRun: opts.DryRun, Targets: results}, nil
}

// splitHarnessCatalogTargetID splits a "<tool>-agents"/"<tool>-skills" id
// into its tool id and ItemKind. ok is false for any id outside that shape
// (currently only "claude-skill", handled separately by its caller).
func splitHarnessCatalogTargetID(id string) (tool string, kind integrations.ItemKind, ok bool) {
	switch {
	case strings.HasSuffix(id, "-agents"):
		return strings.TrimSuffix(id, "-agents"), integrations.KindAgents, true
	case strings.HasSuffix(id, "-skills"):
		return strings.TrimSuffix(id, "-skills"), integrations.KindSkills, true
	default:
		return "", "", false
	}
}

// selectDeclaredTargets validates opts.Targets against declared (an unknown id
// is a usage error) and returns the requested subset of declared, preserving
// declared's order. An empty requested selects every declared id.
func selectDeclaredTargets(declared []string, requested []string) ([]string, error) {
	if len(requested) == 0 {
		out := make([]string, len(declared))
		copy(out, declared)
		return out, nil
	}
	known := make(map[string]bool, len(declared))
	for _, id := range declared {
		known[id] = true
	}
	want := make(map[string]bool, len(requested))
	for _, id := range requested {
		if !known[id] {
			return nil, &UnknownHarnessTargetError{ID: id}
		}
		want[id] = true
	}
	out := make([]string, 0, len(want))
	for _, id := range declared {
		if want[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// claudeSettingsDisplayPath returns the display path for Claude Code's settings.json.
// When the resolved path is under homedir.Dir(), it is abbreviated with "~/"
// (e.g., "~/.claude/settings.json" in the control case, or a custom subpath
// when CLAUDE_CONFIG_DIR points inside the home directory).
// When the resolved path is outside homedir.Dir() (e.g., CLAUDE_CONFIG_DIR=/tmp/…,
// or USERPROFILE ≠ HOME on Windows), the absolute path is returned as-is.
// Falls back to "~/.claude/settings.json" when claudeDir is empty (error path).
//
// ML-1B (REQ-2026-09-01): replaces the fixed const displayPath = "~/.claude/settings.json"
// in harnessCredentialGuardTargetClaude and harnessGitBranchGuardTargetClaude so that
// CLAUDE_CONFIG_DIR and Windows USERPROFILE divergence are reflected in the output.
func claudeSettingsDisplayPath(claudeDir string) string {
	if claudeDir == "" {
		return "~/.claude/settings.json"
	}
	path := filepath.Join(claudeDir, "settings.json")
	home, err := homedir.Dir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return "~/" + filepath.ToSlash(rel)
}

// withD5Notice appends the D5 inert-remainder notice to a TargetUpdated or
// TargetSkipped result when HOME/.claude/settings.json exists and contains
// trackfw guard wiring but claudeDir points elsewhere (ADR-2026-10-14 D5,
// ML-1B REQ-2026-09-01).
//
// For TargetFailed / TargetMissing results the function is a no-op — the
// D5 notice is informational only and must not obscure error/missing states.
//
// R-3 (ML-2D): the message is conditional on the origin of claudeDir and the
// presence of Windows HOME≠USERPROFILE divergence:
//
//   - Origin CLAUDE_CONFIG_DIR, no Windows divergence: the guard at HOME/.claude
//     is inert only for Claude Code instances that also run with CLAUDE_CONFIG_DIR.
//     Without the variable, HOME/.claude IS the active guard — the notice warns
//     without instructing the user to delete it.
//   - Windows divergence (HOME≠USERPROFILE) regardless of CLAUDE_CONFIG_DIR:
//     Claude Code reads USERPROFILE/.claude (or CLAUDE_CONFIG_DIR), so HOME/.claude
//     is unconditionally inert. The existing "remove it by hand" message applies.
func withD5Notice(result TargetResult, claudeDir string) TargetResult {
	if result.State != TargetUpdated && result.State != TargetSkipped {
		return result
	}
	inertPath, present := agentconfig.InertClaudeGuardPath(claudeDir)
	if !present {
		return result
	}
	// R-3 discriminant: conditional message only when CLAUDE_CONFIG_DIR drove resolution
	// AND there is no Windows HOME≠USERPROFILE divergence. With divergence, HOME/.claude
	// is unconditionally inert (Claude reads USERPROFILE/.claude or CLAUDE_CONFIG_DIR,
	// neither of which is HOME/.claude).
	envDir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	_, _, divergent := agentconfig.HomeDivergencePaths()
	if envDir != "" && !divergent {
		result.Message = fmt.Sprintf(
			"guard under HOME at %s is inert only for a Claude Code that sees CLAUDE_CONFIG_DIR=%s; "+
				"if Claude Code runs without CLAUDE_CONFIG_DIR, that file is the active one — do not remove it",
			inertPath, filepath.Clean(envDir),
		)
	} else {
		result.Message = fmt.Sprintf(
			"old guard under HOME is inert — Claude Code does not read %s; remove it by hand if you want",
			inertPath,
		)
	}
	return result
}

// rejectHarnessSymlink checks path for symlinks in its ancestry (up to and
// including home) before any filesystem mutation. It returns a TargetFailed
// result and true if the guard fires, or TargetResult{} and false when the
// path is clean. The refusal is written to stderr (ADR-2026-09-18, decision 3:
// recusa audível — writes that would escape $HOME must never be silent).
func rejectHarnessSymlink(home, path, id, displayPath string) (TargetResult, bool) {
	if guardErr := pathguard.RejectAndReport(home, path); guardErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: guardErr.Error()}, true
	}
	return TargetResult{}, false
}

// harnessGitBranchGuardScriptTarget evaluates (and, unless DryRun, applies)
// the global git-branch-guard shell script at
// ~/.trackfw/scripts/trackfw-git-branch-guard.sh.
//
// Unlike wiring targets (e.g. claude-git-branch-guard), this target OWNS the
// script file itself. It compares content before writing so that an already
// up-to-date script is reported as "skipped" and its mtime is left unchanged
// (REQ-2026-09-09 AC3). Creation does not require --install-missing because
// ~/.trackfw/scripts/ is 100% trackfw-owned (unlike ~/.claude/settings.json).
func harnessGitBranchGuardScriptTarget(home string, opts UpdateOptions) TargetResult {
	const id = "git-branch-guard-script"
	const displayPath = "~/.trackfw/scripts/trackfw-git-branch-guard.sh"

	path := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	desired := []byte(gitBranchGuardScript)

	data, err := os.ReadFile(path)
	if err == nil && bytes.Equal(data, desired) {
		// Content identical — skip write, repair execute bit without touching mtime.
		_ = os.Chmod(path, 0755)
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	// File is missing or outdated — write it.
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0755); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessCredentialGuardScriptTarget evaluates (and, unless DryRun, applies)
// the global credential-guard shell script at
// ~/.trackfw/scripts/trackfw-credential-guard.sh.
//
// Mirrors harnessGitBranchGuardScriptTarget — same ownership model, same
// idempotency contract, same absence of --install-missing gate.
func harnessCredentialGuardScriptTarget(home string, opts UpdateOptions) TargetResult {
	const id = "credential-guard-script"
	const displayPath = "~/.trackfw/scripts/trackfw-credential-guard.sh"

	path := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	desired := []byte(globalCredentialGuardScript)

	data, err := os.ReadFile(path)
	if err == nil && bytes.Equal(data, desired) {
		// Content identical — skip write, repair execute bit without touching mtime.
		_ = os.Chmod(path, 0755)
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	// File is missing or outdated — write it.
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0755); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessClaudeSkillTarget evaluates (and, unless DryRun, applies) the
// historical global Claude compatibility skill.
//
// ML-2B (REQ-2026-09-01, B2): claudeDir is the resolved Claude config directory
// (honours CLAUDE_CONFIG_DIR and, on Windows, USERPROFILE). claudeDirErr != nil
// means resolution failed — treated as TargetFailed so the harness reports the
// problem rather than silently writing to the wrong path (ADR-2026-10-14 A7).
// home stays as the trackfw artefacts root (~/.trackfw/).
func harnessClaudeSkillTarget(home, claudeDir string, claudeDirErr error, opts UpdateOptions) TargetResult {
	const id = "claude-skill"
	const displayPath = "~/.claude/skills/trackfw/SKILL.md"

	if claudeDirErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: fmt.Sprintf("resolving Claude config dir: %s", claudeDirErr)}
	}
	path := filepath.Join(claudeDir, "skills", "trackfw", "SKILL.md")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B). Root is filepath.Dir(claudeDir) — checks
	// claudeDir itself (e.g. ~/.claude) for symlink, same depth as the
	// credential and git-branch guard targets.
	if result, guarded := rejectHarnessSymlink(filepath.Dir(claudeDir), path, id, displayPath); guarded {
		return result
	}
	desired := GlobalClaudeSkillContent()

	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	if string(data) == string(desired) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessCredentialGuardTargetClaude evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Claude Code:
// PreToolUse[matcher:"Bash"]/PostToolUse[matcher:"Bash"] entries in
// ~/.claude/settings.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh. This must be an absolute
// path (unlike InjectClaudeHooks's project-scope "scripts/trackfw-credential-
// guard.sh", which is relative because the hook always runs from the project
// root) since a global hook can fire from any project's cwd. Reuses
// mergeClaudeHookArray (agentfiles.go) — the same idempotent merge helper
// InjectClaudeHooks/InjectCodexHooks/InjectGeminiHooks already use — so any
// pre-existing content in ~/.claude/settings.json (other hooks, user config)
// is preserved; only the credential-guard entry is added/merged.
// harnessCredentialGuardTargetClaude evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Claude Code.
//
// ML-1A (REQ-2026-09-01): claudeDir is the resolved Claude config directory
// (honours CLAUDE_CONFIG_DIR and, on Windows, USERPROFILE). claudeDirErr != nil
// means resolution failed (USERPROFILE absent, etc.) — treated as TargetFailed so
// the harness reports the problem rather than silently writing to the wrong path.
// home stays as the trackfw artefacts root (~/.trackfw/).
func harnessCredentialGuardTargetClaude(home, claudeDir string, claudeDirErr error, opts UpdateOptions) TargetResult {
	const id = "claude-credential-guard"
	// ML-1B (REQ-2026-09-01): displayPath is now dynamic — shows the real resolved path.
	// In the control case (HOME == USERPROFILE, no CLAUDE_CONFIG_DIR) this produces
	// "~/.claude/settings.json", identical to the previous hardcoded value.
	displayPath := claudeSettingsDisplayPath(claudeDir)
	if claudeDirErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: "~/.claude/settings.json", Message: fmt.Sprintf("resolving Claude config dir: %s", claudeDirErr)}
	}
	path := filepath.Join(claudeDir, "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B). Root is filepath.Dir(claudeDir) — this checks
	// claudeDir itself (e.g. ~/.claude) for symlink, preserving the same depth
	// as before. In the control case (HOME == USERPROFILE) Dir(claudeDir) == home.
	if result, guarded := rejectHarnessSymlink(filepath.Dir(claudeDir), path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
		}
		root := make(map[string]interface{})
		mergeCredentialGuardClaudeHooks(root, claudeShellMatcher, guardCredentialGlobalCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-5A: also migrate the "Bash" matcher → claudeShellMatcher for guard entries.
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline forms → D11 revised forms.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyScriptPath, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyScriptPath, guardCredentialGlobalCmdPSPOSIX)
		for _, matcher := range []string{claudeShellMatcher, "Bash"} {
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		}
		migrateGuardHookMatcher(hooks["PreToolUse"], "Bash", claudeShellMatcher, guardCredentialGlobalCmdPSPOSIX)
		migrateGuardHookMatcher(hooks["PostToolUse"], "Bash", claudeShellMatcher, guardCredentialGlobalCmdPSPOSIX)
		// D13 dedup (REQ-2026-10-23): collapse duplicates produced by multiple
		// migrateHookCommand calls (home-dependent legacyScriptPath + registry forms).
		// Pass legacyScriptPath at the call site — it cannot live in guardforms (home-dependent).
		globalLegacyForms := append(append([]string{}, guardforms.CredentialGlobalLegacyPSPOSIX...), legacyScriptPath)
		for _, m := range []string{claudeShellMatcher, "Bash"} {
			hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], m, guardCredentialGlobalCmdPSPOSIX, globalLegacyForms)
			hooks["PostToolUse"] = migrateDedupGuardInNestedArray(hooks["PostToolUse"], m, guardCredentialGlobalCmdPSPOSIX, globalLegacyForms)
		}
	}
	mergeCredentialGuardClaudeHooks(root, claudeShellMatcher, guardCredentialGlobalCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return withD5Notice(TargetResult{ID: id, State: TargetSkipped, Path: displayPath}, claudeDir)
	}
	if opts.DryRun {
		return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
}

// mergeCredentialGuardClaudeHooks merges the guard PreToolUse/PostToolUse[matcher]
// entries into root["hooks"], preserving any other hook groups/matchers already
// present (same merge contract as InjectClaudeHooks, minus the attention-signal/
// cleanup entries which stay project-scope only). Despite the name (kept for
// git-blame continuity with ML-2A), this helper is shape-agnostic — it only
// touches root["hooks"]["PreToolUse"/"PostToolUse"] with the given matcher.
//
// The matcher parameter controls which tool name is intercepted:
//   - Claude Code callers (harnessCredentialGuardTargetClaude,
//     harnessGitBranchGuardTargetClaude): pass claudeShellMatcher
//     ("Bash|PowerShell") so the guard fires on both POSIX and Windows
//     (ML-5A / REQ-2026-09-05).
//   - Codex callers (harnessCredentialGuardTargetCodex,
//     harnessGitBranchGuardTargetCodex): pass "Bash" — Codex's tool
//     name does not vary by platform.
func mergeCredentialGuardClaudeHooks(root map[string]interface{}, matcher, scriptPath string) {
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], matcher, scriptPath)
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], matcher, scriptPath)
	root["hooks"] = hooks
}

// harnessCredentialGuardTargetCodex evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Codex CLI:
// PreToolUse[matcher:"Bash"]/PostToolUse[matcher:"Bash"] entries in
// ~/.codex/hooks.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh — mirrors
// harnessCredentialGuardTargetClaude exactly (same 4-state contract, same
// idempotent merge via mergeCredentialGuardClaudeHooks, same reason for an
// absolute path over InjectCodexHooks's project-relative
// "scripts/trackfw-credential-guard.sh": a global hook can fire from any
// project's cwd).
//
// Investigation (ROADMAP-2026-08-06 Wave 2/ML-2B, confirmed 2026-08-06
// against https://developers.openai.com/codex/hooks): "Hooks are enabled by
// default. To turn them off in config.toml, set: [features] hooks = false.
// Use hooks as the canonical feature key. codex_hooks still works as a
// deprecated alias." This resolves the contradiction the ADR flagged with
// high confidence — no `[features] codex_hooks = true` opt-in is required
// (the flag exists only to turn hooks OFF, and is a deprecated alias for
// the canonical `hooks` key); https://developers.openai.com/codex/config-
// advanced (also fetched 2026-08-06) has no conflicting requirement. No
// extra warning Message is added to the TargetResult because of this: the
// investigation resolved with confidence, so per the ML's own instructions
// (fall back to a warning only "se não conseguir resolver a contradição com
// confiança total") no hedge is needed.
func harnessCredentialGuardTargetCodex(home string, opts UpdateOptions) TargetResult {
	const id = "codex-credential-guard"
	const displayPath = "~/.codex/hooks.json"

	path := filepath.Join(home, ".codex", "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardClaudeHooks(root, "Bash", guardCredentialGlobalCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyScriptPath, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyScriptPath, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
	}
	// D13 dedup (REQ-2026-10-23): after individual migrations, collapse any
	// remaining duplicates (e.g. two legacy forms simultaneously present → two
	// canonicals after migration → deduplicated to exactly one).
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		legacies := append(append([]string{}, guardforms.CredentialGlobalLegacyPSPOSIX...), legacyScriptPath)
		hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], "Bash", guardCredentialGlobalCmdPSPOSIX, legacies)
		hooks["PostToolUse"] = migrateDedupGuardInNestedArray(hooks["PostToolUse"], "Bash", guardCredentialGlobalCmdPSPOSIX, legacies)
		root["hooks"] = hooks
	}
	mergeCredentialGuardClaudeHooks(root, "Bash", guardCredentialGlobalCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// mergeCredentialGuardGeminiHooks merges the credential-guard BeforeTool/
// AfterTool[matcher:"run_shell_command"] entries into root["hooks"],
// preserving any other hook groups/matchers already present. Gemini CLI uses
// different top-level event names than Claude/Codex (BeforeTool/AfterTool
// instead of PreToolUse/PostToolUse — see InjectGeminiHooks, agentfiles.go,
// confirmed against https://geminicli.com/docs/hooks/reference), but the
// per-entry array shape ({"matcher","hooks":[{"type","command"}]}) is
// identical, so mergeClaudeHookArray is reused verbatim — only the top-level
// key names and matcher value differ from mergeCredentialGuardClaudeHooks.
func mergeCredentialGuardGeminiHooks(root map[string]interface{}, scriptPath string) {
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}
	hooks["BeforeTool"] = mergeClaudeHookArray(hooks["BeforeTool"], "run_shell_command", scriptPath)
	hooks["AfterTool"] = mergeClaudeHookArray(hooks["AfterTool"], "run_shell_command", scriptPath)
	root["hooks"] = hooks
}

// harnessCredentialGuardTargetGemini evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Gemini CLI:
// BeforeTool[matcher:"run_shell_command"]/AfterTool[matcher:"run_shell_command"]
// entries in ~/.gemini/settings.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh — mirrors
// harnessCredentialGuardTargetClaude/harnessCredentialGuardTargetCodex
// exactly (same 4-state contract, same idempotent merge, now via
// mergeCredentialGuardGeminiHooks since the event key names differ from
// Claude/Codex's PreToolUse/PostToolUse), same reason for an absolute path
// over InjectGeminiHooks's project-relative
// "scripts/trackfw-credential-guard.sh": a global hook can fire from any
// project's cwd.
func harnessCredentialGuardTargetGemini(home string, opts UpdateOptions) TargetResult {
	const id = "gemini-credential-guard"
	const displayPath = "~/.gemini/settings.json"

	path := filepath.Join(home, ".gemini", "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardGeminiHooks(root, guardCredentialGlobalCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyScriptPath, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyScriptPath, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX)
	}
	// D13 dedup (REQ-2026-10-23): collapse duplicates after individual migrations.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		legacies := append(append([]string{}, guardforms.CredentialGlobalLegacyPSPOSIX...), legacyScriptPath)
		hooks["BeforeTool"] = migrateDedupGuardInNestedArray(hooks["BeforeTool"], "run_shell_command", guardCredentialGlobalCmdPSPOSIX, legacies)
		hooks["AfterTool"] = migrateDedupGuardInNestedArray(hooks["AfterTool"], "run_shell_command", guardCredentialGlobalCmdPSPOSIX, legacies)
		root["hooks"] = hooks
	}
	mergeCredentialGuardGeminiHooks(root, guardCredentialGlobalCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// mergeCredentialGuardCursorHooks merges the credential-guard
// beforeShellExecution/afterShellExecution entries into root["hooks"],
// preserving any other hook event arrays already present. Cursor's
// hooks.json schema (confirmed by InjectCursorHooks, agentfiles.go) is
// structurally different from Claude/Codex/Gemini's: each event holds a flat
// array of simple {"command": "..."} objects — no per-entry "matcher", no
// nested {"type","hooks":[...]} — so this reuses mergeSimpleCommandArray
// (agentfiles.go), not mergeClaudeHookArray.
func mergeCredentialGuardCursorHooks(root map[string]interface{}, scriptPath string) {
	if _, ok := root["version"]; !ok {
		root["version"] = 1
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
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
	hooks["beforeShellExecution"] = mergeSimpleCommandArray(hooks["beforeShellExecution"], scriptPath, makeEntry, getCmd)
	hooks["afterShellExecution"] = mergeSimpleCommandArray(hooks["afterShellExecution"], scriptPath, makeEntry, getCmd)
	root["hooks"] = hooks
}

// harnessCredentialGuardTargetCursor evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Cursor:
// hooks.beforeShellExecution/hooks.afterShellExecution entries in
// ~/.cursor/hooks.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh — same 4-state contract and
// same reason for an absolute path as
// harnessCredentialGuardTargetClaude/Codex/Gemini (a global hook can fire
// from any project's cwd), but via mergeCredentialGuardCursorHooks since
// Cursor's hooks.json schema (ROADMAP-2026-08-06 ML-2D) differs from the
// other three CLIs — see that helper's doc comment.
func harnessCredentialGuardTargetCursor(home string, opts UpdateOptions) TargetResult {
	const id = "cursor-credential-guard"
	const displayPath = "~/.cursor/hooks.json"

	path := filepath.Join(home, ".cursor", "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	cursorGetCmd := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		cmd, _ := obj["command"].(string)
		return cmd
	}

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardCursorHooks(root, guardCredentialGlobalCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyScriptPath, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyScriptPath, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD11RevCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyWABashCredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD2CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD11CredentialGlobalCmdPSPOSIX, guardCredentialGlobalCmdPSPOSIX, cursorGetCmd)
	}
	// D13 dedup (REQ-2026-10-23): collapse duplicates after individual migrations.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		legacies := append(append([]string{}, guardforms.CredentialGlobalLegacyPSPOSIX...), legacyScriptPath)
		hooks["beforeShellExecution"] = dedupFlatGuardEntries(hooks["beforeShellExecution"], "", guardCredentialGlobalCmdPSPOSIX, legacies, cursorGetCmd)
		hooks["afterShellExecution"] = dedupFlatGuardEntries(hooks["afterShellExecution"], "", guardCredentialGlobalCmdPSPOSIX, legacies, cursorGetCmd)
		root["hooks"] = hooks
	}
	mergeCredentialGuardCursorHooks(root, guardCredentialGlobalCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// mergeCredentialGuardCopilotHooks merges the credential-guard
// preToolUse/postToolUse[matcher:"bash"] entries into root["hooks"],
// preserving any other hook groups/entries already present.
//
// Investigation (ROADMAP-2026-08-06 Wave 2/ML-2E, confirmed 2026-08-06
// against https://docs.github.com/en/copilot/reference/hooks-reference,
// section "Hooks locations"): the user-level scope offers TWO distinct
// mechanisms — (a) a dedicated directory of standalone hook files,
// "*.json files in the user-level hooks directory... by default
// ~/.copilot/hooks/", and (b) "Inline hooks block in user-level config — the
// hooks field at the top level of ~/.copilot/settings.json". This ML follows
// the roadmap's explicit instruction to target ~/.copilot/settings.json
// (option b), which the doc confirms is NOT a dedicated hooks file: it is
// Copilot CLI's general user config file (also holds "model", other CLI
// settings), unlike .github/hooks/trackfw-attention.json (project scope,
// InjectCopilotHooks) which is safely overwritten wholesale. So, exactly like
// harnessCredentialGuardTargetClaude/Codex/Gemini's own general settings
// files, this merges into root["hooks"] only and never overwrites/replaces
// any other top-level key.
//
// Entry shape: the doc states "Hook configuration files use JSON format with
// version 1" without carving out an exception for the inline hooks field —
// i.e. the same command-entry shape documented for standalone hook files
// ({"type":"command","bash":"...","cwd":"...","timeoutSec":N}, with an
// optional "matcher") applies here too, identical to what InjectCopilotHooks
// (agentfiles.go, project scope) already emits. This function does NOT add a
// top-level "version" key to root, though: that field belongs to dedicated
// hooks-file format examples in the doc (e.g. the standalone
// {"version":1,"hooks":{...}} shown for .github/hooks/*.json and policy
// files) — nothing in the doc shows or requires a "version" key at the root
// of the general settings.json file itself, so adding one here would be an
// unconfirmed assumption about a file this code does not own.
//
// Merge mechanics: reuses mergeSimpleCommandArray (agentfiles.go) with a
// custom getCmd/makeEntry pair since the match key is "bash" (unlike
// Cursor's flat {"command":"..."} shape, matched on "command") and the full
// entry also carries "type"/"matcher"/"cwd"/"timeoutSec", mirroring the
// project-scope entries InjectCopilotHooks writes for credential-guard.
func mergeCredentialGuardCopilotHooks(root map[string]interface{}, scriptPath string) {
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}
	makeEntry := func(command string) interface{} {
		return map[string]interface{}{
			"type":       "command",
			"matcher":    "bash",
			"bash":       command,
			"cwd":        ".",
			"timeoutSec": 10,
		}
	}
	getCmd := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		bash, _ := obj["bash"].(string)
		return bash
	}
	hooks["preToolUse"] = mergeSimpleCommandArray(hooks["preToolUse"], scriptPath, makeEntry, getCmd)
	hooks["postToolUse"] = mergeSimpleCommandArray(hooks["postToolUse"], scriptPath, makeEntry, getCmd)
	root["hooks"] = hooks
}

// harnessCredentialGuardTargetCopilot evaluates (and, unless DryRun,
// applies) the global-scope credential-guard hook wiring for GitHub Copilot:
// hooks.preToolUse/hooks.postToolUse[matcher:"bash"] entries in
// ~/.copilot/settings.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh — same 4-state contract and
// same reason for an absolute path as
// harnessCredentialGuardTargetClaude/Codex/Gemini/Cursor (a global hook can
// fire from any project's cwd), but via mergeCredentialGuardCopilotHooks
// since Copilot's command-hook entry shape (ROADMAP-2026-08-06 ML-2E) is
// its own — see that helper's doc comment for the full format investigation.
func harnessCredentialGuardTargetCopilot(home string, opts UpdateOptions) TargetResult {
	const id = "copilot-credential-guard"
	const displayPath = "~/.copilot/settings.json"

	path := filepath.Join(home, ".copilot", "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardCopilotHooks(root, guardCredentialGlobalCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	// Copilot entries use "bash" key (not "command"), so inline migration is needed.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		for _, eventKey := range []string{"preToolUse", "postToolUse"} {
			arr, _ := hooks[eventKey].([]interface{})
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if bash, _ := obj["bash"].(string); bash == legacyScriptPath {
					obj["bash"] = guardCredentialGlobalCmdPSPOSIX
				} else if bash == legacyD11RevCredentialGlobalCmdPSPOSIX {
					obj["bash"] = guardCredentialGlobalCmdPSPOSIX
				} else if bash == legacyWABashCredentialGlobalCmdPSPOSIX {
					obj["bash"] = guardCredentialGlobalCmdPSPOSIX
				} else if bash == legacyD2CredentialGlobalCmdPSPOSIX {
					obj["bash"] = guardCredentialGlobalCmdPSPOSIX
				} else if bash == legacyD11CredentialGlobalCmdPSPOSIX {
					obj["bash"] = guardCredentialGlobalCmdPSPOSIX
				}
			}
		}
	}
	// D13 dedup (REQ-2026-10-23): Copilot entries use "bash" key (not "command"),
	// so the generic dedupFlatGuardEntries helper is not used here. Inline dedup:
	// pass 1 migrates any remaining legacies (those not covered by the switch
	// above), pass 2 keeps only the first canonical entry per event.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		copilotAllForms := make(map[string]bool, len(guardforms.CredentialGlobalLegacyPSPOSIX)+2)
		for _, f := range guardforms.CredentialGlobalLegacyPSPOSIX {
			copilotAllForms[f] = true
		}
		copilotAllForms[legacyScriptPath] = true
		copilotAllForms[guardCredentialGlobalCmdPSPOSIX] = true
		for _, eventKey := range []string{"preToolUse", "postToolUse"} {
			arr, _ := hooks[eventKey].([]interface{})
			// Pass 1: rewrite any remaining legacy bash values to canonical.
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				bash, _ := obj["bash"].(string)
				if copilotAllForms[bash] && bash != guardCredentialGlobalCmdPSPOSIX {
					obj["bash"] = guardCredentialGlobalCmdPSPOSIX
				}
			}
			// Pass 2: keep first canonical entry, drop subsequent duplicates.
			seen := false
			var deduped []interface{}
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					deduped = append(deduped, item)
					continue
				}
				bash, _ := obj["bash"].(string)
				if bash == guardCredentialGlobalCmdPSPOSIX {
					if !seen {
						seen = true
						deduped = append(deduped, item)
					}
					// else: drop duplicate.
				} else {
					deduped = append(deduped, item)
				}
			}
			if deduped != nil {
				hooks[eventKey] = deduped
			}
		}
		root["hooks"] = hooks
	}
	mergeCredentialGuardCopilotHooks(root, guardCredentialGlobalCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessCredentialGuardTargetWindsurf evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Windsurf at
// ~/.codeium/windsurf/hooks.json. This is the standard Windsurf user-scope
// hooks file (same schema as the project-scope .windsurf/hooks.json, which
// InjectWindsurfHooks manages). Uses mergeSimpleCommandArray — the same
// idempotent merge InjectWindsurfHooks uses for project-scope files — so any
// pre-existing entries from other tools or a prior trackfw run are preserved.
//
// Two events are wired with guardCredentialGlobalCmdPSPOSIX (PS/POSIX family,
// same as every other global credential-guard target):
//   - pre_run_command: guards shell commands before execution (Layer 1 + gap R1).
//   - pre_write_code: guards file writes before content is written (Layer 1).
//
// Note: there is NO windsurf-git-branch-guard global target. Windsurf's
// documented global hook path is ~/.codeium/windsurf/hooks.json, which this
// target owns. Adding a separate git-branch-guard global target would require
// a second schema for the same file or a wholesale-rewrite approach — neither
// has been confirmed against official documentation (R4-equivalent). Declared
// as an open gap (ML-1B, REQ-2026-10-06).
func harnessCredentialGuardTargetWindsurf(home string, opts UpdateOptions) TargetResult {
	const id = "windsurf-credential-guard"
	const displayPath = "~/.codeium/windsurf/hooks.json"

	path := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
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

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		hooks := make(map[string]interface{})
		for _, event := range []string{"pre_run_command", "pre_write_code"} {
			hooks[event] = mergeSimpleCommandArray(
				hooks[event], guardCredentialGlobalCmdPSPOSIX, windsurfMakeEntry, windsurfGetCmd,
			)
		}
		root["hooks"] = hooks
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}
	// D13 dedup (REQ-2026-10-23): migrate ALL global legacy POSIX forms and
	// collapse duplicates to exactly one canonical entry per event.
	// Replaces the prior single-form migrateCursorSimpleCommand(D11Rev...) call.
	windsurfLegaciesGlobal := append(append([]string{}, guardforms.CredentialGlobalLegacyPSPOSIX...),
		filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh"))
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		hooks[event] = dedupFlatGuardEntries(hooks[event], "", guardCredentialGlobalCmdPSPOSIX, windsurfLegaciesGlobal, windsurfGetCmd)
	}
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		hooks[event] = mergeSimpleCommandArray(
			hooks[event], guardCredentialGlobalCmdPSPOSIX, windsurfMakeEntry, windsurfGetCmd,
		)
	}
	root["hooks"] = hooks

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessCredentialGuardTargetKiro evaluates (and, unless DryRun, applies)
// the global-scope credential-guard hook wiring for Kiro: a dedicated file
// at ~/.kiro/hooks/trackfw-credential-guard.json (NOT a merge into a shared
// settings file — unlike Claude/Codex/Gemini/Copilot's general settings
// files, ~/.kiro/hooks/ is a directory of one-file-per-hook, confirmed by
// InjectKiroHooks's own investigation and by kiro.dev/changelog/cli/2-13/:
// "Hooks placed in ~/.kiro/hooks/ now fire in every workspace automatically
// ... Workspace-level hooks continue to work alongside global ones"). Same
// schema as InjectKiroHooks (project scope, agentfiles.go): top-level
// {"version":"v1","hooks":[...]}, each entry {"name","description",
// "trigger","matcher","action":{"type":"command","command":<absolute path>}}
// — but the command here is the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh (a global hook can fire
// from any project's cwd, unlike the project-scope wiring's relative
// "scripts/trackfw-credential-guard.sh"), and the two hook names are
// "trackfw-credential-guard-global-pre"/"-global-post" — deliberately
// DISTINCT from the project-scope names ("trackfw-credential-guard-pre"/
// "-post") rather than reused, because this writes an entirely different
// file (~/.kiro/hooks/ vs <project>/.kiro/hooks/) and nothing in the
// changelog's "workspace hooks continue to work alongside global ones"
// documents whether Kiro deduplicates same-named hooks originating from
// different files/scopes — distinct names avoid betting on unconfirmed
// merge-by-name behavior; ML-3A's future project-scope dedup will match on
// the script path, not the hook name, same as every other tool's dedup.
//
// Kiro v3 caveat (ROADMAP-2026-08-06 Wave 2/ML-2F investigation, confirmed
// 2026-08-06 against kiro.dev/changelog/cli/2-13/): global hooks are
// "Available in V3 (`kiro-cli --v3`)". Re-fetching that page found `--v3` is
// a LAUNCH-MODE flag on the same installed binary ("kiro-cli --v3"), not a
// value any `kiro-cli --version`/`kiro --version` style command reports —
// the doc/marketing pages fetched for this ML (kiro.dev/docs/cli/) document
// no such flag at all. There is therefore no persistent, installed-version
// fact to probe from a separate process (trackfw never invokes Kiro
// itself): whether a given Kiro session honors this file depends on how the
// USER launches their next session, not on anything on disk right now. This
// target intentionally does NOT attempt a `kiro`/`kiro-cli` subprocess
// version probe (per the roadmap's fallback instruction for "not detectable
// in a confiable way"), and does NOT put the caveat in TargetResult.Message
// either: the pinned JSON contract (docs/cli-parity.md, "message" only on
// "failed") and TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract
// both establish Message is failure-only, so inventing a message on
// "updated" would break that contract. The v3 prerequisite is documented
// instead in this comment and in docs/cli-parity.md's own "Kiro global-scope
// wiring (ML-2F)" section, both of which the release notes/changelog can
// point users to — the same choice already made for Copilot's own doc-only
// caveats (see harnessCredentialGuardTargetCopilot above).
func harnessCredentialGuardTargetKiro(home string, opts UpdateOptions) TargetResult {
	const id = "kiro-credential-guard"
	const displayPath = "~/.kiro/hooks/trackfw-credential-guard.json"

	path := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	// Kiro/Amazon Q are CmdExe-family: no "exit $LASTEXITCODE" suffix.
	content := map[string]interface{}{
		"version": "v1",
		"hooks": []interface{}{
			map[string]interface{}{
				"name":        "trackfw-credential-guard-global-pre",
				"description": "Blocks/warns on possible plaintext credential materialization before a shell command executes (global, all projects)",
				"trigger":     "PreToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdCmdExe},
			},
			map[string]interface{}{
				"name":        "trackfw-credential-guard-global-post",
				"description": "Warns on possible plaintext credential materialization after a shell command executes (global, all projects)",
				"trigger":     "PostToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdCmdExe},
			},
		},
	}
	out, marshalErr := marshalJSONNoEscape(content)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')

	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	if string(data) == string(desired) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// ────────────────────────────────────────────────────────────────────────────
// git-branch-guard global-scope wiring (ROADMAP-2026-08-17 Wave 2/ML-2A).
// Mirrors the credential-guard targets immediately above, entry-for-entry —
// same 4-state contract, same displayPath/scriptPath shape, same reuse of
// mergeCredentialGuardClaudeHooks/...GeminiHooks/...CursorHooks/
// ...CopilotHooks (those helpers are shape-agnostic despite the name, see
// their own doc comments — they only need a scriptPath, which here is
// trackfw-git-branch-guard.sh instead of trackfw-credential-guard.sh). The
// only structural difference from the credential-guard set is Kiro: its
// credential-guard writer owns ~/.kiro/hooks/trackfw-credential-guard.json
// WHOLESALE (rewrites the entire document, never merges — see
// harnessCredentialGuardTargetKiro above), so sharing that file between two
// wholesale writers would make them flap between each other's desired state
// every run (an idempotency failure). git-branch-guard therefore gets its
// OWN dedicated file, ~/.kiro/hooks/trackfw-git-branch-guard.json, same
// {"version":"v1","hooks":[pre,post]} schema, hook names
// "trackfw-git-branch-guard-global-pre"/"-global-post".
// ────────────────────────────────────────────────────────────────────────────

// harnessGitBranchGuardTargetClaude wires the global-scope git-branch-guard
// hook for Claude Code into ~/.claude/settings.json, mirroring
// harnessCredentialGuardTargetClaude exactly (merges into the SAME
// hooks.PreToolUse/PostToolUse[matcher:"Bash"] arrays credential-guard also
// merges into — mergeClaudeHookArray appends a second, distinct command
// entry rather than overwriting the first).
//
// ML-1A (REQ-2026-09-01): claudeDir/claudeDirErr — same contract as
// harnessCredentialGuardTargetClaude. home stays as the trackfw artefacts root.
func harnessGitBranchGuardTargetClaude(home, claudeDir string, claudeDirErr error, opts UpdateOptions) TargetResult {
	const id = "claude-git-branch-guard"
	// ML-1B (REQ-2026-09-01): displayPath is now dynamic — shows the real resolved path.
	// In the control case (HOME == USERPROFILE, no CLAUDE_CONFIG_DIR) this produces
	// "~/.claude/settings.json", identical to the previous hardcoded value.
	displayPath := claudeSettingsDisplayPath(claudeDir)
	if claudeDirErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: "~/.claude/settings.json", Message: fmt.Sprintf("resolving Claude config dir: %s", claudeDirErr)}
	}
	path := filepath.Join(claudeDir, "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B). Root is filepath.Dir(claudeDir) — checks claudeDir
	// itself for symlink; in control case (HOME==USERPROFILE) equals home.
	if result, guarded := rejectHarnessSymlink(filepath.Dir(claudeDir), path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
		}
		root := make(map[string]interface{})
		mergeCredentialGuardClaudeHooks(root, claudeShellMatcher, guardGitBranchCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-5A: also migrate the "Bash" matcher → claudeShellMatcher for guard entries.
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline forms → D11 revised forms.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyScriptPath, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyScriptPath, guardGitBranchCmdPSPOSIX)
		for _, matcher := range []string{claudeShellMatcher, "Bash"} {
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PreToolUse"], matcher, legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
			migrateHookCommand(hooks["PostToolUse"], matcher, legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		}
		migrateGuardHookMatcher(hooks["PreToolUse"], "Bash", claudeShellMatcher, guardGitBranchCmdPSPOSIX)
		migrateGuardHookMatcher(hooks["PostToolUse"], "Bash", claudeShellMatcher, guardGitBranchCmdPSPOSIX)
		// D13 dedup (REQ-2026-10-23): collapse duplicates from multiple migrateHookCommand calls.
		gitLegacyForms := append(append([]string{}, guardforms.GitBranchLegacyPSPOSIX...), legacyScriptPath)
		for _, m := range []string{claudeShellMatcher, "Bash"} {
			hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], m, guardGitBranchCmdPSPOSIX, gitLegacyForms)
			hooks["PostToolUse"] = migrateDedupGuardInNestedArray(hooks["PostToolUse"], m, guardGitBranchCmdPSPOSIX, gitLegacyForms)
		}
	}
	mergeCredentialGuardClaudeHooks(root, claudeShellMatcher, guardGitBranchCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return withD5Notice(TargetResult{ID: id, State: TargetSkipped, Path: displayPath}, claudeDir)
	}
	if opts.DryRun {
		return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return withD5Notice(TargetResult{ID: id, State: TargetUpdated, Path: displayPath}, claudeDir)
}

// harnessGitBranchGuardTargetCodex wires the global-scope git-branch-guard
// hook for Codex CLI into ~/.codex/hooks.json, mirroring
// harnessCredentialGuardTargetCodex/harnessGitBranchGuardTargetClaude.
func harnessGitBranchGuardTargetCodex(home string, opts UpdateOptions) TargetResult {
	const id = "codex-git-branch-guard"
	const displayPath = "~/.codex/hooks.json"

	path := filepath.Join(home, ".codex", "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardClaudeHooks(root, "Bash", guardGitBranchCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyScriptPath, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyScriptPath, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PreToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["PostToolUse"], "Bash", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	}
	// D13 dedup (REQ-2026-10-23): collapse duplicates after individual migrations.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		legacies := append(append([]string{}, guardforms.GitBranchLegacyPSPOSIX...), legacyScriptPath)
		hooks["PreToolUse"] = migrateDedupGuardInNestedArray(hooks["PreToolUse"], "Bash", guardGitBranchCmdPSPOSIX, legacies)
		hooks["PostToolUse"] = migrateDedupGuardInNestedArray(hooks["PostToolUse"], "Bash", guardGitBranchCmdPSPOSIX, legacies)
		root["hooks"] = hooks
	}
	mergeCredentialGuardClaudeHooks(root, "Bash", guardGitBranchCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessGitBranchGuardTargetGemini wires the global-scope git-branch-guard
// hook for Gemini CLI into ~/.gemini/settings.json, mirroring
// harnessCredentialGuardTargetGemini.
func harnessGitBranchGuardTargetGemini(home string, opts UpdateOptions) TargetResult {
	const id = "gemini-git-branch-guard"
	const displayPath = "~/.gemini/settings.json"

	path := filepath.Join(home, ".gemini", "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardGeminiHooks(root, guardGitBranchCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyScriptPath, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyScriptPath, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["BeforeTool"], "run_shell_command", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
		migrateHookCommand(hooks["AfterTool"], "run_shell_command", legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX)
	}
	// D13 dedup (REQ-2026-10-23): collapse duplicates after individual migrations.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		legacies := append(append([]string{}, guardforms.GitBranchLegacyPSPOSIX...), legacyScriptPath)
		hooks["BeforeTool"] = migrateDedupGuardInNestedArray(hooks["BeforeTool"], "run_shell_command", guardGitBranchCmdPSPOSIX, legacies)
		hooks["AfterTool"] = migrateDedupGuardInNestedArray(hooks["AfterTool"], "run_shell_command", guardGitBranchCmdPSPOSIX, legacies)
		root["hooks"] = hooks
	}
	mergeCredentialGuardGeminiHooks(root, guardGitBranchCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessGitBranchGuardTargetCursor wires the global-scope git-branch-guard
// hook for Cursor into ~/.cursor/hooks.json, mirroring
// harnessCredentialGuardTargetCursor.
func harnessGitBranchGuardTargetCursor(home string, opts UpdateOptions) TargetResult {
	const id = "cursor-git-branch-guard"
	const displayPath = "~/.cursor/hooks.json"

	path := filepath.Join(home, ".cursor", "hooks.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	cursorGetCmd := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		cmd, _ := obj["command"].(string)
		return cmd
	}

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardCursorHooks(root, guardGitBranchCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B/ML-6C: migrate D2 revised and pre-ML-6C D11 inline form → D11 revised form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyScriptPath, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyScriptPath, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD11RevGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyWABashGitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD2GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["beforeShellExecution"], legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
		migrateCursorSimpleCommand(hooks["afterShellExecution"], legacyD11GitBranchCmdPSPOSIX, guardGitBranchCmdPSPOSIX, cursorGetCmd)
	}
	// D13 dedup (REQ-2026-10-23): collapse duplicates after individual migrations.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		legacies := append(append([]string{}, guardforms.GitBranchLegacyPSPOSIX...), legacyScriptPath)
		hooks["beforeShellExecution"] = dedupFlatGuardEntries(hooks["beforeShellExecution"], "", guardGitBranchCmdPSPOSIX, legacies, cursorGetCmd)
		hooks["afterShellExecution"] = dedupFlatGuardEntries(hooks["afterShellExecution"], "", guardGitBranchCmdPSPOSIX, legacies, cursorGetCmd)
		root["hooks"] = hooks
	}
	mergeCredentialGuardCursorHooks(root, guardGitBranchCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessGitBranchGuardTargetCopilot wires the global-scope git-branch-guard
// hook for GitHub Copilot into ~/.copilot/settings.json, mirroring
// harnessCredentialGuardTargetCopilot.
func harnessGitBranchGuardTargetCopilot(home string, opts UpdateOptions) TargetResult {
	const id = "copilot-git-branch-guard"
	const displayPath = "~/.copilot/settings.json"

	path := filepath.Join(home, ".copilot", "settings.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	legacyScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		root := make(map[string]interface{})
		mergeCredentialGuardCopilotHooks(root, guardGitBranchCmdPSPOSIX)
		desired, marshalErr := marshalJSONNoEscape(root)
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, append(desired, '\n'), 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	var root map[string]interface{}
	if len(raw) > 0 {
		if unmarshalErr := json.Unmarshal(raw, &root); unmarshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: unmarshalErr.Error()}
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	// ML-2A: migrate any pre-ML-2A global entry (abs .sh path → inline command).
	// ML-6B: migrate D2 revised inline form → fail-closed D11 form.
	// REQ-2026-10-20: migrate D11 revised pré-REQ-2026-10-20 and workaround bash wrapper forms → new form.
	// Copilot entries use "bash" key (not "command"), so inline migration is needed.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		for _, eventKey := range []string{"preToolUse", "postToolUse"} {
			arr, _ := hooks[eventKey].([]interface{})
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if bash, _ := obj["bash"].(string); bash == legacyScriptPath {
					obj["bash"] = guardGitBranchCmdPSPOSIX
				} else if bash == legacyD11RevGitBranchCmdPSPOSIX {
					obj["bash"] = guardGitBranchCmdPSPOSIX
				} else if bash == legacyWABashGitBranchCmdPSPOSIX {
					obj["bash"] = guardGitBranchCmdPSPOSIX
				} else if bash == legacyD2GitBranchCmdPSPOSIX {
					obj["bash"] = guardGitBranchCmdPSPOSIX
				} else if bash == legacyD11GitBranchCmdPSPOSIX {
					obj["bash"] = guardGitBranchCmdPSPOSIX
				}
			}
		}
	}
	// D13 dedup (REQ-2026-10-23): Copilot uses "bash" key — inline dedup.
	if hooks, _ := root["hooks"].(map[string]interface{}); hooks != nil {
		copilotGitAllForms := make(map[string]bool, len(guardforms.GitBranchLegacyPSPOSIX)+2)
		for _, f := range guardforms.GitBranchLegacyPSPOSIX {
			copilotGitAllForms[f] = true
		}
		copilotGitAllForms[legacyScriptPath] = true
		copilotGitAllForms[guardGitBranchCmdPSPOSIX] = true
		for _, eventKey := range []string{"preToolUse", "postToolUse"} {
			arr, _ := hooks[eventKey].([]interface{})
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				bash, _ := obj["bash"].(string)
				if copilotGitAllForms[bash] && bash != guardGitBranchCmdPSPOSIX {
					obj["bash"] = guardGitBranchCmdPSPOSIX
				}
			}
			seen := false
			var deduped []interface{}
			for _, item := range arr {
				obj, ok := item.(map[string]interface{})
				if !ok {
					deduped = append(deduped, item)
					continue
				}
				bash, _ := obj["bash"].(string)
				if bash == guardGitBranchCmdPSPOSIX {
					if !seen {
						seen = true
						deduped = append(deduped, item)
					}
				} else {
					deduped = append(deduped, item)
				}
			}
			if deduped != nil {
				hooks[eventKey] = deduped
			}
		}
		root["hooks"] = hooks
	}
	mergeCredentialGuardCopilotHooks(root, guardGitBranchCmdPSPOSIX)

	out, marshalErr := marshalJSONNoEscape(root)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')
	if string(desired) == string(raw) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessGitBranchGuardTargetKiro wires the global-scope git-branch-guard
// hook for Kiro into a DEDICATED file,
// ~/.kiro/hooks/trackfw-git-branch-guard.json — deliberately NOT the same
// file harnessCredentialGuardTargetKiro writes
// (~/.kiro/hooks/trackfw-credential-guard.json). That writer rewrites its
// document WHOLESALE (never merges), so two wholesale writers sharing one
// file would each overwrite the other's entries every run: run
// credential-guard, then git-branch-guard, then credential-guard again would
// report "updated" forever (both targets flap forever between their own
// desired 2-entry document) — a hard idempotency failure. Same schema as
// harnessCredentialGuardTargetKiro: top-level {"version":"v1","hooks":[...]},
// each entry {"name","description","trigger","matcher","action":{"type":
// "command","command":<absolute path>}} — but the two hook names are
// "trackfw-git-branch-guard-global-pre"/"-global-post", matching the
// "<tool>-<guard>-global-pre/-post" naming convention
// harnessCredentialGuardTargetKiro already established.
func harnessGitBranchGuardTargetKiro(home string, opts UpdateOptions) TargetResult {
	const id = "kiro-git-branch-guard"
	const displayPath = "~/.kiro/hooks/trackfw-git-branch-guard.json"

	path := filepath.Join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json")
	// Guard: reject writes through symlinks before any filesystem mutation
	// (ADR-2026-09-18 / ML-1B).
	if result, guarded := rejectHarnessSymlink(home, path, id, displayPath); guarded {
		return result
	}
	// ML-2A: global configs now emit the inline command instead of an absolute .sh path.
	// Kiro/Amazon Q are CmdExe-family: no "exit $LASTEXITCODE" suffix.
	content := map[string]interface{}{
		"version": "v1",
		"hooks": []interface{}{
			map[string]interface{}{
				"name":        "trackfw-git-branch-guard-global-pre",
				"description": "Blocks branch-creation git subcommands issued outside trackfw branch new (global, all trackfw projects)",
				"trigger":     "PreToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": guardGitBranchCmdCmdExe},
			},
			map[string]interface{}{
				"name":        "trackfw-git-branch-guard-global-post",
				"description": "Warns on branch-creation git subcommands issued outside trackfw branch new (global, all trackfw projects)",
				"trigger":     "PostToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": guardGitBranchCmdCmdExe},
			},
		},
	}
	out, marshalErr := marshalJSONNoEscape(content)
	if marshalErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
	}
	desired := append(out, '\n')

	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if !opts.InstallMissing {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if opts.DryRun {
			return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case err != nil:
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	if string(data) == string(desired) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	if opts.DryRun {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// harnessCatalogTarget evaluates (and, unless DryRun, applies) every
// global-scope catalog item of the given (tool, kind) pair. Multiple catalog
// items (one per agent/skill) share a single reported target, matching the
// contract's example ("codex-agents" reporting one state for the
// "~/.codex/agents" directory as a whole):
//
//   - Every catalog item currently NOT installed is left alone (default) or
//     installed (--install-missing); it never turns the whole target into
//     "failed" merely for being absent.
//   - If at least one item write fails, the whole target is "failed".
//   - Else if at least one item was installed or brought current, "updated".
//   - Else if nothing at all is installed, "missing".
//   - Else (everything installed and already current), "skipped".
//
// displayPath is derived from the catalog itself (integrations.GlobalGroupPath)
// rather than from any individual installed plan's destination, so the
// reported path never depends on catalog item iteration order.
// harnessCatalogTarget evaluates (and, unless DryRun, applies) every
// global-scope catalog item of the given (tool, kind) pair.
//
// claudeDir is the pre-resolved agentconfig.ClaudeConfigDir value from the
// caller (UpdateHarness). It is passed in to satisfy ADR-2026-10-14 D3 (writer
// and auditor use the same resolver instance). claudeDirErr is the error from
// that resolution; if non-nil and tool is "claude", the Manager will propagate
// the error through ClaudeDir.
func harnessCatalogTarget(catalog *integrations.Catalog, id, tool string, kind integrations.ItemKind, home, claudeDir string, claudeDirErr error, opts UpdateOptions) TargetResult {
	displayPath, pathErr := integrations.GlobalGroupPath(catalog, tool, kind)
	if pathErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: "", Message: pathErr.Error()}
	}
	ident, err := identity.Load(home)
	if err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}
	harnessCwd, _ := os.Getwd() // for AC14 diagnostic only; failure is OK (empty → no diagnostic)
	harnessAgentModels, harnessWarnMsg := config.ResolveAgentModels("global", home, harnessCwd)
	if harnessWarnMsg != "" {
		fmt.Fprintln(os.Stderr, harnessWarnMsg)
	}
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind:        kind,
		Targets:     []string{tool},
		Scope:       "global",
		Identity:    ident,
		AgentModels: harnessAgentModels,
	})
	if err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	// ML-1D (ADR-2026-10-14 D1/A1): set ClaudeDir only for the claude tool so
	// global claude agents/skills are written at the path Claude Code actually
	// reads. claudeDir comes from the caller (D3: one resolved value per harness
	// run). Non-Claude tools are unaffected (D2: behaviour unchanged).
	manager := integrations.Manager{ProjectRoot: home, HomeDir: home}
	if tool == agentconfig.CLIClaude {
		capturedDir, capturedErr := claudeDir, claudeDirErr
		manager.ClaudeDir = func() (string, error) { return capturedDir, capturedErr }
	}
	anyInstalled := false
	anyChanged := false
	for _, plan := range plans {
		inspection, inspectErr := manager.Inspect(plan)
		if inspectErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: inspectErr.Error()}
		}
		switch inspection.State {
		case integrations.StateNotInstalled:
			if !opts.InstallMissing {
				continue
			}
			anyInstalled = true
			if opts.DryRun {
				anyChanged = true
				continue
			}
			if installErr := manager.Install([]integrations.PlannedArtifact{plan}, false); installErr != nil {
				return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: installErr.Error()}
			}
			anyChanged = true
		case integrations.StateCurrent:
			anyInstalled = true
		case integrations.StateModified:
			// Preserved, never overwritten here: either genuinely unmanaged
			// content that doesn't match a trackfw template (must not be
			// overwritten, per contract's "skipped" definition) or a
			// manifest-owned file locally modified by the user (this surface
			// never sets force, so it is never clobbered). Counts as
			// installed but unchanged.
			anyInstalled = true
		case integrations.StateOutdated:
			// Either manifest-owned and behind the current catalog version,
			// or unmanaged bytes that match a recognized legacy hash — both
			// are safe to migrate/update without --force.
			anyInstalled = true
			if opts.DryRun {
				anyChanged = true
				continue
			}
			if updateErr := manager.Update([]integrations.PlannedArtifact{plan}, false); updateErr != nil {
				return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: updateErr.Error()}
			}
			anyChanged = true
		}
	}

	if !anyInstalled {
		return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
	}
	if anyChanged {
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	}
	return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
}

// ────────────────────────────────────────────────────────────────────────────
// trackfw update (project scope) — four-state model, contract in
// docs/cli-parity.md ("`trackfw update` vs `trackfw update harness`",
// "Flags", "JSON document"). This section exposes the same --dry-run,
// --json, --targets and --install-missing surface as the harness command,
// over the SAME writes Update(cwd) already performs — it does not change
// what gets written, only how the outcome is reported.
//
// NOTE ON CROSS-RUNTIME PARITY: the pinned target list in cli-parity.md
// covers `update harness` only. The three runtimes' project-scope target
// SETS are not reconcilable byte-for-byte, but the gap is narrower than it
// used to be: as of REQ-2026-08-28, the Python CLI declares `ci-workflow`
// in PROJECT_TARGET_IDS under the same condition as Go and Node.js
// (pypi/trackfw/commands/update.py's project_target_ids()) and dispatches
// `claude-commands` like the other two runtimes (pre-existing, not part of
// that REQ). The one target that remains genuinely absent from Python is
// `git-hooks`: the Python `init` has no surface to opt into a hook
// framework in the first place, so `update` has nothing to declare a
// target for — tracked as its own gap in
// REQ-2026-08-28-cli-python-nao-oferece-superficie-de-ci-e-git-hooks-no-init-e-nao-declara-git-hooks-como-alvo-do-update.md.
// The four states, four flags and JSON document SHAPE are shared; the
// target ID list is not pinned and is reported here, not silently forced
// into agreement.
// ────────────────────────────────────────────────────────────────────────────

// ProjectTargetIDs is the declared order of `trackfw update` (project scope)
// targets for this runtime. "git-hooks" only appears when the project's
// trackfw.yaml opted into a hook framework. "ci-workflow" appears when
// EITHER the project's trackfw.yaml opted into a CI system OR
// trackfw-validate.yml (written by `trackfw discover --init`, an independent
// install mechanism — DiscoverGitHubActionsWorkflowPath, scaffold_doctor.go)
// is already present on disk (AC17(c), REQ-2026-08-28) — that second clause
// is what lets `update` manage a discover-installed workflow even in a
// `ci: none` project, closing the gap where that file was otherwise outside
// any command's management and the doctor's `trackfw update` remedy for it
// was inert.
func ProjectTargetIDs(cfg Config, discoverWorkflowPresent bool) []string {
	ids := []string{"agent-rules", "agent-hooks", "codex-project-agents", "validate-script"}
	if (cfg.CI != "" && cfg.CI != "none") || discoverWorkflowPresent {
		ids = append(ids, "ci-workflow")
	}
	if cfg.Hooks == "husky" || cfg.Hooks == "lefthook" {
		ids = append(ids, "git-hooks")
	}
	ids = append(ids, "claude-commands")
	return ids
}

// discoverWorkflowPresent reports whether DiscoverGitHubActionsWorkflowPath
// (.github/workflows/trackfw-validate.yml) already exists under root as a
// REGULAR file. Used both to decide whether "ci-workflow" is declared
// (AC17(c)) and, inside its apply function, to decide whether to refresh it
// — the existence check always reads the real project tree (root passed to
// ProjectTargetIDs is always cwd, never the --dry-run sandbox, mirroring how
// cfg itself is loaded from the real cwd before the sandbox is built).
//
// Uses os.Lstat, NOT os.Stat: a symlink at this path is not something
// `update` owns or can safely refresh — os.Stat follows the link and would
// make an attacker-controlled path outside the project look "present",
// pulling "ci-workflow" into the declared target set (and, via
// refreshDiscoverGitHubActionsWorkflowIfPresent below, into the write path)
// purely because a symlink exists on disk. Symlinks are therefore treated as
// NOT present for this file: `update` will not declare/manage a target on
// their account, and refreshDiscoverGitHubActionsWorkflowIfPresent (which
// re-checks independently) refuses to write through them either way.
func discoverWorkflowPresent(root string) bool {
	info, err := os.Lstat(filepath.Join(root, DiscoverGitHubActionsWorkflowPath))
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink == 0
}

// refreshDiscoverGitHubActionsWorkflowIfPresent refreshes
// .github/workflows/trackfw-validate.yml ONLY when it already exists under
// root as a REGULAR file — `update` never creates this file (AC17(b)):
// ownership of the install decision belongs to `trackfw discover --init`,
// not `update`. Writes the SAME builder scaffold doctor compares against
// (BuildDiscoverGitHubActionsWorkflowContent, scaffold_doctor.go) so what
// `update` writes and what `doctor` expects can never drift apart by
// construction (REQ-2026-08-28 AC17).
//
// Uses os.Lstat to decide presence, not os.Stat: this path is the most
// sensitive one `update` can write to (it controls what runs in CI for
// anyone who checks the project out), so if it is a symlink — live or
// dangling — this function refuses to write through it. It is not
// `update`'s call whether to follow a link planted at a path it manages;
// the file may not even belong to this project. Refusing is loud (stderr),
// never silent, so "update didn't refresh my workflow" is diagnosable.
func refreshDiscoverGitHubActionsWorkflowIfPresent(rootDir string) error {
	// ML-8A / #402: resolve the root FIRST and derive path from it, so the guard
	// compares two paths in the same namespace. filepath.Clean(root) only
	// normalised text. The Lstat below still runs on the unresolved-by-Join
	// target, so a symlinked workflow file is still seen as a symlink.
	guardRoot, rootErr := pathguard.ResolveRoot(rootDir)
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(filepath.Join(rootDir, DiscoverGitHubActionsWorkflowPath), rootErr)
	}
	path := filepath.Join(guardRoot, DiscoverGitHubActionsWorkflowPath)
	if _, err := os.Lstat(path); err != nil {
		return nil // not installed — update never creates it (AC17(b))
	}
	// Single containment check for BOTH the leaf and every ancestor up to root:
	// pathguard.RejectSymlinks Lstats the leaf first, so the separate leaf-only
	// branch that used to live here (with its own "aviso: … é um symlink"
	// wording) was a sixth message grammar for an outcome this call already
	// produces. ML-7B collapsed it; refusal stays loud, now in the single
	// grammar emitted by pathguard.
	if guardErr := pathguard.RejectAndReport(guardRoot, path); guardErr != nil {
		return nil
	}
	// Read the configured trigger from trackfw.yaml via config.Load().
	// This is safe because every caller (Update, UpdateProject) chdirs to the project root
	// before invoking this function, so config.Load() reads the correct trackfw.yaml.
	// The test path (no trackfw.yaml) returns CITrigger="" → pull_request template (AC2).
	ciTrigger := config.Load().Update.CITrigger
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	return os.WriteFile(path, []byte(BuildDiscoverGitHubActionsWorkflowContentWithTrigger(IsProducerGoMod(guardRoot), ciTrigger)), 0o644)
}

// UpdateProject evaluates (and, unless DryRun, applies) every declared
// project-scope target for the project rooted at cwd. --dry-run runs every
// target's real writer against a throwaway copy of the project tree so nothing
// under cwd is ever touched; the real run writes to cwd directly (identical
// to what Update(cwd) has always done).
func UpdateProject(cwd string, opts UpdateOptions) (UpdateReport, error) {
	if _, err := os.Stat(filepath.Join(cwd, "trackfw.yaml")); err != nil {
		return UpdateReport{}, fmt.Errorf("trackfw.yaml não encontrado — execute trackfw init primeiro")
	}
	var cfg Config
	if err := withChdir(cwd, func() error { cfg = loadUpdateConfig(); return nil }); err != nil {
		return UpdateReport{}, fmt.Errorf("loading trackfw.yaml: %w", err)
	}

	// AC4 (REQ-2026-10-24): refuse early when ci_trigger is invalid.
	if cfg.CITriggerInvalid {
		return UpdateReport{}, fmt.Errorf("ci_trigger: %q é inválido — valores aceitos: pull_request, manual", cfg.CITriggerRawValue)
	}

	declared := ProjectTargetIDs(cfg, discoverWorkflowPresent(cwd))
	selected, err := selectDeclaredTargets(declared, opts.Targets)
	if err != nil {
		return UpdateReport{}, err
	}

	applyRoot := cwd
	if opts.DryRun {
		tmp, mkErr := os.MkdirTemp("", "trackfw-update-")
		if mkErr != nil {
			return UpdateReport{}, fmt.Errorf("preparing dry-run sandbox: %w", mkErr)
		}
		defer os.RemoveAll(tmp) //nolint:errcheck
		if cpErr := copyProjectTree(cwd, tmp, buildSandboxInclusion(selected, cfg)); cpErr != nil {
			return UpdateReport{}, fmt.Errorf("preparing dry-run sandbox: %w", cpErr)
		}
		applyRoot = tmp
	}

	results := make([]TargetResult, 0, len(selected))
	for _, id := range selected {
		results = append(results, runProjectTarget(id, applyRoot, cfg, opts))
	}
	return UpdateReport{Scope: "project", DryRun: opts.DryRun, Targets: results}, nil
}

// runProjectTarget dispatches a single declared project target id to its
// writer and relevant paths.
func runProjectTarget(id, root string, cfg Config, opts UpdateOptions) TargetResult {
	switch id {
	case "agent-rules":
		return runFileTarget(id,
			"CLAUDE.md, AGENTS.md, GEMINI.md, .github/copilot-instructions.md, .windsurfrules, .amazonq/developer/guidelines.md, .cursor/rules/trackfw.mdc",
			root,
			[]string{"CLAUDE.md", "AGENTS.md", "GEMINI.md", ".github/copilot-instructions.md", ".windsurfrules", ".amazonq/developer/guidelines.md", ".cursor/rules/trackfw.mdc"},
			func(r string) error { return InjectRulesDetected(r) },
			opts)
	case "agent-hooks":
		return runFileTarget(id,
			".claude/settings.json, .codex/hooks.json, .gemini/settings.json, .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, scripts/trackfw-attention-*.sh, scripts/trackfw-credential-guard.sh, scripts/trackfw-git-branch-guard.sh, .windsurf/hooks.json, .amazonq/cli-agents/q_cli_default.json",
			root,
			[]string{
				".claude/settings.json",
				".codex/hooks.json",
				".gemini/settings.json",
				".kiro/hooks/trackfw-attention.json",
				".github/hooks/trackfw-attention.json",
				".cursor/hooks.json",
				"scripts/trackfw-attention-signal.sh",
				"scripts/trackfw-attention-cleanup.sh",
				"scripts/trackfw-credential-guard.sh",
				"scripts/trackfw-git-branch-guard.sh",
				".windsurf/hooks.json",                   // Gap A: InjectWindsurfHooks writes this
				".amazonq/cli-agents/q_cli_default.json", // Gap B: InjectAmazonQHooks writes this
			},
			func(r string) error {
				return withChdir(r, func() error {
					if err := InjectHooksDetected(r); err != nil {
						return err
					}
					if err := GenerateAttentionScripts(""); err != nil {
						return err
					}
					if err := GenerateCredentialGuardScript(""); err != nil {
						return err
					}
					return GenerateGitBranchGuardScript("")
				})
			},
			opts)
	case "codex-project-agents":
		displayPath := ".codex/agents, .agents/skills"
		_, agentsErr := os.Stat(filepath.Join(root, "AGENTS.md"))
		_, codexErr := os.Stat(filepath.Join(root, ".codex"))
		if agentsErr != nil && codexErr != nil {
			return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
		}
		if err := codexProjectAgentsApply(root, opts); err != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
		}
		return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
	case "validate-script":
		return runFileTarget(id, "scripts/trackfw-validate.sh", root,
			[]string{"scripts/trackfw-validate.sh"},
			func(r string) error { return withChdir(r, func() error { return generateValidateScript(cfg) }) },
			opts)
	case "ci-workflow":
		return runFileTarget(id,
			".github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml, "+DiscoverGitHubActionsWorkflowPath,
			root,
			[]string{".github/workflows/trackfw-gate.yml", ".gitlab-ci-trackfw.yml", DiscoverGitHubActionsWorkflowPath},
			func(r string) error {
				if err := withChdir(r, func() error { return generateCIWorkflow(cfg) }); err != nil {
					return err
				}
				return refreshDiscoverGitHubActionsWorkflowIfPresent(r)
			},
			opts)
	case "git-hooks":
		relPath := ".husky/pre-commit"
		if cfg.Hooks == "lefthook" {
			relPath = "lefthook.yml"
		}
		return runFileTarget(id, relPath, root, []string{relPath},
			func(r string) error {
				return withChdir(r, func() error { updateHooksSurgical(r, cfg); return nil })
			},
			opts)
	case "claude-commands":
		return runFileTarget(id, ".claude/commands/trackfw", root,
			[]string{".claude/commands/trackfw"},
			func(r string) error { return withChdir(r, func() error { return ForceGenerateClaudeCommands() }) },
			opts)
	default:
		return TargetResult{ID: id, State: TargetFailed, Path: "", Message: fmt.Sprintf("unhandled target %q", id)}
	}
}

// codexProjectAgentsApply re-applies (and, with InstallMissing, installs)
// the project-scoped Codex agents/skills catalog bundle, using the identity
// currently persisted at ~/.trackfw/identity.json. Generalizes
// updateDetectedCodexIntegrations with --install-missing support.
func codexProjectAgentsApply(root string, opts UpdateOptions) error {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}
	home, err := homedir.Dir()
	if err != nil {
		return err
	}
	ident, err := identity.Load(home)
	if err != nil {
		return err
	}
	// ML-1D: ClaudeDir is intentionally not set — this Manager handles Codex
	// project-scope artifacts only (Targets: []string{"codex"}, Scope: "project").
	// No claude/global plans reach it, so ClaudeDir is behaviour-neutral here.
	manager := integrations.Manager{ProjectRoot: root, HomeDir: home}
	var projectAgentModels map[string]string
	if chErr := withChdir(root, func() error {
		projectAgentModels = config.Load().AgentModels
		return nil
	}); chErr != nil {
		return chErr
	}
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		plans, planErr := integrations.BuildPlans(catalog, integrations.PlanRequest{Kind: kind, Targets: []string{"codex"}, Scope: "project", Identity: ident, AgentModels: projectAgentModels})
		if planErr != nil {
			return planErr
		}
		for _, plan := range plans {
			inspection, inspectErr := manager.Inspect(plan)
			if inspectErr != nil {
				return inspectErr
			}
			switch inspection.State {
			case integrations.StateNotInstalled:
				if !opts.InstallMissing {
					continue
				}
				if instErr := manager.Install([]integrations.PlannedArtifact{plan}, false); instErr != nil {
					return instErr
				}
			case integrations.StateOutdated:
				if updErr := manager.Update([]integrations.PlannedArtifact{plan}, false); updErr != nil {
					return updErr
				}
			}
		}
	}
	return nil
}

// runFileTarget computes updated/skipped/missing/failed for a target whose
// only observable effect is writing under a fixed set of paths (files or
// directories) relative to root, by diffing content hashes before/after
// invoking apply(root). Mirrors npm/src/lib/update-engine.js:runFileTarget.
//
// "missing" never installs: if every declared relPath is absent before
// apply and InstallMissing is not set, apply is never called.
func runFileTarget(id, displayPath, root string, relPaths []string, apply func(root string) error, opts UpdateOptions) TargetResult {
	before := hashRelPaths(root, relPaths)
	if allEmpty(before) && !opts.InstallMissing {
		return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
	}

	if err := apply(root); err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	after := hashRelPaths(root, relPaths)
	if allEmpty(before) && allEmpty(after) {
		return TargetResult{ID: id, State: TargetMissing, Path: displayPath}
	}
	if equalHashes(before, after) {
		return TargetResult{ID: id, State: TargetSkipped, Path: displayPath}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

func hashRelPaths(root string, relPaths []string) []string {
	hashes := make([]string, len(relPaths))
	for i, rel := range relPaths {
		hashes[i] = hashPathContent(filepath.Join(root, rel))
	}
	return hashes
}

func allEmpty(hashes []string) bool {
	for _, h := range hashes {
		if h != "" {
			return false
		}
	}
	return true
}

func equalHashes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// hashPathContent returns "" when path does not exist, a content hash for a
// file, or a hash of the recursive (relative-path, content-hash) listing for
// a directory.
func hashPathContent(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if !info.IsDir() {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return ""
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	var entries []string
	walkErr := filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil //nolint:nilerr
		}
		rel, relErr := filepath.Rel(path, p)
		if relErr != nil {
			return nil //nolint:nilerr
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil //nolint:nilerr
		}
		sum := sha256.Sum256(data)
		entries = append(entries, rel+":"+hex.EncodeToString(sum[:]))
		return nil
	})
	if walkErr != nil {
		return ""
	}
	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:])
}

// withChdir runs fn with the process working directory temporarily set to
// root, restoring the original directory afterward. Several existing
// generator functions (generateValidateScript, GenerateAttentionScripts,
// generateCIWorkflow, updateHooksSurgical, ForceGenerateClaudeCommands)
// write through relative paths and rely on the caller having already
// changed directory — this lets UpdateProject reuse them unmodified against
// either the real project root or a --dry-run sandbox copy.
func withChdir(root string, fn func() error) error {
	orig, err := os.Getwd()
	if err != nil {
		return err
	}
	if chErr := os.Chdir(root); chErr != nil {
		return chErr
	}
	defer os.Chdir(orig) //nolint:errcheck
	return fn()
}

// buildSandboxInclusion returns the union of paths that must be copied into
// the --dry-run sandbox for all selected targets to operate correctly.
//
// It includes three categories:
//  1. Each selected target's declared relPaths (the written outputs that
//     dry-run hashes before and after apply to detect changes).
//  2. trackfw.yaml — always a prerequisite: agent-rules reads it for
//     agent_conventions when generating CLAUDE.md (Gap E).
//  3. Detection-signal files for agent-hooks (Gap C): InjectHooksDetected
//     checks these files/dirs to decide which hooks to inject. Without them
//     the sandbox silently skips hooks the real run would write.
func buildSandboxInclusion(selected []string, cfg Config) []string {
	seen := make(map[string]struct{})
	add := func(p string) { seen[p] = struct{}{} }

	// Gap E prerequisite: always copy trackfw.yaml so agent-rules can read
	// agent_conventions and produce a byte-identical CLAUDE.md.
	add("trackfw.yaml")

	agentHooksSelected := false
	for _, id := range selected {
		switch id {
		case "agent-rules":
			for _, p := range []string{
				"CLAUDE.md",
				"AGENTS.md",
				"GEMINI.md",
				".github/copilot-instructions.md",
				".windsurfrules",
				".amazonq/developer/guidelines.md",
				".cursor/rules/trackfw.mdc",
			} {
				add(p)
			}
		case "agent-hooks":
			agentHooksSelected = true
			for _, p := range []string{
				".claude/settings.json",
				".codex/hooks.json",
				".gemini/settings.json",
				".kiro/hooks/trackfw-attention.json",
				".github/hooks/trackfw-attention.json",
				".cursor/hooks.json",
				"scripts/trackfw-attention-signal.sh",
				"scripts/trackfw-attention-cleanup.sh",
				"scripts/trackfw-credential-guard.sh",
				"scripts/trackfw-git-branch-guard.sh",
				".windsurf/hooks.json",                   // Gap A
				".amazonq/cli-agents/q_cli_default.json", // Gap B
			} {
				add(p)
			}
		case "validate-script":
			add("scripts/trackfw-validate.sh")
		case "claude-commands":
			add(".claude/commands/trackfw")
		case "ci-workflow":
			add(".github/workflows/trackfw-gate.yml")
			add(".gitlab-ci-trackfw.yml")
			add(DiscoverGitHubActionsWorkflowPath)
		case "git-hooks":
			relPath := ".husky/pre-commit"
			if cfg.Hooks == "lefthook" {
				relPath = "lefthook.yml"
			}
			add(relPath)
		}
	}

	// Gap C: InjectHooksDetected (agent-hooks apply) reads these files to
	// detect which agents are installed. If only --targets agent-hooks is
	// requested (without agent-rules), they would be absent from the
	// sandbox and hooks would be silently omitted.
	if agentHooksSelected {
		for _, p := range []string{
			"CLAUDE.md",
			"AGENTS.md",
			"GEMINI.md",
			".github/copilot-instructions.md",
			".windsurfrules",
			".amazonq/developer/guidelines.md",
		} {
			add(p)
		}
	}

	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// copyPath copies src to dst using os.Lstat (symlink-aware).
//
// Broken symlinks are silently skipped — the destination is simply absent,
// which causes hashPathContent to return "" (treated as "missing"), matching
// Node.js's fs.existsSync behaviour (existsSync follows symlinks; broken
// symlink → false → copyPath returns without copying).
//
// Valid symlinks are copied as regular files (the content of the symlink
// target is written to dst, not the symlink itself), also matching Node.js's
// fs.copyFileSync behaviour (R6 in the declared residual).
//
// Directories are recursed: each entry is copied via copyPath, preserving
// symlink semantics throughout the subtree. The directory itself is created
// with os.MkdirAll before the loop so an empty declared directory materialises
// as present (not absent) in the sandbox.
func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if os.IsNotExist(err) {
		return nil // absent or broken symlink — skip
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		// Symlink: verify it resolves before reading through it.
		if _, statErr := os.Stat(src); statErr != nil {
			return nil // broken symlink — skip
		}
		// Valid symlink: copy content (os.ReadFile follows symlinks).
		data, readErr := os.ReadFile(src)
		if readErr != nil {
			return readErr
		}
		// write-containment-allowed: safe — dst is always an os.MkdirTemp sandbox root (never $HOME); no pathguard needed
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
			return mkErr
		}
		// write-containment-allowed: safe — dst is always an os.MkdirTemp sandbox root (never $HOME); no pathguard needed
		return os.WriteFile(dst, data, 0644)
	}
	if info.IsDir() {
		// write-containment-allowed: safe — dst is always an os.MkdirTemp sandbox root (never $HOME); no pathguard needed
		if mkErr := os.MkdirAll(dst, 0755); mkErr != nil {
			return mkErr
		}
		entries, readErr := os.ReadDir(src)
		if readErr != nil {
			return readErr
		}
		for _, entry := range entries {
			if err := copyPath(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	// Regular file.
	data, readErr := os.ReadFile(src)
	if readErr != nil {
		return readErr
	}
	// write-containment-allowed: safe — dst is always an os.MkdirTemp sandbox root (never $HOME); no pathguard needed
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
		return mkErr
	}
	// write-containment-allowed: safe — dst is always an os.MkdirTemp sandbox root (never $HOME); no pathguard needed
	return os.WriteFile(dst, data, 0644)
}

// copyProjectTree copies only the declared inclusion paths from src into dst,
// using copyPath (Lstat-based, symlink-safe) for each entry.
//
// This replaces the former WalkDir-based copy that traversed the entire
// project tree, which aborted on broken symlinks outside the declared
// target set (KG's CMDB incident: .venv/bin/python → removed python3.13).
// With inclusion-based copying, anything not in paths is irrelevant —
// broken symlinks outside the declared set have zero effect.
func copyProjectTree(src, dst string, paths []string) error {
	for _, rel := range paths {
		if err := copyPath(filepath.Join(src, rel), filepath.Join(dst, rel)); err != nil {
			return fmt.Errorf("sandbox: copying %s: %w", rel, err)
		}
	}
	return nil
}
