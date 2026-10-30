package generators

// consumer_install_test.go — ML-1A+ML-3A of ROADMAP-2026-10-30-go-install-entrega-versao-errada
// Tests for AC1, AC2, AC3, AC5, AC6 of REQ-2026-10-30.
//
// Reconciliation (Regra Dura):
//   TestConsumerInstall_NoGoInstall_NoBuildersConsumers (F3: regexp, not Contains)
//     — affirms AC2: no consumer builder emits go install / go run / go get for
//       github.com/kgsaran/trackfw (with optional flags between verb and path) on a
//       non-comment line.
//   TestConsumerInstall_NoReleasesLatest
//     — affirms AC3: no consumer builder references releases/latest on a non-comment line.
//   TestConsumerInstall_VersionPinEqualsVersionDotVersion (F2: nonCommentLines for positive checks)
//     — affirms AC1+AC3: TRACKFW_VERSION in every consumer output (non-comment lines only)
//       equals version.Version and the URL uses the shell variable ${TRACKFW_VERSION}
//       (not a literal) — a commented-out env block no longer satisfies this check.
//   TestConsumerInstall_VersionStubNotHardcoded (F1: new)
//     — affirms that version.Version is evaluated at call time, not hardcoded: with the
//       stub "9.9.9-stub", every consumer builder (3 builders × 2 triggers) emits
//       TRACKFW_VERSION: "9.9.9-stub" on a non-comment line.
//   TestConsumerInstall_ManualTriggerHasWorkflowDispatch
//     — affirms AC2 extension: the manual-trigger variants contain workflow_dispatch (GH)
//       or the web pipeline rule (GitLab), not pull_request / merge_requests.
//   TestConsumerInstall_ProducerHashesUnchanged
//     — affirms AC6: sha256 of every producer template matches the pre-fix golden (Apêndice A).
//   TestConsumerInstall_StaleFixture_DoctorDetects_UpdateFixes (F4: arm 2 for manual trigger)
//     — affirms AC5 for both PR and manual triggers: the pre-fix consumer
//       trackfw-validate.yml in either variant triggers a scaffold-divergent finding;
//       trackfw update corrects it to the correct variant (PR or manual); the post-fix
//       template produces no scaffold-divergent finding (contra-braço).

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/version"
)

// consumerBuilderOutput returns (pullRequestOutput, manualOutput) for a consumer template.
type consumerBuilderOutput struct {
	name string
	pr   string
	man  string
}

func allConsumerBuilderOutputs() []consumerBuilderOutput {
	return []consumerBuilderOutput{
		{
			name: "BuildDiscoverGitHubActionsWorkflowContent",
			pr:   BuildDiscoverGitHubActionsWorkflowContent(false),
			man:  BuildDiscoverGitHubActionsWorkflowContentWithTrigger(false, "manual"),
		},
		{
			name: "buildGitHubActionsWorkflowContent",
			pr:   buildGitHubActionsWorkflowContent(false),
			man:  buildGitHubActionsWorkflowContentWithTrigger(false, "manual"),
		},
		{
			name: "buildGitLabCIWorkflowContent",
			pr:   buildGitLabCIWorkflowContent(Config{}),
			man:  buildGitLabCIWorkflowContent(Config{CITrigger: "manual"}),
		},
	}
}

// nonCommentLines returns every line in s that is not a YAML comment (# prefix after
// optional whitespace). This mirrors the strip used in check-ci-workflow-pin-parity.sh
// to avoid false positives from explanatory text.
func nonCommentLines(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "#") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// goModuleRe matches any non-flagged or flagged invocation of go install/run/get for
// the trackfw module on a non-comment line (F3 fix: flags like -v between verb and path
// would previously bypass strings.Contains).
// Pattern: word boundary, go, whitespace, verb, optional flags (-flag arg …), module path.
var goModuleRe = regexp.MustCompile(`\bgo\s+(install|run|get)\s+(?:-\S+\s+)*github\.com/kgsaran/trackfw`)

// TestConsumerInstall_NoGoInstall_NoBuildersConsumers asserts that no consumer builder
// emits `go install`, `go run`, or `go get` of github.com/kgsaran/trackfw on a
// non-comment line — including forms with flags between the verb and the module path,
// e.g. `go install -v github.com/kgsaran/trackfw/…` (AC2, ADR-2026-10-30).
// Producer builders are not covered — they intentionally compile from source and are
// tested separately.
func TestConsumerInstall_NoGoInstall_NoBuildersConsumers(t *testing.T) {
	for _, b := range allConsumerBuilderOutputs() {
		for _, trigger := range []struct{ name, content string }{
			{"pull_request", b.pr},
			{"manual", b.man},
		} {
			nc := nonCommentLines(trigger.content)
			if goModuleRe.MatchString(nc) {
				t.Errorf("%s [%s]: non-comment line contains 'go install/run/get github.com/kgsaran/trackfw' (ADR-2026-10-30: not a channel)\ncontent:\n%s", b.name, trigger.name, trigger.content)
			}
		}
	}
}

