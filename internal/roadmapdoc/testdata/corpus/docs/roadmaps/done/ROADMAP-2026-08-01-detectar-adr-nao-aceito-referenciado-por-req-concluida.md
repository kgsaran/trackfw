---
status: done
date: 2026-08-01
req: "docs/req/REQ-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida.md"
squad: ""
---

# Roadmap: Detectar ADR nao aceito referenciado por REQ concluida

> Created: 2026-08-01 | Status: done

## Context

REQ: docs/req/REQ-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida.md
ADR: docs/adr/ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito-e-regra-de-aceite-exigido-por-req-concluida.md

O `ADR-2026-07-26` ficou `Proposed` enquanto **7 REQs que ele governa** foram concluídas. Nenhum
gate detectou.

Causa mais profunda: **o vocabulário de "ADR não aceito" está fragmentado**. O validador só
reconhece `Status: Draft`, mas `adr new` — o caminho normal — emite `Proposed`. Só `NewADRDraft`
(chamado por `req new`) produz `Draft`.

Daí **duas** lacunas com a mesma raiz: a `blocked_by_draft_adr` é cega a `Proposed`, e não existe
regra para ADR não aceito referenciado por REQ `Done`.

### Decisão do ADR

Helper canônico `adrNotAccepted` (`Draft` **ou** `Proposed`) como único dono do vocabulário;
`blocked_by_draft_adr` passa a usá-lo (**sem renomear** — o nome é chave pública de config);
regra nova `adr_accepted_when_req_done` com severidade `error`; "aceito" definido **por exclusão**,
preservando `Superseded`/`Deprecated`/`Rejected`.

### Pontos de código verificados em 2026-08-01

| | Helper (`Draft` hardcoded) | Chamadores | Regras default |
|---|---|---|---|
| Go | `internal/validator/validator.go:1221-1235` | `:1136`, `:1172`, `:1217` | `internal/config/config.go:84` |
| Node | `npm/src/validator/index.js` | — | config Node |
| Python | `pypi/trackfw/validator.py:396` | `:864`, `:881` | config Python |

Geradores: `internal/generators/adr.go:60,67` → `Proposed`; `:214` (`NewADRDraft`) → `Draft`.

### Dependências e paralelismo

**Wave 1 tem paralelismo real** — cada CLI tem validador e config próprios, arquivos disjuntos.

`make parity` e `make quality` **falham** até os três estarem prontos, porque os gates comparam
comportamento entre CLIs. Por isso **nenhum ML da Wave 1 tem `parity` nos critérios**; cada um
valida só o próprio CLI. A paridade é a Wave 2, que age como barreira. Mesmo padrão do PR #96.

## Critérios de Aceite

Consolidados da REQ (AC1–AC10). Detalhamento por microlote abaixo.

- [ ] Helper canônico único por CLI cobrindo `Draft` e `Proposed`; nenhum literal solto
- [ ] `blocked_by_draft_adr` usa o helper e deixa de ser cega a `Proposed`; **nome inalterado**
- [ ] Regra nova `adr_accepted_when_req_done`, severidade `error`, no mapa de regras default
- [ ] Mensagem identifica **ADR e REQ**
- [ ] `Superseded`/`Deprecated`/`Rejected` **não** violam; REQ não-`Done` **não** dispara
- [ ] `validate` verde neste repositório
- [ ] Paridade dos 3 CLIs; cenário de falsificação permanente cobrindo as 2 regras × 3 CLIs
- [ ] `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes

---

## Wave 1 — Validadores (3 MLs EM PARALELO)
> Dependências: nenhuma. Arquivos disjuntos por CLI.

### ML-1A — CLI Go
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `internal/validator/validator.go`, `internal/config/config.go` + testes Go

**Ações:** criar o helper canônico; migrar `adrDraftStatusForRule` e os 3 chamadores para ele;
implementar `adr_accepted_when_req_done`; registrar no mapa `Rules`; testes.

**Acceptance criteria:**
- [ ] `make build`, `make lint`, `go test ./...` verdes
- [ ] Teste: REQ `Done` + ADR `Proposed` → violação, mensagem cita **ambos**
- [ ] Teste: REQ `Done` + ADR `Draft` → violação
- [ ] Teste: REQ `Done` + ADR `Superseded` → **sem** violação (definição por exclusão)
- [ ] Teste: REQ `Open` + ADR `Proposed` → **sem** violação da regra nova
- [ ] Teste: REQ `Open` **bloqueada** por ADR `Proposed` → violação de `blocked_by_draft_adr`
      (é a correção da cegueira; antes não disparava)
- [ ] `bin/trackfw validate` verde neste repositório
- [ ] Não tocar em `npm/`, `pypi/`; não renomear a regra existente

### ML-1B — CLI Node
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `npm/src/validator/index.js` + config Node + testes Node

**Acceptance criteria:** equivalentes ao ML-1A, no CLI Node (`npm test` verde).
- [ ] Não tocar em `internal/`, `pypi/`

### ML-1C — CLI Python
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/validator.py` + config Python + testes Python

