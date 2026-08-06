---
status: Done
date: 2026-08-06
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-06-corrige-divergencia-de-versao-pypi-e-schema-legado-de-hooks-do-cursor.md"
---

# REQ: corrige divergencia de versao pypi e schema legado de hooks do cursor

> Date: 2026-08-06 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

Dois achados documentados como "fora de escopo" durante a auditoria do
`ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
(PR #141, mergeado), registrados em `docs/cli-parity.md` e no REQ/roadmap daquele ciclo, ainda sem
correção:

1. **Divergência de versão pré-existente bloqueia `make quality`/`make parity` de ponta a ponta.**
   `pypi/trackfw/__init__.py` tem fallback hardcoded `"6.3.1"` enquanto Go e Node.js já estão em
   `6.4.1` — `scripts/check-cli-parity.sh` falha nesse mismatch antes de alcançar os demais gates da
   cadeia `parity` (incluindo o `check-agent-hooks-parity.sh` novo). Não é possível confirmar
   `make quality` verde de ponta a ponta enquanto esse mismatch existir.
2. **Wiring legado de attention-signal/cleanup do Cursor usa um schema que não corresponde a nenhum
   evento real documentado.** `InjectCursorHooks` (Go: `internal/generators/agentfiles.go`; Node:
   `npm/src/generators/hooks.js`; Python: `pypi/trackfw/generators/hooks.py`) escreve
   `preToolUse`/`postToolUse` no nível raiz de `.cursor/hooks.json` — mas a documentação oficial
   (`cursor.com/docs/agent/hooks`, confirmada durante o ML-2E daquele ciclo) não lista nenhum evento
   genérico `preToolUse`/`postToolUse`; o schema real é `{"version":1,"hooks":{"<eventName>":[...]}}`
   com eventos `sessionStart`/`sessionEnd`/`beforeShellExecution`/`beforeMCPExecution`/
   `afterShellExecution`/`afterMCPExecution`/`beforeReadFile`/`afterFileEdit`/`beforeSubmitPrompt`/
   `preCompact`/`stop`/`beforeTabFileRead`/`afterTabFileEdit`. O mecanismo de attention-signal/cleanup
   para Cursor está, hoje, funcionalmente inerte em uso real — o Cursor real ignora essas chaves.

## Acceptance Criteria
- [x] `pypi/trackfw/__init__.py` alinhado à versão real do projeto (`6.4.1`, mesma fonte de verdade
      que `pyproject.toml`/Go/Node) e `check-cli-parity.sh` passando
- [x] `make quality`/`make parity` verdes de ponta a ponta — confirmado (102/102 cenários de
      falsificação, todos os gates de paridade, `trackfw validate` limpo)
- [x] Investigação registrada no roadmap sobre qual evento real do Cursor é o análogo correto para
      attention-signal/cleanup — achado: a documentação do Cursor mudou entre 2026-08-05 e 2026-08-06,
      passou a documentar `preToolUse`/`postToolUse`/`postToolUseFailure` como eventos genéricos reais
      (não existiam na investigação do ciclo anterior)
- [x] `.cursor/hooks.json` migrado para os eventos reais confirmados (`hooks.preToolUse`/
      `hooks.postToolUse`, nested), paridade Go/Node/Python mantida, sem quebrar o wiring de
      `beforeShellExecution`/`afterShellExecution` do credential-guard já existente (PR #141);
      migração automática de arquivos escritos por versões anteriores do trackfw
- [x] `trackfw validate` sem violações novas

## Linked ADR
<!-- nenhuma decisão de design não-trivial nova esperada; ambos os fixes são correção de bug/divergência
     já investigada no ciclo anterior — avaliar durante o roadmap se a investigação do evento real do
     Cursor revela algo que justifique ADR própria -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-06-corrige-divergencia-de-versao-pypi-e-schema-legado-de-hooks-do-cursor.md`
