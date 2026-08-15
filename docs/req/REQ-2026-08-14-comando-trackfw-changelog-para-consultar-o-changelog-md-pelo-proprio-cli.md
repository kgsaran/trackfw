---
status: Done
date: 2026-08-14
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-14-comando-trackfw-changelog-para-consultar-o-changelog-md-pelo-proprio-cli.md"
---

# REQ: comando trackfw changelog para consultar o CHANGELOG.md pelo proprio CLI

> Date: 2026-08-14 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Hoje `CHANGELOG.md` é um arquivo Markdown estático na raiz do repo — não há nenhum
comando `trackfw` que o leia ou formate. Confirmado por busca: nenhum comando lista
"changelog" no `--help`, nenhum arquivo em `internal/commands/` referencia
`CHANGELOG.md`. Consultar hoje exige abrir o arquivo manualmente (editor, `cat`, GitHub).

Um comando `trackfw changelog` daria paridade com o padrão já usado por `trackfw
status`/`trackfw context` (informação do projeto acessível via CLI, sem sair do
terminal), e seria especialmente útil logo após `trackfw update`, quando o usuário quer
saber rapidamente "o que mudou" sem procurar o arquivo manualmente.

## Acceptance Criteria
- [x] Novo comando `trackfw changelog` (Go/Node/Python, paridade completa) lê
      `CHANGELOG.md` da raiz do projeto e imprime a seção mais recente (`## [x.y.z]` ou
      `## [Unreleased]`, o que vier primeiro no arquivo) formatada no terminal.
- [x] Flag `--version <x.y.z>` (ou posicional) imprime uma versão específica, se
      existir no arquivo; erro claro se a versão não for encontrada.
- [x] Flag `--all`/`--full` imprime o arquivo inteiro.
- [x] Parsing deve ser tolerante ao formato real do `CHANGELOG.md` deste projeto (Keep a
      Changelog: `## [x.y.z] - YYYY-MM-DD`, seções `### Added`/`### Fixed`/etc.) — não
      assumir um schema mais rígido do que o que já existe no arquivo.
- [x] Comportamento idêntico nos 3 CLIs (mesma seção extraída, mesma formatação de
      saída) — mensagens de erro (arquivo ausente, versão não encontrada) byte-idênticas.
- [x] `make quality` passa sem novas divergências de paridade.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-14-comando-trackfw-changelog-para-consultar-o-changelog-md-pelo-proprio-cli.md
