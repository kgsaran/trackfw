package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/thirdparty"
	"github.com/kgsaran/trackfw/internal/validator"
)

// TestThirdPartyInstall_PassesValidateEndToEnd is the real, cross-package
// regression guard for the thirdparty_artifact_has_provenance rule
// (internal/validator/validator_thirdparty_provenance.go, ADR-2026-08-15
// D2, ML-3A): it drives the ACTUAL `third-party fetch` + `third-party
// install` commands (not hand-authored fixtures) and then asserts
// validator.Validate() reports no thirdparty_artifact_has_provenance
// violation.
//
// This exists because the rule's own package-local tests
// (validator_thirdparty_provenance_test.go) hand-author the manifest and
// provenance JSON, and an incorrect assumption baked into BOTH the rule's
// implementation and its own test fixtures would pass there while still
// being wrong against the real command. That is exactly what happened
// during this ML's implementation: the rule initially looked up a
// provenance entry by the manifest's ABSOLUTE destination key, but
// VerifyApproval/UpsertProvenanceEntry in integrations_thirdparty.go are
// actually called with the PROJECT-RELATIVE destination string
// (ResolveThirdPartySkillDestination's return value, before
// Manager.resolve() joins it against root for the manifest key) — a real
// install would have failed validate() with a false "no provenance entry"
// violation. This test would have caught that; the hand-authored ones did
// not, because they encoded the same wrong assumption on both sides.
func TestThirdPartyInstall_PassesValidateEndToEnd(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)

	agentsInstall := newAgentsCmd()
	agentsInstall.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := agentsInstall.Execute(); err != nil {
		t.Fatal(err)
	}

	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))
	url := "https://example.com/skills/my-skill.md"
	checksum := runFetch(t, url)

	dest := ".claude/skills/thirdparty/my-skill.md"
	if err := thirdparty.UpsertProvenanceEntry(project, dest, thirdparty.ProvenanceEntry{
		URL: url, ChecksumSHA256: checksum, InstalledAt: "2026-08-15T00:00:00Z",
		ApprovedBy: "hades-tf", ReviewReference: "docs/seguranca/test.md", Scope: "project",
	}); err != nil {
		t.Fatal(err)
	}

	install := newSkillsCmd()
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--apply-to", "backend",
		"--yes-i-trust-this-source",
	})
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}

	violations, _, err := validator.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, v := range violations {
		if strings.Contains(v, "thirdparty_artifact_has_provenance") {
			t.Fatalf("a correctly approved+installed third-party artifact must not trip thirdparty_artifact_has_provenance, got: %s", v)
		}
	}
}

// TestThirdPartyInstall_TamperAfterInstallFailsValidateEndToEnd is the
// negative counterpart of TestThirdPartyInstall_PassesValidateEndToEnd —
// same real command path, but the installed file is edited by hand after
// install, and validator.Validate() must catch it.
func TestThirdPartyInstall_TamperAfterInstallFailsValidateEndToEnd(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)

	agentsInstall := newAgentsCmd()
	agentsInstall.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := agentsInstall.Execute(); err != nil {
		t.Fatal(err)
	}

	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))
	url := "https://example.com/skills/my-skill.md"
	checksum := runFetch(t, url)

	dest := ".claude/skills/thirdparty/my-skill.md"
	if err := thirdparty.UpsertProvenanceEntry(project, dest, thirdparty.ProvenanceEntry{
		URL: url, ChecksumSHA256: checksum, InstalledAt: "2026-08-15T00:00:00Z",
		ApprovedBy: "hades-tf", ReviewReference: "docs/seguranca/test.md", Scope: "project",
	}); err != nil {
		t.Fatal(err)
	}

	install := newSkillsCmd()
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--apply-to", "backend",
		"--yes-i-trust-this-source",
	})
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(project+"/"+dest, []byte("# Example Third-Party Skill\n\nTAMPERED.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	violations, _, err := validator.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "thirdparty_artifact_has_provenance") && strings.Contains(v, "D2 branch ii") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a thirdparty_artifact_has_provenance D2 branch ii violation after tampering, got: %v", violations)
	}
}

