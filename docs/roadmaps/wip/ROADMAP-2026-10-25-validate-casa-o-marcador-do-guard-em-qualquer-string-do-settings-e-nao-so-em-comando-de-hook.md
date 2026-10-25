---
status: wip
date: 2026-10-25
req: "docs/req/REQ-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: validate casa o marcador do guard em qualquer string do settings, e não só em comando de hook

> Created: 2026-10-25 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md
ADR: ADR-2026-10-04, D16. Issue: #618.

Causa: `collectCommandsWithMarker` (`internal/validator/validator_credential_guard.go:332`) percorre o JSON inteiro e casa
o marcador em qualquer string. 12 chamadores passam o arquivo inteiro. Lição da REQ anterior (#613): a enumeração precisa
cobrir `scripts/` e os testes que afirmam o contrato atual, e não só o Go de produção.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com a evidência no ML que o fecha.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e matriz de sítios
> Dependencies: nenhuma. Bloqueia toda implementação.

### ML-0A — Threat model, campo de comando por CLI e matriz de chamadores
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-wave0-marcador-fora-de-hook.md` (novo; o único arquivo que o ML escreve)
**Acceptance criteria:**
- [x] **Campo de comando por CLI**, para projeto e global (Claude, Codex, Gemini, Cursor, Copilot, Kiro, Windsurf e Amazon Q): o caminho JSON exato em que o comando de hook vive, com a fonte (gerador do trackfw e documentação do CLI)
- [x] **Matriz de chamadores:** todo uso de `collectCommandsWithMarker` e de qualquer outra busca por substring em arquivo de hook, em `internal/` **e** `scripts/`, com arquivo:linha, a regra que alimenta (violation, aviso ou presença) e se recebe o arquivo inteiro ou o subárvore
- [x] **Fail-open:** para cada regra de presença, a medição de se uma string fora de hook conta como fiação, com fixture em `t.TempDir()` ou `mktemp -d` no binário 11.0.1
- [x] **Falso positivo:** a reprodução do caso do `.autoMode.environment` e de uma entrada `permissions.allow` no `.claude/settings.json` de projeto
- [x] Testes que afirmam o comportamento atual e vão mudar, e cenários de `scripts/check-gates-falsify.sh` que dependem dele
- [x] Recomendação de `Files affected` para a Wave 1

**Auditoria do arquiteto (2026-10-25):** aprovado.
- **Fail-open reproduzido pelo arquiteto:** em `mktemp -d` com HOME falso, um `.windsurf/hooks.json` com
  `"command":"echo x"` e a linha exata do guard em `"note"` dá `validate` rc=0, sem nenhuma mensagem do Windsurf. O
  controle sem o `note` dá rc=1 ("has no credential guard entry"). Com texto não exato no `note`, o resultado é
  violation de forma (rc=1), e por isso o fail-open exige a linha exata.
- **Conclusões que mudam o desenho:**
  - restringir ao subárvore `hooks` não basta, porque L758/L773 já passam só o evento e deixam passar;
  - restringir à chave `command` cega o Copilot global (`bash`).
  O coletor precisa conhecer o esquema de cada CLI.
- **Lacuna vizinha:** o Windsurf global está ausente de `globalGuardConfigFiles`. É a mesma superfície (arquivo global de
  hook não lido), e entra no ML-1A.
- **Pendente, medido no ML-1A:** o fail-open no Amazon Q com a estrutura correta, no Copilot global e no Kiro aninhado.
- **Nota do vault:** `vault/notes/validator-guard-marcador-em-campo-nao-comando-fail-open-2026-10-25.md`.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-25-wave0-marcador-fora-de-hook.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Coleta restrita a comando de hook em todos os sítios
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/validator/validator_credential_guard.go`, `internal/validator/validator_git_branch_guard.go`, `internal/validator/validator_credential_guard_test.go`, `internal/validator/validator_guard_hook_concordance_external_test.go`, `internal/validator/validator_guard_command_schema_test.go` (novo), `docs/cli-parity.md`, `internal/validator/validator_guard_wiring.go`, `internal/validator/validator_git_branch_guard_test.go`, `internal/validator/export_test.go`, `internal/validator/validator_guard_d16_wiring_external_test.go` (novo) (acrescentados na auditoria)
**Acceptance criteria:**
- [x] AC1–AC5 da REQ, com contra-braço em `origin/main`
**Auditoria do arquiteto (2026-10-25):** aprovado depois do corretivo ML-1A2.
- **Primeira entrega, reprovada:** cobria só o Windsurf. Faltavam os testes de Amazon Q, Kiro, `guard_wiring_removed` e
  `permissions.allow` do projeto, a concordância do lado projeto, o teste contra o coletor livre, o Windsurf global e o
  contra-braço.
- **Código:** o coletor livre `collectCommandsWithMarker` saiu, e os 14 chamadores usam o coletor por esquema. O Windsurf
  global entrou em `globalGuardConfigFiles`. O Kiro não tem regra de presença por disco, e isso está declarado.
- **Contra-braço medido pelo arquiteto:** os testes novos rodados numa cópia de `origin/main` (via `git archive`, sem o
  teste de layout, que usa símbolo novo).
  - **Reprovam lá:**
    - `Windsurf_MarkerSoEmNote` e `AmazonQ_MarkerSoEmNote` (fail-open);
    - `AutoModeEnvironment_SemAviso`, `AutoModeComHookCorreto` (AC5), `PermissionsAllowNaoGeraViolation`,
      `EnvMarkerNaoGeraViolation` e o braço "decoy" (falso positivo);
    - `RecursivoAusente_CodigoFonte` (concordância);
    - `Windsurf_MissingScript` (Windsurf global lido).
  - **Passam lá (fixações de regressão, declaradas):** `GuardWiringRemoved_MutacaoDoComandoParaEchoX`, nos 8 CLIs,
    porque o `guard_wiring_removed` já usava o extrator por esquema; e `CopilotGlobal_MarkerEmNote`.
- **Gates do executor:** `go build`, `go vet` e os testes de `validator`, `generators` e `commands` com rc=0.

## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-25-revisao-marcador-fora-de-hook.md`

### ML-2B — Red-team
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-redteam-marcador-fora-de-hook.md`

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality, CI e PR
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] Neste repositório, `trackfw validate` (binário da branch) sem o aviso falso de `~/.claude/settings.json`
- [ ] CI verde; PR com `Closes #618`
