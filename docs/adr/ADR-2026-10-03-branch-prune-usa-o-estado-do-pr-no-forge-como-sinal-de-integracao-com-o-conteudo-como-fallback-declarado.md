---
status: Accepted
date: 2026-10-03
author: "trackfw_architect"
---

# ADR: `branch prune` usa o estado do PR no forge como sinal de integração, com o conteúdo como fallback declarado

> Date: 2026-10-03 | Status: Accepted

REQ: `docs/req/REQ-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md`
Origem: **#481** (com medição de um segundo corpus no comentário de consumidor externo)
Revoga em parte: a **decisão 2** da `REQ-2026-08-18` ("fonte de verdade é só o git; sem consulta a forge").
Mantém dela: o comando funciona offline, `--dry-run` é o padrão, `review` nunca é apagado sozinho.

## Context

O `trackfw branch prune` decide integração só por conteúdo: arquivos tocados desde o `merge-base`
contra o tip de `origin/main`. Esse teste **não distingue** "a branch entrou e a `main` seguiu mexendo
nos mesmos arquivos" de "a branch nunca entrou".

**Medido neste repositório (2026-10-03), 52 branches locais:**

| sinal | branches |
|---|---|
| upstream `[gone]` + PR `MERGED` + tip local == head do PR mergeado | **48** |
| PR `MERGED`, tip ≠ head | 3 |
| PR `MERGED`, upstream vivo, tip == head | 1 |
| veredito atual do `prune` | 1 delete · 25 review · resto keep |

No comentário da #481, o consumidor externo mediu 120 branches deste repositório: **nenhuma** tem um arquivo divergente
que a evolução ou o apagamento da `main` não expliquem, e **0 de 120** tips são ancestrais da `main`
(squash-merge). **Por conteúdo, nada aqui se distingue; pelo estado do PR, quase tudo.**

## Decision

### D1 — Ordem dos sinais, do mais forte ao mais fraco

Para cada branch local (fora da corrente, da default e das presas em worktree, como hoje):

0. **Algum PR `OPEN` com esse head (não fork)** → `keep`, motivo "open PR #N". Isso prevalece sobre
   qualquer PR `MERGED` antigo com o mesmo nome, e é a defesa contra nome de branch reaproveitado.
1. **PR mergeado que contém a branch inteira** → `delete`. Se houver vários PRs `MERGED` com o mesmo
   head, basta que **um** contenha o tip. Isso vale quando o forge informa um PR
   `MERGED` cuja **head branch** tem o nome da branch local, cujo **repositório de origem é o próprio
   `origin`** (não um fork com o mesmo nome de branch), e cujo **commit de head contém o tip local**
   (`git merge-base --is-ancestor <tip> <head do PR>`; tip igual ao head é o caso comum). Se o commit
   de head não estiver disponível localmente, o veredito é `review`, e não `delete`.
2. **PR mergeado, mas o tip local tem commits além do head do PR** → `keep`, motivo "commits after the
   merged PR". É trabalho feito depois do merge.
2b. **PR mergeado, mas o tip divergiu** de todos os heads (nem ancestral nem descendente, por exemplo
   depois de rebase ou amend) → `keep`, motivo "diverged from the merged PR".
3. **PR fechado sem merge** → `review`, motivo "PR closed without merge". Nunca é apagado sozinho.
4. **Sem PR e sem upstream** (nunca empurrada) → `keep`, motivo "never pushed, no PR". 🔴 **Nunca é
   apagada, mesmo com conteúdo idêntico.** É a única classe em que apagar perde trabalho que não existe
   em nenhum outro lugar.
5. **Sem PR, com upstream** → a heurística de conteúdo de hoje decide, sem mudança.

### D2 — Sem forge, o comando degrada declarando

Quando o forge não é GitHub, ou o `gh` não está disponível ou autenticado, ou a consulta falha, o
`prune` usa **só** a heurística de conteúdo de hoje e imprime **uma** linha dizendo que o estado dos
PRs não foi consultado e por quê. Ele nunca decide `delete` com um sinal mais fraco do que o de hoje.
O comportamento offline da REQ-2026-08-18 continua valendo.

### D3 — Uma consulta, não uma por branch

O estado dos PRs vem de **uma** chamada (`gh pr list --state all` com os campos de head, estado,
número e repositório de origem). Ela é cruzada localmente com as branches. Isso evita rate limit e
mantém o comando rápido. 🔴 **A janela da consulta é parte do contrato**: a primeira medição do
consumidor na #481 deu 0 falsos positivos porque pediu só os últimos 100 PRs, e as branches estavam
fora da janela. Uma resposta que pode estar truncada (número de itens igual ao limite pedido) é tratada
como **consulta incompleta**. Branch sem PR nessa resposta não é "sem PR": fica no veredito de hoje, e o
relatório diz isso.

