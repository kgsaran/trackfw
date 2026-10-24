package config

// Tests for ci_trigger parsing (ML-1A/ML-1E, REQ-2026-10-24).
//
// AC contract assertions per test:
//   TestLoad_CITrigger_Absent           — absent key → CITrigger="", Invalid=false
//   TestLoad_CITrigger_PullRequest      — "pull_request" → CITrigger="pull_request", Invalid=false
//   TestLoad_CITrigger_Manual           — "manual" → CITrigger="manual", Invalid=false
//   TestLoad_CITrigger_InvalidString    — unknown value → CITriggerInvalid=true, CITrigger=""
//   TestLoad_CITrigger_Sequence         — YAML sequence → CITriggerInvalid=true (non-scalar)
//   TestLoad_CITrigger_Mapping          — YAML mapping → CITriggerInvalid=true (non-scalar)
//   TestLoad_CITrigger_InjectionAttempt — value with newlines → rejected as invalid
//   TestLoad_CITrigger_EmptyAndNull     — bare key ("") and ~(null) → treated as absent, no violation (ML-1E R-2)

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_CITrigger_Absent(t *testing.T) {
	// Assert: absent ci_trigger key → zero CITrigger, no invalid flag.
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte("req_dir: docs/req/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Update.CITrigger != "" {
		t.Errorf("CITrigger: want empty, got %q", cfg.Update.CITrigger)
	}
	if cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want false, got true")
	}
}

func TestLoad_CITrigger_PullRequest(t *testing.T) {
	// Assert: ci_trigger: pull_request → accepted enum value, no invalid flag.
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: pull_request\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Update.CITrigger != "pull_request" {
		t.Errorf("CITrigger: want pull_request, got %q", cfg.Update.CITrigger)
	}
	if cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want false, got true")
	}
}

func TestLoad_CITrigger_Manual(t *testing.T) {
	// Assert: ci_trigger: manual → accepted enum value, no invalid flag.
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: manual\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Update.CITrigger != "manual" {
		t.Errorf("CITrigger: want manual, got %q", cfg.Update.CITrigger)
	}
	if cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want false, got true")
	}
}

func TestLoad_CITrigger_InvalidString(t *testing.T) {
	// Assert: unrecognised value → CITriggerInvalid=true, CITrigger="" (not propagated).
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger: push\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Update.CITrigger != "" {
		t.Errorf("CITrigger: want empty for invalid value, got %q", cfg.Update.CITrigger)
	}
	if !cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want true for unrecognised value, got false")
	}
	if cfg.Update.CITriggerRawValue == "" {
		t.Error("CITriggerRawValue: want non-empty (the rejected raw value), got empty")
	}
}

func TestLoad_CITrigger_Sequence(t *testing.T) {
	// Assert: YAML sequence → non-scalar → CITriggerInvalid=true.
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger:\n    - pull_request\n    - push\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Update.CITrigger != "" {
		t.Errorf("CITrigger: want empty for sequence value, got %q", cfg.Update.CITrigger)
	}
	if !cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want true for sequence value, got false")
	}
}

func TestLoad_CITrigger_Mapping(t *testing.T) {
	// Assert: YAML mapping → non-scalar → CITriggerInvalid=true.
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	yaml := "ci_trigger:\n    type: pull_request\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Update.CITrigger != "" {
		t.Errorf("CITrigger: want empty for mapping value, got %q", cfg.Update.CITrigger)
	}
	if !cfg.Update.CITriggerInvalid {
		t.Error("CITriggerInvalid: want true for mapping value, got false")
	}
}

// TestLoad_CITrigger_EmptyAndNull asserts that both an empty string value and a YAML null
// value for ci_trigger are treated as absent — the same as if the key was not present.
// No ci_trigger_invalid violation should be raised.
//
// Contra-braço (HEAD before ML-1E R-2): both sub-tests would have failed because the
// s == "" path landed in the else-branch and set CITriggerInvalid = true.
//
// ML-1E R-2, REQ-2026-10-24.
func TestLoad_CITrigger_EmptyAndNull(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{"empty_string", "ci_trigger: \"\"\n"},
		{"null_value", "ci_trigger: ~\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			tmp := t.TempDir()
			orig, _ := os.Getwd()
			defer func() { _ = os.Chdir(orig) }()
			if err := os.Chdir(tmp); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(tc.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := Load()
			if cfg.Update.CITriggerInvalid {
				t.Errorf("CITriggerInvalid: want false (treat as absent), got true (yaml: %q)", tc.yaml)
			}
			if cfg.Update.CITrigger != "" {
				t.Errorf("CITrigger: want empty (absent), got %q (yaml: %q)", cfg.Update.CITrigger, tc.yaml)
			}
		})
	}
}

func TestLoad_CITrigger_InjectionAttempt(t *testing.T) {
	// Assert: value with embedded newlines is rejected as invalid (YAML injection attempt,
	// T2 mitigation from ML-1A). The raw value contains a newline; the parser must treat
	// the first token as the ci_trigger value and must not find a valid enum member.
	// (YAML parses this as a plain scalar with the text before the newline only — which
	// is still not a valid enum member, so CITriggerInvalid must be true.)
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	// Note: In YAML this is two lines, but ci_trigger's scalar value is "manual" followed
	// by the rest as a plain continuation — YAML normalises it. What matters is that the
	// raw value is not "pull_request" or "manual" alone.
	yaml := "ci_trigger: \"manual\\non: push\"\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	// The embedded literal newline makes this not "manual", so it must be invalid.
	if !cfg.Update.CITriggerInvalid {
		t.Errorf("CITriggerInvalid: want true for injection attempt value %q, got false", cfg.Update.CITriggerRawValue)
	}
}
