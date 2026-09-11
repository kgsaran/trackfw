---
status: abandoned
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-by-agent-req-new-e-roadmap-new-sempre-criam-no-primeiro-agente-e-so-o-python-aceita-agent.md"
squad: ""
---

# Roadmap: by_agent: req new e roadmap new sempre criam no primeiro agente, e so o Python aceita --agent

> Created: 2026-09-11 | Status: abandoned

## Context
<!-- Derived from REQ: REQ-2026-09-11-by-agent-req-new-e-roadmap-new-sempre-criam-no-primeiro-agente-e-so-o-python-aceita-agent.md -->
REQ: docs/req/REQ-2026-09-11-by-agent-req-new-e-roadmap-new-sempre-criam-no-primeiro-agente-e-so-o-python-aceita-agent.md

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

### ML-1A — **AC1** — sem flag, o artefato nasce no agente **certo**, nao em agents[0]. O que e "certo"
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — sem flag, o artefato nasce no agente **certo**, nao em agents[0]. O que e "certo"
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — roadmap new --req <caminho> **herda o agente da REQ**, usando o mesmo mecanismo do
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — roadmap new --req <caminho> **herda o agente da REQ**, usando o mesmo mecanismo do
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — --agent existe e funciona nos **3 CLIs**, em req new e roadmap new. A
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — --agent existe e funciona nos **3 CLIs**, em req new e roadmap new. A
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — sitio do req new do Go **derivado**, nao presumido. O relator declarou que nao o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — sitio do req new do Go **derivado**, nao presumido. O relator declarou que nao o
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — falsificacao nas duas direcoes, com agents: [alpha, beta]: artefato vai para o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — falsificacao nas duas direcoes, com agents: [alpha, beta]: artefato vai para o
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — 🔴 **o consumer-smoke-by-agent passa a VERDE**, e o continue-on-error: true dele
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — 🔴 **o consumer-smoke-by-agent passa a VERDE**, e o continue-on-error: true dele
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — flat continua funcionando. A correcao nao pode quebrar quem nao usa by_agent.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — flat continua funcionando. A correcao nao pode quebrar quem nao usa by_agent.
- [ ] build passes
- [ ] tests green

---

## 🔴 SUPERSEDED no mesmo dia — 2026-09-11

**Esta REQ nao deveria ter sido aberta.** O defeito ja tinha REQ, desde **2026-08-29**:

```
docs/req/REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md
```

Ela ja nomeava a causa (`cfg.Agents[0]` em silencio), **ja tinha o mecanismo decidido pelo KG** — um
namespace usa, varios sem `--agent` falham nomeando — e a triagem de `2026-09-05` (linha 7) a marcava
**AINDA VALIDA (verificado)**, citando **"AC5 nao implementado"**.

Todo o conteudo do #320 foi movido para la como **AC10-AC15**, pela `Regra Dura de Causa Raiz`:
**mesma causa ⇒ mesma REQ ⇒ mesmo PR.**

### Como o engano aconteceu — e o que o evitou

Li o issue, medi o defeito, e parti para REQ nova sem procurar REQ existente com a mesma causa.
🔴 **E exatamente o padrao que a regra existe para impedir** — e ele produz backlog que cresce por
construcao, com o defeito vivo por tras da aparencia tranquilizadora de estar "registrado".

O que pegou foi a varredura que precedeu o desenho: ao procurar se `agents[0]` estava **documentado**
como default, apareceu `docs/portabilidade/2026-09-05-triagem-das-reqs-abertas.md` linha 7. **A REQ
duplicada durou minutos, e nao um ciclo, porque a medicao veio antes da decisao.**
