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
