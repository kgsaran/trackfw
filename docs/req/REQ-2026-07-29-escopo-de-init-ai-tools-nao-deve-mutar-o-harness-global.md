---
status: Open
date: 2026-07-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md"
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-07-29-escopo-de-init-ai-tools-nao-deve-mutar-o-harness-global.md"
---

# REQ: Escopo de init --ai-tools nao deve mutar o harness global

> Date: 2026-07-29 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

`trackfw init --ai-tools <tool>`, executado dentro de um projeto, grava em `~/.gemini/agents/`.
Constatado empiricamente durante a validação da Wave 5 do roadmap da barrier: a execução falhou com
`artifact "/Users/<user>/.gemini/agents/trackfw-architect.md" is outdated; use update`, provando que
o comando alcança o HOME do usuário.

É a mesma classe de defeito que o ADR vinculado corrigiu em `trackfw update`: um comando de escopo
de projeto mutando o harness global. O contrato de escopo em `docs/cli-parity.md` cobre `update` e
`update harness`; `init` ficou fora.

Extraído do roadmap da barrier em vez de inflá-lo — aquele roadmap já cresceu de 9 para 21 MLs, e
este defeito está fora da REQ que o originou.

## Acceptance Criteria
- [ ] `trackfw init --ai-tools <tool>` não escreve fora do diretório do projeto.
- [ ] Instalação global passa a exigir escopo explícito, coerente com `trackfw update harness`.
- [ ] Teste com HOME isolado prova a ausência de escrita global nos três runtimes.
- [ ] Gate encadeado no alvo `parity` prova a ausência de escrita global.
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-07-29-escopo-de-init-ai-tools-nao-deve-mutar-o-harness-global.md`
