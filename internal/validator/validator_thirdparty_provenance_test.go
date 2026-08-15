package validator

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256HexForTest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func writeThirdPartyManifest(t *testing.T, root, destination, origin string) {
	t.Helper()
	claim := map[string]interface{}{
		"target":  "claude",
		"surface": "code",
		"scope":   "project",
		"kind":    "skills",
		"item":    "thirdparty-example",
	}
	if origin != "" {
		claim["origin"] = origin
	}
	manifest := map[string]interface{}{
		"schema_version": 1,
		"artifacts": map[string]interface{}{
			destination: map[string]interface{}{
				"destination":     destination,
				"sha256":          "irrelevant-for-this-rule",
				"catalog_version": "thirdparty:abcdef123456",
				"claims":          []interface{}{claim},
			},
		},
	}
	writeJSONFile(t, filepath.Join(root, ".trackfw", "integrations-manifest.json"), manifest)
}

// writeThirdPartyProvenance keys the entry by destination MADE RELATIVE TO
// root — provenance is keyed by the project-root-relative path
// (ResolveThirdPartySkillDestination's return value, before
// Manager.resolve() joins it against root), never by the manifest's
// absolute destination. Verified empirically against the real install
// command (see this ML's delivery report); do not "fix" this back to an
// absolute key, it would silently break the rule.
func writeThirdPartyProvenance(t *testing.T, root, destination, checksum string) {
	t.Helper()
	relDest, err := filepath.Rel(root, destination)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	prov := map[string]interface{}{
		"schema_version": 1,
		"entries": map[string]interface{}{
			relDest: map[string]interface{}{
				"url":              "https://example.com/skill.md",
				"checksum_sha256":  checksum,
				"installed_at":     "2026-08-15T00:00:00Z",
				"approved_by":      "hades-tf",
				"review_reference": "docs/seguranca/example.md",
				"scope":            "project",
				"marker_override":  false,
			},
		},
	}
	writeJSONFile(t, filepath.Join(root, ".trackfw", "thirdparty-provenance.json"), prov)
}

func writeThirdPartyQuarantine(t *testing.T, root string, rawContent []byte) string {
	t.Helper()
	checksum := sha256HexForTest(rawContent)
	entry := map[string]interface{}{
		"schema_version":  1,
		"url":             "https://example.com/skill.md",
		"checksum_sha256": checksum,
		"fetched_at":      "2026-08-15T00:00:00Z",
		"content_base64":  base64.StdEncoding.EncodeToString(rawContent),
		"marker_check":    map[string]interface{}{"result": "pass", "matched_markers": []string{}},
		"kind":            "skill",
		"requested_targets": []string{
			"claude",
		},
	}
	writeJSONFile(t, filepath.Join(root, ".trackfw", "thirdparty-quarantine", checksum+".json"), entry)
	return checksum
}

func writeJSONFile(t *testing.T, path string, v interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// resolvedRoot returns dir with symlinks resolved, mirroring what
// validateThirdPartyArtifactHasProvenance does internally to os.Getwd() —
// needed so the manifest destination key (an absolute path we construct in
// the test) matches what the rule resolves os.Getwd() to, on platforms
// (macOS) where t.TempDir() lives under a symlinked prefix.
func resolvedRootForTest(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return resolved
}

func TestThirdPartyArtifactHasProvenance_CleanNoManifest(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no violations, got: %v", msgs)
	}
}

func TestThirdPartyArtifactHasProvenance_CatalogClaimNeverFlagged(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)
	destination := filepath.Join(root, "skill.md")
	if err := os.WriteFile(destination, []byte("catalog content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// origin == "" (catalog) — must never be checked against provenance.
	writeThirdPartyManifest(t, root, destination, "")

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no violations for a catalog claim, got: %v", msgs)
	}
}

