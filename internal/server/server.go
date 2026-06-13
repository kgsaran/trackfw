package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// ADRInfo holds parsed metadata from an ADR markdown file.
type ADRInfo struct {
	Path   string `json:"path"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// REQInfo holds parsed metadata from a REQ markdown file.
type REQInfo struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	LinkedADR string `json:"linkedAdr"`
}

// RoadmapInfo holds parsed metadata from a roadmap markdown file.
type RoadmapInfo struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	State string `json:"state"`
	Squad string `json:"squad"`
}

// pageData aggregates all parsed data for the HTML template.
type pageData struct {
	ADRs       []ADRInfo
	REQs       []REQInfo
	Roadmaps   []RoadmapInfo
	ByState    map[string][]RoadmapInfo
	StateOrder []string
}

var (
	reTitle     = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reStatus    = regexp.MustCompile(`(?m)\|\s*Status:\s*([^|\n]+)`)
	reLinkedADR = regexp.MustCompile(`(?m)\|\s*ADR:\s*([^|\n]+)`)
	reSquad     = regexp.MustCompile(`(?m)^squad:\s*(.+)$`)
)

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func firstMatch(re *regexp.Regexp, content string) string {
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// parseADRs globs each configured adr_dir for *.md files.
func parseADRs(rootDir string, adrDirs []string) []ADRInfo {
	var allFiles []string
	for _, d := range adrDirs {
		pattern := filepath.Join(rootDir, d, "*.md")
		files, _ := filepath.Glob(pattern)
		allFiles = append(allFiles, files...)
	}
	sort.Strings(allFiles)
	result := make([]ADRInfo, 0, len(allFiles))
	for _, f := range allFiles {
		content := readFile(f)
		title := firstMatch(reTitle, content)
		if title == "" {
			title = filepath.Base(f)
		}
		status := firstMatch(reStatus, content)
		if status == "" {
			status = "Unknown"
		}
		result = append(result, ADRInfo{
			Path:   f,
			Title:  title,
			Status: status,
		})
	}
	return result
}

// parseREQs globs the configured req_dir for *.md files (flat and one level deep).
func parseREQs(rootDir, reqDir string) []REQInfo {
	base := filepath.Join(rootDir, reqDir)
	var allFiles []string
	for _, pattern := range []string{
		filepath.Join(base, "*.md"),
		filepath.Join(base, "*", "*.md"),
	} {
		files, _ := filepath.Glob(pattern)
		allFiles = append(allFiles, files...)
	}
	sort.Strings(allFiles)
	result := make([]REQInfo, 0, len(allFiles))
	for _, f := range allFiles {
		content := readFile(f)
		title := firstMatch(reTitle, content)
		if title == "" {
			title = filepath.Base(f)
		}
		status := firstMatch(reStatus, content)
		if status == "" {
			status = "Unknown"
		}
		linkedADR := firstMatch(reLinkedADR, content)
		result = append(result, REQInfo{
			Path:      f,
			Title:     title,
			Status:    status,
			LinkedADR: strings.TrimSpace(linkedADR),
		})
	}
	return result
}

// parseRoadmaps globs the configured roadmap_dir using the correct depth for flat vs by_agent.
func parseRoadmaps(rootDir, roadmapDir, namespacing string) []RoadmapInfo {
	base := filepath.Join(rootDir, roadmapDir)
	var patterns []string
	if namespacing == "by_agent" {
		// docs/roadmaps/<agent>/<state>/*.md
		patterns = []string{filepath.Join(base, "*", "*", "*.md")}
	} else {
		// docs/roadmaps/<state>/*.md
		patterns = []string{filepath.Join(base, "*", "*.md")}
	}
	var allFiles []string
	for _, p := range patterns {
		files, _ := filepath.Glob(p)
		allFiles = append(allFiles, files...)
	}
	sort.Strings(allFiles)
	result := make([]RoadmapInfo, 0, len(allFiles))
	for _, f := range allFiles {
		if strings.Contains(f, ".trackfw-log") {
			continue
		}
		content := readFile(f)
		title := firstMatch(reTitle, content)
		if title == "" {
			title = filepath.Base(f)
		}
		state := filepath.Base(filepath.Dir(f))
		squad := firstMatch(reSquad, content)
		result = append(result, RoadmapInfo{
			Path:  f,
			Title: title,
			State: state,
			Squad: squad,
		})
	}
	return result
}

func statusClass(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "proposed":
		return "proposed"
	case "draft":
		return "draft"
	case "accepted", "approved":
		return "accepted"
	case "deprecated", "rejected":
		return "deprecated"
	case "open":
		return "open"
	default:
		return "unknown"
	}
}

func upper(s string) string {
	return strings.ToUpper(s)
}

func base(s string) string {
	return filepath.Base(s)
}

var funcMap = template.FuncMap{
	"statusClass": statusClass,
	"upper":       upper,
	"base":        base,
}

const htmlTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>trackfw — Governance Dashboard</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  background: #1a1a2e;
  color: #e0e0e0;
  font-family: 'Courier New', Courier, monospace;
  font-size: 14px;
  line-height: 1.6;
}
.container { max-width: 1200px; margin: 0 auto; padding: 16px; }
h1 { color: #a0c4ff; font-size: 1.6rem; margin-bottom: 8px; }
h2 { color: #7ec8e3; font-size: 1.1rem; margin: 24px 0 10px; border-bottom: 1px solid #333; padding-bottom: 4px; }
h3 { color: #b5ead7; font-size: 0.95rem; margin: 0 0 8px; }
p.subtitle { color: #888; margin-bottom: 24px; font-size: 0.85rem; }
table { width: 100%; border-collapse: collapse; margin-bottom: 16px; }
th { background: #16213e; color: #a0c4ff; text-align: left; padding: 8px 10px; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; }
td { padding: 7px 10px; border-bottom: 1px solid #222; vertical-align: top; }
tr:hover td { background: #1e2a45; }
.status { display: inline-block; padding: 2px 8px; border-radius: 3px; font-size: 0.75rem; font-weight: bold; }
.s-proposed, .s-draft { background: #3a3a00; color: #ffd700; }
.s-accepted, .s-approved { background: #003a00; color: #7fff7f; }
.s-deprecated, .s-rejected { background: #3a0000; color: #ff7f7f; }
.s-open { background: #003a3a; color: #7fffff; }
.s-unknown { background: #2a2a2a; color: #999; }
.kanban { display: flex; gap: 12px; flex-wrap: wrap; }
.kanban-col { flex: 1; min-width: 200px; background: #16213e; border-radius: 6px; padding: 12px; }
.kanban-item { background: #1a1a2e; border: 1px solid #2a3a5a; border-radius: 4px; padding: 8px; margin-bottom: 8px; font-size: 0.82rem; }
.kanban-item .item-title { color: #e0e0e0; margin-bottom: 4px; }
.kanban-item .item-path { color: #666; font-size: 0.72rem; }
.kanban-item .item-squad { color: #a0c4ff; font-size: 0.72rem; }
.state-wip { border-left: 3px solid #ffd700; }
.state-done { border-left: 3px solid #7fff7f; }
.state-blocked { border-left: 3px solid #ff7f7f; }
.state-backlog { border-left: 3px solid #888; }
.state-abandoned { border-left: 3px solid #555; }
.empty-msg { color: #555; font-style: italic; font-size: 0.85rem; }
nav { margin-bottom: 20px; }
nav a { margin-right: 16px; color: #7ec8e3; text-decoration: none; }
nav a:hover { text-decoration: underline; }
</style>
</head>
<body>
<div class="container">
  <h1>trackfw — Governance Dashboard</h1>
  <p class="subtitle">Traceability chain: ADR &#8594; REQ &#8594; ROADMAP</p>
  <nav>
    <a href="#traceability">Traceability Graph</a>
    <a href="#timeline">ADR Timeline</a>
    <a href="#kanban">Roadmap Kanban</a>
    <a href="/api/data">JSON API</a>
  </nav>

  <!-- TRACEABILITY GRAPH -->
  <h2 id="traceability">Traceability Graph</h2>
  {{if .REQs}}
  <table>
    <thead>
      <tr>
        <th>REQ</th>
        <th>Status</th>
        <th>Linked ADR</th>
      </tr>
    </thead>
    <tbody>
      {{range .REQs}}
      <tr>
        <td title="{{.Path}}">{{.Title}}</td>
        <td><span class="status s-{{statusClass .Status}}">{{.Status}}</span></td>
        <td>{{if .LinkedADR}}{{.LinkedADR}}{{else}}<span class="empty-msg">&#8212;</span>{{end}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="empty-msg">No REQs found</p>
  {{end}}

  <!-- ADR TIMELINE -->
  <h2 id="timeline">ADR Timeline</h2>
  {{if .ADRs}}
  <table>
    <thead>
      <tr>
        <th>File</th>
        <th>Title</th>
        <th>Status</th>
      </tr>
    </thead>
    <tbody>
      {{range .ADRs}}
      <tr>
        <td style="color:#666;font-size:0.8rem;">{{base .Path}}</td>
        <td>{{.Title}}</td>
        <td><span class="status s-{{statusClass .Status}}">{{.Status}}</span></td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="empty-msg">No ADRs found</p>
  {{end}}

  <!-- KANBAN -->
  <h2 id="kanban">Roadmap Kanban</h2>
  {{if .Roadmaps}}
  <div class="kanban">
    {{range .StateOrder}}
    {{$state := .}}
    {{$items := index $.ByState $state}}
    {{if $items}}
    <div class="kanban-col">
      <h3>{{upper $state}}</h3>
      {{range $items}}
      <div class="kanban-item state-{{.State}}">
        <div class="item-title">{{.Title}}</div>
        <div class="item-path">{{base .Path}}</div>
        {{if .Squad}}<div class="item-squad">Squad: {{.Squad}}</div>{{end}}
      </div>
      {{end}}
    </div>
    {{end}}
    {{end}}
  </div>
  {{else}}
  <p class="empty-msg">No roadmaps found</p>
  {{end}}
</div>
</body>
</html>`

func buildPageData(dir string) pageData {
	cfg := config.Load()
	adrs := parseADRs(dir, cfg.ADRDirs)
	reqs := parseREQs(dir, cfg.REQDir)
	roadmaps := parseRoadmaps(dir, cfg.RoadmapDir, cfg.RoadmapNamespacing)

	byState := make(map[string][]RoadmapInfo)
	for _, r := range roadmaps {
		byState[r.State] = append(byState[r.State], r)
	}

	stateOrder := []string{"wip", "backlog", "blocked", "done", "abandoned"}

	return pageData{
		ADRs:       adrs,
		REQs:       reqs,
		Roadmaps:   roadmaps,
		ByState:    byState,
		StateOrder: stateOrder,
	}
}

func newTemplate() *template.Template {
	return template.Must(template.New("page").Funcs(funcMap).Parse(htmlTmpl))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	dir, _ := os.Getwd()
	data := buildPageData(dir)
	tmpl := newTemplate()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}

func handleAPIData(w http.ResponseWriter, r *http.Request) {
	dir, _ := os.Getwd()
	type apiResponse struct {
		ADRs     []ADRInfo     `json:"adrs"`
		REQs     []REQInfo     `json:"reqs"`
		Roadmaps []RoadmapInfo `json:"roadmaps"`
	}
	cfg := config.Load()
	resp := apiResponse{
		ADRs:     parseADRs(dir, cfg.ADRDirs),
		REQs:     parseREQs(dir, cfg.REQDir),
		Roadmaps: parseRoadmaps(dir, cfg.RoadmapDir, cfg.RoadmapNamespacing),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Start registers handlers and starts the HTTP server on the given port.
func Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/data", handleAPIData)

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, mux)
}
