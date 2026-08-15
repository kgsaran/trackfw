package commands

import (
	"fmt"
	"os"

	"github.com/kgsaran/trackfw/internal/changelog"
	"github.com/spf13/cobra"
)

func newChangelogCmd() *cobra.Command {
	var versionFlag string
	var allFlag bool
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Show entries from CHANGELOG.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			content, err := changelog.Read(root)
			if err != nil {
				return err
			}
			if allFlag {
				fmt.Print(content)
				return nil
			}
			sections, err := changelog.ParseSections(content)
			if err != nil {
				return err
			}
			var section changelog.Section
			if versionFlag != "" {
				section, err = changelog.FindVersion(sections, versionFlag)
			} else {
				section, err = changelog.FirstSection(sections)
			}
			if err != nil {
				return err
			}
			fmt.Print(changelog.FormatSection(section))
			return nil
		},
	}
	cmd.Flags().StringVar(&versionFlag, "version", "", "Show a specific version section")
	cmd.Flags().BoolVar(&allFlag, "all", false, "Show the entire CHANGELOG.md")
	return cmd
}
