package integrations

import (
	"fmt"
	"strings"
)

type PlanRequest struct {
	Kind        ItemKind
	Targets     []string
	Items       []string
	Scope       string
	Surfaces    map[string]string
	AllSurfaces bool
}

// BuildPlans resolves catalog selections into deterministic lifecycle plans.
func BuildPlans(catalog *Catalog, request PlanRequest) ([]PlannedArtifact, error) {
	items, err := selectedItems(catalog, request.Kind, request.Items)
	if err != nil {
		return nil, err
	}
	targets, err := selectedTargets(catalog, request.Targets)
	if err != nil {
		return nil, err
	}
	var plans []PlannedArtifact
	for _, target := range targets {
		surfaces, err := selectedSurfaces(target, request.Kind, request.Surfaces[target.ID], request.AllSurfaces)
		if err != nil {
			return nil, err
		}
		for _, surface := range surfaces {
			capability := surface.Capabilities.Agents
			paths := surface.Paths.Agents
			if request.Kind == KindSkills {
				capability = surface.Capabilities.Skills
				paths = surface.Paths.Skills
			}
			if capability.SupportLevel == "unsupported" {
				continue
			}
			installPath, ok := pathForScope(paths, request.Scope)
			if !ok {
				return nil, fmt.Errorf("target %s surface %s does not support %s scope", target.ID, surface.ID, request.Scope)
			}
			for _, item := range items {
				source, err := catalog.ReadAsset(item)
				if err != nil {
					return nil, err
				}
				content, err := Render(item, request.Kind, capability, source)
				if err != nil {
					return nil, err
				}
				claim := Claim{Target: target.ID, Surface: surface.ID, Scope: request.Scope, Kind: request.Kind, Item: item.ID}
				plans = append(plans, PlannedArtifact{
					Claim:       claim,
					Destination: strings.ReplaceAll(installPath.Path, "{{id}}", item.ID),
					Content:     content, CatalogVersion: catalog.Version, SupportLevel: capability.SupportLevel,
					LegacyHashes: LegacyHashes(claim),
				})
			}
		}
	}
	return plans, nil
}

func selectedItems(catalog *Catalog, kind ItemKind, ids []string) ([]Item, error) {
	if len(ids) == 0 {
		return catalog.Items(kind), nil
	}
	result := make([]Item, 0, len(ids))
	for _, id := range ids {
		item, ok := catalog.Item(kind, id)
		if !ok {
			return nil, fmt.Errorf("unknown %s item %q", kind, id)
		}
		result = append(result, item)
	}
	return result, nil
}

func selectedTargets(catalog *Catalog, ids []string) ([]Target, error) {
	if len(ids) == 0 {
		return catalog.Targets, nil
	}
	result := make([]Target, 0, len(ids))
	for _, id := range ids {
		target, ok := catalog.Target(id)
		if !ok {
			return nil, fmt.Errorf("unknown target %q", id)
		}
		result = append(result, target)
	}
	return result, nil
}

func selectedSurfaces(target Target, kind ItemKind, explicit string, all bool) ([]Surface, error) {
	if explicit != "" {
		for _, surface := range target.Surfaces {
			if surface.ID == explicit {
				return []Surface{surface}, nil
			}
		}
		return nil, fmt.Errorf("unknown surface %q for target %s", explicit, target.ID)
	}
	if all {
		return target.Surfaces, nil
	}
	for _, surface := range target.Surfaces {
		capability := surface.Capabilities.Agents
		if kind == KindSkills {
			capability = surface.Capabilities.Skills
		}
		if capability.SupportLevel != "legacy" && capability.SupportLevel != "unsupported" {
			return []Surface{surface}, nil
		}
	}
	return nil, fmt.Errorf("target %s has no supported %s surface", target.ID, kind)
}

func pathForScope(paths []InstallPath, scope string) (InstallPath, bool) {
	for _, candidate := range paths {
		if candidate.Scope == scope {
			return candidate, true
		}
	}
	return InstallPath{}, false
}
