package integrations

import (
	"fmt"
	"sort"

	"github.com/kgsaran/trackfw/internal/identity"
)

// DoctorFindingKind distinguishes the two states a project's artifacts can be
// in relative to the manifest that this file's ClassifyDoctor reports. They
// require different remedies and must never be merged — see
// docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md
// and ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md.
type DoctorFindingKind string

const (
	// DoctorUnregisteredWrite: on-disk content is byte-identical to what the
	// catalog currently renders, and the manifest has NO entry at all for
	// this destination. This is the state the pre-ADR-2026-08-18 write order
	// could leave behind if interrupted between writing bytes and persisting
	// the manifest: the bytes are trackfw's own, only the record is missing.
	// Remedy is safe — `install --force` adopts, it never overwrites content
	// that differs from what it would write anyway.
	DoctorUnregisteredWrite DoctorFindingKind = "unregistered-write"

	// DoctorHandModified: the manifest has an entry for this destination
	// under this exact claim, but the on-disk hash no longer matches the
	// hash the manifest recorded — someone (or something) edited the file
	// after trackfw wrote it. Remedy overwrites that edit, so it is a human
	// decision, never automatic.
	DoctorHandModified DoctorFindingKind = "hand-modified"
)

// DoctorFinding is one artifact requiring the user's attention, plus a
// ready-to-copy remediation command naming the exact flags to reproduce this
// finding's claim.
type DoctorFinding struct {
	FindingKind DoctorFindingKind `json:"finding"`
	Claim       Claim             `json:"claim"`
	Destination string            `json:"destination"`
	Remedy      string            `json:"remedy"`
}

// ClassifyDoctor separates the two disk/manifest mismatches doctor reports
// from every other lifecycle state. It is intentionally narrow: an artifact
// that is current-and-registered, outdated (handled by `update`),
// not-installed, or unmanaged-with-content-that-does-not-match-the-catalog
// (content that simply is not trackfw's) is never reported here — flagging
// any of those would be the false positive that is this command's dominant
// risk (see the roadmap's "🔴 Risco dominante").
//
// The classification deliberately keys off Inspection.Registered, not
// Inspection.Managed: Managed additionally requires this exact claim to own
// the manifest entry (claimOwned), so a destination registered under a
// *different* claim would read Managed=false while still being registered.
// Treating that as an "unregistered write" would be exactly the dominant
// false-positive this command exists to avoid — it is registered, just not
// by this claim, and is out of scope for doctor either way.
//
// Both classes are hash-based and cannot distinguish "interrupted before the
// optimistic manifest write settled" (ML-1A) from a genuine hand edit when
// Registered=true and the hash differs — the manifest write is optimistic by
// design (see planArtifactWrite's doc comment), so a crash mid-write can
// leave a manifest hash describing content that was never actually written.
// That ambiguity is inherent to a hash-only signal and is left for Wave 3
// (ML-3A) to rule on; ClassifyDoctor implements the table as specified.
func ClassifyDoctor(plans []PlannedArtifact, inspections []Inspection) []DoctorFinding {
	// Non-nil from the start (never `var findings []DoctorFinding`): the zero
	// case must round-trip through `--json` as `[]`, matching Node's
	// `Array.prototype.filter` and Python's `list` — encoding/json renders a
	// nil slice as `null`, which is not what Node/Python ever emit and would
	// be a real cross-CLI divergence on the JSON surface (ML-2B's gate,
	// scripts/check-doctor-parity.sh, exists to catch exactly this).
	findings := []DoctorFinding{}
	for i, plan := range plans {
		if i >= len(inspections) {
			break
		}
		inspection := inspections[i]
		switch {
		case !inspection.Registered && inspection.State == StateCurrent:
			findings = append(findings, DoctorFinding{
				FindingKind: DoctorUnregisteredWrite,
				Claim:       plan.Claim,
				Destination: inspection.Destination,
				Remedy:      doctorRemedy(inspection.Destination, plan.Claim, "adopts it — content already matches the catalog template, only the manifest entry is missing"),
			})
		case inspection.Managed && inspection.State == StateModified:
			findings = append(findings, DoctorFinding{
				FindingKind: DoctorHandModified,
				Claim:       plan.Claim,
				Destination: inspection.Destination,
				Remedy:      doctorRemedy(inspection.Destination, plan.Claim, "overwrites it with the catalog template — you will lose the hand edit"),
			})
		}
	}
	sortDoctorFindings(findings)
	return findings
}

