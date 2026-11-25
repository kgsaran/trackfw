---
status: wip
date: 2026-11-24
req: "docs/req/REQ-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md"
squad: ""
---

# Roadmap: Teste de orçamento de alias afirma o trackfw.yaml deste repositório e reprova em consumidor

> Created: 2026-11-24 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md -->
REQ: docs/req/REQ-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] O teste do arquivo real afirma só `parse()` sem erro (issue #704); sem "campos não vazios", vácuo por `defaults()`
- [x] Braço: a fixture de consumidor é lida com os valores dela; contra-braço: partindo de `ProjectConfig{}`, sem as
      chaves no yaml, os campos ficam vazios
- [x] Censo de testes que afirmam arquivo versionado deste repositório (Wave 0: sítio único; A2–A7 residual)
- [ ] `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap (hades-tf)
**Status:** ✅ Done — sítio único da mesma causa: `config_merge_key_test.go:501`. Seis outros testes leem arquivo
versionado por `../../` (A2–A7) e afirmam invariante de sincronia que um fork herda intacto; é outra causa, declarada
como residual. Achado crítico: `defaults()` (`config.go:398-403`) já preenche `req_dir`, `roadmap_dir` e `adr_dirs`,
então "campos não vazios" passaria com um `parse()` que não faz nada. O ML-1A foi ajustado.
**Files affected:** `docs/seguranca/2026-11-24-wave0-correcoes-de-consumidor-704-705.md` (novo; compartilhado com o roadmap irmão da mesma branch)
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Wave gates:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
test -s docs/seguranca/2026-11-24-wave0-correcoes-de-consumidor-704-705.md
```

## Wave 1 — Implementation
> Dependencies: Wave 0 audited. Disjoint from the sibling roadmap (#705); one apolo dispatch covers both.

### ML-1A — Teste afirma a propriedade, não o conteúdo do yaml (apolo-tf)
**Status:** ✅ Done — `TestAliasBomb_Budget_RealTrackfwYaml` afirma só o parse; novos `TestAliasBomb_Budget_ConsumerFixture`
(valores da fixture a partir de `ProjectConfig{}`) e `TestAliasBomb_Budget_CounterArm` (sem as chaves, campos vazios).
Censo confirma o Wave 0. Gates do arquiteto: build e `go test ./internal/config/ ./internal/validator/` rc=0.
**Files affected:** `internal/config/config_merge_key_test.go`
**Actions:**
1. Em `TestAliasBomb_Budget_RealTrackfwYaml` (~L500–530), remover as cinco comparações de valor (L514–529). O teste
   afirma só o que o nome diz: `parse()` sem erro sobre o arquivo real (orçamento suficiente). Não acrescente "campos
   não vazios": `defaults()` já os preenche e a asserção seria vácua (Wave 0). Manter os `t.Skip` do arquivo ausente.
2. Novo teste com fixture em `t.TempDir()` (yaml de consumidor: `req_dir: docs/requisições`, `governance_mode: strict`,
   `roadmap_namespacing: by_agent`, `agents: [a, b, c]`): `parse()` sem erro e os campos iguais AOS VALORES DA FIXTURE
   (`cfg.REQDir == "docs/requisições"` etc.). Braço: as asserções antigas (valores deste repo) reprovam nele.
3. Contra-braço: partindo de `ProjectConfig{}` (zero, não `defaults()`), um yaml sem essas chaves deixa os campos
   vazios, o que mostra que a asserção do item 2 só passa porque o `parse()` leu a fixture.
4. Censo (relatório): outros `_test.go` que leem arquivo versionado da raiz do repo (`../../`, `repoRoot`,
   `runtime.Caller`) e afirmam valor deste repositório. Liste; não altere fora desta lista.
**Acceptance criteria:**
- [x] `go test ./internal/config/ -count=1` rc=0; frase de reconciliação por teste novo ou alterado
- [x] nenhum arquivo fora de "Files affected" alterado

## Wave 2 — Review
> Dependencies: Wave 1 audited. Mudança só em teste, sem superfície de ataque nova: a revisão de qualidade substitui o
> red team (decisão do arquiteto, registrada aqui).

### ML-2A — Revisão de qualidade dos dois roadmaps (hefesto-tf)
**Status:** ✅ Done — APROVA COM RESSALVAS. M1 (ACs do roadmap com o critério vácuo) corrigido pelo arquiteto; L3
(ADR-2026-11-18 com o remédio antigo) com nota do arquiteto; L1 e L2 (comentários de teste) no ML-2C.
**Files affected:** `docs/seguranca/2026-11-24-revisao-correcoes-de-consumidor-704-705.md` (novo)
**Acceptance criteria:**
- [x] revisão escrita; nenhum achado bloqueante aberto

### ML-2C — Comentários dos testes de orçamento (apolo-tf)
**Status:** ✅ Done — comentário do braço corrigido (a guarda antiga de `ADRDirs` também reprovaria) e dependência de par
documentada no `CounterArm`. Só comentários.
**Files affected:** `internal/config/config_merge_key_test.go`;
`docs/adr/ADR-2026-11-18-guard-de-credencial-e-de-git-branch-no-google-antigravity-e-gemini-cli-como-legado.md`
(nota do arquiteto, L3 da revisão)
**Acceptance criteria:**
- [x] L1 e L2 da revisão corrigidos; `go test ./internal/config/ -count=1` rc=0

### ML-2B — `make quality` e CI (zeus-tf)
**Status:** ⬜ Pending
**Files affected:** este roadmap
**Acceptance criteria:**
- [ ] `make quality` rc=0; CI verde

**Wave gates:**
```bash
go build ./...
go test ./internal/config/ -count=1
```
