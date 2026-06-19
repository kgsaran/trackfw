package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// boardItem represents a single roadmap entry on the kanban board.
type boardItem struct {
	File     string `json:"file"`
	Title    string `json:"title"`
	State    string `json:"state"`
	Agent    string `json:"agent"`
	Path     string `json:"path"`
	MLTotal  int    `json:"ml_total"`
	MLDone   int    `json:"ml_done"`
	ActiveML string `json:"active_ml"`
	NextML   string `json:"next_ml"`
}

// boardResponse is the JSON shape returned by GET /api/board.
type boardResponse struct {
	Columns map[string][]boardItem `json:"columns"`
	Agents  []string               `json:"agents"`
}

var boardStates = []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}

// boardHandler handles GET /api/board.
func boardHandler(w http.ResponseWriter, _ *http.Request, cfg config.ProjectConfig) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")

	columns := make(map[string][]boardItem)
	for _, s := range boardStates {
		columns[s] = []boardItem{}
	}
	agentSet := map[string]bool{}

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		// layout: rootDir/agent/state/file.md
		entries, err := os.ReadDir(cfg.RoadmapDir)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, "cannot read roadmap dir", http.StatusInternalServerError)
			return
		}
		for _, agentEntry := range entries {
			if !agentEntry.IsDir() {
				continue
			}
			agent := agentEntry.Name()
			agentDir := filepath.Join(cfg.RoadmapDir, agent)
			for _, state := range boardStates {
				stateDir := filepath.Join(agentDir, state)
				items := readStateDir(stateDir, state, agent, cfg.RoadmapDir)
				if len(items) > 0 {
					columns[state] = append(columns[state], items...)
					agentSet[agent] = true
				}
			}
		}
	} else {
		// flat layout: rootDir/state/file.md
		for _, state := range boardStates {
			stateDir := filepath.Join(cfg.RoadmapDir, state)
			items := readStateDir(stateDir, state, "", cfg.RoadmapDir)
			columns[state] = append(columns[state], items...)
		}
	}

	agents := make([]string, 0, len(agentSet))
	for a := range agentSet {
		agents = append(agents, a)
	}
	sort.Strings(agents)

	resp := boardResponse{
		Columns: columns,
		Agents:  agents,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// readStateDir scans a directory for .md files and returns boardItems.
func readStateDir(dir, state, agent, rootDir string) []boardItem {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var items []boardItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		title := extractTitle(fullPath, e.Name())
		// path relative to working dir — keep the original cfg.RoadmapDir prefix
		relPath := filepath.Join(rootDir, agent)
		if agent != "" {
			relPath = filepath.Join(rootDir, agent, state, e.Name())
		} else {
			relPath = filepath.Join(rootDir, state, e.Name())
		}
		total, done, activeML, nextML := parseMLProgress(fullPath)
		items = append(items, boardItem{
			File:     e.Name(),
			Title:    title,
			State:    state,
			Agent:    agent,
			Path:     relPath,
			MLTotal:  total,
			MLDone:   done,
			ActiveML: activeML,
			NextML:   nextML,
		})
	}
	return items
}

// parseMLProgress scans a roadmap file and returns:
// - total: number of ML-* sections found
// - done: number of MLs with status ✅
// - activeML: "<wave title> · <ml title>" of the first ML with status 🔄, or ""
// - nextML: "<wave title> · <ml title>" of the first ML with status ⬜ (pending), or ""
func parseMLProgress(path string) (total, done int, activeML, nextML string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, "", ""
	}
	var waveCurrent string
	var mlTitle string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && strings.Contains(trimmed, "Wave") {
			waveCurrent = strings.TrimPrefix(trimmed, "## ")
		} else if strings.HasPrefix(trimmed, "### ML-") {
			mlTitle = strings.TrimPrefix(trimmed, "### ")
			total++
		} else if strings.HasPrefix(trimmed, "**Status:**") {
			if strings.Contains(trimmed, "✅") {
				done++
			} else if strings.Contains(trimmed, "🔄") && activeML == "" {
				if waveCurrent != "" {
					activeML = waveCurrent + " · " + mlTitle
				} else {
					activeML = mlTitle
				}
			} else if strings.Contains(trimmed, "⬜") && nextML == "" {
				if waveCurrent != "" {
					nextML = waveCurrent + " · " + mlTitle
				} else {
					nextML = mlTitle
				}
			}
		}
	}
	return total, done, activeML, nextML
}

// extractTitle reads the first `# ` heading from a markdown file,
// falling back to the filename without extension.
func extractTitle(path, filename string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimSuffix(filename, ".md")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(filename, ".md")
}

// setCORSHeaders sets the Access-Control-Allow-Origin header for local dev.
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
