---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-serve-api-file-valida-o-caminho-lexico-e-abre-o-fisico-symlink-em-docs-req-le-qualquer-arquivo-no-go-e-no-node.md"
squad: ""
---

# Roadmap: serve /api/file valida o caminho lexico e abre o fisico: symlink em docs/req/ le qualquer arquivo no Go e no Node

> Created: 2026-09-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-11-serve-api-file-valida-o-caminho-lexico-e-abre-o-fisico-symlink-em-docs-req-le-qualquer-arquivo-no-go-e-no-node.md -->
REQ: docs/req/REQ-2026-09-11-serve-api-file-valida-o-caminho-lexico-e-abre-o-fisico-symlink-em-docs-req-le-qualquer-arquivo-no-go-e-no-node.md

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

### ML-1A — **AC1** — Go e Node canonicalizam **a raiz autorizada e o arquivo pedido** antes de comparar, e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Go e Node canonicalizam **a raiz autorizada e o arquivo pedido** antes de comparar, e
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — M-03: o servidor de estáticos do Node usa o mesmo tratamento.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — M-03: o servidor de estáticos do Node usa o mesmo tratamento.
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — resposta **403** e 🔴 **ausência do conteúdo no corpo** — asserir as duas coisas. Só
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — resposta **403** e 🔴 **ausência do conteúdo no corpo** — asserir as duas coisas. Só
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Contra-braço:** arquivo legítimo dentro da raiz continua devolvendo **200** com o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Contra-braço:** arquivo legítimo dentro da raiz continua devolvendo **200** com o
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — teste do vetor nos **3 runtimes**, inclusive no Python, que hoje passa: a defesa dele
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — teste do vetor nos **3 runtimes**, inclusive no Python, que hoje passa: a defesa dele
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — gate de segurança do servidor cobre o caso, com falsificação: revertendo o realpath,
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — gate de segurança do servidor cobre o caso, com falsificação: revertendo o realpath,
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — 🔴 **Varredura:** derivar **todo** sítio dos 3 runtimes que valida caminho
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — 🔴 **Varredura:** derivar **todo** sítio dos 3 runtimes que valida caminho
- [ ] build passes
- [ ] tests green
