---
status: Done
date: 2026-07-18
author: "Codex"
adr: "docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md"
---

# REQ: Catálogo unificado de agents e skills multi-CLI

> Date: 2026-07-18 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

Transformar `trackfw agents` e `trackfw skills` em gerenciadores completos e
seguros, com descoberta do catálogo, instalação seletiva por CLI, atualização e
remoção. O comportamento deve ser equivalente nos pacotes Go/Homebrew, npm e PyPI
e usar o formato nativo de cada assistente suportado.

## Acceptance Criteria
- [x] `agents` e `skills` expõem `list`, `install`, `uninstall` e `update` nos três runtimes.
- [x] `list` mostra todos os itens do catálogo e o estado por CLI, com saída humana e `--json` equivalente.
- [x] `install` oferece multiseleção de CLIs e itens em TTY e flags determinísticas fora de TTY.
- [x] Claude, Codex, Gemini, Antigravity, Cursor, Copilot, Windsurf, Amazon Q e Kiro usam adapters nativos.
- [x] `update` preserva arquivos modificados, salvo `--force`, e atualiza somente ownership trackfw.
- [x] `uninstall` remove somente arquivos comprovadamente gerenciados pelo trackfw.
- [x] Instalações legadas reconhecidas são adotadas sem duplicar artefatos.
- [x] Assets empacotados por Go, npm e PyPI são byte-identical/hash-identical.
- [x] Comandos antigos de ferramenta continuam funcionando como aliases compatíveis.
- [x] `make quality`, testes npm, testes Python, package smokes e `trackfw validate` passam.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md
