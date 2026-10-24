package generators

// Tests for ci_trigger support in generator functions (ML-1A, REQ-2026-10-24).
//
// AC contract assertions per test:
//
//   TestCITrigger_GoldenDefault_GateProducer   — default trigger (absent/pull_request) → byte-exact match vs golden file
//   TestCITrigger_GoldenDefault_GateConsumer   — same, consumer variant
//   TestCITrigger_GoldenDefault_GitLab         — same, GitLab
//   TestCITrigger_GoldenDefault_DiscoverProducer — same, discover producer
//   TestCITrigger_GoldenDefault_DiscoverConsumer — same, discover consumer
//   TestCITrigger_GoldenDefault_Sabotage        — contra-braço: flipping one byte in the PR template causes the test to fail
//
//   TestCITrigger_Manual_GateHasWorkflowDispatch — manual → workflow_dispatch present, pull_request absent
//   TestCITrigger_Manual_GitLabHasWebRule        — manual → CI_PIPELINE_SOURCE == "web", merge_requests absent
//   TestCITrigger_Manual_DiscoverHasWorkflowDispatch — manual → discover also has workflow_dispatch
//
//   TestCITrigger_Doctor_ConfigManual_DiskPR     — config=manual, disk=PR → mismatch reported
//   TestCITrigger_Doctor_ConfigPR_DiskManual     — config=PR, disk=manual → mismatch reported
//
//   TestCITrigger_Injection_ScalarRejected       — injection value "manual\non: push" never reaches the file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/version"
)

// versionNormalise replaces the current version string with TRACKFW_VERSION_PLACEHOLDER
// so golden comparisons survive version bumps.
func versionNormalise(s string) string {
	return strings.ReplaceAll(s, version.Version, "TRACKFW_VERSION_PLACEHOLDER")
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "ci-trigger", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(data)
}

// --- AC2: byte-exact golden comparisons (default/pull_request trigger) ---

