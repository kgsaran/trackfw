package commands

import (
	"fmt"

	"github.com/kgsaran/trackfw/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print trackfw version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("trackfw", version.Version)
		},
	}
}
