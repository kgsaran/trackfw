package validator

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// reqFrontmatter retorna conteúdo de REQ com o campo traceField configurado e status: Open.
func reqFrontmatter(traceField, traceVal string) string {
	return "---\n" + traceField + ": " + traceVal + "\nstatus: Open\n---\n# REQ\n\nADR: ADR-001.md\nRoadmap: ROADMAP-001.md\n"
}

// reqFrontmatterDone retorna conteúdo de REQ com o campo traceField configurado e status: Done.
// Usado nos testes de orphan_req onde o recorte semântico (C1) exige Done para disparar.
func reqFrontmatterDone(traceField, traceVal string) string {
	return "---\n" + traceField + ": " + traceVal + "\nstatus: Done\n---\n# REQ\n\nADR: ADR-001.md\nRoadmap: ROADMAP-001.md\n"
}

// reqFrontmatterStatus retorna conteúdo de REQ com o campo traceField e status arbitrário.
func reqFrontmatterStatus(traceField, traceVal, status string) string {
	return "---\n" + traceField + ": " + traceVal + "\nstatus: " + status + "\n---\n# REQ\n\nADR: ADR-001.md\nRoadmap: ROADMAP-001.md\n"
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

// TestTraceIdOrphanReq: REQ Done com req_id sem Roadmap correspondente → violation traceid_orphan_req
// Reconciliação: afirma que REQ com status: Done sem par de roadmap dispara a regra (AC: contra-braço).
func TestTraceIdOrphanReq(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// REQ Done com req_id REQ-001 — sem Roadmap correspondente (sinal legítimo)
	writeFile(t, dir, "docs/req/REQ-001-orphan.md",
		reqFrontmatterDone("req_id", "REQ-001"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_orphan_req") {
		t.Errorf("esperado violation traceid_orphan_req para REQ Done sem roadmap, obteve: %v", vs)
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

	// REQ Done com req_id orphan-001 — sem Roadmap correspondente (sinal legítimo)
	writeFile(t, dir, "req/claude/wip/REQ-orphan-001.md",
		reqFrontmatterDone("req_id", "orphan-001"))

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

// --- Testes ML-1A: recorte semântico (C1) e casamento por req: (C2) ---

// TestOrphanReqOpenSilencio: REQ Open sem roadmap NÃO dispara traceid_orphan_req (C1).
// Reconciliação: afirma que REQ com status: Open sem roadmap é fluxo normal — ainda não começou —
// e não deve ser sinalizada como órfã (ADR D2).
func TestOrphanReqOpenSilencio(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	writeFile(t, dir, "docs/req/REQ-open.md",
		reqFrontmatter("req_id", "REQ-OPEN-001"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	for _, v := range vs {
		if contains(v, "traceid_orphan_req") {
			t.Errorf("REQ Open sem roadmap NÃO deve disparar traceid_orphan_req, obteve: %v", vs)
		}
	}
}

// TestOrphanReqDoneDisparas: REQ Done sem roadmap DEVE disparar traceid_orphan_req (contra-braço C1).
// Reconciliação: afirma que REQ Done sem roadmap é sinal legítimo de algo perdido — a correção não
// pode silenciar este caso, senão "parou de reprovar" seria indistinguível de "parou de funcionar".
func TestOrphanReqDoneDisparas(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	writeFile(t, dir, "docs/req/REQ-done-orphan.md",
		reqFrontmatterDone("req_id", "REQ-DONE-001"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_orphan_req") {
		t.Errorf("REQ Done sem roadmap DEVE disparar traceid_orphan_req, obteve: %v", vs)
	}
}

// TestOrphanReqSupersededSilencio: REQ Superseded sem roadmap NÃO dispara (ADR D2-bis).
// Reconciliação: afirma que Superseded é desfecho sem execução — ausência de roadmap é esperada.
func TestOrphanReqSupersededSilencio(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	writeFile(t, dir, "docs/req/REQ-superseded.md",
		reqFrontmatterStatus("req_id", "REQ-SUP-001", "Superseded"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	for _, v := range vs {
		if contains(v, "traceid_orphan_req") {
			t.Errorf("REQ Superseded NÃO deve disparar traceid_orphan_req, obteve: %v", vs)
		}
	}
}

// TestOrphanReqClosedSilencio: REQ Closed sem roadmap NÃO dispara (ADR D2-bis).
// Reconciliação: afirma que Closed é desfecho sem execução — ausência de roadmap é esperada.
func TestOrphanReqClosedSilencio(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	writeFile(t, dir, "docs/req/REQ-closed.md",
		reqFrontmatterStatus("req_id", "REQ-CLO-001", "Closed"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	for _, v := range vs {
		if contains(v, "traceid_orphan_req") {
			t.Errorf("REQ Closed NÃO deve disparar traceid_orphan_req, obteve: %v", vs)
		}
	}
}

// TestOrphanReqC2PairByReqField: par REQ↔roadmap com req: no roadmap mas sem req_id NÃO dispara (C2).
// Reconciliação: afirma que roadmap new escreve req: mas não req_id (medido: grep -c '^req_id:' = 0),
// e o casamento via req: é suficiente para o par ser reconhecido como válido.
func TestOrphanReqC2PairByReqField(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// REQ Done com req_id — o par seria invisível sem C2
	writeFile(t, dir, "docs/req/REQ-c2-pair.md",
		reqFrontmatterDone("req_id", "REQ-C2-001"))

	// Roadmap SEM req_id mas COM req: apontando para a REQ (o que roadmap new grava)
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-c2.md",
		"---\nstatus: WIP\nreq: docs/req/REQ-c2-pair.md\n---\n# Roadmap C2\n")

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	for _, v := range vs {
		if contains(v, "traceid_orphan_req") {
			t.Errorf("par REQ+roadmap via req: NÃO deve disparar traceid_orphan_req, obteve: %v", vs)
		}
	}
}

// TestOrphanReqC2WindowsSeparator: req: com separador Windows (\) ainda casa (ADR D3 + normalizeRefSeparator).
// Reconciliação: afirma que req: "docs\\req\\REQ-x.md" (backslash Windows gravado em artefato versionado)
// é tratado igual a "docs/req/REQ-x.md" após normalizeRefSeparator, e o par é reconhecido como válido.
func TestOrphanReqC2WindowsSeparator(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	writeFile(t, dir, "docs/req/REQ-win-sep.md",
		reqFrontmatterDone("req_id", "REQ-WIN-001"))

	// Roadmap com separador Windows no campo req:
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-winsep.md",
		"---\nstatus: WIP\nreq: \"docs\\\\req\\\\REQ-win-sep.md\"\n---\n# Roadmap Windows Sep\n")

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	for _, v := range vs {
		if contains(v, "traceid_orphan_req") {
			t.Errorf("req: com separador Windows deve ser casado via normalizeRefSeparator, obteve: %v", vs)
		}
	}
}

// TestOrphanReqDoneInFlatLayoutDisparas: REQ Done em layout plano (e.state = "") DEVE disparar.
// Reconciliação: afirma que a decisão é pelo frontmatter (reqStatusIsDone), nunca por e.state.
// 🔴 Guard: se alguém substituir reqStatusIsDone por e.state == "done", e.state é "" em layout plano
// e o teste FALHA — capturando a reintrodução de decisão por diretório.
func TestOrphanReqDoneInFlatLayoutDisparas(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	// Layout PLANO: req_dir sem subpastas de estado — e.state será "" para todos os arquivos
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req")

	// REQ Done no diretório raiz (flat) — e.state = ""
	writeFile(t, dir, "docs/req/REQ-flat-done.md",
		reqFrontmatterDone("req_id", "REQ-FLAT-001"))

	cfg := config.ProjectConfig{
		REQDir:       dir + "/docs/req",
		RoadmapDir:   dir + "/docs/roadmaps",
		TraceIdField: "req_id",
		Rules:        map[string]string{},
	}

	vs, _ := validateTraceId(cfg)
	if !hasViolation(vs, "traceid_orphan_req") {
		t.Errorf("REQ Done em layout plano (e.state='') DEVE disparar traceid_orphan_req — "+
			"se este teste falha, a decisão está usando e.state (vazio em flat) em vez de reqStatusIsDone: %v", vs)
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
