package config

// Tests for the config_scalar_type recording mechanism (ML-1B, REQ-2026-10-24).
//
// AC contract assertions per test:
//
//	TestScalarType_Positive             — scalar value → no ScalarTypeError recorded
//	TestScalarType_SequenceOnCI         — ci: [github-actions] → one error, key="ci", got="sequence"
//	TestScalarType_MappingOnHooks       — hooks: {a: b} → one error, key="hooks", got="mapping"
//	TestScalarType_ThreeKeysAsSequence  — req_dir, ci, forge all as sequences → three errors
//	TestScalarType_AllKnownKeys         — all keys from KnownScalarKeys() as sequences → one error per key

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScalarType_Positive(t *testing.T) {
	// Assert: scalar value on a known key → no ScalarTypeError recorded.
	cfg := ProjectConfig{}
	parse("ci: github-actions\nhooks: husky\nreq_dir: docs/req/\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 0 {
		t.Errorf("want 0 ScalarTypeErrors for scalar values, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
}

func TestScalarType_SequenceOnCI(t *testing.T) {
	// Assert: ci: [github-actions] (sequence) → one ScalarTypeError with Key="ci", Got="sequence".
	cfg := ProjectConfig{}
	parse("ci: [github-actions]\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "ci" {
		t.Errorf("Key: want %q, got %q", "ci", e.Key)
	}
	if e.Got != "sequence" {
		t.Errorf("Got: want %q, got %q", "sequence", e.Got)
	}
	// field is treated as absent (backward-compatible)
	if cfg.Update.CI != "" {
		t.Errorf("CI: want empty (absent fallback), got %q", cfg.Update.CI)
	}
}

func TestScalarType_MappingOnHooks(t *testing.T) {
	// Assert: hooks: {a: b} (mapping) → one ScalarTypeError with Key="hooks", Got="mapping".
	cfg := ProjectConfig{}
	parse("hooks:\n  a: b\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "hooks" {
		t.Errorf("Key: want %q, got %q", "hooks", e.Key)
	}
	if e.Got != "mapping" {
		t.Errorf("Got: want %q, got %q", "mapping", e.Got)
	}
	// field is treated as absent
	if cfg.Update.Hooks != "" {
		t.Errorf("Hooks: want empty, got %q", cfg.Update.Hooks)
	}
}

func TestScalarType_ThreeKeysAsSequence(t *testing.T) {
	// Assert: req_dir, ci, and forge as YAML sequences → exactly three ScalarTypeErrors.
	cfg := ProjectConfig{}
	parse("req_dir: [docs/req/]\nci: [github-actions]\nforge: [github]\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 3 {
		t.Fatalf("want 3 ScalarTypeErrors, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	keys := make(map[string]bool)
	for _, e := range cfg.ScalarTypeErrors {
		keys[e.Key] = true
		if e.Got != "sequence" {
			t.Errorf("key %q: Got want %q, got %q", e.Key, "sequence", e.Got)
		}
	}
	for _, want := range []string{"req_dir", "ci", "forge"} {
		if !keys[want] {
			t.Errorf("missing ScalarTypeError for key %q", want)
		}
	}
}

func TestScalarType_AllKnownKeys(t *testing.T) {
	// Assert: all keys from KnownScalarKeys() given a sequence value → one error per key.
	// The list comes from the parser itself (not hand-copied), so a new key added to parse()
	// must be added to KnownScalarKeys() as well or this count assertion will catch it.
	known := KnownScalarKeys()
	if len(known) == 0 {
		t.Fatal("KnownScalarKeys() returned empty — implementation error")
	}

	// Build YAML with every known scalar key set to a sequence value [dummy].
	var sb strings.Builder
	for _, k := range known {
		sb.WriteString(fmt.Sprintf("%s: [dummy]\n", k))
	}

	cfg := ProjectConfig{}
	parse(sb.String(), &cfg)

	if len(cfg.ScalarTypeErrors) != len(known) {
		got := make(map[string]bool)
		for _, e := range cfg.ScalarTypeErrors {
			got[e.Key] = true
		}
		var missing []string
		for _, k := range known {
			if !got[k] {
				missing = append(missing, k)
			}
		}
		t.Errorf("want %d ScalarTypeErrors (one per KnownScalarKeys entry), got %d; missing: %v",
			len(known), len(cfg.ScalarTypeErrors), missing)
	}
}

// TestScalarType_CITriggerSequence_NoCrossover verifies that ci_trigger: [manual] does NOT
// produce a config_scalar_type entry — it is handled exclusively by ci_trigger_invalid to
// avoid double violations. This test asserts the dedup decision: ci_trigger_invalid owns
// the non-scalar case for ci_trigger; config_scalar_type skips that key entirely.
func TestScalarType_CITriggerSequence_NoCrossover(t *testing.T) {
	// Assert: ci_trigger: [manual] → CITriggerInvalid=true, but no ScalarTypeError.
	Reset()
	t.Cleanup(Reset)
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte("ci_trigger: [manual]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if !cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want true for ci_trigger: [manual]")
	}
	for _, e := range cfg.ScalarTypeErrors {
		if e.Key == "ci_trigger" {
			t.Errorf("ci_trigger must not appear in ScalarTypeErrors (owned by ci_trigger_invalid); got: %v", e)
		}
	}
}
