package validator

// validator_adr_req_has_content_test.go — ML-1A (REQ-2026-11-02, #659)
//
// Testes da regra adr_req_has_content: ADR ou REQ cujas seções obrigatórias
// contêm apenas conteúdo do modelo (comentários HTML, bare checkboxes, whitespace)
// emitem aviso. Seções preenchidas com texto substantivo não emitem.
//
// Vetores de teste obrigatórios (P-8 da Wave 0, lista fechada):
//   a — bare checkbox em ## Acceptance Criteria → aviso
//   b — comentário single-line em ## Context → aviso
//   c — comentário multi-linha → aviso
//   d — <!-- não fechado ao final da seção → aviso
//   e — CRLF no comentário → aviso
//   f — parágrafo de texto em qualquer seção mandatória → sem aviso
//   g — ## Context com "CQRS (<!-- sigla -->)" → sem aviso (texto além do comentário)
//   h — ## Linked ADR com <!-- ... --> (seção opcional) → sem aviso
//   i — seção mandatória cujo conteúdo está inteiramente sob ### subseções → sem aviso
//   j — ADR com modelo via ValidateUnfiltered → warning roteado (prova de fiação)
//
// Contra-braço: desligar a detecção de comentário multilinha (alterar
// adrReqHasContentMultilineCommentRe para não usar (?s)) reprova o vetor c.
// Contra-braço: comentar a fiação em ValidateUnfiltered reprova o vetor j.
//
// Regra Dura de Reconciliação (CLAUDE.md): uma frase por teste.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// writeADRFile writes an ADR markdown file with the given content under adrDir.
func writeADRFile(t *testing.T, adrDir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", adrDir, err)
	}
	path := filepath.Join(adrDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// writeREQFile writes a REQ markdown file under reqDir.
func writeREQFile(t *testing.T, reqDir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", reqDir, err)
	}
	path := filepath.Join(reqDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// setupADRReqDirs sets up a minimal project dir with trackfw.yaml pointing at
// adr/ and req/ subdirs and returns (dir, adrDir, reqDir).
func setupADRReqDirs(t *testing.T) (dir, adrDir, reqDir string) {
	t.Helper()
	dir = t.TempDir()
	adrDir = filepath.Join(dir, "adr")
	reqDir = filepath.Join(dir, "req")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("mkdir adr: %v", err)
	}
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("mkdir req: %v", err)
	}
	yamlContent := "adr_dirs:\n  - adr/\nreq_dir: req/\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	return
}

