---
status: wip
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-convergir-o-comando-status-dos-tres-clis-num-formato-unico.md"
squad: ""
---

# Roadmap: Convergir o comando status dos tres CLIs num formato unico

> Created: 2026-08-02 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-02-convergir-o-comando-status-dos-tres-clis-num-formato-unico.md
ADR: docs/adr/ADR-2026-08-02-formato-unico-do-comando-status-nos-tres-clis.md

Ponto **1** da fila do PR #103 — o último. `trackfw status` tem **duas implementações
completamente diferentes**: Go/Node com visão acionável e moldura; Python com inventário de
contagens. Dois defeitos silenciosos do Python vieram junto: `analyzing` omitido da enumeração
(5 de 6 estados, em 3 pontos) e `Done`/`Closed` agrupados.

**Não é breaking change.** KG confirmou que o trackfw ainda não tem usuários externos — não há
saída consumida por terceiros. O custo é interno: fixtures e asserções dos 3 CLIs.

### Formato alvo (decidido com KG)

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

Rótulos **em inglês e hardcoded**, como já eram — `Inventory`, não `Inventário`. Passar o `status`
por i18n é escopo próprio.

### Dependências e paralelismo

Wave 1 com 3 MLs em paralelo (arquivos disjuntos). Mas a saída precisa ficar **byte-idêntica**, e
isso só se verifica com os três prontos → paridade é Wave 3.

**Aviso baseado no histórico:** nos ciclos anteriores deste projeto, os três MLs paralelos
divergiram **todas as vezes** — em fonte de dado, em texto de mensagem, e em raio de alcance da
mudança. A **Wave 2 é um ML de reconciliação já previsto**, executado por **um único** agente nos
3 CLIs, porque garantir saída byte-idêntica com um executor é mais seguro que coordenar três.

## Critérios de Aceite

- [ ] Saída byte-idêntica nos 3 CLIs no formato alvo
- [ ] Os 6 estados enumerados; roadmap em `analyzing/` **é contado** (antes não era)
- [ ] REQs discriminadas em `Open`/`Done`/`Closed`
- [ ] Seção `⏳ REQs blocked by not-accepted ADRs` também no Python, texto idêntico
- [ ] Rótulos hardcoded em inglês; sem i18n neste ciclo
- [ ] Gates de paridade passam
- [ ] Cenário de falsificação contra **literal pinado**, com fixture cobrindo `analyzing` e os 3 status de REQ
- [ ] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 1 — Implementação por CLI (3 MLs EM PARALELO)
> Dependências: nenhuma. Arquivos disjuntos.

### ML-1A — CLI Go
**Status:** pending
**Agente:** Apolo
**Arquivos afetados:** `internal/validator/validator.go` (`GetStatus`, ~701) + testes Go

**Ações:** acrescentar o bloco `📊 Inventory` no topo, dentro da moldura existente; enumerar os
**6** estados; discriminar REQs por status real.

**Acceptance criteria:**
- [ ] `make build`, `make lint`, `go test ./...` verdes
- [ ] Teste: roadmap em `analyzing/` aparece na contagem
- [ ] Teste: REQs `Open`/`Done`/`Closed` discriminadas
- [ ] Formato exato do ADR reproduzido
- [ ] Não tocar em `npm/`, `pypi/`; não mexer em `validate`; não adicionar i18n

### ML-1B — CLI Node
**Status:** pending
**Agente:** Apolo
**Arquivos afetados:** `npm/src/validator/index.js` (`getStatus`, ~1317, **async**) + testes Node

**Acceptance criteria:** equivalentes ao ML-1A (`npm test` verde).
- [ ] Não tocar em `internal/`, `pypi/`

### ML-1C — CLI Python
**Status:** pending
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/commands/status.py` + testes Python

**Maior mudança dos três** — o Python passa a ter moldura, seções acionáveis (WIP/Blocked/Done)
e a seção condicional `⏳ REQs blocked by not-accepted ADRs`, que hoje não existe nele.

**Acceptance criteria:** equivalentes ao ML-1A, **mais**:
- [ ] Seção `⏳` implementada com texto idêntico ao de Go/Node
- [ ] Estados hardcoded corrigidos nos **três** pontos (~73, ~81, ~141)
- [ ] Não tocar em `internal/`, `npm/`, `pypi/build/lib/`

---

## Wave 2 — Reconciliação (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Saída byte-idêntica nos 3
**Status:** pending
**Agente:** Apolo (executor **único**, deliberadamente)

Comparar a saída dos 3 CLIs contra a **mesma** fixture e eliminar qualquer divergência de
espaçamento, alinhamento, pluralização ou ordem. Provar igualdade **rodando os CLIs e
diferenciando a saída** (`diff`/`od -c`), não comparando literais no código.

**Acceptance criteria:**
- [ ] `diff` das três saídas contra a mesma fixture: vazio
- [ ] Divergências encontradas listadas no relatório, com a decisão tomada em cada uma
- [ ] Testes dos 3 CLIs verdes

---

## Wave 3 — Barreira (1 ML)
> Dependências: **Wave 2 aprovada**

### ML-3A — Paridade e seam
**Status:** pending
**Agente:** Ártemis

**Ações:**
1. Gates de paridade passam; `make quality` exit 0.
2. Confirmar que os **65** cenários existentes seguem passando.
3. **Cenário permanente:** saída do `status` dos 3 CLIs comparada contra **literal pinado** — não
   os três entre si, que passaria se todos derivassem juntos. Fixture **precisa** conter roadmap
   em `analyzing/` e REQs `Open`/`Done`/`Closed`, senão não discrimina AC2 e AC3.
4. Braço de detecção: reverter a enumeração de `analyzing` num CLI → a contagem muda → o gate acusa.
5. Contador e linha final atualizados.

**Acceptance criteria:**
- [ ] Gates passam; `make quality` exit 0
- [ ] 65 cenários herdados confirmados
- [ ] Cenário novo com fixture discriminante; provado não vacuoso
- [ ] Contador atualizado
- [ ] `git status --porcelain` sem resíduo
