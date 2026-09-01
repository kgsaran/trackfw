package commands

import (
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
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
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new ADR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := cmd.Flags().GetString("scope")
			if err != nil {
				return err
			}
			adrDir, err := resolveADRDir(scope)
			if err != nil {
				return err
			}

			content := generators.ADRContent{Title: args[0]}

			// Detectar se stdin é TTY — wizard interativo somente em TTY
			if cbterm.IsTerminal(uintptr(os.Stdin.Fd())) {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Context").
							Description("What is the situation that motivates this decision?").
							Value(&content.Context),
					),
					huh.NewGroup(
						huh.NewInput().
							Title("Decision").
							Description("What was decided?").
							Value(&content.Decision),
					),
					huh.NewGroup(
						huh.NewInput().
							Title("Consequences").
							Description("What are the positive and negative consequences?").
							Value(&content.Consequences),
					),
					huh.NewGroup(
						huh.NewInput().
							Title("Alternatives Considered").
							Description("What other options were evaluated and why were they rejected?").
							Value(&content.Alternatives),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("wizard: %w", err)
				}
			}

			return generators.NewADR(content, adrDir)
		},
	}
	cmd.Flags().String("scope", "project", "ADR scope: project (docs/adr, default) or global (~/.trackfw/adr, cross-project)")
	return cmd
}

func newADRListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all ADRs in docs/adr/",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := cmd.Flags().GetString("scope")
			if err != nil {
				return err
			}
			adrDir, err := resolveADRDir(scope)
			if err != nil {
				return err
			}
			return generators.ListADRs(adrDir)
		},
	}
	cmd.Flags().String("scope", "project", "ADR scope: project (docs/adr, default) or global (~/.trackfw/adr, cross-project)")
	return cmd
}

// resolveADRDir resolves the ADR directory to use based on --scope. "project" (default)
// reads adr_dirs[0] from trackfw.yaml, same as before this flag existed. "global" resolves
// to ~/.trackfw/adr without requiring trackfw.yaml/a project root in the cwd — same pattern
// as UpdateHarness, which never requires a project.
func resolveADRDir(scope string) (string, error) {
	switch scope {
	case "project", "":
		return config.Load().ADRDirs[0], nil
	case "global":
		home, err := homedir.Dir()
		if err != nil {
			return "", fmt.Errorf("localizando home dir: %w", err)
		}
		return generators.GlobalADRDir(home), nil
	default:
		return "", fmt.Errorf("--scope inválido: %q (use \"project\" ou \"global\")", scope)
	}
}
