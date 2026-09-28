---
status: wip
date: 2026-09-28
req: "docs/req/REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md"
squad: ""
---

# Roadmap: a direcao roadmap para REQ nao tem o tratamento de stale que a direcao inversa ja tem

> Created: 2026-09-28 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md -->
REQ: docs/req/REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: medir o que NÃO precisa ser construído
> Dependências: nenhuma. **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(git ls-files 'docs/req/*.md' | xargs -n1 basename | sort -u | wc -l | tr -d ' '); t=$(git ls-files 'docs/req/*.md' | wc -l | tr -d ' '); test "$n" = "$t" && echo "Gate W0: $t REQs, $n basenames unicos — zero colisao" || { echo "GATE FALHOU: $t REQs mas $n basenames unicos — ha colisao, o ramo de ambiguidade e alcancavel" >&2; exit 1; }
```

⚠️ Este gate **passa hoje** e é de vigilância: se algum dia houver colisão de basename, o ramo de
ambiguidade deixa de ser inalcançável e a decisão do `ML-0A` precisa ser revista.

### ML-0A — o ramo de ambiguidade é alcançável?
**Owner:** `hades-tf`
**Status:** ⬜ Pendente

🔴 **A pergunta que decide escopo, e a resposta provável é "não".** Medi **0 colisões em 233 REQs** —
a convenção `REQ-YYYY-MM-DD-slug` parece garantir unicidade. Se o ramo `default` (ambíguo) for
**inalcançável**, implementá-lo é **dívida disfarçada de segurança**: código que nenhum teste
consegue exercitar sem fabricar estado impossível.

**Ações:**
1. Determinar se **duas REQs com o mesmo basename** são criáveis pelo produto — `req new` recusa?
   E num consumidor com `by_agent`, dois agentes podem ter REQs homônimas?
2. 🔴 Se for alcançável, **como** — e aí o ramo entra. Se não for, **declarar** e deixar de fora.
3. Decidir o **AC4**: `ref_targets_exist` só varre `wip` e `blocked`. Ampliar para `backlog` custa o
   quê, e o que se perde deixando como está?

**Critérios de aceite:**
- [ ] Veredito escrito sobre alcançabilidade do ramo de ambiguidade, com a medição
- [ ] Decisão do AC4 (ampliar × declarar), com o custo de cada lado
- [ ] 🔴 Nenhuma linha de implementação neste ML

## Wave 1 — Espelhar o tratamento que a direção inversa já tem
> Dependências: **Wave 0 auditada**.

Os MLs saem **depois** do `ML-0A` — o escopo depende de o ramo de ambiguidade ser alcançável ou não.

O que já está decidido e independe disso:

- **espelhar** `resolveRoadmapRefStatus`, não escrever um segundo mecanismo (AC1);
- `links to ADR` fica **fora** — ADR não tem dimensão de estado (AC3);
- falsificação nas duas direções: REQ movida → **stale**; REQ apagada → **violação** (AC5).
