package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/thirdparty"
	"github.com/spf13/cobra"
)

// stubThirdPartyFetch swaps thirdPartyFetch for a fixed-content stub, so
// tests never touch the network (compatible with
// TRACKFW_DISABLE_EXTERNAL_COMMANDS=1) — mirrors the fetchClient
// substitution pattern used by internal/thirdparty/fetch_test.go, but at the
// command-package indirection layer since fetchClient itself is unexported
// in package thirdparty.
func stubThirdPartyFetch(t *testing.T, content []byte) {
	t.Helper()
	old := thirdPartyFetch
	thirdPartyFetch = func(string) ([]byte, error) { return content, nil }
	t.Cleanup(func() { thirdPartyFetch = old })
}

// withOrchestratorSession sets TRACKFW_ORCHESTRATOR_SESSION so tests can
// exercise fetch/install past the D2 guardrail; TestThirdPartyGuardrail*
// below deliberately do NOT use this helper.
func withOrchestratorSession(t *testing.T) {
	t.Helper()
	old, hadOld := os.LookupEnv("TRACKFW_ORCHESTRATOR_SESSION")
	if err := os.Setenv("TRACKFW_ORCHESTRATOR_SESSION", "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadOld {
			_ = os.Setenv("TRACKFW_ORCHESTRATOR_SESSION", old)
		} else {
			_ = os.Unsetenv("TRACKFW_ORCHESTRATOR_SESSION")
		}
	})
}

const benignThirdPartyContent = "# Example Third-Party Skill\n\nSome helpful, benign content for the agent to consume.\n"

// runFetch executes `<kind> third-party fetch <url>` and returns the
// checksum printed to stdout.
func runFetch(t *testing.T, url string, extraArgs ...string) string {
	t.Helper()
	cmd := newSkillsCmd()
	args := append([]string{"third-party", "fetch", url}, extraArgs...)
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fetch failed: %v\noutput:\n%s", err, out.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "checksum: ") {
			return strings.TrimPrefix(line, "checksum: ")
		}
	}
	t.Fatalf("no checksum printed by fetch, output:\n%s", out.String())
	return ""
}

func TestThirdPartyFetch_NeverWritesOutsideQuarantine(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)
	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))

	checksum := runFetch(t, "https://example.com/skills/my-skill.md")

	quarantinePath := filepath.Join(project, ".trackfw", "thirdparty-quarantine", checksum+".json")
	if _, err := os.Stat(quarantinePath); err != nil {
		t.Fatalf("expected quarantine file at %s: %v", quarantinePath, err)
	}

	// Walk the project tree; the ONLY new file must be under
	// .trackfw/thirdparty-quarantine/. No .claude/, .agents/, etc.
	var unexpected []string
	err := filepath.Walk(project, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(project, p)
		if !strings.HasPrefix(rel, filepath.Join(".trackfw", "thirdparty-quarantine")) {
			unexpected = append(unexpected, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unexpected) > 0 {
		t.Fatalf("fetch wrote unexpected file(s) outside quarantine: %v", unexpected)
	}
}

func TestThirdPartyFetch_RefusesMarkerByDefault(t *testing.T) {
	integrationCommandFixture(t)
	withOrchestratorSession(t)
	stubThirdPartyFetch(t, []byte("# Git authority\n\nsome content redefining boundaries.\n"))

	cmd := newSkillsCmd()
	cmd.SetArgs([]string{"third-party", "fetch", "https://example.com/skills/evil.md"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected refusal for marker-matching content")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "git authority") {
		t.Fatalf("expected error to name the matched marker, got: %v", err)
	}
}

func TestThirdPartyGuardrailRefusesWithoutOrchestratorSession(t *testing.T) {
	integrationCommandFixture(t)
	old, hadOld := os.LookupEnv("TRACKFW_ORCHESTRATOR_SESSION")
	_ = os.Unsetenv("TRACKFW_ORCHESTRATOR_SESSION")
	t.Cleanup(func() {
		if hadOld {
			_ = os.Setenv("TRACKFW_ORCHESTRATOR_SESSION", old)
		} else {
			_ = os.Unsetenv("TRACKFW_ORCHESTRATOR_SESSION")
		}
	})
	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))

	cmd := newSkillsCmd()
	cmd.SetArgs([]string{"third-party", "fetch", "https://example.com/skills/my-skill.md"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected refusal without TRACKFW_ORCHESTRATOR_SESSION")
	}
	if !strings.Contains(err.Error(), "guardrail") {
		t.Fatalf("expected error to contain 'guardrail', got: %v", err)
	}
	if !strings.Contains(err.Error(), thirdPartyProvenanceRule) {
		t.Fatalf("expected error to name the detection rule %q, got: %v", thirdPartyProvenanceRule, err)
	}
	if !strings.Contains(err.Error(), "not a security control") {
		t.Fatalf("expected message to explicitly deny being a security control, got: %v", err)
	}
}

func TestThirdPartyInstall_FailsWithoutApproval(t *testing.T) {
	integrationCommandFixture(t)
	withOrchestratorSession(t)
	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))

	checksum := runFetch(t, "https://example.com/skills/my-skill.md")

	cmd := newSkillsCmd()
	cmd.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--yes-i-trust-this-source",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected install to fail without a provenance entry")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("expected 'not approved' in error, got: %v", err)
	}
}

