---
name: REQ-2026-06-14-trackfw-serve-ui
title: "v2.7.0 — Feat: trackfw serve — Dashboard Web com Kanban, Chain View e Métricas"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/v2.7.0-trackfw-serve-ui-2026-06-14.md
created: 2026-06-14
author: zeus
---

# REQ — v2.7.0 Feat: trackfw serve — Dashboard Web Interativo

## Contexto

O `trackfw serve` hoje sobe um servidor HTTP básico sem interface visual.
A proposta é evoluir para um dashboard web completo, explorando os dados que
o trackfw já coleta: artefatos (ADR/REQ/ROADMAP), grafo de dependências, log
de transições (`.trackfw-log`) e saída do `validate --json`.

Tecnologia escolhida: **HTMX + marked.js + Chart.js** servidos via `embed.FS`
no binário Go — sem bundler, sem npm no runtime, zero dependência externa em produção.

---

## Feature

### Views do dashboard

#### 1. Kanban Board (view principal)
- Colunas: `backlog | wip | blocked | done | abandoned`
- Cards com título do roadmap, agente (modo by_agent), badge de estado
- Suporte a `roadmap_namespacing: by_agent` (agrupar/filtrar por agente)
- **Cards clicáveis**: abrem painel lateral (drawer) com markdown renderizado do arquivo
- Links internos no markdown (`REQ:`, `ADR:`, `req:` no frontmatter) são clicáveis e abrem o artefato referenciado no mesmo drawer

#### 2. Chain View (ADR → REQ → ROADMAP)
- Grafo visual das dependências entre artefatos
- Nós clicáveis: abrem drawer com markdown renderizado
- Colorir por estado (verde=done, amarelo=wip, vermelho=blocked, cinza=backlog)

#### 3. Métricas (derivadas do `.trackfw-log` + scan de arquivos)
- **Lead time**: data criação → data done (por roadmap)
- **Cycle time**: data entrada em wip → data done
- **Taxa de abandono** por agente e período
- **Distribuição por estado** (donut chart)
- **Burndown**: roadmaps `wip+backlog` acumulados ao longo do tempo vs `done`

### Endpoints Go (`internal/serve/`)

| Endpoint | Descrição |
|----------|-----------|
| `GET /` | Serve `index.html` (embed) |
| `GET /static/*` | Serve assets (embed): JS, CSS, marked.js, Chart.js CDN local |
| `GET /api/board` | JSON: roadmaps agrupados por estado/agente |
| `GET /api/chain` | JSON: grafo ADR→REQ→ROADMAP com arestas e metadados |
| `GET /api/metrics` | JSON: lead time, cycle time, taxa de abandono, burndown series |
| `GET /api/file?path=` | Conteúdo raw do arquivo `.md` (validado por allowlist de paths) |

### Markdown rendering
- `marked.js` (CDN local no embed, ~50KB minificado) — client-side
- Links `[ADR-001](docs/adr/...)` → clicáveis, abrem no drawer via `fetch /api/file?path=`
- Frontmatter YAML renderizado como tabela de metadados no topo do drawer

### Segurança
- `/api/file` valida que o path está dentro dos diretórios configurados (adr_dir, req_dir, roadmap_dir)
- Sem path traversal: `filepath.Clean` + prefix check

---

## Critérios de aceite

- [ ] `trackfw serve` abre browser com dashboard kanban funcional
- [ ] Cards clicáveis abrem drawer com markdown renderizado (marked.js)
- [ ] Links internos no markdown são clicáveis e abrem artefato no drawer
- [ ] Chain View renderiza grafo ADR→REQ→ROADMAP com nós clicáveis
- [ ] Métricas: lead time, cycle time, distribuição por estado visíveis
- [ ] Burndown derivado do `.trackfw-log` renderizado como line chart (Chart.js)
- [ ] Suporte a `roadmap_namespacing: by_agent` (filtro por agente no kanban)
- [ ] `/api/file` não permite path traversal fora dos dirs configurados
- [ ] Binário continua single-binary (assets via `embed.FS`)
- [ ] `trackfw serve` exclusivo Go (Node.js/Python não implementam serve — exceção da regra de paridade)

## Não está no escopo

- Edição de arquivos pela UI (read-only)
- Autenticação/autorização
- Deploy remoto / modo multi-usuário
- Story points / velocity (o trackfw não tem esse conceito)
- Node.js e Python CLIs (serve é exclusivo Go)
