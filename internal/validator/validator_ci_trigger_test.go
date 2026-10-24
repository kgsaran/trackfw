package validator

// Tests for ci_trigger_invalid and ci_trigger_manual rules (ML-1A, REQ-2026-10-24).
//
// AC contract assertions per test:
//   TestValidateCITrigger_Absent         — no ci_trigger key → no violation, no warning for either rule
//   TestValidateCITrigger_PullRequest    — ci_trigger: pull_request → no violation, no warning
//   TestValidateCITrigger_Manual         — ci_trigger: manual → no violation, one ci_trigger_manual warning
//   TestValidateCITrigger_InvalidValue   — ci_trigger: push → one ci_trigger_invalid violation
//   TestValidateCITrigger_InvalidValue_RawValueInMessage — invalid value appears in the violation message
//   TestValidateCITrigger_ManualWarning_Via_ValidateUnfiltered — manual triggers ci_trigger_manual via ValidateUnfiltered

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

func TestValidateCITrigger_Absent(t *testing.T) {
	// Assert: absent ci_trigger → validateCITrigger returns empty slices for both rules.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// trackfw.yaml with no ci_trigger key.
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("req_dir: docs/req/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	invalidMsgs, manualMsgs := validateCITrigger()
	if len(invalidMsgs) != 0 {
		t.Errorf("ci_trigger_invalid: want 0 messages for absent key, got %d: %v", len(invalidMsgs), invalidMsgs)
	}
	if len(manualMsgs) != 0 {
		t.Errorf("ci_trigger_manual: want 0 messages for absent key, got %d: %v", len(manualMsgs), manualMsgs)
	}
}

func TestValidateCITrigger_PullRequest(t *testing.T) {
	// Assert: ci_trigger: pull_request → no violation, no warning for either rule.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: pull_request\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	invalidMsgs, manualMsgs := validateCITrigger()
	if len(invalidMsgs) != 0 {
		t.Errorf("ci_trigger_invalid: want 0 for pull_request, got %d: %v", len(invalidMsgs), invalidMsgs)
	}
	if len(manualMsgs) != 0 {
		t.Errorf("ci_trigger_manual: want 0 for pull_request, got %d: %v", len(manualMsgs), manualMsgs)
	}
}

func TestValidateCITrigger_Manual(t *testing.T) {
	// Assert: ci_trigger: manual → no violation, exactly one ci_trigger_manual warning.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: manual\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	invalidMsgs, manualMsgs := validateCITrigger()
	if len(invalidMsgs) != 0 {
		t.Errorf("ci_trigger_invalid: want 0 for manual, got %d: %v", len(invalidMsgs), invalidMsgs)
	}
	if len(manualMsgs) != 1 {
		t.Errorf("ci_trigger_manual: want 1 warning for manual, got %d: %v", len(manualMsgs), manualMsgs)
	}
}

func TestValidateCITrigger_InvalidValue(t *testing.T) {
	// Assert: ci_trigger: push (unrecognised) → exactly one ci_trigger_invalid message, no manual warning.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: push\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	invalidMsgs, manualMsgs := validateCITrigger()
	if len(invalidMsgs) != 1 {
		t.Errorf("ci_trigger_invalid: want 1 violation for invalid value, got %d: %v", len(invalidMsgs), invalidMsgs)
	}
	if len(manualMsgs) != 0 {
		t.Errorf("ci_trigger_manual: want 0 warnings for invalid value, got %d: %v", len(manualMsgs), manualMsgs)
	}
}

func TestValidateCITrigger_InvalidValue_RawValueInMessage(t *testing.T) {
	// Assert: the raw invalid value appears in the violation message (T3 observability).
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: on_demand\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	invalidMsgs, _ := validateCITrigger()
	if len(invalidMsgs) == 0 {
		t.Fatal("ci_trigger_invalid: want at least 1 message, got 0")
	}
	if !strings.Contains(invalidMsgs[0], "on_demand") {
		t.Errorf("ci_trigger_invalid: want the raw value %q in message, got: %s", "on_demand", invalidMsgs[0])
	}
}

func TestValidateCITrigger_ManualWarning_Via_ValidateUnfiltered(t *testing.T) {
	// Assert: ci_trigger: manual triggers ci_trigger_manual in ValidateUnfiltered output.
	// Uses mkdirs/chdir helpers from the main test file (same package).
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/req",
		"docs/adr",
	)
	writeWriteScopeSettings(t, dir)

	yaml := "ci_trigger: manual\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	var found bool
	for _, w := range warnings {
		if strings.Contains(w, "ci_trigger_manual") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ci_trigger_manual warning in ValidateUnfiltered output; warnings: %v", warnings)
	}
}
