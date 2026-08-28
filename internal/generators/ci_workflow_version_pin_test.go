package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/version"
)

// TestBuildGitHubActionsWorkflowContent_PinsCurrentVersion verifies AC6/AC7: the
// generated GitHub Actions workflow embeds internal/version.Version in the
// governance job, not a literal string, alongside timeout-minutes: 10.
func TestBuildGitHubActionsWorkflowContent_PinsCurrentVersion(t *testing.T) {
	content := buildGitHubActionsWorkflowContent(Config{})

	want := `TRACKFW_VERSION: "` + version.Version + `"`
	if !strings.Contains(content, want) {
		t.Fatalf("workflow content missing %q\ngot:\n%s", want, content)
	}
	if !strings.Contains(content, "timeout-minutes: 10") {
		t.Fatalf("workflow content missing timeout-minutes: 10\ngot:\n%s", content)
	}
}

// TestBuildGitLabCIWorkflowContent_PinsCurrentVersion verifies AC6/AC7 for the
// GitLab CI builder: TRACKFW_VERSION is emitted via the variables: block, sourced
// from internal/version.Version.
func TestBuildGitLabCIWorkflowContent_PinsCurrentVersion(t *testing.T) {
	content := buildGitLabCIWorkflowContent(Config{})

	want := `TRACKFW_VERSION: "` + version.Version + `"`
	if !strings.Contains(content, want) {
		t.Fatalf("gitlab-ci content missing %q\ngot:\n%s", want, content)
	}
	if !strings.Contains(content, "variables:") {
		t.Fatalf("gitlab-ci content missing variables: block\ngot:\n%s", content)
	}
	if !strings.Contains(content, "timeout: 10 minutes") {
		t.Fatalf("gitlab-ci content missing timeout: 10 minutes (GitLab analogue of GitHub Actions' timeout-minutes: 10 — ADR-2026-08-28)\ngot:\n%s", content)
	}
}

// TestCIWorkflowVersionPin_NotHardcoded proves the pinned version tracks
// internal/version.Version rather than being a literal baked into the builder: it
// stubs the package var to two different values and asserts the emitted pin follows
// each one. This is the falsification target named by the roadmap (ML-2A): "versão
// fica hardcoded no código-fonte do gerador" would fail this test because the output
// would stay constant across both stub values.
func TestCIWorkflowVersionPin_NotHardcoded(t *testing.T) {
	orig := version.Version
	defer func() { version.Version = orig }()

	version.Version = "1.2.3"
	ghOne := buildGitHubActionsWorkflowContent(Config{})
	glOne := buildGitLabCIWorkflowContent(Config{})

	version.Version = "9.9.9"
	ghTwo := buildGitHubActionsWorkflowContent(Config{})
	glTwo := buildGitLabCIWorkflowContent(Config{})

	if !strings.Contains(ghOne, `TRACKFW_VERSION: "1.2.3"`) {
		t.Fatalf("expected github-actions pin to follow stubbed version 1.2.3, got:\n%s", ghOne)
	}
	if !strings.Contains(ghTwo, `TRACKFW_VERSION: "9.9.9"`) {
		t.Fatalf("expected github-actions pin to follow stubbed version 9.9.9, got:\n%s", ghTwo)
	}
	if strings.Contains(ghTwo, "1.2.3") {
		t.Fatalf("github-actions workflow retained stale pin 1.2.3 after version changed:\n%s", ghTwo)
	}

	if !strings.Contains(glOne, `TRACKFW_VERSION: "1.2.3"`) {
		t.Fatalf("expected gitlab-ci pin to follow stubbed version 1.2.3, got:\n%s", glOne)
	}
	if !strings.Contains(glTwo, `TRACKFW_VERSION: "9.9.9"`) {
		t.Fatalf("expected gitlab-ci pin to follow stubbed version 9.9.9, got:\n%s", glTwo)
	}
	if strings.Contains(glTwo, "1.2.3") {
		t.Fatalf("gitlab-ci workflow retained stale pin 1.2.3 after version changed:\n%s", glTwo)
	}
}

