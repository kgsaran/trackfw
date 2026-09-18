---
status: Done
date: 2026-09-16
author: "claude"
adr: "docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md"
roadmap: "docs/roadmaps/claude/done/ROADMAP-2026-09-16-gates-so-nossos-depois-da-v8-tirar-o-node-e-o-python-que-o-upstream-removeu.md"
---

# REQ: gates só nossos depois da v8: tirar o Node e o Python que o upstream removeu

> Date: 2026-09-16 | Status: Done

## Motivation

O upstream mesclou a v8.0.0 ([#365](https://github.com/kgsaran/trackfw/pull/365)): uma implementação
em Go entregue por três canais, com `npm/src`, `npm/tests`, `pypi/trackfw` e `pypi/tests` removidos.
O sync (`f979c01`) trouxe 491 arquivos de produto e reteve 6 de `docs/`; `go build` e `validate`
passam. **Os gates só nossos não passam.** Medido na árvore do merge, gate a gate:

| gate | resultado | causa |
|---|---|---|
| `run-local-gates.sh` | **rc=1** antes de rodar gate nenhum | pré-condição exige `node npm/bin/trackfw` e `python3 -m trackfw` |
| `check-slug-inventory.sh` | **rc=1** | `grep: npm/src/generators: No such file or directory` |
| `check-subcommand-parity.sh` | **rc=1**, sem saída | invoca o CLI Node com stderr descartado |
| `check-os-predicate-classification.sh` | rc=0 com **7 declarações obsoletas** | baseline cita 7 arquivos de `npm/src` e `pypi/trackfw` |
| `measure-os-predicate-sites.sh` | rc=0, "a superfície MUDOU" | baseline com sítios de runtime removido |
| os outros 5 | rc=0 | não dependem dos runtimes |

É uma causa só — a remoção dos dois runtimes — e por isso uma REQ só, pela Regra Dura de Causa Raiz.
O mesmo vale para o `local-gates.yml`, que instala Node e Python só para esses gates, e para as seções
do `CLAUDE.md` que afirmam fatos sobre os runtimes removidos.

Já estava previsto: o risco da Wave 3 na pré-condição do agregador foi anotado no sync de 13/09.

## Acceptance Criteria

- [x] **AC1** — `scripts/run-local-gates.sh` com **0 falhas** na árvore da v8, sem exigir Node nem
      Python; a pré-condição continua exigindo `bin/trackfw` e continua reprovando **nomeada** sem ele
      (falsificado).
- [x] **AC2** — `check-subcommand-parity.sh` **retirado**, com o motivo escrito: a propriedade que ele
      media (divergência de subcomando entre implementações) deixou de ser definível com uma
      implementação só — o mesmo discriminante que o mantenedor escreveu ao fechar a
      [#298](https://github.com/kgsaran/trackfw/issues/298). Não é detector perdido com defeito vivo.
- [x] **AC3** — `check-slug-inventory.sh` reduzido ao Go e **ainda reprovando** implementação nova ou
      sumida (falsificado nos dois sentidos).
- [x] **AC4** — os dois instrumentos de predicado de SO com escopo `internal cmd` (restrito a `.go` em
      2026-09-18, ML-1D, depois que o #395 pôs testdata `.md` em `internal/`), **zero** declaração
      obsoleta no lint, baseline do `measure` regravado **só depois** de conferir que todo nome que sai
      é de arquivo removido pela v8; as guardas de vacuidade continuam de pé.
- [x] **AC5** — `local-gates.yml` sem `setup-node`, `setup-python`, `npm ci` e `pip install`, e o job
      verde no CI.
- [x] **AC6** — `CLAUDE.md`: toda afirmação sobre os runtimes removidos que ficou falsa é corrigida ou
      marcada como caducada **com o fato que mudou**; nenhuma é apagada em silêncio.
- [x] **AC7** — `trackfw validate` com 0 violações; CI comparado por nome contra o run do upstream em
      `66b7ad8`.
- [x] **AC8** — `docs/cli-parity.md` trazido do upstream e **mantido** pelo `upstream-sync.sh` (decisão
      do usuário em 2026-09-16), com a retenção do resto de `docs/` ainda provada por efeito, o arquivo
      provado igual ao do REF, e a falsificação reprovando um sync sem a exceção.

      Descoberto no CI desta PR, e é a mesma causa: a v8 apagou os gates que o nosso `cli-parity.md`
      retido citava. O `check-parity-contract-coverage.sh` reprovou o `parity-other-gates` em
      `Makefile:35` — antes do barrier, onde ele parava —, e o item 4 do `windows-defect-reproduction`,
      que roda o mesmo gate, travou no `run.ps1:148` até o timeout.

## Evidência — 2026-09-16

Detalhe por microlote no roadmap. Aqui, o sítio de cada AC:

| AC | onde se comprova |
|---|---|
| AC1 | `scripts/run-local-gates.sh`: `9 executado(s) · 0 falha(s)`; worktree sem `bin/` → rc=1 nomeando o binário (ML-1A) |
| AC2 | `scripts/check-subcommand-parity.sh` removido em `08e1db8`, motivo no comentário do agregador e no `CLAUDE.md` |
| AC3 | `scripts/check-slug-inventory.sh`: 1 implementação; slug novo e slug sumido → rc=1 (ML-1A) |
| AC4 | lint `204 · 57 D1 · 4 D2 em 2 arquivos`, 0 obsoleta; baseline do `measure` reconciliado por `arquivo:predicado` e regravado (ML-1B) |
| AC5 | `Gates locais do fork` **success** em `push` e `pull_request` no head `c07582c` da PR #135, sem os passos de Node e Python |
| AC6 | tabela de veredito por seção no ML-2A |
| AC7 | `validate` 0 violações; Quality `35142703865` contra o upstream `35135752812`: **18 jobs, nomes idênticos nos dois sentidos**; não-verdes só `parity` e `parity-other-gates`, este parando em `Makefile:63` — `check-roadmap-barrier-contract: um ou mais cenários FALHARAM (49 executados)`, o snapshot congelado —; `windows-defect-reproduction` **success** |
| AC8 | `scripts/upstream-sync.sh` com `PRODUTO_EM_DOCS`; falsificação OK e controle sem a exceção rc=1 (ML-3A); primeiro sync real com a exceção: `93f046e` (#371) |

**O CI precisou de três rodadas, e cada uma ensinou algo.** A primeira reprovou o passo de divergência
porque o upstream mesclou o #370 durante o run (`release.yml` diferia da `upstream/main` nova, não por
mudança nossa) — trazido na mesma PR. A segunda expôs o `docs/cli-parity.md` retido e defasado
(AC8). A terceira fechou com o conjunto conhecido.

## Escopo negativo

- **Não** mexe em arquivo compartilhado com o upstream (`Makefile`, `quality.yml`, gates dele): a
  divergência de produto continua zero.
- **Não** corrige a [#366](https://github.com/kgsaran/trackfw/issues/366) (`check-tty-detection.sh`
  roda `init` no cwd e reescreve o `trackfw.yaml` real). É do upstream; aqui fica só o aviso de não
  rodar `parity-rest` na raiz do fork.
- **Não** trata o sítio restante da [#363](https://github.com/kgsaran/trackfw/issues/363) nem a
  metade que resta da [#268](https://github.com/kgsaran/trackfw/issues/268): são do upstream.

## Linked ADR
ADR: docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/claude/done/ROADMAP-2026-09-16-gates-so-nossos-depois-da-v8-tirar-o-node-e-o-python-que-o-upstream-removeu.md
