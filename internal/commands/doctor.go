package commands

import (
	"encoding/json"
	"fmt"

	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Detect artifacts on disk missing from the manifest, and distinguish them from hand-modified artifacts",
		Long: `trackfw doctor sweeps every catalog-managed agents/skills destination, in
both project and global scope, and reports two distinct disk/manifest
mismatches with different remedies — they are never merged:

  unregistered-write   on-disk content matches the current catalog template
                        exactly, but the manifest has no entry for it. This
                        is what the pre-ADR-2026-08-18 write order could
                        leave behind if interrupted: the bytes are trackfw's
                        own, only the manifest record is missing. Safe to
                        adopt.

  hand-modified         the manifest owns the destination, but its on-disk
                        hash no longer matches what the manifest recorded —
                        the file was edited after trackfw wrote it. Adopting
                        overwrites that edit; it is a human decision, never
                        automatic.

Content that does not match the catalog template and has no manifest entry
is not trackfw's and is never reported — flagging it would be a false
positive. Each finding prints a ready-to-copy remediation command; doctor
never writes anything itself.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			findings, err := runDoctor()
			if err != nil {
				return err
			}
			if jsonOut {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(findings)
			}
			printDoctorReport(cmd, findings)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit findings as a JSON array instead of the text report")
	return cmd
}

func runDoctor() ([]integrations.DoctorFinding, error) {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return nil, err
	}
	manager, err := integrationsManager()
	if err != nil {
		return nil, err
	}
	ident, err := identity.Load(manager.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("doctor: identidade invalida: %w", err)
	}
	return integrations.RunDoctor(catalog, manager, ident)
}

func printDoctorReport(cmd *cobra.Command, findings []integrations.DoctorFinding) {
	out := cmd.OutOrStdout()
	if len(findings) == 0 {
		fmt.Fprintln(out, "trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact.")
		return
	}
	unregistered, handModified := 0, 0
	for _, finding := range findings {
		if finding.FindingKind == integrations.DoctorUnregisteredWrite {
			unregistered++
		} else {
			handModified++
		}
	}
	fmt.Fprintf(out, "trackfw doctor: %d finding(s) -- %d unregistered-write, %d hand-modified\n\n", len(findings), unregistered, handModified)
	for _, finding := range findings {
		fmt.Fprintf(out, "[%s] %s\n  remedy: %s\n\n", finding.FindingKind, finding.Destination, finding.Remedy)
	}
}
