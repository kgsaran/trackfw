---
status: Done
date: 2026-09-05
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md"
---

# REQ: gate de palavra-chave de fechamento nao reavalia em edited e le exemplo citado como diretiva

> Date: 2026-09-05 | Status: Done
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

- [x] **AC1** — O gate reavalia no evento `edited`, não só na abertura.
      → `ML-N3`. `on.pull_request.types` passou a incluir `edited`, e o `GH_TOKEN` ligou o caminho da
      API que lia o corpo **vivo**. 🔴 **Este AC foi o ÚLTIMO de propósito:** ligar o token antes de
      fechar a forma 4 red-linaria todo PR com escopo negativo — o **#293** está mergeado com
      `**Não fecha #290**` no corpo e passou verde porque o payload era congelado
- [x] **AC2** — 🔴 **Sem repetir as suítes.** Um `edited` que dispare o `quality` inteiro faz o custo
      do gate crescer com a edição de texto. Workflow dedicado, ou filtro de caminho.
      → **workflow dedicado** (`.github/workflows/pr-closing-keyword.yml`). `on.pull_request.types` é
      do **workflow**, não do job: medido que `edited` no `quality.yml` dispararia **13 dos 14 jobs**,
      incluindo as três suítes de `windows-latest`
- [x] **AC3** — ⚠️ **AMPLIADO por decisão do arquiteto (2026-09-25), contra a letra original.** A auditoria é
      explícita: *não deve ser inferido de aspas apenas.* Escrever a regra — bloco de código, cerca,
      seção declarada — e falsificá-la nos dois sentidos: um exemplo citado **não** isenta; uma
      palavra-chave real **fora** de bloco de código continua valendo.
      → 🔴 **A letra deste AC proibia o que salva a frase canônica do #258.** Medido: retiradas as
      aspas, **não sobra sinal nenhum** naquela frase — nem cerca, nem indentação, nem blockquote.
      Manter o AC estrito deixaria o gate acusando para sempre o texto que documenta o próprio bug.
      Aspas entraram como o **quinto de cinco** mecanismos de zona, não como o único — que é o que o
      AC de fato proibia. `ML-N2`: antes `rc=1`, depois `rc=0`
- [x] **AC4** — Falsificação nas duas direções para o `edited`: PR aberto sem palavra-chave e
      corrigido depois → **passa a aprovar**; PR aberto com ela e esvaziado depois → **passa a
      reprovar**. Saída real dos dois.
      → `ML-N3`, arms `edited-api-vence-payload-obsoleto` e `edited-api-acusa-corpo-que-regrediu`,
      falsificadas por sabotagem (`gh_status -eq 0` → `-eq 99`): **as duas direções caem**
- [x] **AC5** — 🔴 **Guarda de vacuidade:** o autoteste do gate já tem cenários de corpo vazio e
      arquivo ausente. Os cenários novos entram lá — um gate que valida corpo de PR e não é
      exercitado sobre corpo nenhum não mede.
      → `--self-test` foi de **26 para 78** cenários. 🔴 **E a distinção foi preservada em vez de
      colapsada:** degradação (API caiu, o payload ainda é corpo **real**) dá veredito e **anuncia** a
      causa, o rc e o stderr; vacuidade (não há corpo) sai **2**. Fazer degradação reprovar
      transformaria *"sem token"* em *"PR vermelho"*, e é assim que se desliga um gate

- [x] **AC6** — 🔴 **A forma 3 (markdown) e a forma 4 (negação) deixam de ser acusadas** — as duas com
      falsificação **nas duas direções**: a frase de escopo negativo **não** é acusada, **e** o
      fechamento legítimo continua sendo exigido
      → `ML-N1`, **no mesmo commit**, e a razão é aritmética: `Não fecha **#421**.` casa as **duas**
      formas, logo a 3 sozinha faria o gate **passar a ver** a frase e acusá-la — falso positivo novo
      onde hoje há silêncio. Corroborado por medição de primeira mão do executor: com a polaridade
      desligada, `rc=1`
