package generators

import (
	"crypto/sha256"
	"encoding/hex"
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
// for each of these nine catalog.json targets, in this exact order.
var harnessCatalogTargetOrder = []string{
	"claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "kiro",
}

// HarnessTargetIDs is the fixed, declared order of `trackfw update harness`
// targets: 19 ids — "claude-skill", then "<tool>-agents" and "<tool>-skills"
// for each catalog target in harnessCatalogTargetOrder. Order here is
// authoritative for both JSON output and iteration — it must never be
// derived from the filesystem or from what happens to be installed on a
// given machine (see docs/cli-parity.md, "targets follows the declared
// target order, not filesystem order").
var HarnessTargetIDs = buildHarnessTargetIDs()

func buildHarnessTargetIDs() []string {
	ids := make([]string, 0, 1+2*len(harnessCatalogTargetOrder))
	ids = append(ids, "claude-skill")
	for _, tool := range harnessCatalogTargetOrder {
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
			".claude/settings.json, .codex/hooks.json, .gemini/settings.json, .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, scripts/trackfw-attention-*.sh",
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
			},
			func(r string) error {
				return withChdir(r, func() error {
					if err := InjectHooksDetected(r); err != nil {
						return err
					}
					return GenerateAttentionScripts("")
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
