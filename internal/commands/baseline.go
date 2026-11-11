package commands

import (
	"fmt"

	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

func newBaselineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "baseline",
		Short: "Save a snapshot of current violations to .trackfw-baseline.json",
		Long: `Runs all validations and saves the result as a baseline.
Subsequent validate runs will only report violations new relative to the baseline.
Commit .trackfw-baseline.json to the repository to document accepted technical debt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			violations, warnings, err := validator.ValidateUnfiltered()
			if err != nil {
				return err
			}
			if err := validator.SaveBaseline(violations, warnings); err != nil {
				return fmt.Errorf("error saving baseline: %w", err)
			}
			fmt.Printf("Baseline saved: %d violations, %d warnings\n",
				len(violations), len(warnings))
			return nil
		},
	}
}
