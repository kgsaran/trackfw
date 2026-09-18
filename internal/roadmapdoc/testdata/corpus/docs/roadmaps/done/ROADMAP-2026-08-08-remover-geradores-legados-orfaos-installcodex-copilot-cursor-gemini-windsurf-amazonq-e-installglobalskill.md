---
status: done
date: 2026-08-08
req: ""
squad: ""
---

# Roadmap: remover geradores legados órfãos (InstallCodex/Copilot/Cursor/Gemini/Windsurf/AmazonQ e installGlobalSkill)

> Created: 2026-08-08 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-08-08-remover-geradores-legados-orfaos-installcodex-copilot-cursor-gemini-windsurf-amazonq-e-installglobalskill.md

Investigação de auditoria (pedida pelo usuário após o fix do bug de
credential-guard) confirmou, com grep exaustivo cruzando os 3 stacks, que
`InstallCodex`/`InstallCopilot`/`InstallCursor`/`InstallGemini`/
`InstallWindsurf`/`InstallAmazonQ` (Go) e seus resquícios em Node.js/Python
não têm nenhum chamador em código de produção — foram superados pelo
sistema de catálogo (`internal/integrations`). Cada stack tem seu próprio
ML porque os arquivos afetados são disjuntos entre si (nenhum ML depende
do resultado de outro) — paralelizável.

**Atenção — não remover em bloco:** os dicts `skills`/`agents` (Node) e
`SKILLS`/`AGENTS` (Python) do módulo `codex` NÃO são órfãos — são fixtures
de teste (`legacyCodexFixtures`) usadas por `agents-skills.test.js`/
`test_agents_skills.py` para provar reconhecimento de conteúdo legado pelo
sistema de catálogo novo. Só as FUNÇÕES instaladoras (`installCodex`/
`install_codex` + helper `_write`) são código morto.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Nenhum símbolo removido tem chamador restante em código de produção
      ou teste (confirmado por grep antes de cada remoção)
- [ ] `legacyCodexFixtures`/`SKILLS`/`AGENTS` preservados intactos
- [ ] Build + testes verdes nos 3 stacks (`go build`, `go test`,
      `node --test`, `pytest`)
- [ ] `trackfw validate` sem violações

## Wave 1 — Remoção por stack (3 MLs paralelos, arquivos disjuntos)
> Dependencies: none

### ML-1A — Go: remover InstallCodex/Copilot/Cursor/Gemini/Windsurf/AmazonQ e installGlobalSkill
**Status:** ✅ Concluído
**Files affected:**
- Deletar: `internal/generators/codex.go`, `internal/generators/codex_test.go`
- Deletar: `internal/generators/copilot.go`, `internal/generators/copilot_test.go`, `internal/generators/templates/copilot/`
- Deletar: `internal/generators/cursor.go`, `internal/generators/cursor_test.go`, `internal/generators/templates/cursor/`
- Deletar: `internal/generators/gemini.go`, `internal/generators/gemini_test.go`, `internal/generators/templates/gemini/`
- Deletar: `internal/generators/windsurf.go`, `internal/generators/windsurf_test.go`, `internal/generators/templates/windsurf/`
- Deletar: `internal/generators/amazonq.go`, `internal/generators/amazonq_test.go`, `internal/generators/templates/amazonq/`
- Editar: `internal/generators/scaffold.go` — remover a função não exportada
  `installGlobalSkill()` (wrapper de 3 linhas em torno de
  `installGlobalSkillInner(false)`, hoje só invocado pelo teste abaixo; o
  fluxo real usa `installSkillsInner` → `installGlobalSkillInner` direto,
  que **não** deve ser tocado)
- Editar: `internal/generators/claudemd_test.go` — remover só a função
  `TestInstallGlobalSkill_GlobalADRsDirective` (as demais funções do
  arquivo, `TestGenerateClaudeMD_*`/`TestTrackfwRulesBlock_*`, testam
  `generateClaudeMD`/`trackfwRulesBlock`, que continuam vivas — não tocar)
**Actions:**
- Antes de cada remoção, confirmar com
  `grep -rn "<Simbolo>(" internal/ --include="*.go" | grep -v "_test.go"`
  que não sobrou chamador de produção.
- Não remover `installGlobalSkillInner`, `installSkillsInner`,
  `InstallSkills`, `ForceInstallSkills`, `GlobalClaudeSkillPath`,
  `GlobalClaudeSkillContent` — são o caminho vivo, permanecem intocados.
