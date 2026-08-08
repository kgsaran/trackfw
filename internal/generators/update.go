package generators

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
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
		Hooks:      u.Hooks,
		CI:         u.CI,
		Backend:    u.Backend,
		Frontend:   u.Frontend,
		PkgManager: u.PkgManager,
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

	// 3. CI workflow (categoria 2 — trackfw-owned, overwrite seguro)
	if err := generateCIWorkflow(cfg); err != nil {
		fmt.Printf("  ⚠ CI workflow: %v\n", err)
	} else if cfg.CI != "" && cfg.CI != "none" {
		fmt.Println("  ✓ CI workflow atualizado")
	}

	// 4. Git hooks — cirúrgico (categoria 3 — shared user files)
	updateHooksSurgical(cfg)

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
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("  ⚠ Codex integration home: %v\n", err)
		return nil
	}
	ident, err := identity.Load(home)
	if err != nil {
		return fmt.Errorf("codex integration identity: %w", err)
	}
	manager := integrations.Manager{ProjectRoot: cwd, HomeDir: home}
	updated := 0
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		plans, planErr := integrations.BuildPlans(catalog, integrations.PlanRequest{Kind: kind, Targets: []string{"codex"}, Scope: "project", Identity: ident})
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
func updateHooksSurgical(cfg Config) {
	switch cfg.Hooks {
	case "husky":
		path := filepath.Join(".husky", "pre-commit")
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "trackfw validate") {
			fmt.Println("  ✓ .husky/pre-commit — trackfw validate já presente")
			return
		}
		os.MkdirAll(".husky", 0755) //nolint:errcheck
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			fmt.Printf("  ⚠ .husky/pre-commit: %v\n", err)
			return
		}
		defer f.Close()
		fmt.Fprintln(f, "\ntrackfw validate")
		fmt.Println("  ✓ .husky/pre-commit — trackfw validate injetado")

	case "lefthook":
		path := "lefthook.yml"
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), "trackfw-validate:") || strings.Contains(string(data), "trackfw validate") {
			fmt.Println("  ✓ lefthook.yml — trackfw já presente")
			return
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("  ⚠ lefthook.yml: %v\n", err)
			return
		}
		defer f.Close()
		fmt.Fprintln(f, "\npre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate")
		fmt.Println("  ✓ lefthook.yml — trackfw-validate injetado")
	}
}

