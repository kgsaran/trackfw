package commands

import (
	"fmt"

	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/serve"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: i18n.T("serve.description"),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("trackfw governance server running at http://localhost:%d\n", port)
			fmt.Println("Press Ctrl+C to stop.")
			return serve.Start(port)
		},
	}
	cmd.Flags().IntVar(&port, "port", 4080, "Port to listen on")
	return cmd
}
