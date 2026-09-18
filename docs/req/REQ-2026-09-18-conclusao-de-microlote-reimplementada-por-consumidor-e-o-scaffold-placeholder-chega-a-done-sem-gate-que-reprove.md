---
status: Open
date: 2026-09-18
---

# REQ: conclusão de microlote reimplementada por consumidor, e o scaffold placeholder chega a done sem gate que reprove

> Date: 2026-09-18 | Status: Open
| Linear Issue: 
| Jira Issue: 

**Issue:** #392
**ADR:** `docs/adr/ADR-2026-09-18-conclusao-de-microlote-tem-uma-implementacao-unica-num-pacote-folha-e-o-contorno-fecha-na-transicao-e-na-cobertura-do-gate.md`

## Motivation

Um roadmap que diz `done` carregando microlote `⬜ Pendente` afirma duas coisas contraditórias, e a
que engana é a que diz "done". O registro passa a dar a aparência tranquilizadora de correção.

Três ocorrências em 2026-09-17, nenhuma detectada por gate. Medido no corpus em 2026-09-18:
**32 de 192** roadmaps em `done/` têm ML não concluído pelo predicado canônico
(`StatusIsComplete`) — 27 pelo recorte estreito `⬜`/`🔄`, mais 5 com status fora do vocabulário;
**4** têm o gate placeholder `exit 1` intacto.

A causa não é a ausência de um check: é que **"este ML está concluído?" é reimplementada por
consumidor** — `barrier` (correto, por token), `serve` (`Contains("✅")`, sem máscara de cerca), e
`validate` (não responde) — e o `scaffold` ensina um **quarto** valor (`pending`) que o verificador
rejeita. O parser correto está preso em `package commands` e é inalcançável pelo `validator` por
ciclo de import.

## Acceptance Criteria

- [ ] **AC1 — Pacote folha.** Existe `internal/roadmapdoc`, sem importar `internal/commands` nem
      `internal/validator`. `go build ./...` RC=0 e nenhum ciclo de import.
- [ ] **AC2 — 🔴 Identidade do `barrier` (falsificador da extração).** A saída `trackfw barrier
      <roadmap> --wave <n> --json` é **byte-idêntica antes e depois** da extração, para todas as
      waves de todos os roadmaps de `done/` + `wip/`. O baseline é capturado **antes** de qualquer
      edição, num arquivo versionado sob `scripts/` ou no diretório de evidência do ML. Divergência
      em qualquer arquivo reprova o ML.
- [ ] **AC3 — 🔴 Predicado reconciliado contra 32/160, com o predicado canônico.**
      O predicado é `roadmapdoc.StatusIsComplete` — **não** um teste por `⬜`/`🔄`. Aplicado aos 192
      roadmaps de `done/`, retorna "não concluído" para **32** e "concluído" para 160.
      **Correção de uma contradição minha, apontada pela Wave 0:** eu escrevi "exatamente os 27" —
      número que vem de medir `⬜`/`🔄`, um predicado *diferente* do que o próprio AC nomeia. A
      diferença são **5** roadmaps com status fora do vocabulário (`pending`, `PENDENTE`,
      `ABANDONADO`, `❌`, `🚫`). Estreitar o predicado para `⬜`/`🔄` faria o AC passar excluindo
      `pending` em silêncio — que é o valor que o AC5 existe para eliminar.
      Braços nomeados, ambos obrigatórios:
      (a) **positivo** — fixture congelado em `internal/roadmapdoc/testdata/`, cópia byte a byte de
      `docs/roadmaps/done/ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md`
      (2 MLs, ambos `⬜ Pendente`, em `done/`) → **tem** de dar "não concluído";
      (b) **contra-braço** — o roadmap do #387 em `done/` (7 MLs, 7 concluídos) → **tem** de dar
      "concluído".
      🔴 O fixture é **congelado em `testdata/`, não lido do corpus vivo**, porque o ML-4A limpa
      exatamente esse arquivo. O braço que eu havia escrito antes apontava para
      `ROADMAP-2026-09-16-run-capture`, que **eu mesmo já tinha limpado em 2026-09-17** — tem 0 MLs,
      e teria calibrado o predicado para não detectar nada.
- [ ] **AC3-bis — Três categorias de status, decididas.** O vocabulário distingue:
      **concluído** (`✅`, `done`, `Concluído`, `CONCLUIDO`) → libera;
      **encerrado sem conclusão** (`ABANDONADO`, `🚫 Abandonado`, `❌ Cancelado`) → **libera**, com o
      motivo registrado na linha;
      **pendente** (`⬜`, `🔄`, `pending`, `PENDENTE`, `❌ Bloqueado`) → **bloqueia**.
      Justificativa de `❌ Bloqueado` bloquear: um ML bloqueado dentro de um roadmap `done` é
      exatamente a contradição que esta REQ fecha. Justificativa de abandonado liberar: encerramento
      explícito é uma decisão registrada, não um esquecimento — e reprová-lo seria o falso-positivo
      que a `ADR-2026-08-17` nomeia.
- [ ] **AC4 — `serve` converge.** `parseMLProgress` usa `roadmapdoc`. Teste que falsifica o defeito
      atual: um roadmap com `✅` **dentro de cerca de código** e ML realmente pendente não é contado
      como concluído.
- [ ] **AC5 — `scaffold` converge, e antes do gate.** `internal/generators/scaffold.go` deixa de
      ensinar `**Status:** pending` e passa a ensinar o valor que o gerador escreve e o
      `statusVocabulary` aceita. Entregue em wave **anterior** à do AC6.
