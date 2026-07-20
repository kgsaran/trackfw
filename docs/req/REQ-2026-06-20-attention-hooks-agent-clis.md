---
status: Closed
date: 2026-06-20
author: Zeus
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-06-20-attention-hooks-agent-clis.md"
---

# REQ: attention-hooks-agent-clis

> Date: 2026-06-20 | Status: Closed

## Motivation

O mecanismo de sinalização de atenção do `trackfw serve` (`.trackfw-attention.json`) não é acionado
na prática porque os agentes de IA (Claude, Codex, Gemini, Kiro, Copilot, Cursor, Windsurf) ignoram
a instrução textual de escrever o arquivo antes de interagir com o usuário.

A solução é integrar o mecanismo via **hooks nativos** de cada CLI: o arquivo de atenção é escrito
automaticamente pelo hook, antes do tool call que aciona a interação — sem depender da memória ou
comportamento do agente.

Pesquisa realizada (2026-06-20) confirma que 7 dos 8 CLIs suportados têm `PreToolUse` (ou
equivalente), e que o Windsurf é o único outlier (hooks por tipo de ação, sem hook genérico).

## Acceptance Criteria

- [x] Claude Code: hook `PreToolUse[AskUserQuestion]` gerado em `.claude/settings.json` que escreve
      `.trackfw-attention.json` antes da pergunta e o apaga via `PostToolUse[AskUserQuestion]`
- [x] Codex CLI: hook `PermissionRequest` + `PostToolUse` gerado em `.codex/hooks.json`
- [x] Gemini CLI: hook `Notification[ToolPermission]` + `AfterTool` gerado em `.gemini/settings.json`
- [x] Kiro: hook `PreToolUse` + `PostToolUse` gerado em `.kiro/hooks/trackfw-attention.json`
- [x] GitHub Copilot: hook `preToolUse` + `postToolUse` gerado em `.github/hooks/trackfw-attention.json`
- [x] Cursor: hook `preToolUse` + `postToolUse` gerado em `.cursor/hooks.json`
- [x] Windsurf: instrução explícita no `.windsurfrules` (sem hook confiável para perguntas ao usuário)
- [x] Script `scripts/trackfw-attention-signal.sh` gerado por `trackfw init`/`discover --init`
- [x] Script `scripts/trackfw-attention-cleanup.sh` gerado por `trackfw init`/`discover --init`
- [x] `trackfw update` regenera/atualiza os hook configs detectados
- [x] Paridade nos 3 CLIs (Go, Node.js, Python)
- [x] Todos os testes existentes continuam verdes

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-06-20-attention-hooks-agent-clis.md
