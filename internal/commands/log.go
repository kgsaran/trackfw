package commands

import (
	"bufio"
	"fmt"
	"os"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "log",
		Short: i18n.T("log.description"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLog(tail)
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 20, i18n.T("log.tail"))
	return cmd
}

func runLog(tail int) error {
	logFile := config.Load().RoadmapDir + "/.trackfw-log"
	f, err := os.Open(logFile)
	if os.IsNotExist(err) {
		fmt.Println(i18n.T("log.empty"))
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
