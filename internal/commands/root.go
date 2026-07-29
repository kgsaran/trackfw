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

	root.SetVersionTemplate("trackfw {{.Version}}\n")
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
		newHelpCmd(),
		newConfigureCmd(),
		newVersionCmd(),
		newLogCmd(),
		newPluginsCmd(),
		NewDiscoverCmd(),
		newServeCmd(),
		newMetricsCmd(),
		newSyncCmd(),
		newContextCmd(),
		newNoteCmd(),
		newShipCmd(),
		newBarrierCmd(),
	)

	root.Args = cobra.ArbitraryArgs
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunPlugin(args[0], args[1:])
		}
		return cmd.Help()
	}

	return root
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
