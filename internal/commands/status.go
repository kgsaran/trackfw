package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trackfw/trackfw/internal/validator"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current governance state (wip, blocked, recent done)",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := validator.GetStatus()
			if err != nil {
				return err
			}
			fmt.Print(state)
			return nil
		},
	}
}