**Acceptance criteria:** equivalentes ao ML-1A, no CLI Python.
- [ ] Não tocar em `internal/`, `npm/`, `pypi/build/lib/`

---

### ML-1D — Reconciliação entre os 3 CLIs (corretivo)
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo (executor único, deliberadamente)

A wave paralela divergiu em **três** dimensões, e **nenhum gate pegaria**:

1. **Fonte do status** — Go e Python liam frontmatter-first; Node só o cabeçalho. Caso
   discriminante: ADR com frontmatter e **sem** linha de cabeçalho.
2. **Falso-positivo de prosa** — o mecanismo herdado varria o documento inteiro com
   `Contains("Status: Draft")`. O **próprio ADR deste ciclo** cita esses literais ao descrever o
   bug: seria flagrado pela regra que o documenta.
3. **Truncamento no pipe** — o fallback do Python não truncava no próximo `" |"`. Cabeçalho
   legado `| Status: Draft | Owner: kg` resolvia `"Draft | Owner: kg"` → falso-**negativo**
   silencioso. Nenhuma fixture cobria esse formato.

Executado por **um único** agente nos 3 CLIs, de propósito: garantir strings byte-idênticas com
um executor é mais seguro que coordenar três. Igualdade provada **rodando os CLIs e diferenciando
a saída**, não comparando literais.

### ML-1E — Rótulo `(Draft)` no bloco de resumo (corretivo)
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo

O cabeçalho `⏳ REQs blocked by Draft ADRs` e o sufixo `(Draft)` são **pré-existentes** — mas a
nossa mudança os tornou mentirosos, porque `blockedREQs()` passou a incluir `Proposed`. Agora
exibem o **status real**. Python não tem esse bloco: lacuna de paridade pré-existente, **não**
fechada aqui (seria feature nova).

### ML-1F — Helper único no Go (corretivo)
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo

**Violação de AC1 detectada na auditoria da Wave 2.** O ADR decidiu um dono único do vocabulário;
Node e Python cumpriam, o Go **não**: três cópias de
`EqualFold(status,"Draft")||EqualFold(status,"Proposed")` em produção, e o helper que deveria ser
canônico era chamado **só pelos testes**. Sem bug funcional — as três cópias eram idênticas — mas
é exatamente a dívida que este ciclo existiu para eliminar.

Refatoração pura: `statusIsNotAccepted(status)` como única expressão. Nenhum teste precisou mudar
de expectativa, o que confirma comportamento inalterado.

---

## Wave 2 — Barreira: paridade e falsificação (1 ML)
> Dependências: **ML-1A, ML-1B e ML-1C completos e auditados**

### ML-2A — Paridade e seam permanente
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Ártemis

**Ações:**
1. `check-artifact-parity.sh` e `check-validate-parity.sh` passam; `make quality` exit 0;
   `validate` verde.
2. Conferir que os **42 cenários existentes** seguem passando — a `blocked_by_draft_adr` ficou
   mais rigorosa e pode afetar fixtures.
3. **Cenário de falsificação permanente** cobrindo **as duas** regras × **3** CLIs: neutralizar o
   helper (ou remover a regra) e provar que o ciclo que deveria falhar **passa** — seam vivo.
   Restaurar. Shell puro, portanto viável em CI, ao contrário do caso do DOMPurify.
4. Atualizar contador e linha final do script.

**Acceptance criteria:**
- [x] Gates de paridade passam; `make quality` exit 0; `validate` verde nos 3
- [x] 42 cenários herdados confirmados passando
- [x] Cenário novo cobrindo 2 regras × 3 CLIs — **15 asserções**, contador **42 → 57**
- [x] Cenário novo provado não vacuoso
- [x] `git status --porcelain` sem resíduo

**O problema central que este ML resolveu:** `check-validate-parity.sh` passava **vacuamente**.
Compara só `(rule, file)`, e este repositório **não tem artefato que viole a regra nova** — o gate
passaria igualmente se a regra não existisse em CLI nenhum.

A Ártemis reforçou o gate com **fixture violadora** e um **guard de vacuidade por regra**: cada
runtime precisa produzir `adr_accepted_when_req_done` **e** `blocked_by_draft_adr`. Provou o guard
capaz de falhar mudando a fixture para `Accepted` — o gate acusa
`the fixture no longer exercises them or a CLI regressed (vacuous check)`.

Cada CLI tem 5 asserções, incluindo um braço `superseded-not-a-violation` que prova que a
definição **por exclusão** continua valendo.
