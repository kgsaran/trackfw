---
status: Accepted
date: 2026-07-31
author: "Zeus"
---

# ADR: Gerador de roadmap emite heading consolidado de criterios de aceite

> Date: 2026-07-31 | Status: Accepted

## Context

O trackfw gera um roadmap que o próprio trackfw rejeita. Sem nenhuma edição manual:

```
trackfw roadmap new ... && trackfw roadmap move <nome> wip && trackfw validate
✗ roadmap "ROADMAP-....md" is in wip but has no acceptance criteria block
```

Divergência de contrato entre gerador e validador:

| Componente | Valor | Referência |
|---|---|---|
| Gerador emite | `**Acceptance criteria:**` | `internal/generators/roadmap.go:115,156` · `npm/src/generators/roadmap.js:444,500` · `pypi/trackfw/generators/roadmap.py:211,321` |
| Validador procura | `## Acceptance Criteria` / `## Critérios de Aceite` | `internal/config/config.go:83` · `internal/validator/validator.go:989` |

Os três CLIs divergem de forma **idêntica** — é defeito de contrato, não drift de paridade. O
gerador de REQ (`internal/generators/req.go:93`) já emite o heading corretamente; a divergência é
específica do gerador de roadmap.

O dano é agravado pela regra `branch_has_wip_roadmap`, que empurra o usuário exatamente para essa
sequência. Aconteceu nesta sessão (2026-07-31) e exigiu contorno manual.

A questão a decidir: **qual dos dois lados do contrato se ajusta**.

## Decision

**O gerador passa a emitir um heading `## Acceptance Criteria` consolidado, além de manter os
blocos `**Acceptance criteria:**` por microlote.**

1. O heading consolidado é posicionado **após a seção de contexto e antes da primeira wave** —
   onde o leitor procura o resumo do que a entrega precisa satisfazer.
2. Os blocos por ML **permanecem intactos**. Eles são a unidade operacional de cada microlote e
   é neles que o agente executor trabalha. O heading é o resumo consolidado, não um substituto.
3. A seção consolidada é gerada como **placeholder a preencher**, não como agregação automática
   dos critérios dos MLs. Agregar duplicaria conteúdo e criaria duas fontes de verdade que
   divergem na primeira edição manual.
4. A mudança é obrigatória nos **três** geradores simultaneamente — `check-artifact-parity.sh`
   compara os artefatos gerados byte-a-byte.

## Consequences

**Positivas**

- O caminho oficial (`roadmap new` → `move wip` → `validate`) passa a funcionar de ponta a ponta.
  Elimina uma armadilha que custa tempo a toda pessoa e todo agente que segue o fluxo documentado.
- O validador permanece rigoroso: continua exigindo um heading de verdade, não aceita marcador
  em negrito.
- Alinha o gerador de roadmap ao de REQ, que já estava correto — reduz inconsistência interna.

**Negativas / aceitas**

- Roadmaps passam a ter os critérios em dois níveis (consolidado e por ML), o que pode divergir
  se ninguém mantiver o topo. Aceito: o consolidado é resumo para leitura humana; a fonte
  operacional é o bloco do ML. O ciclo desta sessão usou exatamente esse arranjo e funcionou.
- Os três MLs tocam arquivos disjuntos e podem rodar em paralelo, mas o gate de paridade só fecha
  com os três prontos — exige barreira antes da validação final.
- Roadmaps existentes não ganham a seção retroativamente. Aceito: a mudança é de template, e
  reescrever roadmaps em massa é risco desnecessário.

## Alternatives Considered

**Adicionar `"**Acceptance criteria:**"` ao default de `AcceptanceMarkers`** — correção de uma
linha, resolve imediatamente. **Rejeitado:** mascara o defeito em vez de corrigi-lo e enfraquece o
validador, que passaria a aceitar um marcador em negrito no meio do documento como se fosse uma
seção. O objetivo da regra é garantir que exista uma **seção** de critérios; degradar o marcador
esvazia a regra.

**Substituir os blocos por ML pelo heading único** — eliminaria a duplicação. **Rejeitado:** os
critérios por microlote são o contrato de execução de cada ML e o que o agente executor lê. Removê-
los tiraria a granularidade que torna o microlote autocontido.

**Agregar automaticamente os critérios dos MLs no heading consolidado** — evitaria placeholder
vazio. **Rejeitado:** duas representações do mesmo conteúdo divergem na primeira edição manual, e
o gerador não tem como reagregar depois que o arquivo passa a ser editado à mão.

**Documentar o contorno e não corrigir** — **Rejeitado:** já existe nota de vault
(`roadmap-new-gera-marcador-de-aceite-invalido-2026-07-31.md`), mas documentar uma armadilha não é
substituto para removê-la. O trackfw é uma ferramenta de governança; gerar artefato que ele próprio
reprova corrói a confiança na ferramenta.

---

## 🔴 Emenda — 2026-09-26: a Decisão 3 não alcança o `--from-req`

**Quem emenda:** `trackfw_architect`, provocado pelo `ML-1B` da
`REQ-2026-09-09-req-nasce-orfa-…`, que **declarou a divergência em vez de assumir precedência** e
verificou por evidência que esta ADR não tinha emenda nem supersessão.

**A tensão, escrita honestamente.** A Decisão 3 diz:

> *"A seção consolidada é gerada como **placeholder a preencher**, não como agregação automática dos
> critérios dos MLs."*

E o **AC7** da REQ-2026-09-09 pede que o bloco de ACs do roadmap gerado por `--from-req` **deixe de
sair vazio** quando a REQ tem ACs. Lidas literalmente, uma contradiz a outra.

**Por que não há contradição real — a razão declarada da ADR é sobre RE-agregação:**

> *"Agregar duplicaria conteúdo e criaria duas fontes de verdade"* · *"o gerador não tem como
> **reagregar** depois que o arquivo passa a ser editado à mão."*

🔴 **O `--from-req` não reagrega: ele SEMEIA, uma vez, no instante da criação** — e naquele instante
os MLs **são** derivados dos ACs da REQ, logo não existem "duas fontes" a divergir. A segunda fonte
só nasce quando alguém edita o roadmap à mão, e a partir daí o gerador não toca mais no arquivo.

**A emenda, portanto:**

| caminho | comportamento | vigora |
|---|---|---|
| `roadmap new` (template simples) | **placeholder a preencher** | Decisão 3, **inalterada** |
| `roadmap new --from-req` | **semeia os ACs da REQ, uma vez, na criação** | esta emenda |
| qualquer reagregação posterior | **proibida** | Decisão 3, **reforçada** |

⚠️ **O que esta emenda NÃO autoriza:** o gerador **não** passa a reconciliar o bloco com a REQ depois
da criação. Se a REQ ganhar um AC novo, o roadmap **não** é atualizado — e isso é deliberado, pela
razão original da Decisão 3. Quem quiser essa reconciliação precisa de decisão própria, porque é
exatamente onde as duas fontes de verdade nasceriam.

**Precedência:** o AC7 é posterior, explícito e do dono do produto. Mas 🔴 **não o tratei como
revogação automática** — a Decisão 3 continua governando os outros dois caminhos, e o executor
restringiu a mudança ao `--from-req` **antes** de eu decidir, o que preservou a ADR íntegra onde ela
não estava sendo contrariada.

