---
status: Open
date: 2026-10-31
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md"
---

# REQ: barrier ml_scope da wave N reprova arquivos declarados em waves posteriores ja iniciadas

> Date: 2026-10-31 | Status: Open
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

Decisão (`docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md`): o alvo contribui com as waves 0..max(N, K).

**Quem afirma o contrato antigo** (precisa mudar junto):
- `TestMLScope_TargetRoadmapExcludedFromMemberLoop` (`internal/commands/barrier_ml_scope_test.go`)
  exige que o arquivo da Wave 2 🔄 reprove no `--wave 1`.
- `docs/cli-parity.md` (linhas ~3150 e ~3199) e os comentários de `barrier.go` (~606, ~1073, ~1173,
  ~1702) dizem "waves 0..N".

## Acceptance Criteria
- [ ] **AC1** — roadmap de 2 waves, ambas iniciadas, com os arquivos das duas commitados: `--wave 1`
      passa no `ml_scope`. (Critério do issue.)
- [ ] **AC2** — a mesma branch com mais um arquivo não declarado em wave nenhuma: `--wave 1` reprova
      e nomeia esse arquivo, e só ele. (Critério do issue.)
- [ ] **AC3** — arquivo declarado só numa wave posterior cujos MLs estão todos ⬜: `--wave 1` reprova
      e nomeia esse arquivo (trabalho adiantado continua detectado).
- [ ] **AC4** — ML de wave > N sem `**Files affected:**` não reprova o barrier da wave N; ML sem
      declaração nas waves 0..N continua reprovando.
- [ ] **AC5** — `TestMLScope_TargetRoadmapExcludedFromMemberLoop` reescrito para o contrato novo,
      mantendo o que ele protege (o alvo não entra pelo laço de membros).
- [ ] **AC6** — `docs/cli-parity.md` e os comentários de `barrier.go` descrevem 0..max(N, K).
- [ ] **AC7** — pacote `internal/commands` verde; `make quality` verde.

## Negative Scope

- Não muda o diff (`merge-base..HEAD` + árvore de trabalho), a leitura do roadmap em `HEAD`, o
  conjunto de orquestração, a rejeição de padrão sem segmento literal, o corte de data nem as regras
  dos membros da união.
- Não restringe o diff por commit ou por wave (opção 2 do issue).
- Não muda os outros checks do barrier.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