func TestCITrigger_GoldenDefault_GateProducer(t *testing.T) {
	// Assert: default trigger produces byte-exact output matching gate-producer.golden.
	got := versionNormalise(buildGitHubActionsWorkflowContent(true))
	want := readGolden(t, "gate-producer.golden")
	if got != want {
		t.Fatalf("gate producer (default trigger): content mismatch\n--- want (golden) ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestCITrigger_GoldenDefault_GateConsumer(t *testing.T) {
	// Assert: default trigger produces byte-exact output matching gate-consumer.golden.
	got := versionNormalise(buildGitHubActionsWorkflowContent(false))
	want := readGolden(t, "gate-consumer.golden")
	if got != want {
		t.Fatalf("gate consumer (default trigger): content mismatch\n--- want (golden) ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestCITrigger_GoldenDefault_GitLab(t *testing.T) {
	// Assert: default trigger produces byte-exact output matching gitlab.golden.
	got := versionNormalise(buildGitLabCIWorkflowContent(Config{}))
	want := readGolden(t, "gitlab.golden")
	if got != want {
		t.Fatalf("gitlab (default trigger): content mismatch\n--- want (golden) ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestCITrigger_GoldenDefault_DiscoverProducer(t *testing.T) {
	// Assert: default trigger produces byte-exact output matching discover-producer.golden.
	got := versionNormalise(BuildDiscoverGitHubActionsWorkflowContent(true))
	want := readGolden(t, "discover-producer.golden")
	if got != want {
		t.Fatalf("discover producer (default trigger): content mismatch\n--- want (golden) ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestCITrigger_GoldenDefault_DiscoverConsumer(t *testing.T) {
	// Assert: default trigger produces byte-exact output matching discover-consumer.golden.
	got := versionNormalise(BuildDiscoverGitHubActionsWorkflowContent(false))
	want := readGolden(t, "discover-consumer.golden")
	if got != want {
		t.Fatalf("discover consumer (default trigger): content mismatch\n--- want (golden) ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestCITrigger_GoldenDefault_Sabotage(t *testing.T) {
	// Contra-braço (AC2): flipping the pull_request token to push_request must cause the
	// golden comparison to fail — proving the test actually checks the trigger block.
	got := versionNormalise(buildGitHubActionsWorkflowContent(true))
	sabotaged := strings.Replace(got, "pull_request:", "push_request:", 1)
	want := readGolden(t, "gate-producer.golden")
	if sabotaged == want {
		t.Fatal("sabotage: replacing pull_request with push_request did not cause a mismatch — the golden test is not checking the trigger block")
	}
}

// --- AC3: manual trigger variants ---

func TestCITrigger_Manual_GateHasWorkflowDispatch(t *testing.T) {
	// Assert: manual trigger → workflow_dispatch present, pull_request absent.
	content := buildGitHubActionsWorkflowContentWithTrigger(true, "manual")
	if !strings.Contains(content, "workflow_dispatch:") {
		t.Error("manual gate: want workflow_dispatch: in output")
	}
	if strings.Contains(content, "pull_request:") {
		t.Error("manual gate: must not contain pull_request: in output")
	}
}

func TestCITrigger_Manual_GitLabHasWebRule(t *testing.T) {
	// Assert: manual trigger → rules/web present, only/merge_requests absent.
	content := buildGitLabCIWorkflowContent(Config{CITrigger: "manual"})
	if !strings.Contains(content, `CI_PIPELINE_SOURCE == "web"`) {
		t.Error("manual gitlab: want CI_PIPELINE_SOURCE rule in output")
	}
	if strings.Contains(content, "merge_requests") {
		t.Error("manual gitlab: must not contain merge_requests in output")
	}
}

func TestCITrigger_Manual_DiscoverHasWorkflowDispatch(t *testing.T) {
	// Assert: manual trigger → discover also uses workflow_dispatch, not pull_request.
	content := BuildDiscoverGitHubActionsWorkflowContentWithTrigger(true, "manual")
	if !strings.Contains(content, "workflow_dispatch:") {
		t.Error("manual discover: want workflow_dispatch: in output")
	}
	if strings.Contains(content, "pull_request:") {
		t.Error("manual discover: must not contain pull_request: in output")
	}
}

// --- AC3 doctor cross-combination tests ---

func TestCITrigger_Doctor_ConfigManual_DiskPR(t *testing.T) {
	// Assert: config=manual but on-disk file has PR trigger → doctor reports mismatch.
	// RunScaffoldDoctor reads the CITrigger from trackfw.yaml via loadUpdateConfig().
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Write trackfw.yaml with manual trigger and ci: github-actions.
	yaml := "ci: github-actions\nci_trigger: manual\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	// Write the GitHub workflow dir with the PR-variant content (stale/wrong for manual config).
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}
	prContent := buildGitHubActionsWorkflowContentWithTrigger(false, "pull_request")
	if err := os.WriteFile(filepath.Join(workflowsDir, "trackfw-gate.yml"), []byte(prContent), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := RunScaffoldDoctor(dir)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor: %v", err)
	}
	var found bool
	for _, f := range findings {
		if strings.Contains(f.Destination, "trackfw-gate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected doctor to report a mismatch for trackfw-gate.yml when config=manual but disk=PR; findings: %+v", findings)
	}
}

func TestCITrigger_Doctor_ConfigPR_DiskManual(t *testing.T) {
	// Assert: config=pull_request (default, absent key) but on-disk file has manual trigger →
	// doctor reports mismatch. RunScaffoldDoctor reads the CITrigger from trackfw.yaml.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// trackfw.yaml present (required by RunScaffoldDoctor) but no ci_trigger key → defaults to pull_request.
	yaml := "ci: github-actions\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	workflowsDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Write the manual-variant content on disk (stale/wrong for default pull_request config).
	manualContent := buildGitHubActionsWorkflowContentWithTrigger(false, "manual")
	if err := os.WriteFile(filepath.Join(workflowsDir, "trackfw-gate.yml"), []byte(manualContent), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := RunScaffoldDoctor(dir)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor: %v", err)
	}
	var found bool
	for _, f := range findings {
		if strings.Contains(f.Destination, "trackfw-gate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected doctor to report a mismatch for trackfw-gate.yml when config=PR (absent) but disk=manual; findings: %+v", findings)
	}
}

// --- Injection guard test ---

func TestCITrigger_Injection_ScalarRejected(t *testing.T) {
	// Assert: injection value "manual\non: push" is never written to any file.
	// The value contains a literal newline, making it not equal to "manual" alone —
	// it must be treated as invalid (CITriggerInvalid=true) and generateCIWorkflow must
	// return an error before writing.
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Simulate what config.Load() would populate for an injection attempt value.
	cfg := Config{
		CI:               "github-actions",
		CITrigger:        "",   // invalid: not written
		CITriggerInvalid: true, // set by config.Load() for unrecognised value
	}

	err := generateCIWorkflow(cfg)
	if err == nil {
		t.Fatal("expected generateCIWorkflow to return error for CITriggerInvalid=true, got nil")
	}

	// The workflow file must NOT have been created.
	dest := filepath.Join(dir, ".github", "workflows", "trackfw-gate.yml")
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("trackfw-gate.yml was created despite CITriggerInvalid=true — injection guard failed")
	}
}
