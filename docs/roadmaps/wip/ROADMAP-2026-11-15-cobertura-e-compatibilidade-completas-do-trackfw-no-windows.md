---
status: wip
date: 2026-11-15
req: "docs/req/REQ-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md"
squad: "hades-tf, artemis-tf"
---

# Roadmap: Cobertura e compatibilidade completas do trackfw no Windows

> Created: 2026-11-15 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md

O KG pediu cobertura e compatibilidade completas no Windows, porque nem todo usuário roda o trackfw no macOS.
A medição da `main` (run 43423202733, `windows-full-suites`) deu 71 testes únicos pulados e 5 known-failures.
Pelo menos duas justificativas de skip são falsas: os 41 `TestMLScope*` e os `TestBarrierReentry_*`. Os
artefatos do consumidor (`.sh`) e os hooks de Cursor e Kiro nunca foram medidos no Windows, e npm/PyPI não têm
smoke no Windows. A REQ traz o diagnóstico completo e a definição mensurável de "completo".

Varredura de issues abertas (2026-11-15): nenhuma trata deste mecanismo. Também não há REQ aberta sobre Windows,
porque a REQ-2026-11-13 foi encerrada neste mesmo PR.

Este PR também carrega o fechamento pós-merge da REQ-2026-11-13 (D18, PR #687): o roadmap vai para `done` e a
REQ para `Done`.

## Acceptance Criteria
- [ ] AC1 — inventário completo com classificação (a)/(b)/(c) por item (Wave 0)
- [ ] AC2 — zero skip de Windows com justificativa falsa
- [ ] AC3 — gate de CI contra skip de Windows sem classificação
- [ ] AC4 — artefatos e hooks por CLI executam no Windows sem Git Bash, ou são declarados pelo validate/doctor
- [ ] AC5 — smoke npm e PyPI no `windows-latest`
- [ ] AC6 — guard via perfil e sem `-NonInteractive` no PS 5.1 e no `pwsh` 7 do Windows
- [ ] AC7 — `windows-full-suites` e `make quality` verdes

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat model e inventário (2 MLs em paralelo; arquivos disjuntos)
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model
**Status:** 🔄 In progress
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-15-wave0-windows-threat-model.md` (novo)
**Actions:**
1. Completude da enumeração: a lista de superfícies da REQ está completa? Procure no repositório outros
   emissores do mesmo artefato (`runtime.GOOS == "windows"`, `t.Skip`, geradores de `.sh`, hooks por CLI,
   workflows de release) antes de declarar a lista fechada.
2. Threat model: quem consegue esvaziar esta Wave 0 sem violar nenhuma regra escrita, e como? Por exemplo,
   reclassificando skip como (c) sem prova, ou fazendo o gate de skip aceitar comentário em vez de evidência.
3. Alvos de falsificação nas duas direções, para cada superfície.
4. Residual declarado.
**Acceptance criteria:**
- [ ] as quatro seções respondidas com evidência, não com uma linha de afirmação
- [ ] nenhuma linha de implementação

### ML-0B — Inventário de Windows
**Status:** 🔄 In progress
**Squad:** artemis-tf
**Files affected:** `docs/portabilidade/2026-11-15-inventario-windows.md` (novo)
**Actions:**
1. A partir do log do `windows-full-suites` da `main` atual (`go test -v`), listar cada skip único e cada
   known-failure, com a justificativa citada, a classificação (a)/(b)/(c) proposta e a evidência.
2. Para cada artefato gerado para o consumidor e cada hook por CLI (Claude, Codex, Gemini, Copilot, Cursor, Kiro,
   Windsurf, Amazon Q), dizer se roda no Windows sem Git Bash, com evidência (código e, se possível, CI ou VM).
3. Canais de distribuição: o que já tem smoke no Windows e o que falta (npm, PyPI, `install.ps1`, arm64).
4. Propor os MLs de implementação, agrupados por causa (Regra Dura de Causa Raiz).
**Acceptance criteria:**
- [ ] cada item com fonte (linha do log, arquivo:linha ou medição); contagem conferida contra o log
- [ ] nenhuma linha de implementação

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-15-wave0-windows-threat-model.md
test -s docs/portabilidade/2026-11-15-inventario-windows.md
```

## Wave 1 — Justificativas falsas já conhecidas (definida após a auditoria da Wave 0)
> Dependencies: Wave 0 auditada

### ML-1A — `TestMLScope*` no Windows
**Status:** ⬜ Pending
**Squad:** artemis-tf
**Files affected:** `internal/commands/barrier_ml_scope_test.go` (confirmar na Wave 0)
**Acceptance criteria:**
- [ ] os 41 testes rodam no Windows, ou cada um tem prova (c)
- [ ] `windows-full-suites` verde

### ML-1B — `TestBarrierReentry_*` no Windows
**Status:** ⬜ Pending
**Squad:** artemis-tf
**Files affected:** `internal/commands/barrier_reentry_test.go` (confirmar na Wave 0)
**Acceptance criteria:**
- [ ] T1/T2/T3/T7/T7e rodam no Windows, ou cada um entra em `windows-known-failures.json` com causa medida
- [ ] `windows-full-suites` verde

## Wave 2..N — MLs propostos pela Wave 0
> Dependencies: Wave 1. Detalhados aqui após a auditoria do inventário.

## Wave final — Red team, `make quality` e CI de Windows
> Dependencies: todas as anteriores

### ML-FA — Red team da entrega
**Status:** ⬜ Pending
**Squad:** hades-tf
**Acceptance criteria:**
- [ ] parecer com cada tentativa e o rc medido

### ML-FB — `make quality` com a máquina ociosa e CI verde
**Status:** ⬜ Pending
**Squad:** zeus-tf
**Acceptance criteria:**
- [ ] `make quality` rc=0
- [ ] `windows-full-suites` verde, e as falhas, por nome, iguais à lista conhecida
