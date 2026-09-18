---
status: done
date: 2026-08-01
req: "docs/req/REQ-2026-08-01-corrigir-403-em-links-markdown-relativos-dentro-do-drawer.md"
squad: ""
---

# Roadmap: Corrigir 403 em links markdown relativos dentro do drawer

> Created: 2026-08-01 | Status: done

## Context

REQ: docs/req/REQ-2026-08-01-corrigir-403-em-links-markdown-relativos-dentro-do-drawer.md
ADR: docs/adr/ADR-2026-08-01-resolucao-de-links-markdown-relativos-ao-documento-aberto-no-drawer.md

O interceptador de links do drawer (`internal/serve/static/app.js:966-977`) passa o **href bruto**
para `openDrawer`, que o envia como `?path=`. Links relativos são rejeitados pelo whitelist.

Reproduzido em 2026-08-01:

```
?path=../roadmaps/done/v2.3-validator-improvements-2026-06-13.md   → 403 Forbidden
?path=docs/roadmaps/done/v2.3-validator-improvements-2026-06-13.md → 200
```

### Levantamento que torna a regra inequívoca

Todas as formas de link `.md` em `docs/`: `./X.md` (13), `X.md` nu (3), `../vault/notes/X.md` (3),
`../../../requisições/claude/X.md` (5), `../roadmaps/done/X.md` (1), `../../req/X.md` (1).

**Nenhuma** é relativa à raiz. Todas são relativas ao documento — não existe ambiguidade a
resolver, o que seria o caso se as duas formas convivessem.

### Decisão do ADR

Resolver o href contra `dirname(_drawerPath)` **no cliente**. O whitelist do servidor **não muda**
— `vault/` continua fora, por decisão do usuário. Link que resolva para fora dos diretórios
permitidos exibe **mensagem explicativa** em vez de `Forbidden` cru.

Correção **frontend puro**: `internal/serve/static/` é canônico, npm e pypi são espelhos
byte-a-byte. Mesma forma do ciclo do DOMPurify.

### Dependências e paralelismo

Três waves **estritamente sequenciais** — arquivo canônico único; a Wave 2 verifica o produto da
Wave 1 e a Wave 3 espelha o que a Wave 2 aprovou. Sem paralelismo possível.

## Acceptance Criteria

Consolidados da REQ (AC1–AC11). Detalhamento por microlote abaixo.

- [x] Href relativo resolvido contra `dirname` do documento aberto, normalizando `.` e `..`
- [x] As três formas funcionam: `./X.md`, `X.md` nu, `../dir/X.md`
- [x] Navegação encadeada A → B → C resolve cada salto contra o documento **então** aberto
- [x] Fora do whitelist → mensagem explicativa com o caminho resolvido, não `Forbidden` cru
- [x] Links externos, âncoras e não-`.md` seguem não interceptados
- [x] Caminho de entrada (Board / Chain / listas) continua funcionando
- [x] npm e pypi byte-a-byte idênticos; `make quality` exit 0

---

## Wave 1 — Resolução canônica (1 ML)
> Dependências: nenhuma

### ML-1A — Resolver href relativo e melhorar o erro
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Afrodite
**Arquivos afetados:** `internal/serve/static/app.js` (apenas este)

**Ações:**
1. Criar helper de resolução: dado o href e o caminho do documento atual, devolver o caminho
   normalizado relativo à raiz. Tratar `./`, `../` encadeados e href nu (mesma pasta).
2. No interceptador (~966-977), resolver o href **antes** de chamar `openDrawer`.
3. No tratamento de erro de `openDrawer` (~984), distinguir **403** dos demais: exibir mensagem
   explicando que o arquivo está fora dos diretórios permitidos, **incluindo o caminho resolvido**.
   Os outros erros mantêm o comportamento atual.
4. Não interceptar href externo (`http://`, `https://`), âncora (`#`) nem não-`.md` — preservar.
5. Garantir que o caminho de **entrada** (path já completo, vindo de Board/Chain/listas) não é
   afetado: `openDrawer` continua recebendo caminho pronto desses três.

**Proibições:**
- Não alterar código de servidor (`.go`, `.js` de servidor, `.py`); em especial `api_file.*`.
- Não alterar o whitelist; `vault/` **não** entra.
- Não criar arquivo novo em `internal/serve/static/`.
- Não alterar `index.html` nem `style.css` (nada visual muda).
- Não tocar em `npm/`, `pypi/`, `pypi/build/lib/`.
- Não reescrever links em `docs/`.

**Acceptance criteria:**
- [x] `make build`, `make test`, `make lint` verdes
- [x] `git status --porcelain` mostra **apenas** `internal/serve/static/app.js`
- [x] Helper `resolveRelativeMdHref` isolado, com clamp que impede escapar acima da raiz
- [x] Erro 403 distinguido, com caminho resolvido na mensagem

**Notas da auditoria de Zeus:**
- `const resolved` é computado **no render**, dentro do `forEach`, quando `_drawerPath` já é o
  documento sendo exibido. É isso que faz o encadeamento A → B → C funcionar sem base fixa —
  cada render reexecuta a resolução com o `_drawerPath` novo.
- Ela tratou `arquivo.md#ancora` sem que eu pedisse: separa a âncora antes de testar `.endsWith`.
  Nenhum caso no corpus atual, mas torna o interceptador robusto.
- **Fragilidade reconhecida e documentada no código:** o helper **não é idempotente** para
  caminho já completo. A segurança vem do isolamento do ponto de chamada — os três caminhos de
  entrada chamam `openDrawer` direto e nunca passam pelo interceptador. Ela documentou isso em
  comentário, o que é o tratamento certo: quem rotear um caminho de entrada pelo helper no futuro
  vai ler o aviso.

