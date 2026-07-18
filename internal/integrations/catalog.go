// Package integrations provides the canonical catalog and lifecycle primitives
// used to install trackfw agents and skills in supported AI CLIs.
package integrations

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed assets
var catalogAssets embed.FS

type ItemKind string

const (
	KindAgents ItemKind = "agents"
	KindSkills ItemKind = "skills"
)

type Catalog struct {
	Version string   `json:"version"`
	Agents  []Item   `json:"agents"`
	Skills  []Item   `json:"skills"`
	Targets []Target `json:"targets"`
}

type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Asset       string `json:"asset"`
}

type Target struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Surfaces           []Surface           `json:"surfaces"`
	AuxiliaryArtifacts []AuxiliaryArtifact `json:"auxiliary_artifacts,omitempty"`
}

// Surface distinguishes products or compatibility generations that share a
// vendor target but use different contracts (for example Kiro IDE and CLI).
type Surface struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Scopes       []string     `json:"scopes"`
	Capabilities Capabilities `json:"capabilities"`
	Paths        TargetPaths  `json:"paths"`
}

type Capabilities struct {
	Agents Capability `json:"agents"`
	Skills Capability `json:"skills"`
}

// Capability records both the native mechanism and the fallback used when a
// CLI does not expose first-class agents or skills.
type Capability struct {
	SupportLevel           string `json:"support_level"`
	Representation         string `json:"representation"`
	FallbackRepresentation string `json:"fallback_representation,omitempty"`
}

type TargetPaths struct {
	Agents []InstallPath `json:"agents"`
	Skills []InstallPath `json:"skills"`
}

type InstallPath struct {
	Scope       string            `json:"scope"`
	Path        string            `json:"path"`
	Extension   string            `json:"extension"`
	Frontmatter map[string]string `json:"frontmatter,omitempty"`
}

type AuxiliaryArtifact struct {
	ID       string `json:"id"`
	Surface  string `json:"surface,omitempty"`
	Scope    string `json:"scope"`
	Path     string `json:"path"`
	Purpose  string `json:"purpose"`
	Required bool   `json:"required"`
}

// LoadCatalog parses and validates the catalog embedded in the Go binary.
func LoadCatalog() (*Catalog, error) {
	data, err := catalogAssets.ReadFile("assets/catalog.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded integration catalog: %w", err)
	}
	return ParseCatalog(data)
}

// ParseCatalog is public so package and parity tests can validate generated
// catalogs with the same invariants as the embedded source of truth.
func ParseCatalog(data []byte) (*Catalog, error) {
	var catalog Catalog
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode integration catalog: %w", err)
	}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return &catalog, nil
}

// Validate rejects ambiguous catalog entries before any filesystem operation.
func (c Catalog) Validate() error {
	if strings.TrimSpace(c.Version) == "" {
		return errors.New("integration catalog version is required")
	}
	if err := validateItems(c.Agents, KindAgents); err != nil {
		return err
	}
	if err := validateItems(c.Skills, KindSkills); err != nil {
		return err
	}

	itemIDs := make(map[string]ItemKind, len(c.Agents)+len(c.Skills))
	for _, group := range []struct {
		kind  ItemKind
		items []Item
	}{{KindAgents, c.Agents}, {KindSkills, c.Skills}} {
		for _, item := range group.items {
			if previous, exists := itemIDs[item.ID]; exists {
				return fmt.Errorf("duplicate item id %q in %s and %s", item.ID, previous, group.kind)
			}
			itemIDs[item.ID] = group.kind
		}
	}

	targetIDs := make(map[string]struct{}, len(c.Targets))
	for _, target := range c.Targets {
		if target.ID == "" || target.Name == "" {
			return errors.New("target id and name are required")
		}
		if _, exists := targetIDs[target.ID]; exists {
			return fmt.Errorf("duplicate target id %q", target.ID)
		}
		targetIDs[target.ID] = struct{}{}
		if err := validateTarget(target); err != nil {
			return fmt.Errorf("target %q: %w", target.ID, err)
		}
	}
	return nil
}

func validateItems(items []Item, kind ItemKind) error {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ID == "" || item.Name == "" || item.Description == "" || item.Asset == "" {
			return fmt.Errorf("%s item id, name, description and asset are required", kind)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate %s item id %q", kind, item.ID)
		}
		seen[item.ID] = struct{}{}
		expectedPrefix := "assets/" + string(kind) + "/"
		if !strings.HasPrefix(item.Asset, expectedPrefix) || !safeAssetPath(item.Asset) {
			return fmt.Errorf("%s item %q has invalid asset path %q", kind, item.ID, item.Asset)
		}
		if _, err := fs.Stat(catalogAssets, item.Asset); err != nil {
			return fmt.Errorf("%s item %q asset %q: %w", kind, item.ID, item.Asset, err)
		}
	}
	return nil
}

