package commands

// doctor_windows_sh_missing_test.go — integration tests for DoctorWindowsShMissing
//
// REQ-IMP-9 (REQ-2026-11-15, ML-3D): on Windows with sh absent, the doctor
// must emit one DoctorWindowsShMissing finding per .sh convenience artifact.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/validator"
)

// makeDoctorShFixture creates the minimal directory tree expected by runDoctor:
// a valid git repository root with trackfw.yaml and the given .sh artifacts.
// Returns the root path and a cleanup function.
func makeDoctorShFixture(t *testing.T, artifacts ...string) string {
	t.Helper()
	root := t.TempDir()
	// Minimal trackfw.yaml so config.Load() does not fail.
	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0o644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	for _, rel := range artifacts {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

// TestDoctorWindowsShMissing_FindingsEmitted asserts that runDoctor emits
// DoctorWindowsShMissing findings: at least one with Destination = a .sh artifact
// path and Remedy containing "sh not found in PATH", plus the barrier finding
// with Destination = "PATH".
//
// The test calls runDoctor(false) from a git-initialised project root so that
// manager.ProjectRoot resolves correctly — this is the full doctor code path,
// not a call to CheckWindowsShMissingForDoctor in isolation.
//
// Reconciliation (ML-3D2): affirms that the loop in doctor.go that iterates over
// CheckWindowsShMissingForDoctor and the barrier-finding append both participate
// in runDoctor output. Deleting the loop or the barrier append makes the
// corresponding assertion fail.
func TestDoctorWindowsShMissing_FindingsEmitted(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	origGOOS := validator.CurrentGOOS
	origFn := validator.ShLookPathFn
	t.Cleanup(func() {
		validator.CurrentGOOS = origGOOS
		validator.ShLookPathFn = origFn
	})
	validator.CurrentGOOS = "windows"
	validator.ShLookPathFn = func(_ string) (string, error) {
		return "", os.ErrNotExist
	}

	// Place .sh artifacts inside the project root (cwd = project after integrationCommandFixture).
	for _, rel := range []string{"scripts/trackfw-validate.sh", ".husky/pre-commit"} {
		full := filepath.Join(project, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	findings, err := runDoctor(false)
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	// Collect windows-sh-missing findings.
	var shFindings []integrations.DoctorFinding
	for _, f := range findings {
		if f.FindingKind == integrations.DoctorWindowsShMissing {
			shFindings = append(shFindings, f)
		}
	}
	if len(shFindings) == 0 {
		t.Fatalf("expected DoctorWindowsShMissing findings, got none (all findings: %+v)", findings)
	}

	// At least one finding must name a .sh artifact as Destination.
	wantArtifacts := map[string]bool{
		"scripts/trackfw-validate.sh": false,
		".husky/pre-commit":           false,
	}
	for _, f := range shFindings {
		if _, ok := wantArtifacts[f.Destination]; ok {
			wantArtifacts[f.Destination] = true
		}
		if !strings.Contains(f.Remedy, "sh not found in PATH") {
			t.Errorf("Remedy must contain 'sh not found in PATH'; got: %s", f.Remedy)
		}
	}
	for art, found := range wantArtifacts {
		if !found {
			t.Errorf("expected a DoctorWindowsShMissing finding with Destination=%q; findings: %+v", art, shFindings)
		}
	}

	// Barrier finding must be present (Destination = "PATH").
	barrierFound := false
	for _, f := range shFindings {
		if f.Destination == "PATH" {
			barrierFound = true
			if !strings.Contains(f.Remedy, "sh not found in PATH") {
				t.Errorf("barrier Remedy must contain 'sh not found in PATH'; got: %s", f.Remedy)
			}
		}
	}
	if !barrierFound {
		t.Errorf("expected a DoctorWindowsShMissing barrier finding with Destination=%q; shFindings: %+v", "PATH", shFindings)
	}
}

// TestDoctorWindowsShMissing_SilentOnNonWindows asserts:
// on non-Windows GOOS, CheckWindowsShMissingForDoctor returns nil even with artifacts present.
// (ML-3D conclusion: GOOS guard suppresses the check on non-Windows platforms.)
func TestDoctorWindowsShMissing_SilentOnNonWindows(t *testing.T) {
	origGOOS := validator.CurrentGOOS
	origFn := validator.ShLookPathFn
	t.Cleanup(func() {
		validator.CurrentGOOS = origGOOS
		validator.ShLookPathFn = origFn
	})

	validator.CurrentGOOS = "darwin"
	validator.ShLookPathFn = func(_ string) (string, error) {
		return "", os.ErrNotExist
	}

	root := makeDoctorShFixture(t, "scripts/trackfw-validate.sh")
	artifacts := validator.CheckWindowsShMissingForDoctor(root)
	if len(artifacts) != 0 {
		t.Errorf("expected no artifacts on darwin, got: %v", artifacts)
	}
}

// TestDoctorWindowsShMissing_PrintReport asserts:
// printDoctorReport includes "windows-sh-missing" in the summary line.
// (ML-3D conclusion: the new kind participates in the doctor summary count.)
func TestDoctorWindowsShMissing_PrintReport(t *testing.T) {
	findings := []integrations.DoctorFinding{
		{
			FindingKind: integrations.DoctorWindowsShMissing,
			Destination: "scripts/trackfw-validate.sh",
			Remedy:      "sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to execute this script",
		},
	}
	cmd := newDoctorCmd()
	var buf strings.Builder
	cmd.SetOut(&buf)
	printDoctorReport(cmd, findings)
	report := buf.String()
	if !strings.Contains(report, "1 windows-sh-missing") {
		t.Errorf("summary must contain '1 windows-sh-missing'; got:\n%s", report)
	}
}
