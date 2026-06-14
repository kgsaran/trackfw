package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// chainNode represents an ADR, REQ, or Roadmap node in the governance chain graph.
type chainNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`  // "adr" | "req" | "roadmap"
	Title string `json:"title"`
	State string `json:"state"`
}

// chainEdge represents a directed link between two nodes.
type chainEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// chainResponse is the JSON shape returned by GET /api/chain.
type chainResponse struct {
	Nodes []chainNode `json:"nodes"`
	Edges []chainEdge `json:"edges"`
}

// chainHandler handles GET /api/chain.
func chainHandler(w http.ResponseWriter, _ *http.Request, cfg config.ProjectConfig) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")

	var nodes []chainNode
	var edges []chainEdge

	// Scan ADRs
	for _, adrDir := range cfg.ADRDirs {
		ns, es := scanChainDir(adrDir, "adr")
		nodes = append(nodes, ns...)
		edges = append(edges, es...)
	}

	// Scan REQs
	{
		ns, es := scanChainDir(cfg.REQDir, "req")
		nodes = append(nodes, ns...)
		edges = append(edges, es...)
	}

	// Scan Roadmaps
	{
		ns, es := scanChainDir(cfg.RoadmapDir, "roadmap")
		nodes = append(nodes, ns...)
		edges = append(edges, es...)
	}

	if nodes == nil {
		nodes = []chainNode{}
	}
	if edges == nil {
		edges = []chainEdge{}
	}

	_ = json.NewEncoder(w).Encode(chainResponse{Nodes: nodes, Edges: edges})
}

// scanChainDir walks a directory tree, reading each .md file and extracting
// frontmatter link fields to build nodes and edges.
func scanChainDir(root, nodeType string) ([]chainNode, []chainEdge) {
	var nodes []chainNode
	var edges []chainEdge

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		// Infer state from parent directory name
		state := inferStateFromPath(path)

		// Extract title
		title := extractTitleFromContent(content, d.Name())

		// Extract frontmatter fields
		fm := parseFrontmatter(content)

		nodes = append(nodes, chainNode{
			ID:    path,
			Type:  nodeType,
			Title: title,
			State: state,
		})

		// Override state from frontmatter if present
		if s, ok := fm["status"]; ok && s != "" {
			nodes[len(nodes)-1].State = s
		}

		// Build edges from link fields: req:, adr:, roadmap:
		for _, field := range []string{"req", "adr", "roadmap"} {
			if val, ok := fm[field]; ok && val != "" {
				// val may be a relative path or just a filename — keep as-is for the frontend
				edges = append(edges, chainEdge{From: path, To: val})
			}
		}

		return nil
	})

	return nodes, edges
}

// parseFrontmatter extracts key-value pairs between leading --- delimiters.
func parseFrontmatter(content string) map[string]string {
	fm := map[string]string{}
	lines := strings.Split(content, "\n")
	inFM := false
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			count++
			if count == 1 {
				inFM = true
				continue
			}
			// closing ---
			break
		}
		if !inFM {
			continue
		}
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		val = strings.Trim(val, `"'`)
		if key != "" {
			fm[strings.ToLower(key)] = val
		}
	}
	return fm
}

// inferStateFromPath guesses the kanban state from the directory component of the path.
func inferStateFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	stateSet := map[string]bool{
		"wip": true, "backlog": true, "blocked": true, "done": true, "abandoned": true,
	}
	// Iterate from deepest directory upward (skip the filename itself)
	for i := len(parts) - 2; i >= 0; i-- {
		if stateSet[parts[i]] {
			return parts[i]
		}
	}
	return "unknown"
}

// extractTitleFromContent returns the first `# ` heading or filename fallback.
func extractTitleFromContent(content, filename string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(filename, ".md")
}
