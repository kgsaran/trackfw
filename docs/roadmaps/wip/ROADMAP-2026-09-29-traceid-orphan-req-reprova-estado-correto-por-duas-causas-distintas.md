---
status: wip
date: 2026-09-29
req: "docs/req/REQ-2026-09-29-traceid-orphan-req-reprova-estado-correto-por-duas-causas-distintas-e-o-baseline-virou-o-mecanismo-de-convivencia.md"
squad: ""
---

# Roadmap: `traceid_orphan_req` reprova estado correto por duas causas distintas

> Created: 2026-09-29 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-29-traceid-orphan-req-reprova-estado-correto-por-duas-causas-distintas-e-o-baseline-virou-o-mecanismo-de-convivencia.md
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
**Status:** ⬜ Pendente
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
- [ ] Regras enumeradas com critério, severidade e isenções, **nenhuma sem razão escrita**
- [ ] Tabela de divergência medida em fixture única
- [ ] Contagem no corpus: hoje × sob o critério proposto
- [ ] 🔴 Veredito sobre D3 **antes** de alguém escrever código: casar por `req:` basta?
- [ ] Veredito medido sobre o #273 — entra nesta REQ ou fica fora, com a razão

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
