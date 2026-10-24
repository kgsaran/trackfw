package validator

// Tests for the config_scalar_type rule (ML-1B, REQ-2026-10-24).
//
// AC contract assertions per test:
//
//	TestConfigScalarType_Positive               — scalar values → no config_scalar_type violation
//	TestConfigScalarType_SequenceOnCI           — ci: [github-actions] → one violation naming "ci"
//	TestConfigScalarType_MappingOnHooks         — hooks: {a: b} → one violation naming "hooks"
//	TestConfigScalarType_ThreeKeys              — req_dir, ci, forge as sequences → three violations
//	TestConfigScalarType_Via_ValidateUnfiltered — sequence on ci → violation in ValidateUnfiltered
//	TestConfigScalarType_Via_Validate           — sequence on ci → violation in Validate (tagged path)
//	TestConfigScalarType_CITrigger_NoDouble     — ci_trigger: [manual] → ci_trigger_invalid only, not config_scalar_type

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

func TestConfigScalarType_Positive(t *testing.T) {
	// Assert: all scalar values → validateConfigScalarType returns no messages.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"),
		[]byte("ci: github-actions\nhooks: husky\nreq_dir: docs/req/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	msgs := validateConfigScalarType()
	if len(msgs) != 0 {
		t.Errorf("want 0 messages for all-scalar config, got %d: %v", len(msgs), msgs)
	}
}

func TestConfigScalarType_SequenceOnCI(t *testing.T) {
	// Assert: ci: [github-actions] → one config_scalar_type violation mentioning "ci" and "sequence".
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("ci: [github-actions]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	msgs := validateConfigScalarType()
	if len(msgs) != 1 {
		t.Fatalf("want 1 config_scalar_type message, got %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], `"ci"`) {
		t.Errorf("message should name the key %q, got: %s", "ci", msgs[0])
	}
	if !strings.Contains(msgs[0], "sequence") {
		t.Errorf("message should mention type %q, got: %s", "sequence", msgs[0])
	}
}

func TestConfigScalarType_MappingOnHooks(t *testing.T) {
	// Assert: hooks: {a: b} → one config_scalar_type violation mentioning "hooks" and "mapping".
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("hooks:\n  a: b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	msgs := validateConfigScalarType()
	if len(msgs) != 1 {
		t.Fatalf("want 1 config_scalar_type message, got %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], `"hooks"`) {
		t.Errorf("message should name the key %q, got: %s", "hooks", msgs[0])
	}
	if !strings.Contains(msgs[0], "mapping") {
		t.Errorf("message should mention type %q, got: %s", "mapping", msgs[0])
	}
}

func TestConfigScalarType_ThreeKeys(t *testing.T) {
	// Assert: req_dir, ci, and forge as sequences → exactly three config_scalar_type violations.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	yaml := "req_dir: [docs/req/]\nci: [github-actions]\nforge: [github]\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	msgs := validateConfigScalarType()
	if len(msgs) != 3 {
		t.Fatalf("want 3 config_scalar_type messages, got %d: %v", len(msgs), msgs)
	}
	for _, want := range []string{`"req_dir"`, `"ci"`, `"forge"`} {
		found := false
		for _, m := range msgs {
			if strings.Contains(m, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no message mentions key %s", want)
		}
	}
}

func TestConfigScalarType_Via_ValidateUnfiltered(t *testing.T) {
	// Assert: ci: [github-actions] → config_scalar_type violation appears in ValidateUnfiltered.
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

	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("ci: [github-actions]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	var found bool
	for _, v := range violations {
		if strings.Contains(v, "config_scalar_type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected config_scalar_type violation in ValidateUnfiltered output; violations: %v", violations)
	}
}

func TestConfigScalarType_Via_Validate(t *testing.T) {
	// Assert: ci: [github-actions] → config_scalar_type violation appears in Validate (tagged path).
	// This test catches a forgotten mirror in validateUnfilteredTagged.
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

	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("ci: [github-actions]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	var found bool
	for _, v := range violations {
		if strings.Contains(v, "config_scalar_type") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected config_scalar_type violation in Validate output; violations: %v", violations)
	}
}

func TestConfigScalarType_CITrigger_NoDouble(t *testing.T) {
	// Assert: ci_trigger: [manual] → ci_trigger_invalid fires, config_scalar_type does NOT.
	// Rationale: ci_trigger_invalid already owns the non-scalar case for ci_trigger; emitting
	// config_scalar_type would create a duplicate violation for the same root cause.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("ci_trigger: [manual]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	scalarMsgs := validateConfigScalarType()
	for _, m := range scalarMsgs {
		if strings.Contains(m, "ci_trigger") {
			t.Errorf("config_scalar_type must not fire for ci_trigger; got: %s", m)
		}
	}

	invalidMsgs, _ := validateCITrigger()
	if len(invalidMsgs) == 0 {
		t.Error("ci_trigger_invalid: expected at least one violation for ci_trigger: [manual]")
	}
}
