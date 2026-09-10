---
status: backlog
date: 2026-09-10
req: "docs/req/REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md"
squad: ""
---

# Roadmap: gate de palavra-chave de fechamento nao reavalia em edited e le exemplo citado como diretiva

> Created: 2026-09-10 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md -->
REQ: docs/req/REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md

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

### ML-1A — **AC1** — O gate reavalia no evento edited, não só na abertura.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — O gate reavalia no evento edited, não só na abertura.
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **Sem repetir as suítes.** Um edited que dispare o quality inteiro faz o custo
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **Sem repetir as suítes.** Um edited que dispare o quality inteiro faz o custo
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — Falsificação nas duas direções para o edited: PR aberto sem palavra-chave e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — Falsificação nas duas direções para o edited: PR aberto sem palavra-chave e
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
- [ ] build passes
- [ ] tests green

### ML-NOVO — A adjacência quebra com markdown entre a palavra e o `#N`

**Status:** ⬜ Pendente · **medido pelo arquiteto em 2026-09-10, contra o próprio PR #312**

```
Fecha #274.                   → gate RECUSA   ✅ correto
Fecha **#274** e **#275**.    → gate PASSA    🔴 defeito
```

O `**` do negrito entre a palavra e o `#N` quebra a adjacência que o gate procura. **Não foi ataque:
é markdown normal**, escrito sem intenção de driblar nada.

**Consequência medida:** o PR **#312** foi mergeado com *"Fecha **#274** e **#275**"*, o gate ficou
**verde**, e os dois issues **continuaram abertos**. Fechados à mão depois.

🔴 **É a mesma classe dos outros dois defeitos deste issue:** o gate mede uma forma mais estreita do
que a que afirma cobrir. Ver
`vault/notes/guarda-que-reporta-ausencia-precisa-distinguir-nao-achei-de-nao-consegui-procurar-2026-09-10.md`.

**Falsificação obrigatória — as formas que um humano escreve sem pensar:**

```
Fecha **#N**        Fecha o **#N**        Fecha [#N](url)
Fecha `#N`          Fecha: #N             Fecha os #N e #M
```

E o contra-braço: prosa legítima que **cita** um issue sem pretender fechá-lo **não** pode reprovar —
senão o gate vira ruído e alguém o desliga.
