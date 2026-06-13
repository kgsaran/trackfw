package commands

import (
	"fmt"

	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

func newBaselineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "baseline",
		Short: "Grava snapshot das violations atuais em .trackfw-baseline.json",
		Long: `Executa todas as validações e salva o resultado como baseline.
O validate subsequente reportará somente violations novas em relação ao baseline.
Commite .trackfw-baseline.json no repositório para documentar o passivo aceito.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			violations, warnings, err := validator.ValidateUnfiltered()
			if err != nil {
				return err
			}
			if err := validator.SaveBaseline(violations, warnings); err != nil {
				return fmt.Errorf("erro ao salvar baseline: %w", err)
			}
			fmt.Printf("Baseline gravado: %d violations, %d warnings\n",
				len(violations), len(warnings))
			return nil
		},
	}
}
