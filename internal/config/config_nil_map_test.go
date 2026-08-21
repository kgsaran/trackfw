package config

import (
	"os"
	"reflect"
	"testing"
)

// TestParseRulesFromContentWithAgentModels_NoPanic proves that
// ParseRulesFromContent does not panic when the YAML content contains
// an agent_models: block.
//
// Without the initConfigMaps fix, ParseRulesFromContent constructs
//   ProjectConfig{Rules: make(map[string]string)}
// leaving AgentModels as nil (Go zero value). parse() then executes
//   cfg.AgentModels[k] = s
// which panics with "assignment to entry in nil map".
//
// Proof of panic before fix: run this test against unfixed code:
//   go test ./internal/config/ -run TestParseRulesFromContentWithAgentModels_NoPanic -v
// Result (without fix): panic: assignment to entry in nil map
// Result (with fix):    PASS
func TestParseRulesFromContentWithAgentModels_NoPanic(t *testing.T) {
	content := "agent_models:\n  sonnet: \"4.6\"\n  opus: \"5\"\nrules:\n  wip_limit: error\n"
	got := ParseRulesFromContent(content)
	if got["wip_limit"] != "error" {
		t.Fatalf("expected rules[wip_limit]=error, got %q", got["wip_limit"])
	}
}

// TestReadAgentConventionsWithAgentModels_NoPanic proves that
// ReadAgentConventions does not panic when trackfw.yaml has agent_models:.
// This construction was fixed in ML-2B (it already initializes AgentModels).
// The test is here to guard against regression and to document both
// constructions as explicitly covered.
func TestReadAgentConventionsWithAgentModels_NoPanic(t *testing.T) {
	tmp := t.TempDir()
	content := "agent_models:\n  sonnet: \"4.6\"\nagent_conventions: \"test-convention\"\n"
	if err := writeFile(tmp+"/trackfw.yaml", content); err != nil {
		t.Fatal(err)
	}
	got := ReadAgentConventions(tmp)
	if got != "test-convention" {
		t.Fatalf("expected 'test-convention', got %q", got)
	}
}

// TestAllMapFieldsInitializedAfterParse is the enforcement gate for the
// initConfigMaps invariant: proves that after any call to parse(), all
// map-kind fields of ProjectConfig are non-nil, regardless of how the
// caller constructed the struct.
//
// This test catches regressions where initConfigMaps is accidentally
// removed, or where a new map field is added to ProjectConfig but the
// reflection sweep stops covering it (it cannot: reflection covers every
// field of the struct automatically).
func TestAllMapFieldsInitializedAfterParse(t *testing.T) {
	cfg := ProjectConfig{} // bare construction — all maps are nil
	parse("", &cfg)        // empty content, but initConfigMaps still runs

	v := reflect.ValueOf(cfg)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		fv := v.Field(i)
		if fv.Kind() == reflect.Map && fv.IsNil() {
			t.Errorf("field %s is nil after parse(empty); initConfigMaps invariant broken", typ.Field(i).Name)
		}
	}
}

// TestAllMapFieldsInitializedAfterParseWithAgentModels supplements
// TestAllMapFieldsInitializedAfterParse with non-empty content that
// exercises the agent_models write path.
func TestAllMapFieldsInitializedAfterParseWithAgentModels(t *testing.T) {
	cfg := ProjectConfig{Rules: make(map[string]string)} // no AgentModels
	content := "agent_models:\n  sonnet: \"4.6\"\n"
	parse(content, &cfg)

	v := reflect.ValueOf(cfg)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		fv := v.Field(i)
		if fv.Kind() == reflect.Map && fv.IsNil() {
			t.Errorf("field %s is nil after parse(content-with-agent_models); invariant broken", typ.Field(i).Name)
		}
	}
	if cfg.AgentModels["sonnet"] != "4.6" {
		t.Fatalf("expected AgentModels[sonnet]=4.6, got %q", cfg.AgentModels["sonnet"])
	}
}

// writeFile is a test helper that writes content to path.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
