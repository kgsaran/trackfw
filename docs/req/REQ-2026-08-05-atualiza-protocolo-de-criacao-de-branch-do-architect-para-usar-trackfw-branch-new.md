---
status: Open
date: 2026-08-05
author: ""
adr: ""
roadmap: ""
---

# REQ: atualiza protocolo de criação de branch do Architect para usar trackfw branch new

> Date: 2026-08-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
`trackfw branch new <type>/<slug>` (v6.4.0, REQ/roadmap
`ROADMAP-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`)
foi criado exatamente para mover o gate `branch_has_wip_roadmap` para *antes* da criação da branch,
em vez de só validar depois via `trackfw validate`. Mas o template canônico do agente Architect
(`internal/integrations/assets/agents/architect.md`, espelhado em `npm/`/`pypi/` e deployado como
`~/.claude/agents/trackfw-architect.md` via `trackfw update harness`) ainda descreve o protocolo de
Git authority com o comando cru: "creates the branch (`git checkout -b`)". Isso significa que todo
orquestrador (Zeus/Architect) rodando com o template atual **nunca chega a usar o comando que
acabamos de construir** — ele descreve exatamente o fluxo antigo que `branch new` foi criado para
substituir. Achado durante uma conversa sobre se os subagentes ou o orquestrador usam `branch new`
hoje: subagentes nunca criam branch (regra à parte, inalterada); o orquestrador tecnicamente
também não está instruído a usá-lo, apesar do comando já existir e estar publicado desde v6.4.0.

## Acceptance Criteria
- [ ] `internal/integrations/assets/agents/architect.md` instrui o uso de `trackfw branch new
      <type>/<slug>` como forma preferencial de criar a branch, com fallback documentado para
      `git checkout -b` quando o comando `trackfw branch new` não estiver disponível (versão antiga
      do binário instalado, ou ambiente sem trackfw) — não pode travar o fluxo do orquestrador se o
      comando não existir
- [ ] Mudança replicada byte-a-byte (ou semanticamente equivalente, se o texto precisar de ajuste
      local) em `npm/src/integrations/assets/agents/architect.md` e
      `pypi/trackfw/integrations/assets/agents/architect.md`, se esses arquivos existirem
      separadamente — ou confirmado que são o mesmo arquivo sincronizado via
      `scripts/sync-integration-assets.sh`
- [ ] `make quality`/`make parity` verdes, `trackfw validate` sem violações
- [ ] Testado manualmente: `trackfw update harness --targets claude-agents` local aplica o novo
      texto em `~/.claude/agents/trackfw-architect.md`

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
