package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/roadmapdoc"
	"github.com/kgsaran/trackfw/internal/validator"
)

// boardItem represents a single roadmap entry on the kanban board.
type boardItem struct {
	File           string `json:"file"`
	Title          string `json:"title"`
	State          string `json:"state"`
	Agent          string `json:"agent"`
	Path           string `json:"path"`
	MLTotal        int    `json:"ml_total"`
	MLDone         int    `json:"ml_done"`
	ActiveML       string `json:"active_ml"`
	NextML         string `json:"next_ml"`
	MalformedWaves int    `json:"malformed_waves,omitempty"`
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
		// layout: rootDir/agent/state/file.md — resolvedor canônico (validator.ResolveAgentNamespaces):
		// união entre agents: e os subdiretórios em disco, sem seguir symlink (REQ-2026-08-29).
		for _, agent := range validator.ResolveAgentNamespaces(cfg, cfg.RoadmapDir) {
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
		// path relative to working dir — keep the original cfg.RoadmapDir prefix.
		//
		// normalizeRefSeparator (mesma função usada pelo node ID de /api/chain, neste
		// pacote): este valor é IDENTIFICADOR emitido em JSON, não caminho de travessia
		// — ADR-2026-09-04 D1, categoria 2. O frontend o devolve verbatim em
		// GET /api/file?path=..., onde o servidor refaz filepath.Clean+filepath.Join;
		// no Windows o Clean reconverte "/" para o separador nativo, então o
		// round-trip é fechado. Evidência de que isso já funciona: o node ID de
		// /api/chain (api_chain.go:111) e o "path" do board Python
		// (serve/api_board.py) já emitem "/" hoje e alimentam o mesmo handler.
		//
		// 🔴 fullPath acima permanece nativo e é o que vai a os.ReadFile — a
		// normalização é de saída, não de travessia (ADR D2).
		relPath := filepath.Join(rootDir, agent)
		if agent != "" {
			relPath = filepath.Join(rootDir, agent, state, e.Name())
		} else {
			relPath = filepath.Join(rootDir, state, e.Name())
		}
		relPath = normalizeRefSeparator(relPath)
		p := parseMLProgressFull(fullPath)
		items = append(items, boardItem{
			File:           e.Name(),
			Title:          title,
			State:          state,
			Agent:          agent,
			Path:           relPath,
			MLTotal:        p.total,
			MLDone:         p.done,
			ActiveML:       p.activeML,
			NextML:         p.nextML,
			MalformedWaves: p.malformedWaves,
		})
	}
	return items
}

// mlProgressResult holds the parsed progress for a single roadmap file.
type mlProgressResult struct {
	total, done, malformedWaves int
	activeML, nextML            string
}

// parseMLProgressFull scans a roadmap file using the roadmapdoc leaf package and returns the
// full progress result including the count of malformed wave headings. It is fence-aware (via
// roadmapdoc.FenceMask) and uses first-token evaluation (roadmapdoc.StatusIsComplete,
// roadmapdoc.StatusCategory) — matching the same dialect the barrier uses (ADR-2026-08-29,
// decision 3).
//
// Malformed wave treatment (board is informative, not a gate): waves with invalid labels are
// counted in MalformedWaves but their MLs are NOT included in Total — roadmapdoc.ParseWaves
// does not return WaveBlocks for malformed headings, so those MLs are silently excluded from
// the count. The MalformedWaves field travels with the JSON response so the board consumer
// can see the count is potentially incomplete.
//
// MLs outside any wave block (measured: 0 in wip, 12 in blocked, 47 in done as of 2026-09-18)
// are also excluded. wip is the only state where count accuracy matters for the live board;
// wip has zero such MLs. Older roadmaps in done/blocked used a non-wave structure; those are
// not corrected here.
//
// activeML/nextML local detection: the first token of the marker is compared directly to "🔄"
// and "⬜". The VS16 variant (🔄️/⬜️, U+FE0F suffix) would not match — roadmapdoc strips VS16
// only for the completion-vocabulary check. Authors using VS16 variants would see empty
// activeML/nextML; the convention in this project uses the plain codepoints.
func parseMLProgressFull(path string) mlProgressResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return mlProgressResult{}
	}
	lines := roadmapdoc.SplitRoadmapLines(string(data))
	fenced := roadmapdoc.FenceMask(lines)
	waves, malformed := roadmapdoc.ParseWaves(lines)

	var res mlProgressResult
	res.malformedWaves = len(malformed)

	for _, wave := range waves {
		waveTitle := strings.TrimPrefix(lines[wave.Start], "## ")
		mls := roadmapdoc.ParseMLs(lines, fenced, wave.Start, wave.End)
		for _, ml := range mls {
			mlTitle := strings.TrimPrefix(lines[ml.Start], "### ")
			res.total++
			marker, found := roadmapdoc.MLStatusMarker(lines, fenced, ml)
			if !found {
				// No status line: treat as pending.
				if res.nextML == "" {
					res.nextML = waveTitle + " · " + mlTitle
				}
				continue
			}
			switch roadmapdoc.StatusCategory(marker) {
			case roadmapdoc.StatusComplete:
				res.done++
			case roadmapdoc.StatusTerminated:
				// Abandoned/cancelled ML — deliberately excluded from active/next.
			default: // StatusPending (covers ⬜, 🔄, ❌ Bloqueado, and anything else)
				fields := strings.Fields(marker)
				if len(fields) == 0 {
					break
				}
				switch fields[0] {
				case "🔄":
					if res.activeML == "" {
						res.activeML = waveTitle + " · " + mlTitle
					}
				case "⬜":
					if res.nextML == "" {
						res.nextML = waveTitle + " · " + mlTitle
					}
				}
				// Other pending markers (❌ Bloqueado, etc.) don't populate activeML/nextML.
			}
		}
	}
	return res
}

// parseMLProgress scans a roadmap file and returns:
// - total: number of ML-* sections found inside valid wave blocks
// - done: number of MLs with status ✅ (first token, fence-aware — ADR-2026-08-29 decision 3)
// - activeML: "<wave title> · <ml title>" of the first ML with status 🔄, or ""
// - nextML: "<wave title> · <ml title>" of the first ML with status ⬜ (pending), or ""
//
// This is a 4-value wrapper around parseMLProgressFull for callers that do not need the
// malformed-wave count. readStateDir calls parseMLProgressFull directly.
func parseMLProgress(path string) (total, done int, activeML, nextML string) {
	p := parseMLProgressFull(path)
	return p.total, p.done, p.activeML, p.nextML
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