func doctorRemedy(destination string, claim Claim, effect string) string {
	return fmt.Sprintf(
		"trackfw %s install --force --items %s --targets %s --scope %s   # %s: %s",
		claim.Kind, claim.Item, claim.Target, claim.Scope, effect, destination,
	)
}

// sortDoctorFindings orders findings by a total key so the three CLIs can be
// gate-compared byte-for-byte (ML-2B): destination alone is not total when a
// single destination carries more than one claim.
func sortDoctorFindings(findings []DoctorFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Destination != b.Destination {
			return a.Destination < b.Destination
		}
		if a.Claim.Kind != b.Claim.Kind {
			return a.Claim.Kind < b.Claim.Kind
		}
		if a.Claim.Item != b.Claim.Item {
			return a.Claim.Item < b.Claim.Item
		}
		if a.Claim.Target != b.Claim.Target {
			return a.Claim.Target < b.Claim.Target
		}
		if a.Claim.Surface != b.Claim.Surface {
			return a.Claim.Surface < b.Claim.Surface
		}
		return a.Claim.Scope < b.Claim.Scope
	})
}

// RunDoctor sweeps every catalog-managed (kind, target, surface, scope)
// combination and returns every ClassifyDoctor finding across the whole
// catalog. It builds plans per (target, surface) instead of calling
// BuildPlans once with AllSurfaces+no Targets filter, because BuildPlans
// errors when a surface has no path for the requested scope (pathForScope
// returning ok=false is a hard error there, by design for explicit
// install/update requests) — doctor's sweep is expected to hit surfaces that
// simply do not support a given scope, and must skip those, not fail.
func RunDoctor(catalog *Catalog, manager Manager, ident identity.Config) ([]DoctorFinding, error) {
	var allPlans []PlannedArtifact
	for _, kind := range []ItemKind{KindAgents, KindSkills} {
		for _, scope := range []string{"project", "global"} {
			plans, err := doctorPlansForScope(catalog, kind, scope, ident, manager.ProjectRoot)
			if err != nil {
				return nil, err
			}
			allPlans = append(allPlans, plans...)
		}
	}
	inspections, err := manager.List(allPlans)
	if err != nil {
		return nil, err
	}
	return ClassifyDoctor(allPlans, inspections), nil
}

// doctorPlansForScope builds plans for every target×surface of kind that
// both support scope and are not "unsupported" for kind, skipping the rest.
func doctorPlansForScope(catalog *Catalog, kind ItemKind, scope string, ident identity.Config, projectRoot string) ([]PlannedArtifact, error) {
	var result []PlannedArtifact
	for _, target := range catalog.Targets {
		for _, surface := range target.Surfaces {
			capability := surface.Capabilities.Agents
			paths := surface.Paths.Agents
			if kind == KindSkills {
				capability = surface.Capabilities.Skills
				paths = surface.Paths.Skills
			}
			if capability.SupportLevel == "unsupported" {
				continue
			}
			if _, ok := pathForScope(paths, scope); !ok {
				continue
			}
			plans, err := BuildPlans(catalog, PlanRequest{
				Kind:        kind,
				Targets:     []string{target.ID},
				Scope:       scope,
				Surfaces:    map[string]string{target.ID: surface.ID},
				Identity:    ident,
				ProjectRoot: projectRoot,
			})
			if err != nil {
				return nil, err
			}
			result = append(result, plans...)
		}
	}
	return result, nil
}
