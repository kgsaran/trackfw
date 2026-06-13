package config

import (
	"os"
	"strings"
	"sync"
)

const (
	NamespacingFlat    = "flat"
	NamespacingByAgent = "by_agent"
)

// ProjectConfig holds all configurable paths and governance settings read from trackfw.yaml.
// Absent fields fall back to retrocompatible defaults (v1/v2 projects work unchanged).
type ProjectConfig struct {
	ADRDirs            []string // default: ["docs/adr"]
	REQDir             string   // default: "docs/req"
	RoadmapDir         string   // default: "docs/roadmaps"
	RoadmapNamespacing string   // "flat" (default) or "by_agent"
	Agents             []string // agent names when by_agent mode
	GovernanceMode     string   // "strict" or "lenient"
	LenientUntil       string   // date string YYYY-MM-DD
	WipLimit           int      // default: 1
	WipBySquad         bool
	RequireReqInCommit bool

	// v2.4 fields
	LinkFieldsReq     []string          // default: ["REQ:"]
	LinkFieldsADR     []string          // default: ["ADR:"]
	LinkFieldsRoadmap []string          // default: ["Roadmap:"]
	AcceptanceMarkers []string          // default: ["## Acceptance Criteria", "## Critérios de Aceite"]
	Rules             map[string]string // governance rule severities

	// v2.5 fields
	TraceIdField string // frontmatter field for bidirectional REQ↔Roadmap tracing (default: "" = disabled)
}

var (
	instance ProjectConfig
	once     sync.Once
)

// Load returns the singleton ProjectConfig, reading trackfw.yaml on first call.
// If trackfw.yaml is absent or a field is missing, retrocompatible defaults apply.
func Load() ProjectConfig {
	once.Do(func() {
		instance = defaults()
		data, err := os.ReadFile("trackfw.yaml")
		if err != nil {
			return
		}
		parse(string(data), &instance)
	})
	return instance
}

// Reset clears the singleton — for use in tests only.
func Reset() {
	once = sync.Once{}
	instance = ProjectConfig{}
}

func defaults() ProjectConfig {
	return ProjectConfig{
		ADRDirs:            []string{"docs/adr"},
		REQDir:             "docs/req",
		RoadmapDir:         "docs/roadmaps",
		RoadmapNamespacing: "flat",
		WipLimit:           1,
		LinkFieldsReq:      []string{"REQ:"},
		LinkFieldsADR:      []string{"ADR:"},
		LinkFieldsRoadmap:  []string{"Roadmap:"},
		AcceptanceMarkers:  []string{"## Acceptance Criteria", "## Critérios de Aceite"},
		Rules: map[string]string{
			"wip_has_req":          "error",
			"wip_acceptance":       "error",
			"wip_limit":            "error",
			"stale_wip":            "warning",
			"adr_orphan":           "warning",
			"ref_targets_exist":    "warning",
			"folder_status":        "warning",
			"filename_uniqueness":  "error",
			"blocked_by_draft_adr": "error",
		},
	}
}

