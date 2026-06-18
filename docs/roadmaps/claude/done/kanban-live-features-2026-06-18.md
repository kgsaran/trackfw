---
name: kanban-live-features-2026-06-18
title: "feat(serve): kanban live — auto-refresh, indicador de atividade e atenção de agente"
status: done
req: ~
created: 2026-06-18
author: zeus
---

# Roadmap: Kanban Live Features — Auto-refresh, Atividade e Atenção

> Criado em: 2026-06-18 | Status: 🔄 WIP

## Diagnóstico / Contexto

Feedback de beta-tester: o board Kanban precisa de três capacidades para uso em
monitoramento de times acompanhando implementações longas:

1. **Auto-refresh configurável** — o board deve se atualizar automaticamente,
   com intervalo configurável e toggle de pausa (persiste em localStorage).

2. **Indicador de atividade proeminente** — cards com ML ativo (`active_ml != ""`)
   devem ter destaque visual claro: ponto pulsante, borda verde, badge "ATIVO" e
   ordenação prioritária no topo da coluna WIP.

3. **Sinalização de atenção de agente** — agentes podem escrever
   `{roadmap_dir}/.trackfw-attention.json` para sinalizar que precisam de
   confirmação/ação. O board exibe um banner de alerta e marca o card correspondente.

**Escopo:** `trackfw serve` (Go-only — exceção explícita à regra de paridade 3 CLIs).

### Convenção do arquivo `.trackfw-attention.json`

```json
{
  "roadmap": "nome-do-arquivo.md",
  "ml": "ML-2A — Título",
  "message": "Texto explicando o que o agente precisa",
  "level": "action_required",
  "timestamp": "2026-06-18T10:30:00Z"
}
```

- `level`: `"action_required"` (âmbar) ou `"info"` (azul)
- Arquivo ausente ou vazio → sem alerta
- Dismiss no browser oculta localmente (persiste por timestamp no localStorage)

---

## Wave 1 — Backend Go (independente) — 1 ML

### ML-1A — Novo endpoint /api/attention

**Status:** ⬜ Pendente

**Arquivos afetados:**
- `internal/serve/api_attention.go` (novo)
- `internal/serve/serve.go` (adicionar rota)

**Ações:**
1. Criar `internal/serve/api_attention.go`:
   ```go
   type attentionResponse struct {
       Active    bool   `json:"active"`
       Roadmap   string `json:"roadmap,omitempty"`
       ML        string `json:"ml,omitempty"`
       Message   string `json:"message,omitempty"`
       Level     string `json:"level,omitempty"`
       Timestamp string `json:"timestamp,omitempty"`
   }
   ```
   - Ler `filepath.Join(cfg.RoadmapDir, ".trackfw-attention.json")`
   - Se arquivo não existir: `{"active": false}`
   - Se existir: unmarshalar e retornar com `"active": true`
   - Em caso de JSON inválido: `{"active": false}`

2. Em `serve.go` registrar:
   ```go
   mux.HandleFunc("/api/attention", func(w http.ResponseWriter, r *http.Request) {
       attentionHandler(w, r, cfg)
   })
   ```

**Critérios de aceite:**
- [ ] `make build` sem erros
- [ ] `make test` verde
- [ ] `GET /api/attention` retorna `{"active":false}` quando arquivo ausente
- [ ] `GET /api/attention` retorna objeto completo quando arquivo existe

---

## Wave 2 — Frontend (depende de Wave 1) — 3 MLs em paralelo

> Dependências: Wave 1 completa

### ML-2A — Auto-refresh (app.js + index.html)

**Status:** ⬜ Pendente

**Arquivos afetados:** `internal/serve/static/app.js`, `internal/serve/static/index.html`

**Ações em app.js:**
1. Adicionar variáveis de estado:
   ```js
   let _refreshInterval = parseInt(localStorage.getItem('trackfw_refresh_interval') || '60');
   let _refreshPaused   = localStorage.getItem('trackfw_refresh_paused') === 'true';
   let _refreshTimer    = null;
   ```
2. Funções:
   - `startRefreshTimer()` — agenda próximo refresh via `setTimeout`; no callback: `_boardData = null` + `loadBoard()` + reagenda
   - `setRefreshInterval(seconds)` — salva em localStorage, reagenda
   - `toggleRefreshPause()` — alterna pausa, salva em localStorage
   - `manualRefresh()` — `_boardData = null; loadBoard(); startRefreshTimer();`
   - `updateRefreshUI()` — sincroniza texto do botão e seletor
3. Chamar `startRefreshTimer()` no `DOMContentLoaded`

**Ações em index.html:**
- Na section `view-board`, transformar a toolbar em `flex items-center gap-3 flex-wrap mb-4`
- Adicionar controles de refresh à direita do filtro de agente:
  - `<select id="refresh-interval">` com opções 15s / 30s / 1min / 2min / 5min
  - `<button id="refresh-toggle">⏸ Pausar</button>`
  - `<button onclick="manualRefresh()">↻</button>`

