package metrics

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestExportCSV_GetWdFailure_FailClosed asserts that ExportCSV returns an error and
// writes nothing when metricsGetwdFn returns an error — fail-closed behaviour (ML-5A).
//
// Reconciliation sentence (Regra Dura de Reconciliação): this test affirms the
// ML-5A conclusion that "precondition failure must be fail-closed" — ExportCSV must
// return a non-nil error (containing "cannot verify containment") and leave the CSV
// file uncreated when the CWD cannot be determined.
//
// Old-code proof: before the fix, the `if cwdErr == nil` guard block was skipped
// (guard inside if, write outside), so os.Create ran unconditionally. The file would
// have been created even when Getwd() failed. This test would have failed against the
// old code:
//   - want: err != nil (refusing write)
//   - got:  err == nil (file created despite Getwd failure)
func TestExportCSV_GetWdFailure_FailClosed(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "output.csv")

	// Temporarily override the Getwd seam to inject a failure.
	orig := metricsGetwdFn
	metricsGetwdFn = func() (string, error) {
		return "", errors.New("injected: no working directory")
	}
	t.Cleanup(func() { metricsGetwdFn = orig })

	m := Metrics{CycleTimeMean: time.Hour, Throughput: 1.0}
	exportErr := ExportCSV(m, nil, csvPath)
	if exportErr == nil {
		t.Fatal("ExportCSV must return an error when Getwd() fails (fail-closed), got nil")
	}
	if !containsMetrics(exportErr.Error(), "cannot verify containment") {
		t.Fatalf("error must mention 'cannot verify containment', got: %v", exportErr)
	}
	if _, statErr := os.Stat(csvPath); statErr == nil {
		t.Error("containment violated: CSV file was written even though Getwd() failed")
	}
}

// TestExportCSV_ExternalAbsPath_StillAllowed asserts that ExportCSV succeeds for an
// absolute path outside the project root when Getwd() works — the named exception
// for user-directed external destinations (ADR-2026-09-18) must be preserved (ML-5A).
//
// Reconciliation sentence: this test affirms the ML-5A braço (b) constraint — the
// Beneath-false path (external absolute path) must continue to work after the
// fail-closed change, because disabling legitimate user-directed exports would be a
// regression, not a security improvement.
func TestExportCSV_ExternalAbsPath_StillAllowed(t *testing.T) {
	// Use an absolute path that is guaranteed to be outside any project root:
	// a path inside t.TempDir() which is in the OS temp directory, not the project.
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "external-metrics.csv")

	m := Metrics{CycleTimeMean: time.Hour, Throughput: 1.0}
	if err := ExportCSV(m, nil, csvPath); err != nil {
		t.Fatalf("ExportCSV must succeed for absolute external path, got: %v", err)
	}
	if _, statErr := os.Stat(csvPath); statErr != nil {
		t.Errorf("CSV file should exist after legitimate write: %v", statErr)
	}
}

// containsMetrics reports whether sub is a substring of s. Named to avoid collision
// with identically-named helpers in other test packages.
func containsMetrics(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
