---
status: Done
date: 2026-09-13
author: "claude"
adr: "docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md"
roadmap: "docs/roadmaps/claude/done/ROADMAP-2026-09-13-upstream-sync-recompila-tambem-o-bin-trackfw-exe-no-windows.md"
---

# REQ: upstream-sync recompila também o bin/trackfw.exe no Windows

> Date: 2026-09-13 | Status: Done

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

- [x] **AC1** — no Windows, depois de um `upstream-sync.sh` que traz commit, `bin/trackfw.exe version`
      é igual a `bin/trackfw version`. Medido por efeito num worktree atrás do upstream, **com
      controle** sem a mudança.
- [x] **AC2** — sem predicado de SO: a extensão vem do toolchain (`go env GOEXE`). Onde `GOEXE` é
      vazio (Linux, macOS), nenhum build a mais acontece.
- [x] **AC3** — os dois sítios de build do script cobertos: baseline e pós-merge.
- [x] **AC4** — `scripts/check-upstream-sync-falsify.sh` continua `OK`, com o limite declarado: ele
      roda o sync com `--skip-verify` e **não exercita os builds** — não é prova do AC1.
- [x] **AC5** — `trackfw validate` com 0 violações e `scripts/run-local-gates.sh` com 0 falhas.

## Evidência — 2026-09-13

**AC1 — medido por efeito, com controle.** Dois worktrees em `9f4c0f1` (2 commits atrás de
`upstream/main`), `bin/` vazio, o mesmo sync nos dois:

| worktree | script | `bin/trackfw` | `bin/trackfw.exe` | `./bin/trackfw` no PowerShell |
|---|---|---|---|---|
| controle | `upstream-sync.sh` antigo | `8.0.0-rc1` | **AUSENTE** | não resolve |
| mudança | `upstream-sync.sh` desta branch | `8.0.0-rc1` | **`8.0.0-rc1`** | **`8.0.0-rc1`** |

Os dois syncs trouxeram o mesmo conteúdo (`go build` exit 0, `validate` 0 antes · 0 depois); a única
diferença é o `.exe`. Confirmado antes de rodar que o sync opera no cwd, não no caminho do próprio
script — é o que permite rodar o script da branch dentro do worktree.

**AC2 e AC3 — no diff.** A extensão vem de `GOEXE_EXT="$(go env GOEXE)"`; o build extra é
`[ -z "$GOEXE_EXT" ] || go build -o "bin/trackfw$GOEXE_EXT" ... || die`, nos dois sítios — baseline
(linhas 84–85) e pós-merge (157, com `${GOEXE_EXT:-}`). Os dois ficam dentro de
`if [ "$SKIP_VERIFY" = "0" ]`, então a variável sempre existe quando o pós-merge roda. `GOEXE` medido:
`.exe` aqui, vazio em `GOOS=linux` e `GOOS=darwin` — onde nada muda.

**AC4.** `check-upstream-sync-falsify.sh`: `OK`. Limite mantido e declarado: ele usa `--skip-verify`
e não exercita os builds; a prova do AC1 é a tabela acima, não este gate.

**AC5.** `trackfw validate`: 0 violações (5 warnings pré-existentes, em REQs de 2026-08-29 que esta
branch não toca). `scripts/run-local-gates.sh`: 10 executados · 0 falhas.

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
Roadmap: docs/roadmaps/claude/done/ROADMAP-2026-09-13-upstream-sync-recompila-tambem-o-bin-trackfw-exe-no-windows.md
