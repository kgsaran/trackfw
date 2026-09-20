package metrics

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// symlinkOrSkipMetrics creates a symlink and skips the test if privilege is unavailable.
// Copied from internal/generators/update_test.go — detects condition, not runtime.GOOS.
func symlinkOrSkipMetrics(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if os.IsPermission(err) {
		t.Skipf("symlink guard not exercisable: %v", err)
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == 1314 {
		t.Skipf("symlink guard not exercisable: %v", err)
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
}

// TestExportCSV_SymlinkDirRefused asserts that ExportCSV rejects a write when
// the destination directory is a symlink pointing outside the project root — braço (a)
// for type (i) relative-path sites: a relative export path inside a symlinked dir.
// AC11: ExportCSV returns a non-nil error and writes nothing outside the project when
// the export subdirectory is a symlink, confirming the guard fires before os.Create.
func TestExportCSV_SymlinkDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// reports/ → symlink pointing outside project
	symlinkOrSkipMetrics(t, outside, filepath.Join(dir, "reports"))

	m := Metrics{CycleTimeMean: time.Hour, Throughput: 1.0}
	err = ExportCSV(m, nil, "reports/metrics.csv")
	if err == nil {
		t.Fatal("ExportCSV should refuse when export dir is a symlink pointing outside root, got nil")
	}
	// Nothing written outside
	if _, statErr := os.Stat(filepath.Join(outside, "metrics.csv")); statErr == nil {
		t.Error("containment violated: metrics.csv was written outside the project root")
	}
}

// TestExportCSV_LegitimateWritePasses asserts that ExportCSV succeeds for a clean
// path inside the project root — braço (b): legitimate export is not blocked.
// AC11: ExportCSV returns nil and creates the CSV file when no symlinks are present,
// confirming the guard does not block normal metrics export.
func TestExportCSV_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "metrics.csv")

	m := Metrics{CycleTimeMean: time.Hour, Throughput: 1.0}
	err := ExportCSV(m, nil, csvPath)
	if err != nil {
		t.Fatalf("ExportCSV should succeed with clean absolute path inside project, got: %v", err)
	}
	if _, statErr := os.Stat(csvPath); statErr != nil {
		t.Errorf("metrics.csv should exist after legitimate write: %v", statErr)
	}
}
