package commands

import (
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newContextCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Print governance context for LLM consumption",
		Long: `Collect ADRs, REQs and Roadmaps from the project, run validate,
compute a governance score and print the result in Markdown (default) or JSON format.

Useful as system-prompt context for AI agents:
  trackfw context --format=md | pbcopy
  trackfw context --format=json > context.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.GetContext(format)
		},
	}

	cmd.Flags().StringVar(&format, "format", "md", "Output format: md or json")

	return cmd
}
