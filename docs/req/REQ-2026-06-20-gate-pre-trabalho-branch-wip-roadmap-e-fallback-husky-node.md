---
status: Open
date: 2026-06-20
author: Zeus
adr: ""
roadmap: "ROADMAP-2026-06-20-gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node.md"
---

# REQ: gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node

> Date: 2026-06-20 | Status: Open

## Motivation

Agentes de IA (e usuários humanos) iniciam trabalho em branches `feat/*`, `fix/*` e `refactor/*`
sem criar previamente os artefatos obrigatórios de governança (REQ + Roadmap em wip).
O `trackfw validate` existente não detecta essa ausência porque só valida artefatos que existem —
quando `wip/` está vazio, todos os checks passam silenciosamente.

Adicionalmente, em ambientes Windows corporativos com restrições de rede, o `lefthook` não consegue
ser instalado, mas Node.js está disponível. O trackfw precisa detectar essa condição e usar husky
automaticamente.

## Acceptance Criteria
- [ ] `trackfw validate` falha com violation `branch_has_wip_roadmap` ao rodar em branch feat/fix/refactor sem nenhum roadmap em wip/
- [ ] A regra é configurável (off/warning/error) via `trackfw.yaml`
- [ ] A regra é implementada nos 3 CLIs (Go, Node.js, Python)
- [ ] `trackfw init` e `trackfw discover --init` detectam Node.js no PATH e usam husky quando lefthook não está disponível
- [ ] O bloco de regras dos agentes (`trackfwRulesBlock`) inclui a instrução do protocolo REQ→Roadmap→branch
- [ ] `trackfw update` propaga as novas regras para todos os agentes configurados
- [ ] Todos os testes existentes continuam verdes

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: ROADMAP-2026-06-20-gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node.md