- Não remover `helperReadJSON`/`helperHasClaudeHook`
  (`internal/generators/agentfiles_test.go`) — usados por outros testes
  (`agentfiles_test.go`, `credential_guard_dedup_test.go`,
  `credential_guard_sabotage_test.go`, `hooks_test.go`).
**Acceptance criteria:**
- [ ] `go build ./...` sem erros
- [ ] `go vet ./...` sem erros
- [ ] `go test ./...` verde (a falha pré-existente
      `TestInstallCodexCreatesNativeArtifacts` desaparece por remoção do teste,
      não por skip/xfail)
**Comandos de validação:** `go build ./... && go vet ./... && go test ./...`

### ML-1B — Node.js: remover installCodex preservando legacyCodexFixtures
**Status:** ✅ Concluído
**Files affected:**
- Editar: `npm/src/generators/codex.js` — remover a função `installCodex`
  (linhas ~84-101) e os `require`s que ficam órfãos após a remoção (`fs`,
  `path`, `injectCodexHooks` de `./hooks`, `injectRulesForTool` de
  `./init`); manter intactos os dicts `skills`/`agents` e atualizar
  `module.exports` para `{ legacyCodexFixtures: { skills, agents } }` (sem
  `installCodex`)
- Deletar: `npm/tests/codex.test.js` (testa exclusivamente `installCodex`;
  já é uma falha pré-existente — `installCodex creates idempotent native
  Codex artifacts` — que desaparece por remoção do teste morto, não por
  skip)
**Actions:**
- Confirmar com `grep -rn "legacyCodexFixtures" npm/` que
  `npm/tests/agents-skills.test.js` continua importando e usando os dicts
  normalmente após a edição.
- Confirmar com `grep -rn "installCodex(" npm/src npm/tests` que não sobra
  nenhuma referência.
**Acceptance criteria:**
- [ ] `node --test npm/tests/*.test.js` verde, incluindo
      `agents-skills.test.js` (fixtures preservadas)
- [ ] nenhum `require` órfão deixado em `codex.js`
**Comandos de validação:** `node --test npm/tests/*.test.js`

### ML-1C — Python: remover install_codex preservando SKILLS/AGENTS
**Status:** ✅ Concluído
**Files affected:**
- Editar: `pypi/trackfw/generators/codex.py` — remover as funções
  `install_codex` e o helper `_write` (usado só por `install_codex`), e os
  imports que ficam órfãos (`os`, `re`, `inject_codex_hooks` de
  `trackfw.generators.hooks`, `inject_rules_for_tool` de
  `trackfw.generators.init_gen`); manter intactos os dicts `SKILLS`/`AGENTS`
- Deletar: `pypi/tests/test_codex.py` (testa exclusivamente `install_codex`)
**Actions:**
- Confirmar com
  `grep -rn "from trackfw.generators.codex import" pypi/ --include="*.py"`
  que `pypi/tests/test_agents_skills.py` (`AGENTS as LEGACY_PYTHON_AGENTS`)
  continua funcionando após a edição.
- Confirmar com `grep -rn "install_codex(" pypi/trackfw pypi/tests` que não
  sobra nenhuma referência.
**Acceptance criteria:**
- [ ] `python3 -m pytest pypi/tests/` verde, incluindo `test_agents_skills.py`
- [ ] nenhum import órfão deixado em `codex.py`
**Comandos de validação:** `python3 -m pytest pypi/tests/`

## Wave 2 — Auditoria final (1 ML, orquestrador)
> Dependencies: Wave 1 completa

### ML-2A — Confirmar paridade e ausência de regressões cross-stack
**Status:** ✅ Concluído
**Files affected:** nenhum (auditoria, sem edição)
**Actions:**
- Rodar build+test completo dos 3 stacks e `trackfw validate`.
- Confirmar que nenhum símbolo removido em um stack tinha equivalente vivo
  em outro (paridade dos 3 CLIs preservada — a remoção é simétrica: código
  morto em todos os stacks onde existia).
**Acceptance criteria:**
- [x] `go build ./... && go vet ./... && go test ./...` verde (14 pacotes ok,
      `TestInstallCodexCreatesNativeArtifacts` removido junto com o código morto)
- [x] `node --test npm/tests/*.test.js` verde (432 pass, 0 fail — o teste
      pré-existente de `installCodex` foi removido junto com o código morto)
- [x] `python3 -m pytest pypi/tests/` verde (971 passed)
- [x] `trackfw validate` sem violações (após atualizar REQ/roadmap para Done)
**Comandos de validação:** `go build ./... && go vet ./... && go test ./... && node --test npm/tests/*.test.js && python3 -m pytest pypi/tests/ && trackfw validate`
