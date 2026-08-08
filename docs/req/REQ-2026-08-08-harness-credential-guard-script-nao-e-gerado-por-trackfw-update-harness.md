---
status: Done
date: 2026-08-08
author: ""
adr: ""
roadmap: ""
---

# REQ: harness credential-guard script não é gerado por trackfw update harness

> Date: 2026-08-08 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
`trackfw update harness` escreve o wiring de hooks (`claude-credential-guard`,
`codex-credential-guard`, `gemini-credential-guard`, `cursor-credential-guard`,
`copilot-credential-guard`, `kiro-credential-guard`) apontando para
`~/.trackfw/scripts/trackfw-credential-guard.sh` nos 3 CLIs (Go/Node.js/Python),
mas nenhum deles chamava a função geradora do script
(`GenerateGlobalCredentialGuardScript`/`generateGlobalCredentialGuardScript`/
`generate_global_credential_guard_script`, criada no ML-1A de
ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md).
Resultado: usuário reporta hooks PreToolUse/PostToolUse:Bash falhando com
"No such file or directory" logo após rodar `trackfw update harness` — o
wiring é instalado apontando para um arquivo que nunca é materializado.
Bug reportado diretamente pelo usuário (kg.saran@gmail.com) em 2026-08-08;
causa raiz confirmada por leitura de código nos 3 stacks (função existente e
testada isoladamente, porém nunca invocada pelo fluxo real de
`update harness`).

## Acceptance Criteria
- [x] `UpdateHarness`/`run()`/`_run()` (Go, Node.js, Python) chamam a função
      geradora do script global antes de aplicar o wiring por-CLI, exceto em
      `--dry-run`
- [x] `~/.trackfw/scripts/trackfw-credential-guard.sh` existe, é executável e
      roda sem erro após `trackfw update harness`
- [x] Testes Go/Node.js/Python de `credential_guard`/`update_harness`
      permanecem verdes (nenhuma falha nova introduzida)

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md (fecha lacuna de implementação da decisão #2/#3 já aceita)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-08-harness-credential-guard-script-nao-e-gerado-por-trackfw-update-harness.md