// TestCIWorkflowVersionPin_NoLiteralInSource is a source-level falsification guard:
// it greps scaffold.go for the current binary's version as a literal string within
// the CI workflow builders' source range. The literal is derived from
// version.Version at test time (not hardcoded as e.g. "7.3.0") so this test keeps
// proving what it claims to prove even after a version bump: if a future edit
// hardcodes a version string into the template instead of referencing
// version.Version, this test fails regardless of which version is currently
// checked out — a hardcoded literal that happened to equal version.Version would
// only pass by coincidence today and silently start failing to catch the
// regression the moment the version changed, which is exactly the false-comfort
// this test must not provide.
func TestCIWorkflowVersionPin_NoLiteralInSource(t *testing.T) {
	src, err := os.ReadFile("scaffold.go")
	if err != nil {
		t.Fatalf("reading scaffold.go: %v", err)
	}
	text := string(src)

	start := strings.Index(text, "func buildGitHubActionsWorkflowContent")
	end := strings.Index(text, "func generateGitHubActionsWorkflow")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("could not locate CI workflow builder block in scaffold.go (start=%d end=%d)", start, end)
	}
	block := text[start:end]

	currentVersionLiteral := `"` + version.Version + `"`
	if strings.Contains(block, currentVersionLiteral) {
		t.Fatalf("scaffold.go CI workflow builders contain hardcoded literal %s instead of version.Version:\n%s", currentVersionLiteral, block)
	}
	if !strings.Contains(block, "version.Version") {
		t.Fatalf("scaffold.go CI workflow builders do not reference version.Version at all:\n%s", block)
	}
}

// setupCIWorkflowArtifact writes the workflow file produced by the current binary for
// the given ci mode into a temp directory. Returns the file's full path and the
// relative path scaffold_doctor identifies it by. This mirrors exactly what
// scaffold_doctor.go's RunScaffoldDoctor does for these two artifacts (it calls
// checkScaffoldArtifact with the same builder output and relPath — see
// scaffold_doctor.go:201/208) without needing to materialize the rest of the scaffold
// (scripts/, .claude/), which is unrelated to the pin and orthogonal to this ML.
func setupCIWorkflowArtifact(t *testing.T, ci string) (fullPath, relPath, content string) {
	t.Helper()
	dir := t.TempDir()

	switch ci {
	case "github-actions":
		relPath = GitHubActionsWorkflowPath
		content = buildGitHubActionsWorkflowContent(Config{})
	case "gitlab-ci":
		relPath = GitLabCIWorkflowPath
		content = buildGitLabCIWorkflowContent(Config{})
	default:
		t.Fatalf("unsupported ci mode in test helper: %q", ci)
	}

	fullPath = filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", relPath, err)
	}

	return fullPath, relPath, content
}

// TestScaffoldDoctor_CIWorkflow_NoMismatchesRightAfterGeneration verifies AC11: a CI
// workflow just written by the current binary must not be accused of
// scaffold-divergent when compared against what the same binary would generate now —
// the pin must not fight itself. Exercises the exact call scaffold_doctor.go makes
// (checkScaffoldArtifact against the builder's own output).
func TestScaffoldDoctor_CIWorkflow_NoMismatchesRightAfterGeneration(t *testing.T) {
	for _, ci := range []string{"github-actions", "gitlab-ci"} {
		t.Run(ci, func(t *testing.T) {
			fullPath, relPath, content := setupCIWorkflowArtifact(t, ci)

			finding := checkScaffoldArtifact(fullPath, relPath, []byte(content), true, false)
			if finding != nil {
				t.Errorf("expected no mismatch right after generation, got: %+v", finding)
			}
		})
	}
}

