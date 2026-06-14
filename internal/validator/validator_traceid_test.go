package validator

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// reqFrontmatter retorna conteúdo de REQ com o campo traceField configurado.
func reqFrontmatter(traceField, traceVal string) string {
	return "---\n" + traceField + ": " + traceVal + "\nstatus: Open\n---\n# REQ\n\nADR: ADR-001.md\nRoadmap: ROADMAP-001.md\n"
}

// roadmapFrontmatter retorna conteúdo de Roadmap com o campo traceField configurado.
func roadmapFrontmatter(traceField, traceVal, status string) string {
	return "---\n" + traceField + ": " + traceVal + "\nstatus: " + status + "\n---\n# Roadmap\n\nREQ: REQ-001.md\n## Acceptance Criteria\n- [ ] done\n"
}

// TestTraceIdOrphanRoadmap: Roadmap com req_id sem REQ correspondente → violation traceid_orphan_roadmap
func TestTraceIdOrphanRoadmap(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	// Criar estrutura de diretórios
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// Roadmap com req_id REQ-999 — sem REQ correspondente
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-orphan.md",
		roadmapFrontmatter("req_id", "REQ-999", "WIP"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_orphan_roadmap") {
		t.Errorf("esperado violation traceid_orphan_roadmap, obteve: %v", vs)
	}
}

// TestTraceIdOrphanReq: REQ com req_id sem Roadmap correspondente → violation traceid_orphan_req
func TestTraceIdOrphanReq(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// REQ com req_id REQ-001 — sem Roadmap correspondente
	writeFile(t, dir, "docs/req/REQ-001-orphan.md",
		reqFrontmatter("req_id", "REQ-001"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_orphan_req") {
		t.Errorf("esperado violation traceid_orphan_req, obteve: %v", vs)
	}
}

// TestTraceIdStateMismatch: REQ em done/, Roadmap em wip/, mesmo req_id → violation traceid_state_mismatch
func TestTraceIdStateMismatch(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req/done")

	// REQ em subpasta done/
	writeFile(t, dir, "docs/req/done/REQ-001-done.md",
		reqFrontmatter("req_id", "REQ-001"))

	// Roadmap em wip/
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-001.md",
		roadmapFrontmatter("req_id", "REQ-001", "WIP"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_state_mismatch") {
		t.Errorf("esperado violation traceid_state_mismatch, obteve: %v", vs)
	}
}

// TestTraceIdDuplicateReq: mesmo req_id em 2 REQs → violation traceid_duplicate_req
func TestTraceIdDuplicateReq(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// Dois REQs com mesmo req_id
	writeFile(t, dir, "docs/req/REQ-001-a.md",
		reqFrontmatter("req_id", "REQ-001"))
	writeFile(t, dir, "docs/req/REQ-001-b.md",
		reqFrontmatter("req_id", "REQ-001"))

	// Roadmap correspondente para evitar orphan_req
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-001.md",
		roadmapFrontmatter("req_id", "REQ-001", "WIP"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_duplicate_req") {
		t.Errorf("esperado violation traceid_duplicate_req, obteve: %v", vs)
	}
}

// TestTraceIdValidPair: REQ + Roadmap com mesmo req_id no mesmo estado → sem violations traceid
func TestTraceIdValidPair(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// Par válido: REQ flat + Roadmap em wip/
	writeFile(t, dir, "docs/req/REQ-001.md",
		reqFrontmatter("req_id", "REQ-001"))
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-001.md",
		roadmapFrontmatter("req_id", "REQ-001", "WIP"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, ws := validateTraceId(cfg)
	for _, v := range vs {
		if contains(v, "traceid_") {
			t.Errorf("não esperado violation traceid_*, obteve: %v", vs)
		}
	}
	for _, w := range ws {
		if contains(w, "traceid_") {
			t.Errorf("não esperado warning traceid_*, obteve: %v", ws)
		}
	}
}

// TestTraceIdDisabled: sem trace_id_field → sem verificação traceid (inalterado)
func TestTraceIdDisabled(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// Roadmap sem par — mas traceid está desativado
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-001.md",
		roadmapFrontmatter("req_id", "REQ-999", "WIP"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "", // desativado
		Rules:        map[string]string{},
	}

	vs, ws := validateTraceId(cfg)
	if len(vs) != 0 || len(ws) != 0 {
		t.Errorf("TraceIdField vazio: esperado 0 violations/warnings, obteve vs=%v ws=%v", vs, ws)
	}
}

// TestTraceIdByAgent: roadmap_namespacing: by_agent — checks disparam corretamente para estrutura agente/estado/
// REQs e Roadmaps seguem a estrutura <dir>/<agente>/<estado>/*.md.
func TestTraceIdByAgent(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	// Criar estrutura by_agent: ambos req e roadmaps com agente/estado/
	mkdirs(t, dir, "req/claude/wip", "roadmaps/claude/wip")

	// REQ com req_id orphan-001 — sem Roadmap correspondente
	writeFile(t, dir, "req/claude/wip/REQ-orphan-001.md",
		reqFrontmatter("req_id", "orphan-001"))

	// Roadmap com req_id orphan-002 — sem REQ correspondente
	writeFile(t, dir, "roadmaps/claude/wip/rm.md",
		roadmapFrontmatter("req_id", "orphan-002", "WIP"))

	cfg := config.ProjectConfig{
		REQDir:             dir + "/req",
		RoadmapDir:         dir + "/roadmaps",
		RoadmapNamespacing: config.NamespacingByAgent,
		TraceIdField:       "req_id",
		Agents:             []string{"claude"},
		Rules:              map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_orphan_req") {
		t.Errorf("esperado violation traceid_orphan_req, obteve: %v", vs)
	}
	if !hasViolation(vs, "traceid_orphan_roadmap") {
		t.Errorf("esperado violation traceid_orphan_roadmap, obteve: %v", vs)
	}
}

// TestTraceIdZeroEntriesSalvaguarda: diretórios vazios → warning de zero entradas indexadas
func TestTraceIdZeroEntriesSalvaguarda(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	// Diretórios existem mas sem arquivos .md
	mkdirs(t, dir, "req", "roadmaps/wip")

	cfg := config.ProjectConfig{
		REQDir:       dir + "/req",
		RoadmapDir:   dir + "/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	_, ws := validateTraceId(cfg)
	if !hasWarning(ws, "trace_id_field is set but no REQ/Roadmap entries were indexed") {
		t.Errorf("esperado warning de zero entradas indexadas, obteve: %v", ws)
	}
}

// contains é um helper local para os testes traceid (evita import strings).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
