---
status: backlog
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md"
squad: ""
---

# Roadmap: instalacao de skills de terceiro via URL para agentes especialistas

> Created: 2026-08-15 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md -->
REQ: docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md

**Nota (2026-08-15, restrição do usuário):** o comando de instalação de skill só pode
ser executado dentro de uma sessão de agente e apenas pelo orquestrador/arquiteto
(`trackfw_architect`/Zeus) — nunca por invocação humana direta de terminal, nunca por
um agente especialista. Fluxo obrigatório: usuário aponta URL → Zeus invoca `hades-tf`
para análise de segurança (prompt injection, agent kidnapping) → só com parecer
favorável a instalação prossegue. Ver REQ vinculada para o detalhe completo — este
roadmap ainda está com MLs em formato stub (gerados automaticamente a partir da REQ) e
DEVE ser reescrito com Waves/MLs detalhados (arquivos exatos, ações exatas) antes do
início da implementação, incluindo uma Wave 0 dedicada à revisão de segurança do
`hades-tf`, que bloqueia toda e qualquer Wave de implementação subsequente.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — Novo comando (ex.: trackfw skill add <url>) baixa o conteúdo, mas **nunca o
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Novo comando (ex.: trackfw skill add <url>) baixa o conteúdo, mas **nunca o
- [ ] build passes
- [ ] tests green

### ML-1B — Validação de que o conteúdo baixado NÃO contém instruções que tentam sobrescrever
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Validação de que o conteúdo baixado NÃO contém instruções que tentam sobrescrever
- [ ] build passes
- [ ] tests green

### ML-1C — "Decidir onde a skill deve residir": a skill nunca SUBSTITUI o arquivo de um
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] "Decidir onde a skill deve residir": a skill nunca SUBSTITUI o arquivo de um
- [ ] build passes
- [ ] tests green

### ML-1D — Escopo de instalação: local ao projeto por padrão (não escopo global —
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Escopo de instalação: local ao projeto por padrão (não escopo global —
- [ ] build passes
- [ ] tests green

### ML-1E — Registrar a proveniência (URL, hash/checksum do conteúdo baixado, data) em algum
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Registrar a proveniência (URL, hash/checksum do conteúdo baixado, data) em algum
- [ ] build passes
- [ ] tests green

### ML-1F — Comportamento idêntico nos 3 CLIs.
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Comportamento idêntico nos 3 CLIs.
- [ ] build passes
- [ ] tests green

### ML-1G — make quality passa sem novas divergências de paridade.
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] make quality passa sem novas divergências de paridade.
- [ ] build passes
- [ ] tests green

### ML-1H — Revisão de hades-tf documentada (ADR ou seção de segurança dedicada no roadmap)
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Revisão de hades-tf documentada (ADR ou seção de segurança dedicada no roadmap)
- [ ] build passes
- [ ] tests green
