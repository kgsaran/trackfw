package commands

import (
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

func newAgentsCmd() *cobra.Command {
	return newIntegrationsLifecycleCmd(integrations.KindAgents)
}
