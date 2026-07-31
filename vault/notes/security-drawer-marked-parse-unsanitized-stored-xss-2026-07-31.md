# Security: drawer usa `marked.parse()` sem sanitização — stored XSS pré-existente

**Data:** 2026-07-31
**Descoberto por:** Hades (revisão de segurança da Wave 1, commit `007ebab`, abas ADRs/REQs)
**Status:** achado real, **não bloqueante** para o commit auditado — pré-existente e já reportado
separadamente para tratamento em REQ própria.

## O que é

Em `internal/serve/static/app.js`, `openDrawer(path)` busca `GET /api/file?path=` e renderiza o corpo
markdown assim:

```js
mdEl.innerHTML = marked.parse(body || raw);
```

Não há `DOMPurify` nem qualquer sanitizador no bundle (`grep -rn "DOMPurify\|sanitiz"
internal/serve/static/` não retorna nada). O `marked@12.0.0` carregado via CDN (`index.html`) **não
sanitiza HTML embutido no markdown por padrão** — a opção `sanitize` foi removida do marked a partir da
v0.7 (deprecada) e não existe mais na v12. Qualquer HTML bruto no corpo do arquivo `.md` (ex.: `<img
src=x onerror=alert(1)>` ou `<script>`) é injetado literalmente no DOM via `innerHTML`.

## Por que importa (vetor real)

Um contribuidor pode abrir PR com um ADR/REQ/Roadmap cujo **corpo** (não só o frontmatter) contenha HTML
malicioso. Se o mantenedor rodar `trackfw serve` localmente e abrir esse artefato no drawer — via Chain
(grafo D3, clique no nó) ou via as novas abas de lista — o payload executa no contexto da página
`localhost`.

## Por que não bloqueia o commit 007ebab (abas ADRs/REQs)

Confirmado via diff reverso (`git show 007ebab~1:internal/serve/static/app.js`): o `openDrawer(d.id)` já
era chamado no clique de qualquer nó do grafo D3, **incluindo nós `type === 'adr'` e `type === 'req'`**
(`NODE_COLORS.adr`, `NODE_COLORS.req`, label `'ADR'`/`'REQ'` já existiam). Ou seja, o sink
`marked.parse()` sem sanitização já era alcançável para ADRs e REQs **antes** desta feature — a nova
lista apenas oferece um segundo caminho de navegação até o mesmo sink pré-existente. Não há
alargamento de superfície de ataque atribuível a este commit.

## Recomendação

Abrir REQ/roadmap próprio (fora do escopo desta feature) para:
- Adicionar `DOMPurify.sanitize()` (ou equivalente) entre `marked.parse()` e a atribuição a `innerHTML`
  em `openDrawer` (`internal/serve/static/app.js` ~linha 919), **nos três CLIs** (Go/Node/Python
  servem os mesmos assets estáticos — verificar se há cópia em `npm/`/`pypi/`).
- Ou configurar `marked` com um renderer que não passe HTML bruto (`marked.use({ renderer: ..., 
  headerIds: false })` não resolve sozinho — DOMPurify pós-render é a solução padrão).

## Como não perder isso de novo

Antes de investigar o drawer/markdown rendering, ler esta nota — evita re-provar reachability do sink
via `git show <commit>~1` toda vez.