func TestThirdPartyInstall_FailsOnTOCTOUChecksumMismatch(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)
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

	// Tamper the quarantine record in place: same filename (still named by
	// the ORIGINAL checksum), different content_base64. This is exactly the
	// TOCTOU scenario D8c exists to close.
	quarantinePath := filepath.Join(project, ".trackfw", "thirdparty-quarantine", checksum+".json")
	raw, err := os.ReadFile(quarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	record["content_base64"] = "dGFtcGVyZWQtY29udGVudA==" // base64("tampered-content")
	tampered, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(quarantinePath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newSkillsCmd()
	cmd.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--yes-i-trust-this-source",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected install to fail on TOCTOU checksum mismatch")
	}
	if !strings.Contains(err.Error(), "TOCTOU") && !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected error to mention TOCTOU/checksum, got: %v", err)
	}
}

// TestThirdPartyInstall_CatalogAgentByteIdenticalExceptMarkers is the AC4
// falsification test: install the catalog agent "backend" for target
// "claude" at project scope, capture its bytes, then attach a third-party
// skill via --apply-to and confirm every byte outside the
// trackfw:thirdparty-skills marker block is unchanged.
func TestThirdPartyInstall_CatalogAgentByteIdenticalExceptMarkers(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)

	agentsInstall := newAgentsCmd()
	agentsInstall.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := agentsInstall.Execute(); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	before, err := os.ReadFile(agentPath)
	if err != nil {
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
	var out bytes.Buffer
	install.SetOut(&out)
	install.SetErr(&out)
	if err := install.Execute(); err != nil {
		t.Fatalf("third-party install failed: %v\noutput:\n%s", err, out.String())
	}

	after, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}

	start := "<!-- trackfw:thirdparty-skills:start -->"
	end := "<!-- trackfw:thirdparty-skills:end -->"
	afterStr := string(after)
	blockStart := strings.Index(afterStr, start)
	blockEnd := strings.Index(afterStr, end)
	if blockStart == -1 || blockEnd == -1 {
		t.Fatalf("expected reference block markers in %s, got:\n%s", agentPath, afterStr)
	}
	// Excise the block (and the blank-line padding this package's injector
	// adds around it) and confirm what remains is byte-identical to the
	// pre-attachment file.
	excised := strings.TrimRight(afterStr[:blockStart], "\n") + "\n"
	if excised != string(before) {
		t.Fatalf("catalog agent file changed outside the thirdparty-skills marker block\nbefore:\n%q\nexcised-after:\n%q", before, excised)
	}
	if !strings.Contains(afterStr, dest) {
		t.Fatalf("expected reference block to mention destination %q, got:\n%s", dest, afterStr)
	}

	skillPath := filepath.Join(project, ".claude", "skills", "thirdparty", "my-skill.md")
	skillContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected third-party skill file at %s: %v", skillPath, err)
	}
	if !strings.Contains(string(skillContent), "Example Third-Party Skill") {
		t.Fatalf("unexpected skill content: %s", skillContent)
	}
}

