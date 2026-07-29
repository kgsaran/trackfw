package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
)

// ReadUpdateConfig lê hooks/ci/backend/frontend/pkg_manager de trackfw.yaml.
// Sem dependências externas — parse linha a linha.
func ReadUpdateConfig(cwd string) Config {
	data, err := os.ReadFile(filepath.Join(cwd, "trackfw.yaml"))
	if err != nil {
		return Config{}
	}
	cfg := Config{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := splitKVupdate(line)
		if !ok {
			continue
		}
		switch key {
		case "hooks":
			cfg.Hooks = val
		case "ci":
			cfg.CI = val
		case "backend":
			cfg.Backend = val
		case "frontend":
			cfg.Frontend = val
		case "pkg_manager":
			cfg.PkgManager = val
		}
	}
	return cfg
}

func splitKVupdate(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	if ci := strings.Index(val, " #"); ci >= 0 {
		val = strings.TrimSpace(val[:ci])
	}
	return key, val, key != ""
}

// Update re-aplica todos os templates atuais do trackfw ao projeto em cwd.
func Update(cwd string) error {
	if _, err := os.Stat(filepath.Join(cwd, "trackfw.yaml")); err != nil {
		return fmt.Errorf("trackfw.yaml não encontrado — execute trackfw init primeiro")
	}

	cfg := ReadUpdateConfig(cwd)

	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		return fmt.Errorf("não foi possível mudar para %s: %w", cwd, err)
	}
	defer os.Chdir(orig) //nolint:errcheck

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

	if err := generateAttentionScripts(); err != nil {
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

// HarnessTargetIDs is the fixed, declared order of `trackfw update harness`
// targets. Order here is authoritative for both JSON output and iteration —
// it must never be derived from the filesystem or from what happens to be
// installed on a given machine (see docs/cli-parity.md, "targets follows the
// declared target order, not filesystem order").
var HarnessTargetIDs = []string{"claude-skill", "codex-agents", "codex-skills"}

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

	results := make([]TargetResult, 0, len(selected))
	for _, id := range selected {
		switch id {
		case "claude-skill":
			results = append(results, harnessClaudeSkillTarget(home, opts))
		case "codex-agents":
			results = append(results, harnessCodexTarget("codex-agents", "~/.codex/agents", integrations.KindAgents, home, opts))
		case "codex-skills":
			results = append(results, harnessCodexTarget("codex-skills", "~/.agents/skills", integrations.KindSkills, home, opts))
		}
	}
	return UpdateReport{Scope: "harness", DryRun: opts.DryRun, Targets: results}, nil
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

// harnessCodexTarget evaluates (and, unless DryRun, applies) every global-scope
// Codex catalog item of the given kind. Multiple catalog items (one per
// agent/skill) share a single reported target, matching the contract's
// example ("codex-agents" reporting one state for the "~/.codex/agents"
// directory as a whole):
//
//   - Every catalog item currently NOT installed is left alone (default) or
//     installed (--install-missing); it never turns the whole target into
//     "failed" merely for being absent.
//   - If at least one item write fails, the whole target is "failed".
//   - Else if at least one item was installed or brought current, "updated".
//   - Else if nothing at all is installed, "missing".
//   - Else (everything installed and already current), "skipped".
func harnessCodexTarget(id, displayPath string, kind integrations.ItemKind, home string, opts UpdateOptions) TargetResult {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}
	ident, err := identity.Load(home)
	if err != nil {
		return TargetResult{ID: id, State: TargetFailed, Path: displayPath, Message: err.Error()}
	}
	plans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
		Kind:     kind,
		Targets:  []string{"codex"},
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
