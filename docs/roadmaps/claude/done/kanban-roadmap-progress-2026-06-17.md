---
name: kanban-roadmap-progress-2026-06-17
title: "feat: kanban board mostra progresso Wave/ML dos roadmaps"
status: done
req: ~
created: 2026-06-17
author: zeus
---

# Roadmap: Kanban Board — Visibilidade de Wave/ML

> Criado em: 2026-06-17 | Status: 🔄 WIP

## Diagnóstico / Contexto

O board Kanban atual trata cada roadmap como uma caixa opaca: o card exibe apenas
título, agent badge e state badge. Não há visibilidade de qual Wave/ML está ativo,
nem do progresso total de microlotes — informação crítica para quem monitora a
execução em andamento (coluna WIP).

**Estrutura parseável nos roadmaps:**
- Waves → headings `## Wave N — <título>`
- MLs → headings `### ML-NA — <título>`
- Status de cada ML → linha `**Status:** ⬜/🔄/✅/❌`

**Decisão de design validada pelo usuário:**
Mostrar no card: barra de progresso (X/Y MLs) + ML atualmente `🔄 Em andamento`
com referência à Wave pai.

**Escopo:** `trackfw serve` (Go-only — exceção explícita à regra de paridade 3 CLIs).

---

## Wave 1 — Backend Go (independente) — 1 ML

> Dependências: nenhuma

### ML-1A — Parsear Waves/MLs em api_board.go

**Status:** ✅ Concluído

**Arquivo afetado:** `internal/serve/api_board.go`

**Ações:**
1. Adicionar campos ao `boardItem`:
   ```go
   MLTotal  int    `json:"ml_total"`
   MLDone   int    `json:"ml_done"`
   ActiveML string `json:"active_ml"` // ex: "Wave 2 · ML-2B — Python CLI"
   ```
2. Criar função `parseMLProgress(path string) (total, done int, activeML string)`:
   - Ler o arquivo com `os.ReadFile`
   - Iterar linha por linha:
     - Ao encontrar `## Wave` (H2), armazenar o título da wave atual (ex: "Wave 2")
     - Ao encontrar `### ML-` (H3), armazenar título do ML atual; incrementar `total`
     - Ao encontrar linha que começa com `**Status:**`:
       - Se contém `✅` → incrementar `done`
       - Se contém `🔄` → definir `activeML = "<wave atual> · <ml atual>"`
   - Retornar total, done, activeML
3. Chamar `parseMLProgress` dentro de `readStateDir` ao construir cada `boardItem`

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde
- [ ] `GET /api/board` retorna `ml_total`, `ml_done`, `active_ml` nos items de roadmaps com MLs
- [ ] Roadmaps sem MLs retornam `ml_total: 0, ml_done: 0, active_ml: ""`

**Comandos de validação:**
```bash
make build
make test
```

---

## Wave 2 — Frontend JS (depende de Wave 1) — 1 ML

> Dependências: Wave 1 completa (API retorna os novos campos)

### ML-2A — Renderizar progresso no card em app.js

**Status:** ✅ Concluído

**Arquivo afetado:** `internal/serve/static/app.js`

**Ações:**
1. Modificar `createCard(card)` (linha ~199): após o bloco de badges, adicionar
   seção de progresso condicional (só renderizar se `card.ml_total > 0`):
   ```html
   <!-- Barra de progresso -->
   <div class="mt-2 pt-2 border-t border-gray-100">
     <div class="flex items-center justify-between mb-1">
       <span class="text-xs text-gray-400">MLs</span>
       <span class="text-xs font-medium text-gray-600">${done}/${total}</span>
     </div>
     <div class="w-full bg-gray-200 rounded-full h-1.5">
       <div class="bg-blue-500 h-1.5 rounded-full transition-all"
            style="width: ${pct}%"></div>
     </div>
     <!-- ML ativo (se houver) -->
     ${activeML ? `<p class="text-xs text-blue-600 mt-1 truncate" title="${activeML}">▶ ${activeML}</p>` : ''}
   </div>
   ```
2. Calcular `pct = total > 0 ? Math.round((done / total) * 100) : 0`
3. Cor da barra:
   - `done === total && total > 0` → `bg-green-500`
   - `card.state === 'blocked'` → `bg-red-400`
   - default → `bg-blue-500`
4. Aplicar `escapeHtml` no `activeML` antes de renderizar

**Critérios de aceite:**
- [ ] Cards com `ml_total > 0` exibem barra de progresso
- [ ] Cards sem MLs (ml_total === 0) não exibem seção de progresso
- [ ] ML ativo aparece com prefixo `▶` e trunca com `…` se longo
- [ ] Barra fica verde quando `ml_done === ml_total` (100%)
- [ ] Nenhuma regressão nas outras views (chain, metrics)

**Comandos de validação:**
```bash
make build
# Abrir http://localhost:4080 e verificar visualmente
```
