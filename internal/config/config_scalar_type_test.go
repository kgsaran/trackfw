package config

// Tests for the config_value_type recording mechanism (ML-1B/ML-1C, REQ-2026-10-24).
// Renamed from config_scalar_type to reflect expanded coverage (ML-1C).
//
// AC contract assertions per test (scalar keys — existing coverage):
//
//	TestScalarType_Positive             — scalar value → no ScalarTypeError recorded
//	TestScalarType_SequenceOnCI         — ci: [github-actions] → one error, key="ci", got="sequence"
//	TestScalarType_MappingOnHooks       — hooks: {a: b} → one error, key="hooks", got="mapping"
//	TestScalarType_ThreeKeysAsSequence  — req_dir, ci, forge all as sequences → three errors
//	TestScalarType_AllKnownKeys         — all keys from KnownScalarKeys() as sequences → one error per key
//
// AC contract assertions per test (ML-1C new coverage):
//
//	TestScalarType_ListKey_ScalarInput       — agents: x → one error, key="agents", got="scalar"
//	TestScalarType_ListKey_MappingInput      — adr_dirs: {a: b} → one error, key="adr_dirs", got="mapping"
//	TestScalarType_ListKey_NullBare          — agents: (bare) → no error (absent)
//	TestScalarType_RulesValue_SequenceInput  — rules.wip_limit: [error] → one error with qualified name
//	TestScalarType_AgentModelsValue_Seq      — agent_models.sonnet: [x] → one error with qualified name
//	TestScalarType_CredGuardMode_Seq         — credential_guard.mode: [x] → one error, key="credential_guard.mode"
//	TestScalarType_ContainerKey_Scalar       — rules: x → one error for rules key, got="scalar"
//	TestScalarType_AgentModels_ContainerSeq  — agent_models: [a] → one error for agent_models key

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

// --- ML-1C new sites ---

func TestScalarType_ListKey_ScalarInput(t *testing.T) {
	// Assert: agents: apolo-tf (scalar) → one ScalarTypeError with Key="agents", Got="scalar", Expected="sequence".
	cfg := ProjectConfig{}
	parse("agents: apolo-tf\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "agents" {
		t.Errorf("Key: want %q, got %q", "agents", e.Key)
	}
	if e.Got != "scalar" {
		t.Errorf("Got: want %q, got %q", "scalar", e.Got)
	}
	if e.Expected != "sequence" {
		t.Errorf("Expected: want %q, got %q", "sequence", e.Expected)
	}
	// field is treated as absent (backward-compatible)
	if cfg.Agents != nil {
		t.Errorf("Agents: want nil (absent fallback), got %v", cfg.Agents)
	}
}

func TestScalarType_ListKey_MappingInput(t *testing.T) {
	// Assert: adr_dirs: {a: b} (mapping) → one ScalarTypeError with Key="adr_dirs", Got="mapping".
	cfg := ProjectConfig{}
	parse("adr_dirs:\n  a: b\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "adr_dirs" {
		t.Errorf("Key: want %q, got %q", "adr_dirs", e.Key)
	}
	if e.Got != "mapping" {
		t.Errorf("Got: want %q, got %q", "mapping", e.Got)
	}
	if e.Expected != "sequence" {
		t.Errorf("Expected: want %q, got %q", "sequence", e.Expected)
	}
}

