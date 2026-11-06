---
status: Open
date: 2026-11-06
author: "Zeus (trackfw_architect)"
adr: "docs/adr/ADR-2026-11-06-ci-vermelho-por-infraestrutura-se-distingue-de-vermelho-por-conteudo.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-fila-do-macos-no-gate-do-shim-prazo-do-powershell-5-1-e-porta-fixa-no-serve.md"
---

# REQ: CI vermelho sem relação com o conteúdo: fila do macOS no gate do shim, prazo do PowerShell 5.1 e porta fixa no serve

> Date: 2026-11-06 | Status: Open

## Motivation

Os relatos de lourivalgarciajunior (Discussion #664 e issue #665) e a falha de porta do PR #666 têm o mesmo efeito: CI vermelho sem defeito no conteúdo. Esse vermelho trava a fila de merge de quem consome o trackfw e ensina a ignorar vermelho. As causas são três e estão no ADR.

## Acceptance Criteria

- [ ] **AC1 (#664).** Num PR cujo diff só toca caminhos do conjunto inerte, o `shim-byte-identity-gate` passa mesmo com o braço macOS `cancelled` ou `skipped`, e escreve `não aplicável` com a lista de arquivos no log. Num PR com qualquer arquivo fora do conjunto, inclusive arquivo novo, desconhecido ou renomeado, o gate continua exigindo `success` dos dois braços. No `push` e na tag, também. Se o diff não puder ser calculado, o gate exige `success`. Como a classificação usa o script da BASE (emenda D1 do ADR), a dispensa só dispara em PR aberto depois do merge; antes do merge, a lógica é verificada pelo autoteste e pela leitura do workflow, e o primeiro PR só de documentação contra a `main` depois do merge é a medição real.
- [ ] **AC2 (#664).** A classificação do diff é um script com autoteste que roda no CI. O autoteste cobre os dois sentidos:
  - diff só de caminhos inertes dá não aplicável;
  - dá aplicável: um arquivo em `internal/`, `npm/` ou `cmd/`, um arquivo na raiz que não é `.md`, um caminho novo desconhecido, um rename de inerte para fora e de fora para inerte, e base ausente.
- [ ] **AC3 (#665).** O prazo da chamada afirmada do PowerShell é `clamp(4 × aquecimento, 15 s, 60 s)`. Um teste unitário da função de prazo fixa os três regimes. A mensagem de estouro cita o aquecimento e o prazo. Depois do primeiro estouro de um binário, as chamadas seguintes desse binário reprovam sem executar.
- [ ] **AC4.** O `check-serve-address-parity.sh` usa porta livre do sistema e tenta de novo só em `address already in use`, até 3 vezes. Com a porta ocupada de propósito, o cenário passa; com outro erro de bind, reprova.
- [ ] **AC5.** `make quality` verde, CI verde no PR, e a Discussion #664 e a issue #665 recebem resposta com o que mudou.

## Negative Scope

- Não muda o conjunto R de required checks nem o nome do `shim-byte-identity-gate`. Emenda o AC9 da REQ-2026-09-12-v8 só no que o ADR D1 declara.
- Não usa `continue-on-error` nem aceita `cancelled` como sucesso quando o diff toca o shim.
- Não tira o macOS da matriz nem troca `macos-latest` por outro rótulo.
- Não muda o ratchet do Windows nem o `.github/windows-known-failures.json`.
- Não muda o comportamento do `trackfw serve` nem do guard; só instrumentos de teste e CI.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-06-ci-vermelho-por-infraestrutura-se-distingue-de-vermelho-por-conteudo.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-fila-do-macos-no-gate-do-shim-prazo-do-powershell-5-1-e-porta-fixa-no-serve.md
