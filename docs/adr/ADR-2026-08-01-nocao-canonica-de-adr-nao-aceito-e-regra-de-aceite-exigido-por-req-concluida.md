---
status: Accepted
date: 2026-08-01
author: "Zeus"
---

# ADR: Nocao canonica de ADR nao aceito e regra de aceite exigido por REQ concluida

> Date: 2026-08-01 | Status: Accepted

## Context

Em 2026-08-01, ao auditar o backlog, encontrei o
`ADR-2026-07-26-principios-de-design-de-gates-verificaveis` ainda em `Proposed` — referenciado por
**7 REQs, todas `Done`**. Uma decisão que governou sete entregas estava formalmente registrada
como proposta, e **nenhum gate detectou isso** durante todo o período.

Investigando por que o validador não pegou, encontrei um problema mais profundo: **o vocabulário
de "ADR não aceito" está fragmentado entre gerador e validador.**

### Dois estados não-aceitos, e o validador só enxerga um

| Estado | Origem | Reconhecido pelo validador? |
|---|---|---|
| `Proposed` | `trackfw adr new` — o caminho normal (`internal/generators/adr.go:60,67`) | **não** |
| `Draft` | `NewADRDraft`, chamado por `req new` ao criar stubs de ADR bloqueadora (`internal/commands/req.go:110`) | sim |

`adrDraftStatusForRule` (`internal/validator/validator.go:1221-1235`) decide se um ADR é "não
aceito" com um único teste: `strings.Contains(content, "Status: Draft")`. Espelhado em
`pypi/trackfw/validator.py:396` e no equivalente Node.

Consequências verificadas:

1. **A regra `blocked_by_draft_adr` é cega a `Proposed`.** Uma REQ `Open` bloqueada por um ADR
   criado com `adr new` — o caminho normal — não dispara violação alguma. A regra só funciona
   para o subcaso dos stubs gerados automaticamente.
2. **Não existe regra alguma** para ADR não aceito referenciado por REQ **concluída**. Foi essa
   ausência que deixou o ADR-2026-07-26 atravessar sete entregas.

Ambas as lacunas têm a **mesma raiz**: não há uma noção canônica de "ADR não aceito" — há um
literal `"Status: Draft"` espalhado pelo código.

## Decision

**Criar uma noção canônica de "ADR não aceito" e construir as duas regras sobre ela.**

1. **Helper canônico** (nome sugerido: `adrNotAccepted` / `adr_not_accepted`) nos 3 CLIs,
   que devolve verdadeiro para ADR cujo status seja `Draft` **ou** `Proposed`. Passa a ser o
   único lugar que conhece esse vocabulário.
2. **`blocked_by_draft_adr` passa a usar o helper**, deixando de ser cega a `Proposed`.
   O **nome da regra não muda** — renomeá-la quebraria configurações `rules:` existentes em
   projetos que usam o trackfw. O nome fica historicamente impreciso; a alternativa é pior.
3. **Regra nova `adr_accepted_when_req_done`**, severidade **`error`**: um ADR não aceito
   referenciado por uma REQ `Done` é violação. Registrada no mapa `Rules` default
   (`internal/config/config.go:84`), portanto configurável como as demais.
4. **A definição de "aceito" é por exclusão**: qualquer status que não seja `Draft` nem
   `Proposed` conta como aceito. Isso preserva estados de fim de vida como `Superseded`,
   `Deprecated` ou `Rejected` sem exigir enumerá-los — uma REQ `Done` apoiada num ADR
   posteriormente substituído é histórico legítimo, não violação.

## Consequences

**Positivas**

- Fecha o buraco que permitiu sete entregas sobre um ADR não aceito, e o fecha **na origem**:
  o vocabulário passa a ter um dono único.
- Corrige de quebra a `blocked_by_draft_adr`, hoje eficaz apenas no caminho minoritário. ADRs
  criados por `adr new` — a maioria — passam a bloquear REQs `Open` como sempre se pretendeu.
- Segue o padrão já estabelecido no projeto: regra nomeada, severidade configurável, cenário
  de falsificação em CI.

**Negativas / aceitas**

- **A `blocked_by_draft_adr` fica mais rigorosa**, e projetos downstream com ADRs `Proposed`
  ligados a REQs `Open` passarão a ver violações que antes não viam. É a regra passando a fazer
  o que o nome sempre prometeu, mas é mudança de comportamento observável e precisa constar do
  CHANGELOG.
- O nome `blocked_by_draft_adr` fica historicamente impreciso, já que passa a cobrir `Proposed`
  também. Aceito para não quebrar configurações existentes.
- Mais uma regra a manter, com paridade byte-a-byte nos 3 CLIs.
- A definição por exclusão aceita como "aceito" qualquer status desconhecido — inclusive um erro
  de digitação como `Accpeted`. É o trade-off deliberado por não enumerar; a alternativa
  (allowlist fechada) quebraria projetos com vocabulário próprio.

## Alternatives Considered

**Só a regra nova, sem tocar na `blocked_by_draft_adr`** — escopo menor e sem mudança de
comportamento. **Rejeitado por decisão do usuário:** deixaria intacta uma regra que hoje só
funciona no caminho minoritário, e manteria duas noções concorrentes de "ADR não aceito" no
código — exatamente a fragmentação que causou o problema.

**Renomear a regra para `blocked_by_unaccepted_adr`** — resolveria a imprecisão do nome.
**Rejeitado:** os nomes de regra são chave pública de configuração (`rules:` no `trackfw.yaml`).
Renomear quebraria silenciosamente a configuração de quem já ajustou a severidade dessa regra.

**Unificar o vocabulário nos geradores, fazendo `NewADRDraft` emitir `Proposed`** — eliminaria o
estado `Draft` e a ambiguidade na origem. **Rejeitado:** `Draft` e `Proposed` carregam semânticas
distintas — stub gerado automaticamente por `req new` versus decisão redigida deliberadamente.
Colapsá-los perderia informação, e invalidaria ADRs `Draft` já existentes em projetos downstream.

**Tornar a regra nova `warning` em vez de `error`** — adoção mais suave. **Rejeitado:** o caso que
originou esta REQ passou despercebido justamente por não haver sinal algum. Um `warning` num
`validate` que já emite outros warnings teria alta chance de ser ignorado do mesmo jeito. A regra
é configurável para quem discordar.