func TestScalarType_ListKey_NullBare(t *testing.T) {
	// Assert: agents: (bare key, YAML null) → no ScalarTypeError recorded (absent, no error).
	cfg := ProjectConfig{}
	parse("agents:\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 0 {
		t.Errorf("want 0 ScalarTypeErrors for bare list key, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	if cfg.Agents != nil {
		t.Errorf("Agents: want nil for bare key, got %v", cfg.Agents)
	}
}

func TestScalarType_RulesValue_SequenceInput(t *testing.T) {
	// Assert: rules.wip_limit: [error] → one ScalarTypeError with qualified key "rules.wip_limit", Got="sequence".
	cfg := ProjectConfig{Rules: make(map[string]string)}
	parse("rules:\n  wip_limit: [error]\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "rules.wip_limit" {
		t.Errorf("Key: want %q, got %q", "rules.wip_limit", e.Key)
	}
	if e.Got != "sequence" {
		t.Errorf("Got: want %q, got %q", "sequence", e.Got)
	}
	if e.Expected != "scalar string" {
		t.Errorf("Expected: want %q, got %q", "scalar string", e.Expected)
	}
	// the rules map must not contain the bad entry
	if _, ok := cfg.Rules["wip_limit"]; ok {
		t.Errorf("rules.wip_limit must not be set when value is a sequence")
	}
}

func TestScalarType_AgentModelsValue_Seq(t *testing.T) {
	// Assert: agent_models.sonnet: [x] → one ScalarTypeError with key "agent_models.sonnet", Got="sequence".
	cfg := ProjectConfig{AgentModels: make(map[string]string)}
	parse("agent_models:\n  sonnet: [x]\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "agent_models.sonnet" {
		t.Errorf("Key: want %q, got %q", "agent_models.sonnet", e.Key)
	}
	if e.Got != "sequence" {
		t.Errorf("Got: want %q, got %q", "sequence", e.Got)
	}
	// entry must not be set
	if _, ok := cfg.AgentModels["sonnet"]; ok {
		t.Errorf("agent_models.sonnet must not be set when value is a sequence")
	}
}

func TestScalarType_CredGuardMode_Seq(t *testing.T) {
	// Assert: credential_guard.mode: [block] → one ScalarTypeError with key "credential_guard.mode", Got="sequence".
	cfg := ProjectConfig{}
	parse("credential_guard:\n  mode: [block]\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "credential_guard.mode" {
		t.Errorf("Key: want %q, got %q", "credential_guard.mode", e.Key)
	}
	if e.Got != "sequence" {
		t.Errorf("Got: want %q, got %q", "sequence", e.Got)
	}
	if e.Expected != "scalar string" {
		t.Errorf("Expected: want %q, got %q", "scalar string", e.Expected)
	}
	// mode must fall back to default
	if cfg.CredentialGuard.Mode != "" {
		t.Errorf("CredentialGuard.Mode: want empty (fallback), got %q", cfg.CredentialGuard.Mode)
	}
}

func TestScalarType_ContainerKey_Scalar(t *testing.T) {
	// Assert: rules: x (scalar instead of mapping) → one ScalarTypeError with key "rules", Got="scalar".
	cfg := ProjectConfig{Rules: make(map[string]string)}
	parse("rules: x\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "rules" {
		t.Errorf("Key: want %q, got %q", "rules", e.Key)
	}
	if e.Got != "scalar" {
		t.Errorf("Got: want %q, got %q", "scalar", e.Got)
	}
	if e.Expected != "mapping" {
		t.Errorf("Expected: want %q, got %q", "mapping", e.Expected)
	}
}

func TestScalarType_AgentModels_ContainerSeq(t *testing.T) {
	// Assert: agent_models: [a] (sequence instead of mapping) → one ScalarTypeError, Got="sequence".
	cfg := ProjectConfig{AgentModels: make(map[string]string)}
	parse("agent_models: [a]\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 1 {
		t.Fatalf("want 1 ScalarTypeError, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	e := cfg.ScalarTypeErrors[0]
	if e.Key != "agent_models" {
		t.Errorf("Key: want %q, got %q", "agent_models", e.Key)
	}
	if e.Got != "sequence" {
		t.Errorf("Got: want %q, got %q", "sequence", e.Got)
	}
	if e.Expected != "mapping" {
		t.Errorf("Expected: want %q, got %q", "mapping", e.Expected)
	}
}

func TestScalarType_Positive_ListKey(t *testing.T) {
	// Assert: agents: [apolo-tf] (correct sequence) → no ScalarTypeError.
	cfg := ProjectConfig{}
	parse("agents:\n  - apolo-tf\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 0 {
		t.Errorf("want 0 ScalarTypeErrors for correct sequence, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0] != "apolo-tf" {
		t.Errorf("Agents: want [apolo-tf], got %v", cfg.Agents)
	}
}

func TestScalarType_Positive_CredGuardMode(t *testing.T) {
	// Assert: credential_guard.mode: warn (correct scalar) → no ScalarTypeError.
	cfg := ProjectConfig{}
	parse("credential_guard:\n  mode: warn\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 0 {
		t.Errorf("want 0 ScalarTypeErrors for scalar mode, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	if cfg.CredentialGuard.Mode != "warn" {
		t.Errorf("CredentialGuard.Mode: want %q, got %q", "warn", cfg.CredentialGuard.Mode)
	}
}

func TestScalarType_Positive_Rules(t *testing.T) {
	// Assert: rules.wip_limit: error (correct scalar) → no ScalarTypeError.
	cfg := ProjectConfig{Rules: make(map[string]string)}
	parse("rules:\n  wip_limit: error\n", &cfg)
	if len(cfg.ScalarTypeErrors) != 0 {
		t.Errorf("want 0 ScalarTypeErrors for valid rules, got %d: %v", len(cfg.ScalarTypeErrors), cfg.ScalarTypeErrors)
	}
	if cfg.Rules["wip_limit"] != "error" {
		t.Errorf("Rules[wip_limit]: want %q, got %q", "error", cfg.Rules["wip_limit"])
	}
}
