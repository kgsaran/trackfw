---
status: Open
date: 2026-11-24
author: "zeus-tf (decisão do KG em 2026-11-24)"
adr: "docs/adr/ADR-2026-11-24-guards-resolvem-trackfw-yaml-de-forma-monotonica-subagente-nao-cria-raiz-nem-afrouxa-regra.md"
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-11-24-guard-de-write-scope-nao-cobre-multiedit-no-claude-code-nem-o-opencode.md"
---

# REQ: Guard de write-scope não cobre MultiEdit no Claude Code nem o OpenCode

> Date: 2026-11-24 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
A REQ-2026-11-23 fechou a re-raiz pelo `trackfw.yaml` aninhado para as ferramentas de escrita que o write-scope
intercepta. O ADR-2026-11-24 declarou dois residuais de **outra causa**, que é a cobertura de ferramenta e não a
resolução de raiz. Por decisão do KG (2026-11-24), eles ganham REQ própria:

- **R-5 (`MultiEdit`):** o matcher do write-scope no Claude Code não inclui `MultiEdit`. Um teste público de 2026
  indica que as versões atuais não emitem mais essa ferramenta, mas não há confirmação oficial. Se ela ainda existir,
  é uma escrita que o guard não vê.
- **R-6 (OpenCode):** o write-scope não é instalado no OpenCode (T13 do ROADMAP-2026-11-17). Além disso, os nomes de
  ferramenta do OpenCode são minúsculos (`write`, `edit`, `patch`), então um matcher copiado do Claude Code não casaria.

Varredura em 2026-11-24: nenhuma issue aberta com o mesmo mecanismo. A REQ-2026-08-04 (compatibilidade com OpenCode)
está Done e não cobre o write-scope.

## Acceptance Criteria
- [ ] Medido se o Claude Code atual emite `MultiEdit`, com a versão e o payload registrados. Se emitir, o matcher do
      write-scope a cobre, com teste de braço (antes rc=0, depois rc=2) e contra-braço.
- [ ] O OpenCode recebe o write-scope pelo plugin existente, cobrindo `write`, `edit` e `patch`/`apply_patch`, com os
      caminhos extraídos do patch pela mesma regra do guard de credencial. Cada vetor do AC1 da Wave 0 da
      REQ-2026-11-23 é aplicável ao OpenCode, com braço e contra-braço.
- [ ] `docs/cli-parity.md` e o ADR-2026-11-24 deixam de listar R-5 e R-6 como residuais.

## Negative scope
- Não trata escrita por shell (Bash), que é o residual do ADR-2026-10-23 e outra causa.
- Não muda D1–D6 do ADR-2026-11-24 nem a resolução de raiz.
- Não cobre outros agentes (Codex, Gemini, Antigravity, Windsurf) além do OpenCode e do Claude Code.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-24-guards-resolvem-trackfw-yaml-de-forma-monotonica-subagente-nao-cria-raiz-nem-afrouxa-regra.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-11-24-guard-de-write-scope-nao-cobre-multiedit-no-claude-code-nem-o-opencode.md
