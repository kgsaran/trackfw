---
status: Open
date: 2026-10-20
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md"
---

# REQ: Hooks de guard gerados falham sob zsh com exit 1 e deixam de executar a protecao

> Date: 2026-10-20 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
As constantes guard*CmdPSPOSIX em internal/generators/agentfiles.go:503–505 abortam sob zsh: exit 1, stderr `zsh:1: 2 not found`, antes de executar o guard. Mesmo payload benigno retorna 0 no Bash. Nove entradas instaladas afetadas (4 globais, 5 projeto). O runtime usa SHELL=/bin/zsh. A semente `=2` sofre expansão de comando pelo zsh; a matriz da ADR mediu sh/bash/PowerShell/cmd, sem zsh.

## Acceptance Criteria
- [ ] Hooks gerados executam sob zsh, sh, bash e runtimes Windows já suportados: allow=0, deny=2, binário ausente ou erro=2.
- [ ] Atualização migra entradas legadas sem duplicar nem remover hooks personalizados.
- [ ] Teste real por subprocess reproduz defeito original e comprova correção, preservando stdin JSON e motivo deny.
- [ ] Contrato, ADR e validação reconhecem forma corrigida; make quality passa.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md

## Escopo negativo

Não alterar políticas dos guards, desativar hooks, trocar shell do usuário, nem refatorar attention. Workaround local macOS usa Bash explícito; não é decisão de geração multiplataforma.
