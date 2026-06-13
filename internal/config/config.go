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
	}
}

// parse reads a flat YAML file line by line without external dependencies.
// Only handles the fields that trackfw uses; ignores unknown keys.
func parse(content string, cfg *ProjectConfig) {
	lines := strings.Split(content, "\n")
	inADRDirs := false
	var adrDirs []string
	var inAgents bool
	var agents []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect list continuation
		if inADRDirs {
			if strings.HasPrefix(trimmed, "- ") {
				adrDirs = append(adrDirs, strings.TrimPrefix(trimmed, "- "))
				continue
			}
			inADRDirs = false
			if len(adrDirs) > 0 {
				cfg.ADRDirs = adrDirs
			}
		}
		if inAgents {
			if strings.HasPrefix(trimmed, "- ") {
				agents = append(agents, strings.TrimPrefix(trimmed, "- "))
				continue
			}
			inAgents = false
			if len(agents) > 0 {
				cfg.Agents = agents
			}
		}

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
		}
	}

	// flush pending lists at EOF
	if inADRDirs && len(adrDirs) > 0 {
		cfg.ADRDirs = adrDirs
	}
	if inAgents && len(agents) > 0 {
		cfg.Agents = agents
	}
}

func splitKV(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
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
