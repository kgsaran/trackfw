---
status: wip
date: 2026-08-01
req: "docs/req/REQ-2026-08-01-corrigir-403-em-links-markdown-relativos-dentro-do-drawer.md"
squad: ""
---

# Roadmap: Corrigir 403 em links markdown relativos dentro do drawer

> Created: 2026-08-01 | Status: wip

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

- [ ] Href relativo resolvido contra `dirname` do documento aberto, normalizando `.` e `..`
- [ ] As três formas funcionam: `./X.md`, `X.md` nu, `../dir/X.md`
- [ ] Navegação encadeada A → B → C resolve cada salto contra o documento **então** aberto
- [ ] Fora do whitelist → mensagem explicativa com o caminho resolvido, não `Forbidden` cru
- [ ] Links externos, âncoras e não-`.md` seguem não interceptados
- [ ] Caminho de entrada (Board / Chain / listas) continua funcionando
- [ ] npm e pypi byte-a-byte idênticos; `make quality` exit 0

---

## Wave 1 — Resolução canônica (1 ML)
> Dependências: nenhuma

### ML-1A — Resolver href relativo e melhorar o erro
**Status:** pending
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
- [ ] `make build`, `make test`, `make lint` verdes
- [ ] `git status --porcelain` mostra **apenas** `internal/serve/static/app.js`
- [ ] Helper de resolução isolado e legível, coberto por raciocínio explícito no relatório
- [ ] Erro 403 distinguido, com caminho resolvido na mensagem

---

## Wave 2 — Verificação em navegador (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Provar a navegação em navegador real
**Status:** pending
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
- [ ] Caso real do repositório navega
- [ ] Três formas relativas funcionam
- [ ] Encadeamento A → B → C correto em cada salto
- [ ] Fora do whitelist → mensagem explicativa, não erro cru
- [ ] Externos e caminho de entrada intactos; console limpo
- [ ] Fixtures removidas; `git status --porcelain` sem resíduo

---

## Wave 3 — Espelhamento (1 ML)
> Dependências: **Wave 2 aprovada**

### ML-3A — Espelhar assets para npm e pypi
**Status:** pending
**Agente:** Afrodite
**Arquivos afetados:** `npm/src/serve/static/{app.js,index.html,style.css}`, `pypi/trackfw/serve/static/{...}`

Cópia mecânica dos **três** arquivos do canônico, inclusive os que não mudaram — o gate compara
byte-a-byte toda a lista derivada do canônico.

**Acceptance criteria:**
- [ ] `scripts/check-static-assets.sh` imprime `Static assets are synchronized`
- [ ] `make quality` exit 0
- [ ] Runtimes Node e Python servem o app.js corrigido
- [ ] `git status --porcelain` sem arquivo inesperado