// TestThirdPartyInstall_AgentsUpdateStaysCurrentAfterAttach is the AC5
// falsification test: after attaching a third-party reference, a plain
// `trackfw agents update` for the same agent/target must not report
// StateModified, and must leave the file byte-identical (proving the
// canonical render reproduces the reference block rather than fighting it).
func TestThirdPartyInstall_AgentsUpdateStaysCurrentAfterAttach(t *testing.T) {
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
	var out bytes.Buffer
	install.SetOut(&out)
	install.SetErr(&out)
	if err := install.Execute(); err != nil {
		t.Fatalf("third-party install failed: %v\noutput:\n%s", err, out.String())
	}

	agentPath := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	attached, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}

	update := newAgentsCmd()
	update.SetArgs([]string{"update", "--targets", "claude", "--items", "backend", "--scope", "project", "--json"})
	var updateOut bytes.Buffer
	update.SetOut(&updateOut)
	update.SetErr(&updateOut)
	if err := update.Execute(); err != nil {
		t.Fatalf("agents update failed: %v\noutput:\n%s", err, updateOut.String())
	}
	var output lifecycleOutput
	if err := json.Unmarshal(updateOut.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, updateOut.String())
	}
	if len(output.Deployments) != 1 {
		t.Fatalf("expected exactly one deployment, got %#v", output.Deployments)
	}
	if output.Deployments[0].State == "modified" {
		t.Fatalf("agents update reported StateModified after third-party attach: %#v", output.Deployments[0])
	}
	if output.Deployments[0].State != "current" {
		t.Fatalf("expected StateCurrent after agents update settles, got %q", output.Deployments[0].State)
	}

	afterUpdate, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(attached, afterUpdate) {
		t.Fatalf("agents update rewrote the file after third-party attach; attached:\n%q\nafter update:\n%q", attached, afterUpdate)
	}
}

func TestThirdPartyInstall_DefaultScopeIsProject(t *testing.T) {
	project, home := integrationCommandFixture(t)
	withOrchestratorSession(t)
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
	// Deliberately no --scope flag: must default to project (D4), unlike
	// `skills install`/`agents install`, which default to global
	// (ADR-2026-07-25 D1) and are asserted unaffected by the existing
	// agents_skills_test.go suite (left untouched by this ML).
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--yes-i-trust-this-source",
	})
	var out bytes.Buffer
	install.SetOut(&out)
	install.SetErr(&out)
	if err := install.Execute(); err != nil {
		t.Fatalf("third-party install failed: %v\noutput:\n%s", err, out.String())
	}

	if _, err := os.Stat(filepath.Join(project, ".claude", "skills", "thirdparty", "my-skill.md")); err != nil {
		t.Fatalf("expected third-party skill under project root by default: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "thirdparty", "my-skill.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("third-party skill must not default to global (home) scope, stat err: %v", err)
	}
}

