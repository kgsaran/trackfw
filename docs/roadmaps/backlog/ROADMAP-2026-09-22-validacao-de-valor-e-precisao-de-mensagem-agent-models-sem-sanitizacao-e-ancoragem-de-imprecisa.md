---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md"
squad: ""
---

# Roadmap: Validação de valor e precisão de mensagem — `agent_models` sem sanitização e ancoragem de `~` imprecisa

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md -->
REQ: docs/req/REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — **AC1** — Valor de agent_models validado por formato antes de interpolar; formato inválido
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Valor de agent_models validado por formato antes de interpolar; formato inválido
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Falsificação nas duas direções: valores legítimos (5, 4.6, 4-5-sonnet) aceitos;
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Falsificação nas duas direções: valores legítimos (5, 4.6, 4-5-sonnet) aceitos;
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — Mensagens de ~usuario/ e "~/" descrevem o motivo **real** da classificação.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — Mensagens de ~usuario/ e "~/" descrevem o motivo **real** da classificação.
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — A classificação em si **não muda** — o ADR-2026-08-22 está correto; o que muda é o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — A classificação em si **não muda** — o ADR-2026-08-22 está correto; o que muda é o
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Paridade nos 3; gate falsificável para AC1 e AC2.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Paridade nos 3; gate falsificável para AC1 e AC2.
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — make quality exit 0 e CI verde.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — make quality exit 0 e CI verde.
- [ ] build passes
- [ ] tests green
