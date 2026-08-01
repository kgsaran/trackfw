---
status: done
date: 2026-07-31
req: "docs/req/REQ-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify.md"
squad: ""
---

# Roadmap: Sanitizar HTML do drawer do dashboard com DOMPurify

> Created: 2026-07-31 | Status: done

## Context

REQ: docs/req/REQ-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify.md
ADR: docs/adr/ADR-2026-07-31-sanitizacao-de-html-no-drawer-do-dashboard-com-dompurify.md

XSS armazenado: `internal/serve/static/app.js:919` faz
`mdEl.innerHTML = marked.parse(body || raw)` sem sanitização. `marked@12.0.0` não sanitiza por
padrão e markdown admite HTML inline. Uma ADR maliciosa vinda de PR executa script quando o
mantenedor abre o drawer.

O sink é único e os três caminhos de navegação (card do Board, nó do grafo Chain, linha das
listas ADRs/REQs) convergem em `openDrawer()` — uma correção cobre todos.

### Valores exatos verificados em 2026-07-31

- Versão: **DOMPurify 3.4.12** (`latest` na API do jsdelivr nesta data)
- URL: `https://cdn.jsdelivr.net/npm/dompurify@3.4.12/dist/purify.min.js`
- SRI: `sha384-piCcpDdJ7qVeK4Tv8Z6Hpcr3ZBIgP16TxQTPVfsLFdZ5uDgwc3Y8Ho7oUnqf12qu`
  (calculado com `openssl dgst -sha384 -binary | openssl base64 -A`, conferido em dois downloads)
- Global UMD exportado: `DOMPurify`
- Licença: Apache-2.0 / MPL-2.0 (Cure53)

### Decisões tomadas com o usuário

1. **Teste de falsificação do AC4 é seam de navegador em auditoria**, não gate de CI. O projeto
   tem zero devDependency em `npm/package.json` e não há infra de DOM; adicionar jsdom seria mudar
   uma propriedade do projeto. O seam prova o **efeito** (payload inerte → remove sanitização →
   payload executa), o que um gate de grep não faria.
2. **SRI apenas na tag do DOMPurify.** Nenhuma das seis tags CDN atuais tem `integrity`; ficará
   inconsistente, mas é estritamente melhor e trata-se justamente da tag de um controle de
   segurança. SRI nas demais é REQ própria.

### Dependências e paralelismo

Wave 1 → Wave 2 → Wave 3 são **estritamente sequenciais**. A Wave 2 é auditoria do produto da
Wave 1, e a Wave 3 espelha o que a Wave 2 aprovou. Não há paralelismo: arquivo canônico único
por asset.

## Critérios de Aceite

Consolidados da REQ (AC1–AC9). Detalhamento por microlote nas waves abaixo.