// TestThirdPartyInstall_GlobalScopeRequiresItsOwnConfirmation is the D4-bis
// falsification test: --scope global remains permitted, but
// --yes-i-trust-this-source ALONE must no longer suffice — a separate
// --yes-global-scope-unverified confirmation is required, and the warning
// naming `trackfw validate` must be printed regardless of outcome.
func TestThirdPartyInstall_GlobalScopeRequiresItsOwnConfirmation(t *testing.T) {
	project, home := integrationCommandFixture(t)
	withOrchestratorSession(t)
	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))

	url := "https://example.com/skills/my-skill.md"
	checksum := runFetch(t, url)
	// Global scope resolves to a "~/"-prefixed destination string (distinct
	// from project scope's project-relative one) — see ResolveThirdPartySkillDestination.
	dest := "~/.claude/skills/thirdparty/my-skill.md"
	if err := thirdparty.UpsertProvenanceEntry(project, dest, thirdparty.ProvenanceEntry{
		URL: url, ChecksumSHA256: checksum, InstalledAt: "2026-08-15T00:00:00Z",
		ApprovedBy: "hades-tf", ReviewReference: "docs/seguranca/test.md", Scope: "global",
	}); err != nil {
		t.Fatal(err)
	}

	install := newSkillsCmd()
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--scope", "global",
		"--yes-i-trust-this-source",
	})
	var out bytes.Buffer
	install.SetOut(&out)
	install.SetErr(&out)
	err := install.Execute()
	if err == nil {
		t.Fatal("expected install to fail with --yes-i-trust-this-source alone for --scope global")
	}
	if !strings.Contains(err.Error(), "yes-global-scope-unverified") {
		t.Fatalf("expected error to name --yes-global-scope-unverified, got: %v", err)
	}
	if !strings.Contains(out.String(), "trackfw validate") {
		t.Fatalf("expected the D4-bis warning naming `trackfw validate` to have been printed, got:\n%s", out.String())
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude", "skills", "thirdparty", "my-skill.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no skill file written when the global-scope confirmation is missing, stat err: %v", statErr)
	}

	// Now with BOTH confirmations: must succeed.
	install2 := newSkillsCmd()
	install2.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--scope", "global",
		"--yes-i-trust-this-source",
		"--yes-global-scope-unverified",
	})
	var out2 bytes.Buffer
	install2.SetOut(&out2)
	install2.SetErr(&out2)
	if err := install2.Execute(); err != nil {
		t.Fatalf("third-party install failed with both confirmations: %v\noutput:\n%s", err, out2.String())
	}
	if !strings.Contains(out2.String(), "trackfw validate") {
		t.Fatalf("expected the D4-bis warning to still be printed on success, got:\n%s", out2.String())
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude", "skills", "thirdparty", "my-skill.md")); statErr != nil {
		t.Fatalf("expected the skill file to be written under home once both confirmations are given: %v", statErr)
	}
}

