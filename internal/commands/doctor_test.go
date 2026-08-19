package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/integrations"
)

// TestDoctorReportsNoFindingsOnEmptyProject proves the command's silent
// path: a project with nothing installed must report zero findings, never a
// false positive from the sweep itself.
func TestDoctorReportsNoFindingsOnEmptyProject(t *testing.T) {
	integrationCommandFixture(t)

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	if !strings.Contains(out.String(), "no mismatches found") {
		t.Fatalf("expected no-findings message, got: %s", out.String())
	}
}

// TestDoctorDistinguishesUnregisteredWriteFromHandModified reproduces both
// classes end-to-end through the real command surface (not the
// ClassifyDoctor unit) and asserts the report names the remedy for each.
func TestDoctorDistinguishesUnregisteredWriteFromHandModified(t *testing.T) {
	project, _ := integrationCommandFixture(t)

	installCmd := newAgentsCmd()
	installCmd.SetArgs([]string{"install", "--items", "backend", "--targets", "claude", "--scope", "project"})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("agents install failed: %v", err)
	}

	// Case 1: unregistered write — simulate the pre-ADR-2026-08-18 crash
	// window by removing this destination's manifest entry, leaving on-disk
	// bytes that still match the current catalog render untouched. The
	// destination is read back from the manifest itself (not reconstructed
	// from the catalog) to avoid a spurious mismatch from /var vs
	// /private/var symlink resolution on macOS.
	manifestFile := integrations.ManifestPath(project)
	raw, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	artifacts, _ := manifest["artifacts"].(map[string]interface{})
	if len(artifacts) != 1 {
		t.Fatalf("expected exactly one manifest artifact, manifest: %s", raw)
	}
	var destination string
	for key := range artifacts {
		destination = key
	}
	delete(artifacts, destination)
	tampered, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestFile, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "["+string(integrations.DoctorUnregisteredWrite)+"]") {
		t.Fatalf("expected unregistered-write finding, got: %s", report)
	}
	if !strings.Contains(report, "agents install --force") {
		t.Fatalf("expected remedy naming agents install --force, got: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorHandModified)+"]") {
		t.Fatalf("must not report hand-modified for this case: %s", report)
	}

	// Restore normal management, then simulate a hand edit after install —
	// distinct class, distinct remedy wording (must still be install
	// --force, but the two finding kinds must not collapse).
	installCmd = newAgentsCmd()
	installCmd.SetArgs([]string{"install", "--items", "backend", "--targets", "claude", "--scope", "project", "--force"})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("agents install --force (re-adopt) failed: %v", err)
	}
	if err := os.WriteFile(destination, []byte("edited by a human"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd = newDoctorCmd()
	out.Reset()
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	report = out.String()
	if !strings.Contains(report, "["+string(integrations.DoctorHandModified)+"]") {
		t.Fatalf("expected hand-modified finding, got: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorUnregisteredWrite)+"]") {
		t.Fatalf("must not report unregistered-write for this case: %s", report)
	}
}

// TestDoctorReportsUnknownContentForNeverInstalledDestination reproduces
// end-to-end (real command surface, not the ClassifyDoctor unit) the state
// ML-3A's audit found silently unreported: a destination the catalog would
// use for this claim, carrying content that was never installed by trackfw
// (no manifest entry) and that does not match the catalog template either.
// The remedy must name the preflight refusal literally.
func TestDoctorReportsUnknownContentForNeverInstalledDestination(t *testing.T) {
	project, _ := integrationCommandFixture(t)

	// Learn the exact destination `agents install` would use, read-only,
	// without ever installing — mirrors scripts/check-doctor-parity.sh's
	// scenario (d)/(f) fixture construction.
	listCmd := newAgentsCmd()
	listCmd.SetArgs([]string{"list", "--items", "backend", "--targets", "claude", "--scope", "project", "--json"})
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("agents list failed: %v", err)
	}
	var listPayload struct {
		Deployments []struct {
			Destination string `json:"destination"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(listOut.Bytes(), &listPayload); err != nil {
		t.Fatalf("agents list --json did not decode: %v\n%s", err, listOut.String())
	}
	if len(listPayload.Deployments) != 1 {
		t.Fatalf("expected exactly one deployment row, got %d: %+v", len(listPayload.Deployments), listPayload.Deployments)
	}
	destination := listPayload.Deployments[0].Destination
	if !strings.HasPrefix(destination, "/") {
		destination = project + "/" + destination
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("nobody installed this through trackfw"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "["+string(integrations.DoctorUnknownContent)+"]") {
		t.Fatalf("expected unknown-content finding, got: %s", report)
	}
	if !strings.Contains(report, "unmanaged artifact") {
		t.Fatalf("expected remedy to name the preflight refusal literally, got: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorUnregisteredWrite)+"]") {
		t.Fatalf("must not report unregistered-write for this case: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorHandModified)+"]") {
		t.Fatalf("must not report hand-modified for this case: %s", report)
	}
}

// TestDoctorJSONOutputIsValidAndFindingLess proves --json emits a decodable
// array with the same zero-findings shape as the text report.
func TestDoctorJSONOutputIsValidAndFindingLess(t *testing.T) {
	integrationCommandFixture(t)

	cmd := newDoctorCmd()
	cmd.SetArgs([]string{"--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor --json failed: %v", err)
	}
	var findings []integrations.DoctorFinding
	if err := json.Unmarshal(out.Bytes(), &findings); err != nil {
		t.Fatalf("doctor --json did not emit a decodable array: %v\n%s", err, out.String())
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0: %+v", len(findings), findings)
	}
}
