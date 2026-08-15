package thirdparty

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// provenanceSchemaVersion is the schema_version written to and required by
// the provenance file. Bump only alongside a migration path — see
// LoadProvenance, which refuses any other value.
//
// Bumped 1 -> 2 (ADR-2026-08-15 D2-bis, ML-3B) to add InstalledSHA256. No
// migration path exists or is needed: at the time of the bump this feature
// had not shipped, so no provenance file existed anywhere with
// schema_version 1 — correcting at the source was free. LoadProvenance
// still refuses any version other than the current one, fail-closed.
const provenanceSchemaVersion = 2

// ProvenanceEntry records how one destination came to hold third-party
// content: which URL it came from, the checksum the approver reviewed, the
// checksum of what was actually written to disk, who approved it, and a
// reference to the human review that justified the approval (D6).
//
// Two different checksums live on this struct, in two different domains,
// and D2-bis exists specifically because conflating them produces a
// systematic false positive:
//
//   - ChecksumSHA256 — SHA-256 of the RAW bytes fetched from the network,
//     before any normalization (D6). This is the approval anchor (D8c): it
//     is the exact byte sequence hades-tf (or another approver) reviewed
//     and is never touched by the install step. Written by the external
//     approver, not by any subcommand — see D10.2.
//   - InstalledSHA256 — SHA-256 of the NORMALIZED bytes
//     (integrations.NormalizeThirdPartyContent, i.e. TrimSpace(raw)+"\n"),
//     computed by executeThirdPartyInstall at install time, by the exact
//     same code path that writes the destination file. This is what
//     validateThirdPartyArtifactHasProvenance's branch (ii) compares
//     against the installed file's own hash — same domain on both sides,
//     no bridge artifact (like the quarantine record) required.
//
// Comparing ChecksumSHA256 directly against sha256(installed file) — the
// literal reading of ADR-2026-08-15 D2's branch (ii) text — is WRONG: the
// installed file is always normalized, so any raw content that was not
// already exactly TrimSpace+"\n" (the common case: any file with a trailing
// blank line) would false-positive as "tampered" on every validate run.
// InstalledSHA256 exists to make branch (ii) compare like domains.
type ProvenanceEntry struct {
	URL             string `json:"url"`
	ChecksumSHA256  string `json:"checksum_sha256"`
	InstalledSHA256 string `json:"installed_sha256"`
	InstalledAt     string `json:"installed_at"`
	ApprovedBy      string `json:"approved_by"`
	ReviewReference string `json:"review_reference"`
	Scope           string `json:"scope"`
	MarkerOverride  bool   `json:"marker_override"`
}

// Provenance is keyed by destination — not appended chronologically —
// because the validate rule that consumes it
// (thirdparty_artifact_has_provenance, Wave 3) needs an O(1) lookup by
// destination to decide whether a managed file has a matching approval,
// not a history of installs.
type Provenance struct {
	SchemaVersion int                        `json:"schema_version"`
	Entries       map[string]ProvenanceEntry `json:"entries"`
}

func emptyProvenance() Provenance {
	return Provenance{SchemaVersion: provenanceSchemaVersion, Entries: make(map[string]ProvenanceEntry)}
}

// ProvenancePath returns the on-disk path of the provenance file, rooted at
// root — the project or home directory the caller is operating on.
func ProvenancePath(root string) string {
	return filepath.Join(root, ".trackfw", "thirdparty-provenance.json")
}

// LoadProvenance reads and validates the provenance file, mirroring
// internal/integrations/manifest.go's loadManifest: a missing file is a
// legitimate "nothing installed from a third party yet" state and returns
// an empty, schema-valid Provenance; invalid JSON or an unsupported
// schema_version are fatal and returned as errors — never silently
// degraded to empty (D8f). Callers that require a specific entry to exist
// (VerifyApproval) get their own fail-closed behavior from the entry
// lookup below, not from this function refusing to run on a missing file.
func LoadProvenance(root string) (Provenance, error) {
	filename := ProvenancePath(root)
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return emptyProvenance(), nil
	}
	if err != nil {
		return Provenance{}, fmt.Errorf("read thirdparty provenance: %w", err)
	}
	var prov Provenance
	if err := json.Unmarshal(data, &prov); err != nil {
		return Provenance{}, fmt.Errorf("decode thirdparty provenance: %w", err)
	}
	if prov.SchemaVersion != provenanceSchemaVersion {
		return Provenance{}, fmt.Errorf("unsupported thirdparty provenance schema %d", prov.SchemaVersion)
	}
	if prov.Entries == nil {
		prov.Entries = make(map[string]ProvenanceEntry)
	}
	return prov, nil
}

// WriteProvenance persists prov atomically.
//
// Failure here MUST propagate to the caller and abort the installation
// (D6). This is the deliberate opposite of
// internal/generators/roadmap.go's appendTransitionLog (~line 456), which
// opens its log file best-effort and silently does nothing on error:
// provenance is the only record of who approved a third-party artifact, so
// losing a write silently would leave an unapproved artifact on disk
// indistinguishable from an approved one. Do not copy the
// appendTransitionLog pattern for this file.
//
// Every entry's URL is passed through RedactURL before serialization
// (D6-bis) — a defense-in-depth boundary, not the only one: no command in
// this codebase writes ProvenanceEntry.URL today (D10.2 — the external
// approver writes the entry directly to the versioned JSON), but this call
// site guarantees the query string is never persisted here even if a
// caller passes an unredacted URL (e.g. copied verbatim from a quarantine
// record written before this fix). Idempotent: redacting an
// already-redacted URL is a no-op.
func WriteProvenance(root string, prov Provenance) error {
	prov.SchemaVersion = provenanceSchemaVersion
	if prov.Entries == nil {
		prov.Entries = make(map[string]ProvenanceEntry)
	}
	for dest, entry := range prov.Entries {
		entry.URL = RedactURL(entry.URL)
		prov.Entries[dest] = entry
	}
	data, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return fmt.Errorf("encode thirdparty provenance: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(ProvenancePath(root), data, 0o600); err != nil {
		return fmt.Errorf("write thirdparty provenance: %w", err)
	}
	return nil
}

// UpsertProvenanceEntry loads the current provenance, sets
// entries[dest] = entry, and writes it back. The write is
// fatal-on-failure (see WriteProvenance) — callers MUST treat a non-nil
// error as "installation aborted", never log-and-continue.
func UpsertProvenanceEntry(root, dest string, entry ProvenanceEntry) error {
	prov, err := LoadProvenance(root)
	if err != nil {
		return err
	}
	prov.Entries[dest] = entry
	return WriteProvenance(root, prov)
}

// VerifyApproval is the D8c TOCTOU close: it only succeeds if dest has a
// provenance entry whose checksum_sha256 matches checksum exactly and
// whose approved_by is non-empty. A loose "approved" boolean is rejected
// by construction — approval is only meaningful when tied to the exact
// bytes it was granted for, so a destination with no entry, an entry for a
// different checksum, or an entry with an empty approved_by all fail
// identically: not approved.
func VerifyApproval(root, checksum, dest string) error {
	prov, err := LoadProvenance(root)
	if err != nil {
		return err
	}
	entry, ok := prov.Entries[dest]
	if !ok {
		return fmt.Errorf("no provenance entry for destination %q: not approved", dest)
	}
	if entry.ChecksumSHA256 != checksum {
		return fmt.Errorf("provenance checksum mismatch for %q: approved %s, got %s", dest, entry.ChecksumSHA256, checksum)
	}
	if entry.ApprovedBy == "" {
		return fmt.Errorf("provenance entry for %q has no approved_by", dest)
	}
	return nil
}
