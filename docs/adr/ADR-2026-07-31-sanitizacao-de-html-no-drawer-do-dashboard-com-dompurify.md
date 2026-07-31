---
status: Accepted
date: 2026-07-31
author: "Zeus"
---

# ADR: Sanitizacao de HTML no drawer do dashboard com DOMPurify

> Date: 2026-07-31 | Status: Accepted

## Context

O drawer do `trackfw serve` renderiza o corpo de ADRs, REQs e roadmaps com
`mdEl.innerHTML = marked.parse(body || raw)` (`internal/serve/static/app.js:919`), sem qualquer
sanitização. `marked@12.0.0` não sanitiza por padrão — a opção `sanitize` foi removida na v5 — e
markdown admite HTML inline. Resultado: **XSS armazenado**.

O vetor é realista: um contribuidor abre PR com uma ADR contendo `<img src=x onerror=...>`; o
mantenedor roda `trackfw serve` para revisar o board e clica no card. O payload executa em
`localhost`, com `fetch` disponível para `/api/file`, que lê qualquer arquivo dentro de
`ADRDirs`/`REQDir`/`RoadmapDir`.

Achado por Hades em revisão de barreira (2026-07-31). Confirmado **pré-existente** ao commit
`007ebab` — verificado em `007ebab~1`, já alcançável pelo grafo da view Chain.

A questão a decidir: **como neutralizar o HTML sem quebrar o markdown legítimo**.

## Decision

**Sanitizar a saída de `marked.parse()` com DOMPurify antes de atribuir a `innerHTML`.**

1. DOMPurify é carregado por **CDN em `index.html`**, no mesmo padrão de `marked`, `chart.js` e
   `d3`, com **versão fixada** (nunca `@latest`) e `integrity` + `crossorigin`.
   Valores definidos em 2026-07-31: **3.4.12**,
   `https://cdn.jsdelivr.net/npm/dompurify@3.4.12/dist/purify.min.js`,
   SRI `sha384-piCcpDdJ7qVeK4Tv8Z6Hpcr3ZBIgP16TxQTPVfsLFdZ5uDgwc3Y8Ho7oUnqf12qu`.
   O `integrity` é aplicado **somente** à tag do DOMPurify — nenhuma das seis tags CDN atuais o
   possui, e estendê-lo às demais é REQ própria. Inconsistente, porém estritamente melhor, e
   trata-se justamente da tag de um controle de segurança.
2. A sanitização acontece em **um único ponto**, dentro de `openDrawer()`. Os três caminhos de
   acesso ao drawer — card do Board, nó do grafo Chain, linha das listas ADRs/REQs — convergem
   ali, então uma correção cobre todos.
3. **Fail-safe explícito:** se DOMPurify não estiver disponível (CDN fora do ar, uso offline), o
   drawer **não** renderiza HTML bruto. Degrada para texto puro ou exibe erro. É proibido cair no
   caminho inseguro por ausência da dependência.
4. **A prova do AC4 é seam de navegador em auditoria, não gate de CI.** `npm/package.json` não
   tem nenhuma devDependency e não há infra de DOM; introduzir jsdom mudaria uma propriedade do
   projeto. O seam prova o **efeito** — payload inerte com sanitização, e executando ao removê-la
   — o que um gate de grep não faz. Trade-off aceito: não há barreira automática contra regressão
   futura em CI.
5. A correção é **frontend puro**. `internal/serve/static/` é canônico; npm e pypi são espelhos
   byte-a-byte. Nenhum código de servidor muda.

## Consequences

**Positivas**

- Elimina a classe inteira de XSS no único sink perigoso do dashboard, cobrindo os três caminhos
  de navegação de uma vez.
- DOMPurify é o sanitizador de referência da indústria, auditado e mantido — muito superior a
  qualquer allowlist artesanal que escreveríamos aqui.
- Custo de implementação baixo e simétrico ao ciclo anterior: um ML canônico mais espelhamento.

**Negativas / aceitas**

- **Mais uma dependência de terceiro carregada em runtime por CDN.** Isso é risco de cadeia de
  suprimentos: um CDN comprometido serve script arbitrário. Mitigação: versão fixada e `integrity`.
  Aceito porque o dashboard já depende de CDNs (Tailwind, htmx, marked, chart.js, d3) — a decisão
  de distribuição já foi tomada e não é escopo desta REQ revertê-la.
- O dashboard passa a não renderizar o drawer plenamente offline. Consequência direta do fail-safe
  da decisão 3, e preferível ao inverso.
- A allowlist do DOMPurify precisa preservar os atributos de que depende o handler de link interno
  já existente em `app.js` (~linha 757), que intercepta cliques e chama `openDrawer(href)`.
  Requer verificação explícita.

## Alternatives Considered

**Renderer restritivo do próprio `marked`, descartando HTML bruto** — sem dependência nova e sem
risco adicional de CDN. **Rejeitado:** seria um controle de segurança artesanal. A cobertura de um
renderer configurado à mão é comprovadamente inferior à de um sanitizador dedicado, e o histórico
de bypasses em filtros caseiros é longo. Trocar uma dependência bem auditada por código de
segurança próprio é má economia.

**Sanitizar no servidor, antes de entregar o conteúdo em `/api/file`** — centralizaria a defesa e
funcionaria offline. **Rejeitado:** exigiria implementar sanitização de HTML em Go, Node e Python
com paridade byte-a-byte de comportamento — três implementações de um controle de segurança, três
superfícies de divergência. Além disso `/api/file` serve texto bruto por contrato, e alterá-lo
quebraria a semântica do endpoint.

**Não corrigir, por ser dashboard local** — **Rejeitado:** o conteúdo é atacante-controlado via PR
e o gatilho é uma ação rotineira de revisão. "Roda em localhost" não é fronteira de confiança
quando o input vem de terceiros.

**Trocar `marked` por um parser que sanitize por padrão** — **Rejeitado:** o defeito é a ausência
de sanitização, não a escolha do parser. Trocar o parser é mudança maior com risco de regressão em
todo o markdown já renderizado, sem ganho proporcional.
