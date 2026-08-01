---
status: wip
date: 2026-08-01
req: "docs/req/REQ-2026-08-01-proteger-dependencias-cdn-do-dashboard-com-sri-e-remover-htmx-morto.md"
squad: ""
---

# Roadmap: Proteger dependencias CDN do dashboard com SRI e remover htmx morto

> Created: 2026-08-01 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-01-proteger-dependencias-cdn-do-dashboard-com-sri-e-remover-htmx-morto.md
ADR: docs/adr/ADR-2026-08-01-sri-nas-dependencias-cdn-versionadas-e-remocao-do-htmx-nao-utilizado.md

Último item da fila de follow-ups aberta desde o ciclo das abas ADRs/REQs. O DOMPurify ganhou SRI
no PR #95; as cinco tags anteriores seguem sem.

O levantamento mudou o escopo: **htmx tem zero usos** (dependência morta) e **o Tailwind não pode
receber SRI** (URL não-versionada, `HTTP/2 302`, `max-age=14400` — hash fixo quebraria o dashboard
no próximo release deles).

### Decisão do ADR

htmx **removido** (eliminar > mitigar); marked, chart.js e d3 recebem SRI; Tailwind fica sem, com
o motivo em comentário no próprio `index.html` para ninguém "uniformizar" sem entender.

Hashes conferidos em **dois downloads independentes cada**, em 2026-08-01:

| Dependência | SRI |
|---|---|
| marked 12.0.0 | `sha384-NNQgBjjuhtXzPmmy4gurS5X7P4uTt1DThyevz4Ua0IVK5+kazYQI1W27JHjbbxQz` |
| chart.js 4.4.4 | `sha384-NrKB+u6Ts6AtkIhwPixiKTzgSKNblyhlk0Sohlgar9UHUBzai/sgnNNWWd291xqt` |
| d3 7.9.0 | `sha384-CjloA8y00+1SDAUkjs099PVfnY2KmDC2BZnws9kh8D/lX1s46w6EPhpXdqMfjK6i` |

### Dependências e paralelismo

Três waves **sequenciais** — arquivo canônico único (`index.html`); a Wave 2 verifica o produto da
Wave 1 e a Wave 3 espelha o que a Wave 2 aprovou.

## Critérios de Aceite

- [ ] htmx removido; nenhuma referência remanescente nos 3 CLIs
- [ ] marked, chart.js e d3 com `integrity`, `crossorigin` e `referrerpolicy`
- [ ] Tailwind sem SRI, com comentário explicando o porquê
- [ ] Dashboard íntegro em navegador real: 5 abas, grafo D3, gráficos Chart.js, drawer, estilo
- [ ] **SRI provado ativo** — hash corrompido bloqueia o script
- [ ] npm e pypi byte-a-byte idênticos; `make quality` exit 0

---

## Wave 1 — index.html canônico (1 ML)
> Dependências: nenhuma

### ML-1A — Remover htmx e aplicar SRI
**Status:** pending
**Agente:** Afrodite
**Arquivos afetados:** `internal/serve/static/index.html` (apenas este)

**Ações:**
1. Remover a tag do htmx (linha 10) e o comentário que a acompanha, se houver.
2. Aplicar `integrity` (valores exatos acima), `crossorigin="anonymous"` e
   `referrerpolicy="no-referrer"` em marked (12), chart.js (14) e d3 (16). Espelhar a formatação
   da tag do DOMPurify (linha ~320), que já segue esse padrão.
3. Acrescentar comentário na tag do Tailwind explicando que a URL é não-versionada e que um SRI
   fixo quebraria o dashboard no próximo release — por isso a ausência é deliberada.

**Proibições:** não adicionar SRI ao Tailwind; não trocar sua URL; não alterar versões; não tocar
em `app.js`, `style.css`, `npm/`, `pypi/`, código de servidor ou whitelist; não criar arquivo novo.

**Acceptance criteria:**
- [ ] `make build`, `make test`, `make lint` verdes
- [ ] `git status --porcelain` mostra **apenas** `internal/serve/static/index.html`
- [ ] `grep -ri htmx internal/serve/` sem resultado
- [ ] Os três `integrity` conferem com os valores acima, byte a byte

---

## Wave 2 — Verificação em navegador (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Provar integridade e ausência de regressão
**Status:** pending
**Agente:** Ártemis

**Ações:**
1. Dashboard íntegro: as 5 abas renderizam; grafo D3 desenha; gráficos Chart.js desenham; drawer
   renderiza markdown (marked) sanitizado (DOMPurify); Tailwind estiliza normalmente.
2. Console sem erro de integridade.
3. **Prova de que o SRI está ativo** (AC5) — sem isso o `integrity` é decorativo: corromper **um**
   hash, `make build`, recarregar, e confirmar que o navegador **bloqueia** o script, que a
   funcionalidade correspondente quebra e que o console registra erro de integridade. Restaurar e
   reconfirmar. Repetir para pelo menos **dois** dos três, para não provar só um caminho.
4. Confirmar que a remoção do htmx não quebrou nada — nenhuma funcionalidade dependia dele.

**Acceptance criteria:**
- [ ] Dashboard íntegro; console limpo
- [ ] SRI provado ativo em ao menos 2 das 3 tags
- [ ] Restauração confirmada
- [ ] `git status --porcelain` sem resíduo

---

## Wave 3 — Espelhamento (1 ML)
> Dependências: **Wave 2 aprovada**

### ML-3A — Espelhar para npm e pypi
**Status:** pending
**Agente:** Afrodite
**Arquivos afetados:** `npm/src/serve/static/{app.js,index.html,style.css}`, `pypi/trackfw/serve/static/{...}`

Cópia mecânica dos três arquivos do canônico. Só `index.html` deve gerar diff — reportar a
contagem e explicá-la.

**Acceptance criteria:**
- [ ] `scripts/check-static-assets.sh` imprime `Static assets are synchronized`
- [ ] `make quality` exit 0
- [ ] Runtimes Node e Python servem o `index.html` sem htmx e com os três `integrity`
- [ ] `git status --porcelain` sem arquivo inesperado
