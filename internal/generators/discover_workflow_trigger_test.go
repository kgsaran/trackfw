package generators

// Tests for discover workflow CI trigger support (ML-1A, REQ-2026-10-24).
//
// AC contract assertion per test:
//   TestBuildDiscoverWorkflow_ManualTrigger_WorkflowDispatchPresent —
//     manual trigger → workflow_dispatch in output; push and pull_request absent.

import (
	"strings"
	"testing"
)

func TestBuildDiscoverWorkflow_ManualTrigger_WorkflowDispatchPresent(t *testing.T) {
	// Assert: BuildDiscoverGitHubActionsWorkflowContentWithTrigger with "manual" produces
	// workflow_dispatch and must NOT contain "push:" or "pull_request:" as trigger keys.
	// (The word "pull_request" may still appear in comments — check for the YAML key form.)
	for _, isProducer := range []bool{true, false} {
		content := BuildDiscoverGitHubActionsWorkflowContentWithTrigger(isProducer, "manual")

		if !strings.Contains(content, "workflow_dispatch:") {
			t.Errorf("isProducer=%v: want workflow_dispatch: in output", isProducer)
		}
		// Neither "  pull_request:" (trigger key) nor "  push:" should appear.
		// Use the indented form to avoid false positives in comments.
		if strings.Contains(content, "\n  pull_request:") {
			t.Errorf("isProducer=%v: must not contain pull_request: trigger key in output", isProducer)
		}
		if strings.Contains(content, "\n  push:") {
			t.Errorf("isProducer=%v: must not contain push: trigger key in output", isProducer)
		}
	}
}
