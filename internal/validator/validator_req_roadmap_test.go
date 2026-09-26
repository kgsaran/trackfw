package validator

// Tests for ML-1A (AC9): uma noção de "vinculada" — frontmatter como fonte de verdade.
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara em comentário qual conclusão
// do ML-1A ele afirma.

import (
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// buildReqRoadmapDir cria um diretório mínimo com os dirs necessários para o validate.
func buildReqRoadmapDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/req",
		"docs/adr",
	)
	return dir
}

// writeREQWithFields grava um arquivo REQ com frontmatter e/ou marcador de corpo configurável.
func writeREQWithFields(t *testing.T, dir, name, fmRoadmap, bodyRoadmap string) {
	t.Helper()
	var fm string
	if fmRoadmap != "" {
		fm = "---\nstatus: Open\ndate: 2026-09-12\nroadmap: \"" + fmRoadmap + "\"\n---\n"
	} else {
		fm = "---\nstatus: Open\ndate: 2026-09-12\nroadmap: \"\"\n---\n"
	}
	body := "\n# REQ: Fixture\n\n> Date: 2026-09-12 | Status: Open\n\n## Linked Roadmap\n"
	if bodyRoadmap != "" {
		body += "Roadmap: " + bodyRoadmap + "\n"
	} else {
		body += "Roadmap: <!-- none -->\n"
	}
	writeFile(t, dir, filepath.Join("docs/req", name), fm+body)
}

// TestValidateREQsHaveRoadmap_FrontmatterOnly — afirma que frontmatter `roadmap:` preenchido
// com corpo vazio é suficiente para req_has_roadmap passar (ML-1A: frontmatter é fonte de verdade).
func TestValidateREQsHaveRoadmap_FrontmatterOnly(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-fm-only.md",
		"docs/roadmaps/done/ROADMAP-x.md", // fmRoadmap preenchido
		"",                                  // bodyRoadmap vazio
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			t.Errorf("frontmatter `roadmap:` preenchido NÃO deve disparar req_has_roadmap, obteve violation: %q", v)
		}
	}
}

// TestValidateREQsHaveRoadmap_BodyOnly — afirma que corpo `Roadmap:` preenchido com frontmatter
// vazio ainda é aceito como fallback (ML-1A: body é fallback quando frontmatter está ausente).
func TestValidateREQsHaveRoadmap_BodyOnly(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-body-only.md",
		"",                                  // fmRoadmap vazio
		"docs/roadmaps/done/ROADMAP-x.md",  // bodyRoadmap preenchido
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			t.Errorf("corpo `Roadmap:` preenchido (frontmatter vazio) NÃO deve disparar req_has_roadmap, obteve violation: %q", v)
		}
	}
}

// TestValidateREQsHaveRoadmap_BothEqual — afirma que ambos os campos preenchidos e iguais passa
// sem violation nem divergence warning (ML-1A: contra-braço — os dois iguais → passa).
func TestValidateREQsHaveRoadmap_BothEqual(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-both-equal.md",
		"docs/roadmaps/done/ROADMAP-x.md",
		"docs/roadmaps/done/ROADMAP-x.md",
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			t.Errorf("ambos iguais NÃO deve disparar req_has_roadmap, obteve: %q", v)
		}
	}
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			t.Errorf("ambos iguais NÃO deve disparar req_roadmap_sync, obteve: %q", w)
		}
	}
}

// TestValidateREQsHaveRoadmap_Neither — afirma que ausência de ambos os campos produz violation
// (ML-1A: ausência → violation preserva o cheque de órfã).
func TestValidateREQsHaveRoadmap_Neither(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-neither.md", "", "") // ambos vazios
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	found := false
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ausência de ambos os campos deve disparar req_has_roadmap violation, obteve violations=%v", violations)
	}
}

// TestValidateREQRoadmapSync_BasenameDiff — afirma que basename diferente entre frontmatter e
// corpo dispara req_roadmap_sync warning (ML-1A decisão 2: warning para divergência de basename).
func TestValidateREQRoadmapSync_BasenameDiff(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// frontmatter aponta para ROADMAP-a.md, corpo aponta para ROADMAP-b.md — basenames diferentes
	writeREQWithFields(t, dir, "REQ-divergent.md",
		"docs/roadmaps/done/ROADMAP-a.md",
		"docs/roadmaps/done/ROADMAP-b.md",
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	found := false
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("basename diferente entre frontmatter e corpo deve disparar req_roadmap_sync warning, obteve warnings=%v", warnings)
	}
}

// TestValidateREQRoadmapSync_StateDiffOnly — afirma que diferença apenas de pasta de estado
// (wip vs done, mesmo basename) NÃO dispara req_roadmap_sync (ML-1A decisão 2: estado diferente
// é esperado após roadmap move, não é divergência real).
func TestValidateREQRoadmapSync_StateDiffOnly(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// frontmatter aponta para done/ROADMAP-x.md, corpo aponta para wip/ROADMAP-x.md (mesmo basename)
	writeREQWithFields(t, dir, "REQ-state-diff.md",
		"docs/roadmaps/done/ROADMAP-x.md",
		"docs/roadmaps/wip/ROADMAP-x.md",
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			t.Errorf("diferença de estado (wip vs done, mesmo basename) NÃO deve disparar req_roadmap_sync, obteve: %q", w)
		}
	}
}

