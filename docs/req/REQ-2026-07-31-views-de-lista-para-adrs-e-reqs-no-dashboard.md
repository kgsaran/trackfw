---
status: Done
date: 2026-07-31
author: "Zeus"
adr: "docs/adr/ADR-2026-07-31-listas-de-adr-e-req-no-dashboard-derivadas-de-api-chain.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md"
---

# REQ: Views de lista para ADRs e REQs no dashboard

> Date: 2026-07-31 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

ADRs e REQs só são alcançáveis no dashboard como nós do grafo da aba **Chain**. Com 137 nós
(67 roadmaps, 58 REQs, 12 ADRs) e 118 arestas, o grafo force-directed não permite busca
dirigida — só exploração por tropeço. Não existe forma de responder "quais ADRs estão
`Proposed`?" ou "onde está a REQ do CLI Python?" sem abrir o repositório.

O backend necessário já existe e está provado funcionando: `/api/chain` devolve `id`
(= caminho relativo), `type`, `title` e `state`; `/api/file` serve o conteúdo e já
autoriza `ADRDirs`/`REQDir`/`RoadmapDir`; o drawer já renderiza markdown + frontmatter.
Falta apenas a camada de navegação.

## Acceptance Criteria

- [x] **AC1** — A barra de navegação exibe cinco abas: `Board`, `Chain`, `Metrics`, `ADRs`, `REQs`.
      Alternar entre elas usa o mecanismo `switchView()` já existente, sem recarregar a página.
- [x] **AC2** — A aba `ADRs` lista **exatamente 13** itens neste repositório com o filtro em
      "Todos"; a aba `REQs` lista **exatamente 59**. A contagem visível confere com
      `curl -s localhost:<porta>/api/chain | python3 -c "import json,sys;from collections import Counter;print(Counter(n['type'] for n in json.load(sys.stdin)['nodes']))"`.
- [x] **AC3** — Cada linha exibe identificador, título e status. O status é apresentado com
      indicador visual (cor/chip) legível.
      **Critério revisado durante a execução (ML-1B):** a redação original exigia legibilidade
      "em tema claro e escuro". O dashboard do trackfw é **light-only por design** — usa Tailwind
      via CDN sem `darkMode` configurado e sem nenhuma variante `dark:`. A primeira implementação
      adicionou um `@media (prefers-color-scheme: dark)` que fazia apenas as linhas novas trocarem
      de tema num SO em modo escuro, destoando de todo o resto da página. O bloco foi removido.
      Contraste medido no tema claro: `unknown` ~7:1, `Accepted` ~4.6:1 (WCAG AA), `Proposed` ~6:1,
      `Done` ~5.7:1. Suporte a tema escuro no dashboard inteiro é trabalho de REQ própria.
- [x] **AC4** — O filtro de status é **derivado dinamicamente** dos valores presentes na
      resposta de `/api/chain`, nunca de uma lista hardcoded. Com o filtro em "Todos",
      **nenhum** nó do tipo é omitido — inclusive os de status `unknown` (há 1 ADR assim hoje).
- [x] **AC5** — Existe busca textual que filtra por título e por identificador,
      case-insensitive e acento-insensitive. Busca vazia restaura a lista completa.
- [x] **AC6** — Clicar numa linha (ou pressionar Enter/Space com foco nela) abre o **drawer
      já existente** via `openDrawer(node.id)`. Nenhum caminho novo de leitura de arquivo é
      criado. O drawer renderiza markdown e a tabela de frontmatter como faz hoje.
- [x] **AC7** — Acessibilidade: as abas novas seguem o padrão das existentes
      (`aria-pressed`, `role="navigation"`); as linhas são focáveis via teclado
      (`tabindex="0"`) e possuem rótulo acessível. As `<section>` novas têm `aria-label`.
- [x] **AC8** — Estado vazio tratado: se um tipo não tiver itens (ou o filtro não casar nada),
      exibe mensagem explicativa em vez de lista em branco. Erro de fetch exibe
      mensagem com `role="alert"`, como nas views existentes.
- [x] **AC9** — **Nenhum arquivo novo** é adicionado a `internal/serve/static/`. As mudanças
      ocorrem apenas em `app.js`, `index.html` e `style.css`.
- [x] **AC10** — `npm/src/serve/static/` e `pypi/trackfw/serve/static/` são espelhos
      byte-a-byte de `internal/serve/static/` para os três arquivos.
      `scripts/check-static-assets.sh` imprime `Static assets are synchronized`.
- [x] **AC11** — `make build`, `make test`, `make lint` e `make parity` passam sem erro.
- [x] **AC12** — Verificação visual manual nos três CLIs: `trackfw serve` (Go, após
      `make build` — os assets são `go:embed`), `node npm/bin/trackfw.js serve` e
      `python -m trackfw serve` exibem as mesmas cinco abas com as mesmas contagens.

## Negative Scope (fora do escopo — NÃO fazer)

- **Nenhum endpoint HTTP novo.** Proibido criar `/api/docs` ou similar. Proibido alterar
  `api_chain.*`, `api_file.*`, `api_board.*`, `api_metrics.*`, `api_attention.*` em
  qualquer um dos três CLIs.
- **Nenhuma mudança de código Go/Node/Python de servidor.** A única exceção é o rebuild
  necessário pelo `go:embed`, que não altera fonte `.go`.
- **Nenhum arquivo estático novo** (sem `docs-list.css`, sem `docs-list.js`) —
  `check-static-assets.sh` deriva a lista de arquivos do canônico e falha nas duas direções.
- Não alterar as views `Board`, `Chain` e `Metrics` nem o comportamento do drawer.
- Não adicionar edição, criação ou exclusão de artefatos pelo dashboard — **leitura apenas**.
- Não exibir data de criação, ADR pareada ou contagem de roadmaps na linha da lista
  (metadados ausentes de `chainNode`) — decidido no ADR.
- Não introduzir framework, bundler ou dependência de frontend nova.
- Não mexer em `pypi/build/lib/` — é artefato de build.
- Não alterar `scripts/check-static-assets.sh`.

## Linked ADR

ADR: docs/adr/ADR-2026-07-31-listas-de-adr-e-req-no-dashboard-derivadas-de-api-chain.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md