// TestConsumerInstall_NoReleasesLatest asserts that no consumer builder references
// releases/latest on a non-comment line (AC3: install.sh must come from the pinned tag).
func TestConsumerInstall_NoReleasesLatest(t *testing.T) {
	for _, b := range allConsumerBuilderOutputs() {
		for _, trigger := range []struct{ name, content string }{
			{"pull_request", b.pr},
			{"manual", b.man},
		} {
			nc := nonCommentLines(trigger.content)
			if strings.Contains(nc, "releases/latest") {
				t.Errorf("%s [%s]: non-comment line contains 'releases/latest' (must use pinned tag URL)\ncontent:\n%s", b.name, trigger.name, trigger.content)
			}
		}
	}
}

// TestConsumerInstall_VersionPinEqualsVersionDotVersion asserts that every consumer
// builder sets TRACKFW_VERSION to version.Version (not a hardcoded literal) and that
// the install URL uses the shell variable ${TRACKFW_VERSION} rather than a literal
// version string — prevents Threat B from Wave 0 (version and URL decoupled).
// Both checks operate on non-comment lines only (F2 fix: a commented-out env block
// must not satisfy the positive check).
func TestConsumerInstall_VersionPinEqualsVersionDotVersion(t *testing.T) {
	wantEnv := fmt.Sprintf(`TRACKFW_VERSION: "%s"`, version.Version)
	for _, b := range allConsumerBuilderOutputs() {
		for _, trigger := range []struct{ name, content string }{
			{"pull_request", b.pr},
			{"manual", b.man},
		} {
			nc := nonCommentLines(trigger.content)
			if !strings.Contains(nc, wantEnv) {
				t.Errorf("%s [%s]: non-comment lines do not contain TRACKFW_VERSION set to version.Version (%q)\ncontent:\n%s",
					b.name, trigger.name, version.Version, trigger.content)
			}
			// The URL must be double-quoted and use the shell variable, not the literal version
			// (P2 of Wave 0: double quotes prevent word-splitting; shell variable prevents
			// TRACKFW_VERSION and the URL from decoupling). Checked on non-comment lines only
			// so that a commented-out install step does not satisfy this assertion.
			const wantURL = `"https://github.com/kgsaran/trackfw/releases/download/v${TRACKFW_VERSION}/install.sh"`
			if !strings.Contains(nc, wantURL) {
				t.Errorf("%s [%s]: non-comment lines do not contain the exact double-quoted URL %q — removing quotes or using a literal version would violate P2 of Wave 0\ncontent:\n%s",
					b.name, trigger.name, wantURL, trigger.content)
			}
		}
	}
}

// TestConsumerInstall_VersionStubNotHardcoded asserts that version.Version is read at
// call time — not hardcoded as a literal in any consumer builder. With the stub value
// "9.9.9-stub" set before calling the builders, every consumer output (3 builders ×
// 2 triggers) must contain TRACKFW_VERSION: "9.9.9-stub" on a non-comment line.
// Catches a regression where the template uses a literal (e.g. "11.2.0") instead of
// version.Version — such a regression survives silently until the next version bump
// and is undetectable by TestConsumerInstall_VersionPinEqualsVersionDotVersion (F1 fix,
// ML-3A of ROADMAP-2026-10-30).
func TestConsumerInstall_VersionStubNotHardcoded(t *testing.T) {
	orig := version.Version
	version.Version = "9.9.9-stub"
	t.Cleanup(func() { version.Version = orig })

	// Call builders AFTER the stub is set — outputs are computed eagerly.
	outputs := allConsumerBuilderOutputs()

	const wantVersion = "9.9.9-stub"
	wantEnv := fmt.Sprintf(`TRACKFW_VERSION: "%s"`, wantVersion)
	for _, b := range outputs {
		for _, trigger := range []struct{ name, content string }{
			{"pull_request", b.pr},
			{"manual", b.man},
		} {
			nc := nonCommentLines(trigger.content)
			if !strings.Contains(nc, wantEnv) {
				t.Errorf("%s [%s]: non-comment lines do not contain %q — version.Version appears hardcoded (not read at call time)\ncontent:\n%s",
					b.name, trigger.name, wantEnv, trigger.content)
			}
		}
	}
}