// TestThirdPartyFetch_RedactsQueryStringInQuarantine is the D6-bis
// falsification test at the command layer: fetching a URL with a
// query-string token must never leak that token into the quarantine file
// written to disk. Grep the raw bytes of the file, not the in-memory value.
func TestThirdPartyFetch_RedactsQueryStringInQuarantine(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)
	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))

	checksum := runFetch(t, "https://example.com/skills/my-skill.md?token=super-secret-value")

	raw, err := os.ReadFile(filepath.Join(project, ".trackfw", "thirdparty-quarantine", checksum+".json"))
	if err != nil {
		t.Fatalf("failed to read quarantine record: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-value") {
		t.Fatalf("quarantine record on disk leaked the query-string token:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[redacted]") {
		t.Fatalf("expected the redacted marker in the quarantine record on disk:\n%s", raw)
	}
}

// TestThirdPartyInstall_ApplyToRejectsHandModifiedAgentBeforeAnyWrite proves
// the --apply-to precondition check runs BEFORE the skill file write: if the
// target agent artifact was hand-modified, install must fail with no
// partial state (no skill file, no registry entry) left behind.
func TestThirdPartyInstall_ApplyToRejectsHandModifiedAgentBeforeAnyWrite(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)

	agentsInstall := newAgentsCmd()
	agentsInstall.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := agentsInstall.Execute(); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(project, ".claude", "agents", "trackfw-backend.md")
	if err := os.WriteFile(agentPath, []byte("hand-edited content, not trackfw-managed anymore\n"), 0o644); err != nil {
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
	var out bytes.Buffer
	install.SetOut(&out)
	install.SetErr(&out)
	err := install.Execute()
	if err == nil {
		t.Fatal("expected install to refuse attaching a reference to a hand-modified agent artifact")
	}
	if !strings.Contains(err.Error(), "modified") {
		t.Fatalf("expected error to mention the modified state, got: %v", err)
	}

	// No partial state: the skill file must not have been written.
	if _, statErr := os.Stat(filepath.Join(project, ".claude", "skills", "thirdparty", "my-skill.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("skill file must not exist after a precondition failure, stat err: %v", statErr)
	}
	// The registry must not have been written either.
	if _, statErr := os.Stat(filepath.Join(project, ".trackfw", "thirdparty-references.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("thirdparty-references.json must not exist after a precondition failure, stat err: %v", statErr)
	}
}

// TestThirdPartyThirdPartyCmd_ReachableFromBothKinds proves D1: the
// third-party subcommand (fetch and install) is reachable both via
// `trackfw agents` and `trackfw skills`, not only skills (every other test
// in this file drives newSkillsCmd() for brevity).
func TestThirdPartyCmd_ReachableFromBothKinds(t *testing.T) {
	for _, root := range []*cobra.Command{newAgentsCmd(), newSkillsCmd()} {
		for _, path := range [][]string{{"third-party", "fetch"}, {"third-party", "install"}} {
			if child, _, err := root.Find(path); err != nil || child == root {
				t.Fatalf("%s missing subcommand %v", root.Name(), path)
			}
		}
	}
}

// TestThirdPartyInstall_ViaAgentsCmdRecordsAgentKindAndSkillDestination
// proves the full fetch→install cycle also works when invoked through
// `trackfw agents third-party` (not just `trackfw skills third-party`):
// the quarantine record's kind must be "agent", and — per D5 — the artifact
// still lands under "skills/thirdparty/", never under "agents/thirdparty/".
func TestThirdPartyInstall_ViaAgentsCmdRecordsAgentKindAndSkillDestination(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	withOrchestratorSession(t)
	stubThirdPartyFetch(t, []byte(benignThirdPartyContent))

	fetch := newAgentsCmd()
	fetch.SetArgs([]string{"third-party", "fetch", "https://example.com/skills/my-skill.md"})
	var fetchOut bytes.Buffer
	fetch.SetOut(&fetchOut)
	fetch.SetErr(&fetchOut)
	if err := fetch.Execute(); err != nil {
		t.Fatalf("agents third-party fetch failed: %v\noutput:\n%s", err, fetchOut.String())
	}
	var checksum string
	for _, line := range strings.Split(fetchOut.String(), "\n") {
		if strings.HasPrefix(line, "checksum: ") {
			checksum = strings.TrimPrefix(line, "checksum: ")
		}
	}
	if checksum == "" {
		t.Fatalf("no checksum printed, output:\n%s", fetchOut.String())
	}

	quarantinePath := filepath.Join(project, ".trackfw", "thirdparty-quarantine", checksum+".json")
	raw, err := os.ReadFile(quarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record["kind"] != "agent" {
		t.Fatalf("expected quarantine record kind %q, got %v", "agent", record["kind"])
	}

	url := "https://example.com/skills/my-skill.md"
	dest := ".claude/skills/thirdparty/my-skill.md"
	if err := thirdparty.UpsertProvenanceEntry(project, dest, thirdparty.ProvenanceEntry{
		URL: url, ChecksumSHA256: checksum, InstalledAt: "2026-08-15T00:00:00Z",
		ApprovedBy: "hades-tf", ReviewReference: "docs/seguranca/test.md", Scope: "project",
	}); err != nil {
		t.Fatal(err)
	}

	install := newAgentsCmd()
	install.SetArgs([]string{
		"third-party", "install",
		"--checksum", checksum,
		"--targets", "claude",
		"--yes-i-trust-this-source",
	})
	var installOut bytes.Buffer
	install.SetOut(&installOut)
	install.SetErr(&installOut)
	if err := install.Execute(); err != nil {
		t.Fatalf("agents third-party install failed: %v\noutput:\n%s", err, installOut.String())
	}

	if _, err := os.Stat(filepath.Join(project, ".claude", "skills", "thirdparty", "my-skill.md")); err != nil {
		t.Fatalf("expected third-party artifact under skills/thirdparty even when invoked via `agents`: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude", "agents", "thirdparty")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("third-party artifact must never land under agents/thirdparty (D5), stat err: %v", err)
	}
}
