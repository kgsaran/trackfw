package commands

import (
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/serve"
	"github.com/spf13/cobra"
)

// ServeHostFlagHelp is the pinned, byte-identical --host help text shared by
// all three runtimes (Go, Node.js, Python).
const ServeHostFlagHelp = "Host to bind to (loopback only by default; use 0.0.0.0 to expose on the network)"

func newServeCmd() *cobra.Command {
	var port int
	var host string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: i18n.T("serve.description"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// serve.Start prints the listening line itself, only after
			// the bind succeeds — see internal/serve/serve.go.
			return serve.Start(port, host)
		},
	}
	cmd.Flags().IntVar(&port, "port", 4080, "Port to listen on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", ServeHostFlagHelp)
	return cmd
}
