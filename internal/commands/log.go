package commands

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show roadmap state transition history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLog(tail)
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 20, "Number of recent transitions to show")
	return cmd
}

func runLog(tail int) error {
	f, err := os.Open("docs/roadmaps/.trackfw-log")
	if os.IsNotExist(err) {
		fmt.Println("No transitions recorded yet.")
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := 0
	if len(lines) > tail {
		start = len(lines) - tail
	}

	fmt.Println("── trackfw log ─────────────────────────")
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	return nil
}
