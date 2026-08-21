package integrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

// TestClassifyDoctorDistinguishesTheThreeClasses exercises the seven cases
// the classification must get right without touching disk or the manifest —
// ClassifyDoctor only reads Inspection/PlannedArtifact fields. See the ML-2A
// roadmap entry for why classes 1 and 3 cannot be merged, ML-2C for why
// class 2 exists, and the doc comment on ClassifyDoctor for why Registered
// (not Managed) is the correct signal for classes 1 and 2.
func TestClassifyDoctorDistinguishesTheThreeClasses(t *testing.T) {
	plan := testPlan("project", "/proj/.claude/agents/trackfw-backend.md", "v1", "rendered")

	cases := []struct {
		name       string
		inspection Inspection
		wantKind   DoctorFindingKind
		wantFlag   bool
	}{
		{
			name: "template match, no manifest entry -> unregistered write",
			inspection: Inspection{
				Claim: plan.Claim, Destination: plan.Destination,
				State: StateCurrent, Managed: false, Registered: false,
			},
			wantKind: DoctorUnregisteredWrite, wantFlag: true,
		},
		{
			name: "manifest-owned, hash differs -> hand modified",
			inspection: Inspection{
				Claim: plan.Claim, Destination: plan.Destination,
				State: StateModified, Managed: true, Registered: true,
			},
			wantKind: DoctorHandModified, wantFlag: true,
		},
		{
			// Was "not our problem" / wantFlag: false before ML-2C — this is
			// exactly the state ML-3A's audit found silently falling outside
			// the switch, which is what makes `agents install`'s preflight
			// refuse with "unmanaged artifact". See DoctorUnknownContent's
			// doc comment.
			name: "content matches neither template nor manifest entry -> unknown content",
			inspection: Inspection{
				Claim: plan.Claim, Destination: plan.Destination,
				State: StateModified, Managed: false, Registered: false,
			},
			wantKind: DoctorUnknownContent, wantFlag: true,
		},
		{
			name: "template match, already registered and owned -> nothing to report",
			inspection: Inspection{
				Claim: plan.Claim, Destination: plan.Destination,
				State: StateCurrent, Managed: true, Registered: true,
			},
			wantFlag: false,
		},
		{
			name: "registered under a DIFFERENT claim, content current -> must not be reported as unregistered",
			inspection: Inspection{
				Claim: plan.Claim, Destination: plan.Destination,
				State: StateCurrent, Managed: false, Registered: true,
			},
			wantFlag: false,
		},
		{
			// The unknown-content analogue of the case above: a destination
			// registered under a DIFFERENT claim whose content also
			// mismatches must stay silent too — it is that other claim's
			// concern, not this one's, regardless of State. This is the
			// discriminant Cenário 72 (check-gates-falsify.sh) falsifies for
			// DoctorUnknownContent, mirroring Cenário 71 for
			// DoctorUnregisteredWrite.
			name: "registered under a DIFFERENT claim, content modified -> must not be reported as unknown content",
			inspection: Inspection{
				Claim: plan.Claim, Destination: plan.Destination,
				State: StateModified, Managed: false, Registered: true,
			},
			wantFlag: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := ClassifyDoctor([]PlannedArtifact{plan}, []Inspection{tc.inspection})
			if tc.wantFlag && len(findings) != 1 {
				t.Fatalf("findings = %d, want 1 (%+v)", len(findings), findings)
			}
			if !tc.wantFlag && len(findings) != 0 {
				t.Fatalf("findings = %d, want 0 (%+v)", len(findings), findings)
			}
			if tc.wantFlag {
				if findings[0].FindingKind != tc.wantKind {
					t.Fatalf("finding kind = %q, want %q", findings[0].FindingKind, tc.wantKind)
				}
				if findings[0].Remedy == "" {
					t.Fatal("remedy is empty")
				}
			}
		})
	}
}

// TestClassifyDoctorSortIsTotal proves the sort key is total across a
// destination shared by two claims (ML-2B's gate needs deterministic order
// across three independent CLI implementations).
func TestClassifyDoctorSortIsTotal(t *testing.T) {
	shared := "/proj/.agents/shared.md"
	planA := testPlan("project", shared, "v1", "content")
	planA.Claim.Target = "zzz"
	planB := testPlan("project", shared, "v1", "content")
	planB.Claim.Target = "aaa"

	inspection := func(p PlannedArtifact) Inspection {
		return Inspection{Claim: p.Claim, Destination: p.Destination, State: StateCurrent, Managed: false, Registered: false}
	}
	findings := ClassifyDoctor([]PlannedArtifact{planA, planB}, []Inspection{inspection(planA), inspection(planB)})
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(findings))
	}
	if findings[0].Claim.Target != "aaa" || findings[1].Claim.Target != "zzz" {
		t.Fatalf("unexpected order: %+v", findings)
	}
}

