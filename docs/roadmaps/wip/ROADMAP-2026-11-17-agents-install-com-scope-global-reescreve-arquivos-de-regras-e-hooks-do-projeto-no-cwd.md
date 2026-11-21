---
status: wip
date: 2026-11-17
req: "docs/req/REQ-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: agents install com scope global reescreve arquivos de regras e hooks do projeto no cwd

> Created: 2026-11-17 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md -->
REQ: docs/req/REQ-2026-11-17-agents-install-com-scope-global-reescreve-arquivos-de-regras-e-hooks-do-projeto-no-cwd.md
ADR: docs/adr/ADR-2026-11-21-agents-e-skills-install-com-scope-global-nao-escrevem-no-projeto.md

Diagnóstico na REQ: `internal/commands/integrations_flags.go:277-296` injeta regras e hooks no `ProjectRoot` sem olhar
`--scope`. Medido no red team ML-FA da REQ-2026-11-15 (reescreveu `CLAUDE.md`/`AGENTS.md` do próprio trackfw).
Decisão do KG em 2026-11-21: com `--scope global`, nada é escrito no projeto (ADR D1–D5).

Varredura em 2026-11-21: nenhuma issue aberta; nenhuma outra REQ aberta com o mesmo mecanismo.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1: contrato D1–D4 escrito em `docs/cli-parity.md` (seção de `agents|skills install`)
- [x] AC2: teste com braço e contra-braço: `--scope global` explícito e default não alteram nenhum byte de
  `CLAUDE.md`/`AGENTS.md`/hooks versionados no cwd e imprimem a dica; `--scope project` continua escrevendo
- [x] AC3: varredura dos demais sítios com escopo global que escrevem regras/hooks no cwd, com o contrato aplicado
  (mesma causa, mesma REQ) ou a lista fechada escrita com evidência
- [ ] AC4: `make quality` rc=0 e CI verde no PR

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model and sweep (hades-tf)
**Status:** ✅ Done — `docs/seguranca/2026-11-21-wave0-scope-global-nao-toca-projeto-threat-model.md`. Sítio único:
`integrations_flags.go:278-298` (o `InjectWindsurfHooks` interno também injeta regras, então o bloco inteiro é gatado).
`update`/`uninstall` não injetam; nenhum cenário de `check-gates-falsify.sh` afirma o contrato antigo; só o comentário
de `check-update-parity.sh:156-159`. Ameaças T1–T9 e residuais R1–R4. Auditoria do arquiteto: R1 (`third-party
install` grava `.trackfw/thirdparty-provenance.json`) fica fora do D1, que trata de regras e hooks; o hades escreveu
um `_test.go` de medição fora do escopo do handoff, que foi retirado da árvore.
**Files affected:** `docs/seguranca/2026-11-21-wave0-scope-global-nao-toca-projeto-threat-model.md` (novo; único arquivo)
**Actions:**
1. Enumeration completeness — list every code path where a command running with global scope (explicit or default)
   writes under the project root / cwd: start at `integrations_flags.go:277-296`, then search callers of
   `InjectRulesForTool`, `InjectRulesDetected`, `InjectHooksForTools`, `InjectHooksDetected`, `InjectWindsurfHooks`
   and any other writer reached from `agents|skills install|update|uninstall` and `third-party install`. Close the
   list with evidence (grep commands and their output), or name what is missing.
2. Threat model — who empties D1 without breaking a written rule, and how (scope resolved after the write,
   interactive prompt default, deprecated alias setting `scopeExplicit`, `ProjectRoot` resolution).
3. Falsification targets in both directions — global writes to the project (regression of D1); project stops writing
   (regression of D3).
