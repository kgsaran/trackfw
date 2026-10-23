package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/discover"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/i18n"
	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/kgsaran/trackfw/internal/translog"
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
			// resolvedCwd is the EvalSymlinks-resolved form of cwd used as the root
			// argument to pathguard. On macOS, os.Getwd() may return /tmp/... while
			// EvalSymlinks resolves to /private/tmp/... — passing the unresolved path
			// as root causes false "escapes root" errors for files genuinely inside
			// the project. Falls back to cwd if EvalSymlinks fails.
			//
			// ML-8A / #402: this used to be `resolvedCwd := cwd` followed by a
			// conditional reassignment. Behaviourally identical, but the two-step
			// shape BINDS resolvedCwd to cwd, so the containment analyser's P2 read
			// this — the very site the fix's exemplar was quoted from — as an
			// unresolved guard root. One assignment from one resolver is what makes
			// the property mechanical instead of a comment.
			resolvedCwd, rootErr := pathguard.ResolveRoot(cwd)
			if rootErr != nil {
				return rootErr
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

			// .trackfw-log / .trackfw-log-entries (ML-1A: new writes go to entries dir)
			if !r.HasTrackfwLog {
				fmt.Fprintln(out, "⚠ No .trackfw-log or .trackfw-log-entries — run with --bootstrap-log to create retroactive history")
			} else {
				fmt.Fprintln(out, "✓ transition log found")
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

			// suggested test framework — printed only, never written to trackfw.yaml
			// automatically (agent_conventions must always be declared by the team).
			if r.SuggestedTestFramework != "" {
				fmt.Fprintf(out, "Suggested test framework: %s (add to trackfw.yaml as agent_conventions: if correct)\n", r.SuggestedTestFramework)
			}

			fmt.Fprintf(out, "\nGovernance Score: %d/100\n", r.GovernanceScore)

			// --init
			if flagInit {
				yamlPath := filepath.Join(resolvedCwd, "trackfw.yaml")
				if _, statErr := os.Stat(yamlPath); statErr == nil {
					// arquivo já existe — avisar e sair sem sobrescrever
					fmt.Fprintln(out, "\n⚠ trackfw.yaml already exists — remove it first if you want to regenerate")
					return nil
				}
				yaml := discover.GenerateYAML(r)
				if err := rejectDiscoverPath(resolvedCwd, yamlPath); err != nil {
					return err
				}
				if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil { // write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectDiscoverPath above
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
				if err := generators.InjectHooksDetected(cwd); err != nil {
					fmt.Fprintf(out, "  ⚠ agent hooks: %v\n", err)
				} else {
					fmt.Fprintln(out, "  ✓ agent hooks configurados")
					generators.PrintArchitectNextSteps(cwd)
				}
			}

			// --bootstrap-log
			if flagBootstrapLog {
				if r.RoadmapDir == "" {
					return fmt.Errorf("no roadmap dir detected — cannot bootstrap log")
				}

				roadmapDir := filepath.Join(resolvedCwd, r.RoadmapDir)
				dotDPath := filepath.Join(roadmapDir, translog.EntriesDirSuffix[1:])

				// Guard the .d/ directory before writing any entries.
				if err := rejectDiscoverPath(resolvedCwd, dotDPath); err != nil {
					return err
				}

				// Build a dedup set from existing transitions (legacy + .d/).
				existingEntries := readLogEntries(r.RoadmapDir)

				// gerar novas entradas
				newContent := discover.GenerateBootstrapLog(r, cwd)
				lines := strings.Split(newContent, "\n")

				written := 0
				for _, line := range lines {
					line = strings.TrimRight(line, "\r\n")
					if line == "" {
						continue
					}
					key := dedupKey(line)
					if existingEntries[key] {
						continue
					}
					// Parse the retroactive timestamp from the line (format: "2006-01-02 15:04  ...").
					var entryTime time.Time
					if len(line) >= 16 {
						if ts, parseErr := time.ParseInLocation("2006-01-02 15:04", line[:16], time.Local); parseErr == nil {
							entryTime = ts
						}
					}
					if entryTime.IsZero() {
						entryTime = time.Now()
					}
					// WriteEntry guards itself via pathguard.RejectAndReport(resolvedRoot, entryPath);
					// rejectDiscoverPath above serves as an early-exit before the loop.
					if _, writeErr := translog.WriteEntry(resolvedCwd, r.RoadmapDir, line, entryTime); writeErr != nil {
						return fmt.Errorf("bootstrap log: write entry: %w", writeErr)
					}
					written++
				}

				fmt.Fprintf(out, "✓ bootstrap log written to %s/%s (%d new entries)\n", r.RoadmapDir, translog.EntriesDirSuffix[1:], written)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flagInit, "init", false, "generate trackfw.yaml calibrated for this project")
	cmd.Flags().BoolVar(&flagBootstrapLog, "bootstrap-log", false, "create retroactive .trackfw-log from done/ files")

	return cmd
}

// rejectDiscoverPath guards a path inside root before any write.
// It delegates to pathguard.RejectAndReport, which is the single site in the
// binary that emits the containment refusal (ML-7B).
func rejectDiscoverPath(root, absTarget string) error {
	// Thin adapter over pathguard.RejectAndReport: it must NOT re-wrap the error
	// (the message already names the refused path) and must NOT print — the single
	// emission point lives in pathguard (ML-7B).
	return pathguard.RejectAndReport(root, absTarget)
}

// readLogEntries lê o log de transições (legacy + .d/) e retorna um set de chaves de dedup.
// The key is the full line reconstructed from the Transition fields, so that bootstrap lines
// already written via translog.WriteEntry are not re-written.
func readLogEntries(roadmapDir string) map[string]bool {
	entries := make(map[string]bool)
	tls, err := translog.ReadAll(roadmapDir)
	if err != nil {
		return entries
	}
	for _, t := range tls {
		// Reconstruct the line the same way GenerateBootstrapLog does it so the dedup
		// key matches. The time format and field separator must be identical.
		line := fmt.Sprintf("%s  %-50s  %s → %s",
			t.Timestamp.Format("2006-01-02 15:04"),
			t.Basename,
			t.From,
			t.To,
		)
		entries[dedupKey(line)] = true
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
