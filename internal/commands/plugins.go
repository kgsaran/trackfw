package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/plugins"
	"github.com/spf13/cobra"
)

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage trackfw plugins",
	}
	cmd.AddCommand(newPluginsListCmd(), newPluginsAddCmd(), newPluginsRemoveCmd())
	return cmd
}

func newPluginsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := plugins.List()
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Println("No plugins installed. Add one with: trackfw plugins add <github-user/repo>")
				return nil
			}
			dir, _ := plugins.Dir()
			fmt.Println("Installed plugins:")
			for _, name := range names {
				fmt.Printf("  %-20s  %s\n", name, filepath.Join(dir, name))
			}
			return nil
		},
	}
}

func newPluginsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <user/repo[@tag]>",
		Short: "Install a plugin from GitHub Releases",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := plugins.Install(args[0]); err != nil {
				return err
			}
			name := filepath.Base(args[0])
			// strip @tag if present
			for i, c := range name {
				if c == '@' {
					name = name[:i]
					break
				}
			}
			fmt.Printf("plugin %s installed\n", name)
			return nil
		},
	}
}

func newPluginsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := plugins.Remove(args[0]); err != nil {
				return err
			}
			fmt.Printf("plugin %s removed\n", args[0])
			return nil
		},
	}
}

// RunPlugin executa um plugin instalado pelo nome, passando os args restantes.
func RunPlugin(name string, args []string) error {
	dir, err := plugins.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("unknown command or plugin: %q", name)
	}
	cmd := exec.Command(path, args...) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
