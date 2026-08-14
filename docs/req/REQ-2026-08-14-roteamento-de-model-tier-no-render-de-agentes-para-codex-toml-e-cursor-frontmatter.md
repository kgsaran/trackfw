---
status: Open
date: 2026-08-14
author: "Zeus"
adr: "docs/adr/ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-14-roteamento-de-model-tier-no-render-de-agentes-para-codex-toml-e-cursor-frontmatter.md"
---

# REQ: roteamento de model tier no render de agentes para codex (toml) e cursor (frontmatter)

> Date: 2026-08-14 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

O catálogo canônico de agentes trackfw já declara um tier de custo por agente
(`model: opus` para `architect`, `model: sonnet` para os demais 9 especialistas), mas
esse tiering só é efetivo hoje em dois alvos: Claude Code (nativo, sem tradução) e
Antigravity CLI (mapeado via `mapModel()`, ADR-2026-07-19). Nos outros alvos que
suportam seleção de modelo por agente:

1. **Codex CLI** (`representation: custom-agent-toml`): `Render()` nunca emite o campo
   `model` no TOML gerado (`.codex/agents/trackfw-*.toml`) — confirmado por leitura
   direta de `internal/integrations/render.go`. O agente customizado do Codex sempre
   roda no modelo default da sessão, ignorando o tier declarado no catálogo, mesmo o
   runtime aceitando `model = "..."` nativamente (confirmado via documentação oficial
   Codex, ver ADR vinculado).
2. **Cursor** (`representation: agent-markdown`, compartilhada com `gemini`/`kiro`):
   `Render()` não tem `case` dedicado, cai no branch `default` e devolve `model: opus`
   / `model: sonnet` verbatim. A documentação oficial da Cursor não lista esses aliases
   como valores aceitos (`cursor.com/docs/subagents` documenta `inherit` ou IDs
   completos como `claude-opus-5`/`composer-2.5`, com parâmetros opcionais entre
   colchetes) — mesmo padrão de risco que já foi comprovado e corrigido para o
   Antigravity (REQ-2026-07-19), mas aqui ainda não confirmado empiricamente.

Sem esse roteamento, a política de custo "orquestrador caro, especialistas baratos"
que o catálogo já expressa fica sem efeito nesses dois runtimes — o usuário paga o
mesmo custo de modelo em todo agente trackfw instalado via Codex ou Cursor,
independente da complexidade da tarefa.

## Acceptance Criteria
<!-- Wave numbers below anticipate the roadmap breakdown; keep this list in sync with ROADMAP MLs -->
- [ ] `Render()` ganha um parâmetro para identificar o `target.ID` do alvo sendo
      renderizado (ex.: `cursor`, `gemini`, `kiro`), permitindo diferenciar
      comportamento dentro da representação `agent-markdown` compartilhada. Todos os
      call-sites nos 3 CLIs (Go, Node.js, Python) são atualizados; testes de contrato
      existentes continuam verdes.
- [ ] Branch `case "custom-agent-toml"` (Codex) emite `model = "<valor mapeado>"` no
      TOML, via nova função `mapModelCodex()` análoga a `mapModel()`: `opus` e
      `sonnet` mapeiam para IDs de modelo Codex vigentes (a decidir na Wave de
      implementação, documentados em comentário no código citando a fonte).
- [ ] Dentro do branch `default`/`agent-markdown`, quando `target.ID == "cursor"`,
      a linha `model:` do frontmatter é reescrita para o valor mapeado por
      `mapModelCursor()` (`opus`/`sonnet` → sintaxe Cursor confirmada em
      `cursor.com/docs/subagents`), preservando todo o restante do frontmatter e do
      corpo byte-a-byte (mesmo contrato de `rewriteFrontmatterFields`).
- [ ] `gemini` e `kiro` (mesma representação `agent-markdown`) permanecem com o
      passthrough atual, byte-a-byte idêntico ao comportamento pré-mudança —
      comprovado por teste de regressão que falha se o output desses dois alvos
      mudar.
- [ ] Paridade nos 3 CLIs (Go, Node.js, Python) com testes de contrato verdes
      (`make quality`).
- [ ] `init --ai-tools codex` gera `.codex/agents/trackfw-architect.toml` com
      `model = "<valor heavy>"` e `.codex/agents/trackfw-backend.toml` com
      `model = "<valor light>"`.
- [ ] `init --ai-tools cursor` gera `.cursor/agents/trackfw-architect.md` com
      `model: <valor heavy>` e `.cursor/agents/trackfw-backend.md` com
      `model: <valor light>`, sintaticamente válidos conforme a documentação oficial
      citada no ADR (verificação manual do usuário registrada na REQ antes de mover
      para `Done`, na ausência de instância local do Cursor para teste automatizado).

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-14-roteamento-de-model-tier-no-render-de-agentes-para-codex-toml-e-cursor-frontmatter.md
