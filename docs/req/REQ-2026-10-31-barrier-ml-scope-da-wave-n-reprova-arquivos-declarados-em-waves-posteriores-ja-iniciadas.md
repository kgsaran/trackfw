---
status: Done
date: 2026-10-31
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md"
---

# REQ: barrier ml_scope da wave N reprova arquivos declarados em waves posteriores ja iniciadas

> Date: 2026-10-31 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Origem

Issue #641: depois que a wave seguinte é commitada, `trackfw barrier <roadmap> --wave N` volta a
reprovar em `ml_scope` uma wave que já tinha passado. Os arquivos acusados estão declarados em
`**Files affected:**` de MLs de waves posteriores. Medido no consumidor CMDB com 11.2.1, num roadmap
de 4 waves todas concluídas: só a última passa.

## Motivation

`checkMLScope` (`internal/commands/barrier.go`) compara o diff da branch inteira com os `Files
affected` das waves 0..N do alvo (passo 9, `if waveInt > currentWaveInt { continue }`). Os membros
da união, na mesma função, já usam as waves 0..K (`mlScopeComputeK`). O veredito de uma wave auditada
muda sem mudança nela, e quem orquestra aprende a ignorar o `ml_scope` das waves antigas.

Decisão (`docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md`, opção B após a Wave 0): o alvo contribui
com as waves 0..N e com cada wave posterior iniciada; os membros da união passam a contribuir só com
as waves iniciadas (a faixa contínua 0..K deles tinha o mesmo furo).

**Quem afirma o contrato antigo** (precisa mudar junto):
- `TestMLScope_TargetRoadmapExcludedFromMemberLoop` (`internal/commands/barrier_ml_scope_test.go`)
  exige que o arquivo da Wave 2 🔄 reprove no `--wave 1`.
- `docs/cli-parity.md` (linhas ~3150 e ~3199) e os comentários de `barrier.go` (~606, ~1073, ~1173,
  ~1702) dizem "waves 0..N".

## Acceptance Criteria
- [x] **AC1** — roadmap de 2 waves, ambas iniciadas, com os arquivos das duas commitados: `--wave 1`
      passa no `ml_scope`. (Critério do issue.)
- [x] **AC2** — a mesma branch com mais um arquivo não declarado em wave nenhuma: `--wave 1` reprova
      e nomeia esse arquivo, e só ele. (Critério do issue.)
- [x] **AC3** — arquivo declarado só numa wave posterior cujos MLs estão todos ⬜: `--wave 1` reprova
      e nomeia esse arquivo, inclusive quando há wave iniciada acima dela (waves 0,1 ✅, 2 ⬜ com
      `x.go`, 3 🔄 com `y.go`: reprova nomeando só `x.go`).
- [x] **AC3b** — o mesmo vetor de wave ⬜ entre waves iniciadas num **membro** da união: o arquivo da
      wave ⬜ do membro não entra no escopo.
- [x] **AC4** — ML de wave > N sem `**Files affected:**` não reprova o barrier da wave N; ML sem
      declaração nas waves 0..N continua reprovando.
- [x] **AC5** — `TestMLScope_TargetRoadmapExcludedFromMemberLoop` reescrito para o contrato novo,
      mantendo o que ele protege (o alvo não entra pelo laço de membros).
- [x] **AC6** — `docs/cli-parity.md` e os comentários de `barrier.go` descrevem a regra nova (0..N +
      waves posteriores iniciadas; membros: waves iniciadas).
- [x] **AC7** — pacote `internal/commands` verde; `make quality` verde.

Acrescentados pelo red team (`docs/seguranca/2026-10-31-redteam-ml-scope-waves-iniciadas.md`);
defeito conhecido se corrige neste PR:
- [x] **AC8** (A-1) — wave de rótulo fracionário é ordenada por `roadmapdoc.CompareWaveLabels`, não pela
      parte inteira: com `--wave 1`, a wave `1b` ⬜ não entra no escopo e um ML dela sem declaração
      não reprova; com `1b` iniciada, entra. `--wave 1b` trata a wave `1` como ≤ N.
- [x] **AC9** (A-2) — `**Status:**` dentro de comentário HTML (`<!-- -->`) não é status do ML, em
      `MLStatusMarker` (vale para `mls_complete`, `ml_scope`, board e `roadmap show`); e critério de
      aceite dentro de comentário HTML não conta em `AcceptanceEvaluateFull` (residual T4, mesma causa).
- [x] **AC11** (mesma causa do A-2, enumerada no ML-3B) — `**Files affected:**` e o bloco
      `**Gates da wave:**` dentro de comentário HTML não contam: não ampliam o escopo do `ml_scope` e
      não viram gate executado.
- [x] **AC10** (A-3) — vetores ponta a ponta para wave > N iniciada só por `❌ Bloqueado` e só por
      `❌ Cancelado`/`🚫 Abandonado`.

## Negative Scope

- Não muda o diff (`merge-base..HEAD` + árvore de trabalho), a leitura do roadmap em `HEAD`, o
  conjunto de orquestração, a rejeição de padrão sem segmento literal, o corte de data nem os filtros
  de estado dos membros da união (só o recorte de waves deles muda).
- Não restringe o diff por commit ou por wave (opção 2 do issue).
- Não muda os outros checks do barrier, exceto o efeito do AC9 sobre `mls_complete` e
  `acceptance_evidence` (status e critério dentro de comentário HTML deixam de contar) e do AC11
  sobre `gates` (gate dentro de comentário HTML deixa de rodar).

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
