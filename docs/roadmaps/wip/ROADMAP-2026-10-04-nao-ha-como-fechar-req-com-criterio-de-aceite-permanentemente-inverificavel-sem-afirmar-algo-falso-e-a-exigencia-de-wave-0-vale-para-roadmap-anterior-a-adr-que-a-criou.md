---
status: wip
date: 2026-10-04
req: "docs/req/REQ-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel-sem-afirmar-algo-falso-e-a-exigencia-de-wave-0-vale-para-roadmap-anterior-a-adr-que-a-criou.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: nao ha como fechar REQ com criterio de aceite permanentemente inverificavel sem afirmar algo falso, e a exigencia de Wave 0 vale para roadmap anterior a ADR que a criou

> Created: 2026-10-04 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel-sem-afirmar-algo-falso-e-a-exigencia-de-wave-0-vale-para-roadmap-anterior-a-adr-que-a-criou.md
ADR: docs/adr/ADR-2026-10-04-criterio-de-aceite-caducado-e-marcado-como-tal-com-justificativa-e-regras-novas-sobre-artefato-antigo-tem-corte-por-data-declarado.md
Issue: #514 (label `req-aberta`). Fecha #514.

Medido com o binário da `main` (`a481da42`), numa fixture (`scratchpad/fx514.sh`):
- `barrier` → `blocked` ("ML-1A: 1 unmet acceptance criteria", `internal/commands/barrier.go:805`) para ML
  `✅` com `- [ ]`;
- `move … done` → bloqueia ML `❌` (`internal/generators/roadmap.go`, `pendingMLsForDone`) e falta de
  `## Wave 0` (mesmo arquivo, ~:765);
- `validate` → passa com REQ `Done` e `- [ ]`.

Acervo:
- 113 de 196 REQs `Done` com caixa aberta;
- 154 roadmaps em `done/` sem Wave 0 (até 2026-09-16);
- 4 roadmaps fora de `done/` sem Wave 0.

O corte existente a imitar é `internal/validator/validator_req_roadmap_cutoff.go` (`reqCreationDate`:
`date:` primeiro, nome do arquivo como fallback).

## Acceptance Criteria
- [ ] AC1–AC9 da REQ, cada um com evidência apontável

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model
> Dependências: nenhuma. Bloqueia toda implementação.

### ML-0A — Parsers de caixa, abuso do `Caducou:` e datas retroativas
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-04-wave0-criterio-caducado-e-cortes.md` (único arquivo)
**Actions:**
1. **Completude:** listar **todo** parser de caixa de critério em `internal/` e `scripts/`, excluindo testdata
   de corpus. Para cada um, dizer o que faz hoje com `- [x]` e se precisa reconhecer `Caducou:`: barrier,
   `serve/api_board.go`, `roadmapdoc`, o gate de placeholder de scaffold e os gates em `scripts/`.
2. **Threat model:**
   - `Caducou:` vazio, em comentário HTML, em cerca de código, colado em item que não é critério, ou com
     indentação errada;
   - `Caducou:` usado para esconder critério verificável (o resíduo de texto livre);
   - data retroativa (D4/D5);
   - REQ sem `date:` e sem data no nome.
3. **Alvos de falsificação nas duas direções**, por superfície.
4. **Resíduo declarado.**
**Acceptance criteria:**
- [x] As quatro seções com evidência (comando e saída)
- [x] Lista de parsers fechada, com o veredito "precisa reconhecer / não precisa" para cada um

      ✅ Parecer: 7 parsers enumerados (só o `AcceptanceEvaluate` e o `barrier` contam critério). Os achados T3, T7 e T8 foram incorporados ao ADR como D6. O arquiteto conferiu o T3 no código (`roadmapdoc.go:46` e :640-655) e mediu 0 caixas fora de `[ ]`/`[x]` no acervo.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-04-wave0-criterio-caducado-e-cortes.md
```

## Wave 1 — Implementação
> Dependências: Wave 0 auditada. Dois MLs **sequenciais**, ambos do apolo-tf. O ML-1B usa o helper que o
> ML-1A cria em `roadmapdoc`, e os dois tocam o mesmo pacote de testes.

### ML-1A — Reconhecer `Caducou:` (D2) e o `barrier` (D3)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/roadmapdoc/` (helper e testes), `internal/commands/barrier.go` e testes, e os
demais parsers que a Wave 0 marcar como "precisa reconhecer".
**Actions:**
- Um helper único em `roadmapdoc` decide se um item `- [ ]` é caducado: linha de continuação indentada
  `Caducou: <texto não vazio>`, fora de cerca de código e de comentário.
