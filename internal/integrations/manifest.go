package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const manifestSchemaVersion = 1

// Claim identifies one logical consumer of a physical artifact. Several claims
// may intentionally share the same destination.
type Claim struct {
	Target  string   `json:"target"`
	Surface string   `json:"surface"`
	Scope   string   `json:"scope"`
	Kind    ItemKind `json:"kind"`
	Item    string   `json:"item"`
	// Origin distinguishes a claim on catalog content from a claim on a
	// third-party artifact (ADR-2026-08-15 D11): "" (zero-value) means
	// catalog — every manifest written before this field existed decodes
	// to "" here, so no migration is needed; "thirdparty" means the claim
	// was created by `third-party install`. This is the field the
	// thirdparty_artifact_has_provenance validate rule uses to identify a
	// third-party destination — see that rule's doc comment for why the
	// alternatives (path sniffing, using provenance itself as the index)
	// were rejected.
	Origin string `json:"origin,omitempty"`
}

// Manifest records only artifacts whose ownership has been established by
// trackfw. Keys are absolute, cleaned destination paths.
type Manifest struct {
	SchemaVersion int                         `json:"schema_version"`
	Artifacts     map[string]ManifestArtifact `json:"artifacts"`
}

type ManifestArtifact struct {
	Destination    string  `json:"destination"`
	Hash           string  `json:"sha256"`
	CatalogVersion string  `json:"catalog_version"`
	Claims         []Claim `json:"claims"`
}

func emptyManifest() Manifest {
	return Manifest{SchemaVersion: manifestSchemaVersion, Artifacts: make(map[string]ManifestArtifact)}
}

func loadManifest(filename string) (Manifest, error) {
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return emptyManifest(), nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read integration manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode integration manifest: %w", err)
	}
	if manifest.SchemaVersion != manifestSchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported integration manifest schema %d", manifest.SchemaVersion)
	}
	if manifest.Artifacts == nil {
		manifest.Artifacts = make(map[string]ManifestArtifact)
	}
	return manifest, nil
}

func writeManifest(filename string, manifest Manifest) error {
	manifest.SchemaVersion = manifestSchemaVersion
	if manifest.Artifacts == nil {
		manifest.Artifacts = make(map[string]ManifestArtifact)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode integration manifest: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(filename, data, 0o600); err != nil {
		return fmt.Errorf("write integration manifest: %w", err)
	}
	return nil
}

func manifestPath(root string) string {
	return filepath.Join(root, ".trackfw", "integrations-manifest.json")
}

// ManifestPath is the exported form of manifestPath, for callers outside
// this package that need to locate .trackfw/integrations-manifest.json
// without duplicating the join (e.g. internal/validator's
// thirdparty_artifact_has_provenance rule, ADR-2026-08-15 D2/D11).
func ManifestPath(root string) string {
	return manifestPath(root)
}

// LoadManifest is the exported form of loadManifest, for read-only
// consumers outside this package (internal/validator). It has the exact
// same fail-closed behavior: a missing file returns an empty, schema-valid
// Manifest; invalid JSON or an unsupported schema_version are returned as
// errors, never silently degraded.
func LoadManifest(root string) (Manifest, error) {
	return loadManifest(manifestPath(root))
}
