---
status: Open
date: 2026-09-13
author: "claude"
adr: "docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md"
roadmap: "docs/roadmaps/claude/wip/ROADMAP-2026-09-13-upstream-sync-recompila-tambem-o-bin-trackfw-exe-no-windows.md"
---

# REQ: upstream-sync recompila também o bin/trackfw.exe no Windows

> Date: 2026-09-13 | Status: Open

## Motivation

O `scripts/upstream-sync.sh` constrói o binário da árvore em dois pontos — o baseline do `validate`
(linha 80) e a reconstrução pós-merge (linha 151) — e nos dois escreve só `bin/trackfw`. No Windows o
`go build -o bin/trackfw` grava exatamente esse nome, sem extensão, e o `bin/trackfw.exe` **nunca é
atualizado por nenhum caminho nosso**.

Medido em 2026-09-13:

| ponto de entrada | versão depois do sync do #356 |
|---|---|
| `bin/trackfw` (bash) | `8.0.0-rc1` |
| `bin/trackfw.exe` | `7.6.0` |
| `./bin/trackfw` no **PowerShell** | `7.6.0` — resolve para o `.exe` |

O custo já foi pago no mesmo dia: um `bin/trackfw.exe` **v2.12.2 de 2026-08-28** fez o
`scripts/smoke-integration-packages.sh` reprovar com `unknown flag: --targets`, porque no Windows ele
empacota o `.exe`. Parecia defeito do upstream; era binário velho. Recompilado à mão, voltou a ficar
uma versão atrás no sync seguinte.

## Acceptance Criteria

- [ ] **AC1** — no Windows, depois de um `upstream-sync.sh` que traz commit, `bin/trackfw.exe version`
      é igual a `bin/trackfw version`. Medido por efeito num worktree atrás do upstream, **com
      controle** sem a mudança.
- [ ] **AC2** — sem predicado de SO: a extensão vem do toolchain (`go env GOEXE`). Onde `GOEXE` é
      vazio (Linux, macOS), nenhum build a mais acontece.
- [ ] **AC3** — os dois sítios de build do script cobertos: baseline e pós-merge.
- [ ] **AC4** — `scripts/check-upstream-sync-falsify.sh` continua `OK`, com o limite declarado: ele
      roda o sync com `--skip-verify` e **não exercita os builds** — não é prova do AC1.
- [ ] **AC5** — `trackfw validate` com 0 violações e `scripts/run-local-gates.sh` com 0 falhas.

## Escopo negativo

- **Não** mexe no `Makefile`: é arquivo do upstream, e o `make build` que só escreve `bin/trackfw` fica
  como residual declarado.
- **Não** mexe em workflow: todos compilam em runner descartável do CI, não deixam `.exe` nesta máquina.
- **Não** muda o `trackfw` do PATH (é `npm link`, outro binário).
- **Não** mexe no `scripts/run-local-gates.sh`: ele não compila, só exige que `bin/trackfw` exista.

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/claude/wip/ROADMAP-2026-09-13-upstream-sync-recompila-tambem-o-bin-trackfw-exe-no-windows.md
