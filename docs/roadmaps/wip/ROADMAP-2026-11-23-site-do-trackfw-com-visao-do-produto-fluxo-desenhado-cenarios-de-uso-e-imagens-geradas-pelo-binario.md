---
status: wip
date: 2026-11-23
req: "docs/req/REQ-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md"
squad: ""
---

# Roadmap: Site do trackfw com visão do produto: fluxo desenhado, cenários de uso e imagens geradas pelo binário

> Created: 2026-11-23 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md -->
REQ: docs/req/REQ-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md

## Acceptance Criteria
- [ ] AC1: conceito aprovado pelo KG
- [ ] AC2: protótipo validado visualmente pelo KG e descartado
- [ ] AC3: ADR e waves de implementação escritos após a validação
- [ ] AC4: site final en + pt-BR com imagens geradas por script, sem dado pessoal
- [ ] AC5: teste do Quick start verde; build do site rc=0; capturas headless
- [ ] AC6: `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Runs in parallel with Wave 1 (different files); blocks Wave 2 (prototype code).

### ML-0A — Threat model do site novo (hades-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md` (novo; único arquivo)
**Actions:**
1. Enumeration completeness: every surface the new site adds or changes — images and GIFs generated from the binary
   (what they can capture: home paths, usernames, tokens, hostnames, git remotes, dates), new npm dependencies
   (diagram plugins, image tooling), external assets (fonts, CDNs, embeds), the deploy workflow
   (`.github/workflows/deploy-docs.yml`) and its permissions.
2. Threat model: who leaks personal data or ships a malicious dependency through the site without breaking a written
   rule, and how.
3. Falsification targets in both directions: an image that leaks; a generator that over-redacts and shows nothing.
4. Declared residual.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md
```

## Wave 1 — Concept (Fase B1)
> Dependencies: none. No code.

### ML-1A — Conceito do site novo (atena-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/visao-projeto/2026-11-23-site-novo-conceito.md` (novo; único arquivo)
**Actions:** information architecture of the home and of the guide; the three usage scenarios (solo dev with an AI
agent, team with CI, company that only allows npm/pip), each with who, pain, what trackfw does, and the 3-5 commands
that show it; the drawn flow ADR → REQ → ROADMAP → waves → barrier → PR, as a text mockup; which images and GIFs, and
which binary command generates each; open questions for the KG. Benchmarks: 2-3 developer-tool sites, cited.
**Acceptance criteria:**
- [ ] Conceito com mockups em texto e perguntas abertas; o KG aprova ou pede ajuste (AC1)

**Wave gates:**
```bash
test -s docs/visao-projeto/2026-11-23-site-novo-conceito.md
```

## Wave 2 — Prototype (Fase B2)
> Dependencies: Wave 0 audited and Wave 1 approved by the KG. Planned after the concept; isolated, discarded before
> the final PR.

## Wave 3 — Implementation (Fase C)
> Dependencies: Wave 2 validated by the KG. ADR and microbatches written after validation.
