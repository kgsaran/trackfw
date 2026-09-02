---
status: Open
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-alinhar-a-versao-do-package-lock.md"
---

# REQ: `npm/package-lock.json` declara 6.1.0 — cinco majors atrás do `package.json`

> Date: 2026-09-02 | Status: Open

## Motivation

```
npm/package.json       "version": "7.3.0"
npm/package-lock.json  "version": "6.1.0"   ← nos dois sítios: raiz e packages[""]
```

## O que eu esperava, e o que medi

Eu esperava que `npm ci` reprovasse. **Não reprova:**

```
$ npm ci
added 42 packages, and audited 43 packages in 18s
```

`npm` só reclama quando as **dependências** divergem, e elas não divergem — as três (`commander`,
`@inquirer/prompts`, `yaml`) são idênticas entre os dois arquivos.

Também procurei quem lê essa versão. **Ninguém lê:** nenhum gate em `scripts/`, nenhum workflow em
`.github/` (o único `package-lock` citado lá é o de `site/`), nenhum código nos três runtimes.

**Então não há quebra medida hoje.** Digo isso na frente para não vender como bug o que é higiene.

## O que sobra, e por que ainda vale corrigir

O passo "bump version files" do protocolo de release em `CLAUDE.md` **pula este arquivo em silêncio**,
e vem pulando desde a 6.1.0. O que está errado não é o efeito — é que o arquivo é a única declaração
de versão do projeto que ninguém verifica, e por isso ela derivou cinco majors sem ninguém notar.

Um número errado que ninguém lê hoje é um número errado que alguém vai ler amanhã, acreditando nele.

## Acceptance Criteria

- [ ] **AC1** — Os dois sítios de `"version"` no lock passam a `7.3.0`, casando com o `package.json`.
- [ ] **AC2** — `npm ci` continua funcionando (não regride o que já funcionava).
- [ ] **AC3** — Nenhuma dependência muda. O lock não é regenerado; só o campo de versão é corrigido,
      para o diff ser lido em dez segundos.

## Negative Scope

**Não regenera o lock.** Regenerar mudaria a árvore de dependências resolvidas junto, e aí o diff
deixaria de ser auditável. `npm audit` reporta 4 vulnerabilidades (3 low, 1 high) na árvore atual —
é assunto real, e é assunto **separado** deste.

**Não instala gate.** Um `check` que compare `package.json` com `package-lock.json` fecharia a
classe inteira, mas é decisão de vocês sobre onde ele entra no `Makefile`. Aqui só corrijo o valor.

## Linked ADR
<!-- Higiene de manifesto; sem decisão nova a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-02-alinhar-a-versao-do-package-lock.md
