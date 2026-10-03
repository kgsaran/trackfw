---
status: wip
date: 2026-10-03
req: "docs/req/REQ-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: branch prune classifica branches ja mergeadas como pending work porque nunca consulta o estado do PR

> Created: 2026-10-03 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md
ADR: docs/adr/ADR-2026-10-03-branch-prune-usa-o-estado-do-pr-no-forge-como-sinal-de-integracao-com-o-conteudo-como-fallback-declarado.md
Issue: #481 (label `req-aberta`). Fecha #481.

O `branch prune` decide só por conteúdo (`internal/commands/branch_prune.go`, `evaluateBranchIntegration`
:93). O aviso do `push`/`ship` (`detectPendingSquashMerges`, `ship.go:786`) usa a mesma função e é chamado
em `push.go:235` e `ship.go:389`, então tem o mesmo defeito. Mesma causa, mesma REQ.

Medido aqui em 2026-10-03, em 52 branches locais:
- 49 têm PR `MERGED` cujo head == tip local (48 delas com upstream `[gone]`);
- 3 têm PR `MERGED` com tip ≠ head.

Veredito atual: 1 delete, 25 review, o resto keep.

## Acceptance Criteria
- [ ] AC1–AC10 da REQ, cada um com evidência apontável

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model (apagar trabalho não integrado)
> Dependências: nenhuma. Bloqueia toda implementação.

### ML-0A — Threat model do sinal de PR no `prune`
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-03-wave0-prune-estado-do-pr.md` (único arquivo escrito)
**Actions:**
1. **Completude da enumeração:** listar todos os pontos que decidem "integrada": `branch_prune.go`,
   `detectPendingSquashMerges` em `ship.go`, e os chamadores em `push.go`/`ship.go`. Procurar por grep
   outros consumidores de `evaluateBranchIntegration`/`branchPruneDecision`.
2. **Threat model**, com o cenário concreto de cada caso:
   - PR de fork com o mesmo nome de head;
   - **nome de branch reaproveitado** depois de um PR antigo mergeado (o tip novo é ancestral do head
     antigo? quando é, o que se perde?);
   - vários PRs com o mesmo head (um MERGED, outro OPEN);
   - **resposta truncada** por limite ou paginação (o erro que o consumidor já mediu na #481);
   - saída do `gh` parcial, ou com erro e exit 0;
   - head do PR ausente localmente;
   - branch sem upstream;
   - `gh` autenticado em outro host ou repositório (a partir de qual remoto o `gh` resolve o
     repositório?).
3. **Alvos de falsificação nas duas direções** para cada caso: o que apaga indevidamente, e o que volta a
   reter branch mergeada.
4. **Resíduo declarado.**
**Acceptance criteria:**
- [x] As quatro seções respondidas com evidência (comando e saída), não com afirmação de uma linha
- [x] Cada cenário com veredito: coberto pelo ADR, coberto com ajuste (qual), ou resíduo
- [x] Nenhuma linha de implementação

      ✅ Parecer: 11 cenários, 8 ajustes (A1–A8) incorporados ao ADR como D5. O arquiteto conferiu por conta própria a medição de `GH_REPO`/`--repo`. Decisão A7: truncamento bloqueia todo `delete` pelo sinal de PR.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-03-wave0-prune-estado-do-pr.md
```

## Wave 1 — Implementação
> Dependências: Wave 0 auditada.
>
> Um ML só. `branch_prune.go`, `ship.go` e `push.go` compartilham a avaliação, e os testes vivem no mesmo
> pacote. Dividir colocaria dois agentes no mesmo arquivo.

