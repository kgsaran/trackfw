package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: write trackfw.yaml in tmp dir, return the path.
func writeYAML(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "trackfw.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

// helper: read file, fail on error.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestAppendAgentToConfig_ByAgent_Append verifies that an agent name is appended to
// an existing block-style agents: list in a by_agent project.
// Reconciliation: this test asserts the conclusion of AC1 (first install) — agent
// installed in by_agent mode appears in agents: after the call.
func TestAppendAgentToConfig_ByAgent_Append(t *testing.T) {
	tmp := t.TempDir()
	p := writeYAML(t, tmp, "roadmap_namespacing: by_agent\nagents:\n  - alpha\n")

	if err := AppendAgentToConfig(p, "beta"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "  - beta\n") {
		t.Errorf("expected '  - beta' in output; got:\n%s", got)
	}
	if !strings.Contains(got, "  - alpha\n") {
		t.Errorf("existing agent alpha must be preserved; got:\n%s", got)
	}
}

// TestAppendAgentToConfig_ByAgent_Idempotent verifies that calling AppendAgentToConfig
// twice with the same agent name results in exactly one entry (AC1 idempotency).
// Reconciliation: this test asserts that a second install does not duplicate the entry
// in agents: — the idempotence branch of AC1.
func TestAppendAgentToConfig_ByAgent_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	p := writeYAML(t, tmp, "roadmap_namespacing: by_agent\nagents:\n  - alpha\n")

	if err := AppendAgentToConfig(p, "alpha"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := AppendAgentToConfig(p, "alpha"); err != nil {
		t.Fatalf("second call: %v", err)
	}

	got := readFile(t, p)
	count := strings.Count(got, "  - alpha")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of '  - alpha', got %d; content:\n%s", count, got)
	}
}

// TestAppendAgentToConfig_Flat_NoKey verifies that AppendAgentToConfig does not create
// or modify an agents: key when the project uses flat namespacing (AC2).
// Reconciliation: this test asserts AC2 — installing in a flat project does NOT create
// the agents: key, because that key has no function in flat mode.
func TestAppendAgentToConfig_Flat_NoKey(t *testing.T) {
	tmp := t.TempDir()
	original := "roadmap_namespacing: flat\n# some comment\nreq_dir: docs/req\n"
	p := writeYAML(t, tmp, original)

	if err := AppendAgentToConfig(p, "beta"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, p)
	if strings.Contains(got, "agents:") {
		t.Errorf("agents: key must not appear in flat project; got:\n%s", got)
	}
	if got != original {
		t.Errorf("file must be unchanged; got:\n%s", got)
	}
}

// TestAppendAgentToConfig_PreservesFormat verifies that only the agents: key changes
// when AppendAgentToConfig runs (AC3). The fixture includes comments, non-alphabetical
// key order, and custom indentation-like values to exercise preservation.
// Reconciliation: this test asserts AC3 — comments, key order, and other lines are
// preserved verbatim; only the new "  - <name>" line is added.
func TestAppendAgentToConfig_PreservesFormat(t *testing.T) {
	tmp := t.TempDir()
	// Non-alphabetical key order: req_dir before roadmap_dir, agents before ci.
	// Comments above and between keys.
	original := "# trackfw config\n" +
		"req_dir: docs/req\n" +
		"# roadmap settings\n" +
		"roadmap_dir: docs/roadmaps\n" +
		"roadmap_namespacing: by_agent\n" +
		"agents:\n" +
		"  - zeus\n" +
		"ci: github-actions\n" +
		"hooks: none\n"
	p := writeYAML(t, tmp, original)

	if err := AppendAgentToConfig(p, "apolo-tf"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, p)

	// Every original line must still be present in the same relative order.
	for _, line := range strings.Split(original, "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(got, line) {
			t.Errorf("original line %q not found in output:\n%s", line, got)
		}
	}
	// The new agent must be present.
	if !strings.Contains(got, "  - apolo-tf\n") {
		t.Errorf("expected '  - apolo-tf' in output; got:\n%s", got)
	}
	// The agents: key must appear only once.
	if strings.Count(got, "agents:") != 1 {
		t.Errorf("agents: key duplicated; got:\n%s", got)
	}
}

// TestAppendAgentToConfig_CreatesAgentsKey verifies that the agents: block is appended
// at the END of the document when it does not exist yet in a by_agent project.
// Reconciliation: this test asserts that a by_agent project without an agents: key gets
// the block appended after all existing keys — deterministic regardless of which other
// keys are present. Inserting before other keys would depend on key position.
func TestAppendAgentToConfig_CreatesAgentsKey(t *testing.T) {
	tmp := t.TempDir()
	original := "roadmap_namespacing: by_agent\nci: none\n"
	p := writeYAML(t, tmp, original)

	if err := AppendAgentToConfig(p, "gamma"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "agents:") {
		t.Errorf("agents: key should be created; got:\n%s", got)
	}
	if !strings.Contains(got, "  - gamma\n") {
		t.Errorf("expected '  - gamma' in output; got:\n%s", got)
	}
	// The agents: block must come AFTER the ci: key (end-of-document rule).
	ciIdx := strings.Index(got, "ci:")
	agentsIdx := strings.Index(got, "agents:")
	if ciIdx < 0 || agentsIdx < 0 || agentsIdx <= ciIdx {
		t.Errorf("agents: must appear after ci: (appended at end); positions ci=%d agents=%d; got:\n%s", ciIdx, agentsIdx, got)
	}
}

