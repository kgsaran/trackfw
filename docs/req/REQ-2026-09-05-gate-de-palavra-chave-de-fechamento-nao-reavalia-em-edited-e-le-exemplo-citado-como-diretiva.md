---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md"
---

# REQ: gate de palavra-chave de fechamento nao reavalia em edited e le exemplo citado como diretiva

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue **`#258`**, aberta em 2026-09-03. 🔴 **A auditoria externa de 2026-09-05 registrou que não havia
correção nem artefato** — a issue estava parada há dois dias.

Dois defeitos no gate `check-pr-closing-keyword`:

**1. Não reavalia quando o corpo do PR muda.** O gate roda na abertura; quem corrige a palavra-chave
depois **continua reprovado**, e quem a remove depois **continua aprovado**. O veredito descreve um
estado que não existe mais.

**2. Lê exemplo citado como diretiva.** Um PR que *fale sobre* palavras-chave — como o próprio PR que
instrumentou o gate — é interpretado como se as estivesse usando.

Este gate tem valor **medido**: de 241 PRs mergeados, apenas 4 fechavam issue automaticamente, porque
o repositório escrevia "Fecha #123" em português, que o GitHub ignora. Depois da instrumentação, o
PR #281 fechou `#278` e `#279` sozinho. **É justamente por funcionar que os dois defeitos importam.**

## 🔴 Ampliação de escopo (2026-09-24): são QUATRO formas, não duas

Pela Regra Dura de Causa Raiz, as duas formas descobertas depois desta REQ **entram nela**, não em
REQ nova. A causa é a mesma: **o discriminante não olha o contexto do verbo.**

| # | forma | medido em | efeito |
|---|---|---|---|
| 1 | não reavalia em `edited` | #258 (2026-09-03) | veredito descreve estado que não existe mais |
| 2 | exemplo citado lido como diretiva | #258 | PR que *fala sobre* o gate é acusado |
| 3 | **`Fecha **#N**`** — markdown entre a palavra e o número | 2026-09-12 | a adjacência quebra, o gate **não vê** |
| 4 | 🔴 **negação lida como afirmação** | PR **#417** (2026-09-23) e PR **#424** (2026-09-24) | o gate manda **fechar o que o PR declara não fechar** |

### A forma 4 é a mais séria, e não é por ser a mais frequente

As três primeiras produzem **ruído**. A quarta produz **dano dirigido**: o conselho do gate
(`Troque por: Closes #N`) fecharia uma issue viva, num PR cuja frase seguinte explica **por que** ela
não fecha. O gate existe para impedir que o texto minta sobre fechamento — e nessa forma **é ele
quem introduz a mentira**.

**Quatro ocorrências, e a quarta é o argumento:**

| ocorrência | autor |
|---|---|
| #417 — *"o PR **não** fecha a #363"* | contribuidor externo |
| #424 — *"**Não fecha a #421**"* | 🔴 **o arquiteto, um dia depois de documentar a forma no #258** |

Se quem conhece o defeito e comentou sobre ele ontem ainda o dispara, **o gate não está educando
ninguém — está cobrando pedágio.**

### 🔴 E o contorno atual treina o time a não declarar escopo negativo

O remédio, nas duas vezes, foi **reescrever a frase para não conter a palavra proibida** — *"a #421
permanece aberta"*. O texto fica pior e a informação é a mesma. Um gate cujo remédio é **piorar a
redação** desincentiva exatamente o que esta casa exige em toda REQ: dizer o que o trabalho **não**
faz.

**Isso muda um critério:** não basta o gate parar de acusar; ele precisa **não punir a frase natural**.

## Acceptance Criteria

- [ ] **AC1** — O gate reavalia no evento `edited`, não só na abertura.
- [ ] **AC2** — 🔴 **Sem repetir as suítes.** Um `edited` que dispare o `quality` inteiro faz o custo
      do gate crescer com a edição de texto. Workflow dedicado, ou filtro de caminho.
- [ ] **AC3** — 🔴 **Contrato próprio para "exemplo citado", não heurística de aspas.** A auditoria é
      explícita: *não deve ser inferido de aspas apenas.* Escrever a regra — bloco de código, cerca,
      seção declarada — e falsificá-la nos dois sentidos: um exemplo citado **não** isenta; uma
      palavra-chave real **fora** de bloco de código continua valendo.
- [ ] **AC4** — Falsificação nas duas direções para o `edited`: PR aberto sem palavra-chave e
      corrigido depois → **passa a aprovar**; PR aberto com ela e esvaziado depois → **passa a
      reprovar**. Saída real dos dois.
- [ ] **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
      arquivo ausente. Os cenários novos entram lá — um gate que valida corpo de PR e não é
      exercitado sobre corpo nenhum não mede.

- [ ] **AC6** — 🔴 **A forma 3 (markdown) e a forma 4 (negação) deixam de ser acusadas** — as duas com
      falsificação **nas duas direções**: a frase de escopo negativo **não** é acusada, **e** o
      fechamento legítimo continua sendo exigido
- [ ] **AC7** — 🔴 **Nenhuma das quatro formas volta pelo remédio da outra.** O discriminante é um só;
      corrigir a negação com uma lista de exceções que quebre o exemplo citado é troca de defeito,
      não correção
- [ ] **AC8** — O corpus de falsificação inclui as **frases reais** que dispararam as 4 ocorrências
      (#417, #424, e as duas do #258) — não paráfrases

## Negative Scope
- ❌ **Não** mudar quais palavras-chave são aceitas. O inglês é exigência do GitHub, não escolha
  nossa, e já está documentado no template.
- ❌ **Não** transformar o gate em verificador de conteúdo de PR além do fechamento de issue.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md`