- [x] `marked.parse()` sanitizado antes de qualquer `innerHTML`
- [x] DOMPurify por CDN com versão fixada e `integrity` + `crossorigin`
- [x] Fail-safe: sem DOMPurify carregado, o drawer não renderiza HTML bruto
- [x] Seam de navegador prova o efeito (payload inerte; sem sanitização, payload se manifesta)
- [x] Markdown legítimo intacto: headings, listas, blockquote, code, tabelas, links
- [x] Handler de link interno `.md` continua funcionando
- [x] npm e pypi byte-a-byte idênticos ao canônico
- [x] `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes

---

## Wave 1 — Sanitização canônica (1 ML)
> Dependências: nenhuma

### ML-1A — DOMPurify no `openDrawer` de `internal/serve/static/`
**Status:** ✅ concluído (auditado 2026-07-31)
**Agente:** Afrodite
**Arquivos afetados (exclusivamente estes dois):**
- `internal/serve/static/index.html`
- `internal/serve/static/app.js`

**Ações:**
1. Em `index.html`, acrescentar a tag do DOMPurify **antes** de `/static/app.js` (linha ~318),
   junto das demais dependências de CDN, com os valores exatos da seção de contexto e
   `crossorigin="anonymous"` + `referrerpolicy="no-referrer"`.
2. Em `app.js`, criar um helper único de renderização segura que receba o markdown, chame
   `marked.parse()` e passe o resultado por `DOMPurify.sanitize()` antes de devolver.
3. Substituir `mdEl.innerHTML = marked.parse(body || raw)` (linha ~919) pela chamada ao helper.
4. **Fail-safe:** se `typeof DOMPurify === 'undefined'`, o helper **não** pode devolver HTML.
   Renderizar como texto puro (via `textContent`) ou exibir erro no bloco de erro do drawer.
   Nunca cair no caminho inseguro em silêncio.
5. Configurar a allowlist do DOMPurify de modo a **preservar `href` em `<a>`**, do qual depende
   o interceptador de links `.md` logo abaixo (`mdEl.querySelectorAll('a[href]')`).
6. Verificar se há outros `innerHTML` no arquivo recebendo HTML derivado de conteúdo de arquivo.
   Os que interpolam via `escapeHtml` já estão corretos e **não** devem ser alterados.

**Proibições (escopo negativo — falha de auditoria se violado):**
- Não alterar código de servidor (`.go`, `.js` de servidor, `.py`); em especial `api_file.go` e o
  whitelist anti-path-traversal, que estão corretos e não são a causa.
- Não trocar `marked` por outro parser.
- Não adicionar bundler, framework ou devDependency.
- Não criar arquivo novo em `internal/serve/static/`.
- Não tocar em `npm/`, `pypi/` nem em `pypi/build/lib/`.
- Não adicionar `@media (prefers-color-scheme)` — dashboard é light-only.
- Não alterar `style.css` (nada visual muda neste ML).

**Critérios de aceite:**
- [x] `make build`, `make test`, `make lint` verdes
- [x] `git status --porcelain` mostra **exatamente** `index.html` e `app.js` do canônico
- [x] Nenhum `innerHTML` recebe saída de `marked.parse()` sem passar pelo sanitizador
- [x] Tag do DOMPurify com versão fixada, `integrity` e `crossorigin`
      (SRI reconferido pela Afrodite contra o CDN real, não copiado do roadmap)

**Notas da auditoria de Zeus:**
- Ponto de sanitização **único e removível** (`renderMarkdownSafe`) — requisito para o seam do ML-2A.
- Fail-safe devolve `null`, nunca HTML; o chamador degrada para `textContent` + aviso.
- Segundo sink verificado: `renderFrontmatterTable` (~1044) usa `createElement` + `textContent`,
  sem interpolação — já seguro. Frontmatter vem do mesmo arquivo não confiável que o corpo.
- **Allowlist sem `img`:** levantei o risco de quebrar diagramas em ADRs de projetos downstream.
  **Descartado por verificação:** o servidor só expõe `/`, `/static/` e `/api/*`; imagens relativas
  já retornavam **404** antes desta mudança (`curl localhost:8793/docs/adr/diagrama.png` → 404).
  Excluir `img` preserva o status quo em vez de regredir, e evita que URLs http(s) absolutas
  virem vetor de tracking/exfiltração disparado automaticamente.

---

## Wave 2 — Seam de falsificação e auditoria visual (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Provar o efeito em navegador real
**Status:** ✅ concluído (auditado 2026-07-31)
**Agente:** Ártemis (+ auditoria de Zeus)
**Arquivos afetados:** nenhum de produto — apenas fixture temporária, removida ao final

**Ações:**
1. Criar fixture temporária (ADR ou REQ descartável, **fora** de `docs/` para não sujar a cadeia,
   ou em `docs/` e removida antes do commit) com payload conhecido no corpo, por exemplo
   `<img src=x onerror="window.__XSS_FIRED=true">` e um `<script>` marcando outra flag.
2. `make build` e servir. No navegador, abrir o drawer da fixture e confirmar:
   `window.__XSS_FIRED` **undefined**, nenhum `img[onerror]` nem `script` no DOM do drawer.
3. **Falsificação (prova de vivacidade):** remover temporariamente a chamada de sanitização,
   `make build`, reabrir. O payload **deve** executar. Se não executar, o seam está inativo e o
   teste do passo 2 é vacuoso — investigar antes de prosseguir.
4. Restaurar a sanitização e reconfirmar o passo 2.
5. **Fail-safe:** bloquear o carregamento do DOMPurify (alterar a URL da tag para um caminho
   inválido, ou bloquear o domínio no navegador) e confirmar que o drawer degrada para texto puro
   ou erro — **não** renderiza HTML bruto. Restaurar.
6. Regressão de markdown legítimo: abrir uma ADR real do repositório e confirmar headings, listas
   ordenadas e não ordenadas, blockquote, `code` inline, blocos de código, tabelas e links.
7. Confirmar que o clique num link interno `.md` dentro do drawer ainda navega via `openDrawer`.
8. Abrir o drawer pelos **três** caminhos: card do Board, nó do grafo Chain, linha das listas.
9. Console sem erros de JavaScript.
10. **Remover a fixture** e confirmar `git status --porcelain` limpo de resíduo.

**Critérios de aceite:**
- [x] Payload inerte com sanitização ativa — quatro flags `undefined`, sem `img`/`script` no DOM
- [x] Seam provado vivo com a sanitização removida
- [x] Sanitização restaurada e Fase 1 reconfirmada
- [x] Fail-safe verificado: `is_plain_text: true`, nenhum vetor executa, aviso exibido
- [x] Markdown legítimo intacto; link `.md` interno interceptado com `preventDefault`
- [x] Três caminhos verificados **com clique real dispatchado**, não chamada direta a `openDrawer`
- [x] Fixtures removidas; `git status --porcelain` vazio; build/test/lint verdes

**Notas da auditoria de Zeus:**
- Verificado independentemente: árvore limpa, zero diff contra `fd7459b`, sem fixture residual,
  `DOMPurify.sanitize` e `integrity` presentes.
- **Rigor do seam:** `__XSS_SCRIPT` **não** dispara nem com a sanitização removida — `<script>`
  inserido via `innerHTML` nunca executa, por especificação HTML. A Ártemis identificou isso e
  provou o vetor por **diferencial de presença do nó** (`has_script` false → true) em vez de
  tratar a flag ausente como sucesso. Sem essa distinção a asserção seria vacuosa.
- Os vetores decisivos foram os de **tag permitida**: `onclick` removido com `href` preservado, e
  `href="javascript:"` neutralizado. Provam a filtragem de atributos, não só a allowlist de tags.
- Achado colateral **não corrigido**: links `.md` relativos com `../` retornam 403 no `/api/file`
  (o interceptador passa o href bruto). Pré-existente, merece REQ própria.
- Armadilhas de instrumentação registradas em
  `vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`.

---

## Wave 3 — Espelhamento e paridade (1 ML)
> Dependências: **Wave 2 aprovada**

### ML-3A — Espelhar assets para npm e pypi
**Status:** ✅ concluído (auditado 2026-07-31)
**Agente:** Afrodite
**Arquivos afetados:**
- `npm/src/serve/static/{app.js,index.html,style.css}`
- `pypi/trackfw/serve/static/{app.js,index.html,style.css}`

**Ações:** cópia mecânica dos **três** arquivos do canônico para os dois destinos — inclusive
`style.css`, que não muda neste ciclo mas entra na comparação byte-a-byte que o gate faz sobre
toda a lista derivada do canônico.

```bash
cp internal/serve/static/app.js internal/serve/static/index.html internal/serve/static/style.css npm/src/serve/static/
cp internal/serve/static/app.js internal/serve/static/index.html internal/serve/static/style.css pypi/trackfw/serve/static/
```

**Proibições:** não editar conteúdo (divergência é bug da Wave 1); não tocar em
`internal/serve/static/`, `pypi/build/lib/` nem em `scripts/check-static-assets.sh`.

**Critérios de aceite:**
- [x] `scripts/check-static-assets.sh` imprime `Static assets are synchronized`
- [x] md5 idêntico nos três diretórios (confirmado por `cmp` na auditoria)
- [x] `make quality` exit 0 — 82 checks OK, 24 cenários de falsificação
- [x] Runtimes Node e Python servem `dompurify@3.4.12` com `integrity` e `DOMPurify.sanitize`
- [x] `git status --porcelain` mostra **quatro** arquivos, não seis: `style.css` já estava
      byte-idêntico nos espelhos (não mudou neste ciclo), então o `cp` não gerou diff.
      Discrepância explicada e correta — a Afrodite reportou espontaneamente em vez de omitir.

---

## Fechamento

Todos os microlotes concluídos e auditados em 2026-07-31. `make quality` exit 0 (82 checks).

**Correção entregue:** `openDrawer` deixa de expor HTML não sanitizado. Ponto único
`renderMarkdownSafe()` com allowlist restrita ao markdown real de ADRs/REQs, e fail-safe que
degrada para `textContent` quando o DOMPurify não carrega.

**Prova:** seam de falsificação em navegador real. Não é gate de CI — decisão consciente do
usuário, porque o projeto tem zero devDependency e jsdom mudaria essa propriedade. O trade-off
aceito é a ausência de barreira automática contra regressão futura.

**Achados colaterais não corrigidos** (candidatos a REQ própria):
1. Links `.md` relativos com `../` retornam **403** no `/api/file` — o interceptador passa o href
   bruto e o whitelist rejeita. Pré-existente. Afeta documentos reais do repositório.
2. Nenhuma das outras cinco tags CDN tem `integrity` — SRI foi aplicado só ao DOMPurify,
   por decisão do usuário.
