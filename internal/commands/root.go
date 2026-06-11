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
		newADRCmd(),
		newReqCmd(),
		newRoadmapCmd(),
		newStatusCmd(),
		newValidateCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