- [ ] **AC6 — `roadmap move ... done` recusa ML pendente.** A recusa **nomeia cada ML pendente** com
      rótulo e número de linha. Contra-braço obrigatório: mover para `done` um roadmap legitimamente
      concluído **não** pode recusar.
- [ ] **AC7 — 🔴 Gate por perda de cobertura, não por presença de placeholder.** O discriminante é
      *"a wave perdeu o gate que o template lhe deu"*. Falsificação em três braços, todos exigidos:
      (a) `exit 1` intacto → **reprova**; (b) **bloco `**Gates da wave:**` inteiramente apagado** →
      **reprova** (este é o braço que mata o discriminante ingênuo, e é a lição do ML-2B do #387);
      (c) gate substituído por comando real → **não reprova**.
- [ ] **AC7-bis — 🔴 Tier 1b: esconder a wave renomeando o heading.** Achado da Wave 0, não previsto
      por mim. `waveHeadingRe` é `^## Wave (\S+) ` — renomear `## Wave 0 — Threat Model` para
      `## Threat Model` (1 linha, mesmo custo do tier anterior) faz a wave **desaparecer do parser**,
      e o discriminante de cobertura falha **vacuosamente**: não há wave para verificar.
      🔴 **Decidido com a medição, e a medição proíbe a solução óbvia:** exigir Wave 0 em todo roadmap
      acenderia **154 dos 192** de `done/` — apenas 38 têm `## Wave 0`, porque a convenção só existe
      desde agosto. Seria pior que os 27 que a decisão 7 da ADR rejeita.
      Regra adotada: *roadmap em `wip`/`blocked`, e todo roadmap na transição para `done`, deve ter
      `## Wave 0`*. **`done/` não é reavaliado.** Custo retroativo medido: `wip` 1/1 ✅, `blocked` 2/3,
      `analyzing` 1/1 ✅ — **1 arquivo**. A assimetria é deliberada e fica escrita.
- [ ] **AC8 — Rótulo duplicado falha fechada.** Duas `## Wave 0` ou dois `### ML-1A` no mesmo roadmap
      são violação nomeada. Falsificação: hoje `barrier.go:877-882` faz `break` no primeiro rótulo
      que casa, e a segunda cópia é **invisível** — o teste tem de demonstrar que passava antes e
      reprova depois.
- [ ] **AC8-bis — O gate não cobra em `backlog`/`analyzing`.** Medido: 6 roadmaps de `backlog/`
      carregam o placeholder legitimamente — ninguém os começou. Cobrar ali é ruído sobre artefato
      sadio. Falsificação: roadmap em `backlog` com `exit 1` **não** reprova; o mesmo arquivo movido
      para `wip` **reprova**.
- [ ] **AC8-ter — Os 6 sítios medidos em `done/` são corrigidos nesta REQ.** 4 com placeholder + 4
      com rótulo duplicado, 2 em comum. Pela Regra Dura de Causa Raiz, defeito medido e localizado se
      corrige, não se registra. Após a correção, o predicado de AC7 e AC8 retorna **zero** em `done/`.
- [ ] **AC9 — Falha de sincronização deixa de ser engolida.** `_ = os.WriteFile` em
      `internal/generators/roadmap.go:581-585` passa a propagar o erro.
- [ ] **AC10 — Gates verdes.** `go build ./...`, `make test`, `make quality` e `trackfw doctor` em
      RC=0 / sem mismatch, e `trackfw validate` sem violação nova.
- [ ] **AC11 — Reconciliação.** Cada teste novo vem acompanhado, no relatório do ML, de **uma frase**
      declarando qual conclusão do próprio ML ele afirma (Regra Dura de Reconciliação).

## Negative scope — o que esta REQ NÃO faz

- ❌ **Regra do `validate` sobre roadmap em `done/` com ML pendente.** Rejeitada pela decisão 7 da
  ADR, com o número que a rejeita: **32 dos 192** acenderiam de imediato, e fechar isso exigiria
  baseline ou carve-out — o mecanismo que a campanha do #387 acabou de provar ser a superfície de
  ataque.
- ❌ **Sanar retroativamente os 32 roadmaps de `done/`.** Risco desproporcional; nomeado e aceito.
- ❌ **Exigir `## Wave 0` retroativamente em `done/`.** **154 dos 192** acenderiam; a convenção só
  existe desde agosto. Ver AC7-bis para a forma sensível ao estado que foi adotada no lugar.
- ❌ **Remover o placeholder do `roadmap new`** (direção 3 do issue). Barrada pela `ADR-2026-07-31`,
  que decidiu explicitamente que a seção consolidada é placeholder a preencher.
- ❌ **A seção `## Linked Roadmap` perdida na reescrita manual da REQ do #387.** Causa **diferente**,
  demonstrada: `req_roadmap_sync` **já detecta** esse caso (foi assim que apareceu); o default dela é
  `warning`. É lacuna de severidade, não de detecção. O #392 é não-detectado por regra alguma.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: `docs/adr/ADR-2026-09-18-conclusao-de-microlote-tem-uma-implementacao-unica-num-pacote-folha-e-o-contorno-fecha-na-transicao-e-na-cobertura-do-gate.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-18-conclusao-de-microlote-reimplementada-por-consumidor-e-o-scaffold-placeholder-chega-a-done-sem-gate-que-reprove.md`
