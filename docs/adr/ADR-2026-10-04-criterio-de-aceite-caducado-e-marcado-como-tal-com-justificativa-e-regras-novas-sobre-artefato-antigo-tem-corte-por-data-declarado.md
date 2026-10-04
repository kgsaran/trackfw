---
status: Accepted
date: 2026-10-04
author: "trackfw_architect"
---

# ADR: critério de aceite caducado é marcado como tal, com justificativa, e regra nova sobre artefato antigo tem corte por data declarado

> Date: 2026-10-04 | Status: Accepted

REQ: `docs/req/REQ-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel-sem-afirmar-algo-falso-e-a-exigencia-de-wave-0-vale-para-roadmap-anterior-a-adr-que-a-criou.md`
Origem: **#514** (consumidor externo, `by_agent`, 82 roadmaps).

## Context

Existe uma classe de critério de aceite que **não pode mais ser verificada**: o artefato que ele cita foi
removido (a suíte pypi saiu na v8) ou o número literal caducou (`version` reporta 7.3.0). Marcar `[x]`
afirmaria algo falso. Deixar `[ ]` hoje tem o seguinte efeito.

**Medido com o binário da `main` (`a481da42`), numa árvore de fixture, em 2026-10-04:**

| situação | resultado |
|---|---|
| `validate`, REQ `Done` com `- [ ]` aberto | **passa** (não há regra) |
| `barrier --wave N`, ML `✅` com `- [ ]` aberto | **`result: blocked`**, "ML-1A: 1 unmet acceptance criteria" (`barrier.go:805`) |
| `roadmap move … done`, ML `❌ Bloqueado` | **bloqueia** (`pendingMLsForDone`) |
| `roadmap move … done`, ML `✅` com `- [ ]` aberto | passa |

Então o caminho honesto é deixar a caixa aberta e marcar o ML como bloqueado, e esse caminho trava o
`barrier` e o `done`. O caminho que passa é mentir na caixa. A #514 descreve isso pela REQ, mas **o lado
que trava é o roadmap**. Pela REQ, o produto não impede nada, e é por isso que **113 das 196 REQs `Done`
deste repositório têm caixa aberta**. Hoje, uma caixa de AC numa REQ fechada não significa nada.

Segundo achado da #514: a exigência de `## Wave 0` (ADR-2026-09-18, decisão 8) vale no `move … done` sem
corte por data. **154** roadmaps em `done/` não têm Wave 0, todos com data até 2026-09-16. Fora de
`done/`, 4 (3 em `backlog/` e 1 em `blocked/`, de 2026-09-08 a 09-12) **não podem chegar a `done/`**.
O `req_has_roadmap` já resolveu o mesmo problema com um corte declarado
(`validator_req_roadmap_cutoff.go`).

## Decision

### D1 — Não há estado novo
Os seis estados continuam (decisão do KG, 2026-10-04). Caducar é atributo do **critério**, não do roadmap.

### D2 — Forma do critério caducado
A caixa **continua `- [ ]`**, porque o critério de fato não foi atendido, e toda contagem existente
continua verdadeira. Logo abaixo do item vem uma linha de continuação indentada:

```markdown
- [ ] Suíte pypi sem regressão contra 95 falhas
      Caducou: a v8.0.0 removeu o CLI Python (#365); não há suíte para rodar.
```

`Caducou:` exige texto não vazio depois dos dois-pontos. Sem justificativa, a linha não conta como
caducado. Esta forma foi escolhida no lugar de um token novo (`[~]`), que cada parser de caixa teria
de aprender e que, nos que não aprendessem, contaria em silêncio como nem atendido nem pendente.

### D3 — O `barrier` reconhece o caducado
Critério aberto com `Caducou:` justificado **não** conta como "unmet". O relatório o mostra à parte
(`ML-1A: 1 lapsed acceptance criterion`), no texto e no JSON. Critério aberto sem `Caducou:` continua
bloqueando como hoje.

### D4 — Regra nova para REQ `Done` com critério aberto, com corte na data de entrada
Regra `req_done_open_criteria`: REQ `Done` com `- [ ]` sem `Caducou:` → **warning**.
- **Corte: 2026-10-04, a data de entrada da regra.** REQ criada antes do corte é isenta, e as isentas são
  ditas em **uma** linha agregada, no padrão do `req_has_roadmap`.
- 🔴 **A direção do corte é a oposta à do `req_has_roadmap`.** Lá a regra já existia como erro, e o corte
  concedia anistia; anistia mínima era a data mais antiga. Aqui a regra é **nova**. Um corte anterior à
  entrada condenaria, nas árvores de consumidor, REQs fechadas sob o regime antigo, o que é exatamente o
  segundo achado da #514. Que o corte de 2026-09-30 zere **este** acervo não prova nada sobre consumidor.
- A data vem da mesma régua do `reqCreationDate`: `date:` do frontmatter primeiro, nome do arquivo como
  fallback.
- Severidade padrão `warning`: a regra nasce visível, sem reprovar. Endurecer é decisão futura.

### D5 — Corte por data para a exigência de Wave 0
`HasWave0` exigido só para roadmap com data **a partir de 2026-09-18** (data da ADR que criou a
exigência), com a mesma régua de data. Vale em **todo** chamador da exigência: o gate do
`move … done` e a regra `roadmap_wave0_required` (`wip/`), que dispara quando um roadmap antigo volta a
`wip/` pela Regra Dura. Isenção visível: uma linha no `move`, e uma linha agregada no `validate`.

## Consequences

- O consumidor fecha os 4 roadmaps da #514 sem mentir: cada AC caducado ganha `Caducou:`, o `barrier`
  passa, o ML pode ficar `✅`, e a REQ fecha `Done`.
- As 113 REQs `Done` com caixa aberta deste repositório **não são editadas** (D4 as isenta, com uma
  linha agregada).
- **Resíduo declarado:** `date:` é editável. Um roadmap novo com data retroativa escapa da D5, e uma REQ
  nova com data retroativa escapa da D4. É o mesmo resíduo já declarado no `req_has_roadmap`.
- **Resíduo declarado:** `Caducou:` é texto livre. O produto verifica que a justificativa existe, não que
  ela é verdadeira.

## Alternatives Considered

- **Estado novo (sétimo).** Rejeitado pelo KG: caducar é atributo do critério.
- **Token `[~]`.** Rejeitado (D2): os parsers que não o aprendessem contariam errado em silêncio.
- **Corte da D4 em 2026-09-30**, que zera este acervo. Rejeitado (D4): é regra nova, e a direção do corte é
  outra.
- **Exigir `Caducou:` também no `move … done`.** Fica fora: o gate do `done` é por status de ML, não
  por caixa, e passa a funcionar pela D3 (ML `✅` honesto).

## Linked REQ

`docs/req/REQ-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel-sem-afirmar-algo-falso-e-a-exigencia-de-wave-0-vale-para-roadmap-anterior-a-adr-que-a-criou.md`
