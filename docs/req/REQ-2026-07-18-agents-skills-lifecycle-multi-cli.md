---
status: Open
date: 2026-07-18
author: "Codex"
adr: "docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md"
---

# REQ: Catálogo unificado de agents e skills multi-CLI

> Date: 2026-07-18 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Transformar `trackfw agents` e `trackfw skills` em gerenciadores completos e
seguros, com descoberta do catálogo, instalação seletiva por CLI, atualização e
remoção. O comportamento deve ser equivalente nos pacotes Go/Homebrew, npm e PyPI
e usar o formato nativo de cada assistente suportado.

## Acceptance Criteria
- [ ] `agents` e `skills` expõem `list`, `install`, `uninstall` e `update` nos três runtimes.
- [ ] `list` mostra todos os itens do catálogo e o estado por CLI, com saída humana e `--json` equivalente.
- [ ] `install` oferece multiseleção de CLIs e itens em TTY e flags determinísticas fora de TTY.
- [ ] Claude, Codex, Gemini, Antigravity, Cursor, Copilot, Windsurf, Amazon Q e Kiro usam adapters nativos.
- [ ] `update` preserva arquivos modificados, salvo `--force`, e atualiza somente ownership trackfw.
- [ ] `uninstall` remove somente arquivos comprovadamente gerenciados pelo trackfw.
- [ ] Instalações legadas reconhecidas são adotadas sem duplicar artefatos.
- [ ] Assets empacotados por Go, npm e PyPI são byte-identical/hash-identical.
- [ ] Comandos antigos de ferramenta continuam funcionando como aliases compatíveis.
- [ ] `make quality`, testes npm, testes Python, package smokes e `trackfw validate` passam.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md
