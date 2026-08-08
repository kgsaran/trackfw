---
status: Done
date: 2026-08-08
author: ""
adr: ""
roadmap: ""
---

# REQ: remover geradores legados órfãos (InstallCodex/Copilot/Cursor/Gemini/Windsurf/AmazonQ e installGlobalSkill)

> Date: 2026-08-08 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
Investigação disparada pelo fix de
REQ-2026-08-08-harness-credential-guard-script-nao-e-gerado-por-trackfw-update-harness.md
(auditoria pedida pelo usuário: "temos mais alguma funcionalidade que deveria
ser chamada pelo update harness e não está sendo chamada?") não encontrou
outra instância do mesmo bug, mas identificou uma família de geradores
pré-catálogo, superados pelo sistema `internal/integrations`
(`trackfw agents install`/`trackfw skills install`), com **zero chamadores
fora de teste** em todos os stacks onde existem:

- Go (`internal/generators/`): `InstallCodex`, `InstallCopilot`,
  `InstallCursor`, `InstallGemini`, `InstallWindsurf`, `InstallAmazonQ`
  (arquivos inteiros dedicados, incluindo os `//go:embed templates/<tool>`
  correspondentes) e o wrapper não exportado `installGlobalSkill()`
  (`scaffold.go`) — só invocado por um teste isolado, já superado pelo
  fluxo real (`installSkillsInner` → `installGlobalSkillInner` direto).
- Node.js (`npm/src/generators/codex.js`): `installCodex` — mas os dicts
  `skills`/`agents` do mesmo arquivo **não** são órfãos: são exportados como
  `legacyCodexFixtures` e consumidos por `npm/tests/agents-skills.test.js`
  para provar que o sistema de catálogo novo reconhece bytes idênticos
  produzidos pelo gerador legado (detecção de artefato legado/adoção).
- Python (`pypi/trackfw/generators/codex.py`): `install_codex` + helper
  `_write` — mesma ressalva: `SKILLS`/`AGENTS` são consumidos por
  `pypi/tests/test_agents_skills.py` e devem ser preservados.

Bônus confirmado: `npm/tests/codex.test.js` e
`internal/generators/codex_test.go` (`TestInstallCodexCreatesNativeArtifacts`)
já falhavam antes desta REQ — ambos testavam exclusivamente a função morta e
assumiam relative paths (`scripts/trackfw-credential-guard.sh`) que o
gerador atual não produz mais. Removê-los junto com a função elimina essas
falhas pré-existentes sem mascarar nenhum bug real (a cobertura de
credential-guard já existe em outros testes).

## Acceptance Criteria
- [ ] `go build ./...` e `go vet ./...` passam sem referenciar os símbolos
      removidos
- [ ] `go test ./...` verde, sem a falha pré-existente de
      `TestInstallCodexCreatesNativeArtifacts` (removida junto com o código morto)
- [ ] `npm run test --workspace=npm` verde, sem a falha pré-existente de
      `installCodex creates idempotent native Codex artifacts` (removida
      junto com o código morto)
- [ ] `legacyCodexFixtures`/`SKILLS`/`AGENTS` preservados e os testes que os
      consomem (`agents-skills.test.js`, `test_agents_skills.py`) continuam
      verdes
- [ ] `python3 -m pytest pypi/tests/` verde
- [ ] `trackfw validate` sem violações

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: <!-- nenhum — remoção de código morto sem decisão de design, não requer novo ADR -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-08-remover-geradores-legados-orfaos-installcodex-copilot-cursor-gemini-windsurf-amazonq-e-installglobalskill.md
