package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
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
		ns, es := scanChainDir(cfg, adrDir, "adr")
		nodes = append(nodes, ns...)
		edges = append(edges, es...)
	}

	// Scan REQs
	{
		ns, es := scanChainDir(cfg, cfg.REQDir, "req")
		nodes = append(nodes, ns...)
		edges = append(edges, es...)
	}

	// Scan Roadmaps
	{
		ns, es := scanChainDir(cfg, cfg.RoadmapDir, "roadmap")
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

// normalizeRefSeparator normaliza um valor já extraído (node ID vindo do WalkDir, ou valor de
// campo de frontmatter) para o separador portável (/). NÃO aplicar ao buffer inteiro de um
// arquivo — só ao valor pontual usado para casar node.ID com edge.To
// (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md, limite duro #2).
func normalizeRefSeparator(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// scanChainDir walks a directory tree, reading each .md file and extracting
// frontmatter link fields to build nodes and edges.
func scanChainDir(cfg config.ProjectConfig, root, nodeType string) ([]chainNode, []chainEdge) {
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

		// nodeID normaliza o separador de path para "/": filepath.WalkDir devolve o caminho
		// no separador nativo do SO que roda `serve`, enquanto o valor de referência lido do
		// frontmatter (edge.To, abaixo) é dado portável e — após o fix de escrita — sempre
		// "/", mas pode ainda ser "\" num artefato herdado do Windows. Sem normalizar os dois
		// lados para a mesma forma, a aresta nunca casa por igualdade de string e o link some
		// silenciosamente do grafo (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md, PoC B).
		nodeID := normalizeRefSeparator(path)

		nodes = append(nodes, chainNode{
			ID:    nodeID,
			Type:  nodeType,
			Title: title,
			State: state,
		})

		// Override state from frontmatter if present
		if s, ok := fm["status"]; ok && s != "" {
			nodes[len(nodes)-1].State = s
		}

		// Build edges from link fields: req:, adr:, roadmap:
		// Only add edges for values that look like real file paths (skip placeholders like "—")
		//
		// ML-3D: o campo é lido com validator.ExtractRefPath (varredura do CONTEÚDO inteiro,
		// linha a linha), não com fm[field] (só o bloco YAML de frontmatter). Achado: o gerador
		// de REQ (internal/generators/req.go) grava `adr: ""` e `roadmap: ""` SEMPRE vazios no
		// frontmatter — o valor real vive no corpo, em "## Linked ADR / ADR: <path>" e
		// "## Linked Roadmap / Roadmap: <path>". Com fm[field], NENHUMA REQ gerada pelo
		// `trackfw req new` jamais produzia aresta — não era caso de borda do vínculo específico
		// que o handoff mediu, era o formato canônico inteiro nunca resolvendo. Ponto único de
		// extração: validator.ExtractRefPath é a mesma função usada por
		// validateRefTargetsExist/validateREQRoadmapLifecycle, para não duplicar a variação
		// linha-a-linha (mesmo princípio do ML-3D para a resolução por basename).
		for _, field := range []string{"req", "adr", "roadmap"} {
			val := validator.ExtractRefPath(content, field)
			if val == "" {
				continue
			}

			// ML-3D: mesma causa do ML-3B (internal/validator/validator.go,
			// resolveRoadmapRefStatus) — um vínculo `roadmap:` grava o caminho COM a pasta de
			// estado, e a pasta É o estado, então todo `trackfw roadmap move` deixa o caminho
			// literal velho. Sem este fallback, o `serve` desenha aresta órfã para o mesmo
			// vínculo que o `validate` já sabe estar resolvido (com aviso de stale state path).
			// Escopo restrito ao campo "roadmap:" — mesma restrição documentada em
			// resolveRoadmapRefByBasename (REQ/ADR não têm dimensão de estado, não precisam e
			// não devem passar por este fallback). Ambíguo (>1) ou não encontrado (0): cai para
			// o append literal abaixo — não inventa nó.
			//
			// A ramificação abaixo é deliberadamente uma sub-cláusula com `continue` própria,
			// preservando o append literal `chainEdge{From: nodeID, To:
			// normalizeRefSeparator(val)}` intocado no caminho comum: é essa substring exata
			// que scripts/check-ref-separator-portability.sh procura como assinatura
			// estrutural de "edge.To sempre passa pelo normalizador" — uma variável
			// intermediária no lugar do literal (`edgeTo := ...; To: edgeTo`) quebraria esse
			// grep estrutural sem quebrar o comportamento (achado do próprio `make quality`
			// rodando este ML).
			if field == "roadmap" {
				if resolved := validator.ResolveRoadmapRef(cfg, normalizeRefSeparator(val)); len(resolved) == 1 {
					edges = append(edges, chainEdge{From: nodeID, To: normalizeRefSeparator(resolved[0])})
					continue
				}
			}

			edges = append(edges, chainEdge{From: nodeID, To: normalizeRefSeparator(val)})
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