// TestThirdPartyArtifactHasProvenance_LegacyManifestNoOriginField is the
// explicit retrocompatibility test required by ML-3A's acceptance
// criteria: a manifest written before Claim.Origin existed has NO "origin"
// key at all in its claim JSON (not even an empty string) — this must
// decode to the zero value and be read as a catalog claim, never flagged.
func TestThirdPartyArtifactHasProvenance_LegacyManifestNoOriginField(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)
	destination := filepath.Join(root, "agent.md")
	if err := os.WriteFile(destination, []byte("legacy agent content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Hand-authored manifest, exactly as trackfw wrote it before D11 —
	// literally no "origin" key anywhere in the claim object.
	legacyManifest := `{
  "schema_version": 1,
  "artifacts": {
    ` + jsonQuote(destination) + `: {
      "destination": ` + jsonQuote(destination) + `,
      "sha256": "irrelevant",
      "catalog_version": "v1",
      "claims": [
        {"target": "claude", "surface": "code", "scope": "project", "kind": "agents", "item": "backend"}
      ]
    }
  }
}
`
	manifestPath := filepath.Join(root, ".trackfw", "integrations-manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(legacyManifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error reading legacy (pre-origin) manifest: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("legacy manifest with no origin field must read as catalog (no violations), got: %v", msgs)
	}
}

func jsonQuote(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

func TestThirdPartyArtifactHasProvenance_BranchI_MissingProvenanceEntry(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)
	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	if err := os.WriteFile(destinationEnsureDir(t, destination), []byte("some content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	// No provenance file at all.

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "D2 branch i") {
		t.Fatalf("expected message to reference D2 branch i, got: %s", msgs[0])
	}
	if !strings.Contains(msgs[0], destination) {
		t.Fatalf("expected message to name the destination, got: %s", msgs[0])
	}
}

func destinationEnsureDir(t *testing.T, destination string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return destination
}

// TestThirdPartyArtifactHasProvenance_BranchII_LegitimateInstallDoesNotFalsePositive
// is the test the advisor flagged as load-bearing: raw fetched content that
// is NOT already canonical (trailing blank line) must still validate clean
// when the destination holds exactly NormalizeThirdPartyContent(raw) — the
// real output of a correct install. A naive "hash the installed file and
// compare to checksum_sha256" implementation FAILS this test, because
// checksum_sha256 is sha256(raw), not sha256(normalized). Do not weaken this
// fixture to already-canonical content; that would hide the exact bug this
// rule's resolution exists to avoid (see this file's package doc / ML-3A
// delivery report and docs/roadmaps/.trackfw-attention.json history).
func TestThirdPartyArtifactHasProvenance_BranchII_LegitimateInstallDoesNotFalsePositive(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)

	// Deliberately NOT canonical: leading blank line, trailing blank lines.
	raw := []byte("\n# hello\n\nsome content\n\n\n")
	normalized := []byte(strings.TrimSpace(string(raw)) + "\n")
	if string(raw) == string(normalized) {
		t.Fatal("test fixture is not actually testing the raw/normalized divergence")
	}

	checksum := writeThirdPartyQuarantine(t, root, raw)
	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	if err := os.WriteFile(destinationEnsureDir(t, destination), normalized, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	writeThirdPartyProvenance(t, root, destination, checksum)

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("a legitimate install with non-canonical raw content must not be flagged, got: %v", msgs)
	}
}

func TestThirdPartyArtifactHasProvenance_BranchII_TamperedAfterApprovalIsCaught(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)

	raw := []byte("# hello\n\nsome content\n")
	checksum := writeThirdPartyQuarantine(t, root, raw)
	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	// Installed content diverges from NormalizeThirdPartyContent(raw) — as
	// if someone hand-edited the file after approval.
	tampered := []byte("# hello\n\nTAMPERED CONTENT\n")
	if err := os.WriteFile(destinationEnsureDir(t, destination), tampered, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	writeThirdPartyProvenance(t, root, destination, checksum)

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "D2 branch ii") {
		t.Fatalf("expected message to reference D2 branch ii, got: %s", msgs[0])
	}
}

func TestThirdPartyArtifactHasProvenance_BranchII_MissingQuarantineFailsClosed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)

	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	if err := os.WriteFile(destinationEnsureDir(t, destination), []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	// Provenance entry references a checksum with no matching quarantine
	// record on disk (e.g. the quarantine directory was pruned).
	writeThirdPartyProvenance(t, root, destination, strings.Repeat("a", 64))

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 violation (fail-closed), got %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "D8f") {
		t.Fatalf("expected message to reference fail-closed D8f, got: %s", msgs[0])
	}
}
