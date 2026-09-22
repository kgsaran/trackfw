package thirdparty

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

// quarantineSchemaVersion is the schema_version written to every quarantine
// record. Bump only alongside a migration path — see ReadQuarantine, which
// refuses any other value rather than guessing at a compatible shape.
const quarantineSchemaVersion = 1

// MarkerCheck records the outcome of the D3 marker-based refusal check that
// was performed on the fetched content, embedded verbatim in the
// quarantine entry so a reviewer can see why an artifact was flagged
// without re-running CheckMarkers against the original URL.
type MarkerCheck struct {
	Result         string   `json:"result"`
	MatchedMarkers []string `json:"matched_markers"`
}

// QuarantineEntry is the on-disk shape of one fetched third-party artifact
// (D8a/b). It is keyed by its own SHA-256 checksum — the filename IS the
// checksum — which makes the record self-verifying and idempotent:
// re-fetching identical content overwrites the same file with
// byte-identical data, and a caller can always confirm a record matches
// its checksum by recomputing Checksum(content) after decoding.
type QuarantineEntry struct {
	SchemaVersion    int         `json:"schema_version"`
	URL              string      `json:"url"`
	ChecksumSHA256   string      `json:"checksum_sha256"`
	FetchedAt        string      `json:"fetched_at"`
	ContentBase64    string      `json:"content_base64"`
	MarkerCheck      MarkerCheck `json:"marker_check"`
	Kind             string      `json:"kind"`
	RequestedTargets []string    `json:"requested_targets"`
}

// NewQuarantineEntry builds a QuarantineEntry from freshly fetched content.
// matchedMarkers is CheckMarkers' return value for raw; an empty slice
// yields marker_check.result == "pass". The content is embedded whole,
// base64-encoded, in content_base64 — never a path to another file. This is
// deliberate (D8b): an indirection through a second file would reopen the
// TOCTOU window the quarantine record exists to close.
//
// rawURL is stored via RedactURL, not verbatim (D6-bis): the quarantine
// record is committed to git, and a pre-signed URL's query string can carry
// a bearer token that must never become a permanent secret in history. The
// unredacted URL was already used, in memory only, for the fetch itself
// (D7) before this constructor is ever called.
func NewQuarantineEntry(rawURL string, raw []byte, matchedMarkers []string, kind string, requestedTargets []string) QuarantineEntry {
	result := "pass"
	if len(matchedMarkers) > 0 {
		result = "fail"
	}
	return QuarantineEntry{
		SchemaVersion:  quarantineSchemaVersion,
		URL:            RedactURL(rawURL),
		ChecksumSHA256: Checksum(raw),
		FetchedAt:      time.Now().UTC().Format(time.RFC3339),
		ContentBase64:  base64.StdEncoding.EncodeToString(raw),
		MarkerCheck: MarkerCheck{
			Result:         result,
			MatchedMarkers: matchedMarkers,
		},
		Kind:             kind,
		RequestedTargets: requestedTargets,
	}
}

// QuarantinePath returns the on-disk path of the quarantine record for
// checksum, rooted at root — the project or home directory the caller is
// operating on, mirroring the root convention of
// internal/integrations.Manager.ProjectRoot/HomeDir rather than assuming
// the current working directory.
func QuarantinePath(root, checksum string) string {
	return filepath.Join(root, ".trackfw", "thirdparty-quarantine", checksum+".json")
}

// WriteQuarantine persists entry atomically at
// QuarantinePath(root, entry.ChecksumSHA256), using the same
// os.CreateTemp + os.Rename pattern as
// internal/integrations/manager.go's atomicWrite.
func WriteQuarantine(root string, entry QuarantineEntry) error {
	entry.SchemaVersion = quarantineSchemaVersion
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode quarantine entry: %w", err)
	}
	data = append(data, '\n')
	dest := QuarantinePath(root, entry.ChecksumSHA256)
	// GuardedWrite applies RejectSymlinks(root, dest) before any filesystem
	// mutation — closing the "symlink in ancestor" write-escape described in
	// REQ-2026-08-31 / ADR-2026-09-18. It also replaces the private
	// atomicWrite that previously lived in this package (declared there as a
	// mirror of internal/integrations/manager.go's atomicWrite — see the
	// atomicWrite doc comment that was removed in ML-1B).
	if err := pathguard.GuardedWrite(filepath.Clean(root), dest, data, 0o600); err != nil {
		return fmt.Errorf("write quarantine entry: %w", err)
	}
	return nil
}

// ReadQuarantine reads and validates the quarantine record for checksum,
// fail-closed (D8f): a missing file, invalid JSON, or an unsupported
// schema_version are all returned as errors, never degraded to a zero
// value. This mirrors the rigor of internal/integrations/manifest.go's
// loadManifest, with one intentional difference: there, a missing file
// means "nothing installed yet" and is not an error; here, the caller
// already holds a checksum obtained from a prior fetch and is asking for
// that specific record, so its absence is itself the failure being
// guarded against (D8f: "arquivo ausente onde é exigido").
func ReadQuarantine(root, checksum string) (QuarantineEntry, error) {
	filename := QuarantinePath(root, checksum)
	data, err := os.ReadFile(filename)
	if err != nil {
		return QuarantineEntry{}, fmt.Errorf("read quarantine entry %q: %w", checksum, err)
	}
	var entry QuarantineEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return QuarantineEntry{}, fmt.Errorf("decode quarantine entry %q: %w", checksum, err)
	}
	if entry.SchemaVersion != quarantineSchemaVersion {
		return QuarantineEntry{}, fmt.Errorf("unsupported quarantine schema %d for %q", entry.SchemaVersion, checksum)
	}
	return entry, nil
}

// DecodeContent decodes e.ContentBase64 back to the original raw bytes.
func (e QuarantineEntry) DecodeContent() ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(e.ContentBase64)
	if err != nil {
		return nil, fmt.Errorf("decode quarantine content: %w", err)
	}
	return raw, nil
}

// atomicWrite was removed in ML-1B (REQ-2026-08-31 / ADR-2026-09-18).
// Its callers now use pathguard.GuardedWrite, which applies RejectSymlinks
// before the atomic write. provenance.go was also updated in the same ML.