// ensureGlobalADRDirRegistered registers ~/.trackfw/adr in trackfw.yaml's
// adr_dirs list, surgically (text-level edit, never config.Load()+re-
// serialize, which would lose the user's comments/formatting) and
// idempotently, but only when there is something to gain from it: the global
// ADR directory must exist AND contain at least one ADR-*.md file. This keeps
// `trackfw update` from cluttering every project's trackfw.yaml with a
// pointer to a directory that is empty or doesn't exist on this machine.
func ensureGlobalADRDirRegistered(cwd string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	globalDir := GlobalADRDir(home)
	if _, statErr := os.Stat(globalDir); statErr != nil {
		return nil // global ADR dir doesn't exist — no-op
	}
	matches, globErr := filepath.Glob(filepath.Join(globalDir, "ADR-*.md"))
	if globErr != nil {
		return fmt.Errorf("checking for ADR files in %s: %w", globalDir, globErr)
	}
	if len(matches) == 0 {
		return nil // global ADR dir has no ADRs yet — no-op
	}

	yamlPath := filepath.Join(cwd, "trackfw.yaml")
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
	Message string // only set when State == TargetFailed
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
// targets: 27 ids — "claude-skill", then "claude-credential-guard" (global
// credential-guard hook wiring for Claude Code — placed immediately after
// claude-skill since both are Claude-Code-scoped global artifacts), then for
// each catalog target in harnessCatalogTargetOrder its "<tool>-agents"/
// "<tool>-skills" pair, with "codex-credential-guard" inserted immediately
// BEFORE "codex-agents"/"codex-skills" (ROADMAP-2026-08-06 Wave 2/ML-2B) and
// "gemini-credential-guard" inserted immediately BEFORE "gemini-agents"/
// "gemini-skills" (ROADMAP-2026-08-06 Wave 2/ML-2C), and
// "cursor-credential-guard" inserted immediately BEFORE "cursor-agents"/
// "cursor-skills" (ROADMAP-2026-08-06 Wave 2/ML-2D), and
// "copilot-credential-guard" inserted immediately BEFORE "copilot-agents"/
// "copilot-skills" (ROADMAP-2026-08-06 Wave 2/ML-2E), and
// "kiro-credential-guard" inserted immediately BEFORE "kiro-agents"/
// "kiro-skills" (ROADMAP-2026-08-06 Wave 2/ML-2F, the last credential-guard
// target of this wave — Windsurf has no native hook mechanism and stays out
// per the ADR) — same relative position as claude-credential-guard/
// codex-credential-guard/gemini-credential-guard/cursor-credential-guard/
// copilot-credential-guard (credential-guard target precedes that tool's
// agents/skills pair, never follows it). Order here
// is authoritative for both JSON output and iteration — it must never be
// derived from the filesystem or from what happens to be installed on a
// given machine (see docs/cli-parity.md, "targets follows the declared
// target order, not filesystem order").
var HarnessTargetIDs = buildHarnessTargetIDs()

func buildHarnessTargetIDs() []string {
	ids := make([]string, 0, 4+2*len(harnessCatalogTargetOrder))
	ids = append(ids, "claude-skill", "claude-credential-guard")
	for _, tool := range harnessCatalogTargetOrder {
		if tool == "codex" {
			ids = append(ids, "codex-credential-guard")
		}
		if tool == "gemini" {
			ids = append(ids, "gemini-credential-guard")
		}
		if tool == "cursor" {
			ids = append(ids, "cursor-credential-guard")
		}
		if tool == "copilot" {
			ids = append(ids, "copilot-credential-guard")
		}
		if tool == "kiro" {
			ids = append(ids, "kiro-credential-guard")
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

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return UpdateReport{}, fmt.Errorf("resolving home directory: %w", homeErr)
	}

	// The per-CLI *-credential-guard targets below only wire hook entries that
	// point at ~/.trackfw/scripts/trackfw-credential-guard.sh — none of them
	// write the script itself (ADR-2026-08-06, decision #2/#3). Without this
	// call the wiring is installed but every hook invocation fails with
	// "No such file or directory" because the script never exists.
	if !opts.DryRun {
		if err := GenerateGlobalCredentialGuardScript(home); err != nil {
			return UpdateReport{}, fmt.Errorf("generating global credential guard script: %w", err)
		}
	}

	catalog, catalogErr := integrations.LoadCatalog()
	if catalogErr != nil {
		return UpdateReport{}, fmt.Errorf("loading integration catalog: %w", catalogErr)
	}

	results := make([]TargetResult, 0, len(selected))
	for _, id := range selected {
		if id == "claude-skill" {
			results = append(results, harnessClaudeSkillTarget(home, opts))
			continue
		}
		if id == "claude-credential-guard" {
			results = append(results, harnessCredentialGuardTargetClaude(home, opts))
			continue
		}
		if id == "codex-credential-guard" {
			results = append(results, harnessCredentialGuardTargetCodex(home, opts))
			continue
		}
		if id == "gemini-credential-guard" {
			results = append(results, harnessCredentialGuardTargetGemini(home, opts))
			continue
		}
		if id == "cursor-credential-guard" {
			results = append(results, harnessCredentialGuardTargetCursor(home, opts))
			continue
		}
		if id == "copilot-credential-guard" {
			results = append(results, harnessCredentialGuardTargetCopilot(home, opts))
			continue
		}
		if id == "kiro-credential-guard" {
			results = append(results, harnessCredentialGuardTargetKiro(home, opts))
			continue
		}
		tool, kind, ok := splitHarnessCatalogTargetID(id)
		if !ok {
			continue
		}
		results = append(results, harnessCatalogTarget(catalog, id, tool, kind, home, opts))
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

// harnessClaudeSkillTarget evaluates (and, unless DryRun, applies) the
// historical global Claude compatibility skill.
func harnessClaudeSkillTarget(home string, opts UpdateOptions) TargetResult {
	const id = "claude-skill"
	const displayPath = "~/.claude/skills/trackfw/SKILL.md"

	path := GlobalClaudeSkillPath(home)
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
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
func harnessCredentialGuardTargetClaude(home string, opts UpdateOptions) TargetResult {
	const id = "claude-credential-guard"
	const displayPath = "~/.claude/settings.json"

	path := filepath.Join(home, ".claude", "settings.json")
	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

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
		mergeCredentialGuardClaudeHooks(root, scriptPath)
		desired, marshalErr := json.MarshalIndent(root, "", "  ")
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
	mergeCredentialGuardClaudeHooks(root, scriptPath)

	out, marshalErr := json.MarshalIndent(root, "", "  ")
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
	if writeErr := os.WriteFile(path, desired, 0644); writeErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: writeErr.Error()}
	}
	return TargetResult{ID: id, State: TargetUpdated, Path: displayPath}
}

// mergeCredentialGuardClaudeHooks merges the credential-guard PreToolUse/
// PostToolUse[matcher:"Bash"] entries into root["hooks"], preserving any
// other hook groups/matchers already present (same merge contract as
// InjectClaudeHooks, minus the attention-signal/cleanup entries which stay
// project-scope only). Despite the name (kept for git-blame continuity with
// ML-2A), this helper is shape-agnostic — it only touches
// root["hooks"]["PreToolUse"/"PostToolUse"] with matcher "Bash", the exact
// same JSON shape Codex's .codex/hooks.json (InjectCodexHooks,
// agentfiles.go) already uses for its own PreToolUse/PostToolUse[Bash]
// entries — so harnessCredentialGuardTargetCodex below reuses it verbatim
// instead of duplicating the merge logic.
func mergeCredentialGuardClaudeHooks(root map[string]interface{}, scriptPath string) {
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}
	hooks["PreToolUse"] = mergeClaudeHookArray(hooks["PreToolUse"], "Bash", scriptPath)
	hooks["PostToolUse"] = mergeClaudeHookArray(hooks["PostToolUse"], "Bash", scriptPath)
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
	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

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
		mergeCredentialGuardClaudeHooks(root, scriptPath)
		desired, marshalErr := json.MarshalIndent(root, "", "  ")
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
	mergeCredentialGuardClaudeHooks(root, scriptPath)

	out, marshalErr := json.MarshalIndent(root, "", "  ")
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
	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

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
		mergeCredentialGuardGeminiHooks(root, scriptPath)
		desired, marshalErr := json.MarshalIndent(root, "", "  ")
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
	mergeCredentialGuardGeminiHooks(root, scriptPath)

	out, marshalErr := json.MarshalIndent(root, "", "  ")
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
	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

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
		mergeCredentialGuardCursorHooks(root, scriptPath)
		desired, marshalErr := json.MarshalIndent(root, "", "  ")
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
	mergeCredentialGuardCursorHooks(root, scriptPath)

	out, marshalErr := json.MarshalIndent(root, "", "  ")
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
	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

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
		mergeCredentialGuardCopilotHooks(root, scriptPath)
		desired, marshalErr := json.MarshalIndent(root, "", "  ")
		if marshalErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: marshalErr.Error()}
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
	mergeCredentialGuardCopilotHooks(root, scriptPath)

	out, marshalErr := json.MarshalIndent(root, "", "  ")
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
	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")

	content := map[string]interface{}{
		"version": "v1",
		"hooks": []interface{}{
			map[string]interface{}{
				"name":        "trackfw-credential-guard-global-pre",
				"description": "Blocks/warns on possible plaintext credential materialization before a shell command executes (global, all projects)",
				"trigger":     "PreToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": scriptPath},
			},
			map[string]interface{}{
				"name":        "trackfw-credential-guard-global-post",
				"description": "Warns on possible plaintext credential materialization after a shell command executes (global, all projects)",
				"trigger":     "PostToolUse",
				"matcher":     "shell",
				"action":      map[string]interface{}{"type": "command", "command": scriptPath},
			},
		},
	}
	out, marshalErr := json.MarshalIndent(content, "", "  ")
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
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: mkErr.Error()}
		}
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
func harnessCatalogTarget(catalog *integrations.Catalog, id, tool string, kind integrations.ItemKind, home string, opts UpdateOptions) TargetResult {
	displayPath, pathErr := integrations.GlobalGroupPath(catalog, tool, kind)
	if pathErr != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: "", Message: pathErr.Error()}
	}
	ident, err := identity.Load(home)
	if err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind:     kind,
		Targets:  []string{tool},
		Scope:    "global",
		Identity: ident,
	})
	if err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}

	manager := integrations.Manager{ProjectRoot: home, HomeDir: home}
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
// SETS are not reconcilable byte-for-byte: the Python CLI intentionally
// implements a reduced project-scope surface (agent rules + hooks + Codex
// project agents only — see pypi/trackfw/commands/update.py's own
// docstring, which points users to the Go/Node.js CLIs for CI/git-hooks/
// Claude commands). The four states, four flags and JSON document SHAPE are
// shared; the target ID list is not pinned and is reported here, not
// silently forced into agreement.
// ────────────────────────────────────────────────────────────────────────────

