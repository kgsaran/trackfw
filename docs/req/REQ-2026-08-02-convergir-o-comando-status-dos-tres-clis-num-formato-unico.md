---
status: Done
date: 2026-08-02
author: "Zeus"
adr: "docs/adr/ADR-2026-08-02-formato-unico-do-comando-status-nos-tres-clis.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-02-convergir-o-comando-status-dos-tres-clis-num-formato-unico.md"
---

# REQ: Convergir o comando status dos tres CLIs num formato unico

> Date: 2026-08-02 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Ponto **1** da fila do PR #103 — o último. `trackfw status` tem **duas implementações
completamente diferentes**: Go/Node exibem uma visão acionável (WIP, Blocked, Done) com moldura;
Python exibe um inventário de contagens sem moldura. O mesmo comando responde coisas diferentes
conforme o runtime instalado.

Ao comparar, dois defeitos silenciosos do Python vieram junto:

1. **`analyzing` omitido** — `pypi/trackfw/commands/status.py` enumera 5 dos 6 estados em três
   pontos (~73, ~81, ~141). Roadmap em `analyzing/` some da contagem.
2. **`Done` e `Closed` agrupados** como "Closed", apagando a distinção entre REQ entregue e REQ
   encerrada sem entrega.

## Acceptance Criteria

- [x] **AC1** — Os 3 CLIs produzem saída **byte-idêntica** para o mesmo projeto, no formato:

```
── trackfw status ──────────────────────

📊 Inventory
   ADRs        <n>
   REQs        <n>  (<n> Open · <n> Done · <n> Closed)
   Roadmaps    <n>
     backlog <n> · analyzing <n> · wip <n>
     blocked <n> · done <n> · abandoned <n>

🔄 WIP (<n>)

❌ Blocked (<n>)

✅ Done (last 5)
   <arquivo>

────────────────────────────────────────
```

- [x] **AC2** — Os **seis** estados de roadmap são enumerados, incluindo `analyzing`. Teste com
      roadmap em `analyzing/` prova que ele **é contado** — antes não era.
- [x] **AC3** — REQs discriminadas por status real (`Open`, `Done`, `Closed`), não agrupadas.
      Teste com um de cada.
- [x] **AC4** — A seção condicional `⏳ REQs blocked by not-accepted ADRs` passa a existir também
      no **Python**, com texto idêntico ao de Go/Node, incluindo o sufixo de status real por ADR.
- [x] **AC5** — Rótulos permanecem **hardcoded em inglês** (`Inventory`, não `Inventário`).
      **Não** passar a saída do `status` por i18n neste ciclo.
- [x] **AC6** — Nenhuma informação das duas visões originais é perdida.
- [x] **AC7** — `scripts/check-artifact-parity.sh` e `scripts/check-validate-parity.sh` passam.
- [x] **AC8** — Cenário de falsificação permanente comparando a saída do `status` dos 3 CLIs
      **contra um literal pinado** — não os três entre si, que passaria se todos derivassem
      juntos. Fixture com **roadmap em `analyzing/`** e **REQs `Open`/`Done`/`Closed`**, senão o
      cenário não discrimina os AC2 e AC3.
- [x] **AC9** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não passar a saída do `status` por i18n.** Os rótulos já eram hardcoded; a dívida é registrada,
  não ampliada em natureza. É candidato a REQ própria.
- **Não traduzir rótulos** — `Inventory` em inglês, junto de `WIP`/`Blocked`/`Done`.
- Não alterar `validate` nem suas regras — só o comando `status`.
- Não alterar o formato `--json`, se existir, sem que o AC exija.
- Não alterar o dashboard (`serve`) — superfície diferente.
- **Não resolver a divergência do delimitador não pareado** (item 4 da fila, medido no PR #104).
- Não alterar o status de nenhum ADR ou REQ do repositório.
- Não mexer em `pypi/build/lib/`; não adicionar dependência.

## Notas de implementação

| | Implementação de `status` |
|---|---|
| Go | `internal/validator/validator.go` → `GetStatus()` (~linha 701) |
| Node | `npm/src/validator/index.js` → `getStatus()` (~linha 1317), **async** |
| Python | `pypi/trackfw/commands/status.py` — estados hardcoded em ~73, ~81, ~141 |

Este ciclo **não é breaking change**: o trackfw ainda não tem usuários externos, então não há
saída consumida por terceiros a proteger. O custo é interno — fixtures e asserções dos 3 CLIs.

Os 3 CLIs têm arquivos disjuntos → MLs podem rodar **em paralelo**. Mas a saída precisa ficar
byte-idêntica, o que só se verifica com os três prontos → paridade é barreira de wave posterior.
Dado o histórico deste projeto (três MLs paralelos divergiram em **todos** os ciclos anteriores),
**espere um ML de reconciliação** e reserve orçamento para ele.

`check-gates-falsify.sh` leva mais de 2 min; rodar em background.

## Linked ADR

ADR: docs/adr/ADR-2026-08-02-formato-unico-do-comando-status-nos-tres-clis.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-02-convergir-o-comando-status-dos-tres-clis-num-formato-unico.md
