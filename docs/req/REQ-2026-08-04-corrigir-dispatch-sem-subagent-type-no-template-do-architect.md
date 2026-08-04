---
status: Done
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect.md"
---

# REQ: corrigir dispatch sem subagent_type no template do Architect

> Date: 2026-08-04 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Reportado pelo usuário: em uma sessão real como `zeus-tf`, o Zeus nomeou especialistas ("Artemis",
"Afrodite") na prosa e no campo `squad:` dos roadmaps, mas ao chamar a Agent tool não passou o
parâmetro técnico `subagent_type` — o harness caiu silenciosamente no default `general-purpose`
(agente genérico sem instruções de domínio), quebrando o modelo de orquestração multi-agente do
trackfw mesmo com o prompt textualmente correto.

Causa raiz: o template canônico do agente Architect (`internal/integrations/assets/agents/architect.md`,
espelhado byte-a-byte em `npm/src/integrations/assets/agents/architect.md` e
`pypi/trackfw/integrations/assets/agents/architect.md`) instrui "dispatch the wave" e "delegate to the
appropriate specialist", mas nunca menciona que é obrigatório passar `subagent_type` explicitamente,
nem como descobrir o valor correto — que depende da identidade configurada pelo usuário
(`~/.trackfw/identity.json`; presets grego/nórdico/HP ou nomes customizados, sempre com sufixo fixo
`-tf`, conforme `docs/cli-parity.md` § Agent identity).

## Acceptance Criteria
- [x] `internal/integrations/assets/agents/architect.md` ganha uma seção explícita instruindo que
      nomear um especialista na prosa/`squad:` não roteia a chamada, que `subagent_type` é obrigatório
      em todo dispatch, e que o valor correto é o `name:` do frontmatter do agente instalado daquele
      role (nunca um nome fixo — a identidade é configurável por preset/custom)
- [x] `npm/src/integrations/assets/agents/architect.md` e `pypi/trackfw/integrations/assets/agents/architect.md`
      permanecem byte-idênticos ao canônico (via `scripts/sync-integration-assets.sh`)
- [x] Goldens `internal/integrations/testdata/architect.subagent.golden.md` e
      `architect.agent-directory.golden.md` refletem o novo conteúdo
- [x] `go build ./...`, `go test ./internal/integrations/...` e
      `bash scripts/check-integration-assets.sh` passam
- [x] Nenhuma menção a `subagent_type` vaza para templates de superfícies não-Claude-Code (Gemini,
      Copilot, Windsurf, Codex) — esse parâmetro não existe fora do Claude Code

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: N/A — correção de conteúdo de template, sem decisão arquitetural nova.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect.md`
