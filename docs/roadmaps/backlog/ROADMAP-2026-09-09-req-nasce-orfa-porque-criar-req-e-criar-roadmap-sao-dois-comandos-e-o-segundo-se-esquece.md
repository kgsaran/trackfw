---
status: backlog
date: 2026-09-09
req: "docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md"
squad: ""
---

# Roadmap: REQ nasce orfa porque criar REQ e criar roadmap sao dois comandos e o segundo se esquece

> Created: 2026-09-09 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md -->
REQ: docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md

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

### ML-1A — **AC1** — 🔴 **Prevenção antes de gate:** criar REQ e roadmap deixa de exigir dois comandos.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — 🔴 **Prevenção antes de gate:** criar REQ e roadmap deixa de exigir dois comandos.
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — severidade endurecida **por data de corte**, não por contagem: REQ criada a partir de
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — severidade endurecida **por data de corte**, não por contagem: REQ criada a partir de
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **O grandfathering é visível, não silencioso.** O relatório diz quantas REQs estão
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **O grandfathering é visível, não silencioso.** O relatório diz quantas REQs estão
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — decisão escrita sobre **onde bloqueia**: só validate, ou também push. 🔴 O push
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — decisão escrita sobre **onde bloqueia**: só validate, ou também push. 🔴 O push
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — re-triagem das 32: quantas **legitimamente** não têm roadmap (decisão pura, fechada sem
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — re-triagem das 32: quantas **legitimamente** não têm roadmap (decisão pura, fechada sem
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — paridade nos 3 CLIs.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — paridade nos 3 CLIs.
- [ ] build passes
- [ ] tests green

### ML-NOVO — 🔴 Frontmatter e corpo são DUAS fontes de verdade, e os comandos discordam

**Medido pelo arquiteto em 2026-09-11, ao "corrigir" REQs órfãs e descobrir que a correção não valeu.**

```
trackfw validate      lê o MARCADOR DE CORPO   "Roadmap: `docs/...`"
trackfw roadmap new --from-req   escreve       (nada — não fecha o laço)
edição do frontmatter roadmap:   escreve       o FRONTMATTER
```

**Consequência medida:** eu vinculei 4 REQs órfãs editando `roadmap:` no frontmatter. **As quatro
continuaram órfãs para o `validate`**, que lê outro lugar. O conserto real exigiu preencher o
marcador de corpo — e aí a contagem caiu de **57 → 52**.

**E as duas contagens discordam por construção:**

```
roadmap: "" no frontmatter          27 REQs   ← o que eu media
"no linked Roadmap" no validate     57 REQs   ← o que o produto mede
```

🔴 **Duas noções de "vinculado", e nenhuma sabe da outra.** A auditoria de governança de 2026-09-10
usou a contagem errada — o número real de REQs sem roadmap **aos olhos do produto** era o dobro.

#### É a mesma causa do issue #306

O **#306** relata que `req list` lê o `status:` **do corpo** e ignora o frontmatter, contradizendo o
`validate`. **Mesmo mecanismo, outro campo.** O relator classificou como *"não é quebra de paridade,
é o contrato"* — e está certo: o contrato tem **duas fontes de verdade para metadado de REQ**, e cada
comando escolhe uma.

**Por isso este ML entra aqui e não em REQ nova:** a causa é a divergência frontmatter↔corpo, não o
campo específico. Corrigir só o `status` deixaria o `roadmap` quebrado, e vice-versa.

#### O que a correção precisa decidir

1. **Qual é a fonte de verdade** — frontmatter, presumo, por consistência com `status`/`serve`, mas é
   **decisão a registrar**, não a inferir;
2. **O que fazer com os dois valores divergindo** — silêncio, aviso ou violação. Corrigir a leitura
   sem decidir isto troca um defeito por outro;
3. **Todo comando que escreve REQ escreve os DOIS**, ou o gerador para de emitir o marcador de corpo
   como placeholder vazio — que é o que cria a órfã silenciosa;
4. **Paridade nos 3 CLIs.**

**Falsificação:** REQ com frontmatter preenchido e corpo vazio ⇒ o comportamento decidido em (2);
REQ com os dois preenchidos e **divergentes** ⇒ idem; REQ com os dois iguais ⇒ passa (contra-braço).
