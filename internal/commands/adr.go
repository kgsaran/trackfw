package commands

import (
	"fmt"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
	"os"
)

func newADRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adr",
		Short: "Manage Architecture Decision Records",
	}
	cmd.AddCommand(newADRNewCmd())
	cmd.AddCommand(newADRListCmd())
	return cmd
}

func newADRNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new ADR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := generators.ADRContent{Title: args[0]}

			// Detectar se stdin é TTY — wizard interativo somente em TTY
			if cbterm.IsTerminal(uintptr(os.Stdin.Fd())) {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewText().
							Title("Context").
							Description("What is the situation that motivates this decision?").
							Value(&content.Context),
						huh.NewText().
							Title("Decision").
							Description("What was decided?").
							Value(&content.Decision),
						huh.NewText().
							Title("Consequences").
							Description("What are the positive and negative consequences?").
							Value(&content.Consequences),
						huh.NewText().
							Title("Alternatives Considered").
							Description("What other options were evaluated and why were they rejected?").
							Value(&content.Alternatives),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("wizard: %w", err)
				}
			}

			return generators.NewADR(content)
		},
	}
}

func newADRListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all ADRs in docs/adr/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.ListADRs("docs/adr")
		},
	}
}
