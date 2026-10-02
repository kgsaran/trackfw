---
status: Done
date: 2026-10-02
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md"
---

# REQ: o driver de falsificação paralelo espera para sempre por chunk que não termina — limite de tempo por chunk com FAIL nomeado

> Date: 2026-10-02 | Status: Done
| GitHub Issue: #504
| Mergeado: PR #506 em `f3d98639` (2026-10-02). #504 fechada pelo merge.

## Motivation

`scripts/run-gates-falsify-parallel.sh` (alvo `parity-falsify` do `make quality` local) dispara 8
chunks e faz `wait` em cada um, **sem limite de tempo**. Três vezes, em 2026-10-01/02, o `make quality`
ficou de 40 a 60 minutos parado ali e foi morto, sem dizer qual chunk travou nem em que comando.

Medido depois (comentário da #504): com a máquina ociosa, 5 de 5 execuções limpas (driver em ~3 min,
`make quality` em 9 min 23 s). A causa dos travamentos **não foi medida**, porque nenhum `ps` foi
capturado. O defeito corrigível é de **observabilidade**: o driver transforma qualquer travamento em
silêncio.

**Decisão (sem ADR: muda só ferramenta de teste local, não o produto):** cada chunk ganha limite de
tempo. Ao estourar, o driver:
1. imprime a árvore de processos do chunk (`pid`, `ppid`, `stat`, `etime`, `%cpu`, `command`);
2. mata a árvore inteira do chunk;
3. emite `FAIL [falsify-driver/chunk-timeout] chunk <N> excedeu <T>s` e sai com rc≠0.

Limite padrão: **1200 s** (o chunk mais lento medido sob o `make quality` levou ~6 min). Pode ser
ajustado por `TRACKFW_FALSIFY_CHUNK_TIMEOUT` (segundos), e o driver denuncia o override no stderr,
como já faz com `TRACKFW_FALSIFY_JOBS`.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** o que o limite pode esconder ou quebrar: chunk lento legítimo virando
  FAIL; processo da árvore que sobrevive ao kill (órfão que contamina a execução seguinte); a guarda
  de conjunto (rótulos ausentes) seguir funcionando. Parecer em `docs/seguranca/`.
      ✅ Evidência: `docs/seguranca/2026-10-02-wave0-limite-por-chunk.md`, APROVA COM AJUSTES (8 ajustes absorvidos)
- [x] **AC2** — Chunk sintético que dorme para sempre, com `TRACKFW_FALSIFY_CHUNK_TIMEOUT=5`: o driver
  sai rc≠0 em até ~T+10 s, com a linha `FAIL [falsify-driver/chunk-timeout] chunk <N>` e a árvore de
  processos impressa. Depois da saída, **nenhum** processo da árvore do chunk segue vivo (conferido
  por `ps`).
      ✅ Evidência: braço 3 de `scripts/check-falsify-chunk-timeout.sh`: ~7 s, FAIL nomeado, árvore impressa, 0 sobreviventes
- [x] **AC3** — Sem override, a suíte real segue verde: 347 OK / 0 FAIL, guarda de conjunto OK.
      ✅ Evidência: `make quality` com a máquina ociosa: `suite completa -- 8 chunks, 347 OK, 0 FAIL`
- [x] **AC4** — O teste do AC2 reprova no driver de `0bf66679`: sem limite, ele não termina, e o
  próprio teste usa um teto externo para não pendurar.
      ✅ Evidência: braço 5: o driver de `0bf66679` continua vivo no teto de 15 s; em clone raso, `SKIP` declarado
- [x] **AC5** — Funciona no `bash` 3.2 do macOS **e** no `bash` 5 do PATH. Se o driver exigir bash 5
  (já usa `mapfile`), o teste roda com o mesmo bash que o `make` usa, e isso fica declarado.
      ✅ Evidência: o driver declara bash 4+ (`mapfile`); driver e autoteste usam o bash do PATH; pulam no MSYS
- [x] **AC6** — `make quality` EXIT=0 e CI verde, inclusive `windows-full-suites`.
      ✅ Evidência: CI do PR #506 em `279babf8`: 20/20 SUCCESS, inclusive `windows-full-suites`; `make quality` local com a máquina ociosa EXIT=0 (347/0)

## Negative scope

- **A causa dos travamentos:** não foi medida e não é alvo. O limite existe para que a próxima
  ocorrência a meça.
- **Shards do CI** (`run-gates-falsify-shard.sh`): já têm o timeout do job do GitHub Actions; não
  mudam.
- **O conteúdo da suíte** (`check-gates-falsify.sh`) e o particionamento (`gen-falsify-chunks.py`).

## Linked ADR
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md
