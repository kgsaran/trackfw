---
status: Open
date: 2026-07-19
author: "Zeus"
adr: "docs/adr/ADR-2026-07-19-antigravity-agent-tools.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-19-antigravity-agent-tools.md"
---

# REQ: Corrigir render Antigravity com tools validos e model tier do agy

> Date: 2026-07-19 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

O `trackfw init --ai-tools antigravity` (surface `current`, representacao `agent-directory`) emite o markdown canonico do asset **verbatim**. Esse output e incorreto para o Antigravity CLI (`agy`), comprovado empiricamente:

1. **`model: opus|sonnet`** (nomes de modelo Anthropic) faz o `agy` **rejeitar silenciosamente** o agente — ele nao aparece em `agy agent`.
2. **Ausencia de `tools:`** faz o agente carregar em **modo read-only** — nao consegue escrever arquivos nem rodar comandos.

Consequencia: os agentes trackfw-* injetados no Antigravity ou nao carregam, ou sao inuteis (read-only). E preciso adaptar o render para o schema do `agy`.

## Acceptance Criteria
- [ ] O render do alvo `antigravity` surface `current` (representacao `agent-directory`) **mapeia** `model`: `opus -> pro`, `sonnet -> flash` (tiers validos do agy); se ausente, omite `model`.
- [ ] O mesmo render **injeta `tools:`** com ids validos do agy: `trackfw-architect` recebe o set orquestrador (14 tools); os demais agentes recebem o set implementador (10 tools).
- [ ] Nenhum id de tool invalido e emitido (proibidos: `edit_file`, `read_file`, `find`, `view_code_item`, `view_file_outline`, `call_mcp_tool`).
- [ ] Os assets compartilhados (`assets/agents/*.md`) **nao** sao alterados; a transformacao ocorre apenas no branch `agent-directory` do render (nao vaza para claude/gemini/cursor).
- [ ] Paridade nos 3 CLIs (Go, Node.js, Python) com testes de contrato verdes.
- [ ] E2E: `init --ai-tools antigravity` gera `~/.gemini/config/agents/trackfw-architect/agent.md` com `tools:` (14) + `model: pro`, e `trackfw-backend/agent.md` com `tools:` (10) + `model: flash`; ambos aceitos por `agy agent`.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-07-19-antigravity-agent-tools.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-19-antigravity-agent-tools.md
