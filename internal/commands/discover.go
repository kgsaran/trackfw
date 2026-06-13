package commands

import (
	"fmt"
	"os"

	"github.com/kgsaran/trackfw/internal/discover"
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
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			fmt.Printf("trackfw discover — scanning %s\n\n", cwd)

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
				fmt.Printf("✓ ADRs found:      %-4d  (%s)\n", r.ADRCount, dirs)
			} else {
				fmt.Println("⚠ No ADRs found")
			}

			// REQ dir
			if r.REQCount > 0 {
				fmt.Printf("✓ REQs found:      %-4d  (%s)\n", r.REQCount, r.REQDir)
			} else {
				fmt.Println("⚠ No REQs found")
			}

			// Roadmaps
			if r.RoadmapCount > 0 {
				mode := r.RoadmapNamespacing
				if mode == "by_agent" {
					mode = "by_agent mode"
				}
				fmt.Printf("✓ Roadmaps found:  %-4d  (%s — %s)\n", r.RoadmapCount, r.RoadmapDir, mode)
			} else {
				fmt.Println("⚠ No roadmaps found")
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
				fmt.Printf("✓ Agents detected: %s\n", agents)
			}

			// trackfw.yaml
			if !r.HasTrackfwYAML {
				fmt.Println("⚠ No trackfw.yaml — run with --init to generate one")
			} else {
				fmt.Println("✓ trackfw.yaml found")
			}

			// .trackfw-log
			if !r.HasTrackfwLog {
				fmt.Println("⚠ No .trackfw-log — run with --bootstrap-log to create retroactive history")
			} else {
				fmt.Println("✓ .trackfw-log found")
			}

			fmt.Printf("\nGovernance Score: %d/100\n", r.GovernanceScore)

			// --init
			if flagInit {
				yaml := discover.GenerateYAML(r)
				if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
					return fmt.Errorf("writing trackfw.yaml: %w", err)
				}
				fmt.Println("\n✓ trackfw.yaml generated")
			}

			// --bootstrap-log
			if flagBootstrapLog {
				if r.RoadmapDir == "" {
					return fmt.Errorf("no roadmap dir detected — cannot bootstrap log")
				}
				logContent := discover.GenerateBootstrapLog(r, cwd)
				logPath := r.RoadmapDir + "/.trackfw-log"
				f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return fmt.Errorf("opening log: %w", err)
				}
				defer f.Close()
				f.WriteString(logContent)
				fmt.Printf("✓ bootstrap log written to %s\n", logPath)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flagInit, "init", false, "generate trackfw.yaml calibrated for this project")
	cmd.Flags().BoolVar(&flagBootstrapLog, "bootstrap-log", false, "create retroactive .trackfw-log from done/ files")

	return cmd
}