// ---------------------------------------------------------------------------
// ML-1D — a regra passa a ler o vínculo pelo MESMO extrator que o ML-1A decidiu
// (contentHasStructuredRefValue → extractRefPath), então placeholder deixa de contar.
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara qual conclusão do ML-1D afirma.
// ---------------------------------------------------------------------------

// reqHasRoadmapFired reporta se req_has_roadmap acusou a REQ, olhando violations E warnings —
// a severidade da regra depende do trackfw.yaml do projeto sob teste, e o que este ML mede é o
// VEREDITO da regra, não o canal por onde ela sai.
func reqHasRoadmapFired(t *testing.T) bool {
	t.Helper()
	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, m := range append(append([]string{}, violations...), warnings...) {
		if hasViolation([]string{m}, "no linked Roadmap") {
			return true
		}
	}
	return false
}

// TestValidateREQsHaveRoadmap_PlaceholderNoneFires — afirma a MEDIÇÃO de abertura do ML-1D: uma REQ
// com `roadmap: none` no frontmatter (e `Roadmap: none` no corpo) marcava ZERO violações de
// "no linked Roadmap" antes desta correção, porque extractFrontmatterField aceitava qualquer valor
// não-vazio. Agora dispara.
func TestValidateREQsHaveRoadmap_PlaceholderNoneFires(t *testing.T) {
	for _, placeholder := range []string{"none", "TBD", "-", "nenhum", "<!-- sem roadmap -->"} {
		t.Run(placeholder, func(t *testing.T) {
			dir := buildReqRoadmapDir(t)
			writeREQWithFields(t, dir, "REQ-placeholder.md", placeholder, placeholder)
			config.Reset()
			chdir(t, dir)
			t.Cleanup(config.Reset)

			if !reqHasRoadmapFired(t) {
				t.Errorf("placeholder %q no frontmatter e no corpo DEVE disparar req_has_roadmap", placeholder)
			}
		})
	}
}

// TestValidateREQsHaveRoadmap_ProseValueFires — afirma que a única violação NOVA medida no corpus
// real dos 231 REQs é genuína: REQ-2026-08-16 passava com
// "Roadmap: (a criar quando esta REQ sair do backlog — não iniciar sem REQ + roadmap em `wip`)",
// uma prosa que diz literalmente que o roadmap não existe.
func TestValidateREQsHaveRoadmap_ProseValueFires(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-prose.md", "",
		"(a criar quando esta REQ sair do backlog — não iniciar sem REQ + roadmap em `wip`)")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if !reqHasRoadmapFired(t) {
		t.Error("valor de prosa no marcador de corpo DEVE disparar req_has_roadmap")
	}
}

// TestValidateREQsHaveRoadmap_BacktickBodyStillPasses — contra-braço: afirma que as 2 REQs do corpus
// real cujo vínculo vive no corpo entre backticks (REQ-2026-09-17 e REQ-2026-09-18) continuam NÃO
// acusadas — extractRefPath remove backtick do primeiro token (Cenário 28 do check-gates-falsify).
func TestValidateREQsHaveRoadmap_BacktickBodyStillPasses(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-backtick.md", "", "`docs/roadmaps/done/ROADMAP-x.md`")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if reqHasRoadmapFired(t) {
		t.Error("vínculo de corpo entre backticks NÃO deve disparar req_has_roadmap")
	}
}

// TestValidateREQsHaveRoadmap_CustomLinkFieldHonored — contra-braço de configurabilidade: afirma que
// trocar o extrator NÃO perdeu link_fields.roadmap, porque contentHasStructuredRefValue deriva o
// field de cada marker configurado (aqui `roadmap_ref`, sem o ":" do marcador default).
func TestValidateREQsHaveRoadmap_CustomLinkFieldHonored(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "trackfw.yaml", "link_fields:\n  roadmap:\n    - roadmap_ref\n")
	writeFile(t, dir, filepath.Join("docs/req", "REQ-custom-field.md"),
		"---\nstatus: Open\ndate: 2026-09-26\n---\n\n# REQ: Fixture\n\nroadmap_ref: docs/roadmaps/done/ROADMAP-x.md\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if reqHasRoadmapFired(t) {
		t.Error("marcador customizado (link_fields.roadmap=[roadmap_ref]) com valor .md NÃO deve disparar req_has_roadmap")
	}
}

// TestValidateREQRoadmapSync_PlaceholderFrontmatterIsNotDivergence — afirma a conclusão do ML-1D de que
// o lado frontmatter de req_roadmap_sync passou a usar o mesmo critério de "referência real": uma REQ
// com `roadmap: "none"` e um caminho real no corpo tem UM vínculo e UM placeholder, não dois vínculos
// conflitantes, e não deve disparar o warning de divergência.
func TestValidateREQRoadmapSync_PlaceholderFrontmatterIsNotDivergence(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-fm-placeholder.md", "none", "docs/roadmaps/done/ROADMAP-x.md")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			t.Errorf("frontmatter com placeholder NÃO deve disparar req_roadmap_sync, obteve: %q", w)
		}
	}
}
