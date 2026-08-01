---
status: Done
date: 2026-08-01
author: "Zeus"
adr: "docs/adr/ADR-2026-08-01-resolucao-de-links-markdown-relativos-ao-documento-aberto-no-drawer.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-01-corrigir-403-em-links-markdown-relativos-dentro-do-drawer.md"
---

# REQ: Corrigir 403 em links markdown relativos dentro do drawer

> Date: 2026-08-01 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Clicar num link `.md` relativo dentro do drawer do `trackfw serve` falha. O interceptador
(`internal/serve/static/app.js:966-977`) passa o **href bruto** para `openDrawer`, e o servidor
rejeita. Reproduzido em 2026-08-01:

```
?path=../roadmaps/done/v2.3-validator-improvements-2026-06-13.md   → 403 Forbidden
?path=docs/roadmaps/done/v2.3-validator-improvements-2026-06-13.md → 200
```

Levantamento de **todas** as formas de link `.md` em `docs/`: `./X.md` (13), `X.md` nu (3),
`../vault/notes/X.md` (3), `../../../requisições/claude/X.md` (5), `../roadmaps/done/X.md` (1),
`../../req/X.md` (1). **Nenhuma** é relativa à raiz — todas são relativas ao documento, o que
torna a regra de resolução inequívoca.

Achado por Ártemis durante o seam do XSS, reportado sem correção por estar fora daquele escopo
(`vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`).

## Acceptance Criteria

- [x] **AC1** — O href relativo é resolvido contra `dirname` do documento aberto (`_drawerPath`),
      normalizando `.` e `..`, **antes** de chamar `openDrawer`.
- [x] **AC2** — As três formas relativas funcionam: `./X.md`, `X.md` (nu) e `../dir/X.md`.
      Verificado em navegador real abrindo o link e confirmando que o drawer troca de conteúdo.
- [x] **AC3** — Caso concreto do repositório: em `docs/req/REQ-2026-06-13-validator-improvements.md`,
      o link `../roadmaps/done/v2.3-validator-improvements-2026-06-13.md` abre o roadmap.
- [x] **AC4** — **Navegação encadeada:** verificado com A em profundidade 2 (`docs/req/`) e B em
      profundidade 3 (`docs/adr/tmp_qa_b/`), de modo que uma base congelada produziria caminho
      **diferente e errado** — o que torna o teste discriminante, não vacuoso. `_drawerPath`
      conferido após **cada** salto.
- [x] **AC5** — Link que resolva para **fora** dos diretórios permitidos exibe mensagem
      explicativa no drawer, informando o caminho resolvido e que está fora dos diretórios
      permitidos. Nada de `Forbidden` cru nem `HTTP 403` sem contexto.
- [x] **AC6** — Os 3 links `../vault/notes/*.md` caem no AC5 — **continuam não abrindo**, agora
      com mensagem clara. O whitelist **não** é alterado.
- [x] **AC7** — Links externos (`http://`, `https://`) seguem **não** interceptados, com o
      comportamento atual de navegação preservado.
- [x] **AC8** — Âncoras (`#secao`) e links não-`.md` seguem não interceptados.
- [x] **AC9** — O primeiro documento aberto (via card do Board, nó do grafo Chain ou linha das
      listas ADRs/REQs) continua funcionando — a resolução não pode quebrar o caminho de entrada,
      onde o path já vem completo.
- [x] **AC10** — npm e pypi byte-a-byte idênticos ao canônico;
      `scripts/check-static-assets.sh` imprime `Static assets are synchronized`.
- [x] **AC11** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não alterar o whitelist de `/api/file`.** `vault/` **não** entra. Decisão explícita do
  usuário: manter a superfície de leitura do servidor inalterada.
- **Não alterar nenhum código de servidor** (`.go`, `.js` de servidor, `.py`). Em especial não
  tocar em `api_file.*` nem na checagem anti-path-traversal, que está correta e não é a causa.
- **Não reescrever os links dos documentos** em `docs/`. A correção é no mecanismo, não nos dados.
  Em particular, não mexer nos 5 links `../../../requisições/claude/*.md` (legado de outro
  projeto, apontam para fora do repositório).
- Não alterar a sanitização com DOMPurify nem a allowlist (entregue no PR #95).
- Não criar arquivo novo em `internal/serve/static/` —
  `scripts/check-static-assets.sh` deriva a lista do canônico e falha nas duas direções.
- Não adicionar dependência, framework ou bundler.
- Não introduzir `@media (prefers-color-scheme)` — o dashboard é light-only por design.
- Não mexer em `pypi/build/lib/`.

## Notas de implementação

Pontos verificados em 2026-08-01:

- `internal/serve/static/app.js:966-977` — interceptador; passa `href` bruto a `openDrawer`.
- `app.js` ~930 — `hide('drawer-error')` no topo de `openDrawer`.
- `app.js` ~984 — bloco de erro do `catch`, onde hoje cai `HTTP 403`.
- `_drawerPath` é setado no início de `openDrawer` e é o documento atualmente aberto.
- O corpo do 403 é literalmente `Forbidden`.

Correção **frontend puro**: `internal/serve/static/` é canônico, npm e pypi são espelhos
byte-a-byte — a paridade dos 3 CLIs se resolve por cópia mecânica, mesma forma do ciclo do
DOMPurify.

Os assets do Go são `go:embed` — exige `make build`. Um `trackfw serve` órfão na porta serve o
binário antigo e mascara alterações: matar com `lsof -ti :<porta> | xargs kill -9` antes de testar.

**Armadilha de instrumentação conhecida** (registrada em
`vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`): `_drawerPath` é declarado
com `let` no topo do arquivo, então **não** é propriedade de `window` — use o identificador puro
em `Runtime.evaluate`. E `closeDrawer()` não readiciona a classe `hidden`: verificar visibilidade
por `getComputedStyle(el).display`, não por `classList`.

## Linked ADR

ADR: docs/adr/ADR-2026-08-01-resolucao-de-links-markdown-relativos-ao-documento-aberto-no-drawer.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-01-corrigir-403-em-links-markdown-relativos-dentro-do-drawer.md