// pathFormMismatchPrecondition sets $PWD to the non-canonical (symlinked) form
// of project, reproducing on macOS the Windows 8.3 short-name scenario:
// os.Getwd() returns the non-canonical form (used by integrationsManager to
// build manifest destinations via filepath.Abs), while EvalSymlinks in the
// validator expands it to the canonical form — the two forms diverge and
// filepath.Rel produces a garbage key. Returns project on success, or skips
// the test if the precondition cannot be established on this host.
func pathFormMismatchPrecondition(t *testing.T, project string) {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Skipf("EvalSymlinks(%q): %v — cannot verify symlink precondition", project, err)
	}
	if canonical == project {
		t.Skipf("TempDir path %q has no symlink component (canonical == non-canonical); "+
			"path-form mismatch not reproducible on this host", project)
	}
	// Set $PWD to the non-canonical form. Go's os.Getwd verifies $PWD and "."
	// share an inode (they do, via the /var → /private/var symlink on macOS),
	// then returns $PWD — so install records the non-canonical absolute path.
	t.Setenv("PWD", project)
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		t.Fatalf("Getwd after PWD trick: %v", cwdErr)
	}
	if cwd != project {
		t.Skipf("PWD trick did not take effect (Getwd=%q, expected %q); "+
			"path-form mismatch not reproducible on this host", cwd, project)
	}
}

// TestThirdPartyInstall_PathFormMismatch_IntactPassesValidate verifies that a
// correctly installed, approved third-party artifact is NOT accused as missing
// provenance when the manifest destination was written in a non-canonical path
// form (Windows 8.3 short names / macOS symlinked $PWD), while the validator
// resolves the project root via EvalSymlinks to the canonical form.
// Conclusion: EvalSymlinks on the manifest destination directory before
// filepath.Rel makes a valid artifact pass validate even when install and
// validate see different string representations of the same directory.
func TestThirdPartyInstall_PathFormMismatch_IntactPassesValidate(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	pathFormMismatchPrecondition(t, project)
	withOrchestratorSession(t)

	agentsInstall := newAgentsCmd()
	agentsInstall.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := agentsInstall.Execute(); err != nil {
		t.Fatal(err)
	}

	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))
	url := "https://example.com/skills/my-skill.md"
	checksum := runFetch(t, url)

	dest := ".claude/skills/thirdparty/my-skill.md"
	if err := thirdparty.UpsertProvenanceEntry(project, dest, thirdparty.ProvenanceEntry{
		URL: url, ChecksumSHA256: checksum, InstalledAt: "2026-08-15T00:00:00Z",
		ApprovedBy: "hades-tf", ReviewReference: "docs/seguranca/test.md", Scope: "project",
	}); err != nil {
		t.Fatal(err)
	}

	install := newSkillsCmd()
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--apply-to", "backend",
		"--yes-i-trust-this-source",
	})
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}

	violations, _, err := validator.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, v := range violations {
		if strings.Contains(v, "thirdparty_artifact_has_provenance") {
			t.Fatalf("correctly approved artifact must not trip thirdparty_artifact_has_provenance "+
				"even when manifest destination is in non-canonical path form (8.3/symlink), got: %s", v)
		}
	}
}

// TestThirdPartyInstall_PathFormMismatch_TamperedTriggersD2BranchII is the
// negative counterpart of TestThirdPartyInstall_PathFormMismatch_IntactPassesValidate:
// same path-form mismatch setup, but the file is tampered after install.
// The validator must catch the tamper as D2 branch ii (checksum mismatch),
// NOT as D2 branch i (missing provenance entry) — proving the provenance lookup
// succeeds under path-form mismatch and only the integrity check fires.
// Conclusion: after EvalSymlinks canonicalization of the destination, a tampered
// artifact reaches branch ii instead of silently falling through to branch i.
func TestThirdPartyInstall_PathFormMismatch_TamperedTriggersD2BranchII(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	pathFormMismatchPrecondition(t, project)
	withOrchestratorSession(t)

	agentsInstall := newAgentsCmd()
	agentsInstall.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := agentsInstall.Execute(); err != nil {
		t.Fatal(err)
	}

	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))
	url := "https://example.com/skills/my-skill.md"
	checksum := runFetch(t, url)

	dest := ".claude/skills/thirdparty/my-skill.md"
	if err := thirdparty.UpsertProvenanceEntry(project, dest, thirdparty.ProvenanceEntry{
		URL: url, ChecksumSHA256: checksum, InstalledAt: "2026-08-15T00:00:00Z",
		ApprovedBy: "hades-tf", ReviewReference: "docs/seguranca/test.md", Scope: "project",
	}); err != nil {
		t.Fatal(err)
	}

	install := newSkillsCmd()
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--apply-to", "backend",
		"--yes-i-trust-this-source",
	})
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}

	// Tamper the installed file.
	if err := os.WriteFile(project+"/"+dest, []byte("# Example Third-Party Skill\n\nTAMPERED.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	violations, _, err := validator.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "thirdparty_artifact_has_provenance") && strings.Contains(v, "D2 branch ii") {
			found = true
		}
	}
	if !found {
		t.Fatalf("tampered artifact under path-form mismatch must trigger D2 branch ii, got: %v", violations)
	}
}
