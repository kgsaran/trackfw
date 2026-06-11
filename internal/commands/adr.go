package commands

import (
	"github.com/spf13/cobra"
	"github.com/trackfw/trackfw/internal/generators"
)

func newADRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adr",
		Short: "Manage Architecture Decision Records",
	}
	cmd.AddCommand(newADRNewCmd())
	return cmd
}

func newADRNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new ADR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.NewADR(args[0])
		},
	}
}
