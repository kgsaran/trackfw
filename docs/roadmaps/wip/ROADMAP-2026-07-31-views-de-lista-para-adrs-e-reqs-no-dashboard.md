---
status: wip
date: 2026-07-31
req: "docs/req/REQ-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md"
squad: ""
---

# Roadmap: Views de lista para ADRs e REQs no dashboard

> Created: 2026-07-31 | Status: wip

## Context

REQ: docs/req/REQ-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md
ADR: docs/adr/ADR-2026-07-31-listas-de-adr-e-req-no-dashboard-derivadas-de-api-chain.md

ADRs e REQs só são alcançáveis pelo grafo da aba Chain (137 nós, ilegível para busca
dirigida). O backend já existe e está provado: `/api/chain` devolve `{id, type, title,
state}` com `id` = caminho relativo, e `openDrawer(id)` → `/api/file?path=` responde 200.
A entrega é **frontend puro**.

`scripts/check-static-assets.sh` define `internal/serve/static/` como **fonte canônica** e
exige espelho byte-a-byte em `npm/src/serve/static/` e `pypi/trackfw/serve/static/`.
Hoje os três diretórios têm md5 idêntico nos três arquivos.

### Dependências e paralelismo

Wave 1 e Wave 2 são **estritamente sequenciais**: a Wave 2 espelha o produto da Wave 1.
Não há paralelismo possível — há um único arquivo canônico por asset. Cada wave tem um
único ML pelo mesmo motivo: `app.js`, `index.html` e `style.css` são coeditados pela mesma
feature e dividir geraria conflito no mesmo arquivo.

## Critérios de Aceite

Consolidados da REQ (AC1–AC12). Detalhamento por microlote nas waves abaixo.