func validateTarget(target Target) error {
	if len(target.Surfaces) == 0 {
		return errors.New("at least one surface is required")
	}
	surfaceIDs := make(map[string]struct{}, len(target.Surfaces))
	allPaths := make(map[string]struct{})
	for _, surface := range target.Surfaces {
		if surface.ID == "" || surface.Name == "" {
			return errors.New("surface id and name are required")
		}
		if _, exists := surfaceIDs[surface.ID]; exists {
			return fmt.Errorf("duplicate surface id %q", surface.ID)
		}
		surfaceIDs[surface.ID] = struct{}{}
		if err := validateSurface(surface, allPaths); err != nil {
			return fmt.Errorf("surface %q: %w", surface.ID, err)
		}
	}

	auxiliaryIDs := make(map[string]struct{}, len(target.AuxiliaryArtifacts))
	for _, artifact := range target.AuxiliaryArtifacts {
		if artifact.ID == "" || artifact.Purpose == "" {
			return errors.New("auxiliary artifact id and purpose are required")
		}
		if _, exists := auxiliaryIDs[artifact.ID]; exists {
			return fmt.Errorf("duplicate auxiliary artifact id %q", artifact.ID)
		}
		auxiliaryIDs[artifact.ID] = struct{}{}
		if artifact.Surface != "" {
			if _, supported := surfaceIDs[artifact.Surface]; !supported {
				return fmt.Errorf("auxiliary artifact %q uses undeclared surface %q", artifact.ID, artifact.Surface)
			}
		}
		if artifact.Scope != "global" && artifact.Scope != "project" {
			return fmt.Errorf("auxiliary artifact %q uses unsupported scope %q", artifact.ID, artifact.Scope)
		}
		if err := validateInstallPath(artifact.Scope, artifact.Path); err != nil {
			return fmt.Errorf("auxiliary artifact %q: %w", artifact.ID, err)
		}
	}
	return nil
}

func validateSurface(surface Surface, seenPaths map[string]struct{}) error {
	scopes := make(map[string]struct{}, len(surface.Scopes))
	for _, scope := range surface.Scopes {
		if scope != "global" && scope != "project" {
			return fmt.Errorf("unsupported scope %q", scope)
		}
		if _, exists := scopes[scope]; exists {
			return fmt.Errorf("duplicate scope %q", scope)
		}
		scopes[scope] = struct{}{}
	}
	if len(scopes) == 0 {
		return errors.New("at least one scope is required")
	}
	for kind, capability := range map[ItemKind]Capability{KindAgents: surface.Capabilities.Agents, KindSkills: surface.Capabilities.Skills} {
		if !validSupportLevel(capability.SupportLevel) {
			return fmt.Errorf("%s support_level %q is invalid", kind, capability.SupportLevel)
		}
		if capability.SupportLevel != "unsupported" && capability.Representation == "" {
			return fmt.Errorf("%s representation is required", kind)
		}
		if capability.SupportLevel == "fallback" && capability.FallbackRepresentation == "" {
			return fmt.Errorf("%s fallback representation is required for non-native capability", kind)
		}
	}

	for kind, paths := range map[ItemKind][]InstallPath{KindAgents: surface.Paths.Agents, KindSkills: surface.Paths.Skills} {
		capability := surface.Capabilities.Agents
		if kind == KindSkills {
			capability = surface.Capabilities.Skills
		}
		if len(paths) == 0 && capability.SupportLevel != "unsupported" {
			return fmt.Errorf("%s path is required", kind)
		}
		if len(paths) != 0 && capability.SupportLevel == "unsupported" {
			return fmt.Errorf("unsupported %s capability cannot declare paths", kind)
		}
		for _, installPath := range paths {
			if _, supported := scopes[installPath.Scope]; !supported {
				return fmt.Errorf("%s path uses undeclared scope %q", kind, installPath.Scope)
			}
			if installPath.Extension == "" || !strings.Contains(installPath.Path, "{{id}}") {
				return fmt.Errorf("%s path %q requires extension and {{id}} placeholder", kind, installPath.Path)
			}
			if err := validateInstallPath(installPath.Scope, installPath.Path); err != nil {
				return fmt.Errorf("%s path: %w", kind, err)
			}
			key := surface.ID + "\x00" + installPath.Scope + "\x00" + installPath.Path
			if _, exists := seenPaths[key]; exists {
				return fmt.Errorf("duplicate install path %q for scope %q", installPath.Path, installPath.Scope)
			}
			seenPaths[key] = struct{}{}
		}
	}

	return nil
}

func validSupportLevel(level string) bool {
	return level == "native" || level == "fallback" || level == "unsupported" || level == "legacy"
}

func safeAssetPath(asset string) bool {
	return asset == path.Clean(asset) && !strings.HasPrefix(asset, "/") && !strings.HasPrefix(asset, "../")
}

func validateInstallPath(scope, destination string) error {
	if destination == "" || strings.Contains(destination, "\\") {
		return fmt.Errorf("invalid destination %q", destination)
	}
	relative := destination
	if scope == "global" {
		if !strings.HasPrefix(destination, "~/") {
			return fmt.Errorf("global destination %q must start with ~/", destination)
		}
		relative = strings.TrimPrefix(destination, "~/")
	} else if strings.HasPrefix(destination, "~/") || strings.HasPrefix(destination, "/") {
		return fmt.Errorf("project destination %q must be relative", destination)
	}
	if relative == "." || relative != path.Clean(relative) || strings.HasPrefix(relative, "../") {
		return fmt.Errorf("unsafe destination %q", destination)
	}
	return nil
}

func (c Catalog) Items(kind ItemKind) []Item {
	if kind == KindAgents {
		return c.Agents
	}
	if kind == KindSkills {
		return c.Skills
	}
	return nil
}

func (c Catalog) Target(id string) (Target, bool) {
	for _, target := range c.Targets {
		if target.ID == id {
			return target, true
		}
	}
	return Target{}, false
}

func (c Catalog) Item(kind ItemKind, id string) (Item, bool) {
	for _, item := range c.Items(kind) {
		if item.ID == id {
			return item, true
		}
	}
	return Item{}, false
}

func (c Catalog) ReadAsset(item Item) ([]byte, error) {
	data, err := catalogAssets.ReadFile(item.Asset)
	if err != nil {
		return nil, fmt.Errorf("read asset %q: %w", item.Asset, err)
	}
	return data, nil
}