// TestConsumerInstall_ManualTriggerHasWorkflowDispatch asserts that the manual-trigger
// variants of GitHub Actions builders contain workflow_dispatch (not pull_request) and
// that the GitLab manual variant uses the web pipeline source rule (not merge_requests).
func TestConsumerInstall_ManualTriggerHasWorkflowDispatch(t *testing.T) {
	for _, b := range allConsumerBuilderOutputs() {
		man := b.man
		if b.name == "buildGitLabCIWorkflowContent" {
			if !strings.Contains(man, `CI_PIPELINE_SOURCE == "web"`) {
				t.Errorf("%s [manual]: missing web pipeline source rule\ncontent:\n%s", b.name, man)
			}
			if strings.Contains(man, "merge_requests") {
				t.Errorf("%s [manual]: must not contain merge_requests\ncontent:\n%s", b.name, man)
			}
		} else {
			if !strings.Contains(man, "workflow_dispatch:") {
				t.Errorf("%s [manual]: missing workflow_dispatch:\ncontent:\n%s", b.name, man)
			}
			if strings.Contains(man, "pull_request:") {
				t.Errorf("%s [manual]: must not contain pull_request:\ncontent:\n%s", b.name, man)
			}
		}
	}
}

// producerHash returns sha256 hex of a producer template (for AC6).
func producerHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// TestConsumerInstall_ProducerHashesUnchanged asserts that all four producer template
// hashes match the golden pre-fix values from Apêndice A of the Wave 0 security audit
// (docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md).
// If any producer hash changes, it means the producer template was accidentally modified
// by this ML — AC6 violation.
func TestConsumerInstall_ProducerHashesUnchanged(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "BuildDiscoverGitHubActionsWorkflowContent(true)/pull_request",
			content: BuildDiscoverGitHubActionsWorkflowContent(true),
			want:    "824938490c0748adc02374cf60ee29e369267cb64a9e63035abb475f97f76b61",
		},
		{
			name:    "BuildDiscoverGitHubActionsWorkflowContentWithTrigger(true,manual)",
			content: BuildDiscoverGitHubActionsWorkflowContentWithTrigger(true, "manual"),
			want:    "c6d21639633cdeb1d0ba11febe1341f5aaafedd7cba629b45e00dc7d0baa0f65",
		},
		{
			name:    "buildGitHubActionsWorkflowContent(true)/pull_request",
			content: buildGitHubActionsWorkflowContent(true),
			want:    "017ab0256086924825555ea3af01da498310f8b6382cdb38f529315b227593db",
		},
		{
			name:    "buildGitHubActionsWorkflowContentWithTrigger(true,manual)",
			content: buildGitHubActionsWorkflowContentWithTrigger(true, "manual"),
			want:    "bb8439421c28e83420d24786d7d0499b82f6c6557fe4c125161007edf4049166",
		},
	}
	for _, c := range cases {
		got := producerHash(c.content)
		if got != c.want {
			t.Errorf("producer template %s: hash changed (AC6 violation)\n  want: %s\n   got: %s\ncontent:\n%s",
				c.name, c.want, got, c.content)
		}
	}
}

// staleValidateYML is the byte-exact content of the consumer trackfw-validate.yml
// generated by the binary BEFORE this fix (pré-fix, version 11.2.0 with go install).
// SHA-256: aadf2cb1b4eac30dd2fe3a94f55538b4864ff826a5eaba59230975ee96cca996
// Source: git show HEAD:internal/generators/scaffold_doctor.go at commit 9167f8ad,
// BuildDiscoverGitHubActionsWorkflowContent(false) with version.Version = "11.2.0".
// After ML-1A, this content is "stale" — the binary post-fix detects it as scaffold-divergent.
const staleValidateYML = `name: trackfw validate
on:
  # push so na main. Com o gatilho de lista (push e pull_request juntos), uma
  # branch do proprio repositorio dispara os DOIS eventos no mesmo commit e o
  # gate roda em dobro — medido num PR real: 2 execucoes do mesmo job, mesmo SHA.
  push:
    branches: [main]
  pull_request:
jobs:
  governance-go-install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: "1.25"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v11.2.0
      - run: trackfw validate
`