- [ ] Cinco abas na navegação: Board, Chain, Metrics, ADRs, REQs
- [ ] Aba ADRs lista 12 itens; aba REQs lista 58, com filtro em "Todos"
- [ ] Filtro de status derivado dinamicamente da resposta de `/api/chain`
- [ ] Busca textual por título e identificador, case- e acento-insensitive
- [ ] Clique/Enter numa linha abre o drawer existente via `openDrawer(node.id)`
- [ ] Acessibilidade: `aria-pressed` nas abas, linhas focáveis, `aria-label` nas sections
- [ ] Estados vazio e de erro tratados
- [ ] Nenhum arquivo novo em `internal/serve/static/`; nenhum endpoint novo
- [ ] npm e pypi byte-a-byte idênticos ao canônico
- [ ] `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes

---

## Wave 1 — Implementação canônica (1 ML)
> Dependências: nenhuma

### ML-1A — Abas ADRs e REQs em `internal/serve/static/`
**Status:** pending
**Agente:** Afrodite
**Arquivos afetados (exclusivamente estes três):**
- `internal/serve/static/index.html`
- `internal/serve/static/app.js`
- `internal/serve/static/style.css`

**Contexto técnico verificado:**
- `index.html:24-43` — `<nav>` com os três botões existentes (`tab-board`, `tab-chain`, `tab-metrics`), classe `.tab-btn`, atributo `aria-pressed`.
- `index.html:60`, `:121`, `:134` — `<section id="view-board|view-chain|view-metrics" class="view-section">`.
- `app.js:73-101` — `switchView(view)`: esconde `.view-section`, marca `.tab-btn` ativa, mostra `el('view-' + view)` e carrega dados por tipo de view.
- `app.js:11` — `let _chainData = null;` cache de `/api/chain`.
- `app.js:424` — `const res = await fetch('/api/chain');`
- `app.js:260-261` — padrão de linha clicável: `div.addEventListener('click', () => openDrawer(card.path))` e `keydown` para Enter/Space.
- `app.js:702` — `async function openDrawer(path)`.
- Formato de nó: `{"id":"docs/adr/ADR-002-....md","type":"adr","title":"...","state":"Accepted"}`.

**Ações:**
1. Em `index.html`, acrescentar dois botões `<button id="tab-adr">ADRs</button>` e
   `<button id="tab-req">REQs</button>` ao `<nav>`, replicando classes e atributos
   (`class="tab-btn ..."`, `aria-pressed="false"`, `onclick="switchView('adr')"` /
   `switchView('req')`) exatamente no padrão dos três existentes.
2. Em `index.html`, acrescentar `<section id="view-adr" class="view-section w-full hidden p-6 overflow-auto" aria-label="Lista de ADRs">` e
   `<section id="view-req" ... aria-label="Lista de REQs">`, cada uma contendo:
   campo de busca, `<select>` de filtro de status, contador de itens, container da lista,
   bloco de estado vazio e bloco de erro com `role="alert"` — espelhando a estrutura das
   views existentes.
3. Em `app.js`, estender `switchView()` com os ramos `'adr'` e `'req'`.
4. Em `app.js`, implementar o carregamento reusando o cache: se `_chainData` for `null`,
   buscar `/api/chain` (mesma função já usada pela aba Chain); filtrar por
   `node.type === 'adr'` / `'req'`.
5. Popular o `<select>` de status **a partir dos valores distintos presentes na resposta**,
   ordenados, mais a opção "Todos" como default. **Nunca hardcodar** `Accepted`/`Proposed`/`Done`.
6. Implementar busca textual sobre `title` e `id`, case-insensitive e acento-insensitive
   (normalizar com `String.prototype.normalize('NFD').replace(/\p{Diacritic}/gu,'')`).
7. Renderizar cada linha com identificador (derivado do basename de `id`), título e chip de
   status colorido; `tabindex="0"`, `role="button"`, rótulo acessível; `click` e
   `keydown` (Enter/Space) chamando `openDrawer(node.id)`.
8. Estilos novos **dentro de `style.css`** — não criar arquivo novo.
9. Estado vazio e estado de erro tratados conforme AC8 da REQ.

**Proibições (escopo negativo — falha de auditoria se violado):**
- Não criar nenhum arquivo novo em `internal/serve/static/`.
- Não tocar em nenhum `.go`, `.js` de servidor, `.py`, nem em `npm/` ou `pypi/`.
- Não alterar `api_chain.go` / `api_file.go` nem qualquer handler.
- Não alterar as views Board, Chain e Metrics nem o comportamento do drawer.
- Não adicionar dependência, framework ou bundler.

**Critérios de aceite:**
- [ ] `make build` passa (assets são `go:embed`; o rebuild é obrigatório para ver a mudança)
- [ ] `make test` verde
- [ ] `make lint` sem erro
- [ ] `git status --porcelain` mostra **exatamente** os três arquivos de `internal/serve/static/`
- [ ] Com `bin/trackfw serve`: aba ADRs lista **12** itens e aba REQs lista **58** com filtro "Todos"
- [ ] O ADR de status `unknown` aparece na lista com filtro "Todos"
- [ ] Clicar numa linha abre o drawer com markdown + frontmatter renderizados
- [ ] Navegação por teclado funciona nas abas e nas linhas

**Comandos de validação:**
```bash
make build && make test && make lint
git status --porcelain
bin/trackfw serve --port 8791 &
sleep 3
curl -s localhost:8791/api/chain | python3 -c "import json,sys;from collections import Counter;print(Counter(n['type'] for n in json.load(sys.stdin)['nodes']))"
# esperado: Counter({'roadmap': 67, 'req': 58, 'adr': 12})
```

---

## Wave 2 — Espelhamento e paridade (1 ML)
> Dependências: **Wave 1 completa e auditada**

### ML-2A — Espelhar assets para npm e pypi
**Status:** pending
**Agente:** Hefesto
**Arquivos afetados:**
- `npm/src/serve/static/{app.js,index.html,style.css}`
- `pypi/trackfw/serve/static/{app.js,index.html,style.css}`

**Ações:**
1. Copiar os **três** arquivos de `internal/serve/static/` para os dois destinos —
   inclusive `style.css` mesmo que pareça inalterado, pois `check-static-assets.sh`
   compara byte-a-byte toda a lista derivada do canônico:
   ```bash
   cp internal/serve/static/app.js internal/serve/static/index.html internal/serve/static/style.css npm/src/serve/static/
   cp internal/serve/static/app.js internal/serve/static/index.html internal/serve/static/style.css pypi/trackfw/serve/static/
   ```
2. Validar a paridade e conferir o funcionamento nos runtimes Node e Python.

**Proibições:**
- Não editar o conteúdo dos arquivos — é cópia mecânica; qualquer divergência é bug do ML-1A.
- Não tocar em `pypi/build/lib/` (artefato de build).
- Não alterar `scripts/check-static-assets.sh`.
- Não tocar em `internal/serve/static/`.

**Critérios de aceite:**
- [ ] `scripts/check-static-assets.sh` imprime `Static assets are synchronized`
- [ ] `make parity` passa por inteiro
- [ ] `make quality` passa (`test` + `test-node` + `test-python` + `lint` + `parity`)
- [ ] md5 idêntico nos três diretórios para os três arquivos
- [ ] `node npm/bin/trackfw.js serve` exibe as cinco abas com 12 ADRs e 58 REQs
- [ ] `python -m trackfw serve` exibe as cinco abas com 12 ADRs e 58 REQs

**Comandos de validação:**
```bash
scripts/check-static-assets.sh
md5 -q internal/serve/static/app.js npm/src/serve/static/app.js pypi/trackfw/serve/static/app.js
md5 -q internal/serve/static/index.html npm/src/serve/static/index.html pypi/trackfw/serve/static/index.html
md5 -q internal/serve/static/style.css npm/src/serve/static/style.css pypi/trackfw/serve/static/style.css
make quality
```
