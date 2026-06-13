package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "trackfw",
	Short: "trackfw — governed software delivery framework",
	Long: `trackfw enforces a traceable delivery chain:
ADR → REQ → ROADMAP → backlog/wip/done

Run 'trackfw init' to set up governance in your project.`,
}

func Execute() {
	rootCmd.AddCommand(
		newInitCmd(),
		newSkillsCmd(),
		newAgentsCmd(),
		newGeminiCmd(),
		newCursorCmd(),
		newCopilotCmd(),
		newWindsurfCmd(),
		newAmazonQCmd(),
		newADRCmd(),
		newReqCmd(),
		newRoadmapCmd(),
		newStatusCmd(),
		newValidateCmd(),
		newBaselineCmd(),
		newVersionCmd(),
		newLogCmd(),
		newPluginsCmd(),
		NewDiscoverCmd(),
		newServeCmd(),
		newMetricsCmd(),
		newSyncCmd(),
		newContextCmd(),
	)

	rootCmd.Args = cobra.ArbitraryArgs
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunPlugin(args[0], args[1:])
		}
		return cmd.Help()
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