---

## Wave 2 — Verificação em navegador (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Provar a navegação em navegador real
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Ártemis

**Ações e verificações:**
1. Caso real do repositório: abrir `docs/req/REQ-2026-06-13-validator-improvements.md` e clicar em
   `../roadmaps/done/v2.3-validator-improvements-2026-06-13.md`. O drawer deve trocar de conteúdo.
2. As três formas relativas (`./X.md`, `X.md` nu, `../dir/X.md`) — usar fixtures temporárias se o
   repositório não tiver todas as formas em posição conveniente. **Remover as fixtures ao final.**
3. **Navegação encadeada A → B → C**, com os três em diretórios diferentes, confirmando que cada
   salto resolve contra o documento **então** aberto. Este é o caso que mais provavelmente
   falha numa implementação ingênua.
4. Link fora do whitelist (`../vault/notes/*.md`): mensagem explicativa com o caminho resolvido,
   **sem** `Forbidden` cru. Confirmar que o whitelist não foi alterado (`vault/` segue 403 no
   servidor).
5. Link externo `https://` continua **não** interceptado.
6. Caminho de entrada intacto: Board, grafo Chain e listas ADRs/REQs.
7. Console sem erros de JavaScript.

**Armadilhas conhecidas** (`vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`):
`_drawerPath` **não** é propriedade de `window` — use o identificador puro em `Runtime.evaluate`.
`closeDrawer()` não readiciona a classe `hidden` — verifique visibilidade por
`getComputedStyle(el).display`.

**Acceptance criteria:**
- [x] Caso real navega: `docs/req/REQ-2026-06-13-...` → `docs/roadmaps/done/v2.3-...`
- [x] Três formas relativas funcionam (`./X.md`, `X.md` nu, `../dir/X.md`)
- [x] Encadeamento A → B → C correto **em cada salto**
- [x] Fora do whitelist → `Arquivo fora dos diretórios permitidos: vault/notes/...`
- [x] Externos e os **quatro** pontos de entrada intactos; console limpo
- [x] Fixtures removidas; `git status --porcelain` vazio; build/test/lint verdes

**Notas da auditoria de Zeus:**
- **O teste encadeado foi desenhado para ser discriminante**, não apenas para passar: A em
  profundidade 2 (`docs/req/`), B em profundidade 3 (`docs/adr/tmp_qa_b/`), com o link B→C sendo
  `../../roadmaps/done/tmp-qa-c.md`. Com base congelada em A o resultado seria
  `roadmaps/done/tmp-qa-c.md` (403); com base correta, `docs/roadmaps/done/tmp-qa-c.md` (200).
  A diferença de profundidade é o que impede o teste de ser vacuoso — mérito dela, não estava no
  meu handoff.
- Ela verificou o `_drawerPath` **após cada salto**, não só no final.
- Verificou também que o card do Board resolve com **prefixo único, não duplicado** — exatamente
  o risco do helper não-idempotente documentado no ML-1A. Confirma que o isolamento do ponto de
  chamada segura.
- Console: separou ruído benigno (aviso do CDN Tailwind, 404 de favicon do próprio Chrome) de
  erro de aplicação. Zero exceções de `app.js`.
- Verificação independente de Zeus: árvore limpa, zero fixtures residuais, `app.js` idêntico ao
  commit `fd04979`.

---

## Wave 3 — Espelhamento (1 ML)
> Dependências: **Wave 2 aprovada**

### ML-3A — Espelhar assets para npm e pypi
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Afrodite
**Arquivos afetados:** `npm/src/serve/static/{app.js,index.html,style.css}`, `pypi/trackfw/serve/static/{...}`

Cópia mecânica dos **três** arquivos do canônico, inclusive os que não mudaram — o gate compara
byte-a-byte toda a lista derivada do canônico.

**Acceptance criteria:**
- [x] `scripts/check-static-assets.sh` imprime `Static assets are synchronized`
- [x] `make quality` exit 0 — 42 cenários de falsificação, incluindo `static-assets/byte-drift`
- [x] Runtimes Node e Python servem `app.js` com `resolveRelativeMdHref`
- [x] `git status --porcelain` mostra **2** arquivos, não 6: `index.html` e `style.css` já estavam
      byte-idênticos nos espelhos, então o `cp` não gerou diff. Discrepância explicada
      espontaneamente por ela, e confirmada por `cmp` na auditoria.

---

## Fechamento

Concluído e auditado em 2026-08-01. `make quality` exit 0; falsificação 42/42.

**Entrega:** links `.md` relativos passam a navegar dentro do drawer nos 3 CLIs, cobrindo
`./X.md`, `X.md` nu e `../` encadeados. Link que resolva para fora dos diretórios permitidos
exibe o caminho resolvido em mensagem explicativa, em vez de `Forbidden` cru.

**Whitelist inalterado** — `vault/` segue fora, por decisão do usuário. Os 3 links para vault
agora falham de forma compreensível em vez de silenciosa. Os 5 links
`../../../requisições/claude/*.md` (legado de outro projeto, apontam para fora do repositório)
seguem falhando, também com mensagem clara.

**Fragilidade conhecida e documentada:** `resolveRelativeMdHref` **não é idempotente** para
caminho já completo. A segurança vem do isolamento do ponto de chamada — só o interceptador de
links do markdown o invoca; os quatro pontos de entrada chamam `openDrawer` direto. Está em
comentário no código, e a Wave 2 confirmou empiricamente (prefixo único no card do Board).
Quem for rotear um caminho de entrada pelo helper no futuro precisa reler isso.