// staleValidateManualYML is the byte-exact content of the consumer trackfw-validate.yml
// manual-trigger variant generated by the binary BEFORE this fix (pré-fix, version 11.2.0
// with go install and workflow_dispatch).
// SHA-256: bf7942d24b84045c2aa9b81f4fc302ecfc0521728a65d0c2c1ff4ac261c1e357
// Source: git show 9167f8ad:internal/generators/scaffold_doctor.go — the pre-fix
// BuildDiscoverGitHubActionsWorkflowContentWithTrigger(false, "manual") with
// version.Version = "11.2.0" (replaces push/pull_request block with workflow_dispatch).
// After ML-1A, this content is "stale" — the post-fix binary detects it as scaffold-divergent
// and corrects it to the install.sh-based manual-trigger template.
const staleValidateManualYML = `name: trackfw validate
on:
  workflow_dispatch:
jobs:
  governance-go-install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: "1.25"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v11.2.0
      - run: trackfw validate
`

// TestConsumerInstall_StaleFixture_DoctorDetects_UpdateFixes asserts AC5 for both the
// pull_request and manual trigger variants (F4 fix, ML-3A):
//  1. A consumer with the pre-fix trackfw-validate.yml (stale) is reported as
//     scaffold-divergent by RunScaffoldDoctor (pós-fix binary).
//  2. UpdateProject corrects the file to the correct trigger variant.
//  3. After update, RunScaffoldDoctor reports zero scaffold-divergent findings.
//  4. Contra-braço: the CURRENT template → zero scaffold-divergent findings.
//
// Arm 1 (pull_request, SHA aadf2cb1…cca996) and Arm 2 (manual, SHA bf7942d2…e357)
// each run in their own fixture root with the appropriate ci_trigger setting.
// Uses Go internal functions in-process (no git commit, no trackfw init binary).
func TestConsumerInstall_StaleFixture_DoctorDetects_UpdateFixes(t *testing.T) {
	// Verify the stale PR fixture matches the expected SHA-256 (P6 of Wave 0).
	staleHash := fmt.Sprintf("%x", sha256.Sum256([]byte(staleValidateYML)))
	wantHash := "aadf2cb1b4eac30dd2fe3a94f55538b4864ff826a5eaba59230975ee96cca996"
	if staleHash != wantHash {
		t.Fatalf("staleValidateYML SHA-256 = %s, want %s — fixture does not match pre-fix golden", staleHash, wantHash)
	}

	// Verify the stale manual fixture SHA-256 (F4, Apêndice A of Wave 0).
	staleManualHash := fmt.Sprintf("%x", sha256.Sum256([]byte(staleValidateManualYML)))
	wantManualHash := "bf7942d24b84045c2aa9b81f4fc302ecfc0521728a65d0c2c1ff4ac261c1e357"
	if staleManualHash != wantManualHash {
		t.Fatalf("staleValidateManualYML SHA-256 = %s, want %s — fixture does not match pre-fix manual golden", staleManualHash, wantManualHash)
	}

	// --- Arm 1: stale PR-trigger fixture → scaffold-divergent ---
	config.Reset()
	t.Cleanup(config.Reset)

	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scaffold all other managed files so the only divergence is trackfw-validate.yml.
	if err := Update(root); err != nil {
		t.Fatalf("Update (initial scaffold): %v", err)
	}

	workflowPath := filepath.Join(root, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflowPath, []byte(staleValidateYML), 0o644); err != nil {
		t.Fatal(err)
	}

	beforeFindings, err := RunScaffoldDoctor(root)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor (before update): %v", err)
	}
	foundDivergent := false
	for _, f := range beforeFindings {
		if f.Destination == DiscoverGitHubActionsWorkflowPath && f.FindingKind == integrations.DoctorScaffoldDivergent {
			foundDivergent = true
		}
	}
	if !foundDivergent {
		t.Fatalf("doctor did not flag the stale %s as scaffold-divergent (AC5 arm 1 failed)\nfindings: %+v", DiscoverGitHubActionsWorkflowPath, beforeFindings)
	}

	// Run update to fix the divergence.
	report, err := UpdateProject(root, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	var ciResult *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "ci-workflow" {
			ciResult = &report.Targets[i]
		}
	}
	if ciResult == nil || ciResult.State != TargetUpdated {
		state := "<nil>"
		if ciResult != nil {
			state = string(ciResult.State)
		}
		t.Fatalf("ci-workflow target not updated (want %q, got %q)", TargetUpdated, state)
	}

	afterFindings, err := RunScaffoldDoctor(root)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor (after update): %v", err)
	}
	for _, f := range afterFindings {
		if f.Destination == DiscoverGitHubActionsWorkflowPath && f.FindingKind == integrations.DoctorScaffoldDivergent {
			t.Errorf("doctor still reports scaffold-divergent for %s after update (AC5 arm 1 failed)", DiscoverGitHubActionsWorkflowPath)
		}
	}

	// --- Contra-braço arm 1: current PR template → zero scaffold-divergent findings ---
	config.Reset()

	root2 := t.TempDir()
	home2 := t.TempDir()
	t.Setenv("HOME", home2)

	if err := os.WriteFile(filepath.Join(root2, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(root2); err != nil {
		t.Fatalf("Update (contra-braço scaffold): %v", err)
	}

	wfPath2 := filepath.Join(root2, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(wfPath2), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write the current (post-fix) template.
	currentContent := BuildDiscoverGitHubActionsWorkflowContent(false)
	if err := os.WriteFile(wfPath2, []byte(currentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	contraFindings, err := RunScaffoldDoctor(root2)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor (contra-braço): %v", err)
	}
	for _, f := range contraFindings {
		if f.Destination == DiscoverGitHubActionsWorkflowPath && f.FindingKind == integrations.DoctorScaffoldDivergent {
			t.Errorf("contra-braço: doctor reports scaffold-divergent for current template — template mismatch in doctor (AC5 contra-braço failed)")
		}
	}

	// --- Arm 2: stale manual-trigger fixture → scaffold-divergent (F4, ML-3A) ---
	// The fixture has ci_trigger: manual so the doctor generates the manual-trigger
	// template for comparison. After UpdateProject, the written file must contain
	// workflow_dispatch: and not pull_request: (verifies the correct variant was written,
	// not a vacuous PR-template pass).
	config.Reset()

	root3 := t.TempDir()
	home3 := t.TempDir()
	t.Setenv("HOME", home3)

	if err := os.WriteFile(filepath.Join(root3, "trackfw.yaml"), []byte("hooks: none\nci: none\nci_trigger: manual\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scaffold all other managed files.
	if err := Update(root3); err != nil {
		t.Fatalf("Update arm 2 (initial scaffold): %v", err)
	}

	workflowPath3 := filepath.Join(root3, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(workflowPath3), 0o755); err != nil {
		t.Fatal(err)
	}
	// Overwrite with the stale manual fixture.
	if err := os.WriteFile(workflowPath3, []byte(staleValidateManualYML), 0o644); err != nil {
		t.Fatal(err)
	}

	beforeFindings3, err := RunScaffoldDoctor(root3)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor arm 2 (before update): %v", err)
	}
	foundDivergent3 := false
	for _, f := range beforeFindings3 {
		if f.Destination == DiscoverGitHubActionsWorkflowPath && f.FindingKind == integrations.DoctorScaffoldDivergent {
			foundDivergent3 = true
		}
	}
	if !foundDivergent3 {
		t.Fatalf("arm 2: doctor did not flag the stale manual %s as scaffold-divergent\nfindings: %+v", DiscoverGitHubActionsWorkflowPath, beforeFindings3)
	}

	// Run update.
	report3, err := UpdateProject(root3, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject arm 2: %v", err)
	}
	var ciResult3 *TargetResult
	for i := range report3.Targets {
		if report3.Targets[i].ID == "ci-workflow" {
			ciResult3 = &report3.Targets[i]
		}
	}
	if ciResult3 == nil || ciResult3.State != TargetUpdated {
		state3 := "<nil>"
		if ciResult3 != nil {
			state3 = string(ciResult3.State)
		}
		t.Fatalf("arm 2: ci-workflow target not updated (want %q, got %q)", TargetUpdated, state3)
	}

	// Assert the updated file is the manual variant (workflow_dispatch present, pull_request absent).
	updated3, err := os.ReadFile(workflowPath3)
	if err != nil {
		t.Fatalf("arm 2: reading updated workflow file: %v", err)
	}
	if !strings.Contains(string(updated3), "workflow_dispatch:") {
		t.Errorf("arm 2: updated %s does not contain workflow_dispatch: — got PR variant instead of manual\ncontent:\n%s", DiscoverGitHubActionsWorkflowPath, updated3)
	}
	if strings.Contains(string(updated3), "pull_request:") {
		t.Errorf("arm 2: updated %s still contains pull_request: — PR variant written instead of manual\ncontent:\n%s", DiscoverGitHubActionsWorkflowPath, updated3)
	}

	afterFindings3, err := RunScaffoldDoctor(root3)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor arm 2 (after update): %v", err)
	}
	for _, f := range afterFindings3 {
		if f.Destination == DiscoverGitHubActionsWorkflowPath && f.FindingKind == integrations.DoctorScaffoldDivergent {
			t.Errorf("arm 2: doctor still reports scaffold-divergent for %s after update (AC5 arm 2 failed)", DiscoverGitHubActionsWorkflowPath)
		}
	}
}
