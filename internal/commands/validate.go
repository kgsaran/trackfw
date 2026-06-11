package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trackfw/trackfw/internal/validator"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check governance consistency (REQ linked, ADR exists, no orphan roadmaps)",
		RunE: func(cmd *cobra.Command, args []string) error {
			violations, err := validator.Validate()
			if err != nil {
				return err
			}
			if len(violations) == 0 {
				fmt.Println("✓ governance is consistent — no violations found.")
				return nil
			}
			for _, v := range violations {
				fmt.Printf("✗ %s\n", v)
			}
			return fmt.Errorf("%d violation(s) found", len(violations))
		},
	}
}
