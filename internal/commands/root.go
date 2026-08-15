package commands

import (
	"fmt"
	"os"

	trackversion "github.com/kgsaran/trackfw/internal/version"
	"github.com/spf13/cobra"
)

// newRootCmd builds the full trackfw command tree. It is extracted from
// Execute so tests can inspect the real, registered subcommand set (e.g. to
// prove a command was removed) without depending on os.Exit side effects.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "trackfw",
		Short: "trackfw — governed software delivery framework",
		Long: `trackfw enforces a traceable delivery chain:
ADR → REQ → ROADMAP → backlog/wip/done

Run 'trackfw init' to set up governance in your project.`,
		Version: trackversion.Version,
	}

	helpCmd := newHelpCmd()

	root.SetVersionTemplate("trackfw {{.Version}}\n")

	// Pré-registra a flag "version" sem shorthand, impedindo que o cobra
	// registre automaticamente "--version / -v" via InitDefaultVersionFlag.
	// O cobra só adiciona a flag padrão quando Flags().Lookup("version") == nil,
	// portanto esta declaração reserva o slot sem o atalho -v.
	// Motivação: -v/-−verbose é reservado para modo verboso futuro (cli-parity.md).
	root.Flags().Bool("version", false, "version for trackfw")

	root.AddCommand(
		newInitCmd(),
		newUpdateCmd(),
		newSkillsCmd(),
		newAgentsCmd(),
		newADRCmd(),
		newReqCmd(),
		newRoadmapCmd(),
		newStatusCmd(),
		newValidateCmd(),
		newBaselineCmd(),
		helpCmd,
		newConfigureCmd(),
		newVersionCmd(),
		newLogCmd(),
		NewDiscoverCmd(),
		newServeCmd(),
		newMetricsCmd(),
		newSyncCmd(),
		newContextCmd(),
		newNoteCmd(),
		newShipCmd(),
		newBarrierCmd(),
		newBranchCmd(),
		newCommitCmd(),
		newChangelogCmd(),
	)

	// trackfw expõe uma única superfície explícita de ajuda ("help").
	// Sem isto, cobra registra seu próprio comando "help" default além do
	// nosso, duplicando a entrada em `trackfw --help` (Available Commands).
	root.SetHelpCommand(helpCmd)

	return root
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
