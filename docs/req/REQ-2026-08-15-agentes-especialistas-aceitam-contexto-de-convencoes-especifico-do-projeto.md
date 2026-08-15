---
status: Done
date: 2026-08-15
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-agentes-especialistas-aceitam-contexto-de-convencoes-especifico-do-projeto.md"
---

# REQ: agentes especialistas aceitam contexto de convencoes especifico do projeto

> Date: 2026-08-15 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Análise dos 12 agentes especialistas do catálogo canônico (`~/.claude/agents/trackfw-*.md`)
confirmou, por diff direto entre os arquivos: ~90% do conteúdo é template de governança
idêntico (mode lock, working context, vault, git authority, protocolo de microbatch) —
a parte específica de cada disciplina é uma única frase na seção "Mission" (ex.: Backend/
Apolo: *"Preserve public contracts, Clean Architecture boundaries, observability and
trackfw traceability"*). Não há nenhuma orientação de stack, framework, padrão de teste
ou convenção arquitetural específica de projeto.

Achado importante: isso **não é um viés dos meus (KG) projetos pessoais vazando para o
catálogo** — os arquivos são deliberadamente stack-agnósticos, o que é correto para um
produto que atende múltiplos times com stacks diferentes. O problema real é o oposto:
são genéricos demais para dar orientação de engenharia diferenciada além de "leia o
código antes de agir, não invente" — o que funciona quando o código existente é claro e
consistente, mas falha silenciosamente em repos legados/inconsistentes ou quando a
convenção vive só na cabeça do time, não no código.

**Fundação já existente a reaproveitar:** `trackfw.yaml` já tem campos `backend:`,
`frontend:`, `pkg_manager:`, `hooks:` (`internal/config/config.go`), populados pelo
`trackfw discover`. Essa é a base natural para a solução — não inventar um mecanismo de
detecção de stack do zero.

## Acceptance Criteria
- [x] Novo campo em `trackfw.yaml` (ex.: `agent_conventions:` ou reaproveitar/estender
      `backend:`/`frontend:` existentes) onde o projeto declara convenções específicas —
      framework de teste, padrão de arquitetura, estilo de API, linter — como texto livre
      ou lista curta de bullets (decisão de design do ML: manter simples, não um schema
      rígido).
- [x] `trackfw discover`/`trackfw init` propõe um preenchimento inicial desses campos por
      heurística (ex.: presença de `jest.config.js` → sugere framework de teste; mas não
      travar a REQ nisso — detecção automática é nice-to-have, o campo manual é o
      essencial).
- [x] O bloco de regras que o trackfw já injeta em `CLAUDE.md`/`AGENTS.md`/etc.
      (`internal/generators/agentfiles.go`, `trackfwRulesBlock()`) passa a incluir essas
      convenções, para que QUALQUER agente (não só os do catálogo trackfw) as veja — ou,
      alternativamente, os agentes do catálogo canônico ganham uma seção nova
      ("Project conventions") que lê esse campo do `trackfw.yaml` — decisão de design a
      resolver no ML, mas preferir o caminho que já existe (`trackfwRulesBlock`) em vez
      de criar um segundo mecanismo de injeção de contexto.
- [x] Comportamento idêntico nos 3 CLIs (mesmo campo, mesmo texto injetado).
- [x] `make quality` passa sem novas divergências de paridade.
- [x] Documentar explicitamente que isso é convenção **declarada pelo time**, não
      detecção automática de "boas práticas" — o trackfw não deve tentar impor um
      padrão arquitetural, só propagar o que o projeto já decidiu.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-15-agentes-especialistas-aceitam-contexto-de-convencoes-especifico-do-projeto.md
