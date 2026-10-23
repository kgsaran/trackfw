package commands

import (
	"fmt"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/translog"
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
	dir := config.Load().RoadmapDir
	transitions, err := translog.ReadAll(dir)
	if err != nil {
		return err
	}
	if len(transitions) == 0 {
		fmt.Println(i18n.T("log.empty"))
		return nil
	}

	start := 0
	if len(transitions) > tail {
		start = len(transitions) - tail
	}

	fmt.Println("── trackfw log ─────────────────────────")
	for _, t := range transitions[start:] {
		fmt.Printf("%s  %-50s  %s → %s\n",
			t.Timestamp.Format("2006-01-02 15:04"),
			t.Basename,
			t.From,
			t.To,
		)
	}
	return nil
}
