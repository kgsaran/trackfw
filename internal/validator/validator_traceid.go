package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// traceIdEntry representa um artefato indexado por req_id.
type traceIdEntry struct {
	reqID string // valor do campo trace_id_field
	state string // pasta de estado (wip, done, backlog, blocked, abandoned) — vazia se flat
	path  string // caminho completo do arquivo
}

// collectTraceIdEntries varre um diretório raiz e retorna entradas indexadas pelo campo traceField.
// Se o diretório tiver subpastas de estado (wip/, done/, etc.), usa a pasta como estado.
// Se não tiver subpastas, trata como flat com estado vazio.
func collectTraceIdEntries(rootDir, traceField string) ([]traceIdEntry, error) {
	stateDirs := []string{"wip", "done", "backlog", "blocked", "abandoned"}

	var entries []traceIdEntry

	// Verificar se rootDir tem subpastas de estado
	hasStateDirs := false
	for _, state := range stateDirs {
		info, err := os.Stat(filepath.Join(rootDir, state))
		if err == nil && info.IsDir() {
			hasStateDirs = true
			break
		}
	}

	if hasStateDirs {
		// Varrer subpastas de estado
		for _, state := range stateDirs {
			dir := filepath.Join(rootDir, state)
			files, err := filepath.Glob(filepath.Join(dir, "*.md"))
			if err != nil || files == nil {
				continue
			}
			for _, f := range files {
				content, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				val := extractFrontmatterField(string(content), traceField)
				if val == "" {
					continue
				}
				entries = append(entries, traceIdEntry{
					reqID: val,
					state: state,
					path:  f,
				})
			}
		}
	}

	// Sempre varrer também os arquivos na raiz (flat) — REQs podem estar flat
	files, err := filepath.Glob(filepath.Join(rootDir, "*.md"))
	if err == nil {
		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			val := extractFrontmatterField(string(content), traceField)
			if val == "" {
				continue
			}
			entries = append(entries, traceIdEntry{
				reqID: val,
				state: "",
				path:  f,
			})
		}
	}

	return entries, nil
}

// collectTraceIdEntriesByAgent varre um diretório de roadmaps organizado por agente (by_agent)
// e retorna entradas indexadas pelo campo traceField.
// Estrutura esperada: rootDir/<agente>/<estado>/*.md
func collectTraceIdEntriesByAgent(roadmapDir, traceField string, cfg config.ProjectConfig) ([]traceIdEntry, error) {
	stateDirs := []string{"wip", "done", "backlog", "blocked", "abandoned"}
	agents := cfg.Agents
	if len(agents) == 0 {
		entries, err := os.ReadDir(roadmapDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					agents = append(agents, e.Name())
				}
			}
		}
	}
	var all []traceIdEntry
	for _, agent := range agents {
		agentDir := filepath.Join(roadmapDir, agent)
		for _, state := range stateDirs {
			dir := filepath.Join(agentDir, state)
			files, err := filepath.Glob(filepath.Join(dir, "*.md"))
			if err != nil || files == nil {
				continue
			}
			for _, f := range files {
				content, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				val := extractFrontmatterField(string(content), traceField)
				if val == "" {
					continue
				}
				all = append(all, traceIdEntry{reqID: val, state: state, path: f})
			}
		}
	}
	return all, nil
}

