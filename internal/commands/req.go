package commands

import (
	"github.com/spf13/cobra"
	"github.com/kgsaran/trackfw/internal/generators"
)

func newReqCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "req",
		Short: "Manage Requirements",
	}
	cmd.AddCommand(newReqNewCmd())
	return cmd
}

func newReqNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new REQ",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.NewREQ(args[0])
		},
	}
}
