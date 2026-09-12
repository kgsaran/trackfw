package config

import (
	"fmt"
	"os"
	"strings"
)

// AppendAgentToConfig adds agentName to the agents: list in the file at yamlPath
// if the file's roadmap_namespacing is "by_agent" and agentName is not already present.
//
// It is a no-op when:
//   - the file does not exist
//   - roadmap_namespacing is not "by_agent"
//   - agentName is already in the agents: list
//
// It preserves the rest of the file exactly: comments, key order, indentation, quoting.
// Returns an error if the agents: key exists in inline-flow format (agents: [a, b]),
// which cannot be safely edited without changing the representation.
func AppendAgentToConfig(yamlPath, agentName string) error {
	data, err := os.ReadFile(yamlPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Use parse() (same package) to check mode and membership.
	// Do NOT use Load() — it is a process-singleton keyed on CWD and would
	// return stale cached config when the caller's CWD differs from yamlPath.
	cfg := defaults()
	parse(string(data), &cfg)

	if cfg.RoadmapNamespacing != NamespacingByAgent {
		return nil // flat mode — agents: has no function here
	}
	for _, a := range cfg.Agents {
		if a == agentName {
			return nil // already present — idempotent
		}
	}

	return appendAgentTextual(yamlPath, data, agentName)
}

// appendAgentTextual performs the surgical line-level edit to add agentName.
// It does NOT use YAML serialisation so the rest of the file is preserved verbatim.
//
// Supported shapes:
//
//	agents:          ← block list: appends "  - <name>" after the last item;
//	  - alpha          the block stays in its current position (do not move it)
//	  - beta
//
//	<absent>         ← appends "agents:\n  - <name>" at the END of the document;
//	                   deterministic regardless of which other keys are present or absent
//
// Rejected shape (returns error, does NOT rewrite):
//
//	agents: [alpha, beta]   ← inline-flow; caller must edit manually
func appendAgentTextual(yamlPath string, data []byte, agentName string) error {
	lines := strings.Split(string(data), "\n")

	// Reject inline-flow format before doing any work.
	for _, line := range lines {
		bare := strings.TrimSpace(line)
		if strings.HasPrefix(bare, "agents:") {
			rest := strings.TrimSpace(strings.TrimPrefix(bare, "agents:"))
			if strings.HasPrefix(rest, "[") {
				return fmt.Errorf(
					"trackfw.yaml has agents: in inline-flow format; edit %s manually to add %q",
					yamlPath, agentName,
				)
			}
		}
	}

	agentsLineIdx := -1  // index of "agents:" line
	agentsBlockEnd := -1 // index of last "  - ..." item line (or agentsLineIdx when block is empty)

	for i, line := range lines {
		bare := strings.TrimRight(line, " \t\r")
		if bare == "agents:" {
			agentsLineIdx = i
			agentsBlockEnd = i
		}
		if agentsLineIdx >= 0 && i > agentsLineIdx {
			if strings.HasPrefix(bare, "  - ") || strings.HasPrefix(bare, "\t- ") {
				agentsBlockEnd = i
			} else if bare != "" && !strings.HasPrefix(bare, "#") &&
				!strings.HasPrefix(bare, " ") && !strings.HasPrefix(bare, "\t") {
				break // non-empty, non-comment, non-indented = end of block
			}
		}
	}

	newEntry := "  - " + agentName
	var result []string

	if agentsLineIdx >= 0 {
		// agents: block exists — insert inside the block, preserving its position
		for i, line := range lines {
			result = append(result, line)
			if i == agentsBlockEnd {
				result = append(result, newEntry)
			}
		}
	} else {
		// agents: absent — append the block at the end of the document.
		// "End" means after the last non-empty, non-trailing line, so we never
		// insert before a trailing newline that the file already has.
		result = lines
		// Trim trailing empty strings produced by a final "\n"
		for len(result) > 0 && strings.TrimRight(result[len(result)-1], " \t\r") == "" {
			result = result[:len(result)-1]
		}
		result = append(result, "agents:")
		result = append(result, newEntry)
		result = append(result, "") // restore trailing newline
	}

	return os.WriteFile(yamlPath, []byte(strings.Join(result, "\n")), 0o644)
}
