---
status: Done
date: 2026-11-24
author: "zeus-tf (issue #705, decisão do KG em 2026-11-24)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-11-24-aviso-gemini-cli-legacy-sugere-comando-sem-scope-project.md"
---

# REQ: Aviso gemini_cli_legacy sugere comando sem --scope project

> Date: 2026-11-24 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
Issue #705 (consumidor externo). A regra `gemini_cli_legacy` (`internal/validator/validator_gemini_cli_legacy.go:48`)
descreve uma condição de projeto (`.gemini/settings.json` com guard e `.agents/hooks.json` ausente) e sugere
`trackfw agents install --targets antigravity` sem `--scope`. Sem TTY, o `--scope` padrão é `global` (`resolveScope`,
`internal/commands/integrations_flags.go`), que desde o #697 não escreve no projeto. Num CI, script ou agente,
seguir o aviso literalmente não resolve o que ele reclama. O `doctor` reusa a mesma regra. Censo do arquiteto em
`internal/`: é a única mensagem de regra que sugere `agents install` sem `--scope`.

## Acceptance Criteria
- [x] A mensagem sugere `trackfw agents install --targets antigravity --scope project`.
- [x] Um teste fixa o `--scope project` na mensagem (braço: reprova com a mensagem antiga). Evidência:
      `TestGeminiCliLegacy_GeminiGuardNoAntigravity_Warns`, rc=1 contra a mensagem antiga.
- [x] `docs/cli-parity.md` (~L10470) mostra a mensagem nova.
- [x] Censo das mensagens de regra de projeto que sugerem comando com escopo (`agents install`, `skills install`,
      `update`) registrado no Wave 0, com o resultado: sítio único; S2–S7 classificados como condição global ou
      escopo já explícito (`docs/seguranca/2026-11-24-wave0-correcoes-de-consumidor-704-705.md`).
- [x] `make quality` rc=0 e CI verde (PR #709: 24/24 checks, Windows incluído; merge `8b110e0e`).

## Negative scope
- Não muda o padrão de `--scope` nem o `resolveScope`.
- Não muda a condição da regra, só o texto do remédio.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-11-24-aviso-gemini-cli-legacy-sugere-comando-sem-scope-project.md
