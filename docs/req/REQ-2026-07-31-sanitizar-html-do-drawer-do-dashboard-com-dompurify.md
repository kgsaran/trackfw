---
status: Open
date: 2026-07-31
author: "Zeus"
adr: "docs/adr/ADR-2026-07-31-sanitizacao-de-html-no-drawer-do-dashboard-com-dompurify.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify.md"
---

# REQ: Sanitizar HTML do drawer do dashboard com DOMPurify

> Date: 2026-07-31 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

**XSS armazenado no drawer do `trackfw serve`.** Achado por Hades na revisão de segurança de
barreira do commit `007ebab` em 2026-07-31, classificado como **não bloqueante para aquela
entrega por ser pré-existente** — mas é uma vulnerabilidade real e aberta.

`internal/serve/static/app.js:919` renderiza o corpo do markdown sem sanitização:

```js
mdEl.innerHTML = marked.parse(body || raw);
```

`marked` **não sanitiza HTML por padrão** (a opção `sanitize` foi removida na v5; o projeto usa
`marked@12.0.0`, carregado por CDN em `index.html:12`). Markdown permite HTML inline, então
qualquer tag no corpo do arquivo é injetada e executada.

### Vetor de exploração concreto

1. Um contribuidor abre PR num projeto que usa trackfw, incluindo uma ADR/REQ/roadmap cujo corpo
   contém `<img src=x onerror="...">` ou `<script>`.
2. O mantenedor roda `trackfw serve` localmente para revisar o board — operação rotineira e
   aparentemente inócua.
3. Ele clica no card do artefato. O payload executa no contexto do dashboard, em `localhost`,
   com acesso a `fetch` para os endpoints locais (`/api/file` lê qualquer arquivo dentro de
   `ADRDirs`/`REQDir`/`RoadmapDir`) e à rede da máquina.

O conteúdo é atacante-controlado e o gatilho é uma ação normal de revisão. Não requer
autenticação nem interação incomum.

### Escopo do problema

O sink é alcançável por **três** caminhos hoje, todos convergindo em `openDrawer()`:
cards do Board, nós do grafo da view Chain (`app.js:514`) e as linhas das listas ADRs/REQs
(introduzidas em `007ebab`). Corrigir no `openDrawer` cobre os três de uma vez.

Nota de vault com o diagnóstico:
`vault/notes/security-drawer-marked-parse-unsanitized-stored-xss-2026-07-31.md`

## Acceptance Criteria

- [ ] **AC1** — A saída de `marked.parse()` é sanitizada antes de chegar a `innerHTML` em
      `internal/serve/static/app.js`. Nenhum `innerHTML` no arquivo recebe HTML derivado de
      conteúdo de arquivo sem passar pelo sanitizador.
- [ ] **AC2** — A biblioteca de sanitização é carregada de forma consistente com as demais
      dependências do dashboard (CDN em `index.html`, como `marked`, `chart.js` e `d3` já são),
      com **versão fixada** (sem `@latest`) e, se viável, `integrity` + `crossorigin`.
- [ ] **AC3** — Se o sanitizador não carregar (CDN indisponível, offline), o drawer **não**
      renderiza HTML não sanitizado. O fail-safe é degradar para texto puro ou exibir erro —
      nunca cair no caminho inseguro silenciosamente.
- [ ] **AC4** — Teste de falsificação (padrão `scripts/check-gates-falsify.sh`) que prova o gate
      não vacuoso: um artefato de fixture com payload conhecido no corpo deve resultar em DOM
      **sem** o vetor executável após a renderização; e, ao remover a sanitização, o teste deve
      **falhar**. Provar o efeito, não apenas a presença da chamada.
- [ ] **AC5** — Markdown legítimo continua renderizando: headings, listas ordenadas e não
      ordenadas, blockquote, `code` inline, blocos de código, tabelas e links. Verificado
      visualmente em navegador real contra uma ADR existente do repositório.
- [ ] **AC6** — Links continuam funcionando, inclusive o handler de link interno já existente
      em `app.js` (por volta da linha 757) que intercepta cliques e chama `openDrawer(href)`.
      Confirmar que a sanitização não remove os atributos de que esse handler depende.
- [ ] **AC7** — Paridade dos 3 CLIs: `internal/serve/static/` é a fonte canônica;
      `npm/src/serve/static/` e `pypi/trackfw/serve/static/` são espelhos byte-a-byte.
      `scripts/check-static-assets.sh` imprime `Static assets are synchronized`.
- [ ] **AC8** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.
- [ ] **AC9** — Verificação visual em navegador real: drawer abre corretamente a partir dos três
      caminhos (card do Board, nó do grafo Chain, linha das listas ADRs/REQs), console sem erros
      de JavaScript.

## Negative Scope (fora do escopo — NÃO fazer)

- Não alterar código de servidor (`.go`, `.js` de servidor, `.py`) — a correção é frontend.
  Em particular, **não** mexer em `api_file.go` nem no whitelist anti-path-traversal, que está
  correto e não é a causa.
- Não trocar `marked` por outra biblioteca de markdown — o problema é a ausência de
  sanitização, não a escolha do parser.
- Não adicionar bundler, framework ou pipeline de build de frontend. O dashboard carrega
  dependências por CDN e isso permanece.
- Não criar arquivo novo em `internal/serve/static/` — `scripts/check-static-assets.sh` deriva
  a lista de arquivos do canônico e falha nas duas direções.
- Não alterar as views Board, Chain, Metrics, ADRs e REQs nem o comportamento de navegação.
- Não mexer em `pypi/build/lib/` — artefato de build.
- Não introduzir `@media (prefers-color-scheme)` — o dashboard é light-only por design
  (ver `vault/notes/dashboard-serve-e-light-only-2026-07-31.md`).

## Notas de implementação

Os assets do Go são `go:embed` (`internal/serve/serve.go:12`) — exige `make build` para ver a
mudança. npm e pypi servem do disco. Um `trackfw serve` órfão na porta serve o binário antigo e
mascara alterações: matar com `lsof -ti :<porta> | xargs kill -9` antes de testar.

Como os assets estáticos são compartilhados, a paridade dos 3 CLIs se resolve por espelhamento
mecânico — mesma forma do ciclo das abas ADRs/REQs.

**Decisão já tomada no ADR pareado:** DOMPurify via CDN, com versão fixada. A alternativa de um
renderer restritivo do próprio `marked` foi rejeitada por constituir controle de segurança
artesanal, historicamente inferior a um sanitizador dedicado. Ver a seção *Alternatives Considered*
do ADR para o racional completo.

## Linked ADR

ADR: docs/adr/ADR-2026-07-31-sanitizacao-de-html-no-drawer-do-dashboard-com-dompurify.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify.md
