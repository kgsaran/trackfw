package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/discover"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/spf13/cobra"
)

func NewDiscoverCmd() *cobra.Command {
	var flagInit bool
	var flagBootstrapLog bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: i18n.T("discover.description"),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "trackfw discover — scanning %s\n\n", cwd)

			r, err := discover.Scan(cwd)
			if err != nil {
				return fmt.Errorf("scanning: %w", err)
			}

			// ADR dirs
			if r.ADRCount > 0 {
				dirs := ""
				for i, d := range r.ADRDirs {
					if i > 0 {
						dirs += ", "
					}
					dirs += d
				}
				fmt.Fprintf(out, "✓ ADRs found:      %-4d  (%s)\n", r.ADRCount, dirs)
			} else {
				fmt.Fprintln(out, "⚠ No ADRs found")
			}

			// REQ dir
			if r.REQCount > 0 {
				fmt.Fprintf(out, "✓ REQs found:      %-4d  (%s)\n", r.REQCount, r.REQDir)
			} else {
				fmt.Fprintln(out, "⚠ No REQs found")
			}

			// Roadmaps
			if r.RoadmapCount > 0 {
				mode := r.RoadmapNamespacing
				if mode == "by_agent" {
					mode = "by_agent mode"
				}
				fmt.Fprintf(out, "✓ Roadmaps found:  %-4d  (%s — %s)\n", r.RoadmapCount, r.RoadmapDir, mode)
			} else {
				fmt.Fprintln(out, "⚠ No roadmaps found")
			}

			// Agents
			if len(r.Agents) > 0 {
				agents := ""
				for i, a := range r.Agents {
					if i > 0 {
						agents += ", "
					}
					agents += a
				}
				fmt.Fprintf(out, "✓ Agents detected: %s\n", agents)
			}

			// trackfw.yaml
			if !r.HasTrackfwYAML {
				fmt.Fprintln(out, "⚠ No trackfw.yaml — run with --init to generate one")
			} else {
				fmt.Fprintln(out, "✓ trackfw.yaml found")
			}

			// .trackfw-log
			if !r.HasTrackfwLog {
				fmt.Fprintln(out, "⚠ No .trackfw-log — run with --bootstrap-log to create retroactive history")
			} else {
				fmt.Fprintln(out, "✓ .trackfw-log found")
			}

			// hooks
			if r.HookFramework != "none" {
				fmt.Fprintf(out, "✓ Hooks: %s\n", r.HookFramework)
			} else {
				fmt.Fprintln(out, "⚠ No hook framework detected")
			}

			// CI
			if r.CISystem != "none" {
				fmt.Fprintf(out, "✓ CI: %s\n", r.CISystem)
			} else {
				fmt.Fprintln(out, "⚠ No CI system detected")
			}

			fmt.Fprintf(out, "\nGovernance Score: %d/100\n", r.GovernanceScore)

			// --init
			if flagInit {
				yamlPath := filepath.Join(cwd, "trackfw.yaml")
				if _, statErr := os.Stat(yamlPath); statErr == nil {
					// arquivo já existe — avisar e sair sem sobrescrever
					fmt.Fprintln(out, "\n⚠ trackfw.yaml already exists — remove it first if you want to regenerate")
					return nil
				}
				yaml := discover.GenerateYAML(r)
				if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
					return fmt.Errorf("writing trackfw.yaml: %w", err)
				}
				fmt.Fprintln(out, "\n✓ trackfw.yaml generated")
				if err := discover.InstallGates(r, cwd, out); err != nil {
					fmt.Fprintf(out, "⚠ gates install partial: %v\n", err)
				} else {
					fmt.Fprintln(out, "✓ governance gates installed")
				}
				if err := generators.InjectRulesDetected(cwd); err != nil {
					fmt.Fprintf(out, "⚠ agent rules inject partial: %v\n", err)
				} else {
					fmt.Fprintln(out, "✓ trackfw rules injected into agent config files")
				}
			}

			// --bootstrap-log
			if flagBootstrapLog {
				if r.RoadmapDir == "" {
					return fmt.Errorf("no roadmap dir detected — cannot bootstrap log")
				}

				logPath := filepath.Join(cwd, r.RoadmapDir, ".trackfw-log")

				// ler entradas já existentes para dedup
				existingEntries := readLogEntries(logPath)

				// gerar novas entradas
				newContent := discover.GenerateBootstrapLog(r, cwd)
				lines := strings.Split(newContent, "\n")

				f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return fmt.Errorf("opening log: %w", err)
				}
				defer f.Close()

				written := 0
				for _, line := range lines {
					if line == "" {
						continue
					}
					// extrair chave de dedup: data + filename (sem o timestamp de hora)
					key := dedupKey(line)
					if existingEntries[key] {
						continue
					}
					fmt.Fprintln(f, line)
					written++
				}

				fmt.Fprintf(out, "✓ bootstrap log written to %s (%d new entries)\n", logPath, written)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flagInit, "init", false, "generate trackfw.yaml calibrated for this project")
	cmd.Flags().BoolVar(&flagBootstrapLog, "bootstrap-log", false, "create retroactive .trackfw-log from done/ files")

	return cmd
}

// readLogEntries lê o arquivo de log e retorna um set de chaves de dedup.
func readLogEntries(logPath string) map[string]bool {
	entries := make(map[string]bool)
	f, err := os.Open(logPath)
	if err != nil {
		return entries
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			entries[dedupKey(line)] = true
		}
	}
	return entries
}

// dedupKey extrai a chave de deduplicação de uma linha de log.
// O formato gerado por GenerateBootstrapLog é:
//
//	"2006-01-02 15:04  filename.md  backlog → done"
//
// A chave usada é a linha inteira (trim), pois o conteúdo representa
// o mesmo arquivo/data independentemente da hora exata.
func dedupKey(line string) string {
	return strings.TrimSpace(line)
}