// TestRunDoctorFindsUnregisteredWriteAfterManifestEntryRemoved reproduces the
// state ML-1A's inversion was written to prevent going forward, but that can
// already exist on disk from before it landed: bytes on disk are byte-
// identical to the current catalog render, but the manifest has no entry for
// the destination. This is an end-to-end test through Manager (not
// hand-built Inspection values), proving inspectResolved/List really is
// reused, not reimplemented.
func TestRunDoctorFindsUnregisteredWriteAfterManifestEntryRemoved(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".claude/agents/trackfw-backend.md", "v1", "rendered content")

	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}

	// Simulate the pre-ADR-2026-08-18 crash window: bytes landed, manifest
	// entry for this destination did not.
	manifestFile := manifestPath(project)
	manifest, err := loadManifest(manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	absoluteDestination := filepath.Join(project, plan.Destination)
	delete(manifest.Artifacts, absoluteDestination)
	if err := writeManifest(manifestFile, manifest); err != nil {
		t.Fatal(err)
	}

	findings := ClassifyDoctor([]PlannedArtifact{plan}, mustList(t, manager, plan))
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].FindingKind != DoctorUnregisteredWrite {
		t.Fatalf("finding kind = %q, want %q", findings[0].FindingKind, DoctorUnregisteredWrite)
	}

	// Same destination but a genuine hand edit after a normal install must
	// classify differently, not collapse into the same bucket.
	if err := os.WriteFile(absoluteDestination, []byte("edited by hand"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Re-register normally, so this second case starts from Registered+owned.
	if err := manager.Install([]PlannedArtifact{plan}, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absoluteDestination, []byte("edited by hand again"), 0o600); err != nil {
		t.Fatal(err)
	}
	findings = ClassifyDoctor([]PlannedArtifact{plan}, mustList(t, manager, plan))
	if len(findings) != 1 || findings[0].FindingKind != DoctorHandModified {
		t.Fatalf("findings = %+v, want single hand-modified finding", findings)
	}
}

// TestRunDoctorFindsUnknownContentWhenNeitherRegisteredNorMatching reproduces
// the state ML-3A's audit found falling silently outside ClassifyDoctor's
// switch: a destination that was never installed (no manifest entry at all)
// but that carries content matching neither the catalog template nor any
// LegacyHashes entry. This is an end-to-end test through Manager (not
// hand-built Inspection values), proving inspectResolved really produces
// StateModified for this case (manager.go:638-645) and that ClassifyDoctor
// now has a case for it.
func TestRunDoctorFindsUnknownContentWhenNeitherRegisteredNorMatching(t *testing.T) {
	manager, project, _ := testManager(t)
	plan := testPlan("project", ".claude/agents/trackfw-backend.md", "v1", "rendered content")

	absoluteDestination := filepath.Join(project, plan.Destination)
	if err := os.MkdirAll(filepath.Dir(absoluteDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	// Alien/orphaned bytes at the exact catalog destination, never installed
	// (zero manifest entry) — content matches neither `desired` nor any
	// LegacyHashes entry.
	if err := os.WriteFile(absoluteDestination, []byte("content nobody wrote through trackfw"), 0o600); err != nil {
		t.Fatal(err)
	}

	findings := ClassifyDoctor([]PlannedArtifact{plan}, mustList(t, manager, plan))
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(findings), findings)
	}
	if findings[0].FindingKind != DoctorUnknownContent {
		t.Fatalf("finding kind = %q, want %q", findings[0].FindingKind, DoctorUnknownContent)
	}
	if !strings.Contains(findings[0].Remedy, "unmanaged artifact") {
		t.Fatalf("remedy must name the preflight refusal literally, got: %q", findings[0].Remedy)
	}
}

// TestRunDoctorSweepsFullCatalogWithoutError proves the per-(target,surface)
// sweep in doctorPlansForScope tolerates surfaces that do not support a
// given scope (pathForScope returning ok=false) instead of failing the way a
// single BuildPlans(AllSurfaces: true, no Targets filter) call would — see
// RunDoctor's doc comment. An empty project must report zero findings.
func TestRunDoctorSweepsFullCatalogWithoutError(t *testing.T) {
	manager, _, _ := testManager(t)
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	findings, err := RunDoctor(catalog, manager, identity.Config{}, nil)
	if err != nil {
		t.Fatalf("RunDoctor() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings on empty project = %d, want 0: %+v", len(findings), findings)
	}
}

func mustList(t *testing.T, manager Manager, plans ...PlannedArtifact) []Inspection {
	t.Helper()
	inspections, err := manager.List(plans)
	if err != nil {
		t.Fatal(err)
	}
	return inspections
}
