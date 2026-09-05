package validator

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
//
// checksum is the D6 raw-bytes approval anchor (checksum_sha256);
// installedSHA256 is the D2-bis field branch (ii) actually checks against
// the installed file's own hash. Callers that only exercise branch (i), or
// that intentionally want branch (ii) to diverge, pass whichever value
// their scenario needs — the two are deliberately independent parameters,
// never derived from one another, matching production where one is written
// by the external approver and the other by the install command.
func writeThirdPartyProvenance(t *testing.T, root, destination, checksum, installedSHA256 string) {
	t.Helper()
	relDest, err := filepath.Rel(root, destination)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	// ML-2A (ADR-2026-09-04, D1 categoria 2): a chave e normalizada porque e o que a
	// PRODUCAO grava — ResolveThirdPartySkillDestination monta o destino por
	// concatenacao explicita com "/" (integrations/render.go:821), em qualquer SO.
	// Sem isto a fixture montava a chave com filepath.Rel (separador NATIVO) e, no
	// Windows, deixava de casar com a chave normalizada que o produto procura
	// (validator_thirdparty_provenance.go:160) — a fixture reprovava o produto CERTO.
	relDest = normalizeRefSeparator(relDest)
	prov := map[string]interface{}{
		"schema_version": 2,
		"entries": map[string]interface{}{
			relDest: map[string]interface{}{
				"url":              "https://example.com/skill.md",
				"checksum_sha256":  checksum,
				"installed_sha256": installedSHA256,
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
	// A mensagem de produção (validateThirdPartyArtifactHasProvenance,
	// validator_thirdparty_provenance.go) embute "destination" via "%q", que escapa cada "\"
	// nativo do Windows como "\\" — comparar contra o literal cru nunca bate nesse SO.
	destinationQuoted := fmt.Sprintf("%q", destination)
	if !strings.Contains(msgs[0], destinationQuoted) {
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
// is the test the advisor flagged as load-bearing (ML-3A) and D2-bis
// (ML-3B) preserves: raw fetched content that is NOT already canonical
// (leading/trailing blank lines) must still validate clean when the
// destination holds exactly NormalizeThirdPartyContent(raw) and the
// provenance entry's installed_sha256 is sha256 of THAT normalized content
// — the real output of a correct install. A naive "hash the installed file
// and compare to checksum_sha256" implementation FAILS this test, because
// checksum_sha256 is sha256(raw), not sha256(normalized); this fixture
// deliberately keeps checksum_sha256 and installed_sha256 at DIFFERENT
// values (sha256(raw) vs sha256(normalized)) to prove branch (ii) is
// comparing against installed_sha256, not checksum_sha256. No quarantine
// record is written anywhere in this test — D2-bis's whole point is that
// branch (ii) no longer touches the quarantine directory. Do not weaken
// this fixture to already-canonical content; that would hide the exact bug
// this rule's resolution exists to avoid (see this file's package doc).
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
	checksumOfRaw := sha256HexForTest(raw)
	installedSHA256 := sha256HexForTest(normalized)
	if checksumOfRaw == installedSHA256 {
		t.Fatal("test fixture must keep checksum_sha256 and installed_sha256 distinct")
	}

	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	if err := os.WriteFile(destinationEnsureDir(t, destination), normalized, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	writeThirdPartyProvenance(t, root, destination, checksumOfRaw, installedSHA256)
	// No quarantine directory exists at all — proves branch (ii) does not
	// depend on it (D2-bis).

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
	normalized := []byte(strings.TrimSpace(string(raw)) + "\n")
	installedSHA256 := sha256HexForTest(normalized)

	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	// Installed content diverges from what installed_sha256 was recorded
	// for — as if someone hand-edited the file after approval. No
	// quarantine record exists anywhere in this test (D2-bis).
	tampered := []byte("# hello\n\nTAMPERED CONTENT\n")
	if err := os.WriteFile(destinationEnsureDir(t, destination), tampered, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	writeThirdPartyProvenance(t, root, destination, sha256HexForTest(raw), installedSHA256)

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

// TestThirdPartyArtifactHasProvenance_BranchII_MissingInstalledSHA256IsCaught
// covers the reachable state of an approver-authored provenance entry
// (D10.2) that has never been through `install` — installed_sha256 is
// ABSENT from the JSON entirely, not merely empty. This is mostly a
// Go/Node/Python parity anchor: Node's naive `entry.installed_sha256` on a
// missing key is `undefined`, which — if interpolated directly into the
// violation message instead of coerced to "" first — renders the JS-only
// literal "undefined" where Go and Python render an empty string, breaking
// byte parity. This test pins the message text so that regression cannot
// land silently.
func TestThirdPartyArtifactHasProvenance_BranchII_MissingInstalledSHA256IsCaught(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)

	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	if err := os.WriteFile(destinationEnsureDir(t, destination), []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")

	// Hand-authored: no "installed_sha256" key at all, exactly what an
	// approver (never having run `install`) would write.
	relDest, err := filepath.Rel(root, destination)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	// ML-2A (ADR-2026-09-04, D1 categoria 2): a chave e normalizada porque e o que a
	// PRODUCAO grava — ResolveThirdPartySkillDestination monta o destino por
	// concatenacao explicita com "/" (integrations/render.go:821), em qualquer SO.
	// Sem isto a fixture montava a chave com filepath.Rel (separador NATIVO) e, no
	// Windows, deixava de casar com a chave normalizada que o produto procura
	// (validator_thirdparty_provenance.go:160) — a fixture reprovava o produto CERTO.
	relDest = normalizeRefSeparator(relDest)
	prov := map[string]interface{}{
		"schema_version": 2,
		"entries": map[string]interface{}{
			relDest: map[string]interface{}{
				"url":              "https://example.com/skill.md",
				"checksum_sha256":  strings.Repeat("a", 64),
				"installed_at":     "2026-08-15T00:00:00Z",
				"approved_by":      "hades-tf",
				"review_reference": "docs/seguranca/example.md",
				"scope":            "project",
				"marker_override":  false,
			},
		},
	}
	writeJSONFile(t, filepath.Join(root, ".trackfw", "thirdparty-provenance.json"), prov)

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %v", len(msgs), msgs)
	}
	if strings.Contains(msgs[0], "undefined") {
		t.Fatalf("message must not contain the JS-only literal 'undefined': %s", msgs[0])
	}
	if !strings.Contains(msgs[0], "installed_sha256  recorded") {
		t.Fatalf("expected empty installed_sha256 rendered as an empty string, got: %s", msgs[0])
	}
}

// TestThirdPartyArtifactHasProvenance_BranchII_QuarantineDeletionDoesNotBreakCleanInstall
// and its sibling below are the load-bearing tests ML-3B exists for
// (ADR-2026-08-15 D2-bis): they reproduce the exact scenario the ML-3A
// design got wrong — build a REAL end-to-end state including a quarantine
// record (as `fetch` would leave it), then delete
// .trackfw/thirdparty-quarantine/ ENTIRELY, and confirm branch (ii) still
// works correctly. This replaces
// TestThirdPartyArtifactHasProvenance_BranchII_MissingQuarantineFailsClosed
// (ML-3A), whose entire premise — that a missing quarantine record must
// fail validate closed — was the footgun D2-bis was written to remove: a
// STAGING directory (name, shape, and location of a directory meant to be
// pruned) can no longer be a hard, unrecoverable dependency of a PERMANENT
// gate. There is no D8f fail-closed case left in branch (ii) for a missing
// quarantine record, because branch (ii) does not read the quarantine
// directory at all anymore.
func TestThirdPartyArtifactHasProvenance_BranchII_QuarantineDeletionDoesNotBreakCleanInstall(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)

	raw := []byte("\n# hello\n\nsome content\n\n\n") // non-canonical, same as above
	normalized := []byte(strings.TrimSpace(string(raw)) + "\n")
	checksumOfRaw := writeThirdPartyQuarantine(t, root, raw) // real quarantine record, as `fetch` would write it

	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	if err := os.WriteFile(destinationEnsureDir(t, destination), normalized, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	writeThirdPartyProvenance(t, root, destination, checksumOfRaw, sha256HexForTest(normalized))

	// Delete the ENTIRE quarantine directory — the ML-3B acceptance
	// criterion, verbatim.
	if err := os.RemoveAll(filepath.Join(root, ".trackfw", "thirdparty-quarantine")); err != nil {
		t.Fatalf("RemoveAll quarantine: %v", err)
	}

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("a genuinely clean install must not be flagged just because the quarantine directory was pruned, got: %v", msgs)
	}
}

func TestThirdPartyArtifactHasProvenance_BranchII_QuarantineDeletionStillDetectsTamper(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	root := resolvedRootForTest(t, dir)

	raw := []byte("# hello\n\nsome content\n")
	normalized := []byte(strings.TrimSpace(string(raw)) + "\n")
	checksumOfRaw := writeThirdPartyQuarantine(t, root, raw)

	destination := filepath.Join(root, "skills", "thirdparty", "example.md")
	tampered := []byte("# hello\n\nTAMPERED CONTENT\n")
	if err := os.WriteFile(destinationEnsureDir(t, destination), tampered, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeThirdPartyManifest(t, root, destination, "thirdparty")
	writeThirdPartyProvenance(t, root, destination, checksumOfRaw, sha256HexForTest(normalized))

	if err := os.RemoveAll(filepath.Join(root, ".trackfw", "thirdparty-quarantine")); err != nil {
		t.Fatalf("RemoveAll quarantine: %v", err)
	}

	msgs, err := validateThirdPartyArtifactHasProvenance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("tamper detection must survive quarantine deletion, got %d violations: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "D2 branch ii") {
		t.Fatalf("expected message to reference D2 branch ii, got: %s", msgs[0])
	}
}
