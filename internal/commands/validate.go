package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: i18n.T("validate.description"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				// Modo JSON: usa ValidateTagged para propagar rule+file nos RuleItems.
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true

				taggedV, taggedW, err := validator.ValidateTagged()
				if err != nil {
					return err
				}

				result := validator.BuildResultTagged(taggedV, taggedW, validator.IsLenient())
				data, marshalErr := json.Marshal(result)
				if marshalErr != nil {
					return marshalErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))

				if len(taggedV) > 0 {
					return fmt.Errorf("%s", i18n.T("validate.violations", "count", strconv.Itoa(len(taggedV))))
				}
				return nil
			}

			// Modo texto (comportamento original inalterado).
			violations, warnings, err := validator.Validate()
			if err != nil {
				return err
			}

			if validator.IsLenient() {
				until := validator.LenientUntilDate()
				if until != "" {
					fmt.Printf("[LENIENT MODE] %s\n", i18n.T("validate.lenient_mode", "date", until))
				}
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

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output validation result as JSON")
	return cmd
}
