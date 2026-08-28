package commands

import (
	"encoding/json"
	"fmt"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Detect artifacts on disk missing from the manifest, distinguishing hand-modified artifacts from unknown content",
		Long: `trackfw doctor sweeps every catalog-managed agents/skills destination, in
both project and global scope, and reports three distinct disk/manifest
mismatches with different remedies — they are never merged:

  unregistered-write   on-disk content matches the current catalog template
                        exactly, but the manifest has no entry for it. This
                        is what the pre-ADR-2026-08-18 write order could
                        leave behind if interrupted: the bytes are trackfw's
                        own, only the manifest record is missing. Safe to
                        adopt.

  unknown-content       on-disk content matches neither the catalog template
                        NOR any manifest entry. This state is genuinely
                        ambiguous: it could be a file that is not trackfw's
                        at all occupying a catalog destination, or an
                        orphaned trackfw artifact whose bytes drifted once
                        the catalog moved on. It is exactly the state that
                        makes "agents install" refuse this destination with
                        "unmanaged artifact" — the remedy names that refusal
                        instead of picking a side: remove or move the file
                        if it is yours; if it is trackfw's and it drifted,
                        "install --force" replaces it with the current
                        template.

  hand-modified         the manifest owns the destination, but its on-disk
                        hash no longer matches what the manifest recorded —
                        the file was edited after trackfw wrote it. Adopting
                        overwrites that edit; it is a human decision, never
                        automatic.

A destination registered under a claim other than the one being inspected is
never reported, regardless of its content — that ambiguity belongs to a
different item, not this one, and is out of scope for doctor. Each finding
prints a ready-to-copy remediation command; doctor never writes anything
itself.`,
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
	catalogFindings, err := integrations.RunDoctor(catalog, manager, ident, config.Load().AgentModels)
	if err != nil {
		return nil, err
	}
	// Scaffold coverage (ADR-2026-08-27): compare scaffold artifacts on disk against
	// the templates the current binary would generate, using the project's own
	// trackfw.yaml. No manifest entry is written or read (AC3).
	scaffoldFindings, err := generators.RunScaffoldDoctor(manager.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("doctor: scaffold scan: %w", err)
	}
	return append(catalogFindings, scaffoldFindings...), nil
}

func printDoctorReport(cmd *cobra.Command, findings []integrations.DoctorFinding) {
	out := cmd.OutOrStdout()
	if len(findings) == 0 {
		fmt.Fprintln(out, "trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact and all scaffold templates.")
		return
	}
	// Counted explicitly by kind (never an if/else fallback into the last
	// bucket) so a new class introduced later fails loudly here instead
	// of silently inflating another kind's count.
	var unregistered, handModified, unknownContent, scaffoldDivergent, scaffoldMissing, scaffoldWrongMode int
	for _, finding := range findings {
		switch finding.FindingKind {
		case integrations.DoctorUnregisteredWrite:
			unregistered++
		case integrations.DoctorHandModified:
			handModified++
		case integrations.DoctorUnknownContent:
			unknownContent++
		case integrations.DoctorScaffoldDivergent:
			scaffoldDivergent++
		case integrations.DoctorScaffoldMissing:
			scaffoldMissing++
		case integrations.DoctorScaffoldWrongMode:
			scaffoldWrongMode++
		}
	}
	fmt.Fprintf(out, "trackfw doctor: %d finding(s) -- %d unregistered-write, %d hand-modified, %d unknown-content, %d scaffold-divergent, %d scaffold-missing, %d scaffold-wrong-mode\n\n",
		len(findings), unregistered, handModified, unknownContent, scaffoldDivergent, scaffoldMissing, scaffoldWrongMode)
	// One blank line BETWEEN findings, none trailing after the last one — matches Node's
	// `lines.join('\n').replace(/\n$/, '')` and Python's `"\n".join(lines).rstrip("\n")`
	// (npm/src/commands/doctor.js, pypi/trackfw/commands/doctor.py). A naive per-finding
	// "\n\n" suffix leaves a trailing blank line only Go would emit — a real byte-level
	// divergence on the text surface caught by scripts/check-doctor-parity.sh (ML-2B).
	for i, finding := range findings {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "[%s] %s\n  remedy: %s\n", finding.FindingKind, finding.Destination, finding.Remedy)
	}
}