- [x] **AC7** — 🔴 **Nenhuma das quatro formas volta pelo remédio da outra.** O discriminante é um só;
      corrigir a negação com uma lista de exceções que quebre o exemplo citado é troca de defeito,
      não correção
      → **um discriminante só**, medido: precisão **1/3 → 4/4**, cobertura **1/4 → 4/4** sobre 358
      corpos reais, e o corpus verificado **PR a PR** a cada microlote — não só no agregado
- [x] **AC8** — O corpus de falsificação inclui as **frases reais** que dispararam as 4 ocorrências
      (#417, #424, e as duas do #258) — não paráfrases
      → e, sem ninguém planejar, **o PR #437 falsificou em produção**: nascido da `main` anterior ao
      merge, o gate **velho** acusou suas linhas `**Não fecha #435**` e `**Não fecha #273**`; depois do
      `update-branch`, o **mesmo corpo byte-idêntico** passou. 🔴 **Nada foi reescrito para agradar ao
      gate — o que mudou foi o discriminante**

## Resultado medido (fechamento pós-merge do PR #436, 2026-09-25)

| | antes | depois |
|---|---|---|
| histograma sobre 358 corpos | `rc=0 355 · rc=1 2 · rc=2 1` | `rc=0 353 · rc=1 4 · rc=2 1` |
| acusados | #247 · **#293** (PR mergeado — FP) | #247 · #312 · #325 · #330 |
| **precisão · cobertura** | **1/3 · 1/4** | **4/4 · 4/4** |
| `--self-test` | 26 cenários | **78** |
| `required_status_checks` | ausente | **presente** (`declared=9, required=9`, D\R=R\W=D\W=∅) |

**A enumeração cresceu de 4 para 8 famílias**, e duas delas mudaram de natureza sob medição:

- 🔴 **A forma 8 NÃO era defeito.** Sete PRs-sonda com braço de controle mediram que o GitHub
  **ignora** palavra-chave em cerca/span/indentado (`[]`) e a **honra** em blockquote/tabela/aspas
  (`[430]`). O comportamento que parecia bug estava **certo**, e "corrigi-lo" instalaria falso
  negativo — pior, porque o incômodo era um aviso **verdadeiro**.
- **A forma 7** (`Sana`, `Soluciona`, `Conserta`) fica **residual aceito e declarado**: ampliar a
  lista de verbos sem medir custo de falso positivo troca gate cego por gate ruidoso.

**Dois falsos negativos vivos foram achados por reconciliação de dívidas, não por busca**, e
corrigidos aqui pela Regra Dura de Causa Raiz. O mais instrutivo: mascarar um code span troca o
trecho por **espaços**, então uma linha que *começa* com code span ganha 4+ espaços à esquerda, a
máscara seguinte a lê como bloco indentado e **engole a linha inteira com a declaração dentro** —
indentação **fabricada pelo passe anterior**, não escrita pelo autor.

**Residual declarado, e não extrapolado por analogia** — foi a analogia que produziu as duas
presunções refutadas: aspas **curvas** e `~~~` (zonas não medidas contra a API, **presas por cenário**
que fica vermelho se entrarem em silêncio), comentário HTML, `<pre>`/`<code>`, segunda referência
coordenada por palavra, e a **superfície de silenciamento da polaridade**, que não fecha em regex:

```
"Não é verdade que fecha #12"    (21 chars)  → deve acusar
"Sem contar o #99, fecha a #12"  (18 chars)  → não deve
```

Mesma janela, vereditos opostos. Como não é eliminável, passou a ser **visível**: toda supressão é
impressa no log, com a frase e o token, nos dois caminhos de saída.

⚠️ **E o "0 falso positivo" é propriedade DESTE corpus, não do discriminante.**

## Negative Scope
- ❌ **Não** mudar quais palavras-chave são aceitas. O inglês é exigência do GitHub, não escolha
  nossa, e já está documentado no template.
- ❌ **Não** transformar o gate em verificador de conteúdo de PR além do fechamento de issue.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md`