4. Declared residual.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML (o `_test.go` de medição foi retirado e não entra no commit)

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-21-wave0-scope-global-nao-toca-projeto-threat-model.md
```

## Wave 1 — Implementation
> Dependencies: Wave 0 audited.

### ML-1A — D1–D4 no install e testes de escopo (apolo-tf)
**Status:** ✅ Done — bloco pós-install gatado por `opts.scope == "project"` depois do `resolveScope`; dica D2 em
stderr (`--json` intacto). 9 testes em `integrations_scope_project_writes_test.go`. Contra-braço do arquiteto: sem a
correção, os 7 braços globais reprovam e os 2 de project passam. Nenhum teste existente afirmava o contrato antigo.
Gates do apolo: build, vet, `internal/commands`, `i18nguard`/`pathguard`, `check-update-parity.sh` e `make parity-rest`, todos rc=0.
**Files affected:** `internal/commands/integrations_flags.go`; `internal/commands/integrations_scope_project_writes_test.go`;
`scripts/check-update-parity.sh` (comentário que afirmava o contrato antigo); `docs/cli-parity.md`
**Actions:**
1. Executar o bloco de `integrations_flags.go:277-296` (regras, hooks e aviso D8) só quando o escopo resolvido é
   `project`.
2. Com escopo `global`, imprimir a dica do D2 depois de instalar.
3. Teste com braço e contra-braço por escopo (global explícito, default, project).
4. Atualizar testes e cenários de `scripts/check-gates-falsify.sh` que afirmem o contrato antigo.
5. Documentar D1–D4 em `docs/cli-parity.md`.
**Acceptance criteria:**
- [x] AC1, AC2 e AC3 da REQ
- [x] `go build ./...` e `go test ./internal/commands/` verdes

**Wave gates:**
```bash
go build ./...
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/commands/
```

## Wave 2 — Review and red team
> Dependencies: Wave 1 audited.

### ML-2A — Revisão de qualidade (hefesto-tf)
**Status:** ✅ Done — APROVA com 8 achados não bloqueantes (NB-1..NB-8), corrigidos no ML-2C.
**Files affected:** `docs/qualidade/2026-11-21-scope-global-nao-toca-projeto-review.md`
**Acceptance criteria:**
- [x] Parecer escrito; achados bloqueantes corrigidos por ML corretivo (nenhum bloqueante; NB no ML-2C)

### ML-2B — Red team (hades-tf)
**Status:** ✅ Done — APROVA com ressalvas: nenhum bypass em 10 alvos × global com contra-braço de project. R1
(provenance de `third-party install`) fica fora do D1 por decisão do arquiteto; R3 (`os.Stderr` direto) entra no ML-2C.
O `zz_redteam_scope_global_test.go` de medição foi retirado da árvore.
**Files affected:** `docs/seguranca/2026-11-21-red-team-scope-global-nao-toca-projeto.md`
**Acceptance criteria:**
- [x] Reimplementa a falsificação a partir do contrato, não do diff; residual declarado

### ML-2C — Corretivo dos achados da Wave 2 (apolo-tf)
**Status:** ✅ Done — NB-1..NB-8 corrigidos; as 5 saídas de `executeIntegrationMutation` em `os.Stderr` passaram a
`cmd.ErrOrStderr()`; `TestAntigravityD4ScopeGatesD8Warning` novo. Contra-braço do arquiteto: sem o gate de escopo, o
teste do D4 reprova no braço global. `internal/commands` e `make parity-rest` rc=0.
**Files affected:** `internal/commands/integrations_flags.go`; `internal/commands/integrations_scope_project_writes_test.go`;
`docs/cli-parity.md`
**Actions:** NB-1..NB-8 do ML-2A; saídas de `executeIntegrationMutation` em `os.Stderr` passam a `cmd.ErrOrStderr()`
(NB-3 e R3 do ML-2B), com teste do D4 capturável.
**Acceptance criteria:**
- [x] cada NB corrigido ou justificado no relatório
- [x] teste afirma que o aviso D8 não sai com escopo global e sai com project

**Wave gates:**
```bash
test -s docs/qualidade/2026-11-21-scope-global-nao-toca-projeto-review.md
test -s docs/seguranca/2026-11-21-red-team-scope-global-nao-toca-projeto.md
```

## Wave 3 — Fechamento
> Dependencies: Wave 2 audited.

### ML-FB — make quality e CI (zeus-tf)
**Status:** ⬜ Pending
**Acceptance criteria:**
- [ ] AC4: `make quality` rc=0 com a máquina ociosa; CI verde no PR

**Wave gates:**
```bash
go build ./...
```
