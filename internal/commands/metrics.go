package commands

import (
	"fmt"
	"time"

	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/metrics"
	"github.com/spf13/cobra"
)

func newMetricsCmd() *cobra.Command {
	var since string
	var export string

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: i18n.T("metrics.description"),
		RunE: func(cmd *cobra.Command, args []string) error {
			logPath := "docs/roadmaps/.trackfw-log"

			transitions, err := metrics.ParseLog(logPath)
			if err != nil {
				return err
			}
			if len(transitions) == 0 {
				fmt.Println(i18n.T("metrics.no_data"))
				return nil
			}

			if since != "" {
				d, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since format (use: 7d, 30d, 90d): %w", err)
				}
				transitions = metrics.Filter(transitions, time.Now().Add(-d))
				if len(transitions) == 0 {
					fmt.Println(i18n.T("metrics.no_data"))
					return nil
				}
			}

			m := metrics.Calculate(transitions)
			printMetrics(m)

			if export != "" {
				if err := metrics.ExportCSV(m, transitions, export); err != nil {
					return err
				}
				fmt.Printf("exported to %s\n", export)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Filter by period (e.g. 7d, 30d, 90d)")
	cmd.Flags().StringVar(&export, "export", "", "Export metrics to CSV file")
	return cmd
}

// parseSinceDuration converte strings como "7d", "30d", "90d" em time.Duration.
func parseSinceDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("formato inválido: %q", s)
	}
	unit := s[len(s)-1]
	if unit != 'd' {
		return 0, fmt.Errorf("unidade não suportada %q (use 'd' para dias)", string(unit))
	}
	days, err := parsePositiveInt(s[:len(s)-1])
	if err != nil {
		return 0, fmt.Errorf("número inválido em %q: %w", s, err)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

// parsePositiveInt converte string em int positivo.
func parsePositiveInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("caractere não numérico: %q", c)
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return 0, fmt.Errorf("valor deve ser positivo")
	}
	return n, nil
}

// printMetrics imprime as métricas em formato tabela ASCII.
func printMetrics(m metrics.Metrics) {
	fmt.Println("── trackfw metrics ──────────────────────")

	// Cycle Time Mean
	if m.CycleTimeMean > 0 {
		d := m.CycleTimeMean
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if days > 0 {
			fmt.Printf("  Cycle Time Mean   : %d days %d hours\n", days, hours)
		} else {
			fmt.Printf("  Cycle Time Mean   : %d hours\n", hours)
		}
	} else {
		fmt.Println("  Cycle Time Mean   : n/a (no completed cycles)")
	}

	// Throughput
	if m.Throughput > 0 {
		fmt.Printf("  Throughput        : %.2f roadmaps/week\n", m.Throughput)
	} else {
		fmt.Println("  Throughput        : n/a (no completed roadmaps)")
	}

	// WIP Age
	if len(m.WIPEntries) == 0 {
		fmt.Println("  WIP Age           : no items in progress")
	} else {
		fmt.Printf("  WIP Age (%d items) :\n", len(m.WIPEntries))
		for _, w := range m.WIPEntries {
			days := int(w.Age.Hours()) / 24
			hours := int(w.Age.Hours()) % 24
			if days > 0 {
				fmt.Printf("    - %s: %d days %d hours\n", w.Basename, days, hours)
			} else {
				fmt.Printf("    - %s: %d hours\n", w.Basename, hours)
			}
		}
	}

	fmt.Println("─────────────────────────────────────────")
}
