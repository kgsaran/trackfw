---
status: Done
date: 2026-08-01
author: "Zeus"
adr: "docs/adr/ADR-2026-08-01-sri-nas-dependencias-cdn-versionadas-e-remocao-do-htmx-nao-utilizado.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-01-proteger-dependencias-cdn-do-dashboard-com-sri-e-remover-htmx-morto.md"
---

# REQ: Proteger dependencias CDN do dashboard com SRI e remover htmx morto

> Date: 2026-08-01 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O DOMPurify ganhou `integrity` no PR #95; as cinco dependências CDN anteriores do dashboard
seguem sem. Estender o SRI ficou registrado como follow-up e é o último item da fila aberta desde
o ciclo das abas ADRs/REQs.

Ao levantar as cinco, dois fatos mudaram o escopo:

1. **htmx tem zero usos** — varredura em `internal/serve/`, `npm/src/serve/` e
   `pypi/trackfw/serve/`: nenhum atributo `hx-*`, nenhuma referência no `app.js`. É dependência
   morta baixada em toda visita.
2. **O Tailwind não pode receber SRI com segurança** — URL não-versionada
   (`cdn.tailwindcss.com`, `HTTP/2 302`, `cache-control: max-age=14400`). Hash fixo quebraria o
   dashboard inteiro no próximo release deles, silenciosamente.

## Acceptance Criteria

- [x] **AC1** — A tag do **htmx é removida** de `internal/serve/static/index.html`. Não recebe SRI.
- [x] **AC2** — `marked@12.0.0`, `chart.js@4.4.4` e `d3@7.9.0` recebem `integrity`,
      `crossorigin="anonymous"` e `referrerpolicy="no-referrer"`, no padrão já usado pelo DOMPurify.
      Hashes exatos (conferidos em dois downloads independentes cada):
      - marked: `sha384-NNQgBjjuhtXzPmmy4gurS5X7P4uTt1DThyevz4Ua0IVK5+kazYQI1W27JHjbbxQz`
      - chart.js: `sha384-NrKB+u6Ts6AtkIhwPixiKTzgSKNblyhlk0Sohlgar9UHUBzai/sgnNNWWd291xqt`
      - d3: `sha384-CjloA8y00+1SDAUkjs099PVfnY2KmDC2BZnws9kh8D/lX1s46w6EPhpXdqMfjK6i`
- [x] **AC3** — O **Tailwind permanece sem SRI**, com **comentário no próprio `index.html`**
      explicando que a URL é não-versionada e que um hash fixo quebraria o dashboard. Isso impede
      que alguém "uniformize" a inconsistência sem entender o motivo.
- [x] **AC4** — O dashboard funciona integralmente após a mudança: as cinco abas
      (Board, Chain, Metrics, ADRs, REQs) renderizam, o grafo D3 desenha, os gráficos Chart.js
      desenham, o drawer renderiza markdown (marked) sanitizado (DOMPurify) e o Tailwind estiliza.
      Verificado em **navegador real**.
- [x] **AC5** — **SRI provado ativo** em 2 das 3 tags (d3 e chart.js). Chrome recusou o recurso
      com `The resource has been blocked`; `typeof d3 === "undefined"` e grafo com 0 elementos;
      `typeof Chart === "undefined"` e `charts-container` sem renderizar. Restaurado byte-a-byte
      e reconfirmado. **Confirmação cruzada não planejada:** o hash que o próprio Chrome computou
      nas mensagens de erro bate com os valores aplicados — verificação independente vinda do
      navegador.
- [x] **AC6** — Console sem erros de integridade com os hashes corretos.
- [x] **AC7** — Nenhuma referência remanescente a htmx em `internal/`, `npm/` ou `pypi/`.
- [x] **AC8** — npm e pypi byte-a-byte idênticos ao canônico;
      `scripts/check-static-assets.sh` imprime `Static assets are synchronized`.
- [x] **AC9** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não adicionar SRI ao Tailwind.** Decisão explícita do usuário — quebraria o dashboard no
  próximo release deles.
- **Não trocar a Play CDN do Tailwind por URL versionada.** É compilador JIT em runtime; migrar
  exige auditoria visual completa e traz risco de regressão de layout. REQ própria, se for o caso.
- **Não baixar dependências para servir localmente** via `go:embed` — contraria a decisão de
  distribuição vigente e mereceria ADR próprio.
- Não readicionar htmx nem introduzir uso novo dele.
- Não atualizar versões de nenhuma biblioteca — este ciclo é sobre integridade, não sobre bump.
- Não alterar `app.js` nem `style.css` (a mudança é só de `index.html`).
- Não alterar código de servidor (`.go`, `.js` de servidor, `.py`), nem o whitelist de `/api/file`.
- Não criar arquivo novo em `internal/serve/static/`.
- Não mexer em `pypi/build/lib/`.

## Notas de implementação

Estado verificado em 2026-08-01, `internal/serve/static/index.html`: linha 8 Tailwind (não
versionada), 10 htmx, 12 marked, 14 chart.js, 16 d3, 320 DOMPurify (já com SRI — usar como
modelo de formatação).

Mudança **frontend pura** e restrita a um arquivo. `internal/serve/static/` é canônico; npm e
pypi são espelhos byte-a-byte — a paridade se resolve por cópia mecânica.

Assets do Go são `go:embed`: `make build` obrigatório. Um `trackfw serve` órfão na porta serve o
binário antigo e mascara alterações (`lsof -ti :<porta> | xargs kill -9`).

**Armadilhas de instrumentação** (`vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`):
`_drawerPath` não é propriedade de `window`; `closeDrawer()` não readiciona a classe `hidden` —
verificar visibilidade por `getComputedStyle(el).display`.

## Linked ADR

ADR: docs/adr/ADR-2026-08-01-sri-nas-dependencias-cdn-versionadas-e-remocao-do-htmx-nao-utilizado.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-01-proteger-dependencias-cdn-do-dashboard-com-sri-e-remover-htmx-morto.md