### D4 — O aviso do `push`/`ship` usa a mesma avaliação

O aviso `appears to have unmerged changes` (`detectPendingSquashMerges`, `internal/commands/ship.go`)
usa a mesma `evaluateBranchIntegration`. A causa é a mesma, então ele passa a consultar o estado do PR
pela mesma função, sem segunda implementação. Ele também degrada quando não há forge.

### D5 — Ajustes do threat model (Wave 0, `docs/seguranca/2026-10-03-wave0-prune-estado-do-pr.md`)

- **A1 — Repositório explícito.** A consulta passa `--repo HOST/OWNER/REPO`, derivado de
  `git remote get-url origin`. Se o host não for `github.com`, o comando degrada (D2). Medido: com
  `GH_REPO=cli/cli`, o `gh` devolve PRs de outro repositório com exit 0, e o `--repo` prevalece sobre
  `GH_REPO`.
- **A2 — Base do PR.** O caso 1 exige `baseRefName` igual à branch default do `origin` (a mesma que
  o `prune` já usa como referência). PR mergeado em outra base, como num PR empilhado, não apaga.
- **A3 — Head ausente.** `git cat-file -e <headRefOid>^{commit}` antes do `is-ancestor`. Se o objeto
  faltar, o veredito é `review`. O exit 1 do `is-ancestor` não pode ser confundido com o exit 128.
- **A4 — Upstream.** "Sem upstream" se lê por `git for-each-ref --format=%(upstream:short)` (campo
  vazio). `@{u}` falha com exit 128 em branch `[gone]`; medido.
- **A5 — Erro.** Exit ≠ 0 ou JSON inválido → D2. `[]` com exit 0 é uma resposta legítima.
- **A6 — Aviso do push/ship.** O casamento usa o nome curto (sem `origin/`). Avisam: os casos 2 e 2b e
  o `pending_work` de hoje. Não avisam: o caso 0 (PR aberto) e o caso 1.
- **A7 + A8 — Truncamento (decisão do arquiteto).** `--limit 3000`. Se a resposta trouxer itens ==
  limite, a consulta inteira é tratada como **incompleta**: nenhuma branch recebe `delete` pelo sinal de
  PR, tudo fica no veredito de hoje, e o relatório diz isso. É mais estrito que "só a branch ausente fica
  no veredito de hoje", porque uma branch presente na janela pode ter outro PR do mesmo head fora dela.
  Medido: 428 PRs em 2,3 s.

## Consequences

- No acervo medido (2026-10-03, `merge-base --is-ancestor` nas duas direções), o `prune` passa a
  apagar **49** das 52 branches. Todas as 49 têm um PR `MERGED` com head == tip. Uma delas tem dois PRs
  mergeados, um com head == tip e outro divergente, e o caso 1 a cobre.
  As **3** restantes ficam em `keep` pelo caso 2, porque o tip está **adiante** do head mergeado
  (2, 2 e 9 commits): `docs/bash-consome-stdout-de-python3-sem-normalizar-crlf`,
  `docs/fechamento-req-2026-09-28-stale-roadmap-para-req` e `fix/criterio-de-adr-por-prefixo`. Isso está
  correto. A última guarda o commit do AC10 da #503, empurrado depois do merge e integrado depois por
  cherry-pick. O PR não sabe disso, e o `keep` não apaga nada.
- **Custo:** uma chamada de rede quando há `gh`. Sem ela, o comportamento é exatamente o de hoje.
- **Resíduo declarado:** só o GitHub é consultado. GitLab, Azure e Bitbucket degradam (D2). Os
  adaptadores existem em `internal/forge/`, mas o comando de listagem de cada um é trabalho futuro.
- **Resíduo declarado:** PR mergeado por rebase com reescrita de commits (o head deixa de existir como
  objeto) cai em `review`, nunca em `delete`.

## Alternatives Considered

- **Balde `obsolete` (arquivos tocados não existem mais na `main`).** Rejeitada pela própria medição
  do comentário da #481: pega 1 de 120 branches neste repositório.
- **Só o sinal `[gone]`.** É forte e local, mas um remoto pode apagar uma branch não mergeada. Fica como
  evidência no relatório, mas não decide sozinho.
- **Manter "sem forge".** Mantém 48 de 52 branches mergeadas como `keep`/`review`, que é o defeito.

## Linked REQ

`docs/req/REQ-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md`