- O `barrier` conta "lapsed" à parte, no texto e no JSON.
- **D6 (T3 e T8):** só `[x]`/`[X]` contam como atendido, e qualquer outra caixa conta como pendente, com a linha nomeada. ML com 0 atendidos e ≥ 1 caducado fica `blocked`. O texto "unmet acceptance criteria" não muda (pinado em `check-roadmap-barrier-contract.sh:984`).
**Acceptance criteria:**
- [x] AC2 e AC3 com testes, mais uma fixture de ponta a ponta com o binário
- [x] Baseline do barrier (`internal/roadmapdoc/testdata/barrier-baseline.txt`) inalterado onde não há
  `Caducou:`
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/` verde

      ✅ Auditoria do arquiteto, com o binário da branch e a fixture `scratchpad/fx514.sh`: ML `✅` com `[x]` + `[ ]`/`Caducou:` → "~ 1 lapsed" e não bloqueia; `[~]` → bloqueia nomeando a linha. 19 testes conferidos por `go test -list`. `git diff` em `internal/roadmapdoc/testdata/` vazio, ou seja, o baseline está intocado.

**Gates da wave:**
```bash
go build ./...
go test ./internal/roadmapdoc/ ./internal/commands/ -count=1
```

### ML-1B — Regra `req_done_open_criteria` (D4) e corte do Wave 0 (D5)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/validator/` (regra nova, corte e testes), `internal/generators/roadmap.go` (gate
do `done`), `internal/validator/validator_roadmap_gates.go`, `docs/cli-parity.md`
**Actions:**
- Regra D4 como warning, com corte em 2026-10-04 e uma linha agregada. Usa o helper do ML-1A e a régua
  `reqCreationDate`.
- Corte D5 (2026-09-18) nos dois chamadores de `HasWave0`, com isenção visível. A data do roadmap vem de uma `roadmapCreationDate` própria (D6/T7): `date:` primeiro, depois a primeira `AAAA-MM-DD` do nome. Sem data → sem isenção (fail-closed).
- Contrato no `cli-parity.md`.
- Último ML da wave: `make parity-rest` e `go test ./...` são **autorizados e obrigatórios**, e o executor
  repete até ficar verde.
**Acceptance criteria:**
- [x] AC4 e AC5 medidos neste repositório: contagem da linha agregada e os 4 roadmaps sem Wave 0
- [x] AC6
- [x] `make parity-rest` EXIT=0 e `go test ./...` verde

      ✅ Auditoria do arquiteto: a contagem da D4 (126 de 211) corrige a medição inicial do arquiteto (113 de 196), que só aceitava `status: Done` exato.
      Corretivo **ML-1C** (apolo-tf, só testes): o corte da D5 no `move … done` não tinha teste, nem o `lapsed` do `show --json`, e as duas "ProvaDeModida" não sabotavam nada.
      Seis sabotagens de produção (S1–S6), cada uma derrubando ≥ 1 teste; 5 testes novos; as provas decorativas foram removidas.
      `go test ./internal/...` e `make parity-rest` verdes.

**Gates da wave:**
```bash
go build ./...
go test ./internal/... -count=1
```

## Wave 2 — Barreira
> Dependências: Wave 1 auditada. Primeiro o Hades, depois o Hefesto.

### ML-2A — Revisão de segurança contra o threat model
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-04-wave2-revisao-criterio-caducado.md`
**Acceptance criteria:**
- [x] Cada cenário da Wave 0 com veredito, e uma tentativa de fazer o `barrier` passar com `Caducou:`
  inválido
      ✅ APROVA COM AJUSTE. Foram 17 vetores contra o binário da branch, e nenhum passou. Os resíduos R3 (`[~]`), R5 (ML só de caducados) e R6 (data no sufixo) foram eliminados.
      O ajuste (o barrier mostrar a justificativa) entrou no **ML-1D** (apolo-tf): `line N: Caducou: <texto>` no texto e `lapsed_details` no JSON, com corte em 120 caracteres. O arquiteto conferiu na fixture.

### ML-2B — Qualidade e gate completo
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-04-revisao-criterio-caducado.md`
**Acceptance criteria:**
- [ ] `make quality` EXIT=0 com a máquina ociosa e o log conferido

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-04-wave2-revisao-criterio-caducado.md
test -s docs/qualidade/2026-10-04-revisao-criterio-caducado.md
```