// parse reads a YAML file line by line without external dependencies.
// Supports flat keys and one level of nested blocks (link_fields, acceptance_markers, rules).
// Only handles the fields that trackfw uses; ignores unknown keys.
func parse(content string, cfg *ProjectConfig) {
	lines := strings.Split(content, "\n")

	// existing list states
	inADRDirs := false
	var adrDirs []string
	inAgents := false
	var agents []string

	// v2.4 nested block states
	inLinkFields        := false
	inLinkFieldsReq     := false
	inLinkFieldsADR     := false
	inLinkFieldsRoadmap := false
	var linkFieldsReq, linkFieldsADR, linkFieldsRoadmap []string

	inAcceptanceMarkers := false
	var acceptanceMarkers []string

	inRules := false
	rules   := map[string]string{}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		hasIndent := len(rawLine) > 0 && (rawLine[0] == ' ' || rawLine[0] == '\t')

		// Sair de todos os blocos aninhados ao encontrar linha top-level
		if !hasIndent {
			// flush blocos existentes
			if inADRDirs && len(adrDirs) > 0 {
				cfg.ADRDirs = adrDirs
			}
			if inAgents && len(agents) > 0 {
				cfg.Agents = agents
			}
			// flush novos blocos v2.4
			if inLinkFields {
				if inLinkFieldsReq && len(linkFieldsReq) > 0 {
					cfg.LinkFieldsReq = linkFieldsReq
				}
				if inLinkFieldsADR && len(linkFieldsADR) > 0 {
					cfg.LinkFieldsADR = linkFieldsADR
				}
				if inLinkFieldsRoadmap && len(linkFieldsRoadmap) > 0 {
					cfg.LinkFieldsRoadmap = linkFieldsRoadmap
				}
			}
			if inAcceptanceMarkers && len(acceptanceMarkers) > 0 {
				cfg.AcceptanceMarkers = acceptanceMarkers
			}
			if inRules && len(rules) > 0 {
				for k, v := range rules {
					cfg.Rules[k] = v
				}
			}
			// reset de todos os estados
			inADRDirs = false
			adrDirs = nil
			inAgents = false
			agents = nil
			inLinkFields = false
			inLinkFieldsReq = false
			inLinkFieldsADR = false
			inLinkFieldsRoadmap = false
			linkFieldsReq = nil
			linkFieldsADR = nil
			linkFieldsRoadmap = nil
			inAcceptanceMarkers = false
			acceptanceMarkers = nil
			inRules = false
			rules = map[string]string{}
		}

		// Processar linha dentro de bloco aninhado
		if hasIndent {
			if inADRDirs {
				if strings.HasPrefix(trimmed, "- ") {
					adrDirs = append(adrDirs, strings.TrimPrefix(trimmed, "- "))
				}
				continue
			}
			if inAgents {
				if strings.HasPrefix(trimmed, "- ") {
					agents = append(agents, strings.TrimPrefix(trimmed, "- "))
				}
				continue
			}
			if inAcceptanceMarkers {
				if strings.HasPrefix(trimmed, "- ") {
					val := strings.TrimPrefix(trimmed, "- ")
					val = strings.Trim(val, `"'`)
					acceptanceMarkers = append(acceptanceMarkers, val)
				}
				continue
			}
			if inRules {
				k, v, ok := splitKV(trimmed)
				if ok {
					rules[k] = v
				}
				continue
			}
			if inLinkFields {
				if strings.HasPrefix(trimmed, "- ") {
					val := strings.TrimPrefix(trimmed, "- ")
					val = strings.Trim(val, `"'`)
					switch {
					case inLinkFieldsReq:
						linkFieldsReq = append(linkFieldsReq, val)
					case inLinkFieldsADR:
						linkFieldsADR = append(linkFieldsADR, val)
					case inLinkFieldsRoadmap:
						linkFieldsRoadmap = append(linkFieldsRoadmap, val)
					}
				} else {
					// sub-chave dentro de link_fields
					key, _, _ := splitKV(trimmed)
					// flush sub-campo anterior
					if inLinkFieldsReq && len(linkFieldsReq) > 0 {
						cfg.LinkFieldsReq = linkFieldsReq
						linkFieldsReq = nil
					}
					if inLinkFieldsADR && len(linkFieldsADR) > 0 {
						cfg.LinkFieldsADR = linkFieldsADR
						linkFieldsADR = nil
					}
					if inLinkFieldsRoadmap && len(linkFieldsRoadmap) > 0 {
						cfg.LinkFieldsRoadmap = linkFieldsRoadmap
						linkFieldsRoadmap = nil
					}
					inLinkFieldsReq = false
					inLinkFieldsADR = false
					inLinkFieldsRoadmap = false
					switch key {
					case "req":
						inLinkFieldsReq = true
					case "adr":
						inLinkFieldsADR = true
					case "roadmap":
						inLinkFieldsRoadmap = true
					}
				}
				continue
			}
			continue
		}

		// Processar linha top-level (hasIndent == false)
		key, val, ok := splitKV(trimmed)
		if !ok {
			continue
		}

		switch key {
		case "adr_dirs":
			inADRDirs = true
			adrDirs = nil
		case "req_dir":
			cfg.REQDir = val
		case "roadmap_dir":
			cfg.RoadmapDir = val
		case "roadmap_namespacing":
			cfg.RoadmapNamespacing = val
		case "agents":
			inAgents = true
			agents = nil
		case "governance_mode":
			cfg.GovernanceMode = val
		case "lenient_until":
			cfg.LenientUntil = val
		case "wip_limit":
			cfg.WipLimit = parseInt(val, 1)
		case "wip_by_squad":
			cfg.WipBySquad = val == "true"
		case "require_req_in_commit":
			cfg.RequireReqInCommit = val == "true"
		case "link_fields":
			inLinkFields = true
		case "acceptance_markers":
			inAcceptanceMarkers = true
		case "rules":
			inRules = true
			rules = map[string]string{}
		case "trace_id_field":
			cfg.TraceIdField = val
		}
	}

	// flush final (EOF)
	if inADRDirs && len(adrDirs) > 0 {
		cfg.ADRDirs = adrDirs
	}
	if inAgents && len(agents) > 0 {
		cfg.Agents = agents
	}
	if inLinkFields {
		if inLinkFieldsReq && len(linkFieldsReq) > 0 {
			cfg.LinkFieldsReq = linkFieldsReq
		}
		if inLinkFieldsADR && len(linkFieldsADR) > 0 {
			cfg.LinkFieldsADR = linkFieldsADR
		}
		if inLinkFieldsRoadmap && len(linkFieldsRoadmap) > 0 {
			cfg.LinkFieldsRoadmap = linkFieldsRoadmap
		}
	}
	if inAcceptanceMarkers && len(acceptanceMarkers) > 0 {
		cfg.AcceptanceMarkers = acceptanceMarkers
	}
	if inRules && len(rules) > 0 {
		for k, v := range rules {
			cfg.Rules[k] = v
		}
	}
}

func splitKV(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	val = strings.Trim(val, "\"'")
	return key, val, key != ""
}

func parseInt(s string, def int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}