### ML-1A — Sinal de PR na avaliação compartilhada
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/commands/branch_prune.go`, `internal/commands/branch_prune_test.go`
- `internal/commands/ship.go`
- `internal/commands/push.go` (só a chamada)
- os testes de push/ship que cobrem o aviso
- `docs/cli-parity.md`
- arquivo novo permitido: `internal/commands/branch_prune_forge.go`, para a consulta, se isso deixar
  `branch_prune.go` mais legível
**Actions:**
1. Uma consulta por execução (ADR D3):
   `gh pr list --state all --limit <N> --json number,state,headRefName,headRefOid,headRepositoryOwner,isCrossRepository`.
   A consulta é injetada por dependência (como o `gitExec`), para que os testes não toquem a rede.
2. Seguir exatamente a ordem de sinais da D1:
   - filtrar forks por `isCrossRepository` e pelo dono;
   - comparar com `git merge-base --is-ancestor <tip> <headRefOid>`;
   - head ausente localmente → `review`.
3. Truncamento (D3): número de itens == limite → consulta incompleta. A branch ausente da resposta fica no
   veredito de hoje, com uma linha no relatório.
4. Degradação (D2): sem `gh`, não autenticado, erro, ou forge que não é GitHub → veredito de hoje mais
   **uma** linha nomeando a causa. Nunca `delete` com sinal mais fraco que o de hoje.
5. D4: `detectPendingSquashMerges` usa a mesma avaliação e não avisa sobre branch cujo PR mergeado contém
   o tip.
6. Remover o comentário "Per REQ-2026-08-18 decision 2, there is no forge lookup" e citar o ADR novo.
8. **Aplicar a D5 do ADR (A1–A8) integralmente**; cada ajuste tem teste próprio.
7. `docs/cli-parity.md`: documentar o contrato do `prune` e do aviso, com `trackfw-contract`.
**Acceptance criteria:**
- [x] AC2: um teste por caso da D1, nomeado, que reprova sem a correção
- [x] AC3: PR MERGED de fork com o mesmo head **não** gera `delete`
- [x] AC4: sem `gh` ou com erro → veredito idêntico ao de hoje, com a linha de causa
- [x] AC5: o contador de chamadas da dependência == 1 com N branches; resposta truncada não vira "sem PR"
- [x] AC6: o aviso do push/ship silencia para a branch mergeada e continua para a pendente
- [x] AC9: no relatório, uma frase por teste novo dizendo o que ele afirma, mais a prova de mordida
  (sabotar só o sítio de chamada, nunca a declaração)
- [x] `go build ./...` e `go test ./internal/commands/` verdes (**não** rodar `make quality`)

      ✅ Auditoria do arquiteto: 22 testes conferidos por `go test -list`; medição real com o binário da branch = 49 delete + 3 keep ("commits after the merged PR"), igual à tabela do ADR; sem `gh` no PATH → linha `Note:` com a causa e veredito de hoje; `GH_REPO=cli/cli` não desvia (52 sinais de PR). Base `main` vem de `branchPruneDefaultLocalName`, a mesma constante que o prune já usava.

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
go test ./internal/commands/ -run 'Prune|PendingSquash' -count=1
```

## Wave 2 — Barreira
> Dependências: Wave 1 auditada. O Hades lê; o Hefesto roda `make quality` com a máquina ociosa.

### ML-2A — Revisão de segurança contra o threat model
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-03-wave2-revisao-prune-estado-do-pr.md`
**Actions:** confrontar cada cenário da Wave 0 com o código e os testes entregues. Tentar produzir um
`delete` indevido com fixture própria.
**Acceptance criteria:**
- [x] Cada cenário da Wave 0 com veredito, e um veredito final explícito
      ✅ APROVA COM AJUSTES. AJ1 (caso 4 ignorava branch nunca empurrada com PR MERGED em base não-main → podia virar delete) e lacunas L1/L3/L4 corrigidas no **ML-1B corretivo** (apolo-tf, `branch_prune_forge.go` e `_test.go`), com mordida provada. Medição real após o ML-1B: 49 delete + 3 keep, inalterada.

### ML-2B — Qualidade e gate completo
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-03-revisao-prune-estado-do-pr.md`
**Actions:** revisão de manutenibilidade. Rodar `make quality` com a máquina ociosa, registrando o EXIT e o
número de linhas do log.
**Acceptance criteria:**
- [ ] `make quality` EXIT=0, com o log conferido (não só o resumo)

### ML-2C — Medição de volta (AC7)
**Status:** ⬜ Pendente
**Squad:** zeus-tf. Só leitura: `trackfw branch prune` em dry-run, com o binário da branch.
**Acceptance criteria:**
- [ ] A classificação bate com a tabela do ADR; nenhuma branch com trabalho não integrado em `delete`

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-03-wave2-revisao-prune-estado-do-pr.md
test -s docs/qualidade/2026-10-03-revisao-prune-estado-do-pr.md
```
