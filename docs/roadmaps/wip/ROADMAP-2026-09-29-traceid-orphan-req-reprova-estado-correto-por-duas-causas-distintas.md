---
status: wip
date: 2026-09-29
req: "docs/req/REQ-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md"
squad: ""
---

# Roadmap: `traceid_orphan_req` reprova estado correto por duas causas distintas

> Created: 2026-09-29 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md  (vigente — a de 2026-09-29 era duplicata, abandonada)
ADR: docs/adr/ADR-2026-09-29-quando-uma-req-deve-ter-roadmap-e-o-casamento-req-roadmap-nao-depende-de-req-id-no-roadmap.md
Origem: **#435**. Duas causas reproduzidas na v9.1.0: REQ que ainda não começou, e par válido
invisível porque `roadmap new` não escreve `req_id:`.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves. -->
- [ ] Regras que decidem "esta REQ deveria ter roadmap?" enumeradas, com critério e divergências
- [ ] C1 fechada por recorte **semântico** (`status:`), nunca por pasta da REQ
- [ ] C2 fechada por casamento via vínculo real (`req:`), não só escrevendo `req_id:` no gerador
- [ ] Par já existente deixa de disparar **sem alterar arquivo nenhum**
- [ ] REQ `Done` sem roadmap **ainda** dispara — a regra continua servindo para algo
- [ ] Delta de violações medido no corpus, com a razão de cada uma que sair
- [ ] `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: quantas regras decidem isso, e o que elas discordam
> Dependências: nenhuma. 🔴 **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(grep -rn 'traceid_orphan_req\|req_has_roadmap' --include='*.go' internal/validator/ | grep -v _test | wc -l | tr -d ' '); test "$n" -gt 0 && echo "Gate W0: $n sitios das duas regras no produto — a triagem parte deste universo" || { echo "GATE FALHOU: zero sitios — a regua esta quebrada, nao o produto" >&2; exit 1; }
```

### ML-0A — enumerar as regras, medir a divergência e refutar a ADR
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29 · 🔴 **refutou a ADR e achou duplicata minha**
**Arquivos:** leitura de `internal/validator/`; escrita em `docs/seguranca/2026-09-29-wave0-orphan-req.md`

**Tarefa:**
1. Enumerar **todas** as regras que decidem *"esta REQ deveria ter roadmap?"* — pelo menos
   `traceid_orphan_req` e `req_has_roadmap`. Para cada uma: critério exato, severidade efetiva,
   isenções (cutoff, grandfathering, baseline) e **em que casos discordam entre si**.
2. 🔴 **Medir a divergência numa fixture única** — o sintoma mais confiável de duas implementações.
3. Medir o **corpus deste repositório**: quantas REQs disparariam cada regra hoje, e quantas
   disparariam sob o critério proposto (`status: Done` sem roadmap).
4. 🔴 **Refutar ou confirmar D3** — que casar pelo `req:` do roadmap resolve C2 **sem migração**.
   Ataque: existe roadmap com `req:` **vazio**, **stale**, com caminho **relativo/absoluto**
   divergente, ou apontando para REQ de **outro** agente? A REQ-2026-09-28 (#452) mexeu nessa
   direção — **leia o que ela já resolveu** antes de presumir.
5. **Decidir sobre o #273:** o issue o cita como parente. Meça — mesma causa, ou só sintoma parecido?
   A Regra Dura exige medição escrita **tanto para separar quanto para juntar**.

**Critérios de aceite:**
- [x] **4 regras, não 2** — `req_has_roadmap`, `traceid_orphan_req`, `ref_targets_exist` (nenhuma
      filtra por estado) e `req_roadmap_lifecycle` (**filtra** — só `Open`). O precedente de regra
      consciente de estado **já existe** no produto
- [x] Fixture única: par válido → `req_has_roadmap` **PASS** e `traceid_orphan_req` **FAIL**, na
      mesma execução. Duas regras, mesmo fato, vereditos opostos
- [x] Corpus de 237 REQs: hoje **0 violations / 13 warnings** grandfathered (3 `Done`, 10
      `Superseded`); sob D2, **9 REQs `Done` pós-cutoff** continuariam violando — o sinal legítimo
      sobrevive
- [x] 🔴 **D3 confirmado PARCIALMENTE, com o limite medido:** fecha C2 para pares gerados pelo
      `roadmap new`, mas **58 roadmaps** ficam fora (32 `req:` stale · 8 vazio/null · 18 sem campo).
      E **exige `normalizeRefSeparator`** — um `req:` com `\` não casa sem ele. Tudo declarado na ADR
- [x] **#273 fica FORA, e está CLOSED.** Medição: lá é algoritmo de match de slug em
      `branch_has_wip_roadmap`, que **já é** consciente de estado. Falsificação nas duas direções —
      corrigir C1/C2 não afeta slug matching, e vice-versa

**Achados que mudaram a governança antes de qualquer código:**

| # | veredito |
|---|---|
| **F5** | 🔴 **refutou a ADR** — "o cutoff ficaria redundante" é **falso**: 3 dos 13 grandfathered são `Done` |
| **F2** | a ADR nomeava **2** regras; são **4** — D4 ampliado |
| **F4** | `Superseded`/`Closed` sem comportamento declarado (14+2 no corpus) — decidido em **D2-bis**: não disparam |
| **F3** | limite de D3 medido: **58 roadmaps** fora do alcance — declarado, não omitido |
| **F1** | 🔴 **refutado pelo arquiteto:** o AC8 da REQ-2026-09-28 estava desmarcado por **negligência de registro**, não por trabalho faltando — o guard existe, tem teste, e o teste passa |
| **F6** | 🔴 **duplicata minha** — ver a seção de consolidação na REQ vigente |

🔴 **E a medição que refinou o desenho:** `e.state` **existe** no `reqIndex` e é consumido por
`traceid_state_mismatch`. Eu ia recusá-lo só invocando a ADR-2026-09-03 D1. A razão real é melhor:
**em layout plano ele é vazio** (`validator_traceid.go:77`), e a regra ficaria **inerte** no layout
deste repositório. Ver **D1-bis** da ADR.

## Wave 1 — o recorte semântico e o casamento por vínculo real
> Dependências: **Wave 0 auditada.** O desenho dos MLs sai da Wave 0 — se ela refutar D3, este
> bloco muda antes de ser despachado.

### ML-1A — (a definir pela Wave 0)
**Status:** ⬜ Pendente
Placeholder consciente: o recorte exato depende do veredito sobre D3 e da contagem do corpus.
🔴 **Não despachar antes da Wave 0 auditada.**

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o binário.
**Critérios de aceite:**
- [ ] Os dois cenários do #435 reconstruídos do zero
- [ ] Contra-braço: REQ `Done` sem roadmap **ainda** dispara
- [ ] Veredito: a regra continua detectando o que deveria, ou virou no-op?