**Critérios de aceite:**
- [ ] Seletor reflete valor salvo em localStorage ao recarregar a página
- [ ] Board atualiza automaticamente no intervalo selecionado
- [ ] Pausar interrompe o ciclo; retomar reinicia
- [ ] Botão ↻ força refresh imediato independente de pausa

---

### ML-2B — Indicador visual de atividade no card (app.js + style.css)

**Status:** ⬜ Pendente

**Arquivos afetados:** `internal/serve/static/app.js`, `internal/serve/static/style.css`

**Ações em app.js — `createCard()`:**
1. Adicionar `data-file="${card.file}"` ao div do card (necessário para vincular com atenção)
2. Adicionar `position: relative` via classe
3. Quando `card.active_ml && card.state === 'wip'`:
   - Classe `card-active` no div
   - Ponto pulsante `<span class="live-dot" aria-hidden="true"></span>` (canto superior direito)
   - Badge `ATIVO` (verde) ao lado dos outros badges
4. Em `renderBoard()`, para a coluna `wip`: ordenar cards com `active_ml` primeiro

**Ações em style.css:**
```css
@keyframes livePulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%       { opacity: 0.4; transform: scale(0.7); }
}
.live-dot {
  position: absolute; top: 8px; right: 8px;
  width: 8px; height: 8px; border-radius: 50%;
  background: #22c55e;
  animation: livePulse 1.4s ease-in-out infinite;
}
.card-active {
  border-left: 3px solid #22c55e;
  background-color: #f0fdf4;
}
.card-active:hover { background-color: #dcfce7; }
```

**Critérios de aceite:**
- [ ] Cards com `active_ml` na coluna WIP aparecem no topo e com estilo verde
- [ ] Ponto pulsante visível no canto superior direito
- [ ] Cards sem `active_ml` ou fora do WIP não recebem estilo especial

---

### ML-2C — Banner de atenção de agente (app.js + index.html + style.css)

**Status:** ⬜ Pendente

**Arquivos afetados:** `internal/serve/static/app.js`, `internal/serve/static/index.html`, `internal/serve/static/style.css`

**Ações em app.js:**
1. Adicionar variável `let _attentionDismissed = localStorage.getItem('trackfw_attention_dismissed') || null;`
2. Funções:
   - `startAttentionPolling()` — `setInterval(pollAttention, 8000)` + poll imediato
   - `pollAttention()` — `fetch('/api/attention')`, se `active && timestamp !== _attentionDismissed`: `showAttentionBanner(data)`; se `!active`: `hideAttentionBanner()`
   - `showAttentionBanner(data)` — preenche ids do banner, remove `hidden`; se `data.roadmap`, chama `markCardAttention(data.roadmap)`
   - `hideAttentionBanner()` — adiciona `hidden`, chama `clearCardAttention()`
   - `dismissAttention()` — salva timestamp em localStorage + `_attentionDismissed`, chama `hideAttentionBanner()`
   - `markCardAttention(roadmapFile)` — adiciona classe `card-attention` ao card com `data-file === roadmapFile`
   - `clearCardAttention()` — remove `card-attention` de todos os cards
3. Chamar `startAttentionPolling()` no `DOMContentLoaded`

**Ações em index.html:**
- Adicionar banner entre `<header>` e `<main>` (não-fixed para empurrar o conteúdo):
  ```html
  <div id="attention-banner" class="hidden attention-banner-action" role="alert" aria-live="assertive">
    <span id="attention-icon">⚠️</span>
    <span class="font-semibold text-sm" id="attention-label"></span>
    <span class="text-sm opacity-80" id="attention-context"></span>
    <span class="text-sm" id="attention-message"></span>
    <button onclick="dismissAttention()" class="ml-auto text-sm underline opacity-80 hover:opacity-100">Dispensar</button>
  </div>
  ```

**Ações em style.css:**
```css
@keyframes attentionPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(245,158,11,0.5); }
  50%       { box-shadow: 0 0 0 8px rgba(245,158,11,0); }
}
.attention-dot {
  position: absolute; top: 8px; right: 8px;
  width: 8px; height: 8px; border-radius: 50%;
  background: #f59e0b;
  animation: livePulse 0.9s ease-in-out infinite;
}
.card-attention {
  border-left: 3px solid #f59e0b;
  animation: attentionPulse 2s ease-in-out infinite;
}
.attention-banner-action {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 24px;
  background: #f59e0b; color: white;
}
.attention-banner-info {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 24px;
  background: #3b82f6; color: white;
}
```

**Critérios de aceite:**
- [ ] Sem arquivo `.trackfw-attention.json`: sem banner
- [ ] Com arquivo válido (`action_required`): banner âmbar aparece abaixo do header
- [ ] Com arquivo válido (`info`): banner azul aparece
- [ ] Card correspondente ao `roadmap` no arquivo fica com borda âmbar pulsante
- [ ] Dispensar oculta o banner e persiste em localStorage (não reaparece até novo timestamp)
- [ ] Ao criar novo arquivo com timestamp diferente: banner reaparece
