package validator

import (
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/config"
)

// buildRoadmapTraceIdIndex retorna o conjunto de trace_id_field VALUES que ALGUM roadmap possui.
// Chama collectTraceIdEntries / collectTraceIdEntriesByAgent — as mesmas funções que
// validateTraceId usa — sem reimplementar a varredura.
// Retorna nil quando TraceIdField está vazio (regra inerte) ou quando ocorre erro de varredura.
//
// ML-1E (ADR D4): suporte ao vínculo reverso de req_has_roadmap via req_id matching.
// 🔴 Não reimplementar: reusar collectTraceIdEntries* é o que garante UMA noção do
// req_id criterion entre req_has_roadmap e traceid_orphan_req.
func buildRoadmapTraceIdIndex(cfg config.ProjectConfig, msgs *[]string) map[string]bool {
	if cfg.TraceIdField == "" {
		return nil
	}
	var entries []traceIdEntry
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		entries, _ = collectTraceIdEntriesByAgent(cfg.RoadmapDir, cfg.TraceIdField, cfg, msgs)
	} else {
		entries, _ = collectTraceIdEntries(cfg.RoadmapDir, cfg.TraceIdField, msgs)
	}
	result := map[string]bool{}
	for _, e := range entries {
		result[e.reqID] = true
	}
	return result
}

// reqHasReverseLink retorna true se algum roadmap aponta para a REQ identificada por
// reqBasename de alguma das duas formas aceitas pela ADR D3/D4 (ML-1E):
//
//  1. req: field do frontmatter do roadmap referencia a REQ por basename
//     (via buildRoadmapReqLinks — normalizeRefSeparator incluso, cobre separador Windows).
//
//  2. cfg.TraceIdField do roadmap coincide com o cfg.TraceIdField da REQ
//     (via buildRoadmapTraceIdIndex — só avaliado quando TraceIdField está configurado).
//
// reqRoadmapLinks e roadmapTraceIdIndex devem ser construídos UMA VEZ fora do loop de REQs.
func reqHasReverseLink(reqBasename, reqContent string, cfg config.ProjectConfig,
	reqRoadmapLinks map[string]bool, roadmapTraceIdIndex map[string]bool) bool {
	// Mecanismo 1: req: do roadmap → basename da REQ
	if reqRoadmapLinks[reqBasename] {
		return true
	}
	// Mecanismo 2: trace_id_field do roadmap coincide com o da REQ
	if roadmapTraceIdIndex == nil {
		return false
	}
	reqTraceID := extractFrontmatterField(reqContent, cfg.TraceIdField)
	if reqTraceID == "" {
		return false
	}
	return roadmapTraceIdIndex[reqTraceID]
}

// reqBasenameFromPath extrai o basename de um caminho de REQ para lookup nos mapas reversos.
// Wrapper explícito para deixar a intenção visível na função chamadora.
func reqBasenameFromPath(path string) string {
	return filepath.Base(path)
}
