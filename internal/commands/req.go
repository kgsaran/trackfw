package commands

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newReqCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "req",
		Short: "Manage Requirements",
	}
	cmd.AddCommand(newReqNewCmd())
	cmd.AddCommand(newReqListCmd())
	return cmd
}

func newReqNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new REQ",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := generators.REQContent{Title: args[0]}

			// Detectar se stdin é TTY — wizard interativo somente em TTY
			if cbterm.IsTerminal(uintptr(os.Stdin.Fd())) {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewText().
							Title("Motivation").
							Description("Why is this requirement needed? What problem does it solve?").
							Value(&content.Motivation),
						huh.NewText().
							Title("Acceptance Criteria").
							Description("List acceptance criteria, one per line").
							Value(&content.Criteria),
						huh.NewInput().
							Title("Linked ADR").
							Description("ADR filename or slug (leave blank if none)").
							Value(&content.LinkedADR),
						huh.NewInput().
							Title("Linked Roadmap").
							Description("Roadmap filename or slug (leave blank if none)").
							Value(&content.LinkedRoadmap),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("wizard: %w", err)
				}
			}

			return generators.NewREQ(content)
		},
	}
}

func newReqListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all REQs in docs/req/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.ListREQs("docs/req")
		},
	}
}
