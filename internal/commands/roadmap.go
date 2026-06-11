package commands

import (
	"github.com/spf13/cobra"
	"github.com/trackfw/trackfw/internal/generators"
)

func newRoadmapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roadmap",
		Short: "Manage Roadmaps",
	}
	cmd.AddCommand(newRoadmapNewCmd(), newRoadmapMoveCmd())
	return cmd
}

func newRoadmapNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new roadmap in backlog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.NewRoadmap(args[0])
		},
	}
}

func newRoadmapMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move <name> <state>",
		Short: "Move a roadmap between states (backlog|wip|blocked|done|abandoned)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.MoveRoadmap(args[0], args[1])
		},
	}
}
