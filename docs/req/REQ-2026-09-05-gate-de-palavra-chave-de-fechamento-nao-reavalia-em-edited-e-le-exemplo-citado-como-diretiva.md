---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: ""
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

## Negative Scope
- ❌ **Não** mudar quais palavras-chave são aceitas. O inglês é exigência do GitHub, não escolha
  nossa, e já está documentado no template.
- ❌ **Não** transformar o gate em verificador de conteúdo de PR além do fechamento de issue.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
