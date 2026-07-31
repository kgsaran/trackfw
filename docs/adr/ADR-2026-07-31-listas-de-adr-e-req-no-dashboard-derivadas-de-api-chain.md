---
status: Accepted
date: 2026-07-31
author: "Zeus"
---

# ADR: Listas de ADR e REQ no dashboard derivadas de /api/chain

> Date: 2026-07-31 | Status: Accepted

## Context

O dashboard do `trackfw serve` expõe hoje três views: **Board** (kanban de roadmaps),
**Chain** (grafo da cadeia ADR→REQ→ROADMAP) e **Metrics**.

ADRs e REQs **já são visualizáveis**, mas apenas como nós do grafo da aba Chain.
Verificação empírica em 2026-07-31, com o servidor rodando neste repositório:

- `GET /api/chain` retorna `{roadmap: 67, req: 58, adr: 12}` — 137 nós e 118 arestas.
- O campo `id` de cada nó **é o caminho relativo do arquivo**
  (ex.: `docs/adr/ADR-002-estrategia-discovery-e-distribuicao.md`).
- `app.js:514` faz `.on('click', (_, d) => openDrawer(d.id))`, e `openDrawer(path)`
  busca `GET /api/file?path=<path>`, que respondeu **200** para uma ADR.
- `api_file.go` já autoriza `ADRDirs`, `REQDir` e `RoadmapDir` no whitelist anti-traversal.

Ou seja: **o backend necessário já existe e funciona nos três CLIs**. O problema é de
navegação — 137 nós num grafo force-directed é ilegível. Não há como *procurar* uma ADR,
apenas tropeçar nela. O usuário pediu explicitamente a capacidade de "ver as ADRs e as REQs
através do dashboard" como atividade de navegação deliberada, não de exploração de grafo.

A questão material é: **de onde a nova view puxa os dados** — reusar o contrato existente
ou criar um endpoint dedicado.

## Decision

**As listas de ADR e REQ derivam de `/api/chain`. Nenhum endpoint novo é criado.**

Decorrências vinculantes:

1. **Zero mudança de backend.** Não se adiciona `/api/docs` nem qualquer handler novo em
   `internal/serve/`, `npm/src/serve/` ou `pypi/trackfw/serve/`.
2. O front reusa o cache `_chainData` já existente em `app.js`, filtrando por `node.type`.
3. **Duas abas separadas** — `ADRs` e `REQs` — e não uma aba unificada. Motivo: 12 ADRs
   contra 58 REQs; numa lista única os ADRs desaparecem no volume, e os domínios de status
   são disjuntos (`Accepted`/`Proposed` vs `Done`/`Closed`).
4. O clique numa linha chama `openDrawer(node.id)` — **o mesmo drawer já existente**.
   Não se cria um segundo caminho de leitura de arquivo.
5. **O filtro de status é derivado dinamicamente** dos valores presentes na resposta.
   `state` é texto livre lido do frontmatter, não um enum fechado: hoje há `Accepted`,
   `Proposed` e `unknown` nas ADRs. Uma lista hardcoded faria sumir silenciosamente
   qualquer artefato com status inesperado.
6. **A mudança é frontend puro.** `internal/serve/static/` é a fonte canônica segundo
   `scripts/check-static-assets.sh`; npm e pypi são espelhos byte-a-byte. A paridade dos
   3 CLIs é satisfeita por espelhamento mecânico, não por reimplementação.

## Consequences

**Positivas**

- Escopo mínimo: três arquivos canônicos (`app.js`, `index.html`, `style.css`) mais o
  espelhamento. Nenhum contrato de API novo para manter em triplicata.
- Risco de drift de paridade próximo de zero — `check-static-assets.sh` já compara
  byte-a-byte e falha em ambas as direções (arquivo faltando **e** arquivo extra).
- O drawer, o whitelist de segurança e o parsing de frontmatter são reaproveitados sem
  alteração; a superfície de ataque não cresce.

**Negativas / aceitas**

- A linha da lista **não** exibe data de criação, ADR pareada nem contagem de roadmaps
  vinculados — esses metadados não estão em `chainNode` (`{ID, Type, Title, State}`).
  Mitigação: o grafo da aba Chain já mostra os vínculos, e o drawer mostra o frontmatter
  completo. Aceito conscientemente.
- Ordenação e busca ficam limitadas a `title` e `id`. Suficiente porque o `id` é o path,
  que carrega a data no nome do arquivo (`REQ-2026-06-13-...`).
- `/api/chain` varre as três árvores a cada request. Com 137 nós o custo é irrelevante;
  se a base crescer uma ordem de grandeza, revisitar com cache — não antes.
- No Go os assets estáticos são servidos via `go:embed` (`internal/serve/serve.go:12`),
  então qualquer alteração exige `go build` para ter efeito. npm e pypi servem do disco.

## Alternatives Considered

**Endpoint dedicado `/api/docs`** — retornaria `path`, `type`, `title`, `state` mais
`date`, `links` e `roadmaps`. Rejeitado: exigiria handler + testes em Go, Node e Python
(≈12 arquivos contra 3), sujeito à regra dura de paridade, para entregar metadados que o
grafo e o drawer já expõem. O custo de manutenção permanente não se paga pelo ganho
marginal de densidade informacional numa linha de lista.

**Aba única "Docs" com filtro de tipo** — rejeitado pelo desequilíbrio 12 vs 58 e pelos
domínios de status disjuntos, que forçariam um filtro de status genérico e pouco útil.

**Manter apenas o grafo** — rejeitado: é o estado atual, e é justamente o que não escala.
137 nós num force-directed não permitem busca dirigida.