// TestScaffoldDoctor_CIWorkflow_DivergentWhenPinManuallyChanged verifies AC10: a
// workflow whose pin was hand-edited to a different version than the current binary
// reports must be flagged scaffold-divergent.
func TestScaffoldDoctor_CIWorkflow_DivergentWhenPinManuallyChanged(t *testing.T) {
	for _, ci := range []string{"github-actions", "gitlab-ci"} {
		t.Run(ci, func(t *testing.T) {
			fullPath, relPath, content := setupCIWorkflowArtifact(t, ci)

			mutated := strings.Replace(
				content,
				`TRACKFW_VERSION: "`+version.Version+`"`,
				`TRACKFW_VERSION: "0.0.1"`,
				1,
			)
			if mutated == content {
				t.Fatalf("test setup bug: mutation did not change the pin, cannot exercise scaffold-divergent")
			}
			if err := os.WriteFile(fullPath, []byte(mutated), 0o644); err != nil {
				t.Fatalf("writing mutated workflow: %v", err)
			}

			// Compare disk (mutated) against what the current binary would generate now
			// (content, pre-mutation) — exactly what RunScaffoldDoctor does.
			finding := checkScaffoldArtifact(fullPath, relPath, []byte(content), true, false)
			if finding == nil {
				t.Fatalf("expected a scaffold-divergent finding after manually changing the pin, got none")
			}
			if finding.FindingKind != "scaffold-divergent" {
				t.Errorf("expected finding kind scaffold-divergent for %s, got %q", relPath, finding.FindingKind)
			}
			if finding.Destination != relPath {
				t.Errorf("expected finding destination %q, got %q", relPath, finding.Destination)
			}
		})
	}
}

// TestRunScaffoldDoctor_CIWorkflow_EndToEnd is a lighter-weight end-to-end check
// through the public RunScaffoldDoctor entry point (not just checkScaffoldArtifact
// directly): confirms that a project with only a trackfw.yaml declaring
// ci: github-actions and a freshly generated workflow reports that specific artifact
// as a match, and reports it as scaffold-divergent once the pin is hand-edited. The
// project intentionally omits scripts/ and .claude/ (RunScaffoldDoctor still reports
// those as scaffold-missing — orthogonal to this ML's scope) so this test filters
// findings down to the one Destination under test.
func TestRunScaffoldDoctor_CIWorkflow_EndToEnd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: github-actions\n"), 0o644); err != nil {
		t.Fatalf("writing trackfw.yaml: %v", err)
	}
	relPath := GitHubActionsWorkflowPath
	fullPath := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := buildGitHubActionsWorkflowContent(Config{})
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing workflow: %v", err)
	}

	findingFor := func() *string {
		findings, err := RunScaffoldDoctor(root)
		if err != nil {
			t.Fatalf("RunScaffoldDoctor: %v", err)
		}
		for _, f := range findings {
			if f.Destination == relPath {
				kind := string(f.FindingKind)
				return &kind
			}
		}
		return nil
	}

	if kind := findingFor(); kind != nil {
		t.Fatalf("expected no finding for %s right after generation, got kind %q", relPath, *kind)
	}

	mutated := strings.Replace(content, `TRACKFW_VERSION: "`+version.Version+`"`, `TRACKFW_VERSION: "0.0.1"`, 1)
	if mutated == content {
		t.Fatalf("test setup bug: mutation did not change the pin")
	}
	if err := os.WriteFile(fullPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("writing mutated workflow: %v", err)
	}

	kind := findingFor()
	if kind == nil {
		t.Fatalf("expected a scaffold-divergent finding for %s after manually changing the pin, got none", relPath)
	}
	if *kind != "scaffold-divergent" {
		t.Errorf("expected finding kind scaffold-divergent for %s, got %q", relPath, *kind)
	}
}

// TestCIWorkflowGeneration_Idempotent proves AC of idempotency: generating the same
// workflow twice with the same binary produces byte-identical content — no
// nondeterminism (e.g. timestamps, map ordering) sneaks into the pinned template.
func TestCIWorkflowGeneration_Idempotent(t *testing.T) {
	first := buildGitHubActionsWorkflowContent(Config{})
	second := buildGitHubActionsWorkflowContent(Config{})
	if first != second {
		t.Fatalf("buildGitHubActionsWorkflowContent is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	firstGL := buildGitLabCIWorkflowContent(Config{})
	secondGL := buildGitLabCIWorkflowContent(Config{})
	if firstGL != secondGL {
		t.Fatalf("buildGitLabCIWorkflowContent is not idempotent:\nfirst:\n%s\nsecond:\n%s", firstGL, secondGL)
	}
}
