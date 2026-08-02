---
status: Done
date: 2026-08-01
author: "Zeus"
adr: "docs/adr/ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito-e-regra-de-aceite-exigido-por-req-concluida.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida.md"
---

# REQ: Detectar ADR nao aceito referenciado por REQ concluida

> Date: 2026-08-01 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O `ADR-2026-07-26-principios-de-design-de-gates-verificaveis` permaneceu `Proposed` enquanto
**7 REQs que ele governa** foram todas concluídas. Nenhum gate detectou. Descoberto por auditoria
manual de backlog em 2026-08-01.

Investigando a causa, encontrei uma lacuna maior: **o vocabulário de "ADR não aceito" está
fragmentado**. O validador reconhece apenas `Status: Draft`
(`internal/validator/validator.go:1221-1235`), mas `trackfw adr new` — o caminho normal — emite
`Proposed`. Só `NewADRDraft` (chamado por `req new`, `internal/commands/req.go:110`) produz
`Draft`.

Duas consequências verificadas:

1. A regra existente `blocked_by_draft_adr` é **cega a `Proposed`** — uma REQ `Open` bloqueada
   por ADR criado com `adr new` não dispara nada.
2. **Não existe regra** para ADR não aceito referenciado por REQ `Done`.

Mesma raiz: não há noção canônica de "ADR não aceito", há um literal espalhado pelo código.

## Acceptance Criteria

- [x] **AC1** — Helper canônico único por CLI. **Este AC reprovou na primeira auditoria:** Node e
      Python cumpriam, mas o Go tinha **três** cópias de
      `EqualFold(status,"Draft")||EqualFold(status,"Proposed")` em produção, e o helper que
      deveria ser canônico era chamado **só pelos testes**. Corrigido no ML-1F
      (`statusIsNotAccepted` como expressão única). Verificado: `grep` retorna 1 linha.
- [x] **AC2** — `blocked_by_draft_adr` passa a usar o helper e **deixa de ser cega a `Proposed`**:
      REQ `Open` bloqueada por ADR `Proposed` produz violação. O **nome da regra não muda**.
- [x] **AC3** — Regra nova `adr_accepted_when_req_done`, severidade **`error`**, registrada no
      mapa `Rules` default (`internal/config/config.go:84` e equivalentes): ADR não aceito
      referenciado por REQ `Done` é violação.
- [x] **AC4** — A mensagem de violação identifica **os dois artefatos** — qual ADR e qual REQ —
      para que o usuário saiba o que corrigir sem investigar.
- [x] **AC5** — **"Aceito" é definido por exclusão**: qualquer status que não seja `Draft` nem
      `Proposed` conta como aceito. `Superseded`, `Deprecated` e `Rejected` **não** produzem
      violação — REQ `Done` apoiada em ADR posteriormente substituído é histórico legítimo.
      Coberto por teste explícito.
- [x] **AC6** — REQ `Open` (ou qualquer status que não `Done`) referenciando ADR não aceito
      **não** dispara a regra nova — é o fluxo normal de trabalho em andamento.
- [x] **AC7** — `trackfw validate` **verde neste repositório** após a mudança. Nenhum artefato
      existente é invalidado — todos os 17 ADRs estão `Accepted` hoje.
- [x] **AC8** — Paridade dos 3 CLIs: mesma regra, mesmo nome, mesma severidade default, mesma
      mensagem. `scripts/check-artifact-parity.sh` e `scripts/check-validate-parity.sh` passam.
- [x] **AC9** — Cenário permanente com **15 asserções** (2 regras × 3 CLIs × baseline/detecção,
      mais um braço `superseded-not-a-violation` por CLI). Contador **42 → 57**.
      **Além do exigido:** `check-validate-parity.sh` passava **vacuamente** — compara só
      `(rule, file)` e este repo não tem artefato violador. Ganhou fixture violadora e **guard de
      vacuidade por regra**, provado capaz de falhar.
- [x] **AC10** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não renomear `blocked_by_draft_adr`.** Nomes de regra são chave pública de configuração
  (`rules:` no `trackfw.yaml`); renomear quebraria silenciosamente projetos que ajustaram a
  severidade. O nome fica historicamente impreciso — aceito no ADR.
- **Não unificar os geradores** fazendo `NewADRDraft` emitir `Proposed`. `Draft` (stub automático)
  e `Proposed` (decisão redigida) têm semânticas distintas, e a mudança invalidaria ADRs `Draft`
  existentes em projetos downstream.
- **Não usar allowlist fechada de status aceitos.** A definição é por exclusão — enumerar
  quebraria projetos com vocabulário próprio.
- Não alterar `extractRefPath`, `referenceExists` nem as demais regras existentes.
- Não alterar o status de nenhum ADR ou REQ do repositório como forma de "passar" no gate.
- Não mexer em `pypi/build/lib/`.
- Não adicionar devDependency nem dependência Python.

## Notas de implementação

Pontos exatos verificados em 2026-08-01:

| | Helper atual (`Draft` hardcoded) | Regras default | Regra a corrigir |
|---|---|---|---|
| Go | `internal/validator/validator.go:1221-1235` | `internal/config/config.go:84` | chamadas em `:1136`, `:1172`, `:1217` |
| Node | equivalente em `npm/src/validator/index.js` | config Node | idem |
| Python | `pypi/trackfw/validator.py:396` | config Python | `:864`, `:881` |

Geradores: `internal/generators/adr.go:60,67` emite `Proposed`; `:214` (`NewADRDraft`) emite
`Draft`.

Os três CLIs têm arquivos disjuntos → os MLs de implementação podem rodar **em paralelo**. Mas
`make parity` e `make quality` só fecham com os três prontos, porque os gates comparam
comportamento entre CLIs — a paridade é barreira de wave posterior, não critério dos MLs
individuais. Mesmo padrão do ciclo do PR #96.

**Atenção ao AC9:** diferente do caso do DOMPurify, aqui o seam é shell puro (gerar artefato,
rodar `validate`, conferir violação) — portanto **é viável em CI** e deve ser gate permanente,
não verificação de auditoria.

`check-gates-falsify.sh` leva mais de 2 min; rodar em background.

## Linked ADR

ADR: docs/adr/ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito-e-regra-de-aceite-exigido-por-req-concluida.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida.md