// hasADRReqContentWarning returns true when any warning message contains
// "adr_req_has_content" and the given substring.
func hasADRReqContentWarning(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, "adr_req_has_content") && strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
// Vetor a — bare checkbox em ## Acceptance Criteria → aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que uma REQ com ## Acceptance Criteria contendo apenas
// `- [ ]` (bare checkbox sem texto) dispara a regra adr_req_has_content.
func TestADRReqHasContent_VetorA_BareCheckbox_Aviso(t *testing.T) {
	_, _, reqDir := setupADRReqDirs(t)
	writeREQFile(t, reqDir, "REQ-2026-01-01-test.md", `---
status: Open
---

# REQ: Test

## Motivation

This is the motivation text.

## Acceptance Criteria

- [ ]
- [ ]
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	if !hasADRReqContentWarning(msgs, "Acceptance Criteria") {
		t.Errorf("vetor a: esperado aviso sobre 'Acceptance Criteria', obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Vetor b — comentário single-line em ## Context → aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que um ADR com ## Context contendo apenas um comentário HTML
// single-line dispara a regra adr_req_has_content.
func TestADRReqHasContent_VetorB_ComentarioSingleLine_Aviso(t *testing.T) {
	_, adrDir, _ := setupADRReqDirs(t)
	writeADRFile(t, adrDir, "ADR-001-test.md", `---
status: Accepted
---

# ADR-001

## Context

<!-- What is the situation that motivates this decision? -->

## Decision

We decided.

## Consequences

Good.

## Alternatives Considered

None.
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	if !hasADRReqContentWarning(msgs, "Context") {
		t.Errorf("vetor b: esperado aviso sobre '## Context', obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Vetor c — comentário multi-linha → aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que um ADR com ## Context contendo apenas um comentário
// HTML multi-linha (que span múltiplas linhas) dispara a regra.
// Contra-braço: alterar adrReqHasContentMultilineCommentRe para remover (?s)
// faz o comentário multi-linha não ser removido → vetor f passa mas vetor c reprova.
func TestADRReqHasContent_VetorC_ComentarioMultiLinha_Aviso(t *testing.T) {
	_, adrDir, _ := setupADRReqDirs(t)
	writeADRFile(t, adrDir, "ADR-002-test.md", `---
status: Accepted
---

# ADR-002

## Context

<!--
This is a multi-line
model comment.
-->

## Decision

We decided something.

## Consequences

Good outcomes.

## Alternatives Considered

None evaluated.
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	if !hasADRReqContentWarning(msgs, "Context") {
		t.Errorf("vetor c: esperado aviso sobre '## Context' com comentário multi-linha, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Vetor d — <!-- não fechado ao final da seção → aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que um ADR com ## Context contendo um comentário HTML não
// fechado (<!-- sem -->) dispara a regra — o comentário é tratado como modelo vazio
// mesmo sem fechar (consumindo até o fim da seção).
func TestADRReqHasContent_VetorD_ComentarioNaoFechado_Aviso(t *testing.T) {
	_, adrDir, _ := setupADRReqDirs(t)
	writeADRFile(t, adrDir, "ADR-003-test.md", `---
status: Accepted
---

# ADR-003

## Context

<!-- unclosed comment without closing tag

## Decision

We decided something.

## Consequences

Good.

## Alternatives Considered

None.
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	if !hasADRReqContentWarning(msgs, "Context") {
		t.Errorf("vetor d: esperado aviso sobre '## Context' com comentário não fechado, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Vetor e — CRLF no comentário → aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que um ADR com ## Context contendo um comentário com
// terminações CRLF dispara a regra — a normalização CRLF→LF ocorre antes da detecção.
func TestADRReqHasContent_VetorE_CRLFNoComentario_Aviso(t *testing.T) {
	_, adrDir, _ := setupADRReqDirs(t)
	// Build content with CRLF line endings manually.
	content := "---\r\nstatus: Accepted\r\n---\r\n\r\n# ADR-004\r\n\r\n## Context\r\n\r\n<!-- model\r\n-->\r\n\r\n## Decision\r\n\r\nWe decided.\r\n\r\n## Consequences\r\n\r\nGood.\r\n\r\n## Alternatives Considered\r\n\r\nNone.\r\n"
	path := filepath.Join(adrDir, "ADR-004-test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write ADR file: %v", err)
	}

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	if !hasADRReqContentWarning(msgs, "Context") {
		t.Errorf("vetor e: esperado aviso sobre '## Context' com CRLF, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Vetor f — parágrafo de texto → sem aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que um ADR com texto substantivo em todas as seções
// obrigatórias NÃO dispara a regra adr_req_has_content.
func TestADRReqHasContent_VetorF_TextoSubstantivo_SemAviso(t *testing.T) {
	_, adrDir, _ := setupADRReqDirs(t)
	writeADRFile(t, adrDir, "ADR-005-test.md", `---
status: Accepted
---

# ADR-005

## Context

We need to handle X because of Y constraint.

## Decision

We chose approach Z.

## Consequences

Performance improves; complexity increases.

## Alternatives Considered

Option A was rejected due to cost.
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, "ADR-005") {
			t.Errorf("vetor f: ADR com texto substantivo não deve dispara aviso, obteve: %v", m)
		}
	}
}

// --------------------------------------------------------------------------
// Vetor g — ## Context com "CQRS (<!-- sigla -->)" → sem aviso
// --------------------------------------------------------------------------

// Reconciliação: afirma que ## Context com texto além do comentário HTML interno
// ("CQRS (<!-- sigla -->)") NÃO dispara a regra — texto fora do comentário
// conta como conteúdo substantivo.
func TestADRReqHasContent_VetorG_TextoComComentarioInterno_SemAviso(t *testing.T) {
	_, adrDir, _ := setupADRReqDirs(t)
	writeADRFile(t, adrDir, "ADR-006-test.md", `---
status: Accepted
---

# ADR-006

## Context

We use CQRS (<!-- sigla -->) to separate reads from writes.

## Decision

Adopted CQRS.

## Consequences

Better scalability.

## Alternatives Considered

Event sourcing was evaluated.
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, "ADR-006") {
			t.Errorf("vetor g: ADR com texto+comentário interno não deve disparar aviso, obteve: %v", m)
		}
	}
}

// --------------------------------------------------------------------------
// Vetor h — ## Linked ADR com <!-- ... --> (seção opcional) → sem aviso
// --------------------------------------------------------------------------

// TestADRReqHasContent_VetorI_SubsecoesSomenteSemAviso — vetor i: uma seção mandatória
// cujo conteúdo está inteiramente sob subseções (### ...) com texto real NÃO dispara.
// Reconciliação: afirma que ### headings dentro de uma seção mandatória são tratados
// como corpo da seção (não terminam o scan), então o texto sob eles é visível e
// adrReqHasMeaningfulContent retorna true — sem aviso.
func TestADRReqHasContent_VetorI_SubsecoesSomenteSemAviso(t *testing.T) {
	adrDir, _, _ := setupADRReqDirs(t)
	writeADRFile(t, adrDir, "ADR-2026-01-09-test.md", `---
status: Proposed
---

# ADR: Section with subsections only

## Context

### Background

Texto real de contexto aqui.

### Motivação

Mais texto real.

## Decision

Decided to use approach X.

## Consequences

### Positive

This is a real positive consequence.

### Negative

This is a real negative consequence.

## Alternatives Considered

Option A was rejected because of Y.
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, "ADR-2026-01-09") {
			t.Errorf("vetor i: seção com subseções e texto real não deve disparar aviso, obteve: %v", m)
		}
	}
}

// Reconciliação: afirma que uma seção opcional (## Linked ADR) com apenas comentário
// HTML NÃO dispara a regra — seções opcionais nunca são verificadas.
func TestADRReqHasContent_VetorH_SecaoOpcionalComComentario_SemAviso(t *testing.T) {
	_, _, reqDir := setupADRReqDirs(t)
	writeREQFile(t, reqDir, "REQ-2026-01-02-test.md", `---
status: Open
---

# REQ: Test Optional Section

## Motivation

This is the real motivation text.

## Acceptance Criteria

- [ ] AC1: The system does X.

## Linked ADR

<!-- Reference the ADR that governs this requirement -->

## Negative scope

<!-- Not in scope -->
`)

	msgs, err := validateADRReqHasContent()
	if err != nil {
		t.Fatalf("validateADRReqHasContent() error: %v", err)
	}
	// The optional sections (Linked ADR, Negative scope) must not fire.
	// But "Acceptance Criteria" has "- [ ] AC1: The system does X." which is NOT a bare
	// checkbox (it has text after the checkbox) — so no warning for that either.
	for _, m := range msgs {
		if strings.Contains(m, "REQ-2026-01-02") {
			t.Errorf("vetor h: seções opcionais com comentário não devem disparar aviso, obteve: %v", m)
		}
	}
}

// TestADRReqHasContent_VetorJ_FiacaoValidateUnfiltered — vetor j: prova que o aviso
// adr_req_has_content aparece na saída de ValidateUnfiltered (não só em chamada direta).
// Reconciliação: afirma que a fiação em ValidateUnfiltered/validateUnfilteredTagged existe
// e que applyRule roteia o aviso para o slice warnings do chamador.
func TestADRReqHasContent_VetorJ_FiacaoValidateUnfiltered(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	adrDir := filepath.Join(dir, "docs/adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("mkdir docs/adr: %v", err)
	}
	// ADR with only model content in ## Consequences.
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-2026-01-10-test.md"), []byte(`---
status: Proposed
---

# ADR: Test

## Context

Real context text.

## Decision

Real decision text.

## Consequences

<!-- Fill in consequences -->

## Alternatives Considered

Real alternatives text.
`), 0644); err != nil {
		t.Fatalf("write ADR: %v", err)
	}
	writeFile(t, dir, "trackfw.yaml", "adr_dirs:\n  - docs/adr/\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "adr_req_has_content") && strings.Contains(w, "ADR-2026-01-10") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("vetor j: ValidateUnfiltered deve emitir warning adr_req_has_content para ADR com conteúdo modelo, warnings=%v", warnings)
	}
}
