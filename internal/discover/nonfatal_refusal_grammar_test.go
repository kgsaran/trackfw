package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nonfatal_refusal_grammar_test.go — ML-7B / AC5 of REQ-2026-08-31.
//
// Reconciliation (one sentence): TestWriteCIWorkflowRefusalUsesTheSingleGrammar
// affirms the ML's conclusion that "non-fatal" is a CONTROL-FLOW property and not
// a licence for a message of its own — this site kept the 5th measured grammar
// ("aviso: … não escreve através de symlinks") purely because it returns nil, and
// after the collapse it still returns nil while speaking the one grammar.
//
// 🔴 No t.Parallel: the test swaps os.Stderr, a package variable read at call time.

func captureStderrDiscover(t *testing.T, body func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	original := os.Stderr
	os.Stderr = writer
	done := make(chan string, 1)
	go func() {
		var builder strings.Builder
		buffer := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				builder.Write(buffer[:n])
			}
			if readErr != nil {
				break
			}
		}
		done <- builder.String()
	}()
	body()
	os.Stderr = original
	writer.Close() //nolint:errcheck
	captured := <-done
	reader.Close() //nolint:errcheck
	return captured
}

func TestWriteCIWorkflowRefusalUsesTheSingleGrammar(t *testing.T) {
	// Control arm: clean tree — the workflow is written, nothing is printed.
	// This proves the loud arm's message comes from the guard and not from a
	// fixture that never reached the write.
	clean := t.TempDir()
	var cleanErr error
	cleanOutput := captureStderrDiscover(t, func() {
		cleanErr = writeCIWorkflow(clean)
	})
	if cleanErr != nil {
		t.Fatalf("clean tree must install the workflow, got: %v", cleanErr)
	}
	if cleanOutput != "" {
		t.Fatalf("clean tree must print nothing, got: %q", cleanOutput)
	}
	if _, err := os.Stat(filepath.Join(clean, ".github", "workflows", "trackfw-validate.yml")); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the loud arm would prove nothing", err)
	}

	// Loud arm: .github is a symlink pointing outside the project.
	root := t.TempDir()
	victim := t.TempDir()
	if err := os.Symlink(victim, filepath.Join(root, ".github")); err != nil {
		t.Fatalf("planting the bait symlink: %v", err)
	}

	var err error
	output := captureStderrDiscover(t, func() {
		err = writeCIWorkflow(root)
	})

	if err != nil {
		t.Errorf("non-fatal contract broken: writeCIWorkflow must still return nil on refusal, got: %v", err)
	}
	if output == "" {
		t.Fatalf("refusal was silent — the site must still speak, non-fatal or not")
	}
	if !strings.Contains(output, "trackfw: refusing write to ") {
		t.Errorf("refusal must use the single grammar; the old 'aviso: … não escreve através de symlinks' wording was the 5th grammar. got: %q", output)
	}
	entries, readErr := os.ReadDir(victim)
	if readErr != nil {
		t.Fatalf("reading the victim dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("containment violated: %d file(s) written into the victim dir", len(entries))
	}
}
