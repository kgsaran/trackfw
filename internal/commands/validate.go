package commands

import (
	"fmt"
	"strconv"

	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: i18n.T("validate.description"),
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
					fmt.Println(i18n.T("validate.ok"))
				} else {
					fmt.Println(i18n.T("validate.warnings", "count", strconv.Itoa(len(warnings))))
				}
				return nil
			}
			for _, v := range violations {
				fmt.Printf("✗ %s\n", v)
			}
			return fmt.Errorf("%s", i18n.T("validate.violations", "count", strconv.Itoa(len(violations))))
		},
	}
}
