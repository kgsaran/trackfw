package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/kgsaran/trackfw/internal/validator"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check governance consistency (REQ linked, ADR exists, no orphan roadmaps)",
		RunE: func(cmd *cobra.Command, args []string) error {
			violations, warnings, err := validator.Validate()
			if err != nil {
				return err
			}
			for _, w := range warnings {
				fmt.Printf("⚠  %s\n", w)
			}
			if len(violations) == 0 {
				if len(warnings) == 0 {
					fmt.Println("✓ governance is consistent — no violations found.")
				} else {
					fmt.Printf("✓ no violations — %d warning(s)\n", len(warnings))
				}
				return nil
			}
			for _, v := range violations {
				fmt.Printf("✗ %s\n", v)
			}
			return fmt.Errorf("%d violation(s) found", len(violations))
		},
	}
}
