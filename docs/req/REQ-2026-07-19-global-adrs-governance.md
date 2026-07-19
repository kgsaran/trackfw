---
status: Open
date: 2026-07-19
author: "Zeus"
adr: "docs/adr/ADR-2026-07-19-global-adrs-governance.md"
roadmap: ""
---

# REQ: Suporte a ADRs Globais Compartilhados e Diretivas de IA

> Date: 2026-07-19 | Status: Open | Blocked by ADRs: 1
| Linear Issue:
| Jira Issue:

## Motivation

Habilitar a centralização de guias de estilo de arquitetura e desenvolvimento compartilhados por múltiplos projetos da empresa, instruindo e forçando ativamente os assistentes de IA a lerem tais especificações globais fora do repositório local.

## Acceptance Criteria

- [ ] Suporte à expansão de til (`~`) no carregamento de `adr_dirs` do `trackfw.yaml` nas linguagens Go, Node.js e Python.
- [ ] Validador não falha builds de CI/CD se um diretório externo configurado não existir no runner.
- [ ] Regra de validação `adr_orphan` ignora arquivos contidos fora da raiz do projeto local.
- [ ] Geradores do comando `trackfw init` injetam a diretiva obrigatória de leitura de ADRs globais nos arquivos `CLAUDE.md` e `AGENTS.md`.
- [ ] Testes de conformidade de caminhos implementados e verdes nas três distribuições do framework.

## Linked ADR

ADR: docs/adr/ADR-2026-07-19-global-adrs-governance.md

## Blocked by ADRs

- ADR-2026-07-19-global-adrs-governance.md (Draft)

## Linked Roadmap

Roadmap:
