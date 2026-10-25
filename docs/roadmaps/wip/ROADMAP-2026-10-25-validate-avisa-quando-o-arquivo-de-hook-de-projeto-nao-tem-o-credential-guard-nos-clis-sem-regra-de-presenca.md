---
status: wip
date: 2026-10-25
req: "docs/req/REQ-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: validate avisa quando o arquivo de hook de projeto não tem o credential guard, nos CLIs sem regra de presença

> Created: 2026-10-25 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md
ADR: ADR-2026-10-04, D17. Issue: #619. Decisão do KG: opção (b), aviso numa minor.

As REQs anteriores (#613 e #618) mostraram que a enumeração precisa cobrir `scripts/check-gates-falsify.sh` e os testes que
afirmam o contrato atual, e não só o Go de produção.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com a evidência no ML que o fecha.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e eventos esperados por CLI
> Dependencies: nenhuma. Bloqueia toda implementação.

### ML-0A — Eventos bloqueantes por CLI, sítios e ruído
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-wave0-aviso-presenca-credential-guard.md` (novo; o único arquivo que o ML escreve)
**Acceptance criteria:**
- [x] Por CLI (os 8): os pares (evento, matcher) de credential guard que o gerador emite no projeto, com arquivo:linha do gerador, e quais deles bloqueiam a execução
- [x] Como a regra de violation do Windsurf e do Amazon Q (`validateCredentialGuardPresenceRequired`) escolhe os eventos hoje, e se essa lista pode virar o ponto único dos 8
- [x] Ruído: medir com o binário 11.0.2, em `mktemp -d`, que `init`/`update` de cada CLI, com e sem global, geram todos os pares, e que o aviso não dispararia
- [x] Testes e cenários de `scripts/` que afirmam "ausência de guard nesses 6 CLIs não é problema" e vão mudar
- [x] Ameaças: aviso que some sem a fiação existir, ou aviso que o `update` não resolve
- [x] Recomendação de `Files affected` para a Wave 1

**Auditoria do arquiteto (2026-10-25):** aprovado.
- **Pares por CLI:** citados com a linha do gerador. Os eventos que bloqueiam são `PreToolUse`/`BeforeTool`/
  `preToolUse`/`beforeShellExecution`.
- **Ruído, medido pelo arquiteto com o 11.0.2:** em `mktemp -d`, para cada um dos 6 CLIs, comparei as ocorrências de
  `trackfw guard credential` no projeto depois de `init` (sem global) e depois de `update harness` + `update` (com
  global). Os resultados foram iguais: Claude 6/6, Codex 10/10, Gemini 12/12, Cursor 12/12, Copilot 12/12 e Kiro 12/12.
  A D15 se mantém, e o aviso novo não deve disparar em projeto gerado.
- **R3, fechado pelo arquiteto:** o `init --ai-tools cursor` emite `beforeShellExecution`, `preToolUse Read` e
  `preToolUse Write`, além dos três pares pós.
- **Para o ML-1A:** quebram 10 testes por contrato (§1.5 do parecer), e o Cenário 47 precisa de ajuste.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-25-wave0-aviso-presenca-credential-guard.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Aviso de presença nos 6 CLIs, com lista única de eventos
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/validator/validator_credential_guard.go`, `internal/validator/validator_credential_guard_test.go`, `internal/validator/validator_guard_d16_wiring_external_test.go`, `internal/validator/validator_guard_hook_concordance_external_test.go`, `internal/validator/validator_credential_guard_presence_warning_test.go` (novo), `docs/cli-parity.md`, `scripts/check-gates-falsify.sh`, `internal/validator/validator.go`, `internal/validator/validator_test.go`, `internal/validator/export_test.go` (acrescentados na auditoria: registro da regra, fixture de settings completo, exports de teste)
**Acceptance criteria:**
- [x] AC1–AC5 da REQ, com contra-braço em `origin/main`
**Auditoria do arquiteto (2026-10-25):** aprovado depois do corretivo ML-1A2.
- **Regra:** `credential_guard_presence_warning`, via `applyRuleWarnOnly`. A tabela única `credentialGuardRequiredEntries`
  tem 6 entradas de aviso e 2 de violation.
- **Contra-braço medido pelo arquiteto com binários:** guard só em `PostToolUse` no `.claude/settings.json`. O binário
  de `origin/main` dá rc=0 e 0 avisos de presença. O da branch dá rc=0 e 3 avisos, um por par. O rc inalterado
  confirma o AC2. O "contra-braço" do executor (o símbolo não existe em `origin/main`) foi rejeitado, porque falha de
  compilação é vacuosa.
- **AC5, as duas direções:** o teste `GeneratorTableBidirectional` cobre gerado ⊆ tabela. A direção tabela ⊆ gerado vem
  dos testes de concordância de ruído (`Inject*`, depois zero avisos), que reprovariam se a tabela exigisse um par não
  gerado.
- **AC3 com o guard global:** os testes cobrem Claude e Kiro. Nos outros 4, a cobertura vem da medição do arquiteto na
  Wave 0 (contagem idêntica com e sem global nos 6 CLIs).
- **Evidência ausente:** o executor relatou `make parity-falsify` com "365 OK", mas nenhum log de falsificação foi
  gravado no caminho combinado nem em outro lugar. A falsificação fica medida no `make quality` do ML-3A. O
  `parity-rest` tem log (`pr-619.log`).

## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ✅ Concluído
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-25-revisao-aviso-presenca-credential-guard.md`
**Acceptance criteria:**
- [x] Parecer APROVADO, sem bloqueantes:
  - R1: falta um teste de consistência entre o `schema` da tabela de presença e o `guardWiringHookLayout`;
  - R2: a travessia JSON está duplicada;
  - R3: um nome de teste tem erro de grafia.
  O R1 e o R3 vão para o ML-2C, e o R2 fica como resíduo de refatoração.


### ML-2B — Red-team
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-redteam-aviso-presenca-credential-guard.md`
**Acceptance criteria:**
- [x] Parecer APROVA: 31 ataques, todos PASS. Resíduos:
  - R5-anchor: num projeto ancorado (com `origin/main`), `rules: off` não silencia o aviso;
  - R-echo, R-chmod/BOM, R-JSON-file-field e R2 do Cursor.
  O R5-anchor contradiz o texto do `cli-parity.md` e vai para o ML-2C.

### ML-2C — Corretivo das revisões
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/validator/validator_guard_wiring_test.go`, `internal/validator/validator_guard_d16_wiring_external_test.go`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] R1 do ML-2A: um teste afirma que o `schema` de cada entrada de aviso de `credentialGuardRequiredEntries` é igual ao do `guardWiringHookLayout` para o mesmo arquivo, e reprova numa divergência injetada
- [x] R3 do ML-2A: o nome `..._NaoTemViolacaoDePresencaDisco` corrigido, e o comentário atualizado
- [x] R5-anchor do ML-2B: o `cli-parity.md` descreve quando `rules: credential_guard_presence_warning: off` silencia e quando não silencia (projeto ancorado em `origin/main`), com a evidência do parecer
**Auditoria do arquiteto (2026-10-25):** aprovado.
- **R1:** `TestPresenceWarnOnlyEntries_SchemaMatchesWiringLayout` passa no HEAD. Numa cópia com o esquema trocado, ele
  reprovou (FAIL relatado pelo executor).
- **R3:** nome corrigido. Os dois testes rodam (`=== RUN` conferido).
- **R5-anchor:** o texto do `cli-parity.md` confere com `validator.go:262-296`.
  - **Sem âncora:** vale só o disco, e `off` silencia.
  - **Âncora OK:** vale o mais estrito entre `origin/main` e o disco. O `off` só no disco não silencia; ele precisa estar
    commitado na `main`.
- **`parity-rest`:** rc=0 (`pr-619c.log`).

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3B — Corretivo do CI: teste dependia do trackfw no PATH
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/validator/validator_test.go`, `.claude/agent-memory/zeus-tf/MEMORY.md`, `.claude/agent-memory/zeus-tf/project_path_local_mascara_sonda_de_binario.md`
**Por que existe:** o CI do #623 reprovou em `go`, em `windows-full-suites` e no agregador `parity`, todos pelo
`TestValidate_Clean` ("trackfw binary not found in PATH"). O ML-1A pôs a linha do guard no `writeWriteScopeSettings`, e isso
aciona a sonda de binário. Localmente, o `trackfw` instalado no PATH deixava o teste verde.
**Acceptance criteria:**
- [x] `writeWriteScopeSettings` chama `stubProbeOK(t)`, a costura que já existia, e assim cobre os 4 testes que o usam
- [x] Com o `trackfw` fora do PATH, `internal/validator` e `internal/generators` passam
- [x] As 15 falhas de `internal/commands` sem o `trackfw` no PATH também aparecem em `origin/main`, medido pelo arquiteto
  via `git archive` (15 `--- FAIL`). A causa é outra: os testes leem o `~/.claude/settings.json` real. O CI não é
  afetado, e o caso ficou numa issue própria.

### ML-3A — make quality, CI e PR
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição
**Acceptance criteria:**
- [x] `make quality` rc=0 com a máquina ociosa e o HOME isolado (falsificação 367 OK / 0 FAIL / guarda OK; os mesmos 333 rótulos de antes)
- [x] Neste repositório, o `validate` do binário da branch não emite o aviso novo (a fiação está completa) — rc=0, 0 avisos de presença
- [ ] CI verde; PR com `Closes #619`
