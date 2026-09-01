package validator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestValidateRefTargetsExist_ToleratesDirtyBackslashReference reproduz o PoC A do parecer de
// ameaça (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md): uma REQ cujo
// frontmatter roadmap: foi gravado com separador nativo do Windows ("\") não deve ser reprovada
// por `ref_targets_exist` — o arquivo referenciado existe de verdade, no caminho certo.
func TestValidateRefTargetsExist_ToleratesDirtyBackslashReference(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/roadmaps/wip", "docs/adr")

	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-dirty.md",
		"---\nstatus: wip\n---\n# Roadmap dirty\n## Acceptance Criteria\n- [ ] x\n")

	// Valor que um `roadmap move` rodado no Windows, antes do fix de escrita, teria gravado —
	// separador nativo "\", montado à mão a partir do caminho real esperado nesta máquina
	// (ML-0A: filepath.Join sempre produz "/" aqui, não dá para reproduzir rodando o comando).
	dirtyRef := strings.ReplaceAll("docs/roadmaps/wip/ROADMAP-dirty.md", "/", `\`)
	writeFile(t, dir, "docs/req/REQ-dirty.md",
		"---\nstatus: Open\nroadmap: \""+dirtyRef+"\"\n---\n\n"+
			"# REQ: Dirty\n\n> Date: 2026-09-01 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: `"+dirtyRef+"`\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateRefTargetsExist()
	if err != nil {
		t.Fatalf("validateRefTargetsExist erro: %v", err)
	}
	if hasWarning(warnings, "which does not exist") {
		t.Errorf("referência suja com separador nativo deveria resolver (arquivo existe); warnings=%v", warnings)
	}
}

// TestValidateRefTargetsExist_ControlBrokenReferenceStillFails — controle: uma referência
// genuinamente quebrada (arquivo não existe, com ou sem "\") continua reprovando.
func TestValidateRefTargetsExist_ControlBrokenReferenceStillFails(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/roadmaps/wip", "docs/adr")

	writeFile(t, dir, "docs/req/REQ-broken.md",
		"---\nstatus: Open\nroadmap: \"docs\\\\roadmaps\\\\wip\\\\ROADMAP-nonexistent.md\"\n---\n\n"+
			"# REQ: Broken\n\n> Date: 2026-09-01 | Status: Open\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateRefTargetsExist()
	if err != nil {
		t.Fatalf("validateRefTargetsExist erro: %v", err)
	}
	if !hasWarning(warnings, "which does not exist") {
		t.Errorf("referência para arquivo inexistente deveria continuar reprovando; warnings=%v", warnings)
	}
}

// TestValidateREQRoadmapLifecycle_ToleratesDirtyBackslashReference — manifestação #2 do
// parecer de ameaça: validateREQRoadmapLifecycle falha fechado silenciosamente quando a
// referência está suja (os.Stat erra, o código faz continue, a checagem nunca dispara). Uma
// REQ Open cujo roadmap (referenciado com "\") já está em done/ deve continuar sendo
// sinalizada.
func TestValidateREQRoadmapLifecycle_ToleratesDirtyBackslashReference(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/roadmaps/done", "docs/adr")

	writeFile(t, dir, "docs/roadmaps/done/DONE-ROADMAP-dirty.md",
		"---\nstatus: Done\ndate: 2026-07-01\n---\n# Roadmap concluído\n## Acceptance Criteria\n- [x] done\n")

	dirtyRef := strings.ReplaceAll("docs/roadmaps/done/DONE-ROADMAP-dirty.md", "/", `\`)
	writeFile(t, dir, "docs/req/REQ-dirty-lifecycle.md",
		"---\nstatus: Open\ndate: 2026-07-01\nroadmap: \""+dirtyRef+"\"\n---\n\n"+
			"# REQ: Dirty lifecycle\n\n> Date: 2026-07-01 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: "+dirtyRef+"\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateREQRoadmapLifecycle()
	if err != nil {
		t.Fatalf("validateREQRoadmapLifecycle erro: %v", err)
	}
	if !hasWarning(warnings, "is Open but linked Roadmap") {
		t.Errorf("REQ Open com roadmap sujo em done/ deveria ser sinalizada; warnings=%v", warnings)
	}
}

// TestNormalizeRefSeparator_ControlDoesNotAlterCleanValue — limite duro: valor já portável
// (sem "\") não é alterado.
func TestNormalizeRefSeparator_ControlDoesNotAlterCleanValue(t *testing.T) {
	in := filepath.ToSlash("docs/roadmaps/wip/ROADMAP-x.md")
	if got := normalizeRefSeparator(in); got != in {
		t.Errorf("valor já portável não deveria mudar; queria %q, obteve %q", in, got)
	}
}
