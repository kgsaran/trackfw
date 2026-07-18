package commands

import (
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	return newIntegrationsLifecycleCmd(integrations.KindSkills)
}
