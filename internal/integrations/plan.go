package integrations

import (
	"fmt"
	"strings"

	"github.com/kgsaran/trackfw/internal/identity"
)

type PlanRequest struct {
	Kind        ItemKind
	Targets     []string
	Items       []string
	Scope       string
	Surfaces    map[string]string
	AllSurfaces bool
	Identity    identity.Config
	// ProjectRoot, when non-empty, lets BuildPlans apply the D5 third-party
	// reference extension point (ApplyThirdPartyReferences, render.go) to
	// every rendered KindAgents artifact. Optional and source-compatible:
	// every existing caller that does not set it gets byte-identical output
	// to before this field existed (ApplyThirdPartyReferences no-ops on an
	// empty root).
	ProjectRoot string

	// AgentModels maps tier names (e.g. "sonnet", "opus") to version strings
	// (e.g. "4.6", "5") read from trackfw.yaml's agent_models field. A nil or
	// empty map leaves rendered output byte-for-byte identical to before this
	// field existed — by construction (Render no-ops the composition branch
	// when len(agentModels) == 0). See ADR-2026-08-21.
	AgentModels map[string]string
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
				content, err := Render(item, request.Kind, capability, source, request.Identity, target.ID, request.AgentModels)
				if err != nil {
					return nil, err
				}
				// D5 extension point: reproduce any persisted third-party
				// reference block so regenerating this exact artifact (e.g.
				// a later `trackfw agents update`) settles at StateCurrent
				// instead of treating the attachment as drift. See the
				// ThirdPartyReference doc comment in render.go for why this
				// cannot live inside Render itself.
				if request.Kind == KindAgents && request.ProjectRoot != "" {
					content, err = ApplyThirdPartyReferences(request.ProjectRoot, content, target.ID, item.ID)
					if err != nil {
						return nil, err
					}
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

// GlobalGroupPath returns the tilde-abbreviated directory shared by every
// catalog item of (targetID, kind) at global scope. It is derived by
// truncating the catalog's path template immediately before the first path
// segment that contains the "{{id}}" placeholder — e.g.
// "~/.codex/agents/trackfw-{{id}}.toml" truncates to "~/.codex/agents".
//
// This is a roll-up display path used by `trackfw update harness` to report
// one target per (tool, kind) pair without depending on catalog item
// iteration order (see docs/cli-parity.md, "Declared harness targets —
// pinned list"). The surface selection mirrors BuildPlans' default (first
// non-legacy, non-unsupported surface for the requested kind), so the same
// surface that would be installed to is the one whose path is reported.
func GlobalGroupPath(catalog *Catalog, targetID string, kind ItemKind) (string, error) {
	target, ok := catalog.Target(targetID)
	if !ok {
		return "", fmt.Errorf("unknown target %q", targetID)
	}
	surfaces, err := selectedSurfaces(target, kind, "", false)
	if err != nil {
		return "", err
	}
	paths := surfaces[0].Paths.Agents
	if kind == KindSkills {
		paths = surfaces[0].Paths.Skills
	}
	installPath, ok := pathForScope(paths, "global")
	if !ok {
		return "", fmt.Errorf("target %s has no global %s path", targetID, kind)
	}
	return truncateBeforeIDSegment(installPath.Path), nil
}

// truncateBeforeIDSegment drops the "{{id}}"-bearing path segment and
// everything after it, returning the shared parent directory.
func truncateBeforeIDSegment(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if strings.Contains(seg, "{{id}}") {
			return strings.Join(segments[:i], "/")
		}
	}
	return p
}
