package commands

import (
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Manage vault knowledge notes",
	}
	cmd.AddCommand(newNoteNewCmd())
	return cmd
}

func newNoteNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new knowledge note in vault/notes/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.NewNote(args[0])
		},
	}
}