// validateTraceId executa as 5 verificações bidirecionais REQ↔Roadmap via trace_id_field.
// Retorna violations e warnings separados por tipo de regra.
func validateTraceId(cfg config.ProjectConfig) (violations []string, warnings []string) {
	traceField := cfg.TraceIdField
	if traceField == "" {
		return nil, nil
	}

	// Indexar REQs
	reqEntries, _ := collectTraceIdEntries(cfg.REQDir, traceField)
	// Indexar Roadmaps — usa by_agent quando configurado
	var roadmapEntries []traceIdEntry
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		roadmapEntries, _ = collectTraceIdEntriesByAgent(cfg.RoadmapDir, traceField, cfg)
	} else {
		roadmapEntries, _ = collectTraceIdEntries(cfg.RoadmapDir, traceField)
	}

	// Montar índices: req_id → []traceIdEntry
	reqIndex := map[string][]traceIdEntry{}
	for _, e := range reqEntries {
		reqIndex[e.reqID] = append(reqIndex[e.reqID], e)
	}
	roadmapIndex := map[string][]traceIdEntry{}
	for _, e := range roadmapEntries {
		roadmapIndex[e.reqID] = append(roadmapIndex[e.reqID], e)
	}

	// Salvaguarda: nenhuma entrada indexada — campo configurado mas diretórios vazios ou mal configurados
	if len(reqEntries) == 0 && len(roadmapEntries) == 0 {
		warnings = append(warnings,
			"trace_id_field is set but no REQ/Roadmap entries were indexed — check req_dir, roadmap_dir and roadmap_namespacing")
		return violations, warnings
	}

	// traceid_duplicate_req: mesmo req_id em mais de 1 REQ
	var dupReqMsgs []string
	for id, list := range reqIndex {
		if len(list) > 1 {
			paths := make([]string, len(list))
			for i, e := range list {
				paths[i] = filepath.Base(e.path)
			}
			dupReqMsgs = append(dupReqMsgs, fmt.Sprintf(
				"traceid_duplicate_req: %q found in %d REQs: %s",
				id, len(list), strings.Join(paths, ", "),
			))
		}
	}
	applyRule("traceid_duplicate_req", dupReqMsgs, &violations, &warnings)

	// traceid_duplicate_roadmap: mesmo req_id em mais de 1 Roadmap
	var dupRoadmapMsgs []string
	for id, list := range roadmapIndex {
		if len(list) > 1 {
			paths := make([]string, len(list))
			for i, e := range list {
				paths[i] = filepath.Base(e.path)
			}
			dupRoadmapMsgs = append(dupRoadmapMsgs, fmt.Sprintf(
				"traceid_duplicate_roadmap: %q found in %d Roadmaps: %s",
				id, len(list), strings.Join(paths, ", "),
			))
		}
	}
	applyRule("traceid_duplicate_roadmap", dupRoadmapMsgs, &violations, &warnings)

	// IDs com duplicata em REQ — não processar orphan/mismatch para evitar ruído duplo
	dupReqIDs := map[string]bool{}
	for id, list := range reqIndex {
		if len(list) > 1 {
			dupReqIDs[id] = true
		}
	}
	dupRoadmapIDs := map[string]bool{}
	for id, list := range roadmapIndex {
		if len(list) > 1 {
			dupRoadmapIDs[id] = true
		}
	}

	// traceid_orphan_roadmap: Roadmap com req_id que não existe em nenhuma REQ
	var orphanRoadmapMsgs []string
	for id, list := range roadmapIndex {
		if dupRoadmapIDs[id] {
			continue // já reportado como duplicata
		}
		if _, exists := reqIndex[id]; !exists {
			for _, e := range list {
				orphanRoadmapMsgs = append(orphanRoadmapMsgs, fmt.Sprintf(
					"traceid_orphan_roadmap: roadmap %q has %s=%q but no REQ with same id",
					filepath.Base(e.path), traceField, id,
				))
			}
		}
	}
	applyRule("traceid_orphan_roadmap", orphanRoadmapMsgs, &violations, &warnings)

	// traceid_orphan_req: REQ com req_id que não existe em nenhum Roadmap
	var orphanReqMsgs []string
	for id, list := range reqIndex {
		if dupReqIDs[id] {
			continue // já reportado como duplicata
		}
		if _, exists := roadmapIndex[id]; !exists {
			for _, e := range list {
				orphanReqMsgs = append(orphanReqMsgs, fmt.Sprintf(
					"traceid_orphan_req: req %q has %s=%q but no Roadmap with same id",
					filepath.Base(e.path), traceField, id,
				))
			}
		}
	}
	applyRule("traceid_orphan_req", orphanReqMsgs, &violations, &warnings)

	// traceid_state_mismatch: REQ e Roadmap com mesmo req_id em estados diferentes
	var mismatchMsgs []string
	for id, reqList := range reqIndex {
		if dupReqIDs[id] || dupRoadmapIDs[id] {
			continue
		}
		roadList, exists := roadmapIndex[id]
		if !exists {
			continue // já tratado como orphan_req
		}
		reqState := reqList[0].state
		roadState := roadList[0].state
		// Só compara se ambos têm estado definido
		if reqState != "" && roadState != "" && reqState != roadState {
			mismatchMsgs = append(mismatchMsgs, fmt.Sprintf(
				"traceid_state_mismatch: %s=%q — REQ is in %q but Roadmap is in %q (%s vs %s)",
				traceField, id, reqState, roadState,
				filepath.Base(reqList[0].path), filepath.Base(roadList[0].path),
			))
		}
	}
	applyRule("traceid_state_mismatch", mismatchMsgs, &violations, &warnings)

	return violations, warnings
}