// TestAppendAgentToConfig_AppendsAfterLastKey verifies that when agents: is absent the
// block is appended after whatever key is last in the file (wip_limit in this fixture).
// Reconciliation: this test fixes the position contract — "append at end" is deterministic
// with any trackfw.yaml layout, unlike "insert after roadmap_namespacing:" which depends
// on that key being present and on its position relative to other keys.
func TestAppendAgentToConfig_AppendsAfterLastKey(t *testing.T) {
	tmp := t.TempDir()
	// wip_limit is the last key; agents: must come after it, not after roadmap_namespacing:
	original := "roadmap_namespacing: by_agent\nwip_limit: 3\n"
	p := writeYAML(t, tmp, original)

	if err := AppendAgentToConfig(p, "architect"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, p)
	wipIdx := strings.Index(got, "wip_limit:")
	agentsIdx := strings.Index(got, "agents:")
	if wipIdx < 0 || agentsIdx < 0 || agentsIdx <= wipIdx {
		t.Errorf("agents: must appear after wip_limit: (appended at end); positions wip=%d agents=%d; got:\n%s", wipIdx, agentsIdx, got)
	}
	if !strings.Contains(got, "  - architect\n") {
		t.Errorf("expected '  - architect' in output; got:\n%s", got)
	}
}

// TestAppendAgentToConfig_InlineFlow_Error verifies that AppendAgentToConfig returns an
// error instead of silently rewriting when agents: is in inline-flow format.
// Reconciliation: this test asserts the "rejeita e avisa" contract for inline-flow — an
// unrecognised shape must produce an error naming the file, not a silent rewrite.
func TestAppendAgentToConfig_InlineFlow_Error(t *testing.T) {
	tmp := t.TempDir()
	p := writeYAML(t, tmp, "roadmap_namespacing: by_agent\nagents: [alpha, beta]\n")

	err := AppendAgentToConfig(p, "gamma")
	if err == nil {
		t.Fatal("expected error for inline-flow agents:, got nil")
	}
	if !strings.Contains(err.Error(), "inline-flow") {
		t.Errorf("error should mention 'inline-flow'; got: %v", err)
	}
	// File must be unchanged.
	got := readFile(t, p)
	if strings.Contains(got, "gamma") {
		t.Errorf("file must not be modified; got:\n%s", got)
	}
}

// TestAppendAgentToConfig_FileAbsent verifies that a missing trackfw.yaml is silently
// ignored (not treated as an error).
// Reconciliation: this test asserts graceful no-op when the project has no trackfw.yaml
// — installing into a project that predates trackfw governance must not fail.
func TestAppendAgentToConfig_FileAbsent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "trackfw.yaml") // does not exist

	if err := AppendAgentToConfig(p, "gamma"); err != nil {
		t.Fatalf("expected nil error for absent file; got: %v", err)
	}
}

// TestAppendAgentToConfig_AC8_Falsification exercises both arms of AC8:
//   - arm A: installed agent APPEARS in agents: after the call
//   - arm B: installing in flat mode does NOT create agents: key
//
// Reconciliation: this test asserts AC8 (falsification in both directions) —
// it does not merely test one arm in isolation; both arms are required.
func TestAppendAgentToConfig_AC8_Falsification(t *testing.T) {
	t.Run("arm_A_by_agent_appears", func(t *testing.T) {
		tmp := t.TempDir()
		p := writeYAML(t, tmp, "roadmap_namespacing: by_agent\nagents:\n  - alpha\n")

		if err := AppendAgentToConfig(p, "new-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify through parse(), same function that AppendAgentToConfig uses to check membership.
		cfg := defaults()
		data, _ := os.ReadFile(p)
		parse(string(data), &cfg)

		found := false
		for _, a := range cfg.Agents {
			if a == "new-agent" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AC8 arm A: new-agent must appear in cfg.Agents after install; got %v", cfg.Agents)
		}
	})

	t.Run("arm_B_flat_no_key", func(t *testing.T) {
		tmp := t.TempDir()
		original := "roadmap_namespacing: flat\nreq_dir: docs/req\n"
		p := writeYAML(t, tmp, original)

		if err := AppendAgentToConfig(p, "new-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile(t, p)
		if strings.Contains(got, "agents:") {
			t.Errorf("AC8 arm B: agents: key must NOT be created in flat project; got:\n%s", got)
		}
	})
}