// ProjectTargetIDs is the declared order of `trackfw update` (project scope)
// targets for this runtime. "ci-workflow" and "git-hooks" only appear when
// the project's trackfw.yaml opted into a CI system / hook framework.
func ProjectTargetIDs(cfg Config) []string {
	ids := []string{"agent-rules", "agent-hooks", "codex-project-agents", "validate-script"}
	if cfg.CI != "" && cfg.CI != "none" {
		ids = append(ids, "ci-workflow")
	}
	if cfg.Hooks == "husky" || cfg.Hooks == "lefthook" {
		ids = append(ids, "git-hooks")
	}
	ids = append(ids, "claude-commands")
	return ids
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

	declared := ProjectTargetIDs(cfg)
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
		if cpErr := copyProjectTree(cwd, tmp); cpErr != nil {
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
			".claude/settings.json, .codex/hooks.json, .gemini/settings.json, .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, scripts/trackfw-attention-*.sh, scripts/trackfw-credential-guard.sh",
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
			},
			func(r string) error {
				return withChdir(r, func() error {
					if err := InjectHooksDetected(r); err != nil {
						return err
					}
					if err := GenerateAttentionScripts(""); err != nil {
						return err
					}
					return GenerateCredentialGuardScript("")
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
		return runFileTarget(id, ".github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml", root,
			[]string{".github/workflows/trackfw-gate.yml", ".gitlab-ci-trackfw.yml"},
			func(r string) error { return withChdir(r, func() error { return generateCIWorkflow(cfg) }) },
			opts)
	case "git-hooks":
		relPath := ".husky/pre-commit"
		if cfg.Hooks == "lefthook" {
			relPath = "lefthook.yml"
		}
		return runFileTarget(id, relPath, root, []string{relPath},
			func(r string) error {
				return withChdir(r, func() error { updateHooksSurgical(cfg); return nil })
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
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ident, err := identity.Load(home)
	if err != nil {
		return err
	}
	manager := integrations.Manager{ProjectRoot: root, HomeDir: home}
	for _, kind := range []integrations.ItemKind{integrations.KindAgents, integrations.KindSkills} {
		plans, planErr := integrations.BuildPlans(catalog, integrations.PlanRequest{Kind: kind, Targets: []string{"codex"}, Scope: "project", Identity: ident})
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

// copyProjectTree recursively copies src into dst, skipping .git and
// node_modules (irrelevant to any project-scope target and potentially
// large), for use as a --dry-run sandbox that the real project tree is
// never written through.
func copyProjectTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(target, data, 0644)
	})
}
