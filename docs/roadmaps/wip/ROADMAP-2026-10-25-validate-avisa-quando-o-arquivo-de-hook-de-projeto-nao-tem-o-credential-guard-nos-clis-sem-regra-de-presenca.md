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
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-25-revisao-aviso-presenca-credential-guard.md`

### ML-2B — Red-team
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-redteam-aviso-presenca-credential-guard.md`

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality, CI e PR
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] Neste repositório, o `validate` do binário da branch não emite o aviso novo (a fiação está completa)
- [ ] CI verde; PR com `Closes #619`
