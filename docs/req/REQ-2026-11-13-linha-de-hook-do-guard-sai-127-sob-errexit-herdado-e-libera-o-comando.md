---
status: Open
date: 2026-11-13
author: ""
adr: "docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md"
---

# REQ: linha de hook do guard sai 127 sob errexit herdado e libera o comando

> Date: 2026-11-13 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
A linha de hook gerada pelo trackfw (família PS/POSIX, ADR-2026-10-04) é poliglota: em sh/bash/dash,
`emulate` e `$LASTEXITCODE=2` são comandos inexistentes por desenho. Com `errexit` ou `nounset` herdados, o
shell sai 127 no primeiro desses pontos, antes de chamar `trackfw guard`. O CLI de agente lê 127 como erro
não bloqueante, e o comando passa: o guard de credencial e o de branch ficam desligados sem aviso.

Medido em 2026-11-13 (macOS, guard falso que nega com 2): `bash -e`, `bash -u`, `sh -e`, `sh -u`, `dash -e` e
`bash` com `BASH_ENV` contendo `set -e` dão 127. `bash`, `zsh -e`, `zsh -u` e `SHELLOPTS=errexit` dão 2.

Origem: resíduo R4 declarado e não corrigido na REQ-2026-10-20 (roadmap `done/`, linha 246). A varredura de
issues de 2026-11-13 não achou issue aberta com o mesmo mecanismo.

Fora desta REQ, por decisão medida: `GIT_TERMINAL_PROMPT=0` nas chamadas git de rede. Sem terminal o git falha
em 0 s (`vault/notes/git-terminal-prompt-sem-terminal-falha-na-hora-2026-10-31.md`, decisão do KG de
2026-10-31); não há defeito a corrigir.

### Negative scope
- Não muda o subcomando `trackfw guard` nem as regras de bloqueio dele.
- Não muda a família POSIX-only nem a nativa de Windows, salvo se a Wave 0 medir o mesmo defeito nelas (aí
  entram como ML nesta REQ).
- Não acrescenta gate estático novo (direção de 2026-10-28); a regressão fica na matriz de testes existente.
- Não trata `GIT_TERMINAL_PROMPT` nem prompts de SSH ou credential helper gráfico.

## Acceptance Criteria
- [ ] AC1 — inventário de toda forma de linha de hook gerada ou aceita como corrente, com o rc medido sob cada opção herdada (Wave 0)
- [ ] AC2 — com guard que nega (2), a linha nova sai 2 sob `bash -e`, `bash -u`, `sh -e`, `sh -u`, `dash -e`, `zsh -e`, `zsh -u`, `BASH_ENV` com `set -e` e `set -u`, e sem opção nenhuma
- [ ] AC3 — com guard que permite (0), a linha nova sai 0 nas mesmas condições
- [ ] AC4 — PowerShell 5.1 e 7 no Windows: deny 2 e allow 0, com o ruído de stderr medido e não maior que o da forma atual
- [ ] AC5 — `TestGuardLineShellMatrix` cobre as opções herdadas do AC2, e a forma atual reprova nele (contra-braço)
- [ ] AC6 — a forma atual vira geração legada no registro da D12: `validate` avisa e `trackfw update harness` migra sem duplicar entrada
- [ ] AC7 — configs de hook deste repositório migradas para a forma nova
- [ ] AC8 — `make quality` verde

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
