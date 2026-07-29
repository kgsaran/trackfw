package commands

// update_harness_test.go — every test redirects HOME to a t.TempDir() and
// never runs a harness-mutating command against the real user home
// directory (see docs/req/ handoff restriction for ML-6B).

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
)

func TestUpdateHarnessCmd_RunsOutsideProjectWithoutTrackfwYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness failed outside a project: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected a text report on stdout")
	}
}

func TestUpdateHarnessCmd_EmptyHarnessExitsZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0 (nil error) for an empty harness, got: %v", err)
	}
}

func TestUpdateHarnessCmd_UnknownTargetIsUsageError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{"--targets", "not-a-real-target"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for an unknown --targets id")
	}
}

// TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract asserts the
// literal key order of the serialized document — not just presence of keys.
// docs/cli-parity.md pins scope, dry_run, targets, summary at the root, and
// id, state, path (message only when present, last) inside each target. This
// mirrors the barrier command's existing key-order regression coverage
// (see docs/cli-parity.md's own warning about the ML-2E gates divergence).
func TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "claude-skill"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(out.String())
	wantRootOrder := []string{`"scope"`, `"dry_run"`, `"targets"`, `"summary"`}
	assertKeyOrder(t, line, wantRootOrder)

	wantTargetOrder := []string{`"id"`, `"state"`, `"path"`}
	assertKeyOrder(t, line, wantTargetOrder)

	// Decode and check shape/values too, not only key order.
	var doc struct {
		Scope   string `json:"scope"`
		DryRun  bool   `json:"dry_run"`
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
		Summary struct {
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Missing int `json:"missing"`
			Failed  int `json:"failed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if doc.Scope != "harness" {
		t.Fatalf("scope = %q, want harness", doc.Scope)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "claude-skill" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "missing" {
		t.Fatalf("state = %q, want missing on an empty harness", doc.Targets[0].State)
	}
	// Summary must always carry all four counters, including zeros.
	if !strings.Contains(line, `"updated":0`) || !strings.Contains(line, `"skipped":0`) ||
		!strings.Contains(line, `"missing":1`) || !strings.Contains(line, `"failed":0`) {
		t.Fatalf("summary must always emit all four counters including zeros: %s", line)
	}
	// A target with no failure must omit "message" entirely — never emit it as "".
	if strings.Contains(line, `"message"`) {
		t.Fatalf("target without a failure must omit \"message\" entirely: %s", line)
	}
}

func TestUpdateHarnessCmd_FailedTargetIncludesMessageAndExitsNonZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Force a failure: make the claude-skill destination a directory instead
	// of a writable file location, so os.ReadFile succeeds reading a dir
	// (fails) and any write attempt fails too.
	skillPath := generators.GlobalClaudeSkillPath(home)
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "claude-skill"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-nil error when a target fails")
	}

	line := strings.TrimSpace(out.String())
	if !strings.Contains(line, `"state":"failed"`) {
		t.Fatalf("expected state failed in JSON output: %s", line)
	}
	if !strings.Contains(line, `"message"`) {
		t.Fatalf("failed target must include a message: %s", line)
	}
}

func assertKeyOrder(t *testing.T, doc string, keys []string) {
	t.Helper()
	var positions []int
	for _, key := range keys {
		pos := strings.Index(doc, key)
		if pos < 0 {
			t.Fatalf("expected key %s to be present in %s", key, doc)
		}
		positions = append(positions, pos)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] >= positions[i] {
			t.Fatalf("expected key order %v, got JSON with wrong order: %s", keys, doc)
		}
	}
}
