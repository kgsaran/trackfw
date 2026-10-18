package commands

import (
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"sort"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

// modelsSourceLine constants are the "source:" header printed to stdout before the table (AC5).
// Must be byte-identical across Go, Node.js and Python — ADR-2026-08-23.
const (
	modelsSourceLineGlobal          = "source: ~/.trackfw/trackfw.yaml"
	modelsSourceLineNone            = "source: não configurado"
	modelsSourceLineProjectOnly     = "source: trackfw.yaml do projeto (não vale para escopo global)"
	modelsSourceLineGlobalMalformed = "source: arquivo global malformado"
)

// newAgentModelsCmd returns the "models" subcommand of "trackfw agents". It
// lists, for each catalog agent × target pair, the model identifier that
// "trackfw agents install" or "trackfw agents update" would write to the
// generated artifact's model field. This lets the user verify that their
// agent_models configuration in trackfw.yaml produces the intended result
// without having to inspect generated files (ADR-2026-08-21, ML-2A).
//
// A "—" in the RESOLVED column means the target's artifact format omits the
// model field entirely (e.g. opencode-agent, cli-agent-json, agent-json).
//
// Warnings are emitted to stderr, once per suspect tier, before the table.
// A value is suspect when it is not a bare version string (digits and dots
// only) AND does not start with "claude-": e.g. "4.6-beta" would be written
// literally to the model field, causing the agent to fail at startup with the
// cause two layers away.
func newAgentModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Show the resolved model each agent uses per target",
		Long: `Lists the effective model identifier for each agent × target combination.

The RESOLVED column shows the value that trackfw agents install/update would
write to the generated artifact's model field. A "—" means the target's
artifact format omits the model field entirely (e.g. opencode, amazonq).

Configure model versions via agent_models in trackfw.yaml:

  agent_models:
    sonnet: "4.6"
    opus: "5"

The claude target is the only one where agent_models takes effect; all other
targets (codex, cursor, antigravity, gemini, kiro, etc.) use their own fixed
mapping regardless of agent_models (ADR-2026-08-21 §4).`,
		Args: cobra.NoArgs,
		RunE: executeAgentModels,
	}
}

const modelsNA = "—"

// column widths — must match npm/src/commands/integrations.js and
// pypi/trackfw/integrations/command.py for byte-identical output.
const (
	modelsAgentWidth  = 14
	modelsTierWidth   = 8
	modelsTargetWidth = 12
)

func executeAgentModels(cmd *cobra.Command, _ []string) error { //nolint:cyclop
	cat, err := integrations.LoadCatalog()
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	// AC5 + AC11: read agent_models from the global config (~/.trackfw/trackfw.yaml),
	// not from the cwd singleton. Show origin before the table.
	homeDir, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}
	agentModels, modelsSource := config.LoadGlobalAgentModels(homeDir, cwd)
	if agentModels == nil {
		agentModels = map[string]string{}
	}

	out := cmd.OutOrStdout()

	// Source line (AC5): show origin before the table; advisory to stderr when not resolved.
	switch modelsSource {
	case config.AgentModelsSourceGlobal:
		fmt.Fprintln(out, modelsSourceLineGlobal)
	case config.AgentModelsSourceNone:
		fmt.Fprintln(out, modelsSourceLineNone)
		fmt.Fprintln(os.Stderr, config.GlobalAgentModelsNoneMessage)
	case config.AgentModelsSourceProjectOnly:
		fmt.Fprintln(out, modelsSourceLineProjectOnly)
		fmt.Fprintln(os.Stderr, config.GlobalAgentModelsProjectOnlyMessage)
	case config.AgentModelsSourceGlobalMalformed:
		fmt.Fprintln(out, modelsSourceLineGlobalMalformed)
		fmt.Fprintln(os.Stderr, config.MalformedGlobalConfigMessage)
	}

	// Warnings: emit once per suspect tier, sorted for deterministic output,
	// before the table so the user sees them before scrolling through rows.
	// Output to stderr so the table (stdout) remains parseable by scripts.
	var suspectTiers []string
	for tier, version := range agentModels {
		if integrations.LooksLikeSuspectModelValue(version) {
			suspectTiers = append(suspectTiers, tier)
		}
	}
	sort.Strings(suspectTiers)
	for _, tier := range suspectTiers {
		fmt.Fprintf(os.Stderr,
			"WARN: agent_models.%s = %q — not a version string and not a claude- model ID; will be written literally and may produce an invalid model identifier\n",
			tier, agentModels[tier],
		)
	}

	// D-1: validate values that the write path would reject; report errors
	// before the table so the user sees them alongside WARN lines. Exit != 0
	// after printing the table so the table is still visible for diagnosis.
	var invalidTiers []string
	for tier, version := range agentModels {
		if err := integrations.ValidateAgentModelValue(tier, version); err != nil {
			invalidTiers = append(invalidTiers, tier)
		}
	}
	sort.Strings(invalidTiers)
	for _, tier := range invalidTiers {
		if validateErr := integrations.ValidateAgentModelValue(tier, agentModels[tier]); validateErr != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v — agents install/update would fail for this tier\n", validateErr)
		}
	}

	fmt.Fprintf(out, "%-*s %-*s %-*s %s\n",
		modelsAgentWidth, "AGENT",
		modelsTierWidth, "TIER",
		modelsTargetWidth, "TARGET",
		"RESOLVED",
	)

	for _, agent := range cat.Agents {
		source, err := cat.ReadAsset(agent)
		if err != nil {
			return fmt.Errorf("reading asset for agent %s: %w", agent.ID, err)
		}
		tier := integrations.AgentTier(source)

		for _, tgt := range cat.Targets {
			surface, ok := integrations.DefaultAgentSurface(tgt)
			if !ok {
				continue
			}
			resolved, present, resolveErr := integrations.ResolveAgentModel(
				tier,
				surface.Capabilities.Agents.Representation,
				tgt.ID,
				agentModels,
			)
			var display string
			switch {
			case resolveErr != nil:
				// Already reported above as ERROR; use a marker in the table.
				display = "[invalid]"
			case !present:
				display = modelsNA
			default:
				display = resolved
			}
			fmt.Fprintf(out, "%-*s %-*s %-*s %s\n",
				modelsAgentWidth, agent.ID,
				modelsTierWidth, tier,
				modelsTargetWidth, tgt.ID,
				display,
			)
		}
	}

	// D-1: exit != 0 when any tier has an invalid value — the display command
	// must not report a different outcome than install would. The table is
	// already printed above so the user can see the [invalid] rows for context.
	if len(invalidTiers) > 0 {
		return fmt.Errorf("agent_models contém %d valor(es) inválido(s); corrija trackfw.yaml e reexecute", len(invalidTiers))
	}
	return nil
}
