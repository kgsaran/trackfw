---
status: Done
date: 2026-07-26
author: "KG"
adr: "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-27-robustez-dos-gates-de-governanca-e-paridade.md"
---

# REQ: robustez dos gates de governanca e paridade

> Date: 2026-07-26 | Status: Done

## Motivation

Três gates do trackfw falharam em duas REQs consecutivas (2026-07-26) e **nenhum foi pego pelo CI** —
todos apareceram em auditoria manual, em cenário real. O produto vende governança verificável; seus
gates são o produto, não infraestrutura acessória.

Os princípios estão no ADR vinculado. Esta REQ aplica-os aos defeitos concretos já mapeados e adiciona
cobertura para impedir reincidência.

Notas de vault com o diagnóstico completo:
- `vault/notes/branch_has_wip_roadmap-conflita-com-a-definition-of-done-2026-07-26.md`
- `vault/notes/argparse-ansi-parity-gate-python313-2026-07-26.md`

## Escopo

1. **`branch_has_wip_roadmap` aceita roadmap em `done/`** cujo slug case com a branch, resolvendo a
   contradição com a Definition of Done. Reprova apenas quando não há roadmap correspondente em `wip/`
   **nem** em `done/`. Reaproveitar `normalizeBranchSlug`. Nos 3 CLIs.
2. **Auditoria dos demais gates contra os princípios P1–P3** — varrer `scripts/*.sh` e as regras do
   validator procurando: número mágico, degradação silenciosa e dependência de ambiente. Corrigir o
   que for encontrado; listar no roadmap o que foi verificado e considerado conforme.
3. **Testes de falsificação para os gates existentes** (P4): cada script de paridade ganha um teste
   que monta o cenário negativo e prova que o gate reprova. Hoje nenhum tem.
4. **Documentar os princípios** em `docs/cli-parity.md` ou documento próprio, para que quem escrever
   o próximo gate os siga.

## Escopo negativo (o que NÃO fazer)

- **Não** reescrever os gates que já foram corrigidos nas REQs anteriores
  (`check-integration-cli-parity.sh` e `check-cli-parity.sh`) — apenas cobri-los com teste de
  falsificação.
- **Não** criar framework de testes de gate: usar o mecanismo de teste já existente em cada CLI.
- **Não** alterar a semântica de nenhuma regra de governança além do `branch_has_wip_roadmap`.
- **Não** rebaixar severidade de regra alguma para fazer gate passar.
- **Não** introduzir dependência nova (biblioteca de cor, framework de shell test).
- **Não** mexer no comando `ship` nem no pacote `internal/forge` — REQ separada, em andamento.

## Acceptance Criteria

- [ ] Roadmap em `done/` com slug da branch **não** dispara `branch_has_wip_roadmap` nos 3 CLIs
- [ ] Branch de feature sem roadmap em `wip/` nem `done/` **continua** disparando a violação
- [ ] Slug que não casa com a branch continua disparando, mesmo com roadmap em `done/`
- [ ] Encerrar um roadmap na própria branch deixa `trackfw validate` verde
- [ ] Cada script de paridade tem teste de falsificação provando que reprova o cenário negativo
- [ ] Varredura de P1–P3 concluída, com a lista do que foi verificado registrada no roadmap
- [ ] Princípios de design de gate documentados
- [ ] `make quality` verde nos 3 CLIs, **sem** variável de ambiente auxiliar

## Linked ADR

ADR: docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Roadmap será criado quando esta REQ entrar em execução -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-27-robustez-dos-gates-de-governanca-e-paridade.md
